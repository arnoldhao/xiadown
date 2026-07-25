package wails

import (
	"os"
	"strings"
	"testing"
)

func TestPersistentMediaWebViewCreationOptionsEnableAutoplay(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		file      string
		signature string
	}{
		{
			name:      "music",
			file:      "listen_player_handler.go",
			signature: "func (player *ListenYouTubeMusicPlayer) createWindowLocked(",
		},
		{
			name:      "live",
			file:      "listen_live_player_handler.go",
			signature: "func (player *ListenYouTubeLivePlayer) createWindowLocked(",
		},
		{
			name:      "local",
			file:      "local_media_transport.go",
			signature: "func (transport *NativeLocalMediaWebviewTransport) ensureWindowLocked(",
		},
	} {
		source := readListenMediaParkingSource(t, test.file)
		constructor := rssVideoFunctionSource(t, source, test.signature)
		if !strings.Contains(
			constructor,
			"EnableAutoplayWithoutUserAction: application.Enabled",
		) {
			t.Errorf("%s persistent WebView does not enable macOS autoplay at construction", test.name)
		}
	}
}

func TestLocalMediaWebViewAllowsWindowsAutoplay(t *testing.T) {
	t.Parallel()

	source := readListenMediaParkingSource(t, "local_media_transport.go")
	constructor := rssVideoFunctionSource(
		t,
		source,
		"func (transport *NativeLocalMediaWebviewTransport) ensureWindowLocked(",
	)
	if !strings.Contains(
		constructor,
		"remoteMediaWebViewAutoplayPermissionKind: application.CoreWebView2PermissionStateAllow",
	) {
		t.Fatal("local persistent WebView does not explicitly allow WebView2 autoplay")
	}
}

func TestDarwinPersistentMediaBridgeUsesOwnedDocumentStartScript(t *testing.T) {
	t.Parallel()

	source := readListenMediaParkingSource(t, "listen_player_webview_darwin.go")
	for _, required := range []string{
		"listenPersistentMediaDocumentStartScriptKey",
		"listenInstallPersistentMediaDocumentStartScript",
		"WKUserScriptInjectionTimeAtDocumentStart",
		"forMainFrameOnly:YES",
		`@"// xiadown-owned:persistent-media-document-start\n"`,
		"[controller addUserScript:script]",
		"for (WKUserScript *registeredScript in controller.userScripts)",
		"listenRemovePersistentMediaDocumentStartScript",
		"NSArray<WKUserScript*> *scripts = [controller.userScripts copy]",
		"[controller removeAllUserScripts]",
		"if (![script.source isEqualToString:source])",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("macOS persistent media document-start bridge is missing %q", required)
		}
	}

	attach := rssVideoFunctionSource(
		t,
		source,
		"func attachListenYouTubeMusicBridge(",
	)
	for _, required := range []string{
		"application.InvokeSync(func()",
		"C.listenInstallPersistentMediaDocumentStartScript(window.NativeWindow(), cScript)",
		"return nil, true",
		"var once sync.Once",
		"C.listenRemovePersistentMediaDocumentStartScript(window.NativeWindow())",
	} {
		if !strings.Contains(attach, required) {
			t.Errorf("macOS persistent media bridge attach is missing %q", required)
		}
	}
	if strings.Contains(attach, "return nil, window != nil && script != \"\"") {
		t.Fatal("macOS persistent media bridge still uses the old did-finish-only no-op attach")
	}

	for _, test := range []struct {
		file      string
		signature string
	}{
		{
			file:      "listen_player_handler.go",
			signature: "func (player *ListenYouTubeMusicPlayer) createWindowLocked(",
		},
		{
			file:      "listen_live_player_handler.go",
			signature: "func (player *ListenYouTubeLivePlayer) createWindowLocked(",
		},
		{
			file:      "local_media_transport.go",
			signature: "func (transport *NativeLocalMediaWebviewTransport) ensureWindowLocked(",
		},
	} {
		constructor := rssVideoFunctionSource(
			t,
			readListenMediaParkingSource(t, test.file),
			test.signature,
		)
		if !strings.Contains(constructor, "JS:") {
			t.Errorf("%s must retain Wails JS as the initial-document compatibility fallback", test.file)
		}
	}
}

