package youtubemusic

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSearchLyricsCandidatesReturnsIdentityEvidenceAndRejectedResults(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "lrclib.net" {
			return testHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
		}
		if request.URL.Path != "/api/search" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("track_name") != "Anthem" ||
			request.URL.Query().Get("artist_name") != "Artist" ||
			request.URL.Query().Get("album_name") != "Original" {
			t.Fatalf("unexpected search query: %s", request.URL.RawQuery)
		}
		return testHTTPResponse(request, http.StatusOK, `[
			{"id":42,"trackName":"Anthem","artistName":"Artist","albumName":"Original","duration":213,"plainLyrics":"plain"},
			{"id":43,"trackName":"Anthem (Live)","artistName":"Artist","albumName":"Original","duration":213,"syncedLyrics":"[00:01.00]live"}
		]`), nil
	})}

	candidates, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title:           "Anthem",
		Artist:          "Artist",
		Album:           "Original",
		DurationSeconds: 213,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected two candidates, got %#v", candidates)
	}
	if candidates[0].ProviderTrackID != "42" || !candidates[0].Accepted || candidates[0].Confidence != 100 {
		t.Fatalf("expected accepted identity-first candidate, got %#v", candidates[0])
	}
	if candidates[0].TimingQuality != "plain" || candidates[0].Attribution != lyricsAttributionLRCLib {
		t.Fatalf("expected candidate capability metadata, got %#v", candidates[0])
	}
	if candidates[1].Accepted || candidates[1].Rejection != "incompatible title version" {
		t.Fatalf("expected live version to remain visible but rejected, got %#v", candidates[1])
	}
}

