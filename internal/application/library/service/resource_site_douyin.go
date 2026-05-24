package service

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type resourceDouyinSiteRules struct{}

var resourceDouyinURLKeyPattern = regexp.MustCompile(`v[^_]+_(([^_]+)_(\d+)p_(\d+))`)

const (
	resourceDouyinLiveHintKind        = "douyin_live"
	resourceDouyinVisibleAwemeIDLimit = 1
)

func (resourceDouyinSiteRules) Name() string {
	return "douyin"
}

func (resourceDouyinSiteRules) Extractor() string {
	return "resource:douyin"
}

func (resourceDouyinSiteRules) PageMetaScript() string {
	return resourceDouyinPageMetaScript()
}

func (resourceDouyinSiteRules) ExtractMediaFromResponse(response resourceAPIResponse) []resourceStructuredMedia {
	if len(response.Body) == 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal(response.Body, &root); err != nil {
		return nil
	}
	items := make([]resourceStructuredMedia, 0, 4)
	visited := 0
	resourceDouyinCollectStructuredMedia(root, response, &items, &visited)
	return dedupeResourceStructuredMedia(items)
}

func (resourceDouyinSiteRules) ExtractNoMediaHintsFromResponse(response resourceAPIResponse) []resourceNoMediaHint {
	if len(response.Body) == 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal(response.Body, &root); err != nil {
		return nil
	}
	hints := make([]resourceNoMediaHint, 0, 2)
	visited := 0
	resourceDouyinCollectNoMediaHints(root, response, &hints, &visited)
	return dedupeResourceNoMediaHints(hints)
}

func (resourceDouyinSiteRules) NoMediaFailure(pageMeta map[string]string, hints []resourceNoMediaHint, _ time.Time) (resourceSniffFailure, bool) {
	if resourceDouyinIsLVDetailPage(pageMeta["location"]) {
		return newResourceSniffFailure(resourceSniffFailureUnsupportedDouyinLVDetail, "douyin", resourceSniffFailureActionNone, false, ""), true
	}
	return resourceSniffFailure{}, false
}

