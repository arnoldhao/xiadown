package service

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var resourceQualityHeightPattern = regexp.MustCompile(`(?i)(?:^|[^\d.])(\d{3,4})\s*p(?:\b|[^a-z])`)

func resourceAddressURLs(value any, extraKeys ...string) []string {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(strings.ToLower(trimmed), "http") {
			return []string{trimmed}
		}
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, resourceAddressURLs(item, extraKeys...)...)
		}
		return dedupeResourceStrings(result)
	case map[string]any:
		keys := []string{
			"url_list", "urlList", "UrlList",
			"urls", "Urls",
			"backupUrls", "backup_urls", "BackupUrls",
			"masterUrl", "master_url", "MasterUrl",
			"url", "Url",
			"main_url", "mainUrl", "MainURL",
			"src", "Src",
		}
		keys = append(keys, extraKeys...)
		result := make([]string, 0, 2)
		for _, key := range keys {
			result = append(result, resourceAddressURLs(typed[key], extraKeys...)...)
		}
		return dedupeResourceStrings(result)
	}
	return nil
}

func resourceFirstAddressURL(value any, extraKeys ...string) string {
	urls := resourceAddressURLs(value, extraKeys...)
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func resourceAddressSize(value any) int64 {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if size := resourceAddressSize(item); size > 0 {
				return size
			}
		}
	case map[string]any:
		return firstPositiveInt64(
			resourceInt64Value(typed, "data_size", "dataSize", "DataSize"),
			resourceInt64Value(typed, "size", "Size"),
			resourceInt64Value(typed, "file_size", "fileSize", "filesize", "fileSize"),
		)
	}
	return 0
}

func resourceQualityHeightFromText(values ...string) int {
	for _, value := range values {
		match := resourceQualityHeightPattern.FindStringSubmatch(strings.TrimSpace(value))
		if len(match) != 2 {
			continue
		}
		height, err := strconv.Atoi(match[1])
		if err == nil && height > 0 {
			return height
		}
	}
	return 0
}

func resourceStructuredMediaForPageID(pageMeta map[string]string, mediaItems []resourceStructuredMedia, pageID string) (resourceStructuredMedia, bool) {
	options := resourceStructuredMediaOptionsForPageID(pageMeta, mediaItems, pageID)
	if len(options) == 0 {
		return resourceStructuredMedia{}, false
	}
	return options[0], true
}

func resourceStructuredMediaOptionsForPageID(pageMeta map[string]string, mediaItems []resourceStructuredMedia, pageID string) []resourceStructuredMedia {
	if len(mediaItems) == 0 {
		return nil
	}
	pageID = strings.TrimSpace(pageID)
	result := make([]resourceStructuredMedia, 0, len(mediaItems))
	add := func(media resourceStructuredMedia) {
		if strings.TrimSpace(media.VideoURL) == "" {
			return
		}
		for _, existing := range result {
			if resourceComparableURL(existing.VideoURL, false) == resourceComparableURL(media.VideoURL, false) {
				return
			}
		}
		result = append(result, media)
	}
	if pageID != "" {
		for _, media := range mediaItems {
			mediaID := strings.TrimSpace(media.ID)
			if strings.EqualFold(mediaID, pageID) {
				add(media)
			}
		}
		sort.SliceStable(result, func(left, right int) bool {
			return resourceStructuredMediaBetter(result[left], result[right])
		})
		return result
	}
	sources := resourceVideoSourcesFromPageMeta(pageMeta)
	if len(sources) > 0 {
		for _, media := range mediaItems {
			if resourceVideoSourceMatchScore(media.VideoURL, sources) > 0 {
				add(media)
			}
		}
		if len(result) > 0 {
			sort.SliceStable(result, func(left, right int) bool {
				return resourceStructuredMediaBetter(result[left], result[right])
			})
			return result
		}
	}
	return nil
}

func resourceSamePageURL(left string, right string) bool {
	left = resourceComparableURL(left, true)
	right = resourceComparableURL(right, true)
	return left != "" && right != "" && left == right
}

