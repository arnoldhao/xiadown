package persistence

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLibraryHistoryOperationKindMigrationsBackfillPrimaryAndDeletedRowsAndUpgradeOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history-operation-kind-v21.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('library-kind', 'Kinds', '{}', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z');

INSERT INTO library_operations (
  id, library_id, kind, status, display_name, correlation_json, input_json,
  output_json, file_count, created_at
) VALUES (
  'operation-transcode', 'library-kind', 'transcode', 'succeeded', 'Transcode', '{}', '{}',
  '{}', 0, '2026-07-20T00:00:00Z'
);

INSERT INTO library_history_records (
  id, library_id, category, action, display_name, status, source_kind,
  operation_id, subject_operation_id, file_count, occurred_at, created_at, updated_at
) VALUES
  ('history-primary-download', 'library-kind', 'operation', 'download', 'Download', 'succeeded', 'desktop',
   NULL, 'operation-download', 0, '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z'),
  ('history-deleted-download', 'library-kind', 'operation_event', 'operation_deleted', 'Download', 'succeeded', 'user_action',
   NULL, 'operation-download', 0, '2026-07-20T01:00:00Z', '2026-07-20T01:00:00Z', '2026-07-20T01:00:00Z'),
  ('history-primary-transcode', 'library-kind', 'operation', 'transcode', 'Transcode', 'succeeded', 'desktop',
   'operation-transcode', NULL, 0, '2026-07-20T02:00:00Z', '2026-07-20T02:00:00Z', '2026-07-20T02:00:00Z'),
  ('history-deleted-transcode', 'library-kind', 'operation_event', 'operation_deleted', 'Transcode', 'succeeded', 'user_action',
   'operation-transcode', NULL, 0, '2026-07-20T03:00:00Z', '2026-07-20T03:00:00Z', '2026-07-20T03:00:00Z'),
  ('history-deleted-orphan', 'library-kind', 'operation_event', 'operation_deleted', 'Orphan', 'failed', 'user_action',
   NULL, 'operation-orphan', 0, '2026-07-20T04:00:00Z', '2026-07-20T04:00:00Z', '2026-07-20T04:00:00Z');

ALTER TABLE library_history_records DROP COLUMN operation_kind;
DELETE FROM schema_migrations WHERE version IN (22, 23);
PRAGMA user_version = 21;
`); err != nil {
		_ = database.Close()
		t.Fatalf("prepare v21 database: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close v21 database: %v", err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade v21 database: %v", err)
	}
	if upgraded.MigrationSnapshotPath == "" {
		_ = upgraded.Close()
		t.Fatal("v21 upgrade did not create a pre-migration snapshot")
	}
	var version, columns int
	var primaryDownloadKind, deletedDownloadKind, primaryTranscodeKind, deletedTranscodeKind string
	var orphanKind sql.NullString
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('library_history_records') WHERE name = 'operation_kind'
`).Scan(&columns); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT operation_kind FROM library_history_records WHERE id = 'history-primary-download'
`).Scan(&primaryDownloadKind); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT operation_kind FROM library_history_records WHERE id = 'history-deleted-download'
`).Scan(&deletedDownloadKind); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT operation_kind FROM library_history_records WHERE id = 'history-primary-transcode'
`).Scan(&primaryTranscodeKind); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT operation_kind FROM library_history_records WHERE id = 'history-deleted-transcode'
`).Scan(&deletedTranscodeKind); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT operation_kind FROM library_history_records WHERE id = 'history-deleted-orphan'
`).Scan(&orphanKind); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if version != latestSQLiteMigrationVersion() || columns != 1 || primaryDownloadKind != "download" || deletedDownloadKind != "download" ||
		primaryTranscodeKind != "transcode" || deletedTranscodeKind != "transcode" || orphanKind.Valid {
		_ = upgraded.Close()
		t.Fatalf(
			"version=%d columns=%d primaryDownload=%q deletedDownload=%q primaryTranscode=%q deletedTranscode=%q orphan=%+v",
			version, columns, primaryDownloadKind, deletedDownloadKind, primaryTranscodeKind, deletedTranscodeKind, orphanKind,
		)
	}
	if err := upgraded.Close(); err != nil {
		t.Fatalf("close upgraded database: %v", err)
	}

	reopened, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen upgraded database: %v", err)
	}
	defer reopened.Close()
	if reopened.MigrationSnapshotPath != "" {
		t.Fatalf("idempotent reopen created migration snapshot %q", reopened.MigrationSnapshotPath)
	}
}

func TestLibraryHistoryOperationEventKindBackfillUpgradesAnAppliedV22Database(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history-operation-kind-v22.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('library-v22', 'V22', '{}', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z');

INSERT INTO library_history_records (
  id, library_id, category, action, display_name, status, source_kind,
  operation_id, subject_operation_id, file_count, operation_kind,
  occurred_at, created_at, updated_at
) VALUES
  ('history-v22-primary', 'library-v22', 'operation', 'download', 'Download', 'succeeded', 'desktop',
   NULL, 'operation-v22', 0, 'download', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z', '2026-07-20T00:00:00Z'),
  ('history-v22-deleted', 'library-v22', 'operation_event', 'operation_deleted', 'Download', 'succeeded', 'user_action',
   NULL, 'operation-v22', 0, NULL, '2026-07-20T01:00:00Z', '2026-07-20T01:00:00Z', '2026-07-20T01:00:00Z');

DELETE FROM schema_migrations WHERE version = 23;
PRAGMA user_version = 22;
`); err != nil {
		_ = database.Close()
		t.Fatalf("prepare v22 database: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close v22 database: %v", err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade v22 database: %v", err)
	}
	defer upgraded.Close()
	var version int
	var kind string
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT operation_kind FROM library_history_records WHERE id = 'history-v22-deleted'
`).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if version != latestSQLiteMigrationVersion() || kind != "download" {
		t.Fatalf("version=%d deleted kind=%q", version, kind)
	}
}
