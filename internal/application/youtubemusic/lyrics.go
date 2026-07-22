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
	"unicode"

	"golang.org/x/text/unicode/norm"
	"xiadown/internal/application/lyricsromanization"
)

const (
	lyricsResultSynced      = "synced"
	lyricsResultPlain       = "plain"
	lyricsResultUnavailable = "unavailable"
	lrcLibGetURL            = "https://lrclib.net/api/get-cached"
	lrcLibSearchURL         = "https://lrclib.net/api/search"
	lrcLibTimeout           = 15 * time.Second
	lyricsCacheTTL          = 24 * time.Hour
	lyricsPlainFreshTTL     = 5 * time.Minute
	lyricsUnavailableTTL    = 2 * time.Minute
	lyricsCacheMaxEntries   = 120
	// Matching is conservative: identity confidence leads, and timing quality
	// only breaks near-ties between candidates that already cleared the gate.
	lrcLibMinimumConfidence   = 0.78
	lrcLibMinimumTitleMatch   = 0.70
	lrcLibMinimumArtistMatch  = 0.55
	lrcLibQualityScoreBand    = 0.015
	lrcLibImmediateConfidence = 0.9999
	maxLyricsSearchVariants   = 3
	maxLyricsVariantWorkers   = 4
)

var (
	lrcTimePattern           = regexp.MustCompile(`\[(\d{2,}):(\d{2})\.(\d{2,3})\]`)
	lrcMetadataPattern       = regexp.MustCompile(`^\[([a-zA-Z]+):([^\]]+)\]\s*$`)
	lrcWordTimePattern       = regexp.MustCompile(`<(\d{2,}):(\d{2})\.(\d{2,3})>`)
	lyricsArtistSeparator    = regexp.MustCompile(`(?i)(?:\s+(?:feat\.?|featuring|ft\.?|with|x)\s+|\s*[&,;/、，]\s*)`)
	lyricsRequestRetryDelays = []time.Duration{220 * time.Millisecond, 650 * time.Millisecond}
	lyricsProviderSoftWait   = 1200 * time.Millisecond
	lyricsTimingUpgradeWait  = 180 * time.Millisecond
	// A line-timed result is immediately usable, but AMLL's independently
	// searched TTML can provide the word/syllable timeline that Focus needs.
	// Bound that quality upgrade window so the optional provider can never
	// hold playback indefinitely when its community search or CDN is down.
	lyricsLineTimingUpgradeWait = 2500 * time.Millisecond
)

type LyricsSearchInfo struct {
	VideoID         string
	Title           string
	Artist          string
	Album           string
	DurationSeconds float64
	PlainOnly       bool
	SearchVariants  []LyricsSearchVariant
}

type LyricsSearchVariant struct {
	Title  string `json:"title,omitempty"`
	Artist string `json:"artist,omitempty"`
}

type LyricLine struct {
	StartMs         int         `json:"startMs"`
	DurationMs      int         `json:"durationMs"`
	Text            string      `json:"text"`
	TranslationText string      `json:"translationText,omitempty"`
	RomanizedText   string      `json:"romanizedText,omitempty"`
	RomanizedKind   string      `json:"romanizedKind,omitempty"`
	Words           []TimedWord `json:"words,omitempty"`
}

type TimedWord struct {
	StartMs       int         `json:"startMs"`
	EndMs         int         `json:"endMs,omitempty"`
	Text          string      `json:"text"`
	EndsWithSpace *bool       `json:"endsWithSpace,omitempty"`
	Syllables     []TimedWord `json:"syllables,omitempty"`
}

