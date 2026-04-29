package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestNormalizeProgressDetailUsesI18nForStageOnlyMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stage string
		line  string
		want  string
	}{
		{
			name:  "subtitle stage line",
			stage: "Downloading subtitles",
			line:  "Downloading subtitles",
			want:  progressText("library.progress.downloadingSubtitles"),
		},
		{
			name:  "thumbnail empty line",
			stage: "Downloading thumbnail",
			line:  "",
			want:  progressText("library.progress.downloadingThumbnail"),
		},
		{
			name:  "download prefixed line",
			stage: "Downloading video",
			line:  "[download] Destination: example.mp4",
			want:  progressText("library.progress.downloadingVideo"),
		},
		{
			name:  "keeps detailed line",
			stage: "Downloading subtitles",
			line:  "subtitle download failed: network error",
			want:  "subtitle download failed: network error",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeProgressDetail(tt.stage, tt.line); got != tt.want {
				t.Fatalf("normalizeProgressDetail() = %q, want %q", got, tt.want)
			}
		})
	}
}

type ytdlpMetadataLibraryRepo struct {
	item  library.Library
	saved []library.Library
}

func (repo *ytdlpMetadataLibraryRepo) List(_ context.Context) ([]library.Library, error) {
	return []library.Library{repo.item}, nil
}

func (repo *ytdlpMetadataLibraryRepo) Get(_ context.Context, _ string) (library.Library, error) {
	return repo.item, nil
}

func (repo *ytdlpMetadataLibraryRepo) Save(_ context.Context, item library.Library) error {
	repo.item = item
	repo.saved = append(repo.saved, item)
	return nil
}

func (repo *ytdlpMetadataLibraryRepo) Delete(_ context.Context, _ string) error {
	return nil
}

type ytdlpMetadataOperationRepo struct {
	saved []library.LibraryOperation
}

func (repo *ytdlpMetadataOperationRepo) List(_ context.Context) ([]library.LibraryOperation, error) {
	return repo.saved, nil
}

