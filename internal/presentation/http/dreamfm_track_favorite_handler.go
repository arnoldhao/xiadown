package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const dreamFMTrackFavoriteTimeout = 25 * time.Second
const dreamFMTrackFavoriteBatchLimit = 50

type dreamFMYouTubeMusicTrackFavoriteClient interface {
	TrackMetadata(ctx context.Context, videoID string) (youtubemusic.TrackMetadata, error)
	RateSong(ctx context.Context, videoID string, rating youtubemusic.LikeStatus) error
	LikedSongs(ctx context.Context, limit int) ([]youtubemusic.Track, error)
}

type DreamFMTrackFavoriteHandler struct {
	ytMusic         dreamFMYouTubeMusicTrackFavoriteClient
	mu              sync.Mutex
	favoriteByVideo map[string]bool
}

type dreamFMTrackFavoritePayload struct {
	VideoID string `json:"videoId"`
	Liked   bool   `json:"liked"`
}

type dreamFMTrackFavoriteResponse struct {
	OK        bool                        `json:"ok,omitempty"`
	VideoID   string                      `json:"videoId,omitempty"`
	Liked     bool                        `json:"liked"`
	Known     bool                        `json:"known"`
	Favorites []dreamFMTrackFavoriteState `json:"favorites,omitempty"`
}

type dreamFMTrackFavoriteState struct {
	VideoID string `json:"videoId"`
	Liked   bool   `json:"liked"`
	Known   bool   `json:"known"`
}

func NewDreamFMTrackFavoriteHandler(ytMusic dreamFMYouTubeMusicTrackFavoriteClient) *DreamFMTrackFavoriteHandler {
	return &DreamFMTrackFavoriteHandler{ytMusic: ytMusic}
}

func (handler *DreamFMTrackFavoriteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	setCORSHeaders(w, r)

	if handler.ytMusic == nil {
		http.Error(w, "youtube music client unavailable", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		handler.handleRead(w, r)
	case http.MethodPost:
		handler.handleWrite(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (handler *DreamFMTrackFavoriteHandler) handleRead(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(r.URL.Query().Get("ids")) != "" {
		handler.handleReadBatch(w, r)
		return
	}

	videoID := strings.TrimSpace(r.URL.Query().Get("id"))
	if !youtubeVideoIDPattern.MatchString(videoID) {
		http.Error(w, "invalid youtube video id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMTrackFavoriteTimeout)
	defer cancel()

	metadata, err := handler.ytMusic.TrackMetadata(ctx, videoID)
	if err != nil {
		if liked, ok := handler.cachedFavorite(videoID); ok {
			writeDreamFMTrackFavoriteJSON(w, r, dreamFMTrackFavoriteResponse{
				VideoID: videoID,
				Liked:   liked,
				Known:   true,
			})
			return
		}
		writeDreamFMTrackFavoriteJSON(w, r, dreamFMTrackFavoriteResponse{
			VideoID: videoID,
			Liked:   false,
			Known:   false,
		})
		return
	}
	if !metadata.LikeStatusKnown {
		if liked, ok := handler.cachedFavorite(videoID); ok {
			writeDreamFMTrackFavoriteJSON(w, r, dreamFMTrackFavoriteResponse{
				VideoID: videoID,
				Liked:   liked,
				Known:   true,
			})
			return
		}
		writeDreamFMTrackFavoriteJSON(w, r, dreamFMTrackFavoriteResponse{
			VideoID: videoID,
			Liked:   false,
			Known:   false,
		})
		return
	}

	liked := metadata.LikeStatus == youtubemusic.LikeStatusLike
	handler.setCachedFavorite(videoID, liked)
	writeDreamFMTrackFavoriteJSON(w, r, dreamFMTrackFavoriteResponse{
		VideoID: videoID,
		Liked:   liked,
		Known:   true,
	})
}

func (handler *DreamFMTrackFavoriteHandler) handleReadBatch(w http.ResponseWriter, r *http.Request) {
	videoIDs := cleanDreamFMFavoriteVideoIDs(r.URL.Query().Get("ids"), dreamFMTrackFavoriteBatchLimit)
	if len(videoIDs) == 0 {
		http.Error(w, "invalid youtube video id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMTrackFavoriteTimeout)
	defer cancel()

	handler.primeCachedFavorites(ctx, videoIDs)

	favorites := make([]dreamFMTrackFavoriteState, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		liked, ok := handler.cachedFavorite(videoID)
		if !ok {
			continue
		}
		favorites = append(favorites, dreamFMTrackFavoriteState{
			VideoID: videoID,
			Liked:   liked,
			Known:   true,
		})
	}

	writeDreamFMTrackFavoriteJSON(w, r, dreamFMTrackFavoriteResponse{
		OK:        true,
		Favorites: favorites,
	})
}

func (handler *DreamFMTrackFavoriteHandler) handleWrite(w http.ResponseWriter, r *http.Request) {
	var payload dreamFMTrackFavoritePayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	videoID := strings.TrimSpace(payload.VideoID)
	if videoID == "" {
		http.Error(w, "invalid youtube video id", http.StatusBadRequest)
		return
	}

	rating := youtubemusic.LikeStatusIndifferent
	if payload.Liked {
		rating = youtubemusic.LikeStatusLike
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMTrackFavoriteTimeout)
	defer cancel()

	if err := handler.ytMusic.RateSong(ctx, videoID, rating); err != nil {
		http.Error(w, "youtube music favorite update unavailable", http.StatusServiceUnavailable)
		return
	}

	handler.setCachedFavorite(videoID, payload.Liked)
	writeDreamFMTrackFavoriteJSON(w, r, dreamFMTrackFavoriteResponse{
		OK:      true,
		VideoID: videoID,
		Liked:   payload.Liked,
		Known:   true,
	})
}

func (handler *DreamFMTrackFavoriteHandler) primeCachedFavorites(ctx context.Context, videoIDs []string) {
	if handler == nil || handler.ytMusic == nil || len(videoIDs) == 0 {
		return
	}
	likedSongs, err := handler.ytMusic.LikedSongs(ctx, dreamFMTrackFavoriteBatchLimit)
	if err != nil {
		return
	}
	requested := make(map[string]struct{}, len(videoIDs))
	for _, videoID := range videoIDs {
		requested[videoID] = struct{}{}
	}
	for _, track := range likedSongs {
		videoID := strings.TrimSpace(track.VideoID)
		if _, ok := requested[videoID]; !ok {
			continue
		}
		handler.setCachedFavorite(videoID, true)
	}
}

func cleanDreamFMFavoriteVideoIDs(value string, limit int) []string {
	if limit <= 0 {
		limit = dreamFMTrackFavoriteBatchLimit
	}
	seen := make(map[string]struct{}, limit)
	parts := strings.FieldsFunc(value, func(character rune) bool {
		return character == ',' || character == ' ' || character == '\n' || character == '\t'
	})
	result := make([]string, 0, min(len(parts), limit))
	for _, part := range parts {
		videoID := strings.TrimSpace(part)
		if !youtubeVideoIDPattern.MatchString(videoID) {
			continue
		}
		if _, exists := seen[videoID]; exists {
			continue
		}
		seen[videoID] = struct{}{}
		result = append(result, videoID)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func (handler *DreamFMTrackFavoriteHandler) cachedFavorite(videoID string) (bool, bool) {
	if handler == nil {
		return false, false
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if handler.favoriteByVideo == nil {
		return false, false
	}
	liked, ok := handler.favoriteByVideo[strings.TrimSpace(videoID)]
	return liked, ok
}

func (handler *DreamFMTrackFavoriteHandler) setCachedFavorite(videoID string, liked bool) {
	if handler == nil {
		return
	}
	trimmed := strings.TrimSpace(videoID)
	if trimmed == "" {
		return
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if handler.favoriteByVideo == nil {
		handler.favoriteByVideo = make(map[string]bool)
	}
	handler.favoriteByVideo[trimmed] = liked
}

func writeDreamFMTrackFavoriteJSON(w http.ResponseWriter, r *http.Request, response dreamFMTrackFavoriteResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
