package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/apperrors"
	appsessionsdto "xiadown/internal/application/appsessions/dto"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	"xiadown/internal/application/events"
	"xiadown/internal/application/library/dto"
	settingsdto "xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/dependencies"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/opener"
)

type settingsReader interface {
	GetSettings(ctx context.Context) (settingsdto.Settings, error)
}

type appSessionReader interface {
	ListAppSessions(ctx context.Context) ([]appsessionsdto.AppSession, error)
	ExportAppSessionCookies(ctx context.Context, id string, format appsessionsservice.CookiesExportFormat) (string, error)
}

type iconResolver interface {
	ResolveDomainIcon(ctx context.Context, domain string) (string, error)
}

type ToolResolver interface {
	ResolveExecPath(ctx context.Context, name dependencies.DependencyName) (string, error)
	ResolveDependencyDirectory(ctx context.Context, name dependencies.DependencyName) (string, error)
	DependencyReadiness(ctx context.Context, name dependencies.DependencyName) (bool, string, error)
}

// CatalogProjectionRunner keeps the Catalog view synchronized with the
// durable LibraryFile registry. LibraryFile remains the retry source if a
// projection attempt is interrupted after the physical record is committed.
type CatalogProjectionRunner interface {
	Run(ctx context.Context) (CatalogBackfillResult, error)
}

type scopedCatalogProjectionRunner interface {
	RunLibrary(ctx context.Context, libraryID string) (CatalogBackfillResult, error)
}

type databaseIntegrityStatusProvider func() (state, checkedAt, detail string)

// listenLocalCatalogMetadataSynchronizer updates the user-facing Catalog item
// after embedded audio tags have been committed to a legacy LibraryFile. The
// catalog projection owns stable file-to-item associations, while this
// boundary owns the mutable title/artist fields shown throughout Library.
type listenLocalCatalogMetadataSynchronizer interface {
	SyncListenLocalTrackMetadata(
		ctx context.Context,
		file library.LibraryFile,
		metadata dto.UpdateListenLocalTrackMetadataRequest,
	) error
}

type LibraryService struct {
	libraries                library.LibraryRepository
	moduleConfig             library.ModuleConfigRepository
	files                    library.FileRepository
	localTracks              library.ListenLocalTrackRepository
	localPlaylists           library.ListenLocalPlaylistRepository
	localMusicMemberships    library.ListenLocalMusicMembershipRepository
	operations               library.OperationRepository
	processes                library.ExternalProcessRepository
	operationChunks          library.OperationChunkRepository
	histories                library.HistoryRepository
	workspace                library.WorkspaceStateRepository
	fileEvents               library.FileEventRepository
	subtitles                library.SubtitleDocumentRepository
	presets                  library.TranscodePresetRepository
	settings                 settingsReader
	iconResolver             iconResolver
	tools                    ToolResolver
	proxyClient              any
	appSessions              appSessionReader
	bus                      events.Bus
	databaseIntegrity        databaseIntegrityStatusProvider
	catalogProjection        CatalogProjectionRunner
	catalogMetadataSync      listenLocalCatalogMetadataSynchronizer
	storageRoots             library.StorageRootRepository
	defaultDownloadRoot      func(context.Context) (string, error)
	embeddedArtworkDirectory string
	embeddedArtworkExtractor func(context.Context, string, string) error
	embeddedArtworkMu        sync.Mutex
	nowFunc                  func() time.Time
	runMu                    sync.Mutex
	runCancels               map[string]context.CancelFunc
	runDone                  map[string]chan struct{}
	localPlaylistMutationMu  sync.Mutex
	musicIOSRepresentationMu sync.Mutex
	// operationOutputMutationMu serializes operation read/modify/write paths
	// which carry the complete output snapshot, including output detach/delete
	// and title rename. Sharing one lock prevents either path from restoring a
	// stale sibling snapshot or recording the wrong pre-mutation activity.
	operationOutputMutationMu sync.Mutex
	// localTrackMutationMu serializes every mutation of a single local-track
	// record (including its backing path) without turning a full index refresh
	// into a single-threaded operation. Access it through
	// lockListenLocalTrackMutation; direct locking would break the stable shard
	// ordering used by bulk maintenance operations.
	localTrackMutationMu      [listenLocalTrackMutationShardCount]sync.Mutex
	localMetadataWriter       func(context.Context, string, dto.UpdateListenLocalTrackMetadataRequest) error
	downloadSchedulerMu       sync.Mutex
	downloadSchedulerActive   map[string]struct{}
	downloadSchedulerRunning  bool
	downloadSchedulerPending  bool
	downloadSchedulerDone     chan struct{}
	shuttingDown              bool
	resourceSniffLifecycleMu  sync.Mutex
	resourceSniffMu           sync.Mutex
	resourceSniffs            map[string]*resourceSniffSession
	resourcePreviewLeases     map[string]resourceSniffPreviewLease
	resourceMedia             map[string]resourceMedia
	resourceMediaMeta         map[string]resourceMediaSnapshotMetadata
	resourceMediaCleanupTimer *time.Timer
}

const operationErrorCodeAppInterrupted = "app_interrupted"

// SetDatabaseIntegrityStatusProvider connects the infrastructure-owned
// background SQLite check to the Library maintenance read model. Keeping this
// as a narrow callback avoids coupling application services to persistence.
func (service *LibraryService) SetDatabaseIntegrityStatusProvider(provider func() (state, checkedAt, detail string)) {
	if service == nil {
		return
	}
	service.databaseIntegrity = provider
}

func NewLibraryService(
	libraries library.LibraryRepository,
	moduleConfig library.ModuleConfigRepository,
	files library.FileRepository,
	localTracks library.ListenLocalTrackRepository,
	localPlaylists library.ListenLocalPlaylistRepository,
	operations library.OperationRepository,
	processes library.ExternalProcessRepository,
	operationChunks library.OperationChunkRepository,
	histories library.HistoryRepository,
	workspace library.WorkspaceStateRepository,
	fileEvents library.FileEventRepository,
	subtitles library.SubtitleDocumentRepository,
	presets library.TranscodePresetRepository,
	settings settingsReader,
	iconResolver iconResolver,
	tools ToolResolver,
	proxyClient any,
	appSessions appSessionReader,
	bus events.Bus,
) *LibraryService {
	return &LibraryService{
		libraries:               libraries,
		moduleConfig:            moduleConfig,
		files:                   files,
		localTracks:             localTracks,
		localPlaylists:          localPlaylists,
		operations:              operations,
		processes:               processes,
		operationChunks:         operationChunks,
		histories:               histories,
		workspace:               workspace,
		fileEvents:              fileEvents,
		subtitles:               subtitles,
		presets:                 presets,
		settings:                settings,
		iconResolver:            iconResolver,
		tools:                   tools,
		proxyClient:             proxyClient,
		appSessions:             appSessions,
		bus:                     bus,
		runCancels:              make(map[string]context.CancelFunc),
		runDone:                 make(map[string]chan struct{}),
		downloadSchedulerActive: make(map[string]struct{}),
		resourceSniffs:          make(map[string]*resourceSniffSession),
		resourcePreviewLeases:   make(map[string]resourceSniffPreviewLease),
		resourceMedia:           make(map[string]resourceMedia),
		resourceMediaMeta:       make(map[string]resourceMediaSnapshotMetadata),
		nowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (service *LibraryService) now() time.Time {
	if service == nil || service.nowFunc == nil {
		return time.Now().UTC()
	}
	return service.nowFunc().UTC()
}

// HasActiveDataOperations is the cleanup barrier used by Settings data
// management. It intentionally reports both download/transcode work and an
// active sniff runtime so executable/profile directories are never moved out
// from underneath a writer.
func (service *LibraryService) HasActiveDataOperations() bool {
	if service == nil {
		return false
	}
	service.runMu.Lock()
	activeRuns := len(service.runCancels) > 0
	service.runMu.Unlock()
	service.downloadSchedulerMu.Lock()
	activeRuns = activeRuns || service.downloadSchedulerRunning || len(service.downloadSchedulerActive) > 0
	service.downloadSchedulerMu.Unlock()
	service.resourceSniffMu.Lock()
	activeSniffs := len(service.resourceSniffs) > 0
	service.resourceSniffMu.Unlock()
	return activeRuns || activeSniffs
}

func (service *LibraryService) ActiveResourceSniffProfileIDs() []string {
	if service == nil {
		return []string{}
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	seen := make(map[string]struct{})
	result := make([]string, 0, len(service.resourceSniffs))
	for _, session := range service.resourceSniffs {
		if session == nil {
			continue
		}
		profileID := strings.TrimSpace(session.ProfileID)
		if profileID == "" {
			continue
		}
		if _, duplicate := seen[profileID]; duplicate {
			continue
		}
		seen[profileID] = struct{}{}
		result = append(result, profileID)
	}
	sort.Strings(result)
	return result
}

func (service *LibraryService) SetCatalogProjectionRunner(runner CatalogProjectionRunner) {
	if service == nil {
		return
	}
	service.catalogProjection = runner
}

func (service *LibraryService) SetStorageRootRepository(
	repository library.StorageRootRepository,
) {
	if service == nil {
		return
	}
	service.storageRoots = repository
}

func (service *LibraryService) SetListenLocalCatalogMetadataSynchronizer(
	synchronizer listenLocalCatalogMetadataSynchronizer,
) {
	if service == nil {
		return
	}
	service.catalogMetadataSync = synchronizer
}

func (service *LibraryService) SetListenLocalMusicMembershipRepository(
	repository library.ListenLocalMusicMembershipRepository,
) {
	if service == nil {
		return
	}
	service.localMusicMemberships = repository
}

func (service *LibraryService) SetDefaultDownloadRootProvider(
	provider func(context.Context) (string, error),
) {
	if service == nil {
		return
	}
	service.defaultDownloadRoot = provider
}

// SetEmbeddedArtworkDirectory configures the private generated-artwork cache.
// Referenced source folders are never modified.
func (service *LibraryService) SetEmbeddedArtworkDirectory(path string) {
	if service == nil {
		return
	}
	service.embeddedArtworkDirectory = strings.TrimSpace(path)
}

func (service *LibraryService) syncCatalogProjection(ctx context.Context, libraryIDs ...string) error {
	if service == nil || service.catalogProjection == nil {
		return nil
	}
	uniqueLibraryIDs := make([]string, 0, len(libraryIDs))
	seenLibraryIDs := make(map[string]struct{}, len(libraryIDs))
	for _, libraryID := range libraryIDs {
		libraryID = strings.TrimSpace(libraryID)
		if libraryID == "" {
			continue
		}
		if _, duplicate := seenLibraryIDs[libraryID]; duplicate {
			continue
		}
		seenLibraryIDs[libraryID] = struct{}{}
		uniqueLibraryIDs = append(uniqueLibraryIDs, libraryID)
	}
	run := service.catalogProjection.Run
	if scoped, ok := service.catalogProjection.(scopedCatalogProjectionRunner); ok && len(uniqueLibraryIDs) > 0 {
		for _, libraryID := range uniqueLibraryIDs {
			if err := retryCatalogProjection(ctx, func(ctx context.Context) error {
				_, err := scoped.RunLibrary(ctx, libraryID)
				return err
			}); err != nil {
				return fmt.Errorf("synchronize library catalog projection for %q: %w", libraryID, err)
			}
		}
		return nil
	}
	if err := retryCatalogProjection(ctx, func(ctx context.Context) error {
		_, err := run(ctx)
		return err
	}); err != nil {
		return fmt.Errorf("synchronize library catalog projection: %w", err)
	}
	return nil
}

func retryCatalogProjection(ctx context.Context, run func(context.Context) error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := run(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if ctx.Err() != nil {
			break
		}
	}
	return lastErr
}

func libraryFileLibraryIDs(items []library.LibraryFile) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.LibraryID)
	}
	return result
}

