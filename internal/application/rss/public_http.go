package rss

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xiadown/internal/application/networkpolicy"
)

const (
	rssDNSDeadline                        = 5 * time.Second
	rssRemoteDialDeadline                 = 10 * time.Second
	rssRemoteTLSHandshakeDeadline         = 10 * time.Second
	rssRemoteResponseHeaderDeadline       = 20 * time.Second
	rssRemoteMaxResponseHeaderBytes int64 = 128 << 10
)

type remoteResourceTransportTimeouts struct {
	dial           time.Duration
	tlsHandshake   time.Duration
	responseHeader time.Duration
}

var defaultRemoteResourceTransportTimeouts = remoteResourceTransportTimeouts{
	dial:           rssRemoteDialDeadline,
	tlsHandshake:   rssRemoteTLSHandshakeDeadline,
	responseHeader: rssRemoteResponseHeaderDeadline,
}

// NewRemoteResourceHTTPClient creates the cookie-free client used by RSS
// image/media proxy handlers. It deliberately has no client-wide timeout:
// handlers apply different image/media body lifetimes, while this client
// always enforces bounded dial, TLS handshake and response-header phases.
func NewRemoteResourceHTTPClient(provider HTTPClientProvider) *http.Client {
	return newRemoteResourceHTTPClient(provider, net.DefaultResolver)
}

func newRemoteResourceHTTPClient(provider HTTPClientProvider, resolver networkpolicy.Resolver) *http.Client {
	return newRemoteResourceHTTPClientWithTimeouts(provider, resolver, defaultRemoteResourceTransportTimeouts)
}

func newRemoteResourceHTTPClientWithTimeouts(
	provider HTTPClientProvider,
	resolver networkpolicy.Resolver,
	timeouts remoteResourceTransportTimeouts,
) *http.Client {
	base := http.DefaultClient
	if provider != nil {
		provided := provider.HTTPClient()
		if provided != nil {
			base = provided
		}
	}
	clone := *base
	if managedDialer, ok := provider.(publicURLRouteDialer); ok {
		clone.Transport = &managedPublicHTTPTransport{
			dialer: managedDialer, timeouts: normalizedRemoteResourceTransportTimeouts(timeouts),
		}
	} else if _, allowed := provider.(pinnedPublicRouteTestSeam); allowed {
		clone.Transport = newPublicHTTPTransportWithTimeouts(base.Transport, resolver, timeouts)
	} else {
		clone.Transport = rejectedPublicHTTPTransport{err: errors.New("RSS public request requires the managed App route")}
	}
	clone.Jar = nil
	clone.Timeout = 0
	clone.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many RSS resource redirects")
		}
		safeRefererOrigin := ""
		if len(via) > 0 && via[0] != nil {
			safeRefererOrigin = canonicalPublicHTTPOrigin(via[0].Header.Get("Referer"))
		}
		// net/http synthesizes a Referer while following redirects. Resource
		// URLs can contain signed query parameters, so keep every hop as
		// credential-free even though the proxy endpoint itself is authenticated.
		// A server-derived publisher origin may be restored only for redirects
		// which remain on the original resource's registrable site.
		for _, header := range []string{
			"Authorization", "Cookie", "Origin", "Referer", "Proxy-Authorization",
			"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-IP",
		} {
			request.Header.Del(header)
		}
		if safeRefererOrigin != "" && redirectTargetRelatedToInitialResource(request.URL, via) {
			request.Header.Set("Referer", safeRefererOrigin)
		}
		if len(via) > 0 && !redirectStayedOnInitialValidatorURL(request.URL, via) {
			// Validators are scoped to one representation at one origin. Carrying
			// them to a different host can disclose opaque ETags and allows a
			// redirector to probe a user's cached upstream state. Compare every hop
			// with the initial validator origin: net/http can rebuild later redirect
			// headers from the first request. Range is safe and intentionally
			// retained so media redirects remain seekable.
			for _, header := range []string{"If-None-Match", "If-Modified-Since", "If-Range"} {
				request.Header.Del(header)
			}
		}
		_, err := networkpolicy.ValidatePublicHTTPURL(request.URL.String())
		return err
	}
	return &clone
}

