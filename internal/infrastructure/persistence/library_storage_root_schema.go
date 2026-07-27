package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// libraryStorageRootSchemaSQL is deliberately additive. Existing absolute
// paths remain available as a compatibility cache while root-relative
// ownership is backfilled after the configured download directory is known.
const libraryStorageRootSchemaSQL = `
ALTER TABLE library_storage_roots
  ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE
  CHECK (is_default IN (FALSE, TRUE));

ALTER TABLE library_files
  ADD COLUMN storage_root_id TEXT;

ALTER TABLE library_files
  ADD COLUMN storage_relative_path TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS library_storage_roots_one_default_idx
  ON library_storage_roots(catalog_id)
  WHERE is_default = TRUE;

CREATE INDEX IF NOT EXISTS library_files_storage_root_idx
  ON library_files(storage_root_id, storage_relative_path);

CREATE TRIGGER IF NOT EXISTS library_storage_roots_detach_files_before_delete
BEFORE DELETE ON library_storage_roots
FOR EACH ROW
BEGIN
  UPDATE library_files
  SET storage_root_id = NULL,
      storage_relative_path = NULL
  WHERE storage_root_id = OLD.id;
END;
`

func applyLibraryStorageRootSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Library storage root migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyLibraryStorageRootSchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Library storage root migration: %w", err)
	}
	return nil
}

func applyLibraryStorageRootSchemaTx(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []struct {
		table     string
		name      string
		statement string
	}{
		{
			"library_storage_roots", "is_default",
			`ALTER TABLE library_storage_roots ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE CHECK (is_default IN (FALSE, TRUE))`,
		},
		{
			"library_files", "storage_root_id",
			`ALTER TABLE library_files ADD COLUMN storage_root_id TEXT`,
		},
		{
			"library_files", "storage_relative_path",
			`ALTER TABLE library_files ADD COLUMN storage_relative_path TEXT`,
		},
	} {
		var exists int
		if err := tx.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`,
			column.table,
			column.name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("inspect %s.%s: %w", column.table, column.name, err)
		}
		if exists == 0 {
			if _, err := tx.ExecContext(ctx, column.statement); err != nil {
				return fmt.Errorf("add %s.%s: %w", column.table, column.name, err)
			}
		}
	}
	const indexesAndTrigger = `
CREATE UNIQUE INDEX IF NOT EXISTS library_storage_roots_one_default_idx
  ON library_storage_roots(catalog_id)
  WHERE is_default = TRUE;

CREATE INDEX IF NOT EXISTS library_files_storage_root_idx
  ON library_files(storage_root_id, storage_relative_path);

CREATE TRIGGER IF NOT EXISTS library_storage_roots_detach_files_before_delete
BEFORE DELETE ON library_storage_roots
FOR EACH ROW
BEGIN
  UPDATE library_files
  SET storage_root_id = NULL,
      storage_relative_path = NULL
  WHERE storage_root_id = OLD.id;
END;
`
	if _, err := tx.ExecContext(ctx, indexesAndTrigger); err != nil {
		return fmt.Errorf("create Library storage root indexes and trigger: %w", err)
	}
	return nil
}
