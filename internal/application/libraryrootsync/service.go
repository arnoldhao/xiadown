package libraryrootsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/fileclassification"
	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
	domain "xiadown/internal/domain/libraryrootsync"
)

const (
	watcherDebounce       = 1500 * time.Millisecond
	watcherRetryInterval  = time.Minute
	fullReconcileInterval = 12 * time.Hour
	progressFlushInterval = 250 * time.Millisecond
	progressFlushFiles    = 32
	scanDiscoveryBatch    = 512
)

var rootSyncNamespace = uuid.MustParse("fa18882b-70b1-452e-ac70-067771c2738c")

type Service struct {
	repository domain.Repository
	files      FileRepository
	importer   ImportRegistrar
	projector  CatalogProjector
	notifier   ProjectionNotifier
	watcher    nativeWatcher
	hasher     func(context.Context, string) (string, error)

	mu           sync.Mutex
	provider     RootProvider
	baseContext  context.Context
	baseCancel   context.CancelFunc
	roots        map[string]Root
	runs         map[string]*scanRun
	watches      map[string]*watchRun
	pending      map[string]scanRequest
	volumeGates  map[string]chan struct{}
	shuttingDown bool
}

type scanRun struct {
	cancel        context.CancelFunc
	done          chan struct{}
	userCancelled atomic.Bool
}

type watchRun struct {
	cancel context.CancelFunc
	done   chan struct{}
	path   string
}

func NewService(
	repository domain.Repository,
	files FileRepository,
	importer ImportRegistrar,
	projector CatalogProjector,
	notifier ProjectionNotifier,
) *Service {
	baseContext, cancel := context.WithCancel(context.Background())
	return &Service{
		repository:  repository,
		files:       files,
		importer:    importer,
		projector:   projector,
		notifier:    notifier,
		watcher:     newNativeWatcher(),
		hasher:      hashFile,
		baseContext: baseContext,
		baseCancel:  cancel,
		roots:       make(map[string]Root),
		runs:        make(map[string]*scanRun),
		watches:     make(map[string]*watchRun),
		pending:     make(map[string]scanRequest),
		volumeGates: make(map[string]chan struct{}),
	}
}

func (service *Service) SetRootProvider(provider RootProvider) {
	if service == nil {
		return
	}
	service.mu.Lock()
	service.provider = provider
	service.mu.Unlock()
}

// Run owns watcher lifecycle, restores scans interrupted by a process exit, and
// performs a low-frequency correctness reconciliation. It returns when ctx is
// cancelled.
func (service *Service) Run(ctx context.Context) error {
	if err := service.validate(); err != nil {
		return err
	}
	runContext, cancel := context.WithCancel(ctx)
	service.mu.Lock()
	if service.baseCancel != nil {
		service.baseCancel()
	}
	service.baseContext = runContext
	service.baseCancel = cancel
	service.shuttingDown = false
	service.mu.Unlock()
	defer cancel()

	if err := service.repository.MarkActiveStatesInterrupted(runContext); err != nil {
		return err
	}
	if err := service.reconcileRoots(runContext, true); err != nil &&
		runContext.Err() == nil {
		return err
	}

	watcherTicker := time.NewTicker(watcherRetryInterval)
	defer watcherTicker.Stop()
	reconcileTicker := time.NewTicker(fullReconcileInterval)
	defer reconcileTicker.Stop()
	for {
		select {
		case <-runContext.Done():
			service.stopAll()
			return runContext.Err()
		case <-watcherTicker.C:
			_ = service.reconcileRoots(runContext, false)
		case <-reconcileTicker.C:
			if err := service.reconcileRoots(runContext, false); err == nil {
				service.enqueueAllOnlineRoots()
			}
		}
	}
}

func (service *Service) ListStates(ctx context.Context) ([]StateDTO, error) {
	if err := service.validate(); err != nil {
		return nil, err
	}
	items, err := service.repository.ListStates(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StateDTO, 0, len(items))
	for _, item := range items {
		result = append(result, stateDTO(item))
	}
	return result, nil
}

