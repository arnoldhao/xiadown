package persistence

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLibraryStorageRootEmojiMigrationChecksumIsImmutable(t *testing.T) {
	const shippedChecksum = "8011eb08acecb7d4641e675312d6b9d009d2e361b753f9ff2808b5dffcc3dcde"

	for _, migration := range sqliteMigrations {
		if migration.version != 34 {
			continue
		}
		if checksum := migration.checksum(); checksum != shippedChecksum {
			t.Fatalf(
				"Library storage root emoji checksum=%q, want shipped checksum %q",
				checksum,
				shippedChecksum,
			)
		}
		return
	}
	t.Fatal("Library storage root emoji migration v34 is missing")
}

func TestLibraryStorageRootMigrationUpgradesV31WithoutRebuildingLibrary(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "library-storage-root-v31.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("open current database: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('library-existing', 'Existing Library', '{}', '2026-07-26T00:00:00Z', '2026-07-26T00:00:00Z');
INSERT INTO library_files (
  id, library_id, kind, name,
  storage_mode, storage_local_path,
  origin_kind, origin_import_batch_id, origin_import_path, origin_imported_at,
  state_json, created_at, updated_at
) VALUES (
  'file-existing', 'library-existing', 'video', 'existing.mp4',
  'local_path', '/legacy/existing.mp4',
  'import', 'batch-existing', '/legacy/existing.mp4', '2026-07-26T00:00:00Z',
  '{}', '2026-07-26T00:00:00Z', '2026-07-26T00:00:00Z'
);
INSERT INTO library_catalogs (
  id, name, status, is_default, created_at, updated_at
) VALUES (
  'catalog-existing', 'Library', 'active', TRUE,
  '2026-07-26T00:00:00Z', '2026-07-26T00:00:00Z'
);
INSERT INTO library_storage_roots (
  id, catalog_id, name, path, mode, status, created_at, updated_at
) VALUES (
  'root-existing', 'catalog-existing', 'Existing Root', '/legacy',
  'referenced', 'online', '2026-07-26T00:00:00Z', '2026-07-26T00:00:00Z'
);
DROP TRIGGER library_storage_roots_detach_files_before_delete;
DROP INDEX library_files_storage_root_idx;
DROP INDEX library_storage_roots_one_default_idx;
ALTER TABLE library_files DROP COLUMN storage_relative_path;
ALTER TABLE library_files DROP COLUMN storage_root_id;
ALTER TABLE library_storage_roots DROP COLUMN is_default;
ALTER TABLE library_storage_roots DROP COLUMN emoji;
DELETE FROM schema_migrations WHERE version >= 32;
PRAGMA user_version = 31;
`); err != nil {
		database.Close()
		t.Fatalf("prepare v31 fixture: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close v31 fixture: %v", err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade v31 fixture: %v", err)
	}
	defer upgraded.Close()
	if upgraded.MigrationSnapshotPath == "" {
		t.Fatal("v31 upgrade did not create a pre-migration snapshot")
	}

	var (
		version     int
		libraryRows int
		fileRows    int
		rootRows    int
		columns     int
		isDefault   bool
		emoji       string
		rootID      sql.NullString
		relative    sql.NullString
	)
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT
  (SELECT COUNT(*) FROM library_libraries WHERE id = 'library-existing'),
  (SELECT COUNT(*) FROM library_files WHERE id = 'file-existing'),
  (SELECT COUNT(*) FROM library_storage_roots WHERE id = 'root-existing'),
  (SELECT COUNT(*) FROM pragma_table_info('library_files')
   WHERE name IN ('storage_root_id', 'storage_relative_path'))
`).Scan(&libraryRows, &fileRows, &rootRows, &columns); err != nil {
		t.Fatalf("inspect upgraded records: %v", err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT is_default, emoji FROM library_storage_roots WHERE id = 'root-existing'
`).Scan(&isDefault, &emoji); err != nil {
		t.Fatalf("read upgraded root: %v", err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT storage_root_id, storage_relative_path
FROM library_files
WHERE id = 'file-existing'
`).Scan(&rootID, &relative); err != nil {
		t.Fatalf("read upgraded file ownership: %v", err)
	}
	if version != latestSQLiteMigrationVersion() ||
		libraryRows != 1 || fileRows != 1 || rootRows != 1 || columns != 2 ||
		isDefault || emoji == "" || rootID.Valid || relative.Valid {
		t.Fatalf(
			"migration rebuilt or rewrote existing Library: version=%d library=%d file=%d root=%d columns=%d default=%t rootID=%#v relative=%#v",
			version,
			libraryRows,
			fileRows,
			rootRows,
			columns,
			isDefault,
			rootID,
			relative,
		)
	}
}
