package listenplayback

import (
	"context"
	"time"
)

func (service *PlayerService) HandleTrackEnded(ctx context.Context, observedVideoID string) error {
	defer service.PublishSnapshot(ctx)
	observedVideoID = normalizedVideoID(observedVideoID)

	service.mu.Lock()
	service.songNearingEnd = false
	if len(service.queue) == 0 {
		if service.repeatMode == RepeatModeOne && (service.hasCurrentTrack || service.pendingPlayVideoID != "") {
			actions := service.replayCurrentSongForRepeatOneWithoutQueueLocked()
			service.mu.Unlock()
			return service.executeActions(ctx, actions...)
		}
		service.markPlaybackEndedLocked()
		service.mu.Unlock()
		service.saveCurrentSession(ctx)
		return nil
	}

	if observedVideoID != "" {
		expectedCurrentVideoID := service.expectedCurrentVideoIDLocked()
		if expectedCurrentVideoID != "" && expectedCurrentVideoID != observedVideoID {
			if service.repeatMode == RepeatModeOne {
				// Repeat one is allowed to recover from an autoplay id.
			} else if service.isRepeatAllWraparoundTrackEndLocked(observedVideoID, expectedCurrentVideoID) {
				// Already wrapped to first queue item before the ended callback arrived.
			} else {
				service.mu.Unlock()
				return nil
			}
		}
	}

	if !service.canAdvanceQueueAfterTrackEndLocked() {
		service.suppressAutoplayAfterEnd = true
		service.markPlaybackEndedLocked()
		service.mu.Unlock()
		if err := service.executeActions(ctx, transportAction{kind: "pause"}); err != nil {
			return err
		}
		service.saveCurrentSession(ctx)
		return nil
	}
	service.suppressAutoplayAfterEnd = false
	if service.repeatMode == RepeatModeOne {
		actions := service.replayCurrentQueueSongForRepeatOneAfterTrackEndLocked(ctx)
		service.mu.Unlock()
		return service.executeActions(ctx, actions...)
	}
	service.mu.Unlock()
	return service.Next(ctx)
}

func (service *PlayerService) UpdateTrackMetadata(ctx context.Context, observed ObservedTrack) error {
	observed.ObservedVideoID = normalizedVideoID(observed.ObservedVideoID)
	observed.Title = stringsTrim(observed.Title)
	observed.Artist = stringsTrim(observed.Artist)
	observed.ThumbnailURL = stringsTrim(observed.ThumbnailURL)
	observed.LikeStatus = stringsTrim(observed.LikeStatus)
	observed.MetadataSource = stringsTrim(observed.MetadataSource)

	var trackForEnrichment Track
	service.mu.Lock()
	before := service.metadataPublishStateLocked()
	actions := service.updateTrackMetadataLocked(ctx, observed)
	shouldPublish := service.metadataPublishStateChangedLocked(before)
	if service.hasCurrentTrack {
		trackForEnrichment = service.currentTrack
	}
	service.mu.Unlock()
	err := service.executeActions(ctx, actions...)
	service.requestTrackMetadataEnrichment(trackForEnrichment)
	if shouldPublish || len(actions) > 0 {
		service.PublishSnapshot(ctx)
		service.saveCurrentSession(ctx)
	}
	if err != nil {
		return err
	}
	return nil
}

