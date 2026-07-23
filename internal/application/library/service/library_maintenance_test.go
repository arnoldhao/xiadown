package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/events"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
)

type staleListMaintenanceFileRepo struct {
	*deleteRuleFileRepo
	listed library.LibraryFile
}

func (repo *staleListMaintenanceFileRepo) List(context.Context) ([]library.LibraryFile, error) {
	return []library.LibraryFile{repo.listed}, nil
}

func TestLibraryDatabaseIntegrityStatusIsQueryableWithoutRunningMaintenanceScan(t *testing.T) {
	t.Parallel()
	service := &LibraryService{}
	if status := service.GetDatabaseIntegrityStatus(context.Background()); status.State != databaseIntegrityUnavailable {
		t.Fatalf("status without provider = %#v", status)
	}
	service.SetDatabaseIntegrityStatusProvider(func() (string, string, string) {
		return databaseIntegrityFailed, "2026-07-20T08:00:00Z", "fixture failure"
	})
	status := service.GetDatabaseIntegrityStatus(context.Background())
	if status.State != databaseIntegrityFailed || status.CheckedAt != "2026-07-20T08:00:00Z" || status.Detail != "fixture failure" {
		t.Fatalf("provider status = %#v", status)
	}
}

func TestLibraryMaintenanceSnapshotFindsRecoverableSoftDeletes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
	existingPath := filepath.Join(t.TempDir(), "recoverable.mp3")
	if err := os.WriteFile(existingPath, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write recoverable file: %v", err)
	}
	existing := mustNewAudioFile(t, "file-existing", "lib-1", "op-existing", existingPath, now)
	existing.State.Deleted = true
	existing.State.Status = "deleted"
	missing := mustNewAudioFile(t, "file-missing", "lib-1", "op-missing", filepath.Join(t.TempDir(), "gone.mp3"), now)
	missing.State.Deleted = true
	missing.State.Status = "deleted"
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{
		existing.ID: existing,
		missing.ID:  missing,
	}}
	fileEvents := &deleteRuleFileEventRepo{}
	service := &LibraryService{files: files, fileEvents: fileEvents, nowFunc: func() time.Time { return now.Add(time.Minute) }}

	snapshot, err := service.GetLibraryMaintenanceSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetLibraryMaintenanceSnapshot: %v", err)
	}
	if snapshot.CheckedFiles != 0 || len(snapshot.DeletedFiles) != 2 {
		t.Fatalf("unexpected maintenance inventory: %#v", snapshot)
	}
	restorable := map[string]bool{}
	for _, file := range snapshot.DeletedFiles {
		restorable[file.FileID] = file.CanRestore
	}
	if !restorable[existing.ID] || restorable[missing.ID] {
		t.Fatalf("unexpected restore eligibility: %#v", restorable)
	}

	result, err := service.RestoreDeletedLibraryFiles(ctx, dto.RestoreDeletedLibraryFilesRequest{
		FileIDs: []string{existing.ID, missing.ID},
	})
	if err != nil {
		t.Fatalf("RestoreDeletedLibraryFiles: %v", err)
	}
	if result.Checked != 2 || result.Restored != 1 || result.Skipped != 1 {
		t.Fatalf("unexpected restore result: %#v", result)
	}
	if stored := files.items[existing.ID]; stored.State.Deleted || stored.State.Status != "active" || stored.State.LastError != "" {
		t.Fatalf("existing file was not restored: %#v", stored.State)
	}
	if stored := files.items[missing.ID]; !stored.State.Deleted || stored.State.Status != "deleted" {
		t.Fatalf("missing file must remain deleted: %#v", stored.State)
	}
	if len(fileEvents.items) != 1 || fileEvents.items[0].EventType != libraryFileEventRestored ||
		fileEvents.items[0].OperationID != "op-existing" {
		t.Fatalf("expected one restored event, got %#v", fileEvents.items)
	}
	detail := dto.FileEventDetailDTO{}
	if err := json.Unmarshal([]byte(fileEvents.items[0].DetailJSON), &detail); err != nil {
		t.Fatalf("decode restored event: %v", err)
	}
	if detail.Before == nil || detail.After == nil || len(detail.Changes) != 1 ||
		detail.Changes[0] != (dto.FileFieldChangeDTO{Field: "fileLifecycle", Before: "deleted", After: "active"}) {
		t.Fatalf("unexpected restored event detail: %#v", detail)
	}
}

