package service

import (
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestProjectLegacyCatalogBundleBuildsLogicalItemsWithoutChangingFiles(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	video := catalogProjectionFile(t, "video-1", "video", "movie.mp4", "", false, now)
	thumbnail := catalogProjectionFile(t, "thumb-1", "thumbnail", "movie.jpg", "", false, now.Add(time.Second))
	subtitle := catalogProjectionFile(t, "subtitle-1", "subtitle", "movie.srt", "video-1", false, now.Add(2*time.Second))
	transcode := catalogProjectionFile(t, "transcode-1", "transcode", "movie-720p.mp4", "video-1", false, now.Add(3*time.Second))
	book := catalogProjectionFile(t, "book-1", "document", "guide.epub", "", false, now.Add(4*time.Second))
	image := catalogProjectionFile(t, "image-1", "other", "photo.heic", "", false, now.Add(5*time.Second))

	projections, err := projectLegacyCatalogBundle(
		"catalog-1",
		[]library.LibraryFile{image, transcode, thumbnail, video, book, subtitle},
		now.Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("projectLegacyCatalogBundle: %v", err)
	}
	if len(projections) != 3 {
		t.Fatalf("projection count = %d, want 3", len(projections))
	}
	if projections[0].Item.Category != library.ItemCategoryVideo || len(projections[0].Assets) != 4 {
		t.Fatalf("video projection = %#v", projections[0])
	}
	roles := map[library.ItemAssetRole]int{}
	fileIDs := map[string]bool{}
	for _, asset := range projections[0].Assets {
		roles[asset.Role]++
		fileIDs[asset.FileID] = true
	}
	if roles[library.ItemAssetRoleOriginal] != 1 || roles[library.ItemAssetRoleArtwork] != 1 ||
		roles[library.ItemAssetRoleAttachment] != 1 || roles[library.ItemAssetRoleRepresentation] != 1 {
		t.Fatalf("unexpected video asset roles: %#v", roles)
	}
	for _, id := range []string{"video-1", "thumb-1", "subtitle-1", "transcode-1"} {
		if !fileIDs[id] {
			t.Fatalf("legacy file %q was not preserved as an item asset", id)
		}
	}
	if projections[1].Item.Category != library.ItemCategoryBook || projections[2].Item.Category != library.ItemCategoryImage {
		t.Fatalf("category projection = %q, %q", projections[1].Item.Category, projections[2].Item.Category)
	}

	mappingCount := 0
	for _, projection := range projections {
		mappingCount += len(projection.Mappings)
		for _, mapping := range projection.Mappings {
			if mapping.CatalogID != "catalog-1" || mapping.TargetType != library.CatalogEntityItemAsset {
				t.Fatalf("unexpected legacy mapping: %#v", mapping)
			}
		}
	}
	if mappingCount != 6 {
		t.Fatalf("mapping count = %d, want one per legacy file", mappingCount)
	}
}

func TestProjectLegacyCatalogBundleIsDeterministicAndQuarantinesOrphans(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	orphan := catalogProjectionFile(t, "subtitle-only", "subtitle", "captions.srt", "", false, now)
	trashed := catalogProjectionFile(t, "deleted-audio", "audio", "track.mp3", "", true, now.Add(time.Second))

	first, err := projectLegacyCatalogBundle("catalog-1", []library.LibraryFile{orphan}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("first projection: %v", err)
	}
	second, err := projectLegacyCatalogBundle("catalog-1", []library.LibraryFile{orphan}, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("second projection: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("projection counts = %d, %d", len(first), len(second))
	}
	if first[0].Item.ID != second[0].Item.ID || first[0].Assets[0].ID != second[0].Assets[0].ID {
		t.Fatal("projection IDs changed with input order or retry time")
	}
	if first[0].Item.Status != library.ItemStatusNeedsReview || first[0].Item.Category != library.ItemCategoryOther {
		t.Fatalf("orphan auxiliary was not quarantined: %#v", first[0].Item)
	}
	trashedProjection, err := projectLegacyCatalogBundle("catalog-1", []library.LibraryFile{trashed}, now.Add(time.Hour))
	if err != nil || len(trashedProjection) != 1 {
		t.Fatalf("trashed projection: %#v, %v", trashedProjection, err)
	}
	if trashedProjection[0].Item.Status != library.ItemStatusTrashed || trashedProjection[0].Item.TrashedAt == nil {
		t.Fatalf("deleted file did not become a recoverable trashed item: %#v", trashedProjection[0].Item)
	}
}

func TestProjectLegacyCatalogBundlePromotesAvailableTranscodeReplacement(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	source := catalogProjectionFile(t, "source-webm", "video", "episode.webm", "", true, now)
	transcode := catalogProjectionFile(t, "output-mp4", "transcode", "episode.mp4", source.ID, false, now.Add(time.Second))

	projections, err := projectLegacyCatalogBundle(
		"catalog-1",
		[]library.LibraryFile{source, transcode},
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("project replacement bundle: %v", err)
	}
	if len(projections) != 1 {
		t.Fatalf("expected one logical item, got %#v", projections)
	}
	if projections[0].Item.Status != library.ItemStatusActive || projections[0].Item.TrashedAt != nil {
		t.Fatalf("expected active item backed by the transcode, got %#v", projections[0].Item)
	}
}

func TestProjectLegacyCatalogBundleDoesNotPromoteArtworkAsMediaReplacement(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 9, 30, 0, 0, time.UTC)
	source := catalogProjectionFile(t, "source-webm", "video", "episode.webm", "", true, now)
	thumbnail := catalogProjectionFile(t, "cover-webp", "thumbnail", "episode.webp", source.ID, false, now.Add(time.Second))

	projections, err := projectLegacyCatalogBundle(
		"catalog-1",
		[]library.LibraryFile{source, thumbnail},
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("project artwork-only bundle: %v", err)
	}
	if len(projections) != 1 || projections[0].Item.Status != library.ItemStatusTrashed {
		t.Fatalf("expected deleted original to remain trashed without a media replacement, got %#v", projections)
	}
}

func catalogProjectionFile(t *testing.T, id, kind, name, rootID string, deleted bool, now time.Time) library.LibraryFile {
	t.Helper()
	originKind := "download"
	operationID := "op-" + id
	if kind == "transcode" {
		originKind = "transcode"
	}
	storageMode := "local_path"
	documentID := ""
	if kind == "subtitle" {
		storageMode = "hybrid"
		documentID = "document-" + id
	}
	item, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: id, LibraryID: "legacy-bundle", Kind: kind, Name: name, DisplayName: name,
		Storage:   library.FileStorage{Mode: storageMode, LocalPath: "/library/" + name, DocumentID: documentID},
		Origin:    library.FileOrigin{Kind: originKind, OperationID: operationID},
		Lineage:   library.FileLineage{RootFileID: rootID},
		State:     library.FileState{Status: "active", Deleted: deleted},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("NewLibraryFile(%s): %v", id, err)
	}
	return item
}
