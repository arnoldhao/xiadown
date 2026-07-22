package service

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"xiadown/internal/domain/dependencies"
	"xiadown/internal/domain/library"
)

type catalogVideoThumbnailItemRepository struct {
	items map[string]library.Item
}

func (repo catalogVideoThumbnailItemRepository) ListByCatalogID(context.Context, string) ([]library.Item, error) {
	result := make([]library.Item, 0, len(repo.items))
	for _, item := range repo.items {
		result = append(result, item)
	}
	return result, nil
}

func (repo catalogVideoThumbnailItemRepository) Get(_ context.Context, id string) (library.Item, error) {
	item, ok := repo.items[id]
	if !ok {
		return library.Item{}, library.ErrFileNotFound
	}
	return item, nil
}

func (catalogVideoThumbnailItemRepository) Save(context.Context, library.Item) error { return nil }
func (catalogVideoThumbnailItemRepository) Delete(context.Context, string) error     { return nil }

type catalogVideoThumbnailTools struct{ directory string }

func (tools catalogVideoThumbnailTools) ResolveExecPath(context.Context, dependencies.DependencyName) (string, error) {
	return filepath.Join(tools.directory, ffmpegExecutableName()), nil
}

func (tools catalogVideoThumbnailTools) ResolveDependencyDirectory(context.Context, dependencies.DependencyName) (string, error) {
	return tools.directory, nil
}

func (catalogVideoThumbnailTools) DependencyReadiness(context.Context, dependencies.DependencyName) (bool, string, error) {
	return true, "", nil
}

func TestCatalogVideoThumbnailGeneratesOnceCachesAndBoundsFFmpeg(t *testing.T) {
	t.Parallel()
	service, sourcePath := newCatalogVideoThumbnailTestService(t, "video")
	var runs atomic.Int32
	var argsMu sync.Mutex
	var capturedArgs []string
	service.runCommand = func(ctx context.Context, _ string, args []string) error {
		runs.Add(1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		argsMu.Lock()
		capturedArgs = append([]string(nil), args...)
		argsMu.Unlock()
		return writeCatalogVideoThumbnailJPEG(args[len(args)-1])
	}

	const callers = 8
	results := make(chan CatalogVideoThumbnail, callers)
	errors := make(chan error, callers)
	var wait sync.WaitGroup
	for range callers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := service.Resolve(context.Background(), "item-video")
			results <- result
			errors <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("resolve thumbnail: %v", err)
		}
	}
	firstPath := ""
	for result := range results {
		if firstPath == "" {
			firstPath = result.Path
		}
		if result.Path != firstPath || result.ETag == "" || result.ModTime.IsZero() {
			t.Fatalf("inconsistent thumbnail result: %#v, first=%q", result, firstPath)
		}
		if result.Release == nil {
			t.Fatal("successful thumbnail did not return a cache lease")
		}
		result.Release()
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("same-source concurrent generation count = %d, want 1", got)
	}

	argsMu.Lock()
	args := append([]string(nil), capturedArgs...)
	argsMu.Unlock()
	assertEveryLocalFFmpegInputHasPolicy(t, args)
	for _, pair := range [][2]string{
		{"-frames:v", "1"},
		{"-filter_threads", "1"},
		{"-threads", "1"},
		{"-ss", "10.000"},
	} {
		if !containsAdjacentArguments(args, pair[0], pair[1]) {
			t.Fatalf("thumbnail args missing %q %q: %v", pair[0], pair[1], args)
		}
	}
	if !slices.Contains(args, "-noaccurate_seek") || !slices.Contains(args, "-an") ||
		!slices.Contains(args, "-sn") || !slices.Contains(args, "-dn") {
		t.Fatalf("thumbnail args do not bound non-video work: %v", args)
	}
	inputIndex := slices.Index(args, "-i")
	if inputIndex < 0 || inputIndex+1 >= len(args) || args[inputIndex+1] != sourcePath {
		t.Fatalf("registered source was not the FFmpeg input: %v", args)
	}

	cached, err := service.Resolve(context.Background(), "item-video")
	if err != nil {
		t.Fatalf("resolve cached thumbnail: %v", err)
	}
	cached.Release()
	if got := runs.Load(); got != 1 {
		t.Fatalf("cache hit invoked FFmpeg; generation count = %d", got)
	}

	if err := os.WriteFile(sourcePath, []byte("updated source identity"), 0o600); err != nil {
		t.Fatalf("update source: %v", err)
	}
	updated, err := service.Resolve(context.Background(), "item-video")
	if err != nil {
		t.Fatalf("resolve changed source thumbnail: %v", err)
	}
	if updated.Path == firstPath || runs.Load() != 2 {
		t.Fatalf("source fingerprint did not invalidate cache: first=%q updated=%q runs=%d", firstPath, updated.Path, runs.Load())
	}
	updated.Release()
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("superseded item thumbnail was not pruned: %v", err)
	}
}

