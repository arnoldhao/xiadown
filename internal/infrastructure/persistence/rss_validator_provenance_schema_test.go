package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRSSValidatorProvenanceMigrationClearsLegacyValidators(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-validator-v12.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, enabled, etag, last_modified,
  created_at, updated_at, revision
) VALUES (
  'legacy-validator', 'rss-default', 'https://example.com/feed.xml', 'Feed', 1,
  '"legacy-etag"', 'Mon, 13 Jul 2026 10:00:00 GMT',
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z', 1
);
ALTER TABLE rss_subscriptions DROP COLUMN validator_url;
DELETE FROM schema_migrations WHERE version = 13;
PRAGMA user_version = 12;
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
		t.Fatal("v12 upgrade did not create a pre-migration snapshot")
	}
	var version, validatorColumns int
	var etag, lastModified, validatorURL string
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('rss_subscriptions') WHERE name = 'validator_url'
`).Scan(&validatorColumns); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT etag, last_modified, validator_url FROM rss_subscriptions WHERE id = 'legacy-validator'
`).Scan(&etag, &lastModified, &validatorURL); err != nil {
		_ = upgraded.Close()
		t.Fatal(err)
	}
	if version != sqliteMigrations[len(sqliteMigrations)-1].version || validatorColumns != 1 || etag != "" || lastModified != "" || validatorURL != "" {
		_ = upgraded.Close()
		t.Fatalf("version=%d columns=%d etag=%q lastModified=%q validatorURL=%q",
			version, validatorColumns, etag, lastModified, validatorURL)
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