func (service *Service) StartRootScan(
	ctx context.Context,
	request RootRequest,
) (StateDTO, error) {
	if err := service.validate(); err != nil {
		return StateDTO{}, err
	}
	root, err := service.resolveRoot(ctx, request.RootID)
	if err != nil {
		return StateDTO{}, err
	}
	if !root.Online {
		return StateDTO{}, fmt.Errorf("storage root %q is offline", root.ID)
	}
	if !isScannableRoot(root) {
		return StateDTO{}, fmt.Errorf("storage root mode %q cannot be scanned", root.Mode)
	}
	item, err := service.repository.GetState(ctx, root.ID)
	if errors.Is(err, domain.ErrStateNotFound) {
		now := time.Now().UTC()
		item, err = domain.NewState(domain.State{
			RootID: root.ID, Status: domain.StatusQueued,
			FullScan: true, CreatedAt: now, UpdatedAt: now,
		})
		if err == nil {
			err = service.repository.SaveState(ctx, item)
		}
	} else if err == nil &&
		item.Status != domain.StatusQueued &&
		item.Status != domain.StatusScanning &&
		item.Status != domain.StatusCancelling {
		item.Status = domain.StatusQueued
		item.FullScan = true
		item.CancelRequested = false
		item.UpdatedAt = time.Now().UTC()
		err = service.repository.SaveState(ctx, item)
	}
	if err != nil {
		return StateDTO{}, err
	}
	service.startWatcher(root)
	service.enqueue(root, scanRequest{full: true})
	return stateDTO(item), nil
}

func (service *Service) CancelRootScan(
	ctx context.Context,
	request RootRequest,
) (StateDTO, error) {
	rootID := strings.TrimSpace(request.RootID)
	if rootID == "" {
		return StateDTO{}, fmt.Errorf("root id is required")
	}
	item, err := service.repository.GetState(ctx, rootID)
	if err != nil {
		return StateDTO{}, err
	}
	if item.Status != domain.StatusQueued &&
		item.Status != domain.StatusScanning &&
		item.Status != domain.StatusCancelling {
		return stateDTO(item), nil
	}
	service.mu.Lock()
	run := service.runs[rootID]
	delete(service.pending, rootID)
	service.mu.Unlock()
	if run == nil {
		switch item.Status {
		case domain.StatusQueued,
			domain.StatusScanning,
			domain.StatusCancelling:
			now := time.Now().UTC()
			item.Status = domain.StatusCancelled
			item.CancelRequested = false
			item.FinishedAt = &now
			item.UpdatedAt = now
			if err := service.repository.SaveState(ctx, item); err != nil {
				return StateDTO{}, err
			}
		}
		return stateDTO(item), nil
	}
	now := time.Now().UTC()
	item.Status = domain.StatusCancelling
	item.CancelRequested = true
	item.UpdatedAt = now
	if err := service.repository.SaveState(ctx, item); err != nil {
		return StateDTO{}, err
	}
	run.userCancelled.Store(true)
	run.cancel()
	return stateDTO(item), nil
}

// StopRoot drains in-flight work before a storage-root row is removed. It never
// deletes or mutates files on disk.
func (service *Service) StopRoot(ctx context.Context, rootID string) error {
	rootID = strings.TrimSpace(rootID)
	if rootID == "" {
		return fmt.Errorf("root id is required")
	}
	service.mu.Lock()
	delete(service.pending, rootID)
	delete(service.roots, rootID)
	run := service.runs[rootID]
	watch := service.watches[rootID]
	if run != nil {
		run.cancel()
	}
	if watch != nil {
		watch.cancel()
	}
	service.mu.Unlock()
	if err := waitDone(ctx, run); err != nil {
		return err
	}
	return waitWatchDone(ctx, watch)
}

func (service *Service) validate() error {
	if service == nil || service.repository == nil || service.files == nil ||
		service.importer == nil || service.projector == nil {
		return fmt.Errorf("Library root sync service is not configured")
	}
	service.mu.Lock()
	provider := service.provider
	service.mu.Unlock()
	if provider == nil {
		return fmt.Errorf("Library root sync root provider is not configured")
	}
	return nil
}

