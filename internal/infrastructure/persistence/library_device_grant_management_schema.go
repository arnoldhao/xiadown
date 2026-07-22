package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// This migration is intentionally separate from catalogSchemaSQL: migration
// signatures are immutable once shipped, and existing v2 databases must be
// upgraded additively without changing their recorded checksum.
const libraryDeviceGrantManagementSchemaSQL = `
ALTER TABLE library_device_grants
ADD COLUMN revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0);

CREATE INDEX IF NOT EXISTS library_device_grants_status_idx
  ON library_device_grants(catalog_id, status, updated_at DESC, id);
`

func applyLibraryDeviceGrantManagementSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin library device grant management migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, libraryDeviceGrantManagementSchemaSQL); err != nil {
		return fmt.Errorf("apply library device grant management schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit library device grant management migration: %w", err)
	}
	return nil
}
