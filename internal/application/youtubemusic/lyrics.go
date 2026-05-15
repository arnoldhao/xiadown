package youtubemusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"xiadown/internal/application/lyricsromanization"
)

const (
	lyricsResultSynced      = "synced"
	lyricsResultPlain       = "plain"
	lyricsResultUnavailable = "unavailable"
	lrcLibGetURL            = "https://lrclib.net/api/get"
	lrcLibSearchURL         = "https://lrclib.net/api/search"
	lrcLibTimeout           = 30 * time.Second
	lyricsCacheTTL          = 24 * time.Hour
	lyricsCacheMaxEntries   = 120
)

var (
	lrcTimePattern           = regexp.MustCompile(`\[(\d{2,}):(\d{2})\.(\d{2,3})\]`)
	lrcMetadataPattern       = regexp.MustCompile(`^\[([a-zA-Z]+):([^\]]+)\]\s*$`)
	lrcWordPattern           = regexp.MustCompile(`<(\d{2,}):(\d{2})\.(\d{2,3})>([^<]+)`)
	lyricsRequestRetryDelays = []time.Duration{220 * time.Millisecond, 650 * time.Millisecond}
)

type LyricsSearchInfo struct {
	VideoID         string
	Title           string
	Artist          string
	DurationSeconds float64
	PlainOnly       bool
}

type LyricLine struct {
	StartMs       int         `json:"startMs"`
	DurationMs    int         `json:"durationMs"`
	Text          string      `json:"text"`
	RomanizedText string      `json:"romanizedText,omitempty"`
	RomanizedKind string      `json:"romanizedKind,omitempty"`
	Words         []TimedWord `json:"words,omitempty"`
}

type TimedWord struct {
	StartMs int    `json:"startMs"`
	Text    string `json:"text"`
}

type LyricsResult struct {
	Kind   string      `json:"kind"`
	Source string      `json:"source,omitempty"`
	Text   string      `json:"text,omitempty"`
	Lines  []LyricLine `json:"lines,omitempty"`
}

type lrcLibModel struct {
	ID           int      `json:"id"`
	TrackName    string   `json:"trackName"`
	ArtistName   string   `json:"artistName"`
	AlbumName    string   `json:"albumName"`
	Duration     *float64 `json:"duration"`
	Instrumental *bool    `json:"instrumental"`
	PlainLyrics  string   `json:"plainLyrics"`
	SyncedLyrics string   `json:"syncedLyrics"`
}

type lyricsProvider struct {
	name   string
	search func(context.Context, LyricsSearchInfo) (LyricsResult, error)
}

type lyricsProviderResult struct {
	provider      string
	providerIndex int
	result        LyricsResult
	err           error
}

type lyricsCacheEntry struct {
	key        string
	result     LyricsResult
	updatedAt  time.Time
	lastAccess time.Time
}

type lyricsFetchCall struct {
	done   chan struct{}
	result LyricsResult
	err    error
}

type lrcLibLookupResult struct {
	name   string
	result LyricsResult
}

type lyricsProxyResolver interface {
	ResolveProxy(rawURL string) (string, error)
}

func logLyricsf(string, ...any) {
}

func lyricsInfoSummary(info LyricsSearchInfo) string {
	return fmt.Sprintf("video=%q title=%q artist=%q duration=%.0f plainOnly=%t", info.VideoID, info.Title, info.Artist, info.DurationSeconds, info.PlainOnly)
}

func lyricsResultSummary(result LyricsResult) string {
	return fmt.Sprintf("kind=%q source=%q lines=%d textChars=%d", result.Kind, result.Source, len(result.Lines), len(strings.TrimSpace(result.Text)))
}

