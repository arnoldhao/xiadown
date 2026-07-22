package wails

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"xiadown/internal/application/listenplayback"
)

type listenLivePlaybackPlayer interface {
	StartPlaybackSession(listenplayback.PlaybackProvider, string, ListenPlayerPlayRequest) error
	ResumePlaybackSession(listenplayback.PlaybackProvider, string) error
	PausePlaybackSession(listenplayback.PlaybackProvider, string) error
	ResetPlaybackSession(listenplayback.PlaybackProvider, string) error
	SeekPlaybackSession(listenplayback.PlaybackProvider, string, float64) error
	SetPlaybackSessionVolume(listenplayback.PlaybackProvider, string, float64, bool) error
	SubscribePlaybackEvents(listenplayback.PlaybackBackendEventListener) func()
}

// ListenLivePlayerBackend adapts the shared YouTube iframe player to either
// the stream (Hush) or YouTube provider. The provider is fixed per adapter,
// while events carry provider/session identity so two adapters can safely
// share one native player instance.
type ListenLivePlayerBackend struct {
	provider  listenplayback.PlaybackProvider
	player    listenLivePlaybackPlayer
	mu        sync.RWMutex
	sessionID string
}

func NewListenLivePlayerBackend(
	provider listenplayback.PlaybackProvider,
	player listenLivePlaybackPlayer,
) *ListenLivePlayerBackend {
	return &ListenLivePlayerBackend{provider: provider, player: player}
}

var _ listenplayback.PlaybackBackend = (*ListenLivePlayerBackend)(nil)

func (backend *ListenLivePlayerBackend) Provider() listenplayback.PlaybackProvider {
	if backend == nil {
		return ""
	}
	return backend.provider
}

func (backend *ListenLivePlayerBackend) Capabilities() listenplayback.PlaybackCapabilities {
	available := backend != nil && backend.player != nil &&
		(backend.provider == listenplayback.PlaybackProviderStream || backend.provider == listenplayback.PlaybackProviderYouTube)
	reason := ""
	if !available {
		reason = "YouTube live player is unavailable"
	}
	kinds := []listenplayback.MediaKind{listenplayback.MediaKindVideo}
	if backend != nil && backend.provider == listenplayback.PlaybackProviderStream {
		kinds = []listenplayback.MediaKind{listenplayback.MediaKindAudio, listenplayback.MediaKindVideo}
	}
	return listenplayback.PlaybackCapabilities{
		Available:         available,
		UnsupportedReason: reason,
		MediaKinds:        kinds,
		PlayPause:         available,
		Stop:              available,
		Seek:              available,
		Volume:            available,
		Video:             available,
		Fullscreen:        available,
	}
}

func (backend *ListenLivePlayerBackend) Start(ctx context.Context, request listenplayback.PlaybackStartRequest) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	if err := playbackContextError(ctx); err != nil {
		return err
	}
	if request.Item.Source.Provider != backend.provider {
		return fmt.Errorf("live player backend %q cannot play provider %q", backend.provider, request.Item.Source.Provider)
	}
	videoID := strings.TrimSpace(request.Item.Source.ID)
	if !listenYouTubeVideoIDPattern.MatchString(videoID) {
		return fmt.Errorf("live player media item requires a valid YouTube video id")
	}
	backend.setSessionID(request.SessionID)
	err := backend.player.StartPlaybackSession(backend.provider, request.SessionID, ListenPlayerPlayRequest{
		VideoID:      videoID,
		Title:        request.Item.Title,
		Artist:       request.Item.Artist,
		Language:     strings.TrimSpace(request.Item.Metadata[youtubeWorkspacePlaybackLanguageMetadataKey]),
		StartSeconds: request.StartSeconds,
		ForceReload:  request.ForceReload,
		Volume:       request.Volume,
		Muted:        request.Muted,
	})
	if err != nil {
		backend.clearSessionID(request.SessionID)
	}
	return err
}

