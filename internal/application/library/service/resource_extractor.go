package service

import (
	"context"
	"strings"
	"time"
)

// resourceSniffSnapshot is the raw boundary between capture and site extractors.
type resourceSniffSnapshot struct {
	PageURL           string
	PageDomain        string
	PageMeta          map[string]string
	Candidates        []resourceCandidate
	Rejected          []resourceRejectedCandidate
	APIResponses      []resourceAPIResponse
	CapturedSubtitles []resourceSubtitle
	StartedAt         time.Time
}

func newResourceSniffSnapshot(pageURL string, pageMeta map[string]string, candidates []resourceCandidate, rejected []resourceRejectedCandidate, apiResponses []resourceAPIResponse, subtitles []resourceSubtitle, startedAt time.Time) resourceSniffSnapshot {
	return resourceSniffSnapshot{
		PageURL:           pageURL,
		PageDomain:        extractRegistrableDomain(pageURL),
		PageMeta:          cloneStringMap(pageMeta),
		Candidates:        cloneResourceCandidates(candidates),
		Rejected:          cloneResourceRejectedCandidates(rejected),
		APIResponses:      cloneResourceAPIResponses(apiResponses),
		CapturedSubtitles: cloneResourceSubtitles(subtitles),
		StartedAt:         startedAt,
	}
}

func cloneResourceCandidates(values []resourceCandidate) []resourceCandidate {
	if len(values) == 0 {
		return nil
	}
	result := make([]resourceCandidate, len(values))
	for index, candidate := range values {
		candidate.headers = cloneStringMap(candidate.headers)
		result[index] = candidate
	}
	return result
}

func cloneResourceRejectedCandidates(values []resourceRejectedCandidate) []resourceRejectedCandidate {
	if len(values) == 0 {
		return nil
	}
	result := make([]resourceRejectedCandidate, len(values))
	for index, candidate := range values {
		candidate.headers = cloneStringMap(candidate.headers)
		result[index] = candidate
	}
	return result
}

func cloneResourceAPIResponses(values []resourceAPIResponse) []resourceAPIResponse {
	if len(values) == 0 {
		return nil
	}
	result := make([]resourceAPIResponse, len(values))
	for index, response := range values {
		response.RequestHeaders = cloneStringMap(response.RequestHeaders)
		response.ResponseHeaders = cloneStringMap(response.ResponseHeaders)
		response.Body = append([]byte(nil), response.Body...)
		result[index] = response
	}
	return result
}

func cloneResourceSubtitles(values []resourceSubtitle) []resourceSubtitle {
	if len(values) == 0 {
		return nil
	}
	result := make([]resourceSubtitle, len(values))
	for index, subtitle := range values {
		subtitle.RequestHeaders = cloneStringMap(subtitle.RequestHeaders)
		result[index] = subtitle
	}
	return result
}

func (service *LibraryService) extractResourceStructuredData(ctx context.Context, extractor resourceExtractor, snapshot resourceSniffSnapshot) ([]resourceStructuredMedia, []resourceNoMediaHint, time.Duration) {
	if extractor == nil {
		extractor = resourceDefaultSiteRules{}
	}
	mediaItems, hints := resourceStructuredDataFromAPIResponses(extractor, snapshot.APIResponses)
	started := time.Now()
	if augmenter, ok := extractor.(resourceStructuredDataAugmenter); ok {
		mediaItems, hints = augmenter.AugmentStructuredData(ctx, service, snapshot.PageURL, snapshot.PageMeta, mediaItems, hints)
	}
	return dedupeResourceStructuredMedia(mediaItems), dedupeResourceNoMediaHints(hints), time.Since(started)
}

