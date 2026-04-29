package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	settingsdto "xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/persistence"
)

type deleteRuleLibraryRepo struct {
	items   map[string]library.Library
	deleted []string
}

func (repo *deleteRuleLibraryRepo) List(_ context.Context) ([]library.Library, error) {
	items := make([]library.Library, 0, len(repo.items))
	for _, item := range repo.items {
		items = append(items, item)
	}
	return items, nil
}

func (repo *deleteRuleLibraryRepo) Get(_ context.Context, id string) (library.Library, error) {
	item, ok := repo.items[id]
	if !ok {
		return library.Library{}, library.ErrLibraryNotFound
	}
	return item, nil
}

func (repo *deleteRuleLibraryRepo) Save(_ context.Context, item library.Library) error {
	if repo.items == nil {
		repo.items = map[string]library.Library{}
	}
	repo.items[item.ID] = item
	return nil
}

func (repo *deleteRuleLibraryRepo) Delete(_ context.Context, id string) error {
	repo.deleted = append(repo.deleted, id)
	delete(repo.items, id)
	return nil
}

type deleteRuleFileRepo struct {
	items         map[string]library.LibraryFile
	savedItems    []library.LibraryFile
	deletedIDs    []string
	listByLibrary map[string][]string
}

func (repo *deleteRuleFileRepo) List(_ context.Context) ([]library.LibraryFile, error) {
	items := make([]library.LibraryFile, 0, len(repo.items))
	for _, item := range repo.items {
		items = append(items, item)
	}
	return items, nil
}

