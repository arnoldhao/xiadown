package wails

import (
	"strings"
	"testing"

	"xiadown/internal/application/listenplayback"
)

func TestNativeLocalMediaReadyActionIsIdempotentForDispatchedSession(t *testing.T) {
	transport := &NativeLocalMediaWebviewTransport{
		current: listenplayback.NativeLocalMediaRequest{
			SessionID:    "local-1",
			URI:          "http://127.0.0.1/media.mp3",
			Kind:         listenplayback.MediaKindAudio,
			StartSeconds: 12,
			Volume:       0.4,
		},
		desiredPlaying: true,
	}

	action, command := transport.readyActionLocked()
	if action != localMediaReadyStart || command.SessionID != "local-1" || !command.Autoplay {
		t.Fatalf("first ready = (%d, %#v), want start", action, command)
	}
	action, command = transport.readyActionLocked()
	if action != localMediaReadyNoop || command != (localMediaStartCommand{}) {
		t.Fatalf("duplicate ready = (%d, %#v), want no-op", action, command)
	}

	transport.current = listenplayback.NativeLocalMediaRequest{}
	action, _ = transport.readyActionLocked()
	if action != localMediaReadyStop {
		t.Fatalf("empty ready action = %d, want stop", action)
	}
}

func TestNativeLocalMediaBridgeAttemptsAutoplayAndReportsRejection(t *testing.T) {
	for _, fragment := range []string{
		`if (request.autoplay !== false) play();`,
		`pending.catch((error) => snapshot("error"`,
	} {
		if !strings.Contains(localMediaBridgeScript, fragment) {
			t.Fatalf("local media bridge is missing autoplay contract %q", fragment)
		}
	}
}
