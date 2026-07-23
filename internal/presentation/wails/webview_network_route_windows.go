//go:build windows && !server

package wails

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	webView2PolicyRegistryBase       = `Software\Policies\Microsoft\Edge\WebView2`
	webView2LegacyPolicyRegistryBase = `Software\Policies\Microsoft\EmbeddedBrowserWebView\LoaderOverride`
)

var webView2LegacyStringOverrideSettings = []string{
	"additionalBrowserArguments",
	"browserExecutableFolder",
	"userDataFolder",
}

type webView2PolicyRegistryRoot struct {
	name string
	key  registry.Key
}

var webView2PolicyRegistryRoots = []webView2PolicyRegistryRoot{
	{name: "HKLM", key: registry.LOCAL_MACHINE},
	{name: "HKCU", key: registry.CURRENT_USER},
}

func prepareWebView2NetworkRouteEnvironment(arguments []string) error {
	// WebView2 appends AdditionalBrowserArguments from Loader policy after the
	// programmatic options and duplicate switches use the last value. Wails'
	// webviewloader package init replaces inherited WEBVIEW2_* variables before
	// XiaDown code can observe them. The current Wails loader also seeds those
	// variables to suppress native registry overrides, but inspect both current
	// and legacy policy layouts directly so a loader/dependency change cannot
	// silently reopen that route.
	if err := validateWebView2HostArguments(os.Args[1:]); err != nil {
		return err
	}
	if err := ensureNoWebView2LoaderPolicyOverride(); err != nil {
		return err
	}

	// Publish XiaDown's exact value before application.New. Wails publishes the
	// same programmatic value again while it creates the shared environment.
	desired := strings.Join(arguments, " ")
	if err := os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", desired); err != nil {
		return fmt.Errorf("%w: set WebView2 browser arguments: %v", ErrWebViewNetworkRouteConflict, err)
	}
	if err := os.Setenv("WEBVIEW2_PIPE_FOR_SCRIPT_DEBUGGER", ""); err != nil {
		return fmt.Errorf("%w: disable WebView2 debugger pipe: %v", ErrWebViewNetworkRouteConflict, err)
	}
	if os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS") != desired || os.Getenv("WEBVIEW2_PIPE_FOR_SCRIPT_DEBUGGER") != "" {
		return fmt.Errorf("%w: WebView2 environment did not retain XiaDown's arguments", ErrWebViewNetworkRouteConflict)
	}
	return nil
}

func ensureNoWebView2LoaderPolicyOverride() error {
	appIDs, err := currentWebView2PolicyAppIDs()
	if err != nil {
		return fmt.Errorf("%w: identify WebView2 policy application: %v", ErrWebViewNetworkRouteConflict, err)
	}
	override, err := findWebView2LoaderPolicyOverride(appIDs, readWebView2LoaderPolicyValue)
	if err != nil {
		return fmt.Errorf("%w: inspect WebView2 Loader policy: %v", ErrWebViewNetworkRouteConflict, err)
	}
	if override != nil {
		return fmt.Errorf(
			"%w: WebView2 Loader policy %s\\%s applies to %q",
			ErrWebViewNetworkRouteConflict,
			override.Root,
			override.Setting,
			override.AppID,
		)
	}
	legacyOverride, err := findWebView2LegacyLoaderOverride(appIDs, readWebView2LegacyLoaderOverride)
	if err != nil {
		return fmt.Errorf("%w: inspect legacy WebView2 LoaderOverride policy: %v", ErrWebViewNetworkRouteConflict, err)
	}
	if legacyOverride != nil {
		return fmt.Errorf(
			"%w: legacy WebView2 LoaderOverride policy %s\\%s\\%s value %q applies",
			ErrWebViewNetworkRouteConflict,
			legacyOverride.Root,
			webView2LegacyPolicyRegistryBase,
			legacyOverride.AppID,
			legacyOverride.Setting,
		)
	}
	return nil
}

