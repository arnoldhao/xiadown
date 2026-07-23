package wails

import (
	"os"
	"strings"
	"testing"
)

func TestRSSSitePlayerMacUsesIndependentInteractiveOverlayAndScopedNavigation(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("listen_player_webview_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(source), "\r\n", "\n")
	for _, marker := range []string{
		"BOOL interactiveOverlay = interactive > 1",
		"if (!interactiveOverlay)",
		"positioned:(interactiveOverlay ? NSWindowAbove : NSWindowBelow)",
		"C.int(2)",
		"ListenRSSSiteNavigationPolicy : NSObject <WKNavigationDelegate, WKUIDelegate>",
		"listenRSSSiteAllowsTopLevelURL(navigationAction.request.URL",
		"listenLoadRSSSiteURL",
		"setCookie:cookie completionHandler",
		"NSInteger navigationGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue] + 1",
		"[webView stopLoading]",
	} {
		if !strings.Contains(text, marker) {
			t.Errorf("macOS RSS site player source is missing %q", marker)
		}
	}
}

func TestRSSSitePlayerWindowsOverlayRemainsFocusableAndSupportsFullscreen(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("listen_player_webview_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(source), "\r\n", "\n")
	for _, marker := range []string{
		"listenWindowsShowEmbeddedWebViewWindow(playerWindow, hostWindow, rect, true)",
		"listenWindowsActivateEmbeddedMediaController(playerWindow, playerHWND)",
		"PutIsVisible(visible)",
		"interactiveOverlay := rect.Interactive",
		"interactiveOverlay: interactiveOverlay",
		"w32.EnableWindow(playerHWND, listenWindowsEmbeddedWebView.interactiveOverlay)",
		"if interactiveOverlay {\n\t\texStyle &^= uint32(w32.WS_EX_NOACTIVATE)",
		"return uint(w32.SWP_NOACTIVATE | w32.SWP_SHOWWINDOW | w32.SWP_FRAMECHANGED)",
		"w32.ShowWindow(playerHWND, w32.SW_SHOWNA)",
		"insertAfter := w32.HWND_TOP",
		"if state.hostTransparent",
		"installListenWindowsEmbeddedFullscreen(window, webview)",
	} {
		if !strings.Contains(text, marker) {
			t.Errorf("Windows RSS site player source is missing %q", marker)
		}
	}
}
