package wails

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
	"xiadown/internal/application/settings/dto"
	"xiadown/internal/application/settings/service"
	"xiadown/internal/domain/settings"
	"xiadown/internal/infrastructure/autostart"
	"xiadown/internal/infrastructure/logging"
	"xiadown/internal/infrastructure/proxy"
)

type SettingsHandler struct {
	service   *service.SettingsService
	windows   *WindowManager
	logger    *logging.Logger
	proxy     *proxy.Manager
	autostart autoStartManager
	players   []settingsOnlinePlayerResetter
}

type autoStartManager interface {
	SetEnabled(enabled bool) error
}

type settingsOnlinePlayerResetter interface {
	Reset() error
}

func NewSettingsHandler(service *service.SettingsService, windows *WindowManager, logger *logging.Logger, proxyMgr *proxy.Manager, autostartMgr *autostart.Manager, players ...settingsOnlinePlayerResetter) *SettingsHandler {
	return &SettingsHandler{service: service, windows: windows, logger: logger, proxy: proxyMgr, autostart: autostartMgr, players: players}
}

func (handler *SettingsHandler) ServiceName() string {
	return "SettingsHandler"
}

func (handler *SettingsHandler) GetSettings(ctx context.Context) (dto.Settings, error) {
	return handler.service.GetSettings(ctx)
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

	if handler.proxy != nil {
		config, err := proxyConfigFromDTO(updated.Proxy)
		if err != nil {
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
		zap.L().Info("proxy applied", proxyFields(updated.Proxy)...)
		if proxyChanged {
			handler.resetOnlinePlayersAfterProxyChange("settings-proxy-updated")
		}
	}

	if handler.windows != nil {
		handler.windows.ApplySettings(updated)
	}
	return updated, nil
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
	if err := handler.proxy.Apply(config); err != nil {
		return dto.SystemProxyInfo{}, err
	}
	handler.resetOnlinePlayersAfterProxyChange("settings-system-proxy-refreshed")
	return handler.GetSystemProxy(ctx)
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
