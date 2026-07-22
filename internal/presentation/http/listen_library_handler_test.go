package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/appsessions"
)

func TestListenLibraryHandlerServesHomeShelvesWithoutLibraryInjection(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
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
	request := httptest.NewRequest("GET", "/api/listen/library", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	for _, unexpected := range []string{
		`"playlistId":"VLPL1234567890"`,
		`"browseId":"UCsuperlofi"`,
		`"id":"ytmusic-liked-songs"`,
		`"title":"Liked Track"`,
	} {
		if strings.Contains(body, unexpected) {
			t.Fatalf("expected Home to exclude library projection %s: %s", unexpected, body)
		}
	}
	if !strings.Contains(body, `"title":"Quick picks"`) || !strings.Contains(body, `"kind":"tracks"`) {
		t.Fatalf("expected track shelf in body: %s", body)
	}
	if !strings.Contains(body, `"playlistId":"VLPLfeedface"`) || !strings.Contains(body, `"kind":"playlists"`) {
		t.Fatalf("expected playlist shelf in body: %s", body)
	}
	if quickPicks := strings.Index(body, `"title":"Quick picks"`); quickPicks < 0 || strings.Index(body, `"title":"Featured"`) < quickPicks {
		t.Fatalf("expected YouTube Music Home shelf order to be preserved: %s", body)
	}
}

func TestListenLibraryHandlerRequestsCompleteHomeShelfWindow(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		browsePageFunc: func(_ context.Context, browseID string, params string, continuation string, sectionLimit int, itemLimit int) (youtubemusic.BrowsePage, error) {
			if browseID != "FEmusic_home" || params != "" || continuation != "" {
				t.Fatalf("unexpected Home browse request: browseID=%q params=%q continuation=%q", browseID, params, continuation)
			}
			if sectionLimit != listenLibraryHomeShelfLimit || itemLimit != listenLibraryShelfItemLimit {
				t.Fatalf("unexpected Home limits: sections=%d items=%d", sectionLimit, itemLimit)
			}
			return youtubemusic.BrowsePage{Shelves: []youtubemusic.Shelf{{
				ID:    "Listen again::playlists::VLPLlistenagain",
				Title: "Listen again",
				Kind:  youtubemusic.ShelfPlaylists,
			}}}, nil
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/library", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Result().StatusCode, recorder.Body.String())
	}
}

func TestListenLibraryHandlerServesBrowseSourceShelves(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
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
	request := httptest.NewRequest("GET", "/api/listen/library?source=charts", nil)
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

func TestListenLibraryHandlerServesDedicatedPlaylistSource(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		browsePageFunc: func(_ context.Context, browseID string, params string, continuation string, _ int, _ int) (youtubemusic.BrowsePage, error) {
			if browseID != "FEmusic_liked_playlists" || params != "" || continuation != "" {
				t.Fatalf("unexpected playlist browse request: browseID=%q params=%q continuation=%q", browseID, params, continuation)
			}
			return youtubemusic.BrowsePage{Shelves: []youtubemusic.Shelf{{
				ID:    "Library playlists::playlists::VLPL1234567890",
				Title: "Library playlists",
				Kind:  youtubemusic.ShelfPlaylists,
				Playlists: []youtubemusic.Playlist{{
					ID:      "VLPL1234567890",
					Title:   "My Library",
					Channel: "Arnold",
				}},
			}}}, nil
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/library?source=playlists", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Result().StatusCode, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"playlists":[{"id":"ytmusic-playlists-playlist-VLPL1234567890"`) ||
		!strings.Contains(body, `"title":"Library playlists"`) {
		t.Fatalf("expected dedicated playlist projection and shelf: %s", body)
	}
}

func TestListenLibrarySourceBrowseIDsMatchYouTubeMusicRoutes(t *testing.T) {
	tests := map[string]string{
		listenLibrarySourceHome:      "FEmusic_home",
		listenLibrarySourceExplore:   "FEmusic_explore",
		listenLibrarySourceCharts:    "FEmusic_charts",
		listenLibrarySourceMoods:     "FEmusic_moods_and_genres",
		listenLibrarySourceNew:       "FEmusic_new_releases",
		listenLibrarySourceHistory:   "FEmusic_history",
		listenLibrarySourceRecent:    "FEmusic_library_landing",
		listenLibrarySourcePodcasts:  "FEmusic_podcasts",
		listenLibrarySourcePlaylists: "FEmusic_liked_playlists",
	}
	for source, expected := range tests {
		if actual := listenLibrarySourceBrowseID(source); actual != expected {
			t.Errorf("source %q: expected %q, got %q", source, expected, actual)
		}
	}
}

func TestListenLibraryHomeAllowsOnlyItsMoodCategoryBrowseTargets(t *testing.T) {
	if !listenLibraryBrowseOverrideAllowed(
		listenLibrarySourceHome,
		"FEmusic_moods_and_genres_category",
	) {
		t.Fatal("expected a Home mood chip target to remain navigable")
	}
	if listenLibraryBrowseOverrideAllowed(
		listenLibrarySourceHome,
		"FEmusic_history",
	) {
		t.Fatal("expected unrelated Home browse overrides to remain rejected")
	}
}

func TestListenLibraryHandlerServesChartTabShelves(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		browsePageFunc: func(_ context.Context, browseID string, params string, _ string, _ int, _ int) (youtubemusic.BrowsePage, error) {
			switch params {
			case "songs":
				return youtubemusic.BrowsePage{Shelves: []youtubemusic.Shelf{{
					ID:    "Top songs::tracks::TopSong0001",
					Title: "Top songs",
					Kind:  youtubemusic.ShelfTracks,
					Tracks: []youtubemusic.Track{{
						VideoID:      "TopSong0001",
						Title:        "Top Song",
						Channel:      "Dream FM",
						ThumbnailURL: "https://i.ytimg.com/vi/TopSong0001/hq720.jpg",
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
	request := httptest.NewRequest("GET", "/api/listen/library?source=charts", nil)
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
		`"hasVideo":true`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %s in body: %s", expected, body)
		}
	}
}

func TestListenLibraryHandlerReturnsBrowseSourceErrorDetails(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		browsePageErr: errors.New("youtube music api status 400: invalid browse id"),
	})
	request := httptest.NewRequest("GET", "/api/listen/library?source=explore", nil)
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

func TestListenLibraryHandlerServesRecentAndPodcastSources(t *testing.T) {
	tests := []struct {
		name                 string
		source               string
		continuation         string
		expectedBrowseID     string
		expectedContinuation string
		shelves              []youtubemusic.Shelf
		expectedBody         string
	}{
		{
			name:                 "recent",
			source:               "recent",
			expectedBrowseID:     "FEmusic_library_landing",
			expectedContinuation: "recent-next",
			shelves: []youtubemusic.Shelf{{
				ID:    "Recent::tracks::TESTVID007G",
				Title: "Recent",
				Kind:  youtubemusic.ShelfTracks,
				Tracks: []youtubemusic.Track{{
					VideoID: "TESTVID007G",
					Title:   "Recently Played",
				}},
			}},
			expectedBody: `"title":"Recently Played"`,
		},
		{
			name:                 "podcasts continuation",
			source:               "podcasts",
			continuation:         "podcast-resume",
			expectedBrowseID:     "FEmusic_podcasts",
			expectedContinuation: "podcast-next",
			shelves: []youtubemusic.Shelf{{
				ID:    "Podcasts::playlists::MPSPPpodcast",
				Title: "Podcasts",
				Kind:  youtubemusic.ShelfPlaylists,
				Playlists: []youtubemusic.Playlist{{
					ID:      "MPSPPpodcast",
					Title:   "Night Talks",
					Channel: "Dream FM",
				}},
			}},
			expectedBody: `"playlistId":"MPSPPpodcast"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewListenLibraryHandler(fakeListenMusicClient{
				browsePageFunc: func(_ context.Context, browseID string, _ string, continuation string, _ int, _ int) (youtubemusic.BrowsePage, error) {
					if browseID != test.expectedBrowseID {
						t.Fatalf("unexpected browse id: %s", browseID)
					}
					if continuation != test.continuation {
						t.Fatalf("unexpected continuation: %s", continuation)
					}
					return youtubemusic.BrowsePage{
						Shelves:      test.shelves,
						Continuation: test.expectedContinuation,
					}, nil
				},
			})
			request := httptest.NewRequest(
				"GET",
				"/api/listen/library?source="+test.source+"&continuation="+test.continuation,
				nil,
			)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Result().StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", recorder.Result().StatusCode, recorder.Body.String())
			}
			body := recorder.Body.String()
			if !strings.Contains(body, test.expectedBody) || !strings.Contains(body, `"continuation":"`+test.expectedContinuation+`"`) {
				t.Fatalf("unexpected body: %s", body)
			}
		})
	}
}

func TestListenLibraryHandlerRejectsUnknownSource(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{})
	request := httptest.NewRequest("GET", "/api/listen/library?source=unknown", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"invalid_source"`) || !strings.Contains(body, `"source":"unknown"`) {
		t.Fatalf("expected invalid source in body: %s", body)
	}
}

func TestListenLibraryHandlerReturnsAuthErrorDetails(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		browsePageErr: youtubemusic.ErrAuthExpired,
	})
	request := httptest.NewRequest("GET", "/api/listen/library?source=explore", nil)
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

func TestListenLibraryHandlerReturnsTypedHomeBrowseUnavailableError(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		browsePageErr: youtubemusic.ErrBrowseUnavailable,
	})
	request := httptest.NewRequest("GET", "/api/listen/library", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"youtube_transient_unavailable"`) ||
		!strings.Contains(body, `"message":"YouTube Music recommendations are temporarily unavailable."`) ||
		!strings.Contains(body, `"detail":"youtube music browse unavailable"`) ||
		!strings.Contains(body, `"retryable":true`) {
		t.Fatalf("expected structured retryable browse error in body: %s", body)
	}
}

func TestListenLibraryHandlerReturnsNonRetryableRegionUnavailableError(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		browsePageErr: youtubemusic.ErrRegionUnavailable,
	})
	request := httptest.NewRequest("GET", "/api/listen/library", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusUnavailableForLegalReasons {
		t.Fatalf("expected 451, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"youtube_region_unavailable"`) ||
		!strings.Contains(body, `"message":"YouTube Music is unavailable in your region."`) ||
		strings.Contains(body, `"retryable":true`) {
		t.Fatalf("expected structured non-retryable region error in body: %s", body)
	}
}

func TestListenLibraryFailureLogIsStructuredAndRedacted(t *testing.T) {
	core, observed := observer.New(zap.WarnLevel)
	restoreLogger := zap.ReplaceGlobals(zap.New(core))
	defer restoreLogger()

	detail := `Post "https://music.youtube.com/youtubei/v1/browse?key=api-secret&token=query-secret": Authorization: Bearer auth-secret; response={"access_token":"json-secret"}; token=body-secret; Cookie: SID=cookie-secret`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/listen/library?source=explore", nil)
	writeListenLibraryError(
		recorder,
		request,
		http.StatusServiceUnavailable,
		"youtube_network_unavailable",
		"YouTube Music network unavailable.",
		detail,
		"explore",
	)

	entries := observed.FilterMessage("listen youtube music library request failed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one structured library failure log, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["status"] != int64(http.StatusServiceUnavailable) || fields["code"] != "youtube_network_unavailable" || fields["source"] != "explore" {
		t.Fatalf("unexpected structured log fields: %#v", fields)
	}
	loggedDetail, _ := fields["detail"].(string)
	for _, secret := range []string{"api-secret", "query-secret", "auth-secret", "body-secret", "cookie-secret", "json-secret"} {
		if strings.Contains(loggedDetail, secret) {
			t.Fatalf("secret %q leaked into log detail: %q", secret, loggedDetail)
		}
	}
	if !strings.Contains(loggedDetail, "[REDACTED]") {
		t.Fatalf("expected redaction marker in log detail: %q", loggedDetail)
	}
	if got := len([]rune(safeListenLibraryLogDetail(strings.Repeat("界", listenLibraryLogDetailLimit+20)))); got != listenLibraryLogDetailLimit+1 {
		t.Fatalf("expected rune-safe truncated detail, got %d runes", got)
	}
}

func TestListenLibraryCanceledRequestDoesNotLogFailure(t *testing.T) {
	core, observed := observer.New(zap.WarnLevel)
	restoreLogger := zap.ReplaceGlobals(zap.New(core))
	defer restoreLogger()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest("GET", "/api/listen/library", nil).WithContext(ctx)
	writeListenLibraryError(
		httptest.NewRecorder(),
		request,
		http.StatusServiceUnavailable,
		"youtube_music_unavailable",
		"YouTube Music library unavailable.",
		"context canceled",
		"home",
	)

	if entries := observed.FilterMessage("listen youtube music library request failed").All(); len(entries) != 0 {
		t.Fatalf("canceled request must not emit a failure warning: %#v", entries)
	}
}

func TestListenLibraryHandlerClassifiesRawTLSTransportErrorAsRetryableNetworkFailure(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		browsePageErr: errors.New("remote error: tls: internal error"),
	})
	request := httptest.NewRequest("GET", "/api/listen/library?source=explore", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"youtube_network_unavailable"`) ||
		!strings.Contains(body, `"retryable":true`) {
		t.Fatalf("expected retryable network classification in body: %s", body)
	}
}

func TestListenLibraryHandlerReturnsMissingCookiesCode(t *testing.T) {
	missingCookiesErr := errors.Join(youtubemusic.ErrNotAuthenticated, appsessions.ErrNoCookies)
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		libraryPlaylistsErr:    missingCookiesErr,
		libraryArtistsErr:      missingCookiesErr,
		likedSongsErr:          missingCookiesErr,
		browsePageErr:          missingCookiesErr,
		homeShelvesErr:         missingCookiesErr,
		homeRecommendationsErr: missingCookiesErr,
	})
	request := httptest.NewRequest("GET", "/api/listen/library", nil)
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

func TestListenLibraryHandlerReturnsTimeoutCode(t *testing.T) {
	timeoutErr := errors.Join(youtubemusic.ErrRequestTimedOut, context.DeadlineExceeded)
	handler := NewListenLibraryHandler(fakeListenMusicClient{
		libraryPlaylistsErr:    timeoutErr,
		libraryArtistsErr:      timeoutErr,
		likedSongsErr:          timeoutErr,
		browsePageErr:          timeoutErr,
		homeShelvesErr:         timeoutErr,
		homeRecommendationsErr: timeoutErr,
	})
	request := httptest.NewRequest("GET", "/api/listen/library", nil)
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

func TestListenLibraryHandlerServesCategoriesAndPodcastCompatibility(t *testing.T) {
	handler := NewListenLibraryHandler(fakeListenMusicClient{
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
					Kind:  "podcasts",
					Playlists: []youtubemusic.Playlist{{
						ID:      "MPSPPpodcast",
						Title:   "Night Talks",
						Channel: "Dream FM",
					}},
				},
			},
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/library?source=moods", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"kind":"categories"`) || !strings.Contains(body, `"browseId":"FEmusic_moods_and_genres_category"`) {
		t.Fatalf("expected category shelf in body: %s", body)
	}
	if strings.Contains(body, `"kind":"podcasts"`) || !strings.Contains(body, `"kind":"playlists"`) || !strings.Contains(body, `"playlistId":"MPSPPpodcast"`) {
		t.Fatalf("expected podcast shelf to use playlist compatibility: %s", body)
	}
	if !strings.Contains(body, `"continuation":"next-token"`) {
		t.Fatalf("expected continuation in body: %s", body)
	}
}
