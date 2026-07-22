package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// Keep this additive migration separate from rssDiscoverySchemaSQL: v10 has
// already shipped to local databases and its checksum is immutable.
const rssDiscoveryParametersSchemaSQL = `
ALTER TABLE rss_discovery_routes
  ADD COLUMN needs_parameters BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE rss_discovery_routes
  ADD COLUMN parameters_json TEXT NOT NULL DEFAULT '[]';

-- Catalog rows are a rebuildable cache. Existing v10 rows were concrete-only
-- and could otherwise remain fresh for 24 hours without parameter metadata.
DELETE FROM rss_discovery_routes;
DELETE FROM rss_discovery_meta;
`

func applyRSSDiscoveryParametersSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin RSS discovery parameters migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, rssDiscoveryParametersSchemaSQL); err != nil {
		return fmt.Errorf("apply RSS discovery parameters migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit RSS discovery parameters migration: %w", err)
	}
	return nil
}