type LyricsResult struct {
	Kind            string      `json:"kind"`
	Source          string      `json:"source,omitempty"`
	ProviderID      string      `json:"providerId,omitempty"`
	ProviderTrackID string      `json:"providerTrackId,omitempty"`
	Attribution     string      `json:"attribution,omitempty"`
	TimingQuality   string      `json:"timingQuality,omitempty"`
	Confidence      int         `json:"confidence,omitempty"`
	Text            string      `json:"text,omitempty"`
	Lines           []LyricLine `json:"lines,omitempty"`
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

type lyricsQueryResult struct {
	queryIndex int
	result     LyricsResult
	err        error
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
	name       string
	result     LyricsResult
	confidence float64
	definitive bool
	err        error
}

type lrcLibHTTPError struct {
	StatusCode int
}

func (err *lrcLibHTTPError) Error() string {
	return fmt.Sprintf("lrclib api status %d", err.StatusCode)
}

func (err *lrcLibHTTPError) HTTPStatusCode() int {
	if err == nil {
		return 0
	}
	return err.StatusCode
}

type lyricsProxyResolver interface {
	ResolveProxy(rawURL string) (string, error)
}

func logLyricsf(string, ...any) {}

func lyricsInfoSummary(info LyricsSearchInfo) string {
	return fmt.Sprintf("video=%q title=%q artist=%q album=%q duration=%.0f plainOnly=%t", info.VideoID, info.Title, info.Artist, info.Album, info.DurationSeconds, info.PlainOnly)
}

func lyricsResultSummary(result LyricsResult) string {
	return fmt.Sprintf("kind=%q source=%q lines=%d textChars=%d", result.Kind, result.Source, len(result.Lines), len(strings.TrimSpace(result.Text)))
}

func (client *Client) TrackLyrics(ctx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
	normalized := normalizeLyricsSearchInfo(info)
	locale := localeFromContext(ctx)
	cacheKey := lyricsCacheKey(normalized, locale)
	requestKey := lyricsRequestCacheKey(normalized, locale)
	logLyricsf("request %s locale=%q cacheKey=%q requestKey=%q", lyricsInfoSummary(normalized), locale, cacheKey, requestKey)
	if cached, age, ok := client.cachedLyrics(cacheKey); ok &&
		lyricsResultAvailable(cached) && lyricsCacheEntryFresh(cached, age) {
		logLyricsf("cache hit fresh age=%s %s result=%s", age.Round(time.Millisecond), lyricsInfoSummary(normalized), lyricsResultSummary(cached))
		return enrichLyricsResult(cached), nil
	}
	if cached, age, ok := client.cachedLyrics(requestKey); ok &&
		cached.Kind == lyricsResultUnavailable && lyricsCacheEntryFresh(cached, age) {
		logLyricsf("negative cache hit fresh age=%s %s", age.Round(time.Millisecond), lyricsInfoSummary(normalized))
		return enrichLyricsResult(cached), nil
	}

	call, leader := client.beginLyricsFetch(requestKey)
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

	cachedPlain, cachedAge, hasCachedPlain := client.cachedLyrics(cacheKey)
	if hasCachedPlain && lyricsResultAvailable(cachedPlain) && lyricsCacheEntryFresh(cachedPlain, cachedAge) {
		client.completeLyricsFetch(requestKey, call, cachedPlain, nil)
		logLyricsf("cache hit after leader election age=%s %s result=%s", cachedAge.Round(time.Millisecond), lyricsInfoSummary(normalized), lyricsResultSummary(cachedPlain))
		return cachedPlain, nil
	}
	if cachedUnavailable, cachedUnavailableAge, ok := client.cachedLyrics(requestKey); ok &&
		cachedUnavailable.Kind == lyricsResultUnavailable && lyricsCacheEntryFresh(cachedUnavailable, cachedUnavailableAge) {
		client.completeLyricsFetch(requestKey, call, cachedUnavailable, nil)
		logLyricsf("negative cache hit after leader election age=%s %s", cachedUnavailableAge.Round(time.Millisecond), lyricsInfoSummary(normalized))
		return cachedUnavailable, nil
	}
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
	} else if !lyricsResultAvailable(result) && hasCachedPlain && cachedPlain.Kind == lyricsResultPlain {
		logLyricsf("fallback to cached plain after unavailable %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(cachedPlain))
		result = cachedPlain
	} else if lyricsResultRank(cachedPlain.Kind) > lyricsResultRank(result.Kind) {
		logLyricsf("keep higher-ranked cached result %s fresh=%s cached=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result), lyricsResultSummary(cachedPlain))
		result = cachedPlain
	}
	result = enrichLyricsResult(result)
	if err == nil && (lyricsResultAvailable(result) || result.Kind == lyricsResultUnavailable) {
		logLyricsf("store cache %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result))
		storeKey := cacheKey
		if result.Kind == lyricsResultUnavailable {
			storeKey = requestKey
		}
		client.storeLyricsCache(storeKey, result)
	}
	client.completeLyricsFetch(requestKey, call, result, err)
	logLyricsf("complete %s result=%s err=%v", lyricsInfoSummary(normalized), lyricsResultSummary(result), err)
	return result, err
}

func enrichLyricsResult(result LyricsResult) LyricsResult {
	result.Source = strings.TrimSpace(result.Source)
	result.ProviderID = strings.ToLower(strings.TrimSpace(result.ProviderID))
	result.ProviderTrackID = strings.TrimSpace(result.ProviderTrackID)
	result.Attribution = strings.TrimSpace(result.Attribution)
	if result.ProviderID == "" {
		if strings.EqualFold(result.Source, "LRCLib") {
			result.ProviderID = lyricsProviderLRCLib
		} else if result.Source != "" {
			result.ProviderID = "youtube_music"
		}
	}
	if result.Attribution == "" {
		if result.ProviderID == lyricsProviderLRCLib {
			result.Attribution = lyricsAttributionLRCLib
		} else {
			result.Attribution = result.Source
		}
	}
	if result.Confidence < 0 {
		result.Confidence = 0
	} else if result.Confidence > 100 {
		result.Confidence = 100
	}
	if result.Confidence == 0 && result.ProviderID == "youtube_music" && lyricsResultAvailable(result) {
		result.Confidence = 100
	}
	if result.TimingQuality == "" {
		result.TimingQuality = lyricsResultTimingQuality(result)
	}
	return result
}

func lyricsResultTimingQuality(result LyricsResult) string {
	if result.TimingQuality != "" {
		return result.TimingQuality
	}
	switch result.Kind {
	case lyricsResultPlain:
		return "plain"
	case lyricsResultSynced:
		quality := "line"
		for _, line := range result.Lines {
			for _, word := range line.Words {
				if len(word.Syllables) > 0 {
					return "syllable"
				}
				quality = "word"
			}
		}
		return quality
	default:
		return ""
	}
}

func lyricsTimingQualityRank(value string) int {
	switch value {
	case "syllable":
		return 4
	case "word":
		return 3
	case "line":
		return 2
	case "plain":
		return 1
	default:
		return 0
	}
}

func lyricsResultTimingRank(result LyricsResult) int {
	return lyricsTimingQualityRank(lyricsResultTimingQuality(result))
}

func lyricsSoftWaitForResult(result LyricsResult, plainOnly bool) (time.Duration, bool) {
	if !lyricsResultAvailable(result) {
		return 0, false
	}
	switch result.Kind {
	case lyricsResultSynced:
		if lyricsResultTimingRank(result) <= lyricsTimingQualityRank("line") {
			return lyricsLineTimingUpgradeWait, true
		}
		return lyricsTimingUpgradeWait, true
	case lyricsResultPlain:
		// In synced mode, plain lyrics are only a fallback. Do not let their
		// latency budget cancel another provider that may still return timing.
		if !plainOnly {
			return 0, false
		}
		return lyricsProviderSoftWait, true
	default:
		return 0, false
	}
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
	result, lrclibErr := searchLyricsQueryVariants(
		ctx,
		normalized,
		func(queryCtx context.Context, query LyricsSearchInfo) (LyricsResult, error) {
			return client.searchLRCLibLyricsWithModeResult(queryCtx, query, true)
		},
	)
	if result.Kind == lyricsResultPlain {
		logLyricsf("plain lrclib selected %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result))
		return result, nil
	}
	if firstErr != nil {
		logLyricsf("plain providers failed %s ytmusicErr=%v lrclib=%s", lyricsInfoSummary(normalized), firstErr, lyricsResultSummary(result))
		return LyricsResult{}, errors.Join(firstErr, lrclibErr)
	}
	if lrclibErr != nil {
		return LyricsResult{}, lrclibErr
	}
	logLyricsf("plain providers unavailable %s result=%s", lyricsInfoSummary(normalized), lyricsResultSummary(result))
	return result, nil
}

func (client *Client) lyricsProvidersForInfo(info LyricsSearchInfo) ([]lyricsProvider, error) {
	if !videoIDPattern.MatchString(info.VideoID) {
		if info.Title == "" {
			return nil, fmt.Errorf("invalid youtube video id")
		}
		providers := []lyricsProvider{{
			name:   "LRCLib",
			search: lyricsVariantProvider(client.lyricsLRCLibProviderForMode(info.PlainOnly)),
		}}
		if !info.PlainOnly {
			providers = append(providers, lyricsProvider{
				name:   "AMLL",
				search: lyricsVariantProvider(client.searchAMLLLyricsProvider),
			})
		}
		logLyricsf("providers %s names=%s reason=no-valid-video-id", lyricsInfoSummary(info), lyricsProviderNames(providers))
		return providers, nil
	}

	providers := []lyricsProvider{{
		name:   "YTMusic",
		search: client.lyricsYTMusicProviderForMode(info.PlainOnly),
	}}
	if !info.PlainOnly && strings.TrimSpace(info.Title) != "" {
		providers = append(providers, lyricsProvider{
			name:   "AMLL",
			search: lyricsVariantProvider(client.searchAMLLLyricsProvider),
		})
		providers = append(providers, lyricsProvider{
			name:   "LRCLib",
			search: lyricsVariantProvider(client.searchLRCLibLyricsProvider),
		})
	}
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.name)
	}
	logLyricsf("providers %s names=%s", lyricsInfoSummary(info), strings.Join(names, ","))
	return providers, nil
}

func lyricsProviderNames(providers []lyricsProvider) string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.name)
	}
	return strings.Join(names, ",")
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

func lyricsVariantProvider(search func(context.Context, LyricsSearchInfo) (LyricsResult, error)) func(context.Context, LyricsSearchInfo) (LyricsResult, error) {
	return func(ctx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
		return searchLyricsQueryVariants(ctx, info, search)
	}
}

