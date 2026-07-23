package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const rssOrganizationSchemaSQL = `
CREATE TABLE rss_categories (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL DEFAULT 'rss-default',
  title TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  FOREIGN KEY (workspace_id) REFERENCES rss_workspaces(id) ON DELETE CASCADE,
  UNIQUE (workspace_id, title COLLATE NOCASE)
);

ALTER TABLE rss_subscriptions
  ADD COLUMN category_id TEXT REFERENCES rss_categories(id) ON DELETE SET NULL;
ALTER TABLE rss_subscriptions
  ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0);

CREATE INDEX rss_categories_order_idx
  ON rss_categories(workspace_id, sort_order, title COLLATE NOCASE, id);
CREATE INDEX rss_subscriptions_category_order_idx
  ON rss_subscriptions(workspace_id, category_id, sort_order, title COLLATE NOCASE, id);

CREATE TABLE rss_collections (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL DEFAULT 'rss-default',
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('subscriptions','entries')),
  view_type TEXT NOT NULL DEFAULT 'auto'
    CHECK (view_type IN ('auto','article','social','image','video')),
  sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  FOREIGN KEY (workspace_id) REFERENCES rss_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX rss_collections_order_idx
  ON rss_collections(workspace_id, sort_order, title COLLATE NOCASE, id);

CREATE TABLE rss_collection_subscriptions (
  collection_id TEXT NOT NULL,
  subscription_id TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
  added_at TIMESTAMP NOT NULL,
  PRIMARY KEY (collection_id, subscription_id),
  FOREIGN KEY (collection_id) REFERENCES rss_collections(id) ON DELETE CASCADE,
  FOREIGN KEY (subscription_id) REFERENCES rss_subscriptions(id) ON DELETE CASCADE
);

CREATE INDEX rss_collection_subscriptions_order_idx
  ON rss_collection_subscriptions(collection_id, sort_order, subscription_id);

CREATE TABLE rss_collection_entries (
  collection_id TEXT NOT NULL,
  entry_id TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
  added_at TIMESTAMP NOT NULL,
  PRIMARY KEY (collection_id, entry_id),
  FOREIGN KEY (collection_id) REFERENCES rss_collections(id) ON DELETE CASCADE,
  FOREIGN KEY (entry_id) REFERENCES rss_entries(id) ON DELETE CASCADE
);

CREATE INDEX rss_collection_entries_order_idx
  ON rss_collection_entries(collection_id, sort_order, entry_id);

CREATE TABLE rss_sources (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL DEFAULT 'rss-default',
  subscription_id TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL CHECK (kind IN ('inbox','notification')),
  handle TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  FOREIGN KEY (workspace_id) REFERENCES rss_workspaces(id) ON DELETE CASCADE,
  FOREIGN KEY (subscription_id) REFERENCES rss_subscriptions(id) ON DELETE CASCADE,
  UNIQUE (workspace_id, kind, handle)
);

CREATE INDEX rss_sources_order_idx
  ON rss_sources(workspace_id, kind, sort_order, handle, id);
`

func applyRSSOrganizationSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, rssOrganizationSchemaSQL); err != nil {
		return fmt.Errorf("apply RSS organization schema: %w", err)
	}
	return nil
}