func TestCatalogVideoThumbnailRejectsNonVideoAndUnregisteredPaths(t *testing.T) {
	t.Parallel()
	service, _ := newCatalogVideoThumbnailTestService(t, "audio")
	service.runCommand = func(context.Context, string, []string) error {
		t.Fatal("FFmpeg must not run for non-video Catalog items")
		return nil
	}
	if _, err := service.Resolve(context.Background(), "item-video"); !errors.Is(err, ErrCatalogVideoThumbnailNotFound) {
		t.Fatalf("non-video resolve error = %v", err)
	}
	if _, err := service.Resolve(context.Background(), "https://example.test/private.mp4"); !errors.Is(err, ErrCatalogVideoThumbnailNotFound) {
		t.Fatalf("caller-controlled path resolve error = %v", err)
	}
	if _, err := service.Resolve(context.Background(), "../item-video"); !errors.Is(err, ErrCatalogVideoThumbnailNotFound) {
		t.Fatalf("path-shaped item ID resolve error = %v", err)
	}
	if _, err := service.Resolve(context.Background(), strings.Repeat("x", 256)); !errors.Is(err, ErrCatalogVideoThumbnailNotFound) {
		t.Fatalf("oversized item ID resolve error = %v", err)
	}
	if _, err := service.Resolve(context.Background(), "item-video\x00"); !errors.Is(err, ErrCatalogVideoThumbnailNotFound) {
		t.Fatalf("control-character item ID resolve error = %v", err)
	}
}

func TestCatalogVideoThumbnailCallerCancellationDoesNotCancelSharedFlight(t *testing.T) {
	t.Parallel()
	service, _ := newCatalogVideoThumbnailTestService(t, "video")
	started := make(chan struct{})
	release := make(chan struct{})
	var runs atomic.Int32
	service.runCommand = func(ctx context.Context, _ string, args []string) error {
		runs.Add(1)
		close(started)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return writeCatalogVideoThumbnailJPEG(args[len(args)-1])
		}
	}

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderResult := make(chan error, 1)
	go func() {
		_, err := service.Resolve(leaderCtx, "item-video")
		leaderResult <- err
	}()
	<-started
	cancelLeader()
	if err := <-leaderResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader cancellation error = %v", err)
	}

	followerResult := make(chan error, 1)
	go func() {
		result, err := service.Resolve(context.Background(), "item-video")
		if err == nil {
			result.Release()
		}
		followerResult <- err
	}()
	waitForCatalogVideoThumbnailFlightWaiters(t, service, "item-video", 1)
	close(release)
	if err := <-followerResult; err != nil {
		t.Fatalf("surviving follower failed with leader cancellation: %v", err)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("shared generation count = %d, want 1", got)
	}
}

