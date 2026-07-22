package library

import "testing"

func TestHistoryRecordSeparatesOperationSnapshotsFromLifecycleEvents(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"operation_renamed", "operation_canceled", "operation_resumed", "operation_delete_requested", "operation_deleted"} {
		action := action
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			if _, err := NewHistoryRecord(HistoryRecordParams{
				ID: "history-" + action, LibraryID: "library-1", Category: "operation_event",
				Action: action, DisplayName: "Task", Status: "succeeded",
			}); err != nil {
				t.Fatalf("expected operation lifecycle action %q to be accepted: %v", action, err)
			}
		})
	}

	invalidPairs := []struct {
		category string
		action   string
	}{
		{category: "operation", action: "operation_deleted"},
		{category: "operation_event", action: "download"},
	}
	for _, pair := range invalidPairs {
		if _, err := NewHistoryRecord(HistoryRecordParams{
			ID: "invalid-history", LibraryID: "library-1", Category: pair.category,
			Action: pair.action, DisplayName: "Task", Status: "succeeded",
		}); err != ErrInvalidHistoryRecord {
			t.Fatalf("expected category/action pair %#v to be rejected, got %v", pair, err)
		}
	}
}
