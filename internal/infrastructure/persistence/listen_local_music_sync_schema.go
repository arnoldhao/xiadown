package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const listenLocalMusicSyncFoundationPreSQL = `
ALTER TABLE listen_local_tracks
  ADD COLUMN revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE listen_local_tracks
  ADD COLUMN content_identity_revision INTEGER NOT NULL DEFAULT 1 CHECK (content_identity_revision > 0);

ALTER TABLE listen_local_playlists
  ADD COLUMN revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0);

DROP INDEX IF EXISTS listen_local_playlist_items_order_idx;
ALTER TABLE listen_local_playlist_items RENAME TO listen_local_playlist_items_legacy;

CREATE TABLE listen_local_playlist_items (
  id TEXT PRIMARY KEY,
  playlist_id TEXT NOT NULL,
  file_id TEXT NOT NULL,
  position INTEGER NOT NULL CHECK (position >= 0),
  added_at TIMESTAMP NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  deleted_at TIMESTAMP,
  track_display_title TEXT NOT NULL,
  track_display_author TEXT NOT NULL DEFAULT '',
  track_display_album TEXT NOT NULL DEFAULT '',
  track_display_duration_ms INTEGER,
  UNIQUE (playlist_id, position),
  FOREIGN KEY (playlist_id) REFERENCES listen_local_playlists(id) ON DELETE RESTRICT
);
`

