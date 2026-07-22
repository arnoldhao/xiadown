package libraryimport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
	importdomain "xiadown/internal/domain/libraryimport"
)

var importExecutionNamespace = uuid.MustParse("2876807f-87cc-5d44-b570-93b45d18ae6d")

type Service struct {
	repository   importdomain.Repository
	scanner      *Scanner
	files        library.FileRepository
	importer     LegacyImporter
	projector    CatalogProjector
	managedRoots ManagedRootRegistrar
	now          func() time.Time

	runMu   sync.Mutex
	cancels map[string]context.CancelFunc
}

func (service *Service) SetManagedRootRegistrar(registrar ManagedRootRegistrar) {
	if service == nil {
		return
	}
	service.managedRoots = registrar
}

func NewService(
	repository importdomain.Repository,
	files library.FileRepository,
	importer LegacyImporter,
	projector CatalogProjector,
	inspector MediaInspector,
) *Service {
	return &Service{
		repository: repository, files: files, importer: importer, projector: projector,
		scanner: NewScanner(inspector), cancels: make(map[string]context.CancelFunc),
		now: func() time.Time { return time.Now().UTC() },
	}
}

// DryRun persists the full preflight snapshot. SourcePaths and ManagedRoot are
// an internal application command: the Wails boundary obtains both exclusively
// from native desktop pickers and no HTTP route accepts this command.
func (service *Service) DryRun(ctx context.Context, command DryRunCommand) (BatchDTO, error) {
	if service == nil || service.repository == nil || service.scanner == nil || service.files == nil {
		return BatchDTO{}, fmt.Errorf("library import service is not configured")
	}
	requestKey := strings.TrimSpace(command.RequestKey)
	if requestKey == "" {
		return BatchDTO{}, fmt.Errorf("request key is required")
	}
	if len(command.SourcePaths) == 0 {
		return BatchDTO{}, fmt.Errorf("at least one desktop-selected source is required")
	}
	mode := command.Mode
	if mode == "" {
		mode = importdomain.ModeReferenced
	}
	hidden := command.HiddenPolicy
	if hidden == "" {
		hidden = importdomain.HiddenExclude
	}
	symlinks := command.SymlinkPolicy
	if symlinks == "" {
		symlinks = importdomain.SymlinkSkip
	}
	managedRoot, err := validateManagedRoot(command.ManagedRoot, mode)
	if err != nil {
		return BatchDTO{}, err
	}
	if mode == importdomain.ModeCopy && service.managedRoots != nil {
		managedRoot, err = service.managedRoots.EnsureManagedImportRoot(ctx, managedRoot)
		if err != nil {
			return BatchDTO{}, fmt.Errorf("register managed import root: %w", err)
		}
	}
	now := service.timestamp()
	batchID := uuid.NewString()
	libraryID := strings.TrimSpace(command.LibraryID)
	if libraryID == "" {
		libraryID = stableImportID(batchID, "library")
	}
	batch, err := importdomain.NewBatch(importdomain.Batch{
		ID: batchID, RequestKey: requestKey, LibraryID: libraryID,
		Mode: mode, ManagedRoot: managedRoot, HiddenPolicy: hidden, SymlinkPolicy: symlinks,
		Status: importdomain.BatchScanning, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return BatchDTO{}, err
	}
	batch, created, err := service.repository.CreateBatch(ctx, batch)
	if err != nil {
		return BatchDTO{}, err
	}
	if !created {
		candidates, listErr := service.repository.ListCandidates(ctx, batch.ID)
		if listErr != nil {
			return BatchDTO{}, listErr
		}
		return toBatchDTO(batch, candidates), nil
	}
	candidates, scanErr := service.scanner.Scan(ctx, command.SourcePaths, scanOptions{
		BatchID: batch.ID, HiddenPolicy: batch.HiddenPolicy, SymlinkPolicy: batch.SymlinkPolicy,
	})
	if scanErr == nil {
		candidates, scanErr = service.markExistingDuplicates(ctx, candidates)
	}
	if scanErr != nil {
		failedAt := service.timestamp()
		batch.Status = importdomain.BatchFailed
		batch.LastErrorCode = "scan_failed"
		batch.LastError = scanErr.Error()
		batch.FinishedAt = &failedAt
		batch.UpdatedAt = failedAt
		if saveErr := service.repository.SaveBatch(context.WithoutCancel(ctx), batch); saveErr != nil {
			return BatchDTO{}, errors.Join(scanErr, saveErr)
		}
		return toBatchDTO(batch, nil), scanErr
	}
	batch.Status = importdomain.BatchReady
	batch.Counts = importdomain.CountsFor(candidates)
	batch.UpdatedAt = service.timestamp()
	if err := service.repository.ReplaceScan(ctx, batch, candidates); err != nil {
		return BatchDTO{}, err
	}
	return toBatchDTO(batch, candidates), nil
}

func (service *Service) GetBatch(ctx context.Context, request BatchRequest) (BatchDTO, error) {
	batch, err := service.repository.GetBatch(ctx, strings.TrimSpace(request.BatchID))
	if err != nil {
		return BatchDTO{}, err
	}
	candidates, err := service.repository.ListCandidates(ctx, batch.ID)
	if err != nil {
		return BatchDTO{}, err
	}
	return toBatchDTO(batch, candidates), nil
}

func (service *Service) ListBatches(ctx context.Context, request ListBatchesRequest) ([]BatchDTO, error) {
	batches, err := service.repository.ListBatches(ctx, request.Limit)
	if err != nil {
		return nil, err
	}
	result := make([]BatchDTO, 0, len(batches))
	for _, batch := range batches {
		result = append(result, toBatchDTO(batch, nil))
	}
	return result, nil
}

func (service *Service) Commit(ctx context.Context, request BatchRequest) (BatchDTO, error) {
	return service.execute(ctx, strings.TrimSpace(request.BatchID), false)
}

func (service *Service) Resume(ctx context.Context, request BatchRequest) (BatchDTO, error) {
	return service.execute(ctx, strings.TrimSpace(request.BatchID), true)
}

func (service *Service) Cancel(ctx context.Context, request BatchRequest) (BatchDTO, error) {
	batchID := strings.TrimSpace(request.BatchID)
	batch, err := service.repository.GetBatch(ctx, batchID)
	if err != nil {
		return BatchDTO{}, err
	}
	service.runMu.Lock()
	cancel := service.cancels[batchID]
	service.runMu.Unlock()
	if batch.Status == importdomain.BatchCompleted || batch.Status == importdomain.BatchCancelled {
		return service.GetBatch(ctx, request)
	}
	now := service.timestamp()
	batch.CancelRequested = true
	if batch.Status == importdomain.BatchRunning && cancel != nil {
		batch.Status = importdomain.BatchCancelling
	} else {
		batch.Status = importdomain.BatchCancelled
		batch.FinishedAt = &now
	}
	batch.UpdatedAt = now
	if err := service.repository.SaveBatch(ctx, batch); err != nil {
		return BatchDTO{}, err
	}
	if cancel != nil {
		cancel()
	}
	return service.GetBatch(ctx, request)
}

func (service *Service) execute(ctx context.Context, batchID string, resume bool) (BatchDTO, error) {
	if service == nil || service.repository == nil || service.importer == nil || service.projector == nil {
		return BatchDTO{}, fmt.Errorf("library import execution is not configured")
	}
	if batchID == "" {
		return BatchDTO{}, fmt.Errorf("batch id is required")
	}
	runCtx, cancel, err := service.registerRun(ctx, batchID)
	if err != nil {
		return BatchDTO{}, err
	}
	defer service.finishRun(batchID, cancel)

	batch, err := service.repository.GetBatch(runCtx, batchID)
	if err != nil {
		return BatchDTO{}, err
	}
	if batch.Status == importdomain.BatchCompleted {
		return service.GetBatch(runCtx, BatchRequest{BatchID: batchID})
	}
	candidates, err := service.repository.ListCandidates(runCtx, batchID)
	if err != nil {
		return BatchDTO{}, err
	}
	if resume {
		if batch.LastErrorCode == "scan_failed" {
			return BatchDTO{}, fmt.Errorf("failed dry run must be repeated from the desktop source picker")
		}
		if batch.Status != importdomain.BatchFailed && batch.Status != importdomain.BatchCancelled &&
			batch.Status != importdomain.BatchReady && batch.Status != importdomain.BatchRunning &&
			batch.Status != importdomain.BatchCancelling {
			return BatchDTO{}, importdomain.ErrInvalidTransition
		}
		for index := range candidates {
			if candidates[index].Status != importdomain.CandidateFailed && candidates[index].Status != importdomain.CandidateCancelled {
				continue
			}
			candidates[index].Status = importdomain.CandidateReady
			candidates[index].ErrorCode = ""
			candidates[index].ErrorMessage = ""
			candidates[index].UpdatedAt = service.timestamp()
			if err := service.repository.SaveCandidate(runCtx, candidates[index]); err != nil {
				return BatchDTO{}, err
			}
		}
	} else if batch.Status != importdomain.BatchReady {
		return BatchDTO{}, importdomain.ErrInvalidTransition
	}

	startedAt := service.timestamp()
	if batch.StartedAt == nil {
		batch.StartedAt = &startedAt
	}
	batch.Status = importdomain.BatchRunning
	batch.CancelRequested = false
	batch.FinishedAt = nil
	batch.LastErrorCode = ""
	batch.LastError = ""
	batch.UpdatedAt = startedAt
	batch.Counts = importdomain.CountsFor(candidates)
	if err := service.repository.SaveBatch(runCtx, batch); err != nil {
		return BatchDTO{}, err
	}
	if _, err := service.importer.EnsureProfessionalImportLibrary(runCtx, batch.LibraryID, "Imported Library"); err != nil {
		return service.failBatch(runCtx, batch, "library_prepare_failed", err)
	}

	var lastErr error
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.Status == importdomain.CandidateDuplicate || candidate.Status == importdomain.CandidateSkipped || candidate.Status == importdomain.CandidateSucceeded {
			continue
		}
		if err := runCtx.Err(); err != nil {
			lastErr = err
			break
		}
		if err := service.processCandidate(runCtx, batch, candidate); err != nil {
			lastErr = err
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				break
			}
		}
		if err := service.saveRunningProgress(runCtx, batch.ID, candidates); err != nil {
			lastErr = err
			break
		}
	}
	if err := runCtx.Err(); err != nil || batchCancellationRequested(service.repository, batchID) {
		recoveryCtx := context.WithoutCancel(ctx)
		projectionErr := service.projectRegisteredCandidates(recoveryCtx, batch.LibraryID, candidates)
		return service.finishCancelled(recoveryCtx, batch, candidates, firstError(err, errors.Join(lastErr, projectionErr)))
	}
	if err := service.projectRegisteredCandidates(runCtx, batch.LibraryID, candidates); err != nil {
		lastErr = err
	}
	return service.finishExecution(ctx, batch, candidates, lastErr)
}

