package service

import (
	"context"
	"strings"

	"xiadown/internal/application/library/dto"
)

const (
	resourceSniffStatusRuntimeNone    = "none"
	resourceSniffStatusRuntimeManaged = "managed"
	resourceSniffStatusRuntimeOrphan  = "orphan"

	resourceSniffStatusStateIdle     = "idle"
	resourceSniffStatusStateStarting = "starting"
	resourceSniffStatusStateActive   = "active"
	resourceSniffStatusStateClosing  = "closing"
	resourceSniffStatusStateError    = "error"
)

// GetResourceSniffStatus returns the single lightweight activity projection
// consumed by workspace status surfaces. Raw resources and preview payloads
// stay inside the service process.
func (service *LibraryService) GetResourceSniffStatus(ctx context.Context) (dto.ResourceSniffStatusSnapshot, error) {
	status, err := service.GetCDPBrowserStatus(ctx)
	if err != nil {
		return dto.ResourceSniffStatusSnapshot{}, err
	}
	return service.projectResourceSniffStatus(ctx, status), nil
}

func (service *LibraryService) projectResourceSniffStatus(ctx context.Context, status dto.CDPBrowserStatus) dto.ResourceSniffStatusSnapshot {
	if status.Active && strings.EqualFold(strings.TrimSpace(status.Mode), resourceSniffStatusRuntimeOrphan) {
		return dto.ResourceSniffStatusSnapshot{
			Runtime:   resourceSniffStatusRuntimeOrphan,
			State:     resourceSniffStatusStateActive,
			RuntimeID: strings.TrimSpace(status.RuntimeID),
			CanStop:   strings.TrimSpace(status.RuntimeID) != "",
		}
	}

	if status.Active && status.Session != nil {
		return service.projectManagedResourceSniffStatus(ctx, *status.Session)
	}

	return dto.ResourceSniffStatusSnapshot{
		Runtime: resourceSniffStatusRuntimeNone,
		State:   resourceSniffStatusStateIdle,
	}
}

func (service *LibraryService) projectManagedResourceSniffStatus(ctx context.Context, sessionDTO dto.ResourceSniffSession) dto.ResourceSniffStatusSnapshot {
	sessionID := strings.TrimSpace(sessionDTO.SessionID)
	rawURL := firstNonEmpty(strings.TrimSpace(sessionDTO.CurrentURL), strings.TrimSpace(sessionDTO.URL))
	domain := extractRegistrableDomain(rawURL)
	title := firstNonEmpty(
		resourceCleanMetadataText(sessionDTO.Title),
		domain,
	)

	resources := []resourceSniffRawResource(nil)
	if session, ok := service.getResourceSniffSession(sessionID); ok {
		resources = applyResourceSniffListPolicy(
			service.listResourceSniffRawResourcesForStatus(session),
			service.resourceSniffListPolicy(ctx),
		)
	}
	downloadableCount := 0
	for _, resource := range resources {
		if resource.Downloadable {
			downloadableCount++
		}
	}

	state := normalizeResourceSniffActivityState(sessionDTO.State, sessionDTO.BrowserStatus)
	return dto.ResourceSniffStatusSnapshot{
		Runtime:           resourceSniffStatusRuntimeManaged,
		State:             state,
		SessionID:         sessionID,
		Title:             title,
		URL:               rawURL,
		Favicon:           service.resolveResourceSniffStatusFavicon(ctx, domain),
		ResourceCount:     len(resources),
		DownloadableCount: downloadableCount,
		LastCaptureAt:     resourceSniffRawResourcesLastCaptureAt(resources),
		CanClear:          sessionID != "" && len(resources) > 0,
		CanStop:           sessionID != "" && state != resourceSniffStatusStateClosing,
	}
}

func normalizeResourceSniffActivityState(state string, browserStatus string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "starting":
		return resourceSniffStatusStateStarting
	case resourceSniffStateClosing:
		return resourceSniffStatusStateClosing
	case resourceSniffStateClosed:
		return resourceSniffStatusStateIdle
	}
	switch strings.ToLower(strings.TrimSpace(browserStatus)) {
	case resourceSniffBrowserStatusClosing:
		return resourceSniffStatusStateClosing
	case resourceSniffBrowserStatusTabClosed, resourceSniffBrowserStatusClosed:
		return resourceSniffStatusStateError
	default:
		return resourceSniffStatusStateActive
	}
}

func (service *LibraryService) resolveResourceSniffStatusFavicon(ctx context.Context, domain string) string {
	domain = strings.TrimSpace(domain)
	if service == nil || service.iconResolver == nil || domain == "" {
		return ""
	}
	if resolver, ok := service.iconResolver.(interface {
		ResolveDomainIconCached(context.Context, string) (string, bool)
	}); ok {
		if icon, hit := resolver.ResolveDomainIconCached(ctx, domain); hit {
			return strings.TrimSpace(icon)
		}
	}
	icon, err := service.iconResolver.ResolveDomainIcon(ctx, domain)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(icon)
}
