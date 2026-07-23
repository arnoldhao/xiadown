package libraryapi

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	applicationrss "xiadown/internal/application/rss"
)

const (
	defaultRSSImageDiskCacheBytes int64 = 256 << 20
	rssImageDiskCacheVersion            = 1
	rssImageDiskMetadataMaxBytes        = 16 << 10
	rssImageDiskTemporaryMaxAge         = 24 * time.Hour
)

var rssImageDiskMagic = [8]byte{'X', 'D', 'R', 'S', 'S', 'I', 'M', 'G'}

type rssImageDiskCacheConfig struct {
	directory    string
	maxBytes     int64
	now          func() time.Time
	asyncCleanup bool
}

type rssImageDiskCache struct {
	directory string
	maxBytes  int64
	now       func() time.Time

	mu sync.Mutex

	cleanupMu      sync.Mutex
	cleanupRunning bool
	cleanupPending bool
	asyncCleanup   bool
}

type rssImageDiskMetadata struct {
	Version     int                               `json:"version"`
	Key         string                            `json:"key"`
	ContentType string                            `json:"contentType"`
	Role        applicationrss.RemoteResourceRole `json:"role"`
	ETag        string                            `json:"etag"`
	FreshUntil  int64                             `json:"freshUntil"`
	StaleUntil  int64                             `json:"staleUntil"`
	Size        int64                             `json:"size"`
}

type rssImageDiskCleanupEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// ConfigureImageDiskCache enables process-independent persistence for already
// validated RSS images. The caller owns path selection; RSSAPI deliberately
// does not infer an OS cache directory. If configuration fails, the existing
// bounded memory cache remains usable.
func (api *RSSAPI) ConfigureImageDiskCache(directory string) error {
	if api == nil || api.imageCache == nil {
		return errors.New("RSS image cache unavailable")
	}
	disk, err := newRSSImageDiskCache(rssImageDiskCacheConfig{
		directory: strings.TrimSpace(directory), maxBytes: defaultRSSImageDiskCacheBytes,
		now: api.imageCache.now, asyncCleanup: true,
	})
	if err != nil {
		return err
	}
	api.imageCache.setDisk(disk)
	disk.scheduleCleanup()
	return nil
}

func newRSSImageDiskCache(config rssImageDiskCacheConfig) (*rssImageDiskCache, error) {
	directory := filepath.Clean(strings.TrimSpace(config.directory))
	if directory == "" || directory == "." {
		return nil, errors.New("RSS image disk cache directory is required")
	}
	if config.maxBytes <= 0 {
		config.maxBytes = defaultRSSImageDiskCacheBytes
	}
	if config.now == nil {
		config.now = func() time.Time { return time.Now().UTC() }
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create RSS image disk cache: %w", err)
	}
	// RSS images can reveal private subscription choices or contain signed
	// publisher URLs indirectly. Keep the cache private even when a parent
	// directory was created previously with broader permissions.
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure RSS image disk cache: %w", err)
	}
	return &rssImageDiskCache{
		directory:    directory,
		maxBytes:     config.maxBytes,
		now:          config.now,
		asyncCleanup: config.asyncCleanup,
	}, nil
}

func (disk *rssImageDiskCache) read(
	key string,
	role applicationrss.RemoteResourceRole,
) (rssImageCacheEntry, bool) {
	if disk == nil || !validRSSImageDiskKey(key) {
		return rssImageCacheEntry{}, false
	}
	disk.mu.Lock()
	defer disk.mu.Unlock()
	return disk.readLocked(key, normalizedRSSImageRole(role))
}

func (disk *rssImageDiskCache) readLocked(
	key string,
	role applicationrss.RemoteResourceRole,
) (rssImageCacheEntry, bool) {
	path := disk.path(key)
	file, err := os.Open(path)
	if err != nil {
		return rssImageCacheEntry{}, false
	}
	stat, err := file.Stat()
	if err != nil || stat.Size() <= int64(len(rssImageDiskMagic)+4) ||
		stat.Size() > maxRSSRemoteImageBytes+rssImageDiskMetadataMaxBytes+int64(len(rssImageDiskMagic)+4) {
		_ = file.Close()
		disk.removeCorruptLocked(path)
		return rssImageCacheEntry{}, false
	}
	metadata, data, err := readRSSImageDiskRecord(file, stat.Size())
	closeErr := file.Close()
	if err != nil || closeErr != nil || !validRSSImageDiskMetadata(metadata, key, role, stat.Size()) {
		disk.removeCorruptLocked(path)
		return rssImageCacheEntry{}, false
	}
	now := disk.now().UTC()
	staleUntil := time.Unix(0, metadata.StaleUntil).UTC()
	if !now.Before(staleUntil) {
		disk.removeCorruptLocked(path)
		return rssImageCacheEntry{}, false
	}
	// Local cache files are untrusted input: re-sniff and re-run every image
	// safety boundary before returning bytes to a WebView or memory cache.
	contentType := canonicalRasterImageMIME(data, metadata.ContentType)
	computedETag := rssImageContentETag(data)
	if contentType == "" || contentType != normalizedMIME(metadata.ContentType) ||
		!safeRSSRasterImage(data, contentType) || !safeRSSRasterImageRole(data, contentType, role) ||
		computedETag != metadata.ETag {
		disk.removeCorruptLocked(path)
		return rssImageCacheEntry{}, false
	}
	_ = os.Chtimes(path, now, now)
	return rssImageCacheEntry{
		key: key,
		image: rssCachedImage{
			data: data, contentType: contentType, etag: computedETag,
		},
		role:       role,
		freshUntil: time.Unix(0, metadata.FreshUntil).UTC(),
		staleUntil: staleUntil,
		cost:       int64(len(data)),
	}, true
}

