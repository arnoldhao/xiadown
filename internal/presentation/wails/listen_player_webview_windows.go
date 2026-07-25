//go:build windows

package wails

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/sitepolicy"
	"xiadown/internal/application/youtubecookies"
	"xiadown/internal/application/youtubemusic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"github.com/wailsapp/wails/webview2/pkg/edge"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

var listenYouTubeMusicRuntimeReadyWindowIDs sync.Map
var listenWindowsWebViewConfiguredWindowIDs sync.Map
var listenWindowsWebResourceHeaderWindowIDs sync.Map
var listenWindowsEmbeddedFullscreenWindowIDs sync.Map
var listenWindowsEmbeddedHostControllers sync.Map
var listenWindowsRemoteNavigationPolicies sync.Map
var listenWindowsPersistentPopupPolicies sync.Map
var listenWindowsMediaVisibilityWindows sync.Map
var listenWindowsPendingDocumentStartRegistrations sync.Map
var listenWindowsRSSBilibiliRefererGuard listenNativeWindowFeatureGuard

var (
	listenWindowsUser32              = syscall.NewLazyDLL("user32.dll")
	listenWindowsGDI32               = syscall.NewLazyDLL("gdi32.dll")
	listenWindowsProcGetAncestor     = listenWindowsUser32.NewProc("GetAncestor")
	listenWindowsProcGetParent       = listenWindowsUser32.NewProc("GetParent")
	listenWindowsProcSetParent       = listenWindowsUser32.NewProc("SetParent")
	listenWindowsProcSetWindowRgn    = listenWindowsUser32.NewProc("SetWindowRgn")
	listenWindowsProcCreateRoundRgn  = listenWindowsGDI32.NewProc("CreateRoundRectRgn")
	listenWindowsEmbeddedWebViewLock sync.Mutex
	listenWindowsEmbeddedWebView     listenWindowsEmbeddedWebViewState
	listenWindowsMediaWebViewParking = make(map[uint]*listenWindowsMediaWebViewParkingState)
)

var listenYouTubeCookieDomains = []string{"youtube.com", "google.com"}

type listenWindowsEmbeddedWebViewState struct {
	active             bool
	fullscreen         bool
	restoreToParking   bool
	playerHWND         w32.HWND
	playerWindow       *application.WebviewWindow
	hostHWND           w32.HWND
	hostWindow         *application.WebviewWindow
	originalParent     w32.HWND
	originalOwner      uintptr
	originalStyle      uint32
	originalEx         uint32
	originalEnabled    bool
	originalRect       w32.RECT
	embeddedFrame      listenWindowsEmbeddedFrameRect
	embeddedRadius     float64
	fullscreenFrame    listenWindowsEmbeddedFrameRect
	fullscreenRadius   float64
	hostController     *edge.ICoreWebView2Controller2
	hostRestoreColor   edge.COREWEBVIEW2_COLOR
	hostTransparent    bool
	interactiveOverlay bool
}

// listenWindowsMediaWebViewParkingState preserves the Wails-owned top-level
// window topology independently from the temporary inline-video topology.
// A parked player is a visible 1x1 child of the main window, but native
// fullscreen must always be able to recover this canonical top-level state.
type listenWindowsMediaWebViewParkingState struct {
	playerWindow *application.WebviewWindow
	playerHWND   w32.HWND
	hostWindow   *application.WebviewWindow
	hostHWND     w32.HWND

	topLevelParent  w32.HWND
	topLevelOwner   uintptr
	topLevelStyle   uint32
	topLevelExStyle uint32
	topLevelEnabled bool
	topLevelRect    w32.RECT

	parkingRequested bool
	parked           bool
}

type listenWindowsEmbeddedHostWebView struct {
	controller   *edge.ICoreWebView2Controller2
	restoreColor edge.COREWEBVIEW2_COLOR
	window       *application.WebviewWindow
	newlyCached  bool
}

type listenWindowsEmbeddedHostControllerCache struct {
	window     *application.WebviewWindow
	controller *edge.ICoreWebView2Controller2
}

type listenWindowsMediaVisibilityCache struct {
	window *application.WebviewWindow
}

func listenYouTubeMusicUserAgent() string {
	return youtubemusic.WindowsWebViewUserAgent
}

func configureListenYouTubeMusicNativeWindow(_ unsafe.Pointer, _ string) {}

func installListenNativeWindowFullscreenEscape(window *application.WebviewWindow) func() {
	if window == nil {
		return nil
	}
	// Wails' Windows fullscreen implementation is a borderless monitor-sized
	// native window. Unlike AppKit, Windows does not provide an implicit Escape
	// gesture, so keep the exit binding on the detached player window itself.
	window.RegisterKeyBinding("escape", func(_ application.Window) {
		if window.IsFullscreen() {
			window.UnFullscreen()
		}
	})
	return func() {}
}

func installRSSVideoPlayerNativeFullscreenEscape(window *application.WebviewWindow) func() {
	return installListenNativeWindowFullscreenEscape(window)
}

func showListenNativeAirPlayPicker(_ unsafe.Pointer, _ ListenAirPlayAnchor) bool {
	return false
}

// registerListenMediaWebViewParking snapshots the real Wails top-level window
// before any SetParent operation. Parking and inline presentation may both make
// the HWND a child of the main window, so that canonical state must live
// outside the singleton inline-presentation record.
func registerListenMediaWebViewParking(
	playerWindow *application.WebviewWindow,
	hostWindow *application.WebviewWindow,
) bool {
	if playerWindow == nil || hostWindow == nil || playerWindow == hostWindow {
		return false
	}

	var registered bool
	application.InvokeSync(func() {
		playerHWND := listenWindowsHWND(playerWindow.NativeWindow())
		hostHWND := listenWindowsHWND(hostWindow.NativeWindow())
		if playerHWND == 0 || hostHWND == 0 ||
			!w32.IsWindow(playerHWND) || !w32.IsWindow(hostHWND) ||
			playerHWND == hostHWND {
			return
		}

		listenWindowsEmbeddedWebViewLock.Lock()
		defer listenWindowsEmbeddedWebViewLock.Unlock()

		windowID := playerWindow.ID()
		if existing := listenWindowsMediaWebViewParking[windowID]; existing != nil {
			if existing.playerWindow == playerWindow && existing.playerHWND == playerHWND {
				if existing.parked && existing.hostHWND != hostHWND {
					if !listenWindowsRestoreMediaWebViewTopLevelLocked(existing, true) {
						return
					}
				}
				existing.hostWindow = hostWindow
				existing.hostHWND = hostHWND
				existing.parkingRequested = true
				if !listenWindowsApplyMediaWebViewParkingLocked(existing) {
					return
				}
				registered = true
				return
			}
			// Window IDs may be reused after native destruction. Never apply an
			// old HWND baseline to the new Wails window instance.
			delete(listenWindowsMediaWebViewParking, windowID)
		}

		style := uint32(w32.GetWindowLong(playerHWND, w32.GWL_STYLE))
		if style&uint32(w32.WS_CHILD) != 0 {
			// Registration after an untracked reparent cannot recover the real
			// top-level owner/style safely. Fail soft and retain the old path.
			return
		}
		rect := w32.GetWindowRect(playerHWND)
		if rect == nil {
			return
		}
		state := &listenWindowsMediaWebViewParkingState{
			playerWindow:     playerWindow,
			playerHWND:       playerHWND,
			hostWindow:       hostWindow,
			hostHWND:         hostHWND,
			topLevelParent:   0,
			topLevelOwner:    w32.GetWindowLongPtr(playerHWND, w32.GWLP_HWNDPARENT),
			topLevelStyle:    style,
			topLevelExStyle:  uint32(w32.GetWindowLong(playerHWND, w32.GWL_EXSTYLE)),
			topLevelEnabled:  w32.IsWindowEnabled(playerHWND),
			topLevelRect:     *rect,
			parkingRequested: true,
		}
		listenWindowsMediaWebViewParking[windowID] = state
		registered = listenWindowsApplyMediaWebViewParkingLocked(state)
		if !registered {
			// Parking can partially mutate style/parent before WebView2 reports
			// a controller error. Restore the captured Wails topology and do
			// not discard the only recovery baseline unless that rollback is
			// confirmed. A retained record lets hide/navigation retry safely.
			restored := listenWindowsRestoreMediaWebViewTopLevelLocked(state, true)
			if restored {
				delete(listenWindowsMediaWebViewParking, windowID)
			} else {
				state.parkingRequested = true
				zap.L().Warn(
					"media WebView parking registration rollback failed; retaining recovery state",
					zap.String("window", playerWindow.Name()),
				)
			}
		}
	})
	return registered
}

// parkListenMediaWebView keeps a media controller in the visible main-window
// hierarchy without exposing a standalone player window. It deliberately does
// not require the main WebView's CompositionController: enterprise/runtime
// fallback to a normal HWND controller must not disable background audio.
func parkListenMediaWebView(playerWindow *application.WebviewWindow) bool {
	if playerWindow == nil {
		return false
	}

	var parked bool
	application.InvokeSync(func() {
		listenWindowsEmbeddedWebViewLock.Lock()
		defer listenWindowsEmbeddedWebViewLock.Unlock()

		state := listenWindowsMediaWebViewParkingForWindowLocked(playerWindow)
		if state == nil {
			return
		}
		state.parkingRequested = true
		if listenWindowsEmbeddedWebView.active &&
			listenWindowsEmbeddedWebView.playerWindow == playerWindow &&
			listenWindowsEmbeddedWebView.playerHWND == state.playerHWND {
			listenWindowsRestoreEmbeddedWebViewLocked()
			parked = state.parked
			return
		}
		parked = listenWindowsApplyMediaWebViewParkingLocked(state)
	})
	return parked
}

// unparkListenMediaWebView restores the canonical top-level HWND while keeping
// it hidden and its WebView2 controller live. Native fullscreen calls Show only
// after WS_CHILD has been removed successfully.
func unparkListenMediaWebView(playerWindow *application.WebviewWindow) bool {
	if playerWindow == nil {
		return false
	}

	var unparked bool
	application.InvokeSync(func() {
		listenWindowsEmbeddedWebViewLock.Lock()
		defer listenWindowsEmbeddedWebViewLock.Unlock()

		playerHWND := listenWindowsHWND(playerWindow.NativeWindow())
		if playerHWND == 0 || !w32.IsWindow(playerHWND) {
			return
		}
		state := listenWindowsMediaWebViewParkingForWindowLocked(playerWindow)
		if state == nil {
			return
		}
		if listenWindowsEmbeddedWebView.active &&
			listenWindowsEmbeddedWebView.playerWindow == playerWindow &&
			listenWindowsEmbeddedWebView.playerHWND == state.playerHWND {
			state.parkingRequested = false
			unparked = listenWindowsDetachEmbeddedWebViewToTopLevelLocked(
				state.playerHWND,
				state,
			)
			if !unparked {
				state.parkingRequested = true
			}
			return
		}
		state.parkingRequested = false
		unparked = listenWindowsRestoreMediaWebViewTopLevelLocked(state, true)
		if !unparked {
			// Treat presentation as a transaction: keep the player recoverable
			// in its live 1x1 host if any HWND or Controller2 restore step fails.
			state.parkingRequested = true
			if !listenWindowsApplyMediaWebViewParkingLocked(state) {
				zap.L().Warn(
					"media WebView top-level restore and parking rollback both failed",
					zap.String("window", playerWindow.Name()),
				)
			}
		}
	})
	return unparked
}

// reassertListenMediaWebViewParking repairs Wails' first-navigation visibility
// nudge. NavigationCompleted may hide a logically Hidden window after it was
// already mounted as the main window's 1x1 child.
func reassertListenMediaWebViewParking(playerWindow *application.WebviewWindow) {
	if playerWindow == nil {
		return
	}

	application.InvokeSync(func() {
		listenWindowsEmbeddedWebViewLock.Lock()
		defer listenWindowsEmbeddedWebViewLock.Unlock()

		state := listenWindowsMediaWebViewParkingForWindowLocked(playerWindow)
		if state == nil {
			// The initial about:blank navigation can complete while the player
			// constructor is still registering its mandatory anchor.
			return
		}
		if listenWindowsEmbeddedWebView.active &&
			listenWindowsEmbeddedWebView.playerWindow == playerWindow &&
			listenWindowsEmbeddedWebView.playerHWND == state.playerHWND {
			if err := listenWindowsPutMediaControllerVisibility(playerWindow, state.playerHWND, true); err != nil {
				zap.L().Warn(
					"media WebView inline visibility reassertion failed",
					zap.String("window", playerWindow.Name()),
					zap.Error(err),
				)
			}
			return
		}
		if !state.parkingRequested {
			return
		}
		if !listenWindowsApplyMediaWebViewParkingLocked(state) {
			zap.L().Warn(
				"media WebView parking reassertion failed",
				zap.String("window", playerWindow.Name()),
			)
		}
	})
}

// releaseListenMediaWebViewParking restores Wails' native topology before its
// owner destroys the window, then forgets the HWND baseline.
func releaseListenMediaWebViewParking(playerWindow *application.WebviewWindow) {
	if playerWindow == nil {
		return
	}

	application.InvokeSync(func() {
		listenWindowsEmbeddedWebViewLock.Lock()
		defer listenWindowsEmbeddedWebViewLock.Unlock()

		state := listenWindowsMediaWebViewParkingForWindowLocked(playerWindow)
		if state == nil {
			return
		}
		if listenWindowsEmbeddedWebView.active &&
			listenWindowsEmbeddedWebView.playerWindow == playerWindow &&
			listenWindowsEmbeddedWebView.playerHWND == state.playerHWND {
			listenWindowsRestoreEmbeddedWebViewLocked()
		}
		state.parkingRequested = false
		_ = listenWindowsRestoreMediaWebViewTopLevelLocked(state, false)
		delete(listenWindowsMediaWebViewParking, playerWindow.ID())
	})
}

func listenWindowsMediaWebViewParkingForWindowLocked(
	playerWindow *application.WebviewWindow,
) *listenWindowsMediaWebViewParkingState {
	if playerWindow == nil {
		return nil
	}
	state := listenWindowsMediaWebViewParking[playerWindow.ID()]
	if state == nil {
		return nil
	}
	playerHWND := listenWindowsHWND(playerWindow.NativeWindow())
	if state.playerWindow != playerWindow || state.playerHWND == 0 ||
		state.playerHWND != playerHWND || !w32.IsWindow(state.playerHWND) {
		delete(listenWindowsMediaWebViewParking, playerWindow.ID())
		return nil
	}
	return state
}

