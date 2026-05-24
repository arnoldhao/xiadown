package service

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

const resourceDouyinAPIAugmentTimeout = 4 * time.Second

func (rules resourceDouyinSiteRules) AugmentStructuredData(ctx context.Context, _ *LibraryService, pageURL string, pageMeta map[string]string, mediaItems []resourceStructuredMedia, hints []resourceNoMediaHint) ([]resourceStructuredMedia, []resourceNoMediaHint) {
	awemeID := resourceDouyinCurrentAwemeID(pageMeta)
	if awemeID == "" {
		return mediaItems, hints
	}
	if resourceDouyinHasStructuredMediaID(mediaItems, awemeID) || resourceDouyinHasNoMediaHintID(hints, awemeID) {
		return mediaItems, hints
	}
	response, ok := resourceDouyinFetchAwemeDetail(ctx, firstNonEmpty(strings.TrimSpace(pageMeta["location"]), pageURL), awemeID)
	if !ok {
		return mediaItems, hints
	}
	extractedMedia := rules.ExtractMediaFromResponse(response)
	extractedHints := rules.ExtractNoMediaHintsFromResponse(response)
	zap.L().Debug("resource sniff douyin detail api parsed",
		zap.String("awemeID", awemeID),
		zap.String("pageURL", resourceSniffLogURL(firstNonEmpty(strings.TrimSpace(pageMeta["location"]), pageURL), 240)),
		zap.String("endpoint", resourceSniffLogURL(response.URL, 240)),
		zap.Int("mediaCount", len(extractedMedia)),
		zap.Int("hintCount", len(extractedHints)),
		zap.Strings("media", resourceSniffStructuredMediaSummaries(extractedMedia, 4)),
		zap.Strings("hints", resourceSniffNoMediaHintSummaries(extractedHints, 4)),
	)
	mergeStartedAt := time.Now()
	existingMediaCount := len(mediaItems)
	existingHintCount := len(hints)
	mediaItems = dedupeResourceStructuredMedia(append(mediaItems, extractedMedia...))
	hints = dedupeResourceNoMediaHints(append(hints, extractedHints...))
	zap.L().Debug("resource sniff douyin detail api merged",
		zap.String("awemeID", awemeID),
		zap.Int("existingMediaCount", existingMediaCount),
		zap.Int("extractedMediaCount", len(extractedMedia)),
		zap.Int("mergedMediaCount", len(mediaItems)),
		zap.Int("existingHintCount", existingHintCount),
		zap.Int("extractedHintCount", len(extractedHints)),
		zap.Int("mergedHintCount", len(hints)),
		zap.Int64("elapsedMs", time.Since(mergeStartedAt).Milliseconds()),
	)
	return mediaItems, hints
}

func resourceDouyinCurrentAwemeID(pageMeta map[string]string) string {
	if len(pageMeta) == 0 {
		return ""
	}
	if id := firstNonEmpty(strings.TrimSpace(pageMeta["awemeID"]), resourceDouyinIDFromURL(pageMeta["location"])); id != "" {
		return id
	}
	if !resourceDouyinIsRecommendPage(pageMeta["location"]) {
		return ""
	}
	visibleIDs := resourceDouyinVisibleAwemeIDsFromPageMeta(pageMeta, resourceDouyinVisibleAwemeIDLimit)
	if len(visibleIDs) == 0 {
		return ""
	}
	return strings.TrimSpace(visibleIDs[0])
}

func resourceDouyinHasStructuredMediaID(mediaItems []resourceStructuredMedia, awemeID string) bool {
	awemeID = strings.TrimSpace(awemeID)
	if awemeID == "" {
		return false
	}
	for _, item := range mediaItems {
		if strings.EqualFold(strings.TrimSpace(item.ID), awemeID) && strings.TrimSpace(item.VideoURL) != "" {
			return true
		}
	}
	return false
}

func resourceDouyinHasNoMediaHintID(hints []resourceNoMediaHint, awemeID string) bool {
	awemeID = strings.TrimSpace(awemeID)
	if awemeID == "" {
		return false
	}
	for _, hint := range hints {
		if strings.EqualFold(strings.TrimSpace(hint.Kind), resourceDouyinLiveHintKind) && strings.EqualFold(strings.TrimSpace(hint.ID), awemeID) {
			return true
		}
	}
	return false
}

