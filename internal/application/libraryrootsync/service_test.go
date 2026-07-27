package libraryrootsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
	domain "xiadown/internal/domain/libraryrootsync"
)

type memoryRepository struct {
	mu      sync.Mutex
	states  map[string]domain.State
	entries map[string]domain.Entry
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		states:  make(map[string]domain.State),
		entries: make(map[string]domain.Entry),
	}
}

func (repo *memoryRepository) ListStates(context.Context) ([]domain.State, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]domain.State, 0, len(repo.states))
	for _, item := range repo.states {
		result = append(result, item)
	}
	return result, nil
}

func (repo *memoryRepository) GetState(
	_ context.Context,
	rootID string,
) (domain.State, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	item, ok := repo.states[rootID]
	if !ok {
		return domain.State{}, domain.ErrStateNotFound
	}
	return item, nil
}

func (repo *memoryRepository) SaveState(
	_ context.Context,
	state domain.State,
) error {
	validated, err := domain.NewState(state)
	if err != nil {
		return err
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if current, ok := repo.states[state.RootID]; ok &&
		current.WatcherCursor > validated.WatcherCursor {
		validated.WatcherCursor = current.WatcherCursor
	}
	repo.states[state.RootID] = validated
	return nil
}

func (repo *memoryRepository) MarkActiveStatesInterrupted(context.Context) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for key, item := range repo.states {
		switch item.Status {
		case domain.StatusQueued, domain.StatusScanning, domain.StatusCancelling:
			item.Status = domain.StatusInterrupted
			item.CancelRequested = false
			repo.states[key] = item
		}
	}
	return nil
}

func (repo *memoryRepository) AdvanceWatcherCursor(
	_ context.Context,
	rootID string,
	cursor uint64,
) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	item, ok := repo.states[rootID]
	if ok && cursor > item.WatcherCursor {
		item.WatcherCursor = cursor
		repo.states[rootID] = item
	}
	return nil
}

func entryKey(rootID, path string) string {
	return rootID + "\x00" + filepath.ToSlash(path)
}

func (repo *memoryRepository) GetEntry(
	_ context.Context,
	rootID string,
	relativePath string,
) (domain.Entry, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	item, ok := repo.entries[entryKey(rootID, relativePath)]
	if !ok {
		return domain.Entry{}, domain.ErrEntryNotFound
	}
	return item, nil
}

