package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

type fakeListenMusicClient struct {
	searchTracks           []youtubemusic.Track
	searchArtists          []youtubemusic.Artist
	searchPlaylists        []youtubemusic.Playlist
	radioTracks            []youtubemusic.Track
	homeTracks             []youtubemusic.Track
	homeShelves            []youtubemusic.Shelf
	homeShelvesErr         error
	homeRecommendationsErr error
	browsePage             youtubemusic.BrowsePage
	browsePageErr          error
	browsePageFunc         func(context.Context, string, string, string, int, int) (youtubemusic.BrowsePage, error)
	browseShelves          []youtubemusic.Shelf
	libraryArtists         []youtubemusic.Artist
	libraryArtistsErr      error
	artistPage             youtubemusic.ArtistPage
	artistPageFunc         func(context.Context, string, int) (youtubemusic.ArtistPage, error)
	playlistPage           youtubemusic.TrackListPage
	playlistTracks         []youtubemusic.Track
	libraryPlaylists       []youtubemusic.Playlist
	libraryPlaylistsErr    error
	likedSongs             []youtubemusic.Track
	likedSongsErr          error
	trackMetadata          youtubemusic.TrackMetadata
	trackMetadataErr       error
	trackLyrics            youtubemusic.LyricsResult
	trackLyricsErr         error
	trackLyricsFunc        func(context.Context, youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error)
	trackDurations         map[string]string
	subscribeFunc          func(context.Context, string) error
	unsubscribeFunc        func(context.Context, string) error
	subscribeArtistFunc    func(context.Context, string) error
	unsubscribeArtistFunc  func(context.Context, string) error
	rateSongFunc           func(context.Context, string, youtubemusic.LikeStatus) error
}

func (client fakeListenMusicClient) SearchSongs(context.Context, string, int) ([]youtubemusic.Track, error) {
	return client.searchTracks, nil
}

func (client fakeListenMusicClient) SearchArtists(context.Context, string, int) ([]youtubemusic.Artist, error) {
	return client.searchArtists, nil
}

func (client fakeListenMusicClient) SearchPlaylists(context.Context, string, int) ([]youtubemusic.Playlist, error) {
	return client.searchPlaylists, nil
}

func (client fakeListenMusicClient) Radio(context.Context, string, int) ([]youtubemusic.Track, error) {
	return client.radioTracks, nil
}

func (client fakeListenMusicClient) HomeRecommendations(context.Context, int) ([]youtubemusic.Track, error) {
	if client.homeRecommendationsErr != nil {
		return nil, client.homeRecommendationsErr
	}
	return client.homeTracks, nil
}

func (client fakeListenMusicClient) HomeShelves(context.Context, int, int) ([]youtubemusic.Shelf, error) {
	if client.homeShelvesErr != nil {
		return nil, client.homeShelvesErr
	}
	return client.homeShelves, nil
}

func (client fakeListenMusicClient) BrowseShelves(context.Context, string, int, int) ([]youtubemusic.Shelf, error) {
	return client.browseShelves, nil
}

func (client fakeListenMusicClient) BrowseShelvesPage(ctx context.Context, browseID string, params string, continuation string, sectionLimit int, itemLimit int) (youtubemusic.BrowsePage, error) {
	if client.browsePageFunc != nil {
		return client.browsePageFunc(ctx, browseID, params, continuation, sectionLimit, itemLimit)
	}
	if client.browsePageErr != nil {
		return youtubemusic.BrowsePage{}, client.browsePageErr
	}
	if len(client.browsePage.Shelves) > 0 || client.browsePage.Continuation != "" {
		return client.browsePage, nil
	}
	if len(client.browseShelves) > 0 {
		return youtubemusic.BrowsePage{Shelves: client.browseShelves}, nil
	}
	return youtubemusic.BrowsePage{Shelves: client.homeShelves}, nil
}

func (client fakeListenMusicClient) ArtistPage(ctx context.Context, browseID string, limit int) (youtubemusic.ArtistPage, error) {
	if client.artistPageFunc != nil {
		return client.artistPageFunc(ctx, browseID, limit)
	}
	return client.artistPage, nil
}

