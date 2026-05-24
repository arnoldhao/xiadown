package service

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"mime"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/library/dto"
	settingsdto "xiadown/internal/application/settings/dto"
)

type resourceSniffRawResource struct {
	dto.ResourceSniffRawResource
	headers map[string]string
	preview *resourcePreviewSnapshot
}

type resourceSniffListPolicy struct {
	scope    string
	minBytes int64
	retain   int
}

func (service *LibraryService) ListResourceSniffSessions(ctx context.Context) ([]dto.ResourceSniffSession, error) {
	ids := service.resourceSniffSessionIDs()
	result := make([]dto.ResourceSniffSession, 0, len(ids))
	for _, id := range ids {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}
		service.syncResourceSniffTargets(id)
		service.probeResourceSniffSessionPageIdentity(id, resourceSniffIdentityProbe)
		session, ok := service.getResourceSniffSession(id)
		if !ok {
			continue
		}
		result = append(result, service.mapResourceSniffSession(session))
	}
	return result, nil
}

func (service *LibraryService) ListResourceSniffResources(ctx context.Context, request dto.ListResourceSniffResourcesRequest) (dto.ListResourceSniffResourcesResponse, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return dto.ListResourceSniffResourcesResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session is required")
	}
	service.syncResourceSniffTargets(sessionID)
	service.probeResourceSniffSessionPageIdentity(sessionID, resourceSniffIdentityProbe)
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok {
		return dto.ListResourceSniffResourcesResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	resources := applyResourceSniffListPolicy(
		service.listResourceSniffRawResources(session),
		service.resourceSniffListPolicy(ctx),
	)
	items := make([]dto.ResourceSniffRawResource, 0, len(resources))
	for _, resource := range resources {
		items = append(items, resource.ResourceSniffRawResource)
	}
	return dto.ListResourceSniffResourcesResponse{
		Session:   service.mapResourceSniffSession(session),
		Resources: items,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}, nil
}

