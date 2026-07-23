package youtubeworkspace

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type innerTubeRequesterStub struct {
	mu        sync.Mutex
	calls     []innerTubeRequesterCall
	responses []map[string]any
	errors    []error
	userAgent string
	requestFn func(context.Context, string, map[string]any, innerTubeAuthPolicy) (map[string]any, error)
}

type innerTubeRequesterCall struct {
	endpoint   string
	body       map[string]any
	authPolicy innerTubeAuthPolicy
	locale     string
}

func (stub *innerTubeRequesterStub) requestRead(
	ctx context.Context,
	endpoint string,
	body map[string]any,
	authPolicy innerTubeAuthPolicy,
) (map[string]any, error) {
	copyBody := make(map[string]any, len(body))
	for key, value := range body {
		copyBody[key] = value
	}
	stub.mu.Lock()
	stub.calls = append(stub.calls, innerTubeRequesterCall{
		endpoint:   endpoint,
		body:       copyBody,
		authPolicy: authPolicy,
		locale:     innerTubeLocaleFromContext(ctx),
	})
	index := len(stub.calls) - 1
	var response map[string]any
	var responseErr error
	if index < len(stub.errors) {
		responseErr = stub.errors[index]
	}
	if index < len(stub.responses) {
		response = stub.responses[index]
	}
	requestFn := stub.requestFn
	stub.mu.Unlock()
	if requestFn != nil {
		return requestFn(ctx, endpoint, copyBody, authPolicy)
	}
	if responseErr != nil {
		return nil, responseErr
	}
	if response != nil {
		return response, nil
	}
	return map[string]any{}, nil
}

func (stub *innerTubeRequesterStub) setUserAgent(userAgent string) {
	stub.userAgent = userAgent
}

func (stub *innerTubeRequesterStub) snapshotCalls() []innerTubeRequesterCall {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return append([]innerTubeRequesterCall(nil), stub.calls...)
}

func newInnerTubeServiceForTest(stub *innerTubeRequesterStub) *Service {
	return &Service{
		requester:             stub,
		continuationEndpoints: make(map[string]continuationEndpointEntry),
	}
}

func TestBrowseRoutesUseExpectedInnerTubeEndpointBodyAuthAndFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		routeID      string
		endpoint     string
		browseID     string
		authPolicy   innerTubeAuthPolicy
		wantItemKind string
		wantVideoID  string
		wantPlaylist string
	}{
		{routeID: "home", endpoint: "browse", browseID: "FEwhat_to_watch", authPolicy: innerTubeAuthOptional, wantItemKind: "video", wantVideoID: "AbCdEfGh123"},
		{routeID: "subscriptions", endpoint: "browse", browseID: "FEsubscriptions", authPolicy: innerTubeAuthRequired, wantItemKind: "video", wantVideoID: "AbCdEfGh123"},
		{routeID: "explore", endpoint: "browse", browseID: "FEgaming_destination", authPolicy: innerTubeAuthOptional, wantItemKind: "video", wantVideoID: "AbCdEfGh123"},
		{routeID: "liked-videos", endpoint: "browse", browseID: "VLLL", authPolicy: innerTubeAuthRequired, wantItemKind: "video", wantVideoID: "AbCdEfGh123"},
		{routeID: "watch-later", endpoint: "browse", browseID: "VLWL", authPolicy: innerTubeAuthRequired, wantItemKind: "video", wantVideoID: "AbCdEfGh123"},
		{routeID: "history", endpoint: "browse", browseID: "FEhistory", authPolicy: innerTubeAuthRequired, wantItemKind: "video", wantVideoID: "AbCdEfGh123"},
		{routeID: "playlists", endpoint: "browse", browseID: "FEplaylist_aggregation", authPolicy: innerTubeAuthRequired, wantItemKind: "playlist", wantPlaylist: "PLworkspace123456"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.routeID, func(t *testing.T) {
			t.Parallel()
			stub := &innerTubeRequesterStub{responses: []map[string]any{innerTubeMixedResponse("next-token")}}
			service := newInnerTubeServiceForTest(stub)
			page, err := service.Browse(context.Background(), BrowseRequest{
				RouteID: test.routeID,
				Locale:  "zh-Hant-TW",
			})
			if err != nil {
				t.Fatalf("browse: %v", err)
			}
			if len(stub.calls) != 1 {
				t.Fatalf("expected one request, got %#v", stub.calls)
			}
			call := stub.calls[0]
			if call.endpoint != test.endpoint || call.body["browseId"] != test.browseID ||
				call.authPolicy != test.authPolicy || call.locale != "zh-TW" {
				t.Fatalf("unexpected request: %#v", call)
			}
			if page.Continuation != "next-token" || len(page.Items) != 1 {
				t.Fatalf("unexpected parsed page: %#v", page)
			}
			item := page.Items[0]
			if item.ItemKind != test.wantItemKind || item.VideoID != test.wantVideoID ||
				item.PlaylistID != test.wantPlaylist {
				t.Fatalf("unexpected filtered item: %#v", item)
			}
		})
	}
}