func TestPersistentMediaWebViewsRegisterParkingBeforeFirstContentCommand(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name              string
		file              string
		constructor       string
		play              string
		firstContentToken string
	}{
		{
			name:              "music",
			file:              "listen_player_handler.go",
			constructor:       "func (player *ListenYouTubeMusicPlayer) createWindowLocked(",
			play:              "func (player *ListenYouTubeMusicPlayer) Play(",
			firstContentToken: "loadListenYouTubeMusicURL(",
		},
		{
			name:              "live",
			file:              "listen_live_player_handler.go",
			constructor:       "func (player *ListenYouTubeLivePlayer) createWindowLocked(",
			play:              "func (player *ListenYouTubeLivePlayer) Play(",
			firstContentToken: "loadListenYouTubeMusicURL(",
		},
	} {
		source := readListenMediaParkingSource(t, test.file)
		constructor := rssVideoFunctionSource(t, source, test.constructor)
		bridge := strings.Index(constructor, "attachListenYouTubeMusicBridge(")
		bridgeAccepted := strings.Index(constructor, "if !bridgeInstalled")
		register := strings.Index(constructor, "registerListenMediaWebViewParking(window,")
		firstHook := strings.Index(constructor, "window.RegisterHook(")
		if bridge < 0 || bridgeAccepted < 0 || register < 0 || firstHook < 0 ||
			bridge >= bridgeAccepted || bridgeAccepted >= register || register >= firstHook {
			t.Errorf(
				"%s parking must register after bridge installation succeeds and before window lifecycle hooks",
				test.name,
			)
		}
		if !strings.Contains(constructor, ".mainWindow") {
			t.Errorf("%s parking registration does not use the stable main-window host", test.name)
		}

		play := rssVideoFunctionSource(t, source, test.play)
		create := strings.Index(play, "createWindowLocked(request)")
		firstContent := strings.Index(play, test.firstContentToken)
		if create < 0 || firstContent < 0 || create >= firstContent {
			t.Errorf(
				"%s can navigate/dispatch player content before the parking-aware constructor completes",
				test.name,
			)
		}
	}

	localSource := readListenMediaParkingSource(t, "local_media_transport.go")
	localConstructor := rssVideoFunctionSource(
		t,
		localSource,
		"func (transport *NativeLocalMediaWebviewTransport) ensureWindowLocked(",
	)
	localRegister := strings.Index(
		localConstructor,
		"registerListenMediaWebViewParking(window,",
	)
	localBridge := strings.Index(
		localConstructor,
		"attachListenYouTubeMusicBridge(window, localMediaBridgeScript)",
	)
	localBridgeAccepted := strings.Index(localConstructor, "if !bridgeInstalled")
	publishWindow := strings.Index(localConstructor, "transport.window = window")
	if localBridge < 0 || localBridgeAccepted < 0 || localRegister < 0 || publishWindow < 0 ||
		localBridge >= localBridgeAccepted ||
		localBridgeAccepted >= localRegister ||
		localRegister >= publishWindow {
		t.Fatal("local media bridge and parking must succeed before the window becomes dispatchable")
	}
	if !strings.Contains(localConstructor, ".mainWindow") {
		t.Fatal("local media parking registration does not use the stable main-window host")
	}

	localStart := rssVideoFunctionSource(
		t,
		localSource,
		"func (transport *NativeLocalMediaWebviewTransport) Start(",
	)
	ensure := strings.Index(localStart, "transport.ensureWindowLocked()")
	firstCommand := strings.Index(localStart, "execListenYouTubeMusicJS(")
	if ensure < 0 || firstCommand < 0 || ensure >= firstCommand {
		t.Fatal("local media can dispatch its first command before the parking-aware constructor completes")
	}
}

