package browsercdp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
)

func testManagedNetworkRoute() *ManagedNetworkRoute {
	return &ManagedNetworkRoute{
		ProxyURL:         "http://127.0.0.1:43199",
		AttestationURL:   "http://0123456789abcdef0123456789abcdef.attest.xiadown.invalid/.well-known/xiadown-network-route",
		AttestationToken: "different-secret",
	}
}

func TestRuntimeShouldSuppressTransientCDPDocumentUpdatedError(t *testing.T) {
	t.Parallel()

	if !runtimeShouldSuppressCDPError("received DOM.documentUpdated when there's no top-level frame") {
		t.Fatal("expected transient DOM.documentUpdated frame error to be suppressed")
	}
	if runtimeShouldSuppressCDPError("could not attach target") {
		t.Fatal("expected unrelated CDP errors to be preserved")
	}
}

func TestBuildLaunchArgs_HeadfulDefersVisibleTargetUntilAfterAttestation(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{})
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --no-startup-window ") {
		t.Fatalf("expected headful launch to suppress the startup window, args=%v", args)
	}
}

func TestStartRequiresManagedNetworkRouteBeforeLaunching(t *testing.T) {
	runtime, err := Start(context.Background(), LaunchOptions{})
	if runtime != nil {
		runtime.Stop()
		t.Fatal("missing route returned a browser runtime")
	}
	if !errors.Is(err, ErrManagedNetworkRouteRequired) {
		t.Fatalf("missing route error = %v", err)
	}
}

func TestManagedNetworkLaunchArgsOwnsAllBrowserRouteFlags(t *testing.T) {
	t.Parallel()
	route := testManagedNetworkRoute()
	args, err := managedNetworkLaunchArgs(route, []string{"--autoplay-policy=no-user-gesture-required"})
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(args, " ") + " "
	for _, required := range []string{
		" --proxy-server=http://127.0.0.1:43199 ",
		" --proxy-bypass-list=<-loopback> ",
		" --disable-quic ",
		" --dns-prefetch-disable ",
		" --force-webrtc-ip-handling-policy=disable_non_proxied_udp ",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("managed route args %v are missing %q", args, required)
		}
	}
	if strings.Contains(joined, " --host-resolver-rules=") {
		t.Fatalf("managed route args %v must not block proxied DNS resolution", args)
	}
}

func TestManagedNetworkLaunchArgsRequiresRoute(t *testing.T) {
	t.Parallel()
	if _, err := managedNetworkLaunchArgs(nil, nil); !errors.Is(err, ErrManagedNetworkRouteRequired) {
		t.Fatalf("missing managed route error = %v", err)
	}
}

func TestLoopbackOnlyNetworkLaunchArgsFailClosedForRemoteTraffic(t *testing.T) {
	t.Parallel()
	args, err := networkLaunchArgs(launchNetworkBoundaryLoopbackOnly, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(args, " ") + " "
	for _, required := range []string{
		" --proxy-server=http://127.0.0.1:1 ",
		" --proxy-bypass-list=127.0.0.0/8;[::1];<-loopback> ",
		" --disable-quic ",
		" --dns-prefetch-disable ",
		" --force-webrtc-ip-handling-policy=disable_non_proxied_udp ",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("loopback-only args %v are missing %q", args, required)
		}
	}
	if _, err := networkLaunchArgs(launchNetworkBoundaryLoopbackOnly, testManagedNetworkRoute(), nil); err == nil {
		t.Fatal("loopback-only start accepted a managed route")
	}
}

func TestManagedNetworkLaunchArgsRejectsCallerOverrides(t *testing.T) {
	t.Parallel()
	for _, argument := range []string{
		"--no-proxy-server",
		"--proxy-server=http://elsewhere:8080",
		"--proxy-pac-url=http://elsewhere/proxy.pac",
		"--host-resolver-rules=MAP * ~NOTFOUND",
		"--enable-quic",
		"--force-webrtc-ip-handling-policy=default",
		"--load-extension=/tmp/untrusted-extension",
	} {
		argument := argument
		t.Run(argument, func(t *testing.T) {
			t.Parallel()
			if _, err := managedNetworkLaunchArgs(testManagedNetworkRoute(), []string{argument}); err == nil {
				t.Fatalf("network override %q was accepted", argument)
			}
		})
	}
}

