# XiaDown network egress contract

Status: implementation contract

This document defines the network-routing invariants for every XiaDown desktop
surface. It applies to macOS, Windows, Linux, backend HTTP clients, embedded web
content, helper processes, and future integrations.

“Network consumer” is intentionally broad. It includes backend/API requests;
WebView navigation, redirects and subresources; audio/video streams and range
requests; artwork, avatars and other images; lyrics and metadata lookup; RSS
feeds, articles and media; App Session login; yt-dlp and its descendants;
managed Chromium/CDP; notifications, updates and dependencies; and every child
process which can inherit or create a network connection. A request does not
leave this contract merely because it is initiated by a browser runtime, media
engine, plugin, connector or subprocess rather than Go code.

## 1. Product invariant

Every public-Internet request initiated by XiaDown MUST be attributable to one
active `NetworkPolicy` generation and MUST use the route selected by that
generation.

No component may silently fall back to the operating-system route, a default
HTTP client, a browser-specific proxy decision, or a direct socket after the
App has selected another route. A fallback is permitted only when the policy
explicitly enables it and the UI describes the resulting privacy behavior.

The invariant covers the complete request, including redirects, images,
lyrics, subresources, media ranges, WebSocket reconnects, service workers,
authentication pages, RSS resources, helper descendants, and requests created
after a page was initially loaded.

## 2. Canonical policy

XiaDown owns exactly one immutable policy snapshot at a time:

```text
NetworkPolicy {
  generation
  mode              // none | system | manual
  manual endpoint   // http | https | socks5; gateway tunnels all proxy routes
  credentials       // HTTP Basic or SOCKS5 username/password; secret, never logged
  bypass rules
  timeout policy
  failover policy   // disabled by default
  platform source   // manual, global system, scoped VPN, PAC/WPAD, enterprise
}
```

All consumers receive either the snapshot itself or a stable local gateway
whose routing engine consumes the snapshot. Consumers MUST NOT reinterpret the
Settings DTO independently.

### 2.1 Mode semantics

- `none` means forced direct egress. It does not mean "clear an override and
  return to the system default".
- `system` means the effective native system policy for each canonical origin,
  including manual proxies, bypass lists, scoped interfaces, PAC, and WPAD when
  the platform supports them.
- `manual` means the configured upstream endpoint and credentials. A failed
  upstream MUST NOT fall back to direct unless a future, explicit failover
  setting enables that behavior.

### 2.2 Bypass semantics

Bypass decisions are made in the routing engine, never in a WebView command
line or individual caller. Rules MUST use canonical exact-host, domain-suffix,
IP/CIDR, and optional port matching. Substring matching is forbidden. A match
means “the gateway selects a direct upstream route”; it never authorizes a
consumer to bypass the loopback gateway itself.

SOCKS5, HTTP CONNECT, backend HTTP, WebViews, and helper processes MUST all
apply the same decision to the same destination. App `NoProxy` remains
authority-scoped: it evaluates the normalized lowercase/IDNA logical hostname
and effective port, never path/query, before native system resolution.
URL-owning callers retain their logical URL through validation, redirect
handling, SSRF checks, `Host` and TLS SNI, but path/query data is not an input
to system PAC selection and is never logged.

After route selection, every direct HTTP(S)/WS(S) destination is resolved
locally once, validated, and pinned to at most eight IP literals for the socket
attempt. A trusted/general route selected for an upstream proxy performs the
same local loopback-alias check, then gives the canonical logical hostname to
that proxy for destination DNS. This is necessary when the selected proxy has
the authoritative DNS view and the host resolver returns a filtered or
poisoned non-loopback answer. A local lookup failure still fails closed because
the loopback-alias check could not be completed. `public-untrusted` never
delegates DNS: it rejects private, loopback, link-local, metadata and all other
non-public addresses locally and gives the proxy only validated pinned IP
literals. Logical host identity is retained for HTTP `Host` and TLS SNI in
every mode.

HTTP and HTTPS upstream proxies are always used with CONNECT, including an
ordinary HTTP destination on port 80. Trusted/general HTTP and SOCKS routes
send the canonical logical hostname; literal destinations remain pinned.
`public-untrusted` HTTP and SOCKS routes always send a validated IP literal,
including when a native resolver returns `socks5h`. A failed proxy route never
falls back to direct.

### 2.3 Canonical-origin system PAC semantics

Every `system`-mode surface asks the native resolver about the same canonical
origin, not its resource URL. Canonicalization is normative and occurs before
CFNetwork, WinHTTP, GIO or an environment fallback sees the input:

- `ws` maps to its protocol-equivalent `http` origin and `wss` maps to `https`;
- the scheme is lowercase and the hostname is lowercase IDNA ASCII (or a
  normalized IP literal), with a trailing DNS dot removed;
