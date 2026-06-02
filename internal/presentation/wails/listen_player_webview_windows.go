//go:build windows

package wails

import (
	"math"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubemusic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"github.com/wailsapp/wails/webview2/pkg/edge"
)

var listenYouTubeMusicRuntimeReadyWindowIDs sync.Map
var listenWindowsWebViewConfiguredWindowIDs sync.Map
var listenWindowsYouTubeAdBlockerWindowIDs sync.Map
var listenWindowsWebResourceHeaderWindowIDs sync.Map

var (
	listenWindowsUser32              = syscall.NewLazyDLL("user32.dll")
	listenWindowsGDI32               = syscall.NewLazyDLL("gdi32.dll")
	listenWindowsProcGetParent       = listenWindowsUser32.NewProc("GetParent")
	listenWindowsProcSetParent       = listenWindowsUser32.NewProc("SetParent")
	listenWindowsProcSetWindowRgn    = listenWindowsUser32.NewProc("SetWindowRgn")
	listenWindowsProcCreateRoundRgn  = listenWindowsGDI32.NewProc("CreateRoundRectRgn")
	listenWindowsEmbeddedWebViewLock sync.Mutex
	listenWindowsEmbeddedWebView     listenWindowsEmbeddedWebViewState
)

type listenWindowsEmbeddedWebViewState struct {
	active         bool
	playerHWND     w32.HWND
	hostHWND       w32.HWND
	originalParent w32.HWND
	originalOwner  uintptr
	originalStyle  uint32
	originalEx     uint32
	originalRect   w32.RECT
}

func listenYouTubeMusicUserAgent() string {
	return youtubemusic.WindowsWebViewUserAgent
}

func configureListenYouTubeMusicNativeWindow(_ unsafe.Pointer, _ string) {}

func showListenNativeAirPlayPicker(_ unsafe.Pointer, _ ListenAirPlayAnchor) bool {
	return false
}

func showListenNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer, hostNativeWindow unsafe.Pointer, rect ListenEmbeddedVideoRect) bool {
	playerHWND := listenWindowsHWND(playerNativeWindow)
	hostHWND := listenWindowsHWND(hostNativeWindow)
	if playerHWND == 0 || hostHWND == 0 || playerHWND == hostHWND {
		return false
	}

	var shown bool
	application.InvokeSync(func() {
		shown = listenWindowsShowEmbeddedWebView(playerHWND, hostHWND, rect)
	})
	return shown
}

func hideListenNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer) bool {
	playerHWND := listenWindowsHWND(playerNativeWindow)
	if playerHWND == 0 {
		return false
	}

	var hidden bool
	application.InvokeSync(func() {
		hidden = listenWindowsHideEmbeddedWebView(playerHWND)
	})
	return hidden
}

func loadListenYouTubeMusicURL(window *application.WebviewWindow, targetURL string, cookies []appcookies.Record) {
	if window == nil || targetURL == "" {
		return
	}
	prepareListenWindowsWebView(window, cookies)
	window.SetURL(targetURL)
}

func prepareListenWindowsWebView(window *application.WebviewWindow, cookies []appcookies.Record) {
	application.InvokeSync(func() {
		chromium := listenWindowsChromium(window)
		if chromium == nil {
			return
		}
		configureListenWindowsWebView(window, chromium)
		installListenWindowsYouTubeAdBlocker(window, chromium)
		installListenWindowsWebResourceHeaders(window, chromium)
		if len(cookies) == 0 {
			return
		}
		manager, err := chromium.GetCookieManager()
		if err != nil || manager == nil {
			return
		}
		defer manager.Release()

		for _, record := range cookies {
			addListenWindowsWebViewCookie(manager, record)
		}
	})
}

func configureListenWindowsWebView(window *application.WebviewWindow, chromium *edge.Chromium) {
	if window == nil || chromium == nil {
		return
	}
	if _, loaded := listenWindowsWebViewConfiguredWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	settings, err := chromium.GetSettings()
	if err != nil || settings == nil {
		listenWindowsWebViewConfiguredWindowIDs.Delete(window.ID())
		return
	}
	defer settings.Release()

	if userAgent := listenYouTubeMusicUserAgent(); userAgent != "" {
		if err := settings.PutUserAgent(userAgent); err != nil {
			listenWindowsWebViewConfiguredWindowIDs.Delete(window.ID())
		}
	}
}

func installListenWindowsYouTubeAdBlocker(window *application.WebviewWindow, chromium *edge.Chromium) {
	if window == nil || chromium == nil {
		return
	}
	if _, loaded := listenWindowsYouTubeAdBlockerWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	chromium.Init(listenYouTubeAdBlockScript())
}

