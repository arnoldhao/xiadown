package youtubemusic

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gabriel-vasile/mimetype"
)

const (
	defaultImageCacheMemoryCountLimit = 200
	defaultImageCacheMemoryByteLimit  = 50 * 1024 * 1024
	defaultImageCacheDiskByteLimit    = 200 * 1024 * 1024
	defaultImageCacheFetchTimeout     = 20 * time.Second
	youtubeWebImageOrigin             = "https://www.youtube.com"
)

var (
	ErrImageURLInvalid  = errors.New("image url invalid")
	ErrImageUnavailable = errors.New("image unavailable")
)

type ImageHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type ImageCacheConfig struct {
	Directory        string
	MemoryCountLimit int
	MemoryByteLimit  int64
	DiskByteLimit    int64
	FetchTimeout     time.Duration
}

type ImageCache struct {
	clientProvider ImageHTTPClientProvider
	directory      string
	memoryMaxCount int
	memoryMaxBytes int64
	diskMaxBytes   int64
	fetchTimeout   time.Duration

	mu          sync.Mutex
	memory      map[string]*list.Element
	memoryOrder *list.List
	memoryBytes int64
	inFlight    map[string]*imageFetchCall
}

type ImageResult struct {
	URL         string
	CacheKey    string
	Data        []byte
	ContentType string
	ModTime     time.Time
}

type imageCacheEntry struct {
	key         string
	url         string
	data        []byte
	contentType string
	modTime     time.Time
	cost        int64
}

type imageFetchCall struct {
	done   chan struct{}
	result ImageResult
	err    error
}

type staticImageHTTPClientProvider struct {
	client *http.Client
}

func (provider staticImageHTTPClientProvider) HTTPClient() *http.Client {
	return provider.client
}

func DefaultDreamFMImageCacheDir() (string, error) {
	baseDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "xiadown", "com.xiadown.dreamfm.imagecache"), nil
}

func NewImageCache(provider ImageHTTPClientProvider, config ImageCacheConfig) (*ImageCache, error) {
	if provider == nil {
		provider = staticImageHTTPClientProvider{client: &http.Client{Timeout: defaultImageCacheFetchTimeout}}
	}
	directory := strings.TrimSpace(config.Directory)
	if directory == "" {
		var err error
		directory, err = DefaultDreamFMImageCacheDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	memoryCountLimit := config.MemoryCountLimit
	if memoryCountLimit <= 0 {
		memoryCountLimit = defaultImageCacheMemoryCountLimit
	}
	memoryByteLimit := config.MemoryByteLimit
	if memoryByteLimit <= 0 {
		memoryByteLimit = defaultImageCacheMemoryByteLimit
	}
	diskByteLimit := config.DiskByteLimit
	if diskByteLimit <= 0 {
		diskByteLimit = defaultImageCacheDiskByteLimit
	}
	fetchTimeout := config.FetchTimeout
	if fetchTimeout <= 0 {
		fetchTimeout = defaultImageCacheFetchTimeout
	}
	cache := &ImageCache{
		clientProvider: provider,
		directory:      directory,
		memoryMaxCount: memoryCountLimit,
		memoryMaxBytes: memoryByteLimit,
		diskMaxBytes:   diskByteLimit,
		fetchTimeout:   fetchTimeout,
		memory:         make(map[string]*list.Element),
		memoryOrder:    list.New(),
		inFlight:       make(map[string]*imageFetchCall),
	}
	go cache.evictDiskCacheIfNeeded()
	return cache, nil
}

func (cache *ImageCache) Image(ctx context.Context, rawURL string) (ImageResult, error) {
	normalizedURL, err := normalizeImageCacheURL(rawURL)
	if err != nil {
		return ImageResult{}, err
	}
	key := imageCacheKey(normalizedURL)
	if result, ok := cache.memoryResult(key); ok {
		return result, nil
	}
	if result, ok := cache.diskResult(normalizedURL, key); ok {
		cache.storeMemory(result)
		return result, nil
	}

	call, leader := cache.beginFetch(key)
	if !leader {
		select {
		case <-call.done:
			return call.result, call.err
		case <-ctx.Done():
			return ImageResult{}, ctx.Err()
		}
	}

	result, err := cache.fetchAndStore(ctx, normalizedURL, key)
	cache.completeFetch(key, call, result, err)
	return result, err
}

func (cache *ImageCache) ClearMemory() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.memory = make(map[string]*list.Element)
	cache.memoryOrder.Init()
	cache.memoryBytes = 0
}

