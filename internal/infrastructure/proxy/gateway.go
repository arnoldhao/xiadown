package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/netutil"
	"xiadown/internal/domain/settings"
)

const (
	gatewayMaxConcurrentConnections = 256
	gatewayMaxHeaderBytes           = 64 << 10
	gatewayProxyConnectHeaderBytes  = 64 << 10
	gatewayAttestationPath          = "/.well-known/xiadown-network-route"
	gatewayAttestationHostSuffix    = ".attest.xiadown.invalid"
	gatewayConnectChallengeInfix    = ".connect."
	gatewayAttestationHeader        = "X-XiaDown-Gateway-Attestation"
	gatewayConnectChallengeHeader   = "X-XiaDown-Gateway-Connect-Challenge"
	gatewayAttestationProofTTL      = 15 * time.Second
	gatewayAttestationProofLimit    = 64
	gatewayAttestationProofBytes    = 28
)

type gatewayAttestationProof struct {
	createdAt time.Time
	connected bool
	consumed  bool
}

type loopbackGateway struct {
	listener         net.Listener
	server           *http.Server
	active           atomic.Pointer[routeState]
	internal         *internalLoopbackRegistry
	closeMu          sync.Mutex
	closed           bool
	url              string
	attestationToken string
	attestationHost  string
	attestationKey   []byte
	attestationMu    sync.Mutex
	attestationProof map[string]gatewayAttestationProof
}

func newLoopbackGateway(initial *routeState) (*loopbackGateway, error) {
	baseListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start proxy gateway: %w", err)
	}
	listener := netutil.LimitListener(baseListener, gatewayMaxConcurrentConnections)
	// The response token, challenge-signing key, and private control authority
	// are independent random values. The random .invalid authority can never be
	// a real remote origin, and every path for it is intercepted below.
	attestationBytes := make([]byte, 80)
	if _, err := rand.Read(attestationBytes); err != nil {
		_ = baseListener.Close()
		return nil, fmt.Errorf("initialize proxy gateway attestation: %w", err)
	}
	gateway := &loopbackGateway{
		listener:         listener,
		url:              "http://" + listener.Addr().String(),
		internal:         newInternalLoopbackRegistry(),
		attestationToken: base64.RawURLEncoding.EncodeToString(attestationBytes[:32]),
		attestationHost:  hex.EncodeToString(attestationBytes[64:]) + gatewayAttestationHostSuffix,
		attestationKey:   append([]byte(nil), attestationBytes[32:64]...),
		attestationProof: make(map[string]gatewayAttestationProof),
	}
	initial.setGatewayAddress(listener.Addr().String())
	gateway.active.Store(initial)
	gateway.server = &http.Server{
		Handler:           gateway,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    gatewayMaxHeaderBytes,
	}
	go func() {
		// Serve errors are intentionally not logged here: the normal Close path
		// returns http.ErrServerClosed and proxy failures must never include a
		// credential-bearing upstream URL in logs.
		_ = gateway.server.Serve(listener)
	}()
	return gateway, nil
}

func (g *loopbackGateway) URL() string { return g.url }

func (g *loopbackGateway) attestation() (string, string) {
	if g == nil || strings.TrimSpace(g.attestationToken) == "" || strings.TrimSpace(g.attestationHost) == "" {
		return "", ""
	}
	return "http://" + g.attestationHost + gatewayAttestationPath, g.attestationToken
}

func (g *loopbackGateway) swap(next *routeState) *routeState {
	next.setGatewayAddress(g.listener.Addr().String())
	return g.active.Swap(next)
}

func (g *loopbackGateway) Close() error {
	g.closeMu.Lock()
	if g.closed {
		g.closeMu.Unlock()
		return nil
	}
	g.closed = true
	g.closeMu.Unlock()

	state := g.active.Swap(nil)
	if state != nil {
		state.close()
	}
	// Close the listener explicitly. Close may race the Serve goroutine before
	// net/http has registered the listener internally; Server.Close alone does
	// not cover that startup window.
	listenerErr := g.listener.Close()
	serverErr := g.server.Close()
	if listenerErr != nil && !isNormalGatewayCloseError(listenerErr) {
		return listenerErr
	}
	if serverErr != nil && !isNormalGatewayCloseError(serverErr) {
		return serverErr
	}
	return nil
}

func isNormalGatewayCloseError(err error) bool {
	return err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, http.ErrServerClosed) ||
		strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

