package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

type catalogPreviewAssetRepository struct{ items []library.ItemAsset }

func (repo catalogPreviewAssetRepository) ListByItemID(context.Context, string) ([]library.ItemAsset, error) {
	return append([]library.ItemAsset(nil), repo.items...), nil
}
func (repo catalogPreviewAssetRepository) Get(_ context.Context, id string) (library.ItemAsset, error) {
	for _, item := range repo.items {
		if item.ID == id {
			return item, nil
		}
	}
	return library.ItemAsset{}, library.ErrFileNotFound
}
func (catalogPreviewAssetRepository) Save(context.Context, library.ItemAsset) error { return nil }
func (catalogPreviewAssetRepository) Delete(context.Context, string) error          { return nil }

type catalogPreviewFileRepository struct {
	items map[string]library.LibraryFile
}

func (repo catalogPreviewFileRepository) List(context.Context) ([]library.LibraryFile, error) {
	result := make([]library.LibraryFile, 0, len(repo.items))
	for _, item := range repo.items {
		result = append(result, item)
	}
	return result, nil
}
func (repo catalogPreviewFileRepository) ListByLibraryID(context.Context, string) ([]library.LibraryFile, error) {
	return repo.List(context.Background())
}
func (repo catalogPreviewFileRepository) Get(_ context.Context, id string) (library.LibraryFile, error) {
	item, ok := repo.items[id]
	if !ok {
		return library.LibraryFile{}, library.ErrFileNotFound
	}
	return item, nil
}
func (catalogPreviewFileRepository) Save(context.Context, library.LibraryFile) error { return nil }
func (catalogPreviewFileRepository) Delete(context.Context, string) error            { return nil }

func TestCatalogListItemDTOSelectsOpaquePrimaryAndArtworkReferences(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	original := catalogListPreviewFile(t, "file-video", "video", filepath.Join(directory, "movie.mp4"), now)
	artwork := catalogListPreviewFile(t, "file-cover", "thumbnail", filepath.Join(directory, "cover.webp"), now)
	for _, path := range []string{original.Storage.LocalPath, artwork.Storage.LocalPath} {
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	primaryAsset := catalogListPreviewAsset(t, "asset-primary", "item-1", original.ID, "original", now)
	artworkAsset := catalogListPreviewAsset(t, "asset-artwork", "item-1", artwork.ID, "artwork", now)
	item, err := library.NewItem(library.ItemParams{
		ID: "item-1", CatalogID: "catalog-1", Category: "video", Status: "active",
		Title: "Movie", Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new item: %v", err)
	}
	service := &CatalogService{
		assets: catalogPreviewAssetRepository{items: []library.ItemAsset{artworkAsset, primaryAsset}},
		files: catalogPreviewFileRepository{items: map[string]library.LibraryFile{
			original.ID: original,
			artwork.ID:  artwork,
		}},
	}

	result, err := service.catalogListItemDTO(context.Background(), item)
	if err != nil {
		t.Fatalf("catalogListItemDTO: %v", err)
	}
	if result.PrimaryAssetID != primaryAsset.ID || result.PrimaryFileID != original.ID ||
		result.ArtworkAssetID != artworkAsset.ID || result.ArtworkFileID != artwork.ID {
		t.Fatalf("unexpected preview references: %#v", result)
	}
	if result.Kind != "video" || result.Format != "mp4" {
		t.Fatalf("artwork replaced primary media facts: %#v", result)
	}
}

func TestCatalogListItemDTOUsesAvailableTranscodeWhenOriginalWasReplaced(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	source := catalogListPreviewFile(t, "file-source", "video", filepath.Join(directory, "episode.webm"), now)
	source.State.Status = "deleted"
	source.State.Deleted = true
	transcode := catalogListPreviewFile(t, "file-transcode", "transcode", filepath.Join(directory, "episode.mp4"), now.Add(time.Second))
	transcode.Lineage.RootFileID = source.ID
	if err := os.WriteFile(transcode.Storage.LocalPath, []byte("transcoded media"), 0o600); err != nil {
		t.Fatalf("write transcode fixture: %v", err)
	}
	originalAsset := catalogListPreviewAsset(t, "asset-original", "item-replaced", source.ID, "original", now)
	transcodeAsset := catalogListPreviewAsset(t, "asset-transcode", "item-replaced", transcode.ID, "representation", now.Add(time.Second))
	item, err := library.NewItem(library.ItemParams{
		ID: "item-replaced", CatalogID: "catalog-1", Category: "video", Status: "active",
		Title: "Episode", Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new item: %v", err)
	}
	service := &CatalogService{
		assets: catalogPreviewAssetRepository{items: []library.ItemAsset{originalAsset, transcodeAsset}},
		files: catalogPreviewFileRepository{items: map[string]library.LibraryFile{
			source.ID:    source,
			transcode.ID: transcode,
		}},
	}

	result, err := service.catalogListItemDTO(context.Background(), item)
	if err != nil {
		t.Fatalf("catalogListItemDTO: %v", err)
	}
	if result.PrimaryAssetID != transcodeAsset.ID || result.PrimaryFileID != transcode.ID {
		t.Fatalf("expected transcode primary references, got %#v", result)
	}
	if result.Kind != string(library.FileKindTranscode) || result.Format != "mp4" {
		t.Fatalf("expected transcode media facts, got %#v", result)
	}
}

func catalogListPreviewFile(t *testing.T, id, kind, path string, now time.Time) library.LibraryFile {
	t.Helper()
	media := &library.MediaInfo{Format: filepath.Ext(path)[1:]}
	item, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: id, LibraryID: "legacy-1", Kind: kind, Name: filepath.Base(path),
		Storage: library.FileStorage{Mode: "local_path", LocalPath: path},
		Origin:  library.FileOrigin{Kind: "download", OperationID: "operation-" + id}, Media: media,
		State: library.FileState{Status: "active"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new file: %v", err)
	}
	return item
}

func catalogListPreviewAsset(t *testing.T, id, itemID, fileID, role string, now time.Time) library.ItemAsset {
	t.Helper()
	item, err := library.NewItemAsset(library.ItemAssetParams{
		ID: id, ItemID: itemID, FileID: fileID, Role: role, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new asset: %v", err)
	}
	return item
}
