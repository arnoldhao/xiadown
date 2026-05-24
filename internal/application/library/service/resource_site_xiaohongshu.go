package service

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

type resourceXiaohongshuSiteRules struct{}

type resourceXiaohongshuFormatItem struct {
	URL           string
	FormatID      string
	FormatNote    string
	VCodec        string
	ACodec        string
	Width         int
	Height        int
	QualityHeight int
	SizeBytes     int64
}

func (resourceXiaohongshuSiteRules) Name() string {
	return "xiaohongshu"
}

func (resourceXiaohongshuSiteRules) Extractor() string {
	return "resource:xiaohongshu"
}

func (resourceXiaohongshuSiteRules) PageMetaScript() string {
	return resourceXiaohongshuPageMetaScript()
}

func (resourceXiaohongshuSiteRules) ExtractMediaFromResponse(response resourceAPIResponse) []resourceStructuredMedia {
	if len(response.Body) == 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal(response.Body, &root); err != nil {
		return nil
	}
	items := make([]resourceStructuredMedia, 0, 4)
	visited := 0
	resourceXiaohongshuCollectStructuredMedia(root, response, &items, &visited)
	return dedupeResourceStructuredMedia(items)
}

func (resourceXiaohongshuSiteRules) EnrichPageMeta(pageMeta map[string]string, mediaItems []resourceStructuredMedia) map[string]string {
	pageID := firstNonEmpty(strings.TrimSpace(pageMeta["noteID"]), resourceXiaohongshuIDFromURL(pageMeta["location"]))
	media, ok := resourceStructuredMediaForPageID(pageMeta, mediaItems, pageID)
	if !ok {
		return pageMeta
	}
	return enrichPageMetaWithStructuredMedia(pageMeta, media, "noteID")
}

func (resourceXiaohongshuSiteRules) SelectCandidate(candidates []resourceCandidate, pageMeta map[string]string, _ time.Time) (resourceCandidate, bool) {
	pageID := firstNonEmpty(strings.TrimSpace(pageMeta["noteID"]), resourceXiaohongshuIDFromURL(pageMeta["location"]))
	if apiVideoURL := strings.TrimSpace(pageMeta["apiVideoURL"]); apiVideoURL != "" {
		if candidate, ok := bestResourceCandidateMatchingVideoSource(candidates, []string{apiVideoURL}); ok {
			return candidate, true
		}
		if candidate, ok := resourceCandidateFromPageMeta(pageMeta, pageID); ok {
			return candidate, true
		}
	}
	if !resourcePageHasVideoDimensions(pageMeta) {
		return resourceCandidate{}, false
	}
	if candidate, ok := bestResourceCandidateMatchingVideoSource(candidates, resourceVideoSourcesFromPageMeta(pageMeta)); ok {
		return candidate, true
	}
	return resourceCandidate{}, false
}

func (rules resourceXiaohongshuSiteRules) MediaFromCandidate(_ *LibraryService, pageURL string, pageDomain string, candidate resourceCandidate, pageMeta map[string]string) resourceMedia {
	title := firstNonEmpty(
		resourceCleanXiaohongshuTitle(pageMeta["apiTitle"]),
		resourceCleanXiaohongshuTitle(pageMeta["videoTitle"]),
		resourceCleanXiaohongshuTitle(pageMeta["jsonTitle"]),
	)
	author := firstNonEmpty(
		resourceCleanAuthor(pageMeta["apiAuthor"]),
		resourceCleanAuthor(pageMeta["jsonAuthor"]),
	)
	thumbnailURL := firstNonEmpty(
		strings.TrimSpace(pageMeta["apiImage"]),
		strings.TrimSpace(pageMeta["jsonImage"]),
	)
	return buildResourceMedia(pageURL, pageDomain, candidate, pageMeta, resourceMediaMetadata{
		Title:        title,
		Author:       author,
		ThumbnailURL: thumbnailURL,
		Extractor:    rules.Extractor(),
	})
}

func (resourceXiaohongshuSiteRules) VerificationRequired(pageMeta map[string]string, rejected []resourceRejectedCandidate) bool {
	return resourceXiaohongshuLooksBlocked(pageMeta, rejected)
}