- user information, resource path, query and fragment are removed; the
  serialized origin has only the root `/` path;
- the default port (`80` for HTTP/WS, `443` for HTTPS/WSS) is omitted and a
  valid non-default port is retained.

For example, `wss://Music.Example:443/socket?id=secret` and every other WSS URL
at that authority resolve policy as `https://music.example/`; an HTTP or WS
origin on port `8080` remains `http://music.example:8080/`.

This is a deliberate cross-surface security/consistency choice. Without TLS
MITM, an HTTPS/WSS WebView or managed browser reveals only CONNECT authority,
not the encrypted resource path. Giving an API, RSS or yt-dlp request a more
specific PAC input would let API browsing and WebView/media listening select
different networks for the same origin. XiaDown therefore uses canonical
origin PAC inputs for API clients, WebViews, media/images/lyrics/RSS,
yt-dlp/descendants, managed Chromium and Settings probes alike.

Path-sensitive PAC routing is unsupported and XiaDown cannot automatically
detect that a PAC script intended to distinguish paths: the same script may
legitimately return a route for the canonical root. A managed environment that
depends on path-specific routing must use `manual` mode or have its
administrator provide an origin-scoped PAC policy. XiaDown does not inspect
TLS, guess a path route, or allow different surfaces to diverge silently.

## 3. Route classes and explicit exceptions

Every outbound surface declares one of these route classes:

| Route class | Required behavior |
| --- | --- |
| `public-internet` | Must use the active XiaDown policy. |
| `public-untrusted` | Must use the active policy plus DNS rebinding/SSRF destination guards. |
| `loopback-internal` | Must connect directly and only to XiaDown-owned loopback endpoints. |
| `local-file-tool` | Must not use network egress; helper protocols, timeouts, environment and inputs are constrained to local media operations. |
| `library-peer` | Direct LAN/Tailscale route after peer identity and destination validation; never inherited accidentally from `NoProxy`. |
| `library-peer-discovery` | Direct, local-scope discovery such as mDNS. It may advertise or discover paired-library endpoints only and must never transport public content. |
| `external-playback-target` | Explicit user-selected LAN playback target such as AirPlay. Discovery and media delivery remain outside an HTTP proxy and require a visible device boundary. |
| `external-user-agent` | Opens the user's independent browser or OS application. XiaDown cannot control its route and must make that boundary explicit. |
| `platform-control-plane` | OS/runtime traffic XiaDown cannot configure, such as WebView runtime updates or certificate-control traffic. It must be documented and cannot carry XiaDown content payloads. |
| `proxy-policy-control-plane` | Native PAC/WPAD, VPN and desktop proxy resolution. It may discover a route, but must never fetch XiaDown content or become an implicit content fallback. |

Inbound Library API traffic is not egress. Responses which fetch a remote
resource on behalf of a caller are egress and use `public-untrusted`.

## 4. Stable loopback gateway

The normative desktop architecture is a process-lifetime HTTP CONNECT gateway
bound to an ephemeral loopback port:

```text
Settings
   -> NetworkPolicy generation
      -> route engine
         <- API, image, lyrics, RSS and media Go HTTP clients
         <- WKWebView / WebView2 / WebKitGTK
         <- yt-dlp, ffmpeg and helper processes
         <- managed Chromium/CDP sessions
```

The gateway endpoint remains stable for the App process. A settings change
atomically swaps the route engine generation and synchronously cancels the old
generation before `Apply` returns. It closes direct/proxy sockets, incomplete
DNS/CONNECT/auth/TLS work, active tunnels and idle transports owned by that
generation. WebViews and helpers therefore do not need to replace their
profile, cookie store, or proxy endpoint when the upstream changes, and an old
connection cannot silently continue on a retired policy.

The gateway MUST:

- bind only to loopback and close on App shutdown;
- reject every unregistered loopback target (including hostnames which resolve
  to loopback) before TCP connect, and allow only exact XiaDown-owned listener
  authorities;
- force registered `loopback-internal` authorities direct independently of
  user bypass rules or the active manual/system proxy, so local media tokens
  and paths can never be disclosed upstream;
- reject link-local, private, metadata, and otherwise special-use destinations
  for `public-untrusted` traffic before connecting;
- support HTTP, HTTPS CONNECT, WS/WSS, range requests, HTTP upstream proxies,
  HTTPS upstream proxies, and authenticated SOCKS5;
- preserve the logical authority for App `NoProxy`, use the canonical origin
  defined in section 2.3 for every system PAC decision, then apply the route-
  class DNS policy from section 2.2;
