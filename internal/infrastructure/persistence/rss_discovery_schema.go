package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const rssDiscoverySchemaSQL = `
CREATE TABLE IF NOT EXISTS rss_discovery_meta (
  source TEXT PRIMARY KEY,
  source_url TEXT NOT NULL DEFAULT '',
  fetched_at TIMESTAMP NOT NULL,
  route_count INTEGER NOT NULL DEFAULT 0 CHECK (route_count >= 0),
  updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS rss_discovery_routes (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL DEFAULT 'rsshub' CHECK (provider IN ('rsshub','rss')),
  title TEXT NOT NULL,
  url TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  source_id TEXT NOT NULL DEFAULT '',
  source_name TEXT NOT NULL DEFAULT '',
  source_url TEXT NOT NULL DEFAULT '',
  site_url TEXT NOT NULL DEFAULT '',
  route_path TEXT NOT NULL,
  example_path TEXT NOT NULL,
  categories_json TEXT NOT NULL DEFAULT '[]',
  heat INTEGER NOT NULL DEFAULT 0 CHECK (heat >= 0),
  language TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  view_type TEXT NOT NULL DEFAULT 'auto'
    CHECK (view_type IN ('auto','article','social','image','video')),
  requires_config BOOLEAN NOT NULL DEFAULT 0,
  requires_puppeteer BOOLEAN NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS rss_discovery_routes_popular_idx
  ON rss_discovery_routes(heat DESC, title COLLATE NOCASE, id);
CREATE INDEX IF NOT EXISTS rss_discovery_routes_language_idx
  ON rss_discovery_routes(language, heat DESC);
`

func applyRSSDiscoverySchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin RSS discovery schema migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, rssDiscoverySchemaSQL); err != nil {
		return fmt.Errorf("apply RSS discovery schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit RSS discovery schema migration: %w", err)
	}
	return nil
}
