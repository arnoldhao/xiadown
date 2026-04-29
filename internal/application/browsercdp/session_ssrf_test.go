package browsercdp

import (
	"context"
	"net"
	"strings"
	"testing"
)

func stubLookupIPAddrsForHost(t *testing.T, stub func(context.Context, string) ([]net.IPAddr, error)) {
	t.Helper()
	original := lookupIPAddrsForHost
	lookupIPAddrsForHost = stub
	t.Cleanup(func() {
		lookupIPAddrsForHost = original
	})
}

func TestAssertURLAllowedRejectsHostnameResolvingToPrivateIP(t *testing.T) {
	stubLookupIPAddrsForHost(t, func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "public.example.com" {
			t.Fatalf("unexpected host lookup %q", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	})

	err := AssertURLAllowed("https://public.example.com/path", SSRFPolicy{
		DangerouslyAllowPrivateNetwork: false,
		AllowedHostnames:               map[string]struct{}{},
	})
	if err == nil || !strings.Contains(err.Error(), "resolving to private IP") {
		t.Fatalf("expected resolved private IP to be blocked, got %v", err)
	}
}

func TestAssertURLAllowedAllowsHostnameResolvingToPublicIP(t *testing.T) {
	stubLookupIPAddrsForHost(t, func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "public.example.com" {
			t.Fatalf("unexpected host lookup %q", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	})

	err := AssertURLAllowed("https://public.example.com/path", SSRFPolicy{
		DangerouslyAllowPrivateNetwork: false,
		AllowedHostnames:               map[string]struct{}{},
	})
	if err != nil {
		t.Fatalf("expected public hostname to be allowed, got %v", err)
	}
}

func TestAssertURLAllowedAllowsExplicitHostnameAllowlist(t *testing.T) {
	stubLookupIPAddrsForHost(t, func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatalf("allowlisted hostname should not require DNS validation")
		return nil, nil
	})

	err := AssertURLAllowed("http://localhost:3000", SSRFPolicy{
		DangerouslyAllowPrivateNetwork: false,
		AllowedHostnames: map[string]struct{}{
			"localhost": {},
		},
	})
	if err != nil {
		t.Fatalf("expected explicit hostname allowlist to bypass private host block, got %v", err)
	}
}

func TestAssertRequestURLAllowedRejectsPrivateWebsocketTargets(t *testing.T) {
	err := assertRequestURLAllowed(context.Background(), "wss://127.0.0.1/socket", SSRFPolicy{
		DangerouslyAllowPrivateNetwork: false,
		AllowedHostnames:               map[string]struct{}{},
	})
	if err == nil || !strings.Contains(err.Error(), "blocked private IP") {
		t.Fatalf("expected private websocket target to be blocked, got %v", err)
	}
}
