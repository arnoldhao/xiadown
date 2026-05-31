package listenplayback

import (
	"context"
	"math"
)

func (service *PlayerService) UpdatePlaybackState(ctx context.Context, isPlaying bool, progress float64, duration float64) error {
	progress = clampSeconds(progress)
	duration = clampSeconds(duration)

	service.mu.Lock()
	if service.shouldIgnoreZeroProgressResetLocked(progress, duration) {
		service.mu.Unlock()
		return nil
	}
	previousProgress := service.progress
	if service.restoringPlaybackSession {
		actions := service.reconcileRestoredPlaybackStateLocked(isPlaying, progress, duration, previousProgress)
		shouldPublish := service.playbackSnapshotChangedLocked()
		service.mu.Unlock()
		if err := service.executeActions(ctx, actions...); err != nil {
			return err
		}
		if shouldPublish || len(actions) > 0 {
			service.PublishSnapshot(ctx)
		}
		return nil
	}
	service.applyObservedPlaybackStateLocked(isPlaying, progress, duration, previousProgress)
	shouldPublish := service.playbackSnapshotChangedLocked()
	service.mu.Unlock()
	if shouldPublish {
		service.PublishSnapshot(ctx)
		service.saveCurrentSession(ctx)
	}
	return nil
}

func (service *PlayerService) UpdateLyricsTime(_ context.Context, currentTime float64) {
	if service == nil {
		return
	}
	currentTime = clampSeconds(currentTime)
	service.mu.Lock()
	service.currentTimeMs = int(currentTime*1000 + 0.5)
	service.mu.Unlock()
}

func (service *PlayerService) shouldIgnoreZeroProgressResetLocked(progress float64, duration float64) bool {
	return service.progress > 0.75 && progress <= 0.05 && duration <= 0.05
}

func (service *PlayerService) ApplyRestoredPlaybackSession(queue []Track, currentIndex int, progress float64, duration float64) {
	defer service.PublishSnapshot(context.Background())
	service.applyRestoredPlaybackSession(queue, QueueKindPlaylist, "", currentIndex, progress, duration)
	service.requestCurrentQueueMetadataEnrichment()
}

func (service *PlayerService) applyRestoredPlaybackSession(
	queue []Track,
	queueKind QueueKind,
	queueTitle string,
	currentIndex int,
	progress float64,
	duration float64,
) {
	tracks := assignUniqueQueueTrackIDs(normalizeTracks(queue))
	if len(tracks) == 0 {
		return
	}
	index := safeQueueIndex(currentIndex, len(tracks))
	currentTrack := tracks[index]
	resolvedDuration := duration
	if resolvedDuration <= 0 {
		resolvedDuration = currentTrack.DurationSeconds
	}
	clampedProgress := service.clampedRestoredProgress(progress, resolvedDuration)

	service.mu.Lock()
	defer service.mu.Unlock()
	service.clearRestoredPlaybackSessionStateLocked()
	service.clearForwardSkipNavigationStackLocked()
	service.queue = tracks
	switch queueKind {
	case QueueKindRadio, QueueKindPlaylist, QueueKindMix:
		service.queueKind = queueKind
	default:
		service.queueKind = QueueKindPlaylist
	}
	service.queueTitle = stringsTrim(queueTitle)
	service.currentIndex = index
	service.currentTrack = currentTrack
	service.hasCurrentTrack = true
	service.pendingPlayVideoID = currentTrack.VideoID
	service.showMiniPlayer = false
	service.songNearingEnd = false
	service.appInitiatedPlayback = false
	service.progress = clampedProgress
	service.currentTimeMs = int(clampedProgress*1000 + 0.5)
	service.duration = resolvedDuration
	service.state = PlaybackStatePaused
	service.pendingRestoredSeek = clampedProgress
	service.pendingRestoredLoadDeferred = true
}

func (service *PlayerService) ClearRestoredPlaybackSessionState() {
	defer service.PublishSnapshot(context.Background())
	service.mu.Lock()
	defer service.mu.Unlock()
	service.clearRestoredPlaybackSessionStateLocked()
}