func (resourceDouyinSiteRules) EnrichPageMeta(pageMeta map[string]string, mediaItems []resourceStructuredMedia) map[string]string {
	media, ok := resourceDouyinStructuredMediaForPage(pageMeta, mediaItems)
	if !ok {
		return pageMeta
	}
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
	setIfEmpty("awemeID", media.ID)
	setIfEmpty("jsonTitle", media.Title)
	setIfEmpty("jsonAuthor", media.Author)
	setIfEmpty("jsonImage", media.ThumbnailURL)
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

func (resourceDouyinSiteRules) SelectCandidate(candidates []resourceCandidate, pageMeta map[string]string, _ time.Time) (resourceCandidate, bool) {
	pageID := firstNonEmpty(strings.TrimSpace(pageMeta["awemeID"]), resourceDouyinIDFromURL(pageMeta["location"]))
	if apiVideoURL := strings.TrimSpace(pageMeta["apiVideoURL"]); apiVideoURL != "" {
		if candidate, ok := bestResourceCandidateMatchingVideoSource(candidates, []string{apiVideoURL}); ok {
			return candidate, true
		}
		if candidate, ok := resourceDouyinCandidateFromPageMeta(pageMeta); ok {
			return candidate, true
		}
	}
	if pageID != "" {
		return resourceCandidate{}, false
	}
	if !resourcePageHasVideoDimensions(pageMeta) {
		return resourceCandidate{}, false
	}
	sources := resourceVideoSourcesFromPageMeta(pageMeta)
	isRecommendPage := resourceDouyinIsRecommendPage(pageMeta["location"])
	if isRecommendPage && len(resourceDouyinVisibleAwemeIDsFromPageMeta(pageMeta, resourceDouyinVisibleAwemeIDLimit)) > 0 {
		if candidate, ok := bestResourceCandidateMatchingVideoSource(candidates, sources); ok {
			return candidate, true
		}
		return resourceCandidate{}, false
	}
	if candidate, ok := bestResourceCandidateMatchingVideoSource(candidates, sources); ok {
		return candidate, true
	}
	return resourceCandidate{}, false
}

func resourceDouyinCandidateFromPageMeta(pageMeta map[string]string) (resourceCandidate, bool) {
	rawURL := strings.TrimSpace(pageMeta["apiVideoURL"])
	pageID := firstNonEmpty(strings.TrimSpace(pageMeta["awemeID"]), resourceDouyinIDFromURL(pageMeta["location"]))
	if rawURL == "" || pageID == "" {
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

func (rules resourceDouyinSiteRules) MediaFromCandidate(_ *LibraryService, pageURL string, pageDomain string, candidate resourceCandidate, pageMeta map[string]string) resourceMedia {
	titleCandidates := []string{
		resourceCleanDouyinTitle(pageMeta["apiTitle"]),
		resourceCleanDouyinTitle(pageMeta["videoTitle"]),
		resourceCleanDouyinTitle(pageMeta["jsonTitle"]),
	}
	title := firstNonEmpty(titleCandidates...)
	authorCandidates := []string{
		resourceCleanAuthor(pageMeta["apiAuthor"]),
		resourceCleanAuthor(pageMeta["jsonAuthor"]),
	}
	author := firstNonEmpty(authorCandidates...)
	thumbnailCandidates := []string{
		strings.TrimSpace(pageMeta["apiImage"]),
		strings.TrimSpace(pageMeta["jsonImage"]),
	}
	thumbnailURL := firstNonEmpty(thumbnailCandidates...)
	return buildResourceMedia(pageURL, pageDomain, candidate, pageMeta, resourceMediaMetadata{
		Title:        title,
		Author:       author,
		ThumbnailURL: thumbnailURL,
		Extractor:    rules.Extractor(),
	})
}

func (resourceDouyinSiteRules) VerificationRequired(pageMeta map[string]string, rejected []resourceRejectedCandidate) bool {
	return resourceDouyinLooksBlocked(pageMeta, rejected)
}

func resourceDouyinCollectStructuredMedia(value any, response resourceAPIResponse, items *[]resourceStructuredMedia, visited *int) {
	if value == nil || *visited > 1200 {
		return
	}
	*visited++
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			resourceDouyinCollectStructuredMedia(item, response, items, visited)
		}
	case map[string]any:
		if mediaItems := resourceDouyinStructuredMediaFromObject(typed, response); len(mediaItems) > 0 {
			*items = append(*items, mediaItems...)
		}
		for _, child := range typed {
			switch child.(type) {
			case map[string]any, []any:
				resourceDouyinCollectStructuredMedia(child, response, items, visited)
			}
		}
	}
}

func resourceDouyinCollectNoMediaHints(value any, response resourceAPIResponse, hints *[]resourceNoMediaHint, visited *int) {
	if value == nil || *visited > 1200 {
		return
	}
	*visited++
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			resourceDouyinCollectNoMediaHints(item, response, hints, visited)
		}
	case map[string]any:
		if hint, ok := resourceDouyinLiveNoMediaHintFromAweme(typed, response); ok {
			*hints = append(*hints, hint)
		}
		for _, child := range typed {
			switch child.(type) {
			case map[string]any, []any:
				resourceDouyinCollectNoMediaHints(child, response, hints, visited)
			}
		}
	}
}

func resourceDouyinLiveNoMediaHintFromAweme(aweme map[string]any, response resourceAPIResponse) (resourceNoMediaHint, bool) {
	if resourceIntValue(aweme, "aweme_type", "awemeType") != 101 {
		return resourceNoMediaHint{}, false
	}
	cellRoom, hasCellRoom := resourceMapValue(aweme, "cell_room", "cellRoom")
	if !hasCellRoom {
		return resourceNoMediaHint{}, false
	}
	if videoObject, hasVideoObject := resourceMapValue(aweme, "video"); hasVideoObject && len(resourceDouyinVideoFormatItems(videoObject)) > 0 {
		return resourceNoMediaHint{}, false
	}
	id := resourceStringValue(aweme, "aweme_id", "awemeId", "item_id", "itemId", "group_id", "groupId", "id")
	if id == "" {
		return resourceNoMediaHint{}, false
	}
	roomID := resourceDouyinLiveRoomIDFromAweme(aweme, cellRoom)
	authorObject, _ := resourceMapValue(aweme, "author", "user", "owner")
	if len(authorObject) == 0 {
		authorObject, _ = resourceMapValue(cellRoom, "owner", "user", "anchor")
	}
	title := firstNonEmpty(
		resourceCleanDouyinTitle(resourceStringValue(aweme, "desc", "description", "title", "caption")),
		resourceCleanDouyinTitle(resourceStringValue(cellRoom, "title", "room_title", "roomTitle")),
	)
	return resourceNoMediaHint{
		Kind:      resourceDouyinLiveHintKind,
		ID:        id,
		AltIDs:    []string{roomID},
		PageURL:   firstNonEmpty(resourceDouyinLivePageURLFromRoomID(roomID), resourceDouyinPageURLFromID(id)),
		Title:     title,
		Author:    resourceCleanAuthor(resourceStringValue(authorObject, "nickname", "name", "unique_id", "uniqueId", "short_id", "shortId")),
		SourceURL: response.URL,
		SeenAt:    firstNonZeroTime(response.SeenAt, time.Now()),
	}, true
}