func (rules resourceXiaohongshuSiteRules) MediaOptionsFromStructured(pageURL string, pageDomain string, pageMeta map[string]string, mediaItems []resourceStructuredMedia) []resourceMedia {
	pageID := firstNonEmpty(strings.TrimSpace(pageMeta["noteID"]), resourceXiaohongshuIDFromURL(pageMeta["location"]))
	items := resourceStructuredMediaOptionsForPageID(pageMeta, mediaItems, pageID)
	if len(items) == 0 {
		return nil
	}
	items = dedupeResourceStructuredFormatOptions(items)
	result := make([]resourceMedia, 0, len(items))
	for _, item := range items {
		media := buildResourceMediaFromStructured(pageURL, pageDomain, item, rules.Extractor())
		if strings.TrimSpace(media.Title) == "" {
			media.Title = firstNonEmpty(
				resourceCleanXiaohongshuTitle(pageMeta["apiTitle"]),
				resourceCleanXiaohongshuTitle(pageMeta["videoTitle"]),
				resourceCleanXiaohongshuTitle(pageMeta["jsonTitle"]),
			)
		}
		if strings.TrimSpace(media.Author) == "" {
			media.Author = firstNonEmpty(
				resourceCleanAuthor(pageMeta["apiAuthor"]),
				resourceCleanAuthor(pageMeta["jsonAuthor"]),
			)
		}
		if strings.TrimSpace(media.ThumbnailURL) == "" {
			media.ThumbnailURL = resourceSecureImageURL(firstNonEmpty(
				strings.TrimSpace(pageMeta["apiImage"]),
				strings.TrimSpace(pageMeta["jsonImage"]),
			))
		}
		result = append(result, media)
	}
	return dedupeResourceMediaOptions(result)
}

func resourceXiaohongshuCollectStructuredMedia(value any, response resourceAPIResponse, items *[]resourceStructuredMedia, visited *int) {
	if value == nil || *visited > 1500 {
		return
	}
	*visited++
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			resourceXiaohongshuCollectStructuredMedia(item, response, items, visited)
		}
	case map[string]any:
		if mediaItems := resourceXiaohongshuStructuredMediaFromObject(typed, response); len(mediaItems) > 0 {
			*items = append(*items, mediaItems...)
		}
		for _, child := range typed {
			switch child.(type) {
			case map[string]any, []any:
				resourceXiaohongshuCollectStructuredMedia(child, response, items, visited)
			}
		}
	}
}

func resourceXiaohongshuStructuredMediaFromObject(object map[string]any, response resourceAPIResponse) []resourceStructuredMedia {
	if note, ok := resourceMapValue(object, "note"); ok {
		return resourceXiaohongshuStructuredMediaFromNote(note, response)
	}
	if _, ok := resourceMapValue(object, "video"); ok {
		return resourceXiaohongshuStructuredMediaFromNote(object, response)
	}
	return nil
}

