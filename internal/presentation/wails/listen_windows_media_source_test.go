package wails

import (
	"os"
	"strings"
	"testing"
)

func TestListenWindowsBridgeInstallsAtDocumentStart(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	attach := rssVideoFunctionSource(t, source, "func attachListenYouTubeMusicBridge(")
	for _, required := range []string{
		"listenWindowsWebViewForWindow(window)",
		"if err := webview.AddScriptToExecuteOnDocumentCreated(script); err != nil",
		"webview.WrapNavigationCompleted",
		"webview.Controller().PutIsVisible(true)",
		"releaseListenWindowsRemoteNavigationPolicy(window)",
		"return nil, false",
	} {
		if !strings.Contains(attach, required) {
			t.Fatalf("Music/Live bridge is missing %q from its WebView2 document-start registration", required)
		}
	}
	if strings.Contains(attach, "listenWindowsChromium(") {
		t.Fatal("Music/Live bridge is not registered with WebView2 at document start")
	}
	for _, lateHook := range []string{"WebViewNavigationCompleted", "window.ExecJS"} {
		if strings.Contains(attach, lateHook) {
			t.Fatalf("Music/Live document-start installer still uses late hook %q", lateHook)
		}
	}
	backgroundHide := rssVideoFunctionSource(t, source, "func hideListenYouTubeMediaWindow(")
	for _, required := range []string{
		"window.Hide()",
		"webview.Controller().PutIsVisible(true)",
	} {
		if !strings.Contains(backgroundHide, required) {
			t.Fatalf("Windows background media hide is missing %q", required)
		}
	}
	for _, name := range []string{"listen_player_handler.go", "listen_live_player_handler.go"} {
		handlerSource, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(handlerSource), ".Hide()") {
			t.Fatalf("%s bypasses the Windows background-media visibility helper", name)
		}
		if !strings.Contains(string(handlerSource), "hideListenYouTubeMediaWindow(") {
			t.Fatalf("%s does not use the background-media visibility helper", name)
		}
	}
}

func TestListenWindowsWebViewBridgeUsesRawCOMInterfaces(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, required := range []string{
		"type listenWindowsWebViewBridge struct",
		`FieldByName("chromium")`,
		`FieldByName("webview")`,
		`FieldByName("controller")`,
		"(*edge.ICoreWebView2)(corePointer)",
		"(*edge.ICoreWebView2Controller)(controllerPointer)",
		"reflect.MakeFunc",
		"WrapWebResourceRequested",
		"WrapNavigationCompleted",
		"WrapContainsFullScreenElementChanged",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Windows WebView2 adapter is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"*edge.Chromium",
		"Interface().(*edge.Chromium)",
		"listenWindowsChromium(",
		"listenWindowsRawCoreWebView2",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Windows WebView2 adapter still depends on private concrete Chromium via %q", forbidden)
		}
	}

	for _, name := range []string{"connector_app_session_windows.go"} {
		dependent, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(dependent), "listenWindowsWebViewForWindow(window)") {
			t.Fatalf("%s does not use the raw COM WebView2 adapter", name)
		}
		if strings.Contains(string(dependent), "listenWindowsChromium(") {
			t.Fatalf("%s still uses the removed concrete Chromium bridge", name)
		}
	}
}

