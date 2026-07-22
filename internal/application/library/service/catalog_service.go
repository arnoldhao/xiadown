package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/library/catalogaudit"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	defaultCatalogListLimit = 100
	maximumCatalogListLimit = 500
	catalogTombstoneTTL     = 90 * 24 * time.Hour
)

var catalogUserStateNamespace = uuid.MustParse("83c619a1-d57c-43d4-8361-a7e6aa13568f")

// CatalogService is the application boundary for the professional logical
// library. Legacy LibraryFile remains the physical asset registry and is only
// read here; lifecycle operations never remove a file from disk.
type CatalogService struct {
	catalogs     library.CatalogRepository
	items        library.CatalogItemRepository
	assets       library.ItemAssetRepository
	files        library.FileRepository
	roots        library.StorageRootRepository
	collections  library.CatalogCollectionRepository
	tags         library.CatalogTagRepository
	userStates   library.UserStateRepository
	mutations    library.CatalogMutationRepository
	professional library.CatalogProfessionalMutationRepository
	changes      library.CatalogChangeRepository
	auditor      catalogaudit.Auditor
	now          func() time.Time
	newID        func() string
}

type catalogCollectionPageRepository interface {
	ListByCatalogIDPage(context.Context, string, int, int) ([]library.Collection, error)
	ListItemsPage(context.Context, string, int, int) ([]library.CollectionItem, error)
}

type catalogTagPageRepository interface {
	ListByCatalogIDPage(context.Context, string, int, int) ([]library.Tag, error)
}

func NewCatalogService(
	catalogs library.CatalogRepository,
	items library.CatalogItemRepository,
	assets library.ItemAssetRepository,
	files library.FileRepository,
	roots library.StorageRootRepository,
	collections library.CatalogCollectionRepository,
	tags library.CatalogTagRepository,
	userStates library.UserStateRepository,
	mutations library.CatalogMutationRepository,
	auditor catalogaudit.Auditor,
	changeRepositories ...library.CatalogChangeRepository,
) *CatalogService {
	result := &CatalogService{
		catalogs: catalogs, items: items, assets: assets, files: files, roots: roots,
		collections: collections, tags: tags, userStates: userStates,
		mutations: mutations, auditor: auditor,
		now: func() time.Time { return time.Now().UTC() }, newID: uuid.NewString,
	}
	result.professional, _ = mutations.(library.CatalogProfessionalMutationRepository)
	if len(changeRepositories) > 0 {
		result.changes = changeRepositories[0]
	}
	return result
}

// SetCatalogChangeRepository keeps existing narrow constructors compatible
// while allowing bootstrap and tests to attach the durable user-activity feed.
func (service *CatalogService) SetCatalogChangeRepository(repository library.CatalogChangeRepository) {
	if service == nil {
		return
	}
	service.changes = repository
}

func (service *CatalogService) GetDefaultCatalogOverview(ctx context.Context) (dto.CatalogOverviewDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.CatalogOverviewDTO{}, err
	}
	items, err := service.items.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return dto.CatalogOverviewDTO{}, err
	}
	roots, err := service.roots.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return dto.CatalogOverviewDTO{}, err
	}
	result := dto.CatalogOverviewDTO{Catalog: catalogDTO(catalog)}
	seenFiles := make(map[string]struct{})
	for _, item := range items {
		result.Categories.All++
		switch item.Category {
		case library.ItemCategoryVideo:
			result.Categories.Video++
		case library.ItemCategoryAudio:
			result.Categories.Audio++
		case library.ItemCategoryBook:
			result.Categories.Books++
		case library.ItemCategoryImage:
			result.Categories.Images++
		case library.ItemCategoryOther:
			result.Categories.Others++
		}
		switch item.Status {
		case library.ItemStatusActive:
			result.Statuses.Active++
		case library.ItemStatusNeedsReview:
			result.Statuses.NeedsReview++
		case library.ItemStatusMissing:
			result.Statuses.Missing++
		case library.ItemStatusTrashed:
			result.Statuses.Trashed++
		}
		assets, listErr := service.assets.ListByItemID(ctx, item.ID)
		if listErr != nil {
			return dto.CatalogOverviewDTO{}, listErr
		}
		if len(assets) == 0 {
			result.Health.ItemsWithoutAssets++
		}
		result.Health.AssetLinks += len(assets)
		for _, asset := range assets {
			if _, exists := seenFiles[asset.FileID]; exists {
				continue
			}
			seenFiles[asset.FileID] = struct{}{}
			file, getErr := service.files.Get(ctx, asset.FileID)
			if errors.Is(getErr, sql.ErrNoRows) || errors.Is(getErr, library.ErrFileNotFound) {
				// A dangling asset is a Catalog integrity finding, not a local file
				// that the maintenance workflow can inspect or clear.
				continue
			}
			if getErr != nil {
				return dto.CatalogOverviewDTO{}, getErr
			}
			if file.Media != nil && file.Media.SizeBytes != nil && *file.Media.SizeBytes > 0 {
				result.TotalSizeBytes += *file.Media.SizeBytes
			}
			if catalogFileNeedsMissingMaintenance(file) {
				result.Health.UnavailableAssetFiles++
			}
			if legacyFileUnhealthy(file) {
				result.Health.LegacyFilesWithErrors++
			}
		}
	}
	for _, root := range roots {
		switch root.Status {
		case library.StorageRootStatusOffline:
			result.Health.OfflineStorageRoots++
		case library.StorageRootStatusReadOnly:
			result.Health.ReadOnlyStorageRoots++
		case library.StorageRootStatusError:
			result.Health.StorageRootsWithErrors++
		}
	}
	return result, nil
}

func (service *CatalogService) ListCatalogItems(ctx context.Context, request dto.ListCatalogItemsRequest) (dto.ListCatalogItemsResponse, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.ListCatalogItemsResponse{}, err
	}
	category := strings.ToLower(strings.TrimSpace(request.Category))
	if category == "books" {
		category = string(library.ItemCategoryBook)
	} else if category == "images" {
		category = string(library.ItemCategoryImage)
	} else if category == "others" {
		category = string(library.ItemCategoryOther)
	}
	if category != "" && category != "all" && !validCatalogCategory(category) {
		return dto.ListCatalogItemsResponse{}, library.ErrInvalidCatalogItem
	}
	status := strings.ToLower(strings.TrimSpace(request.Status))
	if status != "" && status != "all" && !validCatalogStatus(status) {
		return dto.ListCatalogItemsResponse{}, library.ErrInvalidCatalogItem
	}
	limit := request.Limit
	if limit <= 0 {
		limit = defaultCatalogListLimit
	}
	if limit > maximumCatalogListLimit || request.Offset < 0 {
		return dto.ListCatalogItemsResponse{}, fmt.Errorf("invalid catalog pagination")
	}
	items, err := service.items.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return dto.ListCatalogItemsResponse{}, err
	}
	query := strings.ToLower(strings.TrimSpace(request.Query))
	filtered := make([]library.Item, 0, len(items))
	for _, item := range items {
		if category != "" && category != "all" && string(item.Category) != category {
			continue
		}
		if request.ExcludeTrashed && item.Status == library.ItemStatusTrashed {
			continue
		}
		if status != "" && status != "all" && string(item.Status) != status {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Title), query) &&
			!strings.Contains(strings.ToLower(item.Description), query) {
			continue
		}
		filtered = append(filtered, item)
	}
	if err := sortCatalogItems(filtered, request.Sort); err != nil {
		return dto.ListCatalogItemsResponse{}, err
	}
	result := dto.ListCatalogItemsResponse{Total: len(filtered), Limit: limit, Offset: request.Offset, Items: []dto.CatalogItemDTO{}}
	if request.Offset >= len(filtered) {
		return result, nil
	}
	end := request.Offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	result.Items = make([]dto.CatalogItemDTO, 0, end-request.Offset)
	for _, item := range filtered[request.Offset:end] {
		summary, summaryErr := service.catalogListItemDTO(ctx, item)
		if summaryErr != nil {
			return dto.ListCatalogItemsResponse{}, summaryErr
		}
		result.Items = append(result.Items, summary)
	}
	return result, nil
}

// ListCatalogSnapshotItems returns a path-free, presentation-ready keyset page
// for the public Library snapshot transport. SQLite performs the narrow
// `id > afterID ORDER BY id LIMIT` query; the in-memory fallback keeps custom
// repository implementations compatible while preserving the same contract.
func (service *CatalogService) ListCatalogSnapshotItems(
	ctx context.Context,
	catalogID string,
	afterID string,
	limit int,
) ([]dto.CatalogItemDTO, error) {
	if limit <= 0 || limit > maximumCatalogListLimit+1 {
		return nil, fmt.Errorf("invalid Catalog snapshot pagination")
	}
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return nil, err
	}
	catalogID = strings.TrimSpace(catalogID)
	afterID = strings.TrimSpace(afterID)
	if catalog.ID != catalogID {
		return nil, sql.ErrNoRows
	}

	var items []library.Item
	if pager, ok := service.items.(library.CatalogItemSnapshotRepository); ok {
		items, err = pager.ListSnapshotPageByCatalogID(ctx, catalogID, afterID, limit)
	} else {
		items, err = service.items.ListByCatalogID(ctx, catalogID)
		if err == nil {
			items = catalogSnapshotFallbackPage(items, afterID, limit)
		}
	}
	if err != nil {
		return nil, err
	}

	result := make([]dto.CatalogItemDTO, 0, len(items))
	previousID := afterID
	for _, item := range items {
		if item.CatalogID != catalogID || item.Status == library.ItemStatusTrashed || item.TrashedAt != nil ||
			strings.Compare(item.ID, previousID) <= 0 {
			return nil, errors.New("invalid Catalog snapshot keyset page")
		}
		summary, summaryErr := service.catalogListItemDTO(ctx, item)
		if summaryErr != nil {
			return nil, summaryErr
		}
		result = append(result, summary)
		previousID = item.ID
	}
	return result, nil
}

