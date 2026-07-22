package proxy

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

// shouldBypass preserves the old package helper while using boundary-aware
// matching. Callers that know the destination port should use
// shouldBypassHostPort so port-qualified rules remain strict.
func shouldBypass(host string, noProxy []string) bool {
	return shouldBypassHostPort(host, "", noProxy)
}

func shouldBypassURL(target *url.URL, noProxy []string) bool {
	if target == nil {
		return false
	}
	port := target.Port()
	if port == "" {
		switch strings.ToLower(target.Scheme) {
		case "http", "ws":
			port = "80"
		case "https", "wss":
			port = "443"
		}
	}
	return shouldBypassHostPort(target.Hostname(), port, noProxy)
}

func shouldBypassAddress(address string, noProxy []string) bool {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		host = address
		port = ""
	}
	return shouldBypassHostPort(host, port, noProxy)
}

func shouldBypassHostPort(host, port string, noProxy []string) bool {
	host = normalizeProxyHost(host)
	if host == "" {
		return false
	}

	// The gateway itself is loopback-only. Loopback must never be sent to an
	// upstream proxy, both to preserve local-service behavior and to prevent a
	// user-entered proxy rule from creating a forwarding loop.
	if host == "localhost" {
		return true
	}
	if ip := parseProxyIP(host); ip != nil && ip.IsLoopback() {
		return true
	}

	for _, rawRule := range noProxy {
		rule := strings.TrimSpace(rawRule)
		if rule == "" {
			continue
		}
		if rule == "*" {
			return true
		}

		if network := parseNoProxyCIDR(rule); network != nil {
			if ip := parseProxyIP(host); ip != nil && network.Contains(ip) {
				return true
			}
			continue
		}

		ruleHost, rulePort := splitNoProxyRule(rule)
		if ruleHost == "" || (rulePort != "" && rulePort != port) {
			continue
		}
		if noProxyHostMatches(host, ruleHost) {
			return true
		}
	}
	return false
}

func normalizeProxyHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	host = strings.TrimSuffix(host, ".")
	if normalized, err := canonicalRouteHostname(host); err == nil {
		host = normalized
	}
	return host
}

func parseProxyIP(host string) net.IP {
	if zone := strings.LastIndex(host, "%"); zone >= 0 {
		host = host[:zone]
	}
	return net.ParseIP(host)
}

func parseNoProxyCIDR(rule string) *net.IPNet {
	if !strings.Contains(rule, "/") {
		return nil
	}
	_, network, err := net.ParseCIDR(strings.TrimSpace(rule))
	if err != nil {
		return nil
	}
	return network
}

func splitNoProxyRule(rule string) (host, port string) {
	rule = strings.TrimSpace(strings.ToLower(rule))
	if strings.Contains(rule, "://") {
		if parsed, err := url.Parse(rule); err == nil {
			return normalizeProxyHost(parsed.Hostname()), parsed.Port()
		}
	}
	if strings.HasPrefix(rule, "[") {
		if h, p, err := net.SplitHostPort(rule); err == nil {
			return normalizeProxyHost(h), p
		}
		return normalizeProxyHost(rule), ""
	}
	if strings.Count(rule, ":") == 1 {
		if h, p, err := net.SplitHostPort(rule); err == nil {
			if _, err := strconv.ParseUint(p, 10, 16); err == nil {
				return normalizeProxyHost(h), p
			}
		}
	}
	// A bare IPv6 literal contains multiple colons and therefore cannot be
	// confused with a host:port rule.
	return normalizeProxyHost(rule), ""
}

func noProxyHostMatches(host, rule string) bool {
	host = normalizeProxyHost(host)
	rule = normalizeProxyHost(rule)
	if host == "" || rule == "" {
		return false
	}

	if hostIP, ruleIP := parseProxyIP(host), parseProxyIP(rule); hostIP != nil || ruleIP != nil {
		return hostIP != nil && ruleIP != nil && hostIP.Equal(ruleIP)
	}

	if strings.HasPrefix(rule, "*.") {
		rule = strings.TrimPrefix(rule, "*.")
		return strings.HasSuffix(host, "."+rule)
	}
	rule = strings.TrimPrefix(rule, ".")
	return host == rule || strings.HasSuffix(host, "."+rule)
}
