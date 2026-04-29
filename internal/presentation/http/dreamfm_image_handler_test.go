package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/youtubemusic"
)

type fakeDreamFMImageCache struct {
	result youtubemusic.ImageResult
	err    error
	rawURL string
}

func (cache *fakeDreamFMImageCache) Image(_ context.Context, rawURL string) (youtubemusic.ImageResult, error) {
	cache.rawURL = rawURL
	return cache.result, cache.err
}

func TestDreamFMImageHandlerServesCachedImage(t *testing.T) {
	cache := &fakeDreamFMImageCache{
		result: youtubemusic.ImageResult{
			URL:         "https://lh3.googleusercontent.com/cover",
			CacheKey:    "cache-key",
			Data:        []byte{0x89, 0x50, 0x4e, 0x47},
			ContentType: "image/png",
			ModTime:     time.Unix(1_700_000_000, 0),
		},
	}
	handler := NewDreamFMImageHandler(cache)
	request := httptest.NewRequest(http.MethodGet, "/api/dreamfm/image?url=https%3A%2F%2Flh3.googleusercontent.com%2Fcover", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	if cache.rawURL != "https://lh3.googleusercontent.com/cover" {
		t.Fatalf("unexpected raw url: %q", cache.rawURL)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "image/png") {
		t.Fatalf("unexpected content type: %q", got)
	}
	if got := response.Header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("unexpected cache control: %q", got)
	}
}

func TestDreamFMImageHandlerMapsInvalidURLToBadRequest(t *testing.T) {
	handler := NewDreamFMImageHandler(&fakeDreamFMImageCache{err: youtubemusic.ErrImageURLInvalid})
	request := httptest.NewRequest(http.MethodGet, "/api/dreamfm/image?url=file%3A%2F%2Fcover", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestDreamFMImageHandlerMapsFetchFailureToBadGateway(t *testing.T) {
	handler := NewDreamFMImageHandler(&fakeDreamFMImageCache{err: errors.Join(youtubemusic.ErrImageUnavailable, errors.New("status 404"))})
	request := httptest.NewRequest(http.MethodGet, "/api/dreamfm/image?url=https%3A%2F%2Fexample.com%2Fmissing.png", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}