func (service *LibraryService) saveAndPublishOperation(ctx context.Context, operation *library.LibraryOperation) error {
	if service == nil || service.operations == nil || operation == nil {
		return fmt.Errorf("operation repository not configured")
	}
	service.operationOutputMutationMu.Lock()
	defer service.operationOutputMutationMu.Unlock()
	return service.saveAndPublishOperationLocked(ctx, operation)
}

func (service *LibraryService) saveAndPublishOperationLocked(ctx context.Context, operation *library.LibraryOperation) error {
	service.reconcileBackgroundOperationDisplayNameLocked(ctx, operation, false)
	if err := service.operations.Save(ctx, *operation); err != nil {
		return err
	}
	if committed, err := service.operations.Get(ctx, operation.ID); err == nil {
		operation.DisplayName = committed.DisplayName
	}
	service.publishOperationUpdate(toOperationDTO(*operation))
	return nil
}

func (service *LibraryService) registerOperationRun(operationID string, cancel context.CancelFunc) bool {
	if service == nil || cancel == nil {
		return false
	}
	trimmed := strings.TrimSpace(operationID)
	if trimmed == "" {
		return false
	}
	service.runMu.Lock()
	defer service.runMu.Unlock()
	if service.shuttingDown {
		return false
	}
	if service.runCancels == nil {
		service.runCancels = make(map[string]context.CancelFunc)
	}
	if service.runDone == nil {
		service.runDone = make(map[string]chan struct{})
	}
	if _, exists := service.runCancels[trimmed]; exists {
		return false
	}
	service.runCancels[trimmed] = cancel
	service.runDone[trimmed] = make(chan struct{})
	return true
}

// BeginShutdown closes the task-admission gate before transports are drained.
// Existing operations remain cancellable, while any request which raced with
// shutdown can at most leave a durable queued operation for next-launch
// recovery; it can no longer start a child process after the shutdown snapshot.
func (service *LibraryService) BeginShutdown() {
	if service == nil {
		return
	}
	service.runMu.Lock()
	service.shuttingDown = true
	service.runMu.Unlock()
}

func (service *LibraryService) isShuttingDown() bool {
	if service == nil {
		return true
	}
	service.runMu.Lock()
	defer service.runMu.Unlock()
	return service.shuttingDown
}

func (service *LibraryService) unregisterOperationRun(operationID string) {
	if service == nil {
		return
	}
	trimmed := strings.TrimSpace(operationID)
	if trimmed == "" {
		return
	}
	var done chan struct{}
	service.runMu.Lock()
	if _, exists := service.runCancels[trimmed]; exists {
		delete(service.runCancels, trimmed)
		done = service.runDone[trimmed]
		delete(service.runDone, trimmed)
	}
	service.runMu.Unlock()
	if done != nil {
		close(done)
	}
}

func (service *LibraryService) cancelOperationRun(operationID string) bool {
	if service == nil {
		return false
	}
	trimmed := strings.TrimSpace(operationID)
	if trimmed == "" {
		return false
	}
	service.runMu.Lock()
	cancel := service.runCancels[trimmed]
	service.runMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (service *LibraryService) ShutdownActiveRuns(ctx context.Context) int {
	if service == nil {
		return 0
	}
	service.BeginShutdown()
	service.waitDownloadScheduler(ctx)
	service.runMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(service.runCancels))
	dones := make([]chan struct{}, 0, len(service.runDone))
	for operationID, cancel := range service.runCancels {
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
		if done := service.runDone[operationID]; done != nil {
			dones = append(dones, done)
		}
	}
	service.runMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	for _, done := range dones {
		select {
		case <-done:
		case <-ctx.Done():
			return len(cancels)
		}
	}
	return len(cancels)
}

func (service *LibraryService) getModuleConfig(ctx context.Context) (library.ModuleConfig, error) {
	if service == nil || service.moduleConfig == nil {
		return library.DefaultModuleConfig(), nil
	}
	config, err := service.moduleConfig.Get(ctx)
	if err != nil {
		return library.ModuleConfig{}, err
	}
	return config, nil
}

func (service *LibraryService) RecoverPendingJobs(ctx context.Context) {
	if service == nil || service.operations == nil {
		return
	}
	items, err := service.operations.List(ctx)
	if err != nil {
		return
	}
	operationsByID := make(map[string]library.LibraryOperation, len(items))
	for _, item := range items {
		operationsByID[item.ID] = item
	}
	service.cleanupStaleExternalProcesses(ctx, operationsByID)
	for _, item := range items {
		switch item.Status {
		case library.OperationStatusQueued:
		case library.OperationStatusRunning:
			service.markInterruptedOperation(ctx, item)
			continue
		default:
			continue
		}
		switch item.Kind {
		case "download":
			request := dto.CreateYTDLPJobRequest{}
			if err := json.Unmarshal([]byte(item.InputJSON), &request); err != nil {
				continue
			}
			if !isResourceDownloadRequest(request) {
				continue
			}
			history, err := service.findOrRebuildOperationHistory(ctx, item, request)
			if err != nil {
				continue
			}
			go service.runDownloadOperation(context.Background(), item, history, request)
		case "transcode":
			request := dto.CreateTranscodeJobRequest{}
			if err := json.Unmarshal([]byte(item.InputJSON), &request); err != nil {
				continue
			}
			go service.runTranscodeOperation(context.Background(), item, request)
		}
	}
	service.signalDownloadScheduler()
}

func (service *LibraryService) markInterruptedOperation(ctx context.Context, operation library.LibraryOperation) {
	if service == nil || service.operations == nil {
		return
	}
	now := service.now()
	operation.Status = library.OperationStatusFailed
	operation.ErrorCode = operationErrorCodeAppInterrupted
	operation.ErrorMessage = "operation interrupted while the application was closed"
	operation.FinishedAt = &now
	operation.Progress = buildOperationProgress(
		now,
		progressText("library.status.failed"),
		progressCurrent(operation.Progress),
		progressTotal(operation.Progress),
		progressText("library.progressDetail.operationInterrupted"),
	)
	service.operationOutputMutationMu.Lock()
	defer service.operationOutputMutationMu.Unlock()
	if err := service.saveAndPublishOperationLocked(ctx, &operation); err != nil {
		return
	}
	_ = service.syncPrimaryOperationHistoryLocked(ctx, operation, now)
}

func (service *LibraryService) findHistoryByOperationID(
	ctx context.Context,
	libraryID string,
	operationID string,
) (library.HistoryRecord, bool, error) {
	if service == nil || service.histories == nil {
		return library.HistoryRecord{}, false, nil
	}
	histories, err := service.histories.ListByLibraryID(ctx, strings.TrimSpace(libraryID))
	if err != nil {
		return library.HistoryRecord{}, false, err
	}
	trimmedOperationID := strings.TrimSpace(operationID)
	for _, history := range histories {
		if history.Category == operationHistoryCategory &&
			(history.Refs.OperationID == trimmedOperationID || history.Refs.SubjectOperationID == trimmedOperationID) {
			return history, true, nil
		}
	}
	return library.HistoryRecord{}, false, nil
}

func (service *LibraryService) findOrRebuildOperationHistory(ctx context.Context, operation library.LibraryOperation, request dto.CreateYTDLPJobRequest) (library.HistoryRecord, error) {
	if service == nil || service.histories == nil {
		return library.HistoryRecord{}, fmt.Errorf("history repository not configured")
	}
	histories, err := service.histories.ListByLibraryID(ctx, operation.LibraryID)
	if err == nil {
		for _, history := range histories {
			if history.Category == operationHistoryCategory &&
				(history.Refs.OperationID == operation.ID || history.Refs.SubjectOperationID == operation.ID) {
				return history, nil
			}
		}
	}
	now := service.now()
	history, buildErr := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          uuid.NewString(),
		LibraryID:   operation.LibraryID,
		Category:    "operation",
		Action:      operation.Kind,
		DisplayName: operation.DisplayName,
		Status:      string(operation.Status),
		Source: library.HistoryRecordSource{
			Kind:   resolveHistorySourceKind(request.Source),
			Caller: strings.TrimSpace(request.Caller),
			RunID:  strings.TrimSpace(request.RunID),
		},
		Refs: library.HistoryRecordRefs{
			OperationID:        operation.ID,
			SubjectOperationID: operation.ID,
		},
		Files:   operation.OutputFiles,
		Metrics: operation.Metrics,
		OperationMeta: &library.OperationRecordMeta{
			Kind:         operation.Kind,
			ErrorCode:    operation.ErrorCode,
			ErrorMessage: operation.ErrorMessage,
		},
		OccurredAt: &now,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	})
	if buildErr != nil {
		return library.HistoryRecord{}, buildErr
	}
	if saveErr := service.histories.Save(ctx, history); saveErr != nil {
		return library.HistoryRecord{}, saveErr
	}
	return history, nil
}

func (service *LibraryService) ListLibraries(ctx context.Context) ([]dto.LibraryDTO, error) {
	items, err := service.libraries.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.LibraryDTO, 0, len(items))
	for _, item := range items {
		libraryDTO, err := service.buildLibraryDTO(ctx, item)
		if err != nil {
			return nil, err
		}
		result = append(result, libraryDTO)
	}
	return result, nil
}

func (service *LibraryService) GetLibrary(ctx context.Context, request dto.GetLibraryRequest) (dto.LibraryDTO, error) {
	item, err := service.libraries.Get(ctx, strings.TrimSpace(request.LibraryID))
	if err != nil {
		return dto.LibraryDTO{}, err
	}
	return service.buildLibraryDTO(ctx, item)
}

func (service *LibraryService) RenameLibrary(ctx context.Context, request dto.RenameLibraryRequest) (dto.LibraryDTO, error) {
	libraryID := strings.TrimSpace(request.LibraryID)
	name := strings.TrimSpace(request.Name)
	if libraryID == "" || name == "" {
		return dto.LibraryDTO{}, library.ErrInvalidLibrary
	}
	item, err := service.libraries.Get(ctx, libraryID)
	if err != nil {
		return dto.LibraryDTO{}, err
	}
	item.Name = name
	item.UpdatedAt = service.now()
	if err := service.libraries.Save(ctx, item); err != nil {
		return dto.LibraryDTO{}, err
	}
	return service.buildLibraryDTO(ctx, item)
}

