package rss

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

const validatorTestFeed = `<?xml version="1.0"?><rss version="2.0"><channel><title>Validator Feed</title><link>https://site.example/</link><item><guid>post-1</guid><title>Post</title><link>https://site.example/post-1</link></item></channel></rss>`

type validatorRepository struct {
	*stateRepositoryStub
	subscription domainrss.Subscription
	updates      int
	upserts      int
}

func (repo *validatorRepository) GetSubscription(context.Context, string) (domainrss.Subscription, error) {
	return repo.subscription, nil
}

func (repo *validatorRepository) UpdateSubscription(_ context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	repo.subscription = item
	repo.updates++
	return item, nil
}

func (repo *validatorRepository) UpsertFeed(_ context.Context, update domainrss.FeedUpdate) (domainrss.UpsertResult, error) {
	repo.subscription = update.Subscription
	repo.upserts++
	return domainrss.UpsertResult{Created: len(update.Entries)}, nil
}

func newValidatorHTTPService(t *testing.T, repository domainrss.Repository, handler http.Handler) (*Service, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	dialAddress := strings.TrimPrefix(server.URL, "http://")
	dialer := &net.Dialer{}
	service := NewService(repository, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, dialAddress)
		},
	}}})
	service.resolver = rssStaticResolver{
		"source-a.example": {{IP: net.ParseIP("8.8.8.8")}},
		"source-b.example": {{IP: net.ParseIP("1.1.1.1")}},
		"mirror-a.example": {{IP: net.ParseIP("8.8.4.4")}},
		"mirror-b.example": {{IP: net.ParseIP("9.9.9.9")}},
	}
	return service, server.Close
}

