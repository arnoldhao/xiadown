package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const listenLocalMusicWriteSchemaSQL = `
DROP TRIGGER IF EXISTS listen_local_music_change_advance_high_water;
PRAGMA legacy_alter_table = ON;
ALTER TABLE listen_local_music_changes RENAME TO listen_local_music_changes_v25;

CREATE TABLE listen_local_music_changes (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL CHECK (entity_type IN (
    'track','playlist','playlist_item','membership',
    'track_state','lyric_document','lyric_selection'
  )),
  entity_id TEXT NOT NULL,
  operation TEXT NOT NULL CHECK (operation IN ('upsert','delete')),
  revision INTEGER NOT NULL CHECK (revision > 0),
  occurred_at TIMESTAMP NOT NULL,
  payload_json TEXT CHECK (payload_json IS NULL OR json_valid(payload_json))
);
INSERT INTO listen_local_music_changes (
  sequence, entity_type, entity_id, operation, revision, occurred_at, payload_json
)
SELECT sequence, entity_type, entity_id, operation, revision, occurred_at, payload_json
FROM listen_local_music_changes_v25
ORDER BY sequence;
DROP TABLE listen_local_music_changes_v25;
CREATE INDEX listen_local_music_changes_entity_idx
  ON listen_local_music_changes(entity_type, entity_id, sequence DESC);

ALTER TABLE listen_local_music_tombstones RENAME TO listen_local_music_tombstones_v25;
CREATE TABLE listen_local_music_tombstones (
  entity_type TEXT NOT NULL CHECK (entity_type IN (
    'track','playlist','playlist_item','membership',
    'track_state','lyric_document','lyric_selection'
  )),
  entity_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK (revision > 0),
  content_identity_revision INTEGER,
  metadata_revision INTEGER,
  resource_revision INTEGER,
  deleted_at TIMESTAMP NOT NULL,
  payload_json TEXT CHECK (payload_json IS NULL OR json_valid(payload_json)),
  PRIMARY KEY (entity_type, entity_id)
);
INSERT INTO listen_local_music_tombstones (
  entity_type, entity_id, revision, content_identity_revision,
  metadata_revision, resource_revision, deleted_at, payload_json
)
SELECT entity_type, entity_id, revision, content_identity_revision,
       metadata_revision, resource_revision, deleted_at, payload_json
FROM listen_local_music_tombstones_v25;
DROP TABLE listen_local_music_tombstones_v25;
PRAGMA legacy_alter_table = OFF;

CREATE TRIGGER listen_local_music_change_advance_high_water
AFTER INSERT ON listen_local_music_changes
BEGIN
  UPDATE listen_local_music_sync_state
  SET high_water = MAX(high_water, NEW.sequence),
      updated_at = CURRENT_TIMESTAMP
  WHERE id = 1;
END;

CREATE TABLE listen_local_music_mutation_receipts (
  receipt_sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  subject_id TEXT NOT NULL,
  mutation_id TEXT NOT NULL,
  family TEXT NOT NULL CHECK (family IN ('state','manage')),
  request_hash TEXT NOT NULL CHECK (
    length(request_hash) = 71 AND request_hash GLOB 'sha256:[0-9a-f]*'
  ),
  mutation_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  result_json TEXT NOT NULL CHECK (json_valid(result_json)),
  created_at TIMESTAMP NOT NULL,
  UNIQUE (subject_id, mutation_id)
);
CREATE INDEX listen_local_music_mutation_receipts_retention_idx
  ON listen_local_music_mutation_receipts(subject_id, receipt_sequence DESC);

CREATE TABLE listen_local_music_track_states (
  subject_id TEXT NOT NULL,
  track_id TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  favorite BOOLEAN NOT NULL DEFAULT FALSE,
  favorite_revision INTEGER NOT NULL DEFAULT 0 CHECK (favorite_revision >= 0),
  position_ms INTEGER NOT NULL DEFAULT 0 CHECK (position_ms >= 0),
  play_session_id TEXT NOT NULL DEFAULT '',
  content_identity_revision INTEGER NOT NULL DEFAULT 0 CHECK (content_identity_revision >= 0),
  progress_revision INTEGER NOT NULL DEFAULT 0 CHECK (progress_revision >= 0),
  cumulative_listened_ms INTEGER NOT NULL DEFAULT 0 CHECK (cumulative_listened_ms >= 0),
  play_count INTEGER NOT NULL DEFAULT 0 CHECK (play_count >= 0),
  skip_count INTEGER NOT NULL DEFAULT 0 CHECK (skip_count >= 0),
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (subject_id, track_id)
);

CREATE TABLE listen_local_music_lyric_documents (
  id TEXT PRIMARY KEY,
  track_id TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  source_kind TEXT NOT NULL CHECK (source_kind IN ('embedded','sidecar','provider')),
  provider_id TEXT NOT NULL DEFAULT '',
  provider_track_id TEXT NOT NULL DEFAULT '',
  timing_kind TEXT NOT NULL CHECK (timing_kind IN ('plain','synced')),
  language TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '' CHECK (
    content_hash = '' OR (length(content_hash) = 64 AND content_hash = lower(content_hash))
  ),
  availability TEXT NOT NULL CHECK (availability IN ('content','refetchRequired','unavailable')),
  license_policy TEXT NOT NULL CHECK (license_policy IN ('cacheAllowed','refetchRequired')),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  UNIQUE (track_id, source_kind, provider_id, provider_track_id, content_hash)
);
CREATE INDEX listen_local_music_lyric_documents_track_idx
  ON listen_local_music_lyric_documents(track_id, updated_at DESC, id);

CREATE TABLE listen_local_music_lyric_selections (
  subject_id TEXT NOT NULL,
  track_id TEXT NOT NULL,
  document_id TEXT NOT NULL,
  offset_ms INTEGER NOT NULL DEFAULT 0,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (subject_id, track_id),
  FOREIGN KEY (document_id) REFERENCES listen_local_music_lyric_documents(id) ON DELETE RESTRICT
);

CREATE TABLE listen_local_music_play_sessions (
  subject_id TEXT NOT NULL,
  play_session_id TEXT NOT NULL,
  track_id TEXT NOT NULL,
  content_identity_revision INTEGER NOT NULL CHECK (content_identity_revision > 0),
  max_sequence INTEGER NOT NULL DEFAULT 0 CHECK (max_sequence >= 0),
  cumulative_listened_ms INTEGER NOT NULL DEFAULT 0 CHECK (cumulative_listened_ms >= 0),
  position_ms INTEGER NOT NULL DEFAULT 0 CHECK (position_ms >= 0),
  terminal BOOLEAN NOT NULL DEFAULT FALSE,
  completed BOOLEAN NOT NULL DEFAULT FALSE,
  end_reason TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (subject_id, play_session_id)
);

CREATE TABLE listen_local_music_play_event_checkpoints (
  subject_id TEXT NOT NULL,
  play_session_id TEXT NOT NULL,
  sequence INTEGER NOT NULL CHECK (sequence > 0),
  event_id TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  cumulative_listened_ms INTEGER NOT NULL CHECK (cumulative_listened_ms >= 0),
  position_ms INTEGER NOT NULL CHECK (position_ms >= 0),
  terminal BOOLEAN NOT NULL,
  completed BOOLEAN NOT NULL,
  end_reason TEXT NOT NULL DEFAULT '',
  device_occurred_at TIMESTAMP,
  received_at TIMESTAMP NOT NULL,
  PRIMARY KEY (subject_id, play_session_id, sequence),
  UNIQUE (subject_id, event_id)
);

CREATE TABLE listen_local_music_play_event_receipts (
  receipt_sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  subject_id TEXT NOT NULL,
  event_id TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  result_json TEXT NOT NULL CHECK (json_valid(result_json)),
  created_at TIMESTAMP NOT NULL,
  UNIQUE (subject_id, event_id)
);
CREATE INDEX listen_local_music_play_event_receipts_retention_idx
  ON listen_local_music_play_event_receipts(subject_id, receipt_sequence DESC);

CREATE TRIGGER listen_local_track_before_delete_phase4
BEFORE DELETE ON listen_local_tracks
BEGIN
  INSERT INTO listen_local_music_tombstones (
    entity_type, entity_id, revision, deleted_at
  )
  SELECT 'lyric_selection', selection.track_id, selection.revision + 1, CURRENT_TIMESTAMP
  FROM listen_local_music_lyric_selections AS selection
  WHERE selection.track_id = OLD.file_id
  ON CONFLICT(entity_type, entity_id) DO UPDATE SET
    revision = MAX(revision, excluded.revision), deleted_at = excluded.deleted_at;

  INSERT INTO listen_local_music_changes (
    entity_type, entity_id, operation, revision, occurred_at
  )
  SELECT 'lyric_selection', selection.track_id, 'delete', selection.revision + 1, CURRENT_TIMESTAMP
  FROM listen_local_music_lyric_selections AS selection
  WHERE selection.track_id = OLD.file_id;

  DELETE FROM listen_local_music_lyric_selections WHERE track_id = OLD.file_id;

  INSERT INTO listen_local_music_tombstones (
    entity_type, entity_id, revision, deleted_at
  )
  SELECT 'lyric_document', document.id, document.revision + 1, CURRENT_TIMESTAMP
  FROM listen_local_music_lyric_documents AS document
  WHERE document.track_id = OLD.file_id
  ON CONFLICT(entity_type, entity_id) DO UPDATE SET
    revision = MAX(revision, excluded.revision), deleted_at = excluded.deleted_at;

  INSERT INTO listen_local_music_changes (
    entity_type, entity_id, operation, revision, occurred_at
  )
  SELECT 'lyric_document', document.id, 'delete', document.revision + 1, CURRENT_TIMESTAMP
  FROM listen_local_music_lyric_documents AS document
  WHERE document.track_id = OLD.file_id;

  DELETE FROM listen_local_music_lyric_documents WHERE track_id = OLD.file_id;

  INSERT INTO listen_local_music_tombstones (
    entity_type, entity_id, revision, deleted_at
  )
  SELECT 'track_state', state.track_id, state.revision + 1, CURRENT_TIMESTAMP
  FROM listen_local_music_track_states AS state
  WHERE state.track_id = OLD.file_id
  ON CONFLICT(entity_type, entity_id) DO UPDATE SET
    revision = MAX(revision, excluded.revision), deleted_at = excluded.deleted_at;

  INSERT INTO listen_local_music_changes (
    entity_type, entity_id, operation, revision, occurred_at
  )
  SELECT 'track_state', state.track_id, 'delete', state.revision + 1, CURRENT_TIMESTAMP
  FROM listen_local_music_track_states AS state
  WHERE state.track_id = OLD.file_id;

  DELETE FROM listen_local_music_track_states WHERE track_id = OLD.file_id;
END;
`

func applyListenLocalMusicWriteSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Listen Local Music write migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyListenLocalMusicWriteSchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Listen Local Music write migration: %w", err)
	}
	return nil
}

func applyListenLocalMusicWriteSchemaTx(ctx context.Context, tx *sql.Tx) error {
	var current int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'table' AND name IN (
  'listen_local_music_mutation_receipts',
  'listen_local_music_track_states',
  'listen_local_music_lyric_documents',
  'listen_local_music_lyric_selections',
  'listen_local_music_play_sessions',
  'listen_local_music_play_event_checkpoints',
  'listen_local_music_play_event_receipts'
)
`).Scan(&current); err != nil {
		return fmt.Errorf("inspect Listen Local Music write schema: %w", err)
	}
	if current == 7 {
		return nil
	}
	if current != 0 {
		return fmt.Errorf("Listen Local Music write schema is partially applied: tables=%d", current)
	}
	if _, err := tx.ExecContext(ctx, listenLocalMusicWriteSchemaSQL); err != nil {
		return fmt.Errorf("create Listen Local Music write schema: %w", err)
	}
	return nil
}
