package libraryrepo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteFileEventRepositoryKeepsFirstImmutablePayload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "file-event-immutable.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	now := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-event", Name: "Events", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-event", LibraryID: libraryItem.ID, Kind: string(library.FileKindVideo), Name: "event.mp4",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: filepath.Join(t.TempDir(), "event.mp4")},
		Origin:  library.FileOrigin{Kind: "download", OperationID: "historical-op"},
		State:   library.FileState{Status: "active"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	if err := NewSQLiteFileRepository(database.Bun).Save(ctx, fileItem); err != nil {
		t.Fatalf("save file: %v", err)
	}
	repo := NewSQLiteFileEventRepository(database.Bun)
	first, err := library.NewFileEventRecord(library.FileEventRecordParams{
		ID: "event-fixed", LibraryID: libraryItem.ID, FileID: fileItem.ID,
		EventType: "file_renamed", DetailJSON: `{"changes":[{"field":"displayName","after":"First"}]}`, CreatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build first event: %v", err)
	}
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("save first event: %v", err)
	}
	retry := first
	retry.DetailJSON = `{"changes":[{"field":"displayName","after":"Rewritten"}]}`
	if err := repo.Save(ctx, retry); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	items, err := repo.ListByLibraryID(ctx, libraryItem.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(items) != 1 || items[0].DetailJSON != first.DetailJSON {
		t.Fatalf("immutable event was rewritten: %#v", items)
	}
}

func TestSQLiteFileRepositoryRollsBackFileWhenLifecycleEventFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "file-event-atomic.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	now := time.Date(2026, 7, 20, 16, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-atomic", Name: "Atomic", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-atomic", LibraryID: libraryItem.ID, Kind: string(library.FileKindThumbnail), Name: "atomic.webp",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: filepath.Join(t.TempDir(), "atomic.webp")},
		Origin:    library.FileOrigin{Kind: "download", OperationID: "operation-atomic"},
		State:     library.FileState{Status: "deleted", Deleted: true},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	fileRepo := NewSQLiteFileRepository(database.Bun)
	if err := fileRepo.Save(ctx, fileItem); err != nil {
		t.Fatalf("save file: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		CREATE TRIGGER fail_file_lifecycle_event
		BEFORE INSERT ON library_file_events
		BEGIN
			SELECT RAISE(ABORT, 'forced lifecycle event failure');
		END;
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	restoredAt := now.Add(time.Hour)
	restored := fileItem
	restored.State.Status = "active"
	restored.State.Deleted = false
	restored.UpdatedAt = restoredAt
	event, err := library.NewFileEventRecord(library.FileEventRecordParams{
		ID: "event-restore", LibraryID: libraryItem.ID, FileID: fileItem.ID,
		EventType: "file_restored", DetailJSON: `{"changes":[{"field":"fileLifecycle"}]}`,
		CreatedAt: &restoredAt,
	})
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	if err := fileRepo.SaveWithFileEvent(ctx, restored, event); err == nil {
		t.Fatal("atomic save unexpectedly succeeded")
	}

	retained, err := fileRepo.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("reload file: %v", err)
	}
	if !retained.State.Deleted || retained.State.Status != "deleted" {
		t.Fatalf("file projection escaped failed transaction: %+v", retained.State)
	}
	events, err := NewSQLiteFileEventRepository(database.Bun).ListByLibraryID(ctx, libraryItem.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("failed transaction retained events: %+v", events)
	}
}

func TestSQLiteFileRepositoryRenameWithEventIsAtomicAndConcurrencySafe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "file-rename-atomic.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	createdAt := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-rename-atomic", Name: "Atomic rename", CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-rename-atomic", LibraryID: libraryItem.ID, Kind: string(library.FileKindVideo),
		Name: "original.mp4", DisplayName: "Original title",
		Metadata:  library.FileMetadata{Title: "Original metadata"},
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: filepath.Join(t.TempDir(), "original.mp4")},
		Origin:    library.FileOrigin{Kind: "download", OperationID: "operation-original"},
		State:     library.FileState{Status: "active"},
		CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	fileRepo := NewSQLiteFileRepository(database.Bun)
	if err := fileRepo.Save(ctx, fileItem); err != nil {
		t.Fatalf("save file: %v", err)
	}

	renamedAt := createdAt.Add(2 * time.Hour)
	staleRename := fileItem
	staleRename.DisplayName = "Renamed title"
	staleRename.UpdatedAt = renamedAt
	event, err := library.NewFileEventRecord(library.FileEventRecordParams{
		ID: "event-file-renamed", LibraryID: libraryItem.ID, FileID: fileItem.ID,
		EventType:  "file_renamed",
		DetailJSON: `{"changes":[{"field":"displayName","before":"Original title","after":"Renamed title"}]}`,
		CreatedAt:  &renamedAt,
	})
	if err != nil {
		t.Fatalf("build rename event: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		CREATE TRIGGER fail_file_rename_event
		BEFORE INSERT ON library_file_events
		WHEN NEW.event_type = 'file_renamed'
		BEGIN
			SELECT RAISE(ABORT, 'forced rename event failure');
		END;
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	if err := fileRepo.RenameDisplayNameWithFileEvent(ctx, staleRename, event); err == nil {
		t.Fatal("atomic rename unexpectedly succeeded")
	}
	retained, err := fileRepo.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("reload rolled-back file: %v", err)
	}
	if retained.DisplayName != fileItem.DisplayName || !retained.UpdatedAt.Equal(fileItem.UpdatedAt) {
		t.Fatalf("rename projection escaped failed transaction: %#v", retained)
	}
	eventsAfterRollback, err := NewSQLiteFileEventRepository(database.Bun).ListByLibraryID(ctx, libraryItem.ID)
	if err != nil || len(eventsAfterRollback) != 0 {
		t.Fatalf("failed transaction retained rename event: events=%#v err=%v", eventsAfterRollback, err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DROP TRIGGER fail_file_rename_event`); err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}

	// Simulate state/relink/metadata persistence committing after RenameFile read
	// its aggregate but before the narrow rename transaction starts.
	concurrentAt := createdAt.Add(time.Hour)
	concurrent := fileItem
	concurrent.Name = "moved.mp4"
	concurrent.Metadata.Title = "Newer metadata"
	concurrent.Storage.LocalPath = filepath.Join(t.TempDir(), "moved.mp4")
	concurrent.LatestOperationID = "operation-newer"
	concurrent.State.LastError = "missing_local_file"
	concurrent.State.LastChecked = concurrentAt.Format(time.RFC3339)
	concurrent.UpdatedAt = concurrentAt
	if err := fileRepo.Save(ctx, concurrent); err != nil {
		t.Fatalf("save concurrent file state: %v", err)
	}
	if err := fileRepo.RenameDisplayNameWithFileEvent(ctx, staleRename, event); err != nil {
		t.Fatalf("atomic rename: %v", err)
	}
	stored, err := fileRepo.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("reload renamed file: %v", err)
	}
	if stored.DisplayName != staleRename.DisplayName || !stored.UpdatedAt.Equal(renamedAt) {
		t.Fatalf("rename projection not committed: %#v", stored)
	}
	if stored.Name != concurrent.Name || stored.Metadata.Title != concurrent.Metadata.Title ||
		stored.Storage.LocalPath != concurrent.Storage.LocalPath || stored.LatestOperationID != concurrent.LatestOperationID ||
		stored.State.LastError != concurrent.State.LastError || stored.State.LastChecked != concurrent.State.LastChecked {
		t.Fatalf("rename overwrote concurrent file state: %#v", stored)
	}
	storedEvents, err := NewSQLiteFileEventRepository(database.Bun).ListByLibraryID(ctx, libraryItem.ID)
	if err != nil || len(storedEvents) != 1 || storedEvents[0].ID != event.ID || storedEvents[0].EventType != "file_renamed" {
		t.Fatalf("rename event not committed: events=%#v err=%v", storedEvents, err)
	}

	// Simulate a background writer completing from its pre-rename snapshot. Its
	// owned state changes commit, but the newer title and timestamp cannot regress.
	staleBackground := concurrent
	staleBackground.State.Status = "verified"
	staleBackground.UpdatedAt = concurrentAt
	if err := fileRepo.SavePreservingDisplayName(ctx, staleBackground); err != nil {
		t.Fatalf("save stale background projection: %v", err)
	}
	finalFile, err := fileRepo.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("reload final file: %v", err)
	}
	if finalFile.DisplayName != staleRename.DisplayName || !finalFile.UpdatedAt.Equal(renamedAt) {
		t.Fatalf("background projection regressed rename: %#v", finalFile)
	}
	if finalFile.State.Status != staleBackground.State.Status {
		t.Fatalf("background-owned state was not persisted: %#v", finalFile.State)
	}
}