func (service *LibraryService) ClearResourceSniffResources(ctx context.Context, request dto.ClearResourceSniffResourcesRequest) error {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session is required")
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	captures := make([]*resourceCaptureState, 0)
	service.resourceSniffMu.Lock()
	session, ok := service.resourceSniffs[sessionID]
	if ok && session != nil {
		if session.Capture != nil {
			captures = append(captures, session.Capture)
		}
		for _, tab := range session.Tabs {
			if tab != nil && tab.Capture != nil {
				captures = append(captures, tab.Capture)
			}
		}
	}
	service.resourceSniffMu.Unlock()
	if !ok || session == nil {
		return apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	for _, capture := range captures {
		capture.clear()
	}
	return nil
}

func (service *LibraryService) GetResourceSniffPreview(ctx context.Context, request dto.GetResourceSniffPreviewRequest) (dto.ResourceSniffPreviewResponse, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	resourceID := strings.TrimSpace(request.ResourceID)
	if sessionID == "" || resourceID == "" {
		return dto.ResourceSniffPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff preview is required")
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return dto.ResourceSniffPreviewResponse{}, ctx.Err()
		default:
		}
	}
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok {
		return dto.ResourceSniffPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	for _, resource := range service.listResourceSniffRawResources(session) {
		if resource.ID != resourceID {
			continue
		}
		if resource.preview == nil || len(resource.preview.Body) == 0 {
			return dto.ResourceSniffPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff preview is unavailable")
		}
		return dto.ResourceSniffPreviewResponse{
			ResourceID: resource.ID,
			Kind:       firstNonEmpty(strings.TrimSpace(resource.preview.Kind), strings.TrimSpace(resource.PreviewKind), strings.TrimSpace(resource.Kind)),
			MimeType:   firstNonEmpty(strings.TrimSpace(resource.preview.MimeType), strings.TrimSpace(resource.PreviewMimeType), "image/png"),
			SizeBytes:  firstPositiveInt64(resource.preview.SizeBytes, int64(len(resource.preview.Body))),
			DataBase64: base64.StdEncoding.EncodeToString(resource.preview.Body),
			SeenAt:     formatResourceSniffRawTime(resource.preview.SeenAt),
		}, nil
	}
	return dto.ResourceSniffPreviewResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff raw resource not found")
}

func (service *LibraryService) PrepareResourceSniffRawDownload(ctx context.Context, request dto.PrepareResourceSniffRawDownloadRequest) (dto.ParseYTDLPDownloadResponse, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	resourceID := strings.TrimSpace(request.ResourceID)
	if sessionID == "" || resourceID == "" {
		return dto.ParseYTDLPDownloadResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff resource is required")
	}
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok {
		return dto.ParseYTDLPDownloadResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	var selected resourceSniffRawResource
	for _, resource := range service.listResourceSniffRawResources(session) {
		if resource.ID == resourceID {
			selected = resource
			break
		}
	}
	if selected.ID == "" {
		return dto.ParseYTDLPDownloadResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff raw resource not found")
	}
	if !resourceSniffRawDownloadable(selected.ResourceSniffRawResource) {
		return dto.ParseYTDLPDownloadResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff raw resource is not downloadable")
	}
	media := resourceMediaFromRawResource(selected)
	mediaID := service.putResourceMediaSnapshot(media)
	if mediaID == "" {
		return dto.ParseYTDLPDownloadResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff media snapshot is unavailable")
	}
	return dto.ParseYTDLPDownloadResponse{
		Title:           media.Title,
		Domain:          media.Domain,
		Extractor:       media.Extractor,
		PageURL:         media.PageURL,
		ResourceMediaID: mediaID,
		Formats:         []dto.YTDLPFormatOption{resourceFormatOptionWithID(mediaID, media)},
		Subtitles:       []dto.YTDLPSubtitleOption{},
	}, nil
}

func (service *LibraryService) GetCDPBrowserStatus(ctx context.Context) (dto.CDPBrowserStatus, error) {
	sessions, err := service.ListResourceSniffSessions(ctx)
	if err != nil {
		return dto.CDPBrowserStatus{}, err
	}
	activeSessionPIDs := service.resourceSniffRuntimePIDs()
	runtimes, runtimeErr := browsercdp.ListRuntimeProcesses(ctx)
	if runtimeErr != nil {
		runtimes = nil
	}
	activeRuntimeCount := 0
	orphanCount := 0
	var orphan *browsercdp.RuntimeProcessInfo
	for index := range runtimes {
		runtime := runtimes[index]
		if !runtime.Ready {
			continue
		}
		activeRuntimeCount++
		if _, ok := activeSessionPIDs[runtime.PID]; ok {
			continue
		}
		orphanCount++
		if orphan == nil {
			orphan = &runtime
		}
	}
	for index := range sessions {
		session := sessions[index]
		if session.BrowserStatus == resourceSniffBrowserStatusClosed {
			continue
		}
		return dto.CDPBrowserStatus{
			Active:        true,
			Mode:          "resource_sniff",
			Session:       &session,
			BrowserStatus: session.BrowserStatus,
			CurrentURL:    session.CurrentURL,
			Title:         session.Title,
			TabCount:      session.TabCount,
			ProcessCount:  activeRuntimeCount,
			OrphanCount:   orphanCount,
		}, nil
	}
	if orphan != nil {
		return dto.CDPBrowserStatus{
			Active:        true,
			Mode:          "orphan",
			RuntimeID:     orphan.ID,
			BrowserStatus: resourceSniffBrowserStatusOpen,
			PID:           orphan.PID,
			ProcessCount:  activeRuntimeCount,
			OrphanCount:   orphanCount,
			StartedAt:     orphan.CreatedAt.Format(time.RFC3339),
		}, nil
	}
	return dto.CDPBrowserStatus{
		Active:       false,
		ProcessCount: activeRuntimeCount,
		OrphanCount:  orphanCount,
	}, nil
}

func (service *LibraryService) StopCDPBrowserRuntime(ctx context.Context, request dto.StopCDPBrowserRuntimeRequest) error {
	return browsercdp.StopRuntimeProcess(request.RuntimeID)
}

func (service *LibraryService) resourceSniffSessionIDs() []string {
	if service == nil {
		return nil
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	ids := make([]string, 0, len(service.resourceSniffs))
	for id := range service.resourceSniffs {
		if strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (service *LibraryService) resourceSniffRuntimePIDs() map[int]struct{} {
	result := map[int]struct{}{}
	if service == nil {
		return result
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	for _, session := range service.resourceSniffs {
		if session == nil || session.Runtime == nil {
			continue
		}
		if pid := session.Runtime.ProcessInfo().PID; pid > 0 {
			result[pid] = struct{}{}
		}
	}
	return result
}

func (service *LibraryService) listResourceSniffRawResources(session *resourceSniffSession) []resourceSniffRawResource {
	if session == nil {
		return nil
	}
	type tabSnapshot struct {
		targetID string
		capture  *resourceCaptureState
	}
	service.resourceSniffMu.Lock()
	tabs := make([]tabSnapshot, 0, len(session.Tabs))
	for _, tab := range session.Tabs {
		if tab == nil || tab.Capture == nil {
			continue
		}
		tabs = append(tabs, tabSnapshot{targetID: tab.TargetID, capture: tab.Capture})
	}
	service.resourceSniffMu.Unlock()

	result := make([]resourceSniffRawResource, 0)
	for _, tab := range tabs {
		accepted, rejected := tab.capture.snapshot()
		responses := tab.capture.apiResponsesSnapshot()
		previews := tab.capture.previewsSnapshot()
		subtitles := tab.capture.subtitlesSnapshot()
		observed := tab.capture.observedSnapshot()
		tabResources := make([]resourceSniffRawResource, 0, len(observed)+len(accepted)+len(rejected)+len(responses)+len(subtitles))
		if len(observed) > 0 {
			tabResources = append(tabResources, rawResourcesFromObserved(tab.targetID, observed)...)
		} else {
			tabResources = append(tabResources, rawResourcesFromCandidates(tab.targetID, accepted)...)
			tabResources = append(tabResources, rawResourcesFromRejected(tab.targetID, rejected)...)
		}
		tabResources = append(tabResources, rawResourcesFromAPIResponses(tab.targetID, responses)...)
		tabResources = append(tabResources, rawResourcesFromSubtitles(tab.targetID, subtitles)...)
		result = append(result, attachResourceSniffRawPreviews(tabResources, previews)...)
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := parseResourceSniffRawSeenAt(result[i].SeenAt)
		right := parseResourceSniffRawSeenAt(result[j].SeenAt)
		if !left.Equal(right) {
			return left.After(right)
		}
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].URL < result[j].URL
	})
	return dedupeResourceSniffRawResources(result)
}

func attachResourceSniffRawPreviews(items []resourceSniffRawResource, previews []resourcePreviewSnapshot) []resourceSniffRawResource {
	if len(items) == 0 || len(previews) == 0 {
		return items
	}
	previewByURL := make(map[string]resourcePreviewSnapshot, len(previews))
	for _, preview := range previews {
		if strings.TrimSpace(preview.Kind) == "" || len(preview.Body) == 0 {
			continue
		}
		key := resourceComparableURL(preview.URL, false)
		if key == "" {
			continue
		}
		previewByURL[key] = preview
	}
	if len(previewByURL) == 0 {
		return items
	}
	for index := range items {
		key := resourceComparableURL(items[index].URL, false)
		preview, ok := previewByURL[key]
		if !ok {
			continue
		}
		previewCopy := preview
		items[index].preview = &previewCopy
		items[index].PreviewAvailable = true
		items[index].PreviewKind = firstNonEmpty(strings.TrimSpace(preview.Kind), "image")
		items[index].PreviewMimeType = strings.TrimSpace(preview.MimeType)
		items[index].PreviewSizeBytes = firstPositiveInt64(preview.SizeBytes, int64(len(preview.Body)))
		items[index].PreviewDataBase64 = base64.StdEncoding.EncodeToString(preview.Body)
	}
	return items
}

func rawResourcesFromObserved(targetID string, observed []resourceObservedResource) []resourceSniffRawResource {
	result := make([]resourceSniffRawResource, 0, len(observed))
	for _, resource := range observed {
		item := dto.ResourceSniffRawResource{
			Source:       "network",
			URL:          strings.TrimSpace(resource.url),
			PageURL:      strings.TrimSpace(resource.pageURL),
			Domain:       extractRegistrableDomain(resource.url),
			MimeType:     strings.TrimSpace(resource.mimeType),
			ContentType:  strings.TrimSpace(resource.contentType),
			ResourceType: strings.TrimSpace(resource.resourceType),
			Status:       resource.status,
			SizeBytes:    resource.sizeBytes,
			TargetID:     strings.TrimSpace(targetID),
			SeenAt:       formatResourceSniffRawTime(resource.seenAt),
		}
		item.Kind = resourceSniffRawKind(item.Source, item.URL, item.MimeType, item.ContentType, item.ResourceType, item.SizeBytes)
		item.Downloadable = resourceSniffRawDownloadable(item)
		item.ID = resourceSniffRawResourceID(item)
		result = append(result, resourceSniffRawResource{
			ResourceSniffRawResource: item,
			headers:                  cloneStringMap(resource.headers),
		})
	}
	return result
}

func rawResourcesFromCandidates(targetID string, candidates []resourceCandidate) []resourceSniffRawResource {
	result := make([]resourceSniffRawResource, 0, len(candidates))
	for _, candidate := range candidates {
		item := dto.ResourceSniffRawResource{
			Source:       "candidate",
			URL:          strings.TrimSpace(candidate.url),
			PageURL:      strings.TrimSpace(candidate.pageURL),
			Domain:       extractRegistrableDomain(candidate.url),
			MimeType:     strings.TrimSpace(candidate.mimeType),
			ContentType:  strings.TrimSpace(candidate.contentType),
			ResourceType: strings.TrimSpace(candidate.resourceType),
			Status:       candidate.status,
			SizeBytes:    candidate.sizeBytes,
			Score:        candidate.score,
			TargetID:     strings.TrimSpace(targetID),
			SeenAt:       formatResourceSniffRawTime(candidate.seenAt),
		}
		item.Kind = resourceSniffRawKind(item.Source, item.URL, item.MimeType, item.ContentType, item.ResourceType, item.SizeBytes)
		item.Downloadable = resourceSniffRawDownloadable(item)
		item.ID = resourceSniffRawResourceID(item)
		result = append(result, resourceSniffRawResource{
			ResourceSniffRawResource: item,
			headers:                  cloneStringMap(candidate.headers),
		})
	}
	return result
}

func rawResourcesFromRejected(targetID string, rejected []resourceRejectedCandidate) []resourceSniffRawResource {
	result := make([]resourceSniffRawResource, 0, len(rejected))
	for _, candidate := range rejected {
		item := dto.ResourceSniffRawResource{
			Source:       "rejected",
			URL:          strings.TrimSpace(candidate.url),
			Domain:       extractRegistrableDomain(candidate.url),
			MimeType:     strings.TrimSpace(candidate.mimeType),
			ContentType:  strings.TrimSpace(candidate.contentType),
			ResourceType: strings.TrimSpace(candidate.resourceType),
			Status:       candidate.status,
			SizeBytes:    candidate.sizeBytes,
			Score:        candidate.score,
			Reason:       strings.TrimSpace(candidate.reason),
			TargetID:     strings.TrimSpace(targetID),
			SeenAt:       formatResourceSniffRawTime(candidate.seenAt),
		}
		item.Kind = resourceSniffRawKind(item.Source, item.URL, item.MimeType, item.ContentType, item.ResourceType, item.SizeBytes)
		item.Downloadable = resourceSniffRawDownloadable(item)
		item.ID = resourceSniffRawResourceID(item)
		result = append(result, resourceSniffRawResource{
			ResourceSniffRawResource: item,
			headers:                  cloneStringMap(candidate.headers),
		})
	}
	return result
}

func rawResourcesFromAPIResponses(targetID string, responses []resourceAPIResponse) []resourceSniffRawResource {
	result := make([]resourceSniffRawResource, 0, len(responses))
	for _, response := range responses {
		source := "api_response"
		if resourceSniffRawManifestStream(response.URL, response.MimeType, response.ContentType) {
			source = "network"
		}
		item := dto.ResourceSniffRawResource{
			Source:       source,
			URL:          strings.TrimSpace(response.URL),
			PageURL:      strings.TrimSpace(response.PageURL),
			Domain:       extractRegistrableDomain(response.URL),
			MimeType:     strings.TrimSpace(response.MimeType),
			ContentType:  strings.TrimSpace(response.ContentType),
			ResourceType: string(response.ResourceType),
			Status:       response.Status,
			SizeBytes:    firstPositiveInt64(response.SizeBytes, int64(len(response.Body))),
			TargetID:     strings.TrimSpace(targetID),
			SeenAt:       formatResourceSniffRawTime(response.SeenAt),
		}
		item.Kind = resourceSniffRawKindWithBody(item.Source, item.URL, item.MimeType, item.ContentType, item.ResourceType, response.Body, item.SizeBytes)
		item.Downloadable = resourceSniffRawDownloadable(item)
		item.ID = resourceSniffRawResourceID(item)
		result = append(result, resourceSniffRawResource{
			ResourceSniffRawResource: item,
			headers:                  cloneStringMap(response.RequestHeaders),
		})
	}
	return result
}

func rawResourcesFromSubtitles(targetID string, subtitles []resourceSubtitle) []resourceSniffRawResource {
	result := make([]resourceSniffRawResource, 0, len(subtitles))
	for _, subtitle := range subtitles {
		size := int64(len(subtitle.Data))
		item := dto.ResourceSniffRawResource{
			Source:      "subtitle",
			URL:         strings.TrimSpace(subtitle.URL),
			PageURL:     strings.TrimSpace(subtitle.PageURL),
			Domain:      extractRegistrableDomain(subtitle.URL),
			ContentType: strings.TrimSpace(subtitle.ContentType),
			SizeBytes:   size,
			TargetID:    strings.TrimSpace(targetID),
			SeenAt:      formatResourceSniffRawTime(subtitle.SeenAt),
		}
		item.Kind = resourceSniffRawKind(item.Source, item.URL, item.MimeType, item.ContentType, item.ResourceType, item.SizeBytes)
		item.Downloadable = resourceSniffRawDownloadable(item)
		item.ID = resourceSniffRawResourceID(item)
		result = append(result, resourceSniffRawResource{
			ResourceSniffRawResource: item,
			headers:                  cloneStringMap(subtitle.RequestHeaders),
		})
	}
	return result
}

func dedupeResourceSniffRawResources(items []resourceSniffRawResource) []resourceSniffRawResource {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]resourceSniffRawResource, 0, len(items))
	byURL := make(map[string]int, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.ID)
		if key == "" {
			key = resourceSniffRawResourceID(item.ResourceSniffRawResource)
			item.ID = key
		}
		if _, ok := seen[key]; ok {
			continue
		}
		if urlKey := resourceSniffRawURLDedupeKey(item.ResourceSniffRawResource); urlKey != "" {
			if existingIndex, ok := byURL[urlKey]; ok {
				if resourceSniffRawResourcePreferred(item.ResourceSniffRawResource, result[existingIndex].ResourceSniffRawResource) {
					delete(seen, strings.TrimSpace(result[existingIndex].ID))
					seen[key] = struct{}{}
					result[existingIndex] = item
				}
				continue
			}
			byURL[urlKey] = len(result)
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func resourceSniffRawURLDedupeKey(resource dto.ResourceSniffRawResource) string {
	if !resourceSniffRawManifestStream(resource.URL, resource.MimeType, resource.ContentType) &&
		!resourceSniffRawFLVStream(resource.URL, resource.MimeType, resource.ContentType) {
		return ""
	}
	key := resourceComparableURL(resource.URL, false)
	if key == "" {
		return ""
	}
	return strings.Join([]string{
		strings.TrimSpace(resource.TargetID),
		key,
		resourceComparableURL(resource.PageURL, false),
	}, "\x00")
}

func resourceSniffRawResourcePreferred(left dto.ResourceSniffRawResource, right dto.ResourceSniffRawResource) bool {
	if left.Downloadable != right.Downloadable {
		return left.Downloadable
	}
	leftKind := strings.TrimSpace(left.Kind)
	rightKind := strings.TrimSpace(right.Kind)
	if leftKind != rightKind {
		if leftKind == "video" || leftKind == "audio" {
			return true
		}
		if rightKind == "video" || rightKind == "audio" {
			return false
		}
	}
	leftSeen := parseResourceSniffRawSeenAt(left.SeenAt)
	rightSeen := parseResourceSniffRawSeenAt(right.SeenAt)
	if !leftSeen.Equal(rightSeen) {
		return leftSeen.After(rightSeen)
	}
	return left.SizeBytes > right.SizeBytes
}

func (service *LibraryService) resourceSniffListPolicy(ctx context.Context) resourceSniffListPolicy {
	current := settingsdto.Settings{
		ResourceSniffScope:    "default",
		ResourceSniffMinBytes: 8 * 1024,
		ResourceSniffRetain:   1000,
	}
	if service != nil && service.settings != nil {
		if settings, err := service.settings.GetSettings(ctx); err == nil {
			current = settings
		}
	}
	return resourceSniffListPolicy{
		scope:    normalizeResourceSniffScope(current.ResourceSniffScope),
		minBytes: int64(normalizeResourceSniffMinBytes(current.ResourceSniffMinBytes)),
		retain:   normalizeResourceSniffRetain(current.ResourceSniffRetain),
	}
}

func applyResourceSniffListPolicy(items []resourceSniffRawResource, policy resourceSniffListPolicy) []resourceSniffRawResource {
	if len(items) == 0 {
		return nil
	}
	scope := normalizeResourceSniffScope(policy.scope)
	retain := normalizeResourceSniffRetain(policy.retain)
	minBytes := int64(normalizeResourceSniffMinBytes(int(policy.minBytes)))
	result := make([]resourceSniffRawResource, 0, minInt(len(items), retain))
	for _, item := range items {
		if resourceSniffRawHiddenFromList(item.ResourceSniffRawResource) {
			continue
		}
		if !resourceSniffScopeAllowsResource(scope, item.ResourceSniffRawResource) {
			continue
		}
		if resourceSniffResourceBelowMinBytes(item.ResourceSniffRawResource, minBytes) {
			continue
		}
		result = append(result, item)
		if len(result) >= retain {
			break
		}
	}
	return result
}

func normalizeResourceSniffScope(value string) string {
	switch strings.TrimSpace(value) {
	case "advanced", "all":
		return strings.TrimSpace(value)
	default:
		return "default"
	}
}

func normalizeResourceSniffMinBytes(value int) int {
	if value <= 0 {
		return 8 * 1024
	}
	const max = 10 * 1024 * 1024
	if value > max {
		return max
	}
	return value
}

func normalizeResourceSniffRetain(value int) int {
	if value <= 0 {
		return 1000
	}
	if value < 100 {
		return 100
	}
	if value > 10000 {
		return 10000
	}
	return value
}

func resourceSniffScopeAllowsResource(scope string, resource dto.ResourceSniffRawResource) bool {
	kind := strings.TrimSpace(resource.Kind)
	source := strings.TrimSpace(resource.Source)
	if scope == "all" {
		return true
	}
	if source == "rejected" {
		return scope == "advanced"
	}
	if scope == "advanced" {
		switch kind {
		case "video", "audio", "live", "manifest", "image", "subtitle", "api", "document", "font", "archive", "other":
			return true
		default:
			return false
		}
	}
	switch kind {
	case "video", "audio", "live", "manifest", "image", "subtitle":
		return source == "network" || source == "candidate" || source == "subtitle"
	default:
		return false
	}
}

func resourceSniffRawHiddenFromList(resource dto.ResourceSniffRawResource) bool {
	if strings.EqualFold(strings.TrimSpace(resource.Kind), "segment") {
		return true
	}
	return resourceSniffRawMediaSegment(resource.URL, resource.MimeType, resource.ContentType, resource.ResourceType)
}

func resourceSniffResourceBelowMinBytes(resource dto.ResourceSniffRawResource, minBytes int64) bool {
	if minBytes <= 0 || resource.SizeBytes <= 0 || resource.SizeBytes >= minBytes {
		return false
	}
	switch strings.TrimSpace(resource.Kind) {
	case "image", "document", "font", "archive", "other":
		return true
	default:
		return false
	}
}

func resourceMediaFromRawResource(resource resourceSniffRawResource) resourceMedia {
	ext := resourceSniffRawExt(resource.ResourceSniffRawResource)
	title := resourceSniffRawTitle(resource.ResourceSniffRawResource)
	kind := resourceSniffRawKind(
		resource.Source,
		resource.URL,
		resource.MimeType,
		resource.ContentType,
		resource.ResourceType,
		resource.SizeBytes,
	)
	return resourceMedia{
		URL:            strings.TrimSpace(resource.URL),
		PageURL:        firstNonEmpty(strings.TrimSpace(resource.PageURL), strings.TrimSpace(resource.URL)),
		Kind:           firstNonEmpty(kind, strings.TrimSpace(resource.Kind)),
		Title:          title,
		Domain:         firstNonEmpty(strings.TrimSpace(resource.Domain), extractRegistrableDomain(resource.URL)),
		Extractor:      "sniff",
		ContentType:    strings.TrimSpace(resource.ContentType),
		MimeType:       strings.TrimSpace(resource.MimeType),
		Ext:            ext,
		SizeBytes:      resource.SizeBytes,
		RequestHeaders: cloneStringMap(resource.headers),
	}
}

func resourceSniffRawKind(source string, rawURL string, mimeType string, contentType string, resourceType string, sizeBytes ...int64) string {
	return resourceSniffRawKindWithBody(source, rawURL, mimeType, contentType, resourceType, nil, sizeBytes...)
}

func resourceSniffRawKindWithBody(source string, rawURL string, mimeType string, contentType string, resourceType string, body []byte, sizeBytes ...int64) string {
	lowerURL := strings.ToLower(strings.TrimSpace(rawURL))
	lowerMime := strings.ToLower(strings.TrimSpace(firstNonEmpty(mimeType, contentType)))
	if resourceSniffRawManifestStream(rawURL, mimeType, contentType) {
		if resourceSniffRawHLSStream(rawURL, mimeType, contentType) && resourceHLSManifestDownloadable(body) {
			return "video"
		}
		return "live"
	}
	if resourceSniffRawMediaSegment(rawURL, mimeType, contentType, resourceType) {
		return "segment"
	}
	if resourceSniffRawFLVStream(rawURL, mimeType, contentType) {
		if firstPositiveInt64(sizeBytes...) > 0 {
			return "video"
		}
		return "live"
	}
	if resourceSniffRawLiveStream(rawURL, mimeType, contentType) {
		return "live"
	}
	if declaredKind := resourceSniffRawDeclaredKind(mimeType, contentType, resourceType); declaredKind != "" {
		return declaredKind
	}
	switch strings.TrimSpace(source) {
	case "api_response":
		return "api"
	}
	if resourceSubtitleExt(rawURL, mimeType, contentType, "") != "" {
		return "subtitle"
	}
	if strings.HasPrefix(lowerMime, "image/") ||
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
	if strings.HasPrefix(lowerMime, "video/") || strings.Contains(lowerURL, ".mp4") || strings.Contains(lowerURL, "mime_type=video") {
		return "video"
	}
	if strings.HasPrefix(lowerMime, "audio/") || strings.Contains(lowerURL, ".m4a") || strings.Contains(lowerURL, ".mp3") {
		return "audio"
	}
	if strings.EqualFold(strings.TrimSpace(resourceType), "Image") {
		return "image"
	}
	if strings.EqualFold(strings.TrimSpace(resourceType), "Media") {
		return "other"
	}
	if strings.TrimSpace(source) == "subtitle" {
		return "subtitle"
	}
	if strings.HasPrefix(lowerMime, "font/") ||
		strings.Contains(lowerMime, "font") ||
		strings.Contains(lowerURL, ".woff2") ||
		strings.Contains(lowerURL, ".woff") ||
		strings.Contains(lowerURL, ".ttf") ||
		strings.Contains(lowerURL, ".otf") ||
		strings.Contains(lowerURL, ".eot") {
		return "font"
	}
	if strings.Contains(lowerMime, "pdf") ||
		strings.Contains(lowerMime, "msword") ||
		strings.Contains(lowerMime, "officedocument") ||
		strings.Contains(lowerMime, "spreadsheet") ||
		strings.Contains(lowerMime, "presentation") ||
		strings.Contains(lowerMime, "wordprocessing") ||
		strings.Contains(lowerURL, ".pdf") ||
		strings.Contains(lowerURL, ".doc") ||
		strings.Contains(lowerURL, ".docx") ||
		strings.Contains(lowerURL, ".xls") ||
		strings.Contains(lowerURL, ".xlsx") ||
		strings.Contains(lowerURL, ".ppt") ||
		strings.Contains(lowerURL, ".pptx") {
		return "document"
	}
	if strings.Contains(lowerMime, "zip") ||
		strings.Contains(lowerMime, "rar") ||
		strings.Contains(lowerMime, "7z") ||
		strings.Contains(lowerURL, ".zip") ||
		strings.Contains(lowerURL, ".rar") ||
		strings.Contains(lowerURL, ".7z") ||
		strings.Contains(lowerURL, ".dmg") ||
		strings.Contains(lowerURL, ".pkg") ||
		strings.Contains(lowerURL, ".exe") {
		return "archive"
	}
	return "other"
}

func resourceSniffRawDeclaredKind(mimeType string, contentType string, resourceType string) string {
	lowerContent := strings.ToLower(strings.TrimSpace(firstNonEmpty(mimeType, contentType)))
	if parsedContent, _, err := mime.ParseMediaType(lowerContent); err == nil {
		lowerContent = strings.TrimSpace(parsedContent)
	}
	switch {
	case resourceSubtitleExt("", lowerContent, "", "") != "":
		return "subtitle"
	case strings.HasPrefix(lowerContent, "image/"):
		return "image"
	case strings.HasPrefix(lowerContent, "audio/"):
		return "audio"
	case strings.HasPrefix(lowerContent, "video/"):
		return "video"
	case strings.EqualFold(strings.TrimSpace(resourceType), "Image"):
		return "image"
	default:
		return ""
	}
}

func resourceSniffRawLiveStream(rawURL string, mimeType string, contentType string) bool {
	lowerMime := strings.ToLower(strings.TrimSpace(firstNonEmpty(mimeType, contentType)))
	return resourceSniffRawFLVStream(rawURL, mimeType, contentType) ||
		strings.Contains(lowerMime, "audio/video")
}

func resourceSniffRawFLVStream(rawURL string, mimeType string, contentType string) bool {
	lowerMime := strings.ToLower(strings.TrimSpace(firstNonEmpty(mimeType, contentType)))
	return resourceSniffURLPathExt(rawURL) == "flv" ||
		strings.Contains(lowerMime, "video/x-flv") ||
		strings.Contains(lowerMime, "x-flv")
}

func resourceSniffRawManifestStream(rawURL string, mimeType string, contentType string) bool {
	return resourceSniffRawHLSStream(rawURL, mimeType, contentType) ||
		resourceSniffRawDASHStream(rawURL, mimeType, contentType) ||
		resourceSniffRawHDSStream(rawURL, mimeType, contentType) ||
		resourceSniffRawSmoothStream(rawURL, mimeType, contentType)
}

func resourceSniffRawHLSStream(rawURL string, mimeType string, contentType string) bool {
	lowerMime := strings.ToLower(strings.TrimSpace(firstNonEmpty(mimeType, contentType)))
	return resourceSniffURLPathExt(rawURL) == "m3u8" ||
		strings.Contains(lowerMime, "mpegurl")
}

func resourceSniffRawDASHStream(rawURL string, mimeType string, contentType string) bool {
	lowerMime := strings.ToLower(strings.TrimSpace(firstNonEmpty(mimeType, contentType)))
	return resourceSniffURLPathExt(rawURL) == "mpd" ||
		strings.Contains(lowerMime, "dash+xml")
}

func resourceHLSManifestDownloadable(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	upper := strings.ToUpper(string(body))
	if !strings.Contains(upper, "#EXTM3U") {
		return false
	}
	if strings.Contains(upper, "#EXT-X-ENDLIST") {
		return true
	}
	for _, rawLine := range strings.Split(upper, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "#EXT-X-PLAYLIST-TYPE:") &&
			strings.Contains(strings.TrimPrefix(line, "#EXT-X-PLAYLIST-TYPE:"), "VOD") {
			return true
		}
	}
	return false
}

func resourceSniffRawHDSStream(rawURL string, mimeType string, contentType string) bool {
	lowerMime := strings.ToLower(strings.TrimSpace(firstNonEmpty(mimeType, contentType)))
	return resourceSniffURLPathExt(rawURL) == "f4m" ||
		strings.Contains(lowerMime, "f4m")
}

func resourceSniffRawSmoothStream(rawURL string, mimeType string, contentType string) bool {
	lowerMime := strings.ToLower(strings.TrimSpace(firstNonEmpty(mimeType, contentType)))
	lowerPath := strings.ToLower(strings.TrimSpace(resourcePathFromURL(rawURL)))
	return strings.Contains(lowerMime, "vnd.ms-sstr+xml") ||
		strings.Contains(lowerMime, "smoothstreaming") ||
		(strings.Contains(lowerPath, ".ism/") && strings.HasSuffix(lowerPath, "/manifest"))
}

func resourceSniffRawMediaSegment(rawURL string, mimeType string, contentType string, resourceTypes ...string) bool {
	lowerTypes := strings.ToLower(strings.TrimSpace(strings.Join([]string{mimeType, contentType}, " ")))
	lowerPath := strings.ToLower(strings.TrimSpace(resourcePathFromURL(rawURL)))
	ext := resourceSniffURLPathExt(rawURL)
	isMediaResource := resourceSniffResourceTypeIsMedia(resourceTypes...)
	if strings.Contains(lowerTypes, "iso.segment") ||
		strings.Contains(lowerTypes, "video/mp2t") ||
		strings.Contains(lowerTypes, "application/mp2t") {
		return true
	}
	switch ext {
	case "m4s", "cmfv", "cmfa", "cmft", "cmfm", "f4f":
		return true
	case "mp2t":
		return true
	case "ts", "m2ts", "mts", "m2t":
		return resourceSniffMPEGTSMediaSegmentType(lowerTypes) ||
			resourceSniffBinaryMediaSegmentType(lowerTypes) ||
			(isMediaResource && resourceSniffURLLooksSegmentPath(rawURL))
	case "part":
		return resourceSniffMediaSegmentPayloadType(lowerTypes) ||
			(isMediaResource && resourceSniffURLLooksSegmentPath(rawURL))
	}
	return strings.Contains(lowerPath, "/fragments(") ||
		strings.Contains(lowerPath, "/fragment(") ||
		strings.Contains(lowerPath, "fragments(video=") ||
		strings.Contains(lowerPath, "fragments(audio=")
}

func resourceSniffResourceTypeIsMedia(resourceTypes ...string) bool {
	for _, resourceType := range resourceTypes {
		if strings.EqualFold(strings.TrimSpace(resourceType), "Media") {
			return true
		}
	}
	return false
}

func resourceSniffMPEGTSMediaSegmentType(lowerTypes string) bool {
	return strings.Contains(lowerTypes, "video/mp2t") ||
		strings.Contains(lowerTypes, "application/mp2t") ||
		strings.Contains(lowerTypes, "mpegts")
}

func resourceSniffBinaryMediaSegmentType(lowerTypes string) bool {
	trimmed := strings.TrimSpace(lowerTypes)
	return strings.Contains(trimmed, "application/octet-stream") ||
		strings.Contains(trimmed, "binary/octet-stream")
}

func resourceSniffMediaSegmentPayloadType(lowerTypes string) bool {
	return strings.HasPrefix(strings.TrimSpace(lowerTypes), "video/") ||
		strings.Contains(lowerTypes, " video/") ||
		strings.HasPrefix(strings.TrimSpace(lowerTypes), "audio/") ||
		strings.Contains(lowerTypes, " audio/") ||
		strings.Contains(lowerTypes, "iso.segment") ||
		strings.Contains(lowerTypes, "mp2t")
}

func resourceSniffURLLooksSegmentPath(rawURL string) bool {
	base := strings.ToLower(strings.TrimSpace(path.Base(resourcePathFromURL(rawURL))))
	if base == "" || base == "." || base == "/" {
		return false
	}
	stem := strings.TrimSuffix(base, path.Ext(base))
	if stem == "" {
		return false
	}
	hasDigit := false
	allDigits := true
	for _, item := range stem {
		if item >= '0' && item <= '9' {
			hasDigit = true
			continue
		}
		allDigits = false
	}
	if hasDigit && allDigits {
		return true
	}
	for _, marker := range []string{"seg", "segment", "chunk", "frag", "fragment", "part", "media"} {
		if strings.Contains(stem, marker) {
			return true
		}
	}
	return false
}

func resourceSniffRawDownloadable(resource dto.ResourceSniffRawResource) bool {
	if resourceSniffRawMediaSegment(resource.URL, resource.MimeType, resource.ContentType, resource.ResourceType) {
		return false
	}
	if resourceSniffRawFLVStream(resource.URL, resource.MimeType, resource.ContentType) && resource.SizeBytes <= 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(resource.Kind)) {
	case "", "segment", "live":
		return false
	default:
		return resourceSniffRawFetchableURL(resource.URL)
	}
}

func resourceSniffRawFetchableURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(parsed.Scheme)) {
	case "http", "https":
		return strings.TrimSpace(parsed.Host) != ""
	default:
		return false
	}
}

