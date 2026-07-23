package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/dependencies"
	"xiadown/internal/domain/library"
)

type listenLocalRefreshFileRepository struct {
	items []library.LibraryFile
}

func (repo *listenLocalRefreshFileRepository) List(context.Context) ([]library.LibraryFile, error) {
	return append([]library.LibraryFile(nil), repo.items...), nil
}

func (repo *listenLocalRefreshFileRepository) ListByLibraryID(_ context.Context, libraryID string) ([]library.LibraryFile, error) {
	items := make([]library.LibraryFile, 0, len(repo.items))
	for _, item := range repo.items {
		if item.LibraryID == libraryID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repo *listenLocalRefreshFileRepository) Get(_ context.Context, id string) (library.LibraryFile, error) {
	for _, item := range repo.items {
		if item.ID == id {
			return item, nil
		}
	}
	return library.LibraryFile{}, library.ErrFileNotFound
}

func (*listenLocalRefreshFileRepository) Save(context.Context, library.LibraryFile) error {
	return nil
}

func (*listenLocalRefreshFileRepository) Delete(context.Context, string) error {
	return nil
}

type listenLocalRefreshTrackRepository struct {
	mu    sync.Mutex
	items map[string]library.ListenLocalTrack
}

type listenLocalMembershipRepository struct {
	mu    sync.Mutex
	items map[string]library.ListenLocalMusicMembership
}

func (repo *listenLocalMembershipRepository) Get(_ context.Context, fileID string) (library.ListenLocalMusicMembership, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	item, ok := repo.items[fileID]
	if !ok {
		return library.ListenLocalMusicMembership{}, library.ErrListenLocalMusicMembershipNotFound
	}
	return item, nil
}

func (repo *listenLocalMembershipRepository) Save(_ context.Context, item library.ListenLocalMusicMembership) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.items == nil {
		repo.items = make(map[string]library.ListenLocalMusicMembership)
	}
	repo.items[item.FileID] = item
	return nil
}

func (repo *listenLocalRefreshTrackRepository) List(context.Context, library.ListenLocalTrackListOptions) ([]library.ListenLocalTrack, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	items := make([]library.ListenLocalTrack, 0, len(repo.items))
	for _, item := range repo.items {
		items = append(items, item)
	}
	return items, nil
}

func (repo *listenLocalRefreshTrackRepository) Get(_ context.Context, fileID string) (library.ListenLocalTrack, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	item, exists := repo.items[fileID]
	if !exists {
		return library.ListenLocalTrack{}, library.ErrFileNotFound
	}
	return item, nil
}

func (repo *listenLocalRefreshTrackRepository) Save(_ context.Context, item library.ListenLocalTrack) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.items == nil {
		repo.items = make(map[string]library.ListenLocalTrack)
	}
	repo.items[item.FileID] = item
	return nil
}

func (repo *listenLocalRefreshTrackRepository) Delete(_ context.Context, fileID string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.items, fileID)
	return nil
}

func (*listenLocalRefreshTrackRepository) DeleteUnavailable(context.Context) (int, error) {
	return 0, nil
}

func (repo *listenLocalRefreshTrackRepository) item(fileID string) library.ListenLocalTrack {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	return repo.items[fileID]
}

type blockingListenLocalProbeResolver struct {
	mu          sync.Mutex
	active      int
	maxActive   int
	started     int
	allStarted  chan struct{}
	startedOnce sync.Once
}

func (*blockingListenLocalProbeResolver) ResolveExecPath(context.Context, dependencies.DependencyName) (string, error) {
	return "", errors.New("unexpected ResolveExecPath call")
}

func (*blockingListenLocalProbeResolver) ResolveDependencyDirectory(context.Context, dependencies.DependencyName) (string, error) {
	return "", errors.New("unexpected ResolveDependencyDirectory call")
}