func resourceXiaohongshuStructuredMediaFromNote(note map[string]any, response resourceAPIResponse) []resourceStructuredMedia {
	video := resourceMapValueOrNil(note, "video")
	if len(video) == 0 {
		return nil
	}
	formatItems := resourceXiaohongshuVideoFormatItems(video)
	if len(formatItems) == 0 {
		return nil
	}
	id := firstNonEmpty(
		resourceStringValue(note, "noteId", "note_id", "id"),
		resourceXiaohongshuIDFromURL(response.PageURL),
		resourceXiaohongshuIDFromURL(response.URL),
	)
	pageURL := firstNonEmpty(
		resourceStringValue(note, "shareUrl", "share_url", "url"),
		resourceStringValue(resourceMapValueOrNil(note, "shareInfo", "share_info"), "link", "url"),
		resourceXiaohongshuPageURLFromID(id),
		response.PageURL,
	)
	user := resourceMapValueOrNil(note, "user", "author", "owner")
	title := resourceCleanXiaohongshuTitle(resourceStringValue(note, "title", "desc", "description"))
	author := resourceCleanAuthor(resourceStringValue(user, "nickname", "nickName", "name", "userName", "username"))
	thumbnailURL := resourceXiaohongshuThumbnailURL(note)
	videoWidth, videoHeight := resourceXiaohongshuVideoDimensions(video)
	seenAt := firstNonZeroTime(response.SeenAt, time.Now())
	result := make([]resourceStructuredMedia, 0, len(formatItems))
	for _, format := range formatItems {
		width := firstPositiveInt(format.Width, videoWidth)
		height := firstPositiveInt(format.Height, videoHeight)
		result = append(result, resourceStructuredMedia{
			ID:            id,
			VideoURL:      format.URL,
			PageURL:       pageURL,
			Title:         title,
			Author:        author,
			ThumbnailURL:  thumbnailURL,
			FormatID:      format.FormatID,
			FormatNote:    format.FormatNote,
			VCodec:        firstNonEmpty(resourceNormalizeVideoCodec(format.VCodec), "h264"),
			ACodec:        firstNonEmpty(format.ACodec, "aac"),
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

func resourceXiaohongshuVideoFormatItems(video map[string]any) []resourceXiaohongshuFormatItem {
	media := resourceMapValueOrNil(video, "media")
	stream := resourceAnyValue(media, "stream")
	result := make([]resourceXiaohongshuFormatItem, 0, 4)
	visited := 0
	resourceXiaohongshuCollectFormatItems(stream, &result, &visited)
	if originKey := resourceStringValue(resourceMapValueOrNil(video, "consumer"), "originVideoKey", "origin_video_key"); originKey != "" {
		if rawURL := resourceXiaohongshuOriginalVideoURL(originKey); rawURL != "" {
			result = append(result, resourceXiaohongshuFormatItem{
				URL:        rawURL,
				FormatID:   "direct",
				FormatNote: "Original video",
				VCodec:     "h264",
				Width:      resourceIntValue(media, "width", "videoWidth"),
				Height:     resourceIntValue(media, "height", "videoHeight"),
			})
		}
	}
	return dedupeResourceXiaohongshuFormatItems(result)
}

func resourceXiaohongshuCollectFormatItems(value any, result *[]resourceXiaohongshuFormatItem, visited *int) {
	if value == nil || *visited > 500 {
		return
	}
	*visited++
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			resourceXiaohongshuCollectFormatItems(item, result, visited)
		}
	case map[string]any:
		if resourceAnyValue(typed, "masterUrl", "master_url", "backupUrls", "backup_urls") != nil {
			formatID := resourceStringValue(typed, "qualityType", "quality_type", "format", "formatId", "format_id")
			width := resourceIntValue(typed, "width")
			height := resourceIntValue(typed, "height")
			qualityHeight := resourceQualityHeightFromText(formatID, resourceStringValue(typed, "quality", "qualityLabel"))
			if qualityHeight <= 0 && width >= height && height > 0 {
				qualityHeight = height
			}
			urls := append(
				resourceAddressURLs(resourceAnyValue(typed, "masterUrl", "master_url")),
				resourceAddressURLs(resourceAnyValue(typed, "backupUrls", "backup_urls"))...,
			)
			for _, rawURL := range dedupeResourceStrings(urls) {
				if !resourceMediaURLLooksVideo(rawURL) {
					continue
				}
				*result = append(*result, resourceXiaohongshuFormatItem{
					URL:           rawURL,
					FormatID:      formatID,
					FormatNote:    firstNonEmpty(formatID, resourceStringValue(typed, "quality", "qualityLabel")),
					VCodec:        resourceStringValue(typed, "videoCodec", "video_codec", "vcodec"),
					ACodec:        resourceStringValue(typed, "audioCodec", "audio_codec", "acodec"),
					Width:         width,
					Height:        height,
					QualityHeight: qualityHeight,
					SizeBytes: firstPositiveInt64(
						resourceAddressSize(typed),
						resourceInt64Value(typed, "size", "fileSize", "filesize"),
					),
				})
			}
		}
		for _, child := range typed {
			switch child.(type) {
			case map[string]any, []any:
				resourceXiaohongshuCollectFormatItems(child, result, visited)
			}
		}
	}
}

func dedupeResourceXiaohongshuFormatItems(items []resourceXiaohongshuFormatItem) []resourceXiaohongshuFormatItem {
	if len(items) == 0 {
		return nil
	}
	result := make([]resourceXiaohongshuFormatItem, 0, len(items))
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

func resourceXiaohongshuThumbnailURL(note map[string]any) string {
	if list, ok := resourceAnyValue(note, "imageList", "image_list", "images").([]any); ok {
		for _, item := range list {
			if rawURL := resourceFirstAddressURL(item, "urlDefault", "urlPre", "url_default", "url_pre", "url"); rawURL != "" {
				return rawURL
			}
		}
	}
	return firstNonEmpty(
		resourceFirstAddressURL(resourceAnyValue(note, "cover", "coverUrl", "coverURL")),
		resourceFirstAddressURL(resourceAnyValue(note, "image", "thumbnail")),
	)
}

func resourceXiaohongshuVideoDimensions(video map[string]any) (int, int) {
	media := resourceMapValueOrNil(video, "media")
	return firstPositiveInt(
			resourceIntValue(media, "width", "videoWidth"),
			resourceIntValue(video, "width", "videoWidth"),
		),
		firstPositiveInt(
			resourceIntValue(media, "height", "videoHeight"),
			resourceIntValue(video, "height", "videoHeight"),
		)
}

func resourceXiaohongshuOriginalVideoURL(originKey string) string {
	originKey = strings.TrimSpace(originKey)
	if originKey == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(originKey), "http") {
		return originKey
	}
	return "https://sns-video-bd.xhscdn.com/" + strings.TrimLeft(originKey, "/")
}

func resourceXiaohongshuIDFromURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	for _, key := range []string{"note_id", "noteId", "id"} {
		if value := strings.TrimSpace(parsed.Query().Get(key)); value != "" {
			return value
		}
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	for index, part := range parts {
		if (part == "explore" || part == "item") && index+1 < len(parts) {
			return strings.TrimSpace(parts[index+1])
		}
		if part == "discovery" && index+2 < len(parts) && parts[index+1] == "item" {
			return strings.TrimSpace(parts[index+2])
		}
	}
	return ""
}

func resourceXiaohongshuPageURLFromID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://www.xiaohongshu.com/explore/" + id
}

func resourceXiaohongshuPageURLCanAnchorMedia(rawURL string) bool {
	return resourceXiaohongshuIDFromURL(rawURL) != ""
}

func resourceCleanXiaohongshuTitle(value string) string {
	title := resourceCleanMetadataText(value)
	for _, suffix := range []string{" - 小红书", " | 小红书", "_小红书", "- Xiaohongshu", "| Xiaohongshu", "- RedNote", "| RedNote"} {
		if strings.HasSuffix(title, suffix) {
			title = strings.TrimSpace(strings.TrimSuffix(title, suffix))
		}
	}
	lower := strings.ToLower(title)
	if title == "小红书" ||
		strings.Contains(title, "验证码") ||
		strings.Contains(title, "登录") ||
		strings.Contains(title, "扫码") ||
		strings.Contains(title, "captcha") ||
		lower == "xiaohongshu" ||
		lower == "rednote" {
		return ""
	}
	return title
}

func resourceNormalizeVideoCodec(value string) string {
	codec := strings.ToLower(strings.TrimSpace(value))
	switch {
	case codec == "":
		return ""
	case strings.Contains(codec, "265"), strings.Contains(codec, "hevc"), strings.Contains(codec, "hvc"):
		return "h265"
	case strings.Contains(codec, "264"), strings.Contains(codec, "avc"):
		return "h264"
	default:
		return strings.TrimSpace(value)
	}
}

func resourceXiaohongshuLooksBlocked(pageMeta map[string]string, rejected []resourceRejectedCandidate) bool {
	title := strings.ToLower(strings.TrimSpace(pageMeta["title"]))
	location := strings.ToLower(strings.TrimSpace(pageMeta["location"]))
	loginHint := strings.TrimSpace(pageMeta["loginHint"])
	if strings.Contains(title, "验证码") ||
		strings.Contains(title, "captcha") ||
		strings.Contains(location, "captcha") ||
		strings.Contains(location, "verify") ||
		loginHint != "" {
		return true
	}
	for _, candidate := range rejected {
		lowerURL := strings.ToLower(strings.TrimSpace(candidate.url))
		if strings.Contains(lowerURL, "captcha") ||
			strings.Contains(lowerURL, "verify") ||
			strings.Contains(lowerURL, "redcaptcha") ||
			strings.Contains(lowerURL, "risk") {
			return true
		}
	}
	return false
}

func resourceXiaohongshuPageMetaScript() string {
	return `(async () => {
		const clean = (value) => String(value || "").replace(/\s+/g, " ").trim();
		const meta = (selector) => {
			const node = document.querySelector(selector);
			return node ? clean(node.getAttribute("content") || "") : "";
		};
		const first = (values) => {
			for (const value of values) {
				const cleaned = clean(value);
				if (cleaned) return cleaned;
			}
			return "";
		};
		const numberFrom = (values) => {
			for (const value of values) {
				const number = Number(value || 0);
				if (Number.isFinite(number) && number > 0) return number;
			}
			return 0;
		};
		const currentNoteID = (() => {
			try {
				const url = new URL(window.location.href || "");
				for (const key of ["note_id", "noteId", "id"]) {
					const value = clean(url.searchParams.get(key) || "");
					if (value) return value;
				}
				const match = url.pathname.match(/\/(?:explore|discovery\/item)\/([\da-f]+)/i);
				return match ? clean(match[1]) : "";
			} catch (_) {
				const match = String(window.location.href || "").match(/\/(?:explore|discovery\/item)\/([\da-f]+)/i);
				return match ? clean(match[1]) : "";
			}
		})();
		const urlFromAddress = (value) => {
			if (!value) return "";
			if (typeof value === "string") return /^https?:\/\//i.test(clean(value)) ? clean(value) : "";
			if (Array.isArray(value)) {
				for (const item of value) {
					const url = urlFromAddress(item);
					if (url) return url;
				}
				return "";
			}
			if (typeof value !== "object") return "";
			for (const key of ["masterUrl", "master_url", "url", "urlDefault", "urlPre", "src"]) {
				const url = urlFromAddress(value[key]);
				if (url) return url;
			}
			for (const key of ["backupUrls", "backup_urls", "urls"]) {
				const url = urlFromAddress(value[key]);
				if (url) return url;
			}
			return "";
		};
		const streamFormats = (stream) => {
			const queue = [stream];
			const result = [];
			let count = 0;
			while (queue.length && count < 500) {
				const value = queue.shift();
				count += 1;
				if (!value || typeof value !== "object") continue;
				if (Array.isArray(value)) {
					for (const item of value) queue.push(item);
					continue;
				}
				const videoURL = first([urlFromAddress(value.masterUrl), urlFromAddress(value.backupUrls)]);
				if (videoURL) {
					result.push({
						videoURL,
						width: numberFrom([value.width]),
						height: numberFrom([value.height]),
						sizeBytes: numberFrom([value.size, value.fileSize, value.filesize]),
						quality: clean(value.qualityType || value.quality || value.format || "")
					});
				}
				for (const child of Object.values(value)) {
					if (child && typeof child === "object") queue.push(child);
				}
			}
			return result.sort((left, right) => {
				const leftPixels = (left.width || 0) * (left.height || 0);
				const rightPixels = (right.width || 0) * (right.height || 0);
				if (rightPixels !== leftPixels) return rightPixels - leftPixels;
				return (right.sizeBytes || 0) - (left.sizeBytes || 0);
			});
		};
		const thumbnailFromNote = (note) => {
			for (const image of (Array.isArray(note?.imageList) ? note.imageList : [])) {
				const url = first([urlFromAddress(image.urlDefault), urlFromAddress(image.urlPre), urlFromAddress(image.url)]);
				if (url) return url;
			}
			return first([urlFromAddress(note?.cover), urlFromAddress(note?.image), urlFromAddress(note?.thumbnail)]);
		};
		const noteFromRoot = (root) => {
			const direct = currentNoteID && root?.note?.noteDetailMap?.[currentNoteID]?.note;
			if (direct) return direct;
			const map = root?.note?.noteDetailMap;
			if (map && typeof map === "object") {
				for (const value of Object.values(map)) {
					if (value?.note?.video) return value.note;
				}
			}
			const queue = [root];
			let count = 0;
			while (queue.length && count < 1500) {
				const value = queue.shift();
				count += 1;
				if (!value || typeof value !== "object") continue;
				if (Array.isArray(value)) {
					for (const item of value.slice(0, 80)) queue.push(item);
					continue;
				}
				const note = value.note && typeof value.note === "object" ? value.note : value;
				if (note.video && (!currentNoteID || !note.noteId || note.noteId === currentNoteID)) return note;
				for (const child of Object.values(value)) {
					if (child && typeof child === "object") queue.push(child);
				}
			}
			return null;
		};
		const mediaFromNote = (note) => {
			if (!note?.video) return null;
			const formats = streamFormats(note.video?.media?.stream);
			const originKey = clean(note.video?.consumer?.originVideoKey || "");
			if (originKey) {
				formats.push({
					videoURL: /^https?:\/\//i.test(originKey) ? originKey : "https://sns-video-bd.xhscdn.com/" + originKey.replace(/^\/+/, ""),
					width: numberFrom([note.video?.media?.width, note.video?.width]),
					height: numberFrom([note.video?.media?.height, note.video?.height]),
					quality: "direct"
				});
			}
			const bestFormat = formats[0];
			if (!bestFormat?.videoURL) return null;
			const user = note.user || note.author || {};
			return {
				id: clean(note.noteId || note.note_id || note.id || ""),
				videoURL: bestFormat.videoURL,
				title: clean(note.title || note.desc || note.description || ""),
				author: clean(user.nickname || user.nickName || user.name || user.userName || ""),
				coverURL: thumbnailFromNote(note),
				width: bestFormat.width || numberFrom([note.video?.media?.width, note.video?.width]),
				height: bestFormat.height || numberFrom([note.video?.media?.height, note.video?.height]),
				sizeBytes: bestFormat.sizeBytes || 0
			};
		};
		const matchedNote = noteFromRoot(window.__INITIAL_STATE__ || window.__INITIAL_SSR_STATE__ || window.__NUXT__ || {});
		const matchedMedia = mediaFromNote(matchedNote);
		const videos = Array.from(document.querySelectorAll("video")).map((video) => {
			const rect = video.getBoundingClientRect();
			const visibleWidth = Math.max(0, Math.min(rect.right, window.innerWidth) - Math.max(rect.left, 0));
			const visibleHeight = Math.max(0, Math.min(rect.bottom, window.innerHeight) - Math.max(rect.top, 0));
			return {
				width: Number(video.videoWidth || 0),
				height: Number(video.videoHeight || 0),
				currentSrc: (video.currentSrc || "").slice(0, 2000),
				src: (video.src || "").slice(0, 2000),
				poster: (video.poster || "").slice(0, 2000),
				paused: Boolean(video.paused),
				visibleArea: Math.round(visibleWidth * visibleHeight)
			};
		}).sort((left, right) => right.visibleArea - left.visibleArea);
		const primaryVideo = videos[0] || {};
		const bodyText = ((document.body && document.body.innerText) || "").slice(0, 2000);
		const loginHint = /登录|登陆|验证码|扫码|sign in|login|captcha/i.test(bodyText) ? "login_or_verify_text_detected" : "";
		const videoItems = matchedMedia ? [{
			currentSrc: matchedMedia.videoURL,
			src: matchedMedia.videoURL,
			poster: matchedMedia.coverURL || "",
			width: matchedMedia.width || 0,
			height: matchedMedia.height || 0,
			visibleArea: 1
		}] : videos.slice(0, 1);
		return {
			location: (window.location.href || "").slice(0, 500),
			title: clean(document.title || ""),
			ogTitle: meta('meta[property="og:title"]'),
			description: meta('meta[name="description"]') || meta('meta[property="og:description"]'),
			image: meta('meta[property="og:image"]'),
			thumbnail: meta('meta[name="thumbnail"]') || meta('meta[name="twitter:image"]'),
			noteID: matchedMedia?.id || currentNoteID,
			apiVideoURL: matchedMedia?.videoURL || "",
			apiTitle: matchedMedia?.title || "",
			apiAuthor: matchedMedia?.author || "",
			apiImage: matchedMedia?.coverURL || "",
			apiSizeBytes: matchedMedia?.sizeBytes ? String(matchedMedia.sizeBytes) : "",
			author: matchedMedia?.author || "",
			videoTitle: matchedMedia?.title || "",
			jsonTitle: matchedMedia?.title || "",
			jsonAuthor: matchedMedia?.author || "",
			jsonImage: matchedMedia?.coverURL || "",
			videoCount: String(document.querySelectorAll("video").length),
			videoCurrentSrc: matchedMedia?.videoURL || primaryVideo.currentSrc || "",
			videoSrc: primaryVideo.src || "",
			videoItems: JSON.stringify(videoItems),
			videoWidth: matchedMedia?.width ? String(matchedMedia.width) : (primaryVideo.width ? String(primaryVideo.width) : ""),
			videoHeight: matchedMedia?.height ? String(matchedMedia.height) : (primaryVideo.height ? String(primaryVideo.height) : ""),
			loginHint
		};
	})()`
}
