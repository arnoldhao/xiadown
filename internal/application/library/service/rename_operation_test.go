package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	librarydto "xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestRenameOperationUpdatesOperationHistoryAndLibraryTimestamp(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	renamedAt := createdAt.Add(2 * time.Hour)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID:        "lib-1",
		Name:      "Library",
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	outputSize := int64(4096)
	outputFiles := []library.OperationOutputFile{{
		FileID:    "file-1",
		Kind:      "image",
		Format:    "webp",
		SizeBytes: &outputSize,
		IsPrimary: true,
	}}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          "op-1",
		LibraryID:   libraryItem.ID,
		Kind:        "download",
		Status:      string(library.OperationStatusSucceeded),
		DisplayName: "Old title",
		OutputFiles: outputFiles,
		Metrics: library.OperationMetrics{
			FileCount:      1,
			TotalSizeBytes: &outputSize,
		},
		CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	history, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          "history-1",
		LibraryID:   libraryItem.ID,
		Category:    "operation",
		Action:      "download",
		DisplayName: "Old title",
		Status:      string(library.OperationStatusSucceeded),
		Refs:        library.HistoryRecordRefs{OperationID: operation.ID, SubjectOperationID: operation.ID},
		Files:       outputFiles,
		Metrics:     operation.Metrics,
		OccurredAt:  &createdAt,
		CreatedAt:   &createdAt,
		UpdatedAt:   &createdAt,
	})
	if err != nil {
		t.Fatalf("build history: %v", err)
	}

	libraries := &ytdlpMetadataLibraryRepo{item: libraryItem}
	operations := &ytdlpMetadataOperationRepo{saved: []library.LibraryOperation{operation}}
	histories := &ytdlpMetadataHistoryRepo{saved: []library.HistoryRecord{history}}
	service := &LibraryService{
		libraries:  libraries,
		operations: operations,
		histories:  histories,
		nowFunc: func() time.Time {
			return renamedAt
		},
	}

	result, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
		OperationID: " op-1 ",
		Name:        " New title ",
	})
	if err != nil {
		t.Fatalf("rename operation: %v", err)
	}
	if result.DisplayName != "New title" {
		t.Fatalf("expected renamed dto, got %q", result.DisplayName)
	}
	if got := operations.saved[len(operations.saved)-1].DisplayName; got != "New title" {
		t.Fatalf("expected operation display name to update, got %q", got)
	}
	var savedHistory library.HistoryRecord
	var renameEvent library.HistoryRecord
	for _, item := range histories.saved {
		if item.ID == history.ID {
			savedHistory = item
		}
		if item.Category == operationEventHistoryCategory && item.Action == operationEventRenamed {
			renameEvent = item
		}
	}
	if savedHistory.DisplayName != "New title" {
		t.Fatalf("expected primary history display name to update, got %q", savedHistory.DisplayName)
	}
	if !savedHistory.UpdatedAt.Equal(renamedAt) {
		t.Fatalf("expected history timestamp %s, got %s", renamedAt, savedHistory.UpdatedAt)
	}
	if renameEvent.ID == "" || renameEvent.DisplayName != "Old title" ||
		renameEvent.Status != string(library.OperationStatusSucceeded) ||
		renameEvent.Source.Kind != "user_action" || renameEvent.Source.Actor != libraryFileEventActorDesktop ||
		renameEvent.Refs.OperationID != operation.ID || renameEvent.Refs.SubjectOperationID != operation.ID ||
		len(renameEvent.Files) != 1 || renameEvent.Files[0].FileID != "file-1" ||
		renameEvent.Metrics.FileCount != 1 || renameEvent.Metrics.TotalSizeBytes == nil ||
		*renameEvent.Metrics.TotalSizeBytes != outputSize || !renameEvent.OccurredAt.Equal(renamedAt) {
		t.Fatalf("expected immutable pre-rename operation event, got %#v", renameEvent)
	}
	if !libraries.item.UpdatedAt.Equal(renamedAt) {
		t.Fatalf("expected library timestamp %s, got %s", renamedAt, libraries.item.UpdatedAt)
	}
}

