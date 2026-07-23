package wails

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/settings/dto"
	"xiadown/internal/application/settings/service"
	"xiadown/internal/application/sniffprofile"
	"xiadown/internal/domain/settings"
	"xiadown/internal/infrastructure/autostart"
	"xiadown/internal/infrastructure/logging"
	"xiadown/internal/infrastructure/opener"
	"xiadown/internal/infrastructure/proxy"
)

type SettingsHandler struct {
	service             *service.SettingsService
	windows             *WindowManager
	logger              *logging.Logger
	proxy               *proxy.Manager
	autostart           autoStartManager
	players             []settingsOnlinePlayerResetter
	downloadScheduler   settingsDownloadScheduler
	sniffProfileEnsurer func(string) (sniffprofile.Manifest, string, error)
	sniffProfileLister  func() ([]sniffprofile.Info, error)
}

type autoStartManager interface {
	SetEnabled(enabled bool) error
}

type settingsOnlinePlayerResetter interface {
	Reset() error
}

type settingsPlaybackAudioQualitySyncer interface {
	SetPlaybackAudioQuality(string) error
}

type settingsDownloadScheduler interface {
	NotifyDownloadScheduler()
}

type settingsSniffProfileActivity interface {
	ActiveResourceSniffProfileIDs() []string
}

func NewSettingsHandler(
	service *service.SettingsService,
	windows *WindowManager,
	logger *logging.Logger,
	proxyMgr *proxy.Manager,
	autostartMgr *autostart.Manager,
	downloadScheduler settingsDownloadScheduler,
	players ...settingsOnlinePlayerResetter,
) *SettingsHandler {
	return &SettingsHandler{
		service:           service,
		windows:           windows,
		logger:            logger,
		proxy:             proxyMgr,
		autostart:         autostartMgr,
		downloadScheduler: downloadScheduler,
		players:           players,
	}
}

func (handler *SettingsHandler) ServiceName() string {
	return "SettingsHandler"
}

func (handler *SettingsHandler) GetSettings(ctx context.Context) (dto.Settings, error) {
	return handler.service.GetSettings(ctx)
}

func (handler *SettingsHandler) GetBrowserCandidates(_ context.Context) ([]browsercdp.Candidate, error) {
	return browsercdp.DetectCandidates(), nil
}

func (handler *SettingsHandler) RefreshBrowserCandidates(_ context.Context) ([]browsercdp.Candidate, error) {
	return browsercdp.RefreshCandidates(), nil
}

func (handler *SettingsHandler) GetSniffProfileInfo(ctx context.Context, request dto.SniffProfileRequest) (dto.SniffProfileInfo, error) {
	if strings.TrimSpace(request.ProfileID) != "" {
		for _, info := range sniffprofile.ExistingProfiles() {
			if info.ProfileID == strings.TrimSpace(request.ProfileID) {
				return sniffProfileInfoDTO(info), nil
			}
		}
		return dto.SniffProfileInfo{}, fmt.Errorf("sniff profile not found")
	}
	browser := handler.resolveSniffProfileBrowser(ctx, request.Browser)
	info := sniffprofile.InfoForPreferredBrowser(browser)
	return sniffProfileInfoDTO(info), nil
}

func (handler *SettingsHandler) ListSniffProfiles(_ context.Context) ([]dto.SniffProfileInfo, error) {
	listProfiles := sniffprofile.ListProfiles
	if handler != nil && handler.sniffProfileLister != nil {
		listProfiles = handler.sniffProfileLister
	}
	profiles, err := listProfiles()
	if err != nil {
		return nil, err
	}
	result := make([]dto.SniffProfileInfo, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, sniffProfileInfoDTO(profile))
	}
	return result, nil
}

func (handler *SettingsHandler) CreateSniffProfile(_ context.Context, request dto.SniffProfileRequest) (dto.SniffProfileInfo, error) {
	// Keep this Wails method for API compatibility, but creating a Sniff Profile
	// now means ensuring the browser's single XiaDown-managed default. The
	// display name is intentionally ignored so callers cannot create additional
	// custom Profiles through the legacy endpoint.
	ensureDefault := sniffprofile.EnsureDefault
	if handler != nil && handler.sniffProfileEnsurer != nil {
		ensureDefault = handler.sniffProfileEnsurer
	}
	manifest, _, err := ensureDefault(request.Browser)
	if err != nil {
		return dto.SniffProfileInfo{}, err
	}
	profiles := sniffprofile.ExistingProfiles()
	if handler != nil && handler.sniffProfileLister != nil {
		profiles, err = handler.sniffProfileLister()
		if err != nil {
			return dto.SniffProfileInfo{}, err
		}
	}
	for _, info := range profiles {
		if info.ProfileID == manifest.ProfileID {
			return sniffProfileInfoDTO(info), nil
		}
	}
	return dto.SniffProfileInfo{}, fmt.Errorf("created sniff profile is unavailable")
}