func (cache *ImageCache) ClearAll() error {
	cache.ClearMemory()
	if err := os.RemoveAll(cache.directory); err != nil {
		return err
	}
	return os.MkdirAll(cache.directory, 0o755)
}

func (cache *ImageCache) DiskCacheSize() int64 {
	entries, err := cache.diskEntries()
	if err != nil {
		return 0
	}
	var total int64
	for _, entry := range entries {
		total += entry.size
	}
	return total
}

func (cache *ImageCache) memoryResult(key string) (ImageResult, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	element := cache.memory[key]
	if element == nil {
		return ImageResult{}, false
	}
	cache.memoryOrder.MoveToFront(element)
	entry := element.Value.(*imageCacheEntry)
	return ImageResult{
		URL:         entry.url,
		CacheKey:    entry.key,
		Data:        entry.data,
		ContentType: entry.contentType,
		ModTime:     entry.modTime,
	}, true
}

func (cache *ImageCache) storeMemory(result ImageResult) {
	if len(result.Data) == 0 {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if element := cache.memory[result.CacheKey]; element != nil {
		entry := element.Value.(*imageCacheEntry)
		cache.memoryBytes -= entry.cost
		entry.url = result.URL
		entry.data = result.Data
		entry.contentType = result.ContentType
		entry.modTime = result.ModTime
		entry.cost = int64(len(result.Data))
		cache.memoryBytes += entry.cost
		cache.memoryOrder.MoveToFront(element)
		cache.evictMemoryLocked()
		return
	}
	entry := &imageCacheEntry{
		key:         result.CacheKey,
		url:         result.URL,
		data:        result.Data,
		contentType: result.ContentType,
		modTime:     result.ModTime,
		cost:        int64(len(result.Data)),
	}
	element := cache.memoryOrder.PushFront(entry)
	cache.memory[result.CacheKey] = element
	cache.memoryBytes += entry.cost
	cache.evictMemoryLocked()
}

func (cache *ImageCache) evictMemoryLocked() {
	for (cache.memoryMaxCount > 0 && len(cache.memory) > cache.memoryMaxCount) ||
		(cache.memoryMaxBytes > 0 && cache.memoryBytes > cache.memoryMaxBytes) {
		element := cache.memoryOrder.Back()
		if element == nil {
			return
		}
		entry := element.Value.(*imageCacheEntry)
		delete(cache.memory, entry.key)
		cache.memoryBytes -= entry.cost
		cache.memoryOrder.Remove(element)
	}
}

func (cache *ImageCache) beginFetch(key string) (*imageFetchCall, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if call := cache.inFlight[key]; call != nil {
		return call, false
	}
	call := &imageFetchCall{done: make(chan struct{})}
	cache.inFlight[key] = call
	return call, true
}

func (cache *ImageCache) completeFetch(key string, call *imageFetchCall, result ImageResult, err error) {
	cache.mu.Lock()
	if cache.inFlight[key] == call {
		delete(cache.inFlight, key)
	}
	call.result = result
	call.err = err
	close(call.done)
	cache.mu.Unlock()
}

func (cache *ImageCache) diskResult(rawURL string, key string) (ImageResult, bool) {
	path := cache.diskPath(key)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ImageResult{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return ImageResult{}, false
	}
	contentType := imageContentType(data, "")
	if contentType == "" {
		_ = os.Remove(path)
		return ImageResult{}, false
	}
	return ImageResult{
		URL:         rawURL,
		CacheKey:    key,
		Data:        data,
		ContentType: contentType,
		ModTime:     info.ModTime(),
	}, true
}

func (cache *ImageCache) fetchAndStore(ctx context.Context, rawURL string, key string) (ImageResult, error) {
	if result, ok := cache.memoryResult(key); ok {
		return result, nil
	}
	if result, ok := cache.diskResult(rawURL, key); ok {
		cache.storeMemory(result)
		return result, nil
	}

	fetchCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && cache.fetchTimeout > 0 {
		fetchCtx, cancel = context.WithTimeout(ctx, cache.fetchTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ImageResult{}, err
	}
	req.Header.Set("User-Agent", BrowserUserAgent)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if imageOrigin := youtubeImageRequestOrigin(req.URL.Hostname()); imageOrigin != "" {
		req.Header.Set("Referer", imageOrigin+"/")
		req.Header.Set("Origin", imageOrigin)
	}

	client := cache.clientProvider.HTTPClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return ImageResult{}, fmt.Errorf("%w: %v", ErrImageUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return ImageResult{}, fmt.Errorf("%w: status %d", ErrImageUnavailable, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageResult{}, fmt.Errorf("%w: %v", ErrImageUnavailable, err)
	}
	if len(data) == 0 {
		return ImageResult{}, fmt.Errorf("%w: empty image content", ErrImageUnavailable)
	}
	contentType := imageContentType(data, resp.Header.Get("Content-Type"))
	if contentType == "" {
		return ImageResult{}, fmt.Errorf("%w: invalid image content", ErrImageUnavailable)
	}
	result := ImageResult{
		URL:         rawURL,
		CacheKey:    key,
		Data:        data,
		ContentType: contentType,
		ModTime:     time.Now(),
	}
	cache.storeMemory(result)
	cache.saveToDisk(result)
	return result, nil
}

func (cache *ImageCache) saveToDisk(result ImageResult) {
	if len(result.Data) == 0 {
		return
	}
	path := cache.diskPath(result.CacheKey)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, result.Data, 0o644); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return
	}
	go cache.evictDiskCacheIfNeeded()
}

func (cache *ImageCache) diskPath(key string) string {
	return filepath.Join(cache.directory, key)
}

type imageDiskEntry struct {
	path    string
	modTime time.Time
	size    int64
}

func (cache *ImageCache) diskEntries() ([]imageDiskEntry, error) {
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		return nil, err
	}
	result := make([]imageDiskEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, imageDiskEntry{
			path:    filepath.Join(cache.directory, entry.Name()),
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}
	return result, nil
}

func (cache *ImageCache) evictDiskCacheIfNeeded() {
	entries, err := cache.diskEntries()
	if err != nil {
		return
	}
	var total int64
	for _, entry := range entries {
		total += entry.size
	}
	if total <= cache.diskMaxBytes {
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modTime.Before(entries[j].modTime)
	})
	sizeToFree := total - cache.diskMaxBytes
	for _, entry := range entries {
		if sizeToFree <= 0 {
			return
		}
		if err := os.Remove(entry.path); err == nil {
			sizeToFree -= entry.size
		}
	}
}

func normalizeImageCacheURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", ErrImageURLInvalid
	}
	if strings.HasPrefix(trimmed, "//") {
		trimmed = "https:" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil {
		return "", ErrImageURLInvalid
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrImageURLInvalid
	}
	if parsed.Host == "" {
		return "", ErrImageURLInvalid
	}
	return parsed.String(), nil
}

func imageCacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}

func imageContentType(data []byte, responseContentType string) string {
	if len(data) == 0 {
		return ""
	}
	detectedContentType := mimetype.Detect(data).String()
	if mediaType := normalizedImageContentType(detectedContentType); mediaType != "" {
		return mediaType
	}
	if mediaType := normalizedImageContentType(responseContentType); mediaType != "" && !looksLikeTextPayload(data, detectedContentType) {
		return mediaType
	}
	return ""
}

func normalizedImageContentType(value string) string {
	mediaType := strings.TrimSpace(strings.Split(value, ";")[0])
	if mediaType == "" {
		return ""
	}
	mediaType = strings.ToLower(mediaType)
	if !strings.HasPrefix(mediaType, "image/") {
		return ""
	}
	return mediaType
}

func looksLikeTextPayload(data []byte, detectedContentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(detectedContentType, ";")[0]))
	if strings.HasPrefix(mediaType, "text/") ||
		mediaType == "application/json" ||
		mediaType == "application/xml" {
		return true
	}
	sample := strings.TrimSpace(string(data[:min(len(data), 512)]))
	if sample == "" {
		return false
	}
	lowerSample := strings.ToLower(sample)
	return strings.HasPrefix(lowerSample, "<!doctype html") ||
		strings.HasPrefix(lowerSample, "<html") ||
		strings.HasPrefix(lowerSample, "{") ||
		strings.HasPrefix(lowerSample, "[")
}

func youtubeImageRequestOrigin(host string) string {
	normalized := strings.ToLower(strings.TrimSpace(host))
	if strings.HasSuffix(normalized, "ytimg.com") ||
		strings.HasSuffix(normalized, ".ytimg.com") {
		return youtubeWebImageOrigin
	}
	if strings.HasSuffix(normalized, "googleusercontent.com") ||
		strings.HasSuffix(normalized, ".googleusercontent.com") {
		return origin
	}
	return ""
}