func TestRenameFileUpdatesDisplayNameAndLibraryTimestamp(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	renamedAt := createdAt.Add(2 * time.Hour)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID:        "lib-1",
		Name:      "Library",
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:          "file-1",
		LibraryID:   libraryItem.ID,
		Kind:        string(library.FileKindVideo),
		Name:        "old.mp4",
		DisplayName: "Old file",
		Storage: library.FileStorage{
			Mode:      "local_path",
			LocalPath: "/tmp/old.mp4",
		},
		Origin: library.FileOrigin{
			Kind:        "download",
			OperationID: "op-1",
		},
		State:     library.FileState{Status: "active"},
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	libraries := &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}}
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	fileEvents := &deleteRuleFileEventRepo{}
	service := &LibraryService{
		libraries:  libraries,
		files:      files,
		fileEvents: fileEvents,
		nowFunc: func() time.Time {
			return renamedAt
		},
	}

	result, err := service.RenameFile(context.Background(), librarydto.RenameFileRequest{
		FileID: " file-1 ",
		Name:   " New file ",
	})
	if err != nil {
		t.Fatalf("rename file: %v", err)
	}
	if result.DisplayName != "New file" {
		t.Fatalf("expected renamed file dto, got %q", result.DisplayName)
	}
	if result.Name != "old.mp4" || result.FileName != "old.mp4" {
		t.Fatalf("expected stored and physical file names to remain, got name=%q fileName=%q", result.Name, result.FileName)
	}
	savedFile := files.savedItems[len(files.savedItems)-1]
	if savedFile.DisplayName != "New file" {
		t.Fatalf("expected file display name to update, got %q", savedFile.DisplayName)
	}
	if !savedFile.UpdatedAt.Equal(renamedAt) {
		t.Fatalf("expected file timestamp %s, got %s", renamedAt, savedFile.UpdatedAt)
	}
	if !libraries.items[libraryItem.ID].UpdatedAt.Equal(renamedAt) {
		t.Fatalf("expected library timestamp %s, got %s", renamedAt, libraries.items[libraryItem.ID].UpdatedAt)
	}
	if len(fileEvents.items) != 1 || fileEvents.items[0].EventType != libraryFileEventRenamed ||
		fileEvents.items[0].OperationID != "op-1" {
		t.Fatalf("expected one rename event, got %#v", fileEvents.items)
	}
	detail := librarydto.FileEventDetailDTO{}
	if err := json.Unmarshal([]byte(fileEvents.items[0].DetailJSON), &detail); err != nil {
		t.Fatalf("decode rename event: %v", err)
	}
	if detail.Before == nil || detail.After == nil || detail.Before.Name != "Old file" || detail.After.Name != "New file" ||
		len(detail.Changes) != 1 || detail.Changes[0] != (librarydto.FileFieldChangeDTO{Field: "displayName", Before: "Old file", After: "New file"}) {
		t.Fatalf("unexpected rename event detail: %#v", detail)
	}
}

func TestNormalizeLibraryDisplayNameValidation(t *testing.T) {
	t.Parallel()

	if got, err := normalizeLibraryDisplayName("  新名称  "); err != nil || got != "新名称" {
		t.Fatalf("expected unicode display name to pass, got name=%q err=%v", got, err)
	}

	invalidNames := []string{
		"",
		"bad/name",
		"bad\\name",
		"bad:name",
		"line\nbreak",
		".",
		"..",
		"CON",
		"con.txt",
		"LPT9.log",
		"trailing.",
		strings.Repeat("界", libraryDisplayNameMaxRunes+1),
	}
	for _, name := range invalidNames {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeLibraryDisplayName(name); err == nil {
				t.Fatalf("expected %q to be rejected", name)
			}
		})
	}
}

func TestRenameCommandsRejectUnsafeNamesBeforeMutation(t *testing.T) {
	t.Parallel()

	operations := &ytdlpMetadataOperationRepo{}
	files := &deleteRuleFileRepo{}
	service := &LibraryService{operations: operations, files: files}

	if _, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
		OperationID: "op-1",
		Name:        "../outside",
	}); err == nil {
		t.Fatal("expected task traversal-like name to be rejected")
	}
	if len(operations.saved) != 0 {
		t.Fatalf("invalid task rename mutated repository: %#v", operations.saved)
	}

	if _, err := service.RenameFile(context.Background(), librarydto.RenameFileRequest{
		FileID: "file-1",
		Name:   `folder\outside.mp4`,
	}); err == nil {
		t.Fatal("expected file traversal-like name to be rejected")
	}
	if len(files.savedItems) != 0 {
		t.Fatalf("invalid file rename mutated repository: %#v", files.savedItems)
	}
}

