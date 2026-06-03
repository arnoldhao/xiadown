package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/chromedp/cdproto/network"

	"xiadown/internal/application/sniffprofile"
)

const (
	resourceMinCandidateBytes             = int64(256 * 1024)
	resourceMaxAPIResponseBodyBytes       = 2 * 1024 * 1024
	resourceMaxDouyinAPIResponseBodyBytes = 12 * 1024 * 1024
	resourceMaxPreviewImageBodyBytes      = 4 * 1024 * 1024
	resourceSniffNetworkTotalBufferBytes  = int64(120 * 1024 * 1024)
	resourceSniffNetworkResourceBytes     = int64(24 * 1024 * 1024)
)

type resourceMedia struct {
	URL            string
	PageURL        string
	Kind           string
	Title          string
	Author         string
	ThumbnailURL   string
	Domain         string
	Extractor      string
	ContentType    string
	MimeType       string
	Ext            string
	Width          int
	Height         int
	QualityHeight  int
	FormatNote     string
	VCodec         string
	ACodec         string
	SizeBytes      int64
	RequestHeaders map[string]string
	Subtitles      []resourceSubtitle
}

type resourceCandidate struct {
	requestID    network.RequestID
	url          string
	pageURL      string
	mimeType     string
	contentType  string
	resourceType string
	status       int64
	sizeBytes    int64
	headers      map[string]string
	score        int
	seenAt       time.Time
	structured   bool
}

type resourceStructuredMedia struct {
	ID            string
	VideoURL      string
	PageURL       string
	Title         string
	Author        string
	ThumbnailURL  string
	FormatID      string
	FormatNote    string
	VCodec        string
	ACodec        string
	Width         int
	Height        int
	QualityHeight int
	SizeBytes     int64
	SourceURL     string
	Headers       map[string]string
	Subtitles     []resourceSubtitle
	SeenAt        time.Time
}

type resourceNoMediaHint struct {
	Kind      string
	ID        string
	AltIDs    []string
	PageURL   string
	Title     string
	Author    string
	SourceURL string
	SeenAt    time.Time
}

type resourceAPIResponse struct {
	URL             string
	PageURL         string
	MimeType        string
	ContentType     string
	Status          int64
	ResourceType    network.ResourceType
	SizeBytes       int64
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
	Body            []byte
	SeenAt          time.Time
}

type resourcePreviewSnapshot struct {
	URL          string
	PageURL      string
	Kind         string
	MimeType     string
	ContentType  string
	ResourceType network.ResourceType
	Status       int64
	SizeBytes    int64
	Body         []byte
	SeenAt       time.Time
}

type resourceObservedResource struct {
	url          string
	pageURL      string
	mimeType     string
	contentType  string
	resourceType string
	status       int64
	sizeBytes    int64
	headers      map[string]string
	seenAt       time.Time
}

type resourceCaptureState struct {
	mu           sync.Mutex
	requests     map[network.RequestID]resourceRequest
	observed     []resourceObservedResource
	candidates   []resourceCandidate
	rejected     []resourceRejectedCandidate
	apiResponses []resourceAPIResponse
	previews     []resourcePreviewSnapshot
	subtitles    []resourceSubtitle
}

type resourceRequest struct {
	url             string
	documentURL     string
	headers         map[string]string
	responseURL     string
	mimeType        string
	contentType     string
	status          int64
	sizeBytes       int64
	responseHeaders map[string]string
	resourceType    network.ResourceType
	seenAt          time.Time
}

type resourceRejectedCandidate struct {
	url          string
	mimeType     string
	contentType  string
	resourceType string
	status       int64
	sizeBytes    int64
	reason       string
	score        int
	headers      map[string]string
	seenAt       time.Time
}

func newResourceCaptureState() *resourceCaptureState {
	return &resourceCaptureState{
		requests: make(map[network.RequestID]resourceRequest),
	}
}

func (state *resourceCaptureState) clear() {
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.requests = make(map[network.RequestID]resourceRequest)
	state.observed = nil
	state.candidates = nil
	state.rejected = nil
	state.apiResponses = nil
	state.previews = nil
	state.subtitles = nil
}

