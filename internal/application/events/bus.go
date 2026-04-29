package events

import (
	"context"
	"sync"
	"time"
)

// Event is the payload delivered through the internal event bus and over WebSocket.
type Event struct {
	ID        string      `json:"id"`
	Topic     string      `json:"topic"`
	Type      string      `json:"type,omitempty"`
	Seq       int64       `json:"seq,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp time.Time   `json:"ts"`
}

type Handler func(Event)

type Bus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(topic string, handler Handler) func()
}

// ReplayBus provides best-effort event replay for reconnect recovery.
type ReplayBus interface {
	Replay(topic string, afterSeq int64, limit int) (events []Event, gap bool)
}

// InMemoryBus is a simple pub/sub implementation; keep it small and synchronous for now.
type InMemoryBus struct {
	mu           sync.RWMutex
	subscribers  map[string]map[string]Handler
	subscription int64
	sequence     int64
	history      map[string][]Event
}

const defaultReplayHistoryLimit = 512

func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		subscribers: make(map[string]map[string]Handler),
		history:     make(map[string][]Event),
	}
}

func (bus *InMemoryBus) Publish(_ context.Context, event Event) error {
	if event.Topic == "" {
		return nil
	}
	if event.ID == "" {
		event.ID = time.Now().Format(time.RFC3339Nano)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	bus.mu.Lock()
	if event.Seq <= 0 {
		bus.sequence++
		event.Seq = bus.sequence
	}
	bus.appendHistoryLocked(event)
	topicHandlers := bus.subscribers[event.Topic]
	handlers := make([]Handler, 0, len(topicHandlers))
	for _, handler := range topicHandlers {
		handlers = append(handlers, handler)
	}
	bus.mu.Unlock()

	for _, handler := range handlers {
		handler(event)
	}

	return nil
}

func (bus *InMemoryBus) Replay(topic string, afterSeq int64, limit int) ([]Event, bool) {
	if topic == "" {
		return nil, false
	}
	if limit <= 0 {
		limit = defaultReplayHistoryLimit
	}
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	history := bus.history[topic]
	if len(history) == 0 {
		return nil, false
	}
	firstSeq := history[0].Seq
	gap := afterSeq > 0 && firstSeq > afterSeq+1
	result := make([]Event, 0, limit)
	for _, event := range history {
		if event.Seq <= afterSeq {
			continue
		}
		result = append(result, event)
		if len(result) >= limit {
			break
		}
	}
	return result, gap
}

func (bus *InMemoryBus) appendHistoryLocked(event Event) {
	if event.Topic == "" {
		return
	}
	history := append(bus.history[event.Topic], event)
	if len(history) > defaultReplayHistoryLimit {
		history = history[len(history)-defaultReplayHistoryLimit:]
	}
	bus.history[event.Topic] = history
}

func (bus *InMemoryBus) Subscribe(topic string, handler Handler) func() {
	if topic == "" || handler == nil {
		return func() {}
	}

	bus.mu.Lock()
	defer bus.mu.Unlock()

	if bus.subscribers[topic] == nil {
		bus.subscribers[topic] = make(map[string]Handler)
	}
	bus.subscription++
	id := topic + ":" + time.Now().Format(time.RFC3339Nano)
	bus.subscribers[topic][id] = handler

	return func() {
		bus.mu.Lock()
		defer bus.mu.Unlock()
		delete(bus.subscribers[topic], id)
		if len(bus.subscribers[topic]) == 0 {
			delete(bus.subscribers, topic)
		}
	}
}
