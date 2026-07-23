package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/persistence"
)

func TestDeleteOperationOutputRejectsNonSucceededOperations(t *testing.T) {
	t.Parallel()

	for _, status := range []library.OperationStatus{
		library.OperationStatusQueued,
		library.OperationStatusRunning,
		library.OperationStatusFailed,
		library.OperationStatusCanceled,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
			libraryItem := mustNewLibrary(t, "lib-1", now)
			fileItem := mustNewVideoFile(t, "file-1", libraryItem.ID, "op-1", filepath.Join(t.TempDir(), "output.mp4"), now)
			operation := mustNewOperationWithKind(t, "op-1", "download", libraryItem.ID, []library.OperationOutputFile{{FileID: fileItem.ID, Kind: "video"}}, now)
			operation.Status = status
			operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
			service := &LibraryService{
				libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
				files:      &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}},
				operations: operations,
				nowFunc:    func() time.Time { return now },
			}

			_, err := service.DeleteOperationOutput(ctx, dto.DeleteOperationOutputRequest{
				OperationID: operation.ID,
				FileID:      fileItem.ID,
			})
			if err == nil || !strings.Contains(err.Error(), "does not support output deletion") {
				t.Fatalf("expected status rejection for %q, got %v", status, err)
			}
			stored, getErr := operations.Get(ctx, operation.ID)
			if getErr != nil {
				t.Fatalf("get operation: %v", getErr)
			}
			if len(stored.OutputFiles) != 1 || containsDetachedOperationOutputFileID(stored.OutputJSON, fileItem.ID) {
				t.Fatalf("rejected mutation changed operation: %#v %s", stored.OutputFiles, stored.OutputJSON)
			}
		})
	}
}

func TestDeleteOperationOutputSerializesConcurrentSiblingDeletes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 7, 20, 10, 30, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-1", now)
	first := mustNewVideoFile(t, "file-1", libraryItem.ID, "op-1", filepath.Join(t.TempDir(), "one.mp4"), now)
	second := mustNewVideoFile(t, "file-2", libraryItem.ID, "op-1", filepath.Join(t.TempDir(), "two.mp4"), now)
	operation := mustNewOperationWithKind(t, "op-1", "download", libraryItem.ID, []library.OperationOutputFile{
		{FileID: first.ID, Kind: "video", IsPrimary: true},
		{FileID: second.ID, Kind: "video"},
	}, now)
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	service := &LibraryService{
		libraries: &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files: &deleteRuleFileRepo{items: map[string]library.LibraryFile{
			first.ID:  first,
			second.ID: second,
		}},
		operations: operations,
		nowFunc:    func() time.Time { return now },
	}

	start := make(chan struct{})
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for _, fileID := range []string{first.ID, second.ID} {
		fileID := fileID
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := service.DeleteOperationOutput(ctx, dto.DeleteOperationOutputRequest{
				OperationID: operation.ID,
				FileID:      fileID,
			})
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent DeleteOperationOutput: %v", err)
		}
	}

	stored, err := operations.Get(ctx, operation.ID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if len(stored.OutputFiles) != 0 {
		t.Fatalf("concurrent deletion resurrected an output: %#v", stored.OutputFiles)
	}
	for _, fileID := range []string{first.ID, second.ID} {
		if !containsDetachedOperationOutputFileID(stored.OutputJSON, fileID) {
			t.Fatalf("missing detached marker for %q in %s", fileID, stored.OutputJSON)
		}
	}
}

