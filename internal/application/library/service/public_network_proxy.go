package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"
	"xiadown/internal/application/networkpolicy"
)

const (
	publicProxyDNSDeadline    = 5 * time.Second
	publicProxyDialDeadline   = 10 * time.Second
	publicProxyCloseDeadline  = 2 * time.Second
	publicProxyMaxConnections = 64
)

// publicNetworkProxy is a per-operation enforcing proxy. It is not merely a
// preflight validator: every TCP connection is resolved, checked and then
// dialled by its selected IP, so DNS rebinding cannot swap in a private address
// after validation. Redirects are issued as new proxy requests and therefore
// pass through the same boundary.
type publicNetworkProxy struct {
	resolver    networkpolicy.Resolver
	managedDial func(context.Context, string, string, *url.URL) (net.Conn, error)
	listener    net.Listener
	server      *http.Server
	transport   *http.Transport
	semaphore   chan struct{}
	done        chan struct{}

	mu      sync.Mutex
	tunnels map[net.Conn]struct{}
	closed  bool
}

func startPublicNetworkProxy(ctx context.Context, providers ...any) (*publicNetworkProxy, error) {
	var managedDial func(context.Context, string, string, *url.URL) (net.Conn, error)
	for _, provider := range providers {
		if managed, ok := provider.(interface {
			PublicDialURLContext(context.Context, string, string, *url.URL) (net.Conn, error)
		}); ok && managed != nil {
			managedDial = managed.PublicDialURLContext
			break
		}
	}
	if managedDial == nil {
		return nil, errors.New("restricted network proxy requires the managed App route")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start restricted network proxy: %w", err)
	}
	proxy := &publicNetworkProxy{
		resolver:    net.DefaultResolver,
		managedDial: managedDial,
		listener:    listener,
		semaphore:   make(chan struct{}, publicProxyMaxConnections),
		done:        make(chan struct{}),
		tunnels:     make(map[net.Conn]struct{}),
	}
	proxy.transport = &http.Transport{
		Proxy:                 nil,
		DialContext:           proxy.dialValidated,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          publicProxyMaxConnections,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   publicProxyDialDeadline,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	proxy.server = &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	go func() {
		_ = proxy.server.Serve(listener)
	}()
	go func() {
		select {
		case <-ctx.Done():
			proxy.Close()
		case <-proxy.done:
		}
	}()
	return proxy, nil
}

func (proxy *publicNetworkProxy) URL() string {
	if proxy == nil || proxy.listener == nil {
		return ""
	}
	return "http://" + proxy.listener.Addr().String()
}

func (proxy *publicNetworkProxy) Close() {
	if proxy == nil {
		return
	}
	proxy.mu.Lock()
	if proxy.closed {
		proxy.mu.Unlock()
		return
	}
	proxy.closed = true
	close(proxy.done)
	tunnels := make([]net.Conn, 0, len(proxy.tunnels))
	for connection := range proxy.tunnels {
		tunnels = append(tunnels, connection)
	}
	proxy.mu.Unlock()

	for _, connection := range tunnels {
		_ = connection.Close()
	}
	if proxy.transport != nil {
		proxy.transport.CloseIdleConnections()
	}
	if proxy.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), publicProxyCloseDeadline)
		_ = proxy.server.Shutdown(shutdownCtx)
		cancel()
		_ = proxy.server.Close()
	}
	if proxy.listener != nil {
		_ = proxy.listener.Close()
	}
}

func (proxy *publicNetworkProxy) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	select {
	case proxy.semaphore <- struct{}{}:
		defer func() { <-proxy.semaphore }()
	default:
		http.Error(w, "restricted proxy connection limit reached", http.StatusServiceUnavailable)
		return
	}
	if request.Method == http.MethodConnect {
		proxy.serveConnect(w, request)
		return
	}
	proxy.serveHTTP(w, request)
}

func (proxy *publicNetworkProxy) serveHTTP(w http.ResponseWriter, request *http.Request) {
	if request.URL == nil || !request.URL.IsAbs() {
		http.Error(w, "absolute target URL required", http.StatusBadRequest)
		return
	}
	parsed, err := networkpolicy.ValidatePublicHTTPURL(request.URL.String())
	if err != nil {
		http.Error(w, "destination blocked", statusForProxyError(err))
		return
	}
	outbound := request.Clone(request.Context())
	urlCopy := *parsed
	outbound.URL = &urlCopy
	outbound.RequestURI = ""
	outbound.Host = parsed.Host
	removeProxyHopHeaders(outbound.Header)
	logicalContext := context.WithValue(outbound.Context(), publicProxyLogicalURLKey{}, parsed)
	response, err := proxy.transport.RoundTrip(outbound.Clone(logicalContext))
	if err != nil {
		http.Error(w, "destination unavailable", statusForProxyError(err))
		return
	}
	defer response.Body.Close()
	copyHTTPHeader(w.Header(), response.Header)
	removeProxyHopHeaders(w.Header())
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (proxy *publicNetworkProxy) serveConnect(w http.ResponseWriter, request *http.Request) {
	host, port, err := splitProxyTarget(request.Host, "443")
	if err != nil {
		http.Error(w, "invalid CONNECT target", http.StatusBadRequest)
		return
	}
	logicalURL := &url.URL{Scheme: "https", Host: net.JoinHostPort(host, port)}
	upstream, err := proxy.dialPublicHost(request.Context(), "tcp", host, port, logicalURL)
	if err != nil {
		http.Error(w, "destination blocked", statusForProxyError(err))
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "CONNECT is unsupported", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	proxy.trackTunnel(client, true)
	proxy.trackTunnel(upstream, true)
	defer func() {
		proxy.trackTunnel(client, false)
		proxy.trackTunnel(upstream, false)
		_ = client.Close()
		_ = upstream.Close()
	}()
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}

	done := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(upstream, buffered)
		closePublicProxyWrite(upstream)
		done <- struct{}{}
	}()
	_, _ = io.Copy(client, upstream)
	closePublicProxyWrite(client)
	select {
	case <-done:
	case <-request.Context().Done():
	}
}