func (resolver *blockingListenLocalProbeResolver) DependencyReadiness(ctx context.Context, _ dependencies.DependencyName) (bool, string, error) {
	resolver.mu.Lock()
	resolver.active++
	resolver.started++
	if resolver.active > resolver.maxActive {
		resolver.maxActive = resolver.active
	}
	if resolver.active == listenLocalRefreshWorkerCount {
		resolver.startedOnce.Do(func() { close(resolver.allStarted) })
	}
	resolver.mu.Unlock()

	<-ctx.Done()

	resolver.mu.Lock()
	resolver.active--
	resolver.mu.Unlock()
	return false, "", ctx.Err()
}

func (resolver *blockingListenLocalProbeResolver) counts() (started int, maxActive int) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	return resolver.started, resolver.maxActive
}

func TestRefreshListenLocalIndexProbeFailureKeepsExistingTrackAvailable(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "song.mp3")
	if err := os.WriteFile(path, []byte("not valid audio"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	createdAt := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	lastCheckedAt := updatedAt.Add(time.Hour)
	nextCheckedAt := lastCheckedAt.Add(time.Hour)
	durationMs := int64(183_000)
	sizeBytes := int64(42_000)
	fileItem := library.LibraryFile{
		ID: "file-1", LibraryID: "library-1", Kind: library.FileKindAudio,
		Name: "song.mp3", DisplayName: "Song", Storage: library.FileStorage{Mode: "local_path", LocalPath: path},
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: fileItem.ID, LibraryID: fileItem.LibraryID, LocalPath: path,
		Title: "Known title", Author: "Known artist", Album: "Known album", AlbumArtist: "Known album artist",
		Genre: "Known genre", TrackNumber: 7, DiscNumber: 2, Year: 2024,
		CoverLocalPath: filepath.Join(tempDir, "cover.jpg"), Format: "mp3", AudioCodec: "mp3",
		DurationMs: &durationMs, SizeBytes: &sizeBytes, ModTimeUnix: 1234,
		Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &lastCheckedAt,
		CreatedAt: &createdAt, UpdatedAt: &updatedAt,
	})
	if err != nil {
		t.Fatalf("new track: %v", err)
	}
	trackRepo := &listenLocalRefreshTrackRepository{items: map[string]library.ListenLocalTrack{track.FileID: track}}
	service := &LibraryService{
		files:       &listenLocalRefreshFileRepository{items: []library.LibraryFile{fileItem}},
		localTracks: trackRepo,
		nowFunc:     func() time.Time { return nextCheckedAt },
	}

	response, err := service.RefreshListenLocalIndex(context.Background(), dto.RefreshListenLocalIndexRequest{FileID: fileItem.ID})
	if err != nil {
		t.Fatalf("refresh local track: %v", err)
	}
	if response.Scanned != 1 || response.Failed != 1 || response.Missing != 0 || response.Updated != 0 {
		t.Fatalf("unexpected refresh response: %#v", response)
	}

	want := track
	want.LastCheckedAt = nextCheckedAt
	want.ProbeError = "ffmpeg is not installed"
	if got := trackRepo.item(track.FileID); !reflect.DeepEqual(got, want) {
		t.Fatalf("probe failure changed known-good track\n got: %#v\nwant: %#v", got, want)
	}
}

