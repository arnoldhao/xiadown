package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRSSSyncV2MigrationUpgradesV11AdditivelyAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-sync-v11.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, enabled, created_at, updated_at, revision
) VALUES (
  'subscription-1', 'rss-default', 'https://example.com/feed.xml', 'Feed', 1,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z', 1
);
INSERT INTO rss_entries (
  id, subscription_id, external_id, title, content_hash, read_at,
  state_revision, created_at, modified_at
) VALUES (
  'entry-1', 'subscription-1', 'external-1', 'Post', 'hash',
  '2026-07-13T01:00:00Z', 2, '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
DROP INDEX rss_changes_scope_sequence_idx;
DROP INDEX rss_subscriptions_workspace_id_idx;
DROP INDEX rss_entries_snapshot_id_idx;
ALTER TABLE rss_sync_state DROP COLUMN retained_from;
ALTER TABLE rss_entries DROP COLUMN article_progress_fraction;
ALTER TABLE rss_entries DROP COLUMN article_progress_anchor;
ALTER TABLE rss_entries DROP COLUMN article_progress_content_revision;
ALTER TABLE rss_entries DROP COLUMN video_progress_seconds;
ALTER TABLE rss_entries DROP COLUMN video_duration_seconds;
ALTER TABLE rss_entries DROP COLUMN video_completed;
ALTER TABLE rss_entries DROP COLUMN read_revision;
ALTER TABLE rss_entries DROP COLUMN starred_revision;
ALTER TABLE rss_entries DROP COLUMN article_progress_revision;
ALTER TABLE rss_entries DROP COLUMN video_progress_seconds_revision;
ALTER TABLE rss_client_mutations DROP COLUMN request_hash;
DELETE FROM schema_migrations WHERE version = 12;
PRAGMA user_version = 11;
`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.MigrationSnapshotPath == "" {
		_ = upgraded.Close()
		t.Fatal("v11 upgrade did not create a pre-migration snapshot")
	}
	var version, entryColumns, syncColumns, mutationColumns int
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('rss_entries') WHERE name IN (
  'article_progress_fraction','article_progress_anchor','article_progress_content_revision',
  'video_progress_seconds','video_duration_seconds','video_completed',
  'read_revision','starred_revision','article_progress_revision','video_progress_seconds_revision'
)
`).Scan(&entryColumns); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('rss_sync_state') WHERE name = 'retained_from'
`).Scan(&syncColumns); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('rss_client_mutations') WHERE name = 'request_hash'
`).Scan(&mutationColumns); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	var readRevision, stateRevision int64
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT read_revision, state_revision FROM rss_entries WHERE id = 'entry-1'
`).Scan(&readRevision, &stateRevision); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if version != sqliteMigrations[len(sqliteMigrations)-1].version || entryColumns != 10 || syncColumns != 1 || mutationColumns != 1 ||
		readRevision != 2 || stateRevision != 2 {
		_ = upgraded.Close()
		t.Fatalf("version=%d entryColumns=%d syncColumns=%d mutationColumns=%d readRevision=%d stateRevision=%d",
			version, entryColumns, syncColumns, mutationColumns, readRevision, stateRevision)
	}
	if err := upgraded.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.MigrationSnapshotPath != "" {
		t.Fatalf("idempotent reopen created migration snapshot %q", reopened.MigrationSnapshotPath)
	}
}