func (repo *memoryRepository) ListEntriesByStatus(
	_ context.Context,
	rootID string,
	status domain.EntryStatus,
) ([]domain.Entry, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]domain.Entry, 0)
	for _, item := range repo.entries {
		if item.RootID == rootID && item.Status == status {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo *memoryRepository) ListActiveEntriesBySize(
	_ context.Context,
	rootID string,
	sizeBytes int64,
) ([]domain.Entry, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]domain.Entry, 0)
	for _, item := range repo.entries {
		if item.RootID == rootID &&
			item.Status == domain.EntryActive &&
			item.SizeBytes == sizeBytes {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo *memoryRepository) FindActiveEntryByDigest(
	_ context.Context,
	rootID string,
	sizeBytes int64,
	contentHash string,
) (domain.Entry, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, item := range repo.entries {
		if item.RootID == rootID &&
			item.Status == domain.EntryActive &&
			item.SizeBytes == sizeBytes &&
			item.ContentHash == contentHash {
			return item, nil
		}
	}
	return domain.Entry{}, domain.ErrEntryNotFound
}

func (repo *memoryRepository) UpsertEntry(
	_ context.Context,
	entry domain.Entry,
) error {
	validated, err := domain.NewEntry(entry)
	if err != nil {
		return err
	}
	repo.mu.Lock()
	repo.entries[entryKey(entry.RootID, entry.RelativePath)] = validated
	repo.mu.Unlock()
	return nil
}

func (repo *memoryRepository) MarkUnseenEntriesMissing(
	_ context.Context,
	rootID string,
	generation int64,
) (int, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	count := 0
	for key, item := range repo.entries {
		if item.RootID == rootID &&
			item.LastSeenGeneration < generation &&
			item.Status != domain.EntryMissing {
			item.Status = domain.EntryMissing
			repo.entries[key] = item
			count++
		}
	}
	return count, nil
}

func (repo *memoryRepository) MarkPathMissing(
	_ context.Context,
	rootID string,
	relativePath string,
	recursive bool,
	generation int64,
) (int, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	prefix := filepath.ToSlash(relativePath)
	count := 0
	for key, item := range repo.entries {
		match := item.RootID == rootID && item.RelativePath == prefix
		if recursive {
			match = item.RootID == rootID &&
				(item.RelativePath == prefix ||
					strings.HasPrefix(item.RelativePath, prefix+"/"))
		}
		if match && item.Status != domain.EntryMissing {
			item.Status = domain.EntryMissing
			item.LastSeenGeneration = generation
			repo.entries[key] = item
			count++
		}
	}
	return count, nil
}

type emptyFileRepository struct{}

func (emptyFileRepository) List(context.Context) ([]library.LibraryFile, error) {
	return nil, nil
}

type staticFileRepository []library.LibraryFile

func (items staticFileRepository) List(context.Context) ([]library.LibraryFile, error) {
	return append([]library.LibraryFile(nil), items...), nil
}

type registrarStub struct {
	mu       sync.Mutex
	requests []libraryservice.ProfessionalImportRequest
	block    bool
	entered  chan struct{}
}

func (stub *registrarStub) EnsureProfessionalImportLibrary(
	context.Context,
	string,
	string,
) (string, error) {
	return "library", nil
}

func (stub *registrarStub) RegisterProfessionalImport(
	ctx context.Context,
	request libraryservice.ProfessionalImportRequest,
) (libraryservice.ProfessionalImportRegistration, error) {
	if stub.block {
		select {
		case stub.entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return libraryservice.ProfessionalImportRegistration{}, ctx.Err()
	}
	stub.mu.Lock()
	stub.requests = append(stub.requests, request)
	stub.mu.Unlock()
	return libraryservice.ProfessionalImportRegistration{
		LibraryID: request.LibraryID,
		FileID:    request.FileID,
	}, nil
}

func (stub *registrarStub) count() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return len(stub.requests)
}

func (stub *registrarStub) snapshot() []libraryservice.ProfessionalImportRequest {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return append([]libraryservice.ProfessionalImportRequest(nil), stub.requests...)
}

type projectorStub struct{}

func (projectorStub) Run(context.Context) (libraryservice.CatalogBackfillResult, error) {
	return libraryservice.CatalogBackfillResult{}, nil
}

func (projectorStub) RunLibrary(
	context.Context,
	string,
) (libraryservice.CatalogBackfillResult, error) {
	return libraryservice.CatalogBackfillResult{}, nil
}

type projectionNotifierStub struct {
	mu           sync.Mutex
	fileIDs      []string
	batchRootIDs []string
	batchFileIDs [][]string
	rootIDs      []string
}

func (stub *projectionNotifierStub) NotifyCatalogProjectionCompleted(
	_ context.Context,
	fileID string,
) {
	stub.mu.Lock()
	stub.fileIDs = append(stub.fileIDs, fileID)
	stub.mu.Unlock()
}

func (stub *projectionNotifierStub) NotifyCatalogProjectionBatchCompleted(
	_ context.Context,
	rootID string,
	fileIDs []string,
) {
	stub.mu.Lock()
	stub.batchRootIDs = append(stub.batchRootIDs, rootID)
	stub.batchFileIDs = append(stub.batchFileIDs, append([]string(nil), fileIDs...))
	stub.mu.Unlock()
}

func (stub *projectionNotifierStub) NotifyCatalogAvailabilityChanged(
	_ context.Context,
	rootID string,
) {
	stub.mu.Lock()
	stub.rootIDs = append(stub.rootIDs, rootID)
	stub.mu.Unlock()
}

func (stub *projectionNotifierStub) roots() []string {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return append([]string(nil), stub.rootIDs...)
}

func (stub *projectionNotifierStub) batches() ([]string, [][]string) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	roots := append([]string(nil), stub.batchRootIDs...)
	files := make([][]string, 0, len(stub.batchFileIDs))
	for _, fileIDs := range stub.batchFileIDs {
		files = append(files, append([]string(nil), fileIDs...))
	}
	return roots, files
}

