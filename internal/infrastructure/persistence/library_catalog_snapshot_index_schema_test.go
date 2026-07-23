package persistence

import (
	"context"
	"testing"
)

func TestLibraryCatalogSnapshotIndexMigrationIsIdempotent(t *testing.T) {
	ctx := context.Background()
	database := openLatestSQLiteTestDatabase(t)
	defer database.Close()

	if err := applyLibraryCatalogSnapshotIndexSchema(ctx, database.SQL); err != nil {
		t.Fatalf("reapply snapshot index once: %v", err)
	}
	if err := applyLibraryCatalogSnapshotIndexSchema(ctx, database.SQL); err != nil {
		t.Fatalf("reapply snapshot index twice: %v", err)
	}
	var indexes int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'index' AND name = 'library_catalog_items_snapshot_idx'
`).Scan(&indexes); err != nil {
		t.Fatalf("inspect snapshot index: %v", err)
	}
	if indexes != 1 {
		t.Fatalf("snapshot index count=%d, want 1", indexes)
	}
}