func (state *resourceCaptureState) recordRequest(id network.RequestID, requestURL string, documentURL string, headers network.Headers) {
	if state == nil || strings.TrimSpace(requestURL) == "" {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	item := state.requests[id]
	item.url = strings.TrimSpace(requestURL)
	item.documentURL = firstNonEmpty(strings.TrimSpace(documentURL), item.documentURL)
	item.headers = mergeHeaders(item.headers, headersToStringMap(headers))
	state.requests[id] = item
}

func (state *resourceCaptureState) recordRequestHeaders(id network.RequestID, headers network.Headers) {
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	item := state.requests[id]
	item.headers = mergeHeaders(item.headers, headersToStringMap(headers))
	state.requests[id] = item
}

func (state *resourceCaptureState) recordResponse(id network.RequestID, responseURL string, status int64, mimeType string, headers network.Headers, resourceType network.ResourceType) {
	if state == nil {
		return
	}
	contentType := strings.TrimSpace(headerValue(headers, "content-type"))
	sizeBytes := contentLengthFromHeaders(headers)
	requestHeaders := map[string]string(nil)
	state.mu.Lock()
	request := state.requests[id]
	if strings.TrimSpace(responseURL) == "" {
		responseURL = request.url
	}
	requestHeaders = cloneStringMap(request.headers)
	pageURL := strings.TrimSpace(request.documentURL)
	request.responseURL = strings.TrimSpace(responseURL)
	request.mimeType = strings.TrimSpace(mimeType)
	request.contentType = contentType
	request.status = status
	request.sizeBytes = sizeBytes
	request.responseHeaders = headersToStringMap(headers)
	request.resourceType = resourceType
	request.seenAt = time.Now()
	state.requests[id] = request
	state.mu.Unlock()
	state.recordObserved(resourceObservedResource{
		url:          strings.TrimSpace(responseURL),
		pageURL:      pageURL,
		mimeType:     strings.TrimSpace(mimeType),
		contentType:  contentType,
		resourceType: string(resourceType),
		status:       status,
		sizeBytes:    sizeBytes,
		headers:      requestHeaders,
		seenAt:       request.seenAt,
	})
	if status < 200 || status >= 300 || status == 204 {
		if isLikelyResourceMediaResponse(responseURL, mimeType, contentType, resourceType) {
			state.recordRejected(responseURL, mimeType, contentType, resourceType, status, sizeBytes, 0, fmt.Sprintf("http_status_%d", status), requestHeaders)
		}
		return
	}
	if subtitle, ok := resourceSubtitleFromResponse(responseURL, pageURL, mimeType, contentType, string(resourceType), requestHeaders, request.seenAt); ok {
		state.recordSubtitle(subtitle)
	}

	score, rejectReason := scoreResourceCandidateWithReason(responseURL, mimeType, headers, resourceType)
	if score <= 0 {
		if rejectReason != "" || isLikelyResourceMediaResponse(responseURL, mimeType, contentType, resourceType) {
			state.recordRejected(responseURL, mimeType, contentType, resourceType, status, sizeBytes, score, rejectReason, requestHeaders)
		}
		return
	}
	candidate := resourceCandidate{
		requestID:    id,
		url:          strings.TrimSpace(responseURL),
		pageURL:      pageURL,
		mimeType:     strings.TrimSpace(mimeType),
		contentType:  contentType,
		resourceType: string(resourceType),
		status:       status,
		sizeBytes:    sizeBytes,
		headers:      requestHeaders,
		score:        score,
		seenAt:       time.Now(),
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	state.candidates = append(state.candidates, candidate)
}

func (state *resourceCaptureState) recordObserved(resource resourceObservedResource) {
	if state == nil || strings.TrimSpace(resource.url) == "" {
		return
	}
	resource.url = strings.TrimSpace(resource.url)
	resource.pageURL = strings.TrimSpace(resource.pageURL)
	resource.mimeType = strings.TrimSpace(resource.mimeType)
	resource.contentType = strings.TrimSpace(resource.contentType)
	resource.resourceType = strings.TrimSpace(resource.resourceType)
	resource.headers = cloneStringMap(resource.headers)
	if resource.seenAt.IsZero() {
		resource.seenAt = time.Now()
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if resourceObservedAlreadySeen(state.observed, resource) {
		return
	}
	state.observed = append(state.observed, resource)
	state.observed = trimObservedResources(state.observed)
}

func (state *resourceCaptureState) shouldCaptureResponseBody(id network.RequestID) bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	request := state.requests[id]
	state.mu.Unlock()
	return shouldCaptureResourceAPIResponse(request) || shouldCaptureResourcePreviewSnapshot(request)
}

func shouldCaptureResourceAPIResponse(request resourceRequest) bool {
	if request.status < 200 || request.status >= 300 || request.status == 204 {
		return false
	}
	if request.sizeBytes > int64(resourceMaxAPIResponseBodyBytesForRequest(request)) {
		return false
	}
	if shouldCaptureResourceManifestResponse(request) {
		return true
	}
	lowerURL := strings.ToLower(firstNonEmpty(request.responseURL, request.url))
	lowerMime := strings.ToLower(strings.TrimSpace(request.mimeType))
	lowerContentType := strings.ToLower(strings.TrimSpace(request.contentType))
	if strings.Contains(lowerMime, "json") || strings.Contains(lowerContentType, "json") {
		return true
	}
	if strings.EqualFold(string(request.resourceType), "XHR") || strings.EqualFold(string(request.resourceType), "Fetch") {
		return strings.Contains(lowerURL, "aweme") ||
			strings.Contains(lowerURL, "graphql") ||
			strings.Contains(lowerURL, "note") ||
			strings.Contains(lowerURL, "item") ||
			strings.Contains(lowerURL, "detail") ||
			strings.Contains(lowerURL, "post") ||
			strings.Contains(lowerURL, "feed") ||
			strings.Contains(lowerURL, "search")
	}
	return false
}

func shouldCaptureResourceManifestResponse(request resourceRequest) bool {
	return resourceSniffRawManifestStream(
		firstNonEmpty(request.responseURL, request.url),
		request.mimeType,
		request.contentType,
	)
}

func shouldCaptureResourcePreviewSnapshot(request resourceRequest) bool {
	if request.status < 200 || request.status >= 300 || request.status == 204 {
		return false
	}
	if request.sizeBytes > resourceMaxPreviewImageBodyBytes {
		return false
	}
	return resourcePreviewKindForRequest(request) == "image" &&
		resourcePreviewDisplaySafeMimeType(resourcePreviewMimeType(request))
}

func (state *resourceCaptureState) recordResponseBody(id network.RequestID, data []byte) {
	if state == nil || len(data) == 0 {
		return
	}
	state.mu.Lock()
	request := state.requests[id]
	state.mu.Unlock()
	if len(data) > resourceMaxAPIResponseBodyBytesForRequest(request) {
		if !shouldCaptureResourcePreviewSnapshot(request) || len(data) > resourceMaxPreviewImageBodyBytes {
			return
		}
	}
	if shouldCaptureResourceAPIResponse(request) && len(data) <= resourceMaxAPIResponseBodyBytesForRequest(request) {
		response := resourceAPIResponse{
			URL:             firstNonEmpty(request.responseURL, request.url),
			PageURL:         request.documentURL,
			MimeType:        request.mimeType,
			ContentType:     request.contentType,
			Status:          request.status,
			ResourceType:    request.resourceType,
			SizeBytes:       request.sizeBytes,
			RequestHeaders:  cloneStringMap(request.headers),
			ResponseHeaders: cloneStringMap(request.responseHeaders),
			Body:            data,
			SeenAt:          request.seenAt,
		}
		state.recordAPIResponse(response)
		subtitles := resourceSubtitlesFromAPIResponse(response)
		for _, subtitle := range subtitles {
			state.recordSubtitle(subtitle)
		}
	}
	if preview, ok := resourcePreviewSnapshotFromResponse(request, data); ok {
		state.recordPreviewSnapshot(preview)
	}
}

func resourceMaxAPIResponseBodyBytesForRequest(request resourceRequest) int {
	lowerURL := strings.ToLower(firstNonEmpty(request.responseURL, request.url))
	if strings.EqualFold(extractRegistrableDomain(lowerURL), "douyin.com") &&
		(strings.Contains(lowerURL, "/aweme/v1/web/tab/feed/") ||
			strings.Contains(lowerURL, "/aweme/v2/web/module/feed/") ||
			strings.Contains(lowerURL, "/aweme/v1/web/aweme/detail/")) {
		return resourceMaxDouyinAPIResponseBodyBytes
	}
	return resourceMaxAPIResponseBodyBytes
}

func (state *resourceCaptureState) recordAPIResponse(response resourceAPIResponse) {
	if state == nil || len(response.Body) == 0 {
		return
	}
	response.URL = strings.TrimSpace(response.URL)
	response.PageURL = strings.TrimSpace(response.PageURL)
	response.MimeType = strings.TrimSpace(response.MimeType)
	response.ContentType = strings.TrimSpace(response.ContentType)
	response.RequestHeaders = cloneStringMap(response.RequestHeaders)
	response.ResponseHeaders = cloneStringMap(response.ResponseHeaders)
	response.Body = append([]byte(nil), response.Body...)
	if response.SeenAt.IsZero() {
		response.SeenAt = time.Now()
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if replaced := replaceResourceAPIResponseIfBetter(state.apiResponses, response); replaced {
		return
	}
	if resourceAPIResponseAlreadySeen(state.apiResponses, response) {
		return
	}
	state.apiResponses = append(state.apiResponses, response)
}

func resourcePreviewSnapshotFromResponse(request resourceRequest, data []byte) (resourcePreviewSnapshot, bool) {
	if len(data) == 0 || len(data) > resourceMaxPreviewImageBodyBytes || !shouldCaptureResourcePreviewSnapshot(request) {
		return resourcePreviewSnapshot{}, false
	}
	kind := resourcePreviewKindForRequest(request)
	if kind == "" {
		return resourcePreviewSnapshot{}, false
	}
	mimeType := firstNonEmpty(resourcePreviewMimeTypeFromBody(data), resourcePreviewMimeType(request))
	if mimeType == "" {
		return resourcePreviewSnapshot{}, false
	}
	if !resourcePreviewDisplaySafeMimeType(mimeType) {
		return resourcePreviewSnapshot{}, false
	}
	sizeBytes := request.sizeBytes
	if sizeBytes <= 0 {
		sizeBytes = int64(len(data))
	}
	return resourcePreviewSnapshot{
		URL:          firstNonEmpty(request.responseURL, request.url),
		PageURL:      request.documentURL,
		Kind:         kind,
		MimeType:     mimeType,
		ContentType:  request.contentType,
		ResourceType: request.resourceType,
		Status:       request.status,
		SizeBytes:    sizeBytes,
		Body:         append([]byte(nil), data...),
		SeenAt:       request.seenAt,
	}, true
}

func resourcePreviewKindForRequest(request resourceRequest) string {
	lowerURL := strings.ToLower(firstNonEmpty(request.responseURL, request.url))
	lowerMime := strings.ToLower(resourcePreviewMimeType(request))
	if strings.HasPrefix(lowerMime, "image/") ||
		strings.EqualFold(string(request.resourceType), "Image") ||
		strings.Contains(lowerURL, ".jpg") ||
		strings.Contains(lowerURL, ".jpeg") ||
		strings.Contains(lowerURL, ".png") ||
		strings.Contains(lowerURL, ".webp") ||
		strings.Contains(lowerURL, ".gif") ||
		strings.Contains(lowerURL, ".avif") ||
		strings.Contains(lowerURL, ".bmp") ||
		strings.Contains(lowerURL, ".ico") {
		return "image"
	}
	return ""
}

func resourcePreviewMimeType(request resourceRequest) string {
	mimeType := strings.TrimSpace(firstNonEmpty(request.mimeType, request.contentType))
	if index := strings.Index(mimeType, ";"); index >= 0 {
		mimeType = strings.TrimSpace(mimeType[:index])
	}
	if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return normalizeResourcePreviewMimeType(mimeType)
	}
	lowerURL := strings.ToLower(firstNonEmpty(request.responseURL, request.url))
	switch {
	case strings.Contains(lowerURL, ".jpg"), strings.Contains(lowerURL, ".jpeg"):
		return "image/jpeg"
	case strings.Contains(lowerURL, ".png"):
		return "image/png"
	case strings.Contains(lowerURL, ".webp"):
		return "image/webp"
	case strings.Contains(lowerURL, ".gif"):
		return "image/gif"
	case strings.Contains(lowerURL, ".avif"):
		return "image/avif"
	case strings.Contains(lowerURL, ".bmp"):
		return "image/bmp"
	case strings.Contains(lowerURL, ".ico"):
		return "image/x-icon"
	default:
		return mimeType
	}
}

func normalizeResourcePreviewMimeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/jpg", "image/pjpeg":
		return "image/jpeg"
	case "image/x-png":
		return "image/png"
	case "image/x-webp":
		return "image/webp"
	case "image/avif-sequence":
		return "image/avif"
	default:
		return strings.TrimSpace(value)
	}
}

