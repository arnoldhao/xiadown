package youtubeworkspace

import (
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// innerTubeItemFilter selects the item family a browse surface accepts.
// Most feeds use innerTubeItemsAll; dedicated Shorts and playlist surfaces
// use the corresponding narrow filter so unrelated renderers are ignored.
type innerTubeItemFilter uint8

const (
	innerTubeItemsAll innerTubeItemFilter = iota
	innerTubeItemsVideosOnly
	innerTubeItemsShortsOnly
	innerTubeItemsPlaylistsOnly
)

// innerTubeParseResult is the package-local boundary between an InnerTube
// response and the workspace service. Items preserve response order, are
// deduplicated by their stable YouTube identity, and are optionally limited.
// Continuation is collected independently of the item limit.
type innerTubeParseResult struct {
	Items        []Video
	Continuation string
}

var (
	innerTubePlaylistIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{2,200}$`)
	innerTubeCountPattern      = regexp.MustCompile(`(?i)([0-9]+(?:[.,][0-9]+)*)(?:\s*([kmbt]))?`)
)

// parseInnerTubeItems recursively parses the renderer generations currently
// returned by YouTube's WEB client. A non-positive limit means unlimited.
func parseInnerTubeItems(
	data map[string]any,
	filter innerTubeItemFilter,
	limit int,
) innerTubeParseResult {
	collector := innerTubeCollector{
		filter: filter,
		seen:   make(map[string]struct{}),
	}
	collector.collect(data)
	items := collector.items
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	if items == nil {
		items = []Video{}
	}
	return innerTubeParseResult{
		Items:        items,
		Continuation: collector.continuation,
	}
}

type innerTubeCollector struct {
	filter       innerTubeItemFilter
	items        []Video
	seen         map[string]struct{}
	continuation string
}

func (collector *innerTubeCollector) collect(value any) {
	switch typed := value.(type) {
	case map[string]any:
		collector.collectMap(typed)
	case []any:
		for _, item := range typed {
			collector.collect(item)
		}
	case []map[string]any:
		for _, item := range typed {
			collector.collect(item)
		}
	}
}

func (collector *innerTubeCollector) collectMap(value map[string]any) {
	if collector.continuation == "" {
		collector.continuation = innerTubeContinuationFromMap(value)
	}

	if renderer, ok := innerTubeMap(value["shortsLockupViewModel"]); ok {
		if video, parsed := parseInnerTubeShortsLockup(renderer); parsed {
			collector.append(video)
		}
		return
	}

	if renderer, ok := innerTubeMap(value["richItemRenderer"]); ok {
		if content, exists := renderer["content"]; exists {
			collector.collect(content)
		}
		return
	}

	for _, key := range []string{
		"videoRenderer",
		"gridVideoRenderer",
		"compactVideoRenderer",
		"videoCardRenderer",
		"playlistVideoRenderer",
	} {
		if renderer, ok := innerTubeMap(value[key]); ok {
			if video, parsed := parseInnerTubeVideoRenderer(renderer); parsed {
				collector.append(video)
			}
			return
		}
	}

	for _, key := range []string{"playlistRenderer", "gridPlaylistRenderer"} {
		if renderer, ok := innerTubeMap(value[key]); ok {
			if playlist, parsed := parseInnerTubePlaylistRenderer(renderer); parsed {
				collector.append(playlist)
			}
			return
		}
	}

	if lockup, ok := innerTubeMap(value["lockupViewModel"]); ok {
		switch strings.ToUpper(innerTubeString(lockup["contentType"])) {
		case "LOCKUP_CONTENT_TYPE_VIDEO":
			if video, parsed := parseInnerTubeVideoLockup(lockup); parsed {
				collector.append(video)
			}
		case "LOCKUP_CONTENT_TYPE_PLAYLIST":
			if playlist, parsed := parseInnerTubePlaylistLockup(lockup); parsed {
				collector.append(playlist)
			}
		}
		return
	}

	if _, ok := value["continuationItemRenderer"]; ok {
		return
	}

	keys := make([]string, 0, len(value))
	for key := range value {
		if key == "engagementPanels" || key == "playerOverlays" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		collector.collect(value[key])
	}
}

func (collector *innerTubeCollector) append(item Video) {
	if item.ItemKind == "playlist" {
		if collector.filter == innerTubeItemsVideosOnly ||
			collector.filter == innerTubeItemsShortsOnly ||
			item.PlaylistID == "" {
			return
		}
	} else {
		if collector.filter == innerTubeItemsPlaylistsOnly {
			return
		}
		if collector.filter == innerTubeItemsVideosOnly && item.Short {
			return
		}
		if collector.filter == innerTubeItemsShortsOnly && !item.Short {
			return
		}
	}

	identity := "video:" + item.VideoID
	if item.ItemKind == "playlist" {
		identity = "playlist:" + item.PlaylistID
	}
	if _, duplicate := collector.seen[identity]; duplicate {
		return
	}
	collector.seen[identity] = struct{}{}
	collector.items = append(collector.items, item)
}

func parseInnerTubeVideoRenderer(renderer map[string]any) (Video, bool) {
	videoID := strings.TrimSpace(innerTubeString(renderer["videoId"]))
	if !youtubeVideoIDPattern.MatchString(videoID) {
		return Video{}, false
	}
	title := innerTubeText(renderer["title"])
	if title == "" {
		title = innerTubeText(renderer["headline"])
	}
	if title == "" {
		return Video{}, false
	}

	channel, channelID := innerTubeLegacyOwner(renderer)
	navigation := firstInnerTubeValue(
		renderer["navigationEndpoint"],
		renderer["onTap"],
		renderer["command"],
	)
	short := innerTubeNavigationIsShort(navigation)
	durationLabel := innerTubeText(renderer["lengthText"])
	if durationLabel == "" {
		durationLabel = innerTubeTimeStatusText(renderer["thumbnailOverlays"])
	}
	live := innerTubeHasLiveMarker(renderer["badges"]) ||
		innerTubeHasLiveMarker(renderer["thumbnailOverlays"]) ||
		strings.EqualFold(strings.TrimSpace(durationLabel), "live")
	if live {
		durationLabel = "LIVE"
	}
	viewLabel := firstInnerTubeText(
		renderer["shortViewCountText"],
		renderer["viewCountText"],
	)
	webURL := innerTubeNavigationURL(navigation)
	if webURL == "" {
		if short {
			webURL = "https://www.youtube.com/shorts/" + url.PathEscape(videoID)
		} else {
			webURL = "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID)
		}
	}

	return Video{
		ItemKind:        "video",
		VideoID:         videoID,
		Title:           title,
		Channel:         channel,
		ChannelID:       channelID,
		ThumbnailURL:    innerTubeBestThumbnailURL(renderer["thumbnail"]),
		DurationSeconds: innerTubeDurationSeconds(durationLabel),
		DurationLabel:   durationLabel,
		ViewCount:       innerTubeCount(viewLabel),
		PublishedLabel:  innerTubeText(renderer["publishedTimeText"]),
		WebURL:          webURL,
		Live:            live,
		Short:           short,
	}, true
}

func parseInnerTubeShortsLockup(lockup map[string]any) (Video, bool) {
	command := innerTubeNestedMap(lockup["onTap"], "innertubeCommand")
	videoID := innerTubeNestedString(command, "reelWatchEndpoint", "videoId")
	if videoID == "" {
		parts := strings.Split(strings.TrimSpace(innerTubeString(lockup["entityId"])), "-")
		if len(parts) > 0 {
			videoID = parts[len(parts)-1]
		}
	}
	if !youtubeVideoIDPattern.MatchString(videoID) {
		return Video{}, false
	}
	overlay, _ := innerTubeMap(lockup["overlayMetadata"])
	title := innerTubeText(overlay["primaryText"])
	if title == "" {
		return Video{}, false
	}
	viewLabel := innerTubeText(overlay["secondaryText"])
	webURL := innerTubeNavigationURL(command)
	if webURL == "" {
		webURL = "https://www.youtube.com/shorts/" + url.PathEscape(videoID)
	}
	return Video{
		ItemKind:     "video",
		VideoID:      videoID,
		Title:        title,
		ThumbnailURL: innerTubeBestThumbnailURL(lockup["thumbnailViewModel"]),
		ViewCount:    innerTubeCount(viewLabel),
		WebURL:       webURL,
		Short:        true,
	}, true
}

func parseInnerTubeVideoLockup(lockup map[string]any) (Video, bool) {
	command := innerTubeLockupCommand(lockup)
	videoID := innerTubeNestedString(command, "watchEndpoint", "videoId")
	if videoID == "" {
		videoID = strings.TrimSpace(innerTubeString(lockup["contentId"]))
	}
	if !youtubeVideoIDPattern.MatchString(videoID) {
		return Video{}, false
	}
	metadata := innerTubeLockupMetadata(lockup)
	title := innerTubeText(metadata["title"])
	if title == "" {
		return Video{}, false
	}
	rows := innerTubeMetadataRows(metadata)
	channel := ""
	channelID := innerTubeFirstBrowseID(metadata, "UC")
	stats := []string{}
	statsStart := 0
	if len(rows) > 0 && len(rows[0]) > 0 {
		// Search and browse lockups usually put the channel in the first row,
		// followed by a row containing views and publication time. A channel's
		// own Videos tab omits that redundant owner row, so treating rows[0] as
		// the channel would lose both statistics (and display "169 views" as the
		// uploader). Keep the first row as statistics when it has a recognizable
		// view label; completeUploaderVideos supplies the known channel later.
		firstRowViewLabel, _ := innerTubeStatsLabels(rows[0])
		if channelID != "" || firstRowViewLabel == "" {
			channel = rows[0][0]
			statsStart = 1
		}
	}
	if len(rows) > statsStart {
		for _, row := range rows[statsStart:] {
			stats = append(stats, row...)
		}
	}
	viewLabel, publishedLabel := innerTubeStatsLabels(stats)
	badge := innerTubePreferredBadgeText(lockup)
	short := innerTubeNavigationIsShort(command) || innerTubeHasPortraitThumbnail(lockup)
	live := strings.EqualFold(strings.TrimSpace(badge), "live") || innerTubeHasLiveMarker(lockup["badges"])
	durationLabel := badge
	if live {
		durationLabel = "LIVE"
	} else if !strings.Contains(durationLabel, ":") {
		durationLabel = ""
	}
	webURL := innerTubeNavigationURL(command)
	if webURL == "" {
		if short {
			webURL = "https://www.youtube.com/shorts/" + url.PathEscape(videoID)
		} else {
			webURL = "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID)
		}
	}
	return Video{
		ItemKind:        "video",
		VideoID:         videoID,
		Title:           title,
		Channel:         channel,
		ChannelID:       channelID,
		ThumbnailURL:    innerTubeBestThumbnailURL(lockup["contentImage"]),
		DurationSeconds: innerTubeDurationSeconds(durationLabel),
		DurationLabel:   durationLabel,
		ViewCount:       innerTubeCount(viewLabel),
		PublishedLabel:  publishedLabel,
		WebURL:          webURL,
		Live:            live,
		Short:           short,
	}, true
}

func parseInnerTubePlaylistLockup(lockup map[string]any) (Video, bool) {
	command := innerTubeLockupCommand(lockup)
	playlistID := strings.TrimSpace(innerTubeString(lockup["contentId"]))
	if playlistID == "" {
		playlistID = innerTubePlaylistIDFromNavigation(command)
	}
	if !innerTubePlaylistIDPattern.MatchString(playlistID) {
		return Video{}, false
	}
	metadata := innerTubeLockupMetadata(lockup)
	title := innerTubeText(metadata["title"])
	if title == "" {
		return Video{}, false
	}
	rows := innerTubeMetadataRows(metadata)
	channel := ""
	if len(rows) > 0 && len(rows[0]) > 0 {
		channel = rows[0][0]
	}
	webURL := innerTubePlaylistNavigationURL(command, playlistID)
	return Video{
		ItemKind:       "playlist",
		PlaylistID:     playlistID,
		Title:          title,
		Channel:        channel,
		ChannelID:      innerTubeFirstBrowseID(metadata, "UC"),
		ThumbnailURL:   innerTubeBestThumbnailURL(lockup["contentImage"]),
		DurationLabel:  innerTubePreferredBadgeText(lockup),
		PublishedLabel: innerTubePlaylistPublishedLabel(rows),
		WebURL:         webURL,
	}, true
}

func parseInnerTubePlaylistRenderer(renderer map[string]any) (Video, bool) {
	playlistID := strings.TrimSpace(innerTubeString(renderer["playlistId"]))
	navigation := firstInnerTubeValue(renderer["navigationEndpoint"], renderer["onTap"])
	if playlistID == "" {
		playlistID = innerTubePlaylistIDFromNavigation(navigation)
	}
	if !innerTubePlaylistIDPattern.MatchString(playlistID) {
		return Video{}, false
	}
	title := innerTubeText(renderer["title"])
	if title == "" {
		return Video{}, false
	}
	channel, channelID := innerTubeLegacyOwner(renderer)
	countLabel := firstInnerTubeText(
		renderer["videoCountText"],
		renderer["videoCountShortText"],
		renderer["thumbnailText"],
	)
	webURL := innerTubePlaylistNavigationURL(navigation, playlistID)
	return Video{
		ItemKind:      "playlist",
		PlaylistID:    playlistID,
		Title:         title,
		Channel:       channel,
		ChannelID:     channelID,
		ThumbnailURL:  innerTubeBestThumbnailURL(renderer),
		DurationLabel: countLabel,
		WebURL:        webURL,
	}, true
}

func innerTubeLegacyOwner(renderer map[string]any) (string, string) {
	for _, key := range []string{"ownerText", "shortBylineText", "longBylineText", "bylineText"} {
		container, ok := innerTubeMap(renderer[key])
		if !ok {
			continue
		}
		runs := innerTubeSlice(container["runs"])
		if len(runs) == 0 {
			continue
		}
		run, ok := innerTubeMap(runs[0])
		if !ok {
			continue
		}
		return strings.TrimSpace(innerTubeString(run["text"])), innerTubeFirstBrowseID(run, "UC")
	}
	return "", ""
}

func innerTubeLockupCommand(lockup map[string]any) map[string]any {
	contextMap := innerTubeNestedMap(lockup["rendererContext"], "commandContext")
	onTap := innerTubeNestedMap(contextMap, "onTap")
	return innerTubeNestedMap(onTap, "innertubeCommand")
}

func innerTubeLockupMetadata(lockup map[string]any) map[string]any {
	metadata, _ := innerTubeMap(lockup["metadata"])
	result, _ := innerTubeMap(metadata["lockupMetadataViewModel"])
	return result
}

func innerTubeMetadataRows(metadata map[string]any) [][]string {
	contentMetadata := innerTubeNestedMap(metadata["metadata"], "contentMetadataViewModel")
	rows := innerTubeSlice(contentMetadata["metadataRows"])
	result := make([][]string, 0, len(rows))
	for _, rawRow := range rows {
		row, ok := innerTubeMap(rawRow)
		if !ok {
			continue
		}
		parts := innerTubeSlice(row["metadataParts"])
		texts := make([]string, 0, len(parts))
		for _, rawPart := range parts {
			part, ok := innerTubeMap(rawPart)
			if !ok {
				continue
			}
			if text := innerTubeText(part["text"]); text != "" {
				texts = append(texts, text)
			}
		}
		if len(texts) > 0 {
			result = append(result, texts)
		}
	}
	return result
}

func innerTubeStatsLabels(stats []string) (string, string) {
	viewLabel := ""
	published := ""
	for _, value := range stats {
		trimmed := strings.Trim(strings.TrimSpace(value), " ·•")
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if viewLabel == "" && (strings.Contains(lower, "view") || innerTubeCount(trimmed) > 0) {
			viewLabel = trimmed
			continue
		}
		if published == "" {
			published = trimmed
		}
	}
	return viewLabel, published
}

func innerTubePlaylistPublishedLabel(rows [][]string) string {
	if len(rows) < 2 {
		return ""
	}
	for _, row := range rows[1:] {
		for _, value := range row {
			lower := strings.ToLower(strings.TrimSpace(value))
			if lower == "" || strings.Contains(lower, "video") {
				continue
			}
			return strings.Trim(value, " ·•")
		}
	}
	return ""
}

func innerTubePreferredBadgeText(value any) string {
	badges := make([]string, 0, 2)
	innerTubeCollectBadgeTexts(value, &badges)
	for _, badge := range badges {
		trimmed := strings.TrimSpace(badge)
		if strings.EqualFold(trimmed, "live") || strings.Contains(trimmed, ":") {
			return trimmed
		}
	}
	if len(badges) > 0 {
		return strings.TrimSpace(badges[0])
	}
	return ""
}

func innerTubeCollectBadgeTexts(value any, result *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		if badge, ok := innerTubeMap(typed["thumbnailBadgeViewModel"]); ok {
			if text := innerTubeText(badge["text"]); text != "" {
				*result = append(*result, text)
			}
			return
		}
		if status, ok := innerTubeMap(typed["thumbnailOverlayTimeStatusRenderer"]); ok {
			if text := innerTubeText(status["text"]); text != "" {
				*result = append(*result, text)
			}
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			innerTubeCollectBadgeTexts(typed[key], result)
		}
	case []any:
		for _, item := range typed {
			innerTubeCollectBadgeTexts(item, result)
		}
	case []map[string]any:
		for _, item := range typed {
			innerTubeCollectBadgeTexts(item, result)
		}
	}
}

func innerTubeTimeStatusText(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		if status, ok := innerTubeMap(typed["thumbnailOverlayTimeStatusRenderer"]); ok {
			return innerTubeText(status["text"])
		}
		for _, nested := range typed {
			if text := innerTubeTimeStatusText(nested); text != "" {
				return text
			}
		}
	case []any:
		for _, nested := range typed {
			if text := innerTubeTimeStatusText(nested); text != "" {
				return text
			}
		}
	case []map[string]any:
		for _, nested := range typed {
			if text := innerTubeTimeStatusText(nested); text != "" {
				return text
			}
		}
	}
	return ""
}

func innerTubeHasLiveMarker(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"style", "label", "text"} {
			text := strings.ToUpper(innerTubeText(typed[key]))
			if strings.Contains(text, "LIVE") {
				return true
			}
		}
		for _, nested := range typed {
			if innerTubeHasLiveMarker(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if innerTubeHasLiveMarker(nested) {
				return true
			}
		}
	case []map[string]any:
		for _, nested := range typed {
			if innerTubeHasLiveMarker(nested) {
				return true
			}
		}
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "live")
	}
	return false
}

func innerTubeHasPortraitThumbnail(value any) bool {
	candidates := make([]innerTubeThumbnail, 0, 4)
	innerTubeCollectThumbnails(value, &candidates)
	for _, candidate := range candidates {
		if candidate.width > 0 && candidate.height > candidate.width {
			return true
		}
	}
	return false
}

type innerTubeThumbnail struct {
	url    string
	width  float64
	height float64
}

func innerTubeBestThumbnailURL(value any) string {
	candidates := make([]innerTubeThumbnail, 0, 8)
	innerTubeCollectThumbnails(value, &candidates)
	best := innerTubeThumbnail{}
	bestScore := float64(-1)
	for _, candidate := range candidates {
		score := candidate.width * candidate.height
		if score <= 0 {
			score = max(candidate.width, candidate.height)
		}
		if best.url == "" || score > bestScore || score == bestScore && candidate.url > best.url {
			best = candidate
			bestScore = score
		}
	}
	return innerTubeNormalizeURL(best.url)
}

func innerTubeCollectThumbnails(value any, result *[]innerTubeThumbnail) {
	switch typed := value.(type) {
	case map[string]any:
		rawURL := strings.TrimSpace(innerTubeString(typed["url"]))
		if rawURL != "" && (typed["width"] != nil || typed["height"] != nil) {
			*result = append(*result, innerTubeThumbnail{
				url:    rawURL,
				width:  innerTubeNumber(typed["width"]),
				height: innerTubeNumber(typed["height"]),
			})
			return
		}
		for _, nested := range typed {
			innerTubeCollectThumbnails(nested, result)
		}
	case []any:
		for _, nested := range typed {
			innerTubeCollectThumbnails(nested, result)
		}
	case []map[string]any:
		for _, nested := range typed {
			innerTubeCollectThumbnails(nested, result)
		}
	}
}

func innerTubeNavigationIsShort(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, ok := typed["reelWatchEndpoint"]; ok {
			return true
		}
		if rawURL := strings.TrimSpace(innerTubeString(typed["url"])); strings.HasPrefix(rawURL, "/shorts/") {
			return true
		}
		for _, nested := range typed {
			if innerTubeNavigationIsShort(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if innerTubeNavigationIsShort(nested) {
				return true
			}
		}
	}
	return false
}

func innerTubeNavigationURL(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		if metadata, ok := innerTubeMap(typed["webCommandMetadata"]); ok {
			if rawURL := strings.TrimSpace(innerTubeString(metadata["url"])); rawURL != "" {
				return innerTubeNormalizeURL(rawURL)
			}
		}
		if reel, ok := innerTubeMap(typed["reelWatchEndpoint"]); ok {
			if videoID := strings.TrimSpace(innerTubeString(reel["videoId"])); youtubeVideoIDPattern.MatchString(videoID) {
				return "https://www.youtube.com/shorts/" + url.PathEscape(videoID)
			}
		}
		if watch, ok := innerTubeMap(typed["watchEndpoint"]); ok {
			if playlistID := strings.TrimSpace(innerTubeString(watch["playlistId"])); playlistID != "" && innerTubeString(watch["videoId"]) == "" {
				return "https://www.youtube.com/playlist?list=" + url.QueryEscape(playlistID)
			}
			if videoID := strings.TrimSpace(innerTubeString(watch["videoId"])); youtubeVideoIDPattern.MatchString(videoID) {
				return "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID)
			}
		}
		if browse, ok := innerTubeMap(typed["browseEndpoint"]); ok {
			if rawURL := strings.TrimSpace(innerTubeString(browse["canonicalBaseUrl"])); rawURL != "" {
				return innerTubeNormalizeURL(rawURL)
			}
			if playlistID := innerTubePlaylistIDFromBrowseEndpoint(browse); playlistID != "" {
				return "https://www.youtube.com/playlist?list=" + url.QueryEscape(playlistID)
			}
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if result := innerTubeNavigationURL(typed[key]); result != "" {
				return result
			}
		}
	case []any:
		for _, nested := range typed {
			if result := innerTubeNavigationURL(nested); result != "" {
				return result
			}
		}
	}
	return ""
}

func innerTubePlaylistNavigationURL(value any, playlistID string) string {
	canonical := "https://www.youtube.com/playlist?list=" + url.QueryEscape(playlistID)
	candidate := innerTubeNavigationURL(value)
	if candidate == "" {
		return canonical
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return canonical
	}
	if parsed.Query().Get("list") != "" || strings.Contains(strings.ToLower(parsed.Path), "playlist") {
		return candidate
	}
	return canonical
}

func innerTubePlaylistIDFromNavigation(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		if watch, ok := innerTubeMap(typed["watchEndpoint"]); ok {
			if playlistID := strings.TrimSpace(innerTubeString(watch["playlistId"])); innerTubePlaylistIDPattern.MatchString(playlistID) {
				return playlistID
			}
		}
		if browse, ok := innerTubeMap(typed["browseEndpoint"]); ok {
			if playlistID := innerTubePlaylistIDFromBrowseEndpoint(browse); playlistID != "" {
				return playlistID
			}
		}
		for _, nested := range typed {
			if playlistID := innerTubePlaylistIDFromNavigation(nested); playlistID != "" {
				return playlistID
			}
		}
	case []any:
		for _, nested := range typed {
			if playlistID := innerTubePlaylistIDFromNavigation(nested); playlistID != "" {
				return playlistID
			}
		}
	}
	return ""
}

func innerTubePlaylistIDFromBrowseEndpoint(endpoint map[string]any) string {
	playlistID := strings.TrimSpace(innerTubeString(endpoint["browseId"]))
	if strings.HasPrefix(playlistID, "VL") {
		playlistID = strings.TrimPrefix(playlistID, "VL")
	}
	if innerTubePlaylistIDPattern.MatchString(playlistID) {
		return playlistID
	}
	return ""
}

func innerTubeContinuationFromMap(value map[string]any) string {
	if renderer, ok := innerTubeMap(value["continuationItemRenderer"]); ok {
		endpoint, _ := innerTubeMap(renderer["continuationEndpoint"])
		command, _ := innerTubeMap(endpoint["continuationCommand"])
		if token := strings.TrimSpace(innerTubeString(command["token"])); token != "" {
			return token
		}
	}
	for _, key := range []string{
		"continuationCommand",
		"nextContinuationData",
		"nextRadioContinuationData",
		"reloadContinuationData",
	} {
		command, ok := innerTubeMap(value[key])
		if !ok {
			continue
		}
		for _, tokenKey := range []string{"token", "continuation"} {
			if token := strings.TrimSpace(innerTubeString(command[tokenKey])); token != "" {
				return token
			}
		}
	}
	return ""
}

func innerTubeFirstBrowseID(value any, prefix string) string {
	switch typed := value.(type) {
	case map[string]any:
		if endpoint, ok := innerTubeMap(typed["browseEndpoint"]); ok {
			browseID := strings.TrimSpace(innerTubeString(endpoint["browseId"]))
			if strings.HasPrefix(browseID, prefix) {
				return browseID
			}
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if browseID := innerTubeFirstBrowseID(typed[key], prefix); browseID != "" {
				return browseID
			}
		}
	case []any:
		for _, nested := range typed {
			if browseID := innerTubeFirstBrowseID(nested, prefix); browseID != "" {
				return browseID
			}
		}
	}
	return ""
}

func innerTubeDurationSeconds(label string) float64 {
	parts := strings.Split(strings.TrimSpace(label), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	total := 0
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 {
			return 0
		}
		total = total*60 + value
	}
	return float64(total)
}

func innerTubeCount(label string) int64 {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return 0
	}
	match := innerTubeCountPattern.FindStringSubmatch(trimmed)
	if len(match) < 2 {
		return 0
	}
	numberText := match[1]
	suffix := strings.ToUpper(match[2])
	if suffix == "" {
		numberText = strings.ReplaceAll(numberText, ",", "")
	} else if strings.Contains(numberText, ",") && !strings.Contains(numberText, ".") && strings.Count(numberText, ",") == 1 {
		numberText = strings.ReplaceAll(numberText, ",", ".")
	} else {
		numberText = strings.ReplaceAll(numberText, ",", "")
	}
	value, err := strconv.ParseFloat(numberText, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	multiplier := float64(1)
	switch suffix {
	case "K":
		multiplier = 1_000
	case "M":
		multiplier = 1_000_000
	case "B":
		multiplier = 1_000_000_000
	case "T":
		multiplier = 1_000_000_000_000
	default:
		if strings.Contains(trimmed, "万") || strings.Contains(trimmed, "萬") {
			multiplier = 10_000
		} else if strings.Contains(trimmed, "亿") || strings.Contains(trimmed, "億") {
			multiplier = 100_000_000
		}
	}
	value *= multiplier
	if value >= math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(math.Round(value))
}

func innerTubeText(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	dict, ok := innerTubeMap(value)
	if !ok {
		return ""
	}
	for _, key := range []string{"simpleText", "content"} {
		if text := strings.TrimSpace(innerTubeString(dict[key])); text != "" {
			return text
		}
	}
	runs := innerTubeSlice(dict["runs"])
	if len(runs) > 0 {
		var builder strings.Builder
		for _, rawRun := range runs {
			run, ok := innerTubeMap(rawRun)
			if !ok {
				continue
			}
			builder.WriteString(innerTubeString(run["text"]))
		}
		if text := strings.TrimSpace(builder.String()); text != "" {
			return text
		}
	}
	return ""
}

func firstInnerTubeText(values ...any) string {
	for _, value := range values {
		if text := innerTubeText(value); text != "" {
			return text
		}
	}
	return ""
}

func firstInnerTubeValue(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func innerTubeNestedMap(value any, keys ...string) map[string]any {
	current, ok := innerTubeMap(value)
	if !ok {
		return nil
	}
	for _, key := range keys {
		current, ok = innerTubeMap(current[key])
		if !ok {
			return nil
		}
	}
	return current
}

func innerTubeNestedString(value any, mapKey string, stringKey string) string {
	container := innerTubeNestedMap(value, mapKey)
	return strings.TrimSpace(innerTubeString(container[stringKey]))
}

func innerTubeMap(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok
}

func innerTubeSlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = typed[index]
		}
		return result
	default:
		return nil
	}
}

func innerTubeString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
}

func innerTubeNumber(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint64:
		return float64(typed)
	case string:
		result, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return result
	default:
		return 0
	}
}

func innerTubeNormalizeURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if strings.HasPrefix(trimmed, "//") {
		return "https:" + trimmed
	}
	if strings.HasPrefix(trimmed, "/") {
		return "https://www.youtube.com" + trimmed
	}
	return trimmed
}
