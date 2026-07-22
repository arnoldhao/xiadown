package rss

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
	"xiadown/internal/application/networkpolicy"
)

// SafeRefererOrigin revalidates the fetch descriptor at the HTTP boundary.
// This keeps narrow test doubles and future callers from turning the resource
// proxy into a client-header forwarder or injecting a path, query, credential,
// or private literal address into an upstream Referer.
func (resource RemoteResource) SafeRefererOrigin() string {
	return canonicalPublicHTTPOrigin(resource.RefererOrigin)
}

func firstPublicHTTPOrigin(candidates ...string) string {
	for _, candidate := range candidates {
		if origin := canonicalPublicHTTPOrigin(candidate); origin != "" {
			return origin
		}
	}
	return ""
}

func canonicalPublicHTTPOrigin(rawURL string) string {
	parsed, err := networkpolicy.ValidatePublicHTTPURL(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	scheme, host, port, ok := normalizedHTTPOrigin(parsed)
	if !ok {
		return ""
	}
	return (&url.URL{
		Scheme: scheme,
		Host:   canonicalHTTPValidatorHost(host, port, scheme),
		Path:   "/",
	}).String()
}

// redirectTargetRelatedToInitialResource deliberately compares a redirect
// with the original resource host, not just the preceding hop. An unrelated
// intermediary must never receive the publisher Referer, even if a later hop
// returns to the original site. Subdomains under one registrable site are
// treated as related to support ordinary CDN/canonical-host redirects.
func redirectTargetRelatedToInitialResource(target *url.URL, via []*http.Request) bool {
	if target == nil || len(via) == 0 || via[0] == nil || via[0].URL == nil {
		return false
	}
	return relatedPublicHTTPHosts(via[0].URL, target)
}

func relatedPublicHTTPHosts(left, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	if _, err := networkpolicy.ValidatePublicHTTPURL(left.String()); err != nil {
		return false
	}
	if _, err := networkpolicy.ValidatePublicHTTPURL(right.String()); err != nil {
		return false
	}
	leftHost := strings.ToLower(strings.TrimSuffix(left.Hostname(), "."))
	rightHost := strings.ToLower(strings.TrimSuffix(right.Hostname(), "."))
	if leftHost == "" || rightHost == "" {
		return false
	}
	if leftHost == rightHost {
		return true
	}
	if net.ParseIP(leftHost) != nil || net.ParseIP(rightHost) != nil {
		return false
	}
	leftSite, leftErr := publicsuffix.EffectiveTLDPlusOne(leftHost)
	rightSite, rightErr := publicsuffix.EffectiveTLDPlusOne(rightHost)
	return leftErr == nil && rightErr == nil && strings.EqualFold(leftSite, rightSite)
}
