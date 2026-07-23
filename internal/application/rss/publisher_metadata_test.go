package rss

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

type publisherMetadataRepository struct {
	*stateRepositoryStub
	subscription domainrss.Subscription
	entries      []domainrss.Entry
	updates      int
}

func (repo *publisherMetadataRepository) GetSubscription(_ context.Context, id string) (domainrss.Subscription, error) {
	if id != repo.subscription.ID {
		return domainrss.Subscription{}, domainrss.ErrNotFound
	}
	return repo.subscription, nil
}

func (repo *publisherMetadataRepository) UpdateSubscription(_ context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	repo.subscription = item
	repo.updates++
	return item, nil
}

func (repo *publisherMetadataRepository) ListEntries(_ context.Context, query domainrss.EntryQuery) (domainrss.EntryPage, error) {
	if query.SubscriptionID != repo.subscription.ID || query.Limit != maxRSSPublisherOriginSamples {
		return domainrss.EntryPage{}, fmt.Errorf("unexpected entry query: %#v", query)
	}
	items := append([]domainrss.Entry(nil), repo.entries...)
	if len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return domainrss.EntryPage{Items: items, Total: len(repo.entries)}, nil
}

func TestRSSPublisherSiteUsesStablePublicEntryOrigin(t *testing.T) {
	feed := enrichParsedFeedPublisherSite(parsedFeed{Entries: []parsedEntry{
		{URL: "https://sspai.com/post/1?utm_source=rss#comments"},
		{URL: "https://sspai.com/post/2"},
		{URL: "https://other.example/outlier"},
	}})
	if feed.SiteURL != "https://sspai.com/" {
		t.Fatalf("inferred publisher site = %q", feed.SiteURL)
	}

	for _, test := range []struct {
		name string
		urls []string
	}{
		{
			name: "tie is not stable",
			urls: []string{"https://one.example/post", "https://two.example/post"},
		},
		{
			name: "private entries are ignored",
			urls: []string{"http://127.0.0.1/post", "http://metadata.google.internal/latest"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := stableRSSPublisherOrigin(test.urls); got != "" {
				t.Fatalf("unstable/private origins produced site %q", got)
			}
		})
	}
}

func TestRSSFaviconDiscoveryUsesBoundedDeclaredHTMLIcon(t *testing.T) {
	requestedPaths := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestedPaths = append(requestedPaths, request.Host+request.URL.Path)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch request.Host {
		case "publisher.example":
			_, _ = io.WriteString(response, `<html><head>
				<base href="/static/">
				<link rel="apple-touch-icon" href="apple.png">
				<link rel="icon" type="image/svg+xml" href="vector.svg">
				<link rel="shortcut icon" type="image/png" href="favicon-64.png">
			</head></html>`)
		case "no-icon.example":
			_, _ = io.WriteString(response, `<html><head><title>No icon</title></head></html>`)
		case "oversized.example":
			_, _ = io.WriteString(response, strings.Repeat("x", maxRSSFaviconHTMLBytes+1))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	dialAddress := strings.TrimPrefix(server.URL, "http://")
	dialer := &net.Dialer{}
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, dialAddress)
		},
	}}})
	service.resolver = rssStaticResolver{
		"publisher.example": {{IP: net.ParseIP("8.8.8.8")}},
		"no-icon.example":   {{IP: net.ParseIP("1.1.1.1")}},
		"oversized.example": {{IP: net.ParseIP("9.9.9.9")}},
	}

	if got := service.discoverRSSFavicon(context.Background(), "http://publisher.example/articles?secret=value"); got != "http://publisher.example/static/favicon-64.png" {
		t.Fatalf("discovered favicon = %q", got)
	}
	if got := service.discoverRSSFavicon(context.Background(), "http://no-icon.example/"); got != "" {
		t.Fatalf("missing icon discovery = %q", got)
	}
	if got := service.discoverRSSFavicon(context.Background(), "http://oversized.example/"); got != "" {
		t.Fatalf("oversized HTML discovery = %q", got)
	}
	if got := service.discoverRSSFavicon(context.Background(), "http://127.0.0.1/private"); got != "" {
		t.Fatalf("private site discovery = %q", got)
	}
	for _, requestPath := range requestedPaths {
		if strings.HasSuffix(requestPath, "/favicon.ico") {
			t.Fatalf("favicon discovery guessed an undeclared path: %q", requestPath)
		}
	}
}

