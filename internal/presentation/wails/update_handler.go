package wails

import (
	"context"
	"time"

	appupdate "xiadown/internal/application/update"
	"xiadown/internal/domain/update"
)

type UpdateHandler struct {
	service *appupdate.Service
	quitter appQuitter
}

type appQuitter interface {
	Quit()
}

func NewUpdateHandler(service *appupdate.Service, quitter appQuitter) *UpdateHandler {
	return &UpdateHandler{service: service, quitter: quitter}
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
	return handler.service.DownloadUpdate(ctx)
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