func searchLyricsQueryVariants(
	ctx context.Context,
	info LyricsSearchInfo,
	search func(context.Context, LyricsSearchInfo) (LyricsResult, error),
) (LyricsResult, error) {
	queries := lyricsSearchQueries(info)
	if err := ctx.Err(); err != nil {
		return LyricsResult{}, wrapRequestError(err)
	}

	var best lyricsQueryResult
	foundBest := false
	errs := make([]error, 0, len(queries))
	recordResult := func(result lyricsQueryResult) {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("lyrics query %d: %w", result.queryIndex, result.err))
			return
		}
		if !lyricsResultAvailable(result.result) {
			return
		}
		if !foundBest || isBetterLyricsQueryResult(result, best) {
			best = result
			foundBest = true
		}
	}

	canonicalResult, canonicalErr := search(ctx, queries[0])
	recordResult(lyricsQueryResult{queryIndex: 0, result: canonicalResult, err: canonicalErr})
	if canonicalErr == nil && lyricsQueryResultTerminal(canonicalResult, info.PlainOnly) {
		return canonicalResult, nil
	}
	if len(queries) == 1 {
		if foundBest {
			return best.result, nil
		}
		if len(errs) > 0 {
			return LyricsResult{}, errors.Join(errs...)
		}
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}
	if err := ctx.Err(); err != nil {
		if foundBest {
			return best.result, nil
		}
		return LyricsResult{}, wrapRequestError(err)
	}

	variantCtx, cancelVariants := context.WithCancel(ctx)
	defer cancelVariants()
	variantCount := len(queries) - 1
	results := make(chan lyricsQueryResult, variantCount)
	workers := make(chan struct{}, min(variantCount, maxLyricsVariantWorkers))
	for queryIndex, query := range queries[1:] {
		queryIndex++
		go func(index int, queryInfo LyricsSearchInfo) {
			select {
			case workers <- struct{}{}:
				defer func() { <-workers }()
			case <-variantCtx.Done():
				results <- lyricsQueryResult{queryIndex: index, err: variantCtx.Err()}
				return
			}
			result, err := search(variantCtx, queryInfo)
			results <- lyricsQueryResult{queryIndex: index, result: result, err: err}
		}(queryIndex, query)
	}

	for received := 0; received < variantCount; received++ {
		select {
		case result := <-results:
			recordResult(result)
		case <-ctx.Done():
			cancelVariants()
			for {
				select {
				case result := <-results:
					recordResult(result)
				default:
					if foundBest {
						return best.result, nil
					}
					return LyricsResult{}, wrapRequestError(ctx.Err())
				}
			}
		}
	}
	if foundBest {
		return best.result, nil
	}
	if len(errs) > 0 {
		return LyricsResult{}, errors.Join(errs...)
	}
	return LyricsResult{Kind: lyricsResultUnavailable}, nil
}

func lyricsQueryResultTerminal(result LyricsResult, plainOnly bool) bool {
	if !lyricsResultAvailable(result) {
		return false
	}
	if plainOnly {
		return result.Kind == lyricsResultPlain
	}
	return result.Kind == lyricsResultSynced
}

func isBetterLyricsQueryResult(candidate lyricsQueryResult, current lyricsQueryResult) bool {
	if candidateRank, currentRank := lyricsResultRank(candidate.result.Kind), lyricsResultRank(current.result.Kind); candidateRank != currentRank {
		return candidateRank > currentRank
	}
	if candidateTimingRank, currentTimingRank := lyricsResultTimingRank(candidate.result), lyricsResultTimingRank(current.result); candidateTimingRank != currentTimingRank {
		return candidateTimingRank > currentTimingRank
	}
	if candidate.result.Confidence != current.result.Confidence {
		return candidate.result.Confidence > current.result.Confidence
	}
	return candidate.queryIndex < current.queryIndex
}

func lyricsSearchQueries(info LyricsSearchInfo) []LyricsSearchInfo {
	info = normalizeLyricsSearchInfo(info)
	canonical := info
	canonical.SearchVariants = nil
	queries := make([]LyricsSearchInfo, 0, 1+len(info.SearchVariants))
	queries = append(queries, canonical)
	for _, variant := range info.SearchVariants {
		query := canonical
		if variant.Title != "" {
			query.Title = variant.Title
		}
		if variant.Artist != "" {
			query.Artist = variant.Artist
		}
		queries = append(queries, query)
	}
	return queries
}

