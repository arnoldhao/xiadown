package wails

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/application/library/service"
	"xiadown/internal/application/libraryrootsync"
	"xiadown/internal/infrastructure/opener"
)

// CatalogHandler is intentionally separate from the legacy LibraryHandler so
// desktop bindings and the future public API can evolve around logical items
// without widening the old path-oriented surface.
type CatalogHandler struct {
	service *service.CatalogService
	windows *WindowManager
	syncer  CatalogStorageRootSyncer
}

type CatalogStorageRootSyncer interface {
	ListStates(context.Context) ([]libraryrootsync.StateDTO, error)
	StartRootScan(
		context.Context,
		libraryrootsync.RootRequest,
	) (libraryrootsync.StateDTO, error)
	CancelRootScan(
		context.Context,
		libraryrootsync.RootRequest,
	) (libraryrootsync.StateDTO, error)
	StopRoot(context.Context, string) error
}

func NewCatalogHandler(
	service *service.CatalogService,
	syncer CatalogStorageRootSyncer,
	windows ...*WindowManager,
) *CatalogHandler {
	var manager *WindowManager
	if len(windows) > 0 {
		manager = windows[0]
	}
	return &CatalogHandler{service: service, windows: manager, syncer: syncer}
}

func startCatalogStorageRootScan(
	syncer CatalogStorageRootSyncer,
	rootID string,
) {
	if syncer == nil || strings.TrimSpace(rootID) == "" {
		return
	}
	_, _ = syncer.StartRootScan(
		context.Background(),
		libraryrootsync.RootRequest{RootID: rootID},
	)
}

func runCatalogStorageRootRelocation(
	ctx context.Context,
	syncer CatalogStorageRootSyncer,
	rootID string,
	relocate func() (dto.CatalogStorageRootDTO, error),
) (dto.CatalogStorageRootDTO, error) {
	if syncer == nil {
		return relocate()
	}
	if err := syncer.StopRoot(ctx, rootID); err != nil {
		// StopRoot removes the root from the syncer's live set before it waits.
		// Restore it even when the wait is interrupted so periodic sync is not
		// required to make the old root live again.
		startCatalogStorageRootScan(syncer, rootID)
		return dto.CatalogStorageRootDTO{}, err
	}
	root, err := relocate()
	restartID := rootID
	if strings.TrimSpace(root.ID) != "" {
		restartID = root.ID
	}
	startCatalogStorageRootScan(syncer, restartID)
	return root, err
}

func (handler *CatalogHandler) ServiceName() string { return "CatalogHandler" }

func (handler *CatalogHandler) GetDefaultCatalogOverview(ctx context.Context) (dto.CatalogOverviewDTO, error) {
	return handler.service.GetDefaultCatalogOverview(ctx)
}

func (handler *CatalogHandler) ListCatalogItems(ctx context.Context, request dto.ListCatalogItemsRequest) (dto.ListCatalogItemsResponse, error) {
	return handler.service.ListCatalogItems(ctx, request)
}

func (handler *CatalogHandler) GetCatalogItem(ctx context.Context, request dto.GetCatalogItemRequest) (dto.CatalogItemDetailDTO, error) {
	return handler.service.GetCatalogItem(ctx, request)
}

func (handler *CatalogHandler) ListCatalogItemActivity(ctx context.Context, request dto.ListCatalogItemActivityRequest) ([]dto.CatalogItemActivityDTO, error) {
	return handler.service.ListCatalogItemActivity(ctx, request)
}

func (handler *CatalogHandler) ListCatalogRepresentations(ctx context.Context, request dto.ListCatalogRepresentationsRequest) ([]dto.CatalogRepresentationDTO, error) {
	return handler.service.ListCatalogRepresentations(ctx, request)
}

func (handler *CatalogHandler) SaveCatalogRepresentation(ctx context.Context, request dto.SaveCatalogRepresentationRequest) (dto.CatalogRepresentationDTO, error) {
	return handler.service.SaveCatalogRepresentation(ctx, request)
}

func (handler *CatalogHandler) ListCatalogMetadataEntries(ctx context.Context, request dto.ListCatalogMetadataEntriesRequest) ([]dto.CatalogMetadataEntryDTO, error) {
	return handler.service.ListCatalogMetadataEntries(ctx, request)
}

func (handler *CatalogHandler) SaveCatalogMetadataEntry(ctx context.Context, request dto.SaveCatalogMetadataEntryRequest) (dto.CatalogMetadataEntryDTO, error) {
	return handler.service.SaveCatalogMetadataEntry(ctx, request)
}

