package youtubemusic

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	lrcLibGetByIDURL        = "https://lrclib.net/api/get/"
	lyricsProviderLRCLib    = "lrclib"
	lyricsAttributionLRCLib = "LRCLIB contributors"

	// Candidate lookup has at most one canonical identity plus three alternates.
	// Each LRCLIB identity can use four relaxation requests; each unique AMLL
	// title can use one community request and one catalog fallback request.
	maxLRCLibCandidateRequestsPerIdentity = 4
	maxAMLLCandidateRequestsPerTitle      = 2
	maxLyricsCandidateHTTPRequests        = (1 + maxLyricsSearchVariants) *
		(maxLRCLibCandidateRequestsPerIdentity + maxAMLLCandidateRequestsPerTitle)
)

// LyricsCandidate exposes identity evidence separately from lyric capability.
// Automatic matching may reject a candidate while still returning it here for
// an explicit, user-confirmed preview.
type LyricsCandidate struct {
	ProviderID      string  `json:"providerId"`
	ProviderTrackID string  `json:"providerTrackId"`
	Title           string  `json:"title"`
	Artist          string  `json:"artist"`
	Album           string  `json:"album,omitempty"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	Instrumental    bool    `json:"instrumental,omitempty"`
	HasSynced       bool    `json:"hasSynced,omitempty"`
	HasPlain        bool    `json:"hasPlain,omitempty"`
	TimingQuality   string  `json:"timingQuality,omitempty"`
	Attribution     string  `json:"attribution,omitempty"`
	Confidence      int     `json:"confidence"`
	TitleScore      int     `json:"titleScore"`
	ArtistScore     int     `json:"artistScore"`
	AlbumScore      int     `json:"albumScore"`
	DurationScore   int     `json:"durationScore"`
	DurationDiff    float64 `json:"durationDiff,omitempty"`
	Accepted        bool    `json:"accepted"`
	Rejection       string  `json:"rejection,omitempty"`
}

type lrcLibCandidateModelEntry struct {
	model     lrcLibModel
	titleOnly bool
}

type lyricsCandidateSearchTask struct {
	provider string
	search   func(context.Context) ([]LyricsCandidate, error)
}

type lyricsCandidateQueryResult struct {
	index      int
	provider   string
	candidates []LyricsCandidate
	err        error
}

// SearchLyricsCandidates returns a bounded, merged result set with the same
// identity evidence used by automatic matching.
func (client *Client) SearchLyricsCandidates(
	ctx context.Context,
	info LyricsSearchInfo,
) ([]LyricsCandidate, error) {
	info = normalizeLyricsSearchInfo(info)
	queries := lyricsSearchQueries(info)
	if len(queries) == 0 {
		return []LyricsCandidate{}, nil
	}

	merged := make([]LyricsCandidate, 0, 40)
	mergedIndex := make(map[string]int, 40)
	lrclibErrs := make([]error, 0, len(queries))
	absorb := func(results []lyricsCandidateQueryResult) {
		for _, result := range results {
			if result.err != nil {
				if result.provider == lyricsProviderLRCLib {
					lrclibErrs = append(lrclibErrs, result.err)
				}
				continue
			}
			for _, candidate := range result.candidates {
				key := strings.ToLower(strings.TrimSpace(candidate.ProviderID)) + "\x00" +
					strings.TrimSpace(candidate.ProviderTrackID)
				if index, exists := mergedIndex[key]; exists {
					merged[index] = mergeLyricsCandidate(merged[index], candidate)
					continue
				}
				mergedIndex[key] = len(merged)
				merged = append(merged, candidate)
			}
		}
	}
	hasAccepted := func() bool {
		for _, candidate := range merged {
			if candidate.Accepted {
				return true
			}
		}
		return false
	}

	canonicalTitleKey := lyricsCandidateAMLLTitleKey(queries[0])
	canonicalAMLLQueries := make([]LyricsSearchInfo, 0, len(queries))
	for _, query := range queries {
		if !query.PlainOnly && lyricsCandidateAMLLTitleKey(query) == canonicalTitleKey {
			canonicalAMLLQueries = append(canonicalAMLLQueries, query)
		}
	}
	canonicalQuery := queries[0]
	canonicalTasks := []lyricsCandidateSearchTask{{
		provider: lyricsProviderLRCLib,
		search: func(taskCtx context.Context) ([]LyricsCandidate, error) {
			return client.searchLRCLibLyricsCandidates(taskCtx, canonicalQuery)
		},
	}}
	if len(canonicalAMLLQueries) > 0 {
		group := append([]LyricsSearchInfo(nil), canonicalAMLLQueries...)
		canonicalTasks = append(canonicalTasks, lyricsCandidateSearchTask{
			provider: lyricsProviderAMLL,
			search: func(taskCtx context.Context) ([]LyricsCandidate, error) {
				return client.searchAMLLLyricsCandidatesForTitle(taskCtx, group)
			},
		})
	}
	canonicalResults, err := runLyricsCandidateSearchTasks(ctx, canonicalTasks)
	if err != nil {
		return nil, err
	}
	absorb(canonicalResults)
	if hasAccepted() {
		return finalizeLyricsCandidates(merged), nil
	}

	fallbackTasks := make([]lyricsCandidateSearchTask, 0, len(queries)*2)
	seenLRCLib := map[string]bool{lyricsCandidateLRCLibIdentityKey(canonicalQuery): true}
	amllGroups := make(map[string][]LyricsSearchInfo, len(queries))
	amllGroupOrder := make([]string, 0, len(queries))
	for _, query := range queries[1:] {
		lrclibKey := lyricsCandidateLRCLibIdentityKey(query)
		if !seenLRCLib[lrclibKey] {
			seenLRCLib[lrclibKey] = true
			queryCopy := query
			fallbackTasks = append(fallbackTasks, lyricsCandidateSearchTask{
				provider: lyricsProviderLRCLib,
				search: func(taskCtx context.Context) ([]LyricsCandidate, error) {
					return client.searchLRCLibLyricsCandidates(taskCtx, queryCopy)
				},
			})
		}
		amllKey := lyricsCandidateAMLLTitleKey(query)
		if query.PlainOnly || amllKey == "" || amllKey == canonicalTitleKey {
			continue
		}
		if _, exists := amllGroups[amllKey]; !exists {
			amllGroupOrder = append(amllGroupOrder, amllKey)
		}
		amllGroups[amllKey] = append(amllGroups[amllKey], query)
	}
	for _, key := range amllGroupOrder {
		group := append([]LyricsSearchInfo(nil), amllGroups[key]...)
		fallbackTasks = append(fallbackTasks, lyricsCandidateSearchTask{
			provider: lyricsProviderAMLL,
			search: func(taskCtx context.Context) ([]LyricsCandidate, error) {
				return client.searchAMLLLyricsCandidatesForTitle(taskCtx, group)
			},
		})
	}
	if len(fallbackTasks) > 0 {
		fallbackResults, runErr := runLyricsCandidateSearchTasks(ctx, fallbackTasks)
		if runErr != nil {
			return nil, runErr
		}
		absorb(fallbackResults)
	}
	if len(merged) == 0 && len(lrclibErrs) > 0 {
		// Preserve the canonical-only API's historical LRCLIB error surface.
		// Empty successful alternates are not evidence that an earlier provider
		// failure was a definitive miss.
		if len(lrclibErrs) == 1 {
			return nil, lrclibErrs[0]
		}
		return nil, errors.Join(lrclibErrs...)
	}
	return finalizeLyricsCandidates(merged), nil
}

func runLyricsCandidateSearchTasks(ctx context.Context, tasks []lyricsCandidateSearchTask) ([]lyricsCandidateQueryResult, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	results := make(chan lyricsCandidateQueryResult, len(tasks))
	workers := make(chan struct{}, min(len(tasks), maxLyricsVariantWorkers))
	for index, task := range tasks {
		go func(index int, task lyricsCandidateSearchTask) {
			select {
			case workers <- struct{}{}:
				defer func() { <-workers }()
			case <-ctx.Done():
				results <- lyricsCandidateQueryResult{index: index, provider: task.provider, err: ctx.Err()}
				return
			}
			candidates, err := task.search(ctx)
			results <- lyricsCandidateQueryResult{
				index:      index,
				provider:   task.provider,
				candidates: candidates,
				err:        err,
			}
		}(index, task)
	}
	ordered := make([]lyricsCandidateQueryResult, len(tasks))
	for received := 0; received < len(tasks); received++ {
		select {
		case result := <-results:
			ordered[result.index] = result
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return ordered, nil
}

func lyricsCandidateLRCLibIdentityKey(info LyricsSearchInfo) string {
	info = normalizeLyricsSearchInfo(info)
	return lyricsSearchVariantKey(info.Title, info.Artist) + "\x00" +
		normalizeLyricsMatchText(info.Album) + "\x00" +
		strconv.FormatFloat(info.DurationSeconds, 'f', 3, 64) + "\x00" +
		strconv.FormatBool(info.PlainOnly)
}

func lyricsCandidateAMLLTitleKey(info LyricsSearchInfo) string {
	if info.PlainOnly {
		return ""
	}
	return normalizeLyricsMatchText(info.Title)
}

func finalizeLyricsCandidates(candidates []LyricsCandidate) []LyricsCandidate {
	merged := append([]LyricsCandidate(nil), candidates...)
	sort.SliceStable(merged, func(left, right int) bool {
		return betterLyricsCandidate(merged[left], merged[right])
	})
	if len(merged) > 30 {
		merged = merged[:30]
	}
	return merged
}

func mergeLyricsCandidate(current LyricsCandidate, candidate LyricsCandidate) LyricsCandidate {
	best := current
	if betterLyricsCandidate(candidate, current) {
		best = candidate
	}
	best.HasSynced = current.HasSynced || candidate.HasSynced
	best.HasPlain = current.HasPlain || candidate.HasPlain
	if lyricsTimingQualityRank(candidate.TimingQuality) > lyricsTimingQualityRank(best.TimingQuality) {
		best.TimingQuality = candidate.TimingQuality
	}
	if lyricsTimingQualityRank(current.TimingQuality) > lyricsTimingQualityRank(best.TimingQuality) {
		best.TimingQuality = current.TimingQuality
	}
	return best
}

func (client *Client) searchLRCLibLyricsCandidates(
	ctx context.Context,
	info LyricsSearchInfo,
) ([]LyricsCandidate, error) {
	info = normalizeLyricsSearchInfo(info)
	variants := buildLRCLibSearchQueryVariants(info)
	if len(variants) == 0 {
		return []LyricsCandidate{}, nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, lrcLibTimeout)
	defer cancel()

	modelsByID := make(map[int]lrcLibCandidateModelEntry, 20)
	modelOrder := make([]int, 0, 20)
	for _, variant := range variants {
		var models []lrcLibModel
		values := buildLRCLibSearchQuery(variant.info)
		if err := client.decodeLRCLibJSON(
			lookupCtx,
			lrcLibSearchURL+"?"+values.Encode(),
			&models,
		); err != nil {
			return nil, err
		}
		for _, model := range models {
			existing, ok := modelsByID[model.ID]
			if !ok {
				modelsByID[model.ID] = lrcLibCandidateModelEntry{model: model, titleOnly: variant.titleOnly}
				modelOrder = append(modelOrder, model.ID)
				continue
			}
			// A record discovered by any identity-bearing query retains that
			// stronger provenance even if a later title-only query repeats it.
			if existing.titleOnly && !variant.titleOnly {
				existing.titleOnly = false
				modelsByID[model.ID] = existing
			}
		}

		if variant.titleOnly || hasAutoEligibleLRCLibCandidateModels(modelsByID, modelOrder, info) {
			break
		}
	}

	candidates := make([]LyricsCandidate, 0, len(modelOrder))
	for _, modelID := range modelOrder {
		entry := modelsByID[modelID]
		automaticEligible := !entry.titleOnly ||
			titleOnlyLRCLibAutomaticEligible(entry.model, info)
		candidate := lyricsCandidateFromLRCLibModelWithEligibility(
			entry.model,
			info,
			automaticEligible,
		)
		if candidate.ProviderTrackID == "" {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return betterLyricsCandidate(candidates[left], candidates[right])
	})
	if len(candidates) > 20 {
		candidates = candidates[:20]
	}
	return candidates, nil
}

func hasAutoEligibleLRCLibCandidateModels(modelsByID map[int]lrcLibCandidateModelEntry, modelOrder []int, info LyricsSearchInfo) bool {
	for _, modelID := range modelOrder {
		entry := modelsByID[modelID]
		if entry.titleOnly {
			continue
		}
		candidate := lyricsCandidateFromLRCLibModelWithEligibility(entry.model, info, true)
		if candidate.Accepted {
			return true
		}
	}
	return false
}

// TrackLyricsCandidate fetches an explicit LRCLIB record by its stable ID. It
// deliberately does not apply the automatic confidence threshold: reaching
// this method represents a user-confirmed selection.
func (client *Client) TrackLyricsCandidate(
	ctx context.Context,
	providerID string,
	providerTrackID string,
	plainOnly bool,
) (LyricsResult, error) {
	normalizedProvider := strings.ToLower(strings.TrimSpace(providerID))
	if normalizedProvider == lyricsProviderAMLL {
		if plainOnly {
			return LyricsResult{Kind: lyricsResultUnavailable}, nil
		}
		return client.fetchAMLLLyricsCandidate(ctx, providerTrackID, 100, lyricsAttributionAMLL)
	}
	if normalizedProvider != lyricsProviderLRCLib {
		return LyricsResult{}, fmt.Errorf("unsupported lyrics provider")
	}
	recordID, err := strconv.ParseInt(strings.TrimSpace(providerTrackID), 10, 64)
	if err != nil || recordID <= 0 {
		return LyricsResult{}, fmt.Errorf("invalid lyrics candidate id")
	}
	lookupCtx, cancel := context.WithTimeout(ctx, lrcLibTimeout)
	defer cancel()

	var model lrcLibModel
	requestURL := lrcLibGetByIDURL + url.PathEscape(strconv.FormatInt(recordID, 10))
	if err := client.decodeLRCLibJSON(lookupCtx, requestURL, &model); err != nil {
		return LyricsResult{}, err
	}
	if model.ID != int(recordID) {
		return LyricsResult{}, fmt.Errorf("lyrics candidate identity mismatch")
	}
	return enrichLyricsResult(lrcLibModelLyricsResult(model, plainOnly)), nil
}

func lyricsCandidateFromLRCLibModel(
	model lrcLibModel,
	info LyricsSearchInfo,
) LyricsCandidate {
	return lyricsCandidateFromLRCLibModelWithEligibility(model, info, true)
}

func lyricsCandidateFromLRCLibModelWithEligibility(
	model lrcLibModel,
	info LyricsSearchInfo,
	automaticEligible bool,
) LyricsCandidate {
	match, accepted := scoreLRCLibCandidate(model, info)
	if accepted && !automaticEligible {
		accepted = false
		match.rejection = "insufficient metadata for automatic match"
	}
	if accepted && lrcLibModelLyricsRank(model, info.PlainOnly) == 0 {
		accepted = false
		switch {
		case model.Instrumental != nil && *model.Instrumental:
			match.rejection = "instrumental record"
		case info.PlainOnly:
			match.rejection = "plain lyrics unavailable"
		default:
			match.rejection = "lyrics unavailable"
		}
	}
	timingQuality := lrcLibModelTimingQuality(model)
	duration := 0.0
	if model.Duration != nil && !math.IsNaN(*model.Duration) && !math.IsInf(*model.Duration, 0) {
		duration = math.Max(0, *model.Duration)
	}
	return LyricsCandidate{
		ProviderID:      lyricsProviderLRCLib,
		ProviderTrackID: strconv.Itoa(model.ID),
		Title:           strings.TrimSpace(model.TrackName),
		Artist:          strings.TrimSpace(model.ArtistName),
		Album:           strings.TrimSpace(model.AlbumName),
		DurationSeconds: duration,
		Instrumental:    model.Instrumental != nil && *model.Instrumental,
		HasSynced:       strings.TrimSpace(model.SyncedLyrics) != "",
		HasPlain:        strings.TrimSpace(model.PlainLyrics) != "",
		TimingQuality:   timingQuality,
		Attribution:     lyricsAttributionLRCLib,
		Confidence:      lyricsCandidatePercent(match.confidence),
		TitleScore:      lyricsCandidatePercent(match.titleScore),
		ArtistScore:     lyricsCandidatePercent(match.artistScore),
		AlbumScore:      lyricsCandidatePercent(match.albumScore),
		DurationScore:   lyricsCandidatePercent(match.durationScore),
		DurationDiff:    match.durationDiff,
		Accepted:        accepted,
		Rejection:       match.rejection,
	}
}

func lrcLibModelTimingQuality(model lrcLibModel) string {
	if strings.TrimSpace(model.SyncedLyrics) != "" {
		lines := parseLRCLines(model.SyncedLyrics)
		for _, line := range lines {
			if len(line.Words) > 0 {
				return "word"
			}
		}
		if len(lines) > 0 {
			return "line"
		}
	}
	if strings.TrimSpace(model.PlainLyrics) != "" {
		return "plain"
	}
	return ""
}

func lyricsCandidatePercent(value float64) int {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int(math.Round(math.Max(0, math.Min(1, value)) * 100))
}

func betterLyricsCandidate(left LyricsCandidate, right LyricsCandidate) bool {
	if left.Accepted != right.Accepted {
		return left.Accepted
	}
	if left.Confidence != right.Confidence {
		return left.Confidence > right.Confidence
	}
	leftRank := lyricsTimingQualityRank(left.TimingQuality)
	rightRank := lyricsTimingQualityRank(right.TimingQuality)
	if leftRank != rightRank {
		return leftRank > rightRank
	}
	if math.Abs(left.DurationDiff-right.DurationDiff) > 0.001 {
		return left.DurationDiff < right.DurationDiff
	}
	return left.ProviderTrackID < right.ProviderTrackID
}
