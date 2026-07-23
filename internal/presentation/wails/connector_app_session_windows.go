//go:build windows

package wails

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/webview2/pkg/edge"
	"golang.org/x/sys/windows"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

func connectorAppSessionNativeSupported() bool {
	return true
}

func loadNativeYouTubeRuntimeCookies() ([]appcookies.Record, error) {
	return nil, appsessions.ErrUnsupported
}

func connectorAppSessionCaptureBeforeClose() bool {
	return true
}

func clearConnectorAppSessionNativeRuntimeData(ctx context.Context, app *application.App, siteKey string, domains []string) error {
	if app == nil {
		return appsessions.ErrUnsupported
	}
	if len(domains) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	clearCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	window := app.Window.NewWithOptions(withRemoteWebViewPermissionPolicy(application.WebviewWindowOptions{
		Name:          fmt.Sprintf("site-app-session-clear-%s-%d", connectorWindowsAppSessionFileName(siteKey), time.Now().UnixNano()),
		Title:         "Clear App Session",
		Width:         320,
		Height:        240,
		Hidden:        true,
		URL:           connectorAppSessionBlankURL,
		DisableResize: true,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
	}))
	registerWebViewRemoteCapabilityPolicy(window)
	if window == nil {
		return appsessions.ErrUnsupported
	}
	defer func() {
		releaseListenWindowsPersistentPopupPolicy(window)
		window.Close()
	}()

	if err := connectorWindowsWaitForCookieManager(clearCtx, window); err != nil {
		return err
	}
	if err := clearConnectorWindowsWebViewCookiesForDomains(clearCtx, window, domains); err != nil &&
		!errors.Is(err, appsessions.ErrNoCookies) {
		return err
	}
	return clearConnectorWindowsWebViewStorageForDomains(clearCtx, window, domains)
}

func prepareConnectorAppSessionNativeWindow(window *application.WebviewWindow, _ string, siteKey string, records []appcookies.Record, domains []string) {
	if window == nil {
		return
	}
	application.InvokeSync(func() {
		webview := listenWindowsWebViewForWindow(window)
		if webview == nil {
			return
		}
		configureConnectorAppSessionWindowsWebView(window, webview, siteKey)

		manager, err := webview.GetCookieManager()
		if err != nil || manager == nil {
			return
		}
		defer manager.Release()

		if len(records) == 0 {
			return
		}
		for _, record := range records {
			addConnectorWindowsWebViewCookie(manager, record)
		}
	})
}

func configureConnectorAppSessionNativeWindow(_ unsafe.Pointer, _ string) {
}

func configureConnectorAppSessionWindowsWebView(window *application.WebviewWindow, webview *listenWindowsWebViewBridge, siteKey string) {
	if window == nil || webview == nil {
		return
	}
	userAgent := strings.TrimSpace(appSessionWebViewUserAgent(siteKey))
	if userAgent == "" {
		return
	}
	settings, err := webview.GetSettings()
	if err != nil || settings == nil {
		return
	}
	defer settings.Release()
	_ = settings.PutUserAgent(userAgent)
}

func loadConnectorAppSessionNativeURL(window *application.WebviewWindow, targetURL string) {
	if window == nil || targetURL == "" {
		return
	}
	windowName := strings.TrimSpace(window.Name())
	if !strings.HasPrefix(windowName, "site-app-session-") ||
		strings.HasPrefix(windowName, "site-app-session-clear-") {
		return
	}
	// StartAppSession registers its cookie-capture hook before calling this
	// loader. Wails stops hook dispatch when that first close is cancelled, so
	// this cleanup runs only on the second, actual close. Register it before
	// validating/installing the URL policy so a rejected URL or partial native
	// installation still releases the persistent popup sink.
	window.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		releaseListenWindowsRemoteNavigationPolicy(window)
	})
	policy, allowed := webViewRemoteNavigationPolicyForAppSession(targetURL)
	if !allowed || !installListenWindowsRemoteNavigationPolicy(window, policy) {
		return
	}
	window.SetURL(targetURL)
}

func setConnectorAppSessionNativeCookies(_ unsafe.Pointer, _ string, _ []appcookies.Record) {
}

