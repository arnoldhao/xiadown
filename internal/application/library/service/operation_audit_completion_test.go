package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type transcodeAuditOperationRepo struct {
	*deleteRuleOperationRepo
	once        sync.Once
	onFirstSave func()
}

func (repo *transcodeAuditOperationRepo) Save(ctx context.Context, item library.LibraryOperation) error {
	if err := repo.deleteRuleOperationRepo.Save(ctx, item); err != nil {
		return err
	}
	repo.once.Do(func() {
		if repo.onFirstSave != nil {
			repo.onFirstSave()
		}
	})
	return nil
}

func TestOperationOutputCreatorsAppendFileCreatedEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	createdAt := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
	eventAt := createdAt.Add(time.Minute)

	t.Run("download", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "cover.jpg")
		if err := os.WriteFile(outputPath, []byte("cover"), 0o600); err != nil {
			t.Fatalf("write download output: %v", err)
		}
		files := &deleteRuleFileRepo{}
		events := &deleteRuleFileEventRepo{}
		service := &LibraryService{
			files: files, fileEvents: events,
			nowFunc: func() time.Time { return eventAt },
		}
		fileItem, err := service.createDownloadedBinaryFile(
			ctx,
			library.LibraryOperation{ID: "download-operation", LibraryID: "download-library"},
			string(library.FileKindThumbnail),
			outputPath,
			"Downloaded cover",
			createdAt,
		)
		if err != nil {
			t.Fatalf("create downloaded output: %v", err)
		}
		assertFileCreatedEvent(t, events.items, fileItem, "download", "download-operation", eventAt)
	})

	t.Run("transcode", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "converted.mp4")
		if err := os.WriteFile(outputPath, []byte("transcoded"), 0o600); err != nil {
			t.Fatalf("write transcode output: %v", err)
		}
		tools := writeAuditFFprobe(t)
		files := &deleteRuleFileRepo{}
		events := &deleteRuleFileEventRepo{}
		service := &LibraryService{files: files, fileEvents: events, tools: tools}
		fileItem, err := service.registerManagedLocalOutputFile(ctx, registeredLocalOutputParams{
			LibraryID:     "transcode-library",
			RootFileID:    "source-file",
			Name:          "converted.mp4",
			DisplayName:   "Converted",
			Kind:          string(library.FileKindTranscode),
			OperationID:   "transcode-operation",
			OperationKind: "transcode",
			OutputPath:    outputPath,
			OccurredAt:    eventAt,
		})
		if err != nil {
			t.Fatalf("register transcode output: %v", err)
		}
		assertFileCreatedEvent(t, events.items, fileItem, "transcode", "transcode-operation", eventAt)
	})
}