- use HTTP CONNECT even for proxied HTTP port 80; trusted/general routes pass
  the canonical hostname while `public-untrusted` passes only pinned IP
  literals to HTTP/HTTPS/SOCKS upstreams;
- keep upstream credentials out of browser arguments, child-process listings,
  events, diagnostics, and logs;
- intercept every path at a process-random
  `<random>.attest.xiadown.invalid` control authority and never forward that
  authority; managed Chromium must complete a short-lived, HMAC-authenticated,
  one-shot HTTPS CONNECT challenge and an HTTP completion request before any
  public navigation, with the final secret exposed only to CDP response
  metadata and never page CORS;
- cap concurrent connections and header sizes and protect against slow clients;
- disable or contain non-proxied UDP transports such as QUIC/WebTransport and
  WebRTC unless a platform adapter can prove they use the selected policy;
- attach surface, generation, destination host, route kind, stage, and a
  redacted error to diagnostics without recording cookies, authorization,
  query strings, or media signatures.

A random port is not authentication. Platform adapters SHOULD authenticate to
the loopback gateway where the WebView API can supply ephemeral proxy
credentials. Until every adapter supports that, the public-destination guard,
loopback-only binding, short lifetime, and resource limits are mandatory and
the residual same-user-process threat must remain documented.

The two-channel managed-Chromium proof has a deliberately narrow meaning: it
proves that both the browser's HTTP request and its HTTPS CONNECT attempt
entered this exact XiaDown gateway. The CONNECT is intentionally rejected by
the gateway after observation. The proof does **not** show that the selected
manual/system upstream proxy, destination DNS, destination TLS, YouTube, RSS,
or media delivery is reachable. Feature health and Settings diagnostics must
therefore run separate real-destination probes through the active generation;
a valid route proof must never be presented as “Internet/proxy works”.

## 5. Platform adapters

### 5.1 macOS

On macOS 14 and later, every XiaDown `WKWebsiteDataStore` that may load remote
content MUST receive the stable gateway through
`WKWebsiteDataStore.proxyConfigurations` before its first remote navigation.
The gateway, not an empty configuration, expresses `none`, because clearing
`proxyConfigurations` resumes the system route.

The main window, Settings, App Session login, YouTube Music, YouTube Live, RSS
video, and any future remote WebView must be covered. Wails App assets use an
in-process custom-scheme handler and require no hostname or loopback network
bypass. Cookie/data stores remain stable across policy changes.

XiaDown's strict-routing desktop build, native objects, and application bundle
all have a macOS 14.0 minimum deployment target. The WebKit proxy API is
therefore an unconditional part of the supported macOS runtime contract.

In `system` mode, the macOS route resolver passes the canonical origin from
section 2.3 to public CFNetwork APIs. `CFNetworkCopySystemProxySettings` and
`CFNetworkCopyProxiesForURL` cover global/scoped settings, exceptions, PAC URLs
and inline PAC JavaScript; PAC execution also receives that same origin. A
native resolution error is an App route error, not permission to use Go's or
WebKit's ambient system route.

### 5.2 Windows

The gateway starts before the first WebView2 environment. Wails configures the
shared environment with a fixed `--proxy-server` argument and
`--proxy-bypass-list=<-loopback>`, which subtracts Chromium's implicit direct
loopback bypass. Wails assets are served in-process and need no network
exception. User bypass rules are not copied to Chromium; the gateway evaluates
them per request. The shared environment also disables QUIC, non-proxied WebRTC
UDP and speculative DNS prefetch. The proxy endpoint is a literal loopback
address, so resolving page-owned names outside the gateway is neither required
nor accepted as a compatibility fallback.

Every Windows WebView2 which can host a remote document or iframe installs a
persistent native `NewWindowRequested` handler; this includes the local main,
settings, tray and media-transport documents. Music, Live, RSS and App Session
also install `NavigationStarting` before their first remote `SetURL`. Music is
restricted to valid `music.youtube.com/watch` video routes; the Live player is
restricted to the requested `www.youtube.com/watch` or `/embed` video identity;
the optimized RSS player is locked to the exact prepared Bilibili video; the
interactive RSS site fallback is locked to the URL-derived Site Policy domain
group, or to the initial eTLD+1 for an unknown site; and App Session allows
HTTPS top-level redirects needed by authentication but rejects credentials,
non-default ports and non-HTTPS schemes. The fallback's initial hostname is
also resolved through the public-address policy before navigation, while the
global gateway resolves and pins every subsequent connection. Every popup is
synchronously marked handled without supplying a new WebView. Failure to
install either required handler leaves a remote window on its local blank
document.

Proxy changes do not change browser arguments or the User Data Folder. This
preserves cookies and avoids rebuilding all shared WebView2 environments.