func (g *loopbackGateway) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	state := g.active.Load()
	if state == nil {
		http.Error(w, "proxy gateway unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("X-XiaDown-Network-Generation", strconv.FormatUint(state.generation, 10))
	if g.serveConnectAttestation(w, request) {
		return
	}
	if g.serveAttestation(w, request) {
		return
	}
	loopback, permitted, forceDirect := g.internal.permitsRequest(request, g.listener.Addr().String())
	if loopback && !permitted {
		w.Header().Set("X-XiaDown-Network-Error", "loopback-denied")
		state.logRouteFailure(request, "destination-policy", "loopback-denied")
		http.Error(w, "loopback destination is not an app-owned endpoint", http.StatusForbidden)
		return
	}
	if request.Method == http.MethodConnect {
		state.serveConnect(w, request, forceDirect)
		return
	}
	state.serveForward(w, request, forceDirect)
}

func (g *loopbackGateway) serveAttestation(w http.ResponseWriter, request *http.Request) bool {
	attestationURL, token := g.attestation()
	if request == nil || request.URL == nil || token == "" {
		return false
	}
	expected, err := url.Parse(attestationURL)
	if err != nil || !strings.EqualFold(requestAuthorityHostname(request), expected.Hostname()) {
		return false
	}
	// Once the private authority matches, never forward any method, port, or
	// path for it. This prevents a configured proxy or VPN from manufacturing a
	// same-origin page that could read the long-lived response token.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	requestTarget, _ := canonicalNetworkTarget(request.URL.Host, "http")
	expectedTarget, _ := canonicalNetworkTarget(expected.Host, "http")
	if !strings.EqualFold(request.URL.Scheme, expected.Scheme) || requestTarget == "" ||
		requestTarget != expectedTarget || request.URL.Path != expected.Path ||
		(request.URL.RawPath != "" && request.URL.RawPath != expected.Path) {
		http.Error(w, "unknown gateway attestation route", http.StatusNotFound)
		return true
	}
	if request.Method != http.MethodGet {
		http.Error(w, "invalid gateway attestation request", http.StatusBadRequest)
		return true
	}
	if request.URL.RawQuery == "" {
		proofID, proofErr := g.beginConnectAttestation(time.Now())
		if proofErr != nil {
			http.Error(w, "gateway attestation unavailable", http.StatusServiceUnavailable)
			return true
		}
		challengeURL := "https://" + proofID + gatewayConnectChallengeInfix + g.attestationHost + gatewayAttestationPath
		w.Header().Set(gatewayConnectChallengeHeader, challengeURL)
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	query, queryErr := url.ParseQuery(request.URL.RawQuery)
	proofIDs, proofOnly := query["proof"]
	if queryErr != nil || len(query) != 1 || !proofOnly || len(proofIDs) != 1 || !g.validAttestationProofID(proofIDs[0], time.Now()) {
		http.Error(w, "invalid gateway attestation proof", http.StatusBadRequest)
		return true
	}
	if !g.consumeConnectAttestation(proofIDs[0]) {
		http.Error(w, "gateway CONNECT attestation is incomplete", http.StatusForbidden)
		return true
	}
	w.Header().Set(gatewayAttestationHeader, token)
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (g *loopbackGateway) serveConnectAttestation(w http.ResponseWriter, request *http.Request) bool {
	if request == nil || request.Method != http.MethodConnect {
		return false
	}
	target, err := normalizeConnectTarget(request.Host)
	if err != nil {
		return false
	}
	host, port, err := net.SplitHostPort(target)
	host = strings.ToLower(strings.TrimSpace(host))
	expectedSuffix := gatewayConnectChallengeInfix + strings.ToLower(g.attestationHost)
	if err != nil || port != "443" || !strings.HasSuffix(host, expectedSuffix) {
		return false
	}
	proofID := strings.TrimSuffix(host, expectedSuffix)
	createdAt, valid := g.attestationProofTime(proofID, time.Now())
	if !valid || !g.markConnectAttestation(proofID, createdAt) {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "unknown gateway CONNECT attestation", http.StatusForbidden)
		return true
	}
	// A successful upstream tunnel is intentionally unnecessary: observing the
	// browser's CONNECT for an unpredictable, one-shot authority proves that
	// HTTPS uses this gateway. Reject it immediately, then let the control-plane
	// request consume the proof and verify the secret response header.
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "gateway CONNECT attestation observed", http.StatusBadGateway)
	return true
}

func (g *loopbackGateway) beginConnectAttestation(now time.Time) (string, error) {
	proof := make([]byte, gatewayAttestationProofBytes)
	binary.BigEndian.PutUint32(proof[:4], uint32(now.Unix()))
	if _, err := rand.Read(proof[4:12]); err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, g.attestationKey)
	_, _ = mac.Write(proof[:12])
	copy(proof[12:], mac.Sum(nil)[:16])
	return hex.EncodeToString(proof), nil
}

func (g *loopbackGateway) markConnectAttestation(proofID string, createdAt time.Time) bool {
	now := time.Now()
	g.attestationMu.Lock()
	defer g.attestationMu.Unlock()
	g.purgeAttestationProofsLocked(now)
	if proof, ok := g.attestationProof[proofID]; ok {
		if proof.consumed {
			return false
		}
		proof.connected = true
		g.attestationProof[proofID] = proof
		return true
	}
	if len(g.attestationProof) >= gatewayAttestationProofLimit {
		var oldestID string
		var oldestTime time.Time
		for candidateID, candidate := range g.attestationProof {
			if oldestTime.IsZero() || candidate.createdAt.Before(oldestTime) {
				oldestID, oldestTime = candidateID, candidate.createdAt
			}
		}
		delete(g.attestationProof, oldestID)
	}
	g.attestationProof[proofID] = gatewayAttestationProof{createdAt: createdAt, connected: true}
	return true
}

func (g *loopbackGateway) consumeConnectAttestation(proofID string) bool {
	now := time.Now()
	g.attestationMu.Lock()
	defer g.attestationMu.Unlock()
	g.purgeAttestationProofsLocked(now)
	proof, ok := g.attestationProof[proofID]
	if !ok || !proof.connected || proof.consumed {
		return false
	}
	proof.consumed = true
	g.attestationProof[proofID] = proof
	return true
}

func (g *loopbackGateway) purgeAttestationProofsLocked(now time.Time) {
	for proofID, proof := range g.attestationProof {
		if now.Sub(proof.createdAt) > gatewayAttestationProofTTL {
			delete(g.attestationProof, proofID)
		}
	}
}

func (g *loopbackGateway) validAttestationProofID(proofID string, now time.Time) bool {
	_, valid := g.attestationProofTime(proofID, now)
	return valid
}

