package proxy

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"unicode"
)

var errWindowsSystemSOCKS4Unsupported = errors.New("Windows system SOCKS4 proxy is unsupported; an explicit SOCKS5 route is required")

// resolveWindowsNamedProxy applies ProxyOverride before selecting the
// destination protocol from a WinINet/WinHTTP proxy string.
func resolveWindowsNamedProxy(target *url.URL, proxyString, proxyBypass string) (*url.URL, error) {
	if windowsProxyBypasses(target, proxyBypass) {
		return nil, nil
	}
	return parseWindowsResolvedProxyForScheme(proxyString, target.Scheme)
}

func parseWindowsResolvedProxyForScheme(proxyString, targetScheme string) (*url.URL, error) {
	proxyString = strings.TrimSpace(proxyString)
	if proxyString == "" {
		return nil, nil
	}

	// Static Windows settings can map one endpoint per destination protocol.
	if strings.Contains(proxyString, "=") {
		return parseWindowsStaticProxyForScheme(proxyString, targetScheme)
	}

	// WinHTTP may return an ordered PAC/failover list. net/http accepts one
	// upstream, so preserve Windows preference by selecting the first entry.
	first, _, _ := strings.Cut(proxyString, ";")
	first = strings.TrimSpace(first)
	if first == "" || strings.EqualFold(first, "DIRECT") {
		return nil, nil
	}

	fields := strings.Fields(first)
	if len(fields) >= 2 {
		endpoint := fields[1]
		switch strings.ToUpper(fields[0]) {
		case "PROXY", "HTTP":
			return parseWindowsProxyEndpoint("http://"+endpoint, targetScheme)
		case "HTTPS":
			return parseWindowsProxyEndpoint("https://"+endpoint, targetScheme)
		case "SOCKS5":
			return parseWindowsProxyEndpoint("socks5://"+endpoint, targetScheme)
		case "SOCKS", "SOCKS4", "SOCKS4A":
			return nil, errWindowsSystemSOCKS4Unsupported
		case "DIRECT":
			return nil, nil
		default:
			// A plain WinHTTP failover list can also be whitespace-separated.
			// In that form the first field is the preferred proxy endpoint.
			first = fields[0]
		}
	}
	return parseWindowsProxyEndpoint(first, targetScheme)
}

func parseWindowsStaticProxyForScheme(proxyString, targetScheme string) (*url.URL, error) {
	targetScheme = strings.ToLower(targetScheme)
	switch targetScheme {
	case "ws":
		targetScheme = "http"
	case "wss":
		targetScheme = "https"
	}

	selected := ""
	defaultProxy := ""
	socks5Proxy := ""
	unsupportedSOCKS4 := false
	for _, entry := range strings.FieldsFunc(proxyString, func(r rune) bool {
		return r == ';' || unicode.IsSpace(r)
	}) {
		entry = strings.TrimSpace(entry)
		key, value, mapped := strings.Cut(entry, "=")
		if !mapped {
			if defaultProxy == "" {
				defaultProxy = entry
			}
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if key == targetScheme && selected == "" {
			selected = value
		}
		if key == "socks5" && socks5Proxy == "" {
			socks5Proxy = value
		}
		if key == "socks" || key == "socks4" || key == "socks4a" {
			unsupportedSOCKS4 = true
		}
	}
	if selected == "" {
		selected = defaultProxy
	}
	if selected == "" && socks5Proxy != "" {
		selected = "socks5://" + strings.TrimPrefix(socks5Proxy, "socks5://")
	}
	if selected == "" && unsupportedSOCKS4 {
		return nil, errWindowsSystemSOCKS4Unsupported
	}
	return parseWindowsProxyEndpoint(selected, targetScheme)
}

func parseWindowsProxyEndpoint(raw, targetScheme string) (*url.URL, error) {
	parsed, err := parseWindowsProxyForScheme(raw, targetScheme)
	if err != nil || parsed == nil {
		return parsed, err
	}
	return normalizeSystemProxyCandidate(parsed.String())
}

func windowsProxyBypasses(target *url.URL, proxyBypass string) bool {
	if target == nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(target.Hostname(), "."))
	if host == "" {
		return false
	}
	port := target.Port()

	for _, rawPattern := range strings.FieldsFunc(proxyBypass, func(r rune) bool {
		return r == ';' || r == ',' || unicode.IsSpace(r)
	}) {
		pattern := strings.ToLower(strings.TrimSpace(rawPattern))
		if pattern == "" {
			continue
		}
		if pattern == "<local>" {
			if net.ParseIP(host) == nil && !strings.Contains(host, ".") {
				return true
			}
			continue
		}
		if pattern == "*" {
			return true
		}
		// WinHTTP defines this list as server names, not URLs or CIDR ranges.
		// Ignore extended forms instead of creating a broader direct route than
		// Windows itself would apply.
		if strings.Contains(pattern, "/") {
			continue
		}
		pattern = strings.TrimSuffix(pattern, ".")

		patternHost := pattern
		patternPort := ""
		if splitHost, splitPort, err := net.SplitHostPort(pattern); err == nil {
			patternHost = strings.Trim(splitHost, "[]")
			patternPort = splitPort
		} else {
			patternHost = strings.Trim(patternHost, "[]")
		}
		if patternPort != "" && patternPort != port {
			continue
		}
		if windowsProxyHostMatches(patternHost, host) {
			return true
		}
	}
	return false
}

func windowsProxyHostMatches(pattern, host string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	host = strings.ToLower(host)
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(host, strings.TrimPrefix(pattern, "*"))
	}
	return host == pattern
}