func listenWindowsMediaWebViewParkingForHWNDLocked(
	playerHWND w32.HWND,
) *listenWindowsMediaWebViewParkingState {
	if playerHWND == 0 {
		return nil
	}
	for windowID, state := range listenWindowsMediaWebViewParking {
		if state == nil || state.playerWindow == nil || state.playerHWND == 0 ||
			listenWindowsHWND(state.playerWindow.NativeWindow()) != state.playerHWND ||
			!w32.IsWindow(state.playerHWND) {
			delete(listenWindowsMediaWebViewParking, windowID)
			continue
		}
		if state.playerHWND == playerHWND {
			return state
		}
	}
	return nil
}

func listenWindowsApplyMediaWebViewParkingLocked(
	state *listenWindowsMediaWebViewParkingState,
) bool {
	if state == nil || state.playerWindow == nil || state.hostWindow == nil ||
		state.playerHWND == 0 || state.hostHWND == 0 ||
		!w32.IsWindow(state.playerHWND) || !w32.IsWindow(state.hostHWND) ||
		listenWindowsHWND(state.playerWindow.NativeWindow()) != state.playerHWND ||
		listenWindowsHWND(state.hostWindow.NativeWindow()) != state.hostHWND {
		return false
	}

	state.parkingRequested = true
	state.parked = false
	playerHWND := state.playerHWND
	w32.ShowWindow(playerHWND, w32.SW_HIDE)
	listenWindowsApplyEmbeddedWindowStyle(
		playerHWND,
		state.topLevelStyle,
		state.topLevelExStyle,
		false,
	)
	if uint32(w32.GetWindowLong(playerHWND, w32.GWL_STYLE))&uint32(w32.WS_CHILD) == 0 {
		_ = listenWindowsRestoreMediaWebViewTopLevelLocked(state, true)
		return false
	}
	w32.EnableWindow(playerHWND, state.topLevelEnabled)
	listenWindowsSetParent(playerHWND, state.hostHWND)
	if listenWindowsGetParent(playerHWND) != state.hostHWND {
		_ = listenWindowsRestoreMediaWebViewTopLevelLocked(state, true)
		return false
	}
	parkingExStyle := uint32(w32.GetWindowLong(playerHWND, w32.GWL_EXSTYLE)) |
		uint32(w32.WS_EX_LAYERED)
	w32.SetWindowLong(playerHWND, w32.GWL_EXSTYLE, parkingExStyle)
	if uint32(w32.GetWindowLong(playerHWND, w32.GWL_EXSTYLE)) != parkingExStyle ||
		!w32.SetLayeredWindowAttributes(playerHWND, 0, 0, w32.LWA_ALPHA) {
		_ = listenWindowsRestoreMediaWebViewTopLevelLocked(state, true)
		return false
	}

	listenWindowsSetWindowRgn(playerHWND, 0, true)
	if !w32.SetWindowPos(
		playerHWND,
		w32.HWND_BOTTOM,
		0,
		0,
		1,
		1,
		listenWindowsEmbeddedWindowPositionFlags(),
	) {
		_ = listenWindowsRestoreMediaWebViewTopLevelLocked(state, true)
		return false
	}
	if err := listenWindowsPutMediaControllerVisibility(state.playerWindow, playerHWND, true); err != nil {
		_ = listenWindowsRestoreMediaWebViewTopLevelLocked(state, true)
		return false
	}
	w32.ShowWindow(playerHWND, w32.SW_SHOWNA)
	w32.InvalidateRect(state.hostHWND, nil, false)
	state.parked = true
	return true
}

func listenWindowsRestoreMediaWebViewTopLevelLocked(
	state *listenWindowsMediaWebViewParkingState,
	controllerVisible bool,
) bool {
	if state == nil || state.playerWindow == nil || state.playerHWND == 0 ||
		!w32.IsWindow(state.playerHWND) ||
		listenWindowsHWND(state.playerWindow.NativeWindow()) != state.playerHWND {
		return false
	}

	playerHWND := state.playerHWND
	state.parked = false
	w32.ShowWindow(playerHWND, w32.SW_HIDE)
	w32.EnableWindow(playerHWND, state.topLevelEnabled)
	listenWindowsSetWindowRgn(playerHWND, 0, true)
	// SetParent(NULL) temporarily reparents a WS_CHILD window to the desktop;
	// GetParent may therefore report the desktop HWND until WS_CHILD is
	// cleared. Validate the final top-level style and owner below instead of
	// rejecting that documented intermediate topology.
	listenWindowsSetParent(playerHWND, state.topLevelParent)
	w32.SetWindowLong(playerHWND, w32.GWL_STYLE, state.topLevelStyle)
	w32.SetWindowLong(playerHWND, w32.GWL_EXSTYLE, state.topLevelExStyle)
	if uint32(w32.GetWindowLong(playerHWND, w32.GWL_STYLE)) != state.topLevelStyle ||
		uint32(w32.GetWindowLong(playerHWND, w32.GWL_EXSTYLE)) != state.topLevelExStyle {
		return false
	}
	if state.topLevelExStyle&uint32(w32.WS_EX_LAYERED) != 0 &&
		!w32.SetLayeredWindowAttributes(playerHWND, 0, 255, w32.LWA_ALPHA) {
		return false
	}
	w32.SetWindowLongPtr(playerHWND, w32.GWLP_HWNDPARENT, state.topLevelOwner)
	if w32.GetWindowLongPtr(playerHWND, w32.GWLP_HWNDPARENT) != state.topLevelOwner {
		return false
	}
	if listenWindowsGetAncestorParent(playerHWND) != w32.GetDesktopWindow() {
		return false
	}

	width := int(state.topLevelRect.Right - state.topLevelRect.Left)
	height := int(state.topLevelRect.Bottom - state.topLevelRect.Top)
	if !w32.SetWindowPos(
		playerHWND,
		w32.HWND_TOP,
		int(state.topLevelRect.Left),
		int(state.topLevelRect.Top),
		max(1, width),
		max(1, height),
		uint(w32.SWP_NOACTIVATE|w32.SWP_NOZORDER|w32.SWP_FRAMECHANGED|w32.SWP_HIDEWINDOW),
	) {
		return false
	}
	state.parked = false
	if controllerVisible {
		if err := listenWindowsPutMediaControllerVisibility(state.playerWindow, playerHWND, true); err != nil {
			return false
		}
	} else {
		_ = listenWindowsPutMediaControllerVisibility(state.playerWindow, playerHWND, false)
	}
	style := uint32(w32.GetWindowLong(playerHWND, w32.GWL_STYLE))
	return style&uint32(w32.WS_CHILD) == 0
}

// The main React WebView uses Wails' topmost DirectComposition target. Mount
// the player as a child HWND below that visual tree: transparent pixels reveal
// video while every painted React pixel, portal and control remains above it.
func showListenNativeEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	return listenWindowsShowEmbeddedWebViewWindow(playerWindow, hostWindow, rect, true)
}

func listenWindowsShowEmbeddedWebViewWindow(
	playerWindow *application.WebviewWindow,
	hostWindow *application.WebviewWindow,
	rect ListenEmbeddedVideoRect,
	transparentHost bool,
) bool {
	if playerWindow == nil || hostWindow == nil {
		return false
	}
	playerHWND := listenWindowsHWND(playerWindow.NativeWindow())
	hostHWND := listenWindowsHWND(hostWindow.NativeWindow())
	if playerHWND == 0 || hostHWND == 0 || playerHWND == hostHWND {
		return false
	}

	var shown bool
	application.InvokeSync(func() {
		playerWebView := listenWindowsWebViewForWindow(playerWindow)
		if playerWebView == nil || playerWebView.Controller() == nil {
			return
		}
		var hostWebView *listenWindowsEmbeddedHostWebView
		if transparentHost {
			resolvedHostWebView, ok := listenWindowsEmbeddedHostWebViewForWindow(hostWindow)
			if !ok {
				return
			}
			hostWebView = &resolvedHostWebView
		}
		shown = listenWindowsShowEmbeddedWebView(playerWindow, playerHWND, hostHWND, rect, hostWebView)
		if !shown && hostWebView != nil && hostWebView.newlyCached {
			listenWindowsReleaseEmbeddedHostController(hostWindow)
		}
	})
	return shown
}

func showRSSNativeEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	rect.Interactive = false
	return showListenNativeEmbeddedWebViewWindow(playerWindow, hostWindow, rect)
}

func showRSSNativeInteractiveEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if playerWindow == nil || hostWindow == nil {
		return false
	}
	rect.Interactive = true
	// Interactive site pages use the same composition aperture but keep their
	// child HWND enabled so the site's own controls receive pointer input.
	return listenWindowsShowEmbeddedWebViewWindow(playerWindow, hostWindow, rect, true)
}

func showListenNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer, hostNativeWindow unsafe.Pointer, rect ListenEmbeddedVideoRect) bool {
	playerHWND := listenWindowsHWND(playerNativeWindow)
	hostHWND := listenWindowsHWND(hostNativeWindow)
	if playerHWND == 0 || hostHWND == 0 || playerHWND == hostHWND {
		return false
	}

	var shown bool
	application.InvokeSync(func() {
		// A bare HWND cannot recover the Wails-owned WebView2 controller. Keep
		// this platform-parity helper fail-closed; production uses the
		// window-aware entry points above.
		shown = listenWindowsShowEmbeddedWebView(nil, playerHWND, hostHWND, rect, nil)
	})
	return shown
}

func showRSSNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer, hostNativeWindow unsafe.Pointer, rect ListenEmbeddedVideoRect) bool {
	return showListenNativeEmbeddedWebView(playerNativeWindow, hostNativeWindow, rect)
}

func hideListenNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer) bool {
	playerHWND := listenWindowsHWND(playerNativeWindow)

	var hidden bool
	application.InvokeSync(func() {
		hidden = listenWindowsHideEmbeddedWebView(playerHWND)
	})
	return hidden
}

func detachListenNativeEmbeddedWebViewForFullscreen(playerNativeWindow unsafe.Pointer) bool {
	playerHWND := listenWindowsHWND(playerNativeWindow)
	if playerHWND == 0 || !w32.IsWindow(playerHWND) {
		return false
	}
	var detached bool
	application.InvokeSync(func() {
		listenWindowsEmbeddedWebViewLock.Lock()
		defer listenWindowsEmbeddedWebViewLock.Unlock()

		if parkingState := listenWindowsMediaWebViewParkingForHWNDLocked(playerHWND); parkingState != nil {
			parkingState.parkingRequested = false
			detached = listenWindowsDetachEmbeddedWebViewToTopLevelLocked(
				playerHWND,
				parkingState,
			)
			return
		}
		// Unregistered transient players retain the legacy inline restore path.
		// Wails fullscreen changes top-level style and monitor bounds and cannot
		// escape clipping while the player is still a WS_CHILD of React.
		style := uint32(w32.GetWindowLong(playerHWND, w32.GWL_STYLE))
		detached = style&uint32(w32.WS_CHILD) == 0
	})
	return detached
}

func listenWindowsDetachEmbeddedWebViewToTopLevelLocked(
	playerHWND w32.HWND,
	parkingState *listenWindowsMediaWebViewParkingState,
) bool {
	state := listenWindowsEmbeddedWebView
	if !state.active ||
		state.playerHWND != playerHWND ||
		parkingState == nil ||
		parkingState.playerHWND != playerHWND {
		return false
	}
	detached := listenWindowsRestoreMediaWebViewTopLevelLocked(parkingState, true)
	if !detached {
		parkingState.parkingRequested = true
		_ = listenWindowsApplyMediaWebViewParkingLocked(parkingState)
	}
	listenWindowsRestoreEmbeddedHostBackground(state)
	listenWindowsReleaseEmbeddedHostController(state.hostWindow)
	if state.hostHWND != 0 && w32.IsWindow(state.hostHWND) {
		w32.InvalidateRect(state.hostHWND, nil, false)
	}
	listenWindowsEmbeddedWebView = listenWindowsEmbeddedWebViewState{}
	return detached
}

func listenNativeEmbeddedVideoFullscreenOwnsPresentation(nativeWindow unsafe.Pointer) (bool, bool) {
	playerHWND := listenWindowsHWND(nativeWindow)
	if playerHWND == 0 {
		return false, false
	}
	listenWindowsEmbeddedWebViewLock.Lock()
	defer listenWindowsEmbeddedWebViewLock.Unlock()
	state := listenWindowsEmbeddedWebView
	if !state.active || state.playerHWND != playerHWND {
		return false, false
	}
	return state.fullscreen, true
}

func listenEmbeddedVideoUsesNativeWindowFullscreen() bool {
	return true
}

// Native fullscreen detaches the player from the React host. Inline geometry
// must remain suspended until the native window exits and is re-embedded.
func listenEmbeddedVideoFullscreenAllowsHostGeometry() bool {
	return false
}

func loadListenYouTubeMusicURL(window *application.WebviewWindow, targetURL string, cookies []appcookies.Record) {
	loadListenWindowsExternalVideoURL(window, targetURL, func() {
		prepareListenWindowsWebView(window, targetURL, cookies)
	})
}

func loadRSSVideoPlayerURL(window *application.WebviewWindow, targetURL string, cookies []appcookies.Record) {
	// Match the YouTube player lifecycle: validate the requested media page,
	// prepare the hidden Wails window, then navigate through WebviewWindow.SetURL.
	// The low-level policy below registers only the two callbacks needed to
	// cancel top-level escapes and consume every popup. It intentionally avoids
	// importing the legacy pkg/webview2 bindings, whose package initialization
	// registers hundreds of unrelated callbacks.
	expectedAdapter, expectedVideoID, validTarget := rssBilibiliPlaybackIdentityFromURL(targetURL)
	if window == nil || targetURL == "" || targetURL == rssBilibiliPlayerBlankURL ||
		!validTarget ||
		!rssBilibiliAllowsTopLevelNavigationForPlayback(targetURL, expectedAdapter, expectedVideoID) {
		return
	}
	loadListenWindowsExternalVideoURL(window, targetURL, func() {
		application.InvokeSync(func() {
			if webview := listenWindowsWebViewForWindow(window); webview != nil {
				configureListenWindowsWebView(window, webview)
			}
		})
		// CookieManager.AddOrUpdateCookie completes synchronously before SetURL,
		// so the first canonical page request sees the App Session snapshot.
		prepareConnectorAppSessionNativeWindow(window, targetURL, "bilibili", cookies, []string{"bilibili.com"})
		installRSSBilibiliWindowsReferer(window)
	})
}

