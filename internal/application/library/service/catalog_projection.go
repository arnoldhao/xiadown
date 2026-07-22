package service

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/library/catalogaudit"
	"xiadown/internal/domain/library"
)

// v2 replays the additive projection once for existing databases so items
// whose downloaded original was replaced by a transcode are reconciled with
// the replacement-aware status rules.
const LegacyCatalogProjectionID = "catalog-foundation-v2"

var catalogProjectionNamespace = uuid.MustParse("0fe46461-2a5d-5b21-9fc7-b223be0cc9c9")

type catalogItemProjection struct {
	Item     library.Item
	Assets   []library.ItemAsset
	Mappings []library.LegacyMapping
}

// projectLegacyCatalogBundle builds a deterministic, non-destructive catalog
// view over one legacy Library bundle. Every LibraryFile is represented by one
// ItemAsset link. Physical file IDs and paths are never rewritten.
func projectLegacyCatalogBundle(catalogID string, files []library.LibraryFile, migratedAt time.Time) ([]catalogItemProjection, error) {
	if migratedAt.IsZero() {
		migratedAt = time.Now().UTC()
	}
	ordered := append([]library.LibraryFile(nil), files...)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].CreatedAt.Equal(ordered[right].CreatedAt) {
			return ordered[left].ID < ordered[right].ID
		}
		return ordered[left].CreatedAt.Before(ordered[right].CreatedAt)
	})
	filesByID := make(map[string]library.LibraryFile, len(ordered))
	for _, file := range ordered {
		filesByID[file.ID] = file
	}

	primaryIDs := make([]string, 0, len(ordered))
	for _, file := range ordered {
		if !catalogFileIsAuxiliary(file, filesByID) {
			primaryIDs = append(primaryIDs, file.ID)
		}
	}

	projections := make([]catalogItemProjection, 0, len(primaryIDs))
	projectionByPrimaryID := make(map[string]int, len(primaryIDs))
	for _, file := range ordered {
		if catalogFileIsAuxiliary(file, filesByID) {
			continue
		}
		projection, err := newCatalogItemProjection(catalogID, file, migratedAt, false)
		if err != nil {
			return nil, err
		}
		projectionByPrimaryID[file.ID] = len(projections)
		projections = append(projections, projection)
	}

	for _, file := range ordered {
		if !catalogFileIsAuxiliary(file, filesByID) {
			continue
		}
		primaryID := catalogAuxiliaryParentID(file, filesByID, primaryIDs)
		projectionIndex, attached := projectionByPrimaryID[primaryID]
		if !attached {
			projection, err := newCatalogItemProjection(catalogID, file, migratedAt, true)
			if err != nil {
				return nil, err
			}
			projectionByPrimaryID[file.ID] = len(projections)
			projections = append(projections, projection)
			continue
		}
		role := catalogAssetRole(file)
		position := catalogNextAssetPosition(projections[projectionIndex].Assets, role)
		link, mapping, err := newCatalogAssetProjection(catalogID, projections[projectionIndex].Item.ID, file, role, position, migratedAt)
		if err != nil {
			return nil, err
		}
		projections[projectionIndex].Assets = append(projections[projectionIndex].Assets, link)
		projections[projectionIndex].Mappings = append(projections[projectionIndex].Mappings, mapping)
	}
	for index := range projections {
		promoteProjectedReplacementStatus(&projections[index], filesByID)
	}
	return projections, nil
}

// promoteProjectedReplacementStatus prevents a removed intermediate download
// from turning the logical item into trash when a healthy transcode now owns
// playback. Artwork and attachments deliberately cannot rescue a missing
// primary media file.
func promoteProjectedReplacementStatus(
	projection *catalogItemProjection,
	filesByID map[string]library.LibraryFile,
) {
	if projection == nil || projection.Item.Status == library.ItemStatusActive {
		return
	}
	hasUnavailableOriginal := false
	hasAvailableReplacement := false
	for _, asset := range projection.Assets {
		file, exists := filesByID[asset.FileID]
		if !exists {
			continue
		}
		switch asset.Role {
		case library.ItemAssetRoleOriginal:
			hasUnavailableOriginal = legacyFileUnhealthy(file)
		case library.ItemAssetRoleRepresentation:
			if !legacyFileUnhealthy(file) {
				hasAvailableReplacement = true
			}
		}
	}
	if !hasUnavailableOriginal || !hasAvailableReplacement {
		return
	}
	projection.Item.Status = library.ItemStatusActive
	projection.Item.TrashedAt = nil
}

