package settings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AppearanceMode string
type LogLevel string
type Language string
type ColorScheme string
type MenuBarVisibility string
type ProxyMode string
type ProxyScheme string

type WindowBounds struct {
	x      int
	y      int
	width  int
	height int
}

type Settings struct {
	appearance            AppearanceMode
	fontFamily            string
	themeColor            string
	colorScheme           ColorScheme
	fontSize              int
	language              Language
	downloadDirectory     string
	mainBounds            WindowBounds
	settingsBounds        WindowBounds
	version               int
	logLevel              LogLevel
	logMaxSizeMB          int
	logMaxBackups         int
	logMaxAgeDays         int
	logCompress           bool
	proxy                 ProxySettings
	menuBarVisibility     MenuBarVisibility
	autoStart             bool
	minimizeToTrayOnStart bool
	appearanceConfig      map[string]any
}

type SettingsParams struct {
	Appearance            string
	FontFamily            string
	ThemeColor            string
	ColorScheme           string
	FontSize              int
	Language              string
	DownloadDirectory     string
	MainBounds            WindowBounds
	SettingsBounds        WindowBounds
	Version               int
	LogLevel              string
	LogMaxSizeMB          int
	LogMaxBackups         int
	LogMaxAgeDays         int
	LogCompress           *bool
	Proxy                 ProxySettingsParams
	MenuBarVisibility     *string
	AutoStart             *bool
	MinimizeToTrayOnStart *bool
	AppearanceConfig      map[string]any
}

const (
	AppearanceLight AppearanceMode = "light"
	AppearanceDark  AppearanceMode = "dark"
	AppearanceAuto  AppearanceMode = "auto"
)

const (
	LanguageEnglish           Language = "en"
	LanguageChineseSimplified Language = "zh-CN"
	DefaultLanguage                    = LanguageEnglish
)

const (
	DefaultMainWidth        = 960
	DefaultMainHeight       = 640
	DefaultSettingsWidth    = 480
	DefaultSettingsHeight   = 550
	MinMainWindowWidth      = 960
	MinMainWindowHeight     = 640
	MinSettingsWindowWidth  = 480
	MinSettingsWindowHeight = 550
	DefaultFontSize         = 15
	MinFontSize             = 12
	MaxFontSize             = 24
)

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"

	DefaultLogLevel = LogLevelInfo
)

const (
	DefaultLogMaxSizeMB  = 50
	DefaultLogMaxBackups = 5
	DefaultLogMaxAgeDays = 7
	DefaultLogCompress   = true
)

const (
	ThemeColorSystem = "system"
)

const (
	ColorSchemeDefault  ColorScheme = "default"
	ColorSchemeContrast ColorScheme = "contrast"
	ColorSchemeSlate    ColorScheme = "slate"
	ColorSchemeWarm     ColorScheme = "warm"
	ColorSchemeFresh    ColorScheme = "fresh"
	ColorSchemeCandy    ColorScheme = "candy"
	ColorSchemePixel    ColorScheme = "pixel"

	DefaultColorScheme = ColorSchemeDefault
)

const (
	MenuBarVisibilityAlways      MenuBarVisibility = "always"
	MenuBarVisibilityWhenRunning MenuBarVisibility = "whenRunning"
	MenuBarVisibilityNever       MenuBarVisibility = "never"

	DefaultMenuBarVisibility = MenuBarVisibilityWhenRunning
)

const (
	DefaultProxyTimeoutSeconds = 30
)

const (
	ProxyModeNone   ProxyMode = "none"
	ProxyModeSystem ProxyMode = "system"
	ProxyModeManual ProxyMode = "manual"
)

const (
	ProxySchemeHTTP   ProxyScheme = "http"
	ProxySchemeHTTPS  ProxyScheme = "https"
	ProxySchemeSocks5 ProxyScheme = "socks5"
)

type ProxySettings struct {
	mode     ProxyMode
	scheme   ProxyScheme
	host     string
	port     int
	username string
	password string
	noProxy  []string
	timeout  time.Duration

	lastTestedAt time.Time
	testSuccess  bool
	testMessage  string
}

type ProxySettingsParams struct {
	Mode           string
	Scheme         string
	Host           string
	Port           int
	Username       string
	Password       string
	NoProxy        []string
	TimeoutSeconds int

	LastTestedAt *time.Time
	TestSuccess  *bool
	TestMessage  string
}

func DefaultDownloadDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(home)
	if trimmed == "" {
		return ""
	}
	return filepath.Join(trimmed, "Downloads", "xiadown")
}

func NewWindowBounds(x, y, width, height int) (WindowBounds, error) {
	return NewWindowBoundsWithMin(x, y, width, height, MinMainWindowWidth, MinMainWindowHeight)
}

