package libraryimport

import (
	"context"
	"time"

	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
	importdomain "xiadown/internal/domain/libraryimport"
)

type MediaInspector interface {
	InspectProfessionalImport(ctx context.Context, path string) (libraryservice.ProfessionalImportProbe, error)
}

type LegacyImporter interface {
	EnsureProfessionalImportLibrary(ctx context.Context, libraryID, displayName string) (string, error)
	RegisterProfessionalImport(ctx context.Context, request libraryservice.ProfessionalImportRequest) (libraryservice.ProfessionalImportRegistration, error)
}

type CatalogProjector interface {
	Run(ctx context.Context) (libraryservice.CatalogBackfillResult, error)
}

type ScopedCatalogProjector interface {
	RunLibrary(ctx context.Context, libraryID string) (libraryservice.CatalogBackfillResult, error)
}

// ManagedRootRegistrar makes copy imports part of Catalog storage management
// instead of accepting an unrelated destination string.
type ManagedRootRegistrar interface {
	EnsureManagedImportRoot(ctx context.Context, path string) (string, error)
}

// CatalogProjectionNotifier emits the post-projection edge used by the
// desktop query cache. It is deliberately optional for headless importers.
type CatalogProjectionNotifier interface {
	NotifyCatalogProjectionCompleted(ctx context.Context, fileID string)
}

type DryRunCommand struct {
	RequestKey    string
	SourcePaths   []string
	LibraryID     string
	Mode          importdomain.Mode
	ManagedRoot   string
	HiddenPolicy  importdomain.HiddenPolicy
	SymlinkPolicy importdomain.SymlinkPolicy
}

type BatchRequest struct {
	BatchID string `json:"batchId"`
}

type ListBatchesRequest struct {
	Limit int `json:"limit,omitempty"`
}

type BatchDTO struct {
	ID              string                     `json:"id"`
	RequestKey      string                     `json:"requestKey"`
	LibraryID       string                     `json:"libraryId"`
	Mode            importdomain.Mode          `json:"mode"`
	ManagedRoot     string                     `json:"managedRoot,omitempty"`
	HiddenPolicy    importdomain.HiddenPolicy  `json:"hiddenPolicy"`
	SymlinkPolicy   importdomain.SymlinkPolicy `json:"symlinkPolicy"`
	Status          importdomain.BatchStatus   `json:"status"`
	Counts          BatchCountsDTO             `json:"counts"`
	LastErrorCode   string                     `json:"lastErrorCode,omitempty"`
	LastError       string                     `json:"lastError,omitempty"`
	CancelRequested bool                       `json:"cancelRequested"`
	StartedAt       string                     `json:"startedAt,omitempty"`
	FinishedAt      string                     `json:"finishedAt,omitempty"`
	CreatedAt       string                     `json:"createdAt"`
	UpdatedAt       string                     `json:"updatedAt"`
	Candidates      []CandidateDTO             `json:"candidates,omitempty"`
}

type BatchCountsDTO struct {
	Total      int   `json:"total"`
	Ready      int   `json:"ready"`
	Duplicate  int   `json:"duplicate"`
	Skipped    int   `json:"skipped"`
	Succeeded  int   `json:"succeeded"`
	Failed     int   `json:"failed"`
	TotalBytes int64 `json:"totalBytes"`
}

type CandidateDTO struct {
	ID                   string                       `json:"id"`
	SourcePath           string                       `json:"sourcePath"`
	RelativePath         string                       `json:"relativePath,omitempty"`
	DisplayName          string                       `json:"displayName"`
	Extension            string                       `json:"extension,omitempty"`
	Category             importdomain.Category        `json:"category"`
	MIMEType             string                       `json:"mimeType,omitempty"`
	MediaProbed          bool                         `json:"mediaProbed"`
	WasSymlink           bool                         `json:"wasSymlink"`
	SizeBytes            int64                        `json:"sizeBytes"`
	ModifiedAt           string                       `json:"modifiedAt,omitempty"`
	HashAlgorithm        string                       `json:"hashAlgorithm,omitempty"`
	ContentHash          string                       `json:"contentHash,omitempty"`
	Status               importdomain.CandidateStatus `json:"status"`
	DuplicateFileID      string                       `json:"duplicateFileId,omitempty"`
	DuplicateCandidateID string                       `json:"duplicateCandidateId,omitempty"`
	ManagedPath          string                       `json:"managedPath,omitempty"`
	FileID               string                       `json:"fileId,omitempty"`
	ErrorCode            string                       `json:"errorCode,omitempty"`
	ErrorMessage         string                       `json:"errorMessage,omitempty"`
	Attempts             int                          `json:"attempts"`
	CreatedAt            string                       `json:"createdAt"`
	UpdatedAt            string                       `json:"updatedAt"`
}

