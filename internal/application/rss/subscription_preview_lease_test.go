package rss

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

func TestAddSubscriptionReusesSuccessfulPreviewWithoutSecondFeedRequest(t *testing.T) {
	repository := &atomicAddRepository{}
	var feedRequests atomic.Int32
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/feed":
			feedRequests.Add(1)
			response.Header().Set("Content-Type", "application/rss+xml")
			_, _ = response.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Preview once</title><link>http://source-a.example/</link><item><guid>post-1</guid><title>First post</title><link>http://source-a.example/posts/1</link></item></channel></rss>`))
		case "/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<html><head></head></html>`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer closeServer()

	preview, err := service.PreviewSubscription(context.Background(), PreviewSubscriptionRequest{
		URL: "http://source-a.example/feed", ViewType: "article",
	})
	if err != nil {
		t.Fatalf("preview subscription: %v", err)
	}
	if preview.PreviewToken == "" {
		t.Fatal("preview token is empty")
	}
	created, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: "http://source-a.example/feed", ViewType: "article", PreviewToken: preview.PreviewToken,
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}
	if got := feedRequests.Load(); got != 1 {
		t.Fatalf("feed requests = %d, want exactly one preview request", got)
	}
	if created.Title != "Preview once" || repository.createCalls != 1 || len(repository.created.Entries) != 1 {
		t.Fatalf("created subscription=%#v update=%#v", created, repository.created)
	}
	if _, _, status := service.claimPreviewLease(preview.PreviewToken, "http://source-a.example/feed", service.now()); status != previewLeaseMissing {
		t.Fatalf("consumed preview status = %v, want missing", status)
	}
}

type pendingAddRepository struct {
	stateRepositoryStub
	created domainrss.Subscription
	calls   int
}

func (repo *pendingAddRepository) CreateSubscription(_ context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	repo.calls++
	repo.created = item
	return item, nil
}

func TestAddSubscriptionCanCreatePendingFeedWithoutPreviewOrNetwork(t *testing.T) {
	repository := &pendingAddRepository{}
	service := NewService(repository, nil)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	created, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: "https://feeds.example.com/rss.xml", Title: "Keep this feed", ViewType: "article", AllowPending: true,
	})
	if err != nil {
		t.Fatalf("add pending subscription: %v", err)
	}
	if repository.calls != 1 {
		t.Fatalf("CreateSubscription calls = %d, want 1", repository.calls)
	}
	if created.Title != "Keep this feed" || created.FeedURL != "https://feeds.example.com/rss.xml" || !created.Enabled {
		t.Fatalf("pending subscription = %#v", created)
	}
	if created.LastError == "" || created.LastFetchedAt != nil || created.LastSuccessAt != nil {
		t.Fatalf("pending fetch state = %#v", created)
	}
}

func TestExpiredPreviewLeaseFallsBackToPendingWhenExplicitlyAllowed(t *testing.T) {
	repository := &pendingAddRepository{}
	service := NewService(repository, nil)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	canonical := "https://feeds.example.com/rss.xml"
	token := service.storePreviewLease(
		canonical,
		newPreviewFeedSnapshot(canonical, parsedFeed{Title: "Expired preview"}, now),
		fetchMetadata{},
		now.Add(-previewLeaseTTL-time.Second),
	)

	created, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: "https://feeds.example.com/rss.xml", PreviewToken: token, AllowPending: true,
	})
	if err != nil {
		t.Fatalf("add pending subscription: %v", err)
	}
	if created.Title == "Expired preview" || created.LastError == "" {
		t.Fatalf("expired preview was unexpectedly reused: %#v", created)
	}
}

type failOncePreviewRepository struct {
	stateRepositoryStub
	mu          sync.Mutex
	createCalls int
}

func (repo *failOncePreviewRepository) CreateFeed(
	_ context.Context,
	update domainrss.FeedUpdate,
) (domainrss.Subscription, domainrss.UpsertResult, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.createCalls++
	if repo.createCalls == 1 {
		return domainrss.Subscription{}, domainrss.UpsertResult{}, errors.New("transient database error")
	}
	return update.Subscription, domainrss.UpsertResult{Created: len(update.Entries)}, nil
}

