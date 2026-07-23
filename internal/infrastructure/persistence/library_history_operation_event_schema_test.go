package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLibraryHistoryOperationEventSchemaPreservesExistingHistoryAndFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: filepath.Join(t.TempDir(), "history-operation-events.db")})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer database.Close()

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('library-events', 'Events', '{}', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z');

INSERT INTO library_operations (
  id, library_id, kind, status, display_name, correlation_json, input_json,
  output_json, file_count, created_at
)
VALUES (
  'operation-events', 'library-events', 'download', 'failed', 'Download', '{}', '{}',
  '{}', 1, '2026-07-20T00:00:00Z'
);

INSERT INTO library_files (
  id, library_id, kind, name, display_name,
  storage_mode, storage_local_path,
  origin_kind, origin_operation_id,
  state_json, created_at, updated_at
)
VALUES (
  'file-events', 'library-events', 'video', 'video.mp4', 'Video',
  'local_path', '/tmp/video.mp4',
  'download', 'operation-events',
  '{"status":"active"}', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z'
);

INSERT INTO library_history_records (
  id, library_id, category, action, display_name, status, source_kind,
  operation_id, subject_operation_id, file_count, occurred_at, created_at, updated_at
)
VALUES (
  'history-primary', 'library-events', 'operation', 'download', 'Download', 'failed', 'desktop',
  'operation-events', 'operation-events', 1,
  '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z'
);

INSERT INTO library_history_files (
  id, history_id, file_id, kind, format, size_bytes, deleted, created_at
)
VALUES (
  'history-file-primary', 'history-primary', 'file-events', 'video', 'mp4', 4096, 0,
  '2026-07-20T00:00:00Z'
);
`); err != nil {
		t.Fatalf("seed History fixture: %v", err)
	}

	// Replaying the table rebuild over populated data exercises the same shape
	// as a v19 database being upgraded to v20.
	if err := applyLibraryHistoryOperationEventSchema(ctx, database.SQL); err != nil {
		t.Fatalf("apply operation-event schema: %v", err)
	}

	var category, subjectOperationID, fileID, format string
	var sizeBytes int64
	if err := database.SQL.QueryRowContext(ctx, `
SELECT history.category, history.subject_operation_id, file.file_id, file.format, file.size_bytes
FROM library_history_records AS history
JOIN library_history_files AS file ON file.history_id = history.id
WHERE history.id = 'history-primary'
`).Scan(&category, &subjectOperationID, &fileID, &format, &sizeBytes); err != nil {
		t.Fatalf("read preserved History: %v", err)
	}
	if category != "operation" || subjectOperationID != "operation-events" || fileID != "file-events" ||
		format != "mp4" || sizeBytes != 4096 {
		t.Fatalf("unexpected preserved History values: category=%q subject=%q file=%q format=%q size=%d", category, subjectOperationID, fileID, format, sizeBytes)
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_history_records (
  id, library_id, category, action, display_name, status, source_kind,
  operation_id, subject_operation_id, file_count, occurred_at, created_at, updated_at
)
VALUES (
  'history-resume-event', 'library-events', 'operation_event', 'operation_resumed',
  'Download', 'failed', 'user_action', 'operation-events', 'operation-events', 1,
  '2026-07-20T01:00:00Z', '2026-07-20T01:00:00Z', '2026-07-20T01:00:00Z'
)
`); err != nil {
		t.Fatalf("insert operation lifecycle event: %v", err)
	}

	var foreignKeyViolations int
	if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyViolations); err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	if foreignKeyViolations != 0 {
		t.Fatalf("History schema rebuild introduced %d foreign-key violations", foreignKeyViolations)
	}
}
