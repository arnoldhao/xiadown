package wails

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bep/debounce"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/zap"

	"xiadown/internal/application/settings/dto"
	"xiadown/internal/application/settings/service"
	"xiadown/internal/domain/settings"
	"xiadown/internal/domain/update"
	"xiadown/internal/presentation/i18n"
)

type WindowManager struct {
	app                 *application.App
	mainWindow          *application.WebviewWindow
	settingsWindow      *application.WebviewWindow
	trayMiniPlayer      *application.WebviewWindow
	settingsService     *service.SettingsService
	appVersion          string
	mainVisibal         bool
	settingsVisible     bool
	boundsMu            sync.Mutex
	lastMainBounds      dto.WindowBounds
	lastSettingsBounds  dto.WindowBounds
	mainBoundsDirty     bool
	settingsBoundsDirty bool
	initialized         bool
	updateState         update.Info
	quitting            atomic.Bool
	applicationStarted  atomic.Bool
	mainBoundsReady     atomic.Bool
	settingsBoundsReady atomic.Bool

	systemTray *SystemTrayController
}

type windowTrayActions struct {
	manager *WindowManager
	app     *application.App
}

func (actions windowTrayActions) OpenMainWindow() {
	if actions.manager == nil {
		return
	}
	actions.manager.ShowMainWindow()
}

func (actions windowTrayActions) OpenNewDownload() {
	if actions.manager == nil {
		return
	}
	actions.manager.OpenNewDownload()
}

func (actions windowTrayActions) OpenSettings() {
	if actions.manager == nil {
		return
	}
	actions.manager.ShowSettingsWindow()
}

func (actions windowTrayActions) ApplyMenuBarVisibility(value string) {
	if actions.manager == nil {
		return
	}
	actions.manager.applyMenuBarVisibilityChange(value)
}

func (actions windowTrayActions) Quit() {
	if actions.manager != nil {
		actions.manager.PrepareQuit()
	}
	if actions.app == nil {
		return
	}
	actions.app.Quit()
}

func (actions windowTrayActions) OpenUpdate() {
	if actions.manager == nil {
		return
	}
	actions.manager.emitNavigateToAbout()
}

func NewWindowManager(app *application.App, settingsService *service.SettingsService, appVersion string, trayIcon []byte, launchedByAutoStart bool) (*WindowManager, error) {
	current, err := settingsService.GetSettings(context.Background())
	if err != nil {
		return nil, err
	}

	mainWindowOptions := buildMainWindowOptions(current, launchedByAutoStart)
	settingsWindowOptions := buildSettingsWindowOptions(current, launchedByAutoStart)
	mainWindow := app.Window.NewWithOptions(mainWindowOptions)
	settingsWindow := app.Window.NewWithOptions(settingsWindowOptions)
	trayMiniPlayer := app.Window.NewWithOptions(buildTrayMiniPlayerWindowOptions(current))
	settingsWindow.Hide()
	startHidden := shouldStartHidden(current, launchedByAutoStart)

	manager := &WindowManager{
		app:                app,
		mainWindow:         mainWindow,
		settingsWindow:     settingsWindow,
		trayMiniPlayer:     trayMiniPlayer,
		settingsService:    settingsService,
		appVersion:         appVersion,
		mainVisibal:        !startHidden,
		settingsVisible:    false,
		lastMainBounds:     current.MainBounds,
		lastSettingsBounds: current.SettingsBounds,
	}

	manager.systemTray = NewSystemTrayController(app, windowTrayActions{
		manager: manager,
		app:     app,
	}, trayIcon, trayMiniPlayer)

	manager.ApplySettings(current)
	manager.registerMainWindowEvents()
	manager.registerSettingsWindowEvents()
	manager.registerDockEvents()
	manager.initialized = true

	return manager, nil
}

func (manager *WindowManager) MarkApplicationStarted() {
	if manager == nil {
		return
	}
	manager.applicationStarted.Store(true)
	manager.restoreStartupWindowBounds()
}

func (manager *WindowManager) canInvokeSync() bool {
	return manager != nil && manager.initialized && manager.applicationStarted.Load()
}

func (manager *WindowManager) ShowMainWindow() {
	manager.mainVisibal = true
	manager.restoreCachedBounds(windowTypeMain)
	manager.ensureWindowVisible(windowTypeMain)
	manager.mainWindow.UnMinimise()
	manager.mainWindow.Show()
	manager.mainWindow.Focus()
	manager.markBoundsTrackingReady(windowTypeMain)
}

func (manager *WindowManager) OpenNewDownload() {
	if manager == nil {
		return
	}
	manager.ShowMainWindow()
	manager.emitOpenNewDownload()
}

func (manager *WindowManager) ShowSettingsWindow() {
	manager.settingsVisible = true
	manager.restoreCachedBounds(windowTypeSettings)
	manager.ensureWindowVisible(windowTypeSettings)
	manager.settingsWindow.UnMinimise()
	manager.settingsWindow.Show()
	manager.settingsWindow.Focus()
	manager.markBoundsTrackingReady(windowTypeSettings)
}

