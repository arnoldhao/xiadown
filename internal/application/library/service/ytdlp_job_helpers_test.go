package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	settingsdto "xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/persistence"
	ydlpinfr "xiadown/internal/infrastructure/ytdlp"
)

type subtitleDownloadFileRepo struct {
	saved []library.LibraryFile
}

func (repo *subtitleDownloadFileRepo) List(_ context.Context) ([]library.LibraryFile, error) {
	return nil, nil
}

func (repo *subtitleDownloadFileRepo) ListByLibraryID(_ context.Context, _ string) ([]library.LibraryFile, error) {
	return nil, nil
}

func (repo *subtitleDownloadFileRepo) Get(_ context.Context, _ string) (library.LibraryFile, error) {
	return library.LibraryFile{}, library.ErrFileNotFound
}

func (repo *subtitleDownloadFileRepo) Save(_ context.Context, item library.LibraryFile) error {
	repo.saved = append(repo.saved, item)
	return nil
}

func (repo *subtitleDownloadFileRepo) Delete(_ context.Context, _ string) error {
	return nil
}

type subtitleDownloadDocumentRepo struct {
	saved []library.SubtitleDocument
}

type ytdlpSettingsReader struct {
	settings settingsdto.Settings
	err      error
}

func (reader ytdlpSettingsReader) GetSettings(context.Context) (settingsdto.Settings, error) {
	return reader.settings, reader.err
}

func (repo *subtitleDownloadDocumentRepo) Get(_ context.Context, _ string) (library.SubtitleDocument, error) {
	return library.SubtitleDocument{}, library.ErrSubtitleDocumentNotFound
}

func (repo *subtitleDownloadDocumentRepo) GetByFileID(_ context.Context, _ string) (library.SubtitleDocument, error) {
	return library.SubtitleDocument{}, library.ErrSubtitleDocumentNotFound
}

func (repo *subtitleDownloadDocumentRepo) Save(_ context.Context, document library.SubtitleDocument) error {
	repo.saved = append(repo.saved, document)
	return nil
}

func (repo *subtitleDownloadDocumentRepo) DeleteByFileID(_ context.Context, _ string) error {
	return nil
}

func TestCreateDownloadedSubtitleFileStoresHybridSourceAndSubtitleDocument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()
	subtitlePath := filepath.Join(tempDir, "episode.en.srt")
	content := "1\n00:00:00,000 --> 00:00:01,000\nHello world.\n"
	if err := os.WriteFile(subtitlePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write subtitle: %v", err)
	}

	files := &subtitleDownloadFileRepo{}
	subtitles := &subtitleDownloadDocumentRepo{}
	service := &LibraryService{
		files:     files,
		subtitles: subtitles,
		nowFunc: func() time.Time {
			return time.Date(2026, 3, 24, 8, 0, 0, 0, time.UTC)
		},
	}

	fileItem, err := service.createDownloadedSubtitleFile(
		ctx,
		library.LibraryOperation{ID: "op-1", LibraryID: "lib-1"},
		subtitlePath,
		"Episode 01",
		time.Date(2026, 3, 24, 7, 30, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("create downloaded subtitle: %v", err)
	}

	if fileItem.Storage.Mode != "hybrid" {
		t.Fatalf("expected hybrid storage mode, got %q", fileItem.Storage.Mode)
	}
	if fileItem.Storage.LocalPath != subtitlePath {
		t.Fatalf("expected downloaded subtitle local path to be preserved, got %q", fileItem.Storage.LocalPath)
	}
	if fileItem.Storage.DocumentID == "" {
		t.Fatal("expected downloaded subtitle document id to be set")
	}
	if len(files.saved) != 1 {
		t.Fatalf("expected one file save, got %d", len(files.saved))
	}
	if len(subtitles.saved) != 1 {
		t.Fatalf("expected one subtitle document save, got %d", len(subtitles.saved))
	}
	if subtitles.saved[0].ID != fileItem.Storage.DocumentID || subtitles.saved[0].FileID != fileItem.ID {
		t.Fatalf("expected saved subtitle document to be linked to file, got %#v", subtitles.saved[0])
	}
	if subtitles.saved[0].OriginalContent != content || subtitles.saved[0].WorkingContent != content {
		t.Fatalf("expected subtitle document content to match source file, got %#v", subtitles.saved[0])
	}
	if _, statErr := os.Stat(subtitlePath); statErr != nil {
		t.Fatalf("expected source subtitle file to remain on disk, stat err=%v", statErr)
	}
}

