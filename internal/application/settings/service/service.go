package service

import (
	"context"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/settings"
)

type ThemeProvider interface {
	IsDarkMode(ctx context.Context) (bool, error)
	AccentColor(ctx context.Context) (string, error)
}

type SettingsService struct {
	repo          settings.Repository
	themeProvider ThemeProvider
	defaults      settings.Settings
}

func NewSettingsService(repo settings.Repository, themeProvider ThemeProvider, defaults settings.Settings) *SettingsService {
	return &SettingsService{
		repo:          repo,
		themeProvider: themeProvider,
		defaults:      defaults,
	}
}

func (service *SettingsService) GetSettings(ctx context.Context) (dto.Settings, error) {
	current, err := service.repo.Get(ctx)
	if err != nil {
		if err != settings.ErrSettingsNotFound {
			return dto.Settings{}, err
		}
		current = service.defaults
		if err := service.repo.Save(ctx, current); err != nil {
			return dto.Settings{}, err
		}
	}

	effectiveAppearance, err := service.resolveAppearance(ctx, current.Appearance())
	if err != nil {
		return dto.Settings{}, err
	}

	return toDTO(current, effectiveAppearance, service.resolveSystemThemeColor(ctx)), nil
}

func (service *SettingsService) UpdateSettings(ctx context.Context, request dto.UpdateSettingsRequest) (dto.Settings, error) {
	current, err := service.repo.Get(ctx)
	if err != nil {
		if err != settings.ErrSettingsNotFound {
			return dto.Settings{}, err
		}
		current = service.defaults
	}

	appearance := current.Appearance().String()
	if request.Appearance != nil {
		appearance = *request.Appearance
	}

	fontFamily := current.FontFamily()
	if request.FontFamily != nil {
		fontFamily = *request.FontFamily
	}

	themeColor := current.ThemeColor()
	if request.ThemeColor != nil {
		themeColor = *request.ThemeColor
	}

	colorScheme := current.ColorScheme().String()
	if request.ColorScheme != nil {
		colorScheme = *request.ColorScheme
	}

	language := current.Language().String()
	if request.Language != nil {
		language = *request.Language
	}

	downloadDirectory := current.DownloadDirectory()
	if request.DownloadDirectory != nil {
		downloadDirectory = strings.TrimSpace(*request.DownloadDirectory)
	}

	fontSize := current.FontSize()
	if request.FontSize != nil {
		fontSize = *request.FontSize
	}

	logLevel := current.LogLevel().String()
	if request.LogLevel != nil {
		logLevel = *request.LogLevel
	}

	logMaxSizeMB := current.LogMaxSizeMB()
	if request.LogMaxSizeMB != nil {
		logMaxSizeMB = *request.LogMaxSizeMB
	}

	logMaxBackups := current.LogMaxBackups()
	if request.LogMaxBackups != nil {
		logMaxBackups = *request.LogMaxBackups
	}

	logMaxAgeDays := current.LogMaxAgeDays()
	if request.LogMaxAgeDays != nil {
		logMaxAgeDays = *request.LogMaxAgeDays
	}

	logCompress := current.LogCompress()
	if request.LogCompress != nil {
		logCompress = *request.LogCompress
	}

	menuBarVisibility := sanitizeMenuBarVisibility(current.MenuBarVisibility().String())
	if request.MenuBarVisibility != nil {
		menuBarVisibility = *request.MenuBarVisibility
	}
	menuBarVisibility = sanitizeMenuBarVisibility(menuBarVisibility)

	autoStart := current.AutoStart()
	if request.AutoStart != nil {
		autoStart = *request.AutoStart
	}

	minimizeToTrayOnStart := current.MinimizeToTrayOnStart()
	if request.MinimizeToTrayOnStart != nil {
		minimizeToTrayOnStart = *request.MinimizeToTrayOnStart
	}

	proxyParams := proxySettingsParamsFromDomain(current.Proxy())
	if request.Proxy != nil {
		proxyParams = proxySettingsParamsFromDTO(*request.Proxy)
	}

	mainBounds := current.MainBounds()
	if request.MainBounds != nil {
		bounds, err := settings.NewMainWindowBounds(
			request.MainBounds.X,
			request.MainBounds.Y,
			request.MainBounds.Width,
			request.MainBounds.Height,
		)
		if err != nil {
			return dto.Settings{}, err
		}
		mainBounds = bounds
	}

	settingsBounds := current.SettingsBounds()
	if request.SettingsBounds != nil {
		bounds, err := settings.NewSettingsWindowBounds(
			request.SettingsBounds.X,
			request.SettingsBounds.Y,
			request.SettingsBounds.Width,
			request.SettingsBounds.Height,
		)
		if err != nil {
			return dto.Settings{}, err
		}
		settingsBounds = bounds
	}

	appearanceConfig := current.AppearanceConfig()
	if request.AppearanceConfig != nil {
		appearanceConfig = cloneAnyMap(request.AppearanceConfig)
	}

	nextVersion := current.Version() + 1
	if nextVersion <= 0 {
		nextVersion = 1
	}

	updated, err := settings.NewSettings(settings.SettingsParams{
		Appearance:            appearance,
		FontFamily:            fontFamily,
		FontSize:              fontSize,
		ThemeColor:            themeColor,
		ColorScheme:           colorScheme,
		Language:              language,
		DownloadDirectory:     downloadDirectory,
		MainBounds:            mainBounds,
		SettingsBounds:        settingsBounds,
		Version:               nextVersion,
		LogLevel:              logLevel,
		LogMaxSizeMB:          logMaxSizeMB,
		LogMaxBackups:         logMaxBackups,
		LogMaxAgeDays:         logMaxAgeDays,
		LogCompress:           &logCompress,
		Proxy:                 proxyParams,
		MenuBarVisibility:     &menuBarVisibility,
		AutoStart:             &autoStart,
		MinimizeToTrayOnStart: &minimizeToTrayOnStart,
		AppearanceConfig:      appearanceConfig,
	})
	if err != nil {
		return dto.Settings{}, err
	}

	if err := service.repo.Save(ctx, updated); err != nil {
		return dto.Settings{}, err
	}

	effectiveAppearance, err := service.resolveAppearance(ctx, updated.Appearance())
	if err != nil {
		return dto.Settings{}, err
	}

	return toDTO(updated, effectiveAppearance, service.resolveSystemThemeColor(ctx)), nil
}