func closePublicProxyWrite(connection net.Conn) {
	if writer, ok := connection.(interface{ CloseWrite() error }); ok {
		_ = writer.CloseWrite()
	}
}

func (proxy *publicNetworkProxy) dialValidated(ctx context.Context, network string, address string) (net.Conn, error) {
	host, port, err := splitProxyTarget(address, "")
	if err != nil {
		return nil, err
	}
	logicalURL, _ := ctx.Value(publicProxyLogicalURLKey{}).(*url.URL)
	if logicalURL == nil {
		logicalURL = &url.URL{Scheme: "https", Host: net.JoinHostPort(host, port)}
	}
	return proxy.dialPublicHost(ctx, network, host, port, logicalURL)
}

type publicProxyLogicalURLKey struct{}

func (proxy *publicNetworkProxy) dialPublicHost(ctx context.Context, network string, host string, port string, logicalURL *url.URL) (net.Conn, error) {
	validatedLogicalURL, err := networkpolicy.ValidatePublicHTTPURL(logicalURL.String())
	if err != nil {
		return nil, err
	}
	logicalAddress, err := canonicalPublicProxyAuthority(host, port)
	if err != nil {
		return nil, err
	}
	validatedPort := validatedLogicalURL.Port()
	if validatedLogicalURL.Port() == "" {
		validatedPort = "80"
		if strings.EqualFold(validatedLogicalURL.Scheme, "https") {
			validatedPort = "443"
		}
	}
	validatedAddress, err := canonicalPublicProxyAuthority(validatedLogicalURL.Hostname(), validatedPort)
	if err != nil || logicalAddress != validatedAddress {
		return nil, errors.New("restricted proxy logical URL does not match its dial authority")
	}
	resolveCtx, cancel := context.WithTimeout(ctx, publicProxyDNSDeadline)
	_, err = networkpolicy.ResolvePublicIPs(resolveCtx, proxy.resolver, host)
	cancel()
	if err != nil {
		return nil, err
	}
	// Preserve the original hostname for NoProxy/PAC policy, but only after
	// this enforcing layer has proved that every current DNS answer is public.
	// The managed route resolves and validates again immediately before dialing,
	// which closes the DNS-rebinding window while still sharing App egress.
	if proxy.managedDial == nil {
		return nil, errors.New("restricted network proxy requires the managed App route")
	}
	return proxy.managedDial(ctx, network, logicalAddress, validatedLogicalURL)
}

func canonicalPublicProxyAuthority(host, port string) (string, error) {
	host = strings.TrimSuffix(strings.TrimSpace(strings.Trim(host, "[]")), ".")
	if host == "" {
		return "", errors.New("restricted proxy hostname is invalid")
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else {
		ascii, err := idna.Lookup.ToASCII(host)
		if err != nil || ascii == "" {
			return "", errors.New("restricted proxy hostname is invalid")
		}
		host = strings.ToLower(ascii)
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return "", errors.New("restricted proxy port is invalid")
	}
	return net.JoinHostPort(host, strconv.FormatUint(parsedPort, 10)), nil
}

func splitProxyTarget(target string, defaultPort string) (string, string, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return "", "", errors.New("empty target")
	}
	host, port, err := net.SplitHostPort(trimmed)
	if err != nil && defaultPort != "" && !strings.Contains(err.Error(), "missing port in address") {
		return "", "", err
	}
	if err != nil {
		host = strings.Trim(trimmed, "[]")
		port = defaultPort
	}
	if strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return "", "", errors.New("target host and port are required")
	}
	return host, port, nil
}

func statusForProxyError(err error) int {
	if errors.Is(err, networkpolicy.ErrDestinationBlocked) {
		return http.StatusForbidden
	}
	return http.StatusBadGateway
}

func removeProxyHopHeaders(header http.Header) {
	for _, connectionHeader := range append(header.Values("Connection"), header.Values("Proxy-Connection")...) {
		for _, token := range strings.Split(connectionHeader, ",") {
			if key := strings.TrimSpace(token); key != "" {
				header.Del(key)
			}
		}
	}
	for _, key := range []string{
		"Connection", "Proxy-Connection", "Proxy-Authorization", "Proxy-Authenticate",
		"Keep-Alive", "TE", "Trailer", "Transfer-Encoding", "Upgrade",
		"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP",
	} {
		header.Del(key)
	}
}

func copyHTTPHeader(destination http.Header, source http.Header) {
	for key, values := range source {
		for _, value := range values {
			destination.Add(key, value)
		}
	}
}

func (proxy *publicNetworkProxy) trackTunnel(connection net.Conn, add bool) {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if add {
		if proxy.closed {
			_ = connection.Close()
			return
		}
		proxy.tunnels[connection] = struct{}{}
		return
	}
	delete(proxy.tunnels, connection)
}

func newRestrictedAuxiliaryHTTPClient(proxyURL string) (*http.Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" {
		return nil, errors.New("invalid restricted proxy URL")
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(parsed),
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   publicProxyDialDeadline,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			_, err := networkpolicy.ValidatePublicHTTPURL(request.URL.String())
			return err
		},
	}, nil
}
