package proxy

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http/httpproxy"
)

var errNativeSystemProxyUnavailable = errors.New("native system proxy resolver is unavailable")

const nativeSystemProxyConcurrency = 16

var nativeSystemProxySlots = make(chan struct{}, nativeSystemProxyConcurrency)

// systemProxyResolver is owned by one network-policy generation. Most
// platforms resolve directly through a stateless desktop API, while Windows
// keeps a WinHTTP session here so PAC/WPAD discovery and script caches survive
// across all URL decisions in that generation.
type systemProxyResolver interface {
	Resolve(*url.URL) (*url.URL, error)
	Close()
}

type statelessSystemProxyResolver struct{}

func (statelessSystemProxyResolver) Resolve(target *url.URL) (*url.URL, error) {
	return platformSystemProxyURL(target)
}

func (statelessSystemProxyResolver) Close() {}

// platformSystemProxyURLContext makes synchronous PAC/WPAD platform APIs
// cancellable to their callers and bounds the native work that can outlive a
// cancelled request. The underlying public APIs are synchronous on some
// supported OS versions, so an in-flight native call may finish in the
// background, but it can no longer indefinitely retain a gateway request or
// create unbounded resolver goroutines.
func platformSystemProxyURLContext(ctx context.Context, target *url.URL) (*url.URL, error) {
	resolver := newPlatformSystemProxyResolver()
	defer resolver.Close()
	return systemProxyURLContext(ctx, resolver, target)
}

func systemProxyURLContext(ctx context.Context, resolver systemProxyResolver, target *url.URL) (*url.URL, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if resolver == nil {
		return nil, errors.New("system proxy resolver is unavailable")
	}
	if target == nil {
		return nil, errors.New("system proxy resolution requires an absolute URL")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case nativeSystemProxySlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// An HTTPS/WSS consumer enters the stable gateway as an opaque CONNECT and
	// cannot expose its encrypted path without TLS interception. Normalize every
	// system decision to the same canonical origin so backend, WebView, yt-dlp,
	// Settings probes, and managed browsers cannot silently select different PAC
	// routes for the same origin. App NoProxy matching happens before this step
	// and remains authority-scoped.
	canonicalTarget, err := canonicalSystemProxyTarget(target)
	if err != nil {
		<-nativeSystemProxySlots
		return nil, err
	}
	targetCopy := *canonicalTarget
	type result struct {
		proxyURL *url.URL
		err      error
	}
	results := make(chan result, 1)
	go func() {
		proxyURL, err := resolver.Resolve(&targetCopy)
		<-nativeSystemProxySlots
		results <- result{proxyURL: proxyURL, err: err}
	}()

	select {
	case resolved := <-results:
		return resolved.proxyURL, resolved.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func systemProxyDiagnosticContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 20*time.Second)
}

// firstSystemProxyCandidate preserves the order returned by the operating
// system. The gateway commits to the first decision only: DIRECT is never
// inferred by skipping an unsupported or malformed earlier candidate.
func firstSystemProxyCandidate(candidates []string) (*url.URL, error) {
	if len(candidates) == 0 {
		return nil, errors.New("system proxy resolver returned no decision")
	}
	return normalizeSystemProxyCandidate(candidates[0])
}

func normalizeSystemProxyCandidate(raw string) (*url.URL, error) {
	candidate, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, errors.New("system proxy resolver returned a malformed candidate")
	}
	scheme := strings.ToLower(candidate.Scheme)
	if scheme == "direct" {
		if candidate.Host != "" || (candidate.Path != "" && candidate.Path != "/") || candidate.RawQuery != "" || candidate.Fragment != "" {
			return nil, errors.New("system proxy resolver returned an invalid DIRECT decision")
		}
		return nil, nil
	}
	switch scheme {
	case "http", "https", "socks5", "socks5h":
		candidate.Scheme = scheme
	case "socks":
		// GIO and the XDG portal use the generic socks:// spelling for the
		// desktop SOCKS proxy. XiaDown's gateway speaks SOCKS5.
		candidate.Scheme = "socks5"
	default:
		return nil, errors.New("system proxy resolver returned an unsupported scheme")
	}
	if candidate.Hostname() == "" || (candidate.Path != "" && candidate.Path != "/") || candidate.RawQuery != "" || candidate.Fragment != "" {
		return nil, errors.New("system proxy resolver returned an invalid proxy endpoint")
	}
	if port := candidate.Port(); port != "" {
		if _, err := parsePort(port); err != nil {
			return nil, errors.New("system proxy resolver returned an invalid proxy port")
		}
	} else if candidate.Scheme == "socks5" || candidate.Scheme == "socks5h" {
		candidate.Host = net.JoinHostPort(candidate.Hostname(), "1080")
	}
	return candidate, nil
}

