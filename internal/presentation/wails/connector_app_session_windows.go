//go:build windows

package wails

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/webview2/pkg/edge"
	"golang.org/x/sys/windows"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

func connectorAppSessionNativeSupported() bool {
	return true
}

func connectorAppSessionCaptureBeforeClose() bool {
	return true
}

func prepareConnectorAppSessionNativeWindow(window *application.WebviewWindow, _ string, siteKey string, records []appcookies.Record, domains []string) {
	if window == nil {
		return
	}
	application.InvokeSync(func() {
		chromium := listenWindowsChromium(window)
		if chromium == nil {
			return
		}
		configureConnectorAppSessionWindowsWebView(window, chromium, siteKey)

		manager, err := chromium.GetCookieManager()
		if err != nil || manager == nil {
			return
		}
		defer manager.Release()

		if len(records) == 0 {
			clearConnectorWindowsWebViewCookiesForDomains(manager, domains)
			return
		}
		for _, record := range records {
			addConnectorWindowsWebViewCookie(manager, record)
		}
	})
}

func configureConnectorAppSessionNativeWindow(_ unsafe.Pointer, _ string) {
}

func configureConnectorAppSessionWindowsWebView(window *application.WebviewWindow, chromium *edge.Chromium, siteKey string) {
	if window == nil || chromium == nil {
		return
	}
	userAgent := strings.TrimSpace(appSessionWebViewUserAgent(siteKey))
	if userAgent == "" {
		return
	}
	settings, err := chromium.GetSettings()
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
	handler := newConnectorWindowsGetCookiesCompletedHandler(domains, seen)
	scheduled := make(chan struct{})
	var scheduleErr error
	go func() {
		application.InvokeSync(func() {
			chromium := listenWindowsChromium(window)
			if chromium == nil {
				scheduleErr = appsessions.ErrSessionDead
				return
			}
			manager, err := chromium.GetCookieManager()
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
		close(scheduled)
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
		connectorWindowsUntrackGetCookiesHandler(handler)
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
			return uintptr(current - 1)
		}
	}
}

func connectorWindowsGetCookiesCompletedHandlerInvoke(this *connectorWindowsGetCookiesCompletedHandler, errorCode uintptr, list *edge.ICoreWebView2CookieList) uintptr {
	if this == nil {
		return uintptr(windows.E_POINTER)
	}
	defer this.once.Do(func() {
		connectorWindowsUntrackGetCookiesHandler(this)
		close(this.done)
	})
	if errorCode != 0 {
		this.err = syscall.Errno(errorCode)
		return uintptr(windows.S_OK)
	}
	if list == nil {
		return uintptr(windows.S_OK)
	}
	records, err := connectorWindowsCookieListRecords(list, this.domains, this.seen)
	this.records = append(this.records, records...)
	this.err = err
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

func clearConnectorWindowsWebViewCookiesForDomains(manager *edge.ICoreWebView2CookieManager, domains []string) {
	if manager == nil || len(domains) == 0 {
		return
	}
	seen := make(map[string]struct{})
	for _, domain := range domains {
		for _, host := range connectorWindowsCookieQueryHosts(domain) {
			for _, candidate := range connectorWindowsCookieDomainVariants(host) {
				key := strings.ToLower(strings.TrimSpace(candidate)) + "\x00/"
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				_ = manager.DeleteCookiesWithDomainAndPath(candidate, "/")
			}
		}
	}
}

func connectorWindowsCookieDomainVariants(host string) []string {
	normalized := connectorWindowsNormalizedHost(host)
	if normalized == "" {
		return nil
	}
	return []string{normalized, "." + normalized}
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
		_ = cookie.PutExpires(float64(record.Expires))
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

func saveSiteAppSessionStoredCookies(siteKey string, records []appcookies.Record) error {
	if len(records) == 0 {
		return appsessions.ErrNoCookies
	}
	data, err := appcookies.EncodeJSON(records)
	if err != nil {
		return err
	}
	protected, err := connectorWindowsProtectData([]byte(data))
	if err != nil {
		return fmt.Errorf("protect app session cookies: %w", err)
	}
	path, err := connectorWindowsAppSessionPath(siteKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, protected, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func loadSiteAppSessionStoredCookies(siteKey string) ([]appcookies.Record, error) {
	path, err := connectorWindowsAppSessionPath(siteKey)
	if err != nil {
		return nil, err
	}
	protected, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, appsessions.ErrNoCookies
	}
	if err != nil {
		return nil, err
	}
	data, err := connectorWindowsUnprotectData(protected)
	if err != nil {
		return nil, fmt.Errorf("unprotect app session cookies: %w", err)
	}
	records := appcookies.DecodeJSON(string(data))
	if len(records) == 0 {
		return nil, appsessions.ErrNoCookies
	}
	return records, nil
}

func clearSiteAppSessionStoredCookies(siteKey string, _ []string) error {
	path, err := connectorWindowsAppSessionPath(siteKey)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func connectorWindowsProtectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{
		Size: uint32(len(data)),
		Data: &data[0],
	}
	var out windows.DataBlob
	description, _ := windows.UTF16PtrFromString("XiaDown App Session")
	if err := windows.CryptProtectData(&in, description, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}

func connectorWindowsUnprotectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{
		Size: uint32(len(data)),
		Data: &data[0],
	}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}

func connectorWindowsAppSessionPath(siteKey string) (string, error) {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = configDir
	}
	return filepath.Join(base, "XiaDown", "app-sessions", connectorWindowsAppSessionFileName(siteKey)+".json.dpapi"), nil
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