// publicURLRouteDialer is implemented by the process-wide proxy manager. RSS
// owns the request URL and its stricter phase/header limits, while the manager
// remains authoritative for generation, PAC/NoProxy, proxy auth, DNS pinning,
// SSRF checks, and synchronous route revocation.
type publicURLRouteDialer interface {
	PublicDialURLContext(context.Context, string, string, *url.URL) (net.Conn, error)
}

// pinnedPublicRouteTestSeam is implemented only by package tests which need to
// exercise the legacy SSRF/pinning transport in isolation. Production callers
// cannot implement this unexported marker from another package; they must
// provide PublicDialURLContext so RSS can never silently leave the App route.
type pinnedPublicRouteTestSeam interface {
	allowPinnedPublicRouteTestSeam()
}

type managedPublicHTTPTransport struct {
	dialer   publicURLRouteDialer
	timeouts remoteResourceTransportTimeouts
}

func (transport *managedPublicHTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport == nil || transport.dialer == nil || request == nil || request.URL == nil {
		return nil, errors.New("managed RSS public route is unavailable")
	}
	logicalURL, err := networkpolicy.ValidatePublicHTTPURL(request.URL.String())
	if err != nil {
		return nil, err
	}
	logicalCopy := *logicalURL
	timeouts := normalizedRemoteResourceTransportTimeouts(transport.timeouts)
	base := &http.Transport{
		Proxy:                  nil,
		ForceAttemptHTTP2:      true,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    timeouts.tlsHandshake,
		ResponseHeaderTimeout:  timeouts.responseHeader,
		MaxResponseHeaderBytes: rssRemoteMaxResponseHeaderBytes,
		ExpectContinueTimeout:  time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return transport.dialer.PublicDialURLContext(ctx, network, address, &logicalCopy)
		},
	}
	forceRemoteResourceTransportTimeouts(base, timeouts)
	return roundTripWithEphemeralTransport(base, request)
}

// publicHTTPTransport enforces the public-network policy at the connection
// boundary. A URL-only preflight is insufficient because DNS can change
// between validation and DialContext, and an HTTP proxy would otherwise
// resolve the destination again on XiaDown's behalf.
type publicHTTPTransport struct {
	base     *http.Transport
	resolver networkpolicy.Resolver
	timeouts remoteResourceTransportTimeouts
}

func newPublicHTTPTransport(base http.RoundTripper, resolver networkpolicy.Resolver) http.RoundTripper {
	return newPublicHTTPTransportWithTimeouts(base, resolver, defaultRemoteResourceTransportTimeouts)
}

func newPublicHTTPTransportWithTimeouts(
	base http.RoundTripper,
	resolver networkpolicy.Resolver,
	timeouts remoteResourceTransportTimeouts,
) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	transport, ok := base.(*http.Transport)
	if !ok || transport == nil {
		// A generic RoundTripper does not expose its connection boundary, so
		// wrapping it with another URL preflight would retain the rebinding gap.
		return rejectedPublicHTTPTransport{err: errors.New("RSS HTTP transport cannot enforce the public-network policy")}
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &publicHTTPTransport{
		base: transport.Clone(), resolver: resolver,
		timeouts: normalizedRemoteResourceTransportTimeouts(timeouts),
	}
}

func (transport *publicHTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("RSS HTTP request URL is required")
	}
	parsed, err := networkpolicy.ValidatePublicHTTPURL(request.URL.String())
	if err != nil {
		return nil, err
	}
	if transport == nil || transport.base == nil {
		return nil, errors.New("RSS HTTP transport is unavailable")
	}
	if transport.base.DialTLS != nil || transport.base.DialTLSContext != nil {
		// A custom TLS dialer bypasses Transport.DialContext. There is no safe
		// way to preserve an opaque dialer while also proving which IP it used.
		return nil, errors.New("RSS HTTP transport cannot safely use a custom TLS dialer")
	}

	proxyURL, err := resolveRequestProxy(transport.base, request)
	if err != nil {
		return nil, fmt.Errorf("resolve RSS proxy: %w", err)
	}
	if proxyURL == nil {
		return transport.roundTripDirect(request)
	}
	return transport.roundTripViaProxy(request, parsed, proxyURL)
}

