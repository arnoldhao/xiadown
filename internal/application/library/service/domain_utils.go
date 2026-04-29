package service

import (
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

func extractRegistrableDomain(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	host := trimmed
	if parsed, err := url.Parse(trimmed); err == nil {
		if parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return host
	}
	if registrable, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return registrable
	}
	return host
}
