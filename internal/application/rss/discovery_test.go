package rss

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

type discoveryRepositoryStub struct {
	stateRepositoryStub
	cache        domainrss.DiscoveryCache
	stateErr     error
	queryErr     error
	replaceErr   error
	replacements int
	queryCalls   int
}

func (stub *discoveryRepositoryStub) GetDiscoveryState(context.Context) (domainrss.DiscoveryState, error) {
	return domainrss.DiscoveryState{
		SourceURL: stub.cache.SourceURL, FetchedAt: stub.cache.FetchedAt, RouteCount: len(stub.cache.Routes),
	}, stub.stateErr
}

func (stub *discoveryRepositoryStub) QueryDiscovery(_ context.Context, query domainrss.DiscoveryQuery) (domainrss.DiscoveryPage, error) {
	stub.queryCalls++
	if stub.queryErr != nil {
		return domainrss.DiscoveryPage{}, stub.queryErr
	}
	cache := stub.cache
	if query.RouteID != "" {
		cache.Routes = slices.DeleteFunc(append([]domainrss.DiscoveryRoute(nil), cache.Routes...), func(route domainrss.DiscoveryRoute) bool {
			return route.ID != query.RouteID
		})
	}
	result := createDiscoveryResult(cache, DiscoveryRequest{
		Query: query.Query, CategoryID: query.CategoryID, Language: query.Language,
		Sort: query.Sort, Offset: query.Offset, Limit: query.Limit,
	})
	return domainrss.DiscoveryPage{
		State: domainrss.DiscoveryState{
			SourceURL: stub.cache.SourceURL, FetchedAt: stub.cache.FetchedAt,
			RouteCount: result.TotalRouteCount,
		},
		Categories: result.Categories, Routes: result.Routes,
		FilteredRouteCount: result.FilteredRouteCount, Offset: result.Offset, Limit: result.Limit,
		HasMore: result.HasMore,
	}, nil
}

func (stub *discoveryRepositoryStub) FindDiscoveryRoute(ctx context.Context, query domainrss.DiscoveryQuery) (domainrss.DiscoveryRoute, error) {
	query.Offset = 0
	query.Limit = 1
	page, err := stub.QueryDiscovery(ctx, query)
	if err != nil {
		return domainrss.DiscoveryRoute{}, err
	}
	if len(page.Routes) == 0 {
		return domainrss.DiscoveryRoute{}, domainrss.ErrNotFound
	}
	return page.Routes[0], nil
}

func (stub *discoveryRepositoryStub) ReplaceDiscoveryCache(_ context.Context, cache domainrss.DiscoveryCache) error {
	stub.replacements++
	if stub.replaceErr == nil {
		stub.cache = cache
	}
	return stub.replaceErr
}

