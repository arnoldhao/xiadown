package persistence

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLibraryRootSyncAvailabilityIndexesAreInstalled(t *testing.T) {
	t.Parallel()
	found := false
	for _, migration := range sqliteMigrations {
		if migration.version != 36 {
			continue
		}
		found = migration.name == "library_storage_root_sync_availability_indexes" &&
			migration.signature == libraryRootSyncAvailabilityIndexSQL
	}
	if !found {
		t.Fatal("Library storage root sync availability index migration v36 is missing")
	}

	database, err := OpenSQLite(context.Background(), SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "availability-index.sqlite"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, name := range []string{
		"library_storage_root_sync_entries_file_idx",
		"library_storage_root_sync_entries_status_idx",
		"library_catalog_items_runtime_browse_idx",
		"library_catalog_items_category_runtime_browse_idx",
	} {
		var indexSQL string
		if err := database.SQL.QueryRow(`
SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?
`, name).Scan(&indexSQL); err != nil {
			t.Fatal(err)
		}
		if indexSQL == "" {
			t.Fatalf("%s was not created", name)
		}
	}
}