Before `application.New`, the adapter checks both the current
`Software\Policies\Microsoft\Edge\WebView2` Loader policy layout and the legacy
`Software\Policies\Microsoft\EmbeddedBrowserWebView\LoaderOverride\{AppId}`
layout under HKLM and HKCU. Each follows its documented AUMID,
executable-name, then wildcard precedence; the legacy layout resolves that
precedence for the whole AppId key. Any applicable non-empty runtime-folder,
non-default release-channel, browser-argument, or user-data-folder override
fails startup instead of being silently masked. It then publishes XiaDown's
exact arguments in `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`, and Wails publishes
the same programmatic value again while creating the shared environment. The
adapter also rejects host-command `--edge-webview-switches` and proxy switches,
because WebView2 processes those late and duplicate switches use the last
value.

The current Wails WebView2 loader replaces inherited `WEBVIEW2_*` environment
variables from package initialization, before XiaDown application code can
record their original values. XiaDown therefore cannot claim detection of an
inherited value with this dependency version. Registry policy is inspected
directly before it is masked, host command-line overrides are rejected, and the
programmatic proxy arguments remain authoritative. A Wails upgrade must expose
a pre-initialization inspection hook before this inherited-environment boundary
can be removed; the source contract keeps this limitation explicit.

The Windows `system` resolver uses WinHTTP plus the current-user Internet proxy
configuration per canonical origin and covers manual proxy, ProxyOverride,
PAC, WPAD, and active connection settings.
One WinHTTP resolver session is retained per XiaDown policy generation so PAC
downloads and WPAD discovery are reused. XiaDown does not add a per-host result
cache: each request class asks with its canonical origin, while all paths at
that origin intentionally receive the same PAC input. Native precedence is
WPAD, explicit PAC, static proxy, then DIRECT. Calls to
`WinHttpGetProxyForUrl` are serialized on that generation's session handle.
Retiring a generation prevents new acquisitions immediately, lets already
acquired and queued calls finish without a lock cycle, and closes the native
handle exactly once after the final release.
PAC/WPAD lookup never opts into automatic forwarding of the signed-in user's
NTLM/Negotiate credentials. An authenticated enterprise proxy which the
gateway cannot negotiate fails closed with a visible `proxy-auth` result; it
does not retry directly.

### 5.3 Linux

Every WebKitGTK network session/context that can load remote content MUST be
configured with the stable gateway before its first remote request. The
adapter must use the public API available in the packaged WebKitGTK version and
keep the WebsiteDataManager/profile stable across policy changes.

The `system` resolver passes the canonical origin from section 2.3 to GIO's
default `GProxyResolver`, which selects the desktop implementation (for example
GNOME or libproxy/PAC) and the Flatpak portal resolver when sandboxed. It
commits to the resolver's first ordered proxy/direct result; it does not skip a
malformed or unsupported first result to infer DIRECT. If no supported decision
is returned, it fails closed. If the runtime lacks the required WebKitGTK API,
remote WebViews use the same fail-closed/visible-unsupported rule as legacy
macOS.

### 5.4 Cross-platform capability matrix

| Capability | macOS | Windows | Linux |
| --- | --- | --- | --- |
| Embedded HTTP/HTTPS/WS/WSS | Stable gateway through `WKWebsiteDataStore` | Stable gateway through shared WebView2 environment | Stable gateway through the default WebKit network session/context |
| QUIC/HTTP3 | Apple exposes no public WKWebView-wide QUIC/WebTransport kill switch; every script-capable remote WKWebView remains an explicit residual, with RSS separately constrained to the exact optimized video or URL-derived fallback site scope | Disabled with `--disable-quic` | HTTP(S) remains on the custom WebKit network session; a future page-owned UDP transport must be disabled or fail closed |
| WebRTC direct UDP | No public WebKit-wide disable API; RSS, App Session and Music/Live remote documents remain reviewed script-capable residuals | `disable_non_proxied_udp` | WebRTC and MediaStream are disabled on the discovered Wails shell WebView through public WebKit settings; any future or separately constructed remote WebView must apply the same settings or remain a documented residual |
| Automatic popups | Disabled on the App shell and both RSS playback surfaces; other script-capable remote WKWebViews remain documented residuals | Every remote WebView2 synchronously handles and denies `NewWindowRequested`; player top-level navigation is allowlisted | Disabled on the App shell through public WebKit settings |
| Remote page permissions | Media capture is denied by a forwarding `WKUIDelegate`; geolocation is also denied where the public SDK/runtime callback exists. Other future WebKit capabilities remain part of the explicit residual | Camera, microphone, geolocation, notifications, clipboard-read, sensors and unknown permission kinds are denied before creation | Camera and microphone are denied explicitly; every other unhandled WebKitGTK permission request retains its native deny behavior |
| Managed Chromium | Gateway required; route-attested at launch, before controlled public navigation and every 10 seconds; extensions, QUIC and non-proxied WebRTC disabled | Same | Same |
| yt-dlp and remote media helpers | Gateway in arguments and sanitized child environment; only reviewed HTTP(S)-family inputs | Same | Same |
| Local ffprobe/ffmpeg | Network protocols denied; local protocol allowlist, bounded timeouts and sanitized environment | Same | Same |

