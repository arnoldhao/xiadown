package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type resourceExtractor interface {
	Name() string
	Extractor() string
	PageMetaScript() string
	ExtractMediaFromResponse(resourceAPIResponse) []resourceStructuredMedia
	EnrichPageMeta(map[string]string, []resourceStructuredMedia) map[string]string
	SelectCandidate([]resourceCandidate, map[string]string, time.Time) (resourceCandidate, bool)
	MediaFromCandidate(*LibraryService, string, string, resourceCandidate, map[string]string) resourceMedia
	VerificationRequired(map[string]string, []resourceRejectedCandidate) bool
}

type resourceDefaultSiteRules struct{}
type resourceStructuredMediaOptionsProvider interface {
	MediaOptionsFromStructured(pageURL string, pageDomain string, pageMeta map[string]string, mediaItems []resourceStructuredMedia) []resourceMedia
}
type resourceStructuredMediaAugmenter interface {
	AugmentStructuredMedia(service *LibraryService, pageURL string, pageMeta map[string]string, mediaItems []resourceStructuredMedia) []resourceStructuredMedia
}
type resourceStructuredDataAugmenter interface {
	AugmentStructuredData(ctx context.Context, service *LibraryService, pageURL string, pageMeta map[string]string, mediaItems []resourceStructuredMedia, hints []resourceNoMediaHint) ([]resourceStructuredMedia, []resourceNoMediaHint)
}
type resourceNoMediaHintProvider interface {
	ExtractNoMediaHintsFromResponse(resourceAPIResponse) []resourceNoMediaHint
	NoMediaFailure(pageMeta map[string]string, hints []resourceNoMediaHint, since time.Time) (resourceSniffFailure, bool)
}

type resourceExtractorRegistration struct {
	Domains []string
	Factory func() resourceExtractor
}

var resourceExtractorRegistry = []resourceExtractorRegistration{
	{
		Domains: []string{"douyin.com", "iesdouyin.com"},
		Factory: func() resourceExtractor {
			return resourceDouyinSiteRules{}
		},
	},
	{
		Domains: []string{
			"xiaohongshu.com",
			"rednote.com",
			"xhs.cn",
			"xhslink.com",
			"xhslink.cn",
			"xhsurl.com",
			"rl.ink",
		},
		Factory: func() resourceExtractor {
			return resourceXiaohongshuSiteRules{}
		},
	},
}

func resourceExtractorForURL(rawURL string) resourceExtractor {
	domain := extractRegistrableDomain(rawURL)
	for _, item := range resourceExtractorRegistry {
		for _, registeredDomain := range item.Domains {
			if strings.EqualFold(domain, strings.TrimSpace(registeredDomain)) && item.Factory != nil {
				return item.Factory()
			}
		}
	}
	return resourceDefaultSiteRules{}
}

func (resourceDefaultSiteRules) Name() string {
	return "default"
}

func (resourceDefaultSiteRules) Extractor() string {
	return "resource"
}

func (resourceDefaultSiteRules) PageMetaScript() string {
	return resourceGenericPageMetaScript()
}

func (resourceDefaultSiteRules) ExtractMediaFromResponse(resourceAPIResponse) []resourceStructuredMedia {
	return nil
}

func (resourceDefaultSiteRules) EnrichPageMeta(pageMeta map[string]string, _ []resourceStructuredMedia) map[string]string {
	return pageMeta
}

func (resourceDefaultSiteRules) SelectCandidate(candidates []resourceCandidate, pageMeta map[string]string, _ time.Time) (resourceCandidate, bool) {
	return selectResourceCandidateForPage(candidates, pageMeta)
}

func selectResourceCandidateForPage(candidates []resourceCandidate, pageMeta map[string]string) (resourceCandidate, bool) {
	if !resourcePageHasVideoDimensions(pageMeta) {
		return resourceCandidate{}, false
	}
	if candidate, ok := bestResourceCandidateMatchingVideoSource(candidates, resourceVideoSourcesFromPageMeta(pageMeta)); ok {
		return candidate, true
	}
	return resourceCandidate{}, false
}

