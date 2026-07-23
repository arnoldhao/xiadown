package youtubemusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testAMLLRawFile   = "1689089845000-39523898-31c2fa0c.ttml"
	testAMLLTimedTTML = `<tt><body><div><p begin="1.000" end="2.000"><span begin="1.000" end="2.000">chosen</span></p></div></body></tt>`
)

func TestAMLLCommunitySearchUsesTitleOnlyAndScoresIdentityLocally(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "amlldb.bikonoo.com" || request.URL.Path != "/api/search-lyrics" {
			t.Fatalf("unexpected AMLL search URL: %s", request.URL)
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["query"] != "Idol" || payload["type"] != "title" {
			t.Fatalf("community search must use title-only mode: %#v", payload)
		}
		return testHTTPResponse(request, http.StatusOK, `[
			{"platform":"raw-lyrics","file":"1689089845000-39523898-31c2fa0c.ttml","title":"Idol","artists":["YOASOBI"],"albums":["Idol"],"authorNames":["Steve-xmh"],"ncmIds":["2048982668"]},
			{"platform":"raw-lyrics","file":"1689089846000-39523898-31c2fa0d.ttml","title":"Idol","artists":["Different Artist"],"albums":["Idol"]}
		]`), nil
	})}

	candidates, err := client.searchAMLLLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title: "Idol", Artist: "YOASOBI", Album: "Idol", DurationSeconds: 210,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("unexpected AMLL candidates: %#v", candidates)
	}
	if candidates[0].ProviderID != lyricsProviderAMLL || candidates[0].ProviderTrackID != "1689089845000-39523898-31c2fa0c.ttml" || !candidates[0].Accepted {
		t.Fatalf("expected stable accepted AMLL candidate: %#v", candidates[0])
	}
	if candidates[0].TimingQuality != "line" || candidates[0].Attribution != "AMLL TTML DB contributors · Steve-xmh" {
		t.Fatalf("catalog capability must remain conservative: %#v", candidates[0])
	}
	if candidates[1].Accepted || candidates[1].Rejection != "artist mismatch" {
		t.Fatalf("community score must not bypass local identity gates: %#v", candidates[1])
	}
}

func TestAMLLEmptyCommunitySearchFallsBackToETagCatalog(t *testing.T) {
	client := NewClient(nil)
	now := time.Unix(1_800_000_000, 0)
	client.now = func() time.Time { return now }
	catalogRequests := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "amlldb.bikonoo.com":
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		case "raw.githubusercontent.com":
			catalogRequests++
			if catalogRequests == 1 {
				response := testHTTPResponse(request, http.StatusOK, `{"metadata":[["musicName",["Idol"]],["artists",["YOASOBI"]],["album",["Idol"]],["ncmMusicId",["2048982668"]]],"rawLyricFile":"1689089845000-39523898-31c2fa0c.ttml"}`+"\n")
				response.Header.Set("ETag", `"catalog-v1"`)
				return response, nil
			}
			if request.Header.Get("If-None-Match") != `"catalog-v1"` {
				t.Fatalf("catalog revalidation omitted ETag: %#v", request.Header)
			}
			if catalogRequests == 2 {
				return testHTTPResponse(request, http.StatusNotModified, ``), nil
			}
			return nil, context.DeadlineExceeded
		default:
			t.Fatalf("unexpected host: %s", request.URL.Host)
			return nil, nil
		}
	})}

	info := LyricsSearchInfo{Title: "Idol", Artist: "YOASOBI", Album: "Idol"}
	first, err := client.searchAMLLLyricsCandidates(context.Background(), info)
	if err != nil || len(first) != 1 || !first[0].Accepted {
		t.Fatalf("empty community response did not fall back to catalog: %#v err=%v", first, err)
	}
	now = now.Add(amllCatalogFreshTTL + time.Minute)
	second, err := client.searchAMLLLyricsCandidates(context.Background(), info)
	if err != nil || len(second) != 1 || catalogRequests != 2 {
		t.Fatalf("ETag catalog revalidation failed: %#v requests=%d err=%v", second, catalogRequests, err)
	}
	now = now.Add(amllCatalogFreshTTL + time.Minute)
	stale, err := client.searchAMLLLyricsCandidates(context.Background(), info)
	if err != nil || len(stale) != 1 || catalogRequests != 3 {
		t.Fatalf("validated stale catalog was not retained through outage: %#v requests=%d err=%v", stale, catalogRequests, err)
	}
}