func (client fakeListenMusicClient) PlaylistQueue(context.Context, string, int) ([]youtubemusic.Track, error) {
	return client.playlistTracks, nil
}

func (client fakeListenMusicClient) PlaylistPage(context.Context, string, string, int) (youtubemusic.TrackListPage, error) {
	if len(client.playlistPage.Tracks) > 0 || client.playlistPage.Continuation != "" {
		return client.playlistPage, nil
	}
	return youtubemusic.TrackListPage{Tracks: client.playlistTracks}, nil
}

func (client fakeListenMusicClient) LibraryPlaylists(context.Context, int) ([]youtubemusic.Playlist, error) {
	if client.libraryPlaylistsErr != nil {
		return nil, client.libraryPlaylistsErr
	}
	return client.libraryPlaylists, nil
}

func (client fakeListenMusicClient) LibraryArtists(context.Context, int) ([]youtubemusic.Artist, error) {
	if client.libraryArtistsErr != nil {
		return nil, client.libraryArtistsErr
	}
	return client.libraryArtists, nil
}

func (client fakeListenMusicClient) LikedSongs(context.Context, int) ([]youtubemusic.Track, error) {
	if client.likedSongsErr != nil {
		return nil, client.likedSongsErr
	}
	return client.likedSongs, nil
}

func (client fakeListenMusicClient) TrackMetadata(context.Context, string) (youtubemusic.TrackMetadata, error) {
	if client.trackMetadataErr != nil {
		return youtubemusic.TrackMetadata{}, client.trackMetadataErr
	}
	return client.trackMetadata, nil
}

func (client fakeListenMusicClient) TrackLyrics(ctx context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error) {
	if client.trackLyricsFunc != nil {
		return client.trackLyricsFunc(ctx, info)
	}
	if client.trackLyricsErr != nil {
		return youtubemusic.LyricsResult{}, client.trackLyricsErr
	}
	return client.trackLyrics, nil
}

func (client fakeListenMusicClient) TrackDurations(context.Context, []string) (map[string]string, error) {
	return client.trackDurations, nil
}

func (client fakeListenMusicClient) SubscribePlaylist(ctx context.Context, playlistID string) error {
	if client.subscribeFunc != nil {
		return client.subscribeFunc(ctx, playlistID)
	}
	return nil
}

func (client fakeListenMusicClient) UnsubscribePlaylist(ctx context.Context, playlistID string) error {
	if client.unsubscribeFunc != nil {
		return client.unsubscribeFunc(ctx, playlistID)
	}
	return nil
}

func (client fakeListenMusicClient) SubscribeArtist(ctx context.Context, channelID string) error {
	if client.subscribeArtistFunc != nil {
		return client.subscribeArtistFunc(ctx, channelID)
	}
	return nil
}

func (client fakeListenMusicClient) UnsubscribeArtist(ctx context.Context, channelID string) error {
	if client.unsubscribeArtistFunc != nil {
		return client.unsubscribeArtistFunc(ctx, channelID)
	}
	return nil
}

func (client fakeListenMusicClient) RateSong(ctx context.Context, videoID string, rating youtubemusic.LikeStatus) error {
	if client.rateSongFunc != nil {
		return client.rateSongFunc(ctx, videoID, rating)
	}
	return nil
}