func (service *SettingsService) resolveAppearance(ctx context.Context, appearance settings.AppearanceMode) (settings.AppearanceMode, error) {
	if appearance != settings.AppearanceAuto {
		return appearance, nil
	}
	if service.themeProvider == nil {
		return settings.AppearanceLight, nil
	}

	isDark, err := service.themeProvider.IsDarkMode(ctx)
	if err != nil {
		return settings.AppearanceLight, err
	}
	if isDark {
		return settings.AppearanceDark, nil
	}
	return settings.AppearanceLight, nil
}

func (service *SettingsService) resolveSystemThemeColor(ctx context.Context) string {
	if service.themeProvider == nil {
		return ""
	}
	accent, err := service.themeProvider.AccentColor(ctx)
	if err != nil {
		return ""
	}
	return accent
}

func sanitizeMenuBarVisibility(value string) string {
	if runtime.GOOS == "windows" && value == settings.MenuBarVisibilityNever.String() {
		return settings.MenuBarVisibilityWhenRunning.String()
	}
	return value
}

func toDTO(current settings.Settings, effective settings.AppearanceMode, systemThemeColor string) dto.Settings {
	return dto.Settings{
		Appearance:          current.Appearance().String(),
		EffectiveAppearance: effective.String(),
		FontFamily:          current.FontFamily(),
		FontSize:            current.FontSize(),
		ThemeColor:          current.ThemeColor(),
		ColorScheme:         current.ColorScheme().String(),
		SystemThemeColor:    systemThemeColor,
		Language:            current.Language().String(),
		DownloadDirectory:   current.DownloadDirectory(),
		MainBounds: dto.WindowBounds{
			X:      current.MainBounds().X(),
			Y:      current.MainBounds().Y(),
			Width:  current.MainBounds().Width(),
			Height: current.MainBounds().Height(),
		},
		SettingsBounds: dto.WindowBounds{
			X:      current.SettingsBounds().X(),
			Y:      current.SettingsBounds().Y(),
			Width:  current.SettingsBounds().Width(),
			Height: current.SettingsBounds().Height(),
		},
		Version:               current.Version(),
		LogLevel:              current.LogLevel().String(),
		LogMaxSizeMB:          current.LogMaxSizeMB(),
		LogMaxBackups:         current.LogMaxBackups(),
		LogMaxAgeDays:         current.LogMaxAgeDays(),
		LogCompress:           current.LogCompress(),
		MenuBarVisibility:     sanitizeMenuBarVisibility(current.MenuBarVisibility().String()),
		AutoStart:             current.AutoStart(),
		MinimizeToTrayOnStart: current.MinimizeToTrayOnStart(),
		Proxy:                 toProxyDTO(current.Proxy()),
		AppearanceConfig:      current.AppearanceConfig(),
	}
}