func (manager *WindowManager) SetMainWindowChromeHidden(hidden bool) {
	if manager == nil || manager.mainWindow == nil || runtime.GOOS != "darwin" {
		return
	}
	state := application.ButtonEnabled
	if hidden {
		state = application.ButtonHidden
	}
	manager.mainWindow.SetMinimiseButtonState(state)
	manager.mainWindow.SetMaximiseButtonState(state)
	manager.mainWindow.SetCloseButtonState(state)
}

func (manager *WindowManager) PrepareQuit() {
	if manager == nil {
		return
	}
	if !manager.quitting.CompareAndSwap(false, true) {
		return
	}
	manager.PersistAllBounds()
}

func (manager *WindowManager) HandleSecondInstanceLaunch() {
	if manager == nil || manager.mainWindow == nil {
		return
	}

	reveal := func() {
		manager.ShowMainWindow()
	}

	if manager.canInvokeSync() {
		application.InvokeSync(reveal)
		return
	}

	reveal()
}

func (manager *WindowManager) SelectDirectoryDialog(title string, initialDir string) (string, error) {
	return manager.selectDirectoryDialog(title, initialDir, manager.settingsWindow)
}

func (manager *WindowManager) SelectMainDirectoryDialog(title string, initialDir string) (string, error) {
	return manager.selectDirectoryDialog(title, initialDir, manager.mainWindow)
}

func (manager *WindowManager) selectDirectoryDialog(title string, initialDir string, attachWindow *application.WebviewWindow) (string, error) {
	if manager == nil || manager.app == nil {
		return "", fmt.Errorf("app not available")
	}
	dialog := manager.app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false)
	if directory := resolveExistingDialogDirectory(initialDir); directory != "" {
		dialog = dialog.SetDirectory(directory)
	}
	if attachWindow != nil {
		dialog = dialog.AttachToWindow(attachWindow)
	}
	selected, err := dialog.PromptForSingleSelection()
	if isDialogCancelledError(err) {
		return "", nil
	}
	return selected, err
}

func resolveExistingDialogDirectory(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	for {
		info, err := os.Stat(cleaned)
		if err == nil && info.IsDir() {
			return cleaned
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned || parent == "." {
			return ""
		}
		cleaned = parent
	}
}

func (manager *WindowManager) SelectFilesDialog(title string, initialDir string) ([]string, error) {
	if manager == nil || manager.app == nil {
		return nil, fmt.Errorf("app not available")
	}
	dialog := manager.app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(false).
		CanChooseFiles(true)
	if initialDir != "" {
		dialog = dialog.SetDirectory(initialDir)
	}
	if manager.mainWindow != nil {
		dialog = dialog.AttachToWindow(manager.mainWindow)
	}
	selected, err := dialog.PromptForMultipleSelection()
	if isDialogCancelledError(err) {
		return []string{}, nil
	}
	return selected, err
}

func isDialogCancelledError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return false
	}
	cancelMarkers := []string{
		"shellitem is nil",
		"shell item is nil",
		"operation was canceled",
		"operation was cancelled",
		"operation canceled",
		"operation cancelled",
		"canceled by the user",
		"cancelled by the user",
		"user canceled",
		"user cancelled",
		"dialog was closed",
		"0x800704c7",
	}
	for _, marker := range cancelMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (manager *WindowManager) HideMainWindow() {
	manager.persistBoundsOrCached(windowTypeMain, "hide-main")
	manager.mainVisibal = false
	manager.mainWindow.Hide()
}

func (manager *WindowManager) HideSettingsWindow() {
	manager.persistBoundsOrCached(windowTypeSettings, "hide-settings")
	manager.settingsVisible = false
	manager.settingsWindow.Hide()
}

func (manager *WindowManager) SetMenu(menu *application.Menu) {
	if manager.mainWindow != nil {
		manager.mainWindow.SetMenu(menu)
		if runtime.GOOS == "windows" {
			manager.mainWindow.HideMenuBar()
		}
	}
	if manager.settingsWindow != nil {
		manager.settingsWindow.SetMenu(menu)
		if runtime.GOOS == "windows" {
			manager.settingsWindow.HideMenuBar()
		}
	}
}

func (manager *WindowManager) ApplySettings(current dto.Settings) {
	apply := func() {
		color := backgroundColour(current)
		manager.syncWindowPresentation(current)
		manager.mainWindow.SetBackgroundColour(color)
		manager.settingsWindow.SetBackgroundColour(color)
		manager.rebuildMenu(current)
		manager.systemTray.Update(current)
		manager.dispatchWindowEvent("settings:updated", current)
		manager.dispatchWindowEvent("theme:changed", current.EffectiveAppearance)
	}

	if manager.canInvokeSync() {
		application.InvokeSync(apply)
		return
	}

	apply()
}

func (manager *WindowManager) dispatchWindowEvent(name string, data any) {
	if manager == nil {
		return
	}

	event := &application.CustomEvent{
		Name: name,
		Data: data,
	}

	if manager.mainWindow != nil {
		manager.mainWindow.DispatchWailsEvent(event)
	}
	if manager.settingsWindow != nil {
		manager.settingsWindow.DispatchWailsEvent(event)
	}
	if manager.trayMiniPlayer != nil {
		manager.trayMiniPlayer.DispatchWailsEvent(event)
	}
}

