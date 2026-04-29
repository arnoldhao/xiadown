package browsercdp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitOnTabTimeRespectsParentCancel(t *testing.T) {
	session := &Session{}
	parent, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	startedAt := time.Now()
	err := session.waitOnTab(parent, nil, WaitRequest{Time: 2 * time.Second}, 2*time.Second)
	elapsed := time.Since(startedAt)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if elapsed >= 500*time.Millisecond {
		t.Fatalf("expected wait to stop early, elapsed=%s", elapsed)
	}
}

func TestSanitizeLogURLRemovesSensitiveParts(t *testing.T) {
	rawURL := "https://user:secret@example.com/path?q=token#frag"
	sanitized := sanitizeLogURL(rawURL)
	if sanitized != "https://example.com/path" {
		t.Fatalf("unexpected sanitized url %q", sanitized)
	}
}
