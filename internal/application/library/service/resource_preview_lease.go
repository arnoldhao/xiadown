package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/library/dto"
	appytdlp "xiadown/internal/application/ytdlp"
)

const resourceSniffPreviewLeaseTTL = 15 * time.Minute
const resourceSniffPreviewProxyURLQueryParam = "url"
const resourceSniffPreviewManifestQueryParam = "manifest_query"

var (
	resourceSniffHLSDoubleQuotedURIPattern = regexp.MustCompile(`URI="([^"]+)"`)
	resourceSniffHLSSingleQuotedURIPattern = regexp.MustCompile(`URI='([^']+)'`)
)

type resourceSniffPreviewLease struct {
	ID              string
	SessionID       string
	ResourceID      string
	URL             string
	PageURL         string
	Kind            string
	MimeType        string
	ContentType     string
	FileName        string
	SizeBytes       int64
	Headers         map[string]string
	HLSKeyOverrides map[string]resourceSniffHLSKeyOverride
	ExpiresAt       time.Time
}

type resourceSniffHLSKeyOverride struct {
	URL         string
	Key         []byte
	Source      string
	Rule        string
	NonStandard bool
}

func (service *LibraryService) PrepareResourceSniffRawPreview(
	ctx context.Context,
	request dto.PrepareResourceSniffRawPreviewRequest,
) (dto.PrepareResourceSniffRawPreviewResponse, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	resourceID := strings.TrimSpace(request.ResourceID)
	if sessionID == "" || resourceID == "" {
		return dto.PrepareResourceSniffRawPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff preview resource is required")
	}
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok {
		return dto.PrepareResourceSniffRawPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	var selected resourceSniffRawResource
	for _, resource := range service.listResourceSniffRawResources(session) {
		if resource.ID == resourceID {
			selected = resource
			break
		}
	}
	if selected.ID == "" {
		return dto.PrepareResourceSniffRawPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff raw resource not found")
	}
	previewKind := resourceSniffPreviewLeaseKind(selected.ResourceSniffRawResource)
	if previewKind == "" {
		return dto.PrepareResourceSniffRawPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff raw resource is not previewable")
	}
	if strings.TrimSpace(selected.URL) == "" {
		return dto.PrepareResourceSniffRawPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff raw resource url is required")
	}
	if !resourceSniffRawFetchableURL(selected.URL) {
		return dto.PrepareResourceSniffRawPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff raw resource url is not fetchable")
	}

	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	service.pruneResourceSniffPreviewLeasesLocked(service.now())
	if service.resourcePreviewLeases == nil {
		service.resourcePreviewLeases = make(map[string]resourceSniffPreviewLease)
	}
	expiresAt := service.now().Add(resourceSniffPreviewLeaseTTL)
	lease := resourceSniffPreviewLease{
		ID:          uuid.NewString(),
		SessionID:   sessionID,
		ResourceID:  resourceID,
		URL:         strings.TrimSpace(selected.URL),
		PageURL:     strings.TrimSpace(selected.PageURL),
		Kind:        previewKind,
		MimeType:    strings.TrimSpace(selected.MimeType),
		ContentType: strings.TrimSpace(selected.ContentType),
		FileName:    resourceSniffPreviewFileName(selected.ResourceSniffRawResource),
		SizeBytes:   selected.SizeBytes,
		Headers:     cloneStringMap(selected.headers),
		ExpiresAt:   expiresAt,
	}
	service.resourcePreviewLeases[lease.ID] = lease
	return dto.PrepareResourceSniffRawPreviewResponse{
		ResourceID: lease.ResourceID,
		LeaseID:    lease.ID,
		Kind:       lease.Kind,
		MimeType:   firstNonEmpty(lease.MimeType, lease.ContentType),
		FileName:   lease.FileName,
		SizeBytes:  lease.SizeBytes,
		ExpiresAt:  expiresAt.Format(time.RFC3339),
	}, nil
}

func (service *LibraryService) ServeResourceSniffPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	leaseID := resourceSniffPreviewLeaseIDFromPath(r.URL.Path)
	if leaseID == "" {
		http.Error(w, "preview lease is required", http.StatusBadRequest)
		return
	}
	lease, ok := service.resourceSniffPreviewLease(leaseID)
	if !ok {
		http.Error(w, "preview lease expired", http.StatusNotFound)
		return
	}

	targetURL, err := resourceSniffPreviewTargetURL(lease, r)
	if err != nil {
		http.Error(w, "invalid preview target url", http.StatusBadRequest)
		return
	}
	if override, ok := service.resourceSniffPreviewHLSKeyOverride(lease.ID, targetURL); ok {
		serveResourceSniffPreviewHLSKeyOverride(w, r, override)
		return
	}

	resp, effectiveURL, err := service.fetchResourceSniffPreviewTarget(r.Context(), r.Method, targetURL, r, lease)
	if err != nil {
		http.Error(w, "failed to fetch preview resource", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if r.Method != http.MethodHead && resourceSniffPreviewShouldRewriteHLS(lease, effectiveURL, resp.Header) {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "failed to read preview manifest", http.StatusBadGateway)
			return
		}
		service.prepareResourceSniffPreviewHLSKeyOverride(r.Context(), lease, effectiveURL, resp.Header.Get("Content-Type"), body)
		rewritten := []byte(rewriteResourceSniffHLSManifest(string(body), effectiveURL, r, lease))
		copyResourceSniffPreviewHeaders(w.Header(), resp.Header, lease)
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Length", strconv.Itoa(len(rewritten)))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(rewritten)
		return
	}

	copyResourceSniffPreviewHeaders(w.Header(), resp.Header, lease)
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, resp.Body)
}

