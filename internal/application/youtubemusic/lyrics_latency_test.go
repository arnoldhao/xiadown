package youtubemusic

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type lyricsFetchTestOutcome struct {
	result LyricsResult
	err    error
}

func TestFetchLyricsFromProvidersPreservesEveryFailureRegardlessOfCompletionOrder(t *testing.T) {
	client := NewClient(fakeCookieProvider{})
	releaseSlow := make(chan struct{})
	providers := []lyricsProvider{
		{
			name: "slow-service",
			search: func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
				<-releaseSlow
				return LyricsResult{}, &lrcLibHTTPError{StatusCode: http.StatusServiceUnavailable}
			},
		},
		{
			name: "fast-generic",
			search: func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
				close(releaseSlow)
				return LyricsResult{}, errors.New("generic provider failure")
			},
		},
	}

	_, err := client.fetchLyricsFromProviders(context.Background(), LyricsSearchInfo{}, providers)
	if err == nil {
		t.Fatal("expected joined provider failure")
	}
	for _, expected := range []string{"slow-service lyrics provider", "lrclib api status 503", "fast-generic lyrics provider", "generic provider failure"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("joined provider failure lost %q: %v", expected, err)
		}
	}
}

func TestFetchLyricsFromProvidersReturnsSyncedWithoutWaitingForSlowerProvider(t *testing.T) {
	client := NewClient(fakeCookieProvider{})
	slowCancelled := make(chan struct{})
	providers := []lyricsProvider{
		{
			name: "fast",
			search: func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
				return LyricsResult{
					Kind:          lyricsResultSynced,
					Source:        "fast",
					TimingQuality: "word",
					Lines: []LyricLine{{
						StartMs: 100,
						Text:    "line",
						Words:   []TimedWord{{StartMs: 100, EndMs: 300, Text: "line"}},
					}},
				}, nil
			},
		},
		{
			name: "slow",
			search: func(ctx context.Context, _ LyricsSearchInfo) (LyricsResult, error) {
				<-ctx.Done()
				close(slowCancelled)
				return LyricsResult{Kind: lyricsResultUnavailable}, nil
			},
		},
	}

	startedAt := time.Now()
	result, err := client.fetchLyricsFromProviders(context.Background(), LyricsSearchInfo{}, providers)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(startedAt); elapsed > 250*time.Millisecond {
		t.Fatalf("expected immediate synced lyrics, took %s", elapsed)
	}
	if result.Kind != lyricsResultSynced || result.Source != "fast" {
		t.Fatalf("unexpected result: %+v", result)
	}
	select {
	case <-slowCancelled:
	case <-time.After(time.Second):
		t.Fatal("expected slower provider to be cancelled")
	}
}

func TestFetchLyricsFromProvidersWaitsForSyncedAfterPlainResult(t *testing.T) {
	previousSoftWait := lyricsProviderSoftWait
	lyricsProviderSoftWait = 15 * time.Millisecond
	defer func() { lyricsProviderSoftWait = previousSoftWait }()

	client := NewClient(fakeCookieProvider{})
	plainReturned := make(chan struct{})
	releaseSynced := make(chan struct{})
	syncedStarted := make(chan struct{})
	providers := []lyricsProvider{
		{
			name: "plain",
			search: func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
				close(plainReturned)
				return LyricsResult{Kind: lyricsResultPlain, Source: "plain", Text: "available"}, nil
			},
		},
		{
			name: "synced",
			search: func(ctx context.Context, _ LyricsSearchInfo) (LyricsResult, error) {
				close(syncedStarted)
				select {
				case <-releaseSynced:
					return LyricsResult{
						Kind:   lyricsResultSynced,
						Source: "synced",
						Lines:  []LyricLine{{StartMs: 100, Text: "timed"}},
					}, nil
				case <-ctx.Done():
					return LyricsResult{Kind: lyricsResultUnavailable}, ctx.Err()
				}
			},
		},
	}

	outcomeCh := make(chan lyricsFetchTestOutcome, 1)
	go func() {
		result, err := client.fetchLyricsFromProviders(context.Background(), LyricsSearchInfo{}, providers)
		outcomeCh <- lyricsFetchTestOutcome{result: result, err: err}
	}()
	<-plainReturned
	<-syncedStarted
	select {
	case outcome := <-outcomeCh:
		t.Fatalf("plain lyrics returned before the synced provider completed: %+v, err=%v", outcome.result, outcome.err)
	case <-time.After(4 * lyricsProviderSoftWait):
	}
	close(releaseSynced)
	outcome := <-outcomeCh
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	if outcome.result.Kind != lyricsResultSynced || outcome.result.Source != "synced" {
		t.Fatalf("expected the slower synced provider to win, got %+v", outcome.result)
	}
}