func TestPreviewLeaseIsReleasedWhenCreateFeedFails(t *testing.T) {
	repository := &failOncePreviewRepository{}
	service := NewService(repository, nil)
	cleanupPreviewLeases(t, service)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	canonical := "https://feeds.example.com/rss.xml"
	token := service.storePreviewLease(
		canonical,
		previewLeaseTestSnapshot(canonical, "Retry without refetch", now),
		fetchMetadata{ETag: `"preview"`},
		now,
	)
	request := AddSubscriptionRequest{
		URL: canonical, ViewType: "article", PreviewToken: token, AllowPending: true,
	}
	if _, err := service.AddSubscription(context.Background(), request); err == nil {
		t.Fatal("first CreateFeed unexpectedly succeeded")
	}
	created, err := service.AddSubscription(context.Background(), request)
	if err != nil {
		t.Fatalf("retry released preview lease: %v", err)
	}
	if created.Title != "Retry without refetch" || repository.createCalls != 2 {
		t.Fatalf("retry result=%#v create calls=%d", created, repository.createCalls)
	}
}

func TestPreviewSnapshotMaterializesTheFinalSelectedView(t *testing.T) {
	repository := &atomicAddRepository{}
	service := NewService(repository, nil)
	cleanupPreviewLeases(t, service)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	canonical := "https://feeds.example.com/rss.xml"
	token := service.storePreviewLease(
		canonical,
		previewLeaseTestSnapshot(canonical, "Change view", now),
		fetchMetadata{ETag: `"preview"`, ValidatorURL: canonical},
		now,
	)
	created, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: canonical, ViewType: "social", PreviewToken: token, AllowPending: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ViewType != domainrss.ViewTypeSocial || len(repository.created.Entries) != 1 ||
		repository.created.Entries[0].Kind != domainrss.EntryKindSocial {
		t.Fatalf("materialized feed = %#v", repository.created)
	}
	if created.ETag != `"preview"` || created.ValidatorURL != canonical {
		t.Fatalf("materialized validators = %#v", created)
	}
}

type blockingPreviewRepository struct {
	stateRepositoryStub
	entered      chan struct{}
	unblock      chan struct{}
	createCalls  atomic.Int32
	pendingCalls atomic.Int32
}

func (repo *blockingPreviewRepository) CreateFeed(
	_ context.Context,
	update domainrss.FeedUpdate,
) (domainrss.Subscription, domainrss.UpsertResult, error) {
	if repo.createCalls.Add(1) == 1 {
		close(repo.entered)
		<-repo.unblock
	}
	return update.Subscription, domainrss.UpsertResult{Created: len(update.Entries)}, nil
}

func (repo *blockingPreviewRepository) CreateSubscription(
	_ context.Context,
	item domainrss.Subscription,
) (domainrss.Subscription, error) {
	repo.pendingCalls.Add(1)
	return item, nil
}

func TestPreviewLeaseRejectsConcurrentSecondClaimWithoutCreatingPending(t *testing.T) {
	repository := &blockingPreviewRepository{entered: make(chan struct{}), unblock: make(chan struct{})}
	service := NewService(repository, nil)
	cleanupPreviewLeases(t, service)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	canonical := "https://feeds.example.com/rss.xml"
	token := service.storePreviewLease(
		canonical,
		previewLeaseTestSnapshot(canonical, "Claim once", now),
		fetchMetadata{},
		now,
	)
	request := AddSubscriptionRequest{
		URL: canonical, ViewType: "article", PreviewToken: token, AllowPending: true,
	}
	firstResult := make(chan error, 1)
	go func() {
		_, err := service.AddSubscription(context.Background(), request)
		firstResult <- err
	}()
	select {
	case <-repository.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first AddSubscription did not reach CreateFeed")
	}
	if _, err := service.AddSubscription(context.Background(), request); !errors.Is(err, errPreviewLeaseInUse) {
		t.Fatalf("concurrent AddSubscription error = %v, want preview busy", err)
	}
	if got := repository.pendingCalls.Load(); got != 0 {
		t.Fatalf("concurrent claim created %d pending subscriptions", got)
	}
	close(repository.unblock)
	if err := <-firstResult; err != nil {
		t.Fatalf("first AddSubscription: %v", err)
	}
	if got := repository.createCalls.Load(); got != 1 {
		t.Fatalf("CreateFeed calls = %d, want 1", got)
	}
}

func TestPreviewLeaseBindsCanonicalURLWithoutConsumingMismatch(t *testing.T) {
	service := NewService(&stateRepositoryStub{}, nil)
	cleanupPreviewLeases(t, service)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	canonical := "https://feeds.example.com/rss.xml"
	token := service.storePreviewLease(
		canonical,
		previewLeaseTestSnapshot(canonical, "Bound", now),
		fetchMetadata{},
		now,
	)
	if _, _, status := service.claimPreviewLease(token, "https://other.example.com/rss.xml", now); status != previewLeaseMissing {
		t.Fatalf("mismatched URL status = %v, want missing", status)
	}
	if _, _, status := service.claimPreviewLease(token, canonical, now); status != previewLeaseAcquired {
		t.Fatalf("matching URL status = %v, want acquired", status)
	}
	service.releasePreviewLease(token, canonical, now)
}