func (client *Client) fetchLyricsFromProviders(ctx context.Context, info LyricsSearchInfo, providers []lyricsProvider) (LyricsResult, error) {
	if len(providers) == 0 {
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}

	providerCtx, cancelProviders := context.WithCancel(ctx)
	defer cancelProviders()
	resultCh := make(chan lyricsProviderResult, len(providers))
	for index, provider := range providers {
		go func(providerIndex int, provider lyricsProvider) {
			providerResult := lyricsProviderResult{
				provider:      provider.name,
				providerIndex: providerIndex,
			}
			defer func() { resultCh <- providerResult }()
			startedAt := time.Now()
			logLyricsf("provider start name=%s %s", provider.name, lyricsInfoSummary(info))
			result, err := provider.search(providerCtx, info)
			logLyricsf("provider done name=%s elapsed=%s %s result=%s err=%v", provider.name, time.Since(startedAt).Round(time.Millisecond), lyricsInfoSummary(info), lyricsResultSummary(result), err)
			providerResult.result = result
			providerResult.err = err
		}(index, provider)
	}

	var best lyricsProviderResult
	var foundBest bool
	providerErrors := make([]error, len(providers))
	var softTimer *time.Timer
	var softTimerCh <-chan time.Time
	var softTimerWait time.Duration
	defer func() {
		if softTimer != nil {
			softTimer.Stop()
		}
	}()
	for received := 0; received < len(providers); received++ {
		select {
		case result := <-resultCh:
			if result.err != nil && result.providerIndex >= 0 && result.providerIndex < len(providerErrors) {
				providerErrors[result.providerIndex] = fmt.Errorf("%s lyrics provider: %w", result.provider, result.err)
			}
			if !foundBest || isBetterLyricsProviderResult(result, best) {
				best = result
				foundBest = true
			}
			// Only syllable timing is terminal quality. Line- or word-synced
			// lyrics get the normal short upgrade window so a richer concurrent
			// provider cannot lose purely because it completed a few milliseconds
			// later.
			if result.err == nil &&
				lyricsResultAvailable(result.result) &&
				lyricsResultTimingRank(result.result) >= lyricsTimingQualityRank("syllable") {
				cancelProviders()
				logLyricsf("provider immediate best name=%s %s result=%s", result.provider, lyricsInfoSummary(info), lyricsResultSummary(result.result))
				return result.result, nil
			}
			// Synced lyrics keep a short timing-quality upgrade window. Plain
			// lyrics only use a latency budget in explicit plain-only mode; in
			// synced mode they must wait for every possible timing provider.
			if wait, shouldWait := lyricsSoftWaitForResult(best.result, info.PlainOnly); received < len(providers)-1 && foundBest && shouldWait {
				if softTimer == nil {
					softTimer = time.NewTimer(wait)
					softTimerCh = softTimer.C
					softTimerWait = wait
				} else if wait < softTimerWait {
					if !softTimer.Stop() {
						select {
						case <-softTimer.C:
						default:
						}
					}
					softTimer.Reset(wait)
					softTimerWait = wait
				}
			}
		case <-softTimerCh:
			cancelProviders()
			logLyricsf("provider soft wait elapsed name=%s %s result=%s", best.provider, lyricsInfoSummary(info), lyricsResultSummary(best.result))
			return best.result, nil
		case <-ctx.Done():
			return LyricsResult{}, wrapRequestError(ctx.Err())
		}
	}

	if foundBest && lyricsResultAvailable(best.result) {
		logLyricsf("provider best name=%s %s result=%s", best.provider, lyricsInfoSummary(info), lyricsResultSummary(best.result))
		return best.result, nil
	}
	if providerErr := errors.Join(providerErrors...); providerErr != nil {
		logLyricsf("provider failed %s err=%v", lyricsInfoSummary(info), providerErr)
		return LyricsResult{}, providerErr
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
	if candidateTimingRank, currentTimingRank := lyricsResultTimingRank(candidate.result), lyricsResultTimingRank(current.result); candidateTimingRank != currentTimingRank {
		return candidateTimingRank > currentTimingRank
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
	return client.searchLRCLibLyricsWithModeResult(ctx, info, false)
}

func (client *Client) searchLRCLibPlainLyricsProvider(ctx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
	return client.searchLRCLibLyricsWithModeResult(ctx, info, true)
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
		normalizeLyricsMatchText(info.Album),
	}
	if info.DurationSeconds > 0 {
		parts = append(parts, strconv.Itoa(int(math.Round(info.DurationSeconds))))
	}
	return strings.Join(parts, "\x00")
}

// lyricsRequestCacheKey scopes in-flight lookups and negative cache entries to
// all metadata that can change fallback-provider matching. Successful video
// lyrics remain cached by video ID through lyricsCacheKey, but a miss observed
// before title/artist/duration enrichment must not suppress a richer lookup.
func lyricsRequestCacheKey(info LyricsSearchInfo, locale string) string {
	info = normalizeLyricsSearchInfo(info)
	cacheKey := lyricsCacheKey(info, locale)
	parts := []string{cacheKey}
	if strings.TrimSpace(info.VideoID) != "" {
		duration := ""
		if info.DurationSeconds > 0 {
			duration = strconv.Itoa(int(math.Round(info.DurationSeconds)))
		}
		parts = append(parts,
			"query",
			normalizeLyricsMatchText(info.Title),
			normalizeLyricsMatchText(info.Artist),
			normalizeLyricsMatchText(info.Album),
			duration,
		)
	}
	if len(info.SearchVariants) == 0 {
		return strings.Join(parts, "\x00")
	}
	parts = append(parts, "variants")
	for _, variant := range info.SearchVariants {
		title := variant.Title
		if title == "" {
			title = info.Title
		}
		artist := variant.Artist
		if artist == "" {
			artist = info.Artist
		}
		parts = append(parts, normalizeLyricsMatchText(title), normalizeLyricsMatchText(artist))
	}
	return strings.Join(parts, "\x00")
}

func (client *Client) cachedLyrics(key string) (LyricsResult, time.Duration, bool) {
	if strings.TrimSpace(key) == "" {
		return LyricsResult{}, 0, false
	}
	now := client.currentTime()
	client.lyricsMu.Lock()
	defer client.lyricsMu.Unlock()
	if client.lyricsCache == nil {
		client.lyricsCache = make(map[string]lyricsCacheEntry)
	}
	entry, ok := client.lyricsCache[key]
	if !ok {
		return LyricsResult{}, 0, false
	}
	if now.Sub(entry.updatedAt) > lyricsCacheTTL {
		delete(client.lyricsCache, key)
		return LyricsResult{}, 0, false
	}
	entry.lastAccess = now
	client.lyricsCache[key] = entry
	return entry.result, max(0, now.Sub(entry.updatedAt)), true
}

func (client *Client) storeLyricsCache(key string, result LyricsResult) {
	if strings.TrimSpace(key) == "" || (!lyricsResultAvailable(result) && result.Kind != lyricsResultUnavailable) {
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

func lyricsCacheEntryFresh(result LyricsResult, age time.Duration) bool {
	if age < 0 {
		age = 0
	}
	switch result.Kind {
	case lyricsResultSynced:
		return age <= lyricsCacheTTL
	case lyricsResultPlain:
		return age <= lyricsPlainFreshTTL
	case lyricsResultUnavailable:
		return age <= lyricsUnavailableTTL
	default:
		return false
	}
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
	info.Title = normalizeLyricsIdentityTitle(info.Title)
	info.Artist = normalizeLyricsIdentityArtist(info.Artist)
	info.Album = strings.TrimSpace(info.Album)
	info.SearchVariants = normalizeLyricsSearchVariants(info.SearchVariants, info.Title, info.Artist)
	if math.IsNaN(info.DurationSeconds) || math.IsInf(info.DurationSeconds, 0) || info.DurationSeconds < 0 {
		info.DurationSeconds = 0
	}
	return info
}

func normalizeLyricsSearchVariants(variants []LyricsSearchVariant, canonicalTitle string, canonicalArtist string) []LyricsSearchVariant {
	if len(variants) == 0 {
		return nil
	}
	canonicalKey := lyricsSearchVariantKey(canonicalTitle, canonicalArtist)
	seen := map[string]struct{}{canonicalKey: {}}
	result := make([]LyricsSearchVariant, 0, min(len(variants), maxLyricsSearchVariants))
	for _, variant := range variants {
		variant.Title = normalizeLyricsIdentityTitle(variant.Title)
		variant.Artist = normalizeLyricsIdentityArtist(variant.Artist)
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
		key := lyricsSearchVariantKey(effectiveTitle, effectiveArtist)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, variant)
		if len(result) == maxLyricsSearchVariants {
			break
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func lyricsSearchVariantKey(title string, artist string) string {
	return normalizeLyricsMatchText(title) + "\x00" + normalizeLyricsMatchText(artist)
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
	result, _ := client.searchLRCLibLyricsWithModeResult(ctx, info, plainOnly)
	return result
}

func (client *Client) searchLRCLibLyricsWithModeResult(ctx context.Context, info LyricsSearchInfo, plainOnly bool) (LyricsResult, error) {
	info = normalizeLyricsSearchInfo(info)
	if strings.TrimSpace(info.Title) == "" {
		logLyricsf("lrclib skip empty title %s", lyricsInfoSummary(info))
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, lrcLibTimeout)
	defer cancel()

	resultCh := make(chan lrcLibLookupResult, 2)
	go func() {
		logLyricsf("lrclib exact lookup start %s", lyricsInfoSummary(info))
		result, confidence, definitive, err := client.getLRCLibExactLyrics(lookupCtx, info, plainOnly)
		resultCh <- lrcLibLookupResult{
			name:       "exact",
			result:     result,
			confidence: confidence,
			definitive: definitive,
			err:        err,
		}
	}()
	go func() {
		logLyricsf("lrclib search lookup start %s", lyricsInfoSummary(info))
		result, confidence, definitive, err := client.searchLRCLibSearchLyrics(lookupCtx, info, plainOnly)
		resultCh <- lrcLibLookupResult{
			name:       "search",
			result:     result,
			confidence: confidence,
			definitive: definitive,
			err:        err,
		}
	}()

	var exact lrcLibLookupResult
	var search lrcLibLookupResult
	var gotExact bool
	var gotSearch bool
	var softTimer *time.Timer
	var softTimerCh <-chan time.Time
	defer func() {
		if softTimer != nil {
			softTimer.Stop()
		}
	}()
	for received := 0; received < 2; received++ {
		select {
		case lookup := <-resultCh:
			switch lookup.name {
			case "exact":
				exact = lookup
				gotExact = true
				logLyricsf("lrclib exact done %s confidence=%.2f result=%s", lyricsInfoSummary(info), exact.confidence, lyricsResultSummary(exact.result))
			case "search":
				search = lookup
				gotSearch = true
				logLyricsf("lrclib search done %s confidence=%.2f result=%s", lyricsInfoSummary(info), search.confidence, lyricsResultSummary(search.result))
			}
			immediateQuality := lookup.err == nil &&
				((plainOnly && lookup.result.Kind == lyricsResultPlain) ||
					(!plainOnly && lyricsResultTimingRank(lookup.result) >= lyricsTimingQualityRank("syllable")))
			if immediateQuality && lookup.confidence >= lrcLibImmediateConfidence {
				cancel()
				logLyricsf("lrclib %s selected %s %s confidence=%.2f result=%s", lookup.name, lookup.result.Kind, lyricsInfoSummary(info), lookup.confidence, lyricsResultSummary(lookup.result))
				return lookup.result, nil
			}
			if wait, shouldWait := lyricsSoftWaitForResult(lookup.result, plainOnly); softTimer == nil && received == 0 && shouldWait {
				softTimer = time.NewTimer(wait)
				softTimerCh = softTimer.C
			}
		case <-softTimerCh:
			cancel()
			logLyricsf("lrclib soft wait elapsed %s exact=%s search=%s", lyricsInfoSummary(info), lyricsResultSummary(exact.result), lyricsResultSummary(search.result))
			return chooseLRCLibLookupResult(info, exact, gotExact, search, gotSearch, nil)
		case <-lookupCtx.Done():
			logLyricsf("lrclib lookup timeout %s exact=%s search=%s gotExact=%t gotSearch=%t err=%v", lyricsInfoSummary(info), lyricsResultSummary(exact.result), lyricsResultSummary(search.result), gotExact, gotSearch, lookupCtx.Err())
			return chooseLRCLibLookupResult(info, exact, gotExact, search, gotSearch, wrapRequestError(lookupCtx.Err()))
		}
	}
	result, err := chooseLRCLibLookupResult(info, exact, gotExact, search, gotSearch, nil)
	if err != nil || lyricsResultAvailable(result) {
		return result, err
	}
	return client.searchLRCLibRelaxedSearchLyrics(lookupCtx, info, plainOnly)
}

func chooseLRCLibLookupResult(info LyricsSearchInfo, exact lrcLibLookupResult, gotExact bool, search lrcLibLookupResult, gotSearch bool, terminalErr error) (LyricsResult, error) {
	exactAvailable := gotExact && exact.err == nil && lyricsResultAvailable(exact.result)
	searchAvailable := gotSearch && search.err == nil && lyricsResultAvailable(search.result)
	if searchAvailable && (!exactAvailable || betterLRCLibLookupResult(search, exact)) {
		logLyricsf("lrclib search selected %s exact=%s search=%s", lyricsInfoSummary(info), lyricsResultSummary(exact.result), lyricsResultSummary(search.result))
		return search.result, nil
	}
	if exactAvailable {
		logLyricsf("lrclib exact fallback selected %s exact=%s search=%s", lyricsInfoSummary(info), lyricsResultSummary(exact.result), lyricsResultSummary(search.result))
		return exact.result, nil
	}
	if (gotExact && exact.err == nil && exact.definitive) ||
		(gotSearch && search.err == nil && search.definitive) {
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}
	errorsFound := make([]error, 0, 3)
	if gotExact && exact.err != nil {
		errorsFound = append(errorsFound, exact.err)
	}
	if gotSearch && search.err != nil {
		errorsFound = append(errorsFound, search.err)
	}
	if terminalErr != nil {
		errorsFound = append(errorsFound, terminalErr)
	}
	if len(errorsFound) == 0 {
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}
	return LyricsResult{Kind: lyricsResultUnavailable}, errors.Join(errorsFound...)
}

func betterLRCLibLookupResult(candidate lrcLibLookupResult, current lrcLibLookupResult) bool {
	return betterLRCLibCandidate(
		lrcLibCandidateMatch{confidence: candidate.confidence},
		lyricsResultTimingRank(candidate.result),
		lrcLibCandidateMatch{confidence: current.confidence},
		lyricsResultTimingRank(current.result),
	)
}

func (client *Client) getLRCLibExactLyrics(ctx context.Context, info LyricsSearchInfo, plainOnly bool) (LyricsResult, float64, bool, error) {
	values := buildLRCLibExactQuery(info)
	if values.Encode() == "" {
		logLyricsf("lrclib exact empty query %s", lyricsInfoSummary(info))
		return LyricsResult{Kind: lyricsResultUnavailable}, 0, false, nil
	}
	requestURL := lrcLibGetURL + "?" + values.Encode()

	var model lrcLibModel
	if err := client.decodeLRCLibJSON(ctx, requestURL, &model); err != nil {
		if isExpectedLRCLibExactMiss(err) {
			return LyricsResult{Kind: lyricsResultUnavailable}, 0, false, nil
		}
		logLyricsf("lrclib exact decode failed %s query=%q", lyricsInfoSummary(info), values.Encode())
		return LyricsResult{Kind: lyricsResultUnavailable}, 0, false, err
	}
	validated, match, ok := bestLRCLibModelForInfoWithModeScored([]lrcLibModel{model}, info, plainOnly)
	if !ok {
		logLyricsf(
			"lrclib exact candidate rejected %s id=%d track=%q artist=%q album=%q duration=%s match=%s",
			lyricsInfoSummary(info),
			model.ID,
			model.TrackName,
			model.ArtistName,
			model.AlbumName,
			formatOptionalFloat(model.Duration),
			match.summary(),
		)
		return LyricsResult{Kind: lyricsResultUnavailable}, 0, true, nil
	}
	result := lrcLibModelLyricsResult(validated, plainOnly)
	result.Confidence = lyricsConfidencePercent(match.confidence)
	logLyricsf(
		"lrclib exact candidate %s id=%d track=%q artist=%q album=%q duration=%s syncedChars=%d plainChars=%d match=%s result=%s",
		lyricsInfoSummary(info),
		model.ID,
		model.TrackName,
		model.ArtistName,
		model.AlbumName,
		formatOptionalFloat(model.Duration),
		len(strings.TrimSpace(model.SyncedLyrics)),
		len(strings.TrimSpace(model.PlainLyrics)),
		match.summary(),
		lyricsResultSummary(result),
	)
	return result, match.confidence, true, nil
}

func (client *Client) searchLRCLibSearchLyrics(ctx context.Context, info LyricsSearchInfo, plainOnly bool) (LyricsResult, float64, bool, error) {
	values := buildLRCLibSearchQuery(info)
	if values.Encode() == "" {
		logLyricsf("lrclib search empty query %s", lyricsInfoSummary(info))
		return LyricsResult{Kind: lyricsResultUnavailable}, 0, false, nil
	}
	requestURL := lrcLibSearchURL + "?" + values.Encode()

	var models []lrcLibModel
	if err := client.decodeLRCLibJSON(ctx, requestURL, &models); err != nil {
		logLyricsf("lrclib search decode failed %s query=%q", lyricsInfoSummary(info), values.Encode())
		return LyricsResult{Kind: lyricsResultUnavailable}, 0, false, err
	}
	total, valid, synced := countLRCLibModels(models)
	logLyricsf("lrclib search candidates %s total=%d valid=%d synced=%d query=%q", lyricsInfoSummary(info), total, valid, synced, values.Encode())

	model, match, ok := bestLRCLibModelForInfoWithModeScored(models, info, plainOnly)
	if !ok {
		logLyricsf("lrclib search no usable candidates %s", lyricsInfoSummary(info))
		return LyricsResult{Kind: lyricsResultUnavailable}, 0, true, nil
	}
	result := lrcLibModelLyricsResult(model, plainOnly)
	result.Confidence = lyricsConfidencePercent(match.confidence)
	logLyricsf(
		"lrclib search candidate selected %s id=%d track=%q artist=%q album=%q duration=%s syncedChars=%d plainChars=%d match=%s result=%s",
		lyricsInfoSummary(info),
		model.ID,
		model.TrackName,
		model.ArtistName,
		model.AlbumName,
		formatOptionalFloat(model.Duration),
		len(strings.TrimSpace(model.SyncedLyrics)),
		len(strings.TrimSpace(model.PlainLyrics)),
		match.summary(),
		lyricsResultSummary(result),
	)
	return result, match.confidence, true, nil
}

// searchLRCLibRelaxedSearchLyrics runs only after both the exact lookup and the
// full LRCLIB search completed without a usable result. The final title-only
// query is manual by default; its sole automatic exception is an exact title
// with a disjoint-script artist and a provider duration within two seconds.
func (client *Client) searchLRCLibRelaxedSearchLyrics(ctx context.Context, info LyricsSearchInfo, plainOnly bool) (LyricsResult, error) {
	variants := buildLRCLibSearchQueryVariants(info)
	if len(variants) <= 1 {
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}

	seenModels := make(map[int]bool, 20)
	for _, variant := range variants[1:] {
		values := buildLRCLibSearchQuery(variant.info)
		var models []lrcLibModel
		if err := client.decodeLRCLibJSON(
			ctx,
			lrcLibSearchURL+"?"+values.Encode(),
			&models,
		); err != nil {
			return LyricsResult{Kind: lyricsResultUnavailable}, err
		}

		uniqueModels := make([]lrcLibModel, 0, len(models))
		for _, model := range models {
			if seenModels[model.ID] {
				continue
			}
			seenModels[model.ID] = true
			uniqueModels = append(uniqueModels, model)
		}
		if variant.titleOnly {
			// Title-only recall is manual by default. The only automatic exception
			// is a disjoint-script artist pair backed by an exact title and a
			// provider duration within two seconds.
			automaticModels := make([]lrcLibModel, 0, len(uniqueModels))
			for _, model := range uniqueModels {
				if titleOnlyLRCLibAutomaticEligible(model, info) {
					automaticModels = append(automaticModels, model)
				}
			}
			uniqueModels = automaticModels
		}

		model, match, ok := bestLRCLibModelForInfoWithModeScored(uniqueModels, info, plainOnly)
		if !ok {
			continue
		}
		result := lrcLibModelLyricsResult(model, plainOnly)
		result.Confidence = lyricsConfidencePercent(match.confidence)
		return result, nil
	}
	return LyricsResult{Kind: lyricsResultUnavailable}, nil
}

func (client *Client) decodeLRCLibJSON(ctx context.Context, requestURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		logLyricsf("lrclib request build failed url=%q err=%v", redactLRCLibURL(requestURL), err)
		return fmt.Errorf("build lrclib request: %w", err)
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
			return wrapRequestError(ctxErr)
		}
		logLyricsf("lrclib request failed url=%q proxy=%q elapsed=%s err=%v", redactLRCLibURL(requestURL), proxyRoute, elapsed, err)
		return wrapRequestError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		logLyricsf("lrclib request non-200 url=%q proxy=%q status=%d elapsed=%s", redactLRCLibURL(requestURL), proxyRoute, resp.StatusCode, time.Since(startedAt).Round(time.Millisecond))
		return &lrcLibHTTPError{StatusCode: resp.StatusCode}
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	err = decoder.Decode(target)
	if err != nil {
		logLyricsf("lrclib decode failed url=%q proxy=%q elapsed=%s err=%v", redactLRCLibURL(requestURL), proxyRoute, time.Since(startedAt).Round(time.Millisecond), err)
		return fmt.Errorf("lrclib response invalid: %w", err)
	}
	logLyricsf("lrclib request ok url=%q proxy=%q elapsed=%s", redactLRCLibURL(requestURL), proxyRoute, time.Since(startedAt).Round(time.Millisecond))
	return nil
}

func isExpectedLRCLibExactMiss(err error) bool {
	var statusErr *lrcLibHTTPError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.StatusCode >= 400 &&
		statusErr.StatusCode < 500 &&
		statusErr.StatusCode != http.StatusRequestTimeout &&
		statusErr.StatusCode != http.StatusTooManyRequests
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
				Kind:            lyricsResultSynced,
				Source:          "LRCLib",
				ProviderID:      lyricsProviderLRCLib,
				ProviderTrackID: strconv.Itoa(model.ID),
				Attribution:     lyricsAttributionLRCLib,
				TimingQuality:   lrcLibModelTimingQuality(model),
				Lines:           lines,
			}
		}
		logLyricsf("lrclib synced parse produced no lines id=%d track=%q artist=%q syncedChars=%d", model.ID, model.TrackName, model.ArtistName, len(strings.TrimSpace(model.SyncedLyrics)))
	}
	if strings.TrimSpace(model.PlainLyrics) != "" {
		text := strings.TrimSpace(model.PlainLyrics)
		return LyricsResult{
			Kind:            lyricsResultPlain,
			Source:          "LRCLib",
			ProviderID:      lyricsProviderLRCLib,
			ProviderTrackID: strconv.Itoa(model.ID),
			Attribution:     lyricsAttributionLRCLib,
			TimingQuality:   "plain",
			Text:            text,
			Lines:           plainLyricLines(text),
		}
	}
	return LyricsResult{Kind: lyricsResultUnavailable}
}

func lyricsConfidencePercent(value float64) int {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int(math.Round(math.Max(0, math.Min(1, value)) * 100))
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
	model, _, ok := bestLRCLibModelForInfoWithModeScored(models, info, plainOnly)
	return model, ok
}

type lrcLibCandidateMatch struct {
	confidence       float64
	titleScore       float64
	artistScore      float64
	albumScore       float64
	durationScore    float64
	durationDiff     float64
	durationCompared bool
	rejection        string
}

func (match lrcLibCandidateMatch) summary() string {
	if match.rejection != "" {
		return fmt.Sprintf(
			"rejected=%q confidence=%.2f title=%.2f artist=%.2f album=%.2f duration=%.2f diff=%.1f",
			match.rejection,
			match.confidence,
			match.titleScore,
			match.artistScore,
			match.albumScore,
			match.durationScore,
			match.durationDiff,
		)
	}
	return fmt.Sprintf(
		"confidence=%.2f title=%.2f artist=%.2f album=%.2f duration=%.2f diff=%.1f",
		match.confidence,
		match.titleScore,
		match.artistScore,
		match.albumScore,
		match.durationScore,
		match.durationDiff,
	)
}

func bestLRCLibModelForInfoWithModeScored(models []lrcLibModel, info LyricsSearchInfo, plainOnly bool) (lrcLibModel, lrcLibCandidateMatch, bool) {
	if !hasLRCLibAutomaticCorroboration(info) {
		return lrcLibModel{}, lrcLibCandidateMatch{
			rejection: "insufficient metadata for automatic match",
		}, false
	}
	var best lrcLibModel
	var bestMatch lrcLibCandidateMatch
	bestRank := 0
	found := false
	var rejected lrcLibCandidateMatch

	for _, model := range models {
		rank := lrcLibModelLyricsRank(model, plainOnly)
		if rank == 0 {
			continue
		}
		match, ok := scoreLRCLibCandidate(model, info)
		if !ok {
			if rejected.rejection == "" || match.confidence > rejected.confidence {
				rejected = match
			}
			continue
		}
		if !found || betterLRCLibCandidate(match, rank, bestMatch, bestRank) {
			best = model
			bestMatch = match
			bestRank = rank
			found = true
		}
	}
	if !found {
		return lrcLibModel{}, rejected, false
	}
	return best, bestMatch, true
}

func hasLRCLibAutomaticCorroboration(info LyricsSearchInfo) bool {
	return strings.TrimSpace(info.Artist) != "" ||
		strings.TrimSpace(info.Album) != "" ||
		(info.DurationSeconds > 0 &&
			!math.IsNaN(info.DurationSeconds) &&
			!math.IsInf(info.DurationSeconds, 0))
}

func lrcLibModelLyricsRank(model lrcLibModel, plainOnly bool) int {
	if model.Instrumental != nil && *model.Instrumental {
		return 0
	}
	if plainOnly {
		if strings.TrimSpace(model.PlainLyrics) != "" {
			return lyricsTimingQualityRank("plain")
		}
		return 0
	}
	return lyricsTimingQualityRank(lrcLibModelTimingQuality(model))
}

// betterLRCLibCandidate only lets richer timing outrank lower-quality lyrics
// inside a narrow confidence band; otherwise the stronger identity match wins.
func betterLRCLibCandidate(candidate lrcLibCandidateMatch, candidateRank int, current lrcLibCandidateMatch, currentRank int) bool {
	const scoreEpsilon = 0.0001
	confidenceDelta := candidate.confidence - current.confidence
	if math.Abs(confidenceDelta) > lrcLibQualityScoreBand {
		return confidenceDelta > 0
	}
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}
	if math.Abs(confidenceDelta) > scoreEpsilon {
		return confidenceDelta > 0
	}
	if candidate.durationCompared != current.durationCompared {
		return candidate.durationCompared
	}
	if candidate.durationCompared && math.Abs(candidate.durationDiff-current.durationDiff) > scoreEpsilon {
		return candidate.durationDiff < current.durationDiff
	}
	return false
}

