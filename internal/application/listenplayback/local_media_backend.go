package listenplayback

import (
	"context"
	"fmt"
	"strings"
)

const defaultUnsupportedLocalPlaybackReason = "native local media playback engine is not installed"

// LocalMediaBackend is the extension point for the desktop-owned local audio
// and video transport. LocalMediaSupported distinguishes a configured engine
// from the explicit unsupported fallback used on unavailable platforms.
type LocalMediaBackend interface {
	PlaybackBackend
	LocalMediaSupported() bool
}

// NativeLocalMediaRequest is the transport-neutral command used by the local
// playback backend. The presentation layer is responsible for converting a
// local path into a URI that its media engine can read.
type NativeLocalMediaRequest struct {
	SessionID    string
	URI          string
	Kind         MediaKind
	StartSeconds float64
	Volume       float64
	Muted        bool
}

// PlaybackBackendEvent reports state observed by an asynchronous playback
// engine. SessionID is optional for provider-wide engines, but local media
// transports should include it so late events from a replaced source can be
// ignored safely.
type PlaybackBackendEvent struct {
	Provider  PlaybackProvider
	SessionID string
	State     PlaybackState
	Position  float64
	Duration  float64
	Volume    float64
	Muted     bool
	Error     string
	HasTiming bool
	HasVolume bool
}

type PlaybackBackendEventListener func(PlaybackBackendEvent)

// NativeLocalMediaTransport is implemented by the desktop presentation layer.
// It deliberately has no Wails dependency, which keeps playback focus and
// capability decisions testable in the application package.
type NativeLocalMediaTransport interface {
	Availability() (available bool, reason string)
	Start(context.Context, NativeLocalMediaRequest) error
	Play(context.Context) error
	Pause(context.Context) error
	Stop(context.Context) error
	Seek(context.Context, float64) error
	SetVolume(context.Context, float64, bool) error
	Subscribe(PlaybackBackendEventListener) func()
	Close() error
}

// NativeLocalMediaBackend is the real local provider adapter. Its injected
// transport may be a hidden desktop WebView today and a platform-native media
// engine later without changing coordinator callers.
type NativeLocalMediaBackend struct {
	transport NativeLocalMediaTransport
}

var _ LocalMediaBackend = (*NativeLocalMediaBackend)(nil)

func NewNativeLocalMediaBackend(transport NativeLocalMediaTransport) *NativeLocalMediaBackend {
	return &NativeLocalMediaBackend{transport: transport}
}

func (backend *NativeLocalMediaBackend) Provider() PlaybackProvider {
	return PlaybackProviderLocal
}

func (backend *NativeLocalMediaBackend) Capabilities() PlaybackCapabilities {
	available, reason := backend.availability()
	return PlaybackCapabilities{
		Available:         available,
		UnsupportedReason: reason,
		MediaKinds:        []MediaKind{MediaKindAudio, MediaKindVideo},
		PlayPause:         available,
		Stop:              available,
		Seek:              available,
		Volume:            available,
		// The hidden transport can decode video files, but it intentionally
		// exposes no visible video presentation surface yet.
		Video: false,
	}
}

func (backend *NativeLocalMediaBackend) LocalMediaSupported() bool {
	available, _ := backend.availability()
	return available
}

func (backend *NativeLocalMediaBackend) Start(ctx context.Context, request PlaybackStartRequest) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	if request.Item.Source.Provider != PlaybackProviderLocal {
		return fmt.Errorf("NativeLocalMediaBackend cannot play provider %q", request.Item.Source.Provider)
	}
	uri := strings.TrimSpace(request.Item.Source.URI)
	if uri == "" {
		return fmt.Errorf("local media item requires a source URI")
	}
	return backend.transport.Start(ctx, NativeLocalMediaRequest{
		SessionID:    strings.TrimSpace(request.SessionID),
		URI:          uri,
		Kind:         request.Item.Kind,
		StartSeconds: clampSeconds(request.StartSeconds),
		Volume:       clampVolume(request.Volume),
		Muted:        request.Muted || request.Volume <= 0,
	})
}

func (backend *NativeLocalMediaBackend) Play(ctx context.Context) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	return backend.transport.Play(ctx)
}

func (backend *NativeLocalMediaBackend) Pause(ctx context.Context) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	return backend.transport.Pause(ctx)
}

func (backend *NativeLocalMediaBackend) Stop(ctx context.Context) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	return backend.transport.Stop(ctx)
}

func (backend *NativeLocalMediaBackend) Seek(ctx context.Context, seconds float64) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	return backend.transport.Seek(ctx, clampSeconds(seconds))
}

