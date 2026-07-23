package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"xiadown/internal/domain/library"
)

// LegacyCatalogBackfillService expands the legacy download/import bundle
// model into one default Catalog. It only writes catalog associations and
// migration state; it never changes a LibraryFile or touches the filesystem.
type LegacyCatalogBackfillService struct {
	libraries library.LibraryRepository
	files     library.FileRepository
	writer    library.CatalogBackfillWriter
	now       func() time.Time
	runMu     sync.Mutex
}

type CatalogBackfillResult struct {
	CatalogID        string
	BundlesProcessed int
	FilesProcessed   int64
	Completed        bool
}

func NewLegacyCatalogBackfillService(
	libraries library.LibraryRepository,
	files library.FileRepository,
	writer library.CatalogBackfillWriter,
) *LegacyCatalogBackfillService {
	return &LegacyCatalogBackfillService{
		libraries: libraries,
		files:     files,
		writer:    writer,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// DefaultLibraryCatalogID is deterministic across retries and installations;
// legacy bundle IDs are deliberately not part of it because every bundle is
// projected into the same user-facing Catalog.
func DefaultLibraryCatalogID() string {
	return deterministicCatalogID("catalog", "default-library")
}

func (service *LegacyCatalogBackfillService) Run(ctx context.Context) (CatalogBackfillResult, error) {
	service.runMu.Lock()
	defer service.runMu.Unlock()

	result := CatalogBackfillResult{CatalogID: DefaultLibraryCatalogID()}
	legacyLibraries, err := service.libraries.List(ctx)
	if err != nil {
		return result, fmt.Errorf("list legacy libraries: %w", err)
	}
	sort.Slice(legacyLibraries, func(left, right int) bool {
		return legacyLibraries[left].ID < legacyLibraries[right].ID
	})

	checkpoint, checkpointExists, err := service.writer.GetBackfillCheckpoint(
		ctx, LegacyCatalogProjectionID, library.MigrationPhaseBackfill,
	)
	if err != nil {
		return result, fmt.Errorf("load catalog backfill checkpoint: %w", err)
	}
	if checkpointExists && checkpoint.Status == library.MigrationStatusCompleted {
		changed, changeErr := service.legacySourcesChanged(ctx, legacyLibraries, checkpoint)
		if changeErr != nil {
			return result, changeErr
		}
		if !changed {
			result.FilesProcessed = checkpoint.Processed
			result.Completed = true
			return result, nil
		}
	}

	startedAt := service.timestamp()
	createdAt := startedAt
	cursor := ""
	processed := int64(0)
	catalogReady := checkpointExists
	if checkpointExists && checkpoint.Status != library.MigrationStatusCompleted {
		createdAt = checkpoint.CreatedAt
		cursor = checkpoint.Cursor
		processed = checkpoint.Processed
		if checkpoint.StartedAt != nil {
			startedAt = checkpoint.StartedAt.UTC()
		}
	} else if checkpointExists {
		// Completed is the boundary of the one-shot migration, not the end of
		// projection. A later legacy write starts an idempotent full scan so a
		// file added to an older (or previously empty) bundle cannot sit behind
		// the lexical cursor forever.
		createdAt = checkpoint.CreatedAt
		checkpointExists = false
	}
	result.FilesProcessed = processed
	currentCheckpoint := checkpoint

	for _, legacyLibrary := range legacyLibraries {
		if cursor != "" && legacyLibrary.ID <= cursor {
			continue
		}
		files, listErr := service.files.ListByLibraryID(ctx, legacyLibrary.ID)
		if listErr != nil {
			return result, fmt.Errorf("list files for legacy library %q: %w", legacyLibrary.ID, listErr)
		}
		// Empty download bundles are an implementation detail, not a user
		// library. In particular, an all-empty legacy database must not gain a
		// default Catalog merely because the migration was attempted.
		if len(files) == 0 {
			continue
		}

		stepAt := service.timestampAtLeast(startedAt)
		for _, file := range files {
			if file.UpdatedAt.After(stepAt) {
				stepAt = file.UpdatedAt.UTC()
			}
		}
		if !catalogReady {
			catalog, catalogErr := library.NewCatalog(library.CatalogParams{
				ID: DefaultLibraryCatalogID(), Name: "Library", Status: string(library.CatalogStatusActive),
				IsDefault: true, CreatedAt: &createdAt, UpdatedAt: &stepAt,
			})
			if catalogErr != nil {
				return result, fmt.Errorf("build default catalog: %w", catalogErr)
			}
			if catalogErr = service.writer.EnsureCatalog(ctx, catalog); catalogErr != nil {
				return result, fmt.Errorf("ensure default catalog: %w", catalogErr)
			}
			catalogReady = true
		}

		projected, projectionErr := projectLegacyCatalogBundle(DefaultLibraryCatalogID(), files, stepAt)
		if projectionErr != nil {
			return result, service.failCheckpoint(ctx, currentCheckpoint, checkpointExists, createdAt, startedAt, processed, projectionErr)
		}
		items := make([]library.CatalogBackfillItem, len(projected))
		for index, item := range projected {
			items[index] = library.CatalogBackfillItem{
				Item: item.Item, Assets: item.Assets, Mappings: item.Mappings,
			}
		}
		bundleMapping, mappingErr := library.NewLegacyMapping(library.LegacyMappingParams{
			MigrationID: LegacyCatalogProjectionID, CatalogID: DefaultLibraryCatalogID(),
			SourceType: string(library.LegacyEntityLibrary), SourceID: legacyLibrary.ID,
			TargetType: string(library.CatalogEntityCatalog), TargetID: DefaultLibraryCatalogID(),
			SourceFingerprint: catalogLegacyLibraryFingerprint(legacyLibrary), MigratedAt: stepAt,
		})
		if mappingErr != nil {
			return result, service.failCheckpoint(ctx, currentCheckpoint, checkpointExists, createdAt, startedAt, processed, mappingErr)
		}
		nextProcessed := processed + int64(len(files))
		nextCheckpoint, checkpointErr := library.NewMigrationCheckpoint(library.MigrationCheckpointParams{
			MigrationID: LegacyCatalogProjectionID, CatalogID: DefaultLibraryCatalogID(),
			Phase: string(library.MigrationPhaseBackfill), Status: string(library.MigrationStatusRunning),
			Cursor: legacyLibrary.ID, Processed: nextProcessed, Failed: 0,
			StartedAt: &startedAt, CreatedAt: &createdAt, UpdatedAt: &stepAt,
		})
		if checkpointErr != nil {
			return result, fmt.Errorf("build catalog backfill checkpoint: %w", checkpointErr)
		}
		bundle := library.CatalogBackfillBundle{
			LegacyLibraryID: legacyLibrary.ID,
			BundleMapping:   bundleMapping,
			Items:           items,
			Checkpoint:      nextCheckpoint,
		}
		if saveErr := service.writer.SaveBackfillBundle(ctx, bundle); saveErr != nil {
			return result, service.failCheckpoint(ctx, currentCheckpoint, checkpointExists, createdAt, startedAt, processed, saveErr)
		}
		checkpointExists = true
		currentCheckpoint = nextCheckpoint
		cursor = legacyLibrary.ID
		processed = nextProcessed
		result.BundlesProcessed++
		result.FilesProcessed = processed
	}

	if !catalogReady {
		// There were no files in any legacy bundle. No Catalog and therefore no
		// FK-bound checkpoint is written; a later run can discover new files.
		result.CatalogID = ""
		result.Completed = true
		return result, nil
	}
	finishedAt := service.timestampAtLeast(startedAt)
	if currentCheckpoint.UpdatedAt.After(finishedAt) {
		finishedAt = currentCheckpoint.UpdatedAt.UTC()
	}
	completed, err := library.NewMigrationCheckpoint(library.MigrationCheckpointParams{
		MigrationID: LegacyCatalogProjectionID, CatalogID: DefaultLibraryCatalogID(),
		Phase: string(library.MigrationPhaseBackfill), Status: string(library.MigrationStatusCompleted),
		Cursor: cursor, Processed: processed, Failed: 0,
		StartedAt: &startedAt, FinishedAt: &finishedAt, CreatedAt: &createdAt, UpdatedAt: &finishedAt,
	})
	if err != nil {
		return result, fmt.Errorf("build completed catalog backfill checkpoint: %w", err)
	}
	if err := service.writer.SaveBackfillCheckpoint(ctx, completed); err != nil {
		return result, fmt.Errorf("complete catalog backfill checkpoint: %w", err)
	}
	result.Completed = true
	return result, nil
}

// RunLibrary is the runtime write path. It projects only the affected legacy
// bundle and deliberately leaves the global migration checkpoint untouched;
// startup Run remains the durable full-reconciliation safety net.
func (service *LegacyCatalogBackfillService) RunLibrary(ctx context.Context, legacyLibraryID string) (CatalogBackfillResult, error) {
	service.runMu.Lock()
	defer service.runMu.Unlock()

	result := CatalogBackfillResult{CatalogID: DefaultLibraryCatalogID()}
	legacyLibraryID = strings.TrimSpace(legacyLibraryID)
	if legacyLibraryID == "" {
		return result, errors.New("legacy library id is required")
	}
	legacyLibrary, err := service.libraries.Get(ctx, legacyLibraryID)
	if err != nil {
		return result, fmt.Errorf("get legacy library %q: %w", legacyLibraryID, err)
	}
	files, err := service.files.ListByLibraryID(ctx, legacyLibrary.ID)
	if err != nil {
		return result, fmt.Errorf("list files for legacy library %q: %w", legacyLibrary.ID, err)
	}
	if len(files) == 0 {
		result.Completed = true
		return result, nil
	}
	projectedAt := service.timestamp()
	for _, file := range files {
		if file.UpdatedAt.After(projectedAt) {
			projectedAt = file.UpdatedAt.UTC()
		}
	}
	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: DefaultLibraryCatalogID(), Name: "Library", Status: string(library.CatalogStatusActive),
		IsDefault: true, CreatedAt: &projectedAt, UpdatedAt: &projectedAt,
	})
	if err != nil {
		return result, fmt.Errorf("build runtime catalog: %w", err)
	}
	if err := service.writer.EnsureCatalog(ctx, catalog); err != nil {
		return result, fmt.Errorf("ensure runtime catalog: %w", err)
	}
	projections, err := projectLegacyCatalogBundle(DefaultLibraryCatalogID(), files, projectedAt)
	if err != nil {
		return result, fmt.Errorf("project runtime legacy library %q: %w", legacyLibrary.ID, err)
	}
	items := make([]library.CatalogBackfillItem, len(projections))
	for index, item := range projections {
		items[index] = library.CatalogBackfillItem{Item: item.Item, Assets: item.Assets, Mappings: item.Mappings}
	}
	bundleMapping, err := library.NewLegacyMapping(library.LegacyMappingParams{
		MigrationID: LegacyCatalogProjectionID, CatalogID: DefaultLibraryCatalogID(),
		SourceType: string(library.LegacyEntityLibrary), SourceID: legacyLibrary.ID,
		TargetType: string(library.CatalogEntityCatalog), TargetID: DefaultLibraryCatalogID(),
		SourceFingerprint: catalogLegacyLibraryFingerprint(legacyLibrary), MigratedAt: projectedAt,
	})
	if err != nil {
		return result, fmt.Errorf("build runtime library mapping: %w", err)
	}
	checkpoint, err := library.NewMigrationCheckpoint(library.MigrationCheckpointParams{
		MigrationID: LegacyCatalogProjectionID, CatalogID: DefaultLibraryCatalogID(),
		Phase: string(library.MigrationPhaseBackfill), Status: string(library.MigrationStatusRunning),
		Cursor: legacyLibrary.ID, Processed: int64(len(files)), StartedAt: &projectedAt,
		CreatedAt: &projectedAt, UpdatedAt: &projectedAt,
	})
	if err != nil {
		return result, fmt.Errorf("build runtime projection checkpoint envelope: %w", err)
	}
	if err := service.writer.SaveRuntimeProjection(ctx, library.CatalogBackfillBundle{
		LegacyLibraryID: legacyLibrary.ID, BundleMapping: bundleMapping, Items: items, Checkpoint: checkpoint,
	}); err != nil {
		return result, fmt.Errorf("save runtime projection for legacy library %q: %w", legacyLibrary.ID, err)
	}
	result.BundlesProcessed = 1
	result.FilesProcessed = int64(len(files))
	result.Completed = true
	return result, nil
}