func (service *LibraryService) DeleteLibrary(ctx context.Context, request dto.DeleteLibraryRequest) error {
	libraryID := strings.TrimSpace(request.LibraryID)
	if libraryID == "" {
		return fmt.Errorf("libraryId is required")
	}
	if _, err := service.libraries.Get(ctx, libraryID); err != nil {
		return err
	}
	files, err := service.files.ListByLibraryID(ctx, libraryID)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := deleteLocalFileIfExists(file.Storage.LocalPath); err != nil {
			return err
		}
	}
	return service.libraries.Delete(ctx, libraryID)
}

func (service *LibraryService) GetModuleConfig(ctx context.Context) (dto.LibraryModuleConfigDTO, error) {
	config, err := service.getModuleConfig(ctx)
	if err != nil {
		return dto.LibraryModuleConfigDTO{}, err
	}
	return toModuleConfigDTO(config), nil
}

func (service *LibraryService) GetDefaultModuleConfig(ctx context.Context) (dto.LibraryModuleConfigDTO, error) {
	return toModuleConfigDTO(library.DefaultModuleConfig()), nil
}

func (service *LibraryService) UpdateModuleConfig(ctx context.Context, request dto.UpdateLibraryModuleConfigRequest) (dto.LibraryModuleConfigDTO, error) {
	config := toDomainModuleConfig(request.Config)
	if service == nil || service.moduleConfig == nil {
		return dto.LibraryModuleConfigDTO{}, fmt.Errorf("module config repository not configured")
	}
	if err := service.moduleConfig.Save(ctx, config); err != nil {
		return dto.LibraryModuleConfigDTO{}, err
	}
	return toModuleConfigDTO(config), nil
}

func (service *LibraryService) ListOperations(ctx context.Context, request dto.ListOperationsRequest) ([]dto.OperationListItemDTO, error) {
	var items []library.LibraryOperation
	var err error
	libraryID := strings.TrimSpace(request.LibraryID)
	if libraryID != "" {
		items, err = service.operations.ListByLibraryID(ctx, libraryID)
	} else {
		items, err = service.operations.List(ctx)
	}
	if err != nil {
		return nil, err
	}
	libraryItems, err := service.libraries.List(ctx)
	if err != nil {
		return nil, err
	}
	libraryNames := make(map[string]string, len(libraryItems))
	for _, item := range libraryItems {
		libraryNames[item.ID] = item.Name
	}
	statuses := toLookup(request.Status)
	kinds := toLookup(request.Kinds)
	query := strings.ToLower(strings.TrimSpace(request.Query))
	result := make([]dto.OperationListItemDTO, 0, len(items))
	for _, item := range items {
		if len(statuses) > 0 {
			if _, ok := statuses[strings.ToLower(string(item.Status))]; !ok {
				continue
			}
		}
		if len(kinds) > 0 {
			if _, ok := kinds[strings.ToLower(item.Kind)]; !ok {
				continue
			}
		}
		if query != "" {
			requestPreview := toOperationRequestPreviewDTO(item)
			requestURL := ""
			if requestPreview != nil {
				requestURL = requestPreview.URL
			}
			candidate := strings.ToLower(strings.Join([]string{
				item.DisplayName,
				item.Kind,
				item.SourceDomain,
				item.Meta.Platform,
				item.Meta.Uploader,
				requestURL,
				libraryNames[item.LibraryID],
			}, " "))
			if !strings.Contains(candidate, query) {
				continue
			}
		}
		result = append(result, toOperationListItemDTO(item, libraryNames[item.LibraryID]))
	}
	return paginateOperationList(result, request.Offset, request.Limit), nil
}

func (service *LibraryService) GetOperation(ctx context.Context, request dto.GetOperationRequest) (dto.LibraryOperationDTO, error) {
	item, err := service.operations.Get(ctx, strings.TrimSpace(request.OperationID))
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	return toOperationDTO(item), nil
}

func (service *LibraryService) RenameOperation(ctx context.Context, request dto.RenameOperationRequest) (dto.LibraryOperationDTO, error) {
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		return dto.LibraryOperationDTO{}, library.ErrInvalidLibraryOperation
	}
	name, err := normalizeLibraryDisplayName(request.Name)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	service.operationOutputMutationMu.Lock()
	unlock := service.operationOutputMutationMu.Unlock
	lockReleased := false
	defer func() {
		if !lockReleased {
			unlock()
		}
	}()
	item, err := service.operations.Get(ctx, operationID)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if item.DisplayName == name {
		return toOperationDTO(item), nil
	}

	before := item
	item.DisplayName = name
	now := service.now()
	var historyBefore *library.HistoryRecord
	var historyAfter *library.HistoryRecord
	history, historyFound, historyErr := service.findHistoryByOperationID(ctx, item.LibraryID, item.ID)
	if historyErr != nil {
		return dto.LibraryOperationDTO{}, historyErr
	}
	if historyFound {
		beforeSnapshot := history
		history.DisplayName = name
		history.UpdatedAt = now
		afterSnapshot := history
		historyBefore = &beforeSnapshot
		historyAfter = &afterSnapshot
	}
	var renameEvent library.HistoryRecord
	if service.histories != nil {
		var eventErr error
		renameEvent, eventErr = service.newOperationLifecycleEvent(before, operationEventRenamed, now)
		if eventErr != nil {
			return dto.LibraryOperationDTO{}, eventErr
		}
	}
	if err := service.saveOperationRenameWithHistoryEvent(
		ctx,
		before,
		item,
		historyBefore,
		historyAfter,
		renameEvent,
	); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	_, usedAtomicRename := service.operations.(operationHistoryEventAtomicRepository)
	if historyAfter != nil {
		committedHistory := *historyAfter
		if usedAtomicRename {
			if current, getErr := service.histories.Get(ctx, historyAfter.ID); getErr == nil {
				committedHistory = current
			}
		}
		service.publishHistoryUpdate(toHistoryDTO(committedHistory))
	}
	if renameEvent.ID != "" {
		service.publishHistoryUpdate(toHistoryDTO(renameEvent))
	}
	committedOperation := item
	if usedAtomicRename {
		if current, getErr := service.operations.Get(ctx, operationID); getErr == nil {
			committedOperation = current
		}
	}
	operationDTO := toOperationDTO(committedOperation)
	service.publishOperationUpdate(operationDTO)
	// The rename and its activity record are already committed. Library
	// freshness is a derived projection and must not turn that success into an
	// RPC error which invites a misleading duplicate retry.
	_ = service.touchLibrary(ctx, item.LibraryID, now)
	unlock()
	lockReleased = true
	return operationDTO, nil
}

func (service *LibraryService) RenameFile(ctx context.Context, request dto.RenameFileRequest) (dto.LibraryFileDTO, error) {
	fileID := strings.TrimSpace(request.FileID)
	if fileID == "" {
		return dto.LibraryFileDTO{}, library.ErrInvalidLibraryFile
	}
	name, err := normalizeLibraryDisplayName(request.Name)
	if err != nil {
		return dto.LibraryFileDTO{}, err
	}
	unlock := service.lockListenLocalTrackMutation(fileID)
	lockReleased := false
	defer func() {
		if !lockReleased {
			unlock()
		}
	}()
	item, err := service.files.Get(ctx, fileID)
	if err != nil {
		return dto.LibraryFileDTO{}, err
	}
	if item.DisplayName == name {
		fileDTO := service.bestEffortFileDTO(ctx, item)
		unlock()
		lockReleased = true
		return fileDTO, nil
	}

	now := service.now()
	before := item
	item.DisplayName = name
	item.UpdatedAt = now
	if _, err := service.saveLibraryFileWithEvent(ctx, before, item, appendLibraryFileEventParams{
		EventType:   libraryFileEventRenamed,
		Category:    "file_metadata",
		OperationID: fileEventOperationID(item),
		FileID:      item.ID,
		LibraryID:   item.LibraryID,
		Before:      fileEventSnapshot(before),
		After:       fileEventSnapshot(item),
		Changes:     []dto.FileFieldChangeDTO{{Field: "displayName", Before: before.DisplayName, After: item.DisplayName}},
		OccurredAt:  now,
	}); err != nil {
		return dto.LibraryFileDTO{}, err
	}
	committedItem := item
	if _, usedAtomicRename := service.files.(libraryFileRenameAtomicRepository); usedAtomicRename {
		if current, getErr := service.files.Get(ctx, fileID); getErr == nil {
			committedItem = current
		}
	}
	fileDTO := service.bestEffortFileDTO(ctx, committedItem)
	service.publishFileUpdate(fileDTO)
	// Library freshness is quick and ordered with same-file renames, but remains
	// best effort after the durable file/event transaction.
	_ = service.touchLibrary(ctx, committedItem.LibraryID, now)
	unlock()
	lockReleased = true

	// Catalog projection is rebuildable from the committed LibraryFile. Keep it
	// synchronous as a best effort, but never report the durable display-name
	// mutation as failed because the projection is temporarily down.
	_ = service.syncCatalogProjection(ctx, committedItem.LibraryID)
	return fileDTO, nil
}

func (service *LibraryService) bestEffortFileDTO(ctx context.Context, item library.LibraryFile) dto.LibraryFileDTO {
	fileDTO, err := service.buildFileDTO(ctx, item)
	if err != nil {
		return toLibraryFileDTO(item)
	}
	return fileDTO
}

func (service *LibraryService) CancelOperation(ctx context.Context, request dto.CancelOperationRequest) (dto.LibraryOperationDTO, error) {
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operationId is required")
	}
	service.operationOutputMutationMu.Lock()
	defer service.operationOutputMutationMu.Unlock()
	item, err := service.operations.Get(ctx, operationID)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if item.Kind != "download" && item.Kind != "transcode" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operation kind %q does not support cancel", item.Kind)
	}
	if item.Status != library.OperationStatusQueued && item.Status != library.OperationStatusRunning {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operation status %q does not support cancel", item.Status)
	}
	before := item
	item, err = service.markOperationCanceledLocked(ctx, item)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	service.cancelOperationRun(item.ID)
	occurredAt := service.now()
	if item.FinishedAt != nil {
		occurredAt = *item.FinishedAt
	}
	if err := service.syncPrimaryOperationHistoryLocked(ctx, item, occurredAt); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if _, err := service.appendOperationLifecycleEvent(ctx, before, operationEventCanceled, occurredAt); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	return toOperationDTO(item), nil
}

func (service *LibraryService) markOperationCanceled(ctx context.Context, item library.LibraryOperation) (library.LibraryOperation, error) {
	if service == nil {
		return library.LibraryOperation{}, library.ErrOperationNotFound
	}
	service.operationOutputMutationMu.Lock()
	defer service.operationOutputMutationMu.Unlock()
	return service.markOperationCanceledLocked(ctx, item)
}