The matrix describes content-bearing traffic. Native DNS, certificate
revocation, captive-portal detection, WebView runtime update checks, and other
OS control-plane traffic are not claimed as gateway traffic and cannot be used
to carry XiaDown request payloads.

Every generic child-process proxy template sets `HTTP_PROXY`, `HTTPS_PROXY`
and `ALL_PROXY` to the gateway and explicitly clears both `NO_PROXY` spellings.
Listing loopback in `NO_PROXY` is forbidden because that variable describes
the destination, not the proxy socket, and would skip the exact internal-target
registry. Inherited proxy variables are removed case-insensitively. If the
gateway is unavailable, templates point all proxy variables at the guaranteed-
dead local endpoint `127.0.0.1:1`; they never omit the variables and accidentally
grant direct access. More restrictive helpers such as local ffmpeg clear all
proxy variables because their route class forbids network entirely.

### 5.5 Proxy authentication and DNS

| Policy | Route and authentication behavior | Destination resolution and socket target |
| --- | --- | --- |
| `none` | Forced direct; it never means “inherit ambient system proxy” | Resolve locally, reject loopback aliases, pin up to eight validated IPs and dial only those literals |
| manual HTTP/HTTPS | TLS is used to an HTTPS proxy. Every destination uses CONNECT, including HTTP port 80. Explicit Basic proxy credentials are sent on CONNECT when configured and never enter WebView arguments or logs | Trusted/general hostnames pass a local loopback-alias check, then the proxy resolves the canonical logical hostname. Literal and `public-untrusted` targets stay locally validated and pinned. |
| manual SOCKS5 (and a normalized system `socks5h` candidate) | With credentials, advertise only SOCKS5 username/password and reject an unauthenticated downgrade; without credentials, use no-auth. Never fall back directly | Trusted/general hostnames use SOCKS domain-name addressing after the local loopback-alias check. Literal and `public-untrusted` targets use a locally validated pinned IP. |
| `system` | CFNetwork (macOS), WinHTTP/current-user Internet settings (Windows), or GIO (Linux) selects DIRECT/HTTP(S)/SOCKS per canonical origin. Native credentials are used only when explicitly returned; unsupported integrated challenges and Windows SOCKS4/4a fail closed | After native route selection, apply the same route-class DNS behavior as manual routes: proxy DNS for trusted/general hostnames and local public-IP pinning for `public-untrusted`. |
| `public-untrusted` modifier | Uses the same active `none`/`system`/`manual` policy and authentication | Resolve and pin up to eight **public** addresses locally; reject private, loopback, link-local, metadata, special-use, NAT64, Teredo and 6to4 targets before either direct or proxied connect, then race only validated candidates |

### 5.6 Session credential control plane

Credential persistence must not become an undeclared network outage. A normal
browse, playback, lyric or download request MUST NOT synchronously wait for an
interactive OS credential prompt. Only an explicit user-initiated connect,
sign-in or credential-management action may ask for authentication.

On macOS, the shared `WKWebsiteDataStore` is the authoritative live and durable
YouTube App Session cookie source. Startup and request-time hydration seed the
in-process request cache directly from WebKit; the legacy Keychain snapshot is
not read or written on this hot path, routine WebKit-to-API synchronization
never writes Keychain, and any compatibility read is performed with UI
interaction disabled. WebKit observations participate in the same epoch and
sequence ordering as explicit Save, Clear and live sync, so a delayed snapshot
cannot roll the API cache back. A successful empty WebKit observation and an
explicitly disconnected session are cached as authoritative empty states; they
cannot fall through to an older Keychain item. A WebKit infrastructure failure
may retain an already validated runtime cache, but a missing cache fails closed
with the hydration error instead of reading Keychain. An ad-hoc development
signature change therefore cannot leave the cache permanently `loading` while
otherwise valid WebKit cookies are available.

Windows and Linux implementations must provide the same observable rule: an OS
credential-store read is bounded, coalesced and non-interactive on request
paths; a failure uses an already validated runtime snapshot or returns a typed
session error. It must never silently switch routes, use browser-imported
cookies from a different profile, or hold all network callers until their HTTP
contexts expire.

## 6. Covered surfaces

The implementation inventory and tests MUST include at least:

