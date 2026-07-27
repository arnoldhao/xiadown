package libraryimport

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
	importdomain "xiadown/internal/domain/libraryimport"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/persistence"
)

func TestServiceResumesProjectionWithoutRegisteringFileTwice(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "movie.mp4")
	writeTestFile(t, source, "movie")
	repository := newMemoryRepository()
	importer := &importerStub{}
	projector := &projectorStub{failuresRemaining: 1}
	service := NewService(repository, &fileRepositoryStub{}, importer, projector, inspectorStub{})
	dryRun, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "request-1", SourcePaths: []string{source}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil || dryRun.Status != importdomain.BatchReady || len(dryRun.Candidates) != 1 {
		t.Fatalf("dry run failed: %+v, err=%v", dryRun, err)
	}
	first, err := service.Commit(ctx, BatchRequest{BatchID: dryRun.ID})
	if err != nil || first.Status != importdomain.BatchFailed || first.Candidates[0].Status != importdomain.CandidateRegistered {
		t.Fatalf("expected durable registered projection failure: %+v, err=%v", first, err)
	}
	resumed, err := service.Resume(ctx, BatchRequest{BatchID: dryRun.ID})
	if err != nil || resumed.Status != importdomain.BatchCompleted || resumed.Candidates[0].Status != importdomain.CandidateSucceeded {
		t.Fatalf("resume failed: %+v, err=%v", resumed, err)
	}
	if importer.registerCalls != 1 {
		t.Fatalf("registered file %d times; retry must only rerun projection", importer.registerCalls)
	}
	if projector.calls != 2 {
		t.Fatalf("expected projection retry, got %d calls", projector.calls)
	}
	idempotent, err := service.Commit(ctx, BatchRequest{BatchID: dryRun.ID})
	if err != nil || idempotent.Status != importdomain.BatchCompleted || importer.registerCalls != 1 {
		t.Fatalf("completed commit was not idempotent: %+v, calls=%d, err=%v", idempotent, importer.registerCalls, err)
	}
}

func TestCopyDryRunRegistersManagedRootWithCatalog(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "book.epub")
	writeTestFile(t, source, "book")
	selectedRoot := t.TempDir()
	registeredRoot := filepath.Join(selectedRoot, "catalog-managed")
	registrar := &managedRootRegistrarStub{registeredPath: registeredRoot}
	service := NewService(newMemoryRepository(), &fileRepositoryStub{}, &importerStub{}, &projectorStub{}, inspectorStub{})
	service.SetManagedRootRegistrar(registrar)

	result, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "managed-root-request", SourcePaths: []string{source},
		Mode: importdomain.ModeCopy, ManagedRoot: selectedRoot,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	expectedSelectedRoot, _ := filepath.EvalSymlinks(selectedRoot)
	if registrar.calls != 1 || registrar.selectedPath != expectedSelectedRoot {
		t.Fatalf("managed root registrar calls=%d path=%q", registrar.calls, registrar.selectedPath)
	}
	if result.ManagedRoot != registeredRoot {
		t.Fatalf("batch managed root=%q, want Catalog path %q", result.ManagedRoot, registeredRoot)
	}
}

func TestReferencedDryRunRegistersSelectedRootsWithCatalog(t *testing.T) {
	ctx := context.Background()
	sourceDirectory := t.TempDir()
	source := filepath.Join(sourceDirectory, "book.epub")
	writeTestFile(t, source, "book")
	registrar := &managedRootRegistrarStub{}
	service := NewService(newMemoryRepository(), &fileRepositoryStub{}, &importerStub{}, &projectorStub{}, inspectorStub{})
	service.SetManagedRootRegistrar(registrar)

	_, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "referenced-root-request", SourcePaths: []string{source},
		ReferenceRoots: []string{sourceDirectory}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if registrar.referenceCalls != 1 ||
		len(registrar.referencePaths) != 1 ||
		registrar.referencePaths[0] != sourceDirectory {
		t.Fatalf(
			"referenced root registrar calls=%d paths=%#v",
			registrar.referenceCalls,
			registrar.referencePaths,
		)
	}
}

