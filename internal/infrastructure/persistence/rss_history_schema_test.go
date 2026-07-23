package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRSSHistoryMigrationUpgradesVersionFourteen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-history-v14.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
DROP TABLE rss_subscription_history;
DELETE FROM schema_migrations WHERE version = 15;
PRAGMA user_version = 14;
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
	defer upgraded.Close()
	if upgraded.MigrationSnapshotPath == "" {
		t.Fatal("v14 upgrade did not create a pre-migration snapshot")
	}
	var version, tables, columns int
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'table' AND name = 'rss_subscription_history'
`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('rss_subscription_history')
WHERE name IN (
  'subscription_id', 'cursor_url', 'capability', 'exhausted',
  'no_progress_count', 'last_attempt_at', 'last_success_at',
  'last_error', 'updated_at'
)
`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if version != sqliteMigrations[len(sqliteMigrations)-1].version || tables != 1 || columns != 9 {
		t.Fatalf("version=%d tables=%d columns=%d", version, tables, columns)
	}
}