func TestDeleteOperationOutputPersistsDetachedAssociationWithSQLite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "operation-output-delete.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = database.Close() }()

	now := time.Date(2026, 7, 20, 11, 30, 0, 0, time.UTC)
	localPath := filepath.Join(t.TempDir(), "durable.mp4")
	if err := os.WriteFile(localPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write output: %v", err)
	}
	libraries := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	files := libraryrepo.NewSQLiteFileRepository(database.Bun)
	operations := libraryrepo.NewSQLiteOperationRepository(database.Bun)
	histories := libraryrepo.NewSQLiteHistoryRepository(database.Bun)
	fileEvents := libraryrepo.NewSQLiteFileEventRepository(database.Bun)
	libraryItem := mustNewLibrary(t, "lib-sqlite", now)
	fileItem := mustNewVideoFile(t, "file-sqlite", libraryItem.ID, "op-sqlite", localPath, now)
	operation := mustNewOperationWithKind(t, "op-sqlite", "download", libraryItem.ID, []library.OperationOutputFile{{FileID: fileItem.ID, Kind: "video"}}, now)
	history := mustNewHistoryForOperation(t, "history-sqlite", libraryItem.ID, operation.ID, operation.Kind, operation.OutputFiles, now)
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
		fileEvents: fileEvents,
		nowFunc:    func() time.Time { return now.Add(time.Minute) },
	}

	if _, err := service.DeleteOperationOutput(ctx, dto.DeleteOperationOutputRequest{
		OperationID: operation.ID,
		FileID:      fileItem.ID,
	}); err != nil {
		t.Fatalf("DeleteOperationOutput: %v", err)
	}
	storedOperation, err := operations.Get(ctx, operation.ID)
	if err != nil {
		t.Fatalf("reload operation: %v", err)
	}
	if len(storedOperation.OutputFiles) != 0 || !containsDetachedOperationOutputFileID(storedOperation.OutputJSON, fileItem.ID) {
		t.Fatalf("detached association was not persisted: %#v %s", storedOperation.OutputFiles, storedOperation.OutputJSON)
	}
	storedFile, err := files.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("reload file: %v", err)
	}
	if storedFile.State.Deleted {
		t.Fatal("SQLite Library file must remain active after record-only detach")
	}
	storedHistory, err := histories.Get(ctx, history.ID)
	if err != nil {
		t.Fatalf("reload history: %v", err)
	}
	if len(storedHistory.Files) != 1 || storedHistory.Files[0].FileID != fileItem.ID {
		t.Fatalf("immutable history output snapshot was changed: %#v", storedHistory.Files)
	}
	eventDTOs, err := service.ListFileEvents(ctx, dto.ListFileEventsRequest{LibraryID: libraryItem.ID})
	if err != nil {
		t.Fatalf("ListFileEvents: %v", err)
	}
	if len(eventDTOs) != 1 {
		t.Fatalf("expected one durable detach event, got %#v", eventDTOs)
	}
	eventDTO := eventDTOs[0]
	if eventDTO.EventType != libraryFileEventOperationOutputDetach || eventDTO.OperationID != operation.ID ||
		eventDTO.OccurredAt != now.Add(time.Minute).Format(time.RFC3339) || eventDTO.CreatedAt != eventDTO.OccurredAt ||
		eventDTO.Detail.DeleteFile || eventDTO.Detail.Cause.Category != "task_output" ||
		eventDTO.Detail.Cause.Actor != libraryFileEventActorDesktop || eventDTO.Detail.Before == nil ||
		eventDTO.Detail.After == nil || len(eventDTO.Detail.Changes) != 1 ||
		eventDTO.Detail.Changes[0] != (dto.FileFieldChangeDTO{Field: "taskAssociation", Before: "attached", After: "detached"}) {
		t.Fatalf("unexpected durable detach event: %#v", eventDTO)
	}
}

func TestDeleteOperationOutputRecordsPurgedFileSubjectWithSQLite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "operation-output-missing-file.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = database.Close() }()

	now := time.Date(2026, 7, 20, 11, 45, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-missing-output", now)
	libraries := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	if err := libraries.Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}

	// Keep the operation foreign-key subject in SQLite, but use the in-memory
	// repository to reproduce an old/repairable Task whose output reference
	// survived after its LibraryFile row was already purged.
	shadowOperation := mustNewOperationWithKind(
		t,
		"op-missing-output",
		"download",
		libraryItem.ID,
		nil,
		now,
	)
	if err := libraryrepo.NewSQLiteOperationRepository(database.Bun).Save(ctx, shadowOperation); err != nil {
		t.Fatalf("save operation subject: %v", err)
	}
	operation := shadowOperation
	operation.OutputFiles = []library.OperationOutputFile{{FileID: "file-already-purged", Kind: "image"}}
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{
		operation.ID: operation,
	}}
	service := &LibraryService{
		libraries:  libraries,
		files:      &deleteRuleFileRepo{items: map[string]library.LibraryFile{}},
		operations: operations,
		fileEvents: libraryrepo.NewSQLiteFileEventRepository(database.Bun),
		nowFunc:    func() time.Time { return now.Add(time.Minute) },
	}

	if _, err := service.DeleteOperationOutput(ctx, dto.DeleteOperationOutputRequest{
		OperationID: operation.ID,
		FileID:      "file-already-purged",
	}); err != nil {
		t.Fatalf("DeleteOperationOutput: %v", err)
	}
	stored, err := operations.Get(ctx, operation.ID)
	if err != nil {
		t.Fatalf("reload operation: %v", err)
	}
	if len(stored.OutputFiles) != 0 ||
		!containsDetachedOperationOutputFileID(stored.OutputJSON, "file-already-purged") {
		t.Fatalf("missing-file output was not durably detached: %#v %s", stored.OutputFiles, stored.OutputJSON)
	}
	events, err := service.ListFileEvents(ctx, dto.ListFileEventsRequest{LibraryID: libraryItem.ID})
	if err != nil {
		t.Fatalf("ListFileEvents: %v", err)
	}
	if len(events) != 1 || events[0].FileID != "file-already-purged" ||
		events[0].EventType != libraryFileEventOperationOutputDetach {
		t.Fatalf("missing-file detach event was not preserved: %#v", events)
	}
}