func toBatchDTO(batch importdomain.Batch, candidates []importdomain.Candidate) BatchDTO {
	result := BatchDTO{
		ID: batch.ID, RequestKey: batch.RequestKey, LibraryID: batch.LibraryID,
		Mode: batch.Mode, ManagedRoot: batch.ManagedRoot, HiddenPolicy: batch.HiddenPolicy,
		SymlinkPolicy: batch.SymlinkPolicy, Status: batch.Status, Counts: toBatchCountsDTO(batch.Counts),
		LastErrorCode: batch.LastErrorCode, LastError: batch.LastError,
		CancelRequested: batch.CancelRequested, CreatedAt: formatTime(batch.CreatedAt), UpdatedAt: formatTime(batch.UpdatedAt),
		StartedAt: formatTimePointer(batch.StartedAt), FinishedAt: formatTimePointer(batch.FinishedAt),
		Candidates: make([]CandidateDTO, 0, len(candidates)),
	}
	for _, item := range candidates {
		result.Candidates = append(result.Candidates, CandidateDTO{
			ID: item.ID, SourcePath: item.SourcePath, RelativePath: item.RelativePath,
			DisplayName: item.DisplayName, Extension: item.Extension, Category: item.Category,
			MIMEType: item.MIMEType, MediaProbed: item.MediaProbed, WasSymlink: item.WasSymlink,
			SizeBytes: item.SizeBytes, ModifiedAt: formatTime(item.ModifiedAt),
			HashAlgorithm: item.HashAlgorithm, ContentHash: item.ContentHash, Status: item.Status,
			DuplicateFileID: item.DuplicateFileID, DuplicateCandidateID: item.DuplicateCandidateID,
			ManagedPath: item.ManagedPath, FileID: item.FileID, ErrorCode: item.ErrorCode,
			ErrorMessage: item.ErrorMessage, Attempts: item.Attempts,
			CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt),
		})
	}
	return result
}

func toBatchCountsDTO(counts importdomain.BatchCounts) BatchCountsDTO {
	return BatchCountsDTO{
		Total: counts.Total, Ready: counts.Ready, Duplicate: counts.Duplicate,
		Skipped: counts.Skipped, Succeeded: counts.Succeeded, Failed: counts.Failed,
		TotalBytes: counts.TotalBytes,
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func formatTimePointer(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}

func fileKindFor(candidate importdomain.Candidate) string {
	switch candidate.Category {
	case importdomain.CategoryVideo:
		return string(library.FileKindVideo)
	case importdomain.CategoryAudio:
		return string(library.FileKindAudio)
	case importdomain.CategoryBook:
		return string(library.FileKindDocument)
	case importdomain.CategoryImage:
		// Standalone images must remain primary catalog items. Thumbnail is an
		// auxiliary role in the legacy model, so use other and let projection's
		// extension classifier assign the image category.
		return string(library.FileKindOther)
	default:
		switch candidate.Extension {
		case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md", ".rtf":
			return string(library.FileKindDocument)
		case ".woff", ".woff2", ".ttf", ".otf", ".eot":
			return string(library.FileKindFont)
		case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz":
			return string(library.FileKindArchive)
		case ".json":
			return string(library.FileKindAPI)
		case ".m3u8", ".mpd", ".f4m", ".ism":
			return string(library.FileKindManifest)
		default:
			return string(library.FileKindOther)
		}
	}
}