func loadRSSSitePlayerURL(
	window *application.WebviewWindow,
	targetURL string,
	siteKey string,
	cookies []appcookies.Record,
	allowedDomains []string,
	registrableSite string,
) {
	if window == nil || targetURL == "" || targetURL == rssSitePlayerBlankURL {
		return
	}
	policy, allowed := webViewRemoteNavigationPolicyForRSSSite(targetURL, allowedDomains, registrableSite)
	if !allowed || !installListenWindowsRemoteNavigationPolicy(window, policy) {
		return
	}
	application.InvokeSync(func() {
		if webview := listenWindowsWebViewForWindow(window); webview != nil {
			installListenWindowsEmbeddedFullscreen(window, webview)
		}
	})
	// The shared CookieManager mutates synchronously before SetURL. Both cookie
	// scope and site identity were derived from the validated target URL.
	prepareConnectorAppSessionNativeWindow(window, targetURL, siteKey, cookies, allowedDomains)
	window.SetURL(targetURL)
}

// loadListenWindowsExternalVideoURL is the common Windows load boundary for
// YouTube and RSS-hosted external video. The allowlist and popup handler must
// be live before cookies are prepared and before Wails starts navigation.
func loadListenWindowsExternalVideoURL(
	window *application.WebviewWindow,
	targetURL string,
	prepare func(),
) {
	if window == nil || targetURL == "" {
		return
	}
	policy, allowed := webViewRemoteNavigationPolicyForPlayer(window.Name(), targetURL)
	if !allowed {
		return
	}
	if !installListenWindowsRemoteNavigationPolicy(window, policy) {
		return
	}
	if prepare != nil {
		prepare()
	}
	window.SetURL(targetURL)
}

func installRSSBilibiliWindowsReferer(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	windowID := window.ID()
	if !listenWindowsRSSBilibiliRefererGuard.Claim(windowID) {
		return
	}
	installed := false
	application.InvokeSync(func() {
		webview := listenWindowsWebViewForWindow(window)
		if webview == nil || webview.Controller() == nil {
			return
		}
		installed = webview.WrapWebResourceRequested(func(request *edge.ICoreWebView2WebResourceRequest) {
			if request == nil {
				return
			}
			rawURL, err := request.GetUri()
			if err != nil {
				return
			}
			parsed, err := url.Parse(strings.TrimSpace(rawURL))
			if err != nil || !sitepolicy.HostMatchesDomain(parsed.Hostname(), "bilibili.com") {
				return
			}
			headers, err := request.GetHeaders()
			if err != nil || headers == nil {
				return
			}
			defer headers.Release()
			_ = headers.SetHeader("Referer", "https://www.bilibili.com/")
		})
	})
	if !installed {
		listenWindowsRSSBilibiliRefererGuard.Release(windowID)
	}
}

func releaseRSSVideoPlayerWindowFeatures(window *application.WebviewWindow) {
	if window != nil {
		listenWindowsMediaVisibilityWindows.Delete(window.ID())
		listenWindowsRSSBilibiliRefererGuard.Release(window.ID())
		releaseListenWindowsRemoteNavigationPolicy(window)
	}
}

func releaseRSSSitePlayerWindowFeatures(window *application.WebviewWindow) {
	if window != nil {
		listenWindowsMediaVisibilityWindows.Delete(window.ID())
		releaseListenWindowsRemoteNavigationPolicy(window)
	}
}

func readListenWindowsYouTubeCookies(
	ctx context.Context,
	window *application.WebviewWindow,
) ([]appcookies.Record, error) {
	records, err := readConnectorWindowsWebViewCookies(ctx, window, listenYouTubeCookieDomains)
	if err != nil {
		return nil, err
	}
	return youtubecookies.Runtime(records, time.Now()), nil
}

func prepareListenWindowsWebView(
	window *application.WebviewWindow,
	targetURL string,
	cookies []appcookies.Record,
) {
	application.InvokeSync(func() {
		if webview := listenWindowsWebViewForWindow(window); webview != nil {
			configureListenWindowsWebView(window, webview)
			installListenWindowsWebResourceHeaders(window, webview)
		}
	})
	readContext, cancelRead := context.WithTimeout(context.Background(), 6*time.Second)
	currentCookies, readErr := readListenWindowsYouTubeCookies(readContext, window)
	cancelRead()
	now := time.Now()
	restoreCookies := planListenPlaybackCookieRestore(
		cookies,
		currentCookies,
		now,
		readErr == nil,
	)
	application.InvokeSync(func() {
		webview := listenWindowsWebViewForWindow(window)
		if webview == nil {
			return
		}
		if len(restoreCookies) == 0 {
			return
		}
		manager, err := webview.GetCookieManager()
		if err != nil || manager == nil {
			return
		}
		defer manager.Release()

		for _, record := range restoreCookies {
			addListenWindowsWebViewCookie(manager, record)
		}
	})
}

func configureListenWindowsWebView(window *application.WebviewWindow, webview *listenWindowsWebViewBridge) {
	if window == nil || webview == nil {
		return
	}
	if _, loaded := listenWindowsWebViewConfiguredWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	settings, err := webview.GetSettings()
	if err != nil || settings == nil {
		listenWindowsWebViewConfiguredWindowIDs.Delete(window.ID())
		return
	}
	defer settings.Release()
	installListenWindowsEmbeddedFullscreen(window, webview)

	if userAgent := listenYouTubeMusicUserAgent(); userAgent != "" {
		if err := settings.PutUserAgent(userAgent); err != nil {
			listenWindowsWebViewConfiguredWindowIDs.Delete(window.ID())
		}
	}
}

func installListenWindowsEmbeddedFullscreen(window *application.WebviewWindow, webview *listenWindowsWebViewBridge) {
	if window == nil || webview == nil {
		return
	}
	if _, loaded := listenWindowsEmbeddedFullscreenWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	playerHWND := listenWindowsHWND(window.NativeWindow())
	if !webview.WrapContainsFullScreenElementChanged(func(sender *edge.ICoreWebView2) bool {
		if sender == nil {
			return false
		}
		fullscreen, err := listenWindowsGetContainsFullScreenElement(sender)
		embeddedHandled := false
		if err == nil {
			if fullscreen {
				embeddedHandled = listenWindowsBeginEmbeddedFullscreen(playerHWND)
			} else {
				embeddedHandled = listenWindowsEndEmbeddedFullscreen(playerHWND)
			}
		}
		return embeddedHandled
	}) {
		listenWindowsEmbeddedFullscreenWindowIDs.Delete(window.ID())
	}
}

func installListenWindowsWebResourceHeaders(window *application.WebviewWindow, webview *listenWindowsWebViewBridge) {
	if window == nil || webview == nil {
		return
	}
	if _, loaded := listenWindowsWebResourceHeaderWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}

	if !webview.WrapWebResourceRequested(func(request *edge.ICoreWebView2WebResourceRequest) {
		applyListenWindowsWebResourceHeaders(request)
	}) {
		listenWindowsWebResourceHeaderWindowIDs.Delete(window.ID())
	}
}

func applyListenWindowsWebResourceHeaders(request *edge.ICoreWebView2WebResourceRequest) {
	if request == nil {
		return
	}
	rawURL, err := request.GetUri()
	if err != nil {
		return
	}
	headers, err := request.GetHeaders()
	if err != nil || headers == nil {
		return
	}
	defer headers.Release()

	if referer := listenWindowsNavigationRefererForURL(rawURL); referer != "" {
		_ = headers.SetHeader("Referer", referer)
	}
	if listenWindowsUsesYouTubeUserAgent(rawURL) {
		_ = headers.SetHeader("User-Agent", listenYouTubeMusicUserAgent())
	}
}

func listenWindowsNavigationRefererForURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch {
	case host == "music.youtube.com" || strings.HasSuffix(host, ".music.youtube.com"):
		return listenYouTubeMusicOrigin + "/"
	case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com"):
		return listenYouTubeClientOrigin()
	default:
		return ""
	}
}

func listenWindowsUsesYouTubeUserAgent(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "youtube.com" ||
		strings.HasSuffix(host, ".youtube.com") ||
		host == "youtube-nocookie.com" ||
		strings.HasSuffix(host, ".youtube-nocookie.com") ||
		host == "googlevideo.com" ||
		strings.HasSuffix(host, ".googlevideo.com") ||
		host == "ytimg.com" ||
		strings.HasSuffix(host, ".ytimg.com") ||
		host == "ggpht.com" ||
		strings.HasSuffix(host, ".ggpht.com")
}

func addListenWindowsWebViewCookie(
	manager *edge.ICoreWebView2CookieManager,
	record appcookies.Record,
) bool {
	if manager == nil {
		return false
	}

	name := strings.TrimSpace(record.Name)
	domain := strings.TrimSpace(record.Domain)
	path := strings.TrimSpace(record.Path)
	if name == "" || record.Value == "" || domain == "" {
		return false
	}
	if path == "" {
		path = "/"
	}

	if addListenWindowsWebViewCookieWithDomain(manager, record, name, domain, path) {
		return true
	}
	if strings.HasPrefix(domain, ".") {
		return addListenWindowsWebViewCookieWithDomain(
			manager,
			record,
			name,
			strings.TrimPrefix(domain, "."),
			path,
		)
	}
	return false
}

func addListenWindowsWebViewCookieWithDomain(
	manager *edge.ICoreWebView2CookieManager,
	record appcookies.Record,
	name string,
	domain string,
	path string,
) bool {
	cookie, err := manager.CreateCookie(name, record.Value, domain, path)
	if err != nil || cookie == nil {
		return false
	}
	defer cookie.Release()

	_ = cookie.PutIsSecure(record.Secure)
	_ = cookie.PutIsHttpOnly(record.HttpOnly)
	if record.Expires > 0 {
		_ = listenWindowsPutCookieExpires(cookie, float64(record.Expires))
	}
	if sameSite, ok := listenWindowsWebViewSameSite(record.SameSite); ok {
		_ = cookie.PutSameSite(sameSite)
	}
	return manager.AddOrUpdateCookie(cookie) == nil
}

func listenWindowsWebViewSameSite(value string) (int32, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none", "no_restriction":
		return 0, true
	case "lax":
		return 1, true
	case "strict":
		return 2, true
	default:
		return 0, false
	}
}

func listenWindowsHWND(nativeWindow unsafe.Pointer) w32.HWND {
	if nativeWindow == nil {
		return 0
	}
	return w32.HWND(uintptr(nativeWindow))
}

func listenWindowsPutMediaControllerVisibility(
	window *application.WebviewWindow,
	expectedHWND w32.HWND,
	visible bool,
) error {
	if window == nil || expectedHWND == 0 || !w32.IsWindow(expectedHWND) ||
		listenWindowsHWND(window.NativeWindow()) != expectedHWND {
		return errors.New("media WebView2 window is unavailable")
	}
	webview := listenWindowsWebViewForWindow(window)
	if webview == nil || webview.Controller() == nil {
		return errors.New("media WebView2 controller is unavailable")
	}
	if err := webview.Controller().PutIsVisible(visible); err != nil {
		return err
	}
	if visible {
		if err := webview.Controller().NotifyParentWindowPositionChanged(); err != nil {
			return fmt.Errorf("notify media WebView2 parent position: %w", err)
		}
	}
	return nil
}

// A player can be embedded before its first remote navigation completes. In
// that race Wails' first-paint callback would hide the controller again after
// the HWND has already been mounted. Install one post-Wails refresh per
// player window and only reassert visibility while that exact window owns the
// embedded surface.
func listenWindowsActivateEmbeddedMediaController(
	window *application.WebviewWindow,
	expectedHWND w32.HWND,
) error {
	if window == nil {
		return errors.New("media WebView2 window is unavailable")
	}
	windowID := window.ID()
	if cached, ok := listenWindowsMediaVisibilityWindows.Load(windowID); ok {
		entry, valid := cached.(listenWindowsMediaVisibilityCache)
		if !valid || entry.window != window {
			listenWindowsMediaVisibilityWindows.Delete(windowID)
		} else {
			return listenWindowsPutMediaControllerVisibility(window, expectedHWND, true)
		}
	}
	webview := listenWindowsWebViewForWindow(window)
	if webview == nil || webview.Controller() == nil {
		return errors.New("media WebView2 controller is unavailable")
	}
	if !webview.WrapNavigationCompleted(func() {
		listenWindowsEmbeddedWebViewLock.Lock()
		state := listenWindowsEmbeddedWebView
		listenWindowsEmbeddedWebViewLock.Unlock()
		if !state.active || state.playerWindow != window || state.playerHWND != expectedHWND {
			return
		}
		_ = listenWindowsPutMediaControllerVisibility(window, expectedHWND, true)
	}) {
		return errors.New("media WebView2 navigation visibility hook is unavailable")
	}
	listenWindowsMediaVisibilityWindows.Store(windowID, listenWindowsMediaVisibilityCache{window: window})
	return listenWindowsPutMediaControllerVisibility(window, expectedHWND, true)
}