func resourceNoMediaHintMatchesID(hint resourceNoMediaHint, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(hint.ID), id) {
		return true
	}
	for _, altID := range hint.AltIDs {
		if strings.EqualFold(strings.TrimSpace(altID), id) {
			return true
		}
	}
	return false
}

func resourceDouyinLiveRoomIDFromAweme(aweme map[string]any, cellRoom map[string]any) string {
	return firstNonEmpty(
		resourceStringValue(cellRoom, "room_id", "roomId", "id_str", "idStr", "id", "webcast_room_id", "webcastRoomId"),
		resourceStringValue(aweme, "room_id", "roomId", "webcast_room_id", "webcastRoomId", "live_room_id", "liveRoomId"),
		resourceStringValue(resourceMapValueOrNil(aweme, "room", "live_room", "liveRoom"), "room_id", "roomId", "id_str", "idStr", "id"),
	)
}

func resourceDouyinLivePageURLFromRoomID(roomID string) string {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return ""
	}
	return "https://live.douyin.com/" + roomID
}

func resourceDouyinStructuredMediaFromObject(object map[string]any, response resourceAPIResponse) []resourceStructuredMedia {
	videoObject, hasVideoObject := resourceMapValue(object, "video")
	if hasVideoObject {
		return resourceDouyinStructuredMediaFromAweme(object, videoObject, response)
	}
	if _, hasPlayAddr := resourceMapValue(object, "play_addr", "playAddr", "download_addr", "downloadAddr"); hasPlayAddr {
		return resourceDouyinStructuredMediaFromVideoObject(object, nil, response)
	}
	return nil
}

func resourceDouyinStructuredMediaFromAweme(aweme map[string]any, video map[string]any, response resourceAPIResponse) []resourceStructuredMedia {
	items := resourceDouyinStructuredMediaFromVideoObject(video, aweme, response)
	if len(items) == 0 {
		return nil
	}
	authorObject, _ := resourceMapValue(aweme, "author", "user", "owner")
	awemeID := resourceStringValue(aweme, "aweme_id", "awemeId", "item_id", "itemId", "group_id", "groupId", "id")
	title := resourceCleanDouyinTitle(resourceStringValue(aweme, "desc", "description", "title", "caption"))
	author := resourceCleanAuthor(resourceStringValue(authorObject, "nickname", "name", "unique_id", "uniqueId", "short_id", "shortId"))
	shareURL := resourceStringValue(aweme, "share_url", "shareUrl", "url")
	for index := range items {
		items[index].ID = firstNonEmpty(items[index].ID, awemeID)
		items[index].Title = firstNonEmpty(title, items[index].Title)
		items[index].Author = firstNonEmpty(author, items[index].Author)
		items[index].PageURL = firstNonEmpty(shareURL, resourceDouyinPageURLFromID(items[index].ID), items[index].PageURL)
	}
	return items
}

func resourceDouyinStructuredMediaFromVideoObject(video map[string]any, parent map[string]any, response resourceAPIResponse) []resourceStructuredMedia {
	formatItems := resourceDouyinVideoFormatItems(video)
	if len(formatItems) == 0 {
		return nil
	}
	coverURL := firstNonEmpty(
		resourceDouyinAddressURL(resourceAnyValue(video, "cover")),
		resourceDouyinAddressURL(resourceAnyValue(video, "ai_dynamic_cover", "aiDynamicCover")),
		resourceDouyinAddressURL(resourceAnyValue(video, "animated_cover", "animatedCover")),
		resourceDouyinAddressURL(resourceAnyValue(video, "ai_dynamic_cover_bak", "aiDynamicCoverBak")),
		resourceDouyinAddressURL(resourceAnyValue(video, "origin_cover", "originCover")),
		resourceDouyinAddressURL(resourceAnyValue(video, "dynamic_cover", "dynamicCover")),
		resourceDouyinAddressURL(resourceAnyValue(video, "thumbnail")),
		resourceDouyinAddressURL(resourceAnyValue(parent, "cover", "cover_url", "coverUrl")),
	)
	id := firstNonEmpty(
		resourceStringValue(parent, "aweme_id", "awemeId", "item_id", "itemId", "group_id", "groupId", "id"),
		resourceStringValue(video, "uri", "id"),
		resourceDouyinIDFromURL(response.URL),
	)
	pageURL := firstNonEmpty(resourceStringValue(parent, "share_url", "shareUrl", "url"), resourceDouyinPageURLFromID(id), response.PageURL)
	title := resourceCleanDouyinTitle(resourceStringValue(parent, "desc", "description", "title", "caption"))
	author := resourceCleanAuthor(resourceStringValue(resourceMapValueOrNil(parent, "author", "user", "owner"), "nickname", "name", "unique_id", "uniqueId"))
	videoWidth := resourceIntValue(video, "width")
	videoHeight := resourceIntValue(video, "height")
	ratio := resourceDouyinVideoRatio(videoWidth, videoHeight)
	seenAt := firstNonZeroTime(response.SeenAt, time.Now())
	result := make([]resourceStructuredMedia, 0, len(formatItems))
	for _, format := range formatItems {
		width, height := format.Width, format.Height
		if format.UseVideoDimensions {
			width = firstPositiveInt(width, videoWidth)
			height = firstPositiveInt(height, videoHeight)
		}
		if format.UseDownloadRatio {
			height = 0
			if width > 0 && ratio > 0 {
				height = int(float64(width) / ratio)
			}
		}
		if format.UseWebQualityDimensions && format.QualityHeight > 0 {
			width, height = resourceDouyinWebQualityDimensions(videoWidth, videoHeight, format.QualityHeight)
		}
		result = append(result, resourceStructuredMedia{
			ID:            id,
			VideoURL:      format.URL,
			PageURL:       pageURL,
			Title:         title,
			Author:        author,
			ThumbnailURL:  coverURL,
			FormatID:      format.FormatID,
			FormatNote:    format.FormatNote,
			VCodec:        format.VCodec,
			ACodec:        "aac",
			Width:         width,
			Height:        height,
			QualityHeight: format.QualityHeight,
			SizeBytes:     format.SizeBytes,
			SourceURL:     response.URL,
			Headers:       cloneStringMap(response.RequestHeaders),
			SeenAt:        seenAt,
		})
	}
	return result
}