func TestRSSNotModifiedRefreshBackfillsExistingPublisherMetadata(t *testing.T) {
	for _, test := range []struct {
		name     string
		html     string
		wantIcon string
	}{
		{
			name:     "declared favicon",
			html:     `<html><head><link rel="icon" type="image/png" href="/assets/favicon.png"></head></html>`,
			wantIcon: "http://publisher.example/assets/favicon.png",
		},
		{
			name:     "no favicon remains empty",
			html:     `<html><head><title>Publisher</title></head></html>`,
			wantIcon: "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
			repository := &publisherMetadataRepository{
				stateRepositoryStub: &stateRepositoryStub{},
				subscription: domainrss.Subscription{
					ID: "subscription-1", WorkspaceID: domainrss.DefaultWorkspaceID,
					FeedURL: "http://source.example/feed", Title: "RSSHub Feed", ViewType: domainrss.ViewTypeAuto,
					Enabled: true, ETag: `"feed-v1"`, ValidatorURL: "http://source.example/feed",
					CreatedAt: now, UpdatedAt: now, Revision: 1,
				},
				entries: []domainrss.Entry{
					{ID: "entry-1", SubscriptionID: "subscription-1", URL: "http://publisher.example/posts/1?utm_source=rss"},
					{ID: "entry-2", SubscriptionID: "subscription-1", URL: "http://publisher.example/posts/2"},
				},
			}
			requestedPaths := make([]string, 0, 2)
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				requestedPaths = append(requestedPaths, request.Host+request.URL.Path)
				switch request.Host + request.URL.Path {
				case "source.example/feed":
					if got := request.Header.Get("If-None-Match"); got != `"feed-v1"` {
						t.Errorf("feed validator = %q", got)
					}
					response.WriteHeader(http.StatusNotModified)
				case "publisher.example/":
					response.Header().Set("Content-Type", "text/html")
					_, _ = io.WriteString(response, test.html)
				default:
					t.Errorf("unexpected metadata request %s%s", request.Host, request.URL.Path)
					response.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			dialAddress := strings.TrimPrefix(server.URL, "http://")
			dialer := &net.Dialer{}
			service := NewService(repository, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return dialer.DialContext(ctx, network, dialAddress)
				},
			}}})
			service.resolver = rssStaticResolver{
				"source.example":    {{IP: net.ParseIP("8.8.8.8")}},
				"publisher.example": {{IP: net.ParseIP("1.1.1.1")}},
			}
			service.now = func() time.Time { return now.Add(time.Minute) }

			if _, err := service.Refresh(context.Background(), RefreshRequest{ID: repository.subscription.ID}); err != nil {
				t.Fatal(err)
			}
			if repository.updates != 1 || repository.subscription.SiteURL != "http://publisher.example/" ||
				repository.subscription.IconURL != test.wantIcon {
				t.Fatalf("backfilled subscription = %#v, updates = %d", repository.subscription, repository.updates)
			}
			for _, requestPath := range requestedPaths {
				if strings.HasSuffix(requestPath, "/favicon.ico") {
					t.Fatalf("refresh guessed undeclared favicon path %q", requestPath)
				}
			}
		})
	}
}

func TestRSSSubscriptionDoesNotGuessFaviconPath(t *testing.T) {
	subscription := subscriptionFromParsedFeed(
		"subscription-1",
		"https://feeds.example/rss.xml",
		domainrss.ViewTypeAuto,
		parsedFeed{Title: "Feed", SiteURL: "https://publisher.example/"},
		time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC),
	)
	if subscription.IconURL != "" {
		t.Fatalf("subscription guessed favicon URL %q", subscription.IconURL)
	}
}