func listenWindowsShowEmbeddedWebView(
	playerWindow *application.WebviewWindow,
	playerHWND w32.HWND,
	hostHWND w32.HWND,
	rect ListenEmbeddedVideoRect,
	hostWebView *listenWindowsEmbeddedHostWebView,
) bool {
	if playerWindow == nil || listenWindowsHWND(playerWindow.NativeWindow()) != playerHWND ||
		!w32.IsWindow(playerHWND) || !w32.IsWindow(hostHWND) {
		return false
	}

	frame := listenWindowsEmbeddedFrame(hostHWND, rect)
	if frame.Width < 1 || frame.Height < 1 {
		return false
	}

	listenWindowsEmbeddedWebViewLock.Lock()
	defer listenWindowsEmbeddedWebViewLock.Unlock()

	interactiveOverlay := rect.Interactive
	parkingState := listenWindowsMediaWebViewParkingForWindowLocked(playerWindow)
	hostChanged := listenWindowsEmbeddedWebView.hostHWND != hostHWND ||
		(listenWindowsEmbeddedWebView.active && listenWindowsEmbeddedWebView.interactiveOverlay != interactiveOverlay)
	if hostWebView != nil && listenWindowsEmbeddedWebView.hostWindow != hostWebView.window {
		hostChanged = true
	}
	if listenWindowsEmbeddedWebView.active &&
		(listenWindowsEmbeddedWebView.playerHWND != playerHWND ||
			listenWindowsEmbeddedWebView.playerWindow != playerWindow || hostChanged) {
		if hostWebView != nil &&
			listenWindowsEmbeddedWebView.hostWindow == hostWebView.window &&
			listenWindowsEmbeddedWebView.hostController == hostWebView.controller {
			// The incoming view borrowed this Controller2 from the same cache entry.
			// Preserve that owning reference while restoring the old player so the
			// new state cannot reuse a pointer that restore just released.
			listenWindowsRestoreEmbeddedWebViewLockedPreservingHost(hostWebView.window, hostWebView.controller)
		} else {
			listenWindowsRestoreEmbeddedWebViewLocked()
		}
	}

	if !listenWindowsEmbeddedWebView.active {
		listenWindowsEmbeddedWebView = listenWindowsEmbeddedWebViewState{
			active:             true,
			restoreToParking:   parkingState != nil,
			playerHWND:         playerHWND,
			playerWindow:       playerWindow,
			hostHWND:           hostHWND,
			interactiveOverlay: interactiveOverlay,
			originalParent:     listenWindowsGetParent(playerHWND),
			originalOwner:      w32.GetWindowLongPtr(playerHWND, w32.GWLP_HWNDPARENT),
			originalStyle:      uint32(w32.GetWindowLong(playerHWND, w32.GWL_STYLE)),
			originalEx:         uint32(w32.GetWindowLong(playerHWND, w32.GWL_EXSTYLE)),
			originalEnabled:    w32.IsWindowEnabled(playerHWND),
			originalRect:       *w32.GetWindowRect(playerHWND),
		}
	} else {
		listenWindowsEmbeddedWebView.playerWindow = playerWindow
		listenWindowsEmbeddedWebView.hostHWND = hostHWND
		listenWindowsEmbeddedWebView.interactiveOverlay = interactiveOverlay
	}
	if hostWebView != nil {
		listenWindowsEmbeddedWebView.hostController = hostWebView.controller
		listenWindowsEmbeddedWebView.hostRestoreColor = hostWebView.restoreColor
		listenWindowsEmbeddedWebView.hostWindow = hostWebView.window
	}
	listenWindowsEmbeddedWebView.embeddedFrame = frame
	listenWindowsEmbeddedWebView.embeddedRadius = rect.Radius * frame.Scale
	if listenWindowsEmbeddedWebView.hostController != nil {
		if err := listenWindowsEmbeddedWebView.hostController.PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{}); err != nil {
			listenWindowsRestoreEmbeddedWebViewLocked()
			return false
		}
		listenWindowsEmbeddedWebView.hostTransparent = true
	}

	listenWindowsApplyEmbeddedWindowStyle(playerHWND, listenWindowsEmbeddedWebView.originalStyle, listenWindowsEmbeddedWebView.originalEx, listenWindowsEmbeddedWebView.interactiveOverlay)
	// A child HWND wins Win32 hit-testing even when a topmost DComp visual is
	// painted above it. Disable video-only players so input reaches React; an
	// interactive RSS page remains enabled and owns the aperture's input.
	w32.EnableWindow(playerHWND, listenWindowsEmbeddedWebView.interactiveOverlay)
	listenWindowsSetParent(playerHWND, hostHWND)
	if listenWindowsGetParent(playerHWND) != hostHWND {
		listenWindowsRestoreEmbeddedWebViewLocked()
		return false
	}

	listenWindowsSetEmbeddedWindowRegion(playerHWND, frame.Width, frame.Height, rect.Radius*frame.Scale)
	insertAfter := w32.HWND_TOP
	if listenWindowsEmbeddedWebView.hostTransparent {
		insertAfter = w32.HWND_BOTTOM
	}
	if !w32.SetWindowPos(
		playerHWND,
		insertAfter,
		frame.X,
		frame.Y,
		frame.Width,
		frame.Height,
		listenWindowsEmbeddedWindowPositionFlags(),
	) {
		listenWindowsRestoreEmbeddedWebViewLocked()
		return false
	}
	if err := listenWindowsActivateEmbeddedMediaController(playerWindow, playerHWND); err != nil {
		listenWindowsRestoreEmbeddedWebViewLocked()
		return false
	}
	w32.ShowWindow(playerHWND, w32.SW_SHOWNA)
	w32.InvalidateRect(hostHWND, nil, false)
	w32.InvalidateRect(playerHWND, nil, false)
	if parkingState != nil {
		parkingState.parked = false
	}
	return true
}

// Keep element fullscreen inside the main window's native composition. The
// player remains below the host's topmost DirectComposition visual so React
// fullscreen controls can continue painting and receiving input.
func listenWindowsBeginEmbeddedFullscreen(playerHWND w32.HWND) bool {
	listenWindowsEmbeddedWebViewLock.Lock()
	defer listenWindowsEmbeddedWebViewLock.Unlock()

	state := &listenWindowsEmbeddedWebView
	if !state.active || state.playerHWND != playerHWND || state.fullscreen ||
		!w32.IsWindow(playerHWND) {
		return false
	}
	if !w32.IsWindow(state.hostHWND) {
		return false
	}
	state.fullscreenFrame = state.embeddedFrame
	state.fullscreenRadius = state.embeddedRadius
	client := w32.GetClientRect(state.hostHWND)
	width := max(1, int(client.Right-client.Left))
	height := max(1, int(client.Bottom-client.Top))
	listenWindowsSetWindowRgn(playerHWND, 0, true)
	listenWindowsApplyEmbeddedWindowStyle(playerHWND, state.originalStyle, state.originalEx, state.interactiveOverlay)
	listenWindowsSetParent(playerHWND, state.hostHWND)
	if listenWindowsGetParent(playerHWND) != state.hostHWND {
		return false
	}
	insertAfter := w32.HWND_TOP
	if state.hostTransparent {
		insertAfter = w32.HWND_BOTTOM
	}
	if !w32.SetWindowPos(
		playerHWND,
		insertAfter,
		0,
		0,
		width,
		height,
		listenWindowsEmbeddedWindowPositionFlags(),
	) {
		listenWindowsSetEmbeddedWindowRegion(
			playerHWND,
			state.embeddedFrame.Width,
			state.embeddedFrame.Height,
			state.embeddedRadius,
		)
		return false
	}
	state.fullscreen = true
	w32.InvalidateRect(state.hostHWND, nil, false)
	return true
}

func listenWindowsEndEmbeddedFullscreen(playerHWND w32.HWND) bool {
	listenWindowsEmbeddedWebViewLock.Lock()
	defer listenWindowsEmbeddedWebViewLock.Unlock()

	state := &listenWindowsEmbeddedWebView
	if !state.active || state.playerHWND != playerHWND || !state.fullscreen ||
		!w32.IsWindow(playerHWND) || !w32.IsWindow(state.hostHWND) {
		return false
	}
	frame := state.fullscreenFrame
	radius := state.fullscreenRadius
	listenWindowsApplyEmbeddedWindowStyle(playerHWND, state.originalStyle, state.originalEx, state.interactiveOverlay)
	listenWindowsSetParent(playerHWND, state.hostHWND)
	if listenWindowsGetParent(playerHWND) != state.hostHWND {
		return false
	}
	listenWindowsSetEmbeddedWindowRegion(playerHWND, frame.Width, frame.Height, radius)
	insertAfter := w32.HWND_TOP
	if state.hostTransparent {
		insertAfter = w32.HWND_BOTTOM
	}
	if !w32.SetWindowPos(
		playerHWND,
		insertAfter,
		frame.X,
		frame.Y,
		max(1, frame.Width),
		max(1, frame.Height),
		listenWindowsEmbeddedWindowPositionFlags(),
	) {
		// The fullscreen geometry remains authoritative until a later retry.
		// Undo the premature inline clipping without lying about state.
		listenWindowsSetWindowRgn(playerHWND, 0, true)
		return false
	}
	state.embeddedFrame = frame
	state.embeddedRadius = radius
	state.fullscreen = false
	w32.InvalidateRect(state.hostHWND, nil, false)
	return true
}

func listenWindowsHideEmbeddedWebView(playerHWND w32.HWND) bool {
	listenWindowsEmbeddedWebViewLock.Lock()
	defer listenWindowsEmbeddedWebViewLock.Unlock()

	if !listenWindowsEmbeddedWebView.active {
		parkingState := listenWindowsMediaWebViewParkingForHWNDLocked(playerHWND)
		if parkingState == nil {
			return false
		}
		if parkingState.parked {
			return true
		}
		if parkingState.parkingRequested {
			return listenWindowsApplyMediaWebViewParkingLocked(parkingState)
		}
		return false
	}
	if listenWindowsEmbeddedWebView.playerHWND != playerHWND &&
		(playerHWND != 0 || w32.IsWindow(listenWindowsEmbeddedWebView.playerHWND)) {
		return false
	}
	// NativeWindow returns nil once Wails marks a player window destroyed. In
	// that close/error path, a missing handle may only restore an orphaned state;
	// it must never tear down another live player's underlay.
	return listenWindowsRestoreEmbeddedWebViewLocked()
}

type listenWindowsEmbeddedFrameRect struct {
	X      int
	Y      int
	Width  int
	Height int
	Scale  float64
}

func listenWindowsEmbeddedFrame(hostHWND w32.HWND, rect ListenEmbeddedVideoRect) listenWindowsEmbeddedFrameRect {
	client := w32.GetClientRect(hostHWND)
	hostWidth := max(1, int(client.Right-client.Left))
	hostHeight := max(1, int(client.Bottom-client.Top))

	viewportWidth := rect.ViewportWidth
	viewportHeight := rect.ViewportHeight
	hasViewport := listenWindowsFinitePositive(viewportWidth) && listenWindowsFinitePositive(viewportHeight)
	if !hasViewport {
		viewportWidth = float64(hostWidth)
		viewportHeight = float64(hostHeight)
	}

	scaleX := float64(hostWidth) / math.Max(1, viewportWidth)
	scaleY := float64(hostHeight) / math.Max(1, viewportHeight)
	frameWidth := listenWindowsClampInt(int(math.Ceil(rect.Width*scaleX)), 1, hostWidth)
	frameHeight := listenWindowsClampInt(int(math.Ceil(rect.Height*scaleY)), 1, hostHeight)

	var x float64
	var y float64
	if hasViewport && listenWindowsFinite(rect.CenterX) && listenWindowsFinite(rect.CenterY) {
		centerX := float64(hostWidth)/2 + ((rect.CenterX - viewportWidth/2) * scaleX)
		centerY := float64(hostHeight)/2 + ((rect.CenterY - viewportHeight/2) * scaleY)
		centerX = listenWindowsClampFloat(centerX, float64(frameWidth)/2, float64(hostWidth)-float64(frameWidth)/2)
		centerY = listenWindowsClampFloat(centerY, float64(frameHeight)/2, float64(hostHeight)-float64(frameHeight)/2)
		x = centerX - float64(frameWidth)/2
		y = centerY - float64(frameHeight)/2
	} else {
		x = rect.X * scaleX
		y = rect.Y * scaleY
	}

	frameX := listenWindowsClampInt(int(math.Floor(x)), 0, max(0, hostWidth-frameWidth))
	frameY := listenWindowsClampInt(int(math.Floor(y)), 0, max(0, hostHeight-frameHeight))

	return listenWindowsEmbeddedFrameRect{
		X:      frameX,
		Y:      frameY,
		Width:  frameWidth,
		Height: frameHeight,
		Scale:  math.Min(scaleX, scaleY),
	}
}

func listenWindowsApplyEmbeddedWindowStyle(playerHWND w32.HWND, originalStyle uint32, originalEx uint32, interactiveOverlay bool) {
	style := originalStyle
	style &^= uint32(
		w32.WS_POPUP |
			w32.WS_CAPTION |
			w32.WS_THICKFRAME |
			w32.WS_MINIMIZEBOX |
			w32.WS_MAXIMIZEBOX |
			w32.WS_SYSMENU |
			w32.WS_BORDER |
			w32.WS_DLGFRAME,
	)
	style |= uint32(w32.WS_CHILD | w32.WS_VISIBLE | w32.WS_CLIPSIBLINGS | w32.WS_CLIPCHILDREN)
	w32.SetWindowLong(playerHWND, w32.GWL_STYLE, style)

	exStyle := originalEx
	exStyle &^= uint32(
		w32.WS_EX_APPWINDOW |
			w32.WS_EX_TOPMOST |
			w32.WS_EX_TOOLWINDOW |
			w32.WS_EX_WINDOWEDGE |
			w32.WS_EX_CLIENTEDGE |
			w32.WS_EX_DLGMODALFRAME,
	)
	exStyle |= uint32(w32.WS_EX_CONTROLPARENT)
	if interactiveOverlay {
		exStyle &^= uint32(w32.WS_EX_NOACTIVATE)
	} else {
		exStyle |= uint32(w32.WS_EX_NOACTIVATE)
	}
	w32.SetWindowLong(playerHWND, w32.GWL_EXSTYLE, exStyle)
	if exStyle&uint32(w32.WS_EX_LAYERED) != 0 {
		w32.SetLayeredWindowAttributes(playerHWND, 0, 255, w32.LWA_ALPHA)
	}
}

func listenWindowsEmbeddedWindowPositionFlags() uint {
	// Geometry may update on every scroll/ResizeObserver tick. Do not activate
	// the child programmatically; removing WS_EX_NOACTIVATE for interactive
	// overlays is sufficient for a real pointer click to focus the WebView.
	return uint(w32.SWP_NOACTIVATE | w32.SWP_SHOWWINDOW | w32.SWP_FRAMECHANGED)
}

func listenWindowsRestoreEmbeddedWebViewLocked() bool {
	return listenWindowsRestoreEmbeddedWebViewLockedPreservingHost(nil, nil)
}