func (service *LibraryService) fetchResourceSniffPreviewTarget(ctx context.Context, method string, targetURL string, r *http.Request, lease resourceSniffPreviewLease) (*http.Response, string, error) {
	resp, err := service.fetchResourceSniffPreviewTargetOnce(ctx, method, targetURL, r, lease)
	if err == nil && !resourceSniffPreviewShouldRetryWithManifestQuery(resp.StatusCode) {
		return resp, targetURL, nil
	}
	if retryURL := resourceSniffPreviewFallbackTargetURL(targetURL, r); retryURL != "" && retryURL != strings.TrimSpace(targetURL) {
		retryResp, retryErr := service.fetchResourceSniffPreviewTargetOnce(ctx, method, retryURL, r, lease)
		if retryErr == nil {
			if resp != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
			return retryResp, retryURL, nil
		}
		if err != nil {
			return nil, "", err
		}
	}
	if err != nil {
		return nil, "", err
	}
	return resp, targetURL, nil
}

func (service *LibraryService) fetchResourceSniffPreviewTargetOnce(ctx context.Context, method string, targetURL string, r *http.Request, lease resourceSniffPreviewLease) (*http.Response, error) {
	outgoing, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
	if err != nil {
		return nil, err
	}
	applyResourceRequestHeaders(outgoing, lease.Headers)
	if r != nil {
		if rangeHeader := strings.TrimSpace(r.Header.Get("Range")); rangeHeader != "" {
			outgoing.Header.Set("Range", rangeHeader)
		}
	}
	if _, ok := findHeader(httpHeaderToStringMap(outgoing.Header), "Accept"); !ok {
		outgoing.Header.Set("Accept", resourceSniffPreviewAcceptHeader(lease.Kind))
	}
	return resourceSniffPreviewHTTPClient(service.ytdlpAuxiliaryHTTPClient()).Do(outgoing)
}

