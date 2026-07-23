package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/events"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type memoryListenLocalMetadataTrackRepository struct {
	item       library.ListenLocalTrack
	saveErrors []error
	saveCalls  int
}

type memoryListenLocalMetadataFileRepository struct {
	item       library.LibraryFile
	getCalls   int
	saveCalls  int
	saveErrors []error
}

func (repo *memoryListenLocalMetadataFileRepository) List(context.Context) ([]library.LibraryFile, error) {
	return []library.LibraryFile{repo.item}, nil
}

func (repo *memoryListenLocalMetadataFileRepository) ListByLibraryID(
	_ context.Context,
	libraryID string,
) ([]library.LibraryFile, error) {
	if repo.item.LibraryID != libraryID {
		return []library.LibraryFile{}, nil
	}
	return []library.LibraryFile{repo.item}, nil
}

func (repo *memoryListenLocalMetadataFileRepository) Get(
	_ context.Context,
	id string,
) (library.LibraryFile, error) {
	repo.getCalls++
	if repo.item.ID != id {
		return library.LibraryFile{}, library.ErrFileNotFound
	}
	return repo.item, nil
}

func (repo *memoryListenLocalMetadataFileRepository) Save(
	_ context.Context,
	item library.LibraryFile,
) error {
	repo.saveCalls++
	if len(repo.saveErrors) > 0 {
		err := repo.saveErrors[0]
		repo.saveErrors = repo.saveErrors[1:]
		if err != nil {
			return err
		}
	}
	repo.item = item
	return nil
}

func (*memoryListenLocalMetadataFileRepository) Delete(context.Context, string) error {
	return nil
}

type memoryListenLocalMetadataLibraryRepository struct {
	item library.Library
}

func (repo *memoryListenLocalMetadataLibraryRepository) List(context.Context) ([]library.Library, error) {
	return []library.Library{repo.item}, nil
}

func (repo *memoryListenLocalMetadataLibraryRepository) Get(
	_ context.Context,
	id string,
) (library.Library, error) {
	if repo.item.ID != id {
		return library.Library{}, library.ErrLibraryNotFound
	}
	return repo.item, nil
}

func (repo *memoryListenLocalMetadataLibraryRepository) Save(
	_ context.Context,
	item library.Library,
) error {
	repo.item = item
	return nil
}

func (*memoryListenLocalMetadataLibraryRepository) Delete(context.Context, string) error {
	return nil
}

type recordingListenLocalCatalogMetadataSynchronizer struct {
	file     library.LibraryFile
	metadata dto.UpdateListenLocalTrackMetadataRequest
	calls    int
	err      error
}

func (sync *recordingListenLocalCatalogMetadataSynchronizer) SyncListenLocalTrackMetadata(
	_ context.Context,
	file library.LibraryFile,
	metadata dto.UpdateListenLocalTrackMetadataRequest,
) error {
	sync.calls++
	sync.file = file
	sync.metadata = metadata
	return sync.err
}

func (repo *memoryListenLocalMetadataTrackRepository) List(
	context.Context,
	library.ListenLocalTrackListOptions,
) ([]library.ListenLocalTrack, error) {
	return []library.ListenLocalTrack{repo.item}, nil
}

func (repo *memoryListenLocalMetadataTrackRepository) Get(
	_ context.Context,
	fileID string,
) (library.ListenLocalTrack, error) {
	if repo.item.FileID != fileID {
		return library.ListenLocalTrack{}, library.ErrFileNotFound
	}
	return repo.item, nil
}

func (repo *memoryListenLocalMetadataTrackRepository) Save(
	_ context.Context,
	item library.ListenLocalTrack,
) error {
	repo.saveCalls++
	if len(repo.saveErrors) > 0 {
		err := repo.saveErrors[0]
		repo.saveErrors = repo.saveErrors[1:]
		if err != nil {
			return err
		}
	}
	repo.item = item
	return nil
}

func (*memoryListenLocalMetadataTrackRepository) Delete(context.Context, string) error {
	return nil
}

func (*memoryListenLocalMetadataTrackRepository) DeleteUnavailable(context.Context) (int, error) {
	return 0, nil
}