func catalogSnapshotFallbackPage(items []library.Item, afterID string, limit int) []library.Item {
	filtered := make([]library.Item, 0, min(limit, len(items)))
	for _, item := range items {
		if item.Status == library.ItemStatusTrashed || item.TrashedAt != nil || strings.Compare(item.ID, afterID) <= 0 {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(left, right int) bool { return filtered[left].ID < filtered[right].ID })
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// catalogListItemDTO enriches the logical item with a small, path-free summary
// of its primary and artwork assets. Opaque asset IDs let remote clients request
// content through the authenticated asset route; opaque file IDs let the
// desktop reuse its already-loaded LibraryFile records without leaking a local
// path into the public list contract.
func (service *CatalogService) catalogListItemDTO(ctx context.Context, item library.Item) (dto.CatalogItemDTO, error) {
	result := catalogItemDTO(item)
	assets, err := service.assets.ListByItemID(ctx, item.ID)
	if err != nil {
		return dto.CatalogItemDTO{}, err
	}
	ordered := make([]library.ItemAsset, 0, len(assets))
	for _, asset := range assets {
		if asset.Role == library.ItemAssetRoleOriginal {
			ordered = append(ordered, asset)
		}
	}
	for _, asset := range assets {
		if asset.Role == library.ItemAssetRoleRepresentation {
			ordered = append(ordered, asset)
		}
	}
	for _, asset := range assets {
		if asset.Role != library.ItemAssetRoleOriginal && asset.Role != library.ItemAssetRoleRepresentation {
			ordered = append(ordered, asset)
		}
	}
	filesByAssetID := make(map[string]library.LibraryFile, len(ordered))
	hasUnclassifiedImageAsset := false
	for _, asset := range ordered {
		file, fileErr := service.files.Get(ctx, asset.FileID)
		if errors.Is(fileErr, library.ErrFileNotFound) || errors.Is(fileErr, sql.ErrNoRows) {
			continue
		}
		if fileErr != nil {
			return dto.CatalogItemDTO{}, fileErr
		}
		filesByAssetID[asset.ID] = file
		previewImage := file.Kind == library.FileKindThumbnail ||
			catalogImageExtension(strings.ToLower(filepath.Ext(file.Storage.LocalPath)))
		if asset.Role != library.ItemAssetRoleOriginal && previewImage {
			hasUnclassifiedImageAsset = true
		}
		if asset.Role == library.ItemAssetRoleArtwork && previewImage &&
			result.ArtworkFileID == "" && catalogFileAvailable(file) {
			result.ArtworkAssetID = asset.ID
			result.ArtworkFileID = file.ID
		}
	}
	if primaryAsset, primaryFile, ok := selectCatalogPrimaryAsset(ordered, filesByAssetID); ok {
		result.PrimaryAssetID = primaryAsset.ID
		result.PrimaryFileID = primaryFile.ID
		result.Kind = string(primaryFile.Kind)
		if primaryFile.Media != nil {
			result.Format = strings.TrimSpace(primaryFile.Media.Format)
			result.DurationMs = primaryFile.Media.DurationMs
			result.SizeBytes = primaryFile.Media.SizeBytes
		}
	}
	// Most downloaded items already carry an explicit artwork role. Only consult
	// the representation table when that fast path did not find a cover, avoiding
	// one additional SQLite query for every row in a large Library page.
	if result.ArtworkFileID == "" && hasUnclassifiedImageAsset && service.professional != nil {
		representations, listErr := service.professional.ListRepresentationsByItemID(ctx, item.ID)
		if listErr != nil {
			return dto.CatalogItemDTO{}, listErr
		}
		for _, representation := range representations {
			if representation.Availability != library.RepresentationAvailabilityAvailable ||
				(representation.Kind != library.RepresentationKindArtwork &&
					representation.Kind != library.RepresentationKindThumbnail &&
					representation.Purpose != library.RepresentationPurposeArtwork) {
				continue
			}
			file, exists := filesByAssetID[representation.AssetID]
			if !exists || (file.Kind != library.FileKindThumbnail &&
				!catalogImageExtension(strings.ToLower(filepath.Ext(file.Storage.LocalPath)))) ||
				!catalogFileAvailable(file) {
				continue
			}
			result.ArtworkAssetID = representation.AssetID
			result.ArtworkFileID = file.ID
			break
		}
	}
	return result, nil
}

func selectCatalogPrimaryAsset(
	assets []library.ItemAsset,
	filesByAssetID map[string]library.LibraryFile,
) (library.ItemAsset, library.LibraryFile, bool) {
	roles := []library.ItemAssetRole{
		library.ItemAssetRoleOriginal,
		library.ItemAssetRoleRepresentation,
	}
	for _, role := range roles {
		for _, asset := range assets {
			file, exists := filesByAssetID[asset.ID]
			if !exists || asset.Role != role || !catalogFileAvailable(file) {
				continue
			}
			return asset, file, true
		}
	}
	// Missing and trashed items still expose their best known media reference
	// for diagnostics. Artwork and attachments never become the primary merely
	// because both media candidates are unavailable.
	for _, role := range roles {
		for _, asset := range assets {
			file, exists := filesByAssetID[asset.ID]
			if exists && asset.Role == role {
				return asset, file, true
			}
		}
	}
	return library.ItemAsset{}, library.LibraryFile{}, false
}

func (service *CatalogService) GetCatalogItem(ctx context.Context, request dto.GetCatalogItemRequest) (dto.CatalogItemDetailDTO, error) {
	catalog, item, err := service.defaultCatalogItem(ctx, request.ID)
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	result := dto.CatalogItemDetailDTO{
		Item: catalogItemDTO(item), Assets: []dto.CatalogItemAssetDTO{},
		Representations: []dto.CatalogRepresentationDTO{}, Metadata: []dto.CatalogMetadataEntryDTO{},
		Tags: []dto.CatalogTagDTO{},
	}
	assets, err := service.assets.ListByItemID(ctx, item.ID)
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	roots, err := service.roots.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	assetRootIDs := make(map[string]string, len(assets))
	for _, asset := range assets {
		assetDTO := catalogItemAssetDTO(asset)
		file, getErr := service.files.Get(ctx, asset.FileID)
		if getErr == nil {
			fileDTO := toLibraryFileDTO(file)
			assetDTO.File = &fileDTO
			assetDTO.FileAvailable = catalogFileAvailable(file)
			assetDTO.StorageRootID = catalogStorageRootIDForPath(roots, file.Storage.LocalPath)
			assetRootIDs[asset.ID] = assetDTO.StorageRootID
		} else if !errors.Is(getErr, sql.ErrNoRows) {
			return dto.CatalogItemDetailDTO{}, getErr
		}
		result.Assets = append(result.Assets, assetDTO)
	}
	if service.professional != nil {
		representations, listErr := service.professional.ListRepresentationsByItemID(ctx, item.ID)
		if listErr != nil {
			return dto.CatalogItemDetailDTO{}, listErr
		}
		for _, representation := range representations {
			representationDTO := catalogRepresentationDTO(representation)
			representationDTO.StorageRootID = assetRootIDs[representation.AssetID]
			result.Representations = append(result.Representations, representationDTO)
		}
		metadata, listErr := service.professional.ListMetadataEntriesByItemID(ctx, item.ID)
		if listErr != nil {
			return dto.CatalogItemDetailDTO{}, listErr
		}
		for _, entry := range metadata {
			result.Metadata = append(result.Metadata, catalogMetadataEntryDTO(entry))
		}
	}
	result.Tags, err = service.itemTagDTOs(ctx, catalog.ID, item.ID)
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	userID := strings.TrimSpace(request.UserID)
	if userID != "" {
		state, getErr := service.userStates.Get(ctx, catalog.ID, item.ID, userID)
		if getErr == nil {
			stateDTO := catalogUserStateDTO(state)
			result.UserState = &stateDTO
		} else if !errors.Is(getErr, sql.ErrNoRows) {
			return dto.CatalogItemDetailDTO{}, getErr
		}
	}
	return result, nil
}

func (service *CatalogService) ListCatalogRepresentations(
	ctx context.Context,
	request dto.ListCatalogRepresentationsRequest,
) ([]dto.CatalogRepresentationDTO, error) {
	professional, err := service.professionalRepository()
	if err != nil {
		return nil, err
	}
	_, item, err := service.defaultCatalogItem(ctx, request.ItemID)
	if err != nil {
		return nil, err
	}
	items, err := professional.ListRepresentationsByItemID(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	rootIDs, err := service.catalogStorageRootAssignments(ctx, item.CatalogID, item.ID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogRepresentationDTO, 0, len(items))
	for _, representation := range items {
		itemDTO := catalogRepresentationDTO(representation)
		itemDTO.StorageRootID = rootIDs[representation.AssetID]
		result = append(result, itemDTO)
	}
	return result, nil
}

func (service *CatalogService) SaveCatalogRepresentation(
	ctx context.Context,
	request dto.SaveCatalogRepresentationRequest,
) (dto.CatalogRepresentationDTO, error) {
	professional, err := service.professionalRepository()
	if err != nil {
		return dto.CatalogRepresentationDTO{}, err
	}
	id := strings.TrimSpace(request.ID)
	itemID := strings.TrimSpace(request.ItemID)
	assetID := strings.TrimSpace(request.AssetID)
	createdAt := service.timestamp()
	revision := int64(1)
	if id == "" {
		if request.ExpectedRevision != 0 || itemID == "" || assetID == "" {
			return dto.CatalogRepresentationDTO{}, library.ErrInvalidRepresentation
		}
		id = service.newID()
	} else {
		current, getErr := professional.GetRepresentation(ctx, id)
		if getErr != nil {
			return dto.CatalogRepresentationDTO{}, getErr
		}
		if request.ExpectedRevision != current.Revision {
			return dto.CatalogRepresentationDTO{}, library.ErrCatalogRevisionConflict
		}
		if itemID == "" {
			itemID = current.ItemID
		}
		if assetID == "" {
			assetID = current.AssetID
		}
		if itemID != current.ItemID || assetID != current.AssetID {
			return dto.CatalogRepresentationDTO{}, library.ErrInvalidRepresentation
		}
		createdAt = current.CreatedAt
		revision = current.Revision + 1
	}
	catalog, item, err := service.defaultCatalogItem(ctx, itemID)
	if err != nil {
		return dto.CatalogRepresentationDTO{}, err
	}
	asset, err := service.assets.Get(ctx, assetID)
	if err != nil {
		return dto.CatalogRepresentationDTO{}, err
	}
	if asset.ItemID != item.ID {
		return dto.CatalogRepresentationDTO{}, sql.ErrNoRows
	}
	updatedAt := service.timestampAtLeast(createdAt)
	representation, err := library.NewRepresentation(library.RepresentationParams{
		ID: id, CatalogID: catalog.ID, ItemID: item.ID, AssetID: asset.ID,
		Kind: request.Kind, Purpose: request.Purpose, MediaType: request.MediaType,
		Container: request.Container, Codec: request.Codec, Width: request.Width, Height: request.Height,
		DurationMs: request.DurationMs, BitrateBps: request.BitrateBps, Language: request.Language,
		ChecksumAlgorithm: request.ChecksumAlgorithm, Checksum: request.Checksum,
		SizeBytes: request.SizeBytes, Availability: request.Availability, Revision: revision,
		CreatedAt: &createdAt, UpdatedAt: &updatedAt,
	})
	if err != nil {
		return dto.CatalogRepresentationDTO{}, err
	}
	if err := professional.SaveRepresentationMutation(ctx, representation, request.ExpectedRevision, request.ActorID); err != nil {
		return dto.CatalogRepresentationDTO{}, err
	}
	return catalogRepresentationDTO(representation), nil
}

func (service *CatalogService) ListCatalogMetadataEntries(
	ctx context.Context,
	request dto.ListCatalogMetadataEntriesRequest,
) ([]dto.CatalogMetadataEntryDTO, error) {
	professional, err := service.professionalRepository()
	if err != nil {
		return nil, err
	}
	_, item, err := service.defaultCatalogItem(ctx, request.ItemID)
	if err != nil {
		return nil, err
	}
	var items []library.MetadataEntry
	representationID := strings.TrimSpace(request.RepresentationID)
	if representationID == "" {
		items, err = professional.ListMetadataEntriesByItemID(ctx, item.ID)
	} else {
		representation, getErr := professional.GetRepresentation(ctx, representationID)
		if getErr != nil {
			return nil, getErr
		}
		if representation.ItemID != item.ID {
			return nil, sql.ErrNoRows
		}
		items, err = professional.ListMetadataEntriesByRepresentationID(ctx, representation.ID)
	}
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogMetadataEntryDTO, 0, len(items))
	for _, entry := range items {
		result = append(result, catalogMetadataEntryDTO(entry))
	}
	return result, nil
}

func (service *CatalogService) SaveCatalogMetadataEntry(
	ctx context.Context,
	request dto.SaveCatalogMetadataEntryRequest,
) (dto.CatalogMetadataEntryDTO, error) {
	professional, err := service.professionalRepository()
	if err != nil {
		return dto.CatalogMetadataEntryDTO{}, err
	}
	id := strings.TrimSpace(request.ID)
	itemID := strings.TrimSpace(request.ItemID)
	representationID := strings.TrimSpace(request.RepresentationID)
	createdAt := service.timestamp()
	revision := int64(1)
	if id == "" {
		if request.ExpectedRevision != 0 || itemID == "" {
			return dto.CatalogMetadataEntryDTO{}, library.ErrInvalidMetadataEntry
		}
		id = service.newID()
	} else {
		current, getErr := professional.GetMetadataEntry(ctx, id)
		if getErr != nil {
			return dto.CatalogMetadataEntryDTO{}, getErr
		}
		if request.ExpectedRevision != current.Revision {
			return dto.CatalogMetadataEntryDTO{}, library.ErrCatalogRevisionConflict
		}
		if itemID == "" {
			itemID = current.ItemID
		}
		if representationID == "" && current.RepresentationID != "" {
			representationID = current.RepresentationID
		}
		if itemID != current.ItemID || representationID != current.RepresentationID {
			return dto.CatalogMetadataEntryDTO{}, library.ErrInvalidMetadataEntry
		}
		createdAt = current.CreatedAt
		revision = current.Revision + 1
	}
	catalog, item, err := service.defaultCatalogItem(ctx, itemID)
	if err != nil {
		return dto.CatalogMetadataEntryDTO{}, err
	}
	if representationID != "" {
		representation, getErr := professional.GetRepresentation(ctx, representationID)
		if getErr != nil {
			return dto.CatalogMetadataEntryDTO{}, getErr
		}
		if representation.CatalogID != catalog.ID || representation.ItemID != item.ID {
			return dto.CatalogMetadataEntryDTO{}, sql.ErrNoRows
		}
	}
	updatedAt := service.timestampAtLeast(createdAt)
	entry, err := library.NewMetadataEntry(library.MetadataEntryParams{
		ID: id, CatalogID: catalog.ID, ItemID: item.ID, RepresentationID: representationID,
		Namespace: request.Namespace, Key: request.Key, ValueType: request.ValueType, ValueJSON: request.ValueJSON,
		Language: request.Language, Position: request.Position, Source: request.Source, Provenance: request.Provenance,
		Confidence: request.Confidence, Locked: request.Locked, Revision: revision,
		CreatedAt: &createdAt, UpdatedAt: &updatedAt,
	})
	if err != nil {
		return dto.CatalogMetadataEntryDTO{}, err
	}
	if err := professional.SaveMetadataEntryMutation(ctx, entry, request.ExpectedRevision, request.ActorID); err != nil {
		return dto.CatalogMetadataEntryDTO{}, err
	}
	return catalogMetadataEntryDTO(entry), nil
}

func (service *CatalogService) UpdateCatalogItem(ctx context.Context, request dto.UpdateCatalogItemRequest) (dto.CatalogItemDetailDTO, error) {
	_, current, err := service.defaultCatalogItem(ctx, request.ID)
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	if request.ExpectedRevision <= 0 || current.Revision != request.ExpectedRevision ||
		(request.Title == nil && request.Description == nil && request.Category == nil) {
		if current.Revision != request.ExpectedRevision {
			return dto.CatalogItemDetailDTO{}, library.ErrCatalogRevisionConflict
		}
		return dto.CatalogItemDetailDTO{}, library.ErrInvalidCatalogItem
	}
	if current.Status == library.ItemStatusTrashed {
		return dto.CatalogItemDetailDTO{}, library.ErrInvalidCatalogItem
	}
	title, sortTitle, description, category := current.Title, current.SortTitle, current.Description, current.Category
	if request.Title != nil {
		title = *request.Title
		sortTitle = title
	}
	if request.Description != nil {
		description = *request.Description
	}
	if request.Category != nil {
		category = library.ItemCategory(strings.ToLower(strings.TrimSpace(*request.Category)))
	}
	updatedAt := service.timestampAtLeast(current.CreatedAt)
	updated, err := library.NewItem(library.ItemParams{
		ID: current.ID, CatalogID: current.CatalogID, Category: string(category), Status: string(current.Status),
		Title: title, SortTitle: sortTitle, Description: description, Revision: current.Revision + 1,
		CreatedAt: &current.CreatedAt, UpdatedAt: &updatedAt,
	})
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	if err := service.mutations.SaveItemMutation(ctx, updated, current.Revision, library.CatalogChangeUpsert, request.ActorID, nil); err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	return service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: updated.ID, UserID: request.UserID})
}

