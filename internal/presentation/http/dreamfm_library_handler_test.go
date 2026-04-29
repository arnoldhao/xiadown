package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/connectors"
)

func TestDreamFMLibraryHandlerServesLibraryAndShelves(t *testing.T) {
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		libraryPlaylists: []youtubemusic.Playlist{{
			ID:           "VLPL1234567890",
			Title:        "My Library",
			Channel:      "Arnold",
			ThumbnailURL: "https://i.ytimg.com/vi/playlist/hqdefault.jpg",
		}},
		libraryArtists: []youtubemusic.Artist{{
			ID:           "UCsuperlofi",
			Name:         "Super Lofi World",
			Subtitle:     "Artist",
			ThumbnailURL: "https://lh3.googleusercontent.com/artist",
		}},
		likedSongs: []youtubemusic.Track{{
			VideoID:       "TESTVID001A",
			Title:         "Liked Track",
			Channel:       "Dream FM",
			DurationLabel: "3:33",
			ThumbnailURL:  "https://lh3.googleusercontent.com/liked",
		}},
		homeShelves: []youtubemusic.Shelf{
			{
				ID:    "Quick picks::tracks::TESTVID007G",
				Title: "Quick picks",
				Kind:  youtubemusic.ShelfTracks,
				Tracks: []youtubemusic.Track{{
					VideoID:       "TESTVID007G",
					Title:         "Lofi Mix",
					Channel:       "Super Lofi World",
					DurationLabel: "3:21",
					ThumbnailURL:  "https://lh3.googleusercontent.com/home",
				}},
			},
			{
				ID:    "Featured::playlists::VLPLfeedface",
				Title: "Featured",
				Kind:  youtubemusic.ShelfPlaylists,
				Playlists: []youtubemusic.Playlist{{
					ID:           "VLPLfeedface",
					Title:        "Late Night",
					Channel:      "Dream FM",
					ThumbnailURL: "https://i.ytimg.com/vi/late-night/hqdefault.jpg",
				}},
			},
		},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"playlistId":"VLPL1234567890"`) {
		t.Fatalf("expected library playlist in body: %s", body)
	}
	if !strings.Contains(body, `"browseId":"UCsuperlofi"`) {
		t.Fatalf("expected library artists in body: %s", body)
	}
	if !strings.Contains(body, `"id":"ytmusic-liked-songs"`) || !strings.Contains(body, `"title":"Liked Track"`) {
		t.Fatalf("expected liked songs shelf in body: %s", body)
	}
	if !strings.Contains(body, `"title":"Quick picks"`) || !strings.Contains(body, `"kind":"tracks"`) {
		t.Fatalf("expected track shelf in body: %s", body)
	}
	if !strings.Contains(body, `"playlistId":"VLPLfeedface"`) || !strings.Contains(body, `"kind":"playlists"`) {
		t.Fatalf("expected playlist shelf in body: %s", body)
	}
}

func TestDreamFMLibraryHandlerServesBrowseSourceShelves(t *testing.T) {
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		browseShelves: []youtubemusic.Shelf{{
			ID:    "Charts::tracks::TESTVID007G",
			Title: "Charts",
			Kind:  youtubemusic.ShelfTracks,
			Tracks: []youtubemusic.Track{{
				VideoID:       "TESTVID007G",
				Title:         "Chart Track",
				Channel:       "Dream FM",
				DurationLabel: "3:21",
			}},
		}},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library?source=charts", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"title":"Charts"`) || !strings.Contains(body, `"id":"ytmusic-charts-TESTVID007G"`) {
		t.Fatalf("expected browse source shelf in body: %s", body)
	}
}

func TestDreamFMLibraryHandlerServesChartTabShelves(t *testing.T) {
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		browsePageFunc: func(_ context.Context, browseID string, params string, _ string, _ int, _ int) (youtubemusic.BrowsePage, error) {
			switch params {
			case "songs":
				return youtubemusic.BrowsePage{Shelves: []youtubemusic.Shelf{{
					ID:    "Top songs::tracks::TopSong0001",
					Title: "Top songs",
					Kind:  youtubemusic.ShelfTracks,
					Tracks: []youtubemusic.Track{{
						VideoID: "TopSong0001",
						Title:   "Top Song",
						Channel: "Dream FM",
					}},
				}}}, nil
			case "artists":
				return youtubemusic.BrowsePage{Shelves: []youtubemusic.Shelf{{
					ID:    "Top artists::artists::UCsuperlofi",
					Title: "Top artists",
					Kind:  youtubemusic.ShelfArtists,
					Artists: []youtubemusic.Artist{{
						ID:   "UCsuperlofi",
						Name: "Super Lofi World",
					}},
				}}}, nil
			default:
				if browseID != "FEmusic_charts" {
					t.Fatalf("unexpected browse id: %s", browseID)
				}
				return youtubemusic.BrowsePage{
					Shelves: []youtubemusic.Shelf{{
						ID:    "Video charts::tracks::VideoChart1",
						Title: "Video charts",
						Kind:  youtubemusic.ShelfTracks,
						Tracks: []youtubemusic.Track{{
							VideoID: "VideoChart1",
							Title:   "Video Chart",
							Channel: "Dream FM",
						}},
					}},
					Tabs: []youtubemusic.BrowseTab{
						{Title: "Video charts", BrowseID: "FEmusic_charts", Selected: true},
						{Title: "Top songs", BrowseID: "FEmusic_charts", Params: "songs"},
						{Title: "Top artists", BrowseID: "FEmusic_charts", Params: "artists"},
					},
				}, nil
			}
		},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library?source=charts", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`"title":"Video charts"`,
		`"title":"Top songs"`,
		`"kind":"artists"`,
		`"browseId":"UCsuperlofi"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %s in body: %s", expected, body)
		}
	}
}

func TestDreamFMLibraryHandlerReturnsBrowseSourceErrorDetails(t *testing.T) {
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		browsePageErr: errors.New("youtube music api status 400: invalid browse id"),
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library?source=explore", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"youtube_music_unavailable"`) ||
		!strings.Contains(body, `"source":"explore"`) ||
		!strings.Contains(body, `"detail":"youtube music api status 400: invalid browse id"`) {
		t.Fatalf("expected structured browse error in body: %s", body)
	}
}

