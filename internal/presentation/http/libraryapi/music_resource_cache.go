package libraryapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"xiadown/internal/domain/library"
)

const (
	defaultPublicMusicResourceCacheEntries = 4_096
	defaultPublicMusicResourceCacheBytes   = int64(20 << 30)
)

var (
	publicMusicResourceBlobPattern = regexp.MustCompile(`^[0-9a-f]{64}\.blob$`)
	errPublicMusicResourceChanged  = errors.New("public Music resource changed")
	errPublicMusicResourceCache    = errors.New("public Music resource cache unavailable")
)

type publicMusicResourceMaterializeCall struct {
	done chan struct{}
	err  error
}

type publicMusicResourcePruneStatus uint8

const (
	publicMusicResourcePruneSucceeded publicMusicResourcePruneStatus = iota
	publicMusicResourcePruneBlocked
	publicMusicResourcePruneFailed
)

type publicMusicResourceLease struct {
	*os.File
	cache *publicMusicResourceMaterializer
	path  string
	once  sync.Once
}

func (lease *publicMusicResourceLease) Close() error {
	if lease == nil {
		return nil
	}
	var closeErr error
	lease.once.Do(func() {
		closeErr = lease.File.Close()
		lease.cache.releaseServingBlob(lease.path)
	})
	return closeErr
}

// publicMusicResourceMaterializer turns a mutable Library source into an
// App-owned, content-verified blob before the HTTP layer publishes any bytes.
// Range and HEAD requests subsequently read only that immutable cache file.
type publicMusicResourceMaterializer struct {
	directory  string
	maxEntries int
	maxBytes   int64

	mu              sync.Mutex
	calls           map[string]*publicMusicResourceMaterializeCall
	reservedEntries int
	reservedBytes   int64
	capacityChanged chan struct{}
	materializing   map[string]int
	serving         map[string]int

	// beforeCopy is a test-only observation hook. It is deliberately invoked
	// only by the single materialization leader.
	beforeCopy func()
	// afterInstall pauses tests in the otherwise tiny rename-to-open window.
	afterInstall func()
}

func newPublicMusicResourceMaterializer(directory string, maxEntries int, maxBytes int64) (*publicMusicResourceMaterializer, error) {
	directory = filepath.Clean(strings.TrimSpace(directory))
	if directory == "." || !filepath.IsAbs(directory) {
		return nil, errPublicMusicResourceCache
	}
	if maxEntries <= 0 {
		maxEntries = defaultPublicMusicResourceCacheEntries
	}
	if maxBytes <= 0 {
		maxBytes = defaultPublicMusicResourceCacheBytes
	}
	if err := ensurePrivatePublicMusicResourceDirectory(directory); err != nil {
		return nil, err
	}
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil || !filepath.IsAbs(resolvedDirectory) {
		return nil, errPublicMusicResourceCache
	}
	directory = resolvedDirectory
	if err := ensurePrivatePublicMusicResourceDirectory(directory); err != nil {
		return nil, err
	}
	cache := &publicMusicResourceMaterializer{
		directory: directory, maxEntries: maxEntries, maxBytes: maxBytes,
		calls:           make(map[string]*publicMusicResourceMaterializeCall),
		capacityChanged: make(chan struct{}),
		materializing:   make(map[string]int),
		serving:         make(map[string]int),
	}
	// No materialization from an earlier process can still own a temporary
	// file. Remove every abandoned staging file before admitting new work so
	// a crash cannot leave fresh, quota-invisible copies behind for 24 hours.
	if err := cache.removeAbandonedTemporaryFiles(); err != nil {
		return nil, err
	}
	cache.mu.Lock()
	pruneStatus := cache.pruneToLimitsLocked(cache.maxEntries, cache.maxBytes)
	cache.mu.Unlock()
	if pruneStatus != publicMusicResourcePruneSucceeded {
		return nil, fmt.Errorf("%w: reconcile cache capacity", errPublicMusicResourceCache)
	}
	return cache, nil
}

func ensurePrivatePublicMusicResourceDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("%w: create directory", errPublicMusicResourceCache)
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errPublicMusicResourceCache
	}
	if err := securePublicMusicResourceDirectory(directory); err != nil {
		return fmt.Errorf("%w: secure directory", errPublicMusicResourceCache)
	}
	return nil
}