func (service *LibraryService) markOperationCanceledLocked(ctx context.Context, item library.LibraryOperation) (library.LibraryOperation, error) {
	now := service.now()
	if service != nil && service.operations != nil {
		if latest, err := service.operations.Get(ctx, item.ID); err == nil {
			if outputJSON, changed := mergeOperationOutputArtifactPaths(item.OutputJSON, latest.OutputJSON); changed {
				item.OutputJSON = outputJSON
			}
			if outputJSON, changed := mergeOperationTemporaryArtifactPaths(item.OutputJSON, latest.OutputJSON); changed {
				item.OutputJSON = outputJSON
			}
		}
	}
	if updated, changed := service.withOperationTemporaryArtifactPaths(ctx, item); changed {
		item = updated
	}
	item.Status = library.OperationStatusCanceled
	item.ErrorCode = "canceled"
	item.ErrorMessage = "operation canceled"
	item.FinishedAt = &now
	item.Progress = buildOperationProgress(
		now,
		progressText("library.status.canceled"),
		progressCurrent(item.Progress),
		progressTotal(item.Progress),
		terminalProgressMessage(item.Kind, library.OperationStatusCanceled),
	)
	if err := service.saveAndPublishOperationLocked(ctx, &item); err != nil {
		return library.LibraryOperation{}, err
	}
	return item, nil
}

func (service *LibraryService) ResumeOperation(ctx context.Context, request dto.ResumeOperationRequest) (dto.LibraryOperationDTO, error) {
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operationId is required")
	}
	service.operationOutputMutationMu.Lock()
	defer service.operationOutputMutationMu.Unlock()
	item, err := service.operations.Get(ctx, operationID)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	switch item.Kind {
	case "download":
		return service.resumeDownloadOperation(ctx, item)
	case "transcode":
		return service.resumeTranscodeOperation(ctx, item)
	default:
		return dto.LibraryOperationDTO{}, fmt.Errorf("operation kind %q does not support resume", item.Kind)
	}
}

func (service *LibraryService) resumeDownloadOperation(ctx context.Context, item library.LibraryOperation) (dto.LibraryOperationDTO, error) {
	if item.Status != library.OperationStatusFailed && item.Status != library.OperationStatusCanceled {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operation status %q does not support resume", item.Status)
	}
	before := item
	resumeRequest := dto.CreateYTDLPJobRequest{}
	if err := json.Unmarshal([]byte(item.InputJSON), &resumeRequest); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if strings.TrimSpace(resumeRequest.URL) == "" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("download url is required")
	}
	if strings.TrimSpace(resumeRequest.ResourceSessionID) != "" || strings.TrimSpace(resumeRequest.ResourceMediaID) != "" {
		return dto.LibraryOperationDTO{}, resourceRetryUnavailableError()
	}
	resumeRequest = withYTDLPOperationLibrary(resumeRequest, item)
	inputJSON, err := json.Marshal(resumeRequest)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	history, err := service.findOrRebuildOperationHistory(ctx, item, resumeRequest)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	now := service.now()
	item.Status = library.OperationStatusQueued
	item.ErrorCode = ""
	item.ErrorMessage = ""
	item.StartedAt = nil
	item.FinishedAt = nil
	item.InputJSON = string(inputJSON)
	item.Progress = buildOperationProgress(
		now,
		progressText("library.status.queued"),
		progressCurrent(item.Progress),
		progressTotal(item.Progress),
		progressText("library.progressDetail.resumeRequested"),
	)
	item.OutputFiles = nil
	item.Metrics = library.OperationMetrics{}
	item.OutputJSON = "{}"
	history.Status = string(item.Status)
	history.DisplayName = item.DisplayName
	history.Files = nil
	history.Metrics = library.OperationMetrics{}
	history.OperationMeta = &library.OperationRecordMeta{Kind: item.Kind}
	history.UpdatedAt = now
	if err := service.persistOperationAndHistoryLocked(ctx, &item, &history); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if _, err := service.appendOperationLifecycleEvent(ctx, before, operationEventResumed, now); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	operationDTO := toOperationDTO(item)
	if isResourceDownloadRequest(resumeRequest) {
		go service.runDownloadOperation(context.Background(), item, history, resumeRequest)
	} else {
		service.signalDownloadScheduler()
	}
	return operationDTO, nil
}

func (service *LibraryService) resumeTranscodeOperation(ctx context.Context, item library.LibraryOperation) (dto.LibraryOperationDTO, error) {
	if item.Status != library.OperationStatusFailed && item.Status != library.OperationStatusCanceled {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operation status %q does not support resume", item.Status)
	}
	before := item
	resumeRequest := dto.CreateTranscodeJobRequest{}
	if err := json.Unmarshal([]byte(item.InputJSON), &resumeRequest); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	now := service.now()
	item.Status = library.OperationStatusQueued
	item.ErrorCode = ""
	item.ErrorMessage = ""
	item.StartedAt = nil
	item.FinishedAt = nil
	item.Progress = buildOperationProgress(
		now,
		progressText("library.status.queued"),
		0,
		progressTotal(item.Progress),
		progressText("library.progressDetail.resumeRequested"),
	)
	item.OutputFiles = nil
	item.Metrics = library.OperationMetrics{}
	item.OutputJSON = buildTranscodeOperationOutput(resumeRequest, "queued", "")
	if err := service.saveAndPublishOperationLocked(ctx, &item); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if err := service.syncPrimaryOperationHistoryLocked(ctx, item, now); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	operationDTO := toOperationDTO(item)
	if _, err := service.appendOperationLifecycleEvent(ctx, before, operationEventResumed, now); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	go service.runTranscodeOperation(context.Background(), item, resumeRequest)
	return operationDTO, nil
}

func (service *LibraryService) DeleteOperation(ctx context.Context, request dto.DeleteOperationRequest) error {
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		return fmt.Errorf("operationId is required")
	}
	return service.deleteOperation(ctx, operationID, request.CascadeFiles)
}

func (service *LibraryService) DeleteOperations(ctx context.Context, request dto.DeleteOperationsRequest) error {
	operationIDs := normalizeOperationIDs(request.OperationIDs)
	if len(operationIDs) == 0 {
		return fmt.Errorf("operationIds is required")
	}
	for _, operationID := range operationIDs {
		if err := service.deleteOperation(ctx, operationID, request.CascadeFiles); err != nil {
			return err
		}
	}
	return nil
}

func (service *LibraryService) deleteOperation(ctx context.Context, operationID string, cascadeFiles bool) error {
	service.operationOutputMutationMu.Lock()
	defer service.operationOutputMutationMu.Unlock()

	item, err := service.operations.Get(ctx, operationID)
	if err != nil {
		return err
	}
	caller := "keep_files"
	if cascadeFiles {
		caller = "cascade_files"
	}
	if _, err := service.appendOperationDeleteLifecycleEvent(
		ctx,
		item,
		operationEventDeleteRequested,
		caller,
		service.now(),
		true,
	); err != nil {
		return err
	}
	if cascadeFiles {
		seenFileIDs := make(map[string]struct{}, len(item.OutputFiles))
		for _, output := range item.OutputFiles {
			fileID := strings.TrimSpace(output.FileID)
			if fileID == "" {
				continue
			}
			if _, exists := seenFileIDs[fileID]; exists {
				continue
			}
			seenFileIDs[fileID] = struct{}{}
			fileItem, getErr := service.files.Get(ctx, fileID)
			if getErr != nil {
				if getErr == library.ErrFileNotFound {
					continue
				}
				return getErr
			}
			if _, _, err := service.markLibraryFileDeletedWithOptions(ctx, libraryFileDeleteOptions{
				fileID:           fileItem.ID,
				deleteLocal:      true,
				eventCategory:    "task_delete",
				eventOperationID: item.ID,
			}); err != nil {
				return err
			}
		}
		if err := service.deleteUntrackedOperationOutputArtifacts(ctx, item); err != nil {
			return err
		}
	}
	if service.operationChunks != nil {
		if err := service.operationChunks.DeleteByOperationID(ctx, operationID); err != nil {
			return err
		}
	}
	if err := service.operations.Delete(ctx, operationID); err != nil {
		return err
	}
	if err := service.touchLibrary(ctx, item.LibraryID, service.now()); err != nil {
		return err
	}
	if _, err := service.appendOperationDeleteLifecycleEvent(
		ctx,
		item,
		operationEventDeleted,
		caller,
		service.now(),
		false,
	); err != nil {
		return err
	}
	service.publishOperationDelete(operationID)
	return nil
}