func TestBrowseSearchUsesAllItemFamiliesAndSearchContinuation(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{responses: []map[string]any{
		innerTubeMixedResponse("search-next"),
		innerTubeVideoResponse("ZyXwVuTs987", "Continuation video", "search-final", false),
	}}
	service := newInnerTubeServiceForTest(stub)

	first, err := service.Browse(context.Background(), BrowseRequest{
		RouteID: "search",
		Query:   "workspace design",
	})
	if err != nil {
		t.Fatalf("initial search: %v", err)
	}
	if len(first.Items) != 2 || first.Items[0].ItemKind != "video" || first.Items[1].ItemKind != "playlist" {
		t.Fatalf("search must retain videos and playlists: %#v", first.Items)
	}
	if got := stub.calls[0]; got.endpoint != "search" || got.body["query"] != "workspace design" ||
		got.authPolicy != innerTubeAuthOptional {
		t.Fatalf("unexpected initial search call: %#v", got)
	}

	second, err := service.Browse(context.Background(), BrowseRequest{
		RouteID:      "search",
		Query:        "workspace design",
		Continuation: first.Continuation,
	})
	if err != nil {
		t.Fatalf("search continuation: %v", err)
	}
	if got := stub.calls[1]; got.endpoint != "search" ||
		!reflect.DeepEqual(got.body, map[string]any{"continuation": "search-next"}) {
		t.Fatalf("unexpected continuation request: %#v", got)
	}
	if second.Continuation != "search-final" || len(second.Items) != 1 || second.Items[0].VideoID != "ZyXwVuTs987" {
		t.Fatalf("unexpected continuation page: %#v", second)
	}
}

func TestBrowseSearchWithoutQueryDoesNotCallInnerTube(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{}
	page, err := newInnerTubeServiceForTest(stub).Browse(
		context.Background(),
		BrowseRequest{RouteID: "search"},
	)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(stub.calls) != 0 || page.EmptyReason != "search_query_required" {
		t.Fatalf("unexpected empty search: calls=%#v page=%#v", stub.calls, page)
	}
}

func TestBrowseHomeContinuationUsesBrowseAndReturnsRawToken(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{responses: []map[string]any{
		innerTubeVideoResponse("AbCdEfGh123", "Home", "raw-token-1", false),
		innerTubeVideoResponse("ZyXwVuTs987", "More", "raw-token-2", false),
	}}
	service := newInnerTubeServiceForTest(stub)
	first, err := service.Browse(context.Background(), BrowseRequest{RouteID: "home"})
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	second, err := service.Browse(context.Background(), BrowseRequest{
		RouteID:      "home",
		Continuation: first.Continuation,
	})
	if err != nil {
		t.Fatalf("home continuation: %v", err)
	}
	if got := stub.calls[1]; got.endpoint != "browse" ||
		!reflect.DeepEqual(got.body, map[string]any{"continuation": "raw-token-1"}) {
		t.Fatalf("unexpected browse continuation: %#v", got)
	}
	if second.Continuation != "raw-token-2" {
		t.Fatalf("continuation token must stay raw, got %q", second.Continuation)
	}
}