func (repo *deleteRuleFileRepo) ListByLibraryID(_ context.Context, libraryID string) ([]library.LibraryFile, error) {
	if ids, ok := repo.listByLibrary[libraryID]; ok {
		items := make([]library.LibraryFile, 0, len(ids))
		for _, id := range ids {
			if item, exists := repo.items[id]; exists {
				items = append(items, item)
			}
		}
		return items, nil
	}
	items := make([]library.LibraryFile, 0)
	for _, item := range repo.items {
		if item.LibraryID == libraryID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repo *deleteRuleFileRepo) Get(_ context.Context, id string) (library.LibraryFile, error) {
	item, ok := repo.items[id]
	if !ok {
		return library.LibraryFile{}, library.ErrFileNotFound
	}
	return item, nil
}

func (repo *deleteRuleFileRepo) Save(_ context.Context, item library.LibraryFile) error {
	if repo.items == nil {
		repo.items = map[string]library.LibraryFile{}
	}
	repo.items[item.ID] = item
	repo.savedItems = append(repo.savedItems, item)
	return nil
}

func (repo *deleteRuleFileRepo) Delete(_ context.Context, id string) error {
	repo.deletedIDs = append(repo.deletedIDs, id)
	delete(repo.items, id)
	return nil
}

type deleteRuleOperationRepo struct {
	items      map[string]library.LibraryOperation
	deletedIDs []string
}

func (repo *deleteRuleOperationRepo) List(_ context.Context) ([]library.LibraryOperation, error) {
	items := make([]library.LibraryOperation, 0, len(repo.items))
	for _, item := range repo.items {
		items = append(items, item)
	}
	return items, nil
}

func (repo *deleteRuleOperationRepo) ListByLibraryID(_ context.Context, libraryID string) ([]library.LibraryOperation, error) {
	items := make([]library.LibraryOperation, 0)
	for _, item := range repo.items {
		if item.LibraryID == libraryID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repo *deleteRuleOperationRepo) Get(_ context.Context, id string) (library.LibraryOperation, error) {
	item, ok := repo.items[id]
	if !ok {
		return library.LibraryOperation{}, library.ErrOperationNotFound
	}
	return item, nil
}

func (repo *deleteRuleOperationRepo) Save(_ context.Context, item library.LibraryOperation) error {
	if repo.items == nil {
		repo.items = map[string]library.LibraryOperation{}
	}
	repo.items[item.ID] = item
	return nil
}

func (repo *deleteRuleOperationRepo) Delete(_ context.Context, id string) error {
	repo.deletedIDs = append(repo.deletedIDs, id)
	delete(repo.items, id)
	return nil
}

type deleteRuleHistoryRepo struct {
	items              map[string]library.HistoryRecord
	savedItems         []library.HistoryRecord
	deletedByOperation []string
}

func (repo *deleteRuleHistoryRepo) ListByLibraryID(_ context.Context, libraryID string) ([]library.HistoryRecord, error) {
	items := make([]library.HistoryRecord, 0)
	for _, item := range repo.items {
		if item.LibraryID == libraryID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repo *deleteRuleHistoryRepo) Get(_ context.Context, _ string) (library.HistoryRecord, error) {
	return library.HistoryRecord{}, library.ErrHistoryRecordNotFound
}

func (repo *deleteRuleHistoryRepo) Save(_ context.Context, item library.HistoryRecord) error {
	if repo.items == nil {
		repo.items = map[string]library.HistoryRecord{}
	}
	repo.items[item.ID] = item
	repo.savedItems = append(repo.savedItems, item)
	return nil
}

func (repo *deleteRuleHistoryRepo) Delete(_ context.Context, _ string) error {
	return nil
}

func (repo *deleteRuleHistoryRepo) DeleteByOperationID(_ context.Context, operationID string) error {
	repo.deletedByOperation = append(repo.deletedByOperation, operationID)
	return nil
}

type deleteRuleOperationChunkRepo struct {
	deletedByOperation []string
}

func (repo *deleteRuleOperationChunkRepo) ListByOperationID(_ context.Context, _ string) ([]library.OperationChunk, error) {
	return nil, nil
}

func (repo *deleteRuleOperationChunkRepo) Save(_ context.Context, _ library.OperationChunk) error {
	return nil
}

func (repo *deleteRuleOperationChunkRepo) DeleteByOperationID(_ context.Context, operationID string) error {
	repo.deletedByOperation = append(repo.deletedByOperation, operationID)
	return nil
}

type deleteRuleWorkspaceRepo struct{}

func (repo *deleteRuleWorkspaceRepo) ListByLibraryID(_ context.Context, _ string) ([]library.WorkspaceStateRecord, error) {
	return nil, nil
}

func (repo *deleteRuleWorkspaceRepo) GetHeadByLibraryID(_ context.Context, _ string) (library.WorkspaceStateRecord, error) {
	return library.WorkspaceStateRecord{}, library.ErrWorkspaceStateNotFound
}

func (repo *deleteRuleWorkspaceRepo) Save(_ context.Context, _ library.WorkspaceStateRecord) error {
	return nil
}

type deleteRuleFileEventRepo struct{}

func (repo *deleteRuleFileEventRepo) ListByLibraryID(_ context.Context, _ string) ([]library.FileEventRecord, error) {
	return nil, nil
}

func (repo *deleteRuleFileEventRepo) Save(_ context.Context, _ library.FileEventRecord) error {
	return nil
}

type deleteRuleSubtitleRepo struct {
	deletedByFile []string
}

func (repo *deleteRuleSubtitleRepo) Get(_ context.Context, _ string) (library.SubtitleDocument, error) {
	return library.SubtitleDocument{}, library.ErrSubtitleDocumentNotFound
}

func (repo *deleteRuleSubtitleRepo) GetByFileID(_ context.Context, _ string) (library.SubtitleDocument, error) {
	return library.SubtitleDocument{}, library.ErrSubtitleDocumentNotFound
}

func (repo *deleteRuleSubtitleRepo) Save(_ context.Context, _ library.SubtitleDocument) error {
	return nil
}

func (repo *deleteRuleSubtitleRepo) DeleteByFileID(_ context.Context, fileID string) error {
	repo.deletedByFile = append(repo.deletedByFile, fileID)
	return nil
}

func TestDeleteFileMarksFileDeletedAndKeepsLocalFileByDefault(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	localPath := filepath.Join(tempDir, "episode.srt")
	if err := os.WriteFile(localPath, []byte("1\n00:00:00,000 --> 00:00:01,000\nhello\n"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	libraryItem := mustNewLibrary(t, "lib-1", now)
	fileItem := mustNewSubtitleFile(t, "file-1", "lib-1", "op-1", localPath, "doc-1", now)

	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	subtitles := &deleteRuleSubtitleRepo{}
	service := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:      files,
		workspace:  &deleteRuleWorkspaceRepo{},
		fileEvents: &deleteRuleFileEventRepo{},
		subtitles:  subtitles,
		nowFunc:    func() time.Time { return now },
	}

	if err := service.DeleteFile(ctx, dto.DeleteFileRequest{FileID: fileItem.ID}); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	got, err := files.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.State.Deleted {
		t.Fatal("expected file to be soft-deleted")
	}
	if got.State.Status != "deleted" {
		t.Fatalf("expected deleted status, got %q", got.State.Status)
	}
	if got.Storage.DocumentID != fileItem.Storage.DocumentID {
		t.Fatalf("expected subtitle document id to be retained for soft delete, got %q", got.Storage.DocumentID)
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("expected local file to remain on disk, got %v", err)
	}
	if len(subtitles.deletedByFile) != 1 || subtitles.deletedByFile[0] != fileItem.ID {
		t.Fatalf("expected subtitle document cleanup for %q, got %#v", fileItem.ID, subtitles.deletedByFile)
	}
	if len(files.deletedIDs) != 0 {
		t.Fatalf("expected no hard delete, got %#v", files.deletedIDs)
	}
}

func TestDeleteFileWithSQLiteRepoKeepsSubtitleStorageConstraintValid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	db, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "library-delete-file.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	libraries := libraryrepo.NewSQLiteLibraryRepository(db.Bun)
	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	subtitles := libraryrepo.NewSQLiteSubtitleDocumentRepository(db.Bun)

	libraryItem := mustNewLibrary(t, "lib-1", now)
	if err := libraries.Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}

	fileItem := mustNewSubtitleFile(t, "file-1", libraryItem.ID, "op-1", "/tmp/episode.srt", "doc-1", now)
	if err := files.Save(ctx, fileItem); err != nil {
		t.Fatalf("save file: %v", err)
	}

	documentItem, err := library.NewSubtitleDocument(library.SubtitleDocumentParams{
		ID:              fileItem.Storage.DocumentID,
		FileID:          fileItem.ID,
		LibraryID:       fileItem.LibraryID,
		Format:          "srt",
		OriginalContent: "1\n00:00:00,000 --> 00:00:01,000\nhello\n",
		CreatedAt:       &now,
		UpdatedAt:       &now,
	})
	if err != nil {
		t.Fatalf("new subtitle document: %v", err)
	}
	if err := subtitles.Save(ctx, documentItem); err != nil {
		t.Fatalf("save subtitle document: %v", err)
	}

	service := &LibraryService{
		libraries:  libraries,
		files:      files,
		workspace:  &deleteRuleWorkspaceRepo{},
		fileEvents: &deleteRuleFileEventRepo{},
		subtitles:  subtitles,
		nowFunc:    func() time.Time { return now },
	}

	if err := service.DeleteFile(ctx, dto.DeleteFileRequest{FileID: fileItem.ID}); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	got, err := files.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.State.Deleted {
		t.Fatal("expected file to be soft-deleted")
	}
	if got.Storage.DocumentID != fileItem.Storage.DocumentID {
		t.Fatalf("expected subtitle document id to remain on deleted file, got %q", got.Storage.DocumentID)
	}
	if _, err := subtitles.GetByFileID(ctx, fileItem.ID); err != library.ErrSubtitleDocumentNotFound {
		t.Fatalf("expected subtitle document to be removed, got %v", err)
	}
}

func TestDeleteFilesMarksMultipleFilesDeletedOnceEach(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 10, 30, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-1", now)
	fileItemOne := mustNewVideoFile(t, "file-1", "lib-1", "op-1", "/tmp/file-1.mp4", now)
	fileItemTwo := mustNewSubtitleFile(t, "file-2", "lib-1", "op-2", "/tmp/file-2.srt", "doc-2", now)

	files := &deleteRuleFileRepo{
		items: map[string]library.LibraryFile{
			fileItemOne.ID: fileItemOne,
			fileItemTwo.ID: fileItemTwo,
		},
	}
	subtitles := &deleteRuleSubtitleRepo{}
	service := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:      files,
		workspace:  &deleteRuleWorkspaceRepo{},
		fileEvents: &deleteRuleFileEventRepo{},
		subtitles:  subtitles,
		nowFunc:    func() time.Time { return now },
	}

	if err := service.DeleteFiles(ctx, dto.DeleteFilesRequest{
		FileIDs: []string{"file-1", "file-2", "file-1", " "},
	}); err != nil {
		t.Fatalf("DeleteFiles: %v", err)
	}

	gotOne, err := files.Get(ctx, fileItemOne.ID)
	if err != nil {
		t.Fatalf("Get file one: %v", err)
	}
	if !gotOne.State.Deleted {
		t.Fatal("expected first file to be soft-deleted")
	}
	gotTwo, err := files.Get(ctx, fileItemTwo.ID)
	if err != nil {
		t.Fatalf("Get file two: %v", err)
	}
	if !gotTwo.State.Deleted {
		t.Fatal("expected second file to be soft-deleted")
	}
	if len(files.deletedIDs) != 0 {
		t.Fatalf("expected no hard delete, got %#v", files.deletedIDs)
	}
	if len(subtitles.deletedByFile) != 2 {
		t.Fatalf("expected subtitle cleanup for both files, got %#v", subtitles.deletedByFile)
	}
}

func TestDeleteOperationCascadeFilesSoftDeletesOutputs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 11, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	localPath := filepath.Join(tempDir, "episode.mp4")
	if err := os.WriteFile(localPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	libraryItem := mustNewLibrary(t, "lib-1", now)
	fileItem := mustNewVideoFile(t, "file-1", "lib-1", "op-1", localPath, now)
	operationItem := mustNewOperation(t, "op-1", "lib-1", []library.OperationOutputFile{{FileID: fileItem.ID, Kind: "video", IsPrimary: true}}, now)

	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operationItem.ID: operationItem}}
	histories := &deleteRuleHistoryRepo{}
	chunks := &deleteRuleOperationChunkRepo{}
	service := &LibraryService{
		libraries:       &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:           files,
		operations:      operations,
		operationChunks: chunks,
		histories:       histories,
		workspace:       &deleteRuleWorkspaceRepo{},
		fileEvents:      &deleteRuleFileEventRepo{},
		subtitles:       &deleteRuleSubtitleRepo{},
		nowFunc:         func() time.Time { return now },
	}

	if err := service.DeleteOperation(ctx, dto.DeleteOperationRequest{OperationID: operationItem.ID, CascadeFiles: true}); err != nil {
		t.Fatalf("DeleteOperation: %v", err)
	}

	if _, err := operations.Get(ctx, operationItem.ID); err != library.ErrOperationNotFound {
		t.Fatalf("expected operation to be deleted, got %v", err)
	}
	got, err := files.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("Get file: %v", err)
	}
	if !got.State.Deleted {
		t.Fatal("expected task output file to be soft-deleted")
	}
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Fatalf("expected local file to be deleted, got %v", err)
	}
	if len(histories.deletedByOperation) != 1 || histories.deletedByOperation[0] != operationItem.ID {
		t.Fatalf("expected history cleanup, got %#v", histories.deletedByOperation)
	}
	if len(chunks.deletedByOperation) != 1 || chunks.deletedByOperation[0] != operationItem.ID {
		t.Fatalf("expected chunk cleanup, got %#v", chunks.deletedByOperation)
	}
}

