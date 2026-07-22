package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// Existing validators have no recorded effective-URL provenance and must not
// survive this migration. A validator is safe to reuse only for the exact URL
// that returned it, including path and query (but excluding fragments).
const rssValidatorProvenanceSchemaSQL = `
ALTER TABLE rss_subscriptions
  ADD COLUMN validator_url TEXT NOT NULL DEFAULT '';

UPDATE rss_subscriptions
SET etag = '', last_modified = '', validator_url = '';
`

func applyRSSValidatorProvenanceSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin RSS validator provenance migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, rssValidatorProvenanceSchemaSQL); err != nil {
		return fmt.Errorf("apply RSS validator provenance schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit RSS validator provenance migration: %w", err)
	}
	return nil
}
