package libraryapi

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	applicationrss "xiadown/internal/application/rss"
)

const (
	defaultRSSImageCacheEntries           = 256
	defaultRSSImageCacheBytes       int64 = 64 << 20
	defaultRSSImageCacheTTL               = 24 * time.Hour
	defaultRSSIconCacheTTL                = 7 * 24 * time.Hour
	defaultRSSImageNegativeCacheTTL       = 5 * time.Minute
	defaultRSSImageStaleRetention         = 30 * 24 * time.Hour
)

var errRSSImageNegativeCache = errors.New("RSS image is temporarily unavailable")

type rssCachedImage struct {
	data        []byte
	contentType string
	etag        string
}

type rssImageCacheConfig struct {
	maxEntries     int
	maxBytes       int64
	imageTTL       time.Duration
	iconTTL        time.Duration
	negativeTTL    time.Duration
	staleRetention time.Duration
	now            func() time.Time
}

type rssImageCache struct {
	mu sync.Mutex

	entries  map[string]*list.Element
	order    *list.List
	bytes    int64
	inFlight map[string]*rssImageCacheCall
	disk     *rssImageDiskCache

	maxEntries     int
	maxBytes       int64
	imageTTL       time.Duration
	iconTTL        time.Duration
	negativeTTL    time.Duration
	staleRetention time.Duration
	now            func() time.Time
}

type rssImageCacheEntry struct {
	key        string
	image      rssCachedImage
	role       applicationrss.RemoteResourceRole
	freshUntil time.Time
	staleUntil time.Time
	retryAfter time.Time
	negative   bool
	cost       int64
}

type rssImageCacheCall struct {
	done  chan struct{}
	image rssCachedImage
	err   error
}

func newRSSImageCache() *rssImageCache {
	return newRSSImageCacheWithConfig(rssImageCacheConfig{})
}

func newRSSImageCacheWithConfig(config rssImageCacheConfig) *rssImageCache {
	if config.maxEntries <= 0 {
		config.maxEntries = defaultRSSImageCacheEntries
	}
	if config.maxBytes <= 0 {
		config.maxBytes = defaultRSSImageCacheBytes
	}
	if config.imageTTL <= 0 {
		config.imageTTL = defaultRSSImageCacheTTL
	}
	if config.iconTTL <= 0 {
		config.iconTTL = defaultRSSIconCacheTTL
	}
	if config.negativeTTL <= 0 {
		config.negativeTTL = defaultRSSImageNegativeCacheTTL
	}
	if config.staleRetention <= 0 {
		config.staleRetention = defaultRSSImageStaleRetention
	}
	if config.now == nil {
		config.now = func() time.Time { return time.Now().UTC() }
	}
	return &rssImageCache{
		entries: make(map[string]*list.Element), order: list.New(), inFlight: make(map[string]*rssImageCacheCall),
		maxEntries: config.maxEntries, maxBytes: config.maxBytes,
		imageTTL: config.imageTTL, iconTTL: config.iconTTL,
		negativeTTL: config.negativeTTL, staleRetention: config.staleRetention,
		now: config.now,
	}
}