const listenLocalMusicSyncFoundationPostSQL = `
DROP TABLE listen_local_playlist_items_legacy;

CREATE INDEX listen_local_playlist_items_order_idx
  ON listen_local_playlist_items(playlist_id, position, id);
CREATE INDEX listen_local_playlist_items_file_idx
  ON listen_local_playlist_items(file_id, playlist_id, position);

CREATE TABLE listen_local_music_memberships (
  file_id TEXT PRIMARY KEY,
  state TEXT NOT NULL CHECK (state IN ('included','excluded')),
  reason TEXT NOT NULL DEFAULT '' CHECK (reason IN ('','user','unsupported','policy')),
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE INDEX listen_local_music_memberships_state_idx
  ON listen_local_music_memberships(state, reason, updated_at DESC);

CREATE TABLE listen_local_music_tombstones (
  entity_type TEXT NOT NULL CHECK (entity_type IN ('track','playlist','playlist_item','membership')),
  entity_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK (revision > 0),
  content_identity_revision INTEGER,
  deleted_at TIMESTAMP NOT NULL,
  payload_json TEXT CHECK (payload_json IS NULL OR json_valid(payload_json)),
  PRIMARY KEY (entity_type, entity_id)
);

CREATE TABLE listen_local_music_changes (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL CHECK (entity_type IN ('track','playlist','playlist_item','membership')),
  entity_id TEXT NOT NULL,
  operation TEXT NOT NULL CHECK (operation IN ('upsert','delete')),
  revision INTEGER NOT NULL CHECK (revision > 0),
  occurred_at TIMESTAMP NOT NULL,
  payload_json TEXT CHECK (payload_json IS NULL OR json_valid(payload_json))
);

CREATE INDEX listen_local_music_changes_entity_idx
  ON listen_local_music_changes(entity_type, entity_id, sequence DESC);

-- Playlist items intentionally survive Track deletion. Refresh their final
-- display fallback first, then record the Track tombstone. This trigger also
-- covers a LibraryFile foreign-key cascade that bypasses the Track repository.
CREATE TRIGGER listen_local_track_before_delete_sync
BEFORE DELETE ON listen_local_tracks
BEGIN
  INSERT INTO listen_local_music_changes (
    entity_type, entity_id, operation, revision, occurred_at
  )
  SELECT 'playlist_item', item.id, 'upsert', item.revision + 1, CURRENT_TIMESTAMP
  FROM listen_local_playlist_items AS item
  WHERE item.file_id = OLD.file_id
    AND (
      item.track_display_title IS NOT OLD.title OR
      item.track_display_author IS NOT COALESCE(OLD.author, '') OR
      item.track_display_album IS NOT COALESCE(OLD.album, '') OR
      item.track_display_duration_ms IS NOT OLD.duration_ms
    );

  UPDATE listen_local_playlist_items
  SET track_display_title = OLD.title,
      track_display_author = COALESCE(OLD.author, ''),
      track_display_album = COALESCE(OLD.album, ''),
      track_display_duration_ms = OLD.duration_ms,
      revision = revision + 1
  WHERE file_id = OLD.file_id
    AND (
      track_display_title IS NOT OLD.title OR
      track_display_author IS NOT COALESCE(OLD.author, '') OR
      track_display_album IS NOT COALESCE(OLD.album, '') OR
      track_display_duration_ms IS NOT OLD.duration_ms
    );

  INSERT INTO listen_local_music_changes (
    entity_type, entity_id, operation, revision, occurred_at
  )
  SELECT 'playlist', playlist.id, 'upsert', playlist.revision + 1, CURRENT_TIMESTAMP
  FROM listen_local_playlists AS playlist
  WHERE EXISTS (
    SELECT 1 FROM listen_local_playlist_items AS item
    WHERE item.playlist_id = playlist.id AND item.file_id = OLD.file_id
  );

  UPDATE listen_local_playlists
  SET revision = revision + 1,
      updated_at = CURRENT_TIMESTAMP
  WHERE EXISTS (
    SELECT 1 FROM listen_local_playlist_items AS item
    WHERE item.playlist_id = listen_local_playlists.id AND item.file_id = OLD.file_id
  );

  INSERT INTO listen_local_music_tombstones (
    entity_type, entity_id, revision, content_identity_revision, deleted_at
  ) VALUES (
    'track', OLD.file_id, OLD.revision + 1, OLD.content_identity_revision, CURRENT_TIMESTAMP
  )
  ON CONFLICT(entity_type, entity_id) DO UPDATE SET
    revision = MAX(revision, excluded.revision),
    content_identity_revision = excluded.content_identity_revision,
    deleted_at = excluded.deleted_at;

  INSERT INTO listen_local_music_changes (
    entity_type, entity_id, operation, revision, occurred_at
  ) VALUES ('track', OLD.file_id, 'delete', OLD.revision + 1, CURRENT_TIMESTAMP);
END;

CREATE TRIGGER listen_local_playlist_item_before_delete_sync
BEFORE DELETE ON listen_local_playlist_items
BEGIN
  INSERT INTO listen_local_music_tombstones (
    entity_type, entity_id, revision, deleted_at
  ) VALUES ('playlist_item', OLD.id, OLD.revision + 1, CURRENT_TIMESTAMP)
  ON CONFLICT(entity_type, entity_id) DO UPDATE SET
    revision = MAX(revision, excluded.revision),
    deleted_at = excluded.deleted_at;

  INSERT INTO listen_local_music_changes (
    entity_type, entity_id, operation, revision, occurred_at
  ) VALUES ('playlist_item', OLD.id, 'delete', OLD.revision + 1, CURRENT_TIMESTAMP);
END;

-- The RESTRICT foreign key prevents a silent cascade. The trigger performs
-- the explicit child deletion so every Item receives its own tombstone.
CREATE TRIGGER listen_local_playlist_before_delete_sync
BEFORE DELETE ON listen_local_playlists
BEGIN
  DELETE FROM listen_local_playlist_items WHERE playlist_id = OLD.id;

  INSERT INTO listen_local_music_tombstones (
    entity_type, entity_id, revision, deleted_at
  ) VALUES ('playlist', OLD.id, OLD.revision + 1, CURRENT_TIMESTAMP)
  ON CONFLICT(entity_type, entity_id) DO UPDATE SET
    revision = MAX(revision, excluded.revision),
    deleted_at = excluded.deleted_at;

  INSERT INTO listen_local_music_changes (
    entity_type, entity_id, operation, revision, occurred_at
  ) VALUES ('playlist', OLD.id, 'delete', OLD.revision + 1, CURRENT_TIMESTAMP);
END;
`

type legacyListenLocalPlaylistItem struct {
	playlistID string
	fileID     string
	position   int
	addedAt    time.Time
	title      string
	author     string
	album      string
	durationMs sql.NullInt64
}

