package settingsrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"xiadown/internal/domain/settings"
	"xiadown/internal/infrastructure/persistence/sqlitedto"
)

type SQLiteSettingsRepository struct {
	db *bun.DB
}

type settingsRow = sqlitedto.SettingsRow

func NewSQLiteSettingsRepository(db *bun.DB) *SQLiteSettingsRepository {
	return &SQLiteSettingsRepository{db: db}
}

func (repo *SQLiteSettingsRepository) Get(ctx context.Context) (settings.Settings, error) {
	row := new(settingsRow)
	if err := repo.db.NewSelect().Model(row).Where("id = 1").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return settings.Settings{}, settings.ErrSettingsNotFound
		}
		return settings.Settings{}, err
	}

	mainBounds, err := settings.NewMainWindowBounds(
		intOrZero(row.MainX),
		intOrZero(row.MainY),
		clampWindowDimensionOrDefault(row.MainWidth, settings.DefaultMainWidth, settings.MinMainWindowWidth),
		clampWindowDimensionOrDefault(row.MainHeight, settings.DefaultMainHeight, settings.MinMainWindowHeight),
	)
	if err != nil {
		return settings.Settings{}, err
	}

	settingsBounds, err := settings.NewSettingsWindowBounds(
		intOrZero(row.SettingsX),
		intOrZero(row.SettingsY),
		clampWindowDimensionOrDefault(row.SettingsWidth, settings.DefaultSettingsWidth, settings.MinSettingsWindowWidth),
		clampWindowDimensionOrDefault(row.SettingsHeight, settings.DefaultSettingsHeight, settings.MinSettingsWindowHeight),
	)
	if err != nil {
		return settings.Settings{}, err
	}

	logCompress := boolOrDefault(row.LogCompress, settings.DefaultLogCompress)
	menuBarVisibility := stringOrEmpty(row.MenuBarVisibility)
	if menuBarVisibility == "" {
		menuBarVisibility = settings.DefaultMenuBarVisibility.String()
	}
	autoStart := boolOrDefault(row.AutoStart, false)
	minimizeToTrayOnStart := boolOrDefault(row.MinimizeToTrayOnStart, false)
	syncedLyricsEnabled := boolOrDefault(row.SyncedLyricsEnabled, settings.DefaultSyncedLyricsEnabled)
	romanizedLyrics := boolOrDefault(row.RomanizedLyrics, settings.DefaultRomanizedLyrics)
	pinyinLyrics := boolOrDefault(row.PinyinLyrics, settings.DefaultPinyinLyrics)
	playbackAudioQuality := stringOrEmpty(row.PlaybackAudioQuality)
	if playbackAudioQuality == "" {
		playbackAudioQuality = settings.DefaultPlaybackAudioQuality.String()
	}
	resourceSniffScope := stringOrEmpty(row.ResourceSniffScope)
	if resourceSniffScope == "" {
		resourceSniffScope = settings.DefaultResourceSniffScope.String()
	}

	var lastTestedAt *time.Time
	if row.ProxyTestedAt.Valid {
		last := row.ProxyTestedAt.Time
		lastTestedAt = &last
	}

	var testSuccess *bool
	if row.ProxyTestSuccess.Valid {
		value := row.ProxyTestSuccess.Bool
		testSuccess = &value
	}

	return settings.NewSettings(settings.SettingsParams{
		Appearance:        row.Appearance,
		FontFamily:        stringOrEmpty(row.FontFamily),
		FontSize:          clampFontSizeOrDefault(row.FontSize),
		ThemeColor:        stringOrEmpty(row.ThemeColor),
		ColorScheme:       stringOrEmpty(row.ColorScheme),
		Language:          stringOrEmpty(row.Language),
		SniffBrowser:      stringOrEmpty(row.SniffBrowser),
		DownloadDirectory: stringOrEmpty(row.DownloadDirectory),
		MainBounds:        mainBounds,
		SettingsBounds:    settingsBounds,
		Version:           row.Version,
		LogLevel:          stringOrEmpty(row.LogLevel),
		LogMaxSizeMB:      clampPositiveOrDefault(row.LogMaxSize, settings.DefaultLogMaxSizeMB),
		LogMaxBackups:     clampPositiveOrDefault(row.LogBackups, settings.DefaultLogMaxBackups),
		LogMaxAgeDays:     clampPositiveOrDefault(row.LogAge, settings.DefaultLogMaxAgeDays),
		LogCompress:       &logCompress,
		Proxy: settings.ProxySettingsParams{
			Mode:           stringOrEmpty(row.ProxyMode),
			Scheme:         stringOrEmpty(row.ProxyScheme),
			Host:           stringOrEmpty(row.ProxyHost),
			Port:           intOrZero(row.ProxyPort),
			Username:       stringOrEmpty(row.ProxyUsername),
			Password:       stringOrEmpty(row.ProxyPassword),
			NoProxy:        parseStringSlice(row.ProxyNoProxy),
			TimeoutSeconds: clampPositiveOrDefault(row.ProxyTimeoutSeconds, settings.DefaultProxyTimeoutSeconds),
			LastTestedAt:   lastTestedAt,
			TestSuccess:    testSuccess,
			TestMessage:    stringOrEmpty(row.ProxyTestMessage),
		},
		MenuBarVisibility:        &menuBarVisibility,
		AutoStart:                &autoStart,
		MinimizeToTrayOnStart:    &minimizeToTrayOnStart,
		SyncedLyricsEnabled:      &syncedLyricsEnabled,
		RomanizedLyrics:          &romanizedLyrics,
		PinyinLyrics:             &pinyinLyrics,
		PlaybackAudioQuality:     playbackAudioQuality,
		ResourceSniffScope:       resourceSniffScope,
		ResourceSniffMinBytes:    clampResourceSniffMinBytes(row.ResourceSniffMinBytes),
		ResourceSniffRetain:      clampResourceSniffRetain(row.ResourceSniffRetain),
		YTDLPConcurrentDownloads: clampYTDLPConcurrentDownloads(row.YTDLPConcurrentDownloads),
		YTDLPConcurrentFragments: clampYTDLPConcurrentFragments(row.YTDLPConcurrentFragments),
		AppearanceConfig:         parseAnyMap(row.AppearanceConfigJSON),
	})
}