func TestRenameNoOpDoesNotAppendDuplicateActivity(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID: "op-noop", LibraryID: "lib-noop", Kind: "download",
		Status: string(library.OperationStatusSucceeded), DisplayName: "Current task", CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-noop", LibraryID: "lib-noop", Kind: string(library.FileKindVideo),
		Name: "current.mp4", DisplayName: "Current file",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: "/tmp/current.mp4"},
		Origin:  library.FileOrigin{Kind: "download", OperationID: operation.ID},
		State:   library.FileState{Status: "active"}, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	operations := &ytdlpMetadataOperationRepo{saved: []library.LibraryOperation{operation}}
	histories := &ytdlpMetadataHistoryRepo{}
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	fileEvents := &deleteRuleFileEventRepo{}
	service := &LibraryService{operations: operations, histories: histories, files: files, fileEvents: fileEvents}

	if _, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
		OperationID: operation.ID, Name: "  Current task  ",
	}); err != nil {
		t.Fatalf("no-op task rename: %v", err)
	}
	if len(operations.saved) != 1 || len(histories.saved) != 0 {
		t.Fatalf("no-op task rename wrote state or activity: operations=%#v histories=%#v", operations.saved, histories.saved)
	}

	if _, err := service.RenameFile(context.Background(), librarydto.RenameFileRequest{
		FileID: fileItem.ID, Name: "  Current file  ",
	}); err != nil {
		t.Fatalf("no-op file rename: %v", err)
	}
	if len(files.savedItems) != 0 || len(fileEvents.items) != 0 {
		t.Fatalf("no-op file rename wrote state or activity: files=%#v events=%#v", files.savedItems, fileEvents.items)
	}
}

func TestRenameFileRollsBackWhenActivityWriteFails(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-file-rollback", Name: "Library", CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-rollback", LibraryID: libraryItem.ID, Kind: string(library.FileKindVideo),
		Name: "original.mp4", DisplayName: "Original file",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: "/tmp/original.mp4"},
		Origin:  library.FileOrigin{Kind: "download", OperationID: "op-rollback"},
		State:   library.FileState{Status: "active"}, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	libraries := &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}}
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	eventErr := errors.New("activity unavailable")
	service := &LibraryService{
		libraries:  libraries,
		files:      files,
		fileEvents: &renameFailingFileEventRepo{err: eventErr},
	}

	if _, err := service.RenameFile(context.Background(), librarydto.RenameFileRequest{
		FileID: fileItem.ID, Name: "Renamed file",
	}); !errors.Is(err, eventErr) {
		t.Fatalf("rename error = %v, want activity failure", err)
	}
	retained := files.items[fileItem.ID]
	if retained.DisplayName != fileItem.DisplayName || retained.UpdatedAt != fileItem.UpdatedAt {
		t.Fatalf("file projection escaped failed rename: before=%#v after=%#v", fileItem, retained)
	}
	if !libraries.items[libraryItem.ID].UpdatedAt.Equal(createdAt) {
		t.Fatalf("failed rename touched library timestamp: %s", libraries.items[libraryItem.ID].UpdatedAt)
	}
}

func TestRenameOperationRollsBackWhenActivityWriteFails(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-operation-rollback", Name: "Library", CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID: "op-rollback", LibraryID: libraryItem.ID, Kind: "download",
		Status: string(library.OperationStatusSucceeded), DisplayName: "Original task", CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	primaryHistory, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: "history-rollback", LibraryID: libraryItem.ID, Category: "operation", Action: "download",
		DisplayName: operation.DisplayName, Status: string(operation.Status),
		Refs:       library.HistoryRecordRefs{OperationID: operation.ID, SubjectOperationID: operation.ID},
		OccurredAt: &createdAt, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build history: %v", err)
	}

	libraries := &ytdlpMetadataLibraryRepo{item: libraryItem}
	operations := &ytdlpMetadataOperationRepo{saved: []library.LibraryOperation{operation}}
	storedHistories := &ytdlpMetadataHistoryRepo{saved: []library.HistoryRecord{primaryHistory}}
	eventErr := errors.New("activity unavailable")
	histories := &renameEventFailingHistoryRepo{ytdlpMetadataHistoryRepo: storedHistories, err: eventErr}
	service := &LibraryService{libraries: libraries, operations: operations, histories: histories}

	if _, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
		OperationID: operation.ID, Name: "Renamed task",
	}); !errors.Is(err, eventErr) {
		t.Fatalf("rename error = %v, want activity failure", err)
	}
	if retained := operations.saved[len(operations.saved)-1]; retained.DisplayName != operation.DisplayName {
		t.Fatalf("operation projection escaped failed rename: %#v", retained)
	}
	var retainedPrimary library.HistoryRecord
	for _, item := range storedHistories.saved {
		if item.ID == primaryHistory.ID {
			retainedPrimary = item
		}
	}
	if retainedPrimary.DisplayName != primaryHistory.DisplayName {
		t.Fatalf("primary history escaped failed rename: %#v", retainedPrimary)
	}
	if !libraries.item.UpdatedAt.Equal(createdAt) {
		t.Fatalf("failed rename touched library timestamp: %s", libraries.item.UpdatedAt)
	}
}

