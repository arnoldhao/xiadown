package service

import (
	"context"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestCancelOperationSynchronizesPrimaryHistoryAndAppendsPreCancelEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	createdAt := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
	canceledAt := createdAt.Add(15 * time.Minute)
	sizeBytes := int64(8192)
	durationMs := int64(3200)
	outputs := []library.OperationOutputFile{{
		FileID:    "file-cancel",
		Kind:      "video",
		Format:    "mp4",
		SizeBytes: &sizeBytes,
		IsPrimary: true,
	}}
	metrics := library.OperationMetrics{FileCount: 1, TotalSizeBytes: &sizeBytes, DurationMs: &durationMs}
	libraryItem := mustNewLibrary(t, "lib-cancel-event", createdAt)
	operation := mustLifecycleOperation(t, lifecycleOperationParams{
		ID:          "op-cancel-event",
		LibraryID:   libraryItem.ID,
		Kind:        "download",
		Status:      library.OperationStatusRunning,
		DisplayName: "Running download",
		InputJSON:   "{}",
		OutputFiles: outputs,
		Metrics:     metrics,
		CreatedAt:   createdAt,
	})
	primary := mustLifecycleHistory(t, "history-cancel-primary", operation, operationHistoryCategory, "download", createdAt)
	olderEvent := mustLifecycleHistory(t, "history-cancel-older-event", operation, operationEventHistoryCategory, operationEventResumed, createdAt.Add(time.Minute))
	olderEvent.Status = string(library.OperationStatusFailed)

	histories := &deleteRuleHistoryRepo{items: map[string]library.HistoryRecord{
		primary.ID:    primary,
		olderEvent.ID: olderEvent,
	}}
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	service := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		operations: operations,
		histories:  histories,
		nowFunc:    func() time.Time { return canceledAt },
	}

	result, err := service.CancelOperation(ctx, dto.CancelOperationRequest{OperationID: operation.ID})
	if err != nil {
		t.Fatalf("CancelOperation: %v", err)
	}
	if result.Status != string(library.OperationStatusCanceled) || result.FinishedAt != canceledAt.Format(time.RFC3339) {
		t.Fatalf("unexpected canceled operation DTO: %#v", result)
	}
	current := operations.items[operation.ID]
	if current.Status != library.OperationStatusCanceled || current.FinishedAt == nil || !current.FinishedAt.Equal(canceledAt) {
		t.Fatalf("operation did not retain exact cancel time: %#v", current)
	}

	updatedPrimary := histories.items[primary.ID]
	if updatedPrimary.Category != operationHistoryCategory || updatedPrimary.Status != string(library.OperationStatusCanceled) ||
		!updatedPrimary.OccurredAt.Equal(canceledAt) || !updatedPrimary.UpdatedAt.Equal(canceledAt) ||
		updatedPrimary.OperationMeta == nil || updatedPrimary.OperationMeta.ErrorCode != "canceled" ||
		len(updatedPrimary.Files) != 1 || updatedPrimary.Files[0].FileID != outputs[0].FileID {
		t.Fatalf("primary History was not synchronized to canceled state: %#v", updatedPrimary)
	}
	cancelEvent, ok := lifecycleEventByAction(histories.items, operationEventCanceled)
	if !ok || cancelEvent.Status != string(library.OperationStatusRunning) || cancelEvent.DisplayName != operation.DisplayName ||
		!cancelEvent.OccurredAt.Equal(canceledAt) || cancelEvent.Refs.OperationID != operation.ID ||
		cancelEvent.Refs.SubjectOperationID != operation.ID || cancelEvent.Source.Kind != "user_action" ||
		cancelEvent.Source.Actor != libraryFileEventActorDesktop || len(cancelEvent.Files) != 1 ||
		cancelEvent.Metrics.FileCount != 1 || cancelEvent.Metrics.DurationMs == nil || *cancelEvent.Metrics.DurationMs != durationMs {
		t.Fatalf("missing immutable pre-cancel event: %#v", cancelEvent)
	}
	if got := histories.items[olderEvent.ID]; got.Status != string(library.OperationStatusFailed) {
		t.Fatalf("primary lookup overwrote an operation event: %#v", got)
	}
}