func TestCatalogVideoThumbnailNegativeCacheAndCancellationPolicy(t *testing.T) {
	t.Parallel()
	service, _ := newCatalogVideoThumbnailTestService(t, "video")
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.negativeTTL = time.Minute
	var runs atomic.Int32
	service.runCommand = func(context.Context, string, []string) error {
		runs.Add(1)
		return errors.New("damaged video")
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := service.Resolve(context.Background(), "item-video"); !errors.Is(err, ErrCatalogVideoThumbnailUnavailable) {
			t.Fatalf("failed thumbnail attempt %d error = %v", attempt, err)
		}
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("negative cache generation count = %d, want 1", got)
	}
	now = now.Add(time.Minute + time.Second)
	if _, err := service.Resolve(context.Background(), "item-video"); !errors.Is(err, ErrCatalogVideoThumbnailUnavailable) {
		t.Fatalf("post-expiry thumbnail error = %v", err)
	}
	if got := runs.Load(); got != 2 {
		t.Fatalf("expired negative cache generation count = %d, want 2", got)
	}

	canceled, _ := newCatalogVideoThumbnailTestService(t, "video")
	var canceledRuns atomic.Int32
	canceled.runCommand = func(context.Context, string, []string) error {
		canceledRuns.Add(1)
		return context.Canceled
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := canceled.Resolve(context.Background(), "item-video"); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled generation attempt %d error = %v", attempt, err)
		}
	}
	if got := canceledRuns.Load(); got != 2 {
		t.Fatalf("context cancellation was negatively cached; runs = %d", got)
	}
}

func TestCatalogVideoThumbnailNegativeCachePrunesExpiredAndBoundsEntries(t *testing.T) {
	t.Parallel()
	service, _ := newCatalogVideoThumbnailTestService(t, "video")
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.negativeTTL = 3 * time.Minute
	service.maxNegativeEntries = 2
	service.mu.Lock()
	service.negative["expired"] = catalogVideoThumbnailNegative{expires: now.Add(-time.Second), err: ErrCatalogVideoThumbnailUnavailable}
	service.negative["oldest"] = catalogVideoThumbnailNegative{expires: now.Add(time.Minute), err: ErrCatalogVideoThumbnailUnavailable}
	service.negative["newest"] = catalogVideoThumbnailNegative{expires: now.Add(2 * time.Minute), err: ErrCatalogVideoThumbnailUnavailable}
	service.mu.Unlock()

	generated := false
	if _, err := service.resolveFlight(context.Background(), "newest", "unused-newest", func(context.Context) (CatalogVideoThumbnail, error) {
		generated = true
		return CatalogVideoThumbnail{}, nil
	}); !errors.Is(err, ErrCatalogVideoThumbnailUnavailable) {
		t.Fatalf("negative lookup error = %v", err)
	}
	if generated {
		t.Fatal("live negative cache entry unexpectedly generated")
	}
	service.mu.Lock()
	_, expiredExists := service.negative["expired"]
	negativeCount := len(service.negative)
	service.mu.Unlock()
	if expiredExists || negativeCount != 2 {
		t.Fatalf("expired negative cache was not pruned: exists=%t count=%d", expiredExists, negativeCount)
	}

	if _, err := service.resolveFlight(context.Background(), "replacement", "unused-replacement", func(context.Context) (CatalogVideoThumbnail, error) {
		return CatalogVideoThumbnail{}, ErrCatalogVideoThumbnailUnavailable
	}); !errors.Is(err, ErrCatalogVideoThumbnailUnavailable) {
		t.Fatalf("replacement negative generation error = %v", err)
	}
	service.mu.Lock()
	_, oldestExists := service.negative["oldest"]
	_, replacementExists := service.negative["replacement"]
	negativeCount = len(service.negative)
	service.mu.Unlock()
	if oldestExists || !replacementExists || negativeCount != 2 {
		t.Fatalf("negative cache bound mismatch: oldest=%t replacement=%t count=%d", oldestExists, replacementExists, negativeCount)
	}
}

