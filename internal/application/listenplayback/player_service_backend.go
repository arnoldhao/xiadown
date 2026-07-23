package listenplayback

import (
	"context"
	"fmt"
	"strconv"
)

const (
	mediaMetadataArtistBrowseID         = "youtubeMusic.artistBrowseId"
	mediaMetadataArtistSource           = "youtubeMusic.artistSource"
	mediaMetadataDurationLabel          = "youtubeMusic.durationLabel"
	mediaMetadataMusicVideoType         = "youtubeMusic.musicVideoType"
	mediaMetadataHasVideo               = "youtubeMusic.hasVideo"
	mediaMetadataVideoAvailabilityKnown = "youtubeMusic.videoAvailabilityKnown"
	mediaMetadataLikeStatus             = "youtubeMusic.likeStatus"
	mediaMetadataInLibrary              = "youtubeMusic.inLibrary"
	mediaMetadataFeedbackAdd            = "youtubeMusic.feedbackAdd"
	mediaMetadataFeedbackRemove         = "youtubeMusic.feedbackRemove"
)

// PlayerServiceBackend adapts the established YouTube Music PlayerService to
// the provider-neutral PlaybackBackend contract. PlayerService itself and its
// Track/Transport API remain unchanged.
type PlayerServiceBackend struct {
	service *PlayerService
}

func NewPlayerServiceBackend(service *PlayerService) *PlayerServiceBackend {
	return &PlayerServiceBackend{service: service}
}

func (backend *PlayerServiceBackend) Provider() PlaybackProvider {
	return PlaybackProviderYouTubeMusic
}

func (backend *PlayerServiceBackend) Capabilities() PlaybackCapabilities {
	if backend == nil || backend.service == nil {
		return PlaybackCapabilities{
			Available:         false,
			UnsupportedReason: "YouTube Music player service is unavailable",
			MediaKinds:        []MediaKind{MediaKindAudio},
		}
	}
	return PlaybackCapabilities{
		Available:  true,
		MediaKinds: []MediaKind{MediaKindAudio},
		PlayPause:  true,
		Stop:       true,
		Seek:       true,
		Previous:   true,
		Next:       true,
		Volume:     true,
		Queue:      true,
		Shuffle:    true,
		Repeat:     true,
		Lyrics:     true,
		Video:      true,
		Fullscreen: true,
	}
}

func (backend *PlayerServiceBackend) Start(ctx context.Context, request PlaybackStartRequest) error {
	if err := backend.ensureService(); err != nil {
		return err
	}
	if request.Item.Source.Provider != PlaybackProviderYouTubeMusic {
		return fmt.Errorf("PlayerServiceBackend cannot play provider %q", request.Item.Source.Provider)
	}
	track := TrackFromMediaItem(request.Item)
	if track.VideoID == "" {
		return fmt.Errorf("YouTube Music media item requires a source id")
	}
	if err := backend.service.SetVolume(ctx, request.Volume, request.Muted); err != nil {
		return err
	}
	return backend.service.PlayTrack(ctx, track, PlayOptions{
		StartSeconds: request.StartSeconds,
		ForceReload:  request.ForceReload,
	})
}

func (backend *PlayerServiceBackend) Play(ctx context.Context) error {
	if err := backend.ensureService(); err != nil {
		return err
	}
	return backend.service.Resume(ctx)
}

func (backend *PlayerServiceBackend) Pause(ctx context.Context) error {
	if err := backend.ensureService(); err != nil {
		return err
	}
	return backend.service.Pause(ctx)
}

func (backend *PlayerServiceBackend) Stop(ctx context.Context) error {
	if err := backend.ensureService(); err != nil {
		return err
	}
	return backend.service.Stop(ctx)
}

func (backend *PlayerServiceBackend) Seek(ctx context.Context, seconds float64) error {
	if err := backend.ensureService(); err != nil {
		return err
	}
	return backend.service.Seek(ctx, seconds)
}