func readConnectorAppSessionNativeCookies(_ context.Context, _ unsafe.Pointer) ([]appcookies.Record, error) {
	return nil, appsessions.ErrUnsupported
}

func readConnectorAppSessionNativeWindowCookies(ctx context.Context, window *application.WebviewWindow, domains []string) ([]appcookies.Record, error) {
	if window == nil {
		return nil, appsessions.ErrSessionDead
	}
	records, err := readConnectorWindowsWebViewCookies(ctx, window, domains)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, appsessions.ErrNoCookies
	}
	return records, nil
}

func readConnectorWindowsWebViewCookies(ctx context.Context, window *application.WebviewWindow, domains []string) ([]appcookies.Record, error) {
	uris := append([]string{""}, connectorWindowsCookieQueryURIs(domains)...)
	seen := make(map[string]struct{})
	records := make([]appcookies.Record, 0)
	for _, uri := range uris {
		items, err := readConnectorWindowsWebViewCookiesForURI(ctx, window, uri, domains, seen)
		if err != nil {
			if errors.Is(err, appsessions.ErrSessionDead) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if len(records) > 0 {
					return records, nil
				}
				return nil, err
			}
			continue
		}
		records = append(records, items...)
		if uri == "" && len(records) > 0 {
			return records, nil
		}
	}
	return records, nil
}

func readConnectorWindowsWebViewCookiesForURI(ctx context.Context, window *application.WebviewWindow, uri string, domains []string, seen map[string]struct{}) ([]appcookies.Record, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	handler := newConnectorWindowsGetCookiesCompletedHandler(domains, seen)
	scheduled := make(chan struct{})
	var scheduleErr error
	go func() {
		defer close(scheduled)
		defer connectorWindowsGetCookiesCompletedHandlerRelease(handler)
		if err := ctx.Err(); err != nil {
			scheduleErr = err
			return
		}
		application.InvokeSync(func() {
			if err := ctx.Err(); err != nil {
				scheduleErr = err
				return
			}
			webview := listenWindowsWebViewForWindow(window)
			if webview == nil {
				scheduleErr = appsessions.ErrSessionDead
				return
			}
			manager, err := webview.GetCookieManager()
			if err != nil || manager == nil {
				if err == nil {
					err = appsessions.ErrSessionDead
				}
				scheduleErr = err
				return
			}
			defer manager.Release()
			scheduleErr = connectorWindowsGetCookies(manager, uri, handler)
		})
	}()
	select {
	case <-ctx.Done():
		runtime.KeepAlive(handler)
		return nil, ctx.Err()
	case <-scheduled:
	}
	if scheduleErr != nil {
		return nil, scheduleErr
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		runtime.KeepAlive(handler)
		return nil, ctx.Err()
	case <-timer.C:
		runtime.KeepAlive(handler)
		return nil, context.DeadlineExceeded
	case <-handler.done:
		runtime.KeepAlive(handler)
		return handler.records, handler.err
	}
}