func TestUpdateListenLocalTrackMetadataPersistsCompleteEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte("fixture"), 0o640); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	createdAt := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: "track-1", LibraryID: "library-1", LocalPath: path, Title: "Old title",
		Availability: library.ListenLocalTrackAvailable, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new track: %v", err)
	}
	repository := &memoryListenLocalMetadataTrackRepository{item: track}
	fileRepository := &memoryListenLocalMetadataFileRepository{item: library.LibraryFile{
		ID: track.FileID, LibraryID: track.LibraryID, Kind: library.FileKindAudio,
		Name: "track.mp3", DisplayName: "Old title",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: path},
		Metadata:  library.FileMetadata{Title: "Old title", Author: "Old artist"},
		Media:     &library.MediaInfo{Format: "mp3"},
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}}
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: track.LibraryID, Name: "Music", CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}
	libraryRepository := &memoryListenLocalMetadataLibraryRepository{item: libraryItem}
	catalogSync := &recordingListenLocalCatalogMetadataSynchronizer{}
	eventBus := events.NewInMemoryBus()
	var publishedFile dto.LibraryFileDTO
	unsubscribe := eventBus.Subscribe(libraryTopicFile, func(event events.Event) {
		publishedFile, _ = event.Payload.(dto.LibraryFileDTO)
	})
	defer unsubscribe()
	writeCalls := 0
	service := &LibraryService{
		libraries:           libraryRepository,
		files:               fileRepository,
		localTracks:         repository,
		catalogMetadataSync: catalogSync,
		bus:                 eventBus,
		nowFunc:             func() time.Time { return updatedAt },
		localMetadataWriter: func(_ context.Context, gotPath string, request dto.UpdateListenLocalTrackMetadataRequest) error {
			writeCalls++
			if gotPath != path || request.Title != "New title" || request.Author != "New artist" {
				t.Fatalf("unexpected writer request: path=%q request=%#v", gotPath, request)
			}
			return nil
		},
	}

	result, err := service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
		FileID: " track-1 ", Title: " New title ", Author: " New artist ", Album: "Album",
		AlbumArtist: "Album artist", Genre: "Pop", TrackNumber: 3, DiscNumber: 2, Year: 2026,
	})
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if writeCalls != 1 {
		t.Fatalf("expected one physical write, got %d", writeCalls)
	}
	if result.Title != "New title" || result.Author != "New artist" || result.Album != "Album" ||
		result.AlbumArtist != "Album artist" || result.Genre != "Pop" || result.TrackNumber != 3 ||
		result.DiscNumber != 2 || result.Year != 2026 || !result.MetadataWritable {
		t.Fatalf("unexpected response: %#v", result)
	}
	if repository.item.UpdatedAt != updatedAt || repository.item.ProbeError != "" ||
		repository.item.Availability != library.ListenLocalTrackAvailable {
		t.Fatalf("unexpected persisted state: %#v", repository.item)
	}
	if fileRepository.item.DisplayName != "New title" ||
		fileRepository.item.Metadata.Title != "New title" ||
		fileRepository.item.Metadata.Author != "New artist" ||
		fileRepository.item.Media == nil || fileRepository.item.Media.SizeBytes == nil ||
		*fileRepository.item.Media.SizeBytes != int64(len("fixture")) ||
		fileRepository.item.UpdatedAt != updatedAt {
		t.Fatalf("Library file metadata was not synchronized: %#v", fileRepository.item)
	}
	if libraryRepository.item.UpdatedAt != updatedAt {
		t.Fatalf("Library version was not touched: %#v", libraryRepository.item)
	}
	if catalogSync.calls != 1 || catalogSync.file.Metadata.Title != "New title" ||
		catalogSync.metadata.Album != "Album" || catalogSync.metadata.AlbumArtist != "Album artist" ||
		catalogSync.metadata.Genre != "Pop" || catalogSync.metadata.TrackNumber != 3 ||
		catalogSync.metadata.DiscNumber != 2 || catalogSync.metadata.Year != 2026 {
		t.Fatalf("Catalog metadata was not synchronized completely: %#v", catalogSync)
	}
	if publishedFile.ID != track.FileID || publishedFile.LibraryID != track.LibraryID ||
		publishedFile.DisplayName != "New title" || publishedFile.Metadata.Author != "New artist" {
		t.Fatalf("Library file event was not published with current metadata: %#v", publishedFile)
	}
}