func (transport *publicHTTPTransport) roundTripDirect(request *http.Request) (*http.Response, error) {
	clone := transport.base.Clone()
	clone.Proxy = nil
	baseDial := transportDialContext(clone)
	clone.Dial = nil
	clone.DialContext = publicDialContext(transport.resolver, baseDial)
	forceRemoteResourceTransportTimeouts(clone, transport.timeouts)
	return roundTripWithEphemeralTransport(clone, request)
}

func (transport *publicHTTPTransport) roundTripViaProxy(request *http.Request, parsed *url.URL, proxyURL *url.URL) (*http.Response, error) {
	scheme := strings.ToLower(strings.TrimSpace(proxyURL.Scheme))
	switch scheme {
	case "http", "https":
		return transport.roundTripViaHTTPProxyTunnel(request, parsed, proxyURL)
	case "socks5", "socks5h":
		return transport.roundTripViaSOCKSProxy(request, parsed, proxyURL)
	default:
		return nil, fmt.Errorf("unsupported RSS proxy scheme %q", proxyURL.Scheme)
	}
}

func (transport *publicHTTPTransport) roundTripViaHTTPProxyTunnel(request *http.Request, parsed *url.URL, proxyURL *url.URL) (*http.Response, error) {
	// Always tunnel, including plain HTTP feeds. Sending an absolute-form HTTP
	// request would let a compliant proxy treat its authority as canonical and
	// resolve the attacker-controlled hostname itself. If a proxy forbids
	// CONNECT to the feed port, fail closed rather than re-open that DNS path.
	clone := transport.base.Clone()
	baseDial := transportDialContext(clone)
	clone.Proxy = nil
	clone.Dial = nil
	clone.DialContext = publicProxyTunnelDialContext(transport.resolver, baseDial, transport.base, proxyURL)
	if strings.EqualFold(parsed.Scheme, "https") {
		originTLS := cloneTLSConfig(clone.TLSClientConfig)
		originTLS.ServerName = strings.TrimSuffix(parsed.Hostname(), ".")
		clone.TLSClientConfig = originTLS
	}
	forceRemoteResourceTransportTimeouts(clone, transport.timeouts)
	return roundTripWithEphemeralTransport(clone, request)
}

func (transport *publicHTTPTransport) roundTripViaSOCKSProxy(request *http.Request, parsed *url.URL, proxyURL *url.URL) (*http.Response, error) {
	resolveCtx, cancel := context.WithTimeout(request.Context(), rssDNSDeadline)
	addresses, err := networkpolicy.ResolvePublicIPs(resolveCtx, transport.resolver, parsed.Hostname())
	cancel()
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, address := range addresses {
		clone, cloneErr := pinnedSOCKSProxyTransport(transport.base, parsed.Hostname(), proxyURL)
		if cloneErr != nil {
			return nil, cloneErr
		}
		forceRemoteResourceTransportTimeouts(clone, transport.timeouts)
		pinnedRequest := cloneRequestWithPinnedIP(request, parsed, address.IP)
		response, roundTripErr := roundTripWithEphemeralTransport(clone, pinnedRequest)
		if roundTripErr == nil {
			// Do not expose the internal pinned-IP URL. http.Client uses the
			// logical request URL for redirect resolution and cookie policy.
			response.Request = request
			return response, nil
		}
		lastErr = roundTripErr
	}
	if lastErr == nil {
		lastErr = errors.New("no public address was available for the RSS destination")
	}
	return nil, lastErr
}

func resolveRequestProxy(transport *http.Transport, request *http.Request) (*url.URL, error) {
	if transport.Proxy == nil {
		return nil, nil
	}
	return transport.Proxy(request)
}

func normalizedRemoteResourceTransportTimeouts(timeouts remoteResourceTransportTimeouts) remoteResourceTransportTimeouts {
	if timeouts.dial <= 0 {
		timeouts.dial = rssRemoteDialDeadline
	}
	if timeouts.tlsHandshake <= 0 {
		timeouts.tlsHandshake = rssRemoteTLSHandshakeDeadline
	}
	if timeouts.responseHeader <= 0 {
		timeouts.responseHeader = rssRemoteResponseHeaderDeadline
	}
	return timeouts
}