func (handler *SettingsHandler) RenameSniffProfile(_ context.Context, request dto.SniffProfileRequest) (dto.SniffProfileInfo, error) {
	releaseMutation := sniffprofile.LockForMutation()
	defer releaseMutation()
	if err := handler.ensureSniffProfileIdle(request.ProfileID, request.Browser); err != nil {
		return dto.SniffProfileInfo{}, err
	}
	manifest, err := sniffprofile.Rename(request.ProfileID, request.DisplayName)
	if err != nil {
		return dto.SniffProfileInfo{}, err
	}
	for _, info := range sniffprofile.ExistingProfiles() {
		if info.ProfileID == manifest.ProfileID {
			return sniffProfileInfoDTO(info), nil
		}
	}
	return dto.SniffProfileInfo{}, fmt.Errorf("renamed sniff profile is unavailable")
}

func (handler *SettingsHandler) DeleteSniffProfile(_ context.Context, request dto.SniffProfileRequest) error {
	releaseMutation := sniffprofile.LockForMutation()
	defer releaseMutation()
	if err := handler.ensureSniffProfileIdle(request.ProfileID, request.Browser); err != nil {
		return err
	}
	return sniffprofile.Delete(request.ProfileID)
}

func (handler *SettingsHandler) OpenSniffProfile(ctx context.Context, request dto.SniffProfileRequest) error {
	releaseRead := sniffprofile.LockForRead()
	defer releaseRead()
	var path string
	var err error
	if strings.TrimSpace(request.ProfileID) != "" {
		_, path, err = sniffprofile.Load(request.ProfileID)
	} else {
		browser := handler.resolveSniffProfileBrowser(ctx, request.Browser)
		path, err = sniffprofile.EnsureDirectoryForPreferredBrowser(browser)
	}
	if err != nil {
		return err
	}
	return opener.RevealPath(path)
}

func (handler *SettingsHandler) ClearSniffProfile(ctx context.Context, request dto.SniffProfileRequest) error {
	releaseMutation := sniffprofile.LockForMutation()
	defer releaseMutation()
	if err := handler.ensureSniffProfileIdle(request.ProfileID, request.Browser); err != nil {
		return err
	}
	if strings.TrimSpace(request.ProfileID) != "" {
		return sniffprofile.Clear(request.ProfileID)
	}
	browser := handler.resolveSniffProfileBrowser(ctx, request.Browser)
	return sniffprofile.ClearPreferredBrowser(browser)
}

func (handler *SettingsHandler) ensureSniffProfileIdle(profileID string, browserID string) error {
	activity, ok := handler.downloadScheduler.(settingsSniffProfileActivity)
	if !ok || activity == nil {
		return nil
	}
	active := make(map[string]struct{})
	for _, value := range activity.ActiveResourceSniffProfileIDs() {
		if value = strings.TrimSpace(value); value != "" {
			active[value] = struct{}{}
		}
	}
	if id := strings.TrimSpace(profileID); id != "" {
		if _, found := active[id]; found {
			return fmt.Errorf("sniff profile is active")
		}
		return nil
	}
	resolvedBrowser := sniffprofile.ResolveBrowserID(browserID)
	for _, profile := range sniffprofile.ExistingProfiles() {
		if profile.Browser != resolvedBrowser {
			continue
		}
		if _, found := active[profile.ProfileID]; found {
			return fmt.Errorf("sniff profile is active")
		}
	}
	return nil
}

func sniffProfileInfoDTO(info sniffprofile.Info) dto.SniffProfileInfo {
	return dto.SniffProfileInfo{
		ProfileID:      info.ProfileID,
		DisplayName:    info.DisplayName,
		Browser:        info.Browser,
		IsDefault:      info.IsDefault,
		Redundant:      info.Redundant,
		Exists:         info.Exists,
		SizeBytes:      info.SizeBytes,
		FileCount:      info.FileCount,
		DirectoryCount: info.DirectoryCount,
		LastUsedAt:     info.LastUsedAt,
		Truncated:      info.Truncated,
		Error:          info.Error,
	}
}

