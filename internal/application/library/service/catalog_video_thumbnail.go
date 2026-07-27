package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"xiadown/internal/domain/library"
)

const (
	catalogVideoThumbnailFormatVersion  = "v1"
	catalogVideoThumbnailMaxWidth       = 640
	catalogVideoThumbnailMaxHeight      = 360
	catalogVideoThumbnailMaxBytes       = 4 << 20
	catalogVideoThumbnailTimeout        = 20 * time.Second
	catalogVideoThumbnailCacheEntries   = 4_096
	catalogVideoThumbnailCacheBytes     = int64(512 << 20)
	catalogVideoThumbnailCacheMaxAge    = 90 * 24 * time.Hour
	catalogVideoThumbnailPruneInterval  = 5 * time.Minute
	catalogVideoThumbnailNegativeTTL    = 2 * time.Minute
	catalogVideoThumbnailNegativeLimit  = 1_024
	catalogVideoThumbnailTempMaxAge     = time.Hour
	catalogVideoThumbnailItemIDMaxRunes = 255
	catalogVideoThumbnailItemIDMaxBytes = catalogVideoThumbnailItemIDMaxRunes * utf8.UTFMax
)

var (
	ErrCatalogVideoThumbnailNotFound     = errors.New("catalog video thumbnail not found")
	ErrCatalogVideoThumbnailUnavailable  = errors.New("catalog video thumbnail unavailable")
	errCatalogVideoThumbnailCacheBlocked = errors.New("catalog video thumbnail cache capacity blocked")
	catalogVideoThumbnailFilePattern     = regexp.MustCompile(`^[0-9a-f]{16}-[0-9a-f]{64}\.jpg$`)
)

// CatalogVideoThumbnail is a generated, immutable preview. The path is kept
// inside the token-protected Desktop HTTP boundary and is never accepted from
// an HTTP caller.
type CatalogVideoThumbnail struct {
	Path    string
	ETag    string
	ModTime time.Time
	// Release ends the cache-file lease held for this response. Consumers must
	// call it after they finish reading Path; the function is safe to call more
	// than once.
	Release func()
}

type catalogVideoThumbnailFlight struct {
	done   chan struct{}
	result CatalogVideoThumbnail
	err    error
}

type catalogVideoThumbnailNegative struct {
	expires time.Time
	err     error
}

type catalogVideoThumbnailCacheEntry struct {
	path    string
	size    int64
	updated time.Time
	pinned  bool
}

type catalogVideoThumbnailCommand func(context.Context, string, []string) error

// CatalogVideoThumbnailService lazily creates one bounded JPEG for a logical
// video item. Requests carry only an opaque Catalog item ID; the service
// resolves both the source file and the trusted FFmpeg executable itself.
// One decoder is allowed process-wide and same-source requests share a flight.
type CatalogVideoThumbnailService struct {
	items    library.CatalogItemRepository
	assets   library.ItemAssetRepository
	files    library.FileRepository
	roots    library.StorageRootRepository
	tools    ToolResolver
	cacheDir string

	decodeSlot chan struct{}
	mu         sync.Mutex
	flights    map[string]*catalogVideoThumbnailFlight
	negative   map[string]catalogVideoThumbnailNegative
	active     map[string]int
	serving    map[string]int
	inflight   map[string]int
	capacity   chan struct{}

	cacheMu            sync.Mutex
	lastPrune          time.Time
	maxCacheEntries    int
	maxCacheBytes      int64
	maxCacheAge        time.Duration
	pruneInterval      time.Duration
	negativeTTL        time.Duration
	maxNegativeEntries int
	generationTimeout  time.Duration
	now                func() time.Time
	runCommand         catalogVideoThumbnailCommand
}

