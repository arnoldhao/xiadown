package service

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type relinkListenLocalTrackRepo struct {
	items      map[string]library.ListenLocalTrack
	savedItems []library.ListenLocalTrack
	deletedIDs []string
}

func (repo *relinkListenLocalTrackRepo) List(_ context.Context, options library.ListenLocalTrackListOptions) ([]library.ListenLocalTrack, error) {
	items := make([]library.ListenLocalTrack, 0, len(repo.items))
	for _, item := range repo.items {
		if !options.IncludeUnavailable && item.Availability != library.ListenLocalTrackAvailable {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].FileID < items[j].FileID
	})
	return items, nil
}

func (repo *relinkListenLocalTrackRepo) Get(_ context.Context, fileID string) (library.ListenLocalTrack, error) {
	item, ok := repo.items[fileID]
	if !ok {
		return library.ListenLocalTrack{}, library.ErrFileNotFound
	}
	return item, nil
}

func (repo *relinkListenLocalTrackRepo) Save(_ context.Context, item library.ListenLocalTrack) error {
	if repo.items == nil {
		repo.items = map[string]library.ListenLocalTrack{}
	}
	repo.items[item.FileID] = item
	repo.savedItems = append(repo.savedItems, item)
	return nil
}

func (repo *relinkListenLocalTrackRepo) Delete(_ context.Context, fileID string) error {
	repo.deletedIDs = append(repo.deletedIDs, fileID)
	delete(repo.items, fileID)
	return nil
}

func (repo *relinkListenLocalTrackRepo) DeleteUnavailable(_ context.Context) (int, error) {
	removed := 0
	for fileID, item := range repo.items {
		if item.Availability == library.ListenLocalTrackAvailable {
			continue
		}
		delete(repo.items, fileID)
		repo.deletedIDs = append(repo.deletedIDs, fileID)
		removed++
	}
	return removed, nil
}

func TestScanMissingLibraryFilesFindsMovedFileByNameAndSize(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old", "Episode 01.mp4")
	newPath := filepath.Join(tempDir, "new", "nested", "Episode 01.mp4")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	content := []byte("same downloaded media bytes")
	if err := os.WriteFile(newPath, content, 0o644); err != nil {
		t.Fatalf("write new: %v", err)
	}

	fileItem := mustNewVideoFile(t, "file-1", "lib-1", "op-1", oldPath, now)
	size := int64(len(content))
	fileItem.Media = &library.MediaInfo{Format: "mp4", SizeBytes: &size}
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	service := &LibraryService{
		files: files,
		nowFunc: func() time.Time {
			return now
		},
	}

	result, err := service.ScanMissingLibraryFiles(ctx, dto.ScanMissingLibraryFilesRequest{Directory: filepath.Join(tempDir, "new")})
	if err != nil {
		t.Fatalf("scan missing: %v", err)
	}
	if result.MissingCount != 1 || result.ScannedFiles != 1 {
		t.Fatalf("unexpected scan counts: %#v", result)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected one relink match, got %#v", result.Matches)
	}
	match := result.Matches[0]
	if match.FileID != fileItem.ID || match.NewPath != newPath || match.Confidence != "exact" {
		t.Fatalf("unexpected match: %#v", match)
	}
}

func TestListMissingLibraryFilesRefreshesAlreadyMissingState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 6, 19, 10, 2, 0, 0, time.UTC)
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old", "Episode 01.mp4")
	fileItem := mustNewVideoFile(t, "file-missing", "lib-1", "op-missing", oldPath, now)
	fileItem.State.LastError = missingLocalFileError
	fileItem.State.LastChecked = "2026-06-18T10:02:00Z"
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	service := &LibraryService{
		files: files,
		nowFunc: func() time.Time {
			return now
		},
	}

	result, err := service.ListMissingLibraryFiles(ctx)
	if err != nil {
		t.Fatalf("list missing: %v", err)
	}
	if len(result.Missing) != 1 {
		t.Fatalf("expected one missing file, got %#v", result)
	}
	expectedChecked := now.Format(time.RFC3339)
	if result.Missing[0].LastChecked != expectedChecked {
		t.Fatalf("expected response last checked %q, got %q", expectedChecked, result.Missing[0].LastChecked)
	}
	if files.items[fileItem.ID].State.LastChecked != expectedChecked {
		t.Fatalf("expected stored last checked %q, got %q", expectedChecked, files.items[fileItem.ID].State.LastChecked)
	}
}

