package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"
)

const routeForwardTransportLimit = 128

// roundTrip keeps the logical request URL intact for validation and NoProxy;
// system PAC canonicalizes it to the origin shared by opaque CONNECT users.
// Trusted/general proxy routes delegate the logical hostname to the selected
// proxy after a local loopback-alias check. This is required for the common
// Windows setup where a loopback proxy owns working remote DNS while the host
// resolver returns a filtered or poisoned non-loopback answer.
// public-untrusted routes remain locally resolved, validated, and pinned in
// roundTripPublic.
func (s *routeState) roundTrip(request *http.Request, forceDirect bool) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("route request URL is missing")
	}
	if forceDirect {
		return s.direct.RoundTrip(request)
	}
	proxyURL, err := s.proxyForLogicalURL(request.Context(), request.URL)
	if err != nil {
		return nil, err
	}
	if proxyURL == nil {
		return s.direct.RoundTrip(request)
	}
	logicalAddress, err := logicalRouteAddress(request.URL)
	if err != nil {
		return nil, err
	}
	transport, err := s.forwardTransport(logicalAddress, nil, proxyURL, false, true)
	if err != nil {
		return nil, err
	}
	return transport.RoundTrip(request)
}

func (s *routeState) roundTripPublic(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("public route request URL is missing")
	}
	logicalAddress, err := logicalRouteAddress(request.URL)
	if err != nil {
		return nil, err
	}
	pinned, err := resolveAndPinPublicAddresses(request.Context(), "tcp", logicalAddress)
	if err != nil {
		return nil, err
	}
	proxyURL, err := s.proxyForLogicalURL(request.Context(), request.URL)
	if err != nil {
		return nil, err
	}
	transport, err := s.forwardTransport(logicalAddress, pinned, proxyURL, true, false)
	if err != nil {
		return nil, err
	}
	return transport.RoundTrip(request)
}

func (s *routeState) proxyForLogicalURL(ctx context.Context, target *url.URL) (*url.URL, error) {
	if target == nil {
		return nil, errors.New("logical route URL is missing")
	}
	copy := *target
	copy.User = nil
	copy.Fragment = ""
	request := (&http.Request{Method: http.MethodConnect, URL: &copy, Host: copy.Host}).WithContext(ctx)
	return s.proxyForRequest(request)
}

func logicalRouteAddress(target *url.URL) (string, error) {
	if target == nil || strings.TrimSpace(target.Hostname()) == "" {
		return "", errors.New("logical route URL requires an authority")
	}
	port := target.Port()
	if port == "" {
		switch strings.ToLower(strings.TrimSpace(target.Scheme)) {
		case "http", "ws":
			port = "80"
		case "https", "wss":
			port = "443"
		default:
			return "", fmt.Errorf("unsupported logical route scheme %q", target.Scheme)
		}
	}
	if _, err := parsePort(port); err != nil {
		return "", err
	}
	host, err := canonicalRouteHostname(target.Hostname())
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, port), nil
}