func TestVerifyLibraryFilesUsesCommittedRenameInActivityAndPublish(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	createdAt := time.Date(2026, 7, 21, 11, 0, 0, 0, time.UTC)
	checkedAt := createdAt.Add(time.Hour)
	path := filepath.Join(t.TempDir(), "available.mp3")
	if err := os.WriteFile(path, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write available file: %v", err)
	}
	stale := mustNewAudioFile(t, "file-renamed-during-scan", "lib-1", "op-1", path, createdAt)
	stale.DisplayName = "Old title"
	stale.State.LastError = missingLocalFileError
	current := stale
	current.DisplayName = "Renamed title"
	files := &staleListMaintenanceFileRepo{
		deleteRuleFileRepo: &deleteRuleFileRepo{items: map[string]library.LibraryFile{current.ID: current}},
		listed:             stale,
	}
	fileEvents := &deleteRuleFileEventRepo{}
	eventBus := events.NewInMemoryBus()
	var published dto.LibraryFileDTO
	unsubscribe := eventBus.Subscribe(libraryTopicFile, func(event events.Event) {
		published, _ = event.Payload.(dto.LibraryFileDTO)
	})
	defer unsubscribe()
	service := &LibraryService{
		files: files, fileEvents: fileEvents, bus: eventBus,
		nowFunc: func() time.Time { return checkedAt },
	}

	result, err := service.VerifyLibraryFiles(ctx)
	if err != nil {
		t.Fatalf("VerifyLibraryFiles: %v", err)
	}
	if result.Checked != 1 || result.Missing != 0 {
		t.Fatalf("unexpected verify result: %#v", result)
	}
	stored := files.items[current.ID]
	if stored.DisplayName != current.DisplayName || stored.State.LastError != "" {
		t.Fatalf("verification replayed stale file projection: %#v", stored)
	}
	if len(fileEvents.items) != 1 || fileEvents.items[0].EventType != libraryFileEventAvailableAgain {
		t.Fatalf("expected available-again activity, got %#v", fileEvents.items)
	}
	detail := dto.FileEventDetailDTO{}
	if err := json.Unmarshal([]byte(fileEvents.items[0].DetailJSON), &detail); err != nil {
		t.Fatalf("decode availability activity: %v", err)
	}
	if detail.Before == nil || detail.After == nil ||
		detail.Before.Name != current.DisplayName || detail.After.Name != current.DisplayName {
		t.Fatalf("availability activity used stale display name: %#v", detail)
	}
	if published.ID != current.ID || published.DisplayName != current.DisplayName {
		t.Fatalf("file publish used stale display name: %#v", published)
	}
}

func TestLibraryTaskMaintenanceUsesPlayableOutputHealth(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	write := func(name string) string {
		t.Helper()
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}
	newFile := func(id, operationID, kind, path string, deleted bool) library.LibraryFile {
		t.Helper()
		originKind := "download"
		if kind == string(library.FileKindTranscode) {
			originKind = "transcode"
		}
		item, err := library.NewLibraryFile(library.LibraryFileParams{
			ID: id, LibraryID: "lib-1", Kind: kind, Name: filepath.Base(path),
			Storage:           library.FileStorage{Mode: "local_path", LocalPath: path},
			Origin:            library.FileOrigin{Kind: originKind, OperationID: operationID},
			LatestOperationID: operationID,
			State:             library.FileState{Status: "active", Deleted: deleted},
			CreatedAt:         &now,
			UpdatedAt:         &now,
		})
		if err != nil {
			t.Fatalf("new %s file: %v", kind, err)
		}
		if deleted {
			item.State.Status = "deleted"
		}
		return item
	}

	source := newFile("source", "op-replacement", "video", filepath.Join(directory, "gone.webm"), true)
	replacement := newFile("replacement", "op-transcode", "transcode", write("ready.mp4"), false)
	replacement.Lineage.RootFileID = source.ID
	audio := newFile("audio", "op-thumbnail", "audio", write("ready.mp3"), false)
	thumbnail := newFile("thumbnail", "op-thumbnail", "thumbnail", filepath.Join(directory, "gone.jpg"), true)
	bad := newFile("bad", "op-bad", "audio", filepath.Join(directory, "gone-bad.mp3"), true)
	badArtwork := newFile("bad-artwork", "op-bad", "thumbnail", write("cover.jpg"), false)
	transcodeSource := newFile("transcode-source", "op-download", "video", write("source.mp4"), false)
	transcodeSource.LatestOperationID = "op-transcode-missing"
	badTranscode := newFile("bad-transcode", "op-transcode-missing", "transcode", filepath.Join(directory, "gone-transcode.mp4"), true)
	badTranscode.Lineage.RootFileID = transcodeSource.ID

	replacementOperation := mustNewOperation(t, "op-replacement", "lib-1", []library.OperationOutputFile{
		{FileID: source.ID, Kind: "video", IsPrimary: true},
	}, now)
	thumbnailOperation := mustNewOperation(t, "op-thumbnail", "lib-1", []library.OperationOutputFile{
		{FileID: audio.ID, Kind: "audio", IsPrimary: true},
		{FileID: thumbnail.ID, Kind: "thumbnail", Deleted: true},
	}, now)
	badOperation := mustNewOperation(t, "op-bad", "lib-1", []library.OperationOutputFile{
		{FileID: bad.ID, Kind: "audio", IsPrimary: true, Deleted: true},
		{FileID: badArtwork.ID, Kind: "thumbnail"},
	}, now)
	declaredMissing := mustNewOperation(t, "op-declared-missing", "lib-1", []library.OperationOutputFile{
		{FileID: "no-record", Kind: "video", IsPrimary: true},
		{FileID: "no-thumbnail", Kind: "thumbnail", Deleted: true},
	}, now)
	missingTranscode := mustNewOperation(t, "op-transcode-missing", "lib-1", []library.OperationOutputFile{
		{FileID: badTranscode.ID, Kind: "transcode", IsPrimary: true, Deleted: true},
	}, now)

	issues := libraryTaskMaintenanceIssues(
		[]library.LibraryOperation{replacementOperation, thumbnailOperation, badOperation, declaredMissing, missingTranscode},
		[]library.LibraryFile{source, replacement, audio, thumbnail, bad, badArtwork, transcodeSource, badTranscode},
	)
	if len(issues) != 3 {
		t.Fatalf("expected only tasks without a playable output, got %#v", issues)
	}
	byID := map[string]dto.LibraryTaskMaintenanceDTO{}
	for _, issue := range issues {
		byID[issue.OperationID] = issue
	}
	if issue, ok := byID[badOperation.ID]; !ok || issue.OutputCount != 1 || issue.DeletedOutputCount != 1 || issue.AvailableOutputCount != 0 {
		t.Fatalf("unexpected deleted-output issue: %#v", issue)
	}
	if issue, ok := byID[declaredMissing.ID]; !ok || issue.OutputCount != 1 || issue.UnavailableOutputCount != 1 || issue.DeletedOutputCount != 0 {
		t.Fatalf("unexpected missing-record issue: %#v", issue)
	}
	if issue, ok := byID[missingTranscode.ID]; !ok || issue.AvailableOutputCount != 0 || issue.DeletedOutputCount != 1 {
		t.Fatalf("source touched by a transcode must not count as its output: %#v", issue)
	}
	if _, exists := byID[replacementOperation.ID]; exists {
		t.Fatal("deleted original with healthy transcode must not be reported")
	}
	if _, exists := byID[thumbnailOperation.ID]; exists {
		t.Fatal("deleted thumbnail must not make a task unhealthy")
	}
}

