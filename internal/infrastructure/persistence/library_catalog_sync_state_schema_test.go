package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLibraryCatalogSyncStateMigrationSeedsAndPersistsEpoch(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog-sync-v7.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (
  'catalog-sync', 'Library', '', 'active', TRUE,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
DROP TRIGGER library_catalog_sync_state_after_catalog_insert;
DROP TABLE library_catalog_sync_state;
DELETE FROM schema_migrations WHERE version = 8;
PRAGMA user_version = 7;
`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.MigrationSnapshotPath == "" {
		_ = upgraded.Close()
		t.Fatal("v7 upgrade did not create a pre-migration snapshot")
	}
	var epoch string
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT epoch FROM library_catalog_sync_state WHERE catalog_id = 'catalog-sync'
`).Scan(&epoch); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if len(epoch) != 32 {
		_ = upgraded.Close()
		t.Fatalf("epoch length = %d", len(epoch))
	}
	if err := upgraded.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var persisted string
	if err := reopened.SQL.QueryRowContext(ctx, `
SELECT epoch FROM library_catalog_sync_state WHERE catalog_id = 'catalog-sync'
`).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if persisted != epoch {
		t.Fatalf("epoch rotated on normal reopen: %q -> %q", epoch, persisted)
	}
}

func TestLibraryCatalogSyncStateTriggerCreatesEpochForNewCatalog(t *testing.T) {
	ctx := context.Background()
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (
  'new-catalog', 'Library', '', 'active', TRUE,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
)
`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_catalog_sync_state
WHERE catalog_id = 'new-catalog'
  AND length(epoch) = 32
  AND epoch NOT GLOB '*[^0-9a-f]*'
`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("sync state count = %d", count)
	}
}