func forceRemoteResourceTransportTimeouts(transport *http.Transport, timeouts remoteResourceTransportTimeouts) {
	if transport == nil {
		return
	}
	timeouts = normalizedRemoteResourceTransportTimeouts(timeouts)
	dial := transportDialContext(transport)
	transport.Dial = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialCtx, cancel := context.WithTimeout(ctx, timeouts.dial)
		defer cancel()
		return dial(dialCtx, network, address)
	}
	// Provider transports may leave these fields at zero (unbounded), or set
	// values intended for long downloads. RSS resources always cap each phase.
	transport.TLSHandshakeTimeout = cappedPositiveDuration(transport.TLSHandshakeTimeout, timeouts.tlsHandshake)
	transport.ResponseHeaderTimeout = cappedPositiveDuration(transport.ResponseHeaderTimeout, timeouts.responseHeader)
	if transport.MaxResponseHeaderBytes <= 0 || transport.MaxResponseHeaderBytes > rssRemoteMaxResponseHeaderBytes {
		transport.MaxResponseHeaderBytes = rssRemoteMaxResponseHeaderBytes
	}
}

func cappedPositiveDuration(value, maximum time.Duration) time.Duration {
	if value <= 0 || value > maximum {
		return maximum
	}
	return value
}

func sameHTTPOrigin(left, right *url.URL) bool {
	leftScheme, leftHost, leftPort, leftOK := normalizedHTTPOrigin(left)
	rightScheme, rightHost, rightPort, rightOK := normalizedHTTPOrigin(right)
	return leftOK && rightOK && leftScheme == rightScheme && leftHost == rightHost && leftPort == rightPort
}

func redirectStayedOnInitialValidatorURL(target *url.URL, via []*http.Request) bool {
	if len(via) == 0 || !sameHTTPValidatorURL(via[0].URL, target) {
		return false
	}
	for _, previous := range via[1:] {
		if previous == nil || !sameHTTPValidatorURL(via[0].URL, previous.URL) {
			return false
		}
	}
	return true
}

func sameHTTPValidatorURL(left, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	leftCopy, rightCopy := *left, *right
	leftCopy.Fragment = ""
	rightCopy.Fragment = ""
	leftScheme, leftHost, leftPort, leftOK := normalizedHTTPOrigin(&leftCopy)
	rightScheme, rightHost, rightPort, rightOK := normalizedHTTPOrigin(&rightCopy)
	if !leftOK || !rightOK || leftScheme != rightScheme || leftHost != rightHost || leftPort != rightPort {
		return false
	}
	leftCopy.Scheme, rightCopy.Scheme = leftScheme, rightScheme
	leftCopy.Host, rightCopy.Host = canonicalHTTPValidatorHost(leftHost, leftPort, leftScheme), canonicalHTTPValidatorHost(rightHost, rightPort, rightScheme)
	return leftCopy.String() == rightCopy.String()
}

func canonicalHTTPValidatorHost(host, port, scheme string) string {
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		if strings.Contains(host, ":") {
			return "[" + host + "]"
		}
		return host
	}
	return net.JoinHostPort(host, port)
}

func normalizedHTTPOrigin(value *url.URL) (scheme, host, port string, ok bool) {
	if value == nil {
		return "", "", "", false
	}
	scheme = strings.ToLower(strings.TrimSpace(value.Scheme))
	host = strings.ToLower(strings.TrimSpace(value.Hostname()))
	if host == "" {
		return "", "", "", false
	}
	port = value.Port()
	switch scheme {
	case "http":
		if port == "" {
			port = "80"
		}
	case "https":
		if port == "" {
			port = "443"
		}
	default:
		return "", "", "", false
	}
	return scheme, host, port, true
}

func transportDialContext(transport *http.Transport) func(context.Context, string, string) (net.Conn, error) {
	if transport.DialContext != nil {
		return transport.DialContext
	}
	if transport.Dial != nil {
		return func(_ context.Context, network, address string) (net.Conn, error) {
			return transport.Dial(network, address)
		}
	}
	dialer := &net.Dialer{Timeout: rssRemoteDialDeadline}
	return dialer.DialContext
}