// SyncListenLocalTrackMetadata projects an embedded-tag edit onto the logical
// Catalog item backed by the same legacy file. This keeps server-side Library
// search, list rows, previews, and remote catalog consumers on the same title
// and artist as Music without changing the physical filename.
func (service *CatalogService) SyncListenLocalTrackMetadata(
	ctx context.Context,
	file library.LibraryFile,
	metadata dto.UpdateListenLocalTrackMetadataRequest,
) error {
	if service == nil || service.items == nil || service.assets == nil || service.professional == nil {
		return errors.New("catalog metadata synchronization is not configured")
	}
	itemID, err := service.resolveListenLocalCatalogItemID(ctx, file.ID)
	if err != nil {
		return err
	}
	title := firstNonEmpty(file.Metadata.Title, file.DisplayName, file.Name, file.ID)

	for attempt := 0; attempt < 2; attempt++ {
		current, err := service.items.Get(ctx, itemID)
		if err != nil {
			return fmt.Errorf("get projected Catalog item for file %q: %w", file.ID, err)
		}
		if current.Status == library.ItemStatusTrashed {
			return nil
		}
		var updatedItem *library.Item
		if current.Title != title {
			updatedAt := service.timestampAtLeast(current.UpdatedAt)
			updated, buildErr := library.NewItem(library.ItemParams{
				ID: current.ID, CatalogID: current.CatalogID,
				Category: string(current.Category), Status: string(current.Status),
				Title: title, SortTitle: title, Description: current.Description,
				Revision: current.Revision + 1, TrashedAt: current.TrashedAt,
				CreatedAt: &current.CreatedAt, UpdatedAt: &updatedAt,
			})
			if buildErr != nil {
				return buildErr
			}
			updatedItem = &updated
		}

		currentEntries, err := service.professional.ListMetadataEntriesByItemID(ctx, itemID)
		if err != nil {
			return err
		}
		upserts, deletes, err := service.planListenLocalCatalogMetadataEntries(itemID, metadata, currentEntries)
		if err != nil {
			return err
		}
		legacyUpserts, err := service.planListenLocalLegacyMetadataEntries(ctx, itemID, file, currentEntries)
		if err != nil {
			return err
		}
		upserts = append(upserts, legacyUpserts...)
		if updatedItem == nil && len(upserts) == 0 && len(deletes) == 0 {
			return nil
		}
		expectedItemRevision := int64(0)
		if updatedItem != nil {
			expectedItemRevision = current.Revision
		}
		err = service.professional.SaveItemMetadataBatchMutation(
			ctx,
			updatedItem,
			expectedItemRevision,
			upserts,
			deletes,
			"local-music-metadata",
		)
		if errors.Is(err, library.ErrCatalogRevisionConflict) && attempt == 0 {
			continue
		}
		if err != nil {
			return err
		}
		return nil
	}
	return library.ErrCatalogRevisionConflict
}

