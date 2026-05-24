package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const listenTrackTimeout = 25 * time.Second

type listenYouTubeMusicTrackClient interface {
	TrackMetadata(ctx context.Context, videoID string) (youtubemusic.TrackMetadata, error)
}

type ListenTrackHandler struct {
	ytMusic listenYouTubeMusicTrackClient
}

type ListenTrackResponse struct {
	Item ListenSearchItem `json:"item"`
}

func NewListenTrackHandler(ytMusic listenYouTubeMusicTrackClient) *ListenTrackHandler {
	return &ListenTrackHandler{ytMusic: ytMusic}
}

func (handler *ListenTrackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if handler.ytMusic == nil {
		writeListenServiceUnavailable(w, r, "youtube_music_client_unavailable", "YouTube Music client unavailable.", "")
		return
	}

	videoID := strings.TrimSpace(r.URL.Query().Get("id"))
	if !youtubeVideoIDPattern.MatchString(videoID) {
		writeListenBadRequest(w, r, "invalid_video_id", "Invalid YouTube video id.", "id: "+videoID)
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenTrackTimeout)
	defer cancel()

	metadata, err := handler.ytMusic.TrackMetadata(ctx, videoID)
	if err != nil {
		writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music track metadata unavailable.", "track")
		return
	}
	writeListenTrackJSON(w, r, ListenTrackResponse{
		Item: mapYouTubeMusicTrackMetadataToListenItem(metadata, videoID),
	})
}

func mapYouTubeMusicTrackMetadataToListenItem(metadata youtubemusic.TrackMetadata, fallbackVideoID string) ListenSearchItem {
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
	thumbnailURL := strings.TrimSpace(metadata.ThumbnailURL)
	hasVideo, videoAvailabilityKnown := listenTrackVideoAvailability(musicVideoType, videoID, thumbnailURL)
	return ListenSearchItem{
		ID:                     "ytmusic-track-" + videoID,
		Group:                  "playlist",
		VideoID:                videoID,
		Title:                  title,
		Channel:                channel,
		Artists:                mapYouTubeMusicTrackArtistsToListenArtistItems(metadata.Artists, "ytmusic-track-artist"),
		ArtistBrowseID:         strings.TrimSpace(metadata.ArtistBrowseID),
		ArtistSource:           strings.TrimSpace(metadata.ArtistSource),
		Description:            "",
		DurationLabel:          strings.TrimSpace(metadata.DurationLabel),
		ThumbnailURL:           thumbnailURL,
		MusicVideoType:         musicVideoType,
		HasVideo:               hasVideo,
		VideoAvailabilityKnown: videoAvailabilityKnown,
	}
}

func writeListenTrackJSON(w http.ResponseWriter, r *http.Request, response ListenTrackResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