func listenWindowsRestoreEmbeddedWebViewLockedPreservingHost(
	preserveWindow *application.WebviewWindow,
	preserveController *edge.ICoreWebView2Controller2,
) bool {
	state := listenWindowsEmbeddedWebView
	if !state.active {
		return false
	}

	playerHWND := state.playerHWND
	var parkingState *listenWindowsMediaWebViewParkingState
	if state.restoreToParking {
		parkingState = listenWindowsMediaWebViewParkingForWindowLocked(state.playerWindow)
	}
	if w32.IsWindow(playerHWND) {
		w32.ShowWindow(playerHWND, w32.SW_HIDE)
		if parkingState == nil {
			_ = listenWindowsPutMediaControllerVisibility(state.playerWindow, playerHWND, false)
			w32.EnableWindow(playerHWND, state.originalEnabled)
			listenWindowsSetWindowRgn(playerHWND, 0, true)
			w32.SetWindowLong(playerHWND, w32.GWL_STYLE, state.originalStyle)
			w32.SetWindowLong(playerHWND, w32.GWL_EXSTYLE, state.originalEx)
			listenWindowsSetParent(playerHWND, state.originalParent)
			w32.SetWindowLongPtr(playerHWND, w32.GWLP_HWNDPARENT, state.originalOwner)

			width := int(state.originalRect.Right - state.originalRect.Left)
			height := int(state.originalRect.Bottom - state.originalRect.Top)
			w32.SetWindowPos(
				playerHWND,
				w32.HWND_TOP,
				int(state.originalRect.Left),
				int(state.originalRect.Top),
				max(1, width),
				max(1, height),
				uint(w32.SWP_NOACTIVATE|w32.SWP_NOZORDER|w32.SWP_FRAMECHANGED|w32.SWP_HIDEWINDOW),
			)
		}
	}
	listenWindowsRestoreEmbeddedHostBackground(state)
	preserveHost := preserveWindow != nil && preserveController != nil &&
		state.hostWindow == preserveWindow && state.hostController == preserveController
	if !preserveHost {
		listenWindowsReleaseEmbeddedHostController(state.hostWindow)
	}
	if state.hostHWND != 0 && w32.IsWindow(state.hostHWND) {
		w32.InvalidateRect(state.hostHWND, nil, false)
	}
	listenWindowsEmbeddedWebView = listenWindowsEmbeddedWebViewState{}
	restored := true
	if parkingState != nil {
		restored = listenWindowsApplyMediaWebViewParkingLocked(parkingState)
	}
	return restored
}

func listenWindowsRestoreEmbeddedHostBackground(state listenWindowsEmbeddedWebViewState) {
	if !state.hostTransparent || !listenWindowsEmbeddedHostControllerIsLive(state) {
		return
	}
	restoreColor := state.hostRestoreColor
	if latest, ok := listenWindowsWebviewWindowDefaultBackground(state.hostWindow); ok {
		restoreColor = latest
	}
	_ = state.hostController.PutDefaultBackgroundColor(restoreColor)
}

func listenWindowsEmbeddedHostControllerIsLive(state listenWindowsEmbeddedWebViewState) bool {
	if state.hostController == nil || state.hostWindow == nil || state.hostHWND == 0 ||
		!w32.IsWindow(state.hostHWND) {
		return false
	}
	return listenWindowsHWND(state.hostWindow.NativeWindow()) == state.hostHWND
}

func listenWindowsGetParent(hwnd w32.HWND) w32.HWND {
	parent, _, _ := listenWindowsProcGetParent.Call(uintptr(hwnd))
	return w32.HWND(parent)
}

func listenWindowsGetAncestorParent(hwnd w32.HWND) w32.HWND {
	const gaParent = 1
	parent, _, _ := listenWindowsProcGetAncestor.Call(uintptr(hwnd), gaParent)
	return w32.HWND(parent)
}

func listenWindowsSetParent(hwnd w32.HWND, parent w32.HWND) w32.HWND {
	previous, _, _ := listenWindowsProcSetParent.Call(uintptr(hwnd), uintptr(parent))
	return w32.HWND(previous)
}

func listenWindowsSetWindowRgn(hwnd w32.HWND, region w32.HRGN, redraw bool) bool {
	result, _, _ := listenWindowsProcSetWindowRgn.Call(
		uintptr(hwnd),
		uintptr(region),
		uintptr(w32.BoolToBOOL(redraw)),
	)
	return result != 0
}

func listenWindowsSetEmbeddedWindowRegion(hwnd w32.HWND, width int, height int, radius float64) {
	if width < 1 || height < 1 || radius <= 0 || !listenWindowsFinite(radius) {
		listenWindowsSetWindowRgn(hwnd, 0, true)
		return
	}

	radius = listenWindowsClampFloat(radius, 0, float64(min(width, height))/2)
	diameter := max(1, int(math.Round(radius*2)))
	region, _, _ := listenWindowsProcCreateRoundRgn.Call(
		0,
		0,
		uintptr(width+1),
		uintptr(height+1),
		uintptr(diameter),
		uintptr(diameter),
	)
	if region == 0 {
		listenWindowsSetWindowRgn(hwnd, 0, true)
		return
	}
	if !listenWindowsSetWindowRgn(hwnd, w32.HRGN(region), true) {
		w32.DeleteObject(w32.HGDIOBJ(region))
	}
}

func listenWindowsFinitePositive(value float64) bool {
	return listenWindowsFinite(value) && value > 0
}

func listenWindowsFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func listenWindowsClampFloat(value float64, minimum float64, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func listenWindowsClampInt(value int, minimum int, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func (player *ListenYouTubeMusicPlayer) EqualizerAudioProcessID() uint32 {
	if player == nil {
		return 0
	}
	window := player.currentWindow()
	if window == nil {
		return 0
	}

	var processID uint32
	application.InvokeSync(func() {
		processID = listenWindowsWebViewBrowserProcessID(listenWindowsWebViewForWindow(window))
	})
	return processID
}

// listenWindowsWebViewBridge exposes only the stable COM interfaces owned by
// Wails' private WebView2 host. The Go Chromium implementation itself is an
// internal, version-specific type and must never cross this boundary.
type listenWindowsWebViewBridge struct {
	chromium   reflect.Value
	core       *edge.ICoreWebView2
	controller *edge.ICoreWebView2Controller
}

func listenWindowsWebViewForWindow(window *application.WebviewWindow) *listenWindowsWebViewBridge {
	if window == nil || window.NativeWindow() == nil {
		return nil
	}

	windowValue := reflect.ValueOf(window)
	if windowValue.Kind() != reflect.Pointer || windowValue.IsNil() {
		return nil
	}

	windowStruct := windowValue.Elem()
	implField := windowStruct.FieldByName("impl")
	if !implField.IsValid() || implField.Kind() != reflect.Interface || implField.IsNil() || !implField.CanAddr() {
		return nil
	}

	implValue := reflect.NewAt(implField.Type(), unsafe.Pointer(implField.UnsafeAddr())).Elem()
	if implValue.Kind() != reflect.Interface || implValue.IsNil() {
		return nil
	}

	concreteImpl := implValue.Elem()
	if concreteImpl.Kind() != reflect.Pointer || concreteImpl.IsNil() {
		return nil
	}

	implStruct := concreteImpl.Elem()
	if implStruct.Kind() != reflect.Struct {
		return nil
	}

	chromiumField := implStruct.FieldByName("chromium")
	if !chromiumField.IsValid() || chromiumField.Kind() != reflect.Pointer || chromiumField.IsNil() {
		return nil
	}
	chromiumStruct := chromiumField.Elem()
	if chromiumStruct.Kind() != reflect.Struct || !chromiumStruct.CanAddr() {
		return nil
	}

	// A controller is assigned partway through WebView2 creation. Wails marks
	// inited only after the controller, core and event registrations are ready.
	initedField := chromiumStruct.FieldByName("inited")
	if !initedField.IsValid() || initedField.Kind() != reflect.Uintptr || !initedField.CanAddr() ||
		atomic.LoadUintptr((*uintptr)(unsafe.Pointer(initedField.UnsafeAddr()))) == 0 {
		return nil
	}
	shuttingDownField := chromiumStruct.FieldByName("shuttingDown")
	if !shuttingDownField.IsValid() || shuttingDownField.Kind() != reflect.Bool || !shuttingDownField.CanAddr() ||
		reflect.NewAt(shuttingDownField.Type(), unsafe.Pointer(shuttingDownField.UnsafeAddr())).Elem().Bool() {
		return nil
	}

	corePointer := listenWindowsReflectedCOMPointer(chromiumStruct.FieldByName("webview"))
	controllerPointer := listenWindowsReflectedCOMPointer(chromiumStruct.FieldByName("controller"))
	if corePointer == nil || controllerPointer == nil {
		return nil
	}

	return &listenWindowsWebViewBridge{
		chromium:   chromiumStruct,
		core:       (*edge.ICoreWebView2)(corePointer),
		controller: (*edge.ICoreWebView2Controller)(controllerPointer),
	}
}

func listenWindowsReflectedCOMPointer(field reflect.Value) unsafe.Pointer {
	if !field.IsValid() || field.Kind() != reflect.Pointer || field.IsNil() || !field.CanAddr() {
		return nil
	}
	// NewAt makes the unexported pointer field readable without converting the
	// private Go object to any external concrete type.
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().UnsafePointer()
}

func (bridge *listenWindowsWebViewBridge) Core() *edge.ICoreWebView2 {
	if bridge == nil {
		return nil
	}
	return bridge.core
}

func (bridge *listenWindowsWebViewBridge) Controller() *edge.ICoreWebView2Controller {
	if bridge == nil {
		return nil
	}
	return bridge.controller
}

// Wails falls back to a windowed controller when DirectComposition setup is
// unavailable. An underlay must fail closed in that case: a transparent
// windowed WebView2 still occupies an HWND plane and would hide the player.
func (bridge *listenWindowsWebViewBridge) CompositionControllerReady() bool {
	if bridge == nil || !bridge.chromium.IsValid() {
		return false
	}
	enabledField := bridge.chromium.FieldByName("CompositionControllerEnabled")
	if !enabledField.IsValid() || enabledField.Kind() != reflect.Bool || !enabledField.CanAddr() ||
		!reflect.NewAt(enabledField.Type(), unsafe.Pointer(enabledField.UnsafeAddr())).Elem().Bool() {
		return false
	}
	return listenWindowsReflectedCOMPointer(bridge.chromium.FieldByName("compositionController")) != nil &&
		listenWindowsReflectedCOMPointer(bridge.chromium.FieldByName("compositionHost")) != nil
}

func (bridge *listenWindowsWebViewBridge) GetSettings() (*edge.ICoreWebViewSettings, error) {
	if bridge == nil || bridge.core == nil {
		return nil, errors.New("WebView2 core is not ready")
	}
	return bridge.core.GetSettings()
}

func (bridge *listenWindowsWebViewBridge) GetCookieManager() (*edge.ICoreWebView2CookieManager, error) {
	if bridge == nil || bridge.core == nil {
		return nil, errors.New("WebView2 core is not ready")
	}
	core2, err := bridge.core.QueryInterface2()
	if err != nil {
		return nil, err
	}
	defer core2.Release()
	return core2.GetCookieManager()
}

func (bridge *listenWindowsWebViewBridge) beginDocumentStartScriptRegistration(
	script string,
) (*listenWindowsDocumentStartScriptRegistration, error) {
	core := listenWindowsWebViewCore(bridge)
	if core == nil {
		return nil, errors.New("WebView2 core is not ready")
	}
	return core.beginDocumentStartScriptRegistration(script)
}

// wrapCallback replaces an internal Chromium callback with a function of its
// exact private type. The previous callback value is copied before replacement
// so wrapper chaining cannot recurse through the reflected field.
func (bridge *listenWindowsWebViewBridge) wrapCallback(
	name string,
	argumentCount int,
	callback func(next reflect.Value, arguments []reflect.Value),
) bool {
	if bridge == nil || !bridge.chromium.IsValid() || callback == nil {
		return false
	}
	field := bridge.chromium.FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.Func || !field.CanAddr() ||
		field.Type().NumIn() != argumentCount || field.Type().NumOut() != 0 {
		return false
	}
	writable := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	next := reflect.New(field.Type()).Elem()
	next.Set(writable)
	writable.Set(reflect.MakeFunc(field.Type(), func(arguments []reflect.Value) (results []reflect.Value) {
		defer func() {
			if recovered := recover(); recovered != nil {
				zap.L().Error(
					"Windows WebView2 callback panicked",
					zap.String("callback", name),
					zap.Any("panic", recovered),
					zap.Stack("stack"),
				)
			}
		}()
		callback(next, arguments)
		return nil
	}))
	return true
}

func (bridge *listenWindowsWebViewBridge) WrapWebResourceRequested(
	callback func(request *edge.ICoreWebView2WebResourceRequest),
) bool {
	if callback == nil {
		return false
	}
	return bridge.wrapCallback("WebResourceRequestedCallback", 2, func(next reflect.Value, arguments []reflect.Value) {
		if !listenWindowsValidCallbackPointers(arguments, 2, 0, 1) {
			return
		}
		if !next.IsNil() {
			next.Call(arguments)
		}
		callback((*edge.ICoreWebView2WebResourceRequest)(arguments[0].UnsafePointer()))
	})
}

// WrapNavigationCompleted runs callback after Wails has processed the event.
// That ordering matters for hidden media windows because Wails' first-paint
// handler ends by setting the controller visibility back to false.
func (bridge *listenWindowsWebViewBridge) WrapNavigationCompleted(
	callback func(),
) bool {
	if callback == nil {
		return false
	}
	return bridge.wrapCallback("NavigationCompletedCallback", 2, func(next reflect.Value, arguments []reflect.Value) {
		if !listenWindowsValidCallbackPointers(arguments, 2, 0, 1) {
			return
		}
		if !next.IsNil() {
			next.Call(arguments)
		}
		callback()
	})
}

func (bridge *listenWindowsWebViewBridge) WrapContainsFullScreenElementChanged(
	callback func(sender *edge.ICoreWebView2) bool,
) bool {
	if callback == nil {
		return false
	}
	return bridge.wrapCallback("ContainsFullScreenElementChangedCallback", 2, func(next reflect.Value, arguments []reflect.Value) {
		// WebView2 defines this event's second IUnknown argument as null because
		// the event has no event-data object. Only the sender is required.
		if !listenWindowsValidCallbackPointers(arguments, 2, 0) {
			return
		}
		handled := callback((*edge.ICoreWebView2)(arguments[0].UnsafePointer()))
		// Wails' default callback fullscreens the dedicated player HWND. An
		// embedded player instead fills the host as its bottom child; only the
		// non-embedded path may invoke Wails' original callback.
		if !handled && !next.IsNil() {
			next.Call(arguments)
		}
	})
}

func listenWindowsValidCallbackPointers(arguments []reflect.Value, count int, required ...int) bool {
	if len(arguments) != count {
		return false
	}
	for _, index := range required {
		if index < 0 || index >= len(arguments) {
			return false
		}
		argument := arguments[index]
		if argument.Kind() != reflect.Pointer || argument.IsNil() {
			return false
		}
	}
	return true
}

func listenWindowsEmbeddedHostWebViewForWindow(window *application.WebviewWindow) (listenWindowsEmbeddedHostWebView, bool) {
	restoreColor, ok := listenWindowsWebviewWindowDefaultBackground(window)
	if !ok {
		return listenWindowsEmbeddedHostWebView{}, false
	}
	webview := listenWindowsWebViewForWindow(window)
	if webview == nil || webview.Controller() == nil || !webview.CompositionControllerReady() {
		return listenWindowsEmbeddedHostWebView{}, false
	}
	if cached, ok := listenWindowsEmbeddedHostControllers.Load(window.ID()); ok {
		cachedHost, valid := cached.(listenWindowsEmbeddedHostControllerCache)
		if valid && cachedHost.window == window && cachedHost.controller != nil {
			return listenWindowsEmbeddedHostWebView{
				controller:   cachedHost.controller,
				restoreColor: restoreColor,
				window:       window,
			}, true
		}
		if valid {
			// If the stale cache is backing the current underlay, restore that
			// underlay before releasing Controller2. Restoration still needs the
			// interface to put the host background colour back.
			listenWindowsEmbeddedWebViewLock.Lock()
			if listenWindowsEmbeddedWebView.active &&
				listenWindowsEmbeddedWebView.hostWindow == cachedHost.window &&
				listenWindowsEmbeddedWebView.hostController == cachedHost.controller {
				listenWindowsRestoreEmbeddedWebViewLocked()
			}
			listenWindowsEmbeddedWebViewLock.Unlock()
			if listenWindowsEmbeddedHostControllers.CompareAndDelete(window.ID(), cachedHost) {
				listenWindowsReleaseController2(cachedHost.controller)
			}
		} else {
			listenWindowsEmbeddedHostControllers.Delete(window.ID())
		}
	}
	controller := webview.Controller().GetICoreWebView2Controller2()
	if controller == nil {
		return listenWindowsEmbeddedHostWebView{}, false
	}
	listenWindowsEmbeddedHostControllers.Store(window.ID(), listenWindowsEmbeddedHostControllerCache{
		window:     window,
		controller: controller,
	})
	return listenWindowsEmbeddedHostWebView{
		controller:   controller,
		restoreColor: restoreColor,
		window:       window,
		newlyCached:  true,
	}, true
}

// GetICoreWebView2Controller2 is a QueryInterface call and returns an owning
// reference. Release that exact Controller2 interface; the bridge's base
// controller pointer is borrowed from Wails and must never be released here.
func listenWindowsReleaseController2(controller *edge.ICoreWebView2Controller2) {
	if controller != nil {
		(*edge.ICoreWebView2Controller)(unsafe.Pointer(controller)).Release()
	}
}

func listenWindowsReleaseEmbeddedHostController(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	value, ok := listenWindowsEmbeddedHostControllers.Load(window.ID())
	if !ok {
		return
	}
	cache, ok := value.(listenWindowsEmbeddedHostControllerCache)
	if !ok || cache.window != window {
		return
	}
	if listenWindowsEmbeddedHostControllers.CompareAndDelete(window.ID(), cache) {
		listenWindowsReleaseController2(cache.controller)
	}
}

func listenWindowsCoreWebViewColor(background application.RGBA) edge.COREWEBVIEW2_COLOR {
	return edge.COREWEBVIEW2_COLOR{
		A: background.Alpha,
		R: background.Red,
		G: background.Green,
		B: background.Blue,
	}
}

// SetBackgroundColour updates both Wails' option snapshot and WebView2's
// native controller. When a player HWND is mounted underneath the main
// WebView, immediately restore transparency while retaining the new colour as
// the value to restore after playback leaves the underlay.
func syncListenNativeVideoHostBackground(window *application.WebviewWindow, background application.RGBA) {
	if window == nil {
		return
	}
	hostHWND := listenWindowsHWND(window.NativeWindow())
	if hostHWND == 0 {
		return
	}

	listenWindowsEmbeddedWebViewLock.Lock()
	defer listenWindowsEmbeddedWebViewLock.Unlock()
	state := &listenWindowsEmbeddedWebView
	if !state.active || state.hostHWND != hostHWND {
		return
	}
	state.hostWindow = window
	state.hostRestoreColor = listenWindowsCoreWebViewColor(background)
	if restoreColor, ok := listenWindowsWebviewWindowDefaultBackground(window); ok {
		state.hostRestoreColor = restoreColor
	}
	if state.hostTransparent && listenWindowsEmbeddedHostControllerIsLive(*state) {
		_ = state.hostController.PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{})
	}
}

// Wails keeps translucent and transparent WebViews clear even when their
// BackgroundColour option is an opaque theme fallback. Restore the controller
// according to BackgroundType so leaving a video underlay never paints over a
// native Acrylic backdrop. Reading the option snapshot is also safer than WebView2's v1
// colour getter, whose wrapper treats the four-byte value as a pointer.
func listenWindowsWebviewWindowDefaultBackground(window *application.WebviewWindow) (edge.COREWEBVIEW2_COLOR, bool) {
	options, ok := listenWindowsWebviewWindowOptions(window)
	if !ok {
		return edge.COREWEBVIEW2_COLOR{}, false
	}
	switch options.BackgroundType {
	case application.BackgroundTypeTransparent, application.BackgroundTypeTranslucent:
		return edge.COREWEBVIEW2_COLOR{}, true
	default:
		return listenWindowsCoreWebViewColor(options.BackgroundColour), true
	}
}

func listenWindowsWebviewWindowOptions(window *application.WebviewWindow) (application.WebviewWindowOptions, bool) {
	if window == nil {
		return application.WebviewWindowOptions{}, false
	}
	windowValue := reflect.ValueOf(window)
	if windowValue.Kind() != reflect.Pointer || windowValue.IsNil() {
		return application.WebviewWindowOptions{}, false
	}
	optionsField := windowValue.Elem().FieldByName("options")
	if !optionsField.IsValid() || !optionsField.CanAddr() {
		return application.WebviewWindowOptions{}, false
	}
	optionsValue := reflect.NewAt(optionsField.Type(), unsafe.Pointer(optionsField.UnsafeAddr())).Elem()
	options, ok := optionsValue.Interface().(application.WebviewWindowOptions)
	if !ok {
		return application.WebviewWindowOptions{}, false
	}
	return options, true
}

type listenWindowsCoreWebView2 struct {
	vtbl *listenWindowsCoreWebView2Vtbl
}

type listenWindowsCoreWebView2Vtbl struct {
	QueryInterface                         edge.ComProc
	AddRef                                 edge.ComProc
	Release                                edge.ComProc
	GetSettings                            edge.ComProc
	GetSource                              edge.ComProc
	Navigate                               edge.ComProc
	NavigateToString                       edge.ComProc
	AddNavigationStarting                  edge.ComProc
	RemoveNavigationStarting               edge.ComProc
	AddContentLoading                      edge.ComProc
	RemoveContentLoading                   edge.ComProc
	AddSourceChanged                       edge.ComProc
	RemoveSourceChanged                    edge.ComProc
	AddHistoryChanged                      edge.ComProc
	RemoveHistoryChanged                   edge.ComProc
	AddNavigationCompleted                 edge.ComProc
	RemoveNavigationCompleted              edge.ComProc
	AddFrameNavigationStarting             edge.ComProc
	RemoveFrameNavigationStarting          edge.ComProc
	AddFrameNavigationCompleted            edge.ComProc
	RemoveFrameNavigationCompleted         edge.ComProc
	AddScriptDialogOpening                 edge.ComProc
	RemoveScriptDialogOpening              edge.ComProc
	AddPermissionRequested                 edge.ComProc
	RemovePermissionRequested              edge.ComProc
	AddProcessFailed                       edge.ComProc
	RemoveProcessFailed                    edge.ComProc
	AddScriptToExecuteOnDocumentCreated    edge.ComProc
	RemoveScriptToExecuteOnDocumentCreated edge.ComProc
	ExecuteScript                          edge.ComProc
	CapturePreview                         edge.ComProc
	Reload                                 edge.ComProc
	PostWebMessageAsJSON                   edge.ComProc
	PostWebMessageAsString                 edge.ComProc
	AddWebMessageReceived                  edge.ComProc
	RemoveWebMessageReceived               edge.ComProc
	CallDevToolsProtocolMethod             edge.ComProc
	GetBrowserProcessID                    edge.ComProc
	GetCanGoBack                           edge.ComProc
	GetCanGoForward                        edge.ComProc
	GoBack                                 edge.ComProc
	GoForward                              edge.ComProc
	GetDevToolsProtocolEventReceiver       edge.ComProc
	Stop                                   edge.ComProc
	AddNewWindowRequested                  edge.ComProc
	RemoveNewWindowRequested               edge.ComProc
	AddDocumentTitleChanged                edge.ComProc
	RemoveDocumentTitleChanged             edge.ComProc
	GetDocumentTitle                       edge.ComProc
	AddHostObjectToScript                  edge.ComProc
	RemoveHostObjectFromScript             edge.ComProc
	OpenDevToolsWindow                     edge.ComProc
	AddContainsFullScreenElementChanged    edge.ComProc
	RemoveContainsFullScreenElementChanged edge.ComProc
	GetContainsFullScreenElement           edge.ComProc
}

type listenWindowsEventRegistrationToken struct {
	value int64
}

const listenWindowsDocumentStartRegistrationTimeout = 2 * time.Second

type listenWindowsDocumentStartScriptRegistration struct {
	done             chan struct{}
	completeOnce     sync.Once
	ownerReleaseOnce sync.Once

	mu        sync.RWMutex
	errorCode uintptr
	scriptID  string
	handler   *listenWindowsDocumentStartScriptCompletedHandler
}

type listenWindowsDocumentStartScriptCompletedHandlerVtbl struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
	Invoke         edge.ComProc
}