func TestResumeDownloadPreservesFailedAttemptInLifecycleEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	createdAt := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	resumedAt := createdAt.Add(30 * time.Minute)
	sizeBytes := int64(16384)
	outputs := []library.OperationOutputFile{{FileID: "file-resume", Kind: "image", Format: "webp", SizeBytes: &sizeBytes}}
	metrics := library.OperationMetrics{FileCount: 1, TotalSizeBytes: &sizeBytes}
	libraryItem := mustNewLibrary(t, "lib-resume-event", createdAt)
	operation := mustLifecycleOperation(t, lifecycleOperationParams{
		ID:           "op-resume-event",
		LibraryID:    libraryItem.ID,
		Kind:         "download",
		Status:       library.OperationStatusFailed,
		DisplayName:  "Failed download",
		InputJSON:    `{"url":"https://example.com/video"}`,
		OutputJSON:   `{"mainPath":"/tmp/partial.webm"}`,
		OutputFiles:  outputs,
		Metrics:      metrics,
		ErrorCode:    "download_failed",
		ErrorMessage: "network unavailable",
		CreatedAt:    createdAt,
	})
	primary := mustLifecycleHistory(t, "history-resume-primary", operation, operationHistoryCategory, "download", createdAt)
	olderEvent := mustLifecycleHistory(t, "history-resume-older-event", operation, operationEventHistoryCategory, operationEventCanceled, createdAt.Add(time.Minute))
	olderEvent.Status = string(library.OperationStatusRunning)

	histories := &deleteRuleHistoryRepo{items: map[string]library.HistoryRecord{
		primary.ID:    primary,
		olderEvent.ID: olderEvent,
	}}
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	service := &LibraryService{
		libraries:    &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		operations:   operations,
		histories:    histories,
		nowFunc:      func() time.Time { return resumedAt },
		shuttingDown: true,
	}

	result, err := service.ResumeOperation(ctx, dto.ResumeOperationRequest{OperationID: operation.ID})
	if err != nil {
		t.Fatalf("ResumeOperation: %v", err)
	}
	if result.Status != string(library.OperationStatusQueued) || len(result.OutputFiles) != 0 || result.Metrics.FileCount != 0 {
		t.Fatalf("unexpected resumed operation DTO: %#v", result)
	}
	current := operations.items[operation.ID]
	if current.Status != library.OperationStatusQueued || current.ErrorCode != "" || current.ErrorMessage != "" ||
		len(current.OutputFiles) != 0 || current.Metrics.FileCount != 0 {
		t.Fatalf("operation was not reset for retry: %#v", current)
	}
	updatedPrimary := histories.items[primary.ID]
	if updatedPrimary.Category != operationHistoryCategory || updatedPrimary.Status != string(library.OperationStatusQueued) ||
		len(updatedPrimary.Files) != 0 || updatedPrimary.Metrics.FileCount != 0 {
		t.Fatalf("primary History was not reset to current retry state: %#v", updatedPrimary)
	}
	resumeEvent, ok := lifecycleEventByAction(histories.items, operationEventResumed)
	if !ok || resumeEvent.Status != string(library.OperationStatusFailed) || resumeEvent.DisplayName != operation.DisplayName ||
		!resumeEvent.OccurredAt.Equal(resumedAt) || len(resumeEvent.Files) != 1 ||
		resumeEvent.Files[0].FileID != outputs[0].FileID || resumeEvent.Metrics.FileCount != 1 ||
		resumeEvent.Metrics.TotalSizeBytes == nil || *resumeEvent.Metrics.TotalSizeBytes != sizeBytes {
		t.Fatalf("failed attempt was not preserved by resume event: %#v", resumeEvent)
	}
	if got := histories.items[olderEvent.ID]; got.Status != string(library.OperationStatusRunning) {
		t.Fatalf("resume overwrote an older lifecycle event: %#v", got)
	}
}

