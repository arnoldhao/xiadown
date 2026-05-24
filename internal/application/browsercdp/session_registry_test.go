package browsercdp

import (
	"testing"

	targetpkg "github.com/chromedp/cdproto/target"
)

func TestSessionRegistryCloseSessionKeyRemovesSessions(t *testing.T) {
	t.Parallel()

	registry := NewSessionRegistry()
	first := registry.GetOrCreate("session-a", "xiadown", SessionOptions{})
	second := registry.GetOrCreate("session-a", "work", SessionOptions{})
	other := registry.GetOrCreate("session-b", "xiadown", SessionOptions{})

	if first == nil || second == nil || other == nil {
		t.Fatalf("expected sessions to be created")
	}

	registry.CloseSessionKey("session-a")

	registry.mu.Lock()
	_, existsA := registry.sessions["session-a"]
	bucketB := registry.sessions["session-b"]
	registry.mu.Unlock()

	if existsA {
		t.Fatalf("expected session-a bucket to be removed")
	}
	if len(bucketB) != 1 {
		t.Fatalf("expected session-b bucket to remain intact, got %#v", bucketB)
	}

	recreated := registry.GetOrCreate("session-a", "xiadown", SessionOptions{})
	if recreated == nil {
		t.Fatalf("expected recreated session")
	}
	if recreated == first {
		t.Fatalf("expected session-a to be recreated after cleanup")
	}
}

func TestSessionRegistryCloseAllRemovesAllSessions(t *testing.T) {
	t.Parallel()

	registry := NewSessionRegistry()
	first := registry.GetOrCreate("session-a", "xiadown", SessionOptions{})
	second := registry.GetOrCreate("session-b", "work", SessionOptions{})

	if first == nil || second == nil {
		t.Fatalf("expected sessions to be created")
	}

	registry.CloseAll()

	registry.mu.Lock()
	sessionCount := len(registry.sessions)
	registry.mu.Unlock()

	if sessionCount != 0 {
		t.Fatalf("expected all sessions to be removed, got %d buckets", sessionCount)
	}

	recreated := registry.GetOrCreate("session-a", "xiadown", SessionOptions{})
	if recreated == nil {
		t.Fatalf("expected recreated session")
	}
	if recreated == first {
		t.Fatalf("expected session to be recreated after close all")
	}
}

func TestSessionTargetDestroyedDetachesManagedTab(t *testing.T) {
	t.Parallel()

	canceled := false
	session := &Session{
		tabs: map[string]*sessionTab{
			"tab-1": {
				TargetID: "tab-1",
				cancel: func() {
					canceled = true
				},
			},
		},
		activeTarget:   "tab-1",
		pendingDialogs: map[string]PendingDialog{"tab-1": {Message: "confirm"}},
		targetInfos: map[string]*targetpkg.Info{
			"tab-1": {TargetID: "tab-1", Type: "page", URL: "https://example.test/"},
		},
	}

	session.handleBrowserTargetGone("tab-1")

	if !canceled {
		t.Fatal("expected closed target context to be canceled")
	}
	if _, ok := session.tabs["tab-1"]; ok {
		t.Fatal("expected closed target to be removed from tabs")
	}
	if _, ok := session.pendingDialogs["tab-1"]; ok {
		t.Fatal("expected pending dialog for closed target to be removed")
	}
	if _, ok := session.targetInfos["tab-1"]; ok {
		t.Fatal("expected target metadata for closed target to be removed")
	}
	if session.activeTarget != "" {
		t.Fatalf("expected active target to be cleared, got %q", session.activeTarget)
	}
}

func TestSessionTargetInfoChangedRefreshesManagedTabMetadata(t *testing.T) {
	t.Parallel()

	session := &Session{
		tabs: map[string]*sessionTab{
			"tab-1": {TargetID: "tab-1"},
		},
		targetInfos: map[string]*targetpkg.Info{},
	}

	session.handleBrowserTargetInfo(&targetpkg.Info{
		TargetID: "tab-1",
		Type:     "page",
		URL:      "https://example.test/current",
		Title:    "Current Page",
	})

	tab := session.tabs["tab-1"]
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	if tab.lastURL != "https://example.test/current" {
		t.Fatalf("expected tab url to refresh, got %q", tab.lastURL)
	}
	if tab.title != "Current Page" {
		t.Fatalf("expected tab title to refresh, got %q", tab.title)
	}
}