func newCatalogItemProjection(catalogID string, file library.LibraryFile, migratedAt time.Time, needsReview bool) (catalogItemProjection, error) {
	itemID := deterministicCatalogID("item", catalogID, file.ID)
	status := catalogItemStatus(file, needsReview)
	var trashedAt *time.Time
	if status == library.ItemStatusTrashed {
		value := file.UpdatedAt.UTC()
		trashedAt = &value
	}
	item, err := library.NewItem(library.ItemParams{
		ID:          itemID,
		CatalogID:   catalogID,
		Category:    string(catalogItemCategory(file)),
		Status:      string(status),
		Title:       catalogItemTitle(file),
		Description: "",
		Revision:    1,
		TrashedAt:   trashedAt,
		CreatedAt:   &file.CreatedAt,
		UpdatedAt:   &file.UpdatedAt,
	})
	if err != nil {
		return catalogItemProjection{}, err
	}
	link, mapping, err := newCatalogAssetProjection(catalogID, item.ID, file, library.ItemAssetRoleOriginal, 0, migratedAt)
	if err != nil {
		return catalogItemProjection{}, err
	}
	return catalogItemProjection{Item: item, Assets: []library.ItemAsset{link}, Mappings: []library.LegacyMapping{mapping}}, nil
}

func newCatalogAssetProjection(catalogID, itemID string, file library.LibraryFile, role library.ItemAssetRole, position int, migratedAt time.Time) (library.ItemAsset, library.LegacyMapping, error) {
	linkID := deterministicCatalogID("item-asset", itemID, file.ID, string(role))
	link, err := library.NewItemAsset(library.ItemAssetParams{
		ID: linkID, ItemID: itemID, FileID: file.ID, Role: string(role),
		Label: catalogAssetLabel(file, role), Position: position,
		CreatedAt: &file.CreatedAt, UpdatedAt: &file.UpdatedAt,
	})
	if err != nil {
		return library.ItemAsset{}, library.LegacyMapping{}, err
	}
	mapping, err := library.NewLegacyMapping(library.LegacyMappingParams{
		MigrationID: LegacyCatalogProjectionID, CatalogID: catalogID,
		SourceType: string(library.LegacyEntityFile), SourceID: file.ID,
		TargetType: string(library.CatalogEntityItemAsset), TargetID: link.ID,
		SourceFingerprint: catalogFileFingerprint(file), MigratedAt: migratedAt,
	})
	if err != nil {
		return library.ItemAsset{}, library.LegacyMapping{}, err
	}
	return link, mapping, nil
}

func catalogFileIsAuxiliary(file library.LibraryFile, filesByID map[string]library.LibraryFile) bool {
	switch file.Kind {
	case library.FileKindThumbnail, library.FileKindSubtitle, library.FileKindAPI, library.FileKindManifest:
		return true
	case library.FileKindTranscode:
		_, hasRoot := filesByID[strings.TrimSpace(file.Lineage.RootFileID)]
		return hasRoot
	default:
		return false
	}
}

func catalogAuxiliaryParentID(file library.LibraryFile, filesByID map[string]library.LibraryFile, primaryIDs []string) string {
	rootID := strings.TrimSpace(file.Lineage.RootFileID)
	if _, exists := filesByID[rootID]; exists {
		return rootID
	}
	stem := catalogFileStem(file)
	matchedID := ""
	for _, primaryID := range primaryIDs {
		primary, exists := filesByID[primaryID]
		if !exists || stem == "" || catalogFileStem(primary) != stem {
			continue
		}
		if matchedID != "" {
			matchedID = ""
			break
		}
		matchedID = primaryID
	}
	if matchedID != "" {
		return matchedID
	}
	if len(primaryIDs) == 1 {
		return primaryIDs[0]
	}
	return ""
}

