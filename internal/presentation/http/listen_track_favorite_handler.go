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

const listenTrackFavoriteTimeout = 25 * time.Second
const listenTrackFavoriteBatchLimit = 50

type listenYouTubeMusicTrackFavoriteClient interface {
	TrackMetadata(ctx context.Context, videoID string) (youtubemusic.TrackMetadata, error)
	RateSong(ctx context.Context, videoID string, rating youtubemusic.LikeStatus) error
	LikedSongs(ctx context.Context, limit int) ([]youtubemusic.Track, error)
}

type listenTrackFavoriteScopeClient interface {
	FavoriteCacheScope(ctx context.Context) string
}

type ListenTrackFavoriteHandler struct {
	ytMusic         listenYouTubeMusicTrackFavoriteClient
	mu              sync.Mutex
	favoriteByVideo map[string]bool
}

type listenTrackFavoritePayload struct {
	VideoID string `json:"videoId"`
	Liked   bool   `json:"liked"`
}

type listenTrackFavoriteResponse struct {
	OK        bool                       `json:"ok,omitempty"`
	VideoID   string                     `json:"videoId,omitempty"`
	Liked     bool                       `json:"liked"`
	Known     bool                       `json:"known"`
	Favorites []listenTrackFavoriteState `json:"favorites,omitempty"`
}

type listenTrackFavoriteState struct {
	VideoID string `json:"videoId"`
	Liked   bool   `json:"liked"`
	Known   bool   `json:"known"`
}

func NewListenTrackFavoriteHandler(ytMusic listenYouTubeMusicTrackFavoriteClient) *ListenTrackFavoriteHandler {
	return &ListenTrackFavoriteHandler{ytMusic: ytMusic}
}

func (handler *ListenTrackFavoriteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	setCORSHeaders(w, r)

	if handler.ytMusic == nil {
		writeListenServiceUnavailable(w, r, "youtube_music_client_unavailable", "YouTube Music client unavailable.", "")
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		handler.handleRead(w, r)
	case http.MethodPost:
		handler.handleWrite(w, r)
	default:
		writeListenMethodNotAllowed(w, r)
	}
}

func (handler *ListenTrackFavoriteHandler) handleRead(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(r.URL.Query().Get("ids")) != "" {
		handler.handleReadBatch(w, r)
		return
	}

	videoID := strings.TrimSpace(r.URL.Query().Get("id"))
	if !youtubeVideoIDPattern.MatchString(videoID) {
		writeListenBadRequest(w, r, "invalid_video_id", "Invalid YouTube video id.", "id: "+videoID)
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenTrackFavoriteTimeout)
	defer cancel()
	scope := handler.favoriteCacheScope(ctx)

	if liked, ok := handler.cachedFavorite(scope, videoID); ok {
		writeListenTrackFavoriteJSON(w, r, listenTrackFavoriteResponse{
			VideoID: videoID,
			Liked:   liked,
			Known:   true,
		})
		return
	}

	metadata, err := handler.ytMusic.TrackMetadata(ctx, videoID)
	if err != nil {
		writeListenTrackFavoriteJSON(w, r, listenTrackFavoriteResponse{
			VideoID: videoID,
			Liked:   false,
			Known:   false,
		})
		return
	}
	if !metadata.LikeStatusKnown {
		writeListenTrackFavoriteJSON(w, r, listenTrackFavoriteResponse{
			VideoID: videoID,
			Liked:   false,
			Known:   false,
		})
		return
	}

	liked := metadata.LikeStatus == youtubemusic.LikeStatusLike
	handler.setCachedFavorite(scope, videoID, liked)
	writeListenTrackFavoriteJSON(w, r, listenTrackFavoriteResponse{
		VideoID: videoID,
		Liked:   liked,
		Known:   true,
	})
}

