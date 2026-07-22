package wails

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/application/library/service"
)

// CatalogHandler is intentionally separate from the legacy LibraryHandler so
// desktop bindings and the future public API can evolve around logical items
// without widening the old path-oriented surface.
type CatalogHandler struct {
	service *service.CatalogService
	windows *WindowManager
}

var ErrCatalogStorageRootSelectionCancelled = errors.New("catalog storage root selection cancelled")

func NewCatalogHandler(service *service.CatalogService, windows ...*WindowManager) *CatalogHandler {
	var manager *WindowManager
	if len(windows) > 0 {
		manager = windows[0]
	}
	return &CatalogHandler{service: service, windows: manager}
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

func (handler *CatalogHandler) SaveCatalogStorageRoot(ctx context.Context, request dto.SaveCatalogStorageRootRequest) (dto.CatalogStorageRootDTO, error) {
	return handler.service.SaveCatalogStorageRoot(ctx, request)
}

// SelectAndSaveCatalogStorageRoot keeps paths out of the webview request. The
// trusted desktop UI supplies only descriptive policy; the native picker owns
// selection of the absolute directory passed to the application service.
func (handler *CatalogHandler) SelectAndSaveCatalogStorageRoot(
	ctx context.Context,
	request SelectCatalogStorageRootRequest,
) (dto.CatalogStorageRootDTO, error) {
	if handler == nil || handler.service == nil || handler.windows == nil {
		return dto.CatalogStorageRootDTO{}, fmt.Errorf("catalog storage root picker is not configured")
	}
	path, err := handler.windows.SelectMainDirectoryDialog("Select Library Storage Location", "")
	if err != nil {
		return dto.CatalogStorageRootDTO{}, err
	}
	if strings.TrimSpace(path) == "" {
		return dto.CatalogStorageRootDTO{}, ErrCatalogStorageRootSelectionCancelled
	}
	return handler.service.SaveCatalogStorageRoot(ctx, dto.SaveCatalogStorageRootRequest{
		Name: strings.TrimSpace(request.Name), Path: path, Mode: strings.TrimSpace(request.Mode),
	})
}

type SelectCatalogStorageRootRequest struct {
	Name string `json:"name"`
	Mode string `json:"mode,omitempty"`
}

func (handler *CatalogHandler) CheckCatalogStorageRoot(ctx context.Context, request dto.CheckCatalogStorageRootRequest) (dto.CatalogStorageRootDTO, error) {
	return handler.service.CheckCatalogStorageRoot(ctx, request)
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