func (manager *WindowManager) EmitDependenciesUpdated() {
	if manager == nil || manager.app == nil {
		return
	}
	emit := func() {
		manager.app.Event.Emit("dependencies:updated")
	}
	if manager.canInvokeSync() {
		application.InvokeSync(emit)
		return
	}
	emit()
}

func (manager *WindowManager) registerMainWindowEvents() {
	mainDebounce := debounce.New(600 * time.Millisecond)

	manager.mainWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		manager.restoreCachedBounds(windowTypeMain)
		manager.ensureWindowVisible(windowTypeMain)
		manager.markBoundsTrackingReady(windowTypeMain)
		configureListenYouTubeMusicNativeWindow(manager.mainWindow.NativeWindow(), listenYouTubeMusicUserAgent())
	})

	manager.mainWindow.OnWindowEvent(events.Common.WindowDidMove, func(_ *application.WindowEvent) {
		if ignoreCommonWindowBoundsEvents() {
			return
		}
		manager.rememberCurrentBounds(windowTypeMain)
		mainDebounce(func() {
			manager.persistBoundsOrCached(windowTypeMain, "common-window-did-move-debounced")
		})
	})

	manager.mainWindow.OnWindowEvent(events.Common.WindowDidResize, func(_ *application.WindowEvent) {
		manager.enforceMinimumSize(windowTypeMain)
		if ignoreCommonWindowBoundsEvents() {
			return
		}
		manager.rememberCurrentBounds(windowTypeMain)
		mainDebounce(func() {
			manager.persistBoundsOrCached(windowTypeMain, "common-window-did-resize-debounced")
		})
	})

	if runtime.GOOS == "windows" {
		manager.mainWindow.OnWindowEvent(events.Windows.WindowEndMove, func(_ *application.WindowEvent) {
			manager.persistBoundsOrCached(windowTypeMain, "windows-window-end-move")
		})
		manager.mainWindow.OnWindowEvent(events.Windows.WindowEndResize, func(_ *application.WindowEvent) {
			manager.persistBoundsOrCached(windowTypeMain, "windows-window-end-resize")
		})
	}

	// Use hook to cancel default destroy flow and just hide.
	manager.mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if manager.quitting.Load() {
			return
		}
		event.Cancel()
		manager.HideMainWindow()
	})

}

func (manager *WindowManager) registerSettingsWindowEvents() {
	settingsDebounce := debounce.New(600 * time.Millisecond)

	manager.settingsWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		manager.restoreCachedBounds(windowTypeSettings)
		manager.ensureWindowVisible(windowTypeSettings)
		manager.markBoundsTrackingReady(windowTypeSettings)
	})

	manager.settingsWindow.OnWindowEvent(events.Common.WindowDidMove, func(_ *application.WindowEvent) {
		if ignoreCommonWindowBoundsEvents() {
			return
		}
		manager.rememberCurrentBounds(windowTypeSettings)
		settingsDebounce(func() {
			manager.persistBoundsOrCached(windowTypeSettings, "common-window-did-move-debounced")
		})
	})

	manager.settingsWindow.OnWindowEvent(events.Common.WindowDidResize, func(_ *application.WindowEvent) {
		manager.enforceMinimumSize(windowTypeSettings)
		if ignoreCommonWindowBoundsEvents() {
			return
		}
		manager.rememberCurrentBounds(windowTypeSettings)
		settingsDebounce(func() {
			manager.persistBoundsOrCached(windowTypeSettings, "common-window-did-resize-debounced")
		})
	})

	if runtime.GOOS == "windows" {
		manager.settingsWindow.OnWindowEvent(events.Windows.WindowEndMove, func(_ *application.WindowEvent) {
			manager.persistBoundsOrCached(windowTypeSettings, "windows-window-end-move")
		})
		manager.settingsWindow.OnWindowEvent(events.Windows.WindowEndResize, func(_ *application.WindowEvent) {
			manager.persistBoundsOrCached(windowTypeSettings, "windows-window-end-resize")
		})
	}

	// Use hook to cancel default destroy flow and just hide.
	manager.settingsWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if manager.quitting.Load() {
			return
		}
		event.Cancel()
		manager.HideSettingsWindow()
	})
}

func (manager *WindowManager) registerDockEvents() {
	if runtime.GOOS != "darwin" {
		return
	}

	manager.app.Event.RegisterApplicationEventHook(events.Mac.ApplicationShouldHandleReopen, func(event *application.ApplicationEvent) {
		if manager.mainWindow == nil {
			return
		}

		if !manager.mainWindow.IsVisible() {
			manager.ShowMainWindow()
		} else if !manager.mainWindow.IsFocused() {
			manager.mainWindow.Focus()
		}

		if !event.Context().HasVisibleWindows() {
			event.Cancel()
		}
	})
}

