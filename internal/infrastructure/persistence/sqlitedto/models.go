package sqlitedto

import (
	"database/sql"
	"time"

	"github.com/uptrace/bun"
)

type ConnectorRow struct {
	bun.BaseModel `bun:"table:connectors"`

	ID             string         `bun:"id,pk"`
	Type           string         `bun:"type"`
	Status         string         `bun:"status"`
	CookiesPath    sql.NullString `bun:"cookies_path"`
	CookiesJSON    sql.NullString `bun:"cookies_json"`
	LastVerifiedAt sql.NullTime   `bun:"last_verified_at"`
	CreatedAt      time.Time      `bun:"created_at"`
	UpdatedAt      time.Time      `bun:"updated_at"`
}

type TelemetryStateRow struct {
	bun.BaseModel `bun:"table:telemetry_state"`

	ID                        int             `bun:"id,pk"`
	InstallID                 string          `bun:"install_id"`
	InstallCreatedAt          time.Time       `bun:"install_created_at"`
	LaunchCount               int             `bun:"launch_count"`
	DistinctDaysUsed          int             `bun:"distinct_days_used"`
	DistinctDaysUsedLastMonth int             `bun:"distinct_days_used_last_month"`
	CompletedSessionCount     int             `bun:"completed_session_count"`
	TotalSessionSeconds       float64         `bun:"total_session_seconds"`
	PreviousSessionSeconds    sql.NullFloat64 `bun:"previous_session_seconds"`
	FirstChatCompletedAt      sql.NullTime    `bun:"first_chat_completed_at"`
	FirstLibraryCompletedAt   sql.NullTime    `bun:"first_library_completed_at"`
	UpdatedAt                 time.Time       `bun:"updated_at"`
}

type DependencyRow struct {
	bun.BaseModel `bun:"table:dependencies"`

	Name        string         `bun:"name,pk"`
	ExecPath    sql.NullString `bun:"exec_path"`
	Version     sql.NullString `bun:"version"`
	Status      sql.NullString `bun:"status"`
	InstalledAt sql.NullTime   `bun:"installed_at"`
	UpdatedAt   time.Time      `bun:"updated_at"`
}

type SpriteRow struct {
	bun.BaseModel `bun:"table:sprites"`

	ID                string         `bun:"id,pk"`
	Name              string         `bun:"name"`
	Description       string         `bun:"description"`
	FrameCount        int            `bun:"frame_count"`
	DeprecatedWidth   int            `bun:"frame_width"`
	DeprecatedHeight  int            `bun:"frame_height"`
	Columns           int            `bun:"columns"`
	Rows              int            `bun:"rows"`
	SpriteFile        string         `bun:"sprite_file"`
	SpritePath        string         `bun:"sprite_path"`
	SourceType        string         `bun:"source_type"`
	Origin            string         `bun:"origin"`
	Scope             string         `bun:"scope"`
	Status            string         `bun:"status"`
	ValidationMessage sql.NullString `bun:"validation_message"`
	ImageWidth        int            `bun:"image_width"`
	ImageHeight       int            `bun:"image_height"`
	AuthorID          string         `bun:"author_id"`
	AuthorDisplayName string         `bun:"author_display_name"`
	CreatedAt         time.Time      `bun:"created_at"`
	Version           string         `bun:"version"`
	CoverPNG          []byte         `bun:"cover_png"`
	UpdatedAt         time.Time      `bun:"updated_at"`
}

type SettingsRow struct {
	bun.BaseModel `bun:"table:settings"`

	ID                    int            `bun:"id,pk"`
	Appearance            string         `bun:"appearance"`
	FontFamily            sql.NullString `bun:"font_family"`
	FontSize              sql.NullInt64  `bun:"font_size"`
	ThemeColor            sql.NullString `bun:"theme_color"`
	ColorScheme           sql.NullString `bun:"color_scheme"`
	Language              sql.NullString `bun:"language"`
	DownloadDirectory     sql.NullString `bun:"download_directory"`
	LogLevel              sql.NullString `bun:"log_level"`
	LogMaxSize            sql.NullInt64  `bun:"log_max_size_mb"`
	LogBackups            sql.NullInt64  `bun:"log_max_backups"`
	LogAge                sql.NullInt64  `bun:"log_max_age_days"`
	LogCompress           sql.NullBool   `bun:"log_compress"`
	MenuBarVisibility     sql.NullString `bun:"menu_bar_visibility"`
	AutoStart             sql.NullBool   `bun:"auto_start"`
	MinimizeToTrayOnStart sql.NullBool   `bun:"minimize_to_tray_on_start"`
	AppearanceConfigJSON  sql.NullString `bun:"appearance_config_json"`
	MainX                 sql.NullInt64  `bun:"main_x"`
	MainY                 sql.NullInt64  `bun:"main_y"`
	MainWidth             sql.NullInt64  `bun:"main_width"`
	MainHeight            sql.NullInt64  `bun:"main_height"`
	SettingsX             sql.NullInt64  `bun:"settings_x"`
	SettingsY             sql.NullInt64  `bun:"settings_y"`
	SettingsWidth         sql.NullInt64  `bun:"settings_width"`
	SettingsHeight        sql.NullInt64  `bun:"settings_height"`
	Version               int            `bun:"version"`
	ProxyMode             sql.NullString `bun:"proxy_mode"`
	ProxyScheme           sql.NullString `bun:"proxy_scheme"`
	ProxyHost             sql.NullString `bun:"proxy_host"`
	ProxyPort             sql.NullInt64  `bun:"proxy_port"`
	ProxyUsername         sql.NullString `bun:"proxy_username"`
	ProxyPassword         sql.NullString `bun:"proxy_password"`
	ProxyNoProxy          sql.NullString `bun:"proxy_no_proxy"`
	ProxyTimeoutSeconds   sql.NullInt64  `bun:"proxy_timeout_seconds"`
	ProxyTestedAt         sql.NullTime   `bun:"proxy_tested_at"`
	ProxyTestSuccess      sql.NullBool   `bun:"proxy_test_success"`
	ProxyTestMessage      sql.NullString `bun:"proxy_test_message"`
}

type TranscodePresetRow struct {
	bun.BaseModel `bun:"table:transcode_presets"`

	ID               string         `bun:"id,pk"`
	Name             string         `bun:"name"`
	OutputType       string         `bun:"output_type"`
	Container        string         `bun:"container"`
	VideoCodec       sql.NullString `bun:"video_codec"`
	AudioCodec       sql.NullString `bun:"audio_codec"`
	QualityMode      sql.NullString `bun:"quality_mode"`
	CRF              sql.NullInt64  `bun:"crf"`
	BitrateKbps      sql.NullInt64  `bun:"bitrate_kbps"`
	AudioBitrateKbps sql.NullInt64  `bun:"audio_bitrate_kbps"`
	Scale            sql.NullString `bun:"scale"`
	Width            sql.NullInt64  `bun:"width"`
	Height           sql.NullInt64  `bun:"height"`
	FFmpegPreset     sql.NullString `bun:"ffmpeg_preset"`
	AllowUpscale     bool           `bun:"allow_upscale"`
	RequiresVideo    bool           `bun:"requires_video"`
	RequiresAudio    bool           `bun:"requires_audio"`
	IsBuiltin        bool           `bun:"is_builtin"`
	Description      sql.NullString `bun:"description"`
	CreatedAt        time.Time      `bun:"created_at"`
	UpdatedAt        time.Time      `bun:"updated_at"`
}