// scoreLRCLibCandidate validates hard identity conflicts before calculating a
// confidence. One missing field is allowed when another field corroborates the
// title; a bare title is not enough when richer target metadata was supplied.
func scoreLRCLibCandidate(model lrcLibModel, info LyricsSearchInfo) (lrcLibCandidateMatch, bool) {
	info = normalizeLyricsSearchInfo(info)
	match := lrcLibCandidateMatch{}
	totalWeight := 0.0
	weightedScore := 0.0
	hasExpectedCorroboration := false
	hasCandidateCorroboration := false
	crossScriptArtistConflict := false

	targetTitle := strings.TrimSpace(info.Title)
	if targetTitle != "" {
		candidateTitle := normalizeLyricsIdentityTitle(model.TrackName)
		if candidateTitle == "" {
			match.rejection = "missing title"
			return match, false
		}
		var compatible bool
		match.titleScore, compatible = lyricsTitleSimilarity(targetTitle, candidateTitle)
		if !compatible {
			match.rejection = "incompatible title version"
			return match, false
		}
		if match.titleScore < lrcLibMinimumTitleMatch {
			match.rejection = "title mismatch"
			return match, false
		}
		totalWeight += 0.55
		weightedScore += match.titleScore * 0.55
	}

	targetArtist := strings.TrimSpace(info.Artist)
	if targetArtist != "" {
		hasExpectedCorroboration = true
		candidateArtist := normalizeLyricsIdentityArtist(model.ArtistName)
		if candidateArtist == "" {
			totalWeight += 0.25
			match.artistScore = 0.15
			weightedScore += match.artistScore * 0.25
		} else {
			match.artistScore = lyricsArtistSimilarity(targetArtist, candidateArtist)
			if match.artistScore < lrcLibMinimumArtistMatch {
				// Disjoint writing systems cannot establish either equality or a
				// conflict. Keep the artist out of the weighted score and require an
				// exact title plus an independently strong duration match below.
				if match.titleScore >= lrcLibImmediateConfidence &&
					lyricsArtistScriptsDisjoint(targetArtist, candidateArtist) {
					crossScriptArtistConflict = true
				} else {
					match.rejection = "artist mismatch"
					return match, false
				}
			} else {
				totalWeight += 0.25
				hasCandidateCorroboration = true
				weightedScore += match.artistScore * 0.25
			}
		}
	}

	targetAlbum := strings.TrimSpace(info.Album)
	if targetAlbum != "" {
		hasExpectedCorroboration = true
		totalWeight += 0.10
		candidateAlbum := strings.TrimSpace(model.AlbumName)
		if candidateAlbum == "" {
			match.albumScore = 0.25
		} else {
			match.albumScore = lyricsTextSimilarity(normalizeLyricsMatchText(targetAlbum), normalizeLyricsMatchText(candidateAlbum))
			if match.albumScore >= 0.45 {
				hasCandidateCorroboration = true
			}
		}
		weightedScore += match.albumScore * 0.10
	}

	if info.DurationSeconds > 0 {
		hasExpectedCorroboration = true
		totalWeight += 0.20
		durationScore, durationDiff, compared, compatible := lrcLibDurationSimilarity(info.DurationSeconds, model.Duration)
		match.durationScore = durationScore
		match.durationDiff = durationDiff
		match.durationCompared = compared
		if !compatible {
			match.rejection = "duration mismatch"
			return match, false
		}
		if compared {
			hasCandidateCorroboration = true
		} else {
			match.durationScore = 0.35
		}
		weightedScore += match.durationScore * 0.20
	}
	if crossScriptArtistConflict &&
		(!match.durationCompared || match.durationScore < lrcLibImmediateConfidence) {
		match.rejection = "artist mismatch"
		return match, false
	}

	if totalWeight == 0 {
		match.rejection = "no identity metadata"
		return match, false
	}
	match.confidence = weightedScore / totalWeight
	if hasExpectedCorroboration && !hasCandidateCorroboration {
		match.rejection = "missing corroborating metadata"
		return match, false
	}
	if match.confidence < lrcLibMinimumConfidence {
		match.rejection = "low identity confidence"
		return match, false
	}
	return match, true
}

