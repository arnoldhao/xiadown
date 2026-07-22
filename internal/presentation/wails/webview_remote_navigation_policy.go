package wails

import (
	"net"
	"net/url"
	"strings"

	"xiadown/internal/application/sitepolicy"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

type webViewRemoteNavigationPolicyKind uint8

const (
	webViewRemoteNavigationPolicyInvalid webViewRemoteNavigationPolicyKind = iota
	webViewRemoteNavigationPolicyYouTubeMusic
	webViewRemoteNavigationPolicyYouTubeLive
	webViewRemoteNavigationPolicyRSSBilibili
	webViewRemoteNavigationPolicyAppSession
	webViewRemoteNavigationPolicyRSSSite
)

// webViewRemoteNavigationPolicy governs top-level documents, not subresources.
// Subresources remain on the platform WebView's app-owned network route. A
// blank document is always safe because it carries no remote page capability.
type webViewRemoteNavigationPolicy struct {
	kind            webViewRemoteNavigationPolicyKind
	expectedAdapter string
	expectedVideoID string
	allowedDomains  []string
	registrableSite string
}

func (policy webViewRemoteNavigationPolicy) allows(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == connectorAppSessionBlankURL {
		return policy.kind != webViewRemoteNavigationPolicyInvalid
	}

	switch policy.kind {
	case webViewRemoteNavigationPolicyYouTubeMusic:
		_, ok := webViewYouTubeMusicVideoID(rawURL)
		return ok
	case webViewRemoteNavigationPolicyYouTubeLive:
		videoID, ok := webViewYouTubeLiveVideoID(rawURL)
		return ok && policy.expectedVideoID != "" && videoID == policy.expectedVideoID
	case webViewRemoteNavigationPolicyRSSBilibili:
		return rssBilibiliAllowsTopLevelNavigationForPlayback(
			rawURL,
			policy.expectedAdapter,
			policy.expectedVideoID,
		)
	case webViewRemoteNavigationPolicyAppSession:
		return webViewAppSessionHTTPSURLAllowed(rawURL)
	case webViewRemoteNavigationPolicyRSSSite:
		return webViewRSSSiteHTTPSURLAllowed(rawURL, policy.allowedDomains, policy.registrableSite)
	default:
		return false
	}
}

func webViewRemoteNavigationPolicyForRSSSite(
	targetURL string,
	allowedDomains []string,
	registrableSite string,
) (webViewRemoteNavigationPolicy, bool) {
	policy := webViewRemoteNavigationPolicy{
		kind:            webViewRemoteNavigationPolicyRSSSite,
		allowedDomains:  cloneWebViewNavigationDomains(allowedDomains),
		registrableSite: strings.ToLower(strings.TrimSpace(registrableSite)),
	}
	if len(policy.allowedDomains) == 0 && policy.registrableSite == "" {
		return webViewRemoteNavigationPolicy{}, false
	}
	return policy, targetURL != connectorAppSessionBlankURL && policy.allows(targetURL)
}

func webViewRemoteNavigationPolicyForPlayer(windowName string, targetURL string) (webViewRemoteNavigationPolicy, bool) {
	var policy webViewRemoteNavigationPolicy
	switch strings.TrimSpace(windowName) {
	case listenPlayerWindowName:
		policy.kind = webViewRemoteNavigationPolicyYouTubeMusic
	case listenLivePlayerWindowName:
		videoID, ok := webViewYouTubeLiveVideoID(targetURL)
		if !ok {
			return webViewRemoteNavigationPolicy{}, false
		}
		policy.kind = webViewRemoteNavigationPolicyYouTubeLive
		policy.expectedVideoID = videoID
	case rssBilibiliPlayerWindowName:
		adapter, videoID, ok := rssBilibiliPlaybackIdentityFromURL(targetURL)
		if !ok {
			return webViewRemoteNavigationPolicy{}, false
		}
		policy.kind = webViewRemoteNavigationPolicyRSSBilibili
		policy.expectedAdapter = adapter
		policy.expectedVideoID = videoID
	default:
		return webViewRemoteNavigationPolicy{}, false
	}
	return policy, targetURL != connectorAppSessionBlankURL && policy.allows(targetURL)
}

func webViewRemoteNavigationPolicyForAppSession(targetURL string) (webViewRemoteNavigationPolicy, bool) {
	policy := webViewRemoteNavigationPolicy{kind: webViewRemoteNavigationPolicyAppSession}
	return policy, targetURL != connectorAppSessionBlankURL && policy.allows(targetURL)
}

func webViewYouTubeMusicVideoID(rawURL string) (string, bool) {
	parsed, ok := webViewHTTPSURLForHost(rawURL, "music.youtube.com")
	if !ok || parsed.EscapedPath() != "/watch" {
		return "", false
	}
	return webViewSingleYouTubeVideoID(parsed)
}

func webViewYouTubeLiveVideoID(rawURL string) (string, bool) {
	parsed, ok := webViewHTTPSURLForHost(rawURL, "www.youtube.com")
	if !ok {
		return "", false
	}
	switch {
	case parsed.EscapedPath() == "/watch":
		return webViewSingleYouTubeVideoID(parsed)
	case strings.HasPrefix(parsed.EscapedPath(), "/embed/"):
		videoID := strings.TrimPrefix(parsed.EscapedPath(), "/embed/")
		return videoID, listenYouTubeVideoIDPattern.MatchString(videoID)
	default:
		return "", false
	}
}

func webViewSingleYouTubeVideoID(parsed *url.URL) (string, bool) {
	if parsed == nil {
		return "", false
	}
	values, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", false
	}
	videoIDs := values["v"]
	if len(videoIDs) != 1 || !listenYouTubeVideoIDPattern.MatchString(videoIDs[0]) {
		return "", false
	}
	return videoIDs[0], true
}

func webViewHTTPSURLForHost(rawURL string, expectedHost string) (*url.URL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.User != nil || parsed.Opaque != "" {
		return nil, false
	}
	if !strings.EqualFold(parsed.Scheme, "https") ||
		!strings.EqualFold(parsed.Hostname(), expectedHost) ||
		(parsed.Port() != "" && parsed.Port() != "443") {
		return nil, false
	}
	return parsed, true
}

func webViewAppSessionHTTPSURLAllowed(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.User != nil || parsed.Opaque != "" || parsed.Hostname() == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "https") &&
		(parsed.Port() == "" || parsed.Port() == "443")
}

func webViewRSSSiteHTTPSURLAllowed(rawURL string, allowedDomains []string, registrableSite string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || parsed.User != nil || parsed.Opaque != "" || parsed.Hostname() == "" {
		return false
	}
	if !strings.EqualFold(parsed.Scheme, "https") || (parsed.Port() != "" && parsed.Port() != "443") {
		return false
	}
	host, err := idna.Lookup.ToASCII(strings.TrimSuffix(strings.ToLower(parsed.Hostname()), "."))
	if err != nil || host == "" || net.ParseIP(host) != nil {
		return false
	}
	if len(allowedDomains) > 0 {
		for _, domain := range allowedDomains {
			if sitepolicy.HostMatchesDomain(host, domain) {
				return true
			}
		}
		return false
	}
	expected := strings.ToLower(strings.TrimSpace(registrableSite))
	actual, err := publicsuffix.EffectiveTLDPlusOne(host)
	return err == nil && expected != "" && strings.EqualFold(actual, expected)
}

func cloneWebViewNavigationDomains(domains []string) []string {
	result := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
		if domain != "" {
			result = append(result, domain)
		}
	}
	return result
}
