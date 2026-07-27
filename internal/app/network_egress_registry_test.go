package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

// egressRegistryEntry is the review record for a low-level network-capable API.
// A new occurrence must be classified here as either app-managed egress or an
// explicit boundary such as the native WebView network.
type egressRegistryEntry struct {
	count      int
	routeClass string
	reason     string
}

type productionSourceCorpus struct {
	goSources       map[string]string
	frontendSources map[string]string
	frontendFiles   []string
	err             error
}

var (
	productionSourcesOnce sync.Once
	productionSources     productionSourceCorpus
)

var defaultHTTPClientRegistry = map[string]egressRegistryEntry{
	"internal/app/app_session_account_social.go":                 {count: 4, routeClass: "public-internet", reason: "test seam; production account fetcher receives proxyManager"},
	"internal/app/youtubemusic_app_session_account.go":           {count: 2, routeClass: "public-internet", reason: "test seam; production account fetcher receives proxyManager"},
	"internal/application/browsercdp/detect.go":                  {count: 1, routeClass: "loopback-internal", reason: "CDP readiness probe for an app-launched browser"},
	"internal/application/imagecache/image_cache.go":             {count: 1, routeClass: "public-internet", reason: "constructor test seam; production callers must inject a provider"},
	"internal/application/rss/public_http.go":                    {count: 1, routeClass: "public-untrusted", reason: "base-client test seam wrapped by the SSRF-enforcing transport"},
	"internal/application/rss/service.go":                        {count: 1, routeClass: "public-untrusted", reason: "service test seam; production RSS service receives proxyManager"},
	"internal/application/youtubemusic/client.go":                {count: 1, routeClass: "public-internet", reason: "legacy constructor seam; production client receives proxyManager"},
	"internal/presentation/http/listen_live_catalog_handler.go":  {count: 1, routeClass: "public-internet", reason: "handler test seam; production handler receives proxyManager"},
	"internal/presentation/http/listen_live_preview_handler.go":  {count: 1, routeClass: "public-internet", reason: "handler test seam; production handler receives proxyManager"},
	"internal/presentation/http/listen_live_status_handler.go":   {count: 1, routeClass: "public-internet", reason: "handler test seam; production handler receives proxyManager"},
	"internal/presentation/wails/os_notification_rich_darwin.go": {count: 1, routeClass: "public-internet", reason: "platform test seam; production notification handler receives proxyManager"},
	"internal/presentation/wails/telemetry_handler.go":           {count: 1, routeClass: "public-internet", reason: "handler test seam; production telemetry handler receives proxyManager"},
}

var defaultHTTPTransportRegistry = map[string]egressRegistryEntry{
	"internal/application/rss/public_http.go":      {count: 1, routeClass: "public-untrusted", reason: "cloned only inside the SSRF-enforcing RSS transport"},
	"internal/infrastructure/update/downloader.go": {count: 1, routeClass: "public-internet", reason: "clone fallback; production downloader receives proxyManager"},
}

var constructedHTTPClientRegistry = map[string]egressRegistryEntry{
	"internal/application/dependencies/service/service.go":             {count: 2, routeClass: "public-internet", reason: "constructor fallback; production dependency service receives proxyManager"},
	"internal/application/imagecache/image_cache.go":                   {count: 1, routeClass: "public-internet", reason: "legacy constructor fallback; no production caller currently instantiates it without a provider"},
	"internal/application/library/service/public_network_proxy.go":     {count: 1, routeClass: "public-untrusted", reason: "restricted loopback proxy client with redirect destination guards"},
	"internal/application/library/service/http_credential_boundary.go": {count: 1, routeClass: "public-internet", reason: "credential-scoped redirect client is pinned to the stable gateway URL in production"},
	"internal/application/library/service/resource_preview_lease.go":   {count: 1, routeClass: "public-internet", reason: "timeout-removal fallback; production preview receives the managed base client"},
	"internal/application/library/service/ytdlp_auxiliary.go":          {count: 2, routeClass: "public-internet-or-untrusted", reason: "one fail-closed transport and one provider fallback; production service receives proxyManager"},
	"internal/application/library/service/ytdlp_failure.go":            {count: 1, routeClass: "public-internet", reason: "diagnostic fallback; production service replaces it with proxyManager"},
	"internal/application/youtubemusic/client.go":                      {count: 1, routeClass: "public-internet", reason: "legacy constructor fallback; production client receives proxyManager"},
	"internal/application/youtubeworkspace/innertube_client.go":        {count: 2, routeClass: "public-internet", reason: "legacy constructor fallbacks; production workspace receives proxyManager"},
	"internal/infrastructure/libraryicons/favicon_cache.go":            {count: 2, routeClass: "public-internet", reason: "legacy constructor fallbacks; production cache receives proxyManager"},
	"internal/infrastructure/proxy/gateway.go":                         {count: 2, routeClass: "route-engine", reason: "authoritative gateway consumer and public clients"},
	"internal/infrastructure/proxy/manager.go":                         {count: 1, routeClass: "route-engine", reason: "authoritative upstream route client"},
}

var embeddedWebViewRegistry = map[string]egressRegistryEntry{
	"internal/presentation/wails/connector_app_session.go":         {count: 1, routeClass: "native-webview-system", reason: "interactive authenticated site session uses the platform/runtime proxy policy"},
	"internal/presentation/wails/connector_app_session_windows.go": {count: 1, routeClass: "native-webview-system-or-loopback", reason: "shared-profile cleanup window uses the native WebView environment"},
	"internal/presentation/wails/listen_live_player_handler.go":    {count: 1, routeClass: "native-webview-system", reason: "YouTube live playback and subresources use the platform/runtime proxy policy"},
	"internal/presentation/wails/listen_player_handler.go":         {count: 1, routeClass: "native-webview-system", reason: "YouTube Music playback and subresources use the platform/runtime proxy policy"},
	"internal/presentation/wails/local_media_transport.go":         {count: 1, routeClass: "native-webview-system-or-loopback", reason: "the native WebView resolves local assets and remote HTMLMediaElement sources"},
	"internal/presentation/wails/rss_video_player_handler.go":      {count: 1, routeClass: "native-webview-system", reason: "RSS/Bilibili playback and subresources use the platform/runtime proxy policy"},
	"internal/presentation/wails/rss_site_player_handler.go":       {count: 1, routeClass: "native-webview-system", reason: "interactive RSS site playback and subresources use the platform/runtime proxy policy"},
	"internal/presentation/wails/window_manager.go":                {count: 1, routeClass: "native-webview-system-or-loopback", reason: "the main Wails WebView uses native networking for all DOM resources"},
}

