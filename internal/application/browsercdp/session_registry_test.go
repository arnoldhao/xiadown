package browsercdp

import "testing"

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