func (manager *WindowManager) ensureWindowVisible(target windowType) {
	if manager == nil || manager.app == nil {
		return
	}

	window := manager.windowForType(target)
	if window == nil {
		return
	}

	screens := manager.app.Screen.GetAll()
	primary := manager.app.Screen.GetPrimary()
	currentBounds := window.Bounds()
	nextBounds, recentered := resolveVisibleWindowBounds(currentBounds, screens, primary)
	if !recentered {
		return
	}

	window.SetBounds(nextBounds)
	manager.rememberBounds(target, nextBounds)
	if manager.persistResolvedBounds(target, nextBounds, "ensure-window-visible") {
		manager.markBoundsClean(target)
	}

	windowName := "main"
	if target == windowTypeSettings {
		windowName = "settings"
	}

	zap.L().Warn(
		"window bounds were off-screen and have been recentered",
		zap.String("window", windowName),
		zap.Int("fromX", currentBounds.X),
		zap.Int("fromY", currentBounds.Y),
		zap.Int("fromWidth", currentBounds.Width),
		zap.Int("fromHeight", currentBounds.Height),
		zap.Int("toX", nextBounds.X),
		zap.Int("toY", nextBounds.Y),
		zap.Int("toWidth", nextBounds.Width),
		zap.Int("toHeight", nextBounds.Height),
	)
}

func (manager *WindowManager) persistBounds(target windowType, source string) bool {
	if !manager.boundsTrackingReady(target) {
		return false
	}

	bounds, ok := manager.capturableWindowBounds(target)
	if !ok {
		return false
	}

	manager.rememberBounds(target, bounds)
	if !manager.persistResolvedBounds(target, bounds, source) {
		return false
	}
	manager.markBoundsClean(target)
	return true
}

func (manager *WindowManager) persistBoundsOrCached(target windowType, source string) {
	if manager.persistBounds(target, source) {
		return
	}
	manager.persistCachedBounds(target, source)
}

func (manager *WindowManager) persistCachedBounds(target windowType, source string) bool {
	bounds, ok := manager.cachedBoundsForPersistence(target)
	if !ok {
		return false
	}
	if !manager.persistResolvedBounds(target, bounds, source+"-cached") {
		return false
	}
	manager.markBoundsClean(target)
	return true
}

func normalizeWindowBoundsForPersistence(bounds application.Rect, target windowType) (application.Rect, bool) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return application.Rect{}, false
	}
	minWidth := settings.MinMainWindowWidth
	minHeight := settings.MinMainWindowHeight
	if target == windowTypeSettings {
		minWidth = settings.MinSettingsWindowWidth
		minHeight = settings.MinSettingsWindowHeight
	}
	if bounds.Width < minWidth {
		bounds.Width = minWidth
	}
	if bounds.Height < minHeight {
		bounds.Height = minHeight
	}
	return bounds, true
}

func (manager *WindowManager) persistResolvedBounds(target windowType, bounds application.Rect, source string) bool {
	request := dto.UpdateSettingsRequest{}

	if target == windowTypeMain {
		request.MainBounds = &dto.WindowBounds{
			X:      bounds.X,
			Y:      bounds.Y,
			Width:  bounds.Width,
			Height: bounds.Height,
		}
	} else {
		request.SettingsBounds = &dto.WindowBounds{
			X:      bounds.X,
			Y:      bounds.Y,
			Width:  bounds.Width,
			Height: bounds.Height,
		}
	}

	_, err := manager.settingsService.UpdateSettings(context.Background(), request)
	if err != nil {
		zap.L().Warn("save window bounds failed",
			zap.String("source", source),
			zap.Error(err),
			zap.String("window", windowTypeName(target)),
			zap.Int("x", bounds.X),
			zap.Int("y", bounds.Y),
			zap.Int("width", bounds.Width),
			zap.Int("height", bounds.Height),
		)
		return false
	}
	return true
}

func (manager *WindowManager) PersistAllBounds() {
	if manager == nil {
		return
	}
	manager.persistBoundsOrCached(windowTypeMain, "persist-all")
	manager.persistBoundsOrCached(windowTypeSettings, "persist-all")
}

func (manager *WindowManager) windowForType(target windowType) *application.WebviewWindow {
	if target == windowTypeMain {
		return manager.mainWindow
	}
	return manager.settingsWindow
}

func (manager *WindowManager) windowBounds(target windowType) application.Rect {
	if target == windowTypeMain {
		return manager.mainWindow.Bounds()
	}
	return manager.settingsWindow.Bounds()
}

func (manager *WindowManager) capturableWindowBounds(target windowType) (application.Rect, bool) {
	window := manager.windowForType(target)
	if window == nil || !window.IsVisible() || window.IsMinimised() || window.IsFullscreen() || window.IsMaximised() {
		return application.Rect{}, false
	}
	return normalizeWindowBoundsForPersistence(manager.windowBounds(target), target)
}

func (manager *WindowManager) rememberCurrentBounds(target windowType) bool {
	if !manager.boundsTrackingReady(target) {
		return false
	}

	bounds, ok := manager.capturableWindowBounds(target)
	if !ok {
		return false
	}
	manager.rememberBounds(target, bounds)
	return true
}

func (manager *WindowManager) rememberBounds(target windowType, bounds application.Rect) {
	if manager == nil {
		return
	}
	bounds, ok := normalizeWindowBoundsForPersistence(bounds, target)
	if !ok {
		return
	}
	remembered := dto.WindowBounds{
		X:      bounds.X,
		Y:      bounds.Y,
		Width:  bounds.Width,
		Height: bounds.Height,
	}
	manager.boundsMu.Lock()
	defer manager.boundsMu.Unlock()
	if target == windowTypeMain {
		manager.lastMainBounds = remembered
		manager.mainBoundsDirty = true
		return
	}
	manager.lastSettingsBounds = remembered
	manager.settingsBoundsDirty = true
}

