package service

import (
	"context"
	"path/filepath"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type catalogItemPresentation struct {
	assets       []library.CatalogItemPresentationAsset
	availability library.ItemAvailability
}

func (service *CatalogService) catalogItemPresentations(
	ctx context.Context,
	items []library.Item,
) (map[string]catalogItemPresentation, bool, error) {
	reader, ok := service.assets.(library.CatalogItemPresentationRepository)
	if !ok {
		return nil, false, nil
	}
	itemIDs := make([]string, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}
	assets, err := reader.ListCatalogItemPresentationAssets(ctx, itemIDs)
	if err != nil {
		return nil, true, err
	}
	byItemID := make(map[string][]library.CatalogItemPresentationAsset, len(items))
	for _, asset := range assets {
		byItemID[asset.ItemID] = append(byItemID[asset.ItemID], asset)
	}
	result := make(map[string]catalogItemPresentation, len(items))
	for _, item := range items {
		itemAssets := byItemID[item.ID]
		result[item.ID] = catalogItemPresentation{
			assets:       itemAssets,
			availability: catalogPresentationItemAvailability(item, itemAssets),
		}
	}
	return result, true, nil
}

func (service *CatalogService) catalogListItemDTOs(
	ctx context.Context,
	items []library.Item,
) ([]dto.CatalogItemDTO, error) {
	if len(items) == 0 {
		return []dto.CatalogItemDTO{}, nil
	}
	presentations, supported, err := service.catalogItemPresentations(ctx, items)
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogItemDTO, 0, len(items))
	if !supported {
		for _, item := range items {
			summary, summaryErr := service.catalogListItemDTOLegacy(ctx, item)
			if summaryErr != nil {
				return nil, summaryErr
			}
			result = append(result, summary)
		}
		return result, nil
	}
	for _, item := range items {
		result = append(result, catalogListItemDTOFromPresentation(
			item,
			presentations[item.ID],
		))
	}
	return result, nil
}

func (service *CatalogService) catalogListItemDTO(
	ctx context.Context,
	item library.Item,
) (dto.CatalogItemDTO, error) {
	items, err := service.catalogListItemDTOs(ctx, []library.Item{item})
	if err != nil {
		return dto.CatalogItemDTO{}, err
	}
	if len(items) == 0 {
		return dto.CatalogItemDTO{}, nil
	}
	return items[0], nil
}

func catalogListItemDTOFromPresentation(
	item library.Item,
	presentation catalogItemPresentation,
) dto.CatalogItemDTO {
	result := catalogItemDTO(item)
	result.Availability = string(presentation.availability)
	primary, primaryOK := selectCatalogPresentationPrimary(presentation.assets)
	if primaryOK {
		result.PrimaryAssetID = primary.AssetID
		result.PrimaryFileID = primary.FileID
		result.Kind = string(primary.Kind)
		if primary.Media != nil {
			result.Format = strings.TrimSpace(primary.Media.Format)
			result.DurationMs = primary.Media.DurationMs
			result.SizeBytes = primary.Media.SizeBytes
		}
	}
	for _, asset := range presentation.assets {
		if catalogPresentationAssetAvailability(asset) != library.ItemAvailabilityAvailable ||
			!catalogImageExtension(strings.ToLower(filepath.Ext(asset.LocalPath))) {
			continue
		}
		if asset.Role == library.ItemAssetRoleArtwork ||
			asset.Kind == library.FileKindThumbnail {
			result.ArtworkAssetID = asset.AssetID
			result.ArtworkFileID = asset.FileID
			break
		}
	}
	if result.ArtworkFileID == "" {
		for _, asset := range presentation.assets {
			if !asset.PreviewArtwork ||
				catalogPresentationAssetAvailability(asset) != library.ItemAvailabilityAvailable ||
				!catalogImageExtension(strings.ToLower(filepath.Ext(asset.LocalPath))) {
				continue
			}
			result.ArtworkAssetID = asset.AssetID
			result.ArtworkFileID = asset.FileID
			break
		}
	}
	return result
}

func selectCatalogPresentationPrimary(
	assets []library.CatalogItemPresentationAsset,
) (library.CatalogItemPresentationAsset, bool) {
	roles := [...]library.ItemAssetRole{
		library.ItemAssetRoleOriginal,
		library.ItemAssetRoleRepresentation,
	}
	for _, role := range roles {
		for _, asset := range assets {
			if asset.Role == role &&
				catalogPresentationAssetAvailability(asset) == library.ItemAvailabilityAvailable {
				return asset, true
			}
		}
	}
	for _, role := range roles {
		for _, asset := range assets {
			if asset.Role == role {
				return asset, true
			}
		}
	}
	return library.CatalogItemPresentationAsset{}, false
}

func catalogPresentationItemAvailability(
	item library.Item,
	assets []library.CatalogItemPresentationAsset,
) library.ItemAvailability {
	candidates := make([]library.ItemAvailability, 0, len(assets))
	for _, asset := range assets {
		if asset.Role != library.ItemAssetRoleOriginal &&
			asset.Role != library.ItemAssetRoleRepresentation {
			continue
		}
		candidates = append(candidates, catalogPresentationAssetAvailability(asset))
	}
	for _, preferred := range [...]library.ItemAvailability{
		library.ItemAvailabilityAvailable,
		library.ItemAvailabilityChecking,
		library.ItemAvailabilityOffline,
		library.ItemAvailabilityError,
		library.ItemAvailabilityMissing,
	} {
		for _, candidate := range candidates {
			if candidate == preferred {
				return preferred
			}
		}
	}
	if item.Status == library.ItemStatusMissing {
		return library.ItemAvailabilityMissing
	}
	return library.ItemAvailabilityChecking
}