func TestParseRSSHubBuildRoutesPreservesParameterizedRoutesAndMapsMetadata(t *testing.T) {
	body := []byte(`{
  "youtube": {
    "name": "YouTube",
    "url": "youtube.com",
    "categories": ["multimedia"],
    "lang": "en-US",
    "routes": {
      "user": {
        "path": "/user/:id",
        "example": "/youtube/user/@OpenAI",
        "name": "User videos",
        "description": "Latest **videos**.",
        "view": 120,
        "parameters": {
          "id": {
            "description": "Channel handle",
            "default": "featured",
            "options": [{"value": "featured", "label": "Featured"}]
          }
        },
        "features": {"requireConfig": true, "requirePuppeteer": false}
      },
      "missing-example": {
        "path": "/channel/:id",
        "name": "Channel",
        "parameters": {"id": "Channel ID"}
      }
    }
  },
  "bilibili": {
    "name": "哔哩哔哩",
    "url": "https://www.bilibili.com/",
    "categories": ["new-media"],
    "routes": {
      "user-video": {
        "path": "/user/video/:uid",
        "example": "/bilibili/user/video/123",
        "name": "用户投稿",
        "categories": ["multimedia"],
        "parameters": {"uid": "用户 UID"},
        "features": {"requirePuppeteer": true}
      }
    }
  },
  "blocked": {
    "name": "OnlyFans adult",
    "routes": {
      "user": {"path": "/user/:id", "example": "/blocked/user/123"}
    }
  }
}`)
	routes, err := parseRSSHubBuildRoutes(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 {
		t.Fatalf("routes = %#v", routes)
	}
	byPath := make(map[string]domainrss.DiscoveryRoute)
	for _, route := range routes {
		byPath[route.RoutePath] = route
		if route.Provider != "rsshub" || route.URL[:len(RSSHubScheme)] != RSSHubScheme {
			t.Fatalf("non-canonical route = %#v", route)
		}
	}
	youtube := byPath["youtube/user/:id"]
	if youtube.URL != "rsshub://youtube/user/:id" || youtube.RoutePath != "youtube/user/:id" ||
		youtube.ExamplePath != "youtube/user/@OpenAI" || youtube.ViewType != domainrss.ViewTypeVideo ||
		!youtube.RequiresConfig || youtube.RequiresPuppeteer || !youtube.NeedsParameters || len(youtube.Parameters) != 1 ||
		youtube.Parameters[0].Name != "id" || youtube.Parameters[0].ExampleValue != "@OpenAI" ||
		youtube.Parameters[0].DefaultValue == nil || *youtube.Parameters[0].DefaultValue != "featured" ||
		len(youtube.Parameters[0].Options) != 1 || youtube.Language != "en" || youtube.Region != "global" {
		t.Fatalf("youtube = %#v", youtube)
	}
	missingExample := byPath["youtube/channel/:id"]
	if !missingExample.NeedsParameters || missingExample.ExamplePath != "" ||
		len(missingExample.Parameters) != 1 || missingExample.Parameters[0].ExampleValue != "" {
		t.Fatalf("missing example route = %#v", missingExample)
	}
	bilibili := byPath["bilibili/user/video/:uid"]
	if bilibili.URL != "rsshub://bilibili/user/video/:uid" || !bilibili.RequiresPuppeteer ||
		!bilibili.NeedsParameters || bilibili.Parameters[0].ExampleValue != "123" ||
		bilibili.ViewType != domainrss.ViewTypeVideo || bilibili.Language != "zh-CN" || bilibili.Region != "CN" {
		t.Fatalf("bilibili = %#v", bilibili)
	}
}

func TestParseRSSHubBuildRoutesUsesArrayAliasKeysAndKeepsDistinctTemplates(t *testing.T) {
	body := []byte(`{
  "alias": {
    "name": "Alias",
    "routes": {
      "/user/:id": {"path": ["/user/:id", "/channel/:id"], "name": "User", "parameters": {"id": "ID"}},
      "/channel/:id": {"path": ["/user/:id", "/channel/:id"], "name": "Channel", "parameters": {"id": "ID"}},
      "/latest": {"path": "/latest", "name": "Latest"}
    }
  }
}`)
	routes, err := parseRSSHubBuildRoutes(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 {
		t.Fatalf("routes = %#v", routes)
	}
	want := map[string]bool{
		"alias/user/:id": false, "alias/channel/:id": false, "alias/latest": false,
	}
	ids := make(map[string]struct{})
	for _, route := range routes {
		if _, exists := want[route.RoutePath]; !exists {
			t.Fatalf("unexpected route path %q", route.RoutePath)
		}
		want[route.RoutePath] = true
		if _, duplicate := ids[route.ID]; duplicate {
			t.Fatalf("duplicate route ID %q", route.ID)
		}
		ids[route.ID] = struct{}{}
	}
	for routePath, found := range want {
		if !found {
			t.Fatalf("missing route %q", routePath)
		}
	}
}

func TestParseRSSHubBuildRoutesRejectsMaliciousStructuralBudgets(t *testing.T) {
	tests := []struct {
		name string
		body func() []byte
		want string
	}{
		{
			name: "depth",
			body: func() []byte {
				return []byte(strings.Repeat(`{"x":`, maxDiscoveryJSONDepth+1) + `0` + strings.Repeat(`}`, maxDiscoveryJSONDepth+1))
			},
			want: "depth",
		},
		{
			name: "raw string",
			body: func() []byte {
				return []byte(`{"source":{"name":"` + strings.Repeat("x", maxDiscoveryJSONStringBytes+1) + `","routes":{}}}`)
			},
			want: "string",
		},
		{
			name: "namespaces",
			body: func() []byte {
				var builder strings.Builder
				builder.WriteByte('{')
				for index := 0; index <= maxDiscoveryNamespaces; index++ {
					if index > 0 {
						builder.WriteByte(',')
					}
					fmt.Fprintf(&builder, `%q:null`, fmt.Sprintf("source-%04d", index))
				}
				builder.WriteByte('}')
				return []byte(builder.String())
			},
			want: "namespaces",
		},
		{
			name: "routes",
			body: func() []byte {
				var builder strings.Builder
				builder.WriteString(`{"source":{"routes":{`)
				for index := 0; index <= maxDiscoveryRoutes; index++ {
					if index > 0 {
						builder.WriteByte(',')
					}
					fmt.Fprintf(&builder, `%q:null`, fmt.Sprintf("/route-%05d", index))
				}
				builder.WriteString(`}}}`)
				return []byte(builder.String())
			},
			want: "routes",
		},
		{
			name: "tokens",
			body: func() []byte {
				var builder strings.Builder
				builder.WriteString(`{"source":{"junk":[`)
				for index := 0; index <= maxDiscoveryJSONTokens; index++ {
					if index > 0 {
						builder.WriteByte(',')
					}
					builder.WriteByte('0')
				}
				builder.WriteString(`],"routes":{}}}`)
				return []byte(builder.String())
			},
			want: "tokens",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseRSSHubBuildRoutes(test.body()); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q budget failure", err, test.want)
			}
		})
	}
}

