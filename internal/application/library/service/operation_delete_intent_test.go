package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
)

type actionFailHistoryRepository struct {
	library.HistoryRepository
	action string
	err    error
}

func (repo *actionFailHistoryRepository) Save(ctx context.Context, item library.HistoryRecord) error {
	if item.Action == repo.action {
		return repo.err
	}
	return repo.HistoryRepository.Save(ctx, item)
}

type deleteFailOperationRepository struct {
	library.OperationRepository
	err error
}

func (repo *deleteFailOperationRepository) Delete(context.Context, string) error {
	return repo.err
}

func TestDeleteOperationCompletionHistoryFailureStillAppearsInDeleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openLibraryServiceTestDatabase(t, "operation-delete-intent.db")

	now := time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-delete-intent", now)
	fileItem := mustNewVideoFile(t, "file-delete-intent", libraryItem.ID, "op-delete-intent", "/tmp/delete-intent.mp4", now)
	operationItem := mustNewOperationWithKind(t, "op-delete-intent", "download", libraryItem.ID, []library.OperationOutputFile{{
		FileID: fileItem.ID, Kind: "video", Format: "mp4", IsPrimary: true,
	}}, now)
	libraries := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	files := libraryrepo.NewSQLiteFileRepository(database.Bun)
	operations := libraryrepo.NewSQLiteOperationRepository(database.Bun)
	storedHistories := libraryrepo.NewSQLiteHistoryRepository(database.Bun)
	if err := libraries.Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	if err := files.Save(ctx, fileItem); err != nil {
		t.Fatalf("save file: %v", err)
	}
	if err := operations.Save(ctx, operationItem); err != nil {
		t.Fatalf("save operation: %v", err)
	}

	completionErr := errors.New("injected operation_deleted history failure")
	histories := &actionFailHistoryRepository{
		HistoryRepository: storedHistories,
		action:            operationEventDeleted,
		err:               completionErr,
	}
	service := &LibraryService{
		libraries:  libraries,
		files:      files,
		operations: operations,
		histories:  histories,
		nowFunc:    func() time.Time { return now },
	}

	if err := service.DeleteOperation(ctx, dto.DeleteOperationRequest{OperationID: operationItem.ID}); !errors.Is(err, completionErr) {
		t.Fatalf("expected injected completion failure, got %v", err)
	}
	if _, err := operations.Get(ctx, operationItem.ID); !errors.Is(err, library.ErrOperationNotFound) {
		t.Fatalf("operation should already be removed, got %v", err)
	}
	records, err := storedHistories.ListByLibraryID(ctx, libraryItem.ID)
	if err != nil {
		t.Fatalf("list histories: %v", err)
	}
	if len(records) != 1 || records[0].Action != operationEventDeleteRequested ||
		records[0].OperationMeta == nil || records[0].OperationMeta.Kind != operationItem.Kind ||
		len(records[0].Files) != 1 || records[0].Files[0].FileID != fileItem.ID ||
		records[0].Refs.SubjectOperationID != operationItem.ID {
		t.Fatalf("deletion intent did not retain the full task snapshot: %#v", records)
	}

	deleted, err := service.ListDeletedLibraryItems(ctx, dto.ListDeletedLibraryItemsRequest{
		Kinds: []string{deletedLibraryItemKindTask}, LibraryID: libraryItem.ID,
	})
	if err != nil {
		t.Fatalf("ListDeletedLibraryItems: %v", err)
	}
	if deleted.Total != 1 || len(deleted.Items) != 1 || deleted.Items[0].ID != operationItem.ID ||
		deleted.Items[0].Detail.TaskHistory == nil ||
		deleted.Items[0].Detail.TaskHistory.Action != operationEventDeleteRequested ||
		deleted.Items[0].Category != operationItem.Kind {
		t.Fatalf("already-removed task disappeared after completion failure: %#v", deleted)
	}
}

func TestDeleteOperationRequestIsNotListedWhileOperationStillExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openLibraryServiceTestDatabase(t, "operation-delete-request-live.db")

	now := time.Date(2026, 7, 20, 18, 5, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-delete-request-live", now)
	operationItem := mustNewOperationWithKind(t, "op-delete-request-live", "transcode", libraryItem.ID, nil, now)
	libraries := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	storedOperations := libraryrepo.NewSQLiteOperationRepository(database.Bun)
	histories := libraryrepo.NewSQLiteHistoryRepository(database.Bun)
	if err := libraries.Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	if err := storedOperations.Save(ctx, operationItem); err != nil {
		t.Fatalf("save operation: %v", err)
	}

	deleteErr := errors.New("injected operation delete failure")
	operations := &deleteFailOperationRepository{OperationRepository: storedOperations, err: deleteErr}
	service := &LibraryService{
		libraries:  libraries,
		operations: operations,
		histories:  histories,
		nowFunc:    func() time.Time { return now },
	}
	if err := service.DeleteOperation(ctx, dto.DeleteOperationRequest{OperationID: operationItem.ID}); !errors.Is(err, deleteErr) {
		t.Fatalf("expected injected operation delete failure, got %v", err)
	}
	if _, err := storedOperations.Get(ctx, operationItem.ID); err != nil {
		t.Fatalf("operation should still exist: %v", err)
	}

	deleted, err := service.ListDeletedLibraryItems(ctx, dto.ListDeletedLibraryItemsRequest{
		Kinds: []string{deletedLibraryItemKindTask}, LibraryID: libraryItem.ID,
	})
	if err != nil {
		t.Fatalf("ListDeletedLibraryItems: %v", err)
	}
	if deleted.Total != 0 || len(deleted.Items) != 0 {
		t.Fatalf("live task was misrepresented as deleted: %#v", deleted)
	}
	records, err := histories.ListByLibraryID(ctx, libraryItem.ID)
	if err != nil {
		t.Fatalf("list histories: %v", err)
	}
	if len(records) != 1 || records[0].Action != operationEventDeleteRequested {
		t.Fatalf("expected durable request audit despite failed delete: %#v", records)
	}
}