var preparedEmbeddedWebViewRegistry = map[string]egressRegistryEntry{
	"internal/presentation/wails/window_manager.go": {count: 2, routeClass: "native-webview-system-or-loopback", reason: "lazy settings and tray WebViews use native networking after their security policies are installed"},
}

var managedChildBrowserRegistry = map[string]egressRegistryEntry{
	"internal/application/browserprofile/profile.go":         {count: 1, routeClass: "managed-gateway-no-navigation", reason: "read-only browser profile cookie snapshot"},
	"internal/application/library/service/resource_sniff.go": {count: 1, routeClass: "public-internet", reason: "interactive resource-sniff Chromium"},
	"internal/application/pets/service/online_import.go":     {count: 1, routeClass: "public-internet", reason: "interactive online-pet Chromium"},
}

var borrowedCurrentBrowserRegistry = map[string]egressRegistryEntry{
	"internal/application/library/service/resource_sniff.go": {count: 1, routeClass: "loopback-internal", reason: "explicitly authorized attachment to the current Chrome session for resource sniffing"},
	"internal/infrastructure/appsessionprofile/reader.go":    {count: 1, routeClass: "loopback-internal", reason: "explicitly authorized detach-only current Chrome cookie snapshot restricted to the App Session domain allowlist"},
}

var externalUserAgentRegistry = map[string]egressRegistryEntry{
	"internal/presentation/wails/system_handler.go": {count: 1, routeClass: "external-user-agent", reason: "explicit user-visible handoff to the OS default browser"},
}

var frontendExternalUserAgentRegistry = map[string]egressRegistryEntry{
	"frontend/src/app/settings/SettingsApp.tsx": {count: 1, routeClass: "external-user-agent", reason: "fixed mailto contact action; dynamic and HTTP(S) links use SystemHandler.OpenURL"},
}

var directListenerRegistry = map[string]egressRegistryEntry{
	"internal/application/library/service/public_network_proxy.go": {count: 1, routeClass: "loopback-internal", reason: "ephemeral SSRF-guarded loopback proxy for a managed yt-dlp job"},
	"internal/infrastructure/libraryserver/server.go":              {count: 3, routeClass: "library-peer-or-loopback", reason: "one loopback backend and interface-pinned TLS LAN listeners"},
	"internal/infrastructure/proxy/gateway.go":                     {count: 1, routeClass: "route-engine", reason: "authoritative process-lifetime loopback gateway"},
	"internal/infrastructure/ws/server.go":                         {count: 1, routeClass: "loopback-internal", reason: "token-guarded app realtime and local HTTP server"},
}

var defaultResolverRegistry = map[string]egressRegistryEntry{
	"internal/application/browsercdp/session.go":                   {count: 1, routeClass: "public-untrusted", reason: "pre-navigation SSRF validation before the managed browser loads a destination"},
	"internal/application/library/service/public_network_proxy.go": {count: 1, routeClass: "public-untrusted", reason: "DNS rebinding defense immediately before a restricted proxy dial"},
	"internal/application/networkpolicy/public.go":                 {count: 1, routeClass: "public-untrusted", reason: "shared public-address validation fallback"},
	"internal/application/rss/public_http.go":                      {count: 2, routeClass: "public-untrusted", reason: "RSS SSRF validation and dial-time rebinding defense"},
	"internal/application/rss/site_player.go":                      {count: 1, routeClass: "public-untrusted", reason: "pre-navigation DNS validation for interactive RSS site playback; subsequent requests use the native WebView network"},
	"internal/application/rss/service.go":                          {count: 1, routeClass: "public-untrusted", reason: "RSS service resolver used by the same SSRF policy"},
	"internal/infrastructure/proxy/public_route.go":                {count: 1, routeClass: "public-untrusted", reason: "gateway public-route validation before any public dial"},
	"internal/infrastructure/proxy/direct_route.go":                {count: 1, routeClass: "route-engine", reason: "resolve-once and IP-pinned direct route with pre-connect loopback rejection"},
	"internal/infrastructure/proxy/gateway.go":                     {count: 1, routeClass: "route-engine", reason: "injects the authoritative resolver into each immutable gateway generation"},
}

var rawDialerRegistry = map[string]egressRegistryEntry{
	"internal/application/rss/public_http.go":       {count: 1, routeClass: "test-seam-only", reason: "legacy pinning transport is gated by an unexported marker; production requires PublicDialURLContext"},
	"internal/infrastructure/proxy/direct_route.go": {count: 1, routeClass: "route-engine", reason: "dials only DNS-validated pinned IP literals through generation-owned tracking"},
	"internal/infrastructure/proxy/gateway.go":      {count: 1, routeClass: "route-engine", reason: "loopback gateway client; every upstream socket uses the generation-owned pinned dialer"},
}

var resolveTCPAddressRegistry = map[string]egressRegistryEntry{
	"internal/infrastructure/libraryserver/lan_runtime.go": {count: 1, routeClass: "library-peer", reason: "compares an active listener with a prevalidated private interface endpoint"},
	"internal/infrastructure/libraryserver/server.go":      {count: 3, routeClass: "library-peer-or-loopback", reason: "resolves only validated loopback or interface-pinned LAN listener addresses"},
}

