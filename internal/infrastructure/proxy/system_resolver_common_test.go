package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"xiadown/internal/domain/settings"
)

type recordingSystemProxyResolver struct {
	targets []string
	closed  bool
}

func (resolver *recordingSystemProxyResolver) Resolve(target *url.URL) (*url.URL, error) {
	resolver.targets = append(resolver.targets, target.String())
	return nil, nil
}

func (resolver *recordingSystemProxyResolver) Close() {
	resolver.closed = true
}

func TestSystemProxyURLContextReusesGenerationResolver(t *testing.T) {
	t.Parallel()
	resolver := &recordingSystemProxyResolver{}
	for _, rawTarget := range []string{
		"https://music.youtube.com/",
		"https://i.ytimg.com/vi/example/default.jpg",
	} {
		if _, err := systemProxyURLContext(context.Background(), resolver, mustParseURL(t, rawTarget)); err != nil {
			t.Fatalf("resolve %s: %v", rawTarget, err)
		}
	}
	if resolver.closed {
		t.Fatal("per-request resolution closed the generation-owned resolver")
	}
	if len(resolver.targets) != 2 {
		t.Fatalf("resolver calls = %d, want 2", len(resolver.targets))
	}
	if resolver.targets[0] != "https://music.youtube.com/" || resolver.targets[1] != "https://i.ytimg.com/" {
		t.Fatalf("canonical system origins = %v", resolver.targets)
	}
}

type blockingSystemProxyResolver struct {
	started chan struct{}
	release chan struct{}
	done    chan struct{}
}

func (resolver *blockingSystemProxyResolver) Resolve(_ *url.URL) (*url.URL, error) {
	close(resolver.started)
	<-resolver.release
	close(resolver.done)
	return nil, nil
}

func (*blockingSystemProxyResolver) Close() {}

func TestRouteStateBoundsNativeSystemProxyResolutionByConfiguredTimeout(t *testing.T) {
	resolver := &blockingSystemProxyResolver{
		started: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
	state := &routeState{
		config: Config{
			Mode:    settings.ProxyModeSystem,
			Timeout: 25 * time.Millisecond,
		},
		systemProxy: resolver,
	}
	request := (&http.Request{URL: mustParseURL(t, "https://music.youtube.com/")}).WithContext(context.Background())

	startedAt := time.Now()
	proxyURL, err := state.proxyForRequest(request)
	if proxyURL != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded system resolution = %v, %v", proxyURL, err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("system resolution ignored configured timeout: %s", elapsed)
	}
	select {
	case <-resolver.started:
	default:
		t.Fatal("native resolver was not invoked")
	}
	close(resolver.release)
	select {
	case <-resolver.done:
	case <-time.After(time.Second):
		t.Fatal("native resolver did not finish after release")
	}
}

func TestPlatformSystemProxyResolutionHonorsPreCancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	proxyURL, err := platformSystemProxyURLContext(ctx, mustParseURL(t, "https://music.youtube.com/"))
	if proxyURL != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled native resolution = %v, %v", proxyURL, err)
	}
}