func (handler *CatalogHandler) UpdateCatalogItem(ctx context.Context, request dto.UpdateCatalogItemRequest) (dto.CatalogItemDetailDTO, error) {
	return handler.service.UpdateCatalogItem(ctx, request)
}

func (handler *CatalogHandler) TrashCatalogItem(ctx context.Context, request dto.CatalogItemLifecycleRequest) (dto.CatalogItemDetailDTO, error) {
	return handler.service.TrashCatalogItem(ctx, request)
}

func (handler *CatalogHandler) RestoreCatalogItem(ctx context.Context, request dto.CatalogItemLifecycleRequest) (dto.CatalogItemDetailDTO, error) {
	return handler.service.RestoreCatalogItem(ctx, request)
}

func (handler *CatalogHandler) ListCatalogCollections(ctx context.Context) ([]dto.CatalogCollectionDTO, error) {
	return handler.service.ListCatalogCollections(ctx)
}

func (handler *CatalogHandler) SaveCatalogCollection(ctx context.Context, request dto.SaveCatalogCollectionRequest) (dto.CatalogCollectionDTO, error) {
	return handler.service.SaveCatalogCollection(ctx, request)
}

func (handler *CatalogHandler) ReplaceCatalogCollectionItems(ctx context.Context, request dto.ReplaceCatalogCollectionItemsRequest) (dto.CatalogCollectionDTO, error) {
	return handler.service.ReplaceCatalogCollectionItems(ctx, request)
}

func (handler *CatalogHandler) ListCatalogTags(ctx context.Context) ([]dto.CatalogTagDTO, error) {
	return handler.service.ListCatalogTags(ctx)
}

func (handler *CatalogHandler) SaveCatalogTag(ctx context.Context, request dto.SaveCatalogTagRequest) (dto.CatalogTagDTO, error) {
	return handler.service.SaveCatalogTag(ctx, request)
}

func (handler *CatalogHandler) ReplaceCatalogItemTags(ctx context.Context, request dto.ReplaceCatalogItemTagsRequest) ([]dto.CatalogTagDTO, error) {
	return handler.service.ReplaceCatalogItemTags(ctx, request)
}

func (handler *CatalogHandler) ListCatalogStorageRoots(ctx context.Context) ([]dto.CatalogStorageRootDTO, error) {
	return handler.service.ListCatalogStorageRoots(ctx)
}

func (handler *CatalogHandler) ListCatalogStorageVolumes(ctx context.Context) ([]dto.CatalogStorageVolumeDTO, error) {
	return handler.service.ListCatalogStorageVolumes(ctx)
}

//wails:ignore
func (handler *CatalogHandler) SaveCatalogStorageRoot(ctx context.Context, request dto.SaveCatalogStorageRootRequest) (dto.CatalogStorageRootDTO, error) {
	return handler.service.SaveCatalogStorageRoot(ctx, request)
}

// SelectAndSaveCatalogStorageRoot keeps paths and storage policy out of the
// webview request. Additional roots are always references; the default
// download directory is the only root created as managed by this surface.
func (handler *CatalogHandler) SelectAndSaveCatalogStorageRoot(
	ctx context.Context,
	_ SelectCatalogStorageRootRequest,
) (*dto.CatalogStorageRootDTO, error) {
	if handler == nil || handler.service == nil || handler.windows == nil {
		return nil, fmt.Errorf("catalog storage root picker is not configured")
	}
	path, err := handler.windows.SelectMainDirectoryDialog("Select Library Storage Location", "")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) || strings.TrimSpace(name) == "" {
		name = "Referenced Files"
	}
	root, err := handler.service.SaveCatalogStorageRoot(ctx, dto.SaveCatalogStorageRootRequest{
		Name: name, Path: path, Mode: "referenced",
	})
	if err != nil {
		return nil, err
	}
	startCatalogStorageRootScan(handler.syncer, root.ID)
	return &root, nil
}

type SelectCatalogStorageRootRequest struct{}

func (handler *CatalogHandler) CheckCatalogStorageRoot(ctx context.Context, request dto.CheckCatalogStorageRootRequest) (dto.CatalogStorageRootDTO, error) {
	return handler.service.CheckCatalogStorageRoot(ctx, request)
}

func (handler *CatalogHandler) UpdateCatalogStorageRoot(
	ctx context.Context,
	request dto.UpdateCatalogStorageRootRequest,
) (dto.CatalogStorageRootDTO, error) {
	return handler.service.UpdateCatalogStorageRoot(ctx, request)
}

