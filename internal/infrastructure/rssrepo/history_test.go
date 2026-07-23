package rssrepo

import (
	"errors"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

func TestSQLiteRepositoryPersistsPrivateSubscriptionHistoryState(t *testing.T) {
	ctx, database, repo := openTestRSSRepository(t, "subscription-history.db")
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	subscription := testSubscription("subscription-history", "https://example.com/feed.json", now)
	if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetSubscriptionHistory(ctx, subscription.ID); !errors.Is(err, domainrss.ErrNotFound) {
		t.Fatalf("missing state error = %v", err)
	}

	attemptedAt := now.Add(time.Minute)
	succeededAt := now.Add(2 * time.Minute)
	state := domainrss.SubscriptionHistoryState{
		SubscriptionID: subscription.ID,
		CursorURL:      "https://example.com/archive?page=2&token=private",
		Capability:     domainrss.HistoryCapabilityAvailable,
		NoProgress:     2,
		LastAttemptAt:  &attemptedAt,
		LastSuccessAt:  &succeededAt,
		LastError:      "temporary failure",
		UpdatedAt:      succeededAt,
	}
	if err := repo.PutSubscriptionHistory(ctx, state); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetSubscriptionHistory(ctx, subscription.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CursorURL != state.CursorURL || stored.Capability != domainrss.HistoryCapabilityAvailable ||
		stored.Exhausted || stored.NoProgress != 2 || stored.LastAttemptAt == nil ||
		!stored.LastAttemptAt.Equal(attemptedAt) || stored.LastSuccessAt == nil ||
		!stored.LastSuccessAt.Equal(succeededAt) || stored.LastError != "temporary failure" {
		t.Fatalf("stored history state = %#v", stored)
	}

	state.CursorURL = ""
	state.Exhausted = true
	state.NoProgress = 3
	state.LastError = ""
	state.UpdatedAt = now.Add(3 * time.Minute)
	if err := repo.PutSubscriptionHistory(ctx, state); err != nil {
		t.Fatal(err)
	}
	stored, err = repo.GetSubscriptionHistory(ctx, subscription.ID)
	if err != nil || !stored.Exhausted || stored.CursorURL != "" || stored.NoProgress != 3 {
		t.Fatalf("updated history state = %#v, err=%v", stored, err)
	}

	invalid := state
	invalid.Capability = "invalid"
	if err := repo.PutSubscriptionHistory(ctx, invalid); !errors.Is(err, domainrss.ErrInvalidRequest) {
		t.Fatalf("invalid capability error = %v", err)
	}
	if err := repo.DeleteSubscription(ctx, subscription.ID, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_subscription_history`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("history rows after subscription delete = %d", rows)
	}
}

func TestSQLiteRepositoryHistoricalUpsertKeepsNewestCrossArchiveDocument(t *testing.T) {
	ctx, _, repo := openTestRSSRepository(t, "historical-cross-archive.db")
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	subscription := testSubscription("subscription-archive", "https://example.com/feed.json", now)
	if _, err := repo.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}

	older := now.Add(-2 * time.Hour)
	newest := now.Add(-time.Hour)
	newer := now.Add(time.Hour)
	readAt := now.Add(-30 * time.Minute)
	archiveEntry := func(title, hash string, updated *time.Time) domainrss.Entry {
		return domainrss.Entry{
			ID: "rss-entry-archive-document", SubscriptionID: subscription.ID,
			ExternalID: "shared-across-archive-pages", URL: "https://example.com/posts/archive",
			Title: title, ContentHTML: "<p>" + title + "</p>", Kind: domainrss.EntryKindArticle,
			SourceUpdatedAt: updated, ReadAt: &readAt, ReadStateUpdatedAt: &readAt,
			FieldRevisions: domainrss.StateFieldRevisions{Read: 1}, StateRevision: 1,
			Revision: 1, ContentHash: hash, CreatedAt: now, ModifiedAt: now,
		}
	}
	videoEntry := domainrss.Entry{
		ID: "rss-entry-archive-video", SubscriptionID: subscription.ID,
		ExternalID: "archive-video", URL: "https://example.com/videos/1",
		Title: "Archived video", Kind: domainrss.EntryKindVideo, SourceUpdatedAt: &older,
		ReadAt: &readAt, ReadStateUpdatedAt: &readAt,
		FieldRevisions: domainrss.StateFieldRevisions{Read: 1}, StateRevision: 1,
		Revision: 1, ContentHash: "video-v1", CreatedAt: now, ModifiedAt: now,
	}
	undatedExisting := archiveEntry("Undated existing archive document", "undated-existing", nil)
	undatedExisting.ID = "rss-entry-undated-existing"
	undatedExisting.ExternalID = "undated-across-archive-pages"
	undatedExisting.URL = "https://example.com/posts/undated"
	upsertArchive := func(entries ...domainrss.Entry) domainrss.HistoricalUpsertResult {
		t.Helper()
		current, err := repo.GetSubscription(ctx, subscription.ID)
		if err != nil {
			t.Fatal(err)
		}
		current.Revision++
		current.UpdatedAt = now.Add(time.Duration(current.Revision) * time.Minute)
		result, err := repo.UpsertHistoricalFeed(ctx, domainrss.FeedUpdate{
			Subscription: current,
			Entries:      entries,
		}, domainrss.EntryKindArticle)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	result := upsertArchive(
		archiveEntry("Newest archive document", "archive-newest", &newest),
		videoEntry,
		undatedExisting,
	)
	if result.Total.Created != 3 || result.Total.Updated != 0 ||
		result.Visible.Created != 2 || result.Visible.Updated != 0 {
		t.Fatalf("initial historical upsert = %#v", result)
	}
	undatedIncoming := undatedExisting
	undatedIncoming.ID = "rss-entry-undated-incoming"
	undatedIncoming.Title = "Dated archive cannot supersede an undated existing snapshot"
	undatedIncoming.ContentHash = "dated-incoming"
	undatedIncoming.SourceUpdatedAt = &newer
	result = upsertArchive(undatedIncoming)
	if result.Total.Updated != 0 || result.Visible.Updated != 0 {
		t.Fatalf("dated incoming against undated existing = %#v", result)
	}
	storedUndated, err := repo.GetEntry(ctx, undatedExisting.ID)
	if err != nil || storedUndated.Title != undatedExisting.Title || storedUndated.SourceUpdatedAt != nil {
		t.Fatalf("undated existing archive document = %#v, err=%v", storedUndated, err)
	}
	for _, stale := range []domainrss.Entry{
		archiveEntry("Older archive document", "archive-older", &older),
		archiveEntry("Same-time conflicting document", "archive-same", &newest),
		archiveEntry("Undated conflicting document", "archive-undated", nil),
	} {
		result = upsertArchive(stale)
		if result.Total.Created != 0 || result.Total.Updated != 0 ||
			result.Visible.Created != 0 || result.Visible.Updated != 0 {
			t.Fatalf("stale historical upsert = %#v", result)
		}
		stored, err := repo.GetEntry(ctx, "rss-entry-archive-document")
		if err != nil || stored.Title != "Newest archive document" || stored.SourceUpdatedAt == nil ||
			!stored.SourceUpdatedAt.Equal(newest) {
			t.Fatalf("document after stale archive = %#v, err=%v", stored, err)
		}
	}

	result = upsertArchive(archiveEntry("Strictly newer archive document", "archive-strictly-newer", &newer))
	if result.Total.Updated != 1 || result.Visible.Updated != 1 {
		t.Fatalf("newer historical upsert = %#v", result)
	}
	stored, err := repo.GetEntry(ctx, "rss-entry-archive-document")
	if err != nil || stored.Title != "Strictly newer archive document" || stored.SourceUpdatedAt == nil ||
		!stored.SourceUpdatedAt.Equal(newer) || stored.ReadAt == nil || stored.FieldRevisions.Read != 1 {
		t.Fatalf("strictly newer archive document = %#v, err=%v", stored, err)
	}

	// Mutable head feeds retain the existing UpsertFeed behavior. A live
	// correction without source_updated_at is allowed to replace archive data.
	current, err := repo.GetSubscription(ctx, subscription.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.Revision++
	current.UpdatedAt = now.Add(10 * time.Minute)
	live := archiveEntry("Live feed correction", "live-correction", nil)
	if result, err := repo.UpsertFeed(ctx, domainrss.FeedUpdate{
		Subscription: current,
		Entries:      []domainrss.Entry{live},
	}); err != nil || result.Updated != 1 {
		t.Fatalf("live upsert result = %#v, err=%v", result, err)
	}
	stored, err = repo.GetEntry(ctx, "rss-entry-archive-document")
	if err != nil || stored.Title != "Live feed correction" || stored.SourceUpdatedAt != nil {
		t.Fatalf("live corrected document = %#v, err=%v", stored, err)
	}
}