func (service *Service) saveRunningProgress(ctx context.Context, batchID string, candidates []importdomain.Candidate) error {
	batch, err := service.repository.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}
	// A concurrent Cancel owns the transition to cancelling/cancelled. Never
	// overwrite it with a stale running progress update.
	if batch.Status != importdomain.BatchRunning || batch.CancelRequested {
		return nil
	}
	batch.Counts = importdomain.CountsFor(candidates)
	batch.UpdatedAt = service.timestamp()
	return service.repository.SaveBatch(ctx, batch)
}

func (service *Service) processCandidate(ctx context.Context, batch importdomain.Batch, candidate *importdomain.Candidate) error {
	if candidate.Status == importdomain.CandidateRegistered {
		return nil
	}
	if candidate.Status != importdomain.CandidateCopied {
		candidate.Status = importdomain.CandidateImporting
		candidate.Attempts++
		candidate.ErrorCode = ""
		candidate.ErrorMessage = ""
		candidate.UpdatedAt = service.timestamp()
		if err := service.repository.SaveCandidate(ctx, *candidate); err != nil {
			return err
		}
		if err := verifyCandidateSource(ctx, *candidate); err != nil {
			return service.failCandidate(ctx, candidate, "source_changed", err)
		}
	}
	storagePath := candidate.SourcePath
	if batch.Mode == importdomain.ModeCopy {
		if candidate.Status == importdomain.CandidateCopied && strings.TrimSpace(candidate.ManagedPath) != "" {
			matches, err := managedPathMatchesDigest(
				ctx, batch.ManagedRoot, candidate.ManagedPath, candidate.SizeBytes, candidate.ContentHash,
			)
			if err != nil || !matches {
				if err == nil {
					err = fmt.Errorf("managed copy checksum mismatch")
				}
				return service.failCandidate(ctx, candidate, "managed_copy_invalid", err)
			}
			storagePath = candidate.ManagedPath
		} else {
			managedPath, err := copyIntoManagedRoot(ctx, batch.ManagedRoot, *candidate)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return service.cancelCandidate(context.WithoutCancel(ctx), candidate)
				}
				return service.failCandidate(ctx, candidate, "copy_failed", err)
			}
			candidate.ManagedPath = managedPath
			candidate.Status = importdomain.CandidateCopied
			candidate.UpdatedAt = service.timestamp()
			if err := service.repository.SaveCandidate(ctx, *candidate); err != nil {
				return err
			}
			storagePath = managedPath
		}
	}
	registration, err := service.importer.RegisterProfessionalImport(ctx, libraryservice.ProfessionalImportRequest{
		BatchID: batch.ID, CandidateID: candidate.ID, LibraryID: batch.LibraryID,
		SourcePath: candidate.SourcePath, StoragePath: storagePath, DisplayName: strings.TrimSuffix(candidate.DisplayName, candidate.Extension),
		Kind: fileKindFor(*candidate), SessionRunID: batch.ID,
		FileID: stableImportID(candidate.ID, "file"), HistoryID: stableImportID(candidate.ID, "history"), FileEventID: stableImportID(candidate.ID, "event"),
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return service.cancelCandidate(context.WithoutCancel(ctx), candidate)
		}
		return service.failCandidate(ctx, candidate, "register_failed", err)
	}
	candidate.FileID = registration.FileID
	candidate.Status = importdomain.CandidateRegistered
	candidate.UpdatedAt = service.timestamp()
	if err := service.repository.SaveCandidate(ctx, *candidate); err != nil {
		return err
	}
	return nil
}

