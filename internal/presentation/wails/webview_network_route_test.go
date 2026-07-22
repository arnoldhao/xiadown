package wails

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type webViewNetworkRouteTestProvider struct {
	gatewayURL string
}

func (provider webViewNetworkRouteTestProvider) GatewayURL() string {
	return provider.gatewayURL
}

func TestParseWebViewNetworkGateway(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		raw       string
		canonical string
		valid     bool
	}{
		{name: "IPv4", raw: "http://127.0.0.1:32100", canonical: "http://127.0.0.1:32100", valid: true},
		{name: "IPv6", raw: "http://[::1]:32100/", canonical: "http://[::1]:32100", valid: true},
		{name: "localhost", raw: "HTTP://LOCALHOST:32100", canonical: "http://localhost:32100", valid: true},
		{name: "empty", raw: "", valid: false},
		{name: "non loopback", raw: "http://192.168.1.10:32100", valid: false},
		{name: "missing port", raw: "http://127.0.0.1", valid: false},
		{name: "zero port", raw: "http://127.0.0.1:0", valid: false},
		{name: "credentials", raw: "http://user:pass@127.0.0.1:32100", valid: false},
		{name: "https", raw: "https://127.0.0.1:32100", valid: false},
		{name: "path", raw: "http://127.0.0.1:32100/proxy", valid: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gateway, err := parseWebViewNetworkGateway(test.raw)
			if !test.valid {
				if !errors.Is(err, ErrWebViewNetworkRouteInvalidGateway) {
					t.Fatalf("parse error = %v, want invalid gateway", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if gateway.URL != test.canonical {
				t.Fatalf("canonical URL = %q, want %q", gateway.URL, test.canonical)
			}
		})
	}
}

func TestWebView2BrowserArgumentsRouteAllRemoteTraffic(t *testing.T) {
	t.Parallel()

	route := NewWebViewNetworkRoute(webViewNetworkRouteTestProvider{
		gatewayURL: "http://127.0.0.1:32100",
	})
	arguments, err := route.WebView2BrowserArguments()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--proxy-server=http://127.0.0.1:32100",
		"--proxy-bypass-list=<-loopback>",
		"--disable-quic",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--dns-prefetch-disable",
	}
	if len(arguments) != len(want) {
		t.Fatalf("arguments = %#v, want %#v", arguments, want)
	}
	for index := range want {
		if arguments[index] != want[index] {
			t.Fatalf("arguments = %#v, want %#v", arguments, want)
		}
	}
	for _, argument := range arguments {
		if strings.Contains(argument, "youtube") || strings.Contains(argument, "googlevideo") {
			t.Fatalf("browser route contains a remote bypass: %q", argument)
		}
	}
}

func TestWebView2HostArgumentsRejectLateProxyOverrides(t *testing.T) {
	t.Parallel()

	for _, argument := range []string{
		"--edge-webview-switches=--proxy-server=http://outside.example:8080",
		"--proxy-server=http://outside.example:8080",
		"--proxy-bypass-list=*",
		"--proxy-pac-url=https://outside.example/proxy.pac",
		"--proxy-auto-detect",
		"--no-proxy-server",
	} {
		argument := argument
		t.Run(argument, func(t *testing.T) {
			t.Parallel()
			if err := validateWebView2HostArguments([]string{argument}); !errors.Is(err, ErrWebViewNetworkRouteConflict) {
				t.Fatalf("argument %q error = %v, want route conflict", argument, err)
			}
		})
	}
	if err := validateWebView2HostArguments([]string{"--safe-option", "document.txt"}); err != nil {
		t.Fatalf("safe host arguments rejected: %v", err)
	}
}