type resourceDouyinFormatItem struct {
	URL                     string
	FormatID                string
	FormatNote              string
	VCodec                  string
	Width                   int
	Height                  int
	QualityHeight           int
	SizeBytes               int64
	Bitrate                 int64
	UseVideoDimensions      bool
	UseDownloadRatio        bool
	UseWebQualityDimensions bool
}

func resourceDouyinVideoFormatItems(video map[string]any) []resourceDouyinFormatItem {
	if len(video) == 0 {
		return nil
	}
	result := make([]resourceDouyinFormatItem, 0, 8)
	addAddress := func(address any, fallbackID string, note string, vcodec string, bitrate int64, useVideoDimensions bool, useDownloadRatio bool, useWebQualityDimensions bool) {
		if address == nil {
			return
		}
		urls := resourceDouyinAddressURLs(address)
		if len(urls) == 0 {
			return
		}
		addressMap, _ := address.(map[string]any)
		formatID, parsedCodec, qualityHeight, parsedBitrate := resourceDouyinAddressFormatMetadata(addressMap, fallbackID)
		if strings.TrimSpace(parsedCodec) != "" {
			vcodec = parsedCodec
		}
		if strings.TrimSpace(vcodec) == "" {
			vcodec = "h264"
		}
		if parsedBitrate > 0 {
			bitrate = parsedBitrate
		}
		for _, rawURL := range urls {
			item := resourceDouyinFormatItem{
				URL:                     rawURL,
				FormatID:                firstNonEmpty(formatID, fallbackID),
				FormatNote:              note,
				VCodec:                  vcodec,
				Width:                   resourceIntValue(addressMap, "width", "Width"),
				Height:                  resourceIntValue(addressMap, "height", "Height"),
				QualityHeight:           qualityHeight,
				SizeBytes:               resourceDouyinAddressSize(address),
				Bitrate:                 bitrate,
				UseVideoDimensions:      useVideoDimensions,
				UseDownloadRatio:        useDownloadRatio,
				UseWebQualityDimensions: useWebQualityDimensions,
			}
			result = append(result, item)
		}
	}
	videoVCodec := "h264"
	if resourceDouyinObjectBool(video, "is_bytevc1", "isBytevc1", "is_h265", "isH265") {
		videoVCodec = "h265"
	}
	addAddress(resourceAnyValue(video, "play_addr", "PlayAddr"), "play_addr", "Direct video", videoVCodec, 0, true, false, false)
	addAddress(resourceAnyValue(video, "download_addr", "DownloadAddr"), "download_addr", resourceDouyinDownloadFormatNote(video), "h264", 0, false, true, false)
	addAddress(resourceAnyValue(video, "play_addr_h264", "playAddrH264"), "play_addr_h264", "Direct video", "h264", 0, false, false, false)
	addAddress(resourceAnyValue(video, "play_addr_bytevc1", "playAddrBytevc1"), "play_addr_bytevc1", "Direct video", "h265", 0, false, false, false)
	addBitrateItems := func(value any) {
		items, ok := value.([]any)
		if !ok {
			return
		}
		for _, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				continue
			}
			address := firstNonNil(
				resourceAnyValue(object, "play_addr", "playAddr", "PlayAddr"),
				resourceAnyValue(object, "download_addr", "downloadAddr", "DownloadAddr"),
			)
			vcodec := "h264"
			if resourceDouyinObjectBool(object, "is_bytevc1", "isBytevc1", "is_h265", "isH265") {
				vcodec = "h265"
			}
			addAddress(address, resourceStringValue(object, "gear_name", "gearName", "format_id", "formatId"), "Playback video", vcodec, resourceInt64Value(object, "bit_rate", "bitRate", "bitrate"), false, false, false)
		}
	}
	addBitrateItems(resourceAnyValue(video, "bit_rate", "bitRate", "bitrate"))
	addWebBitrateItems := func(value any) {
		items, ok := value.([]any)
		if !ok {
			return
		}
		for _, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				continue
			}
			addAddress(resourceAnyValue(object, "PlayAddr"), "", "", "h264", resourceInt64Value(object, "bitrate", "Bitrate", "bit_rate", "bitRate"), false, false, true)
		}
	}
	addWebBitrateItems(resourceAnyValue(video, "bitrateInfo"))
	addAddress(resourceAnyValue(video, "playAddr"), "play", "Direct video", "h264", 0, true, false, false)
	addAddress(resourceAnyValue(video, "downloadAddr"), "download", "watermarked", "h264", 0, false, false, false)
	return dedupeResourceDouyinFormatItems(result)
}