func (backend *PlayerServiceBackend) SetVolume(ctx context.Context, volume float64, muted bool) error {
	if err := backend.ensureService(); err != nil {
		return err
	}
	return backend.service.SetVolume(ctx, volume, muted)
}

func (backend *PlayerServiceBackend) Previous(ctx context.Context) error {
	if err := backend.ensureService(); err != nil {
		return err
	}
	return backend.service.Previous(ctx)
}

func (backend *PlayerServiceBackend) Next(ctx context.Context) error {
	if err := backend.ensureService(); err != nil {
		return err
	}
	return backend.service.Next(ctx)
}

// Snapshot converts the legacy service snapshot for consumers migrating to the
// global contract without changing PlayerService.Snapshot.
func (backend *PlayerServiceBackend) Snapshot(ctx context.Context) PlaybackSnapshot {
	if backend == nil || backend.service == nil {
		return PlaybackSnapshot{}
	}
	return PlaybackSnapshotFromLegacy(backend.service.Snapshot(ctx), PlaybackProviderYouTubeMusic)
}

// Subscribe adapts legacy snapshot events. It is intentionally optional rather
// than part of PlaybackBackend so backends can be integrated incrementally.
func (backend *PlayerServiceBackend) Subscribe(listener PlaybackSnapshotListener) func() {
	if backend == nil || backend.service == nil || listener == nil {
		return func() {}
	}
	return backend.service.Subscribe(func(snapshot Snapshot) {
		listener(PlaybackSnapshotFromLegacy(snapshot, PlaybackProviderYouTubeMusic))
	})
}

func (backend *PlayerServiceBackend) ensureService() error {
	if backend == nil || backend.service == nil {
		return &PlaybackUnsupportedError{
			Provider: PlaybackProviderYouTubeMusic,
			Reason:   "YouTube Music player service is unavailable",
		}
	}
	return nil
}

// MediaItemFromTrack translates the existing Track shape without changing it.
func MediaItemFromTrack(track Track, provider PlaybackProvider) MediaItem {
	if provider == "" {
		provider = PlaybackProviderYouTubeMusic
	}
	kind := MediaKindAudio
	if provider == PlaybackProviderYouTube {
		kind = MediaKindVideo
	}
	artists := make([]string, 0, len(track.Artists))
	for _, artist := range track.Artists {
		if artist.Name != "" {
			artists = append(artists, artist.Name)
		}
	}
	metadata := map[string]string{
		mediaMetadataArtistBrowseID:         track.ArtistBrowseID,
		mediaMetadataArtistSource:           track.ArtistSource,
		mediaMetadataDurationLabel:          track.DurationLabel,
		mediaMetadataMusicVideoType:         track.MusicVideoType,
		mediaMetadataHasVideo:               strconv.FormatBool(track.HasVideo),
		mediaMetadataVideoAvailabilityKnown: strconv.FormatBool(track.VideoAvailabilityKnown),
		mediaMetadataLikeStatus:             track.LikeStatus,
		mediaMetadataInLibrary:              strconv.FormatBool(track.InLibrary),
		mediaMetadataFeedbackAdd:            track.FeedbackTokens.Add,
		mediaMetadataFeedbackRemove:         track.FeedbackTokens.Remove,
	}
	for key, value := range metadata {
		if value == "" {
			delete(metadata, key)
		}
	}
	return MediaItem{
		ID:         track.ID,
		Kind:       kind,
		Source:     PlaybackSource{Provider: provider, ID: track.VideoID},
		Title:      track.Title,
		Artist:     track.Artist,
		Artists:    artists,
		ArtworkURL: track.ThumbnailURL,
		Duration:   track.DurationSeconds,
		Metadata:   metadata,
	}
}

