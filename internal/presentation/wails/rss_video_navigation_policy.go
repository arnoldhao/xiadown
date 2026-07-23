package wails

import (
	"net/url"
	"regexp"
	"strings"
)

const (
	rssBilibiliVideoAdapter   = "video"
	rssBilibiliBangumiAdapter = "bangumi"
)

var (
	rssBilibiliCanonicalVideoPathPattern   = regexp.MustCompile(`^/video/(?i:BV[0-9a-z]{10}|av[1-9][0-9]*)/?$`)
	rssBilibiliCanonicalBangumiPathPattern = regexp.MustCompile(`^/bangumi/play/(?i:ep|ss)[1-9][0-9]*/?$`)
)

func rssBilibiliPlaybackIdentityFromURL(rawURL string) (string, string, bool) {
	parsed, adapter, ok := rssBilibiliCanonicalPlaybackURL(rawURL)
	if !ok {
		return "", "", false
	}
	path := strings.TrimSuffix(parsed.EscapedPath(), "/")
	var prefix string
	switch adapter {
	case rssBilibiliVideoAdapter:
		prefix = "/video/"
	case rssBilibiliBangumiAdapter:
		prefix = "/bangumi/play/"
	default:
		return "", "", false
	}
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	videoID := strings.TrimPrefix(path, prefix)
	if videoID == "" || strings.Contains(videoID, "/") {
		return "", "", false
	}
	switch {
	case adapter == rssBilibiliVideoAdapter && len(videoID) >= 2 && strings.EqualFold(videoID[:2], "BV"):
		return adapter, "BV" + videoID[2:], true
	case adapter == rssBilibiliVideoAdapter && len(videoID) >= 2 && strings.EqualFold(videoID[:2], "av"):
		return adapter, "av" + videoID[2:], true
	case adapter == rssBilibiliBangumiAdapter && len(videoID) >= 2 && strings.EqualFold(videoID[:2], "ep"):
		return adapter, "ep" + videoID[2:], true
	case adapter == rssBilibiliBangumiAdapter && len(videoID) >= 2 && strings.EqualFold(videoID[:2], "ss"):
		return adapter, "ss" + videoID[2:], true
	default:
		return "", "", false
	}
}

func rssBilibiliVideoIDFromURL(rawURL string) (string, bool) {
	adapter, videoID, ok := rssBilibiliPlaybackIdentityFromURL(rawURL)
	if !ok || adapter != rssBilibiliVideoAdapter {
		return "", false
	}
	return videoID, true
}

func rssBilibiliCanonicalPlaybackURL(rawURL string) (*url.URL, string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.User != nil || parsed.Opaque != "" {
		return nil, "", false
	}
	if !strings.EqualFold(parsed.Scheme, "https") ||
		!strings.EqualFold(parsed.Hostname(), "www.bilibili.com") {
		return nil, "", false
	}
	port := parsed.Port()
	if port != "" && port != "443" {
		return nil, "", false
	}
	switch {
	case rssBilibiliCanonicalVideoPathPattern.MatchString(parsed.EscapedPath()):
		return parsed, rssBilibiliVideoAdapter, true
	case rssBilibiliCanonicalBangumiPathPattern.MatchString(parsed.EscapedPath()):
		return parsed, rssBilibiliBangumiAdapter, true
	default:
		return nil, "", false
	}
}

func rssBilibiliCanonicalVideoURL(rawURL string) (*url.URL, bool) {
	parsed, adapter, ok := rssBilibiliCanonicalPlaybackURL(rawURL)
	return parsed, ok && adapter == rssBilibiliVideoAdapter
}

// rssBilibiliAllowsTopLevelNavigation is deliberately narrower than the
// network requests the canonical watch page may issue. Subresources stay
// under the WebView's normal policy, while the visible document may only be a
// validated Bilibili video page. Homepage, external-player, and lookalike URLs
// must never replace the video-only native surface.
func rssBilibiliAllowsTopLevelNavigation(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == rssBilibiliPlayerBlankURL {
		return true
	}
	_, _, ok := rssBilibiliCanonicalPlaybackURL(rawURL)
	return ok
}

// rssBilibiliAllowsTopLevelNavigationForPlayback locks a playback-only
// WebView to both the adapter and canonical media identity selected by Prepare.
// Query, fragment, and multipart changes remain available because they do not
// alter either part of that identity.
func rssBilibiliAllowsTopLevelNavigationForPlayback(
	rawURL string,
	expectedAdapter string,
	expectedVideoID string,
) bool {
	rawURL = strings.TrimSpace(rawURL)
	adapter, videoID, ok := rssBilibiliPlaybackIdentityFromURL(rawURL)
	return ok &&
		expectedAdapter != "" && adapter == expectedAdapter &&
		expectedVideoID != "" && videoID == expectedVideoID
}

// rssBilibiliAllowsTopLevelNavigationForVideo locks a playback-only WebView
// to the exact identity selected by Prepare. Query, fragment, and multipart
// changes remain available because they do not alter the canonical path ID.
func rssBilibiliAllowsTopLevelNavigationForVideo(rawURL string, expectedVideoID string) bool {
	return rssBilibiliAllowsTopLevelNavigationForPlayback(
		rawURL,
		rssBilibiliVideoAdapter,
		expectedVideoID,
	)
}