func (handler *ListenTrackFavoriteHandler) handleReadBatch(w http.ResponseWriter, r *http.Request) {
	videoIDs := cleanListenFavoriteVideoIDs(r.URL.Query().Get("ids"), listenTrackFavoriteBatchLimit)
	if len(videoIDs) == 0 {
		writeListenBadRequest(w, r, "invalid_video_id", "Invalid YouTube video id.", "")
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenTrackFavoriteTimeout)
	defer cancel()
	scope := handler.favoriteCacheScope(ctx)

	handler.primeCachedFavorites(ctx, scope, videoIDs)

	favorites := make([]listenTrackFavoriteState, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		liked, ok := handler.cachedFavorite(scope, videoID)
		if !ok {
			continue
		}
		favorites = append(favorites, listenTrackFavoriteState{
			VideoID: videoID,
			Liked:   liked,
			Known:   true,
		})
	}

	writeListenTrackFavoriteJSON(w, r, listenTrackFavoriteResponse{
		OK:        true,
		Favorites: favorites,
	})
}

func (handler *ListenTrackFavoriteHandler) handleWrite(w http.ResponseWriter, r *http.Request) {
	var payload listenTrackFavoritePayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&payload); err != nil {
		writeListenBadRequest(w, r, "invalid_request_body", "Invalid request body.", err.Error())
		return
	}

	videoID := strings.TrimSpace(payload.VideoID)
	if !youtubeVideoIDPattern.MatchString(videoID) {
		writeListenBadRequest(w, r, "invalid_video_id", "Invalid YouTube video id.", "videoId: "+videoID)
		return
	}

	rating := youtubemusic.LikeStatusIndifferent
	if payload.Liked {
		rating = youtubemusic.LikeStatusLike
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenTrackFavoriteTimeout)
	defer cancel()
	scope := handler.favoriteCacheScope(ctx)

	if err := handler.ytMusic.RateSong(ctx, videoID, rating); err != nil {
		writeListenYouTubeMusicUnavailable(w, r, err, "YouTube Music favorite update unavailable.", "favorite")
		return
	}

	handler.setCachedFavorite(scope, videoID, payload.Liked)
	writeListenTrackFavoriteJSON(w, r, listenTrackFavoriteResponse{
		OK:      true,
		VideoID: videoID,
		Liked:   payload.Liked,
		Known:   true,
	})
}

func (handler *ListenTrackFavoriteHandler) primeCachedFavorites(ctx context.Context, scope string, videoIDs []string) {
	if handler == nil || handler.ytMusic == nil || len(videoIDs) == 0 {
		return
	}
	likedSongs, err := handler.ytMusic.LikedSongs(ctx, listenTrackFavoriteBatchLimit)
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
		if _, ok := handler.cachedFavorite(scope, videoID); ok {
			continue
		}
		handler.setCachedFavorite(scope, videoID, true)
	}
}

func cleanListenFavoriteVideoIDs(value string, limit int) []string {
	if limit <= 0 {
		limit = listenTrackFavoriteBatchLimit
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

func (handler *ListenTrackFavoriteHandler) cachedFavorite(scope string, videoID string) (bool, bool) {
	if handler == nil {
		return false, false
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if handler.favoriteByVideo == nil {
		return false, false
	}
	liked, ok := handler.favoriteByVideo[listenFavoriteCacheKey(scope, videoID)]
	return liked, ok
}

func (handler *ListenTrackFavoriteHandler) setCachedFavorite(scope string, videoID string, liked bool) {
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
	handler.favoriteByVideo[listenFavoriteCacheKey(scope, trimmed)] = liked
}

func (handler *ListenTrackFavoriteHandler) favoriteCacheScope(ctx context.Context) string {
	if handler == nil || handler.ytMusic == nil {
		return "default"
	}
	scopeClient, ok := handler.ytMusic.(listenTrackFavoriteScopeClient)
	if !ok {
		return "default"
	}
	if scope := strings.TrimSpace(scopeClient.FavoriteCacheScope(ctx)); scope != "" {
		return scope
	}
	return "default"
}

func listenFavoriteCacheKey(scope string, videoID string) string {
	return strings.TrimSpace(scope) + "\x00" + strings.TrimSpace(videoID)
}

func writeListenTrackFavoriteJSON(w http.ResponseWriter, r *http.Request, response listenTrackFavoriteResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
