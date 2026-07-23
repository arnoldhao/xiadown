package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

// SQLiteCatalogMutationRepository makes an aggregate mutation, its change-feed
// cursor, and its current tombstone state visible atomically.
type SQLiteCatalogMutationRepository struct{ db *bun.DB }

func NewSQLiteCatalogMutationRepository(db *bun.DB) *SQLiteCatalogMutationRepository {
	return &SQLiteCatalogMutationRepository{db: db}
}

func (repo *SQLiteCatalogMutationRepository) SaveItemMutation(
	ctx context.Context,
	item library.Item,
	expectedRevision int64,
	kind library.CatalogChangeKind,
	actorID string,
	tombstoneExpiresAt *time.Time,
) error {
	validated, err := library.NewItem(library.ItemParams{
		ID: item.ID, CatalogID: item.CatalogID, Category: string(item.Category), Status: string(item.Status),
		Title: item.Title, SortTitle: item.SortTitle, Description: item.Description, Revision: item.Revision,
		TrashedAt: item.TrashedAt, CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil || expectedRevision <= 0 || validated.Revision != expectedRevision+1 {
		return library.ErrInvalidCatalogItem
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: validated.CatalogID, EntityType: library.CatalogEntityItem,
		EntityID: validated.ID, Kind: kind, Revision: validated.Revision,
		ActorID: strings.TrimSpace(actorID), OccurredAt: validated.UpdatedAt,
	})
	if err != nil {
		return err
	}
	if kind == library.CatalogChangeDelete && validated.Status != library.ItemStatusTrashed {
		return library.ErrInvalidCatalogItem
	}
	if kind == library.CatalogChangeUpsert && validated.Status == library.ItemStatusTrashed {
		return library.ErrInvalidCatalogItem
	}

	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE library_catalog_items SET
  category = ?, status = ?, title = ?, sort_title = ?, description = ?,
  revision = ?, trashed_at = ?, updated_at = ?
WHERE id = ? AND catalog_id = ? AND revision = ?
`, validated.Category, validated.Status, validated.Title, validated.SortTitle, validated.Description,
			validated.Revision, validated.TrashedAt, validated.UpdatedAt,
			validated.ID, validated.CatalogID, expectedRevision)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return classifyRevisionFailure(ctx, tx, "library_catalog_items", validated.ID, validated.CatalogID)
		}
		sequence, err := appendCatalogChange(ctx, tx, change)
		if err != nil {
			return err
		}
		if kind == library.CatalogChangeDelete {
			tombstone, err := library.NewTombstone(library.TombstoneParams{
				Sequence: sequence, CatalogID: validated.CatalogID, EntityType: string(library.CatalogEntityItem),
				EntityID: validated.ID, Revision: validated.Revision, DeletedAt: validated.UpdatedAt,
				ExpiresAt: tombstoneExpiresAt,
			})
			if err != nil {
				return err
			}
			return upsertCatalogTombstone(ctx, tx, tombstone)
		}
		_, err = tx.ExecContext(ctx, `
DELETE FROM library_catalog_tombstones
WHERE catalog_id = ? AND entity_type = ? AND entity_id = ?
`, validated.CatalogID, library.CatalogEntityItem, validated.ID)
		return err
	})
}

func (repo *SQLiteCatalogMutationRepository) ListRepresentationsByItemID(ctx context.Context, itemID string) ([]library.Representation, error) {
	return NewSQLiteRepresentationRepository(repo.db).ListRepresentationsByItemID(ctx, itemID)
}

func (repo *SQLiteCatalogMutationRepository) GetRepresentation(ctx context.Context, id string) (library.Representation, error) {
	return NewSQLiteRepresentationRepository(repo.db).GetRepresentation(ctx, id)
}

func (repo *SQLiteCatalogMutationRepository) SaveRepresentation(ctx context.Context, item library.Representation) error {
	return NewSQLiteRepresentationRepository(repo.db).SaveRepresentation(ctx, item)
}

func (repo *SQLiteCatalogMutationRepository) DeleteRepresentation(ctx context.Context, id string) error {
	return NewSQLiteRepresentationRepository(repo.db).DeleteRepresentation(ctx, id)
}

func (repo *SQLiteCatalogMutationRepository) ListMetadataEntriesByItemID(ctx context.Context, itemID string) ([]library.MetadataEntry, error) {
	return NewSQLiteMetadataEntryRepository(repo.db).ListMetadataEntriesByItemID(ctx, itemID)
}

func (repo *SQLiteCatalogMutationRepository) ListMetadataEntriesByRepresentationID(ctx context.Context, representationID string) ([]library.MetadataEntry, error) {
	return NewSQLiteMetadataEntryRepository(repo.db).ListMetadataEntriesByRepresentationID(ctx, representationID)
}

func (repo *SQLiteCatalogMutationRepository) GetMetadataEntry(ctx context.Context, id string) (library.MetadataEntry, error) {
	return NewSQLiteMetadataEntryRepository(repo.db).GetMetadataEntry(ctx, id)
}

func (repo *SQLiteCatalogMutationRepository) SaveMetadataEntry(ctx context.Context, item library.MetadataEntry) error {
	return NewSQLiteMetadataEntryRepository(repo.db).SaveMetadataEntry(ctx, item)
}

func (repo *SQLiteCatalogMutationRepository) DeleteMetadataEntry(ctx context.Context, id string) error {
	return NewSQLiteMetadataEntryRepository(repo.db).DeleteMetadataEntry(ctx, id)
}

func (repo *SQLiteCatalogMutationRepository) SaveRepresentationMutation(
	ctx context.Context,
	item library.Representation,
	expectedRevision int64,
	actorID string,
) error {
	validated, err := validateRepresentation(item)
	if err != nil || expectedRevision < 0 || validated.Revision != expectedRevision+1 {
		return library.ErrInvalidRepresentation
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: validated.CatalogID, EntityType: library.CatalogEntityRepresentation,
		EntityID: validated.ID, Kind: library.CatalogChangeUpsert, Revision: validated.Revision,
		ActorID: strings.TrimSpace(actorID), OccurredAt: validated.UpdatedAt,
	})
	if err != nil {
		return err
	}
	row := toRepresentationRow(validated)
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var itemCatalogID string
		if err := tx.QueryRowContext(ctx, `
SELECT items.catalog_id
FROM library_item_assets AS assets
JOIN library_catalog_items AS items ON items.id = assets.item_id
WHERE assets.id = ? AND assets.item_id = ?
`, validated.AssetID, validated.ItemID).Scan(&itemCatalogID); err != nil {
			return err
		}
		if itemCatalogID != validated.CatalogID {
			return sql.ErrNoRows
		}
		if expectedRevision == 0 {
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				var existingID string
				if lookupErr := tx.QueryRowContext(ctx, "SELECT id FROM library_representations WHERE id = ?", validated.ID).Scan(&existingID); lookupErr == nil {
					return library.ErrCatalogRevisionConflict
				}
				return err
			}
		} else {
			result, err := tx.NewUpdate().Model(&row).
				Column(
					"kind", "purpose", "media_type", "container", "codec", "width", "height",
					"duration_ms", "bitrate_bps", "language", "checksum_algorithm", "checksum",
					"size_bytes", "availability", "revision", "updated_at",
				).
				Where("id = ?", validated.ID).
				Where("catalog_id = ?", validated.CatalogID).
				Where("item_id = ?", validated.ItemID).
				Where("asset_id = ?", validated.AssetID).
				Where("revision = ?", expectedRevision).
				Exec(ctx)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				return classifyRevisionFailure(ctx, tx, "library_representations", validated.ID, validated.CatalogID)
			}
		}
		if _, err := appendCatalogChange(ctx, tx, change); err != nil {
			return err
		}
		return appendOwningItemInvalidation(
			ctx, tx, validated.CatalogID, validated.ItemID, strings.TrimSpace(actorID), validated.UpdatedAt,
		)
	})
}

func (repo *SQLiteCatalogMutationRepository) SaveMetadataEntryMutation(
	ctx context.Context,
	item library.MetadataEntry,
	expectedRevision int64,
	actorID string,
) error {
	validated, err := validateMetadataEntry(item)
	if err != nil || expectedRevision < 0 || validated.Revision != expectedRevision+1 {
		return library.ErrInvalidMetadataEntry
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: validated.CatalogID, EntityType: library.CatalogEntityMetadataEntry,
		EntityID: validated.ID, Kind: library.CatalogChangeUpsert, Revision: validated.Revision,
		ActorID: strings.TrimSpace(actorID), OccurredAt: validated.UpdatedAt,
	})
	if err != nil {
		return err
	}
	row := toMetadataEntryRow(validated)
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var itemCatalogID string
		if err := tx.QueryRowContext(ctx, "SELECT catalog_id FROM library_catalog_items WHERE id = ?", validated.ItemID).Scan(&itemCatalogID); err != nil {
			return err
		}
		if itemCatalogID != validated.CatalogID {
			return sql.ErrNoRows
		}
		if validated.RepresentationID != "" {
			var representationCatalogID, representationItemID string
			if err := tx.QueryRowContext(ctx, `
SELECT catalog_id, item_id FROM library_representations WHERE id = ?
`, validated.RepresentationID).Scan(&representationCatalogID, &representationItemID); err != nil {
				return err
			}
			if representationCatalogID != validated.CatalogID || representationItemID != validated.ItemID {
				return sql.ErrNoRows
			}
		}
		if expectedRevision == 0 {
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				var existingID string
				if lookupErr := tx.QueryRowContext(ctx, "SELECT id FROM library_metadata_entries WHERE id = ?", validated.ID).Scan(&existingID); lookupErr == nil {
					return library.ErrCatalogRevisionConflict
				}
				return err
			}
		} else {
			result, err := tx.NewUpdate().Model(&row).
				Column(
					"namespace", "key", "value_type", "value_json", "language", "position",
					"source", "provenance", "confidence", "locked", "revision", "updated_at",
				).
				Where("id = ?", validated.ID).
				Where("catalog_id = ?", validated.CatalogID).
				Where("item_id = ?", validated.ItemID).
				Where("COALESCE(representation_id, '') = ?", validated.RepresentationID).
				Where("revision = ?", expectedRevision).
				Exec(ctx)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				return classifyRevisionFailure(ctx, tx, "library_metadata_entries", validated.ID, validated.CatalogID)
			}
		}
		if _, err := appendCatalogChange(ctx, tx, change); err != nil {
			return err
		}
		return appendOwningItemInvalidation(
			ctx, tx, validated.CatalogID, validated.ItemID, strings.TrimSpace(actorID), validated.UpdatedAt,
		)
	})
}

func (repo *SQLiteCatalogMutationRepository) DeleteMetadataEntryMutation(
	ctx context.Context,
	item library.MetadataEntry,
	actorID string,
) error {
	validated, err := validateMetadataEntry(item)
	if err != nil {
		return library.ErrInvalidMetadataEntry
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: validated.CatalogID, EntityType: library.CatalogEntityMetadataEntry,
		EntityID: validated.ID, Kind: library.CatalogChangeDelete, Revision: validated.Revision + 1,
		ActorID: strings.TrimSpace(actorID), OccurredAt: validated.UpdatedAt,
	})
	if err != nil {
		return err
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		result, err := tx.NewDelete().
			Model((*catalogMetadataEntryRow)(nil)).
			Where("id = ?", validated.ID).
			Where("catalog_id = ?", validated.CatalogID).
			Where("item_id = ?", validated.ItemID).
			Where("revision = ?", validated.Revision).
			Exec(ctx)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return classifyRevisionFailure(ctx, tx, "library_metadata_entries", validated.ID, validated.CatalogID)
		}
		if _, err := appendCatalogChange(ctx, tx, change); err != nil {
			return err
		}
		return appendOwningItemInvalidation(
			ctx, tx, validated.CatalogID, validated.ItemID,
			strings.TrimSpace(actorID), validated.UpdatedAt,
		)
	})
}

type validatedMetadataBatchUpsert struct {
	item     library.MetadataEntry
	expected int64
	change   library.CatalogChange
}

type validatedMetadataBatchDelete struct {
	item   library.MetadataEntry
	change library.CatalogChange
}

// SaveItemMetadataBatchMutation is the aggregate write used by local Music
// tag editing. The public Item title, first-class music fields, legacy JSON
// snapshot, and every corresponding change-feed record become visible in one
// SQLite commit. Any revision conflict or later trigger/change-feed failure
// rolls the whole edit back.
func (repo *SQLiteCatalogMutationRepository) SaveItemMetadataBatchMutation(
	ctx context.Context,
	item *library.Item,
	expectedItemRevision int64,
	upserts []library.MetadataEntry,
	deletes []library.MetadataEntry,
	actorID string,
) error {
	actorID = strings.TrimSpace(actorID)
	var validatedItem *library.Item
	var itemChange library.CatalogChange
	catalogID := ""
	itemID := ""
	if item != nil {
		value, err := library.NewItem(library.ItemParams{
			ID: item.ID, CatalogID: item.CatalogID, Category: string(item.Category), Status: string(item.Status),
			Title: item.Title, SortTitle: item.SortTitle, Description: item.Description, Revision: item.Revision,
			TrashedAt: item.TrashedAt, CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
		})
		if err != nil || expectedItemRevision <= 0 || value.Revision != expectedItemRevision+1 ||
			value.Status == library.ItemStatusTrashed {
			return library.ErrInvalidCatalogItem
		}
		change, err := validateCatalogChangeForAppend(library.CatalogChange{
			CatalogID: value.CatalogID, EntityType: library.CatalogEntityItem,
			EntityID: value.ID, Kind: library.CatalogChangeUpsert, Revision: value.Revision,
			ActorID: actorID, OccurredAt: value.UpdatedAt,
		})
		if err != nil {
			return err
		}
		validatedItem = &value
		itemChange = change
		catalogID, itemID = value.CatalogID, value.ID
	} else if expectedItemRevision != 0 {
		return library.ErrInvalidCatalogItem
	}

	seenIDs := make(map[string]struct{}, len(upserts)+len(deletes))
	validatedUpserts := make([]validatedMetadataBatchUpsert, 0, len(upserts))
	validatedDeletes := make([]validatedMetadataBatchDelete, 0, len(deletes))
	latestChangeAt := time.Time{}
	for _, candidate := range upserts {
		value, err := validateMetadataEntry(candidate)
		expected := value.Revision - 1
		if err != nil || expected < 0 {
			return library.ErrInvalidMetadataEntry
		}
		if catalogID == "" {
			catalogID, itemID = value.CatalogID, value.ItemID
		}
		if value.CatalogID != catalogID || value.ItemID != itemID {
			return library.ErrInvalidMetadataEntry
		}
		if _, duplicate := seenIDs[value.ID]; duplicate {
			return library.ErrInvalidMetadataEntry
		}
		seenIDs[value.ID] = struct{}{}
		change, err := validateCatalogChangeForAppend(library.CatalogChange{
			CatalogID: value.CatalogID, EntityType: library.CatalogEntityMetadataEntry,
			EntityID: value.ID, Kind: library.CatalogChangeUpsert, Revision: value.Revision,
			ActorID: actorID, OccurredAt: value.UpdatedAt,
		})
		if err != nil {
			return err
		}
		if value.UpdatedAt.After(latestChangeAt) {
			latestChangeAt = value.UpdatedAt
		}
		validatedUpserts = append(validatedUpserts, validatedMetadataBatchUpsert{
			item: value, expected: expected, change: change,
		})
	}
	for _, candidate := range deletes {
		value, err := validateMetadataEntry(candidate)
		if err != nil {
			return library.ErrInvalidMetadataEntry
		}
		if catalogID == "" {
			catalogID, itemID = value.CatalogID, value.ItemID
		}
		if value.CatalogID != catalogID || value.ItemID != itemID {
			return library.ErrInvalidMetadataEntry
		}
		if _, duplicate := seenIDs[value.ID]; duplicate {
			return library.ErrInvalidMetadataEntry
		}
		seenIDs[value.ID] = struct{}{}
		change, err := validateCatalogChangeForAppend(library.CatalogChange{
			CatalogID: value.CatalogID, EntityType: library.CatalogEntityMetadataEntry,
			EntityID: value.ID, Kind: library.CatalogChangeDelete, Revision: value.Revision + 1,
			ActorID: actorID, OccurredAt: value.UpdatedAt,
		})
		if err != nil {
			return err
		}
		if value.UpdatedAt.After(latestChangeAt) {
			latestChangeAt = value.UpdatedAt
		}
		validatedDeletes = append(validatedDeletes, validatedMetadataBatchDelete{item: value, change: change})
	}
	if validatedItem == nil && len(validatedUpserts) == 0 && len(validatedDeletes) == 0 {
		return nil
	}
	if validatedItem != nil && validatedItem.UpdatedAt.After(latestChangeAt) {
		latestChangeAt = validatedItem.UpdatedAt
	}

	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if validatedItem != nil {
			result, err := tx.ExecContext(ctx, `
UPDATE library_catalog_items SET
  category = ?, status = ?, title = ?, sort_title = ?, description = ?,
  revision = ?, trashed_at = ?, updated_at = ?
WHERE id = ? AND catalog_id = ? AND revision = ?
`, validatedItem.Category, validatedItem.Status, validatedItem.Title, validatedItem.SortTitle,
				validatedItem.Description, validatedItem.Revision, validatedItem.TrashedAt, validatedItem.UpdatedAt,
				validatedItem.ID, validatedItem.CatalogID, expectedItemRevision)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				return classifyRevisionFailure(ctx, tx, "library_catalog_items", validatedItem.ID, validatedItem.CatalogID)
			}
		} else {
			var storedCatalogID string
			if err := tx.QueryRowContext(ctx, `
SELECT catalog_id FROM library_catalog_items WHERE id = ?
`, itemID).Scan(&storedCatalogID); err != nil {
				return err
			}
			if storedCatalogID != catalogID {
				return sql.ErrNoRows
			}
		}

		validateRepresentation := func(entry library.MetadataEntry) error {
			if entry.RepresentationID == "" {
				return nil
			}
			var representationCatalogID, representationItemID string
			if err := tx.QueryRowContext(ctx, `
SELECT catalog_id, item_id FROM library_representations WHERE id = ?
`, entry.RepresentationID).Scan(&representationCatalogID, &representationItemID); err != nil {
				return err
			}
			if representationCatalogID != catalogID || representationItemID != itemID {
				return sql.ErrNoRows
			}
			return nil
		}
		for _, mutation := range validatedUpserts {
			if err := validateRepresentation(mutation.item); err != nil {
				return err
			}
			row := toMetadataEntryRow(mutation.item)
			if mutation.expected == 0 {
				if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
					var existingID string
					if lookupErr := tx.QueryRowContext(ctx, `
SELECT id FROM library_metadata_entries WHERE id = ?
`, mutation.item.ID).Scan(&existingID); lookupErr == nil {
						return library.ErrCatalogRevisionConflict
					}
					return err
				}
			} else {
				result, err := tx.NewUpdate().Model(&row).
					Column(
						"namespace", "key", "value_type", "value_json", "language", "position",
						"source", "provenance", "confidence", "locked", "revision", "updated_at",
					).
					Where("id = ?", mutation.item.ID).
					Where("catalog_id = ?", mutation.item.CatalogID).
					Where("item_id = ?", mutation.item.ItemID).
					Where("COALESCE(representation_id, '') = ?", mutation.item.RepresentationID).
					Where("revision = ?", mutation.expected).
					Exec(ctx)
				if err != nil {
					return err
				}
				affected, err := result.RowsAffected()
				if err != nil {
					return err
				}
				if affected == 0 {
					return classifyRevisionFailure(ctx, tx, "library_metadata_entries", mutation.item.ID, mutation.item.CatalogID)
				}
			}
			if _, err := appendCatalogChange(ctx, tx, mutation.change); err != nil {
				return err
			}
		}
		for _, mutation := range validatedDeletes {
			if err := validateRepresentation(mutation.item); err != nil {
				return err
			}
			result, err := tx.NewDelete().
				Model((*catalogMetadataEntryRow)(nil)).
				Where("id = ?", mutation.item.ID).
				Where("catalog_id = ?", mutation.item.CatalogID).
				Where("item_id = ?", mutation.item.ItemID).
				Where("revision = ?", mutation.item.Revision).
				Exec(ctx)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				return classifyRevisionFailure(ctx, tx, "library_metadata_entries", mutation.item.ID, mutation.item.CatalogID)
			}
			if _, err := appendCatalogChange(ctx, tx, mutation.change); err != nil {
				return err
			}
		}
		// The aggregate Item change is deliberately last: any client stopping at
		// that cursor has already consumed every nested metadata change committed
		// by this request. When the Item itself did not change, append one owning
		// Item invalidation in the same final position instead.
		if validatedItem != nil {
			_, err := appendCatalogChange(ctx, tx, itemChange)
			return err
		}
		if len(validatedUpserts) > 0 || len(validatedDeletes) > 0 {
			return appendOwningItemInvalidation(ctx, tx, catalogID, itemID, actorID, latestChangeAt)
		}
		return nil
	})
}

func (repo *SQLiteCatalogMutationRepository) SaveUserStateMutation(
	ctx context.Context,
	item library.UserState,
	expectedRevision int64,
	actorID string,
) error {
	validated, err := library.NewUserState(library.UserStateParams{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, UserID: item.UserID,
		Favorite: item.Favorite, Rating: item.Rating, Progress: item.Progress, PositionMs: item.PositionMs,
		Locator: item.Locator, Completed: item.Completed, Revision: item.Revision,
		LastOpenedAt: item.LastOpenedAt, CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil || expectedRevision < 0 || validated.Revision != expectedRevision+1 {
		return library.ErrInvalidUserState
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: validated.CatalogID, EntityType: library.CatalogEntityUserState,
		EntityID: validated.ID, Kind: library.CatalogChangeUpsert, Revision: validated.Revision,
		ActorID: strings.TrimSpace(actorID), OccurredAt: validated.UpdatedAt,
	})
	if err != nil {
		return err
	}
	row := catalogUserStateRow{
		ID: validated.ID, CatalogID: validated.CatalogID, ItemID: validated.ItemID, UserID: validated.UserID,
		Favorite: validated.Favorite, Rating: validated.Rating, Progress: validated.Progress,
		PositionMs: validated.PositionMs, Locator: validated.Locator, Completed: validated.Completed,
		Revision: validated.Revision, LastOpenedAt: validated.LastOpenedAt,
		CreatedAt: validated.CreatedAt, UpdatedAt: validated.UpdatedAt,
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var itemCatalogID string
		if err := tx.QueryRowContext(ctx, "SELECT catalog_id FROM library_catalog_items WHERE id = ?", validated.ItemID).Scan(&itemCatalogID); err != nil {
			return err
		}
		if itemCatalogID != validated.CatalogID {
			return sql.ErrNoRows
		}
		if expectedRevision == 0 {
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				var existingID string
				lookupErr := tx.QueryRowContext(ctx, `
SELECT id FROM library_user_states
WHERE id = ? OR (catalog_id = ? AND item_id = ? AND user_id = ?)
LIMIT 1
`, validated.ID, validated.CatalogID, validated.ItemID, validated.UserID).Scan(&existingID)
				if lookupErr == nil {
					return library.ErrCatalogRevisionConflict
				}
				return err
			}
		} else {
			result, err := tx.NewUpdate().Model(&row).
				Column("favorite", "rating", "progress", "position_ms", "locator", "completed", "revision", "last_opened_at", "updated_at").
				Where("id = ?", validated.ID).
				Where("catalog_id = ?", validated.CatalogID).
				Where("revision = ?", expectedRevision).
				Exec(ctx)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				return classifyRevisionFailure(ctx, tx, "library_user_states", validated.ID, validated.CatalogID)
			}
		}
		_, err := appendCatalogChange(ctx, tx, change)
		return err
	})
}

func (repo *SQLiteCatalogMutationRepository) SaveCollectionMutation(
	ctx context.Context,
	item library.Collection,
	expectedRevision int64,
	actorID string,
) error {
	validated, err := validateCollectionMutation(item, expectedRevision, actorID)
	if err != nil {
		return err
	}
	row := toCatalogCollectionRow(validated.item)
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if expectedRevision == 0 {
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				var existingID string
				if lookupErr := tx.QueryRowContext(ctx, "SELECT id FROM library_collections WHERE id = ?", validated.item.ID).Scan(&existingID); lookupErr == nil {
					return library.ErrCatalogRevisionConflict
				}
				return err
			}
		} else if err := updateCollectionMutation(ctx, tx, validated.item, expectedRevision); err != nil {
			return err
		}
		_, err := appendCatalogChange(ctx, tx, validated.change)
		return err
	})
}

func (repo *SQLiteCatalogMutationRepository) ReplaceCollectionItemsMutation(
	ctx context.Context,
	item library.Collection,
	members []library.CollectionItem,
	expectedRevision int64,
	actorID string,
) error {
	if expectedRevision <= 0 {
		return library.ErrInvalidCollection
	}
	validated, err := validateCollectionMutation(item, expectedRevision, actorID)
	if err != nil {
		return err
	}
	rows := make([]catalogCollectionItemRow, 0, len(members))
	seenItems := make(map[string]struct{}, len(members))
	for position, member := range members {
		normalized, buildErr := library.NewCollectionItem(member.ID, member.CollectionID, member.ItemID, member.Position, member.AddedAt)
		if buildErr != nil || normalized.CollectionID != validated.item.ID || normalized.Position != position {
			return library.ErrInvalidCollectionItem
		}
		if _, duplicate := seenItems[normalized.ItemID]; duplicate {
			return library.ErrInvalidCollectionItem
		}
		seenItems[normalized.ItemID] = struct{}{}
		rows = append(rows, catalogCollectionItemRow{
			ID: normalized.ID, CollectionID: normalized.CollectionID, ItemID: normalized.ItemID,
			Position: normalized.Position, AddedAt: normalized.AddedAt,
		})
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := updateCollectionMutation(ctx, tx, validated.item, expectedRevision); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*catalogCollectionItemRow)(nil)).Where("collection_id = ?", validated.item.ID).Exec(ctx); err != nil {
			return err
		}
		if len(rows) > 0 {
			if _, err := tx.NewInsert().Model(&rows).Exec(ctx); err != nil {
				return err
			}
		}
		_, err := appendCatalogChange(ctx, tx, validated.change)
		return err
	})
}

// SaveTagMutation stores a tag and advances the tag's change generation in
// the same SQLite transaction. Tags predate revision columns, so the durable
// feed is the source of their per-entity mutation generation.
func (repo *SQLiteCatalogMutationRepository) SaveTagMutation(
	ctx context.Context,
	item library.Tag,
	actorID string,
) error {
	validated, err := library.NewTag(library.TagParams{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name,
		CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil {
		return err
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: validated.CatalogID, EntityType: library.CatalogEntityTag,
		EntityID: validated.ID, Kind: library.CatalogChangeUpsert, Revision: 1,
		ActorID: strings.TrimSpace(actorID), OccurredAt: validated.UpdatedAt,
	})
	if err != nil {
		return err
	}
	row := catalogTagRow{
		ID: validated.ID, CatalogID: validated.CatalogID, Name: validated.Name,
		NormalizedName: validated.NormalizedName, CreatedAt: validated.CreatedAt, UpdatedAt: validated.UpdatedAt,
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var existingCatalogID string
		err := tx.QueryRowContext(ctx, "SELECT catalog_id FROM library_tags WHERE id = ?", validated.ID).Scan(&existingCatalogID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				return err
			}
		case err != nil:
			return err
		case existingCatalogID != validated.CatalogID:
			return sql.ErrNoRows
		default:
			result, err := tx.NewUpdate().Model(&row).
				Column("name", "normalized_name", "updated_at").
				Where("id = ?", validated.ID).
				Where("catalog_id = ?", validated.CatalogID).
				Exec(ctx)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				return sql.ErrNoRows
			}
		}
		// The entity write above acquires SQLite's single-writer lock before
		// MAX(revision)+1 is allocated by the INSERT statement. Concurrent
		// successful mutations therefore cannot observe or commit the same
		// generation even though the transaction begins in deferred mode.
		_, err = appendCatalogChangeNextRevision(ctx, tx, change)
		return err
	})
}

// ReplaceItemTagsMutation treats the complete membership set as one sync
// aggregate. Its stable change entity ID is the owning item ID; binding IDs
// remain private storage details and an empty set is still an upsert.
func (repo *SQLiteCatalogMutationRepository) ReplaceItemTagsMutation(
	ctx context.Context,
	catalogID string,
	itemID string,
	members []library.ItemTag,
	actorID string,
	occurredAt time.Time,
) error {
	catalogID = strings.TrimSpace(catalogID)
	itemID = strings.TrimSpace(itemID)
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: catalogID, EntityType: library.CatalogEntityItemTag,
		EntityID: itemID, Kind: library.CatalogChangeUpsert, Revision: 1,
		ActorID: strings.TrimSpace(actorID), OccurredAt: occurredAt,
	})
	if err != nil {
		return library.ErrInvalidItemTag
	}
	rows := make([]catalogItemTagRow, 0, len(members))
	seenIDs := make(map[string]struct{}, len(members))
	seenTags := make(map[string]struct{}, len(members))
	for _, member := range members {
		validated, buildErr := library.NewItemTag(member.ID, member.ItemID, member.TagID, member.AddedBy, member.AddedAt)
		if buildErr != nil || validated.ItemID != itemID {
			return library.ErrInvalidItemTag
		}
		if _, duplicate := seenIDs[validated.ID]; duplicate {
			return library.ErrInvalidItemTag
		}
		if _, duplicate := seenTags[validated.TagID]; duplicate {
			return library.ErrInvalidItemTag
		}
		seenIDs[validated.ID] = struct{}{}
		seenTags[validated.TagID] = struct{}{}
		rows = append(rows, catalogItemTagRow{
			ID: validated.ID, ItemID: validated.ItemID, TagID: validated.TagID,
			AddedBy: validated.AddedBy, AddedAt: validated.AddedAt,
		})
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var itemCatalogID string
		if err := tx.QueryRowContext(ctx, "SELECT catalog_id FROM library_catalog_items WHERE id = ?", itemID).Scan(&itemCatalogID); err != nil {
			return err
		}
		if itemCatalogID != catalogID {
			return sql.ErrNoRows
		}
		for _, row := range rows {
			var tagCatalogID string
			if err := tx.QueryRowContext(ctx, "SELECT catalog_id FROM library_tags WHERE id = ?", row.TagID).Scan(&tagCatalogID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return library.ErrInvalidItemTag
				}
				return err
			}
			if tagCatalogID != catalogID {
				return library.ErrInvalidItemTag
			}
		}
		// DELETE is deliberately issued before allocating the revision. Even
		// for an already-empty set it starts a write transaction, serializing
		// the following MAX(revision)+1 INSERT with concurrent replacements.
		if _, err := tx.NewDelete().Model((*catalogItemTagRow)(nil)).Where("item_id = ?", itemID).Exec(ctx); err != nil {
			return err
		}
		if len(rows) > 0 {
			if _, err := tx.NewInsert().Model(&rows).Exec(ctx); err != nil {
				return err
			}
		}
		if _, err := appendCatalogChangeNextRevision(ctx, tx, change); err != nil {
			return err
		}
		return appendOwningItemInvalidation(ctx, tx, catalogID, itemID, strings.TrimSpace(actorID), occurredAt)
	})
}

type owningItemChangeExecutor interface {
	catalogChangeExecutor
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// appendOwningItemInvalidation makes nested catalog mutations consumable by
// public clients. The nested change remains an internal audit record, while
// this durable aggregate change tells clients which public Item detail to
// refetch. Both records share the entity transaction, so a cursor can never
// observe one without the other.
func appendOwningItemInvalidation(
	ctx context.Context,
	executor owningItemChangeExecutor,
	catalogID string,
	itemID string,
	actorID string,
	occurredAt time.Time,
) error {
	var storedCatalogID, status string
	var revision int64
	if err := executor.QueryRowContext(ctx, `
SELECT catalog_id, status, revision
FROM library_catalog_items
WHERE id = ?
`, strings.TrimSpace(itemID)).Scan(&storedCatalogID, &status, &revision); err != nil {
		return err
	}
	if storedCatalogID != strings.TrimSpace(catalogID) {
		return sql.ErrNoRows
	}
	kind := library.CatalogChangeUpsert
	if library.ItemStatus(status) == library.ItemStatusTrashed {
		kind = library.CatalogChangeDelete
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: storedCatalogID, EntityType: library.CatalogEntityItem,
		EntityID: strings.TrimSpace(itemID), Kind: kind, Revision: revision,
		ActorID: strings.TrimSpace(actorID), OccurredAt: occurredAt,
	})
	if err != nil {
		return err
	}
	_, err = appendCatalogChange(ctx, executor, change)
	return err
}

// appendCatalogChangeNextRevision allocates and appends a non-revisioned
// aggregate's next generation in one SQL statement. Callers must acquire the
// transaction's SQLite writer lock by mutating the aggregate first.
func appendCatalogChangeNextRevision(
	ctx context.Context,
	executor catalogChangeExecutor,
	item library.CatalogChange,
) (int64, error) {
	result, err := executor.ExecContext(ctx, `
INSERT INTO library_catalog_changes (
  catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
)
SELECT ?, ?, ?, ?, COALESCE(MAX(revision), 0) + 1, ?, ?
FROM library_catalog_changes
WHERE catalog_id = ? AND entity_type = ? AND entity_id = ?
`, item.CatalogID, item.EntityType, item.EntityID, item.Kind, item.ActorID, item.OccurredAt.UTC(),
		item.CatalogID, item.EntityType, item.EntityID)
	if err != nil {
		return 0, err
	}
	sequence, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read appended catalog change sequence: %w", err)
	}
	if sequence <= 0 {
		return 0, errors.New("sqlite returned a non-positive catalog change sequence")
	}
	return sequence, nil
}

type validatedCollectionMutation struct {
	item   library.Collection
	change library.CatalogChange
}

func validateCollectionMutation(item library.Collection, expectedRevision int64, actorID string) (validatedCollectionMutation, error) {
	validated, err := library.NewCollection(library.CollectionParams{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, Description: item.Description,
		Kind: string(item.Kind), SmartQuery: item.SmartQuery, Revision: item.Revision,
		CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil || expectedRevision < 0 || validated.Revision != expectedRevision+1 {
		return validatedCollectionMutation{}, library.ErrInvalidCollection
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: validated.CatalogID, EntityType: library.CatalogEntityCollection,
		EntityID: validated.ID, Kind: library.CatalogChangeUpsert, Revision: validated.Revision,
		ActorID: strings.TrimSpace(actorID), OccurredAt: validated.UpdatedAt,
	})
	if err != nil {
		return validatedCollectionMutation{}, err
	}
	return validatedCollectionMutation{item: validated, change: change}, nil
}

func updateCollectionMutation(ctx context.Context, tx bun.Tx, item library.Collection, expectedRevision int64) error {
	result, err := tx.ExecContext(ctx, `
UPDATE library_collections SET
  name = ?, description = ?, kind = ?, smart_query = ?, revision = ?, updated_at = ?
WHERE id = ? AND catalog_id = ? AND revision = ?
`, item.Name, item.Description, item.Kind, item.SmartQuery, item.Revision, item.UpdatedAt,
		item.ID, item.CatalogID, expectedRevision)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return classifyRevisionFailure(ctx, tx, "library_collections", item.ID, item.CatalogID)
	}
	return nil
}

func classifyRevisionFailure(ctx context.Context, tx bun.Tx, table, id, catalogID string) error {
	var existingCatalogID string
	query := fmt.Sprintf("SELECT catalog_id FROM %s WHERE id = ?", table)
	err := tx.QueryRowContext(ctx, query, id).Scan(&existingCatalogID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && existingCatalogID != catalogID) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	return library.ErrCatalogRevisionConflict
}

var _ library.CatalogMutationRepository = (*SQLiteCatalogMutationRepository)(nil)
var _ library.CatalogProfessionalMutationRepository = (*SQLiteCatalogMutationRepository)(nil)
