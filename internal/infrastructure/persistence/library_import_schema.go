package persistence

import (
	"context"
	"database/sql"
)

// libraryImportSchemaSQL is the v6 additive migration. It is kept separate
// from the existing catalog migrations, and no legacy table or filesystem
// path is rewritten.
const libraryImportSchemaSQL = `
CREATE TABLE IF NOT EXISTS library_import_batches (
  id TEXT PRIMARY KEY,
  request_key TEXT NOT NULL UNIQUE,
  library_id TEXT NOT NULL,
  mode TEXT NOT NULL CHECK (mode IN ('referenced','copy')),
  managed_root TEXT NOT NULL DEFAULT '',
  hidden_policy TEXT NOT NULL CHECK (hidden_policy IN ('exclude','include')),
  symlink_policy TEXT NOT NULL CHECK (symlink_policy IN ('skip','follow_files')),
  status TEXT NOT NULL CHECK (status IN ('scanning','ready','running','cancelling','cancelled','completed','failed')),
  total_count INTEGER NOT NULL DEFAULT 0 CHECK (total_count >= 0),
  ready_count INTEGER NOT NULL DEFAULT 0 CHECK (ready_count >= 0),
  duplicate_count INTEGER NOT NULL DEFAULT 0 CHECK (duplicate_count >= 0),
  skipped_count INTEGER NOT NULL DEFAULT 0 CHECK (skipped_count >= 0),
  succeeded_count INTEGER NOT NULL DEFAULT 0 CHECK (succeeded_count >= 0),
  failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
  total_bytes INTEGER NOT NULL DEFAULT 0 CHECK (total_bytes >= 0),
  last_error_code TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  cancel_requested BOOLEAN NOT NULL DEFAULT FALSE,
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  CHECK ((mode = 'copy' AND length(trim(managed_root)) > 0) OR
         (mode = 'referenced' AND managed_root = '')),
  CHECK (total_count = ready_count + duplicate_count + skipped_count + succeeded_count + failed_count)
);

CREATE INDEX IF NOT EXISTS library_import_batches_status_idx
  ON library_import_batches(status, updated_at DESC, id);

CREATE TABLE IF NOT EXISTS library_import_candidates (
  id TEXT PRIMARY KEY,
  batch_id TEXT NOT NULL,
  source_path TEXT NOT NULL,
  relative_path TEXT NOT NULL DEFAULT '',
  display_name TEXT NOT NULL,
  extension TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL CHECK (category IN ('video','audio','book','image','other')),
  mime_type TEXT NOT NULL DEFAULT '',
  media_probed BOOLEAN NOT NULL DEFAULT FALSE,
  was_symlink BOOLEAN NOT NULL DEFAULT FALSE,
  size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
  modified_at TIMESTAMP,
  hash_algorithm TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('ready','duplicate','skipped','importing','copied','registered','succeeded','failed','cancelled')),
  duplicate_file_id TEXT,
  duplicate_candidate_id TEXT,
  managed_path TEXT NOT NULL DEFAULT '',
  file_id TEXT,
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (batch_id) REFERENCES library_import_batches(id) ON DELETE CASCADE,
  FOREIGN KEY (duplicate_file_id) REFERENCES library_files(id) ON DELETE RESTRICT,
  FOREIGN KEY (duplicate_candidate_id) REFERENCES library_import_candidates(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE RESTRICT,
  UNIQUE (batch_id, source_path),
  CHECK ((status = 'skipped') OR (hash_algorithm = 'sha256' AND length(content_hash) = 64)),
  CHECK ((status != 'duplicate') OR (duplicate_file_id IS NOT NULL OR duplicate_candidate_id IS NOT NULL)),
  CHECK ((status != 'succeeded') OR file_id IS NOT NULL),
  CHECK ((status != 'failed') OR length(trim(error_message)) > 0)
);

CREATE INDEX IF NOT EXISTS library_import_candidates_batch_idx
  ON library_import_candidates(batch_id, status, source_path);
CREATE INDEX IF NOT EXISTS library_import_candidates_digest_idx
  ON library_import_candidates(size_bytes, content_hash)
  WHERE hash_algorithm = 'sha256';
`

func applyLibraryImportSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, libraryImportSchemaSQL)
	return err
}

// ApplyLibraryImportSchema is exported for composition tests and bootstrap
// work performed after migration v6 is registered. Runtime startup must use
// the migration registry rather than calling this as an ad-hoc schema patch.
func ApplyLibraryImportSchema(ctx context.Context, db *sql.DB) error {
	return applyLibraryImportSchema(ctx, db)
}