func TestDeleteOperationCascadeFilesRemovesFailedOutputArtifacts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 11, 15, 0, 0, time.UTC)
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "xiadown", "yt-dlp", "youtube", "failed.mp4")
	partialPath := outputPath + ".part"
	previewPath := filepath.Join(tempDir, "xiadown", ".thumbnail-prefetch", "op-1", "thumbnail.jpg")
	for _, path := range []string{partialPath, previewPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir artifact parent: %v", err)
		}
		if err := os.WriteFile(path, []byte("artifact"), 0o644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
	}

	libraryItem := mustNewLibrary(t, "lib-1", now)
	operationItem := mustNewOperationWithKind(t, "op-1", "download", "lib-1", nil, now)
	operationItem.Status = library.OperationStatusFailed
	outputJSON, err := json.Marshal(map[string]any{
		"mainPath":             outputPath,
		"outputPaths":          []string{outputPath},
		"thumbnailPreviewPath": previewPath,
	})
	if err != nil {
		t.Fatalf("marshal output json: %v", err)
	}
	operationItem.OutputJSON = string(outputJSON)

	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operationItem.ID: operationItem}}
	service := &LibraryService{
		libraries:       &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:           &deleteRuleFileRepo{},
		operations:      operations,
		operationChunks: &deleteRuleOperationChunkRepo{},
		histories:       &deleteRuleHistoryRepo{},
		workspace:       &deleteRuleWorkspaceRepo{},
		fileEvents:      &deleteRuleFileEventRepo{},
		subtitles:       &deleteRuleSubtitleRepo{},
		nowFunc:         func() time.Time { return now },
	}

	if err := service.DeleteOperation(ctx, dto.DeleteOperationRequest{OperationID: operationItem.ID, CascadeFiles: true}); err != nil {
		t.Fatalf("DeleteOperation: %v", err)
	}
	for _, path := range []string{partialPath, previewPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected artifact %q to be deleted, got %v", path, err)
		}
	}
	if _, err := operations.Get(ctx, operationItem.ID); err != library.ErrOperationNotFound {
		t.Fatalf("expected operation to be deleted, got %v", err)
	}
}