func (service *Service) reconcileRoots(ctx context.Context, restore bool) error {
	service.mu.Lock()
	provider := service.provider
	service.mu.Unlock()
	if provider == nil {
		return fmt.Errorf("Library root sync root provider is not configured")
	}
	roots, err := provider(ctx)
	if err != nil {
		return err
	}
	current := make(map[string]Root, len(roots))
	for _, root := range roots {
		root.ID = strings.TrimSpace(root.ID)
		root.Path = strings.TrimSpace(root.Path)
		if root.ID == "" || root.Path == "" {
			continue
		}
		current[root.ID] = root
	}
	var filesForRepair []library.LibraryFile
	if restore {
		filesForRepair, err = service.files.List(ctx)
		if err != nil {
			return err
		}
	}
	service.mu.Lock()
	previous := service.roots
	service.roots = current
	stale := make([]*watchRun, 0)
	for rootID, watch := range service.watches {
		root, ok := current[rootID]
		if !ok || !root.Online || !isScannableRoot(root) ||
			canonicalPath(root.Path) != canonicalPath(watch.path) {
			delete(service.watches, rootID)
			stale = append(stale, watch)
		}
	}
	service.mu.Unlock()
	for _, watch := range stale {
		watch.cancel()
	}
	for _, root := range current {
		if !root.Online || !isScannableRoot(root) {
			continue
		}
		needsFullScan := false
		service.mu.Lock()
		_, watcherRunning := service.watches[root.ID]
		service.mu.Unlock()
		if restore {
			state, stateErr := service.repository.GetState(ctx, root.ID)
			switch {
			case errors.Is(stateErr, domain.ErrStateNotFound):
				now := time.Now().UTC()
				state, stateErr = domain.NewState(domain.State{
					RootID: root.ID, Status: domain.StatusQueued,
					FullScan: true, CreatedAt: now, UpdatedAt: now,
				})
				if stateErr == nil {
					stateErr = service.repository.SaveState(ctx, state)
				}
				if stateErr != nil {
					return stateErr
				}
				needsFullScan = true
			case stateErr != nil:
				return stateErr
			case state.Status == domain.StatusInterrupted:
				needsFullScan = true
			case !service.watcher.SupportsReplay() ||
				state.WatcherCursor == 0:
				needsFullScan = true
			}
		} else if prior, existed := previous[root.ID]; !existed ||
			!prior.Online ||
			canonicalPath(prior.Path) != canonicalPath(root.Path) {
			needsFullScan = true
		} else if service.watcher.Available() &&
			!service.watcher.SupportsReplay() &&
			!watcherRunning {
			needsFullScan = true
		}
		service.startWatcher(root)
		if needsFullScan {
			service.enqueue(root, scanRequest{full: true})
		} else if restore {
			// A file can be restored while XiaDown is not running, before a
			// native watcher cursor is resumed. Recheck only prior missing
			// paths on startup instead of walking the entire root.
			missingEntries, listErr := service.repository.ListEntriesByStatus(
				ctx,
				root.ID,
				domain.EntryMissing,
			)
			if listErr != nil {
				return listErr
			}
			repairPaths := ambiguousClassificationRepairPaths(root, filesForRepair)
			for _, entry := range missingEntries {
				repairPaths[filepath.Join(
					root.Path,
					filepath.FromSlash(entry.RelativePath),
				)] = struct{}{}
			}
			if len(repairPaths) > 0 {
				service.enqueue(root, scanRequest{paths: repairPaths})
			}
		}
	}
	return nil
}

func (service *Service) resolveRoot(ctx context.Context, rootID string) (Root, error) {
	rootID = strings.TrimSpace(rootID)
	if rootID == "" {
		return Root{}, fmt.Errorf("root id is required")
	}
	service.mu.Lock()
	provider := service.provider
	service.mu.Unlock()
	if provider == nil {
		return Root{}, fmt.Errorf("Library root sync root provider is not configured")
	}
	roots, err := provider(ctx)
	if err != nil {
		return Root{}, err
	}
	var root Root
	ok := false
	service.mu.Lock()
	defer service.mu.Unlock()
	for _, candidate := range roots {
		service.roots[candidate.ID] = candidate
		if candidate.ID == rootID {
			root = candidate
			ok = true
		}
	}
	if !ok {
		return Root{}, fmt.Errorf("storage root %q not found", rootID)
	}
	return root, nil
}

func (service *Service) enqueueAllOnlineRoots() {
	service.mu.Lock()
	roots := make([]Root, 0, len(service.roots))
	for _, root := range service.roots {
		if root.Online && isScannableRoot(root) {
			roots = append(roots, root)
		}
	}
	service.mu.Unlock()
	for _, root := range roots {
		service.enqueue(root, scanRequest{full: true})
	}
}

func isScannableRoot(root Root) bool {
	switch strings.ToLower(strings.TrimSpace(root.Mode)) {
	case "managed", "referenced":
		return true
	default:
		return false
	}
}

func ambiguousClassificationRepairPaths(
	root Root,
	files []library.LibraryFile,
) map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range files {
		path := strings.TrimSpace(item.Storage.LocalPath)
		if item.State.Deleted ||
			item.Kind != library.FileKindVideo ||
			!fileclassification.IsAmbiguousMPEGTransportPath(path) ||
			hasMediaClassificationEvidence(item.Media) ||
			!pathWithin(canonicalPath(path), canonicalPath(root.Path)) {
			continue
		}
		result[path] = struct{}{}
	}
	return result
}

