package librarybackup

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"xiadown/internal/infrastructure/persistence"
)

func TestLogicalRestoreMapsUpgradedTableColumnsByName(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "upgraded-source.db")
	source, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: sourcePath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('library', 'Library', '{}', '2026-07-19T00:00:00Z', '2026-07-19T00:00:00Z');
INSERT INTO library_files (
  id, library_id, kind, name, display_name, storage_mode, storage_local_path,
  origin_kind, origin_import_path, state_json, created_at, updated_at
) VALUES (
  'track-file', 'library', 'audio', 'track.flac', 'Track', 'local_path', '/tmp/track.flac',
  'import', '/tmp/track.flac', '{"status":"active"}',
  '2026-07-19T00:00:00Z', '2026-07-19T00:00:00Z'
);
INSERT INTO listen_local_tracks (
  file_id, library_id, local_path, title, author,
  album, album_artist, genre, track_number, disc_number, year,
  cover_local_path, format, audio_codec, duration_ms, size_bytes, mod_time_unix,
  availability, last_checked_at, probe_error, created_at, updated_at
) VALUES (
  'track-file', 'library', '/tmp/track.flac', 'Track title', 'Track artist',
  'Track album', 'Album artist', 'Test genre', 7, 2, 2026,
  '/tmp/cover.jpg', 'flac', 'flac', 123456, 987654, 1777777777,
  'available', '2026-07-19T00:00:00Z', 'distinct probe value',
  '2026-07-19T00:00:00Z', '2026-07-19T00:00:00Z'
);
UPDATE listen_local_tracks
SET content_identity_signature = 'mci1p:1111111111111111111111111111111111111111111111111111111111111111'
WHERE file_id = 'track-file';
`); err != nil {
		_ = source.Close()
		t.Fatal(err)
	}

	// This is the physical layout produced when album-related columns were
	// added with ALTER TABLE: the same column-name set as a fresh database, but
	// the new columns are appended after updated_at. SELECT * would shift most
	// values and either corrupt them silently or violate mod_time_unix NOT NULL.
	connection, err := source.SQL.Conn(ctx)
	if err != nil {
		_ = source.Close()
		t.Fatal(err)
	}
	if _, err := connection.ExecContext(ctx, `
PRAGMA foreign_keys = OFF;
CREATE TABLE listen_local_tracks_upgraded_order (
  file_id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  local_path TEXT NOT NULL,
  title TEXT NOT NULL,
  author TEXT,
  cover_local_path TEXT,
  format TEXT,
  audio_codec TEXT,
  duration_ms INTEGER,
  size_bytes INTEGER,
  mod_time_unix INTEGER NOT NULL DEFAULT 0,
  availability TEXT NOT NULL CHECK (availability IN ('available','missing')),
  last_checked_at TIMESTAMP NOT NULL,
  probe_error TEXT,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  album TEXT,
	  album_artist TEXT,
	  genre TEXT,
	  track_number INTEGER,
	  disc_number INTEGER,
	  year INTEGER,
	  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
	  content_identity_revision INTEGER NOT NULL DEFAULT 1 CHECK (content_identity_revision > 0),
	  content_identity_signature TEXT NOT NULL DEFAULT '' CHECK (length(content_identity_signature) <= 128),
	  metadata_revision INTEGER NOT NULL DEFAULT 1 CHECK (metadata_revision > 0),
	  resource_revision INTEGER NOT NULL DEFAULT 1 CHECK (resource_revision > 0),
	  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE CASCADE,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE
);
INSERT INTO listen_local_tracks_upgraded_order (
  file_id, library_id, local_path, title, author,
  cover_local_path, format, audio_codec, duration_ms, size_bytes, mod_time_unix,
  availability, last_checked_at, probe_error, created_at, updated_at,
	  album, album_artist, genre, track_number, disc_number, year,
	  revision, content_identity_revision, content_identity_signature,
	  metadata_revision, resource_revision
)
SELECT
  file_id, library_id, local_path, title, author,
  cover_local_path, format, audio_codec, duration_ms, size_bytes, mod_time_unix,
  availability, last_checked_at, probe_error, created_at, updated_at,
	  album, album_artist, genre, track_number, disc_number, year,
	  revision, content_identity_revision, content_identity_signature,
	  metadata_revision, resource_revision
