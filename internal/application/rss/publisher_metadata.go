package rss

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"xiadown/internal/application/networkpolicy"
	domainrss "xiadown/internal/domain/rss"

	xhtml "golang.org/x/net/html"
)

const (
	maxRSSPublisherOriginSamples = 16
	maxRSSFaviconHTMLBytes       = 512 << 10
	maxRSSFaviconHTMLTokens      = 20_000
	maxRSSFaviconCandidates      = 32
	rssFaviconDiscoveryTimeout   = 8 * time.Second
)

type rssFaviconCandidate struct {
	href     string
	mimeType string
	score    int
}

// enrichParsedFeedPublisherSite fills feeds such as RSSHub responses which do
// not expose channel-level site metadata. A unique majority is required when
// entries disagree, preventing one outlier link from becoming the publisher
// identity for an aggregated feed.
func enrichParsedFeedPublisherSite(feed parsedFeed) parsedFeed {
	if canonicalPublicHTTPOrigin(feed.SiteURL) != "" {
		return feed
	}
	urls := make([]string, 0, min(len(feed.Entries), maxRSSPublisherOriginSamples))
	for index := 0; index < len(feed.Entries) && index < maxRSSPublisherOriginSamples; index++ {
		urls = append(urls, feed.Entries[index].URL)
	}
	feed.SiteURL = stableRSSPublisherOrigin(urls)
	return feed
}

func stableRSSPublisherOrigin(urls []string) string {
	type originFrequency struct {
		origin string
		count  int
	}
	frequencies := make([]originFrequency, 0, maxRSSPublisherOriginSamples)
	positions := make(map[string]int, maxRSSPublisherOriginSamples)
	validCount := 0
	for index := 0; index < len(urls) && index < maxRSSPublisherOriginSamples; index++ {
		origin := canonicalPublicHTTPOrigin(urls[index])
		if origin == "" {
			continue
		}
		validCount++
		if position, ok := positions[origin]; ok {
			frequencies[position].count++
			continue
		}
		positions[origin] = len(frequencies)
		frequencies = append(frequencies, originFrequency{origin: origin, count: 1})
	}
	if validCount == 0 {
		return ""
	}
	best := frequencies[0]
	for _, frequency := range frequencies[1:] {
		if frequency.count > best.count {
			best = frequency
		}
	}
	if len(frequencies) > 1 && best.count*2 <= validCount {
		return ""
	}
	return best.origin
}

func (service *Service) enrichParsedFeedIcon(ctx context.Context, feed parsedFeed, fallbackSiteURL string) parsedFeed {
	if strings.TrimSpace(feed.IconURL) != "" {
		return feed
	}
	feed.IconURL = service.discoverRSSFavicon(ctx, firstNonEmpty(feed.SiteURL, fallbackSiteURL))
	return feed
}

// backfillSubscriptionMetadata is intentionally best-effort. Metadata failure
// must not turn a valid conditional 304 into a failed feed refresh.
func (service *Service) backfillSubscriptionMetadata(ctx context.Context, subscription domainrss.Subscription) domainrss.Subscription {
	if strings.TrimSpace(subscription.SiteURL) == "" && service != nil && service.repository != nil {
		page, err := service.repository.ListEntries(ctx, domainrss.EntryQuery{
			SubscriptionID: subscription.ID,
			Limit:          maxRSSPublisherOriginSamples,
		})
		if err == nil {
			urls := make([]string, 0, len(page.Items))
			for _, entry := range page.Items {
				urls = append(urls, entry.URL)
			}
			subscription.SiteURL = stableRSSPublisherOrigin(urls)
		}
	}
	if strings.TrimSpace(subscription.IconURL) == "" {
		subscription.IconURL = service.discoverRSSFavicon(ctx, subscription.SiteURL)
	}
	return subscription
}

