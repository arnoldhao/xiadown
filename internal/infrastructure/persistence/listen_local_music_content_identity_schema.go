package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const listenLocalMusicContentIdentitySchemaSQL = `
ALTER TABLE listen_local_tracks
  ADD COLUMN content_identity_signature TEXT NOT NULL DEFAULT ''
  CHECK (length(content_identity_signature) <= 128);
`

func applyListenLocalMusicContentIdentitySchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Listen Local Music content-identity migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyListenLocalMusicContentIdentitySchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Listen Local Music content-identity migration: %w", err)
	}
	return nil
}

func applyListenLocalMusicContentIdentitySchemaTx(ctx context.Context, tx *sql.Tx) error {
	var columns int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM pragma_table_info('listen_local_tracks')
WHERE name = 'content_identity_signature'
`).Scan(&columns); err != nil {
		return fmt.Errorf("inspect Listen Local Music content identity: %w", err)
	}
	switch columns {
	case 0:
		if _, err := tx.ExecContext(ctx, listenLocalMusicContentIdentitySchemaSQL); err != nil {
			return fmt.Errorf("add Listen Local Music content identity: %w", err)
		}
	case 1:
		// Idempotent for development databases that applied the schema before a
		// migration-ledger transaction completed.
	default:
		return fmt.Errorf("Listen Local Music content identity has %d columns", columns)
	}
	return nil
}