func (service *CatalogService) resolveListenLocalCatalogItemID(ctx context.Context, fileID string) (string, error) {
	fileID = strings.TrimSpace(fileID)
	directID := deterministicCatalogID("item", DefaultLibraryCatalogID(), fileID)
	if _, err := service.items.Get(ctx, directID); err == nil {
		return directID, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("get projected Catalog item for file %q: %w", fileID, err)
	}

	// Transcodes with a valid root are representations of the root Item, not
	// standalone deterministic Items. Metadata editing still surfaces those
	// audio-only files, so resolve the owning Item through its asset link.
	items, err := service.items.ListByCatalogID(ctx, DefaultLibraryCatalogID())
	if err != nil {
		return "", err
	}
	resolvedID := ""
	for _, item := range items {
		assets, listErr := service.assets.ListByItemID(ctx, item.ID)
		if listErr != nil {
			return "", listErr
		}
		for _, asset := range assets {
			if asset.FileID != fileID {
				continue
			}
			if resolvedID != "" && resolvedID != item.ID {
				return "", fmt.Errorf("file %q is linked to multiple Catalog items", fileID)
			}
			resolvedID = item.ID
		}
	}
	if resolvedID == "" {
		return "", fmt.Errorf("get projected Catalog item for file %q: %w", fileID, sql.ErrNoRows)
	}
	return resolvedID, nil
}

type listenLocalCatalogMetadataValue struct {
	key       string
	valueType library.MetadataValueType
	value     any
	present   bool
}

func (service *CatalogService) planListenLocalCatalogMetadataEntries(
	itemID string,
	metadata dto.UpdateListenLocalTrackMetadataRequest,
	currentEntries []library.MetadataEntry,
) ([]library.MetadataEntry, []library.MetadataEntry, error) {
	values := []listenLocalCatalogMetadataValue{
		{key: "title", valueType: library.MetadataValueString, value: metadata.Title, present: metadata.Title != ""},
		{key: "artist", valueType: library.MetadataValueString, value: metadata.Author, present: metadata.Author != ""},
		{key: "album", valueType: library.MetadataValueString, value: metadata.Album, present: metadata.Album != ""},
		{key: "album_artist", valueType: library.MetadataValueString, value: metadata.AlbumArtist, present: metadata.AlbumArtist != ""},
		{key: "genre", valueType: library.MetadataValueString, value: metadata.Genre, present: metadata.Genre != ""},
		{key: "track_number", valueType: library.MetadataValueInteger, value: metadata.TrackNumber, present: metadata.TrackNumber > 0},
		{key: "disc_number", valueType: library.MetadataValueInteger, value: metadata.DiscNumber, present: metadata.DiscNumber > 0},
		{key: "year", valueType: library.MetadataValueInteger, value: metadata.Year, present: metadata.Year > 0},
	}
	currentByID := make(map[string]library.MetadataEntry, len(currentEntries))
	for _, entry := range currentEntries {
		currentByID[entry.ID] = entry
	}
	upserts := make([]library.MetadataEntry, 0, len(values))
	deletes := make([]library.MetadataEntry, 0, len(values))
	for _, value := range values {
		upsert, remove, changed, err := service.planListenLocalCatalogMetadataEntry(
			itemID,
			value,
			currentByID,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("plan music.%s: %w", value.key, err)
		}
		if !changed {
			continue
		}
		if remove.ID != "" {
			deletes = append(deletes, remove)
		} else {
			upserts = append(upserts, upsert)
		}
	}
	return upserts, deletes, nil
}

func (service *CatalogService) planListenLocalCatalogMetadataEntry(
	itemID string,
	value listenLocalCatalogMetadataValue,
	currentByID map[string]library.MetadataEntry,
) (library.MetadataEntry, library.MetadataEntry, bool, error) {
	const namespace = "music"
	const provenance = "music.local-metadata-editor"
	// Position zero is the default used by Library's manual Metadata editor.
	// Keep the embedded-tag projection in its own deterministic slot so a user
	// entry with the same namespace/key remains independent and is never
	// overwritten or removed when Music clears a tag.
	const position = 1000
	entryID := deterministicCatalogID("metadata-entry", itemID, namespace, value.key, provenance)
	current, exists := currentByID[entryID]
	if !value.present {
		if !exists {
			return library.MetadataEntry{}, library.MetadataEntry{}, false, nil
		}
		current.UpdatedAt = service.timestampAtLeast(current.UpdatedAt)
		return library.MetadataEntry{}, current, true, nil
	}

	valueJSON, err := json.Marshal(value.value)
	if err != nil {
		return library.MetadataEntry{}, library.MetadataEntry{}, false, err
	}
	createdAt := service.timestamp()
	expectedRevision := int64(0)
	if exists {
		if current.ValueType == value.valueType && string(current.Value) == string(valueJSON) {
			return library.MetadataEntry{}, library.MetadataEntry{}, false, nil
		}
		createdAt = current.CreatedAt
		expectedRevision = current.Revision
	}
	updatedAt := service.timestampAtLeast(createdAt)
	entry, err := library.NewMetadataEntry(library.MetadataEntryParams{
		ID: entryID, CatalogID: DefaultLibraryCatalogID(), ItemID: itemID,
		Namespace: namespace, Key: value.key, ValueType: string(value.valueType),
		ValueJSON: string(valueJSON), Source: string(library.MetadataSourceEmbedded),
		Provenance: provenance, Position: position, Revision: expectedRevision + 1,
		CreatedAt: &createdAt, UpdatedAt: &updatedAt,
	})
	return entry, library.MetadataEntry{}, err == nil, err
}

func (service *CatalogService) planListenLocalLegacyMetadataEntries(
	ctx context.Context,
	itemID string,
	file library.LibraryFile,
	currentEntries []library.MetadataEntry,
) ([]library.MetadataEntry, error) {
	const provenance = "legacy.library_files.metadata_json"
	assets, err := service.assets.ListByItemID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	fileAssetIDs := make(map[string]struct{})
	for _, asset := range assets {
		if asset.FileID == file.ID {
			fileAssetIDs[asset.ID] = struct{}{}
		}
	}
	if len(fileAssetIDs) == 0 {
		return nil, nil
	}
	representations, err := service.professional.ListRepresentationsByItemID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	representationIDs := make(map[string]struct{}, len(fileAssetIDs))
	for _, representation := range representations {
		if _, matchesFile := fileAssetIDs[representation.AssetID]; matchesFile {
			representationIDs[representation.ID] = struct{}{}
		}
	}
	if len(representationIDs) == 0 {
		return nil, nil
	}
	valueJSON, err := json.Marshal(file.Metadata)
	if err != nil {
		return nil, err
	}
	result := make([]library.MetadataEntry, 0, 1)
	for _, current := range currentEntries {
		if current.Source != library.MetadataSourceMigration || current.Provenance != provenance {
			continue
		}
		if _, matchesFile := representationIDs[current.RepresentationID]; !matchesFile {
			continue
		}
		if current.ValueType == library.MetadataValueJSON && string(current.Value) == string(valueJSON) {
			continue
		}
		updatedAt := service.timestampAtLeast(current.UpdatedAt)
		updated, buildErr := library.NewMetadataEntry(library.MetadataEntryParams{
			ID: current.ID, CatalogID: current.CatalogID, ItemID: current.ItemID,
			RepresentationID: current.RepresentationID,
			Namespace:        current.Namespace, Key: current.Key,
			ValueType: string(library.MetadataValueJSON), ValueJSON: string(valueJSON),
			Language: current.Language, Position: current.Position,
			Source: string(current.Source), Provenance: current.Provenance,
			Confidence: current.Confidence, Locked: current.Locked, Revision: current.Revision + 1,
			CreatedAt: &current.CreatedAt, UpdatedAt: &updatedAt,
		})
		if buildErr != nil {
			return nil, buildErr
		}
		result = append(result, updated)
	}
	return result, nil
}

func (service *CatalogService) TrashCatalogItem(ctx context.Context, request dto.CatalogItemLifecycleRequest) (dto.CatalogItemDetailDTO, error) {
	_, current, err := service.defaultCatalogItem(ctx, request.ID)
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	if current.Revision != request.ExpectedRevision || request.ExpectedRevision <= 0 {
		return dto.CatalogItemDetailDTO{}, library.ErrCatalogRevisionConflict
	}
	if current.Status == library.ItemStatusTrashed {
		return service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: current.ID, UserID: request.UserID})
	}
	trashedAt := service.timestampAtLeast(current.CreatedAt)
	trashed, err := library.NewItem(library.ItemParams{
		ID: current.ID, CatalogID: current.CatalogID, Category: string(current.Category), Status: string(library.ItemStatusTrashed),
		Title: current.Title, SortTitle: current.SortTitle, Description: current.Description, Revision: current.Revision + 1,
		TrashedAt: &trashedAt, CreatedAt: &current.CreatedAt, UpdatedAt: &trashedAt,
	})
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	expiresAt := trashedAt.Add(catalogTombstoneTTL)
	if err := service.mutations.SaveItemMutation(ctx, trashed, current.Revision, library.CatalogChangeDelete, request.ActorID, &expiresAt); err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	return service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: trashed.ID, UserID: request.UserID})
}

