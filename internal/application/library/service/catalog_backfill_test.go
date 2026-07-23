package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/persistence"
)

func TestDownloadedFileIsProjectedIntoCatalogBeforeCreateReturns(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 13, 7, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "download-bundle", now)
	path := filepath.Join(t.TempDir(), "download.pdf")
	if err := os.WriteFile(path, []byte("downloaded document"), 0o600); err != nil {
		t.Fatalf("write downloaded fixture: %v", err)
	}

	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	projection := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun), files,
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	projection.now = func() time.Time { return now.Add(time.Minute) }
	service := &LibraryService{files: files, catalogProjection: projection}
	file, err := service.createDownloadedBinaryFile(ctx, library.LibraryOperation{
		ID: "download-operation", LibraryID: "download-bundle",
	}, string(library.FileKindDocument), path, "Downloaded", now)
	if err != nil {
		t.Fatalf("createDownloadedBinaryFile: %v", err)
	}
	if file.ID == "" {
		t.Fatal("downloaded file has no stable ID")
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*)
FROM library_legacy_mappings
WHERE migration_id = 'catalog-foundation-v2'
  AND source_type = 'library_file' AND source_id = ?
`, 1, file.ID)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*)
FROM library_item_assets AS asset
JOIN library_catalog_items AS item ON item.id = asset.item_id
WHERE asset.file_id = ? AND item.status = 'active'
`, 1, file.ID)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*)
FROM library_representations AS representation
JOIN library_item_assets AS asset ON asset.id = representation.asset_id
WHERE asset.file_id = ? AND representation.availability = 'available'
`, 1, file.ID)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_catalog_changes WHERE catalog_id = ?
	`, 3, DefaultLibraryCatalogID())
}

func TestListenLocalMetadataSynchronizesCatalogItemAndAllTagFields(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "music-bundle", now)
	seedCatalogBackfillFile(
		t, ctx, db, "song-file", "music-bundle", "audio", "old-song.m4a",
		filepath.Join(t.TempDir(), "old-song.m4a"), "", now,
	)

	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	projection := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun), files,
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	projection.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := projection.RunLibrary(ctx, "music-bundle"); err != nil {
		t.Fatalf("project music bundle: %v", err)
	}

	items := libraryrepo.NewSQLiteCatalogItemRepository(db.Bun)
	itemID := deterministicCatalogID("item", DefaultLibraryCatalogID(), "song-file")
	item, err := items.Get(ctx, itemID)
	if err != nil {
		t.Fatalf("get projected item: %v", err)
	}
	item.Description = "Keep this user note"
	item.Revision++
	item.UpdatedAt = now.Add(2 * time.Minute)
	if err := items.Save(ctx, item); err != nil {
		t.Fatalf("seed user description: %v", err)
	}

	mutations := libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun)
	catalogService := NewCatalogService(
		libraryrepo.NewSQLiteCatalogRepository(db.Bun),
		items,
		libraryrepo.NewSQLiteItemAssetRepository(db.Bun),
		files,
		libraryrepo.NewSQLiteStorageRootRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		mutations,
		nil,
	)
	catalogService.now = func() time.Time { return now.Add(3 * time.Minute) }
	file, err := files.Get(ctx, "song-file")
	if err != nil {
		t.Fatalf("get music file: %v", err)
	}
	file.DisplayName = "New Song"
	file.Metadata.Title = "New Song"
	file.Metadata.Author = "New Artist"
	if err := catalogService.SyncListenLocalTrackMetadata(ctx, file, dto.UpdateListenLocalTrackMetadataRequest{
		FileID: file.ID, Title: "New Song", Author: "New Artist", Album: "New Album",
		AlbumArtist: "Album Artist", Genre: "Electronic", TrackNumber: 4, DiscNumber: 2, Year: 2026,
	}); err != nil {
		t.Fatalf("sync local metadata to catalog: %v", err)
	}

	updated, err := items.Get(ctx, itemID)
	if err != nil || updated.Title != "New Song" || updated.SortTitle != "New Song" ||
		updated.Description != "Keep this user note" {
		t.Fatalf("unexpected synchronized item: %#v err=%v", updated, err)
	}
	entries, err := mutations.ListMetadataEntriesByItemID(ctx, itemID)
	if err != nil {
		t.Fatalf("list synchronized metadata: %v", err)
	}
	got := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.Namespace != "music" || entry.Source != library.MetadataSourceEmbedded ||
			entry.Provenance != "music.local-metadata-editor" {
			t.Fatalf("unexpected metadata provenance: %#v", entry)
		}
		got[entry.Key] = string(entry.Value)
	}
	want := map[string]string{
		"title": `"New Song"`, "artist": `"New Artist"`, "album": `"New Album"`,
		"album_artist": `"Album Artist"`, "genre": `"Electronic"`,
		"track_number": "4", "disc_number": "2", "year": "2026",
	}
	if len(got) != len(want) {
		t.Fatalf("expected every editable field in Catalog metadata: %#v", got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("metadata %s: got %q want %q", key, got[key], value)
		}
	}
	initialItemRevision := updated.Revision
	initialEntryRevisions := make(map[string]int64, len(entries))
	for _, entry := range entries {
		initialEntryRevisions[entry.Key] = entry.Revision
	}
	if err := catalogService.SyncListenLocalTrackMetadata(ctx, file, dto.UpdateListenLocalTrackMetadataRequest{
		FileID: file.ID, Title: "New Song", Author: "New Artist", Album: "New Album",
		AlbumArtist: "Album Artist", Genre: "Electronic", TrackNumber: 4, DiscNumber: 2, Year: 2026,
	}); err != nil {
		t.Fatalf("repeat identical metadata sync: %v", err)
	}
	idempotentItem, _ := items.Get(ctx, itemID)
	idempotentEntries, _ := mutations.ListMetadataEntriesByItemID(ctx, itemID)
	if idempotentItem.Revision != initialItemRevision {
		t.Fatalf("identical sync changed item revision: got %d want %d", idempotentItem.Revision, initialItemRevision)
	}
	for _, entry := range idempotentEntries {
		if entry.Revision != initialEntryRevisions[entry.Key] {
			t.Fatalf("identical sync changed %s revision: got %d want %d", entry.Key, entry.Revision, initialEntryRevisions[entry.Key])
		}
	}
	catalogService.now = func() time.Time { return now.Add(210 * time.Second) }
	if err := catalogService.SyncListenLocalTrackMetadata(ctx, file, dto.UpdateListenLocalTrackMetadataRequest{
		FileID: file.ID, Title: "New Song", Author: "New Artist", Album: "Revised Album",
		AlbumArtist: "Album Artist", Genre: "Electronic", TrackNumber: 4, DiscNumber: 2, Year: 2026,
	}); err != nil {
		t.Fatalf("second metadata edit: %v", err)
	}
	secondEditEntries, _ := mutations.ListMetadataEntriesByItemID(ctx, itemID)
	for _, entry := range secondEditEntries {
		wantRevision := initialEntryRevisions[entry.Key]
		if entry.Key == "album" {
			wantRevision++
			if string(entry.Value) != `"Revised Album"` {
				t.Fatalf("second album edit was not stored: %#v", entry)
			}
		}
		if entry.Revision != wantRevision {
			t.Fatalf("second edit revision for %s: got %d want %d", entry.Key, entry.Revision, wantRevision)
		}
	}
	var albumEditFinalType, albumEditFinalID string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT entity_type, entity_id