func TestCreateDownloadedSubtitleFilePersistsWithSQLiteStorageConstraint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 3, 24, 8, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	subtitlePath := filepath.Join(tempDir, "episode.en.srt")
	content := "1\n00:00:00,000 --> 00:00:01,000\nHello world.\n"
	if err := os.WriteFile(subtitlePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write subtitle: %v", err)
	}

	db, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(tempDir, "library-ytdlp-subtitle.db"),
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

	service := &LibraryService{files: files, subtitles: subtitles}
	fileItem, err := service.createDownloadedSubtitleFile(
		ctx,
		library.LibraryOperation{ID: "op-1", LibraryID: libraryItem.ID},
		subtitlePath,
		"Episode 01",
		now,
	)
	if err != nil {
		t.Fatalf("create downloaded subtitle: %v", err)
	}

	storedFile, err := files.Get(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("get stored file: %v", err)
	}
	if storedFile.Storage.Mode != "hybrid" || storedFile.Storage.DocumentID == "" || storedFile.Storage.LocalPath != subtitlePath {
		t.Fatalf("expected sqlite subtitle file to keep hybrid storage, got %#v", storedFile.Storage)
	}
	storedDocument, err := subtitles.GetByFileID(ctx, fileItem.ID)
	if err != nil {
		t.Fatalf("get subtitle document: %v", err)
	}
	if storedDocument.OriginalContent != content {
		t.Fatalf("expected stored subtitle document content to match source file, got %q", storedDocument.OriginalContent)
	}
}

func TestPrepareYTDLPOutputUsesSingleXiadownForDefaultDirectory(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	service := &LibraryService{}
	outputTemplate, subtitleTemplate, thumbnailTemplate, err := service.prepareYTDLPOutput(context.Background())
	if err != nil {
		t.Fatalf("prepareYTDLPOutput returned error: %v", err)
	}

	expectedBase := filepath.Join(tempHome, "Downloads", "xiadown", "yt-dlp")
	if filepath.Dir(filepath.Dir(outputTemplate)) != expectedBase {
		t.Fatalf("expected output base %q, got %q", expectedBase, outputTemplate)
	}
	if filepath.Dir(filepath.Dir(filepath.Dir(subtitleTemplate))) != expectedBase {
		t.Fatalf("expected subtitle base %q, got %q", expectedBase, subtitleTemplate)
	}
	if filepath.Base(filepath.Dir(subtitleTemplate)) != "subtitles" {
		t.Fatalf("expected subtitle directory to be subtitles, got %q", subtitleTemplate)
	}
	if filepath.Dir(filepath.Dir(filepath.Dir(thumbnailTemplate))) != expectedBase {
		t.Fatalf("expected thumbnail base %q, got %q", expectedBase, thumbnailTemplate)
	}
}

func TestPrepareYTDLPOutputAddsXiadownUnderCustomParentDirectory(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	downloadDirectory := filepath.Join(tempHome, "Downloads")
	service := &LibraryService{
		settings: ytdlpSettingsReader{
			settings: settingsdto.Settings{DownloadDirectory: downloadDirectory},
		},
	}

	outputTemplate, subtitleTemplate, thumbnailTemplate, err := service.prepareYTDLPOutput(context.Background())
	if err != nil {
		t.Fatalf("prepareYTDLPOutput returned error: %v", err)
	}

	expectedBase := filepath.Join(downloadDirectory, "xiadown", "yt-dlp")
	if filepath.Dir(filepath.Dir(outputTemplate)) != expectedBase {
		t.Fatalf("expected output base %q, got %q", expectedBase, outputTemplate)
	}
	if filepath.Dir(filepath.Dir(filepath.Dir(subtitleTemplate))) != expectedBase {
		t.Fatalf("expected subtitle base %q, got %q", expectedBase, subtitleTemplate)
	}
	if filepath.Base(filepath.Dir(subtitleTemplate)) != "subtitles" {
		t.Fatalf("expected subtitle directory to be subtitles, got %q", subtitleTemplate)
	}
	if filepath.Dir(filepath.Dir(filepath.Dir(thumbnailTemplate))) != expectedBase {
		t.Fatalf("expected thumbnail base %q, got %q", expectedBase, thumbnailTemplate)
	}
}