func TestFirstSystemProxyCandidatePreservesNativeOrder(t *testing.T) {
	t.Parallel()

	proxyURL, err := firstSystemProxyCandidate([]string{
		"socks://user:secret@proxy.example:1080",
		"direct://",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyURL.String(); got != "socks5://user:secret@proxy.example:1080" {
		t.Fatalf("first candidate = %q", got)
	}

	proxyURL, err = firstSystemProxyCandidate([]string{
		"direct://",
		"http://proxy.example:8080",
	})
	if err != nil || proxyURL != nil {
		t.Fatalf("explicit DIRECT = %v, %v", proxyURL, err)
	}
}

func TestFirstSystemProxyCandidateFailsClosedBeforeLaterDirect(t *testing.T) {
	t.Parallel()

	proxyURL, err := firstSystemProxyCandidate([]string{
		"unsupported://sensitive-user:sensitive-password@proxy.example:1",
		"direct://",
	})
	if err == nil || proxyURL != nil {
		t.Fatalf("unsupported first candidate = %v, %v", proxyURL, err)
	}
	if strings.Contains(err.Error(), "sensitive-user") || strings.Contains(err.Error(), "sensitive-password") {
		t.Fatalf("resolution error leaked credentials: %q", err)
	}
	if _, err := firstSystemProxyCandidate(nil); err == nil {
		t.Fatal("empty native decision list was treated as direct")
	}
}

func TestSystemProxyURLFromPartsEncodesCredentials(t *testing.T) {
	t.Parallel()

	proxyURL, err := systemProxyURLFromParts("http", "2001:db8::1", 8443, "name@example", "p:a/s")
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyURL.Host; got != "[2001:db8::1]:8443" {
		t.Fatalf("proxy host = %q", got)
	}
	if username := proxyURL.User.Username(); username != "name@example" {
		t.Fatalf("username = %q", username)
	}
	if password, ok := proxyURL.User.Password(); !ok || password != "p:a/s" {
		t.Fatalf("password = %q, %v", password, ok)
	}
	if _, err := systemProxyURLFromParts("direct", "", 0, "", ""); err != nil {
		t.Fatalf("explicit direct: %v", err)
	}
}

func TestCanonicalSystemProxyTargetUsesOriginWithoutCredentials(t *testing.T) {
	t.Parallel()

	target := mustParseURL(t, "wss://user:secret@music.example/watch?id=one#private")
	canonical, err := canonicalSystemProxyTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := canonical.String(); got != "https://music.example/" {
		t.Fatalf("canonical target = %q", got)
	}
	canonical, err = canonicalSystemProxyTarget(mustParseURL(t, "http://MUSIC.EXAMPLE.:80/other?secret=two"))
	if err != nil || canonical.String() != "http://music.example/" {
		t.Fatalf("canonical default-port origin = %v, %v", canonical, err)
	}
	canonical, err = canonicalSystemProxyTarget(mustParseURL(t, "https://music.example:8443/other"))
	if err != nil || canonical.String() != "https://music.example:8443/" {
		t.Fatalf("canonical non-default-port origin = %v, %v", canonical, err)
	}
	if _, err := canonicalSystemProxyTarget(mustParseURL(t, "file:///tmp/song")); err == nil {
		t.Fatal("unsupported target scheme reached the native resolver")
	}
}

func TestLogicalRouteAndSystemOriginCanonicalizeInternationalHostname(t *testing.T) {
	t.Parallel()
	target := mustParseURL(t, "https://音乐.example/路径?secret=one")
	address, err := logicalRouteAddress(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(address, "音乐") || !strings.HasSuffix(address, ":443") {
		t.Fatalf("IDN logical address = %q", address)
	}
	if !sameCanonicalNetworkTarget(address, net.JoinHostPort("音乐.example.", "443")) {
		t.Fatalf("IDN authority comparison rejected %q", address)
	}
	canonical, err := canonicalSystemProxyTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(canonical.String(), "音乐") || canonical.Path != "/" || canonical.RawQuery != "" {
		t.Fatalf("IDN canonical system origin = %q", canonical)
	}
	if !shouldBypassURL(target, []string{"音乐.example"}) || !shouldBypassURL(target, []string{canonical.Hostname()}) {
		t.Fatal("IDN NoProxy rules did not share canonical hostname semantics")
	}
}

func TestNormalizeSystemProxyCandidateAddsSOCKSDefaultPort(t *testing.T) {
	t.Parallel()

	proxyURL, err := normalizeSystemProxyCandidate("socks://proxy.example")
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyURL.String(); got != "socks5://proxy.example:1080" {
		t.Fatalf("SOCKS candidate = %q", got)
	}
}

func TestEnvironmentSystemProxyFallbackRequiresExplicitPolicy(t *testing.T) {
	for _, name := range []string{
		"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy",
		"ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy", "REQUEST_METHOD",
	} {
		t.Setenv(name, "")
	}

	target := mustParseURL(t, "https://music.example/path")
	if proxyURL, err := environmentSystemProxyURL(target); proxyURL != nil || !errors.Is(err, errNativeSystemProxyUnavailable) {
		t.Fatalf("absent fallback policy = %v, %v", proxyURL, err)
	}

	t.Setenv("NO_PROXY", "another.example")
	if proxyURL, err := environmentSystemProxyURL(target); proxyURL != nil || !errors.Is(err, errNativeSystemProxyUnavailable) {
		t.Fatalf("unmatched NO_PROXY-only fallback = %v, %v", proxyURL, err)
	}
	t.Setenv("NO_PROXY", "music.example")
	if proxyURL, err := environmentSystemProxyURL(target); proxyURL != nil || err != nil {
		t.Fatalf("matching NO_PROXY-only fallback = %v, %v", proxyURL, err)
	}

	t.Setenv("NO_PROXY", "")
	t.Setenv("ALL_PROXY", "socks5://proxy.example:1080")
	proxyURL, err := environmentSystemProxyURL(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyURL.String(); got != "socks5://proxy.example:1080" {
		t.Fatalf("ALL_PROXY = %q", got)
	}

	t.Setenv("NO_PROXY", "music.example")
	proxyURL, err = environmentSystemProxyURL(target)
	if err != nil || proxyURL != nil {
		t.Fatalf("explicit NO_PROXY = %v, %v", proxyURL, err)
	}
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