func TestDeleteOperationCascadeFilesRemovesResidualYTDLPPartByRequestURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 11, 20, 0, 0, time.UTC)
	tempDir := t.TempDir()
	downloadDirectory := filepath.Join(tempDir, "Downloads")
	partialPath := filepath.Join(
		downloadDirectory,
		"xiadown",
		"yt-dlp",
		"youtube",
		"Synthetic Creator-Sample Travel Clip-2160p60+medium-TESTVID003C.f401.mp4.part",
	)
	oldPartialPath := filepath.Join(
		downloadDirectory,
		"xiadown",
		"yt-dlp",
		"youtube",
		"Synthetic Creator-Older Residual-TESTVID003C.f401.mp4.part",
	)
	if err := os.MkdirAll(filepath.Dir(partialPath), 0o755); err != nil {
		t.Fatalf("mkdir partial parent: %v", err)
	}
	for _, path := range []string{partialPath, oldPartialPath} {
		if err := os.WriteFile(path, []byte("partial"), 0o644); err != nil {
			t.Fatalf("write partial: %v", err)
		}
	}
	oldTime := now.Add(-time.Minute)
	if err := os.Chtimes(oldPartialPath, oldTime, oldTime); err != nil {
		t.Fatalf("set old partial time: %v", err)
	}
	newTime := now.Add(time.Minute)
	if err := os.Chtimes(partialPath, newTime, newTime); err != nil {
		t.Fatalf("set partial time: %v", err)
	}

	libraryItem := mustNewLibrary(t, "lib-1", now)
	operationItem := mustNewOperationWithKind(t, "op-1", "download", "lib-1", nil, now)
	operationItem.Status = library.OperationStatusCanceled
	operationItem.InputJSON = `{"url":"https://www.youtube.com/watch?v=TESTVID003C"}`
	operationItem.OutputJSON = "{}"

	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operationItem.ID: operationItem}}
	service := &LibraryService{
		libraries:       &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:           &deleteRuleFileRepo{},
		operations:      operations,
		operationChunks: &deleteRuleOperationChunkRepo{},
		histories:       &deleteRuleHistoryRepo{},
		workspace:       &deleteRuleWorkspaceRepo{},
		fileEvents:      &deleteRuleFileEventRepo{},
		subtitles:       &deleteRuleSubtitleRepo{},
		settings:        ytdlpSettingsReader{settings: settingsdto.Settings{DownloadDirectory: downloadDirectory}},
		nowFunc:         func() time.Time { return now },
	}

	if err := service.DeleteOperation(ctx, dto.DeleteOperationRequest{OperationID: operationItem.ID, CascadeFiles: true}); err != nil {
		t.Fatalf("DeleteOperation: %v", err)
	}
	if _, err := os.Stat(partialPath); !os.IsNotExist(err) {
		t.Fatalf("expected residual part file to be deleted, got %v", err)
	}
	if _, err := os.Stat(oldPartialPath); err != nil {
		t.Fatalf("expected older residual part file to remain, got %v", err)
	}
}

