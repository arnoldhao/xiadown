package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	appdefaults "xiadown/internal/app/defaults"
	appsessionidentity "xiadown/internal/application/appsessions"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	"xiadown/internal/application/browsercdp"
	dependenciesservice "xiadown/internal/application/dependencies/service"
	"xiadown/internal/application/equalizer"
	appevents "xiadown/internal/application/events"
	fontsservice "xiadown/internal/application/fonts/service"
	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/application/listenlyrics"
	"xiadown/internal/application/listenplayback"
	petsservice "xiadown/internal/application/pets/service"
	"xiadown/internal/application/settings/service"
	softwareupdate "xiadown/internal/application/softwareupdate"
	apptelemetry "xiadown/internal/application/telemetry"
	applicationupdate "xiadown/internal/application/update"
	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/settings"
	"xiadown/internal/infrastructure/appsessionsrepo"
	"xiadown/internal/infrastructure/autostart"
	"xiadown/internal/infrastructure/dependenciesrepo"
	"xiadown/internal/infrastructure/equalizeraudio"
	"xiadown/internal/infrastructure/equalizerstore"
	"xiadown/internal/infrastructure/libraryicons"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/listenplaybackstore"
	"xiadown/internal/infrastructure/logging"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/petsrepo"
	"xiadown/internal/infrastructure/proxy"
	"xiadown/internal/infrastructure/settingsrepo"
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
	if err := browsercdp.CleanupStaleRuntimes(context.Background()); err != nil {
		zap.L().Warn("cleanup stale browser runtimes", zap.Error(err))
	}
	appIcon := loadAppIcon(assets)
	trayIcon := loadTrayIcon(assets)
	var windowManager *wails.WindowManager
	var listenPlayer *wails.ListenYouTubeMusicPlayer
	var listenLivePlayer *wails.ListenYouTubeLivePlayer
	var telemetryShutdown func()
	var listenPlaybackSnapshotUnsubscribe func()
	var equalizerPlaybackUnsubscribe func()
	var equalizerService *equalizer.Service

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
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.dreamapp.xiadown",
			ExitCode: 0,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if windowManager != nil {
					windowManager.HandleSecondInstanceLaunch()
				}
			},
		},
		ShouldQuit: func() bool {
			if windowManager != nil {
				windowManager.PrepareQuit()
			}
			return true
		},
		RawMessageHandler: func(window application.Window, message string, originInfo *application.OriginInfo) {
			if listenPlayer != nil && listenPlayer.HandleRawMessage(window, message, originInfo) {
				return
			}
			if listenLivePlayer != nil && listenLivePlayer.HandleRawMessage(window, message, originInfo) {
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
	var petsService *petsservice.Service
	var appSessionsService *appsessionsservice.AppSessionsService
	app.OnShutdown(func() {
		if libraryService != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			stoppedSniffs := libraryService.ShutdownResourceSniffSessions(shutdownCtx)
			stoppedRuns := libraryService.ShutdownActiveRuns(shutdownCtx)
			cancel()
			if stoppedSniffs > 0 {
				zap.L().Info("resource sniff sessions stopped on shutdown", zap.Int("count", stoppedSniffs))
			}
			if stoppedRuns > 0 {
				zap.L().Info("library operation runs stopped on shutdown", zap.Int("count", stoppedRuns))
			}
		}
		if windowManager != nil {
			windowManager.PrepareQuit()
		}
		if petsService != nil {
			petsService.ShutdownOnlinePetImportSessions()
		}
		if appSessionsService != nil {
			stoppedAppSessions := appSessionsService.ShutdownSessions()
			if stoppedAppSessions > 0 {
				zap.L().Info("app session browser sessions stopped on shutdown", zap.Int("count", stoppedAppSessions))
			}
		}
		if telemetryShutdown != nil {
			telemetryShutdown()
		}
		if listenPlaybackSnapshotUnsubscribe != nil {
			listenPlaybackSnapshotUnsubscribe()
		}
		if equalizerPlaybackUnsubscribe != nil {
			equalizerPlaybackUnsubscribe()
		}
		if equalizerService != nil {
			equalizerService.Stop()
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

	appSessionProvider := wails.NewNativeAppSessionProvider(app)
	appSessionsRepo := appsessionsrepo.NewSQLiteRepository(database.Bun)
	appSessionsService = appsessionsservice.NewAppSessionsService(
		appSessionsRepo,
		appsessionsservice.WithProvider(appSessionProvider),
		appsessionsservice.WithAccountFetcher(newAppSessionAccountFetcher(proxyManager)),
	)
	if err := appSessionsService.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	listenPlayer = wails.NewListenYouTubeMusicPlayer(app, windowManager, appSessionsService)
	_ = listenPlayer.SetPlaybackAudioQuality(currentSettings.PlaybackAudioQuality)
	listenLivePlayer = wails.NewListenYouTubeLivePlayer(app, windowManager, appSessionsService)

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
	ytMusicClient := youtubemusic.NewClientWithHTTPClientProvider(appSessionsService, proxyManager)
	ytMusicClient.SetUserAgent(appsessionidentity.HTTPUserAgent("youtube"))
	listenPlaybackStore, err := listenplaybackstore.DefaultJSONSessionStore()
	if err != nil {
		return nil, err
	}
	listenPlaybackService := listenplayback.NewPlayerService(
		wails.NewListenPlaybackTransport(listenPlayer),
		listenplayback.WithLibraryClient(wails.NewListenPlaybackLibraryClient(ytMusicClient)),
		listenplayback.WithSessionStore(listenPlaybackStore),
		listenplayback.WithUserInteractionUnlocked(),
	)
	listenLyricsService := listenlyrics.NewService(wails.NewListenLyricsClient(ytMusicClient))
	listenPlayer.SetPlaybackService(listenPlaybackService)
	_, _ = listenPlaybackService.RestorePlaybackSession(ctx)
	listenPlaybackSnapshotUnsubscribe = wails.NewListenPlaybackSnapshotEmitter(app, listenPlaybackService)
	equalizerStore, err := equalizerstore.DefaultJSONStore()
	if err != nil {
		return nil, err
	}
	equalizerService = equalizer.NewService(equalizerStore, equalizeraudio.NewEngine(
		equalizeraudio.WithTargetProcessProvider(func() uint32 {
			if listenPlayer == nil {
				return 0
			}
			return listenPlayer.EqualizerAudioProcessID()
		}),
	))
	equalizerPlaybackUnsubscribe = listenPlaybackService.Subscribe(func(snapshot listenplayback.Snapshot) {
		active := snapshot.State == listenplayback.PlaybackStatePlaying
		equalizerService.ObservePlayback(active, snapshot.Progress)
	})
	listenLiveCatalogHandler := presentationhttp.NewListenLiveCatalogHandler(proxyManager)
	realtimeServer.Handle("/api/listen/live/catalog", listenLiveCatalogHandler)
	realtimeServer.Handle("/api/listen/live/catalog/", listenLiveCatalogHandler)
	listenLiveStatusHandler := presentationhttp.NewListenLiveStatusHandler(proxyManager)
	realtimeServer.Handle("/api/listen/live/status", listenLiveStatusHandler)
	realtimeServer.Handle("/api/listen/live/status/", listenLiveStatusHandler)
	listenLivePreviewHandler := presentationhttp.NewListenLivePreviewHandler(proxyManager)
	realtimeServer.Handle("/api/listen/live/preview", listenLivePreviewHandler)
	realtimeServer.Handle("/api/listen/live/preview/", listenLivePreviewHandler)
	listenSearchHandler := presentationhttp.NewListenSearchHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/search", listenSearchHandler)
	realtimeServer.Handle("/api/listen/search/", listenSearchHandler)
	listenLibraryHandler := presentationhttp.NewListenLibraryHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/library", listenLibraryHandler)
	realtimeServer.Handle("/api/listen/library/", listenLibraryHandler)
	listenArtistHandler := presentationhttp.NewListenArtistHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/artist", listenArtistHandler)
	realtimeServer.Handle("/api/listen/artist/", listenArtistHandler)
	listenPlaylistLibraryHandler := presentationhttp.NewListenPlaylistLibraryHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/library/playlist", listenPlaylistLibraryHandler)
	realtimeServer.Handle("/api/listen/library/playlist/", listenPlaylistLibraryHandler)
	listenLyricsHandler := presentationhttp.NewListenLyricsHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/track/lyrics", listenLyricsHandler)
	realtimeServer.Handle("/api/listen/track/lyrics/", listenLyricsHandler)
	listenTrackHandler := presentationhttp.NewListenTrackHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/track", listenTrackHandler)
	realtimeServer.Handle("/api/listen/track/", listenTrackHandler)
	listenTrackFavoriteHandler := presentationhttp.NewListenTrackFavoriteHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/track/favorite", listenTrackFavoriteHandler)
	realtimeServer.Handle("/api/listen/track/favorite/", listenTrackFavoriteHandler)
	listenRadioHandler := presentationhttp.NewListenRadioHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/radio", listenRadioHandler)
	realtimeServer.Handle("/api/listen/radio/", listenRadioHandler)
	listenPlaylistHandler := presentationhttp.NewListenPlaylistHandler(ytMusicClient)
	realtimeServer.Handle("/api/listen/playlist", listenPlaylistHandler)
	realtimeServer.Handle("/api/listen/playlist/", listenPlaylistHandler)

	petsBaseDir, err := petsservice.DefaultPetsBaseDir()
	if err != nil {
		return nil, err
	}
	petRepo := petsrepo.NewSQLitePetRepository(database.Bun)
	petsService = petsservice.NewService(
		petsBaseDir,
		petAssets,
		"embedded/pets",
		filepath.Join("images", "pets"),
		petsservice.WithMetadataRepository(petRepo),
		petsservice.WithSettingsReader(settingsService),
	)
	if err := petsService.EnsureBuiltinPets(ctx); err != nil {
		return nil, err
	}

	libraryRepo := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	moduleConfigRepo := libraryrepo.NewSQLiteModuleConfigRepository(database.Bun)
	fileRepo := libraryrepo.NewSQLiteFileRepository(database.Bun)
	localTrackRepo := libraryrepo.NewSQLiteListenLocalTrackRepository(database.Bun)
	listenLiveChannelRepo := libraryrepo.NewSQLiteListenLiveChannelRepository(database.Bun)
	operationRepo := libraryrepo.NewSQLiteOperationRepository(database.Bun)
	externalProcessRepo := libraryrepo.NewSQLiteExternalProcessRepository(database.Bun)
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
		externalProcessRepo,
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
		appSessionsService,
		eventBus,
		telemetryService,
	)
	if err := libraryService.EnsureDefaultTranscodePresets(ctx); err != nil {
		return nil, err
	}
	libraryFileMaintenanceHandler := presentationhttp.NewLibraryFileMaintenanceHandler(libraryService)
	realtimeServer.Handle("/api/library/files/", libraryFileMaintenanceHandler)
	resourceSniffPreviewHandler := presentationhttp.NewResourceSniffPreviewHandler(libraryService)
	realtimeServer.Handle("/api/sniff/resource-preview/", resourceSniffPreviewHandler)
	listenLocalHandler := presentationhttp.NewListenLocalHandler(libraryService)
	realtimeServer.Handle("/api/listen/local", listenLocalHandler)
	realtimeServer.Handle("/api/listen/local/", listenLocalHandler)
	listenLiveUserCatalogHandler := presentationhttp.NewListenLiveUserCatalogHandler(listenLiveChannelRepo)
	realtimeServer.Handle("/api/listen/live/user-catalog", listenLiveUserCatalogHandler)
	realtimeServer.Handle("/api/listen/live/user-catalog/", listenLiveUserCatalogHandler)

	osNotifications := notifications.New()
	settingsHandler := wails.NewSettingsHandler(settingsService, windowManager, appLogger, proxyManager, autostartManager, libraryService, listenPlayer, listenLivePlayer)
	app.RegisterService(application.NewService(settingsHandler))
	app.RegisterService(application.NewService(wails.NewAppSessionsHandler(appSessionsService, windowManager, telemetryService, listenPlayer, listenLivePlayer)))
	app.RegisterService(application.NewService(wails.NewDependenciesHandler(dependenciesService, windowManager, telemetryService)))
	app.RegisterService(application.NewService(wails.NewLibraryHandler(libraryService, windowManager)))
	app.RegisterService(application.NewService(wails.NewSystemHandler(fontService)))
	app.RegisterService(application.NewService(wails.NewOSNotificationHandlerWithHTTPClientProvider(osNotifications, app, proxyManager)))
	app.RegisterService(application.NewService(wails.NewRealtimeHandler(realtimeServer)))
	app.RegisterService(application.NewService(wails.NewPetsHandler(petsService)))
	app.RegisterService(application.NewService(wails.NewEqualizerHandler(equalizerService)))
	app.RegisterService(application.NewService(wails.NewListenPlayerHandler(listenPlayer, listenPlaybackService)))
	app.RegisterService(application.NewService(wails.NewListenLyricsHandler(listenLyricsService)))
	app.RegisterService(application.NewService(wails.NewListenLivePlayerHandler(listenLivePlayer)))
	telemetryHandler := wails.NewTelemetryHandler(telemetryService, apptelemetry.AppLaunchContext{
		LaunchedByAutoStart: startup.launchedByAutoStart,
	}, proxyManager)
	telemetryShutdown = func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := telemetryHandler.FlushSessionSummaryForShutdown(shutdownCtx); err != nil {
			zap.L().Debug("telemetry: shutdown session summary failed", zap.Error(err))
		}
	}
	app.RegisterService(application.NewService(telemetryHandler))
	app.RegisterService(application.NewService(wails.NewUpdateHandler(updateService, telemetryService, app)))

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		windowManager.MarkApplicationStarted()
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