func (repo *ytdlpMetadataOperationRepo) ListByLibraryID(_ context.Context, libraryID string) ([]library.LibraryOperation, error) {
	result := make([]library.LibraryOperation, 0)
	for _, item := range repo.saved {
		if item.LibraryID == libraryID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo *ytdlpMetadataOperationRepo) Get(_ context.Context, id string) (library.LibraryOperation, error) {
	for _, item := range repo.saved {
		if item.ID == id {
			return item, nil
		}
	}
	return library.LibraryOperation{}, library.ErrOperationNotFound
}

func (repo *ytdlpMetadataOperationRepo) Save(_ context.Context, item library.LibraryOperation) error {
	repo.saved = append(repo.saved, item)
	return nil
}

func (repo *ytdlpMetadataOperationRepo) Delete(_ context.Context, _ string) error {
	return nil
}

type ytdlpMetadataHistoryRepo struct {
	saved []library.HistoryRecord
}

func (repo *ytdlpMetadataHistoryRepo) ListByLibraryID(_ context.Context, libraryID string) ([]library.HistoryRecord, error) {
	result := make([]library.HistoryRecord, 0)
	for _, item := range repo.saved {
		if item.LibraryID == libraryID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo *ytdlpMetadataHistoryRepo) Get(_ context.Context, id string) (library.HistoryRecord, error) {
	for _, item := range repo.saved {
		if item.ID == id {
			return item, nil
		}
	}
	return library.HistoryRecord{}, library.ErrHistoryRecordNotFound
}

func (repo *ytdlpMetadataHistoryRepo) Save(_ context.Context, item library.HistoryRecord) error {
	repo.saved = append(repo.saved, item)
	return nil
}

func (repo *ytdlpMetadataHistoryRepo) Delete(_ context.Context, _ string) error {
	return nil
}

func (repo *ytdlpMetadataHistoryRepo) DeleteByOperationID(_ context.Context, _ string) error {
	return nil
}

func TestApplyYTDLPMetadataPopulatesThumbnailURLFromInfo(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	operation := library.LibraryOperation{}
	request := dto.CreateYTDLPJobRequest{}

	service.applyYTDLPMetadata(&operation, &request, map[string]any{
		"info": map[string]any{
			"thumbnail": "https://example.com/thumb.jpg",
		},
	})

	if request.ThumbnailURL != "https://example.com/thumb.jpg" {
		t.Fatalf("expected thumbnail url to be filled from metadata, got %q", request.ThumbnailURL)
	}
}

func TestOperationDTOsExposeExistingThumbnailPreviewPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	previewPath := filepath.Join(tempDir, ".thumbnail-prefetch", "op-1", "thumbnail.jpg")
	if err := os.MkdirAll(filepath.Dir(previewPath), 0o755); err != nil {
		t.Fatalf("mkdir preview dir: %v", err)
	}
	if err := os.WriteFile(previewPath, []byte("fake-jpg"), 0o644); err != nil {
		t.Fatalf("write preview thumbnail: %v", err)
	}
	outputJSON, changed := withOperationThumbnailPreviewPath("{}", previewPath)
	if !changed {
		t.Fatalf("expected preview path to change operation output")
	}

	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	operation := library.LibraryOperation{
		ID:          "op-1",
		LibraryID:   "lib-1",
		Kind:        "download",
		Status:      library.OperationStatusRunning,
		DisplayName: "Episode",
		InputJSON:   `{"url":"https://example.com/watch?v=episode","caller":"test"}`,
		OutputJSON:  outputJSON,
		CreatedAt:   now,
	}

	full := toOperationDTO(operation)
	if full.ThumbnailPreviewPath != previewPath {
		t.Fatalf("expected full operation preview path %q, got %q", previewPath, full.ThumbnailPreviewPath)
	}
	listItem := toOperationListItemDTO(operation, "Library")
	if listItem.ThumbnailPreviewPath != previewPath {
		t.Fatalf("expected list operation preview path %q, got %q", previewPath, listItem.ThumbnailPreviewPath)
	}
	if listItem.Request == nil || listItem.Request.URL != "https://example.com/watch?v=episode" {
		t.Fatalf("expected list operation request URL to be exposed, got %+v", listItem.Request)
	}
}

func TestOperationListItemDTOExposesTranscodeRequestPreview(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	operation := library.LibraryOperation{
		ID:          "op-transcode",
		LibraryID:   "lib-1",
		Kind:        "transcode",
		Status:      library.OperationStatusSucceeded,
		DisplayName: "Episode 1080p",
		InputJSON:   `{"inputPath":"/media/source.mov","presetId":"preset-1080p","format":"mp4","videoCodec":"h264","audioCodec":"aac","scale":"1080p","deleteSourceFileAfterTranscode":true}`,
		OutputJSON:  "{}",
		CreatedAt:   now,
	}

	listItem := toOperationListItemDTO(operation, "Library")
	if listItem.Request == nil {
		t.Fatalf("expected list operation request preview")
	}
	if listItem.Request.InputPath != "/media/source.mov" || listItem.Request.Format != "mp4" {
		t.Fatalf("expected transcode request preview to expose source and format, got %+v", listItem.Request)
	}
	if !listItem.Request.DeleteSourceFileAfterTranscode {
		t.Fatalf("expected transcode request preview to expose delete-source flag")
	}
}

func TestOperationListItemDTOExposesFailureReason(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	operation := library.LibraryOperation{
		ID:           "op-failed",
		LibraryID:    "lib-1",
		Kind:         "download",
		Status:       library.OperationStatusFailed,
		DisplayName:  "Failed episode",
		InputJSON:    `{"url":"https://example.com/watch?v=failed"}`,
		OutputJSON:   "{}",
		ErrorCode:    "network_error",
		ErrorMessage: "request timed out",
		CreatedAt:    now,
	}

	listItem := toOperationListItemDTO(operation, "Library")
	if listItem.ErrorCode != "network_error" {
		t.Fatalf("expected list operation error code to be exposed, got %q", listItem.ErrorCode)
	}
	if listItem.ErrorMessage != "request timed out" {
		t.Fatalf("expected list operation error message to be exposed, got %q", listItem.ErrorMessage)
	}
}

func TestYTDLPProgressReporterPublishesThumbnailPreviewPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	previewPath := filepath.Join(tempDir, ".thumbnail-prefetch", "op-1", "thumbnail.jpg")
	if err := os.MkdirAll(filepath.Dir(previewPath), 0o755); err != nil {
		t.Fatalf("mkdir preview dir: %v", err)
	}
	if err := os.WriteFile(previewPath, []byte("fake-jpg"), 0o644); err != nil {
		t.Fatalf("write preview thumbnail: %v", err)
	}

	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	operationRepo := &ytdlpMetadataOperationRepo{}
	service := &LibraryService{operations: operationRepo}
	operation := library.LibraryOperation{
		ID:          "op-1",
		LibraryID:   "lib-1",
		Kind:        "download",
		Status:      library.OperationStatusRunning,
		DisplayName: "Episode",
		OutputJSON:  "{}",
		CreatedAt:   now,
	}
	reporter := newYTDLPProgressReporter(service, &operation)

	reporter.publishThumbnailPreviewPath(previewPath)

	if len(operationRepo.saved) != 1 {
		t.Fatalf("expected operation to be saved once, got %d", len(operationRepo.saved))
	}
	if got := extractOperationThumbnailPreviewPath(operationRepo.saved[0].OutputJSON); got != previewPath {
		t.Fatalf("expected saved preview path %q, got %q", previewPath, got)
	}
}

