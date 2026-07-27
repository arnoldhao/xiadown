package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestCatalogItemSourceSeparatesManagedLocationFromDownloadOrigin(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "Downloads")
	managedPath := filepath.Join(parent, "xiadown")
	source := catalogItemSourceDTO(
		library.LibraryFile{
			Storage: library.FileStorage{
				RootID: "managed-root", LocalPath: filepath.Join(managedPath, "movie.mp4"),
			},
			Origin: library.FileOrigin{Kind: "download", OperationID: "operation-1"},
		},
		[]library.StorageRoot{{
			ID: "managed-root", Name: "XiaDown Downloads", Path: managedPath,
			Mode: library.StorageRootModeManaged, IsDefault: true,
		}},
	)

	if source.OriginKind != "download" ||
		source.OperationID != "operation-1" ||
		source.StorageMode != "managed" ||
		source.StorageRootPath != parent {
		t.Fatalf("unexpected managed source: %#v", source)
	}
}

func TestCatalogItemSourcePreservesReferencedImportEvidence(t *testing.T) {
	rootPath := t.TempDir()
	importedAt := time.Now().UTC().Add(-time.Hour)
	filePath := filepath.Join(rootPath, "recording.flac")
	if err := os.WriteFile(filePath, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write referenced file: %v", err)
	}
	source := catalogItemSourceDTO(
		library.LibraryFile{
			Storage: library.FileStorage{LocalPath: filePath},
			Origin: library.FileOrigin{
				Kind: "import",
				Import: &library.ImportOrigin{
					BatchID: "batch-1", ImportPath: filePath,
					ImportedAt: importedAt, KeepSourceFile: true,
				},
			},
		},
		[]library.StorageRoot{{
			ID: "reference-root", Name: "Recordings", Path: rootPath,
			Mode: library.StorageRootModeReferenced,
		}},
	)

	if source.OriginKind != "import" ||
		source.StorageMode != "referenced" ||
		source.StorageRootID != "reference-root" ||
		source.ImportBatchID != "batch-1" ||
		source.ImportPath != filePath ||
		source.ImportedAt == "" ||
		!source.KeepSourceFile {
		t.Fatalf("unexpected referenced source: %#v", source)
	}
}
