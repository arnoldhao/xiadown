package rss

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/rssrepo"
)

type historyReadTrackingRepository struct {
	*rssrepo.SQLiteRepository
	getCalls atomic.Int32
	signalAt int32
	signaled chan struct{}
	once     sync.Once
}

func (repository *historyReadTrackingRepository) GetSubscription(
	ctx context.Context,
	id string,
) (domainrss.Subscription, error) {
	item, err := repository.SQLiteRepository.GetSubscription(ctx, id)
	if repository.getCalls.Add(1) == repository.signalAt {
		repository.once.Do(func() { close(repository.signaled) })
	}
	return item, err
}

func TestBackfillHistoryImportsOneReadPageAndPersistsPrivateCursor(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "history-vertical.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := rssrepo.NewSQLiteRepository(database.Bun)
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	lastSuccessAt := now.Add(-time.Minute)
	subscription := domainrss.Subscription{
		ID: "subscription-history", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "http://source-a.example/feed.json", SiteURL: "http://source-a.example/",
		Title: "History feed", ViewType: domainrss.ViewTypeArticle, Enabled: true,
		LastSuccessAt: &lastSuccessAt,
		CreatedAt:     now, UpdatedAt: now, Revision: 1,
	}
	if _, err := repository.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}

	var headRequests, pageTwoRequests, pageThreeRequests atomic.Int32
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/feed+json")
		switch request.Host + request.URL.Path {
		case "source-a.example/feed.json":
			headRequests.Add(1)
			_, _ = io.WriteString(response, `{
  "version":"https://jsonfeed.org/version/1.1",
  "title":"History feed",
  "home_page_url":"http://source-a.example/",
  "next_url":"/history/page-2.json",
  "items":[]
}`)
		case "source-a.example/history/page-2.json":
			pageTwoRequests.Add(1)
			_, _ = io.WriteString(response, `{
  "version":"https://jsonfeed.org/version/1.1",
  "title":"History feed",
  "home_page_url":"http://source-a.example/",
  "next_url":"page-3.json?token=private-cursor",
  "items":[{
    "id":"old-entry-2",
    "url":"http://source-a.example/posts/2",
    "title":"Older entry two",
    "content_text":"Archived body"
  }]
}`)
		case "source-a.example/history/page-3.json":
			pageThreeRequests.Add(1)
			if request.URL.Query().Get("token") != "private-cursor" {
				t.Errorf("history cursor token = %q", request.URL.Query().Get("token"))
			}
			_, _ = io.WriteString(response, `{
  "version":"https://jsonfeed.org/version/1.1",
  "title":"History feed",
  "home_page_url":"http://source-a.example/",
  "items":[{
    "id":"old-entry-1",
    "url":"http://source-a.example/posts/1",
    "title":"Oldest entry",
    "content_text":"Oldest archived body"
  }]
}`)
		default:
			t.Errorf("unexpected request %s%s", request.Host, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer closeServer()
	service.now = func() time.Time { return now.Add(10 * time.Minute) }

	first, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: subscription.ID, Kind: "article"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Subscriptions != 1 || first.Attempted != 1 || first.Supported != 1 ||
		first.Unsupported != 0 || first.Created != 1 || first.Updated != 0 || first.Failed != 0 ||
		first.Exhausted != 0 || !first.HasMore || len(first.Sources) != 1 || first.Sources[0].Exhausted {
		t.Fatalf("first backfill = %#v", first)
	}
	if headRequests.Load() != 1 || pageTwoRequests.Load() != 1 || pageThreeRequests.Load() != 0 {
		t.Fatalf("first request counts head=%d page2=%d page3=%d", headRequests.Load(), pageTwoRequests.Load(), pageThreeRequests.Load())
	}
	state, err := repository.GetSubscriptionHistory(ctx, subscription.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.CursorURL != "http://source-a.example/history/page-3.json?token=private-cursor" ||
		state.Capability != domainrss.HistoryCapabilityAvailable || state.Exhausted || state.LastSuccessAt == nil {
		t.Fatalf("history state = %#v", state)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-cursor") || strings.Contains(string(encoded), "cursorUrl") {
		t.Fatalf("Wails result exposed private cursor: %s", encoded)
	}
	page, err := repository.ListEntries(ctx, domainrss.EntryQuery{SubscriptionID: subscription.ID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ReadAt == nil || page.Items[0].FieldRevisions.Read != 1 ||
		page.Items[0].StateRevision != 1 || page.Items[0].Title != "Older entry two" {
		t.Fatalf("historical entry = %#v", page.Items)
	}
	if page.Items[0].ReadAt == nil || !page.Items[0].ReadAt.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("historical read time = %v", page.Items[0].ReadAt)
	}
	listed, err := repository.ListSubscriptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].UnreadCount != 0 {
		t.Fatalf("history polluted unread count: %#v", listed)
	}

	second, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: subscription.ID})
	if err != nil {
		t.Fatal(err)
	}
	if second.Created != 1 || second.Exhausted != 1 || second.HasMore || second.Failed != 0 ||
		pageThreeRequests.Load() != 1 || headRequests.Load() != 1 || pageTwoRequests.Load() != 1 {
		t.Fatalf("second backfill = %#v; requests head=%d page2=%d page3=%d", second, headRequests.Load(), pageTwoRequests.Load(), pageThreeRequests.Load())
	}
	third, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: subscription.ID})
	if err != nil {
		t.Fatal(err)
	}
	if third.Attempted != 0 || third.Exhausted != 1 || third.HasMore || pageThreeRequests.Load() != 1 {
		t.Fatalf("exhausted backfill = %#v", third)
	}
}