func (disk *rssImageDiskCache) write(entry rssImageCacheEntry) error {
	if disk == nil || !validRSSImageDiskKey(entry.key) || entry.negative ||
		len(entry.image.data) == 0 || int64(len(entry.image.data)) > maxRSSRemoteImageBytes {
		return errors.New("invalid RSS image disk cache entry")
	}
	role := normalizedRSSImageRole(entry.role)
	contentType := canonicalRasterImageMIME(entry.image.data, entry.image.contentType)
	etag := rssImageContentETag(entry.image.data)
	if !validRSSImageDiskRole(role) || contentType == "" || contentType != normalizedMIME(entry.image.contentType) ||
		!safeRSSRasterImage(entry.image.data, contentType) ||
		!safeRSSRasterImageRole(entry.image.data, contentType, role) ||
		entry.image.etag != etag || entry.freshUntil.IsZero() || entry.staleUntil.IsZero() ||
		entry.freshUntil.After(entry.staleUntil) {
		return errors.New("unsafe RSS image disk cache entry")
	}
	metadata := rssImageDiskMetadata{
		Version: rssImageDiskCacheVersion, Key: entry.key, ContentType: contentType,
		Role: role, ETag: etag, FreshUntil: entry.freshUntil.UTC().UnixNano(),
		StaleUntil: entry.staleUntil.UTC().UnixNano(), Size: int64(len(entry.image.data)),
	}
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil || len(encodedMetadata) == 0 || len(encodedMetadata) > rssImageDiskMetadataMaxBytes {
		return errors.New("encode RSS image disk cache metadata")
	}

	disk.mu.Lock()
	err = disk.writeLocked(entry.key, encodedMetadata, entry.image.data)
	disk.mu.Unlock()
	if err == nil {
		disk.scheduleCleanup()
	}
	return err
}

func (disk *rssImageDiskCache) writeLocked(key string, metadata, data []byte) (returnErr error) {
	temporary, err := os.CreateTemp(disk.directory, ".rss-image-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(rssImageDiskMagic[:]); err != nil {
		return err
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(metadata)))
	if _, err := temporary.Write(length[:]); err != nil {
		return err
	}
	if _, err := temporary.Write(metadata); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	path := disk.path(key)
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	now := disk.now().UTC()
	_ = os.Chtimes(path, now, now)
	if directory, err := os.Open(disk.directory); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func (disk *rssImageDiskCache) scheduleCleanup() {
	if disk == nil || !disk.asyncCleanup {
		return
	}
	disk.cleanupMu.Lock()
	if disk.cleanupRunning {
		disk.cleanupPending = true
		disk.cleanupMu.Unlock()
		return
	}
	disk.cleanupRunning = true
	disk.cleanupMu.Unlock()
	go func() {
		for {
			_ = disk.cleanup()
			disk.cleanupMu.Lock()
			if disk.cleanupPending {
				disk.cleanupPending = false
				disk.cleanupMu.Unlock()
				continue
			}
			disk.cleanupRunning = false
			disk.cleanupMu.Unlock()
			return
		}
	}()
}

func (disk *rssImageDiskCache) cleanup() error {
	if disk == nil {
		return nil
	}
	disk.mu.Lock()
	defer disk.mu.Unlock()
	return disk.cleanupLocked(disk.now().UTC())
}

func (disk *rssImageDiskCache) cleanupLocked(now time.Time) error {
	children, err := os.ReadDir(disk.directory)
	if err != nil {
		return err
	}
	entries := make([]rssImageDiskCleanupEntry, 0, len(children))
	var total int64
	for _, child := range children {
		if child.IsDir() {
			continue
		}
		name := child.Name()
		path := filepath.Join(disk.directory, name)
		info, statErr := child.Info()
		if statErr != nil {
			continue
		}
		if strings.HasPrefix(name, ".rss-image-") {
			if now.Sub(info.ModTime()) > rssImageDiskTemporaryMaxAge {
				_ = os.Remove(path)
			}
			continue
		}
		key := strings.TrimSuffix(name, ".cache")
		if name == key || !validRSSImageDiskKey(key) {
			continue
		}
		metadata, metadataErr := readRSSImageDiskMetadataFile(path, info.Size())
		role := normalizedRSSImageRole(metadata.Role)
		if metadataErr != nil || !validRSSImageDiskMetadata(metadata, key, role, info.Size()) ||
			!now.Before(time.Unix(0, metadata.StaleUntil)) {
			_ = os.Remove(path)
			continue
		}
		entries = append(entries, rssImageDiskCleanupEntry{path: path, size: info.Size(), modTime: info.ModTime()})
		total += info.Size()
	}
	if total <= disk.maxBytes {
		return nil
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].modTime.Before(entries[right].modTime)
	})
	for _, entry := range entries {
		if total <= disk.maxBytes {
			break
		}
		if err := os.Remove(entry.path); err == nil || errors.Is(err, os.ErrNotExist) {
			total -= entry.size
		}
	}
	return nil
}

