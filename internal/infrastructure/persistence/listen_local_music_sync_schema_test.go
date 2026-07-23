package persistence

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

func TestListenLocalMusicSyncFoundationMigratesLegacyPlaylistIdentityAndPreservesReferences(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "listen-local-v23.db")
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	createListenLocalV23Fixture(t, ctx, path, now)

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	if upgraded.MigrationSnapshotPath == "" {
		t.Fatal("v23 upgrade did not create a pre-migration snapshot")
	}

	var version int
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != latestSQLiteMigrationVersion() {
		t.Fatalf("user_version=%d want=%d", version, latestSQLiteMigrationVersion())
	}

	var itemID, title, author, album, contentIdentitySignature string
	var trackRevision, contentIdentityRevision, metadataRevision, resourceRevision, playlistRevision, itemRevision int64
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT item.id, item.track_display_title, item.track_display_author, item.track_display_album,
       track.revision, track.content_identity_revision, track.content_identity_signature,
       track.metadata_revision, track.resource_revision,
       playlist.revision, item.revision
FROM listen_local_playlist_items AS item
JOIN listen_local_tracks AS track ON track.file_id = item.file_id
JOIN listen_local_playlists AS playlist ON playlist.id = item.playlist_id
WHERE item.playlist_id = 'playlist-legacy'
`).Scan(
		&itemID, &title, &author, &album, &trackRevision, &contentIdentityRevision, &contentIdentitySignature,
		&metadataRevision, &resourceRevision, &playlistRevision, &itemRevision,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(itemID); err != nil {
		t.Fatalf("backfilled item id %q is not a UUID: %v", itemID, err)
	}
	if title != "Legacy Track" || author != "Legacy Artist" || album != "Legacy Album" ||
		contentIdentitySignature != "" ||
		trackRevision != 1 || contentIdentityRevision != 1 || metadataRevision != 1 || resourceRevision != 1 ||
		playlistRevision != 1 || itemRevision != 1 {
		t.Fatalf("unexpected migrated values: id=%q title=%q author=%q album=%q revisions=(%d,%d,%d,%d,%d,%d)",
			itemID, title, author, album, trackRevision, contentIdentityRevision, metadataRevision,
			resourceRevision, playlistRevision, itemRevision)
	}
	var epoch string
	var highWater, minimumCursor int64
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT epoch, high_water, minimum_cursor FROM listen_local_music_sync_state WHERE id = 1
`).Scan(&epoch, &highWater, &minimumCursor); err != nil {
		t.Fatal(err)
	}
	decodedEpoch, decodeErr := hex.DecodeString(epoch)
	if decodeErr != nil || len(decodedEpoch) != 16 || highWater != 0 || minimumCursor != 0 {
		t.Fatalf("initial Music sync state epoch=%q highWater=%d minimum=%d decodeErr=%v",
			epoch, highWater, minimumCursor, decodeErr)
	}

	duplicateID := uuid.NewString()
	if _, err := upgraded.SQL.ExecContext(ctx, `
INSERT INTO listen_local_playlist_items (
  id, playlist_id, file_id, position, added_at, revision,
  track_display_title, track_display_author, track_display_album
) VALUES (?, 'playlist-legacy', 'track-legacy', 1, ?, 1, 'Legacy Track', 'Legacy Artist', 'Legacy Album')
`, duplicateID, now); err != nil {
		t.Fatalf("target schema rejected duplicate Track membership: %v", err)
	}
	if _, err := upgraded.SQL.ExecContext(ctx, `DELETE FROM listen_local_tracks WHERE file_id = 'track-legacy'`); err != nil {
		t.Fatalf("delete migrated Track: %v", err)
	}

	var retained, trackTombstones, trackDeleteChanges int
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM listen_local_playlist_items WHERE file_id = 'track-legacy'`).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM listen_local_music_tombstones
WHERE entity_type = 'track' AND entity_id = 'track-legacy' AND revision = 2
  AND content_identity_revision = 1 AND metadata_revision = 1 AND resource_revision = 1
`).Scan(&trackTombstones); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM listen_local_music_changes
WHERE entity_type = 'track' AND entity_id = 'track-legacy' AND operation = 'delete' AND revision = 2
`).Scan(&trackDeleteChanges); err != nil {
		t.Fatal(err)
	}
	if retained != 2 || trackTombstones != 1 || trackDeleteChanges != 1 {
		t.Fatalf("delete foundation retained=%d tombstones=%d changes=%d", retained, trackTombstones, trackDeleteChanges)
	}
	var advancedHighWater, maxSequence int64
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT high_water FROM listen_local_music_sync_state WHERE id = 1`).Scan(&advancedHighWater); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM listen_local_music_changes`).Scan(&maxSequence); err != nil {
		t.Fatal(err)
	}
	if advancedHighWater != maxSequence || advancedHighWater == 0 {
		t.Fatalf("Music high-water=%d maxSequence=%d", advancedHighWater, maxSequence)
	}
	foreignKeyRows, err := upgraded.SQL.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer foreignKeyRows.Close()
	if foreignKeyRows.Next() {
		t.Fatal("foreign_key_check returned a violation")
	}
}

func createListenLocalV23Fixture(t *testing.T, ctx context.Context, path string, now time.Time) {
	t.Helper()
	db, err := sqlite3driver.Open(path, configureSQLiteConnection)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, schemaMigrationsSQL); err != nil {
		t.Fatal(err)
	}
	for _, migration := range sqliteMigrations[:23] {
		if err := migration.apply(ctx, db); err != nil {
			t.Fatalf("apply fixture migration %d: %v", migration.version, err)
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO schema_migrations (version, name, checksum, applied_at, duration_ms)
VALUES (?, ?, ?, ?, 0)
`, migration.version, migration.name, migration.checksum(), now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", sqliteApplicationID)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA user_version = 23`); err != nil {
		t.Fatal(err)
	}
	seed := []struct {
		statement string
		args      []any
	}{
		{`INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('library-legacy', 'Legacy Music', '{}', ?, ?)`, []any{now, now}},
		{`INSERT INTO library_files (
  id, library_id, kind, name, display_name, storage_mode,
  storage_local_path, origin_kind, origin_import_path, state_json,
  created_at, updated_at
) VALUES (
  'track-legacy', 'library-legacy', 'audio', 'legacy.mp3', 'Legacy Track', 'local_path',
  '/tmp/legacy.mp3', 'import', '/tmp/legacy.mp3', '{"status":"active"}', ?, ?
)`, []any{now, now}},
		{`INSERT INTO listen_local_tracks (
  file_id, library_id, local_path, title, author, album, duration_ms,
  mod_time_unix, availability, last_checked_at, created_at, updated_at
) VALUES (
  'track-legacy', 'library-legacy', '/tmp/legacy.mp3', 'Legacy Track', 'Legacy Artist',
  'Legacy Album', 180000, 0, 'available', ?, ?, ?
)`, []any{now, now, now}},
		{`INSERT INTO listen_local_playlists (id, name, created_at, updated_at)
VALUES ('playlist-legacy', 'Legacy Playlist', ?, ?)`, []any{now, now}},
		{`INSERT INTO listen_local_playlist_items (playlist_id, file_id, position, added_at)
VALUES ('playlist-legacy', 'track-legacy', 0, ?)`, []any{now}},
	}
	for _, item := range seed {
		if _, err := db.ExecContext(ctx, item.statement, item.args...); err != nil {
			t.Fatalf("seed v23 Listen Local fixture: %v", err)
		}
	}
}