func (service *CatalogService) RestoreCatalogItem(ctx context.Context, request dto.CatalogItemLifecycleRequest) (dto.CatalogItemDetailDTO, error) {
	_, current, err := service.defaultCatalogItem(ctx, request.ID)
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	if current.Revision != request.ExpectedRevision || request.ExpectedRevision <= 0 {
		return dto.CatalogItemDetailDTO{}, library.ErrCatalogRevisionConflict
	}
	if current.Status != library.ItemStatusTrashed {
		return service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: current.ID, UserID: request.UserID})
	}
	status, err := service.restoredItemStatus(ctx, current.ID)
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	updatedAt := service.timestampAtLeast(current.CreatedAt)
	restored, err := library.NewItem(library.ItemParams{
		ID: current.ID, CatalogID: current.CatalogID, Category: string(current.Category), Status: string(status),
		Title: current.Title, SortTitle: current.SortTitle, Description: current.Description, Revision: current.Revision + 1,
		CreatedAt: &current.CreatedAt, UpdatedAt: &updatedAt,
	})
	if err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	if err := service.mutations.SaveItemMutation(ctx, restored, current.Revision, library.CatalogChangeUpsert, request.ActorID, nil); err != nil {
		return dto.CatalogItemDetailDTO{}, err
	}
	return service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: restored.ID, UserID: request.UserID})
}

func (service *CatalogService) ListCatalogCollections(ctx context.Context) ([]dto.CatalogCollectionDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.collections.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogCollectionDTO, 0, len(items))
	for _, item := range items {
		members, listErr := service.collections.ListItems(ctx, item.ID)
		if listErr != nil {
			return nil, listErr
		}
		result = append(result, catalogCollectionDTO(item, members))
	}
	return result, nil
}

// ListCatalogCollectionsPage is the bounded remote-read path. It prevents a
// small public API response from first materializing every collection and
// every membership row in a professional-size Catalog.
func (service *CatalogService) ListCatalogCollectionsPage(
	ctx context.Context,
	limit int,
	offset int,
	memberLimit int,
) ([]dto.CatalogCollectionDTO, error) {
	if limit < 1 || limit > maximumCatalogListLimit || offset < 0 || memberLimit < 1 || memberLimit > maximumCatalogListLimit {
		return nil, errors.New("invalid Catalog collection pagination")
	}
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return nil, err
	}
	var items []library.Collection
	if pager, ok := service.collections.(catalogCollectionPageRepository); ok {
		items, err = pager.ListByCatalogIDPage(ctx, catalog.ID, limit, offset)
	} else {
		items, err = service.collections.ListByCatalogID(ctx, catalog.ID)
		if err == nil {
			if offset >= len(items) {
				items = nil
			} else {
				items = items[offset:min(offset+limit, len(items))]
			}
		}
	}
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogCollectionDTO, 0, len(items))
	for _, item := range items {
		var members []library.CollectionItem
		if pager, ok := service.collections.(catalogCollectionPageRepository); ok {
			members, err = pager.ListItemsPage(ctx, item.ID, memberLimit+1, 0)
		} else {
			members, err = service.collections.ListItems(ctx, item.ID)
		}
		if err != nil {
			return nil, err
		}
		truncated := len(members) > memberLimit
		if truncated {
			members = members[:memberLimit]
		}
		value := catalogCollectionDTO(item, members)
		value.ItemIDsTruncated = truncated
		result = append(result, value)
	}
	return result, nil
}

func (service *CatalogService) ListCatalogCollectionItemsPage(
	ctx context.Context,
	collectionID string,
	limit int,
	offset int,
) (dto.CatalogCollectionItemsPageDTO, error) {
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" || limit < 1 || limit > maximumCatalogListLimit || offset < 0 {
		return dto.CatalogCollectionItemsPageDTO{}, errors.New("invalid Catalog collection item pagination")
	}
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.CatalogCollectionItemsPageDTO{}, err
	}
	collection, err := service.collections.Get(ctx, collectionID)
	if err != nil {
		return dto.CatalogCollectionItemsPageDTO{}, err
	}
	if collection.CatalogID != catalog.ID {
		return dto.CatalogCollectionItemsPageDTO{}, sql.ErrNoRows
	}
	var members []library.CollectionItem
	if pager, ok := service.collections.(catalogCollectionPageRepository); ok {
		members, err = pager.ListItemsPage(ctx, collectionID, limit+1, offset)
	} else {
		members, err = service.collections.ListItems(ctx, collectionID)
		if err == nil {
			if offset >= len(members) {
				members = nil
			} else {
				members = members[offset:min(offset+limit+1, len(members))]
			}
		}
	}
	if err != nil {
		return dto.CatalogCollectionItemsPageDTO{}, err
	}
	hasMore := len(members) > limit
	if hasMore {
		members = members[:limit]
	}
	result := dto.CatalogCollectionItemsPageDTO{
		CatalogID: catalog.ID, CollectionID: collectionID,
		Items:      make([]dto.CatalogCollectionItemDTO, 0, len(members)),
		NextOffset: offset + len(members), HasMore: hasMore,
	}
	for _, member := range members {
		result.Items = append(result.Items, dto.CatalogCollectionItemDTO{
			ID: member.ID, CollectionID: member.CollectionID, ItemID: member.ItemID, Position: member.Position,
		})
	}
	return result, nil
}

func (service *CatalogService) SaveCatalogCollection(ctx context.Context, request dto.SaveCatalogCollectionRequest) (dto.CatalogCollectionDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.CatalogCollectionDTO{}, err
	}
	now := service.timestamp()
	createdAt, revision, id := now, int64(1), strings.TrimSpace(request.ID)
	if id == "" {
		if request.ExpectedRevision != 0 {
			return dto.CatalogCollectionDTO{}, library.ErrCatalogRevisionConflict
		}
		id = service.newID()
	} else {
		current, getErr := service.collections.Get(ctx, id)
		if getErr != nil {
			return dto.CatalogCollectionDTO{}, getErr
		}
		if current.CatalogID != catalog.ID {
			return dto.CatalogCollectionDTO{}, sql.ErrNoRows
		}
		if request.ExpectedRevision != current.Revision {
			return dto.CatalogCollectionDTO{}, library.ErrCatalogRevisionConflict
		}
		createdAt, revision = current.CreatedAt, current.Revision+1
		now = service.timestampAtLeast(createdAt)
	}
	item, err := library.NewCollection(library.CollectionParams{
		ID: id, CatalogID: catalog.ID, Name: request.Name, Description: request.Description,
		Kind: request.Kind, SmartQuery: request.SmartQuery, Revision: revision,
		CreatedAt: &createdAt, UpdatedAt: &now,
	})
	if err != nil {
		return dto.CatalogCollectionDTO{}, err
	}
	if err := service.mutations.SaveCollectionMutation(ctx, item, request.ExpectedRevision, ""); err != nil {
		return dto.CatalogCollectionDTO{}, err
	}
	members, err := service.collections.ListItems(ctx, item.ID)
	if err != nil {
		return dto.CatalogCollectionDTO{}, err
	}
	return catalogCollectionDTO(item, members), nil
}

func (service *CatalogService) ReplaceCatalogCollectionItems(ctx context.Context, request dto.ReplaceCatalogCollectionItemsRequest) (dto.CatalogCollectionDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.CatalogCollectionDTO{}, err
	}
	current, err := service.collections.Get(ctx, strings.TrimSpace(request.CollectionID))
	if err != nil {
		return dto.CatalogCollectionDTO{}, err
	}
	if current.CatalogID != catalog.ID {
		return dto.CatalogCollectionDTO{}, sql.ErrNoRows
	}
	if current.Revision != request.ExpectedRevision || request.ExpectedRevision <= 0 {
		return dto.CatalogCollectionDTO{}, library.ErrCatalogRevisionConflict
	}
	if current.Kind == library.CollectionKindSmart {
		return dto.CatalogCollectionDTO{}, library.ErrInvalidCollectionItem
	}
	now := service.timestampAtLeast(current.CreatedAt)
	members := make([]library.CollectionItem, 0, len(request.ItemIDs))
	seen := make(map[string]struct{}, len(request.ItemIDs))
	for position, itemID := range request.ItemIDs {
		itemID = strings.TrimSpace(itemID)
		if _, duplicate := seen[itemID]; itemID == "" || duplicate {
			return dto.CatalogCollectionDTO{}, library.ErrInvalidCollectionItem
		}
		seen[itemID] = struct{}{}
		item, getErr := service.items.Get(ctx, itemID)
		if getErr != nil {
			return dto.CatalogCollectionDTO{}, getErr
		}
		if item.CatalogID != catalog.ID || item.Status == library.ItemStatusTrashed {
			return dto.CatalogCollectionDTO{}, library.ErrInvalidCollectionItem
		}
		member, buildErr := library.NewCollectionItem(service.newID(), current.ID, item.ID, position, now)
		if buildErr != nil {
			return dto.CatalogCollectionDTO{}, buildErr
		}
		members = append(members, member)
	}
	updated, err := library.NewCollection(library.CollectionParams{
		ID: current.ID, CatalogID: current.CatalogID, Name: current.Name, Description: current.Description,
		Kind: string(current.Kind), SmartQuery: current.SmartQuery, Revision: current.Revision + 1,
		CreatedAt: &current.CreatedAt, UpdatedAt: &now,
	})
	if err != nil {
		return dto.CatalogCollectionDTO{}, err
	}
	if err := service.mutations.ReplaceCollectionItemsMutation(ctx, updated, members, current.Revision, ""); err != nil {
		return dto.CatalogCollectionDTO{}, err
	}
	return catalogCollectionDTO(updated, members), nil
}

