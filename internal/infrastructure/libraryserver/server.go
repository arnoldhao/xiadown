// Package libraryserver runs the isolated Library API on two deliberately
// separate transports: a loopback HTTP backend for Tailscale Serve and an
// optional LAN TLS listener. It never receives the desktop app mux.
package libraryserver

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	Handler               http.Handler
	BackendAddress        string
	LANAddress            string
	TLSIdentity           *TLSIdentity
	ReadTimeout           time.Duration
	WriteTimeout          time.Duration
	IdleTimeout           time.Duration
	ShutdownTimeout       time.Duration
	MaxConcurrentRequests int
}

type Server struct {
	config Config

	mu              sync.RWMutex
	backendListener net.Listener
	lanListeners    []net.Listener
	backendServer   *http.Server
	lanServer       *http.Server
	started         bool
	requestSlots    chan struct{}
}

type LANInfo struct {
	Address   string
	Addresses []string
	Port      int
}

func New(config Config) (*Server, error) {
	if config.Handler == nil {
		return nil, errors.New("Library public API server requires an isolated handler")
	}
	if strings.TrimSpace(config.BackendAddress) == "" {
		config.BackendAddress = "127.0.0.1:0"
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 15 * time.Second
	}
	// Metadata/auth/error responses need an absolute write bound. Asset handlers
	// explicitly replace this connection deadline with their sliding deadline on
	// every successful chunk, so healthy multi-gigabyte streams remain unbounded
	// in total duration without permitting an indefinitely stalled write.
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 30 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 60 * time.Second
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = 10 * time.Second
	}
	if config.MaxConcurrentRequests <= 0 {
		config.MaxConcurrentRequests = 96
	}
	if strings.TrimSpace(config.LANAddress) != "" && config.TLSIdentity == nil {
		return nil, errors.New("Library LAN listener requires a TLS identity")
	}
	return &Server{config: config, requestSlots: make(chan struct{}, config.MaxConcurrentRequests)}, nil
}

func (server *Server) Start(ctx context.Context) error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.started {
		return nil
	}
	backendListener, err := net.Listen("tcp", server.config.BackendAddress)
	if err != nil {
		return fmt.Errorf("listen on Library Tailscale backend: %w", err)
	}
	if !isLoopbackListener(backendListener.Addr()) {
		_ = backendListener.Close()
		return errors.New("Library Tailscale backend must listen on loopback")
	}
	backendServer := server.newHTTPServer()

	var lanListener net.Listener
	var lanServer *http.Server
	if strings.TrimSpace(server.config.LANAddress) != "" {
		if err := validateLANBindAddress(server.config.LANAddress); err != nil {
			_ = backendListener.Close()
			return err
		}
		plainListener, listenErr := net.Listen("tcp", server.config.LANAddress)
		if listenErr != nil {
			_ = backendListener.Close()
			return fmt.Errorf("listen on Library LAN TLS address: %w", listenErr)
		}
		tlsConfig := &tls.Config{
			MinVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{server.config.TLSIdentity.Certificate},
		}
		lanListener = tls.NewListener(plainListener, tlsConfig)
		lanServer = server.newHTTPServer()
	}

	server.backendListener = backendListener
	server.backendServer = backendServer
	if lanListener != nil {
		server.lanListeners = []net.Listener{lanListener}
	}
	server.lanServer = lanServer
	server.started = true
	go serve(backendServer, backendListener)
	if lanServer != nil {
		go serve(lanServer, lanListener)
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	return nil
}

func (server *Server) Shutdown(ctx context.Context) error {
	server.mu.Lock()
	if !server.started {
		server.mu.Unlock()
		return nil
	}
	backendServer := server.backendServer
	lanServer := server.lanServer
	backendListener := server.backendListener
	lanListeners := append([]net.Listener(nil), server.lanListeners...)
	server.backendServer = nil
	server.lanServer = nil
	server.backendListener = nil
	server.lanListeners = nil
	server.started = false
	server.mu.Unlock()

	var result error
	if lanServer != nil {
		result = errors.Join(result, shutdownHTTPServer(ctx, lanServer, server.config.ShutdownTimeout))
	}
	if backendServer != nil {
		result = errors.Join(result, shutdownHTTPServer(ctx, backendServer, server.config.ShutdownTimeout))
	}
	for _, listener := range append(lanListeners, backendListener) {
		if listener != nil {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				result = errors.Join(result, err)
			}
		}
	}
	return result
}

func (server *Server) BackendAddress() string {
	server.mu.RLock()
	defer server.mu.RUnlock()
	if server.backendListener == nil {
		return ""
	}
	return server.backendListener.Addr().String()
}