func TestRenameOperationHistoryLookupFailureHasNoMutationOrActivity(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID: "op-history-read-failure", LibraryID: "lib-history-read-failure", Kind: "download",
		Status: string(library.OperationStatusSucceeded), DisplayName: "Original task", CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	operations := &ytdlpMetadataOperationRepo{saved: []library.LibraryOperation{operation}}
	listErr := errors.New("history list unavailable")
	histories := &renameListFailingHistoryRepo{
		ytdlpMetadataHistoryRepo: &ytdlpMetadataHistoryRepo{},
		err:                      listErr,
	}
	service := &LibraryService{operations: operations, histories: histories}

	if _, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
		OperationID: operation.ID, Name: "Renamed task",
	}); !errors.Is(err, listErr) {
		t.Fatalf("rename error = %v, want history lookup failure", err)
	}
	if len(operations.saved) != 1 || operations.saved[0].DisplayName != operation.DisplayName {
		t.Fatalf("history lookup failure mutated operation: %#v", operations.saved)
	}
	if histories.saveCalls != 0 || len(histories.saved) != 0 {
		t.Fatalf("history lookup failure wrote history/activity: calls=%d items=%#v", histories.saveCalls, histories.saved)
	}
	if err := service.syncPrimaryOperationHistory(context.Background(), operation, createdAt); !errors.Is(err, listErr) {
		t.Fatalf("primary history sync error = %v, want lookup failure", err)
	}
}

func TestRenameOperationReturnsCommittedResultWhenLibraryTouchFails(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-operation-derived-failure", Name: "Library", CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID: "op-derived-failure", LibraryID: libraryItem.ID, Kind: "download",
		Status: string(library.OperationStatusSucceeded), DisplayName: "Original task", CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	operations := &ytdlpMetadataOperationRepo{saved: []library.LibraryOperation{operation}}
	histories := &ytdlpMetadataHistoryRepo{}
	touchErr := errors.New("library touch unavailable")
	libraries := &renameFailingTouchLibraryRepo{
		ytdlpMetadataLibraryRepo: &ytdlpMetadataLibraryRepo{item: libraryItem},
		err:                      touchErr,
	}
	service := &LibraryService{libraries: libraries, operations: operations, histories: histories}

	result, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
		OperationID: operation.ID, Name: "Renamed task",
	})
	if err != nil {
		t.Fatalf("committed rename returned derived error: %v", err)
	}
	if result.DisplayName != "Renamed task" || operations.saved[len(operations.saved)-1].DisplayName != "Renamed task" {
		t.Fatalf("rename did not return committed projection: result=%#v operations=%#v", result, operations.saved)
	}
	if libraries.saveCalls != 1 {
		t.Fatalf("expected one best-effort touch, got %d", libraries.saveCalls)
	}
	if len(histories.saved) != 1 || histories.saved[0].Action != operationEventRenamed {
		t.Fatalf("committed rename omitted activity: %#v", histories.saved)
	}
}