func TestFindWebView2LoaderPolicyOverrideUsesDocumentedPrecedence(t *testing.T) {
	t.Parallel()

	type policyValue struct {
		exists   bool
		nonEmpty bool
	}
	type policyKey struct {
		root    string
		setting string
		appID   string
	}
	probe := func(values map[policyKey]policyValue) webView2LoaderPolicyProbe {
		return func(root, setting, appID string) (bool, bool, error) {
			value := values[policyKey{root: root, setting: setting, appID: appID}]
			return value.exists, value.nonEmpty, nil
		}
	}
	appIDs := []string{"com.xiadown.desktop", "XiaDown.exe"}

	for _, test := range []struct {
		name   string
		values map[policyKey]policyValue
		want   *webView2LoaderPolicyOverride
	}{
		{
			name: "AUMID value",
			values: map[policyKey]policyValue{
				{root: "HKLM", setting: "AdditionalBrowserArguments", appID: "com.xiadown.desktop"}: {exists: true, nonEmpty: true},
			},
			want: &webView2LoaderPolicyOverride{Root: "HKLM", Setting: "AdditionalBrowserArguments", AppID: "com.xiadown.desktop"},
		},
		{
			name: "empty AUMID masks lower app values",
			values: map[policyKey]policyValue{
				{root: "HKLM", setting: "AdditionalBrowserArguments", appID: "com.xiadown.desktop"}: {exists: true},
				{root: "HKLM", setting: "AdditionalBrowserArguments", appID: "XiaDown.exe"}:         {exists: true, nonEmpty: true},
				{root: "HKLM", setting: "AdditionalBrowserArguments", appID: "*"}:                   {exists: true, nonEmpty: true},
				{root: "HKCU", setting: "AdditionalBrowserArguments", appID: "com.xiadown.desktop"}: {exists: true, nonEmpty: true},
			},
		},
		{
			name: "machine root masks current user root",
			values: map[policyKey]policyValue{
				{root: "HKLM", setting: "BrowserExecutableFolder", appID: "XiaDown.exe"}:         {exists: true},
				{root: "HKCU", setting: "BrowserExecutableFolder", appID: "com.xiadown.desktop"}: {exists: true, nonEmpty: true},
			},
		},
		{
			name: "wildcard applies",
			values: map[policyKey]policyValue{
				{root: "HKCU", setting: "ReleaseChannelPreference", appID: "*"}: {exists: true, nonEmpty: true},
			},
			want: &webView2LoaderPolicyOverride{Root: "HKCU", Setting: "ReleaseChannelPreference", AppID: "*"},
		},
		{
			name: "wildcard user data folder is not applicable",
			values: map[policyKey]policyValue{
				{root: "HKLM", setting: "UserDataFolder", appID: "*"}: {exists: true, nonEmpty: true},
			},
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := findWebView2LoaderPolicyOverride(appIDs, probe(test.values))
			if err != nil {
				t.Fatal(err)
			}
			if test.want == nil {
				if got != nil {
					t.Fatalf("override = %#v, want none", got)
				}
				return
			}
			if got == nil || *got != *test.want {
				t.Fatalf("override = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestFindWebView2LegacyLoaderOverrideUsesKeyPrecedence(t *testing.T) {
	t.Parallel()

	type policyKey struct {
		root  string
		appID string
	}
	type policyValue struct {
		exists  bool
		setting string
		err     error
	}
	probe := func(values map[policyKey]policyValue) webView2LegacyLoaderOverrideProbe {
		return func(root, appID string) (bool, string, error) {
			value := values[policyKey{root: root, appID: appID}]
			return value.exists, value.setting, value.err
		}
	}
	appIDs := []string{"com.xiadown.desktop", "XiaDown.exe"}
	probeErr := errors.New("registry unreadable")

	for _, test := range []struct {
		name    string
		values  map[policyKey]policyValue
		want    *webView2LegacyLoaderOverride
		wantErr error
	}{
		{
			name: "machine AUMID additional arguments",
			values: map[policyKey]policyValue{
				{root: "HKLM", appID: "com.xiadown.desktop"}: {exists: true, setting: "additionalBrowserArguments"},
			},
			want: &webView2LegacyLoaderOverride{Root: "HKLM", AppID: "com.xiadown.desktop", Setting: "additionalBrowserArguments"},
		},
		{
			name: "machine executable fallback",
			values: map[policyKey]policyValue{
				{root: "HKLM", appID: "XiaDown.exe"}: {exists: true, setting: "browserExecutableFolder"},
			},
			want: &webView2LegacyLoaderOverride{Root: "HKLM", AppID: "XiaDown.exe", Setting: "browserExecutableFolder"},
		},
		{
			name: "first empty key masks lower application keys",
			values: map[policyKey]policyValue{
				{root: "HKLM", appID: "com.xiadown.desktop"}: {exists: true},
				{root: "HKLM", appID: "XiaDown.exe"}:         {exists: true, setting: "browserExecutableFolder"},
				{root: "HKCU", appID: "com.xiadown.desktop"}: {exists: true, setting: "userDataFolder"},
			},
		},
		{
			name: "machine wildcard precedes current user AUMID",
			values: map[policyKey]policyValue{
				{root: "HKLM", appID: "*"}:                   {exists: true, setting: "releaseChannelPreference"},
				{root: "HKCU", appID: "com.xiadown.desktop"}: {exists: true, setting: "additionalBrowserArguments"},
			},
			want: &webView2LegacyLoaderOverride{Root: "HKLM", AppID: "*", Setting: "releaseChannelPreference"},
		},
		{
			name: "current user AUMID fallback",
			values: map[policyKey]policyValue{
				{root: "HKCU", appID: "com.xiadown.desktop"}: {exists: true, setting: "additionalBrowserArguments"},
			},
			want: &webView2LegacyLoaderOverride{Root: "HKCU", AppID: "com.xiadown.desktop", Setting: "additionalBrowserArguments"},
		},
		{
			name: "current user executable fallback",
			values: map[policyKey]policyValue{
				{root: "HKCU", appID: "XiaDown.exe"}: {exists: true, setting: "browserExecutableFolder"},
			},
			want: &webView2LegacyLoaderOverride{Root: "HKCU", AppID: "XiaDown.exe", Setting: "browserExecutableFolder"},
		},
		{
			name: "legacy wildcard user data folder applies",
			values: map[policyKey]policyValue{
				{root: "HKCU", appID: "*"}: {exists: true, setting: "userDataFolder"},
			},
			want: &webView2LegacyLoaderOverride{Root: "HKCU", AppID: "*", Setting: "userDataFolder"},
		},
		{
			name: "registry error fails closed",
			values: map[policyKey]policyValue{
				{root: "HKLM", appID: "com.xiadown.desktop"}: {err: probeErr},
			},
			wantErr: probeErr,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := findWebView2LegacyLoaderOverride(appIDs, probe(test.values))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if test.want == nil {
				if got != nil {
					t.Fatalf("override = %#v, want none", got)
				}
				return
			}
			if got == nil || *got != *test.want {
				t.Fatalf("override = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestWebViewNetworkRouteServiceAppliesOnceForStableGateway(t *testing.T) {
	t.Parallel()

	applyCount := 0
	route := NewWebViewNetworkRoute(webViewNetworkRouteTestProvider{
		gatewayURL: "http://127.0.0.1:32100",
	})
	route.apply = func(gateway webViewNetworkGateway, _ bool) error {
		applyCount++
		if gateway.URL != "http://127.0.0.1:32100" {
			t.Fatalf("gateway = %q", gateway.URL)
		}
		return nil
	}
	if err := route.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := route.Apply(); err != nil {
		t.Fatal(err)
	}
	if applyCount != 1 {
		t.Fatalf("native apply count = %d, want 1", applyCount)
	}
}

func TestWebView2BrowserArgumentsPrepareTheSameRouteForStartup(t *testing.T) {
	t.Parallel()

	preparedForWebView2 := false
	route := NewWebViewNetworkRoute(webViewNetworkRouteTestProvider{
		gatewayURL: "http://127.0.0.1:32100",
	})
	route.apply = func(_ webViewNetworkGateway, prepared bool) error {
		preparedForWebView2 = prepared
		return nil
	}
	if _, err := route.WebView2BrowserArguments(); err != nil {
		t.Fatal(err)
	}
	if err := route.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	if !preparedForWebView2 {
		t.Fatal("ServiceStartup did not observe the pre-run WebView2 arguments")
	}
}

func TestWebViewNetworkRoutePlatformSourcesKeepPersistentProfiles(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		file      string
		required  []string
		forbidden []string
	}{
		{
			file: "webview_network_route_darwin.go",
			required: []string{
				"nw_proxy_config_create_http_connect",
				"nw_proxy_config_set_failover_allowed(config, false)",
				"[WKWebsiteDataStore defaultDataStore].proxyConfigurations",
			},
			forbidden: []string{"@available(macOS 14.0, *)", "XiaDownWebViewNetworkRouteUnsupported", "nonPersistentDataStore", "initWithIdentifier", "wails.localhost", `excluded_domain(config, "localhost")`, `excluded_domain(config, "127.0.0.1")`},
		},
		{
			file: "webview_network_route_windows.go",
			required: []string{
				"validateWebView2HostArguments(os.Args[1:])",
				"ensureNoWebView2LoaderPolicyOverride()",
				`Software\Policies\Microsoft\Edge\WebView2`,
				`Software\Policies\Microsoft\EmbeddedBrowserWebView\LoaderOverride`,
				"readWebView2LegacyLoaderOverride",
				"findWebView2LegacyLoaderOverride",
				`"additionalBrowserArguments"`,
				`"browserExecutableFolder"`,
				`"userDataFolder"`,
				`"releaseChannelPreference"`,
				"key.GetStringValue(setting)",
				"key.GetIntegerValue(releaseChannelPreference)",
				"registry.LOCAL_MACHINE",
				"registry.CURRENT_USER",
				"currentProcessApplicationUserModelID()",
				"filepath.Base(executablePath)",
				`os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", desired)`,
				`os.Setenv("WEBVIEW2_PIPE_FOR_SCRIPT_DEBUGGER", "")`,
			},
		},
		{
			file: "webview_network_route.go",
			required: []string{
				"BrowserExecutableFolder",
				"ReleaseChannelPreference",
				"AdditionalBrowserArguments",
				"UserDataFolder",
				`[]string{"HKLM", "HKCU"}`,
			},
		},
		{
			file: "webview_network_route_linux.go",
			required: []string{
				"webkit_network_session_get_default",
				"webkit_network_session_set_proxy_settings",
				"WEBKIT_NETWORK_PROXY_MODE_CUSTOM",
				`webkit_network_proxy_settings_new(gateway_uri, NULL)`,
			},
			forbidden: []string{"wails.localhost"},
		},
		{
			file: "webview_network_route_linux_gtk3.go",
			required: []string{
				"webkit_web_context_get_default",
				"webkit_web_context_get_website_data_manager",
				"webkit_website_data_manager_set_network_proxy_settings",
				"WEBKIT_NETWORK_PROXY_MODE_CUSTOM",
				`webkit_network_proxy_settings_new(gateway_uri, NULL)`,
			},
			forbidden: []string{"wails.localhost"},
		},
	} {
		source, err := os.ReadFile(test.file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		for _, required := range test.required {
			if !strings.Contains(text, required) {
				t.Fatalf("%s is missing %q", test.file, required)
			}
		}
		for _, forbidden := range test.forbidden {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s replaces the persistent profile through %q", test.file, forbidden)
			}
		}
		if test.file == "webview_network_route_windows.go" {
			policyCheck := strings.Index(text, "ensureNoWebView2LoaderPolicyOverride()")
			environmentOverride := strings.Index(text, `os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", desired)`)
			if policyCheck < 0 || environmentOverride < 0 || policyCheck >= environmentOverride {
				t.Fatal("Windows WebView2 Loader policy is not checked before its environment is replaced")
			}
		}
	}
}