func hasMediaClassificationEvidence(media *library.MediaInfo) bool {
	return media != nil &&
		(strings.TrimSpace(media.Format) != "" ||
			strings.TrimSpace(media.VideoCodec) != "" ||
			strings.TrimSpace(media.AudioCodec) != "")
}

func (service *Service) enqueue(root Root, request scanRequest) {
	service.mu.Lock()
	if service.shuttingDown {
		service.mu.Unlock()
		return
	}
	current, exists := service.roots[root.ID]
	if !exists ||
		canonicalPath(current.Path) != canonicalPath(root.Path) {
		service.mu.Unlock()
		return
	}
	if _, active := service.runs[root.ID]; active {
		service.pending[root.ID] = mergeScanRequests(service.pending[root.ID], request)
		service.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(service.baseContext)
	run := &scanRun{cancel: cancel, done: make(chan struct{})}
	service.runs[root.ID] = run
	service.mu.Unlock()
	go service.runScan(ctx, root, request, run)
}

func mergeScanRequests(current, next scanRequest) scanRequest {
	if current.paths == nil {
		current.paths = make(map[string]struct{})
	}
	current.full = current.full || next.full || next.overflowEquivalent()
	current.settle = current.settle || next.settle
	if next.cursor > current.cursor {
		current.cursor = next.cursor
	}
	if current.full {
		current.paths = nil
		return current
	}
	for path := range next.paths {
		current.paths[path] = struct{}{}
	}
	return current
}

func (request scanRequest) overflowEquivalent() bool {
	return !request.full && request.paths == nil
}

func (service *Service) runScan(
	ctx context.Context,
	root Root,
	request scanRequest,
	run *scanRun,
) {
	defer close(run.done)
	err := service.markQueued(ctx, root, request)
	if err == nil {
		err = service.performScan(ctx, root, request)
	}
	service.finishScan(root, request, err, run.userCancelled.Load())

	service.mu.Lock()
	delete(service.runs, root.ID)
	next, pending := service.pending[root.ID]
	delete(service.pending, root.ID)
	if run.userCancelled.Load() {
		pending = false
	}
	_, stillPresent := service.roots[root.ID]
	service.mu.Unlock()
	if pending && stillPresent {
		service.enqueue(root, next)
	}
}

func (service *Service) markQueued(
	ctx context.Context,
	root Root,
	request scanRequest,
) error {
	now := time.Now().UTC()
	state, err := service.repository.GetState(ctx, root.ID)
	if errors.Is(err, domain.ErrStateNotFound) {
		state, err = domain.NewState(domain.State{
			RootID: root.ID, Status: domain.StatusQueued,
			FullScan: request.full, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		return service.repository.SaveState(ctx, state)
	}
	if err != nil {
		return err
	}
	state.Status = domain.StatusQueued
	state.FullScan = request.full
	state.DiscoveredCount = 0
	state.ProcessedCount = 0
	state.UnchangedCount = 0
	state.DuplicateCount = 0
	state.MissingCount = 0
	state.FailedCount = 0
	state.ProcessedBytes = 0
	state.CancelRequested = false
	state.StartedAt = nil
	state.FinishedAt = nil
	state.UpdatedAt = now
	return service.repository.SaveState(ctx, state)
}

func (service *Service) performScan(
	ctx context.Context,
	root Root,
	request scanRequest,
) error {
	release, err := service.acquireVolume(ctx, root)
	if err != nil {
		return err
	}
	defer release()

	now := time.Now().UTC()
	state, err := service.repository.GetState(ctx, root.ID)
	if errors.Is(err, domain.ErrStateNotFound) {
		state, err = domain.NewState(domain.State{
			RootID: root.ID, Status: domain.StatusIdle,
			CreatedAt: now, UpdatedAt: now,
		})
	}
	if err != nil {
		return err
	}
	state.Status = domain.StatusScanning
	state.Generation++
	state.FullScan = request.full
	state.DiscoveredCount = 0
	state.ProcessedCount = 0
	state.UnchangedCount = 0
	state.DuplicateCount = 0
	state.MissingCount = 0
	state.FailedCount = 0
	state.ProcessedBytes = 0
	state.CancelRequested = false
	state.StartedAt = &now
	state.FinishedAt = nil
	state.UpdatedAt = now
	if err := service.repository.SaveState(ctx, state); err != nil {
		return err
	}

	libraryID := stableID("library", root.ID)
	if _, err := service.importer.EnsureProfessionalImportLibrary(
		ctx,
		libraryID,
		root.Name,
	); err != nil {
		return err
	}
	pathFiles, err := service.libraryFilesByPath(ctx)
	if err != nil {
		return err
	}

	targetPaths := make([]string, 0, len(request.paths))
	for path := range request.paths {
		targetPaths = append(targetPaths, path)
	}
	if request.full {
		targetPaths = nil
	}
	changedFiles := make(map[string]struct{})
	lastFlush := time.Now()
	sinceFlush := 0
	flush := func(force bool) error {
		if !force && sinceFlush < progressFlushFiles &&
			time.Since(lastFlush) < progressFlushInterval {
			return nil
		}
		state.UpdatedAt = time.Now().UTC()
		if request.cursor > state.WatcherCursor {
			state.WatcherCursor = request.cursor
		}
		if err := service.repository.SaveState(ctx, state); err != nil {
			return err
		}
		lastFlush = time.Now()
		sinceFlush = 0
		return nil
	}
	process := func(item discoveredFile) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if item.info == nil {
			count, err := service.repository.MarkPathMissing(
				ctx,
				root.ID,
				item.relative,
				true,
				state.Generation,
			)
			if err != nil {
				return err
			}
			state.MissingCount += count
			state.ProcessedCount++
			return flush(false)
		}

		existing, entryErr := service.repository.GetEntry(
			ctx,
			root.ID,
			item.relative,
		)
		if entryErr != nil && !errors.Is(entryErr, domain.ErrEntryNotFound) {
			return entryErr
		}
		resolvedKind := ""
		needsReclassification := false
		if fileclassification.IsAmbiguousMPEGTransportPath(item.path) {
			resolvedKind = service.resolvedFileKind(ctx, item.path)
			if known, exists := pathFiles[canonicalPath(item.path)]; exists {
				needsReclassification = string(known.Kind) != resolvedKind
			}
		}
		fingerprintReusable := entryErr == nil &&
			existing.FingerprintMatches(
				item.info.Size(),
				item.info.ModTime().UnixNano(),
			)
		if fingerprintReusable && existing.Status == domain.EntryDuplicate {
			var verifyErr error
			fingerprintReusable, verifyErr = service.duplicateCanonicalPresent(
				ctx,
				root,
				existing,
			)
			if verifyErr != nil {
				return verifyErr
			}
		}
		if fingerprintReusable && !needsReclassification {
			existing.LastSeenGeneration = state.Generation
			existing.UpdatedAt = time.Now().UTC()
			if err := service.repository.UpsertEntry(ctx, existing); err != nil {
				return err
			}
			state.UnchangedCount++
			state.ProcessedCount++
			state.ProcessedBytes += item.info.Size()
			return flush(false)
		}

		info := item.info
		if request.settle {
			latest, statErr := os.Stat(item.path)
			if statErr != nil {
				return service.recordFileFailure(
					ctx, root, state.Generation, item, statErr, &state, flush,
				)
			}
			if latest.Size() != info.Size() ||
				latest.ModTime().UnixNano() != info.ModTime().UnixNano() {
				service.queuePath(root, item.path, request.cursor)
				state.UnchangedCount++
				state.ProcessedCount++
				return flush(false)
			}
			info = latest
		}
		fileID := ""
		if entryErr == nil {
			fileID = existing.FileID
		}
		contentHash, duplicate, hashErr, identityErr :=
			service.resolvePotentialDuplicate(
				ctx,
				root,
				item.relative,
				info,
			)
		if identityErr != nil {
			return identityErr
		}
		if hashErr != nil {
			return service.recordFileFailure(
				ctx, root, state.Generation, item, hashErr, &state, flush,
			)
		}
		if duplicate != nil && duplicate.present {
			return service.recordDuplicate(
				ctx, root, state.Generation, item, info, contentHash,
				&state, flush,
			)
		}
		if duplicate != nil {
			missing, missingErr := service.repository.MarkPathMissing(
				ctx,
				root.ID,
				duplicate.entry.RelativePath,
				false,
				state.Generation,
			)
			if missingErr != nil {
				return missingErr
			}
			state.MissingCount += missing
			fileID = duplicate.entry.FileID
		}
		if fileID == "" {
			if known, exists := pathFiles[canonicalPath(item.path)]; exists {
				if resolvedKind == "" {
					resolvedKind = service.resolvedFileKind(ctx, item.path)
				}
				if fileclassification.IsAmbiguousMPEGTransportPath(item.path) &&
					string(known.Kind) != resolvedKind {
					fileID = known.ID
				} else {
					entry, entryErr := domain.NewEntry(domain.Entry{
						RootID: root.ID, RelativePath: item.relative,
						SizeBytes: info.Size(), ModifiedUnixNano: info.ModTime().UnixNano(),
						ContentHash: contentHash, FileID: known.ID,
						Status: domain.EntryActive, LastSeenGeneration: state.Generation,
						CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
					})
					if entryErr != nil {
						return entryErr
					}
					if err := service.repository.UpsertEntry(ctx, entry); err != nil {
						return err
					}
					state.UnchangedCount++
					state.ProcessedCount++
					state.ProcessedBytes += info.Size()
					return flush(false)
				}
			}
		}
		if fileID == "" {
			fileID = stableID("file", root.ID, item.relative)
		}
		if resolvedKind == "" {
			resolvedKind = service.resolvedFileKind(ctx, item.path)
		}
		registration, registerErr := service.importer.RegisterProfessionalImport(
			ctx,
			libraryservice.ProfessionalImportRequest{
				BatchID: stableID(
					"batch",
					root.ID,
					fmt.Sprint(state.Generation),
				),
				CandidateID: item.relative,
				LibraryID:   libraryID,
				SourcePath:  item.path,
				StoragePath: item.path,
				DisplayName: filepath.Base(item.path),
				Kind:        resolvedKind,
				SessionRunID: stableID(
					"session",
					root.ID,
					fmt.Sprint(state.Generation),
				),
				FileID: fileID,
				HistoryID: stableID(
					"history",
					root.ID,
					item.relative,
					fmt.Sprint(state.Generation),
				),
				FileEventID: stableID(
					"event",
					root.ID,
					item.relative,
					fmt.Sprint(state.Generation),
				),
			},
		)
		if registerErr != nil {
			return service.recordFileFailure(
				ctx, root, state.Generation, item, registerErr, &state, flush,
			)
		}
		entry, entryErr := domain.NewEntry(domain.Entry{
			RootID: root.ID, RelativePath: item.relative,
			SizeBytes: info.Size(), ModifiedUnixNano: info.ModTime().UnixNano(),
			ContentHash: contentHash, FileID: registration.FileID,
			Status: domain.EntryActive, LastSeenGeneration: state.Generation,
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		})
		if entryErr != nil {
			return entryErr
		}
		if err := service.repository.UpsertEntry(ctx, entry); err != nil {
			return err
		}
		changedFiles[registration.FileID] = struct{}{}
		state.ProcessedCount++
		state.ProcessedBytes += info.Size()
		return flush(false)
	}
	// Discover ahead of registration so the UI receives a useful denominator,
	// but drain in bounded batches so very large roots cannot grow memory
	// proportionally to their file count.
	pending := make([]discoveredFile, 0, scanDiscoveryBatch)
	processPending := func() error {
		for _, item := range pending {
			if err := process(item); err != nil {
				return err
			}
		}
		pending = pending[:0]
		return nil
	}
	discover := func(item discoveredFile) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		pending = append(pending, item)
		state.DiscoveredCount++
		sinceFlush++
		if err := flush(state.DiscoveredCount == 1); err != nil {
			return err
		}
		if len(pending) < scanDiscoveryBatch {
			return nil
		}
		if err := flush(true); err != nil {
			return err
		}
		return processPending()
	}
	if err := walkScanTargets(ctx, root.Path, targetPaths, discover); err != nil {
		return err
	}
	if err := flush(true); err != nil {
		return err
	}
	if err := processPending(); err != nil {
		return err
	}
	if request.full {
		missing, err := service.repository.MarkUnseenEntriesMissing(
			ctx,
			root.ID,
			state.Generation,
		)
		if err != nil {
			return err
		}
		state.MissingCount += missing
	}
	if err := flush(true); err != nil {
		return err
	}
	if scoped, ok := service.projector.(ScopedCatalogProjector); ok {
		_, err = scoped.RunLibrary(ctx, libraryID)
	} else {
		_, err = service.projector.Run(ctx)
	}
	if err != nil {
		return err
	}
	if service.notifier != nil {
		if batch, ok := service.notifier.(CatalogProjectionBatchNotifier); ok {
			fileIDs := make([]string, 0, len(changedFiles))
			for fileID := range changedFiles {
				fileIDs = append(fileIDs, fileID)
			}
			sort.Strings(fileIDs)
			batch.NotifyCatalogProjectionBatchCompleted(ctx, root.ID, fileIDs)
		} else {
			for fileID := range changedFiles {
				service.notifier.NotifyCatalogProjectionCompleted(ctx, fileID)
			}
		}
		if notifier, ok := service.notifier.(CatalogAvailabilityNotifier); ok {
			notifier.NotifyCatalogAvailabilityChanged(ctx, root.ID)
		}
	}
	return nil
}

type contentMatch struct {
	entry   domain.Entry
	present bool
}

// duplicateCanonicalPresent keeps unchanged duplicate scans metadata-only while
// ensuring a duplicate can take ownership when its active canonical copy has
// disappeared or changed.
func (service *Service) duplicateCanonicalPresent(
	ctx context.Context,
	root Root,
	duplicate domain.Entry,
) (bool, error) {
	canonical, err := service.repository.FindActiveEntryByDigest(
		ctx,
		root.ID,
		duplicate.SizeBytes,
		duplicate.ContentHash,
	)
	if errors.Is(err, domain.ErrEntryNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	canonicalPath := filepath.Join(
		root.Path,
		filepath.FromSlash(canonical.RelativePath),
	)
	if !pathWithin(canonicalPath, root.Path) {
		return false, nil
	}
	info, err := os.Lstat(canonicalPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("verify canonical duplicate entry: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return false, nil
	}
	return canonical.FingerprintMatches(
		info.Size(),
		info.ModTime().UnixNano(),
	), nil
}

// resolvePotentialDuplicate keeps the common registration path metadata-only.
// A full-file digest is calculated only when another active entry has the same
// byte length, which preserves exact duplicate detection without rereading
// every large media file during its first scan.
func (service *Service) resolvePotentialDuplicate(
	ctx context.Context,
	root Root,
	relativePath string,
	info os.FileInfo,
) (string, *contentMatch, error, error) {
	candidates, err := service.repository.ListActiveEntriesBySize(
		ctx,
		root.ID,
		info.Size(),
	)
	if err != nil {
		return "", nil, nil, err
	}
	currentHash := ""
	var missingMatch *contentMatch
	for _, candidate := range candidates {
		if candidate.RelativePath == relativePath {
			continue
		}
		candidatePath := filepath.Join(
			root.Path,
			filepath.FromSlash(candidate.RelativePath),
		)
		if !pathWithin(candidatePath, root.Path) {
			continue
		}
		candidateInfo, statErr := os.Lstat(candidatePath)
		candidatePresent := statErr == nil &&
			candidateInfo.Mode().IsRegular() &&
			candidateInfo.Mode()&os.ModeSymlink == 0
		candidateMissing := errors.Is(statErr, os.ErrNotExist)
		if !candidatePresent && !candidateMissing {
			continue
		}

		candidateHash := candidate.ContentHash
		fingerprintCurrent := candidatePresent &&
			candidate.SizeBytes == candidateInfo.Size() &&
			candidate.ModifiedUnixNano == candidateInfo.ModTime().UnixNano()
		if candidatePresent && !fingerprintCurrent {
			if candidateInfo.Size() != info.Size() {
				continue
			}
			candidateHash = ""
		}
		if candidatePresent && candidateHash == "" {
			candidateHash, err = service.hashPath(ctx, candidatePath)
			if err != nil {
				if ctx.Err() != nil {
					return "", nil, nil, ctx.Err()
				}
				continue
			}
			if fingerprintCurrent {
				candidate.ContentHash = candidateHash
				candidate.UpdatedAt = time.Now().UTC()
				if err := service.repository.UpsertEntry(ctx, candidate); err != nil {
					return "", nil, nil, err
				}
			}
		}
		if candidateHash == "" {
			continue
		}
		if currentHash == "" {
			currentHash, err = service.hashPath(ctx, filepath.Join(
				root.Path,
				filepath.FromSlash(relativePath),
			))
			if err != nil {
				return "", nil, err, nil
			}
		}
		if currentHash != candidateHash {
			continue
		}
		match := &contentMatch{entry: candidate, present: candidatePresent}
		if candidatePresent {
			return currentHash, match, nil, nil
		}
		if missingMatch == nil {
			missingMatch = match
		}
	}
	return currentHash, missingMatch, nil, nil
}

func (service *Service) hashPath(
	ctx context.Context,
	path string,
) (string, error) {
	if service.hasher == nil {
		return hashFile(ctx, path)
	}
	return service.hasher(ctx, path)
}

func (service *Service) recordDuplicate(
	ctx context.Context,
	root Root,
	generation int64,
	item discoveredFile,
	info os.FileInfo,
	contentHash string,
	state *domain.State,
	flush func(bool) error,
) error {
	entry, err := domain.NewEntry(domain.Entry{
		RootID: root.ID, RelativePath: item.relative,
		SizeBytes: info.Size(), ModifiedUnixNano: info.ModTime().UnixNano(),
		ContentHash: contentHash, Status: domain.EntryDuplicate,
		LastSeenGeneration: generation,
		CreatedAt:          time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	if err := service.repository.UpsertEntry(ctx, entry); err != nil {
		return err
	}
	state.DuplicateCount++
	state.ProcessedCount++
	state.ProcessedBytes += info.Size()
	return flush(false)
}

func (service *Service) recordFileFailure(
	ctx context.Context,
	root Root,
	generation int64,
	item discoveredFile,
	cause error,
	state *domain.State,
	flush func(bool) error,
) error {
	if errors.Is(cause, context.Canceled) ||
		errors.Is(cause, context.DeadlineExceeded) {
		return cause
	}
	size := int64(0)
	modified := int64(0)
	if item.info != nil {
		size = item.info.Size()
		modified = item.info.ModTime().UnixNano()
	}
	entry, err := domain.NewEntry(domain.Entry{
		RootID: root.ID, RelativePath: item.relative,
		SizeBytes: size, ModifiedUnixNano: modified,
		Status: domain.EntryFailed, LastSeenGeneration: generation,
		LastError: cause.Error(),
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	if err := service.repository.UpsertEntry(ctx, entry); err != nil {
		return err
	}
	state.FailedCount++
	state.ProcessedCount++
	return flush(false)
}

func (service *Service) finishScan(
	root Root,
	request scanRequest,
	scanErr error,
	userCancelled bool,
) {
	ctx := context.Background()
	state, err := service.repository.GetState(ctx, root.ID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	state.CancelRequested = false
	state.FinishedAt = &now
	state.UpdatedAt = now
	if request.cursor > state.WatcherCursor {
		state.WatcherCursor = request.cursor
	}
	switch {
	case scanErr == nil:
		state.Status = domain.StatusWatching
		state.LastReconciledAt = &now
	case userCancelled ||
		state.Status == domain.StatusCancelling ||
		state.CancelRequested:
		state.Status = domain.StatusCancelled
	case errors.Is(scanErr, context.Canceled):
		state.Status = domain.StatusInterrupted
	default:
		state.Status = domain.StatusFailed
		state.LastErrorCode = "scan_failed"
		state.LastError = scanErr.Error()
	}
	_ = service.repository.SaveState(ctx, state)
}

func (service *Service) libraryFilesByPath(
	ctx context.Context,
) (map[string]library.LibraryFile, error) {
	items, err := service.files.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]library.LibraryFile, len(items))
	for _, item := range items {
		path := strings.TrimSpace(item.Storage.LocalPath)
		if path == "" || item.State.Deleted {
			continue
		}
		result[canonicalPath(path)] = item
	}
	return result, nil
}

func (service *Service) acquireVolume(
	ctx context.Context,
	root Root,
) (func(), error) {
	key := strings.TrimSpace(root.VolumeID)
	if key == "" {
		key = filepath.VolumeName(root.Path)
	}
	if key == "" {
		key = root.ID
	}
	service.mu.Lock()
	gate := service.volumeGates[key]
	if gate == nil {
		gate = make(chan struct{}, 1)
		service.volumeGates[key] = gate
	}
	service.mu.Unlock()
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func stableID(parts ...string) string {
	return uuid.NewSHA1(
		rootSyncNamespace,
		[]byte(strings.Join(parts, "\x00")),
	).String()
}

func waitDone(ctx context.Context, run *scanRun) error {
	if run == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-run.done:
		return nil
	}
}

func waitWatchDone(ctx context.Context, run *watchRun) error {
	if run == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-run.done:
		return nil
	}
}

func (service *Service) stopAll() {
	service.mu.Lock()
	service.shuttingDown = true
	service.pending = make(map[string]scanRequest)
	runs := make([]*scanRun, 0, len(service.runs))
	for _, run := range service.runs {
		run.cancel()
		runs = append(runs, run)
	}
	watches := make([]*watchRun, 0, len(service.watches))
	for _, watch := range service.watches {
		watch.cancel()
		watches = append(watches, watch)
	}
	service.mu.Unlock()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for _, run := range runs {
		select {
		case <-run.done:
		case <-timer.C:
			return
		}
	}
	for _, watch := range watches {
		select {
		case <-watch.done:
		case <-timer.C:
			return
		}
	}
}