func (service *PlayerService) clearRestoredPlaybackSessionStateLocked() {
	service.pendingRestoredSeek = 0
	service.pendingRestoredLoadDeferred = false
	service.restoringPlaybackSession = false
	service.autoResumeAfterRestoredSeek = false
}

func (service *PlayerService) BeginRestoredPlaybackLoad(autoResumeAfterSeek bool) {
	defer service.PublishSnapshot(context.Background())
	service.mu.Lock()
	defer service.mu.Unlock()
	service.beginRestoredPlaybackLoadLocked(autoResumeAfterSeek)
}

func (service *PlayerService) beginRestoredPlaybackLoadLocked(autoResumeAfterSeek bool) {
	service.pendingRestoredLoadDeferred = false
	service.restoringPlaybackSession = true
	service.autoResumeAfterRestoredSeek = autoResumeAfterSeek
	if autoResumeAfterSeek {
		service.state = PlaybackStateLoading
	}
}

func (service *PlayerService) ShouldLoadPendingVideoBeforePlayback(ctx context.Context) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.shouldLoadPendingVideoBeforePlaybackLocked(ctx)
}

func (service *PlayerService) shouldLoadPendingVideoBeforePlaybackLocked(ctx context.Context) bool {
	if service.pendingPlayVideoID == "" {
		return false
	}
	if service.transport == nil {
		return true
	}
	return service.transport.CurrentVideoID(ctx) != service.pendingPlayVideoID
}

func (service *PlayerService) applyObservedPlaybackStateLocked(isPlaying bool, progress float64, duration float64, previousProgress float64) {
	service.progress = progress
	service.currentTimeMs = int(progress*1000 + 0.5)
	service.duration = duration
	if isPlaying {
		service.confirmPlaybackStartedLocked()
	} else if service.state == PlaybackStatePlaying {
		service.state = PlaybackStatePaused
	}
	if duration > 0 && progress >= duration-2 && previousProgress < duration-2 {
		service.songNearingEnd = true
	}
}

func (service *PlayerService) playbackSnapshotChangedLocked() bool {
	if service.state != service.lastPublishedPlaybackState ||
		service.songNearingEnd != service.lastPublishedPlaybackNearingEnd {
		return true
	}
	if playbackSecondsChanged(service.lastPublishedPlaybackDuration, service.duration) {
		return true
	}
	return playbackSecondsChanged(service.lastPublishedPlaybackProgress, service.progress)
}

func playbackSecondsChanged(previous float64, next float64) bool {
	if previous == next {
		return false
	}
	return math.Abs(next-previous) >= playbackSnapshotMinDelta.Seconds()
}

func (service *PlayerService) reconcileRestoredPlaybackStateLocked(
	isPlaying bool,
	progress float64,
	duration float64,
	previousProgress float64,
) []transportAction {
	resolvedDuration := service.resolveRestoredDurationLocked(duration)
	if service.pendingRestoredSeek > 0 {
		return service.reconcilePendingRestoredSeekLocked(
			isPlaying,
			progress,
			service.pendingRestoredSeek,
			resolvedDuration,
		)
	}
	if progress > 0 {
		service.progress = progress
		service.currentTimeMs = int(progress*1000 + 0.5)
	} else {
		service.progress = previousProgress
		service.currentTimeMs = int(previousProgress*1000 + 0.5)
	}
	return service.reconcileRestoredPlaybackWithoutPendingSeekLocked(isPlaying, resolvedDuration)
}

func (service *PlayerService) resolveRestoredDurationLocked(duration float64) float64 {
	resolvedDuration := duration
	if resolvedDuration <= 0 {
		resolvedDuration = service.duration
	}
	service.duration = resolvedDuration
	return resolvedDuration
}