// get resolves a cache key from the server-derived resource descriptor, never
// from a public path or request header. The descriptor is resolved from the
// current persisted entity before this method is reached, so changing a slot's
// source URL necessarily selects a different cache entry.
func (cache *rssImageCache) get(
	ctx context.Context,
	resource applicationrss.RemoteResource,
	loader func(context.Context) (rssCachedImage, error),
) (rssCachedImage, error) {
	if loader == nil {
		return rssCachedImage{}, errors.New("RSS image loader unavailable")
	}
	if cache == nil {
		return loader(ctx)
	}
	key, ok := rssImageResourceCacheKey(resource)
	if !ok {
		return loader(ctx)
	}
	now := cache.now().UTC()

	cache.mu.Lock()
	entry, immediate, stale := cache.lookupLocked(key, now)
	if immediate {
		cache.mu.Unlock()
		if entry.negative {
			return rssCachedImage{}, errRSSImageNegativeCache
		}
		return entry.image, nil
	}
	if call := cache.inFlight[key]; call != nil {
		cache.mu.Unlock()
		select {
		case <-call.done:
			return call.image, call.err
		case <-ctx.Done():
			return rssCachedImage{}, ctx.Err()
		}
	}
	call := &rssImageCacheCall{done: make(chan struct{})}
	cache.inFlight[key] = call
	disk := cache.disk
	cache.mu.Unlock()

	// A disk lookup happens only for the singleflight leader, so concurrent
	// cold starts do not all read and validate the same file. Fresh disk bytes
	// are promoted into memory; an expired-fresh entry is retained as a
	// stale-if-error candidate without extending either persisted deadline.
	if len(stale.image.data) == 0 && disk != nil {
		if diskEntry, ok := disk.read(key, resource.Role); ok {
			now = cache.now().UTC()
			cache.mu.Lock()
			cache.storeLocked(diskEntry)
			if now.Before(diskEntry.freshUntil) {
				call.image = diskEntry.image
				delete(cache.inFlight, key)
				close(call.done)
				cache.mu.Unlock()
				return call.image, nil
			}
			stale = diskEntry
			cache.mu.Unlock()
		}
	}

	image, loadErr := loader(ctx)
	if loadErr == nil && (len(image.data) == 0 || image.contentType == "") {
		loadErr = errors.New("RSS image loader returned an empty image")
	}
	if loadErr == nil {
		role := normalizedRSSImageRole(resource.Role)
		contentType := canonicalRasterImageMIME(image.data, image.contentType)
		if contentType == "" || !safeRSSRasterImage(image.data, contentType) ||
			!safeRSSRasterImageRole(image.data, contentType, role) {
			loadErr = errors.New("RSS image loader returned an unsafe image")
		} else {
			image.contentType = contentType
			// Never persist or expose an upstream-provided validator. The local
			// validator is always derived from the bytes that passed validation.
			image.etag = rssImageContentETag(image.data)
		}
	}
	now = cache.now().UTC()
	var positive rssImageCacheEntry
	if loadErr == nil {
		positive = cache.positiveEntry(key, resource.Role, image, now)
		if disk != nil {
			// Persistence is an optimization. A read-only/full cache directory must
			// never turn an otherwise-valid RSS image into a failed response.
			_ = disk.write(positive)
		}
	}
	cache.mu.Lock()
	switch {
	case loadErr == nil && len(image.data) > 0 && image.contentType != "":
		cache.storeLocked(positive)
		call.image = image
	case len(stale.image.data) > 0 && now.Before(stale.staleUntil):
		// Stale-if-error prevents a temporary publisher or proxy failure from
		// replacing an already-validated image with a broken-image surface. The
		// short retry gate also prevents repeated failed upstream requests.
		stale.retryAfter = now.Add(cache.negativeTTL)
		cache.storeLocked(stale)
		call.image = stale.image
	case rssImageErrorCanBeNegativeCached(loadErr):
		cache.storeLocked(rssImageCacheEntry{
			key: key, role: normalizedRSSImageRole(resource.Role), negative: true,
			retryAfter: now.Add(cache.negativeTTL), staleUntil: now.Add(cache.negativeTTL),
		})
		call.err = loadErr
	default:
		call.err = loadErr
	}
	delete(cache.inFlight, key)
	close(call.done)
	cache.mu.Unlock()
	return call.image, call.err
}

func (cache *rssImageCache) setDisk(disk *rssImageDiskCache) {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	cache.disk = disk
	cache.mu.Unlock()
}