func readWebView2LegacyLoaderOverride(root, appID string) (bool, string, error) {
	registryRoot, err := lookupWebView2PolicyRegistryRoot(root)
	if err != nil {
		return false, "", err
	}
	key, err := registry.OpenKey(
		registryRoot,
		webView2LegacyPolicyRegistryBase+`\`+appID,
		registry.QUERY_VALUE,
	)
	if errors.Is(err, registry.ErrNotExist) {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("open %s legacy LoaderOverride for %q: %w", root, appID, err)
	}
	defer key.Close()

	for _, setting := range webView2LegacyStringOverrideSettings {
		value, valueType, valueErr := key.GetStringValue(setting)
		if errors.Is(valueErr, registry.ErrNotExist) {
			continue
		}
		if valueErr != nil || valueType != registry.SZ {
			if valueErr == nil {
				valueErr = fmt.Errorf("unexpected registry type %d", valueType)
			}
			return true, "", fmt.Errorf("read %s legacy LoaderOverride %q for %q: %w", root, setting, appID, valueErr)
		}
		if strings.TrimSpace(value) != "" {
			return true, setting, nil
		}
	}

	const releaseChannelPreference = "releaseChannelPreference"
	value, valueType, valueErr := key.GetIntegerValue(releaseChannelPreference)
	if errors.Is(valueErr, registry.ErrNotExist) {
		return true, "", nil
	}
	if valueErr != nil || valueType != registry.DWORD {
		if valueErr == nil {
			valueErr = fmt.Errorf("unexpected registry type %d", valueType)
		}
		return true, "", fmt.Errorf("read %s legacy LoaderOverride %q for %q: %w", root, releaseChannelPreference, appID, valueErr)
	}
	if value != 0 {
		return true, releaseChannelPreference, nil
	}
	return true, "", nil
}

func lookupWebView2PolicyRegistryRoot(root string) (registry.Key, error) {
	for _, candidate := range webView2PolicyRegistryRoots {
		if candidate.name == root {
			return candidate.key, nil
		}
	}
	return 0, fmt.Errorf("unknown registry root %q", root)
}

func readWebView2LoaderPolicyValue(root, setting, appID string) (bool, bool, error) {
	registryRoot, err := lookupWebView2PolicyRegistryRoot(root)
	if err != nil {
		return false, false, err
	}

	key, err := registry.OpenKey(
		registryRoot,
		webView2PolicyRegistryBase+`\`+setting,
		registry.QUERY_VALUE,
	)
	if errors.Is(err, registry.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("open %s %s: %w", root, setting, err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(appID)
	if errors.Is(err, registry.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		// The documented policy schema uses REG_SZ. Treat an unexpected type as
		// unreadable policy and fail closed instead of guessing its effect.
		return false, false, fmt.Errorf("read %s %s for %q: %w", root, setting, appID, err)
	}
	return true, strings.TrimSpace(value) != "", nil
}

func currentWebView2PolicyAppIDs() ([]string, error) {
	appIDs := make([]string, 0, 2)
	applicationUserModelID, err := currentProcessApplicationUserModelID()
	if err != nil {
		return nil, err
	}
	appIDs = appendUniqueWebView2PolicyAppID(appIDs, applicationUserModelID)

	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}
	executableName := strings.TrimSpace(filepath.Base(executablePath))
	if executableName == "" || executableName == "." {
		return nil, fmt.Errorf("resolve executable name from %q", executablePath)
	}
	appIDs = appendUniqueWebView2PolicyAppID(appIDs, executableName)
	return appIDs, nil
}

func currentProcessApplicationUserModelID() (string, error) {
	// Unpackaged applications may set an explicit process AUMID.
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	getExplicit := shell32.NewProc("GetCurrentProcessExplicitAppUserModelID")
	if err := getExplicit.Find(); err != nil {
		return "", fmt.Errorf("find GetCurrentProcessExplicitAppUserModelID: %w", err)
	}
	var explicit *uint16
	hresult, _, _ := getExplicit.Call(uintptr(unsafe.Pointer(&explicit)))
	if uint32(hresult) == uint32(windows.S_OK) {
		if explicit != nil {
			defer windows.CoTaskMemFree(unsafe.Pointer(explicit))
			if value := strings.TrimSpace(windows.UTF16PtrToString(explicit)); value != "" {
				return value, nil
			}
		}
	} else if uint32(hresult) != 0x80004005 { // E_FAIL means no explicit AUMID.
		return "", fmt.Errorf("GetCurrentProcessExplicitAppUserModelID returned %#x", uint32(hresult))
	}

	// Packaged applications expose their AUMID through the application-model
	// API even when no explicit process AUMID was assigned.
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	getPackaged := kernel32.NewProc("GetCurrentApplicationUserModelId")
	if err := getPackaged.Find(); err != nil {
		return "", fmt.Errorf("find GetCurrentApplicationUserModelId: %w", err)
	}
	var length uint32
	result, _, _ := getPackaged.Call(uintptr(unsafe.Pointer(&length)), 0)
	if uint32(result) == uint32(windows.APPMODEL_ERROR_NO_APPLICATION) {
		return "", nil
	}
	if uint32(result) != uint32(windows.ERROR_INSUFFICIENT_BUFFER) || length == 0 {
		return "", fmt.Errorf("GetCurrentApplicationUserModelId size returned %#x", uint32(result))
	}
	buffer := make([]uint16, length)
	result, _, _ = getPackaged.Call(
		uintptr(unsafe.Pointer(&length)),
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if uint32(result) != uint32(windows.ERROR_SUCCESS) {
		return "", fmt.Errorf("GetCurrentApplicationUserModelId returned %#x", uint32(result))
	}
	return strings.TrimSpace(windows.UTF16ToString(buffer)), nil
}

func applyWebViewNetworkRoutePlatform(gateway webViewNetworkGateway, preparedForWebView2 bool) error {
	if !preparedForWebView2 {
		return fmt.Errorf(
			"%w: append WebView2BrowserArguments to application.Options.Windows.AdditionalBrowserArgs before application.New",
			ErrWebViewNetworkRouteRequiresPreRun,
		)
	}
	// Wails creates one WebView2 browser environment for all windows. The
	// command-line route is immutable after that environment is constructed;
	// keeping the gateway address stable allows upstream policy changes without
	// replacing the environment or its persistent user-data profile.
	_ = gateway
	return nil
}