func TestListenSearchHandlerPrefersYouTubeMusic(t *testing.T) {
	handler := NewListenSearchHandler(fakeListenMusicClient{searchTracks: []youtubemusic.Track{{
		VideoID:        "TESTVID007G",
		Title:          "Lofi Mix",
		Channel:        "Super Lofi World",
		Artists:        []youtubemusic.TrackArtist{{Name: "Super Lofi World", BrowseID: "UCsuperlofi"}},
		ArtistBrowseID: "UCsuperlofi",
		DurationLabel:  "3:21",
		ThumbnailURL:   "https://lh3.googleusercontent.com/cover",
		MusicVideoType: "MUSIC_VIDEO_TYPE_ATV",
	}}})
	request := httptest.NewRequest("GET", "/api/listen/search?q=lofi", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"id":"ytmusic-search-TESTVID007G"`) || !strings.Contains(body, `"artistBrowseId":"UCsuperlofi"`) || !strings.Contains(body, `"thumbnailUrl":"https://lh3.googleusercontent.com/cover"`) {
		t.Fatalf("unexpected body: %s", body)
	}
	if !strings.Contains(body, `"artists":[`) || !strings.Contains(body, `"browseId":"UCsuperlofi"`) {
		t.Fatalf("expected structured track artists in body: %s", body)
	}
	if !strings.Contains(body, `"musicVideoType":"MUSIC_VIDEO_TYPE_ATV"`) {
		t.Fatalf("unexpected body: %s", body)
	}
	if strings.Contains(body, `"hasVideo"`) {
		t.Fatalf("expected false hasVideo to be omitted, got %s", body)
	}
	if !strings.Contains(body, `"videoAvailabilityKnown":true`) {
		t.Fatalf("audio endpoint type should be serialized as confirmed no-video availability: %s", body)
	}
}

func TestListenSearchHandlerDoesNotConfirmUGCAvailability(t *testing.T) {
	handler := NewListenSearchHandler(fakeListenMusicClient{searchTracks: []youtubemusic.Track{{
		VideoID:        "TESTVID007G",
		Title:          "Fan Upload",
		Channel:        "Uploader",
		MusicVideoType: "MUSIC_VIDEO_TYPE_UGC",
	}}})
	request := httptest.NewRequest("GET", "/api/listen/search?q=fan", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"musicVideoType":"MUSIC_VIDEO_TYPE_UGC"`) {
		t.Fatalf("expected response body to contain UGC musicVideoType, got %s", body)
	}
	if strings.Contains(body, `"hasVideo"`) || strings.Contains(body, `"videoAvailabilityKnown"`) {
		t.Fatalf("UGC endpoint type should not be serialized as confirmed video availability: %s", body)
	}
}

func TestListenSearchHandlerConfirmsNoVideoFromNonVideoThumbnail(t *testing.T) {
	handler := NewListenSearchHandler(fakeListenMusicClient{searchTracks: []youtubemusic.Track{{
		VideoID:        "TESTVID007G",
		Title:          "Audio Artwork",
		Channel:        "Uploader",
		MusicVideoType: "MUSIC_VIDEO_TYPE_UGC",
		ThumbnailURL:   "https://lh3.googleusercontent.com/art=w544-h544",
	}}})
	request := httptest.NewRequest("GET", "/api/listen/search?q=audio", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if strings.Contains(body, `"hasVideo"`) {
		t.Fatalf("expected false hasVideo to be omitted, got %s", body)
	}
	if !strings.Contains(body, `"videoAvailabilityKnown":true`) {
		t.Fatalf("non-video thumbnail should serialize confirmed no-video availability: %s", body)
	}
}

func TestListenSearchHandlerInfersVideoFromYouTubeThumbnail(t *testing.T) {
	handler := NewListenSearchHandler(fakeListenMusicClient{searchTracks: []youtubemusic.Track{{
		VideoID:        "TESTVID007G",
		Title:          "Video Upload",
		Channel:        "Uploader",
		MusicVideoType: "MUSIC_VIDEO_TYPE_PODCAST_EPISODE",
		ThumbnailURL:   "https://i.ytimg.com/vi/TESTVID007G/hq720.jpg",
	}}})
	request := httptest.NewRequest("GET", "/api/listen/search?q=video", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`"hasVideo":true`,
		`"videoAvailabilityKnown":true`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, body)
		}
	}
}

