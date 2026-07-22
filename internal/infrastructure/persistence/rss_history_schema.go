package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const rssHistorySchemaSQL = `
CREATE TABLE IF NOT EXISTS rss_subscription_history (
  subscription_id TEXT PRIMARY KEY,
  cursor_url TEXT NOT NULL DEFAULT '',
  capability TEXT NOT NULL DEFAULT 'unknown'
    CHECK (capability IN ('unknown','available','unsupported')),
  exhausted BOOLEAN NOT NULL DEFAULT 0,
  no_progress_count INTEGER NOT NULL DEFAULT 0
    CHECK (no_progress_count >= 0),
  last_attempt_at TIMESTAMP,
  last_success_at TIMESTAMP,
  last_error TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (subscription_id) REFERENCES rss_subscriptions(id) ON DELETE CASCADE
);
`

func applyRSSHistorySchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin RSS history schema migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, rssHistorySchemaSQL); err != nil {
		return fmt.Errorf("apply RSS history schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit RSS history schema migration: %w", err)
	}
	return nil
}