func lrcLibDurationSimilarity(target float64, candidate *float64) (float64, float64, bool, bool) {
	if target <= 0 || math.IsNaN(target) || math.IsInf(target, 0) {
		return 0, 0, false, true
	}
	if candidate == nil || *candidate <= 0 || math.IsNaN(*candidate) || math.IsInf(*candidate, 0) {
		return 0, 0, false, true
	}
	diff := math.Abs(*candidate - target)
	// Six percent absorbs normal metadata drift, while the absolute and relative
	// caps keep short clips and long live recordings from matching by accident.
	tolerance := math.Min(18, math.Max(4, target*0.06))
	tolerance = math.Min(tolerance, math.Max(2, target*0.20))
	if diff > tolerance {
		return 0, diff, true, false
	}
	if diff <= 2 {
		return 1, diff, true, true
	}
	return math.Max(0.45, 1-(diff-2)/(tolerance-2)*0.55), diff, true, true
}

type lyricsTitleMatchParts struct {
	normalized string
	core       string
	modifiers  map[string]bool
}

func lyricsTitleSimilarity(target string, candidate string) (float64, bool) {
	targetParts := splitLyricsTitleMatchParts(target)
	candidateParts := splitLyricsTitleMatchParts(candidate)
	if targetParts.normalized == "" || candidateParts.normalized == "" {
		return 0, false
	}
	if !lyricsTitleVersionsCompatible(targetParts.modifiers, candidateParts.modifiers) {
		return 0, false
	}
	if targetParts.normalized == candidateParts.normalized {
		return 1, true
	}
	if targetParts.core != "" && targetParts.core == candidateParts.core {
		if sameLyricsModifierSet(targetParts.modifiers, candidateParts.modifiers) {
			return 0.96, true
		}
		return 0.92, true
	}
	return lyricsTextSimilarity(targetParts.core, candidateParts.core), true
}