func TestManagedNetworkLaunchArgsRejectsPreAttestationStartupTargets(t *testing.T) {
	t.Parallel()
	for _, argument := range []string{
		"https://example.com/",
		"--",
		"--app=https://example.com/",
		"--new-window=https://example.com/",
		"--restore-last-session",
		"--headless=new",
		"--no-startup-window",
	} {
		argument := argument
		t.Run(argument, func(t *testing.T) {
			t.Parallel()
			if _, err := managedNetworkLaunchArgs(testManagedNetworkRoute(), []string{argument}); err == nil {
				t.Fatalf("startup target option %q was accepted", argument)
			}
		})
	}
}

func TestManagedNetworkLaunchArgsRejectsNonLoopbackProxy(t *testing.T) {
	t.Parallel()
	_, err := managedNetworkLaunchArgs(&ManagedNetworkRoute{
		ProxyURL:         "http://proxy.example:8080",
		AttestationURL:   "http://0123456789abcdef0123456789abcdef.attest.xiadown.invalid/.well-known/xiadown-network-route",
		AttestationToken: "test",
	}, nil)
	if err == nil {
		t.Fatal("non-loopback managed proxy was accepted")
	}
}

func TestManagedNetworkLaunchArgsRejectsAttestationSecretInURL(t *testing.T) {
	t.Parallel()
	_, err := managedNetworkLaunchArgs(&ManagedNetworkRoute{
		ProxyURL:         "http://127.0.0.1:43199",
		AttestationURL:   "http://deadbeefdeadbeefdeadbeefdeadbeef.attest.xiadown.invalid/.well-known/xiadown-network-route",
		AttestationToken: "deadbeef",
	}, nil)
	if err == nil {
		t.Fatal("attestation secret in URL was accepted")
	}
}

func TestManagedConnectProofValidationAndHeaderLookup(t *testing.T) {
	t.Parallel()
	const beginURL = "http://0123456789abcdef0123456789abcdef.attest.xiadown.invalid/.well-known/xiadown-network-route"
	const beginHost = "0123456789abcdef0123456789abcdef.attest.xiadown.invalid"
	const proofID = "0123456789abcdef0123456789abcdef0123456789abcdef01234567"
	got, err := managedConnectProofID("https://"+proofID+".connect."+beginHost+"/.well-known/xiadown-network-route", beginURL)
	if err != nil || got != proofID {
		t.Fatalf("CONNECT proof = %q, %v", got, err)
	}
	for _, invalid := range []string{
		"http://" + proofID + ".connect." + beginHost + "/.well-known/xiadown-network-route",
		"https://short.connect." + beginHost + "/.well-known/xiadown-network-route",
		"https://" + proofID + ".connect." + beginHost + ":443/.well-known/xiadown-network-route",
		"https://" + proofID + ".connect.ffffffffffffffffffffffffffffffff.attest.xiadown.invalid/.well-known/xiadown-network-route",
		"https://" + proofID + ".example.com/.well-known/xiadown-network-route",
	} {
		if _, err := managedConnectProofID(invalid, beginURL); err == nil {
			t.Fatalf("invalid CONNECT challenge %q was accepted", invalid)
		}
	}
	if header := managedNetworkHeader(network.Headers{"x-xiadown-gateway-attestation": " token "}, "X-XiaDown-Gateway-Attestation"); header != "token" {
		t.Fatalf("case-insensitive CDP header = %q", header)
	}
}

func TestManagedNetworkLaunchArgsRequiresRandomAttestationAuthority(t *testing.T) {
	t.Parallel()
	for _, attestationURL := range []string{
		"http://192.0.2.1/.well-known/xiadown-network-route",
		"http://short.attest.xiadown.invalid/.well-known/xiadown-network-route",
		"http://0123456789abcdef0123456789abcdef.attest.xiadown.invalid:80/.well-known/xiadown-network-route",
		"http://0123456789abcdef0123456789abcdef.example.invalid/.well-known/xiadown-network-route",
		"http://0123456789abcdef0123456789abcdef.attest.xiadown.invalid/other",
	} {
		_, err := managedNetworkLaunchArgs(&ManagedNetworkRoute{
			ProxyURL:         "http://127.0.0.1:43199",
			AttestationURL:   attestationURL,
			AttestationToken: "token",
		}, nil)
		if err == nil {
			t.Fatalf("unsafe attestation URL %q was accepted", attestationURL)
		}
	}
}

