//go:build windows

package proxy

import (
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	winHTTPAccessTypeNoProxy    = 1
	winHTTPAccessTypeNamedProxy = 3

	winHTTPAutoProxyAutoDetect     = 0x00000001
	winHTTPAutoProxyConfigURL      = 0x00000002
	winHTTPAutoProxyNoCacheClient  = 0x00080000
	winHTTPAutoProxyNoCacheService = 0x00100000

	winHTTPAutoDetectTypeDHCP = 0x00000001
	winHTTPAutoDetectTypeDNSA = 0x00000002

	winHTTPAutoProxyServiceError = 12178
	winHTTPAutoDetectionFailed   = 12180
	winHTTPLoginFailure          = 12015
	winHTTPBadAutoProxyScript    = 12166
	winHTTPUnableDownloadScript  = 12167

	winHTTPProxyResolveTimeoutMillis = 10_000
)

var (
	winHTTPDLL                            = windows.NewLazySystemDLL("winhttp.dll")
	winHTTPGetIEProxyConfigForCurrentUser = winHTTPDLL.NewProc("WinHttpGetIEProxyConfigForCurrentUser")
	winHTTPOpen                           = winHTTPDLL.NewProc("WinHttpOpen")
	winHTTPGetProxyForURL                 = winHTTPDLL.NewProc("WinHttpGetProxyForUrl")
	winHTTPSetTimeouts                    = winHTTPDLL.NewProc("WinHttpSetTimeouts")
	winHTTPCloseHandle                    = winHTTPDLL.NewProc("WinHttpCloseHandle")

	kernel32DLL = windows.NewLazySystemDLL("kernel32.dll")
	globalFree  = kernel32DLL.NewProc("GlobalFree")
)

// These layouts mirror WINHTTP_CURRENT_USER_IE_PROXY_CONFIG,
// WINHTTP_AUTOPROXY_OPTIONS, and WINHTTP_PROXY_INFO from winhttp.h.
type winHTTPCurrentUserIEProxyConfig struct {
	AutoDetect    int32
	AutoConfigURL *uint16
	Proxy         *uint16
	ProxyBypass   *uint16
}

type winHTTPAutoProxyOptions struct {
	Flags                 uint32
	AutoDetectFlags       uint32
	AutoConfigURL         *uint16
	Reserved              unsafe.Pointer
	ReservedFlags         uint32
	AutoLogonIfChallenged int32
}

type winHTTPProxyInfo struct {
	AccessType  uint32
	Proxy       *uint16
	ProxyBypass *uint16
}

type windowsSystemProxyResolver struct {
	automatic windowsAutomaticProxySession
}

// windowsAutomaticProxySession owns one WinHTTP session for one XiaDown
// network-policy generation. WinHTTP attaches PAC script, PAC URL, and WPAD
// discovery caches to this handle, so closing it after every destination would
// repeat discovery and downloads for every app-managed request.
type windowsAutomaticProxySession struct {
	state windowsAutomaticProxySessionState
}

type windowsProxyNativeError struct {
	operation string
	code      uint32
}

func (e *windowsProxyNativeError) Error() string {
	switch e.code {
	case winHTTPLoginFailure:
		// Deliberately do not retry with fAutoLogonIfChallenged=TRUE. Doing so
		// can send the signed-in user's NTLM/Negotiate credentials to a PAC
		// endpoint discovered through WPAD. XiaDown needs an explicit enterprise
		// policy before it is allowed to cross that credential boundary.
		return "Windows automatic proxy configuration requires integrated authentication; automatic credential forwarding is disabled"
	case winHTTPBadAutoProxyScript:
		return "Windows automatic proxy script evaluation failed"
	case winHTTPUnableDownloadScript:
		return "Windows automatic proxy script download failed"
	case winHTTPAutoProxyServiceError, winHTTPAutoDetectionFailed:
		return "Windows automatic proxy discovery found no usable configuration"
	default:
		return fmt.Sprintf("%s failed (Windows error %d)", e.operation, e.code)
	}
}

// platformSystemProxyURL resolves the current user's Windows proxy policy for
// one canonical destination origin. Callers must not reuse that result for a
// different origin or policy generation.
//
// This function resolves route selection only. It does not extract Windows
// credentials and the Go HTTP transport does not negotiate NTLM/Negotiate for
// a subsequent proxy 407 response. That enterprise-auth transport boundary is
// intentionally separate from PAC/WPAD discovery.
func platformSystemProxyURL(target *url.URL) (*url.URL, error) {
	resolver := newPlatformSystemProxyResolver()
	defer resolver.Close()
	return resolver.Resolve(target)
}

func newPlatformSystemProxyResolver() systemProxyResolver {
	return &windowsSystemProxyResolver{}
}

func (resolver *windowsSystemProxyResolver) Close() {
	if resolver != nil {
		resolver.automatic.Close()
	}
}