type testWatcher struct{}

func (testWatcher) Available() bool      { return true }
func (testWatcher) SupportsReplay() bool { return true }

func (testWatcher) Watch(
	ctx context.Context,
	_ string,
	_ uint64,
	_ func(watchEvent),
) error {
	<-ctx.Done()
	return ctx.Err()
}

func newTestService(
	repository *memoryRepository,
	importer *registrarStub,
	root Root,
) *Service {
	service := NewService(
		repository,
		emptyFileRepository{},
		importer,
		projectorStub{},
		nil,
	)
	service.watcher = testWatcher{}
	service.SetRootProvider(func(context.Context) ([]Root, error) {
		return []Root{root}, nil
	})
	service.mu.Lock()
	service.roots[root.ID] = root
	service.mu.Unlock()
	return service
}

func waitForState(
	t *testing.T,
	repository *memoryRepository,
	rootID string,
	match func(domain.State) bool,
) domain.State {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		item, err := repository.GetState(context.Background(), rootID)
		if err == nil && match(item) {
			return item
		}
		time.Sleep(10 * time.Millisecond)
	}
	item, err := repository.GetState(context.Background(), rootID)
	t.Fatalf("state did not converge: state=%#v err=%v", item, err)
	return domain.State{}
}

func TestEnqueueWatchEventPreservesOverflowSignalWhenQueueIsFull(t *testing.T) {
	events := make(chan watchEvent, 1)
	events <- watchEvent{path: "already-queued"}
	overflows := make(chan watchEvent, 1)

	enqueueWatchEvent(
		context.Background(),
		events,
		overflows,
		watchEvent{path: "dropped", cursor: 42},
	)

	select {
	case event := <-overflows:
		if !event.overflow || event.cursor != 42 {
			t.Fatalf("unexpected overflow event: %#v", event)
		}
	default:
		t.Fatal("overflow signal was dropped with the saturated event queue")
	}
	if got := len(events); got != 1 {
		t.Fatalf("event queue length = %d, want existing event preserved", got)
	}
}

func TestServiceFullScanRegistersThenReconcilesWithoutDuplicates(
	t *testing.T,
) {
	rootPath := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(rootPath, "clip.mp4"),
		[]byte("video"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(rootPath, "track.mp3"),
		[]byte("audio"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	root := Root{
		ID: "root-a", Name: "Referenced", Path: rootPath,
		VolumeID: "volume-a", Mode: "referenced", Online: true,
	}
	repository := newMemoryRepository()
	importer := &registrarStub{}
	service := newTestService(repository, importer, root)
	defer service.StopRoot(context.Background(), root.ID)

	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	first := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 1
	})
	if first.ProcessedCount != 2 || first.FailedCount != 0 ||
		importer.count() != 2 {
		t.Fatalf("unexpected first scan: state=%#v registrations=%d", first, importer.count())
	}

	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	second := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 2
	})
	if second.UnchangedCount != 2 || importer.count() != 2 {
		t.Fatalf("unchanged scan reimported files: state=%#v registrations=%d", second, importer.count())
	}

	if err := os.Remove(filepath.Join(rootPath, "track.mp3")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	third := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 3
	})
	if third.MissingCount != 1 || importer.count() != 2 {
		t.Fatalf("missing reconciliation mismatch: state=%#v registrations=%d", third, importer.count())
	}
}

