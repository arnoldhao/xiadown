//go:build darwin && cgo && !ios

package libraryrootsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDarwinNativeWatcherEmitsFileEvent(t *testing.T) {
	root := t.TempDir()
	watcher := platformNativeWatcher()
	if !watcher.Available() || !watcher.SupportsReplay() {
		t.Fatal("Darwin watcher must support journal replay")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan watchEvent, 32)
	done := make(chan error, 1)
	go func() {
		done <- watcher.Watch(ctx, root, 0, func(event watchEvent) {
			events <- event
		})
	}()

	select {
	case event := <-events:
		if !event.checkpoint || event.cursor == 0 {
			t.Fatalf("first event is not a journal checkpoint: %#v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FSEvents checkpoint timed out")
	}
	target := filepath.Join(root, "created.mp4")
	if err := os.WriteFile(target, []byte("event"), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(8 * time.Second)
	for {
		select {
		case event := <-events:
			relative, relativeErr := filepath.Rel(root, event.path)
			if relativeErr != nil ||
				relative == ".." ||
				strings.HasPrefix(
					relative,
					".."+string(filepath.Separator),
				) {
				if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
					relative, relativeErr = filepath.Rel(
						resolvedRoot,
						event.path,
					)
				}
			}
			if !event.checkpoint && relativeErr == nil &&
				filepath.Clean(relative) == "created.mp4" {
				cancel()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("FSEvents watcher did not stop")
				}
				return
			}
		case <-deadline:
			t.Fatal("FSEvents file event timed out")
		}
	}
}
