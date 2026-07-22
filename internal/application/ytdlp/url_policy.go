package ytdlp

import (
	"errors"
	"net/url"
	"strings"
)

var ErrUnsupportedNetworkURL = errors.New("URL must be an absolute HTTP(S) URL without userinfo")

// ValidateNetworkURL enforces the only URL shape that XiaDown may hand to
// yt-dlp, ffmpeg, or a manifest-derived network request. In particular,
// credentials must travel through the App's managed headers/cookie/proxy
// paths, never in URL userinfo where helpers can log or reinterpret them.
func ValidateNetworkURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ErrUnsupportedNetworkURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil || !parsed.IsAbs() || parsed.Opaque != "" {
		return ErrUnsupportedNetworkURL
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return ErrUnsupportedNetworkURL
	}
	if strings.TrimSpace(parsed.Hostname()) == "" || parsed.User != nil {
		return ErrUnsupportedNetworkURL
	}
	return nil
}

func resolveNetworkReference(baseURL string, reference string) string {
	trimmedReference := strings.TrimSpace(reference)
	if trimmedReference == "" || ValidateNetworkURL(baseURL) != nil {
		return ""
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return ""
	}
	parsedReference, err := url.Parse(trimmedReference)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(parsedReference)
	if resolved == nil || ValidateNetworkURL(resolved.String()) != nil {
		return ""
	}
	return resolved.String()
}