func resourcePageHasVideoDimensions(pageMeta map[string]string) bool {
	if parsePageMetaInt(pageMeta, "videoWidth") <= 0 ||
		parsePageMetaInt(pageMeta, "videoHeight") <= 0 {
		return false
	}
	visibleArea, ok := resourcePrimaryVideoVisibleArea(pageMeta)
	return !ok || visibleArea > 0
}

func resourcePrimaryVideoVisibleArea(pageMeta map[string]string) (int, bool) {
	type videoItem struct {
		VisibleArea *int `json:"visibleArea"`
	}
	var items []videoItem
	raw := strings.TrimSpace(pageMeta["videoItems"])
	if raw == "" || json.Unmarshal([]byte(raw), &items) != nil || len(items) == 0 || items[0].VisibleArea == nil {
		return 0, false
	}
	return *items[0].VisibleArea, true
}

func (rules resourceDefaultSiteRules) MediaFromCandidate(_ *LibraryService, pageURL string, pageDomain string, candidate resourceCandidate, pageMeta map[string]string) resourceMedia {
	title := firstNonEmpty(
		resourceCleanGenericTitle(pageMeta["videoTitle"]),
		resourceCleanGenericTitle(pageMeta["jsonTitle"]),
	)
	thumbnailURL := firstNonEmpty(
		strings.TrimSpace(pageMeta["jsonImage"]),
	)
	return buildResourceMedia(pageURL, pageDomain, candidate, pageMeta, resourceMediaMetadata{
		Title:        title,
		Author:       resourceCleanAuthor(pageMeta["jsonAuthor"]),
		ThumbnailURL: thumbnailURL,
		Extractor:    rules.Extractor(),
	})
}

func (resourceDefaultSiteRules) VerificationRequired(map[string]string, []resourceRejectedCandidate) bool {
	return false
}

type resourceMediaMetadata struct {
	Title         string
	Author        string
	ThumbnailURL  string
	Extractor     string
	QualityHeight int
	FormatNote    string
	VCodec        string
	ACodec        string
}

func buildResourceMedia(pageURL string, pageDomain string, candidate resourceCandidate, pageMeta map[string]string, metadata resourceMediaMetadata) resourceMedia {
	contentType := strings.TrimSpace(candidate.mimeType)
	ext := resourceExtension(candidate.url, contentType)
	if ext == "" {
		ext = ".mp4"
	}
	return resourceMedia{
		URL:            strings.TrimSpace(candidate.url),
		PageURL:        strings.TrimSpace(pageURL),
		Kind:           "video",
		Title:          strings.TrimSpace(metadata.Title),
		Author:         strings.TrimSpace(metadata.Author),
		ThumbnailURL:   resourceSecureImageURL(metadata.ThumbnailURL),
		Domain:         extractRegistrableDomain(firstNonEmpty(pageDomain, pageURL, candidate.url)),
		Extractor:      strings.TrimSpace(metadata.Extractor),
		ContentType:    contentType,
		MimeType:       contentType,
		Ext:            ext,
		Width:          parsePageMetaInt(pageMeta, "videoWidth"),
		Height:         parsePageMetaInt(pageMeta, "videoHeight"),
		QualityHeight:  metadata.QualityHeight,
		FormatNote:     strings.TrimSpace(metadata.FormatNote),
		VCodec:         strings.TrimSpace(metadata.VCodec),
		ACodec:         strings.TrimSpace(metadata.ACodec),
		SizeBytes:      candidate.sizeBytes,
		RequestHeaders: normalizeResourceDownloadHeaders(candidate.headers, pageURL),
	}
}

func buildResourceMediaFromStructured(pageURL string, pageDomain string, media resourceStructuredMedia, fallbackExtractor string) resourceMedia {
	contentType := "video/mp4"
	if resourceSniffRawHLSStream(media.VideoURL, "", "") {
		contentType = "application/vnd.apple.mpegurl"
	}
	ext := resourceExtension(media.VideoURL, contentType)
	if ext == "" {
		ext = ".mp4"
	}
	return resourceMedia{
		URL:            strings.TrimSpace(media.VideoURL),
		PageURL:        strings.TrimSpace(firstNonEmpty(media.PageURL, pageURL)),
		Kind:           "video",
		Title:          strings.TrimSpace(media.Title),
		Author:         strings.TrimSpace(media.Author),
		ThumbnailURL:   resourceSecureImageURL(media.ThumbnailURL),
		Domain:         extractRegistrableDomain(firstNonEmpty(pageDomain, pageURL, media.PageURL, media.VideoURL)),
		Extractor:      strings.TrimSpace(fallbackExtractor),
		ContentType:    contentType,
		MimeType:       contentType,
		Ext:            ext,
		Width:          media.Width,
		Height:         media.Height,
		QualityHeight:  media.QualityHeight,
		FormatNote:     strings.TrimSpace(media.FormatNote),
		VCodec:         strings.TrimSpace(media.VCodec),
		ACodec:         strings.TrimSpace(media.ACodec),
		SizeBytes:      media.SizeBytes,
		RequestHeaders: normalizeResourceDownloadHeaders(media.Headers, pageURL),
		Subtitles:      dedupeResourceSubtitles(media.Subtitles),
	}
}