func (handler *SettingsHandler) UpdateSettings(ctx context.Context, request dto.UpdateSettingsRequest) (dto.Settings, error) {
	var previousSettings dto.Settings
	var hasPrevious bool
	if current, err := handler.service.GetSettings(ctx); err == nil {
		previousSettings = current
		hasPrevious = true
	}

	updated, err := handler.service.UpdateSettings(ctx, request)
	if err != nil {
		return dto.Settings{}, err
	}
	proxyChanged := request.Proxy != nil
	if proxyChanged && hasPrevious {
		proxyChanged = proxyNetworkConfigChanged(previousSettings.Proxy, updated.Proxy)
	}

	if request.AutoStart != nil && handler.autostart == nil {
		if hasPrevious {
			handler.rollbackSettings(ctx, previousSettings)
		}
		return dto.Settings{}, fmt.Errorf("autostart manager unavailable")
	}
	if request.AutoStart != nil {
		if err := handler.autostart.SetEnabled(updated.AutoStart); err != nil {
			if hasPrevious {
				handler.rollbackSettings(ctx, previousSettings)
			}
			return dto.Settings{}, err
		}
	}

	if handler.logger != nil {
		if err := handler.logger.SetLevel(settings.LogLevel(updated.LogLevel)); err != nil {
			if hasPrevious {
				handler.rollbackSettings(ctx, previousSettings)
			}
			return dto.Settings{}, err
		}
	}

	if request.PlaybackAudioQuality != nil {
		for _, player := range handler.players {
			if syncer, ok := player.(settingsPlaybackAudioQualitySyncer); ok {
				if err := syncer.SetPlaybackAudioQuality(updated.PlaybackAudioQuality); err != nil {
					if hasPrevious {
						handler.rollbackSettings(ctx, previousSettings)
					}
					return dto.Settings{}, err
				}
			}
		}
	}

	// Publishing a network generation is the final fallible settings side
	// effect. A non-network settings save must not tear down app-managed
	// connections, and a later failure must never leave the persisted DTO on the
	// old proxy while the live backend/helper gateway uses the new one. Native
	// WebViews keep their independent platform/runtime network policy.
	if handler.proxy != nil && proxyChanged {
		config, err := proxyConfigFromDTO(updated.Proxy)
		if err != nil {
			if hasPrevious {
				handler.rollbackSettings(ctx, previousSettings)
			}
			return dto.Settings{}, err
		}
		if err := handler.proxy.Apply(config); err != nil {
			zap.L().Error("apply proxy failed", append(proxyFields(updated.Proxy), zap.Error(err))...)
			if hasPrevious {
				handler.rollbackSettings(ctx, previousSettings)
				if handler.logger != nil {
					_ = handler.logger.SetLevel(settings.LogLevel(previousSettings.LogLevel))
				}
			}
			return dto.Settings{}, err
		}
		zap.L().Info("network policy applied", append(proxyFields(updated.Proxy), handler.networkRouteFields()...)...)
		handler.resetOnlinePlayersAfterProxyChange("settings-proxy-updated")
	}

	if handler.windows != nil {
		handler.windows.ApplySettings(updated)
	}
	if request.YTDLPConcurrentDownloads != nil && handler.downloadScheduler != nil {
		handler.downloadScheduler.NotifyDownloadScheduler()
	}
	return updated, nil
}

func (handler *SettingsHandler) resolveSniffProfileBrowser(_ context.Context, requested string) string {
	// Browser selection is operation-scoped. The persisted SniffBrowser field is
	// retained only so existing databases can open; it is never consulted here.
	return strings.TrimSpace(requested)
}

func (handler *SettingsHandler) ShowSettingsWindow() {
	handler.windows.ShowSettingsWindow()
}

func (handler *SettingsHandler) ShowMainWindow() {
	if handler == nil || handler.windows == nil {
		return
	}
	handler.windows.ShowMainWindow()
}

// MarkMainWindowBootReady is called after React has committed its stable first
// frame. The native startup overlay may already have made the window visible.
func (handler *SettingsHandler) MarkMainWindowBootReady() {
	if handler == nil || handler.windows == nil {
		return
	}
	handler.windows.MarkMainWindowBootReady()
}

// MarkMainWindowBootFailed exposes the inline HTML recovery surface after an
// explicit frontend startup failure. WindowManager verifies that navigation
// completed before removing the native overlay.
func (handler *SettingsHandler) MarkMainWindowBootFailed() {
	if handler == nil || handler.windows == nil {
		return
	}
	handler.windows.ReleaseMainWindowBootFallback()
}