func (manager *WindowManager) cachedBounds(target windowType) (application.Rect, bool) {
	if manager == nil {
		return application.Rect{}, false
	}
	manager.boundsMu.Lock()
	defer manager.boundsMu.Unlock()
	bounds := manager.lastMainBounds
	if target == windowTypeSettings {
		bounds = manager.lastSettingsBounds
	}
	return normalizeWindowBoundsForPersistence(application.Rect{
		X:      bounds.X,
		Y:      bounds.Y,
		Width:  bounds.Width,
		Height: bounds.Height,
	}, target)
}

func (manager *WindowManager) cachedBoundsForPersistence(target windowType) (application.Rect, bool) {
	if manager == nil {
		return application.Rect{}, false
	}
	manager.boundsMu.Lock()
	defer manager.boundsMu.Unlock()
	bounds := manager.lastMainBounds
	dirty := manager.mainBoundsDirty
	if target == windowTypeSettings {
		bounds = manager.lastSettingsBounds
		dirty = manager.settingsBoundsDirty
	}
	if !dirty {
		return application.Rect{}, false
	}
	return normalizeWindowBoundsForPersistence(application.Rect{
		X:      bounds.X,
		Y:      bounds.Y,
		Width:  bounds.Width,
		Height: bounds.Height,
	}, target)
}

func (manager *WindowManager) markBoundsClean(target windowType) {
	if manager == nil {
		return
	}
	manager.boundsMu.Lock()
	defer manager.boundsMu.Unlock()
	if target == windowTypeMain {
		manager.mainBoundsDirty = false
		return
	}
	manager.settingsBoundsDirty = false
}

func (manager *WindowManager) boundsTrackingReady(target windowType) bool {
	if manager == nil {
		return false
	}
	if target == windowTypeSettings {
		return manager.settingsBoundsReady.Load()
	}
	return manager.mainBoundsReady.Load()
}

func (manager *WindowManager) markBoundsTrackingReady(target windowType) {
	if manager == nil {
		return
	}
	if target == windowTypeSettings {
		manager.settingsBoundsReady.Store(true)
		return
	}
	manager.mainBoundsReady.Store(true)
}

func (manager *WindowManager) restoreStartupWindowBounds() {
	if manager == nil {
		return
	}
	manager.restoreCachedBounds(windowTypeMain)
	manager.restoreCachedBounds(windowTypeSettings)
}

func (manager *WindowManager) restoreCachedBounds(target windowType) {
	window := manager.windowForType(target)
	if window == nil || window.IsFullscreen() || window.IsMaximised() {
		return
	}
	bounds, ok := manager.cachedBounds(target)
	if !ok {
		return
	}
	window.SetBounds(bounds)
}

func resolveVisibleWindowBounds(bounds application.Rect, screens []*application.Screen, primary *application.Screen) (application.Rect, bool) {
	if bounds.IsEmpty() || len(screens) == 0 {
		return bounds, false
	}
	if isWindowRectVisibleOnScreens(bounds, screens) {
		return bounds, false
	}

	targetScreen := primary
	if targetScreen == nil && len(screens) > 0 {
		targetScreen = screens[0]
	}
	if targetScreen == nil {
		return bounds, false
	}

	visibleArea := screenVisibleArea(targetScreen)
	if visibleArea.IsEmpty() {
		return bounds, false
	}

	next := bounds
	next.X = visibleArea.X
	next.Y = visibleArea.Y
	if visibleArea.Width > bounds.Width {
		next.X = visibleArea.X + (visibleArea.Width-bounds.Width)/2
	}
	if visibleArea.Height > bounds.Height {
		next.Y = visibleArea.Y + (visibleArea.Height-bounds.Height)/2
	}

	return next, true
}

func isWindowRectVisibleOnScreens(bounds application.Rect, screens []*application.Screen) bool {
	if bounds.IsEmpty() {
		return false
	}
	for _, screen := range screens {
		if screen == nil {
			continue
		}
		visibleArea := screenVisibleArea(screen)
		if visibleArea.IsEmpty() {
			continue
		}
		if !visibleArea.Intersect(bounds).IsEmpty() {
			return true
		}
	}
	return false
}

func screenVisibleArea(screen *application.Screen) application.Rect {
	if screen == nil {
		return application.Rect{}
	}
	if !screen.WorkArea.IsEmpty() {
		return screen.WorkArea
	}
	return screen.Bounds
}

type windowType int

const (
	windowTypeMain windowType = iota
	windowTypeSettings
)

func windowTypeName(target windowType) string {
	if target == windowTypeSettings {
		return "settings"
	}
	return "main"
}

func ignoreCommonWindowBoundsEvents() bool {
	return runtime.GOOS == "windows"
}

func buildMainWindowOptions(current dto.Settings, launchedByAutoStart bool) application.WebviewWindowOptions {
	mainBounds := normalizeWindowBoundsForLaunch(current.MainBounds, windowTypeMain)
	titles := resolveWindowTitles(current)
	options := buildWindowOptions(
		"main",
		titles.Main,
		"/",
		mainBounds,
		current,
		launchedByAutoStart,
		false,
	)
	if runtime.GOOS == "darwin" {
		options.MinimiseButtonState = application.ButtonHidden
		options.MaximiseButtonState = application.ButtonHidden
		options.CloseButtonState = application.ButtonHidden
	}
	return options
}

