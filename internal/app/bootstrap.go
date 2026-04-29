package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	appdefaults "xiadown/internal/app/defaults"
	connectorsservice "xiadown/internal/application/connectors/service"
	dependenciesservice "xiadown/internal/application/dependencies/service"
	appevents "xiadown/internal/application/events"
	fontsservice "xiadown/internal/application/fonts/service"
	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/application/settings/service"
	softwareupdate "xiadown/internal/application/softwareupdate"
	spritesservice "xiadown/internal/application/sprites/service"
	apptelemetry "xiadown/internal/application/telemetry"
	applicationupdate "xiadown/internal/application/update"
	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/settings"
	"xiadown/internal/infrastructure/autostart"
	"xiadown/internal/infrastructure/connectorsrepo"
	"xiadown/internal/infrastructure/dependenciesrepo"
	"xiadown/internal/infrastructure/libraryicons"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/logging"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/proxy"
	"xiadown/internal/infrastructure/settingsrepo"
	"xiadown/internal/infrastructure/spritesrepo"
	"xiadown/internal/infrastructure/telemetryrepo"
	infrastructureupdate "xiadown/internal/infrastructure/update"
	"xiadown/internal/infrastructure/ws"
	presentationhttp "xiadown/internal/presentation/http"
	"xiadown/internal/presentation/i18n"
	"xiadown/internal/presentation/wails"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"go.uber.org/zap"
)

var (
	// AppVersion can be overridden via APP_VERSION env or ldflags "-X xiadown/internal/app.AppVersion=1.2.3".
	AppVersion     = "dev"
	AppName        = "XiaDown"
	AppDescription = "A yt-dlp powered downloader and transcoder."
)

