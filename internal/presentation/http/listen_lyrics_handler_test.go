package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

type fakeLyricsCandidateHTTPClient struct {
	fakeListenMusicClient
	candidates      []youtubemusic.LyricsCandidate
	candidateLyrics youtubemusic.LyricsResult
	searchInfo      youtubemusic.LyricsSearchInfo
	providerID      string
	providerTrackID string
}

func (client *fakeLyricsCandidateHTTPClient) SearchLyricsCandidates(_ context.Context, info youtubemusic.LyricsSearchInfo) ([]youtubemusic.LyricsCandidate, error) {
	client.searchInfo = info
	return client.candidates, nil
}

func (client *fakeLyricsCandidateHTTPClient) TrackLyricsCandidate(_ context.Context, providerID string, providerTrackID string, _ bool) (youtubemusic.LyricsResult, error) {
	client.providerID = providerID
	client.providerTrackID = providerTrackID
	return client.candidateLyrics, nil
}

func TestListenLyricsHandlerReturnsSyncedLyrics(t *testing.T) {
	handler := NewListenLyricsHandler(fakeListenMusicClient{
		trackLyrics: youtubemusic.LyricsResult{
			Kind:   "synced",
			Source: "YTMusic",
			Lines: []youtubemusic.LyricLine{{
				StartMs:    1200,
				DurationMs: 3200,
				Text:       "First line",
			}},
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track/lyrics?id=TESTVID007G&title=Track&artist=Artist&duration=213", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`"videoId":"TESTVID007G"`,
		`"kind":"synced"`,
		`"source":"YTMusic"`,
		`"startMs":1200`,
		`"text":"First line"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, body)
		}
	}
}

func TestListenLyricsHandlerAllowsLocalTitleSearch(t *testing.T) {
	var got youtubemusic.LyricsSearchInfo
	handler := NewListenLyricsHandler(fakeListenMusicClient{
		trackLyricsFunc: func(_ context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error) {
			got = info
			return youtubemusic.LyricsResult{
				Kind:   "plain",
				Source: "LRCLib",
				Text:   "Local lyrics",
			}, nil
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track/lyrics?key=local%3Afile-1&title=Local+Track&artist=Local+Artist&duration=213", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if got.VideoID != "" || got.Title != "Local Track" || got.Artist != "Local Artist" || got.DurationSeconds != 213 {
		t.Fatalf("unexpected lyrics search info: %+v", got)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`"videoId":"local:file-1"`,
		`"kind":"plain"`,
		`"source":"LRCLib"`,
		`"text":"Local lyrics"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, body)
		}
	}
}

func TestListenLyricsHandlerPassesPlainOnlyMode(t *testing.T) {
	var got youtubemusic.LyricsSearchInfo
	handler := NewListenLyricsHandler(fakeListenMusicClient{
		trackLyricsFunc: func(_ context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error) {
			got = info
			return youtubemusic.LyricsResult{
				Kind:   "plain",
				Source: "YTMusic",
				Text:   "Plain lyrics",
			}, nil
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track/lyrics?id=TESTVID007G&title=Track&synced=false", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if !got.PlainOnly {
		t.Fatalf("expected plain-only lyrics search info, got %+v", got)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"kind":"plain"`) {
		t.Fatalf("expected plain response, got %s", body)
	}
}

func TestListenLyricsHandlerReturnsStructuredErrorDetails(t *testing.T) {
	handler := NewListenLyricsHandler(fakeListenMusicClient{
		trackLyricsErr: errors.New("youtube music api status 404: requested entity was not found"),
	})
	request := httptest.NewRequest("GET", "/api/listen/track/lyrics?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"lyrics_unavailable"`) ||
		!strings.Contains(body, `"detail":"youtube music api status 404: requested entity was not found"`) {
		t.Fatalf("expected structured lyrics error, got %s", body)
	}
}

func TestListenLyricsHandlerWrapsRetryableNetworkErrorCode(t *testing.T) {
	handler := NewListenLyricsHandler(fakeListenMusicClient{
		trackLyricsErr: errors.Join(youtubemusic.ErrNetworkUnavailable, io.EOF),
	})
	request := httptest.NewRequest("GET", "/api/listen/track/lyrics?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`"code":"youtube_network_unavailable"`,
		`"message":"YouTube Music lyrics network unavailable."`,
		`"retryable":true`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, body)
		}
	}
}

func TestListenLyricsHandlerSearchesAndPreviewsCandidates(t *testing.T) {
	client := &fakeLyricsCandidateHTTPClient{
		candidates: []youtubemusic.LyricsCandidate{{
			ProviderID:      "lrclib",
			ProviderTrackID: "42",
			Title:           "Track",
			Confidence:      98,
			Accepted:        true,
		}},
		candidateLyrics: youtubemusic.LyricsResult{
			Kind:            "synced",
			Source:          "LRCLib",
			ProviderID:      "lrclib",
			ProviderTrackID: "42",
			Attribution:     "LRCLIB contributors",
			TimingQuality:   "line",
			Lines:           []youtubemusic.LyricLine{{StartMs: 1000, Text: "chosen"}},
		},
	}
	handler := NewListenLyricsHandler(client)

	searchRequest := httptest.NewRequest("GET", "/api/listen/track/lyrics/candidates?title=Track&artist=Artist&album=Album&duration=213", nil)
	searchRecorder := httptest.NewRecorder()
	handler.ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusOK || !strings.Contains(searchRecorder.Body.String(), `"providerTrackId":"42"`) {
		t.Fatalf("unexpected candidate search response: %d %s", searchRecorder.Code, searchRecorder.Body.String())
	}
	if client.searchInfo.Album != "Album" || client.searchInfo.DurationSeconds != 213 {
		t.Fatalf("candidate identity fields were not forwarded: %#v", client.searchInfo)
	}

	previewRequest := httptest.NewRequest("GET", "/api/listen/track/lyrics/candidate?provider=lrclib&providerTrackId=42&key=local%3Aone", nil)
	previewRecorder := httptest.NewRecorder()
	handler.ServeHTTP(previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected candidate preview status: %d %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	for _, expected := range []string{`"videoId":"local:one"`, `"timingQuality":"line"`, `"text":"chosen"`} {
		if !strings.Contains(previewRecorder.Body.String(), expected) {
			t.Fatalf("expected preview to contain %s, got %s", expected, previewRecorder.Body.String())
		}
	}
	if client.providerID != "lrclib" || client.providerTrackID != "42" {
		t.Fatalf("candidate identity not forwarded: %q %q", client.providerID, client.providerTrackID)
	}
}