var commandRegistry = map[string]egressRegistryEntry{
	"internal/application/browsercdp/process_group_windows.go":      {count: 1, routeClass: "local-process-control", reason: "terminates an already registered browser process tree"},
	"internal/application/browsercdp/runtime.go":                    {count: 1, routeClass: "public-internet", reason: "managed Chromium receives the stable gateway and UDP-bypass suppression arguments"},
	"internal/application/dependencies/service/service.go":          {count: 3, routeClass: "local-tool", reason: "archive inspection/extraction and macOS quarantine removal only"},
	"internal/application/library/service/process_group_windows.go": {count: 1, routeClass: "local-process-control", reason: "terminates an already registered media helper process tree"},
	"internal/application/ytdlp/process_group_windows.go":           {count: 1, routeClass: "local-process-control", reason: "terminates an already registered yt-dlp process tree"},
	"internal/infrastructure/logging/path.go":                       {count: 3, routeClass: "external-user-agent", reason: "opens a validated local log directory in the OS file manager"},
	"internal/infrastructure/opener/opener.go":                      {count: 6, routeClass: "external-user-agent", reason: "validated local path or HTTP(S) OS handoff"},
	"internal/infrastructure/update/process_unix.go":                {count: 1, routeClass: "local-process-control", reason: "starts a prepared local update helper detached"},
	"internal/infrastructure/update/process_windows.go":             {count: 1, routeClass: "local-process-control", reason: "starts a prepared local update helper detached"},
}

var commandContextRegistry = map[string]egressRegistryEntry{
	"internal/application/dependencies/service/service.go":                   {count: 2, routeClass: "local-tool", reason: "installed dependency version probes only"},
	"internal/application/library/service/catalog_video_thumbnail.go":        {count: 1, routeClass: "local-file-tool", reason: "bounded ffmpeg local video thumbnail generation with network protocols denied"},
	"internal/application/library/service/embedded_artwork.go":               {count: 1, routeClass: "local-file-tool", reason: "bounded ffmpeg local embedded-artwork extraction with network protocols denied"},
	"internal/application/library/service/listen_local_content_identity.go":  {count: 1, routeClass: "local-file-tool", reason: "bounded ffprobe audio-packet identity sampling with network protocols denied"},
	"internal/application/library/service/listen_local_metadata.go":          {count: 1, routeClass: "local-file-tool", reason: "ffmpeg local metadata rewrite with network protocols denied"},
	"internal/application/library/service/listen_local_metadata_manifest.go": {count: 1, routeClass: "local-file-tool", reason: "ffprobe local metadata read with network protocols denied"},
	"internal/application/library/service/media_preprocess.go":               {count: 1, routeClass: "local-file-tool", reason: "ffprobe local media inspection with network protocols denied"},
	"internal/application/library/service/transcode_operation.go":            {count: 1, routeClass: "local-file-tool", reason: "ffmpeg local transcode with network protocols denied"},
	"internal/application/library/service/ytdlp_failure.go":                  {count: 1, routeClass: "local-tool", reason: "yt-dlp version probe only; connectivity uses the managed HTTP client"},
	"internal/application/ytdlp/command.go":                                  {count: 1, routeClass: "public-internet", reason: "yt-dlp and its process tree receive gateway-only proxy environment"},
	"internal/application/ytdlp/info.go":                                     {count: 1, routeClass: "public-internet", reason: "yt-dlp metadata process tree receives gateway-only proxy environment"},
	"internal/infrastructure/firewall/firewall.go":                           {count: 1, routeClass: "platform-control-plane", reason: "constrained Windows Private-profile firewall rule management"},
	"internal/infrastructure/tailscale/manager.go":                           {count: 1, routeClass: "platform-control-plane", reason: "local Tailscale CLI control plane"},
	"internal/infrastructure/update/installer.go":                            {count: 2, routeClass: "local-process-control", reason: "local update inspection and archive extraction"},
	"internal/presentation/wails/system_user_profile_darwin.go":              {count: 1, routeClass: "local-os-query", reason: "bounded dscl query for the current user's local profile image"},
}

var goWebSocketServerRegistry = map[string]egressRegistryEntry{
	"internal/infrastructure/ws/server.go": {count: 1, routeClass: "loopback-internal", reason: "token-guarded realtime server on the loopback listener"},
}

var cdpWebSocketRegistry = map[string]egressRegistryEntry{
	"internal/application/browsercdp/current_browser.go": {count: 1, routeClass: "loopback-internal", reason: "CDP attachment to Chrome's consent-gated endpoint after strict path, owner, UUID, port, and 127.0.0.1 validation, followed by a browser-level Chrome version check"},
	"internal/application/browsercdp/runtime.go":         {count: 1, routeClass: "loopback-internal", reason: "CDP attachment to the exact loopback port of the app-launched browser"},
}

var frontendWebSocketRegistry = map[string]egressRegistryEntry{
	"frontend/src/shared/realtime/client.ts": {count: 1, routeClass: "loopback-internal", reason: "realtime URL is returned by the loopback-only RealtimeHandler"},
}

type directRouteException struct {
	name       string
	routeClass string
	path       string
	markers    []string
}