func resourceDouyinFetchAwemeDetail(ctx context.Context, pageURL string, awemeID string) (resourceAPIResponse, bool) {
	awemeID = strings.TrimSpace(awemeID)
	if awemeID == "" {
		return resourceAPIResponse{}, false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, resourceDouyinAPIAugmentTimeout)
	defer cancel()

	endpoint := resourceDouyinAwemeDetailEndpoint(pageURL, awemeID)
	if endpoint == "" {
		zap.L().Debug("resource sniff douyin detail api skipped",
			zap.String("awemeID", awemeID),
			zap.String("pageURL", resourceSniffLogURL(pageURL, 240)),
			zap.String("reason", "empty_endpoint"),
		)
		return resourceAPIResponse{}, false
	}
	if response, ok := resourceDouyinFetchAwemeDetailInPage(requestCtx, pageURL, endpoint, awemeID); ok {
		return response, true
	}
	zap.L().Debug("resource sniff douyin detail api skipped",
		zap.String("awemeID", awemeID),
		zap.String("pageURL", resourceSniffLogURL(pageURL, 240)),
		zap.String("endpoint", resourceSniffLogURL(endpoint, 240)),
		zap.String("reason", "browser_fetch_failed"),
	)
	return resourceAPIResponse{}, false
}

type resourceDouyinBrowserDetailResponse struct {
	OK            bool              `json:"ok"`
	Status        int               `json:"status"`
	URL           string            `json:"url"`
	ContentType   string            `json:"contentType"`
	ContentLength string            `json:"contentLength"`
	TTLogID       string            `json:"ttLogID"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	Error         string            `json:"error"`
}

func resourceDouyinFetchAwemeDetailInPage(ctx context.Context, pageURL string, endpoint string, awemeID string) (resourceAPIResponse, bool) {
	if ctx == nil || !strings.EqualFold(extractRegistrableDomain(endpoint), "douyin.com") {
		return resourceAPIResponse{}, false
	}
	zap.L().Debug("resource sniff douyin detail api browser request",
		zap.String("awemeID", awemeID),
		zap.String("pageURL", resourceSniffLogURL(pageURL, 240)),
		zap.String("endpoint", resourceSniffLogURL(endpoint, 240)),
	)
	script := `(async () => {
		const endpoint = ` + strconv.Quote(endpoint) + `;
		try {
			const response = await fetch(endpoint, {
				credentials: "include",
				headers: { "Accept": "application/json, text/plain, */*" }
			});
			const body = await response.text();
			const headers = {};
			try {
				response.headers.forEach((value, key) => { headers[key] = value; });
			} catch (_) {}
			return {
				ok: true,
				status: response.status,
				url: response.url || endpoint,
				contentType: response.headers.get("content-type") || "",
				contentLength: response.headers.get("content-length") || "",
				ttLogID: response.headers.get("x-tt-logid") || "",
				headers,
				body
			};
		} catch (error) {
			return { ok: false, status: 0, url: endpoint, error: String(error && (error.message || error)) };
		}
	})()`
	var result resourceDouyinBrowserDetailResponse
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &result, resourceEvalAwaitPromise)); err != nil {
		zap.L().Debug("resource sniff douyin detail api browser request failed",
			zap.String("awemeID", awemeID),
			zap.String("endpoint", resourceSniffLogURL(endpoint, 240)),
			zap.String("reason", "evaluate_failed"),
			zap.Error(err),
		)
		return resourceAPIResponse{}, false
	}
	if !result.OK {
		zap.L().Debug("resource sniff douyin detail api browser response rejected",
			zap.String("awemeID", awemeID),
			zap.String("endpoint", resourceSniffLogURL(endpoint, 240)),
			zap.String("reason", "fetch_failed"),
			zap.String("error", strings.TrimSpace(result.Error)),
		)
		return resourceAPIResponse{}, false
	}
	body := []byte(result.Body)
	if result.Status < 200 || result.Status >= 300 {
		zap.L().Debug("resource sniff douyin detail api browser response rejected",
			zap.String("awemeID", awemeID),
			zap.String("endpoint", resourceSniffLogURL(firstNonEmpty(result.URL, endpoint), 240)),
			zap.Int("status", result.Status),
			zap.String("contentType", strings.TrimSpace(result.ContentType)),
			zap.String("ttLogID", strings.TrimSpace(result.TTLogID)),
			zap.String("reason", "bad_status"),
		)
		return resourceAPIResponse{}, false
	}
	if len(body) == 0 {
		zap.L().Debug("resource sniff douyin detail api browser response rejected",
			zap.String("awemeID", awemeID),
			zap.String("endpoint", resourceSniffLogURL(firstNonEmpty(result.URL, endpoint), 240)),
			zap.Int("status", result.Status),
			zap.String("contentType", strings.TrimSpace(result.ContentType)),
			zap.String("ttLogID", strings.TrimSpace(result.TTLogID)),
			zap.String("reason", "empty_body"),
		)
		return resourceAPIResponse{}, false
	}
	if len(body) > resourceMaxDouyinAPIResponseBodyBytes {
		zap.L().Debug("resource sniff douyin detail api browser response rejected",
			zap.String("awemeID", awemeID),
			zap.String("endpoint", resourceSniffLogURL(firstNonEmpty(result.URL, endpoint), 240)),
			zap.Int("status", result.Status),
			zap.String("contentType", strings.TrimSpace(result.ContentType)),
			zap.String("ttLogID", strings.TrimSpace(result.TTLogID)),
			zap.Int("bodyBytes", len(body)),
			zap.String("reason", "body_too_large"),
		)
		return resourceAPIResponse{}, false
	}
	sizeBytes, _ := strconv.ParseInt(strings.TrimSpace(result.ContentLength), 10, 64)
	if sizeBytes < 0 {
		sizeBytes = 0
	}
	responseHeaders := cloneStringMap(result.Headers)
	if responseHeaders == nil {
		responseHeaders = map[string]string{}
	}
	if strings.TrimSpace(result.ContentType) != "" {
		responseHeaders["content-type"] = strings.TrimSpace(result.ContentType)
	}
	if strings.TrimSpace(result.TTLogID) != "" {
		responseHeaders["x-tt-logid"] = strings.TrimSpace(result.TTLogID)
	}
	zap.L().Debug("resource sniff douyin detail api browser response accepted",
		zap.String("awemeID", awemeID),
		zap.String("endpoint", resourceSniffLogURL(firstNonEmpty(result.URL, endpoint), 240)),
		zap.Int("status", result.Status),
		zap.Int64("contentLength", sizeBytes),
		zap.Int("bodyBytes", len(body)),
		zap.String("contentType", strings.TrimSpace(result.ContentType)),
		zap.String("ttLogID", strings.TrimSpace(result.TTLogID)),
	)
	return resourceAPIResponse{
		URL:             firstNonEmpty(strings.TrimSpace(result.URL), endpoint),
		PageURL:         pageURL,
		ContentType:     strings.TrimSpace(result.ContentType),
		Status:          int64(result.Status),
		SizeBytes:       sizeBytes,
		RequestHeaders:  resourceDouyinBrowserFetchMediaHeaders(pageURL),
		ResponseHeaders: responseHeaders,
		Body:            body,
		SeenAt:          time.Now(),
	}, true
}

func resourceDouyinAwemeDetailEndpoint(pageURL string, awemeID string) string {
	awemeID = strings.TrimSpace(awemeID)
	if awemeID == "" {
		return ""
	}
	origin := "https://www.douyin.com"
	if parsed, err := url.Parse(strings.TrimSpace(pageURL)); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		origin = parsed.Scheme + "://" + parsed.Host
	}
	endpoint, err := url.Parse(origin + "/aweme/v1/web/aweme/detail/")
	if err != nil {
		return ""
	}
	query := endpoint.Query()
	query.Set("aweme_id", awemeID)
	endpoint.RawQuery = query.Encode()
	return endpoint.String()
}

func resourceDouyinBrowserFetchMediaHeaders(pageURL string) map[string]string {
	headers := map[string]string{
		"Accept":  "application/json, text/plain, */*",
		"Referer": firstNonEmpty(strings.TrimSpace(pageURL), "https://www.douyin.com/"),
	}
	return normalizeResourceDownloadHeaders(headers, pageURL)
}