func (service *Service) projectRegisteredCandidates(ctx context.Context, libraryID string, candidates []importdomain.Candidate) error {
	registered := make([]int, 0)
	for index := range candidates {
		if candidates[index].Status == importdomain.CandidateRegistered {
			registered = append(registered, index)
		}
	}
	if len(registered) == 0 {
		return nil
	}
	var projectionErr error
	if scoped, ok := service.projector.(ScopedCatalogProjector); ok && strings.TrimSpace(libraryID) != "" {
		_, projectionErr = scoped.RunLibrary(ctx, strings.TrimSpace(libraryID))
	} else {
		_, projectionErr = service.projector.Run(ctx)
	}
	if projectionErr != nil {
		err := projectionErr
		resultErr := err
		for _, index := range registered {
			candidate := &candidates[index]
			candidate.ErrorCode = "catalog_projection_failed"
			candidate.ErrorMessage = err.Error()
			candidate.UpdatedAt = service.timestamp()
			if saveErr := service.repository.SaveCandidate(context.WithoutCancel(ctx), *candidate); saveErr != nil {
				resultErr = errors.Join(resultErr, saveErr)
			}
		}
		return resultErr
	}
	notifier, _ := service.importer.(CatalogProjectionNotifier)
	for _, index := range registered {
		candidate := &candidates[index]
		candidate.Status = importdomain.CandidateSucceeded
		candidate.ErrorCode = ""
		candidate.ErrorMessage = ""
		candidate.UpdatedAt = service.timestamp()
		if notifier != nil {
			notifier.NotifyCatalogProjectionCompleted(ctx, candidate.FileID)
		}
		if err := service.repository.SaveCandidate(ctx, *candidate); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) markExistingDuplicates(ctx context.Context, candidates []importdomain.Candidate) ([]importdomain.Candidate, error) {
	files, err := service.files.List(ctx)
	if err != nil {
		return nil, err
	}
	bySize := make(map[int64][]library.LibraryFile)
	for _, file := range files {
		if file.State.Deleted || strings.TrimSpace(file.Storage.LocalPath) == "" {
			continue
		}
		// Physical stat is authoritative. Media metadata may be stale after an
		// external edit and must never create a false strong-dedupe decision.
		if info, statErr := os.Stat(file.Storage.LocalPath); statErr == nil && info.Mode().IsRegular() {
			bySize[info.Size()] = append(bySize[info.Size()], file)
		}
	}
	hashCache := make(map[string]string)
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.Status != importdomain.CandidateReady {
			continue
		}
		for _, file := range bySize[candidate.SizeBytes] {
			digest, cached := hashCache[file.ID]
			if !cached {
				digest, err = hashFileSHA256(ctx, file.Storage.LocalPath)
				if err != nil {
					continue
				}
				hashCache[file.ID] = digest
			}
			if digest == candidate.ContentHash {
				candidate.Status = importdomain.CandidateDuplicate
				candidate.DuplicateFileID = file.ID
				break
			}
		}
		validated, err := importdomain.NewCandidate(*candidate)
		if err != nil {
			return nil, err
		}
		*candidate = validated
	}
	return candidates, nil
}