func TestUpdateListenLocalTrackMetadataMergesFreshLibraryFileAfterConcurrentChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	concurrentAt := createdAt.Add(time.Minute)
	updatedAt := createdAt.Add(2 * time.Minute)
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: "track-1", LibraryID: "library-1", LocalPath: path, Title: "Old",
		Availability: library.ListenLocalTrackAvailable, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	oldSize := int64(99)
	files := &memoryListenLocalMetadataFileRepository{item: library.LibraryFile{
		ID: track.FileID, LibraryID: track.LibraryID, Kind: library.FileKindAudio,
		Name: "track.mp3", DisplayName: "Old display name",
		Storage:  library.FileStorage{Mode: "local_path", LocalPath: path},
		Metadata: library.FileMetadata{Title: "Old", Author: "Old artist", Extractor: "keep-extractor"},
		Media:    &library.MediaInfo{Format: "mp3", Codec: "old-codec", SizeBytes: &oldSize},
		State:    library.FileState{Status: "ready"}, CreatedAt: createdAt, UpdatedAt: createdAt,
	}}
	catalogSync := &recordingListenLocalCatalogMetadataSynchronizer{}
	service := &LibraryService{
		files: files, localTracks: &memoryListenLocalMetadataTrackRepository{item: track},
		catalogMetadataSync: catalogSync,
		nowFunc:             func() time.Time { return updatedAt },
		localMetadataWriter: func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error {
			// Simulate RenameFile plus an independent probe/state update while the
			// physical tag writer is running.
			width := 1920
			probeSize := int64(1234)
			files.item.DisplayName = "Renamed concurrently"
			files.item.Storage.DocumentID = "keep-document"
			files.item.State.Archived = true
			files.item.State.LastError = "keep-state-error"
			files.item.Media = &library.MediaInfo{
				Format: "mp3", Codec: "newer-probe-codec", Width: &width, SizeBytes: &probeSize,
			}
			files.item.UpdatedAt = concurrentAt
			return nil
		},
	}

	_, err = service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
		FileID: track.FileID, Title: "Edited title", Author: "Edited artist", Album: "Album",
	})
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	got := files.item
	if files.getCalls < 2 {
		t.Fatalf("LibraryFile was not reloaded after the physical write: get calls=%d", files.getCalls)
	}
	if got.DisplayName != "Renamed concurrently" || got.Storage.DocumentID != "keep-document" ||
		got.Metadata.Title != "Edited title" || got.Metadata.Author != "Edited artist" ||
		got.Metadata.Extractor != "keep-extractor" {
		t.Fatalf("fresh LibraryFile fields were overwritten: %#v", got)
	}
	if !got.State.Archived || got.State.LastError != "keep-state-error" || got.State.Status != "ready" ||
		got.State.LastChecked != updatedAt.Format(time.RFC3339) {
		t.Fatalf("fresh LibraryFile state was overwritten: %#v", got.State)
	}
	if got.Media == nil || got.Media.Codec != "newer-probe-codec" || got.Media.Width == nil ||
		*got.Media.Width != 1920 || got.Media.SizeBytes == nil || *got.Media.SizeBytes != int64(len("fixture")) {
		t.Fatalf("fresh LibraryFile media was overwritten: %#v", got.Media)
	}
	if got.UpdatedAt != updatedAt || catalogSync.calls != 1 ||
		catalogSync.file.DisplayName != "Renamed concurrently" ||
		catalogSync.file.Metadata.Title != "Edited title" {
		t.Fatalf("Catalog sync did not receive the merged current file: file=%#v sync=%#v", got, catalogSync)
	}
}

func TestUpdateListenLocalTrackMetadataRevalidatesLibraryFileAfterPhysicalWrite(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*library.LibraryFile, string)
	}{
		{name: "deleted", mutate: func(file *library.LibraryFile, _ string) { file.State.Deleted = true }},
		{name: "relinked", mutate: func(file *library.LibraryFile, directory string) {
			file.Storage.LocalPath = filepath.Join(directory, "replacement.mp3")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "track.mp3")
			if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
			track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
				FileID: "track-1", LibraryID: "library-1", LocalPath: path, Title: "Old",
				Availability: library.ListenLocalTrackAvailable, CreatedAt: &now, UpdatedAt: &now,
			})
			if err != nil {
				t.Fatal(err)
			}
			files := &memoryListenLocalMetadataFileRepository{item: library.LibraryFile{
				ID: track.FileID, LibraryID: track.LibraryID, Kind: library.FileKindAudio,
				Name: "track.mp3", DisplayName: "Old",
				Storage:   library.FileStorage{Mode: "local_path", LocalPath: path},
				CreatedAt: now, UpdatedAt: now,
			}}
			catalogSync := &recordingListenLocalCatalogMetadataSynchronizer{}
			service := &LibraryService{
				files: files, localTracks: &memoryListenLocalMetadataTrackRepository{item: track},
				catalogMetadataSync: catalogSync,
				localMetadataWriter: func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error {
					test.mutate(&files.item, directory)
					return nil
				},
			}
			_, err = service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
				FileID: track.FileID, Title: "Edited",
			})
			if !errors.Is(err, library.ErrListenLocalMetadataIndexStale) ||
				!errors.Is(err, library.ErrListenLocalFileChanged) {
				t.Fatalf("expected committed-write file-change error, got %v", err)
			}
			if files.saveCalls != 0 || catalogSync.calls != 0 {
				t.Fatalf("changed LibraryFile was persisted/projected: saves=%d syncs=%d", files.saveCalls, catalogSync.calls)
			}
		})
	}
}

func TestUpdateListenLocalTrackMetadataPublishesFileAfterCatalogSyncFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: "track-1", LibraryID: "library-1", LocalPath: path, Title: "Old",
		Availability: library.ListenLocalTrackAvailable, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := &memoryListenLocalMetadataFileRepository{item: library.LibraryFile{
		ID: track.FileID, LibraryID: track.LibraryID, Kind: library.FileKindAudio,
		Name: "track.mp3", DisplayName: "Old",
		Storage:  library.FileStorage{Mode: "local_path", LocalPath: path},
		Metadata: library.FileMetadata{Title: "Old"}, CreatedAt: now, UpdatedAt: now,
	}}
	eventBus := events.NewInMemoryBus()
	var published dto.LibraryFileDTO
	eventBus.Subscribe(libraryTopicFile, func(event events.Event) {
		published, _ = event.Payload.(dto.LibraryFileDTO)
	})
	service := &LibraryService{
		files:       files,
		localTracks: &memoryListenLocalMetadataTrackRepository{item: track},
		catalogMetadataSync: &recordingListenLocalCatalogMetadataSynchronizer{
			err: errors.New("catalog unavailable"),
		},
		bus: eventBus,
		localMetadataWriter: func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error {
			return nil
		},
		nowFunc: func() time.Time { return now.Add(time.Minute) },
	}

	_, err = service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
		FileID: track.FileID, Title: "New", Author: "Artist",
	})
	if !errors.Is(err, library.ErrListenLocalMetadataIndexStale) {
		t.Fatalf("expected recoverable stale-index error, got %v", err)
	}
	if files.item.Metadata.Title != "New" || published.ID != track.FileID ||
		published.Metadata.Title != "New" || published.Metadata.Author != "Artist" {
		t.Fatalf("committed Library metadata was not published after Catalog failure: file=%#v event=%#v", files.item, published)
	}
}

