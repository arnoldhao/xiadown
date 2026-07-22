package libraryrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

// SQLiteCatalogBackfillWriter keeps the migration cursor in the same SQLite
// transaction as every association created for a legacy Library bundle.
type SQLiteCatalogBackfillWriter struct{ db *bun.DB }

func NewSQLiteCatalogBackfillWriter(db *bun.DB) *SQLiteCatalogBackfillWriter {
	return &SQLiteCatalogBackfillWriter{db: db}
}

func (writer *SQLiteCatalogBackfillWriter) EnsureCatalog(ctx context.Context, item library.Catalog) error {
	validated, err := library.NewCatalog(library.CatalogParams{
		ID: item.ID, Name: item.Name, Description: item.Description, Status: string(item.Status),
		IsDefault: item.IsDefault, CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil {
		return err
	}
	_, err = writer.db.ExecContext(ctx, `
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING
`, validated.ID, validated.Name, validated.Description, validated.Status, validated.IsDefault,
		validated.CreatedAt, validated.UpdatedAt)
	return err
}

func (writer *SQLiteCatalogBackfillWriter) GetBackfillCheckpoint(
	ctx context.Context,
	migrationID string,
	phase library.MigrationPhase,
) (library.MigrationCheckpoint, bool, error) {
	item, err := NewSQLiteCatalogMigrationRepository(writer.db).GetCheckpoint(ctx, migrationID, phase)
	if errors.Is(err, sql.ErrNoRows) {
		return library.MigrationCheckpoint{}, false, nil
	}
	if err != nil {
		return library.MigrationCheckpoint{}, false, err
	}
	return item, true, nil
}

func (writer *SQLiteCatalogBackfillWriter) SaveBackfillCheckpoint(ctx context.Context, item library.MigrationCheckpoint) error {
	validated, err := validateBackfillCheckpoint(item)
	if err != nil {
		return err
	}
	return upsertBackfillCheckpoint(ctx, writer.db, validated)
}

func (writer *SQLiteCatalogBackfillWriter) SaveBackfillBundle(ctx context.Context, bundle library.CatalogBackfillBundle) error {
	return writer.saveBackfillBundle(ctx, bundle, true)
}

// SaveRuntimeProjection applies the same immutable mapping and runtime-state
// reconciliation transaction without moving the global migration cursor.
// The unchanged cursor intentionally leaves startup backfill as a durable
// safety net for an interrupted runtime projection.
func (writer *SQLiteCatalogBackfillWriter) SaveRuntimeProjection(ctx context.Context, bundle library.CatalogBackfillBundle) error {
	return writer.saveBackfillBundle(ctx, bundle, false)
}

func (writer *SQLiteCatalogBackfillWriter) saveBackfillBundle(
	ctx context.Context,
	bundle library.CatalogBackfillBundle,
	advanceCheckpoint bool,
) error {
	validated, err := validateBackfillBundle(bundle)
	if err != nil {
		return err
	}
	return writer.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		affectedItemIDs := make(map[string]struct{})
		ownerChangeEmitted := make(map[string]bool)
		nestedChangeEmitted := make(map[string]bool)
		for _, projected := range validated.Items {
			missingMappings := make(map[string]library.LegacyMapping, len(projected.Mappings))
			for _, mapping := range projected.Mappings {
				stored, exists, err := readBackfillMapping(ctx, tx, mapping)
				if err != nil {
					return fmt.Errorf("read legacy file mapping %q: %w", mapping.SourceID, err)
				}
				if exists {
					if err := verifyExistingFileMapping(ctx, tx, mapping, stored); err != nil {
						return err
					}
					itemID, err := readBackfillMappedItemID(ctx, tx, stored.TargetID)
					if err != nil {
						return fmt.Errorf("read mapped item for legacy file %q: %w", mapping.SourceID, err)
					}
					affectedItemIDs[itemID] = struct{}{}
					continue
				}
				missingMappings[mapping.TargetID] = mapping
			}
			// Stable mappings remain authoritative if later files would cause the
			// projection grouper to choose a different item. They still enter
			// affectedItemIDs above so runtime health is reconciled on every pass.
			if len(missingMappings) == 0 {
				continue
			}
			item := projected.Item
			insertResult, err := tx.ExecContext(ctx, `
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, description,
  revision, trashed_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING
`, item.ID, item.CatalogID, item.Category, item.Status, item.Title, item.SortTitle,
				item.Description, item.Revision, item.TrashedAt, item.CreatedAt, item.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert projected item %q: %w", item.ID, err)
			}
			itemInserted, err := resultChanged(insertResult)
			if err != nil {
				return fmt.Errorf("read projected item %q insert result: %w", item.ID, err)
			}
			var storedCatalogID string
			if err := tx.QueryRowContext(ctx, `
SELECT catalog_id
FROM library_catalog_items
WHERE id = ?
`, item.ID).Scan(&storedCatalogID); err != nil {
				return fmt.Errorf("verify projected item %q: %w", item.ID, err)
			}
			if storedCatalogID != item.CatalogID {
				return fmt.Errorf("projected item %q already belongs to another catalog", item.ID)
			}
			affectedItemIDs[item.ID] = struct{}{}
			if itemInserted {
				kind := library.CatalogChangeUpsert
				if item.Status == library.ItemStatusTrashed {
					kind = library.CatalogChangeDelete
				}
				if err := appendBackfillChange(ctx, tx, item.CatalogID, library.CatalogEntityItem,
					item.ID, kind, item.Revision, validated.Checkpoint.UpdatedAt); err != nil {
					return fmt.Errorf("append projected item %q change: %w", item.ID, err)
				}
				ownerChangeEmitted[item.ID] = true
			}

			for _, asset := range projected.Assets {
				mapping, missing := missingMappings[asset.ID]
				if !missing {
					continue
				}
				insertResult, err := tx.ExecContext(ctx, `
INSERT INTO library_item_assets (
  id, item_id, file_id, role, label, position, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING
`, asset.ID, asset.ItemID, asset.FileID, asset.Role, asset.Label, asset.Position,
					asset.CreatedAt, asset.UpdatedAt)
				if err != nil {
					return fmt.Errorf("insert projected asset %q: %w", asset.ID, err)
				}
				assetInserted, err := resultChanged(insertResult)
				if err != nil {
					return fmt.Errorf("read projected asset %q insert result: %w", asset.ID, err)
				}
				var storedItemID, storedFileID string
				if err := tx.QueryRowContext(ctx, `
SELECT item_id, file_id
FROM library_item_assets
WHERE id = ?
`, asset.ID).Scan(&storedItemID, &storedFileID); err != nil {
					return fmt.Errorf("verify projected asset %q: %w", asset.ID, err)
				}
				if storedItemID != asset.ItemID || storedFileID != asset.FileID {
					return fmt.Errorf("projected asset %q conflicts with an existing asset", asset.ID)
				}
				if err := insertStableBackfillMapping(ctx, tx, mapping); err != nil {
					return fmt.Errorf("insert legacy file mapping %q: %w", mapping.SourceID, err)
				}
				if assetInserted {
					nestedChangeEmitted[item.ID] = true
					if err := appendBackfillChange(ctx, tx, item.CatalogID, library.CatalogEntityItemAsset,
						asset.ID, library.CatalogChangeUpsert, 1, validated.Checkpoint.UpdatedAt); err != nil {
						return fmt.Errorf("append projected asset %q change: %w", asset.ID, err)
					}
					if err := appendBackfillRepresentationChange(ctx, tx, item.CatalogID, asset.ID,
						validated.Checkpoint.UpdatedAt); err != nil {
						return fmt.Errorf("append projected representation %q change: %w", asset.ID, err)
					}
				}
			}
		}
		for itemID := range affectedItemIDs {
			if err := reconcileBackfillItemRuntimeState(
				ctx, tx, validated.Checkpoint.CatalogID, itemID, validated.Checkpoint.UpdatedAt,
				ownerChangeEmitted[itemID], nestedChangeEmitted[itemID],
			); err != nil {
				return err
			}
		}
		if err := insertStableBackfillMapping(ctx, tx, validated.BundleMapping); err != nil {
			return fmt.Errorf("insert legacy bundle mapping: %w", err)
		}
		if advanceCheckpoint {
			if err := upsertBackfillCheckpoint(ctx, tx, validated.Checkpoint); err != nil {
				return fmt.Errorf("advance bundle checkpoint: %w", err)
			}
		}
		return nil
	})
}