func TestLocalMediaWebViewParkingFailureKeepsCompatibilityFallback(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		file      string
		signature string
		nextHook  string
	}{
		{
			name:      "local",
			file:      "local_media_transport.go",
			signature: "func (transport *NativeLocalMediaWebviewTransport) ensureWindowLocked(",
			nextHook:  "transport.closeHook = window.RegisterHook(",
		},
	} {
		source := readListenMediaParkingSource(t, test.file)
		constructor := rssVideoFunctionSource(t, source, test.signature)
		fallbackStart := strings.Index(constructor, "if !parkingRegistered {")
		nextHook := strings.Index(constructor, test.nextHook)
		if fallbackStart < 0 || nextHook < 0 || fallbackStart >= nextHook {
			t.Errorf("%s constructor does not expose an isolated parking fallback", test.name)
			continue
		}
		fallback := constructor[fallbackStart:nextHook]
		if !strings.Contains(fallback, "hideListenYouTubeMediaWindow(window)") {
			t.Errorf("%s parking failure does not keep the legacy hidden-window path", test.name)
		}
		for _, forbidden := range []string{"window.Close()", "return nil"} {
			if strings.Contains(fallback, forbidden) {
				t.Errorf("%s parking failure incorrectly makes WebView availability fatal through %q", test.name, forbidden)
			}
		}
	}
}

func TestPersistentMediaWebViewsReleaseParkingBeforeClose(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		file      string
		signature string
	}{
		{
			name:      "music",
			file:      "listen_player_handler.go",
			signature: "func (player *ListenYouTubeMusicPlayer) Reset() error",
		},
		{
			name:      "live",
			file:      "listen_live_player_handler.go",
			signature: "func (player *ListenYouTubeLivePlayer) Reset() error",
		},
		{
			name:      "local",
			file:      "local_media_transport.go",
			signature: "func (transport *NativeLocalMediaWebviewTransport) Close() error",
		},
	} {
		source := readListenMediaParkingSource(t, test.file)
		closePath := rssVideoFunctionSource(t, source, test.signature)
		release := strings.Index(closePath, "releaseListenMediaWebViewParking(window)")
		closeWindow := strings.Index(closePath, "window.Close()")
		if release < 0 || closeWindow < 0 || release >= closeWindow {
			t.Errorf("%s must release its native parking registration before Close", test.name)
		}
		if test.name == "local" {
			closeBridge := strings.Index(closePath, "bridgeHook()")
			releasePolicy := strings.Index(closePath, "releaseWebViewRemoteCapabilityPolicy(window)")
			if closeBridge < 0 || releasePolicy < 0 ||
				closeBridge >= release || release >= releasePolicy || releasePolicy >= closeWindow {
				t.Error("local media must detach its bridge, parking, and remote policy before Close")
			}
		} else {
			closeBridge := strings.Index(closePath, "bridgeHook()")
			resetNavigation := strings.Index(
				closePath,
				"window.SetURL(listenYouTubeMusicBlankURL)",
			)
			if closeBridge < 0 || resetNavigation < 0 ||
				closeBridge >= resetNavigation ||
				resetNavigation >= release ||
				release >= closeWindow {
				t.Errorf(
					"%s must detach its document-start bridge before blank navigation, parking release, and Close",
					test.name,
				)
			}
		}
	}
}

