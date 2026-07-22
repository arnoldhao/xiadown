package wails

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"xiadown/internal/application/listenplayback"
)

type handlerPlaybackBackend struct {
	mu    sync.Mutex
	calls []string
}

func (backend *handlerPlaybackBackend) Provider() listenplayback.PlaybackProvider {
	return listenplayback.PlaybackProviderLocal
}

func (backend *handlerPlaybackBackend) Capabilities() listenplayback.PlaybackCapabilities {
	return listenplayback.PlaybackCapabilities{
		Available:  true,
		MediaKinds: []listenplayback.MediaKind{listenplayback.MediaKindAudio},
		PlayPause:  true,
		Stop:       true,
		Seek:       true,
		Previous:   true,
		Next:       true,
		Volume:     true,
	}
}

func (backend *handlerPlaybackBackend) Start(_ context.Context, request listenplayback.PlaybackStartRequest) error {
	return backend.record("start:" + request.SessionID)
}

func (backend *handlerPlaybackBackend) Play(context.Context) error  { return backend.record("play") }
func (backend *handlerPlaybackBackend) Pause(context.Context) error { return backend.record("pause") }
func (backend *handlerPlaybackBackend) Stop(context.Context) error  { return backend.record("stop") }
func (backend *handlerPlaybackBackend) Seek(_ context.Context, seconds float64) error {
	return backend.record(fmt.Sprintf("seek:%g", seconds))
}
func (backend *handlerPlaybackBackend) SetVolume(_ context.Context, volume float64, muted bool) error {
	return backend.record(fmt.Sprintf("volume:%g:%t", volume, muted))
}
func (backend *handlerPlaybackBackend) Previous(context.Context) error {
	return backend.record("previous")
}
func (backend *handlerPlaybackBackend) Next(context.Context) error { return backend.record("next") }

func (backend *handlerPlaybackBackend) record(call string) error {
	backend.mu.Lock()
	backend.calls = append(backend.calls, call)
	backend.mu.Unlock()
	return nil
}

func TestPlaybackCoordinatorHandlerExposesGlobalCommands(t *testing.T) {
	backend := &handlerPlaybackBackend{}
	coordinator, err := listenplayback.NewPlaybackCoordinator(backend)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	handler := NewPlaybackCoordinatorHandler(nil, coordinator)
	defer ShutdownPlaybackCoordinatorHandler(handler)
	ctx := context.Background()

	started, err := handler.StartPersistent(ctx, listenplayback.PlaybackSessionRequest{
		SessionID: "station",
		Focus:     listenplayback.PlaybackFocusTransientPreview,
		Item: listenplayback.MediaItem{
			ID:     "song",
			Kind:   listenplayback.MediaKindAudio,
			Source: listenplayback.PlaybackSource{Provider: listenplayback.PlaybackProviderLocal, URI: "/tmp/song.mp3"},
			Title:  "Song",
		},
	})
	if err != nil {
		t.Fatalf("StartPersistent: %v", err)
	}
	if started.Active == nil || started.Active.Focus != listenplayback.PlaybackFocusPersistent {
		t.Fatalf("started snapshot = %+v", started)
	}
	if _, err := handler.Pause(ctx); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if _, err := handler.Play(ctx); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if _, err := handler.Seek(ctx, PlaybackCoordinatorSeekRequest{Seconds: 9}); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if _, err := handler.SetVolume(ctx, PlaybackCoordinatorVolumeRequest{Volume: .6}); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if _, err := handler.Next(ctx); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if _, err := handler.Previous(ctx); err != nil {
		t.Fatalf("Previous: %v", err)
	}
	if _, err := handler.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	snapshot, err := handler.Snapshot(ctx)
	if err != nil || snapshot.Active == nil || snapshot.Active.State != listenplayback.PlaybackStateEnded {
		t.Fatalf("Snapshot = %+v, %v", snapshot, err)
	}
	backend.mu.Lock()
	calls := append([]string(nil), backend.calls...)
	backend.mu.Unlock()
	wantCalls := []string{"start:station", "pause", "play", "seek:9", "volume:0.6:false", "next", "previous", "stop"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestResolveNativeLocalMediaURI(t *testing.T) {
	const base = "http://127.0.0.1:1234/_xiadown/token"
	remote, err := resolveNativeLocalMediaURI(base, "https://cdn.example.test/media.mp3")
	if err != nil || remote != "https://cdn.example.test/media.mp3" {
		t.Fatalf("remote URI = %q, %v", remote, err)
	}
	local, err := resolveNativeLocalMediaURI(base, "/Users/demo/My Song.mp3")
	if err != nil {
		t.Fatalf("local URI: %v", err)
	}
	parsed, err := url.Parse(local)
	if err != nil || !strings.HasSuffix(parsed.Path, "/api/library/asset/local-media") || parsed.Query().Get("path") != "/Users/demo/My Song.mp3" {
		t.Fatalf("resolved local URI = %q (%v)", local, err)
	}
	fileURI, err := resolveNativeLocalMediaURI(base, "file:///Users/demo/My%20Song.mp3")
	if err != nil {
		t.Fatalf("file URI: %v", err)
	}
	parsed, _ = url.Parse(fileURI)
	if parsed.Query().Get("path") != "/Users/demo/My Song.mp3" {
		t.Fatalf("resolved file URI = %q", fileURI)
	}
	if _, err := resolveNativeLocalMediaURI(base, "ftp://example.test/media.mp3"); err == nil {
		t.Fatal("unsupported URI scheme was accepted")
	}
}
