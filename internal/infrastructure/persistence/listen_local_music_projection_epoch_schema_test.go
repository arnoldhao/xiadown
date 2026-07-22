package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestListenLocalMusicProjectionEpochMigrationRotatesOnceAndPreservesWindow(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "music-projection-epoch.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("open current database: %v", err)
	}

	const oldEpoch = "0123456789abcdef0123456789abcdef"
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE listen_local_music_sync_state
SET epoch = ?, high_water = 17, minimum_cursor = 5
WHERE id = 1
`, oldEpoch); err != nil {
		database.Close()
		t.Fatalf("prepare v29 Music position: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = 30`); err != nil {
		database.Close()
		t.Fatalf("remove v30 fixture ledger: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `PRAGMA user_version = 29`); err != nil {
		database.Close()
		t.Fatalf("set v29 fixture identity: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close v29 fixture: %v", err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade v29 fixture: %v", err)
	}
	if upgraded.MigrationSnapshotPath == "" {
		upgraded.Close()
		t.Fatal("v29 upgrade did not create a pre-migration snapshot")
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
}