type listenWindowsDocumentStartScriptCompletedHandler struct {
	vtbl         *listenWindowsDocumentStartScriptCompletedHandlerVtbl
	registration *listenWindowsDocumentStartScriptRegistration
	refCount     atomic.Int32
}

type listenWindowsNavigationStartingEventArgs struct {
	vtbl *listenWindowsNavigationStartingEventArgsVtbl
}

type listenWindowsNavigationStartingEventArgsVtbl struct {
	QueryInterface     edge.ComProc
	AddRef             edge.ComProc
	Release            edge.ComProc
	GetURI             edge.ComProc
	GetIsUserInitiated edge.ComProc
	GetIsRedirected    edge.ComProc
	GetRequestHeaders  edge.ComProc
	GetCancel          edge.ComProc
	PutCancel          edge.ComProc
	GetNavigationID    edge.ComProc
}

type listenWindowsNewWindowRequestedEventArgs struct {
	vtbl *listenWindowsNewWindowRequestedEventArgsVtbl
}

type listenWindowsNewWindowRequestedEventArgsVtbl struct {
	QueryInterface     edge.ComProc
	AddRef             edge.ComProc
	Release            edge.ComProc
	GetURI             edge.ComProc
	PutNewWindow       edge.ComProc
	GetNewWindow       edge.ComProc
	PutHandled         edge.ComProc
	GetHandled         edge.ComProc
	GetIsUserInitiated edge.ComProc
	GetDeferral        edge.ComProc
	GetWindowFeatures  edge.ComProc
}

type listenWindowsNavigationStartingEventHandlerVtbl struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
	Invoke         edge.ComProc
}

type listenWindowsNavigationStartingEventHandler struct {
	vtbl  *listenWindowsNavigationStartingEventHandlerVtbl
	state *listenWindowsRemoteNavigationState
}

type listenWindowsNewWindowRequestedEventHandlerVtbl struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
	Invoke         edge.ComProc
}

type listenWindowsNewWindowRequestedEventHandler struct {
	vtbl *listenWindowsNewWindowRequestedEventHandlerVtbl
}

type listenWindowsRemoteNavigationState struct {
	mu sync.RWMutex

	policy webViewRemoteNavigationPolicy
	core   *listenWindowsCoreWebView2

	navigationToken   listenWindowsEventRegistrationToken
	navigationHandler *listenWindowsNavigationStartingEventHandler
}

type listenWindowsPersistentPopupState struct {
	core    *listenWindowsCoreWebView2
	token   listenWindowsEventRegistrationToken
	handler *listenWindowsNewWindowRequestedEventHandler
}

var (
	listenWindowsIUnknownIID = windows.GUID{
		Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	listenWindowsNavigationStartingEventHandlerIID = windows.GUID{
		Data1: 0x9adbe429,
		Data2: 0xf36d,
		Data3: 0x432b,
		Data4: [8]byte{0x9d, 0xdc, 0xf8, 0x88, 0x1f, 0xbd, 0x76, 0xe3},
	}
	listenWindowsNewWindowRequestedEventHandlerIID = windows.GUID{
		Data1: 0xd4c185fe,
		Data2: 0xc81c,
		Data3: 0x4989,
		Data4: [8]byte{0x97, 0xaf, 0x2d, 0x3f, 0xa7, 0xab, 0x56, 0x51},
	}
	listenWindowsDocumentStartScriptCompletedHandlerIID = windows.GUID{
		Data1: 0xb99369f3,
		Data2: 0x9b11,
		Data3: 0x47b5,
		Data4: [8]byte{0xbc, 0x6f, 0x8e, 0x78, 0x95, 0xfc, 0xea, 0x17},
	}
	listenWindowsDocumentStartScriptCompletedHandlerVTable = listenWindowsDocumentStartScriptCompletedHandlerVtbl{
		QueryInterface: edge.NewComProc(listenWindowsDocumentStartScriptCompletedHandlerQueryInterface),
		AddRef:         edge.NewComProc(listenWindowsDocumentStartScriptCompletedHandlerAddRef),
		Release:        edge.NewComProc(listenWindowsDocumentStartScriptCompletedHandlerRelease),
		Invoke:         edge.NewComProc(listenWindowsDocumentStartScriptCompletedHandlerInvoke),
	}
	listenWindowsNavigationStartingEventHandlerVTable = listenWindowsNavigationStartingEventHandlerVtbl{
		QueryInterface: edge.NewComProc(listenWindowsNavigationStartingEventHandlerQueryInterface),
		AddRef:         edge.NewComProc(listenWindowsNavigationStartingEventHandlerAddRef),
		Release:        edge.NewComProc(listenWindowsNavigationStartingEventHandlerRelease),
		Invoke:         edge.NewComProc(listenWindowsNavigationStartingEventHandlerInvoke),
	}
	listenWindowsNewWindowRequestedEventHandlerVTable = listenWindowsNewWindowRequestedEventHandlerVtbl{
		QueryInterface: edge.NewComProc(listenWindowsNewWindowRequestedEventHandlerQueryInterface),
		AddRef:         edge.NewComProc(listenWindowsNewWindowRequestedEventHandlerAddRef),
		Release:        edge.NewComProc(listenWindowsNewWindowRequestedEventHandlerRelease),
		Invoke:         edge.NewComProc(listenWindowsNewWindowRequestedEventHandlerInvoke),
	}
)

func (args *listenWindowsNavigationStartingEventArgs) URI() (string, error) {
	if args == nil || args.vtbl == nil {
		return "", errors.New("WebView2 navigation args are unavailable")
	}
	var rawURI *uint16
	hr, _, _ := args.vtbl.GetURI.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(unsafe.Pointer(&rawURI)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return "", syscall.Errno(hr)
	}
	if rawURI == nil {
		return "", errors.New("WebView2 navigation URI is unavailable")
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(rawURI))
	return windows.UTF16PtrToString(rawURI), nil
}

func (args *listenWindowsNavigationStartingEventArgs) Cancel() error {
	if args == nil || args.vtbl == nil {
		return errors.New("WebView2 navigation args are unavailable")
	}
	// WebView2 BOOL setters take the four-byte value by value. Passing a Go
	// bool pointer (as the legacy wrapper does) corrupts the COM call ABI.
	hr, _, _ := args.vtbl.PutCancel.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(1),
	)
	if windows.Handle(hr) != windows.S_OK {
		return syscall.Errno(hr)
	}
	return nil
}