FROM library_catalog_changes
ORDER BY sequence DESC
LIMIT 1
`).Scan(&albumEditFinalType, &albumEditFinalID); err != nil {
		t.Fatal(err)
	}
	if albumEditFinalType != string(library.CatalogEntityItem) || albumEditFinalID != itemID {
		t.Fatalf("metadata-only batch did not end with Item invalidation: type=%q id=%q", albumEditFinalType, albumEditFinalID)
	}

	userEntry, err := library.NewMetadataEntry(library.MetadataEntryParams{
		ID: "user-album", CatalogID: DefaultLibraryCatalogID(), ItemID: itemID,
		Namespace: "music", Key: "album", ValueType: "string", ValueJSON: `"Personal Album"`,
		Source: "user", Provenance: "library-metadata-tab", Revision: 1,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mutations.SaveMetadataEntryMutation(ctx, userEntry, 0, "desktop-user"); err != nil {
		t.Fatalf("seed user metadata entry: %v", err)
	}

	catalogService.now = func() time.Time { return now.Add(4 * time.Minute) }
	file.Metadata.Author = ""
	if err := catalogService.SyncListenLocalTrackMetadata(ctx, file, dto.UpdateListenLocalTrackMetadataRequest{
		FileID: file.ID, Title: "New Song",
	}); err != nil {
		t.Fatalf("clear local metadata in catalog: %v", err)
	}
	entries, err = mutations.ListMetadataEntriesByItemID(ctx, itemID)
	if err != nil || len(entries) != 2 {
		t.Fatalf("cleared fields left stale Catalog metadata: %#v err=%v", entries, err)
	}
	remaining := map[string]library.MetadataSource{}
	for _, entry := range entries {
		remaining[entry.ID] = entry.Source
	}
	if remaining["user-album"] != library.MetadataSourceUser {
		t.Fatalf("clearing editor metadata removed the user's same-key entry: %#v", entries)
	}
	if _, exists := remaining[deterministicCatalogID(
		"metadata-entry", itemID, "music", "title", "music.local-metadata-editor",
	)]; !exists {
		t.Fatalf("required title metadata was removed: %#v", entries)
	}
}

func TestListenLocalMetadataRefreshesLegacySnapshotInOneRecoverableBatch(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 19, 11, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "legacy-music-bundle", now)
	seedCatalogBackfillFile(
		t, ctx, db, "legacy-song-file", "legacy-music-bundle", "audio", "legacy-song.m4a",
		filepath.Join(t.TempDir(), "legacy-song.m4a"), "", now,
	)

	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	file, err := files.Get(ctx, "legacy-song-file")
	if err != nil {
		t.Fatal(err)
	}
	file.DisplayName = "Old title"
	file.Metadata = library.FileMetadata{Title: "Old title", Author: "Old artist", Extractor: "keep-extractor"}
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("seed historical file metadata: %v", err)
	}
	projection := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun), files,
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	projection.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := projection.RunLibrary(ctx, file.LibraryID); err != nil {
		t.Fatalf("project historical music file: %v", err)
	}

	items := libraryrepo.NewSQLiteCatalogItemRepository(db.Bun)
	mutations := libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun)
	itemID := deterministicCatalogID("item", DefaultLibraryCatalogID(), file.ID)
	beforeItem, err := items.Get(ctx, itemID)
	if err != nil || beforeItem.Title != "Old title" {
		t.Fatalf("unexpected historical item: %#v err=%v", beforeItem, err)
	}
	beforeEntries, err := mutations.ListMetadataEntriesByItemID(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	var beforeLegacy library.MetadataEntry
	for _, entry := range beforeEntries {
		if entry.Provenance == "legacy.library_files.metadata_json" {
			beforeLegacy = entry
			break
		}
	}
	if beforeLegacy.ID == "" {
		t.Fatalf("projection did not create the historical file metadata entry: %#v", beforeEntries)
	}
	var oldSnapshot library.FileMetadata
	if err := json.Unmarshal(beforeLegacy.Value, &oldSnapshot); err != nil ||
		oldSnapshot.Title != "Old title" || oldSnapshot.Author != "Old artist" {
		t.Fatalf("unexpected historical metadata snapshot: %#v err=%v", oldSnapshot, err)
	}

	catalogService := NewCatalogService(
		libraryrepo.NewSQLiteCatalogRepository(db.Bun),
		items,
		libraryrepo.NewSQLiteItemAssetRepository(db.Bun),
		files,
		libraryrepo.NewSQLiteStorageRootRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		mutations,
		nil,
	)
	catalogService.now = func() time.Time { return now.Add(2 * time.Minute) }
	file.DisplayName = "New title"
	file.Metadata.Title = "New title"
	file.Metadata.Author = "New artist"
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("save current LibraryFile before Catalog synchronization: %v", err)
	}
	request := dto.UpdateListenLocalTrackMetadataRequest{
		FileID: file.ID, Title: "New title", Author: "New artist", Album: "New album",
		AlbumArtist: "New album artist", Genre: "Jazz", TrackNumber: 7, DiscNumber: 1, Year: 2026,
	}
	var beforeChangeCount int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_catalog_changes`).Scan(&beforeChangeCount); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
CREATE TRIGGER fail_local_music_metadata_batch
BEFORE INSERT ON library_metadata_entries
WHEN NEW.provenance = 'music.local-metadata-editor' AND NEW.key = 'genre'
BEGIN
  SELECT RAISE(ABORT, 'forced local music metadata batch failure');
