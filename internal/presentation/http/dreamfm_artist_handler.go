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
	dreamFMArtistLimit   = 50
	dreamFMArtistTimeout = 25 * time.Second
)

type dreamFMYouTubeMusicArtistClient interface {
	ArtistPage(ctx context.Context, browseID string, limit int) (youtubemusic.ArtistPage, error)
	BrowseShelvesPage(ctx context.Context, browseID string, params string, continuation string, sectionLimit int, itemLimit int) (youtubemusic.BrowsePage, error)
	SearchSongs(ctx context.Context, query string, limit int) ([]youtubemusic.Track, error)
	SubscribeArtist(ctx context.Context, channelID string) error
	UnsubscribeArtist(ctx context.Context, channelID string) error
}

type DreamFMArtistHandler struct {
	ytMusic dreamFMYouTubeMusicArtistClient
}

type DreamFMArtistResponse struct {
	ID            string                `json:"id"`
	Title         string                `json:"title"`
	Subtitle      string                `json:"subtitle,omitempty"`
	ChannelID     string                `json:"channelId,omitempty"`
	IsSubscribed  bool                  `json:"isSubscribed,omitempty"`
	MixPlaylistID string                `json:"mixPlaylistId,omitempty"`
	MixVideoID    string                `json:"mixVideoId,omitempty"`
	Items         []DreamFMSearchItem   `json:"items"`
	Shelves       []DreamFMLibraryShelf `json:"shelves"`
	Continuation  string                `json:"continuation,omitempty"`
}

type dreamFMArtistSubscriptionPayload struct {
	ChannelID  string `json:"channelId"`
	Subscribed bool   `json:"subscribed"`
}

type dreamFMArtistSubscriptionResponse struct {
	OK         bool `json:"ok,omitempty"`
	Subscribed bool `json:"subscribed"`
}

func NewDreamFMArtistHandler(ytMusic dreamFMYouTubeMusicArtistClient) *DreamFMArtistHandler {
	return &DreamFMArtistHandler{ytMusic: ytMusic}
}

func (handler *DreamFMArtistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(w, r)

	if handler.ytMusic == nil {
		http.Error(w, "youtube music client unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodPost {
		handler.handleSubscription(w, r)
		return
	}

	browseID := strings.TrimSpace(r.URL.Query().Get("id"))
	artistName := strings.TrimSpace(r.URL.Query().Get("name"))
	continuation := strings.TrimSpace(r.URL.Query().Get("continuation"))
	if browseID == "" && continuation == "" && len([]rune(artistName)) < 2 {
		http.Error(w, "invalid youtube music artist", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMArtistTimeout)
	defer cancel()

	if continuation != "" {
		page, err := handler.ytMusic.BrowseShelvesPage(ctx, "", "", continuation, dreamFMArtistLimit, dreamFMArtistLimit)
		if err != nil {
			http.Error(w, "youtube music artist unavailable", http.StatusServiceUnavailable)
			return
		}
		page.Shelves = enrichDreamFMShelfTrackDurations(ctx, handler.ytMusic, page.Shelves)
		writeDreamFMArtistJSON(w, r, DreamFMArtistResponse{
			ID:           browseID,
			Title:        firstNonEmptyString(artistName, browseID),
			Items:        mapYouTubeMusicTracksToDreamFMItems(tracksFromDreamFMShelves(page.Shelves), "ytmusic-artist"),
			Shelves:      mapYouTubeMusicShelvesToDreamFMShelvesWithPrefixes(page.Shelves, "ytmusic-artist", "ytmusic-artist-playlist"),
			Continuation: page.Continuation,
		})
		return
	}

	if browseID != "" {
		page, err := handler.ytMusic.ArtistPage(ctx, browseID, dreamFMArtistLimit)
		if err == nil && (len(page.Tracks) > 0 || len(page.Shelves) > 0) {
			page.Tracks = enrichDreamFMTrackDurations(ctx, handler.ytMusic, page.Tracks)
			page.Shelves = enrichDreamFMShelfTrackDurations(ctx, handler.ytMusic, page.Shelves)
			writeDreamFMArtistJSON(w, r, DreamFMArtistResponse{
				ID:            page.ID,
				Title:         firstNonEmptyString(page.Title, artistName, page.ID),
				Subtitle:      page.Subtitle,
				ChannelID:     page.ChannelID,
				IsSubscribed:  page.IsSubscribed,
				MixPlaylistID: page.MixPlaylistID,
				MixVideoID:    page.MixVideoID,
				Items:         mapYouTubeMusicTracksToDreamFMItems(page.Tracks, "ytmusic-artist"),
				Shelves:       mapYouTubeMusicShelvesToDreamFMShelvesWithPrefixes(page.Shelves, "ytmusic-artist", "ytmusic-artist-playlist"),
				Continuation:  page.Continuation,
			})
			return
		}
		if artistName == "" && err != nil {
			http.Error(w, "youtube music artist unavailable", http.StatusServiceUnavailable)
			return
		}
	}

	if artistName == "" {
		writeDreamFMArtistJSON(w, r, DreamFMArtistResponse{
			ID:      browseID,
			Title:   browseID,
			Items:   []DreamFMSearchItem{},
			Shelves: []DreamFMLibraryShelf{},
		})
		return
	}

	tracks, err := handler.ytMusic.SearchSongs(ctx, artistName, dreamFMArtistLimit)
	if err != nil {
		http.Error(w, "youtube music artist unavailable", http.StatusServiceUnavailable)
		return
	}
	tracks = enrichDreamFMTrackDurations(ctx, handler.ytMusic, tracks)
	writeDreamFMArtistJSON(w, r, DreamFMArtistResponse{
		ID:      browseID,
		Title:   artistName,
		Items:   mapYouTubeMusicTracksToDreamFMItems(tracks, "ytmusic-artist-search"),
		Shelves: []DreamFMLibraryShelf{},
	})
}

func (handler *DreamFMArtistHandler) handleSubscription(w http.ResponseWriter, r *http.Request) {
	var payload dreamFMArtistSubscriptionPayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	channelID := strings.TrimSpace(payload.ChannelID)
	if !strings.HasPrefix(channelID, "UC") {
		http.Error(w, "invalid youtube music artist channel id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMArtistTimeout)
	defer cancel()

	var err error
	if payload.Subscribed {
		err = handler.ytMusic.SubscribeArtist(ctx, channelID)
	} else {
		err = handler.ytMusic.UnsubscribeArtist(ctx, channelID)
	}
	if err != nil {
		http.Error(w, "youtube music artist subscription unavailable", http.StatusServiceUnavailable)
		return
	}
	writeDreamFMArtistSubscriptionJSON(w, r, dreamFMArtistSubscriptionResponse{
		OK:         true,
		Subscribed: payload.Subscribed,
	})
}

func writeDreamFMArtistSubscriptionJSON(w http.ResponseWriter, r *http.Request, response dreamFMArtistSubscriptionResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}

func writeDreamFMArtistJSON(w http.ResponseWriter, r *http.Request, response DreamFMArtistResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}

func tracksFromDreamFMShelves(shelves []youtubemusic.Shelf) []youtubemusic.Track {
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