func buildSettingsWindowOptions(current dto.Settings, launchedByAutoStart bool) application.WebviewWindowOptions {
	settingsBounds := normalizeWindowBoundsForLaunch(current.SettingsBounds, windowTypeSettings)
	if settingsBounds.Width == 960 && settingsBounds.Height == 640 {
		settingsBounds.Width = settings.DefaultSettingsWidth
		settingsBounds.Height = settings.DefaultSettingsHeight
	}
	titles := resolveWindowTitles(current)
	return buildWindowOptions(
		"settings",
		titles.Settings,
		"/?window=settings",
		settingsBounds,
		current,
		launchedByAutoStart,
		true,
	)
}

func buildTrayMiniPlayerWindowOptions(current dto.Settings) application.WebviewWindowOptions {
	titles := resolveWindowTitles(current)
	backgroundType, background := trayMiniPlayerWindowBackground(current)
	return application.WebviewWindowOptions{
		Name:                       "tray-miniplayer",
		Title:                      titles.Main,
		Width:                      300,
		Height:                     132,
		MinWidth:                   300,
		MinHeight:                  132,
		MaxWidth:                   300,
		MaxHeight:                  132,
		URL:                        "/?window=tray-miniplayer",
		Hidden:                     true,
		AlwaysOnTop:                true,
		DisableResize:              true,
		Frameless:                  true,
		BackgroundType:             backgroundType,
		BackgroundColour:           background,
		HideOnFocusLost:            true,
		HideOnEscape:               true,
		DefaultContextMenuDisabled: true,
		Mac:                        trayMiniPlayerMacWindowOptions(current),
		Windows:                    trayMiniPlayerWindowsWindowOptions(current),
		Linux: application.LinuxWindow{
			WindowIsTranslucent: true,
		},
	}
}

func trayMiniPlayerWindowBackground(current dto.Settings) (application.BackgroundType, application.RGBA) {
	if runtime.GOOS == "windows" {
		return application.BackgroundTypeSolid, backgroundColour(current)
	}
	return application.BackgroundTypeTransparent, application.RGBA{Alpha: 0}
}

func buildWindowOptions(name, title, url string, bounds dto.WindowBounds, current dto.Settings, launchedByAutoStart bool, isSettings bool) application.WebviewWindowOptions {
	minWidth := settings.MinMainWindowWidth
	minHeight := settings.MinMainWindowHeight
	if isSettings {
		minWidth = settings.MinSettingsWindowWidth
		minHeight = settings.MinSettingsWindowHeight
	}
	options := application.WebviewWindowOptions{
		Name:             name,
		Title:            title,
		Width:            bounds.Width,
		Height:           bounds.Height,
		MinWidth:         minWidth,
		MinHeight:        minHeight,
		URL:              url,
		Frameless:        runtime.GOOS == "windows",
		BackgroundColour: backgroundColour(current),
		InitialPosition:  application.WindowCentered,
		EnableFileDrop:   !isSettings,
		Mac:              macWindowOptions(current),
		Windows:          windowsWindowOptions(current),
	}

	if bounds.X != 0 || bounds.Y != 0 {
		options.X = bounds.X
		options.Y = bounds.Y
		options.InitialPosition = application.WindowXY
	}

	if isSettings {
		options.Hidden = true
	}
	if !isSettings && shouldStartHidden(current, launchedByAutoStart) {
		options.Hidden = true
	}

	return options
}

func normalizeWindowBoundsForLaunch(bounds dto.WindowBounds, target windowType) dto.WindowBounds {
	minWidth := settings.MinMainWindowWidth
	minHeight := settings.MinMainWindowHeight
	if target == windowTypeSettings {
		minWidth = settings.MinSettingsWindowWidth
		minHeight = settings.MinSettingsWindowHeight
	}
	if bounds.Width < minWidth {
		bounds.Width = minWidth
	}
	if bounds.Height < minHeight {
		bounds.Height = minHeight
	}
	return bounds
}

func macWindowOptions(current dto.Settings) application.MacWindow {
	return application.MacWindow{
		Backdrop:                application.MacBackdropNormal,
		Appearance:              macAppearance(current),
		TitleBar:                application.MacTitleBarHiddenInset,
		InvisibleTitleBarHeight: 52,
	}
}

func trayMiniPlayerMacWindowOptions(current dto.Settings) application.MacWindow {
	return application.MacWindow{
		Backdrop:    application.MacBackdropTransparent,
		Appearance:  macAppearance(current),
		TitleBar:    application.MacTitleBarHidden,
		WindowLevel: application.MacWindowLevelPopUpMenu,
		CollectionBehavior: application.MacWindowCollectionBehaviorTransient |
			application.MacWindowCollectionBehaviorMoveToActiveSpace |
			application.MacWindowCollectionBehaviorIgnoresCycle,
	}
}

