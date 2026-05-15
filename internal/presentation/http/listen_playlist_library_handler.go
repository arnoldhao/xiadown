package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const listenPlaylistLibraryTimeout = 25 * time.Second

type listenYouTubeMusicPlaylistLibraryClient interface {
	SubscribePlaylist(ctx context.Context, playlistID string) error
	UnsubscribePlaylist(ctx context.Context, playlistID string) error
}

type ListenPlaylistLibraryHandler struct {
	ytMusic listenYouTubeMusicPlaylistLibraryClient
}

type listenPlaylistLibraryPayload struct {
	PlaylistID string `json:"playlistId"`
	Action     string `json:"action"`
}

type listenPlaylistLibraryResponse struct {
	OK bool `json:"ok"`
}

func NewListenPlaylistLibraryHandler(ytMusic listenYouTubeMusicPlaylistLibraryClient) *ListenPlaylistLibraryHandler {
	return &ListenPlaylistLibraryHandler{ytMusic: ytMusic}
}

func (handler *ListenPlaylistLibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeListenMethodNotAllowed(w, r)
		return
	}
	setCORSHeaders(w, r)

	if handler.ytMusic == nil {
		writeListenServiceUnavailable(w, r, "youtube_music_client_unavailable", "YouTube Music client unavailable.", "")
		return
	}

	var payload listenPlaylistLibraryPayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&payload); err != nil {
		writeListenBadRequest(w, r, "invalid_request_body", "Invalid request body.", err.Error())
		return
	}

	playlistID := strings.TrimSpace(payload.PlaylistID)
	if playlistID == "" {
		writeListenBadRequest(w, r, "invalid_playlist_id", "Invalid YouTube Music playlist id.", "")
		return
	}
	action := strings.TrimSpace(payload.Action)
	if action != "add" && action != "remove" {
		writeListenBadRequest(w, r, "invalid_playlist_library_action", "Invalid playlist library action.", "action: "+action)
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenPlaylistLibraryTimeout)
	defer cancel()

	var err error
	switch action {
	case "add":
		err = handler.ytMusic.SubscribePlaylist(ctx, playlistID)
	case "remove":
		err = handler.ytMusic.UnsubscribePlaylist(ctx, playlistID)
	}
	if err != nil {
		writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music playlist library unavailable.", "playlist_library")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(listenPlaylistLibraryResponse{OK: true})
}