func (service *LibraryService) resourceSniffPreviewLease(leaseID string) (resourceSniffPreviewLease, bool) {
	if service == nil {
		return resourceSniffPreviewLease{}, false
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	now := service.now()
	service.pruneResourceSniffPreviewLeasesLocked(now)
	lease, ok := service.resourcePreviewLeases[strings.TrimSpace(leaseID)]
	if !ok || (!lease.ExpiresAt.IsZero() && !lease.ExpiresAt.After(now)) {
		return resourceSniffPreviewLease{}, false
	}
	lease.Headers = cloneStringMap(lease.Headers)
	lease.HLSKeyOverrides = cloneResourceSniffHLSKeyOverrides(lease.HLSKeyOverrides)
	return lease, true
}

func (service *LibraryService) pruneResourceSniffPreviewLeasesLocked(now time.Time) {
	if service == nil || len(service.resourcePreviewLeases) == 0 {
		return
	}
	for id, lease := range service.resourcePreviewLeases {
		if lease.ExpiresAt.IsZero() || lease.ExpiresAt.After(now) {
			continue
		}
		delete(service.resourcePreviewLeases, id)
	}
}

func cloneResourceSniffHLSKeyOverrides(overrides map[string]resourceSniffHLSKeyOverride) map[string]resourceSniffHLSKeyOverride {
	if len(overrides) == 0 {
		return nil
	}
	result := make(map[string]resourceSniffHLSKeyOverride, len(overrides))
	for key, override := range overrides {
		cloned := override
		cloned.Key = append([]byte(nil), override.Key...)
		result[key] = cloned
	}
	return result
}

func (service *LibraryService) resourceSniffPreviewHLSKeyOverride(leaseID string, targetURL string) (resourceSniffHLSKeyOverride, bool) {
	if service == nil {
		return resourceSniffHLSKeyOverride{}, false
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	now := service.now()
	service.pruneResourceSniffPreviewLeasesLocked(now)
	lease, ok := service.resourcePreviewLeases[strings.TrimSpace(leaseID)]
	if !ok || len(lease.HLSKeyOverrides) == 0 {
		return resourceSniffHLSKeyOverride{}, false
	}
	override, ok := lease.HLSKeyOverrides[strings.TrimSpace(targetURL)]
	if !ok || len(override.Key) == 0 {
		return resourceSniffHLSKeyOverride{}, false
	}
	override.Key = append([]byte(nil), override.Key...)
	return override, true
}

func (service *LibraryService) setResourceSniffPreviewHLSKeyOverride(leaseID string, override resourceSniffHLSKeyOverride) {
	targetURL := strings.TrimSpace(override.URL)
	if service == nil || strings.TrimSpace(leaseID) == "" || targetURL == "" || len(override.Key) == 0 {
		return
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	now := service.now()
	service.pruneResourceSniffPreviewLeasesLocked(now)
	lease, ok := service.resourcePreviewLeases[strings.TrimSpace(leaseID)]
	if !ok {
		return
	}
	if lease.HLSKeyOverrides == nil {
		lease.HLSKeyOverrides = make(map[string]resourceSniffHLSKeyOverride)
	}
	normalized := override
	normalized.URL = targetURL
	normalized.Key = append([]byte(nil), override.Key...)
	lease.HLSKeyOverrides[targetURL] = normalized
	service.resourcePreviewLeases[strings.TrimSpace(leaseID)] = lease
}

func serveResourceSniffPreviewHLSKeyOverride(w http.ResponseWriter, r *http.Request, override resourceSniffHLSKeyOverride) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(override.Key)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(override.Key)
}

func (service *LibraryService) prepareResourceSniffPreviewHLSKeyOverride(ctx context.Context, lease resourceSniffPreviewLease, manifestURL string, contentType string, body []byte) {
	if strings.TrimSpace(lease.Kind) != "live" || len(body) == 0 {
		return
	}
	preflight := appytdlp.AnalyzeStreamManifest(manifestURL, body, contentType, nil)
	if preflight.Kind != appytdlp.StreamManifestKindHLS ||
		preflight.EncryptionType != appytdlp.StreamEncryptionAES128 ||
		preflight.DRM ||
		preflight.KeyURICount != 1 ||
		strings.TrimSpace(preflight.KeyURI) == "" {
		return
	}
	keyProbe := service.probeYTDLPHLSKey(ctx, preflight, body, lease.Headers)
	if keyProbe == nil {
		return
	}
	preflight = appytdlp.AnalyzeStreamManifest(manifestURL, body, contentType, keyProbe)
	if preflight.IsUnsupported() ||
		preflight.KeyProbe == nil ||
		!preflight.KeyProbe.ManifestKeyOverride ||
		strings.TrimSpace(preflight.KeyProbe.NormalizedKeyHex) == "" {
		return
	}
	key, err := hex.DecodeString(strings.TrimSpace(preflight.KeyProbe.NormalizedKeyHex))
	if err != nil || len(key) != 16 {
		return
	}
	targetURL := resourceSniffPreviewReferenceTargetURL(manifestURL, preflight.KeyURI)
	if strings.TrimSpace(targetURL) == "" {
		return
	}
	service.setResourceSniffPreviewHLSKeyOverride(lease.ID, resourceSniffHLSKeyOverride{
		URL:         targetURL,
		Key:         key,
		Source:      preflight.KeyProbe.NormalizedKeySource,
		Rule:        preflight.KeyProbe.NormalizedKeyRule,
		NonStandard: preflight.KeyProbe.NormalizedKeyNonStandard,
	})
}

func resourceSniffPreviewLeaseKind(resource dto.ResourceSniffRawResource) string {
	if resourceSniffRawFLVStream(resource.URL, resource.MimeType, resource.ContentType) &&
		!resourceSniffRawMediaSegment(resource.URL, resource.MimeType, resource.ContentType, resource.ResourceType) {
		return "flv"
	}
	kind := strings.TrimSpace(resource.Kind)
	switch kind {
	case "image":
		return "image"
	case "video":
		if resourceSniffRawProgressiveVideo(resource) {
			return "video"
		}
		if resourceSniffRawLivePreview(resource) {
			return "live"
		}
	case "live":
		if resourceSniffRawLivePreview(resource) {
			return "live"
		}
	}
	return ""
}

func resourceSniffRawLivePreview(resource dto.ResourceSniffRawResource) bool {
	if resourceSniffRawMediaSegment(resource.URL, resource.MimeType, resource.ContentType, resource.ResourceType) {
		return false
	}
	if resourceSniffRawFLVStream(resource.URL, resource.MimeType, resource.ContentType) && resource.SizeBytes > 0 {
		return false
	}
	return resourceSniffRawManifestStream(resource.URL, resource.MimeType, resource.ContentType) ||
		resourceSniffRawLiveStream(resource.URL, resource.MimeType, resource.ContentType)
}

func resourceSniffRawProgressiveVideo(resource dto.ResourceSniffRawResource) bool {
	lowerContent := strings.ToLower(firstNonEmpty(resource.MimeType, resource.ContentType))
	if resourceSniffRawMediaSegment(resource.URL, resource.MimeType, resource.ContentType, resource.ResourceType) {
		return false
	}
	if resourceSniffRawManifestStream(resource.URL, resource.MimeType, resource.ContentType) ||
		resourceSniffRawLiveStream(resource.URL, resource.MimeType, resource.ContentType) {
		if !resourceSniffRawFLVStream(resource.URL, resource.MimeType, resource.ContentType) || resource.SizeBytes <= 0 {
			return false
		}
	}
	if strings.HasPrefix(lowerContent, "video/") {
		return true
	}
	switch strings.ToLower(strings.TrimPrefix(path.Ext(resourcePathFromURL(resource.URL)), ".")) {
	case "mp4", "m4v", "mov", "webm", "flv":
		return true
	default:
		return false
	}
}

func resourceSniffPreviewFileName(resource dto.ResourceSniffRawResource) string {
	rawName := strings.TrimSpace(path.Base(resourcePathFromURL(resource.URL)))
	if rawName == "" || rawName == "." || rawName == "/" {
		rawName = strings.TrimSpace(resource.ID)
	}
	if rawName == "" {
		rawName = "sniff-preview"
	}
	if filepath.Ext(rawName) == "" {
		if ext := strings.TrimPrefix(resourceSniffRawExt(resource), "."); ext != "" {
			rawName += "." + ext
		}
	}
	return sanitizeFileName(rawName)
}

func resourceSniffPreviewLeaseIDFromPath(rawPath string) string {
	value := strings.TrimSpace(rawPath)
	value = strings.TrimPrefix(value, "/api/sniff/resource-preview")
	value = strings.TrimPrefix(value, "/")
	if value == "" {
		return ""
	}
	segment := value
	if index := strings.IndexByte(segment, '/'); index >= 0 {
		segment = segment[:index]
	}
	decoded, err := url.PathUnescape(segment)
	if err != nil {
		return strings.TrimSpace(segment)
	}
	return strings.TrimSpace(decoded)
}

func resourceSniffPreviewPathAfterLeaseID(rawPath string) string {
	value := strings.TrimSpace(rawPath)
	value = strings.TrimPrefix(value, "/api/sniff/resource-preview")
	value = strings.TrimPrefix(value, "/")
	if value == "" {
		return ""
	}
	index := strings.IndexByte(value, '/')
	if index < 0 || index+1 >= len(value) {
		return ""
	}
	return strings.TrimSpace(value[index+1:])
}

func resourceSniffPreviewTargetURL(lease resourceSniffPreviewLease, r *http.Request) (string, error) {
	if strings.TrimSpace(lease.Kind) != "live" {
		return strings.TrimSpace(lease.URL), nil
	}
	if r != nil {
		if rawTarget := strings.TrimSpace(r.URL.Query().Get(resourceSniffPreviewProxyURLQueryParam)); rawTarget != "" {
			return resourceSniffResolvePreviewTargetURL(lease.URL, rawTarget)
		}
		relativePath := resourceSniffPreviewPathAfterLeaseID(r.URL.Path)
		if decoded, err := url.PathUnescape(relativePath); err == nil {
			relativePath = decoded
		}
		if relativePath != "" && relativePath != strings.TrimSpace(lease.FileName) {
			relativeRef := relativePath
			if rawQuery := strings.TrimSpace(r.URL.RawQuery); rawQuery != "" {
				relativeRef += "?" + rawQuery
			}
			return resourceSniffResolvePreviewTargetURL(lease.URL, relativeRef)
		}
	}
	return strings.TrimSpace(lease.URL), nil
}

func resourceSniffResolvePreviewTargetURL(baseRaw string, referenceRaw string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(baseRaw))
	if err != nil {
		return "", err
	}
	reference, err := url.Parse(strings.TrimSpace(referenceRaw))
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(reference).String()
	if !resourceSniffRawFetchableURL(resolved) {
		return "", fmt.Errorf("preview target is not fetchable")
	}
	return resolved, nil
}

func resourceSniffPreviewShouldRewriteHLS(lease resourceSniffPreviewLease, targetURL string, headers http.Header) bool {
	if strings.TrimSpace(lease.Kind) != "live" {
		return false
	}
	return resourceSniffRawHLSStream(targetURL, headers.Get("Content-Type"), firstNonEmpty(lease.MimeType, lease.ContentType))
}

func rewriteResourceSniffHLSManifest(body string, baseURL string, r *http.Request, lease resourceSniffPreviewLease) string {
	if body == "" {
		return body
	}
	lines := strings.SplitAfter(body, "\n")
	var builder strings.Builder
	builder.Grow(len(body) + 128)
	for _, rawLine := range lines {
		line, ending := splitResourceSniffLineEnding(rawLine)
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			builder.WriteString(line)
		case strings.HasPrefix(trimmed, "#"):
			builder.WriteString(rewriteResourceSniffHLSURIAttributes(line, baseURL, r, lease))
		default:
			builder.WriteString(resourceSniffPreviewProxyURLForReference(trimmed, baseURL, r, lease))
		}
		builder.WriteString(ending)
	}
	return builder.String()
}