func catalogPresentationAssetAvailability(
	asset library.CatalogItemPresentationAsset,
) library.ItemAvailability {
	state := strings.ToLower(strings.TrimSpace(asset.FileState.Status))
	if asset.FileState.Deleted || state == "deleted" {
		return library.ItemAvailabilityMissing
	}
	switch asset.RootStatus {
	case library.StorageRootStatusOffline:
		return library.ItemAvailabilityOffline
	case library.StorageRootStatusError:
		return library.ItemAvailabilityError
	}
	switch state {
	case "offline":
		return library.ItemAvailabilityOffline
	case "missing", "unavailable":
		return library.ItemAvailabilityMissing
	case "error", "corrupt":
		return library.ItemAvailabilityError
	}
	if lastError := strings.TrimSpace(asset.FileState.LastError); lastError != "" {
		if strings.EqualFold(lastError, missingLocalFileError) {
			return library.ItemAvailabilityMissing
		}
		return library.ItemAvailabilityError
	}
	syncState := strings.ToLower(strings.TrimSpace(asset.SyncStateStatus))
	switch strings.ToLower(strings.TrimSpace(asset.SyncEntryStatus)) {
	case "active":
		return library.ItemAvailabilityAvailable
	case "missing":
		if syncState == "queued" || syncState == "scanning" ||
			syncState == "cancelling" {
			return library.ItemAvailabilityChecking
		}
		return library.ItemAvailabilityMissing
	case "failed":
		return library.ItemAvailabilityError
	}
	if syncState == "queued" || syncState == "scanning" ||
		syncState == "cancelling" {
		return library.ItemAvailabilityChecking
	}
	// A healthy LibraryFile written by a XiaDown producer remains trustworthy
	// while its watcher registration catches up. Deletion events normally
	// settle in 1.5 seconds and then replace this fallback with a sync status.
	return library.ItemAvailabilityAvailable
}

func catalogPresentationAssetsByID(
	presentation catalogItemPresentation,
) map[string]library.CatalogItemPresentationAsset {
	result := make(map[string]library.CatalogItemPresentationAsset, len(presentation.assets))
	for _, asset := range presentation.assets {
		result[asset.AssetID] = asset
	}
	return result
}

func catalogRepresentationPhysicalAvailability(
	stored library.RepresentationAvailability,
	physical library.ItemAvailability,
) library.RepresentationAvailability {
	switch physical {
	case library.ItemAvailabilityOffline:
		return library.RepresentationAvailabilityOffline
	case library.ItemAvailabilityMissing:
		return library.RepresentationAvailabilityMissing
	case library.ItemAvailabilityError:
		return library.RepresentationAvailabilityCorrupt
	case library.ItemAvailabilityChecking:
		return library.RepresentationAvailabilityProcessing
	case library.ItemAvailabilityAvailable:
		// Explicit processing and corruption describe representation work, not
		// file presence, and must survive a healthy physical check.
		if stored == library.RepresentationAvailabilityProcessing ||
			stored == library.RepresentationAvailabilityCorrupt {
			return stored
		}
		return library.RepresentationAvailabilityAvailable
	default:
		return stored
	}
}

func (service *CatalogService) catalogFileAvailability(
	ctx context.Context,
	item library.LibraryFile,
) library.ItemAvailability {
	state := strings.ToLower(strings.TrimSpace(item.State.Status))
	if item.State.Deleted || state == "deleted" {
		return library.ItemAvailabilityMissing
	}
	if service != nil && service.roots != nil && strings.TrimSpace(item.Storage.RootID) != "" {
		root, err := service.roots.Get(ctx, item.Storage.RootID)
		if err == nil {
			switch root.Status {
			case library.StorageRootStatusOffline:
				return library.ItemAvailabilityOffline
			case library.StorageRootStatusError:
				return library.ItemAvailabilityError
			}
		}
	}
	if legacyFileUnhealthy(item) {
		if state == "offline" {
			return library.ItemAvailabilityOffline
		}
		if state == "error" || state == "corrupt" ||
			(item.State.LastError != "" && item.State.LastError != missingLocalFileError) {
			return library.ItemAvailabilityError
		}
		return library.ItemAvailabilityMissing
	}
	switch inspectLocalFilePresence(item.Storage.LocalPath) {
	case localFilePresenceAvailable:
		return library.ItemAvailabilityAvailable
	case localFilePresenceMissing:
		return library.ItemAvailabilityMissing
	default:
		return library.ItemAvailabilityError
	}
}

// catalogFileCanAttemptRead is the cheap guard for user-triggered preview
// endpoints. It checks durable state and the owning root before any filesystem
// call, so an offline removable/network volume cannot stall os.Stat or
// os.Open. The actual open remains the authoritative last-mile check.
func catalogFileCanAttemptRead(
	ctx context.Context,
	item library.LibraryFile,
	roots library.StorageRootRepository,
) bool {
	if legacyFileUnhealthy(item) || strings.TrimSpace(item.Storage.LocalPath) == "" {
		return false
	}
	rootID := strings.TrimSpace(item.Storage.RootID)
	if roots == nil || rootID == "" {
		return true
	}
	root, err := roots.Get(ctx, rootID)
	if err != nil {
		return false
	}
	return root.Status != library.StorageRootStatusOffline &&
		root.Status != library.StorageRootStatusError
}