func TestBackfillHistoryPersistsUnsupportedWhenSourceHasNoHistoryLink(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "history-unsupported.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := rssrepo.NewSQLiteRepository(database.Bun)
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	lastSuccessAt := now.Add(-time.Minute)
	subscription := domainrss.Subscription{
		ID: "subscription-no-history", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "http://source-b.example/feed.atom", Title: "Finite feed",
		ViewType: domainrss.ViewTypeAuto, Enabled: true, LastSuccessAt: &lastSuccessAt,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	if _, err := repository.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		response.Header().Set("Content-Type", "application/atom+xml")
		_, _ = io.WriteString(response, `<feed xmlns="http://www.w3.org/2005/Atom"><title>Finite feed</title></feed>`)
	}))
	defer closeServer()
	service.now = func() time.Time { return now.Add(time.Minute) }

	result, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: subscription.ID, Kind: "video"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Subscriptions != 1 || result.Attempted != 1 || result.Unsupported != 1 ||
		result.Exhausted != 1 || result.Supported != 0 || result.Failed != 0 || result.HasMore {
		t.Fatalf("unsupported result = %#v", result)
	}
	state, err := repository.GetSubscriptionHistory(ctx, subscription.ID)
	if err != nil || state.Capability != domainrss.HistoryCapabilityUnsupported || !state.Exhausted || state.CursorURL != "" {
		t.Fatalf("unsupported state = %#v, err=%v", state, err)
	}
	if requests.Load() != 1 {
		t.Fatalf("unsupported source requests = %d", requests.Load())
	}
	result, err = service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: subscription.ID})
	if err != nil || result.Attempted != 0 || requests.Load() != 1 {
		t.Fatalf("cached unsupported result = %#v, err=%v, requests=%d", result, err, requests.Load())
	}
}

