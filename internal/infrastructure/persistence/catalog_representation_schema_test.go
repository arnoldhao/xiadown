package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

func TestExistingV4DatabaseUpgradesRepresentationMetadataWithoutMovingFiles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog-v4.db")
	legacyPath := filepath.Join(t.TempDir(), "movie.mp4")
	legacyUpdatedAt := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	createCatalogV4Fixture(t, ctx, path, legacyPath, legacyUpdatedAt)

	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade v4 database: %v", err)
	}
	defer database.Close()
	if database.MigrationSnapshotPath == "" {
		t.Fatal("v4 upgrade did not create a pre-migration snapshot")
	}

	var userVersion int
	if err := database.SQL.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	wantVersion := sqliteMigrations[len(sqliteMigrations)-1].version
	if userVersion != wantVersion {
		t.Fatalf("user_version=%d, want %d", userVersion, wantVersion)
	}
	var representationMigrationCount int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM schema_migrations
WHERE version = 5 AND name = 'library_representation_metadata_foundation'
`).Scan(&representationMigrationCount); err != nil {
		t.Fatal(err)
	}
	if representationMigrationCount != 1 {
		t.Fatal("v5 representation metadata migration is missing from the ledger")
	}
	var localPath string
	if err := database.SQL.QueryRowContext(ctx, "SELECT storage_local_path FROM library_files WHERE id = 'file-1'").Scan(&localPath); err != nil {
		t.Fatal(err)
	}
	if localPath != legacyPath {
		t.Fatalf("migration moved physical reference: got %q want %q", localPath, legacyPath)
	}

	var kind, purpose, container, codec, availability string
	var width, height int
	var durationMs, bitrateBps, sizeBytes int64
	if err := database.SQL.QueryRowContext(ctx, `
SELECT kind, purpose, container, codec, width, height, duration_ms, bitrate_bps, size_bytes, availability
FROM library_representations WHERE id = 'asset-1'
`).Scan(&kind, &purpose, &container, &codec, &width, &height, &durationMs, &bitrateBps, &sizeBytes, &availability); err != nil {
		t.Fatalf("read migrated representation: %v", err)
	}
	if kind != "original" || purpose != "primary" || container != "mp4" || codec != "h264" ||
		width != 1920 || height != 1080 || durationMs != 90_000 || bitrateBps != 8_000_000 ||
		sizeBytes != 42_000_000 || availability != "available" {
		t.Fatalf("unexpected migrated representation: kind=%s purpose=%s container=%s codec=%s %dx%d duration=%d bitrate=%d size=%d availability=%s",
			kind, purpose, container, codec, width, height, durationMs, bitrateBps, sizeBytes, availability)
	}

	var metadataCount int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_metadata_entries WHERE item_id = 'item-1'
`).Scan(&metadataCount); err != nil {
		t.Fatal(err)
	}
	if metadataCount != 2 {
		t.Fatalf("metadata projection count=%d, want item + file entries", metadataCount)
	}
	var invalidMetadata int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_metadata_entries