func publicDialContext(resolver networkpolicy.Resolver, dial func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid RSS dial destination: %w", err)
		}
		resolveCtx, cancel := context.WithTimeout(ctx, rssDNSDeadline)
		addresses, err := networkpolicy.ResolvePublicIPs(resolveCtx, resolver, host)
		cancel()
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, resolved := range addresses {
			if network == "tcp4" && resolved.IP.To4() == nil {
				continue
			}
			if network == "tcp6" && resolved.IP.To4() != nil {
				continue
			}
			connection, dialErr := dial(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
			if dialErr == nil {
				return connection, nil
			}
			lastErr = dialErr
		}
		if lastErr == nil {
			lastErr = errors.New("no public address matched the RSS connection network")
		}
		return nil, lastErr
	}
}

func publicProxyTunnelDialContext(
	resolver networkpolicy.Resolver,
	dial func(context.Context, string, string) (net.Conn, error),
	base *http.Transport,
	proxyURL *url.URL,
) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid RSS proxy tunnel destination: %w", err)
		}
		resolveCtx, cancel := context.WithTimeout(ctx, rssDNSDeadline)
		addresses, err := networkpolicy.ResolvePublicIPs(resolveCtx, resolver, host)
		cancel()
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, resolved := range addresses {
			if network == "tcp4" && resolved.IP.To4() == nil {
				continue
			}
			if network == "tcp6" && resolved.IP.To4() != nil {
				continue
			}
			target := net.JoinHostPort(resolved.IP.String(), port)
			connection, dialErr := dialHTTPProxyTunnel(ctx, network, target, dial, base, proxyURL)
			if dialErr == nil {
				return connection, nil
			}
			lastErr = dialErr
		}
		if lastErr == nil {
			lastErr = errors.New("no public address matched the RSS proxy tunnel network")
		}
		return nil, lastErr
	}
}

func dialHTTPProxyTunnel(
	ctx context.Context,
	network string,
	target string,
	dial func(context.Context, string, string) (net.Conn, error),
	base *http.Transport,
	proxyURL *url.URL,
) (net.Conn, error) {
	proxyAddress, err := canonicalProxyAddress(proxyURL)
	if err != nil {
		return nil, err
	}
	connection, err := dial(ctx, network, proxyAddress)
	if err != nil {
		return nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = connection.Close()
		}
	}()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}

	if strings.EqualFold(proxyURL.Scheme, "https") {
		proxyTLS := cloneTLSConfig(base.TLSClientConfig)
		proxyTLS.ServerName = strings.TrimSuffix(proxyURL.Hostname(), ".")
		// CONNECT is deliberately HTTP/1.1. Origin ALPN is negotiated later,
		// independently, inside the established tunnel.
		proxyTLS.NextProtos = []string{"http/1.1"}
		tlsConnection := tls.Client(connection, proxyTLS)
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		connection = tlsConnection
	}

	header, err := proxyConnectHeader(ctx, base, proxyURL, target)
	if err != nil {
		return nil, err
	}
	connectRequest := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: target},
		Host:   target,
		Header: header,
	}
	if err := connectRequest.Write(connection); err != nil {
		return nil, err
	}
	response, reader, err := readRSSProxyConnectResponse(connection, connectRequest)
	if err != nil {
		return nil, err
	}
	if base.OnProxyConnectResponse != nil {
		if err := base.OnProxyConnectResponse(ctx, proxyURL, connectRequest, response); err != nil {
			_ = response.Body.Close()
			return nil, err
		}
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("RSS proxy CONNECT failed: %s", response.Status)
	}

	_ = connection.SetDeadline(time.Time{})
	closeOnError = false
	return &bufferedConn{Conn: connection, reader: reader}, nil
}

var errRSSProxyConnectResponseHeadersTooLarge = errors.New("RSS proxy CONNECT response headers exceed the safety limit")