type backfillFileState struct {
	Status    string
	Deleted   bool
	LastError string
}

type backfillAssetRuntimeState struct {
	assetID   string
	role      library.ItemAssetRole
	fileOK    bool
	fileState backfillFileState
}

func resultChanged(result sql.Result) (bool, error) {
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func readBackfillMappedItemID(ctx context.Context, executor backfillExecutor, assetID string) (string, error) {
	var itemID string
	err := executor.QueryRowContext(ctx, `
SELECT item_id FROM library_item_assets WHERE id = ?
`, strings.TrimSpace(assetID)).Scan(&itemID)
	return itemID, err
}

func appendBackfillChange(
	ctx context.Context,
	executor catalogChangeExecutor,
	catalogID string,
	entityType library.CatalogEntityType,
	entityID string,
	kind library.CatalogChangeKind,
	revision int64,
	occurredAt time.Time,
) error {
	change, err := library.NewCatalogChange(library.CatalogChangeParams{
		Sequence: 1, CatalogID: catalogID, EntityType: string(entityType), EntityID: entityID,
		Kind: string(kind), Revision: revision, OccurredAt: occurredAt,
	})
	if err != nil {
		return err
	}
	_, err = appendCatalogChange(ctx, executor, change)
	return err
}

func appendBackfillRepresentationChange(
	ctx context.Context,
	executor backfillExecutor,
	catalogID string,
	representationID string,
	occurredAt time.Time,
) error {
	var revision int64
	err := executor.QueryRowContext(ctx, `
SELECT revision FROM library_representations WHERE id = ? AND catalog_id = ?
`, representationID, catalogID).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return appendBackfillChange(ctx, executor, catalogID, library.CatalogEntityRepresentation,
		representationID, library.CatalogChangeUpsert, revision, occurredAt)
}

func reconcileBackfillItemRuntimeState(
	ctx context.Context,
	tx bun.Tx,
	catalogID string,
	itemID string,
	updatedAt time.Time,
	ownerChangeEmitted bool,
	nestedChangeEmitted bool,
) error {
	rows, err := tx.QueryContext(ctx, `
SELECT asset.id, asset.role, files.state_json
FROM library_item_assets AS asset
LEFT JOIN library_files AS files ON files.id = asset.file_id
WHERE asset.item_id = ?
ORDER BY CASE WHEN asset.role = 'original' THEN 0 ELSE 1 END, asset.position, asset.id
`, itemID)
	if err != nil {
		return fmt.Errorf("list runtime assets for catalog item %q: %w", itemID, err)
	}
	states := make([]backfillAssetRuntimeState, 0)
	for rows.Next() {
		var assetID, role string
		var stateJSON sql.NullString
		if err := rows.Scan(&assetID, &role, &stateJSON); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan runtime asset for catalog item %q: %w", itemID, err)
		}
		state := backfillAssetRuntimeState{assetID: assetID, role: library.ItemAssetRole(role), fileOK: stateJSON.Valid}
		if stateJSON.Valid && json.Unmarshal([]byte(stateJSON.String), &state.fileState) != nil {
			state.fileOK = false
		}
		states = append(states, state)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, state := range states {
		availability := runtimeRepresentationAvailability(state)
		var current string
		var revision int64
		err := tx.QueryRowContext(ctx, `
SELECT availability, revision
FROM library_representations
WHERE id = ? AND asset_id = ? AND item_id = ?
`, state.assetID, state.assetID, itemID).Scan(&current, &revision)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read generated representation %q: %w", state.assetID, err)
		}
		if current == string(availability) {
			continue
		}
		revision++
		if _, err := tx.ExecContext(ctx, `
UPDATE library_representations
SET availability = ?, revision = ?, updated_at = ?
WHERE id = ? AND asset_id = ? AND item_id = ?
`, availability, revision, updatedAt, state.assetID, state.assetID, itemID); err != nil {
			return fmt.Errorf("update generated representation %q availability: %w", state.assetID, err)
		}
		if err := appendBackfillChange(ctx, tx, catalogID, library.CatalogEntityRepresentation,
			state.assetID, library.CatalogChangeUpsert, revision, updatedAt); err != nil {
			return fmt.Errorf("append generated representation %q change: %w", state.assetID, err)
		}
		nestedChangeEmitted = true
	}

	var currentStatus string
	var currentRevision int64
	if err := tx.QueryRowContext(ctx, `
SELECT status, revision FROM library_catalog_items WHERE id = ? AND catalog_id = ?
`, itemID, catalogID).Scan(&currentStatus, &currentRevision); err != nil {
		return fmt.Errorf("read catalog item %q runtime status: %w", itemID, err)
	}
	desiredStatus := runtimeCatalogItemStatus(states)
	if desiredStatus == library.ItemStatusActive && library.ItemStatus(currentStatus) == library.ItemStatusNeedsReview {
		// needs_review can be an explicit user workflow state. A healthy source
		// must not silently dismiss that work; unhealthy source state still wins.
		return appendBackfillOwnerInvalidationIfNeeded(
			ctx, tx, catalogID, itemID, updatedAt, ownerChangeEmitted, nestedChangeEmitted,
		)
	}
	if library.ItemStatus(currentStatus) == library.ItemStatusTrashed {
		var userTombstone int
		if err := tx.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM library_catalog_tombstones
  WHERE catalog_id = ? AND entity_type = ? AND entity_id = ?
)
`, catalogID, library.CatalogEntityItem, itemID).Scan(&userTombstone); err != nil {
			return fmt.Errorf("read catalog item %q tombstone: %w", itemID, err)
		}
		if userTombstone != 0 {
			return appendBackfillOwnerInvalidationIfNeeded(
				ctx, tx, catalogID, itemID, updatedAt, ownerChangeEmitted, nestedChangeEmitted,
			)
		}
	}
	if currentStatus == string(desiredStatus) {
		return appendBackfillOwnerInvalidationIfNeeded(
			ctx, tx, catalogID, itemID, updatedAt, ownerChangeEmitted, nestedChangeEmitted,
		)
	}
	nextRevision := currentRevision + 1
	var trashedAt any
	kind := library.CatalogChangeUpsert
	if desiredStatus == library.ItemStatusTrashed {
		trashedAt = updatedAt
		kind = library.CatalogChangeDelete
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE library_catalog_items
SET status = ?, revision = ?, trashed_at = ?, updated_at = ?
WHERE id = ? AND catalog_id = ?
`, desiredStatus, nextRevision, trashedAt, updatedAt, itemID, catalogID); err != nil {
		return fmt.Errorf("update catalog item %q runtime status: %w", itemID, err)
	}
	if err := appendBackfillChange(ctx, tx, catalogID, library.CatalogEntityItem,
		itemID, kind, nextRevision, updatedAt); err != nil {
		return fmt.Errorf("append catalog item %q runtime change: %w", itemID, err)
	}
	return nil
}