func TestRenameFileReturnsCommittedFallbackWhenDerivedMaintenanceFails(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-file-derived-failure", Name: "Library", CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-derived-failure", LibraryID: libraryItem.ID, Kind: string(library.FileKindVideo),
		Name: "original.mp4", DisplayName: "Original file",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: "/tmp/original.mp4"},
		Origin:  library.FileOrigin{Kind: "download", OperationID: "op-derived-failure"},
		State:   library.FileState{Status: "active"}, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	fileEvents := &deleteRuleFileEventRepo{}
	libraries := &renameFailingTouchLibraryRepo{
		ytdlpMetadataLibraryRepo: &ytdlpMetadataLibraryRepo{item: libraryItem},
		err:                      errors.New("library touch unavailable"),
	}
	projection := &renameFailingCatalogProjection{err: errors.New("catalog unavailable")}
	service := &LibraryService{
		libraries:         libraries,
		moduleConfig:      &renameFailingModuleConfigRepo{err: errors.New("module config unavailable")},
		files:             files,
		fileEvents:        fileEvents,
		catalogProjection: projection,
	}

	result, err := service.RenameFile(context.Background(), librarydto.RenameFileRequest{
		FileID: fileItem.ID, Name: "Renamed file",
	})
	if err != nil {
		t.Fatalf("committed file rename returned derived error: %v", err)
	}
	if result.DisplayName != "Renamed file" || result.Name != fileItem.Name || result.FileName != fileItem.Name {
		t.Fatalf("fallback DTO does not describe committed display rename: %#v", result)
	}
	if stored := files.items[fileItem.ID]; stored.DisplayName != "Renamed file" {
		t.Fatalf("file projection was not committed: %#v", stored)
	}
	if len(fileEvents.items) != 1 || fileEvents.items[0].EventType != libraryFileEventRenamed {
		t.Fatalf("committed file rename omitted activity: %#v", fileEvents.items)
	}
	if libraries.saveCalls != 1 || projection.calls != 3 {
		t.Fatalf("best-effort maintenance attempts = touch:%d projection:%d", libraries.saveCalls, projection.calls)
	}
}

func TestRenameUsesExistingAggregateMutationLocks(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "lib-rename-lock", Name: "Library", CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build library: %v", err)
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID: "op-rename-lock", LibraryID: libraryItem.ID, Kind: "download",
		Status: string(library.OperationStatusSucceeded), DisplayName: "Original task", CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build operation: %v", err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "file-rename-lock", LibraryID: libraryItem.ID, Kind: string(library.FileKindVideo),
		Name: "original.mp4", DisplayName: "Original file",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: "/tmp/original.mp4"},
		Origin:  library.FileOrigin{Kind: "download", OperationID: operation.ID},
		State:   library.FileState{Status: "active"}, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	operationGet := make(chan struct{})
	operationRepo := &renameObservedOperationRepo{
		ytdlpMetadataOperationRepo: &ytdlpMetadataOperationRepo{saved: []library.LibraryOperation{operation}},
		getStarted:                 operationGet,
	}
	service := &LibraryService{
		libraries:  &ytdlpMetadataLibraryRepo{item: libraryItem},
		operations: operationRepo,
		histories:  &ytdlpMetadataHistoryRepo{},
	}
	service.operationOutputMutationMu.Lock()
	operationDone := make(chan error, 1)
	go func() {
		_, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
			OperationID: operation.ID, Name: "Renamed task",
		})
		operationDone <- err
	}()
	assertRenameReadBlocked(t, operationGet, "operation")
	service.operationOutputMutationMu.Unlock()
	if err := <-operationDone; err != nil {
		t.Fatalf("rename operation after lock release: %v", err)
	}

	fileGet := make(chan struct{})
	fileRepo := &renameObservedFileRepo{
		deleteRuleFileRepo: &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}},
		getStarted:         fileGet,
	}
	fileService := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:      fileRepo,
		fileEvents: &deleteRuleFileEventRepo{},
	}
	unlockFile := fileService.lockListenLocalTrackMutation(fileItem.ID)
	fileDone := make(chan error, 1)
	go func() {
		_, err := fileService.RenameFile(context.Background(), librarydto.RenameFileRequest{
			FileID: fileItem.ID, Name: "Renamed file",
		})
		fileDone <- err
	}()
	assertRenameReadBlocked(t, fileGet, "file")
	unlockFile()
	if err := <-fileDone; err != nil {
		t.Fatalf("rename file after lock release: %v", err)
	}
}

