package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

type deleteRelinkRaceFileRepository struct {
	mu    sync.Mutex
	items map[string]library.LibraryFile

	blockGetStarted chan struct{}
	blockGetRelease chan struct{}
	blockGetOnce    bool

	blockListStarted chan struct{}
	blockListRelease chan struct{}
	blockListOnce    bool
}

func (repo *deleteRelinkRaceFileRepository) List(ctx context.Context) ([]library.LibraryFile, error) {
	repo.mu.Lock()
	items := make([]library.LibraryFile, 0, len(repo.items))
	for _, item := range repo.items {
		items = append(items, item)
	}
	shouldBlock := repo.blockListOnce
	repo.blockListOnce = false
	started := repo.blockListStarted
	release := repo.blockListRelease
	repo.mu.Unlock()

	if shouldBlock {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return items, nil
}

func (repo *deleteRelinkRaceFileRepository) ListByLibraryID(_ context.Context, libraryID string) ([]library.LibraryFile, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	items := make([]library.LibraryFile, 0, len(repo.items))
	for _, item := range repo.items {
		if item.LibraryID == libraryID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repo *deleteRelinkRaceFileRepository) Get(ctx context.Context, id string) (library.LibraryFile, error) {
	repo.mu.Lock()
	item, ok := repo.items[id]
	shouldBlock := repo.blockGetOnce
	repo.blockGetOnce = false
	started := repo.blockGetStarted
	release := repo.blockGetRelease
	repo.mu.Unlock()

	if !ok {
		return library.LibraryFile{}, library.ErrFileNotFound
	}
	if shouldBlock {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return library.LibraryFile{}, ctx.Err()
		}
	}
	return item, nil
}

func (repo *deleteRelinkRaceFileRepository) Save(_ context.Context, item library.LibraryFile) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.items == nil {
		repo.items = make(map[string]library.LibraryFile)
	}
	repo.items[item.ID] = item
	return nil
}

func (repo *deleteRelinkRaceFileRepository) Delete(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.items, id)
	return nil
}

func (repo *deleteRelinkRaceFileRepository) item(id string) library.LibraryFile {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	return repo.items[id]
}

