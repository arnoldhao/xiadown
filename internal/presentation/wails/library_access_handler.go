package wails

import (
	"context"

	libraryaccess "xiadown/internal/application/libraryaccess"
)

type LibraryAccessHandler struct {
	service           *libraryaccess.Service
	localPortProvider func() int
}

type LibraryAccessUpdateResult struct {
	Config libraryaccess.Config `json:"config"`
	Status libraryaccess.Status `json:"status"`
}

func NewLibraryAccessHandler(service *libraryaccess.Service, localPortProvider func() int) *LibraryAccessHandler {
	return &LibraryAccessHandler{service: service, localPortProvider: localPortProvider}
}

func (handler *LibraryAccessHandler) ServiceName() string {
	return "LibraryAccessHandler"
}

func (handler *LibraryAccessHandler) GetLibraryAccessConfig(ctx context.Context) (libraryaccess.Config, error) {
	return handler.service.GetConfig(ctx)
}

func (handler *LibraryAccessHandler) GetLibraryAccessStatus(ctx context.Context) (libraryaccess.Status, error) {
	return handler.service.GetStatus(ctx)
}

func (handler *LibraryAccessHandler) UpdateLibraryAccessConfig(
	ctx context.Context,
	request libraryaccess.UpdateConfigRequest,
) (LibraryAccessUpdateResult, error) {
	config, err := handler.service.UpdateConfig(ctx, request)
	if err != nil {
		return LibraryAccessUpdateResult{}, err
	}
	status, err := handler.service.Apply(ctx, handler.localPort())
	if err != nil {
		return LibraryAccessUpdateResult{}, err
	}
	return LibraryAccessUpdateResult{Config: config, Status: status}, nil
}

func (handler *LibraryAccessHandler) ReconcileLibraryAccess(ctx context.Context) (libraryaccess.Status, error) {
	return handler.service.Apply(ctx, handler.localPort())
}

func (handler *LibraryAccessHandler) localPort() int {
	if handler == nil || handler.localPortProvider == nil {
		return 0
	}
	return handler.localPortProvider()
}