func TestProgressReportersSerializeWithCompanionRename(t *testing.T) {
	testCases := []struct {
		name string
		run  func(*LibraryService, *library.LibraryOperation)
	}{
		{
			name: "ytdlp",
			run: func(service *LibraryService, operation *library.LibraryOperation) {
				reporter := newYTDLPProgressReporter(service, operation)
				reporter.persistProgress(nil, nil, nil, "1MiB/s", "1MiB/s")
			},
		},
		{
			name: "ffmpeg",
			run: func(service *LibraryService, operation *library.LibraryOperation) {
				reporter := newFFmpegProgressReporter(service, operation, 1000)
				reporter.currentMs = 500
				reporter.persistLocked(false)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)
			operation := library.LibraryOperation{
				ID: "op-reporter-race-" + testCase.name, LibraryID: "lib-1", Kind: "download",
				Status: library.OperationStatusRunning, DisplayName: "Original task", OutputJSON: "{}", CreatedAt: now,
			}
			baseRepo := &retryOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
			repo := &renameInterleavingOperationRepo{retryOperationRepo: baseRepo}
			repo.blockNextSave()
			service := &LibraryService{operations: repo, nowFunc: func() time.Time { return now }}
			staleOperation := operation

			reporterDone := make(chan struct{})
			go func() {
				testCase.run(service, &staleOperation)
				close(reporterDone)
			}()
			select {
			case <-repo.saveStarted:
			case <-time.After(time.Second):
				t.Fatal("reporter did not reach the controlled Save")
			}

			renameDone := make(chan error, 1)
			go func() {
				_, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
					OperationID: operation.ID, Name: "Companion title",
				})
				renameDone <- err
			}()
			select {
			case err := <-renameDone:
				t.Fatalf("rename escaped reporter mutation lock: %v", err)
			case <-time.After(30 * time.Millisecond):
			}

			close(repo.releaseSave)
			<-reporterDone
			if err := <-renameDone; err != nil {
				t.Fatalf("rename after reporter Save: %v", err)
			}
			stored, err := repo.Get(context.Background(), operation.ID)
			if err != nil {
				t.Fatalf("get stored operation: %v", err)
			}
			if stored.DisplayName != "Companion title" {
				t.Fatalf("stale reporter won controlled interleaving: %#v", stored)
			}
		})
	}
}

func TestTerminalOperationPersistenceKeepsCompanionRenameInOperationAndHistory(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	operation := library.LibraryOperation{
		ID: "op-terminal-rename", LibraryID: "lib-1", Kind: "download",
		Status: library.OperationStatusRunning, DisplayName: "Original task", OutputJSON: "{}", CreatedAt: now,
	}
	history, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: "history-terminal-rename", LibraryID: operation.LibraryID, Category: operationHistoryCategory,
		Action: operation.Kind, DisplayName: operation.DisplayName, Status: string(operation.Status),
		Refs:       library.HistoryRecordRefs{OperationID: operation.ID, SubjectOperationID: operation.ID},
		OccurredAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build primary history: %v", err)
	}
	operations := &retryOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	histories := &retryHistoryRepo{items: map[string]library.HistoryRecord{history.ID: history}}
	service := &LibraryService{operations: operations, histories: histories, nowFunc: func() time.Time { return now }}
	staleOperation := operation
	staleHistory := history

	if _, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
		OperationID: operation.ID, Name: "Companion title",
	}); err != nil {
		t.Fatalf("rename running operation: %v", err)
	}
	staleOperation.Status = library.OperationStatusSucceeded
	staleOperation.OutputJSON = `{"completed":true}`
	staleHistory.Status = string(staleOperation.Status)
	if err := service.persistOperationAndHistory(context.Background(), &staleOperation, &staleHistory); err != nil {
		t.Fatalf("persist terminal operation: %v", err)
	}

	storedOperation, err := operations.Get(context.Background(), operation.ID)
	if err != nil {
		t.Fatalf("get terminal operation: %v", err)
	}
	storedHistory, err := histories.Get(context.Background(), history.ID)
	if err != nil {
		t.Fatalf("get terminal history: %v", err)
	}
	if storedOperation.DisplayName != "Companion title" || storedHistory.DisplayName != "Companion title" {
		t.Fatalf("terminal save reverted renamed projections: operation=%#v history=%#v", storedOperation, storedHistory)
	}
	if storedOperation.Status != library.OperationStatusSucceeded || storedHistory.Status != string(library.OperationStatusSucceeded) {
		t.Fatalf("terminal fields were not persisted: operation=%#v history=%#v", storedOperation, storedHistory)
	}
}

