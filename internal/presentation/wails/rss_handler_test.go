package wails

import (
	"context"
	"strconv"
	"strings"
	"testing"

	applicationrss "xiadown/internal/application/rss"
	domainrss "xiadown/internal/domain/rss"
)

type rssDiscoveryHandlerStub struct {
	rssService
	request applicationrss.DiscoveryRequest
	result  applicationrss.DiscoveryResult
}

type rssStateHandlerStub struct {
	rssService
	request applicationrss.SetEntryStateRequest
	result  domainrss.EntryState
}

type rssMarkAllReadHandlerStub struct {
	rssService
	request applicationrss.MarkAllReadRequest
	result  applicationrss.MarkAllReadResult
}

type rssHistoryHandlerStub struct {
	rssService
	request applicationrss.BackfillHistoryRequest
	result  applicationrss.BackfillHistoryResult
}

type rssEntriesHandlerStub struct {
	rssService
	page          domainrss.EntryPage
	entry         domainrss.Entry
	request       applicationrss.ListEntriesRequest
	subscriptions []domainrss.Subscription
	preview       applicationrss.PreviewSubscriptionResult
}

func (stub *rssEntriesHandlerStub) ListSubscriptions(context.Context) ([]domainrss.Subscription, error) {
	return stub.subscriptions, nil
}

func (stub *rssEntriesHandlerStub) PreviewSubscription(context.Context, applicationrss.PreviewSubscriptionRequest) (applicationrss.PreviewSubscriptionResult, error) {
	return stub.preview, nil
}

func (stub *rssEntriesHandlerStub) ListEntries(_ context.Context, request applicationrss.ListEntriesRequest) (domainrss.EntryPage, error) {
	stub.request = request
	return stub.page, nil
}

func (stub *rssEntriesHandlerStub) GetEntry(_ context.Context, _ applicationrss.SubscriptionRequest) (domainrss.Entry, error) {
	return stub.entry, nil
}

func (stub *rssStateHandlerStub) SetEntryState(_ context.Context, request applicationrss.SetEntryStateRequest) (domainrss.EntryState, error) {
	stub.request = request
	return stub.result, nil
}

func (stub *rssMarkAllReadHandlerStub) MarkAllRead(_ context.Context, request applicationrss.MarkAllReadRequest) (applicationrss.MarkAllReadResult, error) {
	stub.request = request
	return stub.result, nil
}

func (stub *rssHistoryHandlerStub) BackfillHistory(_ context.Context, request applicationrss.BackfillHistoryRequest) (applicationrss.BackfillHistoryResult, error) {
	stub.request = request
	return stub.result, nil
}

func (stub *rssDiscoveryHandlerStub) ListDiscovery(_ context.Context, request applicationrss.DiscoveryRequest) (applicationrss.DiscoveryResult, error) {
	stub.request = request
	return stub.result, nil
}