// discoverRSSFavicon reads only a bounded HTML document and honors declared
// icon links. It deliberately does not guess /favicon.ico: a missing, invalid,
// private, oversized, or unreachable icon declaration simply leaves metadata
// empty and never blocks subscription creation or refresh.
func (service *Service) discoverRSSFavicon(ctx context.Context, siteURL string) string {
	if service == nil {
		return ""
	}
	siteOrigin := canonicalPublicHTTPOrigin(siteURL)
	if siteOrigin == "" {
		return ""
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, rssFaviconDiscoveryTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(discoveryCtx, http.MethodGet, siteOrigin, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "XiaDown RSS Metadata/1.0")
	response, err := service.httpClient().Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices ||
		response.ContentLength > maxRSSFaviconHTMLBytes {
		return ""
	}
	if contentType := strings.TrimSpace(response.Header.Get("Content-Type")); contentType != "" {
		mediaType, _, parseErr := mime.ParseMediaType(contentType)
		if parseErr != nil || (mediaType != "text/html" && mediaType != "application/xhtml+xml") {
			return ""
		}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxRSSFaviconHTMLBytes+1))
	if err != nil || len(body) == 0 || len(body) > maxRSSFaviconHTMLBytes {
		return ""
	}
	baseURL := siteOrigin
	if response.Request != nil && response.Request.URL != nil {
		baseURL = response.Request.URL.String()
	}
	candidates, documentBaseURL := rssFaviconLinks(body, baseURL)
	bestURL, bestScore := "", -1
	for _, candidate := range candidates {
		resolved := resolveURL(documentBaseURL, candidate.href)
		parsed, validateErr := networkpolicy.ValidatePublicHTTPURL(resolved)
		if validateErr != nil || parsed == nil || !supportedRSSFaviconCandidate(parsed, candidate.mimeType) {
			continue
		}
		parsed.Fragment = ""
		if candidate.score > bestScore {
			bestURL, bestScore = parsed.String(), candidate.score
		}
	}
	return bestURL
}

func rssFaviconLinks(body []byte, baseURL string) ([]rssFaviconCandidate, string) {
	tokenizer := xhtml.NewTokenizer(bytes.NewReader(body))
	candidates := make([]rssFaviconCandidate, 0, 4)
	documentBaseURL := baseURL
	baseSet := false
	for tokens := 0; tokens < maxRSSFaviconHTMLTokens; tokens++ {
		tokenType := tokenizer.Next()
		if tokenType == xhtml.ErrorToken {
			break
		}
		if tokenType != xhtml.StartTagToken && tokenType != xhtml.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if strings.EqualFold(token.Data, "base") && !baseSet {
			for _, attribute := range token.Attr {
				if strings.EqualFold(attribute.Key, "href") {
					resolved := resolveURL(baseURL, strings.TrimSpace(attribute.Val))
					if _, err := networkpolicy.ValidatePublicHTTPURL(resolved); err == nil {
						documentBaseURL = resolved
						baseSet = true
					}
					break
				}
			}
			continue
		}
		if !strings.EqualFold(token.Data, "link") || len(candidates) >= maxRSSFaviconCandidates {
			continue
		}
		var rel, href, mimeType string
		for _, attribute := range token.Attr {
			switch strings.ToLower(attribute.Key) {
			case "rel":
				rel = strings.ToLower(strings.TrimSpace(attribute.Val))
			case "href":
				href = strings.TrimSpace(attribute.Val)
			case "type":
				mimeType = strings.ToLower(strings.TrimSpace(attribute.Val))
			}
		}
		score := rssFaviconRelScore(rel)
		if score < 0 || href == "" || len(href) > maxFeedValidatorURLBytes {
			continue
		}
		candidates = append(candidates, rssFaviconCandidate{href: href, mimeType: mimeType, score: score})
	}
	return candidates, documentBaseURL
}

func rssFaviconRelScore(rel string) int {
	relations := strings.Fields(rel)
	hasIcon, hasAppleTouch, hasMask := false, false, false
	for _, relation := range relations {
		switch relation {
		case "icon":
			hasIcon = true
		case "apple-touch-icon", "apple-touch-icon-precomposed":
			hasAppleTouch = true
		case "mask-icon":
			hasMask = true
		}
	}
	if hasMask {
		return -1
	}
	if hasIcon {
		return 300
	}
	if hasAppleTouch {
		return 200
	}
	return -1
}

func supportedRSSFaviconCandidate(candidate *url.URL, mimeType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(mimeType))
	if parsedType, _, err := mime.ParseMediaType(mediaType); err == nil {
		mediaType = parsedType
	}
	if strings.Contains(mediaType, "svg") {
		return false
	}
	return !strings.EqualFold(path.Ext(candidate.Path), ".svg")
}