func TestNativeFullscreenDetachFailureRestoresInlinePresentation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		file         string
		signature    string
		restoreToken string
	}{
		{
			name:         "music",
			file:         "listen_player_handler.go",
			signature:    "func (player *ListenYouTubeMusicPlayer) requestEmbeddedVideoNativeWindowFullscreenLocked(",
			restoreToken: "player.showEmbeddedVideoWindow(window, embeddedRect)",
		},
		{
			name:         "live",
			file:         "listen_live_player_handler.go",
			signature:    "func (player *ListenYouTubeLivePlayer) requestEmbeddedVideoNativeWindowFullscreenLocked(",
			restoreToken: "player.showEmbeddedVideoWindow(window, embeddedRect)",
		},
		{
			name:         "rss",
			file:         "rss_video_player_handler.go",
			signature:    "func (player *rssBilibiliVideoPlayer) requestNativeWindowFullscreenLocked(",
			restoreToken: "rssShowNativeEmbeddedWebViewForOwner(",
		},
	} {
		source := readListenMediaParkingSource(t, test.file)
		fullscreen := rssVideoFunctionSource(t, source, test.signature)
		detach := strings.Index(
			fullscreen,
			"if !detachListenNativeEmbeddedWebViewForFullscreen(",
		)
		restoreInline := strings.Index(fullscreen, test.restoreToken)
		returnError := strings.Index(fullscreen, "could not detach for native fullscreen")
		if detach < 0 || restoreInline < 0 || returnError < 0 ||
			detach >= restoreInline || restoreInline >= returnError {
			t.Errorf("%s fullscreen detach failure does not restore its previous inline presentation", test.name)
		}
	}

	liveSource := readListenMediaParkingSource(t, "listen_live_player_handler.go")
	liveFullscreen := rssVideoFunctionSource(
		t,
		liveSource,
		"func (player *ListenYouTubeLivePlayer) requestEmbeddedVideoNativeWindowFullscreenLocked(",
	)
	clearLiveFullscreen := strings.Index(
		liveFullscreen,
		"listenYouTubeLiveNativeWindowFullscreenModeScript(false)",
	)
	restoreLiveInline := strings.Index(
		liveFullscreen,
		"player.showEmbeddedVideoWindow(window, embeddedRect)",
	)
	if clearLiveFullscreen < 0 || restoreLiveInline < 0 ||
		clearLiveFullscreen >= restoreLiveInline {
		t.Fatal("Live detach recovery does not clear native-fullscreen DOM state before restoring inline")
	}

	rssSource := readListenMediaParkingSource(t, "rss_video_player_handler.go")
	rssFullscreen := rssVideoFunctionSource(
		t,
		rssSource,
		"func (player *rssBilibiliVideoPlayer) requestNativeWindowFullscreenLocked(",
	)
	rssRestore := strings.Index(rssFullscreen, "rssShowNativeEmbeddedWebViewForOwner(")
	if rssRestore < 0 {
		t.Fatal("RSS detach recovery does not restore inline")
	}
	rssRestorePath := rssFullscreen[:rssRestore]
	if strings.Contains(rssRestorePath, "listenClaimEmbeddedVideoOwner(window)") {
		t.Fatal("RSS detach recovery must not steal aperture ownership from a newer player")
	}
	if !strings.Contains(rssRestorePath, "listenEmbeddedVideoOwnerID(window)") {
		t.Fatal("RSS detach recovery does not reuse its original aperture owner ID")
	}
}

func TestRSSPlayersDoNotJoinPersistentMediaParking(t *testing.T) {
	t.Parallel()

	parkingAPIs := []string{
		"registerListenMediaWebViewParking(",
		"parkListenMediaWebView(",
		"unparkListenMediaWebView(",
		"reassertListenMediaWebViewParking(",
		"releaseListenMediaWebViewParking(",
	}
	for _, file := range []string{
		"rss_video_player_handler.go",
		"rss_site_player_handler.go",
	} {
		source := readListenMediaParkingSource(t, file)
		for _, parkingAPI := range parkingAPIs {
			if strings.Contains(source, parkingAPI) {
				t.Errorf("%s must keep its transient RSS lifecycle instead of using %s", file, parkingAPI)
			}
		}
	}
}

