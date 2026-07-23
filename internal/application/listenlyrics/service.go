package listenlyrics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	KindSynced      = "synced"
	KindPlain       = "plain"
	KindUnavailable = "unavailable"

	TimingQualityPlain     = "plain"
	TimingQualityLine      = "line"
	TimingQualityWord      = "word"
	TimingQualitySyllable  = "syllable"
	TimingQualityEstimated = "estimated"

	ErrorCodeRequestCancelled    = "request_cancelled"
	ErrorCodeTimeout             = "lyrics_timeout"
	ErrorCodeNetworkUnavailable  = "lyrics_network_unavailable"
	ErrorCodeAuthRequired        = "lyrics_auth_required"
	ErrorCodeRateLimited         = "lyrics_rate_limited"
	ErrorCodeProviderUnavailable = "lyrics_provider_unavailable"
	ErrorCodeUnavailable         = "lyrics_unavailable"

	cacheRetentionTTL   = 24 * time.Hour
	plainFreshTTL       = 5 * time.Minute
	unavailableFreshTTL = 2 * time.Minute
	localFreshTTL       = time.Minute
	cacheMaxEntries     = 120
	maxSearchVariants   = 3
)

type Client interface {
	TrackLyrics(ctx context.Context, request Request) (Snapshot, error)
}

type Request struct {
	Key             string
	VideoID         string
	Title           string
	Artist          string
	Album           string
	LocalPath       string
	DurationSeconds float64
	PlainOnly       bool
	Language        string
	SearchVariants  []SearchVariant
}

type SearchVariant struct {
	Title  string `json:"title,omitempty"`
	Artist string `json:"artist,omitempty"`
}

type Snapshot struct {
	VideoID         string `json:"videoId,omitempty"`
	Kind            string `json:"kind,omitempty"`
	Source          string `json:"source,omitempty"`
	ProviderID      string `json:"providerId,omitempty"`
	ProviderTrackID string `json:"providerTrackId,omitempty"`
	Attribution     string `json:"attribution,omitempty"`
	TimingQuality   string `json:"timingQuality,omitempty"`
	Confidence      int    `json:"confidence,omitempty"`
	Text            string `json:"text,omitempty"`
	Lines           []Line `json:"lines,omitempty"`
	Loading         bool   `json:"loading,omitempty"`
	Error           string `json:"error,omitempty"`
	ErrorCode       string `json:"errorCode,omitempty"`
	Retryable       bool   `json:"retryable,omitempty"`
	ActiveProvider  string `json:"activeProvider,omitempty"`
}

type Line struct {
	StartMs         int             `json:"startMs"`
	DurationMs      int             `json:"durationMs"`
	EndEstimated    bool            `json:"endEstimated,omitempty"`
	Text            string          `json:"text"`
	TranslationText string          `json:"translationText,omitempty"`
	RomanizedText   string          `json:"romanizedText,omitempty"`
	RomanizedKind   string          `json:"romanizedKind,omitempty"`
	AlternateTexts  []AlternateText `json:"alternateTexts,omitempty"`
	Words           []Word          `json:"words,omitempty"`
}

type Word struct {
	StartMs       int    `json:"startMs"`
	EndMs         int    `json:"endMs,omitempty"`
	Text          string `json:"text"`
	EndsWithSpace *bool  `json:"endsWithSpace,omitempty"`
	Syllables     []Word `json:"syllables,omitempty"`
}

type AlternateText struct {
	Role     string `json:"role"`
	Language string `json:"language,omitempty"`
	Text     string `json:"text"`
}

type Service struct {
	mu sync.Mutex

	client Client
	now    func() time.Time

	current    Snapshot
	generation uint64
	keyLatest  map[string]keyRequestVersion
	cache      map[string]cacheEntry
}

type keyRequestVersion struct {
	generation uint64
	identity   string
}

type cacheEntry struct {
	identity   string
	snapshot   Snapshot
	updatedAt  time.Time
	lastAccess time.Time
}

func NewService(client Client) *Service {
	return &Service{
		client:    client,
		now:       time.Now,
		keyLatest: make(map[string]keyRequestVersion),
		cache:     make(map[string]cacheEntry),
	}
}

