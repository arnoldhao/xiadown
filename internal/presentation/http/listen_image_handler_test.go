package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/imagecache"
)

type fakeListenImageCache struct {
	result                imagecache.ImageResult
	err                   error
	rawURL                string
	variantURL            string
	variantSize           int
	prefetchURLs          []string
	prefetchSize          int
	prefetchMaxConcurrent int
	prefetchDone          chan struct{}
}

func (cache *fakeListenImageCache) Image(_ context.Context, rawURL string) (imagecache.ImageResult, error) {
	cache.rawURL = rawURL
	return cache.result, cache.err
}

func (cache *fakeListenImageCache) ImageVariant(_ context.Context, rawURL string, size int) (imagecache.ImageResult, error) {
	cache.variantURL = rawURL
	cache.variantSize = size
	return cache.result, cache.err
}

func (cache *fakeListenImageCache) Prefetch(_ context.Context, urls []string, size int, maxConcurrent int) {
	cache.prefetchURLs = append([]string(nil), urls...)
	cache.prefetchSize = size
	cache.prefetchMaxConcurrent = maxConcurrent
	if cache.prefetchDone != nil {
		close(cache.prefetchDone)
	}
}

func TestListenImageHandlerServesCachedImage(t *testing.T) {
	cache := &fakeListenImageCache{
		result: imagecache.ImageResult{
			URL:         "https://lh3.googleusercontent.com/cover",
			CacheKey:    "cache-key",
			Data:        []byte{0x89, 0x50, 0x4e, 0x47},
			ContentType: "image/png",
			ModTime:     time.Unix(1_700_000_000, 0),
		},
	}
	handler := NewListenImageHandler(cache)
	request := httptest.NewRequest(http.MethodGet, "/api/listen/image?url=https%3A%2F%2Flh3.googleusercontent.com%2Fcover", nil)
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

func TestListenImageHandlerRequestsVariantWhenSizeProvided(t *testing.T) {
	cache := &fakeListenImageCache{
		result: imagecache.ImageResult{
			URL:         "https://lh3.googleusercontent.com/cover",
			CacheKey:    "variant-cache-key",
			Data:        []byte{0x89, 0x50, 0x4e, 0x47},
			ContentType: "image/png",
			ModTime:     time.Unix(1_700_000_000, 0),
		},
	}
	handler := NewListenImageHandler(cache)
	request := httptest.NewRequest(http.MethodGet, "/api/listen/image?url=https%3A%2F%2Flh3.googleusercontent.com%2Fcover&size=320", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Result().StatusCode)
	}
	if cache.rawURL != "" {
		t.Fatalf("expected variant path, got plain image url %q", cache.rawURL)
	}
	if cache.variantURL != "https://lh3.googleusercontent.com/cover" || cache.variantSize != 320 {
		t.Fatalf("unexpected variant request: url=%q size=%d", cache.variantURL, cache.variantSize)
	}
}

func TestListenImageHandlerMapsInvalidURLToBadRequest(t *testing.T) {
	handler := NewListenImageHandler(&fakeListenImageCache{err: imagecache.ErrImageURLInvalid})
	request := httptest.NewRequest(http.MethodGet, "/api/listen/image?url=file%3A%2F%2Fcover", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestListenImageHandlerMapsFetchFailureToBadGateway(t *testing.T) {
	handler := NewListenImageHandler(&fakeListenImageCache{err: errors.Join(imagecache.ErrImageUnavailable, errors.New("status 404"))})
	request := httptest.NewRequest(http.MethodGet, "/api/listen/image?url=https%3A%2F%2Fexample.com%2Fmissing.png", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestListenImageHandlerPrefetchesThroughGoCache(t *testing.T) {
	done := make(chan struct{})
	cache := &fakeListenImageCache{prefetchDone: done}
	handler := NewListenImageHandler(cache)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/listen/image/prefetch",
		strings.NewReader(`{"urls":["http://localhost/api/listen/image?url=https%3A%2F%2Flh3.googleusercontent.com%2Fcover","https://i.ytimg.com/vi/video/hqdefault.jpg"],"size":320,"maxConcurrent":2}`),
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected prefetch to reach image cache")
	}
	if cache.prefetchSize != 320 || cache.prefetchMaxConcurrent != 2 {
		t.Fatalf("unexpected prefetch options: size=%d max=%d", cache.prefetchSize, cache.prefetchMaxConcurrent)
	}
	want := []string{
		"https://lh3.googleusercontent.com/cover",
		"https://i.ytimg.com/vi/video/hqdefault.jpg",
	}
	if strings.Join(cache.prefetchURLs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected prefetch urls: %#v", cache.prefetchURLs)
	}
}