func TestBrowsePreservesEveryItemBeforeServerContinuation(t *testing.T) {
	t.Parallel()
	contents := make([]any, 0, 31)
	for index := range 30 {
		videoID := fmt.Sprintf("%011d", index)
		contents = append(contents, innerTubeVideoRenderer(videoID, "Video "+videoID, false))
	}
	contents = append(contents, innerTubeContinuationRenderer("server-page-2"))
	stub := &innerTubeRequesterStub{responses: []map[string]any{{"contents": contents}}}

	page, err := newInnerTubeServiceForTest(stub).Browse(
		context.Background(),
		BrowseRequest{RouteID: "home"},
	)
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	if len(page.Items) != 30 || page.Continuation != "server-page-2" {
		t.Fatalf("server page was truncated before its continuation: items=%d continuation=%q", len(page.Items), page.Continuation)
	}
}

func TestBrowseHomeUsesPublicDestinationFallbackForEmptyGuestShell(t *testing.T) {
	t.Parallel()
	responses := guestHomeFallbackResponses()
	stub := &innerTubeRequesterStub{requestFn: func(
		_ context.Context,
		_ string,
		body map[string]any,
		_ innerTubeAuthPolicy,
	) (map[string]any, error) {
		browseID, _ := body["browseId"].(string)
		if browseID == "FEwhat_to_watch" {
			return map[string]any{}, nil
		}
		return responses[browseID], nil
	}}

	page, err := newInnerTubeServiceForTest(stub).Browse(
		context.Background(),
		BrowseRequest{RouteID: "home"},
	)
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	if len(page.Items) != 4 || page.Continuation != "" || page.EmptyReason != "" {
		t.Fatalf("unexpected guest fallback page: %#v", page)
	}
	for index, item := range page.Items {
		wantVideoID := fmt.Sprintf("%011d", index+1)
		if item.VideoID != wantVideoID {
			t.Fatalf("fallback item %d = %q, want %q", index, item.VideoID, wantVideoID)
		}
	}
	calls := stub.snapshotCalls()
	if len(calls) != 5 {
		t.Fatalf("guest home calls = %d, want initial home plus four destinations", len(calls))
	}
	seenBrowseIDs := make(map[string]int, len(calls))
	for _, call := range calls {
		if call.endpoint != "browse" || call.authPolicy != innerTubeAuthOptional {
			t.Fatalf("unexpected fallback call: %#v", call)
		}
		browseID, _ := call.body["browseId"].(string)
		seenBrowseIDs[browseID]++
	}
	for _, browseID := range append([]string{"FEwhat_to_watch"}, youtubeGuestHomeFallbackBrowseIDs...) {
		if seenBrowseIDs[browseID] != 1 {
			t.Fatalf("browse id %q called %d times; calls=%#v", browseID, seenBrowseIDs[browseID], calls)
		}
	}
}