- YouTube and YouTube Music browse/search/library/lyrics APIs;
- main-window avatars, artwork, thumbnails and other remote DOM resources;
- YouTube Music, YouTube Live and RSS/Bilibili playback WebViews;
- App Session login, cookie hydration and account probes;
- RSS discovery, feed fetch, article resources and media previews;
- image cache, favicon, notifications, online pets, telemetry and updates;
- dependency downloads and manifests;
- yt-dlp parsing/downloads, subtitle/thumbnail fetches, ffmpeg network inputs,
  and all inherited child-process environments;
- managed Chromium/CDP sniffing, including every subresource rather than only
  the top-level URL;
- any connector/plugin HTTP boundary added later.

Opening a URL in the user's default browser is `external-user-agent`, not proof
that the App proxy is broken. The UI and diagnostics must distinguish it from
an embedded App Session.

### 6.1 Audited production registry

The executable registry is maintained by
`internal/app/network_egress_registry_test.go`. It records every production
use of a default HTTP transport, embedded WebView constructor and managed
Chromium launcher. Counts are intentional: adding one of these low-level APIs
fails CI until its route class and wiring are reviewed.

| Surface | Route class | Authoritative wiring |
| --- | --- | --- |
| YouTube Music/workspace, lyrics and account APIs | `public-internet` | `proxyManager.HTTPClient()` |
| App Session account and avatar probes | `public-internet` | account fetcher receives `proxyManager` |
| live catalog/status/preview, telemetry, notifications, dependencies and updates | `public-internet` | each production constructor receives `proxyManager` |
| RSS feeds, remote images/media and paired-device resource relay | `public-untrusted` | `PublicDialURLContext` managed route plus redirect, DNS-rebinding and destination guards; system PAC still receives only the canonical origin |
| main UI DOM artwork/media, App Session, Music/Live/RSS players | `public-internet` | one global WKWebView/WebView2/WebKitGTK gateway route |
| resource sniff and online-pet Chromium | `public-internet` | centrally owned loopback gateway arguments; caller overrides rejected; process-random route attestation required before public navigation; extensions, QUIC and non-proxied WebRTC disabled |
| current-Chrome App Session sync | `loopback-internal` only | explicit Chrome consent bridge; one approved BrowserContext, existing-tab detach-only attachment and allowlisted `Network.getCookies` URLs; it never navigates or launches a public-network child browser |
| yt-dlp, subtitles, thumbnails and spawned helpers | `public-internet` or `public-untrusted` | gateway/restricted gateway in both command arguments and inherited proxy environment; user configs, plugin directories, Python injection and `--exec` are disabled |
| local ffprobe/ffmpeg metadata and transcode tools | `local-file-tool` | local/pipe/crypto/data protocol allowlist, network protocol denylist, bounded I/O/probe timeouts and a proxy-free sanitized environment; local playlists cannot fetch remote segments |
| direct resource downloader | `public-internet` | stable gateway URL supplied by `LibraryService` |

The following are the only reviewed non-gateway boundaries:

| Exception | Route class | Scope |
| --- | --- | --- |
| Wails asset host, realtime server, local-media asset server and CDP control endpoint | `loopback-internal` | exact loopback endpoints only; never a user `NoProxy` expansion |
| Library LAN HTTPS listeners | inbound, not egress | selected physical-interface listeners serving paired clients |
| Tailscale Serve backend and CLI | `library-peer` / `platform-control-plane` | loopback backend plus an explicit managed route; it does not transport arbitrary App public requests |
| mDNS library discovery | `library-peer-discovery` | local-scope UDP discovery only; no public payload fetch |
| AirPlay and other selected receiver protocols | `external-playback-target` | explicit user-selected LAN target; never presented as App-proxy traffic |
| `SystemHandler.OpenURL` through the OS opener | `external-user-agent` | user-visible handoff to the independent default browser; never used as an embedded fetch mechanism |
| PAC/WPAD/native proxy discovery | `proxy-policy-control-plane` | route selection only; never a content transport or fallback |

Tailscale route recovery starts only after the native application has finished
launching and never blocks window creation. Each CLI process has a 10-second
hard deadline plus bounded process-pipe cleanup; the surrounding reconciler
also has a bounded attempt context. Authorization and route-ownership failures
remain visible in Library Access status and require an explicit user action
instead of being retried continuously in the background.

Frontend `fetch` calls are limited to XiaDown-owned local HTTP origins. Dynamic
remote `<img>`, `<audio>` and `<video>` sources are permitted only inside a
registered managed WebView; literal remote DOM sources fail the source guard.

### 6.2 RSS public transport