func (service *CatalogService) ListCatalogTags(ctx context.Context) ([]dto.CatalogTagDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.tags.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogTagDTO, 0, len(items))
	for _, item := range items {
		result = append(result, catalogTagDTO(item))
	}
	return result, nil
}

// ListCatalogTagsPage is the bounded remote-read counterpart of the desktop
// full list method.
func (service *CatalogService) ListCatalogTagsPage(ctx context.Context, limit int, offset int) ([]dto.CatalogTagDTO, error) {
	if limit < 1 || limit > maximumCatalogListLimit || offset < 0 {
		return nil, errors.New("invalid Catalog tag pagination")
	}
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return nil, err
	}
	var items []library.Tag
	if pager, ok := service.tags.(catalogTagPageRepository); ok {
		items, err = pager.ListByCatalogIDPage(ctx, catalog.ID, limit, offset)
	} else {
		items, err = service.tags.ListByCatalogID(ctx, catalog.ID)
		if err == nil {
			if offset >= len(items) {
				items = nil
			} else {
				items = items[offset:min(offset+limit, len(items))]
			}
		}
	}
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogTagDTO, 0, len(items))
	for _, item := range items {
		result = append(result, catalogTagDTO(item))
	}
	return result, nil
}

func (service *CatalogService) SaveCatalogTag(ctx context.Context, request dto.SaveCatalogTagRequest) (dto.CatalogTagDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.CatalogTagDTO{}, err
	}
	id, now, createdAt := strings.TrimSpace(request.ID), service.timestamp(), time.Time{}
	updatedAtFloor := createdAt
	if id == "" {
		id, createdAt = service.newID(), now
	} else {
		items, listErr := service.tags.ListByCatalogID(ctx, catalog.ID)
		if listErr != nil {
			return dto.CatalogTagDTO{}, listErr
		}
		for _, item := range items {
			if item.ID == id {
				createdAt = item.CreatedAt
				updatedAtFloor = item.UpdatedAt
				break
			}
		}
		if createdAt.IsZero() {
			return dto.CatalogTagDTO{}, sql.ErrNoRows
		}
		now = service.timestampAtLeast(updatedAtFloor)
	}
	item, err := library.NewTag(library.TagParams{
		ID: id, CatalogID: catalog.ID, Name: request.Name, CreatedAt: &createdAt, UpdatedAt: &now,
	})
	if err != nil {
		return dto.CatalogTagDTO{}, err
	}
	if err := service.mutations.SaveTagMutation(ctx, item, ""); err != nil {
		return dto.CatalogTagDTO{}, err
	}
	return catalogTagDTO(item), nil
}

func (service *CatalogService) ReplaceCatalogItemTags(ctx context.Context, request dto.ReplaceCatalogItemTagsRequest) ([]dto.CatalogTagDTO, error) {
	catalog, item, err := service.defaultCatalogItem(ctx, request.ItemID)
	if err != nil {
		return nil, err
	}
	available, err := service.tags.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]library.Tag, len(available))
	for _, tag := range available {
		byID[tag.ID] = tag
	}
	existing, err := service.tags.ListByItemID(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	existingByTag := make(map[string]library.ItemTag, len(existing))
	for _, binding := range existing {
		existingByTag[binding.TagID] = binding
	}
	now := service.timestamp()
	bindings := make([]library.ItemTag, 0, len(request.TagIDs))
	seen := make(map[string]struct{}, len(request.TagIDs))
	unchanged := len(existing) == len(request.TagIDs)
	for _, tagID := range request.TagIDs {
		tagID = strings.TrimSpace(tagID)
		if _, duplicate := seen[tagID]; tagID == "" || duplicate {
			return nil, library.ErrInvalidItemTag
		}
		seen[tagID] = struct{}{}
		if _, exists := byID[tagID]; !exists {
			return nil, library.ErrInvalidItemTag
		}
		binding, exists := existingByTag[tagID]
		if !exists {
			unchanged = false
			var buildErr error
			binding, buildErr = library.NewItemTag(service.newID(), item.ID, tagID, "", now)
			if buildErr != nil {
				return nil, buildErr
			}
		}
		bindings = append(bindings, binding)
	}
	if unchanged {
		return service.itemTagDTOs(ctx, catalog.ID, item.ID)
	}
	if err := service.mutations.ReplaceItemTagsMutation(ctx, catalog.ID, item.ID, bindings, "", now); err != nil {
		return nil, err
	}
	return service.itemTagDTOs(ctx, catalog.ID, item.ID)
}

func (service *CatalogService) ListCatalogStorageRoots(ctx context.Context) ([]dto.CatalogStorageRootDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.roots.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogStorageRootDTO, 0, len(items))
	for _, item := range items {
		result = append(result, catalogStorageRootDTO(item))
	}
	return result, nil
}

// EnsureManagedImportRoot turns the native copy destination into an actual
// managed Catalog root. File ownership is then derived by canonical path, so
// existing LibraryFile and Representation schemas remain backward compatible.
func (service *CatalogService) EnsureManagedImportRoot(ctx context.Context, rawPath string) (string, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return "", err
	}
	path, err := canonicalCatalogStoragePath(rawPath)
	if err != nil {
		return "", err
	}
	status, detail := inspectStorageRoot(path)
	if status != library.StorageRootStatusOnline {
		if detail == "" {
			detail = string(status)
		}
		return "", fmt.Errorf("managed storage root is not writable: %s", detail)
	}
	roots, err := service.roots.ListByCatalogID(ctx, catalog.ID)
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		rootPath, canonicalErr := canonicalCatalogStoragePath(root.Path)
		if canonicalErr != nil || !catalogPathsEqual(rootPath, path) {
			continue
		}
		if root.Mode != library.StorageRootModeManaged {
			return "", fmt.Errorf("storage root %q is registered as referenced", root.Name)
		}
		if _, err := service.CheckCatalogStorageRoot(ctx, dto.CheckCatalogStorageRootRequest{ID: root.ID}); err != nil {
			return "", err
		}
		return rootPath, nil
	}
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) || strings.TrimSpace(name) == "" {
		name = "Managed Library"
	}
	created, err := service.SaveCatalogStorageRoot(ctx, dto.SaveCatalogStorageRootRequest{
		Name: name, Path: path, Mode: string(library.StorageRootModeManaged),
	})
	if err != nil {
		return "", err
	}
	return created.Path, nil
}

func (service *CatalogService) SaveCatalogStorageRoot(ctx context.Context, request dto.SaveCatalogStorageRootRequest) (dto.CatalogStorageRootDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.CatalogStorageRootDTO{}, err
	}
	path := strings.TrimSpace(request.Path)
	if path == "" || !filepath.IsAbs(path) {
		return dto.CatalogStorageRootDTO{}, library.ErrInvalidStorageRoot
	}
	path = filepath.Clean(path)
	id, now, createdAt := strings.TrimSpace(request.ID), service.timestamp(), time.Time{}
	if id == "" {
		id, createdAt = service.newID(), now
	} else {
		current, getErr := service.roots.Get(ctx, id)
		if getErr != nil {
			return dto.CatalogStorageRootDTO{}, getErr
		}
		if current.CatalogID != catalog.ID {
			return dto.CatalogStorageRootDTO{}, sql.ErrNoRows
		}
		createdAt = current.CreatedAt
		now = service.timestampAtLeast(createdAt)
	}
	status, lastError := inspectStorageRoot(path)
	checkedAt := now
	mode := strings.TrimSpace(request.Mode)
	if mode == "" {
		mode = string(library.StorageRootModeReferenced)
	}
	item, err := library.NewStorageRoot(library.StorageRootParams{
		ID: id, CatalogID: catalog.ID, Name: request.Name, Path: path, VolumeID: request.VolumeID,
		Mode: mode, Status: string(status), LastCheckedAt: &checkedAt, LastError: lastError,
		CreatedAt: &createdAt, UpdatedAt: &now,
	})
	if err != nil {
		return dto.CatalogStorageRootDTO{}, err
	}
	if err := service.roots.Save(ctx, item); err != nil {
		return dto.CatalogStorageRootDTO{}, err
	}
	return catalogStorageRootDTO(item), nil
}

func (service *CatalogService) CheckCatalogStorageRoot(ctx context.Context, request dto.CheckCatalogStorageRootRequest) (dto.CatalogStorageRootDTO, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.CatalogStorageRootDTO{}, err
	}
	current, err := service.roots.Get(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return dto.CatalogStorageRootDTO{}, err
	}
	if current.CatalogID != catalog.ID {
		return dto.CatalogStorageRootDTO{}, sql.ErrNoRows
	}
	now := service.timestampAtLeast(current.CreatedAt)
	status, lastError := inspectStorageRoot(current.Path)
	checked, err := library.NewStorageRoot(library.StorageRootParams{
		ID: current.ID, CatalogID: current.CatalogID, Name: current.Name, Path: current.Path,
		VolumeID: current.VolumeID, Mode: string(current.Mode), Status: string(status),
		LastCheckedAt: &now, LastError: lastError, CreatedAt: &current.CreatedAt, UpdatedAt: &now,
	})
	if err != nil {
		return dto.CatalogStorageRootDTO{}, err
	}
	if err := service.roots.Save(ctx, checked); err != nil {
		return dto.CatalogStorageRootDTO{}, err
	}
	return catalogStorageRootDTO(checked), nil
}

