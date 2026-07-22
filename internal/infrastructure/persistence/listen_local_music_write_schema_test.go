package persistence

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestListenLocalMusicWriteProtocolMigratesAndTombstonesStateGraph(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "listen-local-v25-write.db")
	now := time.Date(2026, 7, 21, 17, 0, 0, 0, time.UTC)
	createListenLocalV23Fixture(t, ctx, path, now)

	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var tables int
	if err := database.SQL.QueryRowContext(ctx, `
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
`).Scan(&tables); err != nil || tables != 7 {
		t.Fatalf("write protocol tables=%d err=%v", tables, err)
	}

	documentID := uuid.NewString()
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO listen_local_music_track_states (
  subject_id, track_id, revision, favorite, favorite_revision,
  position_ms, play_session_id, content_identity_revision, progress_revision,
  cumulative_listened_ms, play_count, skip_count, updated_at
) VALUES ('music-owner', 'track-legacy', 3, TRUE, 1, 1000, '', 1, 1, 1000, 0, 0, ?)
`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO listen_local_music_lyric_documents (
  id, track_id, revision, source_kind, provider_id, provider_track_id,
  timing_kind, language, content_hash, availability, license_policy, created_at, updated_at
) VALUES (?, 'track-legacy', 2, 'provider', 'provider', 'remote-1',
          'synced', 'en', '', 'refetchRequired', 'refetchRequired', ?, ?)
`, documentID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO listen_local_music_lyric_selections (
  subject_id, track_id, document_id, offset_ms, revision, updated_at
) VALUES ('music-owner', 'track-legacy', ?, -50, 4, ?)
`, documentID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DELETE FROM listen_local_tracks WHERE file_id = 'track-legacy'`); err != nil {
		t.Fatal(err)
	}

	for table, entityType := range map[string]string{
		"listen_local_music_track_states":     "track_state",
		"listen_local_music_lyric_documents":  "lyric_document",
		"listen_local_music_lyric_selections": "lyric_selection",
	} {
		var remaining, tombstones, changes int
		if err := database.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM listen_local_music_tombstones WHERE entity_type = ?
`, entityType).Scan(&tombstones); err != nil {
			t.Fatal(err)
		}
		if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM listen_local_music_changes WHERE entity_type = ? AND operation = 'delete'
`, entityType).Scan(&changes); err != nil {
			t.Fatal(err)
		}
		if remaining != 0 || tombstones != 1 || changes != 1 {
			t.Fatalf("%s remaining=%d tombstones=%d deleteChanges=%d", entityType, remaining, tombstones, changes)
		}
	}
	var highWater, maxSequence int64
	if err := database.SQL.QueryRowContext(ctx, `SELECT high_water FROM listen_local_music_sync_state WHERE id = 1`).Scan(&highWater); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM listen_local_music_changes`).Scan(&maxSequence); err != nil {
		t.Fatal(err)
	}
	if highWater != maxSequence || highWater == 0 {
		t.Fatalf("highWater=%d maxSequence=%d", highWater, maxSequence)
	}
	rows, err := database.SQL.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign_key_check returned a violation")
	}
}
