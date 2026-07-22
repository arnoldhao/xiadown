package rss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/rssrepo"
)

func TestPendingSubscriptionQueuedBeforeRunHydratesWithAppContext(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "pending-hydration.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := rssrepo.NewSQLiteRepository(database.Bun)
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.Host + request.URL.Path {
		case "source-a.example/feed":
			response.Header().Set("Content-Type", "application/rss+xml")
			_, _ = io.WriteString(response, `<?xml version="1.0"?><rss version="2.0"><channel>
  <title>Hydrated immediately</title>
  <link>http://source-a.example/</link>
  <item><guid>post-1</guid><title>First hydrated post</title><link>http://source-a.example/posts/1</link></item>
</channel></rss>`)
		case "source-a.example/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(response, `<html><head></head></html>`)
		default:
			t.Errorf("unexpected request %s%s", request.Host, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer closeServer()
	now := time.Date(2026, 7, 14, 12, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	created, err := service.AddSubscription(requestCtx, AddSubscriptionRequest{
		URL: "http://source-a.example/feed", ViewType: "article", AllowPending: true,
	})
	cancelRequest()
	if err != nil {
		t.Fatal(err)
	}
	if created.LastSuccessAt != nil || created.Title != "source-a.example" {
		t.Fatalf("initial pending subscription = %#v", created)
	}

	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		service.Run(runCtx, time.Hour, time.Hour)
		close(runDone)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		stored, getErr := repository.GetSubscription(ctx, created.ID)
		page, listErr := repository.ListEntries(ctx, domainrss.EntryQuery{SubscriptionID: created.ID, Limit: 10})
		if getErr == nil && listErr == nil && stored.LastSuccessAt != nil && len(page.Items) == 1 {
			if stored.Title != "Hydrated immediately" || stored.LastError != "" ||
				page.Items[0].Title != "First hydrated post" {
				t.Fatalf("hydrated subscription=%#v entries=%#v", stored, page.Items)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pending hydration timed out: subscription=%#v getErr=%v page=%#v listErr=%v", stored, getErr, page, listErr)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancelRun()
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("RSS Run did not wait for hydration workers to stop")
	}
}

func TestPendingHydrationQueueCoalescesAndBoundsWorkers(t *testing.T) {
	queue := newPendingHydrationQueue(8)
	for index := 0; index < 6; index++ {
		id := fmt.Sprintf("subscription-%d", index)
		if !queue.Enqueue(id) {
			t.Fatalf("enqueue %s failed", id)
		}
		if !queue.Enqueue(id) || !queue.Enqueue("  "+id+"  ") {
			t.Fatalf("coalesce %s failed", id)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	completed := make(chan struct{}, 6)
	var active, peak, calls atomic.Int64
	wait := queue.startWorkers(ctx, 2, func(context.Context, string) (domainrss.UpsertResult, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := peak.Load()
			if current <= previous || peak.CompareAndSwap(previous, current) {
				break
			}
		}
		call := calls.Add(1)
		if call <= 2 {
			started <- struct{}{}
		}
		<-release
		completed <- struct{}{}
		return domainrss.UpsertResult{}, nil
	})
	for range 2 {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for bounded hydration workers")
		}
	}
	if calls.Load() != 2 || peak.Load() != 2 {
		t.Fatalf("blocked calls=%d peak=%d", calls.Load(), peak.Load())
	}
	close(release)
	for range 6 {
		select {
		case <-completed:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for hydration completion")
		}
	}
	if calls.Load() != 6 || peak.Load() > 2 {
		t.Fatalf("coalesced calls=%d peak=%d", calls.Load(), peak.Load())
	}
	cancel()
	waitForPendingHydrationWorkers(t, wait)
}

func TestPendingHydrationQueueRetriesWithoutBlockingWorkerAndStopsCleanly(t *testing.T) {
	queue := newPendingHydrationQueue(2)
	queue.retryDelays = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	if !queue.Enqueue("subscription-retry") {
		t.Fatal("enqueue retry fixture failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int64
	succeeded := make(chan struct{}, 1)
	wait := queue.startWorkers(ctx, 1, func(context.Context, string) (domainrss.UpsertResult, error) {
		if calls.Add(1) < 3 {
			return domainrss.UpsertResult{}, errors.New("temporary feed failure")
		}
		succeeded <- struct{}{}
		return domainrss.UpsertResult{Created: 1}, nil
	})
	select {
	case <-succeeded:
	case <-time.After(2 * time.Second):
		t.Fatal("bounded hydration retries did not succeed")
	}
	if calls.Load() != 3 {
		t.Fatalf("retry calls = %d, want initial plus two retries", calls.Load())
	}
	cancel()
	queue.stopRetries()
	waitForPendingHydrationWorkers(t, wait)

	shutdownQueue := newPendingHydrationQueue(1)
	shutdownQueue.retryDelays = []time.Duration{time.Hour}
	if !shutdownQueue.Enqueue("subscription-shutdown") {
		t.Fatal("enqueue shutdown fixture failed")
	}
	shutdownCtx, cancelShutdown := context.WithCancel(context.Background())
	attempted := make(chan struct{}, 1)
	shutdownWait := shutdownQueue.startWorkers(shutdownCtx, 1, func(context.Context, string) (domainrss.UpsertResult, error) {
		attempted <- struct{}{}
		return domainrss.UpsertResult{}, errors.New("offline")
	})
	<-attempted
	cancelShutdown()
	shutdownQueue.stopRetries()
	waitForPendingHydrationWorkers(t, shutdownWait)
	time.Sleep(20 * time.Millisecond)
	if len(shutdownQueue.jobs) != 0 {
		t.Fatalf("shutdown retry re-entered queue: %d jobs", len(shutdownQueue.jobs))
	}

	boundedQueue := newPendingHydrationQueue(1)
	boundedQueue.retryDelays = []time.Duration{
		5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond,
	}
	if !boundedQueue.Enqueue("subscription-bounded") {
		t.Fatal("enqueue bounded retry fixture failed")
	}
	boundedCtx, cancelBounded := context.WithCancel(context.Background())
	var boundedCalls atomic.Int64
	boundedAttempts := make(chan struct{}, 4)
	boundedWait := boundedQueue.startWorkers(boundedCtx, 1, func(context.Context, string) (domainrss.UpsertResult, error) {
		boundedCalls.Add(1)
		boundedAttempts <- struct{}{}
		return domainrss.UpsertResult{}, errors.New("still offline")
	})
	for range 4 {
		select {
		case <-boundedAttempts:
		case <-time.After(time.Second):
			t.Fatal("bounded retry sequence did not finish")
		}
	}
	time.Sleep(30 * time.Millisecond)
	boundedQueue.mu.Lock()
	tracked := len(boundedQueue.ids)
	boundedQueue.mu.Unlock()
	if boundedCalls.Load() != 4 || tracked != 0 {
		t.Fatalf("bounded attempts=%d tracked=%d, want initial plus three retries and no timer", boundedCalls.Load(), tracked)
	}
	cancelBounded()
	boundedQueue.stopRetries()
	waitForPendingHydrationWorkers(t, boundedWait)
}

func TestPendingHydrationQueueFullFallsBackWithoutBlocking(t *testing.T) {
	queue := newPendingHydrationQueue(2)
	if !queue.Enqueue("subscription-1") || !queue.Enqueue("subscription-2") {
		t.Fatal("queue rejected work before reaching capacity")
	}
	if queue.Enqueue("subscription-3") {
		t.Fatal("queue accepted work beyond capacity")
	}
	if !queue.Enqueue("subscription-1") {
		t.Fatal("coalesced ID was rejected at capacity")
	}

	retrying := newPendingHydrationQueue(1)
	retrying.retryDelays = []time.Duration{time.Hour}
	if !retrying.Enqueue("subscription-retrying") {
		t.Fatal("retry fixture was not enqueued")
	}
	<-retrying.jobs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !retrying.scheduleRetry(ctx, "subscription-retrying") {
		t.Fatal("retry fixture was not scheduled")
	}
	if retrying.Enqueue("subscription-new") {
		t.Fatal("queue exceeded capacity while an ID was waiting on a retry timer")
	}
	retrying.stopRetries()
}

type refreshStateFailureRepository struct {
	domainrss.Repository
	subscription domainrss.Subscription
	persisted    domainrss.Subscription
	persistErr   error
	updates      int
}

type hydrationEligibilityRepository struct {
	domainrss.Repository
	subscription domainrss.Subscription
	err          error
}

func (repository *hydrationEligibilityRepository) GetSubscription(context.Context, string) (domainrss.Subscription, error) {
	return repository.subscription, repository.err
}

func TestPendingHydrationStopsForDeletedDisabledAndSuccessfulSubscriptions(t *testing.T) {
	now := time.Date(2026, 7, 14, 13, 30, 0, 0, time.UTC)
	tests := []struct {
		name         string
		subscription domainrss.Subscription
		err          error
	}{
		{name: "deleted", err: domainrss.ErrNotFound},
		{name: "disabled", subscription: domainrss.Subscription{ID: "disabled", Enabled: false}},
		{name: "already hydrated", subscription: domainrss.Subscription{ID: "hydrated", Enabled: true, LastSuccessAt: &now}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(&hydrationEligibilityRepository{
				subscription: test.subscription, err: test.err,
			}, nil)
			if _, err := service.hydratePendingSubscription(context.Background(), test.subscription.ID); err != nil {
				t.Fatalf("ineligible hydration error = %v", err)
			}
		})
	}
}

func (repository *refreshStateFailureRepository) GetSubscription(context.Context, string) (domainrss.Subscription, error) {
	return repository.subscription, nil
}

func (repository *refreshStateFailureRepository) UpdateSubscription(_ context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	repository.persisted = item
	repository.updates++
	return domainrss.Subscription{}, repository.persistErr
}

func TestRefreshOneReturnsFetchAndStatePersistenceErrors(t *testing.T) {
	now := time.Date(2026, 7, 14, 13, 0, 0, 0, time.UTC)
	persistErr := errors.New("persist refresh failure state")
	repository := &refreshStateFailureRepository{
		subscription: domainrss.Subscription{
			ID: "subscription-failure", WorkspaceID: domainrss.DefaultWorkspaceID,
			FeedURL: "http://source-a.example/feed", Title: "Pending",
			ViewType: domainrss.ViewTypeAuto, Enabled: true,
			CreatedAt: now, UpdatedAt: now, Revision: 1,
		},
		persistErr: persistErr,
	}
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadGateway)
	}))
	defer closeServer()
	service.now = func() time.Time { return now.Add(time.Minute) }

	_, err := service.refreshOne(context.Background(), repository.subscription.ID)
	if err == nil || !errors.Is(err, persistErr) || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("joined refresh error = %v", err)
	}
	if repository.updates != 1 || repository.persisted.LastFetchedAt == nil ||
		repository.persisted.LastError == "" || repository.persisted.Revision != 2 ||
		!repository.persisted.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("persisted failure state = %#v, updates=%d", repository.persisted, repository.updates)
	}
}

func waitForPendingHydrationWorkers(t *testing.T, wait interface{ Wait() }) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pending hydration workers did not stop")
	}
}
