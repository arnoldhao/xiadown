package persistence

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteMigratesLibraryFilesCompletedKinds(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data.db")
	rawDB, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	_, err = rawDB.ExecContext(ctx, `
CREATE TABLE library_libraries (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_by_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE TABLE library_files (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
	  kind TEXT NOT NULL CHECK (kind IN ('video','audio','subtitle','thumbnail','transcode','other')),
  name TEXT NOT NULL,
  metadata_json TEXT,
  display_name TEXT,

  storage_mode TEXT NOT NULL CHECK (storage_mode IN ('local_path','db_document','hybrid')),
  storage_local_path TEXT,
  storage_document_id TEXT,

  origin_kind TEXT NOT NULL CHECK (origin_kind IN ('import','download','transcode')),
  origin_operation_id TEXT,
  origin_import_batch_id TEXT,
  origin_import_path TEXT,
  origin_imported_at TIMESTAMP,
  origin_keep_source_file BOOLEAN,

  lineage_root_file_id TEXT,
  latest_operation_id TEXT,

  state_json TEXT NOT NULL,
  media_json TEXT,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,

  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (lineage_root_file_id) REFERENCES library_files(id) ON DELETE SET NULL,

  CHECK (
	    (kind IN ('video','audio','thumbnail','transcode','other') AND storage_mode IN ('local_path','hybrid') AND COALESCE(storage_local_path,'') <> '') OR
    (kind = 'subtitle' AND storage_mode IN ('db_document','hybrid') AND COALESCE(storage_document_id,'') <> '')
  ),
  CHECK (
    (origin_kind = 'import' AND COALESCE(origin_import_path,'') <> '' AND origin_operation_id IS NULL) OR
    (origin_kind IN ('download','transcode') AND COALESCE(origin_operation_id,'') <> '' AND origin_import_path IS NULL)
  )
);
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('lib-1', 'Library', '{}', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
INSERT INTO library_files (
  id,
  library_id,
  kind,
  name,
  metadata_json,
  display_name,
  storage_mode,
  storage_local_path,
  storage_document_id,
  origin_kind,
  origin_operation_id,
  origin_import_batch_id,
  origin_import_path,
  origin_imported_at,
  origin_keep_source_file,
  lineage_root_file_id,
  latest_operation_id,
  state_json,
  media_json,
  created_at,
  updated_at
) VALUES (
  'file-video',
  'lib-1',
  'video',
  'video',
  NULL,
  'video',
  'local_path',
  '/tmp/video.mp4',
  NULL,
  'download',
  'op-video',
  NULL,
  NULL,
  NULL,
  NULL,
  NULL,
  'op-video',
  '{"status":"active"}',
  NULL,
  '2026-01-01T00:00:00Z',
  '2026-01-01T00:00:00Z'
);
`)
	if err != nil {
		_ = rawDB.Close()
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw sqlite: %v", err)
	}

	db, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer db.Close()

	for _, kind := range []string{"other", "document", "font", "api", "archive", "manifest"} {
		kind := kind
		_, err = db.SQL.ExecContext(ctx, `
	INSERT INTO library_files (
	  id,
	  library_id,
  kind,
  name,
  display_name,
  storage_mode,
  storage_local_path,
  origin_kind,
  origin_operation_id,
  latest_operation_id,
  state_json,
  created_at,
	  updated_at
	) VALUES (
	  ?,
	  'lib-1',
	  ?,
	  'payload',
	  'payload',
	  'local_path',
  '/tmp/payload.bin',
  'download',
  'op-other',
  'op-other',
	  '{"status":"active"}',
	  '2026-01-01T00:00:00Z',
	  '2026-01-01T00:00:00Z'
	)`, "file-"+kind, kind)
		if err != nil {
			t.Fatalf("insert %s file after migration: %v", kind, err)
		}
	}
}