func resourceStructuredMediaBetter(left resourceStructuredMedia, right resourceStructuredMedia) bool {
	leftPixels := left.Width * left.Height
	rightPixels := right.Width * right.Height
	switch {
	case leftPixels > 0 && rightPixels > 0 && leftPixels != rightPixels:
		return leftPixels > rightPixels
	case left.QualityHeight != right.QualityHeight:
		return left.QualityHeight > right.QualityHeight
	case left.Height != right.Height:
		return left.Height > right.Height
	case left.Width != right.Width:
		return left.Width > right.Width
	case left.SizeBytes != right.SizeBytes:
		return left.SizeBytes > right.SizeBytes
	case strings.EqualFold(left.FormatID, "photo") && !strings.EqualFold(right.FormatID, "photo"):
		return true
	case !strings.EqualFold(left.FormatID, "photo") && strings.EqualFold(right.FormatID, "photo"):
		return false
	case strings.EqualFold(left.VCodec, "h264") && !strings.EqualFold(right.VCodec, "h264"):
		return true
	case !strings.EqualFold(left.VCodec, "h264") && strings.EqualFold(right.VCodec, "h264"):
		return false
	case !left.SeenAt.Equal(right.SeenAt):
		return left.SeenAt.After(right.SeenAt)
	default:
		return false
	}
}

func enrichPageMetaWithStructuredMedia(pageMeta map[string]string, media resourceStructuredMedia, idKey string) map[string]string {
	enriched := cloneStringMap(pageMeta)
	if enriched == nil {
		enriched = map[string]string{}
	}
	setIfEmpty := func(key string, value string) {
		value = strings.TrimSpace(value)
		if value == "" || strings.TrimSpace(enriched[key]) != "" {
			return
		}
		enriched[key] = value
	}
	setIfEmpty("apiVideoURL", media.VideoURL)
	setIfEmpty("apiTitle", media.Title)
	setIfEmpty("apiAuthor", media.Author)
	setIfEmpty("apiImage", media.ThumbnailURL)
	setIfEmpty("jsonTitle", media.Title)
	setIfEmpty("jsonAuthor", media.Author)
	setIfEmpty("jsonImage", media.ThumbnailURL)
	if idKey != "" {
		setIfEmpty(idKey, media.ID)
	}
	if media.Width > 0 {
		setIfEmpty("videoWidth", strconv.Itoa(media.Width))
	}
	if media.Height > 0 {
		setIfEmpty("videoHeight", strconv.Itoa(media.Height))
	}
	if media.SizeBytes > 0 {
		setIfEmpty("apiSizeBytes", strconv.FormatInt(media.SizeBytes, 10))
	}
	if strings.TrimSpace(enriched["videoItems"]) == "" && strings.TrimSpace(media.VideoURL) != "" {
		if data, err := json.Marshal([]map[string]any{{
			"currentSrc":  media.VideoURL,
			"width":       media.Width,
			"height":      media.Height,
			"poster":      media.ThumbnailURL,
			"visibleArea": 1,
		}}); err == nil {
			enriched["videoItems"] = string(data)
		}
	}
	setIfEmpty("videoCurrentSrc", media.VideoURL)
	return enriched
}

func resourceCandidateFromPageMeta(pageMeta map[string]string, pageID string) (resourceCandidate, bool) {
	rawURL := strings.TrimSpace(pageMeta["apiVideoURL"])
	if rawURL == "" || strings.TrimSpace(pageID) == "" {
		return resourceCandidate{}, false
	}
	return resourceCandidate{
		url:       rawURL,
		pageURL:   strings.TrimSpace(pageMeta["location"]),
		mimeType:  "video/mp4",
		status:    200,
		sizeBytes: parsePageMetaInt64(pageMeta, "apiSizeBytes"),
		score:     220,
		seenAt:    time.Now(),
	}, true
}