func splitResourceSniffLineEnding(line string) (string, string) {
	if strings.HasSuffix(line, "\r\n") {
		return strings.TrimSuffix(line, "\r\n"), "\r\n"
	}
	if strings.HasSuffix(line, "\n") {
		return strings.TrimSuffix(line, "\n"), "\n"
	}
	return line, ""
}

func rewriteResourceSniffHLSURIAttributes(line string, baseURL string, r *http.Request, lease resourceSniffPreviewLease) string {
	line = resourceSniffHLSDoubleQuotedURIPattern.ReplaceAllStringFunc(line, func(match string) string {
		parts := resourceSniffHLSDoubleQuotedURIPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return fmt.Sprintf(`URI="%s"`, resourceSniffPreviewProxyURLForReference(parts[1], baseURL, r, lease))
	})
	line = resourceSniffHLSSingleQuotedURIPattern.ReplaceAllStringFunc(line, func(match string) string {
		parts := resourceSniffHLSSingleQuotedURIPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return fmt.Sprintf("URI='%s'", resourceSniffPreviewProxyURLForReference(parts[1], baseURL, r, lease))
	})
	return line
}

func resourceSniffPreviewProxyURLForReference(reference string, baseURL string, _ *http.Request, _ resourceSniffPreviewLease) string {
	targetURL := resourceSniffPreviewReferenceTargetURL(baseURL, reference)
	if strings.TrimSpace(targetURL) == "" {
		return reference
	}
	values := url.Values{}
	values.Set(resourceSniffPreviewProxyURLQueryParam, targetURL)
	if manifestQuery := appytdlp.ManifestRawQuery(baseURL); strings.TrimSpace(manifestQuery) != "" {
		values.Set(resourceSniffPreviewManifestQueryParam, manifestQuery)
	}
	return (&url.URL{
		Path:     "proxy",
		RawQuery: values.Encode(),
	}).String()
}