func (resolver *windowsSystemProxyResolver) Resolve(target *url.URL) (*url.URL, error) {
	if resolver == nil {
		return nil, errors.New("Windows proxy resolver is unavailable")
	}
	if target == nil || target.Hostname() == "" {
		return nil, errors.New("Windows proxy resolution requires an absolute URL")
	}
	target = windowsProxyTargetURL(target)

	config, err := readWindowsCurrentUserProxyConfig()
	if err != nil {
		return nil, err
	}
	defer freeWindowsCurrentUserProxyConfig(&config)

	manualProxy := windowsUTF16String(config.Proxy)
	manualBypass := windowsUTF16String(config.ProxyBypass)
	var automaticErr error
	var sessionValue uintptr
	acquiredSession := false
	resolveAutomatic := func(flags uint32, autoConfigURL *uint16) (*url.URL, error) {
		if !acquiredSession {
			var acquireErr error
			sessionValue, acquireErr = resolver.automatic.acquire()
			if acquireErr != nil {
				return nil, acquireErr
			}
			acquiredSession = true
		}
		return resolver.automatic.resolve(sessionValue, target, flags, autoConfigURL)
	}
	defer func() {
		if acquiredSession {
			resolver.automatic.release()
		}
	}()

	// Match Windows' documented system order: WPAD auto-detection, then the
	// explicit setup script, then the static proxy, and finally DIRECT. A PAC
	// decision of DIRECT is successful and must not fall through to a later
	// mechanism; only discovery/download/evaluation failure advances the chain.
	recordAutomaticError := func(resolveErr error) {
		if resolveErr != nil && automaticErr == nil && !isWindowsAutoDiscoveryMiss(resolveErr) {
			automaticErr = resolveErr
		}
	}
	if config.AutoDetect != 0 {
		resolved, resolveErr := resolveAutomatic(winHTTPAutoProxyAutoDetect, nil)
		if resolveErr == nil {
			return resolved, nil
		}
		recordAutomaticError(resolveErr)
	}

	if config.AutoConfigURL != nil {
		resolved, resolveErr := resolveAutomatic(winHTTPAutoProxyConfigURL, config.AutoConfigURL)
		if resolveErr == nil {
			return resolved, nil
		}
		recordAutomaticError(resolveErr)
	}

	if manualProxy != "" {
		return resolveWindowsNamedProxy(target, manualProxy, manualBypass)
	}
	if automaticErr != nil {
		return nil, automaticErr
	}
	return nil, nil
}

func windowsProxyTargetURL(target *url.URL) *url.URL {
	copy := *target
	copy.User = nil
	copy.Fragment = ""
	switch copy.Scheme {
	case "ws":
		copy.Scheme = "http"
	case "wss":
		copy.Scheme = "https"
	}
	return &copy
}

func readWindowsCurrentUserProxyConfig() (winHTTPCurrentUserIEProxyConfig, error) {
	var config winHTTPCurrentUserIEProxyConfig
	ok, _, callErr := winHTTPGetIEProxyConfigForCurrentUser.Call(uintptr(unsafe.Pointer(&config)))
	if ok == 0 {
		freeWindowsCurrentUserProxyConfig(&config)
		// WinHTTP documents ERROR_FILE_NOT_FOUND as "no Internet Explorer
		// proxy settings were found". For an otherwise unconfigured interactive
		// user that is an explicit DIRECT system policy, not a resolver failure.
		if errors.Is(callErr, windows.ERROR_FILE_NOT_FOUND) {
			return winHTTPCurrentUserIEProxyConfig{}, nil
		}
		return winHTTPCurrentUserIEProxyConfig{}, newWindowsProxyNativeError("read Windows current-user proxy configuration", callErr)
	}
	return config, nil
}

func (session *windowsAutomaticProxySession) acquire() (uintptr, error) {
	if session == nil {
		return 0, errors.New("Windows automatic proxy session is unavailable")
	}
	return session.state.acquire(openWindowsAutomaticProxySession)
}

func (session *windowsAutomaticProxySession) resolve(sessionValue uintptr, target *url.URL, flags uint32, autoConfigURL *uint16) (*url.URL, error) {
	if session == nil {
		return nil, errors.New("Windows automatic proxy session is unavailable")
	}
	var resolved *url.URL
	err := session.state.serializedCall(sessionValue, func(handle uintptr) error {
		var resolveErr error
		resolved, resolveErr = resolveWindowsAutomaticProxy(handle, target, flags, autoConfigURL)
		return resolveErr
	})
	return resolved, err
}

func (session *windowsAutomaticProxySession) release() {
	if session == nil {
		return
	}
	session.state.release(closeWindowsAutomaticProxySession)
}

func (session *windowsAutomaticProxySession) Close() {
	if session == nil {
		return
	}
	session.state.close(closeWindowsAutomaticProxySession)
}

func closeWindowsAutomaticProxySession(handle uintptr) {
	_, _, _ = winHTTPCloseHandle.Call(handle)
}