func NewCatalogVideoThumbnailService(
	items library.CatalogItemRepository,
	assets library.ItemAssetRepository,
	files library.FileRepository,
	tools ToolResolver,
	cacheDir string,
	roots ...library.StorageRootRepository,
) *CatalogVideoThumbnailService {
	service := &CatalogVideoThumbnailService{
		items: items, assets: assets, files: files, tools: tools,
		cacheDir:           strings.TrimSpace(cacheDir),
		decodeSlot:         make(chan struct{}, 1),
		flights:            make(map[string]*catalogVideoThumbnailFlight),
		negative:           make(map[string]catalogVideoThumbnailNegative),
		active:             make(map[string]int),
		serving:            make(map[string]int),
		inflight:           make(map[string]int),
		capacity:           make(chan struct{}),
		maxCacheEntries:    catalogVideoThumbnailCacheEntries,
		maxCacheBytes:      catalogVideoThumbnailCacheBytes,
		maxCacheAge:        catalogVideoThumbnailCacheMaxAge,
		pruneInterval:      catalogVideoThumbnailPruneInterval,
		negativeTTL:        catalogVideoThumbnailNegativeTTL,
		maxNegativeEntries: catalogVideoThumbnailNegativeLimit,
		generationTimeout:  catalogVideoThumbnailTimeout,
		now:                func() time.Time { return time.Now().UTC() },
		runCommand:         runCatalogVideoThumbnailCommand,
	}
	if len(roots) > 0 {
		service.roots = roots[0]
	}
	return service
}

func (service *CatalogVideoThumbnailService) Resolve(
	ctx context.Context,
	itemID string,
) (CatalogVideoThumbnail, error) {
	if service == nil || service.items == nil || service.assets == nil || service.files == nil ||
		service.tools == nil || service.cacheDir == "" {
		return CatalogVideoThumbnail{}, ErrCatalogVideoThumbnailUnavailable
	}
	if !ValidCatalogVideoThumbnailItemID(itemID) {
		return CatalogVideoThumbnail{}, ErrCatalogVideoThumbnailNotFound
	}
	itemID = strings.TrimSpace(itemID)
	item, err := service.items.Get(ctx, itemID)
	if err != nil {
		return CatalogVideoThumbnail{}, ErrCatalogVideoThumbnailNotFound
	}
	if item.Category != library.ItemCategoryVideo || item.Status == library.ItemStatusTrashed ||
		item.Status == library.ItemStatusMissing {
		return CatalogVideoThumbnail{}, ErrCatalogVideoThumbnailNotFound
	}

	source, err := service.resolveSourceFile(ctx, item.ID)
	if err != nil {
		return CatalogVideoThumbnail{}, err
	}
	sourcePath, sourceInfo, err := resolveCatalogVideoThumbnailSource(source)
	if err != nil {
		return CatalogVideoThumbnail{}, err
	}
	cacheKey := catalogVideoThumbnailCacheKey(item.ID, source.ID, sourcePath, sourceInfo)
	itemPrefix := catalogVideoThumbnailItemPrefix(item.ID)
	result := CatalogVideoThumbnail{
		Path: filepath.Join(service.cacheDir, itemPrefix+"-"+cacheKey+".jpg"),
		ETag: `"xiadown-video-thumbnail-` + cacheKey + `"`,
	}
	service.pinActivePath(result.Path)
	if modTime, ok := service.leaseFreshCacheEntry(result.Path); ok {
		result.ModTime = modTime
		service.clearNegative(cacheKey)
		_ = service.pruneCache(false, result.Path, 0, 0)
		result.Release = service.releaseCatalogVideoThumbnailLease(result.Path)
		return result, nil
	}

	resolved, err := service.resolveFlight(ctx, cacheKey, result.Path, func(runCtx context.Context) (CatalogVideoThumbnail, error) {
		if modTime, ok := validCatalogVideoThumbnail(result.Path); ok && service.cacheEntryFresh(modTime) {
			result.ModTime = modTime
			return result, nil
		}
		select {
		case service.decodeSlot <- struct{}{}:
			defer func() { <-service.decodeSlot }()
		case <-runCtx.Done():
			return CatalogVideoThumbnail{}, runCtx.Err()
		}
		if err := ensureCatalogVideoThumbnailCacheDirectory(service.cacheDir); err != nil {
			return CatalogVideoThumbnail{}, ErrCatalogVideoThumbnailUnavailable
		}
		// A stale file may still be open by an HTTP response that crossed the age
		// threshold. Wait for that serving lease before replacing the same path;
		// this is required on Windows and avoids unlinking an active Unix reader.
		if err := service.removeReplaceableCacheEntry(runCtx, result.Path); err != nil {
			return CatalogVideoThumbnail{}, err
		}
		if err := service.reserveCacheCapacity(runCtx, result.Path); err != nil {
			return CatalogVideoThumbnail{}, err
		}
		if err := service.generate(runCtx, source, sourcePath, result.Path); err != nil {
			return CatalogVideoThumbnail{}, err
		}
		modTime, ok := validCatalogVideoThumbnail(result.Path)
		if !ok {
			return CatalogVideoThumbnail{}, ErrCatalogVideoThumbnailUnavailable
		}
		result.ModTime = modTime
		service.pruneSuperseded(itemPrefix, result.Path)
		if err := service.pruneCache(true, result.Path, 0, 0); err != nil {
			_ = os.Remove(result.Path)
			return CatalogVideoThumbnail{}, ErrCatalogVideoThumbnailUnavailable
		}
		return result, nil
	})
	if err != nil {
		service.unpinActivePath(result.Path)
		return CatalogVideoThumbnail{}, err
	}
	service.promoteActivePathToServing(resolved.Path)
	resolved.Release = service.releaseCatalogVideoThumbnailLease(resolved.Path)
	return resolved, nil
}

