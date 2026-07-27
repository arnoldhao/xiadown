package persistence

import (
	"context"
	"database/sql"
)

const libraryRootSyncSchemaSQL = `
CREATE TABLE IF NOT EXISTS library_storage_root_sync_states (
  root_id TEXT PRIMARY KEY,
  status TEXT NOT NULL CHECK (status IN (
    'idle','queued','scanning','watching','cancelling','cancelled','interrupted','failed'
  )),
  generation INTEGER NOT NULL DEFAULT 0 CHECK (generation >= 0),
  full_scan BOOLEAN NOT NULL DEFAULT FALSE CHECK (full_scan IN (FALSE, TRUE)),
  discovered_count INTEGER NOT NULL DEFAULT 0 CHECK (discovered_count >= 0),
  processed_count INTEGER NOT NULL DEFAULT 0 CHECK (processed_count >= 0),
  unchanged_count INTEGER NOT NULL DEFAULT 0 CHECK (unchanged_count >= 0),
  duplicate_count INTEGER NOT NULL DEFAULT 0 CHECK (duplicate_count >= 0),
  missing_count INTEGER NOT NULL DEFAULT 0 CHECK (missing_count >= 0),
  failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
  processed_bytes INTEGER NOT NULL DEFAULT 0 CHECK (processed_bytes >= 0),
  cancel_requested BOOLEAN NOT NULL DEFAULT FALSE CHECK (cancel_requested IN (FALSE, TRUE)),
  watcher_cursor INTEGER NOT NULL DEFAULT 0 CHECK (watcher_cursor >= 0),
  last_error_code TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  last_reconciled_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (root_id) REFERENCES library_storage_roots(id) ON DELETE CASCADE,
  CHECK (processed_count <= discovered_count),
  CHECK ((status = 'failed' AND length(trim(last_error)) > 0) OR
         (status <> 'failed' AND last_error = '' AND last_error_code = ''))
);

CREATE INDEX IF NOT EXISTS library_storage_root_sync_states_status_idx
  ON library_storage_root_sync_states(status, updated_at DESC, root_id);

CREATE TABLE IF NOT EXISTS library_storage_root_sync_entries (
  root_id TEXT NOT NULL,
  relative_path TEXT NOT NULL,
  size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
  modified_unix_nano INTEGER NOT NULL DEFAULT 0 CHECK (modified_unix_nano >= 0),
  content_hash TEXT NOT NULL DEFAULT '',
  file_id TEXT,
  status TEXT NOT NULL CHECK (status IN ('active','duplicate','missing','failed')),
  last_seen_generation INTEGER NOT NULL DEFAULT 0 CHECK (last_seen_generation >= 0),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (root_id, relative_path),
  FOREIGN KEY (root_id) REFERENCES library_storage_roots(id) ON DELETE CASCADE,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE CASCADE,
  CHECK (content_hash = '' OR length(content_hash) = 64),
  CHECK ((status = 'active' AND file_id IS NOT NULL) OR status <> 'active'),
  CHECK ((status = 'failed' AND length(trim(last_error)) > 0) OR
         (status <> 'failed' AND last_error = ''))
);

CREATE INDEX IF NOT EXISTS library_storage_root_sync_entries_digest_idx
  ON library_storage_root_sync_entries(root_id, size_bytes, content_hash)
  WHERE status = 'active' AND length(content_hash) = 64;

CREATE INDEX IF NOT EXISTS library_storage_root_sync_entries_generation_idx
  ON library_storage_root_sync_entries(root_id, last_seen_generation, relative_path);
`

func applyLibraryRootSyncSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, libraryRootSyncSchemaSQL)
	return err
}
