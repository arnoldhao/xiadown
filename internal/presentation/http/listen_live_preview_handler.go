package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const (
	listenLivePreviewTimeout     = 16 * time.Second
	listenLivePreviewMaxBodySize = 4 * 1024 * 1024
)

var (
	listenLivePreviewURLVideoIDPattern      = regexp.MustCompile(`(?:v=|vi=|youtu\.be/|/(?:embed|live|shorts|v)/)([A-Za-z0-9_-]{11})`)
	listenLivePreviewAuthorImageSizePattern = regexp.MustCompile(`=s\d+(?:-[^/?#&]*)?$`)
)

type ListenLivePreviewHandler struct {
	clientProvider listenLiveStatusHTTPClientProvider
}

type ListenLivePreviewResponse struct {
	VideoID       string `json:"videoId"`
	Title         string `json:"title"`
	Channel       string `json:"channel"`
	Description   string `json:"description"`
	DurationLabel string `json:"durationLabel"`
	ThumbnailURL  string `json:"thumbnailUrl"`
}

func NewListenLivePreviewHandler(clientProvider listenLiveStatusHTTPClientProvider) *ListenLivePreviewHandler {
	return &ListenLivePreviewHandler{clientProvider: clientProvider}
}

func (handler *ListenLivePreviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(w, r)

	videoID := extractListenLivePreviewVideoID(r.URL.Query().Get("url"))
	if videoID == "" {
		writeListenLiveUserCatalogError(w, http.StatusBadRequest, "Invalid YouTube live link.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), listenLivePreviewTimeout)
	defer cancel()
	preview, err := handler.fetchPreview(ctx, videoID)
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "not a live stream") {
			status = http.StatusBadRequest
		}
		writeListenLiveUserCatalogError(w, status, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(preview)
}

func (handler *ListenLivePreviewHandler) fetchPreview(ctx context.Context, videoID string) (ListenLivePreviewResponse, error) {
	html, err := handler.fetchYouTubeHTML(ctx, videoID)
	if err != nil {
		return ListenLivePreviewResponse{}, err
	}
	status := resolveListenLiveStatusFromHTML(videoID, html).Status
	if status != "live" && status != "upcoming" {
		return ListenLivePreviewResponse{}, fmt.Errorf("This YouTube link is not a live stream.")
	}

	playerResponse := listenLivePreviewExtractJSONObject(html, "ytInitialPlayerResponse")
	videoDetails := listenLivePreviewMap(playerResponse["videoDetails"])
	microformat := listenLivePreviewMap(listenLivePreviewMap(playerResponse["microformat"])["playerMicroformatRenderer"])
	title := firstListenLivePreviewText(
		listenLivePreviewString(videoDetails["title"]),
		listenLivePreviewSimpleText(microformat["title"]),
		listenLivePreviewString(microformat["title"]),
		videoID,
	)
	channel := firstListenLivePreviewText(
		listenLivePreviewString(videoDetails["author"]),
		listenLivePreviewString(microformat["ownerChannelName"]),
		"YouTube Live",
	)
	description := firstListenLivePreviewText(
		listenLivePreviewString(videoDetails["shortDescription"]),
		listenLivePreviewSimpleText(microformat["description"]),
	)
	thumbnailURL := firstListenLivePreviewText(
		listenLivePreviewAuthorThumbnailURL(html),
		listenLivePreviewLargestThumbnailURL(microformat["thumbnail"]),
		listenLivePreviewLargestThumbnailURL(videoDetails["thumbnail"]),
		"https://i.ytimg.com/vi/"+url.PathEscape(videoID)+"/hqdefault.jpg",
	)
	durationLabel := "LIVE"
	if status == "upcoming" {
		durationLabel = "UPCOMING"
	}

	return ListenLivePreviewResponse{
		VideoID:       videoID,
		Title:         title,
		Channel:       channel,
		Description:   description,
		DurationLabel: durationLabel,
		ThumbnailURL:  thumbnailURL,
	}, nil
}

func (handler *ListenLivePreviewHandler) fetchYouTubeHTML(ctx context.Context, videoID string) (string, error) {
	watchURL := "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID) + "&bpctr=9999999999&has_verified=1&hl=en"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", youtubemusic.BrowserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.youtube.com/")

	client := http.DefaultClient
	if handler.clientProvider != nil {
		if provided := handler.clientProvider.HTTPClient(); provided != nil {
			client = provided
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, listenLivePreviewMaxBodySize+1))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("youtube request failed: status %d", resp.StatusCode)
	}
	if readErr != nil {
		return "", readErr
	}
	if len(body) > listenLivePreviewMaxBodySize {
		return "", fmt.Errorf("youtube response is too large")
	}
	return string(body), nil
}

func extractListenLivePreviewVideoID(rawURL string) string {
	value := strings.TrimSpace(rawURL)
	if youtubeVideoIDPattern.MatchString(value) {
		return value
	}
	if parsed, err := url.Parse(value); err == nil {
		host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
		pathParts := strings.FieldsFunc(parsed.EscapedPath(), func(r rune) bool { return r == '/' })
		if host == "youtu.be" && len(pathParts) > 0 {
			if id := normalizeListenLivePreviewVideoID(pathParts[0]); id != "" {
				return id
			}
		}
		if strings.HasSuffix(host, "youtube.com") || strings.HasSuffix(host, "youtube-nocookie.com") {
			if id := normalizeListenLivePreviewVideoID(parsed.Query().Get("v")); id != "" {
				return id
			}
			if id := normalizeListenLivePreviewVideoID(parsed.Query().Get("vi")); id != "" {
				return id
			}
			for index, part := range pathParts {
				if part == "embed" || part == "live" || part == "shorts" || part == "v" {
					if index+1 < len(pathParts) {
						if id := normalizeListenLivePreviewVideoID(pathParts[index+1]); id != "" {
							return id
						}
					}
				}
			}
		}
	}
	match := listenLivePreviewURLVideoIDPattern.FindStringSubmatch(value)
	if len(match) > 1 {
		return normalizeListenLivePreviewVideoID(match[1])
	}
	return ""
}

func normalizeListenLivePreviewVideoID(value string) string {
	candidate := strings.TrimSpace(value)
	if len(candidate) > 11 {
		candidate = candidate[:11]
	}
	if youtubeVideoIDPattern.MatchString(candidate) {
		return candidate
	}
	return ""
}

func listenLivePreviewExtractJSONObject(html string, marker string) map[string]any {
	markerIndex := strings.Index(html, marker)
	if markerIndex < 0 {
		return map[string]any{}
	}
	start := strings.Index(html[markerIndex:], "{")
	if start < 0 {
		return map[string]any{}
	}
	start += markerIndex
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(html); index++ {
		character := html[index]
		if inString {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				inString = false
			}
			continue
		}
		if character == '"' {
			inString = true
			continue
		}
		if character == '{' {
			depth++
			continue
		}
		if character == '}' {
			depth--
			if depth == 0 {
				var result map[string]any
				if err := json.Unmarshal([]byte(html[start:index+1]), &result); err == nil {
					return result
				}
				return map[string]any{}
			}
		}
	}
	return map[string]any{}
}

