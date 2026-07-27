package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	roots := libraryrepo.NewSQLiteStorageRootRepository(db.Bun)
	storageRoot, err := library.NewStorageRoot(library.StorageRootParams{
		ID: "root-managed", CatalogID: catalog.ID, Name: "Managed files",
		Path: filepath.Dir(legacyPath), VolumeID: "volume-managed",
		Mode: "managed", Status: "online", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new storage root: %v", err)
	}
	if err := roots.Save(ctx, storageRoot); err != nil {
		t.Fatalf("save storage root: %v", err)
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
		detail.Assets[0].File == nil || detail.Assets[0].File.Storage.LocalPath != legacyPath ||
		detail.Source == nil || detail.Source.OriginKind != "import" ||
		detail.Source.StorageMode != "managed" ||
		detail.Source.StorageRootID != storageRoot.ID ||
		detail.Source.ImportBatchID != "batch-1" ||
		detail.Source.ImportPath != legacyPath {
		t.Fatalf("unexpected item detail: %#v, err=%v", detail, err)
	}
	if listed.Items[0].Availability != "available" ||
		detail.Item.Availability != "available" ||
		detail.Assets[0].Availability != "available" {
		t.Fatalf("healthy availability was not projected: list=%#v detail=%#v", listed.Items[0], detail)
	}

	offlineRoot := storageRoot
	offlineRoot.Status = library.StorageRootStatusOffline
	offlineRoot.VolumeID = ""
	offlineRoot.UpdatedAt = time.Now().UTC()
	if err := roots.Save(ctx, offlineRoot); err != nil {
		t.Fatalf("mark root offline: %v", err)
	}
	offline, err := service.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{Category: "video"})
	if err != nil || len(offline.Items) != 1 || offline.Items[0].Availability != "offline" {
		t.Fatalf("offline item availability: %#v err=%v", offline, err)
	}
	offlineDetail, err := service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: item.ID})
	if err != nil || offlineDetail.Item.Availability != "offline" ||
		offlineDetail.Assets[0].FileAvailable ||
		offlineDetail.Assets[0].Availability != "offline" ||
		offlineDetail.Representations[0].Availability != "offline" {
		t.Fatalf("offline detail availability: %#v err=%v", offlineDetail, err)
	}

	availabilityChanges := make([]string, 0, 1)
	service.SetCatalogAvailabilityChangeNotifier(func(_ context.Context, rootID string) {
		availabilityChanges = append(availabilityChanges, rootID)
	})
	onlineRoot, err := service.CheckCatalogStorageRoot(
		ctx,
		dto.CheckCatalogStorageRootRequest{ID: storageRoot.ID},
	)
	if err != nil || onlineRoot.Status != "online" ||
		len(availabilityChanges) != 1 || availabilityChanges[0] != storageRoot.ID {
		t.Fatalf("restore root online: root=%#v changes=%v err=%v", onlineRoot, availabilityChanges, err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
INSERT INTO library_storage_root_sync_entries (
  root_id, relative_path, size_bytes, modified_unix_nano, content_hash,
  file_id, status, last_seen_generation, last_error, created_at, updated_at
) VALUES (?, ?, 0, 0, '', ?, 'missing', 1, '', ?, ?)
`, storageRoot.ID, filepath.Base(legacyPath), legacyFile.ID, now, now); err != nil {
		t.Fatalf("seed missing sync entry: %v", err)
	}
	missing, err := service.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{Category: "video"})
	if err != nil || len(missing.Items) != 1 || missing.Items[0].Availability != "missing" {
		t.Fatalf("missing item availability: %#v err=%v", missing, err)
	}
	missingDetail, err := service.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: item.ID})
	if err != nil || missingDetail.Item.Availability != "missing" ||
		missingDetail.Assets[0].FileAvailable ||
		missingDetail.Assets[0].Availability != "missing" ||
		missingDetail.Representations[0].Availability != "missing" {
		t.Fatalf("missing detail availability: %#v err=%v", missingDetail, err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
UPDATE library_storage_root_sync_entries
SET status = 'active', updated_at = ?
WHERE root_id = ? AND file_id = ?
`, time.Now().UTC(), storageRoot.ID, legacyFile.ID); err != nil {
		t.Fatalf("restore active sync entry: %v", err)
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
	offlineForRestore := storageRoot
	offlineForRestore.Status = library.StorageRootStatusOffline
	offlineForRestore.UpdatedAt = time.Now().UTC()
	if err := roots.Save(ctx, offlineForRestore); err != nil {
		t.Fatalf("mark root offline before lifecycle restore: %v", err)
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
	if err != nil || restored.Item.Status != "active" ||
		restored.Item.Availability != "offline" ||
		restored.Item.Revision != 4 {
		t.Fatalf("restore item: %#v, err=%v", restored, err)
	}
	onlineAfterRestore := storageRoot
	onlineAfterRestore.UpdatedAt = time.Now().UTC()
	if err := roots.Save(ctx, onlineAfterRestore); err != nil {
		t.Fatalf("restore root after lifecycle test: %v", err)
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
		ID: storageRoot.ID, Name: "Managed files",
		Path: filepath.Dir(legacyPath), Mode: "managed",
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

	rootPath := t.TempDir()
	roots := libraryrepo.NewSQLiteStorageRootRepository(db.Bun)
	root, err := library.NewStorageRoot(library.StorageRootParams{
		ID: "root-overview", CatalogID: catalog.ID, Name: "Overview root",
		Path: rootPath, VolumeID: "volume-overview",
		Mode: "referenced", Status: "online", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new overview root: %v", err)
	}
	if err := roots.Save(ctx, root); err != nil {
		t.Fatalf("save overview root: %v", err)
	}
	existingPath := filepath.Join(rootPath, "existing.mp3")
	if err := os.WriteFile(existingPath, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	missingPath := filepath.Join(rootPath, "missing.mp3")
	deletedPath := filepath.Join(rootPath, "deleted.mp3")
	for _, path := range []string{missingPath, deletedPath} {
		if err := os.WriteFile(path, []byte("indexed"), 0o600); err != nil {
			t.Fatalf("write indexed fixture: %v", err)
		}
	}
	indeterminatePath := filepath.Join(rootPath, "indeterminate")
	if err := os.MkdirAll(indeterminatePath, 0o755); err != nil {
		t.Fatalf("create indeterminate directory: %v", err)
	}

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
	for _, path := range []string{missingPath, deletedPath} {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove indexed fixture: %v", err)
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
		roots,
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun), nil,
	)
	overview, err := service.GetDefaultCatalogOverview(ctx)
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if overview.Health.AssetLinks != 5 || overview.Health.UnavailableAssetFiles != 1 {
		t.Fatalf("unexpected overview health: %#v", overview.Health)
	}
	if overview.TotalSizeBytes != 36 {
		t.Fatalf("overview total size = %d, want rooted unique file size 36", overview.TotalSizeBytes)
	}
}

func TestCatalogBrowseOnlyReturnsItemsBackedByStorageRoots(t *testing.T) {
	ctx := context.Background()
	db := openCatalogServiceTestDatabase(t, "catalog-storage-scope.db")
	now := time.Now().UTC()

	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-storage-scope", Name: "Library",
		Status: "active", IsDefault: true, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	catalogs := libraryrepo.NewSQLiteCatalogRepository(db.Bun)
	if err := catalogs.Save(ctx, catalog); err != nil {
		t.Fatal(err)
	}
	rootPath := t.TempDir()
	root, err := library.NewStorageRoot(library.StorageRootParams{
		ID: "root-storage-scope", CatalogID: catalog.ID, Name: "Reference",
		Path: rootPath, VolumeID: "volume-storage-scope",
		Mode: "referenced", Status: "online", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	roots := libraryrepo.NewSQLiteStorageRootRepository(db.Bun)
	if err := roots.Save(ctx, root); err != nil {
		t.Fatal(err)
	}
	bundle, err := library.NewLibrary(library.LibraryParams{
		ID: "bundle-storage-scope", Name: "Files",
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := libraryrepo.NewSQLiteLibraryRepository(db.Bun).Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	insidePath := filepath.Join(rootPath, "inside.mp4")
	artworkPath := filepath.Join(rootPath, "outside-cover.jpg")
	outsidePath := filepath.Join(t.TempDir(), "outside.mp4")
	for path, body := range map[string]string{
		insidePath: "inside", artworkPath: "cover", outsidePath: "outside",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	for _, file := range []struct {
		id, kind, name, path string
	}{
		{"file-inside", "video", "inside.mp4", insidePath},
		{"file-outside", "video", "outside.mp4", outsidePath},
		{"file-outside-artwork", "thumbnail", "outside-cover.jpg", artworkPath},
	} {
		item, buildErr := library.NewLibraryFile(library.LibraryFileParams{
			ID: file.id, LibraryID: bundle.ID, Kind: file.kind,
			Name: file.name, Storage: library.FileStorage{
				Mode: "local_path", LocalPath: file.path,
			},
			Origin: library.FileOrigin{
				Kind: "import",
				Import: &library.ImportOrigin{
					BatchID:    "batch-storage-scope",
					ImportPath: file.path, ImportedAt: now,
				},
			},
			CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := files.Save(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	items := libraryrepo.NewSQLiteCatalogItemRepository(db.Bun)
	assets := libraryrepo.NewSQLiteItemAssetRepository(db.Bun)
	for _, item := range []struct {
		id, title, fileID string
	}{
		{"item-inside", "Inside", "file-inside"},
		{"item-outside", "Outside", "file-outside"},
	} {
		catalogItem, buildErr := library.NewItem(library.ItemParams{
			ID: item.id, CatalogID: catalog.ID, Category: "video",
			Status: "active", Title: item.title, Revision: 1,
			CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := items.Save(ctx, catalogItem); err != nil {
			t.Fatal(err)
		}
		asset, buildErr := library.NewItemAsset(library.ItemAssetParams{
			ID: "asset-" + item.id, ItemID: item.id, FileID: item.fileID,
			Role: "original", CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := assets.Save(ctx, asset); err != nil {
			t.Fatal(err)
		}
	}
	artwork, err := library.NewItemAsset(library.ItemAssetParams{
		ID: "asset-outside-artwork", ItemID: "item-outside",
		FileID: "file-outside-artwork", Role: "artwork",
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := assets.Save(ctx, artwork); err != nil {
		t.Fatal(err)
	}

	service := application.NewCatalogService(
		catalogs, items, assets, files, roots,
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun), nil,
	)
	listed, err := service.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{
		Status: "all", ExcludeTrashed: true,
	})
	if err != nil || listed.Total != 1 || len(listed.Items) != 1 ||
		listed.Items[0].ID != "item-inside" {
		t.Fatalf("storage-scoped browse = %#v, err=%v", listed, err)
	}
	snapshot, err := service.ListCatalogSnapshotItems(ctx, catalog.ID, "", 10)
	if err != nil || len(snapshot) != 1 || snapshot[0].ID != "item-inside" {
		t.Fatalf("storage-scoped snapshot = %#v, err=%v", snapshot, err)
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
	if root.Emoji == "" {
		t.Fatal("new storage root did not receive a persisted emoji")
	}
	fox := "🦊"
	root, err = service.UpdateCatalogStorageRoot(ctx, dto.UpdateCatalogStorageRootRequest{
		ID: root.ID, Name: root.Name, Mode: root.Mode, Emoji: &fox,
	})
	if err != nil || root.Emoji != fox {
		t.Fatalf("update storage root emoji: root=%#v err=%v", root, err)
	}
	invalidEmoji := "not an emoji"
	if _, err := service.UpdateCatalogStorageRoot(ctx, dto.UpdateCatalogStorageRootRequest{
		ID: root.ID, Name: root.Name, Mode: root.Mode, Emoji: &invalidEmoji,
	}); err == nil {
		t.Fatal("expected invalid storage root emoji to be rejected")
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

func TestCatalogServiceDefaultRootBackfillLifecycleAndRelocation(t *testing.T) {
	ctx := context.Background()
	db := openCatalogServiceTestDatabase(t, "catalog-storage-root-lifecycle.db")
	now := time.Now().UTC()

	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-storage", Name: "Library", Status: "active", IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	catalogs := libraryrepo.NewSQLiteCatalogRepository(db.Bun)
	if err := catalogs.Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	bundle, err := library.NewLibrary(library.LibraryParams{
		ID: "library-storage", Name: "Existing Library", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new Library: %v", err)
	}
	if err := libraryrepo.NewSQLiteLibraryRepository(db.Bun).Save(ctx, bundle); err != nil {
		t.Fatalf("save Library: %v", err)
	}
	downloadParent := t.TempDir()
	downloadRoot := filepath.Join(downloadParent, "xiadown")
	if err := os.MkdirAll(filepath.Join(downloadRoot, "resource"), 0o755); err != nil {
		t.Fatalf("create download root: %v", err)
	}
	expectedDownloadRoot, err := filepath.EvalSymlinks(downloadRoot)
	if err != nil {
		t.Fatalf("resolve download root: %v", err)
	}
	expectedDownloadParent := filepath.Dir(expectedDownloadRoot)
	existingPath := filepath.Join(downloadRoot, "resource", "existing.mp4")
	if err := os.WriteFile(existingPath, []byte("existing-media"), 0o600); err != nil {
		t.Fatalf("write existing media: %v", err)
	}
	size := int64(len("existing-media"))
	existingFile, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-storage-existing", LibraryID: bundle.ID, Kind: "video", Name: "existing.mp4",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: existingPath},
		Origin:    library.FileOrigin{Kind: "download", OperationID: "operation-storage-existing"},
		Media:     &library.MediaInfo{Format: "mp4", SizeBytes: &size},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new existing file: %v", err)
	}
	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	if err := files.Save(ctx, existingFile); err != nil {
		t.Fatalf("save existing file: %v", err)
	}
	item, err := library.NewItem(library.ItemParams{
		ID: "item-storage-existing", CatalogID: catalog.ID, Category: "video",
		Status: "active", Title: "Existing", Revision: 1,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new item: %v", err)
	}
	items := libraryrepo.NewSQLiteCatalogItemRepository(db.Bun)
	if err := items.Save(ctx, item); err != nil {
		t.Fatalf("save item: %v", err)
	}
	asset, err := library.NewItemAsset(library.ItemAssetParams{
		ID: "asset-storage-existing", ItemID: item.ID, FileID: existingFile.ID,
		Role: "original", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new item asset: %v", err)
	}
	assets := libraryrepo.NewSQLiteItemAssetRepository(db.Bun)
	if err := assets.Save(ctx, asset); err != nil {
		t.Fatalf("save item asset: %v", err)
	}

	roots := libraryrepo.NewSQLiteStorageRootRepository(db.Bun)
	legacyRoot, err := library.NewStorageRoot(library.StorageRootParams{
		ID: "root-storage-legacy-parent", CatalogID: catalog.ID, Name: "XiaDown Downloads",
		Path: downloadParent, Mode: "managed", IsDefault: true, Status: "online",
		LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new legacy parent root: %v", err)
	}
	if err := roots.SaveAsDefault(ctx, legacyRoot); err != nil {
		t.Fatalf("save legacy parent root: %v", err)
	}
	strayPath := filepath.Join(downloadParent, "yt-dlp", "stray.mp3")
	if err := os.MkdirAll(filepath.Dir(strayPath), 0o755); err != nil {
		t.Fatalf("create stray parent directory: %v", err)
	}
	if err := os.WriteFile(strayPath, []byte("stray"), 0o600); err != nil {
		t.Fatalf("write stray parent file: %v", err)
	}
	strayFile, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-storage-stray-parent", LibraryID: bundle.ID, Kind: "audio", Name: "stray.mp3",
		Storage: library.FileStorage{
			Mode: "local_path", LocalPath: strayPath,
			RootID: legacyRoot.ID, RelativePath: "yt-dlp/stray.mp3",
		},
		Origin:    library.FileOrigin{Kind: "download", OperationID: "operation-storage-stray"},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new stray parent file: %v", err)
	}
	if err := files.Save(ctx, strayFile); err != nil {
		t.Fatalf("save stray parent file: %v", err)
	}
	service := application.NewCatalogService(
		catalogs, items, assets, files, roots,
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun), nil,
	)
	defaultRoot, err := service.EnsureDefaultDownloadStorageRoot(ctx, downloadParent)
	if err != nil {
		t.Fatalf("ensure default download root: %v", err)
	}
	if defaultRoot.ID != legacyRoot.ID || defaultRoot.Path != expectedDownloadRoot ||
		defaultRoot.LocationPath != expectedDownloadParent ||
		!defaultRoot.IsDefault || defaultRoot.Mode != "managed" ||
		defaultRoot.FileCount != 1 || defaultRoot.AssetCount != 1 ||
		defaultRoot.VideoCount != 1 || defaultRoot.AudioCount != 0 ||
		defaultRoot.SizeBytes != size || defaultRoot.TotalBytes <= 0 {
		t.Fatalf("unexpected default root statistics: %#v", defaultRoot)
	}
	storedRoots, err := roots.ListByCatalogID(ctx, catalog.ID)
	if err != nil || len(storedRoots) != 1 || storedRoots[0].Path != expectedDownloadRoot {
		t.Fatalf("legacy parent root was not repaired in place: roots=%#v err=%v", storedRoots, err)
	}
	realVolumeID := storedRoots[0].VolumeID
	if strings.TrimSpace(realVolumeID) == "" {
		t.Fatal("default root did not persist its storage volume identity")
	}
	guardedRoot := storedRoots[0]
	guardedRoot.VolumeID = "different-persisted-volume"
	if err := roots.SaveAsDefault(ctx, guardedRoot); err != nil {
		t.Fatalf("seed changed storage volume identity: %v", err)
	}
	if _, err := service.EnsureDefaultDownloadStorageRoot(
		ctx,
		downloadParent,
	); err == nil || !strings.Contains(err.Error(), "volume identity changed") {
		t.Fatalf("expected changed storage volume to be rejected, got %v", err)
	}
	preservedRoot, err := roots.Get(ctx, guardedRoot.ID)
	if err != nil ||
		preservedRoot.VolumeID != guardedRoot.VolumeID ||
		preservedRoot.Status != library.StorageRootStatusError ||
		preservedRoot.LastError != "storage volume identity changed" {
		t.Fatalf(
			"changed storage volume overwrote the persisted binding: root=%#v err=%v",
			preservedRoot,
			err,
		)
	}
	preservedRoot.VolumeID = realVolumeID
	if err := roots.SaveAsDefault(ctx, preservedRoot); err != nil {
		t.Fatalf("restore expected storage volume identity: %v", err)
	}
	recoveredRoot, err := service.EnsureDefaultDownloadStorageRoot(ctx, downloadParent)
	if err != nil ||
		recoveredRoot.VolumeID != realVolumeID ||
		recoveredRoot.Status != string(library.StorageRootStatusOnline) {
		t.Fatalf("recover default storage root identity: root=%#v err=%v", recoveredRoot, err)
	}
	loaded, err := files.Get(ctx, existingFile.ID)
	if err != nil {
		t.Fatalf("load backfilled file: %v", err)
	}
	if loaded.Storage.LocalPath != existingPath ||
		loaded.Storage.RootID != defaultRoot.ID ||
		loaded.Storage.RelativePath != "resource/existing.mp4" {
		t.Fatalf("existing file was not additively backfilled: %#v", loaded.Storage)
	}
	strayLoaded, err := files.Get(ctx, strayFile.ID)
	if err != nil || strayLoaded.Storage.LocalPath != strayPath ||
		strayLoaded.Storage.RootID != "" || strayLoaded.Storage.RelativePath != "" {
		t.Fatalf("file outside repaired managed root was not safely detached: %#v err=%v", strayLoaded.Storage, err)
	}

	secondPath := filepath.Join(t.TempDir(), "second-downloads")
	if err := os.MkdirAll(secondPath, 0o755); err != nil {
		t.Fatalf("create second root: %v", err)
	}
	second, err := service.SaveCatalogStorageRoot(ctx, dto.SaveCatalogStorageRootRequest{
		Name: "Second Downloads", Path: secondPath, Mode: "managed",
	})
	if err != nil {
		t.Fatalf("save second managed root: %v", err)
	}
	var syncedPath string
	service.SetDefaultStorageRootPathUpdater(func(_ context.Context, path string) error {
		syncedPath = path
		return nil
	})
	second, err = service.SetDefaultCatalogStorageRoot(ctx, dto.CatalogStorageRootIDRequest{ID: second.ID})
	if err != nil {
		t.Fatalf("set second default root: %v", err)
	}
	expectedSecondParent, err := filepath.EvalSymlinks(secondPath)
	if err != nil {
		t.Fatalf("resolve second root parent: %v", err)
	}
	expectedSecondPath := filepath.Join(expectedSecondParent, "xiadown")
	resolvedDefault, err := service.DefaultStorageRootPath(ctx)
	if err != nil || second.Path != expectedSecondPath ||
		second.LocationPath != expectedSecondParent ||
		resolvedDefault != expectedSecondPath || syncedPath != expectedSecondPath || !second.IsDefault {
		t.Fatalf("default root did not synchronize: root=%#v resolved=%q synced=%q err=%v", second, resolvedDefault, syncedPath, err)
	}

	service.SetDefaultStorageRootPathUpdater(func(context.Context, string) error {
		return errors.New("settings unavailable")
	})
	if _, err := service.SetDefaultCatalogStorageRoot(
		ctx,
		dto.CatalogStorageRootIDRequest{ID: defaultRoot.ID},
	); err == nil {
		t.Fatal("expected failed settings synchronization")
	}
	resolvedDefault, err = service.DefaultStorageRootPath(ctx)
	if err != nil || resolvedDefault != second.Path {
		t.Fatalf("failed default switch was not rolled back: path=%q err=%v", resolvedDefault, err)
	}

	service.SetDefaultStorageRootPathUpdater(func(_ context.Context, path string) error {
		syncedPath = path
		return nil
	})
	relocatedPath := filepath.Join(t.TempDir(), "relocated-downloads")
	if err := os.MkdirAll(relocatedPath, 0o755); err != nil {
		t.Fatalf("create relocated root: %v", err)
	}
	expectedRelocatedParent, err := filepath.EvalSymlinks(relocatedPath)
	if err != nil {
		t.Fatalf("resolve relocated root: %v", err)
	}
	expectedRelocatedPath := filepath.Join(expectedRelocatedParent, "xiadown")
	relocated, err := service.RelocateCatalogStorageRoot(ctx, dto.RelocateCatalogStorageRootRequest{
		ID: second.ID, Path: relocatedPath,
	})
	if err != nil {
		t.Fatalf("relocate default root: %v", err)
	}
	resolvedDefault, err = service.DefaultStorageRootPath(ctx)
	if err != nil || relocated.Path != expectedRelocatedPath ||
		relocated.LocationPath != expectedRelocatedParent ||
		resolvedDefault != expectedRelocatedPath || syncedPath != expectedRelocatedParent {
		t.Fatalf("relocated default root did not synchronize: root=%#v resolved=%q synced=%q err=%v", relocated, resolvedDefault, syncedPath, err)
	}
	if err := service.RemoveCatalogStorageRoot(
		ctx,
		dto.CatalogStorageRootIDRequest{ID: defaultRoot.ID},
	); err == nil {
		t.Fatal("managed root owning existing files was removed")
	}

	referencePath := filepath.Join(t.TempDir(), "referenced")
	if err := os.MkdirAll(referencePath, 0o755); err != nil {
		t.Fatalf("create referenced root: %v", err)
	}
	referenceRoot, err := service.SaveCatalogStorageRoot(ctx, dto.SaveCatalogStorageRootRequest{
		Name: "Referenced", Path: referencePath, Mode: "referenced",
	})
	if err != nil {
		t.Fatalf("save referenced root: %v", err)
	}
	if referenceRoot.LocationPath != referenceRoot.Path {
		t.Fatalf("referenced root location changed: root=%#v", referenceRoot)
	}
	referencedFile, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-storage-referenced", LibraryID: bundle.ID, Kind: "video", Name: "reference.mp4",
		Storage: library.FileStorage{
			Mode: "local_path", LocalPath: filepath.Join(referencePath, "reference.mp4"),
		},
		Origin: library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{
			BatchID: "batch-storage-reference", ImportPath: filepath.Join(referencePath, "reference.mp4"),
			ImportedAt: now, KeepSourceFile: true,
		}},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new referenced file: %v", err)
	}
	if err := files.Save(ctx, referencedFile); err != nil {
		t.Fatalf("save referenced file: %v", err)
	}
	if err := service.RemoveCatalogStorageRoot(
		ctx,
		dto.CatalogStorageRootIDRequest{ID: referenceRoot.ID},
	); err != nil {
		t.Fatalf("remove referenced root: %v", err)
	}
	referencedFile, err = files.Get(ctx, referencedFile.ID)
	if err != nil || referencedFile.Storage.RootID != "" ||
		referencedFile.Storage.RelativePath != "" ||
		referencedFile.Storage.LocalPath != filepath.Join(referencePath, "reference.mp4") {
		t.Fatalf("referenced root removal lost compatibility path or retained ownership: %#v err=%v", referencedFile.Storage, err)
	}
}
