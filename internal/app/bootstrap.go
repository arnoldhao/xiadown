package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	appdefaults "xiadown/internal/app/defaults"
	appsessionidentity "xiadown/internal/application/appsessions"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/browserprofile"
	dependenciesservice "xiadown/internal/application/dependencies/service"
	"xiadown/internal/application/equalizer"
	appevents "xiadown/internal/application/events"
	fontsservice "xiadown/internal/application/fonts/service"
	libraryaccessauth "xiadown/internal/application/library/access"
	"xiadown/internal/application/library/catalogaudit"
	libraryservice "xiadown/internal/application/library/service"
	libraryaccessservice "xiadown/internal/application/libraryaccess"
	applicationlibrarybackup "xiadown/internal/application/librarybackup"
	applicationlibraryimport "xiadown/internal/application/libraryimport"
	"xiadown/internal/application/listenlyrics"
	"xiadown/internal/application/listenplayback"
	petsservice "xiadown/internal/application/pets/service"
	applicationrss "xiadown/internal/application/rss"
	"xiadown/internal/application/settings/service"
	softwareupdate "xiadown/internal/application/softwareupdate"
	apptelemetry "xiadown/internal/application/telemetry"
	applicationupdate "xiadown/internal/application/update"
	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/application/youtubeworkspace"
	domainlibrary "xiadown/internal/domain/library"
	domainrss "xiadown/internal/domain/rss"
	"xiadown/internal/domain/settings"
	"xiadown/internal/infrastructure/appsessionprofile"
	"xiadown/internal/infrastructure/appsessionsrepo"
	"xiadown/internal/infrastructure/appsessionvault"
	"xiadown/internal/infrastructure/autostart"
	"xiadown/internal/infrastructure/dependenciesrepo"
	"xiadown/internal/infrastructure/discovery"
	"xiadown/internal/infrastructure/equalizeraudio"
	"xiadown/internal/infrastructure/equalizerstore"
	"xiadown/internal/infrastructure/firewall"
	"xiadown/internal/infrastructure/libraryaccessrepo"
	infrastructurelibrarybackup "xiadown/internal/infrastructure/librarybackup"
	"xiadown/internal/infrastructure/libraryicons"
	infrastructurelibraryimport "xiadown/internal/infrastructure/libraryimportrepo"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/libraryserver"
	"xiadown/internal/infrastructure/listenplaybackstore"
	"xiadown/internal/infrastructure/locallyricsreader"
	"xiadown/internal/infrastructure/logging"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/petsrepo"
	"xiadown/internal/infrastructure/proxy"
	"xiadown/internal/infrastructure/rssrepo"
	"xiadown/internal/infrastructure/settingsrepo"
	"xiadown/internal/infrastructure/tailscale"
	"xiadown/internal/infrastructure/telemetryrepo"
	infrastructureupdate "xiadown/internal/infrastructure/update"
	"xiadown/internal/infrastructure/ws"
	presentationhttp "xiadown/internal/presentation/http"
	"xiadown/internal/presentation/http/libraryapi"
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

const productionSingleInstanceID = "com.dreamapp.xiadown"