func TestParseRSSHubBuildRoutesCapsPerRouteCollectionsDuringDecode(t *testing.T) {
	categories := make([]string, 0, maxDiscoveryCategories+8)
	parameters := make(map[string]any, maxDiscoveryParameters+8)
	for index := 0; index < maxDiscoveryCategories+8; index++ {
		categories = append(categories, fmt.Sprintf("category-%02d", index))
	}
	for index := 0; index < maxDiscoveryParameters+8; index++ {
		parameters[fmt.Sprintf("p%02d", index)] = "Parameter"
	}
	options := make([]any, 0, maxDiscoveryParameterOptions+12)
	for index := 0; index < maxDiscoveryParameterOptions+12; index++ {
		options = append(options, map[string]any{"value": fmt.Sprintf("v%d", index), "label": fmt.Sprintf("L%d", index)})
	}
	parameters["p00"] = map[string]any{"description": "Primary", "options": options}
	body, err := json.Marshal(map[string]any{
		"safe": map[string]any{
			"categories": categories,
			"routes": map[string]any{
				"/feed/:p00": map[string]any{"name": "Feed", "parameters": parameters},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	document, err := decodeRSSHubBuildDocument(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Namespaces) != 1 || len(document.Namespaces[0].Categories) != maxDiscoveryCategories ||
		len(document.Namespaces[0].Routes) != 1 || len(document.Namespaces[0].Routes[0].Parameters) != maxDiscoveryParameters {
		t.Fatalf("decoded collection budgets were not enforced: %#v", document)
	}
	var primary rssHubBuildParameter
	for _, parameter := range document.Namespaces[0].Routes[0].Parameters {
		if parameter.Name == "p00" {
			primary = parameter
			break
		}
	}
	if len(primary.Options) != maxDiscoveryParameterOptions {
		t.Fatalf("options = %d, want %d", len(primary.Options), maxDiscoveryParameterOptions)
	}
	routes, err := parseRSSHubBuildRoutes(body)
	if err != nil || len(routes) != 1 || len(routes[0].Categories) != maxDiscoveryCategories ||
		len(routes[0].Parameters) != 1 || len(routes[0].Parameters[0].Options) != maxDiscoveryParameterOptions {
		t.Fatalf("routes = %#v, err = %v", routes, err)
	}
}

func TestDiscoveryTemplateBudgetsAndMemoizedCatchAllMatching(t *testing.T) {
	segments := []string{"source"}
	for index := 0; index < maxDiscoveryCatchAllParameters; index++ {
		segments = append(segments, fmt.Sprintf(":p%d{.+}?", index))
	}
	segments = append(segments, "tail")
	routePath := strings.Join(segments, "/")
	example := []string{"source"}
	for index := 0; index < 50; index++ {
		example = append(example, "value")
	}
	example = append(example, "tail")
	started := time.Now()
	values := matchDiscoveryExample(routePath, strings.Join(example, "/"))
	if len(values) == 0 || time.Since(started) > time.Second {
		t.Fatalf("memoized catch-all match failed or was unexpectedly slow: values=%#v elapsed=%s", values, time.Since(started))
	}
	example[len(example)-1] = "not-tail"
	started = time.Now()
	if values := matchDiscoveryExample(routePath, strings.Join(example, "/")); len(values) != 0 || time.Since(started) > time.Second {
		t.Fatalf("memoized failed match = %#v elapsed=%s", values, time.Since(started))
	}
	tooManyCatchAlls := append([]string{"source"}, segments[1:len(segments)-1]...)
	tooManyCatchAlls = append(tooManyCatchAlls, ":extra{.+}", "tail")
	if normalized := normalizeDiscoveryRouteTemplate("", strings.Join(tooManyCatchAlls, "/")); normalized != "" {
		t.Fatalf("too many catch-alls normalized to %q", normalized)
	}
	tooManySegments := make([]string, maxDiscoveryTemplateSegments+1)
	for index := range tooManySegments {
		tooManySegments[index] = "segment"
	}
	if normalized := normalizeDiscoveryRouteTemplate("", strings.Join(tooManySegments, "/")); normalized != "" {
		t.Fatalf("too many segments normalized to %q", normalized)
	}
}

func TestParseRSSHubRoutesFeedFindsConcreteMirrorExample(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>RSSHub routes</title>
<item><guid>youtube/user/:id</guid><title>YouTube - User videos</title>
<link>https://docs.rsshub.app/routes/social-media</link>
<description>Example: https://rsshub.rssforever.com/youtube/user/@OpenAI</description></item>
<item><guid>github/repos/:user</guid><title>GitHub repos</title><description>No example</description></item>
</channel></rss>`)
	routes, err := parseRSSHubRoutesFeed(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes[0].URL != "rsshub://youtube/user/:id" ||
		routes[0].RoutePath != "youtube/user/:id" || !routes[0].NeedsParameters ||
		routes[0].Parameters[0].ExampleValue != "@OpenAI" || routes[0].ViewType != domainrss.ViewTypeVideo ||
		routes[1].RoutePath != "github/repos/:user" || routes[1].ExamplePath != "" || !routes[1].NeedsParameters {
		t.Fatalf("routes = %#v", routes)
	}
}

func TestDiscoveryParametersPreserveStructuredSchemaAndCatchAllExample(t *testing.T) {
	emptyDefault := ""
	raw := map[string]any{
		"email": "Mailbox address",
		"folder": map[string]any{
			"description": "Nested folder",
			"default":     emptyDefault,
			"options": []any{
				map[string]any{"value": "Inbox", "label": "Inbox"},
				map[string]any{"value": "Archive/2026", "label": "Archive"},
			},
		},
	}
	parameters := createDiscoveryParameters(
		"mail/imap/:email/:folder{.+}?", raw, "mail/imap/a%40example.com/Inbox/Sub",
	)
	if len(parameters) != 2 {
		t.Fatalf("parameters = %#v", parameters)
	}
	if email := parameters[0]; email.Name != "email" || email.Optional || email.CatchAll ||
		email.ExampleValue != "a@example.com" || email.Description != "Mailbox address" ||
		email.DefaultValue != nil || email.Type != "string" || len(email.Options) != 0 {
		t.Fatalf("email = %#v", email)
	}
	if folder := parameters[1]; folder.Name != "folder" || !folder.Optional || !folder.CatchAll ||
		folder.ExampleValue != "Inbox/Sub" || folder.DefaultValue == nil || *folder.DefaultValue != "" ||
		len(folder.Options) != 2 || folder.Options[1].Value != "Archive/2026" {
		t.Fatalf("folder = %#v", folder)
	}
}

func TestDiscoveryParameterExampleExcludesQueryString(t *testing.T) {
	parameters := createDiscoveryParameters(
		"nowcoder/hots/:type?", nil, "nowcoder/hots/1?limit=20",
	)
	if len(parameters) != 1 || parameters[0].Name != "type" ||
		parameters[0].ExampleValue != "1" || !parameters[0].Optional {
		t.Fatalf("parameters = %#v", parameters)
	}
}

func TestDiscoveryBareWildcardsBecomeNamedCatchAllParameters(t *testing.T) {
	routePath := normalizeDiscoveryRouteTemplate("example", "/*/*")
	if routePath != "example/:catchAll{.+}/:catchAll2{.+}" {
		t.Fatalf("routePath = %q", routePath)
	}
	parameters := createDiscoveryParameters(routePath, nil, "example/one/two")
	if len(parameters) != 2 || parameters[0].Name != "catchAll" || !parameters[0].CatchAll ||
		parameters[1].Name != "catchAll2" || !parameters[1].CatchAll {
		t.Fatalf("parameters = %#v", parameters)
	}
}

func TestCreateDiscoveryResultRepairsOldConcreteExampleCache(t *testing.T) {
	result := createDiscoveryResult(domainrss.DiscoveryCache{Routes: []domainrss.DiscoveryRoute{
		{
			ID: "legacy", Provider: "rsshub", Title: "YouTube", URL: "rsshub://youtube/user/@OpenAI",
			RoutePath: "youtube/user/:id", ExamplePath: "youtube/user/@OpenAI",
			Categories: []string{"multimedia"}, Language: "en", ViewType: domainrss.ViewTypeVideo,
		},
	}}, DiscoveryRequest{})
	if len(result.Routes) != 1 {
		t.Fatalf("result = %#v", result)
	}
	route := result.Routes[0]
	if route.URL != "rsshub://youtube/user/:id" || !route.NeedsParameters || len(route.Parameters) != 1 ||
		route.Parameters[0].Name != "id" || route.Parameters[0].ExampleValue != "@OpenAI" || route.ID == "legacy" {
		t.Fatalf("repaired route = %#v", route)
	}
}

func TestCreateDiscoveryResultMatchesAnySearchTokenAndRanksCompleteMatches(t *testing.T) {
	result := createDiscoveryResult(domainrss.DiscoveryCache{Routes: []domainrss.DiscoveryRoute{
		{ID: "complete", Title: "Bilibili creator video", URL: "rsshub://bilibili/user/video/:uid", RoutePath: "bilibili/user/video/:uid", Categories: []string{"multimedia"}, Heat: 20},
		{ID: "partial-video", Title: "Video archive", URL: "rsshub://archive/video", RoutePath: "archive/video", Categories: []string{"multimedia"}, Heat: 100},
		{ID: "partial-bili", Title: "Bilibili timeline", URL: "rsshub://bilibili/timeline", RoutePath: "bilibili/timeline", Categories: []string{"social-media"}, Heat: 90},
		{ID: "unrelated", Title: "Reading list", URL: "rsshub://reading/list", RoutePath: "reading/list", Categories: []string{"reading"}, Heat: 200},
	}}, DiscoveryRequest{Query: "bilibili video", Sort: "popular", Limit: 20})
	if result.FilteredRouteCount != 3 || len(result.Routes) != 3 {
		t.Fatalf("broad result = %#v", result)
	}
	if result.Routes[0].RoutePath != "bilibili/user/video/:uid" {
		t.Fatalf("ranked routes = %#v", result.Routes)
	}
}

func TestListDiscoveryUsesFreshCacheAndFiltersBeforePagination(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	repository := &discoveryRepositoryStub{cache: domainrss.DiscoveryCache{
		SourceURL: "https://example.com/routes.json", FetchedAt: now.Add(-time.Hour),
		Routes: []domainrss.DiscoveryRoute{
			{ID: "bili", Title: "Bilibili Videos", URL: "rsshub://bilibili/latest", RoutePath: "bilibili/latest", SourceName: "Bilibili", Categories: []string{"multimedia"}, Heat: 90, Language: "zh-CN"},
			{ID: "weibo", Title: "Weibo Timeline", URL: "rsshub://weibo/latest", RoutePath: "weibo/latest", SourceName: "Weibo", Categories: []string{"social-media"}, Heat: 70, Language: "zh-CN"},
			{ID: "youtube", Title: "YouTube Videos", URL: "rsshub://youtube/latest", RoutePath: "youtube/latest", SourceName: "YouTube", Categories: []string{"multimedia"}, Heat: 100, Language: "en"},
		},
	}}
	service := NewService(repository, nil)
	service.now = func() time.Time { return now }
	service.discoveryFetcher = func(context.Context) (domainrss.DiscoveryCache, error) {
		t.Fatal("fresh cache unexpectedly refreshed")
		return domainrss.DiscoveryCache{}, nil
	}
	result, err := service.ListDiscovery(context.Background(), DiscoveryRequest{
		Query: "bili", CategoryID: "multimedia", Language: "zh_cn", Sort: "title", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalRouteCount != 3 || result.FilteredRouteCount != 1 || len(result.Routes) != 1 ||
		result.Routes[0].RoutePath != "bilibili/latest" || result.HasMore || result.Limit != 1 || result.FetchedAt == "" {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Categories) != 3 || result.Categories[0].ID != "all" || result.Categories[0].Count != 2 {
		t.Fatalf("categories = %#v", result.Categories)
	}
	if repository.queryCalls != 1 {
		t.Fatalf("queryCalls = %d, want one paged repository query", repository.queryCalls)
	}
}

func TestDiscoveryResultFromPageNormalizesRoutesAndRestoresCategoryOrder(t *testing.T) {
	page := domainrss.DiscoveryPage{
		State: domainrss.DiscoveryState{RouteCount: 4, FetchedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)},
		Categories: []domainrss.DiscoveryCategory{
			{ID: "z-custom", Count: 2, Examples: []string{"Z", "Z2", "Z3"}},
			{ID: "multimedia", Count: 20, Examples: []string{"Video"}},
			{ID: "all", Count: 4, Examples: []string{"All"}},
			{ID: "a-custom", Count: 3, Examples: []string{"A"}},
			{ID: "social-media", Count: 1, Examples: []string{"Social"}},
		},
		Routes: []domainrss.DiscoveryRoute{{
			ID: "legacy", Provider: "rsshub", Title: "YouTube", URL: "rsshub://youtube/user/@OpenAI",
			RoutePath: "youtube/user/:id", ExamplePath: "youtube/user/@OpenAI",
			Categories: []string{"multimedia"}, Language: "en", ViewType: domainrss.ViewTypeVideo,
		}},
		FilteredRouteCount: 1, Limit: 1,
	}
	result := discoveryResultFromPage(page)
	wantOrder := []string{"all", "social-media", "multimedia", "a-custom", "z-custom"}
	if len(result.Categories) != len(wantOrder) {
		t.Fatalf("categories = %#v", result.Categories)
	}
	for index, want := range wantOrder {
		if result.Categories[index].ID != want {
			t.Fatalf("category[%d] = %q, want %q", index, result.Categories[index].ID, want)
		}
	}
	if len(result.Categories[len(result.Categories)-1].Examples) != 2 {
		t.Fatalf("examples were not bounded: %#v", result.Categories)
	}
	if len(result.Routes) != 1 || result.Routes[0].URL != "rsshub://youtube/user/:id" ||
		!result.Routes[0].NeedsParameters || result.Routes[0].ID == "legacy" {
		t.Fatalf("normalized routes = %#v", result.Routes)
	}
}

func TestListDiscoveryRefreshesExpiredCacheAndFallsBackToStaleOnFailure(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	staleRoute := domainrss.DiscoveryRoute{ID: "stale", Title: "Stale", URL: "rsshub://stale/example", RoutePath: "stale/example", Categories: []string{"other"}}
	repository := &discoveryRepositoryStub{cache: domainrss.DiscoveryCache{
		Routes: []domainrss.DiscoveryRoute{staleRoute}, SourceURL: "https://old.example/routes", FetchedAt: now.Add(-24 * time.Hour),
	}}
	service := NewService(repository, nil)
	service.now = func() time.Time { return now }
	var fetchCalls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	service.discoveryFetcher = func(context.Context) (domainrss.DiscoveryCache, error) {
		fetchCalls.Add(1)
		close(started)
		<-release
		return domainrss.DiscoveryCache{}, errors.New("offline")
	}
	startedAt := time.Now()
	result, err := service.ListDiscovery(context.Background(), DiscoveryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("stale cache response blocked for %s", elapsed)
	}
	if len(result.Routes) != 1 || result.Routes[0].RoutePath != "stale/example" || repository.replacements != 0 {
		t.Fatalf("result=%#v replacements=%d", result, repository.replacements)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not start")
	}
	if _, err := service.ListDiscovery(context.Background(), DiscoveryRequest{}); err != nil {
		t.Fatal(err)
	}
	if fetchCalls.Load() != 1 {
		t.Fatalf("concurrent stale reads started %d refreshes", fetchCalls.Load())
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for service.discoveryRefreshing.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if service.discoveryRefreshing.Load() || fetchCalls.Load() != 1 {
		t.Fatalf("background refresh state=%v calls=%d", service.discoveryRefreshing.Load(), fetchCalls.Load())
	}

	refreshedRoute := domainrss.DiscoveryRoute{ID: "fresh", Title: "Fresh", URL: "rsshub://fresh/example", RoutePath: "fresh/example", Categories: []string{"other"}}
	service.discoveryFetcher = func(context.Context) (domainrss.DiscoveryCache, error) {
		fetchCalls.Add(1)
		return domainrss.DiscoveryCache{Routes: []domainrss.DiscoveryRoute{refreshedRoute}, SourceURL: "https://new.example/routes", FetchedAt: now}, nil
	}
	result, err = service.ListDiscovery(context.Background(), DiscoveryRequest{ForceRefresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if fetchCalls.Load() != 2 || repository.replacements != 1 || len(result.Routes) != 1 || result.Routes[0].RoutePath != "fresh/example" {
		t.Fatalf("fetchCalls=%d result=%#v replacements=%d", fetchCalls.Load(), result, repository.replacements)
	}
}

func TestListDiscoveryBoundsEntireInitialRefresh(t *testing.T) {
	repository := &discoveryRepositoryStub{}
	service := NewService(repository, nil)
	service.discoveryTimeout = 20 * time.Millisecond
	service.discoveryFetcher = func(ctx context.Context) (domainrss.DiscoveryCache, error) {
		<-ctx.Done()
		return domainrss.DiscoveryCache{}, ctx.Err()
	}
	startedAt := time.Now()
	if _, err := service.ListDiscovery(context.Background(), DiscoveryRequest{}); err == nil {
		t.Fatal("timed out initial refresh unexpectedly succeeded")
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("refresh deadline was not global: %s", elapsed)
	}
}

func TestDiscoveryExternalURLsRejectCredentialsAndPrivateLiteralsAndUseFavicon(t *testing.T) {
	for _, unsafe := range []string{
		"http://rss:secret@public.example/path",
		"http://127.0.0.1/internal",
		"http://[::1]/internal",
		"http://metadata.google.internal/latest",
	} {
		if got := homepageURL(unsafe); got != "" {
			t.Fatalf("homepageURL(%q) = %q", unsafe, got)
		}
	}
	route := newDiscoveryRoute(discoveryRouteInput{
		SourceID: "safe", SourceName: "Safe", SourceURL: "https://example.com/docs/routes?from=rss#catalog",
		SiteURL: "http://127.0.0.1/private", RoutePath: "safe/example", ExamplePath: "safe/example",
		Title: "Safe", URL: "rsshub://safe/example", Categories: []string{"other"},
	})
	if route.SiteURL != "" || route.SourceURL != "https://example.com/docs/routes" ||
		route.IconURL != "https://example.com/favicon.ico" {
		t.Fatalf("sanitized route = %#v", route)
	}
	categories := createDiscoveryCategories([]domainrss.DiscoveryRoute{route})
	if len(categories) < 1 || categories[0].IconURL != "https://example.com/favicon.ico" ||
		categories[0].IconURL == route.SourceURL {
		t.Fatalf("categories = %#v", categories)
	}
}

func TestExplicitDiscoveryFilterDoesNotDropJavaScriptSources(t *testing.T) {
	javascript := domainrss.DiscoveryRoute{Title: "JavaScript Weekly", URL: "rsshub://javascript/weekly"}
	if explicitDiscoveryRoute(javascript) {
		t.Fatal("JavaScript source was treated as JAV content")
	}
	if !explicitDiscoveryRoute(domainrss.DiscoveryRoute{Title: "JAV videos", URL: "rsshub://example/jav"}) {
		t.Fatal("explicit token was not filtered")
	}
	for _, sourceID := range []string{
		"141jav", "141ppv", "91porn", "javlibrary", "missav", "projectjav",
		"rule34video", "xhamster",
	} {
		if !explicitDiscoveryRoute(domainrss.DiscoveryRoute{SourceID: sourceID, Title: "Catalog source"}) {
			t.Fatalf("explicit source %q was not filtered", sourceID)
		}
	}
}

func TestListDiscoveryFailsWhenInitialCatalogCannotBeFetched(t *testing.T) {
	repository := &discoveryRepositoryStub{}
	service := NewService(repository, nil)
	service.discoveryFetcher = func(context.Context) (domainrss.DiscoveryCache, error) {
		return domainrss.DiscoveryCache{}, errors.New("unavailable")
	}
	if _, err := service.ListDiscovery(context.Background(), DiscoveryRequest{}); err == nil {
		t.Fatal("empty unavailable discovery unexpectedly succeeded")
	}
}

func TestListDiscoverySurfacesExplicitForceRefreshFailure(t *testing.T) {
	repository := &discoveryRepositoryStub{cache: domainrss.DiscoveryCache{
		Routes:    []domainrss.DiscoveryRoute{{ID: "cached", Title: "Cached", URL: "rsshub://cached/example", RoutePath: "cached/example"}},
		FetchedAt: time.Now().UTC(),
	}}
	service := NewService(repository, nil)
	service.discoveryFetcher = func(context.Context) (domainrss.DiscoveryCache, error) {
		return domainrss.DiscoveryCache{}, errors.New("offline")
	}
	if _, err := service.ListDiscovery(context.Background(), DiscoveryRequest{ForceRefresh: true}); err == nil {
		t.Fatal("explicit force refresh failure was hidden")
	}
}
