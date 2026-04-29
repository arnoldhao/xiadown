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

func TestDreamFMLyricsHandlerReturnsSyncedLyrics(t *testing.T) {
	handler := NewDreamFMLyricsHandler(fakeDreamFMMusicClient{
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
	request := httptest.NewRequest("GET", "/api/dreamfm/track/lyrics?id=TESTVID007G&title=Track&artist=Artist&duration=213", nil)
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

func TestDreamFMLyricsHandlerAllowsLocalTitleSearch(t *testing.T) {
	var got youtubemusic.LyricsSearchInfo
	handler := NewDreamFMLyricsHandler(fakeDreamFMMusicClient{
		trackLyricsFunc: func(_ context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error) {
			got = info
			return youtubemusic.LyricsResult{
				Kind:   "plain",
				Source: "LRCLib",
				Text:   "Local lyrics",
			}, nil
		},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/track/lyrics?key=local%3Afile-1&title=Local+Track&artist=Local+Artist&duration=213", nil)
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

func TestDreamFMLyricsHandlerReturnsStructuredErrorDetails(t *testing.T) {
	handler := NewDreamFMLyricsHandler(fakeDreamFMMusicClient{
		trackLyricsErr: errors.New("youtube music api status 404: requested entity was not found"),
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/track/lyrics?id=TESTVID007G", nil)
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

func TestDreamFMLyricsHandlerWrapsRetryableNetworkErrorCode(t *testing.T) {
	handler := NewDreamFMLyricsHandler(fakeDreamFMMusicClient{
		trackLyricsErr: errors.Join(youtubemusic.ErrNetworkUnavailable, io.EOF),
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/track/lyrics?id=TESTVID007G", nil)
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
