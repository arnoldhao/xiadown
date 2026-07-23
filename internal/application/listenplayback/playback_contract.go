package listenplayback

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// PlaybackProvider identifies the system responsible for resolving and playing
// a media source. It intentionally describes a provider rather than a UI
// workspace so playback can survive workspace navigation.
type PlaybackProvider string

const (
	PlaybackProviderYouTubeMusic PlaybackProvider = "youtube_music"
	PlaybackProviderYouTube      PlaybackProvider = "youtube"
	PlaybackProviderLocal        PlaybackProvider = "local"
	PlaybackProviderStream       PlaybackProvider = "stream"
)

// MediaKind describes the presentation/decoder shape of a media item.
type MediaKind string

const (
	MediaKindAudio MediaKind = "audio"
	MediaKindVideo MediaKind = "video"
)

// PlaybackSource is the provider-owned identity of a media item. ID is used
// for provider catalog entries while URI is used for files and direct streams.
type PlaybackSource struct {
	Provider PlaybackProvider `json:"provider"`
	ID       string           `json:"id,omitempty"`
	URI      string           `json:"uri,omitempty"`
	Live     bool             `json:"live,omitempty"`
}

// MediaItem is the provider-neutral item consumed by global playback UI.
// Metadata is reserved for provider details that do not affect core controls.
type MediaItem struct {
	ID           string            `json:"id"`
	Kind         MediaKind         `json:"kind"`
	Source       PlaybackSource    `json:"source"`
	Title        string            `json:"title"`
	Artist       string            `json:"artist,omitempty"`
	Artists      []string          `json:"artists,omitempty"`
	ArtworkURL   string            `json:"artworkUrl,omitempty"`
	CanonicalURL string            `json:"canonicalUrl,omitempty"`
	Duration     float64           `json:"duration,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// PlaybackCapabilities lets each surface render only controls supported by
// the selected backend. Available=false is an explicit unsupported backend,
// not an implicit absence of implementation.
type PlaybackCapabilities struct {
	Available         bool        `json:"available"`
	UnsupportedReason string      `json:"unsupportedReason,omitempty"`
	MediaKinds        []MediaKind `json:"mediaKinds,omitempty"`
	PlayPause         bool        `json:"playPause"`
	Stop              bool        `json:"stop"`
	Seek              bool        `json:"seek"`
	Previous          bool        `json:"previous"`
	Next              bool        `json:"next"`
	Volume            bool        `json:"volume"`
	Queue             bool        `json:"queue"`
	Shuffle           bool        `json:"shuffle"`
	Repeat            bool        `json:"repeat"`
	Lyrics            bool        `json:"lyrics"`
	Video             bool        `json:"video"`
	Like              bool        `json:"like"`
	Dislike           bool        `json:"dislike"`
	Captions          bool        `json:"captions"`
	AudioTracks       bool        `json:"audioTracks"`
	Quality           bool        `json:"quality"`
	Fullscreen        bool        `json:"fullscreen"`
}

func (capabilities PlaybackCapabilities) SupportsKind(kind MediaKind) bool {
	if !capabilities.Available {
		return false
	}
	if len(capabilities.MediaKinds) == 0 {
		return true
	}
	for _, supported := range capabilities.MediaKinds {
		if supported == kind {
			return true
		}
	}
	return false
}

// PlaybackStartRequest is the atomic load-and-play operation expected from a
// backend. Keeping start atomic allows the existing YouTube Music service to
// adapt without changing its established PlayTrack behavior.
type PlaybackStartRequest struct {
	SessionID    string    `json:"sessionId"`
	Item         MediaItem `json:"item"`
	StartSeconds float64   `json:"startSeconds,omitempty"`
	Volume       float64   `json:"volume"`
	Muted        bool      `json:"muted"`
	ForceReload  bool      `json:"forceReload,omitempty"`
}

// PlaybackBackend is the provider-facing transport contract. Implementations
// may wrap a webview player, a remote stream, or a desktop local-media engine.
type PlaybackBackend interface {
	Provider() PlaybackProvider
	Capabilities() PlaybackCapabilities
	Start(context.Context, PlaybackStartRequest) error
	Play(context.Context) error
	Pause(context.Context) error
	Stop(context.Context) error
	Seek(context.Context, float64) error
	SetVolume(context.Context, float64, bool) error
	Previous(context.Context) error
	Next(context.Context) error
}

// PlaybackSnapshotBackend is implemented by backends that can expose their
// current, provider-owned state. The coordinator uses this optional contract
// when legacy entry points can start playback outside StartSession.
type PlaybackSnapshotBackend interface {
	PlaybackBackend
	Snapshot(context.Context) PlaybackSnapshot
}

// PlaybackFocus defines ownership of the single audible slot.
type PlaybackFocus string

const (
	PlaybackFocusPersistent       PlaybackFocus = "persistent"
	PlaybackFocusTransientPreview PlaybackFocus = "transient_preview"
)

// PreviewResumePolicy controls what happens to a persistent session when a
// transient preview closes.
type PreviewResumePolicy string

const (
	PreviewResumeIfPreviouslyPlaying PreviewResumePolicy = "resume_if_previously_playing"
	PreviewKeepPersistentPaused      PreviewResumePolicy = "keep_persistent_paused"
)

// PlaybackSessionRequest opens a focus-owning playback session. An empty ID is
// filled by PlaybackCoordinator. When Volume is omitted, an existing active
// session's volume and mute state are inherited across the handoff.
type PlaybackSessionRequest struct {
	SessionID           string              `json:"sessionId,omitempty"`
	Focus               PlaybackFocus       `json:"focus"`
	Item                MediaItem           `json:"item"`
	StartSeconds        float64             `json:"startSeconds,omitempty"`
	Volume              *float64            `json:"volume,omitempty"`
	Muted               bool                `json:"muted,omitempty"`
	ForceReload         bool                `json:"forceReload,omitempty"`
	RetainRollback      bool                `json:"retainRollback,omitempty"`
	PreviewResumePolicy PreviewResumePolicy `json:"previewResumePolicy,omitempty"`
}

// PlaybackSessionSnapshot is a serializable view of one coordinator session.
type PlaybackSessionSnapshot struct {
	ID             string               `json:"id"`
	Focus          PlaybackFocus        `json:"focus"`
	State          PlaybackState        `json:"state"`
	ErrorMessage   string               `json:"errorMessage,omitempty"`
	Item           MediaItem            `json:"item"`
	Capabilities   PlaybackCapabilities `json:"capabilities"`
	Position       float64              `json:"position"`
	Duration       float64              `json:"duration"`
	Volume         float64              `json:"volume"`
	Muted          bool                 `json:"muted"`
	Queue          []MediaItem          `json:"queue,omitempty"`
	CurrentIndex   int                  `json:"currentIndex"`
	ShuffleEnabled bool                 `json:"shuffleEnabled"`
	RepeatMode     RepeatMode           `json:"repeatMode"`
}

// PlaybackSnapshot is global playback state, independent of the active
// workspace. At most one session can be identified by AudibleSessionID.
type PlaybackSnapshot struct {
	Version             uint64                   `json:"version"`
	AudibleSessionID    string                   `json:"audibleSessionId,omitempty"`
	Active              *PlaybackSessionSnapshot `json:"active,omitempty"`
	SuspendedPersistent *PlaybackSessionSnapshot `json:"suspendedPersistent,omitempty"`
}

var (
	ErrPlaybackBackendNotFound     = errors.New("playback backend not found")
	ErrPlaybackUnsupported         = errors.New("playback is unsupported")
	ErrPlaybackInvalidMedia        = errors.New("invalid media item")
	ErrPlaybackSessionNotActive    = errors.New("playback session is not active")
	ErrPlaybackRollbackUnavailable = errors.New("playback rollback is unavailable")
)

// PlaybackUnsupportedError keeps the provider and capability reason available
// while still supporting errors.Is(err, ErrPlaybackUnsupported).
type PlaybackUnsupportedError struct {
	Provider PlaybackProvider
	Reason   string
}

func (err *PlaybackUnsupportedError) Error() string {
	reason := strings.TrimSpace(err.Reason)
	if reason == "" {
		reason = "backend is unavailable"
	}
	if err.Provider == "" {
		return fmt.Sprintf("%v: %s", ErrPlaybackUnsupported, reason)
	}
	return fmt.Sprintf("%v for %s: %s", ErrPlaybackUnsupported, err.Provider, reason)
}

func (err *PlaybackUnsupportedError) Unwrap() error {
	return ErrPlaybackUnsupported
}

func normalizeMediaItem(item MediaItem) (MediaItem, error) {
	item.ID = strings.TrimSpace(item.ID)
	item.Source.ID = strings.TrimSpace(item.Source.ID)
	item.Source.URI = strings.TrimSpace(item.Source.URI)
	item.Title = strings.TrimSpace(item.Title)
	item.Artist = strings.TrimSpace(item.Artist)
	if item.ID == "" {
		item.ID = item.Source.ID
	}
	if item.ID == "" {
		item.ID = item.Source.URI
	}
	if item.Source.Provider == "" {
		return MediaItem{}, fmt.Errorf("%w: playback provider is required", ErrPlaybackInvalidMedia)
	}
	if item.Source.ID == "" && item.Source.URI == "" {
		return MediaItem{}, fmt.Errorf("%w: source id or uri is required", ErrPlaybackInvalidMedia)
	}
	if item.Kind != MediaKindAudio && item.Kind != MediaKindVideo {
		return MediaItem{}, fmt.Errorf("%w: unsupported media kind %q", ErrPlaybackInvalidMedia, item.Kind)
	}
	if item.Title == "" {
		item.Title = item.ID
	}
	if item.Duration < 0 {
		item.Duration = 0
	}
	return item, nil
}

func playbackStateMayBeAudible(state PlaybackState) bool {
	return state == PlaybackStateLoading || state == PlaybackStatePlaying || state == PlaybackStateBuffering
}
