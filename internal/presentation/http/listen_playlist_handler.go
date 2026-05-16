package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const (
	listenPlaylistLimit   = 100
	listenPlaylistTimeout = 25 * time.Second
)

type listenYouTubeMusicPlaylistClient interface {
	PlaylistQueue(ctx context.Context, playlistID string, limit int) ([]youtubemusic.Track, error)
	PlaylistPage(ctx context.Context, playlistID string, continuation string, limit int) (youtubemusic.TrackListPage, error)
}

type ListenPlaylistHandler struct {
	ytMusic listenYouTubeMusicPlaylistClient
}

func NewListenPlaylistHandler(ytMusic listenYouTubeMusicPlaylistClient) *ListenPlaylistHandler {
	return &ListenPlaylistHandler{ytMusic: ytMusic}
}

func (handler *ListenPlaylistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeListenMethodNotAllowed(w, r)
		return
	}
	setCORSHeaders(w, r)

	playlistID := strings.TrimSpace(r.URL.Query().Get("id"))
	continuation := strings.TrimSpace(r.URL.Query().Get("continuation"))
	if playlistID == "" && continuation == "" {
		writeListenBadRequest(w, r, "invalid_playlist_id", "Invalid YouTube Music playlist id.", "")
		return
	}
	if strings.HasPrefix(playlistID, "MPSPP") {
		writeListenBadRequest(w, r, "unsupported_playlist_id", "Podcast playlists are not supported.", "")
		return
	}
	if handler.ytMusic == nil {
		writeListenServiceUnavailable(w, r, "youtube_music_client_unavailable", "YouTube Music client unavailable.", "")
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenPlaylistTimeout)
	defer cancel()

	page, err := handler.ytMusic.PlaylistPage(ctx, playlistID, continuation, listenPlaylistLimit)
	if err != nil {
		writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music playlist unavailable.", "playlist")
		return
	}
	tracks := page.Tracks
	tracks = enrichListenTrackDurations(ctx, handler.ytMusic, tracks)
	writeListenSearchJSON(w, r, ListenSearchResponse{
		Items:        mapYouTubeMusicTracksToListenItems(tracks, "ytmusic-playlist-track"),
		Continuation: page.Continuation,
		Title:        strings.TrimSpace(page.Title),
		Author:       strings.TrimSpace(page.Author),
	})
}