func TestBrowseHomeFallbackIsBoundedConcurrentAndPreservesBrowseOrder(t *testing.T) {
	t.Parallel()
	started := make(chan string, len(youtubeGuestHomeFallbackBrowseIDs))
	release := make(chan struct{})
	responses := guestHomeFallbackResponses()
	stub := &innerTubeRequesterStub{requestFn: func(
		ctx context.Context,
		_ string,
		body map[string]any,
		_ innerTubeAuthPolicy,
	) (map[string]any, error) {
		browseID, _ := body["browseId"].(string)
		if browseID == "FEwhat_to_watch" {
			return map[string]any{}, nil
		}
		started <- browseID
		select {
		case <-release:
			return responses[browseID], nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	service := newInnerTubeServiceForTest(stub)
	service.guestHomeFallbackTimeout = time.Second

	type browseResult struct {
		page BrowsePage
		err  error
	}
	done := make(chan browseResult, 1)
	go func() {
		page, err := service.Browse(context.Background(), BrowseRequest{RouteID: "home"})
		done <- browseResult{page: page, err: err}
	}()

	for index := 0; index < youtubeGuestHomeFallbackConcurrency; index++ {
		select {
		case <-started:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("fallback requests did not begin concurrently")
		}
	}
	select {
	case browseID := <-started:
		t.Fatalf("fallback concurrency exceeded %d; %q also started", youtubeGuestHomeFallbackConcurrency, browseID)
	case <-time.After(40 * time.Millisecond):
	}
	close(release)

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("home fallback: %v", result.err)
		}
		if len(result.page.Items) != len(youtubeGuestHomeFallbackBrowseIDs) {
			t.Fatalf("fallback items = %#v", result.page.Items)
		}
		for index, item := range result.page.Items {
			wantVideoID := fmt.Sprintf("%011d", index+1)
			if item.VideoID != wantVideoID {
				t.Fatalf("fallback order[%d] = %q, want %q", index, item.VideoID, wantVideoID)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("fallback did not finish after delayed requests were released")
	}
}

func TestBrowseHomeFallbackToleratesOneDestinationFailure(t *testing.T) {
	t.Parallel()
	responses := guestHomeFallbackResponses()
	stub := &innerTubeRequesterStub{requestFn: func(
		_ context.Context,
		_ string,
		body map[string]any,
		_ innerTubeAuthPolicy,
	) (map[string]any, error) {
		browseID, _ := body["browseId"].(string)
		if browseID == "FEwhat_to_watch" {
			return map[string]any{}, nil
		}
		if browseID == "FEsports_destination" {
			return nil, errors.New("sports unavailable")
		}
		return responses[browseID], nil
	}}

	page, err := newInnerTubeServiceForTest(stub).Browse(
		context.Background(),
		BrowseRequest{RouteID: "home"},
	)
	if err != nil {
		t.Fatalf("one destination failure should be tolerated: %v", err)
	}
	if got := videoIDs(page.Items); !reflect.DeepEqual(got, []string{"00000000001", "00000000003", "00000000004"}) {
		t.Fatalf("unexpected fallback items after one failure: %#v", got)
	}
}

func TestBrowseHomeFallbackReturnsAtInternalDeadline(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{requestFn: func(
		ctx context.Context,
		_ string,
		body map[string]any,
		_ innerTubeAuthPolicy,
	) (map[string]any, error) {
		if body["browseId"] == "FEwhat_to_watch" {
			return map[string]any{}, nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	service := newInnerTubeServiceForTest(stub)
	service.guestHomeFallbackTimeout = 30 * time.Millisecond
	startedAt := time.Now()

	page, err := service.Browse(context.Background(), BrowseRequest{RouteID: "home"})
	if err != nil {
		t.Fatalf("internal fallback deadline should yield an empty page: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("fallback exceeded its total deadline: %v", elapsed)
	}
	if len(page.Items) != 0 || page.EmptyReason != "no_videos" {
		t.Fatalf("unexpected timed-out fallback page: %#v", page)
	}
}

func TestBrowseHomeFallbackPropagatesCallerCancellation(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 1)
	stub := &innerTubeRequesterStub{requestFn: func(
		ctx context.Context,
		_ string,
		body map[string]any,
		_ innerTubeAuthPolicy,
	) (map[string]any, error) {
		if body["browseId"] == "FEwhat_to_watch" {
			return map[string]any{}, nil
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := newInnerTubeServiceForTest(stub).Browse(ctx, BrowseRequest{RouteID: "home"})
		done <- err
	}()

	select {
	case <-started:
		cancel()
	case <-time.After(500 * time.Millisecond):
		cancel()
		t.Fatal("fallback did not start")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("caller cancellation = %v, want context.Canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("fallback did not stop after caller cancellation")
	}
}

func TestContinuationEndpointsExpireWithoutBreakingLiveTokens(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 11, 12, 0, 0, 0, time.UTC)
	service := newInnerTubeServiceForTest(&innerTubeRequesterStub{})
	service.continuationNow = func() time.Time { return now }

	service.rememberContinuationEndpoint("shorts", "live-token", "search")
	if got := service.continuationEndpoint("shorts", "live-token"); got != "search" {
		t.Fatalf("live continuation endpoint = %q, want search", got)
	}
	now = now.Add(youtubeContinuationEndpointTTL - time.Nanosecond)
	if got := service.continuationEndpoint("shorts", "live-token"); got != "search" {
		t.Fatalf("continuation expired before TTL: %q", got)
	}
	now = now.Add(time.Nanosecond)
	if got := service.continuationEndpoint("shorts", "live-token"); got != "" {
		t.Fatalf("expired continuation endpoint = %q, want empty", got)
	}
	if len(service.continuationEndpoints) != 0 {
		t.Fatalf("expired continuation was not removed: %#v", service.continuationEndpoints)
	}
}

func TestContinuationEndpointsStayWithinCapacityAndEvictOldest(t *testing.T) {
	t.Parallel()
	service := newInnerTubeServiceForTest(&innerTubeRequesterStub{})
	service.continuationNow = func() time.Time {
		return time.Date(2026, time.July, 11, 12, 0, 0, 0, time.UTC)
	}
	for index := 0; index < youtubeContinuationEndpointCapacity+8; index++ {
		service.rememberContinuationEndpoint("shorts", fmt.Sprintf("token-%04d", index), "search")
	}
	if got := len(service.continuationEndpoints); got != youtubeContinuationEndpointCapacity {
		t.Fatalf("continuation endpoint count = %d, want %d", got, youtubeContinuationEndpointCapacity)
	}
	if got := service.continuationEndpoint("shorts", "token-0000"); got != "" {
		t.Fatalf("oldest continuation endpoint was retained: %q", got)
	}
	latestToken := fmt.Sprintf("token-%04d", youtubeContinuationEndpointCapacity+7)
	if got := service.continuationEndpoint("shorts", latestToken); got != "search" {
		t.Fatalf("latest continuation endpoint = %q, want search", got)
	}
}

func TestBrowseShortsUsesHomeThenRealSearchFallback(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{responses: []map[string]any{
		innerTubeVideoResponse("AbCdEfGh123", "Regular video", "home-next", false),
		innerTubeVideoResponse("ZyXwVuTs987", "Short result", "shorts-search-next", true),
		innerTubeVideoResponse("QwErTyUi456", "More shorts", "", true),
	}}
	service := newInnerTubeServiceForTest(stub)
	first, err := service.Browse(context.Background(), BrowseRequest{RouteID: "shorts"})
	if err != nil {
		t.Fatalf("shorts: %v", err)
	}
	if len(stub.calls) != 2 || stub.calls[0].endpoint != "browse" ||
		stub.calls[0].body["browseId"] != "FEwhat_to_watch" ||
		stub.calls[1].endpoint != "search" || stub.calls[1].body["query"] != "#shorts" {
		t.Fatalf("unexpected shorts fallback calls: %#v", stub.calls)
	}
	if len(first.Items) != 1 || !first.Items[0].Short || first.Continuation != "shorts-search-next" {
		t.Fatalf("unexpected shorts fallback page: %#v", first)
	}

	second, err := service.Browse(context.Background(), BrowseRequest{
		RouteID:      "shorts",
		Continuation: first.Continuation,
	})
	if err != nil {
		t.Fatalf("shorts continuation: %v", err)
	}
	if got := stub.calls[2]; got.endpoint != "search" ||
		!reflect.DeepEqual(got.body, map[string]any{"continuation": "shorts-search-next"}) {
		t.Fatalf("fallback continuation must return to search: %#v", got)
	}
	if len(second.Items) != 1 || !second.Items[0].Short {
		t.Fatalf("unexpected shorts continuation: %#v", second)
	}
}

func TestBrowsePlaylistDetailUsesVLBrowseWithOptionalAuth(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{responses: []map[string]any{
		innerTubeMixedResponse("playlist-next"),
		innerTubeVideoResponse("ZyXwVuTs987", "More playlist video", "", false),
	}}
	service := newInnerTubeServiceForTest(stub)
	page, err := service.Browse(context.Background(), BrowseRequest{
		RouteID:    "playlists",
		PlaylistID: "PLworkspace123456",
	})
	if err != nil {
		t.Fatalf("playlist detail: %v", err)
	}
	if got := stub.calls[0]; got.endpoint != "browse" || got.body["browseId"] != "VLPLworkspace123456" ||
		got.authPolicy != innerTubeAuthOptional {
		t.Fatalf("unexpected playlist request: %#v", got)
	}
	if len(page.Items) != 1 || page.Items[0].ItemKind != "video" ||
		page.WebURL != "https://www.youtube.com/playlist?list=PLworkspace123456" {
		t.Fatalf("playlist detail must retain videos only: %#v", page)
	}

	continued, err := service.Browse(context.Background(), BrowseRequest{
		RouteID:      "playlists",
		PlaylistID:   "PLworkspace123456",
		Continuation: page.Continuation,
	})
	if err != nil {
		t.Fatalf("playlist continuation: %v", err)
	}
	if got := stub.calls[1]; got.endpoint != "browse" ||
		!reflect.DeepEqual(got.body, map[string]any{"continuation": "playlist-next"}) ||
		got.authPolicy != innerTubeAuthOptional {
		t.Fatalf("unexpected playlist continuation request: %#v", got)
	}
	if len(continued.Items) != 1 || continued.Items[0].VideoID != "ZyXwVuTs987" {
		t.Fatalf("unexpected playlist continuation page: %#v", continued)
	}
}

func TestBrowseRejectsInvalidPlaylistBeforeRequest(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{}
	_, err := newInnerTubeServiceForTest(stub).Browse(context.Background(), BrowseRequest{
		RouteID:    "playlists",
		PlaylistID: "invalid playlist!",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid youtube playlist id") {
		t.Fatalf("expected invalid playlist error, got %v", err)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("invalid playlist must not make a request: %#v", stub.calls)
	}
}

func TestBrowsePrivateAuthErrorsBecomeSignInPage(t *testing.T) {
	t.Parallel()
	for _, authErr := range []error{
		errYouTubeInnerTubeNotAuthenticated,
		errYouTubeInnerTubeAuthExpired,
	} {
		stub := &innerTubeRequesterStub{errors: []error{authErr}}
		page, err := newInnerTubeServiceForTest(stub).Browse(
			context.Background(),
			BrowseRequest{RouteID: "subscriptions"},
		)
		if err != nil {
			t.Fatalf("auth error should map to page: %v", err)
		}
		if !page.RequiresAuthentication || page.EmptyReason != "youtube_sign_in_required" || len(page.Items) != 0 {
			t.Fatalf("unexpected auth page: %#v", page)
		}
	}
}

func TestBrowseDoesNotManufactureKeywordFallbackOnFailure(t *testing.T) {
	t.Parallel()
	backendErr := errors.New("backend unavailable")
	stub := &innerTubeRequesterStub{errors: []error{backendErr}}
	_, err := newInnerTubeServiceForTest(stub).Browse(
		context.Background(),
		BrowseRequest{RouteID: "home"},
	)
	if !errors.Is(err, backendErr) {
		t.Fatalf("expected original backend error, got %v", err)
	}
	if len(stub.calls) != 1 {
		t.Fatalf("home failure must not trigger keyword fallback: %#v", stub.calls)
	}
}

func TestSetUserAgentForwardsToInnerTubeClient(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{}
	service := newInnerTubeServiceForTest(stub)
	service.SetUserAgent("Platform App Session UA")
	if stub.userAgent != "Platform App Session UA" {
		t.Fatalf("user agent was not forwarded: %q", stub.userAgent)
	}
}

func TestPreparePlaybackBuildsPlayerNeutralDescriptor(t *testing.T) {
	t.Parallel()
	service := NewService(nil, nil)
	descriptor, err := service.PreparePlayback(Video{
		VideoID:         "AbCdEfGh123",
		Title:           "  Demo Video  ",
		Channel:         "  Demo Channel  ",
		ChannelID:       "  UCabcdefghijklmnopqrstuv  ",
		ThumbnailURL:    "  https://example.com/thumb.jpg  ",
		DurationSeconds: 123,
		ViewCount:       4567,
		PublishedLabel:  "  2 days ago  ",
	})
	if err != nil {
		t.Fatalf("prepare playback: %v", err)
	}
	if descriptor.Source != "youtube" || descriptor.MediaKind != "video" ||
		descriptor.VideoID != "AbCdEfGh123" || descriptor.Title != "Demo Video" ||
		descriptor.Artist != "Demo Channel" || descriptor.ChannelID != "UCabcdefghijklmnopqrstuv" ||
		descriptor.ThumbnailURL != "https://example.com/thumb.jpg" ||
		descriptor.DurationSeconds != 123 || descriptor.ViewCount != 4567 ||
		descriptor.PublishedLabel != "2 days ago" ||
		descriptor.WebURL != "https://www.youtube.com/watch?v=AbCdEfGh123" {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
}

func TestPreparePlaybackRejectsInvalidVideo(t *testing.T) {
	t.Parallel()
	_, err := NewService(nil, nil).PreparePlayback(Video{VideoID: "invalid"})
	if err == nil {
		t.Fatal("expected invalid video id error")
	}
}

func guestHomeFallbackResponses() map[string]map[string]any {
	responses := make(map[string]map[string]any, len(youtubeGuestHomeFallbackBrowseIDs))
	for index, browseID := range youtubeGuestHomeFallbackBrowseIDs {
		videoID := fmt.Sprintf("%011d", index+1)
		responses[browseID] = innerTubeVideoResponse(videoID, browseID, "ignored-next", false)
	}
	return responses
}

func videoIDs(videos []Video) []string {
	result := make([]string, 0, len(videos))
	for _, video := range videos {
		result = append(result, video.VideoID)
	}
	return result
}

func innerTubeMixedResponse(continuation string) map[string]any {
	return map[string]any{
		"contents": []any{
			innerTubeVideoRenderer("AbCdEfGh123", "Video result", false),
			innerTubePlaylistRenderer("PLworkspace123456", "Playlist result"),
			innerTubeContinuationRenderer(continuation),
		},
	}
}

func innerTubeVideoResponse(videoID string, title string, continuation string, short bool) map[string]any {
	return map[string]any{
		"contents": []any{
			innerTubeVideoRenderer(videoID, title, short),
			innerTubeContinuationRenderer(continuation),
		},
	}
}

func innerTubeVideoRenderer(videoID string, title string, short bool) map[string]any {
	navigation := map[string]any{
		"watchEndpoint": map[string]any{"videoId": videoID},
	}
	if short {
		navigation = map[string]any{
			"reelWatchEndpoint": map[string]any{"videoId": videoID},
		}
	}
	return map[string]any{
		"videoRenderer": map[string]any{
			"videoId":            videoID,
			"title":              map[string]any{"simpleText": title},
			"navigationEndpoint": navigation,
			"lengthText":         map[string]any{"simpleText": "2:03"},
		},
	}
}

func innerTubePlaylistRenderer(playlistID string, title string) map[string]any {
	return map[string]any{
		"playlistRenderer": map[string]any{
			"playlistId": playlistID,
			"title":      map[string]any{"simpleText": title},
		},
	}
}

func innerTubeContinuationRenderer(token string) map[string]any {
	if strings.TrimSpace(token) == "" {
		return map[string]any{}
	}
	return map[string]any{
		"continuationItemRenderer": map[string]any{
			"continuationEndpoint": map[string]any{
				"continuationCommand": map[string]any{
					"token": token,
				},
			},
		},
	}
}