func TestOperationPersistenceAllowsAutomaticTitleBeforeCompanionRename(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 11, 0, 0, 0, time.UTC)
	storedOperation := library.LibraryOperation{
		ID: "op-automatic-title", LibraryID: "lib-1", Kind: "download",
		Status: library.OperationStatusRunning, DisplayName: "https://example.com/video", OutputJSON: "{}", CreatedAt: now,
	}
	storedHistory, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: "history-automatic-title", LibraryID: storedOperation.LibraryID, Category: operationHistoryCategory,
		Action: storedOperation.Kind, DisplayName: storedOperation.DisplayName, Status: string(storedOperation.Status),
		Refs:       library.HistoryRecordRefs{OperationID: storedOperation.ID, SubjectOperationID: storedOperation.ID},
		OccurredAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build primary history: %v", err)
	}
	operations := &retryOperationRepo{items: map[string]library.LibraryOperation{storedOperation.ID: storedOperation}}
	histories := &retryHistoryRepo{items: map[string]library.HistoryRecord{storedHistory.ID: storedHistory}}
	service := &LibraryService{operations: operations, histories: histories, nowFunc: func() time.Time { return now }}
	metadataOperation := storedOperation
	metadataOperation.DisplayName = "Resolved metadata title"
	metadataHistory := storedHistory
	metadataHistory.DisplayName = metadataOperation.DisplayName

	if err := service.persistOperationAndHistory(context.Background(), &metadataOperation, &metadataHistory); err != nil {
		t.Fatalf("persist automatic metadata title: %v", err)
	}
	committedOperation, _ := operations.Get(context.Background(), storedOperation.ID)
	committedHistory, _ := histories.Get(context.Background(), storedHistory.ID)
	if committedOperation.DisplayName != metadataOperation.DisplayName || committedHistory.DisplayName != metadataOperation.DisplayName {
		t.Fatalf("automatic title was blocked before a rename event: operation=%#v history=%#v", committedOperation, committedHistory)
	}
}

func TestCancelOperationWaitsForRenameAndUsesCommittedTitle(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	operation := library.LibraryOperation{
		ID: "op-cancel-rename-race", LibraryID: "lib-1", Kind: "download",
		Status: library.OperationStatusRunning, DisplayName: "Original task", InputJSON: `{}`, OutputJSON: `{}`, CreatedAt: now,
	}
	primary, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID: "history-cancel-rename-race", LibraryID: operation.LibraryID, Category: operationHistoryCategory,
		Action: operation.Kind, DisplayName: operation.DisplayName, Status: string(operation.Status),
		Refs:       library.HistoryRecordRefs{OperationID: operation.ID, SubjectOperationID: operation.ID},
		OccurredAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("build primary history: %v", err)
	}
	baseRepo := &retryOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	operations := &renameInterleavingOperationRepo{retryOperationRepo: baseRepo}
	operations.blockNextSave()
	histories := &retryHistoryRepo{items: map[string]library.HistoryRecord{primary.ID: primary}}
	service := &LibraryService{operations: operations, histories: histories, nowFunc: func() time.Time { return now }}

	renameDone := make(chan error, 1)
	go func() {
		_, err := service.RenameOperation(context.Background(), librarydto.RenameOperationRequest{
			OperationID: operation.ID, Name: "Companion title",
		})
		renameDone <- err
	}()
	select {
	case <-operations.saveStarted:
	case <-time.After(time.Second):
		t.Fatal("rename did not reach the controlled Save")
	}

	type cancelResult struct {
		item librarydto.LibraryOperationDTO
		err  error
	}
	cancelDone := make(chan cancelResult, 1)
	go func() {
		item, err := service.CancelOperation(context.Background(), librarydto.CancelOperationRequest{OperationID: operation.ID})
		cancelDone <- cancelResult{item: item, err: err}
	}()
	select {
	case result := <-cancelDone:
		t.Fatalf("cancel escaped the in-flight rename lock: %#v", result)
	case <-time.After(30 * time.Millisecond):
	}

	close(operations.releaseSave)
	if err := <-renameDone; err != nil {
		t.Fatalf("complete rename: %v", err)
	}
	result := <-cancelDone
	if result.err != nil {
		t.Fatalf("cancel renamed operation: %v", result.err)
	}
	if result.item.DisplayName != "Companion title" || result.item.Status != string(library.OperationStatusCanceled) {
		t.Fatalf("cancel returned stale title/state: %#v", result.item)
	}
	committedOperation, _ := operations.Get(context.Background(), operation.ID)
	committedPrimary, _ := histories.Get(context.Background(), primary.ID)
	if committedOperation.DisplayName != "Companion title" || committedPrimary.DisplayName != "Companion title" {
		t.Fatalf("cancel reverted renamed projections: operation=%#v history=%#v", committedOperation, committedPrimary)
	}
	var canceledEvent library.HistoryRecord
	for _, item := range histories.items {
		if item.Category == operationEventHistoryCategory && item.Action == operationEventCanceled {
			canceledEvent = item
		}
	}
	if canceledEvent.DisplayName != "Companion title" {
		t.Fatalf("cancel lifecycle event captured stale title: %#v", canceledEvent)
	}
}