func (service *PlayerService) updateTrackMetadataLocked(ctx context.Context, observed ObservedTrack) []transportAction {
	resolvedVideoID := service.resolvedObservedVideoIDLocked(observed.ObservedVideoID)
	trackChanged := observed.TrackChanged ||
		(service.hasCurrentTrack &&
			(service.currentTrack.Title != observed.Title ||
				service.currentTrack.Artist != observed.Artist ||
				service.currentTrack.VideoID != resolvedVideoID))

	if service.suppressUnexpectedAutoplayAfterQueueEndIfNeededLocked(trackChanged, observed) {
		return []transportAction{{kind: "pause"}}
	}
	if actions, handled := service.handleAppInitiatedPlaybackMetadataLocked(observed, trackChanged); handled {
		return actions
	}
	if actions, handled := service.handleNearEndTrackChangeIfNeededLocked(ctx, observed, trackChanged); handled {
		return actions
	}
	if actions, handled := service.handleUnexpectedQueueDriftIfNeededLocked(observed, trackChanged); handled {
		return actions
	}
	if actions, handled := service.finalRepeatOneSafetyNetIfNeededLocked(ctx, observed, trackChanged); handled {
		return actions
	}
	if queued, ok := service.currentQueueTrackLocked(); ok && queued.VideoID == resolvedVideoID {
		service.keepQueueTrackVisibleLocked(queued, observed)
		return nil
	}
	if service.repeatMode == RepeatModeOne && len(service.queue) > 0 {
		if queued, ok := service.currentQueueTrackLocked(); ok {
			service.keepQueueTrackVisibleLocked(queued, observed)
		}
		return nil
	}
	service.currentTrack = service.observedTrackLocked(resolvedVideoID, observed)
	service.hasCurrentTrack = true
	return nil
}

type metadataPublishState struct {
	hasCurrentTrack bool
	currentTrack    Track
	queue           []Track
	currentIndex    int
	state           PlaybackState
	pendingVideoID  string
}

func (service *PlayerService) metadataPublishStateLocked() metadataPublishState {
	return metadataPublishState{
		hasCurrentTrack: service.hasCurrentTrack,
		currentTrack:    service.currentTrack,
		queue:           cloneTracks(service.queue),
		currentIndex:    service.currentIndex,
		state:           service.state,
		pendingVideoID:  service.pendingPlayVideoID,
	}
}

func (service *PlayerService) metadataPublishStateChangedLocked(before metadataPublishState) bool {
	if before.hasCurrentTrack != service.hasCurrentTrack ||
		!trackEqual(before.currentTrack, service.currentTrack) ||
		before.currentIndex != service.currentIndex ||
		before.state != service.state ||
		before.pendingVideoID != service.pendingPlayVideoID {
		return true
	}
	return !tracksEqual(before.queue, service.queue)
}

func (service *PlayerService) suppressUnexpectedAutoplayAfterQueueEndIfNeededLocked(trackChanged bool, observed ObservedTrack) bool {
	if !trackChanged || !service.suppressAutoplayAfterEnd {
		return false
	}
	current, ok := service.currentQueueTrackLocked()
	if !ok || service.observedTrackMatchesTrackLocked(observed, current) {
		return false
	}
	service.markPlaybackEndedLocked()
	service.keepQueueTrackVisibleLocked(current, observed)
	return true
}

func (service *PlayerService) handleAppInitiatedPlaybackMetadataLocked(observed ObservedTrack, trackChanged bool) ([]transportAction, bool) {
	if !service.appInitiatedPlayback || len(service.queue) == 0 {
		return nil, false
	}
	intended, ok := service.currentQueueTrackLocked()
	if !ok {
		service.appInitiatedPlayback = false
		return nil, false
	}
	matchesObservedVideo := observed.ObservedVideoID != "" && observed.ObservedVideoID == intended.VideoID
	if matchesObservedVideo && service.shouldKeepQueueMetadataLocked(observed, intended) {
		service.appInitiatedPlayback = false
		service.keepQueueTrackVisibleLocked(intended, observed)
		return nil, true
	}
	if service.observedTrackMatchesTrackLocked(observed, intended) {
		service.appInitiatedPlayback = false
		return nil, false
	}
	if !trackChanged {
		return nil, false
	}
	service.appInitiatedPlayback = false
	action, err := service.preparePlayTrackLocked(intended, VideoLoadForceFullPageWhenSameVideoID, PlayOptions{ForceReload: true})
	if err != nil {
		return nil, true
	}
	return []transportAction{action}, true
}