func TestApplyLibraryRelinksUpdatesStoredPathAndOperationOutput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 6, 19, 10, 5, 0, 0, time.UTC)
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old", "Episode 02.mp4")
	newPath := filepath.Join(tempDir, "new", "Episode 02.mp4")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	content := []byte("same media bytes for relink")
	if err := os.WriteFile(newPath, content, 0o644); err != nil {
		t.Fatalf("write new: %v", err)
	}

	fileItem := mustNewVideoFile(t, "file-2", "lib-1", "op-2", oldPath, now)
	size := int64(len(content))
	fileItem.Media = &library.MediaInfo{Format: "mp4", SizeBytes: &size}
	operation := mustNewOperation(t, "op-2", "lib-1", nil, now)
	operation.OutputJSON = `{"mainPath":"` + oldPath + `","outputPaths":["` + oldPath + `"]}`

	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	libraries := &deleteRuleLibraryRepo{items: map[string]library.Library{"lib-1": mustNewLibrary(t, "lib-1", now)}}
	service := &LibraryService{
		libraries:  libraries,
		files:      files,
		operations: operations,
		nowFunc: func() time.Time {
			return now
		},
	}

	result, err := service.ApplyLibraryRelinks(ctx, dto.ApplyLibraryRelinksRequest{
		Matches: []dto.LibraryRelinkSelectionDTO{{FileID: fileItem.ID, Path: newPath}},
	})
	if err != nil {
		t.Fatalf("apply relinks: %v", err)
	}
	if result.Relinked != 1 {
		t.Fatalf("expected one relink, got %#v", result)
	}
	stored := files.items[fileItem.ID]
	if stored.Storage.LocalPath != newPath {
		t.Fatalf("expected stored path %q, got %q", newPath, stored.Storage.LocalPath)
	}
	if stored.State.LastError != "" {
		t.Fatalf("expected last error to be cleared, got %q", stored.State.LastError)
	}
	updatedOperation := operations.items[operation.ID]
	if !strings.Contains(updatedOperation.OutputJSON, newPath) || strings.Contains(updatedOperation.OutputJSON, oldPath) {
		t.Fatalf("expected operation output to reference new path only, got %s", updatedOperation.OutputJSON)
	}
}

func TestScanMissingListenLocalFilesFindsMovedAvailableTrack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 6, 19, 10, 7, 0, 0, time.UTC)
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old", "Song 01.m4a")
	newPath := filepath.Join(tempDir, "new", "nested", "Song 01.m4a")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	content := []byte("same local audio bytes")
	if err := os.WriteFile(newPath, content, 0o644); err != nil {
		t.Fatalf("write new: %v", err)
	}

	fileItem := mustNewAudioFile(t, "audio-1", "lib-1", "op-audio-1", oldPath, now)
	size := int64(len(content))
	fileItem.Media = &library.MediaInfo{Format: "m4a", SizeBytes: &size}
	track := mustNewListenLocalTrack(t, fileItem, library.ListenLocalTrackAvailable, now)
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	localTracks := &relinkListenLocalTrackRepo{items: map[string]library.ListenLocalTrack{track.FileID: track}}
	service := &LibraryService{
		files:       files,
		localTracks: localTracks,
		nowFunc: func() time.Time {
			return now
		},
	}

	result, err := service.ScanMissingListenLocalFiles(ctx, dto.ScanMissingLibraryFilesRequest{Directory: filepath.Join(tempDir, "new")})
	if err != nil {
		t.Fatalf("scan listen local missing: %v", err)
	}
	if result.MissingCount != 1 || result.ScannedFiles != 1 {
		t.Fatalf("unexpected scan counts: %#v", result)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected one listen local relink match, got %#v", result.Matches)
	}
	match := result.Matches[0]
	if match.FileID != fileItem.ID || match.NewPath != newPath || match.Confidence != "exact" {
		t.Fatalf("unexpected listen local match: %#v", match)
	}
	if got := localTracks.items[fileItem.ID].Availability; got != library.ListenLocalTrackMissing {
		t.Fatalf("expected missing track state to be persisted, got %q", got)
	}
}

