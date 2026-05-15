package http

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

func TestListenTrackFavoriteHandlerReadsFavoriteStatus(t *testing.T) {
	handler := NewListenTrackFavoriteHandler(fakeListenMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:         "TESTVID007G",
			LikeStatus:      youtubemusic.LikeStatusLike,
			LikeStatusKnown: true,
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track/favorite?id=TESTVID007G", nil)
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

func TestListenTrackFavoriteHandlerWritesFavoriteStatus(t *testing.T) {
	var gotVideoID string
	var gotRating youtubemusic.LikeStatus
	handler := NewListenTrackFavoriteHandler(fakeListenMusicClient{
		rateSongFunc: func(_ context.Context, videoID string, rating youtubemusic.LikeStatus) error {
			gotVideoID = videoID
			gotRating = rating
			return nil
		},
	})
	request := httptest.NewRequest("POST", "/api/listen/track/favorite", strings.NewReader(`{"videoId":"TESTVID007G","liked":true}`))
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

func TestListenTrackFavoriteHandlerFallsBackToCachedStatus(t *testing.T) {
	handler := NewListenTrackFavoriteHandler(fakeListenMusicClient{
		trackMetadataErr: errors.New("metadata unavailable"),
	})
	handler.setCachedFavorite("default", "TESTVID007G", true)
	request := httptest.NewRequest("GET", "/api/listen/track/favorite?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if !strings.Contains(recorder.Body.String(), `"liked":true`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestListenTrackFavoriteHandlerPrefersCachedStatusOverStaleMetadata(t *testing.T) {
	handler := NewListenTrackFavoriteHandler(fakeListenMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:         "TESTVID007G",
			LikeStatus:      youtubemusic.LikeStatusLike,
			LikeStatusKnown: true,
		},
	})
	handler.setCachedFavorite("default", "TESTVID007G", false)
	request := httptest.NewRequest("GET", "/api/listen/track/favorite?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if !strings.Contains(recorder.Body.String(), `"liked":false`) {
		t.Fatalf("expected cached favorite status, got: %s", recorder.Body.String())
	}
}

func TestListenTrackFavoriteHandlerReportsUnknownUncachedStatus(t *testing.T) {
	handler := NewListenTrackFavoriteHandler(fakeListenMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:         "TESTVID007G",
			LikeStatus:      youtubemusic.LikeStatusIndifferent,
			LikeStatusKnown: false,
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track/favorite?id=TESTVID007G", nil)
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

func TestListenTrackFavoriteHandlerBatchKeepsCachedUnlikeDuringLikedSongsLag(t *testing.T) {
	handler := NewListenTrackFavoriteHandler(fakeListenMusicClient{
		likedSongs: []youtubemusic.Track{{
			VideoID: "TESTVID007G",
			Title:   "Liked Song",
		}},
	})
	handler.setCachedFavorite("default", "TESTVID007G", false)
	request := httptest.NewRequest("GET", "/api/listen/track/favorite?ids=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if !strings.Contains(recorder.Body.String(), `"liked":false`) {
		t.Fatalf("expected cached unlike status, got: %s", recorder.Body.String())
	}
}

func TestListenTrackFavoriteHandlerPrimesBatchFromLikedSongs(t *testing.T) {
	handler := NewListenTrackFavoriteHandler(fakeListenMusicClient{
		likedSongs: []youtubemusic.Track{{
			VideoID: "TESTVID007G",
			Title:   "Liked Song",
		}},
	})
	request := httptest.NewRequest("GET", "/api/listen/track/favorite?ids=TESTVID007G,TESTVID011K", nil)
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