func (service *PlayerService) handleNearEndTrackChangeIfNeededLocked(ctx context.Context, observed ObservedTrack, trackChanged bool) ([]transportAction, bool) {
	if !trackChanged || len(service.queue) == 0 || !service.songNearingEnd {
		return nil, false
	}
	service.songNearingEnd = false
	if expectedIndex, ok := service.expectedQueueIndexAfterCurrentTrackLocked(); ok {
		expected := service.queue[expectedIndex]
		if !service.observedTrackMatchesTrackLocked(observed, expected) {
			if service.repeatMode == RepeatModeOne {
				return service.replayCurrentQueueSongForRepeatOneAfterTrackEndLocked(ctx), true
			}
			return service.nextActionsLocked(), true
		}
		service.currentIndex = expectedIndex
		if service.shouldKeepQueueMetadataLocked(observed, expected) {
			service.keepQueueTrackVisibleLocked(expected, observed)
			return nil, true
		}
		return nil, false
	}
	if service.canAdvanceQueueAfterTrackEndLocked() {
		if service.repeatMode == RepeatModeOne {
			return service.replayCurrentQueueSongForRepeatOneAfterTrackEndLocked(ctx), true
		}
		return service.nextActionsLocked(), true
	}
	service.markPlaybackEndedLocked()
	return []transportAction{{kind: "pause"}}, true
}

func (service *PlayerService) handleUnexpectedQueueDriftIfNeededLocked(observed ObservedTrack, trackChanged bool) ([]transportAction, bool) {
	if len(service.queue) == 0 || observed.ObservedVideoID == "" {
		return nil, false
	}
	current, ok := service.currentQueueTrackLocked()
	if !ok || current.VideoID == observed.ObservedVideoID {
		return nil, false
	}
	if !trackChanged && service.repeatMode != RepeatModeOne {
		return nil, false
	}
	if service.repeatMode == RepeatModeOne {
		action, err := service.preparePlayTrackLocked(current, VideoLoadForceFullPageWhenSameVideoID, PlayOptions{ForceReload: true})
		if err != nil {
			return nil, true
		}
		return []transportAction{action}, true
	}
	for index, track := range service.queue {
		if track.VideoID != observed.ObservedVideoID {
			continue
		}
		queueIndexChanged := index != service.currentIndex
		if queueIndexChanged {
			service.currentIndex = index
		}
		if queueIndexChanged || service.shouldKeepQueueMetadataLocked(observed, track) {
			service.keepQueueTrackVisibleLocked(track, observed)
			return nil, true
		}
		return nil, false
	}
	action, err := service.preparePlayTrackLocked(current, VideoLoadForceFullPageWhenSameVideoID, PlayOptions{ForceReload: true})
	if err != nil {
		return nil, true
	}
	return []transportAction{action}, true
}

func (service *PlayerService) finalRepeatOneSafetyNetIfNeededLocked(ctx context.Context, observed ObservedTrack, trackChanged bool) ([]transportAction, bool) {
	if service.repeatMode != RepeatModeOne || !service.hasUserInteractedThisSession {
		return nil, false
	}
	queued, ok := service.currentQueueTrackLocked()
	if !ok {
		return nil, false
	}
	videoMismatch := observed.ObservedVideoID != "" && observed.ObservedVideoID != queued.VideoID
	titleDriftWithoutVideoID := observed.ObservedVideoID == "" && observed.Title != "" && trackChanged &&
		!service.metadataMatchesTrackLocked(observed.Title, observed.Artist, queued)
	if !videoMismatch && !titleDriftWithoutVideoID {
		return nil, false
	}
	service.keepQueueTrackVisibleLocked(queued, observed)
	now := time.Now()
	if !service.lastRepeatOneRecoveryAt.IsZero() && now.Sub(service.lastRepeatOneRecoveryAt) < repeatOneRecoveryThrottle {
		return nil, true
	}
	service.lastRepeatOneRecoveryAt = now
	action, err := service.preparePlayTrackLocked(queued, VideoLoadForceFullPageWhenSameVideoID, PlayOptions{ForceReload: true})
	if err != nil {
		return nil, true
	}
	return []transportAction{action}, true
}