func TestAMLLRawFallbackParsesBareSecondsWordsTranslationAndRomanization(t *testing.T) {
	client := NewClient(nil)
	officialRequests := 0
	communityRequests := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "raw.githubusercontent.com":
			officialRequests++
			return testHTTPResponse(request, http.StatusServiceUnavailable, ``), nil
		case "amlldb.bikonoo.com":
			communityRequests++
			return testHTTPResponse(request, http.StatusOK, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"><body><div><p begin="1.000" end="3.000"><span begin="1.000" end="1.300">sto</span><span begin="1.300" end="1.700">ry</span> <span begin="1.900" end="2.500">time</span><span ttm:role="x-translation">故事时间</span><span ttm:role="x-romanization">story taimu</span></p></div></body></tt>`), nil
		default:
			t.Fatalf("unexpected host: %s", request.URL.Host)
			return nil, nil
		}
	})}

	result, err := client.fetchAMLLLyricsCandidate(
		context.Background(),
		"1689089845000-39523898-31c2fa0c.ttml",
		96,
		"AMLL TTML DB contributors · Example",
	)
	if err != nil {
		t.Fatal(err)
	}
	if officialRequests != 1 || communityRequests != 1 {
		t.Fatalf("expected official/raw fallback sequence, official=%d community=%d", officialRequests, communityRequests)
	}
	if result.ProviderID != lyricsProviderAMLL || result.ProviderTrackID != "1689089845000-39523898-31c2fa0c.ttml" || result.TimingQuality != "syllable" {
		t.Fatalf("unexpected normalized AMLL result: %#v", result)
	}
	if len(result.Lines) != 1 || result.Lines[0].TranslationText != "故事时间" || result.Lines[0].RomanizedText != "story taimu" {
		t.Fatalf("AMLL supporting text was lost: %#v", result.Lines)
	}
	if len(result.Lines[0].Words) != 2 || len(result.Lines[0].Words[0].Syllables) != 2 || result.Lines[0].Words[0].EndsWithSpace == nil || !*result.Lines[0].Words[0].EndsWithSpace {
		t.Fatalf("AMLL word timing/spacing was lost: %#v", result.Lines[0].Words)
	}
}

func TestAMLLHedgedRawCommunityWinsWhenOfficialHangs(t *testing.T) {
	client := NewClient(nil)
	officialCanceled := make(chan struct{}, 1)
	communityStarted := make(chan struct{}, 1)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "raw.githubusercontent.com":
			<-request.Context().Done()
			officialCanceled <- struct{}{}
			return nil, request.Context().Err()
		case "amlldb.bikonoo.com":
			communityStarted <- struct{}{}
			return testHTTPResponse(request, http.StatusOK, testAMLLTimedTTML), nil
		default:
			return nil, fmt.Errorf("unexpected AMLL raw host: %s", request.URL.Host)
		}
	})}

	startedAt := time.Now()
	result, err := client.fetchAMLLLyricsCandidate(context.Background(), testAMLLRawFile, 100, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != lyricsProviderAMLL || result.ProviderTrackID != testAMLLRawFile {
		t.Fatalf("community hedge did not produce AMLL lyrics: %#v", result)
	}
	if elapsed := time.Since(startedAt); elapsed < amllCommunityRawHedgeWait || elapsed > amllCommunityRawHedgeWait+time.Second {
		t.Fatalf("community hedge escaped its bounded start window: %s", elapsed)
	}
	select {
	case <-communityStarted:
	default:
		t.Fatal("community hedge was not requested")
	}
	select {
	case <-officialCanceled:
	case <-time.After(time.Second):
		t.Fatal("winning community response did not cancel the official request")
	}
}

func TestAMLLHedgedRawOfficialFastAvoidsCommunity(t *testing.T) {
	client := NewClient(nil)
	var communityRequests atomic.Int32
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "raw.githubusercontent.com":
			return testHTTPResponse(request, http.StatusOK, testAMLLTimedTTML), nil
		case "amlldb.bikonoo.com":
			communityRequests.Add(1)
			return testHTTPResponse(request, http.StatusOK, testAMLLTimedTTML), nil
		default:
			return nil, fmt.Errorf("unexpected AMLL raw host: %s", request.URL.Host)
		}
	})}

	result, err := client.fetchAMLLLyricsCandidate(context.Background(), testAMLLRawFile, 100, "")
	if err != nil || result.Kind != lyricsResultSynced {
		t.Fatalf("official AMLL response failed: %#v err=%v", result, err)
	}
	if requests := communityRequests.Load(); requests != 0 {
		t.Fatalf("fast official response unnecessarily started %d community requests", requests)
	}
}

func TestAMLLHedgedRawOfficialWinCancelsStartedCommunity(t *testing.T) {
	client := NewClient(nil)
	communityStarted := make(chan struct{})
	communityCanceled := make(chan struct{}, 1)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "raw.githubusercontent.com":
			<-communityStarted
			return testHTTPResponse(request, http.StatusOK, testAMLLTimedTTML), nil
		case "amlldb.bikonoo.com":
			close(communityStarted)
			<-request.Context().Done()
			communityCanceled <- struct{}{}
			return nil, request.Context().Err()
		default:
			return nil, fmt.Errorf("unexpected AMLL raw host: %s", request.URL.Host)
		}
	})}

	result, err := client.fetchAMLLLyricsCandidate(context.Background(), testAMLLRawFile, 100, "")
	if err != nil || result.Kind != lyricsResultSynced {
		t.Fatalf("official hedge response failed: %#v err=%v", result, err)
	}
	select {
	case <-communityCanceled:
	case <-time.After(time.Second):
		t.Fatal("winning official response did not cancel the started community request")
	}
}