func TestListenSearchHandlerServesArtistsAndPlaylists(t *testing.T) {
	handler := NewListenSearchHandler(fakeListenMusicClient{
		searchArtists: []youtubemusic.Artist{{
			ID:           "UCsuperlofi",
			Name:         "Super Lofi World",
			Subtitle:     "1.2M subscribers",
			ThumbnailURL: "https://lh3.googleusercontent.com/artist",
		}},
		searchPlaylists: []youtubemusic.Playlist{{
			ID:           "VLPL1234567890",
			Title:        "Focus Queue",
			Channel:      "Dream FM",
			ThumbnailURL: "https://i.ytimg.com/vi/focus/hqdefault.jpg",
		}},
	})
	request := httptest.NewRequest("GET", "/api/listen/search?q=lofi", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"browseId":"UCsuperlofi"`) || !strings.Contains(body, `"name":"Super Lofi World"`) {
		t.Fatalf("expected artist result in body: %s", body)
	}
	if !strings.Contains(body, `"playlistId":"VLPL1234567890"`) || !strings.Contains(body, `"title":"Focus Queue"`) {
		t.Fatalf("expected playlist result in body: %s", body)
	}
}

func TestListenSearchHandlerRequiresYouTubeMusic(t *testing.T) {
	handler := NewListenSearchHandler(nil)
	request := httptest.NewRequest("GET", "/api/listen/search?q=lofi", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", recorder.Result().StatusCode)
	}
}

func TestListenSearchHandlerEnrichesMissingDurations(t *testing.T) {
	handler := NewListenSearchHandler(fakeListenMusicClient{
		searchTracks: []youtubemusic.Track{{
			VideoID: "TESTVID007G",
			Title:   "Lofi Mix",
			Channel: "Super Lofi World",
		}},
		trackDurations: map[string]string{"TESTVID007G": "3:21"},
	})
	request := httptest.NewRequest("GET", "/api/listen/search?q=lofi", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"durationLabel":"3:21"`) {
		t.Fatalf("expected enriched duration in body: %s", body)
	}
}

