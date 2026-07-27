package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLibraryTransientRootImportCleanupPurgesOnlyMissingOperationTemps(
	t *testing.T,
) {
	ctx := context.Background()
	database, err := OpenSQLite(ctx, SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "transient-cleanup.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	now := "2026-07-27T06:27:30Z"
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{
			query: `
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES ('catalog', 'Library', '', 'active', TRUE, ?, ?)`,
			args: []any{now, now},
		},
		{
			query: `
INSERT INTO library_libraries (
  id, name, created_by_json, created_at, updated_at
) VALUES ('bundle', 'Imported', '{}', ?, ?)`,
			args: []any{now, now},
		},
		{
			query: `
INSERT INTO library_storage_roots (
  id, catalog_id, name, path, volume_id, mode, status, last_error,
  created_at, updated_at, is_default, emoji
) VALUES (
  'root', 'catalog', 'Downloads', '/library', 'volume', 'managed',
  'online', '', ?, ?, TRUE, ''
)`,
			args: []any{now, now},
		},
		{
			query: `
INSERT INTO library_storage_root_sync_states (
  root_id, status, generation, full_scan, discovered_count,
  processed_count, unchanged_count, duplicate_count, missing_count,
  failed_count, processed_bytes, cancel_requested, watcher_cursor,
  last_error_code, last_error, created_at, updated_at
) VALUES (
  'root', 'watching', 1, FALSE, 2, 2, 0, 0, 2, 0, 0, FALSE, 0,
  '', '', ?, ?
)`,
			args: []any{now, now},
		},
	} {
		if _, err := database.SQL.ExecContext(
			ctx,
			statement.query,
			statement.args...,
		); err != nil {
			t.Fatal(err)
		}
	}

	seed := func(
		fileID string,
		itemID string,
		assetID string,
		relativePath string,
	) {
		t.Helper()
		localPath := filepath.Join("/library", filepath.FromSlash(relativePath))
		statements := []struct {
			query string
			args  []any
		}{
			{
				query: `
INSERT INTO library_files (
  id, library_id, kind, name, metadata_json, display_name,
  storage_mode, storage_local_path, origin_kind,
  origin_import_batch_id, origin_import_path, origin_imported_at,
  origin_keep_source_file, state_json, media_json, created_at, updated_at
) VALUES (?, 'bundle', 'audio', ?, '{"Title":"Track"}', 'Track',
  'local_path', ?, 'import', 'batch', ?, ?, TRUE,
  '{"Status":"active","Deleted":false,"Archived":false,"LastError":"","LastChecked":""}',
  '{"Format":"mp3","AudioCodec":"mp3"}', ?, ?)`,
				args: []any{
					fileID,
					relativePath,
					localPath,
					localPath,
					now,
					now,
					now,
				},
			},
			{
				query: `
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, description,
  subtype, metadata_json, revision, created_at, updated_at
) VALUES (?, 'catalog', 'audio', 'active', 'Track', 'Track', '',
  '', '{}', 1, ?, ?)`,
				args: []any{itemID, now, now},
			},
			{
				query: `
INSERT INTO library_item_assets (
  id, item_id, file_id, role, label, position, created_at, updated_at
) VALUES (?, ?, ?, 'original', 'Original', 0, ?, ?)`,
				args: []any{assetID, itemID, fileID, now, now},
			},
			{
				query: `
INSERT INTO library_storage_root_sync_entries (
  root_id, relative_path, size_bytes, modified_unix_nano, content_hash,
  file_id, status, last_seen_generation, last_error, created_at, updated_at
) VALUES ('root', ?, 10, 1, '', ?, 'missing', 1, '', ?, ?)`,
				args: []any{relativePath, fileID, now, now},
			},
			{
				query: `
INSERT INTO listen_local_tracks (
  file_id, library_id, local_path, title, format, audio_codec,
  mod_time_unix, availability, last_checked_at, created_at, updated_at
) VALUES (?, 'bundle', ?, 'Track', 'mp3', 'mp3', 1, 'missing', ?, ?, ?)`,
				args: []any{fileID, localPath, now, now, now},
			},
		}
		for _, statement := range statements {
			if _, err := database.SQL.ExecContext(
				ctx,
				statement.query,
				statement.args...,
			); err != nil {
				t.Fatal(err)
			}
		}
	}

	seed(
		"transient-file",
		"transient-item",
		"transient-asset",
		"track.999d1c0d-f524-4dee-b2a2-703ef73ca8c9.tmp.mp3",
	)
	seed(
		"ordinary-file",
		"ordinary-item",
		"ordinary-asset",
		"track.not-a-uuid.tmp.mp3",
	)

	tx, err := database.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyLibraryTransientRootImportCleanupTx(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var status string
	var deleted bool
	if err := database.SQL.QueryRowContext(ctx, `
SELECT json_extract(state_json, '$.Status'),
       json_extract(state_json, '$.Deleted')
FROM library_files
WHERE id = 'transient-file'
`).Scan(&status, &deleted); err != nil {
		t.Fatal(err)
	}
	if status != "purged" || !deleted {
		t.Fatalf("transient file status=%q deleted=%t", status, deleted)
	}

	var itemStatus string
	if err := database.SQL.QueryRowContext(
		ctx,
		"SELECT status FROM library_catalog_items WHERE id = 'transient-item'",
	).Scan(&itemStatus); err != nil {
		t.Fatal(err)
	}
	if itemStatus != "trashed" {
		t.Fatalf("transient item status=%q", itemStatus)
	}

	for _, table := range []string{
		"library_storage_root_sync_entries",
		"listen_local_tracks",
	} {
		var count int
		if err := database.SQL.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+table+" WHERE file_id = 'transient-file'",
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s retained %d transient row(s)", table, count)
		}
	}

	var ordinaryStatus string
	if err := database.SQL.QueryRowContext(ctx, `
SELECT json_extract(state_json, '$.Status')
FROM library_files
WHERE id = 'ordinary-file'
`).Scan(&ordinaryStatus); err != nil {
		t.Fatal(err)
	}
	if ordinaryStatus != "active" {
		t.Fatalf("ordinary temporary name status=%q", ordinaryStatus)
	}
	var deleteChanges int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM library_catalog_changes
WHERE entity_type = 'item'
  AND entity_id = 'transient-item'
  AND kind = 'delete'
`).Scan(&deleteChanges); err != nil {
		t.Fatal(err)
	}
	if deleteChanges != 1 {
		t.Fatalf("catalog delete changes=%d, want 1", deleteChanges)
	}
}
