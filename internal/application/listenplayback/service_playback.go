package listenplayback

import (
	"context"
	"fmt"
	"math"
	"strings"
)

const manualSeekToEndThreshold = 0.5

type transportAction struct {
	kind     string
	request  PlayRequest
	strategy VideoLoadStrategy
	seconds  float64
	volume   float64
	muted    bool
}

func (service *PlayerService) PlayVideo(ctx context.Context, videoID string, options PlayOptions) error {
	track := Track{
		ID:      normalizedVideoID(videoID),
		VideoID: normalizedVideoID(videoID),
		Title:   normalizedVideoID(videoID),
	}
	return service.PlayTrack(ctx, track, options)
}

func (service *PlayerService) PlayTrack(ctx context.Context, track Track, options PlayOptions) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	action, err := service.preparePlayTrackLocked(track, VideoLoadStandard, options)
	service.mu.Unlock()
	if err != nil {
		return err
	}
	return service.executeActions(ctx, action)
}

func (service *PlayerService) PlayPause(ctx context.Context) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	if service.pendingRestoredLoadDeferred || service.shouldLoadPendingVideoBeforePlaybackLocked(ctx) {
		service.mu.Unlock()
		return service.Resume(ctx)
	}
	service.clearRestoredPlaybackSessionStateLocked()
	if service.pendingPlayVideoID != "" {
		service.state = PlaybackStateLoading
		service.mu.Unlock()
		return service.executeActions(ctx, transportAction{kind: "play"})
	}
	playing := service.state.IsPlaying()
	service.mu.Unlock()
	if playing {
		return service.Pause(ctx)
	}
	return service.Resume(ctx)
}

func (service *PlayerService) Pause(ctx context.Context) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	if service.pendingRestoredLoadDeferred {
		service.state = PlaybackStatePaused
		service.mu.Unlock()
		return nil
	}
	service.clearRestoredPlaybackSessionStateLocked()
	service.state = PlaybackStatePaused
	service.mu.Unlock()
	return service.executeActions(ctx, transportAction{kind: "pause"})
}

func (service *PlayerService) Resume(ctx context.Context) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	pendingVideoID := service.pendingPlayVideoID
	if pendingVideoID == "" {
		service.clearRestoredPlaybackSessionStateLocked()
		service.state = PlaybackStateLoading
		service.mu.Unlock()
		return service.executeActions(ctx, transportAction{kind: "play"})
	}

	shouldLoadPending := service.shouldLoadPendingVideoBeforePlaybackLocked(ctx)
	if service.pendingRestoredLoadDeferred {
		service.beginRestoredPlaybackLoadLocked(service.hasUserInteractedThisSession)
	} else {
		service.clearRestoredPlaybackSessionStateLocked()
	}

	if shouldLoadPending {
		if !service.hasUserInteractedThisSession {
			service.showMiniPlayer = true
			service.state = PlaybackStatePaused
			service.mu.Unlock()
			return nil
		}
		service.showMiniPlayer = false
		service.state = PlaybackStateLoading
		track := service.currentTrack
		if !service.hasCurrentTrack || track.VideoID == "" {
			track = Track{ID: pendingVideoID, VideoID: pendingVideoID, Title: pendingVideoID}
		}
		request := service.playRequestLocked(track, PlayOptions{
			StartSeconds: service.pendingRestoredSeek,
		})
		service.mu.Unlock()
		return service.executeActions(ctx, transportAction{
			kind:     "load",
			request:  request,
			strategy: VideoLoadStandard,
		})
	}

	service.state = PlaybackStateLoading
	service.mu.Unlock()
	return service.executeActions(ctx, transportAction{kind: "play"})
}