func assertRenameReadBlocked(t *testing.T, started <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-started:
		t.Fatalf("%s rename read escaped aggregate mutation lock", label)
	case <-time.After(30 * time.Millisecond):
	}
}

type renameFailingFileEventRepo struct{ err error }

func (*renameFailingFileEventRepo) ListByLibraryID(context.Context, string) ([]library.FileEventRecord, error) {
	return nil, nil
}

func (repo *renameFailingFileEventRepo) Save(context.Context, library.FileEventRecord) error {
	return repo.err
}

type renameEventFailingHistoryRepo struct {
	*ytdlpMetadataHistoryRepo
	err error
}

func (repo *renameEventFailingHistoryRepo) Save(ctx context.Context, item library.HistoryRecord) error {
	if item.Category == operationEventHistoryCategory && item.Action == operationEventRenamed {
		return repo.err
	}
	return repo.ytdlpMetadataHistoryRepo.Save(ctx, item)
}

type renameListFailingHistoryRepo struct {
	*ytdlpMetadataHistoryRepo
	err       error
	saveCalls int
}

func (repo *renameListFailingHistoryRepo) ListByLibraryID(context.Context, string) ([]library.HistoryRecord, error) {
	return nil, repo.err
}

func (repo *renameListFailingHistoryRepo) Save(ctx context.Context, item library.HistoryRecord) error {
	repo.saveCalls++
	return repo.ytdlpMetadataHistoryRepo.Save(ctx, item)
}

type renameFailingTouchLibraryRepo struct {
	*ytdlpMetadataLibraryRepo
	err       error
	saveCalls int
}

func (repo *renameFailingTouchLibraryRepo) Save(context.Context, library.Library) error {
	repo.saveCalls++
	return repo.err
}

type renameFailingModuleConfigRepo struct{ err error }

func (repo *renameFailingModuleConfigRepo) Get(context.Context) (library.ModuleConfig, error) {
	return library.ModuleConfig{}, repo.err
}

func (*renameFailingModuleConfigRepo) Save(context.Context, library.ModuleConfig) error { return nil }

type renameFailingCatalogProjection struct {
	err   error
	calls int
}

func (projection *renameFailingCatalogProjection) Run(context.Context) (CatalogBackfillResult, error) {
	projection.calls++
	return CatalogBackfillResult{}, projection.err
}

type renameObservedOperationRepo struct {
	*ytdlpMetadataOperationRepo
	getStarted chan struct{}
	getOnce    sync.Once
}

type renameInterleavingOperationRepo struct {
	*retryOperationRepo
	blockMu     sync.Mutex
	shouldBlock bool
	saveStarted chan struct{}
	releaseSave chan struct{}
	startedOnce sync.Once
}

func (repo *renameInterleavingOperationRepo) blockNextSave() {
	repo.blockMu.Lock()
	defer repo.blockMu.Unlock()
	repo.shouldBlock = true
	repo.saveStarted = make(chan struct{})
	repo.releaseSave = make(chan struct{})
}

func (repo *renameInterleavingOperationRepo) Save(ctx context.Context, item library.LibraryOperation) error {
	repo.blockMu.Lock()
	shouldBlock := repo.shouldBlock
	if shouldBlock {
		repo.shouldBlock = false
	}
	repo.blockMu.Unlock()
	if shouldBlock {
		repo.startedOnce.Do(func() { close(repo.saveStarted) })
		<-repo.releaseSave
	}
	return repo.retryOperationRepo.Save(ctx, item)
}

func (repo *renameObservedOperationRepo) Get(ctx context.Context, id string) (library.LibraryOperation, error) {
	repo.getOnce.Do(func() { close(repo.getStarted) })
	return repo.ytdlpMetadataOperationRepo.Get(ctx, id)
}

type renameObservedFileRepo struct {
	*deleteRuleFileRepo
	getStarted chan struct{}
	getOnce    sync.Once
}

func (repo *renameObservedFileRepo) Get(ctx context.Context, id string) (library.LibraryFile, error) {
	repo.getOnce.Do(func() { close(repo.getStarted) })
	return repo.deleteRuleFileRepo.Get(ctx, id)
}