func TestListenArtistHandlerServesArtistBrowse(t *testing.T) {
	handler := NewListenArtistHandler(fakeListenMusicClient{artistPage: youtubemusic.ArtistPage{
		ID:            "UCsuperlofi",
		Title:         "Super Lofi World",
		Subtitle:      "1.2M monthly listeners",
		ThumbnailURL:  "https://lh3.googleusercontent.com/artist",
		ChannelID:     "UCsuperlofi",
		IsSubscribed:  true,
		MixPlaylistID: "RDARTISTsuperlofi",
		Tracks: []youtubemusic.Track{{
			VideoID:        "TESTVID007G",
			Title:          "Lofi Mix",
			Channel:        "Super Lofi World",
			ArtistBrowseID: "UCsuperlofi",
			DurationLabel:  "3:21",
		}},
		Shelves: []youtubemusic.Shelf{{
			ID:    "Top songs::tracks::TESTVID007G",
			Title: "Top songs",
			Kind:  youtubemusic.ShelfTracks,
			Tracks: []youtubemusic.Track{{
				VideoID:      "TESTVID007G",
				Title:        "Lofi Mix",
				Channel:      "Super Lofi World",
				ThumbnailURL: "https://i.ytimg.com/vi/TESTVID007G/hq720.jpg",
			}},
		}},
	}})
	request := httptest.NewRequest("GET", "/api/listen/artist?id=UCsuperlofi&name=Super+Lofi+World", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"title":"Super Lofi World"`) || !strings.Contains(body, `"subtitle":"1.2M monthly listeners"`) || !strings.Contains(body, `"thumbnailUrl":"https://lh3.googleusercontent.com/artist"`) || !strings.Contains(body, `"channelId":"UCsuperlofi"`) || !strings.Contains(body, `"isSubscribed":true`) || !strings.Contains(body, `"mixPlaylistId":"RDARTISTsuperlofi"`) || !strings.Contains(body, `"id":"ytmusic-artist-TESTVID007G"`) || !strings.Contains(body, `"shelves"`) || !strings.Contains(body, `"Top songs"`) || !strings.Contains(body, `"hasVideo":true`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestListenArtistHandlerResolvesNameOnlyArtistBrowse(t *testing.T) {
	var gotBrowseID string
	handler := NewListenArtistHandler(fakeListenMusicClient{
		searchArtists: []youtubemusic.Artist{{
			ID:           "UCsecond",
			Name:         "Second Artist",
			ThumbnailURL: "https://lh3.googleusercontent.com/search-artist",
		}},
		artistPageFunc: func(_ context.Context, browseID string, _ int) (youtubemusic.ArtistPage, error) {
			gotBrowseID = browseID
			return youtubemusic.ArtistPage{
				ID:           browseID,
				Title:        "Second Artist",
				ThumbnailURL: "https://lh3.googleusercontent.com/artist-page",
				Tracks: []youtubemusic.Track{{
					VideoID: "TESTVID008H",
					Title:   "Second Song",
					Channel: "Second Artist",
				}},
				Shelves: []youtubemusic.Shelf{{
					ID:    "Top songs::tracks::TESTVID008H",
					Title: "Top songs",
					Kind:  youtubemusic.ShelfTracks,
					Tracks: []youtubemusic.Track{{
						VideoID: "TESTVID008H",
						Title:   "Second Song",
						Channel: "Second Artist",
					}},
				}},
			}, nil
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/artist?name=Second+Artist", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if gotBrowseID != "UCsecond" {
		t.Fatalf("expected artist browse id resolution, got %q", gotBrowseID)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"id":"UCsecond"`) ||
		!strings.Contains(body, `"thumbnailUrl":"https://lh3.googleusercontent.com/artist-page"`) ||
		!strings.Contains(body, `"Top songs"`) ||
		!strings.Contains(body, `"id":"ytmusic-artist-TESTVID008H"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestListenArtistHandlerServesShelfBrowse(t *testing.T) {
	var gotBrowseID string
	var gotParams string
	handler := NewListenArtistHandler(fakeListenMusicClient{
		browsePageFunc: func(_ context.Context, browseID string, params string, continuation string, _ int, _ int) (youtubemusic.BrowsePage, error) {
			gotBrowseID = browseID
			gotParams = params
			if continuation != "" {
				t.Fatalf("unexpected continuation: %q", continuation)
			}
			return youtubemusic.BrowsePage{
				Title: "Top songs",
				Shelves: []youtubemusic.Shelf{{
					ID:    "Top songs::tracks::TESTVID008H",
					Title: "Top songs",
					Kind:  youtubemusic.ShelfTracks,
					Tracks: []youtubemusic.Track{{
						VideoID:      "TESTVID008H",
						Title:        "More Lofi",
						Channel:      "Super Lofi World",
						ThumbnailURL: "https://i.ytimg.com/vi/TESTVID008H/hq720.jpg",
					}},
				}},
			}, nil
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/artist?id=UCsuperlofi&name=Super+Lofi+World&browseId=MPLAUCsuperlofi&params=wAEB", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if gotBrowseID != "MPLAUCsuperlofi" || gotParams != "wAEB" {
		t.Fatalf("unexpected browse request: %q %q", gotBrowseID, gotParams)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"title":"Top songs"`) || !strings.Contains(body, `"videoId":"TESTVID008H"`) || !strings.Contains(body, `"hasVideo":true`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestListenArtistHandlerUpdatesSubscription(t *testing.T) {
	var gotChannelID string
	var subscribeCalled bool
	handler := NewListenArtistHandler(fakeListenMusicClient{
		subscribeArtistFunc: func(_ context.Context, channelID string) error {
			gotChannelID = channelID
			subscribeCalled = true
			return nil
		},
	})
	request := httptest.NewRequest("POST", "/api/listen/artist", strings.NewReader(`{"channelId":"UCsuperlofi","subscribed":true}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if !subscribeCalled || gotChannelID != "UCsuperlofi" {
		t.Fatalf("unexpected subscription call: called=%v channel=%q", subscribeCalled, gotChannelID)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"subscribed":true`) {
		t.Fatalf("unexpected body: %s", body)
	}
}
