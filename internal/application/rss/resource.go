package rss

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"xiadown/internal/application/networkpolicy"
	domainrss "xiadown/internal/domain/rss"
)

type RemoteResourceKind string
type RemoteResourceRole string
type DiscoveryResourceKind string

const (
	RemoteResourceImage RemoteResourceKind = "image"
	RemoteResourceMedia RemoteResourceKind = "media"

	RemoteResourceRoleIcon           RemoteResourceRole = "icon"
	RemoteResourceRoleThumbnail      RemoteResourceRole = "thumbnail"
	RemoteResourceRoleContentImage   RemoteResourceRole = "content_image"
	RemoteResourceRoleMediaThumbnail RemoteResourceRole = "media_thumbnail"
	RemoteResourceRoleMedia          RemoteResourceRole = "media"

	DiscoveryResourceCategoryIcon DiscoveryResourceKind = "categories"
	DiscoveryResourceRouteIcon    DiscoveryResourceKind = "routes"
)

// RemoteResource is an internal fetch descriptor. The source URL is never
// serialized into a desktop/public resource route; callers address a persisted
// subscription/entry slot or an opaque discovery catalog identifier instead.
type RemoteResource struct {
	URL           string
	Kind          RemoteResourceKind
	MIMEType      string
	Role          RemoteResourceRole
	RefererOrigin string
}