func TestPersistYTDLPLogsDoesNotWriteLogFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	downloadDirectory := filepath.Join(tempDir, "Downloads", "xiadown")
	service := &LibraryService{
		settings: ytdlpSettingsReader{
			settings: settingsdto.Settings{DownloadDirectory: downloadDirectory},
		},
	}

	snapshot := service.persistYTDLPLogs(
		context.Background(),
		library.LibraryOperation{ID: "op-1"},
		ydlpinfr.RunResult{
			Logs: []ydlpinfr.LogEntry{{
				Timestamp: time.Now(),
				Pipe:      "stderr",
				Line:      "ERROR: sample failure",
			}},
		},
		true,
		true,
		nil,
	)

	if snapshot.Path != "" || snapshot.SizeBytes != 0 || snapshot.LineCount != 0 {
		t.Fatalf("expected empty log snapshot when log persistence is disabled, got %#v", snapshot)
	}
	logDir := filepath.Join(downloadDirectory, "xiadown", "yt-dlp", "logs")
	if _, err := os.Stat(logDir); err == nil {
		t.Fatalf("expected yt-dlp log directory not to be created: %s", logDir)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat log directory: %v", err)
	}
}

func TestPrepareYTDLPSubtitleOutputTemplateCreatesSiblingSubtitlesDirectory(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "youtube", "episode-1080p-id.mp4")
	subtitleTemplate, err := prepareYTDLPSubtitleOutputTemplate(outputPath, "fallback")
	if err != nil {
		t.Fatalf("prepare subtitle output template: %v", err)
	}

	expectedDir := filepath.Join(tempDir, "youtube", "subtitles")
	if filepath.Dir(subtitleTemplate) != expectedDir {
		t.Fatalf("expected subtitle dir %q, got %q", expectedDir, subtitleTemplate)
	}
	if filepath.Base(subtitleTemplate) != "episode-1080p-id.%(ext)s" {
		t.Fatalf("expected concrete subtitle template from output path, got %q", subtitleTemplate)
	}
	if _, err := os.Stat(expectedDir); err != nil {
		t.Fatalf("expected subtitle dir to be created: %v", err)
	}
}

func TestPrepareYTDLPSubtitleOutputTemplateEscapesLiteralPercent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "youtube", "episode 100%-id.mp4")
	subtitleTemplate, err := prepareYTDLPSubtitleOutputTemplate(outputPath, "fallback")
	if err != nil {
		t.Fatalf("prepare subtitle output template: %v", err)
	}
	if filepath.Base(subtitleTemplate) != "episode 100%%-id.%(ext)s" {
		t.Fatalf("expected literal percent to be escaped, got %q", subtitleTemplate)
	}
}

func TestBuildYTDLPOutputsIncludesDownloadedSubtitlesInOperationMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "youtube", "episode.mp4")
	subtitlePath := filepath.Join(tempDir, "youtube", "subtitles", "episode.en.vtt")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(subtitlePath), 0o755); err != nil {
		t.Fatalf("mkdir subtitle dir: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write output file: %v", err)
	}
	if err := os.WriteFile(subtitlePath, []byte("WEBVTT\n\n00:00.000 --> 00:01.000\nHello\n"), 0o644); err != nil {
		t.Fatalf("write subtitle file: %v", err)
	}

	files := &subtitleDownloadFileRepo{}
	subtitles := &subtitleDownloadDocumentRepo{}
	service := &LibraryService{files: files, subtitles: subtitles}
	videoSize := int64(5)
	primaryFile := library.LibraryFile{
		ID:        "file-video",
		LibraryID: "lib-1",
		Kind:      library.FileKindVideo,
		Name:      "Episode",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: outputPath},
		Media:     &library.MediaInfo{Format: "mp4", SizeBytes: &videoSize},
		State:     library.FileState{Status: "active"},
	}
	started := time.Now().Add(-time.Minute)
	finished := time.Now()

	snapshot, err := service.buildYTDLPOutputs(
		ctx,
		dto.CreateYTDLPJobRequest{},
		library.LibraryOperation{ID: "op-1", LibraryID: "lib-1"},
		primaryFile,
		started,
		outputPath,
		outputPath,
		[]string{outputPath},
		[]string{subtitlePath},
		nil,
	)
	if err != nil {
		t.Fatalf("build outputs: %v", err)
	}

	metrics := buildOperationMetricsForOperation(snapshot.files, &started, &finished)
	if metrics.FileCount != 2 {
		t.Fatalf("expected video and subtitle to be counted, got %#v", metrics)
	}
	foundSubtitle := false
	for _, output := range snapshot.outputFiles {
		if output.Kind == string(library.FileKindSubtitle) {
			foundSubtitle = true
			break
		}
	}
	if !foundSubtitle {
		t.Fatalf("expected operation output files to include subtitle, got %#v", snapshot.outputFiles)
	}
	if len(files.saved) != 1 || files.saved[0].Kind != library.FileKindSubtitle {
		t.Fatalf("expected saved subtitle file, got %#v", files.saved)
	}
	if len(subtitles.saved) != 1 {
		t.Fatalf("expected saved subtitle document, got %#v", subtitles.saved)
	}
}