func CreateApplication(assets fs.FS) (*application.App, error) {
	appVersion := resolveVersion(os.Getenv("APP_ENV"))
	startup := currentStartupContext(os.Args[1:])
	appIcon := loadAppIcon(assets)
	trayIcon := loadTrayIcon(assets)
	var windowManager *wails.WindowManager
	var dreamFMPlayer *wails.DreamFMYouTubeMusicPlayer
	var dreamFMLivePlayer *wails.DreamFMYouTubeLivePlayer

	app := application.New(application.Options{
		Name:        AppName,
		Description: AppDescription,
		Icon:        appIcon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			AdditionalBrowserArgs: []string{`--user-agent="` + youtubemusic.WindowsWebViewUserAgent + `"`},
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.dreamapp.xiadown",
			ExitCode: 0,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if windowManager != nil {
					windowManager.HandleSecondInstanceLaunch()
				}
			},
		},
		RawMessageHandler: func(window application.Window, message string, originInfo *application.OriginInfo) {
			if dreamFMPlayer != nil && dreamFMPlayer.HandleRawMessage(window, message, originInfo) {
				return
			}
			if dreamFMLivePlayer != nil && dreamFMLivePlayer.HandleRawMessage(window, message, originInfo) {
				return
			}
		},
	})

	ctx := context.Background()

	database, err := openDatabase(ctx)
	if err != nil {
		return nil, err
	}
	var libraryService *libraryservice.LibraryService
	app.OnShutdown(func() {
		if libraryService != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			stoppedRuns := libraryService.ShutdownActiveRuns(shutdownCtx)
			cancel()
			if stoppedRuns > 0 {
				zap.L().Info("library operation runs stopped on shutdown", zap.Int("count", stoppedRuns))
			}
		}
		if windowManager != nil {
			windowManager.PersistAllBounds()
		}
		_ = database.Close()
	})

	repo := settingsrepo.NewSQLiteSettingsRepository(database.Bun)
	themeProvider := NewAppThemeProvider(app)
	defaultLanguage := i18n.DetectSystemLanguage()
	defaultSettings := appdefaults.WithRandomThemePack(settings.DefaultSettingsWithLanguage(defaultLanguage.String()))
	settingsService := service.NewSettingsService(repo, themeProvider, defaultSettings)

	currentSettings, err := settingsService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	logDir, err := logging.DefaultLogDir()
	if err != nil {
		return nil, err
	}

	proxyManager, err := proxy.NewManager(proxy.Config{
		Mode:     settings.ProxyMode(currentSettings.Proxy.Mode),
		Scheme:   settings.ProxyScheme(currentSettings.Proxy.Scheme),
		Host:     currentSettings.Proxy.Host,
		Port:     currentSettings.Proxy.Port,
		Username: currentSettings.Proxy.Username,
		Password: currentSettings.Proxy.Password,
		NoProxy:  currentSettings.Proxy.NoProxy,
		Timeout:  time.Duration(currentSettings.Proxy.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	telemetryConfig := resolveTelemetryConfig()
	telemetryService := apptelemetry.NewService(
		telemetryrepo.NewSQLiteStateRepository(database.Bun),
		wails.NewTelemetrySignalEmitter(app),
		settingsService,
		telemetryConfig.AppID,
		appVersion,
	)

	appLogger, err := logging.NewLogger(logging.Config{
		Directory:  logDir,
		Level:      settings.LogLevel(currentSettings.LogLevel),
		MaxSizeMB:  currentSettings.LogMaxSizeMB,
		MaxBackups: currentSettings.LogMaxBackups,
		MaxAgeDays: currentSettings.LogMaxAgeDays,
		Compress:   currentSettings.LogCompress,
	})
	if err != nil {
		return nil, err
	}
	app.OnShutdown(func() {
		_ = appLogger.Sync()
	})

	zap.L().Info("application started",
		zap.String("logDir", logDir),
		zap.String("logLevel", currentSettings.LogLevel),
		zap.String("language", currentSettings.Language),
		zap.String("appearance", currentSettings.Appearance),
	)

	autostartManager, err := autostart.NewManager(AppName)
	if err != nil {
		zap.L().Warn("autostart manager unavailable", zap.Error(err))
	}

	eventBus := appevents.NewInMemoryBus()
	serverCtx, serverCancel := context.WithCancel(ctx)
	realtimeServer := ws.NewServer("127.0.0.1:0", eventBus)
	fontService := fontsservice.NewFontService()
	realtimeServer.Handle("/api/library/asset", presentationhttp.NewLibraryAssetHandler())
	realtimeServer.Handle("/api/library/asset/", presentationhttp.NewLibraryAssetHandler())
	if err := realtimeServer.Start(serverCtx); err != nil {
		serverCancel()
		return nil, err
	}
	app.OnShutdown(func() {
		serverCancel()
		_ = realtimeServer.Shutdown(context.Background())
	})

	windowManager, err = wails.NewWindowManager(app, settingsService, appVersion, trayIcon, startup.launchedByAutoStart)
	if err != nil {
		return nil, err
	}

	accentCtx, accentCancel := context.WithCancel(ctx)
	app.OnShutdown(accentCancel)
	startAccentColorWatcher(accentCtx, settingsService, windowManager)

	updateCatalog := buildSoftwareUpdateService(proxyManager)
	updateService, err := buildUpdateService(ctx, proxyManager, eventBus, windowManager, updateCatalog, appVersion)
	if err != nil {
		return nil, err
	}

	connectorsRepo := connectorsrepo.NewSQLiteConnectorRepository(database.Bun)
	connectorsService := connectorsservice.NewConnectorsService(connectorsRepo)
	if err := connectorsService.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	dreamFMPlayer = wails.NewDreamFMYouTubeMusicPlayer(app, windowManager, connectorsService)
	dreamFMLivePlayer = wails.NewDreamFMYouTubeLivePlayer(app, windowManager, connectorsService)

	dependenciesRepo := dependenciesrepo.NewSQLiteDependencyRepository(database.Bun)
	dependenciesService := dependenciesservice.NewDependenciesService(
		dependenciesRepo,
		updateCatalog,
		appVersion,
		dependenciesservice.WithHTTPClientProvider(proxyManager),
	)
	if err := dependenciesService.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	ytMusicClient := youtubemusic.NewClientWithHTTPClientProvider(connectorsService, proxyManager)
	dreamFMImageCache, err := youtubemusic.NewImageCache(proxyManager, youtubemusic.ImageCacheConfig{})
	if err != nil {
		return nil, err
	}
	dreamFMImageHandler := presentationhttp.NewDreamFMImageHandler(dreamFMImageCache)
	realtimeServer.Handle("/api/dreamfm/image", dreamFMImageHandler)
	realtimeServer.Handle("/api/dreamfm/image/", dreamFMImageHandler)
	dreamFMLiveCatalogHandler := presentationhttp.NewDreamFMLiveCatalogHandler(proxyManager)
	realtimeServer.Handle("/api/dreamfm/live/catalog", dreamFMLiveCatalogHandler)
	realtimeServer.Handle("/api/dreamfm/live/catalog/", dreamFMLiveCatalogHandler)
	dreamFMLiveStatusHandler := presentationhttp.NewDreamFMLiveStatusHandler(proxyManager)
	realtimeServer.Handle("/api/dreamfm/live/status", dreamFMLiveStatusHandler)
	realtimeServer.Handle("/api/dreamfm/live/status/", dreamFMLiveStatusHandler)
	dreamFMSearchHandler := presentationhttp.NewDreamFMSearchHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/search", dreamFMSearchHandler)
	realtimeServer.Handle("/api/dreamfm/search/", dreamFMSearchHandler)
	dreamFMLibraryHandler := presentationhttp.NewDreamFMLibraryHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/library", dreamFMLibraryHandler)
	realtimeServer.Handle("/api/dreamfm/library/", dreamFMLibraryHandler)
	dreamFMArtistHandler := presentationhttp.NewDreamFMArtistHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/artist", dreamFMArtistHandler)
	realtimeServer.Handle("/api/dreamfm/artist/", dreamFMArtistHandler)
	dreamFMPlaylistLibraryHandler := presentationhttp.NewDreamFMPlaylistLibraryHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/library/playlist", dreamFMPlaylistLibraryHandler)
	realtimeServer.Handle("/api/dreamfm/library/playlist/", dreamFMPlaylistLibraryHandler)
	dreamFMLyricsHandler := presentationhttp.NewDreamFMLyricsHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/track/lyrics", dreamFMLyricsHandler)
	realtimeServer.Handle("/api/dreamfm/track/lyrics/", dreamFMLyricsHandler)
	dreamFMTrackHandler := presentationhttp.NewDreamFMTrackHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/track", dreamFMTrackHandler)
	realtimeServer.Handle("/api/dreamfm/track/", dreamFMTrackHandler)
	dreamFMTrackFavoriteHandler := presentationhttp.NewDreamFMTrackFavoriteHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/track/favorite", dreamFMTrackFavoriteHandler)
	realtimeServer.Handle("/api/dreamfm/track/favorite/", dreamFMTrackFavoriteHandler)
	dreamFMRadioHandler := presentationhttp.NewDreamFMRadioHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/radio", dreamFMRadioHandler)
	realtimeServer.Handle("/api/dreamfm/radio/", dreamFMRadioHandler)
	dreamFMPlaylistHandler := presentationhttp.NewDreamFMPlaylistHandler(ytMusicClient)
	realtimeServer.Handle("/api/dreamfm/playlist", dreamFMPlaylistHandler)
	realtimeServer.Handle("/api/dreamfm/playlist/", dreamFMPlaylistHandler)

	spritesBaseDir, err := spritesservice.DefaultSpritesBaseDir()
	if err != nil {
		return nil, err
	}
	spriteRepo := spritesrepo.NewSQLiteSpriteRepository(database.Bun)
	spritesService := spritesservice.NewService(
		spritesBaseDir,
		spriteAssets,
		"embedded/sprites",
		filepath.Join("images", "sprites"),
		spritesservice.WithMetadataRepository(spriteRepo),
	)
	if err := spritesService.EnsureBuiltinSprites(ctx); err != nil {
		return nil, err
	}

	libraryRepo := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	moduleConfigRepo := libraryrepo.NewSQLiteModuleConfigRepository(database.Bun)
	fileRepo := libraryrepo.NewSQLiteFileRepository(database.Bun)
	localTrackRepo := libraryrepo.NewSQLiteDreamFMLocalTrackRepository(database.Bun)
	operationRepo := libraryrepo.NewSQLiteOperationRepository(database.Bun)
	operationChunkRepo := libraryrepo.NewSQLiteOperationChunkRepository(database.Bun)
	presetRepo := libraryrepo.NewSQLiteTranscodePresetRepository(database.Bun)
	historyRepo := libraryrepo.NewSQLiteHistoryRepository(database.Bun)
	workspaceStateRepo := libraryrepo.NewSQLiteWorkspaceStateRepository(database.Bun)
	fileEventRepo := libraryrepo.NewSQLiteFileEventRepository(database.Bun)
	subtitleDocumentRepo := libraryrepo.NewSQLiteSubtitleDocumentRepository(database.Bun)
	faviconCache := libraryicons.NewFaviconCacheWithHTTPClientProvider(proxyManager)
	libraryService = libraryservice.NewLibraryService(
		libraryRepo,
		moduleConfigRepo,
		fileRepo,
		localTrackRepo,
		operationRepo,
		operationChunkRepo,
		historyRepo,
		workspaceStateRepo,
		fileEventRepo,
		subtitleDocumentRepo,
		presetRepo,
		settingsService,
		faviconCache,
		dependenciesService,
		proxyManager,
		connectorsService,
		eventBus,
		telemetryService,
	)
	if err := libraryService.EnsureDefaultTranscodePresets(ctx); err != nil {
		return nil, err
	}
	libraryFileMaintenanceHandler := presentationhttp.NewLibraryFileMaintenanceHandler(libraryService)
	realtimeServer.Handle("/api/library/files/", libraryFileMaintenanceHandler)
	dreamFMLocalHandler := presentationhttp.NewDreamFMLocalHandler(libraryService)
	realtimeServer.Handle("/api/dreamfm/local", dreamFMLocalHandler)
	realtimeServer.Handle("/api/dreamfm/local/", dreamFMLocalHandler)

	osNotifications := notifications.New()
	app.RegisterService(application.NewService(wails.NewSettingsHandler(settingsService, windowManager, appLogger, proxyManager, autostartManager, dreamFMPlayer, dreamFMLivePlayer)))
	app.RegisterService(application.NewService(wails.NewConnectorsHandler(connectorsService, telemetryService, dreamFMPlayer, dreamFMLivePlayer)))
	app.RegisterService(application.NewService(wails.NewDependenciesHandler(dependenciesService, windowManager, telemetryService)))
	app.RegisterService(application.NewService(wails.NewLibraryHandler(libraryService)))
	app.RegisterService(application.NewService(wails.NewSystemHandler(fontService)))
	app.RegisterService(application.NewService(wails.NewOSNotificationHandlerWithHTTPClientProvider(osNotifications, app, proxyManager)))
	app.RegisterService(application.NewService(wails.NewRealtimeHandler(realtimeServer)))
	app.RegisterService(application.NewService(wails.NewSpritesHandler(spritesService)))
	app.RegisterService(application.NewService(wails.NewDreamFMPlayerHandler(dreamFMPlayer)))
	app.RegisterService(application.NewService(wails.NewDreamFMLivePlayerHandler(dreamFMLivePlayer)))
	app.RegisterService(application.NewService(wails.NewTelemetryHandler(telemetryService, apptelemetry.AppLaunchContext{
		LaunchedByAutoStart: startup.launchedByAutoStart,
	}, proxyManager)))
	app.RegisterService(application.NewService(wails.NewUpdateHandler(updateService, telemetryService, app)))

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		go func() {
			time.Sleep(500 * time.Millisecond)
			libraryService.RecoverPendingJobs(context.Background())
		}()
		updateService.PublishCurrentState()
		updateService.ScheduleAutoCheck(ctx, 10*time.Second, time.Hour, appVersion)
	})
	app.Event.OnApplicationEvent(events.Common.ThemeChanged, func(_ *application.ApplicationEvent) {
		updated, err := settingsService.GetSettings(ctx)
		if err != nil {
			return
		}
		windowManager.ApplySettings(updated)
	})

	return app, nil
}