func TestDeleteOperationOutputDetachesRecordWithoutDeletingLibraryFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.mp4")
	keptPath := filepath.Join(tempDir, "kept.mp4")
	for _, path := range []string{targetPath, keptPath} {
		if err := os.WriteFile(path, []byte("video"), 0o644); err != nil {
			t.Fatalf("write output: %v", err)
		}
	}
	libraryItem := mustNewLibrary(t, "lib-1", now)
	target := mustNewVideoFile(t, "file-target", libraryItem.ID, "op-1", targetPath, now)
	kept := mustNewVideoFile(t, "file-kept", libraryItem.ID, "op-1", keptPath, now)
	outputs := []library.OperationOutputFile{
		{FileID: target.ID, Kind: "video", IsPrimary: true},
		{FileID: kept.ID, Kind: "video"},
	}
	operation := mustNewOperationWithKind(t, "op-1", "download", libraryItem.ID, outputs, now)
	operation.OutputJSON = `{"mainPath":"` + targetPath + `","outputPaths":["` + targetPath + `","` + keptPath + `"],"outputFiles":[{"fileId":"file-target"},{"fileId":"file-kept"}]}`
	history := mustNewHistoryForOperation(t, "history-1", libraryItem.ID, operation.ID, operation.Kind, outputs, now)

	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{target.ID: target, kept.ID: kept}}
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	histories := &deleteRuleHistoryRepo{items: map[string]library.HistoryRecord{history.ID: history}}
	fileEvents := &deleteRuleFileEventRepo{}
	service := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:      files,
		operations: operations,
		histories:  histories,
		fileEvents: fileEvents,
		nowFunc:    func() time.Time { return now.Add(time.Minute) },
	}

	result, err := service.DeleteOperationOutput(ctx, dto.DeleteOperationOutputRequest{
		OperationID: operation.ID,
		FileID:      target.ID,
	})
	if err != nil {
		t.Fatalf("DeleteOperationOutput: %v", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("detached output must remain on disk: %v", err)
	}
	storedFile, err := files.Get(ctx, target.ID)
	if err != nil {
		t.Fatalf("get detached Library file: %v", err)
	}
	if storedFile.State.Deleted {
		t.Fatal("detaching a task record must not delete the Library file record")
	}
	storedOperation, err := operations.Get(ctx, operation.ID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if len(storedOperation.OutputFiles) != 1 || storedOperation.OutputFiles[0].FileID != kept.ID {
		t.Fatalf("expected only the kept output, got %#v", storedOperation.OutputFiles)
	}
	if !containsDetachedOperationOutputFileID(storedOperation.OutputJSON, target.ID) {
		t.Fatalf("expected durable detached marker, got %s", storedOperation.OutputJSON)
	}
	if strings.Contains(storedOperation.OutputJSON, targetPath) || strings.Contains(storedOperation.OutputJSON, `"fileId":"file-target"`) {
		t.Fatalf("detached output remained in operation payload: %s", storedOperation.OutputJSON)
	}
	if !strings.Contains(storedOperation.OutputJSON, keptPath) {
		t.Fatalf("kept output disappeared from operation payload: %s", storedOperation.OutputJSON)
	}
	storedHistory := histories.items[history.ID]
	if len(storedHistory.Files) != 2 || storedHistory.Files[0].FileID != target.ID || storedHistory.Files[1].FileID != kept.ID {
		t.Fatalf("expected immutable history to preserve both original outputs, got %#v", storedHistory.Files)
	}
	if len(result.DetachedOutputFileIDs) != 1 || result.DetachedOutputFileIDs[0] != target.ID {
		t.Fatalf("unexpected response marker: %#v", result.DetachedOutputFileIDs)
	}
	if len(fileEvents.items) != 1 {
		t.Fatalf("expected one output-detached event, got %#v", fileEvents.items)
	}
	detail := dto.FileEventDetailDTO{}
	if err := json.Unmarshal([]byte(fileEvents.items[0].DetailJSON), &detail); err != nil {
		t.Fatalf("decode detach detail: %v", err)
	}
	if fileEvents.items[0].EventType != libraryFileEventOperationOutputDetach ||
		fileEvents.items[0].OperationID != operation.ID || detail.DeleteFile ||
		detail.Before == nil || detail.After == nil || detail.Before.LocalPath != targetPath ||
		detail.After.LocalPath != targetPath || len(detail.Changes) != 1 ||
		detail.Changes[0].Field != "taskAssociation" {
		t.Fatalf("unexpected record-only detach event: event=%#v detail=%#v", fileEvents.items[0], detail)
	}
}

func TestDeleteOperationOutputCanAlsoDeletePhysicalFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 7, 20, 12, 30, 0, 0, time.UTC)
	localPath := filepath.Join(t.TempDir(), "target.mp4")
	if err := os.WriteFile(localPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write output: %v", err)
	}
	libraryItem := mustNewLibrary(t, "lib-1", now)
	fileItem := mustNewVideoFile(t, "file-1", libraryItem.ID, "op-1", localPath, now)
	operation := mustNewOperationWithKind(t, "op-1", "download", libraryItem.ID, []library.OperationOutputFile{{FileID: fileItem.ID, Kind: "video"}}, now)
	fileEvents := &deleteRuleFileEventRepo{}
	service := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:      &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}},
		operations: &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}},
		histories:  &deleteRuleHistoryRepo{},
		fileEvents: fileEvents,
		subtitles:  &deleteRuleSubtitleRepo{},
		nowFunc:    func() time.Time { return now },
	}

	if _, err := service.DeleteOperationOutput(ctx, dto.DeleteOperationOutputRequest{
		OperationID: operation.ID,
		FileID:      fileItem.ID,
		DeleteFile:  true,
	}); err != nil {
		t.Fatalf("DeleteOperationOutput: %v", err)
	}
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Fatalf("expected physical file to be deleted, got %v", err)
	}
	stored, err := service.files.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("get file record: %v", err)
	}
	if !stored.State.Deleted {
		t.Fatal("expected the Library file record to enter deleted lifecycle")
	}
	if len(fileEvents.items) != 1 {
		t.Fatalf("expected one composite cascade event, got %#v", fileEvents.items)
	}
	detail := dto.FileEventDetailDTO{}
	if err := json.Unmarshal([]byte(fileEvents.items[0].DetailJSON), &detail); err != nil {
		t.Fatalf("decode cascade event: %v", err)
	}
	if !detail.DeleteFile || detail.Cause.Category != "task_output" || len(detail.Changes) != 3 {
		t.Fatalf("unexpected cascade event detail: %#v", detail)
	}
	wantFields := []string{"taskAssociation", "fileLifecycle", "localFile"}
	for index, field := range wantFields {
		if detail.Changes[index].Field != field {
			t.Fatalf("cascade change %d = %#v, want %q", index, detail.Changes[index], field)
		}
	}
}

