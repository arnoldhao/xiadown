package libraryrepo

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/application/library/catalogaudit"
	"xiadown/internal/domain/library"
)

var _ catalogaudit.Auditor = (*SQLiteCatalogAuditor)(nil)

// SQLiteCatalogAuditor reconciles the additive Catalog projection against the
// legacy physical file registry. Audit uses a single read-only transaction and
// deliberately exposes no repair operation.
type SQLiteCatalogAuditor struct {
	db  *bun.DB
	now func() time.Time
}

func NewSQLiteCatalogAuditor(db *bun.DB) *SQLiteCatalogAuditor {
	return &SQLiteCatalogAuditor{db: db, now: time.Now}
}

type catalogAuditLegacyFile struct {
	ID                string
	LibraryID         string
	Kind              string
	Name              string
	DisplayName       sql.NullString
	StorageMode       string
	StorageLocalPath  sql.NullString
	StorageDocumentID sql.NullString
	LineageRootFileID sql.NullString
	UpdatedAt         time.Time
}

func (file catalogAuditLegacyFile) reference() catalogaudit.LegacyFileReference {
	name := strings.TrimSpace(file.Name)
	displayName := strings.TrimSpace(file.DisplayName.String)
	if displayName == "" {
		displayName = name
	}
	return catalogaudit.LegacyFileReference{
		ID:              strings.TrimSpace(file.ID),
		LibraryID:       strings.TrimSpace(file.LibraryID),
		Kind:            strings.TrimSpace(file.Kind),
		Name:            name,
		DisplayName:     displayName,
		StorageMode:     strings.TrimSpace(file.StorageMode),
		LocalPath:       strings.TrimSpace(file.StorageLocalPath.String),
		DocumentID:      strings.TrimSpace(file.StorageDocumentID.String),
		LineageRootID:   strings.TrimSpace(file.LineageRootFileID.String),
		SourceUpdatedAt: file.UpdatedAt,
	}
}

type catalogAuditMapping struct {
	SourceID          string
	TargetType        string
	TargetID          string
	SourceFingerprint string
}

type catalogAuditAsset struct {
	ID                  string
	ItemID              string
	FileID              string
	ItemCatalogID       sql.NullString
	ItemExists          bool
	FileExists          bool
	RepresentationCount int64
}

type catalogAuditRepresentation struct {
	ID            string
	CatalogID     string
	ItemID        string
	AssetID       string
	ItemCatalogID sql.NullString
	AssetItemID   sql.NullString
	ItemExists    bool
	AssetExists   bool
}

type catalogAuditMetadataEntry struct {
	ID                      string
	CatalogID               string
	ItemID                  string
	RepresentationID        sql.NullString
	ItemCatalogID           sql.NullString
	RepresentationCatalogID sql.NullString
	RepresentationItemID    sql.NullString
	ItemExists              bool
	RepresentationExists    bool
}