func NewMainWindowBounds(x, y, width, height int) (WindowBounds, error) {
	return NewWindowBoundsWithMin(x, y, width, height, MinMainWindowWidth, MinMainWindowHeight)
}

func NewSettingsWindowBounds(x, y, width, height int) (WindowBounds, error) {
	return NewWindowBoundsWithMin(x, y, width, height, MinSettingsWindowWidth, MinSettingsWindowHeight)
}

func NewWindowBoundsWithMin(x, y, width, height, minWidth, minHeight int) (WindowBounds, error) {
	if width < minWidth || height < minHeight {
		return WindowBounds{}, fmt.Errorf("%w: window size", ErrInvalidSettings)
	}

	return WindowBounds{x: x, y: y, width: width, height: height}, nil
}

func NewSettings(params SettingsParams) (Settings, error) {
	appearance, err := ParseAppearanceMode(params.Appearance)
	if err != nil {
		return Settings{}, err
	}

	colorScheme, err := ParseColorScheme(params.ColorScheme)
	if err != nil {
		return Settings{}, err
	}

	fontSize := params.FontSize
	if fontSize <= 0 {
		fontSize = DefaultFontSize
	}
	if fontSize < MinFontSize || fontSize > MaxFontSize {
		return Settings{}, fmt.Errorf("%w: font size", ErrInvalidSettings)
	}

	parsedLanguage, err := ParseLanguage(params.Language)
	if err != nil {
		return Settings{}, err
	}

	downloadDirectory := strings.TrimSpace(params.DownloadDirectory)
	if downloadDirectory == "" {
		downloadDirectory = DefaultDownloadDirectory()
	}

	logLevel, err := ParseLogLevel(params.LogLevel)
	if err != nil {
		return Settings{}, err
	}

	if params.Version <= 0 {
		params.Version = 1
	}

	logMaxSizeMB := positiveOrDefault(params.LogMaxSizeMB, DefaultLogMaxSizeMB)
	logMaxBackups := positiveOrDefault(params.LogMaxBackups, DefaultLogMaxBackups)
	logMaxAgeDays := positiveOrDefault(params.LogMaxAgeDays, DefaultLogMaxAgeDays)

	logCompress := DefaultLogCompress
	if params.LogCompress != nil {
		logCompress = *params.LogCompress
	}

	proxySettings, err := NewProxySettings(params.Proxy)
	if err != nil {
		return Settings{}, err
	}

	menuBarVisibility := DefaultMenuBarVisibility
	if params.MenuBarVisibility != nil {
		menuBarVisibility, err = ParseMenuBarVisibility(*params.MenuBarVisibility)
		if err != nil {
			return Settings{}, err
		}
	}

	autoStart := false
	if params.AutoStart != nil {
		autoStart = *params.AutoStart
	}

	minimizeToTrayOnStart := false
	if params.MinimizeToTrayOnStart != nil {
		minimizeToTrayOnStart = *params.MinimizeToTrayOnStart
	}

	return Settings{
		appearance:            appearance,
		fontFamily:            strings.TrimSpace(params.FontFamily),
		themeColor:            strings.TrimSpace(params.ThemeColor),
		colorScheme:           colorScheme,
		fontSize:              fontSize,
		language:              parsedLanguage,
		downloadDirectory:     downloadDirectory,
		mainBounds:            params.MainBounds,
		settingsBounds:        params.SettingsBounds,
		version:               params.Version,
		logLevel:              logLevel,
		logMaxSizeMB:          logMaxSizeMB,
		logMaxBackups:         logMaxBackups,
		logMaxAgeDays:         logMaxAgeDays,
		logCompress:           logCompress,
		proxy:                 proxySettings,
		menuBarVisibility:     menuBarVisibility,
		autoStart:             autoStart,
		minimizeToTrayOnStart: minimizeToTrayOnStart,
		appearanceConfig:      cloneAnyMap(params.AppearanceConfig),
	}, nil
}

func DefaultSettingsWithLanguage(language string) Settings {
	mainBounds, _ := NewMainWindowBounds(0, 0, DefaultMainWidth, DefaultMainHeight)
	settingsBounds, _ := NewSettingsWindowBounds(0, 0, DefaultSettingsWidth, DefaultSettingsHeight)
	parsedLanguage, _ := ParseLanguage(language)
	return Settings{
		appearance:            AppearanceAuto,
		themeColor:            ThemeColorSystem,
		colorScheme:           DefaultColorScheme,
		fontSize:              DefaultFontSize,
		language:              parsedLanguage,
		downloadDirectory:     DefaultDownloadDirectory(),
		mainBounds:            mainBounds,
		settingsBounds:        settingsBounds,
		version:               1,
		logLevel:              DefaultLogLevel,
		logMaxSizeMB:          DefaultLogMaxSizeMB,
		logMaxBackups:         DefaultLogMaxBackups,
		logMaxAgeDays:         DefaultLogMaxAgeDays,
		logCompress:           DefaultLogCompress,
		proxy:                 DefaultProxySettings(),
		menuBarVisibility:     DefaultMenuBarVisibility,
		autoStart:             false,
		minimizeToTrayOnStart: false,
		appearanceConfig:      nil,
	}
}

