package http

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

func TestDreamFMTrackFavoriteHandlerReadsFavoriteStatus(t *testing.T) {
	handler := NewDreamFMTrackFavoriteHandler(fakeDreamFMMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:         "TESTVID007G",
			LikeStatus:      youtubemusic.LikeStatusLike,
			LikeStatusKnown: true,
		},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/track/favorite?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if !strings.Contains(recorder.Body.String(), `"liked":true`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"known":true`) {
		t.Fatalf("expected known status in response body: %s", recorder.Body.String())
	}
}

func TestDreamFMTrackFavoriteHandlerWritesFavoriteStatus(t *testing.T) {
	var gotVideoID string
	var gotRating youtubemusic.LikeStatus
	handler := NewDreamFMTrackFavoriteHandler(fakeDreamFMMusicClient{
		rateSongFunc: func(_ context.Context, videoID string, rating youtubemusic.LikeStatus) error {
			gotVideoID = videoID
			gotRating = rating
			return nil
		},
	})
	request := httptest.NewRequest("POST", "/api/dreamfm/track/favorite", strings.NewReader(`{"videoId":"TESTVID007G","liked":true}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if gotVideoID != "TESTVID007G" {
		t.Fatalf("unexpected video id: %q", gotVideoID)
	}
	if gotRating != youtubemusic.LikeStatusLike {
		t.Fatalf("unexpected rating: %q", gotRating)
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestDreamFMTrackFavoriteHandlerFallsBackToCachedStatus(t *testing.T) {
	handler := NewDreamFMTrackFavoriteHandler(fakeDreamFMMusicClient{
		trackMetadataErr: errors.New("metadata unavailable"),
	})
	handler.setCachedFavorite("TESTVID007G", true)
	request := httptest.NewRequest("GET", "/api/dreamfm/track/favorite?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if !strings.Contains(recorder.Body.String(), `"liked":true`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestDreamFMTrackFavoriteHandlerReportsUnknownUncachedStatus(t *testing.T) {
	handler := NewDreamFMTrackFavoriteHandler(fakeDreamFMMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:         "TESTVID007G",
			LikeStatus:      youtubemusic.LikeStatusIndifferent,
			LikeStatusKnown: false,
		},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/track/favorite?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"liked":false`) || !strings.Contains(body, `"known":false`) {
		t.Fatalf("unexpected response body: %s", body)
	}
}

func TestDreamFMTrackFavoriteHandlerPrimesBatchFromLikedSongs(t *testing.T) {
	handler := NewDreamFMTrackFavoriteHandler(fakeDreamFMMusicClient{
		likedSongs: []youtubemusic.Track{{
			VideoID: "TESTVID007G",
			Title:   "Liked Song",
		}},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/track/favorite?ids=TESTVID007G,TESTVID011K", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"videoId":"TESTVID007G"`) || !strings.Contains(body, `"liked":true`) || !strings.Contains(body, `"known":true`) {
		t.Fatalf("expected liked song in response body: %s", body)
	}
}
