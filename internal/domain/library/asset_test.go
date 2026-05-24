package library

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewLibraryFileAcceptsDownloadResultKindsWithLocalPath(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	kinds := []FileKind{
		FileKindOther,
		FileKindDocument,
		FileKindFont,
		FileKindAPI,
		FileKindArchive,
		FileKindManifest,
	}
	for _, kind := range kinds {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			item, err := NewLibraryFile(LibraryFileParams{
				ID:        "file-" + string(kind),
				LibraryID: "lib-1",
				Kind:      string(kind),
				Name:      "payload",
				Storage: FileStorage{
					Mode:      "local_path",
					LocalPath: filepath.Join(t.TempDir(), "payload.bin"),
				},
				Origin:    FileOrigin{Kind: "download", OperationID: "op-1"},
				State:     FileState{Status: "active"},
				CreatedAt: &now,
				UpdatedAt: &now,
			})
			if err != nil {
				t.Fatalf("NewLibraryFile: %v", err)
			}
			if item.Kind != kind {
				t.Fatalf("expected %q kind, got %q", kind, item.Kind)
			}
		})
	}
}

func TestNewLibraryFileRejectsDownloadResultKindsWithoutLocalPath(t *testing.T) {
	t.Parallel()

	kinds := []FileKind{
		FileKindOther,
		FileKindDocument,
		FileKindFont,
		FileKindAPI,
		FileKindArchive,
		FileKindManifest,
	}
	for _, kind := range kinds {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			_, err := NewLibraryFile(LibraryFileParams{
				ID:        "file-" + string(kind),
				LibraryID: "lib-1",
				Kind:      string(kind),
				Name:      "payload",
				Storage:   FileStorage{Mode: "local_path"},
				Origin:    FileOrigin{Kind: "download", OperationID: "op-1"},
				State:     FileState{Status: "active"},
			})
			if err == nil {
				t.Fatalf("expected missing local path to be rejected")
			}
		})
	}
}