func TestDarwinPersistentMediaParkingPlatformContract(t *testing.T) {
	t.Parallel()

	source := readListenMediaParkingSource(t, "listen_player_webview_darwin.go")
	for _, signature := range []string{
		"func registerListenMediaWebViewParking(",
		"func parkListenMediaWebView(",
		"func unparkListenMediaWebView(",
		"func reassertListenMediaWebViewParking(",
		"func releaseListenMediaWebViewParking(",
	} {
		body := rssVideoFunctionSource(t, source, signature)
		if !strings.Contains(body, "application.InvokeSync(") {
			t.Errorf("Darwin parking API %s does not marshal AppKit work to the UI thread", signature)
		}
	}
	darwinUnpark := rssVideoFunctionSource(
		t,
		source,
		"func unparkListenMediaWebView(",
	)
	if !strings.Contains(darwinUnpark, "return unparked > 0") {
		t.Fatal("Darwin unpark does not require a registered anchor transition")
	}

	mediaHide := rssVideoFunctionSource(
		t,
		source,
		"func hideListenYouTubeMediaWindow(",
	)
	if !strings.Contains(mediaHide, "parkListenMediaWebView(window)") {
		t.Fatal("Darwin standalone media hide does not return the persistent WebView to parking")
	}
	hideWindow := strings.Index(mediaHide, "window.Hide()")
	parkWindow := strings.Index(mediaHide, "parkListenMediaWebView(window)")
	if hideWindow < 0 || parkWindow < 0 || hideWindow >= parkWindow {
		t.Fatal("Darwin standalone media hide must hide the player window before reattaching parking")
	}

	hide := rssVideoFunctionSource(
		t,
		source,
		"func hideListenNativeEmbeddedWebView(",
	)
	if !strings.Contains(hide, "C.listenHideEmbeddedWebView(") {
		t.Fatal("Darwin inline-video hide bypasses the native parking-aware restore")
	}
	nativeHide := listenMediaNativeFunctionSource(
		t,
		source,
		"static int listenHideEmbeddedWebView(",
	)
	if !strings.Contains(nativeHide, "listenParkMediaWebView(") {
		t.Fatal("Darwin native inline-video hide does not return a registered player to parking")
	}

	detach := rssVideoFunctionSource(
		t,
		source,
		"func detachListenNativeEmbeddedWebViewForFullscreen(",
	)
	if !strings.Contains(detach, "C.listenUnparkMediaWebView(") {
		t.Fatal("Darwin native fullscreen does not unpark the player before presentation")
	}

	for _, nativeContract := range []string{
		"listenRegisterMediaWebViewParking",
		"listenParkMediaWebView",
		"listenUnparkMediaWebView",
		"listenReleaseMediaWebViewParking",
	} {
		if !strings.Contains(source, nativeContract) {
			t.Errorf("Darwin native parking implementation is missing %q", nativeContract)
		}
	}
	nativeRegister := listenMediaNativeFunctionSource(
		t,
		source,
		"static int listenRegisterMediaWebViewParking(",
	)
	if !strings.Contains(nativeRegister, "listenParkMediaWebView(") {
		t.Fatal("Darwin parking registration does not immediately park the new player")
	}

	nativeAttach := listenMediaNativeFunctionSource(
		t,
		source,
		"static BOOL listenAttachMediaWebViewParkingContainer(",
	)
	for _, required := range []string{
		"positioned:NSWindowAbove",
		"entry->slotIndex",
		"hostZPosition + 1",
		"NSMakeRect(x, y, 1, 1)",
		"container.hidden = NO",
		"container.alphaValue = 0",
	} {
		if !strings.Contains(nativeAttach, required) {
			t.Errorf("Darwin parking container does not preserve visible, distinct host attachment through %q", required)
		}
	}
	if strings.Contains(nativeAttach, "positioned:NSWindowBelow") {
		t.Fatal("Darwin parking container must not sit below the opaque host WebView")
	}
}