func TestRSSRefreshPersistsAndReusesOnlyExactEffectiveURLValidators(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	repository := &validatorRepository{
		stateRepositoryStub: &stateRepositoryStub{},
		subscription: domainrss.Subscription{
			ID: "subscription-1", WorkspaceID: domainrss.DefaultWorkspaceID,
			FeedURL: "http://source-a.example/feed", Title: "Feed", ViewType: domainrss.ViewTypeAuto,
			Enabled: true, ETag: `"source-a-v1"`, ValidatorURL: "http://source-a.example/feed",
			CreatedAt: now, UpdatedAt: now, Revision: 1,
		},
	}
	var sourceARequests, sourceBRequests int
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.Host + request.URL.Path {
		case "source-a.example/feed":
			sourceARequests++
			if sourceARequests == 1 && request.Header.Get("If-None-Match") != `"source-a-v1"` {
				t.Errorf("initial source validator = %q", request.Header.Get("If-None-Match"))
			}
			response.Header().Set("Location", "http://source-b.example/final")
			response.WriteHeader(http.StatusFound)
		case "source-b.example/final":
			sourceBRequests++
			if sourceBRequests == 1 && request.Header.Get("If-None-Match") != "" {
				t.Errorf("redirect target received source A validator %q", request.Header.Get("If-None-Match"))
			}
			if request.Header.Get("If-None-Match") == `"source-b-v2"` {
				response.WriteHeader(http.StatusNotModified)
				return
			}
			response.Header().Set("Content-Type", "application/rss+xml")
			response.Header().Set("ETag", `"source-b-v2"`)
			_, _ = io.WriteString(response, validatorTestFeed)
		default:
			t.Errorf("unexpected request %s%s", request.Host, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer closeServer()
	service.now = func() time.Time { return now.Add(time.Minute) }

	if _, err := service.Refresh(context.Background(), RefreshRequest{ID: repository.subscription.ID}); err != nil {
		t.Fatal(err)
	}
	if repository.upserts != 1 || repository.subscription.ETag != `"source-b-v2"` ||
		repository.subscription.ValidatorURL != "http://source-b.example/final" {
		t.Fatalf("first refresh subscription = %#v, upserts=%d", repository.subscription, repository.upserts)
	}

	// Once the candidate itself is the effective URL, the validator is safe to
	// send and a matching 304 can update refresh timestamps without parsing.
	repository.subscription.FeedURL = "http://source-b.example/final"
	if _, err := service.Refresh(context.Background(), RefreshRequest{ID: repository.subscription.ID}); err != nil {
		t.Fatal(err)
	}
	if repository.updates != 1 || sourceBRequests != 2 || repository.subscription.ETag != `"source-b-v2"` {
		t.Fatalf("304 refresh updates=%d sourceB=%d subscription=%#v", repository.updates, sourceBRequests, repository.subscription)
	}
}

func TestRSSFeedValidatorProvenanceRejectsRedirectReturnAndScopesPathsAndMirrors(t *testing.T) {
	var loopVisits int
	service, closeServer := newValidatorHTTPService(t, nil, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.Host + request.URL.Path {
		case "source-a.example/loop":
			loopVisits++
			if loopVisits == 1 {
				if request.Header.Get("If-None-Match") != `"loop-v1"` {
					t.Errorf("initial loop validator = %q", request.Header.Get("If-None-Match"))
				}
				response.Header().Set("Location", "http://source-b.example/middle")
				response.WriteHeader(http.StatusFound)
				return
			}
			if request.Header.Get("If-None-Match") != "" {
				t.Errorf("returned source received stripped validator %q", request.Header.Get("If-None-Match"))
			}
			response.WriteHeader(http.StatusNotModified)
		case "source-b.example/middle":
			if request.Header.Get("If-None-Match") != "" {
				t.Errorf("middle redirect received validator %q", request.Header.Get("If-None-Match"))
			}
			response.Header().Set("Location", "http://source-a.example/loop")
			response.WriteHeader(http.StatusFound)
		case "source-a.example/new-path":
			if request.Header.Get("If-None-Match") != "" {
				t.Errorf("same-origin different path received validator %q", request.Header.Get("If-None-Match"))
			}
			response.Header().Set("Content-Type", "application/rss+xml")
			response.Header().Set("ETag", `"new-path"`)
			_, _ = io.WriteString(response, validatorTestFeed)
		case "mirror-a.example/route":
			if request.Header.Get("If-None-Match") != `"mirror-a"` {
				t.Errorf("matching mirror validator = %q", request.Header.Get("If-None-Match"))
			}
			response.WriteHeader(http.StatusServiceUnavailable)
		case "mirror-b.example/route":
			if request.Header.Get("If-None-Match") != "" {
				t.Errorf("fallback mirror received validator %q", request.Header.Get("If-None-Match"))
			}
			response.Header().Set("Content-Type", "application/rss+xml")
			response.Header().Set("ETag", `"mirror-b"`)
			_, _ = io.WriteString(response, validatorTestFeed)
		default:
			t.Errorf("unexpected request %s%s", request.Host, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer closeServer()

	if _, _, _, err := service.fetch(
		context.Background(), "http://source-a.example/loop", "http://source-a.example/loop", `"loop-v1"`, "",
	); err == nil || !strings.Contains(err.Error(), "unprovenanced HTTP 304") {
		t.Fatalf("redirect-return 304 error = %v", err)
	}
	_, _, pathMetadata, err := service.fetch(
		context.Background(), "http://source-a.example/new-path", "http://source-a.example/old-path", `"old-path"`, "",
	)
	if err != nil || pathMetadata.ValidatorURL != "http://source-a.example/new-path" || pathMetadata.ETag != `"new-path"` {
		t.Fatalf("path metadata=%#v error=%v", pathMetadata, err)
	}
	service.mirrors = []string{"http://mirror-a.example", "http://mirror-b.example"}
	_, mirrorMetadata, err := service.fetchAndParse(
		context.Background(), "rsshub://route", "http://mirror-a.example/route", `"mirror-a"`, "",
	)
	if err != nil || mirrorMetadata.ValidatorURL != "http://mirror-b.example/route" || mirrorMetadata.ETag != `"mirror-b"` {
		t.Fatalf("mirror metadata=%#v error=%v", mirrorMetadata, err)
	}
}

func TestRSSFeedValidatorMetadataIsBounded(t *testing.T) {
	service, closeServer := newValidatorHTTPService(t, nil, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("If-None-Match") != "" {
			t.Errorf("oversized stored validator was sent: %q", request.Header.Get("If-None-Match"))
		}
		response.Header().Set("Content-Type", "application/rss+xml")
		response.Header().Set("ETag", strings.Repeat("e", maxFeedValidatorBytes+1))
		_, _ = io.WriteString(response, validatorTestFeed)
	}))
	defer closeServer()
	_, _, metadata, err := service.fetch(
		context.Background(), "http://source-a.example/feed", "http://source-a.example/feed",
		strings.Repeat("v", maxFeedValidatorBytes+1), "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ETag != "" || metadata.LastModified != "" || metadata.ValidatorURL != "" {
		t.Fatalf("oversized response validators persisted: %#v", metadata)
	}
	if _, _, _, err := service.fetch(
		context.Background(), "http://source-a.example/"+strings.Repeat("x", maxFeedValidatorURLBytes), "", "", "",
	); err == nil {
		t.Fatal("oversized candidate feed URL was accepted")
	}
}

func TestRSSRefreshErrorsNeverPersistSourceURLOrSignedFeedQueries(t *testing.T) {
	const secret = "signed-refresh-secret"
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	repository := &validatorRepository{
		stateRepositoryStub: &stateRepositoryStub{},
		subscription: domainrss.Subscription{
			ID: "subscription-signed", WorkspaceID: domainrss.DefaultWorkspaceID,
			FeedURL: "http://source-a.example/feed?token=" + secret, Title: "Feed",
			ViewType: domainrss.ViewTypeAuto, Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
		},
	}
	service, closeServer := newValidatorHTTPService(t, repository, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadGateway)
	}))
	defer closeServer()
	_, err := service.Refresh(context.Background(), RefreshRequest{ID: repository.subscription.ID})
	if err == nil {
		t.Fatal("failed signed refresh unexpectedly succeeded")
	}
	for _, value := range []string{err.Error(), repository.subscription.LastError} {
		for _, forbidden := range []string{secret, "?token=", "source-a.example", "/feed", "http://"} {
			if strings.Contains(value, forbidden) {
				t.Fatalf("source detail %q leaked through refresh error %q", forbidden, value)
			}
		}
		if !strings.Contains(value, "<feed-ref:") || !strings.Contains(value, "HTTP 502") {
			t.Fatalf("safe refresh error lost status or opaque correlation: %q", value)
		}
	}
}
