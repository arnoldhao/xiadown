package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestListMissingLibraryFilesReturnsOnlyDefinitelyMissingPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 7, 19, 11, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	existingPath := filepath.Join(tempDir, "existing.mp3")
	if err := os.WriteFile(existingPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	missing := mustNewAudioFile(t, "missing", "lib-maintenance", "op-missing", filepath.Join(tempDir, "missing.mp3"), now)
	existing := mustNewAudioFile(t, "existing", "lib-maintenance", "op-existing", existingPath, now)
	directory := mustNewAudioFile(t, "directory", "lib-maintenance", "op-directory", tempDir, now)
	indeterminate := mustNewAudioFile(t, "indeterminate", "lib-maintenance", "op-indeterminate", filepath.Join(tempDir, "invalid\x00path.mp3"), now)
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{
		missing.ID:       missing,
		existing.ID:      existing,
		directory.ID:     directory,
		indeterminate.ID: indeterminate,
	}}
	fileEvents := &deleteRuleFileEventRepo{}
	service := &LibraryService{files: files, fileEvents: fileEvents, nowFunc: func() time.Time { return now }}

	result, err := service.ListMissingLibraryFiles(ctx)
	if err != nil {
		t.Fatalf("list missing library files: %v", err)
	}
	if result.Checked != 4 {
		t.Fatalf("checked = %d, want 4", result.Checked)
	}
	if len(result.Missing) != 1 || result.Missing[0].FileID != missing.ID {
		t.Fatalf("missing candidates = %#v, want only %q", result.Missing, missing.ID)
	}
	if files.items[missing.ID].State.LastError != missingLocalFileError {
		t.Fatalf("definitely missing file was not marked missing: %#v", files.items[missing.ID].State)
	}
	for _, fileID := range []string{directory.ID, indeterminate.ID} {
		if files.items[fileID].State.LastError != "" {
			t.Fatalf("indeterminate file %q was marked missing: %#v", fileID, files.items[fileID].State)
		}
	}
	if len(fileEvents.items) != 1 || fileEvents.items[0].EventType != libraryFileEventMissingDetected ||
		fileEvents.items[0].OperationID != "op-missing" {
		t.Fatalf("expected one missing transition event, got %#v", fileEvents.items)
	}
	if _, err := service.ListMissingLibraryFiles(ctx); err != nil {
		t.Fatalf("repeat missing scan: %v", err)
	}
	if len(fileEvents.items) != 1 {
		t.Fatalf("unchanged missing state emitted duplicate events: %#v", fileEvents.items)
	}
	if err := os.WriteFile(missing.Storage.LocalPath, []byte("restored audio"), 0o644); err != nil {
		t.Fatalf("restore missing file: %v", err)
	}
	if _, err := service.ListMissingLibraryFiles(ctx); err != nil {
		t.Fatalf("scan restored file: %v", err)
	}
	if len(fileEvents.items) != 2 || fileEvents.items[1].EventType != libraryFileEventAvailableAgain ||
		fileEvents.items[1].OperationID != "op-missing" {
		t.Fatalf("expected one availability recovery event, got %#v", fileEvents.items)
	}
}

func TestClearSelectedMissingLibraryFilesOnlyClearsSelectedDefiniteMissingRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 7, 19, 11, 5, 0, 0, time.UTC)
	tempDir := t.TempDir()
	existingPath := filepath.Join(tempDir, "existing.mp3")
	if err := os.WriteFile(existingPath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	missing := mustNewAudioFile(t, "missing-selected", "lib-maintenance", "op-missing", filepath.Join(tempDir, "missing.mp3"), now)
	existing := mustNewAudioFile(t, "existing-selected", "lib-maintenance", "op-existing", existingPath, now)
	indeterminate := mustNewAudioFile(t, "indeterminate-selected", "lib-maintenance", "op-indeterminate", filepath.Join(tempDir, "invalid\x00path.mp3"), now)
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{
		missing.ID:       missing,
		existing.ID:      existing,
		indeterminate.ID: indeterminate,
	}}
	service := &LibraryService{
		files: files,
		libraries: &deleteRuleLibraryRepo{items: map[string]library.Library{
			missing.LibraryID: mustNewLibrary(t, missing.LibraryID, now),
		}},
		nowFunc: func() time.Time { return now.Add(time.Minute) },
	}

	result, err := service.ClearSelectedMissingLibraryFiles(ctx, dto.ClearSelectedMissingLibraryFilesRequest{
		FileIDs: []string{missing.ID, existing.ID, indeterminate.ID, missing.ID, " "},
	})
	if err != nil {
		t.Fatalf("clear selected missing library files: %v", err)
	}
	if result.Checked != 3 || result.Removed != 1 {
		t.Fatalf("clear result = %#v, want checked=3 removed=1", result)
	}
	if !files.items[missing.ID].State.Deleted {
		t.Fatalf("missing record was not cleared: %#v", files.items[missing.ID].State)
	}
	for _, fileID := range []string{existing.ID, indeterminate.ID} {
		if files.items[fileID].State.Deleted {
			t.Fatalf("non-missing record %q was cleared: %#v", fileID, files.items[fileID].State)
		}
	}
	if content, readErr := os.ReadFile(existingPath); readErr != nil || string(content) != "keep me" {
		t.Fatalf("selected cleanup touched the local file: content=%q err=%v", content, readErr)
	}
}

func TestClearSelectedMissingLibraryFilesRechecksAfterRelink(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 7, 19, 11, 10, 0, 0, time.UTC)
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old", "song.mp3")
	newPath := filepath.Join(tempDir, "new", "song.mp3")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatalf("mkdir relink directory: %v", err)
	}
	if err := os.WriteFile(newPath, []byte("relinked"), 0o644); err != nil {
		t.Fatalf("write relinked file: %v", err)
	}

	fileItem := mustNewAudioFile(t, "missing-relinked", "lib-maintenance", "op-download", oldPath, now)
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	service := &LibraryService{
		files: files,
		libraries: &deleteRuleLibraryRepo{items: map[string]library.Library{
			fileItem.LibraryID: mustNewLibrary(t, fileItem.LibraryID, now),
		}},
		nowFunc: func() time.Time { return now.Add(time.Minute) },
	}

	scan, err := service.ListMissingLibraryFiles(ctx)
	if err != nil || len(scan.Missing) != 1 {
		t.Fatalf("scan missing before relink: %#v err=%v", scan, err)
	}
	if _, err := service.applyLibraryFileRelink(ctx, fileItem, newPath); err != nil {
		t.Fatalf("apply relink: %v", err)
	}

	result, err := service.ClearSelectedMissingLibraryFiles(ctx, dto.ClearSelectedMissingLibraryFilesRequest{
		FileIDs: []string{fileItem.ID},
	})
	if err != nil {
		t.Fatalf("clear stale selection: %v", err)
	}
	if result.Removed != 0 || files.items[fileItem.ID].State.Deleted {
		t.Fatalf("stale selection cleared relinked file: result=%#v state=%#v", result, files.items[fileItem.ID].State)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("relinked local file was touched: %v", err)
	}
}