func TestBuildLaunchArgsBindsRemoteDebuggingToLoopback(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{})
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --remote-debugging-address=127.0.0.1 ") {
		t.Fatalf("expected cdp to bind loopback, args=%v", args)
	}
}

func TestBuildLaunchArgs_TransientProfileUsesMockKeychain(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{})
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --password-store=basic ") {
		t.Fatalf("expected transient profile to use basic password store, args=%v", args)
	}
	if !strings.Contains(joined, " --use-mock-keychain ") {
		t.Fatalf("expected transient profile to use mock keychain, args=%v", args)
	}
}

func TestBuildLaunchArgs_PersistentProfileUsesRealKeychain(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{PersistentProfile: true})
	joined := " " + strings.Join(args, " ") + " "

	if strings.Contains(joined, " --use-mock-keychain ") {
		t.Fatalf("expected persistent profile to avoid mock keychain, args=%v", args)
	}
	if strings.Contains(joined, " --password-store=basic ") {
		t.Fatalf("expected persistent profile to avoid basic password store, args=%v", args)
	}
}

func TestBuildLaunchArgs_HeadlessKeepsHiddenMode(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{Headless: true})
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --headless=new ") {
		t.Fatalf("expected headless launch flag, args=%v", args)
	}
	if strings.Contains(joined, " --new-window ") {
		t.Fatalf("expected headless launch to skip visible window, args=%v", args)
	}
	if strings.Contains(joined, " --no-startup-window ") {
		t.Fatalf("headless launch does not need a startup-window flag, args=%v", args)
	}
}

func TestAppendBrowserLaunchArgs_DisablesVivaldiWelcome(t *testing.T) {
	t.Parallel()

	args := appendBrowserLaunchArgs([]string{"--custom-flag"}, BrowserVivaldi)
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --disable-vivaldi ") {
		t.Fatalf("expected Vivaldi launch to disable Vivaldi welcome, args=%v", args)
	}
}

func TestAppendBrowserLaunchArgs_LeavesOtherBrowsersUnchanged(t *testing.T) {
	t.Parallel()

	args := appendBrowserLaunchArgs([]string{"--custom-flag"}, BrowserChrome)

	if len(args) != 1 || args[0] != "--custom-flag" {
		t.Fatalf("expected Chrome launch args unchanged, got %v", args)
	}
}

func TestRuntimeGracefulCloseTimeout_ExtendsForPersistentProfile(t *testing.T) {
	t.Parallel()

	if got := runtimeGracefulCloseTimeout(LaunchOptions{}); got != 1500*time.Millisecond {
		t.Fatalf("expected transient close timeout 1.5s, got %s", got)
	}
	if got := runtimeGracefulCloseTimeout(LaunchOptions{PersistentProfile: true}); got != 3*time.Second {
		t.Fatalf("expected persistent close timeout 3s, got %s", got)
	}
}

func TestRuntimeTerminateGraceTimeout_ExtendsForPersistentProfile(t *testing.T) {
	t.Parallel()

	if got := runtimeTerminateGraceTimeout(LaunchOptions{}); got != 300*time.Millisecond {
		t.Fatalf("expected transient terminate grace 300ms, got %s", got)
	}
	if got := runtimeTerminateGraceTimeout(LaunchOptions{PersistentProfile: true}); got != 1*time.Millisecond {
		t.Fatalf("expected persistent terminate grace 1ms, got %s", got)
	}
}

