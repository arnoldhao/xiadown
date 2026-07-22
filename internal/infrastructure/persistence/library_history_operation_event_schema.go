package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// SQLite cannot alter a CHECK constraint in place. Rebuild the two History
// tables together so their foreign-key relationship remains intact while the
// category vocabulary grows to include immutable operation lifecycle events.
const libraryHistoryOperationEventSchemaSQL = `
DROP TABLE IF EXISTS library_history_files_operation_event_next;
DROP TABLE IF EXISTS library_history_records_operation_event_next;

CREATE TABLE library_history_records_operation_event_next (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  category TEXT NOT NULL CHECK (category IN ('operation','operation_event','import')),
  action TEXT NOT NULL,
  display_name TEXT NOT NULL,
  status TEXT NOT NULL,

  source_kind TEXT NOT NULL,
  source_caller TEXT,
  source_run_id TEXT,
  source_actor TEXT,

  operation_id TEXT,
  subject_operation_id TEXT,
  import_batch_id TEXT,

  file_count INTEGER NOT NULL DEFAULT 0,
  total_size_bytes INTEGER,
  duration_ms INTEGER,

  import_path TEXT,
  keep_source_file BOOLEAN,
  error_code TEXT,
  error_message TEXT,

  occurred_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,

  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (operation_id) REFERENCES library_operations(id) ON DELETE SET NULL
);

CREATE TABLE library_history_files_operation_event_next (
  id TEXT PRIMARY KEY,
  history_id TEXT NOT NULL,
  file_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  format TEXT,
  size_bytes INTEGER,
  deleted BOOLEAN NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (history_id) REFERENCES library_history_records_operation_event_next(id) ON DELETE CASCADE,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE CASCADE,
  UNIQUE (history_id, file_id)
);

INSERT INTO library_history_records_operation_event_next (
  id, library_id, category, action, display_name, status,
  source_kind, source_caller, source_run_id, source_actor,
  operation_id, subject_operation_id, import_batch_id,
  file_count, total_size_bytes, duration_ms,
  import_path, keep_source_file, error_code, error_message,
  occurred_at, created_at, updated_at
)
SELECT
  id, library_id, category, action, display_name, status,
  source_kind, source_caller, source_run_id, source_actor,
  operation_id, subject_operation_id, import_batch_id,
  file_count, total_size_bytes, duration_ms,
  import_path, keep_source_file, error_code, error_message,
  occurred_at, created_at, updated_at
FROM library_history_records;

INSERT INTO library_history_files_operation_event_next (
  id, history_id, file_id, kind, format, size_bytes, deleted, created_at
)
SELECT id, history_id, file_id, kind, format, size_bytes, deleted, created_at
FROM library_history_files;

DROP TABLE library_history_files;
DROP TABLE library_history_records;
ALTER TABLE library_history_records_operation_event_next RENAME TO library_history_records;
ALTER TABLE library_history_files_operation_event_next RENAME TO library_history_files;

CREATE INDEX IF NOT EXISTS library_history_library_occurred_idx
ON library_history_records(library_id, occurred_at DESC);
`

func applyLibraryHistoryOperationEventSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Library History operation-event migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyLibraryHistoryOperationEventSchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Library History operation-event migration: %w", err)
	}
	return nil
}

func applyLibraryHistoryOperationEventSchemaTx(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, libraryHistoryOperationEventSchemaSQL); err != nil {
		return fmt.Errorf("expand Library History operation-event category: %w", err)
	}
	return nil
}
