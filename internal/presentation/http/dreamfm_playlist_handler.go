package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const (
	dreamFMPlaylistLimit   = 100
	dreamFMPlaylistTimeout = 25 * time.Second
)

type dreamFMYouTubeMusicPlaylistClient interface {
	PlaylistQueue(ctx context.Context, playlistID string, limit int) ([]youtubemusic.Track, error)
	PlaylistPage(ctx context.Context, playlistID string, continuation string, limit int) (youtubemusic.TrackListPage, error)
}

type DreamFMPlaylistHandler struct {
	ytMusic dreamFMYouTubeMusicPlaylistClient
}

func NewDreamFMPlaylistHandler(ytMusic dreamFMYouTubeMusicPlaylistClient) *DreamFMPlaylistHandler {
	return &DreamFMPlaylistHandler{ytMusic: ytMusic}
}

func (handler *DreamFMPlaylistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(w, r)

	playlistID := strings.TrimSpace(r.URL.Query().Get("id"))
	continuation := strings.TrimSpace(r.URL.Query().Get("continuation"))
	if playlistID == "" && continuation == "" {
		http.Error(w, "invalid youtube music playlist id", http.StatusBadRequest)
		return
	}
	if isDreamFMPodcastPlaylistID(playlistID) {
		http.Error(w, "youtube music podcasts unavailable", http.StatusBadRequest)
		return
	}
	if handler.ytMusic == nil {
		http.Error(w, "youtube music client unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMPlaylistTimeout)
	defer cancel()

	page, err := handler.ytMusic.PlaylistPage(ctx, playlistID, continuation, dreamFMPlaylistLimit)
	if err != nil {
		http.Error(w, "youtube music playlist unavailable", http.StatusServiceUnavailable)
		return
	}
	tracks := page.Tracks
	tracks = enrichDreamFMTrackDurations(ctx, handler.ytMusic, tracks)
	writeDreamFMSearchJSON(w, r, DreamFMSearchResponse{
		Items:        mapYouTubeMusicTracksToDreamFMItems(tracks, "ytmusic-playlist-track"),
		Continuation: page.Continuation,
		Title:        strings.TrimSpace(page.Title),
		Author:       strings.TrimSpace(page.Author),
	})
}

func isDreamFMPodcastPlaylistID(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "MPSPP")
}