func openWindowsAutomaticProxySession() (uintptr, error) {
	agent, err := windows.UTF16PtrFromString("XiaDown")
	if err != nil {
		return 0, errors.New("initialize Windows proxy resolver")
	}
	sessionValue, _, callErr := winHTTPOpen.Call(
		uintptr(unsafe.Pointer(agent)),
		winHTTPAccessTypeNoProxy,
		0,
		0,
		0,
	)
	runtime.KeepAlive(agent)
	if sessionValue == 0 {
		return 0, newWindowsProxyNativeError("open Windows proxy resolver", callErr)
	}

	ok, _, callErr := winHTTPSetTimeouts.Call(
		sessionValue,
		winHTTPProxyResolveTimeoutMillis,
		winHTTPProxyResolveTimeoutMillis,
		winHTTPProxyResolveTimeoutMillis,
		winHTTPProxyResolveTimeoutMillis,
	)
	if ok == 0 {
		_, _, _ = winHTTPCloseHandle.Call(sessionValue)
		return 0, newWindowsProxyNativeError("configure Windows proxy resolver timeout", callErr)
	}
	return sessionValue, nil
}

func resolveWindowsAutomaticProxy(sessionValue uintptr, target *url.URL, flags uint32, autoConfigURL *uint16) (*url.URL, error) {
	options := winHTTPAutoProxyOptions{
		// Reuse the generation-owned PAC script/WPAD discovery caches, while
		// avoiding WinHTTP's broader client/service result caches so a decision
		// from an older application session cannot become implicit App policy.
		Flags:         flags | winHTTPAutoProxyNoCacheClient | winHTTPAutoProxyNoCacheService,
		AutoConfigURL: autoConfigURL,
		// See windowsProxyNativeError: integrated credentials are not sent
		// implicitly. This remains FALSE unless a future explicit policy opts in.
		AutoLogonIfChallenged: 0,
	}
	if flags&winHTTPAutoProxyAutoDetect != 0 {
		options.AutoDetectFlags = winHTTPAutoDetectTypeDHCP | winHTTPAutoDetectTypeDNSA
	}
	targetText, err := windows.UTF16PtrFromString(target.String())
	if err != nil {
		return nil, errors.New("encode Windows proxy target URL")
	}
	var info winHTTPProxyInfo
	ok, _, callErr := winHTTPGetProxyForURL.Call(
		sessionValue,
		uintptr(unsafe.Pointer(targetText)),
		uintptr(unsafe.Pointer(&options)),
		uintptr(unsafe.Pointer(&info)),
	)
	runtime.KeepAlive(targetText)
	runtime.KeepAlive(autoConfigURL)
	if ok == 0 {
		freeWindowsProxyInfo(&info)
		return nil, newWindowsProxyNativeError("resolve Windows automatic proxy", callErr)
	}
	defer freeWindowsProxyInfo(&info)

	if info.AccessType == winHTTPAccessTypeNoProxy {
		return nil, nil
	}
	if info.AccessType != winHTTPAccessTypeNamedProxy {
		return nil, errors.New("Windows automatic proxy resolver returned an unsupported access type")
	}
	proxyText := windowsUTF16String(info.Proxy)
	if proxyText == "" {
		return nil, nil
	}
	return resolveWindowsNamedProxy(target, proxyText, windowsUTF16String(info.ProxyBypass))
}

func isWindowsAutoDiscoveryMiss(err error) bool {
	var nativeErr *windowsProxyNativeError
	if !errors.As(err, &nativeErr) {
		return false
	}
	return nativeErr.code == winHTTPAutoProxyServiceError || nativeErr.code == winHTTPAutoDetectionFailed
}

func newWindowsProxyNativeError(operation string, callErr error) error {
	code := uint32(0)
	if errno, ok := callErr.(syscall.Errno); ok {
		code = uint32(errno)
	}
	return &windowsProxyNativeError{operation: operation, code: code}
}

func windowsUTF16String(value *uint16) string {
	if value == nil {
		return ""
	}
	return windows.UTF16PtrToString(value)
}

func freeWindowsCurrentUserProxyConfig(config *winHTTPCurrentUserIEProxyConfig) {
	if config == nil {
		return
	}
	windowsGlobalFree(config.AutoConfigURL)
	windowsGlobalFree(config.Proxy)
	windowsGlobalFree(config.ProxyBypass)
	config.AutoConfigURL = nil
	config.Proxy = nil
	config.ProxyBypass = nil
}

func freeWindowsProxyInfo(info *winHTTPProxyInfo) {
	if info == nil {
		return
	}
	windowsGlobalFree(info.Proxy)
	windowsGlobalFree(info.ProxyBypass)
	info.Proxy = nil
	info.ProxyBypass = nil
}

func windowsGlobalFree(value *uint16) {
	if value != nil {
		_, _, _ = globalFree.Call(uintptr(unsafe.Pointer(value)))
	}
}

// Kept for the legacy diagnostic helper. Route selection itself must use
// platformSystemProxyURL because this string cannot represent PAC/WPAD.
func getSystemProxyFromRegistry() (string, error) {
	config, err := readWindowsCurrentUserProxyConfig()
	if err != nil {
		return "", err
	}
	defer freeWindowsCurrentUserProxyConfig(&config)
	return windowsUTF16String(config.Proxy), nil
}
