package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const listenLocalMusicSyncStateSchemaSQL = `
CREATE TABLE IF NOT EXISTS listen_local_music_sync_state (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  epoch TEXT NOT NULL CHECK (
    length(epoch) = 32 AND epoch NOT GLOB '*[^0-9a-f]*'
  ),
  high_water INTEGER NOT NULL DEFAULT 0 CHECK (high_water >= 0),
  minimum_cursor INTEGER NOT NULL DEFAULT 0 CHECK (minimum_cursor >= 0),
  updated_at TIMESTAMP NOT NULL
);

INSERT OR IGNORE INTO listen_local_music_sync_state (
  id, epoch, high_water, minimum_cursor, updated_at
)
SELECT 1, lower(hex(randomblob(16))), COALESCE(MAX(sequence), 0), 0, CURRENT_TIMESTAMP
FROM listen_local_music_changes;

UPDATE listen_local_music_sync_state
SET high_water = MAX(
      high_water,
      COALESCE((SELECT MAX(sequence) FROM listen_local_music_changes), 0)
    ),
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1;

CREATE TRIGGER IF NOT EXISTS listen_local_music_change_advance_high_water
AFTER INSERT ON listen_local_music_changes
BEGIN
  UPDATE listen_local_music_sync_state
  SET high_water = MAX(high_water, NEW.sequence),
      updated_at = CURRENT_TIMESTAMP
  WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS listen_local_track_before_delete_revision_extension
BEFORE DELETE ON listen_local_tracks
BEGIN
  INSERT INTO listen_local_music_tombstones (
    entity_type, entity_id, revision, content_identity_revision,
    metadata_revision, resource_revision, deleted_at
  ) VALUES (
    'track', OLD.file_id, OLD.revision + 1, OLD.content_identity_revision,
    OLD.metadata_revision, OLD.resource_revision, CURRENT_TIMESTAMP
  )
  ON CONFLICT(entity_type, entity_id) DO UPDATE SET
    revision = MAX(revision, excluded.revision),
    content_identity_revision = excluded.content_identity_revision,
    metadata_revision = excluded.metadata_revision,
    resource_revision = excluded.resource_revision,
    deleted_at = excluded.deleted_at;
END;
`

func applyListenLocalMusicSyncStateSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Listen Local Music sync-state migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyListenLocalMusicSyncStateSchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Listen Local Music sync-state migration: %w", err)
	}
	return nil
}

func applyListenLocalMusicSyncStateSchemaTx(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []struct {
		table     string
		name      string
		statement string
	}{
		{"listen_local_tracks", "metadata_revision", `ALTER TABLE listen_local_tracks ADD COLUMN metadata_revision INTEGER NOT NULL DEFAULT 1 CHECK (metadata_revision > 0)`},
		{"listen_local_tracks", "resource_revision", `ALTER TABLE listen_local_tracks ADD COLUMN resource_revision INTEGER NOT NULL DEFAULT 1 CHECK (resource_revision > 0)`},
		{"listen_local_music_tombstones", "metadata_revision", `ALTER TABLE listen_local_music_tombstones ADD COLUMN metadata_revision INTEGER`},
		{"listen_local_music_tombstones", "resource_revision", `ALTER TABLE listen_local_music_tombstones ADD COLUMN resource_revision INTEGER`},
	} {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, column.table, column.name).Scan(&exists); err != nil {
			return fmt.Errorf("inspect Listen Local Track %s: %w", column.name, err)
		}
		if exists == 0 {
			if _, err := tx.ExecContext(ctx, column.statement); err != nil {
				return fmt.Errorf("add Listen Local Track %s: %w", column.name, err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, listenLocalMusicSyncStateSchemaSQL); err != nil {
		return fmt.Errorf("create Listen Local Music sync state: %w", err)
	}
	return nil
}