func TestRunBoundedRSSHistoryBackfillsUsesAtMostFourWorkers(t *testing.T) {
	const subscriptionCount = 1_000
	subscriptions := make([]domainrss.Subscription, subscriptionCount)
	for index := range subscriptions {
		subscriptions[index] = domainrss.Subscription{ID: fmt.Sprintf("subscription-%d", index)}
	}
	release := make(chan struct{})
	started := make(chan struct{}, maxConcurrentRSSRefreshes)
	var active, peak, calls atomic.Int64
	done := make(chan BackfillHistoryResult, 1)
	go func() {
		result, _ := runBoundedRSSHistoryBackfills(
			context.Background(), subscriptions, maxConcurrentRSSRefreshes,
			func(_ context.Context, subscription domainrss.Subscription) BackfillHistorySourceResult {
				current := active.Add(1)
				defer active.Add(-1)
				for {
					previous := peak.Load()
					if current <= previous || peak.CompareAndSwap(previous, current) {
						break
					}
				}
				call := calls.Add(1)
				if call <= maxConcurrentRSSRefreshes {
					started <- struct{}{}
				}
				<-release
				return BackfillHistorySourceResult{
					SubscriptionID: subscription.ID, Attempted: true,
					Capability: string(domainrss.HistoryCapabilityAvailable), Created: 1,
				}
			},
		)
		done <- result
	}()
	for range maxConcurrentRSSRefreshes {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for history workers")
		}
	}
	if calls.Load() != maxConcurrentRSSRefreshes {
		t.Fatalf("blocked history calls = %d", calls.Load())
	}
	close(release)
	select {
	case result := <-done:
		if result.Subscriptions != subscriptionCount || result.Attempted != subscriptionCount ||
			result.Created != subscriptionCount || result.Supported != subscriptionCount || !result.HasMore {
			t.Fatalf("bounded history result = %#v", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("bounded history backfill did not finish")
	}
	if peak.Load() != maxConcurrentRSSRefreshes || calls.Load() != subscriptionCount {
		t.Fatalf("history peak=%d calls=%d", peak.Load(), calls.Load())
	}
}

func TestBackfillHistoryExhaustsRepeatedNoProgressAndPersistsRetryableErrors(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "history-progress.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := rssrepo.NewSQLiteRepository(database.Bun)
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	lastSuccessAt := now.Add(-time.Minute)
	for _, subscription := range []domainrss.Subscription{
		{
			ID: "subscription-no-progress", WorkspaceID: domainrss.DefaultWorkspaceID,
			FeedURL: "http://source-a.example/feed.json", Title: "No progress",
			ViewType: domainrss.ViewTypeAuto, Enabled: true, LastSuccessAt: &lastSuccessAt,
			CreatedAt: now, UpdatedAt: now, Revision: 1,
		},
		{
			ID: "subscription-history-error", WorkspaceID: domainrss.DefaultWorkspaceID,
			FeedURL: "http://source-b.example/feed.json", Title: "Retryable history",
			ViewType: domainrss.ViewTypeAuto, Enabled: true, LastSuccessAt: &lastSuccessAt,
			CreatedAt: now, UpdatedAt: now, Revision: 1,
		},
	} {
		if _, err := repository.CreateSubscription(ctx, subscription); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.PutSubscriptionHistory(ctx, domainrss.SubscriptionHistoryState{
		SubscriptionID: "subscription-no-progress",
		CursorURL:      "http://source-a.example/history/repeat.json",
		Capability:     domainrss.HistoryCapabilityAvailable,
		NoProgress:     maxRSSHistoryNoProgress - 1,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.PutSubscriptionHistory(ctx, domainrss.SubscriptionHistoryState{
		SubscriptionID: "subscription-history-error",
		CursorURL:      "http://source-b.example/history/error.json?token=private",
		Capability:     domainrss.HistoryCapabilityAvailable,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatal(err)
	}

	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.Host + request.URL.Path {
		case "source-a.example/history/repeat.json":
			response.Header().Set("Content-Type", "application/feed+json")
			_, _ = io.WriteString(response, `{
  "version":"https://jsonfeed.org/version/1.1",
  "title":"No progress",
  "next_url":"/history/still-more.json",
  "items":[]
}`)
		case "source-b.example/history/error.json":
			response.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(response, "upstream failed")
		default:
			t.Errorf("unexpected request %s%s", request.Host, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer closeServer()
	service.now = func() time.Time { return now.Add(time.Minute) }

	noProgress, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: "subscription-no-progress"})
	if err != nil {
		t.Fatal(err)
	}
	if noProgress.Attempted != 1 || noProgress.Exhausted != 1 || noProgress.HasMore ||
		noProgress.Created != 0 || noProgress.Updated != 0 || noProgress.Sources[0].NoProgress != maxRSSHistoryNoProgress {
		t.Fatalf("no-progress result = %#v", noProgress)
	}
	state, err := repository.GetSubscriptionHistory(ctx, "subscription-no-progress")
	if err != nil || !state.Exhausted || state.NoProgress != maxRSSHistoryNoProgress ||
		state.CursorURL != "http://source-a.example/history/still-more.json" {
		t.Fatalf("no-progress state = %#v, err=%v", state, err)
	}

	retryable, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: "subscription-history-error"})
	if err != nil {
		t.Fatal(err)
	}
	if retryable.Attempted != 1 || retryable.Failed != 1 || retryable.Exhausted != 0 ||
		!retryable.HasMore || retryable.Sources[0].Error == "" ||
		strings.Contains(retryable.Sources[0].Error, "private") {
		t.Fatalf("retryable error result = %#v", retryable)
	}
	state, err = repository.GetSubscriptionHistory(ctx, "subscription-history-error")
	if err != nil || state.Exhausted || state.LastError == "" ||
		state.CursorURL != "http://source-b.example/history/error.json?token=private" ||
		strings.Contains(state.LastError, "private") {
		t.Fatalf("retryable error state = %#v, err=%v", state, err)
	}
}

func TestBackfillHistorySkipsPendingSubscriptionsForAggregateAndExplicitScopes(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "history-pending.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := rssrepo.NewSQLiteRepository(database.Bun)
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	lastSuccessAt := now.Add(-time.Minute)
	hydrated := domainrss.Subscription{
		ID: "subscription-hydrated", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "https://example.com/hydrated.xml", Title: "Hydrated",
		ViewType: domainrss.ViewTypeArticle, Enabled: true, LastSuccessAt: &lastSuccessAt,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	pending := domainrss.Subscription{
		ID: "subscription-pending", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "https://example.com/pending.xml", Title: "Pending",
		ViewType: domainrss.ViewTypeArticle, Enabled: true,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	for _, item := range []domainrss.Subscription{hydrated, pending} {
		if _, err := repository.CreateSubscription(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.PutSubscriptionHistory(ctx, domainrss.SubscriptionHistoryState{
		SubscriptionID: hydrated.ID, Capability: domainrss.HistoryCapabilityUnsupported,
		Exhausted: true, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, nil)

	aggregate, err := service.BackfillHistory(ctx, BackfillHistoryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if aggregate.Subscriptions != 1 || len(aggregate.Sources) != 1 ||
		aggregate.Sources[0].SubscriptionID != hydrated.ID || aggregate.HasMore {
		t.Fatalf("aggregate pending filter = %#v", aggregate)
	}
	explicit, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: pending.ID})
	if err != nil {
		t.Fatal(err)
	}
	if explicit.Subscriptions != 0 || len(explicit.Sources) != 0 || explicit.HasMore ||
		explicit.Attempted != 0 || explicit.Created != 0 || explicit.Updated != 0 {
		t.Fatalf("explicit pending result = %#v", explicit)
	}
}

func TestBackfillHistoryKindCountsOnlyVisibleEntriesWhilePersistingMixedPage(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "history-kind-visible.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := rssrepo.NewSQLiteRepository(database.Bun)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	lastSuccessAt := now.Add(-time.Minute)
	subscription := domainrss.Subscription{
		ID: "subscription-mixed-history", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "http://source-a.example/feed.json", Title: "Mixed history",
		ViewType: domainrss.ViewTypeAuto, Enabled: true, LastSuccessAt: &lastSuccessAt,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	if _, err := repository.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	if err := repository.PutSubscriptionHistory(ctx, domainrss.SubscriptionHistoryState{
		SubscriptionID: subscription.ID,
		CursorURL:      "http://source-a.example/history/mixed.json",
		Capability:     domainrss.HistoryCapabilityAvailable,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatal(err)
	}
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Host+request.URL.Path != "source-a.example/history/mixed.json" {
			t.Errorf("unexpected request %s%s", request.Host, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
			return
		}
		response.Header().Set("Content-Type", "application/feed+json")
		_, _ = io.WriteString(response, `{
  "version":"https://jsonfeed.org/version/1.1",
  "title":"Mixed history",
  "home_page_url":"http://source-a.example/",
  "items":[
    {"id":"archived-article","url":"http://source-a.example/posts/article","title":"Archived article","content_text":"Article body","date_modified":"2026-07-01T00:00:00Z"},
    {"id":"archived-video","url":"https://www.youtube.com/watch?v=abcdefghijk","title":"Archived video","date_modified":"2026-07-01T01:00:00Z"}
  ]
}`)
	}))
	defer closeServer()
	service.now = func() time.Time { return now.Add(time.Minute) }

	result, err := service.BackfillHistory(ctx, BackfillHistoryRequest{Kind: "article"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Subscriptions != 1 || result.Created != 1 || result.Updated != 0 ||
		len(result.Sources) != 1 || result.Sources[0].Created != 1 ||
		result.Sources[0].Updated != 0 || result.Sources[0].NoProgress != 0 {
		t.Fatalf("kind-visible history result = %#v", result)
	}
	all, err := repository.ListEntries(ctx, domainrss.EntryQuery{SubscriptionID: subscription.ID, Limit: 20})
	if err != nil || all.Total != 2 || len(all.Items) != 2 {
		t.Fatalf("persisted mixed history = %#v, err=%v", all, err)
	}
	articles, err := repository.ListEntries(ctx, domainrss.EntryQuery{
		SubscriptionID: subscription.ID, Kind: domainrss.EntryKindArticle, Limit: 20,
	})
	if err != nil || articles.Total != 1 {
		t.Fatalf("persisted article history = %#v, err=%v", articles, err)
	}
	videos, err := repository.ListEntries(ctx, domainrss.EntryQuery{
		SubscriptionID: subscription.ID, Kind: domainrss.EntryKindVideo, Limit: 20,
	})
	if err != nil || videos.Total != 1 {
		t.Fatalf("persisted video history = %#v, err=%v", videos, err)
	}
}

func TestBackfillHistoryNeverProjectsCursorUserinfoQueryOrPathTokens(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "history-secret-errors.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := rssrepo.NewSQLiteRepository(database.Bun)
	now := time.Date(2026, 7, 14, 13, 0, 0, 0, time.UTC)
	lastSuccessAt := now.Add(-time.Minute)
	subscription := domainrss.Subscription{
		ID: "subscription-secret-history", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "https://example.com/feed.json", Title: "Secret history",
		ViewType: domainrss.ViewTypeArticle, Enabled: true, LastSuccessAt: &lastSuccessAt,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	if _, err := repository.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	privateCursor := "http://userinfo-secret:password-secret@source-a.example/history/path-token-secret.json?token=query-secret"
	if err := repository.PutSubscriptionHistory(ctx, domainrss.SubscriptionHistoryState{
		SubscriptionID: subscription.ID, CursorURL: privateCursor,
		Capability: domainrss.HistoryCapabilityAvailable, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, nil)
	service.now = func() time.Time { return now.Add(time.Minute) }
	result, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: subscription.ID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed != 1 || len(result.Sources) != 1 || result.Sources[0].Error != historyErrorUnavailable {
		t.Fatalf("secret cursor failure = %#v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"userinfo-secret", "password-secret", "path-token-secret", "query-secret"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("history result leaked %q: %s", secret, encoded)
		}
	}
	state, err := repository.GetSubscriptionHistory(ctx, subscription.ID)
	if err != nil || state.CursorURL != privateCursor || state.LastError != historyErrorUnavailable {
		t.Fatalf("private history state = %#v, err=%v", state, err)
	}
}

func TestConcurrentBackfillHistoryReloadsSubscriptionAfterPerFeedLock(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "history-concurrent.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	baseRepository := rssrepo.NewSQLiteRepository(database.Bun)
	repository := &historyReadTrackingRepository{
		SQLiteRepository: baseRepository,
		signalAt:         3,
		signaled:         make(chan struct{}),
	}
	now := time.Date(2026, 7, 14, 14, 0, 0, 0, time.UTC)
	lastSuccessAt := now.Add(-time.Minute)
	subscription := domainrss.Subscription{
		ID: "subscription-concurrent-history", WorkspaceID: domainrss.DefaultWorkspaceID,
		FeedURL: "http://source-a.example/feed.json", Title: "Concurrent history",
		ViewType: domainrss.ViewTypeArticle, Enabled: true, LastSuccessAt: &lastSuccessAt,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	if _, err := repository.CreateSubscription(ctx, subscription); err != nil {
		t.Fatal(err)
	}
	pageTwoStarted := make(chan struct{})
	releasePageTwo := make(chan struct{})
	var pageTwoOnce sync.Once
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/feed+json")
		switch request.Host + request.URL.Path {
		case "source-a.example/feed.json":
			_, _ = io.WriteString(response, `{"version":"https://jsonfeed.org/version/1.1","title":"Concurrent history","next_url":"/history/page-2.json","items":[]}`)
		case "source-a.example/history/page-2.json":
			pageTwoOnce.Do(func() { close(pageTwoStarted) })
			<-releasePageTwo
			_, _ = io.WriteString(response, `{"version":"https://jsonfeed.org/version/1.1","title":"Concurrent history","next_url":"/history/page-3.json","items":[{"id":"old-2","title":"Older two","date_modified":"2026-07-02T00:00:00Z"}]}`)
		case "source-a.example/history/page-3.json":
			_, _ = io.WriteString(response, `{"version":"https://jsonfeed.org/version/1.1","title":"Concurrent history","items":[{"id":"old-1","title":"Older one","date_modified":"2026-07-01T00:00:00Z"}]}`)
		default:
			t.Errorf("unexpected request %s%s", request.Host, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer closeServer()
	service.now = func() time.Time { return now.Add(time.Minute) }
	type outcome struct {
		result BackfillHistoryResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	run := func() {
		result, err := service.BackfillHistory(ctx, BackfillHistoryRequest{SubscriptionID: subscription.ID})
		outcomes <- outcome{result: result, err: err}
	}
	go run()
	select {
	case <-pageTwoStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("first history request did not reach page two")
	}
	go run()
	select {
	case <-repository.signaled:
	case <-time.After(5 * time.Second):
		t.Fatal("second history request did not capture the stale list revision")
	}
	close(releasePageTwo)
	for range 2 {
		select {
		case outcome := <-outcomes:
			if outcome.err != nil || outcome.result.Created != 1 || outcome.result.Failed != 0 {
				t.Fatalf("concurrent history outcome = %#v, err=%v", outcome.result, outcome.err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent history request did not finish")
		}
	}
	page, err := repository.ListEntries(ctx, domainrss.EntryQuery{SubscriptionID: subscription.ID, Limit: 20})
	if err != nil || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("concurrent history entries = %#v, err=%v", page, err)
	}
	current, err := repository.SQLiteRepository.GetSubscription(ctx, subscription.ID)
	if err != nil || current.Revision != 3 {
		t.Fatalf("subscription after concurrent history = %#v, err=%v", current, err)
	}
	state, err := repository.GetSubscriptionHistory(ctx, subscription.ID)
	if err != nil || !state.Exhausted || state.CursorURL != "" {
		t.Fatalf("history state after concurrent requests = %#v, err=%v", state, err)
	}
}

func TestBackfillHistoryRejectsInvalidKindBeforeNetworkWork(t *testing.T) {
	service := NewService(nil, nil)
	if _, err := service.BackfillHistory(context.Background(), BackfillHistoryRequest{Kind: "audio"}); err == nil || err.Error() != "invalid RSS entry kind" {
		t.Fatal("invalid history kind unexpectedly succeeded")
	}
}