func (repo *SQLiteSettingsRepository) Save(ctx context.Context, current settings.Settings) error {
	proxy := current.Proxy()
	row := settingsRow{
		ID:                       1,
		Appearance:               current.Appearance().String(),
		FontFamily:               nullString(current.FontFamily()),
		FontSize:                 nullInt64(current.FontSize()),
		ThemeColor:               nullString(current.ThemeColor()),
		ColorScheme:              nullString(current.ColorScheme().String()),
		Language:                 nullString(current.Language().String()),
		SniffBrowser:             nullString(current.SniffBrowser()),
		DownloadDirectory:        nullString(current.DownloadDirectory()),
		LogLevel:                 nullString(current.LogLevel().String()),
		LogMaxSize:               nullInt64(current.LogMaxSizeMB()),
		LogBackups:               nullInt64(current.LogMaxBackups()),
		LogAge:                   nullInt64(current.LogMaxAgeDays()),
		LogCompress:              nullBool(current.LogCompress()),
		MenuBarVisibility:        nullString(current.MenuBarVisibility().String()),
		AutoStart:                nullBool(current.AutoStart()),
		MinimizeToTrayOnStart:    nullBool(current.MinimizeToTrayOnStart()),
		SyncedLyricsEnabled:      nullBool(current.SyncedLyricsEnabled()),
		RomanizedLyrics:          nullBool(current.RomanizedLyrics()),
		PinyinLyrics:             nullBool(current.PinyinLyrics()),
		PlaybackAudioQuality:     nullString(current.PlaybackAudioQuality().String()),
		ResourceSniffScope:       nullString(current.ResourceSniffScope().String()),
		ResourceSniffMinBytes:    nullInt64(current.ResourceSniffMinBytes()),
		ResourceSniffRetain:      nullInt64(current.ResourceSniffRetain()),
		YTDLPConcurrentDownloads: nullInt64(current.YTDLPConcurrentDownloads()),
		YTDLPConcurrentFragments: nullInt64(current.YTDLPConcurrentFragments()),
		AppearanceConfigJSON:     jsonAnyMap(current.AppearanceConfig()),
		MainX:                    nullInt64(current.MainBounds().X()),
		MainY:                    nullInt64(current.MainBounds().Y()),
		MainWidth:                nullInt64(current.MainBounds().Width()),
		MainHeight:               nullInt64(current.MainBounds().Height()),
		SettingsX:                nullInt64(current.SettingsBounds().X()),
		SettingsY:                nullInt64(current.SettingsBounds().Y()),
		SettingsWidth:            nullInt64(current.SettingsBounds().Width()),
		SettingsHeight:           nullInt64(current.SettingsBounds().Height()),
		Version:                  current.Version(),
		ProxyMode:                nullString(proxy.Mode().String()),
		ProxyScheme:              nullString(proxy.Scheme().String()),
		ProxyHost:                nullString(proxy.Host()),
		ProxyPort:                nullInt64(proxy.Port()),
		ProxyUsername:            nullString(proxy.Username()),
		ProxyPassword:            nullString(proxy.Password()),
		ProxyNoProxy:             jsonStringSlice(proxy.NoProxy()),
		ProxyTimeoutSeconds:      nullInt64(int(proxy.Timeout().Seconds())),
		ProxyTestedAt:            nullTime(proxy.LastTestedAt()),
		ProxyTestSuccess:         nullBool(proxy.TestSuccess()),
		ProxyTestMessage:         nullString(proxy.TestMessage()),
	}

	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("appearance = EXCLUDED.appearance").
		Set("font_family = EXCLUDED.font_family").
		Set("font_size = EXCLUDED.font_size").
		Set("theme_color = EXCLUDED.theme_color").
		Set("color_scheme = EXCLUDED.color_scheme").
		Set("language = EXCLUDED.language").
		Set("default_browser = EXCLUDED.default_browser").
		Set("download_directory = EXCLUDED.download_directory").
		Set("log_level = EXCLUDED.log_level").
		Set("log_max_size_mb = EXCLUDED.log_max_size_mb").
		Set("log_max_backups = EXCLUDED.log_max_backups").
		Set("log_max_age_days = EXCLUDED.log_max_age_days").
		Set("log_compress = EXCLUDED.log_compress").
		Set("menu_bar_visibility = EXCLUDED.menu_bar_visibility").
		Set("auto_start = EXCLUDED.auto_start").
		Set("minimize_to_tray_on_start = EXCLUDED.minimize_to_tray_on_start").
		Set("synced_lyrics_enabled = EXCLUDED.synced_lyrics_enabled").
		Set("romanized_lyrics = EXCLUDED.romanized_lyrics").
		Set("pinyin_lyrics = EXCLUDED.pinyin_lyrics").
		Set("playback_audio_quality = EXCLUDED.playback_audio_quality").
		Set("resource_sniff_scope = EXCLUDED.resource_sniff_scope").
		Set("resource_sniff_min_bytes = EXCLUDED.resource_sniff_min_bytes").
		Set("resource_sniff_retain = EXCLUDED.resource_sniff_retain").
		Set("ytdlp_concurrent_downloads = EXCLUDED.ytdlp_concurrent_downloads").
		Set("ytdlp_concurrent_fragments = EXCLUDED.ytdlp_concurrent_fragments").
		Set("appearance_config_json = EXCLUDED.appearance_config_json").
		Set("main_x = EXCLUDED.main_x").
		Set("main_y = EXCLUDED.main_y").
		Set("main_width = EXCLUDED.main_width").
		Set("main_height = EXCLUDED.main_height").
		Set("settings_x = EXCLUDED.settings_x").
		Set("settings_y = EXCLUDED.settings_y").
		Set("settings_width = EXCLUDED.settings_width").
		Set("settings_height = EXCLUDED.settings_height").
		Set("proxy_mode = EXCLUDED.proxy_mode").
		Set("proxy_scheme = EXCLUDED.proxy_scheme").
		Set("proxy_host = EXCLUDED.proxy_host").
		Set("proxy_port = EXCLUDED.proxy_port").
		Set("proxy_username = EXCLUDED.proxy_username").
		Set("proxy_password = EXCLUDED.proxy_password").
		Set("proxy_no_proxy = EXCLUDED.proxy_no_proxy").
		Set("proxy_timeout_seconds = EXCLUDED.proxy_timeout_seconds").
		Set("proxy_tested_at = EXCLUDED.proxy_tested_at").
		Set("proxy_test_success = EXCLUDED.proxy_test_success").
		Set("proxy_test_message = EXCLUDED.proxy_test_message").
		Set("version = EXCLUDED.version").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

func intOrZero(value sql.NullInt64) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int64)
}

