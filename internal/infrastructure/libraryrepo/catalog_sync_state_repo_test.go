package libraryrepo

import (
	"context"
	"testing"
)

func TestSQLiteCatalogSyncStateRepositoryReturnsPersistentEpochAndCursor(t *testing.T) {
	ctx := context.Background()
	database, err := openLibraryRepoTestDatabase(t, ctx, "catalog-sync.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (
  'catalog-sync', 'Library', '', 'active', TRUE,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_catalog_changes (
  catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
) VALUES (
  'catalog-sync', 'catalog', 'catalog-sync', 'upsert', 1, '',
  '2026-07-13T00:00:00Z'
);
`); err != nil {
		t.Fatal(err)
	}
	repository := NewSQLiteCatalogSyncStateRepository(database.Bun)
	state, err := repository.GetCatalogSyncState(ctx, "catalog-sync")
	if err != nil {
		t.Fatal(err)
	}
	if state.CatalogID != "catalog-sync" || len(state.Epoch) != 32 || state.Cursor != 1 || state.RotatedAt.IsZero() {
		t.Fatalf("sync state = %+v", state)
	}
}