func TestApplyListenLocalRelinksUpdatesOnlyMissingListenLocalTrack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 6, 19, 10, 8, 0, 0, time.UTC)
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old", "Song 02.m4a")
	newPath := filepath.Join(tempDir, "new", "Song 02.m4a")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	content := []byte("same local audio bytes for relink")
	if err := os.WriteFile(newPath, content, 0o644); err != nil {
		t.Fatalf("write new: %v", err)
	}

	fileItem := mustNewAudioFile(t, "audio-2", "lib-1", "op-audio-2", oldPath, now)
	size := int64(len(content))
	fileItem.Media = &library.MediaInfo{Format: "m4a", SizeBytes: &size}
	track := mustNewListenLocalTrack(t, fileItem, library.ListenLocalTrackMissing, now)
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{fileItem.ID: fileItem}}
	localTracks := &relinkListenLocalTrackRepo{items: map[string]library.ListenLocalTrack{track.FileID: track}}
	service := &LibraryService{
		libraries:   &deleteRuleLibraryRepo{items: map[string]library.Library{"lib-1": mustNewLibrary(t, "lib-1", now)}},
		files:       files,
		localTracks: localTracks,
		nowFunc: func() time.Time {
			return now
		},
	}

	result, err := service.ApplyListenLocalRelinks(ctx, dto.ApplyLibraryRelinksRequest{
		Matches: []dto.LibraryRelinkSelectionDTO{{FileID: fileItem.ID, Path: newPath}},
	})
	if err != nil {
		t.Fatalf("apply listen local relinks: %v", err)
	}
	if result.Relinked != 1 {
		t.Fatalf("expected one relink, got %#v", result)
	}
	if got := files.items[fileItem.ID].Storage.LocalPath; got != newPath {
		t.Fatalf("expected stored path %q, got %q", newPath, got)
	}

	otherFile := mustNewAudioFile(t, "audio-3", "lib-1", "op-audio-3", filepath.Join(tempDir, "old", "Song 03.m4a"), now)
	otherSize := int64(len(content))
	otherFile.Media = &library.MediaInfo{Format: "m4a", SizeBytes: &otherSize}
	files.items[otherFile.ID] = otherFile
	if _, err := service.ApplyListenLocalRelinks(ctx, dto.ApplyLibraryRelinksRequest{
		Matches: []dto.LibraryRelinkSelectionDTO{{FileID: otherFile.ID, Path: newPath}},
	}); err == nil {
		t.Fatal("expected non-listen-local file relink to be rejected")
	}
}

func mustNewAudioFile(t *testing.T, id string, libraryID string, operationID string, localPath string, now time.Time) library.LibraryFile {
	t.Helper()
	item, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:        id,
		LibraryID: libraryID,
		Kind:      "audio",
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
		t.Fatalf("new audio file: %v", err)
	}
	return item
}

func mustNewListenLocalTrack(t *testing.T, fileItem library.LibraryFile, availability string, now time.Time) library.ListenLocalTrack {
	t.Helper()
	item, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID:        fileItem.ID,
		LibraryID:     fileItem.LibraryID,
		LocalPath:     fileItem.Storage.LocalPath,
		Title:         strings.TrimSuffix(filepath.Base(fileItem.Storage.LocalPath), filepath.Ext(fileItem.Storage.LocalPath)),
		Format:        mediaFormatFromFile(fileItem),
		SizeBytes:     mediaSizeFromFile(fileItem),
		Availability:  availability,
		LastCheckedAt: &now,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	})
	if err != nil {
		t.Fatalf("new listen local track: %v", err)
	}
	return item
}
