package events

import "testing"

func TestGatewayEventEnvelopeDefaults(t *testing.T) {
	env := NewGatewayEventEnvelope("queue.updated", "queue")
	if env.EventID == "" {
		t.Fatalf("expected event id")
	}
	if env.Topic != "queue.updated" {
		t.Fatalf("unexpected topic: %s", env.Topic)
	}
	if env.Timestamp.IsZero() {
		t.Fatalf("expected timestamp")
	}
}
