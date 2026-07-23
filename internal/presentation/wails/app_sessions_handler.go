package wails

import (
	"context"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	appsessionsdto "xiadown/internal/application/appsessions/dto"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	"xiadown/internal/application/browserprofile"
	"xiadown/internal/domain/appsessions"
)

type AppSessionsHandler struct {
	service         *appsessionsservice.AppSessionsService
	windows         *WindowManager
	playerResetters []appSessionsOnlinePlayerResetter
}

const ListenYouTubeAppSessionChangedEvent = "listen:youtube-app-session:changed"
const AppSessionsChangedEvent = "app-sessions:changed"

type appSessionsOnlinePlayerResetter interface {
	Reset() error
}

func NewAppSessionsHandler(service *appsessionsservice.AppSessionsService, windows *WindowManager, playerResetters ...appSessionsOnlinePlayerResetter) *AppSessionsHandler {
	handler := &AppSessionsHandler{service: service, windows: windows, playerResetters: playerResetters}
	if service != nil {
		service.SetChangeListener(handler.handleAppSessionChanged)
	}
	return handler
}

func (handler *AppSessionsHandler) ServiceName() string {
	return "AppSessionsHandler"
}

func (handler *AppSessionsHandler) ListAppSessions(ctx context.Context) ([]appsessionsdto.AppSession, error) {
	return handler.service.ListAppSessions(ctx)
}

func (handler *AppSessionsHandler) ClearAppSession(ctx context.Context, request appsessionsdto.ClearAppSessionRequest) error {
	if err := handler.service.ClearAppSession(ctx, request); err != nil {
		return err
	}
	return nil
}

func (handler *AppSessionsHandler) ListBrowserProfileSources(_ context.Context) []browserprofile.Source {
	return browserprofile.ListSources()
}

func (handler *AppSessionsHandler) DiscoverBrowserProfiles(
	_ context.Context,
	request appsessionsdto.BrowserProfileDiscoveryRequest,
) (browserprofile.DiscoveryResult, error) {
	browserID := strings.ToLower(strings.TrimSpace(request.BrowserID))
	if !appsessionsservice.SupportsBrowserProfileImport(browserID) {
		return browserprofile.DiscoveryResult{}, appsessions.ErrUnsupported
	}
	return browserprofile.DiscoverForDomains(
		browserID,
		appsessionsservice.AppSessionBrowserCookieDomains(),
	)
}

// OpenBrowserDataPermissionGuide is the explicit fallback that takes the user
// to macOS Full Disk Access after the narrower App Data prompt was unavailable
// or denied. XiaDown never bypasses that boundary or silently treats a denial
// as an empty profile list.
func (handler *AppSessionsHandler) OpenBrowserDataPermissionGuide(_ context.Context) error {
	return openPermissionGuide(browserDataPermissionGuideRequest("", ""))
}

func (handler *AppSessionsHandler) ScanBrowserProfile(
	ctx context.Context,
	request appsessionsdto.BrowserProfileSelection,
) (appsessionsdto.AppSessionBrowserScanResult, error) {
	return handler.service.ScanBrowserProfile(ctx, request)
}

func (handler *AppSessionsHandler) ImportBrowserProfile(
	ctx context.Context,
	request appsessionsdto.AppSessionBrowserImportRequest,
) (appsessionsdto.AppSessionBrowserImportResult, error) {
	return handler.service.ImportBrowserProfile(ctx, request)
}

func (handler *AppSessionsHandler) StartAppSessionConnect(ctx context.Context, request appsessionsdto.StartAppSessionConnectRequest) (appsessionsdto.StartAppSessionConnectResult, error) {
	resetBeforeStart := handler.appSessionIDHasSite(ctx, request.ID, "youtube")
	if resetBeforeStart {
		handler.resetOnlinePlayer()
	}
	result, err := handler.service.StartAppSessionConnect(ctx, request)
	if err != nil {
		return appsessionsdto.StartAppSessionConnectResult{}, err
	}
	if !resetBeforeStart && result.AppSession.SiteKey == "youtube" {
		handler.resetOnlinePlayer()
	}
	return result, nil
}

func (handler *AppSessionsHandler) FinishAppSessionConnect(ctx context.Context, request appsessionsdto.FinishAppSessionConnectRequest) (appsessionsdto.FinishAppSessionConnectResult, error) {
	result, err := handler.service.FinishAppSessionConnect(ctx, request)
	if err != nil {
		return appsessionsdto.FinishAppSessionConnectResult{}, err
	}
	return result, nil
}

func (handler *AppSessionsHandler) CancelAppSessionConnect(ctx context.Context, request appsessionsdto.CancelAppSessionConnectRequest) error {
	return handler.service.CancelAppSessionConnect(ctx, request)
}

func (handler *AppSessionsHandler) GetAppSessionConnectSession(ctx context.Context, request appsessionsdto.GetAppSessionConnectSessionRequest) (appsessionsdto.AppSessionConnectSession, error) {
	return handler.service.GetAppSessionConnectSession(ctx, request)
}

func (handler *AppSessionsHandler) OpenAppSessionSite(ctx context.Context, request appsessionsdto.OpenAppSessionSiteRequest) (appsessionsdto.StartAppSessionConnectResult, error) {
	return handler.service.OpenAppSessionSite(ctx, request)
}

func (handler *AppSessionsHandler) VerifyAppSession(ctx context.Context, request appsessionsdto.VerifyAppSessionRequest) (appsessionsdto.AppSession, error) {
	return handler.service.VerifyAppSession(ctx, request)
}

func (handler *AppSessionsHandler) appSessionIDHasSite(ctx context.Context, id string, siteKey string) bool {
	id = strings.TrimSpace(id)
	if id == "" || handler == nil || handler.service == nil {
		return false
	}
	items, err := handler.service.ListAppSessions(ctx)
	if err != nil {
		return id == "site-app-session-youtube" && siteKey == "youtube"
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) == id && item.SiteKey == siteKey {
			return true
		}
	}
	return false
}

func (handler *AppSessionsHandler) resetOnlinePlayer() {
	if handler == nil {
		return
	}
	for _, resetter := range handler.playerResetters {
		if resetter != nil {
			_ = resetter.Reset()
		}
	}
}

func (handler *AppSessionsHandler) handleAppSessionChanged(_ context.Context, event appsessionsservice.AppSessionChangeEvent) {
	if handler == nil {
		return
	}
	siteKey := strings.TrimSpace(event.AppSession.SiteKey)
	payload := map[string]any{
		"action":             strings.TrimSpace(event.Action),
		"appSessionId":       strings.TrimSpace(event.AppSession.ID),
		"siteKey":            siteKey,
		"status":             strings.TrimSpace(event.AppSession.Status),
		"verificationStatus": strings.TrimSpace(event.AppSession.AccountVerificationStatus),
		"saved":              event.Saved,
		"reason":             strings.TrimSpace(event.Reason),
	}
	handler.dispatchAppSessionWindowEvent(AppSessionsChangedEvent, payload)
	if siteKey == "youtube" {
		handler.resetOnlinePlayer()
		handler.dispatchAppSessionWindowEvent(ListenYouTubeAppSessionChangedEvent, payload)
	}
}

func (handler *AppSessionsHandler) dispatchAppSessionWindowEvent(name string, payload any) {
	if handler == nil || handler.windows == nil {
		return
	}
	dispatch := func() {
		handler.windows.dispatchWindowEvent(name, payload)
	}
	if handler.windows.canInvokeSync() {
		application.InvokeSync(dispatch)
		return
	}
	dispatch()
}