func TestTranscodePrimaryHistoryReusesOneRecordAcrossLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	auditNow := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	inputPath := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(inputPath, []byte("source"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	sourceFile := mustNewTranscodeTestFile(t, library.LibraryFileParams{
		ID:          "source-file",
		LibraryID:   "transcode-library",
		Kind:        string(library.FileKindVideo),
		Name:        "source.mp4",
		DisplayName: "Source clip",
		Storage:     library.FileStorage{Mode: "local_path", LocalPath: inputPath},
		Origin:      library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{BatchID: "batch", ImportPath: inputPath}},
		State:       library.FileState{Status: "active"},
	})
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{sourceFile.ID: sourceFile}}
	histories := &deleteRuleHistoryRepo{items: map[string]library.HistoryRecord{}}
	baseOperations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{}}
	operations := &transcodeAuditOperationRepo{deleteRuleOperationRepo: baseOperations}
	service := &LibraryService{
		files: files, operations: operations, histories: histories,
		tools: writeAuditFFprobe(t), shuttingDown: false,
		nowFunc: func() time.Time { return auditNow },
	}
	operations.onFirstSave = service.BeginShutdown

	created, err := service.CreateTranscodeJob(ctx, dto.CreateTranscodeJobRequest{
		FileID: sourceFile.ID, Title: "Audit clip", Source: "test", RunID: "run-audit",
		Format: "mp4", VideoCodec: "h264", AudioCodec: "aac",
	})
	if err != nil {
		t.Fatalf("CreateTranscodeJob: %v", err)
	}
	primaryID := assertSingleTranscodePrimaryHistory(t, histories.items, created.ID, library.OperationStatusQueued)

	auditNow = auditNow.Add(time.Minute)
	operation, err := operations.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get queued operation: %v", err)
	}
	operation.Status = library.OperationStatusRunning
	operation.StartedAt = timePointer(auditNow)
	if err := operations.Save(ctx, operation); err != nil {
		t.Fatalf("save running operation: %v", err)
	}
	if err := service.syncPrimaryOperationHistory(ctx, operation, auditNow); err != nil {
		t.Fatalf("sync running history: %v", err)
	}
	assertPrimaryHistoryIDAndStatus(t, histories.items, created.ID, primaryID, library.OperationStatusRunning)

	auditNow = auditNow.Add(time.Minute)
	service.failTranscodeOperation(ctx, operation, dto.CreateTranscodeJobRequest{}, errors.New("encoder failed"))
	failed := assertPrimaryHistoryIDAndStatus(t, histories.items, created.ID, primaryID, library.OperationStatusFailed)
	if failed.OperationMeta == nil || failed.OperationMeta.ErrorCode != "transcode_failed" || failed.OperationMeta.ErrorMessage != "encoder failed" {
		t.Fatalf("failed primary History lost terminal error: %#v", failed.OperationMeta)
	}

	auditNow = auditNow.Add(time.Minute)
	if _, err := service.ResumeOperation(ctx, dto.ResumeOperationRequest{OperationID: created.ID}); err != nil {
		t.Fatalf("resume failed transcode: %v", err)
	}
	assertPrimaryHistoryIDAndStatus(t, histories.items, created.ID, primaryID, library.OperationStatusQueued)

	auditNow = auditNow.Add(time.Minute)
	if _, err := service.CancelOperation(ctx, dto.CancelOperationRequest{OperationID: created.ID}); err != nil {
		t.Fatalf("cancel queued transcode: %v", err)
	}
	assertPrimaryHistoryIDAndStatus(t, histories.items, created.ID, primaryID, library.OperationStatusCanceled)

	auditNow = auditNow.Add(time.Minute)
	if _, err := service.ResumeOperation(ctx, dto.ResumeOperationRequest{OperationID: created.ID}); err != nil {
		t.Fatalf("resume canceled transcode: %v", err)
	}
	operation, err = operations.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get resumed operation: %v", err)
	}
	operation.Status = library.OperationStatusRunning
	operation.StartedAt = timePointer(auditNow)
	if err := operations.Save(ctx, operation); err != nil {
		t.Fatalf("save second running attempt: %v", err)
	}
	if err := service.syncPrimaryOperationHistory(ctx, operation, auditNow); err != nil {
		t.Fatalf("sync second running attempt: %v", err)
	}

	auditNow = auditNow.Add(time.Minute)
	service.markInterruptedOperation(ctx, operation)
	interrupted := assertPrimaryHistoryIDAndStatus(t, histories.items, created.ID, primaryID, library.OperationStatusFailed)
	if interrupted.OperationMeta == nil || interrupted.OperationMeta.ErrorCode != operationErrorCodeAppInterrupted {
		t.Fatalf("interrupted primary History lost error metadata: %#v", interrupted.OperationMeta)
	}

	auditNow = auditNow.Add(time.Minute)
	if _, err := service.ResumeOperation(ctx, dto.ResumeOperationRequest{OperationID: created.ID}); err != nil {
		t.Fatalf("resume interrupted transcode: %v", err)
	}
	operation, err = operations.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get final resumed operation: %v", err)
	}
	auditNow = auditNow.Add(time.Minute)
	operation.Status = library.OperationStatusSucceeded
	operation.ErrorCode = ""
	operation.ErrorMessage = ""
	operation.OutputFiles = []library.OperationOutputFile{{FileID: "output-file", Kind: "transcode", IsPrimary: true}}
	operation.Metrics = library.OperationMetrics{FileCount: 1}
	operation.FinishedAt = timePointer(auditNow)
	if err := operations.Save(ctx, operation); err != nil {
		t.Fatalf("save succeeded operation: %v", err)
	}
	if err := service.syncPrimaryOperationHistory(ctx, operation, auditNow); err != nil {
		t.Fatalf("sync succeeded history: %v", err)
	}
	succeeded := assertPrimaryHistoryIDAndStatus(t, histories.items, created.ID, primaryID, library.OperationStatusSucceeded)
	if len(succeeded.Files) != 1 || succeeded.Files[0].FileID != "output-file" || succeeded.Metrics.FileCount != 1 {
		t.Fatalf("succeeded primary History lost output snapshot: %#v", succeeded)
	}
}