func (client *Client) TrackLyrics(ctx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
	normalized := normalizeLyricsSearchInfo(info)
	cacheKey := lyricsCacheKey(normalized, localeFromContext(ctx))
	logLyricsf("request %s locale=%q cacheKey=%q", lyricsInfoSummary(normalized), localeFromContext(ctx), cacheKey)
	if cached, ok := client.cachedLyrics(cacheKey); ok && cached.Kind == lyricsResultSynced {
		logLyricsf("cache hit synced %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(cached))
		return cached, nil
	}

	call, leader := client.beginLyricsFetch(cacheKey)
	if !leader {
		logLyricsf("join in-flight %s", lyricsInfoSummary(normalized))
		select {
		case <-call.done:
			logLyricsf("in-flight done %s result=%s err=%v", lyricsInfoSummary(normalized), lyricsResultSummary(call.result), call.err)
			return call.result, call.err
		case <-ctx.Done():
			return LyricsResult{}, wrapRequestError(ctx.Err())
		}
	}

	cachedPlain, hasCachedPlain := client.cachedLyrics(cacheKey)
	if hasCachedPlain {
		logLyricsf("cache hit non-synced %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(cachedPlain))
	}
	result, err := client.trackLyricsUncached(ctx, normalized)
	if err != nil {
		logLyricsf("uncached failed %s err=%v", lyricsInfoSummary(normalized), err)
		if hasCachedPlain && cachedPlain.Kind == lyricsResultPlain {
			logLyricsf("fallback to cached plain %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(cachedPlain))
			result = cachedPlain
			err = nil
		}
	} else if result.Kind == lyricsResultPlain && hasCachedPlain && cachedPlain.Kind == lyricsResultPlain {
		logLyricsf("keep cached plain over fresh plain %s fresh=%s cached=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result), lyricsResultSummary(cachedPlain))
		result = cachedPlain
	} else if !lyricsResultAvailable(result) && hasCachedPlain && cachedPlain.Kind == lyricsResultPlain {
		logLyricsf("fallback to cached plain after unavailable %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(cachedPlain))
		result = cachedPlain
	} else if lyricsResultRank(cachedPlain.Kind) > lyricsResultRank(result.Kind) {
		logLyricsf("keep higher-ranked cached result %s fresh=%s cached=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result), lyricsResultSummary(cachedPlain))
		result = cachedPlain
	}
	if err == nil && lyricsResultAvailable(result) {
		logLyricsf("store cache %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result))
		client.storeLyricsCache(cacheKey, result)
	}
	client.completeLyricsFetch(cacheKey, call, result, err)
	logLyricsf("complete %s result=%s err=%v", lyricsInfoSummary(normalized), lyricsResultSummary(result), err)
	return result, err
}

func (client *Client) trackLyricsUncached(ctx context.Context, normalized LyricsSearchInfo) (LyricsResult, error) {
	if normalized.PlainOnly {
		return client.trackPlainLyricsUncached(ctx, normalized)
	}
	providers, err := client.lyricsProvidersForInfo(normalized)
	if err != nil {
		return LyricsResult{}, err
	}
	return client.fetchLyricsFromProviders(ctx, normalized, providers)
}

func (client *Client) trackPlainLyricsUncached(ctx context.Context, normalized LyricsSearchInfo) (LyricsResult, error) {
	var firstErr error
	if videoIDPattern.MatchString(normalized.VideoID) {
		order := "YTMusic"
		if strings.TrimSpace(normalized.Title) != "" {
			order = "YTMusic,LRCLib"
		}
		logLyricsf("plain providers %s order=%s", lyricsInfoSummary(normalized), order)
		result, err := client.searchYTMusicPlainLyricsProvider(ctx, normalized)
		if err != nil {
			firstErr = err
			logLyricsf("plain ytmusic failed %s err=%v", lyricsInfoSummary(normalized), err)
		} else if result.Kind == lyricsResultPlain {
			logLyricsf("plain ytmusic selected %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result))
			return result, nil
		} else {
			logLyricsf("plain ytmusic unavailable %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result))
		}
	}
	if strings.TrimSpace(normalized.Title) == "" {
		if firstErr != nil {
			return LyricsResult{}, firstErr
		}
		if normalized.VideoID == "" {
			return LyricsResult{Kind: lyricsResultUnavailable}, nil
		}
		return LyricsResult{}, fmt.Errorf("invalid youtube video id")
	}
	logLyricsf("plain lrclib fallback start %s", lyricsInfoSummary(normalized))
	result := client.searchLRCLibPlainLyrics(ctx, normalized)
	if result.Kind == lyricsResultPlain {
		logLyricsf("plain lrclib selected %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result))
		return result, nil
	}
	if firstErr != nil {
		logLyricsf("plain providers failed %s ytmusicErr=%v lrclib=%s", lyricsInfoSummary(normalized), firstErr, lyricsResultSummary(result))
		return LyricsResult{}, firstErr
	}
	logLyricsf("plain providers unavailable %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result))
	return result, nil
}

func (client *Client) lyricsProvidersForInfo(info LyricsSearchInfo) ([]lyricsProvider, error) {
	if !videoIDPattern.MatchString(info.VideoID) {
		if info.Title == "" {
			return nil, fmt.Errorf("invalid youtube video id")
		}
		logLyricsf("providers %s names=LRCLib reason=no-valid-video-id", lyricsInfoSummary(info))
		return []lyricsProvider{{
			name:   "LRCLib",
			search: client.lyricsLRCLibProviderForMode(info.PlainOnly),
		}}, nil
	}

	providers := []lyricsProvider{{
		name:   "YTMusic",
		search: client.lyricsYTMusicProviderForMode(info.PlainOnly),
	}}
	if !info.PlainOnly && strings.TrimSpace(info.Title) != "" {
		providers = append(providers, lyricsProvider{
			name:   "LRCLib",
			search: client.searchLRCLibLyricsProvider,
		})
	}
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.name)
	}
	logLyricsf("providers %s names=%s", lyricsInfoSummary(info), strings.Join(names, ","))
	return providers, nil
}

func (client *Client) lyricsYTMusicProviderForMode(plainOnly bool) func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
	if plainOnly {
		return client.searchYTMusicPlainLyricsProvider
	}
	return client.searchYTMusicLyricsProvider
}

func (client *Client) lyricsLRCLibProviderForMode(plainOnly bool) func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
	if plainOnly {
		return client.searchLRCLibPlainLyricsProvider
	}
	return client.searchLRCLibLyricsProvider
}

func (client *Client) fetchLyricsFromProviders(ctx context.Context, info LyricsSearchInfo, providers []lyricsProvider) (LyricsResult, error) {
	if len(providers) == 0 {
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}

	resultCh := make(chan lyricsProviderResult, len(providers))
	for index, provider := range providers {
		go func(providerIndex int, provider lyricsProvider) {
			startedAt := time.Now()
			logLyricsf("provider start name=%s %s", provider.name, lyricsInfoSummary(info))
			result, err := provider.search(ctx, info)
			logLyricsf("provider done name=%s elapsed=%s %s result=%s err=%v", provider.name, time.Since(startedAt).Round(time.Millisecond), lyricsInfoSummary(info), lyricsResultSummary(result), err)
			resultCh <- lyricsProviderResult{
				provider:      provider.name,
				providerIndex: providerIndex,
				result:        result,
				err:           err,
			}
		}(index, provider)
	}

	var best lyricsProviderResult
	var foundBest bool
	var firstErr error
	for range providers {
		select {
		case result := <-resultCh:
			if result.err != nil && firstErr == nil {
				firstErr = result.err
			}
			if !foundBest || isBetterLyricsProviderResult(result, best) {
				best = result
				foundBest = true
			}
		case <-ctx.Done():
			return LyricsResult{}, wrapRequestError(ctx.Err())
		}
	}

	if foundBest && lyricsResultAvailable(best.result) {
		logLyricsf("provider best name=%s %s result=%s", best.provider, lyricsInfoSummary(info), lyricsResultSummary(best.result))
		return best.result, nil
	}
	if firstErr != nil {
		logLyricsf("provider failed %s err=%v", lyricsInfoSummary(info), firstErr)
		return LyricsResult{}, firstErr
	}
	logLyricsf("provider unavailable %s", lyricsInfoSummary(info))
	return LyricsResult{Kind: lyricsResultUnavailable}, nil
}

func isBetterLyricsProviderResult(candidate lyricsProviderResult, current lyricsProviderResult) bool {
	candidateRank := lyricsResultRank(candidate.result.Kind)
	currentRank := lyricsResultRank(current.result.Kind)
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}
	if candidate.result.Kind == lyricsResultPlain && current.result.Kind == lyricsResultPlain {
		candidateIsYTMusic := candidate.provider == "YTMusic"
		currentIsYTMusic := current.provider == "YTMusic"
		if candidateIsYTMusic != currentIsYTMusic {
			return candidateIsYTMusic
		}
	}
	return candidate.providerIndex < current.providerIndex
}

func (client *Client) searchYTMusicLyricsProvider(ctx context.Context, normalized LyricsSearchInfo) (LyricsResult, error) {
	return client.searchYTMusicLyrics(ctx, normalized, true)
}

func (client *Client) searchYTMusicPlainLyricsProvider(ctx context.Context, normalized LyricsSearchInfo) (LyricsResult, error) {
	return client.searchYTMusicLyrics(ctx, normalized, false)
}

func (client *Client) searchYTMusicLyrics(ctx context.Context, normalized LyricsSearchInfo, allowSynced bool) (LyricsResult, error) {
	if !videoIDPattern.MatchString(normalized.VideoID) {
		logLyricsf("ytmusic skip invalid video id %s", lyricsInfoSummary(normalized))
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}

	mode := "synced"
	if !allowSynced {
		mode = "plain"
	}
	logLyricsf("ytmusic next start mode=%s %s", mode, lyricsInfoSummary(normalized))
	nextData, err := client.requestLyricsYouTubeData(ctx, "next", map[string]any{
		"videoId":                       normalized.VideoID,
		"enablePersistentPlaylistPanel": true,
		"isAudioOnly":                   true,
		"tunerSettingValue":             "AUTOMIX_SETTING_NORMAL",
	})
	if err != nil {
		logLyricsf("ytmusic next failed %s err=%v", lyricsInfoSummary(normalized), err)
		return LyricsResult{}, err
	}

	if allowSynced {
		if synced := extractTimedLyrics(nextData, "YTMusic"); synced.Kind == lyricsResultSynced {
			logLyricsf("ytmusic next synced %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(synced))
			return synced, nil
		}
	}

	metadata := parseTrackMetadata(nextData, normalized.VideoID)
	if normalized.Title == "" {
		normalized.Title = metadata.Title
	}
	if normalized.Artist == "" {
		normalized.Artist = metadata.Channel
	}
	if normalized.DurationSeconds <= 0 {
		normalized.DurationSeconds = durationLabelSeconds(metadata.DurationLabel)
	}
	if allowSynced {
		logLyricsf("ytmusic next no timed lyrics %s metadataTitle=%q metadataArtist=%q metadataDuration=%q", lyricsInfoSummary(normalized), metadata.Title, metadata.Channel, metadata.DurationLabel)
	} else {
		logLyricsf("ytmusic next plain mode skip timed lyrics %s metadataTitle=%q metadataArtist=%q metadataDuration=%q", lyricsInfoSummary(normalized), metadata.Title, metadata.Channel, metadata.DurationLabel)
	}

	if browseID := extractLyricsBrowseID(nextData); browseID != "" {
		logLyricsf("ytmusic browse start %s browseID=%q", lyricsInfoSummary(normalized), browseID)
		browseData, err := client.requestLyricsYouTubeData(ctx, "browse", map[string]any{
			"browseId": browseID,
		})
		if err != nil {
			logLyricsf("ytmusic browse failed %s browseID=%q err=%v", lyricsInfoSummary(normalized), browseID, err)
			return LyricsResult{}, err
		}
		if allowSynced {
			if synced := extractTimedLyrics(browseData, "YTMusic"); synced.Kind == lyricsResultSynced {
				logLyricsf("ytmusic browse synced %s browseID=%q result=%s", lyricsInfoSummary(normalized), browseID, lyricsResultSummary(synced))
				return synced, nil
			}
		}
		if plain := extractPlainLyrics(browseData, "YTMusic"); plain.Kind == lyricsResultPlain {
			logLyricsf("ytmusic browse plain %s browseID=%q result=%s", lyricsInfoSummary(normalized), browseID, lyricsResultSummary(plain))
			return plain, nil
		}
		logLyricsf("ytmusic browse unavailable %s browseID=%q", lyricsInfoSummary(normalized), browseID)
	} else {
		logLyricsf("ytmusic no lyrics browse id %s", lyricsInfoSummary(normalized))
	}

	return LyricsResult{Kind: lyricsResultUnavailable}, nil
}

func (client *Client) searchLRCLibLyricsProvider(ctx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
	return client.searchLRCLibLyrics(ctx, info), nil
}

func (client *Client) searchLRCLibPlainLyricsProvider(ctx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
	return client.searchLRCLibPlainLyrics(ctx, info), nil
}

func lyricsCacheKey(info LyricsSearchInfo, locale string) string {
	normalizedLocale := NormalizeLocale(locale)
	mode := "synced"
	if info.PlainOnly {
		mode = "plain"
	}
	videoID := strings.TrimSpace(info.VideoID)
	if videoID != "" {
		return strings.Join([]string{normalizedLocale, mode, "video", videoID}, "\x00")
	}
	parts := []string{
		normalizedLocale,
		mode,
		"title",
		normalizeLyricsMatchText(info.Title),
		normalizeLyricsMatchText(info.Artist),
	}
	if info.DurationSeconds > 0 {
		parts = append(parts, strconv.Itoa(int(math.Round(info.DurationSeconds))))
	}
	return strings.Join(parts, "\x00")
}

func (client *Client) cachedLyrics(key string) (LyricsResult, bool) {
	if strings.TrimSpace(key) == "" {
		return LyricsResult{}, false
	}
	now := client.currentTime()
	client.lyricsMu.Lock()
	defer client.lyricsMu.Unlock()
	if client.lyricsCache == nil {
		client.lyricsCache = make(map[string]lyricsCacheEntry)
	}
	entry, ok := client.lyricsCache[key]
	if !ok {
		return LyricsResult{}, false
	}
	if now.Sub(entry.updatedAt) > lyricsCacheTTL {
		delete(client.lyricsCache, key)
		return LyricsResult{}, false
	}
	entry.lastAccess = now
	client.lyricsCache[key] = entry
	return entry.result, true
}

func (client *Client) storeLyricsCache(key string, result LyricsResult) {
	if strings.TrimSpace(key) == "" || !lyricsResultAvailable(result) {
		return
	}
	now := client.currentTime()
	client.lyricsMu.Lock()
	defer client.lyricsMu.Unlock()
	if client.lyricsCache == nil {
		client.lyricsCache = make(map[string]lyricsCacheEntry)
	}
	if existing, ok := client.lyricsCache[key]; ok && lyricsResultRank(existing.result.Kind) > lyricsResultRank(result.Kind) {
		existing.lastAccess = now
		client.lyricsCache[key] = existing
		return
	}
	client.lyricsCache[key] = lyricsCacheEntry{
		key:        key,
		result:     result,
		updatedAt:  now,
		lastAccess: now,
	}
	client.evictLyricsCacheLocked(now)
}

func (client *Client) evictLyricsCacheLocked(now time.Time) {
	for key, entry := range client.lyricsCache {
		if now.Sub(entry.updatedAt) > lyricsCacheTTL {
			delete(client.lyricsCache, key)
		}
	}
	for len(client.lyricsCache) > lyricsCacheMaxEntries {
		var oldestKey string
		var oldestAccess time.Time
		for key, entry := range client.lyricsCache {
			if oldestKey == "" || entry.lastAccess.Before(oldestAccess) {
				oldestKey = key
				oldestAccess = entry.lastAccess
			}
		}
		if oldestKey == "" {
			return
		}
		delete(client.lyricsCache, oldestKey)
	}
}

func (client *Client) beginLyricsFetch(key string) (*lyricsFetchCall, bool) {
	client.lyricsMu.Lock()
	defer client.lyricsMu.Unlock()
	if client.lyricsInFlight == nil {
		client.lyricsInFlight = make(map[string]*lyricsFetchCall)
	}
	if call := client.lyricsInFlight[key]; call != nil {
		return call, false
	}
	call := &lyricsFetchCall{done: make(chan struct{})}
	client.lyricsInFlight[key] = call
	return call, true
}

func (client *Client) completeLyricsFetch(key string, call *lyricsFetchCall, result LyricsResult, err error) {
	client.lyricsMu.Lock()
	if client.lyricsInFlight[key] == call {
		delete(client.lyricsInFlight, key)
	}
	call.result = result
	call.err = err
	close(call.done)
	client.lyricsMu.Unlock()
}

func (client *Client) currentTime() time.Time {
	if client.now != nil {
		return client.now()
	}
	return time.Now()
}

func lyricsResultAvailable(result LyricsResult) bool {
	switch result.Kind {
	case lyricsResultSynced:
		return len(result.Lines) > 0
	case lyricsResultPlain:
		return strings.TrimSpace(result.Text) != ""
	default:
		return false
	}
}

func lyricsResultRank(kind string) int {
	switch kind {
	case lyricsResultSynced:
		return 2
	case lyricsResultPlain:
		return 1
	default:
		return 0
	}
}

func (client *Client) requestLyricsYouTubeData(ctx context.Context, endpoint string, body map[string]any) (map[string]any, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		data, err := client.request(ctx, endpoint, body)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !isRetryableLyricsRequestError(err) || attempt >= len(lyricsRequestRetryDelays) {
			break
		}
		delay := lyricsRequestRetryDelays[attempt]
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, wrapRequestError(ctx.Err())
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isRetryableLyricsRequestError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, ErrRequestTimedOut) || errors.Is(err, ErrNetworkUnavailable) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "youtube music api status 429") ||
		strings.Contains(lower, "youtube music api status 500") ||
		strings.Contains(lower, "youtube music api status 502") ||
		strings.Contains(lower, "youtube music api status 503") ||
		strings.Contains(lower, "youtube music api status 504")
}

func normalizeLyricsSearchInfo(info LyricsSearchInfo) LyricsSearchInfo {
	info.VideoID = strings.TrimSpace(info.VideoID)
	info.Title = strings.TrimSpace(info.Title)
	info.Artist = strings.TrimSpace(info.Artist)
	if math.IsNaN(info.DurationSeconds) || math.IsInf(info.DurationSeconds, 0) || info.DurationSeconds < 0 {
		info.DurationSeconds = 0
	}
	return info
}

func extractLyricsBrowseID(data map[string]any) string {
	contents := asMap(data["contents"])
	watchNext := asMap(contents["singleColumnMusicWatchNextResultsRenderer"])
	tabbed := asMap(watchNext["tabbedRenderer"])
	watchNextTabbed := asMap(tabbed["watchNextTabbedResultsRenderer"])
	for _, tab := range mapsFromArray(watchNextTabbed["tabs"]) {
		tabRenderer := asMap(tab["tabRenderer"])
		endpoint := asMap(tabRenderer["endpoint"])
		browseEndpoint := asMap(endpoint["browseEndpoint"])
		browseID := stringInMap(browseEndpoint, "browseId")
		if strings.HasPrefix(browseID, "MPLYt") {
			return browseID
		}
	}
	return ""
}

func extractTimedLyrics(data map[string]any, source string) LyricsResult {
	timedModel := findMapByKey(data, "timedLyricsModel")
	if timedModel == nil {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	lyricsData, ok := timedModel["lyricsData"].([]any)
	if !ok || len(lyricsData) == 0 {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	lines := make([]LyricLine, 0, len(lyricsData))
	for _, item := range lyricsData {
		lineData := asMap(item)
		text := strings.TrimSpace(stringInMap(lineData, "lyricLine"))
		startMs, ok := parseFlexibleInt(lineData["startTimeMs"])
		if !ok {
			continue
		}
		durationMs, _ := parseFlexibleInt(lineData["durationMs"])
		lines = append(lines, LyricLine{
			StartMs:    max(0, startMs),
			DurationMs: max(0, durationMs),
			Text:       text,
		})
	}
	if len(lines) == 0 {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	sort.SliceStable(lines, func(left, right int) bool {
		return lines[left].StartMs < lines[right].StartMs
	})
	fillLyricLineDurations(lines)
	fillRomanizedLyricLines(lines)
	return LyricsResult{
		Kind:   lyricsResultSynced,
		Source: source,
		Lines:  lines,
	}
}

func extractPlainLyrics(data map[string]any, fallbackSource string) LyricsResult {
	contents := asMap(data["contents"])
	sectionList := asMap(contents["sectionListRenderer"])
	for _, section := range mapsFromArray(sectionList["contents"]) {
		shelf := asMap(section["musicDescriptionShelfRenderer"])
		if shelf == nil {
			continue
		}
		description := asMap(shelf["description"])
		text := strings.TrimSpace(strings.Join(rawRunsText(description), ""))
		if text == "" {
			continue
		}
		source := strings.TrimSpace(strings.Join(rawRunsText(asMap(shelf["footer"])), ""))
		if source == "" {
			source = fallbackSource
		}
		return LyricsResult{
			Kind:   lyricsResultPlain,
			Source: source,
			Text:   text,
			Lines:  plainLyricLines(text),
		}
	}
	return LyricsResult{Kind: lyricsResultUnavailable}
}

func (client *Client) searchLRCLibLyrics(ctx context.Context, info LyricsSearchInfo) LyricsResult {
	return client.searchLRCLibLyricsWithMode(ctx, info, false)
}

func (client *Client) searchLRCLibPlainLyrics(ctx context.Context, info LyricsSearchInfo) LyricsResult {
	return client.searchLRCLibLyricsWithMode(ctx, info, true)
}

func (client *Client) searchLRCLibLyricsWithMode(ctx context.Context, info LyricsSearchInfo, plainOnly bool) LyricsResult {
	if strings.TrimSpace(info.Title) == "" {
		logLyricsf("lrclib skip empty title %s", lyricsInfoSummary(info))
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	lookupCtx, cancel := context.WithTimeout(ctx, lrcLibTimeout)
	defer cancel()

	resultCh := make(chan lrcLibLookupResult, 2)
	go func() {
		logLyricsf("lrclib exact lookup start %s", lyricsInfoSummary(info))
		resultCh <- lrcLibLookupResult{
			name:   "exact",
			result: client.getLRCLibExactLyrics(lookupCtx, info, plainOnly),
		}
	}()
	go func() {
		logLyricsf("lrclib search lookup start %s", lyricsInfoSummary(info))
		resultCh <- lrcLibLookupResult{
			name:   "search",
			result: client.searchLRCLibSearchLyrics(lookupCtx, info, plainOnly),
		}
	}()

	var exact LyricsResult
	var search LyricsResult
	var gotExact bool
	var gotSearch bool
	for received := 0; received < 2; received++ {
		select {
		case lookup := <-resultCh:
			switch lookup.name {
			case "exact":
				exact = lookup.result
				gotExact = true
				logLyricsf("lrclib exact done %s result=%s", lyricsInfoSummary(info), lyricsResultSummary(exact))
			case "search":
				search = lookup.result
				gotSearch = true
				logLyricsf("lrclib search done %s result=%s", lyricsInfoSummary(info), lyricsResultSummary(search))
			}
			if lookup.result.Kind == lyricsResultSynced || (plainOnly && lookup.result.Kind == lyricsResultPlain) {
				cancel()
				logLyricsf("lrclib %s selected %s %s result=%s", lookup.name, lookup.result.Kind, lyricsInfoSummary(info), lyricsResultSummary(lookup.result))
				return lookup.result
			}
		case <-lookupCtx.Done():
			logLyricsf("lrclib lookup timeout %s exact=%s search=%s gotExact=%t gotSearch=%t err=%v", lyricsInfoSummary(info), lyricsResultSummary(exact), lyricsResultSummary(search), gotExact, gotSearch, lookupCtx.Err())
			return chooseLRCLibLookupResult(info, exact, gotExact, search, gotSearch)
		}
	}
	return chooseLRCLibLookupResult(info, exact, gotExact, search, gotSearch)
}

func chooseLRCLibLookupResult(info LyricsSearchInfo, exact LyricsResult, gotExact bool, search LyricsResult, gotSearch bool) LyricsResult {
	if gotSearch && lyricsResultRank(search.Kind) > lyricsResultRank(exact.Kind) {
		logLyricsf("lrclib search selected %s exact=%s search=%s", lyricsInfoSummary(info), lyricsResultSummary(exact), lyricsResultSummary(search))
		return search
	}
	if gotExact && lyricsResultAvailable(exact) {
		logLyricsf("lrclib exact fallback selected %s exact=%s search=%s", lyricsInfoSummary(info), lyricsResultSummary(exact), lyricsResultSummary(search))
		return exact
	}
	logLyricsf("lrclib search fallback selected %s result=%s", lyricsInfoSummary(info), lyricsResultSummary(search))
	return search
}

func (client *Client) getLRCLibExactLyrics(ctx context.Context, info LyricsSearchInfo, plainOnly bool) LyricsResult {
	values := buildLRCLibExactQuery(info)
	if values.Encode() == "" {
		logLyricsf("lrclib exact empty query %s", lyricsInfoSummary(info))
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	requestURL := lrcLibGetURL + "?" + values.Encode()

	var model lrcLibModel
	if !client.decodeLRCLibJSON(ctx, requestURL, &model) {
		if ctx.Err() != nil {
			return LyricsResult{Kind: lyricsResultUnavailable}
		}
		logLyricsf("lrclib exact decode failed %s query=%q", lyricsInfoSummary(info), values.Encode())
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	result := lrcLibModelLyricsResult(model, plainOnly)
	logLyricsf(
		"lrclib exact candidate %s id=%d track=%q artist=%q duration=%s syncedChars=%d plainChars=%d result=%s",
		lyricsInfoSummary(info),
		model.ID,
		model.TrackName,
		model.ArtistName,
		formatOptionalFloat(model.Duration),
		len(strings.TrimSpace(model.SyncedLyrics)),
		len(strings.TrimSpace(model.PlainLyrics)),
		lyricsResultSummary(result),
	)
	return result
}

func (client *Client) searchLRCLibSearchLyrics(ctx context.Context, info LyricsSearchInfo, plainOnly bool) LyricsResult {
	values := buildLRCLibSearchQuery(info)
	if values.Encode() == "" {
		logLyricsf("lrclib search empty query %s", lyricsInfoSummary(info))
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	requestURL := lrcLibSearchURL + "?" + values.Encode()

	var models []lrcLibModel
	if !client.decodeLRCLibJSON(ctx, requestURL, &models) {
		if ctx.Err() != nil {
			return LyricsResult{Kind: lyricsResultUnavailable}
		}
		logLyricsf("lrclib search decode failed %s query=%q", lyricsInfoSummary(info), values.Encode())
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	total, valid, synced := countLRCLibModels(models)
	logLyricsf("lrclib search candidates %s total=%d valid=%d synced=%d query=%q", lyricsInfoSummary(info), total, valid, synced, values.Encode())

	model, ok := bestLRCLibModelForInfoWithMode(models, info, plainOnly)
	if !ok {
		logLyricsf("lrclib search no usable candidates %s", lyricsInfoSummary(info))
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	result := lrcLibModelLyricsResult(model, plainOnly)
	logLyricsf(
		"lrclib search candidate selected %s id=%d track=%q artist=%q duration=%s syncedChars=%d plainChars=%d result=%s",
		lyricsInfoSummary(info),
		model.ID,
		model.TrackName,
		model.ArtistName,
		formatOptionalFloat(model.Duration),
		len(strings.TrimSpace(model.SyncedLyrics)),
		len(strings.TrimSpace(model.PlainLyrics)),
		lyricsResultSummary(result),
	)
	return result
}

func (client *Client) decodeLRCLibJSON(ctx context.Context, requestURL string, target any) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		logLyricsf("lrclib request build failed url=%q err=%v", redactLRCLibURL(requestURL), err)
		return false
	}
	req.Header.Set("User-Agent", "XiaDown/1.0")
	req.Header.Set("Accept", "application/json")

	proxyRoute := client.lrclibProxyRoute(requestURL)
	startedAt := time.Now()
	logLyricsf("lrclib request start url=%q proxy=%q", redactLRCLibURL(requestURL), proxyRoute)
	resp, err := client.httpClientForRequest().Do(req)
	if err != nil {
		elapsed := time.Since(startedAt).Round(time.Millisecond)
		if ctxErr := ctx.Err(); ctxErr != nil {
			logLyricsf("lrclib request cancelled url=%q proxy=%q elapsed=%s err=%v", redactLRCLibURL(requestURL), proxyRoute, elapsed, ctxErr)
			return false
		}
		logLyricsf("lrclib request failed url=%q proxy=%q elapsed=%s err=%v", redactLRCLibURL(requestURL), proxyRoute, elapsed, err)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
		logLyricsf("lrclib request non-200 url=%q proxy=%q status=%d elapsed=%s", redactLRCLibURL(requestURL), proxyRoute, resp.StatusCode, time.Since(startedAt).Round(time.Millisecond))
		return false
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	err = decoder.Decode(target)
	_ = resp.Body.Close()
	if err != nil {
		logLyricsf("lrclib decode failed url=%q proxy=%q elapsed=%s err=%v", redactLRCLibURL(requestURL), proxyRoute, time.Since(startedAt).Round(time.Millisecond), err)
		return false
	}
	logLyricsf("lrclib request ok url=%q proxy=%q elapsed=%s", redactLRCLibURL(requestURL), proxyRoute, time.Since(startedAt).Round(time.Millisecond))
	return true
}

func (client *Client) lrclibProxyRoute(requestURL string) string {
	if client == nil || client.httpClientProvider == nil {
		return "default-client"
	}
	resolver, ok := client.httpClientProvider.(lyricsProxyResolver)
	if !ok {
		return "unknown"
	}
	proxyURL, err := resolver.ResolveProxy(requestURL)
	if err != nil {
		return "error:" + err.Error()
	}
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return "direct"
	}
	return redactProxyURL(proxyURL)
}

func lrcLibModelLyricsResult(model lrcLibModel, plainOnly bool) LyricsResult {
	if !plainOnly && strings.TrimSpace(model.SyncedLyrics) != "" {
		if lines := parseLRCLines(model.SyncedLyrics); len(lines) > 0 {
			fillRomanizedLyricLines(lines)
			return LyricsResult{
				Kind:   lyricsResultSynced,
				Source: "LRCLib",
				Lines:  lines,
			}
		}
		logLyricsf("lrclib synced parse produced no lines id=%d track=%q artist=%q syncedChars=%d", model.ID, model.TrackName, model.ArtistName, len(strings.TrimSpace(model.SyncedLyrics)))
	}
	if strings.TrimSpace(model.PlainLyrics) != "" {
		text := strings.TrimSpace(model.PlainLyrics)
		return LyricsResult{
			Kind:   lyricsResultPlain,
			Source: "LRCLib",
			Text:   text,
			Lines:  plainLyricLines(text),
		}
	}
	return LyricsResult{Kind: lyricsResultUnavailable}
}

func countLRCLibModels(models []lrcLibModel) (int, int, int) {
	valid := 0
	synced := 0
	for _, model := range models {
		if model.Instrumental != nil && *model.Instrumental {
			continue
		}
		hasSynced := strings.TrimSpace(model.SyncedLyrics) != ""
		hasPlain := strings.TrimSpace(model.PlainLyrics) != ""
		if !hasSynced && !hasPlain {
			continue
		}
		valid++
		if hasSynced {
			synced++
		}
	}
	return len(models), valid, synced
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(int(math.Round(*value)))
}

func redactLRCLibURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Scheme + "://" + parsed.Host + parsed.Path + "?" + parsed.RawQuery
}

func redactProxyURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.User != nil {
		parsed.User = url.User("***")
	}
	return parsed.String()
}

func bestLRCLibModel(models []lrcLibModel, targetDuration float64) (lrcLibModel, bool) {
	return bestLRCLibModelForInfo(models, LyricsSearchInfo{DurationSeconds: targetDuration})
}

func bestLRCLibModelForInfo(models []lrcLibModel, info LyricsSearchInfo) (lrcLibModel, bool) {
	return bestLRCLibModelForInfoWithMode(models, info, false)
}

func bestLRCLibModelForInfoWithMode(models []lrcLibModel, info LyricsSearchInfo, plainOnly bool) (lrcLibModel, bool) {
	valid := make([]lrcLibModel, 0, len(models))
	for _, model := range models {
		instrumental := model.Instrumental != nil && *model.Instrumental
		if instrumental {
			continue
		}
		if plainOnly {
			if strings.TrimSpace(model.PlainLyrics) == "" {
				continue
			}
			valid = append(valid, model)
			continue
		}
		if strings.TrimSpace(model.SyncedLyrics) == "" && strings.TrimSpace(model.PlainLyrics) == "" {
			continue
		}
		valid = append(valid, model)
	}
	if len(valid) == 0 {
		return lrcLibModel{}, false
	}
	if plainOnly {
		return closestLRCLibModel(valid, info.DurationSeconds), true
	}
	synced := make([]lrcLibModel, 0, len(valid))
	for _, model := range valid {
		if strings.TrimSpace(model.SyncedLyrics) != "" {
			synced = append(synced, model)
		}
	}
	if len(synced) > 0 {
		return closestLRCLibModel(synced, info.DurationSeconds), true
	}
	return closestLRCLibModel(valid, info.DurationSeconds), true
}

func closestLRCLibModel(models []lrcLibModel, targetDuration float64) lrcLibModel {
	if targetDuration <= 0 {
		return models[0]
	}
	best := models[0]
	bestDiff := math.MaxFloat64
	for _, model := range models {
		duration := 0.0
		if model.Duration != nil {
			duration = *model.Duration
		}
		diff := math.Abs(duration - targetDuration)
		if diff < bestDiff {
			best = model
			bestDiff = diff
		}
	}
	return best
}

func buildLRCLibSearchQuery(info LyricsSearchInfo) url.Values {
	title := strings.TrimSpace(info.Title)
	artist := strings.TrimSpace(info.Artist)
	values := url.Values{}
	if title != "" {
		values.Set("track_name", title)
	}
	if artist != "" {
		values.Set("artist_name", artist)
	}
	return values
}

func buildLRCLibExactQuery(info LyricsSearchInfo) url.Values {
	values := buildLRCLibSearchQuery(info)
	if values.Encode() == "" {
		return values
	}
	if info.DurationSeconds > 0 {
		values.Set("duration", strconv.Itoa(int(math.Round(info.DurationSeconds))))
	}
	return values
}

func normalizeLyricsMatchText(value string) string {
	cleaned := strings.ToLower(value)
	cleaned = regexp.MustCompile(`[^a-z0-9\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}]+`).ReplaceAllString(cleaned, " ")
	return strings.Join(strings.Fields(cleaned), " ")
}

func parseLRCLines(raw string) []LyricLine {
	rawLines := strings.Split(raw, "\n")
	offsetMs := 0
	lines := make([]LyricLine, 0, len(rawLines))
	for _, rawLine := range rawLines {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" {
			continue
		}
		if match := lrcMetadataPattern.FindStringSubmatch(line); len(match) == 3 {
			if strings.EqualFold(match[1], "offset") {
				if offset, err := strconv.Atoi(strings.TrimSpace(match[2])); err == nil {
					offsetMs = offset
				}
			}
			continue
		}

		matches := lrcTimePattern.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		textOnly := strings.TrimSpace(lrcTimePattern.ReplaceAllString(line, ""))
		words := parseLRCWords(textOnly, offsetMs)
		if len(words) > 0 {
			textOnly = strings.TrimSpace(lrcWordPattern.ReplaceAllString(textOnly, "$4"))
		}
		for _, match := range matches {
			startMs, ok := parseLRCTimeMatch(line, match)
			if !ok {
				continue
			}
			lines = append(lines, LyricLine{
				StartMs: max(0, startMs-offsetMs),
				Text:    textOnly,
				Words:   words,
			})
		}
	}
	if len(lines) == 0 {
		return nil
	}
	sort.SliceStable(lines, func(left, right int) bool {
		return lines[left].StartMs < lines[right].StartMs
	})
	if lines[0].StartMs > 300 {
		lines = append([]LyricLine{{
			StartMs:    0,
			DurationMs: lines[0].StartMs,
			Text:       "",
		}}, lines...)
	}
	fillLyricLineDurations(lines)
	return lines
}

func parseLRCWords(text string, offsetMs int) []TimedWord {
	matches := lrcWordPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	words := make([]TimedWord, 0, len(matches))
	for _, match := range matches {
		if len(match) != 5 {
			continue
		}
		startMs, ok := parseLRCTimeParts(match[1], match[2], match[3])
		if !ok {
			continue
		}
		words = append(words, TimedWord{
			StartMs: max(0, startMs-offsetMs),
			Text:    strings.TrimSpace(match[4]),
		})
	}
	return words
}

func parseLRCTimeMatch(line string, match []int) (int, bool) {
	if len(match) < 8 {
		return 0, false
	}
	return parseLRCTimeParts(line[match[2]:match[3]], line[match[4]:match[5]], line[match[6]:match[7]])
}

func parseLRCTimeParts(minutesText string, secondsText string, fractionText string) (int, bool) {
	minutes, err := strconv.Atoi(minutesText)
	if err != nil {
		return 0, false
	}
	seconds, err := strconv.Atoi(secondsText)
	if err != nil {
		return 0, false
	}
	fraction := fractionText
	for len(fraction) < 3 {
		fraction += "0"
	}
	if len(fraction) > 3 {
		fraction = fraction[:3]
	}
	millis, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, false
	}
	return minutes*60*1000 + seconds*1000 + millis, true
}

func fillLyricLineDurations(lines []LyricLine) {
	for index := range lines {
		if lines[index].DurationMs > 0 {
			continue
		}
		if index < len(lines)-1 {
			lines[index].DurationMs = max(0, lines[index+1].StartMs-lines[index].StartMs)
			continue
		}
		lines[index].DurationMs = 5000
	}
}

func fillRomanizedLyricLines(lines []LyricLine) {
	for index := range lines {
		if strings.TrimSpace(lines[index].RomanizedText) != "" {
			continue
		}
		romanized := lyricsromanization.Transcribe(lines[index].Text)
		lines[index].RomanizedText = romanized.Text
		lines[index].RomanizedKind = string(romanized.Kind)
	}
}

func plainLyricLines(text string) []LyricLine {
	rawLines := strings.Split(text, "\n")
	lines := make([]LyricLine, 0, len(rawLines))
	for _, rawLine := range rawLines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		lines = append(lines, LyricLine{Text: line})
	}
	fillRomanizedLyricLines(lines)
	return lines
}

func rawRunsText(text map[string]any) []string {
	if text == nil {
		return nil
	}
	runs, ok := text["runs"].([]any)
	if !ok {
		value, _ := text["text"].(string)
		if value == "" {
			return nil
		}
		return []string{value}
	}
	result := make([]string, 0, len(runs))
	for _, run := range runs {
		value, _ := asMap(run)["text"].(string)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func findMapByKey(value any, key string) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		if found := asMap(typed[key]); found != nil {
			return found
		}
		for _, item := range typed {
			if found := findMapByKey(item, key); found != nil {
				return found
			}
		}
	case []any:
		for _, item := range typed {
			if found := findMapByKey(item, key); found != nil {
				return found
			}
		}
	}
	return nil
}

func parseFlexibleInt(value any) (int, bool) {
	switch typed := value.(type) {
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		return number, err == nil
	case json.Number:
		number, err := typed.Int64()
		return int(number), err == nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	default:
		return 0, false
	}
}

func durationLabelSeconds(label string) float64 {
	parts := strings.Split(strings.TrimSpace(label), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	total := 0
	for _, part := range parts {
		number, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return 0
		}
		total = total*60 + number
	}
	return float64(total)
}
