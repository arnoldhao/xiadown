package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const (
	listenRadioLimit   = 24
	listenRadioTimeout = 25 * time.Second
)

type listenYouTubeMusicRadioClient interface {
	Radio(ctx context.Context, videoID string, limit int) ([]youtubemusic.Track, error)
}

type ListenRadioHandler struct {
	ytMusic listenYouTubeMusicRadioClient
}

func NewListenRadioHandler(ytMusic listenYouTubeMusicRadioClient) *ListenRadioHandler {
	return &ListenRadioHandler{ytMusic: ytMusic}
}

func (handler *ListenRadioHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	videoID := strings.TrimSpace(r.URL.Query().Get("id"))
	if !youtubeVideoIDPattern.MatchString(videoID) {
		writeListenBadRequest(w, r, "invalid_video_id", "Invalid YouTube video id.", "id: "+videoID)
		return
	}
	if handler.ytMusic == nil {
		writeListenServiceUnavailable(w, r, "youtube_music_client_unavailable", "YouTube Music client unavailable.", "")
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenRadioTimeout)
	defer cancel()

	tracks, err := handler.ytMusic.Radio(ctx, videoID, listenRadioLimit)
	if err != nil {
		writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music radio unavailable.", "radio")
		return
	}
	tracks = enrichListenTrackDurations(ctx, handler.ytMusic, tracks)
	writeListenSearchJSON(w, r, ListenSearchResponse{Items: mapYouTubeMusicTracksToListenItems(tracks, "ytmusic-radio")})
}