var directRouteExceptionRegistry = []directRouteException{
	{
		name: "current Chrome DevTools endpoint", routeClass: "loopback-internal",
		path: "internal/application/browsercdp/current_browser.go",
		markers: []string{
			"readTrustedCurrentBrowserFile(userDataDir, activePortPath",
			`!ip.Equal(net.ParseIP("127.0.0.1"))`,
			"chromedp.NoModifyURL",
			"browserpkg.GetVersion().Do(cdp.WithExecutor",
		},
	},
	{
		name: "CDP control endpoint", routeClass: "loopback-internal",
		path:    "internal/application/browsercdp/detect.go",
		markers: []string{`host = "127.0.0.1"`, "ip.IsLoopback()", `/json/version`},
	},
	{
		name: "Wails realtime origin", routeClass: "loopback-internal",
		path: "internal/app/bootstrap.go",
		markers: []string{
			`ws.NewServer("127.0.0.1:0", eventBus)`,
		},
	},
	{
		name: "Library Tailscale backend", routeClass: "library-peer",
		path:    "internal/infrastructure/libraryserver/server.go",
		markers: []string{`config.BackendAddress = "127.0.0.1:0"`, "isLoopbackListener(backendListener.Addr())"},
	},
	{
		name: "Library LAN listener", routeClass: "library-peer",
		path:    "internal/infrastructure/libraryserver/server.go",
		markers: []string{"validateLANBindAddress(server.config.LANAddress)", "MinVersion:   tls.VersionTLS13"},
	},
	{
		name: "Tailscale Serve CLI", routeClass: "platform-control-plane",
		path:    "internal/infrastructure/tailscale/manager.go",
		markers: []string{"exec.CommandContext(runCtx, resolved, args...)", `executable == "tailscale"`},
	},
	{
		name: "OS default browser", routeClass: "external-user-agent",
		path:    "internal/infrastructure/opener/opener.go",
		markers: []string{`scheme != "http" && scheme != "https"`, `parsedURL.Host == ""`, `exec.Command("open", trimmedURL)`, `exec.Command("xdg-open", trimmedURL)`, "openURLWindows(trimmedURL)"},
	},
	{
		name: "fixed Settings contact email", routeClass: "external-user-agent",
		path: "frontend/src/app/settings/SettingsApp.tsx",
		markers: []string{
			`const ABOUT_CONTACT_EMAIL_URL = "mailto:xunruhao@gmail.com";`,
			"Browser.OpenURL(ABOUT_CONTACT_EMAIL_URL)",
			`import { openExternalURL, useFontFamilies } from "@/shared/query/system";`,
		},
	},
}

var platformNetworkSurfaceRegistry = []directRouteException{
	{
		name: "native AirPlay route picker", routeClass: "external-playback-target",
		path: "internal/presentation/wails/listen_player_webview_darwin.go",
		markers: []string{
			"configuration.allowsAirPlayForMediaPlayback = YES",
			"AVRoutePickerView *picker",
			"listenShowAirPlayRoutePicker",
		},
	},
	{
		name: "WebKit AirPlay fallback", routeClass: "external-playback-target",
		path: "internal/presentation/wails/listen_player_handler.go",
		markers: []string{
			"showListenNativeAirPlayPicker(player.windows.mainWindow.NativeWindow(), anchor)",
			"listenYouTubeMusicAirPlayScript()",
			"video.webkitShowPlaybackTargetPicker()",
		},
	},
	{
		name: "live-player AirPlay route picker", routeClass: "external-playback-target",
		path: "internal/presentation/wails/listen_live_player_handler.go",
		markers: []string{
			"showListenNativeAirPlayPicker(player.windows.mainWindow.NativeWindow(), anchor)",
		},
	},
	{
		name: "LAN DNS-SD eligibility policy", routeClass: "library-peer-discovery",
		path: "internal/infrastructure/discovery/discovery.go",
		markers: []string{
			`const ServiceType = "_xiadown._tcp"`,
			"iface.Flags&net.FlagMulticast != 0",
			"(!ip.IsPrivate() && !ip.IsLinkLocalUnicast())",
			"excludedInterfaceName",
		},
	},
	{
		name: "macOS DNS-SD registration", routeClass: "library-peer-discovery",
		path: "internal/infrastructure/discovery/discovery_darwin.go",
		markers: []string{
			"DNSServiceRegister(ref, 0, interfaceIndex",
			"C.DNSServiceRefDeallocate(ref)",
		},
	},
	{
		name: "Windows DNS-SD registration", routeClass: "library-peer-discovery",
		path: "internal/infrastructure/discovery/discovery_windows.go",
		markers: []string{
			`dnsapi.NewProc("DnsServiceRegister")`,
			"InterfaceIndex: uint32(interfaceIndex)",
			"procDeregisterService.Call",
		},
	},
	{
		name: "bounded native PAC and WPAD work", routeClass: "proxy-policy-control-plane",
		path: "internal/infrastructure/proxy/system_resolver_common.go",
		markers: []string{
			"const nativeSystemProxyConcurrency = 16",
			"platformSystemProxyURLContext",
			"case <-ctx.Done()",
			"return normalizeSystemProxyCandidate(candidates[0])",
		},
	},
	{
		name: "macOS CFNetwork PAC resolver", routeClass: "proxy-policy-control-plane",
		path: "internal/infrastructure/proxy/system_resolver_darwin.go",
		markers: []string{
			"CFNetworkCopySystemProxySettings()",
			"CFNetworkCopyProxiesForURL(targetURL, settings)",
			"CFNetworkExecuteProxyAutoConfigurationURL",
			"CFNetworkCopyProxiesForAutoConfigurationScript",
		},
	},
	{
		name: "Windows WinHTTP PAC and WPAD resolver", routeClass: "proxy-policy-control-plane",
		path: "internal/infrastructure/proxy/system_windows.go",
		markers: []string{
			`winHTTPGetProxyForURL                 = winHTTPDLL.NewProc("WinHttpGetProxyForUrl")`,
			"winHTTPAutoDetectTypeDHCP | winHTTPAutoDetectTypeDNSA",
			"AutoLogonIfChallenged: 0",
		},
	},
	{
		name: "Linux desktop and portal proxy resolver", routeClass: "proxy-policy-control-plane",
		path: "internal/infrastructure/proxy/system_resolver_linux.go",
		markers: []string{
			"C.g_proxy_resolver_get_default()",
			"C.g_proxy_resolver_lookup(resolver, rawTarget",
			"return firstSystemProxyCandidate(candidates)",
		},
	},
}

