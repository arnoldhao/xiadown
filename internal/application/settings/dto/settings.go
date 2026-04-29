package dto

type WindowBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Settings struct {
	Appearance            string         `json:"appearance"`
	EffectiveAppearance   string         `json:"effectiveAppearance"`
	FontFamily            string         `json:"fontFamily"`
	FontSize              int            `json:"fontSize"`
	ThemeColor            string         `json:"themeColor"`
	ColorScheme           string         `json:"colorScheme"`
	SystemThemeColor      string         `json:"systemThemeColor"`
	Language              string         `json:"language"`
	DownloadDirectory     string         `json:"downloadDirectory"`
	MainBounds            WindowBounds   `json:"mainBounds"`
	SettingsBounds        WindowBounds   `json:"settingsBounds"`
	Version               int            `json:"version"`
	LogLevel              string         `json:"logLevel"`
	LogMaxSizeMB          int            `json:"logMaxSizeMB"`
	LogMaxBackups         int            `json:"logMaxBackups"`
	LogMaxAgeDays         int            `json:"logMaxAgeDays"`
	LogCompress           bool           `json:"logCompress"`
	MenuBarVisibility     string         `json:"menuBarVisibility"`
	AutoStart             bool           `json:"autoStart"`
	MinimizeToTrayOnStart bool           `json:"minimizeToTrayOnStart"`
	Proxy                 Proxy          `json:"proxy"`
	AppearanceConfig      map[string]any `json:"appearanceConfig,omitempty"`
}

type UpdateSettingsRequest struct {
	Appearance            *string        `json:"appearance"`
	FontFamily            *string        `json:"fontFamily"`
	FontSize              *int           `json:"fontSize"`
	ThemeColor            *string        `json:"themeColor"`
	ColorScheme           *string        `json:"colorScheme"`
	Language              *string        `json:"language"`
	DownloadDirectory     *string        `json:"downloadDirectory"`
	MainBounds            *WindowBounds  `json:"mainBounds"`
	SettingsBounds        *WindowBounds  `json:"settingsBounds"`
	LogLevel              *string        `json:"logLevel"`
	LogMaxSizeMB          *int           `json:"logMaxSizeMB"`
	LogMaxBackups         *int           `json:"logMaxBackups"`
	LogMaxAgeDays         *int           `json:"logMaxAgeDays"`
	LogCompress           *bool          `json:"logCompress"`
	MenuBarVisibility     *string        `json:"menuBarVisibility"`
	AutoStart             *bool          `json:"autoStart"`
	MinimizeToTrayOnStart *bool          `json:"minimizeToTrayOnStart"`
	Proxy                 *Proxy         `json:"proxy"`
	AppearanceConfig      map[string]any `json:"appearanceConfig,omitempty"`
}

type Proxy struct {
	Mode           string   `json:"mode"`
	Scheme         string   `json:"scheme"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	Password       string   `json:"password"`
	NoProxy        []string `json:"noProxy"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
	TestedAt       string   `json:"testedAt"`
	TestSuccess    bool     `json:"testSuccess"`
	TestMessage    string   `json:"testMessage"`
}

type SystemProxyInfo struct {
	Address string `json:"address"`
	Source  string `json:"source,omitempty"`
	Name    string `json:"name,omitempty"`
}