func appendBackfillOwnerInvalidationIfNeeded(
	ctx context.Context,
	tx bun.Tx,
	catalogID string,
	itemID string,
	updatedAt time.Time,
	ownerChangeEmitted bool,
	nestedChangeEmitted bool,
) error {
	if ownerChangeEmitted || !nestedChangeEmitted {
		return nil
	}
	return appendOwningItemInvalidation(ctx, tx, catalogID, itemID, "", updatedAt)
}

func runtimeRepresentationAvailability(state backfillAssetRuntimeState) library.RepresentationAvailability {
	if !state.fileOK || state.fileState.Deleted || strings.TrimSpace(state.fileState.LastError) != "" {
		return library.RepresentationAvailabilityMissing
	}
	switch strings.ToLower(strings.TrimSpace(state.fileState.Status)) {
	case "deleted", "missing", "offline", "error", "unavailable":
		return library.RepresentationAvailabilityMissing
	default:
		return library.RepresentationAvailabilityAvailable
	}
}

func runtimeCatalogItemStatus(states []backfillAssetRuntimeState) library.ItemStatus {
	if len(states) == 0 {
		return library.ItemStatusNeedsReview
	}
	selected := states[0]
	for _, state := range states {
		if state.role == library.ItemAssetRoleOriginal && runtimeAssetAvailable(state) {
			return library.ItemStatusActive
		}
	}
	for _, state := range states {
		if state.role == library.ItemAssetRoleRepresentation && runtimeAssetAvailable(state) {
			return library.ItemStatusActive
		}
	}
	for _, state := range states {
		if state.role == library.ItemAssetRoleOriginal {
			selected = state
			break
		}
	}
	if !selected.fileOK || strings.TrimSpace(selected.fileState.LastError) != "" {
		return library.ItemStatusMissing
	}
	if selected.fileState.Deleted || strings.EqualFold(strings.TrimSpace(selected.fileState.Status), "deleted") {
		return library.ItemStatusTrashed
	}
	switch strings.ToLower(strings.TrimSpace(selected.fileState.Status)) {
	case "missing", "offline", "error", "unavailable":
		return library.ItemStatusMissing
	default:
		return library.ItemStatusActive
	}
}

