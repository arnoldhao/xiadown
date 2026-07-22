package rssrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteRepositorySubscriptionMutationsAreRevisionGuardedAndChangeTracked(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "subscription-mutations.db")
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)

	empty, err := repo.ListChanges(ctx, 0, 10)
	if err != nil {
		t.Fatalf("list initial changes: %v", err)
	}
	assertSyncEpoch(t, empty.Epoch)
	if len(empty.Changes) != 0 || empty.Cursor != 0 || empty.HighWater != 0 || empty.HasMore {
		t.Fatalf("unexpected initial change page: %#v", empty)
	}

	subscription := testSubscription("subscription-1", "https://example.com/feed.xml", now)
	created, err := repo.CreateSubscription(ctx, subscription)
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if created.ID != subscription.ID || created.WorkspaceID != domainrss.DefaultWorkspaceID || created.Revision != 1 {
		t.Fatalf("unexpected created subscription: %#v", created)
	}

	// A retried create is rejected as the same logical feed and must not append
	// another change record.
	if _, err := repo.CreateSubscription(ctx, subscription); !errors.Is(err, domainrss.ErrDuplicateFeed) {
		t.Fatalf("duplicate create error = %v, want ErrDuplicateFeed", err)
	}
	duplicateURL := testSubscription("subscription-2", subscription.FeedURL, now)
	if _, err := repo.CreateSubscription(ctx, duplicateURL); !errors.Is(err, domainrss.ErrDuplicateFeed) {
		t.Fatalf("duplicate feed URL error = %v, want ErrDuplicateFeed", err)
	}

	updated := subscription
	updated.Title = "Renamed feed"
	updated.Revision = 2
	updated.UpdatedAt = now.Add(time.Minute)
	if _, err := repo.UpdateSubscription(ctx, updated); err != nil {
		t.Fatalf("update subscription: %v", err)
	}

	stale := subscription
	stale.Title = "Stale writer"
	stale.Revision = 2
	stale.UpdatedAt = now.Add(2 * time.Minute)
	if _, err := repo.UpdateSubscription(ctx, stale); !errors.Is(err, domainrss.ErrRevisionConflict) {
		t.Fatalf("stale update error = %v, want ErrRevisionConflict", err)
	}
	stored, err := repo.GetSubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("get updated subscription: %v", err)
	}
	if stored.Title != updated.Title || stored.Revision != updated.Revision || !stored.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("stale writer changed subscription: %#v", stored)
	}
	listed, err := repo.ListSubscriptions(ctx)
	if err != nil {
		t.Fatalf("list updated subscriptions: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != updated.ID || listed[0].Title != updated.Title || listed[0].Revision != 2 {
		t.Fatalf("unexpected subscription list: %#v", listed)
	}

	firstPage, err := repo.ListChanges(ctx, 0, 1)
	if err != nil {
		t.Fatalf("list first change page: %v", err)
	}
	if firstPage.Epoch != empty.Epoch || len(firstPage.Changes) != 1 || !firstPage.HasMore ||
		firstPage.Cursor != firstPage.Changes[0].Sequence || firstPage.HighWater != 2 {
		t.Fatalf("unexpected first change page: %#v", firstPage)
	}
	firstChange := firstPage.Changes[0]
	if firstChange.EntityType != "subscription" || firstChange.EntityID != subscription.ID ||
		firstChange.Operation != "upsert" || firstChange.Revision != 1 {
		t.Fatalf("unexpected create change: %#v", firstChange)
	}
	secondPage, err := repo.ListChanges(ctx, firstPage.Cursor, 1)
	if err != nil {
		t.Fatalf("list second change page: %v", err)
	}
	if secondPage.Epoch != empty.Epoch || len(secondPage.Changes) != 1 || secondPage.HasMore ||
		secondPage.Cursor != secondPage.HighWater || secondPage.HighWater != firstPage.HighWater {
		t.Fatalf("unexpected second change page: %#v", secondPage)
	}
	secondChange := secondPage.Changes[0]
	if secondChange.EntityType != "subscription" || secondChange.EntityID != subscription.ID ||
		secondChange.Operation != "upsert" || secondChange.Revision != 2 {
		t.Fatalf("unexpected update change: %#v", secondChange)
	}
	var changedSubscription domainrss.Subscription
	if err := json.Unmarshal(secondChange.Payload, &changedSubscription); err != nil {
		t.Fatalf("decode subscription change: %v", err)
	}
	if changedSubscription.Title != updated.Title || changedSubscription.Revision != 2 {
		t.Fatalf("unexpected subscription change payload: %#v", changedSubscription)
	}

	var changeCount int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_changes").Scan(&changeCount); err != nil {
		t.Fatalf("count subscription changes: %v", err)
	}
	if changeCount != 2 {
		t.Fatalf("duplicate/conflicting subscription writes appended %d changes, want 2", changeCount)
	}
}