func CreateApplication(assets fs.FS) (*application.App, error) {
	appVersion := resolveVersion(os.Getenv("APP_ENV"))
	startup := currentStartupContext(os.Args[1:])
	if err := browsercdp.CleanupStaleRuntimes(context.Background()); err != nil {
		zap.L().Warn("cleanup stale browser runtimes",
			safeStationErrorLogFields("browser_runtime_cleanup_failed", err)...,
		)
	}
	if err := browserprofile.CleanupStaleSnapshots(context.Background()); err != nil {
		zap.L().Warn("cleanup stale browser profile snapshots",
			safeStationErrorLogFields("browser_profile_cleanup_failed", err)...,
		)
	}
	appIcon := loadAppIcon(assets)
	startupIcon := loadStartupIcon(assets)
	trayIcon := loadTrayIcon(assets)
	var windowManager *wails.WindowManager
	var listenPlayer *wails.ListenYouTubeMusicPlayer
	var listenLivePlayer *wails.ListenYouTubeLivePlayer
	var rssVideoPlayerHandler *wails.RSSVideoPlayerHandler
	var rssVideoPlayerRawHandler *wails.RSSVideoPlayerRawMessageHandler
	var rssSitePlayerHandler *wails.RSSSitePlayerHandler
	var localMediaTransport *wails.NativeLocalMediaWebviewTransport
	var localMediaBackend *listenplayback.NativeLocalMediaBackend
	var playbackCoordinatorHandler *wails.PlaybackCoordinatorHandler
	var localMediaCoordinatorUnsubscribe func()
	var streamCoordinatorUnsubscribe func()
	var youtubeCoordinatorUnsubscribe func()
	var youtubeMusicCoordinatorUnsubscribe func()
	var youtubeMusicCoordinatorCancel context.CancelFunc
	var listenPlaybackSnapshotUnsubscribe func()
	var equalizerPlaybackUnsubscribe func()
	var equalizerService *equalizer.Service
	var publicServer *libraryserver.Server
	var serverCancel context.CancelFunc
	var startLibraryAccessReconciler func()
	var waitLibraryAccessReconciler func(context.Context) error

	// The gateway endpoint must exist before application.New: WebView2 fixes
	// browser proxy arguments when its shared environment is created. Start on
	// an explicit direct policy, then apply the persisted policy before any
	// pending Wails window is allowed to run.
	proxyManager, err := proxy.NewManager(proxy.Config{
		Mode:    settings.ProxyModeNone,
		Scheme:  settings.ProxySchemeHTTP,
		NoProxy: []string{"localhost", "127.0.0.1", "::1"},
		Timeout: time.Duration(settings.DefaultProxyTimeoutSeconds) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	if devServerURL := strings.TrimSpace(os.Getenv("FRONTEND_DEVSERVER_URL")); devServerURL != "" {
		if err := proxyManager.RegisterInternalLoopbackURL(devServerURL); err != nil {
			return nil, fmt.Errorf("register frontend development endpoint: %w", err)
		}
	}
	proxyManagerOwnedByApp := false
	defer func() {
		if !proxyManagerOwnedByApp {
			_ = proxyManager.Close()
		}
	}()
	webViewNetworkRoute := wails.NewWebViewNetworkRoute(proxyManager)
	webView2BrowserArguments, err := webViewNetworkRoute.WebView2BrowserArguments()
	if err != nil {
		return nil, err
	}

	app := application.New(application.Options{
		Name:        AppName,
		Description: AppDescription,
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(webViewNetworkRoute),
		},
		Logger: logging.NewSlogLogger(),
		ErrorHandler: func(err error) {
			zap.L().Error("wails runtime error",
				safeStationErrorLogFields("wails_runtime_error", err)...,
			)
		},
		PanicHandler: func(details *application.PanicDetails) {
			zap.L().Error("wails runtime panic",
				safeStationTextLogFields("wails_runtime_panic", fmt.Sprint(details))...,
			)
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			AdditionalBrowserArgs: webView2BrowserArguments,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceUniqueID(appVersion),
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
			if localMediaTransport != nil && localMediaTransport.HandleRawMessage(window, message, originInfo) {
				return
			}
			if rssVideoPlayerRawHandler != nil && rssVideoPlayerRawHandler.HandleRawMessage(window, message, originInfo) {
				return
			}
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
	databasePath, backupDirectory, err := libraryPersistencePaths()
	if err != nil {
		return nil, err
	}
	backupManager, err := infrastructurelibrarybackup.NewManager(infrastructurelibrarybackup.Config{
		DB: database.SQL, DatabasePath: databasePath, BackupDirectory: backupDirectory,
		AppName: AppName, AppVersion: appVersion,
	})
	if err != nil {
		return nil, err
	}
	libraryBackupService := applicationlibrarybackup.NewService(backupManager)
	var libraryService *libraryservice.LibraryService
	var petsService *petsservice.Service
	var appSessionsService *appsessionsservice.AppSessionsService
	app.OnShutdown(func() {
		shutdownCtx, cancelIngress := context.WithTimeout(context.Background(), 10*time.Second)
		if err := quiesceLibraryIngress(shutdownCtx, libraryService, publicServer, serverCancel); err != nil {
			zap.L().Warn("shutdown Library public API", safeStationErrorLogFields("paired_api_shutdown_failed", err)...)
		}
		cancelIngress()
		if waitLibraryAccessReconciler != nil {
			reconcilerWaitCtx, cancelReconcilerWait := context.WithTimeout(context.Background(), 3*time.Second)
			if err := waitLibraryAccessReconciler(reconcilerWaitCtx); err != nil {
				zap.L().Warn("wait for Library access reconciler shutdown", safeStationErrorLogFields("paired_api_reconciler_shutdown_failed", err)...)
			}
			cancelReconcilerWait()
		}
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
		if rssVideoPlayerHandler != nil {
			_ = rssVideoPlayerHandler.Shutdown()
		}
		if rssSitePlayerHandler != nil {
			_ = rssSitePlayerHandler.Shutdown()
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
		if listenPlaybackSnapshotUnsubscribe != nil {
			listenPlaybackSnapshotUnsubscribe()
		}
		if localMediaCoordinatorUnsubscribe != nil {
			localMediaCoordinatorUnsubscribe()
		}
		if streamCoordinatorUnsubscribe != nil {
			streamCoordinatorUnsubscribe()
		}
		if youtubeCoordinatorUnsubscribe != nil {
			youtubeCoordinatorUnsubscribe()
		}
		if youtubeMusicCoordinatorUnsubscribe != nil {
			youtubeMusicCoordinatorUnsubscribe()
		}
		if youtubeMusicCoordinatorCancel != nil {
			youtubeMusicCoordinatorCancel()
		}
		if playbackCoordinatorHandler != nil {
			wails.ShutdownPlaybackCoordinatorHandler(playbackCoordinatorHandler)
		}
		if localMediaBackend != nil {
			_ = localMediaBackend.Close()
		}
		if equalizerPlaybackUnsubscribe != nil {
			equalizerPlaybackUnsubscribe()
		}
		if equalizerService != nil {
			equalizerService.Stop()
		}
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

	if err := proxyManager.Apply(proxy.Config{
		Mode:     settings.ProxyMode(currentSettings.Proxy.Mode),
		Scheme:   settings.ProxyScheme(currentSettings.Proxy.Scheme),
		Host:     currentSettings.Proxy.Host,
		Port:     currentSettings.Proxy.Port,
		Username: currentSettings.Proxy.Username,
		Password: currentSettings.Proxy.Password,
		NoProxy:  currentSettings.Proxy.NoProxy,
		Timeout:  time.Duration(currentSettings.Proxy.TimeoutSeconds) * time.Second,
	}); err != nil {
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

	zap.L().Info("application started", safeApplicationStartedLogFields(
		logDir,
		currentSettings.LogLevel,
		currentSettings.Language,
		currentSettings.Appearance,
		currentSettings.Proxy.Mode,
		proxyManager.Generation(),
		proxyManager.GatewayURL(),
	)...)

	autostartManager, err := autostart.NewManager(AppName)
	if err != nil {
		zap.L().Warn("autostart manager unavailable",
			safeStationErrorLogFields("autostart_manager_unavailable", err)...,
		)
	}

	eventBus := appevents.NewInMemoryBus()
	serverCtx, cancelServers := context.WithCancel(ctx)
	serverCancel = cancelServers
	realtimeServer := ws.NewServer("127.0.0.1:0", eventBus)
	fontService := fontsservice.NewFontService()
	realtimeServer.Handle("/api/library/asset", presentationhttp.NewLibraryAssetHandler())
	realtimeServer.Handle("/api/library/asset/", presentationhttp.NewLibraryAssetHandler())
	if err := realtimeServer.Start(serverCtx); err != nil {
		serverCancel()
		return nil, err
	}
	if err := proxyManager.RegisterInternalLoopbackURL(realtimeServer.HTTPURL()); err != nil {
		serverCancel()
		_ = realtimeServer.Shutdown(context.Background())
		return nil, fmt.Errorf("register realtime network endpoint: %w", err)
	}
	app.OnShutdown(func() {
		serverCancel()
		_ = realtimeServer.Shutdown(context.Background())
	})

	windowManager, err = wails.NewWindowManager(app, settingsService, appVersion, startupIcon, trayIcon, startup.launchedByAutoStart)
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

	appSessionVault := appsessionvault.New(database.Bun)
	appSessionProvider := wails.NewNativeAppSessionProvider(app, appSessionVault)
	appSessionsRepo := appsessionsrepo.NewSQLiteRepository(database.Bun)
	appSessionsService = appsessionsservice.NewAppSessionsService(
		appSessionsRepo,
		appsessionsservice.WithProvider(appSessionProvider),
		appsessionsservice.WithAccountFetcher(newAppSessionAccountFetcher(proxyManager)),
		appsessionsservice.WithBrowserProfileReader(appsessionprofile.New(proxyManager)),
		appsessionsservice.WithImportCommitter(appSessionVault),
	)
	if err := appSessionsService.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	listenPlayer = wails.NewListenYouTubeMusicPlayer(app, windowManager, appSessionsService)
	_ = listenPlayer.SetPlaybackAudioQuality(currentSettings.PlaybackAudioQuality)
	listenLivePlayer = wails.NewListenYouTubeLivePlayer(app, windowManager, appSessionsService)
	rssVideoPlayerHandler = wails.NewRSSVideoPlayerHandler(
		app,
		windowManager,
		applicationrss.NewVideoPlayerService(appSessionsService, proxyManager),
	)
	rssVideoPlayerRawHandler = wails.NewRSSVideoPlayerRawMessageHandler(rssVideoPlayerHandler)
	rssSitePlayerHandler = wails.NewRSSSitePlayerHandler(
		app,
		windowManager,
		applicationrss.NewSitePlayerService(appSessionsService),
	)

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
	youtubeWorkspaceService := youtubeworkspace.NewService(appSessionsService, proxyManager)
	youtubeWorkspaceService.SetUserAgent(appsessionidentity.HTTPUserAgent("youtube"))
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
	listenLyricsService := listenlyrics.NewService(
		wails.NewListenLyricsClient(ytMusicClient, locallyricsreader.New()),
	)
	listenPlayer.SetPlaybackService(listenPlaybackService)
	_, _ = listenPlaybackService.RestorePlaybackSession(ctx)
	listenPlaybackSnapshotUnsubscribe = wails.NewListenPlaybackSnapshotEmitter(app, listenPlaybackService)
	youTubeMusicPlaybackBackend := listenplayback.NewPlayerServiceBackend(listenPlaybackService)
	localMediaTransport = wails.NewNativeLocalMediaWebviewTransport(app, realtimeServer.HTTPURL())
	localMediaBackend = listenplayback.NewNativeLocalMediaBackend(localMediaTransport)
	streamPlaybackBackend := wails.NewListenLivePlayerBackend(listenplayback.PlaybackProviderStream, listenLivePlayer)
	youtubePlaybackBackend := wails.NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, listenLivePlayer)
	playbackCoordinator, err := listenplayback.NewPlaybackCoordinator(
		youTubeMusicPlaybackBackend,
		localMediaBackend,
		streamPlaybackBackend,
		youtubePlaybackBackend,
	)
	if err != nil {
		return nil, err
	}
	localMediaCoordinatorUnsubscribe = localMediaBackend.Subscribe(func(event listenplayback.PlaybackBackendEvent) {
		playbackCoordinator.ObserveBackendEvent(event)
	})
	streamCoordinatorUnsubscribe = streamPlaybackBackend.Subscribe(func(event listenplayback.PlaybackBackendEvent) {
		playbackCoordinator.ObserveBackendEvent(event)
	})
	youtubeCoordinatorUnsubscribe = youtubePlaybackBackend.Subscribe(func(event listenplayback.PlaybackBackendEvent) {
		playbackCoordinator.ObserveBackendEvent(event)
	})
	youtubeObservationCtx, cancelYouTubeObservation := context.WithCancel(ctx)
	youtubeMusicCoordinatorCancel = cancelYouTubeObservation
	youtubeSnapshotReady := make(chan struct{}, 1)
	youtubeMusicCoordinatorUnsubscribe = youTubeMusicPlaybackBackend.Subscribe(func(listenplayback.PlaybackSnapshot) {
		select {
		case youtubeSnapshotReady <- struct{}{}:
		default:
		}
	})
	go func() {
		for {
			select {
			case <-youtubeObservationCtx.Done():
				return
			case <-youtubeSnapshotReady:
				if _, err := playbackCoordinator.SynchronizeBackendSnapshot(
					youtubeObservationCtx,
					listenplayback.PlaybackProviderYouTubeMusic,
				); err != nil {
					zap.L().Debug("synchronize legacy YouTube Music playback",
						safeStationErrorLogFields("legacy_youtube_music_sync_failed", err)...,
					)
				}
			}
		}
	}()
	playbackCoordinatorHandler = wails.NewPlaybackCoordinatorHandler(app, playbackCoordinator)
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
		petsservice.WithNetworkGateway(proxyManager),
	)
	if err := petsService.EnsureBuiltinPets(ctx); err != nil {
		return nil, err
	}

	libraryRepo := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	moduleConfigRepo := libraryrepo.NewSQLiteModuleConfigRepository(database.Bun)
	fileRepo := libraryrepo.NewSQLiteFileRepository(database.Bun)
	localTrackRepo := libraryrepo.NewSQLiteListenLocalTrackRepository(database.Bun)
	localPlaylistRepo := libraryrepo.NewSQLiteListenLocalPlaylistRepository(database.Bun)
	localMusicMembershipRepo := libraryrepo.NewSQLiteListenLocalMusicMembershipRepository(database.Bun)
	listenLiveChannelRepo := libraryrepo.NewSQLiteListenLiveChannelRepository(database.Bun)
	rssRepository := rssrepo.NewSQLiteRepository(database.Bun)
	rssService := applicationrss.NewService(rssRepository, proxyManager)
	rssRefreshCtx, cancelRSSRefresh := context.WithCancel(ctx)
	app.OnShutdown(cancelRSSRefresh)
	go rssService.Run(rssRefreshCtx, 12*time.Second, 30*time.Minute)
	operationRepo := libraryrepo.NewSQLiteOperationRepository(database.Bun)
	externalProcessRepo := libraryrepo.NewSQLiteExternalProcessRepository(database.Bun)
	operationChunkRepo := libraryrepo.NewSQLiteOperationChunkRepository(database.Bun)
	presetRepo := libraryrepo.NewSQLiteTranscodePresetRepository(database.Bun)
	historyRepo := libraryrepo.NewSQLiteHistoryRepository(database.Bun)
	workspaceStateRepo := libraryrepo.NewSQLiteWorkspaceStateRepository(database.Bun)
	fileEventRepo := libraryrepo.NewSQLiteFileEventRepository(database.Bun)
	subtitleDocumentRepo := libraryrepo.NewSQLiteSubtitleDocumentRepository(database.Bun)
	catalogBackfill := libraryservice.NewLegacyCatalogBackfillService(
		libraryRepo,
		fileRepo,
		libraryrepo.NewSQLiteCatalogBackfillWriter(database.Bun),
	)
	backfillResult, err := catalogBackfill.Run(ctx)
	if err != nil {
		return nil, err
	}
	catalogID := libraryservice.DefaultLibraryCatalogID()
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE rss_workspaces
SET catalog_id = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND catalog_id <> ?
`, catalogID, domainrss.DefaultWorkspaceID, catalogID); err != nil {
		return nil, err
	}
	catalogRepo := libraryrepo.NewSQLiteCatalogRepository(database.Bun)
	if backfillResult.CatalogID == "" {
		createdAt := time.Now().UTC()
		catalog, catalogErr := domainlibrary.NewCatalog(domainlibrary.CatalogParams{
			ID: catalogID, Name: "Library", Status: string(domainlibrary.CatalogStatusActive),
			IsDefault: true, CreatedAt: &createdAt, UpdatedAt: &createdAt,
		})
		if catalogErr != nil {
			return nil, catalogErr
		}
		if catalogErr := catalogRepo.Save(ctx, catalog); catalogErr != nil {
			return nil, catalogErr
		}
	}
	catalogAuditor := libraryrepo.NewSQLiteCatalogAuditor(database.Bun)
	catalogItemRepo := libraryrepo.NewSQLiteCatalogItemRepository(database.Bun)
	catalogAssetRepo := libraryrepo.NewSQLiteItemAssetRepository(database.Bun)
	catalogRootRepo := libraryrepo.NewSQLiteStorageRootRepository(database.Bun)
	catalogCollectionRepo := libraryrepo.NewSQLiteCatalogCollectionRepository(database.Bun)
	catalogTagRepo := libraryrepo.NewSQLiteCatalogTagRepository(database.Bun)
	catalogUserStateRepo := libraryrepo.NewSQLiteUserStateRepository(database.Bun)
	catalogChangeRepo := libraryrepo.NewSQLiteCatalogChangeRepository(database.Bun)
	catalogDeviceGrantRepo := libraryrepo.NewSQLiteDeviceGrantRepository(database.Bun)
	catalogService := libraryservice.NewCatalogService(
		catalogRepo, catalogItemRepo, catalogAssetRepo, fileRepo, catalogRootRepo,
		catalogCollectionRepo, catalogTagRepo, catalogUserStateRepo,
		libraryrepo.NewSQLiteCatalogMutationRepository(database.Bun), catalogAuditor, catalogChangeRepo,
	)
	videoThumbnailCache := ""
	if cacheBase, cacheErr := os.UserCacheDir(); cacheErr != nil {
		zap.L().Warn("Library video thumbnail cache unavailable",
			safeStationErrorLogFields("library_video_thumbnail_cache_directory_failed", cacheErr)...,
		)
	} else {
		videoThumbnailCache = libraryVideoThumbnailCacheDirectory(cacheBase, appVersion)
	}
	catalogVideoThumbnails := libraryservice.NewCatalogVideoThumbnailService(
		catalogItemRepo, catalogAssetRepo, fileRepo, dependenciesService, videoThumbnailCache,
	)
	realtimeServer.Handle(
		"/api/library/video-thumbnail/",
		presentationhttp.NewCatalogVideoThumbnailHandler(catalogVideoThumbnails),
	)
	faviconCache := libraryicons.NewFaviconCacheWithHTTPClientProvider(proxyManager)
	libraryService = libraryservice.NewLibraryService(
		libraryRepo,
		moduleConfigRepo,
		fileRepo,
		localTrackRepo,
		localPlaylistRepo,
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
	)
	libraryService.SetDatabaseIntegrityStatusProvider(func() (state, checkedAt, detail string) {
		status := persistence.InspectSQLiteIntegrityStatus(databasePath)
		return status.State, status.CheckedAt, status.Detail
	})
	libraryService.SetCatalogProjectionRunner(catalogBackfill)
	libraryService.SetListenLocalCatalogMetadataSynchronizer(catalogService)
	libraryService.SetListenLocalMusicMembershipRepository(localMusicMembershipRepo)
	if err := libraryService.EnsureDefaultTranscodePresets(ctx); err != nil {
		return nil, err
	}
	libraryImportService := applicationlibraryimport.NewService(
		infrastructurelibraryimport.NewSQLiteRepository(database.Bun),
		fileRepo,
		libraryService,
		catalogBackfill,
		libraryService,
	)
	libraryImportService.SetManagedRootRegistrar(catalogService)
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

	pairingService, err := libraryaccessauth.NewService(catalogDeviceGrantRepo, catalogID, libraryaccessauth.Options{})
	if err != nil {
		return nil, err
	}
	publicLibrarySync := libraryrepo.NewSQLiteCatalogSyncStateRepository(database.Bun)
	publicBusinessAPI, err := libraryapi.NewBusinessAPI(libraryapi.BusinessConfig{
		CatalogID: catalogID, Catalog: catalogService, Items: catalogItemRepo,
		Assets: catalogAssetRepo, Files: fileRepo, Changes: catalogChangeRepo,
		Sync: publicLibrarySync,
	})
	if err != nil {
		return nil, err
	}
	publicTaskAPI, err := libraryapi.NewTaskAPI(libraryService)
	if err != nil {
		return nil, err
	}
	publicMusicReader := libraryrepo.NewSQLiteListenLocalMusicReadRepository(database.Bun, catalogID)
	musicResourceCache := ""
	if cacheBase, cacheErr := os.UserCacheDir(); cacheErr != nil {
		zap.L().Warn("Music resource cache unavailable", safeStationErrorLogFields("music_resource_cache_directory_failed", cacheErr)...)
	} else {
		musicResourceCache = musicResourceCacheDirectory(cacheBase, appVersion)
	}
	publicMusicAPI, err := libraryapi.NewMusicAPI(libraryapi.MusicConfig{
		CatalogID:                catalogID,
		Reader:                   publicMusicReader,
		Writer:                   libraryrepo.NewSQLiteListenLocalMusicWriteRepository(database.Bun),
		CompatibleRepresentation: libraryService,
		ResourceCacheDirectory:   musicResourceCache,
	})
	if err != nil {
		return nil, err
	}
	publicRoutes := append(publicBusinessAPI.Routes(), publicTaskAPI.Routes()...)
	publicRoutes = append(publicRoutes, publicMusicAPI.Routes()...)
	publicRSSAPI, err := libraryapi.NewRSSAPI(rssService, proxyManager)
	if err != nil {
		return nil, err
	}
	if cacheBase, cacheErr := os.UserCacheDir(); cacheErr != nil {
		zap.L().Warn("RSS image disk cache unavailable; using memory cache", safeStationErrorLogFields("rss_image_cache_directory_failed", cacheErr)...)
	} else {
		cacheDirectory := rssImageDiskCacheDirectory(cacheBase)
		if cacheErr := publicRSSAPI.ConfigureImageDiskCache(cacheDirectory); cacheErr != nil {
			fields := []zap.Field{zap.String("cacheRef", safeStationLogReference(cacheDirectory))}
			fields = append(fields, safeStationErrorLogFields("rss_image_cache_open_failed", cacheErr)...)
			zap.L().Warn("RSS image disk cache unavailable; using memory cache", fields...)
		}
	}
	// Keep the calling surface explicit: the tokenized loopback handler receives
	// the Desktop-reserved RSS stream capacity, while authenticated Routes use
	// the paired-device budget. Never merge these through Host/header inference.
	realtimeServer.Handle("/api/rss/", publicRSSAPI.LocalResourceHandler())
	publicRoutes = append(publicRoutes, publicRSSAPI.Routes()...)
	publicSyncEvents, err := libraryapi.NewSyncEventAPI(libraryapi.SyncEventConfig{
		Revalidator: pairingService,
		LibraryProbe: func(probeCtx context.Context) (libraryapi.SyncStationPosition, error) {
			position, probeErr := publicLibrarySync.GetCatalogSyncState(probeCtx, catalogID)
			return libraryapi.SyncStationPosition{
				Station: "library", Epoch: position.Epoch, HighWater: position.Cursor,
			}, probeErr
		},
		MusicProbe: func(probeCtx context.Context) (libraryapi.SyncStationPosition, error) {
			position, probeErr := publicMusicReader.GetSyncPosition(probeCtx)
			return libraryapi.SyncStationPosition{
				Station: "music", Epoch: position.Epoch, HighWater: position.HighWater,
			}, probeErr
		},
		RSSProbe: func(probeCtx context.Context) (libraryapi.SyncStationPosition, error) {
			position, probeErr := rssService.GetSyncOverview(probeCtx, catalogID)
			return libraryapi.SyncStationPosition{
				Station: "rss", Epoch: position.Epoch, HighWater: position.HighWater,
			}, probeErr
		},
		Events: eventBus,
	})
	if err != nil {
		return nil, err
	}
	publicRouter, err := libraryapi.NewRouter(libraryapi.Config{
		Version: AppVersion, CatalogID: catalogID, Authenticator: pairingService, Pairer: pairingService,
		Capabilities: []string{libraryapi.SyncEventsCapability},
		StationCapabilities: map[string][]string{
			"library": {"library-v1"},
			"music": {
				"music-sync-v1", "snapshot-keyset-v1", "changes-epoch-v1", "versioned-resource-id-v1",
				"music-mutations-v1", "music-play-events-v1", "music-track-state-v1",
				"music-playlists-v1", "music-provider-lyric-selection-v1",
				libraryapi.MusicIOSAudioRepresentationCapability,
			},
			"rss": {
				"rss-sync-v1", "opaque-resource-slots-v1",
				"rss-subscription-mutations-v1", "rss-shared-public-fetch-v1",
				"rss-observations-v1", "rss-fetch-lease-v1",
			},
		},
		AuthenticatedRoutes: publicSyncEvents.Routes(),
		Routes:              publicRoutes,
	})
	if err != nil {
		return nil, err
	}
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	deviceName, _ := os.Hostname()
	if strings.TrimSpace(deviceName) == "" {
		deviceName = AppName
	}
	certificateDirectory := filepath.Join(configDirectory, "xiadown", "library-access")
	tlsIdentity, err := libraryserver.LoadOrCreateCertificate(libraryserver.CertificateFiles{
		CertificatePath: filepath.Join(certificateDirectory, "library.crt"),
		PrivateKeyPath:  filepath.Join(certificateDirectory, "library.key"),
	}, libraryserver.CertificateOptions{DNSNames: []string{deviceName}})
	if err != nil {
		return nil, err
	}
	if tlsIdentity.PersistenceError != "" {
		fields := []zap.Field{zap.String("fingerprint", tlsIdentity.Fingerprint)}
		fields = append(fields, safeStationTextLogFields("paired_api_tls_identity_persistence_failed", tlsIdentity.PersistenceError)...)
		zap.L().Warn("Library TLS identity is ephemeral; paired clients must refresh their certificate pin after restart", fields...)
	} else if tlsIdentity.Rotated {
		zap.L().Warn(
			"Library TLS identity was rotated; paired clients must refresh their certificate pin",
			zap.String("fingerprint", tlsIdentity.Fingerprint),
		)
	}
	publicServer, err = libraryserver.New(libraryserver.Config{Handler: publicRouter})
	if err != nil {
		return nil, err
	}
	if err := publicServer.Start(serverCtx); err != nil {
		return nil, err
	}
	go publicSyncEvents.Run(serverCtx)
	executablePath, _ := os.Executable()
	lanRuntime, err := libraryserver.NewLANRuntime(libraryserver.LANRuntimeConfig{
		Server: publicServer, Identity: &tlsIdentity, Advertiser: discovery.NewAdvertiser(),
		Firewall: firewall.NewManager(), Program: executablePath,
	})
	if err != nil {
		return nil, err
	}
	libraryAccessService := libraryaccessservice.NewService(
		libraryaccessrepo.NewSQLiteRepository(database.Bun), tailscale.NewManager(nil), lanRuntime, deviceName,
	)
	publicBackendPort := func() int {
		_, portValue, splitErr := net.SplitHostPort(publicServer.BackendAddress())
		if splitErr != nil {
			return 0
		}
		port, _ := strconv.Atoi(portValue)
		return port
	}
	libraryAccessReconcilerOptions := libraryaccessservice.ReconcilerOptions{
		InitialReconcile: true,
		NetworkRevision: func() string {
			endpoints, endpointErr := discovery.SystemEligibleLANEndpoints()
			if endpointErr != nil || len(endpoints) == 0 {
				return "unavailable"
			}
			var revision strings.Builder
			for _, endpoint := range endpoints {
				revision.WriteString(strconv.Itoa(endpoint.InterfaceIndex))
				revision.WriteByte(':')
				revision.WriteString(endpoint.ListenAddress(0))
				revision.WriteByte('|')
			}
			return revision.String()
		},
		OnResult: func(status libraryaccessservice.Status, reconcileErr error) {
			if reconcileErr != nil {
				zap.L().Warn("reconcile Library access transports", safeStationErrorLogFields("paired_api_transport_reconcile_failed", reconcileErr)...)
				return
			}
			if status.LAN.LastError != "" || status.Tailscale.LastError != "" {
				fields := []zap.Field{zap.String("errorCode", "paired_api_transport_unavailable")}
				if status.LAN.LastError != "" {
					fields = append(fields, zap.String("lanErrorRef", safeStationLogReference(status.LAN.LastError)))
				}
				if status.Tailscale.LastError != "" {
					fields = append(fields, zap.String("tailscaleErrorRef", safeStationLogReference(status.Tailscale.LastError)))
				}
				zap.L().Warn("Library access transport remains unavailable", fields...)
			}
		},
	}
	libraryAccessReconcilerDone := make(chan struct{})
	var libraryAccessReconcilerOnce sync.Once
	startLibraryAccessReconciler = func() {
		libraryAccessReconcilerOnce.Do(func() {
			go func() {
				defer close(libraryAccessReconcilerDone)
				libraryAccessService.RunReconciler(serverCtx, publicBackendPort, libraryAccessReconcilerOptions)
			}()
		})
	}
	waitLibraryAccessReconciler = func(waitCtx context.Context) error {
		// If shutdown wins the race with ApplicationStarted, mark the lifecycle
		// complete without ever launching external transport work.
		libraryAccessReconcilerOnce.Do(func() { close(libraryAccessReconcilerDone) })
		select {
		case <-libraryAccessReconcilerDone:
			return nil
		case <-waitCtx.Done():
			return waitCtx.Err()
		}
	}

	osNotifications := notifications.New()
	settingsHandler := wails.NewSettingsHandler(settingsService, windowManager, appLogger, proxyManager, autostartManager, libraryService, listenPlayer, listenLivePlayer)
	app.RegisterService(application.NewService(settingsHandler))
	cacheRoot, _ := os.UserCacheDir()
	applicationResetManager, err := newApplicationResetManager()
	if err != nil {
		return nil, err
	}
	app.RegisterService(application.NewService(wails.NewDataManagementHandler(wails.DataManagementConfig{
		ConfigRoot:               filepath.Dir(databasePath),
		CacheRoot:                cacheRoot,
		LogDirectory:             appLogger.LogDir(),
		DatabasePath:             databasePath,
		Database:                 database.SQL,
		BackupDirectory:          backupDirectory,
		AppSessions:              appSessionsService,
		Activity:                 libraryService,
		Dependencies:             dependenciesService,
		SessionVaultKeyInventory: appsessionvault.MasterKeyInventory,
		Resetter:                 applicationResetManager,
		Quitter:                  app,
	})))
	app.RegisterService(application.NewService(wails.NewAppSessionsHandler(appSessionsService, windowManager, listenPlayer, listenLivePlayer)))
	app.RegisterService(application.NewService(wails.NewDependenciesHandler(dependenciesService, windowManager)))
	app.RegisterService(application.NewService(wails.NewLibraryHandler(libraryService, windowManager)))
	app.RegisterService(application.NewService(wails.NewCatalogHandler(catalogService, windowManager)))
	app.RegisterService(application.NewService(wails.NewLibraryBackupHandler(libraryBackupService)))
	app.RegisterService(application.NewService(wails.NewLibraryImportHandler(libraryImportService, windowManager)))
	app.RegisterService(application.NewService(wails.NewLibraryAccessHandler(libraryAccessService, publicBackendPort)))
	app.RegisterService(application.NewService(wails.NewLibraryPairingHandler(
		pairingService,
		publicServer.TLSFingerprint,
		publicServer.LANAddress,
		func(statusCtx context.Context) string {
			status, statusErr := libraryAccessService.GetStatus(statusCtx)
			if statusErr != nil {
				return ""
			}
			return status.Tailscale.ServeURL
		},
		publicServer.LANAddresses,
	)))
	app.RegisterService(application.NewService(wails.NewSystemHandler(fontService)))
	app.RegisterService(application.NewService(wails.NewOSNotificationHandlerWithHTTPClientProvider(osNotifications, app, proxyManager)))
	app.RegisterService(application.NewService(wails.NewRealtimeHandler(realtimeServer)))
	app.RegisterService(application.NewService(wails.NewPetsHandler(petsService)))
	app.RegisterService(application.NewService(wails.NewEqualizerHandler(equalizerService)))
	app.RegisterService(application.NewService(wails.NewListenPlayerHandler(listenPlayer, listenPlaybackService)))
	app.RegisterService(application.NewService(wails.NewListenLyricsHandler(listenLyricsService)))
	app.RegisterService(application.NewService(wails.NewListenLivePlayerHandler(listenLivePlayer, playbackCoordinator)))
	app.RegisterService(application.NewService(wails.NewYouTubeWorkspaceHandler(youtubeWorkspaceService, listenLivePlayer, playbackCoordinator)))
	app.RegisterService(application.NewService(wails.NewRSSHandlerWithConfig(
		rssService,
		wails.RSSHandlerConfig{
			ResourceBaseURL: realtimeServer.HTTPURL(),
			ImageLoader:     publicRSSAPI,
			SaveDialog:      windowManager,
		},
	)))
	app.RegisterService(application.NewService(rssVideoPlayerHandler))
	app.RegisterService(application.NewService(rssSitePlayerHandler))
	app.RegisterService(application.NewService(playbackCoordinatorHandler))
	telemetryHandler := wails.NewTelemetryHandler(telemetryService, windowManager, proxyManager)
	app.RegisterService(application.NewService(telemetryHandler))
	app.RegisterService(application.NewService(wails.NewUpdateHandler(updateService, app)))

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		appSessionProvider.MarkWebKitReady()
		windowManager.MarkApplicationStarted()
		windowManager.StartMainWindowBootReadyFallback(serverCtx)
		go func() {
			// The diagnostic audit can scan a large Library and is warning-only.
			// Reconciliation remains synchronous above: LibraryFile writes and
			// their Catalog projection are not one atomic transaction, so startup
			// Run is the durable recovery boundary after an interrupted write. Wait
			// for the frontend-ready handshake (or its native fallback) so a slow
			// launch never competes with this optional scan for database I/O.
			ticker := time.NewTicker(25 * time.Millisecond)
			defer ticker.Stop()
			for !windowManager.MainBootSettled() {
				select {
				case <-serverCtx.Done():
					return
				case <-ticker.C:
				}
			}
			if serverCtx.Err() != nil {
				return
			}
			settle := time.NewTimer(300 * time.Millisecond)
			defer settle.Stop()
			select {
			case <-serverCtx.Done():
				return
			case <-settle.C:
			}
			auditReport, auditErr := catalogAuditor.Audit(serverCtx, catalogaudit.Request{
				CatalogID: catalogID, MigrationID: libraryservice.LegacyCatalogProjectionID,
			})
			if auditErr != nil {
				zap.L().Warn("deferred catalog projection audit failed",
					safeStationErrorLogFields("catalog_projection_audit_failed", auditErr)...,
				)
				return
			}
			if !auditReport.IsHealthy() {
				zap.L().Warn("catalog projection audit found inconsistencies",
					zap.Int64("findings", auditReport.Findings.Total()),
					zap.Int64("legacyFiles", auditReport.Counts.LegacyFiles),
				)
			}
		}()
		// External transport recovery must never delay application.Run or window
		// creation. The reconciler performs a bounded initial attempt only after
		// the native application has finished launching.
		startLibraryAccessReconciler()
		go func() {
			time.Sleep(500 * time.Millisecond)
			libraryService.RecoverPendingJobs(context.Background())
		}()
		go func() {
			hydrateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = appSessionsService.RecordsForSiteKey(hydrateCtx, "youtube")
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

	// The database is deliberately the final shutdown registration. Wails runs
	// shutdown hooks in registration order, so every transport, scheduler and
	// background service above has quiesced before SQLite is closed.
	app.OnShutdown(func() { _ = proxyManager.Close() })
	app.OnShutdown(func() { _ = database.Close() })
	proxyManagerOwnedByApp = true

	return app, nil
}

func rssImageDiskCacheDirectory(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	// Keep the on-disk format version in the namespace so a future incompatible
	// record format can start cleanly without probing or migrating every file.
	return filepath.Join(base, "xiadown", "rss", "resources", "v1")
}

func musicResourceCacheDirectory(base string, version string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	channel := "development"
	if isComparableReleaseVersion(version) {
		channel = "production"
	}
	// Release and development builds intentionally use different
	// SingleInstance IDs, so they can be alive at the same time. Keep their
	// CAS directories under the same invariant to prevent either process from
	// deleting the other's temp files or independently spending the same quota.
	return filepath.Join(base, "xiadown", "music", "resources", "v1", channel)
}

func libraryVideoThumbnailCacheDirectory(base string, version string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	channel := "development"
	if isComparableReleaseVersion(version) {
		channel = "production"
	}
	// Development and release instances may run concurrently. Isolate their
	// generated previews so neither process prunes the other's active cache.
	return filepath.Join(base, "xiadown", "library", "video-thumbnails", "v1", channel)
}

func openDatabase(ctx context.Context) (*persistence.Database, error) {
	path, backupDirectory, err := libraryPersistencePaths()
	if err != nil {
		return nil, err
	}
	return openDatabaseAt(ctx, path, backupDirectory, "")
}

func openDatabaseAt(ctx context.Context, path, backupDirectory, markerPath string) (*persistence.Database, error) {
	restoreConfig := infrastructurelibrarybackup.StartupRestoreConfig{
		DatabasePath: path, BackupDirectory: backupDirectory, RestoreMarkerPath: markerPath,
	}
	result, err := infrastructurelibrarybackup.ApplyPendingRestore(ctx, restoreConfig)
	if err != nil {
		return nil, err
	}
	database, openErr := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
	if openErr != nil {
		if !result.Applied {
			return nil, openErr
		}
		rollbackErr := infrastructurelibrarybackup.RollbackPendingRestore(ctx, restoreConfig)
		if rollbackErr != nil {
			return nil, errors.Join(openErr, rollbackErr)
		}
		recovered, recoveredErr := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
		if recoveredErr != nil {
			return nil, errors.Join(openErr, recoveredErr)
		}
		return recovered, nil
	}
	if result.Applied {
		if err := infrastructurelibrarybackup.FinalizePendingRestore(ctx, restoreConfig); err != nil {
			_ = database.Close()
			return nil, err
		}
	}
	return database, nil
}

func libraryPersistencePaths() (string, string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}

	appDir := filepath.Join(configDir, "xiadown")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", "", err
	}
	return filepath.Join(appDir, "data.db"), filepath.Join(appDir, "library-backups"), nil
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
		zap.L().Warn("update: restore prepared update failed",
			safeStationErrorLogFields("prepared_update_restore_failed", err)...,
		)
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

func singleInstanceUniqueID(version string) string {
	if isComparableReleaseVersion(version) {
		return productionSingleInstanceID
	}
	return productionSingleInstanceID + ".dev"
}