func (args *listenWindowsNewWindowRequestedEventArgs) Handle() error {
	if args == nil || args.vtbl == nil {
		return errors.New("WebView2 new-window args are unavailable")
	}
	hr, _, _ := args.vtbl.PutHandled.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(1),
	)
	if windows.Handle(hr) != windows.S_OK {
		return syscall.Errno(hr)
	}
	return nil
}

func (registration *listenWindowsDocumentStartScriptRegistration) complete(
	errorCode uintptr,
	scriptID string,
) {
	if registration == nil {
		return
	}
	registration.completeOnce.Do(func() {
		registration.mu.Lock()
		registration.errorCode = errorCode
		registration.scriptID = scriptID
		registration.mu.Unlock()
		close(registration.done)
	})
}

func (registration *listenWindowsDocumentStartScriptRegistration) wait(
	timeout time.Duration,
) error {
	if registration == nil {
		return errors.New("WebView2 document-start registration is unavailable")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-registration.done:
		registration.mu.RLock()
		errorCode := registration.errorCode
		scriptID := registration.scriptID
		registration.mu.RUnlock()
		if code := uint32(errorCode); code != uint32(windows.S_OK) {
			return fmt.Errorf(
				"WebView2 document-start registration failed (HRESULT 0x%08X): %w",
				code,
				syscall.Errno(code),
			)
		}
		if strings.TrimSpace(scriptID) == "" {
			return errors.New("WebView2 document-start registration returned an empty script ID")
		}
		return nil
	case <-timer.C:
		return fmt.Errorf(
			"WebView2 document-start registration timed out after %s",
			timeout,
		)
	}
}

func (registration *listenWindowsDocumentStartScriptRegistration) releaseOwner() {
	if registration == nil {
		return
	}
	registration.ownerReleaseOnce.Do(func() {
		if registration.handler != nil {
			listenWindowsDocumentStartScriptCompletedHandlerRelease(registration.handler)
		}
	})
}

func (core *listenWindowsCoreWebView2) beginDocumentStartScriptRegistration(
	script string,
) (*listenWindowsDocumentStartScriptRegistration, error) {
	if core == nil || core.vtbl == nil {
		return nil, errors.New("WebView2 core is unavailable")
	}
	if script == "" {
		return nil, errors.New("WebView2 document-start script is empty")
	}
	utf16Script, err := windows.UTF16PtrFromString(script)
	if err != nil {
		return nil, err
	}
	registration := &listenWindowsDocumentStartScriptRegistration{
		done: make(chan struct{}),
	}
	handler := &listenWindowsDocumentStartScriptCompletedHandler{
		vtbl:         &listenWindowsDocumentStartScriptCompletedHandlerVTable,
		registration: registration,
	}
	handler.refCount.Store(1)
	registration.handler = handler
	// The WebView2 API retains the COM interface, but the Go GC cannot observe
	// that native reference. Keep a Go root until COM and our owner both release.
	listenWindowsPendingDocumentStartRegistrations.Store(handler, handler)

	hr, _, _ := core.vtbl.AddScriptToExecuteOnDocumentCreated.Call(
		uintptr(unsafe.Pointer(core)),
		uintptr(unsafe.Pointer(utf16Script)),
		uintptr(unsafe.Pointer(handler)),
	)
	runtime.KeepAlive(utf16Script)
	runtime.KeepAlive(handler)
	if windows.Handle(hr) != windows.S_OK {
		registration.releaseOwner()
		return nil, fmt.Errorf(
			"submit WebView2 document-start registration (HRESULT 0x%08X): %w",
			uint32(hr),
			syscall.Errno(uint32(hr)),
		)
	}
	return registration, nil
}

func (core *listenWindowsCoreWebView2) addNavigationStarting(
	handler *listenWindowsNavigationStartingEventHandler,
) (listenWindowsEventRegistrationToken, error) {
	if core == nil || core.vtbl == nil || handler == nil {
		return listenWindowsEventRegistrationToken{}, errors.New("WebView2 navigation handler is unavailable")
	}
	var token listenWindowsEventRegistrationToken
	hr, _, _ := core.vtbl.AddNavigationStarting.Call(
		uintptr(unsafe.Pointer(core)),
		uintptr(unsafe.Pointer(handler)),
		uintptr(unsafe.Pointer(&token)),
	)
	runtime.KeepAlive(handler)
	if windows.Handle(hr) != windows.S_OK {
		return listenWindowsEventRegistrationToken{}, syscall.Errno(hr)
	}
	return token, nil
}

func (core *listenWindowsCoreWebView2) removeNavigationStarting(token listenWindowsEventRegistrationToken) error {
	if core == nil || core.vtbl == nil {
		return errors.New("WebView2 core is unavailable")
	}
	return listenWindowsRemoveEventHandler(core.vtbl.RemoveNavigationStarting, core, token)
}

func (core *listenWindowsCoreWebView2) addNewWindowRequested(
	handler *listenWindowsNewWindowRequestedEventHandler,
) (listenWindowsEventRegistrationToken, error) {
	if core == nil || core.vtbl == nil || handler == nil {
		return listenWindowsEventRegistrationToken{}, errors.New("WebView2 new-window handler is unavailable")
	}
	var token listenWindowsEventRegistrationToken
	hr, _, _ := core.vtbl.AddNewWindowRequested.Call(
		uintptr(unsafe.Pointer(core)),
		uintptr(unsafe.Pointer(handler)),
		uintptr(unsafe.Pointer(&token)),
	)
	runtime.KeepAlive(handler)
	if windows.Handle(hr) != windows.S_OK {
		return listenWindowsEventRegistrationToken{}, syscall.Errno(hr)
	}
	return token, nil
}

func (core *listenWindowsCoreWebView2) removeNewWindowRequested(token listenWindowsEventRegistrationToken) error {
	if core == nil || core.vtbl == nil {
		return errors.New("WebView2 core is unavailable")
	}
	return listenWindowsRemoveEventHandler(core.vtbl.RemoveNewWindowRequested, core, token)
}

func listenWindowsRemoveEventHandler(
	remove edge.ComProc,
	core *listenWindowsCoreWebView2,
	token listenWindowsEventRegistrationToken,
) error {
	// EventRegistrationToken is an eight-byte struct passed by value. The
	// legacy Go wrapper passes its address, which turns the address itself into
	// the token on 64-bit Windows and leaves the callback registered. Split the
	// value into two stack words only for the 32-bit COM ABI.
	var hr uintptr
	if unsafe.Sizeof(uintptr(0)) == 4 {
		raw := uint64(token.value)
		hr, _, _ = remove.Call(
			uintptr(unsafe.Pointer(core)),
			uintptr(uint32(raw)),
			uintptr(uint32(raw>>32)),
		)
	} else {
		hr, _, _ = remove.Call(
			uintptr(unsafe.Pointer(core)),
			uintptr(token.value),
		)
	}
	if windows.Handle(hr) != windows.S_OK {
		return syscall.Errno(hr)
	}
	return nil
}

func (state *listenWindowsRemoteNavigationState) update(policy webViewRemoteNavigationPolicy) {
	if state == nil {
		return
	}
	state.mu.Lock()
	state.policy = policy
	state.mu.Unlock()
}

func (state *listenWindowsRemoteNavigationState) allows(rawURL string) bool {
	if state == nil {
		return false
	}
	state.mu.RLock()
	policy := state.policy
	state.mu.RUnlock()
	return policy.allows(rawURL)
}

func listenWindowsDocumentStartScriptCompletedHandlerQueryInterface(
	this *listenWindowsDocumentStartScriptCompletedHandler,
	refiid uintptr,
	object uintptr,
) uintptr {
	result := listenWindowsRemoteEventHandlerQueryInterface(
		unsafe.Pointer(this),
		refiid,
		object,
		listenWindowsDocumentStartScriptCompletedHandlerIID,
	)
	if result == uintptr(windows.S_OK) {
		listenWindowsDocumentStartScriptCompletedHandlerAddRef(this)
	}
	return result
}

func listenWindowsDocumentStartScriptCompletedHandlerAddRef(
	this *listenWindowsDocumentStartScriptCompletedHandler,
) uintptr {
	if this == nil {
		return 0
	}
	return uintptr(this.refCount.Add(1))
}

func listenWindowsDocumentStartScriptCompletedHandlerRelease(
	this *listenWindowsDocumentStartScriptCompletedHandler,
) uintptr {
	if this == nil {
		return 0
	}
	remaining := this.refCount.Add(-1)
	if remaining <= 0 {
		listenWindowsPendingDocumentStartRegistrations.Delete(this)
		return 0
	}
	return uintptr(remaining)
}

func listenWindowsDocumentStartScriptCompletedHandlerInvoke(
	this *listenWindowsDocumentStartScriptCompletedHandler,
	errorCode uintptr,
	scriptID *uint16,
) uintptr {
	if this == nil || this.registration == nil {
		return uintptr(windows.S_OK)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			this.registration.complete(uintptr(0x80004005), "")
			this.registration.releaseOwner()
		}
	}()
	id := ""
	if scriptID != nil {
		// The completion callback lends this LPCWSTR only for Invoke. Copy it
		// and never pass its native address beyond the callback.
		id = windows.UTF16PtrToString(scriptID)
	}
	this.registration.complete(errorCode, id)
	this.registration.releaseOwner()
	return uintptr(windows.S_OK)
}

func listenWindowsRemoteEventHandlerQueryInterface(
	this unsafe.Pointer,
	refiid uintptr,
	object uintptr,
	handlerIID windows.GUID,
) uintptr {
	const (
		listenWindowsEPointer     = uintptr(0x80004003)
		listenWindowsENoInterface = uintptr(0x80004002)
	)
	if object == 0 {
		return listenWindowsEPointer
	}
	result := (*unsafe.Pointer)(unsafe.Pointer(object))
	*result = nil
	if this == nil || refiid == 0 {
		return listenWindowsENoInterface
	}
	iid := *(*windows.GUID)(unsafe.Pointer(refiid))
	if iid != listenWindowsIUnknownIID && iid != handlerIID {
		return listenWindowsENoInterface
	}
	*result = this
	return uintptr(windows.S_OK)
}

func listenWindowsNavigationStartingEventHandlerQueryInterface(
	this *listenWindowsNavigationStartingEventHandler,
	refiid uintptr,
	object uintptr,
) uintptr {
	return listenWindowsRemoteEventHandlerQueryInterface(
		unsafe.Pointer(this),
		refiid,
		object,
		listenWindowsNavigationStartingEventHandlerIID,
	)
}

func listenWindowsNavigationStartingEventHandlerAddRef(_ *listenWindowsNavigationStartingEventHandler) uintptr {
	return 1
}

func listenWindowsNavigationStartingEventHandlerRelease(_ *listenWindowsNavigationStartingEventHandler) uintptr {
	return 1
}

func listenWindowsNavigationStartingEventHandlerInvoke(
	this *listenWindowsNavigationStartingEventHandler,
	_ *listenWindowsCoreWebView2,
	args *listenWindowsNavigationStartingEventArgs,
) uintptr {
	if args == nil {
		return uintptr(windows.S_OK)
	}
	// URI extraction and policy evaluation both fail closed. The deferred guard
	// also cancels if an unexpected Go panic crosses this COM callback boundary.
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = args.Cancel()
		}
	}()
	rawURL, err := args.URI()
	allowed := err == nil && this != nil && this.state != nil
	if allowed && !this.state.allows(rawURL) {
		allowed = false
	}
	if !allowed {
		_ = args.Cancel()
	}
	return uintptr(windows.S_OK)
}

func listenWindowsNewWindowRequestedEventHandlerQueryInterface(
	this *listenWindowsNewWindowRequestedEventHandler,
	refiid uintptr,
	object uintptr,
) uintptr {
	return listenWindowsRemoteEventHandlerQueryInterface(
		unsafe.Pointer(this),
		refiid,
		object,
		listenWindowsNewWindowRequestedEventHandlerIID,
	)
}

func listenWindowsNewWindowRequestedEventHandlerAddRef(_ *listenWindowsNewWindowRequestedEventHandler) uintptr {
	return 1
}

func listenWindowsNewWindowRequestedEventHandlerRelease(_ *listenWindowsNewWindowRequestedEventHandler) uintptr {
	return 1
}

func listenWindowsNewWindowRequestedEventHandlerInvoke(
	_ *listenWindowsNewWindowRequestedEventHandler,
	_ *listenWindowsCoreWebView2,
	args *listenWindowsNewWindowRequestedEventArgs,
) uintptr {
	// Every popup is owned by the host. Setting Handled synchronously and not
	// supplying NewWindow makes window.open/target=_blank a no-navigation sink.
	if args != nil {
		_ = args.Handle()
	}
	return uintptr(windows.S_OK)
}

