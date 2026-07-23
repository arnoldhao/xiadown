package libraryrepo

import (
	"context"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSQLiteOperationRepositorySaveWithHistoryEventIsAtomic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := openLibraryRepoTestDatabase(t, ctx, "operation-history-atomic.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	now := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-operation-atomic", Name: "Atomic", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID: "operation-atomic", LibraryID: libraryItem.ID, Kind: "download",
		Status: "succeeded", DisplayName: "Original title", InputJSON: "{}", OutputJSON: "{}",
		CreatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	primary, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: "history-primary", LibraryID: libraryItem.ID, Category: "operation", Action: "download",
		DisplayName: operation.DisplayName, Status: string(operation.Status),
		Source:     library.HistoryRecordSource{Kind: "download"},
		Refs:       library.HistoryRecordRefs{OperationID: operation.ID},
		OccurredAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build primary history: %v", err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	operationRepo := NewSQLiteOperationRepository(database.Bun)
	historyRepo := NewSQLiteHistoryRepository(database.Bun)
	if err := operationRepo.Save(ctx, operation); err != nil {
		t.Fatalf("save operation: %v", err)
	}
	if err := historyRepo.Save(ctx, primary); err != nil {
		t.Fatalf("save primary history: %v", err)
	}

	renamedAt := now.Add(time.Hour)
	renamedOperation := operation
	renamedOperation.DisplayName = "Renamed title"
	renamedPrimary := primary
	renamedPrimary.DisplayName = renamedOperation.DisplayName
	renamedPrimary.UpdatedAt = renamedAt
	event, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: "event-rename", LibraryID: libraryItem.ID, Category: "operation_event", Action: "operation_renamed",
		DisplayName: operation.DisplayName, Status: string(operation.Status),
		Source: library.HistoryRecordSource{Kind: "user_action", Actor: "library-companion"},
		Refs: library.HistoryRecordRefs{
			OperationID: operation.ID, SubjectOperationID: operation.ID,
		},
		OccurredAt: &renamedAt, CreatedAt: &renamedAt, UpdatedAt: &renamedAt,
	})
	if err != nil {
		t.Fatalf("build lifecycle event: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		CREATE TRIGGER fail_operation_lifecycle_event
		BEFORE INSERT ON library_history_records
		WHEN NEW.category = 'operation_event'
		BEGIN
			SELECT RAISE(ABORT, 'forced lifecycle event failure');
		END;
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if err := operationRepo.SaveWithHistoryEvent(ctx, renamedOperation, &renamedPrimary, event); err == nil {
		t.Fatal("atomic save unexpectedly succeeded")
	}
	retainedOperation, err := operationRepo.Get(ctx, operation.ID)
	if err != nil {
		t.Fatalf("reload operation: %v", err)
	}
	if retainedOperation.DisplayName != operation.DisplayName {
		t.Fatalf("operation projection escaped failed transaction: %q", retainedOperation.DisplayName)
	}
	retainedPrimary, err := historyRepo.Get(ctx, primary.ID)
	if err != nil {
		t.Fatalf("reload primary history: %v", err)
	}
	if retainedPrimary.DisplayName != primary.DisplayName || !retainedPrimary.UpdatedAt.Equal(primary.UpdatedAt) {
		t.Fatalf("primary history escaped failed transaction: %#v", retainedPrimary)
	}
	if _, err := historyRepo.Get(ctx, event.ID); err != library.ErrHistoryRecordNotFound {
		t.Fatalf("failed transaction retained lifecycle event: %v", err)
	}

	if _, err := database.SQL.ExecContext(ctx, `DROP TRIGGER fail_operation_lifecycle_event`); err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}
	// Simulate progress/status persistence racing after the rename caller read
	// its snapshot. The narrow rename transaction must not write those stale
	// fields back while changing the title.
	concurrentOperation := operation
	concurrentOperation.Status = library.OperationStatusFailed
	concurrentOperation.ErrorCode = "concurrent_failure"
	concurrentOperation.ErrorMessage = "newer operation state"
	concurrentOperation.OutputJSON = `{"newer":true}`
	if err := operationRepo.Save(ctx, concurrentOperation); err != nil {
		t.Fatalf("save concurrent operation state: %v", err)
	}
	concurrentPrimary := primary
	concurrentPrimary.Status = string(library.OperationStatusFailed)
	if err := historyRepo.Save(ctx, concurrentPrimary); err != nil {
		t.Fatalf("save concurrent primary history state: %v", err)
	}
	if err := operationRepo.SaveWithHistoryEvent(ctx, renamedOperation, &renamedPrimary, event); err != nil {
		t.Fatalf("atomic save: %v", err)
	}
	storedOperation, err := operationRepo.Get(ctx, operation.ID)
	if err != nil {
		t.Fatalf("reload renamed operation: %v", err)
	}
	storedPrimary, err := historyRepo.Get(ctx, primary.ID)
	if err != nil {
		t.Fatalf("reload renamed primary history: %v", err)
	}
	storedEvent, err := historyRepo.Get(ctx, event.ID)
	if err != nil {
		t.Fatalf("reload lifecycle event: %v", err)
	}
	if storedOperation.DisplayName != renamedOperation.DisplayName || storedPrimary.DisplayName != renamedPrimary.DisplayName {
		t.Fatalf("atomic save did not refresh projections: operation=%q history=%q", storedOperation.DisplayName, storedPrimary.DisplayName)
	}
	if storedOperation.Status != concurrentOperation.Status || storedOperation.ErrorCode != concurrentOperation.ErrorCode ||
		storedOperation.OutputJSON != concurrentOperation.OutputJSON || storedPrimary.Status != concurrentPrimary.Status {
		t.Fatalf("rename overwrote newer operation/history state: operation=%#v history=%#v", storedOperation, storedPrimary)
	}
	if storedEvent.Category != "operation_event" || storedEvent.Action != event.Action {
		t.Fatalf("atomic save did not persist lifecycle event: %#v", storedEvent)
	}

	// Once the rename event exists, every ordinary background/terminal upsert
	// may advance runtime fields but must treat both display-name projections as
	// user owned. SQLite serializes a concurrent writer on either side of the
	// rename transaction, so this stale post-rename write covers the losing side
	// of the interleaving deterministically.
	staleTerminalOperation := operation
	staleTerminalOperation.Status = library.OperationStatusSucceeded
	staleTerminalOperation.OutputJSON = `{"terminal":true}`
	if err := operationRepo.Save(ctx, staleTerminalOperation); err != nil {
		t.Fatalf("save stale terminal operation: %v", err)
	}
	staleTerminalPrimary := primary
	staleTerminalPrimary.Status = string(library.OperationStatusSucceeded)
	staleTerminalPrimary.UpdatedAt = renamedAt.Add(time.Minute)
	if err := historyRepo.Save(ctx, staleTerminalPrimary); err != nil {
		t.Fatalf("save stale terminal primary history: %v", err)
	}
	terminalOperation, err := operationRepo.Get(ctx, operation.ID)
	if err != nil {
		t.Fatalf("reload terminal operation: %v", err)
	}
	terminalPrimary, err := historyRepo.Get(ctx, primary.ID)
	if err != nil {
		t.Fatalf("reload terminal primary history: %v", err)
	}
	if terminalOperation.DisplayName != renamedOperation.DisplayName || terminalPrimary.DisplayName != renamedPrimary.DisplayName {
		t.Fatalf("stale terminal save reverted rename: operation=%q history=%q", terminalOperation.DisplayName, terminalPrimary.DisplayName)
	}
	if terminalOperation.Status != staleTerminalOperation.Status || terminalOperation.OutputJSON != staleTerminalOperation.OutputJSON ||
		terminalPrimary.Status != staleTerminalPrimary.Status {
		t.Fatalf("runtime fields did not advance after rename: operation=%#v history=%#v", terminalOperation, terminalPrimary)
	}
}