END;
`); err != nil {
		t.Fatalf("create metadata failure trigger: %v", err)
	}
	if err := catalogService.SyncListenLocalTrackMetadata(ctx, file, request); err == nil {
		t.Fatal("expected injected aggregate metadata failure")
	}

	afterFailureItem, err := items.Get(ctx, itemID)
	if err != nil || afterFailureItem.Title != beforeItem.Title ||
		afterFailureItem.Revision != beforeItem.Revision {
		t.Fatalf("Item escaped failed metadata transaction: before=%#v after=%#v err=%v", beforeItem, afterFailureItem, err)
	}
	afterFailureEntries, err := mutations.ListMetadataEntriesByItemID(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range afterFailureEntries {
		if entry.Provenance == "music.local-metadata-editor" {
			t.Fatalf("first-class metadata escaped failed transaction: %#v", entry)
		}
		if entry.ID == beforeLegacy.ID &&
			(entry.Revision != beforeLegacy.Revision || string(entry.Value) != string(beforeLegacy.Value)) {
			t.Fatalf("legacy snapshot escaped failed transaction: before=%#v after=%#v", beforeLegacy, entry)
		}
	}
	var afterFailureChangeCount int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_catalog_changes`).Scan(&afterFailureChangeCount); err != nil {
		t.Fatal(err)
	}
	if afterFailureChangeCount != beforeChangeCount {
		t.Fatalf("change feed escaped failed transaction: before=%d after=%d", beforeChangeCount, afterFailureChangeCount)
	}
	if _, err := db.SQL.ExecContext(ctx, `DROP TRIGGER fail_local_music_metadata_batch`); err != nil {
		t.Fatal(err)
	}
	if err := catalogService.SyncListenLocalTrackMetadata(ctx, file, request); err != nil {
		t.Fatalf("retry identical metadata request: %v", err)
	}

	afterRetryItem, err := items.Get(ctx, itemID)
	if err != nil || afterRetryItem.Title != "New title" ||
		afterRetryItem.Revision != beforeItem.Revision+1 {
		t.Fatalf("retry did not commit Item: %#v err=%v", afterRetryItem, err)
	}
	afterRetryEntries, err := mutations.ListMetadataEntriesByItemID(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	musicEntryCount := 0
	var afterLegacy library.MetadataEntry
	for _, entry := range afterRetryEntries {
		if entry.Provenance == "music.local-metadata-editor" {
			musicEntryCount++
		}
		if entry.ID == beforeLegacy.ID {
			afterLegacy = entry
		}
	}
	if musicEntryCount != 8 {
		t.Fatalf("retry did not commit all first-class fields: count=%d entries=%#v", musicEntryCount, afterRetryEntries)
	}
	var refreshedSnapshot library.FileMetadata
	if afterLegacy.ID == "" {
		t.Fatal("retry removed the historical metadata snapshot")
	}
	if err := json.Unmarshal(afterLegacy.Value, &refreshedSnapshot); err != nil ||
		refreshedSnapshot.Title != "New title" || refreshedSnapshot.Author != "New artist" ||
		refreshedSnapshot.Extractor != "keep-extractor" || afterLegacy.Revision != beforeLegacy.Revision+1 {
		t.Fatalf("historical metadata snapshot stayed stale: %#v entry=%#v err=%v", refreshedSnapshot, afterLegacy, err)
	}
	var finalEntityType, finalEntityID string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT entity_type, entity_id
FROM library_catalog_changes
ORDER BY sequence DESC
LIMIT 1
`).Scan(&finalEntityType, &finalEntityID); err != nil {
		t.Fatal(err)
	}
	if finalEntityType != string(library.CatalogEntityItem) || finalEntityID != itemID {
		t.Fatalf("aggregate invalidation was not the final change: type=%q id=%q", finalEntityType, finalEntityID)
	}
}

func TestListenLocalMetadataResolvesTranscodeRepresentationOwner(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "transcode-music-bundle", now)
	seedCatalogBackfillFile(
		t, ctx, db, "source-video", "transcode-music-bundle", "video", "source.mp4",
		filepath.Join(t.TempDir(), "source.mp4"), "", now,
	)
	seedCatalogBackfillFile(
		t, ctx, db, "audio-transcode", "transcode-music-bundle", "transcode", "song.m4a",
		filepath.Join(t.TempDir(), "song.m4a"), "source-video", now.Add(time.Second),
	)
	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	projection := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun), files,
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	if _, err := projection.RunLibrary(ctx, "transcode-music-bundle"); err != nil {
		t.Fatal(err)
	}
	items := libraryrepo.NewSQLiteCatalogItemRepository(db.Bun)
	mutations := libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun)
	service := NewCatalogService(
		libraryrepo.NewSQLiteCatalogRepository(db.Bun), items,
		libraryrepo.NewSQLiteItemAssetRepository(db.Bun), files,
		libraryrepo.NewSQLiteStorageRootRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun), mutations, nil,
	)
	transcode, err := files.Get(ctx, "audio-transcode")
	if err != nil {
		t.Fatal(err)
	}
	transcode.Metadata.Title = "Representation song"
	transcode.Metadata.Author = "Representation artist"
	if err := service.SyncListenLocalTrackMetadata(ctx, transcode, dto.UpdateListenLocalTrackMetadataRequest{
		FileID: transcode.ID, Title: transcode.Metadata.Title, Author: transcode.Metadata.Author,
	}); err != nil {
		t.Fatalf("sync transcode metadata through owning Item: %v", err)
	}
	rootItemID := deterministicCatalogID("item", DefaultLibraryCatalogID(), "source-video")
	rootItem, err := items.Get(ctx, rootItemID)
	if err != nil || rootItem.Title != "Representation song" {
		t.Fatalf("owning Item did not receive representation metadata: %#v err=%v", rootItem, err)
	}
	if _, err := items.Get(ctx, deterministicCatalogID("item", DefaultLibraryCatalogID(), transcode.ID)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("transcode unexpectedly gained a standalone Item: %v", err)
	}
}

func TestLegacyCatalogBackfillProjectsAllNonEmptyBundlesIntoOneCatalog(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-0-empty", now)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-a-video", now.Add(time.Minute))
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-b-book", now.Add(2*time.Minute))
	videoPath := "/legacy/media/movie.mp4"
	subtitlePath := "/legacy/media/movie.srt"
	bookPath := "/legacy/books/guide.epub"
	seedCatalogBackfillFile(t, ctx, db, "file-video", "bundle-a-video", "video", "movie.mp4", videoPath, "", now)
	seedCatalogBackfillFile(t, ctx, db, "file-subtitle", "bundle-a-video", "subtitle", "movie.srt", subtitlePath, "file-video", now.Add(time.Second))
	seedCatalogBackfillFile(t, ctx, db, "file-book", "bundle-b-book", "document", "guide.epub", bookPath, "", now.Add(2*time.Second))

	service := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun),
		libraryrepo.NewSQLiteFileRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	service.now = func() time.Time { return now.Add(10 * time.Minute) }
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.CatalogID != DefaultLibraryCatalogID() || result.BundlesProcessed != 2 ||
		result.FilesProcessed != 3 || !result.Completed {
		t.Fatalf("unexpected result: %#v", result)
	}

	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_catalogs WHERE is_default = TRUE", 1)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_catalog_items", 2)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_item_assets", 3)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_legacy_mappings
WHERE migration_id = 'catalog-foundation-v2' AND source_type = 'library_file'
`, 3)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_legacy_mappings
WHERE migration_id = 'catalog-foundation-v2' AND source_type = 'legacy_library'
`, 2)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*)
FROM library_files AS f
WHERE (SELECT COUNT(*) FROM library_item_assets AS a WHERE a.file_id = f.id) != 1
   OR (SELECT COUNT(*) FROM library_legacy_mappings AS m
       WHERE m.migration_id = 'catalog-foundation-v2'
         AND m.source_type = 'library_file' AND m.source_id = f.id) != 1
`, 0)

	for fileID, expectedPath := range map[string]string{
		"file-video": videoPath, "file-subtitle": subtitlePath, "file-book": bookPath,
	} {
		var actualPath string
		if err := db.SQL.QueryRowContext(ctx, "SELECT storage_local_path FROM library_files WHERE id = ?", fileID).Scan(&actualPath); err != nil {
			t.Fatalf("read legacy path for %s: %v", fileID, err)
		}
		if actualPath != expectedPath {
			t.Fatalf("legacy path for %s changed from %q to %q", fileID, expectedPath, actualPath)
		}
	}

	var status, cursor string
	var processed int64
	if err := db.SQL.QueryRowContext(ctx, `
SELECT status, cursor, processed
FROM library_migration_checkpoints
WHERE migration_id = 'catalog-foundation-v2' AND phase = 'backfill'
`).Scan(&status, &cursor, &processed); err != nil {
		t.Fatalf("read checkpoint: %v", err)
	}
	if status != "completed" || cursor != "bundle-b-book" || processed != 3 {
		t.Fatalf("unexpected checkpoint: status=%q cursor=%q processed=%d", status, cursor, processed)
	}

	retry, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("retry Run: %v", err)
	}
	if retry.BundlesProcessed != 0 || retry.FilesProcessed != 3 || !retry.Completed {
		t.Fatalf("retry was not a checkpoint no-op: %#v", retry)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_item_assets", 3)
}