func DefaultSettings() Settings {
	return DefaultSettingsWithLanguage(DefaultLanguage.String())
}

func ParseAppearanceMode(value string) (AppearanceMode, error) {
	switch AppearanceMode(strings.TrimSpace(value)) {
	case AppearanceLight, AppearanceDark, AppearanceAuto:
		return AppearanceMode(strings.TrimSpace(value)), nil
	default:
		return "", fmt.Errorf("%w: appearance", ErrInvalidSettings)
	}
}

func ParseLanguage(value string) (Language, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultLanguage, nil
	}
	switch Language(trimmed) {
	case LanguageEnglish, LanguageChineseSimplified:
		return Language(trimmed), nil
	default:
		return "", fmt.Errorf("%w: language", ErrInvalidSettings)
	}
}

func ParseColorScheme(value string) (ColorScheme, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultColorScheme, nil
	}
	switch ColorScheme(trimmed) {
	case ColorSchemeDefault, ColorSchemeContrast, ColorSchemeSlate, ColorSchemeWarm, ColorSchemeFresh, ColorSchemeCandy, ColorSchemePixel:
		return ColorScheme(trimmed), nil
	default:
		return "", fmt.Errorf("%w: color scheme", ErrInvalidSettings)
	}
}

func ParseLogLevel(value string) (LogLevel, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultLogLevel, nil
	}
	switch LogLevel(trimmed) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		return LogLevel(trimmed), nil
	default:
		return "", fmt.Errorf("%w: log level", ErrInvalidSettings)
	}
}

func ParseMenuBarVisibility(value string) (MenuBarVisibility, error) {
	switch MenuBarVisibility(strings.TrimSpace(value)) {
	case MenuBarVisibilityAlways, MenuBarVisibilityWhenRunning, MenuBarVisibilityNever:
		return MenuBarVisibility(strings.TrimSpace(value)), nil
	default:
		return "", fmt.Errorf("%w: menu bar visibility", ErrInvalidSettings)
	}
}

func ParseProxyMode(value string) (ProxyMode, error) {
	switch ProxyMode(strings.TrimSpace(value)) {
	case ProxyModeNone, ProxyModeSystem, ProxyModeManual:
		return ProxyMode(strings.TrimSpace(value)), nil
	default:
		return "", fmt.Errorf("%w: proxy mode", ErrInvalidSettings)
	}
}

func ParseProxyScheme(value string) (ProxyScheme, error) {
	switch ProxyScheme(strings.TrimSpace(value)) {
	case ProxySchemeHTTP, ProxySchemeHTTPS, ProxySchemeSocks5:
		return ProxyScheme(strings.TrimSpace(value)), nil
	default:
		return "", fmt.Errorf("%w: proxy scheme", ErrInvalidSettings)
	}
}

func DefaultProxySettings() ProxySettings {
	return ProxySettings{
		mode:    ProxyModeNone,
		scheme:  ProxySchemeHTTP,
		timeout: DefaultProxyTimeoutSeconds * time.Second,
		noProxy: []string{"localhost", "127.0.0.1"},
	}
}

func NewProxySettings(params ProxySettingsParams) (ProxySettings, error) {
	mode := ProxyModeNone
	if params.Mode != "" {
		parsedMode, err := ParseProxyMode(params.Mode)
		if err != nil {
			return ProxySettings{}, err
		}
		mode = parsedMode
	}

	scheme := ProxySchemeHTTP
	if params.Scheme != "" {
		parsedScheme, err := ParseProxyScheme(params.Scheme)
		if err != nil {
			return ProxySettings{}, err
		}
		scheme = parsedScheme
	}

	timeoutSeconds := params.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = DefaultProxyTimeoutSeconds
	}

	if mode == ProxyModeManual {
		if strings.TrimSpace(params.Host) == "" {
			return ProxySettings{}, fmt.Errorf("%w: proxy host", ErrInvalidSettings)
		}
		if params.Port <= 0 || params.Port > 65535 {
			return ProxySettings{}, fmt.Errorf("%w: proxy port", ErrInvalidSettings)
		}
	}

	noProxy := make([]string, 0, len(params.NoProxy))
	for _, entry := range params.NoProxy {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			noProxy = append(noProxy, trimmed)
		}
	}
	if len(noProxy) == 0 {
		noProxy = []string{"localhost", "127.0.0.1"}
	}

	testSuccess := false
	if params.TestSuccess != nil {
		testSuccess = *params.TestSuccess
	}

	var testedAt time.Time
	if params.LastTestedAt != nil {
		testedAt = *params.LastTestedAt
	}

	return ProxySettings{
		mode:         mode,
		scheme:       scheme,
		host:         strings.TrimSpace(params.Host),
		port:         params.Port,
		username:     strings.TrimSpace(params.Username),
		password:     params.Password,
		noProxy:      noProxy,
		timeout:      time.Duration(timeoutSeconds) * time.Second,
		lastTestedAt: testedAt,
		testSuccess:  testSuccess,
		testMessage:  strings.TrimSpace(params.TestMessage),
	}, nil
}