func TestRSSHandlerListDiscoveryForwardsStableContract(t *testing.T) {
	stub := &rssDiscoveryHandlerStub{result: applicationrss.DiscoveryResult{
		TotalRouteCount: 5, FilteredRouteCount: 2, Offset: 20, Limit: 20, HasMore: true,
	}}
	handler := NewRSSHandler(stub)
	result, err := handler.ListDiscovery(context.Background(), applicationrss.DiscoveryRequest{
		Query: "video", CategoryID: "multimedia", Language: "zh-CN", Sort: "title",
		Offset: 20, Limit: 20, ForceRefresh: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.request.Query != "video" || stub.request.CategoryID != "multimedia" ||
		stub.request.Language != "zh-CN" || stub.request.Sort != "title" ||
		stub.request.Offset != 20 || stub.request.Limit != 20 || !stub.request.ForceRefresh {
		t.Fatalf("request = %#v", stub.request)
	}
	if result.TotalRouteCount != 5 || result.FilteredRouteCount != 2 || !result.HasMore {
		t.Fatalf("result = %#v", result)
	}
}

func TestRSSHandlerUsesTaxonomyCategoryIconsAndProjectsDiscoverySourceIcons(t *testing.T) {
	const baseURL = "http://127.0.0.1:43127/_xiadown/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	stub := &rssDiscoveryHandlerStub{result: applicationrss.DiscoveryResult{
		Categories: []domainrss.DiscoveryCategory{{
			ID: "multimedia", IconURL: "https://catalog.example/favicon.ico",
		}},
		Routes: []domainrss.DiscoveryRoute{{
			ID: "rsshub:bilibili-ranking", IconURL: "https://bilibili.example/favicon.ico",
		}},
	}}
	result, err := NewRSSHandler(stub, baseURL).ListDiscovery(context.Background(), applicationrss.DiscoveryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	wantRoute := baseURL + "/api/rss/discovery/routes/rsshub:bilibili-ranking/icon"
	if len(result.Categories) != 1 || result.Categories[0].IconURL != "" ||
		len(result.Routes) != 1 || result.Routes[0].IconURL != wantRoute {
		t.Fatalf("projected discovery = %#v", result)
	}
	if strings.Contains(result.Routes[0].IconURL, "bilibili.example") {
		t.Fatalf("projected discovery leaked upstream URLs: %#v", result)
	}

	withoutProjection, err := NewRSSHandler(stub).ListDiscovery(context.Background(), applicationrss.DiscoveryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if withoutProjection.Categories[0].IconURL != "" || withoutProjection.Routes[0].IconURL != "" {
		t.Fatalf("handler without a resource base leaked icons: %#v", withoutProjection)
	}
}

func TestRSSHandlerSetEntryStateExposesDesktopV2Contract(t *testing.T) {
	revision := int64(2)
	starred := true
	stub := &rssStateHandlerStub{result: domainrss.EntryState{
		EntryID: "entry-1", Starred: true, FieldRevisions: domainrss.StateFieldRevisions{Starred: 3}, Revision: 5,
	}}
	handler := NewRSSHandler(stub)
	result, err := handler.SetEntryState(context.Background(), applicationrss.SetEntryStateRequest{
		ID: "entry-1", Field: domainrss.EntryStateFieldStarred, Starred: &starred,
		ExpectedRevision: &revision, MutationID: "desktop-star-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.request.ID != "entry-1" || stub.request.Field != domainrss.EntryStateFieldStarred ||
		stub.request.Starred == nil || !*stub.request.Starred || stub.request.ExpectedRevision == nil ||
		*stub.request.ExpectedRevision != 2 || stub.request.MutationID != "desktop-star-1" {
		t.Fatalf("request = %#v", stub.request)
	}
	if !result.Starred || result.FieldRevisions.Starred != 3 || result.Revision != 5 {
		t.Fatalf("result = %#v", result)
	}
}

func TestRSSHandlerMarkAllReadForwardsCollectionScope(t *testing.T) {
	stub := &rssMarkAllReadHandlerStub{result: applicationrss.MarkAllReadResult{Updated: 731}}
	handler := NewRSSHandler(stub)
	result, err := handler.MarkAllRead(context.Background(), applicationrss.MarkAllReadRequest{
		SubscriptionID: "subscription-1", Kind: "article", StarredOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.request.SubscriptionID != "subscription-1" || stub.request.Kind != "article" || !stub.request.StarredOnly {
		t.Fatalf("request = %#v", stub.request)
	}
	if result.Updated != 731 {
		t.Fatalf("result = %#v", result)
	}
}

func TestRSSHandlerBackfillHistoryForwardsSourceAndKindScope(t *testing.T) {
	stub := &rssHistoryHandlerStub{result: applicationrss.BackfillHistoryResult{
		Subscriptions: 1, Attempted: 1, Supported: 1, Created: 12,
	}}
	handler := NewRSSHandler(stub)
	result, err := handler.BackfillHistory(context.Background(), applicationrss.BackfillHistoryRequest{
		SubscriptionID: "subscription-1", Kind: "video",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.request.SubscriptionID != "subscription-1" || stub.request.Kind != "video" {
		t.Fatalf("request = %#v", stub.request)
	}
	if result.Subscriptions != 1 || result.Attempted != 1 || result.Supported != 1 || result.Created != 12 {
		t.Fatalf("result = %#v", result)
	}
}

func TestRSSHandlerListEntriesDefersFullBodyUntilDetailHydration(t *testing.T) {
	stub := &rssEntriesHandlerStub{
		page: domainrss.EntryPage{Items: []domainrss.Entry{{
			ID: "entry-1", Title: "Entry", Summary: "Bounded summary",
			ContentHTML: "<article>large body</article>",
		}}},
		entry: domainrss.Entry{
			ID: "entry-1", Title: "Entry", Summary: "Bounded summary",
			ContentHTML: "<article>large body</article>",
		},
	}
	handler := NewRSSHandler(stub)

	page, err := handler.ListEntries(context.Background(), applicationrss.ListEntriesRequest{Limit: 80})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ContentHTML != "" || page.Items[0].Summary != "Bounded summary" {
		t.Fatalf("lightweight page = %#v", page)
	}

	detail, err := handler.GetEntry(context.Background(), applicationrss.SubscriptionRequest{ID: "entry-1"})
	if err != nil {
		t.Fatal(err)
	}
	if detail.ContentHTML != "<article>large body</article>" {
		t.Fatalf("hydrated detail = %#v", detail)
	}
}

func TestRSSHandlerListEntriesPreservesMixedKindsAndSnapshotForAll(t *testing.T) {
	stub := &rssEntriesHandlerStub{page: domainrss.EntryPage{
		Items: []domainrss.Entry{
			{ID: "entry-article", Kind: domainrss.EntryKindArticle, Title: "Article"},
			{ID: "entry-video", Kind: domainrss.EntryKindVideo, Title: "Video"},
		},
		Total: 2, Snapshot: 17,
	}}
	handler := NewRSSHandler(stub)
	page, err := handler.ListEntries(context.Background(), applicationrss.ListEntriesRequest{Limit: 80})
	if err != nil {
		t.Fatal(err)
	}
	if stub.request.Kind != "" || stub.request.Limit != 80 {
		t.Fatalf("all request = %#v", stub.request)
	}
	if page.Total != 2 || page.Snapshot != 17 || len(page.Items) != 2 ||
		page.Items[0].Kind != domainrss.EntryKindArticle || page.Items[1].Kind != domainrss.EntryKindVideo {
		t.Fatalf("mixed Wails page = %#v", page)
	}
}

func TestRSSHandlerProjectsPersistedResourcesAndDropsUnmappedReaderSources(t *testing.T) {
	const baseURL = "http://127.0.0.1:43127/_xiadown/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	stub := &rssEntriesHandlerStub{
		subscriptions: []domainrss.Subscription{{
			ID: "subscription-1", IconURL: "https://cdn.example/icon.png",
		}},
		entry: domainrss.Entry{
			ID: "entry-1", SubscriptionID: "subscription-1", URL: "https://news.example/article",
			ThumbnailURL: "https://cdn.example/cover.jpg",
			ImageURLs:    []string{"https://cdn.example/body.jpg"},
			MediaURL:     "https://cdn.example/video.mp4",
			Media: []domainrss.Media{{
				URL: "https://cdn.example/video.mp4", Kind: "video", MIMEType: "video/mp4",
				Thumbnail: "https://cdn.example/poster.jpg",
			}},
			ContentHTML: `<article><img src="https://cdn.example/body.jpg"><img src="https://unmapped.example/tracker.png"><video src="https://cdn.example/video.mp4" poster="https://cdn.example/poster.jpg"><source src="https://unmapped.example/track.mp4"></video></article>`,
		},
	}
	handler := NewRSSHandler(stub, baseURL)

	subscriptions, err := handler.ListSubscriptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantIcon := baseURL + "/api/rss/subscriptions/subscription-1/icon"
	if len(subscriptions) != 1 || subscriptions[0].IconURL != wantIcon {
		t.Fatalf("subscriptions = %#v", subscriptions)
	}

	detail, err := handler.GetEntry(context.Background(), applicationrss.SubscriptionRequest{ID: "entry-1"})
	if err != nil {
		t.Fatal(err)
	}
	if detail.URL != "https://news.example/article" {
		t.Fatalf("source link changed: %q", detail.URL)
	}
	if detail.ThumbnailURL != baseURL+"/api/rss/entries/entry-1/resources/thumbnail" ||
		len(detail.ImageURLs) != 1 || detail.ImageURLs[0] != baseURL+"/api/rss/entries/entry-1/resources/image-0" ||
		len(detail.Media) != 1 || detail.Media[0].URL != baseURL+"/api/rss/entries/entry-1/resources/media-0" ||
		detail.Media[0].Thumbnail != baseURL+"/api/rss/entries/entry-1/resources/media-0-thumbnail" ||
		detail.MediaURL != baseURL+"/api/rss/entries/entry-1/resources/media-0" {
		t.Fatalf("projected detail = %#v", detail)
	}
	for _, forbidden := range []string{"cdn.example", "unmapped.example"} {
		if strings.Contains(detail.ContentHTML, forbidden) {
			t.Fatalf("reader retained remote source %q: %s", forbidden, detail.ContentHTML)
		}
	}
	for _, expected := range []string{
		baseURL + "/api/rss/entries/entry-1/resources/image-0",
		baseURL + "/api/rss/entries/entry-1/resources/media-0",
		baseURL + "/api/rss/entries/entry-1/resources/media-0-thumbnail",
	} {
		if !strings.Contains(detail.ContentHTML, expected) {
			t.Fatalf("reader missing projected source %q: %s", expected, detail.ContentHTML)
		}
	}
	if !strings.Contains(detail.ContentHTML, `loading="lazy"`) || !strings.Contains(detail.ContentHTML, `decoding="async"`) {
		t.Fatalf("reader image is not decode-lazy: %s", detail.ContentHTML)
	}
}

func TestRSSHandlerReusesContentImageProjectionForDuplicateThumbnailSlots(t *testing.T) {
	const baseURL = "http://127.0.0.1:43127/_xiadown/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const imageURL = "https://cdn.example/photo.jpg"
	stub := &rssEntriesHandlerStub{entry: domainrss.Entry{
		ID:           "entry-1",
		ThumbnailURL: imageURL,
		ImageURLs:    []string{imageURL},
		Media: []domainrss.Media{{
			URL: imageURL, Kind: "image", Thumbnail: imageURL,
		}},
		ContentHTML: `<figure><img src="https://cdn.example/photo.jpg"></figure>`,
	}}

	detail, err := NewRSSHandler(stub, baseURL).GetEntry(
		context.Background(), applicationrss.SubscriptionRequest{ID: "entry-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := baseURL + "/api/rss/entries/entry-1/resources/image-0"
	if detail.ThumbnailURL != want || len(detail.ImageURLs) != 1 || detail.ImageURLs[0] != want ||
		len(detail.Media) != 1 || detail.Media[0].URL != want || detail.Media[0].Thumbnail != want {
		t.Fatalf("duplicate image projections = %#v", detail)
	}
	if strings.Count(detail.ContentHTML, want) != 1 || strings.Contains(detail.ContentHTML, "/resources/thumbnail") {
		t.Fatalf("reader did not reuse the content-image slot: %s", detail.ContentHTML)
	}
}

func TestRSSHandlerKeepsUnpersistedPreviewTextOnly(t *testing.T) {
	stub := &rssEntriesHandlerStub{preview: applicationrss.PreviewSubscriptionResult{
		Subscription: domainrss.Subscription{ID: "rss-preview", IconURL: "https://cdn.example/icon.png"},
		Entries: []domainrss.Entry{{
			ID: "rss-preview-entry", ThumbnailURL: "https://cdn.example/cover.jpg",
			ImageURLs:       []string{"https://cdn.example/body.jpg"},
			Media:           []domainrss.Media{{URL: "https://cdn.example/video.mp4", Kind: "video"}},
			Platform:        "youtube",
			PlatformVideoID: "AbCdEfGhI12",
			PlaybackURL:     "https://www.youtube-nocookie.com/embed/AbCdEfGhI12",
			ContentHTML:     `<p>Preview<img src="https://cdn.example/body.jpg"></p><figure data-xiadown-rss-video-provider="youtube" data-xiadown-rss-video-id="AbCdEfGhI12"></figure>`,
		}},
	}}
	result, err := NewRSSHandler(stub, "http://127.0.0.1:43127").PreviewSubscription(
		context.Background(), applicationrss.PreviewSubscriptionRequest{URL: "https://feed.example/rss"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Subscription.IconURL != "" || len(result.Entries) != 1 ||
		result.Entries[0].ThumbnailURL != "" || len(result.Entries[0].ImageURLs) != 0 ||
		len(result.Entries[0].Media) != 0 || strings.Contains(result.Entries[0].ContentHTML, "<img") ||
		strings.Contains(result.Entries[0].ContentHTML, "data-xiadown-rss-video-provider") ||
		result.Entries[0].Platform != "" || result.Entries[0].PlatformVideoID != "" ||
		result.Entries[0].PlaybackURL != "" ||
		!strings.Contains(result.Entries[0].ContentHTML, "Preview") {
		t.Fatalf("preview resources were not stripped: %#v", result)
	}
}

func TestRSSHandlerVersionsDesktopResourcesByEntityRevision(t *testing.T) {
	const baseURL = "http://127.0.0.1:43127/_xiadown/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	stub := &rssEntriesHandlerStub{
		subscriptions: []domainrss.Subscription{{
			ID: "subscription-1", IconURL: "https://cdn.example/icon.png", Revision: 4,
		}},
		entry: domainrss.Entry{
			ID: "entry-1", ThumbnailURL: "https://cdn.example/cover.jpg",
			ImageURLs: []string{"https://cdn.example/body.jpg"}, Revision: 9,
		},
	}
	handler := NewRSSHandler(stub, baseURL)

	subscriptions, err := handler.ListSubscriptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	detail, err := handler.GetEntry(
		context.Background(),
		applicationrss.SubscriptionRequest{ID: "entry-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscriptions[0].IconURL != baseURL+"/api/rss/subscriptions/subscription-1/icon?v=4" ||
		detail.ThumbnailURL != baseURL+"/api/rss/entries/entry-1/resources/thumbnail?v=9" ||
		len(detail.ImageURLs) != 1 ||
		detail.ImageURLs[0] != baseURL+"/api/rss/entries/entry-1/resources/image-0?v=9" {
		t.Fatalf("versioned resources: subscription=%#v entry=%#v", subscriptions[0], detail)
	}
}

func TestRSSHandlerProjectsOnlyResolvableIndexedResourceSlots(t *testing.T) {
	const baseURL = "http://127.0.0.1:43127/_xiadown/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	images := make([]string, 65)
	media := make([]domainrss.Media, 65)
	for index := range images {
		images[index] = "https://cdn.example/image-" + strconv.Itoa(index) + ".jpg"
		media[index] = domainrss.Media{
			URL:       "https://cdn.example/media-" + strconv.Itoa(index) + ".mp4",
			Kind:      "video",
			Thumbnail: "https://cdn.example/poster-" + strconv.Itoa(index) + ".jpg",
		}
	}
	stub := &rssEntriesHandlerStub{entry: domainrss.Entry{
		ID:        "entry-1",
		ImageURLs: images,
		Media:     media,
		MediaURL:  media[64].URL,
		ContentHTML: `<article><img src="https://cdn.example/image-63.jpg"><img src="https://cdn.example/image-64.jpg">` +
			`<video src="https://cdn.example/media-63.mp4"></video><video src="https://cdn.example/media-64.mp4"></video></article>`,
	}}

	detail, err := NewRSSHandler(stub, baseURL).GetEntry(
		context.Background(), applicationrss.SubscriptionRequest{ID: "entry-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.ImageURLs) != 64 || len(detail.Media) != 64 {
		t.Fatalf("projected indexed resource counts = images %d, media %d", len(detail.ImageURLs), len(detail.Media))
	}
	if detail.MediaURL != "" {
		t.Fatalf("out-of-range primary media URL was projected: %q", detail.MediaURL)
	}
	for _, forbidden := range []string{"resources/image-64", "resources/media-64", "cdn.example/image-64", "cdn.example/media-64"} {
		if strings.Contains(detail.ContentHTML, forbidden) {
			t.Fatalf("reader retained out-of-range resource %q: %s", forbidden, detail.ContentHTML)
		}
	}
	for _, expected := range []string{"resources/image-63", "resources/media-63"} {
		if !strings.Contains(detail.ContentHTML, expected) {
			t.Fatalf("reader omitted in-range resource %q: %s", expected, detail.ContentHTML)
		}
	}
}