func canonicalRouteHostname(raw string) (string, error) {
	host := strings.TrimSuffix(strings.TrimSpace(strings.Trim(raw, "[]")), ".")
	if host == "" || strings.ContainsAny(host, "\r\n\x00") {
		return "", errors.New("route hostname is invalid")
	}
	base, zone := host, ""
	if index := strings.LastIndex(base, "%"); index >= 0 {
		base, zone = base[:index], base[index+1:]
	}
	if ip := net.ParseIP(base); ip != nil {
		normalized := ip.String()
		if zone != "" {
			normalized += "%" + zone
		}
		return normalized, nil
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil || strings.TrimSpace(ascii) == "" {
		return "", errors.New("route hostname is not a valid DNS name")
	}
	return strings.ToLower(ascii), nil
}

func (s *routeState) dialLogicalRoute(ctx context.Context, network, address string, targetURL *url.URL, public bool) (net.Conn, error) {
	logicalAddress, err := logicalRouteAddress(targetURL)
	if err != nil {
		return nil, err
	}
	if !sameCanonicalNetworkTarget(logicalAddress, address) {
		return nil, errors.New("logical route URL does not match the requested authority")
	}
	if public {
		return s.dialPublicURL(ctx, network, address, targetURL)
	}
	proxyURL, err := s.proxyForLogicalURL(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	if proxyURL == nil {
		return s.dialDirect(ctx, network, address)
	}
	return s.dialTrustedProxyRoute(ctx, network, logicalAddress, proxyURL)
}

func (s *routeState) forwardTransport(
	logicalAddress string,
	pinned []string,
	proxyURL *url.URL,
	public bool,
	delegateProxyDNS bool,
) (*http.Transport, error) {
	if s == nil || s.direct == nil || (!delegateProxyDNS && len(pinned) == 0) || (delegateProxyDNS && proxyURL == nil) {
		return nil, errors.New("route transport is unavailable")
	}
	// Preserve the resolver's family/preference order for Happy-Eyeballs-style
	// fallback. Reordering validated literals for a cache key can otherwise
	// turn a healthy dual-stack route into a predictable delay.
	ordered := append([]string(nil), pinned...)
	proxyKey := "DIRECT"
	if proxyURL != nil {
		proxyKey = proxyURL.String()
	}
	key := fmt.Sprintf("%t\x00%t\x00%s\x00%s\x00%s", public, delegateProxyDNS, logicalAddress, strings.Join(ordered, ","), proxyKey)

	s.forwardMu.Lock()
	if s.forwardTransports == nil {
		s.forwardMu.Unlock()
		return nil, errors.New("proxy policy changed while preparing a route")
	}
	if existing := s.forwardTransports[key]; existing != nil {
		s.forwardMu.Unlock()
		return existing, nil
	}
	transport := s.direct.Clone()
	transport.Proxy = nil
	transport.Dial = nil
	transport.DialTLS = nil
	transport.DialTLSContext = nil
	transport.DialContext = func(ctx context.Context, network, requestedAddress string) (net.Conn, error) {
		if !sameCanonicalNetworkTarget(requestedAddress, logicalAddress) {
			return nil, errors.New("route transport refused an unexpected authority")
		}
		if delegateProxyDNS {
			return s.dialTrustedProxyRoute(ctx, network, logicalAddress, proxyURL)
		}
		return s.dialPinnedRoute(ctx, network, ordered, proxyURL)
	}
	var evicted *http.Transport
	if len(s.forwardTransports) >= routeForwardTransportLimit {
		for evictedKey, candidate := range s.forwardTransports {
			delete(s.forwardTransports, evictedKey)
			evicted = candidate
			break
		}
	}
	s.forwardTransports[key] = transport
	s.forwardMu.Unlock()
	if evicted != nil {
		evicted.CloseIdleConnections()
	}
	return transport, nil
}

// dialTrustedProxyRoute verifies the host's local answer is not a loopback
// alias, but deliberately does not use that answer as the proxy tunnel target.
// A user-selected manual proxy (or the OS-selected system proxy) frequently has
// the authoritative working DNS view, especially on Windows networks where the
// local resolver returns a filtered or poisoned non-loopback answer. A failed
// local lookup still fails closed because the loopback-alias check could not be
// completed. Literal targets stay pinned because delegating an IP literal
// cannot improve DNS compatibility and would weaken family checks.
func (s *routeState) dialTrustedProxyRoute(ctx context.Context, network, logicalAddress string, proxyURL *url.URL) (net.Conn, error) {
	if proxyURL == nil {
		return nil, errors.New("trusted proxy route requires an upstream proxy")
	}
	pinned, _, err := s.resolveAndPinDirectAddresses(ctx, network, logicalAddress)
	if err != nil {
		return nil, err
	}
	host, _, err := net.SplitHostPort(logicalAddress)
	if err != nil {
		return nil, err
	}
	if net.ParseIP(strings.Trim(host, "[]")) != nil {
		return s.dialPinnedRoute(ctx, network, pinned, proxyURL)
	}
	return s.dialProxyTarget(ctx, logicalAddress, proxyURL)
}

func (s *routeState) dialProxyTarget(ctx context.Context, target string, proxyURL *url.URL) (net.Conn, error) {
	switch strings.ToLower(strings.TrimSpace(proxyURL.Scheme)) {
	case "http", "https":
		return dialHTTPConnectProxyWithDialer(ctx, proxyURL, target, s.config.Timeout, s.dialDirect)
	case "socks5", "socks5h":
		username, password := "", ""
		if proxyURL.User != nil {
			username = proxyURL.User.Username()
			password, _ = proxyURL.User.Password()
		}
		return dialSOCKS5WithDialer(ctx, proxyURL.Host, target, username, password, s.config.Timeout, s.dialDirect)
	default:
		return nil, fmt.Errorf("unsupported upstream proxy scheme %q", proxyURL.Scheme)
	}
}

func (s *routeState) dialPinnedRoute(ctx context.Context, network string, pinned []string, proxyURL *url.URL) (net.Conn, error) {
	return raceRouteConnections(ctx, pinned, func(candidateContext context.Context, candidate string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(candidate)
		if err != nil {
			return nil, err
		}
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if ip == nil || !networkAllowsIP(network, ip) {
			return nil, errors.New("pinned address does not match the requested network")
		}
		if proxyURL == nil {
			return s.dialDirect(candidateContext, network, candidate)
		}
		return s.dialProxyTarget(candidateContext, candidate, proxyURL)
	})
}

func raceRouteConnections(
	ctx context.Context,
	addresses []string,
	dial func(context.Context, string) (net.Conn, error),
) (net.Conn, error) {
	if len(addresses) == 0 || dial == nil {
		return nil, errors.New("route has no pinned destination")
	}
	raceContext, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		connection net.Conn
		err        error
	}
	results := make(chan result, len(addresses))
	for index, address := range addresses {
		index, address := index, address
		go func() {
			if index > 0 {
				timer := time.NewTimer(time.Duration(index) * publicDialFallbackDelay)
				defer timer.Stop()
				select {
				case <-raceContext.Done():
					results <- result{err: raceContext.Err()}
					return
				case <-timer.C:
				}
			}
			connection, err := dial(raceContext, address)
			results <- result{connection: connection, err: err}
		}()
	}
	var lastErr error
	for completed := 0; completed < len(addresses); completed++ {
		result := <-results
		if result.err != nil {
			lastErr = result.err
			continue
		}
		cancel()
		remaining := len(addresses) - completed - 1
		if remaining > 0 {
			go func() {
				for range remaining {
					late := <-results
					if late.connection != nil {
						_ = late.connection.Close()
					}
				}
			}()
		}
		return result.connection, nil
	}
	if lastErr == nil {
		lastErr = errors.New("route connection failed")
	}
	return nil, lastErr
}

type routeStateRoundTripper struct{ state *routeState }

func (transport *routeStateRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport == nil || transport.state == nil || request == nil {
		return nil, errors.New("route transport is unavailable")
	}
	ctx, cancel := transport.state.generationContext(request.Context())
	response, err := transport.state.roundTrip(request.Clone(ctx), false)
	return bindRouteResponseContext(response, err, cancel)
}

func (transport *routeStateRoundTripper) CloseIdleConnections() {
	if transport != nil && transport.state != nil {
		transport.state.transport.CloseIdleConnections()
		transport.state.direct.CloseIdleConnections()
	}
}

type managerPublicRoundTripper struct{ manager *Manager }

func (transport *managerPublicRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport == nil || transport.manager == nil || request == nil {
		return nil, errors.New("public route transport is unavailable")
	}
	transport.manager.mu.RLock()
	if transport.manager.closed || transport.manager.gateway == nil {
		transport.manager.mu.RUnlock()
		return nil, errors.New("proxy manager is closed")
	}
	state := transport.manager.gateway.active.Load()
	transport.manager.mu.RUnlock()
	if state == nil {
		return nil, errors.New("proxy manager is closed")
	}
	ctx, cancel := state.generationContext(request.Context())
	response, err := state.roundTripPublic(request.Clone(ctx))
	return bindRouteResponseContext(response, err, cancel)
}

func (*managerPublicRoundTripper) CloseIdleConnections() {}

func bindRouteResponseContext(response *http.Response, err error, cancel context.CancelFunc) (*http.Response, error) {
	if err != nil || response == nil {
		cancel()
		return response, err
	}
	if response.Body == nil {
		cancel()
		return response, nil
	}
	response.Body = &routeResponseBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

type routeResponseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (body *routeResponseBody) Read(buffer []byte) (int, error) {
	read, err := body.ReadCloser.Read(buffer)
	if err != nil {
		body.finish()
	}
	return read, err
}

func (body *routeResponseBody) Close() error {
	err := body.ReadCloser.Close()
	body.finish()
	return err
}

func (body *routeResponseBody) finish() {
	body.once.Do(body.cancel)
}