func TestYTDLPProgressReporterPublishesOutputArtifactPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "Episode.f315.webm")
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	operation := library.LibraryOperation{
		ID:          "op-1",
		LibraryID:   "lib-1",
		Kind:        "download",
		Status:      library.OperationStatusRunning,
		DisplayName: "Episode",
		OutputJSON:  "{}",
		CreatedAt:   now,
	}
	operationRepo := &retryOperationRepo{
		items: map[string]library.LibraryOperation{operation.ID: operation},
	}
	service := &LibraryService{operations: operationRepo}
	reporter := newYTDLPProgressReporter(service, &operation)

	reporter.publishOutputArtifactPath(outputPath)

	storedOperation, err := operationRepo.Get(context.Background(), operation.ID)
	if err != nil {
		t.Fatalf("get stored operation: %v", err)
	}
	paths := collectOperationOutputFileArtifactPaths(storedOperation.OutputJSON)
	if len(paths) != 1 || paths[0] != outputPath {
		t.Fatalf("expected stored output path %q, got %#v", outputPath, paths)
	}
}

func TestYTDLPProgressReporterPreservesPersistedThumbnailPreviewPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	previewPath := filepath.Join(tempDir, ".thumbnail-prefetch", "op-1", "thumbnail.jpg")
	if err := os.MkdirAll(filepath.Dir(previewPath), 0o755); err != nil {
		t.Fatalf("mkdir preview dir: %v", err)
	}
	if err := os.WriteFile(previewPath, []byte("fake-jpg"), 0o644); err != nil {
		t.Fatalf("write preview thumbnail: %v", err)
	}
	outputJSON, changed := withOperationThumbnailPreviewPath("{}", previewPath)
	if !changed {
		t.Fatalf("expected preview path to change operation output")
	}
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	persistedOperation := library.LibraryOperation{
		ID:          "op-1",
		LibraryID:   "lib-1",
		Kind:        "download",
		Status:      library.OperationStatusRunning,
		DisplayName: "Episode",
		InputJSON:   `{"url":"https://example.com/watch?v=episode"}`,
		OutputJSON:  outputJSON,
		CreatedAt:   now,
	}
	operationRepo := &retryOperationRepo{
		items: map[string]library.LibraryOperation{persistedOperation.ID: persistedOperation},
	}
	staleOperation := persistedOperation
	staleOperation.OutputJSON = "{}"
	service := &LibraryService{
		operations: operationRepo,
		nowFunc: func() time.Time {
			return now
		},
	}
	reporter := newYTDLPProgressReporter(service, &staleOperation)

	reporter.persistProgress(nil, nil, nil, "1MiB/s", "1MiB/s")

	storedOperation, err := operationRepo.Get(context.Background(), persistedOperation.ID)
	if err != nil {
		t.Fatalf("get stored operation: %v", err)
	}
	if storedPreviewPath := extractOperationThumbnailPreviewPath(storedOperation.OutputJSON); storedPreviewPath != previewPath {
		t.Fatalf("expected progress save to preserve preview path %q, got %q", previewPath, storedPreviewPath)
	}
}

