package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRSSDiscoveryParametersMigrationUpgradesVersionTenAndInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-discovery-v10.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, enabled, created_at, updated_at, revision
) VALUES (
  'existing-rss', 'rss-default', 'https://example.com/feed.xml', 'Existing', 1,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z', 1
);
INSERT INTO rss_discovery_routes (
  id, provider, title, url, route_path, example_path, categories_json,
  view_type, needs_parameters, parameters_json
) VALUES (
  'legacy-route', 'rsshub', 'Legacy example', 'rsshub://youtube/user/example',
  'youtube/user/:id', 'youtube/user/example', '["multimedia"]',
  'video', 0, '[]'
);
INSERT INTO rss_discovery_meta (source, source_url, fetched_at, route_count, updated_at)
VALUES ('rsshub', 'https://example.com/routes.json', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP);
ALTER TABLE rss_discovery_routes DROP COLUMN parameters_json;
ALTER TABLE rss_discovery_routes DROP COLUMN needs_parameters;
DELETE FROM schema_migrations WHERE version = 11;
PRAGMA user_version = 10;
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
		t.Fatal("v10 upgrade did not create a pre-migration snapshot")
	}
	var existing, discoveryTables, discoveryColumns, cachedRoutes, cachedMeta, version int
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_subscriptions WHERE id = 'existing-rss'`).Scan(&existing); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'table' AND name IN ('rss_discovery_meta','rss_discovery_routes')
`).Scan(&discoveryTables); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('rss_discovery_routes')
WHERE name IN ('needs_parameters','parameters_json')
`).Scan(&discoveryColumns); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_discovery_routes`).Scan(&cachedRoutes); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_discovery_meta`).Scan(&cachedMeta); err != nil {
		t.Fatal(err)
	}
	if existing != 1 || discoveryTables != 2 || discoveryColumns != 2 || cachedRoutes != 0 || cachedMeta != 0 || version != sqliteMigrations[len(sqliteMigrations)-1].version {
		t.Fatalf("existing=%d discoveryTables=%d discoveryColumns=%d cachedRoutes=%d cachedMeta=%d version=%d", existing, discoveryTables, discoveryColumns, cachedRoutes, cachedMeta, version)
	}
}