func (service *LegacyCatalogBackfillService) legacySourcesChanged(
	ctx context.Context,
	legacyLibraries []library.Library,
	checkpoint library.MigrationCheckpoint,
) (bool, error) {
	var fileCount int64
	for _, legacyLibrary := range legacyLibraries {
		files, err := service.files.ListByLibraryID(ctx, legacyLibrary.ID)
		if err != nil {
			return false, fmt.Errorf("check files for legacy library %q: %w", legacyLibrary.ID, err)
		}
		fileCount += int64(len(files))
		for _, file := range files {
			if file.UpdatedAt.After(checkpoint.UpdatedAt) {
				return true, nil
			}
		}
	}
	return fileCount != checkpoint.Processed, nil
}

func (service *LegacyCatalogBackfillService) failCheckpoint(
	ctx context.Context,
	previous library.MigrationCheckpoint,
	previousExists bool,
	createdAt time.Time,
	startedAt time.Time,
	processed int64,
	cause error,
) error {
	failedAt := service.timestampAtLeast(startedAt)
	cursor := ""
	if previousExists {
		cursor = previous.Cursor
		if previous.UpdatedAt.After(failedAt) {
			failedAt = previous.UpdatedAt
		}
	}
	failed, err := library.NewMigrationCheckpoint(library.MigrationCheckpointParams{
		MigrationID: LegacyCatalogProjectionID, CatalogID: DefaultLibraryCatalogID(),
		Phase: string(library.MigrationPhaseBackfill), Status: string(library.MigrationStatusFailed),
		Cursor: cursor, Processed: processed, Failed: 0, LastError: cause.Error(),
		StartedAt: &startedAt, FinishedAt: &failedAt, CreatedAt: &createdAt, UpdatedAt: &failedAt,
	})
	if err != nil {
		return errors.Join(cause, fmt.Errorf("build failed catalog checkpoint: %w", err))
	}
	if err := service.writer.SaveBackfillCheckpoint(ctx, failed); err != nil {
		return errors.Join(cause, fmt.Errorf("save failed catalog checkpoint: %w", err))
	}
	return cause
}

func (service *LegacyCatalogBackfillService) timestamp() time.Time {
	return service.now().UTC()
}

func (service *LegacyCatalogBackfillService) timestampAtLeast(minimum time.Time) time.Time {
	value := service.timestamp()
	if value.Before(minimum) {
		return minimum.UTC()
	}
	return value
}

func catalogLegacyLibraryFingerprint(item library.Library) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		item.ID, item.Name, item.CreatedBy.Source, item.CreatedBy.TriggerOperationID,
		item.CreatedBy.ImportBatchID, item.CreatedBy.Actor,
		item.CreatedAt.UTC().Format(time.RFC3339Nano), item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}, "\x00")))
	return "sha256:" + hex.EncodeToString(digest[:])
}