func TestServiceSkipsFullHashWhenFileSizesAreUnique(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(rootPath, "first.bin"),
		[]byte("first"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(rootPath, "second.bin"),
		[]byte("second-file"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	root := Root{
		ID: "root-unique-sizes", Name: "Referenced", Path: rootPath,
		VolumeID: "volume-unique-sizes", Mode: "referenced", Online: true,
	}
	repository := newMemoryRepository()
	importer := &registrarStub{}
	service := newTestService(repository, importer, root)
	defer service.StopRoot(context.Background(), root.ID)
	var hashCalls atomic.Int32
	service.hasher = func(ctx context.Context, path string) (string, error) {
		hashCalls.Add(1)
		return hashFile(ctx, path)
	}

	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	state := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 1
	})
	if state.ProcessedCount != 2 || importer.count() != 2 {
		t.Fatalf("unexpected scan result: state=%#v registrations=%d", state, importer.count())
	}
	if got := hashCalls.Load(); got != 0 {
		t.Fatalf("unique-size files were fully hashed %d time(s)", got)
	}
	for _, relative := range []string{"first.bin", "second.bin"} {
		entry, err := repository.GetEntry(context.Background(), root.ID, relative)
		if err != nil {
			t.Fatal(err)
		}
		if entry.ContentHash != "" {
			t.Fatalf("%s unexpectedly stored a full digest", relative)
		}
	}
}

func TestServiceKeepsExactDuplicateAndPromotesItWhenCanonicalDisappears(
	t *testing.T,
) {
	rootPath := t.TempDir()
	for _, name := range []string{"first.bin", "second.bin"} {
		if err := os.WriteFile(
			filepath.Join(rootPath, name),
			[]byte("identical"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	root := Root{
		ID: "root-duplicate", Name: "Referenced", Path: rootPath,
		VolumeID: "volume-duplicate", Mode: "referenced", Online: true,
	}
	repository := newMemoryRepository()
	importer := &registrarStub{}
	service := newTestService(repository, importer, root)
	defer service.StopRoot(context.Background(), root.ID)
	var hashCalls atomic.Int32
	service.hasher = func(ctx context.Context, path string) (string, error) {
		hashCalls.Add(1)
		return hashFile(ctx, path)
	}

	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	state := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 1
	})
	if state.DuplicateCount != 1 || state.ProcessedCount != 2 ||
		importer.count() != 1 {
		t.Fatalf("exact duplicate was not preserved: state=%#v registrations=%d", state, importer.count())
	}
	if got := hashCalls.Load(); got != 2 {
		t.Fatalf("same-size duplicate hashing calls = %d, want 2", got)
	}
	first, err := repository.GetEntry(context.Background(), root.ID, "first.bin")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.GetEntry(context.Background(), root.ID, "second.bin")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != domain.EntryActive || first.ContentHash == "" ||
		second.Status != domain.EntryDuplicate ||
		second.ContentHash != first.ContentHash {
		t.Fatalf("unexpected duplicate entries: first=%#v second=%#v", first, second)
	}

	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	unchanged := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 2
	})
	if unchanged.UnchangedCount != 2 || importer.count() != 1 {
		t.Fatalf(
			"unchanged duplicates were reimported: state=%#v registrations=%d",
			unchanged,
			importer.count(),
		)
	}
	if got := hashCalls.Load(); got != 2 {
		t.Fatalf("unchanged duplicate hashing calls = %d, want 2", got)
	}

	if err := os.Remove(filepath.Join(rootPath, "first.bin")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	promotedState := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 3
	})
	promoted, err := repository.GetEntry(
		context.Background(),
		root.ID,
		"second.bin",
	)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := repository.GetEntry(
		context.Background(),
		root.ID,
		"first.bin",
	)
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Status != domain.EntryActive ||
		promoted.FileID != first.FileID ||
		missing.Status != domain.EntryMissing ||
		promotedState.MissingCount != 1 ||
		promotedState.DuplicateCount != 0 ||
		importer.count() != 2 {
		t.Fatalf(
			"duplicate did not take over the missing canonical entry: "+
				"state=%#v promoted=%#v missing=%#v registrations=%d",
			promotedState,
			promoted,
			missing,
			importer.count(),
		)
	}
	if got := hashCalls.Load(); got != 3 {
		t.Fatalf("duplicate promotion hashing calls = %d, want 3", got)
	}
}