func resourceDouyinAddressFormatMetadata(address map[string]any, fallbackID string) (string, string, int, int64) {
	urlKey := resourceStringValue(address, "url_key", "urlKey", "UrlKey")
	if urlKey == "" {
		return strings.TrimSpace(fallbackID), "", 0, 0
	}
	match := resourceDouyinURLKeyPattern.FindStringSubmatch(urlKey)
	if len(match) != 5 {
		return firstNonEmpty(urlKey, fallbackID), "", 0, 0
	}
	vcodec := strings.TrimSpace(match[2])
	if vcodec == "bytevc1" {
		vcodec = "h265"
	}
	qualityHeight, _ := strconv.Atoi(match[3])
	bitrate, _ := strconv.ParseInt(match[4], 10, 64)
	if bitrate > 0 {
		bitrate /= 1000
	}
	return match[1], vcodec, qualityHeight, bitrate
}

func resourceDouyinDownloadFormatNote(video map[string]any) string {
	if resourceDouyinObjectBool(video, "has_watermark", "hasWatermark") {
		return "Download video, watermarked"
	}
	return "Download video"
}

func resourceDouyinVideoRatio(videoWidth int, videoHeight int) float64 {
	if videoWidth <= 0 || videoHeight <= 0 {
		return 0.5625
	}
	ratio := float64(videoWidth) / float64(videoHeight)
	if ratio <= 0 {
		return 0.5625
	}
	return ratio
}

func resourceDouyinWebQualityDimensions(videoWidth int, videoHeight int, qualityHeight int) (int, int) {
	if qualityHeight <= 0 || videoWidth <= 0 || videoHeight <= 0 {
		return 0, 0
	}
	ratio := resourceDouyinVideoRatio(videoWidth, videoHeight)
	dimension := qualityHeight
	if dimension == 540 {
		dimension = 576
	}
	if ratio < 1 {
		computedHeight := int(float64(dimension) / ratio)
		return dimension, computedHeight - computedHeight%2
	}
	computedWidth := int(float64(dimension) * ratio)
	return computedWidth + computedWidth%2, dimension
}

func resourceDouyinStructuredMediaForPage(pageMeta map[string]string, mediaItems []resourceStructuredMedia) (resourceStructuredMedia, bool) {
	options := resourceDouyinStructuredMediaOptionsForPage(pageMeta, mediaItems)
	if len(options) == 0 {
		return resourceStructuredMedia{}, false
	}
	return options[0], true
}

