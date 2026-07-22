package youtubemusic

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
)

func TestSearchLyricsQueryVariantsFindsTraditionalChineseAndSoftensPartialFailure(t *testing.T) {
	var calls atomic.Int32
	result, err := searchLyricsQueryVariants(
		context.Background(),
		LyricsSearchInfo{
			Title:  "后来",
			Artist: "刘若英",
			SearchVariants: []LyricsSearchVariant{
				{Title: "後來", Artist: "劉若英"},
			},
		},
		func(_ context.Context, info LyricsSearchInfo) (LyricsResult, error) {
			calls.Add(1)
			if info.Title == "后来" {
				return LyricsResult{}, errors.New("canonical provider failure")
			}
			if info.Title == "後來" && info.Artist == "劉若英" {
				return LyricsResult{
					Kind:          lyricsResultSynced,
					TimingQuality: "line",
					Confidence:    92,
					Lines:         []LyricLine{{StartMs: 1000, Text: "後來"}},
				}, nil
			}
			return LyricsResult{Kind: lyricsResultUnavailable}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || result.Kind != lyricsResultSynced || len(result.Lines) != 1 {
		t.Fatalf("traditional variant was not selected: calls=%d result=%#v", calls.Load(), result)
	}
}

func TestNormalizeLyricsSearchVariantsIsBoundedStableAndRequestCacheSensitive(t *testing.T) {
	variants := []LyricsSearchVariant{
		{Title: " Canonical ", Artist: "Artist"},
		{Title: "Variant 1", Artist: "Artist"},
		{Title: "Variant 1", Artist: "Artist"},
	}
	for index := 2; index <= 12; index++ {
		variants = append(variants, LyricsSearchVariant{Title: "Variant " + strconv.Itoa(index), Artist: "Artist"})
	}
	info := normalizeLyricsSearchInfo(LyricsSearchInfo{
		Title: "Canonical", Artist: "Artist", SearchVariants: variants,
	})
	if maxLyricsSearchVariants != 3 || len(info.SearchVariants) != 3 {
		t.Fatalf("expected hard variant cap 3, constant=%d variants=%#v", maxLyricsSearchVariants, info.SearchVariants)
	}
	if info.SearchVariants[0].Title != "Variant 1" || info.SearchVariants[1].Title != "Variant 2" {
		t.Fatalf("variant order was not stable: %#v", info.SearchVariants)
	}
	withoutVariants := info
	withoutVariants.SearchVariants = nil
	if lyricsCacheKey(info, "en") != lyricsCacheKey(withoutVariants, "en") {
		t.Fatal("search variants changed the successful canonical cache identity")
	}
	if lyricsRequestCacheKey(info, "en") == lyricsRequestCacheKey(withoutVariants, "en") {
		t.Fatal("search variants did not change the in-flight and negative-cache identity")
	}
	equivalent := LyricsSearchInfo{
		Title: " Canonical ", Artist: " Artist ",
		SearchVariants: []LyricsSearchVariant{
			{Title: " Variant 1 ", Artist: " Artist "},
			{Title: "Variant 1", Artist: "Artist"},
			{Title: "Variant 2", Artist: "Artist"},
			{Title: "Variant 3", Artist: "Artist"},
		},
	}
	if lyricsRequestCacheKey(info, "en") != lyricsRequestCacheKey(equivalent, "en") {
		t.Fatal("request cache identity was sensitive to unnormalized duplicate variants")
	}
	reordered := info
	reordered.SearchVariants = append([]LyricsSearchVariant(nil), info.SearchVariants...)
	reordered.SearchVariants[0], reordered.SearchVariants[1] = reordered.SearchVariants[1], reordered.SearchVariants[0]
	if lyricsRequestCacheKey(info, "en") == lyricsRequestCacheKey(reordered, "en") {
		t.Fatal("request cache identity ignored normalized variant priority")
	}
}

func TestSearchLyricsQueryVariantsAggregatesWhenEveryQueryFails(t *testing.T) {
	_, err := searchLyricsQueryVariants(
		context.Background(),
		LyricsSearchInfo{Title: "canonical", SearchVariants: []LyricsSearchVariant{{Title: "variant"}}},
		func(_ context.Context, info LyricsSearchInfo) (LyricsResult, error) {
			return LyricsResult{}, errors.New(info.Title + " failed")
		},
	)
	if err == nil || !strings.Contains(err.Error(), "canonical failed") || !strings.Contains(err.Error(), "variant failed") {
		t.Fatalf("expected stable aggregate failure evidence, got %v", err)
	}
}

func TestSearchLyricsQueryVariantsWithoutVariantsCallsProviderOnce(t *testing.T) {
	var calls atomic.Int32
	_, err := searchLyricsQueryVariants(
		context.Background(),
		LyricsSearchInfo{Title: "Track", Artist: "Artist"},
		func(_ context.Context, _ LyricsSearchInfo) (LyricsResult, error) {
			calls.Add(1)
			return LyricsResult{Kind: lyricsResultUnavailable}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("canonical-only request called provider %d times", calls.Load())
	}
}

func TestSearchLyricsQueryVariantsReturnsTerminalCanonicalBeforeAlternates(t *testing.T) {
	var calls atomic.Int32
	result, err := searchLyricsQueryVariants(
		context.Background(),
		LyricsSearchInfo{
			Title:          "canonical",
			SearchVariants: []LyricsSearchVariant{{Title: "variant"}},
		},
		func(_ context.Context, info LyricsSearchInfo) (LyricsResult, error) {
			calls.Add(1)
			return LyricsResult{
				Kind:          lyricsResultSynced,
				TimingQuality: "line",
				Confidence:    90,
				Text:          info.Title,
				Lines:         []LyricLine{{Text: info.Title}},
			}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || result.Text != "canonical" {
		t.Fatalf("terminal canonical result did not stop variants: calls=%d result=%#v", calls.Load(), result)
	}
}

func TestSearchLyricsQueryVariantsPreservesErrorWhenOthersAreUnavailable(t *testing.T) {
	_, err := searchLyricsQueryVariants(
		context.Background(),
		LyricsSearchInfo{
			Title: "canonical",
			SearchVariants: []LyricsSearchVariant{
				{Title: "unavailable"},
				{Title: "failed"},
			},
		},
		func(_ context.Context, info LyricsSearchInfo) (LyricsResult, error) {
			switch info.Title {
			case "canonical":
				return LyricsResult{}, errors.New("canonical failed")
			case "failed":
				return LyricsResult{}, errors.New("alternate failed")
			default:
				return LyricsResult{Kind: lyricsResultUnavailable}, nil
			}
		},
	)
	if err == nil || !strings.Contains(err.Error(), "canonical failed") || !strings.Contains(err.Error(), "alternate failed") {
		t.Fatalf("unavailable alternate swallowed provider errors: %v", err)
	}
}

func TestSearchLyricsQueryVariantsReturnsBestAvailableAtDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, err := searchLyricsQueryVariants(
		ctx,
		LyricsSearchInfo{
			Title: "canonical",
			SearchVariants: []LyricsSearchVariant{
				{Title: "better"},
				{Title: "blocked"},
			},
		},
		func(searchCtx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
			switch info.Title {
			case "canonical":
				return LyricsResult{
					Kind: lyricsResultPlain,
					Text: "canonical plain",
				}, nil
			case "better":
				return LyricsResult{
					Kind:          lyricsResultSynced,
					TimingQuality: "line",
					Text:          "better line",
					Lines:         []LyricLine{{Text: "better line"}},
				}, nil
			default:
				<-searchCtx.Done()
				return LyricsResult{}, searchCtx.Err()
			}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "better line" {
		t.Fatalf("deadline discarded the best completed result: %#v", result)
	}
}

func TestSearchLyricsQueryVariantsRanksQualityConfidenceThenStableQueryOrder(t *testing.T) {
	result, err := searchLyricsQueryVariants(
		context.Background(),
		LyricsSearchInfo{
			Title: "canonical",
			SearchVariants: []LyricsSearchVariant{
				{Title: "higher-confidence"},
				{Title: "same-confidence-later"},
			},
		},
		func(_ context.Context, info LyricsSearchInfo) (LyricsResult, error) {
			confidence := 80
			kind := lyricsResultPlain
			timingQuality := "plain"
			lines := []LyricLine(nil)
			if info.Title != "canonical" {
				kind = lyricsResultSynced
				confidence = 95
				timingQuality = "word"
				lines = []LyricLine{{Text: info.Title, Words: []TimedWord{{Text: info.Title}}}}
			}
			return LyricsResult{
				Kind:          kind,
				TimingQuality: timingQuality,
				Confidence:    confidence,
				Text:          info.Title,
				Lines:         lines,
			}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "higher-confidence" {
		t.Fatalf("expected confidence then stable query order, got %#v", result)
	}
}

func TestYTMusicProviderQueriesCanonicalVideoOnceWithVariants(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{{
		Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true,
	}}})
	var nextCalls atomic.Int32
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/youtubei/v1/next" {
			nextCalls.Add(1)
			return testHTTPResponse(request, http.StatusOK, `{}`), nil
		}
		return testHTTPResponse(request, http.StatusOK, `[]`), nil
	})}
	info := LyricsSearchInfo{
		VideoID: "TESTVID007G",
		Title:   "后来",
		Artist:  "刘若英",
		SearchVariants: []LyricsSearchVariant{
			{Title: "後來", Artist: "劉若英"},
			{Title: "Hou Lai", Artist: "Rene Liu"},
		},
	}
	providers, err := client.lyricsProvidersForInfo(normalizeLyricsSearchInfo(info))
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 || providers[0].name != "YTMusic" {
		t.Fatalf("unexpected providers: %#v", providers)
	}
	if _, err := providers[0].search(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	if nextCalls.Load() != 1 {
		t.Fatalf("YTMusic video-ID provider ran %d canonical queries", nextCalls.Load())
	}
}

func TestSearchLyricsCandidatesDeduplicatesAcrossVariants(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "lrclib.net" {
			return testHTTPResponse(request, http.StatusOK, `[{
				"id":42,"trackName":"后来","artistName":"刘若英","duration":213,"plainLyrics":"plain","syncedLyrics":"[00:01.00]line"
			}]`), nil
		}
		return testHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
	})}
	candidates, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title: "后来", Artist: "刘若英", DurationSeconds: 213,
		SearchVariants: []LyricsSearchVariant{{Title: "後來", Artist: "劉若英"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.ProviderID, lyricsProviderLRCLib) && candidate.ProviderTrackID == "42" {
			count++
			if !candidate.HasSynced || !candidate.HasPlain {
				t.Fatalf("deduplication lost capability flags: %#v", candidate)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected one deduplicated LRCLIB candidate, got %d in %#v", count, candidates)
	}
}

func TestSearchLyricsCandidatesKeepsAMLLResultsWhenLRCLIBFails(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "lrclib.net":
			return testHTTPResponse(request, http.StatusTooManyRequests, `{}`), nil
		case "amlldb.bikonoo.com":
			return testHTTPResponse(request, http.StatusOK, `[{
				"platform":"raw-lyrics",
				"file":"1689089845000-39523898-31c2fa0c.ttml",
				"title":"Idol",
				"artists":["YOASOBI"],
				"albums":["Idol"]
			}]`), nil
		default:
			t.Fatalf("unexpected provider request: %s", request.URL)
			return nil, nil
		}
	})}

	candidates, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title: "Idol", Artist: "YOASOBI", Album: "Idol", DurationSeconds: 210,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ProviderID != lyricsProviderAMLL {
		t.Fatalf("LRCLIB failure discarded AMLL candidates: %#v", candidates)
	}
}