func (service *PlayerService) reconcilePendingRestoredSeekLocked(
	isPlaying bool,
	progress float64,
	targetProgress float64,
	resolvedDuration float64,
) []transportAction {
	clampedTarget := service.clampedRestoredProgress(targetProgress, resolvedDuration)
	service.progress = clampedTarget
	service.currentTimeMs = int(clampedTarget*1000 + 0.5)
	if resolvedDuration <= 0 && clampedTarget > 0 {
		if service.autoResumeAfterRestoredSeek {
			service.state = PlaybackStateLoading
		} else {
			service.state = PlaybackStatePaused
		}
		return nil
	}
	atRestoredPosition := service.isAtRestoredPosition(progress, clampedTarget)
	var actions []transportAction
	if !atRestoredPosition && resolvedDuration > 0 {
		actions = append(actions, transportAction{kind: "seek", seconds: clampedTarget})
	}
	if service.autoResumeAfterRestoredSeek {
		return append(actions, service.finishRestoredAutoResumeLoadLocked(isPlaying, progress, clampedTarget, atRestoredPosition)...)
	}
	return append(actions, service.finishRestoredPausedLoadLocked(isPlaying, progress, clampedTarget, atRestoredPosition)...)
}

func (service *PlayerService) finishRestoredAutoResumeLoadLocked(
	isPlaying bool,
	observedProgress float64,
	targetProgress float64,
	atRestoredPosition bool,
) []transportAction {
	service.state = PlaybackStateLoading
	if !atRestoredPosition && targetProgress > 0 {
		if isPlaying {
			return []transportAction{{kind: "pause"}}
		}
		return nil
	}
	if atRestoredPosition {
		service.progress = observedProgress
		service.currentTimeMs = int(observedProgress*1000 + 0.5)
	} else {
		service.progress = targetProgress
		service.currentTimeMs = int(targetProgress*1000 + 0.5)
	}
	shouldIssuePlay := !isPlaying
	service.clearRestoredPlaybackSessionStateLocked()
	if shouldIssuePlay {
		return []transportAction{{kind: "play"}}
	}
	service.state = PlaybackStatePlaying
	return nil
}

func (service *PlayerService) finishRestoredPausedLoadLocked(
	isPlaying bool,
	observedProgress float64,
	targetProgress float64,
	atRestoredPosition bool,
) []transportAction {
	service.state = PlaybackStatePaused
	if isPlaying {
		return []transportAction{{kind: "pause"}}
	}
	if !atRestoredPosition && targetProgress > 0 {
		return nil
	}
	if atRestoredPosition {
		service.progress = observedProgress
		service.currentTimeMs = int(observedProgress*1000 + 0.5)
	} else {
		service.progress = targetProgress
		service.currentTimeMs = int(targetProgress*1000 + 0.5)
	}
	service.clearRestoredPlaybackSessionStateLocked()
	return nil
}

func (service *PlayerService) reconcileRestoredPlaybackWithoutPendingSeekLocked(isPlaying bool, resolvedDuration float64) []transportAction {
	if service.autoResumeAfterRestoredSeek {
		service.state = PlaybackStateLoading
		if isPlaying {
			service.clearRestoredPlaybackSessionStateLocked()
			service.state = PlaybackStatePlaying
			return nil
		}
		if resolvedDuration > 0 {
			service.clearRestoredPlaybackSessionStateLocked()
			return []transportAction{{kind: "play"}}
		}
		return nil
	}
	service.state = PlaybackStatePaused
	if !isPlaying && resolvedDuration > 0 {
		service.clearRestoredPlaybackSessionStateLocked()
	}
	return nil
}

func (service *PlayerService) clampedRestoredProgress(progress float64, duration float64) float64 {
	progress = clampSeconds(progress)
	duration = clampSeconds(duration)
	if duration > 0 && progress > duration {
		return duration
	}
	return progress
}

func (service *PlayerService) isAtRestoredPosition(observedProgress float64, targetProgress float64) bool {
	return math.Abs(clampSeconds(observedProgress)-clampSeconds(targetProgress)) <= restoredSeekTolerance
}
