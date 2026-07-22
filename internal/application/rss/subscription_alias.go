package rss

import (
	"context"
	"strings"

	domainrss "xiadown/internal/domain/rss"
)

// ensureNoSubscriptionAlias catches the common redirect-alias case that the
// rss_subscriptions.feed_url unique constraint cannot see. Exact feed URLs
// remain protected atomically by SQLite; this preflight additionally compares
// the already persisted, server-validated final URL without making a network
// request for pending subscriptions.
func (service *Service) ensureNoSubscriptionAlias(
	ctx context.Context,
	canonical string,
	metadata fetchMetadata,
) error {
	if service == nil || service.repository == nil {
		return nil
	}
	candidates := feedAliasSet(canonical, metadata.ResolvedURL, metadata.ValidatorURL)
	items, err := service.repository.ListSubscriptions(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		for alias := range feedAliasSet(item.FeedURL, item.PublicFeedURL, item.ValidatorURL) {
			if _, duplicate := candidates[alias]; duplicate {
				return domainrss.ErrDuplicateFeed
			}
		}
	}
	return nil
}

func feedAliasSet(values ...string) map[string]struct{} {
	aliases := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(value), RSSHubScheme) {
			if normalized, err := normalizeFeedURL(value); err == nil {
				aliases[normalized] = struct{}{}
			}
			continue
		}
		if normalized, err := normalizeFeedValidatorURL(value); err == nil {
			aliases[normalized] = struct{}{}
		}
	}
	return aliases
}