func (handler *CatalogHandler) SetDefaultCatalogStorageRoot(
	ctx context.Context,
	request dto.CatalogStorageRootIDRequest,
) (dto.CatalogStorageRootDTO, error) {
	return handler.service.SetDefaultCatalogStorageRoot(ctx, request)
}

func (handler *CatalogHandler) RemoveCatalogStorageRoot(
	ctx context.Context,
	request dto.CatalogStorageRootIDRequest,
) error {
	current, err := handler.service.GetCatalogStorageRoot(ctx, request)
	if err != nil {
		return err
	}
	if current.IsDefault || handler.syncer == nil {
		return handler.service.RemoveCatalogStorageRoot(ctx, request)
	}
	if err := handler.syncer.StopRoot(ctx, request.ID); err != nil {
		startCatalogStorageRootScan(handler.syncer, request.ID)
		return err
	}
	if err := handler.service.RemoveCatalogStorageRoot(ctx, request); err != nil {
		startCatalogStorageRootScan(handler.syncer, request.ID)
		return err
	}
	return nil
}

func (handler *CatalogHandler) ListCatalogStorageRootSyncStates(
	ctx context.Context,
) ([]libraryrootsync.StateDTO, error) {
	if handler == nil || handler.syncer == nil {
		return []libraryrootsync.StateDTO{}, nil
	}
	return handler.syncer.ListStates(ctx)
}

func (handler *CatalogHandler) StartCatalogStorageRootScan(
	ctx context.Context,
	request libraryrootsync.RootRequest,
) (libraryrootsync.StateDTO, error) {
	if handler == nil || handler.syncer == nil {
		return libraryrootsync.StateDTO{},
			fmt.Errorf("catalog storage root sync is not configured")
	}
	return handler.syncer.StartRootScan(ctx, request)
}

func (handler *CatalogHandler) CancelCatalogStorageRootScan(
	ctx context.Context,
	request libraryrootsync.RootRequest,
) (libraryrootsync.StateDTO, error) {
	if handler == nil || handler.syncer == nil {
		return libraryrootsync.StateDTO{},
			fmt.Errorf("catalog storage root sync is not configured")
	}
	return handler.syncer.CancelRootScan(ctx, request)
}

func (handler *CatalogHandler) SelectAndRelocateCatalogStorageRoot(
	ctx context.Context,
	request dto.CatalogStorageRootIDRequest,
) (*dto.CatalogStorageRootDTO, error) {
	if handler == nil || handler.service == nil || handler.windows == nil {
		return nil, fmt.Errorf("catalog storage root picker is not configured")
	}
	current, err := handler.service.GetCatalogStorageRoot(ctx, request)
	if err != nil {
		return nil, err
	}
	initialPath := current.LocationPath
	if strings.TrimSpace(initialPath) == "" {
		initialPath = current.Path
	}
	path, err := handler.windows.SelectMainDirectoryDialog("Relocate Library Storage", initialPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	root, err := runCatalogStorageRootRelocation(
		ctx,
		handler.syncer,
		request.ID,
		func() (dto.CatalogStorageRootDTO, error) {
			return handler.service.RelocateCatalogStorageRoot(
				ctx,
				dto.RelocateCatalogStorageRootRequest{
					ID: request.ID, Path: path,
				},
			)
		},
	)
	if err != nil {
		return nil, err
	}
	return &root, nil
}

func (handler *CatalogHandler) OpenCatalogStorageRoot(
	ctx context.Context,
	request dto.CatalogStorageRootIDRequest,
) error {
	current, err := handler.service.GetCatalogStorageRoot(ctx, request)
	if err != nil {
		return err
	}
	path := current.LocationPath
	if strings.TrimSpace(path) == "" {
		path = current.Path
	}
	return opener.OpenDirectory(path)
}

func (handler *CatalogHandler) GetCatalogUserState(ctx context.Context, request dto.GetCatalogUserStateRequest) (dto.CatalogUserStateDTO, error) {
	return handler.service.GetCatalogUserState(ctx, request)
}

func (handler *CatalogHandler) UpdateCatalogUserState(ctx context.Context, request dto.UpdateCatalogUserStateRequest) (dto.CatalogUserStateDTO, error) {
	return handler.service.UpdateCatalogUserState(ctx, request)
}

func (handler *CatalogHandler) GetCatalogMigrationAudit(ctx context.Context, request dto.CatalogMigrationAuditRequest) (dto.CatalogMigrationAuditDTO, error) {
	return handler.service.GetCatalogMigrationAudit(ctx, request)
}