func TestLegacyCatalogBackfillRollsBackBundleAndResumesFromFailedCheckpoint(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-failure", now)
	seedCatalogBackfillFile(t, ctx, db, "file-first", "bundle-failure", "video", "first.mp4", "/legacy/first.mp4", "", now)
	seedCatalogBackfillFile(t, ctx, db, "file-fail", "bundle-failure", "audio", "fail.mp3", "/legacy/fail.mp3", "", now.Add(time.Second))
	if _, err := db.SQL.ExecContext(ctx, `
CREATE TRIGGER fail_catalog_file_mapping
BEFORE INSERT ON library_legacy_mappings
WHEN NEW.source_type = 'library_file' AND NEW.source_id = 'file-fail'
BEGIN
  SELECT RAISE(FAIL, 'injected catalog mapping failure');
END;
`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	service := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun),
		libraryrepo.NewSQLiteFileRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	if _, err := service.Run(ctx); err == nil {
		t.Fatal("Run unexpectedly succeeded with injected mapping failure")
	}
	// Catalog creation is intentionally separate so failure can be recorded;
	// the entire legacy bundle itself must have rolled back.
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_catalogs", 1)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_catalog_items", 0)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_item_assets", 0)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_legacy_mappings", 0)
	var status, cursor string
	var processed int64
	if err := db.SQL.QueryRowContext(ctx, `
SELECT status, cursor, processed FROM library_migration_checkpoints
WHERE migration_id = 'catalog-foundation-v2' AND phase = 'backfill'
`).Scan(&status, &cursor, &processed); err != nil {
		t.Fatalf("read failed checkpoint: %v", err)
	}
	if status != "failed" || cursor != "" || processed != 0 {
		t.Fatalf("unexpected failed checkpoint: status=%q cursor=%q processed=%d", status, cursor, processed)
	}

	if _, err := db.SQL.ExecContext(ctx, "DROP TRIGGER fail_catalog_file_mapping"); err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}
	resumed, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("resumed Run: %v", err)
	}
	if !resumed.Completed || resumed.BundlesProcessed != 1 || resumed.FilesProcessed != 2 {
		t.Fatalf("unexpected resumed result: %#v", resumed)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_catalog_items", 2)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_item_assets", 2)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_legacy_mappings WHERE source_type = 'library_file'
`, 2)
}

func TestLegacyCatalogBackfillDoesNotCreateCatalogForEmptyBundles(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "empty-a", now)
	seedCatalogBackfillLibrary(t, ctx, db, "empty-b", now.Add(time.Minute))

	service := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun),
		libraryrepo.NewSQLiteFileRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Completed || result.CatalogID != "" || result.FilesProcessed != 0 {
		t.Fatalf("unexpected empty result: %#v", result)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_catalogs", 0)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_migration_checkpoints", 0)
}

func TestLegacyCatalogBackfillContinuouslyProjectsFilesAddedAfterCompletion(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 13, 11, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-a", now)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-z-was-empty", now)
	seedCatalogBackfillFile(t, ctx, db, "file-a", "bundle-a", "video", "a.mp4", "/legacy/a.mp4", "", now)

	service := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun),
		libraryrepo.NewSQLiteFileRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	service.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := service.Run(ctx); err != nil {
		t.Fatalf("initial Run: %v", err)
	}

	addedAt := now.Add(2 * time.Minute)
	seedCatalogBackfillFile(t, ctx, db, "file-added-to-old", "bundle-a", "audio", "new.mp3", "/legacy/new.mp3", "", addedAt)
	seedCatalogBackfillFile(t, ctx, db, "file-added-to-empty", "bundle-z-was-empty", "document", "new.epub", "/legacy/new.epub", "", addedAt)
	service.now = func() time.Time { return now.Add(3 * time.Minute) }
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("continuous Run: %v", err)
	}
	if !result.Completed || result.FilesProcessed != 3 || result.BundlesProcessed != 2 {
		t.Fatalf("unexpected continuous result: %#v", result)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_item_assets", 3)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_legacy_mappings WHERE source_type = 'library_file'
`, 3)
}

