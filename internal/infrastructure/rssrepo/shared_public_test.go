package rssrepo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

func TestSharedPublicSubscriptionMutationIsIdempotentRevisionedAndTombstoned(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "shared-public-subscription.db")
	now := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	subscriptionID := "7f7a68d0-49ab-4e90-88bb-28fd77551d4e"
	sortOrder := 7
	enabled := true
	add := domainrss.SubscriptionMutation{
		DeviceID: "device-a", MutationID: "mutation-add", RequestHash: "hash-add",
		Operation: domainrss.SubscriptionMutationAdd, SubscriptionID: subscriptionID,
		Title: "Public News", ViewType: domainrss.ViewTypeArticle,
		SortOrder: &sortOrder, Enabled: &enabled,
		SourceAccess:  domainrss.SubscriptionSourceSharedPublic,
		PublicFeedURL: "https://feeds.example.test/public.xml", ChangedAt: now,
	}

	created, err := repo.ApplySubscriptionMutation(ctx, add)
	if err != nil {
		t.Fatalf("add shared-public subscription: %v", err)
	}
	if created.Revision != 1 || created.ChangeCursor != 1 || created.Subscription == nil {
		t.Fatalf("unexpected add result: %#v", created)
	}
	if created.Subscription.ID != subscriptionID || created.Subscription.SourceAccess != domainrss.SubscriptionSourceSharedPublic {
		t.Fatalf("unexpected subscription projection: %#v", created.Subscription)
	}
	// Journal and mutation receipts must not become another public-feed URL
	// store. Scope-aware HTTP code may enrich the DTO from rss_subscriptions.
	if created.Subscription.PublicFeedURL != "" {
		t.Fatalf("mutation projection leaked publicFeedURL: %#v", created.Subscription)
	}

	replayed, err := repo.ApplySubscriptionMutation(ctx, add)
	if err != nil {
		t.Fatalf("replay add mutation: %v", err)
	}
	if replayed.Revision != created.Revision || replayed.ChangeCursor != created.ChangeCursor || replayed.Subscription == nil || replayed.Subscription.ID != subscriptionID {
		t.Fatalf("replayed result %#v differs from original %#v", replayed, created)
	}
	conflictingRetry := add
	conflictingRetry.RequestHash = "different-body-hash"
	if _, err := repo.ApplySubscriptionMutation(ctx, conflictingRetry); !errors.Is(err, domainrss.ErrIdempotencyConflict) {
		t.Fatalf("same mutation id with a different body error = %v, want ErrIdempotencyConflict", err)
	}

	staleRevision := int64(0)
	stale := domainrss.SubscriptionMutation{
		DeviceID: "device-a", MutationID: "mutation-stale", RequestHash: "hash-stale",
		Operation: domainrss.SubscriptionMutationUpdate, SubscriptionID: subscriptionID,
		ExpectedRevision: &staleRevision, FieldMask: []string{"title"},
		Title: "Stale title", ChangedAt: now.Add(time.Minute),
	}
	if _, err := repo.ApplySubscriptionMutation(ctx, stale); !errors.Is(err, domainrss.ErrRevisionConflict) {
		t.Fatalf("stale subscription update error = %v, want ErrRevisionConflict", err)
	}

	expectedRevision := int64(1)
	update := domainrss.SubscriptionMutation{
		DeviceID: "device-b", MutationID: "mutation-update", RequestHash: "hash-update",
		Operation: domainrss.SubscriptionMutationUpdate, SubscriptionID: subscriptionID,
		ExpectedRevision: &expectedRevision, FieldMask: []string{"title"},
		Title: "Renamed Public News", ChangedAt: now.Add(2 * time.Minute),
	}
	updated, err := repo.ApplySubscriptionMutation(ctx, update)
	if err != nil {
		t.Fatalf("update shared-public subscription: %v", err)
	}
	if updated.Revision != 2 || updated.Subscription == nil || updated.Subscription.Title != update.Title {
		t.Fatalf("unexpected update result: %#v", updated)
	}
	stored, err := repo.GetSubscription(ctx, subscriptionID)
	if err != nil {
		t.Fatalf("get updated subscription: %v", err)
	}
	if stored.Title != update.Title || stored.CategoryID != "" || stored.SortOrder != sortOrder || !stored.Enabled || stored.Revision != 2 {
		t.Fatalf("field-mask update cleared unrelated fields: %#v", stored)
	}
	if stored.FeedURL == stored.PublicFeedURL || stored.FeedURL == "" || stored.PublicFeedURL != add.PublicFeedURL {
		t.Fatalf("private and shared descriptors were not stored separately: %#v", stored)
	}
	// Subscription mutations use entity-level CAS. Even an unrelated field mask
	// cannot merge from a stale revision, and a future revision is also rejected.
	mergeExpectedRevision := int64(1)
	disabled := false
	if _, err := repo.ApplySubscriptionMutation(ctx, domainrss.SubscriptionMutation{
		DeviceID: "device-c", MutationID: "mutation-field-merge", RequestHash: "hash-field-merge",
		Operation: domainrss.SubscriptionMutationUpdate, SubscriptionID: subscriptionID,
		ExpectedRevision: &mergeExpectedRevision, FieldMask: []string{"enabled"}, Enabled: &disabled,
		ChangedAt: now.Add(3 * time.Minute),
	}); !errors.Is(err, domainrss.ErrRevisionConflict) {
		t.Fatalf("stale unrelated-field update error=%v, want ErrRevisionConflict", err)
	}
	futureRevision := int64(99)
	if _, err := repo.ApplySubscriptionMutation(ctx, domainrss.SubscriptionMutation{
		DeviceID: "device-c", MutationID: "mutation-future", RequestHash: "hash-future",
		Operation: domainrss.SubscriptionMutationUpdate, SubscriptionID: subscriptionID,
		ExpectedRevision: &futureRevision, FieldMask: []string{"enabled"}, Enabled: &disabled,
		ChangedAt: now.Add(3 * time.Minute),
	}); !errors.Is(err, domainrss.ErrRevisionConflict) {
		t.Fatalf("future subscription update error=%v, want ErrRevisionConflict", err)
	}
	currentRevision := int64(2)
	merged, err := repo.ApplySubscriptionMutation(ctx, domainrss.SubscriptionMutation{
		DeviceID: "device-c", MutationID: "mutation-current", RequestHash: "hash-current",
		Operation: domainrss.SubscriptionMutationUpdate, SubscriptionID: subscriptionID,
		ExpectedRevision: &currentRevision, FieldMask: []string{"enabled"}, Enabled: &disabled,
		ChangedAt: now.Add(3 * time.Minute),
	})
	if err != nil || merged.Revision != 3 || merged.Subscription == nil || merged.Subscription.Enabled || merged.Subscription.Title != update.Title {
		t.Fatalf("current revision update result=%#v err=%v", merged, err)
	}

	deleteRevision := int64(3)
	deleteMutation := domainrss.SubscriptionMutation{
		DeviceID: "device-a", MutationID: "mutation-delete", RequestHash: "hash-delete",
		Operation: domainrss.SubscriptionMutationDelete, SubscriptionID: subscriptionID,
		ExpectedRevision: &deleteRevision, ChangedAt: now.Add(4 * time.Minute),
	}
	deleted, err := repo.ApplySubscriptionMutation(ctx, deleteMutation)
	if err != nil {
		t.Fatalf("delete shared-public subscription: %v", err)
	}
	if deleted.DeletedID != subscriptionID || deleted.Revision != 4 || deleted.ChangeCursor != 4 {
		t.Fatalf("unexpected delete result: %#v", deleted)
	}
	if _, err := repo.ApplySubscriptionMutation(ctx, deleteMutation); err != nil {
		t.Fatalf("replay delete mutation after physical delete: %v", err)
	}
	if _, err := repo.GetSubscription(ctx, subscriptionID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("deleted subscription lookup error = %v, want ErrNotFound", err)
	}

	resurrect := add
	resurrect.MutationID = "mutation-resurrect"
	resurrect.RequestHash = "hash-resurrect"
	resurrect.ChangedAt = now.Add(5 * time.Minute)
	if _, err := repo.ApplySubscriptionMutation(ctx, resurrect); !errors.Is(err, domainrss.ErrRevisionConflict) {
		t.Fatalf("tombstoned subscription resurrection error = %v, want ErrRevisionConflict", err)
	}

	var changeCount, tombstoneCount, receiptCount int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_changes`).Scan(&changeCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_tombstones
WHERE workspace_id = ? AND entity_type = 'subscription' AND entity_id = ?
`, domainrss.DefaultWorkspaceID, subscriptionID).Scan(&tombstoneCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_public_mutations`).Scan(&receiptCount); err != nil {
		t.Fatal(err)
	}
	if changeCount != 4 || tombstoneCount != 1 || receiptCount != 4 {
		t.Fatalf("changes=%d tombstones=%d receipts=%d, want 4/1/4", changeCount, tombstoneCount, receiptCount)
	}
}

func TestSharedPublicObservationDeduplicatesOriginAndRejectsOlderContent(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "shared-public-observation.db")
	now := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	subscriptionID := "fdc30b89-77f5-49a4-a448-a70e8f7bb023"
	seedSharedPublicSubscription(t, ctx, repo, subscriptionID, now)
	originKey := "v1:95f740796258c0ae583b9895f3db0a44667f5510e34e4b26a3c2481866df4d2e"

	first := sharedPublicObservation(
		"device-a", "observation-1", "hash-observation-1", subscriptionID,
		originKey, "entry-proposal-a", "First canonical title", "content-v1", now.Add(time.Minute),
	)
	created, err := repo.ApplyFeedObservation(ctx, first)
	if err != nil {
		t.Fatalf("apply first observation: %v", err)
	}
	if created.Created != 1 || created.Updated != 0 || len(created.Mappings) != 1 {
		t.Fatalf("unexpected first observation result: %#v", created)
	}
	canonicalEntryID := created.Mappings[0].EntryID
	if canonicalEntryID == "" || created.Mappings[0].OriginKey != originKey || created.Mappings[0].ContentRevision != 1 {
		t.Fatalf("unexpected origin mapping: %#v", created.Mappings)
	}

	replayed, err := repo.ApplyFeedObservation(ctx, first)
	if err != nil {
		t.Fatalf("replay observation: %v", err)
	}
	if replayed.Created != created.Created || replayed.ChangeCursor != created.ChangeCursor || len(replayed.Mappings) != 1 || replayed.Mappings[0].EntryID != canonicalEntryID {
		t.Fatalf("replayed observation %#v differs from original %#v", replayed, created)
	}
	conflict := first
	conflict.RequestHash = "different-observation-body"
	if _, err := repo.ApplyFeedObservation(ctx, conflict); !errors.Is(err, domainrss.ErrIdempotencyConflict) {
		t.Fatalf("observation mutation hash conflict error = %v, want ErrIdempotencyConflict", err)
	}

	duplicate := sharedPublicObservation(
		"device-b", "observation-2", "hash-observation-2", subscriptionID,
		originKey, "different-client-proposal", "First canonical title", "content-v1", now.Add(2*time.Minute),
	)
	duplicate.CanonicalEntries[0].ID = canonicalEntryID
	duplicateResult, err := repo.ApplyFeedObservation(ctx, duplicate)
	if err != nil {
		t.Fatalf("apply same-content observation from another device: %v", err)
	}
	if duplicateResult.Created != 0 || duplicateResult.Updated != 0 || len(duplicateResult.Mappings) != 1 || duplicateResult.Mappings[0].EntryID != canonicalEntryID {
		t.Fatalf("same origin did not deduplicate: %#v", duplicateResult)
	}

	newest := sharedPublicObservation(
		"device-a", "observation-3", "hash-observation-3", subscriptionID,
		originKey, canonicalEntryID, "Newest canonical title", "content-v2", now.Add(4*time.Minute),
	)
	// A fallback-derived external ID may change when canonical content changes.
	// The versioned origin mapping, rather than that parser-local external ID,
	// must keep the canonical entry identity stable.
	newest.CanonicalEntries[0].ExternalID = "fallback-external-v2"
	newestResult, err := repo.ApplyFeedObservation(ctx, newest)
	if err != nil {
		t.Fatalf("apply newer observation: %v", err)
	}
	if newestResult.Created != 0 || newestResult.Updated != 1 || newestResult.Mappings[0].EntryID != canonicalEntryID || newestResult.Mappings[0].ContentRevision != 2 {
		t.Fatalf("unexpected newer observation result: %#v", newestResult)
	}

	older := sharedPublicObservation(
		"device-b", "observation-4", "hash-observation-4", subscriptionID,
		originKey, canonicalEntryID, "Older payload must not win", "content-stale", now.Add(3*time.Minute),
	)
	olderResult, err := repo.ApplyFeedObservation(ctx, older)
	if err != nil {
		t.Fatalf("apply out-of-order observation: %v", err)
	}
	if olderResult.Created != 0 || olderResult.Updated != 0 || olderResult.Mappings[0].EntryID != canonicalEntryID || olderResult.Mappings[0].ContentRevision != 2 {
		t.Fatalf("out-of-order observation was not treated as a no-op: %#v", olderResult)
	}
	stored, err := repo.GetEntry(ctx, canonicalEntryID)
	if err != nil {
		t.Fatalf("get canonical entry: %v", err)
	}
	if stored.Title != newest.CanonicalEntries[0].Title || stored.ContentHash != newest.CanonicalEntries[0].ContentHash || stored.Revision != 2 {
		t.Fatalf("older observation rolled back canonical entry: %#v", stored)
	}

	var entryCount, originCount, entryChangeCount int
	var mappedEntryID string
	var lastObservedAt time.Time
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_entries WHERE subscription_id = ?`, subscriptionID).Scan(&entryCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_entry_origins WHERE subscription_id = ? AND origin_key = ?
`, subscriptionID, originKey).Scan(&originCount); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT entry_id, last_observed_at
FROM rss_entry_origins WHERE subscription_id = ? AND origin_key = ?
`, subscriptionID, originKey).Scan(&mappedEntryID, &lastObservedAt); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_changes WHERE entity_type = 'entry'`).Scan(&entryChangeCount); err != nil {
		t.Fatal(err)
	}
	if entryCount != 1 || originCount != 1 || mappedEntryID != canonicalEntryID || !lastObservedAt.Equal(newest.FetchedAt) || entryChangeCount != 2 {
		t.Fatalf("entries=%d origins=%d mapped=%q observed=%s changes=%d", entryCount, originCount, mappedEntryID, lastObservedAt, entryChangeCount)
	}
}

func TestSharedPublicFetchLeaseIsExclusiveRenewableAndExpires(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "shared-public-lease.db")
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	subscriptionID := "248c47e6-87b3-4609-88cf-da31f688dc53"
	seedSharedPublicSubscription(t, ctx, repo, subscriptionID, now)

	first, err := repo.AcquireFetchLease(ctx, domainrss.FetchLeaseRequest{
		DeviceID: "device-a", SubscriptionID: subscriptionID, LeaseID: "lease-a",
		RequestedTTL: time.Second, RequestedAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("acquire first lease: %v", err)
	}
	minimumExpiry := now.Add(time.Minute + 30*time.Second)
	if !first.Granted || first.LeaseID != "lease-a" || !first.ExpiresAt.Equal(minimumExpiry) {
		t.Fatalf("minimum TTL was not enforced: %#v", first)
	}

	sameOwner, err := repo.AcquireFetchLease(ctx, domainrss.FetchLeaseRequest{
		DeviceID: "device-a", SubscriptionID: subscriptionID, LeaseID: "replacement-must-not-win",
		RequestedTTL: 5 * time.Minute, RequestedAt: now.Add(time.Minute + time.Second),
	})
	if err != nil {
		t.Fatalf("retry active owner lease: %v", err)
	}
	if !sameOwner.Granted || sameOwner.LeaseID != first.LeaseID || !sameOwner.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("same-owner retry changed active lease: first=%#v retry=%#v", first, sameOwner)
	}

	contender, err := repo.AcquireFetchLease(ctx, domainrss.FetchLeaseRequest{
		DeviceID: "device-b", SubscriptionID: subscriptionID, LeaseID: "lease-b",
		RequestedTTL: time.Minute, RequestedAt: now.Add(time.Minute + 10*time.Second),
	})
	if err != nil {
		t.Fatalf("contended lease: %v", err)
	}
	if contender.Granted || contender.LeaseID != "" || contender.RetryAfterSeconds != 20 || !contender.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("active lease was not exclusive: %#v", contender)
	}

	takeoverAt := first.ExpiresAt.Add(time.Second)
	afterExpiry, err := repo.AcquireFetchLease(ctx, domainrss.FetchLeaseRequest{
		DeviceID: "device-b", SubscriptionID: subscriptionID, LeaseID: "lease-b",
		RequestedTTL: 30 * time.Minute, RequestedAt: takeoverAt,
	})
	if err != nil {
		t.Fatalf("acquire expired lease: %v", err)
	}
	if !afterExpiry.Granted || afterExpiry.LeaseID != "lease-b" || !afterExpiry.ExpiresAt.Equal(takeoverAt.Add(10*time.Minute)) {
		t.Fatalf("expiry takeover or maximum TTL failed: %#v", afterExpiry)
	}

	var leaseCount int
	var owner, leaseID string
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*), MAX(device_id), MAX(lease_id) FROM rss_fetch_leases WHERE subscription_id = ?`, subscriptionID).
		Scan(&leaseCount, &owner, &leaseID); err != nil {
		t.Fatal(err)
	}
	if leaseCount != 1 || owner != "device-b" || leaseID != "lease-b" {
		t.Fatalf("lease rows=%d owner=%q lease=%q", leaseCount, owner, leaseID)
	}
}