func (service *PlayerService) nextActionsLocked() []transportAction {
	if len(service.queue) == 0 {
		return nil
	}
	service.alignCurrentIndexToCurrentTrackLocked()
	if service.shuffleEnabled && len(service.queue) > 1 && service.currentIndex >= len(service.queue)-1 {
		service.materializeShuffleQueueLocked(service.queue, service.currentIndex, false, false)
	}
	index := service.currentIndex + 1
	if service.currentIndex >= len(service.queue)-1 {
		if service.repeatMode == RepeatModeAll {
			index = 0
		} else {
			return nil
		}
	}
	action, err := service.playQueueIndexLocked(index, true, VideoLoadStandard)
	if err != nil {
		return nil
	}
	return []transportAction{action}
}

func (service *PlayerService) replayCurrentQueueSongForRepeatOneAfterTrackEndLocked(ctx context.Context) []transportAction {
	current, ok := service.currentQueueTrackLocked()
	if !ok {
		return nil
	}
	service.songNearingEnd = false
	aligned := service.pendingPlayVideoID == current.VideoID
	if service.transport != nil {
		aligned = aligned && service.transport.CurrentVideoID(ctx) == current.VideoID
	}
	if service.hasUserInteractedThisSession && aligned {
		if service.state == PlaybackStateEnded || service.state == PlaybackStateLoading {
			service.state = PlaybackStatePlaying
		}
		return []transportAction{{kind: "seek", seconds: 0}, {kind: "play"}}
	}
	action, err := service.preparePlayTrackLocked(current, VideoLoadPreferInPlaceWhenSameVideoID, PlayOptions{RestartFromStart: true})
	if err != nil {
		return nil
	}
	return []transportAction{action}
}

func (service *PlayerService) replayCurrentSongForRepeatOneWithoutQueueLocked() []transportAction {
	service.songNearingEnd = false
	if service.hasCurrentTrack {
		action, err := service.preparePlayTrackLocked(service.currentTrack, VideoLoadPreferInPlaceWhenSameVideoID, PlayOptions{RestartFromStart: true})
		if err != nil {
			return nil
		}
		return []transportAction{action}
	}
	if service.pendingPlayVideoID != "" {
		track := Track{ID: service.pendingPlayVideoID, VideoID: service.pendingPlayVideoID, Title: service.pendingPlayVideoID}
		action, err := service.preparePlayTrackLocked(track, VideoLoadStandard, PlayOptions{RestartFromStart: true})
		if err != nil {
			return nil
		}
		return []transportAction{action}
	}
	return nil
}

func (service *PlayerService) canAdvanceQueueAfterTrackEndLocked() bool {
	return service.shuffleEnabled ||
		service.repeatMode == RepeatModeOne ||
		service.currentIndex < len(service.queue)-1 ||
		service.repeatMode == RepeatModeAll ||
		service.mixContinuationToken != ""
}

func (service *PlayerService) expectedQueueIndexAfterCurrentTrackLocked() (int, bool) {
	if len(service.queue) == 0 {
		return 0, false
	}
	if service.repeatMode == RepeatModeOne {
		return service.currentIndex, true
	}
	if service.shuffleEnabled {
		return 0, false
	}
	if service.currentIndex < len(service.queue)-1 {
		return service.currentIndex + 1, true
	}
	if service.repeatMode == RepeatModeAll {
		return 0, true
	}
	return 0, false
}

func (service *PlayerService) isRepeatAllWraparoundTrackEndLocked(observedVideoID string, expectedCurrentVideoID string) bool {
	if service.repeatMode != RepeatModeAll || service.shuffleEnabled {
		return false
	}
	expectedIndex, ok := service.expectedQueueIndexAfterCurrentTrackLocked()
	if !ok || expectedIndex != 0 || len(service.queue) == 0 || service.currentIndex >= len(service.queue) {
		return false
	}
	return service.queue[service.currentIndex].VideoID == expectedCurrentVideoID &&
		service.queue[0].VideoID == observedVideoID
}