func TestLegacyCatalogRuntimeProjectionScopesBundleAndKeepsStartupReconciliationPending(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 13, 11, 30, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-runtime-a", now)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-runtime-b", now)
	seedCatalogBackfillFile(t, ctx, db, "file-runtime-a", "bundle-runtime-a", "video", "a.mp4", "/legacy/a.mp4", "", now)
	seedCatalogBackfillFile(t, ctx, db, "file-runtime-b", "bundle-runtime-b", "audio", "b.mp3", "/legacy/b.mp3", "", now)

	service := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun),
		libraryrepo.NewSQLiteFileRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	service.now = func() time.Time { return now.Add(time.Minute) }
	result, err := service.RunLibrary(ctx, "bundle-runtime-a")
	if err != nil || !result.Completed || result.BundlesProcessed != 1 || result.FilesProcessed != 1 {
		t.Fatalf("RunLibrary: result=%#v err=%v", result, err)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_legacy_mappings
WHERE source_type = 'library_file' AND source_id = 'file-runtime-a'
`, 1)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_legacy_mappings
WHERE source_type = 'library_file' AND source_id = 'file-runtime-b'
`, 0)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_migration_checkpoints", 0)

	service.now = func() time.Time { return now.Add(2 * time.Minute) }
	startup, err := service.Run(ctx)
	if err != nil || !startup.Completed || startup.FilesProcessed != 2 {
		t.Fatalf("startup reconciliation: result=%#v err=%v", startup, err)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_item_assets", 2)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_migration_checkpoints", 1)
}

func TestLegacyCatalogBackfillPreservesUserCatalogDataWhileAddingNewFiles(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-user-managed", now)
	seedCatalogBackfillFile(
		t, ctx, db, "file-existing", "bundle-user-managed", "video",
		"original.mp4", "/legacy/original.mp4", "", now,
	)

	service := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun),
		libraryrepo.NewSQLiteFileRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	service.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := service.Run(ctx); err != nil {
		t.Fatalf("initial Run: %v", err)
	}

	var itemID, assetID, originalFingerprint, originalMigratedAt string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT asset.item_id, mapping.target_id, mapping.source_fingerprint, CAST(mapping.migrated_at AS TEXT)
FROM library_legacy_mappings AS mapping
JOIN library_item_assets AS asset ON asset.id = mapping.target_id
WHERE mapping.migration_id = 'catalog-foundation-v2'
  AND mapping.source_type = 'library_file'
  AND mapping.source_id = 'file-existing'
