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
	app                  *application.App
	mainWindow           *application.WebviewWindow
	secondaryWindowsMu   sync.RWMutex
	settingsWindow       *application.WebviewWindow
	trayMiniPlayer       *application.WebviewWindow
	settingsWindowOnce   sync.Once
	trayMiniPlayerOnce   sync.Once
	currentSettings      dto.Settings
	currentMenu          *application.Menu
	secondaryRevision    uint64
	settingsService      *service.SettingsService
	appVersion           string
	startupIcon          []byte
	startupAppearance    string
	mainWindowMu         sync.Mutex
	mainBoot             mainWindowBootState
	settingsVisible      bool
	boundsMu             sync.Mutex
	lastMainBounds       dto.WindowBounds
	lastSettingsBounds   dto.WindowBounds
	mainBoundsDirty      bool
	settingsBoundsDirty  bool
	initialized          bool
	updateState          update.Info
	quitting             atomic.Bool
	applicationStarted   atomic.Bool
	mainBoundsReady      atomic.Bool
	settingsBoundsReady  atomic.Bool
	mainHTMLSurfaceReady atomic.Bool
	mainBootFallback     atomic.Bool

	systemTray *SystemTrayController
}

// mainWindowBootState separates the user's visibility intent from the two
// startup milestones. A native surface may make the hidden window safe to show
// before the frontend is ready; frontendReady later permits the native surface
// to be removed. Recording the applied visibility prevents the second milestone
// from focusing an already visible window again.
type mainWindowBootState struct {
	mu                 sync.Mutex
	applicationStarted bool
	nativeSurfaceReady bool
	frontendReady      bool
	fallbackReady      bool
	visibleRequested   bool
	nativeVisible      bool
}

func newMainWindowBootState(visibleRequested bool) mainWindowBootState {
	return mainWindowBootState{visibleRequested: visibleRequested}
}

func (state *mainWindowBootState) isReady() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.frontendReady
}

func (state *mainWindowBootState) isSettled() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.frontendReady || state.fallbackReady
}

func (state *mainWindowBootState) isNativeSurfaceReady() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.nativeSurfaceReady
}

func (state *mainWindowBootState) claimRevealLocked() bool {
	if state.nativeVisible ||
		!state.applicationStarted ||
		!state.visibleRequested ||
		(!state.nativeSurfaceReady && !state.frontendReady && !state.fallbackReady) {
		return false
	}
	state.nativeVisible = true
	return true
}

// markReady records that React has committed a stable first frame. The first
// return value reports the idempotent state transition; the second reports
// whether a window without a native startup surface should now be revealed.
func (state *mainWindowBootState) markReady() (bool, bool) {
	if state == nil {
		return false, false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.frontendReady {
		return false, false
	}
	state.frontendReady = true
	return true, state.claimRevealLocked()
}

func (state *mainWindowBootState) markNativeSurfaceReady() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.nativeSurfaceReady = true
	return state.claimRevealLocked()
}

func (state *mainWindowBootState) isFallbackReady() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.fallbackReady
}

func (state *mainWindowBootState) markFallbackReady() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.fallbackReady = true
	return state.claimRevealLocked()
}

func (state *mainWindowBootState) shouldApplyReveal() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.nativeVisible &&
		state.applicationStarted &&
		state.visibleRequested &&
		(state.nativeSurfaceReady || state.frontendReady || state.fallbackReady)
}

func (state *mainWindowBootState) markApplicationStarted() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.applicationStarted {
		return false
	}
	state.applicationStarted = true
	return state.claimRevealLocked()
}

// requestShow records visibility intent and applies it as soon as either the
// native startup surface or the stable frontend is available.
func (state *mainWindowBootState) requestShow() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.visibleRequested = true
	if !state.applicationStarted ||
		(!state.nativeSurfaceReady && !state.frontendReady && !state.fallbackReady) {
		return false
	}
	// An explicit show is also a request to restore and focus an already-visible
	// window (for example from the Dock, tray, or a second-instance launch).
	// Startup milestones use claimRevealLocked to avoid stealing focus twice;
	// user actions must not be deduplicated by nativeVisible.
	state.nativeVisible = true
	return true
}