func runtimeAssetAvailable(state backfillAssetRuntimeState) bool {
	if !state.fileOK || state.fileState.Deleted || strings.TrimSpace(state.fileState.LastError) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(state.fileState.Status)) {
	case "deleted", "missing", "offline", "error", "unavailable":
		return false
	default:
		return true
	}
}

type backfillExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type storedBackfillMapping struct {
	CatalogID  string
	TargetType library.CatalogEntityType
	TargetID   string
}

func readBackfillMapping(
	ctx context.Context,
	executor backfillExecutor,
	item library.LegacyMapping,
) (storedBackfillMapping, bool, error) {
	var stored storedBackfillMapping
	err := executor.QueryRowContext(ctx, `
SELECT catalog_id, target_type, target_id
FROM library_legacy_mappings
WHERE migration_id = ? AND source_type = ? AND source_id = ?
`, item.MigrationID, item.SourceType, item.SourceID).Scan(
		&stored.CatalogID, &stored.TargetType, &stored.TargetID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storedBackfillMapping{}, false, nil
	}
	if err != nil {
		return storedBackfillMapping{}, false, err
	}
	return stored, true, nil
}

func insertStableBackfillMapping(ctx context.Context, executor backfillExecutor, item library.LegacyMapping) error {
	if _, err := executor.ExecContext(ctx, `
INSERT INTO library_legacy_mappings (
  migration_id, catalog_id, source_type, source_id, target_type,
  target_id, source_fingerprint, migrated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(migration_id, source_type, source_id) DO NOTHING
`, item.MigrationID, item.CatalogID, item.SourceType, item.SourceID, item.TargetType,
		item.TargetID, item.SourceFingerprint, item.MigratedAt); err != nil {
		return err
	}
	stored, exists, err := readBackfillMapping(ctx, executor, item)
	if err != nil {
		return err
	}
	if !exists || stored.CatalogID != item.CatalogID || stored.TargetType != item.TargetType || stored.TargetID != item.TargetID {
		return errors.New("legacy mapping conflicts with an existing immutable mapping")
	}
	return nil
}