func (server *Server) LANAddress() string {
	server.mu.RLock()
	defer server.mu.RUnlock()
	if len(server.lanListeners) == 0 {
		return ""
	}
	return server.lanListeners[0].Addr().String()
}

// LANAddresses returns every concrete interface address currently served by
// the LAN TLS server. The first entry is kept as LANAddress for compatibility.
func (server *Server) LANAddresses() []string {
	server.mu.RLock()
	defer server.mu.RUnlock()
	addresses := make([]string, 0, len(server.lanListeners))
	for _, listener := range server.lanListeners {
		if listener != nil {
			addresses = append(addresses, listener.Addr().String())
		}
	}
	return addresses
}

// EnableLAN starts or replaces only the LAN TLS listener. The loopback
// backend remains stable so toggling Local/Remote does not disturb Tailscale
// Serve or the desktop process-token server.
func (server *Server) EnableLAN(address string, identity *TLSIdentity) (LANInfo, error) {
	return server.EnableLANAddresses([]string{address}, identity)
}

// EnableLANAddresses binds all selected physical-interface endpoints on one
// stable port. DNS-SD may therefore advertise every selected interface without
// returning an address on which the API is not actually listening.
func (server *Server) EnableLANAddresses(addresses []string, identity *TLSIdentity) (LANInfo, error) {
	if identity == nil {
		return LANInfo{}, errors.New("Library LAN listener requires a TLS identity")
	}
	addresses = normalizeLANBindAddresses(addresses)
	if len(addresses) == 0 {
		return LANInfo{}, errors.New("Library LAN listener requires an interface-bound address")
	}
	for _, address := range addresses {
		if err := validateLANBindAddress(address); err != nil {
			return LANInfo{}, err
		}
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if !server.started {
		return LANInfo{}, errors.New("Library public API backend is not running")
	}
	if len(server.lanListeners) > 0 {
		if listenersMatchLANAddresses(server.lanListeners, addresses) {
			return tcpListenerAddresses(server.lanListeners), nil
		}
		if server.lanServer != nil {
			_ = server.lanServer.Close()
		}
		for _, listener := range server.lanListeners {
			_ = listener.Close()
		}
		server.lanListeners = nil
		server.lanServer = nil
	}
	listeners, err := listenTLSLANAddresses(addresses, identity)
	if err != nil {
		return LANInfo{}, err
	}
	httpServer := server.newHTTPServer()
	server.lanListeners = listeners
	server.lanServer = httpServer
	server.config.TLSIdentity = identity
	for _, listener := range listeners {
		go serve(httpServer, listener)
	}
	return tcpListenerAddresses(listeners), nil
}

func normalizeLANBindAddresses(addresses []string) []string {
	result := make([]string, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		result = append(result, address)
	}
	return result
}

func listenTLSLANAddresses(addresses []string, identity *TLSIdentity) ([]net.Listener, error) {
	listeners := make([]net.Listener, 0, len(addresses))
	port := 0
	rollback := func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}
	for index, address := range addresses {
		resolved, err := net.ResolveTCPAddr("tcp", address)
		if err != nil || resolved == nil {
			rollback()
			return nil, fmt.Errorf("resolve Library LAN TLS address %q: %w", address, err)
		}
		if index > 0 {
			if resolved.Port != 0 && resolved.Port != port {
				rollback()
				return nil, errors.New("Library LAN listeners must share one port")
			}
			resolved.Port = port
		}
		plainListener, listenErr := net.Listen("tcp", resolved.String())
		if listenErr != nil {
			rollback()
			return nil, fmt.Errorf("listen on Library LAN TLS address %q: %w", resolved.String(), listenErr)
		}
		if index == 0 {
			port = tcpListenerAddress(plainListener.Addr()).Port
			if port == 0 {
				_ = plainListener.Close()
				rollback()
				return nil, errors.New("Library LAN listener did not receive a port")
			}
		}
		listeners = append(listeners, tls.NewListener(plainListener, &tls.Config{
			MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{identity.Certificate},
		}))
	}
	return listeners, nil
}

func listenersMatchLANAddresses(listeners []net.Listener, requestedAddresses []string) bool {
	if len(listeners) != len(requestedAddresses) || len(listeners) == 0 {
		return false
	}
	port := 0
	for index, listener := range listeners {
		current, currentOK := listener.Addr().(*net.TCPAddr)
		requested, requestedErr := net.ResolveTCPAddr("tcp", requestedAddresses[index])
		if !currentOK || current == nil || requestedErr != nil || requested == nil ||
			!current.IP.Equal(requested.IP) || strings.TrimSpace(current.Zone) != strings.TrimSpace(requested.Zone) ||
			(requested.Port != 0 && requested.Port != current.Port) {
			return false
		}
		if index == 0 {
			port = current.Port
		} else if current.Port != port {
			return false
		}
	}
	return port > 0
}