func (auditor *SQLiteCatalogAuditor) Audit(ctx context.Context, request catalogaudit.Request) (catalogaudit.Report, error) {
	catalogID := strings.TrimSpace(request.CatalogID)
	migrationID := strings.TrimSpace(request.MigrationID)
	if catalogID == "" || migrationID == "" {
		return catalogaudit.Report{}, catalogaudit.ErrInvalidRequest
	}

	tx, err := auditor.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return catalogaudit.Report{}, err
	}
	defer tx.Rollback()

	legacyFiles, err := loadCatalogAuditLegacyFiles(ctx, tx)
	if err != nil {
		return catalogaudit.Report{}, err
	}
	mappings, err := loadCatalogAuditMappings(ctx, tx, catalogID, migrationID)
	if err != nil {
		return catalogaudit.Report{}, err
	}
	assets, err := loadCatalogAuditAssets(ctx, tx)
	if err != nil {
		return catalogaudit.Report{}, err
	}
	representations, err := loadCatalogAuditRepresentations(ctx, tx, catalogID)
	if err != nil {
		return catalogaudit.Report{}, err
	}
	metadataEntries, err := loadCatalogAuditMetadataEntries(ctx, tx, catalogID)
	if err != nil {
		return catalogaudit.Report{}, err
	}

	report := catalogaudit.Report{
		CatalogID:   catalogID,
		MigrationID: migrationID,
		Issues:      make([]catalogaudit.Issue, 0),
		AuditedAt:   auditor.now().UTC(),
	}
	report.Counts.LegacyFiles = int64(len(legacyFiles))
	report.Counts.LegacyMappings = int64(len(mappings))
	if err := loadCatalogAuditItemCounts(ctx, tx, catalogID, &report.Counts); err != nil {
		return catalogaudit.Report{}, err
	}

	legacyByID := make(map[string]catalogAuditLegacyFile, len(legacyFiles))
	for _, file := range legacyFiles {
		legacyByID[file.ID] = file
	}
	assetsByID := make(map[string]catalogAuditAsset, len(assets))
	for _, asset := range assets {
		assetsByID[asset.ID] = asset
		if asset.ItemExists && asset.ItemCatalogID.String == catalogID {
			report.Counts.AssetLinks++
			if asset.RepresentationCount == 0 {
				report.Findings.AssetsWithoutRepresentations++
				report.Issues = append(report.Issues, catalogaudit.Issue{
					Kind: catalogaudit.IssueAssetWithoutRepresentation, AssetID: asset.ID,
					Description: "item asset has no technical representation",
				})
			}
		}
	}
	for _, representation := range representations {
		if representation.CatalogID == catalogID {
			report.Counts.Representations++
		}
		if !representation.ItemExists || !representation.AssetExists {
			report.Findings.DanglingRepresentations++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind:             catalogaudit.IssueDanglingRepresentation,
				RepresentationID: representation.ID, AssetID: representation.AssetID,
				Description: "representation references a missing item or item asset",
			})
			continue
		}
		if representation.ItemCatalogID.String != representation.CatalogID || representation.AssetItemID.String != representation.ItemID {
			report.Findings.RepresentationMismatches++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind:             catalogaudit.IssueRepresentationMismatch,
				RepresentationID: representation.ID, AssetID: representation.AssetID,
				Description: "representation catalog/item does not match its item asset",
			})
		}
	}
	for _, entry := range metadataEntries {
		if entry.CatalogID == catalogID {
			report.Counts.MetadataEntries++
		}
		representationRequired := entry.RepresentationID.Valid
		if !entry.ItemExists || representationRequired && !entry.RepresentationExists {
			report.Findings.DanglingMetadataEntries++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind:            catalogaudit.IssueDanglingMetadataEntry,
				MetadataEntryID: entry.ID, RepresentationID: entry.RepresentationID.String,
				Description: "metadata entry references a missing item or representation",
			})
			continue
		}
		mismatch := entry.ItemCatalogID.String != entry.CatalogID
		if representationRequired {
			mismatch = mismatch || entry.RepresentationCatalogID.String != entry.CatalogID || entry.RepresentationItemID.String != entry.ItemID
		}
		if mismatch {
			report.Findings.MetadataRepresentationMismatches++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind:            catalogaudit.IssueMetadataRepresentationMismatch,
				MetadataEntryID: entry.ID, RepresentationID: entry.RepresentationID.String,
				Description: "metadata entry catalog/item does not match its representation",
			})
		}
	}

	mappingSources := make(map[string][]catalogAuditMapping, len(mappings))
	mappingTargets := make(map[string][]catalogAuditMapping, len(mappings))
	for _, mapping := range mappings {
		mappingSources[mapping.SourceID] = append(mappingSources[mapping.SourceID], mapping)
		key := mapping.TargetType + "\x00" + mapping.TargetID
		mappingTargets[key] = append(mappingTargets[key], mapping)
	}
	for _, file := range legacyFiles {
		if len(mappingSources[file.ID]) != 0 {
			continue
		}
		report.Findings.UnmappedLegacyFiles++
		report.Issues = append(report.Issues, catalogaudit.Issue{
			Kind: catalogaudit.IssueUnmappedLegacyFile, SourceID: file.ID,
			Description: "legacy file has no mapping in the requested migration",
		})
	}
	for sourceID, duplicates := range mappingSources {
		if len(duplicates) <= 1 {
			continue
		}
		report.Findings.DuplicateMappings++
		report.Issues = append(report.Issues, catalogaudit.Issue{
			Kind: catalogaudit.IssueDuplicateMapping, SourceID: sourceID,
			Description: fmt.Sprintf("legacy source has %d mappings in the requested migration", len(duplicates)),
		})
	}
	for _, duplicates := range mappingTargets {
		if len(duplicates) <= 1 {
			continue
		}
		report.Findings.DuplicateMappings++
		report.Issues = append(report.Issues, catalogaudit.Issue{
			Kind: catalogaudit.IssueDuplicateMapping, TargetID: duplicates[0].TargetID,
			Description: fmt.Sprintf("catalog target is used by %d legacy mappings", len(duplicates)),
		})
	}

	requestedTargets := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		if mapping.TargetType == string(library.CatalogEntityItemAsset) {
			requestedTargets[mapping.TargetID] = struct{}{}
		}
	}
	for _, asset := range assets {
		_, isRequestedTarget := requestedTargets[asset.ID]
		belongsToCatalog := asset.ItemExists && asset.ItemCatalogID.String == catalogID
		if (!asset.ItemExists && isRequestedTarget) || (!asset.FileExists && (belongsToCatalog || isRequestedTarget)) {
			report.Findings.DanglingAssets++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind: catalogaudit.IssueDanglingAsset, AssetID: asset.ID,
				Description: "item asset references a missing catalog item or legacy file",
			})
		}
	}

	for _, mapping := range mappings {
		file, sourceExists := legacyByID[mapping.SourceID]
		if !sourceExists {
			report.Findings.MissingMappingSources++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind: catalogaudit.IssueMissingMappingSource, SourceID: mapping.SourceID, TargetID: mapping.TargetID,
				Description: "mapping references a legacy file that no longer exists",
			})
		}

		if mapping.TargetType != string(library.CatalogEntityItemAsset) {
			report.Findings.MappingAssetMismatches++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind: catalogaudit.IssueMappingAssetMismatch, SourceID: mapping.SourceID, TargetID: mapping.TargetID,
				Description: "legacy file mapping does not target an item_asset",
			})
		} else if asset, targetExists := assetsByID[mapping.TargetID]; !targetExists {
			report.Findings.MissingMappingTargets++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind: catalogaudit.IssueMissingMappingTarget, SourceID: mapping.SourceID, TargetID: mapping.TargetID,
				Description: "mapping target item asset does not exist",
			})
		} else if !asset.ItemExists || asset.ItemCatalogID.String != catalogID || asset.FileID != mapping.SourceID {
			report.Findings.MappingAssetMismatches++
			report.Issues = append(report.Issues, catalogaudit.Issue{
				Kind: catalogaudit.IssueMappingAssetMismatch, SourceID: mapping.SourceID,
				TargetID: mapping.TargetID, AssetID: asset.ID,
				Description: "mapping target belongs to another catalog or does not preserve the legacy file ID",
			})
		} else if sourceExists {
			report.Counts.PreservedFileIDs++
		}

		if sourceExists {
			currentFingerprint := catalogaudit.FingerprintLegacyFileReference(file.reference())
			if mapping.SourceFingerprint == currentFingerprint {
				report.Counts.PreservedPhysicalReferences++
			} else {
				report.Findings.ChangedPhysicalReferences++
				report.Issues = append(report.Issues, catalogaudit.Issue{
					Kind: catalogaudit.IssueChangedPhysicalReference, SourceID: mapping.SourceID, TargetID: mapping.TargetID,
					Description: "legacy identity or physical storage reference changed after projection",
				})
			}
		}
	}
	sort.Slice(report.Issues, func(left, right int) bool {
		leftIssue := report.Issues[left]
		rightIssue := report.Issues[right]
		leftKey := string(leftIssue.Kind) + "\x00" + leftIssue.SourceID + "\x00" + leftIssue.TargetID + "\x00" + leftIssue.AssetID + "\x00" + leftIssue.RepresentationID + "\x00" + leftIssue.MetadataEntryID
		rightKey := string(rightIssue.Kind) + "\x00" + rightIssue.SourceID + "\x00" + rightIssue.TargetID + "\x00" + rightIssue.AssetID + "\x00" + rightIssue.RepresentationID + "\x00" + rightIssue.MetadataEntryID
		return leftKey < rightKey
	})

	if err := tx.Commit(); err != nil {
		return catalogaudit.Report{}, err
	}
	return report, nil
}