// TrackFromMediaItem translates a generic item for the legacy YouTube Music
// service. Provider-specific details are retained when the item originated via
// MediaItemFromTrack.
func TrackFromMediaItem(item MediaItem) Track {
	artists := make([]TrackArtist, 0, len(item.Artists))
	for _, name := range item.Artists {
		if name != "" {
			artists = append(artists, TrackArtist{Name: name})
		}
	}
	track := Track{
		ID:              item.ID,
		VideoID:         item.Source.ID,
		Title:           item.Title,
		Artist:          item.Artist,
		Artists:         artists,
		DurationSeconds: item.Duration,
		ThumbnailURL:    item.ArtworkURL,
		HasVideo:        item.Kind == MediaKindVideo,
	}
	if track.ID == "" {
		track.ID = track.VideoID
	}
	if item.Metadata == nil {
		return track
	}
	track.ArtistBrowseID = item.Metadata[mediaMetadataArtistBrowseID]
	track.ArtistSource = item.Metadata[mediaMetadataArtistSource]
	track.DurationLabel = item.Metadata[mediaMetadataDurationLabel]
	track.MusicVideoType = item.Metadata[mediaMetadataMusicVideoType]
	track.LikeStatus = item.Metadata[mediaMetadataLikeStatus]
	track.FeedbackTokens = FeedbackTokens{
		Add:    item.Metadata[mediaMetadataFeedbackAdd],
		Remove: item.Metadata[mediaMetadataFeedbackRemove],
	}
	track.HasVideo = metadataBool(item.Metadata, mediaMetadataHasVideo, track.HasVideo)
	track.VideoAvailabilityKnown = metadataBool(item.Metadata, mediaMetadataVideoAvailabilityKnown, false)
	track.InLibrary = metadataBool(item.Metadata, mediaMetadataInLibrary, false)
	return track
}

// PlaybackSnapshotFromLegacy translates a legacy snapshot into the generic
// persistent-session shape used by new workspace surfaces.
func PlaybackSnapshotFromLegacy(snapshot Snapshot, provider PlaybackProvider) PlaybackSnapshot {
	if provider == "" {
		provider = PlaybackProviderYouTubeMusic
	}
	result := PlaybackSnapshot{Version: snapshot.Version}
	track := snapshot.CurrentTrack
	if track == nil && snapshot.PendingPlayVideoID != "" {
		track = &Track{
			ID:      snapshot.PendingPlayVideoID,
			VideoID: snapshot.PendingPlayVideoID,
			Title:   snapshot.PendingPlayVideoID,
		}
	}
	if track == nil {
		return result
	}
	item := MediaItemFromTrack(*track, provider)
	queue := make([]MediaItem, len(snapshot.Queue))
	for index, queuedTrack := range snapshot.Queue {
		queue[index] = MediaItemFromTrack(queuedTrack, provider)
	}
	capabilities := PlaybackCapabilities{
		Available:  true,
		MediaKinds: []MediaKind{item.Kind},
		PlayPause:  true,
		Stop:       true,
		Seek:       true,
		Previous:   true,
		Next:       true,
		Volume:     true,
		Queue:      true,
		Shuffle:    true,
		Repeat:     true,
	}
	if provider == PlaybackProviderYouTubeMusic {
		capabilities.Lyrics = true
		capabilities.Video = true
		capabilities.Fullscreen = true
	}
	session := PlaybackSessionSnapshot{
		ID:             "legacy:" + string(provider),
		Focus:          PlaybackFocusPersistent,
		State:          snapshot.State,
		Item:           item,
		Capabilities:   capabilities,
		Position:       snapshot.Progress,
		Duration:       snapshot.Duration,
		Volume:         snapshot.Volume,
		Muted:          snapshot.Muted,
		Queue:          queue,
		CurrentIndex:   snapshot.CurrentIndex,
		ShuffleEnabled: snapshot.ShuffleEnabled,
		RepeatMode:     snapshot.RepeatMode,
	}
	result.Active = &session
	if playbackStateMayBeAudible(snapshot.State) {
		result.AudibleSessionID = session.ID
	}
	return result
}

func metadataBool(metadata map[string]string, key string, fallback bool) bool {
	value, exists := metadata[key]
	if !exists {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