func TestYTDLPProgressReporterPreservesPersistedOutputArtifactPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "Episode.f315.webm")
	outputJSON, changed := withOperationOutputArtifactPath("{}", outputPath)
	if !changed {
		t.Fatalf("expected output artifact path to change operation output")
	}
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	persistedOperation := library.LibraryOperation{
		ID:          "op-1",
		LibraryID:   "lib-1",
		Kind:        "download",
		Status:      library.OperationStatusRunning,
		DisplayName: "Episode",
		InputJSON:   `{"url":"https://example.com/watch?v=episode"}`,
		OutputJSON:  outputJSON,
		CreatedAt:   now,
	}
	operationRepo := &retryOperationRepo{
		items: map[string]library.LibraryOperation{persistedOperation.ID: persistedOperation},
	}
	staleOperation := persistedOperation
	staleOperation.OutputJSON = "{}"
	service := &LibraryService{
		operations: operationRepo,
		nowFunc: func() time.Time {
			return now
		},
	}
	reporter := newYTDLPProgressReporter(service, &staleOperation)

	reporter.persistProgress(nil, nil, nil, "1MiB/s", "1MiB/s")

	storedOperation, err := operationRepo.Get(context.Background(), persistedOperation.ID)
	if err != nil {
		t.Fatalf("get stored operation: %v", err)
	}
	paths := collectOperationOutputFileArtifactPaths(storedOperation.OutputJSON)
	if len(paths) != 1 || paths[0] != outputPath {
		t.Fatalf("expected progress save to preserve output path %q, got %#v", outputPath, paths)
	}
}

func TestYTDLPProgressReporterDoesNotResurrectCanceledOperation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 10, 5, 0, 0, time.UTC)
	persistedOperation := library.LibraryOperation{
		ID:           "op-1",
		LibraryID:    "lib-1",
		Kind:         "download",
		Status:       library.OperationStatusCanceled,
		ErrorCode:    "canceled",
		ErrorMessage: "operation canceled",
		OutputJSON:   "{}",
		CreatedAt:    now,
	}
	operationRepo := &retryOperationRepo{
		items: map[string]library.LibraryOperation{persistedOperation.ID: persistedOperation},
	}
	staleOperation := persistedOperation
	staleOperation.Status = library.OperationStatusRunning
	staleOperation.ErrorCode = ""
	staleOperation.ErrorMessage = ""
	service := &LibraryService{
		operations: operationRepo,
		nowFunc: func() time.Time {
			return now
		},
	}
	reporter := newYTDLPProgressReporter(service, &staleOperation)

	reporter.persistProgress(nil, nil, nil, "1MiB/s", "1MiB/s")

	storedOperation, err := operationRepo.Get(context.Background(), persistedOperation.ID)
	if err != nil {
		t.Fatalf("get stored operation: %v", err)
	}
	if storedOperation.Status != library.OperationStatusCanceled {
		t.Fatalf("expected canceled operation to stay canceled, got %q", storedOperation.Status)
	}
	if storedOperation.ErrorCode != persistedOperation.ErrorCode {
		t.Fatalf("expected canceled error code to be preserved, got %q", storedOperation.ErrorCode)
	}
}

