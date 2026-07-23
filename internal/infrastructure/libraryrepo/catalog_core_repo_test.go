package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSQLiteCatalogSnapshotPageUsesStablePartialKeysetIndex(t *testing.T) {
	ctx := context.Background()
	db, err := openLibraryRepoTestDatabase(t, ctx, "catalog-snapshot.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 22, 9, 30, 0, 0, time.UTC)
	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-snapshot", Name: "Snapshot", Status: "active", IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	if err := NewSQLiteCatalogRepository(db.Bun).Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}

	items := NewSQLiteCatalogItemRepository(db.Bun)
	for index, id := range []string{"item-001", "item-002", "item-003", "item-004", "item-005"} {
		status := library.ItemStatusActive
		var trashedAt *time.Time
		if id == "item-003" {
			status = library.ItemStatusTrashed
			value := now.Add(time.Duration(index) * time.Minute)
			trashedAt = &value
		}
		updatedAt := now.Add(time.Duration(index) * time.Minute)
		item, itemErr := library.NewItem(library.ItemParams{
			ID: id, CatalogID: catalog.ID, Category: "audio", Status: string(status),
			Title: fmt.Sprintf("Track %d", index+1), Revision: 1, TrashedAt: trashedAt,
			CreatedAt: &now, UpdatedAt: &updatedAt,
		})
		if itemErr != nil {
			t.Fatalf("new item %s: %v", id, itemErr)
		}
		if err := items.Save(ctx, item); err != nil {
			t.Fatalf("save item %s: %v", id, err)
		}
	}

	first, err := items.ListSnapshotPageByCatalogID(ctx, catalog.ID, "", 2)
	if err != nil || len(first) != 2 || first[0].ID != "item-001" || first[1].ID != "item-002" {
		t.Fatalf("first keyset page=%#v err=%v", first, err)
	}
	second, err := items.ListSnapshotPageByCatalogID(ctx, catalog.ID, first[1].ID, 3)
	if err != nil || len(second) != 2 || second[0].ID != "item-004" || second[1].ID != "item-005" {
		t.Fatalf("second keyset page=%#v err=%v", second, err)
	}

	planRows, err := db.SQL.QueryContext(ctx, `
EXPLAIN QUERY PLAN
SELECT * FROM library_catalog_items
WHERE catalog_id = ? AND status <> 'trashed' AND trashed_at IS NULL AND id > ?
ORDER BY id ASC LIMIT ?
`, catalog.ID, "item-002", 200)
	if err != nil {
		t.Fatalf("explain snapshot keyset: %v", err)
	}
	defer planRows.Close()
	var plan strings.Builder
	for planRows.Next() {
		var id, parent, unused int
		var detail string
		if err := planRows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatalf("scan query plan: %v", err)
		}
		plan.WriteString(detail)
		plan.WriteByte('\n')
	}
	if err := planRows.Err(); err != nil {
		t.Fatalf("read query plan: %v", err)
	}
	detail := plan.String()
	if !strings.Contains(detail, "library_catalog_items_snapshot_idx") ||
		strings.Contains(strings.ToUpper(detail), "USE TEMP B-TREE") {
		t.Fatalf("snapshot query is not an indexed keyset:\n%s", detail)
	}
}