func TestFetchLyricsFromProvidersWaitsForUnavailableBeforePlainFallback(t *testing.T) {
	previousSoftWait := lyricsProviderSoftWait
	lyricsProviderSoftWait = 15 * time.Millisecond
	defer func() { lyricsProviderSoftWait = previousSoftWait }()

	client := NewClient(fakeCookieProvider{})
	releaseUnavailable := make(chan struct{})
	unavailableStarted := make(chan struct{})
	providers := []lyricsProvider{
		{
			name: "plain",
			search: func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
				return LyricsResult{Kind: lyricsResultPlain, Source: "plain", Text: "fallback"}, nil
			},
		},
		{
			name: "unavailable",
			search: func(ctx context.Context, _ LyricsSearchInfo) (LyricsResult, error) {
				close(unavailableStarted)
				select {
				case <-releaseUnavailable:
					return LyricsResult{Kind: lyricsResultUnavailable}, nil
				case <-ctx.Done():
					return LyricsResult{Kind: lyricsResultUnavailable}, ctx.Err()
				}
			},
		},
	}

	outcomeCh := make(chan lyricsFetchTestOutcome, 1)
	go func() {
		result, err := client.fetchLyricsFromProviders(context.Background(), LyricsSearchInfo{}, providers)
		outcomeCh <- lyricsFetchTestOutcome{result: result, err: err}
	}()
	<-unavailableStarted
	select {
	case outcome := <-outcomeCh:
		t.Fatalf("plain fallback returned before the other provider completed: %+v, err=%v", outcome.result, outcome.err)
	case <-time.After(4 * lyricsProviderSoftWait):
	}
	close(releaseUnavailable)
	outcome := <-outcomeCh
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	if outcome.result.Kind != lyricsResultPlain || outcome.result.Text != "fallback" {
		t.Fatalf("unexpected fallback result: %+v", outcome.result)
	}
}

func TestFetchLyricsFromProvidersPlainOnlyKeepsSoftBudget(t *testing.T) {
	previousSoftWait := lyricsProviderSoftWait
	lyricsProviderSoftWait = 15 * time.Millisecond
	defer func() { lyricsProviderSoftWait = previousSoftWait }()

	client := NewClient(fakeCookieProvider{})
	slowCancelled := make(chan struct{})
	providers := []lyricsProvider{
		{
			name: "plain",
			search: func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
				return LyricsResult{Kind: lyricsResultPlain, Source: "plain", Text: "available"}, nil
			},
		},
		{
			name: "slow",
			search: func(ctx context.Context, _ LyricsSearchInfo) (LyricsResult, error) {
				<-ctx.Done()
				close(slowCancelled)
				return LyricsResult{Kind: lyricsResultUnavailable}, nil
			},
		},
	}

	startedAt := time.Now()
	result, err := client.fetchLyricsFromProviders(context.Background(), LyricsSearchInfo{PlainOnly: true}, providers)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(startedAt); elapsed > 250*time.Millisecond {
		t.Fatalf("expected plain-only lyrics after the soft budget, took %s", elapsed)
	}
	if result.Kind != lyricsResultPlain || result.Text != "available" {
		t.Fatalf("unexpected result: %+v", result)
	}
	select {
	case <-slowCancelled:
	case <-time.After(time.Second):
		t.Fatal("expected the slow plain-only provider to be cancelled")
	}
}

func TestTrackLyricsServesFreshPlainAndUnavailableCacheEntries(t *testing.T) {
	client := NewClient(fakeCookieProvider{})
	now := time.Unix(1_700_000_000, 0)
	client.now = func() time.Time { return now }

	plainInfo := LyricsSearchInfo{VideoID: "TESTVID007G", Title: "Track"}
	plainKey := lyricsCacheKey(normalizeLyricsSearchInfo(plainInfo), localeFromContext(context.Background()))
	client.storeLyricsCache(plainKey, LyricsResult{Kind: lyricsResultPlain, Source: "cache", Text: "plain"})
	plain, err := client.TrackLyrics(context.Background(), plainInfo)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Kind != lyricsResultPlain || plain.Text != "plain" {
		t.Fatalf("unexpected plain cache result: %+v", plain)
	}

	unavailableInfo := LyricsSearchInfo{VideoID: "OTHERID007G", Title: "Missing"}
	unavailableKey := lyricsRequestCacheKey(normalizeLyricsSearchInfo(unavailableInfo), localeFromContext(context.Background()))
	client.storeLyricsCache(unavailableKey, LyricsResult{Kind: lyricsResultUnavailable})
	unavailable, err := client.TrackLyrics(context.Background(), unavailableInfo)
	if err != nil {
		t.Fatal(err)
	}
	if unavailable.Kind != lyricsResultUnavailable {
		t.Fatalf("unexpected negative cache result: %+v", unavailable)
	}
}

func TestLRCLibPlainExactWaitsForSlowerSyncedSearch(t *testing.T) {
	previousSoftWait := lyricsProviderSoftWait
	lyricsProviderSoftWait = 15 * time.Millisecond
	defer func() { lyricsProviderSoftWait = previousSoftWait }()

	client := NewClient(fakeCookieProvider{})
	searchCompleted := make(chan struct{})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/get-cached":
			return testHTTPResponse(request, http.StatusOK, `{"id":1,"trackName":"Track","artistName":"Artist","duration":213,"plainLyrics":"Plain"}`), nil
		case "/api/search":
			timer := time.NewTimer(4 * lyricsProviderSoftWait)
			defer timer.Stop()
			select {
			case <-request.Context().Done():
				return nil, request.Context().Err()
			case <-timer.C:
				close(searchCompleted)
				return testHTTPResponse(request, http.StatusOK, `[{"id":2,"trackName":"Track","artistName":"Artist","duration":213,"syncedLyrics":"[00:01.00]Synced"}]`), nil
			}
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result := client.searchLRCLibLyrics(context.Background(), LyricsSearchInfo{
		Title:           "Track",
		Artist:          "Artist",
		DurationSeconds: 213,
	})
	select {
	case <-searchCompleted:
	default:
		t.Fatal("expected the synced search lookup to complete instead of being cancelled by the plain exact result")
	}
	if result.Kind != lyricsResultSynced || len(result.Lines) == 0 || result.Lines[len(result.Lines)-1].Text != "Synced" {
		t.Fatalf("unexpected LRCLib result: %+v", result)
	}
}