func normalizeOperationIDs(operationIDs []string) []string {
	if len(operationIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(operationIDs))
	result := make([]string, 0, len(operationIDs))
	for _, operationID := range operationIDs {
		trimmed := strings.TrimSpace(operationID)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func (service *LibraryService) DeleteFile(ctx context.Context, request dto.DeleteFileRequest) error {
	fileID := strings.TrimSpace(request.FileID)
	if fileID == "" {
		return fmt.Errorf("fileId is required")
	}
	return service.deleteFile(ctx, fileID, request.DeleteFiles)
}

func (service *LibraryService) DeleteFiles(ctx context.Context, request dto.DeleteFilesRequest) error {
	fileIDs := normalizeFileIDs(request.FileIDs)
	if len(fileIDs) == 0 {
		return fmt.Errorf("fileIds is required")
	}
	for _, fileID := range fileIDs {
		if err := service.deleteFile(ctx, fileID, request.DeleteFiles); err != nil {
			return err
		}
	}
	return nil
}

func (service *LibraryService) deleteFile(ctx context.Context, fileID string, deleteFiles bool) error {
	return service.markLibraryFileDeleted(ctx, fileID, deleteFiles)
}

func normalizeFileIDs(fileIDs []string) []string {
	if len(fileIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(fileIDs))
	result := make([]string, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		trimmed := strings.TrimSpace(fileID)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func (service *LibraryService) ListLibraryHistory(ctx context.Context, request dto.ListLibraryHistoryRequest) ([]dto.LibraryHistoryRecordDTO, error) {
	items, err := service.histories.ListByLibraryID(ctx, strings.TrimSpace(request.LibraryID))
	if err != nil {
		return nil, err
	}
	categories := toLookup(request.Categories)
	actions := toLookup(request.Actions)
	result := make([]dto.LibraryHistoryRecordDTO, 0, len(items))
	for _, item := range items {
		if len(categories) > 0 {
			if _, ok := categories[strings.ToLower(item.Category)]; !ok {
				continue
			}
		}
		if len(actions) > 0 {
			if _, ok := actions[strings.ToLower(item.Action)]; !ok {
				continue
			}
		}
		result = append(result, toHistoryDTO(item))
	}
	return paginateHistory(result, request.Offset, request.Limit), nil
}

func (service *LibraryService) ListFileEvents(ctx context.Context, request dto.ListFileEventsRequest) ([]dto.FileEventRecordDTO, error) {
	items, err := service.fileEvents.ListByLibraryID(ctx, strings.TrimSpace(request.LibraryID))
	if err != nil {
		return nil, err
	}
	result := make([]dto.FileEventRecordDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toFileEventDTO(item))
	}
	return paginateFileEvents(result, request.Offset, request.Limit), nil
}

func (service *LibraryService) SaveWorkspaceState(ctx context.Context, request dto.SaveWorkspaceStateRequest) (dto.WorkspaceStateRecordDTO, error) {
	libraryID := strings.TrimSpace(request.LibraryID)
	if libraryID == "" {
		return dto.WorkspaceStateRecordDTO{}, fmt.Errorf("libraryId is required")
	}
	stateJSON := strings.TrimSpace(request.StateJSON)
	if stateJSON == "" {
		return dto.WorkspaceStateRecordDTO{}, fmt.Errorf("stateJson is required")
	}
	version := 1
	if head, err := service.workspace.GetHeadByLibraryID(ctx, libraryID); err == nil {
		version = head.StateVersion + 1
	} else if err != nil && err != library.ErrWorkspaceStateNotFound {
		return dto.WorkspaceStateRecordDTO{}, err
	}
	now := service.now()
	item, err := library.NewWorkspaceStateRecord(library.WorkspaceStateRecordParams{
		ID:           uuid.NewString(),
		LibraryID:    libraryID,
		StateVersion: version,
		StateJSON:    stateJSON,
		OperationID:  strings.TrimSpace(request.OperationID),
		CreatedAt:    &now,
	})
	if err != nil {
		return dto.WorkspaceStateRecordDTO{}, err
	}
	if err := service.workspace.Save(ctx, item); err != nil {
		return dto.WorkspaceStateRecordDTO{}, err
	}
	if err := service.touchLibrary(ctx, libraryID, now); err != nil {
		return dto.WorkspaceStateRecordDTO{}, err
	}
	result := toWorkspaceDTO(item)
	service.publishWorkspaceUpdate(result)
	return result, nil
}

func (service *LibraryService) GetWorkspaceState(ctx context.Context, request dto.GetWorkspaceStateRequest) (dto.WorkspaceStateRecordDTO, error) {
	item, err := service.workspace.GetHeadByLibraryID(ctx, strings.TrimSpace(request.LibraryID))
	if err != nil {
		if err == library.ErrWorkspaceStateNotFound {
			return dto.WorkspaceStateRecordDTO{
				LibraryID: strings.TrimSpace(request.LibraryID),
			}, nil
		}
		return dto.WorkspaceStateRecordDTO{}, err
	}
	return toWorkspaceDTO(item), nil
}

func (service *LibraryService) OpenFileLocation(ctx context.Context, request dto.OpenFileLocationRequest) error {
	fileID := strings.TrimSpace(request.FileID)
	if fileID == "" {
		return fmt.Errorf("fileId is required")
	}
	item, err := service.files.Get(ctx, fileID)
	if err != nil {
		return err
	}
	if !catalogFileCanAttemptRead(ctx, item, service.storageRoots) {
		return fmt.Errorf("file is not currently available")
	}
	path := strings.TrimSpace(item.Storage.LocalPath)
	if path == "" {
		return fmt.Errorf("file has no local path")
	}
	cleaned := filepath.Clean(path)
	if info, statErr := os.Stat(cleaned); statErr == nil {
		if info.IsDir() {
			return opener.OpenDirectory(cleaned)
		}
		return opener.RevealPath(cleaned)
	}
	parent := filepath.Dir(cleaned)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("file location is not currently available")
	}
	return opener.OpenDirectory(parent)
}

func (service *LibraryService) OpenPath(_ context.Context, request dto.OpenPathRequest) error {
	path := strings.TrimSpace(request.Path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	cleaned := filepath.Clean(path)
	if strings.EqualFold(filepath.Base(cleaned), "xiadown") {
		if err := os.MkdirAll(cleaned, 0o755); err != nil {
			return err
		}
		return opener.OpenDirectory(cleaned)
	}
	if info, err := os.Stat(cleaned); err == nil {
		if info.IsDir() {
			return opener.OpenDirectory(cleaned)
		}
		return opener.RevealPath(cleaned)
	}
	return opener.OpenDirectory(filepath.Dir(cleaned))
}

func (service *LibraryService) CreateYTDLPJob(ctx context.Context, request dto.CreateYTDLPJobRequest) (dto.LibraryOperationDTO, error) {
	if service.isShuttingDown() {
		return dto.LibraryOperationDTO{}, fmt.Errorf("Library service is shutting down")
	}
	resolvedURL, _, err := validateDownloadURL(request.URL)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	request.URL = resolvedURL
	request, claimedResourceMediaID, err := service.claimResourceMediaForQueuedOperation(request)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	operation, history, _, err := service.createDownloadOperation(ctx, request)
	if err != nil {
		service.discardResourceMediaSnapshots(claimedResourceMediaID)
		return dto.LibraryOperationDTO{}, err
	}
	if isResourceDownloadRequest(request) {
		go service.runDownloadOperation(context.Background(), operation, history, withYTDLPOperationLibrary(request, operation))
	} else {
		service.signalDownloadScheduler()
	}
	return toOperationDTO(operation), nil
}

func (service *LibraryService) CreateYTDLPBatchJobs(ctx context.Context, request dto.CreateYTDLPBatchJobsRequest) (dto.CreateYTDLPBatchJobsResponse, error) {
	if service.isShuttingDown() {
		return dto.CreateYTDLPBatchJobsResponse{}, fmt.Errorf("Library service is shutting down")
	}
	if len(request.Items) == 0 {
		return dto.CreateYTDLPBatchJobsResponse{}, apperrors.New(apperrors.CodeDownloadBatchEmpty, "download batch is empty")
	}
	if len(request.Items) > 100 {
		return dto.CreateYTDLPBatchJobsResponse{}, apperrors.New(apperrors.CodeDownloadBatchTooLarge, "download batch is too large")
	}
	batchRunID := uuid.NewString()
	normalizedItems, err := normalizeYTDLPBatchItems(request.Items, batchRunID)
	if err != nil {
		return dto.CreateYTDLPBatchJobsResponse{}, err
	}
	operations := make([]dto.LibraryOperationDTO, 0, len(normalizedItems))
	for _, item := range normalizedItems {
		operation, _, _, err := service.createDownloadOperation(ctx, item)
		if err != nil {
			return dto.CreateYTDLPBatchJobsResponse{}, err
		}
		operations = append(operations, toOperationDTO(operation))
	}
	service.signalDownloadScheduler()
	return dto.CreateYTDLPBatchJobsResponse{Operations: operations}, nil
}

func normalizeYTDLPBatchItems(items []dto.CreateYTDLPJobRequest, batchRunID string) ([]dto.CreateYTDLPJobRequest, error) {
	normalizedItems := make([]dto.CreateYTDLPJobRequest, 0, len(items))
	for _, item := range items {
		if isResourceDownloadRequest(item) {
			return nil, apperrors.New(apperrors.CodeDownloadURLUnsupported, "resource downloads cannot be created as a batch")
		}
		resolvedItems, err := parseDownloadURLs(item.URL)
		if err != nil {
			return nil, err
		}
		for _, resolvedItem := range resolvedItems {
			next := item
			next.URL = resolvedItem.URL
			if strings.TrimSpace(next.RunID) == "" {
				next.RunID = batchRunID
			}
			normalizedItems = append(normalizedItems, next)
			if len(normalizedItems) > 100 {
				return nil, apperrors.New(apperrors.CodeDownloadBatchTooLarge, "download batch is too large")
			}
		}
	}
	if len(normalizedItems) == 0 {
		return nil, apperrors.New(apperrors.CodeDownloadBatchEmpty, "download batch is empty")
	}
	return normalizedItems, nil
}

func (service *LibraryService) CreateVideoImport(ctx context.Context, request dto.CreateVideoImportRequest) (dto.LibraryFileDTO, error) {
	resolvedPath, err := service.resolveInputPath(ctx, request.Path, request.Source, false)
	if err != nil {
		return dto.LibraryFileDTO{}, err
	}
	name := strings.TrimSpace(request.Title)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(resolvedPath), filepath.Ext(resolvedPath))
	}
	libraryItem, err := service.ensureLibrary(ctx, ensureLibraryParams{
		LibraryID:       request.LibraryID,
		FallbackName:    deriveLibraryName(name, resolvedPath),
		CreatedBySource: "import_video",
	})
	if err != nil {
		return dto.LibraryFileDTO{}, err
	}
	fileItem, history, eventRecord, err := service.createImportFile(ctx, importFileParams{
		LibraryID:      libraryItem.ID,
		Path:           resolvedPath,
		Name:           name,
		Kind:           string(library.FileKindVideo),
		Source:         request.Source,
		SessionRunID:   request.RunID,
		KeepSourceFile: true,
		Action:         "import_video",
	})
	if err != nil {
		return dto.LibraryFileDTO{}, err
	}
	service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
	service.publishHistoryUpdate(toHistoryDTO(history))
	service.publishFileEventUpdate(toFileEventDTO(eventRecord))
	return service.mustBuildFileDTO(ctx, fileItem), nil
}

func (service *LibraryService) CreateTranscodeJob(ctx context.Context, request dto.CreateTranscodeJobRequest) (dto.LibraryOperationDTO, error) {
	if service.isShuttingDown() {
		return dto.LibraryOperationDTO{}, fmt.Errorf("Library service is shutting down")
	}
	sourceFile, err := service.resolveSourceFileForTranscode(ctx, request)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if sourceFile.LibraryID == "" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("source file is not attached to a library")
	}
	request = service.enrichTranscodeRequestForSource(ctx, request, sourceFile)
	probe, err := service.probeRequiredMedia(ctx, sourceFile.Storage.LocalPath)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	plan, err := service.resolveTranscodePlan(ctx, request, probe)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	now := service.now()
	operationID := uuid.NewString()
	displayName := resolveTranscodeDisplayName(request, sourceFile, plan.preset)
	inputJSON, err := json.Marshal(request)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          operationID,
		LibraryID:   sourceFile.LibraryID,
		Kind:        "transcode",
		Status:      string(library.OperationStatusQueued),
		DisplayName: displayName,
		Correlation: library.OperationCorrelation{
			RunID:             strings.TrimSpace(request.RunID),
			ParentOperationID: resolveTranscodeParentOperationID(sourceFile),
		},
		InputJSON:  string(inputJSON),
		OutputJSON: buildTranscodeOperationOutput(request, "queued", ""),
		Progress: buildOperationProgress(
			now,
			progressText("library.status.queued"),
			0,
			1,
			progressText("library.progressDetail.ffmpegTranscodeQueued"),
		),
		CreatedAt: &now,
	})
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if err := service.operations.Save(ctx, operation); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	history, err := service.createTranscodePrimaryHistory(ctx, operation, request, now)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	operationDTO := toOperationDTO(operation)
	service.publishOperationUpdate(operationDTO)
	service.publishHistoryUpdate(toHistoryDTO(history))
	go service.runTranscodeOperation(context.Background(), operation, request)
	return operationDTO, nil
}

type ensureLibraryParams struct {
	LibraryID          string
	FallbackName       string
	InitialNameFromID  bool
	CreatedBySource    string
	TriggerOperationID string
}