func TestRefreshListenLocalIndexUsesBoundedWorkersAndStopsOnCancel(t *testing.T) {
	tempDir := t.TempDir()
	files := make([]library.LibraryFile, 0, 12)
	for index := range 12 {
		path := filepath.Join(tempDir, fmt.Sprintf("song-%02d.mp3", index))
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatalf("write fixture %d: %v", index, err)
		}
		files = append(files, library.LibraryFile{
			ID: fmt.Sprintf("file-%02d", index), LibraryID: "library-1", Kind: library.FileKindAudio,
			Name: filepath.Base(path), DisplayName: filepath.Base(path),
			Storage: library.FileStorage{Mode: "local_path", LocalPath: path},
		})
	}
	resolver := &blockingListenLocalProbeResolver{allStarted: make(chan struct{})}
	service := &LibraryService{
		files:       &listenLocalRefreshFileRepository{items: files},
		localTracks: &listenLocalRefreshTrackRepository{},
		tools:       resolver,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		response dto.ListenLocalIndexRefreshResponse
		err      error
	}
	resultCh := make(chan result, 1)
	go func() {
		response, err := service.RefreshListenLocalIndex(ctx, dto.RefreshListenLocalIndexRequest{})
		resultCh <- result{response: response, err: err}
	}()

	select {
	case <-resolver.allStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not start the bounded worker pool")
	}
	started, maxActive := resolver.counts()
	if started != listenLocalRefreshWorkerCount || maxActive != listenLocalRefreshWorkerCount {
		t.Fatalf("expected exactly %d concurrent probes before release, started=%d max=%d", listenLocalRefreshWorkerCount, started, maxActive)
	}

	cancel()
	select {
	case got := <-resultCh:
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", got.err)
		}
		if got.response.Scanned != listenLocalRefreshWorkerCount || got.response.Failed != 0 {
			t.Fatalf("unexpected partial response after cancellation: %#v", got.response)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not stop after context cancellation")
	}
	started, maxActive = resolver.counts()
	if started != listenLocalRefreshWorkerCount || maxActive > listenLocalRefreshWorkerCount {
		t.Fatalf("worker pool exceeded its bound: started=%d max=%d", started, maxActive)
	}
}

func TestRefreshListenLocalTrackDiscardsQueuedProbeAfterRelink(t *testing.T) {
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old.mp3")
	newPath := filepath.Join(tempDir, "new.mp3")
	if err := os.WriteFile(newPath, []byte("not valid audio"), 0o600); err != nil {
		t.Fatalf("write relinked fixture: %v", err)
	}
	queuedFile := library.LibraryFile{
		ID: "file-1", LibraryID: "library-1", Kind: library.FileKindAudio,
		Name: "old.mp3", DisplayName: "Old",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: oldPath},
	}
	currentFile := queuedFile
	currentFile.Name = "new.mp3"
	currentFile.DisplayName = "New"
	currentFile.Storage.LocalPath = newPath
	trackRepo := &listenLocalRefreshTrackRepository{items: map[string]library.ListenLocalTrack{}}
	service := &LibraryService{
		files:       &listenLocalRefreshFileRepository{items: []library.LibraryFile{currentFile}},
		localTracks: trackRepo,
	}
	staleProbe := mediaProbe{
		StreamInfo: true,
		HasAudio:   true,
		Format:     "mp3",
		AudioCodec: "mp3",
		Title:      "Old path title",
	}
	response := dto.ListenLocalIndexRefreshResponse{}

	service.refreshListenLocalTrack(
		context.Background(),
		queuedFile,
		&staleProbe,
		listenLocalCoverLookup{},
		&response,
	)

	if _, err := trackRepo.Get(context.Background(), queuedFile.ID); !errors.Is(err, library.ErrFileNotFound) {
		t.Fatalf("stale queued probe was saved for relinked path: %v", err)
	}
	if response.Scanned != 1 || response.Failed != 1 || response.Added != 0 {
		t.Fatalf("unexpected refresh response: %#v", response)
	}
}