func TestPreviewLeaseCountAndByteBudgetsEvictOldestUnclaimedLease(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		service := NewService(&stateRepositoryStub{}, nil)
		cleanupPreviewLeases(t, service)
		now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		tokens := make([]string, 0, maxPreviewLeases+1)
		for index := 0; index <= maxPreviewLeases; index++ {
			createdAt := now.Add(time.Duration(index) * time.Nanosecond)
			canonical := fmt.Sprintf("https://feeds.example.com/%d.xml", index)
			tokens = append(tokens, service.storePreviewLease(
				canonical,
				previewLeaseTestSnapshot(canonical, fmt.Sprintf("Feed %d", index), createdAt),
				fetchMetadata{},
				createdAt,
			))
		}
		if _, _, status := service.claimPreviewLease(tokens[0], "https://feeds.example.com/0.xml", now.Add(time.Second)); status != previewLeaseMissing {
			t.Fatalf("oldest count-bounded lease status = %v, want missing", status)
		}
		if got := len(service.previewLeases); got != maxPreviewLeases {
			t.Fatalf("lease count = %d, want %d", got, maxPreviewLeases)
		}
	})

	t.Run("bytes", func(t *testing.T) {
		service := NewService(&stateRepositoryStub{}, nil)
		cleanupPreviewLeases(t, service)
		now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		largeContent := strings.Repeat("x", int(maxPreviewLeaseBytes/2))
		firstSnapshot := previewFeedSnapshot{
			title: "First", entries: []previewEntrySnapshot{{externalID: "first", title: "First", contentHTML: largeContent}},
		}
		secondSnapshot := previewFeedSnapshot{
			title: "Second", entries: []previewEntrySnapshot{{externalID: "second", title: "Second", contentHTML: largeContent}},
		}
		first := service.storePreviewLease("https://feeds.example.com/first.xml", firstSnapshot, fetchMetadata{}, now)
		second := service.storePreviewLease("https://feeds.example.com/second.xml", secondSnapshot, fetchMetadata{}, now.Add(time.Nanosecond))
		if first == "" || second == "" {
			t.Fatalf("budget test tokens = (%q, %q)", first, second)
		}
		if _, _, status := service.claimPreviewLease(first, "https://feeds.example.com/first.xml", now.Add(time.Second)); status != previewLeaseMissing {
			t.Fatalf("oldest byte-bounded lease status = %v, want missing", status)
		}
		if service.previewLeaseBytes > maxPreviewLeaseBytes {
			t.Fatalf("lease bytes = %d, budget = %d", service.previewLeaseBytes, maxPreviewLeaseBytes)
		}
	})
}

func TestPreviewLeaseExpiryCallbackReleasesByteBudget(t *testing.T) {
	service := NewService(&stateRepositoryStub{}, nil)
	cleanupPreviewLeases(t, service)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	canonical := "https://feeds.example.com/rss.xml"
	token := service.storePreviewLease(
		canonical,
		previewLeaseTestSnapshot(canonical, "Expires", now),
		fetchMetadata{},
		now,
	)
	service.previewMu.Lock()
	expiresAt := service.previewLeases[token].expiresAt
	service.previewMu.Unlock()
	service.expirePreviewLease(token, expiresAt)
	if len(service.previewLeases) != 0 || service.previewLeaseBytes != 0 {
		t.Fatalf("expired leases=%d bytes=%d", len(service.previewLeases), service.previewLeaseBytes)
	}
}

func previewLeaseTestSnapshot(canonical, title string, now time.Time) previewFeedSnapshot {
	return newPreviewFeedSnapshot(canonical, parsedFeed{
		Title: title,
		Entries: []parsedEntry{{
			ExternalID: "entry-1",
			URL:        "https://feeds.example.com/entries/1",
			Title:      "First entry",
			Content:    "<p>Preview content</p>",
		}},
	}, now)
}

func cleanupPreviewLeases(t *testing.T, service *Service) {
	t.Helper()
	t.Cleanup(func() {
		service.previewMu.Lock()
		defer service.previewMu.Unlock()
		for token := range service.previewLeases {
			service.removePreviewLeaseLocked(token)
		}
	})
}