func (service *PlayerService) Seek(ctx context.Context, seconds float64) error {
	defer service.PublishSnapshot(ctx)
	target := clampSeconds(seconds)
	service.mu.Lock()
	if service.duration > 0 && target > service.duration {
		target = service.duration
	}
	if service.pendingRestoredLoadDeferred {
		service.progress = target
		service.currentTimeMs = int(target*1000 + 0.5)
		service.pendingRestoredSeek = target
		service.mu.Unlock()
		return nil
	}
	service.clearRestoredPlaybackSessionStateLocked()
	if service.manualSeekReachedEndLocked(target) {
		duration := service.duration
		service.progress = duration
		service.currentTimeMs = int(duration*1000 + 0.5)
		observedVideoID := service.expectedCurrentVideoIDLocked()
		actions := service.terminalManualSeekActionsLocked(duration)
		service.mu.Unlock()
		if err := service.executeActions(ctx, actions...); err != nil {
			return err
		}
		return service.HandleTrackEnded(ctx, observedVideoID)
	}
	service.progress = target
	service.currentTimeMs = int(target*1000 + 0.5)
	service.mu.Unlock()
	return service.executeActions(ctx, transportAction{kind: "seek", seconds: target})
}

func (service *PlayerService) manualSeekReachedEndLocked(target float64) bool {
	if service.duration <= manualSeekToEndThreshold || target <= 0 {
		return false
	}
	return target >= service.duration-manualSeekToEndThreshold ||
		math.Abs(target-service.duration) <= manualSeekToEndThreshold
}

func (service *PlayerService) terminalManualSeekActionsLocked(duration float64) []transportAction {
	if !service.shouldSynchronizeTerminalManualSeekLocked() {
		return nil
	}
	actions := []transportAction{{kind: "seek", seconds: duration}}
	if len(service.queue) == 0 && service.repeatMode != RepeatModeOne {
		actions = append(actions, transportAction{kind: "pause"})
	}
	return actions
}

func (service *PlayerService) shouldSynchronizeTerminalManualSeekLocked() bool {
	if len(service.queue) == 0 {
		return !(service.repeatMode == RepeatModeOne && (service.hasCurrentTrack || service.pendingPlayVideoID != ""))
	}
	return !service.canAdvanceQueueAfterTrackEndLocked()
}

func (service *PlayerService) SetVolume(ctx context.Context, volume float64, muted bool) error {
	defer service.PublishSnapshot(ctx)
	nextVolume := clampVolume(volume)
	service.mu.Lock()
	service.volume = nextVolume
	service.muted = muted || nextVolume <= 0
	if nextVolume > 0 {
		service.volumeBeforeMute = nextVolume
	}
	service.mu.Unlock()
	err := service.executeActions(ctx, transportAction{
		kind:   "volume",
		volume: nextVolume,
		muted:  muted || nextVolume <= 0,
	})
	if err != nil {
		return err
	}
	service.saveCurrentSession(ctx)
	return nil
}

func (service *PlayerService) ToggleMute(ctx context.Context) error {
	service.mu.Lock()
	if service.muted || service.volume <= 0 {
		volume := service.volumeBeforeMute
		if volume <= 0 {
			volume = 1
		}
		service.mu.Unlock()
		return service.SetVolume(ctx, volume, false)
	}
	volumeBeforeMute := service.volume
	if volumeBeforeMute <= 0 {
		volumeBeforeMute = 1
	}
	service.volumeBeforeMute = volumeBeforeMute
	service.mu.Unlock()
	return service.SetVolume(ctx, 0, true)
}

func (service *PlayerService) Stop(ctx context.Context) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	service.clearRestoredPlaybackSessionStateLocked()
	service.state = PlaybackStateIdle
	service.songNearingEnd = false
	service.appInitiatedPlayback = false
	service.suppressAutoplayAfterEnd = false
	service.hasCurrentTrack = false
	service.currentTrack = Track{}
	service.progress = 0
	service.currentTimeMs = 0
	service.duration = 0
	service.pendingPlayVideoID = ""
	service.mu.Unlock()
	return service.executeActions(ctx, transportAction{kind: "pause"})
}