func (service *CatalogVideoThumbnailService) resolveSourceFile(
	ctx context.Context,
	itemID string,
) (library.LibraryFile, error) {
	assets, err := service.assets.ListByItemID(ctx, itemID)
	if err != nil {
		return library.LibraryFile{}, ErrCatalogVideoThumbnailUnavailable
	}
	filesByAssetID := make(map[string]library.LibraryFile, len(assets))
	for _, asset := range assets {
		if asset.Role != library.ItemAssetRoleOriginal && asset.Role != library.ItemAssetRoleRepresentation {
			continue
		}
		file, fileErr := service.files.Get(ctx, asset.FileID)
		if fileErr != nil {
			continue
		}
		filesByAssetID[asset.ID] = file
	}
	_, source, ok := selectCatalogPrimaryAsset(assets, filesByAssetID)
	if !ok || !catalogFileCanAttemptRead(ctx, source, service.roots) {
		return library.LibraryFile{}, ErrCatalogVideoThumbnailNotFound
	}
	return source, nil
}

func resolveCatalogVideoThumbnailSource(
	source library.LibraryFile,
) (string, os.FileInfo, error) {
	rawPath := strings.TrimSpace(source.Storage.LocalPath)
	if rawPath == "" {
		return "", nil, ErrCatalogVideoThumbnailNotFound
	}
	path, err := filepath.Abs(rawPath)
	if err != nil || !filepath.IsAbs(path) {
		return "", nil, ErrCatalogVideoThumbnailNotFound
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || !info.Mode().IsRegular() {
		return "", nil, ErrCatalogVideoThumbnailNotFound
	}
	return path, info, nil
}

func (service *CatalogVideoThumbnailService) resolveFlight(
	ctx context.Context,
	key string,
	targetPath string,
	generate func(context.Context) (CatalogVideoThumbnail, error),
) (CatalogVideoThumbnail, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	service.mu.Lock()
	now := service.currentTime()
	service.pruneNegativeLocked(now)
	if flight := service.flights[key]; flight != nil {
		service.mu.Unlock()
		return waitCatalogVideoThumbnailFlight(ctx, flight)
	}
	if negative, exists := service.negative[key]; exists {
		if now.Before(negative.expires) {
			service.mu.Unlock()
			return CatalogVideoThumbnail{}, negative.err
		}
		delete(service.negative, key)
	}
	flight := &catalogVideoThumbnailFlight{done: make(chan struct{})}
	service.flights[key] = flight
	service.inflight[targetPath]++
	service.mu.Unlock()

	go func() {
		timeout := service.generationTimeout
		if timeout <= 0 {
			timeout = catalogVideoThumbnailTimeout
		}
		runCtx, cancel := context.WithTimeout(context.Background(), timeout)
		result, err := generate(runCtx)
		cancel()

		service.mu.Lock()
		flight.result = result
		flight.err = err
		delete(service.flights, key)
		if count := service.inflight[targetPath]; count <= 1 {
			delete(service.inflight, targetPath)
		} else {
			service.inflight[targetPath] = count - 1
		}
		now := service.currentTime()
		service.pruneNegativeLocked(now)
		if err == nil {
			delete(service.negative, key)
		} else if !errors.Is(err, context.Canceled) {
			ttl := service.negativeTTL
			if ttl <= 0 {
				ttl = catalogVideoThumbnailNegativeTTL
			}
			service.negative[key] = catalogVideoThumbnailNegative{
				expires: now.Add(ttl),
				err:     ErrCatalogVideoThumbnailUnavailable,
			}
			service.pruneNegativeLocked(now)
		}
		close(flight.done)
		service.signalCapacityLocked()
		service.mu.Unlock()
	}()

	return waitCatalogVideoThumbnailFlight(ctx, flight)
}

func waitCatalogVideoThumbnailFlight(
	ctx context.Context,
	flight *catalogVideoThumbnailFlight,
) (CatalogVideoThumbnail, error) {
	select {
	case <-flight.done:
		return flight.result, flight.err
	case <-ctx.Done():
		return CatalogVideoThumbnail{}, ctx.Err()
	}
}

func (service *CatalogVideoThumbnailService) generate(
	ctx context.Context,
	source library.LibraryFile,
	sourcePath string,
	targetPath string,
) error {
	if err := ensureCatalogVideoThumbnailCacheDirectory(service.cacheDir); err != nil {
		return ErrCatalogVideoThumbnailUnavailable
	}
	execPath, err := resolveFFmpegExecPath(ctx, service.tools)
	if err != nil {
		return ErrCatalogVideoThumbnailUnavailable
	}
	temporary, err := os.CreateTemp(service.cacheDir, ".video-thumbnail-*.jpg")
	if err != nil {
		return ErrCatalogVideoThumbnailUnavailable
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return ErrCatalogVideoThumbnailUnavailable
	}
	defer os.Remove(temporaryPath)

	args := buildCatalogVideoThumbnailFFmpegArgs(source, sourcePath, temporaryPath)
	runner := service.runCommand
	if runner == nil {
		runner = runCatalogVideoThumbnailCommand
	}
	if err := runner(ctx, execPath, args); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return ErrCatalogVideoThumbnailUnavailable
	}
	if _, ok := validCatalogVideoThumbnail(temporaryPath); !ok {
		return ErrCatalogVideoThumbnailUnavailable
	}
	info, err := os.Stat(temporaryPath)
	if err != nil || info.Size() > service.cacheByteLimit() {
		return ErrCatalogVideoThumbnailUnavailable
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return ErrCatalogVideoThumbnailUnavailable
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return ErrCatalogVideoThumbnailUnavailable
	}
	return nil
}