func TestProductionDefaultHTTPClientsAreRegistered(t *testing.T) {
	t.Parallel()

	assertProductionTokenRegistry(t, "http.DefaultClient", defaultHTTPClientRegistry)
	assertProductionTokenRegistry(t, "http.DefaultTransport", defaultHTTPTransportRegistry)
	assertProductionTokenRegistry(t, "&http.Client{", constructedHTTPClientRegistry)
	for _, forbidden := range []string{"http.Get(", "http.Post(", "http.PostForm(", "http.Head("} {
		if occurrences := productionGoTokenOccurrences(t, forbidden); len(occurrences) != 0 {
			t.Fatalf("unmanaged convenience HTTP call %q found at %v", forbidden, occurrences)
		}
	}
}

func TestEveryEmbeddedWebViewIsInventoriedAndUsesNativeNetwork(t *testing.T) {
	t.Parallel()

	assertProductionTokenRegistry(t, "Window.NewWithOptions(", embeddedWebViewRegistry)
	assertProductionTokenRegistry(t, "application.NewWindow(", preparedEmbeddedWebViewRegistry)
	assertProductionTokenRegistry(t, "application.New(application.Options{", map[string]egressRegistryEntry{
		"internal/app/bootstrap.go": {count: 1, routeClass: "native-webview-system", reason: "the sole Wails application leaves proxy selection to each platform runtime"},
	})

	bootstrap := readRepoSource(t, "internal/app/bootstrap.go")
	for _, forbidden := range []string{
		"NewWebViewNetworkRoute",
		"WebView2BrowserArguments",
		"application.NewService(webViewNetworkRoute)",
		"AdditionalBrowserArgs",
		"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
		"RegisterInternalLoopbackURL",
	} {
		if strings.Contains(bootstrap, forbidden) {
			t.Fatalf("bootstrap must not inject the internal egress route into native WebViews: %q", forbidden)
		}
	}
}

func TestEveryManagedChildBrowserIsRegisteredAndFailClosed(t *testing.T) {
	t.Parallel()

	assertProductionTokenRegistry(t, "browsercdp.Start(", managedChildBrowserRegistry)
	assertProductionTokenRegistry(t, "browsercdp.StartBorrowedCurrentBrowser(", borrowedCurrentBrowserRegistry)
	if occurrences := productionGoTokenOccurrences(t, "browsercdp.StartLoopbackOnly("); len(occurrences) != 0 {
		t.Fatalf("loopback-only browser entry is not a production remote-browser escape hatch: %v", occurrences)
	}
	for path, gatewayMarker := range map[string]string{
		"internal/application/library/service/resource_sniff.go": "service.managedBrowserNetworkRoute()",
		"internal/application/pets/service/online_import.go":     "service.onlinePetBrowserOptions()",
	} {
		source := readRepoSource(t, path)
		for _, marker := range []string{
			gatewayMarker,
			"NetworkRoute:",
		} {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s is missing managed child-browser marker %q", path, marker)
			}
		}
	}

	launcher := readRepoSource(t, "internal/application/browsercdp/runtime.go")
	if strings.Count(launcher, "exec.Command(candidate.ExecPath, args...)") != 1 {
		t.Fatal("browsercdp must remain the single reviewed child-browser launcher")
	}
	attestation := strings.Index(launcher, "verifyManagedNetworkRoute(browserCtx, options.NetworkRoute, nil)")
	targetManager := strings.Index(launcher, "startPageTargetManager(runtime)")
	visibleTarget := strings.Index(launcher, "createPageTarget(runtime, 10*time.Second, true)")
	if attestation < 0 || targetManager < 0 || visibleTarget < 0 || !(attestation < targetManager && targetManager < visibleTarget) {
		t.Fatalf("managed browser startup order must be attestation -> target manager -> visible target: attestation=%d manager=%d visible=%d", attestation, targetManager, visibleTarget)
	}
	assertSourceMarkers(t, "internal/application/browsercdp/session.go", []string{
		"type SessionOptions struct {",
		"routeSnapshot := *options.NetworkRoute",
		"func sessionLaunchOptions(options SessionOptions) LaunchOptions",
		"runtime, err = Start(startCtx, launchOptions)",
		"runtime, err = StartLoopbackOnly(startCtx, launchOptions)",
	})
}

func TestDirectRouteExceptionsAreExplicitAndNarrow(t *testing.T) {
	t.Parallel()

	assertProductionTokenRegistry(t, "opener.OpenURL(", externalUserAgentRegistry)
	assertProductionFrontendTokenRegistry(t, "Browser.OpenURL(", frontendExternalUserAgentRegistry)
	for _, exception := range directRouteExceptionRegistry {
		if exception.name == "" || exception.routeClass == "" || exception.path == "" || len(exception.markers) == 0 {
			t.Fatalf("incomplete direct-route exception: %#v", exception)
		}
		assertSourceMarkers(t, exception.path, exception.markers)
	}
	assertSourceMarkers(t, "frontend/src/shared/query/system.ts", []string{
		"export function isExternalHTTPURL(url: string): boolean",
		`parsed.protocol === "http:" || parsed.protocol === "https:"`,
		"external URL must use http or https",
		"${SYSTEM_HANDLER_SERVICE}.OpenURL",
	})
	assertSourceMarkers(t, "frontend/src/shared/markdown/dialog-markdown.tsx", []string{
		"isExternalHTTPURL(href)",
		"void openExternalURL(externalURL)",
	})
}