func resourceDouyinStructuredMediaOptionsForPage(pageMeta map[string]string, mediaItems []resourceStructuredMedia) []resourceStructuredMedia {
	if len(mediaItems) == 0 {
		return nil
	}
	pageID := firstNonEmpty(strings.TrimSpace(pageMeta["awemeID"]), resourceDouyinIDFromURL(pageMeta["location"]))
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
	addByID := func(id string) bool {
		id = strings.TrimSpace(id)
		if id == "" {
			return false
		}
		start := len(result)
		for _, media := range mediaItems {
			if strings.EqualFold(strings.TrimSpace(media.ID), id) {
				add(media)
			}
		}
		return len(result) > start
	}
	if pageID != "" {
		addByID(pageID)
		sort.SliceStable(result, func(left, right int) bool {
			return resourceDouyinStructuredMediaBetter(result[left], result[right])
		})
		return result
	}
	if resourceDouyinIsRecommendPage(pageMeta["location"]) {
		visibleIDs := resourceDouyinVisibleAwemeIDsFromPageMeta(pageMeta, resourceDouyinVisibleAwemeIDLimit)
		if len(visibleIDs) > 0 {
			if addByID(visibleIDs[0]) {
				sort.SliceStable(result, func(left, right int) bool {
					return resourceDouyinStructuredMediaBetter(result[left], result[right])
				})
				return result
			}
			return nil
		}
	}
	sources := resourceVideoSourcesFromPageMeta(pageMeta)
	if len(sources) > 0 {
		bestSourceMedia := resourceStructuredMedia{}
		bestSourceScore := 0
		for _, media := range mediaItems {
			score := resourceVideoSourceMatchScore(media.VideoURL, sources)
			if score > 0 {
				add(media)
				if bestSourceScore == 0 || score > bestSourceScore || (score == bestSourceScore && resourceDouyinStructuredMediaBetter(media, bestSourceMedia)) {
					bestSourceMedia = media
					bestSourceScore = score
				}
			}
		}
		if len(result) > 0 {
			if matchedID := strings.TrimSpace(bestSourceMedia.ID); matchedID != "" {
				result = result[:0]
				for _, media := range mediaItems {
					if strings.EqualFold(strings.TrimSpace(media.ID), matchedID) {
						add(media)
					}
				}
			}
			sort.SliceStable(result, func(left, right int) bool {
				return resourceDouyinStructuredMediaBetter(result[left], result[right])
			})
			return result
		}
	}
	if pageID != "" {
		return nil
	}
	return nil
}

func resourceDouyinIsRecommendPage(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return strings.Contains(strings.ToLower(trimmed), "recommend")
	}
	if !strings.EqualFold(extractRegistrableDomain(trimmed), "douyin.com") {
		return false
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	if path != "" {
		return false
	}
	if _, ok := parsed.Query()["recommend"]; ok {
		return true
	}
	return strings.Contains(strings.ToLower(parsed.RawQuery), "recommend")
}

func resourceDouyinIsLVDetailPage(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return strings.Contains(strings.ToLower(trimmed), "/lvdetail/")
	}
	if !strings.EqualFold(extractRegistrableDomain(trimmed), "douyin.com") {
		return false
	}
	return strings.HasPrefix(strings.ToLower(strings.Trim(parsed.EscapedPath(), "/")), "lvdetail/")
}

func resourceDouyinIsLivePage(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return strings.Contains(strings.ToLower(trimmed), "live.douyin.com")
	}
	if !strings.EqualFold(extractRegistrableDomain(trimmed), "douyin.com") {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "live.douyin.com" || strings.HasSuffix(host, ".live.douyin.com")
}