func buildCatalogVideoThumbnailFFmpegArgs(
	source library.LibraryFile,
	sourcePath string,
	targetPath string,
) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-filter_threads", "1", "-threads", "1",
	}
	if seek := catalogVideoThumbnailSeek(source); seek > 0 {
		args = append(args, "-ss", strconv.FormatFloat(seek, 'f', 3, 64), "-noaccurate_seek")
	}
	args = appendLocalMediaFFmpegInput(args, sourcePath)
	return append(args,
		"-map", "0:v:0",
		"-an", "-sn", "-dn",
		"-frames:v", "1",
		"-vf", "scale=640:360:force_original_aspect_ratio=decrease:force_divisible_by=2:flags=fast_bilinear,setsar=1",
		"-threads:v", "1",
		"-q:v", "5",
		"-f", "image2", "-update", "1", "-y", targetPath,
	)
}

func catalogVideoThumbnailSeek(source library.LibraryFile) float64 {
	if source.Media == nil || source.Media.DurationMs == nil || *source.Media.DurationMs <= 2_000 {
		return 0
	}
	duration := float64(*source.Media.DurationMs) / 1000
	seek := duration * 0.1
	if seek < 1 {
		seek = 1
	}
	if seek > 30 {
		seek = 30
	}
	if latest := duration - 0.5; seek > latest {
		seek = latest
	}
	if seek < 0 {
		return 0
	}
	return seek
}