func toProxyDTO(proxy settings.ProxySettings) dto.Proxy {
	testedAt := ""
	if !proxy.LastTestedAt().IsZero() {
		testedAt = proxy.LastTestedAt().Format(time.RFC3339)
	}
	return dto.Proxy{
		Mode:           proxy.Mode().String(),
		Scheme:         proxy.Scheme().String(),
		Host:           proxy.Host(),
		Port:           proxy.Port(),
		Username:       proxy.Username(),
		Password:       proxy.Password(),
		NoProxy:        proxy.NoProxy(),
		TimeoutSeconds: int(proxy.Timeout().Seconds()),
		TestedAt:       testedAt,
		TestSuccess:    proxy.TestSuccess(),
		TestMessage:    proxy.TestMessage(),
	}
}

func proxySettingsParamsFromDomain(proxy settings.ProxySettings) settings.ProxySettingsParams {
	params := settings.ProxySettingsParams{
		Mode:           proxy.Mode().String(),
		Scheme:         proxy.Scheme().String(),
		Host:           proxy.Host(),
		Port:           proxy.Port(),
		Username:       proxy.Username(),
		Password:       proxy.Password(),
		NoProxy:        proxy.NoProxy(),
		TimeoutSeconds: int(proxy.Timeout().Seconds()),
		TestMessage:    proxy.TestMessage(),
	}
	if !proxy.LastTestedAt().IsZero() {
		testedAt := proxy.LastTestedAt()
		params.LastTestedAt = &testedAt
	}
	testSuccess := proxy.TestSuccess()
	params.TestSuccess = &testSuccess
	return params
}

func proxySettingsParamsFromDTO(proxy dto.Proxy) settings.ProxySettingsParams {
	return settings.ProxySettingsParams{
		Mode:           proxy.Mode,
		Scheme:         proxy.Scheme,
		Host:           proxy.Host,
		Port:           proxy.Port,
		Username:       proxy.Username,
		Password:       proxy.Password,
		NoProxy:        proxy.NoProxy,
		TimeoutSeconds: proxy.TimeoutSeconds,
		LastTestedAt:   parseProxyTestedAt(proxy.TestedAt),
		TestSuccess:    &proxy.TestSuccess,
		TestMessage:    proxy.TestMessage,
	}
}

func parseProxyTestedAt(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		zap.L().Warn("invalid proxy testedAt, ignoring", zap.String("testedAt", value), zap.Error(err))
		return nil
	}
	return &parsed
}

func cloneAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