func validateLANBindAddress(address string) error {
	resolved, err := net.ResolveTCPAddr("tcp", strings.TrimSpace(address))
	if err != nil || resolved == nil || resolved.IP == nil ||
		resolved.IP.IsUnspecified() || resolved.IP.IsMulticast() {
		return errors.New("Library LAN listener must bind an explicit unicast interface address")
	}
	return nil
}

func (server *Server) DisableLAN(ctx context.Context) error {
	server.mu.Lock()
	httpServer := server.lanServer
	listeners := append([]net.Listener(nil), server.lanListeners...)
	server.lanServer = nil
	server.lanListeners = nil
	server.mu.Unlock()
	var result error
	if httpServer != nil {
		result = errors.Join(result, shutdownHTTPServer(ctx, httpServer, server.config.ShutdownTimeout))
	}
	for _, listener := range listeners {
		if listener != nil {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				result = errors.Join(result, err)
			}
		}
	}
	return result
}

func shutdownHTTPServer(parent context.Context, httpServer *http.Server, maximum time.Duration) error {
	if httpServer == nil {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx := parent
	cancel := func() {}
	if maximum > 0 {
		if deadline, hasDeadline := parent.Deadline(); !hasDeadline || time.Until(deadline) > maximum {
			ctx, cancel = context.WithTimeout(parent, maximum)
		}
	}
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		// Shutdown does not close active connections after its context expires.
		// Close is required to make Remote-off and process exit actually bounded.
		return errors.Join(err, httpServer.Close())
	}
	return nil
}

func (server *Server) TLSFingerprint() string {
	server.mu.RLock()
	defer server.mu.RUnlock()
	if server.config.TLSIdentity == nil {
		return ""
	}
	return server.config.TLSIdentity.Fingerprint
}

func (server *Server) newHTTPServer() *http.Server {
	return &http.Server{
		Handler:           server.withRequestLimit(server.config.Handler),
		ErrorLog:          log.New(pairedAPISafeErrorLogWriter{}, "", 0),
		ReadTimeout:       server.config.ReadTimeout,
		WriteTimeout:      server.config.WriteTimeout,
		IdleTimeout:       server.config.IdleTimeout,
		ReadHeaderTimeout: 10 * time.Second,
	}
}

// net/http's default ErrorLog can include a request-derived panic, a URL, a
// local path from a stack trace, or transport text containing an address. The
// paired-device server keeps only a fixed category and a one-way reference.
type pairedAPISafeErrorLogWriter struct{}

func (pairedAPISafeErrorLogWriter) Write(message []byte) (int, error) {
	digest := sha256.Sum256(message)
	zap.L().Warn(
		"paired API server event",
		zap.String("errorCode", "paired_api_server_error"),
		zap.String("errorRef", hex.EncodeToString(digest[:8])),
	)
	return len(message), nil
}

func (server *Server) withRequestLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		select {
		case server.requestSlots <- struct{}{}:
			defer func() { <-server.requestSlots }()
		case <-request.Context().Done():
			return
		default:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("{\"error\":\"server_busy\"}\n"))
			return
		}
		next.ServeHTTP(w, request)
	})
}

func serve(server *http.Server, listener net.Listener) {
	if server == nil || listener == nil {
		return
	}
	_ = server.Serve(listener)
}

func isLoopbackListener(address net.Addr) bool {
	tcpAddress, ok := address.(*net.TCPAddr)
	return ok && tcpAddress.IP != nil && tcpAddress.IP.IsLoopback()
}

func tcpListenerAddress(address net.Addr) LANInfo {
	if tcpAddress, ok := address.(*net.TCPAddr); ok {
		return LANInfo{Address: tcpAddress.String(), Port: tcpAddress.Port}
	}
	return LANInfo{Address: address.String()}
}

func tcpListenerAddresses(listeners []net.Listener) LANInfo {
	result := LANInfo{Addresses: make([]string, 0, len(listeners))}
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		info := tcpListenerAddress(listener.Addr())
		if result.Address == "" {
			result.Address = info.Address
			result.Port = info.Port
		}
		result.Addresses = append(result.Addresses, info.Address)
	}
	return result
}
