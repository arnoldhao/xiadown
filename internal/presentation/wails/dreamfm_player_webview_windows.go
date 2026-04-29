//go:build windows

package wails

import (
	"net/url"
	"reflect"
	"strings"
	"sync"
	"unsafe"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubemusic"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var dreamFMYouTubeMusicRuntimeReadyWindowIDs sync.Map
var dreamFMWindowsWebViewConfiguredWindowIDs sync.Map
var dreamFMWindowsWebResourceHeaderWindowIDs sync.Map

func dreamFMYouTubeMusicUserAgent() string {
	return youtubemusic.WindowsWebViewUserAgent
}

func configureDreamFMYouTubeMusicNativeWindow(_ unsafe.Pointer, _ string) {}

func showDreamFMNativeAirPlayPicker(_ unsafe.Pointer, _ DreamFMAirPlayAnchor) bool {
	return false
}

func loadDreamFMYouTubeMusicURL(window *application.WebviewWindow, targetURL string, cookies []appcookies.Record) {
	if window == nil || targetURL == "" {
		return
	}
	prepareDreamFMWindowsWebView(window, cookies)
	window.SetURL(targetURL)
}

func prepareDreamFMWindowsWebView(window *application.WebviewWindow, cookies []appcookies.Record) {
	application.InvokeSync(func() {
		chromium := dreamFMWindowsChromium(window)
		if chromium == nil {
			return
		}
		configureDreamFMWindowsWebView(window, chromium)
		installDreamFMWindowsWebResourceHeaders(window, chromium)
		if len(cookies) == 0 {
			return
		}
		manager, err := chromium.GetCookieManager()
		if err != nil || manager == nil {
			return
		}
		defer manager.Release()

		for _, record := range cookies {
			addDreamFMWindowsWebViewCookie(manager, record)
		}
	})
}

func configureDreamFMWindowsWebView(window *application.WebviewWindow, chromium *edge.Chromium) {
	if window == nil || chromium == nil {
		return
	}
	if _, loaded := dreamFMWindowsWebViewConfiguredWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	settings, err := chromium.GetSettings()
	if err != nil || settings == nil {
		dreamFMWindowsWebViewConfiguredWindowIDs.Delete(window.ID())
		return
	}
	defer settings.Release()

	if userAgent := dreamFMYouTubeMusicUserAgent(); userAgent != "" {
		if err := settings.PutUserAgent(userAgent); err != nil {
			dreamFMWindowsWebViewConfiguredWindowIDs.Delete(window.ID())
		}
	}
}

func installDreamFMWindowsWebResourceHeaders(window *application.WebviewWindow, chromium *edge.Chromium) {
	if window == nil || chromium == nil {
		return
	}
	if _, loaded := dreamFMWindowsWebResourceHeaderWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}

	next := chromium.WebResourceRequestedCallback
	chromium.WebResourceRequestedCallback = func(
		request *edge.ICoreWebView2WebResourceRequest,
		args *edge.ICoreWebView2WebResourceRequestedEventArgs,
	) {
		if next != nil {
			next(request, args)
		}
		applyDreamFMWindowsWebResourceHeaders(request)
	}
}

func applyDreamFMWindowsWebResourceHeaders(request *edge.ICoreWebView2WebResourceRequest) {
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

	if referer := dreamFMWindowsNavigationRefererForURL(rawURL); referer != "" {
		_ = headers.SetHeader("Referer", referer)
	}
	if dreamFMWindowsUsesYouTubeUserAgent(rawURL) {
		_ = headers.SetHeader("User-Agent", dreamFMYouTubeMusicUserAgent())
	}
}

func dreamFMWindowsNavigationRefererForURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch {
	case host == "music.youtube.com" || strings.HasSuffix(host, ".music.youtube.com"):
		return dreamFMYouTubeMusicOrigin + "/"
	case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com"):
		return dreamFMYouTubeClientOrigin()
	default:
		return ""
	}
}

func dreamFMWindowsUsesYouTubeUserAgent(rawURL string) bool {
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

func addDreamFMWindowsWebViewCookie(manager *edge.ICoreWebView2CookieManager, record appcookies.Record) {
	if manager == nil {
		return
	}

	name := strings.TrimSpace(record.Name)
	domain := strings.TrimSpace(record.Domain)
	path := strings.TrimSpace(record.Path)
	if name == "" || record.Value == "" || domain == "" {
		return
	}
	if path == "" {
		path = "/"
	}

	if addDreamFMWindowsWebViewCookieWithDomain(manager, record, name, domain, path) {
		return
	}
	if strings.HasPrefix(domain, ".") {
		_ = addDreamFMWindowsWebViewCookieWithDomain(manager, record, name, strings.TrimPrefix(domain, "."), path)
	}
}

func addDreamFMWindowsWebViewCookieWithDomain(
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
		_ = cookie.PutExpires(float64(record.Expires))
	}
	if sameSite, ok := dreamFMWindowsWebViewSameSite(record.SameSite); ok {
		_ = cookie.PutSameSite(sameSite)
	}
	return manager.AddOrUpdateCookie(cookie) == nil
}

func dreamFMWindowsWebViewSameSite(value string) (int32, bool) {
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

func dreamFMWindowsChromium(window *application.WebviewWindow) *edge.Chromium {
	if window == nil {
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
	if !chromiumField.IsValid() || chromiumField.Kind() != reflect.Pointer || chromiumField.IsNil() || !chromiumField.CanAddr() {
		return nil
	}

	chromiumValue := reflect.NewAt(chromiumField.Type(), unsafe.Pointer(chromiumField.UnsafeAddr())).Elem()
	chromium, _ := chromiumValue.Interface().(*edge.Chromium)
	return chromium
}

func execDreamFMYouTubeMusicJS(window *application.WebviewWindow, script string) {
	if window == nil || script == "" {
		return
	}
	markDreamFMYouTubeMusicRuntimeReady(window)
	window.ExecJS(script)
}

func attachDreamFMYouTubeMusicBridge(window *application.WebviewWindow, script string) func() {
	if window == nil || script == "" {
		return nil
	}

	return window.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(_ *application.WindowEvent) {
		execDreamFMYouTubeMusicJS(window, script)
	})
}

func markDreamFMYouTubeMusicRuntimeReady(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	if _, loaded := dreamFMYouTubeMusicRuntimeReadyWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	window.HandleMessage("wails:runtime:ready")
}