func TestSharedPublicFetchLeaseConcurrentContendersHaveSingleWinner(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "shared-public-lease-concurrent.db")
	now := time.Date(2026, 7, 21, 10, 30, 0, 0, time.UTC)
	subscriptionID := "d0934639-e680-4181-83e8-997a3062e44e"
	seedSharedPublicSubscription(t, ctx, repo, subscriptionID, now)
	// Separate repository instances ensure the database transaction provides the
	// exclusion; an instance-local mutex must not make this test pass by accident.
	contenders := [2]*SQLiteRepository{repo, NewSQLiteRepository(database.Bun)}

	start := make(chan struct{})
	type outcome struct {
		result domainrss.FetchLeaseResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for index := range 2 {
		contender := contenders[index]
		deviceID := "device-a"
		leaseID := "lease-a"
		if index == 1 {
			deviceID = "device-b"
			leaseID = "lease-b"
		}
		go func() {
			ready.Done()
			<-start
			result, err := contender.AcquireFetchLease(ctx, domainrss.FetchLeaseRequest{
				DeviceID: deviceID, SubscriptionID: subscriptionID, LeaseID: leaseID,
				RequestedTTL: time.Minute, RequestedAt: now.Add(time.Minute),
			})
			outcomes <- outcome{result: result, err: err}
		}()
	}
	ready.Wait()
	close(start)

	first, second := <-outcomes, <-outcomes
	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent lease errors: first=%v second=%v", first.err, second.err)
	}
	granted := 0
	denied := 0
	for _, item := range []outcome{first, second} {
		if item.result.Granted {
			granted++
			if item.result.LeaseID != "lease-a" && item.result.LeaseID != "lease-b" {
				t.Fatalf("winner returned unknown lease: %#v", item.result)
			}
		} else {
			denied++
			if item.result.LeaseID != "" || item.result.RetryAfterSeconds <= 0 {
				t.Fatalf("loser exposed lease token or omitted retry delay: %#v", item.result)
			}
		}
	}
	if granted != 1 || denied != 1 {
		t.Fatalf("concurrent lease outcomes: first=%#v second=%#v", first.result, second.result)
	}
}