func (g *loopbackGateway) attestationProofTime(proofID string, now time.Time) (time.Time, bool) {
	decoded, err := hex.DecodeString(proofID)
	if err != nil || len(decoded) != gatewayAttestationProofBytes || len(g.attestationKey) == 0 {
		return time.Time{}, false
	}
	mac := hmac.New(sha256.New, g.attestationKey)
	_, _ = mac.Write(decoded[:12])
	if !hmac.Equal(decoded[12:], mac.Sum(nil)[:16]) {
		return time.Time{}, false
	}
	createdAt := time.Unix(int64(binary.BigEndian.Uint32(decoded[:4])), 0)
	age := now.Sub(createdAt)
	if age < -time.Second || age > gatewayAttestationProofTTL {
		return time.Time{}, false
	}
	return createdAt, true
}

func requestAuthorityHostname(request *http.Request) string {
	if request == nil || request.URL == nil {
		return ""
	}
	if hostname := request.URL.Hostname(); hostname != "" {
		return strings.ToLower(strings.TrimSuffix(hostname, "."))
	}
	authority := strings.TrimSpace(request.Host)
	if host, _, err := net.SplitHostPort(authority); err == nil {
		authority = host
	}
	return strings.ToLower(strings.TrimSuffix(strings.Trim(authority, "[]"), "."))
}

type routeState struct {
	config      Config
	generation  uint64
	transport   *http.Transport
	direct      *http.Transport
	context     context.Context
	cancel      context.CancelFunc
	systemProxy systemProxyResolver
	directDNS   directIPResolver

	forwardMu         sync.Mutex
	forwardTransports map[string]*http.Transport

	gatewayAddress string

	tunnelMu    sync.Mutex
	tunnels     map[*activeTunnel]struct{}
	connections map[*trackedRouteConn]struct{}
	closed      bool

	diagnosticMu   sync.Mutex
	diagnosticLast map[string]time.Time
}

func newRouteState(config Config, generation uint64) (*routeState, error) {
	config = cloneConfig(config)
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	routeContext, cancel := context.WithCancel(context.Background())
	state := &routeState{
		config:            config,
		generation:        generation,
		context:           routeContext,
		cancel:            cancel,
		tunnels:           make(map[*activeTunnel]struct{}),
		connections:       make(map[*trackedRouteConn]struct{}),
		diagnosticLast:    make(map[string]time.Time),
		directDNS:         net.DefaultResolver,
		forwardTransports: make(map[string]*http.Transport),
	}
	if config.Mode == settings.ProxyModeSystem {
		state.systemProxy = newPlatformSystemProxyResolver()
	}
	tlsTimeout := config.Timeout
	if tlsTimeout <= 0 || tlsTimeout > 10*time.Second {
		tlsTimeout = 10 * time.Second
	}
	state.transport = &http.Transport{
		Proxy:                  nil,
		DialContext:            state.dialDirect,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           100,
		IdleConnTimeout:        90 * time.Second,
		TLSHandshakeTimeout:    tlsTimeout,
		ResponseHeaderTimeout:  config.Timeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 1 << 20,
	}
	state.direct = state.transport.Clone()
	state.direct.Proxy = nil
	state.direct.DialContext = state.dialDirect
	return state, nil
}

func (s *routeState) setGatewayAddress(address string) {
	s.gatewayAddress = address
}

func (s *routeState) close() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.tunnelMu.Lock()
	if s.closed {
		s.tunnelMu.Unlock()
		return
	}
	s.closed = true
	tunnels := make([]*activeTunnel, 0, len(s.tunnels))
	for tunnel := range s.tunnels {
		tunnels = append(tunnels, tunnel)
	}
	connections := make([]*trackedRouteConn, 0, len(s.connections))
	for connection := range s.connections {
		connections = append(connections, connection)
	}
	s.tunnelMu.Unlock()
	if s.transport != nil {
		s.transport.CloseIdleConnections()
	}
	if s.direct != nil {
		s.direct.CloseIdleConnections()
	}
	s.forwardMu.Lock()
	forwardTransports := make([]*http.Transport, 0, len(s.forwardTransports))
	for _, transport := range s.forwardTransports {
		forwardTransports = append(forwardTransports, transport)
	}
	s.forwardTransports = nil
	s.forwardMu.Unlock()
	for _, transport := range forwardTransports {
		transport.CloseIdleConnections()
	}
	for _, tunnel := range tunnels {
		tunnel.close()
	}
	for _, connection := range connections {
		_ = connection.Close()
	}
	if s.systemProxy != nil {
		s.systemProxy.Close()
	}
}

func (s *routeState) registerTunnel(tunnel *activeTunnel) bool {
	s.tunnelMu.Lock()
	defer s.tunnelMu.Unlock()
	if s.closed {
		return false
	}
	s.tunnels[tunnel] = struct{}{}
	return true
}

func (s *routeState) unregisterTunnel(tunnel *activeTunnel) {
	s.tunnelMu.Lock()
	delete(s.tunnels, tunnel)
	s.tunnelMu.Unlock()
}

func (s *routeState) registerConnection(connection *trackedRouteConn) bool {
	s.tunnelMu.Lock()
	defer s.tunnelMu.Unlock()
	if s.closed {
		return false
	}
	s.connections[connection] = struct{}{}
	return true
}

func (s *routeState) unregisterConnection(connection *trackedRouteConn) {
	s.tunnelMu.Lock()
	delete(s.connections, connection)
	s.tunnelMu.Unlock()
}