RSS remote resources retain each validated logical URL through redirects and
pass it to `PublicDialURLContext`. This preserves the authority match, SSRF
validation, logical `Host`/SNI and pinned-socket relationship; it does not make
path/query data part of PAC. In `system` mode the manager still reduces that
logical URL to the same canonical origin used by WebViews, yt-dlp, API clients
and Settings probes.

RSS owns stricter phase and header limits even when the managed App route is
the dialer: dial is capped at 10 seconds, TLS handshake at 10 seconds, response
headers at 20 seconds, and response-header bytes at 128 KiB. These caps are
applied directly at the managed dial and transport boundary. Body lifetimes
remain handler-specific so range media is not silently cut off by a
client-wide timeout.

Production RSS feed and resource clients require `PublicDialURLContext`; a
missing or merely generic HTTP client provider fails before any socket is
opened. The older pinned transport remains reachable only through an
unexported package-test marker so its SSRF and proxy edge cases can be tested.
Even in that test seam, authenticated SOCKS fails before the handshake; it
never drops credentials, delegates destination DNS to the proxy, or retries
directly.

## 7. Policy changes

A successful settings update follows this order:

1. validate and compile the complete candidate policy and construct its route
   state/HTTP clients without mutating the active generation;
2. atomically publish the new generation and replace the manager's client
   handles;
3. keep every WebView and child-process template pointed at the same stable
   gateway URL, whose active state now resolves the new policy;
4. synchronously cancel the old generation, close every tracked direct/proxy
   socket (including incomplete CONNECT/auth/TLS handshakes), then close idle
   transports and active tunnels;
5. retry or reload affected playback surfaces without replacing persistent
   cookie/profile storage;
6. publish one redacted route-change event.

Candidate construction validates configuration and native resolver setup; it
does not prove that a destination or upstream proxy is reachable. The explicit
Settings network test is a separate, non-mutating operation and its result must
not be conflated with generation publication.

Work which completed before publication may finish normally. Anything still
holding the retired generation is cancelled and has its tracked socket/tunnel
closed synchronously before `Apply` returns, with an explicit route-change or
cancellation error. It must never finish later on the old route or migrate
silently midway to the new one.

Native `system` decisions are requested for each new route using its canonical
origin and therefore observe current OS policy on new requests. The Settings
refresh action additionally publishes a new XiaDown generation and closes
existing tunnels. XiaDown does not currently subscribe to every platform's
proxy-change notification; until that watcher exists, an already-open tunnel
changes only after explicit refresh or another network-settings update.

## 8. Diagnostics and UI

Settings exposes the effective policy source and generation, not only the
configured host. Diagnostics provide separate probes for backend HTTP and each
embedded-browser platform through the same gateway. A green status requires
both route agreement and a destination relevant to the feature being tested;
`gstatic generate_204` alone is insufficient for YouTube playback.

Managed-Chromium HTTP/CONNECT attestation is reported separately as
“gateway route observed”. It is not an upstream connectivity result and cannot
make the overall proxy, Internet, YouTube, lyrics or media status green by
itself.

User-facing failures distinguish at least DNS, direct dial, local gateway,
upstream proxy connect/authentication, TLS, HTTP status, WebView navigation,
media readiness, and organization-policy override.

Every gateway response exposes a redacted network generation and, on failure,
an error class. Throttled structured logs record only generation, surface,
route kind, stage, destination host and error class. They must never record URL
queries, cookies, authorization headers, media signatures, usernames or
passwords.

The gateway accepts at most 256 simultaneous loopback connections, limits
request headers to 64 KiB, applies a 15-second header timeout, cancels streaming
responses and tunnels when their generation is replaced, and rejects a route
which resolves back to the gateway itself. Direct hostname routes resolve once,
reject a loopback result before any connection side effect, and dial only the
validated pinned IP literals while retaining a post-connect rebinding guard.
Trusted/general proxied hostname routes also reject a locally observed loopback
alias before delegating canonical hostname resolution to the selected proxy.
Upstream HTTP CONNECT response headers are also bounded at 64 KiB.

## 9. Enforcement and tests

CI must contain:

- route-engine contract tests for every mode, protocol, auth method, bypass
  rule, redirect, PAC result, DNS outcome, timeout and failover decision;
- canonical-origin PAC tests covering HTTP/HTTPS/WS/WSS equivalence, IDNA and
  case normalization, default/non-default ports, stripped credentials/path/
  query/fragment, and identical decisions for API, CONNECT, yt-dlp, RSS and
  Settings inputs at one origin;
- gateway tests for HTTP, CONNECT, WS/WSS, media ranges, cancellation, limits,
  credential redaction and generation changes;
- bootstrap wiring tests proving every production HTTP provider, WebView and
  helper receives the gateway/policy;
- source guards for new uses of `http.DefaultClient`, naked remote WebViews,
  unmanaged remote DOM URLs, and child processes without an egress class;
