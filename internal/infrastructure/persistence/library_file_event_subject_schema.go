package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// File events are immutable audit facts. Keep file_id as the stable subject
// identifier, but deliberately do not attach it to library_files with a
// cascading foreign key: an event must survive a later hard purge, and a Task
// output can need repair after its file row has already disappeared.
const libraryFileEventStableSubjectSchemaSQL = `
DROP TABLE IF EXISTS library_file_events_stable_subject_next;

CREATE TABLE library_file_events_stable_subject_next (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  file_id TEXT NOT NULL,
  operation_id TEXT,
  event_type TEXT NOT NULL,
  detail_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (operation_id) REFERENCES library_operations(id) ON DELETE SET NULL
);

INSERT INTO library_file_events_stable_subject_next (
  id, library_id, file_id, operation_id, event_type, detail_json, created_at
)
SELECT id, library_id, file_id, operation_id, event_type, detail_json, created_at
FROM library_file_events;

DROP TABLE library_file_events;
ALTER TABLE library_file_events_stable_subject_next RENAME TO library_file_events;

CREATE INDEX IF NOT EXISTS library_events_library_created_idx
ON library_file_events(library_id, created_at DESC);
`

func applyLibraryFileEventStableSubjectSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Library file-event subject migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyLibraryFileEventStableSubjectSchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Library file-event subject migration: %w", err)
	}
	return nil
}

func applyLibraryFileEventStableSubjectSchemaTx(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, libraryFileEventStableSubjectSchemaSQL); err != nil {
		return fmt.Errorf("stabilize Library file-event subject: %w", err)
	}
	return nil
}
