package rss_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	applicationrss "xiadown/internal/application/rss"
	domainrss "xiadown/internal/domain/rss"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/rssrepo"
)

func TestUpdateSubscriptionReturnsAuthoritativeResolvedViewType(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "update-subscription-view.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repository := rssrepo.NewSQLiteRepository(database.Bun)
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	subscription := domainrss.Subscription{
		ID: "subscription-view", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "https://example.com/feed.xml", SiteURL: "https://example.com/",
		Title: "Example feed", ViewType: domainrss.ViewTypeAuto, Enabled: true,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	if _, err := repository.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_entries (
	id, subscription_id, external_id, title, kind, content_hash, created_at, modified_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, "article-entry", subscription.ID, "article-entry", "Article", domainrss.EntryKindArticle, "article-hash", now, now); err != nil {
		t.Fatal(err)
	}

	service := applicationrss.NewService(repository, nil)
	explicit, err := service.UpdateSubscription(ctx, applicationrss.UpdateSubscriptionRequest{
		ID: subscription.ID, ViewType: string(domainrss.ViewTypeVideo),
	})
	if err != nil {
		t.Fatal(err)
	}
	if explicit.ViewType != domainrss.ViewTypeVideo || explicit.ResolvedViewType != domainrss.ViewTypeVideo {
		t.Fatalf("explicit update returned view=%q resolved=%q, want video/video", explicit.ViewType, explicit.ResolvedViewType)
	}

	automatic, err := service.UpdateSubscription(ctx, applicationrss.UpdateSubscriptionRequest{
		ID: subscription.ID, ViewType: string(domainrss.ViewTypeAuto),
	})
	if err != nil {
		t.Fatal(err)
	}
	if automatic.ViewType != domainrss.ViewTypeAuto || automatic.ResolvedViewType != domainrss.ViewTypeArticle {
		t.Fatalf("automatic update returned view=%q resolved=%q, want auto/article", automatic.ViewType, automatic.ResolvedViewType)
	}
}