func splitLyricsTitleMatchParts(value string) lyricsTitleMatchParts {
	normalized := normalizeLyricsMatchText(value)
	tokens := strings.Fields(normalized)
	for i, token := range tokens {
		if token == "feat" || token == "featuring" || token == "ft" {
			tokens = tokens[:i]
			break
		}
	}
	parts := lyricsTitleMatchParts{
		normalized: strings.Join(tokens, " "),
		modifiers:  make(map[string]bool),
	}
	// A recognized suffix starts the edition descriptor. This lets titles such
	// as "Song - Live at Wembley" share a core without ignoring live/remix intent.
	modifierIndex := -1
	for index, token := range tokens {
		kind, ok := lyricsTitleModifier(token, index)
		if !ok {
			continue
		}
		parts.modifiers[kind] = true
		if modifierIndex < 0 {
			modifierIndex = index
		}
	}
	coreTokens := tokens
	if modifierIndex > 0 {
		coreTokens = tokens[:modifierIndex]
	}
	parts.core = strings.Join(coreTokens, " ")
	return parts
}

func lyricsTitleModifier(token string, index int) (string, bool) {
	if index == 0 {
		return "", false
	}
	switch token {
	case "live", "concert", "现场", "現場", "现场版", "現場版", "ライブ":
		return "live", true
	case "remix", "rmx", "mix", "混音", "混音版", "リミックス":
		return "remix", true
	case "acoustic", "unplugged", "不插电", "不插電", "アコースティック":
		return "acoustic", true
	case "instrumental", "karaoke", "伴奏", "纯音乐", "純音樂", "インスト", "インストゥルメンタル":
		return "instrumental", true
	case "demo", "デモ", "小样", "小樣":
		return "demo", true
	case "cover", "翻唱", "カバー":
		return "cover", true
	case "sped", "slowed", "nightcore", "加速", "慢速":
		return "tempo", true
	case "remaster", "remastered", "edit", "edited", "radio", "version", "ver", "mono", "stereo", "extended", "deluxe", "original", "重制", "重製", "版本":
		return "edition", true
	default:
		return "", false
	}
}