func TestCancelOperationRecordsResidualYTDLPPartByRequestURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 11, 25, 0, 0, time.UTC)
	tempDir := t.TempDir()
	downloadDirectory := filepath.Join(tempDir, "Downloads")
	partialPath := filepath.Join(
		downloadDirectory,
		"xiadown",
		"yt-dlp",
		"youtube",
		"Synthetic Creator-Market Walkthrough-2160p HDR+medium-TESTVID004D.f337.webm.part",
	)
	oldPartialPath := filepath.Join(
		downloadDirectory,
		"xiadown",
		"yt-dlp",
		"youtube",
		"Synthetic Creator-Older Residual-TESTVID004D.f337.webm.part",
	)
	if err := os.MkdirAll(filepath.Dir(partialPath), 0o755); err != nil {
		t.Fatalf("mkdir partial parent: %v", err)
	}
	for _, path := range []string{partialPath, oldPartialPath} {
		if err := os.WriteFile(path, []byte("partial"), 0o644); err != nil {
			t.Fatalf("write partial: %v", err)
		}
	}
	oldTime := now.Add(-time.Minute)
	if err := os.Chtimes(oldPartialPath, oldTime, oldTime); err != nil {
		t.Fatalf("set old partial time: %v", err)
	}
	newTime := now.Add(time.Minute)
	if err := os.Chtimes(partialPath, newTime, newTime); err != nil {
		t.Fatalf("set partial time: %v", err)
	}

	operationItem := mustNewOperationWithKind(t, "op-1", "download", "lib-1", nil, now)
	operationItem.Status = library.OperationStatusRunning
	operationItem.InputJSON = `{"url":"https://www.youtube.com/watch?v=TESTVID004D"}`
	operationItem.OutputJSON = "{}"

	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operationItem.ID: operationItem}}
	service := &LibraryService{
		operations: operations,
		settings:   ytdlpSettingsReader{settings: settingsdto.Settings{DownloadDirectory: downloadDirectory}},
		nowFunc:    func() time.Time { return now },
	}

	canceled, err := service.markOperationCanceled(ctx, operationItem)
	if err != nil {
		t.Fatalf("markOperationCanceled: %v", err)
	}
	temporaryPaths := collectOperationArtifactPaths(canceled.OutputJSON, nil, []string{operationTemporaryPathsKey})
	if len(temporaryPaths) != 1 || temporaryPaths[0] != partialPath {
		t.Fatalf("expected temporary path %q, got %#v", partialPath, temporaryPaths)
	}
	stored, err := operations.Get(ctx, operationItem.ID)
	if err != nil {
		t.Fatalf("get stored operation: %v", err)
	}
	storedTemporaryPaths := collectOperationArtifactPaths(stored.OutputJSON, nil, []string{operationTemporaryPathsKey})
	if len(storedTemporaryPaths) != 1 || storedTemporaryPaths[0] != partialPath {
		t.Fatalf("expected stored temporary path %q, got %#v", partialPath, storedTemporaryPaths)
	}
}