func (service *Service) Current() Snapshot {
	if service == nil {
		return Snapshot{Kind: KindUnavailable}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return normalizeSnapshot(cloneSnapshot(service.current))
}

func (service *Service) TrackLyrics(ctx context.Context, request Request) (Snapshot, error) {
	if service == nil {
		return Snapshot{}, fmt.Errorf("listen lyrics service unavailable")
	}
	request = normalizeRequest(request)
	if request.Key == "" && request.VideoID == "" && request.Title == "" {
		return Snapshot{Kind: KindUnavailable}, nil
	}

	key := cacheKey(request)
	requestKey := lyricsRequestCacheKey(request)
	identity := cacheTrackIdentityKey(request)
	service.mu.Lock()
	service.generation++
	generation := service.generation
	service.keyLatest[requestKey] = keyRequestVersion{generation: generation, identity: identity}
	now := service.currentTime()
	cached, cachedAge, hasCached := service.cachedLocked(key, now)
	if hasCached && cached.Kind != KindUnavailable && cacheEntryFresh(cached, cachedAge) {
		delete(service.keyLatest, requestKey)
		service.current = cloneSnapshot(cached)
		service.mu.Unlock()
		return cloneSnapshot(cached), nil
	}
	cachedUnavailable, cachedUnavailableAge, hasCachedUnavailable := service.cachedLocked(requestKey, now)
	if hasCachedUnavailable && cachedUnavailable.Kind == KindUnavailable && cacheEntryFresh(cachedUnavailable, cachedUnavailableAge) {
		delete(service.keyLatest, requestKey)
		service.current = cloneSnapshot(cachedUnavailable)
		service.mu.Unlock()
		return cloneSnapshot(cachedUnavailable), nil
	}
	if hasCached {
		service.current = cloneSnapshot(cached)
		service.current.Loading = true
	} else {
		service.current = Snapshot{
			VideoID: requestIdentityID(request),
			Loading: true,
		}
	}
	service.mu.Unlock()

	var result Snapshot
	var err error
	if service.client == nil {
		result = Snapshot{VideoID: requestIdentityID(request), Kind: KindUnavailable}
	} else {
		result, err = service.client.TrackLyrics(ctx, request)
	}
	result = normalizeSnapshot(result)
	if result.VideoID == "" {
		result.VideoID = requestIdentityID(request)
	}
	if err != nil {
		if hasCached && cached.Kind == KindPlain {
			result = cloneSnapshot(cached)
			err = nil
		} else {
			errorCode, retryable := classifyLyricsError(err)
			result = Snapshot{
				VideoID:   requestIdentityID(request),
				Kind:      KindUnavailable,
				ErrorCode: errorCode,
				Retryable: retryable,
			}
		}
	} else if !snapshotAvailable(result) && hasCached && cached.Kind == KindPlain && !isLocalSnapshot(cached) {
		result = cloneSnapshot(cached)
	} else if !isLocalSnapshot(cached) && rank(cached.Kind) > rank(result.Kind) {
		result = cloneSnapshot(cached)
	}

	service.mu.Lock()
	defer service.mu.Unlock()
	now = service.currentTime()
	result = normalizeSnapshot(result)
	result.Loading = false
	if result.ActiveProvider == "" {
		result.ActiveProvider = result.Source
	}
	isLatestForKey := service.keyLatest[requestKey].generation == generation
	if isLatestForKey {
		delete(service.keyLatest, requestKey)
	}
	if isLatestForKey && (err == nil || result.Kind != KindUnavailable) {
		storeKey := key
		if result.Kind == KindUnavailable {
			storeKey = requestKey
		}
		service.storeCacheLocked(storeKey, identity, result, now)
	}
	if generation != service.generation {
		return cloneSnapshot(result), err
	}
	service.current = cloneSnapshot(result)
	return cloneSnapshot(result), err
}

func (service *Service) currentTime() time.Time {
	if service != nil && service.now != nil {
		return service.now()
	}
	return time.Now()
}

func (service *Service) cachedLocked(key string, now time.Time) (Snapshot, time.Duration, bool) {
	entry, ok := service.cache[key]
	if !ok {
		return Snapshot{}, 0, false
	}
	age := now.Sub(entry.updatedAt)
	if age > cacheRetentionTTL {
		delete(service.cache, key)
		return Snapshot{}, 0, false
	}
	if age < 0 {
		age = 0
	}
	entry.lastAccess = now
	service.cache[key] = entry
	return cloneSnapshot(entry.snapshot), age, true
}

func (service *Service) storeCacheLocked(key string, identity string, snapshot Snapshot, now time.Time) {
	if key == "" || (snapshot.Kind != KindSynced && snapshot.Kind != KindPlain && snapshot.Kind != KindUnavailable) {
		return
	}
	if existing, ok := service.cache[key]; ok && !isLocalSnapshot(existing.snapshot) && rank(existing.snapshot.Kind) > rank(snapshot.Kind) {
		existing.lastAccess = now
		service.cache[key] = existing
		return
	}
	service.cache[key] = cacheEntry{
		identity:   identity,
		snapshot:   cloneSnapshot(snapshot),
		updatedAt:  now,
		lastAccess: now,
	}
	service.evictCacheLocked(now)
}

func (service *Service) evictCacheLocked(now time.Time) {
	for key, entry := range service.cache {
		if now.Sub(entry.updatedAt) > cacheRetentionTTL {
			delete(service.cache, key)
		}
	}
	for len(service.cache) > cacheMaxEntries {
		var oldestKey string
		var oldestAccess time.Time
		for key, entry := range service.cache {
			if oldestKey == "" || entry.lastAccess.Before(oldestAccess) {
				oldestKey = key
				oldestAccess = entry.lastAccess
			}
		}
		if oldestKey == "" {
			return
		}
		delete(service.cache, oldestKey)
	}
}

func cacheEntryFresh(snapshot Snapshot, age time.Duration) bool {
	if age < 0 {
		age = 0
	}
	if isLocalSnapshot(snapshot) {
		return age <= localFreshTTL
	}
	switch snapshot.Kind {
	case KindSynced:
		return age <= cacheRetentionTTL
	case KindPlain:
		return age <= plainFreshTTL
	case KindUnavailable:
		return age <= unavailableFreshTTL
	default:
		return false
	}
}

func isLocalSnapshot(snapshot Snapshot) bool {
	return strings.HasPrefix(snapshot.ProviderID, "local_") || strings.HasPrefix(snapshot.VideoID, "local:")
}

func normalizeRequest(request Request) Request {
	request.Key = strings.TrimSpace(request.Key)
	request.VideoID = strings.TrimSpace(request.VideoID)
	request.Title = strings.TrimSpace(request.Title)
	request.Artist = strings.TrimSpace(request.Artist)
	request.Album = strings.TrimSpace(request.Album)
	request.LocalPath = strings.TrimSpace(request.LocalPath)
	request.Language = strings.TrimSpace(request.Language)
	request.SearchVariants = normalizeSearchVariants(request.SearchVariants, request.Title, request.Artist)
	if math.IsNaN(request.DurationSeconds) || math.IsInf(request.DurationSeconds, 0) || request.DurationSeconds < 0 {
		request.DurationSeconds = 0
	}
	return request
}

func normalizeSearchVariants(variants []SearchVariant, canonicalTitle string, canonicalArtist string) []SearchVariant {
	if len(variants) == 0 {
		return nil
	}
	canonicalKey := searchVariantKey(canonicalTitle, canonicalArtist)
	seen := map[string]struct{}{canonicalKey: {}}
	result := make([]SearchVariant, 0, min(len(variants), maxSearchVariants))
	for _, variant := range variants {
		variant.Title = normalizeSearchVariantText(variant.Title)
		variant.Artist = normalizeSearchVariantText(variant.Artist)
		if variant.Title == "" && variant.Artist == "" {
			continue
		}
		effectiveTitle := variant.Title
		if effectiveTitle == "" {
			effectiveTitle = canonicalTitle
		}
		effectiveArtist := variant.Artist
		if effectiveArtist == "" {
			effectiveArtist = canonicalArtist
		}
		key := searchVariantKey(effectiveTitle, effectiveArtist)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, variant)
		if len(result) == maxSearchVariants {
			break
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeSearchVariantText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func searchVariantKey(title string, artist string) string {
	return strings.ToLower(normalizeSearchVariantText(title)) + "\x00" +
		strings.ToLower(normalizeSearchVariantText(artist))
}

func requestIdentityID(request Request) string {
	if request.VideoID != "" {
		return request.VideoID
	}
	return request.Key
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	snapshot.VideoID = strings.TrimSpace(snapshot.VideoID)
	snapshot.Kind = strings.TrimSpace(snapshot.Kind)
	snapshot.Source = strings.TrimSpace(snapshot.Source)
	snapshot.ProviderID = strings.TrimSpace(snapshot.ProviderID)
	snapshot.ProviderTrackID = strings.TrimSpace(snapshot.ProviderTrackID)
	snapshot.Attribution = strings.TrimSpace(snapshot.Attribution)
	snapshot.TimingQuality = normalizeTimingQuality(snapshot.TimingQuality, snapshot)
	if snapshot.Confidence < 0 {
		snapshot.Confidence = 0
	} else if snapshot.Confidence > 100 {
		snapshot.Confidence = 100
	}
	snapshot.Error = strings.TrimSpace(snapshot.Error)
	snapshot.ErrorCode = strings.TrimSpace(snapshot.ErrorCode)
	snapshot.ActiveProvider = strings.TrimSpace(snapshot.ActiveProvider)
	if snapshot.Kind != KindSynced && snapshot.Kind != KindPlain && snapshot.Kind != KindUnavailable {
		snapshot.Kind = KindUnavailable
	}
	snapshot.Lines = cloneLines(snapshot.Lines)
	return snapshot
}

func classifyLyricsError(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	if errors.Is(err, context.Canceled) {
		return ErrorCodeRequestCancelled, false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorCodeTimeout, true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, marker := range []string{"timed out", "timeout", "deadline exceeded"} {
		if strings.Contains(lower, marker) {
			return ErrorCodeTimeout, true
		}
	}
	if status := lyricsErrorHTTPStatus(err, lower); status != 0 {
		switch status {
		case 408:
			return ErrorCodeTimeout, true
		case 429:
			return ErrorCodeRateLimited, true
		case 500, 502, 503, 504:
			return ErrorCodeProviderUnavailable, true
		}
	}
	for _, marker := range []string{"too many requests", "rate limit", "rate-limit"} {
		if strings.Contains(lower, marker) {
			return ErrorCodeRateLimited, true
		}
	}
	for _, marker := range []string{"bad gateway", "service unavailable", "gateway timeout"} {
		if strings.Contains(lower, marker) {
			return ErrorCodeProviderUnavailable, true
		}
	}
	if strings.Contains(lower, "lrclib response invalid") {
		return ErrorCodeProviderUnavailable, true
	}
	for _, marker := range []string{"network unavailable", "no such host", "network is unreachable", "connection refused", "connection reset", "dial tcp", "unexpected eof"} {
		if strings.Contains(lower, marker) {
			return ErrorCodeNetworkUnavailable, true
		}
	}
	for _, marker := range []string{"not authenticated", "auth expired", "cookies are missing", "no cookies"} {
		if strings.Contains(lower, marker) {
			return ErrorCodeAuthRequired, false
		}
	}
	return ErrorCodeUnavailable, false
}

type lyricsHTTPStatusError interface {
	HTTPStatusCode() int
}

func lyricsErrorHTTPStatus(err error, lower string) int {
	for _, status := range []int{408, 429, 500, 502, 503, 504} {
		if lyricsErrorContainsHTTPStatus(err, status) {
			return status
		}
		value := strconv.Itoa(status)
		for _, prefix := range []string{"status ", "status=", "http ", "http status "} {
			if strings.Contains(lower, prefix+value) {
				return status
			}
		}
	}
	return 0
}

func lyricsErrorContainsHTTPStatus(err error, status int) bool {
	if err == nil {
		return false
	}
	if statusErr, ok := err.(lyricsHTTPStatusError); ok && statusErr.HTTPStatusCode() == status {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, nested := range joined.Unwrap() {
			if lyricsErrorContainsHTTPStatus(nested, status) {
				return true
			}
		}
		return false
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return lyricsErrorContainsHTTPStatus(wrapped.Unwrap(), status)
	}
	return false
}

func normalizeTimingQuality(value string, snapshot Snapshot) string {
	switch strings.TrimSpace(value) {
	case TimingQualityPlain, TimingQualityLine, TimingQualityWord, TimingQualitySyllable, TimingQualityEstimated:
		return strings.TrimSpace(value)
	}
	if snapshot.Kind == KindPlain {
		return TimingQualityPlain
	}
	if snapshot.Kind != KindSynced {
		return ""
	}
	for _, line := range snapshot.Lines {
		if len(line.Words) > 0 {
			return TimingQualityWord
		}
	}
	return TimingQualityLine
}

func cacheKey(request Request) string {
	mode := "synced"
	if request.PlainOnly {
		mode = "plain"
	}
	if request.VideoID != "" {
		return strings.Join([]string{request.Language, mode, "video", request.VideoID}, "\x00")
	}
	if request.Key != "" {
		parts := []string{
			request.Language,
			mode,
			"key",
			request.Key,
			strings.ToLower(request.Title),
			strings.ToLower(request.Artist),
			strings.ToLower(request.Album),
		}
		if request.DurationSeconds > 0 {
			parts = append(parts, strconv.Itoa(int(math.Round(request.DurationSeconds))))
		}
		return strings.Join(parts, "\x00")
	}
	parts := []string{
		request.Language,
		mode,
		"title",
		strings.ToLower(request.Title),
		strings.ToLower(request.Artist),
		strings.ToLower(request.Album),
	}
	if request.DurationSeconds > 0 {
		parts = append(parts, strconv.Itoa(int(math.Round(request.DurationSeconds))))
	}
	return strings.Join(parts, "\x00")
}

// lyricsRequestCacheKey scopes negative results and same-query request
// ordering to every input that can change provider fallback matching. Positive
// lyrics remain shared through cacheKey because search variants are alternate
// spellings of the same canonical track identity.
func lyricsRequestCacheKey(request Request) string {
	duration := ""
	if request.DurationSeconds > 0 {
		duration = strconv.Itoa(int(math.Round(request.DurationSeconds)))
	}
	parts := []string{
		cacheKey(request),
		"query",
		strings.ToLower(normalizeSearchVariantText(request.Title)),
		strings.ToLower(normalizeSearchVariantText(request.Artist)),
		strings.ToLower(normalizeSearchVariantText(request.Album)),
		request.LocalPath,
		duration,
		strconv.Itoa(len(request.SearchVariants)),
	}
	for _, variant := range request.SearchVariants {
		title := variant.Title
		if title == "" {
			title = request.Title
		}
		artist := variant.Artist
		if artist == "" {
			artist = request.Artist
		}
		parts = append(parts,
			strings.ToLower(normalizeSearchVariantText(title)),
			strings.ToLower(normalizeSearchVariantText(artist)),
		)
	}
	return strings.Join(parts, "\x00")
}

func cacheTrackIdentityKey(request Request) string {
	if request.VideoID != "" {
		return strings.Join([]string{"video", request.VideoID}, "\x00")
	}
	if request.Key != "" {
		return strings.Join([]string{"key", request.Key}, "\x00")
	}
	parts := []string{
		"title",
		strings.ToLower(request.Title),
		strings.ToLower(request.Artist),
		strings.ToLower(request.Album),
	}
	if request.DurationSeconds > 0 {
		parts = append(parts, strconv.Itoa(int(math.Round(request.DurationSeconds))))
	}
	return strings.Join(parts, "\x00")
}

func snapshotAvailable(snapshot Snapshot) bool {
	switch snapshot.Kind {
	case KindSynced:
		return len(snapshot.Lines) > 0
	case KindPlain:
		return strings.TrimSpace(snapshot.Text) != ""
	default:
		return false
	}
}

func rank(kind string) int {
	switch kind {
	case KindSynced:
		return 2
	case KindPlain:
		return 1
	default:
		return 0
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot.Lines = cloneLines(snapshot.Lines)
	return snapshot
}

func cloneLines(lines []Line) []Line {
	if len(lines) == 0 {
		return nil
	}
	clone := make([]Line, len(lines))
	for index, line := range lines {
		clone[index] = line
		if len(line.AlternateTexts) > 0 {
			clone[index].AlternateTexts = append([]AlternateText(nil), line.AlternateTexts...)
		}
		if len(line.Words) > 0 {
			clone[index].Words = cloneWords(line.Words)
		}
	}
	return clone
}

func cloneWords(words []Word) []Word {
	if len(words) == 0 {
		return nil
	}
	clone := make([]Word, len(words))
	for index, word := range words {
		clone[index] = word
		if word.EndsWithSpace != nil {
			endsWithSpace := *word.EndsWithSpace
			clone[index].EndsWithSpace = &endsWithSpace
		}
		clone[index].Syllables = cloneWords(word.Syllables)
	}
	return clone
}