func TestUpdateListenLocalTrackMetadataRejectsUnsafeInputsBeforeWrite(t *testing.T) {
	tests := []struct {
		name    string
		request dto.UpdateListenLocalTrackMetadataRequest
	}{
		{name: "missing title", request: dto.UpdateListenLocalTrackMetadataRequest{FileID: "track-1"}},
		{name: "invalid year", request: dto.UpdateListenLocalTrackMetadataRequest{FileID: "track-1", Title: "Song", Year: 99}},
		{name: "invalid track", request: dto.UpdateListenLocalTrackMetadataRequest{FileID: "track-1", Title: "Song", TrackNumber: -1}},
		{name: "nul byte", request: dto.UpdateListenLocalTrackMetadataRequest{FileID: "track-1", Title: "Song\x00Title"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &LibraryService{localTracks: &memoryListenLocalMetadataTrackRepository{}}
			_, err := service.UpdateListenLocalTrackMetadata(context.Background(), test.request)
			if !errors.Is(err, library.ErrInvalidListenLocalMetadata) {
				t.Fatalf("expected invalid metadata, got %v", err)
			}
		})
	}
}

func TestUpdateListenLocalTrackMetadataRejectsUnsupportedContainer(t *testing.T) {
	createdAt := time.Now().UTC()
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: "track-1", LibraryID: "library-1", LocalPath: filepath.Join(t.TempDir(), "track.wav"),
		Title: "Song", Availability: library.ListenLocalTrackAvailable, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new track: %v", err)
	}
	called := false
	service := &LibraryService{
		localTracks: &memoryListenLocalMetadataTrackRepository{item: track},
		localMetadataWriter: func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error {
			called = true
			return nil
		},
	}
	_, err = service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
		FileID: track.FileID, Title: "New title",
	})
	if !errors.Is(err, library.ErrListenLocalMetadataUnsupported) || called {
		t.Fatalf("expected unsupported container before writer, err=%v called=%t", err, called)
	}
}

func TestUpdateListenLocalTrackMetadataRetriesIndexSaveAfterCommittedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	createdAt := time.Now().UTC()
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: "track-1", LibraryID: "library-1", LocalPath: path, Title: "Old",
		Availability: library.ListenLocalTrackAvailable, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new track: %v", err)
	}
	repository := &memoryListenLocalMetadataTrackRepository{
		item: track, saveErrors: []error{errors.New("database busy")},
	}
	service := &LibraryService{
		localTracks: repository,
		localMetadataWriter: func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error {
			return nil
		},
	}
	result, err := service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
		FileID: track.FileID, Title: "New",
	})
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if repository.saveCalls != 2 || result.Title != "New" || repository.item.Title != "New" {
		t.Fatalf("expected one successful retry, calls=%d result=%#v item=%#v", repository.saveCalls, result, repository.item)
	}
}

func TestUpdateListenLocalTrackMetadataReportsRecoverableStaleIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	createdAt := time.Now().UTC()
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: "track-1", LibraryID: "library-1", LocalPath: path, Title: "Old",
		Availability: library.ListenLocalTrackAvailable, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new track: %v", err)
	}
	repository := &memoryListenLocalMetadataTrackRepository{
		item: track,
		saveErrors: []error{
			errors.New("database unavailable"),
			errors.New("database unavailable"),
		},
	}
	service := &LibraryService{
		localTracks: repository,
		localMetadataWriter: func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error {
			return nil
		},
	}
	_, err = service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
		FileID: track.FileID, Title: "New",
	})
	if !errors.Is(err, library.ErrListenLocalMetadataIndexStale) || repository.saveCalls != 2 {
		t.Fatalf("expected explicit stale-index error after retry, calls=%d err=%v", repository.saveCalls, err)
	}
}