func verifyExistingFileMapping(
	ctx context.Context,
	executor backfillExecutor,
	projected library.LegacyMapping,
	stored storedBackfillMapping,
) error {
	if stored.CatalogID != projected.CatalogID || stored.TargetType != library.CatalogEntityItemAsset {
		return fmt.Errorf("legacy file mapping %q has an invalid immutable target", projected.SourceID)
	}
	var storedFileID, storedCatalogID string
	if err := executor.QueryRowContext(ctx, `
SELECT asset.file_id, item.catalog_id
FROM library_item_assets AS asset
JOIN library_catalog_items AS item ON item.id = asset.item_id
WHERE asset.id = ?
`, stored.TargetID).Scan(&storedFileID, &storedCatalogID); err != nil {
		return fmt.Errorf("verify immutable legacy file mapping %q: %w", projected.SourceID, err)
	}
	if storedFileID != projected.SourceID || storedCatalogID != projected.CatalogID {
		return fmt.Errorf("legacy file mapping %q no longer identifies its source asset", projected.SourceID)
	}
	return nil
}

func upsertBackfillCheckpoint(ctx context.Context, executor backfillExecutor, item library.MigrationCheckpoint) error {
	_, err := executor.ExecContext(ctx, `
INSERT INTO library_migration_checkpoints (
  migration_id, catalog_id, phase, status, cursor, processed, failed,
  last_error, started_at, finished_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(migration_id, phase) DO UPDATE SET
  catalog_id = EXCLUDED.catalog_id,
  status = EXCLUDED.status,
  cursor = EXCLUDED.cursor,
  processed = EXCLUDED.processed,
  failed = EXCLUDED.failed,
  last_error = EXCLUDED.last_error,
  started_at = EXCLUDED.started_at,
  finished_at = EXCLUDED.finished_at,
  updated_at = EXCLUDED.updated_at
`, item.MigrationID, item.CatalogID, item.Phase, item.Status, item.Cursor, item.Processed,
		item.Failed, item.LastError, item.StartedAt, item.FinishedAt, item.CreatedAt, item.UpdatedAt)
	return err
}

