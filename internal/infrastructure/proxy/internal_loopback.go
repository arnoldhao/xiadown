package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// internalLoopbackRegistry contains only process-owned loopback listeners.
// WebViews enter the gateway even for loopback URLs so a remote page cannot
// use XiaDown as an unrestricted localhost proxy. Product code must register
// an exact listener authority before exposing its URL to a WebView.
type internalLoopbackRegistry struct {
	mu      sync.RWMutex
	targets map[string]struct{}
}

func newInternalLoopbackRegistry() *internalLoopbackRegistry {
	return &internalLoopbackRegistry{targets: make(map[string]struct{})}
}

func (registry *internalLoopbackRegistry) register(rawURL string) error {
	target, loopback, err := canonicalLoopbackURLTarget(rawURL)
	if err != nil {
		return err
	}
	if !loopback {
		return errors.New("internal network target must be loopback")
	}
	registry.mu.Lock()
	registry.targets[target] = struct{}{}
	registry.mu.Unlock()
	return nil
}

// permitsRequest reports whether a request targets loopback, whether that
// exact authority is permitted, and whether the permitted route must ignore
// the active user/system proxy and connect directly. Keeping forceDirect as
// an explicit policy result prevents a future change to generic NoProxy
// matching from sending app-owned tokens or media paths to an upstream proxy.
func (registry *internalLoopbackRegistry) permitsRequest(request *http.Request, gatewayAddress string) (loopback, permitted, forceDirect bool) {
	target, loopback := canonicalLoopbackRequestTarget(request)
	if !loopback {
		return false, true, false
	}
	if sameCanonicalNetworkTarget(target, gatewayAddress) {
		// Force the recursive request through dialDirect so it is rejected
		// locally instead of ever being disclosed to an upstream proxy.
		return true, true, true
	}
	registry.mu.RLock()
	_, permitted = registry.targets[target]
	registry.mu.RUnlock()
	return true, permitted, permitted
}

func canonicalLoopbackURLTarget(rawURL string) (string, bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", false, errors.New("invalid internal network target URL")
	}
	if parsed.User != nil || parsed.Opaque != "" || parsed.Hostname() == "" {
		return "", false, errors.New("invalid internal network target URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "ws":
		scheme = "http"
	case "https", "wss":
		scheme = "https"
	default:
		return "", false, errors.New("internal network target must use HTTP(S) or WS(S)")
	}
	target, loopback := canonicalNetworkTarget(parsed.Host, scheme)
	if target == "" {
		return "", false, errors.New("invalid internal network target authority")
	}
	return target, loopback, nil
}

func canonicalLoopbackRequestTarget(request *http.Request) (string, bool) {
	if request == nil {
		return "", false
	}
	scheme := ""
	authority := request.Host
	if request.Method == http.MethodConnect {
		scheme = "https"
	} else if request.URL != nil {
		scheme = strings.ToLower(request.URL.Scheme)
		if request.URL.Host != "" {
			authority = request.URL.Host
		}
	}
	switch scheme {
	case "ws":
		scheme = "http"
	case "wss":
		scheme = "https"
	}
	return canonicalNetworkTarget(authority, scheme)
}

func canonicalNetworkTarget(authority, scheme string) (string, bool) {
	authority = strings.TrimSpace(authority)
	if authority == "" || strings.ContainsAny(authority, "\r\n\x00") {
		return "", false
	}
	host, port, err := net.SplitHostPort(authority)
	if err != nil {
		host = strings.Trim(authority, "[]")
		switch strings.ToLower(scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", false
		}
	}
	if host == "" {
		return "", false
	}
	if _, err := parsePort(port); err != nil {
		return "", false
	}
	host = strings.ToLower(strings.TrimSpace(strings.Trim(host, "[]")))
	if normalized, err := canonicalRouteHostname(host); err == nil {
		host = normalized
	}
	loopback := host == "localhost"
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
		loopback = ip.IsLoopback()
	}
	return net.JoinHostPort(host, port), loopback
}

func sameCanonicalNetworkTarget(target, other string) bool {
	canonicalOther, _ := canonicalNetworkTarget(other, "")
	return target != "" && target == canonicalOther
}
