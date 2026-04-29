package events

import (
	"context"
	"testing"
)

func TestInMemoryBusReplay(t *testing.T) {
	t.Parallel()

	bus := NewInMemoryBus()
	for i := 0; i < 5; i++ {
		if err := bus.Publish(context.Background(), Event{
			Topic: "library.operation",
			Type:  "operation-updated",
			Payload: map[string]any{
				"index": i,
			},
		}); err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	events, gap := bus.Replay("library.operation", 2, 10)
	if gap {
		t.Fatalf("unexpected replay gap")
	}
	if len(events) != 3 {
		t.Fatalf("unexpected replay size: got %d want %d", len(events), 3)
	}
	if events[0].Seq != 3 || events[2].Seq != 5 {
		t.Fatalf("unexpected replay seq range: got [%d..%d]", events[0].Seq, events[2].Seq)
	}
}

func TestInMemoryBusReplayGap(t *testing.T) {
	t.Parallel()

	bus := NewInMemoryBus()
	total := defaultReplayHistoryLimit + 32
	for i := 0; i < total; i++ {
		if err := bus.Publish(context.Background(), Event{
			Topic: "library.operation",
			Type:  "operation-updated",
			Payload: map[string]any{
				"index": i,
			},
		}); err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	events, gap := bus.Replay("library.operation", 1, 10)
	if !gap {
		t.Fatalf("expected replay gap when cursor is older than retained history")
	}
	if len(events) == 0 {
		t.Fatalf("expected replay to still provide available tail events")
	}
	if events[0].Seq <= 1 {
		t.Fatalf("replay tail should be newer than cursor: got %d", events[0].Seq)
	}
}