func (cache *publicMusicResourceMaterializer) open(
	ctx context.Context,
	resource library.ListenLocalMusicResource,
) (*publicMusicResourceLease, os.FileInfo, error) {
	if cache == nil {
		return nil, nil, errPublicMusicResourceCache
	}
	key, finalPath, err := cache.resourceKey(resource)
	if err != nil {
		return nil, nil, err
	}
	if file, info, found := cache.openCached(finalPath, resource); found {
		return file, info, nil
	}

	cache.mu.Lock()
	if call := cache.calls[key]; call != nil {
		cache.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-call.done:
			if call.err != nil {
				if (errors.Is(call.err, context.Canceled) || errors.Is(call.err, context.DeadlineExceeded)) &&
					ctx.Err() == nil {
					return cache.open(ctx, resource)
				}
				return nil, nil, call.err
			}
			file, info, found := cache.openCached(finalPath, resource)
			if !found {
				// Another key may have needed the entire cache budget after
				// the leader opened its immutable fd. Join or lead a fresh
				// materialization instead of surfacing a transient 404.
				return cache.open(ctx, resource)
			}
			return file, info, nil
		}
	}
	call := &publicMusicResourceMaterializeCall{done: make(chan struct{})}
	cache.calls[key] = call
	cache.mu.Unlock()

	file, info, err := cache.materialize(ctx, finalPath, resource)
	cache.mu.Lock()
	call.err = err
	delete(cache.calls, key)
	close(call.done)
	cache.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	if file == nil || info == nil {
		return nil, nil, errPublicMusicResourceCache
	}
	return file, info, nil
}

func (cache *publicMusicResourceMaterializer) resourceKey(resource library.ListenLocalMusicResource) (string, string, error) {
	checksum := strings.ToLower(strings.TrimSpace(resource.Checksum))
	if !strings.HasPrefix(checksum, "sha256:") || len(checksum) != len("sha256:")+64 || resource.Revision < 1 ||
		strings.TrimSpace(resource.ID) == "" || resource.ByteLength == nil || *resource.ByteLength < 0 ||
		*resource.ByteLength > cache.maxBytes {
		return "", "", errPublicMusicResourceChanged
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(checksum, "sha256:")); err != nil {
		return "", "", errPublicMusicResourceChanged
	}
	keyPayload := fmt.Sprintf(
		"xiadown-public-music-cas-v1\x00%s\x00%d\x00%d\x00%s",
		strings.TrimSpace(resource.ID), resource.Revision, *resource.ByteLength, checksum,
	)
	keyDigest := sha256.Sum256([]byte(keyPayload))
	key := hex.EncodeToString(keyDigest[:])
	return key, filepath.Join(cache.directory, key+".blob"), nil
}

