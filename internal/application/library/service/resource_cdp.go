package service

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"

	"xiadown/internal/application/sniffprofile"
)

const (
	resourceMinCandidateBytes            = int64(256 * 1024)
	resourceMaxPreviewImageBodyBytes     = 4 * 1024 * 1024
	resourceSniffNetworkTotalBufferBytes = int64(120 * 1024 * 1024)
	resourceSniffNetworkResourceBytes    = int64(24 * 1024 * 1024)
	resourceSniffRequestLateExtraGrace   = 10 * time.Second
	resourceSniffRequestMaxChains        = 4096
)

type resourceRequestCaptureID uint64

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
	captureID    resourceRequestCaptureID
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
	captureID    resourceRequestCaptureID
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
	mu                  sync.Mutex
	requests            map[network.RequestID]resourceRequest
	requestChains       map[network.RequestID]*resourceRequestExtraInfoBuilder
	nextCaptureID       resourceRequestCaptureID
	requestCleanupTimer *time.Timer
	observed            []resourceObservedResource
	candidates          []resourceCandidate
	rejected            []resourceRejectedCandidate
	previews            []resourcePreviewSnapshot
	subtitles           []resourceSubtitle
}

type resourceRequest struct {
	captureID       resourceRequestCaptureID
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
	captureID    resourceRequestCaptureID
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

// resourceRequestExtraInfoBuilder mirrors the ordering model used by Chrome
// DevTools. Redirect hops reuse a CDP RequestID, while request extra-info can
// arrive before or after requestWillBeSent. Slice position, rather than event
// arrival time, is therefore the only safe way to associate on-wire headers
// with a redirect stage.
type resourceRequestExtraInfoBuilder struct {
	requests              []*resourceRequestStage
	responseExtraInfoFlag []bool
	requestExtraInfos     []*resourceRequestExtraInfo
	createdAt             time.Time
	finishedAt            time.Time
}

type resourceRequestStage struct {
	request      resourceRequest
	materialized bool
}

type resourceRequestExtraInfo struct {
	headers map[string]string
}

func newResourceCaptureState() *resourceCaptureState {
	return &resourceCaptureState{
		requests:      make(map[network.RequestID]resourceRequest),
		requestChains: make(map[network.RequestID]*resourceRequestExtraInfoBuilder),
	}
}

func (state *resourceCaptureState) clear() {
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.requestCleanupTimer != nil {
		state.requestCleanupTimer.Stop()
		state.requestCleanupTimer = nil
	}
	state.requests = make(map[network.RequestID]resourceRequest)
	state.requestChains = make(map[network.RequestID]*resourceRequestExtraInfoBuilder)
	state.observed = nil
	state.candidates = nil
	state.rejected = nil
	state.previews = nil
	state.subtitles = nil
}

func (state *resourceCaptureState) recordRequest(id network.RequestID, requestURL string, documentURL string, headers network.Headers) {
	if state == nil || strings.TrimSpace(requestURL) == "" {
		return
	}
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.pruneRequestChainsLocked(now)
	builder := state.requestBuilderLocked(id, now)
	state.nextCaptureID++
	stage := &resourceRequestStage{request: resourceRequest{
		captureID:   state.nextCaptureID,
		url:         strings.TrimSpace(requestURL),
		documentURL: strings.TrimSpace(documentURL),
		headers:     headersToStringMap(headers),
	}}
	builder.requests = append(builder.requests, stage)
	state.requests[id] = stage.request
	state.syncRequestExtraInfoLocked(id, builder, len(builder.requests)-1)
}

func (state *resourceCaptureState) recordRequestHeaders(id network.RequestID, headers network.Headers) {
	if state == nil {
		return
	}
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.pruneRequestChainsLocked(now)
	builder := state.requestBuilderLocked(id, now)
	builder.requestExtraInfos = append(builder.requestExtraInfos, &resourceRequestExtraInfo{
		headers: headersToStringMap(headers),
	})
	state.syncRequestExtraInfoLocked(id, builder, len(builder.requestExtraInfos)-1)
}

func (state *resourceCaptureState) recordResponse(id network.RequestID, responseURL string, status int64, mimeType string, headers network.Headers, resourceType network.ResourceType, extraInfo ...bool) {
	if state == nil {
		return
	}
	// Internal callers predating the CDP flag passed no final argument and had
	// already supplied request extra-info directly. Preserve that behavior;
	// live CDP events always pass the authoritative flag explicitly.
	hasExtraInfo := len(extraInfo) == 0 || extraInfo[0]
	contentType := strings.TrimSpace(headerValue(headers, "content-type"))
	sizeBytes := contentLengthFromHeaders(headers)
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.pruneRequestChainsLocked(now)
	builder := state.requestBuilderLocked(id, now)
	stageIndex := len(builder.responseExtraInfoFlag)
	if !hasExtraInfo {
		builder.requestExtraInfos = insertResourceRequestExtraInfo(builder.requestExtraInfos, stageIndex, nil)
	}
	builder.responseExtraInfoFlag = append(builder.responseExtraInfoFlag, hasExtraInfo)
	for len(builder.requests) <= stageIndex {
		state.nextCaptureID++
		builder.requests = append(builder.requests, &resourceRequestStage{request: resourceRequest{captureID: state.nextCaptureID}})
	}
	stage := builder.requests[stageIndex]
	request := stage.request
	if strings.TrimSpace(responseURL) == "" {
		responseURL = request.url
	}
	request.responseURL = strings.TrimSpace(responseURL)
	request.mimeType = strings.TrimSpace(mimeType)
	request.contentType = contentType
	request.status = status
	request.sizeBytes = sizeBytes
	request.responseHeaders = headersToStringMap(headers)
	request.resourceType = resourceType
	request.seenAt = now
	stage.request = request
	state.requests[id] = request
	state.materializeResourceResponseLocked(stage, headers)
	state.syncRequestExtraInfoLocked(id, builder, stageIndex)
}

func (state *resourceCaptureState) materializeResourceResponseLocked(stage *resourceRequestStage, responseHeaders network.Headers) {
	if state == nil || stage == nil || stage.materialized {
		return
	}
	request := stage.request
	responseURL := firstNonEmpty(strings.TrimSpace(request.responseURL), strings.TrimSpace(request.url))
	requestHeaders := cloneStringMap(request.headers)
	state.recordObservedLocked(resourceObservedResource{
		captureID:    request.captureID,
		url:          responseURL,
		pageURL:      strings.TrimSpace(request.documentURL),
		mimeType:     request.mimeType,
		contentType:  request.contentType,
		resourceType: string(request.resourceType),
		status:       request.status,
		sizeBytes:    request.sizeBytes,
		headers:      requestHeaders,
		seenAt:       request.seenAt,
	})
	stage.materialized = true
	if request.status < 200 || request.status >= 300 || request.status == 204 {
		if isLikelyResourceMediaResponse(responseURL, request.mimeType, request.contentType, request.resourceType) {
			state.recordRejectedLocked(resourceRejectedCandidate{
				captureID: request.captureID,
				url:       responseURL, mimeType: request.mimeType, contentType: request.contentType,
				resourceType: string(request.resourceType), status: request.status, sizeBytes: request.sizeBytes,
				reason: fmt.Sprintf("http_status_%d", request.status), headers: requestHeaders, seenAt: request.seenAt,
			})
		}
		return
	}
	if subtitle, ok := resourceSubtitleFromResponse(responseURL, request.documentURL, request.mimeType, request.contentType, string(request.resourceType), requestHeaders, request.seenAt); ok {
		subtitle.captureID = request.captureID
		state.recordSubtitleLocked(subtitle)
	}

	score, rejectReason := scoreResourceCandidateWithReason(responseURL, request.mimeType, responseHeaders, request.resourceType)
	if score <= 0 {
		if rejectReason != "" || isLikelyResourceMediaResponse(responseURL, request.mimeType, request.contentType, request.resourceType) {
			state.recordRejectedLocked(resourceRejectedCandidate{
				captureID: request.captureID,
				url:       responseURL, mimeType: request.mimeType, contentType: request.contentType,
				resourceType: string(request.resourceType), status: request.status, sizeBytes: request.sizeBytes,
				score: score, reason: rejectReason, headers: requestHeaders, seenAt: request.seenAt,
			})
		}
		return
	}
	state.candidates = append(state.candidates, resourceCandidate{
		captureID:    request.captureID,
		url:          responseURL,
		pageURL:      strings.TrimSpace(request.documentURL),
		mimeType:     request.mimeType,
		contentType:  request.contentType,
		resourceType: string(request.resourceType),
		status:       request.status,
		sizeBytes:    request.sizeBytes,
		headers:      requestHeaders,
		score:        score,
		seenAt:       request.seenAt,
	})
}

func insertResourceRequestExtraInfo(values []*resourceRequestExtraInfo, index int, value *resourceRequestExtraInfo) []*resourceRequestExtraInfo {
	if index < 0 {
		return values
	}
	for len(values) < index {
		values = append(values, nil)
	}
	if index == len(values) {
		return append(values, value)
	}
	values = append(values, nil)
	copy(values[index+1:], values[index:])
	values[index] = value
	return values
}

func (state *resourceCaptureState) requestBuilderLocked(id network.RequestID, now time.Time) *resourceRequestExtraInfoBuilder {
	if state.requestChains == nil {
		state.requestChains = make(map[network.RequestID]*resourceRequestExtraInfoBuilder)
	}
	builder := state.requestChains[id]
	if builder == nil {
		builder = &resourceRequestExtraInfoBuilder{createdAt: now}
		state.requestChains[id] = builder
	}
	return builder
}

func (state *resourceCaptureState) syncRequestExtraInfoLocked(id network.RequestID, builder *resourceRequestExtraInfoBuilder, index int) {
	if state == nil || builder == nil || index < 0 || index >= len(builder.requests) || index >= len(builder.responseExtraInfoFlag) {
		return
	}
	// A false response extra-info flag occupies this redirect stage. The nil
	// placeholder prevents a request extra-info event for the next hop from
	// being applied to this one.
	if !builder.responseExtraInfoFlag[index] || index >= len(builder.requestExtraInfos) || builder.requestExtraInfos[index] == nil {
		return
	}
	stage := builder.requests[index]
	if stage == nil {
		return
	}
	stage.request.headers = mergeHeaders(stage.request.headers, builder.requestExtraInfos[index].headers)
	if index == len(builder.requests)-1 {
		state.requests[id] = stage.request
	}
	if stage.materialized {
		state.patchCapturedHeadersLocked(stage.request.captureID, stage.request.headers)
	}
}

func (state *resourceCaptureState) patchCapturedHeadersLocked(captureID resourceRequestCaptureID, headers map[string]string) {
	if state == nil || captureID == 0 {
		return
	}
	for index := range state.observed {
		if state.observed[index].captureID == captureID {
			state.observed[index].headers = cloneStringMap(headers)
		}
	}
	for index := range state.candidates {
		if state.candidates[index].captureID == captureID {
			state.candidates[index].headers = cloneStringMap(headers)
		}
	}
	for index := range state.rejected {
		if state.rejected[index].captureID == captureID {
			state.rejected[index].headers = cloneStringMap(headers)
		}
	}
	for index := range state.subtitles {
		if state.subtitles[index].captureID == captureID {
			state.subtitles[index].RequestHeaders = normalizeResourceDownloadHeaders(
				headers,
				firstNonEmpty(state.subtitles[index].PageURL, state.subtitles[index].URL),
			)
		}
	}
}

func (state *resourceCaptureState) markRequestFinished(id network.RequestID) {
	if state == nil {
		return
	}
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.pruneRequestChainsLocked(now)
	if builder := state.requestChains[id]; builder != nil {
		builder.finishedAt = now
		state.scheduleRequestCleanupLocked()
	}
}

func (state *resourceCaptureState) pruneRequestChainsLocked(now time.Time) {
	if state == nil || len(state.requestChains) == 0 {
		return
	}
	for id, builder := range state.requestChains {
		if builder == nil ||
			(!builder.finishedAt.IsZero() && now.Sub(builder.finishedAt) >= resourceSniffRequestLateExtraGrace) ||
			(len(builder.requests) == 0 && now.Sub(builder.createdAt) >= resourceSniffRequestLateExtraGrace) {
			delete(state.requestChains, id)
			delete(state.requests, id)
		}
	}
	if len(state.requestChains) <= resourceSniffRequestMaxChains {
		return
	}
	type requestChainAge struct {
		id network.RequestID
		at time.Time
	}
	ages := make([]requestChainAge, 0, len(state.requestChains))
	for id, builder := range state.requestChains {
		at := builder.createdAt
		if !builder.finishedAt.IsZero() {
			at = builder.finishedAt
		}
		ages = append(ages, requestChainAge{id: id, at: at})
	}
	sort.Slice(ages, func(left int, right int) bool { return ages[left].at.Before(ages[right].at) })
	for _, item := range ages[:len(ages)-resourceSniffRequestMaxChains] {
		delete(state.requestChains, item.id)
		delete(state.requests, item.id)
	}
}

func (state *resourceCaptureState) scheduleRequestCleanupLocked() {
	if state == nil || state.requestCleanupTimer != nil {
		return
	}
	state.requestCleanupTimer = time.AfterFunc(resourceSniffRequestLateExtraGrace, state.cleanupFinishedRequestChains)
}

func (state *resourceCaptureState) cleanupFinishedRequestChains() {
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.requestCleanupTimer = nil
	state.pruneRequestChainsLocked(time.Now())
	for _, builder := range state.requestChains {
		if builder != nil && (!builder.finishedAt.IsZero() || len(builder.requests) == 0) {
			state.scheduleRequestCleanupLocked()
			break
		}
	}
}

func (state *resourceCaptureState) recordObserved(resource resourceObservedResource) {
	if state == nil || strings.TrimSpace(resource.url) == "" {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.recordObservedLocked(resource)
}

func (state *resourceCaptureState) recordObservedLocked(resource resourceObservedResource) {
	resource.url = strings.TrimSpace(resource.url)
	if resource.url == "" {
		return
	}
	resource.pageURL = strings.TrimSpace(resource.pageURL)
	resource.mimeType = strings.TrimSpace(resource.mimeType)
	resource.contentType = strings.TrimSpace(resource.contentType)
	resource.resourceType = strings.TrimSpace(resource.resourceType)
	resource.headers = cloneStringMap(resource.headers)
	if resource.seenAt.IsZero() {
		resource.seenAt = time.Now()
	}
	for index, item := range state.observed {
		if resourceComparableURL(item.url, false) == resourceComparableURL(resource.url, false) &&
			resourceComparableURL(item.pageURL, false) == resourceComparableURL(resource.pageURL, false) &&
			strings.TrimSpace(item.resourceType) == strings.TrimSpace(resource.resourceType) {
			// Re-observing a signed URL must refresh its request headers. Keeping
			// the first Cookie here makes later downloads use an expired session.
			if resource.captureID > item.captureID ||
				(resource.captureID == item.captureID && resource.seenAt.After(item.seenAt)) {
				state.observed[index] = resource
			}
			return
		}
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
	return shouldCaptureResourcePreviewSnapshot(request)
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
	if state == nil || len(data) == 0 || len(data) > resourceMaxPreviewImageBodyBytes {
		return
	}
	state.mu.Lock()
	request := state.requests[id]
	state.mu.Unlock()
	if preview, ok := resourcePreviewSnapshotFromResponse(request, data); ok {
		state.recordPreviewSnapshot(preview)
	}
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
	state.recordSubtitleLocked(subtitle)
}

func (state *resourceCaptureState) recordSubtitleLocked(subtitle resourceSubtitle) {
	if !subtitle.Valid() {
		return
	}
	key := resourceSubtitleKey(subtitle)
	for index, item := range state.subtitles {
		if key != "" && resourceSubtitleKey(item) == key {
			if subtitle.captureID > item.captureID ||
				(subtitle.captureID == item.captureID && subtitle.SeenAt.After(item.SeenAt)) {
				state.subtitles[index] = subtitle
			}
			return
		}
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
	defer state.mu.Unlock()
	state.recordRejectedLocked(rejected)
}

func (state *resourceCaptureState) recordRejectedLocked(rejected resourceRejectedCandidate) {
	rejected.url = strings.TrimSpace(rejected.url)
	rejected.mimeType = strings.TrimSpace(rejected.mimeType)
	rejected.contentType = strings.TrimSpace(rejected.contentType)
	rejected.resourceType = strings.TrimSpace(rejected.resourceType)
	rejected.reason = strings.TrimSpace(rejected.reason)
	rejected.headers = cloneStringMap(rejected.headers)
	if rejected.seenAt.IsZero() {
		rejected.seenAt = time.Now()
	}
	state.rejected = append(state.rejected, rejected)
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
	// Browser choice is supplied with each StartResourceSniff request. Keep the
	// helper for compatibility with tests and old internal callers, but never
	// read the persisted global SniffBrowser setting.
	return ""
}

func (service *LibraryService) resourceConnectorProfilePath(ctx context.Context, rawURL string) (string, error) {
	if service == nil {
		return "", nil
	}
	return sniffprofile.PathForPreferredBrowser("")
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

func resourcePageIdentityScript() string {
	return `(() => ({
		location: String(window.location.href || "").slice(0, 500),
		title: String(document.title || "").replace(/\s+/g, " ").trim(),
		readyState: String(document.readyState || ""),
		visibilityState: String(document.visibilityState || ""),
		hasFocus: Boolean(document.hasFocus && document.hasFocus())
	}))()`
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
	result := make(map[string]string, len(left)+len(right))
	for _, source := range []map[string]string{left, right} {
		for key, value := range source {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				continue
			}
			for existingKey := range result {
				if strings.EqualFold(existingKey, key) {
					delete(result, existingKey)
				}
			}
			result[key] = value
		}
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