func runCatalogVideoThumbnailCommand(ctx context.Context, execPath string, args []string) error {
	command := exec.CommandContext(ctx, execPath, args...)
	configureLocalMediaToolCommand(command)
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail failed: %w", err)
	}
	return nil
}

func ensureCatalogVideoThumbnailCacheDirectory(path string) error {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return ErrCatalogVideoThumbnailUnavailable
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrCatalogVideoThumbnailUnavailable
	}
	return os.Chmod(path, 0o700)
}

func (service *CatalogVideoThumbnailService) currentTime() time.Time {
	if service == nil || service.now == nil {
		return time.Now().UTC()
	}
	return service.now().UTC()
}

func (service *CatalogVideoThumbnailService) cacheEntryFresh(modTime time.Time) bool {
	maxAge := service.maxCacheAge
	if maxAge <= 0 {
		maxAge = catalogVideoThumbnailCacheMaxAge
	}
	return modTime.After(service.currentTime().Add(-maxAge))
}

func (service *CatalogVideoThumbnailService) cacheEntryLimit() int {
	if service != nil && service.maxCacheEntries > 0 {
		return service.maxCacheEntries
	}
	return catalogVideoThumbnailCacheEntries
}

func (service *CatalogVideoThumbnailService) cacheByteLimit() int64 {
	if service != nil && service.maxCacheBytes > 0 {
		return service.maxCacheBytes
	}
	return catalogVideoThumbnailCacheBytes
}

func (service *CatalogVideoThumbnailService) pinActivePath(path string) {
	service.mu.Lock()
	service.active[path]++
	service.mu.Unlock()
}

func (service *CatalogVideoThumbnailService) leaseFreshCacheEntry(path string) (time.Time, bool) {
	service.cacheMu.Lock()
	defer service.cacheMu.Unlock()
	modTime, ok := validCatalogVideoThumbnail(path)
	if !ok || !service.cacheEntryFresh(modTime) {
		return time.Time{}, false
	}
	service.mu.Lock()
	service.serving[path]++
	service.mu.Unlock()
	return modTime, true
}

func (service *CatalogVideoThumbnailService) promoteActivePathToServing(path string) {
	// Serialize the handoff with exact-path replacement. The active pin already
	// prevents global eviction; cacheMu closes the smaller validate/replace race.
	service.cacheMu.Lock()
	service.mu.Lock()
	service.serving[path]++
	service.mu.Unlock()
	service.cacheMu.Unlock()
}

func (service *CatalogVideoThumbnailService) releaseCatalogVideoThumbnailLease(path string) func() {
	return sync.OnceFunc(func() {
		service.mu.Lock()
		if count := service.serving[path]; count <= 1 {
			delete(service.serving, path)
		} else {
			service.serving[path] = count - 1
		}
		if count := service.active[path]; count <= 1 {
			delete(service.active, path)
		} else {
			service.active[path] = count - 1
		}
		service.signalCapacityLocked()
		service.mu.Unlock()
	})
}

func (service *CatalogVideoThumbnailService) unpinActivePath(path string) {
	service.mu.Lock()
	if count := service.active[path]; count <= 1 {
		delete(service.active, path)
	} else {
		service.active[path] = count - 1
	}
	service.signalCapacityLocked()
	service.mu.Unlock()
}

