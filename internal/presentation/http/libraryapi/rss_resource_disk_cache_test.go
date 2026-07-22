package libraryapi

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	applicationrss "xiadown/internal/application/rss"
)

type rssDiskTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *rssDiskTestClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *rssDiskTestClock) Add(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	clock.mu.Unlock()
}

func TestRSSImageDiskCacheSurvivesCacheRecreationWithoutLeakingSourceMetadata(t *testing.T) {
	directory := t.TempDir()
	clock := &rssDiskTestClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)}
	resource := applicationrss.RemoteResource{
		URL: "https://signed.example/cover.png?token=do-not-persist", Kind: applicationrss.RemoteResourceImage,
		Role: applicationrss.RemoteResourceRoleThumbnail, RefererOrigin: "https://publisher.example/private?token=secret",
	}
	payload := rssCacheTestPNG(t, color.RGBA{R: 80, G: 20, B: 140, A: 255})
	first := newRSSImageCacheWithConfig(rssImageCacheConfig{now: clock.Now})
	first.setDisk(newRSSImageDiskCacheForTest(t, directory, clock.Now, defaultRSSImageDiskCacheBytes))
	var loaders atomic.Int32
	loaded, err := first.get(context.Background(), resource, func(context.Context) (rssCachedImage, error) {
		loaders.Add(1)
		return rssCachedImage{data: payload, contentType: "image/png", etag: rssImageContentETag(payload)}, nil
	})
	if err != nil || !bytes.Equal(loaded.data, payload) {
		t.Fatalf("initial cache fill = %d bytes, %v", len(loaded.data), err)
	}

	key, ok := rssImageResourceCacheKey(resource)
	if !ok {
		t.Fatal("resource cache key unavailable")
	}
	record, err := os.ReadFile(first.disk.path(key))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"signed.example", "do-not-persist", "publisher.example", "secret"} {
		if strings.Contains(string(record), secret) {
			t.Fatalf("disk record leaked source metadata %q", secret)
		}
	}
	if runtime.GOOS != "windows" {
		directoryInfo, statErr := os.Stat(directory)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if directoryInfo.Mode().Perm() != 0o700 {
			t.Fatalf("disk cache directory permissions = %v", directoryInfo.Mode().Perm())
		}
		recordInfo, statErr := os.Stat(first.disk.path(key))
		if statErr != nil {
			t.Fatal(statErr)
		}
		if recordInfo.Mode().Perm() != 0o600 {
			t.Fatalf("disk cache record permissions = %v", recordInfo.Mode().Perm())
		}
	}

	second := newRSSImageCacheWithConfig(rssImageCacheConfig{now: clock.Now})
	second.setDisk(newRSSImageDiskCacheForTest(t, directory, clock.Now, defaultRSSImageDiskCacheBytes))
	restored, err := second.get(context.Background(), resource, func(context.Context) (rssCachedImage, error) {
		loaders.Add(1)
		return rssCachedImage{}, errors.New("loader must not run for a fresh disk hit")
	})
	if err != nil || !bytes.Equal(restored.data, payload) || loaders.Load() != 1 {
		t.Fatalf("restored cache = %d bytes, loaders=%d, err=%v", len(restored.data), loaders.Load(), err)
	}
}

func TestRSSImageDiskCacheCorruptionFailsClosedAndDeletesRecord(t *testing.T) {
	directory := t.TempDir()
	clock := &rssDiskTestClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)}
	resource := applicationrss.RemoteResource{
		URL: "https://cdn.example/corrupt.png", Kind: applicationrss.RemoteResourceImage,
		Role: applicationrss.RemoteResourceRoleContentImage,
	}
	payload := rssCacheTestPNG(t, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	first := newRSSImageCacheWithConfig(rssImageCacheConfig{now: clock.Now})
	firstDisk := newRSSImageDiskCacheForTest(t, directory, clock.Now, defaultRSSImageDiskCacheBytes)
	first.setDisk(firstDisk)
	if _, err := first.get(context.Background(), resource, func(context.Context) (rssCachedImage, error) {
		return rssCachedImage{data: payload, contentType: "image/png", etag: rssImageContentETag(payload)}, nil
	}); err != nil {
		t.Fatal(err)
	}
	key, _ := rssImageResourceCacheKey(resource)
	path := firstDisk.path(key)
	record, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	record[len(record)-1] ^= 0xff // metadata ETag and payload no longer agree.
	if err := os.WriteFile(path, record, 0o600); err != nil {
		t.Fatal(err)
	}

	second := newRSSImageCacheWithConfig(rssImageCacheConfig{now: clock.Now})
	second.setDisk(newRSSImageDiskCacheForTest(t, directory, clock.Now, defaultRSSImageDiskCacheBytes))
	_, err = second.get(context.Background(), resource, func(context.Context) (rssCachedImage, error) {
		return rssCachedImage{}, errors.New("upstream unavailable")
	})
	if err == nil {
		t.Fatal("corrupt disk bytes were served")
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("corrupt record was not deleted: %v", statErr)
	}
}

