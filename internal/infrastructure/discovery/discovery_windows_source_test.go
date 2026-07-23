package discovery

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsRegistrationConstructsInstanceWithLocalHostName(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("discovery_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	if !strings.Contains(
		source,
		"func freeWindowsDNSServiceInstance(instance uintptr) {\n\tprocFreeService.Call(instance)\n}",
	) {
		t.Fatal("Windows callback instance releaser must call DnsServiceFreeInstance")
	}
	start := strings.Index(source, "func registerWindowsInterface(")
	if start < 0 {
		t.Fatal("registerWindowsInterface source start was not found")
	}
	end := strings.Index(source[start:], "\nfunc (")
	if end < 0 {
		t.Fatal("registerWindowsInterface source end was not found")
	}
	body := source[start : start+end]

	for _, required := range []string{
		"systemHostName, err := os.Hostname()",
		"dnsHostName, err := windowsDNSServiceHostName(systemHostName)",
		"nativeHostName, err := windows.UTF16PtrFromString(dnsHostName)",
		"uintptr(unsafe.Pointer(serviceName)), uintptr(unsafe.Pointer(nativeHostName)), 0, 0,",
		"runtime.KeepAlive(nativeHostName)",
		"instance, _, _ := procConstructService.Call(",
		"windowsDNSSDNativeCallError(",
		"callback := syscall.NewCallback(func(status, _, callbackInstance uintptr) uintptr {",
		"releaseWindowsDNSSDCallbackInstance(callbackInstance, freeWindowsDNSServiceInstance)",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("Windows DNS-SD registration is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"uintptr(unsafe.Pointer(serviceName)), 0, 0, 0",
		`fmt.Errorf("construct Windows DNS-SD instance: %w", callErr)`,
		"instance, _, callErr := procConstructService.Call(",
		"func(status, _, _ uintptr) uintptr",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("Windows DNS-SD registration still contains %q", forbidden)
		}
	}

	callbackStart := strings.Index(body, "callback := syscall.NewCallback(")
	if callbackStart < 0 {
		t.Fatal("Windows DNS-SD callback source was not found")
	}
	callbackBody := body[callbackStart:]
	callbackRelease := strings.Index(callbackBody, "releaseWindowsDNSSDCallbackInstance(")
	callbackPublish := strings.Index(callbackBody, "case events <- uint32(status):")
	if callbackRelease < 0 || callbackPublish < 0 || callbackRelease >= callbackPublish {
		t.Fatal("Windows DNS-SD callback must release its returned instance before publishing status")
	}
	if strings.Contains(callbackBody[:callbackPublish], "procFreeService.Call(instance)") {
		t.Fatal("Windows DNS-SD callback must not free the original constructed instance")
	}
}

func TestWindowsDeregisterSynchronousFailureRetainsOriginalInstance(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("discovery_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	start := strings.Index(source, "func (registration *windowsRegistration) Close()")
	if start < 0 {
		t.Fatal("windowsRegistration.Close source start was not found")
	}
	body := source[start:]
	synchronousFailure := strings.Index(body, "if status != dnsRequestPending {")
	failureReturn := strings.Index(body, "\n\t\t\treturn")
	originalFree := strings.Index(body, "procFreeService.Call(registration.instance)")
	if synchronousFailure < 0 || failureReturn < 0 || originalFree < 0 ||
		synchronousFailure >= failureReturn || failureReturn >= originalFree {
		t.Fatal("synchronous deregistration failure must return before freeing the original instance")
	}
	if strings.Contains(body[synchronousFailure:failureReturn], "procFreeService.Call(registration.instance)") {
		t.Fatal("synchronous deregistration failure frees the possibly still-registered original instance")
	}
}