func TestMarkLibraryFileDeletedReloadsRelinkCommittedWhileDeleteWaits(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old", "song.mp3")
	newPath := filepath.Join(tempDir, "new", "song.mp3")
	for _, path := range []string{oldPath, newPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(filepath.Dir(path)), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	staleSnapshot := mustNewAudioFile(t, "audio-delete-race", "lib-delete-race", "snapshot-op", oldPath, now)
	current := staleSnapshot
	current.LatestOperationID = "current-op"
	repo := &deleteRelinkRaceFileRepository{
		items:           map[string]library.LibraryFile{current.ID: current},
		blockGetStarted: make(chan struct{}),
		blockGetRelease: make(chan struct{}),
		blockGetOnce:    true,
	}
	service := &LibraryService{
		files:     repo,
		libraries: &deleteRuleLibraryRepo{items: map[string]library.Library{current.LibraryID: mustNewLibrary(t, current.LibraryID, now)}},
		nowFunc:   func() time.Time { return now.Add(time.Minute) },
	}

	relinkDone := make(chan error, 1)
	go func() {
		_, err := service.applyLibraryFileRelink(ctx, staleSnapshot, newPath)
		relinkDone <- err
	}()
	select {
	case <-repo.blockGetStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("relink did not reach its locked repository reload")
	}

	deleteInvoked := make(chan struct{})
	deleteDone := make(chan error, 1)
	go func() {
		close(deleteInvoked)
		deleteDone <- service.markLibraryFileDeleted(ctx, staleSnapshot.ID, true)
	}()
	<-deleteInvoked
	close(repo.blockGetRelease)

	select {
	case err := <-relinkDone:
		if err != nil {
			t.Fatalf("relink: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("relink did not complete")
	}
	select {
	case err := <-deleteDone:
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("delete did not complete")
	}

	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("stale path must not be deleted: %v", err)
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Fatalf("current relinked path must be deleted, got %v", err)
	}
	stored := repo.item(current.ID)
	if stored.Storage.LocalPath != newPath {
		t.Fatalf("stored path rolled back: want %q, got %q", newPath, stored.Storage.LocalPath)
	}
	if stored.LatestOperationID != "current-op" {
		t.Fatalf("latest operation rolled back: want current-op, got %q", stored.LatestOperationID)
	}
	if !stored.State.Deleted || stored.State.Status != "deleted" {
		t.Fatalf("expected current record to be marked deleted, got %#v", stored.State)
	}
}

func TestClearMissingLibraryFilesDoesNotDeleteConcurrentlyRelinkedFile(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 10, 5, 0, 0, time.UTC)
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "missing", "song.mp3")
	newPath := filepath.Join(tempDir, "found", "song.mp3")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	if err := os.WriteFile(newPath, []byte("found"), 0o644); err != nil {
		t.Fatalf("write new: %v", err)
	}

	fileItem := mustNewAudioFile(t, "audio-clear-race", "lib-clear-race", "download-op", oldPath, now)
	repo := &deleteRelinkRaceFileRepository{
		items:            map[string]library.LibraryFile{fileItem.ID: fileItem},
		blockListStarted: make(chan struct{}),
		blockListRelease: make(chan struct{}),
		blockListOnce:    true,
	}
	service := &LibraryService{
		files:     repo,
		libraries: &deleteRuleLibraryRepo{items: map[string]library.Library{fileItem.LibraryID: mustNewLibrary(t, fileItem.LibraryID, now)}},
		nowFunc:   func() time.Time { return now.Add(time.Minute) },
	}

	clearDone := make(chan struct {
		removed int
		err     error
	}, 1)
	go func() {
		response, err := service.ClearMissingLibraryFiles(ctx)
		clearDone <- struct {
			removed int
			err     error
		}{removed: response.Removed, err: err}
	}()
	select {
	case <-repo.blockListStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("clear-missing did not capture its stale list snapshot")
	}

	if _, err := service.applyLibraryFileRelink(ctx, fileItem, newPath); err != nil {
		t.Fatalf("relink: %v", err)
	}
	close(repo.blockListRelease)

	select {
	case result := <-clearDone:
		if result.err != nil {
			t.Fatalf("clear missing: %v", result.err)
		}
		if result.removed != 0 {
			t.Fatalf("expected relinked file not to be removed, got %d", result.removed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("clear-missing did not complete")
	}

	stored := repo.item(fileItem.ID)
	if stored.State.Deleted {
		t.Fatal("concurrently relinked file was marked deleted")
	}
	if stored.Storage.LocalPath != newPath {
		t.Fatalf("stored path rolled back: want %q, got %q", newPath, stored.Storage.LocalPath)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("relinked file should remain on disk: %v", err)
	}
}

func TestTranscodeCleanupMergesLatestOperationWithoutRollingBackRelink(t *testing.T) {
	tests := []struct {
		name                string
		currentOperationID  string
		expectedOperationID string
	}{
		{
			name:                "applies intended transcode operation",
			currentOperationID:  "download-op",
			expectedOperationID: "transcode-op",
		},
		{
			name:                "preserves a newer concurrent operation",
			currentOperationID:  "newer-op",
			expectedOperationID: "newer-op",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 7, 16, 10, 10, 0, 0, time.UTC)
			tempDir := t.TempDir()
			oldPath := filepath.Join(tempDir, "old", "source.mp4")
			newPath := filepath.Join(tempDir, "new", "source.mp4")
			for _, path := range []string{oldPath, newPath} {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
				}
				if err := os.WriteFile(path, []byte(filepath.Dir(path)), 0o644); err != nil {
					t.Fatalf("write %s: %v", path, err)
				}
			}

			staleSnapshot := mustNewVideoFile(t, "video-transcode-race", "lib-transcode-race", "download-op", oldPath, now)
			current := staleSnapshot
			current.Storage.LocalPath = newPath
			current.Name = filepath.Base(newPath)
			current.Metadata.Title = "relinked title"
			current.LatestOperationID = test.currentOperationID
			repo := &deleteRelinkRaceFileRepository{items: map[string]library.LibraryFile{current.ID: current}}
			service := &LibraryService{
				files:     repo,
				libraries: &deleteRuleLibraryRepo{items: map[string]library.Library{current.LibraryID: mustNewLibrary(t, current.LibraryID, now)}},
				nowFunc:   func() time.Time { return now.Add(time.Minute) },
			}

			updated := service.cleanupSourceFileAfterSuccessfulTranscode(ctx, staleSnapshot, "transcode-op")
			if !updated.State.Deleted {
				t.Fatal("expected source to be marked deleted")
			}
			if _, err := os.Stat(oldPath); err != nil {
				t.Fatalf("stale source path must not be deleted: %v", err)
			}
			if _, err := os.Stat(newPath); !os.IsNotExist(err) {
				t.Fatalf("current source path must be deleted, got %v", err)
			}
			stored := repo.item(current.ID)
			if stored.Storage.LocalPath != newPath {
				t.Fatalf("stored path rolled back: want %q, got %q", newPath, stored.Storage.LocalPath)
			}
			if stored.LatestOperationID != test.expectedOperationID {
				t.Fatalf("unexpected latest operation: want %q, got %q", test.expectedOperationID, stored.LatestOperationID)
			}
			if stored.Metadata.Title != "relinked title" {
				t.Fatalf("current metadata was rolled back: %#v", stored.Metadata)
			}
		})
	}
}