func TestCatalogVideoThumbnailServingLeaseProtectsPruneAndStaleReplacement(t *testing.T) {
	t.Parallel()
	service, _ := newCatalogVideoThumbnailTestService(t, "video")
	var runs atomic.Int32
	service.runCommand = func(_ context.Context, _ string, args []string) error {
		runs.Add(1)
		return writeCatalogVideoThumbnailJPEG(args[len(args)-1])
	}
	first, err := service.Resolve(context.Background(), "item-video")
	if err != nil {
		t.Fatalf("resolve first thumbnail: %v", err)
	}
	if first.Release == nil {
		t.Fatal("first thumbnail has no serving lease")
	}
	service.maxCacheAge = time.Hour
	now := first.ModTime.Add(2 * time.Hour)
	service.now = func() time.Time { return now }

	type resolveResult struct {
		thumbnail CatalogVideoThumbnail
		err       error
	}
	secondResult := make(chan resolveResult, 1)
	go func() {
		thumbnail, resolveErr := service.Resolve(context.Background(), "item-video")
		secondResult <- resolveResult{thumbnail: thumbnail, err: resolveErr}
	}()
	waitForCatalogVideoThumbnailServingReplacement(t, service, first.Path)
	if got := runs.Load(); got != 1 {
		t.Fatalf("stale replacement ran while prior thumbnail was served: runs=%d", got)
	}
	if err := service.pruneCache(true, "", 0, 0); err != nil {
		t.Fatalf("prune while serving: %v", err)
	}
	if _, err := os.Stat(first.Path); err != nil {
		t.Fatalf("serving thumbnail was pruned: %v", err)
	}
	select {
	case outcome := <-secondResult:
		if outcome.err == nil {
			outcome.thumbnail.Release()
		}
		t.Fatalf("stale replacement completed before lease release: %v", outcome.err)
	default:
	}

	first.Release()
	first.Release()
	select {
	case outcome := <-secondResult:
		if outcome.err != nil {
			t.Fatalf("stale replacement after release: %v", outcome.err)
		}
		outcome.thumbnail.Release()
	case <-time.After(2 * time.Second):
		t.Fatal("stale replacement did not resume after serving lease release")
	}
	if got := runs.Load(); got != 2 {
		t.Fatalf("stale replacement generation count = %d, want 2", got)
	}
}

func TestCatalogVideoThumbnailCachePrunesAgeEntriesBytesAndProtectsLivePaths(t *testing.T) {
	t.Parallel()
	service, _ := newCatalogVideoThumbnailTestService(t, "video")
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.maxCacheEntries = 2
	service.maxCacheBytes = 1_000
	service.maxCacheAge = 24 * time.Hour
	if err := ensureCatalogVideoThumbnailCacheDirectory(service.cacheDir); err != nil {
		t.Fatalf("prepare cache: %v", err)
	}
	oldPath := writeCatalogVideoThumbnailCacheRecord(t, service.cacheDir, "1", "a", 30, now.Add(-48*time.Hour))
	middlePath := writeCatalogVideoThumbnailCacheRecord(t, service.cacheDir, "2", "b", 40, now.Add(-10*time.Hour))
	currentPath := writeCatalogVideoThumbnailCacheRecord(t, service.cacheDir, "3", "c", 60, now.Add(-time.Hour))
	temporaryPath := filepath.Join(service.cacheDir, ".video-thumbnail-current.jpg")
	if err := os.WriteFile(temporaryPath, []byte("in flight"), 0o600); err != nil {
		t.Fatalf("write live temporary: %v", err)
	}
	if err := os.Chtimes(temporaryPath, now, now); err != nil {
		t.Fatalf("set live temporary time: %v", err)
	}

	service.pinActivePath(oldPath)
	service.mu.Lock()
	service.inflight[middlePath]++
	service.mu.Unlock()
	if err := service.pruneCache(true, currentPath, 0, 0); !errors.Is(err, errCatalogVideoThumbnailCacheBlocked) {
		t.Fatalf("fully pinned over-capacity prune error = %v", err)
	}
	for _, path := range []string{oldPath, middlePath, currentPath, temporaryPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("live cache path was pruned: %s: %v", path, err)
		}
	}

	service.unpinActivePath(oldPath)
	service.mu.Lock()
	delete(service.inflight, middlePath)
	service.signalCapacityLocked()
	service.mu.Unlock()
	if err := service.pruneCache(true, currentPath, 0, 0); err != nil {
		t.Fatalf("age prune: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expired thumbnail survived age prune: %v", err)
	}
	for _, path := range []string{middlePath, currentPath, temporaryPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("valid cache path was unexpectedly removed: %s: %v", path, err)
		}
	}

	service.maxCacheEntries = 10
	service.maxCacheBytes = 60
	if err := service.pruneCache(true, currentPath, 0, 0); err != nil {
		t.Fatalf("byte-budget prune: %v", err)
	}
	if _, err := os.Stat(middlePath); !os.IsNotExist(err) {
		t.Fatalf("oldest thumbnail survived byte-budget prune: %v", err)
	}
	if _, err := os.Stat(currentPath); err != nil {
		t.Fatalf("current thumbnail was pruned: %v", err)
	}
	if _, err := os.Stat(temporaryPath); err != nil {
		t.Fatalf("recent in-flight temporary was pruned: %v", err)
	}
}