func canonicalSystemProxyTarget(target *url.URL) (*url.URL, error) {
	if target == nil || target.Hostname() == "" {
		return nil, errors.New("system proxy resolution requires an absolute URL")
	}
	canonical := *target
	canonical.User = nil
	canonical.Fragment = ""
	canonical.RawFragment = ""
	canonical.Path = "/"
	canonical.RawPath = ""
	canonical.RawQuery = ""
	canonical.ForceQuery = false
	defaultPort := ""
	switch strings.ToLower(canonical.Scheme) {
	case "http", "https":
		canonical.Scheme = strings.ToLower(canonical.Scheme)
	case "ws":
		canonical.Scheme = "http"
	case "wss":
		canonical.Scheme = "https"
	default:
		return nil, errors.New("system proxy resolution does not support this URL scheme")
	}
	if canonical.Scheme == "http" {
		defaultPort = "80"
	} else {
		defaultPort = "443"
	}
	host, err := canonicalRouteHostname(canonical.Hostname())
	if err != nil {
		return nil, errors.New("system proxy resolution requires a valid origin hostname")
	}
	port := canonical.Port()
	if port != "" {
		if _, err := parsePort(port); err != nil {
			return nil, errors.New("system proxy resolution requires a valid origin port")
		}
	}
	if port == "" || port == defaultPort {
		if strings.Contains(host, ":") {
			canonical.Host = "[" + host + "]"
		} else {
			canonical.Host = host
		}
	} else {
		canonical.Host = net.JoinHostPort(host, port)
	}
	return &canonical, nil
}

func systemProxyURLFromParts(kind, host string, port int, username, password string) (*url.URL, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "direct":
		return nil, nil
	case "http", "https", "socks", "socks5", "socks5h":
	default:
		return nil, errors.New("native system proxy resolver returned an unsupported decision")
	}
	if strings.TrimSpace(host) == "" || port < 1 || port > 65535 {
		return nil, errors.New("native system proxy resolver returned an invalid endpoint")
	}
	scheme := strings.ToLower(strings.TrimSpace(kind))
	if scheme == "socks" {
		scheme = "socks5"
	}
	proxyURL := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}
	if username != "" || password != "" {
		proxyURL.User = url.UserPassword(username, password)
	}
	return proxyURL, nil
}

// environmentSystemProxyURL is a fail-closed fallback for server and
// CGO-disabled builds. Environment variables are authoritative only when at
// least one proxy variable is explicitly present; an absent native resolver
// must not silently turn System mode into direct mode.
func environmentSystemProxyURL(target *url.URL) (*url.URL, error) {
	canonicalTarget, err := canonicalSystemProxyTarget(target)
	if err != nil {
		return nil, err
	}
	configuration := httpproxy.FromEnvironment()
	allProxy := firstEnvironmentValue("ALL_PROXY", "all_proxy")
	if configuration.HTTPProxy == "" {
		configuration.HTTPProxy = allProxy
	}
	if configuration.HTTPSProxy == "" {
		configuration.HTTPSProxy = allProxy
	}
	if configuration.HTTPProxy == "" && configuration.HTTPSProxy == "" {
		if configuration.NoProxy == "" || !environmentNoProxyExplicitlyMatches(configuration, canonicalTarget) {
			return nil, errNativeSystemProxyUnavailable
		}
		return nil, nil
	}
	resolved, err := configuration.ProxyFunc()(canonicalTarget)
	if err != nil {
		return nil, errors.New("environment system proxy resolution failed")
	}
	if resolved == nil {
		// This is an explicit per-scheme or NO_PROXY direct decision because
		// at least one environment policy variable was present above.
		return nil, nil
	}
	return normalizeSystemProxyCandidate(resolved.String())
}

func environmentNoProxyExplicitlyMatches(configuration *httpproxy.Config, target *url.URL) bool {
	probe := *configuration
	// A sentinel endpoint lets ProxyFunc distinguish a matching NO_PROXY rule
	// from the otherwise ambiguous "there is no proxy configured" result.
	probe.HTTPProxy = "http://proxy-policy-probe.invalid:1"
	probe.HTTPSProxy = probe.HTTPProxy
	resolved, err := probe.ProxyFunc()(target)
	return err == nil && resolved == nil
}

func firstEnvironmentValue(names ...string) string {
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