func resourceSniffPreviewReferenceTargetURL(baseURL string, reference string) string {
	targetURL, err := resourceSniffResolvePreviewTargetURL(baseURL, reference)
	if err != nil {
		return ""
	}
	return targetURL
}

func resourceSniffPreviewFallbackTargetURL(targetURL string, r *http.Request) string {
	if r == nil {
		return ""
	}
	manifestQuery := strings.TrimSpace(r.URL.Query().Get(resourceSniffPreviewManifestQueryParam))
	if manifestQuery == "" {
		return ""
	}
	return resourceSniffPreviewApplyRawQuery(targetURL, manifestQuery)
}

func resourceSniffPreviewApplyRawQuery(targetURL string, rawQuery string) string {
	manifestQuery := strings.TrimSpace(rawQuery)
	if strings.TrimSpace(manifestQuery) == "" {
		return targetURL
	}
	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil {
		return targetURL
	}
	if strings.TrimSpace(parsed.RawQuery) == "" {
		return appytdlp.AppendRawQuery(targetURL, manifestQuery)
	}
	existing, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return targetURL
	}
	extra, err := url.ParseQuery(manifestQuery)
	if err != nil {
		return targetURL
	}
	changed := false
	for key, values := range extra {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if _, ok := existing[key]; ok {
			continue
		}
		existing[key] = values
		changed = true
	}
	if !changed {
		return targetURL
	}
	parsed.RawQuery = existing.Encode()
	return parsed.String()
}

