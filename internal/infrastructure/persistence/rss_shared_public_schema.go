package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const rssSharedPublicColumnsSQL = `
ALTER TABLE rss_subscriptions ADD COLUMN source_access TEXT NOT NULL DEFAULT 'desktopManaged'
  CHECK (source_access IN ('desktopManaged','sharedPublic'));
ALTER TABLE rss_subscriptions ADD COLUMN public_feed_url TEXT NOT NULL DEFAULT '';
`

const rssSharedPublicObjectsSQL = `
CREATE INDEX IF NOT EXISTS rss_subscriptions_source_access_idx
  ON rss_subscriptions(source_access, enabled, updated_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS rss_subscriptions_public_feed_url_unique_idx
  ON rss_subscriptions(public_feed_url)
  WHERE source_access = 'sharedPublic';

CREATE TRIGGER IF NOT EXISTS rss_subscriptions_source_access_insert_guard
BEFORE INSERT ON rss_subscriptions
WHEN (NEW.source_access = 'sharedPublic' AND trim(NEW.public_feed_url) = '')
  OR (NEW.source_access = 'desktopManaged' AND trim(NEW.public_feed_url) <> '')
BEGIN
  SELECT RAISE(ABORT, 'invalid RSS subscription source access');
END;

CREATE TRIGGER IF NOT EXISTS rss_subscriptions_source_access_update_guard
BEFORE UPDATE OF source_access, public_feed_url ON rss_subscriptions
WHEN (NEW.source_access = 'sharedPublic' AND trim(NEW.public_feed_url) = '')
  OR (NEW.source_access = 'desktopManaged' AND trim(NEW.public_feed_url) <> '')
BEGIN
  SELECT RAISE(ABORT, 'invalid RSS subscription source access');
END;

CREATE TABLE IF NOT EXISTS rss_entry_origins (
  subscription_id TEXT NOT NULL,
  origin_key TEXT NOT NULL,
  entry_id TEXT NOT NULL,
  last_observed_at TIMESTAMP NOT NULL,
  PRIMARY KEY (subscription_id, origin_key),
  FOREIGN KEY (subscription_id) REFERENCES rss_subscriptions(id) ON DELETE CASCADE,
  FOREIGN KEY (entry_id) REFERENCES rss_entries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS rss_entry_origins_entry_idx
  ON rss_entry_origins(entry_id);

CREATE TABLE IF NOT EXISTS rss_subscription_field_revisions (
  subscription_id TEXT NOT NULL,
  field_name TEXT NOT NULL
    CHECK (field_name IN ('title','viewType','categoryId','sortOrder','enabled','sourceAccess','publicFeedURL')),
  revision INTEGER NOT NULL CHECK (revision > 0),
  PRIMARY KEY (subscription_id, field_name),
  FOREIGN KEY (subscription_id) REFERENCES rss_subscriptions(id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO rss_subscription_field_revisions (subscription_id, field_name, revision)
SELECT id, 'title', revision FROM rss_subscriptions
UNION ALL SELECT id, 'viewType', revision FROM rss_subscriptions
UNION ALL SELECT id, 'categoryId', revision FROM rss_subscriptions
UNION ALL SELECT id, 'sortOrder', revision FROM rss_subscriptions
UNION ALL SELECT id, 'enabled', revision FROM rss_subscriptions
UNION ALL SELECT id, 'sourceAccess', revision FROM rss_subscriptions
UNION ALL SELECT id, 'publicFeedURL', revision FROM rss_subscriptions;

CREATE TABLE IF NOT EXISTS rss_observation_sources (
  subscription_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  upstream_etag TEXT NOT NULL DEFAULT '',
  last_modified TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL,
  fetched_at TIMESTAMP NOT NULL,
  accepted_at TIMESTAMP NOT NULL,
  PRIMARY KEY (subscription_id, device_id),
  FOREIGN KEY (subscription_id) REFERENCES rss_subscriptions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rss_public_mutations (
  device_id TEXT NOT NULL,
  mutation_id TEXT NOT NULL,
  mutation_kind TEXT NOT NULL CHECK (mutation_kind IN ('subscription','observation')),
  request_hash TEXT NOT NULL,
  result_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  PRIMARY KEY (device_id, mutation_id)
);

CREATE INDEX IF NOT EXISTS rss_public_mutations_created_idx
  ON rss_public_mutations(created_at DESC);

CREATE TABLE IF NOT EXISTS rss_fetch_leases (
  subscription_id TEXT PRIMARY KEY,
  lease_id TEXT NOT NULL UNIQUE,
  device_id TEXT NOT NULL,
  acquired_at TIMESTAMP NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  FOREIGN KEY (subscription_id) REFERENCES rss_subscriptions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS rss_fetch_leases_expiry_idx
  ON rss_fetch_leases(expires_at);

CREATE TRIGGER IF NOT EXISTS rss_subscriptions_shared_public_cleanup
AFTER UPDATE OF source_access, public_feed_url ON rss_subscriptions
WHEN NEW.source_access <> 'sharedPublic' OR NEW.public_feed_url <> OLD.public_feed_url
BEGIN
  DELETE FROM rss_fetch_leases WHERE subscription_id = NEW.id;
  DELETE FROM rss_observation_sources WHERE subscription_id = NEW.id;
END;
`

const rssSharedPublicSchemaSQL = rssSharedPublicColumnsSQL + rssSharedPublicObjectsSQL

func applyRSSSharedPublicSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin RSS shared-public protocol schema: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyRSSSharedPublicSchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit RSS shared-public protocol schema: %w", err)
	}
	return nil
}

func applyRSSSharedPublicSchemaTx(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []struct {
		name      string
		statement string
	}{
		{"source_access", `ALTER TABLE rss_subscriptions ADD COLUMN source_access TEXT NOT NULL DEFAULT 'desktopManaged' CHECK (source_access IN ('desktopManaged','sharedPublic'))`},
		{"public_feed_url", `ALTER TABLE rss_subscriptions ADD COLUMN public_feed_url TEXT NOT NULL DEFAULT ''`},
	} {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('rss_subscriptions') WHERE name = ?`, column.name).Scan(&exists); err != nil {
			return fmt.Errorf("inspect RSS subscription %s: %w", column.name, err)
		}
		if exists == 0 {
			if _, err := tx.ExecContext(ctx, column.statement); err != nil {
				return fmt.Errorf("add RSS subscription %s: %w", column.name, err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, rssSharedPublicObjectsSQL); err != nil {
		return fmt.Errorf("apply RSS shared-public protocol schema: %w", err)
	}
	return nil
}