func TestMetadataWriteAndRefreshCannotEndWithStaleProbeTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	createdAt := time.Now().UTC()
	fileItem := library.LibraryFile{
		ID: "track-1", LibraryID: "library-1", Kind: library.FileKindAudio, Name: "track.mp3",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: path}, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: fileItem.ID, LibraryID: fileItem.LibraryID, LocalPath: path, Title: "Old",
		Availability: library.ListenLocalTrackAvailable, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new track: %v", err)
	}
	tracks := &listenLocalRefreshTrackRepository{items: map[string]library.ListenLocalTrack{track.FileID: track}}
	writerStarted := make(chan struct{})
	releaseWriter := make(chan struct{})
	service := &LibraryService{
		files:       &listenLocalRefreshFileRepository{items: []library.LibraryFile{fileItem}},
		localTracks: tracks,
		localMetadataWriter: func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error {
			close(writerStarted)
			<-releaseWriter
			return nil
		},
	}

	updateDone := make(chan error, 1)
	go func() {
		_, err := service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
			FileID: track.FileID, Title: "New",
		})
		updateDone <- err
	}()
	<-writerStarted

	refreshDone := make(chan error, 1)
	go func() {
		_, err := service.RefreshListenLocalIndex(context.Background(), dto.RefreshListenLocalIndexRequest{FileID: track.FileID})
		refreshDone <- err
	}()
	select {
	case err := <-refreshDone:
		t.Fatalf("refresh did not wait for in-flight metadata write: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseWriter)
	if err := <-updateDone; err != nil {
		t.Fatalf("metadata update: %v", err)
	}
	if err := <-refreshDone; err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := tracks.item(track.FileID); got.Title != "New" || got.ProbeError == "" {
		t.Fatalf("stale refresh overwrote metadata edit: %#v", got)
	}
}

func TestUpdateListenLocalTrackMetadataRejectsRelinkedPathSnapshot(t *testing.T) {
	directory := t.TempDir()
	oldPath := filepath.Join(directory, "old.mp3")
	newPath := filepath.Join(directory, "new.mp3")
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old fixture: %v", err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o600); err != nil {
		t.Fatalf("write new fixture: %v", err)
	}
	createdAt := time.Now().UTC()
	fileItem := library.LibraryFile{
		ID: "track-1", LibraryID: "library-1", Kind: library.FileKindAudio, Name: "new.mp3",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: newPath}, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: fileItem.ID, LibraryID: fileItem.LibraryID, LocalPath: oldPath, Title: "Old",
		Availability: library.ListenLocalTrackAvailable, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new track: %v", err)
	}
	called := false
	service := &LibraryService{
		files:       &listenLocalRefreshFileRepository{items: []library.LibraryFile{fileItem}},
		localTracks: &memoryListenLocalMetadataTrackRepository{item: track},
		localMetadataWriter: func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error {
			called = true
			return nil
		},
	}
	_, err = service.UpdateListenLocalTrackMetadata(context.Background(), dto.UpdateListenLocalTrackMetadataRequest{
		FileID: track.FileID, Title: "New",
	})
	if !errors.Is(err, library.ErrListenLocalFileChanged) || called {
		t.Fatalf("expected relink conflict before physical write, called=%t err=%v", called, err)
	}
}

func TestBuildListenLocalMetadataFFmpegArgsPreservesStreamsAndClearsFields(t *testing.T) {
	args := buildListenLocalMetadataFFmpegArgs("input.mp3", "output.mp3", dto.UpdateListenLocalTrackMetadataRequest{
		Title: "Song", Author: "Artist", TrackNumber: 7,
	})
	for _, expected := range []string{
		"-map", "0", "-map_metadata", "-map_chapters", "-c", "copy",
		"title=Song", "artist=Artist", "album=", "album_artist=", "genre=", "track=7", "disc=", "date=",
		"-id3v2_version", "3", "output.mp3",
	} {
		if !slices.Contains(args, expected) {
			t.Fatalf("expected %q in ffmpeg args: %#v", expected, args)
		}
	}
}

func TestSnapshotListenLocalFileDetectsSameSizeAndModTimeRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	modTime := time.Date(2026, 7, 16, 12, 0, 0, 123, time.UTC)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set original times: %v", err)
	}
	before, err := snapshotListenLocalFile(context.Background(), path)
	if err != nil {
		t.Fatalf("snapshot original: %v", err)
	}
	if err := os.WriteFile(path, []byte("modified"), 0o600); err != nil {
		t.Fatalf("rewrite same-size file: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("restore file times: %v", err)
	}
	after, err := snapshotListenLocalFile(context.Background(), path)
	if err != nil {
		t.Fatalf("snapshot modified: %v", err)
	}
	if sameListenLocalFileSnapshot(before, after) {
		t.Fatal("content digest must detect a rewrite hidden by identical size and mtime")
	}
	changedMode := os.FileMode(0o640)
	if runtime.GOOS == "windows" {
		// Windows only maps the owner-write bit to FILE_ATTRIBUTE_READONLY;
		// changing group bits from 0600 to 0640 would not change file state.
		changedMode = 0o444
	}
	if err := os.Chmod(path, changedMode); err != nil {
		t.Fatalf("change file mode: %v", err)
	}
	modeChanged, err := snapshotListenLocalFile(context.Background(), path)
	if err != nil {
		t.Fatalf("snapshot mode change: %v", err)
	}
	if sameListenLocalFileSnapshot(after, modeChanged) {
		t.Fatal("snapshot must detect an external permission change")
	}
}