func resourceSniffPreviewShouldRetryWithManifestQuery(statusCode int) bool {
	return statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden ||
		statusCode == http.StatusNotFound ||
		statusCode == http.StatusGone
}

func resourceSniffPreviewAcceptHeader(kind string) string {
	switch strings.TrimSpace(kind) {
	case "image":
		return "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8"
	case "video":
		return "video/*,*/*;q=0.8"
	case "flv":
		return "video/x-flv,video/*,*/*;q=0.8"
	case "live":
		return "application/vnd.apple.mpegurl,application/x-mpegurl,application/dash+xml,video/x-flv,video/*,*/*;q=0.8"
	default:
		return "*/*"
	}
}

func resourceSniffPreviewHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		return &http.Client{}
	}
	if base.Timeout == 0 {
		return base
	}
	clone := *base
	clone.Timeout = 0
	return &clone
}

func copyResourceSniffPreviewHeaders(target http.Header, source http.Header, lease resourceSniffPreviewLease) {
	for _, key := range []string{
		"Accept-Ranges",
		"Cache-Control",
		"Content-Length",
		"Content-Range",
		"ETag",
		"Last-Modified",
	} {
		if value := strings.TrimSpace(source.Get(key)); value != "" {
			target.Set(key, value)
		}
	}
	contentType := strings.TrimSpace(source.Get("Content-Type"))
	if contentType == "" {
		contentType = firstNonEmpty(lease.MimeType, lease.ContentType)
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(lease.FileName))
	}
	if contentType == "" && strings.TrimSpace(lease.Kind) == "flv" {
		contentType = "video/x-flv"
	}
	if contentType != "" {
		target.Set("Content-Type", contentType)
	}
	target.Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", lease.FileName))
}