func (service *CatalogVideoThumbnailService) removeReplaceableCacheEntry(
	ctx context.Context,
	path string,
) error {
	for {
		service.cacheMu.Lock()
		service.mu.Lock()
		serving := service.serving[path]
		changed := service.capacity
		service.mu.Unlock()
		if serving == 0 {
			err := os.Remove(path)
			service.cacheMu.Unlock()
			if err == nil || errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return ErrCatalogVideoThumbnailUnavailable
		}
		service.cacheMu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (service *CatalogVideoThumbnailService) clearNegative(key string) {
	service.mu.Lock()
	service.pruneNegativeLocked(service.currentTime())
	delete(service.negative, key)
	service.mu.Unlock()
}

func (service *CatalogVideoThumbnailService) pruneNegativeLocked(now time.Time) {
	for key, negative := range service.negative {
		if !now.Before(negative.expires) {
			delete(service.negative, key)
		}
	}
	limit := service.maxNegativeEntries
	if limit <= 0 {
		limit = catalogVideoThumbnailNegativeLimit
	}
	for len(service.negative) > limit {
		victimKey := ""
		var victimExpiry time.Time
		for key, negative := range service.negative {
			if victimKey == "" || negative.expires.Before(victimExpiry) ||
				(negative.expires.Equal(victimExpiry) && key < victimKey) {
				victimKey = key
				victimExpiry = negative.expires
			}
		}
		if victimKey == "" {
			return
		}
		delete(service.negative, victimKey)
	}
}

func (service *CatalogVideoThumbnailService) signalCapacityLocked() {
	if service.capacity != nil {
		close(service.capacity)
	}
	service.capacity = make(chan struct{})
}

func (service *CatalogVideoThumbnailService) protectedCachePaths(keepPath string) map[string]struct{} {
	service.mu.Lock()
	defer service.mu.Unlock()
	result := make(map[string]struct{}, len(service.active)+len(service.inflight)+1)
	for path, count := range service.active {
		if count > 0 {
			result[path] = struct{}{}
		}
	}
	for path, count := range service.inflight {
		if count > 0 {
			result[path] = struct{}{}
		}
	}
	if keepPath != "" {
		result[keepPath] = struct{}{}
	}
	return result
}

func (service *CatalogVideoThumbnailService) reserveCacheCapacity(ctx context.Context, keepPath string) error {
	byteLimit := service.cacheByteLimit()
	reservation := int64(catalogVideoThumbnailMaxBytes)
	if reservation > byteLimit {
		reservation = byteLimit
	}
	for {
		service.mu.Lock()
		changed := service.capacity
		service.mu.Unlock()
		err := service.pruneCache(true, keepPath, 1, reservation)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errCatalogVideoThumbnailCacheBlocked) {
			return ErrCatalogVideoThumbnailUnavailable
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

// pruneCache enforces generation age, entry count and aggregate byte limits.
// Only committed files matching the internal name grammar participate; live
// targets are pinned and temporary files are ignored until safely abandoned.
func (service *CatalogVideoThumbnailService) pruneCache(
	force bool,
	keepPath string,
	reserveEntries int,
	reserveBytes int64,
) error {
	if err := ensureCatalogVideoThumbnailCacheDirectory(service.cacheDir); err != nil {
		return ErrCatalogVideoThumbnailUnavailable
	}
	service.cacheMu.Lock()
	defer service.cacheMu.Unlock()
	now := service.currentTime()
	interval := service.pruneInterval
	if interval <= 0 {
		interval = catalogVideoThumbnailPruneInterval
	}
	if !force && !service.lastPrune.IsZero() && now.Sub(service.lastPrune) < interval {
		return nil
	}
	protected := service.protectedCachePaths(keepPath)
	children, err := os.ReadDir(service.cacheDir)
	if err != nil {
		return ErrCatalogVideoThumbnailUnavailable
	}
	maxAge := service.maxCacheAge
	if maxAge <= 0 {
		maxAge = catalogVideoThumbnailCacheMaxAge
	}
	entries := make([]catalogVideoThumbnailCacheEntry, 0, len(children))
	var total int64
	for _, child := range children {
		name := child.Name()
		path := filepath.Join(service.cacheDir, name)
		info, infoErr := os.Lstat(path)
		if infoErr != nil {
			continue
		}
		if strings.HasPrefix(name, ".video-thumbnail-") {
			if now.Sub(info.ModTime()) > catalogVideoThumbnailTempMaxAge {
				if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					return ErrCatalogVideoThumbnailUnavailable
				}
			}
			continue
		}
		if !catalogVideoThumbnailFilePattern.MatchString(name) {
			continue
		}
		_, pinned := protected[path]
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			if !pinned {
				if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					return ErrCatalogVideoThumbnailUnavailable
				}
			}
			continue
		}
		if now.Sub(info.ModTime()) > maxAge && !pinned {
			if removeErr := os.Remove(path); removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
				continue
			}
		}
		entries = append(entries, catalogVideoThumbnailCacheEntry{
			path: path, size: info.Size(), updated: info.ModTime(), pinned: pinned,
		})
		total += info.Size()
	}
	entryLimit := service.cacheEntryLimit() - reserveEntries
	if entryLimit < 0 {
		entryLimit = 0
	}
	byteLimit := service.cacheByteLimit() - reserveBytes
	if byteLimit < 0 {
		byteLimit = 0
	}
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].updated.Equal(entries[right].updated) {
			return entries[left].path < entries[right].path
		}
		return entries[left].updated.Before(entries[right].updated)
	})
	remaining := len(entries)
	for _, entry := range entries {
		if remaining <= entryLimit && total <= byteLimit {
			break
		}
		if entry.pinned {
			continue
		}
		if removeErr := os.Remove(entry.path); removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
			remaining--
			total -= entry.size
		}
	}
	if remaining <= entryLimit && total <= byteLimit {
		service.lastPrune = now
		return nil
	}
	for _, entry := range entries {
		if entry.pinned {
			return errCatalogVideoThumbnailCacheBlocked
		}
	}
	return ErrCatalogVideoThumbnailUnavailable
}

