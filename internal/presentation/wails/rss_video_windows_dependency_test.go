package wails

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestWindowsExternalVideoPlayersShareWailsNavigationBoundary(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	youtubeLoader := rssVideoFunctionSource(t, text, "func loadListenYouTubeMusicURL(")
	rssLoader := rssVideoFunctionSource(t, text, "func loadRSSVideoPlayerURL(")
	sharedLoader := rssVideoFunctionSource(t, text, "func loadListenWindowsExternalVideoURL(")

	if !strings.Contains(youtubeLoader, "loadListenWindowsExternalVideoURL(window, targetURL") {
		t.Fatal("YouTube player does not use the shared Wails navigation boundary")
	}
	for _, required := range []string{
		`targetURL == rssBilibiliPlayerBlankURL`,
		`rssBilibiliPlaybackIdentityFromURL(targetURL)`,
		`!rssBilibiliAllowsTopLevelNavigationForPlayback(targetURL, expectedAdapter, expectedVideoID)`,
		`loadListenWindowsExternalVideoURL(window, targetURL`,
		`prepareConnectorAppSessionNativeWindow(window, targetURL, "bilibili", cookies, []string{"bilibili.com"})`,
		`installRSSBilibiliWindowsReferer(window)`,
	} {
		if !strings.Contains(rssLoader, required) {
			t.Fatalf("Windows RSS canonical-page loader is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`rssBilibiliVideoIDFromURL(targetURL)`,
		`rssBilibiliAllowsTopLevelNavigationForVideo(targetURL, expectedVideoID)`,
	} {
		if strings.Contains(rssLoader, forbidden) {
			t.Fatalf("Windows RSS loader still uses video-only policy %q", forbidden)
		}
	}
	for _, required := range []string{
		"webViewRemoteNavigationPolicyForPlayer(window.Name(), targetURL)",
		"installListenWindowsRemoteNavigationPolicy(window, policy)",
	} {
		if !strings.Contains(sharedLoader, required) {
			t.Fatalf("shared external-video loader is missing %q", required)
		}
	}
	policy := strings.Index(sharedLoader, "installListenWindowsRemoteNavigationPolicy(window, policy)")
	prepare := strings.Index(sharedLoader, "prepare()")
	navigate := strings.Index(sharedLoader, "window.SetURL(targetURL)")
	if policy < 0 || prepare < 0 || navigate < 0 || policy >= prepare || prepare >= navigate {
		t.Fatal("shared external-video loader must install policy, prepare, then call SetURL")
	}
}

func TestWindowsRemoteWebViewsCancelEscapesAndHandleEveryPopup(t *testing.T) {
	t.Parallel()

	playerSource, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(playerSource)
	installer := rssVideoFunctionSource(t, text, "func installListenWindowsRemoteNavigationPolicy(")
	for _, required := range []string{
		"installListenWindowsPersistentPopupPolicy(window)",
		"core.addNavigationStarting(state.navigationHandler)",
		"listenWindowsRemoteNavigationPolicies.Store(windowID, state)",
	} {
		if !strings.Contains(installer, required) {
			t.Fatalf("Windows remote navigation installer is missing %q", required)
		}
	}
	popupInstaller := rssVideoFunctionSource(t, text, "func installListenWindowsPersistentPopupPolicyWhileActive(")
	for _, required := range []string{
		"window.NativeWindow() == nil",
		"core.addNewWindowRequested(state.handler)",
		"listenWindowsPersistentPopupPolicies.Store(windowID, state)",
	} {
		if !strings.Contains(popupInstaller, required) {
			t.Fatalf("Windows persistent popup installer is missing %q", required)
		}
	}
	nativeGuard := strings.Index(popupInstaller, "window.NativeWindow() == nil")
	invokeMainThread := strings.Index(popupInstaller, "application.InvokeSync(func()")
	if nativeGuard < 0 || invokeMainThread < 0 || nativeGuard >= invokeMainThread {
		t.Fatal("Windows popup policy must reject pending windows before application.InvokeSync")
	}
	popupRelease := rssVideoFunctionSource(t, text, "func releaseListenWindowsPersistentPopupPolicy(")
	cancelPending := strings.Index(popupRelease, "cancelWebViewRemoteCapabilityPolicyRegistration(window)")
	releaseNativeGuard := strings.Index(popupRelease, "window.NativeWindow() == nil")
	releaseMainThread := strings.Index(popupRelease, "application.InvokeSync(func()")
	if cancelPending < 0 || releaseNativeGuard < 0 || releaseMainThread < 0 ||
		cancelPending >= releaseNativeGuard || releaseNativeGuard >= releaseMainThread {
		t.Fatal("Windows popup release must cancel pending retries and reject pending windows before application.InvokeSync")
	}
	innerPopupRelease := popupRelease[releaseMainThread:]
	loadAndDelete := strings.Index(innerPopupRelease, "listenWindowsPersistentPopupPolicies.LoadAndDelete(window.ID())")
	innerNativeGuard := strings.Index(innerPopupRelease, "window.NativeWindow() == nil")
	removePopup := strings.Index(innerPopupRelease, "state.core.removeNewWindowRequested(state.token)")
	if loadAndDelete < 0 || innerNativeGuard < 0 || removePopup < 0 ||
		loadAndDelete >= innerNativeGuard || innerNativeGuard >= removePopup {
		t.Fatal("Windows popup release must recheck native lifetime on the UI thread before removing its COM handler")
	}
	addNavigation := strings.Index(installer, "core.addNavigationStarting(state.navigationHandler)")
	store := strings.Index(installer, "listenWindowsRemoteNavigationPolicies.Store(windowID, state)")
	if addNavigation < 0 || store < 0 || addNavigation >= store {
		t.Fatal("Windows remote policy must install NavigationStarting before becoming active")
	}

	navigationCallback := rssVideoFunctionSource(t, text, "func listenWindowsNavigationStartingEventHandlerInvoke(")
	for _, required := range []string{"args.URI()", "!this.state.allows(rawURL)", "args.Cancel()"} {
		if !strings.Contains(navigationCallback, required) {
			t.Fatalf("Windows NavigationStarting callback is missing %q", required)
		}
	}
	popupCallback := rssVideoFunctionSource(t, text, "func listenWindowsNewWindowRequestedEventHandlerInvoke(")
	if !strings.Contains(popupCallback, "args.Handle()") {
		t.Fatal("Windows NewWindowRequested callback does not synchronously set Handled")
	}

	connectorSource, err := os.ReadFile("connector_app_session_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	connectorLoader := rssVideoFunctionSource(t, string(connectorSource), "func loadConnectorAppSessionNativeURL(")
	releaseHook := strings.Index(connectorLoader, "window.RegisterHook(events.Common.WindowClosing")
	policyValidation := strings.Index(connectorLoader, "webViewRemoteNavigationPolicyForAppSession(targetURL)")
	install := strings.Index(connectorLoader, "installListenWindowsRemoteNavigationPolicy(window, policy)")
	navigate := strings.Index(connectorLoader, "window.SetURL(targetURL)")
	if releaseHook < 0 || policyValidation < 0 || install < 0 || navigate < 0 ||
		releaseHook >= policyValidation || policyValidation >= install || install >= navigate {
		t.Fatal("Windows App Session must register unconditional cleanup and install its native policy before SetURL")
	}
	clearRuntime := rssVideoFunctionSource(t, string(connectorSource), "func clearConnectorAppSessionNativeRuntimeData(")
	releasePopup := strings.Index(clearRuntime, "releaseListenWindowsPersistentPopupPolicy(window)")
	closeWindow := strings.Index(clearRuntime, "window.Close()")
	if releasePopup < 0 || closeWindow < 0 || releasePopup >= closeWindow {
		t.Fatal("Windows clear App Session must release its temporary popup handler before Close")
	}
}

func TestProductionDoesNotImportLegacyWebview2Bindings(t *testing.T) {
	t.Parallel()

	_, sourcePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(sourcePath), "..", "..", ".."))
	const legacyImport = "github.com/wailsapp/wails/webview2/pkg/webview2"
	checkFile := func(path string) error {
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if value == legacyImport {
				t.Errorf("production source %s imports panic-prone legacy WebView2 bindings", path)
			}
		}
		return nil
	}
	rootFiles, err := filepath.Glob(filepath.Join(repositoryRoot, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range rootFiles {
		if err := checkFile(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, directory := range []string{"build", "internal"} {
		root := filepath.Join(repositoryRoot, directory)
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			return checkFile(path)
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWindowsRSSBilibiliCanonicalPageKeepsDocumentCreatedBridge(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	attach := rssVideoFunctionSource(t, string(source), "func attachRSSVideoPlayerDocumentStartBridge(")
	for _, required := range []string{
		"listenWindowsWebViewForWindow(window)",
		"webview.AddScriptToExecuteOnDocumentCreated(script)",
		"webview.WrapNavigationCompleted",
		"webview.Controller().PutIsVisible(true)",
		"return nil, false",
	} {
		if !strings.Contains(attach, required) {
			t.Fatalf("Windows RSS canonical-page bridge is missing %q", required)
		}
	}
	for _, forbidden := range []string{"WebViewNavigationCompleted", "ExecuteScript", "ExecJS"} {
		if strings.Contains(attach, forbidden) {
			t.Fatalf("Windows RSS canonical-page bridge must not depend on late injection via %q", forbidden)
		}
	}
}