func TestRawSocketAndResolverSurfacesAreRegistered(t *testing.T) {
	t.Parallel()

	assertProductionTokenRegistry(t, "net.Listen(", directListenerRegistry)
	assertProductionTokenRegistry(t, "net.DefaultResolver", defaultResolverRegistry)
	assertProductionTokenRegistry(t, "net.Dialer", rawDialerRegistry)
	assertProductionTokenRegistry(t, "net.ResolveTCPAddr(", resolveTCPAddressRegistry)
	for _, forbidden := range []string{
		"net.Dial(", "net.DialTimeout(", "net.DialTCP(", "net.DialUDP(", "net.DialIP(",
		"net.ListenPacket(", "net.ListenTCP(", "net.ListenUDP(", "net.ListenIP(",
		"net.ListenConfig", "net.Resolver{", "net.ResolveIPAddr(", "net.ResolveUDPAddr(",
		"net.LookupHost(", "net.LookupIP(", "net.LookupAddr(", "net.LookupPort(",
		"net.LookupCNAME(", "net.LookupMX(", "net.LookupNS(", "net.LookupSRV(", "net.LookupTXT(",
		"tls.Dial(", "tls.DialWithDialer(",
	} {
		if occurrences := productionGoTokenOccurrences(t, forbidden); len(occurrences) != 0 {
			t.Errorf("unregistered raw network primitive %q found at %v", forbidden, occurrences)
		}
	}

	assertSourceMarkers(t, "internal/application/networkpolicy/public.go", []string{
		"resolver.LookupIPAddr(ctx",
		"!IsPublicIP(address.IP)",
	})
	assertSourceMarkers(t, "internal/application/library/service/public_network_proxy.go", []string{
		`net.Listen("tcp", "127.0.0.1:0")`,
		"networkpolicy.ResolvePublicIPs(resolveCtx, proxy.resolver, host)",
	})
	assertSourceMarkers(t, "internal/infrastructure/proxy/gateway.go", []string{
		`net.Listen("tcp4", "127.0.0.1:0")`,
	})
}

func TestEveryProcessLaunchIsRegistered(t *testing.T) {
	t.Parallel()

	assertProductionTokenRegistry(t, "exec.Command(", commandRegistry)
	assertProductionTokenRegistry(t, "exec.CommandContext(", commandContextRegistry)
	for _, forbidden := range []string{"os.StartProcess(", "syscall.StartProcess(", "syscall.ForkExec("} {
		if occurrences := productionGoTokenOccurrences(t, forbidden); len(occurrences) != 0 {
			t.Errorf("unregistered process launch primitive %q found at %v", forbidden, occurrences)
		}
	}

	assertSourceMarkers(t, "internal/application/browsercdp/runtime.go", []string{
		"exec.Command(candidate.ExecPath, args...)",
		"appendBrowserLaunchArgs(args, candidate.ID)",
		"ErrManagedNetworkRouteRequired",
		"verifyManagedNetworkRoute(browserCtx, options.NetworkRoute, nil)",
		"targetManager.ExcludeTargetID(string(targetID))",
		"--proxy-bypass-list=<-loopback>",
		"--disable-quic",
		"--dns-prefetch-disable",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--disable-extensions",
		"--no-startup-window",
	})
	assertSourceMarkers(t, "internal/application/ytdlp/command.go", []string{
		"restrictedProxyEnvironment(command.Env, strings.TrimSpace(options.ProxyURL))",
	})
	assertSourceMarkers(t, "internal/application/ytdlp/info.go", []string{
		"restrictedProxyEnvironment(command.Env, proxyURL)",
	})
}

func TestEveryWebSocketSurfaceIsRegisteredAndLoopbackBound(t *testing.T) {
	t.Parallel()

	assertProductionTokenRegistry(t, "websocket.Server{", goWebSocketServerRegistry)
	assertProductionTokenRegistry(t, "chromedp.NewRemoteAllocator(", cdpWebSocketRegistry)
	assertProductionFrontendTokenRegistry(t, "new WebSocket(", frontendWebSocketRegistry)
	for _, forbidden := range []string{"websocket.Dial(", "websocket.DialConfig("} {
		if occurrences := productionGoTokenOccurrences(t, forbidden); len(occurrences) != 0 {
			t.Errorf("unregistered WebSocket client %q found at %v", forbidden, occurrences)
		}
	}

	assertSourceMarkers(t, "internal/app/bootstrap.go", []string{
		`ws.NewServer("127.0.0.1:0", eventBus)`,
	})
	assertSourceMarkers(t, "internal/infrastructure/ws/server.go", []string{
		"server.withAccessGuard(mux)",
		`return fmt.Sprintf("ws://%s%s", server.listener.Addr().String(), localaccess.WebSocketPath(server.token))`,
	})
	assertSourceMarkers(t, "frontend/src/shared/realtime/index.ts", []string{
		"xiadown/internal/presentation/wails.RealtimeHandler.WebSocketURL",
		"new WebSocketClient(url",
	})
	assertSourceMarkers(t, "internal/application/browsercdp/runtime.go", []string{
		`!strings.EqualFold(parsed.Scheme, "ws")`,
		"endpointIP == nil || !endpointIP.IsLoopback()",
		`fmt.Sprintf("ws://127.0.0.1:%d%s", port, websocketEndpoint)`,
	})
}