func (backend *NativeLocalMediaBackend) SetVolume(ctx context.Context, volume float64, muted bool) error {
	if err := backend.ensureAvailable(); err != nil {
		return err
	}
	volume = clampVolume(volume)
	return backend.transport.SetVolume(ctx, volume, muted || volume <= 0)
}

func (backend *NativeLocalMediaBackend) Previous(context.Context) error {
	return &PlaybackUnsupportedError{Provider: PlaybackProviderLocal, Reason: "previous is unavailable for a single local media item"}
}

func (backend *NativeLocalMediaBackend) Next(context.Context) error {
	return &PlaybackUnsupportedError{Provider: PlaybackProviderLocal, Reason: "next is unavailable for a single local media item"}
}

func (backend *NativeLocalMediaBackend) Subscribe(listener PlaybackBackendEventListener) func() {
	if backend == nil || backend.transport == nil || listener == nil {
		return func() {}
	}
	return backend.transport.Subscribe(func(event PlaybackBackendEvent) {
		event.Provider = PlaybackProviderLocal
		listener(event)
	})
}

func (backend *NativeLocalMediaBackend) Close() error {
	if backend == nil || backend.transport == nil {
		return nil
	}
	return backend.transport.Close()
}

func (backend *NativeLocalMediaBackend) availability() (bool, string) {
	if backend == nil || backend.transport == nil {
		return false, defaultUnsupportedLocalPlaybackReason
	}
	available, reason := backend.transport.Availability()
	if !available && strings.TrimSpace(reason) == "" {
		reason = defaultUnsupportedLocalPlaybackReason
	}
	return available, strings.TrimSpace(reason)
}

func (backend *NativeLocalMediaBackend) ensureAvailable() error {
	available, reason := backend.availability()
	if available {
		return nil
	}
	return &PlaybackUnsupportedError{Provider: PlaybackProviderLocal, Reason: reason}
}

// UnsupportedLocalMediaBackend advertises that local playback is intentionally
// unavailable. It keeps callers capability-driven until a native engine lands.
type UnsupportedLocalMediaBackend struct {
	reason string
}

var _ LocalMediaBackend = (*UnsupportedLocalMediaBackend)(nil)

func NewUnsupportedLocalMediaBackend(reason string) *UnsupportedLocalMediaBackend {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = defaultUnsupportedLocalPlaybackReason
	}
	return &UnsupportedLocalMediaBackend{reason: reason}
}

func (backend *UnsupportedLocalMediaBackend) Provider() PlaybackProvider {
	return PlaybackProviderLocal
}

func (backend *UnsupportedLocalMediaBackend) Capabilities() PlaybackCapabilities {
	return PlaybackCapabilities{
		Available:         false,
		UnsupportedReason: backend.unsupportedReason(),
		MediaKinds:        []MediaKind{MediaKindAudio, MediaKindVideo},
	}
}

func (backend *UnsupportedLocalMediaBackend) LocalMediaSupported() bool {
	return false
}

func (backend *UnsupportedLocalMediaBackend) Start(context.Context, PlaybackStartRequest) error {
	return backend.unsupportedError()
}

func (backend *UnsupportedLocalMediaBackend) Play(context.Context) error {
	return backend.unsupportedError()
}

func (backend *UnsupportedLocalMediaBackend) Pause(context.Context) error {
	return backend.unsupportedError()
}

func (backend *UnsupportedLocalMediaBackend) Stop(context.Context) error {
	return backend.unsupportedError()
}

func (backend *UnsupportedLocalMediaBackend) Seek(context.Context, float64) error {
	return backend.unsupportedError()
}

func (backend *UnsupportedLocalMediaBackend) SetVolume(context.Context, float64, bool) error {
	return backend.unsupportedError()
}

func (backend *UnsupportedLocalMediaBackend) Previous(context.Context) error {
	return backend.unsupportedError()
}

func (backend *UnsupportedLocalMediaBackend) Next(context.Context) error {
	return backend.unsupportedError()
}

func (backend *UnsupportedLocalMediaBackend) unsupportedReason() string {
	if backend == nil || strings.TrimSpace(backend.reason) == "" {
		return defaultUnsupportedLocalPlaybackReason
	}
	return backend.reason
}

func (backend *UnsupportedLocalMediaBackend) unsupportedError() error {
	return &PlaybackUnsupportedError{
		Provider: PlaybackProviderLocal,
		Reason:   backend.unsupportedReason(),
	}
}
