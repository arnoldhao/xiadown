package events

import "time"

type GatewayEventEnvelope struct {
	EventID    string    `json:"eventId"`
	Type       string    `json:"type"`
	Topic      string    `json:"topic"`
	SessionID  string    `json:"sessionId,omitempty"`
	SessionKey string    `json:"sessionKey,omitempty"`
	RunID      string    `json:"runId,omitempty"`
	Sequence   int64     `json:"sequence"`
	Timestamp  time.Time `json:"timestamp"`
}

func NewGatewayEventEnvelope(topic, eventType string) GatewayEventEnvelope {
	return GatewayEventEnvelope{
		EventID:   time.Now().Format(time.RFC3339Nano),
		Type:      eventType,
		Topic:     topic,
		Timestamp: time.Now(),
	}
}