func TestPlatformNetworkBypassesAreExplicitAndConstrained(t *testing.T) {
	t.Parallel()

	for token, registry := range map[string]map[string]egressRegistryEntry{
		"webkitShowPlaybackTargetPicker": {
			"internal/presentation/wails/listen_player_handler.go": {count: 4, routeClass: "external-playback-target", reason: "user-initiated WebKit AirPlay fallback"},
		},
		"AVRoutePickerView": {
			"internal/presentation/wails/listen_player_webview_darwin.go": {count: 2, routeClass: "external-playback-target", reason: "user-initiated native macOS AirPlay picker"},
		},
		"DNSServiceRegister(": {
			"internal/infrastructure/discovery/discovery_darwin.go": {count: 1, routeClass: "library-peer-discovery", reason: "interface-pinned macOS DNS-SD advertisement"},
		},
		`NewProc("DnsServiceRegister")`: {
			"internal/infrastructure/discovery/discovery_windows.go": {count: 1, routeClass: "library-peer-discovery", reason: "interface-pinned Windows DNS-SD advertisement"},
		},
		"WinHttpGetProxyForUrl": {
			"internal/infrastructure/proxy/system_windows.go": {count: 1, routeClass: "proxy-policy-control-plane", reason: "per-destination Windows PAC/WPAD decision"},
		},
		"CFNetworkExecuteProxyAutoConfigurationURL": {
			"internal/infrastructure/proxy/system_resolver_darwin.go": {count: 1, routeClass: "proxy-policy-control-plane", reason: "per-destination macOS PAC decision"},
		},
		"g_proxy_resolver_lookup": {
			"internal/infrastructure/proxy/system_resolver_linux.go": {count: 1, routeClass: "proxy-policy-control-plane", reason: "per-destination Linux desktop/portal proxy decision"},
		},
	} {
		assertProductionTokenRegistry(t, token, registry)
	}
	for _, surface := range platformNetworkSurfaceRegistry {
		if surface.name == "" || surface.routeClass == "" || surface.path == "" || len(surface.markers) == 0 {
			t.Fatalf("incomplete platform network surface: %#v", surface)
		}
		assertSourceMarkers(t, surface.path, surface.markers)
	}
}

func TestEgressRegistryExcludesTestsAndGeneratedSources(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"generated", "__generated__", "testdata", "__tests__"} {
		if !isNonProductionSourceDirectory(name) {
			t.Errorf("expected %q to be excluded from production source scans", name)
		}
	}
	for _, path := range []string{"client.generated.ts", "bindings_generated.go", "route.gen.tsx"} {
		if !isGeneratedSourceName(path) {
			t.Errorf("expected %q to be recognized as generated", path)
		}
	}
	if !hasGeneratedSourceHeader([]byte("// Code generated by tool. DO NOT EDIT.\n")) {
		t.Fatal("generated source header must be excluded")
	}
	for _, path := range productionFrontendFiles(t) {
		if strings.Contains(path, ".test.") || strings.Contains(path, ".spec.") {
			t.Errorf("test file leaked into production frontend scan: %s", path)
		}
	}
}

func TestProductionEgressProvidersAreInjected(t *testing.T) {
	t.Parallel()

	assertSourceMarkers(t, "internal/app/bootstrap.go", []string{
		"appsessionsservice.WithAccountFetcher(newAppSessionAccountFetcher(proxyManager))",
		"dependenciesservice.WithHTTPClientProvider(proxyManager)",
		"youtubeworkspace.NewService(appSessionsService, proxyManager)",
		"youtubemusic.NewClientWithHTTPClientProvider(appSessionsService, proxyManager)",
		"presentationhttp.NewListenLiveCatalogHandler(proxyManager)",
		"presentationhttp.NewListenLiveStatusHandler(proxyManager)",
		"presentationhttp.NewListenLivePreviewHandler(proxyManager)",
		"petsservice.WithNetworkGateway(proxyManager)",
		"applicationrss.NewService(rssRepository, proxyManager)",
		"applicationrss.NewVideoPlayerService(appSessionsService, proxyManager)",
		"libraryicons.NewFaviconCacheWithHTTPClientProvider(proxyManager)",
		"libraryapi.NewRSSAPI(rssService, proxyManager)",
		"wails.NewOSNotificationHandlerWithHTTPClientProvider(osNotifications, app, proxyManager)",
		"wails.NewTelemetryHandler(telemetryService, windowManager, proxyManager)",
		"infrastructureupdate.NewManifestCatalogProviderWithClientProvider(proxyManager, \"\")",
		"infrastructureupdate.NewHTTPDownloaderWithClientProvider(proxyManager)",
	})

	for path, markers := range map[string][]string{
		"internal/application/library/service/ytdlp_parse.go": {
			"ProxyURL:     service.resolveYTDLPProxy(resolvedURL)",
		},
		"internal/application/library/service/ytdlp_auxiliary.go": {
			"ProxyURL:         service.resolveYTDLPProxyForRequest(request)",
		},
		"internal/application/library/service/ytdlp_job_helpers.go": {
			"startPublicNetworkProxy(ctx, service.proxyClient)",
			"ProxyURL:            service.resolveYTDLPProxyForRequest(request)",
		},
		"internal/application/library/service/ytdlp_failure.go": {
			"appytdlp.HermeticArgs(\"--version\")",
			"command.Env = appytdlp.HermeticEnvironment(os.Environ())",
		},
		"internal/application/library/service/public_network_proxy.go": {
			"PublicDialURLContext(context.Context, string, string, *url.URL)",
			"return proxy.managedDial(ctx, network, logicalAddress, validatedLogicalURL)",
		},
		"internal/application/ytdlp/command.go": {
			"command.Env = hermeticYTDLPEnvironment(os.Environ())",
			"restrictedProxyEnvironment(command.Env, strings.TrimSpace(options.ProxyURL))",
		},
		"internal/application/ytdlp/info.go": {
			"command.Env = hermeticYTDLPEnvironment(os.Environ())",
			"command.Env = restrictedProxyEnvironment(command.Env, proxyURL)",
		},
	} {
		assertSourceMarkers(t, path, markers)
	}
}

func TestRemoteDOMResourceSurfacesRemainInsideManagedWebViews(t *testing.T) {
	t.Parallel()

	// Dynamic media/artwork URLs are intentionally rendered by the main Wails
	// WebView. Its platform/runtime network stack owns redirects, subresources,
	// and proxy selection; this list keeps every remote-capable DOM surface
	// explicit even though it is outside XiaDown's internal egress gateway.
	for path, markers := range map[string][]string{
		"frontend/src/app/media/MediaPreviewSurface.tsx": {"src={mediaUrl}"},
		"frontend/src/app/media/VidstackPreview.tsx":     {"src={playerSource}"},
		"frontend/src/app/rss/RSSRemoteImage.tsx":        {"src={requested ? controlled : undefined}"},
		"frontend/src/app/rss/RSSWorkspacePage.tsx":      {"src={rssReaderVideoEmbedURL(embed)}"},
		"frontend/src/app/rss/RSSWebVideoPlayback.tsx":   {"src={experience.playbackUrl}"},
	} {
		assertSourceMarkers(t, path, markers)
	}

	for _, path := range productionFrontendFiles(t) {
		source := readRepoSource(t, path)
		for _, forbidden := range []string{
			`src="http://`, `src="https://`,
			`fetch("http://`, `fetch("https://`,
			`fetch('http://`, `fetch('https://`,
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains unmanaged literal remote DOM/network source %q", path, forbidden)
			}
		}
	}
}