- macOS, Windows and Linux compile gates plus platform adapter source tests;
- end-to-end probes which observe the same generation and route for Go,
  embedded WebView and helper-process requests;
- cookie/profile persistence tests across policy changes;
- negative tests proving a failed proxy does not silently reach the
  destination directly;
- RSS tests proving the 10-second dial, 10-second TLS, 20-second response-header
  and 128-KiB header caps survive managed-route wrapping, and that legacy
  authenticated SOCKS fails before a handshake rather than downgrading.

### 9.1 Build provenance for runtime validation

Runtime evidence is valid only when it comes from a package built from the
current checkout **after** the relevant network changes and test gates. The
artifact path, build time and source revision/worktree state must be recorded
with the result; an icon, bundle identifier, running process name or installed
version string is not sufficient evidence of freshness.

Interactive validation on every platform must use the freshly generated
workspace artifact rather than a system-installed or previously built copy.
Compile, source-guard and unit-test success on one operating system is not
runtime parity: native evidence must cover CFNetwork/WKWebView on macOS,
WinHTTP/WebView2 on Windows, and GIO/WebKitGTK on Linux, including real policy
changes and failure paths.

## 10. Explicit residual boundaries

The following limitations are visible and fail closed where applicable; none
may be described as fully proxy-controlled:

- the ephemeral loopback gateway has no per-client authentication because the
  three WebView adapters cannot all attach proxy credentials; loopback-only
  binding, an exact app-owned target registry, DNS-to-loopback rejection,
  destination guards, connection limits and process lifetime reduce but do not
  remove the same-user local-process threat;
- proxy secrets are still persisted by the existing Settings repository and
  require a separate secure-storage migration; they are already excluded from
  browser arguments, child command lines, diagnostics and frontend system-
  proxy DTOs;
- Windows integrated NTLM/Negotiate proxy authentication is unsupported and
  returns `proxy-auth` rather than silently enabling automatic logon or direct
  fallback;
- `public-untrusted` locally pinned destination IPs and CONNECT for HTTP port
  80 are intentional security choices. They can reject a remote-only DNS name
  or a proxy which forbids CONNECT-to-80; compatibility fallback to proxy DNS,
  forward-proxy absolute-form requests or direct egress is not implemented for
  that route class. Trusted/general proxy routes do delegate canonical hostname
  DNS after a local loopback-alias check, so callers must not misclassify an
  attacker-controlled URL as trusted/general. The selected proxy can still see
  a different DNS answer after that check; this is an explicit residual for the
  trusted/general route class, not a relaxation of `public-untrusted`;
- system PAC is deliberately origin-scoped on every surface because encrypted
  WebView/Chromium traffic cannot expose a resource path without TLS MITM.
  Path-sensitive PAC is unsupported and cannot be detected reliably; managed
  deployments which require it need `manual` mode or an administrator-provided
  origin-scoped PAC;
- Apple exposes no public WKWebView-wide WebRTC/WebTransport kill switch. RSS
  playback pages are script-capable even though top-level navigation is locked
  to the exact optimized video or URL-derived fallback site scope and popups
  are denied; App Session and Music/Live documents are also reviewed capability
  boundaries rather than fully proxy-controlled UDP surfaces;
- Linux disables WebRTC and MediaStream on the discovered Wails shell WebView,
  but does not yet have interactive CI proving those settings for every future
  independently constructed WebKitGTK view; such a view must be configured or
  treated as unsupported before it can load remote content;
- managed Chromium rejects caller-owned route switches, disables extensions,
  performs an HTTP plus one-shot HTTPS CONNECT gateway challenge at launch and
  before controlled navigation, reads the secret response through CDP rather
  than page JavaScript, repeats it every ten seconds, and terminates on
  mismatch. This proves gateway entry only. A browser policy can still change
  between probes; eliminating that final interval requires OS-level
  per-process egress enforcement on each platform;
- external browsers, AirPlay/receiver protocols, mDNS/Tailscale discovery and
  operating-system control-plane requests are explicitly outside the App HTTP
  proxy and cannot be used as hidden content-fetch fallbacks.

## 11. Completion criteria

The network-unification goal is complete only when:

1. the inventory contains no undeclared production egress;
2. all supported desktop platforms satisfy this contract or visibly fail
   closed where the native runtime cannot;
3. all route modes pass the cross-surface matrix;
4. changing a policy does not lose App Session cookies or require an App
   restart;
5. the latest source build passes unit, integration and platform compile gates;
6. fresh native macOS, Windows and Linux artifacts verify their WebView,
   system-policy, browse, artwork, login, playback, RSS media, helper-download
   and failure paths before cross-platform runtime parity is claimed.
