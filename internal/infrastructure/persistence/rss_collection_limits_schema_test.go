package persistence

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRSSCollectionLimitsMigrationUpgradesVersionSeventeen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-collection-limits-v17.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
DROP TRIGGER rss_collection_subscriptions_max_items;
DROP TRIGGER rss_collection_subscriptions_max_items_update;
DROP TRIGGER rss_collection_entries_max_items;
DROP TRIGGER rss_collection_entries_max_items_update;
DELETE FROM schema_migrations WHERE version = 18;
PRAGMA user_version = 17;
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
		t.Fatal("v17 upgrade did not create a pre-migration snapshot")
	}
	var version, triggers int
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'trigger' AND name IN (
  'rss_collection_subscriptions_max_items', 'rss_collection_subscriptions_max_items_update',
  'rss_collection_entries_max_items', 'rss_collection_entries_max_items_update'
)
`).Scan(&triggers); err != nil {
		t.Fatal(err)
	}
	if version != sqliteMigrations[len(sqliteMigrations)-1].version || triggers != 4 {
		t.Fatalf("version=%d triggers=%d, want latest/4", version, triggers)
	}
}

func TestRSSCollectionLimitsMigrationPreservesExistingOversizedCollectionForRepair(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-collection-limits-oversized-v17.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
DROP TRIGGER rss_collection_subscriptions_max_items;
DROP TRIGGER rss_collection_subscriptions_max_items_update;
DROP TRIGGER rss_collection_entries_max_items;
DROP TRIGGER rss_collection_entries_max_items_update;
DELETE FROM schema_migrations WHERE version = 18;
PRAGMA user_version = 17;

INSERT INTO rss_collections (
  id, workspace_id, title, description, kind, view_type, sort_order,
  created_at, updated_at, revision
) VALUES (
  'oversized-legacy-collection', 'rss-default', 'Oversized legacy collection', '',
  'subscriptions', 'auto', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
);

WITH RECURSIVE numbers(value) AS (
  SELECT 1
  UNION ALL
  SELECT value + 1 FROM numbers WHERE value < 10001
)
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, enabled, created_at, updated_at, revision
)
SELECT printf('oversized-sub-%05d', value), 'rss-default',
       printf('https://example.com/oversized/%d.xml', value),
       printf('Oversized %d', value), 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
FROM numbers;

INSERT INTO rss_collection_subscriptions (collection_id, subscription_id, sort_order, added_at)
SELECT 'oversized-legacy-collection', id,
       CAST(substr(id, 15) AS INTEGER) - 1, CURRENT_TIMESTAMP
FROM rss_subscriptions
WHERE id LIKE 'oversized-sub-%';
`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("v18 migration locked out an oversized legacy collection: %v", err)
	}
	defer upgraded.Close()
	var version, count int
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_collection_subscriptions
WHERE collection_id = 'oversized-legacy-collection'
`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if version != sqliteMigrations[len(sqliteMigrations)-1].version || count != 10001 {
		t.Fatalf("upgraded legacy collection version=%d count=%d, want latest/10001", version, count)
	}
	if _, err := upgraded.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, enabled, created_at, updated_at, revision
) VALUES (
  'oversized-repair-new', 'rss-default', 'https://example.com/oversized/new.xml',
  'Repair target', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
);
INSERT INTO rss_collection_subscriptions (collection_id, subscription_id, sort_order, added_at)
VALUES ('oversized-legacy-collection', 'oversized-repair-new', 10001, CURRENT_TIMESTAMP);
`); err == nil || !strings.Contains(strings.ToLower(err.Error()), "item limit") {
		t.Fatalf("oversized legacy collection accepted further growth: %v", err)
	}
	if _, err := upgraded.SQL.ExecContext(ctx, `
DELETE FROM rss_collection_subscriptions
WHERE collection_id = 'oversized-legacy-collection'
  AND subscription_id IN ('oversized-sub-00001', 'oversized-sub-00002');
INSERT INTO rss_collection_subscriptions (collection_id, subscription_id, sort_order, added_at)
VALUES ('oversized-legacy-collection', 'oversized-repair-new', 9999, CURRENT_TIMESTAMP);
`); err != nil {
		t.Fatalf("oversized legacy collection could not be repaired: %v", err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_collection_subscriptions
WHERE collection_id = 'oversized-legacy-collection'
`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 10000 {
		t.Fatalf("repaired legacy collection count=%d, want 10000", count)
	}
}
