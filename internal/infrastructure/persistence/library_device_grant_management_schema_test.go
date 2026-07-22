package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLibraryDeviceGrantManagementMigrationAddsRevision(t *testing.T) {
	database, err := OpenSQLite(context.Background(), SQLiteConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	var revision int64
	if err := database.SQL.QueryRowContext(context.Background(), `
SELECT revision FROM library_device_grants LIMIT 1
`).Scan(&revision); err == nil {
		t.Fatal("empty grant table unexpectedly returned a row")
	}

	rows, err := database.SQL.QueryContext(context.Background(), "PRAGMA table_info(library_device_grants)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == "revision" {
			found = notNull == 1
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("revision column is missing or nullable")
	}
}

func TestExistingV3DatabaseUpgradesDeviceGrantManagementAdditively(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v3.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("create current database: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
DROP INDEX library_device_grants_status_idx;
ALTER TABLE library_device_grants DROP COLUMN revision;
DELETE FROM schema_migrations WHERE version = 4;
PRAGMA user_version = 3;
`); err != nil {
		database.Close()
		t.Fatalf("prepare v3 fixture: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade v3 database: %v", err)
	}
	defer upgraded.Close()
	if upgraded.MigrationSnapshotPath == "" {
		t.Fatal("v3 upgrade did not produce a pre-migration snapshot")
	}
	var version int
	if err := upgraded.SQL.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	wantVersion := sqliteMigrations[len(sqliteMigrations)-1].version
	if version != wantVersion {
		t.Fatalf("user_version=%d, want %d", version, wantVersion)
	}
}
