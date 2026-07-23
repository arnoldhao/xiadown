package service

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/idna"

	"xiadown/internal/application/networkpolicy"
	appytdlp "xiadown/internal/application/ytdlp"
)

const resourceRedirectLimit = 10

var (
	errHTTPSRedirectDowngrade = errors.New("HTTPS redirect downgrade is not allowed")
	errTooManyRedirects       = errors.New("too many resource redirects")
)

type capturedURLIdentity struct {
	scheme string
	host   string
	port   string
}

func isCookieCredentialHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "cookie", "cookie2":
		return true
	default:
		return false
	}
}

func isProxyCredentialHeader(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "Proxy-Authorization")
}

func isFreelyForwardableCapturedHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "accept", "accept-language", "origin", "user-agent":
		return true
	default:
		return false
	}
}

// scopeCapturedHTTPHeaders applies two independent browser-credential scopes:
// cookies follow the initial hostname without an HTTPS downgrade, while all
// other non-public captured headers require the initial exact origin. Proxy
// credentials are transport-owned and are never sent as origin headers.
func scopeCapturedHTTPHeaders(header http.Header, initialURL *url.URL, targetURL *url.URL) {
	cookieAllowed := capturedCookieAllowed(initialURL, targetURL)
	originAllowed := capturedExactOrigin(initialURL, targetURL)
	for name := range header {
		switch {
		case isProxyCredentialHeader(name):
			header.Del(name)
		case isCookieCredentialHeader(name):
			if !cookieAllowed {
				header.Del(name)
			}
		case strings.EqualFold(strings.TrimSpace(name), "Referer"):
			if !originAllowed {
				if origin, ok := normalizedCapturedHTTPOrigin(header.Get(name)); ok {
					header.Set(name, origin+"/")
				} else {
					header.Del(name)
				}
			}
		case isFreelyForwardableCapturedHeader(name):
			// Public representation headers are safe across a validated redirect.
		default:
			if !originAllowed {
				header.Del(name)
			}
		}
	}
}

// enforceCredentialSafeRedirect preserves credentials only inside their
// explicit scope, restores net/http's ten-hop safety limit, and validates every
// redirect target through the App's public HTTP URL policy.
func enforceCredentialSafeRedirect(request *http.Request, via []*http.Request) error {
	if request == nil || request.URL == nil {
		return networkpolicy.ErrDestinationBlocked
	}
	var initialURL *url.URL
	if len(via) > 0 && via[0] != nil {
		initialURL = via[0].URL
	}
	scopeCapturedHTTPHeaders(request.Header, initialURL, request.URL)
	if len(via) >= resourceRedirectLimit {
		return errTooManyRedirects
	}
	if err := appytdlp.ValidateNetworkURL(request.URL.String()); err != nil {
		return err
	}
	if _, err := networkpolicy.ValidatePublicHTTPURL(request.URL.String()); err != nil {
		return err
	}
	if len(via) == 0 {
		return nil
	}
	previous := via[len(via)-1]
	if previous != nil && previous.URL != nil &&
		strings.EqualFold(previous.URL.Scheme, "https") &&
		strings.EqualFold(request.URL.Scheme, "http") {
		return errHTTPSRedirectDowngrade
	}
	return nil
}

func capturedCookieAllowed(initialURL *url.URL, targetURL *url.URL) bool {
	initial, initialOK := capturedURLIdentityFor(initialURL)
	target, targetOK := capturedURLIdentityFor(targetURL)
	if !initialOK || !targetOK || initial.host != target.host {
		return false
	}
	return initial.scheme != "https" || target.scheme == "https"
}

func capturedExactOrigin(initialURL *url.URL, targetURL *url.URL) bool {
	initial, initialOK := capturedURLIdentityFor(initialURL)
	target, targetOK := capturedURLIdentityFor(targetURL)
	return initialOK && targetOK && initial == target
}

func capturedURLIdentityFor(value *url.URL) (capturedURLIdentity, bool) {
	if value == nil || appytdlp.ValidateNetworkURL(value.String()) != nil {
		return capturedURLIdentity{}, false
	}
	scheme := strings.ToLower(strings.TrimSpace(value.Scheme))
	host, ok := normalizedCapturedHostname(value.Hostname())
	if !ok {
		return capturedURLIdentity{}, false
	}
	port := strings.TrimSpace(value.Port())
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	} else {
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return capturedURLIdentity{}, false
		}
		port = strconv.Itoa(parsedPort)
	}
	return capturedURLIdentity{scheme: scheme, host: host, port: port}, true
}

func normalizedCapturedHostname(raw string) (string, bool) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if host == "" {
		return "", false
	}
	if parsedIP := net.ParseIP(strings.Trim(host, "[]")); parsedIP != nil {
		return parsedIP.String(), true
	}
	ascii, err := idna.Lookup.ToASCII(host)
	return ascii, err == nil && strings.TrimSpace(ascii) != ""
}

func normalizedCapturedHTTPOrigin(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	identity, ok := capturedURLIdentityFor(parsed)
	if !ok {
		return "", false
	}
	host := identity.host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	defaultPort := (identity.scheme == "https" && identity.port == "443") ||
		(identity.scheme == "http" && identity.port == "80")
	if !defaultPort {
		host = net.JoinHostPort(identity.host, identity.port)
	}
	return identity.scheme + "://" + host, true
}

func newResourceHTTPClient(proxyURL string) (*http.Client, error) {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	if trimmed := strings.TrimSpace(proxyURL); trimmed != "" {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{
		Transport:     transport,
		CheckRedirect: enforceCredentialSafeRedirect,
	}, nil
}