func openDatabase(ctx context.Context) (*persistence.Database, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	appDir := filepath.Join(configDir, "xiadown")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, err
	}

	path := filepath.Join(appDir, "data.db")
	return persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
}

func buildSoftwareUpdateService(proxyManager *proxy.Manager) *softwareupdate.Service {
	return softwareupdate.NewService(softwareupdate.ServiceParams{
		CatalogProvider: infrastructureupdate.NewManifestCatalogProviderWithClientProvider(proxyManager, ""),
	})
}

func buildUpdateService(ctx context.Context, proxyManager *proxy.Manager, bus appevents.Bus, notifier applicationupdate.Notifier, catalog *softwareupdate.Service, currentVersion string) (*applicationupdate.Service, error) {
	downloader := infrastructureupdate.NewHTTPDownloaderWithClientProvider(proxyManager)
	installer, err := infrastructureupdate.NewInstaller("")
	if err != nil {
		return nil, err
	}

	service := applicationupdate.NewService(applicationupdate.ServiceParams{
		Catalog:    catalog,
		Downloader: downloader,
		Installer:  installer,
		Bus:        bus,
		Notifier:   notifier,
	})
	service.SetCurrentVersion(currentVersion)
	if _, err := service.RestorePreparedUpdate(ctx); err != nil {
		zap.L().Warn("update: restore prepared update failed", zap.Error(err))
	}
	return service, nil
}

func startAccentColorWatcher(ctx context.Context, settingsService *service.SettingsService, windowManager *wails.WindowManager) {
	initial, err := settingsService.GetSettings(ctx)
	lastAccent := ""
	if err == nil {
		lastAccent = strings.ToLower(strings.TrimSpace(initial.SystemThemeColor))
	}

	ticker := time.NewTicker(2 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, err := settingsService.GetSettings(ctx)
				if err != nil {
					continue
				}
				if !settings.IsSystemThemeColor(current.ThemeColor) {
					continue
				}
				accent := strings.ToLower(strings.TrimSpace(current.SystemThemeColor))
				if accent == "" || accent == lastAccent {
					continue
				}
				lastAccent = accent
				windowManager.ApplySettings(current)
			}
		}
	}()
}

func resolveVersion(env string) string {
	if v := strings.TrimSpace(os.Getenv("APP_VERSION")); v != "" {
		return v
	}
	if env == "dev" || env == "development" {
		return "dev"
	}
	v := strings.TrimSpace(AppVersion)
	if v == "" {
		return "dev"
	}
	return v
}
