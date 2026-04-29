package wails

import (
	"context"
	"time"

	appupdate "xiadown/internal/application/update"
	"xiadown/internal/domain/update"
)

type UpdateHandler struct {
	service   *appupdate.Service
	telemetry updateTelemetry
	quitter   appQuitter
}

type updateTelemetry interface {
	TrackUpdateReadyToRestart(ctx context.Context, latestVersion string)
}

type appQuitter interface {
	Quit()
}

func NewUpdateHandler(service *appupdate.Service, telemetry updateTelemetry, quitter appQuitter) *UpdateHandler {
	return &UpdateHandler{service: service, telemetry: telemetry, quitter: quitter}
}

func (handler *UpdateHandler) ServiceName() string {
	return "UpdateHandler"
}

func (handler *UpdateHandler) GetState(_ context.Context) update.Info {
	return handler.service.State()
}

func (handler *UpdateHandler) GetWhatsNew(ctx context.Context) (update.WhatsNew, error) {
	return handler.service.GetWhatsNew(ctx)
}

func (handler *UpdateHandler) CheckForUpdate(ctx context.Context, currentVersion string) (update.Info, error) {
	return handler.service.CheckForUpdate(ctx, currentVersion)
}

func (handler *UpdateHandler) DownloadUpdate(ctx context.Context) (update.Info, error) {
	info, err := handler.service.DownloadUpdate(ctx)
	if err == nil && handler.telemetry != nil && info.Status == update.StatusReadyToRestart {
		latestVersion := info.PreparedVersion
		if latestVersion == "" {
			latestVersion = info.LatestVersion
		}
		handler.telemetry.TrackUpdateReadyToRestart(ctx, latestVersion)
	}
	return info, err
}

func (handler *UpdateHandler) RestartToApply(ctx context.Context) (update.Info, error) {
	info, err := handler.service.RestartToApply(ctx)
	if err == nil && handler.quitter != nil {
		go func() {
			time.Sleep(150 * time.Millisecond)
			handler.quitter.Quit()
		}()
	}
	return info, err
}

func (handler *UpdateHandler) DismissWhatsNew(ctx context.Context, version string) error {
	return handler.service.DismissWhatsNew(ctx, version)
}