func (service *LibraryService) ensureLibrary(ctx context.Context, params ensureLibraryParams) (library.Library, error) {
	trimmedID := strings.TrimSpace(params.LibraryID)
	if trimmedID != "" {
		return service.libraries.Get(ctx, trimmedID)
	}
	now := service.now()
	libraryID := uuid.NewString()
	item, err := library.NewLibrary(library.LibraryParams{
		ID:   libraryID,
		Name: resolveInitialLibraryName(libraryID, params.FallbackName, params.InitialNameFromID),
		CreatedBy: library.CreateMeta{
			Source:             strings.TrimSpace(params.CreatedBySource),
			TriggerOperationID: strings.TrimSpace(params.TriggerOperationID),
		},
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		return library.Library{}, err
	}
	if err := service.libraries.Save(ctx, item); err != nil {
		return library.Library{}, err
	}
	return item, nil
}

func (service *LibraryService) touchLibrary(ctx context.Context, libraryID string, updatedAt time.Time) error {
	if service == nil || service.libraries == nil || strings.TrimSpace(libraryID) == "" {
		return nil
	}
	item, err := service.libraries.Get(ctx, libraryID)
	if err != nil {
		return err
	}
	item.UpdatedAt = updatedAt
	return service.libraries.Save(ctx, item)
}

func (service *LibraryService) renameLibraryFromFirstFileIfNeeded(ctx context.Context, libraryID string, fileName string, updatedAt time.Time) error {
	trimmedLibraryID := strings.TrimSpace(libraryID)
	if trimmedLibraryID == "" {
		return nil
	}
	nextName := resolveLibraryNameFromFile(fileName)
	if nextName == "" {
		return nil
	}
	item, err := service.libraries.Get(ctx, trimmedLibraryID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(item.Name) != strings.TrimSpace(item.ID) {
		return nil
	}
	files, err := service.files.ListByLibraryID(ctx, trimmedLibraryID)
	if err != nil {
		return err
	}
	if len(files) != 1 {
		return nil
	}
	item.Name = nextName
	item.UpdatedAt = updatedAt
	return service.libraries.Save(ctx, item)
}

type importFileParams struct {
	LibraryID              string
	Path                   string
	OriginPath             string
	Name                   string
	Kind                   string
	Source                 string
	SessionRunID           string
	KeepSourceFile         bool
	Action                 string
	BatchID                string
	FileID                 string
	HistoryID              string
	EventID                string
	OptionalProbe          bool
	SkipProbe              bool
	DeferCatalogProjection bool
}

func (service *LibraryService) createImportFile(ctx context.Context, params importFileParams) (library.LibraryFile, library.HistoryRecord, library.FileEventRecord, error) {
	now := service.now()
	importedAt := now
	batchID := strings.TrimSpace(params.BatchID)
	if batchID == "" {
		batchID = uuid.NewString()
	}
	storage := library.FileStorage{Mode: "local_path", LocalPath: params.Path}
	media := mediaProbe{}
	var err error
	if params.SkipProbe {
		media = probeLocalMedia(params.Path)
	} else if params.OptionalProbe {
		media = service.probeLocalMedia(ctx, params.Path)
	} else {
		media, err = service.probeRequiredMedia(ctx, params.Path)
		if err != nil {
			return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
		}
	}
	mediaInfo := media.toMediaInfo()
	kind := strings.TrimSpace(params.Kind)
	if kind == "" {
		kind = inferImportFileKindFromProbe(media)
	}
	fileID := strings.TrimSpace(params.FileID)
	if fileID == "" {
		fileID = uuid.NewString()
	}
	originPath := strings.TrimSpace(params.OriginPath)
	if originPath == "" {
		originPath = params.Path
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:          fileID,
		LibraryID:   params.LibraryID,
		Kind:        kind,
		Name:        resolveStoredFileName(params.Path, params.Name),
		DisplayName: strings.TrimSpace(params.Name),
		Metadata: library.FileMetadata{
			Title:  firstNonEmpty(media.Title, strings.TrimSpace(params.Name)),
			Author: strings.TrimSpace(media.Artist),
		},
		Storage: storage,
		Origin: library.FileOrigin{
			Kind: "import",
			Import: &library.ImportOrigin{
				BatchID:        batchID,
				ImportPath:     originPath,
				ImportedAt:     importedAt,
				KeepSourceFile: params.KeepSourceFile,
			},
		},
		Media:     &mediaInfo,
		State:     library.FileState{Status: "active"},
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
	}
	if err := service.files.Save(ctx, fileItem); err != nil {
		return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
	}
	// Embedded artwork is a generated auxiliary asset. Keep it outside a
	// referenced source folder and register it before the local-music and
	// Catalog projections consume this import.
	_, _ = service.ensureEmbeddedArtwork(ctx, fileItem, &media)
	historyID := strings.TrimSpace(params.HistoryID)
	if historyID == "" {
		historyID = uuid.NewString()
	}
	history, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          historyID,
		LibraryID:   params.LibraryID,
		Category:    "import",
		Action:      params.Action,
		DisplayName: fileItem.DisplayName,
		Status:      "succeeded",
		Source:      library.HistoryRecordSource{Kind: "import", RunID: strings.TrimSpace(params.SessionRunID)},
		Refs:        library.HistoryRecordRefs{ImportBatchID: batchID, FileIDs: []string{fileItem.ID}},
		Files: []library.OperationOutputFile{{
			FileID:    fileItem.ID,
			Kind:      string(fileItem.Kind),
			Format:    mediaFormatFromFile(fileItem),
			SizeBytes: mediaSizeFromFile(fileItem),
			IsPrimary: true,
			Deleted:   fileItem.State.Deleted,
		}},
		Metrics: buildOperationMetrics([]library.LibraryFile{fileItem}),
		ImportMeta: &library.ImportRecordMeta{
			ImportPath:     originPath,
			KeepSourceFile: params.KeepSourceFile,
			ImportedAt:     importedAt.Format(time.RFC3339),
		},
		OccurredAt: &now,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	})
	if err != nil {
		return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
	}
	if err := service.histories.Save(ctx, history); err != nil {
		return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
	}
	detailJSON := marshalJSON(dto.FileEventDetailDTO{
		Cause:  dto.FileEventCauseDTO{Category: "import", ImportBatchID: batchID},
		After:  &dto.FileEventFileSnapshotDTO{FileID: fileItem.ID, Kind: string(fileItem.Kind), Name: fileItem.DisplayName, LocalPath: fileItem.Storage.LocalPath, DocumentID: fileItem.Storage.DocumentID},
		Import: &dto.LibraryImportOriginDTO{BatchID: batchID, ImportPath: originPath, ImportedAt: importedAt.Format(time.RFC3339), KeepSourceFile: params.KeepSourceFile},
	})
	eventID := strings.TrimSpace(params.EventID)
	if eventID == "" {
		eventID = uuid.NewString()
	}
	eventRecord, err := library.NewFileEventRecord(library.FileEventRecordParams{
		ID:         eventID,
		LibraryID:  params.LibraryID,
		FileID:     fileItem.ID,
		EventType:  "file_imported",
		DetailJSON: detailJSON,
		CreatedAt:  &now,
	})
	if err != nil {
		return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
	}
	if err := service.fileEvents.Save(ctx, eventRecord); err != nil {
		return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
	}
	if err := service.touchLibrary(ctx, params.LibraryID, now); err != nil {
		return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
	}
	service.syncListenLocalTrackFromFile(ctx, fileItem, &media)
	if !params.DeferCatalogProjection {
		if err := service.syncCatalogProjection(ctx, params.LibraryID); err != nil {
			return library.LibraryFile{}, library.HistoryRecord{}, library.FileEventRecord{}, err
		}
	}
	return fileItem, history, eventRecord, nil
}

func (service *LibraryService) resolveSourceFileForTranscode(ctx context.Context, request dto.CreateTranscodeJobRequest) (library.LibraryFile, error) {
	if fileID := strings.TrimSpace(request.FileID); fileID != "" {
		return service.files.Get(ctx, fileID)
	}
	path := strings.TrimSpace(request.InputPath)
	if path == "" {
		return library.LibraryFile{}, fmt.Errorf("fileId or inputPath is required")
	}
	resolvedPath, err := service.resolveInputPath(ctx, path, request.Source, false)
	if err != nil {
		return library.LibraryFile{}, err
	}
	name := strings.TrimSpace(request.Title)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(resolvedPath), filepath.Ext(resolvedPath))
	}
	libraryItem, err := service.ensureLibrary(ctx, ensureLibraryParams{
		LibraryID:       request.LibraryID,
		FallbackName:    deriveLibraryName(name, resolvedPath),
		CreatedBySource: "transcode_import",
	})
	if err != nil {
		return library.LibraryFile{}, err
	}
	imported, _, _, err := service.createImportFile(ctx, importFileParams{
		LibraryID:      libraryItem.ID,
		Path:           resolvedPath,
		Name:           name,
		Source:         request.Source,
		SessionRunID:   request.RunID,
		KeepSourceFile: true,
		Action:         "import_media",
	})
	return imported, err
}

func inferImportFileKindFromProbe(probe mediaProbe) string {
	if mediaProbeHasVideo(probe) {
		return string(library.FileKindVideo)
	}
	if mediaProbeHasAudio(probe) {
		return string(library.FileKindAudio)
	}
	return string(library.FileKindVideo)
}

func resolveTranscodeParentOperationID(sourceFile library.LibraryFile) string {
	switch strings.TrimSpace(sourceFile.Origin.Kind) {
	case "download", "transcode":
		return strings.TrimSpace(sourceFile.Origin.OperationID)
	default:
		return ""
	}
}

func (service *LibraryService) buildLibraryDTO(ctx context.Context, item library.Library) (dto.LibraryDTO, error) {
	files, err := service.files.ListByLibraryID(ctx, item.ID)
	if err != nil {
		return dto.LibraryDTO{}, err
	}
	history, err := service.histories.ListByLibraryID(ctx, item.ID)
	if err != nil {
		return dto.LibraryDTO{}, err
	}
	workspaceStates, err := service.workspace.ListByLibraryID(ctx, item.ID)
	if err != nil {
		return dto.LibraryDTO{}, err
	}
	var workspaceHead *dto.WorkspaceStateRecordDTO
	if head, headErr := service.workspace.GetHeadByLibraryID(ctx, item.ID); headErr == nil {
		mapped := toWorkspaceDTO(head)
		workspaceHead = &mapped
	} else if headErr != nil && headErr != library.ErrWorkspaceStateNotFound {
		return dto.LibraryDTO{}, headErr
	}
	fileEvents, err := service.fileEvents.ListByLibraryID(ctx, item.ID)
	if err != nil {
		return dto.LibraryDTO{}, err
	}
	moduleConfig, err := service.getModuleConfig(ctx)
	if err != nil {
		return dto.LibraryDTO{}, err
	}
	fileDTOs := make([]dto.LibraryFileDTO, 0, len(files))
	for _, fileItem := range files {
		fileDTO, buildErr := service.buildFileDTOWithConfig(ctx, fileItem, moduleConfig)
		if buildErr != nil {
			fileDTO = toLibraryFileDTO(fileItem)
		}
		fileDTOs = append(fileDTOs, fileDTO)
	}
	historyDTOs := make([]dto.LibraryHistoryRecordDTO, 0, len(history))
	for _, record := range history {
		historyDTOs = append(historyDTOs, toHistoryDTO(record))
	}
	workspaceDTOs := make([]dto.WorkspaceStateRecordDTO, 0, len(workspaceStates))
	for _, state := range workspaceStates {
		workspaceDTOs = append(workspaceDTOs, toWorkspaceDTO(state))
	}
	fileEventDTOs := make([]dto.FileEventRecordDTO, 0, len(fileEvents))
	for _, eventRecord := range fileEvents {
		fileEventDTOs = append(fileEventDTOs, toFileEventDTO(eventRecord))
	}
	return dto.LibraryDTO{
		Version:   dto.LibrarySchemaVersion,
		ID:        item.ID,
		Name:      item.Name,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
		CreatedBy: dto.LibraryCreateMetaDTO{
			Source:             item.CreatedBy.Source,
			TriggerOperationID: item.CreatedBy.TriggerOperationID,
			ImportBatchID:      item.CreatedBy.ImportBatchID,
			Actor:              item.CreatedBy.Actor,
		},
		Files: fileDTOs,
		Records: dto.LibraryRecordsDTO{
			History:            historyDTOs,
			WorkspaceStateHead: workspaceHead,
			WorkspaceStates:    workspaceDTOs,
			FileEvents:         fileEventDTOs,
		},
	}, nil
}