// generationContext uses the policy generation as the direct parent so
// cancel() closes Done synchronously for every request already holding this
// state. Caller cancellation/deadlines are then joined without reopening the
// post-Apply window that context.AfterFunc(state, cancel) would create.
func (s *routeState) generationContext(caller context.Context) (context.Context, context.CancelFunc) {
	if caller == nil {
		caller = context.Background()
	}
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if deadline, ok := caller.Deadline(); ok {
		ctx, cancel = context.WithDeadline(s.context, deadline)
	} else {
		ctx, cancel = context.WithCancel(s.context)
	}
	stopCallerCancel := context.AfterFunc(caller, cancel)
	if caller.Err() != nil {
		cancel()
	}
	return &callerValueContext{Context: ctx, caller: caller}, func() {
		stopCallerCancel()
		cancel()
	}
}

type callerValueContext struct {
	context.Context
	caller context.Context
}

func (ctx *callerValueContext) Value(key any) any {
	return ctx.caller.Value(key)
}

func (s *routeState) proxyForRequest(request *http.Request) (*url.URL, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("proxy target URL is missing")
	}
	if shouldBypassURL(request.URL, s.config.NoProxy) {
		return nil, nil
	}
	if s.config.Mode == settings.ProxyModeSystem {
		resolveContext := request.Context()
		if s.config.Timeout > 0 {
			var cancel context.CancelFunc
			resolveContext, cancel = context.WithTimeout(resolveContext, s.config.Timeout)
			defer cancel()
		}
		return systemProxyURLContext(resolveContext, s.systemProxy, request.URL)
	}
	return resolveProxyURL(s.config, request)
}

func resolveProxyURL(config Config, request *http.Request) (*url.URL, error) {
	switch config.Mode {
	case settings.ProxyModeNone:
		return nil, nil
	case settings.ProxyModeManual:
		proxyURL := buildProxyURL(config)
		if proxyURL == nil {
			return nil, errors.New("manual proxy is incomplete")
		}
		return proxyURL, nil
	case settings.ProxyModeSystem:
		return nil, errors.New("system proxy resolver is not bound to a policy generation")
	default:
		return nil, fmt.Errorf("unsupported proxy mode %q", config.Mode.String())
	}
}

