package rss

import (
	"context"
	"errors"
	"testing"

	domainrss "xiadown/internal/domain/rss"
)

type subscriptionAliasRepository struct {
	stateRepositoryStub
	items        []domainrss.Subscription
	pendingCalls int
}

func (repo *subscriptionAliasRepository) ListSubscriptions(context.Context) ([]domainrss.Subscription, error) {
	return append([]domainrss.Subscription(nil), repo.items...), nil
}

func (repo *subscriptionAliasRepository) CreateSubscription(
	_ context.Context,
	item domainrss.Subscription,
) (domainrss.Subscription, error) {
	repo.pendingCalls++
	return item, nil
}

func TestPendingSubscriptionRejectsPersistedRedirectAliasWithoutPreview(t *testing.T) {
	repository := &subscriptionAliasRepository{items: []domainrss.Subscription{{
		ID: "existing", FeedURL: "https://example.com/old-feed",
		ValidatorURL: "https://feeds.example.com/rss.xml",
	}}}
	service := NewService(repository, nil)

	_, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: "HTTPS://FEEDS.EXAMPLE.COM:443/rss.xml#ignored", AllowPending: true,
	})
	if !errors.Is(err, domainrss.ErrDuplicateFeed) {
		t.Fatalf("redirect alias error = %v, want ErrDuplicateFeed", err)
	}
	if repository.pendingCalls != 0 {
		t.Fatalf("duplicate redirect alias created %d pending subscriptions", repository.pendingCalls)
	}
}

func TestResolvedPreviewAliasMatchesExistingRequestedURL(t *testing.T) {
	repository := &subscriptionAliasRepository{items: []domainrss.Subscription{{
		ID: "existing", FeedURL: "https://feeds.example.com/rss.xml",
	}}}
	service := NewService(repository, nil)
	err := service.ensureNoSubscriptionAlias(context.Background(), "https://example.com/redirect", fetchMetadata{
		ResolvedURL: "https://feeds.example.com:443/rss.xml#response",
	})
	if !errors.Is(err, domainrss.ErrDuplicateFeed) {
		t.Fatalf("resolved alias error = %v, want ErrDuplicateFeed", err)
	}
}
