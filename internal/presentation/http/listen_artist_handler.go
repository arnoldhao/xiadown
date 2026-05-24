package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const (
	listenArtistLimit   = 50
	listenArtistTimeout = 25 * time.Second
)

type listenYouTubeMusicArtistClient interface {
	ArtistPage(ctx context.Context, browseID string, limit int) (youtubemusic.ArtistPage, error)
	BrowseShelvesPage(ctx context.Context, browseID string, params string, continuation string, sectionLimit int, itemLimit int) (youtubemusic.BrowsePage, error)
	SearchArtists(ctx context.Context, query string, limit int) ([]youtubemusic.Artist, error)
	SearchSongs(ctx context.Context, query string, limit int) ([]youtubemusic.Track, error)
	SubscribeArtist(ctx context.Context, channelID string) error
	UnsubscribeArtist(ctx context.Context, channelID string) error
}

type ListenArtistHandler struct {
	ytMusic listenYouTubeMusicArtistClient
}

type ListenArtistResponse struct {
	ID            string               `json:"id"`
	Title         string               `json:"title"`
	Subtitle      string               `json:"subtitle,omitempty"`
	ThumbnailURL  string               `json:"thumbnailUrl,omitempty"`
	ChannelID     string               `json:"channelId,omitempty"`
	IsSubscribed  bool                 `json:"isSubscribed,omitempty"`
	MixPlaylistID string               `json:"mixPlaylistId,omitempty"`
	MixVideoID    string               `json:"mixVideoId,omitempty"`
	Items         []ListenSearchItem   `json:"items"`
	Shelves       []ListenLibraryShelf `json:"shelves"`
	Continuation  string               `json:"continuation,omitempty"`
}

type listenArtistSubscriptionPayload struct {
	ChannelID  string `json:"channelId"`
	Subscribed bool   `json:"subscribed"`
}

type listenArtistSubscriptionResponse struct {
	OK         bool `json:"ok,omitempty"`
	Subscribed bool `json:"subscribed"`
}

func NewListenArtistHandler(ytMusic listenYouTubeMusicArtistClient) *ListenArtistHandler {
	return &ListenArtistHandler{ytMusic: ytMusic}
}