func (s *routeState) serveForward(w http.ResponseWriter, incoming *http.Request, forceDirect bool) {
	requestContext, cancel := s.generationContext(incoming.Context())
	defer cancel()

	outgoing := incoming.Clone(requestContext)
	outgoing.RequestURI = ""
	if outgoing.URL.Scheme == "" {
		outgoing.URL.Scheme = "http"
	}
	switch strings.ToLower(outgoing.URL.Scheme) {
	case "ws":
		outgoing.URL.Scheme = "http"
	case "wss":
		outgoing.URL.Scheme = "https"
	}
	if outgoing.URL.Host == "" {
		outgoing.URL.Host = incoming.Host
	}
	requestedUpgrade := httpUpgradeType(outgoing.Header)
	if requestedUpgrade != "" && !isPrintableASCII(requestedUpgrade) {
		http.Error(w, "invalid protocol upgrade", http.StatusBadRequest)
		return
	}
	removeHopByHopHeaders(outgoing.Header)
	if requestedUpgrade != "" {
		outgoing.Header.Set("Connection", "Upgrade")
		outgoing.Header.Set("Upgrade", requestedUpgrade)
	}
	outgoing.Header.Del("Proxy-Authorization")
	outgoing.Header.Del("Proxy-Connection")

	response, err := s.roundTrip(outgoing, forceDirect)
	if err != nil {
		errorClass := classifyRouteError(err)
		w.Header().Set("X-XiaDown-Network-Error", errorClass)
		s.logRouteFailure(outgoing, "forward", errorClass)
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	if response.StatusCode == http.StatusProxyAuthRequired {
		response.Body.Close()
		w.Header().Set("X-XiaDown-Network-Error", "proxy-auth")
		s.logRouteFailure(outgoing, "proxy-auth", "proxy-auth")
		http.Error(w, "upstream proxy authentication failed", http.StatusBadGateway)
		return
	}
	if response.StatusCode == http.StatusSwitchingProtocols {
		if requestedUpgrade == "" {
			response.Body.Close()
			w.Header().Set("X-XiaDown-Network-Error", "unexpected-upgrade")
			http.Error(w, "unexpected upstream protocol upgrade", http.StatusBadGateway)
			return
		}
		s.serveUpgrade(w, incoming, response, requestedUpgrade)
		return
	}
	defer response.Body.Close()
	removeHopByHopHeaders(response.Header)
	copyHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	controller := http.NewResponseController(w)
	_ = controller.Flush()
	if _, copyErr := io.Copy(&flushResponseWriter{writer: w, controller: controller}, response.Body); copyErr != nil {
		s.logRouteFailure(outgoing, "response-body", classifyRouteError(copyErr))
	}
}

type flushResponseWriter struct {
	writer     io.Writer
	controller *http.ResponseController
}

func (writer *flushResponseWriter) Write(data []byte) (int, error) {
	written, err := writer.writer.Write(data)
	if flushErr := writer.controller.Flush(); err == nil && flushErr != nil {
		err = flushErr
	}
	return written, err
}

func (s *routeState) serveUpgrade(w http.ResponseWriter, incoming *http.Request, response *http.Response, requestedUpgrade string) {
	responseUpgrade := httpUpgradeType(response.Header)
	if !strings.EqualFold(requestedUpgrade, responseUpgrade) || !isPrintableASCII(responseUpgrade) {
		response.Body.Close()
		w.Header().Set("X-XiaDown-Network-Error", "upgrade-mismatch")
		http.Error(w, "upstream protocol upgrade mismatch", http.StatusBadGateway)
		return
	}
	upstream, ok := response.Body.(io.ReadWriteCloser)
	if !ok {
		response.Body.Close()
		w.Header().Set("X-XiaDown-Network-Error", "upgrade-unsupported")
		http.Error(w, "upstream protocol upgrade unsupported", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		upstream.Close()
		http.Error(w, "protocol upgrade unsupported", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		upstream.Close()
		return
	}
	tunnel := &activeTunnel{client: client, upstream: upstream}
	if !s.registerTunnel(tunnel) {
		tunnel.close()
		return
	}
	defer s.unregisterTunnel(tunnel)
	defer tunnel.close()

	copyHeaders(response.Header, w.Header())
	response.Header.Set("X-XiaDown-Network-Generation", strconv.FormatUint(s.generation, 10))
	response.Body = nil
	if err := response.Write(buffered); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	clientSource := io.Reader(client)
	if buffered.Reader.Buffered() > 0 {
		clientSource = buffered.Reader
	}

	relayBidirectional(tunnel, clientSource)
}

func (s *routeState) serveConnect(w http.ResponseWriter, request *http.Request, forceDirect bool) {
	requestContext, cancel := s.generationContext(request.Context())
	defer cancel()
	target, err := normalizeConnectTarget(request.Host)
	if err != nil {
		http.Error(w, "invalid CONNECT target", http.StatusBadRequest)
		return
	}
	var upstream net.Conn
	if forceDirect {
		upstream, err = s.dialDirect(requestContext, "tcp", target)
	} else {
		upstream, err = s.dialTunnel(requestContext, target)
	}
	if err != nil {
		errorClass := classifyRouteError(err)
		w.Header().Set("X-XiaDown-Network-Error", errorClass)
		s.logRouteFailure(request, "connect", errorClass)
		http.Error(w, "upstream connection failed", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "CONNECT unsupported", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	tunnel := &activeTunnel{client: client, upstream: upstream}
	if !s.registerTunnel(tunnel) {
		tunnel.close()
		return
	}
	defer s.unregisterTunnel(tunnel)
	if _, err := fmt.Fprintf(
		buffered,
		"HTTP/1.1 200 Connection Established\r\nX-XiaDown-Network-Generation: %d\r\n\r\n",
		s.generation,
	); err != nil {
		tunnel.close()
		return
	}
	if err := buffered.Flush(); err != nil {
		tunnel.close()
		return
	}
	clientSource := net.Conn(client)
	if buffered.Reader.Buffered() > 0 {
		clientSource = &bufferedNetConn{Conn: client, reader: buffered.Reader}
	}

	relayBidirectional(tunnel, clientSource)
}

func (s *routeState) dialTunnel(ctx context.Context, target string) (net.Conn, error) {
	host, port, _ := net.SplitHostPort(target)
	targetURL := &url.URL{Scheme: "https", Host: net.JoinHostPort(host, port)}
	return s.dialLogicalRoute(ctx, "tcp", target, targetURL, false)
}

func (s *routeState) dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	targetURL := &url.URL{Scheme: "https", Host: address}
	return s.dialPublicURL(ctx, network, address, targetURL)
}

func (s *routeState) dialPublicURL(ctx context.Context, network, address string, targetURL *url.URL) (net.Conn, error) {
	logicalAddress, err := logicalRouteAddress(targetURL)
	if err != nil {
		return nil, err
	}
	if !sameCanonicalNetworkTarget(logicalAddress, address) {
		return nil, errors.New("public logical URL does not match the requested authority")
	}
	requestContext, cancel := s.generationContext(ctx)
	defer cancel()

	pinnedAddresses, err := resolveAndPinPublicAddresses(requestContext, network, address)
	if err != nil {
		return nil, err
	}
	proxyURL, err := s.proxyForLogicalURL(requestContext, targetURL)
	if err != nil {
		return nil, err
	}
	return s.dialPinnedRoute(requestContext, network, pinnedAddresses, proxyURL)
}

type trackedRouteConn struct {
	net.Conn
	state *routeState
	once  sync.Once
}

func (c *trackedRouteConn) Close() error {
	var err error
	c.once.Do(func() {
		err = c.Conn.Close()
		c.state.unregisterConnection(c)
	})
	return err
}

func (c *trackedRouteConn) CloseWrite() error {
	if writer, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return writer.CloseWrite()
	}
	return errors.New("half-close is unsupported")
}

type activeTunnel struct {
	client   net.Conn
	upstream io.ReadWriteCloser
	once     sync.Once
}

func (t *activeTunnel) close() {
	t.once.Do(func() {
		_ = t.client.Close()
		_ = t.upstream.Close()
	})
}

func relayBidirectional(tunnel *activeTunnel, clientSource io.Reader) {
	if tunnel == nil {
		return
	}
	var copies sync.WaitGroup
	copies.Add(2)
	go func() {
		defer copies.Done()
		_, err := io.Copy(tunnel.upstream, clientSource)
		if err != nil || !closeWrite(tunnel.upstream) {
			tunnel.close()
		}
	}()
	go func() {
		defer copies.Done()
		_, err := io.Copy(tunnel.client, tunnel.upstream)
		if err != nil || !closeWrite(tunnel.client) {
			tunnel.close()
		}
	}()
	copies.Wait()
	tunnel.close()
}

func closeWrite(connection any) bool {
	writer, ok := connection.(interface{ CloseWrite() error })
	if !ok {
		return false
	}
	return writer.CloseWrite() == nil
}

func newGatewayHTTPClient(gatewayURL string, timeout time.Duration) (*http.Client, error) {
	parsed, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, fmt.Errorf("parse proxy gateway URL: %w", err)
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(parsed),
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func (s *routeState) dialDirect(ctx context.Context, network, address string) (net.Conn, error) {
	pinnedAddresses, explicitlyLoopback, err := s.resolveAndPinDirectAddresses(ctx, network, address)
	if err != nil {
		return nil, err
	}
	for _, pinnedAddress := range pinnedAddresses {
		if sameCanonicalNetworkTarget(pinnedAddress, s.gatewayAddress) {
			return nil, errors.New("network policy rejected a recursive gateway route")
		}
	}
	connection, err := dialPinnedAddresses(ctx, network, pinnedAddresses, s.config.Timeout)
	if err != nil {
		return nil, err
	}
	if sameNetworkEndpoint(connection.RemoteAddr(), s.gatewayAddress) {
		_ = connection.Close()
		return nil, errors.New("network policy rejected a recursive gateway route")
	}
	if remoteIP := networkAddressIP(connection.RemoteAddr()); remoteIP != nil && remoteIP.IsLoopback() {
		if !explicitlyLoopback {
			_ = connection.Close()
			return nil, errors.New("network policy rejected a hostname resolving to loopback")
		}
	}
	tracked := &trackedRouteConn{Conn: connection, state: s}
	if !s.registerConnection(tracked) {
		_ = connection.Close()
		return nil, errors.New("proxy policy changed while connecting")
	}
	return tracked, nil
}

func networkAddressIP(address net.Addr) net.IP {
	if address == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		return nil
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

func sameNetworkEndpoint(remote net.Addr, expected string) bool {
	if remote == nil || strings.TrimSpace(expected) == "" {
		return false
	}
	remoteHost, remotePort, remoteErr := net.SplitHostPort(remote.String())
	expectedHost, expectedPort, expectedErr := net.SplitHostPort(expected)
	if remoteErr != nil || expectedErr != nil || remotePort != expectedPort {
		return false
	}
	remoteIP := net.ParseIP(strings.Trim(remoteHost, "[]"))
	expectedIP := net.ParseIP(strings.Trim(expectedHost, "[]"))
	if remoteIP != nil && expectedIP != nil {
		return remoteIP.Equal(expectedIP)
	}
	return strings.EqualFold(remoteHost, expectedHost)
}

func newPublicHTTPClient(manager *Manager, timeout time.Duration) *http.Client {
	return &http.Client{Transport: &managerPublicRoundTripper{manager: manager}, Timeout: timeout}
}

func normalizeConnectTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" || strings.ContainsAny(target, "\r\n\x00") {
		return "", errors.New("invalid target")
	}
	if host, port, err := net.SplitHostPort(target); err == nil {
		if host == "" || port == "" {
			return "", errors.New("invalid target")
		}
		if _, err := parsePort(port); err != nil {
			return "", err
		}
		return net.JoinHostPort(host, port), nil
	}
	if strings.Contains(target, ":") {
		// A bare IPv6 literal is valid and gets CONNECT's default HTTPS port.
		if ip := net.ParseIP(strings.Trim(target, "[]")); ip != nil {
			return net.JoinHostPort(ip.String(), "443"), nil
		}
		return "", errors.New("invalid target")
	}
	return net.JoinHostPort(target, "443"), nil
}

func dialHTTPConnectProxyWithDialer(
	ctx context.Context,
	proxyURL *url.URL,
	target string,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (net.Conn, error) {
	proxyAddress, err := proxyAddress(proxyURL)
	if err != nil {
		return nil, err
	}
	if dialContext == nil {
		return nil, errors.New("HTTP proxy route requires a generation-owned dialer")
	}
	conn, err := dialContext(ctx, "tcp", proxyAddress)
	if err != nil {
		return nil, err
	}
	stop, err := applyHandshakeContext(ctx, conn, timeout)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	success := false
	defer func() {
		stop()
		if !success {
			_ = conn.Close()
		}
	}()
	if strings.EqualFold(proxyURL.Scheme, "https") {
		tlsConnection := tls.Client(conn, &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: proxyURL.Hostname(),
		})
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		conn = tlsConnection
	}

	header := make(http.Header)
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(proxyURL.User.Username() + ":" + password))
		header.Set("Proxy-Authorization", "Basic "+token)
	}
	connectRequest := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: target},
		Host:   target,
		Header: header,
	}
	if err := connectRequest.Write(conn); err != nil {
		return nil, err
	}
	response, reader, err := readBoundedProxyConnectResponse(conn, connectRequest)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		if response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, fmt.Errorf("HTTP proxy CONNECT returned status %d", response.StatusCode)
	}
	stop()
	_ = conn.SetDeadline(time.Time{})
	success = true
	if reader.Buffered() > 0 {
		return &bufferedNetConn{Conn: conn, reader: reader}, nil
	}
	return conn, nil
}

