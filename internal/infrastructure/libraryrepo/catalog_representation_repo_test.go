package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteRepresentationAndMetadataRepositoriesRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, now := openCatalogRepresentationTestDB(t, ctx)
	defer db.Close()

	representations := NewSQLiteRepresentationRepository(db.Bun)
	items, err := representations.ListRepresentationsByItemID(ctx, "item-1")
	if err != nil || len(items) != 1 {
		t.Fatalf("list trigger-created representation: %#v, err=%v", items, err)
	}
	width, height := 1920, 1080
	duration, bitrate, size := int64(90_000), int64(8_000_000), int64(42_000_000)
	updatedAt := now.Add(time.Minute)
	representation, err := library.NewRepresentation(library.RepresentationParams{
		ID: items[0].ID, CatalogID: items[0].CatalogID, ItemID: items[0].ItemID, AssetID: items[0].AssetID,
		Kind: "optimized", Purpose: "playback", MediaType: "video/mp4", Container: "mp4", Codec: "h264",
		Width: &width, Height: &height, DurationMs: &duration, BitrateBps: &bitrate, Language: "en",
		SizeBytes: &size, Availability: "available", Revision: 2,
		CreatedAt: &items[0].CreatedAt, UpdatedAt: &updatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := representations.SaveRepresentation(ctx, representation); err != nil {
		t.Fatalf("save representation: %v", err)
	}
	loadedRepresentation, err := representations.GetRepresentation(ctx, representation.ID)
	if err != nil || loadedRepresentation.Codec != "h264" || loadedRepresentation.Width == nil || *loadedRepresentation.Width != 1920 {
		t.Fatalf("unexpected representation: %#v, err=%v", loadedRepresentation, err)
	}

	confidence := 0.95
	metadata := NewSQLiteMetadataEntryRepository(db.Bun)
	entry, err := library.NewMetadataEntry(library.MetadataEntryParams{
		ID: "metadata-title", CatalogID: "catalog-1", ItemID: "item-1", RepresentationID: representation.ID,
		Namespace: "dc", Key: "title", ValueType: "string", ValueJSON: `"Movie"`, Language: "en",
		Source: "embedded", Provenance: "ffprobe:format.tags.title", Confidence: &confidence,
		Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.SaveMetadataEntry(ctx, entry); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	byItem, err := metadata.ListMetadataEntriesByItemID(ctx, entry.ItemID)
	if err != nil || len(byItem) != 1 || byItem[0].ID != entry.ID {
		t.Fatalf("list metadata by item: %#v, err=%v", byItem, err)
	}
	byRepresentation, err := metadata.ListMetadataEntriesByRepresentationID(ctx, representation.ID)
	if err != nil || len(byRepresentation) != 1 || string(byRepresentation[0].Value) != `"Movie"` {
		t.Fatalf("list metadata by representation: %#v, err=%v", byRepresentation, err)
	}
	loadedEntry, err := metadata.GetMetadataEntry(ctx, entry.ID)
	if err != nil || loadedEntry.Provenance != entry.Provenance || loadedEntry.Confidence == nil || *loadedEntry.Confidence != confidence {
		t.Fatalf("unexpected metadata: %#v, err=%v", loadedEntry, err)
	}

	if _, err := db.SQL.ExecContext(ctx, `
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, description, revision, created_at, updated_at
) VALUES ('item-2', 'catalog-1', 'video', 'active', 'Other', 'Other', '', 1, ?, ?)
`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
INSERT INTO library_representations (
  id, catalog_id, item_id, asset_id, kind, purpose, availability, revision, created_at, updated_at
) VALUES ('cross-item', 'catalog-1', 'item-2', 'asset-1', 'original', 'primary', 'available', 1, ?, ?)
`, now, now); err == nil {
		t.Fatal("cross-item representation unexpectedly succeeded")
	}
}

func TestProfessionalMutationsAreRevisionGuardedAndChangeFedAtomically(t *testing.T) {
	ctx := context.Background()
	db, now := openCatalogRepresentationTestDB(t, ctx)
	defer db.Close()

	repo := NewSQLiteCatalogMutationRepository(db.Bun)
	current, err := repo.GetRepresentation(ctx, "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	updatedAt := now.Add(time.Minute)
	updated, err := library.NewRepresentation(library.RepresentationParams{
		ID: current.ID, CatalogID: current.CatalogID, ItemID: current.ItemID, AssetID: current.AssetID,
		Kind: string(current.Kind), Purpose: string(current.Purpose), Container: "mp4", Codec: "h264",
		Availability: "available", Revision: 2, CreatedAt: &current.CreatedAt, UpdatedAt: &updatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveRepresentationMutation(ctx, updated, 1, "desktop-user"); err != nil {
		t.Fatalf("mutate representation: %v", err)
	}
	if err := repo.SaveRepresentationMutation(ctx, updated, 1, "desktop-user"); !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("stale representation mutation error=%v", err)
	}

	entry, err := library.NewMetadataEntry(library.MetadataEntryParams{
		ID: "metadata-title", CatalogID: "catalog-1", ItemID: "item-1",
		Namespace: "dc", Key: "title", ValueType: "string", ValueJSON: `"Movie"`,
		Source: "user", Provenance: "desktop-user", Locked: true, Revision: 1,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMetadataEntryMutation(ctx, entry, 0, "desktop-user"); err != nil {
		t.Fatalf("create metadata mutation: %v", err)
	}
	entryUpdatedAt := now.Add(2 * time.Minute)
	entry, err = library.NewMetadataEntry(library.MetadataEntryParams{
		ID: entry.ID, CatalogID: entry.CatalogID, ItemID: entry.ItemID,
		Namespace: entry.Namespace, Key: entry.Key, ValueType: "string", ValueJSON: `"Edited Movie"`,
		Source: "user", Provenance: "desktop-user", Locked: true, Revision: 2,
		CreatedAt: &entry.CreatedAt, UpdatedAt: &entryUpdatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMetadataEntryMutation(ctx, entry, 1, "desktop-user"); err != nil {
		t.Fatalf("update metadata mutation: %v", err)
	}
	if err := repo.SaveMetadataEntryMutation(ctx, entry, 1, "desktop-user"); !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("stale metadata mutation error=%v", err)
	}

	deleteEntry, err := library.NewMetadataEntry(library.MetadataEntryParams{
		ID: "metadata-delete", CatalogID: "catalog-1", ItemID: "item-1",
		Namespace: "music", Key: "album", ValueType: "string", ValueJSON: `"Album"`,
		Source: "user", Provenance: "desktop-user", Revision: 1,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMetadataEntryMutation(ctx, deleteEntry, 0, "desktop-user"); err != nil {
		t.Fatalf("create metadata for delete: %v", err)
	}
	staleDelete := deleteEntry
	deleteEntryUpdatedAt := now.Add(3 * time.Minute)
	deleteEntry, err = library.NewMetadataEntry(library.MetadataEntryParams{
		ID: deleteEntry.ID, CatalogID: deleteEntry.CatalogID, ItemID: deleteEntry.ItemID,
		Namespace: deleteEntry.Namespace, Key: deleteEntry.Key, ValueType: "string", ValueJSON: `"Edited Album"`,
		Source: "user", Provenance: "desktop-user", Revision: 2,
		CreatedAt: &deleteEntry.CreatedAt, UpdatedAt: &deleteEntryUpdatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMetadataEntryMutation(ctx, deleteEntry, 1, "desktop-user"); err != nil {
		t.Fatalf("update metadata for delete: %v", err)
	}
	staleDelete.UpdatedAt = now.Add(4 * time.Minute)
	if err := repo.DeleteMetadataEntryMutation(ctx, staleDelete, "desktop-user"); !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("stale metadata delete error=%v", err)
	}
	entry.UpdatedAt = now.Add(4 * time.Minute)
	if err := repo.DeleteMetadataEntryMutation(ctx, entry, "desktop-user"); err != nil {
		t.Fatalf("delete metadata mutation: %v", err)
	}
	if _, err := repo.GetMetadataEntry(ctx, entry.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted metadata is still readable: %v", err)
	}

	changes, err := NewSQLiteCatalogChangeRepository(db.Bun).ListAfter(ctx, "catalog-1", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 12 || changes[0].EntityType != library.CatalogEntityRepresentation ||
		changes[1].EntityType != library.CatalogEntityItem || changes[1].EntityID != "item-1" ||
		changes[10].EntityType != library.CatalogEntityMetadataEntry || changes[10].Kind != library.CatalogChangeDelete ||
		changes[11].EntityType != library.CatalogEntityItem || changes[11].EntityID != "item-1" {
		t.Fatalf("unexpected professional change feed: %#v", changes)
	}

	if _, err := db.SQL.ExecContext(ctx, `
CREATE TRIGGER reject_professional_change
BEFORE INSERT ON library_catalog_changes
WHEN NEW.entity_type = 'item'
BEGIN SELECT RAISE(ABORT, 'change feed unavailable'); END
`); err != nil {
		t.Fatal(err)
	}
	failedAt := now.Add(5 * time.Minute)
	failed, err := library.NewRepresentation(library.RepresentationParams{
		ID: updated.ID, CatalogID: updated.CatalogID, ItemID: updated.ItemID, AssetID: updated.AssetID,
		Kind: string(updated.Kind), Purpose: string(updated.Purpose), Container: "mkv", Codec: "av1",
		Availability: "available", Revision: 3, CreatedAt: &updated.CreatedAt, UpdatedAt: &failedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveRepresentationMutation(ctx, failed, 2, "desktop-user"); err == nil {
		t.Fatal("mutation unexpectedly succeeded without change feed")
	}
	persisted, err := repo.GetRepresentation(ctx, updated.ID)
	if err != nil || persisted.Revision != 2 || persisted.Container != "mp4" {
		t.Fatalf("representation update did not roll back: %#v, err=%v", persisted, err)
	}
	deleteEntry.UpdatedAt = now.Add(6 * time.Minute)
	if err := repo.DeleteMetadataEntryMutation(ctx, deleteEntry, "desktop-user"); err == nil {
		t.Fatal("metadata delete unexpectedly succeeded without owning-item change")
	}
	persistedEntry, err := repo.GetMetadataEntry(ctx, deleteEntry.ID)
	if err != nil || persistedEntry.Revision != 2 || string(persistedEntry.Value) != `"Edited Album"` {
		t.Fatalf("metadata delete did not roll back atomically: %#v err=%v", persistedEntry, err)
	}
}

func openCatalogRepresentationTestDB(t *testing.T, ctx context.Context) (*persistence.Database, time.Time) {
	t.Helper()
	db, err := openLibraryRepoTestDatabase(t, ctx, "catalog-professional.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	catalog, _ := library.NewCatalog(library.CatalogParams{
		ID: "catalog-1", Name: "Library", Status: "active", IsDefault: true, CreatedAt: &now, UpdatedAt: &now,
	})
	if err := NewSQLiteCatalogRepository(db.Bun).Save(ctx, catalog); err != nil {
		db.Close()
		t.Fatal(err)
	}
	legacy, _ := library.NewLibrary(library.LibraryParams{ID: "legacy-1", Name: "Legacy", CreatedAt: &now, UpdatedAt: &now})
	if err := NewSQLiteLibraryRepository(db.Bun).Save(ctx, legacy); err != nil {
		db.Close()
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "movie.mp4")
	file, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-1", LibraryID: legacy.ID, Kind: "video", Name: "movie.mp4",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: path},
		Origin:    library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{BatchID: "batch-1", ImportPath: path, ImportedAt: now}},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := NewSQLiteFileRepository(db.Bun).Save(ctx, file); err != nil {
		db.Close()
		t.Fatal(err)
	}
	item, _ := library.NewItem(library.ItemParams{
		ID: "item-1", CatalogID: catalog.ID, Category: "video", Status: "active", Title: "Movie",
		Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err := NewSQLiteCatalogItemRepository(db.Bun).Save(ctx, item); err != nil {
		db.Close()
		t.Fatal(err)
	}
	asset, _ := library.NewItemAsset(library.ItemAssetParams{
		ID: "asset-1", ItemID: item.ID, FileID: file.ID, Role: "original", CreatedAt: &now, UpdatedAt: &now,
	})
	if err := NewSQLiteItemAssetRepository(db.Bun).Save(ctx, asset); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db, now
}