func installListenWindowsRemoteNavigationPolicy(
	window *application.WebviewWindow,
	policy webViewRemoteNavigationPolicy,
) bool {
	if window == nil || policy.kind == webViewRemoteNavigationPolicyInvalid {
		return false
	}
	// Popup denial is intentionally independent from top-level navigation so
	// local shell windows can keep the same handler for their entire lifetime.
	if !installListenWindowsPersistentPopupPolicy(window) {
		return false
	}
	installed := false
	application.InvokeSync(func() {
		windowID := window.ID()
		if existing, ok := listenWindowsRemoteNavigationPolicies.Load(windowID); ok {
			state, valid := existing.(*listenWindowsRemoteNavigationState)
			if valid && state != nil {
				state.update(policy)
				installed = true
			}
			return
		}

		webview := listenWindowsWebViewForWindow(window)
		core := listenWindowsWebViewCore(webview)
		if core == nil || core.vtbl == nil {
			return
		}
		state := &listenWindowsRemoteNavigationState{
			policy: policy,
			core:   core,
		}
		state.navigationHandler = &listenWindowsNavigationStartingEventHandler{
			vtbl:  &listenWindowsNavigationStartingEventHandlerVTable,
			state: state,
		}

		navigationToken, err := core.addNavigationStarting(state.navigationHandler)
		if err != nil {
			return
		}
		state.navigationToken = navigationToken
		listenWindowsRemoteNavigationPolicies.Store(windowID, state)
		installed = true
	})
	return installed
}

func installListenWindowsPersistentPopupPolicy(window *application.WebviewWindow) bool {
	return installListenWindowsPersistentPopupPolicyWhileActive(window, nil)
}

func installListenWindowsPersistentPopupPolicyWhileActive(
	window *application.WebviewWindow,
	isActive func() bool,
) bool {
	// application.InvokeSync dereferences Wails' platform App implementation,
	// which does not exist until Application.Run starts. A pending window has no
	// native handle either, so return before crossing the main-thread boundary.
	// The registration wrapper retries after WebViewNavigationCompleted.
	if window == nil || window.NativeWindow() == nil ||
		(isActive != nil && !isActive()) {
		return false
	}
	installed := false
	application.InvokeSync(func() {
		if isActive != nil && !isActive() {
			return
		}
		windowID := window.ID()
		if existing, ok := listenWindowsPersistentPopupPolicies.Load(windowID); ok {
			state, valid := existing.(*listenWindowsPersistentPopupState)
			installed = valid && state != nil
			return
		}

		webview := listenWindowsWebViewForWindow(window)
		core := listenWindowsWebViewCore(webview)
		if core == nil || core.vtbl == nil {
			return
		}
		state := &listenWindowsPersistentPopupState{
			core: core,
			handler: &listenWindowsNewWindowRequestedEventHandler{
				vtbl: &listenWindowsNewWindowRequestedEventHandlerVTable,
			},
		}
		token, err := core.addNewWindowRequested(state.handler)
		if err != nil {
			return
		}
		state.token = token
		if isActive != nil && !isActive() {
			_ = core.removeNewWindowRequested(state.token)
			runtime.KeepAlive(state.handler)
			return
		}
		listenWindowsPersistentPopupPolicies.Store(windowID, state)
		installed = true
	})
	return installed
}

func releaseListenWindowsRemoteNavigationPolicy(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	cancelWebViewRemoteCapabilityPolicyRegistration(window)
	if window.NativeWindow() == nil {
		if value, ok := listenWindowsRemoteNavigationPolicies.LoadAndDelete(window.ID()); ok {
			if state, valid := value.(*listenWindowsRemoteNavigationState); valid {
				runtime.KeepAlive(state)
			}
		}
		releaseListenWindowsPersistentPopupPolicy(window)
		return
	}
	application.InvokeSync(func() {
		value, ok := listenWindowsRemoteNavigationPolicies.LoadAndDelete(window.ID())
		if !ok {
			return
		}
		state, valid := value.(*listenWindowsRemoteNavigationState)
		if !valid || state == nil || state.core == nil {
			return
		}
		// WindowClosing listeners run concurrently in Wails. The native window
		// can therefore disappear after the pre-InvokeSync guard but before this
		// closure reaches the UI thread. Detach the Go state first, then avoid a
		// COM call if native destruction already won that race.
		if window.NativeWindow() == nil {
			runtime.KeepAlive(state)
			return
		}
		_ = state.core.removeNavigationStarting(state.navigationToken)
		runtime.KeepAlive(state.navigationHandler)
	})
	releaseListenWindowsPersistentPopupPolicy(window)
}

func releaseListenWindowsPersistentPopupPolicy(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	cancelWebViewRemoteCapabilityPolicyRegistration(window)
	if window.NativeWindow() == nil {
		if value, ok := listenWindowsPersistentPopupPolicies.LoadAndDelete(window.ID()); ok {
			if state, valid := value.(*listenWindowsPersistentPopupState); valid {
				runtime.KeepAlive(state)
			}
		}
		return
	}
	application.InvokeSync(func() {
		value, ok := listenWindowsPersistentPopupPolicies.LoadAndDelete(window.ID())
		if !ok {
			return
		}
		state, ok := value.(*listenWindowsPersistentPopupState)
		if !ok || state == nil || state.core == nil {
			return
		}
		// Recheck on the UI thread for the same close/destruction race handled by
		// releaseListenWindowsRemoteNavigationPolicy above.
		if window.NativeWindow() == nil {
			runtime.KeepAlive(state)
			return
		}
		_ = state.core.removeNewWindowRequested(state.token)
		runtime.KeepAlive(state.handler)
	})
}

// The Wails alpha2.117 copy of WebView2 correctly receives the Win32 BOOL
// out-parameter into four bytes. The standalone v1.0.28 wrapper still uses a
// one-byte Go bool, so call the same COM slot directly to avoid a stack write
// past the destination.
func listenWindowsGetContainsFullScreenElement(sender *edge.ICoreWebView2) (bool, error) {
	webview := (*listenWindowsCoreWebView2)(unsafe.Pointer(sender))
	if webview == nil || webview.vtbl == nil {
		return false, errors.New("WebView2 core is not ready")
	}
	var result int32
	hr, _, _ := webview.vtbl.GetContainsFullScreenElement.Call(
		uintptr(unsafe.Pointer(webview)),
		uintptr(unsafe.Pointer(&result)),
	)
	if hr != 0 {
		return false, syscall.Errno(hr)
	}
	return result != 0, nil
}

// Go cannot marshal a by-value double into a Windows ARM64 COM call today.
// Keep such cookies session-scoped there instead of invoking the standalone
// wrapper's unsafe PutExpires implementation with the bits in an integer
// register. Other cookie attributes and both amd64 persistence paths remain
// unchanged.
func listenWindowsPutCookieExpires(cookie *edge.ICoreWebView2Cookie, expires float64) error {
	if cookie == nil {
		return errors.New("WebView2 cookie is not ready")
	}
	if runtime.GOARCH == "arm64" {
		return nil
	}
	return cookie.PutExpires(expires)
}

func listenWindowsWebViewBrowserProcessID(bridge *listenWindowsWebViewBridge) uint32 {
	webview := listenWindowsWebViewCore(bridge)
	if webview == nil || webview.vtbl == nil {
		return 0
	}
	var processID uint32
	hr, _, _ := webview.vtbl.GetBrowserProcessID.Call(
		uintptr(unsafe.Pointer(webview)),
		uintptr(unsafe.Pointer(&processID)),
	)
	if hr != 0 {
		return 0
	}
	return processID
}

func listenWindowsWebViewCore(bridge *listenWindowsWebViewBridge) *listenWindowsCoreWebView2 {
	if bridge == nil || bridge.Core() == nil {
		return nil
	}
	return (*listenWindowsCoreWebView2)(unsafe.Pointer(bridge.Core()))
}

func execListenYouTubeMusicJS(window *application.WebviewWindow, script string) {
	if window == nil || script == "" {
		return
	}
	markListenYouTubeMusicRuntimeReady(window)
	window.ExecJS(script)
}

// Wails alpha2.117 hides both the outer HWND and the WebView2 controller for a
// logically hidden window. Persistent media players return to their 1x1
// main-window parking child after every standalone/inline presentation. The
// controller-only fallback below is limited to transient non-music windows;
// the persistent music and radio path requires a successful anchor return.
func hideListenYouTubeMediaWindow(window *application.WebviewWindow) bool {
	if window == nil {
		return false
	}
	window.Hide()
	if parkListenMediaWebView(window) {
		return true
	}
	if window.Name() == listenPlayerWindowName ||
		window.Name() == listenLivePlayerWindowName {
		return false
	}
	zap.L().Warn(
		"media WebView parking failed; using controller-visible fallback",
		zap.String("window", window.Name()),
	)
	application.InvokeSync(func() {
		webview := listenWindowsWebViewForWindow(window)
		if webview == nil || webview.Controller() == nil {
			return
		}
		if err := webview.Controller().PutIsVisible(true); err != nil {
			zap.L().Warn(
				"media WebView controller visibility fallback failed",
				zap.String("window", window.Name()),
				zap.Error(err),
			)
		}
	})
	return true
}

func attachListenWindowsDocumentStartBridge(
	window *application.WebviewWindow,
	script string,
	afterNavigationCompleted func(),
) (func(), bool) {
	if window == nil || script == "" {
		return nil, false
	}

	fallbackRequired := &atomic.Bool{}
	var registration *listenWindowsDocumentStartScriptRegistration
	var registrationErr error
	nativeReady := false
	navigationFallbackInstalled := false
	application.InvokeSync(func() {
		webview := listenWindowsWebViewForWindow(window)
		if webview == nil || webview.Controller() == nil {
			return
		}
		// Only submit on WebView2's UI STA. The completion callback also needs
		// that STA, so the bounded wait happens after InvokeSync returns.
		registration, registrationErr = webview.beginDocumentStartScriptRegistration(script)
		if registrationErr != nil {
			fallbackRequired.Store(true)
		}
		// Wails' NavigationCompleted callback performs a first-paint Hide/Show
		// and then re-hides logically hidden windows. Run after that callback and
		// retain a late-injection path for older/policy-restricted WebView2 hosts.
		navigationFallbackInstalled = webview.WrapNavigationCompleted(func() {
			if window.NativeWindow() == nil {
				return
			}
			if fallbackRequired.Load() {
				// NavigationCompleted runs on WebView2's UI STA. ExecJS and the
				// parking helper synchronously dispatch to that same thread, so
				// leave the callback before invoking either one.
				go execListenYouTubeMusicJS(window, script)
			}
			if afterNavigationCompleted != nil {
				afterNavigationCompleted()
			}
		})
		if err := webview.Controller().PutIsVisible(true); err != nil {
			// This is a liveness hint, not a bridge capability. Managed WebView2
			// policies may reject it while document-start injection still works.
			zap.L().Warn(
				"media WebView2 initial visibility hint failed",
				zap.String("window", window.Name()),
				zap.Error(err),
			)
		}
		nativeReady = true
	})
	if !nativeReady {
		if registration != nil {
			registration.releaseOwner()
		}
		return nil, false
	}
	if !navigationFallbackInstalled {
		zap.L().Warn(
			"media WebView2 navigation compatibility hook unavailable",
			zap.String("window", window.Name()),
		)
	}

	if registrationErr == nil {
		if registration == nil {
			registrationErr = errors.New("WebView2 document-start registration was not created")
		} else {
			registrationErr = registration.wait(listenWindowsDocumentStartRegistrationTimeout)
		}
	}
	if registrationErr != nil {
		// Registration failure must not turn WebView2 availability into a hard
		// startup/playback gate. The post-navigation bridge is less early but is
		// compatible with managed/older runtimes and remains observable in logs.
		fallbackRequired.Store(true)
		zap.L().Warn(
			"WebView2 document-start bridge unavailable; using navigation fallback",
			zap.String("window", window.Name()),
			zap.Error(registrationErr),
		)
		if !navigationFallbackInstalled {
			// There is no verified early registration and no compatible late
			// injection point. Preserve a clear playback error instead of
			// pretending that the bridge was installed.
			if registration != nil {
				registration.releaseOwner()
			}
			return nil, false
		}
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			if registration != nil {
				registration.releaseOwner()
			}
		})
	}, true
}

func attachListenYouTubeMusicBridge(window *application.WebviewWindow, script string) (func(), bool) {
	if window == nil || script == "" {
		return nil, false
	}
	documentStartScript := script
	includesYouTubeAdBlocker := window.Name() == listenPlayerWindowName ||
		window.Name() == listenLivePlayerWindowName
	if includesYouTubeAdBlocker {
		// Register one composite script so both the bridge and ad blocker share
		// the same completion barrier before the first YouTube navigation.
		documentStartScript += "\n" + listenYouTubeAdBlockScript()
	}
	ensureCurrentLocalDocument := window.Name() == localMediaWindowName
	registrationHook, installed := attachListenWindowsDocumentStartBridge(
		window,
		documentStartScript,
		func() {
			if ensureCurrentLocalDocument {
				// Local's NavigateToString starts inside NewWithOptions, before
				// this attach point. Reinstall after that one current document
				// commits; the bridge's window guard makes this idempotent.
				go execListenYouTubeMusicJS(window, script)
			}
			go reassertListenMediaWebViewParking(window)
		},
	)
	if !installed {
		// Capability registration may already own native COM handlers by this
		// point. Release them before the caller closes the rejected window.
		releaseListenWindowsRemoteNavigationPolicy(window)
		return nil, false
	}
	if ensureCurrentLocalDocument {
		// If the local document committed before the callback wrapper was
		// installed, force Wails' runtime-ready boundary before injecting. The
		// custom NavigateToString document does not emit that message itself.
		// If navigation is still in flight, the idempotent completion injection
		// above installs the bridge into the replacement document.
		execListenYouTubeMusicJS(window, script)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if registrationHook != nil {
				registrationHook()
			}
			listenWindowsMediaVisibilityWindows.Delete(window.ID())
			releaseListenWindowsRemoteNavigationPolicy(window)
		})
	}, true
}

// WebView2's document-created hook is the only injection point early enough
// for the playback-only stylesheet and bridge to precede the canonical
// Bilibili page bootstrap. NavigationCompleted/ExecuteScript remains
// appropriate for ad-hoc transport commands, but is deliberately not used to
// install this bridge.
func attachRSSVideoPlayerDocumentStartBridge(
	window *application.WebviewWindow,
	script string,
) (func(), bool) {
	return attachListenWindowsDocumentStartBridge(window, script, func() {
		// The React surface is intentionally withheld until this hidden page
		// discovers its media element and reports controls. Wails otherwise puts
		// the controller into efficiency mode after first navigation, which can
		// stall that readiness bridge before the surface is ever eligible to show.
		if window != nil && window.NativeWindow() != nil {
			current := listenWindowsWebViewForWindow(window)
			if current == nil || current.Controller() == nil {
				return
			}
			_ = current.Controller().PutIsVisible(true)
		}
	})
}

func markListenYouTubeMusicRuntimeReady(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	if _, loaded := listenYouTubeMusicRuntimeReadyWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	window.HandleMessage("wails:runtime:ready")
}
