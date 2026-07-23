package wails

import (
	"os"
	"strings"
	"testing"

	settingsdto "xiadown/internal/application/settings/dto"
)

func TestLinuxShellRemoteCapabilityPolicyUsesPublicWebKitSettings(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"webview_remote_capability_policy_linux.go",
		"webview_remote_capability_policy_linux_gtk3.go",
	} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		for _, required := range []string{
			"webkit_web_view_get_settings",
			"webkit_settings_set_enable_webrtc(settings, FALSE)",
			"webkit_settings_set_enable_media_stream(settings, FALSE)",
			"!WEBKIT_CHECK_VERSION(2, 48, 0)",
			"webkit_settings_set_enable_dns_prefetching(settings, FALSE)",
			"!WEBKIT_CHECK_VERSION(2, 50, 0)",
			"webkit_settings_set_enable_hyperlink_auditing(settings, FALSE)",
			"webkit_settings_set_javascript_can_open_windows_automatically(settings, FALSE)",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s is missing %q", name, required)
			}
		}
		for _, privateAPI := range []string{"_WK", "setValue:forKey", "performSelector", "dlsym("} {
			if strings.Contains(text, privateAPI) {
				t.Fatalf("%s contains private WebKit marker %q", name, privateAPI)
			}
		}
	}
}

func TestEveryWebViewRegistersRemoteCapabilityPolicy(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("window_manager.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"registerWebViewRemoteCapabilityPolicy(mainWindow)",
		"application.NewWindow(withRemoteWebViewPermissionPolicy(",
		"registerWebViewRemoteCapabilityPolicy(window)",
		"JavaScriptCanOpenWindowsAutomatically: application.Disabled",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("shell window policy is missing %q", required)
		}
	}
	if count := strings.Count(text, "registerWebViewRemoteCapabilityPolicy("); count != 3 {
		t.Fatalf("shell policy registration count = %d, want exactly main/settings/tray", count)
	}

	for _, name := range []string{
		"connector_app_session.go",
		"connector_app_session_windows.go",
		"local_media_transport.go",
		"listen_player_handler.go",
		"listen_live_player_handler.go",
		"rss_video_player_handler.go",
		"rss_site_player_handler.go",
	} {
		playerSource, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if count := strings.Count(string(playerSource), "registerWebViewRemoteCapabilityPolicy("); count != 1 {
			t.Fatalf("dedicated WebView %s policy registration count = %d, want 1", name, count)
		}
	}
}

func TestWindowsCapabilityPolicyInstallsPersistentPopupSink(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("webview_remote_capability_policy_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	register := rssVideoFunctionSource(
		t,
		text,
		"func registerWebViewRemoteCapabilityPolicy(window *application.WebviewWindow)",
	)
	for _, required := range []string{
		"installWindowPolicyWhenReady(",
		"events.Windows.WebViewNavigationCompleted",
		"window.OnWindowEvent(",
		"events.Common.WindowClosing",
		"window.NativeWindow() != nil",
		"installListenWindowsPersistentPopupPolicyWhileActive(",
	} {
		if !strings.Contains(register, required) {
			t.Fatalf("Windows persistent popup policy is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"RegisterHook(events.Common.WindowClosing",
		"releaseListenWindowsPersistentPopupPolicy(window)",
	} {
		if strings.Contains(register, forbidden) {
			t.Fatalf("Windows persistent popup policy must survive cancelled closes through %q", forbidden)
		}
	}
}

func TestDedicatedWebViewOwnersReleaseWindowsPolicyBeforeClose(t *testing.T) {
	t.Parallel()

	localSource, err := os.ReadFile("local_media_transport.go")
	if err != nil {
		t.Fatal(err)
	}
	localClose := rssVideoFunctionSource(
		t,
		string(localSource),
		"func (transport *NativeLocalMediaWebviewTransport) Close() error",
	)
	localRelease := strings.Index(localClose, "releaseWebViewRemoteCapabilityPolicy(window)")
	localWindowClose := strings.Index(localClose, "window.Close()")
	if localRelease < 0 || localWindowClose < 0 || localRelease >= localWindowClose {
		t.Fatal("local media window must release its Windows capability policy before Close")
	}

	rssSource, err := os.ReadFile("rss_video_player_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	rssCreate := rssVideoFunctionSource(
		t,
		string(rssSource),
		"func (player *rssBilibiliVideoPlayer) createWindow(",
	)
	failure := strings.Index(rssCreate, "if !installed {")
	if failure < 0 {
		t.Fatal("RSS video window is missing its document-start failure branch")
	}
	failureBranch := rssCreate[failure:]
	rssRelease := strings.Index(failureBranch, "releaseRSSVideoPlayerWindowFeatures(window)")
	rssWindowClose := strings.Index(failureBranch, "window.Close()")
	if rssRelease < 0 || rssWindowClose < 0 || rssRelease >= rssWindowClose {
		t.Fatal("failed RSS video window creation must release native policies before Close")
	}
}

func TestMacShellWindowsDisableAutomaticJavaScriptPopups(t *testing.T) {
	t.Parallel()
	main := macWindowOptions(settingsdto.Settings{})
	mainPopup := main.WebviewPreferences.JavaScriptCanOpenWindowsAutomatically
	if !mainPopup.IsSet() || mainPopup.Get() {
		t.Fatal("macOS main/settings shell must explicitly disable automatic JavaScript windows")
	}
	tray := trayMiniPlayerMacWindowOptions(settingsdto.Settings{})
	trayPopup := tray.WebviewPreferences.JavaScriptCanOpenWindowsAutomatically
	if !trayPopup.IsSet() || trayPopup.Get() {
		t.Fatal("macOS tray shell must explicitly disable automatic JavaScript windows")
	}
}