func TestServiceClassifiesTypeScriptAndTransportStreamByContent(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(rootPath, "test.ts"),
		[]byte("export const test: string = 'source';\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	transport := make([]byte, 188*4)
	for packet := 0; packet < 4; packet++ {
		position := packet * 188
		transport[position] = 0x47
		transport[position+1] = 0x40
		transport[position+3] = 0x10
	}
	if err := os.WriteFile(
		filepath.Join(rootPath, "recording.ts"),
		transport,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	root := Root{
		ID: "root-ambiguous-ts", Name: "Referenced", Path: rootPath,
		VolumeID: "volume-ts", Mode: "referenced", Online: true,
	}
	repository := newMemoryRepository()
	importer := &registrarStub{}
	service := newTestService(repository, importer, root)
	defer service.StopRoot(context.Background(), root.ID)

	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 1
	})
	kinds := make(map[string]string)
	for _, request := range importer.snapshot() {
		kinds[request.DisplayName] = request.Kind
	}
	if got := kinds["test.ts"]; got != string(library.FileKindOther) {
		t.Fatalf("TypeScript kind = %q, want other", got)
	}
	if got := kinds["recording.ts"]; got != string(library.FileKindVideo) {
		t.Fatalf("MPEG transport kind = %q, want video", got)
	}
}