func validCatalogVideoThumbnail(path string) (time.Time, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Size() <= 0 || info.Size() > catalogVideoThumbnailMaxBytes {
		return time.Time{}, false
	}
	file, err := os.Open(path)
	if err != nil {
		return time.Time{}, false
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil || format != "jpeg" || config.Width <= 0 || config.Height <= 0 ||
		config.Width > catalogVideoThumbnailMaxWidth || config.Height > catalogVideoThumbnailMaxHeight {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// ValidCatalogVideoThumbnailItemID applies the Catalog domain's opaque ID
// length boundary before either HTTP or repository work is attempted.
func ValidCatalogVideoThumbnailItemID(value string) bool {
	if len(value) == 0 || len(value) > catalogVideoThumbnailItemIDMaxBytes || !utf8.ValidString(value) {
		return false
	}
	value = strings.TrimSpace(value)
	return value != "" && utf8.RuneCountInString(value) <= catalogVideoThumbnailItemIDMaxRunes &&
		!strings.ContainsAny(value, "/\\") &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}

func catalogVideoThumbnailCacheKey(itemID string, fileID string, sourcePath string, info os.FileInfo) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(catalogVideoThumbnailFormatVersion))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strings.TrimSpace(itemID)))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strings.TrimSpace(fileID)))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(filepath.Clean(strings.TrimSpace(sourcePath))))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strconv.FormatInt(info.Size(), 10)))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	return hex.EncodeToString(hash.Sum(nil))
}

func catalogVideoThumbnailItemPrefix(itemID string) string {
	value := sha256.Sum256([]byte(strings.TrimSpace(itemID)))
	return hex.EncodeToString(value[:8])
}

func (service *CatalogVideoThumbnailService) pruneSuperseded(itemPrefix string, keepPath string) {
	if service == nil || len(itemPrefix) != 16 || service.cacheDir == "" {
		return
	}
	service.cacheMu.Lock()
	defer service.cacheMu.Unlock()
	protected := service.protectedCachePaths(keepPath)
	entries, err := os.ReadDir(service.cacheDir)
	if err != nil {
		return
	}
	prefix := itemPrefix + "-"
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".jpg") {
			continue
		}
		path := filepath.Join(service.cacheDir, entry.Name())
		if _, pinned := protected[path]; path != keepPath && !pinned {
			_ = os.Remove(path)
		}
	}
}