func macAppearance(current dto.Settings) application.MacAppearanceType {
	appearance := application.DefaultAppearance
	if current.Appearance == settings.AppearanceLight.String() {
		appearance = application.NSAppearanceNameAqua
	} else if current.Appearance == settings.AppearanceDark.String() {
		appearance = application.NSAppearanceNameDarkAqua
	}
	return appearance
}

func windowsWindowOptions(current dto.Settings) application.WindowsWindow {
	backdrop := application.None

	theme := application.SystemDefault
	if current.Appearance == settings.AppearanceLight.String() {
		theme = application.Light
	} else if current.Appearance == settings.AppearanceDark.String() {
		theme = application.Dark
	}

	return application.WindowsWindow{
		BackdropType: backdrop,
		Theme:        theme,
	}
}

func trayMiniPlayerWindowsWindowOptions(current dto.Settings) application.WindowsWindow {
	options := windowsWindowOptions(current)
	options.HiddenOnTaskbar = true
	options.DisableFramelessWindowDecorations = true
	return options
}

func backgroundColour(current dto.Settings) application.RGBA {
	isDark := current.EffectiveAppearance == settings.AppearanceDark.String()

	if isDark {
		return application.RGBA{Red: 18, Green: 18, Blue: 20, Alpha: 255}
	}

	return application.RGBA{Red: 245, Green: 245, Blue: 247, Alpha: 255}
}

func (manager *WindowManager) rebuildMenu(current dto.Settings) {
	buildMenu := func() {
		if manager.app == nil {
			return
		}
		menu := manager.app.NewMenu()
		if menu == nil {
			return
		}
		lang, err := settings.ParseLanguage(current.Language)
		if err != nil {
			lang = settings.DefaultLanguage
		}
		menuStrings := i18n.Menu(lang)

		appMenu := menu.AddSubmenu(menuStrings.AppTitle)
		appMenu.Add(menuStrings.About).SetRole(application.About).SetBitmap(menuIconBitmap())
		manager.appendUpdateMenuItem(appMenu, menuStrings)
		appMenu.Add(menuStrings.Settings).SetAccelerator("CmdOrCtrl+,").SetBitmap(menuIconBitmap()).OnClick(func(_ *application.Context) {
			manager.ShowSettingsWindow()
		})
		appMenu.AddSeparator()
		appMenu.Add(menuStrings.Hide).SetRole(application.Hide)
		appMenu.Add(menuStrings.HideOthers).SetRole(application.HideOthers)
		appMenu.Add(menuStrings.ShowAll).SetRole(application.UnHide)
		appMenu.AddSeparator()
		appMenu.Add(menuStrings.Quit).SetRole(application.Quit)

		fileMenu := menu.AddSubmenu(menuStrings.File)
		fileMenu.Add(menuStrings.Close).SetRole(application.CloseWindow)

		editMenu := menu.AddSubmenu(menuStrings.Edit)
		editMenu.AddRole(application.Undo)
		editMenu.AddRole(application.Redo)
		editMenu.AddSeparator()
		editMenu.AddRole(application.Cut)
		editMenu.AddRole(application.Copy)
		// On Windows (WebView2), registering native Paste role can cause Ctrl+V
		// to be applied twice in focused web inputs.
		if runtime.GOOS != "windows" {
			editMenu.AddRole(application.Paste)
		}
		editMenu.AddRole(application.Delete)
		editMenu.AddRole(application.SelectAll)

		if item := editMenu.FindByRole(application.Undo); item != nil {
			item.SetLabel(menuStrings.Undo)
		}
		if item := editMenu.FindByRole(application.Redo); item != nil {
			item.SetLabel(menuStrings.Redo)
		}
		if item := editMenu.FindByRole(application.Cut); item != nil {
			item.SetLabel(menuStrings.Cut)
		}
		if item := editMenu.FindByRole(application.Copy); item != nil {
			item.SetLabel(menuStrings.Copy)
		}
		if item := editMenu.FindByRole(application.Paste); item != nil {
			item.SetLabel(menuStrings.Paste)
		}
		if item := editMenu.FindByRole(application.Delete); item != nil {
			item.SetLabel(menuStrings.Delete)
		}
		if item := editMenu.FindByRole(application.SelectAll); item != nil {
			item.SetLabel(menuStrings.SelectAll)
		}

		windowMenu := menu.AddSubmenu(menuStrings.Window)
		windowMenu.Add(menuStrings.Minimize).SetRole(application.Minimise)
		windowMenu.Add(menuStrings.Zoom).SetRole(application.Zoom)
		if runtime.GOOS != "windows" {
			windowMenu.Add(menuStrings.FullScreen).SetRole(application.FullScreen)
		}
		windowMenu.AddSeparator()
		windowMenu.Add(menuStrings.BringAllToFront).SetRole(application.BringAllToFront)

		menu.AddSubmenu(menuStrings.Help)
		if manager.app.Menu != nil {
			manager.app.Menu.Set(menu)
		}
		manager.SetMenu(menu)
	}

	if manager.canInvokeSync() {
		application.InvokeSync(buildMenu)
	} else {
		buildMenu()
	}
}

func (manager *WindowManager) applyMenuBarVisibilityChange(value string) {
	go func() {
		ctx := context.Background()
		updated, err := manager.settingsService.UpdateSettings(ctx, dto.UpdateSettingsRequest{
			MenuBarVisibility: &value,
		})
		if err != nil {
			zap.L().Error("update menu bar visibility failed", zap.Error(err))
			return
		}
		manager.ApplySettings(updated)
	}()
}