func TestListenWindowsEmbeddedPlayerUsesMainWebViewOverlayPlane(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	windowEntry := rssVideoFunctionSource(t, source, "func showListenNativeEmbeddedWebViewWindow(")
	if !strings.Contains(windowEntry, "listenWindowsShowEmbeddedWebViewWindow(playerWindow, hostWindow, rect, true)") {
		t.Fatal("public Windows embedded entry no longer uses the transparent composition-hosted underlay")
	}
	windowPath := rssVideoFunctionSource(t, source, "func listenWindowsShowEmbeddedWebViewWindow(")
	for _, required := range []string{
		"listenWindowsWebViewForWindow(playerWindow)",
		"listenWindowsEmbeddedHostWebViewForWindow(hostWindow)",
		"listenWindowsShowEmbeddedWebView(playerWindow, playerHWND, hostHWND, rect, hostWebView)",
		"hostWebView != nil && hostWebView.newlyCached",
	} {
		if !strings.Contains(windowPath, required) {
			t.Fatalf("window-aware embedded path is missing %q", required)
		}
	}
	show := rssVideoFunctionSource(t, source, "func listenWindowsShowEmbeddedWebView(")
	for _, required := range []string{
		"PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{})",
		"insertAfter := w32.HWND_TOP",
		"insertAfter = w32.HWND_BOTTOM",
		"w32.EnableWindow(playerHWND, listenWindowsEmbeddedWebView.interactiveOverlay)",
		"listenWindowsActivateEmbeddedMediaController(playerWindow, playerHWND)",
	} {
		if !strings.Contains(show, required) {
			t.Fatalf("Windows embedded player composition is missing %q", required)
		}
	}
	if strings.Contains(show, "if listenWindowsEmbeddedWebView.fullscreen") {
		t.Fatal("Windows fullscreen must continue applying host geometry updates")
	}
	restore := rssVideoFunctionSource(t, source, "func listenWindowsRestoreEmbeddedWebViewLocked(")
	for _, required := range []string{"listenWindowsRestoreEmbeddedWebViewLockedPreservingHost(nil, nil)"} {
		if !strings.Contains(restore, required) {
			t.Fatalf("Windows player underlay restore is missing %q", required)
		}
	}
	preserveRestore := rssVideoFunctionSource(t, source, "func listenWindowsRestoreEmbeddedWebViewLockedPreservingHost(")
	for _, required := range []string{
		"listenWindowsRestoreEmbeddedHostBackground(state)",
		"listenWindowsPutMediaControllerVisibility(state.playerWindow, playerHWND, false)",
		"w32.EnableWindow(playerHWND, state.originalEnabled)",
		"state.hostWindow == preserveWindow",
		"state.hostController == preserveController",
		"if !preserveHost",
		"listenWindowsReleaseEmbeddedHostController(state.hostWindow)",
	} {
		if !strings.Contains(preserveRestore, required) {
			t.Fatalf("Windows player host-preserving restore is missing %q", required)
		}
	}
	transition := rssVideoFunctionSource(t, source, "func listenWindowsShowEmbeddedWebView(")
	if !strings.Contains(transition, "listenWindowsRestoreEmbeddedWebViewLockedPreservingHost(hostWebView.window, hostWebView.controller)") {
		t.Fatal("switching Windows players on the same host releases the Controller2 reference before reuse")
	}
	visibility := rssVideoFunctionSource(t, source, "func listenWindowsPutMediaControllerVisibility(")
	for _, required := range []string{
		"listenWindowsHWND(window.NativeWindow()) != expectedHWND",
		"webview.Controller().PutIsVisible(visible)",
		"NotifyParentWindowPositionChanged()",
	} {
		if !strings.Contains(visibility, required) {
			t.Fatalf("Windows media controller visibility adapter is missing %q", required)
		}
	}
	interactive := rssVideoFunctionSource(t, source, "func showRSSNativeInteractiveEmbeddedWebViewWindow(")
	if !strings.Contains(interactive, "listenWindowsShowEmbeddedWebViewWindow(playerWindow, hostWindow, rect, true)") ||
		strings.Contains(interactive, "showListenNativeEmbeddedWebView(playerWindow.NativeWindow()") {
		t.Fatal("interactive RSS player bypasses the controller-aware window path")
	}
	restoreHost := rssVideoFunctionSource(t, source, "func listenWindowsRestoreEmbeddedHostBackground(")
	for _, required := range []string{
		"listenWindowsEmbeddedHostControllerIsLive(state)",
		"listenWindowsWebviewWindowDefaultBackground(state.hostWindow)",
		"PutDefaultBackgroundColor(restoreColor)",
	} {
		if !strings.Contains(restoreHost, required) {
			t.Fatalf("Windows player host-background restore is missing %q", required)
		}
	}
	defaultBackground := rssVideoFunctionSource(t, source, "func listenWindowsWebviewWindowDefaultBackground(")
	for _, required := range []string{
		"application.BackgroundTypeTransparent",
		"application.BackgroundTypeTranslucent",
		"return edge.COREWEBVIEW2_COLOR{}, true",
		"listenWindowsCoreWebViewColor(options.BackgroundColour)",
	} {
		if !strings.Contains(defaultBackground, required) {
			t.Fatalf("Windows player host-background policy is missing %q", required)
		}
	}
	themeSync := rssVideoFunctionSource(t, source, "func syncListenNativeVideoHostBackground(")
	for _, required := range []string{
		"state.hostRestoreColor = listenWindowsCoreWebViewColor(background)",
		"listenWindowsWebviewWindowDefaultBackground(window)",
		"listenWindowsEmbeddedHostControllerIsLive(*state)",
		"PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{})",
	} {
		if !strings.Contains(themeSync, required) {
			t.Fatalf("Windows theme update underlay sync is missing %q", required)
		}
	}
	windowManagerBytes, err := os.ReadFile("window_manager.go")
	if err != nil {
		t.Fatal(err)
	}
	applySettings := rssVideoFunctionSource(t, string(windowManagerBytes), "func (manager *WindowManager) ApplySettings(")
	if !strings.Contains(applySettings, "syncListenNativeVideoHostBackground(manager.mainWindow, color)") {
		t.Fatal("runtime theme changes do not preserve the active native-video underlay")
	}
	fullscreenHook := rssVideoFunctionSource(t, source, "func installListenWindowsEmbeddedFullscreen(")
	for _, required := range []string{
		"webview.WrapContainsFullScreenElementChanged",
		"listenWindowsGetContainsFullScreenElement(sender)",
		"embeddedHandled = listenWindowsBeginEmbeddedFullscreen(playerHWND)",
		"embeddedHandled = listenWindowsEndEmbeddedFullscreen(playerHWND)",
		"return embeddedHandled",
	} {
		if !strings.Contains(fullscreenHook, required) {
			t.Fatalf("Windows fullscreen overlay path is missing %q", required)
		}
	}
	fullscreenState := rssVideoFunctionSource(t, source, "func listenWindowsGetContainsFullScreenElement(")
	for _, required := range []string{
		"var result int32",
		"webview.vtbl.GetContainsFullScreenElement.Call(",
	} {
		if !strings.Contains(fullscreenState, required) {
			t.Fatalf("Windows fullscreen state adapter is missing %q", required)
		}
	}
	if strings.Contains(fullscreenHook, "sender.GetContainsFullScreenElement()") {
		t.Fatal("Windows fullscreen state must not use v1.0.28's one-byte BOOL receiver")
	}
	fullscreenWrapper := rssVideoFunctionSource(t, source, "func (bridge *listenWindowsWebViewBridge) WrapContainsFullScreenElementChanged(")
	for _, required := range []string{
		`bridge.wrapCallback("ContainsFullScreenElementChangedCallback", 2`,
		"if !handled && !next.IsNil()",
		"next.Call(arguments)",
	} {
		if !strings.Contains(fullscreenWrapper, required) {
			t.Fatalf("Windows fullscreen callback adapter is missing %q", required)
		}
	}
	beginFullscreen := rssVideoFunctionSource(t, source, "func listenWindowsBeginEmbeddedFullscreen(")
	if strings.Contains(beginFullscreen, "state.originalParent") ||
		!strings.Contains(beginFullscreen, "insertAfter := w32.HWND_TOP") ||
		!strings.Contains(beginFullscreen, "if state.hostTransparent") {
		t.Fatal("Windows app fullscreen must keep transparent-host media below the topmost DComp visual")
	}
	geometryPolicy := rssVideoFunctionSource(t, source, "func listenEmbeddedVideoFullscreenAllowsHostGeometry(")
	if !strings.Contains(geometryPolicy, "return false") {
		t.Fatal("Windows native player-window fullscreen must suspend React host geometry")
	}
	nativeFullscreenPolicy := rssVideoFunctionSource(t, source, "func listenEmbeddedVideoUsesNativeWindowFullscreen(")
	if !strings.Contains(nativeFullscreenPolicy, "return true") {
		t.Fatal("Windows video fullscreen must use a detached native player window")
	}
	detach := rssVideoFunctionSource(t, source, "func detachListenNativeEmbeddedWebViewForFullscreen(")
	for _, required := range []string{
		"hideListenNativeEmbeddedWebView(playerNativeWindow)",
		"style&uint32(w32.WS_CHILD) == 0",
	} {
		if !strings.Contains(detach, required) {
			t.Fatalf("Windows native fullscreen detach validation is missing %q", required)
		}
	}
	escape := rssVideoFunctionSource(t, source, "func installListenNativeWindowFullscreenEscape(")
	if !strings.Contains(escape, `RegisterKeyBinding("escape"`) ||
		!strings.Contains(escape, "window.UnFullscreen()") {
		t.Fatal("Windows native fullscreen must provide an Escape exit binding")
	}
	for _, required := range []string{
		"listenWindowsEmbeddedWebView.hostHWND != hostHWND",
		"listenWindowsEmbeddedWebView.hostWindow != hostWebView.window",
		"listenWindowsRestoreEmbeddedWebViewLocked()",
	} {
		if !strings.Contains(show, required) {
			t.Fatalf("Windows host recreation path is missing %q", required)
		}
	}
	hostLookup := rssVideoFunctionSource(t, source, "func listenWindowsEmbeddedHostWebViewForWindow(")
	if !strings.Contains(hostLookup, "cachedHost.window == window") ||
		!strings.Contains(hostLookup, "webview.CompositionControllerReady()") {
		t.Fatal("Windows WebView2 controller cache is not bound to the concrete host window instance")
	}
	compositionReady := rssVideoFunctionSource(t, source, "func (bridge *listenWindowsWebViewBridge) CompositionControllerReady(")
	for _, required := range []string{
		`FieldByName("CompositionControllerEnabled")`,
		`FieldByName("compositionController")`,
		`FieldByName("compositionHost")`,
	} {
		if !strings.Contains(compositionReady, required) {
			t.Fatalf("Windows composition readiness check is missing %q", required)
		}
	}
	hideEntry := rssVideoFunctionSource(t, source, "func hideListenNativeEmbeddedWebView(")
	if strings.Contains(hideEntry, "if playerHWND == 0") ||
		!strings.Contains(hideEntry, "listenWindowsHideEmbeddedWebView(playerHWND)") {
		t.Fatal("destroyed Windows player windows cannot reach orphaned-underlay cleanup")
	}
	hide := rssVideoFunctionSource(t, source, "func listenWindowsHideEmbeddedWebView(")
	if !strings.Contains(hide, "playerHWND != 0 || w32.IsWindow(listenWindowsEmbeddedWebView.playerHWND)") {
		t.Fatal("orphaned-underlay cleanup is not guarded against replacing a live player")
	}
	liveSourceBytes, err := os.ReadFile("listen_live_player_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	liveShow := rssVideoFunctionSource(t, string(liveSourceBytes), "func (player *ListenYouTubeLivePlayer) ShowEmbeddedVideo(")
	if !strings.Contains(liveShow, "fullscreenOwnsPresentation && !listenEmbeddedVideoFullscreenAllowsHostGeometry()") {
		t.Fatal("live embedded player does not suspend geometry while native fullscreen owns presentation")
	}
}

func TestListenWindowsCookieExpiryAvoidsUnsupportedARM64DoubleCall(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	helper := rssVideoFunctionSource(t, source, "func listenWindowsPutCookieExpires(")
	for _, required := range []string{
		`runtime.GOARCH == "arm64"`,
		"return cookie.PutExpires(expires)",
	} {
		if !strings.Contains(helper, required) {
			t.Fatalf("Windows cookie expiry adapter is missing %q", required)
		}
	}
	for _, name := range []string{
		"listen_player_webview_windows.go",
		"connector_app_session_windows.go",
	} {
		dependent, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		remaining := strings.ReplaceAll(string(dependent), "return cookie.PutExpires(expires)", "")
		if strings.Contains(remaining, "cookie.PutExpires(") {
			t.Fatalf("%s bypasses the ARM64-safe cookie expiry adapter", name)
		}
	}
}

func TestListenWindowsPrivateCallbackAndControllerLifetimesAreGuarded(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	bridge := rssVideoFunctionSource(t, source, "func listenWindowsWebViewForWindow(")
	if !strings.Contains(bridge, "window.NativeWindow() == nil") {
		t.Fatal("Windows private bridge can be returned after the native window is destroyed")
	}
	if !strings.Contains(bridge, `FieldByName("shuttingDown")`) {
		t.Fatal("Windows private bridge can be returned while Wails is shutting down")
	}
	callback := rssVideoFunctionSource(t, source, "func (bridge *listenWindowsWebViewBridge) wrapCallback(")
	for _, required := range []string{"recover()", "zap.L().Error", "reflect.MakeFunc"} {
		if !strings.Contains(callback, required) {
			t.Fatalf("Windows reflected callback trampoline is missing %q", required)
		}
	}
	for _, test := range []struct {
		signature  string
		validation string
	}{
		{
			signature:  "func (bridge *listenWindowsWebViewBridge) WrapWebResourceRequested(",
			validation: "listenWindowsValidCallbackPointers(arguments, 2, 0, 1)",
		},
		{
			signature:  "func (bridge *listenWindowsWebViewBridge) WrapContainsFullScreenElementChanged(",
			validation: "listenWindowsValidCallbackPointers(arguments, 2, 0)",
		},
	} {
		wrapper := rssVideoFunctionSource(t, source, test.signature)
		validation := strings.Index(wrapper, test.validation)
		chained := strings.Index(wrapper, "next.Call(arguments)")
		if validation < 0 || chained < 0 || validation >= chained {
			t.Fatalf("%s does not validate native callback pointers before chaining", test.signature)
		}
	}

	hostLookup := rssVideoFunctionSource(t, source, "func listenWindowsEmbeddedHostWebViewForWindow(")
	if !strings.Contains(hostLookup, "listenWindowsReleaseController2(cachedHost.controller)") {
		t.Fatal("replacing a Windows Controller2 cache entry does not release its owning QI reference")
	}
	restore := rssVideoFunctionSource(t, source, "func listenWindowsRestoreEmbeddedWebViewLockedPreservingHost(")
	if !strings.Contains(restore, "listenWindowsReleaseEmbeddedHostController(state.hostWindow)") {
		t.Fatal("destroying a Windows embedded host does not release its cached Controller2 reference")
	}
	release := rssVideoFunctionSource(t, source, "func listenWindowsReleaseController2(")
	if !strings.Contains(release, "(*edge.ICoreWebView2Controller)(unsafe.Pointer(controller)).Release()") {
		t.Fatal("Controller2 owning reference is not released through its own IUnknown identity")
	}
}

func TestListenDocumentStartVolumeAndEscapeContracts(t *testing.T) {
	t.Parallel()

	for name, script := range map[string]string{
		"music": listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{VideoID: "AbCdEfGh123", Volume: .37}),
		"live":  listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{VideoID: "ZyXwVuTs987", Volume: .37}),
	} {
		for _, required := range []string{
			"window.top !== window",
			"DOCUMENT_VOLUME_BOOT_KEY",
			"window.sessionStorage.setItem(DOCUMENT_VOLUME_BOOT_KEY, \"true\")",
			"patchMediaElementPlay()",
			"installVolumeGuards();",
			"installFullscreenEscape();",
			"document.exitFullscreen",
			"video.webkitExitFullscreen",
		} {
			if !strings.Contains(script, required) {
				t.Fatalf("%s bridge is missing %q", name, required)
			}
		}
		storage := strings.Index(script, "window.sessionStorage.setItem(DOCUMENT_VOLUME_BOOT_KEY")
		guards := strings.LastIndex(script, "installVolumeGuards();")
		if storage < 0 || guards < 0 || storage >= guards {
			t.Fatalf("%s bridge does not persist requested volume before installing playback guards", name)
		}
	}
}