func (backend *ListenLivePlayerBackend) Play(ctx context.Context) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	if err := playbackContextError(ctx); err != nil {
		return err
	}
	return backend.player.ResumePlaybackSession(backend.provider, backend.currentSessionID())
}

func (backend *ListenLivePlayerBackend) Pause(ctx context.Context) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	if err := playbackContextError(ctx); err != nil {
		return err
	}
	return backend.player.PausePlaybackSession(backend.provider, backend.currentSessionID())
}

func (backend *ListenLivePlayerBackend) Stop(ctx context.Context) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	if err := playbackContextError(ctx); err != nil {
		return err
	}
	sessionID := backend.currentSessionID()
	err := backend.player.ResetPlaybackSession(backend.provider, sessionID)
	if err == nil {
		backend.clearSessionID(sessionID)
	}
	return err
}

func (backend *ListenLivePlayerBackend) Seek(ctx context.Context, seconds float64) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	if err := playbackContextError(ctx); err != nil {
		return err
	}
	if seconds < 0 {
		seconds = 0
	}
	return backend.player.SeekPlaybackSession(backend.provider, backend.currentSessionID(), seconds)
}

func (backend *ListenLivePlayerBackend) SetVolume(ctx context.Context, volume float64, muted bool) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	if err := playbackContextError(ctx); err != nil {
		return err
	}
	if volume < 0 {
		volume = 0
	} else if volume > 1 {
		volume = 1
	}
	return backend.player.SetPlaybackSessionVolume(
		backend.provider,
		backend.currentSessionID(),
		volume,
		muted || volume <= 0,
	)
}

func (backend *ListenLivePlayerBackend) Previous(context.Context) error {
	return &listenplayback.PlaybackUnsupportedError{Provider: backend.Provider(), Reason: "previous is unavailable for the live player"}
}

func (backend *ListenLivePlayerBackend) Next(context.Context) error {
	return &listenplayback.PlaybackUnsupportedError{Provider: backend.Provider(), Reason: "next is unavailable for the live player"}
}

// Subscribe asynchronously relays only this adapter's provider events. The
// queue breaks synchronous player callbacks away from coordinator commands,
// avoiding op-lock re-entry during pause/start handoffs.
func (backend *ListenLivePlayerBackend) Subscribe(listener listenplayback.PlaybackBackendEventListener) func() {
	if backend == nil || backend.player == nil || listener == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan listenplayback.PlaybackBackendEvent, 8)
	unsubscribePlayer := backend.player.SubscribePlaybackEvents(func(event listenplayback.PlaybackBackendEvent) {
		if event.Provider != backend.provider || event.SessionID == "" || event.SessionID != backend.currentSessionID() {
			return
		}
		select {
		case events <- event:
		default:
			select {
			case <-events:
			default:
			}
			select {
			case events <- event:
			default:
			}
		}
	})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-events:
				listener(event)
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			unsubscribePlayer()
			cancel()
		})
	}
}

func (backend *ListenLivePlayerBackend) currentSessionID() string {
	if backend == nil {
		return ""
	}
	backend.mu.RLock()
	defer backend.mu.RUnlock()
	return backend.sessionID
}

func (backend *ListenLivePlayerBackend) setSessionID(sessionID string) {
	backend.mu.Lock()
	backend.sessionID = strings.TrimSpace(sessionID)
	backend.mu.Unlock()
}

func (backend *ListenLivePlayerBackend) clearSessionID(sessionID string) {
	backend.mu.Lock()
	if backend.sessionID == strings.TrimSpace(sessionID) {
		backend.sessionID = ""
	}
	backend.mu.Unlock()
}

func (backend *ListenLivePlayerBackend) ensureAvailable() error {
	capabilities := backend.Capabilities()
	if capabilities.Available {
		return nil
	}
	return &listenplayback.PlaybackUnsupportedError{
		Provider: backend.Provider(),
		Reason:   capabilities.UnsupportedReason,
	}
}

func playbackContextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