func TestSQLiteRepositoryCategoriesAndCollectionsProvideOrderedAggregateTimelines(t *testing.T) {
	ctx, _, repo := openTestRSSRepository(t, "organization-timelines.db")
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	category, err := repo.CreateCategory(ctx, domainrss.Category{
		ID: "category-tech", WorkspaceID: domainrss.DefaultWorkspaceID, Title: "Tech",
		SortOrder: 0, CreatedAt: now, UpdatedAt: now, Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	first := testSubscription("organization-feed-1", "https://example.com/one.xml", now)
	first.CategoryID = category.ID
	first.SortOrder = 1
	second := testSubscription("organization-feed-2", "https://example.com/two.xml", now)
	second.CategoryID = category.ID
	second.SortOrder = 0
	entries := []domainrss.Entry{
		{ID: "organization-entry-1", SubscriptionID: first.ID, ExternalID: "one", Title: "One", Kind: domainrss.EntryKindArticle, ContentHash: "one", CreatedAt: now, ModifiedAt: now},
		{ID: "organization-entry-2", SubscriptionID: second.ID, ExternalID: "two", Title: "Two", Kind: domainrss.EntryKindArticle, ContentHash: "two", CreatedAt: now, ModifiedAt: now},
	}
	if _, _, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{Subscription: first, Entries: entries[:1]}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{Subscription: second, Entries: entries[1:]}); err != nil {
		t.Fatal(err)
	}

	categories, err := repo.ListCategories(ctx)
	if err != nil || len(categories) != 1 || categories[0].SubscriptionCount != 2 || categories[0].UnreadCount != 2 {
		t.Fatalf("categories=%#v err=%v", categories, err)
	}
	subscriptions, err := repo.ListSubscriptions(ctx)
	if err != nil || len(subscriptions) != 2 || subscriptions[0].ID != second.ID || subscriptions[1].ID != first.ID {
		t.Fatalf("ordered subscriptions=%#v err=%v", subscriptions, err)
	}
	categoryPage, err := repo.ListEntries(ctx, domainrss.EntryQuery{CategoryID: category.ID})
	if err != nil || categoryPage.Total != 2 {
		t.Fatalf("category page=%#v err=%v", categoryPage, err)
	}

	feedList, err := repo.CreateCollection(ctx, domainrss.Collection{
		ID: "collection-feeds", WorkspaceID: domainrss.DefaultWorkspaceID, Title: "Daily",
		Kind: domainrss.CollectionKindSubscriptions, ViewType: domainrss.ViewTypeAuto,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	feedList, err = repo.AddCollectionItems(ctx, feedList.ID, feedList.Kind, []string{first.ID}, now)
	if err != nil || feedList.ItemCount != 1 || feedList.UnreadCount != 1 {
		t.Fatalf("first collection add=%#v err=%v", feedList, err)
	}
	feedList, err = repo.AddCollectionItems(ctx, feedList.ID, feedList.Kind, []string{first.ID, second.ID}, now.Add(time.Minute))
	if err != nil || feedList.ItemCount != 2 || feedList.UnreadCount != 2 {
		t.Fatalf("incremental collection add=%#v err=%v", feedList, err)
	}
	items, err := repo.ListCollectionItems(ctx, feedList.ID)
	if err != nil || len(items.ItemIDs) != 2 || items.ItemIDs[0] != first.ID || items.ItemIDs[1] != second.ID {
		t.Fatalf("collection items=%#v err=%v", items, err)
	}
	feedPage, err := repo.ListEntries(ctx, domainrss.EntryQuery{CollectionID: feedList.ID})
	if err != nil || feedPage.Total != 2 {
		t.Fatalf("feed collection page=%#v err=%v", feedPage, err)
	}

	saved, err := repo.CreateCollection(ctx, domainrss.Collection{
		ID: "collection-entries", WorkspaceID: domainrss.DefaultWorkspaceID, Title: "Research",
		Kind: domainrss.CollectionKindEntries, ViewType: domainrss.ViewTypeArticle,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	saved, err = repo.ReplaceCollectionItems(ctx, saved.ID, saved.Kind, []string{entries[1].ID}, now)
	if err != nil || saved.ItemCount != 1 || saved.UnreadCount != 1 {
		t.Fatalf("saved collection=%#v err=%v", saved, err)
	}
	savedPage, err := repo.ListEntries(ctx, domainrss.EntryQuery{CollectionID: saved.ID})
	if err != nil || savedPage.Total != 1 || savedPage.Items[0].ID != entries[1].ID {
		t.Fatalf("saved collection page=%#v err=%v", savedPage, err)
	}
}

func TestSQLiteRepositoryCollectionItemLimitCoversDuplicatesBatchesAndDirectWrites(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "collection-item-limit.db")
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	seedCollectionLimitSubscriptions(t, ctx, database, 10_002)
	collection, err := repo.CreateCollection(ctx, domainrss.Collection{
		ID: "collection-limit", WorkspaceID: domainrss.DefaultWorkspaceID, Title: "Limit",
		Kind: domainrss.CollectionKindSubscriptions, ViewType: domainrss.ViewTypeAuto,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_collection_subscriptions (collection_id, subscription_id, sort_order, added_at)
SELECT ?, id, CAST(substr(id, 11) AS INTEGER) - 1, ?
FROM rss_subscriptions
WHERE id BETWEEN 'limit-sub-00001' AND 'limit-sub-09999'
`, collection.ID, now); err != nil {
		t.Fatal(err)
	}

	collection, err = repo.AddCollectionItems(ctx, collection.ID, collection.Kind,
		[]string{"limit-sub-00001", "limit-sub-10000"}, now.Add(time.Minute))
	if err != nil || collection.ItemCount != 10_000 || collection.Revision != 2 {
		t.Fatalf("boundary add with duplicate = %#v, %v", collection, err)
	}
	unchanged, err := repo.AddCollectionItems(ctx, collection.ID, collection.Kind,
		[]string{"limit-sub-00001", "limit-sub-10000"}, now.Add(2*time.Minute))
	if err != nil || unchanged.ItemCount != 10_000 || unchanged.Revision != collection.Revision {
		t.Fatalf("duplicate-only add at limit = %#v, %v", unchanged, err)
	}
	if _, err := repo.AddCollectionItems(ctx, collection.ID, collection.Kind,
		[]string{"limit-sub-10001"}, now.Add(3*time.Minute)); !errors.Is(err, domainrss.ErrInvalidRequest) {
		t.Fatalf("incremental over-limit add error = %v, want ErrInvalidRequest", err)
	}
	stored, err := repo.GetCollection(ctx, collection.ID)
	if err != nil || stored.ItemCount != 10_000 || stored.Revision != collection.Revision {
		t.Fatalf("failed add changed collection = %#v, %v", stored, err)
	}

	allIDs := make([]string, 10_001)
	for index := range allIDs {
		allIDs[index] = fmt.Sprintf("limit-sub-%05d", index+1)
	}
	if _, err := repo.ReplaceCollectionItems(ctx, collection.ID, collection.Kind, allIDs,
		now.Add(4*time.Minute)); !errors.Is(err, domainrss.ErrInvalidRequest) {
		t.Fatalf("over-limit replace error = %v, want ErrInvalidRequest", err)
	}
	stored, err = repo.GetCollection(ctx, collection.ID)
	if err != nil || stored.ItemCount != 10_000 || stored.Revision != collection.Revision {
		t.Fatalf("failed replace was not atomic = %#v, %v", stored, err)
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_collection_subscriptions (collection_id, subscription_id, sort_order, added_at)
VALUES (?, 'limit-sub-10001', 10000, ?)
`, collection.ID, now.Add(5*time.Minute)); err == nil || !strings.Contains(strings.ToLower(err.Error()), "item limit") {
		t.Fatalf("direct SQL bypass error = %v, want collection limit trigger", err)
	}

	overflowSource, err := repo.CreateCollection(ctx, domainrss.Collection{
		ID: "collection-limit-update-source", WorkspaceID: domainrss.DefaultWorkspaceID, Title: "Update source",
		Kind: domainrss.CollectionKindSubscriptions, ViewType: domainrss.ViewTypeAuto,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_collection_subscriptions (collection_id, subscription_id, sort_order, added_at)
VALUES (?, 'limit-sub-10001', 0, ?)
`, overflowSource.ID, now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE rss_collection_subscriptions
SET collection_id = ?
WHERE collection_id = ? AND subscription_id = 'limit-sub-10001'
`, collection.ID, overflowSource.ID); err == nil || !strings.Contains(strings.ToLower(err.Error()), "item limit") {
		t.Fatalf("direct UPDATE bypass error = %v, want collection limit trigger", err)
	}
	var fullCount, sourceCount int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_collection_subscriptions WHERE collection_id = ?
`, collection.ID).Scan(&fullCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_collection_subscriptions WHERE collection_id = ?
`, overflowSource.ID).Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if fullCount != 10_000 || sourceCount != 1 {
		t.Fatalf("failed UPDATE changed collection sizes: full=%d source=%d", fullCount, sourceCount)
	}
}

func TestSQLiteRepositoryConcurrentCollectionAddsCannotExceedLimit(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "collection-item-limit-concurrent.db")
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	seedCollectionLimitSubscriptions(t, ctx, database, 10_001)
	collection, err := repo.CreateCollection(ctx, domainrss.Collection{
		ID: "collection-limit-concurrent", WorkspaceID: domainrss.DefaultWorkspaceID, Title: "Concurrent Limit",
		Kind: domainrss.CollectionKindSubscriptions, ViewType: domainrss.ViewTypeAuto,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_collection_subscriptions (collection_id, subscription_id, sort_order, added_at)
SELECT ?, id, CAST(substr(id, 11) AS INTEGER) - 1, ?
FROM rss_subscriptions
WHERE id BETWEEN 'limit-sub-00001' AND 'limit-sub-09999'
`, collection.ID, now); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []string{"limit-sub-10000", "limit-sub-10001"} {
		id := id
		go func() {
			<-start
			_, addErr := repo.AddCollectionItems(ctx, collection.ID, collection.Kind, []string{id}, now.Add(time.Minute))
			results <- addErr
		}()
	}
	close(start)
	successes, rejected := 0, 0
	for range 2 {
		switch addErr := <-results; {
		case addErr == nil:
			successes++
		case errors.Is(addErr, domainrss.ErrInvalidRequest):
			rejected++
		default:
			t.Fatalf("unexpected concurrent add error: %v", addErr)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("concurrent adds: success=%d rejected=%d, want 1/1", successes, rejected)
	}
	stored, err := repo.GetCollection(ctx, collection.ID)
	if err != nil || stored.ItemCount != 10_000 || stored.Revision != 2 {
		t.Fatalf("concurrent final collection = %#v, %v", stored, err)
	}
}

func seedCollectionLimitSubscriptions(
	t *testing.T,
	ctx context.Context,
	database *persistence.Database,
	count int,
) {
	t.Helper()
	if count < 1 || count > 100_000 {
		t.Fatalf("invalid collection-limit fixture count: %d", count)
	}
	_, err := database.SQL.ExecContext(ctx, `
WITH digits(value) AS (
  VALUES (0),(1),(2),(3),(4),(5),(6),(7),(8),(9)
), numbers(value) AS (
	SELECT ten_thousands.value * 10000 + thousands.value * 1000 + hundreds.value * 100 + tens.value * 10 + ones.value + 1
	FROM digits AS ten_thousands
	CROSS JOIN digits AS thousands
  CROSS JOIN digits AS hundreds
  CROSS JOIN digits AS tens
  CROSS JOIN digits AS ones
)
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, enabled, created_at, updated_at, revision
)
SELECT printf('limit-sub-%05d', value), 'rss-default',
       printf('https://example.com/limit/%d.xml', value),
       printf('Limit %d', value), 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
FROM numbers
WHERE value <= ?
`, count)
	if err != nil {
		t.Fatalf("seed collection-limit subscriptions: %v", err)
	}
}

func TestSQLiteRepositoryLocalSourcesStayOutOfFeedListAndShareEntryStateQueries(t *testing.T) {
	ctx, _, repo := openTestRSSRepository(t, "local-sources.db")
	now := time.Date(2026, 7, 17, 11, 0, 0, 0, time.UTC)
	subscription := testSubscription("source-subscription", "xiadown-source://inbox/source-inbox", now)
	subscription.Title = "Reading Inbox"
	source, err := repo.CreateSource(ctx, domainrss.Source{
		ID: "source-inbox", WorkspaceID: domainrss.DefaultWorkspaceID, SubscriptionID: subscription.ID,
		Kind: domainrss.SourceKindInbox, Handle: "reading", Title: subscription.Title, Enabled: true,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}, subscription)
	if err != nil {
		t.Fatal(err)
	}
	feeds, err := repo.ListSubscriptions(ctx)
	if err != nil || len(feeds) != 0 {
		t.Fatalf("internal source leaked into feeds: %#v err=%v", feeds, err)
	}
	sources, err := repo.ListSources(ctx)
	if err != nil || len(sources) != 1 || sources[0].ID != source.ID || sources[0].Title != subscription.Title {
		t.Fatalf("sources=%#v err=%v", sources, err)
	}

	entry := domainrss.Entry{
		ID: "source-entry", SubscriptionID: subscription.ID, ExternalID: "message-1",
		Title: "Inbox message", Kind: domainrss.EntryKindArticle, ContentHash: "message-1",
		CreatedAt: now, ModifiedAt: now,
	}
	subscription.Revision++
	subscription.UpdatedAt = now.Add(time.Minute)
	if _, err := repo.UpsertFeed(ctx, domainrss.FeedUpdate{Subscription: subscription, Entries: []domainrss.Entry{entry}}); err != nil {
		t.Fatal(err)
	}
	defaultPage, err := repo.ListEntries(ctx, domainrss.EntryQuery{})
	if err != nil || defaultPage.Total != 0 {
		t.Fatalf("internal source leaked into default timeline: %#v err=%v", defaultPage, err)
	}
	articlePage, err := repo.ListEntries(ctx, domainrss.EntryQuery{Kind: domainrss.EntryKindArticle})
	if err != nil || articlePage.Total != 0 {
		t.Fatalf("internal source leaked into article timeline: %#v err=%v", articlePage, err)
	}
	subscriptionPage, err := repo.ListEntries(ctx, domainrss.EntryQuery{SubscriptionID: subscription.ID})
	if err != nil || subscriptionPage.Total != 1 || subscriptionPage.Items[0].ID != entry.ID {
		t.Fatalf("explicit source subscription page=%#v err=%v", subscriptionPage, err)
	}
	inboxPage, err := repo.ListEntries(ctx, domainrss.EntryQuery{SourceKind: domainrss.SourceKindInbox})
	if err != nil || inboxPage.Total != 1 || inboxPage.Items[0].ID != entry.ID {
		t.Fatalf("inbox page=%#v err=%v", inboxPage, err)
	}
	notificationPage, err := repo.ListEntries(ctx, domainrss.EntryQuery{SourceKind: domainrss.SourceKindNotification})
	if err != nil || notificationPage.Total != 0 {
		t.Fatalf("notification page=%#v err=%v", notificationPage, err)
	}
	sources, err = repo.ListSources(ctx)
	if err != nil || sources[0].UnreadCount != 1 {
		t.Fatalf("source unread projection=%#v err=%v", sources, err)
	}
}

func TestSQLiteRepositoryDesktopLocalSourcesNeverLeakIntoPublicSync(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "desktop-local-source-sync.db")
	now := time.Date(2026, 7, 17, 11, 30, 0, 0, time.UTC)
	scope := domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID}

	regularSubscription := testSubscription("public-subscription", "https://example.com/public.xml", now)
	regularEntry := domainrss.Entry{
		ID: "public-entry", SubscriptionID: regularSubscription.ID, ExternalID: "public-1",
		Title: "Public entry", Kind: domainrss.EntryKindArticle, ContentHash: "public-1",
		CreatedAt: now, ModifiedAt: now,
	}
	if _, _, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{
		Subscription: regularSubscription, Entries: []domainrss.Entry{regularEntry},
	}); err != nil {
		t.Fatal(err)
	}

	sourceSubscription := testSubscription("local-source-subscription", "xiadown-source://inbox/local-source", now)
	sourceSubscription.Title = "Local inbox"
	source, err := repo.CreateSource(ctx, domainrss.Source{
		ID: "local-source", WorkspaceID: domainrss.DefaultWorkspaceID, SubscriptionID: sourceSubscription.ID,
		Kind: domainrss.SourceKindInbox, Handle: "local", Title: sourceSubscription.Title, Enabled: true,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}, sourceSubscription)
	if err != nil {
		t.Fatal(err)
	}
	sourceEntry := domainrss.Entry{
		ID: "local-source-entry", SubscriptionID: sourceSubscription.ID, ExternalID: "local-1",
		Title: "Local entry", Kind: domainrss.EntryKindArticle, ContentHash: "local-1",
		CreatedAt: now, ModifiedAt: now,
	}
	sourceSubscription.Revision = 2
	sourceSubscription.UpdatedAt = now.Add(time.Minute)
	if _, err := repo.UpsertFeed(ctx, domainrss.FeedUpdate{
		Subscription: sourceSubscription, Entries: []domainrss.Entry{sourceEntry},
	}); err != nil {
		t.Fatal(err)
	}
	storedSourceEntry, err := repo.GetSourceEntry(ctx, source.ID, sourceEntry.ExternalID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetSyncSubscription(ctx, sourceSubscription.ID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired subscription detail error=%v, want ErrNotFound", err)
	}
	if _, err := repo.GetSyncEntry(ctx, storedSourceEntry.ID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired entry detail error=%v, want ErrNotFound", err)
	}
	if desktopEntry, err := repo.GetEntry(ctx, storedSourceEntry.ID); err != nil || desktopEntry.ID != storedSourceEntry.ID {
		t.Fatalf("desktop source detail=%#v err=%v", desktopEntry, err)
	}
	read := true
	localMutation := domainrss.StateMutation{
		Scope: scope, EntryID: storedSourceEntry.ID, Field: domainrss.EntryStateFieldRead, Read: &read,
		ExpectedRevision: 0, DeviceID: "desktop-local", MutationID: "local-read",
		RequestHash: strings.Repeat("b", 64), ChangedAt: now.Add(2 * time.Minute), AllowDesktopLocal: true,
	}
	localState, err := repo.ApplyStateMutation(ctx, localMutation)
	if err != nil {
		t.Fatal(err)
	}
	pairedReplay := localMutation
	pairedReplay.AllowDesktopLocal = false
	if _, err := repo.ApplyStateMutation(ctx, pairedReplay); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired replay of desktop source receipt error=%v, want ErrNotFound", err)
	}
	pairedNew := pairedReplay
	pairedNew.DeviceID = "paired-device"
	pairedNew.MutationID = "paired-local-read"
	if _, err := repo.ApplyStateMutation(ctx, pairedNew); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired source mutation error=%v, want ErrNotFound", err)
	}
	source.Title = "Renamed local inbox"
	source.Revision = 2
	source.UpdatedAt = now.Add(3 * time.Minute)
	sourceSubscription.Title = source.Title
	sourceSubscription.Revision = 3
	sourceSubscription.UpdatedAt = source.UpdatedAt
	if _, err := repo.UpdateSource(ctx, source, sourceSubscription); err != nil {
		t.Fatal(err)
	}

	var writeChangeCount int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_changes").Scan(&writeChangeCount); err != nil {
		t.Fatal(err)
	}
	if writeChangeCount != 2 {
		t.Fatalf("source writes appended public changes: count=%d, want only public subscription+entry", writeChangeCount)
	}

	insertLegacySourceChange := func(entityType, entityID, subjectID string, revision int64, payload any, changedAt time.Time) {
		t.Helper()
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_changes (
  workspace_id, subject_id, entity_type, entity_id, operation, revision, payload_json, changed_at
) VALUES (?, ?, ?, ?, 'upsert', ?, ?, ?)
`, domainrss.DefaultWorkspaceID, subjectID, entityType, entityID, revision, string(encoded), changedAt); err != nil {
			t.Fatal(err)
		}
	}
	insertLegacySourceChange(
		"subscription", sourceSubscription.ID, "", sourceSubscription.Revision,
		syncSubscriptionProjection(sourceSubscription), now.Add(4*time.Minute),
	)
	insertLegacySourceChange(
		"entry", storedSourceEntry.ID, "", storedSourceEntry.Revision,
		syncEntryProjection(storedSourceEntry), now.Add(5*time.Minute),
	)

	regularSubscription.Title = "Updated public subscription"
	regularSubscription.Revision = 2
	regularSubscription.UpdatedAt = now.Add(6 * time.Minute)
	if _, err := repo.UpdateSubscription(ctx, regularSubscription); err != nil {
		t.Fatal(err)
	}
	insertLegacySourceChange(
		"entry_state", storedSourceEntry.ID, domainrss.DefaultSubjectID, localState.Revision,
		localState, now.Add(7*time.Minute),
	)

	lightweightSubscriptions, err := repo.ListLightweightSyncSubscriptions(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(lightweightSubscriptions) != 1 || lightweightSubscriptions[0].ID != regularSubscription.ID {
		t.Fatalf("lightweight subscriptions leaked source: %#v", lightweightSubscriptions)
	}
	syncEntries, err := repo.ListSyncEntries(ctx, domainrss.EntryQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if syncEntries.Total != 1 || len(syncEntries.Items) != 1 || syncEntries.Items[0].ID != regularEntry.ID {
		t.Fatalf("sync entries leaked source: %#v", syncEntries)
	}
	localOnlyEntries, err := repo.ListSyncEntries(ctx, domainrss.EntryQuery{
		SubscriptionID: sourceSubscription.ID, Limit: 100,
	})
	if err != nil || localOnlyEntries.Total != 0 || len(localOnlyEntries.Items) != 0 {
		t.Fatalf("source-specific public entry query=%#v err=%v", localOnlyEntries, err)
	}

	overview, err := repo.GetSyncOverview(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	firstSnapshot, err := repo.ListSyncSnapshot(ctx, domainrss.SyncSnapshotQuery{
		Scope: scope, Epoch: overview.Epoch, HighWater: overview.HighWater, Stage: "subscriptions", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstSnapshot.Records) != 1 || firstSnapshot.Records[0].EntityID != regularSubscription.ID ||
		!firstSnapshot.HasMore || firstSnapshot.NextStage != "subscriptions" {
		t.Fatalf("first filtered snapshot page=%#v", firstSnapshot)
	}
	secondSnapshot, err := repo.ListSyncSnapshot(ctx, domainrss.SyncSnapshotQuery{
		Scope: scope, Epoch: overview.Epoch, HighWater: overview.HighWater,
		Stage: firstSnapshot.NextStage, AfterID: firstSnapshot.NextID, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(secondSnapshot.Records) != 1 || secondSnapshot.Records[0].EntityID != regularEntry.ID || secondSnapshot.HasMore {
		t.Fatalf("second filtered snapshot page=%#v", secondSnapshot)
	}

	visibleChanges := make([]domainrss.Change, 0)
	cursor := int64(0)
	pageCount := 0
	for {
		page, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
			Scope: scope, Epoch: overview.Epoch, After: cursor, Limit: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		pageCount++
		if page.Cursor <= cursor {
			t.Fatalf("filtered change cursor stalled: before=%d page=%#v", cursor, page)
		}
		visibleChanges = append(visibleChanges, page.Changes...)
		cursor = page.Cursor
		if !page.HasMore {
			if page.Cursor != overview.HighWater {
				t.Fatalf("final filtered cursor=%d, want high-water=%d", page.Cursor, overview.HighWater)
			}
			break
		}
	}
	if pageCount != 3 || len(visibleChanges) != 3 {
		t.Fatalf("visible change pages=%d changes=%#v", pageCount, visibleChanges)
	}
	for _, change := range visibleChanges {
		if change.EntityID == sourceSubscription.ID || change.EntityID == storedSourceEntry.ID {
			t.Fatalf("incremental changes leaked source: %#v", visibleChanges)
		}
	}
	emptyTail, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: overview.Epoch, After: cursor, Limit: 1,
	})
	if err != nil || len(emptyTail.Changes) != 0 || emptyTail.HasMore || emptyTail.Cursor != overview.HighWater {
		t.Fatalf("filtered journal tail=%#v err=%v", emptyTail, err)
	}

	if err := repo.DeleteSubscription(ctx, sourceSubscription.ID, now.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	var sourceReceipts int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_client_mutations WHERE entry_id = ?
`, storedSourceEntry.ID).Scan(&sourceReceipts); err != nil {
		t.Fatal(err)
	}
	if sourceReceipts != 0 {
		t.Fatalf("deleted source retained %d mutation receipts", sourceReceipts)
	}
	if _, err := repo.ApplyStateMutation(ctx, pairedReplay); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired replay after source deletion error=%v, want ErrNotFound", err)
	}
	postDeleteOverview, err := repo.GetSyncOverview(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if postDeleteOverview.HighWater != overview.HighWater {
		t.Fatalf("source deletion moved sync high-water: before=%d after=%d", overview.HighWater, postDeleteOverview.HighWater)
	}
	postDeleteChanges, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: overview.Epoch, After: 0, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(postDeleteChanges.Changes) != 3 || postDeleteChanges.Cursor != postDeleteOverview.HighWater || postDeleteChanges.HasMore {
		t.Fatalf("post-delete filtered changes=%#v", postDeleteChanges)
	}
	for _, change := range postDeleteChanges.Changes {
		if change.EntityID == sourceSubscription.ID || change.EntityID == storedSourceEntry.ID {
			t.Fatalf("deleted source leaked through incremental changes: %#v", postDeleteChanges)
		}
	}
	var publicDeleteTombstones, localMarkers int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_tombstones WHERE entity_type = 'subscription' AND entity_id = ?
`, sourceSubscription.ID).Scan(&publicDeleteTombstones); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_tombstones WHERE entity_type LIKE 'local_%' AND entity_id IN (?, ?)
`, sourceSubscription.ID, storedSourceEntry.ID).Scan(&localMarkers); err != nil {
		t.Fatal(err)
	}
	if publicDeleteTombstones != 0 || localMarkers != 2 {
		t.Fatalf("source delete tombstones public=%d localMarkers=%d", publicDeleteTombstones, localMarkers)
	}
	if _, err := database.SQL.ExecContext(ctx, `
DELETE FROM rss_tombstones WHERE entity_type LIKE 'local_%' AND entity_id IN (?, ?)
`, sourceSubscription.ID, storedSourceEntry.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyStateMutation(ctx, pairedReplay); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("paired replay after marker retention cleanup error=%v, want ErrNotFound", err)
	}
}

func TestSQLiteRepositoryLegacyLocalMarkersProtectStillPresentEntitiesByID(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "legacy-local-marker-id-boundary.db")
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	subscription := testSubscription("legacy-local-subscription", "https://example.com/legacy.xml", now)
	entry := domainrss.Entry{
		ID: "legacy-local-entry", SubscriptionID: subscription.ID, ExternalID: "legacy-local",
		Title: "Legacy local entry", Kind: domainrss.EntryKindArticle, ContentHash: "legacy-local",
		CreatedAt: now, ModifiedAt: now,
	}
	if _, _, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription, Entries: []domainrss.Entry{entry},
	}); err != nil {
		t.Fatal(err)
	}
	overview, err := repo.GetSyncOverview(ctx, domainrss.SyncScope{
		WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []struct {
		entityType string
		entityID   string
	}{
		{entityType: "local_subscription", entityID: subscription.ID},
		{entityType: "local_entry", entityID: entry.ID},
	} {
		if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_tombstones (workspace_id, entity_type, entity_id, deleted_sequence, deleted_at)
VALUES (?, ?, ?, ?, ?)
`, domainrss.DefaultWorkspaceID, marker.entityType, marker.entityID, overview.HighWater, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.GetSyncSubscription(ctx, subscription.ID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("marker-protected subscription error=%v, want ErrNotFound", err)
	}
	if _, err := repo.GetSyncEntry(ctx, entry.ID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("marker-protected entry error=%v, want ErrNotFound", err)
	}
	if desktopEntry, err := repo.GetEntry(ctx, entry.ID); err != nil || desktopEntry.ID != entry.ID {
		t.Fatalf("desktop marker-protected entry=%#v err=%v", desktopEntry, err)
	}
	read := true
	mutation := domainrss.StateMutation{
		Scope:   domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID},
		EntryID: entry.ID, Field: domainrss.EntryStateFieldRead, Read: &read, ExpectedRevision: 0,
		DeviceID: "paired-legacy", MutationID: "paired-legacy-read",
		RequestHash: strings.Repeat("c", 64), ChangedAt: now.Add(time.Minute),
	}
	if _, err := repo.ApplyStateMutation(ctx, mutation); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("marker-protected paired mutation error=%v, want ErrNotFound", err)
	}
	mutation.AllowDesktopLocal = true
	mutation.DeviceID = "desktop"
	mutation.MutationID = "desktop-legacy-read"
	if state, err := repo.ApplyStateMutation(ctx, mutation); err != nil || !state.Read {
		t.Fatalf("desktop marker-protected mutation=%#v err=%v", state, err)
	}
	var pairedReceipts int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_client_mutations WHERE device_id = 'paired-legacy'
`).Scan(&pairedReceipts); err != nil {
		t.Fatal(err)
	}
	if pairedReceipts != 0 {
		t.Fatalf("rejected marker-protected mutation wrote %d receipts", pairedReceipts)
	}
}

func TestSQLiteRepositoryListSubscriptionsResolvesAutoViewFromAllEntries(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "subscription-resolved-view.db")
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)

	testCases := []struct {
		id       string
		title    string
		viewType domainrss.ViewType
		kinds    []domainrss.EntryKind
		want     domainrss.ViewType
	}{
		{
			id: "dominant-article", title: "A", viewType: domainrss.ViewTypeAuto,
			kinds: []domainrss.EntryKind{
				domainrss.EntryKindArticle, domainrss.EntryKindArticle, domainrss.EntryKindArticle,
				domainrss.EntryKindArticle, domainrss.EntryKindArticle, domainrss.EntryKindArticle,
				domainrss.EntryKindVideo, domainrss.EntryKindVideo, domainrss.EntryKindVideo, domainrss.EntryKindVideo,
			},
			want: domainrss.ViewTypeArticle,
		},
		{
			id: "mixed-auto", title: "B", viewType: domainrss.ViewTypeAuto,
			kinds: []domainrss.EntryKind{
				domainrss.EntryKindArticle, domainrss.EntryKindArticle, domainrss.EntryKindArticle,
				domainrss.EntryKindArticle, domainrss.EntryKindArticle,
				domainrss.EntryKindVideo, domainrss.EntryKindVideo, domainrss.EntryKindVideo,
				domainrss.EntryKindVideo, domainrss.EntryKindVideo,
			},
			want: domainrss.ViewTypeAuto,
		},
		{
			id: "explicit-image", title: "C", viewType: domainrss.ViewTypeImage,
			kinds: []domainrss.EntryKind{domainrss.EntryKindArticle},
			want:  domainrss.ViewTypeImage,
		},
		{id: "empty-auto", title: "D", viewType: domainrss.ViewTypeAuto, want: domainrss.ViewTypeAuto},
	}

	for _, testCase := range testCases {
		subscription := testSubscription(testCase.id, "https://example.com/"+testCase.id+".xml", now)
		subscription.Title = testCase.title
		subscription.ViewType = testCase.viewType
		if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
			t.Fatalf("create %s: %v", testCase.id, err)
		}
		for index, kind := range testCase.kinds {
			entryID := fmt.Sprintf("%s-entry-%d", testCase.id, index)
			if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_entries (
	id, subscription_id, external_id, title, kind, content_hash, created_at, modified_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, entryID, testCase.id, entryID, entryID, kind, entryID+"-hash", now, now); err != nil {
				t.Fatalf("insert %s: %v", entryID, err)
			}
		}
	}

	listed, err := repo.ListSubscriptions(ctx)
	if err != nil {
		t.Fatalf("list subscriptions: %v", err)
	}
	resolved := make(map[string]domainrss.ViewType, len(listed))
	for _, subscription := range listed {
		resolved[subscription.ID] = subscription.ResolvedViewType
	}
	for _, testCase := range testCases {
		if got := resolved[testCase.id]; got != testCase.want {
			t.Errorf("%s resolved view=%q, want %q", testCase.id, got, testCase.want)
		}
	}
}

func TestSQLiteRepositoryCreateFeedPersistsInitialEntriesAndChangesAtomically(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "create-feed.db")
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	subscription := testSubscription("subscription-create-feed", "https://example.com/create.xml", now)
	entry := domainrss.Entry{
		ID: "entry-create-feed", SubscriptionID: subscription.ID, ExternalID: "post-1",
		URL: "https://example.com/posts/1", Title: "Initial entry", Kind: domainrss.EntryKindArticle,
		ContentHash: "initial-entry-hash", CreatedAt: now, ModifiedAt: now,
	}

	created, result, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription,
		Entries:      []domainrss.Entry{entry},
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if created.ID != subscription.ID || created.Revision != 1 || created.UnreadCount != 1 {
		t.Fatalf("created subscription = %#v", created)
	}
	if result.Created != 1 || result.Updated != 0 {
		t.Fatalf("create feed result = %#v", result)
	}
	page, err := repo.ListEntries(ctx, domainrss.EntryQuery{SubscriptionID: subscription.ID})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != entry.ID {
		t.Fatalf("initial entry page = %#v, error=%v", page, err)
	}
	changes, err := repo.ListChanges(ctx, 0, 10)
	if err != nil {
		t.Fatalf("list create-feed changes: %v", err)
	}
	if len(changes.Changes) != 2 || changes.HighWater != 2 ||
		changes.Changes[0].EntityType != "subscription" || changes.Changes[0].Revision != 1 ||
		changes.Changes[1].EntityType != "entry" || changes.Changes[1].Revision != 1 {
		t.Fatalf("create-feed changes = %#v", changes)
	}
	var changedSubscription domainrss.SyncSubscription
	if err := json.Unmarshal(changes.Changes[0].Payload, &changedSubscription); err != nil {
		t.Fatalf("decode create-feed subscription change: %v", err)
	}
	if changedSubscription.UnreadCount != created.UnreadCount {
		t.Fatalf("create-feed change unreadCount=%d, want %d", changedSubscription.UnreadCount, created.UnreadCount)
	}

	duplicate := testSubscription("subscription-duplicate-feed", subscription.FeedURL, now.Add(time.Minute))
	if _, _, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{Subscription: duplicate}); !errors.Is(err, domainrss.ErrDuplicateFeed) {
		t.Fatalf("duplicate CreateFeed error = %v, want ErrDuplicateFeed", err)
	}
	var subscriptionCount, entryCount, changeCount int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_subscriptions").Scan(&subscriptionCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_entries").Scan(&entryCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_changes").Scan(&changeCount); err != nil {
		t.Fatal(err)
	}
	if subscriptionCount != 1 || entryCount != 1 || changeCount != 2 {
		t.Fatalf("counts after duplicate create: subscriptions=%d entries=%d changes=%d", subscriptionCount, entryCount, changeCount)
	}
}

func TestSQLiteRepositoryListEntriesKeepsAllAndKindPagesOnOneSnapshot(t *testing.T) {
	ctx, _, repo := openTestRSSRepository(t, "mixed-kind-entry-page.db")
	now := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)
	articlePublishedAt := now.Add(time.Minute)
	videoPublishedAt := now.Add(2 * time.Minute)
	subscription := testSubscription("subscription-mixed-page", "https://example.com/mixed.xml", now)
	article := domainrss.Entry{
		ID: "entry-mixed-article", SubscriptionID: subscription.ID, ExternalID: "article-1",
		Title: "Mixed article", Kind: domainrss.EntryKindArticle, PublishedAt: &articlePublishedAt,
		ContentHash: "mixed-article-hash", CreatedAt: now, ModifiedAt: now,
	}
	video := domainrss.Entry{
		ID: "entry-mixed-video", SubscriptionID: subscription.ID, ExternalID: "video-1",
		Title: "Mixed video", Kind: domainrss.EntryKindVideo, PublishedAt: &videoPublishedAt,
		ContentHash: "mixed-video-hash", CreatedAt: now, ModifiedAt: now,
	}
	if _, _, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription,
		Entries:      []domainrss.Entry{article, video},
	}); err != nil {
		t.Fatal(err)
	}

	all, err := repo.ListEntries(ctx, domainrss.EntryQuery{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 || len(all.Items) != 2 || all.Snapshot <= 0 || all.NextOffset != 0 {
		t.Fatalf("all page = %#v", all)
	}
	if all.Items[0].ID != video.ID || all.Items[0].Kind != domainrss.EntryKindVideo ||
		all.Items[1].ID != article.ID || all.Items[1].Kind != domainrss.EntryKindArticle {
		t.Fatalf("all page dropped or reordered a kind: %#v", all.Items)
	}

	videos, err := repo.ListEntries(ctx, domainrss.EntryQuery{Kind: domainrss.EntryKindVideo, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if videos.Total != 1 || len(videos.Items) != 1 || videos.Items[0].ID != video.ID ||
		videos.Snapshot != all.Snapshot {
		t.Fatalf("video page = %#v, all snapshot=%d", videos, all.Snapshot)
	}

	first, err := repo.ListEntries(ctx, domainrss.EntryQuery{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.ListEntries(ctx, domainrss.EntryQuery{Limit: 1, Offset: first.NextOffset})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 2 || first.NextOffset != 1 || len(first.Items) != 1 ||
		len(second.Items) != 1 || second.NextOffset != 0 || first.Snapshot != second.Snapshot ||
		first.Items[0].ID != video.ID || second.Items[0].ID != article.ID {
		t.Fatalf("mixed pagination first=%#v second=%#v", first, second)
	}
}

func TestSQLiteRepositoryListEntriesFiltersStarredCollectionsInSQL(t *testing.T) {
	ctx, _, repo := openTestRSSRepository(t, "starred-entry-filter.db")
	now := time.Date(2026, 7, 14, 14, 0, 0, 0, time.UTC)
	starredAt := now.Add(time.Minute)
	readAt := now.Add(2 * time.Minute)

	first := testSubscription("subscription-starred-a", "https://example.com/starred-a.xml", now)
	_, _, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{
		Subscription: first,
		Entries: []domainrss.Entry{
			{
				ID: "entry-a-starred-unread", SubscriptionID: first.ID, ExternalID: "a-1",
				Title: "Starred article", Kind: domainrss.EntryKindArticle, StarredAt: &starredAt,
				ContentHash: "a-1-hash", CreatedAt: now, ModifiedAt: now,
			},
			{
				ID: "entry-a-starred-read", SubscriptionID: first.ID, ExternalID: "a-2",
				Title: "Starred video", Kind: domainrss.EntryKindVideo, StarredAt: &starredAt, ReadAt: &readAt,
				ContentHash: "a-2-hash", CreatedAt: now, ModifiedAt: now,
			},
			{
				ID: "entry-a-plain", SubscriptionID: first.ID, ExternalID: "a-3",
				Title: "Plain article", Kind: domainrss.EntryKindArticle,
				ContentHash: "a-3-hash", CreatedAt: now, ModifiedAt: now,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	second := testSubscription("subscription-starred-b", "https://example.com/starred-b.xml", now)
	_, _, err = repo.CreateFeed(ctx, domainrss.FeedUpdate{
		Subscription: second,
		Entries: []domainrss.Entry{{
			ID: "entry-b-starred-unread", SubscriptionID: second.ID, ExternalID: "b-1",
			Title: "Starred image", Kind: domainrss.EntryKindImage, StarredAt: &starredAt,
			ContentHash: "b-1-hash", CreatedAt: now, ModifiedAt: now,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	starred, err := repo.ListEntries(ctx, domainrss.EntryQuery{StarredOnly: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if starred.Total != 3 || len(starred.Items) != 1 || starred.NextOffset != 1 || starred.Items[0].StarredAt == nil {
		t.Fatalf("starred page = %#v", starred)
	}

	unread, err := repo.ListEntries(ctx, domainrss.EntryQuery{StarredOnly: true, UnreadOnly: true, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if unread.Total != 2 || len(unread.Items) != 2 {
		t.Fatalf("unread starred page = %#v", unread)
	}
	for _, item := range unread.Items {
		if item.StarredAt == nil || item.ReadAt != nil {
			t.Fatalf("unread starred filter leaked item = %#v", item)
		}
	}

	firstSubscription, err := repo.ListEntries(ctx, domainrss.EntryQuery{
		SubscriptionID: first.ID, StarredOnly: true, Limit: 20,
	})
	if err != nil || firstSubscription.Total != 2 {
		t.Fatalf("subscription starred page = %#v, error=%v", firstSubscription, err)
	}
	articles, err := repo.ListEntries(ctx, domainrss.EntryQuery{
		Kind: domainrss.EntryKindArticle, StarredOnly: true, Limit: 20,
	})
	if err != nil || articles.Total != 1 || len(articles.Items) != 1 || articles.Items[0].ID != "entry-a-starred-unread" {
		t.Fatalf("article starred page = %#v, error=%v", articles, err)
	}
}

func TestSQLiteRepositoryUpsertFeedChangeIncludesPostWriteUnreadCount(t *testing.T) {
	ctx, _, repo := openTestRSSRepository(t, "upsert-feed-unread-change.db")
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	subscription := testSubscription("subscription-upsert-unread", "https://example.com/upsert.xml", now)
	if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	subscription.Revision = 2
	subscription.UpdatedAt = now.Add(time.Minute)
	entry := domainrss.Entry{
		ID: "entry-upsert-unread", SubscriptionID: subscription.ID, ExternalID: "post-1",
		Title: "New unread entry", Kind: domainrss.EntryKindArticle,
		ContentHash: "new-unread-hash", CreatedAt: now, ModifiedAt: now,
	}
	if _, err := repo.UpsertFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription,
		Entries:      []domainrss.Entry{entry},
	}); err != nil {
		t.Fatal(err)
	}
	changes, err := repo.ListChanges(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Changes) != 3 || changes.Changes[1].EntityType != "subscription" ||
		changes.Changes[1].Revision != 2 || changes.Changes[2].EntityType != "entry" {
		t.Fatalf("upsert-feed changes = %#v", changes)
	}
	var changed domainrss.SyncSubscription
	if err := json.Unmarshal(changes.Changes[1].Payload, &changed); err != nil {
		t.Fatal(err)
	}
	if changed.UnreadCount != 1 {
		t.Fatalf("upsert-feed subscription change = %#v", changed)
	}
}

func TestSQLiteRepositoryCreateFeedRollsBackSubscriptionWhenAnEntryFails(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "create-feed-rollback.db")
	now := time.Date(2026, 7, 14, 10, 30, 0, 0, time.UTC)
	subscription := testSubscription("subscription-rollback", "https://example.com/rollback.xml", now)
	entries := []domainrss.Entry{
		{
			ID: "duplicate-entry-id", SubscriptionID: subscription.ID, ExternalID: "post-1",
			Title: "First", Kind: domainrss.EntryKindArticle, ContentHash: "first-hash",
			CreatedAt: now, ModifiedAt: now,
		},
		{
			ID: "duplicate-entry-id", SubscriptionID: subscription.ID, ExternalID: "post-2",
			Title: "Second", Kind: domainrss.EntryKindArticle, ContentHash: "second-hash",
			CreatedAt: now, ModifiedAt: now,
		},
	}

	if _, _, err := repo.CreateFeed(ctx, domainrss.FeedUpdate{Subscription: subscription, Entries: entries}); err == nil {
		t.Fatal("CreateFeed succeeded with duplicate entry primary keys")
	}
	if _, err := repo.GetSubscription(ctx, subscription.ID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("subscription survived failed CreateFeed: %v", err)
	}
	var subscriptionCount, entryCount, changeCount int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_subscriptions").Scan(&subscriptionCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_entries").Scan(&entryCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_changes").Scan(&changeCount); err != nil {
		t.Fatal(err)
	}
	if subscriptionCount != 0 || entryCount != 0 || changeCount != 0 {
		t.Fatalf("failed CreateFeed left rows: subscriptions=%d entries=%d changes=%d", subscriptionCount, entryCount, changeCount)
	}
}

func TestSQLiteRepositoryPersistsValidatorProvenanceAcrossUpdateAndFeedUpsert(t *testing.T) {
	ctx, _, repo := openTestRSSRepository(t, "subscription-validator-provenance.db")
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	subscription := testSubscription("subscription-validator", "https://example.com/feed.xml", now)
	if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	subscription.Revision = 2
	subscription.UpdatedAt = now.Add(time.Minute)
	subscription.ETag = `"v1"`
	subscription.LastModified = "Mon, 13 Jul 2026 10:00:00 GMT"
	subscription.ValidatorURL = "https://cdn.example.com/effective.xml?source=one"
	persisted, err := repo.UpdateSubscription(ctx, subscription)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetSubscription(ctx, subscription.ID)
	if err != nil || stored.ETag != subscription.ETag || stored.LastModified != subscription.LastModified ||
		stored.ValidatorURL != subscription.ValidatorURL {
		t.Fatalf("updated validator provenance = %#v, error=%v", stored, err)
	}

	subscription.Revision = persisted.Revision + 1
	subscription.UpdatedAt = now.Add(2 * time.Minute)
	subscription.ETag = `"v2"`
	subscription.LastModified = ""
	subscription.ValidatorURL = "https://mirror.example.com/feed.xml"
	if _, err := repo.UpsertFeed(ctx, domainrss.FeedUpdate{Subscription: subscription}); err != nil {
		t.Fatal(err)
	}
	stored, err = repo.GetSubscription(ctx, subscription.ID)
	if err != nil || stored.ETag != subscription.ETag || stored.LastModified != "" ||
		stored.ValidatorURL != subscription.ValidatorURL {
		t.Fatalf("upserted validator provenance = %#v, error=%v", stored, err)
	}
}

func TestSQLiteRepositoryRefreshOnlyUpdatesDoNotAdvancePublicRevisionOrChangeFeed(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "subscription-refresh-only.db")
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	subscription := testSubscription("subscription-refresh-only", "https://example.com/feed.xml", now)
	if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}

	fetchedAt := now.Add(30 * time.Minute)
	subscription.Revision++
	subscription.UpdatedAt = fetchedAt
	subscription.LastFetchedAt = &fetchedAt
	subscription.LastError = "temporary upstream failure"
	persisted, err := repo.UpdateSubscription(ctx, subscription)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Revision != 1 || !persisted.UpdatedAt.Equal(now) {
		t.Fatalf("refresh-only update advanced public version: %#v", persisted)
	}
	stored, err := repo.GetSubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 1 || !stored.UpdatedAt.Equal(now) || stored.LastFetchedAt == nil ||
		!stored.LastFetchedAt.Equal(fetchedAt) || stored.LastError == "" {
		t.Fatalf("refresh-only state = %#v", stored)
	}

	succeededAt := now.Add(time.Hour)
	stored.Revision++
	stored.UpdatedAt = succeededAt
	stored.LastFetchedAt = &succeededAt
	stored.LastSuccessAt = &succeededAt
	stored.LastError = ""
	stored.ETag = `"unchanged"`
	if _, err := repo.UpsertFeed(ctx, domainrss.FeedUpdate{Subscription: stored}); err != nil {
		t.Fatal(err)
	}
	stored, err = repo.GetSubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 1 || !stored.UpdatedAt.Equal(now) || stored.LastSuccessAt == nil ||
		!stored.LastSuccessAt.Equal(succeededAt) || stored.ETag != `"unchanged"` {
		t.Fatalf("unchanged feed refresh state = %#v", stored)
	}
	var changes int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_changes").Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if changes != 1 {
		t.Fatalf("refresh-only updates appended %d changes, want only the create", changes)
	}
}

func TestSQLiteRepositoryLightweightListsAndSnapshotNeverHydrateArticleBodies(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "lightweight-entry-projections.db")
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	subscription := testSubscription("subscription-lightweight", "https://example.com/light.xml", now)
	if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	largeBody := "<p>" + strings.Repeat("large-body-", 3000) + "</p>"
	tx, err := database.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	statement, err := tx.PrepareContext(ctx, `
INSERT INTO rss_entries (
  id, subscription_id, external_id, title, content_html, content_hash, created_at, modified_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	for index := 0; index < 500; index++ {
		id := fmt.Sprintf("entry-large-%03d", index)
		if _, err := statement.ExecContext(ctx, id, subscription.ID, id, "Large entry", largeBody, id+"-hash", now, now); err != nil {
			_ = statement.Close()
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	_ = statement.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	page, err := repo.ListEntries(ctx, domainrss.EntryQuery{Limit: 500})
	if err != nil || len(page.Items) != 500 {
		t.Fatalf("list entries count=%d error=%v", len(page.Items), err)
	}
	for _, item := range page.Items {
		if item.ContentHTML != "" {
			t.Fatalf("list hydrated %d body bytes", len(item.ContentHTML))
		}
	}
	detail, err := repo.GetEntry(ctx, "entry-large-000")
	if err != nil || detail.ContentHTML != largeBody {
		t.Fatalf("detail body bytes=%d error=%v", len(detail.ContentHTML), err)
	}

	scope := domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID}
	overview, err := repo.GetSyncOverview(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.ListSyncSnapshot(ctx, domainrss.SyncSnapshotQuery{
		Scope: scope, Epoch: overview.Epoch, HighWater: overview.HighWater, Stage: "entries", Limit: 500,
	})
	if err != nil || len(snapshot.Records) != 500 {
		t.Fatalf("snapshot records=%d error=%v", len(snapshot.Records), err)
	}
	for _, record := range snapshot.Records {
		if strings.Contains(string(record.Payload), "large-body-") || strings.Contains(string(record.Payload), "contentHtml") {
			t.Fatalf("snapshot hydrated article content: %s", record.Payload)
		}
	}
}

func TestSQLiteRepositorySyncEntryQueriesNeverSelectOrDecodeRemoteSourceArrays(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "sync-entry-remote-projections.db")
	querySQL := selectLightweightSyncEntryColumns(
		repo.db.NewSelect().Model((*lightweightSyncEntryRow)(nil)), "entry_row",
	).String()
	for _, forbidden := range []string{
		`"entry_row"."external_id"`, `"entry_row"."url"`, `"entry_row"."content_html"`,
		`"entry_row"."image_urls_json"`, `"entry_row"."media_json"`, `"entry_row"."media_url"`,
		`"entry_row"."media_type"`, `"entry_row"."content_hash"`,
	} {
		if strings.Contains(querySQL, forbidden) {
			t.Fatalf("sync SELECT contains forbidden source column %s: %s", forbidden, querySQL)
		}
	}
	if !strings.Contains(querySQL, "thumbnail_available") || !strings.Contains(querySQL, "CASE WHEN") {
		t.Fatalf("sync SELECT does not derive thumbnail availability: %s", querySQL)
	}

	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	subscription := testSubscription("subscription-sync-lightweight", "https://example.com/sync.xml", now)
	if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	const marker = "never-hydrate-source-secret"
	images := make([]string, 0, 64)
	media := make([]domainrss.Media, 0, 64)
	for index := 0; index < 64; index++ {
		suffix := fmt.Sprintf("%s-%02d-%s", marker, index, strings.Repeat("x", 480))
		imageURL := "https://cdn.example/images/" + suffix
		mediaURL := "https://cdn.example/media/" + suffix
		images = append(images, imageURL)
		media = append(media, domainrss.Media{
			URL: mediaURL, MIMEType: "video/mp4", Kind: "video", Thumbnail: imageURL,
		})
	}
	imagesJSON, err := json.Marshal(images)
	if err != nil {
		t.Fatal(err)
	}
	mediaJSON, err := json.Marshal(media)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := database.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	statement, err := tx.PrepareContext(ctx, `
INSERT INTO rss_entries (
  id, subscription_id, external_id, url, title, author, summary, kind,
  content_html, image_urls_json, media_json, media_url, media_type, thumbnail_url,
  platform, platform_video_id, content_hash, created_at, modified_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	for index := 0; index < 500; index++ {
		id := fmt.Sprintf("entry-sync-large-%03d", index)
		if _, err := statement.ExecContext(
			ctx, id, subscription.ID, id, "https://source.example/"+marker,
			"Needle video", "Author", "Summary", string(domainrss.EntryKindVideo),
			"<p>"+marker+"</p>", string(imagesJSON), string(mediaJSON), "https://cdn.example/"+marker,
			"video/mp4", images[0], "generic", "video-id", id+"-hash", now, now,
		); err != nil {
			_ = statement.Close()
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	_ = statement.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	page, err := repo.ListSyncEntries(ctx, domainrss.EntryQuery{
		SubscriptionID: subscription.ID, Kind: domainrss.EntryKindVideo,
		Query: "needle", UnreadOnly: true, Limit: 500,
	})
	if err != nil || len(page.Items) != 500 || page.Total != 500 {
		t.Fatalf("sync entries count=%d total=%d error=%v", len(page.Items), page.Total, err)
	}
	for _, item := range page.Items {
		encoded, marshalErr := json.Marshal(item)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if strings.Contains(string(encoded), marker) || !item.ThumbnailAvailable {
			t.Fatalf("sync item leaked or lost derived availability: %s", encoded)
		}
	}

	scope := domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID}
	overview, err := repo.GetSyncOverview(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.ListSyncSnapshot(ctx, domainrss.SyncSnapshotQuery{
		Scope: scope, Epoch: overview.Epoch, HighWater: overview.HighWater, Stage: "entries", Limit: 500,
	})
	if err != nil || len(snapshot.Records) != 500 {
		t.Fatalf("snapshot records=%d error=%v", len(snapshot.Records), err)
	}
	for _, record := range snapshot.Records {
		if strings.Contains(string(record.Payload), marker) {
			t.Fatalf("snapshot leaked a source-only field: %s", record.Payload)
		}
	}
}

func TestSyncEntryFromLightweightRowMapsStateWithoutSourceFields(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 30, 0, 0, time.UTC)
	fraction, contentRevision := 0.75, int64(9)
	videoProgress, videoDuration := 12.5, 20.0
	row := lightweightSyncEntryRow{
		ID: "entry-1", SubscriptionID: "subscription-1", Title: "Title", Author: "Author", Summary: "Summary",
		Kind: domainrss.EntryKindVideo, ThumbnailAvailable: true, Platform: "youtube", PlatformVideoID: "video-1",
		PublishedAt: &now, ReadAt: &now, StarredAt: &now,
		ArticleProgressFraction: &fraction, ArticleProgressAnchor: "section-2", ArticleProgressContentRevision: &contentRevision,
		VideoProgressSeconds: &videoProgress, VideoDurationSeconds: &videoDuration, VideoCompleted: false,
		ReadRevision: 1, StarredRevision: 2, ArticleProgressRevision: 3, VideoProgressSecondsRevision: 4,
		StateRevision: 5, ContentRevision: 6, CreatedAt: now, ModifiedAt: now,
	}
	item := syncEntryFromLightweightRow(row)
	if item.ID != row.ID || !item.ThumbnailAvailable || !item.Read || !item.Starred || item.ArticleProgress == nil ||
		item.ArticleProgress.Fraction != fraction || item.ArticleProgress.Anchor != "section-2" ||
		item.VideoProgressSeconds == nil || *item.VideoProgressSeconds != videoProgress ||
		item.FieldRevisions.Read != 1 || item.FieldRevisions.Starred != 2 ||
		item.FieldRevisions.ArticleProgress != 3 || item.FieldRevisions.VideoProgressSeconds != 4 ||
		item.StateRevision != 5 || item.ContentRevision != 6 {
		t.Fatalf("mapped sync entry = %#v", item)
	}
}

func TestSQLiteRepositoryDiscoveryCacheRoundTripAndAtomicReplacement(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "discovery-cache.db")
	fetchedAt := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	first := domainrss.DiscoveryCache{
		SourceURL: "https://raw.githubusercontent.com/DIYgod/RSSHub/refs/heads/gh-pages/build/routes.json",
		FetchedAt: fetchedAt,
		Routes: []domainrss.DiscoveryRoute{
			{
				ID: "rsshub:bilibili-user-video-123", Provider: "rsshub", Title: "Bilibili - User videos",
				URL: "rsshub://bilibili/user/video/123", Description: "User video updates",
				SourceID: "bilibili", SourceName: "Bilibili", SourceURL: "https://www.bilibili.com",
				SiteURL: "https://www.bilibili.com", RoutePath: "bilibili/user/video/:uid",
				ExamplePath: "bilibili/user/video/123", Categories: []string{"multimedia", "new-media"},
				Heat: 88, Language: "zh-CN", Region: "CN", ViewType: domainrss.ViewTypeVideo,
				RequiresConfig: true, RequiresPuppeteer: true,
				NeedsParameters: true,
				Parameters: []domainrss.DiscoveryParameter{
					{
						Name: "uid", Description: "User ID", ExampleValue: "123", Type: "string",
						Options: []domainrss.DiscoveryParameterOption{},
					},
				},
			},
		},
	}
	if err := repo.ReplaceDiscoveryCache(ctx, first); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.LoadDiscoveryCache(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SourceURL != first.SourceURL || !loaded.FetchedAt.Equal(fetchedAt) || len(loaded.Routes) != 1 {
		t.Fatalf("loaded cache = %#v", loaded)
	}
	if got := loaded.Routes[0]; got.URL != first.Routes[0].URL || got.ViewType != domainrss.ViewTypeVideo ||
		!got.RequiresConfig || !got.RequiresPuppeteer || !got.NeedsParameters || len(got.Parameters) != 1 ||
		got.Parameters[0].Name != "uid" || got.Parameters[0].ExampleValue != "123" || len(got.Categories) != 2 {
		t.Fatalf("loaded route = %#v", got)
	}

	second := domainrss.DiscoveryCache{
		SourceURL: "https://cdn.jsdelivr.net/gh/DIYgod/RSSHub@gh-pages/build/routes.json",
		FetchedAt: fetchedAt.Add(time.Hour),
		Routes: []domainrss.DiscoveryRoute{
			{
				ID: "rsshub:youtube-user-openai", Provider: "rsshub", Title: "YouTube",
				URL: "rsshub://youtube/user/@OpenAI", RoutePath: "youtube/user/:id",
				ExamplePath: "youtube/user/@OpenAI", Categories: []string{"multimedia"},
				Language: "en", Region: "global", ViewType: domainrss.ViewTypeVideo,
			},
		},
	}
	if err := repo.ReplaceDiscoveryCache(ctx, second); err != nil {
		t.Fatal(err)
	}
	loaded, err = repo.LoadDiscoveryCache(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SourceURL != second.SourceURL || len(loaded.Routes) != 1 || loaded.Routes[0].ID != second.Routes[0].ID {
		t.Fatalf("replacement cache = %#v", loaded)
	}
	var staleRows, metaCount int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_discovery_routes WHERE id = ?`, first.Routes[0].ID).Scan(&staleRows); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT route_count FROM rss_discovery_meta WHERE source = 'rsshub'`).Scan(&metaCount); err != nil {
		t.Fatal(err)
	}
	if staleRows != 0 || metaCount != 1 {
		t.Fatalf("staleRows=%d metaCount=%d", staleRows, metaCount)
	}
}

func TestSQLiteRepositoryDiscoveryCacheStoresMultipleTemplateRoutes(t *testing.T) {
	ctx, _, repo := openTestRSSRepository(t, "discovery-template-cache.db")
	defaultValue := "featured"
	cache := domainrss.DiscoveryCache{
		SourceURL: "https://example.com/routes.json", FetchedAt: time.Date(2026, 7, 13, 11, 0, 0, 0, time.UTC),
		Routes: []domainrss.DiscoveryRoute{
			{
				ID: "route-user", Provider: "rsshub", Title: "User", URL: "rsshub://example/user/:id",
				RoutePath: "example/user/:id", Categories: []string{"social-media"}, ViewType: domainrss.ViewTypeSocial,
				NeedsParameters: true, Parameters: []domainrss.DiscoveryParameter{
					{Name: "id", Description: "User ID", DefaultValue: &defaultValue, Type: "string", Options: []domainrss.DiscoveryParameterOption{}},
				},
			},
			{
				ID: "route-channel", Provider: "rsshub", Title: "Channel", URL: "rsshub://example/channel/:id",
				RoutePath: "example/channel/:id", Categories: []string{"multimedia"}, ViewType: domainrss.ViewTypeVideo,
				NeedsParameters: true, Parameters: []domainrss.DiscoveryParameter{
					{Name: "id", Description: "Channel ID", ExampleValue: "news", Type: "string", Options: []domainrss.DiscoveryParameterOption{}},
				},
			},
			{
				ID: "route-latest", Provider: "rsshub", Title: "Latest", URL: "rsshub://example/latest",
				RoutePath: "example/latest", Categories: []string{"other"}, ViewType: domainrss.ViewTypeArticle,
				Parameters: []domainrss.DiscoveryParameter{},
			},
		},
	}
	if err := repo.ReplaceDiscoveryCache(ctx, cache); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.LoadDiscoveryCache(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Routes) != 3 {
		t.Fatalf("routes = %#v", loaded.Routes)
	}
	byPath := make(map[string]domainrss.DiscoveryRoute, len(loaded.Routes))
	for _, route := range loaded.Routes {
		byPath[route.RoutePath] = route
	}
	if user := byPath["example/user/:id"]; !user.NeedsParameters || len(user.Parameters) != 1 ||
		user.Parameters[0].DefaultValue == nil || *user.Parameters[0].DefaultValue != "featured" {
		t.Fatalf("user route = %#v", user)
	}
	if channel := byPath["example/channel/:id"]; !channel.NeedsParameters || channel.Parameters[0].ExampleValue != "news" {
		t.Fatalf("channel route = %#v", channel)
	}
	if latest := byPath["example/latest"]; latest.NeedsParameters || len(latest.Parameters) != 0 {
		t.Fatalf("latest route = %#v", latest)
	}
}

func TestSQLiteRepositoryQueryDiscoveryFiltersCountsAndPaginatesInSQLite(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "discovery-query.db")
	fetchedAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	defaultValue := "featured"
	cache := domainrss.DiscoveryCache{
		SourceURL: "https://example.com/routes.json", FetchedAt: fetchedAt,
		Routes: []domainrss.DiscoveryRoute{
			{
				ID: "bili-video", Provider: "rsshub", Title: "Bilibili Videos", URL: "rsshub://bilibili/video/:uid",
				RoutePath: "bilibili/video/:uid", Categories: []string{"multimedia", "new-media"},
				Heat: 90, Language: "zh-CN", ViewType: domainrss.ViewTypeVideo, NeedsParameters: true,
				Parameters: []domainrss.DiscoveryParameter{{Name: "uid", DefaultValue: &defaultValue, Type: "string", Options: []domainrss.DiscoveryParameterOption{}}},
			},
			{ID: "video-archive", Provider: "rsshub", Title: "Video Archive", URL: "rsshub://archive/latest", RoutePath: "archive/latest", Categories: []string{"multimedia"}, Heat: 80, Language: "zh-CN", ViewType: domainrss.ViewTypeVideo, Parameters: []domainrss.DiscoveryParameter{}},
			{ID: "weibo", Provider: "rsshub", Title: "Weibo Timeline", URL: "rsshub://weibo/latest", RoutePath: "weibo/latest", Categories: []string{"social-media"}, Heat: 70, Language: "zh-CN", ViewType: domainrss.ViewTypeAuto, Parameters: []domainrss.DiscoveryParameter{}},
			{ID: "javascript", Provider: "rsshub", Title: "JavaScript Weekly", URL: "rsshub://javascript/weekly", RoutePath: "javascript/weekly", Categories: []string{"programming"}, Heat: 60, Language: "zh-CN", ViewType: domainrss.ViewTypeAuto, Parameters: []domainrss.DiscoveryParameter{}},
			{ID: "broken-json", Provider: "rsshub", Title: "Broken metadata", URL: "rsshub://broken/latest", RoutePath: "broken/latest", Categories: []string{"other"}, Heat: 50, Language: "zh-CN", ViewType: domainrss.ViewTypeAuto, Parameters: []domainrss.DiscoveryParameter{}},
			{ID: "youtube", Provider: "rsshub", Title: "YouTube Videos", URL: "rsshub://youtube/latest", RoutePath: "youtube/latest", Categories: []string{"multimedia"}, Heat: 100, Language: "en", ViewType: domainrss.ViewTypeAuto, Parameters: []domainrss.DiscoveryParameter{}},
		},
	}
	if err := repo.ReplaceDiscoveryCache(ctx, cache); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `UPDATE rss_discovery_routes SET categories_json = '{broken' WHERE id = 'broken-json'`); err != nil {
		t.Fatal(err)
	}

	state, err := repo.GetDiscoveryState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.RouteCount != 6 || state.SourceURL != cache.SourceURL || !state.FetchedAt.Equal(fetchedAt) {
		t.Fatalf("state = %#v", state)
	}
	page, err := repo.QueryDiscovery(ctx, domainrss.DiscoveryQuery{
		Query: "VIDEO", CategoryID: "multimedia", Language: "zh-CN", Sort: "popular", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.State.RouteCount != 6 || page.FilteredRouteCount != 2 || len(page.Routes) != 1 ||
		page.Routes[0].ID != "bili-video" || !page.HasMore || page.Offset != 0 || page.Limit != 1 {
		t.Fatalf("page = %#v", page)
	}
	if len(page.Routes[0].Parameters) != 1 || page.Routes[0].Parameters[0].DefaultValue == nil ||
		*page.Routes[0].Parameters[0].DefaultValue != defaultValue {
		t.Fatalf("parameters = %#v", page.Routes[0].Parameters)
	}
	categories := make(map[string]domainrss.DiscoveryCategory, len(page.Categories))
	for _, category := range page.Categories {
		categories[category.ID] = category
	}
	if categories["all"].Count != 5 || categories["multimedia"].Count != 2 ||
		len(categories["multimedia"].Examples) != 2 || categories["other"].Count != 0 {
		t.Fatalf("categories = %#v", page.Categories)
	}

	second, err := repo.QueryDiscovery(ctx, domainrss.DiscoveryQuery{
		Query: "video", CategoryID: "multimedia", Language: "zh-CN", Sort: "popular", Offset: 1, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.FilteredRouteCount != 2 || len(second.Routes) != 1 || second.Routes[0].ID != "video-archive" || second.HasMore {
		t.Fatalf("second page = %#v", second)
	}
	exactCategory, err := repo.QueryDiscovery(ctx, domainrss.DiscoveryQuery{CategoryID: "media", Language: "zh-CN", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if exactCategory.FilteredRouteCount != 0 || len(exactCategory.Routes) != 0 {
		t.Fatalf("non-exact category matched = %#v", exactCategory)
	}
	exactRoute, err := repo.QueryDiscovery(ctx, domainrss.DiscoveryQuery{RouteID: "video-archive", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if exactRoute.FilteredRouteCount != 1 || len(exactRoute.Routes) != 1 || exactRoute.Routes[0].ID != "video-archive" {
		t.Fatalf("exact route = %#v", exactRoute)
	}
	missingRoute, err := repo.QueryDiscovery(ctx, domainrss.DiscoveryQuery{RouteID: "video", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if missingRoute.FilteredRouteCount != 0 || len(missingRoute.Routes) != 0 {
		t.Fatalf("non-exact route matched = %#v", missingRoute)
	}
	iconRoute, err := repo.FindDiscoveryRoute(ctx, domainrss.DiscoveryQuery{RouteID: "video-archive"})
	if err != nil || iconRoute.ID != "video-archive" {
		t.Fatalf("route icon lookup = %#v, err = %v", iconRoute, err)
	}
	categoryIconRoute, err := repo.FindDiscoveryRoute(ctx, domainrss.DiscoveryQuery{CategoryID: "multimedia"})
	if err != nil || categoryIconRoute.ID != "youtube" {
		t.Fatalf("category icon lookup = %#v, err = %v", categoryIconRoute, err)
	}
	if _, err := repo.FindDiscoveryRoute(ctx, domainrss.DiscoveryQuery{RouteID: "missing"}); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("missing icon route error = %v", err)
	}

	// SQL search deliberately uses substring matching for all query lengths. This
	// broadens the former <=2-rune token-prefix rule while keeping count and
	// pagination entirely inside SQLite.
	shortQuery, err := repo.QueryDiscovery(ctx, domainrss.DiscoveryQuery{Query: "av", Language: "zh-CN", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if shortQuery.FilteredRouteCount != 1 || len(shortQuery.Routes) != 1 || shortQuery.Routes[0].ID != "javascript" {
		t.Fatalf("short substring query = %#v", shortQuery)
	}

	broadQuery, err := repo.QueryDiscovery(ctx, domainrss.DiscoveryQuery{Query: "bilibili archive", Language: "zh-CN", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if broadQuery.FilteredRouteCount != 2 || len(broadQuery.Routes) != 2 ||
		broadQuery.Routes[0].ID != "bili-video" || broadQuery.Routes[1].ID != "video-archive" {
		t.Fatalf("multi-token broad query = %#v", broadQuery)
	}
	punctuationOnly, err := repo.QueryDiscovery(ctx, domainrss.DiscoveryQuery{Query: "!!!", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if punctuationOnly.FilteredRouteCount != 0 || len(punctuationOnly.Routes) != 0 {
		t.Fatalf("punctuation-only query = %#v", punctuationOnly)
	}
}

func TestSQLiteRepositoryReadMutationIsIdempotentAndRevisionGuarded(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "read-mutations.db")
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	entry := seedTestEntry(t, ctx, repo, now)

	expectedZero := int64(0)
	readAt := now.Add(2 * time.Minute)
	first, err := repo.ApplyReadMutation(ctx, domainrss.ReadMutation{
		EntryID: entry.ID, Read: true, ExpectedRevision: &expectedZero,
		DeviceID: "ios-device", MutationID: "mutation-read-1", ChangedAt: readAt,
	})
	if err != nil {
		t.Fatalf("apply read mutation: %v", err)
	}
	if !first.Read || first.ReadAt == nil || !first.ReadAt.Equal(readAt) || first.Revision != 1 ||
		first.UpdatedBy != "ios-device" || first.MutationID != "mutation-read-1" ||
		first.SubjectID != domainrss.DefaultSubjectID {
		t.Fatalf("unexpected first read state: %#v", first)
	}

	// A transport retry with the same canonical payload returns the original
	// result even though the field revision has advanced.
	replayed, err := repo.ApplyReadMutation(ctx, domainrss.ReadMutation{
		EntryID: entry.ID, Read: true, ExpectedRevision: &expectedZero,
		DeviceID: "ios-device", MutationID: "mutation-read-1", ChangedAt: readAt.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("replay read mutation: %v", err)
	}
	if !replayed.Read || replayed.ReadAt == nil || !replayed.ReadAt.Equal(readAt) || replayed.Revision != 1 ||
		!replayed.UpdatedAt.Equal(first.UpdatedAt) || replayed.MutationID != first.MutationID {
		t.Fatalf("idempotent replay = %#v, want original %#v", replayed, first)
	}
	bogusRevision := int64(99)
	if _, err := repo.ApplyReadMutation(ctx, domainrss.ReadMutation{
		EntryID: entry.ID, Read: false, ExpectedRevision: &bogusRevision,
		DeviceID: "ios-device", MutationID: "mutation-read-1", ChangedAt: readAt.Add(2 * time.Hour),
	}); !errors.Is(err, domainrss.ErrIdempotencyConflict) {
		t.Fatalf("idempotency key accepted a different payload: %v", err)
	}

	stored, err := repo.GetEntry(ctx, entry.ID)
	if err != nil {
		t.Fatalf("get read entry: %v", err)
	}
	if stored.StateRevision != 1 || stored.ReadAt == nil || !stored.ReadAt.Equal(readAt) {
		t.Fatalf("unexpected persisted read state: %#v", stored)
	}
	if unreadCount := queryUnreadCount(t, ctx, database, entry.SubscriptionID); unreadCount != 0 {
		t.Fatalf("unread count after read = %d, want 0", unreadCount)
	}

	beforeConflict, err := repo.ListChanges(ctx, 0, 100)
	if err != nil {
		t.Fatalf("list changes before conflict: %v", err)
	}
	if _, err := repo.ApplyReadMutation(ctx, domainrss.ReadMutation{
		EntryID: entry.ID, Read: false, ExpectedRevision: &expectedZero,
		DeviceID: "ios-device", MutationID: "mutation-stale", ChangedAt: readAt.Add(time.Minute),
	}); !errors.Is(err, domainrss.ErrRevisionConflict) {
		t.Fatalf("stale read mutation error = %v, want ErrRevisionConflict", err)
	}
	afterConflict, err := repo.ListChanges(ctx, 0, 100)
	if err != nil {
		t.Fatalf("list changes after conflict: %v", err)
	}
	if afterConflict.HighWater != beforeConflict.HighWater || len(afterConflict.Changes) != len(beforeConflict.Changes) {
		t.Fatalf("conflicting read mutation advanced changes: before=%#v after=%#v", beforeConflict, afterConflict)
	}
	var staleMutationCount int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_client_mutations
WHERE device_id = 'ios-device' AND mutation_id = 'mutation-stale'
`).Scan(&staleMutationCount); err != nil {
		t.Fatalf("count stale mutations: %v", err)
	}
	if staleMutationCount != 0 {
		t.Fatalf("conflicting read mutation left %d idempotency rows", staleMutationCount)
	}

	expectedOne := int64(1)
	unreadAt := readAt.Add(2 * time.Minute)
	unread, err := repo.ApplyReadMutation(ctx, domainrss.ReadMutation{
		EntryID: entry.ID, Read: false, ExpectedRevision: &expectedOne,
		DeviceID: "ios-device", MutationID: "mutation-read-2", ChangedAt: unreadAt,
	})
	if err != nil {
		t.Fatalf("apply unread mutation: %v", err)
	}
	if unread.Read || unread.ReadAt != nil || unread.Revision != 2 || !unread.UpdatedAt.Equal(unreadAt) {
		t.Fatalf("unexpected unread state: %#v", unread)
	}
	if unreadCount := queryUnreadCount(t, ctx, database, entry.SubscriptionID); unreadCount != 1 {
		t.Fatalf("unread count after marking unread = %d, want 1", unreadCount)
	}

	var stateChanges, clientMutations int
	if err := database.SQL.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rss_changes WHERE entity_type = 'entry_state'",
	).Scan(&stateChanges); err != nil {
		t.Fatalf("count entry state changes: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_client_mutations").Scan(&clientMutations); err != nil {
		t.Fatalf("count client mutations: %v", err)
	}
	if stateChanges != 2 || clientMutations != 2 {
		t.Fatalf("idempotency rows/change rows = (%d, %d), want (2, 2)", clientMutations, stateChanges)
	}
}

func TestSQLiteRepositoryDeleteWritesTombstoneAndCascadesFeedData(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "subscription-delete.db")
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	entry := seedTestEntry(t, ctx, repo, now)
	deletedAt := now.Add(3 * time.Minute)

	if err := repo.DeleteSubscription(ctx, entry.SubscriptionID, deletedAt); err != nil {
		t.Fatalf("delete subscription: %v", err)
	}

	var (
		deletedSequence int64
		changeSequence  int64
		operation       string
		revision        int64
		payloadJSON     string
		storedDeletedAt time.Time
	)
	err := database.SQL.QueryRowContext(ctx, `
SELECT t.deleted_sequence, c.sequence, c.operation, c.revision, c.payload_json, t.deleted_at
FROM rss_tombstones AS t
JOIN rss_changes AS c ON c.sequence = t.deleted_sequence
WHERE t.workspace_id = ? AND t.entity_type = 'subscription' AND t.entity_id = ?
`, domainrss.DefaultWorkspaceID, entry.SubscriptionID).Scan(
		&deletedSequence, &changeSequence, &operation, &revision, &payloadJSON, &storedDeletedAt,
	)
	if err != nil {
		t.Fatalf("read subscription tombstone: %v", err)
	}
	if deletedSequence <= 0 || deletedSequence != changeSequence || operation != "delete" || revision != 3 ||
		!storedDeletedAt.Equal(deletedAt) {
		t.Fatalf("unexpected tombstone/change pair: tombstone=%d change=%d operation=%q revision=%d deletedAt=%v",
			deletedSequence, changeSequence, operation, revision, storedDeletedAt)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		t.Fatalf("decode delete payload: %v", err)
	}
	if payload["id"] != entry.SubscriptionID {
		t.Fatalf("delete payload = %#v, want subscription ID", payload)
	}
	if _, err := repo.GetSubscription(ctx, entry.SubscriptionID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("get deleted subscription error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetEntry(ctx, entry.ID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("get cascaded entry error = %v, want ErrNotFound", err)
	}

	changePage, err := repo.ListChanges(ctx, deletedSequence-1, 10)
	if err != nil {
		t.Fatalf("list delete change: %v", err)
	}
	if len(changePage.Changes) != 1 || changePage.Changes[0].Sequence != deletedSequence ||
		changePage.Changes[0].Operation != "delete" {
		t.Fatalf("delete missing from change feed: %#v", changePage)
	}

	// A repeated delete reports absence without manufacturing a second
	// tombstone or delete event.
	if err := repo.DeleteSubscription(ctx, entry.SubscriptionID, deletedAt.Add(time.Hour)); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("repeated delete error = %v, want ErrNotFound", err)
	}
	var tombstones, deleteChanges int
	if err := database.SQL.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rss_tombstones WHERE entity_type = 'subscription' AND entity_id = ?", entry.SubscriptionID,
	).Scan(&tombstones); err != nil {
		t.Fatalf("count tombstones: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rss_changes WHERE entity_type = 'subscription' AND entity_id = ? AND operation = 'delete'", entry.SubscriptionID,
	).Scan(&deleteChanges); err != nil {
		t.Fatalf("count delete changes: %v", err)
	}
	if tombstones != 1 || deleteChanges != 1 {
		t.Fatalf("repeated delete produced tombstones=%d deleteChanges=%d, want 1 each", tombstones, deleteChanges)
	}
}

func TestSQLiteRepositorySyncEpochPersistsAcrossNormalReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sync-epoch.db")
	firstDatabase, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("open first sqlite: %v", err)
	}
	firstPage, err := NewSQLiteRepository(firstDatabase.Bun).ListChanges(ctx, 0, 1)
	if err != nil {
		_ = firstDatabase.Close()
		t.Fatalf("read first epoch: %v", err)
	}
	assertSyncEpoch(t, firstPage.Epoch)
	if err := firstDatabase.Close(); err != nil {
		t.Fatalf("close first sqlite: %v", err)
	}

	secondDatabase, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer secondDatabase.Close()
	secondPage, err := NewSQLiteRepository(secondDatabase.Bun).ListChanges(ctx, 0, 1)
	if err != nil {
		t.Fatalf("read reopened epoch: %v", err)
	}
	if secondPage.Epoch != firstPage.Epoch {
		t.Fatalf("sync epoch rotated on normal reopen: %q -> %q", firstPage.Epoch, secondPage.Epoch)
	}
}

func TestSQLiteRepositoryRebuildsPlatformAwareDownloadTargetsAfterReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "media-roundtrip.db")
	now := time.Date(2026, 7, 13, 11, 0, 0, 0, time.UTC)
	firstDatabase, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("open first sqlite: %v", err)
	}
	firstRepo := NewSQLiteRepository(firstDatabase.Bun)
	subscription := testSubscription("subscription-media", "https://example.com/media.xml", now)
	if _, err := firstRepo.CreateSubscription(ctx, subscription); err != nil {
		_ = firstDatabase.Close()
		t.Fatalf("create subscription: %v", err)
	}
	subscription.Revision = 2
	subscription.UpdatedAt = now.Add(time.Minute)
	entries := []domainrss.Entry{
		{
			ID: "rss-entry-generic", SubscriptionID: subscription.ID, ExternalID: "generic",
			URL: "https://example.com/posts/clip", Title: "Generic clip", Kind: domainrss.EntryKindVideo,
			Media:    []domainrss.Media{{URL: "https://cdn.example.com/clip.mp4", MIMEType: "video/mp4", Kind: "video"}},
			MediaURL: "https://cdn.example.com/clip.mp4", MediaType: "video/mp4", Platform: "generic",
			Revision: 1, ContentHash: "generic-hash", CreatedAt: now, ModifiedAt: now,
		},
		{
			ID: "rss-entry-youtube", SubscriptionID: subscription.ID, ExternalID: "youtube",
			URL: "https://www.youtube.com/watch?v=AbCdEfGhI12", Title: "YouTube clip", Kind: domainrss.EntryKindVideo,
			Media:    []domainrss.Media{{URL: "https://cdn.example.com/fallback.mp4", MIMEType: "video/mp4", Kind: "video"}},
			MediaURL: "https://cdn.example.com/fallback.mp4", MediaType: "video/mp4", Platform: "youtube",
			PlatformVideoID: "AbCdEfGhI12", Revision: 1, ContentHash: "youtube-hash", CreatedAt: now, ModifiedAt: now,
		},
	}
	if _, err := firstRepo.UpsertFeed(ctx, domainrss.FeedUpdate{Subscription: subscription, Entries: entries}); err != nil {
		_ = firstDatabase.Close()
		t.Fatalf("upsert media entries: %v", err)
	}
	if err := firstDatabase.Close(); err != nil {
		t.Fatalf("close first sqlite: %v", err)
	}

	secondDatabase, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer secondDatabase.Close()
	secondRepo := NewSQLiteRepository(secondDatabase.Bun)
	generic, err := secondRepo.GetEntry(ctx, "rss-entry-generic")
	if err != nil {
		t.Fatalf("get generic entry: %v", err)
	}
	if generic.DownloadTarget != generic.MediaURL || generic.DownloadTarget != "https://cdn.example.com/clip.mp4" {
		t.Fatalf("generic download target after reopen = %q, want media URL %q", generic.DownloadTarget, generic.MediaURL)
	}
	youtube, err := secondRepo.GetEntry(ctx, "rss-entry-youtube")
	if err != nil {
		t.Fatalf("get YouTube entry: %v", err)
	}
	if youtube.DownloadTarget != youtube.URL {
		t.Fatalf("YouTube download target after reopen = %q, want page URL %q", youtube.DownloadTarget, youtube.URL)
	}

	page, err := secondRepo.ListEntries(ctx, domainrss.EntryQuery{SubscriptionID: subscription.ID})
	if err != nil {
		t.Fatalf("list reopened media entries: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("reopened media entry count = %d, want 2", len(page.Items))
	}
	for _, item := range page.Items {
		if item.ID == generic.ID && item.DownloadTarget != generic.MediaURL {
			t.Fatalf("listed generic download target = %q, want %q", item.DownloadTarget, generic.MediaURL)
		}
	}
}

func TestSQLiteRepositorySyncOverviewSnapshotChangesAndScopeIsolation(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "sync-contract.db")
	now := time.Date(2026, 7, 13, 13, 0, 0, 0, time.UTC)
	entry := seedTestEntry(t, ctx, repo, now)
	scope := domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID}

	overview, err := repo.GetSyncOverview(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	assertSyncEpoch(t, overview.Epoch)
	if overview.WorkspaceID != domainrss.DefaultWorkspaceID || overview.SubjectID != domainrss.DefaultSubjectID ||
		overview.HighWater < 3 || overview.RetainedFrom != 0 {
		t.Fatalf("overview = %#v", overview)
	}

	first, err := repo.ListSyncSnapshot(ctx, domainrss.SyncSnapshotQuery{
		Scope: scope, Epoch: overview.Epoch, HighWater: overview.HighWater, Stage: "subscriptions", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Records) != 1 || first.Records[0].EntityType != "subscription" || !first.HasMore ||
		first.NextStage != "subscriptions" || first.NextID != entry.SubscriptionID {
		t.Fatalf("first snapshot page = %#v", first)
	}
	if strings.Contains(string(first.Records[0].Payload), "feedUrl") || strings.Contains(string(first.Records[0].Payload), "etag") ||
		strings.Contains(string(first.Records[0].Payload), "siteUrl") || strings.Contains(string(first.Records[0].Payload), "iconUrl") ||
		strings.Contains(string(first.Records[0].Payload), "https://") || !strings.Contains(string(first.Records[0].Payload), `"iconAvailable":true`) {
		t.Fatalf("subscription snapshot leaked private feed metadata: %s", first.Records[0].Payload)
	}
	second, err := repo.ListSyncSnapshot(ctx, domainrss.SyncSnapshotQuery{
		Scope: scope, Epoch: overview.Epoch, HighWater: overview.HighWater,
		Stage: first.NextStage, AfterID: first.NextID, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Records) != 1 || second.Records[0].EntityType != "entry" || second.HasMore ||
		second.Records[0].EntityID != entry.ID {
		t.Fatalf("second snapshot page = %#v", second)
	}
	entryPayload := string(second.Records[0].Payload)
	for _, forbidden := range []string{"contentHtml", "externalId", "contentHash", "media\":", `"url":`, "thumbnailUrl", "https://"} {
		if strings.Contains(entryPayload, forbidden) {
			t.Fatalf("entry snapshot leaked %q: %s", forbidden, entryPayload)
		}
	}

	starred := true
	state, err := repo.ApplyStateMutation(ctx, domainrss.StateMutation{
		Scope: scope, EntryID: entry.ID, Field: domainrss.EntryStateFieldStarred, Starred: &starred,
		ExpectedRevision: 0, DeviceID: "iphone-1", MutationID: "star-1",
		RequestHash: strings.Repeat("a", 64), ChangedAt: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	changes, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: overview.Epoch, After: overview.HighWater, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Changes) != 1 || changes.Changes[0].EntityType != "entry_state" ||
		changes.Changes[0].Revision != state.Revision || changes.HighWater <= overview.HighWater {
		t.Fatalf("incremental changes = %#v", changes)
	}
	if _, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: strings.Repeat("f", 32), After: changes.Cursor, Limit: 10,
	}); !errors.Is(err, domainrss.ErrSyncResetRequired) {
		t.Fatalf("stale epoch error = %v", err)
	}
	if _, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: overview.Epoch, After: changes.HighWater + 1, Limit: 10,
	}); !errors.Is(err, domainrss.ErrSyncResetRequired) {
		t.Fatalf("ahead cursor error = %v", err)
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_workspaces (id, catalog_id, owner_subject_id, created_at, updated_at)
VALUES ('rss-other', 'other', 'other-owner', '2026-07-13T14:00:00Z', '2026-07-13T14:00:00Z');
INSERT INTO rss_sync_state (workspace_id, epoch, rotated_at, retained_from)
VALUES ('rss-other', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', '2026-07-13T14:00:00Z', 0);
INSERT INTO rss_subscriptions (id, workspace_id, feed_url, title, enabled, created_at, updated_at, revision)
VALUES ('other-sub', 'rss-other', 'https://other.example/feed.xml', 'Other', 1, '2026-07-13T14:00:00Z', '2026-07-13T14:00:00Z', 1);
INSERT INTO rss_entries (id, subscription_id, external_id, title, content_hash, created_at, modified_at)
VALUES ('other-entry', 'other-sub', 'other-external', 'Other post', 'other-hash', '2026-07-13T14:00:00Z', '2026-07-13T14:00:00Z');
`); err != nil {
		t.Fatal(err)
	}
	listedSubscriptions, err := repo.ListSubscriptions(ctx)
	if err != nil || len(listedSubscriptions) != 1 || listedSubscriptions[0].ID != entry.SubscriptionID {
		t.Fatalf("workspace-isolated subscriptions=%#v err=%v", listedSubscriptions, err)
	}
	listedEntries, err := repo.ListEntries(ctx, domainrss.EntryQuery{Limit: 100})
	if err != nil || len(listedEntries.Items) != 1 || listedEntries.Items[0].ID != entry.ID {
		t.Fatalf("workspace-isolated entries=%#v err=%v", listedEntries, err)
	}
	if _, err := repo.GetEntry(ctx, "other-entry"); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("cross-workspace detail error = %v", err)
	}

	if _, err := database.SQL.ExecContext(ctx, `
UPDATE rss_sync_state SET retained_from = ? WHERE workspace_id = ?
`, changes.HighWater, domainrss.DefaultWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: overview.Epoch, After: changes.HighWater - 1, Limit: 10,
	}); !errors.Is(err, domainrss.ErrSyncResetRequired) {
		t.Fatalf("below-retention cursor error = %v", err)
	}
}

func TestPruneRSSSyncJournalBoundsChangesAndMutationReceiptTTL(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "sync-retention.db")
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	entry := seedTestEntry(t, ctx, repo, now)
	for _, statement := range []string{
		"DELETE FROM rss_tombstones",
		"DELETE FROM rss_changes",
		"DELETE FROM rss_client_mutations",
	} {
		if _, err := database.SQL.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE rss_sync_state SET retained_from = 0 WHERE workspace_id = ?
`, domainrss.DefaultWorkspaceID); err != nil {
		t.Fatal(err)
	}

	var firstSequence, highWater int64
	for index := 0; index < 6; index++ {
		_, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_changes (
  workspace_id, entity_type, entity_id, operation, revision, payload_json, changed_at
) VALUES (?, 'entry', ?, 'upsert', 1, '{}', ?)
`, domainrss.DefaultWorkspaceID, fmt.Sprintf("retained-entry-%d", index), now.Add(time.Duration(index)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		var sequence int64
		if err := database.SQL.QueryRowContext(ctx, "SELECT MAX(sequence) FROM rss_changes").Scan(&sequence); err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			firstSequence = sequence
		}
		highWater = sequence
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_tombstones (
  workspace_id, entity_type, entity_id, deleted_sequence, deleted_at
) VALUES (?, 'entry', 'expired-tombstone', ?, ?)
`, domainrss.DefaultWorkspaceID, firstSequence, now); err != nil {
		t.Fatal(err)
	}
	receipts := []struct {
		id        string
		createdAt time.Time
	}{
		{id: "expired-by-ttl", createdAt: now.Add(-45 * 24 * time.Hour)},
		{id: "expired-by-cap", createdAt: now.Add(-3 * time.Hour)},
		{id: "retained-b", createdAt: now.Add(-2 * time.Hour)},
		{id: "retained-c", createdAt: now.Add(-time.Hour)},
	}
	for _, receipt := range receipts {
		if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_client_mutations (
  device_id, mutation_id, entry_id, request_hash, result_json, created_at
) VALUES ('iphone-retention', ?, ?, 'hash', '{}', ?)
`, receipt.id, entry.ID, receipt.createdAt); err != nil {
			t.Fatal(err)
		}
	}

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pruneRSSSyncJournalTx(
		ctx, tx, domainrss.DefaultWorkspaceID, 3, 2, now.Add(-30*24*time.Hour),
	); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var changeCount int
	var minSequence, retainedFrom, storedHighWater int64
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*), MIN(sequence), MAX(sequence) FROM rss_changes WHERE workspace_id = ?
`, domainrss.DefaultWorkspaceID).Scan(&changeCount, &minSequence, &storedHighWater); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT retained_from FROM rss_sync_state WHERE workspace_id = ?
`, domainrss.DefaultWorkspaceID).Scan(&retainedFrom); err != nil {
		t.Fatal(err)
	}
	if changeCount != 3 || retainedFrom != minSequence-1 || storedHighWater != highWater {
		t.Fatalf(
			"retained journal count=%d retainedFrom=%d min=%d highWater=%d wantHighWater=%d",
			changeCount, retainedFrom, minSequence, storedHighWater, highWater,
		)
	}
	var tombstones int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_tombstones").Scan(&tombstones); err != nil {
		t.Fatal(err)
	}
	if tombstones != 0 {
		t.Fatalf("expired tombstones retained = %d", tombstones)
	}
	var mutationCount, removedReceipts int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM rss_client_mutations").Scan(&mutationCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_client_mutations
WHERE mutation_id IN ('expired-by-ttl', 'expired-by-cap')
`).Scan(&removedReceipts); err != nil {
		t.Fatal(err)
	}
	if mutationCount != 2 || removedReceipts != 0 {
		t.Fatalf("mutation receipt retention count=%d removedStillPresent=%d", mutationCount, removedReceipts)
	}

	overview, err := repo.GetSyncOverview(ctx, domainrss.SyncScope{
		WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if overview.HighWater != highWater || overview.RetainedFrom != retainedFrom {
		t.Fatalf("retained overview = %#v", overview)
	}
	if _, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID},
		Epoch: overview.Epoch, After: retainedFrom - 1, Limit: 10,
	}); !errors.Is(err, domainrss.ErrSyncResetRequired) {
		t.Fatalf("pre-retention cursor error = %v", err)
	}
	page, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID},
		Epoch: overview.Epoch, After: retainedFrom, Limit: 10,
	})
	if err != nil || len(page.Changes) != 3 || page.HighWater != highWater {
		t.Fatalf("post-retention page = %#v, error=%v", page, err)
	}
}