func newCatalogVideoThumbnailTestService(t *testing.T, category string) (*CatalogVideoThumbnailService, string) {
	t.Helper()
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "movie.mp4")
	if err := os.WriteFile(sourcePath, []byte("registered local video"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	duration := int64(100_000)
	source := catalogListPreviewFile(t, "file-video", "video", sourcePath, now)
	source.Media.DurationMs = &duration
	asset := catalogListPreviewAsset(t, "asset-video", "item-video", source.ID, "original", now)
	item, err := library.NewItem(library.ItemParams{
		ID: "item-video", CatalogID: "catalog-1", Category: category, Status: "active",
		Title: "Movie", Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new Catalog item: %v", err)
	}
	toolDirectory := filepath.Join(directory, "tools")
	if err := os.MkdirAll(toolDirectory, 0o700); err != nil {
		t.Fatalf("create tool directory: %v", err)
	}
	toolPath := filepath.Join(toolDirectory, ffmpegExecutableName())
	mode := os.FileMode(0o700)
	if runtime.GOOS == "windows" {
		mode = 0o600
	}
	if err := os.WriteFile(toolPath, []byte("test executable placeholder"), mode); err != nil {
		t.Fatalf("write tool placeholder: %v", err)
	}
	return NewCatalogVideoThumbnailService(
		catalogVideoThumbnailItemRepository{items: map[string]library.Item{item.ID: item}},
		catalogPreviewAssetRepository{items: []library.ItemAsset{asset}},
		catalogPreviewFileRepository{items: map[string]library.LibraryFile{source.ID: source}},
		catalogVideoThumbnailTools{directory: toolDirectory},
		filepath.Join(directory, "cache"),
	), sourcePath
}

func writeCatalogVideoThumbnailJPEG(path string) error {
	preview := image.NewRGBA(image.Rect(0, 0, 320, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 320; x++ {
			preview.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 96, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	err = jpeg.Encode(file, preview, &jpeg.Options{Quality: 75})
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func containsAdjacentArguments(args []string, first string, second string) bool {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == first && args[index+1] == second {
			return true
		}
	}
	return false
}

func waitForCatalogVideoThumbnailFlightWaiters(
	t *testing.T,
	service *CatalogVideoThumbnailService,
	_ string,
	wantActive int,
) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		service.mu.Lock()
		active := 0
		for _, count := range service.active {
			active += count
		}
		flights := len(service.flights)
		service.mu.Unlock()
		if flights == 1 && active >= wantActive {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for shared thumbnail follower")
}

func waitForCatalogVideoThumbnailServingReplacement(
	t *testing.T,
	service *CatalogVideoThumbnailService,
	path string,
) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		service.mu.Lock()
		flights := len(service.flights)
		serving := service.serving[path]
		service.mu.Unlock()
		if flights == 1 && serving == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for stale replacement behind serving lease")
}

func writeCatalogVideoThumbnailCacheRecord(
	t *testing.T,
	directory string,
	prefixCharacter string,
	keyCharacter string,
	size int,
	modTime time.Time,
) string {
	t.Helper()
	path := filepath.Join(
		directory,
		strings.Repeat(prefixCharacter, 16)+"-"+strings.Repeat(keyCharacter, 64)+".jpg",
	)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o600); err != nil {
		t.Fatalf("write cache record: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set cache record time: %v", err)
	}
	return path
}