func (service *PlayerService) preparePlayTrackLocked(
	track Track,
	strategy VideoLoadStrategy,
	options PlayOptions,
) (transportAction, error) {
	track = normalizeTrack(track)
	if track.VideoID == "" {
		return transportAction{}, fmt.Errorf("track video id is required")
	}
	service.clearRestoredPlaybackSessionStateLocked()
	service.state = PlaybackStateLoading
	service.err = ""
	service.songNearingEnd = false
	service.suppressAutoplayAfterEnd = false
	service.currentTrack = track
	service.hasCurrentTrack = true
	service.progress = clampSeconds(options.StartSeconds)
	service.currentTimeMs = int(service.progress*1000 + 0.5)
	service.duration = clampSeconds(track.DurationSeconds)
	service.observedPlaybackAudioQuality = ""
	service.pendingPlayVideoID = track.VideoID
	service.appInitiatedPlayback = true

	if !service.hasUserInteractedThisSession {
		service.showMiniPlayer = true
		return transportAction{}, nil
	}
	service.showMiniPlayer = false
	return transportAction{
		kind:     "load",
		request:  service.playRequestLocked(track, options),
		strategy: strategy,
	}, nil
}

func (service *PlayerService) playRequestLocked(track Track, options PlayOptions) PlayRequest {
	return PlayRequest{
		Track:            track,
		StartSeconds:     clampSeconds(options.StartSeconds),
		RestartFromStart: options.RestartFromStart,
		ForceReload:      options.ForceReload,
		Volume:           service.volume,
		Muted:            service.muted,
	}
}

func (service *PlayerService) executeActions(ctx context.Context, actions ...transportAction) error {
	for _, action := range actions {
		var err error
		switch action.kind {
		case "":
			continue
		case "load":
			service.requestTrackMetadataEnrichment(action.request.Track)
			if service.transport == nil {
				continue
			}
			err = service.transport.LoadVideo(ctx, action.request, action.strategy)
		case "play":
			if service.transport == nil {
				continue
			}
			err = service.transport.Play(ctx)
		case "pause":
			if service.transport == nil {
				continue
			}
			err = service.transport.Pause(ctx)
		case "seek":
			if service.transport == nil {
				continue
			}
			err = service.transport.Seek(ctx, action.seconds)
		case "volume":
			if service.transport == nil {
				continue
			}
			err = service.transport.SetVolume(ctx, action.volume, action.muted)
		case "next":
			if service.transport == nil {
				continue
			}
			err = service.transport.Next(ctx)
		case "previous":
			if service.transport == nil {
				continue
			}
			err = service.transport.Previous(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func normalizeTrack(track Track) Track {
	track.ID = stringsTrim(track.ID)
	track.VideoID = normalizedVideoID(track.VideoID)
	track.Title = stringsTrim(track.Title)
	track.Artist = stringsTrim(track.Artist)
	track.Artists = normalizeTrackArtists(track.Artists)
	track.ArtistBrowseID = stringsTrim(track.ArtistBrowseID)
	track.ArtistSource = stringsTrim(track.ArtistSource)
	track.DurationLabel = stringsTrim(track.DurationLabel)
	track.ThumbnailURL = stringsTrim(track.ThumbnailURL)
	track.MusicVideoType = stringsTrim(track.MusicVideoType)
	track.LikeStatus = stringsTrim(track.LikeStatus)
	if track.ID == "" {
		track.ID = track.VideoID
	}
	if track.Title == "" {
		track.Title = track.VideoID
	}
	return normalizeTrackVideoAvailability(track)
}

func normalizeTrackArtists(artists []TrackArtist) []TrackArtist {
	if len(artists) == 0 {
		return nil
	}
	normalized := make([]TrackArtist, 0, len(artists))
	seen := make(map[string]struct{}, len(artists))
	for _, artist := range artists {
		name := stringsTrim(artist.Name)
		browseID := stringsTrim(artist.BrowseID)
		if name == "" {
			continue
		}
		key := browseID
		if key == "" {
			key = strings.ToLower(name)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, TrackArtist{Name: name, BrowseID: browseID})
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