func (handler *SettingsHandler) HideSettingsWindow() {
	handler.windows.HideSettingsWindow()
}

func (handler *SettingsHandler) SetWelcomeWindowChromeHidden(hidden bool) {
	if handler == nil || handler.windows == nil {
		return
	}
	handler.windows.SetMainWindowChromeHidden(hidden)
}

func (handler *SettingsHandler) OpenLogDirectory(_ context.Context) error {
	if handler.logger == nil {
		return nil
	}
	return logging.OpenLogDir(handler.logger.LogDir())
}

func (handler *SettingsHandler) SelectDownloadDirectory(ctx context.Context, title string) (string, error) {
	if handler.windows == nil {
		return "", fmt.Errorf("window manager not available")
	}
	normalizedTitle := strings.TrimSpace(title)
	current, err := handler.service.GetSettings(ctx)
	if err != nil {
		return "", err
	}
	initialDir := strings.TrimSpace(current.DownloadDirectory)
	if initialDir == "" {
		initialDir = settings.DefaultDownloadDirectory()
	}
	return handler.windows.SelectDirectoryDialog(normalizedTitle, initialDir)
}

func (handler *SettingsHandler) SelectDirectory(_ context.Context, title string, initialDir string) (string, error) {
	if handler.windows == nil {
		return "", fmt.Errorf("window manager not available")
	}
	normalizedTitle := strings.TrimSpace(title)
	normalizedInitialDir := strings.TrimSpace(initialDir)
	if normalizedInitialDir == "" {
		normalizedInitialDir = settings.DefaultDownloadDirectory()
	}
	return handler.windows.SelectDirectoryDialog(normalizedTitle, normalizedInitialDir)
}

func (handler *SettingsHandler) TestProxy(ctx context.Context, request dto.Proxy) (dto.Proxy, error) {
	if handler.proxy == nil {
		return request, fmt.Errorf("proxy manager not available")
	}

	config, err := proxyConfigFromDTO(request)
	if err != nil {
		return dto.Proxy{}, err
	}

	zap.L().Info("proxy test requested", proxyFields(request)...)
	result, err := handler.proxy.Test(ctx, config)
	if err != nil {
		zap.L().Error("proxy test error", append(proxyFields(request), zap.Error(err))...)
		return dto.Proxy{}, err
	}

	request.TestSuccess = result.Success
	request.TestMessage = result.Message
	request.TestedAt = result.TestedAt.Format(time.RFC3339)
	if result.Success {
		zap.L().Info("proxy test succeeded", proxyFields(request)...)
	} else {
		zap.L().Warn("proxy test failed", proxyFields(request)...)
	}
	return request, nil
}

func (handler *SettingsHandler) GetSystemProxy(_ context.Context) (dto.SystemProxyInfo, error) {
	if handler.proxy == nil {
		return dto.SystemProxyInfo{}, nil
	}
	info, err := handler.proxy.ResolveSystemProxyInfo("")
	if err != nil {
		return dto.SystemProxyInfo{}, err
	}
	source := string(info.Source)
	if source == "" {
		source = string(proxy.SystemProxySourceSystem)
	}
	return dto.SystemProxyInfo{
		Address: info.Address,
		Source:  source,
		Name:    info.Name,
	}, nil
}

func (handler *SettingsHandler) RefreshSystemProxy(ctx context.Context) (dto.SystemProxyInfo, error) {
	if handler.proxy == nil {
		return dto.SystemProxyInfo{}, nil
	}
	current, err := handler.service.GetSettings(ctx)
	if err != nil {
		return dto.SystemProxyInfo{}, err
	}
	config, err := proxyConfigFromDTO(current.Proxy)
	if err != nil {
		return dto.SystemProxyInfo{}, err
	}
	if config.Mode == settings.ProxyModeSystem {
		if err := handler.proxy.Apply(config); err != nil {
			return dto.SystemProxyInfo{}, err
		}
		zap.L().Info("system network policy refreshed", handler.networkRouteFields()...)
		handler.resetOnlinePlayersAfterProxyChange("settings-system-proxy-refreshed")
	}
	return handler.GetSystemProxy(ctx)
}

func (handler *SettingsHandler) networkRouteFields() []zap.Field {
	if handler == nil || handler.proxy == nil {
		return nil
	}
	return []zap.Field{
		zap.Uint64("networkGeneration", handler.proxy.Generation()),
		zap.String("networkGateway", handler.proxy.GatewayURL()),
	}
}