func TestYTDLPProgressReporterCanAttachOutputArtifactPathAfterCancellation(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "Episode.f315.webm")
	now := time.Date(2026, 4, 1, 10, 6, 0, 0, time.UTC)
	persistedOperation := library.LibraryOperation{
		ID:         "op-1",
		LibraryID:  "lib-1",
		Kind:       "download",
		Status:     library.OperationStatusCanceled,
		OutputJSON: "{}",
		CreatedAt:  now,
	}
	operationRepo := &retryOperationRepo{
		items: map[string]library.LibraryOperation{persistedOperation.ID: persistedOperation},
	}
	staleOperation := persistedOperation
	staleOperation.Status = library.OperationStatusRunning
	service := &LibraryService{
		operations: operationRepo,
		nowFunc: func() time.Time {
			return now
		},
	}
	reporter := newYTDLPProgressReporter(service, &staleOperation)

	reporter.publishOutputArtifactPath(outputPath)

	storedOperation, err := operationRepo.Get(context.Background(), persistedOperation.ID)
	if err != nil {
		t.Fatalf("get stored operation: %v", err)
	}
	if storedOperation.Status != library.OperationStatusCanceled {
		t.Fatalf("expected output path save to keep canceled status, got %q", storedOperation.Status)
	}
	paths := collectOperationOutputFileArtifactPaths(storedOperation.OutputJSON)
	if len(paths) != 1 || paths[0] != outputPath {
		t.Fatalf("expected canceled operation to keep output path %q, got %#v", outputPath, paths)
	}
}

