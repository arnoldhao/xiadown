package proxy

import (
	"os"
	"strings"
	"testing"
)

// The CI host is not necessarily Windows. Keep the native API and ownership
// contract under executable source checks in addition to Windows cross-builds.
func TestWindowsNativeResolverSourceContract(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("system_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		`NewProc("WinHttpGetIEProxyConfigForCurrentUser")`,
		`NewProc("WinHttpGetProxyForUrl")`,
		`NewProc("WinHttpSetTimeouts")`,
		`NewProc("WinHttpCloseHandle")`,
		`NewProc("GlobalFree")`,
		`AutoLogonIfChallenged: 0`,
		`type windowsAutomaticProxySession struct`,
		`func (session *windowsAutomaticProxySession) acquire()`,
		`func (session *windowsAutomaticProxySession) resolve(`,
		`func (session *windowsAutomaticProxySession) release()`,
		`func (session *windowsAutomaticProxySession) Close()`,
		`return resolver.automatic.resolve(sessionValue, target, flags, autoConfigURL)`,
		`session.state.serializedCall(sessionValue`,
		`resolveWindowsAutomaticProxy(handle, target, flags, autoConfigURL)`,
		`errors.Is(callErr, windows.ERROR_FILE_NOT_FOUND)`,
		`flags | winHTTPAutoProxyNoCacheClient | winHTTPAutoProxyNoCacheService`,
		`windowsGlobalFree(config.AutoConfigURL)`,
		`windowsGlobalFree(config.Proxy)`,
		`windowsGlobalFree(config.ProxyBypass)`,
		`windowsGlobalFree(info.Proxy)`,
		`windowsGlobalFree(info.ProxyBypass)`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("system_windows.go is missing native resolver contract %q", required)
		}
	}
	for _, forbidden := range []string{
		`x/sys/windows/registry`,
		`AutoLogonIfChallenged: 1`,
		`defer winHTTPCloseHandle.Call(sessionValue)`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("system_windows.go contains forbidden native resolver behavior %q", forbidden)
		}
	}
	if count := strings.Count(text, `winHTTPOpen.Call(`); count != 1 {
		t.Fatalf("WinHTTP session must be opened only by the reusable session owner; found %d call sites", count)
	}
	if count := strings.Count(text, `resolveWindowsAutomaticProxy(`); count != 2 {
		t.Fatalf("WinHTTP automatic resolution must have one definition and one serialized call site; found %d occurrences", count)
	}
	autoDetect := strings.Index(text, `resolveAutomatic(winHTTPAutoProxyAutoDetect, nil)`)
	explicitPAC := strings.Index(text, `resolveAutomatic(winHTTPAutoProxyConfigURL, config.AutoConfigURL)`)
	manualProxy := strings.Index(text, `if manualProxy != ""`)
	if autoDetect < 0 || explicitPAC < 0 || manualProxy < 0 || !(autoDetect < explicitPAC && explicitPAC < manualProxy) {
		t.Fatal("Windows resolver does not preserve WPAD -> explicit PAC -> static proxy fallback order")
	}

	sessionPolicySource, err := os.ReadFile("system_windows_session_policy.go")
	if err != nil {
		t.Fatal(err)
	}
	sessionPolicyText := string(sessionPolicySource)
	for _, required := range []string{
		`lifecycleMu sync.Mutex`,
		`callMu      sync.Mutex`,
		`state.callMu.Lock()`,
		`state.callMu.Unlock()`,
		`state.active++`,
		`state.closed && state.active == 0`,
		`closeWindowsAutomaticProxyHandle(handleToClose, closeHandle)`,
	} {
		if !strings.Contains(sessionPolicyText, required) {
			t.Fatalf("Windows session policy is missing concurrency contract %q", required)
		}
	}
	lockStart := strings.Index(sessionPolicyText, `state.lifecycleMu.Lock()`)
	lockEnd := strings.Index(sessionPolicyText, `state.lifecycleMu.Unlock()`)
	closeCall := strings.Index(sessionPolicyText, `closeWindowsAutomaticProxyHandle(handleToClose, closeHandle)`)
	if lockStart < 0 || lockEnd < 0 || closeCall < 0 || !(lockStart < lockEnd && lockEnd < closeCall) {
		t.Fatal("Windows session policy must close native handles after releasing the lifecycle lock")
	}

	gatewaySource, err := os.ReadFile("gateway.go")
	if err != nil {
		t.Fatal(err)
	}
	gatewayText := string(gatewaySource)
	for _, required := range []string{
		`state.systemProxy = newPlatformSystemProxyResolver()`,
		`systemProxyURLContext(resolveContext, s.systemProxy, request.URL)`,
		`s.systemProxy.Close()`,
	} {
		if !strings.Contains(gatewayText, required) {
			t.Fatalf("gateway.go is missing per-generation Windows resolver ownership %q", required)
		}
	}
}