func resourceDouyinNoMediaHintIDsFromPageMeta(pageMeta map[string]string) []string {
	if len(pageMeta) == 0 {
		return nil
	}
	result := make([]string, 0, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range result {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		result = append(result, value)
	}
	add(pageMeta["awemeID"])
	for _, id := range resourceDouyinVisibleAwemeIDsFromPageMeta(pageMeta, resourceDouyinVisibleAwemeIDLimit) {
		add(id)
	}
	for _, id := range resourceDouyinVisibleLiveIDsFromPageMeta(pageMeta, resourceDouyinVisibleAwemeIDLimit) {
		add(id)
	}
	return result
}

func resourceDouyinVisibleAwemeIDsFromPageMeta(pageMeta map[string]string, limit int) []string {
	return resourceDouyinVisibleIDsFromPageMeta(pageMeta, limit, "visibleAwemeID", "visibleAwemeIDs")
}

func resourceDouyinVisibleLiveIDsFromPageMeta(pageMeta map[string]string, limit int) []string {
	return resourceDouyinVisibleIDsFromPageMeta(pageMeta, limit, "visibleLiveID", "visibleLiveIDs")
}

func resourceDouyinVisibleIDsFromPageMeta(pageMeta map[string]string, limit int, primaryKey string, listKey string) []string {
	if len(pageMeta) == 0 || limit == 0 {
		return nil
	}
	result := make([]string, 0, 2)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range result {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		result = append(result, value)
	}
	add(pageMeta[primaryKey])
	var visibleIDs []string
	if raw := strings.TrimSpace(pageMeta[listKey]); raw != "" && json.Unmarshal([]byte(raw), &visibleIDs) == nil {
		for _, id := range visibleIDs {
			add(id)
			if limit > 0 && len(result) >= limit {
				return result
			}
		}
	}
	if limit > 0 && len(result) > limit {
		return result[:limit]
	}
	return result
}

func resourceDouyinStructuredMediaBetter(left resourceStructuredMedia, right resourceStructuredMedia) bool {
	leftQuality := left.QualityHeight
	rightQuality := right.QualityHeight
	leftPlayable := !strings.EqualFold(strings.TrimSpace(left.VCodec), "bytevc2")
	rightPlayable := !strings.EqualFold(strings.TrimSpace(right.VCodec), "bytevc2")
	switch {
	case leftPlayable != rightPlayable:
		return leftPlayable
	case leftQuality != rightQuality:
		return leftQuality > rightQuality
	case !left.SeenAt.Equal(right.SeenAt):
		return left.SeenAt.After(right.SeenAt)
	default:
		return false
	}
}

func (rules resourceDouyinSiteRules) MediaOptionsFromStructured(pageURL string, pageDomain string, pageMeta map[string]string, mediaItems []resourceStructuredMedia) []resourceMedia {
	items := resourceDouyinStructuredMediaOptionsForPage(pageMeta, mediaItems)
	if len(items) == 0 {
		return nil
	}
	items = dedupeResourceDouyinStructuredFormatOptions(items)
	result := make([]resourceMedia, 0, len(items))
	for _, item := range items {
		media := buildResourceMediaFromStructured(pageURL, pageDomain, item, rules.Extractor())
		if strings.TrimSpace(media.Title) == "" {
			titleCandidates := []string{
				resourceCleanDouyinTitle(pageMeta["apiTitle"]),
				resourceCleanDouyinTitle(pageMeta["videoTitle"]),
				resourceCleanDouyinTitle(pageMeta["jsonTitle"]),
			}
			media.Title = firstNonEmpty(titleCandidates...)
		}
		if strings.TrimSpace(media.Author) == "" {
			authorCandidates := []string{
				resourceCleanAuthor(pageMeta["apiAuthor"]),
				resourceCleanAuthor(pageMeta["jsonAuthor"]),
			}
			media.Author = firstNonEmpty(authorCandidates...)
		}
		if strings.TrimSpace(media.ThumbnailURL) == "" {
			thumbnailCandidates := []string{
				strings.TrimSpace(pageMeta["apiImage"]),
				strings.TrimSpace(pageMeta["jsonImage"]),
			}
			media.ThumbnailURL = resourceSecureImageURL(firstNonEmpty(thumbnailCandidates...))
		}
		result = append(result, media)
	}
	return dedupeResourceMediaOptions(result)
}

func dedupeResourceDouyinStructuredFormatOptions(items []resourceStructuredMedia) []resourceStructuredMedia {
	if len(items) == 0 {
		return nil
	}
	result := make([]resourceStructuredMedia, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := resourceDouyinStructuredFormatOptionKey(item)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		result = append(result, item)
	}
	return result
}

func resourceDouyinStructuredFormatOptionKey(media resourceStructuredMedia) string {
	formatID := strings.TrimSpace(media.FormatID)
	if formatID == "" && media.QualityHeight <= 0 && media.Width <= 0 && media.Height <= 0 && media.SizeBytes <= 0 {
		return ""
	}
	parts := []string{
		formatID,
		strings.TrimSpace(media.FormatNote),
		strings.ToLower(strings.TrimSpace(media.VCodec)),
		strings.ToLower(strings.TrimSpace(media.ACodec)),
		strconv.Itoa(media.QualityHeight),
		strconv.Itoa(media.Width),
		strconv.Itoa(media.Height),
		strconv.FormatInt(media.SizeBytes, 10),
	}
	return strings.Join(parts, "\x00")
}

func dedupeResourceStructuredMedia(items []resourceStructuredMedia) []resourceStructuredMedia {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]resourceStructuredMedia, 0, len(items))
	for _, item := range items {
		key := resourceComparableURL(item.VideoURL, false)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func dedupeResourceNoMediaHints(items []resourceNoMediaHint) []resourceNoMediaHint {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]resourceNoMediaHint, 0, len(items))
	for _, item := range items {
		kind := strings.TrimSpace(item.Kind)
		id := strings.TrimSpace(item.ID)
		if kind == "" || id == "" {
			continue
		}
		key := strings.ToLower(kind) + "\x00" + strings.ToLower(id) + "\x00" + resourceComparableURL(item.SourceURL, false)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func resourceDouyinIDFromURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	if parsed, err := url.Parse(trimmed); err == nil {
		for _, key := range []string{"modal_id", "aweme_id", "awemeId", "gid", "group_id", "groupId", "item_id", "itemId"} {
			if value := strings.TrimSpace(parsed.Query().Get(key)); value != "" {
				return value
			}
		}
		parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
		for index, part := range parts {
			if (part == "video" || part == "note") && index+1 < len(parts) {
				return strings.TrimSpace(parts[index+1])
			}
		}
	}
	return ""
}