`).Scan(&itemID, &assetID, &originalFingerprint, &originalMigratedAt); err != nil {
		t.Fatalf("read initial projection: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
UPDATE library_catalog_items
SET category = 'book', status = 'needs_review', title = 'User title',
    sort_title = 'User sort title', description = 'User description',
    revision = 9, updated_at = ?
WHERE id = ?
`, now.Add(2*time.Minute), itemID); err != nil {
		t.Fatalf("edit projected item: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
UPDATE library_item_assets
SET role = 'artwork', label = 'User asset label', position = 7, updated_at = ?
WHERE id = ?
`, now.Add(2*time.Minute), assetID); err != nil {
		t.Fatalf("edit projected asset: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
INSERT INTO library_representations (
  id, catalog_id, item_id, asset_id, kind, purpose,
  availability, revision, created_at, updated_at
) VALUES ('user-preview-representation', ?, ?, ?, 'preview', 'preview',
          'available', 4, ?, ?)
`, DefaultLibraryCatalogID(), itemID, assetID, now.Add(2*time.Minute), now.Add(2*time.Minute)); err != nil {
		t.Fatalf("insert user representation: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
INSERT INTO library_metadata_entries (
  id, catalog_id, item_id, representation_id, namespace, key,
  value_type, value_json, source, provenance, locked, revision,
  created_at, updated_at
) VALUES ('user-preview-metadata', ?, ?, 'user-preview-representation',
          'user', 'annotation', 'string', '"keep me"', 'user',
          'manual edit', TRUE, 5, ?, ?)
`, DefaultLibraryCatalogID(), itemID, now.Add(2*time.Minute), now.Add(2*time.Minute)); err != nil {
		t.Fatalf("insert user metadata: %v", err)
	}

	seedCatalogBackfillFile(
		t, ctx, db, "file-new", "bundle-user-managed", "video",
		"new.mp4", "/legacy/new.mp4", "", now.Add(3*time.Minute),
	)
	service.now = func() time.Time { return now.Add(4 * time.Minute) }
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("incremental Run: %v", err)
	}
	if !result.Completed || result.FilesProcessed != 2 || result.BundlesProcessed != 1 {
		t.Fatalf("unexpected incremental result: %#v", result)
	}

	var category, status, title, sortTitle, description string
	var revision int
	if err := db.SQL.QueryRowContext(ctx, `
SELECT category, status, title, sort_title, description, revision
FROM library_catalog_items
WHERE id = ?
`, itemID).Scan(&category, &status, &title, &sortTitle, &description, &revision); err != nil {
		t.Fatalf("read preserved item: %v", err)
	}
	if category != "book" || status != "needs_review" || title != "User title" ||
		sortTitle != "User sort title" || description != "User description" || revision != 9 {
		t.Fatalf("user item data was changed: category=%q status=%q title=%q sort=%q description=%q revision=%d",
			category, status, title, sortTitle, description, revision)
	}
	var role, label string
	var position int
	if err := db.SQL.QueryRowContext(ctx, `
SELECT role, label, position FROM library_item_assets WHERE id = ?
`, assetID).Scan(&role, &label, &position); err != nil {
		t.Fatalf("read preserved asset: %v", err)
	}
	if role != "artwork" || label != "User asset label" || position != 7 {
		t.Fatalf("user asset data was changed: role=%q label=%q position=%d", role, label, position)
	}
	var currentTarget, currentFingerprint, currentMigratedAt string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT target_id, source_fingerprint, CAST(migrated_at AS TEXT)
FROM library_legacy_mappings
WHERE migration_id = 'catalog-foundation-v2'
  AND source_type = 'library_file' AND source_id = 'file-existing'
`).Scan(&currentTarget, &currentFingerprint, &currentMigratedAt); err != nil {
		t.Fatalf("read preserved mapping: %v", err)
	}
	if currentTarget != assetID || currentFingerprint != originalFingerprint || currentMigratedAt != originalMigratedAt {
		t.Fatalf("immutable mapping changed: target=%q fingerprint=%q migrated_at=%q",
			currentTarget, currentFingerprint, currentMigratedAt)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_representations
WHERE id = 'user-preview-representation' AND revision = 4
`, 1)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_metadata_entries
WHERE id = 'user-preview-metadata' AND value_json = '"keep me"'
  AND locked = TRUE AND revision = 5
`, 1)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_legacy_mappings
WHERE migration_id = 'catalog-foundation-v2'
  AND source_type = 'library_file' AND source_id = 'file-new'
`, 1)
	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_item_assets", 2)
}

func TestLegacyCatalogBackfillReconcilesMappedFileAvailabilityAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 13, 13, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-runtime", now)
	path := filepath.Join(t.TempDir(), "runtime.mp4")
	if err := os.WriteFile(path, []byte("runtime media"), 0o600); err != nil {
		t.Fatalf("write runtime fixture: %v", err)
	}
	seedCatalogBackfillFile(t, ctx, db, "file-runtime", "bundle-runtime", "video", "runtime.mp4", path, "", now)

	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	projection := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun), files,
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	projection.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := projection.Run(ctx); err != nil {
		t.Fatalf("initial Run: %v", err)
	}
	var itemID, assetID string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT asset.item_id, asset.id
FROM library_legacy_mappings AS mapping
JOIN library_item_assets AS asset ON asset.id = mapping.target_id
WHERE mapping.migration_id = 'catalog-foundation-v2'
  AND mapping.source_type = 'library_file' AND mapping.source_id = 'file-runtime'
`).Scan(&itemID, &assetID); err != nil {
		t.Fatalf("read runtime mapping: %v", err)
	}

	assertRuntimeCatalogState := func(wantItem, wantRepresentation string, wantAvailable bool) {
		t.Helper()
		var itemStatus, representationAvailability string
		if err := db.SQL.QueryRowContext(ctx, `
SELECT item.status, representation.availability
FROM library_catalog_items AS item
JOIN library_representations AS representation ON representation.item_id = item.id
WHERE item.id = ? AND representation.id = ?
`, itemID, assetID).Scan(&itemStatus, &representationAvailability); err != nil {
			t.Fatalf("read reconciled runtime state: %v", err)
		}
		if itemStatus != wantItem || representationAvailability != wantRepresentation {
			t.Fatalf("runtime state item=%q representation=%q, want %q/%q",
				itemStatus, representationAvailability, wantItem, wantRepresentation)
		}
		catalogService := NewCatalogService(
			libraryrepo.NewSQLiteCatalogRepository(db.Bun),
			libraryrepo.NewSQLiteCatalogItemRepository(db.Bun),
			libraryrepo.NewSQLiteItemAssetRepository(db.Bun),
			files,
			libraryrepo.NewSQLiteStorageRootRepository(db.Bun),
			libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
			libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
			libraryrepo.NewSQLiteUserStateRepository(db.Bun),
			libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun), nil,
		)
		detail, err := catalogService.GetCatalogItem(ctx, dto.GetCatalogItemRequest{ID: itemID})
		if err != nil {
			t.Fatalf("GetCatalogItem: %v", err)
		}
		if len(detail.Assets) != 1 || detail.Assets[0].FileAvailable != wantAvailable {
			t.Fatalf("detail availability=%#v, want %v", detail.Assets, wantAvailable)
		}
	}

	file, err := files.Get(ctx, "file-runtime")
	if err != nil {
		t.Fatalf("get runtime file: %v", err)
	}
	file.State.LastError = missingLocalFileError
	file.UpdatedAt = now.Add(2 * time.Minute)
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("mark runtime file missing: %v", err)
	}
	projection.now = func() time.Time { return now.Add(3 * time.Minute) }
	if _, err := projection.Run(ctx); err != nil {
		t.Fatalf("missing Run: %v", err)
	}
	assertRuntimeCatalogState("missing", "missing", false)

	file.State.LastError = ""
	file.State.Status = "active"
	file.UpdatedAt = now.Add(4 * time.Minute)
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("restore missing runtime file: %v", err)
	}
	projection.now = func() time.Time { return now.Add(5 * time.Minute) }
	if _, err := projection.Run(ctx); err != nil {
		t.Fatalf("missing recovery Run: %v", err)
	}
	assertRuntimeCatalogState("active", "available", true)

	file.State.Deleted = true
	file.State.Status = "deleted"
	file.UpdatedAt = now.Add(6 * time.Minute)
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("soft-delete runtime file: %v", err)
	}
	projection.now = func() time.Time { return now.Add(7 * time.Minute) }
	if _, err := projection.Run(ctx); err != nil {
		t.Fatalf("delete Run: %v", err)
	}
	assertRuntimeCatalogState("trashed", "missing", false)

	file.State.Deleted = false
	file.State.Status = "active"
	file.UpdatedAt = now.Add(8 * time.Minute)
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("restore soft-deleted runtime file: %v", err)
	}
	projection.now = func() time.Time { return now.Add(9 * time.Minute) }
	if _, err := projection.Run(ctx); err != nil {
		t.Fatalf("delete recovery Run: %v", err)
	}
	assertRuntimeCatalogState("active", "available", true)

	var itemRevision, representationRevision, changesBefore int64
	if err := db.SQL.QueryRowContext(ctx, "SELECT revision FROM library_catalog_items WHERE id = ?", itemID).Scan(&itemRevision); err != nil {
		t.Fatalf("read item revision: %v", err)
	}
	if err := db.SQL.QueryRowContext(ctx, "SELECT revision FROM library_representations WHERE id = ?", assetID).Scan(&representationRevision); err != nil {
		t.Fatalf("read representation revision: %v", err)
	}
	if err := db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_changes").Scan(&changesBefore); err != nil {
		t.Fatalf("read change count: %v", err)
	}
	file.UpdatedAt = now.Add(10 * time.Minute)
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("touch healthy mapped file: %v", err)
	}
	projection.now = func() time.Time { return now.Add(11 * time.Minute) }
	if retry, err := projection.Run(ctx); err != nil || !retry.Completed || retry.BundlesProcessed != 1 {
		t.Fatalf("idempotent retry: result=%#v err=%v", retry, err)
	}
	var nextItemRevision, nextRepresentationRevision, changesAfter int64
	_ = db.SQL.QueryRowContext(ctx, "SELECT revision FROM library_catalog_items WHERE id = ?", itemID).Scan(&nextItemRevision)
	_ = db.SQL.QueryRowContext(ctx, "SELECT revision FROM library_representations WHERE id = ?", assetID).Scan(&nextRepresentationRevision)
	_ = db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_changes").Scan(&changesAfter)
	if nextItemRevision != itemRevision || nextRepresentationRevision != representationRevision || changesAfter != changesBefore {
		t.Fatalf("idempotent retry changed revisions/feed: item=%d/%d representation=%d/%d changes=%d/%d",
			itemRevision, nextItemRevision, representationRevision, nextRepresentationRevision, changesBefore, changesAfter)
	}
}