func resourceStructuredDataFromAPIResponses(extractor resourceExtractor, responses []resourceAPIResponse) ([]resourceStructuredMedia, []resourceNoMediaHint) {
	if extractor == nil || len(responses) == 0 {
		return nil, nil
	}
	var mediaItems []resourceStructuredMedia
	var hints []resourceNoMediaHint
	var hintProvider resourceNoMediaHintProvider
	if provider, ok := extractor.(resourceNoMediaHintProvider); ok {
		hintProvider = provider
	}
	for _, response := range responses {
		if media, ok := resourceStructuredMediaFromManifestResponse(response); ok {
			mediaItems = append(mediaItems, media)
		}
		for _, media := range extractor.ExtractMediaFromResponse(response) {
			if normalized, ok := normalizeResourceStructuredMedia(media, response); ok {
				mediaItems = append(mediaItems, normalized)
			}
		}
		if hintProvider != nil {
			for _, hint := range hintProvider.ExtractNoMediaHintsFromResponse(response) {
				if normalized, ok := normalizeResourceNoMediaHint(hint, response); ok {
					hints = append(hints, normalized)
				}
			}
		}
	}
	return dedupeResourceStructuredMedia(mediaItems), dedupeResourceNoMediaHints(hints)
}

func resourceStructuredMediaFromManifestResponse(response resourceAPIResponse) (resourceStructuredMedia, bool) {
	if !resourceSniffRawHLSStream(response.URL, response.MimeType, response.ContentType) ||
		!resourceHLSManifestDownloadable(response.Body) {
		return resourceStructuredMedia{}, false
	}
	return normalizeResourceStructuredMedia(resourceStructuredMedia{
		VideoURL:   strings.TrimSpace(response.URL),
		PageURL:    strings.TrimSpace(response.PageURL),
		FormatNote: "HLS VOD",
		Headers:    cloneStringMap(response.RequestHeaders),
		SeenAt:     response.SeenAt,
	}, response)
}

func normalizeResourceStructuredMedia(media resourceStructuredMedia, response resourceAPIResponse) (resourceStructuredMedia, bool) {
	media.VideoURL = strings.TrimSpace(media.VideoURL)
	if media.VideoURL == "" {
		return resourceStructuredMedia{}, false
	}
	media.SourceURL = firstNonEmpty(media.SourceURL, response.URL)
	media.PageURL = firstNonEmpty(media.PageURL, response.PageURL)
	media.Headers = mergeHeaders(media.Headers, response.RequestHeaders)
	if media.SeenAt.IsZero() {
		media.SeenAt = firstNonZeroTime(response.SeenAt, time.Now())
	}
	return media, true
}

func normalizeResourceNoMediaHint(hint resourceNoMediaHint, response resourceAPIResponse) (resourceNoMediaHint, bool) {
	hint.Kind = strings.TrimSpace(hint.Kind)
	hint.ID = strings.TrimSpace(hint.ID)
	hint.AltIDs = dedupeResourceStrings(hint.AltIDs)
	if hint.Kind == "" || hint.ID == "" {
		return resourceNoMediaHint{}, false
	}
	hint.SourceURL = firstNonEmpty(hint.SourceURL, response.URL)
	hint.PageURL = firstNonEmpty(hint.PageURL, response.PageURL)
	if hint.SeenAt.IsZero() {
		hint.SeenAt = firstNonZeroTime(response.SeenAt, time.Now())
	}
	return hint, true
}

func resourcePreflightFailure(ctx context.Context, extractor resourceExtractor, pageURL string, pageMeta map[string]string) (resourceSniffFailure, bool) {
	if extractor == nil {
		extractor = resourceExtractorForURL(pageURL)
	}
	switch extractor.Name() {
	case (resourceDouyinSiteRules{}).Name():
		return resourceDouyinRecommendLoginFailure(ctx, pageURL, pageMeta)
	default:
		return resourceSniffFailure{}, false
	}
}

func resourceKnownExtractor(rawURL string) bool {
	return resourceExtractorForURL(rawURL).Name() != (resourceDefaultSiteRules{}).Name()
}
