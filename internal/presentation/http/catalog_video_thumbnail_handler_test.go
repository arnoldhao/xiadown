package http

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	libraryservice "xiadown/internal/application/library/service"
)

type catalogVideoThumbnailResolverStub struct {
	itemID string
	result libraryservice.CatalogVideoThumbnail
	err    error
}

func (resolver *catalogVideoThumbnailResolverStub) Resolve(
	_ context.Context,
	itemID string,
) (libraryservice.CatalogVideoThumbnail, error) {
	resolver.itemID = itemID
	return resolver.result, resolver.err
}

func TestCatalogVideoThumbnailHandlerServesOpaqueItemPreview(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "preview.jpg")
	if err := writeCatalogVideoThumbnailHandlerJPEG(path); err != nil {
		t.Fatalf("write JPEG: %v", err)
	}
	modTime := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set JPEG time: %v", err)
	}
	releases := 0
	resolver := &catalogVideoThumbnailResolverStub{result: libraryservice.CatalogVideoThumbnail{
		Path: path, ETag: `"thumbnail-etag"`, ModTime: modTime,
		Release: func() { releases++ },
	}}
	handler := NewCatalogVideoThumbnailHandler(resolver)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/library/video-thumbnail/item-1?path=/etc/passwd",
		nil,
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("thumbnail response status = %d, body=%q", response.Code, response.Body.String())
	}
	if resolver.itemID != "item-1" {
		t.Fatalf("resolver item ID = %q", resolver.itemID)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "image/jpeg" {
		t.Fatalf("thumbnail content type = %q", contentType)
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" ||
		response.Header().Get("ETag") != `"thumbnail-etag"` {
		t.Fatalf("missing safe cache headers: %v", response.Header())
	}
	if releases != 1 {
		t.Fatalf("successful response released cache lease %d times, want 1", releases)
	}

	conditional := httptest.NewRequest(http.MethodGet, "/api/library/video-thumbnail/item-1", nil)
	conditional.Header.Set("If-None-Match", `"thumbnail-etag"`)
	conditionalResponse := httptest.NewRecorder()
	handler.ServeHTTP(conditionalResponse, conditional)
	if conditionalResponse.Code != http.StatusNotModified || conditionalResponse.Body.Len() != 0 {
		t.Fatalf("conditional response = %d %q", conditionalResponse.Code, conditionalResponse.Body.String())
	}
	if releases != 2 {
		t.Fatalf("conditional response released cache lease %d times, want 2", releases)
	}
}

func TestCatalogVideoThumbnailHandlerRejectsPathShapedIDs(t *testing.T) {
	t.Parallel()
	resolver := &catalogVideoThumbnailResolverStub{}
	handler := NewCatalogVideoThumbnailHandler(resolver)
	for _, target := range []string{
		"/api/library/video-thumbnail/",
		"/api/library/video-thumbnail/folder/item-1",
		"/api/library/video-thumbnail/" + strings.Repeat("x", 256),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, response.Code)
		}
	}
	if resolver.itemID != "" {
		t.Fatalf("invalid ID reached resolver: %q", resolver.itemID)
	}
}

func writeCatalogVideoThumbnailHandlerJPEG(path string) error {
	preview := image.NewRGBA(image.Rect(0, 0, 2, 2))
	preview.Set(0, 0, color.RGBA{R: 128, G: 64, B: 32, A: 255})
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	err = jpeg.Encode(file, preview, nil)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
