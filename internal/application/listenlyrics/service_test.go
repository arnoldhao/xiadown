package listenlyrics

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

type fakeClient struct {
	requests []Request
	results  []Snapshot
	err      error
}

func (client *fakeClient) TrackLyrics(_ context.Context, request Request) (Snapshot, error) {
	client.requests = append(client.requests, request)
	if len(client.results) == 0 {
		return Snapshot{VideoID: request.VideoID, Kind: KindUnavailable}, client.err
	}
	result := client.results[0]
	client.results = client.results[1:]
	return result, client.err
}

func TestTrackLyricsCachesSyncedResult(t *testing.T) {
	client := &fakeClient{
		results: []Snapshot{{
			VideoID: "video-one",
			Kind:    KindSynced,
			Source:  "YTMusic",
			Lines:   []Line{{StartMs: 100, Text: "hello"}},
		}},
	}
	service := NewService(client)

	first, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
	if err != nil {
		t.Fatal(err)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected synced lyrics to be served from cache, got %d requests", len(client.requests))
	}
	if first.Kind != KindSynced || second.Kind != KindSynced || len(second.Lines) != 1 {
		t.Fatalf("unexpected synced lyrics results: first=%#v second=%#v", first, second)
	}
}

func TestNormalizeRequestBoundsAndDeduplicatesSearchVariants(t *testing.T) {
	request := normalizeRequest(Request{
		Title:  "Canonical",
		Artist: "Artist",
		SearchVariants: []SearchVariant{
			{Title: "  Canonical ", Artist: " Artist "},
			{Title: "繁  體", Artist: "歌  手"},
			{Title: "繁 體", Artist: "歌 手"},
			{Title: "Variant Two", Artist: "Artist"},
			{Title: "Variant Three", Artist: "Artist"},
			{Title: "Ignored", Artist: "Artist"},
		},
	})

	want := []SearchVariant{
		{Title: "繁 體", Artist: "歌 手"},
		{Title: "Variant Two", Artist: "Artist"},
		{Title: "Variant Three", Artist: "Artist"},
	}
	if len(request.SearchVariants) != len(want) {
		t.Fatalf("unexpected normalized variants: %#v", request.SearchVariants)
	}
	for index := range want {
		if request.SearchVariants[index] != want[index] {
			t.Fatalf("variant %d mismatch: got %#v want %#v", index, request.SearchVariants[index], want[index])
		}
	}
}

func TestTrackLyricsCanonicalCacheIgnoresSearchVariants(t *testing.T) {
	client := &fakeClient{results: []Snapshot{{
		VideoID: "video-one",
		Kind:    KindSynced,
		Lines:   []Line{{StartMs: 100, Text: "cached"}},
	}}}
	service := NewService(client)

	canonical := Request{VideoID: "video-one", Title: "后来", Artist: "刘若英"}
	if _, err := service.TrackLyrics(context.Background(), canonical); err != nil {
		t.Fatal(err)
	}
	canonical.SearchVariants = []SearchVariant{{Title: "後來", Artist: "劉若英"}}
	if _, err := service.TrackLyrics(context.Background(), canonical); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("search variants changed canonical cache identity: %d requests", len(client.requests))
	}
}

