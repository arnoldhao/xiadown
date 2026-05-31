package wails

import (
	"context"
	"fmt"
	"strings"

	"xiadown/internal/application/listenplayback"
)

type ListenPlaybackQueueRequest struct {
	Tracks       []listenplayback.Track `json:"tracks"`
	StartingAt   int                    `json:"startingAt"`
	Title        string                 `json:"title,omitempty"`
	Kind         string                 `json:"kind,omitempty"`
	PlaylistID   string                 `json:"playlistId,omitempty"`
	StartVideoID string                 `json:"startVideoId,omitempty"`
}

type ListenPlaybackTrackRequest struct {
	Track            listenplayback.Track `json:"track"`
	StartSeconds     float64              `json:"startSeconds,omitempty"`
	RestartFromStart bool                 `json:"restartFromStart,omitempty"`
	ForceReload      bool                 `json:"forceReload,omitempty"`
}

type ListenPlaybackTrackMetadataRequest struct {
	Track listenplayback.Track `json:"track"`
}

type ListenPlaybackObservationRequest struct {
	ObservedVideoID string                       `json:"observedVideoId,omitempty"`
	Title           string                       `json:"title,omitempty"`
	Artist          string                       `json:"artist,omitempty"`
	ThumbnailURL    string                       `json:"thumbnailUrl,omitempty"`
	LikeStatus      string                       `json:"likeStatus,omitempty"`
	TrackChanged    bool                         `json:"trackChanged,omitempty"`
	State           listenplayback.PlaybackState `json:"state,omitempty"`
	Progress        float64                      `json:"progress,omitempty"`
	Duration        float64                      `json:"duration,omitempty"`
	Paused          bool                         `json:"paused,omitempty"`
	Ended           bool                         `json:"ended,omitempty"`
}

type ListenPlaybackSeekRequest struct {
	Seconds float64 `json:"seconds"`
}

type ListenPlaybackVolumeRequest struct {
	Volume float64 `json:"volume"`
	Muted  bool    `json:"muted"`
}

type ListenPlaybackMoveQueueRequest struct {
	Source      []int `json:"source"`
	Destination int   `json:"destination"`
}

type ListenPlaybackQueueItemsRequest struct {
	Tracks []listenplayback.Track `json:"tracks"`
}

type ListenPlaybackRemoveQueueRequest struct {
	TrackIDs []string `json:"trackIds"`
	VideoIDs []string `json:"videoIds"`
}

type ListenPlaybackReorderQueueRequest struct {
	VideoIDs []string `json:"videoIds"`
}

type ListenPlaybackRepeatModeRequest struct {
	Mode listenplayback.RepeatMode `json:"mode"`
}

type ListenPlaybackShuffleRequest struct {
	Enabled bool `json:"enabled"`
}

func (handler *ListenPlayerHandler) PlaybackSnapshot(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) PlayTrack(ctx context.Context, request ListenPlaybackTrackRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.RecordPlaybackIntent()
	err := handler.service.PlayTrack(ctx, request.Track, listenplayback.PlayOptions{
		StartSeconds:     request.StartSeconds,
		RestartFromStart: request.RestartFromStart,
		ForceReload:      request.ForceReload,
	})
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) MergeTrackMetadata(ctx context.Context, request ListenPlaybackTrackMetadataRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.MergeTrackMetadata(ctx, request.Track)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) ObservePlayback(ctx context.Context, request ListenPlaybackObservationRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	isPlaying := (request.State == listenplayback.PlaybackStatePlaying ||
		request.State == listenplayback.PlaybackStateBuffering) &&
		!request.Paused &&
		!request.Ended
	if err := handler.service.UpdatePlaybackState(ctx, isPlaying, request.Progress, request.Duration); err != nil {
		return handler.service.Snapshot(ctx), err
	}
	var err error
	if request.TrackChanged ||
		strings.TrimSpace(request.ObservedVideoID) != "" ||
		strings.TrimSpace(request.Title) != "" ||
		strings.TrimSpace(request.Artist) != "" ||
		strings.TrimSpace(request.ThumbnailURL) != "" ||
		strings.TrimSpace(request.LikeStatus) != "" {
		err = handler.service.UpdateTrackMetadata(ctx, listenplayback.ObservedTrack{
			ObservedVideoID: request.ObservedVideoID,
			Title:           request.Title,
			Artist:          request.Artist,
			ThumbnailURL:    request.ThumbnailURL,
			LikeStatus:      request.LikeStatus,
			TrackChanged:    request.TrackChanged,
			MetadataSource:  "frontend-observation",
		})
	}
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) PlayQueue(ctx context.Context, request ListenPlaybackQueueRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.RecordPlaybackIntent()
	var err error
	switch strings.TrimSpace(request.Kind) {
	case "mix":
		err = handler.service.PlayWithMix(ctx, request.PlaylistID, request.StartVideoID, request.Title)
	case "radio":
		if len(request.Tracks) == 1 {
			err = handler.service.PlayWithRadio(ctx, request.Tracks[0], request.Title)
		} else {
			err = handler.service.PlayRadioQueue(ctx, request.Tracks, request.StartingAt, request.Title)
		}
	default:
		err = handler.service.PlayQueue(ctx, request.Tracks, request.StartingAt, request.Title)
	}
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) PlayPause(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.RecordPlaybackIntent()
	err := handler.service.PlayPause(ctx)
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) Next(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.RecordPlaybackIntent()
	err := handler.service.Next(ctx)
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) Previous(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.RecordPlaybackIntent()
	err := handler.service.Previous(ctx)
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) PlaybackPause(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	err := handler.service.Pause(ctx)
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) PlaybackResume(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.RecordPlaybackIntent()
	err := handler.service.Resume(ctx)
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) PlaybackSeek(ctx context.Context, request ListenPlaybackSeekRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	err := handler.service.Seek(ctx, request.Seconds)
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) PlaybackSetVolume(ctx context.Context, request ListenPlaybackVolumeRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	err := handler.service.SetVolume(ctx, request.Volume, request.Muted)
	return handler.service.Snapshot(ctx), err
}