func TestSQLiteRepositoryStateV2UsesPerFieldClocksAndPayloadBoundIdempotency(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "state-v2.db")
	now := time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC)
	entry := seedTestEntry(t, ctx, repo, now)
	scope := domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID}

	starred := true
	starMutation := domainrss.StateMutation{
		Scope: scope, EntryID: entry.ID, Field: domainrss.EntryStateFieldStarred, Starred: &starred,
		ExpectedRevision: 0, DeviceID: "iphone-1", MutationID: "mutation-star",
		RequestHash: strings.Repeat("1", 64), ChangedAt: now.Add(time.Minute),
	}
	starState, err := repo.ApplyStateMutation(ctx, starMutation)
	if err != nil {
		t.Fatal(err)
	}
	if !starState.Starred || starState.FieldRevisions.Starred != 1 || starState.FieldRevisions.Read != 0 || starState.Revision != 1 {
		t.Fatalf("star state = %#v", starState)
	}
	replayed, err := repo.ApplyStateMutation(ctx, starMutation)
	if err != nil || replayed.Revision != starState.Revision || replayed.MutationID != starState.MutationID {
		t.Fatalf("same-payload replay=%#v err=%v", replayed, err)
	}
	different := starMutation
	different.RequestHash = strings.Repeat("2", 64)
	if _, err := repo.ApplyStateMutation(ctx, different); !errors.Is(err, domainrss.ErrIdempotencyConflict) {
		t.Fatalf("different payload reused idempotency key: %v", err)
	}

	article := &domainrss.ArticleProgress{Fraction: 0.45, Anchor: "section-2", ContentRevision: 1}
	articleState, err := repo.ApplyStateMutation(ctx, domainrss.StateMutation{
		Scope: scope, EntryID: entry.ID, Field: domainrss.EntryStateFieldArticleProgress,
		ArticleProgress: article, ExpectedRevision: 0, DeviceID: "iphone-1", MutationID: "mutation-article",
		RequestHash: strings.Repeat("3", 64), ChangedAt: now.Add(2 * time.Minute),
	})
	if err != nil || articleState.ArticleProgress == nil || articleState.ArticleProgress.Fraction != 0.45 ||
		articleState.FieldRevisions.ArticleProgress != 1 || articleState.FieldRevisions.Starred != 1 || articleState.Revision != 2 {
		t.Fatalf("article state=%#v err=%v", articleState, err)
	}

	position, duration := 90.25, 90.25
	videoState, err := repo.ApplyStateMutation(ctx, domainrss.StateMutation{
		Scope: scope, EntryID: entry.ID, Field: domainrss.EntryStateFieldVideoProgressSeconds,
		VideoProgressSeconds: &position, VideoDurationSeconds: &duration,
		ExpectedRevision: 0, DeviceID: "iphone-1", MutationID: "mutation-video",
		RequestHash: strings.Repeat("4", 64), ChangedAt: now.Add(3 * time.Minute),
	})
	if err != nil || videoState.VideoProgressSeconds == nil || *videoState.VideoProgressSeconds != position ||
		videoState.VideoDurationSeconds == nil || *videoState.VideoDurationSeconds != duration || !videoState.VideoCompleted ||
		videoState.FieldRevisions.VideoProgressSeconds != 1 || videoState.Revision != 3 {
		t.Fatalf("video state=%#v err=%v", videoState, err)
	}

	read := true
	readState, err := repo.ApplyStateMutation(ctx, domainrss.StateMutation{
		Scope: scope, EntryID: entry.ID, Field: domainrss.EntryStateFieldRead, Read: &read,
		ExpectedRevision: 0, DeviceID: "iphone-1", MutationID: "mutation-read",
		RequestHash: strings.Repeat("5", 64), ChangedAt: now.Add(4 * time.Minute),
	})
	if err != nil || !readState.Read || readState.FieldRevisions.Read != 1 || readState.Revision != 4 {
		t.Fatalf("read state=%#v err=%v", readState, err)
	}

	if _, err := repo.ApplyStateMutation(ctx, domainrss.StateMutation{
		Scope: scope, EntryID: entry.ID, Field: domainrss.EntryStateFieldStarred, Starred: &starred,
		ExpectedRevision: 0, DeviceID: "iphone-1", MutationID: "mutation-star-stale",
		RequestHash: strings.Repeat("6", 64), ChangedAt: now.Add(5 * time.Minute),
	}); !errors.Is(err, domainrss.ErrRevisionConflict) {
		t.Fatalf("stale star clock error = %v", err)
	} else {
		var conflict *domainrss.StateConflictError
		if !errors.As(err, &conflict) || conflict.State.FieldRevisions.Starred != 1 || conflict.State.Revision != 4 {
			t.Fatalf("state conflict payload = %#v", conflict)
		}
	}

	stored, err := repo.GetEntry(ctx, entry.ID)
	if err != nil || stored.ArticleProgress == nil || stored.VideoProgressSeconds == nil ||
		stored.FieldRevisions.Read != 1 || stored.FieldRevisions.Starred != 1 ||
		stored.FieldRevisions.ArticleProgress != 1 || stored.FieldRevisions.VideoProgressSeconds != 1 {
		t.Fatalf("persisted v2 state=%#v err=%v", stored, err)
	}
	var mutationCount int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_client_mutations`).Scan(&mutationCount); err != nil {
		t.Fatal(err)
	}
	if mutationCount != 4 {
		t.Fatalf("client mutation rows=%d, want 4", mutationCount)
	}
}

func openTestRSSRepository(t *testing.T, name string) (context.Context, *persistence.Database, *SQLiteRepository) {
	t.Helper()
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), name),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return ctx, database, NewSQLiteRepository(database.Bun)
}

func testSubscription(id, feedURL string, now time.Time) domainrss.Subscription {
	return domainrss.Subscription{
		ID: id, WorkspaceID: domainrss.DefaultWorkspaceID, FeedURL: feedURL,
		SiteURL: "https://example.com/", Title: "Example feed", Description: "Feed description",
		IconURL: "https://example.com/icon.png", ViewType: domainrss.ViewTypeAuto, Enabled: true,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
}

func seedTestEntry(t *testing.T, ctx context.Context, repo *SQLiteRepository, now time.Time) domainrss.Entry {
	t.Helper()
	subscription := testSubscription("subscription-1", "https://example.com/feed.xml", now)
	if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
		t.Fatalf("create seed subscription: %v", err)
	}
	subscription.Revision = 2
	subscription.UpdatedAt = now.Add(time.Minute)
	entry := domainrss.Entry{
		ID: "rss-entry-test-1", SubscriptionID: subscription.ID, ExternalID: "entry-1",
		URL: "https://example.com/posts/1", Title: "Entry one", Author: "Author",
		Summary: "Summary", ContentHTML: "<p>Body</p>", Kind: domainrss.EntryKindArticle,
		ImageURLs: []string{}, Media: []domainrss.Media{}, Revision: 1,
		ContentHash: "content-hash-1", CreatedAt: now, ModifiedAt: now,
	}
	result, err := repo.UpsertFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription,
		Entries:      []domainrss.Entry{entry},
	})
	if err != nil {
		t.Fatalf("upsert seed feed: %v", err)
	}
	if result.Created != 1 || result.Updated != 0 {
		t.Fatalf("seed upsert result = %#v, want one created entry", result)
	}
	return entry
}

func assertSyncEpoch(t *testing.T, epoch string) {
	t.Helper()
	if len(epoch) != 32 || strings.Trim(epoch, "0123456789abcdef") != "" {
		t.Fatalf("sync epoch = %q, want 32 lowercase hex characters", epoch)
	}
}

func queryUnreadCount(t *testing.T, ctx context.Context, database *persistence.Database, subscriptionID string) int {
	t.Helper()
	var count int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_entries WHERE subscription_id = ? AND read_at IS NULL
`, subscriptionID).Scan(&count); err != nil {
		t.Fatalf("query unread count: %v", err)
	}
	return count
}
