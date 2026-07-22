package rss

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	domainrss "xiadown/internal/domain/rss"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	youtubeIDPattern        = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)
	bilibiliBVPattern       = regexp.MustCompile(`(?i)^BV[A-Za-z0-9]+$`)
	bilibiliAVPattern       = regexp.MustCompile(`(?i)^av([0-9]+)$`)
	bilibiliBangumiPattern  = regexp.MustCompile(`(?i)^(ep|ss)([0-9]+)$`)
	tiktokUsernamePattern   = regexp.MustCompile(`^@[A-Za-z0-9._]{1,64}$`)
	socialVideoTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,128}$`)
	niconicoVideoIDPattern  = regexp.MustCompile(`(?i)^(sm|nm|so)([0-9]+)$`)
)

func entriesFromFeed(subscriptionID string, view domainrss.ViewType, feed parsedFeed, now time.Time) []domainrss.Entry {
	entryCount := min(len(feed.Entries), maxRSSFeedEntries)
	items := make([]domainrss.Entry, 0, entryCount)
	for index, source := range feed.Entries[:entryCount] {
		originKey := rssOriginKeyFromParsed(subscriptionID, source)
		source.URL = safeEntryResourceURL(feed.SiteURL, source.URL)
		baseURL := firstNonEmpty(source.URL, feed.SiteURL)
		rawContent := limitString(source.Content, maxRSSEntryContentHTMLBytes)
		extractedMedia := extractHTMLMedia(rawContent, baseURL)
		source.Content = sanitizeEntryHTML(rawContent, baseURL)
		source.Title = limitString(strings.TrimSpace(source.Title), maxRSSEntryTitleBytes)
		source.Author = limitString(strings.TrimSpace(source.Author), maxRSSEntryAuthorBytes)
		source.Summary = limitString(strings.TrimSpace(source.Summary), maxRSSEntrySummaryBytes)
		externalID := boundedRSSExternalID(source.ExternalID)
		if externalID == "" {
			externalID = source.URL
		}
		if externalID == "" {
			externalID = stableDigest(source.Title, source.Author, timeValue(source.Published), source.Content, strconv.Itoa(index))
		}
		media := make([]domainrss.Media, 0, min(len(source.Media)+4, maxRSSPersistedEntryMedia))
		mediaSeen := make(map[string]struct{}, maxRSSPersistedEntryMedia)
		for sourceIndex, candidate := range source.Media {
			if sourceIndex >= maxRSSParsedEntryMediaItems || len(media) >= maxRSSPersistedEntryMedia {
				break
			}
			resolved := safeEntryResourceURL(baseURL, candidate.URL)
			if resolved == "" {
				continue
			}
			media = appendUniqueMedia(media, mediaSeen, domainrss.Media{
				URL: resolved, MIMEType: limitString(strings.ToLower(strings.TrimSpace(candidate.MIMEType)), maxRSSMediaMIMETypeBytes),
				Kind:      mediaKind(candidate.MIMEType, resolved),
				Thumbnail: safeEntryResourceURL(baseURL, candidate.Thumbnail), Width: boundedRSSMediaDimension(candidate.Width),
				Height: boundedRSSMediaDimension(candidate.Height), Duration: boundedRSSMediaDurationMillis(candidate.Duration),
			}, maxRSSPersistedEntryMedia)
		}
		for _, candidate := range extractedMedia {
			media = appendUniqueMedia(media, mediaSeen, candidate, maxRSSPersistedEntryMedia)
			if len(media) >= maxRSSPersistedEntryMedia {
				break
			}
		}
		images := make([]string, 0, min(len(media), maxRSSPersistedEntryImages))
		imageSeen := make(map[string]struct{}, maxRSSPersistedEntryImages)
		mediaURL, mediaType, thumbnail := "", "", ""
		for _, candidate := range media {
			if candidate.Thumbnail != "" && thumbnail == "" {
				thumbnail = candidate.Thumbnail
			}
			switch candidate.Kind {
			case "image":
				images = appendUniqueString(images, imageSeen, candidate.URL, maxRSSPersistedEntryImages)
				if thumbnail == "" {
					thumbnail = candidate.URL
				}
			case "video":
				if mediaURL == "" {
					mediaURL, mediaType = candidate.URL, candidate.MIMEType
				}
			}
		}

		platform, platformID, playback := resolveVideoPlatform(source.URL, media)
		kind := classifyEntry(view, source.URL, media)
		if platform != "" {
			kind = domainrss.EntryKindVideo
		}
		title := source.Title
		if title == "" {
			title = firstNonEmpty(cleanText(source.Summary, 160), cleanText(source.Content, 160), "Untitled")
		}
		summary := source.Summary
		if summary == "" {
			summary = cleanText(source.Content, 8192)
		}
		entry := domainrss.Entry{
			ID:             "rss-entry-" + stableDigest(subscriptionID, externalID)[:32],
			SubscriptionID: subscriptionID, ExternalID: externalID, OriginKey: originKey, ObservedAt: now, URL: source.URL,
			Title: limitString(title, maxRSSEntryTitleBytes), Author: source.Author,
			Summary:     limitString(summary, maxRSSEntrySummaryBytes),
			ContentHTML: limitString(strings.TrimSpace(source.Content), maxRSSEntryContentHTMLBytes), Kind: kind, ImageURLs: images,
			Media: media, MediaURL: mediaURL, MediaType: mediaType, ThumbnailURL: thumbnail,
			Platform: platform, PlatformVideoID: platformID, PlaybackURL: playback,
			PublishedAt:     source.Published,
			SourceUpdatedAt: source.Updated, Revision: 1, CreatedAt: now, ModifiedAt: now,
		}
		entry.DownloadTarget = downloadTargetForEntry(entry)
		entry.ContentHash = contentHashForEntry(entry)
		items = append(items, entry)
	}
	return items
}

func classifyEntry(view domainrss.ViewType, rawURL string, media []domainrss.Media) domainrss.EntryKind {
	for _, item := range media {
		if item.Kind == "video" {
			return domainrss.EntryKindVideo
		}
	}
	if platform, _, _ := resolveVideoPlatform(rawURL, media); platform != "" {
		return domainrss.EntryKindVideo
	}
	switch view {
	case domainrss.ViewTypeVideo:
		// A subscription's view type is a presentation preference, not proof
		// that every item it publishes is playable media. Video enclosures and
		// recognized video-page identities have already returned above; a plain
		// webpage in a video-oriented feed must remain readable as an article.
		return domainrss.EntryKindArticle
	case domainrss.ViewTypeSocial:
		return domainrss.EntryKindSocial
	case domainrss.ViewTypeImage:
		return domainrss.EntryKindImage
	case domainrss.ViewTypeArticle:
		return domainrss.EntryKindArticle
	}
	if isSocialURL(rawURL) {
		return domainrss.EntryKindSocial
	}
	for _, item := range media {
		if item.Kind == "image" {
			return domainrss.EntryKindImage
		}
	}
	return domainrss.EntryKindArticle
}

func extractHTMLMedia(markup, baseURL string) []domainrss.Media {
	markup = limitString(markup, maxRSSEntryContentHTMLBytes)
	contextNode := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := xhtml.ParseFragment(strings.NewReader(markup), contextNode)
	if err != nil {
		return nil
	}
	type frame struct {
		node  *xhtml.Node
		depth int
	}
	stack := make([]frame, 0, min(len(nodes), 64))
	for index := len(nodes) - 1; index >= 0; index-- {
		stack = append(stack, frame{node: nodes[index], depth: 1})
	}
	items := make([]domainrss.Media, 0, 8)
	seen := make(map[string]struct{}, maxRSSPersistedEntryMedia)
	visited, resourceCandidates := 0, 0
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current.node == nil {
			continue
		}
		visited++
		if visited > maxRSSEntryHTMLNodes || current.depth > maxRSSEntryHTMLDepth {
			return nil
		}
		node := current.node
		if node.Type == xhtml.ElementNode {
			name := strings.ToLower(node.Data)
			attributes := make(map[string]string, min(len(node.Attr), maxRSSEntryHTMLAttributes))
			for index, attribute := range node.Attr {
				if index >= maxRSSEntryHTMLAttributes {
					break
				}
				attributes[strings.ToLower(attribute.Key)] = attribute.Val
			}
			rawURL, mimeType, kind := "", "", ""
			resourceCandidate := true
			switch name {
			case "img":
				rawURL, kind = firstSafeRSSImageCandidate(baseURL, attributes), "image"
			case "video":
				rawURL, mimeType, kind = attributes["src"], attributes["type"], "video"
			case "source":
				rawURL, mimeType = firstSafeRSSImageCandidate(baseURL, attributes), attributes["type"]
				kind = mediaKind(mimeType, rawURL)
			case "iframe":
				rawURL, mimeType = attributes["src"], "text/html"
			default:
				resourceCandidate = false
			}
			if resourceCandidate {
				resourceCandidates++
				resolved := safeEntryResourceURL(baseURL, rawURL)
				if name == "iframe" && isKnownVideoURL(resolved) {
					kind = "video"
				}
				if resolved != "" && kind != "" {
					items = appendUniqueMedia(items, seen, domainrss.Media{
						URL: resolved, MIMEType: limitString(strings.ToLower(strings.TrimSpace(mimeType)), maxRSSMediaMIMETypeBytes), Kind: kind,
						Thumbnail: safeEntryResourceURL(baseURL, attributes["poster"]),
					}, maxRSSPersistedEntryMedia)
				}
				if resourceCandidates >= maxRSSPersistedEntryMedia {
					return items
				}
			}
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, frame{node: child, depth: current.depth + 1})
		}
	}
	return items
}

func mediaKind(mimeType, rawURL string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case mimeType == "text/html" && isKnownVideoURL(rawURL):
		return "video"
	}
	extension := strings.ToLower(path.Ext(strings.Split(rawURL, "?")[0]))
	switch extension {
	case ".mp4", ".m4v", ".webm", ".mov", ".m3u8", ".mpd":
		return "video"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif":
		return "image"
	case ".mp3", ".m4a", ".aac", ".ogg", ".opus", ".flac":
		return "audio"
	default:
		return ""
	}
}

func resolveVideoPlatform(rawURL string, media []domainrss.Media) (string, string, string) {
	candidates := []string{rawURL}
	for _, item := range media {
		if item.Kind == "video" {
			candidates = append(candidates, item.URL)
		}
	}
	for _, candidate := range candidates {
		parsed, err := url.Parse(strings.TrimSpace(candidate))
		if err != nil {
			continue
		}
		host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
		segments := strings.FieldsFunc(strings.Trim(parsed.Path, "/"), func(r rune) bool { return r == '/' })
		switch {
		case host == "youtu.be" && len(segments) > 0:
			if youtubeIDPattern.MatchString(segments[0]) {
				return "youtube", segments[0], "https://www.youtube-nocookie.com/embed/" + segments[0]
			}
		case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") ||
			host == "youtube-nocookie.com" || strings.HasSuffix(host, ".youtube-nocookie.com"):
			videoID := parsed.Query().Get("v")
			if videoID == "" && len(segments) >= 2 && (segments[0] == "shorts" || segments[0] == "embed" || segments[0] == "live") {
				videoID = segments[1]
			}
			if youtubeIDPattern.MatchString(videoID) {
				return "youtube", videoID, "https://www.youtube-nocookie.com/embed/" + videoID
			}
		case host == "bilibili.com" || strings.HasSuffix(host, ".bilibili.com"):
			if strings.HasPrefix(host, "player.") {
				if videoID := parsed.Query().Get("bvid"); bilibiliBVPattern.MatchString(videoID) {
					return "bilibili", videoID, canonicalBilibiliVideoPage(videoID)
				}
				if aid := parsed.Query().Get("aid"); aid != "" {
					if _, err := strconv.ParseInt(aid, 10, 64); err == nil {
						videoID := "av" + aid
						return "bilibili", videoID, canonicalBilibiliVideoPage(videoID)
					}
				}
			}
			if len(segments) == 3 && segments[0] == "bangumi" && segments[1] == "play" {
				if videoID := canonicalBilibiliBangumiID(segments[2]); videoID != "" {
					return "bilibili", videoID, canonicalBilibiliBangumiPage(videoID)
				}
			}
			for index, segment := range segments {
				if segment != "video" || index+1 >= len(segments) {
					continue
				}
				videoID := segments[index+1]
				if bilibiliBVPattern.MatchString(videoID) {
					return "bilibili", videoID, canonicalBilibiliVideoPage(videoID)
				}
				if matches := bilibiliAVPattern.FindStringSubmatch(videoID); len(matches) == 2 {
					return "bilibili", videoID, canonicalBilibiliVideoPage(videoID)
				}
			}
		case host == "vimeo.com" || strings.HasSuffix(host, ".vimeo.com"):
			if len(segments) > 0 {
				if _, err := strconv.ParseInt(segments[len(segments)-1], 10, 64); err == nil {
					id := segments[len(segments)-1]
					return "vimeo", id, "https://player.vimeo.com/video/" + id
				}
			}
		}
		if platform, videoID, playbackURL := resolveSocialVideoPage(parsed, host, segments); platform != "" {
			return platform, videoID, playbackURL
		}
	}
	for _, item := range media {
		if item.Kind == "video" {
			return "generic", "", item.URL
		}
	}
	return "", "", ""
}

func canonicalBilibiliVideoPage(videoID string) string {
	return "https://www.bilibili.com/video/" + url.PathEscape(videoID) + "/"
}

func canonicalBilibiliBangumiID(rawVideoID string) string {
	matches := bilibiliBangumiPattern.FindStringSubmatch(rawVideoID)
	if len(matches) != 3 {
		return ""
	}
	numericID, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil || numericID == 0 {
		return ""
	}
	return strings.ToLower(matches[1]) + strconv.FormatUint(numericID, 10)
}

func canonicalBilibiliBangumiPage(videoID string) string {
	return "https://www.bilibili.com/bangumi/play/" + url.PathEscape(videoID)
}

// resolveSocialVideoPage deliberately recognizes only URL shapes that carry a
// stable video identity. A site being social or browser-enabled is not itself
// video evidence: profile, discovery, ordinary post, and unresolved short-link
// URLs must continue through the normal article/social classification path.
func resolveSocialVideoPage(parsed *url.URL, host string, segments []string) (string, string, string) {
	if !isSafeSocialVideoSourceURL(parsed) {
		return "", "", ""
	}
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))

	switch {
	case hostMatchesVideoDomain(host, "tiktok.com") && !hostMatchesVideoDomain(host, "vm.tiktok.com"):
		if len(segments) != 3 || !tiktokUsernamePattern.MatchString(segments[0]) || segments[1] != "video" {
			return "", "", ""
		}
		videoID := canonicalPositiveVideoID(segments[2])
		if videoID == "" {
			return "", "", ""
		}
		username := strings.TrimPrefix(segments[0], "@")
		return "tiktok", videoID, "https://www.tiktok.com/@" + url.PathEscape(username) + "/video/" + videoID

	case hostMatchesVideoDomain(host, "douyin.com"):
		if len(segments) != 2 || segments[0] != "video" {
			return "", "", ""
		}
		videoID := canonicalPositiveVideoID(segments[1])
		if videoID == "" {
			return "", "", ""
		}
		return "douyin", videoID, "https://www.douyin.com/video/" + videoID

	case hostMatchesVideoDomain(host, "instagram.com"):
		if len(segments) != 2 || segments[0] != "reel" || !socialVideoTokenPattern.MatchString(segments[1]) {
			return "", "", ""
		}
		videoID := segments[1]
		return "instagram", videoID, "https://www.instagram.com/reel/" + url.PathEscape(videoID) + "/"

	case hostMatchesVideoDomain(host, "facebook.com"):
		if len(segments) == 2 && segments[0] == "reel" {
			videoID := canonicalPositiveVideoID(segments[1])
			if videoID != "" {
				return "facebook", videoID, "https://www.facebook.com/reel/" + videoID + "/"
			}
		}
		if len(segments) == 1 && segments[0] == "watch" {
			videoID := unambiguousPositiveQueryVideoID(parsed, "v")
			if videoID != "" {
				return "facebook", videoID, "https://www.facebook.com/watch/?v=" + videoID
			}
		}
		return "", "", ""

	case host == "fb.watch":
		if len(segments) != 1 || !socialVideoTokenPattern.MatchString(segments[0]) {
			return "", "", ""
		}
		videoID := segments[0]
		return "facebook", videoID, "https://fb.watch/" + url.PathEscape(videoID) + "/"

	case hostMatchesVideoDomain(host, "twitch.tv") && !hostMatchesVideoDomain(host, "clips.twitch.tv"):
		if len(segments) != 2 || segments[0] != "videos" {
			return "", "", ""
		}
		videoID := canonicalPositiveVideoID(segments[1])
		if videoID == "" {
			return "", "", ""
		}
		return "twitch", videoID, "https://www.twitch.tv/videos/" + videoID

	case host == "clips.twitch.tv":
		if len(segments) != 1 || !socialVideoTokenPattern.MatchString(segments[0]) {
			return "", "", ""
		}
		videoID := segments[0]
		return "twitch", videoID, "https://clips.twitch.tv/" + url.PathEscape(videoID)

	case hostMatchesVideoDomain(host, "nicovideo.jp"):
		if len(segments) != 2 || segments[0] != "watch" {
			return "", "", ""
		}
		videoID := canonicalNiconicoVideoID(segments[1])
		if videoID == "" {
			return "", "", ""
		}
		return "niconico", videoID, "https://www.nicovideo.jp/watch/" + url.PathEscape(videoID)

	case host == "nico.ms":
		if len(segments) != 1 {
			return "", "", ""
		}
		videoID := canonicalNiconicoVideoID(segments[0])
		if videoID == "" {
			return "", "", ""
		}
		return "niconico", videoID, "https://www.nicovideo.jp/watch/" + url.PathEscape(videoID)
	}
	return "", "", ""
}

func isSafeSocialVideoSourceURL(parsed *url.URL) bool {
	if parsed == nil || parsed.Opaque != "" || parsed.User != nil || parsed.Hostname() == "" {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return parsed.Port() == "" || parsed.Port() == "443"
	case "http":
		return parsed.Port() == "" || parsed.Port() == "80"
	default:
		return false
	}
}

func hostMatchesVideoDomain(host, domain string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func canonicalPositiveVideoID(rawVideoID string) string {
	if rawVideoID == "" || len(rawVideoID) > 20 {
		return ""
	}
	videoID, err := strconv.ParseUint(rawVideoID, 10, 64)
	if err != nil || videoID == 0 {
		return ""
	}
	return strconv.FormatUint(videoID, 10)
}

func unambiguousPositiveQueryVideoID(parsed *url.URL, key string) string {
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return ""
	}
	values := query[key]
	if len(values) != 1 {
		return ""
	}
	return canonicalPositiveVideoID(values[0])
}

func canonicalNiconicoVideoID(rawVideoID string) string {
	matches := niconicoVideoIDPattern.FindStringSubmatch(rawVideoID)
	if len(matches) != 3 {
		return ""
	}
	numericID := canonicalPositiveVideoID(matches[2])
	if numericID == "" {
		return ""
	}
	return strings.ToLower(matches[1]) + numericID
}

func isKnownVideoURL(rawURL string) bool {
	platform, _, _ := resolveVideoPlatform(rawURL, nil)
	return platform != "" && platform != "generic"
}

// Download targets are derived from durable entry fields, just like playback
// URLs. Extractor-backed platforms need their canonical page URL so yt-dlp can
// resolve the source, while a generic enclosure is already the downloadable
// media and must not be replaced by its surrounding article URL.
func downloadTargetForEntry(entry domainrss.Entry) string {
	platform := strings.ToLower(strings.TrimSpace(entry.Platform))
	if platform == "" {
		platform, _, _ = resolveVideoPlatform(entry.URL, entry.Media)
	}
	if platform != "" && platform != "generic" {
		return firstNonEmpty(entry.URL, entry.MediaURL)
	}
	return firstNonEmpty(entry.MediaURL, entry.URL)
}

func isSocialURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	for _, suffix := range []string{
		"x.com", "twitter.com", "mastodon.social", "threads.net", "instagram.com",
		"facebook.com", "weibo.com", "bsky.app", "tiktok.com",
	} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func appendUniqueMedia(items []domainrss.Media, seen map[string]struct{}, candidate domainrss.Media, limit int) []domainrss.Media {
	candidate.URL = strings.TrimSpace(candidate.URL)
	if candidate.URL == "" || len(items) >= limit {
		return items
	}
	if _, duplicate := seen[candidate.URL]; duplicate {
		return items
	}
	seen[candidate.URL] = struct{}{}
	return append(items, candidate)
}

func appendUniqueString(items []string, seen map[string]struct{}, candidate string, limit int) []string {
	if candidate == "" || len(items) >= limit {
		return items
	}
	if _, duplicate := seen[candidate]; duplicate {
		return items
	}
	seen[candidate] = struct{}{}
	return append(items, candidate)
}

func stableDigest(values ...string) string {
	digest := sha256.New()
	for _, value := range values {
		_, _ = digest.Write([]byte(value))
		_, _ = digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func mediaFingerprint(items []domainrss.Media) string {
	var builder strings.Builder
	for _, item := range items {
		_, _ = fmt.Fprintf(
			&builder,
			"%s\x00%s\x00%s\x00%s\x00%d\x00%d\x00%d\x00",
			item.URL, item.MIMEType, item.Kind, item.Thumbnail, item.Width, item.Height, item.Duration,
		)
	}
	return builder.String()
}

func contentHashForEntry(entry domainrss.Entry) string {
	return stableDigest(
		entry.Title,
		entry.Author,
		entry.Summary,
		entry.ContentHTML,
		string(entry.Kind),
		entry.URL,
		strings.Join(entry.ImageURLs, "\x00"),
		mediaFingerprint(entry.Media),
		entry.MediaURL,
		entry.MediaType,
		entry.ThumbnailURL,
		entry.Platform,
		entry.PlatformVideoID,
		entry.PlaybackURL,
		entry.DownloadTarget,
		timeValue(entry.PublishedAt),
		timeValue(entry.SourceUpdatedAt),
	)
}

func timeValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