func installListenWindowsWebResourceHeaders(window *application.WebviewWindow, chromium *edge.Chromium) {
	if window == nil || chromium == nil {
		return
	}
	if _, loaded := listenWindowsWebResourceHeaderWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
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
		applyListenWindowsWebResourceHeaders(request)
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

func addListenWindowsWebViewCookie(manager *edge.ICoreWebView2CookieManager, record appcookies.Record) {
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

	if addListenWindowsWebViewCookieWithDomain(manager, record, name, domain, path) {
		return
	}
	if strings.HasPrefix(domain, ".") {
		_ = addListenWindowsWebViewCookieWithDomain(manager, record, name, strings.TrimPrefix(domain, "."), path)
	}
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
		_ = cookie.PutExpires(float64(record.Expires))
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

func listenWindowsShowEmbeddedWebView(playerHWND w32.HWND, hostHWND w32.HWND, rect ListenEmbeddedVideoRect) bool {
	if !w32.IsWindow(playerHWND) || !w32.IsWindow(hostHWND) {
		return false
	}

	frame := listenWindowsEmbeddedFrame(hostHWND, rect)
	if frame.Width < 1 || frame.Height < 1 {
		return false
	}

	listenWindowsEmbeddedWebViewLock.Lock()
	defer listenWindowsEmbeddedWebViewLock.Unlock()

	if listenWindowsEmbeddedWebView.active && listenWindowsEmbeddedWebView.playerHWND != playerHWND {
		listenWindowsRestoreEmbeddedWebViewLocked()
	}

	if !listenWindowsEmbeddedWebView.active {
		listenWindowsEmbeddedWebView = listenWindowsEmbeddedWebViewState{
			active:         true,
			playerHWND:     playerHWND,
			hostHWND:       hostHWND,
			originalParent: listenWindowsGetParent(playerHWND),
			originalOwner:  w32.GetWindowLongPtr(playerHWND, w32.GWLP_HWNDPARENT),
			originalStyle:  uint32(w32.GetWindowLong(playerHWND, w32.GWL_STYLE)),
			originalEx:     uint32(w32.GetWindowLong(playerHWND, w32.GWL_EXSTYLE)),
			originalRect:   *w32.GetWindowRect(playerHWND),
		}
	} else {
		listenWindowsEmbeddedWebView.hostHWND = hostHWND
	}

	listenWindowsApplyEmbeddedWindowStyle(playerHWND, listenWindowsEmbeddedWebView.originalStyle, listenWindowsEmbeddedWebView.originalEx)
	listenWindowsSetParent(playerHWND, hostHWND)
	if listenWindowsGetParent(playerHWND) != hostHWND {
		listenWindowsRestoreEmbeddedWebViewLocked()
		return false
	}

	listenWindowsSetEmbeddedWindowRegion(playerHWND, frame.Width, frame.Height, rect.Radius*frame.Scale)
	w32.SetWindowPos(
		playerHWND,
		w32.HWND_TOP,
		frame.X,
		frame.Y,
		frame.Width,
		frame.Height,
		uint(w32.SWP_NOACTIVATE|w32.SWP_SHOWWINDOW|w32.SWP_FRAMECHANGED),
	)
	w32.ShowWindow(playerHWND, w32.SW_SHOWNA)
	w32.InvalidateRect(hostHWND, nil, false)
	w32.InvalidateRect(playerHWND, nil, false)
	return true
}

func listenWindowsHideEmbeddedWebView(playerHWND w32.HWND) bool {
	listenWindowsEmbeddedWebViewLock.Lock()
	defer listenWindowsEmbeddedWebViewLock.Unlock()

	if !listenWindowsEmbeddedWebView.active || listenWindowsEmbeddedWebView.playerHWND != playerHWND {
		return false
	}
	listenWindowsRestoreEmbeddedWebViewLocked()
	return true
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

func listenWindowsApplyEmbeddedWindowStyle(playerHWND w32.HWND, originalStyle uint32, originalEx uint32) {
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
	exStyle |= uint32(w32.WS_EX_CONTROLPARENT | w32.WS_EX_NOACTIVATE)
	w32.SetWindowLong(playerHWND, w32.GWL_EXSTYLE, exStyle)
}

func listenWindowsRestoreEmbeddedWebViewLocked() {
	state := listenWindowsEmbeddedWebView
	if !state.active {
		return
	}

	playerHWND := state.playerHWND
	if w32.IsWindow(playerHWND) {
		w32.ShowWindow(playerHWND, w32.SW_HIDE)
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
	if state.hostHWND != 0 && w32.IsWindow(state.hostHWND) {
		w32.InvalidateRect(state.hostHWND, nil, false)
	}
	listenWindowsEmbeddedWebView = listenWindowsEmbeddedWebViewState{}
}

func listenWindowsGetParent(hwnd w32.HWND) w32.HWND {
	parent, _, _ := listenWindowsProcGetParent.Call(uintptr(hwnd))
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
		processID = listenWindowsChromiumBrowserProcessID(listenWindowsChromium(window))
	})
	return processID
}

func listenWindowsChromium(window *application.WebviewWindow) *edge.Chromium {
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
}

func listenWindowsChromiumBrowserProcessID(chromium *edge.Chromium) uint32 {
	webview := listenWindowsChromiumWebView(chromium)
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

func listenWindowsChromiumWebView(chromium *edge.Chromium) *listenWindowsCoreWebView2 {
	if chromium == nil {
		return nil
	}
	chromiumValue := reflect.ValueOf(chromium)
	if chromiumValue.Kind() != reflect.Pointer || chromiumValue.IsNil() {
		return nil
	}
	chromiumStruct := chromiumValue.Elem()
	webviewField := chromiumStruct.FieldByName("webview")
	if !webviewField.IsValid() || webviewField.Kind() != reflect.Pointer || webviewField.IsNil() || !webviewField.CanAddr() {
		return nil
	}
	webviewValue := reflect.NewAt(webviewField.Type(), unsafe.Pointer(webviewField.UnsafeAddr())).Elem()
	webview, _ := webviewValue.Interface().(*edge.ICoreWebView2)
	return (*listenWindowsCoreWebView2)(unsafe.Pointer(webview))
}

func execListenYouTubeMusicJS(window *application.WebviewWindow, script string) {
	if window == nil || script == "" {
		return
	}
	markListenYouTubeMusicRuntimeReady(window)
	window.ExecJS(script)
}

func attachListenYouTubeMusicBridge(window *application.WebviewWindow, script string) func() {
	if window == nil || script == "" {
		return nil
	}

	return window.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(_ *application.WindowEvent) {
		execListenYouTubeMusicJS(window, script)
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