func TestFailYTDLPOperationRecordsResidualYTDLPPartByRequestURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 11, 28, 0, 0, time.UTC)
	tempDir := t.TempDir()
	downloadDirectory := filepath.Join(tempDir, "Downloads")
	partialPath := filepath.Join(
		downloadDirectory,
		"xiadown",
		"yt-dlp",
		"youtube",
		"Synthetic Creator-City Walkthrough-2160p60+medium-TESTVID005E.f315.webm.part",
	)
	oldPartialPath := filepath.Join(
		downloadDirectory,
		"xiadown",
		"yt-dlp",
		"youtube",
		"Synthetic Creator-Older Residual-TESTVID005E.f315.webm.part",
	)
	if err := os.MkdirAll(filepath.Dir(partialPath), 0o755); err != nil {
		t.Fatalf("mkdir partial parent: %v", err)
	}
	for _, path := range []string{partialPath, oldPartialPath} {
		if err := os.WriteFile(path, []byte("partial"), 0o644); err != nil {
			t.Fatalf("write partial: %v", err)
		}
	}
	oldTime := now.Add(-time.Minute)
	if err := os.Chtimes(oldPartialPath, oldTime, oldTime); err != nil {
		t.Fatalf("set old partial time: %v", err)
	}
	newTime := now.Add(time.Minute)
	if err := os.Chtimes(partialPath, newTime, newTime); err != nil {
		t.Fatalf("set partial time: %v", err)
	}

	libraryItem := mustNewLibrary(t, "lib-1", now)
	operationItem := mustNewOperationWithKind(t, "op-1", "download", "lib-1", nil, now)
	operationItem.Status = library.OperationStatusRunning
	operationItem.InputJSON = `{"url":"https://www.youtube.com/watch?v=TESTVID005E"}`
	operationItem.OutputJSON = "{}"
	historyItem := mustNewHistoryForOperation(t, "history-1", "lib-1", operationItem.ID, operationItem.Kind, nil, now)

	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operationItem.ID: operationItem}}
	service := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		operations: operations,
		histories:  &deleteRuleHistoryRepo{items: map[string]library.HistoryRecord{historyItem.ID: historyItem}},
		settings:   ytdlpSettingsReader{settings: settingsdto.Settings{DownloadDirectory: downloadDirectory}},
		nowFunc:    func() time.Time { return now },
	}

	service.failYTDLPOperation(ctx, &operationItem, &historyItem, fmt.Errorf("yt-dlp failed"), "download_failed", "")

	stored, err := operations.Get(ctx, operationItem.ID)
	if err != nil {
		t.Fatalf("get stored operation: %v", err)
	}
	temporaryPaths := collectOperationArtifactPaths(stored.OutputJSON, nil, []string{operationTemporaryPathsKey})
	if len(temporaryPaths) != 1 || temporaryPaths[0] != partialPath {
		t.Fatalf("expected temporary path %q, got %#v", partialPath, temporaryPaths)
	}
}

