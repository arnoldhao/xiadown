package wails

import (
	"os"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestRemoteWebViewPermissionPolicyDeniesEverySupportedCapability(t *testing.T) {
	t.Parallel()

	const futureCrossPlatformPermission application.PermissionType = 255
	const futureWindowsPermission application.CoreWebView2PermissionKind = 255
	original := application.WebviewWindowOptions{
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionCamera:  application.PermissionAllow,
			futureCrossPlatformPermission: application.PermissionAllow,
		},
		Windows: application.WindowsWindow{
			Permissions: map[application.CoreWebView2PermissionKind]application.CoreWebView2PermissionState{
				application.CoreWebView2PermissionKindCamera: application.CoreWebView2PermissionStateAllow,
				futureWindowsPermission:                      application.CoreWebView2PermissionStateAllow,
			},
		},
	}

	got := withRemoteWebViewPermissionPolicy(original)
	for _, permission := range remoteWebViewPermissionTypes {
		if state := got.Permissions[permission]; state != application.PermissionDeny {
			t.Errorf("cross-platform permission %d = %d, want deny", permission, state)
		}
	}
	for _, permission := range remoteWebViewWindowsPermissionKinds {
		if state := got.Windows.Permissions[permission]; state != application.CoreWebView2PermissionStateDeny {
			t.Errorf("Windows permission %d = %d, want deny", permission, state)
		}
	}
	if got.Permissions[futureCrossPlatformPermission] != application.PermissionAllow {
		t.Fatal("helper discarded an unrelated cross-platform permission")
	}
	if got.Windows.Permissions[futureWindowsPermission] != application.CoreWebView2PermissionStateAllow {
		t.Fatal("helper discarded an unrelated Windows permission")
	}
	if original.Permissions[application.PermissionCamera] != application.PermissionAllow {
		t.Fatal("helper mutated the caller's cross-platform permission map")
	}
	if original.Windows.Permissions[application.CoreWebView2PermissionKindCamera] != application.CoreWebView2PermissionStateAllow {
		t.Fatal("helper mutated the caller's Windows permission map")
	}
}

func TestRemoteMediaWebViewPolicyKeepsScopedAutoplayAllow(t *testing.T) {
	t.Parallel()

	original := application.WebviewWindowOptions{
		Windows: application.WindowsWindow{
			Permissions: map[application.CoreWebView2PermissionKind]application.CoreWebView2PermissionState{
				remoteMediaWebViewAutoplayPermissionKind: application.CoreWebView2PermissionStateAllow,
			},
		},
	}
	got := withRemoteWebViewPermissionPolicy(original)
	if got.Windows.Permissions[remoteMediaWebViewAutoplayPermissionKind] != application.CoreWebView2PermissionStateAllow {
		t.Fatal("remote media permission policy discarded the scoped WebView2 autoplay allow")
	}
	for _, permission := range remoteWebViewWindowsPermissionKinds {
		if got.Windows.Permissions[permission] != application.CoreWebView2PermissionStateDeny {
			t.Fatalf("remote media permission %d was not denied", permission)
		}
	}
	if original.Windows.Permissions[remoteMediaWebViewAutoplayPermissionKind] != application.CoreWebView2PermissionStateAllow {
		t.Fatal("remote media permission policy mutated the caller's autoplay permission")
	}
}

func TestEveryRemoteWailsWindowAppliesPermissionPolicyBeforeCreation(t *testing.T) {
	t.Parallel()

	for name, wantCount := range map[string]int{
		"connector_app_session.go":         1,
		"connector_app_session_windows.go": 1,
		"listen_player_handler.go":         1,
		"listen_live_player_handler.go":    1,
		"rss_site_player_handler.go":       1,
		"rss_video_player_handler.go":      1,
	} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		const constructor = "NewWithOptions(withRemoteWebViewPermissionPolicy(application.WebviewWindowOptions{"
		if count := strings.Count(string(source), constructor); count != wantCount {
			t.Errorf("%s remote permission constructor count = %d, want %d", name, count, wantCount)
		}
	}

	for _, name := range []string{
		"listen_player_handler.go",
		"listen_live_player_handler.go",
		"rss_site_player_handler.go",
		"rss_video_player_handler.go",
	} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "remoteMediaWebViewAutoplayPermissionKind: application.CoreWebView2PermissionStateAllow") {
			t.Errorf("%s does not scope WebView2 autoplay permission to its media window", name)
		}
	}

	for name, required := range map[string][]string{
		"window_manager.go": {
			"NewWithOptions(withRemoteWebViewPermissionPolicy(mainWindowOptions))",
			"application.NewWindow(withRemoteWebViewPermissionPolicy(",
			"buildSettingsWindowOptions(current, false)",
			"buildTrayMiniPlayerWindowOptions(current)",
		},
		"local_media_transport.go": {
			"NewWithOptions(withRemoteWebViewPermissionPolicy(application.WebviewWindowOptions{",
		},
	} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, marker := range required {
			if !strings.Contains(string(source), marker) {
				t.Errorf("%s is missing permission policy marker %q", name, marker)
			}
		}
	}

	windowManagerSource, err := os.ReadFile("window_manager.go")
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(windowManagerSource), "application.NewWindow(withRemoteWebViewPermissionPolicy("); count != 2 {
		t.Fatalf("lazy shell permission constructor count = %d, want settings + tray", count)
	}
}

func TestConnectorAppSessionPopupReusesParentWebKitConfiguration(t *testing.T) {
	t.Parallel()

	constructorSource, err := os.ReadFile("connector_app_session.go")
	if err != nil {
		t.Fatal(err)
	}
	prepareIndex := strings.Index(string(constructorSource), "prepareConnectorAppSessionNativeWindow(")
	registerIndex := strings.Index(string(constructorSource), "registerWebViewRemoteCapabilityPolicy(window)")
	if prepareIndex < 0 || registerIndex < 0 || registerIndex <= prepareIndex {
		t.Fatal("App Session permission wrapper must be registered after its popup-capable native delegate")
	}

	source, err := os.ReadFile("connector_app_session_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"createWebViewWithConfiguration:(WKWebViewConfiguration *)configuration",
		"configuration:configuration]",
		"popupWebView.UIDelegate = self",
		"requestMediaCapturePermissionForOrigin:(WKSecurityOrigin *)origin",
		"requestGeolocationPermissionForOrigin:(WKSecurityOrigin *)origin",
		"decisionHandler(WKPermissionDecisionDeny)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("macOS App Session popup no longer reuses the parent WebKit configuration: missing %q", required)
		}
	}
}

func TestMacRemoteCapabilityDelegateDeniesAndForwards(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("webview_remote_capability_policy_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"XiaDownRemoteCapabilityUIDelegate : NSObject <WKUIDelegate>",
		"forwardedUIDelegate",
		"respondsToSelector:(SEL)selector",
		"forwardingTargetForSelector:(SEL)selector",
		"requestMediaCapturePermissionForOrigin:(WKSecurityOrigin *)origin",
		"__MAC_OS_X_VERSION_MAX_ALLOWED >= 270000",
		"requestGeolocationPermissionForOrigin:(WKSecurityOrigin *)origin",
		"decisionHandler(WKPermissionDecisionDeny)",
		"objc_setAssociatedObject",
		"events.Mac.WebViewDidStartProvisionalNavigation",
		"events.Common.WindowRuntimeReady",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("macOS remote capability policy is missing %q", required)
		}
	}
}