func seedSharedPublicSubscription(
	t *testing.T,
	ctx context.Context,
	repo *SQLiteRepository,
	subscriptionID string,
	now time.Time,
) {
	t.Helper()
	_, err := repo.ApplySubscriptionMutation(ctx, domainrss.SubscriptionMutation{
		DeviceID: "seed-device", MutationID: "seed-" + subscriptionID, RequestHash: "seed-hash-" + subscriptionID,
		Operation: domainrss.SubscriptionMutationAdd, SubscriptionID: subscriptionID,
		Title: "Shared feed", ViewType: domainrss.ViewTypeArticle,
		SourceAccess:  domainrss.SubscriptionSourceSharedPublic,
		PublicFeedURL: "https://feeds.example.test/" + subscriptionID + ".xml", ChangedAt: now,
	})
	if err != nil {
		t.Fatalf("seed shared-public subscription: %v", err)
	}
}

func sharedPublicObservation(
	deviceID, mutationID, requestHash, subscriptionID, originKey, entryID, title, contentHash string,
	fetchedAt time.Time,
) domainrss.FeedObservation {
	publishedAt := fetchedAt.Add(-time.Hour)
	return domainrss.FeedObservation{
		DeviceID: deviceID, MutationID: mutationID, RequestHash: requestHash,
		SubscriptionID: subscriptionID, UpstreamETag: `"etag"`,
		LastModified: fetchedAt.Format(time.RFC1123), FetchedAt: fetchedAt, ContentHash: contentHash,
		CanonicalEntries: []domainrss.Entry{{
			ID: entryID, SubscriptionID: subscriptionID, ExternalID: "feed-external-v1",
			OriginKey: originKey, ObservedAt: fetchedAt,
			URL: "https://feeds.example.test/posts/one", Title: title, Summary: title,
			ContentHTML: "<p>" + title + "</p>", Kind: domainrss.EntryKindArticle,
			PublishedAt: &publishedAt, SourceUpdatedAt: &fetchedAt,
			ContentHash: contentHash, CreatedAt: fetchedAt, ModifiedAt: fetchedAt,
		}},
	}
}