func resourceDouyinPageURLFromID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://www.douyin.com/video/" + id
}

func resourceMapValue(object map[string]any, keys ...string) (map[string]any, bool) {
	value := resourceAnyValue(object, keys...)
	typed, ok := value.(map[string]any)
	return typed, ok
}

func resourceMapValueOrNil(object map[string]any, keys ...string) map[string]any {
	value, _ := resourceMapValue(object, keys...)
	return value
}

func resourceAnyValue(object map[string]any, keys ...string) any {
	if len(object) == 0 {
		return nil
	}
	for _, key := range keys {
		if value, ok := object[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func resourceStringValue(object map[string]any, keys ...string) string {
	value := resourceAnyValue(object, keys...)
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}

func resourceIntValue(object map[string]any, keys ...string) int {
	value := resourceInt64Value(object, keys...)
	if value <= 0 || value > int64(^uint(0)>>1) {
		return 0
	}
	return int(value)
}

func resourceInt64Value(object map[string]any, keys ...string) int64 {
	value := resourceAnyValue(object, keys...)
	switch typed := value.(type) {
	case json.Number:
		value, err := typed.Int64()
		if err == nil && value > 0 {
			return value
		}
	case float64:
		if typed > 0 {
			return int64(typed)
		}
	case int64:
		if typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return int64(typed)
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func resourceDouyinAddressURL(value any) string {
	switch typed := value.(type) {
	case string:
		return resourceDouyinResolveAddressURL(typed)
	case []any:
		for _, item := range typed {
			if url := resourceDouyinAddressURL(item); url != "" {
				return url
			}
		}
	case map[string]any:
		for _, key := range []string{"url_list", "urlList", "UrlList", "urls", "Urls"} {
			if list, ok := typed[key].([]any); ok {
				for _, item := range list {
					if url := resourceDouyinAddressURL(item); url != "" {
						return url
					}
				}
			}
		}
		for _, key := range []string{"url", "Url", "main_url", "mainUrl", "MainURL", "src", "Src", "download"} {
			if url := resourceDouyinAddressURL(typed[key]); url != "" {
				return url
			}
		}
	}
	return ""
}

func resourceDouyinAddressURLs(value any) []string {
	switch typed := value.(type) {
	case string:
		if resolved := resourceDouyinResolveAddressURL(typed); resolved != "" {
			return []string{resolved}
		}
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, resourceDouyinAddressURLs(item)...)
		}
		return dedupeResourceStrings(result)
	case map[string]any:
		result := make([]string, 0, 2)
		for _, key := range []string{"url_list", "urlList", "UrlList", "urls", "Urls"} {
			if list, ok := typed[key].([]any); ok {
				result = append(result, resourceDouyinAddressURLs(list)...)
			}
		}
		for _, key := range []string{"url", "Url", "main_url", "mainUrl", "MainURL", "src", "Src", "download"} {
			result = append(result, resourceDouyinAddressURLs(typed[key])...)
		}
		return dedupeResourceStrings(result)
	}
	return nil
}

func resourceDouyinResolveAddressURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	resolved := resourceResolveURL(trimmed, "https://www.douyin.com/")
	parsed, err := url.Parse(strings.TrimSpace(resolved))
	if err != nil || parsed.Host == "" {
		return ""
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return ""
	}
	return parsed.String()
}

func resourceDouyinAddressSize(value any) int64 {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if size := resourceDouyinAddressSize(item); size > 0 {
				return size
			}
		}
	case map[string]any:
		return firstPositiveInt64(
			resourceInt64Value(typed, "data_size", "dataSize", "DataSize", "size", "file_size", "fileSize"),
			resourceInt64Value(typed, "Size"),
		)
	}
	return 0
}

func resourceDouyinObjectBool(object map[string]any, keys ...string) bool {
	value := resourceAnyValue(object, keys...)
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "1" || normalized == "true" || normalized == "yes"
	case float64:
		return typed > 0
	case int:
		return typed > 0
	case int64:
		return typed > 0
	}
	return false
}

func dedupeResourceDouyinFormatItems(items []resourceDouyinFormatItem) []resourceDouyinFormatItem {
	if len(items) == 0 {
		return nil
	}
	result := make([]resourceDouyinFormatItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		seen := false
		for _, existing := range result {
			if resourceComparableURL(existing.URL, false) == resourceComparableURL(item.URL, false) {
				seen = true
				break
			}
		}
		if !seen {
			result = append(result, item)
		}
	}
	return result
}
