package wails

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	// ErrWebViewNetworkRouteUnsupported means the current native WebView does
	// not expose a public API that can install XiaDown's gateway route.
	ErrWebViewNetworkRouteUnsupported = errors.New("webview network route unsupported")
	// ErrWebViewNetworkRouteInvalidGateway means GatewayURL did not identify an
	// app-owned loopback HTTP CONNECT gateway.
	ErrWebViewNetworkRouteInvalidGateway = errors.New("invalid webview network gateway")
	// ErrWebViewNetworkRouteRequiresPreRun means the platform route must be put
	// in application.Options before Wails creates its browser environment.
	ErrWebViewNetworkRouteRequiresPreRun = errors.New("webview network route requires pre-run application options")
	// ErrWebViewNetworkRouteConflict means an inherited host/browser option can
	// be processed after XiaDown's proxy switch and silently replace it.
	ErrWebViewNetworkRouteConflict = errors.New("webview network route conflicts with an external browser option")
)

// WebViewNetworkRouteProvider exposes the stable, app-owned HTTP CONNECT
// gateway used by native WebViews. The gateway address must remain stable for
// the lifetime of the browser profile; upstream route changes happen inside
// the gateway rather than by replacing the native WebView profile.
type WebViewNetworkRouteProvider interface {
	GatewayURL() string
}

type webViewNetworkGateway struct {
	URL  string
	Host string
	Port string
}

type webViewNetworkRouteApplyFunc func(webViewNetworkGateway, bool) error

var webView2LoaderPolicySettings = []string{
	"BrowserExecutableFolder",
	"ReleaseChannelPreference",
	"ChannelSearchKind",
	"ReleaseChannels",
	"AdditionalBrowserArguments",
	"UserDataFolder",
}

type webView2LoaderPolicyOverride struct {
	Root    string
	Setting string
	AppID   string
}

type webView2LoaderPolicyProbe func(root, setting, appID string) (exists, nonEmpty bool, err error)

// The legacy WebView2 LoaderOverride policy uses one key per application,
// rather than one setting key with application-named values. The Loader stops
// at the first existing key in HKLM/HKCU and AUMID/executable/wildcard order,
// even when that key contains no material override value.
type webView2LegacyLoaderOverride struct {
	Root    string
	AppID   string
	Setting string
}

type webView2LegacyLoaderOverrideProbe func(root, appID string) (exists bool, setting string, err error)

// WebViewNetworkRoute installs the app-owned gateway on the platform's shared
// persistent WebView network session. Register it before services that create
// or navigate WebViews so ServiceStartup runs before the first page load.
//
// On Windows, call WebView2BrowserArguments and append the returned values to
// application.Options.Windows.AdditionalBrowserArgs before application.New.
// ServiceStartup then verifies that this pre-run step used the current gateway.
type WebViewNetworkRoute struct {
	provider WebViewNetworkRouteProvider
	apply    webViewNetworkRouteApplyFunc

	mu                      sync.Mutex
	appliedGatewayURL       string
	preparedWebView2Gateway string
}

func NewWebViewNetworkRoute(provider WebViewNetworkRouteProvider) *WebViewNetworkRoute {
	return &WebViewNetworkRoute{
		provider: provider,
		apply:    applyWebViewNetworkRoutePlatform,
	}
}

func (route *WebViewNetworkRoute) ServiceName() string {
	return "WebViewNetworkRoute"
}

// ServiceStartup applies the native route before Wails creates pending
// windows. It intentionally returns platform and configuration errors: letting
// startup continue would silently put WebView traffic outside the app policy.
func (route *WebViewNetworkRoute) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return route.Apply()
}

// Apply installs the current stable gateway and is safe to call repeatedly.
// A changed gateway may interrupt active WebView requests, so production code
// should keep GatewayURL stable and switch only the gateway's upstream route.
func (route *WebViewNetworkRoute) Apply() error {
	if route == nil || route.provider == nil {
		return fmt.Errorf("%w: provider is unavailable", ErrWebViewNetworkRouteInvalidGateway)
	}
	gateway, err := parseWebViewNetworkGateway(route.provider.GatewayURL())
	if err != nil {
		return err
	}

	route.mu.Lock()
	defer route.mu.Unlock()
	if route.appliedGatewayURL == gateway.URL {
		return nil
	}
	if route.apply == nil {
		return fmt.Errorf("%w: native adapter is unavailable", ErrWebViewNetworkRouteUnsupported)
	}
	preparedForWebView2 := route.preparedWebView2Gateway == gateway.URL
	if err := route.apply(gateway, preparedForWebView2); err != nil {
		return err
	}
	route.appliedGatewayURL = gateway.URL
	return nil
}

// WebView2BrowserArguments returns the global WebView2 environment arguments
// required on Windows. Append them to
// application.Options.Windows.AdditionalBrowserArgs before application.New;
// WebView2 shares that environment (and its persistent cookie profile) across
// every XiaDown window.
func (route *WebViewNetworkRoute) WebView2BrowserArguments() ([]string, error) {
	if route == nil || route.provider == nil {
		return nil, fmt.Errorf("%w: provider is unavailable", ErrWebViewNetworkRouteInvalidGateway)
	}
	gateway, err := parseWebViewNetworkGateway(route.provider.GatewayURL())
	if err != nil {
		return nil, err
	}

	arguments := webView2NetworkRouteBrowserArguments(gateway)
	if err := prepareWebView2NetworkRouteEnvironment(arguments); err != nil {
		return nil, err
	}
	route.mu.Lock()
	route.preparedWebView2Gateway = gateway.URL
	route.mu.Unlock()
	return arguments, nil
}

