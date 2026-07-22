package persistence

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

func TestRSSSharedPublicV27MigrationPreservesPrivateDescriptorsAndEnforcesSchemaBoundaries(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-v26.db")
	now := time.Date(2026, 7, 21, 11, 0, 0, 0, time.UTC)
	createRSSV26SharedPublicFixture(t, ctx, path, now)

	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade v26 RSS database: %v", err)
	}
	defer database.Close()
	if database.MigrationSnapshotPath == "" {
		t.Fatal("v26 upgrade did not create a pre-migration snapshot")
	}

	var userVersion int
	if err := database.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if userVersion != latestSQLiteMigrationVersion() || userVersion < 27 {
		t.Fatalf("user_version=%d latest=%d, want v27 or newer", userVersion, latestSQLiteMigrationVersion())
	}
	var migrationName string
	if err := database.SQL.QueryRowContext(ctx, `SELECT name FROM schema_migrations WHERE version = 27`).Scan(&migrationName); err != nil {
		t.Fatalf("read v27 migration ledger: %v", err)
	}
	if migrationName != "rss_shared_public_protocol" {
		t.Fatalf("v27 migration name=%q", migrationName)
	}

	var feedURL, sourceAccess, publicFeedURL string
	if err := database.SQL.QueryRowContext(ctx, `
SELECT feed_url, source_access, public_feed_url
FROM rss_subscriptions WHERE id = 'legacy-private-feed'
`).Scan(&feedURL, &sourceAccess, &publicFeedURL); err != nil {
		t.Fatal(err)
	}
	if feedURL != "https://private.example.test/feed.xml?token=secret" || sourceAccess != "desktopManaged" || publicFeedURL != "" {
		t.Fatalf("migrated descriptor feed=%q source=%q public=%q", feedURL, sourceAccess, publicFeedURL)
	}

	if _, err := database.SQL.ExecContext(ctx, `
UPDATE rss_subscriptions SET public_feed_url = 'https://feeds.example.test/forbidden.xml'
WHERE id = 'legacy-private-feed'
`); err == nil {
		t.Fatal("desktopManaged subscription accepted a public_feed_url")
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, source_access, public_feed_url, title,
  created_at, updated_at, revision
) VALUES (
  'missing-public-url', 'rss-default', 'shared-public:missing', 'sharedPublic', '',
  'Missing URL', ?, ?, 1
)
`, now, now); err == nil {
		t.Fatal("sharedPublic subscription accepted an empty public_feed_url")
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, source_access, public_feed_url, title,
  created_at, updated_at, revision
) VALUES (
  'invalid-source-access', 'rss-default', 'shared-public:invalid', 'deviceLocal',
  'https://feeds.example.test/invalid.xml', 'Invalid source', ?, ?, 1
)
`, now, now); err == nil {
		t.Fatal("Desktop schema accepted deviceLocal source_access")
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, source_access, public_feed_url, title,
  created_at, updated_at, revision
) VALUES (
  'shared-feed-one', 'rss-default', 'shared-public:one', 'sharedPublic',
  'https://feeds.example.test/public.xml', 'Shared one', ?, ?, 1
)
`, now, now); err != nil {
		t.Fatalf("insert valid sharedPublic subscription: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, source_access, public_feed_url, title,
  created_at, updated_at, revision
) VALUES (
  'shared-feed-two', 'rss-default', 'shared-public:two', 'sharedPublic',
  'https://feeds.example.test/public.xml', 'Shared two', ?, ?, 1
)
`, now, now); err == nil {
		t.Fatal("schema accepted duplicate shared public_feed_url")
	}

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_public_mutations (
  device_id, mutation_id, mutation_kind, request_hash, result_json, created_at
) VALUES ('device-a', 'bad-kind', 'lease', 'hash', '{}', ?)
`, now); err == nil {
		t.Fatal("rss_public_mutations accepted an unknown mutation_kind")
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO rss_fetch_leases (subscription_id, lease_id, device_id, acquired_at, expires_at)
VALUES ('shared-feed-one', 'lease-one', 'device-a', ?, ?)
`, now, now.Add(time.Minute)); err != nil {
		t.Fatalf("insert fetch lease: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DELETE FROM rss_subscriptions WHERE id = 'shared-feed-one'`); err != nil {
		t.Fatalf("delete leased subscription: %v", err)
	}
	var leaseCount int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM rss_fetch_leases WHERE subscription_id = 'shared-feed-one'`).Scan(&leaseCount); err != nil {
		t.Fatal(err)
	}
	if leaseCount != 0 {
		t.Fatalf("fetch lease rows after subscription delete=%d, want 0", leaseCount)
	}

	foreignKeyRows, err := database.SQL.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer foreignKeyRows.Close()
	if foreignKeyRows.Next() {
		t.Fatal("foreign_key_check returned a violation after v27 migration")
	}
}

func createRSSV26SharedPublicFixture(t *testing.T, ctx context.Context, path string, now time.Time) {
	t.Helper()
	db, err := sqlite3driver.Open(path, configureSQLiteConnection)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, schemaMigrationsSQL); err != nil {
		t.Fatal(err)
	}
	for _, migration := range sqliteMigrations {
		if migration.version >= 27 {
			break
		}
		if err := migration.apply(ctx, db); err != nil {
			t.Fatalf("apply fixture migration %d: %v", migration.version, err)
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO schema_migrations (version, name, checksum, applied_at, duration_ms)
VALUES (?, ?, ?, ?, 0)
`, migration.version, migration.name, migration.checksum(), now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, created_at, updated_at, revision
) VALUES (
  'legacy-private-feed', 'rss-default',
  'https://private.example.test/feed.xml?token=secret', 'Legacy private feed', ?, ?, 3
)
`, now.Add(-time.Hour), now); err != nil {
		t.Fatalf("seed v26 RSS subscription: %v", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", sqliteApplicationID)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA user_version = 26`); err != nil {
		t.Fatal(err)
	}
}