func (proxy ProxySettings) Mode() ProxyMode        { return proxy.mode }
func (proxy ProxySettings) Scheme() ProxyScheme    { return proxy.scheme }
func (proxy ProxySettings) Host() string           { return proxy.host }
func (proxy ProxySettings) Port() int              { return proxy.port }
func (proxy ProxySettings) Username() string       { return proxy.username }
func (proxy ProxySettings) Password() string       { return proxy.password }
func (proxy ProxySettings) Timeout() time.Duration { return proxy.timeout }
func (proxy ProxySettings) LastTestedAt() time.Time {
	return proxy.lastTestedAt
}
func (proxy ProxySettings) TestSuccess() bool   { return proxy.testSuccess }
func (proxy ProxySettings) TestMessage() string { return proxy.testMessage }

func (proxy ProxySettings) NoProxy() []string {
	copied := make([]string, len(proxy.noProxy))
	copy(copied, proxy.noProxy)
	return copied
}

func (proxy ProxySettings) WithTestResult(success bool, message string, testedAt time.Time) ProxySettings {
	proxy.testSuccess = success
	proxy.testMessage = strings.TrimSpace(message)
	proxy.lastTestedAt = testedAt
	return proxy
}

func (settings Settings) Appearance() AppearanceMode           { return settings.appearance }
func (settings Settings) FontFamily() string                   { return settings.fontFamily }
func (settings Settings) FontSize() int                        { return settings.fontSize }
func (settings Settings) ThemeColor() string                   { return settings.themeColor }
func (settings Settings) ColorScheme() ColorScheme             { return settings.colorScheme }
func (settings Settings) Language() Language                   { return settings.language }
func (settings Settings) DownloadDirectory() string            { return settings.downloadDirectory }
func (settings Settings) MainBounds() WindowBounds             { return settings.mainBounds }
func (settings Settings) SettingsBounds() WindowBounds         { return settings.settingsBounds }
func (settings Settings) Version() int                         { return settings.version }
func (settings Settings) LogLevel() LogLevel                   { return settings.logLevel }
func (settings Settings) LogMaxSizeMB() int                    { return settings.logMaxSizeMB }
func (settings Settings) LogMaxBackups() int                   { return settings.logMaxBackups }
func (settings Settings) LogMaxAgeDays() int                   { return settings.logMaxAgeDays }
func (settings Settings) LogCompress() bool                    { return settings.logCompress }
func (settings Settings) Proxy() ProxySettings                 { return settings.proxy }
func (settings Settings) MenuBarVisibility() MenuBarVisibility { return settings.menuBarVisibility }
func (settings Settings) AutoStart() bool                      { return settings.autoStart }
func (settings Settings) MinimizeToTrayOnStart() bool          { return settings.minimizeToTrayOnStart }
func (settings Settings) AppearanceConfig() map[string]any {
	return cloneAnyMap(settings.appearanceConfig)
}

func (settings Settings) WithAppearanceConfig(config map[string]any) Settings {
	settings.appearanceConfig = cloneAnyMap(config)
	return settings
}

func (bounds WindowBounds) X() int      { return bounds.x }
func (bounds WindowBounds) Y() int      { return bounds.y }
func (bounds WindowBounds) Width() int  { return bounds.width }
func (bounds WindowBounds) Height() int { return bounds.height }

func (mode AppearanceMode) String() string          { return string(mode) }
func (language Language) String() string            { return string(language) }
func (level LogLevel) String() string               { return string(level) }
func (scheme ColorScheme) String() string           { return string(scheme) }
func (visibility MenuBarVisibility) String() string { return string(visibility) }
func (mode ProxyMode) String() string               { return string(mode) }
func (scheme ProxyScheme) String() string           { return string(scheme) }

func IsSystemThemeColor(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), ThemeColorSystem)
}

func positiveOrDefault(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func cloneAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = cloneAnyValue(value)
	}
	return result
}

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		copied := make([]any, len(typed))
		for index, item := range typed {
			copied[index] = cloneAnyValue(item)
		}
		return copied
	case []string:
		copied := make([]string, len(typed))
		copy(copied, typed)
		return copied
	default:
		return value
	}
}

type Repository interface {
	Get(ctx context.Context) (Settings, error)
	Save(ctx context.Context, current Settings) error
}
