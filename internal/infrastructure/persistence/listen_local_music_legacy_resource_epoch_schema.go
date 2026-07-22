package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// Legacy Listen Local tracks can now expose their canonical local audio and
// artwork as opaque public resources when no managed Catalog representation
// exists. Rotate the public Music generation once so clients that completed
// the preceding projection epoch bootstrap these newly available resources.
// Keep the journal window and every canonical/user-owned row unchanged.
const listenLocalMusicLegacyResourceEpochSchemaSQL = `
UPDATE listen_local_music_sync_state
SET epoch = lower(hex(randomblob(16))),
    minimum_cursor = MIN(minimum_cursor, high_water),
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1;
`

func applyListenLocalMusicLegacyResourceEpochSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Listen Local Music legacy resource epoch migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, listenLocalMusicLegacyResourceEpochSchemaSQL); err != nil {
		return fmt.Errorf("rotate Listen Local Music legacy resource epoch: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Listen Local Music legacy resource epoch migration: %w", err)
	}
	return nil
}