func TestWindowsPersistentMediaParkingPlatformContract(t *testing.T) {
	t.Parallel()

	source := readListenMediaParkingSource(t, "listen_player_webview_windows.go")
	for _, signature := range []string{
		"func registerListenMediaWebViewParking(",
		"func parkListenMediaWebView(",
		"func unparkListenMediaWebView(",
		"func reassertListenMediaWebViewParking(",
		"func releaseListenMediaWebViewParking(",
	} {
		_ = rssVideoFunctionSource(t, source, signature)
	}

	mediaHide := rssVideoFunctionSource(
		t,
		source,
		"func hideListenYouTubeMediaWindow(",
	)
	if !strings.Contains(mediaHide, "parkListenMediaWebView(window)") {
		t.Fatal("Windows standalone media hide does not return the persistent WebView to parking")
	}
	hideWindow := strings.Index(mediaHide, "window.Hide()")
	parkWindow := strings.Index(mediaHide, "parkListenMediaWebView(window)")
	if hideWindow < 0 || parkWindow < 0 || hideWindow >= parkWindow {
		t.Fatal("Windows standalone media hide must hide the player window before reattaching parking")
	}

	register := rssVideoFunctionSource(
		t,
		source,
		"func registerListenMediaWebViewParking(",
	)
	record := strings.LastIndex(register, "listenWindowsMediaWebViewParking[windowID] =")
	initialPark := strings.LastIndex(register, "listenWindowsApplyMediaWebViewParkingLocked(")
	if record < 0 || initialPark < 0 || record >= initialPark {
		t.Fatal("Windows parking registration does not immediately park the newly recorded player")
	}
	registerFailure := strings.Index(register, "if !registered {")
	rollbackRestore := strings.Index(
		register,
		"restored := listenWindowsRestoreMediaWebViewTopLevelLocked(",
	)
	confirmedRollback := strings.Index(register, "if restored {")
	deleteBaseline := strings.LastIndex(register, "delete(listenWindowsMediaWebViewParking, windowID)")
	retainRecovery := strings.LastIndex(register, "state.parkingRequested = true")
	if registerFailure < 0 || rollbackRestore < 0 || confirmedRollback < 0 ||
		deleteBaseline < 0 || retainRecovery < 0 ||
		registerFailure >= rollbackRestore ||
		rollbackRestore >= confirmedRollback ||
		confirmedRollback >= deleteBaseline ||
		deleteBaseline >= retainRecovery {
		t.Fatal("Windows registration failure must retain its recovery baseline unless rollback succeeds")
	}

	park := rssVideoFunctionSource(t, source, "func parkListenMediaWebView(")
	if !strings.Contains(park, "listenWindowsApplyMediaWebViewParkingLocked(") {
		t.Fatal("Windows public park API bypasses the canonical parking transition")
	}
	requestParking := strings.Index(park, "state.parkingRequested = true")
	applyRequestedParking := strings.LastIndex(park, "listenWindowsApplyMediaWebViewParkingLocked(")
	if requestParking < 0 || applyRequestedParking < 0 || requestParking >= applyRequestedParking {
		t.Fatal("Windows public park API does not retain parking intent before applying it")
	}
	applyParking := rssVideoFunctionSource(
		t,
		source,
		"func listenWindowsApplyMediaWebViewParkingLocked(",
	)
	for _, required := range []string{
		"w32.EnableWindow",
		"listenWindowsSetParent",
		"w32.SetWindowPos",
		"w32.ShowWindow",
		"w32.SW_SHOWNA",
		"listenWindowsPutMediaControllerVisibility",
	} {
		if !strings.Contains(applyParking, required) {
			t.Errorf("Windows parking is missing %q", required)
		}
	}
	if !strings.Contains(applyParking, "1,\n\t\t1,") {
		t.Fatal("Windows parking does not constrain the persistent player to 1x1")
	}
	windowStyle := rssVideoFunctionSource(
		t,
		source,
		"func listenWindowsApplyEmbeddedWindowStyle(",
	)
	for _, required := range []string{
		"w32.WS_CHILD",
		"w32.WS_VISIBLE",
		"w32.WS_EX_NOACTIVATE",
	} {
		if !strings.Contains(windowStyle, required) {
			t.Errorf("Windows non-interactive parking style is missing %q", required)
		}
	}
	visibility := rssVideoFunctionSource(
		t,
		source,
		"func listenWindowsPutMediaControllerVisibility(",
	)
	for _, required := range []string{"PutIsVisible(visible)", "NotifyParentWindowPositionChanged"} {
		if !strings.Contains(visibility, required) {
			t.Errorf("Windows parking controller visibility is missing %q", required)
		}
	}
	if strings.Contains(park, "CompositionControllerReady") ||
		strings.Contains(applyParking, "CompositionControllerReady") {
		t.Fatal("Windows parking must not depend on composition-controller readiness")
	}
	applyRequest := strings.Index(applyParking, "state.parkingRequested = true")
	clearParked := strings.Index(applyParking, "state.parked = false")
	applyMutation := strings.Index(applyParking, "w32.ShowWindow(")
	if applyRequest < 0 || clearParked < 0 || applyMutation < 0 ||
		applyRequest >= clearParked || clearParked >= applyMutation {
		t.Fatal("Windows canonical parking transition does not conservatively update state before mutating the HWND")
	}

	reassert := rssVideoFunctionSource(
		t,
		source,
		"func reassertListenMediaWebViewParking(",
	)
	checkRequested := strings.Index(reassert, "if !state.parkingRequested")
	reapplyParking := strings.LastIndex(reassert, "listenWindowsApplyMediaWebViewParkingLocked(")
	if checkRequested < 0 || reapplyParking < 0 || checkRequested >= reapplyParking {
		t.Fatal("Windows navigation reassertion does not retry a requested parking transition")
	}
	for _, signature := range []string{
		"func unparkListenMediaWebView(",
		"func releaseListenMediaWebViewParking(",
	} {
		transition := rssVideoFunctionSource(t, source, signature)
		clearRequested := strings.Index(transition, "state.parkingRequested = false")
		restoreTopLevel := strings.Index(transition, "listenWindowsRestoreMediaWebViewTopLevelLocked(")
		if clearRequested < 0 || restoreTopLevel < 0 || clearRequested >= restoreTopLevel {
			t.Errorf("Windows transition %s does not clear parking retry intent before top-level restore", signature)
		}
	}

	unparkFallback := rssVideoFunctionSource(
		t,
		source,
		"func unparkListenMediaWebView(",
	)
	stateMissing := strings.Index(unparkFallback, "if state == nil {")
	if stateMissing < 0 {
		t.Fatal("Windows unpark does not reject a missing anchor registration")
	}
	restoreRegisteredTopLevel := strings.LastIndex(
		unparkFallback,
		"unparked = listenWindowsRestoreMediaWebViewTopLevelLocked(",
	)
	rollbackFailedRestore := strings.LastIndex(unparkFallback, "if !unparked {")
	retryParking := strings.LastIndex(unparkFallback, "state.parkingRequested = true")
	reapplyAfterFailure := strings.LastIndex(
		unparkFallback,
		"listenWindowsApplyMediaWebViewParkingLocked(",
	)
	if restoreRegisteredTopLevel < 0 || rollbackFailedRestore < 0 ||
		retryParking < 0 || reapplyAfterFailure < 0 ||
		restoreRegisteredTopLevel >= rollbackFailedRestore ||
		rollbackFailedRestore >= retryParking ||
		retryParking >= reapplyAfterFailure {
		t.Fatal("Windows unpark failure does not transactionally return the player to recoverable parking")
	}

	topLevelRestore := rssVideoFunctionSource(
		t,
		source,
		"func listenWindowsRestoreMediaWebViewTopLevelLocked(",
	)
	clearRestoreParked := strings.Index(topLevelRestore, "state.parked = false")
	restoreMutation := strings.Index(topLevelRestore, "w32.ShowWindow(")
	if clearRestoreParked < 0 || restoreMutation < 0 || clearRestoreParked >= restoreMutation {
		t.Fatal("Windows top-level restore does not clear stale parked state before mutating the HWND")
	}
	restoreParent := strings.Index(topLevelRestore, "listenWindowsSetParent(")
	restoreStyle := strings.Index(topLevelRestore, "w32.SetWindowLong(playerHWND, w32.GWL_STYLE")
	validateStyle := strings.Index(
		topLevelRestore,
		"uint32(w32.GetWindowLong(playerHWND, w32.GWL_STYLE)) != state.topLevelStyle",
	)
	restoreOwner := strings.Index(
		topLevelRestore,
		"w32.SetWindowLongPtr(playerHWND, w32.GWLP_HWNDPARENT, state.topLevelOwner)",
	)
	validateOwner := strings.Index(
		topLevelRestore,
		"w32.GetWindowLongPtr(playerHWND, w32.GWLP_HWNDPARENT) != state.topLevelOwner",
	)
	validateTopLevelParent := strings.Index(
		topLevelRestore,
		"listenWindowsGetAncestorParent(playerHWND) != w32.GetDesktopWindow()",
	)
	restorePosition := strings.Index(topLevelRestore, "if !w32.SetWindowPos(")
	if restoreParent < 0 || restoreStyle < 0 ||
		validateStyle < 0 || restoreOwner < 0 || validateOwner < 0 ||
		validateTopLevelParent < 0 || restorePosition < 0 ||
		restoreParent >= restoreStyle ||
		restoreStyle >= validateStyle ||
		validateStyle >= restoreOwner ||
		restoreOwner >= validateOwner ||
		validateOwner >= validateTopLevelParent ||
		validateTopLevelParent >= restorePosition {
		t.Fatal("Windows top-level restore must set parent, then validate style and owner before positioning")
	}
	if strings.Contains(
		topLevelRestore,
		"listenWindowsGetParent(playerHWND) != state.topLevelParent",
	) {
		t.Fatal("Windows top-level restore must not reject SetParent(NULL)'s desktop intermediate state")
	}

	inline := rssVideoFunctionSource(
		t,
		source,
		"func listenWindowsShowEmbeddedWebView(",
	)
	for _, required := range []string{
		"listenWindowsMediaWebViewParkingForWindowLocked(playerWindow)",
		"restoreToParking:",
	} {
		if !strings.Contains(inline, required) {
			t.Errorf("Windows inline presentation does not preserve parking ownership through %q", required)
		}
	}
	if !strings.Contains(inline, "if !w32.SetWindowPos(") {
		t.Fatal("Windows inline presentation reports success without validating SetWindowPos")
	}
	for _, signature := range []string{
		"func listenWindowsBeginEmbeddedFullscreen(",
		"func listenWindowsEndEmbeddedFullscreen(",
	} {
		fullscreenTransition := rssVideoFunctionSource(t, source, signature)
		if !strings.Contains(fullscreenTransition, "if !w32.SetWindowPos(") {
			t.Errorf("Windows transition %s reports success without validating SetWindowPos", signature)
		}
	}

	restore := rssVideoFunctionSource(
		t,
		source,
		"func listenWindowsRestoreEmbeddedWebViewLockedPreservingHost(",
	)
	for _, required := range []string{
		"if state.restoreToParking",
		"listenWindowsApplyMediaWebViewParkingLocked(",
	} {
		if !strings.Contains(restore, required) {
			t.Errorf("Windows inline-video restore is missing %q", required)
		}
	}

	detach := rssVideoFunctionSource(
		t,
		source,
		"func detachListenNativeEmbeddedWebViewForFullscreen(",
	)
	for _, required := range []string{
		"listenWindowsDetachEmbeddedWebViewToTopLevelLocked(",
		"w32.WS_CHILD",
	} {
		if !strings.Contains(detach, required) {
			t.Errorf("Windows native fullscreen detach is missing %q", required)
		}
	}
}

func readListenMediaParkingSource(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func listenMediaNativeFunctionSource(t *testing.T, source string, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("source is missing native function %q", signature)
	}
	tail := source[start+len(signature):]
	if next := strings.Index(tail, "\nstatic "); next >= 0 {
		tail = tail[:next]
	}
	return signature + tail
}