func resourcePreviewDisplaySafeMimeType(value string) bool {
	switch strings.ToLower(normalizeResourcePreviewMimeType(value)) {
	case "image/avif",
		"image/bmp",
		"image/gif",
		"image/jpeg",
		"image/png",
		"image/webp",
		"image/x-icon",
		"image/vnd.microsoft.icon":
		return true
	default:
		return false
	}
}

func resourcePreviewMimeTypeFromBody(data []byte) string {
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(data) >= 6 {
		signature := string(data[:6])
		if signature == "GIF87a" || signature == "GIF89a" {
			return "image/gif"
		}
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		brandWindow := data[8:minInt(len(data), 40)]
		if strings.Contains(string(brandWindow), "avif") || strings.Contains(string(brandWindow), "avis") {
			return "image/avif"
		}
	}
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 2 && string(data[:2]) == "BM" {
		return "image/bmp"
	}
	if len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 && data[3] == 0x00 {
		return "image/x-icon"
	}
	return ""
}

func (state *resourceCaptureState) recordPreviewSnapshot(snapshot resourcePreviewSnapshot) {
	if state == nil || len(snapshot.Body) == 0 {
		return
	}
	snapshot.URL = strings.TrimSpace(snapshot.URL)
	snapshot.PageURL = strings.TrimSpace(snapshot.PageURL)
	snapshot.Kind = strings.TrimSpace(snapshot.Kind)
	snapshot.MimeType = strings.TrimSpace(snapshot.MimeType)
	snapshot.ContentType = strings.TrimSpace(snapshot.ContentType)
	snapshot.Body = append([]byte(nil), snapshot.Body...)
	if snapshot.SeenAt.IsZero() {
		snapshot.SeenAt = time.Now()
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if resourcePreviewAlreadySeen(state.previews, snapshot) {
		return
	}
	state.previews = append(state.previews, snapshot)
}

func resourceAPIResponseAlreadySeen(existing []resourceAPIResponse, response resourceAPIResponse) bool {
	for _, item := range existing {
		if resourceComparableURL(item.URL, false) == resourceComparableURL(response.URL, false) &&
			resourceComparableURL(item.PageURL, false) == resourceComparableURL(response.PageURL, false) {
			return true
		}
	}
	return false
}

func replaceResourceAPIResponseIfBetter(existing []resourceAPIResponse, response resourceAPIResponse) bool {
	if len(existing) == 0 || !resourceSniffRawHLSStream(response.URL, response.MimeType, response.ContentType) || !resourceHLSManifestDownloadable(response.Body) {
		return false
	}
	for index, item := range existing {
		if resourceComparableURL(item.URL, false) != resourceComparableURL(response.URL, false) ||
			resourceComparableURL(item.PageURL, false) != resourceComparableURL(response.PageURL, false) {
			continue
		}
		if !resourceHLSManifestDownloadable(item.Body) {
			existing[index] = response
		}
		return true
	}
	return false
}

func resourcePreviewAlreadySeen(existing []resourcePreviewSnapshot, snapshot resourcePreviewSnapshot) bool {
	for _, item := range existing {
		if resourceComparableURL(item.URL, false) == resourceComparableURL(snapshot.URL, false) &&
			resourceComparableURL(item.PageURL, false) == resourceComparableURL(snapshot.PageURL, false) &&
			strings.TrimSpace(item.Kind) == strings.TrimSpace(snapshot.Kind) {
			return true
		}
	}
	return false
}

func resourceObservedAlreadySeen(existing []resourceObservedResource, resource resourceObservedResource) bool {
	for _, item := range existing {
		if resourceComparableURL(item.url, false) == resourceComparableURL(resource.url, false) &&
			resourceComparableURL(item.pageURL, false) == resourceComparableURL(resource.pageURL, false) &&
			strings.TrimSpace(item.resourceType) == strings.TrimSpace(resource.resourceType) {
			return true
		}
	}
	return false
}

func trimObservedResources(values []resourceObservedResource) []resourceObservedResource {
	const maxObservedResources = 10000
	if len(values) <= maxObservedResources {
		return values
	}
	return append([]resourceObservedResource(nil), values[len(values)-maxObservedResources:]...)
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func resourceStructuredMediaAlreadySeen(existing []resourceStructuredMedia, media resourceStructuredMedia) bool {
	for _, item := range existing {
		if strings.TrimSpace(item.VideoURL) != "" && resourceComparableURL(item.VideoURL, false) == resourceComparableURL(media.VideoURL, false) {
			return true
		}
	}
	return false
}

func scoreResourceStructuredMediaCandidate(media resourceStructuredMedia) int {
	score := 120
	if media.Width > 0 && media.Height > 0 {
		score += 30
	}
	if media.QualityHeight > 0 {
		score += minInt(media.QualityHeight/10, 80)
	}
	if media.SizeBytes >= resourceMinCandidateBytes {
		score += 20
	}
	if strings.TrimSpace(media.ID) != "" {
		score += 10
	}
	return score
}

func (state *resourceCaptureState) best() (resourceCandidate, bool) {
	if state == nil {
		return resourceCandidate{}, false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	var best resourceCandidate
	found := false
	for _, candidate := range state.candidates {
		if strings.TrimSpace(candidate.url) == "" {
			continue
		}
		if !found || candidate.score > best.score || (candidate.score == best.score && candidate.sizeBytes > best.sizeBytes) {
			best = candidate
			found = true
		}
	}
	if !found {
		return resourceCandidate{}, false
	}
	return best, true
}

func (state *resourceCaptureState) bestForPage(pageMeta map[string]string, since time.Time) (resourceCandidate, bool) {
	return state.bestForPageUsing(resourceDefaultSiteRules{}, pageMeta, since)
}

func (state *resourceCaptureState) bestForPageUsing(extractor resourceExtractor, pageMeta map[string]string, since time.Time) (resourceCandidate, bool) {
	if state == nil {
		return resourceCandidate{}, false
	}
	if extractor == nil {
		extractor = resourceDefaultSiteRules{}
	}
	return extractor.SelectCandidate(state.candidatesSnapshot(), pageMeta, since)
}

func (state *resourceCaptureState) candidatesSnapshot() []resourceCandidate {
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return append([]resourceCandidate(nil), state.candidates...)
}

func (state *resourceCaptureState) observedSnapshot() []resourceObservedResource {
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	result := make([]resourceObservedResource, 0, len(state.observed))
	for _, item := range state.observed {
		item.headers = cloneStringMap(item.headers)
		result = append(result, item)
	}
	return result
}

func (state *resourceCaptureState) apiResponsesSnapshot() []resourceAPIResponse {
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return cloneResourceAPIResponses(state.apiResponses)
}

func (state *resourceCaptureState) previewsSnapshot() []resourcePreviewSnapshot {
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return cloneResourcePreviewSnapshots(state.previews)
}

func cloneResourcePreviewSnapshots(values []resourcePreviewSnapshot) []resourcePreviewSnapshot {
	if len(values) == 0 {
		return nil
	}
	result := make([]resourcePreviewSnapshot, 0, len(values))
	for _, value := range values {
		value.Body = append([]byte(nil), value.Body...)
		result = append(result, value)
	}
	return result
}

func (state *resourceCaptureState) recordSubtitle(subtitle resourceSubtitle) {
	if state == nil || !subtitle.Valid() {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if resourceSubtitleAlreadySeen(state.subtitles, subtitle) {
		return
	}
	state.subtitles = append(state.subtitles, subtitle)
}

func (state *resourceCaptureState) subtitlesSnapshot() []resourceSubtitle {
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return append([]resourceSubtitle(nil), state.subtitles...)
}

func bestResourceCandidateMatchingVideoSource(candidates []resourceCandidate, sources []string) (resourceCandidate, bool) {
	if len(candidates) == 0 || len(sources) == 0 {
		return resourceCandidate{}, false
	}
	var best resourceCandidate
	bestScore := 0
	found := false
	for _, candidate := range candidates {
		matchScore := resourceVideoSourceMatchScore(candidate.url, sources)
		if matchScore <= 0 {
			continue
		}
		score := matchScore*100000 + candidate.score*1000 + resourceCandidateSizeScore(candidate.sizeBytes)
		if !found || score > bestScore || (score == bestScore && resourceCandidateBetter(candidate, best)) {
			best = candidate
			bestScore = score
			found = true
		}
	}
	return best, found
}

func resourceCandidateBetter(left resourceCandidate, right resourceCandidate) bool {
	switch {
	case left.score != right.score:
		return left.score > right.score
	case left.sizeBytes != right.sizeBytes:
		return left.sizeBytes > right.sizeBytes
	case !left.seenAt.Equal(right.seenAt):
		return left.seenAt.After(right.seenAt)
	default:
		return strings.TrimSpace(left.url) < strings.TrimSpace(right.url)
	}
}

func resourceCandidateSizeScore(sizeBytes int64) int {
	if sizeBytes <= 0 {
		return 0
	}
	megabytes := sizeBytes / (1024 * 1024)
	if megabytes > 500 {
		megabytes = 500
	}
	return int(megabytes)
}

func resourceVideoSourcesFromPageMeta(pageMeta map[string]string) []string {
	if len(pageMeta) == 0 {
		return nil
	}
	sources := make([]string, 0, 4)
	add := func(value string) {
		trimmed := strings.TrimSpace(value)
		lower := strings.ToLower(trimmed)
		if trimmed == "" || strings.HasPrefix(lower, "blob:") || strings.HasPrefix(lower, "data:") {
			return
		}
		sources = append(sources, trimmed)
	}
	add(pageMeta["apiVideoURL"])
	add(pageMeta["videoSrc"])
	add(pageMeta["videoCurrentSrc"])
	type videoItem struct {
		CurrentSrc string `json:"currentSrc"`
		Src        string `json:"src"`
	}
	var items []videoItem
	if raw := strings.TrimSpace(pageMeta["videoItems"]); raw != "" && json.Unmarshal([]byte(raw), &items) == nil {
		for index, item := range items {
			if index > 0 {
				break
			}
			add(item.CurrentSrc)
			add(item.Src)
		}
	}
	return dedupeResourceStrings(sources)
}

func resourceVideoSourceMatchScore(candidateURL string, sources []string) int {
	candidate := resourceComparableURL(candidateURL, false)
	candidateNoQuery := resourceComparableURL(candidateURL, true)
	if candidate == "" {
		return 0
	}
	best := 0
	for index, source := range sources {
		source = resourceComparableURL(source, false)
		sourceNoQuery := resourceComparableURL(source, true)
		if source == "" {
			continue
		}
		score := 0
		switch {
		case candidate == source:
			score = 1000
		case candidateNoQuery != "" && candidateNoQuery == sourceNoQuery:
			score = 900
		case len(source) >= 64 && strings.HasPrefix(candidate, source):
			score = 800
		case len(candidate) >= 64 && strings.HasPrefix(source, candidate):
			score = 700
		case len(sourceNoQuery) >= 32 && candidateNoQuery != "" && strings.HasPrefix(candidateNoQuery, sourceNoQuery):
			score = 600
		}
		if score > 0 {
			score -= index
			if score > best {
				best = score
			}
		}
	}
	return best
}

func resourceComparableURL(rawURL string, dropQuery bool) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return trimmed
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if dropQuery {
		parsed.RawQuery = ""
	}
	return parsed.String()
}

func resourceURLHasQueryKey(rawURL string, key string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || strings.TrimSpace(key) == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	_, ok := parsed.Query()[key]
	return ok
}

func dedupeResourceStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := resourceComparableURL(trimmed, false)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func (state *resourceCaptureState) recordRejected(rawURL string, mimeType string, contentType string, resourceType network.ResourceType, status int64, sizeBytes int64, score int, reason string, requestHeaders map[string]string) {
	if state == nil {
		return
	}
	rejected := resourceRejectedCandidate{
		url:          strings.TrimSpace(rawURL),
		mimeType:     strings.TrimSpace(mimeType),
		contentType:  strings.TrimSpace(contentType),
		resourceType: string(resourceType),
		status:       status,
		sizeBytes:    sizeBytes,
		score:        score,
		reason:       strings.TrimSpace(reason),
		headers:      cloneStringMap(requestHeaders),
		seenAt:       time.Now(),
	}
	state.mu.Lock()
	state.rejected = append(state.rejected, rejected)
	state.mu.Unlock()
}

func (state *resourceCaptureState) snapshot() ([]resourceCandidate, []resourceRejectedCandidate) {
	if state == nil {
		return nil, nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	candidates := append([]resourceCandidate(nil), state.candidates...)
	rejected := append([]resourceRejectedCandidate(nil), state.rejected...)
	return candidates, rejected
}

func (service *LibraryService) preferredResourceBrowser(ctx context.Context) string {
	if service == nil || service.settings == nil {
		return ""
	}
	settings, err := service.settings.GetSettings(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(settings.SniffBrowser)
}

func (service *LibraryService) resourceConnectorProfilePath(ctx context.Context, rawURL string) (string, error) {
	if service == nil {
		return "", nil
	}
	return sniffprofile.PathForPreferredBrowser(service.preferredResourceBrowser(ctx))
}

func (service *LibraryService) resourceMediaFromCandidate(pageURL string, pageDomain string, candidate resourceCandidate, pageMeta map[string]string) resourceMedia {
	return resourceExtractorForURL(pageURL).MediaFromCandidate(service, pageURL, pageDomain, candidate, pageMeta)
}

func resourceCleanGenericTitle(value string) string {
	title := resourceCleanMetadataText(value)
	lower := strings.ToLower(title)
	if strings.Contains(title, "验证码") ||
		strings.Contains(title, "captcha") ||
		lower == "douyin" {
		return ""
	}
	return title
}

func resourceCleanDouyinTitle(value string) string {
	title := resourceCleanMetadataText(value)
	for _, suffix := range []string{
		" - 抖音",
		" | 抖音",
		"_抖音",
		"- Douyin",
		"| Douyin",
	} {
		if strings.HasSuffix(title, suffix) {
			title = strings.TrimSpace(strings.TrimSuffix(title, suffix))
		}
	}
	lower := strings.ToLower(title)
	if title == "抖音" ||
		strings.Contains(title, "验证码") ||
		strings.Contains(title, "captcha") ||
		strings.Contains(title, "记录美好生活") ||
		lower == "douyin" {
		return ""
	}
	return title
}

func resourceCleanAuthor(value string) string {
	author := resourceCleanMetadataText(value)
	if !resourceLooksLikeAuthor(author) {
		return ""
	}
	return author
}

func resourceLooksLikeAuthor(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, keyword := range []string{
		"我的", "首页", "朋友", "消息", "登录", "登陆", "发布", "拍摄", "热点", "商城",
		"关注", "粉丝", "获赞", "点赞", "评论", "分享", "收藏", "更多", "推荐", "搜索", "直播",
		"follow", "followers", "likes", "comments", "share",
	} {
		if strings.Contains(lower, keyword) {
			return false
		}
	}
	hasLetter := false
	for _, item := range trimmed {
		if unicode.IsLetter(item) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}
	if runes := []rune(trimmed); len(runes) > 80 {
		return false
	}
	return true
}

func resourceSecureImageURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || !strings.EqualFold(parsed.Scheme, "http") {
		return trimmed
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return trimmed
	}
	parsed.Scheme = "https"
	return parsed.String()
}

func resourceCleanMetadataText(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	cleaned := strings.Join(fields, " ")
	cleaned = resourceTrimWrappedQuotes(cleaned)
	if runes := []rune(cleaned); len(runes) > 180 {
		cleaned = strings.TrimSpace(string(runes[:180]))
	}
	return cleaned
}

func resourceTrimWrappedQuotes(value string) string {
	cleaned := strings.TrimSpace(value)
	for {
		runes := []rune(cleaned)
		if len(runes) < 2 {
			return cleaned
		}
		if !resourceIsWrappedQuotePair(runes[0], runes[len(runes)-1]) {
			return cleaned
		}
		next := strings.TrimSpace(string(runes[1 : len(runes)-1]))
		if next == cleaned {
			return cleaned
		}
		cleaned = next
	}
}

func resourceIsWrappedQuotePair(left rune, right rune) bool {
	switch {
	case left == '"' && right == '"':
		return true
	case left == '\'' && right == '\'':
		return true
	case left == '“' && right == '”':
		return true
	case left == '‘' && right == '’':
		return true
	default:
		return false
	}
}

func resourcePageKickScript() string {
	return `(() => {
		try {
			window.scrollBy(0, 1);
			for (const video of document.querySelectorAll("video")) {
				video.muted = true;
				video.playsInline = true;
				const result = video.play();
				if (result && typeof result.catch === "function") {
					result.catch(() => {});
				}
			}
		} catch (_) {}
		return true;
	})()`
}

func resourcePageIdentityScript() string {
	return `(() => ({
		location: String(window.location.href || "").slice(0, 500),
		title: String(document.title || "").replace(/\s+/g, " ").trim(),
		visibilityState: String(document.visibilityState || ""),
		hasFocus: Boolean(document.hasFocus && document.hasFocus())
	}))()`
}

func resourceGenericPageMetaScript() string {
	return `(() => {
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
		const videos = Array.from(document.querySelectorAll("video")).map((video) => {
			const rect = video.getBoundingClientRect();
			const visibleWidth = Math.max(0, Math.min(rect.right, window.innerWidth) - Math.max(rect.left, 0));
			const visibleHeight = Math.max(0, Math.min(rect.bottom, window.innerHeight) - Math.max(rect.top, 0));
			const visibleArea = Math.round(visibleWidth * visibleHeight);
			return {
				width: Number(video.videoWidth || 0),
				height: Number(video.videoHeight || 0),
				currentSrc: (video.currentSrc || "").slice(0, 2000),
				src: (video.src || "").slice(0, 2000),
				poster: (video.poster || "").slice(0, 2000),
				paused: Boolean(video.paused),
				visibleArea
			};
		}).sort((left, right) => right.visibleArea - left.visibleArea);
		const primaryVideo = videos[0] || {};
		return {
			location: (window.location.href || "").slice(0, 500),
			title: clean(document.title || ""),
			ogTitle: meta('meta[property="og:title"]'),
			description: meta('meta[name="description"]') || meta('meta[property="og:description"]'),
			image: meta('meta[property="og:image"]'),
			thumbnail: meta('meta[name="thumbnail"]') || meta('meta[name="twitter:image"]'),
			author: meta('meta[name="author"]') || meta('meta[property="article:author"]'),
			videoTitle: "",
				videoCount: String(document.querySelectorAll("video").length),
			videoCurrentSrc: primaryVideo.currentSrc || "",
			videoSrc: primaryVideo.src || "",
			videoItems: JSON.stringify(videos.slice(0, 1)),
			videoWidth: primaryVideo.width ? String(primaryVideo.width) : "",
			videoHeight: primaryVideo.height ? String(primaryVideo.height) : "",
			loginHint: ""
		};
	})()`
}

func resourceDouyinPageMetaScript() string {
	return `(async () => {
		const clean = (value) => String(value || "").replace(/\s+/g, " ").trim();
		const meta = (selector) => {
			const node = document.querySelector(selector);
			return node ? clean(node.getAttribute("content") || "") : "";
		};
		const awemeIDFromURL = (value) => {
			try {
				const url = new URL(value || "", window.location.href || "https://www.douyin.com");
				for (const key of ["modal_id", "aweme_id", "awemeId", "gid", "group_id", "groupId", "item_id", "itemId"]) {
					const id = clean(url.searchParams.get(key) || "");
					if (/^\d{8,}$/.test(id)) return id;
				}
				const match = url.pathname.match(/\/(?:video|note)\/(\d+)/);
				return match ? match[1] : "";
			} catch (_) {
				const match = String(value || "").match(/(?:modal_id=|aweme_id=|gid=|\/(?:video|note)\/)(\d+)/);
				return match ? match[1] : "";
			}
		};
		const liveIDFromURL = (value) => {
			try {
				const url = new URL(value || "", window.location.href || "https://www.douyin.com");
				for (const key of ["room_id", "roomId", "webcast_room_id", "webcastRoomId", "live_room_id", "liveRoomId"]) {
					const id = clean(url.searchParams.get(key) || "");
					if (/^\d{6,}$/.test(id)) return id;
				}
				const host = (url.hostname || "").toLowerCase();
				if (host === "live.douyin.com" || host.endsWith(".live.douyin.com")) {
					const match = url.pathname.match(/\/(\d{6,})/);
					return match ? match[1] : "";
				}
				const match = url.pathname.match(/\/(?:live|webcast)\/(\d{6,})/);
				return match ? match[1] : "";
			} catch (_) {
				const match = String(value || "").match(/(?:room_id=|roomId=|webcast_room_id=|webcastRoomId=|live_room_id=|liveRoomId=|live\.douyin\.com\/|\/(?:live|webcast)\/)(\d{6,})/);
				return match ? match[1] : "";
			}
		};
		const currentAwemeID = awemeIDFromURL(window.location.href || "");
		const nodeIsRenderable = (node) => {
			for (let current = node; current && current.nodeType === 1; current = current.parentElement) {
				const style = getComputedStyle(current);
				if (style.display === "none" || style.visibility === "hidden" || Number(style.opacity || 1) <= 0.01) {
					return false;
				}
			}
			return true;
		};
		const foregroundScoreForVideo = (video, rect) => {
			const points = [
				[rect.left + rect.width / 2, rect.top + rect.height / 2],
				[rect.left + rect.width * 0.35, rect.top + rect.height / 2],
				[rect.left + rect.width * 0.65, rect.top + rect.height / 2],
				[rect.left + rect.width / 2, rect.top + rect.height * 0.35],
				[rect.left + rect.width / 2, rect.top + rect.height * 0.65]
			];
			let score = 0;
			for (const [rawX, rawY] of points) {
				const x = Math.min(Math.max(Math.round(rawX), 0), Math.max(window.innerWidth - 1, 0));
				const y = Math.min(Math.max(Math.round(rawY), 0), Math.max(window.innerHeight - 1, 0));
				const top = document.elementFromPoint(x, y);
				if (!top) continue;
				if (top === video || video.contains(top)) {
					score += 2;
					continue;
				}
				for (let current = top, depth = 0; current && depth < 5; current = current.parentElement, depth += 1) {
					if (current.contains && current.contains(video)) {
						score += 1;
						break;
					}
				}
			}
			return score;
		};
		const videos = Array.from(document.querySelectorAll("video")).map((video) => {
				const rect = video.getBoundingClientRect();
				const renderable = nodeIsRenderable(video);
				const visibleWidth = renderable ? Math.max(0, Math.min(rect.right, window.innerWidth) - Math.max(rect.left, 0)) : 0;
				const visibleHeight = renderable ? Math.max(0, Math.min(rect.bottom, window.innerHeight) - Math.max(rect.top, 0)) : 0;
				const visibleArea = Math.round(visibleWidth * visibleHeight);
				const width = Number(video.videoWidth || 0);
				const height = Number(video.videoHeight || 0);
				return {
					width,
					height,
					readyState: Number(video.readyState || 0),
					networkState: Number(video.networkState || 0),
					currentSrc: (video.currentSrc || "").slice(0, 2000),
					src: (video.src || "").slice(0, 2000),
					poster: (video.poster || "").slice(0, 2000),
					paused: Boolean(video.paused),
					currentTime: Number(video.currentTime || 0),
					visibleArea,
					foregroundScore: visibleArea > 0 ? foregroundScoreForVideo(video, rect) : 0
				};
			}).sort((left, right) => {
				if (right.foregroundScore !== left.foregroundScore) return right.foregroundScore - left.foregroundScore;
				if (right.visibleArea !== left.visibleArea) return right.visibleArea - left.visibleArea;
				if (Number(left.paused) !== Number(right.paused)) return Number(left.paused) - Number(right.paused);
				return (right.width * right.height) - (left.width * left.height);
			});
		const topVideos = videos.slice(0, 5);
		const primaryVideo = topVideos[0] || {};
			const visibleAwemeIDs = (() => {
				const entries = [];
				const viewportCenterX = Math.max(0, window.innerWidth || 0) / 2;
				const viewportCenterY = Math.max(0, window.innerHeight || 0) / 2;
			for (const link of document.querySelectorAll("a[href]")) {
				const id = awemeIDFromURL(link.href || "");
				if (!id) continue;
				const rect = link.getBoundingClientRect();
				const visibleWidth = Math.max(0, Math.min(rect.right, window.innerWidth) - Math.max(rect.left, 0));
				const visibleHeight = Math.max(0, Math.min(rect.bottom, window.innerHeight) - Math.max(rect.top, 0));
				const visibleArea = Math.round(visibleWidth * visibleHeight);
				if (visibleArea <= 0) continue;
				if (!nodeIsRenderable(link)) continue;
				const centerX = rect.left + rect.width / 2;
				const centerY = rect.top + rect.height / 2;
				const distance = Math.hypot(centerX - viewportCenterX, centerY - viewportCenterY);
				entries.push({ id, visibleArea, distance });
			}
			entries.sort((left, right) => {
				if (right.visibleArea !== left.visibleArea) return right.visibleArea - left.visibleArea;
				return left.distance - right.distance;
			});
			const result = [];
			for (const entry of entries) {
				if (!result.includes(entry.id)) result.push(entry.id);
					if (result.length >= 8) break;
				}
				return result;
			})();
			const visibleLiveIDs = (() => {
				const entries = [];
				const viewportCenterX = Math.max(0, window.innerWidth || 0) / 2;
				const viewportCenterY = Math.max(0, window.innerHeight || 0) / 2;
				for (const link of document.querySelectorAll("a[href]")) {
					const id = liveIDFromURL(link.href || "");
					if (!id) continue;
					const rect = link.getBoundingClientRect();
					const visibleWidth = Math.max(0, Math.min(rect.right, window.innerWidth) - Math.max(rect.left, 0));
					const visibleHeight = Math.max(0, Math.min(rect.bottom, window.innerHeight) - Math.max(rect.top, 0));
					const visibleArea = Math.round(visibleWidth * visibleHeight);
					if (visibleArea <= 0) continue;
					if (!nodeIsRenderable(link)) continue;
					const centerX = rect.left + rect.width / 2;
					const centerY = rect.top + rect.height / 2;
					const distance = Math.hypot(centerX - viewportCenterX, centerY - viewportCenterY);
					entries.push({ id, visibleArea, distance });
				}
				entries.sort((left, right) => {
					if (right.visibleArea !== left.visibleArea) return right.visibleArea - left.visibleArea;
					return left.distance - right.distance;
				});
				const result = [];
				for (const entry of entries) {
					if (!result.includes(entry.id)) result.push(entry.id);
					if (result.length >= 8) break;
				}
				return result;
			})();
			return {
				location: (window.location.href || "").slice(0, 500),
			title: clean(document.title || ""),
			ogTitle: meta('meta[property="og:title"]'),
			description: meta('meta[name="description"]') || meta('meta[property="og:description"]'),
			image: meta('meta[property="og:image"]'),
			thumbnail: meta('meta[name="thumbnail"]') || meta('meta[name="twitter:image"]'),
				awemeID: currentAwemeID,
				visibleAwemeID: visibleAwemeIDs[0] || "",
				visibleAwemeIDs: JSON.stringify(visibleAwemeIDs),
				visibleLiveID: visibleLiveIDs[0] || "",
				visibleLiveIDs: JSON.stringify(visibleLiveIDs),
				apiVideoURL: "",
			apiTitle: "",
			apiAuthor: "",
			apiImage: "",
			apiSizeBytes: "",
			author: "",
			videoTitle: "",
			jsonTitle: "",
			jsonAuthor: "",
			jsonImage: "",
			videoCount: String(document.querySelectorAll("video").length),
			videoCurrentSrc: currentAwemeID ? "" : primaryVideo.currentSrc || "",
			videoSrc: currentAwemeID ? "" : primaryVideo.src || "",
			videoItems: JSON.stringify(topVideos),
			videoWidth: currentAwemeID ? "" : (primaryVideo.width ? String(primaryVideo.width) : ""),
			videoHeight: currentAwemeID ? "" : (primaryVideo.height ? String(primaryVideo.height) : ""),
			loginHint: ""
		};
	})()`
}

func scoreResourceCandidate(rawURL string, mimeType string, headers network.Headers, resourceType network.ResourceType) int {
	score, _ := scoreResourceCandidateWithReason(rawURL, mimeType, headers, resourceType)
	return score
}

func scoreResourceCandidateWithReason(rawURL string, mimeType string, headers network.Headers, resourceType network.ResourceType) (int, string) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return 0, "empty_url"
	}
	lowerURL := strings.ToLower(trimmedURL)
	lowerMime := strings.ToLower(strings.TrimSpace(mimeType))
	contentType := strings.ToLower(headerValue(headers, "content-type"))
	sizeBytes := contentLengthFromHeaders(headers)
	if sizeBytes > 0 && sizeBytes < resourceMinCandidateBytes {
		return 0, "too_small"
	}
	if resourceSniffRawMediaSegment(trimmedURL, lowerMime, contentType, string(resourceType)) {
		return 0, "segment"
	}
	if resourceSniffRawFLVStream(trimmedURL, lowerMime, contentType) && sizeBytes <= 0 {
		return 0, "live_stream"
	}
	hasVideoMIME := strings.HasPrefix(lowerMime, "video/") || strings.HasPrefix(contentType, "video/")
	hasVideoURL := strings.Contains(lowerURL, ".mp4") ||
		strings.Contains(lowerURL, "mime_type=video") ||
		strings.Contains(lowerURL, "video/tos") ||
		strings.Contains(lowerURL, "douyinvod") ||
		strings.Contains(lowerURL, "sns-video")
	isMediaResource := strings.EqualFold(string(resourceType), "Media")
	switch {
	case strings.HasPrefix(lowerMime, "image/"), strings.HasPrefix(contentType, "image/"),
		strings.HasPrefix(lowerMime, "audio/"), strings.HasPrefix(contentType, "audio/"):
		return 0, "not_video"
	case strings.HasPrefix(lowerMime, "text/"), strings.HasPrefix(contentType, "text/"),
		strings.Contains(lowerMime, "json"), strings.Contains(contentType, "json"),
		strings.Contains(lowerMime, "javascript"), strings.Contains(contentType, "javascript"):
		if !hasVideoMIME && !hasVideoURL && !isMediaResource {
			return 0, "not_video"
		}
	}
	if !hasVideoMIME && !hasVideoURL && !isMediaResource {
		return 0, "no_video_signal"
	}

	score := 0
	if hasVideoMIME {
		score += 80
	}
	if isMediaResource {
		score += 35
	}
	if hasVideoURL {
		score += 30
	}
	if strings.Contains(lowerURL, "douyinvod") || strings.Contains(lowerURL, "bytecdn") || strings.Contains(lowerURL, "bytedance") || strings.Contains(lowerURL, "snssdk") || strings.Contains(lowerURL, "amemv") ||
		strings.Contains(lowerURL, "sns-video") || strings.Contains(lowerURL, "xhscdn") {
		score += 20
	}
	if sizeBytes >= resourceMinCandidateBytes {
		score += 20
	}
	if strings.Contains(lowerURL, ".m3u8") || strings.Contains(lowerMime, "mpegurl") || strings.Contains(contentType, "mpegurl") {
		score -= 80
	}
	if score <= 0 {
		return score, "low_score"
	}
	return score, ""
}

func parsePageMetaInt(pageMeta map[string]string, key string) int {
	if len(pageMeta) == 0 {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(pageMeta[key]))
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func parsePageMetaInt64(pageMeta map[string]string, key string) int64 {
	if len(pageMeta) == 0 {
		return 0
	}
	value, err := strconv.ParseInt(strings.TrimSpace(pageMeta[key]), 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func isLikelyResourceMediaResponse(rawURL string, mimeType string, contentType string, resourceType network.ResourceType) bool {
	lowerURL := strings.ToLower(strings.TrimSpace(rawURL))
	lowerMime := strings.ToLower(strings.TrimSpace(mimeType))
	lowerContentType := strings.ToLower(strings.TrimSpace(contentType))
	if strings.EqualFold(string(resourceType), "Media") {
		return true
	}
	if strings.HasPrefix(lowerMime, "video/") || strings.HasPrefix(lowerContentType, "video/") {
		return true
	}
	if strings.HasPrefix(lowerMime, "audio/") || strings.HasPrefix(lowerContentType, "audio/") {
		return true
	}
	if strings.Contains(lowerURL, ".mp4") || strings.Contains(lowerURL, ".m3u8") || strings.Contains(lowerURL, "mime_type=video") || strings.Contains(lowerURL, "video/tos") ||
		strings.Contains(lowerURL, "sns-video") {
		return true
	}
	return strings.Contains(lowerURL, "douyinvod") ||
		strings.Contains(lowerURL, "bytecdn") ||
		strings.Contains(lowerURL, "bytedance") ||
		strings.Contains(lowerURL, "snssdk") ||
		strings.Contains(lowerURL, "amemv") ||
		strings.Contains(lowerURL, "xhscdn")
}

func resourceDouyinLooksBlocked(pageMeta map[string]string, rejected []resourceRejectedCandidate) bool {
	_, _, blocked := resourceDouyinBlockReason(pageMeta, rejected)
	return blocked
}

func resourceDouyinBlockReason(pageMeta map[string]string, rejected []resourceRejectedCandidate) (string, string, bool) {
	title := strings.ToLower(strings.TrimSpace(pageMeta["title"]))
	location := strings.ToLower(strings.TrimSpace(pageMeta["location"]))
	if strings.Contains(title, "验证码") {
		return "page_title_contains_verification", pageMeta["title"], true
	}
	if strings.Contains(title, "captcha") {
		return "page_title_contains_captcha", pageMeta["title"], true
	}
	if strings.Contains(location, "captcha") {
		return "page_location_contains_captcha", pageMeta["location"], true
	}
	if strings.Contains(location, "verify") {
		return "page_location_contains_verify", pageMeta["location"], true
	}
	return "", "", false
}

func resourceDouyinRejectedVerificationSignal(candidate resourceRejectedCandidate) string {
	lowerURL := strings.ToLower(strings.TrimSpace(candidate.url))
	if lowerURL == "" {
		return ""
	}
	if strings.Contains(lowerURL, "captcha") {
		return "rejected_url_contains_captcha"
	}
	if strings.Contains(lowerURL, "verifycenter") {
		return "rejected_url_contains_verifycenter"
	}
	if strings.Contains(lowerURL, "verify.snssdk.com") {
		return "rejected_url_contains_verify_snssdk"
	}
	if strings.Contains(lowerURL, "zijieapi.com") {
		return "rejected_url_contains_zijieapi"
	}
	return ""
}

func resourceHasHeader(headers map[string]string, key string) bool {
	_, ok := findHeader(headers, key)
	return ok
}

func headersToStringMap(headers network.Headers) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, len(headers))
	for key, raw := range headers {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" {
			continue
		}
		result[trimmedKey] = value
	}
	return result
}

func mergeHeaders(left map[string]string, right map[string]string) map[string]string {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	result := cloneStringMap(left)
	if result == nil {
		result = map[string]string{}
	}
	for key, value := range right {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func headerValue(headers network.Headers, key string) string {
	if len(headers) == 0 {
		return ""
	}
	for headerKey, raw := range headers {
		if strings.EqualFold(strings.TrimSpace(headerKey), strings.TrimSpace(key)) {
			return strings.TrimSpace(fmt.Sprint(raw))
		}
	}
	return ""
}

func contentLengthFromHeaders(headers network.Headers) int64 {
	if total := parseContentRangeTotal(headerValue(headers, "content-range")); total > 0 {
		return total
	}
	raw := headerValue(headers, "content-length")
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func resourceExtension(rawURL string, contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "video/x-flv":
		return ".flv"
	case "application/vnd.apple.mpegurl", "application/x-mpegurl":
		return ".m3u8"
	}
	if parsed, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
		ext := strings.ToLower(strings.TrimSpace(filepath.Ext(parsed.Path)))
		switch ext {
		case ".mp4", ".webm", ".mov", ".m4v", ".flv", ".m3u8":
			return ext
		}
	}
	return ""
}

func normalizeResourceDownloadHeaders(headers map[string]string, rawURL string) map[string]string {
	result := map[string]string{}
	for key, value := range headers {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		lowerKey := strings.ToLower(trimmedKey)
		if _, forbidden := resourceForbiddenDownloadHeaders[lowerKey]; forbidden {
			continue
		}
		result[trimmedKey] = trimmedValue
	}
	if _, ok := findHeader(result, "User-Agent"); !ok {
		result["User-Agent"] = resourceDefaultUserAgent
	}
	if _, ok := findHeader(result, "Accept"); !ok {
		result["Accept"] = resourceDefaultAccept
	}
	if _, ok := findHeader(result, "Accept-Language"); !ok {
		result["Accept-Language"] = resourceDefaultAcceptLanguage
	}
	if _, ok := findHeader(result, "Referer"); !ok {
		if parsed, err := url.Parse(strings.TrimSpace(rawURL)); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			result["Referer"] = parsed.Scheme + "://" + parsed.Host + "/"
		}
	}
	return result
}

func findHeader(headers map[string]string, key string) (string, bool) {
	for headerKey, value := range headers {
		if strings.EqualFold(headerKey, key) {
			return value, true
		}
	}
	return "", false
}