func nullInt64(value int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(value), Valid: true}
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}

func nullBool(value bool) sql.NullBool {
	return sql.NullBool{Bool: value, Valid: true}
}

func nullTime(value time.Time) sql.NullTime {
	if value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value, Valid: true}
}

func stringOrEmpty(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func clampWindowDimensionOrDefault(value sql.NullInt64, fallback int, min int) int {
	if !value.Valid {
		return clampWindowDimension(fallback, min)
	}
	return clampWindowDimension(int(value.Int64), min)
}

func clampWindowDimension(value int, min int) int {
	if value <= 0 {
		return min
	}
	if value < min {
		return min
	}
	return value
}

func clampFontSizeOrDefault(value sql.NullInt64) int {
	if !value.Valid {
		return settings.DefaultFontSize
	}
	fontSize := int(value.Int64)
	if fontSize < settings.MinFontSize {
		return settings.MinFontSize
	}
	if fontSize > settings.MaxFontSize {
		return settings.MaxFontSize
	}
	return fontSize
}

func clampPositiveOrDefault(value sql.NullInt64, fallback int) int {
	if !value.Valid {
		return fallback
	}
	val := int(value.Int64)
	if val <= 0 {
		return fallback
	}
	return val
}

func clampResourceSniffMinBytes(value sql.NullInt64) int {
	if !value.Valid {
		return settings.DefaultResourceSniffMinBytes
	}
	val := int(value.Int64)
	if val <= 0 {
		return settings.DefaultResourceSniffMinBytes
	}
	if val > settings.MaxResourceSniffMinBytes {
		return settings.MaxResourceSniffMinBytes
	}
	return val
}

func clampResourceSniffRetain(value sql.NullInt64) int {
	if !value.Valid {
		return settings.DefaultResourceSniffRetain
	}
	val := int(value.Int64)
	if val < settings.MinResourceSniffRetain {
		return settings.MinResourceSniffRetain
	}
	if val > settings.MaxResourceSniffRetain {
		return settings.MaxResourceSniffRetain
	}
	return val
}

func clampYTDLPConcurrentFragments(value sql.NullInt64) int {
	if !value.Valid {
		return settings.DefaultYTDLPConcurrentFragments
	}
	val := int(value.Int64)
	if val <= 0 {
		return settings.DefaultYTDLPConcurrentFragments
	}
	if val > settings.MaxYTDLPConcurrentFragments {
		return settings.MaxYTDLPConcurrentFragments
	}
	return val
}

func clampYTDLPConcurrentDownloads(value sql.NullInt64) int {
	if !value.Valid {
		return settings.DefaultYTDLPConcurrentDownloads
	}
	val := int(value.Int64)
	if val <= 0 {
		return settings.DefaultYTDLPConcurrentDownloads
	}
	if val < settings.MinYTDLPConcurrentDownloads {
		return settings.MinYTDLPConcurrentDownloads
	}
	if val > settings.MaxYTDLPConcurrentDownloads {
		return settings.MaxYTDLPConcurrentDownloads
	}
	return val
}

func boolOrDefault(value sql.NullBool, fallback bool) bool {
	if !value.Valid {
		return fallback
	}
	return value.Bool
}

func parseStringSlice(value sql.NullString) []string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	var entries []string
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return nil
	}
	return entries
}

func parseAnyMap(value sql.NullString) map[string]any {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}
	if len(payload) == 0 {
		return nil
	}
	return payload
}

func jsonStringSlice(values []string) sql.NullString {
	if len(values) == 0 {
		return sql.NullString{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(encoded), Valid: true}
}

func jsonAnyMap(config map[string]any) sql.NullString {
	if len(config) == 0 {
		return sql.NullString{}
	}
	payload, err := json.Marshal(config)
	if err != nil {
		return sql.NullString{}
	}
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}