func loadCatalogAuditLegacyFiles(ctx context.Context, tx bun.Tx) ([]catalogAuditLegacyFile, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, library_id, kind, name, display_name, storage_mode,
       storage_local_path, storage_document_id, lineage_root_file_id, updated_at
FROM library_files
ORDER BY id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]catalogAuditLegacyFile, 0)
	for rows.Next() {
		var row catalogAuditLegacyFile
		if err := rows.Scan(
			&row.ID, &row.LibraryID, &row.Kind, &row.Name, &row.DisplayName,
			&row.StorageMode, &row.StorageLocalPath, &row.StorageDocumentID,
			&row.LineageRootFileID, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadCatalogAuditMappings(ctx context.Context, tx bun.Tx, catalogID, migrationID string) ([]catalogAuditMapping, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT source_id, target_type, target_id, source_fingerprint
FROM library_legacy_mappings
WHERE catalog_id = ? AND migration_id = ? AND source_type = ?
ORDER BY source_id, target_type, target_id
`, catalogID, migrationID, string(library.LegacyEntityFile))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]catalogAuditMapping, 0)
	for rows.Next() {
		var row catalogAuditMapping
		if err := rows.Scan(&row.SourceID, &row.TargetType, &row.TargetID, &row.SourceFingerprint); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadCatalogAuditAssets(ctx context.Context, tx bun.Tx) ([]catalogAuditAsset, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT assets.id, assets.item_id, assets.file_id, items.catalog_id,
       items.id IS NOT NULL, files.id IS NOT NULL,
       (SELECT COUNT(*) FROM library_representations AS representations
        WHERE representations.asset_id = assets.id AND representations.item_id = assets.item_id)
FROM library_item_assets AS assets
LEFT JOIN library_catalog_items AS items ON items.id = assets.item_id
LEFT JOIN library_files AS files ON files.id = assets.file_id
ORDER BY assets.id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]catalogAuditAsset, 0)
	for rows.Next() {
		var row catalogAuditAsset
		if err := rows.Scan(
			&row.ID, &row.ItemID, &row.FileID, &row.ItemCatalogID,
			&row.ItemExists, &row.FileExists, &row.RepresentationCount,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadCatalogAuditRepresentations(ctx context.Context, tx bun.Tx, catalogID string) ([]catalogAuditRepresentation, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT representations.id, representations.catalog_id, representations.item_id, representations.asset_id,
       items.catalog_id, assets.item_id,
       items.id IS NOT NULL, assets.id IS NOT NULL
FROM library_representations AS representations
LEFT JOIN library_catalog_items AS items ON items.id = representations.item_id
LEFT JOIN library_item_assets AS assets ON assets.id = representations.asset_id
WHERE representations.catalog_id = ? OR items.catalog_id = ?
ORDER BY representations.id
`, catalogID, catalogID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]catalogAuditRepresentation, 0)
	for rows.Next() {
		var row catalogAuditRepresentation
		if err := rows.Scan(
			&row.ID, &row.CatalogID, &row.ItemID, &row.AssetID,
			&row.ItemCatalogID, &row.AssetItemID, &row.ItemExists, &row.AssetExists,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadCatalogAuditMetadataEntries(ctx context.Context, tx bun.Tx, catalogID string) ([]catalogAuditMetadataEntry, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT metadata.id, metadata.catalog_id, metadata.item_id, metadata.representation_id,
       items.catalog_id, representations.catalog_id, representations.item_id,
       items.id IS NOT NULL,
       metadata.representation_id IS NULL OR representations.id IS NOT NULL
FROM library_metadata_entries AS metadata
LEFT JOIN library_catalog_items AS items ON items.id = metadata.item_id
LEFT JOIN library_representations AS representations ON representations.id = metadata.representation_id
WHERE metadata.catalog_id = ? OR items.catalog_id = ?
ORDER BY metadata.id
`, catalogID, catalogID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]catalogAuditMetadataEntry, 0)
	for rows.Next() {
		var row catalogAuditMetadataEntry
		if err := rows.Scan(
			&row.ID, &row.CatalogID, &row.ItemID, &row.RepresentationID,
			&row.ItemCatalogID, &row.RepresentationCatalogID, &row.RepresentationItemID,
			&row.ItemExists, &row.RepresentationExists,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadCatalogAuditItemCounts(ctx context.Context, tx bun.Tx, catalogID string, counts *catalogaudit.Counts) error {
	return tx.QueryRowContext(ctx, `
SELECT COUNT(*),
       COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'missing' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'trashed' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'needs_review' THEN 1 ELSE 0 END), 0)
FROM library_catalog_items
WHERE catalog_id = ?
`, catalogID).Scan(
		&counts.Items,
		&counts.ActiveItems,
		&counts.MissingItems,
		&counts.TrashedItems,
		&counts.NeedsReviewItems,
	)
}