func writeAuditFFprobe(t *testing.T) *mediaProbeToolResolverStub {
	t.Helper()
	toolDir := t.TempDir()
	writeFFprobeTestFixture(t, toolDir, `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1280,"height":720},{"index":1,"codec_type":"audio","codec_name":"aac","channels":2}],"format":{"format_name":"mov,mp4","duration":"12.5","size":"1234"}}`)
	return &mediaProbeToolResolverStub{ready: true, toolDir: toolDir}
}

func assertFileCreatedEvent(
	t *testing.T,
	events []library.FileEventRecord,
	fileItem library.LibraryFile,
	category string,
	operationID string,
	occurredAt time.Time,
) {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("expected one file_created event, got %#v", events)
	}
	event := events[0]
	if event.EventType != libraryFileEventCreated || event.FileID != fileItem.ID || event.OperationID != operationID || !event.CreatedAt.Equal(occurredAt) {
		t.Fatalf("unexpected file_created envelope: %#v", event)
	}
	detail := dto.FileEventDetailDTO{}
	if err := json.Unmarshal([]byte(event.DetailJSON), &detail); err != nil {
		t.Fatalf("decode file_created detail: %v", err)
	}
	if detail.Cause.Category != category || detail.Cause.OperationID != operationID || detail.Before != nil || detail.After == nil ||
		detail.After.FileID != fileItem.ID || detail.After.Kind != string(fileItem.Kind) || detail.After.LocalPath != fileItem.Storage.LocalPath {
		t.Fatalf("unexpected file_created detail: %#v", detail)
	}
	if len(detail.Changes) != 1 || detail.Changes[0].Field != "fileLifecycle" || detail.Changes[0].Before != "absent" || detail.Changes[0].After != "active" {
		t.Fatalf("unexpected creation transition: %#v", detail.Changes)
	}
}

func assertSingleTranscodePrimaryHistory(
	t *testing.T,
	items map[string]library.HistoryRecord,
	operationID string,
	status library.OperationStatus,
) string {
	t.Helper()
	item := assertPrimaryHistoryIDAndStatus(t, items, operationID, "", status)
	return item.ID
}

func assertPrimaryHistoryIDAndStatus(
	t *testing.T,
	items map[string]library.HistoryRecord,
	operationID string,
	expectedID string,
	status library.OperationStatus,
) library.HistoryRecord {
	t.Helper()
	primary := make([]library.HistoryRecord, 0, 1)
	for _, item := range items {
		if item.Category == operationHistoryCategory &&
			(item.Refs.OperationID == operationID || item.Refs.SubjectOperationID == operationID) {
			primary = append(primary, item)
		}
	}
	if len(primary) != 1 {
		t.Fatalf("expected exactly one transcode primary History, got %#v", primary)
	}
	item := primary[0]
	if expectedID != "" && item.ID != expectedID {
		t.Fatalf("primary History identity changed: got %q, want %q", item.ID, expectedID)
	}
	if item.Status != string(status) {
		t.Fatalf("primary History status = %q, want %q", item.Status, status)
	}
	return item
}

func timePointer(value time.Time) *time.Time {
	return &value
}
