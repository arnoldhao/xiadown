package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/library/catalogaudit"
	"xiadown/internal/application/library/dto"
	application "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
)

type catalogServiceAuditorStub struct{ report catalogaudit.Report }

func (stub catalogServiceAuditorStub) Audit(_ context.Context, request catalogaudit.Request) (catalogaudit.Report, error) {
	report := stub.report
	report.CatalogID = request.CatalogID
	report.MigrationID = request.MigrationID
	return report, nil
}

func TestCatalogServiceItemLifecycleAndManagement(t *testing.T) {
	ctx := context.Background()
	db := openCatalogServiceTestDatabase(t, "catalog-service.db")

	legacyPath := filepath.Join(t.TempDir(), "movie.mp4")
	if err := os.WriteFile(legacyPath, []byte("media stays here"), 0o600); err != nil {
		t.Fatalf("write legacy asset: %v", err)
	}
	now := time.Now().UTC().Add(-time.Hour)
	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-main", Name: "Library", Status: "active", IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	catalogs := libraryrepo.NewSQLiteCatalogRepository(db.Bun)
	if err := catalogs.Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	legacyLibrary, err := library.NewLibrary(library.LibraryParams{ID: "bundle-1", Name: "Movie", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatalf("new legacy library: %v", err)
	}
	if err := libraryrepo.NewSQLiteLibraryRepository(db.Bun).Save(ctx, legacyLibrary); err != nil {
		t.Fatalf("save legacy library: %v", err)
	}
	legacyFile, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-1", LibraryID: legacyLibrary.ID, Kind: "video", Name: "movie.mp4",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: legacyPath},
		Origin: library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{
			BatchID: "batch-1", ImportPath: legacyPath, ImportedAt: now,
		}},
		Media: &library.MediaInfo{Format: "mp4"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new legacy file: %v", err)
	}
	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	if err := files.Save(ctx, legacyFile); err != nil {
		t.Fatalf("save legacy file: %v", err)
	}
	item, err := library.NewItem(library.ItemParams{
		ID: "item-1", CatalogID: catalog.ID, Category: "video", Status: "active", Title: "Movie",
		Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new item: %v", err)
	}
	items := libraryrepo.NewSQLiteCatalogItemRepository(db.Bun)
	if err := items.Save(ctx, item); err != nil {
		t.Fatalf("save item: %v", err)
	}
	asset, err := library.NewItemAsset(library.ItemAssetParams{
		ID: "asset-1", ItemID: item.ID, FileID: legacyFile.ID, Role: "original",
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new asset: %v", err)
	}
	assets := libraryrepo.NewSQLiteItemAssetRepository(db.Bun)
	if err := assets.Save(ctx, asset); err != nil {
		t.Fatalf("save asset: %v", err)
	}

	roots := libraryrepo.NewSQLiteStorageRootRepository(db.Bun)
	collections := libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun)
	tags := libraryrepo.NewSQLiteCatalogTagRepository(db.Bun)
	userStates := libraryrepo.NewSQLiteUserStateRepository(db.Bun)
	mutations := libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun)
	changeRepository := libraryrepo.NewSQLiteCatalogChangeRepository(db.Bun)
	service := application.NewCatalogService(
		catalogs, items, assets, files, roots, collections, tags, userStates, mutations,
		catalogServiceAuditorStub{report: catalogaudit.Report{AuditedAt: now}},
		changeRepository,
	)

	overview, err := service.GetDefaultCatalogOverview(ctx)
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if overview.Categories.All != 1 || overview.Categories.Video != 1 || overview.Health.AssetLinks != 1 {
		t.Fatalf("unexpected overview: %#v", overview)
	}
	listed, err := service.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{Category: "video", Query: "mov", Sort: "title_asc"})
	if err != nil || listed.Total != 1 || listed.Items[0].ID != item.ID ||
		listed.Items[0].Kind != "video" || listed.Items[0].Format != "mp4" ||
		listed.Items[0].PrimaryAssetID != asset.ID || listed.Items[0].PrimaryFileID != legacyFile.ID {
		t.Fatalf("unexpected item list: %#v, err=%v", listed, err)
	}
	snapshot, err := service.ListCatalogSnapshotItems(ctx, catalog.ID, "", 2)
	if err != nil || len(snapshot) != 1 || snapshot[0].ID != item.ID ||
		snapshot[0].Kind != "video" || snapshot[0].Format != "mp4" ||
		snapshot[0].PrimaryAssetID != asset.ID || snapshot[0].PrimaryFileID != legacyFile.ID {
		t.Fatalf("unexpected keyset snapshot summary: %#v, err=%v", snapshot, err)
	}
	detail, err := service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: item.ID})
	if err != nil || len(detail.Assets) != 1 || len(detail.Representations) != 1 ||
		detail.Representations[0].Kind != "original" || !detail.Assets[0].FileAvailable ||
		detail.Assets[0].File == nil || detail.Assets[0].File.Storage.LocalPath != legacyPath {
		t.Fatalf("unexpected item detail: %#v, err=%v", detail, err)
	}

	newTitle := "Edited Movie"
	updated, err := service.UpdateCatalogItem(ctx, dto.UpdateCatalogItemRequest{
		ID: item.ID, ExpectedRevision: 1, Title: &newTitle, ActorID: "desktop-user",
	})
	if err != nil || updated.Item.Revision != 2 || updated.Item.Title != newTitle {
		t.Fatalf("update item: %#v, err=%v", updated, err)
	}
	_, err = service.UpdateCatalogItem(ctx, dto.UpdateCatalogItemRequest{ID: item.ID, ExpectedRevision: 1, Title: &newTitle})
	if !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("expected stale update conflict, got %v", err)
	}
	trashed, err := service.TrashCatalogItem(ctx, dto.CatalogItemLifecycleRequest{
		ID: item.ID, ExpectedRevision: 2, ActorID: "desktop-user",
	})
	if err != nil || trashed.Item.Status != "trashed" || trashed.Item.Revision != 3 {
		t.Fatalf("trash item: %#v, err=%v", trashed, err)
	}
	visible, err := service.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{
		Status: "all", Limit: 1, ExcludeTrashed: true,
	})
	if err != nil || visible.Total != 0 || len(visible.Items) != 0 {
		t.Fatalf("normal browse must exclude trash before pagination: %#v, err=%v", visible, err)
	}
	snapshot, err = service.ListCatalogSnapshotItems(ctx, catalog.ID, "", 2)
	if err != nil || len(snapshot) != 0 {
		t.Fatalf("keyset snapshot must exclude trash: %#v, err=%v", snapshot, err)
	}
	trash, err := service.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{
		Status: "trashed", Limit: 1,
	})
	if err != nil || trash.Total != 1 || len(trash.Items) != 1 || trash.Items[0].ID != item.ID {
		t.Fatalf("maintenance trash listing must remain available: %#v, err=%v", trash, err)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("trash touched physical file: %v", err)
	}
	var tombstones int
	if err := db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_tombstones WHERE entity_id = ?", item.ID).Scan(&tombstones); err != nil || tombstones != 1 {
		t.Fatalf("expected current tombstone, count=%d err=%v", tombstones, err)
	}
	for _, noise := range []library.CatalogChange{
		{CatalogID: catalog.ID, EntityType: library.CatalogEntityItem, EntityID: item.ID, Kind: library.CatalogChangeUpsert, Revision: 30, ActorID: "system:projection", OccurredAt: time.Now().UTC()},
		{CatalogID: catalog.ID, EntityType: library.CatalogEntityItem, EntityID: item.ID, Kind: library.CatalogChangeUpsert, Revision: 31, ActorID: "", OccurredAt: time.Now().UTC()},
		{CatalogID: catalog.ID, EntityType: library.CatalogEntityItem, EntityID: "other-item", Kind: library.CatalogChangeUpsert, Revision: 1, ActorID: "desktop-user", OccurredAt: time.Now().UTC()},
	} {
		if err := changeRepository.Save(ctx, noise); err != nil {
			t.Fatalf("save activity noise: %v", err)
		}
	}
	restored, err := service.RestoreCatalogItem(ctx, dto.CatalogItemLifecycleRequest{
		ID: item.ID, ExpectedRevision: 3, ActorID: "desktop-user",
	})
	if err != nil || restored.Item.Status != "active" || restored.Item.Revision != 4 {
		t.Fatalf("restore item: %#v, err=%v", restored, err)
	}
	if err := db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_tombstones WHERE entity_id = ?", item.ID).Scan(&tombstones); err != nil || tombstones != 0 {
		t.Fatalf("restore left current tombstone, count=%d err=%v", tombstones, err)
	}
	changes, err := changeRepository.ListAfter(ctx, catalog.ID, 0, 10)
	if err != nil || len(changes) != 6 || changes[0].Kind != library.CatalogChangeUpsert || changes[1].Kind != library.CatalogChangeDelete || changes[5].Kind != library.CatalogChangeUpsert {
		t.Fatalf("unexpected item changes: %#v, err=%v", changes, err)
	}
	activity, err := service.ListCatalogItemActivity(ctx, dto.ListCatalogItemActivityRequest{ItemID: item.ID, Limit: 10})
	if err != nil || len(activity) != 3 ||
		activity[0].Action != "catalog_item_restored" || activity[0].Revision != 4 ||
		activity[1].Action != "catalog_item_trashed" || activity[1].Revision != 3 ||
		activity[2].Action != "catalog_item_updated" || activity[2].Revision != 2 {
		t.Fatalf("unexpected item activity: %#v, err=%v", activity, err)
	}
	for _, event := range activity {
		if event.Actor != "desktop-user" || event.OccurredAt == "" {
			t.Fatalf("activity lost actor/time: %#v", event)
		}
	}

	representations, err := service.ListCatalogRepresentations(ctx, dto.ListCatalogRepresentationsRequest{ItemID: item.ID})
	if err != nil || len(representations) != 1 {
		t.Fatalf("list representations: %#v, err=%v", representations, err)
	}
	width, height := 1920, 1080
	duration, bitrate, size := int64(90_000), int64(8_000_000), int64(42_000_000)
	representation, err := service.SaveCatalogRepresentation(ctx, dto.SaveCatalogRepresentationRequest{
		ID: representations[0].ID, ItemID: item.ID, AssetID: asset.ID,
		ExpectedRevision: representations[0].Revision, Kind: "optimized", Purpose: "playback",
		MediaType: "video/mp4", Container: "mp4", Codec: "h264", Width: &width, Height: &height,
		DurationMs: &duration, BitrateBps: &bitrate, SizeBytes: &size, Availability: "available",
		ActorID: "desktop-user",
	})
	if err != nil || representation.Revision != 2 || representation.Codec != "h264" {
		t.Fatalf("save representation: %#v, err=%v", representation, err)
	}
	_, err = service.SaveCatalogRepresentation(ctx, dto.SaveCatalogRepresentationRequest{
		ID: representation.ID, ItemID: item.ID, AssetID: asset.ID,
		ExpectedRevision: 1, Kind: "optimized", Availability: "available",
	})
	if !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("expected stale representation conflict, got %v", err)
	}
	confidence := 0.99
	metadata, err := service.SaveCatalogMetadataEntry(ctx, dto.SaveCatalogMetadataEntryRequest{
		ItemID: item.ID, RepresentationID: representation.ID,
		Namespace: "dc", Key: "title", ValueType: "string", ValueJSON: `"Movie"`, Language: "en",
		Source: "user", Provenance: "desktop-user", Confidence: &confidence, Locked: true,
		ActorID: "desktop-user",
	})
	if err != nil || metadata.Revision != 1 || metadata.ValueJSON != `"Movie"` {
		t.Fatalf("save metadata: %#v, err=%v", metadata, err)
	}
	metadataList, err := service.ListCatalogMetadataEntries(ctx, dto.ListCatalogMetadataEntriesRequest{
		ItemID: item.ID, RepresentationID: representation.ID,
	})
	if err != nil || len(metadataList) != 1 || metadataList[0].ID != metadata.ID {
		t.Fatalf("list representation metadata: %#v, err=%v", metadataList, err)
	}
	detail, err = service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: item.ID})
	if err != nil || len(detail.Representations) != 1 || len(detail.Metadata) != 1 || detail.Metadata[0].Locked != true {
		t.Fatalf("professional item detail: %#v, err=%v", detail, err)
	}

	collection, err := service.SaveCatalogCollection(ctx, dto.SaveCatalogCollectionRequest{Name: "Favorites", Kind: "manual"})
	if err != nil || collection.Revision != 1 {
		t.Fatalf("save collection: %#v, err=%v", collection, err)
	}
	collection, err = service.ReplaceCatalogCollectionItems(ctx, dto.ReplaceCatalogCollectionItemsRequest{
		CollectionID: collection.ID, ItemIDs: []string{item.ID}, ExpectedRevision: collection.Revision,
	})
	if err != nil || collection.Revision != 2 || len(collection.ItemIDs) != 1 {
		t.Fatalf("replace collection items: %#v, err=%v", collection, err)
	}
	collectionPage, err := service.ListCatalogCollectionsPage(ctx, 1, 0, 1)
	if err != nil || len(collectionPage) != 1 || collectionPage[0].ID != collection.ID {
		t.Fatalf("paged collections: %#v, err=%v", collectionPage, err)
	}
	memberPage, err := service.ListCatalogCollectionItemsPage(ctx, collection.ID, 1, 0)
	if err != nil || memberPage.CatalogID != collection.CatalogID || memberPage.CollectionID != collection.ID ||
		len(memberPage.Items) != 1 || memberPage.Items[0].ItemID != item.ID || memberPage.Items[0].Position != 0 ||
		memberPage.NextOffset != 1 || memberPage.HasMore {
		t.Fatalf("paged collection members: %#v, err=%v", memberPage, err)
	}
	taxonomyStateBefore, err := libraryrepo.NewSQLiteCatalogSyncStateRepository(db.Bun).GetCatalogSyncState(ctx, catalog.ID)
	if err != nil {
		t.Fatalf("taxonomy cursor before mutations: %v", err)
	}
	tag, err := service.SaveCatalogTag(ctx, dto.SaveCatalogTagRequest{Name: " Cinema "})
	if err != nil || tag.NormalizedName != "cinema" {
		t.Fatalf("save tag: %#v, err=%v", tag, err)
	}
	tagPage, err := service.ListCatalogTagsPage(ctx, 1, 0)
	if err != nil || len(tagPage) != 1 || tagPage[0].ID != tag.ID {
		t.Fatalf("paged tags: %#v, err=%v", tagPage, err)
	}
	itemTags, err := service.ReplaceCatalogItemTags(ctx, dto.ReplaceCatalogItemTagsRequest{ItemID: item.ID, TagIDs: []string{tag.ID}})
	if err != nil || len(itemTags) != 1 || itemTags[0].ID != tag.ID {
		t.Fatalf("replace item tags: %#v, err=%v", itemTags, err)
	}
	taxonomyChanges, err := libraryrepo.NewSQLiteCatalogChangeRepository(db.Bun).ListAfter(
		ctx, catalog.ID, taxonomyStateBefore.Cursor, 10,
	)
	if err != nil || len(taxonomyChanges) != 3 ||
		taxonomyChanges[0].EntityType != library.CatalogEntityTag || taxonomyChanges[0].EntityID != tag.ID ||
		taxonomyChanges[1].EntityType != library.CatalogEntityItemTag || taxonomyChanges[1].EntityID != item.ID ||
		taxonomyChanges[2].EntityType != library.CatalogEntityItem || taxonomyChanges[2].EntityID != item.ID {
		t.Fatalf("taxonomy service mutations missing from change feed: %#v, err=%v", taxonomyChanges, err)
	}
	taxonomyStateAfter, err := libraryrepo.NewSQLiteCatalogSyncStateRepository(db.Bun).GetCatalogSyncState(ctx, catalog.ID)
	if err != nil || taxonomyStateAfter.Cursor != taxonomyChanges[2].Sequence {
		t.Fatalf("taxonomy cursor after mutations: %#v, err=%v", taxonomyStateAfter, err)
	}
	if _, err := service.ReplaceCatalogItemTags(ctx, dto.ReplaceCatalogItemTagsRequest{ItemID: item.ID, TagIDs: []string{tag.ID}}); err != nil {
		t.Fatalf("repeat item tags mutation: %v", err)
	}
	repeatChanges, err := libraryrepo.NewSQLiteCatalogChangeRepository(db.Bun).ListAfter(
		ctx, catalog.ID, taxonomyStateAfter.Cursor, 10,
	)
	if err != nil || len(repeatChanges) != 0 {
		t.Fatalf("idempotent item-tag replacement advanced the change feed: %#v, err=%v", repeatChanges, err)
	}
	root, err := service.SaveCatalogStorageRoot(ctx, dto.SaveCatalogStorageRootRequest{
		Name: "Managed files", Path: filepath.Dir(legacyPath), Mode: "managed",
	})
	if err != nil || root.Status != "online" {
		t.Fatalf("save storage root: %#v, err=%v", root, err)
	}
	checked, err := service.CheckCatalogStorageRoot(ctx, dto.CheckCatalogStorageRootRequest{ID: root.ID})
	if err != nil || checked.Status != "online" || checked.LastCheckedAt == "" {
		t.Fatalf("check storage root: %#v, err=%v", checked, err)
	}
	managedPath, err := service.EnsureManagedImportRoot(ctx, filepath.Dir(legacyPath))
	expectedManagedPath, _ := filepath.EvalSymlinks(filepath.Dir(legacyPath))
	if err != nil || managedPath != expectedManagedPath {
		t.Fatalf("ensure managed import root: path=%q err=%v", managedPath, err)
	}
	rootedDetail, err := service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: item.ID})
	if err != nil || len(rootedDetail.Assets) != 1 || len(rootedDetail.Representations) != 1 ||
		rootedDetail.Assets[0].StorageRootID != root.ID || rootedDetail.Representations[0].StorageRootID != root.ID {
		t.Fatalf("storage root asset ownership missing: %#v, err=%v", rootedDetail, err)
	}
	rootedRepresentations, err := service.ListCatalogRepresentations(ctx, dto.ListCatalogRepresentationsRequest{ItemID: item.ID})
	if err != nil || len(rootedRepresentations) != 1 || rootedRepresentations[0].StorageRootID != root.ID {
		t.Fatalf("representation storage root ownership missing: %#v, err=%v", rootedRepresentations, err)
	}
	favorite := true
	progress := 0.5
	state, err := service.UpdateCatalogUserState(ctx, dto.UpdateCatalogUserStateRequest{
		ItemID: item.ID, UserID: "local-user", ExpectedRevision: 0,
		Favorite: &favorite, Progress: &progress, OpenedNow: true,
	})
	if err != nil || state.Revision != 1 || !state.Favorite || state.Progress != 0.5 || state.LastOpenedAt == "" {
		t.Fatalf("create user state: %#v, err=%v", state, err)
	}
	completed := true
	state, err = service.UpdateCatalogUserState(ctx, dto.UpdateCatalogUserStateRequest{
		ItemID: item.ID, UserID: "local-user", ExpectedRevision: state.Revision, Completed: &completed,
	})
	if err != nil || state.Revision != 2 || !state.Completed || state.Progress != 1 {
		t.Fatalf("update user state: %#v, err=%v", state, err)
	}
	_, err = service.UpdateCatalogUserState(ctx, dto.UpdateCatalogUserStateRequest{
		ItemID: item.ID, UserID: "local-user", ExpectedRevision: 1, Completed: &completed,
	})
	if !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("expected stale user state conflict, got %v", err)
	}
	audit, err := service.GetCatalogMigrationAudit(ctx, dto.CatalogMigrationAuditRequest{})
	if err != nil || !audit.Healthy || audit.MigrationID != application.LegacyCatalogProjectionID {
		t.Fatalf("get migration audit: %#v, err=%v", audit, err)
	}
}

