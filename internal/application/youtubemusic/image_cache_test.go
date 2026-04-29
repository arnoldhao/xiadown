package youtubemusic

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

var tinyAVIFHeader = []byte{
	0x00, 0x00, 0x00, 0x20,
	'f', 't', 'y', 'p',
	'a', 'v', 'i', 'f',
	0x00, 0x00, 0x00, 0x00,
	'a', 'v', 'i', 'f',
}

var tinySVG = []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"></svg>`)

var tinyWEBPHeader = []byte{
	'R', 'I', 'F', 'F',
	0x08, 0x00, 0x00, 0x00,
	'W', 'E', 'B', 'P',
	'V', 'P', '8', ' ',
}

type imageCacheClientProvider struct {
	client *http.Client
}

func (provider imageCacheClientProvider) HTTPClient() *http.Client {
	return provider.client
}

type imageRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn imageRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func imageCacheTestClient(fn imageRoundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func imageCacheTestResponse(status int, contentType string, body []byte) *http.Response {
	header := make(http.Header)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func TestImageCacheFetchesAndStoresOnDisk(t *testing.T) {
	var requestCount atomic.Int32
	imageURL := "https://image.example/cover.png"
	client := imageCacheTestClient(func(request *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		if request.URL.String() != imageURL {
			t.Fatalf("unexpected request url: %s", request.URL.String())
		}
		return imageCacheTestResponse(http.StatusOK, "image/png", tinyPNG), nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	result, err := cache.Image(context.Background(), imageURL)
	if err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if !bytes.Equal(result.Data, tinyPNG) || result.ContentType != "image/png" {
		t.Fatalf("unexpected image result: contentType=%q size=%d", result.ContentType, len(result.Data))
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected one network request, got %d", requestCount.Load())
	}
	if cache.DiskCacheSize() == 0 {
		t.Fatal("expected image bytes on disk")
	}
	files, err := os.ReadDir(cache.directory)
	if err != nil {
		t.Fatalf("read cache directory: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected cache to store a single image body file, got %d files", len(files))
	}
	diskData, err := os.ReadFile(filepath.Join(cache.directory, files[0].Name()))
	if err != nil {
		t.Fatalf("read cached image body: %v", err)
	}
	if !bytes.Equal(diskData, tinyPNG) {
		t.Fatal("expected disk cache to store raw image bytes")
	}

	cache.ClearMemory()
	result, err = cache.Image(context.Background(), imageURL)
	if err != nil {
		t.Fatalf("Image() from disk error = %v", err)
	}
	if !bytes.Equal(result.Data, tinyPNG) {
		t.Fatal("unexpected disk cache data")
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected disk cache hit without network, got %d requests", requestCount.Load())
	}
}

func TestImageCacheDetectsStoredAVIFContentTypeFromImageBytes(t *testing.T) {
	var requestCount atomic.Int32
	imageURL := "https://lh3.googleusercontent.com/cover-avif"
	client := imageCacheTestClient(func(_ *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		return imageCacheTestResponse(http.StatusOK, "image/avif", tinyAVIFHeader), nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	result, err := cache.Image(context.Background(), imageURL)
	if err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if result.ContentType != "image/avif" {
		t.Fatalf("unexpected content type: %q", result.ContentType)
	}
	cache.ClearMemory()

	result, err = cache.Image(context.Background(), imageURL)
	if err != nil {
		t.Fatalf("Image() from disk error = %v", err)
	}
	if result.ContentType != "image/avif" {
		t.Fatalf("unexpected disk content type: %q", result.ContentType)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected disk cache hit without network, got %d requests", requestCount.Load())
	}
}

func TestImageCacheDetectsAVIFDiskEntryFromImageBytes(t *testing.T) {
	imageURL := "https://lh3.googleusercontent.com/cached-avif"
	dir := t.TempDir()
	key := imageCacheKey(imageURL)
	if err := os.WriteFile(filepath.Join(dir, key), tinyAVIFHeader, 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}
	client := imageCacheTestClient(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("unexpected network request for AVIF disk cache hit")
		return nil, nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: dir,
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	result, err := cache.Image(context.Background(), imageURL)
	if err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if result.ContentType != "image/avif" {
		t.Fatalf("unexpected content type: %q", result.ContentType)
	}
}

func TestImageCacheDetectsSVGDiskEntryFromImageBytes(t *testing.T) {
	imageURL := "https://image.example/cached.svg"
	dir := t.TempDir()
	key := imageCacheKey(imageURL)
	if err := os.WriteFile(filepath.Join(dir, key), tinySVG, 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}
	client := imageCacheTestClient(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("unexpected network request for SVG disk cache hit")
		return nil, nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: dir,
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	result, err := cache.Image(context.Background(), imageURL)
	if err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if result.ContentType != "image/svg+xml" {
		t.Fatalf("unexpected content type: %q", result.ContentType)
	}
}

func TestImageCacheDetectsWEBPDiskEntryFromImageBytes(t *testing.T) {
	imageURL := "https://image.example/cached.webp"
	dir := t.TempDir()
	key := imageCacheKey(imageURL)
	if err := os.WriteFile(filepath.Join(dir, key), tinyWEBPHeader, 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}
	client := imageCacheTestClient(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("unexpected network request for WebP disk cache hit")
		return nil, nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: dir,
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	result, err := cache.Image(context.Background(), imageURL)
	if err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if result.ContentType != "image/webp" {
		t.Fatalf("unexpected content type: %q", result.ContentType)
	}
}

func TestImageCacheRejectsHTMLServedAsImage(t *testing.T) {
	var requestCount atomic.Int32
	client := imageCacheTestClient(func(_ *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		return imageCacheTestResponse(http.StatusOK, "image/png", []byte("<!doctype html><html>blocked</html>")), nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	_, err = cache.Image(context.Background(), "https://lh3.googleusercontent.com/blocked.png")
	if !errors.Is(err, ErrImageUnavailable) {
		t.Fatalf("expected ErrImageUnavailable, got %v", err)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected one network request, got %d", requestCount.Load())
	}
	if cache.DiskCacheSize() != 0 {
		t.Fatalf("expected rejected HTML image response to stay out of disk cache, got %d bytes", cache.DiskCacheSize())
	}
}

func TestImageCacheDedupesConcurrentFetches(t *testing.T) {
	var requestCount atomic.Int32
	imageURL := "https://image.example/cover.png"
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	client := imageCacheTestClient(func(_ *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		startedOnce.Do(func() { close(started) })
		<-release
		return imageCacheTestResponse(http.StatusOK, "image/png", tinyPNG), nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := cache.Image(context.Background(), imageURL)
		errs <- err
	}()
	<-started
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := cache.Image(context.Background(), imageURL)
		errs <- err
	}()
	time.Sleep(25 * time.Millisecond)
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Image() error = %v", err)
		}
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected one network request, got %d", requestCount.Load())
	}
}

func TestImageCacheReportsFetchFailure(t *testing.T) {
	client := imageCacheTestClient(func(_ *http.Request) (*http.Response, error) {
		return imageCacheTestResponse(http.StatusNotFound, "text/plain", []byte("missing")), nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	_, err = cache.Image(context.Background(), "https://image.example/missing.png")
	if !errors.Is(err, ErrImageUnavailable) {
		t.Fatalf("expected ErrImageUnavailable, got %v", err)
	}
}

func TestImageCacheRejectsNonHTTPURL(t *testing.T) {
	cache, err := NewImageCache(nil, ImageCacheConfig{Directory: t.TempDir()})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	_, err = cache.Image(context.Background(), "file:///tmp/cover.png")
	if !errors.Is(err, ErrImageURLInvalid) {
		t.Fatalf("expected ErrImageURLInvalid, got %v", err)
	}
}

func TestImageCacheSendsYouTubeMusicImageHeadersForGoogleImages(t *testing.T) {
	var referer string
	var originHeader string
	client := imageCacheTestClient(func(request *http.Request) (*http.Response, error) {
		referer = request.Header.Get("Referer")
		originHeader = request.Header.Get("Origin")
		return imageCacheTestResponse(http.StatusOK, "image/png", tinyPNG), nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	if _, err := cache.Image(context.Background(), "https://lh3.googleusercontent.com/cover.png"); err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if referer != origin+"/" || originHeader != origin {
		t.Fatalf("unexpected image headers: Referer=%q Origin=%q", referer, originHeader)
	}
}

func TestImageCacheSendsYouTubeImageHeadersForYtimgImages(t *testing.T) {
	var referer string
	var originHeader string
	client := imageCacheTestClient(func(request *http.Request) (*http.Response, error) {
		referer = request.Header.Get("Referer")
		originHeader = request.Header.Get("Origin")
		return imageCacheTestResponse(http.StatusOK, "image/png", tinyPNG), nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	if _, err := cache.Image(context.Background(), "https://i.ytimg.com/vi/TESTVID006F/hqdefault.jpg"); err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if referer != youtubeWebImageOrigin+"/" || originHeader != youtubeWebImageOrigin {
		t.Fatalf("unexpected image headers: Referer=%q Origin=%q", referer, originHeader)
	}
}

func TestImageCacheDoesNotSendYouTubeMusicImageHeadersForThirdPartyImages(t *testing.T) {
	var referer string
	var originHeader string
	client := imageCacheTestClient(func(request *http.Request) (*http.Response, error) {
		referer = request.Header.Get("Referer")
		originHeader = request.Header.Get("Origin")
		return imageCacheTestResponse(http.StatusOK, "image/png", tinyPNG), nil
	})

	cache, err := NewImageCache(imageCacheClientProvider{client: client}, ImageCacheConfig{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewImageCache() error = %v", err)
	}

	if _, err := cache.Image(context.Background(), "https://image.example/cover.png"); err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if referer != "" || originHeader != "" {
		t.Fatalf("unexpected third-party image headers: Referer=%q Origin=%q", referer, originHeader)
	}
}