func TestRestoreDeletedLibraryFileMakesCatalogItemVisible(t *testing.T) {
	ctx := context.Background()
	db := openCatalogBackfillTestDatabase(t)
	now := time.Date(2026, 7, 20, 11, 0, 0, 0, time.UTC)
	seedCatalogBackfillLibrary(t, ctx, db, "bundle-restore", now)
	path := filepath.Join(t.TempDir(), "restore.mp4")
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatalf("write restore fixture: %v", err)
	}
	seedCatalogBackfillFile(t, ctx, db, "file-restore", "bundle-restore", "video", "restore.mp4", path, "", now)

	libraries := libraryrepo.NewSQLiteLibraryRepository(db.Bun)
	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	projection := NewLegacyCatalogBackfillService(
		libraries, files, libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	projection.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := projection.Run(ctx); err != nil {
		t.Fatalf("initial projection: %v", err)
	}
	file, err := files.Get(ctx, "file-restore")
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	file.State.Deleted = true
	file.State.Status = "deleted"
	file.UpdatedAt = now.Add(2 * time.Minute)
	if err := files.Save(ctx, file); err != nil {
		t.Fatalf("soft-delete file: %v", err)
	}
	projection.now = func() time.Time { return now.Add(3 * time.Minute) }
	if _, err := projection.RunLibrary(ctx, file.LibraryID); err != nil {
		t.Fatalf("project trash: %v", err)
	}

	service := &LibraryService{
		libraries: libraries,
		files:     files,
		nowFunc:   func() time.Time { return now.Add(4 * time.Minute) },
	}
	service.SetCatalogProjectionRunner(projection)
	result, err := service.RestoreDeletedLibraryFiles(ctx, dto.RestoreDeletedLibraryFilesRequest{
		FileIDs: []string{file.ID},
	})
	if err != nil || result.Restored != 1 {
		t.Fatalf("restore file: result=%#v err=%v", result, err)
	}

	catalogService := NewCatalogService(
		libraryrepo.NewSQLiteCatalogRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogItemRepository(db.Bun),
		libraryrepo.NewSQLiteItemAssetRepository(db.Bun),
		files,
		libraryrepo.NewSQLiteStorageRootRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogCollectionRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogTagRepository(db.Bun),
		libraryrepo.NewSQLiteUserStateRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogMutationRepository(db.Bun), nil,
	)
	visible, err := catalogService.ListCatalogItems(ctx, dto.ListCatalogItemsRequest{
		Status: "all", ExcludeTrashed: true,
	})
	if err != nil || visible.Total != 1 || len(visible.Items) != 1 || visible.Items[0].Status != "active" {
		t.Fatalf("restored file did not return to normal browse: %#v, err=%v", visible, err)
	}
}