func (handler *ListenPlayerHandler) ToggleShuffle(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.ToggleShuffle()
	handler.service.PersistPlaybackSession(ctx)
	return handler.service.PublishSnapshot(ctx), nil
}

func (handler *ListenPlayerHandler) SetShuffle(ctx context.Context, request ListenPlaybackShuffleRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.SetShuffleEnabled(request.Enabled)
	handler.service.PersistPlaybackSession(ctx)
	return handler.service.PublishSnapshot(ctx), nil
}

func (handler *ListenPlayerHandler) CycleRepeatMode(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.CycleRepeatMode()
	handler.service.PersistPlaybackSession(ctx)
	return handler.service.PublishSnapshot(ctx), nil
}

func (handler *ListenPlayerHandler) SetRepeatMode(ctx context.Context, request ListenPlaybackRepeatModeRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.SetRepeatMode(request.Mode)
	handler.service.PersistPlaybackSession(ctx)
	return handler.service.PublishSnapshot(ctx), nil
}

func (handler *ListenPlayerHandler) MoveQueueItems(ctx context.Context, request ListenPlaybackMoveQueueRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.MoveQueueItems(ctx, request.Source, request.Destination)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) InsertNextInQueue(ctx context.Context, request ListenPlaybackQueueItemsRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.InsertNextInQueue(ctx, request.Tracks)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) AppendToQueue(ctx context.Context, request ListenPlaybackQueueItemsRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.AppendToQueue(ctx, request.Tracks)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) RemoveFromQueue(ctx context.Context, request ListenPlaybackRemoveQueueRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	trackIDs := make(map[string]struct{}, len(request.TrackIDs))
	for _, trackID := range request.TrackIDs {
		trackID = strings.TrimSpace(trackID)
		if trackID != "" {
			trackIDs[trackID] = struct{}{}
		}
	}
	videoIDs := make(map[string]struct{}, len(request.VideoIDs))
	for _, videoID := range request.VideoIDs {
		videoID = strings.TrimSpace(videoID)
		if videoID != "" {
			videoIDs[videoID] = struct{}{}
		}
	}
	handler.service.RemoveFromQueue(ctx, trackIDs, videoIDs)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) ReorderQueue(ctx context.Context, request ListenPlaybackReorderQueueRequest) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.ReorderQueue(ctx, request.VideoIDs)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) ClearQueue(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.ClearQueue(ctx)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) ClearQueueEntirely(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.ClearQueueEntirely(ctx)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) UndoQueue(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.UndoQueue(ctx)
	return handler.service.Snapshot(ctx), nil
}

func (handler *ListenPlayerHandler) RedoQueue(ctx context.Context) (listenplayback.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenplayback.Snapshot{}, fmt.Errorf("listen playback service unavailable")
	}
	handler.service.RedoQueue(ctx)
	return handler.service.Snapshot(ctx), nil
}
