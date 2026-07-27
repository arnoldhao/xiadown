package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLibraryRootSyncCandidateIndexMigrationIsInstalled(t *testing.T) {
	t.Parallel()
	found := false
	for _, migration := range sqliteMigrations {
		if migration.version != 35 {
			continue
		}
		found = migration.name == "library_storage_root_sync_candidate_index" &&
			migration.signature == libraryRootSyncCandidateIndexSQL
	}
	if !found {
		t.Fatal("Library storage root sync candidate index migration v35 is missing")
	}

	database, err := OpenSQLite(context.Background(), SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "candidate-index.sqlite"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var indexSQL string
	if err := database.SQL.QueryRow(`
SELECT sql
FROM sqlite_master
WHERE type = 'index'
  AND name = 'library_storage_root_sync_entries_size_idx'
`).Scan(&indexSQL); err != nil {
		t.Fatal(err)
	}
	if indexSQL == "" {
		t.Fatal("Library storage root sync candidate index was not created")
	}
}