func readBoundedProxyConnectResponse(connection io.Reader, request *http.Request) (*http.Response, *bufio.Reader, error) {
	if connection == nil {
		return nil, nil, io.EOF
	}
	source := bufio.NewReader(connection)
	var header bytes.Buffer
	for {
		fragment, err := source.ReadSlice('\n')
		if header.Len()+len(fragment) > gatewayProxyConnectHeaderBytes {
			return nil, nil, errors.New("HTTP proxy CONNECT response headers exceed the safety limit")
		}
		_, _ = header.Write(fragment)
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return nil, nil, err
		}
		payload := header.Bytes()
		if bytes.HasSuffix(payload, []byte("\r\n\r\n")) || bytes.HasSuffix(payload, []byte("\n\n")) {
			break
		}
	}
	response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(header.Bytes())), request)
	if err != nil {
		return nil, nil, err
	}
	return response, source, nil
}

func proxyAddress(proxyURL *url.URL) (string, error) {
	if proxyURL == nil || proxyURL.Hostname() == "" {
		return "", errors.New("proxy address is empty")
	}
	port := proxyURL.Port()
	if port == "" {
		if strings.EqualFold(proxyURL.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	if _, err := parsePort(port); err != nil {
		return "", err
	}
	return net.JoinHostPort(proxyURL.Hostname(), port), nil
}

type bufferedNetConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedNetConn) Read(buffer []byte) (int, error) {
	return c.reader.Read(buffer)
}

func (c *bufferedNetConn) CloseWrite() error {
	if writer, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return writer.CloseWrite()
	}
	return errors.New("half-close is unsupported")
}