func toModuleConfigDTO(config library.ModuleConfig) dto.LibraryModuleConfigDTO {
	return dto.LibraryModuleConfigDTO{
		Workspace: dto.LibraryWorkspaceConfigDTO{FastReadLatestState: config.Workspace.FastReadLatestState},
	}
}

func toDomainModuleConfig(config dto.LibraryModuleConfigDTO) library.ModuleConfig {
	result := library.DefaultModuleConfig()
	result.Workspace.FastReadLatestState = config.Workspace.FastReadLatestState
	return library.NormalizeModuleConfig(result)
}

func toLibraryFileDTO(item library.LibraryFile) dto.LibraryFileDTO {
	result := dto.LibraryFileDTO{
		ID:          item.ID,
		LibraryID:   item.LibraryID,
		Kind:        string(item.Kind),
		Name:        item.Name,
		DisplayName: resolveLibraryFileDisplayName(item),
		FileName:    resolveLibraryFileName(item),
		Storage: dto.LibraryFileStorageDTO{
			Mode: item.Storage.Mode, LocalPath: item.Storage.LocalPath, DocumentID: item.Storage.DocumentID,
			RootID: item.Storage.RootID, RelativePath: item.Storage.RelativePath,
		},
		Lineage:           dto.LibraryFileLineageDTO{RootFileID: item.Lineage.RootFileID},
		Metadata:          dto.LibraryFileMetaDTO{Title: item.Metadata.Title, Author: item.Metadata.Author, Extractor: item.Metadata.Extractor},
		LatestOperationID: item.LatestOperationID,
		State: dto.LibraryFileStateDTO{
			Status:      item.State.Status,
			Deleted:     item.State.Deleted,
			Archived:    item.State.Archived,
			LastError:   item.State.LastError,
			LastChecked: item.State.LastChecked,
		},
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
	if item.Origin.Import != nil {
		result.Origin = dto.LibraryFileOriginDTO{
			Kind: item.Origin.Kind,
			Import: &dto.LibraryImportOriginDTO{
				BatchID:        item.Origin.Import.BatchID,
				ImportPath:     item.Origin.Import.ImportPath,
				ImportedAt:     item.Origin.Import.ImportedAt.Format(time.RFC3339),
				KeepSourceFile: item.Origin.Import.KeepSourceFile,
			},
		}
	} else {
		result.Origin = dto.LibraryFileOriginDTO{Kind: item.Origin.Kind, OperationID: item.Origin.OperationID}
	}
	if item.Media != nil {
		media := toLibraryMediaInfoDTO(*item.Media)
		result.Media = &media
	}
	if format := strings.TrimSpace(mediaFormatFromFile(item)); format != "" {
		if result.Media == nil {
			result.Media = &dto.LibraryMediaInfoDTO{}
		}
		if strings.TrimSpace(result.Media.Format) == "" {
			result.Media.Format = format
		}
	}
	result.DisplayLabel = buildLibraryFileDisplayLabel(result)
	return result
}

func toOperationDTO(item library.LibraryOperation) dto.LibraryOperationDTO {
	startedAt := ""
	if item.StartedAt != nil && !item.StartedAt.IsZero() {
		startedAt = item.StartedAt.Format(time.RFC3339)
	}
	finishedAt := ""
	if item.FinishedAt != nil && !item.FinishedAt.IsZero() {
		finishedAt = item.FinishedAt.Format(time.RFC3339)
	}
	return dto.LibraryOperationDTO{
		ID:                    item.ID,
		LibraryID:             item.LibraryID,
		Kind:                  item.Kind,
		Status:                string(item.Status),
		DisplayName:           item.DisplayName,
		Correlation:           dto.OperationCorrelationDTO{RequestID: item.Correlation.RequestID, RunID: item.Correlation.RunID, ParentOperationID: item.Correlation.ParentOperationID},
		InputJSON:             item.InputJSON,
		OutputJSON:            item.OutputJSON,
		SourceDomain:          item.SourceDomain,
		SourceIcon:            item.SourceIcon,
		Meta:                  dto.OperationMetaDTO{Platform: item.Meta.Platform, Uploader: item.Meta.Uploader, PublishTime: item.Meta.PublishTime},
		Request:               toOperationRequestPreviewDTO(item),
		Progress:              toProgressDTO(item.Progress, item.Kind, item.Status, item.ErrorMessage),
		OutputFiles:           toOutputFileDTOs(item.OutputFiles),
		DetachedOutputFileIDs: detachedOperationOutputFileIDs(item.OutputJSON),
		ThumbnailPreviewPath:  extractOperationThumbnailPreviewPath(item.OutputJSON),
		Metrics:               dto.OperationMetricsDTO{FileCount: item.Metrics.FileCount, TotalSizeBytes: item.Metrics.TotalSizeBytes, DurationMs: item.Metrics.DurationMs},
		ErrorCode:             item.ErrorCode,
		ErrorMessage:          item.ErrorMessage,
		CreatedAt:             item.CreatedAt.Format(time.RFC3339),
		StartedAt:             startedAt,
		FinishedAt:            finishedAt,
	}
}

func toOperationListItemDTO(item library.LibraryOperation, libraryName string) dto.OperationListItemDTO {
	startedAt := ""
	if item.StartedAt != nil && !item.StartedAt.IsZero() {
		startedAt = item.StartedAt.Format(time.RFC3339)
	}
	finishedAt := ""
	if item.FinishedAt != nil && !item.FinishedAt.IsZero() {
		finishedAt = item.FinishedAt.Format(time.RFC3339)
	}
	return dto.OperationListItemDTO{
		OperationID:           item.ID,
		LibraryID:             item.LibraryID,
		LibraryName:           strings.TrimSpace(libraryName),
		Name:                  item.DisplayName,
		Kind:                  item.Kind,
		Status:                string(item.Status),
		Correlation:           dto.OperationCorrelationDTO{RequestID: item.Correlation.RequestID, RunID: item.Correlation.RunID, ParentOperationID: item.Correlation.ParentOperationID},
		Domain:                item.SourceDomain,
		SourceIcon:            item.SourceIcon,
		Platform:              item.Meta.Platform,
		Uploader:              item.Meta.Uploader,
		PublishTime:           item.Meta.PublishTime,
		Request:               toOperationRequestPreviewDTO(item),
		Progress:              toProgressDTO(item.Progress, item.Kind, item.Status, item.ErrorMessage),
		OutputFiles:           toOutputFileDTOs(item.OutputFiles),
		DetachedOutputFileIDs: detachedOperationOutputFileIDs(item.OutputJSON),
		ThumbnailPreviewPath:  extractOperationThumbnailPreviewPath(item.OutputJSON),
		Metrics:               dto.OperationMetricsDTO{FileCount: item.Metrics.FileCount, TotalSizeBytes: item.Metrics.TotalSizeBytes, DurationMs: item.Metrics.DurationMs},
		ErrorCode:             item.ErrorCode,
		ErrorMessage:          item.ErrorMessage,
		StartedAt:             startedAt,
		FinishedAt:            finishedAt,
		CreatedAt:             item.CreatedAt.Format(time.RFC3339),
	}
}

const (
	operationDownloadMethodResourceSniff = "resource-sniff"
	operationDownloadMethodYTDLP         = "yt-dlp"
)

func resolveOperationDownloadMethod(request dto.CreateYTDLPJobRequest, item library.LibraryOperation) string {
	if item.Kind != "download" {
		return ""
	}
	if isResourceDownloadRequest(request) ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.Meta.Platform)), "resource:") {
		return operationDownloadMethodResourceSniff
	}
	return operationDownloadMethodYTDLP
}

func isResourceDownloadRequest(request dto.CreateYTDLPJobRequest) bool {
	return strings.TrimSpace(request.ResourceSessionID) != "" ||
		strings.TrimSpace(request.ResourceMediaID) != "" ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(request.Extractor)), "resource:")
}

func toOperationRequestPreviewDTO(item library.LibraryOperation) *dto.OperationRequestPreviewDTO {
	switch item.Kind {
	case "download":
		request := dto.CreateYTDLPJobRequest{}
		if err := json.Unmarshal([]byte(item.InputJSON), &request); err != nil {
			return nil
		}
		preview := dto.OperationRequestPreviewDTO{
			URL:            strings.TrimSpace(request.URL),
			Caller:         strings.TrimSpace(request.Caller),
			Extractor:      firstNonEmpty(strings.TrimSpace(item.Meta.Platform), strings.TrimSpace(request.Extractor)),
			Author:         firstNonEmpty(strings.TrimSpace(item.Meta.Uploader), strings.TrimSpace(request.Author)),
			ThumbnailURL:   strings.TrimSpace(request.ThumbnailURL),
			DownloadMethod: resolveOperationDownloadMethod(request, item),
		}
		if preview == (dto.OperationRequestPreviewDTO{}) {
			return nil
		}
		return &preview
	case "transcode":
		request := dto.CreateTranscodeJobRequest{}
		if err := json.Unmarshal([]byte(item.InputJSON), &request); err != nil {
			return nil
		}
		preview := dto.OperationRequestPreviewDTO{
			FileID:                         strings.TrimSpace(request.FileID),
			InputPath:                      strings.TrimSpace(request.InputPath),
			RootFileID:                     strings.TrimSpace(request.RootFileID),
			PresetID:                       strings.TrimSpace(request.PresetID),
			Format:                         strings.TrimSpace(request.Format),
			VideoCodec:                     strings.TrimSpace(request.VideoCodec),
			AudioCodec:                     strings.TrimSpace(request.AudioCodec),
			QualityMode:                    strings.TrimSpace(request.QualityMode),
			Scale:                          strings.TrimSpace(request.Scale),
			Width:                          request.Width,
			Height:                         request.Height,
			DeleteSourceFileAfterTranscode: request.DeleteSourceFileAfterTranscode,
		}
		if preview == (dto.OperationRequestPreviewDTO{}) {
			return nil
		}
		return &preview
	default:
		return nil
	}
}