func (state *mainWindowBootState) requestHide() {
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.visibleRequested = false
	state.nativeVisible = false
}

type windowTrayActions struct {
	manager *WindowManager
	app     *application.App
}

func (actions windowTrayActions) ToggleMiniPlayer() bool {
	if actions.manager == nil {
		return false
	}
	return actions.manager.ToggleTrayMiniPlayer()
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

func NewWindowManager(app *application.App, settingsService *service.SettingsService, appVersion string, startupIcon []byte, trayIcon []byte, launchedByAutoStart bool) (*WindowManager, error) {
	current, err := settingsService.GetSettings(context.Background())
	if err != nil {
		return nil, err
	}
	mainWindowOptions := buildMainWindowOptions(current, launchedByAutoStart)
	mainWindow := app.Window.NewWithOptions(withRemoteWebViewPermissionPolicy(mainWindowOptions))
	registerWebViewRemoteCapabilityPolicy(mainWindow)
	startHidden := shouldStartHidden(current, launchedByAutoStart)

	manager := &WindowManager{
		app:                app,
		mainWindow:         mainWindow,
		currentSettings:    current,
		settingsService:    settingsService,
		appVersion:         appVersion,
		startupIcon:        append([]byte(nil), startupIcon...),
		startupAppearance:  current.EffectiveAppearance,
		mainBoot:           newMainWindowBootState(!startHidden),
		settingsVisible:    false,
		lastMainBounds:     current.MainBounds,
		lastSettingsBounds: current.SettingsBounds,
	}
	registerMainWindowStartupOverlayEvents(manager, mainWindow)

	manager.systemTray = NewSystemTrayController(app, windowTrayActions{
		manager: manager,
		app:     app,
	}, trayIcon)

	manager.ApplySettings(current)
	manager.registerMainWindowEvents()
	manager.registerDockEvents()
	manager.initialized = true

	return manager, nil
}

func (manager *WindowManager) MarkApplicationStarted() {
	if manager == nil {
		return
	}
	if manager.applicationStarted.Swap(true) {
		return
	}
	manager.restoreStartupWindowBounds()
	manager.mainWindowMu.Lock()
	shouldReveal := manager.mainBoot.markApplicationStarted()
	manager.mainWindowMu.Unlock()
	if shouldReveal {
		manager.revealMainWindowIfCurrent()
	}
}

func (manager *WindowManager) canInvokeSync() bool {
	return manager != nil && manager.initialized && manager.applicationStarted.Load()
}

func (manager *WindowManager) MainBootReady() bool {
	return manager != nil && manager.mainBoot.isReady()
}

// MainBootSettled reports that startup is no longer competing for the first
// usable surface, whether React completed normally or the recovery fallback
// took ownership after a frontend failure.
func (manager *WindowManager) MainBootSettled() bool {
	return manager != nil && manager.mainBoot.isSettled()
}

// MainWindowVisible is the native source of truth for user-visible station
// telemetry. In particular, a hidden autostart WebView can be fully mounted
// while it is not yet a user-visible app session.
func (manager *WindowManager) MainWindowVisible() bool {
	return manager != nil &&
		manager.mainWindow != nil &&
		manager.mainWindow.IsVisible() &&
		!manager.mainWindow.IsMinimised()
}

// MarkMainWindowBootReady completes the native/frontend startup handshake. It
// is safe to call more than once. On macOS the window is already visible behind
// its native overlay, so this transition removes the overlay instead of showing
// or refocusing the window.
func (manager *WindowManager) MarkMainWindowBootReady() {
	if manager == nil {
		return
	}
	manager.markMainWindowBootReady()
}

func (manager *WindowManager) markMainWindowBootReady() bool {
	manager.mainWindowMu.Lock()
	becameReady, shouldReveal := manager.mainBoot.markReady()
	manager.mainWindowMu.Unlock()

	if manager.mainWindow != nil && !manager.quitting.Load() {
		application.InvokeSync(func() {
			if manager.quitting.Load() || manager.mainWindow == nil {
				return
			}
			dismissMainWindowStartupOverlay(manager.mainWindow.NativeWindow())
		})
	}
	if shouldReveal {
		manager.revealMainWindowIfCurrent()
	}
	return becameReady
}

// ensureMainWindowStartupOverlay is called by macOS' provisional-navigation
// event, after Wails has published a valid NSWindow handle. The window remains
// hidden until the native layer is fully installed, so no empty translucent
// frame can escape.
func (manager *WindowManager) ensureMainWindowStartupOverlay() {
	if manager == nil || manager.mainWindow == nil || manager.MainBootReady() {
		return
	}
	manager.mainWindowMu.Lock()
	if manager.mainBoot.isNativeSurfaceReady() ||
		manager.mainBoot.isReady() ||
		manager.mainBoot.isFallbackReady() ||
		manager.quitting.Load() {
		manager.mainWindowMu.Unlock()
		return
	}
	installed := false
	application.InvokeSync(func() {
		if manager.quitting.Load() || manager.mainWindow == nil {
			return
		}
		installed = installMainWindowStartupOverlay(
			manager.mainWindow.NativeWindow(),
			manager.startupIcon,
			manager.startupAppearance,
		)
	})
	shouldReveal := installed && manager.mainBoot.markNativeSurfaceReady()
	manager.mainWindowMu.Unlock()

	if shouldReveal {
		manager.revealMainWindowIfCurrent()
	}
}

const defaultMainWindowBootReadyFallbackTimeout = 5 * time.Second
const nativeMainWindowBootReadyFallbackTimeout = 12 * time.Second

// StartMainWindowBootReadyFallback prevents a broken frontend handshake from
// leaving a manually launched app permanently hidden. Marking ready still
// honours the current visibility intent, so autostart/minimise-to-tray launches
// remain hidden.
func (manager *WindowManager) StartMainWindowBootReadyFallback(ctx context.Context) {
	if manager == nil {
		return
	}
	go func() {
		timeout := defaultMainWindowBootReadyFallbackTimeout
		if supportsMainWindowStartupOverlay() {
			timeout = nativeMainWindowBootReadyFallbackTimeout
		}
		if !awaitMainWindowBootReadyFallback(
			ctx,
			timeout,
			manager.MainBootReady,
		) || manager.quitting.Load() {
			return
		}
		manager.mainBootFallback.Store(true)
		// Never remove a native surface unless WebKit has finished navigation and
		// the inline HTML recovery shell is known to exist underneath it.
		if supportsMainWindowStartupOverlay() && !manager.mainHTMLSurfaceReady.Load() {
			zap.L().Warn(
				"main window frontend-ready handshake timed out before HTML loaded; keeping native startup surface",
				zap.Duration("timeout", timeout),
			)
			return
		}
		if manager.releaseMainWindowBootFallback() {
			zap.L().Warn(
				"main window boot-ready handshake timed out; showing fallback surface",
				zap.Duration("timeout", timeout),
			)
		}
	}()
}

func (manager *WindowManager) markMainWindowHTMLSurfaceReady() {
	if manager == nil {
		return
	}
	manager.mainHTMLSurfaceReady.Store(true)
	// A timeout or explicit frontend failure may race with WebKit's navigation
	// callback. Latching the request guarantees whichever event arrives second
	// exposes the inline recovery surface instead of leaving the native overlay
	// up forever.
	if manager.mainBootFallback.Load() && !manager.quitting.Load() {
		manager.releaseMainWindowBootFallback()
	}
}

// ReleaseMainWindowBootFallback is used after an explicit frontend startup
// failure. It only reveals the inline recovery shell once navigation has made
// that surface real; a blocked dev server therefore keeps the native icon.
func (manager *WindowManager) ReleaseMainWindowBootFallback() {
	if manager == nil {
		return
	}
	manager.mainBootFallback.Store(true)
	if !canReleaseMainWindowBootFallback(
		supportsMainWindowStartupOverlay(),
		manager.mainHTMLSurfaceReady.Load(),
	) {
		return
	}
	manager.releaseMainWindowBootFallback()
}

func canReleaseMainWindowBootFallback(nativeOverlay, htmlSurfaceReady bool) bool {
	// A frontend failure can only call this method after index.html and the
	// bootstrap module exist. Platforms without a native overlay therefore have
	// a usable HTML recovery surface even though Wails does not expose their
	// navigation-finished event. macOS additionally waits for its explicit
	// WebKit callback before removing the native layer.
	return !nativeOverlay || htmlSurfaceReady
}

func (manager *WindowManager) releaseMainWindowBootFallback() bool {
	if manager == nil {
		return false
	}
	manager.mainWindowMu.Lock()
	alreadyReleased := manager.mainBoot.isFallbackReady()
	shouldReveal := manager.mainBoot.markFallbackReady()
	manager.mainWindowMu.Unlock()

	if manager.mainWindow != nil && !manager.quitting.Load() {
		application.InvokeSync(func() {
			if manager.quitting.Load() || manager.mainWindow == nil {
				return
			}
			dismissMainWindowStartupOverlay(manager.mainWindow.NativeWindow())
		})
	}
	if shouldReveal {
		manager.revealMainWindowIfCurrent()
	}
	return !alreadyReleased
}

func awaitMainWindowBootReadyFallback(
	ctx context.Context,
	timeout time.Duration,
	isReady func() bool,
) bool {
	if isReady == nil || isReady() {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return !isReady()
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return !isReady()
	}
}

func (manager *WindowManager) ShowMainWindow() {
	if manager == nil {
		return
	}
	manager.mainWindowMu.Lock()
	shouldReveal := manager.mainBoot.requestShow()
	manager.mainWindowMu.Unlock()
	if shouldReveal {
		manager.revealMainWindowIfCurrent()
	}
}

func (manager *WindowManager) revealMainWindowIfCurrent() {
	if manager == nil {
		return
	}
	manager.mainWindowMu.Lock()
	defer manager.mainWindowMu.Unlock()
	if !manager.mainBoot.shouldApplyReveal() {
		return
	}
	manager.revealMainWindow()
}

func (manager *WindowManager) revealMainWindow() {
	if manager == nil || manager.mainWindow == nil {
		return
	}
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

// ensureSettingsWindow creates the settings WebView on first use. Secondary
// WebViews are deliberately excluded from App.Run's pending window list so
// WebKit only has to initialise the visible main surface during startup.
func (manager *WindowManager) ensureSettingsWindow() *application.WebviewWindow {
	if manager == nil || manager.app == nil || manager.quitting.Load() || !manager.applicationStarted.Load() {
		return nil
	}

	manager.settingsWindowOnce.Do(func() {
		manager.secondaryWindowsMu.RLock()
		current := manager.currentSettings
		manager.secondaryWindowsMu.RUnlock()
		window := application.NewWindow(withRemoteWebViewPermissionPolicy(
			buildSettingsWindowOptions(current, false),
		))
		registerWebViewRemoteCapabilityPolicy(window)
		manager.registerSettingsWindowEvents(window)
		// Add the policy-configured window before Run. NewWithOptions would run a
		// window immediately once the application is live, leaving no opportunity
		// to install the remote-capability and lifecycle hooks before navigation.
		manager.app.Window.Add(window)
		window.Run()

		// Publish only after the native window is live and has caught up with the
		// latest presentation revision. UI calls stay outside the state lock so a
		// synchronous native callback cannot deadlock while it reads the manager.
		for {
			manager.secondaryWindowsMu.RLock()
			current = manager.currentSettings
			menu := manager.currentMenu
			revision := manager.secondaryRevision
			manager.secondaryWindowsMu.RUnlock()

			window.SetTitle(resolveWindowTitles(current).Settings)
			window.SetBackgroundColour(resolveSettingsWindowBackground(
				runtime.GOOS,
				resolveSettingsWindowSurfaceStyle(current.AppearanceConfig),
				backgroundColour(current),
			))
			if menu != nil {
				window.SetMenu(menu)
			}
			if shouldHideNativeMenuBar(runtime.GOOS, manager.appVersion) {
				window.HideMenuBar()
			}

			manager.secondaryWindowsMu.Lock()
			if revision == manager.secondaryRevision {
				manager.settingsWindow = window
				manager.secondaryWindowsMu.Unlock()
				break
			}
			manager.secondaryWindowsMu.Unlock()
		}
	})

	return manager.settingsWindowSnapshot()
}

// ensureTrayMiniPlayer creates and attaches the tray WebView only when the
// user first asks for it. The tray icon and menu remain available from launch.
func (manager *WindowManager) ensureTrayMiniPlayer() *application.WebviewWindow {
	if manager == nil || manager.app == nil || manager.systemTray == nil || manager.quitting.Load() || !manager.applicationStarted.Load() {
		return nil
	}

	manager.trayMiniPlayerOnce.Do(func() {
		manager.secondaryWindowsMu.RLock()
		current := manager.currentSettings
		manager.secondaryWindowsMu.RUnlock()
		window := application.NewWindow(withRemoteWebViewPermissionPolicy(
			buildTrayMiniPlayerWindowOptions(current),
		))
		registerWebViewRemoteCapabilityPolicy(window)

		manager.app.Window.Add(window)
		window.Run()
		for {
			manager.secondaryWindowsMu.RLock()
			current = manager.currentSettings
			revision := manager.secondaryRevision
			manager.secondaryWindowsMu.RUnlock()

			window.SetTitle(resolveWindowTitles(current).Main)
			_, background := trayMiniPlayerWindowBackground(current)
			window.SetBackgroundColour(background)

			manager.secondaryWindowsMu.Lock()
			if revision == manager.secondaryRevision {
				manager.trayMiniPlayer = window
				manager.secondaryWindowsMu.Unlock()
				break
			}
			manager.secondaryWindowsMu.Unlock()
		}
		manager.systemTray.AttachMiniPlayer(window)
	})

	return manager.trayMiniPlayerSnapshot()
}

func (manager *WindowManager) settingsWindowSnapshot() *application.WebviewWindow {
	if manager == nil {
		return nil
	}
	manager.secondaryWindowsMu.RLock()
	defer manager.secondaryWindowsMu.RUnlock()
	return manager.settingsWindow
}

func (manager *WindowManager) trayMiniPlayerSnapshot() *application.WebviewWindow {
	if manager == nil {
		return nil
	}
	manager.secondaryWindowsMu.RLock()
	defer manager.secondaryWindowsMu.RUnlock()
	return manager.trayMiniPlayer
}

func (manager *WindowManager) ShowSettingsWindow() {
	window := manager.ensureSettingsWindow()
	if window == nil {
		return
	}
	manager.secondaryWindowsMu.Lock()
	manager.settingsVisible = true
	manager.secondaryWindowsMu.Unlock()
	manager.restoreCachedBounds(windowTypeSettings)
	manager.ensureWindowVisible(windowTypeSettings)
	window.UnMinimise()
	window.Show()
	window.Focus()
	manager.markBoundsTrackingReady(windowTypeSettings)
}

func (manager *WindowManager) ToggleTrayMiniPlayer() bool {
	if manager.ensureTrayMiniPlayer() == nil {
		return false
	}
	return manager.systemTray.ToggleMiniPlayer()
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
	manager.mainWindow.SetFullscreenButtonState(state)
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
	return manager.selectDirectoryDialog(title, initialDir, manager.settingsWindowSnapshot())
}

func (manager *WindowManager) SelectMainDirectoryDialog(title string, initialDir string) (string, error) {
	return manager.selectDirectoryDialog(title, initialDir, manager.mainWindow)
}

type SaveFileDialogFilter struct {
	DisplayName string
	Pattern     string
}

type SaveFileDialogOptions struct {
	Title               string
	Message             string
	Filename            string
	ButtonText          string
	Directory           string
	Filters             []SaveFileDialogFilter
	AllowOtherFileTypes bool
	HideExtension       bool
}

// SaveMainFileDialog presents a native save panel attached to the main
// window. Cancellation is returned as an empty path without an error, matching
// the existing file and directory picker wrappers.
func (manager *WindowManager) SaveMainFileDialog(options SaveFileDialogOptions) (string, error) {
	if manager == nil || manager.app == nil {
		return "", fmt.Errorf("app not available")
	}
	filters := make([]application.FileFilter, 0, len(options.Filters))
	for _, filter := range options.Filters {
		displayName := strings.TrimSpace(filter.DisplayName)
		pattern := strings.TrimSpace(filter.Pattern)
		if displayName == "" || pattern == "" {
			continue
		}
		filters = append(filters, application.FileFilter{
			DisplayName: displayName,
			Pattern:     pattern,
		})
	}
	dialogOptions := &application.SaveFileDialogOptions{
		CanCreateDirectories: true,
		AllowOtherFileTypes:  options.AllowOtherFileTypes,
		HideExtension:        options.HideExtension,
		Title:                strings.TrimSpace(options.Title),
		Message:              strings.TrimSpace(options.Message),
		Filename:             strings.TrimSpace(options.Filename),
		ButtonText:           strings.TrimSpace(options.ButtonText),
		Filters:              filters,
	}
	if directory := resolveExistingDialogDirectory(options.Directory); directory != "" {
		dialogOptions.Directory = directory
	}
	dialog := manager.app.Dialog.SaveFile()
	dialog.SetOptions(dialogOptions)
	if manager.mainWindow != nil {
		dialog = dialog.AttachToWindow(manager.mainWindow)
	}
	selected, err := dialog.PromptForSingleSelection()
	if isDialogCancelledError(err) {
		return "", nil
	}
	return selected, err
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
	if manager == nil {
		return
	}
	manager.mainWindowMu.Lock()
	defer manager.mainWindowMu.Unlock()
	manager.mainBoot.requestHide()
	if manager.mainWindow == nil {
		return
	}
	manager.persistBoundsOrCached(windowTypeMain, "hide-main")
	manager.mainWindow.Hide()
}

func (manager *WindowManager) HideSettingsWindow() {
	if manager == nil {
		return
	}
	window := manager.settingsWindowSnapshot()
	if window == nil {
		return
	}
	manager.persistBoundsOrCached(windowTypeSettings, "hide-settings")
	manager.secondaryWindowsMu.Lock()
	manager.settingsVisible = false
	manager.secondaryWindowsMu.Unlock()
	window.Hide()
}

func (manager *WindowManager) SetMenu(menu *application.Menu) {
	if manager == nil {
		return
	}
	manager.secondaryWindowsMu.Lock()
	manager.currentMenu = menu
	manager.secondaryRevision++
	settingsWindow := manager.settingsWindow
	manager.secondaryWindowsMu.Unlock()
	if manager.mainWindow != nil {
		manager.mainWindow.SetMenu(menu)
		if shouldHideNativeMenuBar(runtime.GOOS, manager.appVersion) {
			manager.mainWindow.HideMenuBar()
		}
	}
	if settingsWindow != nil {
		settingsWindow.SetMenu(menu)
		if shouldHideNativeMenuBar(runtime.GOOS, manager.appVersion) {
			settingsWindow.HideMenuBar()
		}
	}
}

func (manager *WindowManager) ApplySettings(current dto.Settings) {
	if manager == nil {
		return
	}
	manager.secondaryWindowsMu.Lock()
	manager.currentSettings = current
	manager.secondaryRevision++
	settingsWindow := manager.settingsWindow
	manager.secondaryWindowsMu.Unlock()

	apply := func() {
		color := backgroundColour(current)
		backgrounds := resolveWindowRuntimeBackgrounds(runtime.GOOS, color)
		settingsBackground := resolveSettingsWindowBackground(
			runtime.GOOS,
			resolveSettingsWindowSurfaceStyle(current.AppearanceConfig),
			color,
		)
		manager.syncWindowPresentation(current)
		manager.mainWindow.SetBackgroundColour(backgrounds.main)
		// The native video host remains an opaque content plane. Settings reveals
		// its preinstalled underlay only while the window-wide style is Glass.
		syncListenNativeVideoHostBackground(manager.mainWindow, color)
		if settingsWindow != nil {
			settingsWindow.SetBackgroundColour(settingsBackground)
		}
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
	settingsWindow := manager.settingsWindowSnapshot()
	trayMiniPlayer := manager.trayMiniPlayerSnapshot()
	if settingsWindow != nil {
		settingsWindow.DispatchWailsEvent(event)
	}
	if trayMiniPlayer != nil {
		trayMiniPlayer.DispatchWailsEvent(event)
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

func (manager *WindowManager) registerSettingsWindowEvents(settingsWindow *application.WebviewWindow) {
	if manager == nil || settingsWindow == nil {
		return
	}
	settingsDebounce := debounce.New(600 * time.Millisecond)

	settingsWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		manager.restoreCachedBounds(windowTypeSettings)
		manager.ensureWindowVisible(windowTypeSettings)
		manager.markBoundsTrackingReady(windowTypeSettings)
	})

	settingsWindow.OnWindowEvent(events.Common.WindowDidMove, func(_ *application.WindowEvent) {
		if ignoreCommonWindowBoundsEvents() {
			return
		}
		manager.rememberCurrentBounds(windowTypeSettings)
		settingsDebounce(func() {
			manager.persistBoundsOrCached(windowTypeSettings, "common-window-did-move-debounced")
		})
	})

	settingsWindow.OnWindowEvent(events.Common.WindowDidResize, func(_ *application.WindowEvent) {
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
		settingsWindow.OnWindowEvent(events.Windows.WindowEndMove, func(_ *application.WindowEvent) {
			manager.persistBoundsOrCached(windowTypeSettings, "windows-window-end-move")
		})
		settingsWindow.OnWindowEvent(events.Windows.WindowEndResize, func(_ *application.WindowEvent) {
			manager.persistBoundsOrCached(windowTypeSettings, "windows-window-end-resize")
		})
	}

	// Use hook to cancel default destroy flow and just hide.
	settingsWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
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
	return manager.settingsWindowSnapshot()
}

func (manager *WindowManager) windowBounds(target windowType) application.Rect {
	window := manager.windowForType(target)
	if window == nil {
		return application.Rect{}
	}
	return window.Bounds()
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

func buildMainWindowOptions(current dto.Settings, _ bool) application.WebviewWindowOptions {
	mainBounds := normalizeWindowBoundsForLaunch(current.MainBounds, windowTypeMain)
	titles := resolveWindowTitles(current)
	options := buildWindowOptions(
		"main",
		titles.Main,
		"/",
		mainBounds,
		current,
		false,
	)
	options.JS = mainWindowStartupThemeScript(current.EffectiveAppearance)
	applyMainWindowMaterialPolicy(&options, runtime.GOOS)
	applyMainWindowCompositionPolicy(&options, runtime.GOOS)
	applyMainWindowControlPolicy(&options, runtime.GOOS)
	return options
}

func mainWindowStartupThemeScript(effectiveAppearance string) string {
	switch effectiveAppearance {
	case settings.AppearanceLight.String():
		return `try { localStorage.setItem("xiadown:startup-theme", "light"); } catch {} document.documentElement.dataset.startupTheme = "light";`
	case settings.AppearanceDark.String():
		return `try { localStorage.setItem("xiadown:startup-theme", "dark"); } catch {} document.documentElement.dataset.startupTheme = "dark";`
	default:
		return ""
	}
}

// macOS owns the traffic-light controls even with MacTitleBarHiddenInset. In
// native fullscreen AppKit reveals that title/toolbar strip when the pointer
// reaches the menu bar; keeping the standard buttons enabled gives users the
// native green exit-fullscreen affordance in addition to Escape.
func applyMainWindowControlPolicy(options *application.WebviewWindowOptions, goos string) {
	if options == nil || goos != "darwin" {
		return
	}
	options.MinimiseButtonState = application.ButtonEnabled
	options.MaximiseButtonState = application.ButtonEnabled
	options.FullscreenButtonState = application.ButtonEnabled
	options.CloseButtonState = application.ButtonEnabled
}

func buildSettingsWindowOptions(current dto.Settings, _ bool) application.WebviewWindowOptions {
	settingsBounds := normalizeWindowBoundsForLaunch(current.SettingsBounds, windowTypeSettings)
	if settingsBounds.Width == 960 && settingsBounds.Height == 640 {
		settingsBounds.Width = settings.DefaultSettingsWidth
		settingsBounds.Height = settings.DefaultSettingsHeight
	}
	titles := resolveWindowTitles(current)
	surfaceStyle := resolveSettingsWindowSurfaceStyle(current.AppearanceConfig)
	options := buildWindowOptions(
		"settings",
		titles.Settings,
		fmt.Sprintf(
			"/?window=settings&surfaceStyle=%s",
			surfaceStyle,
		),
		settingsBounds,
		current,
		true,
	)
	applySettingsWindowMaterialPolicy(
		&options,
		runtime.GOOS,
		surfaceStyle,
	)
	return options
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

func buildWindowOptions(name, title, url string, bounds dto.WindowBounds, current dto.Settings, isSettings bool) application.WebviewWindowOptions {
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
	// WindowManager explicitly shows this window only after either the native
	// startup overlay or the stable frontend is ready. Wails' automatic show is
	// navigation-dependent and can otherwise expose an empty translucent frame.
	options.Hidden = true

	if bounds.X != 0 || bounds.Y != 0 {
		options.X = bounds.X
		options.Y = bounds.Y
		options.InitialPosition = application.WindowXY
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
		WebviewPreferences: application.MacWebviewPreferences{
			JavaScriptCanOpenWindowsAutomatically: application.Disabled,
		},
	}
}

func trayMiniPlayerMacWindowOptions(current dto.Settings) application.MacWindow {
	return application.MacWindow{
		Backdrop:    application.MacBackdropTransparent,
		Appearance:  macAppearance(current),
		TitleBar:    application.MacTitleBarHidden,
		WindowLevel: application.MacWindowLevelPopUpMenu,
		WebviewPreferences: application.MacWebviewPreferences{
			JavaScriptCanOpenWindowsAutomatically: application.Disabled,
		},
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

		// Keep the standard reload, force-reload and DevTools commands reachable
		// while running `wails3 dev`. The custom production menu intentionally
		// stays unchanged.
		if shouldExposeDeveloperMenu(manager.appVersion) {
			menu.AddRole(application.ViewMenu)
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

func shouldExposeDeveloperMenu(version string) bool {
	return !isReleaseAppVersion(version)
}

func shouldHideNativeMenuBar(goos string, version string) bool {
	return goos == "windows" && !shouldExposeDeveloperMenu(version)
}

func isReleaseAppVersion(version string) bool {
	normalized := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if normalized == "" {
		return false
	}
	for _, part := range strings.Split(normalized, ".") {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
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
	// The settings window is lazy; rebuilding the app/main menu must not force
	// its creation just because the update state changed.
	if manager.mainWindow != nil {
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
			manager.emitNavigateToAbout()
		})
	}
}

func (manager *WindowManager) emitNavigateToAbout() {
	if manager == nil || manager.app == nil {
		return
	}
	manager.ShowSettingsWindow()
	window := manager.settingsWindowSnapshot()
	if window == nil {
		return
	}
	// Keep a durable hand-off for the first lazy load, then emit the live event
	// for an already-mounted settings app. ExecJS queues until Wails' runtime is
	// ready, and SettingsApp consumes this shared localStorage key on mount.
	window.ExecJS(`try { const key = "xiadown:settings-tab"; localStorage.setItem(key, "about"); window.dispatchEvent(new StorageEvent("storage", { key, newValue: "about" })); } catch {}`)
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
	settingsWindow := manager.settingsWindowSnapshot()
	trayMiniPlayer := manager.trayMiniPlayerSnapshot()
	titles := resolveWindowTitles(current)
	if manager.mainWindow != nil {
		manager.mainWindow.SetTitle(titles.Main)
	}
	if settingsWindow != nil {
		settingsWindow.SetTitle(titles.Settings)
	}
	if trayMiniPlayer != nil {
		trayMiniPlayer.SetTitle(titles.Main)
	}
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
		window = manager.settingsWindowSnapshot()
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