func TestProfessionalImportProjectsRegisteredBatchOnce(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "first.mp4"), "first")
	writeTestFile(t, filepath.Join(directory, "second.mp3"), "second")
	importer := &importerStub{}
	projector := &projectorStub{}
	service := NewService(newMemoryRepository(), &fileRepositoryStub{}, importer, projector, inspectorStub{})
	dryRun, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "batch-projection", SourcePaths: []string{directory}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil || len(dryRun.Candidates) != 2 {
		t.Fatalf("DryRun: candidates=%d err=%v", len(dryRun.Candidates), err)
	}
	result, err := service.Commit(ctx, BatchRequest{BatchID: dryRun.ID})
	if err != nil || result.Status != importdomain.BatchCompleted || result.Counts.Succeeded != 2 {
		t.Fatalf("Commit: result=%#v err=%v", result, err)
	}
	if importer.registerCalls != 2 || projector.calls != 1 {
		t.Fatalf("register calls=%d projection calls=%d, want 2/1", importer.registerCalls, projector.calls)
	}
	if len(importer.projectionNotifications) != 2 {
		t.Fatalf("post-projection notifications=%#v", importer.projectionNotifications)
	}
}

func TestResumeRecoversInterruptedRunningAndImportingStates(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "movie.mp4")
	writeTestFile(t, source, "movie")
	repository := newMemoryRepository()
	importer := &importerStub{}
	service := NewService(repository, &fileRepositoryStub{}, importer, &projectorStub{}, inspectorStub{})
	dryRun, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "request-interrupted", SourcePaths: []string{source}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil {
		t.Fatal(err)
	}
	batch := repository.batches[dryRun.ID]
	batch.Status = importdomain.BatchRunning
	repository.batches[dryRun.ID] = batch
	for id, candidate := range repository.candidates[dryRun.ID] {
		candidate.Status = importdomain.CandidateImporting
		repository.candidates[dryRun.ID][id] = candidate
	}
	result, err := service.Resume(ctx, BatchRequest{BatchID: dryRun.ID})
	if err != nil || result.Status != importdomain.BatchCompleted || result.Candidates[0].Status != importdomain.CandidateSucceeded {
		t.Fatalf("interrupted execution was not recovered: %+v, err=%v", result, err)
	}
}

func TestServiceRejectsSourceMutationAfterDryRun(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "book.epub")
	writeTestFile(t, source, "aaaa")
	repository := newMemoryRepository()
	importer := &importerStub{}
	service := NewService(repository, &fileRepositoryStub{}, importer, &projectorStub{}, inspectorStub{})
	dryRun, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "request-2", SourcePaths: []string{source}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, source, "bbbb")
	result, err := service.Commit(ctx, BatchRequest{BatchID: dryRun.ID})
	if err != nil || result.Status != importdomain.BatchFailed || result.Candidates[0].ErrorCode != "source_changed" {
		t.Fatalf("unexpected source mutation result: %+v, err=%v", result, err)
	}
	if importer.registerCalls != 0 {
		t.Fatal("mutated source must not be registered")
	}
}

func TestDryRunDetectsExistingFileBySizeAndStrongHash(t *testing.T) {
	ctx := context.Background()
	existingPath := filepath.Join(t.TempDir(), "existing.mp3")
	selectedPath := filepath.Join(t.TempDir(), "selected.mp3")
	writeTestFile(t, existingPath, "same audio")
	writeTestFile(t, selectedPath, "same audio")
	size := int64(len("same audio"))
	files := &fileRepositoryStub{items: []library.LibraryFile{{
		ID: "existing-file", Storage: library.FileStorage{LocalPath: existingPath},
		Media: &library.MediaInfo{SizeBytes: &size}, State: library.FileState{Status: "active"},
	}}}
	service := NewService(newMemoryRepository(), files, &importerStub{}, &projectorStub{}, inspectorStub{})
	result, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "request-3", SourcePaths: []string{selectedPath}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil || result.Candidates[0].Status != importdomain.CandidateDuplicate || result.Candidates[0].DuplicateFileID != "existing-file" {
		t.Fatalf("strong existing-file dedupe failed: %+v, err=%v", result, err)
	}
}

func TestCancelStopsRunningBatchAndPersistsPerFileResult(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "movie.mp4")
	writeTestFile(t, source, "movie")
	repository := newMemoryRepository()
	started := make(chan struct{})
	importer := &blockingImporterStub{started: started}
	service := NewService(repository, &fileRepositoryStub{}, importer, &projectorStub{}, inspectorStub{})
	dryRun, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "request-cancel", SourcePaths: []string{source}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil {
		t.Fatal(err)
	}
	resultChannel := make(chan BatchDTO, 1)
	errorChannel := make(chan error, 1)
	go func() {
		result, commitErr := service.Commit(ctx, BatchRequest{BatchID: dryRun.ID})
		resultChannel <- result
		errorChannel <- commitErr
	}()
	<-started
	if _, err := service.Cancel(ctx, BatchRequest{BatchID: dryRun.ID}); err != nil {
		t.Fatal(err)
	}
	result := <-resultChannel
	if err := <-errorChannel; err != nil {
		t.Fatal(err)
	}
	if result.Status != importdomain.BatchCancelled || len(result.Candidates) != 1 || result.Candidates[0].Status != importdomain.CandidateCancelled {
		t.Fatalf("unexpected cancelled result: %+v", result)
	}
}