func resourceSniffRawExt(resource dto.ResourceSniffRawResource) string {
	lowerContent := strings.ToLower(firstNonEmpty(resource.ContentType, resource.MimeType))
	if parsedContent, _, err := mime.ParseMediaType(lowerContent); err == nil {
		lowerContent = strings.TrimSpace(parsedContent)
	}
	urlExt := resourceSniffURLPathExt(resource.URL)
	switch {
	case resourceSniffRawHLSStream(resource.URL, resource.MimeType, resource.ContentType):
		return "m3u8"
	case resourceSniffRawDASHStream(resource.URL, resource.MimeType, resource.ContentType):
		return "mpd"
	case resourceSniffRawHDSStream(resource.URL, resource.MimeType, resource.ContentType):
		return "f4m"
	case resourceSniffRawSmoothStream(resource.URL, resource.MimeType, resource.ContentType):
		return "ism"
	case resourceSniffRawLiveStream(resource.URL, resource.MimeType, resource.ContentType):
		if urlExt != "" && len(urlExt) <= 5 {
			return urlExt
		}
		return "flv"
	case strings.HasPrefix(lowerContent, "audio/"):
		if strings.Contains(lowerContent, "mpeg") {
			return "mp3"
		}
		return "m4a"
	case strings.HasPrefix(lowerContent, "video/"):
		if urlExt != "" && len(urlExt) <= 5 {
			return urlExt
		}
		return "mp4"
	case strings.HasPrefix(lowerContent, "image/"):
		switch strings.TrimPrefix(lowerContent, "image/") {
		case "jpeg", "pjpeg":
			return "jpg"
		case "png", "webp", "gif", "avif", "bmp":
			return strings.TrimPrefix(lowerContent, "image/")
		case "svg+xml":
			return "svg"
		case "x-icon", "vnd.microsoft.icon":
			return "ico"
		}
		if urlExt != "" && len(urlExt) <= 5 {
			return urlExt
		}
		return "jpg"
	case lowerContent == "text/vtt":
		return "vtt"
	case strings.Contains(lowerContent, "subrip"):
		return "srt"
	case strings.Contains(lowerContent, "ttml"):
		return "ttml"
	case strings.Contains(lowerContent, "json"):
		return "json"
	case strings.HasPrefix(lowerContent, "font/") || strings.Contains(lowerContent, "font"):
		return "woff2"
	case strings.Contains(lowerContent, "zip"):
		return "zip"
	case strings.Contains(lowerContent, "rar"):
		return "rar"
	case strings.Contains(lowerContent, "7z"):
		return "7z"
	}
	if urlExt != "" && len(urlExt) <= 5 {
		return urlExt
	}
	switch strings.TrimSpace(resource.Kind) {
	case "image":
		return "jpg"
	case "subtitle":
		return "vtt"
	case "audio":
		return "mp3"
	case "video":
		return "mp4"
	case "live":
		return "flv"
	case "api":
		return "json"
	case "document", "font", "archive", "other":
		return "bin"
	}
	return "bin"
}