func proxyConfigFromDTO(proxyDTO dto.Proxy) (proxy.Config, error) {
	noProxy := proxyDTO.NoProxy
	if noProxy == nil {
		noProxy = []string{}
	}
	timeout := time.Duration(proxyDTO.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = settings.DefaultProxyTimeoutSeconds * time.Second
	}
	mode := proxyDTO.Mode
	if mode == "" {
		mode = settings.ProxyModeNone.String()
	}
	scheme := proxyDTO.Scheme
	if scheme == "" {
		scheme = settings.ProxySchemeHTTP.String()
	}

	proxySettings, err := settings.NewProxySettings(settings.ProxySettingsParams{
		Mode:           mode,
		Scheme:         scheme,
		Host:           proxyDTO.Host,
		Port:           proxyDTO.Port,
		Username:       proxyDTO.Username,
		Password:       proxyDTO.Password,
		NoProxy:        noProxy,
		TimeoutSeconds: int(timeout.Seconds()),
	})
	if err != nil {
		return proxy.Config{}, err
	}

	return proxy.ConfigFromSettings(proxySettings), nil
}

func proxyFields(proxyDTO dto.Proxy) []zap.Field {
	return []zap.Field{
		zap.String("mode", proxyDTO.Mode),
		zap.String("scheme", proxyDTO.Scheme),
		zap.String("host", proxyDTO.Host),
		zap.Int("port", proxyDTO.Port),
		zap.Bool("testSuccess", proxyDTO.TestSuccess),
		zap.String("testMessage", proxyDTO.TestMessage),
		zap.String("testedAt", proxyDTO.TestedAt),
	}
}

func proxyNetworkConfigChanged(previous dto.Proxy, current dto.Proxy) bool {
	return previous.Mode != current.Mode ||
		previous.Scheme != current.Scheme ||
		previous.Host != current.Host ||
		previous.Port != current.Port ||
		previous.Username != current.Username ||
		previous.Password != current.Password ||
		previous.TimeoutSeconds != current.TimeoutSeconds ||
		!slices.Equal(previous.NoProxy, current.NoProxy)
}

func (handler *SettingsHandler) resetOnlinePlayersAfterProxyChange(reason string) {
	if handler == nil {
		return
	}
	for _, player := range handler.players {
		if player == nil {
			continue
		}
		if err := player.Reset(); err != nil {
			zap.L().Warn("reset online player after proxy change failed", zap.String("reason", reason), zap.Error(err))
		}
	}
}

func (handler *SettingsHandler) rollbackSettings(ctx context.Context, previous dto.Settings) {
	_, err := handler.service.UpdateSettings(ctx, dto.UpdateSettingsRequest{
		Appearance:            &previous.Appearance,
		FontFamily:            &previous.FontFamily,
		FontSize:              &previous.FontSize,
		ThemeColor:            &previous.ThemeColor,
		ColorScheme:           &previous.ColorScheme,
		Language:              &previous.Language,
		SniffBrowser:          &previous.SniffBrowser,
		DownloadDirectory:     &previous.DownloadDirectory,
		MainBounds:            &previous.MainBounds,
		SettingsBounds:        &previous.SettingsBounds,
		LogLevel:              &previous.LogLevel,
		LogMaxSizeMB:          &previous.LogMaxSizeMB,
		LogMaxBackups:         &previous.LogMaxBackups,
		LogMaxAgeDays:         &previous.LogMaxAgeDays,
		LogCompress:           &previous.LogCompress,
		MenuBarVisibility:     &previous.MenuBarVisibility,
		AutoStart:             &previous.AutoStart,
		MinimizeToTrayOnStart: &previous.MinimizeToTrayOnStart,
		Proxy: &dto.Proxy{
			Mode:           previous.Proxy.Mode,
			Scheme:         previous.Proxy.Scheme,
			Host:           previous.Proxy.Host,
			Port:           previous.Proxy.Port,
			Username:       previous.Proxy.Username,
			Password:       previous.Proxy.Password,
			NoProxy:        previous.Proxy.NoProxy,
			TimeoutSeconds: previous.Proxy.TimeoutSeconds,
			TestedAt:       previous.Proxy.TestedAt,
			TestSuccess:    previous.Proxy.TestSuccess,
			TestMessage:    previous.Proxy.TestMessage,
		},
	})
	if err != nil {
		zap.L().Error("rollback settings failed", zap.Error(err))
	}
}