func TestRSSImageDiskCacheUsesPersistedStaleBytesOnlyAfterRefreshFailure(t *testing.T) {
	directory := t.TempDir()
	clock := &rssDiskTestClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)}
	resource := applicationrss.RemoteResource{
		URL: "https://cdn.example/stale-across-restart.png", Kind: applicationrss.RemoteResourceImage,
		Role: applicationrss.RemoteResourceRoleContentImage,
	}
	payload := rssCacheTestPNG(t, color.RGBA{R: 101, G: 102, B: 103, A: 255})
	config := rssImageCacheConfig{
		imageTTL: time.Minute, staleRetention: time.Hour, negativeTTL: 5 * time.Minute, now: clock.Now,
	}
	first := newRSSImageCacheWithConfig(config)
	first.setDisk(newRSSImageDiskCacheForTest(t, directory, clock.Now, defaultRSSImageDiskCacheBytes))
	if _, err := first.get(context.Background(), resource, func(context.Context) (rssCachedImage, error) {
		return rssCachedImage{data: payload, contentType: "image/png", etag: rssImageContentETag(payload)}, nil
	}); err != nil {
		t.Fatal(err)
	}
	clock.Add(2 * time.Minute)

	second := newRSSImageCacheWithConfig(config)
	second.setDisk(newRSSImageDiskCacheForTest(t, directory, clock.Now, defaultRSSImageDiskCacheBytes))
	var refreshes atomic.Int32
	stale, err := second.get(context.Background(), resource, func(context.Context) (rssCachedImage, error) {
		refreshes.Add(1)
		return rssCachedImage{}, errors.New("temporary publisher failure")
	})
	if err != nil || !bytes.Equal(stale.data, payload) || refreshes.Load() != 1 {
		t.Fatalf("persisted stale fallback = %d bytes, refreshes=%d, err=%v", len(stale.data), refreshes.Load(), err)
	}
}

func TestRSSImageDiskCacheDoesNotPersistNegativeEntries(t *testing.T) {
	directory := t.TempDir()
	clock := &rssDiskTestClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)}
	resource := applicationrss.RemoteResource{
		URL: "https://cdn.example/missing.png", Kind: applicationrss.RemoteResourceImage,
		Role: applicationrss.RemoteResourceRoleContentImage,
	}
	cache := newRSSImageCacheWithConfig(rssImageCacheConfig{now: clock.Now})
	cache.setDisk(newRSSImageDiskCacheForTest(t, directory, clock.Now, defaultRSSImageDiskCacheBytes))
	if _, err := cache.get(context.Background(), resource, func(context.Context) (rssCachedImage, error) {
		return rssCachedImage{}, errors.New("missing")
	}); err == nil {
		t.Fatal("missing image unexpectedly succeeded")
	}
	children, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 0 {
		t.Fatalf("negative cache wrote %d disk records", len(children))
	}
}

func TestRSSImageDiskCacheConfigurationFailureLeavesMemoryCacheAvailable(t *testing.T) {
	api, err := NewRSSAPI(&rssServiceStub{})
	if err != nil {
		t.Fatal(err)
	}
	blockedPath := t.TempDir() + "/not-a-directory"
	if err := os.WriteFile(blockedPath, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := api.ConfigureImageDiskCache(blockedPath); err == nil {
		t.Fatal("disk cache accepted a regular file as its directory")
	}
	resource := applicationrss.RemoteResource{
		URL: "https://cdn.example/memory-only.png", Kind: applicationrss.RemoteResourceImage,
		Role: applicationrss.RemoteResourceRoleContentImage,
	}
	payload := rssCacheTestPNG(t, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	loaded, err := api.imageCache.get(context.Background(), resource, func(context.Context) (rssCachedImage, error) {
		return rssCachedImage{data: payload, contentType: "image/png", etag: rssImageContentETag(payload)}, nil
	})
	if err != nil || !bytes.Equal(loaded.data, payload) {
		t.Fatalf("memory fallback = %d bytes, %v", len(loaded.data), err)
	}
}

func TestRSSImageDiskCacheCleanupEvictsOldestAndExpiredRecords(t *testing.T) {
	directory := t.TempDir()
	clock := &rssDiskTestClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)}
	disk := newRSSImageDiskCacheForTest(t, directory, clock.Now, defaultRSSImageDiskCacheBytes)
	payload := rssCacheTestPNG(t, color.RGBA{R: 4, G: 8, B: 16, A: 255})
	entry := func(resource applicationrss.RemoteResource) rssImageCacheEntry {
		key, ok := rssImageResourceCacheKey(resource)
		if !ok {
			t.Fatal("resource cache key unavailable")
		}
		now := clock.Now()
		return rssImageCacheEntry{
			key: key, role: resource.Role,
			image:      rssCachedImage{data: payload, contentType: "image/png", etag: rssImageContentETag(payload)},
			freshUntil: now.Add(time.Minute), staleUntil: now.Add(2 * time.Minute), cost: int64(len(payload)),
		}
	}
	first := entry(applicationrss.RemoteResource{
		URL: "https://cdn.example/first.png", Kind: applicationrss.RemoteResourceImage,
		Role: applicationrss.RemoteResourceRoleThumbnail,
	})
	if err := disk.write(first); err != nil {
		t.Fatal(err)
	}
	clock.Add(time.Second)
	second := entry(applicationrss.RemoteResource{
		URL: "https://cdn.example/second.png", Kind: applicationrss.RemoteResourceImage,
		Role: applicationrss.RemoteResourceRoleThumbnail,
	})
	if err := disk.write(second); err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(disk.path(second.key))
	if err != nil {
		t.Fatal(err)
	}
	disk.maxBytes = secondInfo.Size()
	if err := disk.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(disk.path(first.key)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oldest record was not evicted: %v", err)
	}
	if _, err := os.Stat(disk.path(second.key)); err != nil {
		t.Fatalf("newest record was evicted: %v", err)
	}

	clock.Add(3 * time.Minute)
	if err := disk.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(disk.path(second.key)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired record was not removed: %v", err)
	}
}

func newRSSImageDiskCacheForTest(
	t *testing.T,
	directory string,
	now func() time.Time,
	maxBytes int64,
) *rssImageDiskCache {
	t.Helper()
	disk, err := newRSSImageDiskCache(rssImageDiskCacheConfig{
		directory: directory, maxBytes: maxBytes, now: now, asyncCleanup: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return disk
}
