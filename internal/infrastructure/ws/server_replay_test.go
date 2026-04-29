package ws

import (
	"context"
	"testing"
	"time"

	"golang.org/x/net/websocket"
	"xiadown/internal/application/events"
)

func TestWebSocketServerReplaysEventsByCursor(t *testing.T) {
	t.Parallel()

	bus := events.NewInMemoryBus()
	server := NewServer("127.0.0.1:0", bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Shutdown(context.Background())

	if err := bus.Publish(context.Background(), events.Event{
		Topic: "library.operation",
		Type:  "operation-updated",
		Payload: map[string]any{
			"operationId": "op-1",
		},
	}); err != nil {
		t.Fatalf("publish 1: %v", err)
	}
	history, gap := bus.Replay("library.operation", 0, 1)
	if gap || len(history) != 1 || history[0].Seq == 0 {
		t.Fatalf("expected one replayable event without gap, got gap=%v history=%#v", gap, history)
	}
	afterSeq := history[0].Seq

	if err := bus.Publish(context.Background(), events.Event{
		Topic: "library.operation",
		Type:  "operation-updated",
		Payload: map[string]any{
			"operationId": "op-2",
		},
	}); err != nil {
		t.Fatalf("publish 2: %v", err)
	}

	conn2 := mustDialWS(t, server.URL())
	_ = mustReadWS(t, conn2) // system.hello
	if err := websocket.JSON.Send(conn2, clientMessage{
		Action:  "subscribe",
		Topics:  []string{"library.operation"},
		Cursors: map[string]int64{"library.operation": afterSeq},
	}); err != nil {
		t.Fatalf("subscribe with cursor: %v", err)
	}

	replayed := mustReadWS(t, conn2)
	if !replayed.Replay {
		t.Fatalf("expected replayed event")
	}
	if replayed.Seq <= afterSeq {
		t.Fatalf("unexpected replay seq: got %d after %d", replayed.Seq, afterSeq)
	}
	if replayed.Topic != "library.operation" {
		t.Fatalf("unexpected replay topic: %s", replayed.Topic)
	}
}

func TestWebSocketServerReplayGapEmitsResyncRequired(t *testing.T) {
	t.Parallel()

	bus := events.NewInMemoryBus()
	server := NewServer("127.0.0.1:0", bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Shutdown(context.Background())

	for i := 0; i < 600; i++ {
		if err := bus.Publish(context.Background(), events.Event{
			Topic: "library.operation",
			Type:  "operation-updated",
			Payload: map[string]any{
				"operationId": "op-gap",
				"index":       i,
			},
		}); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	conn := mustDialWS(t, server.URL())
	_ = mustReadWS(t, conn) // system.hello
	if err := websocket.JSON.Send(conn, clientMessage{
		Action:  "subscribe",
		Topics:  []string{"library.operation"},
		Cursors: map[string]int64{"library.operation": 1},
	}); err != nil {
		t.Fatalf("subscribe with stale cursor: %v", err)
	}

	// Replay gap emits a deterministic resync-required event first.
	event := mustReadWS(t, conn)
	if event.Type != "resync-required" {
		t.Fatalf("unexpected event type: %s", event.Type)
	}
	if event.Topic != "library.operation" {
		t.Fatalf("unexpected event topic: %s", event.Topic)
	}
}

func mustDialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, err := websocket.Dial(url, "", "http://localhost/")
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	return conn
}

func mustReadWS(t *testing.T, conn *websocket.Conn) outboundMessage {
	t.Helper()
	type result struct {
		msg outboundMessage
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var message outboundMessage
		err := websocket.JSON.Receive(conn, &message)
		ch <- result{msg: message, err: err}
	}()
	select {
	case received := <-ch:
		if received.err != nil {
			t.Fatalf("read ws message: %v", received.err)
		}
		return received.msg
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for ws message")
	}
	return outboundMessage{}
}