FROM listen_local_tracks;
DROP TABLE listen_local_tracks;
ALTER TABLE listen_local_tracks_upgraded_order RENAME TO listen_local_tracks;
PRAGMA foreign_keys = ON;
`); err != nil {
		_ = connection.Close()
		_ = source.Close()
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		_ = source.Close()
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	currentPath := filepath.Join(directory, "fresh-current.db")
	current, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: currentPath})
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	if err := replaceAllowedLibraryMetadata(ctx, current.SQL, sourcePath); err != nil {
		t.Fatalf("restore same columns with different ordinal layout: %v", err)
	}

	var got struct {
		title, author, album, albumArtist, genre string
		trackNumber, discNumber, year            int64
		cover, format, codec                     string
		duration, size, modTime                  int64
		availability, probeError                 string
		contentIdentitySignature                 string
	}
	if err := current.SQL.QueryRowContext(ctx, `
SELECT title, author, album, album_artist, genre,
       track_number, disc_number, year,
       cover_local_path, format, audio_codec,
       duration_ms, size_bytes, mod_time_unix,
       availability, probe_error, content_identity_signature
FROM listen_local_tracks
WHERE file_id = 'track-file'
`).Scan(
		&got.title, &got.author, &got.album, &got.albumArtist, &got.genre,
		&got.trackNumber, &got.discNumber, &got.year,
		&got.cover, &got.format, &got.codec,
		&got.duration, &got.size, &got.modTime,
		&got.availability, &got.probeError, &got.contentIdentitySignature,
	); err != nil {
		t.Fatal(err)
	}
	if got.title != "Track title" || got.author != "Track artist" ||
		got.album != "Track album" || got.albumArtist != "Album artist" ||
		got.genre != "Test genre" || got.trackNumber != 7 || got.discNumber != 2 || got.year != 2026 ||
		got.cover != "/tmp/cover.jpg" || got.format != "flac" || got.codec != "flac" ||
		got.duration != 123456 || got.size != 987654 || got.modTime != 1777777777 ||
		got.availability != "available" || got.probeError != "distinct probe value" ||
		got.contentIdentitySignature != "mci1p:"+strings.Repeat("1", 64) {
		t.Fatalf("restored reordered local track = %+v", got)
	}
	var sourceEpoch, restoredEpoch string
	if err := sourceEpochFromDatabase(ctx, sourcePath, &sourceEpoch); err != nil {
		t.Fatal(err)
	}
	var musicHighWater, musicMinimum int64
	if err := current.SQL.QueryRowContext(ctx, `
SELECT epoch, high_water, minimum_cursor
FROM listen_local_music_sync_state
WHERE id = 1
`).Scan(&restoredEpoch, &musicHighWater, &musicMinimum); err != nil {
		t.Fatal(err)
	}
	if len(restoredEpoch) != 32 || restoredEpoch == sourceEpoch || musicHighWater < musicMinimum {
		t.Fatalf("restored Music sync state sourceEpoch=%q restored=(%q,%d,%d)",
			sourceEpoch, restoredEpoch, musicHighWater, musicMinimum)
	}
	assertNoForeignKeyViolations(t, ctx, current.SQL)
}

func sourceEpochFromDatabase(ctx context.Context, path string, destination *string) error {
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
	if err != nil {
		return err
	}
	defer database.Close()
	return database.SQL.QueryRowContext(ctx,
		`SELECT epoch FROM listen_local_music_sync_state WHERE id = 1`,
	).Scan(destination)
}

func TestCopyRestoreTableRejectsDifferentColumnSets(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "column-set.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	connection, err := database.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, `
CREATE TABLE restore_column_probe (id TEXT PRIMARY KEY, value TEXT NOT NULL);
ATTACH DATABASE ':memory:' AS restore_source;
CREATE TABLE restore_source.restore_column_probe (
  id TEXT PRIMARY KEY,
  unexpected TEXT NOT NULL
);
INSERT INTO restore_source.restore_column_probe (id, unexpected)
VALUES ('probe', 'must not be silently substituted');
`); err != nil {
		t.Fatal(err)
	}
	transaction, err := connection.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = copyRestoreTableByColumnName(ctx, transaction, "restore_column_probe")
	_ = transaction.Rollback()
	if err == nil || !strings.Contains(err.Error(), `missing current column "value"`) {
		t.Fatalf("different restore column set error = %v", err)
	}
	var count int
	if err := connection.QueryRowContext(ctx, "SELECT COUNT(*) FROM restore_column_probe").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("mismatched restore table copied %d rows", count)
	}
}

func assertNoForeignKeyViolations(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	rows, err := database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("restored database contains a foreign-key violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