func dialSOCKS5WithDialer(
	ctx context.Context,
	socksAddress,
	target,
	username,
	password string,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (net.Conn, error) {
	if _, _, err := net.SplitHostPort(socksAddress); err != nil {
		return nil, errors.New("invalid SOCKS5 proxy address")
	}
	if dialContext == nil {
		return nil, errors.New("SOCKS5 route requires a generation-owned dialer")
	}
	conn, err := dialContext(ctx, "tcp", socksAddress)
	if err != nil {
		return nil, err
	}
	stop, err := applyHandshakeContext(ctx, conn, timeout)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	success := false
	defer func() {
		stop()
		if !success {
			_ = conn.Close()
		}
	}()

	requireAuthentication := username != "" || password != ""
	methods := []byte{0x00}
	if requireAuthentication {
		// Supplying credentials is an explicit authentication requirement. Do
		// not also advertise no-auth and allow a proxy to downgrade the route.
		methods = []byte{0x02}
	}
	greeting := append([]byte{0x05, byte(len(methods))}, methods...)
	if err := writeAll(conn, greeting); err != nil {
		return nil, err
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(conn, methodReply); err != nil {
		return nil, err
	}
	if methodReply[0] != 0x05 {
		return nil, errors.New("SOCKS5 proxy returned an invalid version")
	}
	switch methodReply[1] {
	case 0x00:
		if requireAuthentication {
			return nil, errors.New("SOCKS5 proxy selected authentication method that was not offered")
		}
	case 0x02:
		if !requireAuthentication {
			return nil, errors.New("SOCKS5 proxy selected authentication method that was not offered")
		}
		if len(username) > 255 || len(password) > 255 {
			return nil, errors.New("SOCKS5 credentials are too long")
		}
		auth := []byte{0x01, byte(len(username))}
		auth = append(auth, username...)
		auth = append(auth, byte(len(password)))
		auth = append(auth, password...)
		if err := writeAll(conn, auth); err != nil {
			return nil, err
		}
		authReply := make([]byte, 2)
		if _, err := io.ReadFull(conn, authReply); err != nil {
			return nil, err
		}
		if authReply[0] != 0x01 || authReply[1] != 0x00 {
			return nil, errors.New("SOCKS5 authentication failed")
		}
	case 0xff:
		return nil, errors.New("SOCKS5 proxy rejected authentication methods")
	default:
		return nil, errors.New("SOCKS5 proxy selected an unsupported authentication method")
	}

	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return nil, err
	}
	port, err := parsePort(portText)
	if err != nil {
		return nil, err
	}
	connect := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			connect = append(connect, 0x01)
			connect = append(connect, ipv4...)
		} else {
			connect = append(connect, 0x04)
			connect = append(connect, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil, errors.New("SOCKS5 target host is too long")
		}
		connect = append(connect, 0x03, byte(len(host)))
		connect = append(connect, host...)
	}
	connect = append(connect, byte(port>>8), byte(port))
	if err := writeAll(conn, connect); err != nil {
		return nil, err
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return nil, err
	}
	if reply[0] != 0x05 || reply[1] != 0x00 {
		return nil, fmt.Errorf("SOCKS5 CONNECT failed with status %d", reply[1])
	}
	addressLength := 0
	switch reply[3] {
	case 0x01:
		addressLength = net.IPv4len
	case 0x04:
		addressLength = net.IPv6len
	case 0x03:
		length := []byte{0}
		if _, err := io.ReadFull(conn, length); err != nil {
			return nil, err
		}
		addressLength = int(length[0])
	default:
		return nil, errors.New("SOCKS5 proxy returned an invalid address type")
	}
	if _, err := io.CopyN(io.Discard, conn, int64(addressLength+2)); err != nil {
		return nil, err
	}
	stop()
	_ = conn.SetDeadline(time.Time{})
	success = true
	return conn, nil
}

func applyHandshakeContext(ctx context.Context, conn net.Conn, timeout time.Duration) (func() bool, error) {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if contextDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || contextDeadline.Before(deadline)) {
		deadline = contextDeadline
	}
	if !deadline.IsZero() {
		if err := conn.SetDeadline(deadline); err != nil {
			return func() bool { return true }, err
		}
	}
	return context.AfterFunc(ctx, func() { _ = conn.Close() }), nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		data = data[written:]
	}
	return nil
}