func TestListenLocalExplicitRemovalPersistsUserExclusionAndScannerDoesNotResurrect(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "excluded.mp3")
	fileItem := library.LibraryFile{
		ID: "file-excluded", LibraryID: "library-1", Kind: library.FileKindAudio,
		Name: "excluded.mp3", DisplayName: "Excluded",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: path},
		CreatedAt: now, UpdatedAt: now,
	}
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: fileItem.ID, LibraryID: fileItem.LibraryID, LocalPath: path, Title: "Excluded",
		Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	tracks := &listenLocalRefreshTrackRepository{items: map[string]library.ListenLocalTrack{track.FileID: track}}
	memberships := &listenLocalMembershipRepository{}
	service := &LibraryService{
		files:       &listenLocalRefreshFileRepository{items: []library.LibraryFile{fileItem}},
		localTracks: tracks, localMusicMemberships: memberships, nowFunc: func() time.Time { return now },
	}
	if err := service.RemoveListenLocalTrack(context.Background(), dto.RemoveListenLocalTrackRequest{FileID: track.FileID}); err != nil {
		t.Fatal(err)
	}
	membership, err := memberships.Get(context.Background(), track.FileID)
	if err != nil || !membership.IsUserExcluded() {
		t.Fatalf("explicit removal membership = %#v err=%v", membership, err)
	}
	if _, err := tracks.Get(context.Background(), track.FileID); !errors.Is(err, library.ErrFileNotFound) {
		t.Fatalf("explicitly removed Track still present: %v", err)
	}

	response := dto.ListenLocalIndexRefreshResponse{}
	service.refreshListenLocalTrack(context.Background(), fileItem, &mediaProbe{
		StreamInfo: true, HasAudio: true, Format: "mp3", AudioCodec: "mp3", Title: "Should not return",
	}, listenLocalCoverLookup{}, &response)
	if _, err := tracks.Get(context.Background(), track.FileID); !errors.Is(err, library.ErrFileNotFound) {
		t.Fatalf("scanner resurrected user-excluded Track: %v", err)
	}
	if response.Scanned != 1 || response.Added != 0 || response.Updated != 0 || response.Failed != 0 {
		t.Fatalf("unexpected exclusion refresh response: %#v", response)
	}
}

func TestIOSCompatibleRepresentationOutputPolicyExclusionSurvivesFullRefresh(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 30, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "compatible.m4a")
	fileItem := library.LibraryFile{
		ID: "file-compatible-output", LibraryID: "library-1", Kind: library.FileKindTranscode,
		Name: "compatible.m4a", DisplayName: "Compatible",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: path},
		CreatedAt: now, UpdatedAt: now,
	}
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: fileItem.ID, LibraryID: fileItem.LibraryID, LocalPath: path, Title: "Duplicate",
		Format: "m4a", AudioCodec: "aac", Availability: library.ListenLocalTrackAvailable,
		LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	tracks := &listenLocalRefreshTrackRepository{items: map[string]library.ListenLocalTrack{track.FileID: track}}
	memberships := &listenLocalMembershipRepository{}
	service := &LibraryService{
		files:       &listenLocalRefreshFileRepository{items: []library.LibraryFile{fileItem}},
		localTracks: tracks, localMusicMemberships: memberships, nowFunc: func() time.Time { return now },
	}
	if err := service.excludeIOSCompatibleRepresentationOutput(context.Background(), fileItem.ID); err != nil {
		t.Fatal(err)
	}
	membership, err := memberships.Get(context.Background(), fileItem.ID)
	if err != nil || !membership.IsExcluded() || membership.Reason != "policy" {
		t.Fatalf("compatible output membership=%#v err=%v", membership, err)
	}

	response := dto.ListenLocalIndexRefreshResponse{}
	service.refreshListenLocalTrack(context.Background(), fileItem, &mediaProbe{
		StreamInfo: true, HasAudio: true, Format: "m4a", AudioCodec: "aac", Title: "Must not index",
	}, listenLocalCoverLookup{}, &response)
	if _, err := tracks.Get(context.Background(), fileItem.ID); !errors.Is(err, library.ErrFileNotFound) {
		t.Fatalf("full refresh recreated compatible output as duplicate Track: %v", err)
	}
	if response.Scanned != 1 || response.Removed != 1 || response.Added != 0 || response.Updated != 0 || response.Failed != 0 {
		t.Fatalf("unexpected policy exclusion refresh response: %#v", response)
	}
}