func TestServiceRunRepairsPreviouslyMisclassifiedTypeScript(t *testing.T) {
	rootPath := t.TempDir()
	sourcePath := filepath.Join(rootPath, "test.ts")
	body := []byte("export function test(): boolean { return true }\n")
	if err := os.WriteFile(sourcePath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	root := Root{
		ID: "root-repair-ts", Name: "Referenced", Path: rootPath,
		VolumeID: "volume-repair-ts", Mode: "referenced", Online: true,
	}
	repository := newMemoryRepository()
	state, err := domain.NewState(domain.State{
		RootID: root.ID, Status: domain.StatusWatching,
		Generation: 1, WatcherCursor: 42, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	entry, err := domain.NewEntry(domain.Entry{
		RootID: root.ID, RelativePath: "test.ts",
		SizeBytes: int64(len(body)), ModifiedUnixNano: info.ModTime().UnixNano(),
		FileID: "file-test-ts", Status: domain.EntryActive,
		LastSeenGeneration: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertEntry(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	files := staticFileRepository{{
		ID: "file-test-ts", LibraryID: "library-test-ts",
		Kind: library.FileKindVideo, Name: "test.ts",
		Storage: library.FileStorage{
			Mode: "local_path", LocalPath: sourcePath,
		},
		State: library.FileState{Status: "active"},
	}}
	importer := &registrarStub{}
	service := NewService(repository, files, importer, projectorStub{}, nil)
	service.watcher = testWatcher{}
	service.SetRootProvider(func(context.Context) ([]Root, error) {
		return []Root{root}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	repaired := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 2
	})
	if repaired.FullScan {
		t.Fatalf("classification repair unexpectedly required a full scan: %#v", repaired)
	}
	requests := importer.snapshot()
	if len(requests) != 1 ||
		requests[0].FileID != "file-test-ts" ||
		requests[0].Kind != string(library.FileKindOther) {
		t.Fatalf("unexpected repair registration: %+v", requests)
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("Run returned %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestServiceRunRechecksOnlyPreviouslyMissingPathsAndCoalescesRefresh(
	t *testing.T,
) {
	rootPath := t.TempDir()
	sourcePath := filepath.Join(rootPath, "restored.mp4")
	body := []byte("restored")
	if err := os.WriteFile(sourcePath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	root := Root{
		ID: "root-restored", Name: "Referenced", Path: rootPath,
		VolumeID: "volume-restored", Mode: "referenced", Online: true,
	}
	repository := newMemoryRepository()
	state, err := domain.NewState(domain.State{
		RootID: root.ID, Status: domain.StatusWatching,
		Generation: 4, WatcherCursor: 42, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	entry, err := domain.NewEntry(domain.Entry{
		RootID: root.ID, RelativePath: filepath.Base(sourcePath),
		SizeBytes: int64(len(body)), ModifiedUnixNano: info.ModTime().UnixNano(),
		FileID: "file-restored", Status: domain.EntryMissing,
		LastSeenGeneration: 3, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertEntry(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	notifier := new(projectionNotifierStub)
	service := NewService(
		repository,
		emptyFileRepository{},
		&registrarStub{},
		projectorStub{},
		notifier,
	)
	service.watcher = testWatcher{}
	service.SetRootProvider(func(context.Context) ([]Root, error) {
		return []Root{root}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	repaired := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 5
	})
	if repaired.FullScan || repaired.DiscoveredCount != 1 {
		t.Fatalf("restored path repair was not bounded: %#v", repaired)
	}
	restored, err := repository.GetEntry(context.Background(), root.ID, filepath.Base(sourcePath))
	if err != nil || restored.Status != domain.EntryActive {
		t.Fatalf("restored entry=%#v err=%v", restored, err)
	}
	if roots := notifier.roots(); len(roots) != 1 || roots[0] != root.ID {
		t.Fatalf("availability refreshes=%v, want one coalesced root refresh", roots)
	}
	batchRoots, batchFiles := notifier.batches()
	if len(batchRoots) != 1 || batchRoots[0] != root.ID ||
		len(batchFiles) != 1 || len(batchFiles[0]) != 1 ||
		batchFiles[0][0] != "file-restored" {
		t.Fatalf(
			"projection batches roots=%v files=%v, want one coalesced file batch",
			batchRoots,
			batchFiles,
		)
	}

	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("Run returned %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestServiceCancelPersistsVisibleProgress(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(rootPath, "large.mp4"),
		[]byte("content"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(rootPath, "second.mp4"),
		[]byte("different content"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	root := Root{
		ID: "root-cancel", Name: "Referenced", Path: rootPath,
		VolumeID: "volume-cancel", Mode: "referenced", Online: true,
	}
	repository := newMemoryRepository()
	importer := &registrarStub{
		block:   true,
		entered: make(chan struct{}, 1),
	}
	service := newTestService(repository, importer, root)
	defer service.StopRoot(context.Background(), root.ID)
	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	select {
	case <-importer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("scanner never reached the importer")
	}
	scanning := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusScanning &&
			item.DiscoveredCount == 2
	})
	if scanning.ProcessedCount != 0 {
		t.Fatalf("blocked file should not be processed: %#v", scanning)
	}
	if _, err := service.CancelRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	cancelled := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusCancelled
	})
	if cancelled.CancelRequested {
		t.Fatalf("cancel request was not cleared: %#v", cancelled)
	}
	if cancelled.DiscoveredCount != 2 {
		t.Fatalf("persisted progress was lost: %#v", cancelled)
	}
}

func TestServiceScansOnlyManagedDownloadChild(t *testing.T) {
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "xiadown")
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(rootPath, "managed.mp4"),
		[]byte("managed"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(parent, "outside.mp4"),
		[]byte("outside"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	root := Root{
		ID: "managed-root", Name: "Downloads", Path: rootPath,
		VolumeID: "volume-managed", Mode: "managed", Online: true,
	}
	repository := newMemoryRepository()
	importer := &registrarStub{}
	service := newTestService(repository, importer, root)
	defer service.StopRoot(context.Background(), root.ID)
	if _, err := service.StartRootScan(
		context.Background(),
		RootRequest{RootID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	state := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching && item.Generation == 1
	})
	requests := importer.snapshot()
	if state.ProcessedCount != 1 || len(requests) != 1 ||
		requests[0].DisplayName != "managed.mp4" {
		t.Fatalf("managed child boundary mismatch: state=%#v registrations=%+v", state, requests)
	}
}

func TestServiceRunRestartsInterruptedScan(t *testing.T) {
	rootPath := t.TempDir()
	state, err := domain.NewState(domain.State{
		RootID: "root-interrupted", Status: domain.StatusScanning,
		Generation: 4, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := newMemoryRepository()
	if err := repository.SaveState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	root := Root{
		ID: state.RootID, Name: "Interrupted", Path: rootPath,
		VolumeID: "volume-interrupted", Mode: "referenced", Online: true,
	}
	service := newTestService(repository, &registrarStub{}, root)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	restored := waitForState(t, repository, root.ID, func(item domain.State) bool {
		return item.Status == domain.StatusWatching &&
			item.Generation == 5
	})
	if restored.FullScan != true {
		t.Fatalf("interrupted scan was not safely reconciled: %#v", restored)
	}
	cancel()
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("Run returned %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}
