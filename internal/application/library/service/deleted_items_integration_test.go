package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/persistence"
)

func TestDeletedLibraryItemsListTaskAndRestoreFileWithoutLosingAudit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "deleted-library-items.db"),
	})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer database.Close()

	libraryRepo := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	fileRepo := libraryrepo.NewSQLiteFileRepository(database.Bun)
	historyRepo := libraryrepo.NewSQLiteHistoryRepository(database.Bun)
	fileEventRepo := libraryrepo.NewSQLiteFileEventRepository(database.Bun)
	service := NewLibraryService(
		libraryRepo, nil, fileRepo, nil, nil, nil, nil, nil, historyRepo, nil,
		fileEventRepo, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	createdAt := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)
	deletedAt := createdAt.Add(time.Hour)
	restoredAt := deletedAt.Add(time.Hour)
	service.nowFunc = func() time.Time { return restoredAt }

	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "library-deleted", Name: "Deleted", CreatedBy: library.CreateMeta{Source: "test"},
		CreatedAt: &createdAt, UpdatedAt: &deletedAt,
	})
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}
	if err := libraryRepo.Save(ctx, libraryItem); err != nil {
		t.Fatalf("Save library: %v", err)
	}

	localPath := filepath.Join(t.TempDir(), "deleted-image.webp")
	if err := os.WriteFile(localPath, []byte("image"), 0o600); err != nil {
		t.Fatalf("write local fixture: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-deleted", LibraryID: libraryItem.ID, Kind: string(library.FileKindThumbnail),
		Name: "deleted-image.webp", DisplayName: "Deleted image",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: localPath},
		Origin: library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{
			BatchID: "batch-deleted", ImportPath: localPath, ImportedAt: createdAt,
		}},
		State:     library.FileState{Status: "deleted", Deleted: true},
		CreatedAt: &createdAt, UpdatedAt: &deletedAt,
	})
	if err != nil {
		t.Fatalf("NewLibraryFile: %v", err)
	}
	if err := fileRepo.Save(ctx, fileItem); err != nil {
		t.Fatalf("Save file: %v", err)
	}

	deletion, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: "history-operation-deleted", LibraryID: libraryItem.ID,
		Category: operationEventHistoryCategory, Action: operationEventDeleted,
		DisplayName: "Deleted download", Status: string(library.OperationStatusSucceeded),
		Source:        library.HistoryRecordSource{Kind: "user_action", Actor: libraryFileEventActorDesktop},
		Refs:          library.HistoryRecordRefs{SubjectOperationID: "operation-deleted"},
		Files:         []library.OperationOutputFile{{FileID: fileItem.ID, Kind: string(fileItem.Kind), IsPrimary: true}},
		Metrics:       library.OperationMetrics{FileCount: 1},
		OperationMeta: &library.OperationRecordMeta{Kind: "download"},
		OccurredAt:    &deletedAt, CreatedAt: &deletedAt, UpdatedAt: &deletedAt,
	})
	if err != nil {
		t.Fatalf("NewHistoryRecord: %v", err)
	}
	if err := historyRepo.Save(ctx, deletion); err != nil {
		t.Fatalf("Save deletion history: %v", err)
	}

	listed, err := service.ListDeletedLibraryItems(ctx, dto.ListDeletedLibraryItemsRequest{})
	if err != nil {
		t.Fatalf("ListDeletedLibraryItems: %v", err)
	}
	if listed.Total != 2 || len(listed.Items) != 2 {
		t.Fatalf("deleted items = %+v, want task and file", listed)
	}
	for _, item := range listed.Items {
		switch item.Kind {
		case deletedLibraryItemKindTask:
			if item.CanRestore || item.Detail.TaskHistory == nil || item.Category != "download" {
				t.Fatalf("task deleted item = %+v", item)
			}
		case deletedLibraryItemKindFile:
			if !item.CanRestore || item.Detail.File == nil || item.Category != "image" {
				t.Fatalf("file deleted item = %+v", item)
			}
		default:
			t.Fatalf("unexpected deleted item kind %q", item.Kind)
		}
	}

	if _, err := service.RestoreDeletedLibraryItem(ctx, dto.DeletedLibraryItemMutationRequest{
		Kind: deletedLibraryItemKindFile, ID: fileItem.ID,
	}); err != nil {
		t.Fatalf("Restore file: %v", err)
	}

	after, err := service.ListDeletedLibraryItems(ctx, dto.ListDeletedLibraryItemsRequest{})
	if err != nil {
		t.Fatalf("List after purge: %v", err)
	}
	if after.Total != 1 || len(after.Items) != 1 || after.Items[0].Kind != deletedLibraryItemKindTask {
		t.Fatalf("deleted companion after restore = %+v, want retained task only", after)
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("restore removed local payload: %v", err)
	}
	retainedFile, err := fileRepo.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("restored file row missing: %v", err)
	}
	if retainedFile.State.Deleted || retainedFile.State.Status != "active" {
		t.Fatalf("restored file state = %+v", retainedFile.State)
	}
	histories, err := historyRepo.ListByLibraryID(ctx, libraryItem.ID)
	if err != nil {
		t.Fatalf("List histories: %v", err)
	}
	if len(histories) != 1 {
		t.Fatalf("history count = %d, want immutable task deletion audit", len(histories))
	}
	if item := histories[0]; item.Action != operationEventDeleted ||
		historySubjectOperationID(item) != "operation-deleted" || len(item.Files) != 1 {
		t.Fatalf("history audit snapshot lost identity/files: %+v", item)
	}
	events, err := fileEventRepo.ListByLibraryID(ctx, libraryItem.ID)
	if err != nil {
		t.Fatalf("List file events: %v", err)
	}
	if len(events) != 1 || events[0].EventType != libraryFileEventRestored || events[0].FileID != fileItem.ID {
		t.Fatalf("file restore audit = %+v", events)
	}

	var historyFileCount, fileRowCount, foreignKeyViolations int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_history_files WHERE file_id = ?`, fileItem.ID).Scan(&historyFileCount); err != nil {
		t.Fatalf("count retained history files: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_files WHERE id = ?`, fileItem.ID).Scan(&fileRowCount); err != nil {
		t.Fatalf("count retained file tombstone: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&foreignKeyViolations); err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	if historyFileCount != 1 || fileRowCount != 1 || foreignKeyViolations != 0 {
		t.Fatalf("deleted audit integrity: historyFiles=%d fileRows=%d foreignKeys=%d", historyFileCount, fileRowCount, foreignKeyViolations)
	}
}

func TestRestoreDeletedLibraryItemRejectsTaskAndMissingFile(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	if _, err := service.RestoreDeletedLibraryItem(context.Background(), dto.DeletedLibraryItemMutationRequest{
		Kind: deletedLibraryItemKindTask, ID: "operation-deleted",
	}); err == nil {
		t.Fatal("task restore unexpectedly succeeded")
	}
	if _, err := service.RestoreDeletedLibraryItem(context.Background(), dto.DeletedLibraryItemMutationRequest{
		Kind: deletedLibraryItemKindFile, ID: "file-missing",
	}); err == nil {
		t.Fatal("missing file restore unexpectedly succeeded")
	}
}
