package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// Keep this additive index separate from rssSchemaSQL: v9 has already shipped
// to local databases and its checksum is immutable.
const rssStarredIndexSchemaSQL = `
CREATE INDEX IF NOT EXISTS rss_entries_starred_unread_idx
  ON rss_entries(read_at, published_at DESC, created_at DESC)
  WHERE starred_at IS NOT NULL;
`

func applyRSSStarredIndexSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin RSS starred index migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, rssStarredIndexSchemaSQL); err != nil {
		return fmt.Errorf("apply RSS starred index migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit RSS starred index migration: %w", err)
	}
	return nil
}