func applyListenLocalMusicSyncFoundation(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Listen Local Music sync foundation migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyListenLocalMusicSyncFoundationTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Listen Local Music sync foundation migration: %w", err)
	}
	return nil
}

func applyListenLocalMusicSyncFoundationTx(ctx context.Context, tx *sql.Tx) error {
	current, err := listenLocalMusicSyncSchemaCurrent(ctx, tx)
	if err != nil {
		return err
	}
	if current {
		return nil
	}
	if _, err := tx.ExecContext(ctx, listenLocalMusicSyncFoundationPreSQL); err != nil {
		return fmt.Errorf("prepare Listen Local Music sync schema: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
SELECT legacy.playlist_id, legacy.file_id, legacy.position, legacy.added_at,
       track.title, COALESCE(track.author, ''), COALESCE(track.album, ''), track.duration_ms
FROM listen_local_playlist_items_legacy AS legacy
JOIN listen_local_tracks AS track ON track.file_id = legacy.file_id
ORDER BY legacy.playlist_id, legacy.position, legacy.file_id
`)
	if err != nil {
		return fmt.Errorf("read legacy Listen Local playlist items: %w", err)
	}
	legacyItems := make([]legacyListenLocalPlaylistItem, 0)
	for rows.Next() {
		var item legacyListenLocalPlaylistItem
		if err := rows.Scan(
			&item.playlistID, &item.fileID, &item.position, &item.addedAt,
			&item.title, &item.author, &item.album, &item.durationMs,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan legacy Listen Local playlist item: %w", err)
		}
		legacyItems = append(legacyItems, item)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy Listen Local playlist items: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy Listen Local playlist items: %w", err)
	}

	for _, item := range legacyItems {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO listen_local_playlist_items (
  id, playlist_id, file_id, position, added_at, revision,
  track_display_title, track_display_author, track_display_album, track_display_duration_ms
) VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
`, uuid.NewString(), item.playlistID, item.fileID, item.position, item.addedAt,
			item.title, item.author, item.album, item.durationMs); err != nil {
			return fmt.Errorf("backfill Listen Local playlist item identity: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, listenLocalMusicSyncFoundationPostSQL); err != nil {
		return fmt.Errorf("finish Listen Local Music sync schema: %w", err)
	}
	return nil
}

func listenLocalMusicSyncSchemaCurrent(ctx context.Context, tx *sql.Tx) (bool, error) {
	var trackColumns, playlistColumns, itemColumns, tables, triggers int
	queries := []struct {
		destination *int
		query       string
	}{
		{&trackColumns, `SELECT COUNT(*) FROM pragma_table_info('listen_local_tracks') WHERE name IN ('revision','content_identity_revision')`},
		{&playlistColumns, `SELECT COUNT(*) FROM pragma_table_info('listen_local_playlists') WHERE name = 'revision'`},
		{&itemColumns, `SELECT COUNT(*) FROM pragma_table_info('listen_local_playlist_items') WHERE name IN ('id','revision','track_display_title')`},
		{&tables, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('listen_local_music_memberships','listen_local_music_tombstones','listen_local_music_changes')`},
		{&triggers, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('listen_local_track_before_delete_sync','listen_local_playlist_item_before_delete_sync','listen_local_playlist_before_delete_sync')`},
	}
	for _, item := range queries {
		if err := tx.QueryRowContext(ctx, item.query).Scan(item.destination); err != nil {
			return false, fmt.Errorf("inspect Listen Local Music sync schema: %w", err)
		}
	}
	if trackColumns == 0 && playlistColumns == 0 && itemColumns == 0 && tables == 0 && triggers == 0 {
		return false, nil
	}
	if trackColumns == 2 && playlistColumns == 1 && itemColumns == 3 && tables == 3 && triggers == 3 {
		return true, nil
	}
	return false, fmt.Errorf(
		"Listen Local Music sync schema is partially applied: track_columns=%d playlist_columns=%d item_columns=%d tables=%d triggers=%d",
		trackColumns, playlistColumns, itemColumns, tables, triggers,
	)
}