func validateBackfillBundle(bundle library.CatalogBackfillBundle) (library.CatalogBackfillBundle, error) {
	bundle.LegacyLibraryID = strings.TrimSpace(bundle.LegacyLibraryID)
	if bundle.LegacyLibraryID == "" || len(bundle.Items) == 0 {
		return library.CatalogBackfillBundle{}, errors.New("invalid empty catalog backfill bundle")
	}
	checkpoint, err := validateBackfillCheckpoint(bundle.Checkpoint)
	if err != nil {
		return library.CatalogBackfillBundle{}, err
	}
	if checkpoint.Status != library.MigrationStatusRunning || checkpoint.Cursor != bundle.LegacyLibraryID {
		return library.CatalogBackfillBundle{}, errors.New("catalog backfill checkpoint does not identify the bundle")
	}
	bundleMapping, err := validateBackfillMapping(bundle.BundleMapping)
	if err != nil {
		return library.CatalogBackfillBundle{}, err
	}
	if bundleMapping.MigrationID != checkpoint.MigrationID ||
		bundleMapping.CatalogID != checkpoint.CatalogID ||
		bundleMapping.SourceType != library.LegacyEntityLibrary ||
		bundleMapping.SourceID != bundle.LegacyLibraryID ||
		bundleMapping.TargetType != library.CatalogEntityCatalog ||
		bundleMapping.TargetID != checkpoint.CatalogID {
		return library.CatalogBackfillBundle{}, errors.New("invalid legacy bundle catalog mapping")
	}

	seenFiles := make(map[string]struct{})
	validatedItems := make([]library.CatalogBackfillItem, len(bundle.Items))
	for itemIndex, projected := range bundle.Items {
		item, err := library.NewItem(library.ItemParams{
			ID: projected.Item.ID, CatalogID: projected.Item.CatalogID,
			Category: string(projected.Item.Category), Status: string(projected.Item.Status),
			Title: projected.Item.Title, SortTitle: projected.Item.SortTitle,
			Description: projected.Item.Description, Revision: projected.Item.Revision,
			TrashedAt: projected.Item.TrashedAt, CreatedAt: &projected.Item.CreatedAt, UpdatedAt: &projected.Item.UpdatedAt,
		})
		if err != nil || item.CatalogID != checkpoint.CatalogID || len(projected.Assets) == 0 || len(projected.Assets) != len(projected.Mappings) {
			return library.CatalogBackfillBundle{}, errors.New("invalid projected catalog item")
		}
		assetsByID := make(map[string]library.ItemAsset, len(projected.Assets))
		assets := make([]library.ItemAsset, len(projected.Assets))
		for assetIndex, projectedAsset := range projected.Assets {
			asset, err := library.NewItemAsset(library.ItemAssetParams{
				ID: projectedAsset.ID, ItemID: projectedAsset.ItemID, FileID: projectedAsset.FileID,
				Role: string(projectedAsset.Role), Label: projectedAsset.Label, Position: projectedAsset.Position,
				CreatedAt: &projectedAsset.CreatedAt, UpdatedAt: &projectedAsset.UpdatedAt,
			})
			if err != nil || asset.ItemID != item.ID {
				return library.CatalogBackfillBundle{}, errors.New("invalid projected catalog asset")
			}
			if _, duplicate := seenFiles[asset.FileID]; duplicate {
				return library.CatalogBackfillBundle{}, errors.New("legacy file appears more than once in catalog backfill bundle")
			}
			seenFiles[asset.FileID] = struct{}{}
			assetsByID[asset.ID] = asset
			assets[assetIndex] = asset
		}
		mappings := make([]library.LegacyMapping, len(projected.Mappings))
		seenTargets := make(map[string]struct{}, len(projected.Mappings))
		for mappingIndex, projectedMapping := range projected.Mappings {
			mapping, err := validateBackfillMapping(projectedMapping)
			if err != nil {
				return library.CatalogBackfillBundle{}, err
			}
			asset, exists := assetsByID[mapping.TargetID]
			if !exists || mapping.MigrationID != checkpoint.MigrationID ||
				mapping.CatalogID != checkpoint.CatalogID ||
				mapping.SourceType != library.LegacyEntityFile ||
				mapping.TargetType != library.CatalogEntityItemAsset ||
				mapping.SourceID != asset.FileID {
				return library.CatalogBackfillBundle{}, errors.New("legacy file mapping does not identify its projected asset")
			}
			if _, duplicate := seenTargets[mapping.TargetID]; duplicate {
				return library.CatalogBackfillBundle{}, errors.New("projected asset has more than one legacy file mapping")
			}
			seenTargets[mapping.TargetID] = struct{}{}
			mappings[mappingIndex] = mapping
		}
		validatedItems[itemIndex] = library.CatalogBackfillItem{Item: item, Assets: assets, Mappings: mappings}
	}
	bundle.BundleMapping = bundleMapping
	bundle.Items = validatedItems
	bundle.Checkpoint = checkpoint
	return bundle, nil
}

func validateBackfillMapping(item library.LegacyMapping) (library.LegacyMapping, error) {
	return library.NewLegacyMapping(library.LegacyMappingParams{
		MigrationID: item.MigrationID, CatalogID: item.CatalogID,
		SourceType: string(item.SourceType), SourceID: item.SourceID,
		TargetType: string(item.TargetType), TargetID: item.TargetID,
		SourceFingerprint: item.SourceFingerprint, MigratedAt: item.MigratedAt,
	})
}

func validateBackfillCheckpoint(item library.MigrationCheckpoint) (library.MigrationCheckpoint, error) {
	return library.NewMigrationCheckpoint(library.MigrationCheckpointParams{
		MigrationID: item.MigrationID, CatalogID: item.CatalogID,
		Phase: string(item.Phase), Status: string(item.Status), Cursor: item.Cursor,
		Processed: item.Processed, Failed: item.Failed, LastError: item.LastError,
		StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
		CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
}

var _ library.CatalogBackfillWriter = (*SQLiteCatalogBackfillWriter)(nil)
