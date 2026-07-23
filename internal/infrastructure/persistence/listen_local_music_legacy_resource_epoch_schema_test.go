package persistence

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestListenLocalMusicLegacyResourceEpochMigrationUpgradesV30OnceWithoutChangingData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "music-legacy-resource-epoch.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("open current database: %v", err)
	}

	const (
		oldEpoch   = "fedcba9876543210fedcba9876543210"
		playlistID = "playlist-preserved-across-v31"
	)
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO listen_local_playlists (id, name, created_at, updated_at, revision)
VALUES (?, 'Preserved Playlist', '2026-07-22T00:00:00Z', '2026-07-22T00:00:00Z', 7)
`, playlistID); err != nil {
		database.Close()
		t.Fatalf("seed v30 user data: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO listen_local_music_changes (
  sequence, entity_type, entity_id, operation, revision, occurred_at, payload_json
) VALUES (
  17, 'playlist', ?, 'upsert', 7, '2026-07-22T00:00:00Z', '{"fixture":"preserve"}'
)
`, playlistID); err != nil {
		database.Close()
		t.Fatalf("seed v30 Music journal: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE listen_local_music_sync_state
SET epoch = ?, high_water = 17, minimum_cursor = 5
WHERE id = 1
`, oldEpoch); err != nil {
		database.Close()
		t.Fatalf("prepare v30 Music position: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = 31`); err != nil {
		database.Close()
		t.Fatalf("remove v31 fixture ledger: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `PRAGMA user_version = 30`); err != nil {
		database.Close()
		t.Fatalf("set v30 fixture identity: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close v30 fixture: %v", err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade v30 fixture: %v", err)
	}
	if upgraded.MigrationSnapshotPath == "" {
		upgraded.Close()
		t.Fatal("v30 upgrade did not create a pre-migration snapshot")
	}

	snapshot, err := sql.Open("sqlite3", upgraded.MigrationSnapshotPath)
	if err != nil {
		upgraded.Close()
		t.Fatalf("open v30 migration snapshot: %v", err)
	}
	var snapshotVersion int
	var snapshotEpoch string
	if err := snapshot.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&snapshotVersion); err != nil {
		snapshot.Close()
		upgraded.Close()
		t.Fatalf("read snapshot schema identity: %v", err)
	}
	if err := snapshot.QueryRowContext(ctx, `
SELECT epoch FROM listen_local_music_sync_state WHERE id = 1
`).Scan(&snapshotEpoch); err != nil {
		snapshot.Close()
		upgraded.Close()
		t.Fatalf("read snapshot Music epoch: %v", err)
	}
	if err := snapshot.Close(); err != nil {
		upgraded.Close()
		t.Fatalf("close v30 migration snapshot: %v", err)
	}
	if snapshotVersion != 30 || snapshotEpoch != oldEpoch {
		upgraded.Close()
		t.Fatalf("snapshot identity version=%d epoch=%q", snapshotVersion, snapshotEpoch)
	}

	var epoch string
	var highWater, minimumCursor int64
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT epoch, high_water, minimum_cursor
FROM listen_local_music_sync_state
WHERE id = 1
`).Scan(&epoch, &highWater, &minimumCursor); err != nil {
		upgraded.Close()
		t.Fatalf("read upgraded Music sync position: %v", err)
	}
	if epoch == oldEpoch || len(epoch) != 32 || highWater != 17 || minimumCursor != 5 {
		upgraded.Close()
		t.Fatalf("upgraded position epoch=%q highWater=%d minimum=%d", epoch, highWater, minimumCursor)
	}
	var playlistName, payload string
	var playlistRevision, journalRevision, journalRows int64
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT name, revision FROM listen_local_playlists WHERE id = ?
`, playlistID).Scan(&playlistName, &playlistRevision); err != nil {
		upgraded.Close()
		t.Fatalf("read preserved playlist: %v", err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT revision, payload_json
FROM listen_local_music_changes
WHERE sequence = 17 AND entity_type = 'playlist' AND entity_id = ?
`, playlistID).Scan(&journalRevision, &payload); err != nil {
		upgraded.Close()
		t.Fatalf("read preserved Music journal row: %v", err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM listen_local_music_changes`).Scan(&journalRows); err != nil {
		upgraded.Close()
		t.Fatalf("count preserved Music journal: %v", err)
	}
	if playlistName != "Preserved Playlist" || playlistRevision != 7 || journalRevision != 7 || journalRows != 1 || payload != `{"fixture":"preserve"}` {
		upgraded.Close()
		t.Fatalf("preserved data playlist=%q/%d journal=%d/%d payload=%q", playlistName, playlistRevision, journalRevision, journalRows, payload)
	}
	if err := upgraded.Close(); err != nil {
		t.Fatalf("close upgraded database: %v", err)
	}

	reopened, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen upgraded database: %v", err)
	}
	defer reopened.Close()
	if reopened.MigrationSnapshotPath != "" {
		t.Fatalf("idempotent reopen created migration snapshot %q", reopened.MigrationSnapshotPath)
	}
	var persisted string
	if err := reopened.SQL.QueryRowContext(ctx, `
SELECT epoch FROM listen_local_music_sync_state WHERE id = 1
`).Scan(&persisted); err != nil {
		t.Fatalf("read persisted Music epoch: %v", err)
	}
	if persisted != epoch {
		t.Fatalf("Music epoch rotated on ordinary reopen: %q -> %q", epoch, persisted)
	}
	matches, err := filepath.Glob(path + ".pre-migration-*.bak")
	if err != nil {
		t.Fatalf("glob migration snapshots: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("migration snapshot count = %d, want 1", len(matches))
	}
}
