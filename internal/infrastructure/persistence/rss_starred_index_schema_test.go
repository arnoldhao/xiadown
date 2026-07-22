package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRSSFoundationMigrationChecksumRemainsCompatible(t *testing.T) {
	const shippedChecksum = "34f08662dc5e2fabb26f6177ccd2d344d500074fc056afb0b05a828cf53d890a"
	migration, ok := sqliteMigrationByVersion(9)
	if !ok {
		t.Fatal("RSS foundation migration is missing")
	}
	if checksum := migration.checksum(); checksum != shippedChecksum {
		t.Fatalf("RSS foundation checksum=%q, want shipped checksum %q", checksum, shippedChecksum)
	}
}

func TestRSSStarredIndexMigrationUpgradesVersionThirteenWithoutChangingFoundation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rss-starred-v13.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
DROP INDEX rss_entries_starred_unread_idx;
DELETE FROM schema_migrations WHERE version = 14;
PRAGMA user_version = 13;
`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	if upgraded.MigrationSnapshotPath == "" {
		t.Fatal("v13 upgrade did not create a pre-migration snapshot")
	}

	var version, indexes int
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'index' AND name = 'rss_entries_starred_unread_idx'
`).Scan(&indexes); err != nil {
		t.Fatal(err)
	}
	if version != sqliteMigrations[len(sqliteMigrations)-1].version || indexes != 1 {
		t.Fatalf("version=%d indexes=%d", version, indexes)
	}
}
