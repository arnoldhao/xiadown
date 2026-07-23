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
)

func TestDeleteOperationPreservesHistoryAndStableSubjectWithSQLite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openLibraryServiceTestDatabase(t, "operation-history-audit.db")

	createdAt := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	deletedAt := createdAt.Add(time.Minute)
	localPath := filepath.Join(t.TempDir(), "preserved.mp4")
	if err := os.WriteFile(localPath, []byte("video"), 0o600); err != nil {
		t.Fatalf("write output: %v", err)
	}
	libraryItem := mustNewLibrary(t, "lib-history-audit", createdAt)
	fileItem := mustNewVideoFile(t, "file-history-audit", libraryItem.ID, "op-history-audit", localPath, createdAt)
	output := library.OperationOutputFile{FileID: fileItem.ID, Kind: "video", IsPrimary: true}
	operation := mustNewOperationWithKind(t, "op-history-audit", "download", libraryItem.ID, []library.OperationOutputFile{output}, createdAt)
	history := mustNewHistoryForOperation(t, "history-original", libraryItem.ID, operation.ID, operation.Kind, operation.OutputFiles, createdAt)

	libraries := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	files := libraryrepo.NewSQLiteFileRepository(database.Bun)
	operations := libraryrepo.NewSQLiteOperationRepository(database.Bun)
	histories := libraryrepo.NewSQLiteHistoryRepository(database.Bun)
	if err := libraries.Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	if err := files.Save(ctx, fileItem); err != nil {
		t.Fatalf("save file: %v", err)
	}
	if err := operations.Save(ctx, operation); err != nil {
		t.Fatalf("save operation: %v", err)
	}
	if err := histories.Save(ctx, history); err != nil {
		t.Fatalf("save history: %v", err)
	}
	service := &LibraryService{
		libraries:  libraries,
		files:      files,
		operations: operations,
		histories:  histories,
		nowFunc:    func() time.Time { return deletedAt },
	}

	if err := service.DeleteOperation(ctx, dto.DeleteOperationRequest{OperationID: operation.ID}); err != nil {
		t.Fatalf("DeleteOperation: %v", err)
	}
	if _, err := operations.Get(ctx, operation.ID); err != library.ErrOperationNotFound {
		t.Fatalf("operation still exists: %v", err)
	}
	stored, err := histories.ListByLibraryID(ctx, libraryItem.ID)
	if err != nil {
		t.Fatalf("list stored histories: %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("expected original snapshot plus deletion request and completion, got %#v", stored)
	}
	byAction := make(map[string]library.HistoryRecord, len(stored))
	for _, item := range stored {
		byAction[item.Action] = item
		if item.Refs.OperationID != "" || item.Refs.SubjectOperationID != operation.ID {
			t.Fatalf("history lost stable subject after operation hard delete: %#v", item.Refs)
		}
	}
	original := byAction["download"]
	if len(original.Files) != 1 || original.Files[0].FileID != fileItem.ID {
		t.Fatalf("original output snapshot was changed: %#v", original.Files)
	}
	request := byAction[operationEventDeleteRequested]
	if request.Category != operationEventHistoryCategory || request.Source.Kind != "user_action" ||
		request.Source.Actor != libraryFileEventActorDesktop || request.Source.Caller != "keep_files" ||
		request.OperationMeta == nil || request.OperationMeta.Kind != operation.Kind ||
		len(request.Files) != 1 || request.Files[0].FileID != fileItem.ID ||
		!request.OccurredAt.Equal(deletedAt) {
		t.Fatalf("unexpected deletion request: %#v", request)
	}
	deletion := byAction["operation_deleted"]
	if deletion.Category != operationEventHistoryCategory || deletion.Source.Kind != "user_action" ||
		deletion.Source.Actor != libraryFileEventActorDesktop ||
		deletion.Source.Caller != "keep_files" || len(deletion.Files) != 1 || deletion.Files[0].FileID != fileItem.ID ||
		!deletion.OccurredAt.Equal(deletedAt) {
		t.Fatalf("unexpected deletion activity: %#v", deletion)
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("keep-files task deletion removed local output: %v", err)
	}

	dtos, err := service.ListLibraryHistory(ctx, dto.ListLibraryHistoryRequest{LibraryID: libraryItem.ID})
	if err != nil {
		t.Fatalf("ListLibraryHistory: %v", err)
	}
	if len(dtos) != 3 {
		t.Fatalf("expected three history DTOs, got %#v", dtos)
	}
	for _, item := range dtos {
		if item.Refs.OperationID != operation.ID {
			t.Fatalf("history DTO did not expose stable operation subject: %#v", item)
		}
	}
}