func lyricsTitleVersionsCompatible(target map[string]bool, candidate map[string]bool) bool {
	for _, modifier := range []string{"live", "remix", "acoustic", "instrumental", "demo", "cover", "tempo"} {
		if target[modifier] != candidate[modifier] {
			return false
		}
	}
	return true
}

func sameLyricsModifierSet(left map[string]bool, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for modifier := range left {
		if !right[modifier] {
			return false
		}
	}
	return true
}

func lyricsArtistSimilarity(target string, candidate string) float64 {
	targetNormalized := normalizeLyricsMatchText(target)
	candidateNormalized := normalizeLyricsMatchText(candidate)
	if targetNormalized == "" || candidateNormalized == "" {
		return 0
	}
	if targetNormalized == candidateNormalized {
		return 1
	}
	targetPrimary := primaryLyricsArtist(target)
	candidatePrimary := primaryLyricsArtist(candidate)
	if targetPrimary != "" && targetPrimary == candidatePrimary {
		return 0.95
	}
	targetOrderedTokens := comparableLyricsArtistTokens(targetNormalized)
	candidateOrderedTokens := comparableLyricsArtistTokens(candidateNormalized)
	if strings.Join(targetOrderedTokens, " ") == strings.Join(candidateOrderedTokens, " ") {
		return 0.95
	}
	targetTokens := uniqueLyricsTokens(strings.Join(targetOrderedTokens, " "))
	candidateTokens := uniqueLyricsTokens(strings.Join(candidateOrderedTokens, " "))
	intersection := lyricsTokenIntersection(targetTokens, candidateTokens)
	if intersection == 0 {
		return lyricsRuneBigramSimilarity(targetNormalized, candidateNormalized) * 0.75
	}
	union := len(targetTokens) + len(candidateTokens) - intersection
	return float64(intersection) / float64(union)
}

func comparableLyricsArtistTokens(normalized string) []string {
	tokens := strings.Fields(normalized)
	if len(tokens) > 1 && tokens[0] == "the" {
		return tokens[1:]
	}
	return tokens
}

func primaryLyricsArtist(value string) string {
	return normalizeLyricsMatchText(primaryLyricsArtistForQuery(value))
}

func lyricsTextSimilarity(left string, right string) float64 {
	if left == "" || right == "" {
		return 0
	}
	if left == right {
		return 1
	}
	leftTokens := uniqueLyricsTokens(left)
	rightTokens := uniqueLyricsTokens(right)
	intersection := lyricsTokenIntersection(leftTokens, rightTokens)
	tokenDice := float64(2*intersection) / float64(len(leftTokens)+len(rightTokens))
	runeDice := lyricsRuneBigramSimilarity(left, right)
	return 0.65*tokenDice + 0.35*runeDice
}

func uniqueLyricsTokens(value string) map[string]bool {
	tokens := make(map[string]bool)
	for _, token := range strings.Fields(value) {
		tokens[token] = true
	}
	return tokens
}

func lyricsTokenIntersection(left map[string]bool, right map[string]bool) int {
	intersection := 0
	for token := range left {
		if right[token] {
			intersection++
		}
	}
	return intersection
}

func lyricsRuneBigramSimilarity(left string, right string) float64 {
	leftRunes := []rune(strings.ReplaceAll(left, " ", ""))
	rightRunes := []rune(strings.ReplaceAll(right, " ", ""))
	if len(leftRunes) == 0 || len(rightRunes) == 0 {
		return 0
	}
	if string(leftRunes) == string(rightRunes) {
		return 1
	}
	if len(leftRunes) == 1 || len(rightRunes) == 1 {
		return 0
	}
	leftBigrams := make(map[string]int, len(leftRunes)-1)
	for index := 0; index < len(leftRunes)-1; index++ {
		leftBigrams[string(leftRunes[index:index+2])]++
	}
	rightBigrams := make(map[string]int, len(rightRunes)-1)
	for index := 0; index < len(rightRunes)-1; index++ {
		rightBigrams[string(rightRunes[index:index+2])]++
	}
	intersection := 0
	for pair, count := range leftBigrams {
		intersection += min(count, rightBigrams[pair])
	}
	return float64(2*intersection) / float64(len(leftRunes)+len(rightRunes)-2)
}

func buildLRCLibSearchQuery(info LyricsSearchInfo) url.Values {
	title := strings.TrimSpace(info.Title)
	artist := strings.TrimSpace(info.Artist)
	album := strings.TrimSpace(info.Album)
	values := url.Values{}
	if title != "" {
		values.Set("track_name", title)
	}
	if artist != "" {
		values.Set("artist_name", artist)
	}
	if album != "" {
		values.Set("album_name", album)
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
	decomposed := norm.NFKD.String(strings.ToLower(strings.TrimSpace(value)))
	var normalized strings.Builder
	normalized.Grow(len(decomposed))
	lastWasSpace := true
	previousBaseWasLatin := false
	for _, character := range decomposed {
		if unicode.Is(unicode.Mn, character) {
			if previousBaseWasLatin {
				continue
			}
			normalized.WriteRune(character)
			lastWasSpace = false
			continue
		}
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			normalized.WriteRune(character)
			lastWasSpace = false
			previousBaseWasLatin = unicode.In(character, unicode.Latin)
			continue
		}
		if !lastWasSpace {
			normalized.WriteByte(' ')
			lastWasSpace = true
		}
		previousBaseWasLatin = false
	}
	return strings.TrimSpace(normalized.String())
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
			textOnly = strings.TrimSpace(lrcWordTimePattern.ReplaceAllString(textOnly, ""))
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
	fillTimedWordEnds(lines)
	return lines
}

func parseLRCWords(text string, offsetMs int) []TimedWord {
	matches := lrcWordTimePattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	words := make([]TimedWord, 0, len(matches))
	for index, match := range matches {
		if len(match) < 8 {
			continue
		}
		startMs, ok := parseLRCTimeMatch(text, match)
		if !ok {
			continue
		}
		contentEnd := len(text)
		if index+1 < len(matches) {
			contentEnd = matches[index+1][0]
		}
		rawText := text[match[1]:contentEnd]
		if strings.TrimSpace(rawText) == "" {
			continue
		}
		endsWithSpace := len(rawText) > 0 && strings.ContainsAny(rawText[len(rawText)-1:], " \t\r\n")
		word := TimedWord{
			StartMs:       max(0, startMs-offsetMs),
			Text:          strings.TrimSpace(rawText),
			EndsWithSpace: &endsWithSpace,
		}
		if index+1 < len(matches) {
			if endMs, valid := parseLRCTimeMatch(text, matches[index+1]); valid {
				word.EndMs = max(word.StartMs, endMs-offsetMs)
			}
		}
		words = append(words, word)
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

func fillTimedWordEnds(lines []LyricLine) {
	for lineIndex := range lines {
		words := lines[lineIndex].Words
		for wordIndex := range words {
			if words[wordIndex].EndMs > words[wordIndex].StartMs {
				continue
			}
			endMs := lines[lineIndex].StartMs + lines[lineIndex].DurationMs
			if wordIndex+1 < len(words) && words[wordIndex+1].StartMs > words[wordIndex].StartMs {
				endMs = words[wordIndex+1].StartMs
			}
			words[wordIndex].EndMs = max(words[wordIndex].StartMs, endMs)
		}
		lines[lineIndex].Words = words
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
