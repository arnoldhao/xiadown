package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const rssSchemaSQL = `
CREATE TABLE IF NOT EXISTS rss_workspaces (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  owner_subject_id TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

INSERT OR IGNORE INTO rss_workspaces (id, catalog_id, owner_subject_id, created_at, updated_at)
VALUES ('rss-default', 'default', 'rss-owner', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

CREATE TABLE IF NOT EXISTS rss_sync_state (
  workspace_id TEXT PRIMARY KEY,
  epoch TEXT NOT NULL,
  rotated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (workspace_id) REFERENCES rss_workspaces(id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO rss_sync_state (workspace_id, epoch, rotated_at)
VALUES ('rss-default', lower(hex(randomblob(16))), CURRENT_TIMESTAMP);

CREATE TABLE IF NOT EXISTS rss_subscriptions (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL DEFAULT 'rss-default',
  feed_url TEXT NOT NULL UNIQUE,
  site_url TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  icon_url TEXT NOT NULL DEFAULT '',
  view_type TEXT NOT NULL DEFAULT 'auto'
    CHECK (view_type IN ('auto','article','social','image','video')),
  enabled BOOLEAN NOT NULL DEFAULT 1,
  etag TEXT NOT NULL DEFAULT '',
  last_modified TEXT NOT NULL DEFAULT '',
  last_fetched_at TIMESTAMP,
  last_success_at TIMESTAMP,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  FOREIGN KEY (workspace_id) REFERENCES rss_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS rss_subscriptions_updated_idx
  ON rss_subscriptions(updated_at DESC, title COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS rss_entries (
  id TEXT PRIMARY KEY,
  subscription_id TEXT NOT NULL,
  external_id TEXT NOT NULL,
  url TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  author TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  content_html TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT 'article'
    CHECK (kind IN ('article','social','image','video')),
  image_urls_json TEXT NOT NULL DEFAULT '[]',
  media_json TEXT NOT NULL DEFAULT '[]',
  media_url TEXT NOT NULL DEFAULT '',
  media_type TEXT NOT NULL DEFAULT '',
  thumbnail_url TEXT NOT NULL DEFAULT '',
  platform TEXT NOT NULL DEFAULT '',
  platform_video_id TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMP,
  source_updated_at TIMESTAMP,
  read_at TIMESTAMP,
  starred_at TIMESTAMP,
  state_revision INTEGER NOT NULL DEFAULT 0 CHECK (state_revision >= 0),
  read_state_updated_at TIMESTAMP,
  read_state_device_id TEXT NOT NULL DEFAULT '',
  read_state_subject_id TEXT NOT NULL DEFAULT 'rss-owner',
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  content_hash TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  modified_at TIMESTAMP NOT NULL,
  FOREIGN KEY (subscription_id) REFERENCES rss_subscriptions(id) ON DELETE CASCADE,
  UNIQUE (subscription_id, external_id)
);

CREATE INDEX IF NOT EXISTS rss_entries_feed_published_idx
  ON rss_entries(subscription_id, published_at DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS rss_entries_kind_published_idx
  ON rss_entries(kind, published_at DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS rss_entries_unread_idx
  ON rss_entries(subscription_id, read_at, published_at DESC);

CREATE TABLE IF NOT EXISTS rss_changes (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  workspace_id TEXT NOT NULL DEFAULT 'rss-default',
  subject_id TEXT,
  entity_type TEXT NOT NULL CHECK (entity_type IN ('subscription','entry','entry_state','download')),
  entity_id TEXT NOT NULL,
  operation TEXT NOT NULL CHECK (operation IN ('upsert','delete')),
  revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
  payload_json TEXT NOT NULL DEFAULT '{}',
  changed_at TIMESTAMP NOT NULL,
  FOREIGN KEY (workspace_id) REFERENCES rss_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS rss_changes_entity_idx
  ON rss_changes(entity_type, entity_id, sequence DESC);

CREATE TABLE IF NOT EXISTS rss_tombstones (
  workspace_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  deleted_sequence INTEGER NOT NULL,
  deleted_at TIMESTAMP NOT NULL,
  PRIMARY KEY (workspace_id, entity_type, entity_id),
  FOREIGN KEY (workspace_id) REFERENCES rss_workspaces(id) ON DELETE CASCADE,
  FOREIGN KEY (deleted_sequence) REFERENCES rss_changes(sequence) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rss_entry_downloads (
  id TEXT PRIMARY KEY,
  entry_id TEXT NOT NULL,
  operation_id TEXT NOT NULL UNIQUE,
  library_item_id TEXT,
  state TEXT NOT NULL CHECK (state IN ('queued','running','succeeded','failed','canceled')),
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (entry_id) REFERENCES rss_entries(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rss_client_mutations (
  device_id TEXT NOT NULL,
  mutation_id TEXT NOT NULL,
  entry_id TEXT NOT NULL,
  result_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  PRIMARY KEY (device_id, mutation_id),
  FOREIGN KEY (entry_id) REFERENCES rss_entries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS rss_client_mutations_created_idx
  ON rss_client_mutations(created_at DESC);
`

func applyRSSSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin RSS schema migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, rssSchemaSQL); err != nil {
		return fmt.Errorf("apply RSS schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit RSS schema migration: %w", err)
	}
	return nil
}