func TestCancelProjectsAlreadyRegisteredCandidatesWithoutMarkingPendingOnesSucceeded(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "first.mp4"), "first")
	writeTestFile(t, filepath.Join(directory, "second.mp4"), "second")
	repository := newMemoryRepository()
	secondStarted := make(chan struct{})
	importer := &stagedBlockingImporterStub{secondStarted: secondStarted}
	projector := &projectorStub{}
	service := NewService(repository, &fileRepositoryStub{}, importer, projector, inspectorStub{})
	dryRun, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "cancel-after-registration", SourcePaths: []string{directory}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil || len(dryRun.Candidates) != 2 {
		t.Fatalf("DryRun: candidates=%d err=%v", len(dryRun.Candidates), err)
	}
	resultChannel := make(chan BatchDTO, 1)
	errorChannel := make(chan error, 1)
	go func() {
		result, commitErr := service.Commit(ctx, BatchRequest{BatchID: dryRun.ID})
		resultChannel <- result
		errorChannel <- commitErr
	}()
	<-secondStarted
	if _, err := service.Cancel(ctx, BatchRequest{BatchID: dryRun.ID}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	result := <-resultChannel
	if err := <-errorChannel; err != nil {
		t.Fatalf("Commit: %v", err)
	}
	succeeded, cancelled := 0, 0
	for _, candidate := range result.Candidates {
		switch candidate.Status {
		case importdomain.CandidateSucceeded:
			succeeded++
		case importdomain.CandidateCancelled:
			cancelled++
		}
	}
	if result.Status != importdomain.BatchCancelled || succeeded != 1 || cancelled != 1 {
		t.Fatalf("cancelled result=%#v, want one projected and one cancelled", result)
	}
	if projector.calls != 1 || projector.lastContextErr != nil {
		t.Fatalf("recovery projection calls=%d contextErr=%v", projector.calls, projector.lastContextErr)
	}
}