func TestCatalogOverviewCountsOnlyDistinctActionableMissingLocalFiles(t *testing.T) {
	ctx := context.Background()
	db := openCatalogServiceTestDatabase(t, "catalog-overview.db")

	now := time.Now().UTC()
	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-overview", Name: "Library", Status: "active", IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	catalogs := libraryrepo.NewSQLiteCatalogRepository(db.Bun)
	if err := catalogs.Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	legacyLibrary, err := library.NewLibrary(library.LibraryParams{
		ID: "library-overview", Name: "Overview files", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}
	if err := libraryrepo.NewSQLiteLibraryRepository(db.Bun).Save(ctx, legacyLibrary); err != nil {
		t.Fatalf("save library: %v", err)
	}

	existingPath := filepath.Join(t.TempDir(), "existing.mp3")
	if err := os.WriteFile(existingPath, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	missingPath := filepath.Join(t.TempDir(), "missing.mp3")
	deletedPath := filepath.Join(t.TempDir(), "deleted.mp3")
	indeterminatePath := t.TempDir() // Directories are not proof that a file is missing.

	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	items := libraryrepo.NewSQLiteCatalogItemRepository(db.Bun)
	assets := libraryrepo.NewSQLiteItemAssetRepository(db.Bun)
	fileCases := []struct {
		id      string
		kind    string
		size    int64
		storage library.FileStorage
		state   library.FileState
	}{
		{id: "existing", kind: "audio", size: 5, storage: library.FileStorage{Mode: "local_path", LocalPath: existingPath}},
		{id: "missing", kind: "audio", size: 7, storage: library.FileStorage{Mode: "local_path", LocalPath: missingPath}},
		{id: "deleted", kind: "audio", size: 11, storage: library.FileStorage{Mode: "local_path", LocalPath: deletedPath}, state: library.FileState{Status: "deleted", Deleted: true, LastError: "missing_local_file"}},
		{id: "indeterminate", kind: "audio", size: 13, storage: library.FileStorage{Mode: "local_path", LocalPath: indeterminatePath}},
		{id: "document", kind: "subtitle", size: 17, storage: library.FileStorage{Mode: "db_document", DocumentID: "subtitle-document"}},
		{id: "dangling", kind: "audio", size: 19, storage: library.FileStorage{Mode: "local_path", LocalPath: existingPath}},
	}
	for _, testCase := range fileCases {
		size := testCase.size
		file, err := library.NewLibraryFile(library.LibraryFileParams{
			ID: testCase.id, LibraryID: legacyLibrary.ID, Kind: testCase.kind, Name: testCase.id + ".dat",
			Storage: testCase.storage, Origin: library.FileOrigin{Kind: "download", OperationID: "operation-" + testCase.id},
			Media: &library.MediaInfo{SizeBytes: &size}, State: testCase.state, CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new file %s: %v", testCase.id, err)
		}
		if err := files.Save(ctx, file); err != nil {
			t.Fatalf("save file %s: %v", testCase.id, err)
		}
		item, err := library.NewItem(library.ItemParams{
			ID: "item-" + testCase.id, CatalogID: catalog.ID, Category: "audio", Status: "active",
			Title: testCase.id, Revision: 1, CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new item %s: %v", testCase.id, err)
		}
		if err := items.Save(ctx, item); err != nil {
			t.Fatalf("save item %s: %v", testCase.id, err)
		}
		asset, err := library.NewItemAsset(library.ItemAssetParams{
			ID: "asset-" + testCase.id, ItemID: item.ID, FileID: file.ID, Role: "original",
			CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new asset %s: %v", testCase.id, err)
		}
		if err := assets.Save(ctx, asset); err != nil {
			t.Fatalf("save asset %s: %v", testCase.id, err)
		}
	}
	// Simulate a corrupt dangling Catalog asset. Integrity audit owns this case;
	// there is no Library file record for file maintenance to act on.
	conn, err := db.SQL.Conn(ctx)
	if err != nil {
		t.Fatalf("open sqlite connection: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		_ = conn.Close()
		t.Fatalf("disable foreign keys: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "DELETE FROM library_files WHERE id = 'dangling'"); err != nil {
		_ = conn.Close()
		t.Fatalf("delete dangling source: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = conn.Close()
		t.Fatalf("restore foreign keys: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close sqlite connection: %v", err)
	}

	// The health count is file-based, even if one physical file is linked from
	// more than one Catalog item.
	duplicateItem, err := library.NewItem(library.ItemParams{
		ID: "item-missing-duplicate", CatalogID: catalog.ID, Category: "audio", Status: "active",
		Title: "missing duplicate", Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new duplicate item: %v", err)
	}
	if err := items.Save(ctx, duplicateItem); err != nil {
		t.Fatalf("save duplicate item: %v", err)
	}
	duplicateAsset, err := library.NewItemAsset(library.ItemAssetParams{
		ID: "asset-missing-duplicate", ItemID: duplicateItem.ID, FileID: "missing", Role: "original",
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new duplicate asset: %v", err)
	}
	if err := assets.Save(ctx, duplicateAsset); err != nil {
		t.Fatalf("save duplicate asset: %v", err)
	}

	service := application.NewCatalogService(
		catalogs, items, assets, files,
		libraryrepo.NewSQLiteStorageRootRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun), nil,
	)
	overview, err := service.GetDefaultCatalogOverview(ctx)
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if overview.Health.AssetLinks != 7 || overview.Health.UnavailableAssetFiles != 1 {
		t.Fatalf("unexpected overview health: %#v", overview.Health)
	}
	if overview.TotalSizeBytes != 53 {
		t.Fatalf("overview total size = %d, want unique linked file size 53", overview.TotalSizeBytes)
	}
}

func TestCatalogServiceRejectsInvalidFiltersAndMissingStorageRoot(t *testing.T) {
	ctx := context.Background()
	db := openCatalogServiceTestDatabase(t, "catalog-validation.db")
	now := time.Now().UTC()
	catalog, _ := library.NewCatalog(library.CatalogParams{ID: "catalog", Name: "Library", Status: "active", IsDefault: true, CreatedAt: &now, UpdatedAt: &now})
	catalogs := libraryrepo.NewSQLiteCatalogRepository(db.Bun)
	if err := catalogs.Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	service := application.NewCatalogService(
		catalogs,
		libraryrepo.NewSQLiteCatalogItemRepository(db.Bun),
		libraryrepo.NewSQLiteItemAssetRepository(db.Bun),
		libraryrepo.NewSQLiteFileRepository(db.Bun),
		libraryrepo.NewSQLiteStorageRootRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun), nil,
	)
	if _, err := service.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{Category: "executable"}); !errors.Is(err, library.ErrInvalidCatalogItem) {
		t.Fatalf("expected invalid category, got %v", err)
	}
	root, err := service.SaveCatalogStorageRoot(ctx, dto.SaveCatalogStorageRootRequest{
		Name: "Disconnected disk", Path: filepath.Join(t.TempDir(), "missing"), Mode: "referenced",
	})
	if err != nil || root.Status != "offline" || root.LastError != "" {
		t.Fatalf("unexpected missing root status: %#v, err=%v", root, err)
	}
	managedDirectory := t.TempDir()
	managedPath, err := service.EnsureManagedImportRoot(ctx, managedDirectory)
	expectedManagedDirectory, _ := filepath.EvalSymlinks(managedDirectory)
	if err != nil || managedPath != expectedManagedDirectory {
		t.Fatalf("auto-register managed import root: path=%q err=%v", managedPath, err)
	}
	roots, err := service.ListCatalogStorageRoots(ctx)
	hasManagedRoot := false
	for _, item := range roots {
		hasManagedRoot = hasManagedRoot || (item.Mode == "managed" && item.Path == expectedManagedDirectory)
	}
	if err != nil || len(roots) != 2 || !hasManagedRoot {
		t.Fatalf("managed root was not persisted: %#v, err=%v", roots, err)
	}
}