func listenLivePreviewMap(value any) map[string]any {
	if record, ok := value.(map[string]any); ok {
		return record
	}
	return map[string]any{}
}

func listenLivePreviewSimpleText(value any) string {
	record := listenLivePreviewMap(value)
	if text := listenLivePreviewString(record["simpleText"]); text != "" {
		return text
	}
	runs, _ := record["runs"].([]any)
	parts := make([]string, 0, len(runs))
	for _, run := range runs {
		if text := listenLivePreviewString(listenLivePreviewMap(run)["text"]); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func listenLivePreviewLargestThumbnailURL(value any) string {
	record := listenLivePreviewMap(value)
	thumbnails, _ := record["thumbnails"].([]any)
	bestURL := ""
	bestWidth := float64(-1)
	for _, thumbnail := range thumbnails {
		item := listenLivePreviewMap(thumbnail)
		itemURL := listenLivePreviewCleanImageURL(listenLivePreviewString(item["url"]))
		width, _ := item["width"].(float64)
		if itemURL != "" && width >= bestWidth {
			bestWidth = width
			bestURL = itemURL
		}
	}
	return bestURL
}

func listenLivePreviewAuthorThumbnailURL(html string) string {
	ownerRenderer := listenLivePreviewExtractJSONObject(html, `"videoOwnerRenderer"`)
	if len(ownerRenderer) == 0 {
		ownerRenderer = listenLivePreviewExtractJSONObject(html, "videoOwnerRenderer")
	}
	if thumbnailURL := listenLivePreviewLargestThumbnailURL(ownerRenderer["thumbnail"]); thumbnailURL != "" {
		return listenLivePreviewHighQualityAuthorImageURL(thumbnailURL)
	}
	return listenLivePreviewHighQualityAuthorImageURL(
		listenLivePreviewFindVideoOwnerThumbnailURL(
			listenLivePreviewExtractJSONObject(html, "ytInitialData"),
			0,
		),
	)
}

func listenLivePreviewFindVideoOwnerThumbnailURL(value any, depth int) string {
	if depth > 48 {
		return ""
	}
	switch typed := value.(type) {
	case map[string]any:
		if secondaryInfo := listenLivePreviewMap(typed["videoSecondaryInfoRenderer"]); len(secondaryInfo) > 0 {
			owner := listenLivePreviewMap(secondaryInfo["owner"])
			ownerRenderer := listenLivePreviewMap(owner["videoOwnerRenderer"])
			if thumbnailURL := listenLivePreviewLargestThumbnailURL(ownerRenderer["thumbnail"]); thumbnailURL != "" {
				return thumbnailURL
			}
		}
		if ownerRenderer := listenLivePreviewMap(typed["videoOwnerRenderer"]); len(ownerRenderer) > 0 {
			if thumbnailURL := listenLivePreviewLargestThumbnailURL(ownerRenderer["thumbnail"]); thumbnailURL != "" {
				return thumbnailURL
			}
		}
		for _, child := range typed {
			if thumbnailURL := listenLivePreviewFindVideoOwnerThumbnailURL(child, depth+1); thumbnailURL != "" {
				return thumbnailURL
			}
		}
	case []any:
		for _, child := range typed {
			if thumbnailURL := listenLivePreviewFindVideoOwnerThumbnailURL(child, depth+1); thumbnailURL != "" {
				return thumbnailURL
			}
		}
	}
	return ""
}

func listenLivePreviewHighQualityAuthorImageURL(value string) string {
	normalized := listenLivePreviewCleanImageURL(value)
	if normalized == "" {
		return ""
	}
	if delimiterIndex := strings.IndexAny(normalized, "?#&"); delimiterIndex >= 0 {
		return listenLivePreviewAuthorImageSizePattern.ReplaceAllString(
			normalized[:delimiterIndex],
			"",
		) + normalized[delimiterIndex:]
	}
	return listenLivePreviewAuthorImageSizePattern.ReplaceAllString(normalized, "")
}

func listenLivePreviewCleanImageURL(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, `\u0026`, "&")
	normalized = strings.ReplaceAll(normalized, "&amp;", "&")
	if normalized == "" {
		return ""
	}
	if strings.HasPrefix(normalized, "//") {
		return "https:" + normalized
	}
	if strings.HasPrefix(strings.ToLower(normalized), "http://") || strings.HasPrefix(strings.ToLower(normalized), "https://") {
		return normalized
	}
	return ""
}

func firstListenLivePreviewText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func listenLivePreviewString(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}