func (handler *ListenArtistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost {
		writeListenMethodNotAllowed(w, r)
		return
	}
	setCORSHeaders(w, r)

	if handler.ytMusic == nil {
		writeListenServiceUnavailable(w, r, "youtube_music_client_unavailable", "YouTube Music client unavailable.", "")
		return
	}
	if r.Method == http.MethodPost {
		handler.handleSubscription(w, r)
		return
	}

	browseID := strings.TrimSpace(r.URL.Query().Get("id"))
	artistName := strings.TrimSpace(r.URL.Query().Get("name"))
	continuation := strings.TrimSpace(r.URL.Query().Get("continuation"))
	shelfBrowseID := strings.TrimSpace(r.URL.Query().Get("browseId"))
	shelfParams := strings.TrimSpace(r.URL.Query().Get("params"))
	if browseID == "" && shelfBrowseID == "" && continuation == "" && len([]rune(artistName)) < 2 {
		writeListenBadRequest(w, r, "invalid_artist", "Invalid YouTube Music artist.", "")
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenArtistTimeout)
	defer cancel()

	if continuation != "" {
		page, err := handler.ytMusic.BrowseShelvesPage(ctx, "", "", continuation, listenArtistLimit, listenArtistLimit)
		if err != nil {
			writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music artist unavailable.", "artist")
			return
		}
		page.Shelves = enrichListenShelfTrackDurations(ctx, handler.ytMusic, page.Shelves)
		writeListenArtistJSON(w, r, ListenArtistResponse{
			ID:           browseID,
			Title:        firstNonEmptyString(artistName, browseID),
			Items:        mapYouTubeMusicTracksToListenItems(tracksFromListenShelves(page.Shelves), "ytmusic-artist"),
			Shelves:      mapYouTubeMusicShelvesToListenShelvesWithPrefixes(page.Shelves, "ytmusic-artist", "ytmusic-artist-playlist"),
			Continuation: page.Continuation,
		})
		return
	}

	if shelfBrowseID != "" {
		page, err := handler.ytMusic.BrowseShelvesPage(ctx, shelfBrowseID, shelfParams, "", listenArtistLimit, listenArtistLimit)
		if err != nil {
			writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music artist unavailable.", "artist")
			return
		}
		page.Shelves = enrichListenShelfTrackDurations(ctx, handler.ytMusic, page.Shelves)
		writeListenArtistJSON(w, r, ListenArtistResponse{
			ID:           firstNonEmptyString(browseID, shelfBrowseID),
			Title:        firstNonEmptyString(page.Title, artistName, browseID, shelfBrowseID),
			Items:        mapYouTubeMusicTracksToListenItems(tracksFromListenShelves(page.Shelves), "ytmusic-artist"),
			Shelves:      mapYouTubeMusicShelvesToListenShelvesWithPrefixes(page.Shelves, "ytmusic-artist", "ytmusic-artist-playlist"),
			Continuation: page.Continuation,
		})
		return
	}

	if browseID != "" {
		page, err := handler.ytMusic.ArtistPage(ctx, browseID, listenArtistLimit)
		if err == nil && (len(page.Tracks) > 0 || len(page.Shelves) > 0) {
			page.Tracks = enrichListenTrackDurations(ctx, handler.ytMusic, page.Tracks)
			page.Shelves = enrichListenShelfTrackDurations(ctx, handler.ytMusic, page.Shelves)
			writeListenArtistJSON(w, r, ListenArtistResponse{
				ID:            page.ID,
				Title:         firstNonEmptyString(page.Title, artistName, page.ID),
				Subtitle:      page.Subtitle,
				ThumbnailURL:  page.ThumbnailURL,
				ChannelID:     page.ChannelID,
				IsSubscribed:  page.IsSubscribed,
				MixPlaylistID: page.MixPlaylistID,
				MixVideoID:    page.MixVideoID,
				Items:         mapYouTubeMusicTracksToListenItems(page.Tracks, "ytmusic-artist"),
				Shelves:       mapYouTubeMusicShelvesToListenShelvesWithPrefixes(page.Shelves, "ytmusic-artist", "ytmusic-artist-playlist"),
				Continuation:  page.Continuation,
			})
			return
		}
		if artistName == "" && err != nil {
			writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music artist unavailable.", "artist")
			return
		}
	}

	if artistName == "" {
		writeListenArtistJSON(w, r, ListenArtistResponse{
			ID:      browseID,
			Title:   browseID,
			Items:   []ListenSearchItem{},
			Shelves: []ListenLibraryShelf{},
		})
		return
	}

	if page, ok, err := handler.resolveArtistPageByName(ctx, artistName); ok || err != nil {
		if err != nil {
			writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music artist unavailable.", "artist")
			return
		}
		page.Tracks = enrichListenTrackDurations(ctx, handler.ytMusic, page.Tracks)
		page.Shelves = enrichListenShelfTrackDurations(ctx, handler.ytMusic, page.Shelves)
		writeListenArtistJSON(w, r, ListenArtistResponse{
			ID:            page.ID,
			Title:         firstNonEmptyString(page.Title, artistName, page.ID),
			Subtitle:      page.Subtitle,
			ThumbnailURL:  page.ThumbnailURL,
			ChannelID:     page.ChannelID,
			IsSubscribed:  page.IsSubscribed,
			MixPlaylistID: page.MixPlaylistID,
			MixVideoID:    page.MixVideoID,
			Items:         mapYouTubeMusicTracksToListenItems(page.Tracks, "ytmusic-artist"),
			Shelves:       mapYouTubeMusicShelvesToListenShelvesWithPrefixes(page.Shelves, "ytmusic-artist", "ytmusic-artist-playlist"),
			Continuation:  page.Continuation,
		})
		return
	}

	tracks, err := handler.ytMusic.SearchSongs(ctx, artistName, listenArtistLimit)
	if err != nil {
		writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music artist unavailable.", "artist")
		return
	}
	tracks = enrichListenTrackDurations(ctx, handler.ytMusic, tracks)
	writeListenArtistJSON(w, r, ListenArtistResponse{
		ID:      browseID,
		Title:   artistName,
		Items:   mapYouTubeMusicTracksToListenItems(tracks, "ytmusic-artist-search"),
		Shelves: []ListenLibraryShelf{},
	})
}