func resourceSniffRawTitle(resource dto.ResourceSniffRawResource) string {
	if base := strings.TrimSpace(path.Base(resourcePathFromURL(resource.URL))); base != "" && base != "." && base != "/" {
		return strings.TrimSuffix(base, path.Ext(base))
	}
	return firstNonEmpty(strings.TrimSpace(resource.Domain), extractRegistrableDomain(resource.URL), "sniff-resource")
}

func resourcePathFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	return parsed.Path
}

func resourceSniffURLPathExt(rawURL string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(path.Ext(resourcePathFromURL(rawURL))), "."))
}

func resourceSniffRawResourceID(resource dto.ResourceSniffRawResource) string {
	hash := sha1.New()
	hash.Write([]byte(strings.TrimSpace(resource.Source)))
	hash.Write([]byte{0})
	hash.Write([]byte(strings.TrimSpace(resource.TargetID)))
	hash.Write([]byte{0})
	hash.Write([]byte(strings.TrimSpace(resource.Kind)))
	hash.Write([]byte{0})
	hash.Write([]byte(resourceComparableURL(resource.URL, false)))
	hash.Write([]byte{0})
	hash.Write([]byte(resourceComparableURL(resource.PageURL, false)))
	hash.Write([]byte{0})
	hash.Write([]byte(strings.TrimSpace(resource.Reason)))
	sum := hash.Sum(nil)
	return hex.EncodeToString(sum[:10])
}

func formatResourceSniffRawTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func parseResourceSniffRawSeenAt(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed
	}
	parsed, _ = time.Parse(time.RFC3339, value)
	return parsed
}
