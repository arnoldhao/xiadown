package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xiadown/internal/application/imagecache"
)

const listenImageCacheControl = "public, max-age=31536000, immutable"

type listenImageCache interface {
	Image(context.Context, string) (imagecache.ImageResult, error)
}

type listenImageVariantCache interface {
	ImageVariant(context.Context, string, int) (imagecache.ImageResult, error)
}

type listenImagePrefetchCache interface {
	Prefetch(context.Context, []string, int, int)
}

type ListenImageHandler struct {
	cache listenImageCache
}

type listenImagePrefetchRequest struct {
	URLs          []string `json:"urls"`
	Size          int      `json:"size,omitempty"`
	MaxConcurrent int      `json:"maxConcurrent,omitempty"`
}

func NewListenImageHandler(cache listenImageCache) *ListenImageHandler {
	return &ListenImageHandler{cache: cache}
}

func (handler *ListenImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if isListenImagePrefetchPath(r.URL.Path) {
		handler.servePrefetch(w, r)
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
	result, err := handler.imageResult(r.Context(), r, rawURL)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, imagecache.ErrImageURLInvalid) {
			status = http.StatusBadRequest
		} else if errors.Is(err, imagecache.ErrImageUnavailable) {
			status = http.StatusBadGateway
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	if result.ContentType != "" {
		w.Header().Set("Content-Type", result.ContentType)
	}
	w.Header().Set("Cache-Control", listenImageCacheControl)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, result.CacheKey, imageResultModTime(result), bytes.NewReader(result.Data))
}

func (handler *ListenImageHandler) servePrefetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(w, r)
	prefetchCache, ok := handler.cache.(listenImagePrefetchCache)
	if handler.cache == nil || !ok {
		http.Error(w, "image cache unavailable", http.StatusServiceUnavailable)
		return
	}
	var request listenImagePrefetchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	urls := normalizeListenImagePrefetchURLs(request.URLs)
	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	go prefetchCache.Prefetch(context.WithoutCancel(r.Context()), urls, request.Size, request.MaxConcurrent)
	w.WriteHeader(http.StatusAccepted)
}

func (handler *ListenImageHandler) imageResult(ctx context.Context, r *http.Request, rawURL string) (imagecache.ImageResult, error) {
	size := listenImageVariantSize(r)
	if size > 0 {
		if variantCache, ok := handler.cache.(listenImageVariantCache); ok {
			return variantCache.ImageVariant(ctx, rawURL, size)
		}
	}
	return handler.cache.Image(ctx, rawURL)
}

func listenImageVariantSize(r *http.Request) int {
	for _, key := range []string{"size", "s"} {
		if size := parseListenImageVariantSize(r.URL.Query().Get(key)); size > 0 {
			return size
		}
	}
	width := parseListenImageVariantSize(r.URL.Query().Get("w"))
	height := parseListenImageVariantSize(r.URL.Query().Get("h"))
	if width > height {
		return width
	}
	return height
}

func parseListenImageVariantSize(value string) int {
	size, err := strconv.Atoi(value)
	if err != nil || size <= 0 {
		return 0
	}
	return size
}

func isListenImagePrefetchPath(path string) bool {
	return strings.TrimRight(path, "/") == "/api/listen/image/prefetch"
}

func normalizeListenImagePrefetchURLs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := listenImagePrefetchSourceURL(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
		if len(result) >= 96 {
			break
		}
	}
	return result
}

func listenImagePrefetchSourceURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	if strings.TrimRight(parsed.Path, "/") == "/api/listen/image" {
		if source := strings.TrimSpace(parsed.Query().Get("url")); source != "" {
			return source
		}
	}
	return trimmed
}

func imageResultModTime(result imagecache.ImageResult) time.Time {
	if result.ModTime.IsZero() {
		return time.Now()
	}
	return result.ModTime
}
