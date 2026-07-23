package youtubemusic

import (
	"context"
	"net/http"
	"testing"
)

func TestNormalizeLyricsIdentityRemovesOnlyPresentationNoise(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "official audio", input: "我知道 (Official Audio)", want: "我知道"},
		{name: "official music video", input: "Song - Official Music Video", want: "Song"},
		{name: "lyrics video", input: "Song [Lyrics Video]", want: "Song"},
		{name: "visualizer", input: "Song (Visualizer)", want: "Song"},
		{name: "chinese official mv", input: "風箏【官方MV】", want: "風箏"},
		{name: "joined chinese lyric video", input: "風箏歌詞版", want: "風箏"},
		{name: "live is semantic", input: "Song (Live)", want: "Song (Live)"},
		{name: "remix is semantic", input: "Song - Remix", want: "Song - Remix"},
		{name: "acoustic is semantic", input: "Song (Acoustic)", want: "Song (Acoustic)"},
		{name: "instrumental is semantic", input: "Song (Instrumental)", want: "Song (Instrumental)"},
		{name: "officially is title text", input: "Officially Missing You", want: "Officially Missing You"},
		{name: "video is title text", input: "Video Games", want: "Video Games"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeLyricsIdentityTitle(test.input); got != test.want {
				t.Fatalf("normalize title %q: want %q, got %q", test.input, test.want, got)
			}
		})
	}
}

func TestNormalizeLyricsIdentityRemovesAnchoredYouTubeTopicArtist(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "By2 - Topic", want: "By2"},
		{input: "弦子 - Topic", want: "弦子"},
		{input: "Artist—Topic", want: "Artist"},
		{input: "Topic", want: "Topic"},
		{input: "Topic Ensemble", want: "Topic Ensemble"},
		{input: "Artist - Topic Sessions", want: "Artist - Topic Sessions"},
	}
	for _, test := range tests {
		if got := normalizeLyricsIdentityArtist(test.input); got != test.want {
			t.Fatalf("normalize artist %q: want %q, got %q", test.input, test.want, got)
		}
	}
}

func TestBuildLRCLibSearchQueryVariantsUsesBoundedRelaxationOrder(t *testing.T) {
	variants := buildLRCLibSearchQueryVariants(LyricsSearchInfo{
		Title:  "我知道 (Official Audio)",
		Artist: "By2 feat. Guest - Topic",
		Album:  "Wrong compilation",
	})
	if len(variants) != 4 {
		t.Fatalf("expected four bounded variants, got %#v", variants)
	}
	queries := make([]string, 0, len(variants))
	for _, variant := range variants {
		queries = append(queries, buildLRCLibSearchQuery(variant.info).Encode())
	}
	wants := []struct {
		title     string
		artist    string
		album     string
		titleOnly bool
	}{
		{title: "我知道", artist: "By2 feat. Guest", album: "Wrong compilation"},
		{title: "我知道", artist: "By2 feat. Guest"},
		{title: "我知道", artist: "By2"},
		{title: "我知道", titleOnly: true},
	}
	for index, want := range wants {
		variant := variants[index]
		if variant.info.Title != want.title || variant.info.Artist != want.artist ||
			variant.info.Album != want.album || variant.titleOnly != want.titleOnly {
			t.Fatalf("variant %d mismatch: query=%q variant=%#v", index, queries[index], variant)
		}
	}
}

func TestScoreLRCLibCandidateRequiresStrongDurationForCrossScriptArtist(t *testing.T) {
	duration := 213.0
	model := lrcLibModel{
		ID:           1,
		TrackName:    "晴天",
		ArtistName:   "周杰伦",
		Duration:     &duration,
		SyncedLyrics: "[00:01.00]line",
	}

	match, accepted := scoreLRCLibCandidate(model, LyricsSearchInfo{
		Title: "晴天", Artist: "Jay Chou", DurationSeconds: duration,
	})
	if !accepted || match.titleScore != 1 || match.durationScore != 1 || match.artistScore != 0 {
		t.Fatalf("strong duration should corroborate an exact cross-script title: accepted=%t match=%+v", accepted, match)
	}

	weak := 216.0
	model.Duration = &weak
	match, accepted = scoreLRCLibCandidate(model, LyricsSearchInfo{
		Title: "晴天", Artist: "Jay Chou", DurationSeconds: duration,
	})
	if accepted || match.rejection != "artist mismatch" {
		t.Fatalf("weak duration must not auto-match cross-script artists: accepted=%t match=%+v", accepted, match)
	}

	model.Duration = &duration
	match, accepted = scoreLRCLibCandidate(model, LyricsSearchInfo{
		Title: "晴天", Artist: "Jay Chou",
	})
	if accepted || match.rejection != "artist mismatch" {
		t.Fatalf("missing duration must keep cross-script aliases manual: accepted=%t match=%+v", accepted, match)
	}
}

