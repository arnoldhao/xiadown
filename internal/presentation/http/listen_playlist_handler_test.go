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

func TestListenPlaylistHandlerRejectsMissingPlaylistID(t *testing.T) {
	handler := NewListenPlaylistHandler(fakeListenMusicClient{})
	request := httptest.NewRequest("GET", "/api/listen/playlist", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Result().StatusCode)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"code":"invalid_playlist_id"`) {
		t.Fatalf("expected invalid playlist id error: %s", body)
	}
}

func TestListenPlaylistHandlerServesPodcastShows(t *testing.T) {
	for _, podcastID := range []string{"MPSPpodcast", "MPSPPpodcast"} {
		t.Run(podcastID, func(t *testing.T) {
			handler := NewListenPlaylistHandler(fakeListenMusicClient{
				playlistPage: youtubemusic.TrackListPage{
					Title:        "Night Talks",
					Author:       "Dream FM",
					Continuation: "episode-next",
					Tracks: []youtubemusic.Track{{
						VideoID:       "TESTVID009I",
						Title:         "Podcast Episode",
						Channel:       "Dream FM",
						DurationLabel: "42:10",
					}},
				},
			})
			request := httptest.NewRequest("GET", "/api/listen/playlist?id="+podcastID, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Result().StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", recorder.Result().StatusCode, recorder.Body.String())
			}
			body := recorder.Body.String()
			for _, expected := range []string{
				`"id":"ytmusic-playlist-track-TESTVID009I"`,
				`"title":"Night Talks"`,
				`"author":"Dream FM"`,
				`"continuation":"episode-next"`,
			} {
				if !strings.Contains(body, expected) {
					t.Fatalf("expected %s in podcast response: %s", expected, body)
				}
			}
		})
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
			Title:           "Midnight Album",
			Author:          "Album Artist",
			AuthorBrowseID:  "UCalbumartist",
			TrackCountLabel: "10 songs",
			DurationLabel:   "42 minutes",
			Description:     "An album made for late-night listening.",
			ThumbnailURL:    "https://lh3.googleusercontent.com/midnight-album",
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
	for _, expected := range []string{
		`"title":"Midnight Album"`,
		`"author":"Album Artist"`,
		`"authorBrowseId":"UCalbumartist"`,
		`"trackCountLabel":"10 songs"`,
		`"durationLabel":"42 minutes"`,
		`"description":"An album made for late-night listening."`,
		`"thumbnailUrl":"https://lh3.googleusercontent.com/midnight-album"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %s in playlist metadata response: %s", expected, body)
		}
	}
}