func (service *CatalogService) GetCatalogUserState(ctx context.Context, request dto.GetCatalogUserStateRequest) (dto.CatalogUserStateDTO, error) {
	catalog, item, err := service.defaultCatalogItem(ctx, request.ItemID)
	if err != nil {
		return dto.CatalogUserStateDTO{}, err
	}
	state, err := service.userStates.Get(ctx, catalog.ID, item.ID, strings.TrimSpace(request.UserID))
	if err != nil {
		return dto.CatalogUserStateDTO{}, err
	}
	return catalogUserStateDTO(state), nil
}

func (service *CatalogService) UpdateCatalogUserState(ctx context.Context, request dto.UpdateCatalogUserStateRequest) (dto.CatalogUserStateDTO, error) {
	catalog, item, err := service.defaultCatalogItem(ctx, request.ItemID)
	if err != nil {
		return dto.CatalogUserStateDTO{}, err
	}
	userID := strings.TrimSpace(request.UserID)
	if userID == "" || request.ExpectedRevision < 0 {
		return dto.CatalogUserStateDTO{}, library.ErrInvalidUserState
	}
	now := service.timestampAtLeast(item.CreatedAt)
	state := library.UserState{CatalogID: catalog.ID, ItemID: item.ID, UserID: userID, Revision: 1, CreatedAt: now, UpdatedAt: now}
	state.ID = uuid.NewSHA1(catalogUserStateNamespace, []byte(strings.Join([]string{catalog.ID, item.ID, userID}, "\x00"))).String()
	if request.ExpectedRevision > 0 {
		state, err = service.userStates.Get(ctx, catalog.ID, item.ID, userID)
		if err != nil {
			return dto.CatalogUserStateDTO{}, err
		}
		if state.Revision != request.ExpectedRevision {
			return dto.CatalogUserStateDTO{}, library.ErrCatalogRevisionConflict
		}
		state.Revision++
		state.UpdatedAt = service.timestampAtLeast(state.CreatedAt)
	} else if _, getErr := service.userStates.Get(ctx, catalog.ID, item.ID, userID); getErr == nil {
		return dto.CatalogUserStateDTO{}, library.ErrCatalogRevisionConflict
	} else if !errors.Is(getErr, sql.ErrNoRows) {
		return dto.CatalogUserStateDTO{}, getErr
	}
	if request.Favorite != nil {
		state.Favorite = *request.Favorite
	}
	if request.Rating != nil {
		state.Rating = *request.Rating
	}
	if request.Progress != nil {
		state.Progress = *request.Progress
		if state.Progress < 1 && request.Completed == nil {
			state.Completed = false
		}
	}
	if request.PositionMs != nil {
		state.PositionMs = *request.PositionMs
	}
	if request.Locator != nil {
		state.Locator = *request.Locator
	}
	if request.Completed != nil {
		state.Completed = *request.Completed
		if state.Completed && state.Progress < 1 {
			state.Progress = 1
		}
	}
	if request.OpenedNow {
		openedAt := state.UpdatedAt
		state.LastOpenedAt = &openedAt
	}
	validated, err := library.NewUserState(library.UserStateParams{
		ID: state.ID, CatalogID: state.CatalogID, ItemID: state.ItemID, UserID: state.UserID,
		Favorite: state.Favorite, Rating: state.Rating, Progress: state.Progress, PositionMs: state.PositionMs,
		Locator: state.Locator, Completed: state.Completed, Revision: state.Revision,
		LastOpenedAt: state.LastOpenedAt, CreatedAt: &state.CreatedAt, UpdatedAt: &state.UpdatedAt,
	})
	if err != nil {
		return dto.CatalogUserStateDTO{}, err
	}
	if err := service.mutations.SaveUserStateMutation(ctx, validated, request.ExpectedRevision, request.ActorID); err != nil {
		return dto.CatalogUserStateDTO{}, err
	}
	return catalogUserStateDTO(validated), nil
}

func (service *CatalogService) GetCatalogMigrationAudit(ctx context.Context, request dto.CatalogMigrationAuditRequest) (dto.CatalogMigrationAuditDTO, error) {
	if service.auditor == nil {
		return dto.CatalogMigrationAuditDTO{}, errors.New("catalog auditor unavailable")
	}
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return dto.CatalogMigrationAuditDTO{}, err
	}
	migrationID := strings.TrimSpace(request.MigrationID)
	if migrationID == "" {
		migrationID = LegacyCatalogProjectionID
	}
	report, err := service.auditor.Audit(ctx, catalogaudit.Request{CatalogID: catalog.ID, MigrationID: migrationID})
	if err != nil {
		return dto.CatalogMigrationAuditDTO{}, err
	}
	return catalogMigrationAuditDTO(report), nil
}

func (service *CatalogService) defaultCatalog(ctx context.Context) (library.Catalog, error) {
	if service == nil || service.catalogs == nil || service.items == nil || service.assets == nil || service.files == nil ||
		service.roots == nil || service.collections == nil || service.tags == nil || service.userStates == nil || service.mutations == nil {
		return library.Catalog{}, errors.New("catalog service unavailable")
	}
	items, err := service.catalogs.List(ctx)
	if err != nil {
		return library.Catalog{}, err
	}
	for _, item := range items {
		if item.IsDefault && item.Status == library.CatalogStatusActive {
			return item, nil
		}
	}
	return library.Catalog{}, sql.ErrNoRows
}

func (service *CatalogService) defaultCatalogItem(ctx context.Context, id string) (library.Catalog, library.Item, error) {
	catalog, err := service.defaultCatalog(ctx)
	if err != nil {
		return library.Catalog{}, library.Item{}, err
	}
	item, err := service.items.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return library.Catalog{}, library.Item{}, err
	}
	if item.CatalogID != catalog.ID {
		return library.Catalog{}, library.Item{}, sql.ErrNoRows
	}
	return catalog, item, nil
}

func (service *CatalogService) professionalRepository() (library.CatalogProfessionalMutationRepository, error) {
	if service == nil || service.professional == nil {
		return nil, errors.New("catalog professional repository unavailable")
	}
	return service.professional, nil
}

func (service *CatalogService) restoredItemStatus(ctx context.Context, itemID string) (library.ItemStatus, error) {
	assets, err := service.assets.ListByItemID(ctx, itemID)
	if err != nil {
		return "", err
	}
	if len(assets) == 0 {
		return library.ItemStatusNeedsReview, nil
	}
	hasPlayableAsset := false
	for _, asset := range assets {
		if asset.Role != library.ItemAssetRoleOriginal && asset.Role != library.ItemAssetRoleRepresentation {
			continue
		}
		hasPlayableAsset = true
		file, getErr := service.files.Get(ctx, asset.FileID)
		if errors.Is(getErr, sql.ErrNoRows) {
			continue
		}
		if getErr != nil {
			return "", getErr
		}
		if catalogFileAvailable(file) {
			// Match runtime projection semantics: an available representation
			// legitimately replaces a removed original. Artwork/attachments alone
			// never make a restored media item healthy.
			return library.ItemStatusActive, nil
		}
	}
	if !hasPlayableAsset {
		return library.ItemStatusNeedsReview, nil
	}
	return library.ItemStatusMissing, nil
}

func (service *CatalogService) itemTagDTOs(ctx context.Context, catalogID, itemID string) ([]dto.CatalogTagDTO, error) {
	bindings, err := service.tags.ListByItemID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	available, err := service.tags.ListByCatalogID(ctx, catalogID)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		wanted[binding.TagID] = struct{}{}
	}
	result := make([]dto.CatalogTagDTO, 0, len(bindings))
	for _, item := range available {
		if _, exists := wanted[item.ID]; exists {
			result = append(result, catalogTagDTO(item))
		}
	}
	return result, nil
}

func (service *CatalogService) timestamp() time.Time {
	return service.now().UTC()
}

func (service *CatalogService) timestampAtLeast(minimum time.Time) time.Time {
	value := service.timestamp()
	if value.Before(minimum) {
		return minimum.UTC()
	}
	return value
}

func sortCatalogItems(items []library.Item, value string) error {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = "updated_desc"
	}
	less := func(left, right library.Item) bool { return false }
	switch value {
	case "updated_desc":
		less = func(left, right library.Item) bool {
			if left.UpdatedAt.Equal(right.UpdatedAt) {
				return left.ID < right.ID
			}
			return left.UpdatedAt.After(right.UpdatedAt)
		}
	case "created_desc", "created_asc":
		less = func(left, right library.Item) bool {
			if left.CreatedAt.Equal(right.CreatedAt) {
				return left.ID < right.ID
			}
			if value == "created_asc" {
				return left.CreatedAt.Before(right.CreatedAt)
			}
			return left.CreatedAt.After(right.CreatedAt)
		}
	case "title_asc", "title_desc", "category_asc":
		less = func(left, right library.Item) bool {
			leftKey, rightKey := strings.ToLower(left.SortTitle), strings.ToLower(right.SortTitle)
			if value == "category_asc" {
				leftKey, rightKey = string(left.Category)+"\x00"+leftKey, string(right.Category)+"\x00"+rightKey
			}
			if leftKey == rightKey {
				return left.ID < right.ID
			}
			if value == "title_desc" {
				return leftKey > rightKey
			}
			return leftKey < rightKey
		}
	default:
		return fmt.Errorf("invalid catalog sort %q", value)
	}
	sort.SliceStable(items, func(left, right int) bool { return less(items[left], items[right]) })
	return nil
}