func TestWriteListenLocalMetadataRejectsHardLinkWithoutBreakingAlias(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "track.mp3")
	alias := filepath.Join(directory, "alias.mp3")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hard links are unavailable: %v", err)
	}
	service := &LibraryService{}
	err := service.writeListenLocalMetadataWithFFmpeg(context.Background(), path, dto.UpdateListenLocalTrackMetadataRequest{Title: "New"})
	if !errors.Is(err, library.ErrListenLocalMetadataUnsupported) {
		t.Fatalf("expected hard-link preservation rejection, got %v", err)
	}
	left, _ := os.ReadFile(path)
	right, _ := os.ReadFile(alias)
	if !bytes.Equal(left, right) || string(left) != "fixture" {
		t.Fatalf("hard-linked files changed: path=%q alias=%q", left, right)
	}
}

func TestWriteListenLocalMetadataRejectsSymlinkWithoutChangingTarget(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.mp3")
	path := filepath.Join(directory, "track.mp3")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	service := &LibraryService{}
	err := service.writeListenLocalMetadataWithFFmpeg(context.Background(), path, dto.UpdateListenLocalTrackMetadataRequest{Title: "New"})
	if !errors.Is(err, library.ErrListenLocalMetadataUnsupported) {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != "fixture" {
		t.Fatalf("symlink target changed: content=%q err=%v", content, readErr)
	}
}

func TestVerifyListenLocalMetadataPreservedRejectsCustomTagCoverAndChapterLoss(t *testing.T) {
	before := listenLocalMetadataManifest{
		Streams: []listenLocalMetadataManifestStream{
			{CodecType: "audio", CodecName: "mp3", Tags: map[string]string{"CUSTOM": "keep"}},
			{CodecType: "video", CodecName: "mjpeg", Disposition: map[string]int{"attached_pic": 1}},
		},
		Chapters: []listenLocalMetadataManifestChapter{{StartTime: "0", EndTime: "1", Tags: map[string]string{"title": "Intro"}}},
		Format:   listenLocalMetadataManifestFormat{Duration: "1", Tags: map[string]string{"comment": "keep"}},
	}
	after := before
	after.Streams = append([]listenLocalMetadataManifestStream(nil), before.Streams...)
	after.Streams[0].Tags = map[string]string{}
	if err := verifyListenLocalMetadataPreserved(before, after); !errors.Is(err, library.ErrListenLocalMetadataUnsupported) {
		t.Fatalf("expected custom tag loss to be rejected, got %v", err)
	}
	after = before
	after.Streams = before.Streams[:1]
	if err := verifyListenLocalMetadataPreserved(before, after); !errors.Is(err, library.ErrListenLocalMetadataUnsupported) {
		t.Fatalf("expected cover loss to be rejected, got %v", err)
	}
	after = before
	after.Chapters = nil
	if err := verifyListenLocalMetadataPreserved(before, after); !errors.Is(err, library.ErrListenLocalMetadataUnsupported) {
		t.Fatalf("expected chapter loss to be rejected, got %v", err)
	}
}

func TestVerifyListenLocalRawTagsPreservedProtectsFramesFFprobeMayNotExpose(t *testing.T) {
	before := listenLocalRawTagManifest{tags: map[string]any{
		"tit2": "Old title",
		"popm": []byte{1, 2, 3},
		"uslt": "embedded lyrics",
	}}
	after := listenLocalRawTagManifest{tags: map[string]any{
		"tit2": "New title",
		"uslt": "embedded lyrics",
	}}
	if err := verifyListenLocalRawTagsPreserved(before, after); !errors.Is(err, library.ErrListenLocalMetadataUnsupported) {
		t.Fatalf("expected opaque frame loss to be rejected, got %v", err)
	}
	after.tags["popm"] = []byte{1, 2, 3}
	if err := verifyListenLocalRawTagsPreserved(before, after); err != nil {
		t.Fatalf("edited title should be ignored while opaque tags are retained: %v", err)
	}
}

func TestWriteListenLocalMetadataRealContainers(t *testing.T) {
	ffmpegPath := listenLocalMetadataTestFFmpegPath()
	if ffmpegPath == "" {
		t.Skip("ffmpeg is unavailable")
	}
	ffprobePath := filepath.Join(filepath.Dir(ffmpegPath), ffprobeExecutableName())
	var err error
	if _, err := os.Stat(ffprobePath); err != nil {
		ffprobePath, err = exec.LookPath("ffprobe")
	}
	if err != nil {
		t.Skip("matching ffprobe is unavailable")
	}

	tests := []struct {
		name         string
		extension    string
		codec        string
		cover        bool
		chapters     bool
		customTagKey string
		legacyID3v1  bool
	}{
		{name: "MP3", extension: ".mp3", codec: "libmp3lame", cover: true, chapters: true, customTagKey: "custom_tag"},
		{name: "MP3 legacy ID3v1", extension: ".mp3", codec: "libmp3lame", customTagKey: "custom_tag", legacyID3v1: true},
		{name: "M4A", extension: ".m4a", codec: "aac", cover: true, chapters: true},
		{name: "M4B", extension: ".m4b", codec: "aac", cover: true, chapters: true},
		{name: "MP4 audio", extension: ".mp4", codec: "aac", cover: true, chapters: true},
		{name: "FLAC", extension: ".flac", codec: "flac", cover: true, customTagKey: "custom_tag"},
		{name: "Ogg Vorbis", extension: ".ogg", codec: listenLocalMetadataVorbisEncoder(ffmpegPath), customTagKey: "custom_tag"},
		{name: "Ogg audio", extension: ".oga", codec: listenLocalMetadataVorbisEncoder(ffmpegPath), customTagKey: "custom_tag"},
		{name: "Opus", extension: ".opus", codec: "libopus", customTagKey: "custom_tag"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			path := createListenLocalMetadataFixture(t, ffmpegPath, directory, test.extension, test.codec, test.cover, test.chapters, test.customTagKey)
			if test.legacyID3v1 {
				appendListenLocalID3v1Fixture(t, path)
			}
			if err := os.Chmod(path, 0o640); err != nil {
				t.Fatalf("set fixture mode: %v", err)
			}
			before, err := probeListenLocalMetadataManifest(context.Background(), ffprobePath, path)
			if err != nil {
				t.Fatalf("probe fixture: %v", err)
			}
			if test.cover && listenLocalAttachedPictureCount(before) == 0 {
				t.Fatalf("fixture has no attached cover: %#v", before.Streams)
			}
			if test.chapters && len(before.Chapters) == 0 {
				t.Fatal("fixture has no chapter")
			}
			if !listenLocalManifestHasTag(before, "comment", "preserve this comment") {
				t.Fatal("fixture has no preservation comment tag")
			}
			if test.customTagKey != "" && !listenLocalManifestHasTag(before, test.customTagKey, "preserve this custom value") {
				t.Fatalf("fixture has no %q custom tag", test.customTagKey)
			}
			beforeAudioHash := listenLocalDecodedAudioHash(t, ffmpegPath, path)

			service := &LibraryService{tools: &mediaProbeToolResolverStub{
				ready: true, toolDir: filepath.Dir(ffmpegPath), execPath: ffmpegPath,
			}}
			request := dto.UpdateListenLocalTrackMetadataRequest{
				Title: "New title", Author: "New artist", Album: "New album", AlbumArtist: "New album artist",
				Genre: "New genre", TrackNumber: 4, DiscNumber: 2, Year: 2026,
			}
			if err := service.writeListenLocalMetadataWithFFmpeg(context.Background(), path, request); err != nil {
				t.Fatalf("write metadata: %v", err)
			}
			probe, err := service.ffprobeLocalMedia(context.Background(), path)
			if err != nil {
				t.Fatalf("probe updated fixture: %v", err)
			}
			if err := verifyListenLocalMetadata(probe, request); err != nil {
				t.Fatal(err)
			}
			after, err := probeListenLocalMetadataManifest(context.Background(), ffprobePath, path)
			if err != nil {
				t.Fatalf("probe updated manifest: %v", err)
			}
			if err := verifyListenLocalMetadataPreserved(before, after); err != nil {
				t.Fatal(err)
			}
			if afterAudioHash := listenLocalDecodedAudioHash(t, ffmpegPath, path); afterAudioHash != beforeAudioHash {
				t.Fatalf("decoded audio changed: before=%q after=%q", beforeAudioHash, afterAudioHash)
			}
			if !listenLocalManifestHasTag(after, "track", "4/12") {
				t.Fatal("track total was not preserved")
			}
			if !listenLocalManifestHasTag(after, "disc", "2/2") {
				t.Fatal("disc total was not preserved")
			}
			if test.cover && listenLocalAttachedPictureCount(after) != listenLocalAttachedPictureCount(before) {
				t.Fatal("attached cover was not preserved")
			}
			if test.chapters && len(after.Chapters) != len(before.Chapters) {
				t.Fatal("chapters were not preserved")
			}
			if runtime.GOOS != "windows" {
				if mode := mustListenLocalFileMode(t, path).Perm(); mode != 0o640 {
					t.Fatalf("file mode changed to %v", mode)
				}
			}
			if temporary, _ := filepath.Glob(filepath.Join(directory, ".xiadown-metadata-*")); len(temporary) != 0 {
				t.Fatalf("temporary files leaked: %#v", temporary)
			}
		})
	}
}

func listenLocalMetadataTestFFmpegPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "Library", "Application Support", "xiadown", "dependencies", "ffmpeg", "7.1.3-5", ffmpegExecutableName())
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	path, _ := exec.LookPath("ffmpeg")
	return path
}

func listenLocalMetadataVorbisEncoder(ffmpegPath string) string {
	command := exec.Command(ffmpegPath, "-hide_banner", "-encoders")
	output, err := command.CombinedOutput()
	if err == nil && bytes.Contains(output, []byte("libvorbis")) {
		return "libvorbis"
	}
	return "vorbis"
}

func createListenLocalMetadataFixture(
	t *testing.T,
	ffmpegPath string,
	directory string,
	extension string,
	codec string,
	withCover bool,
	withChapters bool,
	customTagKey string,
) string {
	t.Helper()
	basePath := filepath.Join(directory, "base"+extension)
	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=0.4",
		"-c:a", codec, "-ac", "2", "-strict", "-2",
		"-metadata", "title=Old title",
		"-metadata", "artist=Old artist",
		"-metadata", "album=Old album",
		"-metadata", "album_artist=Old album artist",
		"-metadata", "genre=Old genre",
		"-metadata", "track=1/12",
		"-metadata", "disc=1/2",
		"-metadata", "date=2020",
		"-metadata", "comment=preserve this comment",
	}
	if customTagKey != "" {
		args = append(args, "-metadata", customTagKey+"=preserve this custom value")
	}
	runListenLocalFixtureFFmpeg(t, ffmpegPath, append(args, basePath)...)
	currentPath := basePath

	if withCover {
		coverPath := filepath.Join(directory, "cover.jpg")
		runListenLocalFixtureFFmpeg(t, ffmpegPath,
			"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
			"-f", "lavfi", "-i", "color=c=red:s=32x32:d=0.1",
			"-frames:v", "1", coverPath,
		)
		coveredPath := filepath.Join(directory, "covered"+extension)
		runListenLocalFixtureFFmpeg(t, ffmpegPath,
			"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
			"-i", currentPath, "-i", coverPath,
			"-map", "0:a", "-map", "1:v", "-map_metadata", "0",
			"-c", "copy", "-disposition:v:0", "attached_pic", coveredPath,
		)
		currentPath = coveredPath
	}

	if withChapters {
		chapterPath := filepath.Join(directory, "chapters.ffmeta")
		chapterData := []byte(";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=200\ntitle=Intro\n")
		if err := os.WriteFile(chapterPath, chapterData, 0o600); err != nil {
			t.Fatalf("write chapter metadata: %v", err)
		}
		chapteredPath := filepath.Join(directory, "chaptered"+extension)
		runListenLocalFixtureFFmpeg(t, ffmpegPath,
			"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
			"-i", currentPath, "-i", chapterPath,
			"-map", "0", "-map_metadata", "0", "-map_chapters", "1", "-c", "copy", chapteredPath,
		)
		currentPath = chapteredPath
	}

	finalPath := filepath.Join(directory, "track"+extension)
	data, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(finalPath, data, 0o640); err != nil {
		t.Fatalf("write final fixture: %v", err)
	}
	return finalPath
}