func TestLegacyCatalogBackfillKeepsTranscodeReplacementActive(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 18, 11, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-replacement", now)
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "episode.webm")
	transcodePath := filepath.Join(directory, "episode.mp4")
	for path, content := range map[string]string{
		sourcePath:    "downloaded source",
		transcodePath: "transcoded output",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	seedCatalogBackfillFile(
		t, ctx, db, "source-webm", "bundle-replacement", "video",
		"episode.webm", sourcePath, "", now,
	)

	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	projection := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun), files,
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	projection.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := projection.Run(ctx); err != nil {
		t.Fatalf("initial Run: %v", err)
	}

	seedCatalogBackfillFile(
		t, ctx, db, "output-mp4", "bundle-replacement", "transcode",
		"episode.mp4", transcodePath, "source-webm", now.Add(2*time.Minute),
	)
	transcode, err := files.Get(ctx, "output-mp4")
	if err != nil {
		t.Fatalf("get transcode: %v", err)
	}
	transcode.Media = &library.MediaInfo{Format: "mp4"}
	if err := files.Save(ctx, transcode); err != nil {
		t.Fatalf("save transcode media: %v", err)
	}
	source, err := files.Get(ctx, "source-webm")
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	source.State.Status = "deleted"
	source.State.Deleted = true
	source.UpdatedAt = now.Add(3 * time.Minute)
	if err := files.Save(ctx, source); err != nil {
		t.Fatalf("mark source replaced: %v", err)
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("remove source fixture: %v", err)
	}

	projection.now = func() time.Time { return now.Add(4 * time.Minute) }
	if _, err := projection.RunLibrary(ctx, "bundle-replacement"); err != nil {
		t.Fatalf("replacement RunLibrary: %v", err)
	}

	assertCatalogBackfillCount(t, ctx, db.SQL, "SELECT COUNT(*) FROM library_catalog_items", 1)
	var sourceItemID, transcodeItemID, sourceAvailability, transcodeAvailability string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT asset.item_id, representation.availability
FROM library_legacy_mappings AS mapping
JOIN library_item_assets AS asset ON asset.id = mapping.target_id
JOIN library_representations AS representation ON representation.asset_id = asset.id
WHERE mapping.migration_id = ? AND mapping.source_type = 'library_file' AND mapping.source_id = 'source-webm'
`, LegacyCatalogProjectionID).Scan(&sourceItemID, &sourceAvailability); err != nil {
		t.Fatalf("read source projection: %v", err)
	}
	if err := db.SQL.QueryRowContext(ctx, `
SELECT asset.item_id, representation.availability
FROM library_legacy_mappings AS mapping
JOIN library_item_assets AS asset ON asset.id = mapping.target_id
JOIN library_representations AS representation ON representation.asset_id = asset.id
WHERE mapping.migration_id = ? AND mapping.source_type = 'library_file' AND mapping.source_id = 'output-mp4'
`, LegacyCatalogProjectionID).Scan(&transcodeItemID, &transcodeAvailability); err != nil {
		t.Fatalf("read transcode projection: %v", err)
	}
	if sourceItemID != transcodeItemID {
		t.Fatalf("replacement split into two items: source=%q transcode=%q", sourceItemID, transcodeItemID)
	}
	if sourceAvailability != string(library.RepresentationAvailabilityMissing) ||
		transcodeAvailability != string(library.RepresentationAvailabilityAvailable) {
		t.Fatalf("unexpected representation availability: source=%q transcode=%q", sourceAvailability, transcodeAvailability)
	}
	var itemStatus string
	if err := db.SQL.QueryRowContext(ctx, "SELECT status FROM library_catalog_items WHERE id = ?", sourceItemID).Scan(&itemStatus); err != nil {
		t.Fatalf("read item status: %v", err)
	}
	if itemStatus != string(library.ItemStatusActive) {
		t.Fatalf("expected active replacement item, got %q", itemStatus)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_catalog_changes
WHERE catalog_id = ? AND entity_type = 'item' AND entity_id = ? AND kind = 'delete'
`, 0, DefaultLibraryCatalogID(), sourceItemID)

	catalogService := NewCatalogService(
		libraryrepo.NewSQLiteCatalogRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogItemRepository(db.Bun),
		libraryrepo.NewSQLiteItemAssetRepository(db.Bun),
		files,
		libraryrepo.NewSQLiteStorageRootRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun), nil,
	)
	listed, err := catalogService.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{Status: "all"})
	if err != nil {
		t.Fatalf("ListCatalogItems: %v", err)
	}
	if listed.Total != 1 || len(listed.Items) != 1 || listed.Items[0].PrimaryFileID != "output-mp4" ||
		listed.Items[0].Status != string(library.ItemStatusActive) || listed.Items[0].Format != "mp4" {
		t.Fatalf("unexpected replacement list item: %#v", listed)
	}
	overview, err := catalogService.GetDefaultCatalogOverview(ctx)
	if err != nil {
		t.Fatalf("GetDefaultCatalogOverview: %v", err)
	}
	if overview.Statuses.Active != 1 || overview.Statuses.Trashed != 0 {
		t.Fatalf("unexpected replacement overview statuses: %#v", overview.Statuses)
	}
	trashed, err := catalogService.TrashCatalogItem(ctx, dto.CatalogItemLifecycleRequest{
		ID: sourceItemID, ExpectedRevision: listed.Items[0].Revision, ActorID: "test-user",
	})
	if err != nil {
		t.Fatalf("TrashCatalogItem: %v", err)
	}
	projection.now = func() time.Time { return now.Add(6 * time.Minute) }
	if _, err := projection.RunLibrary(ctx, "bundle-replacement"); err != nil {
		t.Fatalf("RunLibrary after user trash: %v", err)
	}
	if err := db.SQL.QueryRowContext(ctx, "SELECT status FROM library_catalog_items WHERE id = ?", sourceItemID).Scan(&itemStatus); err != nil {
		t.Fatalf("read user-trashed item status: %v", err)
	}
	if itemStatus != string(library.ItemStatusTrashed) || trashed.Item.Status != string(library.ItemStatusTrashed) {
		t.Fatalf("replacement reconciliation overrode user trash: stored=%q response=%#v", itemStatus, trashed.Item)
	}
	restored, err := catalogService.RestoreCatalogItem(ctx, dto.CatalogItemLifecycleRequest{
		ID: sourceItemID, ExpectedRevision: trashed.Item.Revision, ActorID: "test-user",
	})
	if err != nil {
		t.Fatalf("RestoreCatalogItem with healthy representation: %v", err)
	}
	if restored.Item.Status != string(library.ItemStatusActive) {
		t.Fatalf("healthy replacement did not restore item active: %#v", restored.Item)
	}
}

