package rss_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	applicationrss "xiadown/internal/application/rss"
	domainrss "xiadown/internal/domain/rss"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/rssrepo"
)

func TestPublicSyncIDBoundariesRejectDesktopLocalSourcesWhileDesktopRemainsAvailable(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "organization-public-id-boundaries.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := rssrepo.NewSQLiteRepository(database.Bun)
	service := applicationrss.NewService(repository, nil)
	now := time.Date(2026, 7, 17, 13, 0, 0, 0, time.UTC)
	subscription := domainrss.Subscription{
		ID: "source-subscription", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "xiadown-source://inbox/source", SiteURL: "https://publisher.example/",
		Title: "Desktop Inbox", IconURL: "https://cdn.example/icon.png",
		ViewType: domainrss.ViewTypeArticle, ResolvedViewType: domainrss.ViewTypeArticle,
		Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	source, err := repository.CreateSource(ctx, domainrss.Source{
		ID: "source", WorkspaceID: domainrss.DefaultWorkspaceID, SubscriptionID: subscription.ID,
		Kind: domainrss.SourceKindInbox, Handle: "desktop", Title: subscription.Title, Enabled: true,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}, subscription)
	if err != nil {
		t.Fatal(err)
	}
	entry := domainrss.Entry{
		ID: "source-entry", SubscriptionID: subscription.ID, ExternalID: "source-entry",
		URL: "https://publisher.example/posts/1", Title: "Desktop-only entry",
		ContentHTML: `<p>Desktop body</p>`, ImageURLs: []string{"https://cdn.example/body.png"},
		Kind: domainrss.EntryKindArticle, ContentHash: "source-entry",
		CreatedAt: now, ModifiedAt: now,
	}
	subscription.Revision = 2
	subscription.UpdatedAt = now.Add(time.Minute)
	if _, err := repository.UpsertFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription, Entries: []domainrss.Entry{entry},
	}); err != nil {
		t.Fatal(err)
	}
	entry, err = repository.GetSourceEntry(ctx, source.ID, entry.ExternalID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.GetSyncEntry(ctx, applicationrss.SubscriptionRequest{ID: entry.ID}); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired detail error=%v, want ErrNotFound", err)
	}
	if desktop, err := service.GetEntry(ctx, applicationrss.SubscriptionRequest{ID: entry.ID}); err != nil || desktop.ID != entry.ID {
		t.Fatalf("desktop detail=%#v err=%v", desktop, err)
	}
	read := true
	expectedRevision := int64(0)
	stateRequest := applicationrss.SetEntryStateRequest{
		ID: entry.ID, Field: domainrss.EntryStateFieldRead, Read: &read,
		ExpectedRevision: &expectedRevision, MutationID: "source-read",
	}
	if _, err := service.SetEntryStateForDevice(ctx, "paired-device", stateRequest); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired state error=%v, want ErrNotFound", err)
	}
	stateRequest.MutationID = "desktop-source-read"
	if state, err := service.SetEntryState(ctx, stateRequest); err != nil || !state.Read {
		t.Fatalf("desktop state=%#v err=%v", state, err)
	}

	if _, err := service.ResolveSyncSubscriptionResource(ctx, subscription.ID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired subscription icon error=%v, want ErrNotFound", err)
	}
	if icon, err := service.ResolveSubscriptionResource(ctx, subscription.ID); err != nil || icon.URL != subscription.IconURL {
		t.Fatalf("desktop subscription icon=%#v err=%v", icon, err)
	}
	if _, err := service.ResolveSyncEntryResource(ctx, entry.ID, "image-0"); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired entry resource error=%v, want ErrNotFound", err)
	}
	if image, err := service.ResolveEntryResource(ctx, entry.ID, "image-0"); err != nil || image.URL != entry.ImageURLs[0] {
		t.Fatalf("desktop entry resource=%#v err=%v", image, err)
	}
}

func TestOrganizationServiceCreatesSourceTimelineAndCollectionWithoutRefresh(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "organization-service.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := applicationrss.NewService(rssrepo.NewSQLiteRepository(database.Bun), nil)

	source, err := service.CreateSource(ctx, applicationrss.CreateSourceRequest{
		Kind: "inbox", Handle: "reading", Title: "Reading Inbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.CreateSourceEntry(ctx, applicationrss.CreateSourceEntryRequest{
		SourceID: source.ID, ExternalID: "message-1", Title: "Saved message",
		URL:         "https://example.com/messages/1",
		ContentHTML: `<p>Hello</p><script>alert("no")</script>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(entry.ContentHTML), "script") {
		t.Fatalf("source entry was not sanitized: %q", entry.ContentHTML)
	}
	feeds, err := service.ListSubscriptions(ctx)
	if err != nil || len(feeds) != 0 {
		t.Fatalf("internal source appeared as feed: %#v err=%v", feeds, err)
	}
	inboxPage, err := service.ListEntries(ctx, applicationrss.ListEntriesRequest{SourceKind: "inbox"})
	if err != nil || inboxPage.Total != 1 || inboxPage.Items[0].ID != entry.ID {
		t.Fatalf("inbox page=%#v err=%v", inboxPage, err)
	}
	notificationPage, err := service.ListEntries(ctx, applicationrss.ListEntriesRequest{SourceKind: "notification"})
	if err != nil || notificationPage.Total != 0 {
		t.Fatalf("notification page=%#v err=%v", notificationPage, err)
	}

	collection, err := service.CreateCollection(ctx, applicationrss.CreateCollectionRequest{
		Title: "Research", Kind: "entries", ViewType: "article",
	})
	if err != nil {
		t.Fatal(err)
	}
	collection, err = service.AddCollectionItems(ctx, applicationrss.UpdateCollectionItemsRequest{
		ID: collection.ID, ItemIDs: []string{entry.ID},
	})
	if err != nil || collection.ItemCount != 1 || collection.UnreadCount != 1 {
		t.Fatalf("collection=%#v err=%v", collection, err)
	}
	result, err := service.MarkAllRead(ctx, applicationrss.MarkAllReadRequest{CollectionID: collection.ID})
	if err != nil || result.Updated != 1 {
		t.Fatalf("mark collection read=%#v err=%v", result, err)
	}
	collectionPage, err := service.ListEntries(ctx, applicationrss.ListEntriesRequest{
		CollectionID: collection.ID, UnreadOnly: true,
	})
	if err != nil || collectionPage.Total != 0 {
		t.Fatalf("unread collection page=%#v err=%v", collectionPage, err)
	}
	sources, err := service.ListSources(ctx)
	if err != nil || len(sources) != 1 || sources[0].UnreadCount != 0 || sources[0].Kind != domainrss.SourceKindInbox {
		t.Fatalf("sources=%#v err=%v", sources, err)
	}
}
