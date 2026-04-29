package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const (
	dreamFMRadioLimit   = 24
	dreamFMRadioTimeout = 25 * time.Second
)

type dreamFMYouTubeMusicRadioClient interface {
	Radio(ctx context.Context, videoID string, limit int) ([]youtubemusic.Track, error)
}

type DreamFMRadioHandler struct {
	ytMusic dreamFMYouTubeMusicRadioClient
}

func NewDreamFMRadioHandler(ytMusic dreamFMYouTubeMusicRadioClient) *DreamFMRadioHandler {
	return &DreamFMRadioHandler{ytMusic: ytMusic}
}

func (handler *DreamFMRadioHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	videoID := strings.TrimSpace(r.URL.Query().Get("id"))
	if !youtubeVideoIDPattern.MatchString(videoID) {
		http.Error(w, "invalid youtube video id", http.StatusBadRequest)
		return
	}
	if handler.ytMusic == nil {
		http.Error(w, "youtube music client unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMRadioTimeout)
	defer cancel()

	tracks, err := handler.ytMusic.Radio(ctx, videoID, dreamFMRadioLimit)
	if err != nil {
		http.Error(w, "youtube music radio unavailable", http.StatusServiceUnavailable)
		return
	}
	tracks = enrichDreamFMTrackDurations(ctx, handler.ytMusic, tracks)
	writeDreamFMSearchJSON(w, r, DreamFMSearchResponse{Items: mapYouTubeMusicTracksToDreamFMItems(tracks, "ytmusic-radio")})
}
