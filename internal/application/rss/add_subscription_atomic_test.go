package rss

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

type atomicAddRepository struct {
	stateRepositoryStub
	created      domainrss.FeedUpdate
	createCalls  int
	legacyCreate int
	legacyUpsert int
}

func (repo *atomicAddRepository) CreateSubscription(_ context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	repo.legacyCreate++
	return item, errors.New("legacy CreateSubscription must not be used by AddSubscription")
}

func (repo *atomicAddRepository) CreateFeed(_ context.Context, update domainrss.FeedUpdate) (domainrss.Subscription, domainrss.UpsertResult, error) {
	repo.createCalls++
	repo.created = update
	repo.listPage = domainrss.EntryPage{Items: update.Entries, Total: len(update.Entries)}
	created := update.Subscription
	created.UnreadCount = len(update.Entries)
	return created, domainrss.UpsertResult{Created: len(update.Entries)}, nil
}

func (repo *atomicAddRepository) UpsertFeed(context.Context, domainrss.FeedUpdate) (domainrss.UpsertResult, error) {
	repo.legacyUpsert++
	return domainrss.UpsertResult{}, errors.New("legacy UpsertFeed must not be used by AddSubscription")
}

func TestAddSubscriptionCreatesFeedOnceAndImmediatelyListsInitialEntries(t *testing.T) {
	repository := &atomicAddRepository{}
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/feed":
			response.Header().Set("Content-Type", "application/rss+xml")
			_, _ = response.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Atomic feed</title><link>http://source-a.example/</link><item><guid>post-1</guid><title>First post</title><link>http://source-a.example/posts/1</link></item></channel></rss>`))
		case "/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<html><head><link rel="icon" href="/icon.png"></head></html>`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer closeServer()
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	created, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: "http://source-a.example/feed", Title: "Saved title", ViewType: "article",
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}
	if repository.createCalls != 1 || repository.legacyCreate != 0 || repository.legacyUpsert != 0 {
		t.Fatalf("repository calls: CreateFeed=%d CreateSubscription=%d UpsertFeed=%d", repository.createCalls, repository.legacyCreate, repository.legacyUpsert)
	}
	if created.Title != "Saved title" || created.Revision != 1 || created.UnreadCount != 1 {
		t.Fatalf("created subscription = %#v", created)
	}
	if repository.created.Subscription.ID != created.ID || len(repository.created.Entries) != 1 ||
		repository.created.Entries[0].SubscriptionID != created.ID {
		t.Fatalf("atomic feed update = %#v", repository.created)
	}
	page, err := service.ListEntries(context.Background(), ListEntriesRequest{SubscriptionID: created.ID})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Title != "First post" {
		t.Fatalf("entries after add = %#v, error=%v", page, err)
	}
}