func inspectStorageRoot(path string) (library.StorageRootStatus, string) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return library.StorageRootStatusOffline, ""
	}
	if err != nil {
		return library.StorageRootStatusError, err.Error()
	}
	if !info.IsDir() {
		return library.StorageRootStatusError, "storage root path is not a directory"
	}
	directory, err := os.Open(path)
	if err != nil {
		return library.StorageRootStatusError, err.Error()
	}
	_ = directory.Close()
	if info.Mode().Perm()&0222 == 0 {
		return library.StorageRootStatusReadOnly, ""
	}
	return library.StorageRootStatusOnline, ""
}

func legacyFileUnhealthy(item library.LibraryFile) bool {
	status := strings.ToLower(strings.TrimSpace(item.State.Status))
	return item.State.Deleted || item.State.LastError != "" || status == "deleted" || status == "missing" || status == "offline" || status == "error" || status == "unavailable"
}

func catalogFileAvailable(item library.LibraryFile) bool {
	if legacyFileUnhealthy(item) {
		return false
	}
	path := strings.TrimSpace(item.Storage.LocalPath)
	return path != "" && localFileExists(path)
}

// catalogFileNeedsMissingMaintenance deliberately matches the actionable
// scope of Library file maintenance. Deleted records are retained for Catalog
// history and synchronization, while empty paths and indeterminate stat errors
// cannot be safely classified as a missing local file.
func catalogFileNeedsMissingMaintenance(item library.LibraryFile) bool {
	status := strings.ToLower(strings.TrimSpace(item.State.Status))
	if item.State.Deleted || status == "deleted" {
		return false
	}
	path := strings.TrimSpace(item.Storage.LocalPath)
	return path != "" && localFileDefinitelyMissing(path)
}

func (service *CatalogService) catalogStorageRootAssignments(
	ctx context.Context,
	catalogID string,
	itemID string,
) (map[string]string, error) {
	result := make(map[string]string)
	roots, err := service.roots.ListByCatalogID(ctx, catalogID)
	if err != nil {
		return nil, err
	}
	assets, err := service.assets.ListByItemID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	for _, asset := range assets {
		file, err := service.files.Get(ctx, asset.FileID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[asset.ID] = catalogStorageRootIDForPath(roots, file.Storage.LocalPath)
	}
	return result, nil
}

func catalogStorageRootIDForPath(roots []library.StorageRoot, rawPath string) string {
	path, err := canonicalCatalogStoragePath(rawPath)
	if err != nil {
		return ""
	}
	matchedID, matchedLength := "", -1
	for _, root := range roots {
		rootPath, err := canonicalCatalogStoragePath(root.Path)
		if err != nil || !catalogPathWithinRoot(path, rootPath) || len(rootPath) <= matchedLength {
			continue
		}
		matchedID, matchedLength = root.ID, len(rootPath)
	}
	return matchedID
}

func canonicalCatalogStoragePath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", library.ErrInvalidStorageRoot
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = filepath.Clean(resolved)
	}
	return absolute, nil
}

func catalogPathsEqual(left, right string) bool {
	if filepath.Separator == '\\' {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func catalogPathWithinRoot(path, root string) bool {
	if catalogPathsEqual(path, root) {
		return true
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(relative) || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validCatalogCategory(value string) bool {
	switch library.ItemCategory(value) {
	case library.ItemCategoryVideo, library.ItemCategoryAudio, library.ItemCategoryBook, library.ItemCategoryImage, library.ItemCategoryOther:
		return true
	default:
		return false
	}
}

func validCatalogStatus(value string) bool {
	switch library.ItemStatus(value) {
	case library.ItemStatusActive, library.ItemStatusNeedsReview, library.ItemStatusMissing, library.ItemStatusTrashed:
		return true
	default:
		return false
	}
}

func catalogDTO(item library.Catalog) dto.CatalogDTO {
	return dto.CatalogDTO{
		ID: item.ID, Name: item.Name, Description: item.Description, Status: string(item.Status), IsDefault: item.IsDefault,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func catalogItemDTO(item library.Item) dto.CatalogItemDTO {
	result := dto.CatalogItemDTO{
		ID: item.ID, CatalogID: item.CatalogID, Category: string(item.Category), Status: string(item.Status),
		Title: item.Title, SortTitle: item.SortTitle, Description: item.Description, Revision: item.Revision,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if item.TrashedAt != nil {
		result.TrashedAt = item.TrashedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func catalogItemAssetDTO(item library.ItemAsset) dto.CatalogItemAssetDTO {
	return dto.CatalogItemAssetDTO{
		ID: item.ID, ItemID: item.ItemID, FileID: item.FileID, Role: string(item.Role), Label: item.Label, Position: item.Position,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func catalogRepresentationDTO(item library.Representation) dto.CatalogRepresentationDTO {
	return dto.CatalogRepresentationDTO{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, AssetID: item.AssetID,
		Kind: string(item.Kind), Purpose: string(item.Purpose), MediaType: item.MediaType,
		Container: item.Container, Codec: item.Codec, Width: item.Width, Height: item.Height,
		DurationMs: item.DurationMs, BitrateBps: item.BitrateBps, Language: item.Language,
		ChecksumAlgorithm: string(item.ChecksumAlgorithm), Checksum: item.Checksum,
		SizeBytes: item.SizeBytes, Availability: string(item.Availability), Revision: item.Revision,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func catalogMetadataEntryDTO(item library.MetadataEntry) dto.CatalogMetadataEntryDTO {
	return dto.CatalogMetadataEntryDTO{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, RepresentationID: item.RepresentationID,
		Namespace: item.Namespace, Key: item.Key, ValueType: string(item.ValueType), ValueJSON: string(item.Value),
		Language: item.Language, Position: item.Position, Source: string(item.Source), Provenance: item.Provenance,
		Confidence: item.Confidence, Locked: item.Locked, Revision: item.Revision,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func catalogCollectionDTO(item library.Collection, members []library.CollectionItem) dto.CatalogCollectionDTO {
	result := dto.CatalogCollectionDTO{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, Description: item.Description,
		Kind: string(item.Kind), SmartQuery: item.SmartQuery, Revision: item.Revision, ItemIDs: []string{},
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	for _, member := range members {
		result.ItemIDs = append(result.ItemIDs, member.ItemID)
	}
	return result
}

func catalogTagDTO(item library.Tag) dto.CatalogTagDTO {
	return dto.CatalogTagDTO{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, NormalizedName: item.NormalizedName,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func catalogStorageRootDTO(item library.StorageRoot) dto.CatalogStorageRootDTO {
	result := dto.CatalogStorageRootDTO{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, Path: item.Path, VolumeID: item.VolumeID,
		Mode: string(item.Mode), Status: string(item.Status), LastError: item.LastError,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if item.LastCheckedAt != nil {
		result.LastCheckedAt = item.LastCheckedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func catalogUserStateDTO(item library.UserState) dto.CatalogUserStateDTO {
	result := dto.CatalogUserStateDTO{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, UserID: item.UserID,
		Favorite: item.Favorite, Rating: item.Rating, Progress: item.Progress, PositionMs: item.PositionMs,
		Locator: item.Locator, Completed: item.Completed, Revision: item.Revision,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if item.LastOpenedAt != nil {
		result.LastOpenedAt = item.LastOpenedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func catalogMigrationAuditDTO(report catalogaudit.Report) dto.CatalogMigrationAuditDTO {
	result := dto.CatalogMigrationAuditDTO{
		CatalogID: report.CatalogID, MigrationID: report.MigrationID, Healthy: report.IsHealthy(),
		Counts: dto.CatalogMigrationAuditCountDTO{
			LegacyFiles: report.Counts.LegacyFiles, LegacyMappings: report.Counts.LegacyMappings,
			AssetLinks: report.Counts.AssetLinks, Items: report.Counts.Items, ActiveItems: report.Counts.ActiveItems,
			MissingItems: report.Counts.MissingItems, TrashedItems: report.Counts.TrashedItems,
			NeedsReviewItems: report.Counts.NeedsReviewItems, PreservedFileIDs: report.Counts.PreservedFileIDs,
			PreservedPhysicalReferences: report.Counts.PreservedPhysicalReferences,
			Representations:             report.Counts.Representations, MetadataEntries: report.Counts.MetadataEntries,
		},
		Findings: dto.CatalogMigrationAuditFindingDTO{
			UnmappedLegacyFiles: report.Findings.UnmappedLegacyFiles, DuplicateMappings: report.Findings.DuplicateMappings,
			DanglingAssets: report.Findings.DanglingAssets, MissingMappingSources: report.Findings.MissingMappingSources,
			MissingMappingTargets: report.Findings.MissingMappingTargets, MappingAssetMismatches: report.Findings.MappingAssetMismatches,
			ChangedPhysicalReferences: report.Findings.ChangedPhysicalReferences, Total: report.Findings.Total(),
			AssetsWithoutRepresentations:     report.Findings.AssetsWithoutRepresentations,
			DanglingRepresentations:          report.Findings.DanglingRepresentations,
			RepresentationMismatches:         report.Findings.RepresentationMismatches,
			DanglingMetadataEntries:          report.Findings.DanglingMetadataEntries,
			MetadataRepresentationMismatches: report.Findings.MetadataRepresentationMismatches,
		},
		Issues: []dto.CatalogMigrationAuditIssueDTO{}, AuditedAt: report.AuditedAt.UTC().Format(time.RFC3339Nano),
	}
	for _, issue := range report.Issues {
		result.Issues = append(result.Issues, dto.CatalogMigrationAuditIssueDTO{
			Kind: string(issue.Kind), SourceID: issue.SourceID, TargetID: issue.TargetID,
			AssetID: issue.AssetID, RepresentationID: issue.RepresentationID,
			MetadataEntryID: issue.MetadataEntryID, Description: issue.Description,
		})
	}
	return result
}
