package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// Keep this additive index outside catalogSchemaSQL: that schema is the
// immutable signature of an already-shipped migration. The partial predicate
// keeps trash rows out of the compact index used by public snapshot keysets.
const libraryCatalogSnapshotIndexSchemaSQL = `
CREATE INDEX IF NOT EXISTS library_catalog_items_snapshot_idx
  ON library_catalog_items(catalog_id, id)
  WHERE status <> 'trashed' AND trashed_at IS NULL;
`

func applyLibraryCatalogSnapshotIndexSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Library Catalog snapshot index migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, libraryCatalogSnapshotIndexSchemaSQL); err != nil {
		return fmt.Errorf("apply Library Catalog snapshot index migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Library Catalog snapshot index migration: %w", err)
	}
	return nil
}