func removeHopByHopHeaders(header http.Header) {
	for _, connectionHeader := range header.Values("Connection") {
		for _, token := range strings.Split(connectionHeader, ",") {
			header.Del(strings.TrimSpace(token))
		}
	}
	for _, name := range []string{
		"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
	} {
		header.Del(name)
	}
}

func httpUpgradeType(header http.Header) string {
	if !headerValuesContainToken(header.Values("Connection"), "upgrade") {
		return ""
	}
	return strings.TrimSpace(header.Get("Upgrade"))
}

func headerValuesContainToken(values []string, target string) bool {
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), target) {
				return true
			}
		}
	}
	return false
}

func isPrintableASCII(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func classifyRouteError(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return "dns"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "recursive gateway"):
		return "gateway-loop"
	case strings.Contains(message, "407"), strings.Contains(message, "proxy authentication"), strings.Contains(message, "authentication required"):
		return "proxy-auth"
	case strings.Contains(message, "tls"), strings.Contains(message, "certificate"), strings.Contains(message, "x509"):
		return "tls"
	case strings.Contains(message, "system proxy"), strings.Contains(message, "pac"), strings.Contains(message, "proxy resolver"):
		return "system-policy"
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return "timeout"
	}
	return "connect"
}

func (s *routeState) logRouteFailure(request *http.Request, stage, errorClass string) {
	destinationHost := ""
	if request != nil {
		if request.URL != nil {
			destinationHost = strings.ToLower(strings.TrimSpace(request.URL.Hostname()))
		}
		if destinationHost == "" {
			host, _, splitErr := net.SplitHostPort(request.Host)
			if splitErr == nil {
				destinationHost = strings.ToLower(strings.TrimSpace(host))
			} else {
				destinationHost = strings.ToLower(strings.TrimSpace(request.Host))
			}
		}
	}
	routeKind := s.routeKind(request)
	key := stage + "\x00" + errorClass + "\x00" + destinationHost + "\x00" + routeKind
	now := time.Now()
	s.diagnosticMu.Lock()
	last := s.diagnosticLast[key]
	if !last.IsZero() && now.Sub(last) < 10*time.Second {
		s.diagnosticMu.Unlock()
		return
	}
	s.diagnosticLast[key] = now
	s.diagnosticMu.Unlock()
	zap.L().Warn(
		"network route failed",
		zap.Uint64("networkGeneration", s.generation),
		zap.String("networkSurface", "gateway"),
		zap.String("networkRoute", routeKind),
		zap.String("networkStage", stage),
		zap.String("destinationHost", destinationHost),
		zap.String("networkErrorClass", errorClass),
	)
}

func (s *routeState) routeKind(request *http.Request) string {
	if request != nil && request.URL != nil && shouldBypassURL(request.URL, s.config.NoProxy) {
		return "direct-bypass"
	}
	switch s.config.Mode {
	case settings.ProxyModeNone:
		return "direct"
	case settings.ProxyModeSystem:
		return "system-native"
	case settings.ProxyModeManual:
		return "manual-" + s.config.Scheme.String()
	default:
		return "unknown"
	}
}

func copyHeaders(destination, source http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func validateConfig(config Config) error {
	if config.Timeout < 0 {
		return errors.New("proxy timeout cannot be negative")
	}
	switch config.Mode {
	case settings.ProxyModeNone, settings.ProxyModeSystem:
		return nil
	case settings.ProxyModeManual:
		if strings.TrimSpace(config.Host) == "" {
			return errors.New("manual proxy host is required")
		}
		if config.Port <= 0 || config.Port > 65535 {
			return errors.New("manual proxy port is invalid")
		}
		switch config.Scheme {
		case settings.ProxySchemeHTTP, settings.ProxySchemeHTTPS, settings.ProxySchemeSocks5:
		default:
			return errors.New("manual proxy scheme is invalid")
		}
		if config.Scheme == settings.ProxySchemeSocks5 && (len(config.Username) > 255 || len(config.Password) > 255) {
			return errors.New("SOCKS5 credentials are too long")
		}
		return nil
	default:
		return errors.New("proxy mode is invalid")
	}
}

func cloneConfig(config Config) Config {
	config.Host = strings.TrimSpace(config.Host)
	config.NoProxy = append([]string(nil), config.NoProxy...)
	return config
}

func parseWindowsProxyForScheme(proxyString, targetScheme string) (*url.URL, error) {
	proxyString = strings.TrimSpace(proxyString)
	if proxyString == "" {
		return nil, nil
	}
	if strings.Contains(proxyString, "=") {
		entries := make(map[string]string)
		for _, part := range strings.Split(proxyString, ";") {
			key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
			if ok && strings.TrimSpace(value) != "" {
				entries[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
			}
		}
		key := strings.ToLower(targetScheme)
		if key == "ws" {
			key = "http"
		} else if key == "wss" {
			key = "https"
		}
		selected := entries[key]
		if selected == "" && key == "https" {
			selected = entries["http"]
		}
		if selected == "" {
			if socks := entries["socks"]; socks != "" {
				selected = "socks5://" + socks
			}
		}
		proxyString = selected
	}
	if proxyString == "" {
		return nil, nil
	}
	if !strings.Contains(proxyString, "://") {
		proxyString = "http://" + proxyString
	}
	parsed, err := url.Parse(proxyString)
	if err != nil {
		return nil, errors.New("invalid Windows proxy configuration")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("invalid Windows proxy configuration")
	}
	return parsed, nil
}