// readRSSProxyConnectResponse applies the same response-header budget used for
// origin responses to the CONNECT handshake itself. The first reader can
// buffer tunnel bytes beyond the terminating empty line, so the returned reader
// replays the bounded header and then drains that original buffer before it
// reaches the connection again.
func readRSSProxyConnectResponse(connection io.Reader, request *http.Request) (*http.Response, *bufio.Reader, error) {
	if connection == nil {
		return nil, nil, io.EOF
	}
	source := bufio.NewReader(connection)
	var header bytes.Buffer
	fragmentedLine := false
	for {
		line, err := source.ReadSlice('\n')
		if int64(header.Len())+int64(len(line)) > rssRemoteMaxResponseHeaderBytes {
			return nil, nil, errRSSProxyConnectResponseHeadersTooLarge
		}
		_, _ = header.Write(line)
		if err != nil {
			if errors.Is(err, bufio.ErrBufferFull) {
				fragmentedLine = true
				continue
			}
			return nil, nil, err
		}
		if !fragmentedLine && (bytes.Equal(line, []byte("\r\n")) || bytes.Equal(line, []byte("\n"))) {
			break
		}
		fragmentedLine = false
	}
	reader := bufio.NewReader(io.MultiReader(bytes.NewReader(header.Bytes()), source))
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		return nil, nil, err
	}
	return response, reader, nil
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (connection *bufferedConn) Read(buffer []byte) (int, error) {
	return connection.reader.Read(buffer)
}

func canonicalProxyAddress(proxyURL *url.URL) (string, error) {
	if proxyURL == nil || strings.TrimSpace(proxyURL.Hostname()) == "" {
		return "", errors.New("RSS proxy hostname is required")
	}
	port := proxyURL.Port()
	if port == "" {
		switch strings.ToLower(strings.TrimSpace(proxyURL.Scheme)) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", fmt.Errorf("unsupported RSS proxy scheme %q", proxyURL.Scheme)
		}
	}
	return net.JoinHostPort(proxyURL.Hostname(), port), nil
}

func proxyConnectHeader(ctx context.Context, base *http.Transport, proxyURL *url.URL, target string) (http.Header, error) {
	var header http.Header
	var err error
	if base.GetProxyConnectHeader != nil {
		header, err = base.GetProxyConnectHeader(ctx, proxyURL, target)
		if err != nil {
			return nil, err
		}
	} else {
		header = base.ProxyConnectHeader
	}
	if header == nil {
		header = make(http.Header)
	} else {
		header = header.Clone()
	}
	if proxyURL.User != nil {
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()
		credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		header.Set("Proxy-Authorization", "Basic "+credentials)
	}
	return header, nil
}

func pinnedSOCKSProxyTransport(base *http.Transport, originHostname string, proxyURL *url.URL) (*http.Transport, error) {
	if proxyURL != nil && proxyURL.User != nil {
		return nil, errors.New("authenticated RSS SOCKS requires the managed App route")
	}
	clone := base.Clone()
	clone.Proxy = http.ProxyURL(proxyURL)

	originTLS := cloneTLSConfig(base.TLSClientConfig)
	originTLS.ServerName = strings.TrimSuffix(originHostname, ".")
	clone.TLSClientConfig = originTLS

	return clone, nil
}

func cloneTLSConfig(config *tls.Config) *tls.Config {
	if config == nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return config.Clone()
}

func cloneRequestWithPinnedIP(
	request *http.Request,
	parsed *url.URL,
	ip net.IP,
) *http.Request {
	clone := request.Clone(request.Context())
	urlCopy := *parsed
	port := parsed.Port()
	if port == "" {
		if strings.EqualFold(parsed.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	urlCopy.Host = net.JoinHostPort(ip.String(), port)
	clone.URL = &urlCopy
	clone.Host = request.Host
	if clone.Host == "" {
		clone.Host = parsed.Host
	}
	return clone
}

func roundTripWithEphemeralTransport(transport *http.Transport, request *http.Request) (*http.Response, error) {
	response, err := transport.RoundTrip(request)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	response.Body = &closeIdleResponseBody{ReadCloser: response.Body, closeIdle: transport.CloseIdleConnections}
	return response, nil
}

type closeIdleResponseBody struct {
	io.ReadCloser
	closeIdle func()
}

func (body *closeIdleResponseBody) Close() error {
	err := body.ReadCloser.Close()
	if body.closeIdle != nil {
		body.closeIdle()
	}
	return err
}

type rejectedPublicHTTPTransport struct{ err error }

func (transport rejectedPublicHTTPTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, transport.err
}