func webView2NetworkRouteBrowserArguments(gateway webViewNetworkGateway) []string {
	return []string{
		"--proxy-server=" + gateway.URL,
		// Chromium implicitly bypasses all loopback destinations. Subtract that
		// broad rule so remote pages cannot silently leave the App route. Wails
		// serves its virtual asset host in-process, so it needs no network bypass.
		"--proxy-bypass-list=<-loopback>",
		// HTTP/3 QUIC would otherwise use direct UDP outside an HTTP CONNECT
		// gateway. WebRTC receives the corresponding no-direct-UDP policy.
		// Disable speculative DNS as well: it is not needed to reach the fixed
		// loopback gateway and must not resolve page-owned names outside it.
		"--disable-quic",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--dns-prefetch-disable",
	}
}

func validateWebView2HostArguments(arguments []string) error {
	for _, raw := range arguments {
		argument := strings.ToLower(strings.TrimSpace(raw))
		for _, switchName := range []string{
			"--edge-webview-switches",
			"--proxy-server",
			"--proxy-bypass-list",
			"--proxy-pac-url",
			"--proxy-auto-detect",
			"--no-proxy-server",
		} {
			if argument == switchName || strings.HasPrefix(argument, switchName+"=") {
				return fmt.Errorf("%w: host command line contains %s", ErrWebViewNetworkRouteConflict, switchName)
			}
		}
	}
	return nil
}

// findWebView2LoaderPolicyOverride mirrors the WebView2 Loader's documented
// lookup order for its per-app policy values. An existing empty value wins its
// precedence slot and prevents a lower-priority value from becoming active.
// UserDataFolder is the one setting for which WebView2 does not accept the '*'
// application value.
func findWebView2LoaderPolicyOverride(appIDs []string, probe webView2LoaderPolicyProbe) (*webView2LoaderPolicyOverride, error) {
	if probe == nil {
		return nil, nil
	}
	appIDs = appendUniqueWebView2PolicyAppID(appIDs, "*")
	for _, setting := range webView2LoaderPolicySettings {
		settingResolved := false
		for _, root := range []string{"HKLM", "HKCU"} {
			for _, appID := range appIDs {
				if setting == "UserDataFolder" && appID == "*" {
					continue
				}
				exists, nonEmpty, err := probe(root, setting, appID)
				if err != nil {
					return nil, err
				}
				if !exists {
					continue
				}
				settingResolved = true
				if nonEmpty {
					return &webView2LoaderPolicyOverride{
						Root:    root,
						Setting: setting,
						AppID:   appID,
					}, nil
				}
				break
			}
			if settingResolved {
				break
			}
		}
	}
	return nil, nil
}

// findWebView2LegacyLoaderOverride mirrors the legacy LoaderOverride key lookup
// documented by CreateCoreWebView2EnvironmentWithOptions. Unlike the current
// Edge\WebView2 policy layout, precedence is resolved for the whole AppId key,
// not independently for each setting within it.
func findWebView2LegacyLoaderOverride(appIDs []string, probe webView2LegacyLoaderOverrideProbe) (*webView2LegacyLoaderOverride, error) {
	if probe == nil {
		return nil, nil
	}
	appIDs = appendUniqueWebView2PolicyAppID(appIDs, "*")
	for _, root := range []string{"HKLM", "HKCU"} {
		for _, appID := range appIDs {
			exists, setting, err := probe(root, appID)
			if err != nil {
				return nil, err
			}
			if !exists {
				continue
			}
			if strings.TrimSpace(setting) == "" {
				return nil, nil
			}
			return &webView2LegacyLoaderOverride{
				Root:    root,
				AppID:   appID,
				Setting: setting,
			}, nil
		}
	}
	return nil, nil
}

func appendUniqueWebView2PolicyAppID(appIDs []string, candidate string) []string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return appIDs
	}
	for _, existing := range appIDs {
		if strings.EqualFold(strings.TrimSpace(existing), candidate) {
			return appIDs
		}
	}
	return append(appIDs, candidate)
}

func parseWebViewNetworkGateway(raw string) (webViewNetworkGateway, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return webViewNetworkGateway{}, fmt.Errorf("%w: URL is empty", ErrWebViewNetworkRouteInvalidGateway)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return webViewNetworkGateway{}, fmt.Errorf("%w: %v", ErrWebViewNetworkRouteInvalidGateway, err)
	}
	if !strings.EqualFold(parsed.Scheme, "http") || parsed.Opaque != "" || parsed.User != nil {
		return webViewNetworkGateway{}, fmt.Errorf("%w: gateway must be an unauthenticated HTTP URL", ErrWebViewNetworkRouteInvalidGateway)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return webViewNetworkGateway{}, fmt.Errorf("%w: gateway URL cannot contain a path, query, or fragment", ErrWebViewNetworkRouteInvalidGateway)
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" || !isWebViewNetworkLoopbackHost(host) {
		return webViewNetworkGateway{}, fmt.Errorf("%w: gateway host must be loopback", ErrWebViewNetworkRouteInvalidGateway)
	}
	port := parsed.Port()
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return webViewNetworkGateway{}, fmt.Errorf("%w: gateway requires a valid non-zero port", ErrWebViewNetworkRouteInvalidGateway)
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	canonical := (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.FormatUint(portNumber, 10)),
	}).String()
	return webViewNetworkGateway{
		URL:  canonical,
		Host: host,
		Port: strconv.FormatUint(portNumber, 10),
	}, nil
}

func isWebViewNetworkLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
