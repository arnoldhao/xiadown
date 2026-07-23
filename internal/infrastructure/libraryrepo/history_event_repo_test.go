package libraryrepo

import (
	"context"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSQLiteHistoryRepositoryKeepsFirstOperationEventSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := openLibraryRepoTestDatabase(t, ctx, "operation-event-immutable.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	now := time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-operation-event", Name: "Events", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID: "op-operation-event", LibraryID: libraryItem.ID, Kind: "download",
		Status: "succeeded", DisplayName: "Original task", Correlation: library.OperationCorrelation{},
		InputJSON: "{}", OutputJSON: "{}", CreatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	if err := NewSQLiteOperationRepository(database.Bun).Save(ctx, operation); err != nil {
		t.Fatalf("save operation: %v", err)
	}

	first, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: "event-fixed", LibraryID: libraryItem.ID, Category: "operation_event",
		Action: "operation_renamed", DisplayName: "Original task", Status: "succeeded",
		Source: library.HistoryRecordSource{Kind: "user_action", Actor: "desktop-library"},
		Refs: library.HistoryRecordRefs{
			OperationID: operation.ID, SubjectOperationID: operation.ID,
		},
		OccurredAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build first event: %v", err)
	}
	repo := NewSQLiteHistoryRepository(database.Bun)
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("save first event: %v", err)
	}

	rewrittenAt := now.Add(time.Hour)
	retry, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: first.ID, LibraryID: libraryItem.ID, Category: "operation_event",
		Action: "operation_canceled", DisplayName: "Rewritten task", Status: "running",
		Source: library.HistoryRecordSource{Kind: "user_action", Actor: "different-actor"},
		Refs: library.HistoryRecordRefs{
			OperationID: operation.ID, SubjectOperationID: operation.ID,
		},
		OccurredAt: &rewrittenAt, CreatedAt: &rewrittenAt, UpdatedAt: &rewrittenAt,
	})
	if err != nil {
		t.Fatalf("build retry event: %v", err)
	}
	if err := repo.Save(ctx, retry); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}

	stored, err := repo.Get(ctx, first.ID)
	if err != nil {
		t.Fatalf("get stored event: %v", err)
	}
	if stored.Action != first.Action || stored.DisplayName != first.DisplayName ||
		stored.Status != first.Status || stored.Source.Actor != first.Source.Actor ||
		!stored.OccurredAt.Equal(first.OccurredAt) {
		t.Fatalf("immutable operation event was rewritten: %#v", stored)
	}
}