func resourceMediaOptionsForPage(service *LibraryService, extractor resourceExtractor, pageURL string, pageDomain string, candidates []resourceCandidate, pageMeta map[string]string, mediaItems []resourceStructuredMedia, _ time.Time) []resourceMedia {
	if provider, ok := extractor.(resourceStructuredMediaOptionsProvider); ok {
		if medias := provider.MediaOptionsFromStructured(pageURL, pageDomain, pageMeta, mediaItems); len(medias) > 0 {
			return medias
		}
	}
	if medias := resourceGenericMediaOptionsFromStructured(pageURL, pageDomain, extractor, mediaItems); len(medias) > 0 {
		return medias
	}
	candidateOptions := resourceCandidateOptionsForPage(candidates, pageMeta)
	if len(candidateOptions) == 0 {
		return nil
	}
	result := make([]resourceMedia, 0, len(candidateOptions))
	for _, candidate := range candidateOptions {
		media := extractor.MediaFromCandidate(service, pageURL, pageDomain, candidate, pageMeta)
		if strings.TrimSpace(media.URL) != "" {
			result = append(result, media)
		}
	}
	return dedupeResourceMediaOptions(result)
}

func resourceGenericMediaOptionsFromStructured(pageURL string, pageDomain string, extractor resourceExtractor, mediaItems []resourceStructuredMedia) []resourceMedia {
	if len(mediaItems) == 0 {
		return nil
	}
	fallbackExtractor := "resource"
	if extractor != nil {
		fallbackExtractor = extractor.Extractor()
	}
	result := make([]resourceMedia, 0, len(mediaItems))
	for _, item := range mediaItems {
		if strings.TrimSpace(item.VideoURL) == "" {
			continue
		}
		media := buildResourceMediaFromStructured(pageURL, pageDomain, item, fallbackExtractor)
		if strings.TrimSpace(media.URL) != "" {
			result = append(result, media)
		}
	}
	return dedupeResourceMediaOptions(result)
}

func resourceCandidateOptionsForPage(candidates []resourceCandidate, pageMeta map[string]string) []resourceCandidate {
	if len(candidates) == 0 || !resourcePageHasVideoDimensions(pageMeta) {
		return nil
	}
	result := make([]resourceCandidate, 0, len(candidates))
	add := func(candidate resourceCandidate) {
		if strings.TrimSpace(candidate.url) == "" {
			return
		}
		for _, existing := range result {
			if resourceComparableURL(existing.url, false) == resourceComparableURL(candidate.url, false) {
				return
			}
		}
		result = append(result, candidate)
	}
	sources := resourceVideoSourcesFromPageMeta(pageMeta)
	if len(sources) > 0 {
		for _, candidate := range candidates {
			if resourceVideoSourceMatchScore(candidate.url, sources) > 0 {
				add(candidate)
			}
		}
	}
	sort.SliceStable(result, func(left, right int) bool {
		return resourceCandidateBetter(result[left], result[right])
	})
	return result
}

func dedupeResourceMediaOptions(values []resourceMedia) []resourceMedia {
	if len(values) == 0 {
		return nil
	}
	result := make([]resourceMedia, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.URL) == "" {
			continue
		}
		seen := false
		for _, existing := range result {
			if resourceComparableURL(existing.URL, false) == resourceComparableURL(value.URL, false) {
				seen = true
				break
			}
		}
		if !seen {
			result = append(result, value)
		}
	}
	return result
}