func TestTrackLyricsNegativeCacheSeparatesSearchVariants(t *testing.T) {
	client := &fakeClient{results: []Snapshot{
		{VideoID: "video-one", Kind: KindUnavailable},
		{
			VideoID: "video-one",
			Kind:    KindSynced,
			Lines:   []Line{{StartMs: 100, Text: "variant match"}},
		},
	}}
	service := NewService(client)
	request := Request{VideoID: "video-one", Title: "后来", Artist: "刘若英"}

	first, err := service.TrackLyrics(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind != KindUnavailable {
		t.Fatalf("expected canonical miss, got %#v", first)
	}
	second, err := service.TrackLyrics(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.Kind != KindUnavailable || len(client.requests) != 1 {
		t.Fatalf("equivalent request did not reuse its negative cache: requests=%d result=%#v", len(client.requests), second)
	}

	request.SearchVariants = []SearchVariant{{Title: " 後來 ", Artist: " 劉若英 "}}
	matched, err := service.TrackLyrics(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if matched.Kind != KindSynced || len(client.requests) != 2 {
		t.Fatalf("variant-aware request was suppressed by canonical miss: requests=%d result=%#v", len(client.requests), matched)
	}
}

func TestTrackLyricsKeepsCachedPlainWhenRefreshUnavailable(t *testing.T) {
	client := &fakeClient{
		results: []Snapshot{
			{VideoID: "video-one", Kind: KindPlain, Source: "YTMusic", Text: "plain"},
			{VideoID: "video-one", Kind: KindUnavailable},
		},
	}
	service := NewService(client)
	now := time.Unix(1_700_000_000, 0)
	service.now = func() time.Time { return now }

	first, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(plainFreshTTL + time.Second)
	second, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
	if err != nil {
		t.Fatal(err)
	}

	if first.Kind != KindPlain || second.Kind != KindPlain || second.Text != "plain" {
		t.Fatalf("expected cached plain fallback, first=%#v second=%#v", first, second)
	}
	if len(client.requests) != 2 {
		t.Fatalf("expected stale plain lyrics to refresh once, got %d requests", len(client.requests))
	}
}

func TestTrackLyricsReplacesStalePlainAfterSuccessfulRefresh(t *testing.T) {
	client := &fakeClient{results: []Snapshot{
		{VideoID: "video-one", Kind: KindPlain, Text: "old"},
		{VideoID: "video-one", Kind: KindPlain, Text: "new"},
	}}
	service := NewService(client)
	now := time.Unix(1_700_000_000, 0)
	service.now = func() time.Time { return now }

	if _, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(plainFreshTTL + time.Second)
	refreshed, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one"})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Text != "new" {
		t.Fatalf("expected successful refresh to replace stale plain lyrics, got %#v", refreshed)
	}
}

type orderedLyricsClient struct {
	mu           sync.Mutex
	requests     int
	firstStarted chan struct{}
	releaseFirst chan struct{}
}

func (client *orderedLyricsClient) TrackLyrics(_ context.Context, request Request) (Snapshot, error) {
	client.mu.Lock()
	client.requests++
	requestNumber := client.requests
	client.mu.Unlock()
	if requestNumber == 1 {
		close(client.firstStarted)
		<-client.releaseFirst
		return Snapshot{VideoID: request.VideoID, Kind: KindSynced, Text: "old"}, nil
	}
	return Snapshot{VideoID: request.VideoID, Kind: KindSynced, Text: "new"}, nil
}

func TestTrackLyricsLateSameKeyRequestCannotOverwriteNewerCache(t *testing.T) {
	client := &orderedLyricsClient{
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	service := NewService(client)
	request := Request{VideoID: "video-one", Title: "One"}
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = service.TrackLyrics(context.Background(), request)
	}()
	<-client.firstStarted

	newer, err := service.TrackLyrics(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if newer.Text != "new" {
		t.Fatalf("unexpected newer result: %#v", newer)
	}
	close(client.releaseFirst)
	<-firstDone

	cached, err := service.TrackLyrics(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if cached.Text != "new" {
		t.Fatalf("late request overwrote newer cache: %#v", cached)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.requests != 2 {
		t.Fatalf("expected fresh newer cache without a third provider request, got %d", client.requests)
	}
}

func TestTrackLyricsServesFreshPlainWithoutRefetching(t *testing.T) {
	client := &fakeClient{results: []Snapshot{{
		VideoID: "video-one",
		Kind:    KindPlain,
		Source:  "YTMusic",
		Text:    "plain",
	}}}
	service := NewService(client)

	for range 2 {
		result, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Kind != KindPlain || result.Text != "plain" {
			t.Fatalf("unexpected plain lyrics: %#v", result)
		}
	}
	if len(client.requests) != 1 {
		t.Fatalf("expected fresh plain cache to avoid refetch, got %d requests", len(client.requests))
	}
}

func TestTrackLyricsLocalRefreshCanDowngradeStaleSyncedLyrics(t *testing.T) {
	client := &fakeClient{results: []Snapshot{
		{Kind: KindSynced, ProviderID: "local_sidecar", Lines: []Line{{StartMs: 1000, Text: "old synced"}}},
		{Kind: KindPlain, ProviderID: "youtube_music", Text: "new plain"},
	}}
	service := NewService(client)
	now := time.Unix(1_700_000_000, 0)
	service.now = func() time.Time { return now }
	request := Request{Key: "local:one", Title: "Song"}
	if _, err := service.TrackLyrics(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	now = now.Add(localFreshTTL + time.Second)
	refreshed, err := service.TrackLyrics(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Kind != KindPlain || refreshed.Text != "new plain" {
		t.Fatalf("stale local synced lyrics masked successful downgrade: %#v", refreshed)
	}
	cached, err := service.TrackLyrics(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if cached.Kind != KindPlain || len(client.requests) != 2 {
		t.Fatalf("downgraded local result was not cached: requests=%d result=%#v", len(client.requests), cached)
	}
}

func TestTrackLyricsLocalMetadataChangeInvalidatesFreshCache(t *testing.T) {
	client := &fakeClient{results: []Snapshot{
		{Kind: KindPlain, ProviderID: "lrclib", Text: "old identity"},
		{Kind: KindPlain, ProviderID: "lrclib", Text: "new identity"},
	}}
	service := NewService(client)
	first := Request{Key: "local:one", Title: "Song", Artist: "Artist - Topic"}
	if _, err := service.TrackLyrics(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Artist = "Artist"
	result, err := service.TrackLyrics(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "new identity" || len(client.requests) != 2 {
		t.Fatalf("metadata edit reused stale local lyrics: requests=%d result=%#v", len(client.requests), result)
	}
}

func TestTrackLyricsLocalRefreshCanRemoveDeletedLyrics(t *testing.T) {
	client := &fakeClient{results: []Snapshot{
		{Kind: KindSynced, ProviderID: "local_sidecar", Lines: []Line{{StartMs: 1000, Text: "old synced"}}},
		{Kind: KindUnavailable},
	}}
	service := NewService(client)
	now := time.Unix(1_700_000_000, 0)
	service.now = func() time.Time { return now }
	request := Request{Key: "local:one", Title: "Song"}
	if _, err := service.TrackLyrics(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	now = now.Add(localFreshTTL + time.Second)
	refreshed, err := service.TrackLyrics(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Kind != KindUnavailable {
		t.Fatalf("deleted local lyrics remained pinned in cache: %#v", refreshed)
	}
}

func TestTrackLyricsNegativeCacheIsFreshAndBounded(t *testing.T) {
	client := &fakeClient{}
	service := NewService(client)
	now := time.Unix(1_700_000_000, 0)
	service.now = func() time.Time { return now }
	lastVideoID := ""

	for index := 0; index < cacheMaxEntries+1; index++ {
		videoID := "missing-" + strconv.Itoa(index)
		lastVideoID = videoID
		result, err := service.TrackLyrics(context.Background(), Request{VideoID: videoID})
		if err != nil {
			t.Fatal(err)
		}
		if result.Kind != KindUnavailable {
			t.Fatalf("unexpected missing result: %#v", result)
		}
		now = now.Add(time.Second)
	}
	if len(service.cache) != cacheMaxEntries {
		t.Fatalf("expected bounded cache size %d, got %d", cacheMaxEntries, len(service.cache))
	}

	requestsBefore := len(client.requests)
	result, err := service.TrackLyrics(context.Background(), Request{VideoID: lastVideoID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != KindUnavailable {
		t.Fatalf("unexpected cached missing result: %#v", result)
	}
	if len(client.requests) != requestsBefore {
		t.Fatalf("expected fresh negative cache to avoid refetch, got %d requests", len(client.requests)-requestsBefore)
	}
}

func TestNormalizeSnapshotDerivesTimingQualityAndClampsConfidence(t *testing.T) {
	snapshot := normalizeSnapshot(Snapshot{
		Kind:       KindSynced,
		Confidence: 140,
		Lines: []Line{{
			Text:  "hello",
			Words: []Word{{StartMs: 100, EndMs: 300, Text: "hello"}},
		}},
	})

	if snapshot.TimingQuality != TimingQualityWord {
		t.Fatalf("expected word timing quality, got %q", snapshot.TimingQuality)
	}
	if snapshot.Confidence != 100 {
		t.Fatalf("expected confidence to clamp to 100, got %d", snapshot.Confidence)
	}
}

func TestTrackLyricsClassifiesRetryableFailures(t *testing.T) {
	service := NewService(&fakeClient{err: context.DeadlineExceeded})
	result, err := service.TrackLyrics(context.Background(), Request{
		VideoID: "video-one",
		Title:   "One",
	})
	if err == nil {
		t.Fatal("expected the provider error to remain observable")
	}
	if result.ErrorCode != ErrorCodeTimeout || !result.Retryable || result.Error != "" {
		t.Fatalf("unexpected classified lyrics error: %#v", result)
	}
}

type fakeLyricsHTTPStatusError int

func (err fakeLyricsHTTPStatusError) Error() string {
	return "provider request failed"
}

func (err fakeLyricsHTTPStatusError) HTTPStatusCode() int {
	return int(err)
}

func TestClassifyLyricsErrorUsesStablePublicCodes(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		code      string
		retryable bool
	}{
		{name: "typed request timeout", err: fakeLyricsHTTPStatusError(408), code: ErrorCodeTimeout, retryable: true},
		{name: "typed rate limit", err: fakeLyricsHTTPStatusError(429), code: ErrorCodeRateLimited, retryable: true},
		{name: "typed bad gateway", err: fakeLyricsHTTPStatusError(502), code: ErrorCodeProviderUnavailable, retryable: true},
		{name: "joined service unavailable", err: errors.Join(errors.New(`Get "https://lrclib.net/api/get-cached?track_name=private": Bad Gateway`), fakeLyricsHTTPStatusError(503)), code: ErrorCodeProviderUnavailable, retryable: true},
		{name: "service unavailable wins after auth", err: errors.Join(errors.New("not authenticated"), fakeLyricsHTTPStatusError(503)), code: ErrorCodeProviderUnavailable, retryable: true},
		{name: "service unavailable wins before auth", err: errors.Join(fakeLyricsHTTPStatusError(503), errors.New("not authenticated")), code: ErrorCodeProviderUnavailable, retryable: true},
		{name: "rate limit wins across joined statuses", err: errors.Join(fakeLyricsHTTPStatusError(503), fakeLyricsHTTPStatusError(429)), code: ErrorCodeRateLimited, retryable: true},
		{name: "gateway text", err: errors.New("upstream proxy returned Bad Gateway"), code: ErrorCodeProviderUnavailable, retryable: true},
		{name: "timeout", err: context.DeadlineExceeded, code: ErrorCodeTimeout, retryable: true},
		{name: "network", err: errors.New("dial tcp: network is unreachable"), code: ErrorCodeNetworkUnavailable, retryable: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, retryable := classifyLyricsError(test.err)
			if code != test.code || retryable != test.retryable {
				t.Fatalf("classifyLyricsError() = (%q, %t), want (%q, %t)", code, retryable, test.code, test.retryable)
			}
		})
	}
}

func TestCloneSnapshotOwnsRichLyricSlices(t *testing.T) {
	endsWithSpace := true
	original := Snapshot{Lines: []Line{{
		Text:           "hello",
		AlternateTexts: []AlternateText{{Role: "translation", Text: "你好"}},
		Words: []Word{{
			StartMs:       100,
			EndMs:         300,
			Text:          "hello",
			EndsWithSpace: &endsWithSpace,
			Syllables:     []Word{{StartMs: 100, EndMs: 200, Text: "hel"}},
		}},
	}}}
	cloned := cloneSnapshot(original)

	original.Lines[0].AlternateTexts[0].Text = "changed"
	original.Lines[0].Words[0].Text = "changed"
	original.Lines[0].Words[0].Syllables[0].Text = "changed"
	*original.Lines[0].Words[0].EndsWithSpace = false
	if cloned.Lines[0].AlternateTexts[0].Text != "你好" ||
		cloned.Lines[0].Words[0].Text != "hello" ||
		cloned.Lines[0].Words[0].EndsWithSpace == nil ||
		!*cloned.Lines[0].Words[0].EndsWithSpace ||
		cloned.Lines[0].Words[0].Syllables[0].Text != "hel" {
		t.Fatalf("expected cloned rich lyric slices to remain isolated: %#v", cloned.Lines[0])
	}
}
