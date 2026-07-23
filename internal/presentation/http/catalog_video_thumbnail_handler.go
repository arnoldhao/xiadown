package http

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	libraryservice "xiadown/internal/application/library/service"
)

type CatalogVideoThumbnailResolver interface {
	Resolve(context.Context, string) (libraryservice.CatalogVideoThumbnail, error)
}

// CatalogVideoThumbnailHandler deliberately accepts only an opaque Catalog
// item ID. Local paths and executable selection remain inside the application
// service and the whole route is additionally guarded by the Desktop server's
// per-process access token.
type CatalogVideoThumbnailHandler struct {
	resolver CatalogVideoThumbnailResolver
}

func NewCatalogVideoThumbnailHandler(resolver CatalogVideoThumbnailResolver) *CatalogVideoThumbnailHandler {
	return &CatalogVideoThumbnailHandler{resolver: resolver}
}

func (handler *CatalogVideoThumbnailHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodOptions {
		setCORSHeaders(response, request)
		response.WriteHeader(http.StatusNoContent)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(response, request)
	if handler == nil || handler.resolver == nil {
		http.Error(response, "thumbnail unavailable", http.StatusServiceUnavailable)
		return
	}

	itemID := strings.TrimPrefix(request.URL.Path, "/api/library/video-thumbnail/")
	if !libraryservice.ValidCatalogVideoThumbnailItemID(itemID) {
		http.NotFound(response, request)
		return
	}
	itemID = strings.TrimSpace(itemID)
	thumbnail, err := handler.resolver.Resolve(request.Context(), itemID)
	if err != nil {
		if errors.Is(err, libraryservice.ErrCatalogVideoThumbnailNotFound) {
			http.NotFound(response, request)
			return
		}
		http.Error(response, "thumbnail unavailable", http.StatusServiceUnavailable)
		return
	}
	if thumbnail.Release != nil {
		defer thumbnail.Release()
	}
	file, err := os.Open(thumbnail.Path)
	if err != nil {
		http.Error(response, "thumbnail unavailable", http.StatusServiceUnavailable)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(response, "thumbnail unavailable", http.StatusServiceUnavailable)
		return
	}

	response.Header().Set("Content-Type", "image/jpeg")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Cache-Control", "private, max-age=3600")
	if thumbnail.ETag != "" {
		response.Header().Set("ETag", thumbnail.ETag)
		if request.Header.Get("If-None-Match") == thumbnail.ETag {
			response.WriteHeader(http.StatusNotModified)
			return
		}
	}
	modTime := thumbnail.ModTime
	if modTime.IsZero() {
		modTime = info.ModTime()
	}
	http.ServeContent(response, request, "video-thumbnail.jpg", modTime, file)
}
