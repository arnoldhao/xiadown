package libraryserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/infrastructure/discovery"
	"xiadown/internal/infrastructure/firewall"
)

type fakeRegistration struct{ closed bool }

func (registration *fakeRegistration) Close() error { registration.closed = true; return nil }

type fakeAdvertiser struct {
	value        discovery.Advertisement
	registration *fakeRegistration
}

func (advertiser *fakeAdvertiser) Register(_ context.Context, value discovery.Advertisement) (discovery.Registration, error) {
	advertiser.value = value
	advertiser.registration = &fakeRegistration{}
	return advertiser.registration, nil
}

type fakeFirewall struct {
	enabled      firewall.Rule
	enableErr    error
	enableCalls  int
	disabled     bool
	disableCalls int
}

func (manager *fakeFirewall) Enable(_ context.Context, rule firewall.Rule) error {
	manager.enabled = rule
	manager.enableCalls++
	return manager.enableErr
}

func (manager *fakeFirewall) Disable(context.Context) error {
	manager.disabled = true
	manager.disableCalls++
	return nil
}

func TestLANRuntimeOwnsTLSDiscoveryAndPrivateFirewallLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := New(Config{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	directory := t.TempDir()
	identity, err := LoadOrCreateCertificate(CertificateFiles{
		CertificatePath: filepath.Join(directory, "library.crt"),
		PrivateKeyPath:  filepath.Join(directory, "library.key"),
	}, CertificateOptions{})
	if err != nil {
		t.Fatalf("certificate: %v", err)
	}
	advertiser := &fakeAdvertiser{}
	firewallManager := &fakeFirewall{}
	runtime, err := NewLANRuntime(LANRuntimeConfig{
		Server: server, Identity: &identity, Advertiser: advertiser,
		Firewall: firewallManager, Program: "/Applications/XiaDown.app/Contents/MacOS/XiaDown",
		LANEndpoints: testLANEndpoints,
	})
	if err != nil {
		t.Fatalf("NewLANRuntime: %v", err)
	}
	info, err := runtime.Enable(context.Background(), 0, "Arnold Library")
	if err != nil || info.State != "running" || info.Port == 0 {
		t.Fatalf("Enable = %#v, %v", info, err)
	}
	if advertiser.value.Name != "Arnold Library" || advertiser.value.Port != info.Port {
		t.Fatalf("advertisement = %#v", advertiser.value)
	}
	if got := fmt.Sprint(advertiser.value.InterfaceIndices); got != "[1 2]" {
		t.Fatalf("advertised interfaces = %s, want [1 2]", got)
	}
	if addresses := server.LANAddresses(); len(addresses) != 2 {
		t.Fatalf("LAN listeners = %#v, want every advertised endpoint", addresses)
	}
	if firewallManager.enabled.Port != info.Port || firewallManager.enabled.Program == "" {
		t.Fatalf("firewall rule = %#v", firewallManager.enabled)
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS13, InsecureSkipVerify: true, // test pins the generated identity below
	}}}
	response, err := client.Get("https://127.0.0.1:" + portString(info.Port))
	if err != nil {
		t.Fatalf("GET LAN TLS: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent || server.TLSFingerprint() != identity.Fingerprint {
		t.Fatalf("LAN response=%d fingerprint=%q", response.StatusCode, server.TLSFingerprint())
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	if err := runtime.Disable(shutdownCtx); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !advertiser.registration.closed || !firewallManager.disabled || firewallManager.disableCalls != 1 || server.LANAddress() != "" {
		t.Fatalf("LAN resources not released")
	}
}

func TestLANRuntimeRebindsSamePortWithoutFirewallMutation(t *testing.T) {
	currentEndpoints := []discovery.LANEndpoint{
		{InterfaceIndex: 1, InterfaceName: "test-lan", IP: net.ParseIP("127.0.0.1")},
		{InterfaceIndex: 2, InterfaceName: "test-ethernet", IP: net.ParseIP("::1")},
	}
	runtime, server, advertiser, firewallManager := newTestLANRuntime(t, func() ([]discovery.LANEndpoint, error) {
		return currentEndpoints, nil
	})

	initial, err := runtime.Enable(context.Background(), 0, "Windows Library")
	if err != nil || initial.State != "running" || initial.Port == 0 {
		t.Fatalf("initial Enable = %#v, %v", initial, err)
	}
	initialRegistration := advertiser.registration
	currentEndpoints = currentEndpoints[:1]
	rebound, err := runtime.Enable(context.Background(), 0, "Renamed Windows Library")
	if err != nil || rebound.State != "running" || rebound.Port != initial.Port {
		t.Fatalf("same-port rebind = %#v, %v; want port %d", rebound, err, initial.Port)
	}
	if got := server.LANAddresses(); len(got) != 1 {
		t.Fatalf("rebound listeners = %#v, want one current endpoint", got)
	}
	if initialRegistration == nil || !initialRegistration.closed || advertiser.registration == initialRegistration {
		t.Fatalf("DNS-SD registration was not replaced for changed endpoint/device")
	}
	if firewallManager.enableCalls != 1 || firewallManager.disableCalls != 0 {
		t.Fatalf("same Program+Port rebind mutated firewall: enable=%d disable=%d",
			firewallManager.enableCalls, firewallManager.disableCalls)
	}
}

func TestLANRuntimePortChangeReplacesFirewallRule(t *testing.T) {
	runtime, _, _, firewallManager := newTestLANRuntime(t, testLANEndpoints)
	initial, err := runtime.Enable(context.Background(), 0, "Windows Library")
	if err != nil || initial.Port == 0 {
		t.Fatalf("initial Enable = %#v, %v", initial, err)
	}
	requestedPort := reserveTCP4Port(t)
	if requestedPort == initial.Port {
		requestedPort = reserveTCP4Port(t)
	}

	changed, err := runtime.Enable(context.Background(), requestedPort, "Windows Library")
	if err != nil || changed.State != "running" || changed.Port != requestedPort {
		t.Fatalf("port-change Enable = %#v, %v", changed, err)
	}
	if firewallManager.disableCalls != 1 || firewallManager.enableCalls != 2 {
		t.Fatalf("port change firewall calls: enable=%d disable=%d, want 2/1",
			firewallManager.enableCalls, firewallManager.disableCalls)
	}
	if firewallManager.enabled.Port != requestedPort {
		t.Fatalf("replacement firewall port = %d, want %d", firewallManager.enabled.Port, requestedPort)
	}
}

func TestLANRuntimeKeepsStableFirewallAcrossTemporaryMissingEndpoints(t *testing.T) {
	currentEndpoints := []discovery.LANEndpoint{
		{InterfaceIndex: 1, InterfaceName: "test-lan", IP: net.ParseIP("127.0.0.1")},
	}
	runtime, server, _, firewallManager := newTestLANRuntime(t, func() ([]discovery.LANEndpoint, error) {
		return currentEndpoints, nil
	})
	initial, err := runtime.Enable(context.Background(), 0, "Windows Library")
	if err != nil || initial.Port == 0 {
		t.Fatalf("initial Enable = %#v, %v", initial, err)
	}

	currentEndpoints = nil
	missing, err := runtime.Enable(context.Background(), 0, "Windows Library")
	if !errors.Is(err, discovery.ErrUnavailable) || missing.State != "error" || missing.Port != initial.Port {
		t.Fatalf("missing endpoints = %#v, %v; want retained port %d", missing, err, initial.Port)
	}
	if server.LANAddress() != "" {
		t.Fatalf("listener remained active without an eligible endpoint: %q", server.LANAddress())
	}
	if firewallManager.enableCalls != 1 || firewallManager.disableCalls != 0 {
		t.Fatalf("missing endpoints mutated firewall: enable=%d disable=%d",
			firewallManager.enableCalls, firewallManager.disableCalls)
	}

	currentEndpoints = []discovery.LANEndpoint{
		{InterfaceIndex: 3, InterfaceName: "replacement-lan", IP: net.ParseIP("127.0.0.1")},
	}
	restored, err := runtime.Enable(context.Background(), 0, "Windows Library")
	if err != nil || restored.State != "running" || restored.Port != initial.Port {
		t.Fatalf("restored endpoints = %#v, %v; want stable port %d", restored, err, initial.Port)
	}
	if firewallManager.enableCalls != 1 || firewallManager.disableCalls != 0 {
		t.Fatalf("endpoint recovery mutated firewall: enable=%d disable=%d",
			firewallManager.enableCalls, firewallManager.disableCalls)
	}
}

func TestLANRuntimeKeepsTLSListenerWhenPrivateFirewallNeedsElevation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := New(Config{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	directory := t.TempDir()
	identity, err := LoadOrCreateCertificate(CertificateFiles{
		CertificatePath: filepath.Join(directory, "library.crt"),
		PrivateKeyPath:  filepath.Join(directory, "library.key"),
	}, CertificateOptions{})
	if err != nil {
		t.Fatalf("certificate: %v", err)
	}
	advertiser := &fakeAdvertiser{}
	firewallManager := &fakeFirewall{enableErr: fmt.Errorf(
		"%w: administrator approval is required for Private and LocalSubnet",
		firewall.ErrRuleNotApplied,
	)}
	runtime, err := NewLANRuntime(LANRuntimeConfig{
		Server: server, Identity: &identity, Advertiser: advertiser,
		Firewall: firewallManager, Program: `C:\Program Files\XiaDown\xiadown.exe`,
		LANEndpoints: testLANEndpoints,
	})
	if err != nil {
		t.Fatalf("NewLANRuntime: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Disable(context.Background()) })

	info, enableErr := runtime.Enable(context.Background(), 0, "Windows Library")
	if !errors.Is(enableErr, firewall.ErrRuleNotApplied) {
		t.Fatalf("Enable error = %v, want ErrRuleNotApplied", enableErr)
	}
	if info.State != "error" || info.Port == 0 || !strings.Contains(info.LastError, "administrator approval") {
		t.Fatalf("degraded LAN info = %#v", info)
	}
	inspected := runtime.Inspect(context.Background())
	if inspected != info {
		t.Fatalf("Inspect = %#v, want retained degraded info %#v", inspected, info)
	}
	if server.LANAddress() == "" || advertiser.registration == nil {
		t.Fatalf("firewall failure released active LAN resources: address=%q registration=%#v", server.LANAddress(), advertiser.registration)
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS13, InsecureSkipVerify: true, // test-only generated certificate
	}}}
	response, err := client.Get("https://127.0.0.1:" + portString(info.Port))
	if err != nil {
		t.Fatalf("GET retained LAN TLS listener: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("retained LAN response = %d", response.StatusCode)
	}

	registration := advertiser.registration
	address := server.LANAddress()
	firewallManager.enableErr = nil
	retried, err := runtime.Enable(context.Background(), 0, "Windows Library")
	if err != nil || retried.State != "running" || retried.Port != info.Port || retried.LastError != "" {
		t.Fatalf("retry = %#v, %v", retried, err)
	}
	if firewallManager.enableCalls != 2 || server.LANAddress() != address || advertiser.registration != registration {
		t.Fatalf("retry replaced active resources: calls=%d address=%q registrationChanged=%v",
			firewallManager.enableCalls, server.LANAddress(), advertiser.registration != registration)
	}
}

func portString(port int) string {
	return fmt.Sprintf("%d", port)
}

func newTestLANRuntime(
	t *testing.T,
	endpointProvider func() ([]discovery.LANEndpoint, error),
) (*LANRuntime, *Server, *fakeAdvertiser, *fakeFirewall) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	server, err := New(Config{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	directory := t.TempDir()
	identity, err := LoadOrCreateCertificate(CertificateFiles{
		CertificatePath: filepath.Join(directory, "library.crt"),
		PrivateKeyPath:  filepath.Join(directory, "library.key"),
	}, CertificateOptions{})
	if err != nil {
		t.Fatalf("certificate: %v", err)
	}
	advertiser := &fakeAdvertiser{}
	firewallManager := &fakeFirewall{}
	runtime, err := NewLANRuntime(LANRuntimeConfig{
		Server: server, Identity: &identity, Advertiser: advertiser,
		Firewall: firewallManager, Program: `C:\Program Files\XiaDown\xiadown.exe`,
		LANEndpoints: endpointProvider,
	})
	if err != nil {
		t.Fatalf("NewLANRuntime: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Disable(context.Background()) })
	return runtime, server, advertiser, firewallManager
}

func reserveTCP4Port(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve TCP port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved TCP port: %v", err)
	}
	return port
}

func testLANEndpoints() ([]discovery.LANEndpoint, error) {
	return []discovery.LANEndpoint{
		{InterfaceIndex: 1, InterfaceName: "test-lan", IP: net.ParseIP("127.0.0.1")},
		{InterfaceIndex: 2, InterfaceName: "test-ethernet", IP: net.ParseIP("::1")},
	}, nil
}