func TestCancelProjectsDurablyRegisteredFileIntoRealCatalog(t *testing.T) {
	ctx := context.Background()
	db, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: filepath.Join(t.TempDir(), "cancel-projection.db")})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "first.mp4"), "first")
	writeTestFile(t, filepath.Join(directory, "second.mp4"), "second")
	libraries := libraryrepo.NewSQLiteLibraryRepository(db.Bun)
	files := libraryrepo.NewSQLiteFileRepository(db.Bun)
	secondStarted := make(chan struct{})
	importer := &stagedCatalogImporterStub{libraries: libraries, files: files, secondStarted: secondStarted}
	projector := libraryservice.NewLegacyCatalogBackfillService(
		libraries, files, libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	service := NewService(newMemoryRepository(), files, importer, projector, inspectorStub{})
	dryRun, err := service.DryRun(ctx, DryRunCommand{
		RequestKey: "cancel-real-catalog", SourcePaths: []string{directory}, Mode: importdomain.ModeReferenced,
		HiddenPolicy: importdomain.HiddenExclude, SymlinkPolicy: importdomain.SymlinkSkip,
	})
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	resultChannel := make(chan BatchDTO, 1)
	go func() {
		result, _ := service.Commit(ctx, BatchRequest{BatchID: dryRun.ID})
		resultChannel <- result
	}()
	<-secondStarted
	if _, err := service.Cancel(ctx, BatchRequest{BatchID: dryRun.ID}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	result := <-resultChannel
	projectedFileID := ""
	for _, candidate := range result.Candidates {
		if candidate.Status == importdomain.CandidateSucceeded {
			projectedFileID = candidate.FileID
		}
	}
	if projectedFileID == "" {
		t.Fatalf("no registered candidate succeeded: %#v", result)
	}
	var mappings, assets, checkpoints int
	_ = db.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_legacy_mappings
WHERE source_type = 'library_file' AND source_id = ?
`, projectedFileID).Scan(&mappings)
	_ = db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_item_assets WHERE file_id = ?", projectedFileID).Scan(&assets)
	_ = db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_migration_checkpoints").Scan(&checkpoints)
	if mappings != 1 || assets != 1 || checkpoints != 0 {
		t.Fatalf("cancel recovery projection mappings=%d assets=%d checkpoints=%d", mappings, assets, checkpoints)
	}
}

type importerStub struct {
	registerCalls           int
	requests                []libraryservice.ProfessionalImportRequest
	projectionNotifications []string
}

func (stub *importerStub) NotifyCatalogProjectionCompleted(_ context.Context, fileID string) {
	stub.projectionNotifications = append(stub.projectionNotifications, fileID)
}

type blockingImporterStub struct{ started chan struct{} }

type stagedBlockingImporterStub struct {
	secondStarted chan struct{}
	calls         int
}

type stagedCatalogImporterStub struct {
	libraries     library.LibraryRepository
	files         library.FileRepository
	secondStarted chan struct{}
	calls         int
}

func (stub *blockingImporterStub) EnsureProfessionalImportLibrary(_ context.Context, libraryID, _ string) (string, error) {
	return libraryID, nil
}

func (stub *blockingImporterStub) RegisterProfessionalImport(ctx context.Context, _ libraryservice.ProfessionalImportRequest) (libraryservice.ProfessionalImportRegistration, error) {
	close(stub.started)
	<-ctx.Done()
	return libraryservice.ProfessionalImportRegistration{}, ctx.Err()
}

func (stub *stagedBlockingImporterStub) EnsureProfessionalImportLibrary(_ context.Context, libraryID, _ string) (string, error) {
	return libraryID, nil
}

func (stub *stagedBlockingImporterStub) RegisterProfessionalImport(ctx context.Context, request libraryservice.ProfessionalImportRequest) (libraryservice.ProfessionalImportRegistration, error) {
	stub.calls++
	if stub.calls == 2 {
		close(stub.secondStarted)
		<-ctx.Done()
		return libraryservice.ProfessionalImportRegistration{}, ctx.Err()
	}
	return libraryservice.ProfessionalImportRegistration{
		BatchID: request.BatchID, LibraryID: request.LibraryID, FileID: request.FileID,
		HistoryID: request.HistoryID, FileEventID: request.FileEventID, StoragePath: request.StoragePath,
	}, nil
}

func (stub *stagedCatalogImporterStub) EnsureProfessionalImportLibrary(ctx context.Context, libraryID, displayName string) (string, error) {
	now := time.Now().UTC()
	item, err := library.NewLibrary(library.LibraryParams{
		ID: libraryID, Name: displayName, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		return "", err
	}
	if err := stub.libraries.Save(ctx, item); err != nil {
		return "", err
	}
	return item.ID, nil
}

func (stub *stagedCatalogImporterStub) RegisterProfessionalImport(ctx context.Context, request libraryservice.ProfessionalImportRequest) (libraryservice.ProfessionalImportRegistration, error) {
	stub.calls++
	if stub.calls == 2 {
		close(stub.secondStarted)
		<-ctx.Done()
		return libraryservice.ProfessionalImportRegistration{}, ctx.Err()
	}
	now := time.Now().UTC()
	file, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: request.FileID, LibraryID: request.LibraryID, Kind: request.Kind,
		Name: filepath.Base(request.StoragePath), DisplayName: request.DisplayName,
		Storage: library.FileStorage{Mode: "local_path", LocalPath: request.StoragePath},
		Origin: library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{
			BatchID: request.BatchID, ImportPath: request.SourcePath, ImportedAt: now, KeepSourceFile: true,
		}},
		State: library.FileState{Status: "active"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		return libraryservice.ProfessionalImportRegistration{}, err
	}
	if err := stub.files.Save(ctx, file); err != nil {
		return libraryservice.ProfessionalImportRegistration{}, err
	}
	return libraryservice.ProfessionalImportRegistration{
		BatchID: request.BatchID, LibraryID: request.LibraryID, FileID: request.FileID,
		HistoryID: request.HistoryID, FileEventID: request.FileEventID, StoragePath: request.StoragePath,
	}, nil
}

func (stub *importerStub) EnsureProfessionalImportLibrary(_ context.Context, libraryID, _ string) (string, error) {
	return libraryID, nil
}

func (stub *importerStub) RegisterProfessionalImport(_ context.Context, request libraryservice.ProfessionalImportRequest) (libraryservice.ProfessionalImportRegistration, error) {
	stub.registerCalls++
	stub.requests = append(stub.requests, request)
	return libraryservice.ProfessionalImportRegistration{
		BatchID: request.BatchID, LibraryID: request.LibraryID, FileID: request.FileID,
		HistoryID: request.HistoryID, FileEventID: request.FileEventID, StoragePath: request.StoragePath,
	}, nil
}

type projectorStub struct {
	calls             int
	failuresRemaining int
	lastContextErr    error
}

type managedRootRegistrarStub struct {
	calls          int
	selectedPath   string
	registeredPath string
	referenceCalls int
	referencePaths []string
}

func (stub *managedRootRegistrarStub) EnsureManagedImportRoot(_ context.Context, path string) (string, error) {
	stub.calls++
	stub.selectedPath = path
	return stub.registeredPath, nil
}

func (stub *managedRootRegistrarStub) EnsureReferencedImportRoots(_ context.Context, paths []string) error {
	stub.referenceCalls++
	stub.referencePaths = append([]string(nil), paths...)
	return nil
}

func (stub *projectorStub) Run(ctx context.Context) (libraryservice.CatalogBackfillResult, error) {
	stub.calls++
	stub.lastContextErr = ctx.Err()
	if stub.failuresRemaining > 0 {
		stub.failuresRemaining--
		return libraryservice.CatalogBackfillResult{}, errors.New("projection unavailable")
	}
	return libraryservice.CatalogBackfillResult{Completed: true}, nil
}

type fileRepositoryStub struct{ items []library.LibraryFile }

func (stub *fileRepositoryStub) List(context.Context) ([]library.LibraryFile, error) {
	return append([]library.LibraryFile(nil), stub.items...), nil
}
func (stub *fileRepositoryStub) ListByLibraryID(context.Context, string) ([]library.LibraryFile, error) {
	return nil, nil
}
func (stub *fileRepositoryStub) Get(context.Context, string) (library.LibraryFile, error) {
	return library.LibraryFile{}, library.ErrFileNotFound
}
func (stub *fileRepositoryStub) Save(context.Context, library.LibraryFile) error { return nil }
func (stub *fileRepositoryStub) Delete(context.Context, string) error            { return nil }

type memoryRepository struct {
	mu         sync.Mutex
	batches    map[string]importdomain.Batch
	byRequest  map[string]string
	candidates map[string]map[string]importdomain.Candidate
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		batches: make(map[string]importdomain.Batch), byRequest: make(map[string]string),
		candidates: make(map[string]map[string]importdomain.Candidate),
	}
}

func (repo *memoryRepository) CreateBatch(_ context.Context, batch importdomain.Batch) (importdomain.Batch, bool, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if id := repo.byRequest[batch.RequestKey]; id != "" {
		return repo.batches[id], false, nil
	}
	repo.batches[batch.ID] = batch
	repo.byRequest[batch.RequestKey] = batch.ID
	return batch, true, nil
}

func (repo *memoryRepository) ReplaceScan(_ context.Context, batch importdomain.Batch, candidates []importdomain.Candidate) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.batches[batch.ID] = batch
	repo.candidates[batch.ID] = make(map[string]importdomain.Candidate)
	for _, item := range candidates {
		repo.candidates[batch.ID][item.ID] = item
	}
	return nil
}

func (repo *memoryRepository) GetBatch(_ context.Context, id string) (importdomain.Batch, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	item, exists := repo.batches[id]
	if !exists {
		return importdomain.Batch{}, importdomain.ErrBatchNotFound
	}
	return item, nil
}

func (repo *memoryRepository) GetBatchByRequestKey(_ context.Context, key string) (importdomain.Batch, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	id := repo.byRequest[key]
	item, exists := repo.batches[id]
	if !exists {
		return importdomain.Batch{}, importdomain.ErrBatchNotFound
	}
	return item, nil
}

func (repo *memoryRepository) ListBatches(_ context.Context, limit int) ([]importdomain.Batch, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]importdomain.Batch, 0, len(repo.batches))
	for _, item := range repo.batches {
		result = append(result, item)
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (repo *memoryRepository) ListCandidates(_ context.Context, batchID string) ([]importdomain.Candidate, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]importdomain.Candidate, 0, len(repo.candidates[batchID]))
	for _, item := range repo.candidates[batchID] {
		result = append(result, item)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].SourcePath < result[right].SourcePath })
	return result, nil
}

func (repo *memoryRepository) SaveBatch(_ context.Context, batch importdomain.Batch) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if _, exists := repo.batches[batch.ID]; !exists {
		return importdomain.ErrBatchNotFound
	}
	repo.batches[batch.ID] = batch
	return nil
}

func (repo *memoryRepository) SaveCandidate(_ context.Context, candidate importdomain.Candidate) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if _, exists := repo.candidates[candidate.BatchID][candidate.ID]; !exists {
		return importdomain.ErrCandidateNotFound
	}
	repo.candidates[candidate.BatchID][candidate.ID] = candidate
	return nil
}