func verifyCandidateSource(ctx context.Context, candidate importdomain.Candidate) error {
	info, err := os.Stat(candidate.SourcePath)
	if err != nil {
		return errors.Join(importdomain.ErrSourceChanged, err)
	}
	if !info.Mode().IsRegular() || info.Size() != candidate.SizeBytes {
		return importdomain.ErrSourceChanged
	}
	digest, err := hashFileSHA256(ctx, candidate.SourcePath)
	if err != nil {
		return err
	}
	if digest != candidate.ContentHash {
		return importdomain.ErrSourceChanged
	}
	return nil
}

func (service *Service) failCandidate(ctx context.Context, candidate *importdomain.Candidate, code string, cause error) error {
	candidate.Status = importdomain.CandidateFailed
	candidate.ErrorCode = strings.TrimSpace(code)
	candidate.ErrorMessage = cause.Error()
	candidate.UpdatedAt = service.timestamp()
	if err := service.repository.SaveCandidate(context.WithoutCancel(ctx), *candidate); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func (service *Service) cancelCandidate(ctx context.Context, candidate *importdomain.Candidate) error {
	candidate.Status = importdomain.CandidateCancelled
	candidate.ErrorCode = "cancelled"
	candidate.ErrorMessage = "import was cancelled"
	candidate.UpdatedAt = service.timestamp()
	if err := service.repository.SaveCandidate(ctx, *candidate); err != nil {
		return err
	}
	return context.Canceled
}

func (service *Service) finishExecution(ctx context.Context, batch importdomain.Batch, candidates []importdomain.Candidate, lastErr error) (BatchDTO, error) {
	refreshed, err := service.repository.ListCandidates(ctx, batch.ID)
	if err != nil {
		return BatchDTO{}, err
	}
	counts := importdomain.CountsFor(refreshed)
	finishedAt := service.timestamp()
	batch.Counts = counts
	batch.FinishedAt = &finishedAt
	batch.UpdatedAt = finishedAt
	if counts.Failed > 0 || counts.Ready > 0 {
		batch.Status = importdomain.BatchFailed
		batch.LastErrorCode = "candidate_failed"
		batch.LastError = "one or more import candidates require retry"
		if lastErr != nil {
			batch.LastError = lastErr.Error()
		}
	} else {
		batch.Status = importdomain.BatchCompleted
		batch.LastErrorCode = ""
		batch.LastError = ""
	}
	if err := service.repository.SaveBatch(ctx, batch); err != nil {
		return BatchDTO{}, err
	}
	return toBatchDTO(batch, refreshed), nil
}

func (service *Service) finishCancelled(ctx context.Context, batch importdomain.Batch, candidates []importdomain.Candidate, cause error) (BatchDTO, error) {
	refreshed, err := service.repository.ListCandidates(ctx, batch.ID)
	if err != nil {
		return BatchDTO{}, err
	}
	finishedAt := service.timestamp()
	batch.Status = importdomain.BatchCancelled
	batch.CancelRequested = true
	batch.Counts = importdomain.CountsFor(refreshed)
	batch.FinishedAt = &finishedAt
	batch.UpdatedAt = finishedAt
	batch.LastErrorCode = "cancelled"
	batch.LastError = "import was cancelled"
	if cause != nil {
		batch.LastError = cause.Error()
	}
	if err := service.repository.SaveBatch(ctx, batch); err != nil {
		return BatchDTO{}, err
	}
	return toBatchDTO(batch, refreshed), nil
}

func (service *Service) failBatch(ctx context.Context, batch importdomain.Batch, code string, cause error) (BatchDTO, error) {
	finishedAt := service.timestamp()
	batch.Status = importdomain.BatchFailed
	batch.LastErrorCode = code
	batch.LastError = cause.Error()
	batch.FinishedAt = &finishedAt
	batch.UpdatedAt = finishedAt
	if err := service.repository.SaveBatch(context.WithoutCancel(ctx), batch); err != nil {
		return BatchDTO{}, errors.Join(cause, err)
	}
	candidates, _ := service.repository.ListCandidates(context.WithoutCancel(ctx), batch.ID)
	return toBatchDTO(batch, candidates), cause
}

func (service *Service) registerRun(ctx context.Context, batchID string) (context.Context, context.CancelFunc, error) {
	service.runMu.Lock()
	defer service.runMu.Unlock()
	if _, exists := service.cancels[batchID]; exists {
		return nil, nil, importdomain.ErrImportAlreadyRuns
	}
	runCtx, cancel := context.WithCancel(ctx)
	service.cancels[batchID] = cancel
	return runCtx, cancel, nil
}

func (service *Service) finishRun(batchID string, cancel context.CancelFunc) {
	cancel()
	service.runMu.Lock()
	delete(service.cancels, batchID)
	service.runMu.Unlock()
}

func (service *Service) timestamp() time.Time {
	if service == nil || service.now == nil {
		return time.Now().UTC()
	}
	return service.now().UTC()
}

func stableImportID(scope, kind string) string {
	return uuid.NewSHA1(importExecutionNamespace, []byte(strings.TrimSpace(scope)+"\x00"+strings.TrimSpace(kind))).String()
}

func validateManagedRoot(raw string, mode importdomain.Mode) (string, error) {
	if mode == importdomain.ModeReferenced {
		return "", nil
	}
	if mode != importdomain.ModeCopy {
		return "", importdomain.ErrInvalidBatch
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", importdomain.ErrManagedRootMissing
	}
	root, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("managed import root must be a directory")
	}
	return filepath.Abs(resolved)
}

func batchCancellationRequested(repository importdomain.Repository, batchID string) bool {
	batch, err := repository.GetBatch(context.Background(), batchID)
	return err == nil && batch.CancelRequested
}

func firstError(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func sortCandidates(items []importdomain.Candidate) {
	sort.Slice(items, func(left, right int) bool { return items[left].SourcePath < items[right].SourcePath })
}