func toHistoryDTO(item library.HistoryRecord) dto.LibraryHistoryRecordDTO {
	operationID := strings.TrimSpace(item.Refs.OperationID)
	if operationID == "" {
		operationID = strings.TrimSpace(item.Refs.SubjectOperationID)
	}
	result := dto.LibraryHistoryRecordDTO{
		RecordID:    item.ID,
		LibraryID:   item.LibraryID,
		Category:    item.Category,
		Action:      item.Action,
		DisplayName: item.DisplayName,
		Status:      item.Status,
		Source:      dto.LibraryHistoryRecordSourceDTO{Kind: item.Source.Kind, Caller: item.Source.Caller, RunID: item.Source.RunID, Actor: item.Source.Actor},
		Refs:        dto.LibraryHistoryRecordRefsDTO{OperationID: operationID, ImportBatchID: item.Refs.ImportBatchID, FileIDs: item.Refs.FileIDs, FileEventIDs: item.Refs.FileEventIDs},
		Files:       toOutputFileDTOs(item.Files),
		Metrics:     dto.OperationMetricsDTO{FileCount: item.Metrics.FileCount, TotalSizeBytes: item.Metrics.TotalSizeBytes, DurationMs: item.Metrics.DurationMs},
		OccurredAt:  item.OccurredAt.Format(time.RFC3339),
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
	}
	if item.ImportMeta != nil {
		importedAt := strings.TrimSpace(item.ImportMeta.ImportedAt)
		if importedAt == "" {
			importedAt = item.OccurredAt.Format(time.RFC3339)
		}
		result.ImportMeta = &dto.LibraryImportRecordMetaDTO{ImportPath: item.ImportMeta.ImportPath, KeepSourceFile: item.ImportMeta.KeepSourceFile, ImportedAt: importedAt}
	}
	if item.OperationMeta != nil {
		result.OperationMeta = &dto.LibraryOperationRecordMetaDTO{Kind: item.OperationMeta.Kind, ErrorCode: item.OperationMeta.ErrorCode, ErrorMessage: item.OperationMeta.ErrorMessage}
	}
	return result
}

func toWorkspaceDTO(item library.WorkspaceStateRecord) dto.WorkspaceStateRecordDTO {
	return dto.WorkspaceStateRecordDTO{ID: item.ID, LibraryID: item.LibraryID, StateVersion: item.StateVersion, StateJSON: item.StateJSON, OperationID: item.OperationID, CreatedAt: item.CreatedAt.Format(time.RFC3339)}
}

func toFileEventDTO(item library.FileEventRecord) dto.FileEventRecordDTO {
	detail := dto.FileEventDetailDTO{}
	if strings.TrimSpace(item.DetailJSON) != "" {
		_ = json.Unmarshal([]byte(item.DetailJSON), &detail)
	}
	operationID := strings.TrimSpace(item.OperationID)
	if operationID == "" {
		operationID = strings.TrimSpace(detail.Cause.OperationID)
	}
	timestamp := item.CreatedAt.Format(time.RFC3339)
	return dto.FileEventRecordDTO{ID: item.ID, LibraryID: item.LibraryID, FileID: item.FileID, OperationID: operationID, EventType: item.EventType, Detail: detail, OccurredAt: timestamp, CreatedAt: timestamp}
}

func toProgressDTO(
	progress *library.OperationProgress,
	kind string,
	status library.OperationStatus,
	errorMessage string,
) *dto.OperationProgressDTO {
	if progress == nil {
		return nil
	}
	speed := strings.TrimSpace(progress.Speed)
	return &dto.OperationProgressDTO{
		Stage:       progress.Stage,
		Percent:     progress.Percent,
		Current:     progress.Current,
		Total:       progress.Total,
		Speed:       speed,
		SpeedMetric: parseProgressSpeedMetric(kind, speed),
		Message:     normalizeTerminalProgressMessage(kind, status, progress.Message, errorMessage),
		UpdatedAt:   progress.UpdatedAt,
	}
}

func toOutputFileDTOs(items []library.OperationOutputFile) []dto.OperationOutputFileDTO {
	result := make([]dto.OperationOutputFileDTO, 0, len(items))
	for _, item := range items {
		result = append(result, dto.OperationOutputFileDTO{
			FileID:    item.FileID,
			Kind:      item.Kind,
			Format:    item.Format,
			SizeBytes: item.SizeBytes,
			IsPrimary: item.IsPrimary,
			Deleted:   item.Deleted,
		})
	}
	return result
}

func buildOperationMetrics(files []library.LibraryFile) library.OperationMetrics {
	metrics := library.OperationMetrics{}
	var total int64
	for _, item := range files {
		if item.State.Deleted {
			continue
		}
		metrics.FileCount++
		if item.Media != nil && item.Media.SizeBytes != nil {
			total += *item.Media.SizeBytes
		}
	}
	if total > 0 {
		metrics.TotalSizeBytes = &total
	}
	return metrics
}

func buildOperationMetricsForOperation(files []library.LibraryFile, startedAt *time.Time, finishedAt *time.Time) library.OperationMetrics {
	metrics := buildOperationMetrics(files)
	metrics.DurationMs = durationMsBetween(startedAt, finishedAt)
	return metrics
}

func durationMsBetween(startedAt *time.Time, finishedAt *time.Time) *int64 {
	if startedAt == nil || finishedAt == nil || startedAt.IsZero() || finishedAt.IsZero() {
		return nil
	}
	duration := finishedAt.Sub(*startedAt).Milliseconds()
	if duration < 0 {
		duration = 0
	}
	return &duration
}

func mediaFormatFromFile(item library.LibraryFile) string {
	if item.Media != nil && strings.TrimSpace(item.Media.Format) != "" {
		return item.Media.Format
	}
	format := normalizeTranscodeFormat(filepath.Ext(strings.TrimSpace(item.Storage.LocalPath)))
	if format != "" {
		return format
	}
	return normalizeTranscodeFormat(filepath.Ext(strings.TrimSpace(item.Name)))
}

func mediaSizeFromFile(item library.LibraryFile) *int64 {
	if item.Media == nil {
		return nil
	}
	return item.Media.SizeBytes
}

func cloneMediaInfo(value *library.MediaInfo) *library.MediaInfo {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func mergeMediaInfo(base *library.MediaInfo, override *library.MediaInfo) *library.MediaInfo {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		return cloneMediaInfo(override)
	}
	if override == nil {
		return base
	}
	if strings.TrimSpace(override.Format) != "" {
		base.Format = override.Format
	}
	if strings.TrimSpace(override.Codec) != "" {
		base.Codec = override.Codec
	}
	if strings.TrimSpace(override.VideoCodec) != "" {
		base.VideoCodec = override.VideoCodec
	}
	if strings.TrimSpace(override.AudioCodec) != "" {
		base.AudioCodec = override.AudioCodec
	}
	if override.DurationMs != nil && *override.DurationMs > 0 {
		value := *override.DurationMs
		base.DurationMs = &value
	}
	if override.Width != nil && *override.Width > 0 {
		value := *override.Width
		base.Width = &value
	}
	if override.Height != nil && *override.Height > 0 {
		value := *override.Height
		base.Height = &value
	}
	if override.FrameRate != nil && *override.FrameRate > 0 {
		value := *override.FrameRate
		base.FrameRate = &value
	}
	if override.BitrateKbps != nil && *override.BitrateKbps > 0 {
		value := *override.BitrateKbps
		base.BitrateKbps = &value
	}
	if override.VideoBitrateKbps != nil && *override.VideoBitrateKbps > 0 {
		value := *override.VideoBitrateKbps
		base.VideoBitrateKbps = &value
	}
	if override.AudioBitrateKbps != nil && *override.AudioBitrateKbps > 0 {
		value := *override.AudioBitrateKbps
		base.AudioBitrateKbps = &value
	}
	if override.Channels != nil && *override.Channels > 0 {
		value := *override.Channels
		base.Channels = &value
	}
	if override.SizeBytes != nil && *override.SizeBytes > 0 {
		value := *override.SizeBytes
		base.SizeBytes = &value
	}
	if override.DPI != nil && *override.DPI > 0 {
		value := *override.DPI
		base.DPI = &value
	}
	if strings.TrimSpace(base.Codec) == "" {
		base.Codec = firstNonEmpty(base.VideoCodec, base.AudioCodec)
	}
	return base
}

func rootFileID(item library.LibraryFile) string {
	if strings.TrimSpace(item.Lineage.RootFileID) != "" {
		return item.Lineage.RootFileID
	}
	return item.ID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func resolveHistorySourceKind(source string) string {
	if strings.EqualFold(strings.TrimSpace(source), "agent") {
		return "agent"
	}
	if strings.TrimSpace(source) == "" {
		return "manual"
	}
	return strings.ToLower(strings.TrimSpace(source))
}

func deriveLibraryName(name string, fallback string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed != "" {
		return trimmed
	}
	if strings.TrimSpace(fallback) == "" {
		return "Library"
	}
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(fallback)), filepath.Ext(strings.TrimSpace(fallback)))
	if base == "" || base == "." {
		return "Library"
	}
	return base
}

func resolveInitialLibraryName(libraryID string, fallback string, initialNameFromID bool) string {
	if initialNameFromID {
		trimmedID := strings.TrimSpace(libraryID)
		if trimmedID != "" {
			return trimmedID
		}
	}
	return defaultLibraryName(fallback)
}

func resolveLibraryNameFromFile(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	base := filepath.Base(trimmed)
	if base == "" || base == "." {
		return ""
	}
	withoutExt := strings.TrimSuffix(base, filepath.Ext(base))
	if strings.TrimSpace(withoutExt) == "" {
		return base
	}
	return withoutExt
}

func defaultLibraryName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "Library"
	}
	return trimmed
}

func marshalJSON(value interface{}) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func toLookup(items []string) map[string]struct{} {
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.ToLower(strings.TrimSpace(item))
		if trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

func paginateOperationList(items []dto.OperationListItemDTO, offset int, limit int) []dto.OperationListItemDTO {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []dto.OperationListItemDTO{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func paginateHistory(items []dto.LibraryHistoryRecordDTO, offset int, limit int) []dto.LibraryHistoryRecordDTO {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []dto.LibraryHistoryRecordDTO{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func paginateFileEvents(items []dto.FileEventRecordDTO, offset int, limit int) []dto.FileEventRecordDTO {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []dto.FileEventRecordDTO{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func (service *LibraryService) deriveManagedOutputPath(ctx context.Context, name string, format string, sourcePath string) (string, error) {
	resolvedFormat := normalizeTranscodeFormat(format)
	if resolvedFormat == "" {
		resolvedFormat = normalizeTranscodeFormat(filepath.Ext(sourcePath))
	}
	if resolvedFormat == "" {
		resolvedFormat = "mp4"
	}
	safeName := sanitizeFileName(name)
	if safeName == "" {
		safeName = uuid.NewString()
	}
	outputDir := strings.TrimSpace(filepath.Dir(sourcePath))
	if outputDir == "" || outputDir == "." {
		baseDir, err := libraryBaseDir()
		if err != nil {
			return "", err
		}
		outputDir = filepath.Join(baseDir, "transcodes")
	}
	return filepath.Join(outputDir, fmt.Sprintf("%s.%s", safeName, resolvedFormat)), nil
}

func sanitizeFileName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "", "\"", "", "<", "", ">", "", "|", "-")
	trimmed = replacer.Replace(trimmed)
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	return strings.Trim(trimmed, ". _")
}