func assertProductionTokenRegistry(t *testing.T, token string, registry map[string]egressRegistryEntry) {
	t.Helper()
	actual := productionGoTokenOccurrences(t, token)
	for path, count := range actual {
		entry, ok := registry[path]
		if !ok {
			t.Errorf("unregistered production occurrence of %q: %s (%d)", token, path, count)
			continue
		}
		if entry.routeClass == "" || entry.reason == "" {
			t.Errorf("registry entry for %s lacks route class or reason", path)
		}
		if count != entry.count {
			t.Errorf("production occurrence count for %q in %s = %d, registry = %d", token, path, count, entry.count)
		}
	}
	for path, entry := range registry {
		if actual[path] != entry.count {
			t.Errorf("registry occurrence count for %q in %s = %d, source = %d", token, path, entry.count, actual[path])
		}
	}
}

func productionGoTokenOccurrences(t *testing.T, token string) map[string]int {
	t.Helper()
	result := make(map[string]int)
	for path, source := range cachedProductionSources(t).goSources {
		if count := strings.Count(source, token); count > 0 {
			result[path] = count
		}
	}
	return result
}

func productionFrontendTokenOccurrences(t *testing.T, token string) map[string]int {
	t.Helper()
	result := make(map[string]int)
	for path, source := range cachedProductionSources(t).frontendSources {
		if count := strings.Count(source, token); count > 0 {
			result[path] = count
		}
	}
	return result
}

func assertProductionFrontendTokenRegistry(t *testing.T, token string, registry map[string]egressRegistryEntry) {
	t.Helper()
	actual := productionFrontendTokenOccurrences(t, token)
	for path, count := range actual {
		entry, ok := registry[path]
		if !ok {
			t.Errorf("unregistered production frontend occurrence of %q: %s (%d)", token, path, count)
			continue
		}
		if entry.routeClass == "" || entry.reason == "" {
			t.Errorf("frontend registry entry for %s lacks route class or reason", path)
		}
		if count != entry.count {
			t.Errorf("production frontend occurrence count for %q in %s = %d, registry = %d", token, path, count, entry.count)
		}
	}
	for path, entry := range registry {
		if actual[path] != entry.count {
			t.Errorf("frontend registry occurrence count for %q in %s = %d, source = %d", token, path, entry.count, actual[path])
		}
	}
}

func productionFrontendFiles(t *testing.T) []string {
	t.Helper()
	return append([]string(nil), cachedProductionSources(t).frontendFiles...)
}

func cachedProductionSources(t *testing.T) *productionSourceCorpus {
	t.Helper()
	root := repoRoot(t)
	productionSourcesOnce.Do(func() {
		productionSources.goSources, productionSources.err = loadProductionSources(
			root,
			filepath.Join(root, "internal"),
			func(path string) bool {
				return strings.HasSuffix(path, ".go") &&
					!strings.HasSuffix(path, "_test.go") &&
					!isGeneratedSourceName(path)
			},
		)
		if productionSources.err != nil {
			return
		}
		productionSources.frontendSources, productionSources.err = loadProductionSources(
			root,
			filepath.Join(root, "frontend", "src"),
			func(path string) bool {
				return (strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx")) &&
					!strings.Contains(path, ".test.") &&
					!strings.Contains(path, ".spec.") &&
					!isGeneratedSourceName(path)
			},
		)
		if productionSources.err != nil {
			return
		}
		productionSources.frontendFiles = make([]string, 0, len(productionSources.frontendSources))
		for path := range productionSources.frontendSources {
			productionSources.frontendFiles = append(productionSources.frontendFiles, path)
		}
		sort.Strings(productionSources.frontendFiles)
	})
	if productionSources.err != nil {
		t.Fatal(productionSources.err)
	}
	return &productionSources
}

func loadProductionSources(
	root string,
	directory string,
	include func(string) bool,
) (map[string]string, error) {
	result := make(map[string]string)
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if isNonProductionSourceDirectory(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !include(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if hasGeneratedSourceHeader(data) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	return result, err
}

func isNonProductionSourceDirectory(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "generated", "__generated__", "testdata", "__tests__":
		return true
	default:
		return false
	}
}

func isGeneratedSourceName(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.Contains(name, ".generated.") || strings.Contains(name, "_generated.") ||
		strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, ".gen.ts") || strings.HasSuffix(name, ".gen.tsx")
}

func hasGeneratedSourceHeader(data []byte) bool {
	if len(data) > 1024 {
		data = data[:1024]
	}
	header := strings.ToLower(string(data))
	return strings.Contains(header, "code generated") && strings.Contains(header, "do not edit")
}

func assertSourceMarkers(t *testing.T, path string, markers []string) {
	t.Helper()
	source := readRepoSource(t, path)
	for _, marker := range markers {
		if !strings.Contains(source, marker) {
			t.Errorf("%s is missing egress contract marker %q", path, marker)
		}
	}
}

func readRepoSource(t *testing.T, path string) string {
	t.Helper()
	path = filepath.ToSlash(path)
	sources := cachedProductionSources(t)
	if source, ok := sources.goSources[path]; ok {
		return source
	}
	if source, ok := sources.frontendSources[path]; ok {
		return source
	}
	data, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, sourcePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourcePath), "..", ".."))
}