WHERE source != 'migration' OR revision != 1 OR NOT json_valid(value_json) OR length(trim(provenance)) = 0
`).Scan(&invalidMetadata); err != nil {
		t.Fatal(err)
	}
	if invalidMetadata != 0 {
		t.Fatalf("migration produced %d invalid metadata rows", invalidMetadata)
	}

	var changes, tombstones int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_changes").Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_tombstones").Scan(&tombstones); err != nil {
		t.Fatal(err)
	}
	if changes != 1 || tombstones != 1 {
		t.Fatalf("change history was not preserved: changes=%d tombstones=%d", changes, tombstones)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_catalog_changes (catalog_id, entity_type, entity_id, kind, revision, occurred_at)
VALUES ('catalog-1', 'representation', 'asset-1', 'upsert', 1, ?),
       ('catalog-1', 'metadata_entry', 'metadata-1', 'upsert', 1, ?)
`, legacyUpdatedAt, legacyUpdatedAt); err != nil {
		t.Fatalf("expanded change entity types rejected: %v", err)
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_files (
  id, library_id, kind, name, metadata_json, display_name,
  storage_mode, storage_local_path, origin_kind, origin_import_batch_id,
  origin_import_path, origin_imported_at, origin_keep_source_file,
  state_json, media_json, created_at, updated_at
) VALUES (
  'file-2', 'legacy-1', 'transcode', 'movie-mobile.mp4', NULL, 'Mobile',
  'local_path', '/unchanged/movie-mobile.mp4', 'import', 'batch-2',
  '/unchanged/movie-mobile.mp4', ?, FALSE,
  '{"Status":"active"}', '{"Format":"mp4","Codec":"","VideoCodec":"h265"}', ?, ?
)`, legacyUpdatedAt, legacyUpdatedAt, legacyUpdatedAt); err != nil {
		t.Fatalf("insert post-upgrade file: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_item_assets (
  id, item_id, file_id, role, label, position, created_at, updated_at
) VALUES ('asset-2', 'item-1', 'file-2', 'representation', 'Mobile', 0, ?, ?)
`, legacyUpdatedAt, legacyUpdatedAt); err != nil {
		t.Fatalf("insert post-upgrade item asset: %v", err)
	}
	var generatedKind, generatedCodec string
	if err := database.SQL.QueryRowContext(ctx, "SELECT kind, codec FROM library_representations WHERE id = 'asset-2'").Scan(&generatedKind, &generatedCodec); err != nil {
		t.Fatalf("future item asset did not get representation: %v", err)
	}
	if generatedKind != "optimized" || generatedCodec != "h265" {
		t.Fatalf("future representation kind=%q codec=%q, want optimized/h265", generatedKind, generatedCodec)
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_representations (
  id, catalog_id, item_id, asset_id, kind, purpose, availability, revision, created_at, updated_at
) VALUES ('bad-representation', 'catalog-1', 'missing-item', 'asset-1', 'original', 'primary', 'available', 1, ?, ?)
`, legacyUpdatedAt, legacyUpdatedAt); err == nil {
		t.Fatal("cross-item representation unexpectedly passed foreign keys")
	}
}

func createCatalogV4Fixture(t *testing.T, ctx context.Context, path, legacyPath string, now time.Time) {
	t.Helper()
	db, err := sqlite3driver.Open(path, configureSQLiteConnection)
	if err != nil {
		t.Fatalf("open v4 fixture: %v", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, schemaMigrationsSQL); err != nil {
		t.Fatal(err)
	}
	for _, migration := range sqliteMigrations[:4] {
		if err := migration.apply(ctx, db); err != nil {
			t.Fatalf("apply fixture migration %d: %v", migration.version, err)
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO schema_migrations (version, name, checksum, applied_at, duration_ms)
VALUES (?, ?, ?, ?, 0)
`, migration.version, migration.name, migration.checksum(), now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", sqliteApplicationID)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA user_version = 4"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('legacy-1', 'Legacy', '{}', ?, ?)
`, now, now); err != nil {
		t.Fatalf("seed v4 legacy library: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_catalogs (id, name, description, status, is_default, created_at, updated_at)
VALUES ('catalog-1', 'Library', '', 'active', TRUE, ?, ?)
`, now, now); err != nil {
		t.Fatalf("seed v4 catalog: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_files (
  id, library_id, kind, name, metadata_json, display_name,
  storage_mode, storage_local_path, origin_kind, origin_import_batch_id,
  origin_import_path, origin_imported_at, origin_keep_source_file,
  state_json, media_json, created_at, updated_at
) VALUES (
  'file-1', 'legacy-1', 'video', 'movie.mp4', '{"Title":"Movie","Author":"Director"}', 'Movie',
  'local_path', ?, 'import', 'batch-1', ?, ?, FALSE,
  '{"Status":"active","Deleted":false}',
  '{"Format":"mp4","VideoCodec":"h264","DurationMs":90000,"Width":1920,"Height":1080,"BitrateKbps":8000,"SizeBytes":42000000}',
  ?, ?
)`, legacyPath, legacyPath, now, now, now); err != nil {
		t.Fatalf("seed v4 file: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, description, subtype, metadata_json,
  revision, created_at, updated_at
) VALUES (
  'item-1', 'catalog-1', 'video', 'active', 'Movie', 'Movie', '', 'movie', '{"year":2026}',
  1, ?, ?
)`, now, now); err != nil {
		t.Fatalf("seed v4 item: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_item_assets (
  id, item_id, file_id, role, label, position, created_at, updated_at
) VALUES ('asset-1', 'item-1', 'file-1', 'original', 'Original', 0, ?, ?)
`, now, now); err != nil {
		t.Fatalf("seed v4 asset: %v", err)
	}
	result, err := db.ExecContext(ctx, `
INSERT INTO library_catalog_changes (
  catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
) VALUES ('catalog-1', 'item', 'item-1', 'delete', 1, 'fixture', ?)
`, now)
	if err != nil {
		t.Fatalf("seed v4 change: %v", err)
	}
	sequence, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read v4 change sequence: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_catalog_tombstones (
  sequence, catalog_id, entity_type, entity_id, revision, deleted_at, expires_at
) VALUES (?, 'catalog-1', 'item', 'old-item', 1, ?, ?)
`, sequence, now, now.Add(24*time.Hour)); err != nil {
		t.Fatalf("seed v4 tombstone: %v", err)
	}
	if err := db.Close(); err != nil && err != sql.ErrConnDone {
		t.Fatal(err)
	}
}
