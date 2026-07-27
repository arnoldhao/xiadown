package libraryrootsync

import (
	"context"
	"time"

	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
	domain "xiadown/internal/domain/libraryrootsync"
)

type Root struct {
	ID       string
	Name     string
	Path     string
	VolumeID string
	Mode     string
	Online   bool
}

type RootProvider func(context.Context) ([]Root, error)

type FileRepository interface {
	List(context.Context) ([]library.LibraryFile, error)
}

type ImportRegistrar interface {
	EnsureProfessionalImportLibrary(ctx context.Context, libraryID, displayName string) (string, error)
	RegisterProfessionalImport(
		ctx context.Context,
		request libraryservice.ProfessionalImportRequest,
	) (libraryservice.ProfessionalImportRegistration, error)
}

type CatalogProjector interface {
	Run(context.Context) (libraryservice.CatalogBackfillResult, error)
}

type ScopedCatalogProjector interface {
	RunLibrary(context.Context, string) (libraryservice.CatalogBackfillResult, error)
}

type ProjectionNotifier interface {
	NotifyCatalogProjectionCompleted(context.Context, string)
}

// CatalogProjectionBatchNotifier lets a scanner collapse a large import into
// one UI invalidation. Implementations that need per-file payloads can omit
// this interface and retain the ProjectionNotifier fallback.
type CatalogProjectionBatchNotifier interface {
	NotifyCatalogProjectionBatchCompleted(context.Context, string, []string)
}

// CatalogAvailabilityNotifier emits one coalesced invalidation after a root
// scan. It deliberately avoids publishing one UI refresh per discovered file.
type CatalogAvailabilityNotifier interface {
	NotifyCatalogAvailabilityChanged(context.Context, string)
}

type StateDTO struct {
	RootID           string        `json:"rootId"`
	Status           domain.Status `json:"status"`
	Generation       int64         `json:"generation"`
	FullScan         bool          `json:"fullScan"`
	DiscoveredCount  int           `json:"discoveredCount"`
	ProcessedCount   int           `json:"processedCount"`
	UnchangedCount   int           `json:"unchangedCount"`
	DuplicateCount   int           `json:"duplicateCount"`
	MissingCount     int           `json:"missingCount"`
	FailedCount      int           `json:"failedCount"`
	ProcessedBytes   int64         `json:"processedBytes"`
	CancelRequested  bool          `json:"cancelRequested"`
	LastErrorCode    string        `json:"lastErrorCode,omitempty"`
	LastError        string        `json:"lastError,omitempty"`
	StartedAt        string        `json:"startedAt,omitempty"`
	FinishedAt       string        `json:"finishedAt,omitempty"`
	LastReconciledAt string        `json:"lastReconciledAt,omitempty"`
	UpdatedAt        string        `json:"updatedAt"`
}

type RootRequest struct {
	RootID string `json:"rootId"`
}

type watchEvent struct {
	path       string
	cursor     uint64
	overflow   bool
	checkpoint bool
	directory  bool
}

type scanRequest struct {
	full   bool
	paths  map[string]struct{}
	settle bool
	cursor uint64
}

func stateDTO(item domain.State) StateDTO {
	return StateDTO{
		RootID: item.RootID, Status: item.Status, Generation: item.Generation,
		FullScan: item.FullScan, DiscoveredCount: item.DiscoveredCount,
		ProcessedCount: item.ProcessedCount, UnchangedCount: item.UnchangedCount,
		DuplicateCount: item.DuplicateCount, MissingCount: item.MissingCount,
		FailedCount: item.FailedCount, ProcessedBytes: item.ProcessedBytes,
		CancelRequested: item.CancelRequested,
		LastErrorCode:   item.LastErrorCode, LastError: item.LastError,
		StartedAt: formatTime(item.StartedAt), FinishedAt: formatTime(item.FinishedAt),
		LastReconciledAt: formatTime(item.LastReconciledAt),
		UpdatedAt:        item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func formatTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