func TestTranscodeCancelAndResumeRecordEventsWithoutPrimaryHistory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	createdAt := time.Date(2026, 7, 20, 11, 0, 0, 0, time.UTC)
	eventAt := createdAt.Add(time.Hour)

	t.Run("cancel", func(t *testing.T) {
		operation := mustLifecycleOperation(t, lifecycleOperationParams{
			ID: "op-transcode-cancel", LibraryID: "lib-transcode", Kind: "transcode",
			Status: library.OperationStatusQueued, DisplayName: "Queued transcode", InputJSON: "{}", CreatedAt: createdAt,
		})
		histories := &deleteRuleHistoryRepo{items: map[string]library.HistoryRecord{}}
		service := &LibraryService{
			operations: &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}},
			histories:  histories,
			nowFunc:    func() time.Time { return eventAt },
		}
		if _, err := service.CancelOperation(ctx, dto.CancelOperationRequest{OperationID: operation.ID}); err != nil {
			t.Fatalf("CancelOperation: %v", err)
		}
		if event, ok := lifecycleEventByAction(histories.items, operationEventCanceled); !ok || event.Status != string(library.OperationStatusQueued) {
			t.Fatalf("cancel without primary History did not append an event: %#v", histories.items)
		}
	})

	t.Run("resume", func(t *testing.T) {
		operation := mustLifecycleOperation(t, lifecycleOperationParams{
			ID: "op-transcode-resume", LibraryID: "lib-transcode", Kind: "transcode",
			Status: library.OperationStatusCanceled, DisplayName: "Canceled transcode", InputJSON: "{}", CreatedAt: createdAt,
		})
		histories := &deleteRuleHistoryRepo{items: map[string]library.HistoryRecord{}}
		service := &LibraryService{
			operations:   &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}},
			histories:    histories,
			nowFunc:      func() time.Time { return eventAt },
			shuttingDown: true,
		}
		if _, err := service.ResumeOperation(ctx, dto.ResumeOperationRequest{OperationID: operation.ID}); err != nil {
			t.Fatalf("ResumeOperation: %v", err)
		}
		if event, ok := lifecycleEventByAction(histories.items, operationEventResumed); !ok || event.Status != string(library.OperationStatusCanceled) {
			t.Fatalf("resume without primary History did not append an event: %#v", histories.items)
		}
	})
}

type lifecycleOperationParams struct {
	ID           string
	LibraryID    string
	Kind         string
	Status       library.OperationStatus
	DisplayName  string
	InputJSON    string
	OutputJSON   string
	OutputFiles  []library.OperationOutputFile
	Metrics      library.OperationMetrics
	ErrorCode    string
	ErrorMessage string
	CreatedAt    time.Time
}

func mustLifecycleOperation(t *testing.T, params lifecycleOperationParams) library.LibraryOperation {
	t.Helper()
	item, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID: params.ID, LibraryID: params.LibraryID, Kind: params.Kind, Status: string(params.Status),
		DisplayName: params.DisplayName, InputJSON: params.InputJSON, OutputJSON: params.OutputJSON,
		OutputFiles: params.OutputFiles, Metrics: params.Metrics, ErrorCode: params.ErrorCode,
		ErrorMessage: params.ErrorMessage, CreatedAt: &params.CreatedAt,
	})
	if err != nil {
		t.Fatalf("build lifecycle operation: %v", err)
	}
	return item
}

func mustLifecycleHistory(
	t *testing.T,
	id string,
	operation library.LibraryOperation,
	category string,
	action string,
	occurredAt time.Time,
) library.HistoryRecord {
	t.Helper()
	item, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: id, LibraryID: operation.LibraryID, Category: category, Action: action,
		DisplayName: operation.DisplayName, Status: string(operation.Status),
		Refs:  library.HistoryRecordRefs{OperationID: operation.ID, SubjectOperationID: operation.ID},
		Files: snapshotOperationOutputFiles(operation.OutputFiles), Metrics: snapshotOperationMetrics(operation.Metrics),
		OccurredAt: &occurredAt, CreatedAt: &occurredAt, UpdatedAt: &occurredAt,
	})
	if err != nil {
		t.Fatalf("build lifecycle History: %v", err)
	}
	return item
}

func lifecycleEventByAction(items map[string]library.HistoryRecord, action string) (library.HistoryRecord, bool) {
	for _, item := range items {
		if item.Category == operationEventHistoryCategory && item.Action == action {
			return item, true
		}
	}
	return library.HistoryRecord{}, false
}