func TestSearchLRCLibCandidatesCleansRealTopicExamplesAtProviderBoundary(t *testing.T) {
	tests := []struct {
		title  string
		artist string
	}{
		{title: "我知道", artist: "By2 - Topic"},
		{title: "風箏", artist: "弦子 - Topic"},
	}
	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			client := NewClient(nil)
			calls := 0
			client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if got := request.URL.Query().Get("track_name"); got != test.title {
					t.Fatalf("unexpected track query: %q", got)
				}
				wantArtist := normalizeLyricsIdentityArtist(test.artist)
				if got := request.URL.Query().Get("artist_name"); got != wantArtist {
					t.Fatalf("Topic suffix reached provider: want %q, got %q", wantArtist, got)
				}
				body := `[{"id":42,"trackName":"` + test.title + `","artistName":"` + wantArtist + `","duration":213,"syncedLyrics":"[00:01.00]line"}]`
				return testHTTPResponse(request, http.StatusOK, body), nil
			})}

			candidates, err := client.searchLRCLibLyricsCandidates(context.Background(), LyricsSearchInfo{
				Title: test.title, Artist: test.artist, DurationSeconds: 213,
			})
			if err != nil || len(candidates) != 1 || !candidates[0].Accepted || calls != 1 {
				t.Fatalf("real Topic example did not match: candidates=%#v calls=%d err=%v", candidates, calls, err)
			}
		})
	}
}

func TestSearchLRCLibCandidatesCleansPresentationTitleAtProviderBoundary(t *testing.T) {
	client := NewClient(nil)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.URL.Query().Get("track_name"); got != "我知道" {
			t.Fatalf("presentation noise reached provider: %q", got)
		}
		return testHTTPResponse(request, http.StatusOK, `[{"id":42,"trackName":"我知道","artistName":"By2","duration":213,"syncedLyrics":"[00:01.00]line"}]`), nil
	})}

	candidates, err := client.searchLRCLibLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title: "我知道 (Official Audio)", Artist: "By2 - Topic", DurationSeconds: 213,
	})
	if err != nil || len(candidates) != 1 || !candidates[0].Accepted {
		t.Fatalf("presentation-cleaned provider query did not match: candidates=%#v err=%v", candidates, err)
	}
}

func TestSearchLRCLibCandidatesRelaxesAlbumAndPrimaryArtistAndDeduplicates(t *testing.T) {
	client := NewClient(nil)
	queries := make([]string, 0, 3)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		queries = append(queries, request.URL.RawQuery)
		query := request.URL.Query()
		switch {
		case query.Get("album_name") != "":
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		case query.Get("artist_name") == "Artist feat. Guest":
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		case query.Get("artist_name") == "Artist":
			return testHTTPResponse(request, http.StatusOK, `[
				{"id":42,"trackName":"Song","artistName":"Artist","duration":213,"syncedLyrics":"[00:01.00]line"},
				{"id":42,"trackName":"Song","artistName":"Artist","duration":213,"syncedLyrics":"[00:01.00]duplicate"}
			]`), nil
		default:
			t.Fatalf("unexpected relaxed query: %s", request.URL.RawQuery)
			return nil, nil
		}
	})}

	candidates, err := client.searchLRCLibLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title: "Song", Artist: "Artist feat. Guest", Album: "Wrong", DurationSeconds: 213,
	})
	if err != nil || len(candidates) != 1 || !candidates[0].Accepted {
		t.Fatalf("primary artist relaxation failed: candidates=%#v err=%v", candidates, err)
	}
	if len(queries) != 3 {
		t.Fatalf("expected bounded full/no-album/primary queries, got %#v", queries)
	}
}

func TestSearchLRCLibCandidatesAllowsDurationBackedCrossScriptTitleOnlyRecall(t *testing.T) {
	client := NewClient(nil)
	calls := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Query().Get("artist_name") != "" {
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		}
		return testHTTPResponse(request, http.StatusOK, `[{"id":42,"trackName":"晴天","artistName":"周杰伦","duration":213,"syncedLyrics":"[00:01.00]line"}]`), nil
	})}

	candidates, err := client.searchLRCLibLyricsCandidates(context.Background(), LyricsSearchInfo{
		Title: "晴天", Artist: "Jay Chou", DurationSeconds: 213,
	})
	if err != nil || len(candidates) != 1 || calls != 2 {
		t.Fatalf("title-only fallback failed: candidates=%#v calls=%d err=%v", candidates, calls, err)
	}
	if !candidates[0].Accepted || candidates[0].TitleScore != 100 || candidates[0].DurationScore != 100 {
		t.Fatalf("duration-backed cross-script title-only recall must be automatic: %#v", candidates[0])
	}
}

