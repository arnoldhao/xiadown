package http

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const dreamFMImageCacheControl = "public, max-age=31536000, immutable"

type dreamFMImageCache interface {
	Image(context.Context, string) (youtubemusic.ImageResult, error)
}

type DreamFMImageHandler struct {
	cache dreamFMImageCache
}

func NewDreamFMImageHandler(cache dreamFMImageCache) *DreamFMImageHandler {
	return &DreamFMImageHandler{cache: cache}
}

func (handler *DreamFMImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if handler.cache == nil {
		http.Error(w, "image cache unavailable", http.StatusServiceUnavailable)
		return
	}
	rawURL := r.URL.Query().Get("url")
	result, err := handler.cache.Image(r.Context(), rawURL)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, youtubemusic.ErrImageURLInvalid) {
			status = http.StatusBadRequest
		} else if errors.Is(err, youtubemusic.ErrImageUnavailable) {
			status = http.StatusBadGateway
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	if result.ContentType != "" {
		w.Header().Set("Content-Type", result.ContentType)
	}
	w.Header().Set("Cache-Control", dreamFMImageCacheControl)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, result.CacheKey, imageResultModTime(result), bytes.NewReader(result.Data))
}

func imageResultModTime(result youtubemusic.ImageResult) time.Time {
	if result.ModTime.IsZero() {
		return time.Now()
	}
	return result.ModTime
}
