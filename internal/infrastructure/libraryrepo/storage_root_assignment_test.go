package libraryrepo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSQLiteFileRepositoryAssignsRootRelativeOwnershipWithoutChangingAbsolutePath(t *testing.T) {
	ctx := context.Background()
	database, err := openLibraryRepoTestDatabase(t, ctx, "storage-root-assignment.db")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer database.Close()

	now := time.Now().UTC()
	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-default", Name: "Library", Status: "active", IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	if err := NewSQLiteCatalogRepository(database.Bun).Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	rootPath := filepath.Join(t.TempDir(), "XiaDown")
	root, err := library.NewStorageRoot(library.StorageRootParams{
		ID: "root-downloads", CatalogID: catalog.ID, Name: "XiaDown Downloads", Path: rootPath,
		Mode: "managed", IsDefault: true, Status: "online",
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new storage root: %v", err)
	}
	if err := NewSQLiteStorageRootRepository(database.Bun).SaveAsDefault(ctx, root); err != nil {
		t.Fatalf("save default storage root: %v", err)
	}

	bundle, err := library.NewLibrary(library.LibraryParams{
		ID: "library-download", Name: "Download", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new Library: %v", err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, bundle); err != nil {
		t.Fatalf("save Library: %v", err)
	}
	absolutePath := filepath.Join(rootPath, "resource", "example", "video.mp4")
	file, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-download", LibraryID: bundle.ID, Kind: "video", Name: "video.mp4",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: absolutePath},
		Origin:    library.FileOrigin{Kind: "download", OperationID: "operation-download"},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new Library file: %v", err)
	}
	files := NewSQLiteFileRepository(database.Bun)
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("save Library file: %v", err)
	}

	loaded, err := files.Get(ctx, file.ID)
	if err != nil {
		t.Fatalf("load Library file: %v", err)
	}
	if loaded.Storage.LocalPath != absolutePath ||
		loaded.Storage.RootID != root.ID ||
		loaded.Storage.RelativePath != "resource/example/video.mp4" {
		t.Fatalf("unexpected dual-path storage ownership: %#v", loaded.Storage)
	}
}