func connectorWindowsWaitForCookieManager(ctx context.Context, window *application.WebviewWindow) error {
	if window == nil {
		return appsessions.ErrSessionDead
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var ready bool
		application.InvokeSync(func() {
			ready = connectorWindowsCookieManagerReady(window)
		})
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func connectorWindowsCookieManagerReady(window *application.WebviewWindow) (ready bool) {
	defer func() {
		if recover() != nil {
			ready = false
		}
	}()
	webview := listenWindowsWebViewForWindow(window)
	if webview == nil {
		return false
	}
	manager, err := webview.GetCookieManager()
	if err != nil || manager == nil {
		return false
	}
	manager.Release()
	return true
}

func clearConnectorWindowsWebViewStorageForDomains(ctx context.Context, window *application.WebviewWindow, domains []string) error {
	origins := connectorWindowsCookieQueryOrigins(domains)
	if len(origins) == 0 {
		return nil
	}
	var clearErr error
	for _, origin := range origins {
		params, err := json.Marshal(map[string]string{
			"origin":       origin,
			"storageTypes": "all",
		})
		if err != nil {
			return err
		}
		if _, err := connectorWindowsCallDevToolsProtocolMethod(ctx, window, "Storage.clearDataForOrigin", string(params)); err != nil {
			clearErr = err
		}
	}
	return clearErr
}

func connectorWindowsCallDevToolsProtocolMethod(ctx context.Context, window *application.WebviewWindow, method string, params string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	handler := newConnectorWindowsDevToolsCompletedHandler()
	scheduled := make(chan struct{})
	var scheduleErr error
	go func() {
		defer close(scheduled)
		defer connectorWindowsDevToolsCompletedHandlerRelease(handler)
		if err := ctx.Err(); err != nil {
			scheduleErr = err
			return
		}
		application.InvokeSync(func() {
			if err := ctx.Err(); err != nil {
				scheduleErr = err
				return
			}
			webview := listenWindowsWebViewForWindow(window)
			if webview == nil {
				scheduleErr = appsessions.ErrSessionDead
				return
			}
			core := listenWindowsWebViewCore(webview)
			if core == nil {
				scheduleErr = appsessions.ErrSessionDead
				return
			}
			scheduleErr = connectorWindowsCallDevTools(core, method, params, handler)
		})
	}()
	select {
	case <-ctx.Done():
		runtime.KeepAlive(handler)
		return "", ctx.Err()
	case <-scheduled:
	}
	if scheduleErr != nil {
		return "", scheduleErr
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		runtime.KeepAlive(handler)
		return "", ctx.Err()
	case <-timer.C:
		runtime.KeepAlive(handler)
		return "", context.DeadlineExceeded
	case <-handler.done:
		runtime.KeepAlive(handler)
		return handler.result, handler.err
	}
}

type connectorWindowsCookieManagerVtbl struct {
	QueryInterface                 edge.ComProc
	AddRef                         edge.ComProc
	Release                        edge.ComProc
	CreateCookie                   edge.ComProc
	CopyCookie                     edge.ComProc
	GetCookies                     edge.ComProc
	AddOrUpdateCookie              edge.ComProc
	DeleteCookie                   edge.ComProc
	DeleteCookies                  edge.ComProc
	DeleteCookiesWithDomainAndPath edge.ComProc
	DeleteAllCookies               edge.ComProc
}

type connectorWindowsCookieManager struct {
	vtbl *connectorWindowsCookieManagerVtbl
}

func connectorWindowsCallDevTools(webview *listenWindowsCoreWebView2, method string, params string, handler *connectorWindowsDevToolsCompletedHandler) error {
	if webview == nil || webview.vtbl == nil || handler == nil {
		return appsessions.ErrSessionDead
	}
	methodUTF16, err := windows.UTF16PtrFromString(method)
	if err != nil {
		return err
	}
	paramsUTF16, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return err
	}
	connectorWindowsTrackDevToolsHandler(handler)
	hr, _, _ := webview.vtbl.CallDevToolsProtocolMethod.Call(
		uintptr(unsafe.Pointer(webview)),
		uintptr(unsafe.Pointer(methodUTF16)),
		uintptr(unsafe.Pointer(paramsUTF16)),
		uintptr(unsafe.Pointer(handler)),
	)
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

type connectorWindowsDevToolsCompletedHandlerVtbl struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
	Invoke         edge.ComProc
}

type connectorWindowsDevToolsCompletedHandler struct {
	vtbl   *connectorWindowsDevToolsCompletedHandlerVtbl
	refs   atomic.Uint32
	once   sync.Once
	done   chan struct{}
	result string
	err    error
}

var connectorWindowsDevToolsCompletedHandlerFn = connectorWindowsDevToolsCompletedHandlerVtbl{
	QueryInterface: edge.NewComProc(connectorWindowsDevToolsCompletedHandlerQueryInterface),
	AddRef:         edge.NewComProc(connectorWindowsDevToolsCompletedHandlerAddRef),
	Release:        edge.NewComProc(connectorWindowsDevToolsCompletedHandlerRelease),
	Invoke:         edge.NewComProc(connectorWindowsDevToolsCompletedHandlerInvoke),
}

var connectorWindowsDevToolsPending sync.Map

func newConnectorWindowsDevToolsCompletedHandler() *connectorWindowsDevToolsCompletedHandler {
	handler := &connectorWindowsDevToolsCompletedHandler{
		vtbl: &connectorWindowsDevToolsCompletedHandlerFn,
		done: make(chan struct{}),
	}
	handler.refs.Store(1)
	return handler
}

func connectorWindowsTrackDevToolsHandler(handler *connectorWindowsDevToolsCompletedHandler) {
	if handler == nil {
		return
	}
	connectorWindowsDevToolsPending.Store(uintptr(unsafe.Pointer(handler)), handler)
}

func connectorWindowsUntrackDevToolsHandler(handler *connectorWindowsDevToolsCompletedHandler) {
	if handler == nil {
		return
	}
	connectorWindowsDevToolsPending.Delete(uintptr(unsafe.Pointer(handler)))
}

func connectorWindowsDevToolsCompletedHandlerQueryInterface(this *connectorWindowsDevToolsCompletedHandler, _ uintptr, object uintptr) uintptr {
	if this == nil || object == 0 {
		return uintptr(windows.E_POINTER)
	}
	*(*uintptr)(unsafe.Pointer(object)) = uintptr(unsafe.Pointer(this))
	connectorWindowsDevToolsCompletedHandlerAddRef(this)
	return uintptr(windows.S_OK)
}

func connectorWindowsDevToolsCompletedHandlerAddRef(this *connectorWindowsDevToolsCompletedHandler) uintptr {
	if this == nil {
		return 0
	}
	return uintptr(this.refs.Add(1))
}

func connectorWindowsDevToolsCompletedHandlerRelease(this *connectorWindowsDevToolsCompletedHandler) uintptr {
	if this == nil {
		return 0
	}
	for {
		current := this.refs.Load()
		if current == 0 {
			return 0
		}
		if this.refs.CompareAndSwap(current, current-1) {
			remaining := current - 1
			if remaining == 0 {
				connectorWindowsUntrackDevToolsHandler(this)
			}
			return uintptr(remaining)
		}
	}
}

func connectorWindowsDevToolsCompletedHandlerInvoke(this *connectorWindowsDevToolsCompletedHandler, errorCode uintptr, result *uint16) uintptr {
	if this == nil {
		return uintptr(windows.E_POINTER)
	}
	this.once.Do(func() {
		if errorCode != 0 {
			this.err = syscall.Errno(errorCode)
		} else if result != nil {
			this.result = windows.UTF16PtrToString(result)
		}
		close(this.done)
	})
	return uintptr(windows.S_OK)
}

func connectorWindowsGetCookies(manager *edge.ICoreWebView2CookieManager, uri string, handler *connectorWindowsGetCookiesCompletedHandler) error {
	if manager == nil || handler == nil {
		return appsessions.ErrSessionDead
	}
	uriUTF16, err := windows.UTF16PtrFromString(uri)
	if err != nil {
		return err
	}
	native := (*connectorWindowsCookieManager)(unsafe.Pointer(manager))
	connectorWindowsTrackGetCookiesHandler(handler)
	hr, _, _ := native.vtbl.GetCookies.Call(
		uintptr(unsafe.Pointer(native)),
		uintptr(unsafe.Pointer(uriUTF16)),
		uintptr(unsafe.Pointer(handler)),
	)
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

type connectorWindowsGetCookiesCompletedHandlerVtbl struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
	Invoke         edge.ComProc
}

type connectorWindowsGetCookiesCompletedHandler struct {
	vtbl    *connectorWindowsGetCookiesCompletedHandlerVtbl
	refs    atomic.Uint32
	once    sync.Once
	done    chan struct{}
	domains []string
	seen    map[string]struct{}
	records []appcookies.Record
	err     error
}

var connectorWindowsGetCookiesCompletedHandlerFn = connectorWindowsGetCookiesCompletedHandlerVtbl{
	QueryInterface: edge.NewComProc(connectorWindowsGetCookiesCompletedHandlerQueryInterface),
	AddRef:         edge.NewComProc(connectorWindowsGetCookiesCompletedHandlerAddRef),
	Release:        edge.NewComProc(connectorWindowsGetCookiesCompletedHandlerRelease),
	Invoke:         edge.NewComProc(connectorWindowsGetCookiesCompletedHandlerInvoke),
}

var connectorWindowsGetCookiesPending sync.Map

func newConnectorWindowsGetCookiesCompletedHandler(domains []string, seen map[string]struct{}) *connectorWindowsGetCookiesCompletedHandler {
	handler := &connectorWindowsGetCookiesCompletedHandler{
		vtbl:    &connectorWindowsGetCookiesCompletedHandlerFn,
		done:    make(chan struct{}),
		domains: append([]string(nil), domains...),
		seen:    seen,
	}
	handler.refs.Store(1)
	return handler
}

func connectorWindowsTrackGetCookiesHandler(handler *connectorWindowsGetCookiesCompletedHandler) {
	if handler == nil {
		return
	}
	connectorWindowsGetCookiesPending.Store(uintptr(unsafe.Pointer(handler)), handler)
}

func connectorWindowsUntrackGetCookiesHandler(handler *connectorWindowsGetCookiesCompletedHandler) {
	if handler == nil {
		return
	}
	connectorWindowsGetCookiesPending.Delete(uintptr(unsafe.Pointer(handler)))
}

func connectorWindowsGetCookiesCompletedHandlerQueryInterface(this *connectorWindowsGetCookiesCompletedHandler, _ uintptr, object uintptr) uintptr {
	if this == nil || object == 0 {
		return uintptr(windows.E_POINTER)
	}
	*(*uintptr)(unsafe.Pointer(object)) = uintptr(unsafe.Pointer(this))
	connectorWindowsGetCookiesCompletedHandlerAddRef(this)
	return uintptr(windows.S_OK)
}

func connectorWindowsGetCookiesCompletedHandlerAddRef(this *connectorWindowsGetCookiesCompletedHandler) uintptr {
	if this == nil {
		return 0
	}
	return uintptr(this.refs.Add(1))
}

func connectorWindowsGetCookiesCompletedHandlerRelease(this *connectorWindowsGetCookiesCompletedHandler) uintptr {
	if this == nil {
		return 0
	}
	for {
		current := this.refs.Load()
		if current == 0 {
			return 0
		}
		if this.refs.CompareAndSwap(current, current-1) {
			remaining := current - 1
			if remaining == 0 {
				connectorWindowsUntrackGetCookiesHandler(this)
			}
			return uintptr(remaining)
		}
	}
}

func connectorWindowsGetCookiesCompletedHandlerInvoke(this *connectorWindowsGetCookiesCompletedHandler, errorCode uintptr, list *edge.ICoreWebView2CookieList) uintptr {
	if this == nil {
		return uintptr(windows.E_POINTER)
	}
	this.once.Do(func() {
		switch {
		case errorCode != 0:
			this.err = syscall.Errno(errorCode)
		case list != nil:
			records, err := connectorWindowsCookieListRecords(list, this.domains, this.seen)
			this.records = append(this.records, records...)
			this.err = err
		}
		close(this.done)
	})
	return uintptr(windows.S_OK)
}

func connectorWindowsCookieListRecords(list *edge.ICoreWebView2CookieList, domains []string, seen map[string]struct{}) ([]appcookies.Record, error) {
	if list == nil {
		return nil, nil
	}
	count, err := list.GetCount()
	if err != nil {
		return nil, err
	}
	records := make([]appcookies.Record, 0, count)
	for index := uint32(0); index < count; index++ {
		cookie, err := list.GetItem(index)
		if err != nil || cookie == nil {
			continue
		}
		record, ok := connectorWindowsCookieRecord(cookie)
		cookie.Release()
		if !ok || !connectorWindowsRecordMatchesDomains(record, domains) {
			continue
		}
		key := strings.Join([]string{
			strings.ToLower(strings.TrimSpace(record.Domain)),
			strings.TrimSpace(record.Path),
			strings.TrimSpace(record.Name),
		}, "\x00")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		records = append(records, record)
	}
	return records, nil
}

func connectorWindowsCookieRecord(cookie *edge.ICoreWebView2Cookie) (appcookies.Record, bool) {
	name, err := cookie.GetName()
	if err != nil {
		return appcookies.Record{}, false
	}
	value, err := cookie.GetValue()
	if err != nil {
		return appcookies.Record{}, false
	}
	domain, err := cookie.GetDomain()
	if err != nil {
		return appcookies.Record{}, false
	}
	path, err := cookie.GetPath()
	if err != nil {
		return appcookies.Record{}, false
	}
	name = strings.TrimSpace(name)
	domain = strings.TrimSpace(domain)
	if name == "" || value == "" || domain == "" {
		return appcookies.Record{}, false
	}
	if strings.TrimSpace(path) == "" {
		path = "/"
	}
	record := appcookies.Record{
		Name:   name,
		Value:  value,
		Domain: domain,
		Path:   path,
	}
	if expires, err := cookie.GetExpires(); err == nil && !math.IsNaN(expires) && !math.IsInf(expires, 0) && expires > 0 {
		record.Expires = int64(expires)
	}
	if httpOnly, err := cookie.GetIsHttpOnly(); err == nil {
		record.HttpOnly = httpOnly
	}
	if secure, err := cookie.GetIsSecure(); err == nil {
		record.Secure = secure
	}
	if sameSite, err := cookie.GetSameSite(); err == nil {
		record.SameSite = connectorWindowsSameSiteString(sameSite)
	}
	return record, true
}

func addConnectorWindowsWebViewCookie(manager *edge.ICoreWebView2CookieManager, record appcookies.Record) {
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
	if addConnectorWindowsWebViewCookieWithDomain(manager, record, name, domain, path) {
		return
	}
	if strings.HasPrefix(domain, ".") {
		_ = addConnectorWindowsWebViewCookieWithDomain(manager, record, name, strings.TrimPrefix(domain, "."), path)
	}
}

func clearConnectorWindowsWebViewCookiesForDomains(ctx context.Context, window *application.WebviewWindow, domains []string) error {
	if window == nil || len(domains) == 0 {
		return nil
	}
	records, err := readConnectorWindowsWebViewCookies(ctx, window, domains)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	var clearErr error
	application.InvokeSync(func() {
		webview := listenWindowsWebViewForWindow(window)
		if webview == nil {
			clearErr = appsessions.ErrSessionDead
			return
		}
		manager, err := webview.GetCookieManager()
		if err != nil || manager == nil {
			if err == nil {
				err = appsessions.ErrSessionDead
			}
			clearErr = err
			return
		}
		defer manager.Release()

		for _, record := range records {
			if err := deleteConnectorWindowsWebViewCookie(manager, record); err != nil && clearErr == nil {
				clearErr = err
			}
		}
	})
	return clearErr
}

func deleteConnectorWindowsWebViewCookie(manager *edge.ICoreWebView2CookieManager, record appcookies.Record) error {
	if manager == nil {
		return appsessions.ErrSessionDead
	}
	name := strings.TrimSpace(record.Name)
	domain := strings.TrimSpace(record.Domain)
	path := strings.TrimSpace(record.Path)
	if name == "" || domain == "" {
		return nil
	}
	if path == "" {
		path = "/"
	}
	candidates := []string{domain}
	if strings.HasPrefix(domain, ".") {
		trimmed := strings.TrimPrefix(domain, ".")
		if trimmed != "" {
			candidates = append(candidates, trimmed)
		}
	} else {
		candidates = append(candidates, "."+domain)
	}
	var lastErr error
	deleted := false
	for _, candidate := range candidates {
		if err := connectorWindowsDeleteCookiesWithDomainAndPath(manager, name, candidate, path); err != nil {
			lastErr = err
			continue
		}
		deleted = true
	}
	if deleted {
		return nil
	}
	return lastErr
}

func connectorWindowsDeleteCookiesWithDomainAndPath(manager *edge.ICoreWebView2CookieManager, name string, domain string, path string) error {
	if manager == nil {
		return appsessions.ErrSessionDead
	}
	nameUTF16, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	domainUTF16, err := windows.UTF16PtrFromString(domain)
	if err != nil {
		return err
	}
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	native := (*connectorWindowsCookieManager)(unsafe.Pointer(manager))
	hr, _, _ := native.vtbl.DeleteCookiesWithDomainAndPath.Call(
		uintptr(unsafe.Pointer(native)),
		uintptr(unsafe.Pointer(nameUTF16)),
		uintptr(unsafe.Pointer(domainUTF16)),
		uintptr(unsafe.Pointer(pathUTF16)),
	)
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

func addConnectorWindowsWebViewCookieWithDomain(manager *edge.ICoreWebView2CookieManager, record appcookies.Record, name string, domain string, path string) bool {
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

func connectorWindowsCookieQueryURIs(domains []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(domains)*2)
	for _, domain := range domains {
		for _, host := range connectorWindowsCookieQueryHosts(domain) {
			for _, scheme := range []string{"https", "http"} {
				uri := scheme + "://" + host + "/"
				if _, ok := seen[uri]; ok {
					continue
				}
				seen[uri] = struct{}{}
				result = append(result, uri)
			}
		}
	}
	return result
}

func connectorWindowsCookieQueryOrigins(domains []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(domains)*2)
	for _, domain := range domains {
		for _, host := range connectorWindowsCookieQueryHosts(domain) {
			for _, scheme := range []string{"https", "http"} {
				origin := scheme + "://" + host
				if _, ok := seen[origin]; ok {
					continue
				}
				seen[origin] = struct{}{}
				result = append(result, origin)
			}
		}
	}
	return result
}

func connectorWindowsCookieQueryHosts(domain string) []string {
	host := connectorWindowsNormalizedHost(domain)
	if host == "" {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, 4)
	add := func(value string) {
		normalized := connectorWindowsNormalizedHost(value)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	add(host)
	if !strings.HasPrefix(host, "www.") && strings.Count(host, ".") == 1 {
		add("www." + host)
	}
	switch host {
	case "youtube.com":
		add("music.youtube.com")
		add("accounts.youtube.com")
	case "google.com":
		add("accounts.google.com")
		add("myaccount.google.com")
	case "bilibili.com":
		add("passport.bilibili.com")
		add("api.bilibili.com")
	case "tiktok.com":
		add("m.tiktok.com")
	case "xiaohongshu.com":
		add("edith.xiaohongshu.com")
		add("creator.xiaohongshu.com")
	case "facebook.com":
		add("m.facebook.com")
	case "nicovideo.jp":
		add("account.nicovideo.jp")
	}
	return result
}

func connectorWindowsRecordMatchesDomains(record appcookies.Record, domains []string) bool {
	if len(domains) == 0 {
		return true
	}
	host := connectorWindowsNormalizedHost(record.Domain)
	if host == "" {
		return false
	}
	for _, domain := range domains {
		if connectorWindowsHostMatchesSuffix(host, domain) {
			return true
		}
	}
	return false
}

func connectorWindowsHostMatchesSuffix(host string, suffix string) bool {
	normalizedHost := connectorWindowsNormalizedHost(host)
	normalizedSuffix := connectorWindowsNormalizedHost(suffix)
	if normalizedHost == "" || normalizedSuffix == "" {
		return false
	}
	return normalizedHost == normalizedSuffix || strings.HasSuffix(normalizedHost, "."+normalizedSuffix)
}

func connectorWindowsNormalizedHost(value string) string {
	trimmed := strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		if parsed, err := urlParseHost(trimmed); err == nil {
			trimmed = parsed
		}
	}
	return strings.Trim(trimmed, ".")
}

func urlParseHost(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(parsed.Hostname())), nil
}

func connectorWindowsSameSiteString(value int32) string {
	switch value {
	case 0:
		return "none"
	case 1:
		return "lax"
	case 2:
		return "strict"
	default:
		return ""
	}
}

func connectorWindowsAppSessionFileName(siteKey string) string {
	trimmed := strings.TrimSpace(siteKey)
	if trimmed == "" {
		return "default"
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, char := range trimmed {
		switch {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '-',
			char == '_',
			char == '.':
			builder.WriteRune(char)
		default:
			builder.WriteByte('_')
		}
	}
	result := strings.Trim(builder.String(), "._-")
	if result == "" {
		return "default"
	}
	return result
}
