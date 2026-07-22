package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRSSOrganizationMigrationUpgradesVersionSixteenWithoutLosingFeeds(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-organization-v16.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, enabled, created_at, updated_at, revision
) VALUES (
  'legacy-feed', 'rss-default', 'https://example.com/legacy.xml', 'Legacy', 1,
  '2026-07-17T00:00:00Z', '2026-07-17T00:00:00Z', 1
);
DROP TABLE rss_sources;
DROP TABLE rss_collection_entries;
DROP TABLE rss_collection_subscriptions;
DROP TABLE rss_collections;
DROP INDEX rss_subscriptions_category_order_idx;
ALTER TABLE rss_subscriptions DROP COLUMN category_id;
ALTER TABLE rss_subscriptions DROP COLUMN sort_order;
DROP TABLE rss_categories;
DELETE FROM schema_migrations WHERE version >= 17;
PRAGMA user_version = 16;
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
		t.Fatal("v16 upgrade did not create a pre-migration snapshot")
	}
	var version, organizationTables, placementColumns, limitTriggers, feedCount, sortOrder int
	var categoryID *string
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'table' AND name IN (
  'rss_categories', 'rss_collections', 'rss_collection_subscriptions',
  'rss_collection_entries', 'rss_sources'
)
`).Scan(&organizationTables); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('rss_subscriptions')
WHERE name IN ('category_id', 'sort_order')
`).Scan(&placementColumns); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'trigger' AND name IN (
  'rss_collection_subscriptions_max_items', 'rss_collection_subscriptions_max_items_update',
  'rss_collection_entries_max_items', 'rss_collection_entries_max_items_update'
)
`).Scan(&limitTriggers); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*), category_id, sort_order FROM rss_subscriptions WHERE id = 'legacy-feed'
`).Scan(&feedCount, &categoryID, &sortOrder); err != nil {
		t.Fatal(err)
	}
	if version != sqliteMigrations[len(sqliteMigrations)-1].version || organizationTables != 5 || placementColumns != 2 || limitTriggers != 4 ||
		feedCount != 1 || categoryID != nil || sortOrder != 0 {
		t.Fatalf("version=%d tables=%d placementColumns=%d limitTriggers=%d feed=(%d,%v,%d)",
			version, organizationTables, placementColumns, limitTriggers, feedCount, categoryID, sortOrder)
	}
}