func TestDeleteOperationOutputPropagatesPhysicalDeleteFailureBeforeDetach(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 7, 20, 13, 0, 0, 0, time.UTC)
	directoryPath := t.TempDir()
	libraryItem := mustNewLibrary(t, "lib-1", now)
	fileItem := mustNewVideoFile(t, "file-1", libraryItem.ID, "op-1", directoryPath, now)
	operation := mustNewOperationWithKind(t, "op-1", "download", libraryItem.ID, []library.OperationOutputFile{{FileID: fileItem.ID, Kind: "video"}}, now)
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	service := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:      &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}},
		operations: operations,
		histories:  &deleteRuleHistoryRepo{},
		subtitles:  &deleteRuleSubtitleRepo{},
		nowFunc:    func() time.Time { return now },
	}

	_, err := service.DeleteOperationOutput(ctx, dto.DeleteOperationOutputRequest{
		OperationID: operation.ID,
		FileID:      fileItem.ID,
		DeleteFile:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to delete directory") {
		t.Fatalf("expected physical delete error, got %v", err)
	}
	stored, getErr := operations.Get(ctx, operation.ID)
	if getErr != nil {
		t.Fatalf("get operation: %v", getErr)
	}
	if len(stored.OutputFiles) != 1 || containsDetachedOperationOutputFileID(stored.OutputJSON, fileItem.ID) {
		t.Fatalf("failed physical delete must leave task projection intact: %#v %s", stored.OutputFiles, stored.OutputJSON)
	}
}
