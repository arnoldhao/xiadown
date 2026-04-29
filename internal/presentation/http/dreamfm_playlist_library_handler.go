package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const dreamFMPlaylistLibraryTimeout = 25 * time.Second

type dreamFMYouTubeMusicPlaylistLibraryClient interface {
	SubscribePlaylist(ctx context.Context, playlistID string) error
	UnsubscribePlaylist(ctx context.Context, playlistID string) error
}

type DreamFMPlaylistLibraryHandler struct {
	ytMusic dreamFMYouTubeMusicPlaylistLibraryClient
}

type dreamFMPlaylistLibraryPayload struct {
	PlaylistID string `json:"playlistId"`
	Action     string `json:"action"`
}

type dreamFMPlaylistLibraryResponse struct {
	OK bool `json:"ok"`
}

func NewDreamFMPlaylistLibraryHandler(ytMusic dreamFMYouTubeMusicPlaylistLibraryClient) *DreamFMPlaylistLibraryHandler {
	return &DreamFMPlaylistLibraryHandler{ytMusic: ytMusic}
}

func (handler *DreamFMPlaylistLibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(w, r)

	if handler.ytMusic == nil {
		http.Error(w, "youtube music client unavailable", http.StatusServiceUnavailable)
		return
	}

	var payload dreamFMPlaylistLibraryPayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	playlistID := strings.TrimSpace(payload.PlaylistID)
	if playlistID == "" {
		http.Error(w, "invalid youtube music playlist id", http.StatusBadRequest)
		return
	}
	action := strings.TrimSpace(payload.Action)
	if action != "add" && action != "remove" {
		http.Error(w, "invalid playlist library action", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMPlaylistLibraryTimeout)
	defer cancel()

	var err error
	switch action {
	case "add":
		err = handler.ytMusic.SubscribePlaylist(ctx, playlistID)
	case "remove":
		err = handler.ytMusic.UnsubscribePlaylist(ctx, playlistID)
	}
	if err != nil {
		http.Error(w, "youtube music playlist library unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(dreamFMPlaylistLibraryResponse{OK: true})
}