func TestDeleteOperationsDeletesMultipleOperationsOnceEach(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 11, 30, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-1", now)
	operationItemOne := mustNewOperation(t, "op-1", "lib-1", nil, now)
	operationItemTwo := mustNewOperation(t, "op-2", "lib-1", nil, now)

	operations := &deleteRuleOperationRepo{
		items: map[string]library.LibraryOperation{
			operationItemOne.ID: operationItemOne,
			operationItemTwo.ID: operationItemTwo,
		},
	}
	histories := &deleteRuleHistoryRepo{}
	chunks := &deleteRuleOperationChunkRepo{}
	service := &LibraryService{
		libraries:       &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:           &deleteRuleFileRepo{},
		operations:      operations,
		operationChunks: chunks,
		histories:       histories,
		workspace:       &deleteRuleWorkspaceRepo{},
		fileEvents:      &deleteRuleFileEventRepo{},
		subtitles:       &deleteRuleSubtitleRepo{},
		nowFunc:         func() time.Time { return now },
	}

	if err := service.DeleteOperations(ctx, dto.DeleteOperationsRequest{
		OperationIDs: []string{"op-1", "op-2", "op-1", " "},
	}); err != nil {
		t.Fatalf("DeleteOperations: %v", err)
	}

	if len(operations.deletedIDs) != 2 {
		t.Fatalf("expected 2 deleted operations, got %#v", operations.deletedIDs)
	}
	if _, err := operations.Get(ctx, operationItemOne.ID); err != library.ErrOperationNotFound {
		t.Fatalf("expected operation %q to be deleted, got %v", operationItemOne.ID, err)
	}
	if _, err := operations.Get(ctx, operationItemTwo.ID); err != library.ErrOperationNotFound {
		t.Fatalf("expected operation %q to be deleted, got %v", operationItemTwo.ID, err)
	}
	if len(histories.deletedByOperation) != 2 {
		t.Fatalf("expected history cleanup for both operations, got %#v", histories.deletedByOperation)
	}
	if len(chunks.deletedByOperation) != 2 {
		t.Fatalf("expected chunk cleanup for both operations, got %#v", chunks.deletedByOperation)
	}
}

func TestDeleteLibraryDeletesLocalFilesBeforeRemovingLibrary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	localPath := filepath.Join(tempDir, "episode.mp4")
	if err := os.WriteFile(localPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	libraryItem := mustNewLibrary(t, "lib-1", now)
	fileItem := mustNewVideoFile(t, "file-1", "lib-1", "op-1", localPath, now)

	libraries := &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}}
	files := &deleteRuleFileRepo{
		items:         map[string]library.LibraryFile{fileItem.ID: fileItem},
		listByLibrary: map[string][]string{libraryItem.ID: {fileItem.ID}},
	}
	service := &LibraryService{
		libraries: libraries,
		files:     files,
		nowFunc:   func() time.Time { return now },
	}

	if err := service.DeleteLibrary(ctx, dto.DeleteLibraryRequest{LibraryID: libraryItem.ID}); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}

	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Fatalf("expected library local file to be deleted, got %v", err)
	}
	if len(libraries.deleted) != 1 || libraries.deleted[0] != libraryItem.ID {
		t.Fatalf("expected library delete call, got %#v", libraries.deleted)
	}
}