func readRSSImageDiskRecord(reader io.Reader, fileSize int64) (rssImageDiskMetadata, []byte, error) {
	var prefix [12]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil || string(prefix[:8]) != string(rssImageDiskMagic[:]) {
		return rssImageDiskMetadata{}, nil, errors.New("invalid RSS image disk cache header")
	}
	metadataSize := int64(binary.BigEndian.Uint32(prefix[8:]))
	if metadataSize <= 0 || metadataSize > rssImageDiskMetadataMaxBytes ||
		fileSize < int64(len(prefix))+metadataSize {
		return rssImageDiskMetadata{}, nil, errors.New("invalid RSS image disk cache metadata length")
	}
	encodedMetadata := make([]byte, metadataSize)
	if _, err := io.ReadFull(reader, encodedMetadata); err != nil {
		return rssImageDiskMetadata{}, nil, err
	}
	var metadata rssImageDiskMetadata
	if err := json.Unmarshal(encodedMetadata, &metadata); err != nil {
		return rssImageDiskMetadata{}, nil, err
	}
	if metadata.Size <= 0 || metadata.Size > maxRSSRemoteImageBytes ||
		fileSize != int64(len(prefix))+metadataSize+metadata.Size {
		return rssImageDiskMetadata{}, nil, errors.New("invalid RSS image disk cache size")
	}
	data := make([]byte, metadata.Size)
	if _, err := io.ReadFull(reader, data); err != nil {
		return rssImageDiskMetadata{}, nil, err
	}
	return metadata, data, nil
}

func readRSSImageDiskMetadataFile(path string, fileSize int64) (rssImageDiskMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return rssImageDiskMetadata{}, err
	}
	defer file.Close()
	var prefix [12]byte
	if _, err := io.ReadFull(file, prefix[:]); err != nil || string(prefix[:8]) != string(rssImageDiskMagic[:]) {
		return rssImageDiskMetadata{}, errors.New("invalid RSS image disk cache header")
	}
	metadataSize := int64(binary.BigEndian.Uint32(prefix[8:]))
	if metadataSize <= 0 || metadataSize > rssImageDiskMetadataMaxBytes ||
		fileSize < int64(len(prefix))+metadataSize {
		return rssImageDiskMetadata{}, errors.New("invalid RSS image disk cache metadata length")
	}
	encodedMetadata := make([]byte, metadataSize)
	if _, err := io.ReadFull(file, encodedMetadata); err != nil {
		return rssImageDiskMetadata{}, err
	}
	var metadata rssImageDiskMetadata
	if err := json.Unmarshal(encodedMetadata, &metadata); err != nil {
		return rssImageDiskMetadata{}, err
	}
	return metadata, nil
}

func validRSSImageDiskMetadata(
	metadata rssImageDiskMetadata,
	key string,
	role applicationrss.RemoteResourceRole,
	fileSize int64,
) bool {
	if metadata.Version != rssImageDiskCacheVersion || metadata.Key != key ||
		!validRSSImageDiskKey(metadata.Key) || metadata.Role != role || !validRSSImageDiskRole(role) ||
		metadata.Size <= 0 || metadata.Size > maxRSSRemoteImageBytes ||
		!allowedRasterImageMIME(normalizedMIME(metadata.ContentType)) ||
		metadata.ETag == "" || metadata.FreshUntil <= 0 || metadata.StaleUntil <= 0 ||
		metadata.FreshUntil > metadata.StaleUntil {
		return false
	}
	minimumSize := int64(len(rssImageDiskMagic) + 4 + 1)
	return fileSize >= minimumSize+metadata.Size &&
		fileSize <= minimumSize+rssImageDiskMetadataMaxBytes+metadata.Size
}

func validRSSImageDiskRole(role applicationrss.RemoteResourceRole) bool {
	switch role {
	case applicationrss.RemoteResourceRoleIcon,
		applicationrss.RemoteResourceRoleThumbnail,
		applicationrss.RemoteResourceRoleContentImage,
		applicationrss.RemoteResourceRoleMediaThumbnail:
		return true
	default:
		return false
	}
}

func validRSSImageDiskKey(key string) bool {
	if len(key) != 64 || strings.ToLower(key) != key {
		return false
	}
	decoded, err := hex.DecodeString(key)
	return err == nil && len(decoded) == 32
}

func (disk *rssImageDiskCache) path(key string) string {
	return filepath.Join(disk.directory, key+".cache")
}

func (disk *rssImageDiskCache) removeCorruptLocked(path string) {
	_ = os.Remove(path)
}