func TestApplyYTDLPMetadataToOperationAndHistoryPersistsBothDisplayNames(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	libraryRepo := &ytdlpMetadataLibraryRepo{
		item: library.Library{
			ID:        "lib-1",
			Name:      "lib-1",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	operationRepo := &ytdlpMetadataOperationRepo{}
	historyRepo := &ytdlpMetadataHistoryRepo{}
	service := &LibraryService{
		libraries:  libraryRepo,
		operations: operationRepo,
		histories:  historyRepo,
		nowFunc: func() time.Time {
			return now.Add(2 * time.Minute)
		},
	}
	operation := library.LibraryOperation{
		ID:          "op-1",
		LibraryID:   "lib-1",
		Kind:        "download",
		Status:      library.OperationStatusRunning,
		DisplayName: "https://example.com/watch?v=1",
		InputJSON:   "{}",
		OutputJSON:  "{}",
		CreatedAt:   now,
	}
	history := library.HistoryRecord{
		ID:          "hist-1",
		LibraryID:   "lib-1",
		Category:    "operation",
		Action:      "download",
		DisplayName: "https://example.com/watch?v=1",
		Status:      "running",
		OccurredAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	request := dto.CreateYTDLPJobRequest{
		URL: "https://example.com/watch?v=1",
	}

	changed := service.applyYTDLPMetadataToOperationAndHistory(
		context.Background(),
		&operation,
		&history,
		&request,
		map[string]any{
			"info": map[string]any{
				"title": "Resolved Title",
			},
		},
	)
	if !changed {
		t.Fatalf("expected metadata update to report a change")
	}
	if operation.DisplayName != "Resolved Title" {
		t.Fatalf("expected operation display name to update, got %q", operation.DisplayName)
	}
	if history.DisplayName != "Resolved Title" {
		t.Fatalf("expected history display name to update, got %q", history.DisplayName)
	}
	if len(operationRepo.saved) != 1 {
		t.Fatalf("expected operation to be saved once, got %d", len(operationRepo.saved))
	}
	if len(historyRepo.saved) != 1 {
		t.Fatalf("expected history to be saved once, got %d", len(historyRepo.saved))
	}
	if historyRepo.saved[0].DisplayName != "Resolved Title" {
		t.Fatalf("expected saved history display name to update, got %q", historyRepo.saved[0].DisplayName)
	}
	if libraryRepo.item.UpdatedAt != now.Add(2*time.Minute) {
		t.Fatalf("expected library updatedAt to be touched, got %s", libraryRepo.item.UpdatedAt.Format(time.RFC3339))
	}
}

func TestDownloadYTDLPThumbnailAndBuildOutputsStoresThumbnailFile(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write([]byte("fake-jpeg"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "youtube", "episode.mp4")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write output file: %v", err)
	}

	files := &subtitleDownloadFileRepo{}
	service := &LibraryService{
		files: files,
		nowFunc: func() time.Time {
			return time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
		},
	}

	thumbnailPath, err := service.downloadYTDLPThumbnail(
		context.Background(),
		nil,
		dto.CreateYTDLPJobRequest{ThumbnailURL: server.URL + "/thumb"},
		outputPath,
	)
	if err != nil {
		t.Fatalf("download thumbnail: %v", err)
	}
	if _, err := os.Stat(thumbnailPath); err != nil {
		t.Fatalf("expected downloaded thumbnail to exist: %v", err)
	}

	primaryFile := library.LibraryFile{
		ID:        "file-primary",
		LibraryID: "lib-1",
		Kind:      library.FileKindVideo,
		Name:      "Episode 01",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: outputPath},
		State:     library.FileState{Status: "active"},
	}
	outputs, err := service.buildYTDLPOutputs(
		context.Background(),
		dto.CreateYTDLPJobRequest{},
		library.LibraryOperation{ID: "op-1", LibraryID: "lib-1"},
		primaryFile,
		time.Date(2026, 3, 25, 11, 59, 0, 0, time.UTC),
		outputPath,
		outputPath,
		[]string{outputPath},
		nil,
		[]string{thumbnailPath},
	)
	if err != nil {
		t.Fatalf("build outputs: %v", err)
	}
	if len(outputs.resolvedThumbnails) != 1 {
		t.Fatalf("expected 1 resolved thumbnail, got %d", len(outputs.resolvedThumbnails))
	}
	if len(files.saved) != 1 {
		t.Fatalf("expected 1 saved thumbnail file, got %d", len(files.saved))
	}
	if files.saved[0].Kind != library.FileKindThumbnail {
		t.Fatalf("expected saved file kind thumbnail, got %q", files.saved[0].Kind)
	}
}

func TestPrefetchedYTDLPThumbnailCanBePromotedToFinalOutputPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write([]byte("fake-png"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	outputTemplate := filepath.Join(tempDir, "xiadown", "yt-dlp", "%(extractor)s", "%(title)s.%(ext)s")
	service := &LibraryService{}

	prefetchedPath, err := service.downloadYTDLPThumbnailPrefetch(
		context.Background(),
		dto.CreateYTDLPJobRequest{ThumbnailURL: server.URL + "/thumb"},
		outputTemplate,
		"op-1",
	)
	if err != nil {
		t.Fatalf("prefetch thumbnail: %v", err)
	}
	if _, err := os.Stat(prefetchedPath); err != nil {
		t.Fatalf("expected prefetched thumbnail to exist: %v", err)
	}
	expectedPrefetchedPath := filepath.Join(tempDir, "xiadown", ".thumbnail-prefetch", "op-1", "thumbnail.png")
	if prefetchedPath != expectedPrefetchedPath {
		t.Fatalf("expected prefetched thumbnail path %q, got %q", expectedPrefetchedPath, prefetchedPath)
	}

	outputPath := filepath.Join(tempDir, "xiadown", "yt-dlp", "youtube", "episode.mp4")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write output file: %v", err)
	}

	promotedPath, err := service.promotePrefetchedYTDLPThumbnail(prefetchedPath, outputPath)
	if err != nil {
		t.Fatalf("promote thumbnail: %v", err)
	}
	if !strings.HasSuffix(promotedPath, filepath.Join("thumbnails", "episode-thumbnail.png")) {
		t.Fatalf("unexpected promoted thumbnail path: %q", promotedPath)
	}
	if _, err := os.Stat(promotedPath); err != nil {
		t.Fatalf("expected promoted thumbnail to exist: %v", err)
	}
	if _, err := os.Stat(prefetchedPath); !os.IsNotExist(err) {
		t.Fatalf("expected prefetched thumbnail to be moved away, stat err=%v", err)
	}
}