func TestSQLiteCatalogCoreRepositoriesRoundTripAndAtomicReplace(t *testing.T) {
	ctx := context.Background()
	db, err := openLibraryRepoTestDatabase(t, ctx, "catalog-core.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 13, 9, 30, 0, 0, time.UTC)
	catalogRepo := NewSQLiteCatalogRepository(db.Bun)
	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-main", Name: "Library", Description: "Main catalog", Status: "active",
		IsDefault: true, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	if err := catalogRepo.Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	catalog.Description = "Updated catalog"
	catalog.UpdatedAt = now.Add(time.Minute)
	if err := catalogRepo.Save(ctx, catalog); err != nil {
		t.Fatalf("update catalog: %v", err)
	}
	loadedCatalog, err := catalogRepo.Get(ctx, catalog.ID)
	if err != nil || loadedCatalog.Description != "Updated catalog" || !loadedCatalog.CreatedAt.Equal(now) {
		t.Fatalf("unexpected catalog: %#v, err=%v", loadedCatalog, err)
	}
	catalogs, err := catalogRepo.List(ctx)
	if err != nil || len(catalogs) != 1 || catalogs[0].ID != catalog.ID {
		t.Fatalf("unexpected catalogs: %#v, err=%v", catalogs, err)
	}

	legacyLibrary, err := library.NewLibrary(library.LibraryParams{
		ID: "legacy-bundle", Name: "Legacy bundle", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new legacy library: %v", err)
	}
	if err := NewSQLiteLibraryRepository(db.Bun).Save(ctx, legacyLibrary); err != nil {
		t.Fatalf("save legacy library: %v", err)
	}
	legacyFile, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-movie", LibraryID: legacyLibrary.ID, Kind: string(library.FileKindVideo), Name: "movie.mp4",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: filepath.Join(t.TempDir(), "movie.mp4")},
		Origin: library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{
			BatchID: "batch-1", ImportPath: filepath.Join(t.TempDir(), "movie.mp4"), ImportedAt: now,
		}},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new legacy file: %v", err)
	}
	if err := NewSQLiteFileRepository(db.Bun).Save(ctx, legacyFile); err != nil {
		t.Fatalf("save legacy file: %v", err)
	}

	itemRepo := NewSQLiteCatalogItemRepository(db.Bun)
	itemA, err := library.NewItem(library.ItemParams{
		ID: "item-a", CatalogID: catalog.ID, Category: "video", Status: "active", Title: "A Movie",
		Description: "First", Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new item A: %v", err)
	}
	itemBTime := now.Add(2 * time.Minute)
	itemB, err := library.NewItem(library.ItemParams{
		ID: "item-b", CatalogID: catalog.ID, Category: "book", Status: "active", Title: "B Book",
		Revision: 1, CreatedAt: &now, UpdatedAt: &itemBTime,
	})
	if err != nil {
		t.Fatalf("new item B: %v", err)
	}
	for _, item := range []library.Item{itemA, itemB} {
		if err := itemRepo.Save(ctx, item); err != nil {
			t.Fatalf("save item %s: %v", item.ID, err)
		}
	}
	items, err := itemRepo.ListByCatalogID(ctx, catalog.ID)
	if err != nil || len(items) != 2 || items[0].ID != itemB.ID || items[1].ID != itemA.ID {
		t.Fatalf("unexpected catalog items: %#v, err=%v", items, err)
	}
	if _, err := db.SQL.ExecContext(ctx, `UPDATE library_catalog_items SET subtype = 'movie', metadata_json = '{"year":2026}' WHERE id = ?`, itemA.ID); err != nil {
		t.Fatalf("seed catalog-only item fields: %v", err)
	}
	itemA.Description = "Updated first"
	itemA.Revision = 2
	itemA.UpdatedAt = now.Add(3 * time.Minute)
	if err := itemRepo.Save(ctx, itemA); err != nil {
		t.Fatalf("update item A: %v", err)
	}
	loadedItem, err := itemRepo.Get(ctx, itemA.ID)
	if err != nil || loadedItem.Description != "Updated first" || loadedItem.Revision != 2 {
		t.Fatalf("unexpected item A: %#v, err=%v", loadedItem, err)
	}
	var subtype, metadataJSON string
	if err := db.SQL.QueryRowContext(ctx, "SELECT subtype, metadata_json FROM library_catalog_items WHERE id = ?", itemA.ID).Scan(&subtype, &metadataJSON); err != nil {
		t.Fatalf("read catalog-only item fields: %v", err)
	}
	if subtype != "movie" || metadataJSON != `{"year":2026}` {
		t.Fatalf("catalog-only item fields changed: subtype=%q metadata=%q", subtype, metadataJSON)
	}

	assetRepo := NewSQLiteItemAssetRepository(db.Bun)
	asset, err := library.NewItemAsset(library.ItemAssetParams{
		ID: "asset-a", ItemID: itemA.ID, FileID: legacyFile.ID, Role: "original", Label: "Original",
		Position: 0, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new item asset: %v", err)
	}
	if err := assetRepo.Save(ctx, asset); err != nil {
		t.Fatalf("save item asset: %v", err)
	}
	assets, err := assetRepo.ListByItemID(ctx, itemA.ID)
	if err != nil || len(assets) != 1 || assets[0].FileID != legacyFile.ID {
		t.Fatalf("unexpected item assets: %#v, err=%v", assets, err)
	}
	loadedAsset, err := assetRepo.Get(ctx, asset.ID)
	if err != nil || loadedAsset.Label != "Original" {
		t.Fatalf("unexpected item asset: %#v, err=%v", loadedAsset, err)
	}

	rootRepo := NewSQLiteStorageRootRepository(db.Bun)
	root, err := library.NewStorageRoot(library.StorageRootParams{
		ID: "root-main", CatalogID: catalog.ID, Name: "Media", Path: filepath.Join(t.TempDir(), "media"),
		VolumeID: "volume-1", Mode: "managed", Status: "online", LastCheckedAt: &now,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new storage root: %v", err)
	}
	if err := rootRepo.Save(ctx, root); err != nil {
		t.Fatalf("save storage root: %v", err)
	}
	roots, err := rootRepo.ListByCatalogID(ctx, catalog.ID)
	if err != nil || len(roots) != 1 || roots[0].Path != root.Path {
		t.Fatalf("unexpected storage roots: %#v, err=%v", roots, err)
	}
	loadedRoot, err := rootRepo.Get(ctx, root.ID)
	if err != nil || loadedRoot.VolumeID != "volume-1" {
		t.Fatalf("unexpected storage root: %#v, err=%v", loadedRoot, err)
	}

	collectionRepo := NewSQLiteCatalogCollectionRepository(db.Bun)
	collection, err := library.NewCollection(library.CollectionParams{
		ID: "collection-queue", CatalogID: catalog.ID, Name: "Queue", Kind: "playlist", Revision: 1,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new collection: %v", err)
	}
	if err := collectionRepo.Save(ctx, collection); err != nil {
		t.Fatalf("save collection: %v", err)
	}
	collectionItemA, _ := library.NewCollectionItem("collection-item-a", collection.ID, itemA.ID, 0, now)
	collection.UpdatedAt = now.Add(4 * time.Minute)
	collection.Revision = 2
	if err := collectionRepo.ReplaceItems(ctx, collection, []library.CollectionItem{collectionItemA}); err != nil {
		t.Fatalf("replace collection items: %v", err)
	}
	collectionItems, err := collectionRepo.ListItems(ctx, collection.ID)
	if err != nil || len(collectionItems) != 1 || collectionItems[0].ItemID != itemA.ID {
		t.Fatalf("unexpected collection items: %#v, err=%v", collectionItems, err)
	}
	collections, err := collectionRepo.ListByCatalogID(ctx, catalog.ID)
	if err != nil || len(collections) != 1 || collections[0].Revision != 2 {
		t.Fatalf("unexpected collections: %#v, err=%v", collections, err)
	}

	brokenCollection := collection
	brokenCollection.Name = "Must Roll Back"
	brokenCollection.Revision = 3
	brokenCollection.UpdatedAt = now.Add(5 * time.Minute)
	duplicateA, _ := library.NewCollectionItem("collection-item-duplicate-a", collection.ID, itemB.ID, 0, now)
	duplicateB, _ := library.NewCollectionItem("collection-item-duplicate-b", collection.ID, itemB.ID, 1, now)
	if err := collectionRepo.ReplaceItems(ctx, brokenCollection, []library.CollectionItem{duplicateA, duplicateB}); err == nil {
		t.Fatal("duplicate collection membership unexpectedly succeeded")
	}
	loadedCollection, err := collectionRepo.Get(ctx, collection.ID)
	if err != nil || loadedCollection.Name != "Queue" || loadedCollection.Revision != 2 {
		t.Fatalf("collection metadata was not rolled back: %#v, err=%v", loadedCollection, err)
	}
	collectionItems, err = collectionRepo.ListItems(ctx, collection.ID)
	if err != nil || len(collectionItems) != 1 || collectionItems[0].ID != collectionItemA.ID {
		t.Fatalf("collection membership was not rolled back: %#v, err=%v", collectionItems, err)
	}

	collectionItemB, _ := library.NewCollectionItem("collection-item-b", collection.ID, itemB.ID, 0, now.Add(time.Second))
	collectionItemA.Position = 1
	collection.Name = "Watch and read"
	collection.Revision = 3
	collection.UpdatedAt = now.Add(6 * time.Minute)
	if err := collectionRepo.ReplaceItems(ctx, collection, []library.CollectionItem{collectionItemB, collectionItemA}); err != nil {
		t.Fatalf("reorder collection items: %v", err)
	}
	collectionItems, err = collectionRepo.ListItems(ctx, collection.ID)
	if err != nil || len(collectionItems) != 2 || collectionItems[0].ItemID != itemB.ID || collectionItems[1].ItemID != itemA.ID {
		t.Fatalf("unexpected reordered collection: %#v, err=%v", collectionItems, err)
	}
	collectionPage, err := collectionRepo.ListByCatalogIDPage(ctx, catalog.ID, 1, 0)
	if err != nil || len(collectionPage) != 1 || collectionPage[0].ID != collection.ID {
		t.Fatalf("unexpected collection page: %#v, err=%v", collectionPage, err)
	}
	memberPage, err := collectionRepo.ListItemsPage(ctx, collection.ID, 1, 1)
	if err != nil || len(memberPage) != 1 || memberPage[0].ItemID != itemA.ID || memberPage[0].Position != 1 {
		t.Fatalf("unexpected collection member page: %#v, err=%v", memberPage, err)
	}

	tagRepo := NewSQLiteCatalogTagRepository(db.Bun)
	tagA, err := library.NewTag(library.TagParams{ID: "tag-a", CatalogID: catalog.ID, Name: "Favorite", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatalf("new tag A: %v", err)
	}
	tagB, err := library.NewTag(library.TagParams{ID: "tag-b", CatalogID: catalog.ID, Name: "Later", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatalf("new tag B: %v", err)
	}
	for _, tag := range []library.Tag{tagB, tagA} {
		if err := tagRepo.Save(ctx, tag); err != nil {
			t.Fatalf("save tag %s: %v", tag.ID, err)
		}
	}
	tagPage, err := tagRepo.ListByCatalogIDPage(ctx, catalog.ID, 1, 1)
	if err != nil || len(tagPage) != 1 || tagPage[0].ID != tagB.ID {
		t.Fatalf("unexpected tag page: %#v, err=%v", tagPage, err)
	}
	tags, err := tagRepo.ListByCatalogID(ctx, catalog.ID)
	if err != nil || len(tags) != 2 || tags[0].ID != tagA.ID || tags[1].ID != tagB.ID {
		t.Fatalf("unexpected tags: %#v, err=%v", tags, err)
	}
	itemTagA, _ := library.NewItemTag("item-tag-a", itemA.ID, tagA.ID, "", now)
	if err := tagRepo.ReplaceItemTags(ctx, itemA.ID, []library.ItemTag{itemTagA}); err != nil {
		t.Fatalf("replace item tags: %v", err)
	}
	itemTags, err := tagRepo.ListByItemID(ctx, itemA.ID)
	if err != nil || len(itemTags) != 1 || itemTags[0].TagID != tagA.ID {
		t.Fatalf("unexpected item tags: %#v, err=%v", itemTags, err)
	}
	duplicateTagA, _ := library.NewItemTag("item-tag-duplicate-a", itemA.ID, tagB.ID, "", now)
	duplicateTagB, _ := library.NewItemTag("item-tag-duplicate-b", itemA.ID, tagB.ID, "", now.Add(time.Second))
	if err := tagRepo.ReplaceItemTags(ctx, itemA.ID, []library.ItemTag{duplicateTagA, duplicateTagB}); err == nil {
		t.Fatal("duplicate item tags unexpectedly succeeded")
	}
	itemTags, err = tagRepo.ListByItemID(ctx, itemA.ID)
	if err != nil || len(itemTags) != 1 || itemTags[0].ID != itemTagA.ID {
		t.Fatalf("item tags were not rolled back: %#v, err=%v", itemTags, err)
	}
	itemTagB, _ := library.NewItemTag("item-tag-b", itemA.ID, tagB.ID, "", now.Add(time.Second))
	if err := tagRepo.ReplaceItemTags(ctx, itemA.ID, []library.ItemTag{itemTagA, itemTagB}); err != nil {
		t.Fatalf("replace item tags with valid set: %v", err)
	}
	itemTags, err = tagRepo.ListByItemID(ctx, itemA.ID)
	if err != nil || len(itemTags) != 2 || itemTags[0].TagID != tagA.ID || itemTags[1].TagID != tagB.ID {
		t.Fatalf("unexpected replaced item tags: %#v, err=%v", itemTags, err)
	}

	if err := assetRepo.Delete(ctx, asset.ID); err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	if _, err := assetRepo.Get(ctx, asset.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted asset error = %v, want sql.ErrNoRows", err)
	}
	if err := rootRepo.Delete(ctx, root.ID); err != nil {
		t.Fatalf("delete storage root: %v", err)
	}
	if err := collectionRepo.Delete(ctx, collection.ID); err != nil {
		t.Fatalf("delete collection: %v", err)
	}
	if err := tagRepo.Delete(ctx, tagA.ID); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	if err := itemRepo.Delete(ctx, itemB.ID); err != nil {
		t.Fatalf("delete item: %v", err)
	}
	if err := catalogRepo.Delete(ctx, catalog.ID); err != nil {
		t.Fatalf("delete catalog: %v", err)
	}
	if _, err := catalogRepo.Get(ctx, catalog.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted catalog error = %v, want sql.ErrNoRows", err)
	}
}

func TestSQLiteCatalogCoreReplaceRejectsInvalidInputBeforeDeleting(t *testing.T) {
	ctx := context.Background()
	db, err := openLibraryRepoTestDatabase(t, ctx, "catalog-validation.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	collectionRepo := NewSQLiteCatalogCollectionRepository(db.Bun)
	if err := collectionRepo.ReplaceItems(ctx, library.Collection{}, nil); !errors.Is(err, library.ErrInvalidCollection) {
		t.Fatalf("empty collection error = %v, want ErrInvalidCollection", err)
	}
	tagRepo := NewSQLiteCatalogTagRepository(db.Bun)
	if err := tagRepo.ReplaceItemTags(ctx, "", nil); !errors.Is(err, library.ErrInvalidItemTag) {
		t.Fatalf("empty item ID error = %v, want ErrInvalidItemTag", err)
	}
	if err := tagRepo.ReplaceItemTags(ctx, "missing-item", nil); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing item error = %v, want sql.ErrNoRows", err)
	}
}