func dedupeResourceStructuredFormatOptions(items []resourceStructuredMedia) []resourceStructuredMedia {
	if len(items) == 0 {
		return nil
	}
	result := make([]resourceStructuredMedia, 0, len(items))
	seen := make(map[string]int, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.VideoURL) != "" {
			merged := false
			for index := range result {
				if resourceComparableURL(result[index].VideoURL, false) != resourceComparableURL(item.VideoURL, false) {
					continue
				}
				result[index] = resourceMergeStructuredMedia(result[index], item)
				merged = true
				break
			}
			if merged {
				continue
			}
		}
		key := resourceStructuredFormatOptionKey(item)
		if key != "" {
			if index, ok := seen[key]; ok {
				result[index] = resourceMergeStructuredMedia(result[index], item)
				continue
			}
			seen[key] = len(result)
		}
		result = append(result, item)
	}
	return result
}

func resourceMergeStructuredMedia(primary resourceStructuredMedia, secondary resourceStructuredMedia) resourceStructuredMedia {
	merged := primary
	merged.ID = firstNonEmpty(strings.TrimSpace(primary.ID), strings.TrimSpace(secondary.ID))
	merged.PageURL = firstNonEmpty(strings.TrimSpace(primary.PageURL), strings.TrimSpace(secondary.PageURL))
	merged.Title = firstNonEmpty(strings.TrimSpace(primary.Title), strings.TrimSpace(secondary.Title))
	merged.Author = firstNonEmpty(strings.TrimSpace(primary.Author), strings.TrimSpace(secondary.Author))
	merged.ThumbnailURL = firstNonEmpty(strings.TrimSpace(primary.ThumbnailURL), strings.TrimSpace(secondary.ThumbnailURL))
	merged.FormatID = firstNonEmpty(strings.TrimSpace(primary.FormatID), strings.TrimSpace(secondary.FormatID))
	merged.FormatNote = firstNonEmpty(strings.TrimSpace(primary.FormatNote), strings.TrimSpace(secondary.FormatNote))
	merged.VCodec = firstNonEmpty(strings.TrimSpace(primary.VCodec), strings.TrimSpace(secondary.VCodec))
	merged.ACodec = firstNonEmpty(strings.TrimSpace(primary.ACodec), strings.TrimSpace(secondary.ACodec))
	merged.Width = firstPositiveInt(primary.Width, secondary.Width)
	merged.Height = firstPositiveInt(primary.Height, secondary.Height)
	merged.QualityHeight = firstPositiveInt(primary.QualityHeight, secondary.QualityHeight)
	merged.SizeBytes = firstPositiveInt64(primary.SizeBytes, secondary.SizeBytes)
	merged.SourceURL = firstNonEmpty(strings.TrimSpace(primary.SourceURL), strings.TrimSpace(secondary.SourceURL))
	merged.Headers = mergeHeaders(primary.Headers, secondary.Headers)
	merged.Subtitles = dedupeResourceSubtitles(append(append([]resourceSubtitle(nil), primary.Subtitles...), secondary.Subtitles...))
	merged.SeenAt = firstNonZeroTime(primary.SeenAt, secondary.SeenAt)
	if merged.VideoURL == "" {
		merged.VideoURL = strings.TrimSpace(secondary.VideoURL)
	}
	return merged
}

func resourceStructuredFormatOptionKey(media resourceStructuredMedia) string {
	formatID := strings.TrimSpace(media.FormatID)
	formatNote := strings.TrimSpace(media.FormatNote)
	vcodec := strings.ToLower(strings.TrimSpace(media.VCodec))
	acodec := strings.ToLower(strings.TrimSpace(media.ACodec))
	if formatID == "" &&
		formatNote == "" &&
		vcodec == "" &&
		acodec == "" &&
		media.QualityHeight <= 0 &&
		media.Width <= 0 &&
		media.Height <= 0 {
		return ""
	}
	parts := []string{
		formatID,
		formatNote,
		vcodec,
		acodec,
		strconv.Itoa(media.QualityHeight),
		strconv.Itoa(media.Width),
		strconv.Itoa(media.Height),
	}
	return strings.Join(parts, "\x00")
}

func resourceMediaURLLooksVideo(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, ".mp4") ||
		strings.Contains(lower, ".m3u8") ||
		strings.Contains(lower, "mime_type=video") ||
		strings.Contains(lower, "/video") ||
		strings.Contains(lower, "videotx") ||
		strings.Contains(lower, "sns-video") ||
		strings.Contains(lower, "douyinvod")
}