func TestCleanupSourceFileAfterSuccessfulTranscodeSyncsDownloadOutputs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "episode.mp4")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	libraryItem := mustNewLibrary(t, "lib-1", now)
	sourceFile := mustNewVideoFile(t, "file-source", "lib-1", "download-op", sourcePath, now)
	downloadOperation := mustNewOperationWithKind(t, "download-op", "download", "lib-1", []library.OperationOutputFile{{
		FileID:    sourceFile.ID,
		Kind:      string(sourceFile.Kind),
		Format:    "mp4",
		IsPrimary: true,
	}}, now)
	downloadHistory := mustNewHistoryForOperation(t, "history-download", "lib-1", "download-op", "download", downloadOperation.OutputFiles, now)

	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{sourceFile.ID: sourceFile}}
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{downloadOperation.ID: downloadOperation}}
	histories := &deleteRuleHistoryRepo{items: map[string]library.HistoryRecord{downloadHistory.ID: downloadHistory}}
	service := &LibraryService{
		libraries:  &deleteRuleLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}},
		files:      files,
		operations: operations,
		histories:  histories,
		workspace:  &deleteRuleWorkspaceRepo{},
		fileEvents: &deleteRuleFileEventRepo{},
		subtitles:  &deleteRuleSubtitleRepo{},
		nowFunc:    func() time.Time { return now },
	}

	updatedSource := service.cleanupSourceFileAfterSuccessfulTranscode(ctx, sourceFile, "transcode-op")

	if !updatedSource.State.Deleted {
		t.Fatal("expected source file to be marked deleted")
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be removed locally, got %v", err)
	}

	storedSource, err := files.Get(ctx, sourceFile.ID)
	if err != nil {
		t.Fatalf("Get source file: %v", err)
	}
	if !storedSource.State.Deleted {
		t.Fatal("expected stored source file to be deleted")
	}

	storedOperation, err := operations.Get(ctx, downloadOperation.ID)
	if err != nil {
		t.Fatalf("Get operation: %v", err)
	}
	if len(storedOperation.OutputFiles) != 1 || !storedOperation.OutputFiles[0].Deleted {
		t.Fatalf("expected download operation output to be marked deleted, got %#v", storedOperation.OutputFiles)
	}
	if storedOperation.Metrics.FileCount != 0 {
		t.Fatalf("expected download operation metrics file count to be 0, got %#v", storedOperation.Metrics)
	}

	storedHistory := histories.items[downloadHistory.ID]
	if len(storedHistory.Files) != 1 || !storedHistory.Files[0].Deleted {
		t.Fatalf("expected download history output to be marked deleted, got %#v", storedHistory.Files)
	}
	if storedHistory.Metrics.FileCount != 0 {
		t.Fatalf("expected download history metrics file count to be 0, got %#v", storedHistory.Metrics)
	}
}

func mustNewLibrary(t *testing.T, id string, now time.Time) library.Library {
	t.Helper()
	item, err := library.NewLibrary(library.LibraryParams{
		ID:        id,
		Name:      id,
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}
	return item
}

func mustNewVideoFile(t *testing.T, id string, libraryID string, operationID string, localPath string, now time.Time) library.LibraryFile {
	t.Helper()
	item, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:        id,
		LibraryID: libraryID,
		Kind:      "video",
		Name:      filepath.Base(localPath),
		Storage: library.FileStorage{
			Mode:      "local_path",
			LocalPath: localPath,
		},
		Origin: library.FileOrigin{
			Kind:        "download",
			OperationID: operationID,
		},
		LatestOperationID: operationID,
		State: library.FileState{
			Status: "ready",
		},
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new video file: %v", err)
	}
	return item
}

func mustNewSubtitleFile(t *testing.T, id string, libraryID string, operationID string, localPath string, documentID string, now time.Time) library.LibraryFile {
	t.Helper()
	item, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:        id,
		LibraryID: libraryID,
		Kind:      "subtitle",
		Name:      filepath.Base(localPath),
		Storage: library.FileStorage{
			Mode:       "hybrid",
			LocalPath:  localPath,
			DocumentID: documentID,
		},
		Origin: library.FileOrigin{
			Kind:        "download",
			OperationID: operationID,
		},
		LatestOperationID: operationID,
		State: library.FileState{
			Status: "ready",
		},
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new subtitle file: %v", err)
	}
	return item
}

func mustNewOperation(t *testing.T, id string, libraryID string, outputFiles []library.OperationOutputFile, now time.Time) library.LibraryOperation {
	t.Helper()
	return mustNewOperationWithKind(t, id, "transcode", libraryID, outputFiles, now)
}

func mustNewOperationWithKind(t *testing.T, id string, kind string, libraryID string, outputFiles []library.OperationOutputFile, now time.Time) library.LibraryOperation {
	t.Helper()
	item, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          id,
		LibraryID:   libraryID,
		Kind:        kind,
		Status:      string(library.OperationStatusSucceeded),
		DisplayName: titleASCII(kind),
		InputJSON:   "{}",
		OutputJSON:  "{}",
		OutputFiles: outputFiles,
		CreatedAt:   &now,
	})
	if err != nil {
		t.Fatalf("new operation: %v", err)
	}
	return item
}

func titleASCII(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:])
}

func mustNewHistoryForOperation(
	t *testing.T,
	id string,
	libraryID string,
	operationID string,
	action string,
	files []library.OperationOutputFile,
	now time.Time,
) library.HistoryRecord {
	t.Helper()
	item, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          id,
		LibraryID:   libraryID,
		Category:    "operation",
		Action:      action,
		DisplayName: action,
		Status:      "succeeded",
		Refs:        library.HistoryRecordRefs{OperationID: operationID},
		Files:       files,
		OccurredAt:  &now,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	})
	if err != nil {
		t.Fatalf("new history: %v", err)
	}
	return item
}