func TestTitleOnlyLRCLibAutomaticEligibilityIsNarrow(t *testing.T) {
	targetDuration := 213.0
	tests := []struct {
		name  string
		info  LyricsSearchInfo
		model lrcLibModel
		want  bool
	}{
		{
			name:  "cross script exact title and duration",
			info:  LyricsSearchInfo{Title: "晴天", Artist: "Jay Chou", DurationSeconds: targetDuration},
			model: lrcLibModel{TrackName: "晴天", ArtistName: "周杰伦", Duration: float64Pointer(targetDuration)},
			want:  true,
		},
		{
			name:  "same script artist mismatch",
			info:  LyricsSearchInfo{Title: "Song", Artist: "Artist One", DurationSeconds: targetDuration},
			model: lrcLibModel{TrackName: "Song", ArtistName: "Artist Two", Duration: float64Pointer(targetDuration)},
		},
		{
			name:  "same script artist match remains ordinary title only",
			info:  LyricsSearchInfo{Title: "Song", Artist: "Artist", DurationSeconds: targetDuration},
			model: lrcLibModel{TrackName: "Song", ArtistName: "Artist", Duration: float64Pointer(targetDuration)},
		},
		{
			name:  "missing target duration",
			info:  LyricsSearchInfo{Title: "晴天", Artist: "Jay Chou"},
			model: lrcLibModel{TrackName: "晴天", ArtistName: "周杰伦", Duration: float64Pointer(targetDuration)},
		},
		{
			name:  "missing provider duration",
			info:  LyricsSearchInfo{Title: "晴天", Artist: "Jay Chou", DurationSeconds: targetDuration},
			model: lrcLibModel{TrackName: "晴天", ArtistName: "周杰伦"},
		},
		{
			name:  "duration outside two seconds",
			info:  LyricsSearchInfo{Title: "晴天", Artist: "Jay Chou", DurationSeconds: targetDuration},
			model: lrcLibModel{TrackName: "晴天", ArtistName: "周杰伦", Duration: float64Pointer(216)},
		},
		{
			name:  "non exact title",
			info:  LyricsSearchInfo{Title: "晴天 Live", Artist: "Jay Chou", DurationSeconds: targetDuration},
			model: lrcLibModel{TrackName: "晴天", ArtistName: "周杰伦", Duration: float64Pointer(targetDuration)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := titleOnlyLRCLibAutomaticEligible(test.model, test.info); got != test.want {
				t.Fatalf("eligibility=%t, want %t", got, test.want)
			}
		})
	}
}

func TestAutomaticLRCLibSearchRelaxesWrongAlbumAndSafelyLimitsTitleOnly(t *testing.T) {
	t.Run("no album fallback", func(t *testing.T) {
		client := NewClient(nil)
		searchCalls := 0
		client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.Path {
			case "/api/get-cached":
				return testHTTPResponse(request, http.StatusNotFound, `{}`), nil
			case "/api/search":
				searchCalls++
				if request.URL.Query().Get("album_name") != "" {
					return testHTTPResponse(request, http.StatusOK, `[]`), nil
				}
				return testHTTPResponse(request, http.StatusOK, `[{"id":42,"trackName":"Song","artistName":"Artist","duration":213,"syncedLyrics":"[00:01.00]line"}]`), nil
			default:
				t.Fatalf("unexpected path: %s", request.URL.Path)
				return nil, nil
			}
		})}
		result := client.searchLRCLibLyrics(context.Background(), LyricsSearchInfo{
			Title: "Song", Artist: "Artist", Album: "Wrong", DurationSeconds: 213,
		})
		if result.Kind != lyricsResultSynced || result.ProviderTrackID != "42" || searchCalls != 2 {
			t.Fatalf("automatic no-album fallback failed: result=%#v calls=%d", result, searchCalls)
		}
	})

	t.Run("duration backed cross script title only is automatic", func(t *testing.T) {
		client := NewClient(nil)
		client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.Path {
			case "/api/get-cached":
				return testHTTPResponse(request, http.StatusNotFound, `{}`), nil
			case "/api/search":
				if request.URL.Query().Get("artist_name") != "" {
					return testHTTPResponse(request, http.StatusOK, `[]`), nil
				}
				return testHTTPResponse(request, http.StatusOK, `[{"id":42,"trackName":"晴天","artistName":"周杰伦","duration":213,"syncedLyrics":"[00:01.00]line"}]`), nil
			default:
				t.Fatalf("unexpected path: %s", request.URL.Path)
				return nil, nil
			}
		})}
		result := client.searchLRCLibLyrics(context.Background(), LyricsSearchInfo{
			Title: "晴天", Artist: "Jay Chou", DurationSeconds: 213,
		})
		if result.Kind != lyricsResultSynced || result.ProviderTrackID != "42" {
			t.Fatalf("automatic search missed safe cross-script title-only recall: %#v", result)
		}
	})
}

func float64Pointer(value float64) *float64 {
	return &value
}
