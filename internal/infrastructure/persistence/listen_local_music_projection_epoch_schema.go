package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// The public Music projection gained canonical resource, membership, state,
// lyric, and compatible-representation semantics after the original sync
// epoch shipped. Clients that had already committed an empty or older
// generation could otherwise see the same epoch/high-water forever and never
// negotiate a fresh snapshot. Rotate exactly once through the migration
// ledger; retain the journal window and all canonical/user data.
const listenLocalMusicProjectionEpochSchemaSQL = `
UPDATE listen_local_music_sync_state
SET epoch = lower(hex(randomblob(16))),
    minimum_cursor = MIN(minimum_cursor, high_water),
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1;
`

func applyListenLocalMusicProjectionEpochSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Listen Local Music projection epoch migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, listenLocalMusicProjectionEpochSchemaSQL); err != nil {
		return fmt.Errorf("rotate Listen Local Music projection epoch: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Listen Local Music projection epoch migration: %w", err)
	}
	return nil
}