func (handler *ListenArtistHandler) handleSubscription(w http.ResponseWriter, r *http.Request) {
	var payload listenArtistSubscriptionPayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&payload); err != nil {
		writeListenBadRequest(w, r, "invalid_request_body", "Invalid request body.", err.Error())
		return
	}

	channelID := strings.TrimSpace(payload.ChannelID)
	if !strings.HasPrefix(channelID, "UC") {
		writeListenBadRequest(w, r, "invalid_artist_channel_id", "Invalid YouTube Music artist channel id.", "channelId: "+channelID)
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenArtistTimeout)
	defer cancel()

	var err error
	if payload.Subscribed {
		err = handler.ytMusic.SubscribeArtist(ctx, channelID)
	} else {
		err = handler.ytMusic.UnsubscribeArtist(ctx, channelID)
	}
	if err != nil {
		writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music artist subscription unavailable.", "artist_subscription")
		return
	}
	writeListenArtistSubscriptionJSON(w, r, listenArtistSubscriptionResponse{
		OK:         true,
		Subscribed: payload.Subscribed,
	})
}

func writeListenArtistSubscriptionJSON(w http.ResponseWriter, r *http.Request, response listenArtistSubscriptionResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}

func writeListenArtistJSON(w http.ResponseWriter, r *http.Request, response ListenArtistResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (handler *ListenArtistHandler) resolveArtistPageByName(ctx context.Context, artistName string) (youtubemusic.ArtistPage, bool, error) {
	name := strings.TrimSpace(artistName)
	if len([]rune(name)) < 2 {
		return youtubemusic.ArtistPage{}, false, nil
	}
	artists, err := handler.ytMusic.SearchArtists(ctx, name, listenSearchArtistLimit)
	if err != nil {
		return youtubemusic.ArtistPage{}, false, nil
	}
	artist, ok := bestListenArtistSearchMatch(artists, name)
	if !ok {
		return youtubemusic.ArtistPage{}, false, nil
	}
	page, err := handler.ytMusic.ArtistPage(ctx, artist.ID, listenArtistLimit)
	if err != nil {
		return youtubemusic.ArtistPage{}, true, err
	}
	if len(page.Tracks) == 0 && len(page.Shelves) == 0 {
		return youtubemusic.ArtistPage{}, false, nil
	}
	if strings.TrimSpace(page.ID) == "" {
		page.ID = strings.TrimSpace(artist.ID)
	}
	if strings.TrimSpace(page.Title) == "" {
		page.Title = strings.TrimSpace(artist.Name)
	}
	if strings.TrimSpace(page.ThumbnailURL) == "" {
		page.ThumbnailURL = strings.TrimSpace(artist.ThumbnailURL)
	}
	return page, true, nil
}

func bestListenArtistSearchMatch(artists []youtubemusic.Artist, artistName string) (youtubemusic.Artist, bool) {
	normalizedName := normalizeListenArtistMatchName(artistName)
	var first youtubemusic.Artist
	for _, artist := range artists {
		if strings.TrimSpace(artist.ID) == "" {
			continue
		}
		if strings.TrimSpace(first.ID) == "" {
			first = artist
		}
		if normalizedName != "" && normalizeListenArtistMatchName(artist.Name) == normalizedName {
			return artist, true
		}
	}
	if strings.TrimSpace(first.ID) == "" {
		return youtubemusic.Artist{}, false
	}
	return first, true
}

func normalizeListenArtistMatchName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func tracksFromListenShelves(shelves []youtubemusic.Shelf) []youtubemusic.Track {
	tracks := make([]youtubemusic.Track, 0)
	seen := make(map[string]struct{})
	for _, shelf := range shelves {
		for _, track := range shelf.Tracks {
			videoID := strings.TrimSpace(track.VideoID)
			if videoID == "" {
				continue
			}
			if _, exists := seen[videoID]; exists {
				continue
			}
			seen[videoID] = struct{}{}
			tracks = append(tracks, track)
		}
	}
	return tracks
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