func (service *Service) ResolveSubscriptionResource(ctx context.Context, id string) (RemoteResource, error) {
	if service == nil || service.repository == nil {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	item, err := service.repository.GetSubscription(ctx, strings.TrimSpace(id))
	if err != nil {
		return RemoteResource{}, err
	}
	if item.WorkspaceID != "" && item.WorkspaceID != domainrss.DefaultWorkspaceID {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	return validatedRemoteResource(item.IconURL, RemoteResourceImage, "", RemoteResourceRoleIcon, item.SiteURL)
}

func (service *Service) ResolveSyncSubscriptionResource(ctx context.Context, id string) (RemoteResource, error) {
	if service == nil || service.syncRepository == nil {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	item, err := service.syncRepository.GetSyncSubscription(ctx, strings.TrimSpace(id))
	if err != nil {
		return RemoteResource{}, err
	}
	return validatedRemoteResource(item.IconURL, RemoteResourceImage, "", RemoteResourceRoleIcon, item.SiteURL)
}

// ResolveDiscoveryResource turns an opaque persisted catalog identifier into
// a server-side image descriptor. The upstream URL is derived from sanitized
// catalog homepages and never appears in the desktop endpoint or renderer DTO.
func (service *Service) ResolveDiscoveryResource(
	ctx context.Context,
	kind DiscoveryResourceKind,
	id string,
) (RemoteResource, error) {
	if service == nil || service.discoveryRepository == nil {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	query := domainrss.DiscoveryQuery{Limit: 1, Sort: "popular"}
	switch kind {
	case DiscoveryResourceCategoryIcon:
		query.CategoryID = id
	case DiscoveryResourceRouteIcon:
		query.RouteID = id
	default:
		return RemoteResource{}, domainrss.ErrNotFound
	}
	route, err := service.discoveryRepository.FindDiscoveryRoute(ctx, query)
	if err != nil {
		return RemoteResource{}, err
	}
	route, ok := normalizeCachedDiscoveryRoute(route)
	if !ok || (kind == DiscoveryResourceRouteIcon && route.ID != id) {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	return validatedRemoteResource(
		discoveryFaviconURL(route.SiteURL, route.SourceURL),
		RemoteResourceImage,
		"",
		RemoteResourceRoleIcon,
		route.SiteURL,
		route.SourceURL,
	)
}

func (service *Service) ResolveEntryResource(ctx context.Context, id, slot string) (RemoteResource, error) {
	if service == nil || service.repository == nil {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	entry, err := service.repository.GetEntry(ctx, strings.TrimSpace(id))
	if err != nil {
		return RemoteResource{}, err
	}
	subscription, err := service.repository.GetSubscription(ctx, entry.SubscriptionID)
	if err != nil {
		return RemoteResource{}, err
	}
	if subscription.WorkspaceID != "" && subscription.WorkspaceID != domainrss.DefaultWorkspaceID {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	return resolveEntryResource(entry, subscription, slot)
}

func (service *Service) ResolveSyncEntryResource(ctx context.Context, id, slot string) (RemoteResource, error) {
	if service == nil || service.syncRepository == nil {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	entry, err := service.syncRepository.GetSyncEntry(ctx, strings.TrimSpace(id))
	if err != nil {
		return RemoteResource{}, err
	}
	subscription, err := service.syncRepository.GetSyncSubscription(ctx, entry.SubscriptionID)
	if err != nil {
		return RemoteResource{}, err
	}
	return resolveEntryResource(entry, subscription, slot)
}

func resolveEntryResource(entry domainrss.Entry, subscription domainrss.Subscription, slot string) (RemoteResource, error) {
	slot = strings.TrimSpace(slot)
	switch slot {
	case "thumbnail":
		return validatedRemoteResource(
			entry.ThumbnailURL, RemoteResourceImage, "", RemoteResourceRoleThumbnail,
			entry.URL, subscription.SiteURL,
		)
	}
	if index, ok := indexedResourceSlot(slot, "image-", ""); ok {
		if index >= len(entry.ImageURLs) {
			return RemoteResource{}, domainrss.ErrNotFound
		}
		return validatedRemoteResource(
			entry.ImageURLs[index], RemoteResourceImage, "", RemoteResourceRoleContentImage,
			entry.URL, subscription.SiteURL,
		)
	}
	if index, ok := indexedResourceSlot(slot, "media-", "-thumbnail"); ok {
		if index >= len(entry.Media) {
			return RemoteResource{}, domainrss.ErrNotFound
		}
		return validatedRemoteResource(
			entry.Media[index].Thumbnail, RemoteResourceImage, "", RemoteResourceRoleMediaThumbnail,
			entry.URL, subscription.SiteURL,
		)
	}
	if index, ok := indexedResourceSlot(slot, "media-", ""); ok {
		if index >= len(entry.Media) {
			return RemoteResource{}, domainrss.ErrNotFound
		}
		item := entry.Media[index]
		kind := RemoteResourceMedia
		if strings.EqualFold(strings.TrimSpace(item.Kind), "image") {
			kind = RemoteResourceImage
		} else if value := strings.ToLower(strings.TrimSpace(item.Kind)); value != "video" && value != "audio" {
			return RemoteResource{}, domainrss.ErrNotFound
		}
		role := RemoteResourceRoleMedia
		if kind == RemoteResourceImage {
			role = RemoteResourceRoleContentImage
		}
		return validatedRemoteResource(
			item.URL, kind, item.MIMEType, role,
			entry.URL, subscription.SiteURL,
		)
	}
	return RemoteResource{}, domainrss.ErrNotFound
}

func indexedResourceSlot(slot, prefix, suffix string) (int, bool) {
	if len(slot) > 64 || !strings.HasPrefix(slot, prefix) || !strings.HasSuffix(slot, suffix) {
		return 0, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(slot, prefix), suffix)
	if raw == "" || len(raw) > 6 || (len(raw) > 1 && raw[0] == '0') {
		return 0, false
	}
	for index := 0; index < len(raw); index++ {
		if raw[index] < '0' || raw[index] > '9' {
			return 0, false
		}
	}
	index, err := strconv.Atoi(raw)
	return index, err == nil && index >= 0 && index < maxSyncMediaItems
}

func validatedRemoteResource(
	rawURL string,
	kind RemoteResourceKind,
	mimeType string,
	role RemoteResourceRole,
	refererCandidates ...string,
) (RemoteResource, error) {
	parsed, err := networkpolicy.ValidatePublicHTTPURL(strings.TrimSpace(rawURL))
	if err != nil {
		return RemoteResource{}, domainrss.ErrNotFound
	}
	parsed.Fragment = ""
	if kind != RemoteResourceImage && kind != RemoteResourceMedia {
		return RemoteResource{}, fmt.Errorf("invalid RSS remote resource kind %q", kind)
	}
	return RemoteResource{
		URL: parsed.String(), Kind: kind, MIMEType: strings.TrimSpace(mimeType), Role: role,
		RefererOrigin: firstPublicHTTPOrigin(refererCandidates...),
	}, nil
}