func (cache *rssImageCache) lookupLocked(
	key string,
	now time.Time,
) (rssImageCacheEntry, bool, rssImageCacheEntry) {
	element := cache.entries[key]
	if element == nil {
		return rssImageCacheEntry{}, false, rssImageCacheEntry{}
	}
	cache.order.MoveToFront(element)
	entry := *(element.Value.(*rssImageCacheEntry))
	if entry.negative {
		if now.Before(entry.retryAfter) {
			return entry, true, rssImageCacheEntry{}
		}
		cache.removeElementLocked(element)
		return rssImageCacheEntry{}, false, rssImageCacheEntry{}
	}
	if now.Before(entry.freshUntil) {
		return entry, true, rssImageCacheEntry{}
	}
	if !now.Before(entry.staleUntil) {
		cache.removeElementLocked(element)
		return rssImageCacheEntry{}, false, rssImageCacheEntry{}
	}
	if now.Before(entry.retryAfter) {
		return entry, true, entry
	}
	return rssImageCacheEntry{}, false, entry
}

func (cache *rssImageCache) positiveEntry(
	key string,
	role applicationrss.RemoteResourceRole,
	image rssCachedImage,
	now time.Time,
) rssImageCacheEntry {
	role = normalizedRSSImageRole(role)
	ttl := cache.imageTTL
	if role == applicationrss.RemoteResourceRoleIcon {
		ttl = cache.iconTTL
	}
	return rssImageCacheEntry{
		key: key, image: image, role: role,
		freshUntil: now.Add(ttl), staleUntil: now.Add(cache.staleRetention),
		cost: int64(len(image.data)),
	}
}

func (cache *rssImageCache) storeLocked(entry rssImageCacheEntry) {
	if existing := cache.entries[entry.key]; existing != nil {
		current := existing.Value.(*rssImageCacheEntry)
		cache.bytes -= current.cost
		*current = entry
		cache.bytes += entry.cost
		cache.order.MoveToFront(existing)
	} else {
		copy := entry
		element := cache.order.PushFront(&copy)
		cache.entries[entry.key] = element
		cache.bytes += entry.cost
	}
	cache.evictLocked()
}

func (cache *rssImageCache) evictLocked() {
	for len(cache.entries) > cache.maxEntries || cache.bytes > cache.maxBytes {
		element := cache.order.Back()
		if element == nil {
			return
		}
		cache.removeElementLocked(element)
	}
}

func (cache *rssImageCache) removeElementLocked(element *list.Element) {
	if element == nil {
		return
	}
	entry, _ := element.Value.(*rssImageCacheEntry)
	if entry != nil {
		delete(cache.entries, entry.key)
		cache.bytes -= entry.cost
	}
	cache.order.Remove(element)
}

func rssImageResourceCacheKey(resource applicationrss.RemoteResource) (string, bool) {
	canonical := canonicalRSSImageCacheURL(resource.URL)
	if canonical == "" {
		return "", false
	}
	role := normalizedRSSImageRole(resource.Role)
	material := "rss-image-cache-v1\x00" + canonical + "\x00" + resource.SafeRefererOrigin() + "\x00" + string(role)
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:]), true
}

func canonicalRSSImageCacheURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || parsed.Opaque != "" || parsed.User != nil {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	hostname := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if hostname == "" {
		return ""
	}
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	parsed.Host = host
	parsed.Fragment = ""
	parsed.RawFragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	if parsed.RawQuery == "" {
		parsed.ForceQuery = false
	}
	return parsed.String()
}

func normalizedRSSImageRole(role applicationrss.RemoteResourceRole) applicationrss.RemoteResourceRole {
	if role == "" {
		return applicationrss.RemoteResourceRoleContentImage
	}
	return role
}

func rssImageContentETag(data []byte) string {
	sum := sha256.Sum256(data)
	return `"rss-` + hex.EncodeToString(sum[:]) + `"`
}

func rssImageErrorCanBeNegativeCached(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var policy interface{ rssImageNegativeCacheable() bool }
	if errors.As(err, &policy) {
		return policy.rssImageNegativeCacheable()
	}
	return true
}
