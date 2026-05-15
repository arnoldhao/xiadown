package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

func TestListenPlaylistHandlerServesPlayableQueue(t *testing.T) {
	handler := NewListenPlaylistHandler(fakeListenMusicClient{
		playlistTracks: []youtubemusic.Track{{
			VideoID:        "TESTVID009I",
			Title:          "Night Drive",
			Channel:        "Dream FM",
			DurationLabel:  "4:20",
			PlayCountLabel: "1.2M plays",
		}},
	})
	request := httptest.NewRequest("GET", "/api/listen/playlist?id=VLPL1234567890", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"id":"ytmusic-playlist-track-TESTVID009I"`) || !strings.Contains(body, `"playCountLabel":"1.2M plays"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestListenPlaylistHandlerRejectsPodcastShows(t *testing.T) {
	handler := NewListenPlaylistHandler(fakeListenMusicClient{
		playlistTracks: []youtubemusic.Track{{
			VideoID: "TESTVID009I",
			Title:   "Podcast Episode",
		}},
	})
	request := httptest.NewRequest("GET", "/api/listen/playlist?id=MPSPPpodcast", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Result().StatusCode)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"code":"unsupported_playlist_id"`) {
		t.Fatalf("expected unsupported podcast playlist error: %s", body)
	}
}

func TestListenPlaylistHandlerServesContinuation(t *testing.T) {
	handler := NewListenPlaylistHandler(fakeListenMusicClient{
		playlistPage: youtubemusic.TrackListPage{
			Continuation: "next-page",
			Tracks: []youtubemusic.Track{{
				VideoID:       "TESTVID009I",
				Title:         "Night Drive",
				Channel:       "Dream FM",
				DurationLabel: "4:20",
			}},
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/playlist?id=VLPL1234567890&continuation=token", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"continuation":"next-page"`) || !strings.Contains(body, `"id":"ytmusic-playlist-track-TESTVID009I"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestListenPlaylistHandlerServesPlaylistAuthor(t *testing.T) {
	handler := NewListenPlaylistHandler(fakeListenMusicClient{
		playlistPage: youtubemusic.TrackListPage{
			Title:  "Midnight Album",
			Author: "Album Artist",
			Tracks: []youtubemusic.Track{{
				VideoID:       "TESTVID009I",
				Title:         "Night Drive",
				DurationLabel: "4:20",
			}},
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/playlist?id=MPREalbum123", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"title":"Midnight Album"`) || !strings.Contains(body, `"author":"Album Artist"`) {
		t.Fatalf("expected playlist metadata in body: %s", body)
	}
}