func (cache *publicMusicResourceMaterializer) openCached(
	path string,
	resource library.ListenLocalMusicResource,
) (*publicMusicResourceLease, os.FileInfo, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	file, err := openPublicAssetNoFollow(path)
	if err != nil {
		return nil, nil, false
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !publicMusicResourceBlobIsProtected(path, info) ||
		resource.ByteLength == nil || info.Size() != *resource.ByteLength {
		_ = file.Close()
		return nil, nil, false
	}
	cache.serving[path]++
	return &publicMusicResourceLease{File: file, cache: cache, path: path}, info, true
}

func (cache *publicMusicResourceMaterializer) releaseServingBlob(path string) {
	cache.mu.Lock()
	if count := cache.serving[path]; count <= 1 {
		delete(cache.serving, path)
	} else {
		cache.serving[path] = count - 1
	}
	cache.signalCapacityChangedLocked()
	cache.mu.Unlock()
}

func (cache *publicMusicResourceMaterializer) materialize(
	ctx context.Context,
	finalPath string,
	resource library.ListenLocalMusicResource,
) (*publicMusicResourceLease, os.FileInfo, error) {
	source, err := openPublicAssetNoFollow(resource.LocalPath)
	if err != nil {
		return nil, nil, errPublicMusicResourceChanged
	}
	defer source.Close()
	sourceInfo, err := source.Stat()
	if err != nil || !sourceInfo.Mode().IsRegular() || resource.ByteLength == nil ||
		sourceInfo.Size() != *resource.ByteLength || sourceInfo.ModTime().UnixNano() != resource.ModTimeUnixNano {
		return nil, nil, errPublicMusicResourceChanged
	}
	if err := cache.reserveMaterialization(ctx, finalPath, *resource.ByteLength); err != nil {
		return nil, nil, err
	}
	defer cache.releaseMaterialization(finalPath, *resource.ByteLength)
	if cache.beforeCopy != nil {
		cache.beforeCopy()
	}

	temporary, err := os.CreateTemp(cache.directory, ".materialize-")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: create temporary blob", errPublicMusicResourceCache)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := securePublicMusicResourceTemporaryFile(temporaryPath); err != nil {
		return nil, nil, fmt.Errorf("%w: secure temporary blob", errPublicMusicResourceCache)
	}

	digest := sha256.New()
	buffer := make([]byte, 1024*1024)
	var copied int64
	expectedLength := *resource.ByteLength
	for copied < expectedLength {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		remaining := expectedLength - copied
		readLimit := len(buffer)
		if remaining < int64(readLimit) {
			readLimit = int(remaining)
		}
		read, readErr := source.Read(buffer[:readLimit])
		if read > 0 {
			chunk := buffer[:read]
			written, writeErr := temporary.Write(chunk)
			if writeErr != nil || written != read {
				return nil, nil, fmt.Errorf("%w: write temporary blob", errPublicMusicResourceCache)
			}
			if _, hashErr := digest.Write(chunk); hashErr != nil {
				return nil, nil, errPublicMusicResourceChanged
			}
			copied += int64(read)
		}
		if errors.Is(readErr, io.EOF) && copied < expectedLength {
			return nil, nil, errPublicMusicResourceChanged
		}
		if readErr != nil {
			return nil, nil, errPublicMusicResourceChanged
		}
		if read == 0 {
			return nil, nil, errPublicMusicResourceChanged
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	var overflowProbe [1]byte
	probeRead, probeErr := source.Read(overflowProbe[:])
	if probeRead != 0 || !errors.Is(probeErr, io.EOF) {
		return nil, nil, errPublicMusicResourceChanged
	}
	finalSourceInfo, err := source.Stat()
	if err != nil || !finalSourceInfo.Mode().IsRegular() || finalSourceInfo.Size() != expectedLength ||
		finalSourceInfo.ModTime().UnixNano() != resource.ModTimeUnixNano {
		return nil, nil, errPublicMusicResourceChanged
	}
	actual := "sha256:" + hex.EncodeToString(digest.Sum(nil))
	expected := strings.ToLower(strings.TrimSpace(resource.Checksum))
	if copied != *resource.ByteLength || subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return nil, nil, errPublicMusicResourceChanged
	}
	if err := temporary.Sync(); err != nil {
		return nil, nil, fmt.Errorf("%w: sync temporary blob", errPublicMusicResourceCache)
	}
	if err := temporary.Close(); err != nil {
		return nil, nil, fmt.Errorf("%w: close temporary blob", errPublicMusicResourceCache)
	}
	if err := securePublicMusicResourceVerifiedBlob(temporaryPath); err != nil {
		return nil, nil, fmt.Errorf("%w: secure verified blob", errPublicMusicResourceCache)
	}
	if file, info, found := cache.openCached(finalPath, resource); found {
		return file, info, nil
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		if file, info, found := cache.openCached(finalPath, resource); found {
			return file, info, nil
		}
		return nil, nil, fmt.Errorf("%w: install verified blob", errPublicMusicResourceCache)
	}
	temporaryPath = ""
	if err := securePublicMusicResourceVerifiedBlob(finalPath); err != nil {
		_ = os.Remove(finalPath)
		return nil, nil, fmt.Errorf("%w: secure installed blob", errPublicMusicResourceCache)
	}
	if cache.afterInstall != nil {
		cache.afterInstall()
	}
	file, info, found := cache.openCached(finalPath, resource)
	if !found {
		return nil, nil, errPublicMusicResourceCache
	}
	return file, info, nil
}

// reserveMaterialization accounts for both the temporary copy and the entry
// it will atomically become. Different resource keys are allowed to hash in
// parallel only while their combined staging bytes fit inside the same cache
// budget as committed blobs. Existing blobs are evicted before any temp file
// is created, so the process never relies on a post-rename prune to enforce
// the bound.
func (cache *publicMusicResourceMaterializer) reserveMaterialization(
	ctx context.Context,
	finalPath string,
	size int64,
) error {
	if size < 0 || size > cache.maxBytes {
		return errPublicMusicResourceChanged
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		cache.mu.Lock()
		if cache.reservedEntries < cache.maxEntries && size <= cache.maxBytes-cache.reservedBytes {
			entryLimit := cache.maxEntries - cache.reservedEntries - 1
			byteLimit := cache.maxBytes - cache.reservedBytes - size
			switch cache.pruneToLimitsLocked(entryLimit, byteLimit) {
			case publicMusicResourcePruneSucceeded:
				cache.reservedEntries++
				cache.reservedBytes += size
				cache.materializing[finalPath]++
				cache.mu.Unlock()
				return nil
			case publicMusicResourcePruneBlocked:
				changed := cache.capacityChanged
				cache.mu.Unlock()
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-changed:
					continue
				}
			case publicMusicResourcePruneFailed:
				cache.mu.Unlock()
				return fmt.Errorf("%w: reclaim cache capacity", errPublicMusicResourceCache)
			}
		}
		changed := cache.capacityChanged
		cache.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (cache *publicMusicResourceMaterializer) releaseMaterialization(finalPath string, size int64) {
	cache.mu.Lock()
	if cache.reservedEntries > 0 {
		cache.reservedEntries--
	}
	if size >= cache.reservedBytes {
		cache.reservedBytes = 0
	} else {
		cache.reservedBytes -= size
	}
	if count := cache.materializing[finalPath]; count <= 1 {
		delete(cache.materializing, finalPath)
	} else {
		cache.materializing[finalPath] = count - 1
	}
	cache.signalCapacityChangedLocked()
	cache.mu.Unlock()
}

func (cache *publicMusicResourceMaterializer) signalCapacityChangedLocked() {
	close(cache.capacityChanged)
	cache.capacityChanged = make(chan struct{})
}

type publicMusicResourceCacheEntry struct {
	path    string
	size    int64
	updated time.Time
	pinned  bool
}

func (cache *publicMusicResourceMaterializer) pruneToLimitsLocked(
	maxEntries int,
	maxBytes int64,
) publicMusicResourcePruneStatus {
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		return publicMusicResourcePruneFailed
	}
	files := make([]publicMusicResourceCacheEntry, 0, len(entries))
	var total int64
	for _, entry := range entries {
		// Every live .materialize-* file is already represented by one
		// reservation. Startup removes abandoned ones before this helper runs.
		if strings.HasPrefix(entry.Name(), ".materialize-") {
			continue
		}
		path := filepath.Join(cache.directory, entry.Name())
		info, infoErr := os.Lstat(path)
		if infoErr != nil {
			return publicMusicResourcePruneFailed
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if removeErr := os.Remove(path); removeErr != nil {
				return publicMusicResourcePruneFailed
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return publicMusicResourcePruneFailed
		}
		// A materialization reservation already accounts for the bytes and
		// entry represented by this just-installed path. Excluding it avoids
		// double-counting while still protecting it from another key's prune.
		if cache.materializing[path] > 0 {
			continue
		}
		files = append(files, publicMusicResourceCacheEntry{
			path: path, size: info.Size(), updated: info.ModTime(), pinned: cache.serving[path] > 0,
		})
		total += info.Size()
	}
	if len(files) <= maxEntries && total <= maxBytes {
		return publicMusicResourcePruneSucceeded
	}
	sort.Slice(files, func(left, right int) bool {
		if files[left].updated.Equal(files[right].updated) {
			return files[left].path < files[right].path
		}
		return files[left].updated.Before(files[right].updated)
	})
	remaining := len(files)
	for _, entry := range files {
		if remaining <= maxEntries && total <= maxBytes {
			break
		}
		if entry.pinned {
			continue
		}
		if err := os.Remove(entry.path); err == nil {
			remaining--
			total -= entry.size
		}
	}
	if remaining <= maxEntries && total <= maxBytes {
		return publicMusicResourcePruneSucceeded
	}
	for _, entry := range files {
		if entry.pinned {
			return publicMusicResourcePruneBlocked
		}
	}
	return publicMusicResourcePruneFailed
}

func (cache *publicMusicResourceMaterializer) removeAbandonedTemporaryFiles() error {
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		return fmt.Errorf("%w: inspect temporary blobs", errPublicMusicResourceCache)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".materialize-") {
			continue
		}
		path := filepath.Join(cache.directory, entry.Name())
		info, infoErr := os.Lstat(path)
		if infoErr != nil {
			return fmt.Errorf("%w: inspect abandoned temporary blob", errPublicMusicResourceCache)
		}
		if info.IsDir() {
			return fmt.Errorf("%w: unexpected temporary directory", errPublicMusicResourceCache)
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("%w: remove abandoned temporary blob", errPublicMusicResourceCache)
		}
	}
	return nil
}
