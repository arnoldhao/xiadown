package ws

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/websocket"
	"xiadown/internal/application/events"
)

type Server struct {
	addr     string
	bus      events.Bus
	httpSrv  *http.Server
	listener net.Listener
	mux      *http.ServeMux
	mu       sync.RWMutex
	started  bool
	handlers map[string]http.Handler
	guardMu  sync.RWMutex
	guard    AccessGuard
}

type AccessGuard func(r *http.Request) (allowed bool, statusCode int, message string)

type clientMessage struct {
	Action  string           `json:"action"`
	Topics  []string         `json:"topics"`
	Cursors map[string]int64 `json:"cursors,omitempty"`
}

type outboundMessage struct {
	ID      string      `json:"id"`
	Topic   string      `json:"topic"`
	Type    string      `json:"type,omitempty"`
	Seq     int64       `json:"seq,omitempty"`
	Replay  bool        `json:"replay,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
	TS      time.Time   `json:"ts"`
}

type client struct {
	conn           *websocket.Conn
	bus            events.Bus
	replayBus      events.ReplayBus
	subscriptions  map[string]func()
	subscriptionsM sync.Mutex
	sendCh         chan outboundMessage
	closed         atomic.Bool
	done           chan struct{}
}

const (
	defaultClientBufferSize = 128
	defaultReplayBatchLimit = 256
)

func NewServer(addr string, bus events.Bus) *Server {
	return &Server{
		addr: strings.TrimSpace(addr),
		bus:  bus,
	}
}

func (server *Server) Handle(path string, handler http.Handler) {
	path = strings.TrimSpace(path)
	if path == "" || handler == nil {
		return
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.handlers == nil {
		server.handlers = make(map[string]http.Handler)
	}
	server.handlers[path] = handler
	if server.started && server.mux != nil {
		server.mux.Handle(path, handler)
	}
}

func (server *Server) SetAccessGuard(guard AccessGuard) {
	if server == nil {
		return
	}
	server.guardMu.Lock()
	server.guard = guard
	server.guardMu.Unlock()
}

func (server *Server) Start(ctx context.Context) error {
	if server.started {
		return nil
	}
	if server.bus == nil {
		return fmt.Errorf("ws server missing bus")
	}

	addr := server.addr
	if addr == "" {
		addr = "127.0.0.1:0"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	server.listener = ln

	mux := http.NewServeMux()
	wsHandler := websocket.Server{
		Handler: websocket.Handler(server.handleWebsocket),
		Handshake: func(config *websocket.Config, req *http.Request) error {
			// Allow all origins; desktop app context is trusted/local.
			return nil
		},
	}
	mux.Handle("/ws", wsHandler)
	server.mu.Lock()
	for path, handler := range server.handlers {
		mux.Handle(path, handler)
	}
	server.mux = mux
	server.mu.Unlock()

	server.httpSrv = &http.Server{
		Handler: server.withAccessGuard(mux),
	}

	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	go func() {
		_ = server.httpSrv.Serve(ln)
	}()

	server.mu.Lock()
	server.started = true
	server.mu.Unlock()

	return nil
}

func (server *Server) withAccessGuard(next http.Handler) http.Handler {
	if server == nil {
		return next
	}
	server.guardMu.RLock()
	guard := server.guard
	server.guardMu.RUnlock()
	if guard == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, statusCode, message := guard(r)
		if allowed {
			next.ServeHTTP(w, r)
			return
		}
		if statusCode == 0 {
			statusCode = http.StatusForbidden
		}
		if message == "" {
			message = http.StatusText(statusCode)
		}
		http.Error(w, message, statusCode)
	})
}

func (server *Server) Shutdown(ctx context.Context) error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if !server.started {
		return nil
	}
	server.started = false
	if server.httpSrv != nil {
		_ = server.httpSrv.Shutdown(ctx)
	}
	if server.listener != nil {
		_ = server.listener.Close()
	}
	return nil
}

// URL returns the public WebSocket endpoint.
func (server *Server) URL() string {
	if server.listener == nil {
		return ""
	}
	return fmt.Sprintf("ws://%s/ws", server.listener.Addr().String())
}

func (server *Server) HTTPURL() string {
	if server.listener == nil {
		return ""
	}
	return fmt.Sprintf("http://%s", server.listener.Addr().String())
}

func (server *Server) handleWebsocket(conn *websocket.Conn) {
	client := &client{
		conn:          conn,
		bus:           server.bus,
		replayBus:     replayBusFrom(server.bus),
		subscriptions: make(map[string]func()),
		sendCh:        make(chan outboundMessage, defaultClientBufferSize),
		done:          make(chan struct{}),
	}
	defer client.close()

	go client.writeLoop()

	_ = websocket.JSON.Send(conn, outboundMessage{
		ID:      "",
		Topic:   "system.hello",
		Payload: map[string]string{"message": "connected"},
		TS:      time.Now(),
	})

	for {
		var msg clientMessage
		if err := websocket.JSON.Receive(conn, &msg); err != nil {
			return
		}

		switch strings.ToLower(msg.Action) {
		case "subscribe":
			client.subscribe(msg.Topics, msg.Cursors)
		case "unsubscribe":
			client.unsubscribe(msg.Topics)
		default:
			// ignore unknown actions
		}
	}
}

func replayBusFrom(bus events.Bus) events.ReplayBus {
	replayBus, _ := bus.(events.ReplayBus)
	return replayBus
}

func (c *client) subscribe(topics []string, cursors map[string]int64) {
	for _, topic := range topics {
		trimmed := strings.TrimSpace(topic)
		if trimmed == "" {
			continue
		}
		afterSeq := int64(0)
		if cursors != nil {
			afterSeq = cursors[trimmed]
		}
		c.subscriptionsM.Lock()
		if _, exists := c.subscriptions[trimmed]; exists {
			c.subscriptionsM.Unlock()
			continue
		}
		unsub := c.bus.Subscribe(trimmed, func(event events.Event) {
			if c.closed.Load() {
				return
			}
			msg := outboundMessage{
				ID:      event.ID,
				Topic:   event.Topic,
				Type:    event.Type,
				Seq:     event.Seq,
				Payload: event.Payload,
				TS:      event.Timestamp,
			}
			if !c.enqueue(msg) {
				zap.L().Debug("ws slow consumer, closing connection",
					zap.String("topic", trimmed),
				)
				_ = c.conn.Close()
			}
		})
		c.subscriptions[trimmed] = unsub
		c.subscriptionsM.Unlock()

		if c.replayBus == nil || afterSeq <= 0 {
			continue
		}
		eventsToReplay, gap := c.replayBus.Replay(trimmed, afterSeq, defaultReplayBatchLimit)
		if gap {
			_ = c.enqueue(outboundMessage{
				ID:     "",
				Topic:  trimmed,
				Type:   "resync-required",
				TS:     time.Now(),
				Replay: true,
				Payload: map[string]interface{}{
					"topic":    trimmed,
					"afterSeq": afterSeq,
				},
			})
			zap.L().Debug("ws replay gap detected",
				zap.String("topic", trimmed),
				zap.Int64("afterSeq", afterSeq),
			)
			continue
		}
		for _, event := range eventsToReplay {
			_ = c.enqueue(outboundMessage{
				ID:      event.ID,
				Topic:   event.Topic,
				Type:    event.Type,
				Seq:     event.Seq,
				Payload: event.Payload,
				TS:      event.Timestamp,
				Replay:  true,
			})
		}
		if len(eventsToReplay) > 0 {
			zap.L().Debug("ws replay delivered",
				zap.String("topic", trimmed),
				zap.Int64("afterSeq", afterSeq),
				zap.Int("count", len(eventsToReplay)),
			)
		}
	}
}

func (c *client) unsubscribe(topics []string) {
	for _, topic := range topics {
		trimmed := strings.TrimSpace(topic)
		if trimmed == "" {
			continue
		}
		c.subscriptionsM.Lock()
		if unsub, exists := c.subscriptions[trimmed]; exists && unsub != nil {
			unsub()
			delete(c.subscriptions, trimmed)
		}
		c.subscriptionsM.Unlock()
	}
}

func (c *client) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.sendCh:
			if err := websocket.JSON.Send(c.conn, msg); err != nil {
				_ = c.conn.Close()
				return
			}
		}
	}
}

func (c *client) enqueue(msg outboundMessage) bool {
	if c.closed.Load() {
		return false
	}
	select {
	case c.sendCh <- msg:
		return true
	default:
		return false
	}
}

func (c *client) close() {
	c.closed.Store(true)
	close(c.done)
	c.subscriptionsM.Lock()
	for _, unsub := range c.subscriptions {
		if unsub != nil {
			unsub()
		}
	}
	c.subscriptions = make(map[string]func())
	c.subscriptionsM.Unlock()

	_ = c.conn.Close()
}