func TestAMLLHedgedRawJoinsBothFailures(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "raw.githubusercontent.com":
			return testHTTPResponse(request, http.StatusBadGateway, ``), nil
		case "amlldb.bikonoo.com":
			return testHTTPResponse(request, http.StatusServiceUnavailable, ``), nil
		default:
			return nil, fmt.Errorf("unexpected AMLL raw host: %s", request.URL.Host)
		}
	})}

	_, err := client.fetchAMLLLyricsCandidate(context.Background(), testAMLLRawFile, 100, "")
	if err == nil || !strings.Contains(err.Error(), "official raw") || !strings.Contains(err.Error(), "community raw") {
		t.Fatalf("expected both AMLL raw failures, got %v", err)
	}
}

func TestAMLLHedgedRawParentCancellationStopsBothRequests(t *testing.T) {
	client := NewClient(nil)
	requestStarted := make(chan string, 2)
	requestCanceled := make(chan string, 2)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestStarted <- request.URL.Host
		<-request.Context().Done()
		requestCanceled <- request.URL.Host
		return nil, request.Context().Err()
	})}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	resultCh := make(chan error, 1)
	go func() {
		_, err := client.fetchAMLLLyricsCandidate(ctx, testAMLLRawFile, 100, "")
		resultCh <- err
	}()

	for started := 0; started < 2; started++ {
		select {
		case <-requestStarted:
		case <-time.After(time.Second):
			t.Fatal("hedged AMLL request did not start before cancellation")
		}
	}
	cancel()
	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("parent cancellation was not preserved: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AMLL hedge ignored parent cancellation")
	}
	for canceled := 0; canceled < 2; canceled++ {
		select {
		case <-requestCanceled:
		case <-time.After(time.Second):
			t.Fatal("AMLL child request survived parent cancellation")
		}
	}
}

func TestAMLLAutomaticBudgetExpiresAsSoftUnavailable(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	started := time.Now()
	result, err := client.searchAMLLLyricsProviderWithin(context.Background(), LyricsSearchInfo{
		Title: "Idol", Artist: "YOASOBI",
	}, 25*time.Millisecond)
	if err != nil || result.Kind != lyricsResultUnavailable {
		t.Fatalf("internal AMLL deadline must be a soft miss: %#v err=%v", result, err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("AMLL soft deadline was not bounded: %s", elapsed)
	}
}

func TestAMLLAutomaticReturnsLineOnlyTTMLAsTimedFallback(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "amlldb.bikonoo.com":
			if request.URL.Path != "/api/search-lyrics" {
				return nil, fmt.Errorf("unexpected AMLL community request: %s", request.URL)
			}
			return testHTTPResponse(request, http.StatusOK, `[{
				"platform":"raw-lyrics","file":"1689089845000-39523898-31c2fa0c.ttml","title":"Idol","artists":["YOASOBI"],"albums":["Idol"]
			}]`), nil
		case "raw.githubusercontent.com":
			return testHTTPResponse(request, http.StatusOK, `<tt><body><div><p begin="1.000" end="3.000">line-only lyrics</p></div></body></tt>`), nil
		default:
			return nil, fmt.Errorf("unexpected AMLL provider host: %s", request.URL.Host)
		}
	})}

	result, err := client.searchAMLLLyricsProviderWithin(context.Background(), LyricsSearchInfo{
		Title: "Idol", Artist: "YOASOBI", Album: "Idol",
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != lyricsResultSynced || result.ProviderID != lyricsProviderAMLL || result.TimingQuality != "line" || len(result.Lines) != 1 || len(result.Lines[0].Words) != 0 {
		t.Fatalf("line-only AMLL TTML was discarded: %#v", result)
	}
}

func TestAMLLProviderIsSyncedModeOnly(t *testing.T) {
	client := NewClient(nil)
	synced, err := client.lyricsProvidersForInfo(LyricsSearchInfo{
		VideoID: "abcdefghijk", Title: "Idol", Artist: "YOASOBI",
	})
	if err != nil {
		t.Fatal(err)
	}
	if names := lyricsProviderNames(synced); names != "YTMusic,AMLL,LRCLib" {
		t.Fatalf("unexpected synced provider order: %s", names)
	}
	plain, err := client.lyricsProvidersForInfo(LyricsSearchInfo{
		VideoID: "abcdefghijk", Title: "Idol", Artist: "YOASOBI", PlainOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if names := lyricsProviderNames(plain); names != "YTMusic" {
		t.Fatalf("AMLL leaked into plain-only mode: %s", names)
	}
}

func TestAMLLRejectsUnsafeRawCandidatePath(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Fatalf("unsafe candidate must be rejected before transport: %s", request.URL)
		return nil, nil
	})}
	if _, err := client.fetchAMLLLyricsCandidate(context.Background(), "../escape.ttml", 100, ""); err == nil {
		t.Fatal("expected unsafe AMLL providerTrackId rejection")
	}
}

func TestReadAMLLBoundedBodyRejectsOversizeResponse(t *testing.T) {
	_, err := readAMLLBoundedBody(strings.NewReader(strings.Repeat("x", 33)), 32)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected bounded response error, got %v", err)
	}
}
