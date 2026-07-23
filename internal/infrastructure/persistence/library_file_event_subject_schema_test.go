package persistence

import (
	"context"
	"testing"
)

func TestLibraryFileEventStableSubjectSurvivesFilePurge(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openLatestSQLiteTestDatabase(t)
	defer database.Close()

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('library-events', 'Events', '{}', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z');

INSERT INTO library_files (
  id, library_id, kind, name, storage_mode, storage_local_path,
  origin_kind, origin_import_path, state_json, created_at, updated_at
)
VALUES (
  'file-events', 'library-events', 'thumbnail', 'stamp.webp', 'local_path', '/tmp/stamp.webp',
  'import', '/tmp/stamp.webp',
  '{"status":"active"}', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z'
);

INSERT INTO library_file_events (
  id, library_id, file_id, event_type, detail_json, created_at
)
VALUES (
  'event-before-purge', 'library-events', 'file-events', 'file_imported', '{}',
  '2026-07-20T00:00:00Z'
);
`); err != nil {
		t.Fatalf("seed file event: %v", err)
	}

	// Replaying the rebuild proves that an upgraded database preserves events.
	if err := applyLibraryFileEventStableSubjectSchema(ctx, database.SQL); err != nil {
		t.Fatalf("apply stable-subject schema: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DELETE FROM library_files WHERE id = 'file-events'`); err != nil {
		t.Fatalf("purge file row: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_file_events (
  id, library_id, file_id, event_type, detail_json, created_at
)
VALUES (
  'event-after-purge', 'library-events', 'file-events', 'operation_output_detached', '{}',
  '2026-07-20T01:00:00Z'
)
`); err != nil {
		t.Fatalf("append event for purged subject: %v", err)
	}

	var eventCount int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_file_events WHERE file_id = 'file-events'
`).Scan(&eventCount); err != nil {
		t.Fatalf("count durable events: %v", err)
	}
	if eventCount != 2 {
		t.Fatalf("file purge retained %d events, want 2", eventCount)
	}

	var foreignKeyViolations int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&foreignKeyViolations); err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	if foreignKeyViolations != 0 {
		t.Fatalf("stable-subject rebuild introduced %d foreign-key violations", foreignKeyViolations)
	}
}