func appendListenLocalID3v1Fixture(t *testing.T, path string) {
	t.Helper()
	legacy := make([]byte, 128)
	copy(legacy[0:3], "TAG")
	copy(legacy[3:33], "Legacy title")
	copy(legacy[33:63], "Legacy artist")
	copy(legacy[63:93], "Legacy album")
	copy(legacy[93:97], "1999")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open ID3v1 fixture: %v", err)
	}
	if _, err := file.Write(legacy); err != nil {
		_ = file.Close()
		t.Fatalf("append ID3v1 fixture: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close ID3v1 fixture: %v", err)
	}
}

func runListenLocalFixtureFFmpeg(t *testing.T, ffmpegPath string, args ...string) {
	t.Helper()
	command := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("ffmpeg fixture command failed: %v: %s\nargs: %s", err, stderr.String(), strings.Join(args, " "))
	}
}

func listenLocalDecodedAudioHash(t *testing.T, ffmpegPath string, path string) string {
	t.Helper()
	command := exec.Command(ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-i", path,
		"-map", "0:a:0", "-vn", "-sn", "-dn",
		"-f", "hash", "-hash", "sha256", "-",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("hash decoded audio: %v: %s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func listenLocalAttachedPictureCount(manifest listenLocalMetadataManifest) int {
	count := 0
	for _, stream := range manifest.Streams {
		count += stream.Disposition["attached_pic"]
	}
	return count
}

func listenLocalManifestHasTag(manifest listenLocalMetadataManifest, key string, value string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for tagKey, tagValue := range manifest.Format.Tags {
		if strings.ToLower(strings.TrimSpace(tagKey)) == key && strings.TrimSpace(tagValue) == value {
			return true
		}
	}
	for _, stream := range manifest.Streams {
		for tagKey, tagValue := range stream.Tags {
			if strings.ToLower(strings.TrimSpace(tagKey)) == key && strings.TrimSpace(tagValue) == value {
				return true
			}
		}
	}
	return false
}

func mustListenLocalFileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	return info.Mode()
}

func TestVerifyListenLocalMetadataRequiresEveryRequestedTag(t *testing.T) {
	request := dto.UpdateListenLocalTrackMetadataRequest{
		Title: "Song", Author: "Artist", Album: "Album", AlbumArtist: "Album artist",
		Genre: "Pop", TrackNumber: 1, DiscNumber: 2, Year: 2025,
	}
	probe := mediaProbe{
		Title: "Song", Artist: "Artist", Album: "Album", AlbumArtist: "Album artist",
		Genre: "Pop", TrackNumber: 1, DiscNumber: 2, Year: 2025,
	}
	if err := verifyListenLocalMetadata(probe, request); err != nil {
		t.Fatalf("verify matching metadata: %v", err)
	}
	probe.Artist = "Wrong"
	if err := verifyListenLocalMetadata(probe, request); err == nil {
		t.Fatal("expected mismatched artist to fail verification")
	}
}