func (manager *WindowManager) SetUpdateAvailable(available bool) {
	current, err := manager.settingsService.GetSettings(context.Background())
	if err != nil {
		zap.L().Warn("failed to refresh system tray after update flag change", zap.Error(err))
		return
	}
	updateTray := func() {
		manager.systemTray.SetUpdateAvailable(available, current)
	}
	if manager.canInvokeSync() {
		application.InvokeSync(updateTray)
		return
	}
	updateTray()
}

// NotifyUpdateState implements update.Notifier to drive menu/tray.
func (manager *WindowManager) NotifyUpdateState(info update.Info) {
	current, err := manager.settingsService.GetSettings(context.Background())
	if err != nil {
		return
	}
	updateTray := func() {
		manager.updateState = info
		manager.systemTray.SetUpdateState(info, current)
	}
	if manager.canInvokeSync() {
		application.InvokeSync(updateTray)
	} else {
		updateTray()
	}
	// rebuildMenu requires windows to be ready; guard nil.
	if manager.mainWindow != nil && manager.settingsWindow != nil {
		manager.rebuildMenu(current)
	}
}

func (manager *WindowManager) appendUpdateMenuItem(appMenu *application.Menu, menuStrings i18n.MenuStrings) {
	state := manager.updateState
	if state.Status == update.StatusChecking {
		appMenu.Add(menuStrings.CheckingForUpdate).SetEnabled(false)
		return
	}

	if state.IsUpdateAvailable() || state.Status == update.StatusReadyToRestart || state.Status == update.StatusInstalling {
		appMenu.Add(menuStrings.InstallUpdate).OnClick(func(_ *application.Context) {
			manager.ShowSettingsWindow()
			manager.emitNavigateToAbout()
		})
	}
}

func (manager *WindowManager) emitNavigateToAbout() {
	manager.app.Event.Emit("settings:navigate", "about")
}

func (manager *WindowManager) emitOpenNewDownload() {
	manager.app.Event.Emit("main:new-download")
}

func resolveWindowTitles(current dto.Settings) i18n.WindowTitleStrings {
	lang, err := settings.ParseLanguage(current.Language)
	if err != nil {
		lang = settings.DefaultLanguage
	}
	return i18n.WindowTitles(lang)
}

func (manager *WindowManager) syncWindowPresentation(current dto.Settings) {
	if manager == nil {
		return
	}
	titles := resolveWindowTitles(current)
	if manager.mainWindow != nil {
		manager.mainWindow.SetTitle(titles.Main)
		manager.mainWindow.SetMinSize(settings.MinMainWindowWidth, settings.MinMainWindowHeight)
	}
	if manager.settingsWindow != nil {
		manager.settingsWindow.SetTitle(titles.Settings)
		manager.settingsWindow.SetMinSize(settings.MinSettingsWindowWidth, settings.MinSettingsWindowHeight)
	}
	if manager.trayMiniPlayer != nil {
		manager.trayMiniPlayer.SetTitle(titles.Main)
	}
	manager.enforceMinimumSize(windowTypeMain)
	manager.enforceMinimumSize(windowTypeSettings)
}

func (manager *WindowManager) enforceMinimumSize(target windowType) {
	if runtime.GOOS != "windows" {
		return
	}
	var (
		window    *application.WebviewWindow
		minWidth  int
		minHeight int
	)
	switch target {
	case windowTypeSettings:
		window = manager.settingsWindow
		minWidth = settings.MinSettingsWindowWidth
		minHeight = settings.MinSettingsWindowHeight
	default:
		window = manager.mainWindow
		minWidth = settings.MinMainWindowWidth
		minHeight = settings.MinMainWindowHeight
	}
	if window == nil {
		return
	}
	bounds := window.Bounds()
	width := bounds.Width
	height := bounds.Height
	if width >= minWidth && height >= minHeight {
		return
	}
	if width < minWidth {
		width = minWidth
	}
	if height < minHeight {
		height = minHeight
	}
	window.SetSize(width, height)
}

func shouldStartHidden(current dto.Settings, launchedByAutoStart bool) bool {
	return current.MinimizeToTrayOnStart && launchedByAutoStart
}

var (
	menuIconOnce sync.Once
	menuIconData []byte
)

func menuIconBitmap() []byte {
	menuIconOnce.Do(func() {
		// Simple 16x16 neutral icon (gray circle) in PNG.
		const base64Icon = "iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAMAAAAoLQ9TAAAARVBMVEX////MzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzP///9kl0ZhAAAAFHRSTlMACg4lRUtTV2Z6+fz9/v7+/v7+/v5qO+YAAABRSURBVHgBjY7bDoAgDERRK8qS/f+X7iVbVAnCukW/qc8D7pgLLgQLH9hEdPzmuC8NioqKC3zGlwP9r0RhejKnR1ksG/0AARy1RZf9joGBJ0D1UwTgBC+wDfvB07iH4AAAAASUVORK5CYII="
		if decoded, err := base64.StdEncoding.DecodeString(base64Icon); err == nil {
			menuIconData = decoded
		}
	})
	return menuIconData
}
