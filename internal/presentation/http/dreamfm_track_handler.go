package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const dreamFMTrackTimeout = 25 * time.Second

type dreamFMYouTubeMusicTrackClient interface {
	TrackMetadata(ctx context.Context, videoID string) (youtubemusic.TrackMetadata, error)
}

type DreamFMTrackHandler struct {
	ytMusic dreamFMYouTubeMusicTrackClient
}

type DreamFMTrackResponse struct {
	Item DreamFMSearchItem `json:"item"`
}

func NewDreamFMTrackHandler(ytMusic dreamFMYouTubeMusicTrackClient) *DreamFMTrackHandler {
	return &DreamFMTrackHandler{ytMusic: ytMusic}
}

func (handler *DreamFMTrackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if handler.ytMusic == nil {
		http.Error(w, "youtube music client unavailable", http.StatusServiceUnavailable)
		return
	}

	videoID := strings.TrimSpace(r.URL.Query().Get("id"))
	if !youtubeVideoIDPattern.MatchString(videoID) {
		http.Error(w, "invalid youtube video id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMTrackTimeout)
	defer cancel()

	metadata, err := handler.ytMusic.TrackMetadata(ctx, videoID)
	if err != nil {
		http.Error(w, "youtube music track metadata unavailable", http.StatusServiceUnavailable)
		return
	}
	writeDreamFMTrackJSON(w, r, DreamFMTrackResponse{
		Item: mapYouTubeMusicTrackMetadataToDreamFMItem(metadata, videoID),
	})
}

func mapYouTubeMusicTrackMetadataToDreamFMItem(metadata youtubemusic.TrackMetadata, fallbackVideoID string) DreamFMSearchItem {
	videoID := strings.TrimSpace(metadata.VideoID)
	if !youtubeVideoIDPattern.MatchString(videoID) {
		videoID = strings.TrimSpace(fallbackVideoID)
	}
	title := strings.TrimSpace(metadata.Title)
	if title == "" {
		title = videoID
	}
	channel := strings.TrimSpace(metadata.Channel)
	if channel == "" {
		channel = "YouTube Music"
	}
	musicVideoType := strings.TrimSpace(metadata.MusicVideoType)
	return DreamFMSearchItem{
		ID:                     "ytmusic-track-" + videoID,
		Group:                  "playlist",
		VideoID:                videoID,
		Title:                  title,
		Channel:                channel,
		ArtistBrowseID:         strings.TrimSpace(metadata.ArtistBrowseID),
		Description:            "",
		DurationLabel:          strings.TrimSpace(metadata.DurationLabel),
		ThumbnailURL:           strings.TrimSpace(metadata.ThumbnailURL),
		MusicVideoType:         musicVideoType,
		HasVideo:               dreamFMMusicVideoTypeHasVideo(musicVideoType),
		VideoAvailabilityKnown: musicVideoType != "",
	}
}

func writeDreamFMTrackJSON(w http.ResponseWriter, r *http.Request, response DreamFMTrackResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