func (service *PlayerService) currentQueueTrackLocked() (Track, bool) {
	if len(service.queue) == 0 {
		return Track{}, false
	}
	index := safeQueueIndex(service.currentIndex, len(service.queue))
	return service.queue[index], true
}

func (service *PlayerService) expectedCurrentVideoIDLocked() string {
	if current, ok := service.currentQueueTrackLocked(); ok {
		return current.VideoID
	}
	if service.hasCurrentTrack {
		return service.currentTrack.VideoID
	}
	return service.pendingPlayVideoID
}

func (service *PlayerService) resolvedObservedVideoIDLocked(videoID string) string {
	if videoID != "" {
		return videoID
	}
	if service.hasCurrentTrack && service.currentTrack.VideoID != "" {
		return service.currentTrack.VideoID
	}
	if service.pendingPlayVideoID != "" {
		return service.pendingPlayVideoID
	}
	return "unknown"
}

func (service *PlayerService) observedTrackMatchesTrackLocked(observed ObservedTrack, track Track) bool {
	if observed.ObservedVideoID != "" {
		return observed.ObservedVideoID == track.VideoID
	}
	return service.metadataMatchesTrackLocked(observed.Title, observed.Artist, track)
}

func (service *PlayerService) metadataMatchesTrackLocked(title string, artist string, track Track) bool {
	return track.Title == title && track.Artist == artist
}

func (service *PlayerService) shouldKeepQueueMetadataLocked(observed ObservedTrack, track Track) bool {
	return observed.Title == "" || observed.Artist == "" || !service.metadataMatchesTrackLocked(observed.Title, observed.Artist, track)
}

func (service *PlayerService) keepQueueTrackVisibleLocked(track Track, observed ObservedTrack) {
	track = service.mergeObservedQueueTrackLocked(track, observed)
	service.currentTrack = track
	service.hasCurrentTrack = true
	service.pendingPlayVideoID = track.VideoID
}

func (service *PlayerService) mergeObservedQueueTrackLocked(track Track, observed ObservedTrack) Track {
	track = normalizeTrack(track)
	if service.shouldFillAuthoritativeTitleLocked(track, observed.Title) {
		track.Title = observed.Title
	}
	if track.DurationSeconds <= 0 && service.duration > 0 {
		track.DurationSeconds = service.duration
	}
	if track.ThumbnailURL == "" || shouldUseObservedThumbnailForVideoAvailability(track, observed.ThumbnailURL) {
		track.ThumbnailURL = observed.ThumbnailURL
	}
	if observed.LikeStatus != "" {
		track.LikeStatus = observed.LikeStatus
	}
	return normalizeTrack(track)
}

func (service *PlayerService) shouldFillAuthoritativeTitleLocked(track Track, observedTitle string) bool {
	if observedTitle == "" {
		return false
	}
	return track.Title == "" || track.Title == track.VideoID || track.Title == "unknown" || track.Title == "Loading..."
}

func (service *PlayerService) observedTrackLocked(resolvedVideoID string, observed ObservedTrack) Track {
	track := Track{
		ID:      resolvedVideoID,
		VideoID: resolvedVideoID,
	}
	if service.hasCurrentTrack && service.currentTrack.VideoID == resolvedVideoID {
		track = normalizeTrack(service.currentTrack)
		track.ID = resolvedVideoID
		track.VideoID = resolvedVideoID
	}
	if observed.Title != "" {
		track.Title = observed.Title
	}
	if observed.Artist != "" {
		track.Artist = observed.Artist
		track.ArtistSource = observed.MetadataSource
		if track.ArtistSource == "" {
			track.ArtistSource = TrackArtistSourceObserved
		}
	}
	if track.ThumbnailURL == "" || shouldUseObservedThumbnailForVideoAvailability(track, observed.ThumbnailURL) {
		track.ThumbnailURL = observed.ThumbnailURL
	}
	if observed.LikeStatus != "" {
		track.LikeStatus = observed.LikeStatus
	}
	if service.duration > 0 {
		track.DurationSeconds = service.duration
	}
	return normalizeTrack(track)
}
