package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestRecoverPendingJobsMarksRunningDownloadInterrupted(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-1", now)
	inputJSON, err := json.Marshal(dto.CreateYTDLPJobRequest{
		URL:   "https://example.com/video",
		Title: "Example",
	})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	total := int64(100)
	current := int64(40)
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          "op-running",
		LibraryID:   libraryItem.ID,
		Kind:        "download",
		Status:      string(library.OperationStatusRunning),
		DisplayName: "Example",
		InputJSON:   string(inputJSON),
		OutputJSON:  "{}",
		Progress: &library.OperationProgress{
			Stage:   progressText("library.progress.downloading"),
			Current: &current,
			Total:   &total,
		},
		CreatedAt: &now,
		StartedAt: &now,
	})
	if err != nil {
		t.Fatalf("new operation: %v", err)
	}
	history, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          "history-1",
		LibraryID:   libraryItem.ID,
		Category:    "operation",
		Action:      "download",
		DisplayName: operation.DisplayName,
		Status:      string(operation.Status),
		Refs:        library.HistoryRecordRefs{OperationID: operation.ID},
		OperationMeta: &library.OperationRecordMeta{
			Kind: operation.Kind,
		},
		OccurredAt: &now,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	})
	if err != nil {
		t.Fatalf("new history: %v", err)
	}

	operations := &retryOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	histories := &retryHistoryRepo{items: map[string]library.HistoryRecord{history.ID: history}}
	service := &LibraryService{
		operations: operations,
		histories:  histories,
		nowFunc:    func() time.Time { return now.Add(time.Minute) },
	}

	service.RecoverPendingJobs(ctx)

	storedOperation, err := operations.Get(ctx, operation.ID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if storedOperation.Status != library.OperationStatusFailed {
		t.Fatalf("expected interrupted operation to be failed, got %q", storedOperation.Status)
	}
	if storedOperation.ErrorCode != operationErrorCodeAppInterrupted {
		t.Fatalf("expected error code %q, got %q", operationErrorCodeAppInterrupted, storedOperation.ErrorCode)
	}
	if storedOperation.Progress == nil || storedOperation.Progress.Message != progressText("library.progressDetail.operationInterrupted") {
		t.Fatalf("expected interrupted progress message, got %#v", storedOperation.Progress)
	}

	storedHistory, err := histories.Get(ctx, history.ID)
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if storedHistory.Status != string(library.OperationStatusFailed) {
		t.Fatalf("expected history failed status, got %q", storedHistory.Status)
	}
	if storedHistory.OperationMeta == nil || storedHistory.OperationMeta.ErrorCode != operationErrorCodeAppInterrupted {
		t.Fatalf("expected history interrupted meta, got %#v", storedHistory.OperationMeta)
	}
}
