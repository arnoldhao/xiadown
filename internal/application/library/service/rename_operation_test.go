package service

import (
	"context"
	"testing"
	"time"

	librarydto "xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestRenameOperationUpdatesOperationHistoryAndLibraryTimestamp(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	renamedAt := createdAt.Add(2 * time.Hour)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID:        "lib-1",
		Name:      "Library",
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          "op-1",
		LibraryID:   libraryItem.ID,
		Kind:        "download",
		Status:      string(library.OperationStatusSucceeded),
		DisplayName: "Old title",
		CreatedAt:   &createdAt,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	history, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          "history-1",
		LibraryID:   libraryItem.ID,
		Category:    "operation",
		Action:      "download",
		DisplayName: "Old title",
		Status:      string(library.OperationStatusSucceeded),
		Refs:        library.HistoryRecordRefs{OperationID: operation.ID},
		OccurredAt:  &createdAt,
		CreatedAt:   &createdAt,
		UpdatedAt:   &createdAt,
	})
	if err != nil {
		t.Fatalf("build history: %v", err)
	}

	libraries := &ytdlpMetadataLibraryRepo{item: libraryItem}
	operations := &ytdlpMetadataOperationRepo{saved: []library.LibraryOperation{operation}}
	histories := &ytdlpMetadataHistoryRepo{saved: []library.HistoryRecord{history}}
	service := &LibraryService{
		libraries:  libraries,
		operations: operations,
		histories:  histories,
		nowFunc: func() time.Time {
			return renamedAt
		},
	}

	result, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
		OperationID: " op-1 ",
		Name:        " New title ",
	})
	if err != nil {
		t.Fatalf("rename operation: %v", err)
	}
	if result.DisplayName != "New title" {
		t.Fatalf("expected renamed dto, got %q", result.DisplayName)
	}
	if got := operations.saved[len(operations.saved)-1].DisplayName; got != "New title" {
		t.Fatalf("expected operation display name to update, got %q", got)
	}
	savedHistory := histories.saved[len(histories.saved)-1]
	if savedHistory.DisplayName != "New title" {
		t.Fatalf("expected history display name to update, got %q", savedHistory.DisplayName)
	}
	if !savedHistory.UpdatedAt.Equal(renamedAt) {
		t.Fatalf("expected history timestamp %s, got %s", renamedAt, savedHistory.UpdatedAt)
	}
	if !libraries.item.UpdatedAt.Equal(renamedAt) {
		t.Fatalf("expected library timestamp %s, got %s", renamedAt, libraries.item.UpdatedAt)
	}
}

func TestRenameFileUpdatesDisplayNameAndLibraryTimestamp(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	renamedAt := createdAt.Add(2 * time.Hour)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID:        "lib-1",
		Name:      "Library",
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:          "file-1",
		LibraryID:   libraryItem.ID,
		Kind:        string(library.FileKindVideo),
		Name:        "old.mp4",
		DisplayName: "Old file",
		Storage: library.FileStorage{
			Mode:      "local_path",
			LocalPath: "/tmp/old.mp4",
		},
		Origin: library.FileOrigin{
			Kind:        "download",
			OperationID: "op-1",
		},
		State:     library.FileState{Status: "active"},
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	libraries := &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}}
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	service := &LibraryService{
		libraries: libraries,
		files:     files,
		nowFunc: func() time.Time {
			return renamedAt
		},
	}

	result, err := service.RenameFile(context.Background(), librarydto.RenameFileRequest{
		FileID: " file-1 ",
		Name:   " New file ",
	})
	if err != nil {
		t.Fatalf("rename file: %v", err)
	}
	if result.DisplayName != "New file" {
		t.Fatalf("expected renamed file dto, got %q", result.DisplayName)
	}
	if result.Name != "old.mp4" || result.FileName != "old.mp4" {
		t.Fatalf("expected stored and physical file names to remain, got name=%q fileName=%q", result.Name, result.FileName)
	}
	savedFile := files.savedItems[len(files.savedItems)-1]
	if savedFile.DisplayName != "New file" {
		t.Fatalf("expected file display name to update, got %q", savedFile.DisplayName)
	}
	if !savedFile.UpdatedAt.Equal(renamedAt) {
		t.Fatalf("expected file timestamp %s, got %s", renamedAt, savedFile.UpdatedAt)
	}
	if !libraries.items[libraryItem.ID].UpdatedAt.Equal(renamedAt) {
		t.Fatalf("expected library timestamp %s, got %s", renamedAt, libraries.items[libraryItem.ID].UpdatedAt)
	}
}

func TestNormalizeLibraryDisplayNameValidation(t *testing.T) {
	t.Parallel()

	if got, err := normalizeLibraryDisplayName("  新名称  "); err != nil || got != "新名称" {
		t.Fatalf("expected unicode display name to pass, got name=%q err=%v", got, err)
	}

	invalidNames := []string{
		"",
		"bad/name",
		"bad\\name",
		"bad:name",
		"line\nbreak",
		".",
		"..",
		"CON",
	}
	for _, name := range invalidNames {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeLibraryDisplayName(name); err == nil {
				t.Fatalf("expected %q to be rejected", name)
			}
		})
	}
}