func TestSearchLyricsCandidatesAggregatesGenericAMLLAndLRCLIBResults(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "lrclib.net":
			return testHTTPResponse(request, http.StatusOK, `[{
				"id":42,"trackName":"Idol","artistName":"YOASOBI","albumName":"Idol","duration":210,"syncedLyrics":"[00:01.00]line"
			}]`), nil
		case "amlldb.bikonoo.com":
			return testHTTPResponse(request, http.StatusOK, `[{
				"platform":"raw-lyrics","file":"1689089845000-39523898-31c2fa0c.ttml","title":"Idol","artists":["YOASOBI"],"albums":["Idol"],"ncmIds":["2048982668"]
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
	providers := map[string]bool{}
	for _, candidate := range candidates {
		providers[candidate.ProviderID] = true
	}
	if !providers[lyricsProviderLRCLib] || !providers[lyricsProviderAMLL] {
		t.Fatalf("generic candidate aggregation lost a provider: %#v", candidates)
	}
}

func TestSearchLyricsCandidatesDoesNotAcceptUnusableIdentityMatches(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return testHTTPResponse(request, http.StatusOK, `[
			{"id":50,"trackName":"Anthem","artistName":"Artist","duration":213,"instrumental":true,"plainLyrics":"marker only"},
			{"id":51,"trackName":"Anthem","artistName":"Artist","duration":213,"syncedLyrics":"[00:01.00]synced only"}
		]`), nil
	})}

	candidates, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title:           "Anthem",
		Artist:          "Artist",
		DurationSeconds: 213,
		PlainOnly:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("unexpected candidates: %#v", candidates)
	}
	for _, candidate := range candidates {
		if candidate.Accepted {
			t.Fatalf("unusable candidate was marked accepted: %#v", candidate)
		}
	}
	rejections := map[string]bool{}
	for _, candidate := range candidates {
		rejections[candidate.Rejection] = true
	}
	if !rejections["instrumental record"] || !rejections["plain lyrics unavailable"] {
		t.Fatalf("missing capability rejection evidence: %#v", candidates)
	}
}

func TestTitleOnlyCandidateRemainsAvailableForManualChoice(t *testing.T) {
	candidate := lyricsCandidateFromLRCLibModel(lrcLibModel{
		ID: 52, TrackName: "Home", ArtistName: "Example", PlainLyrics: "lyrics",
	}, LyricsSearchInfo{Title: "Home"})
	if !candidate.Accepted || candidate.ProviderTrackID != "52" {
		t.Fatalf("title-only candidate should remain manually selectable: %#v", candidate)
	}
	if _, _, ok := bestLRCLibModelForInfoWithModeScored([]lrcLibModel{{
		ID: 52, TrackName: "Home", ArtistName: "Example", PlainLyrics: "lyrics",
	}}, LyricsSearchInfo{Title: "Home"}, false); ok {
		t.Fatal("title-only candidate must not be selected automatically")
	}
}

func TestTrackLyricsCandidateFetchesStableIDWithoutAutomaticThreshold(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/get/42" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		return testHTTPResponse(request, http.StatusOK, `{
			"id":42,
			"trackName":"Manual choice",
			"artistName":"Different metadata",
			"duration":213,
			"syncedLyrics":"[00:01.00]chosen line"
		}`), nil
	})}

	result, err := client.TrackLyricsCandidate(context.Background(), "LRCLIB", "42", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != lyricsResultSynced || result.Source != "LRCLib" || len(result.Lines) != 2 {
		t.Fatalf("unexpected candidate lyrics: %#v", result)
	}
}

func TestTrackLyricsCandidateFetchesAMLLStableRawID(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "raw.githubusercontent.com" || request.URL.Path != "/amll-dev/amll-ttml-db/refs/heads/main/raw-lyrics/1689089845000-39523898-31c2fa0c.ttml" {
			t.Fatalf("unexpected AMLL candidate URL: %s", request.URL)
		}
		return testHTTPResponse(request, http.StatusOK, `<tt><body><div><p begin="1.000" end="2.000"><span begin="1.000" end="2.000">chosen</span></p></div></body></tt>`), nil
	})}

	result, err := client.TrackLyricsCandidate(
		context.Background(),
		"AMLL_TTML_DB",
		"1689089845000-39523898-31c2fa0c.ttml",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != lyricsResultSynced || result.ProviderID != lyricsProviderAMLL || result.ProviderTrackID != "1689089845000-39523898-31c2fa0c.ttml" {
		t.Fatalf("unexpected AMLL manual candidate: %#v", result)
	}
}

func TestTrackLyricsCandidateRejectsUnsupportedProviderAndMismatchedID(t *testing.T) {
	client := NewClient(nil)
	if _, err := client.TrackLyricsCandidate(context.Background(), "unknown", "42", false); err == nil {
		t.Fatal("expected unsupported provider error")
	}

	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return testHTTPResponse(request, http.StatusOK, `{"id":43,"plainLyrics":"wrong"}`), nil
	})}
	if _, err := client.TrackLyricsCandidate(context.Background(), "lrclib", "42", false); err == nil {
		t.Fatal("expected candidate identity mismatch")
	}
}

func TestSearchLyricsCandidatesPreservesProviderStatusError(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return testHTTPResponse(request, http.StatusTooManyRequests, `{}`), nil
	})}
	_, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{Title: "Track"})
	if err == nil || err.Error() != "lrclib api status 429" {
		t.Fatalf("expected provider status evidence, got %v", err)
	}
}

func TestSearchLyricsCandidatesAcceptedCanonicalSkipsSearchVariants(t *testing.T) {
	client := NewClient(nil)
	var alternateCalls atomic.Int32
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "lrclib.net":
			title := request.URL.Query().Get("track_name")
			if title == "Alternate" {
				alternateCalls.Add(1)
				return testHTTPResponse(request, http.StatusOK, `[]`), nil
			}
			return testHTTPResponse(request, http.StatusOK, `[{
				"id":42,
				"trackName":"Canonical",
				"artistName":"Artist",
				"albumName":"Album",
				"duration":213,
				"syncedLyrics":"[00:01.00]line"
			}]`), nil
		case "amlldb.bikonoo.com":
			if lyricsCandidateTestAMLLQuery(request) == "Alternate" {
				alternateCalls.Add(1)
			}
			return testHTTPResponse(request, http.StatusOK, `[{
				"platform":"raw-lyrics",
				"file":"1689089845000-39523898-31c2fa0c.ttml",
				"title":"Canonical",
				"artists":["Artist"],
				"albums":["Album"]
			}]`), nil
		default:
			return testHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
		}
	})}

	candidates, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title:           "Canonical",
		Artist:          "Artist",
		Album:           "Album",
		DurationSeconds: 213,
		SearchVariants: []LyricsSearchVariant{
			{Title: "Alternate", Artist: "Alternate Artist"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	accepted := false
	for _, candidate := range candidates {
		accepted = accepted || candidate.Accepted
	}
	if !accepted {
		t.Fatalf("canonical search did not return an accepted candidate: %#v", candidates)
	}
	if alternateCalls.Load() != 0 {
		t.Fatalf("accepted canonical candidate triggered %d alternate provider requests", alternateCalls.Load())
	}
}

func TestSearchLyricsCandidatesReusesAMLLTitleRecordsAcrossArtistVariants(t *testing.T) {
	client := NewClient(nil)
	var amllCalls atomic.Int32
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "lrclib.net":
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		case "amlldb.bikonoo.com":
			amllCalls.Add(1)
			if query := lyricsCandidateTestAMLLQuery(request); query != "Shared Title" {
				return testHTTPResponse(request, http.StatusBadRequest, `{}`), nil
			}
			return testHTTPResponse(request, http.StatusOK, `[{
				"platform":"raw-lyrics",
				"file":"1689089845000-39523898-31c2fa0c.ttml",
				"title":"Shared Title",
				"artists":["Target Artist"],
				"albums":["Album"]
			}]`), nil
		default:
			return testHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
		}
	})}

	candidates, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title:  "Shared Title",
		Artist: "Unrelated Canonical Artist",
		Album:  "Album",
		SearchVariants: []LyricsSearchVariant{
			{Title: "Shared Title", Artist: "Target Artist"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if amllCalls.Load() != 1 {
		t.Fatalf("same-title identities issued %d AMLL community requests, want 1", amllCalls.Load())
	}
	for _, candidate := range candidates {
		if candidate.ProviderID == lyricsProviderAMLL &&
			candidate.ProviderTrackID == "1689089845000-39523898-31c2fa0c.ttml" {
			if !candidate.Accepted || candidate.Artist != "Target Artist" {
				t.Fatalf("AMLL records were not rescored against the alternate identity: %#v", candidate)
			}
			return
		}
	}
	t.Fatalf("missing AMLL candidate scored by the alternate artist: %#v", candidates)
}

func TestSearchLyricsCandidatesPreservesCanonicalLRCLIBErrorAcrossEmptyAlternate(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "lrclib.net":
			if request.URL.Query().Get("track_name") == "Canonical" {
				return testHTTPResponse(request, http.StatusTooManyRequests, `{}`), nil
			}
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		case "amlldb.bikonoo.com":
			// Keep AMLL out of the result without invoking its catalog fallback:
			// the response is non-empty, but its raw lyric filename is invalid.
			return testHTTPResponse(request, http.StatusOK, `[{
				"platform":"raw-lyrics",
				"file":"invalid.ttml",
				"title":"Unrelated"
			}]`), nil
		default:
			return testHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
		}
	})}

	_, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title:  "Canonical",
		Artist: "Artist",
		SearchVariants: []LyricsSearchVariant{
			{Title: "Alternate", Artist: "Artist"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "lrclib api status 429") {
		t.Fatalf("empty successful alternate swallowed canonical LRCLIB failure: %v", err)
	}
}

func TestSearchLyricsCandidatesVariantRequestBudget(t *testing.T) {
	client := NewClient(nil)
	var lrclibCalls atomic.Int32
	var amllCalls atomic.Int32
	var observedMu sync.Mutex
	fullLRCLIBIdentities := make(map[string]bool)
	amllTitles := make(map[string]bool)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "lrclib.net":
			lrclibCalls.Add(1)
			values := request.URL.Query()
			if values.Get("album_name") != "Album" {
				return testHTTPResponse(request, http.StatusOK, `[]`), nil
			}
			identity := values.Get("track_name") + "\x00" + values.Get("artist_name")
			observedMu.Lock()
			fullLRCLIBIdentities[identity] = true
			observedMu.Unlock()
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		case "amlldb.bikonoo.com":
			amllCalls.Add(1)
			title := lyricsCandidateTestAMLLQuery(request)
			observedMu.Lock()
			amllTitles[title] = true
			observedMu.Unlock()
			return testHTTPResponse(request, http.StatusOK, `[{
				"platform":"raw-lyrics",
				"file":"invalid.ttml",
				"title":"Unrelated"
			}]`), nil
		default:
			return testHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
		}
	})}

	variants := make([]LyricsSearchVariant, 0, 8)
	for index := 1; index <= 8; index++ {
		variants = append(variants, LyricsSearchVariant{
			Title:  "Title " + string(rune('0'+index)),
			Artist: "Artist " + string(rune('0'+index)) + " feat. Guest " + string(rune('0'+index)),
		})
	}
	_, err := client.SearchLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title:          "Title 0",
		Artist:         "Artist 0 feat. Guest 0",
		Album:          "Album",
		SearchVariants: variants,
	})
	if err != nil {
		t.Fatal(err)
	}

	observedMu.Lock()
	fullIdentityCount := len(fullLRCLIBIdentities)
	amllTitleCount := len(amllTitles)
	observedMu.Unlock()
	identityBudget := 1 + maxLyricsSearchVariants
	if fullIdentityCount != identityBudget {
		t.Fatalf("observed %d full LRCLIB identities, want bounded canonical + variants = %d", fullIdentityCount, identityBudget)
	}
	if amllTitleCount != identityBudget || int(amllCalls.Load()) != identityBudget {
		t.Fatalf("AMLL title dedup/budget mismatch: titles=%d HTTP=%d want=%d", amllTitleCount, amllCalls.Load(), identityBudget)
	}
	// Each LRCLIB identity currently has a deterministic four-step relaxation
	// ladder (full, no album, primary artist, title only).
	lrclibHTTPBudget := identityBudget * maxLRCLibCandidateRequestsPerIdentity
	if int(lrclibCalls.Load()) > lrclibHTTPBudget {
		t.Fatalf("LRCLIB issued %d HTTP requests, upper bound is %d", lrclibCalls.Load(), lrclibHTTPBudget)
	}
	fullHTTPBudget := identityBudget *
		(maxLRCLibCandidateRequestsPerIdentity + maxAMLLCandidateRequestsPerTitle)
	if maxLyricsCandidateHTTPRequests != fullHTTPBudget {
		t.Fatalf("candidate HTTP budget drifted: got %d want %d", maxLyricsCandidateHTTPRequests, fullHTTPBudget)
	}
}

func lyricsCandidateTestAMLLQuery(request *http.Request) string {
	var payload struct {
		Query string `json:"query"`
	}
	if request.Body == nil {
		return ""
	}
	_ = json.NewDecoder(request.Body).Decode(&payload)
	return payload.Query
}

func TestEnhancedLRCWordsPreserveExactEndsAndSpacingSemantics(t *testing.T) {
	lines := parseLRCLines("[00:01.00]<00:01.00>Hello <00:01.50>世界<00:02.00>")
	if len(lines) != 2 || len(lines[1].Words) != 2 {
		t.Fatalf("unexpected enhanced LRC lines: %#v", lines)
	}
	words := lines[1].Words
	if words[0].EndMs != 1500 || words[0].EndsWithSpace == nil || !*words[0].EndsWithSpace {
		t.Fatalf("expected first word exact end and trailing space: %#v", words[0])
	}
	if words[1].EndMs != 2000 || words[1].EndsWithSpace == nil || *words[1].EndsWithSpace {
		t.Fatalf("expected explicit no-space semantics on final word: %#v", words[1])
	}
}

func TestEnrichLyricsResultSeparatesProviderFromAttribution(t *testing.T) {
	result := enrichLyricsResult(LyricsResult{
		Kind:   lyricsResultPlain,
		Source: "LyricFind",
		Text:   "plain",
	})
	if result.ProviderID != "youtube_music" || result.Attribution != "LyricFind" || result.TimingQuality != "plain" {
		t.Fatalf("unexpected provider metadata: %#v", result)
	}

	lrclib := enrichLyricsResult(lrcLibModelLyricsResult(lrcLibModel{
		ID:          42,
		PlainLyrics: "plain",
	}, true))
	if lrclib.ProviderID != "lrclib" || lrclib.ProviderTrackID != "42" || lrclib.Attribution != lyricsAttributionLRCLib {
		t.Fatalf("unexpected LRCLIB metadata: %#v", lrclib)
	}
}
