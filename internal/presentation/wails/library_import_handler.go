package wails

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	application "xiadown/internal/application/libraryimport"
	importdomain "xiadown/internal/domain/libraryimport"
)

var ErrLibraryImportSelectionCancelled = errors.New("library import selection cancelled")

type SelectLibraryImportRequest struct {
	SelectionKind string                     `json:"selectionKind"`
	Mode          importdomain.Mode          `json:"mode"`
	LibraryID     string                     `json:"libraryId,omitempty"`
	HiddenPolicy  importdomain.HiddenPolicy  `json:"hiddenPolicy,omitempty"`
	SymlinkPolicy importdomain.SymlinkPolicy `json:"symlinkPolicy,omitempty"`
}

// LibraryImportHandler is intentionally the only path-bearing boundary for
// professional imports. Its request contains policy and IDs only; source and
// managed-root paths are returned by native Wails desktop dialogs and passed
// directly to the application service in memory.
type LibraryImportHandler struct {
	service *application.Service
	windows *WindowManager
}

func NewLibraryImportHandler(service *application.Service, windows *WindowManager) *LibraryImportHandler {
	return &LibraryImportHandler{service: service, windows: windows}
}

func (handler *LibraryImportHandler) ServiceName() string { return "LibraryImportHandler" }

func (handler *LibraryImportHandler) SelectAndDryRun(ctx context.Context, request SelectLibraryImportRequest) (application.BatchDTO, error) {
	if handler == nil || handler.service == nil || handler.windows == nil {
		return application.BatchDTO{}, fmt.Errorf("library import handler is not configured")
	}
	var sources []string
	var err error
	switch strings.ToLower(strings.TrimSpace(request.SelectionKind)) {
	case "directory":
		var selected string
		selected, err = handler.windows.SelectMainDirectoryDialog("Select Library Folder", "")
		if strings.TrimSpace(selected) != "" {
			sources = []string{selected}
		}
	case "files", "file", "":
		sources, err = handler.windows.SelectFilesDialog("Select Library Files", "")
	default:
		return application.BatchDTO{}, fmt.Errorf("unsupported library import selection kind")
	}
	if err != nil {
		return application.BatchDTO{}, err
	}
	if len(sources) == 0 {
		return application.BatchDTO{}, ErrLibraryImportSelectionCancelled
	}
	managedRoot := ""
	referenceRoots := make([]string, 0)
	if request.Mode == importdomain.ModeCopy {
		managedRoot, err = handler.windows.SelectMainDirectoryDialog("Select Managed Library Location", "")
		if err != nil {
			return application.BatchDTO{}, err
		}
		if strings.TrimSpace(managedRoot) == "" {
			return application.BatchDTO{}, ErrLibraryImportSelectionCancelled
		}
	} else {
		seen := make(map[string]struct{})
		for _, source := range sources {
			root := source
			if info, statErr := os.Stat(source); statErr == nil && !info.IsDir() {
				root = filepath.Dir(source)
			}
			root = filepath.Clean(strings.TrimSpace(root))
			if root == "" {
				continue
			}
			if _, duplicate := seen[root]; duplicate {
				continue
			}
			seen[root] = struct{}{}
			referenceRoots = append(referenceRoots, root)
		}
	}
	return handler.service.DryRun(ctx, application.DryRunCommand{
		RequestKey: uuid.NewString(), SourcePaths: sources, LibraryID: strings.TrimSpace(request.LibraryID),
		Mode: request.Mode, ManagedRoot: managedRoot, ReferenceRoots: referenceRoots,
		HiddenPolicy: request.HiddenPolicy, SymlinkPolicy: request.SymlinkPolicy,
	})
}

func (handler *LibraryImportHandler) GetBatch(ctx context.Context, request application.BatchRequest) (application.BatchDTO, error) {
	return handler.service.GetBatch(ctx, request)
}

func (handler *LibraryImportHandler) ListBatches(ctx context.Context, request application.ListBatchesRequest) ([]application.BatchDTO, error) {
	return handler.service.ListBatches(ctx, request)
}

func (handler *LibraryImportHandler) Commit(ctx context.Context, request application.BatchRequest) (application.BatchDTO, error) {
	return handler.service.Commit(ctx, request)
}

func (handler *LibraryImportHandler) Cancel(ctx context.Context, request application.BatchRequest) (application.BatchDTO, error) {
	return handler.service.Cancel(ctx, request)
}

func (handler *LibraryImportHandler) Resume(ctx context.Context, request application.BatchRequest) (application.BatchDTO, error) {
	return handler.service.Resume(ctx, request)
}