func TestLegacyCatalogProjectionV2ReconcilesCompletedV1Replacement(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-v1-replacement", now)
	seedCatalogBackfillFile(
		t, ctx, db, "v1-source", "bundle-v1-replacement", "video",
		"episode.webm", "/legacy/episode.webm", "", now,
	)
	seedCatalogBackfillFile(
		t, ctx, db, "v1-output", "bundle-v1-replacement", "transcode",
		"episode.mp4", "/legacy/episode.mp4", "v1-source", now.Add(time.Second),
	)
	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	source, err := files.Get(ctx, "v1-source")
	if err != nil {
		t.Fatalf("get v1 source: %v", err)
	}
	source.State.Status = "deleted"
	source.State.Deleted = true
	if err := files.Save(ctx, source); err != nil {
		t.Fatalf("save v1 source state: %v", err)
	}

	projection := NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun), files,
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	projection.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := projection.Run(ctx); err != nil {
		t.Fatalf("seed current projection: %v", err)
	}
	var itemID string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT asset.item_id
FROM library_legacy_mappings AS mapping
JOIN library_item_assets AS asset ON asset.id = mapping.target_id
WHERE mapping.migration_id = ? AND mapping.source_type = 'library_file' AND mapping.source_id = 'v1-source'
`, LegacyCatalogProjectionID).Scan(&itemID); err != nil {
		t.Fatalf("read seeded replacement item: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx,
		"UPDATE library_legacy_mappings SET migration_id = 'catalog-foundation-v1' WHERE migration_id = ?",
		LegacyCatalogProjectionID,
	); err != nil {
		t.Fatalf("move mappings to v1: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx,
		"UPDATE library_migration_checkpoints SET migration_id = 'catalog-foundation-v1' WHERE migration_id = ?",
		LegacyCatalogProjectionID,
	); err != nil {
		t.Fatalf("move checkpoint to v1: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
UPDATE library_catalog_items SET status = 'trashed', trashed_at = ?, updated_at = ? WHERE id = ?
`, now.Add(2*time.Minute), now.Add(2*time.Minute), itemID); err != nil {
		t.Fatalf("simulate v1 replacement status: %v", err)
	}

	projection.now = func() time.Time { return now.Add(3 * time.Minute) }
	result, err := projection.Run(ctx)
	if err != nil {
		t.Fatalf("upgrade projection Run: %v", err)
	}
	if !result.Completed || result.FilesProcessed != 2 {
		t.Fatalf("unexpected v2 upgrade result: %#v", result)
	}
	var status string
	if err := db.SQL.QueryRowContext(ctx, "SELECT status FROM library_catalog_items WHERE id = ?", itemID).Scan(&status); err != nil {
		t.Fatalf("read upgraded item: %v", err)
	}
	if status != string(library.ItemStatusActive) {
		t.Fatalf("expected v2 to repair the old replacement item, got %q", status)
	}
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_legacy_mappings
WHERE migration_id = ? AND source_type = 'library_file'
`, 2, LegacyCatalogProjectionID)
	assertCatalogBackfillCount(t, ctx, db.SQL, `
SELECT COUNT(*) FROM library_migration_checkpoints
WHERE migration_id = ? AND phase = 'backfill' AND status = 'completed'
`, 1, LegacyCatalogProjectionID)
}

func openCatalogBackfillTestDatabase(t *testing.T) *persistence.Database {
	t.Helper()
	db, err := persistence.OpenSQLite(context.Background(), persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "catalog-backfill.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedCatalogBackfillLibrary(t *testing.T, ctx context.Context, db *persistence.Database, id string, at time.Time) {
	t.Helper()
	item, err := library.NewLibrary(library.LibraryParams{ID: id, Name: id, CreatedAt: &at, UpdatedAt: &at})
	if err != nil {
		t.Fatalf("new legacy library %s: %v", id, err)
	}
	if err := libraryrepo.NewSQLiteLibraryRepository(db.Bun).Save(ctx, item); err != nil {
		t.Fatalf("save legacy library %s: %v", id, err)
	}
}

func seedCatalogBackfillFile(
	t *testing.T,
	ctx context.Context,
	db *persistence.Database,
	id, libraryID, kind, name, path, rootID string,
	at time.Time,
) {
	t.Helper()
	storage := library.FileStorage{Mode: "local_path", LocalPath: path}
	if kind == string(library.FileKindSubtitle) {
		storage.Mode = "hybrid"
		storage.DocumentID = "document-" + id
	}
	item, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: id, LibraryID: libraryID, Kind: kind, Name: name, DisplayName: name,
		Storage: storage, Origin: library.FileOrigin{Kind: "download", OperationID: "operation-" + id},
		Lineage: library.FileLineage{RootFileID: rootID}, State: library.FileState{Status: "active"},
		CreatedAt: &at, UpdatedAt: &at,
	})
	if err != nil {
		t.Fatalf("new legacy file %s: %v", id, err)
	}
	if err := libraryrepo.NewSQLiteFileRepository(db.Bun).Save(ctx, item); err != nil {
		t.Fatalf("save legacy file %s: %v", id, err)
	}
}

func assertCatalogBackfillCount(t *testing.T, ctx context.Context, db *sql.DB, query string, expected int, args ...any) {
	t.Helper()
	var actual int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&actual); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if actual != expected {
		t.Fatalf("count = %d, want %d for query %q", actual, expected, query)
	}
}
