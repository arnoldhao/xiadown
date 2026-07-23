package listenplayback

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

type fakeNativeLocalMediaTransport struct {
	mu         sync.Mutex
	available  bool
	reason     string
	requests   []NativeLocalMediaRequest
	calls      []string
	listeners  map[uint64]PlaybackBackendEventListener
	nextID     uint64
	closed     bool
	commandErr error
}

func newFakeNativeLocalMediaTransport() *fakeNativeLocalMediaTransport {
	return &fakeNativeLocalMediaTransport{
		available: true,
		listeners: make(map[uint64]PlaybackBackendEventListener),
	}
}

func (transport *fakeNativeLocalMediaTransport) Availability() (bool, string) {
	return transport.available, transport.reason
}

func (transport *fakeNativeLocalMediaTransport) Start(_ context.Context, request NativeLocalMediaRequest) error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.requests = append(transport.requests, request)
	transport.calls = append(transport.calls, "start")
	return transport.commandErr
}

func (transport *fakeNativeLocalMediaTransport) Play(context.Context) error {
	return transport.record("play")
}

func (transport *fakeNativeLocalMediaTransport) Pause(context.Context) error {
	return transport.record("pause")
}

func (transport *fakeNativeLocalMediaTransport) Stop(context.Context) error {
	return transport.record("stop")
}

func (transport *fakeNativeLocalMediaTransport) Seek(context.Context, float64) error {
	return transport.record("seek")
}

func (transport *fakeNativeLocalMediaTransport) SetVolume(context.Context, float64, bool) error {
	return transport.record("volume")
}

func (transport *fakeNativeLocalMediaTransport) Subscribe(listener PlaybackBackendEventListener) func() {
	transport.mu.Lock()
	transport.nextID++
	id := transport.nextID
	transport.listeners[id] = listener
	transport.mu.Unlock()
	return func() {
		transport.mu.Lock()
		delete(transport.listeners, id)
		transport.mu.Unlock()
	}
}

func (transport *fakeNativeLocalMediaTransport) Close() error {
	transport.mu.Lock()
	transport.closed = true
	transport.mu.Unlock()
	return nil
}

func (transport *fakeNativeLocalMediaTransport) emit(event PlaybackBackendEvent) {
	transport.mu.Lock()
	listeners := make([]PlaybackBackendEventListener, 0, len(transport.listeners))
	for _, listener := range transport.listeners {
		listeners = append(listeners, listener)
	}
	transport.mu.Unlock()
	for _, listener := range listeners {
		listener(event)
	}
}

func (transport *fakeNativeLocalMediaTransport) record(call string) error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.calls = append(transport.calls, call)
	return transport.commandErr
}

func TestNativeLocalMediaBackendMapsCommandsAndCapabilities(t *testing.T) {
	transport := newFakeNativeLocalMediaTransport()
	backend := NewNativeLocalMediaBackend(transport)
	capabilities := backend.Capabilities()
	if !capabilities.Available || !capabilities.PlayPause || !capabilities.Stop || !capabilities.Seek || !capabilities.Volume {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	if capabilities.Next || capabilities.Previous || !capabilities.SupportsKind(MediaKindVideo) {
		t.Fatalf("unexpected local capabilities = %+v", capabilities)
	}

	err := backend.Start(context.Background(), PlaybackStartRequest{
		SessionID: "preview-1",
		Item: MediaItem{
			ID:     "clip",
			Kind:   MediaKindVideo,
			Source: PlaybackSource{Provider: PlaybackProviderLocal, URI: "/tmp/clip.mp4"},
		},
		StartSeconds: -4,
		Volume:       2,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := NativeLocalMediaRequest{
		SessionID: "preview-1",
		URI:       "/tmp/clip.mp4",
		Kind:      MediaKindVideo,
		Volume:    1,
	}
	if len(transport.requests) != 1 || !reflect.DeepEqual(transport.requests[0], want) {
		t.Fatalf("requests = %#v, want %#v", transport.requests, want)
	}

	if err := backend.Play(context.Background()); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if err := backend.Pause(context.Background()); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if err := backend.Seek(context.Background(), 12); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if err := backend.SetVolume(context.Background(), .4, false); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if err := backend.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !reflect.DeepEqual(transport.calls, []string{"start", "play", "pause", "seek", "volume", "stop"}) {
		t.Fatalf("calls = %#v", transport.calls)
	}
}

func TestNativeLocalMediaBackendRelaysProviderScopedEvents(t *testing.T) {
	transport := newFakeNativeLocalMediaTransport()
	backend := NewNativeLocalMediaBackend(transport)
	received := make(chan PlaybackBackendEvent, 1)
	unsubscribe := backend.Subscribe(func(event PlaybackBackendEvent) { received <- event })
	transport.emit(PlaybackBackendEvent{
		Provider:  PlaybackProviderStream,
		SessionID: "local-1",
		State:     PlaybackStatePlaying,
	})
	event := <-received
	if event.Provider != PlaybackProviderLocal || event.SessionID != "local-1" {
		t.Fatalf("event = %+v", event)
	}
	unsubscribe()
	if err := backend.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !transport.closed {
		t.Fatal("transport was not closed")
	}
}

func TestNativeLocalMediaBackendExplicitlyReportsUnavailableTransport(t *testing.T) {
	transport := newFakeNativeLocalMediaTransport()
	transport.available = false
	transport.reason = "decoder unavailable"
	backend := NewNativeLocalMediaBackend(transport)
	if backend.LocalMediaSupported() {
		t.Fatal("backend unexpectedly supported")
	}
	err := backend.Play(context.Background())
	if !errors.Is(err, ErrPlaybackUnsupported) || backend.Capabilities().UnsupportedReason != "decoder unavailable" {
		t.Fatalf("error/capabilities = %v / %+v", err, backend.Capabilities())
	}
}