func catalogFileStem(file library.LibraryFile) string {
	name := filepath.Base(catalogFilePath(file))
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name))))
}

func catalogAssetRole(file library.LibraryFile) library.ItemAssetRole {
	switch file.Kind {
	case library.FileKindThumbnail:
		return library.ItemAssetRoleArtwork
	case library.FileKindTranscode:
		return library.ItemAssetRoleRepresentation
	default:
		return library.ItemAssetRoleAttachment
	}
}

func catalogNextAssetPosition(assets []library.ItemAsset, role library.ItemAssetRole) int {
	position := 0
	for _, asset := range assets {
		if asset.Role == role && asset.Position >= position {
			position = asset.Position + 1
		}
	}
	return position
}

func catalogItemStatus(file library.LibraryFile, needsReview bool) library.ItemStatus {
	if file.State.Deleted {
		return library.ItemStatusTrashed
	}
	if needsReview {
		return library.ItemStatusNeedsReview
	}
	if legacyFileUnhealthy(file) {
		return library.ItemStatusMissing
	}
	return library.ItemStatusActive
}

func catalogItemCategory(file library.LibraryFile) library.ItemCategory {
	extension := strings.ToLower(filepath.Ext(catalogFilePath(file)))
	switch file.Kind {
	case library.FileKindVideo:
		return library.ItemCategoryVideo
	case library.FileKindAudio:
		return library.ItemCategoryAudio
	case library.FileKindDocument:
		if catalogBookExtension(extension) {
			return library.ItemCategoryBook
		}
	case library.FileKindThumbnail:
		return library.ItemCategoryImage
	case library.FileKindTranscode:
		if file.Media != nil && (file.Media.VideoCodec != "" || file.Media.Width != nil || file.Media.Height != nil) {
			return library.ItemCategoryVideo
		}
		if file.Media != nil && file.Media.AudioCodec != "" {
			return library.ItemCategoryAudio
		}
	}
	if catalogImageExtension(extension) {
		return library.ItemCategoryImage
	}
	if catalogBookExtension(extension) {
		return library.ItemCategoryBook
	}
	return library.ItemCategoryOther
}

func catalogBookExtension(extension string) bool {
	switch extension {
	case ".pdf", ".epub", ".mobi", ".azw", ".azw3", ".fb2", ".cbz", ".cbr":
		return true
	default:
		return false
	}
}

func catalogImageExtension(extension string) bool {
	switch extension {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".heif", ".avif", ".tif", ".tiff", ".bmp":
		return true
	default:
		return false
	}
}

func catalogItemTitle(file library.LibraryFile) string {
	for _, value := range []string{file.DisplayName, file.Metadata.Title, file.Name, filepath.Base(file.Storage.LocalPath)} {
		value = strings.TrimSpace(value)
		if value != "" {
			return strings.TrimSuffix(value, filepath.Ext(value))
		}
	}
	return file.ID
}

func catalogAssetLabel(file library.LibraryFile, role library.ItemAssetRole) string {
	if role == library.ItemAssetRoleOriginal {
		return "Original"
	}
	if file.Media != nil {
		format := strings.TrimSpace(file.Media.Format)
		if format != "" {
			return strings.ToUpper(format)
		}
	}
	return strings.TrimSpace(file.DisplayName)
}

func catalogFilePath(file library.LibraryFile) string {
	if value := strings.TrimSpace(file.Storage.LocalPath); value != "" {
		return value
	}
	return file.Name
}

func catalogFileFingerprint(file library.LibraryFile) string {
	return catalogaudit.FingerprintLegacyFileReference(catalogaudit.LegacyFileReference{
		ID: file.ID, LibraryID: file.LibraryID, Kind: string(file.Kind), Name: file.Name,
		DisplayName: file.DisplayName, StorageMode: file.Storage.Mode,
		LocalPath: file.Storage.LocalPath, DocumentID: file.Storage.DocumentID,
		LineageRootID: file.Lineage.RootFileID, SourceUpdatedAt: file.UpdatedAt,
	})
}

func deterministicCatalogID(parts ...string) string {
	return uuid.NewSHA1(catalogProjectionNamespace, []byte(strings.Join(parts, "\x00"))).String()
}