func TestReadDevToolsActivePortBuildsWebSocketURL(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "DevToolsActivePort")
	if err := os.WriteFile(path, []byte("43123\n/devtools/browser/session-id\n"), 0o644); err != nil {
		t.Fatalf("write devtools active port: %v", err)
	}

	port, wsURL, err := readDevToolsActivePort(path)
	if err != nil {
		t.Fatalf("read devtools active port: %v", err)
	}
	if port != 43123 {
		t.Fatalf("expected port 43123, got %d", port)
	}
	if wsURL != "ws://127.0.0.1:43123/devtools/browser/session-id" {
		t.Fatalf("unexpected websocket url: %q", wsURL)
	}
}

func TestReadDevToolsActivePortAcceptsOnlyMatchingLoopbackWebSocket(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		value   string
		wantURL string
	}{
		{
			name:    "absolute loopback",
			value:   "43123\nws://127.0.0.1:43123/devtools/browser/session-id\n",
			wantURL: "ws://127.0.0.1:43123/devtools/browser/session-id",
		},
		{name: "remote host", value: "43123\nws://attacker.example:43123/devtools/browser/session-id\n"},
		{name: "secure remote socket", value: "43123\nwss://127.0.0.1:43123/devtools/browser/session-id\n"},
		{name: "different port", value: "43123\nws://127.0.0.1:43124/devtools/browser/session-id\n"},
		{name: "wrong path", value: "43123\n/devtools/page/session-id\n"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "DevToolsActivePort")
			if err := os.WriteFile(path, []byte(test.value), 0o644); err != nil {
				t.Fatal(err)
			}
			_, gotURL, err := readDevToolsActivePort(path)
			if test.wantURL == "" {
				if err == nil {
					t.Fatalf("unsafe endpoint accepted as %q", gotURL)
				}
				return
			}
			if err != nil || gotURL != test.wantURL {
				t.Fatalf("endpoint = %q, %v; want %q", gotURL, err, test.wantURL)
			}
		})
	}
}

func TestRuntimeRegistryCleanupRemovesStaleRecord(t *testing.T) {
	tempDir := t.TempDir()
	previousRegistryDir := runtimeRegistryDir
	runtimeRegistryDir = func() string { return tempDir }
	t.Cleanup(func() { runtimeRegistryDir = previousRegistryDir })

	id, err := registerRuntimeProcess(runtimeProcessRecord{
		PID:            0,
		ProcessGroupID: 0,
		ExecutablePath: "/tmp/browser",
		UserDataDir:    "/tmp/profile",
		CDPPort:        9222,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("register runtime process: %v", err)
	}
	if id != "" {
		t.Fatalf("expected invalid pid to skip registration, got %q", id)
	}

	record := runtimeProcessRecord{
		ID:             "stale",
		PID:            -1,
		ProcessGroupID: 0,
		ExecutablePath: "/tmp/browser",
		UserDataDir:    "/tmp/profile",
		CDPPort:        9223,
		CreatedAt:      time.Now(),
	}
	payload := []byte(`{"id":"stale","pid":-1,"executablePath":"/tmp/browser","cdpPort":9223}`)
	if err := os.WriteFile(filepath.Join(tempDir, record.ID+".json"), payload, 0o644); err != nil {
		t.Fatalf("write stale record: %v", err)
	}

	if err := CleanupStaleRuntimes(context.Background()); err != nil {
		t.Fatalf("cleanup stale runtimes: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, record.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected stale record to be removed, err=%v", err)
	}
}

func TestRegisterRuntimeProcessUsesPrivatePermissions(t *testing.T) {
	tempDir := filepath.Join(t.TempDir(), "registry")
	previousRegistryDir := runtimeRegistryDir
	runtimeRegistryDir = func() string { return tempDir }
	t.Cleanup(func() { runtimeRegistryDir = previousRegistryDir })

	id, err := registerRuntimeProcess(runtimeProcessRecord{
		PID:            12345,
		ProcessGroupID: 12345,
		ExecutablePath: "/tmp/browser",
		UserDataDir:    "/tmp/profile",
		CDPPort:        9222,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("register runtime process: %v", err)
	}
	if id == "" {
		t.Fatal("expected runtime process record id")
	}
	assertPrivateRuntimeRegistryPath(t, tempDir, true, 0o700)
	assertPrivateRuntimeRegistryPath(t, filepath.Join(tempDir, id+".json"), false, 0o600)
}
