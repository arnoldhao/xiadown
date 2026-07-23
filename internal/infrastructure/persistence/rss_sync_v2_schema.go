package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// Keep this additive migration separate from rssSchemaSQL. The v9 migration
// signature is immutable for databases that already contain the RSS station.
const rssSyncV2SchemaSQL = `
ALTER TABLE rss_sync_state
  ADD COLUMN retained_from INTEGER NOT NULL DEFAULT 0 CHECK (retained_from >= 0);

ALTER TABLE rss_entries
  ADD COLUMN article_progress_fraction REAL;
ALTER TABLE rss_entries
  ADD COLUMN article_progress_anchor TEXT NOT NULL DEFAULT '';
ALTER TABLE rss_entries
  ADD COLUMN article_progress_content_revision INTEGER;
ALTER TABLE rss_entries
  ADD COLUMN video_progress_seconds REAL;
ALTER TABLE rss_entries
  ADD COLUMN video_duration_seconds REAL;
ALTER TABLE rss_entries
  ADD COLUMN video_completed BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE rss_entries
  ADD COLUMN read_revision INTEGER NOT NULL DEFAULT 0 CHECK (read_revision >= 0);
ALTER TABLE rss_entries
  ADD COLUMN starred_revision INTEGER NOT NULL DEFAULT 0 CHECK (starred_revision >= 0);
ALTER TABLE rss_entries
  ADD COLUMN article_progress_revision INTEGER NOT NULL DEFAULT 0 CHECK (article_progress_revision >= 0);
ALTER TABLE rss_entries
  ADD COLUMN video_progress_seconds_revision INTEGER NOT NULL DEFAULT 0 CHECK (video_progress_seconds_revision >= 0);

-- Existing read state predates per-field clocks. Preserve its aggregate clock
-- as the read clock so an already-read article cannot be overwritten by a
-- client that legitimately starts at revision zero.
UPDATE rss_entries
SET read_revision = state_revision
WHERE state_revision > 0 AND read_revision = 0;

ALTER TABLE rss_client_mutations
  ADD COLUMN request_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS rss_changes_scope_sequence_idx
  ON rss_changes(workspace_id, subject_id, sequence);
CREATE INDEX IF NOT EXISTS rss_subscriptions_workspace_id_idx
  ON rss_subscriptions(workspace_id, id);
CREATE INDEX IF NOT EXISTS rss_entries_snapshot_id_idx
  ON rss_entries(id);
`

func applyRSSSyncV2Schema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin RSS sync v2 migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, rssSyncV2SchemaSQL); err != nil {
		return fmt.Errorf("apply RSS sync v2 migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit RSS sync v2 migration: %w", err)
	}
	return nil
}