func TestDreamFMLibraryHandlerIgnoresPodcastSource(t *testing.T) {
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		libraryPodcasts: []youtubemusic.PodcastShow{{
			ID:     "MPSPPpodcast",
			Title:  "Night Talks",
			Author: "Dream FM",
		}},
		homeTracks: []youtubemusic.Track{{
			VideoID: "TESTVID007G",
			Title:   "Home Track",
			Channel: "Dream FM",
		}},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library?source=podcasts", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "MPSPPpodcast") || strings.Contains(body, `"kind":"podcasts"`) {
		t.Fatalf("expected podcast source to be ignored: %s", body)
	}
	if !strings.Contains(body, `"title":"Home Track"`) {
		t.Fatalf("expected home content in body: %s", body)
	}
}

func TestDreamFMLibraryHandlerReturnsAuthErrorDetails(t *testing.T) {
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		browsePageErr: youtubemusic.ErrAuthExpired,
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library?source=explore", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"youtube_auth_expired"`) ||
		!strings.Contains(body, `"message":"YouTube Music authentication expired."`) ||
		!strings.Contains(body, `"detail":"youtube music auth expired"`) {
		t.Fatalf("expected structured auth error in body: %s", body)
	}
}

func TestDreamFMLibraryHandlerReturnsMissingCookiesCode(t *testing.T) {
	missingCookiesErr := errors.Join(youtubemusic.ErrNotAuthenticated, connectors.ErrNoCookies)
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		libraryPlaylistsErr:    missingCookiesErr,
		libraryArtistsErr:      missingCookiesErr,
		likedSongsErr:          missingCookiesErr,
		browsePageErr:          missingCookiesErr,
		homeShelvesErr:         missingCookiesErr,
		homeRecommendationsErr: missingCookiesErr,
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"youtube_cookies_missing"`) ||
		!strings.Contains(body, `"message":"YouTube Music cookies are missing."`) {
		t.Fatalf("expected missing cookies code in body: %s", body)
	}
}

func TestDreamFMLibraryHandlerReturnsTimeoutCode(t *testing.T) {
	timeoutErr := errors.Join(youtubemusic.ErrRequestTimedOut, context.DeadlineExceeded)
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		libraryPlaylistsErr:    timeoutErr,
		libraryArtistsErr:      timeoutErr,
		likedSongsErr:          timeoutErr,
		browsePageErr:          timeoutErr,
		homeShelvesErr:         timeoutErr,
		homeRecommendationsErr: timeoutErr,
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"youtube_timeout"`) ||
		!strings.Contains(body, `"message":"YouTube Music request timed out."`) {
		t.Fatalf("expected timeout code in body: %s", body)
	}
}

func TestDreamFMLibraryHandlerServesCategoriesAndSkipsPodcasts(t *testing.T) {
	handler := NewDreamFMLibraryHandler(fakeDreamFMMusicClient{
		browsePage: youtubemusic.BrowsePage{
			Continuation: "next-token",
			Shelves: []youtubemusic.Shelf{
				{
					ID:    "Moods::categories::focus",
					Title: "Moods",
					Kind:  youtubemusic.ShelfCategories,
					Categories: []youtubemusic.Category{{
						ID:       "FEmusic_moods_and_genres_category_params",
						BrowseID: "FEmusic_moods_and_genres_category",
						Params:   "params",
						Title:    "Focus",
						ColorHex: "#336699",
					}},
				},
				{
					ID:    "Podcasts::podcasts::MPSPPpodcast",
					Title: "Podcasts",
					Kind:  youtubemusic.ShelfPodcasts,
					Podcasts: []youtubemusic.PodcastShow{{
						ID:     "MPSPPpodcast",
						Title:  "Night Talks",
						Author: "Dream FM",
					}},
				},
			},
		},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/library?source=moods", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"kind":"categories"`) || !strings.Contains(body, `"browseId":"FEmusic_moods_and_genres_category"`) {
		t.Fatalf("expected category shelf in body: %s", body)
	}
	if strings.Contains(body, `"kind":"podcasts"`) || strings.Contains(body, `"playlistId":"MPSPPpodcast"`) {
		t.Fatalf("expected podcast shelf to be skipped: %s", body)
	}
	if !strings.Contains(body, `"continuation":"next-token"`) {
		t.Fatalf("expected continuation in body: %s", body)
	}
}
