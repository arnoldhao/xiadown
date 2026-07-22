package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"xiadown/internal/application/networkpolicy"
)

type proxyStaticResolver map[string][]net.IPAddr

type closeWriteTrackingConn struct {
	net.Conn
	closed atomic.Bool
}

type proxyManagedDialProvider func(context.Context, string, string, *url.URL) (net.Conn, error)

func (provider proxyManagedDialProvider) PublicDialURLContext(
	ctx context.Context,
	network string,
	address string,
	logicalURL *url.URL,
) (net.Conn, error) {
	return provider(ctx, network, address, logicalURL)
}

func (connection *closeWriteTrackingConn) CloseWrite() error {
	connection.closed.Store(true)
	return nil
}

func (resolver proxyStaticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	return resolver[host], nil
}

func TestPublicNetworkProxyRejectsPrivateDNSBeforeDial(t *testing.T) {
	t.Parallel()
	var dialled atomic.Bool
	proxy := &publicNetworkProxy{
		resolver: proxyStaticResolver{"private.example": {{IP: net.ParseIP("10.0.0.8")}}},
		managedDial: func(context.Context, string, string, *url.URL) (net.Conn, error) {
			dialled.Store(true)
			return nil, nil
		},
	}
	_, err := proxy.dialPublicHost(context.Background(), "tcp", "private.example", "443", mustProxyTestURL(t, "https://private.example/"))
	if err == nil {
		t.Fatal("expected private DNS answer to be blocked")
	}
	if dialled.Load() {
		t.Fatal("dialer was called for a blocked DNS answer")
	}
}

func TestPublicNetworkProxyUsesManagedRouteAfterDestinationValidation(t *testing.T) {
	t.Parallel()
	var managedAddress atomic.Value
	proxy := &publicNetworkProxy{
		resolver: proxyStaticResolver{"public.example": {{IP: net.ParseIP("8.8.8.8")}}},
		managedDial: func(_ context.Context, _ string, address string, target *url.URL) (net.Conn, error) {
			managedAddress.Store(address)
			if target.String() != "https://public.example/resource?policy=one" {
				t.Fatalf("managed logical URL = %q", target)
			}
			return nil, context.Canceled
		},
	}
	_, err := proxy.dialPublicHost(context.Background(), "tcp", "public.example", "443", mustProxyTestURL(t, "https://public.example/resource?policy=one"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("managed route error = %v", err)
	}
	if got, _ := managedAddress.Load().(string); got != "public.example:443" {
		t.Fatalf("managed route address = %q", got)
	}
}

func TestPublicNetworkProxyCanonicalizesIDNAuthorityBeforeManagedDial(t *testing.T) {
	t.Parallel()
	const asciiHost = "xn--bcher-kva.example"
	var managedAddress atomic.Value
	proxy := &publicNetworkProxy{
		resolver: proxyStaticResolver{asciiHost: {{IP: net.ParseIP("8.8.8.8")}}},
		managedDial: func(_ context.Context, _ string, address string, target *url.URL) (net.Conn, error) {
			managedAddress.Store(address)
			if target.Hostname() != "bücher.example" || target.Path != "/path" {
				t.Fatalf("managed IDN logical URL = %q", target)
			}
			return nil, context.Canceled
		},
	}
	_, err := proxy.dialPublicHost(
		context.Background(), "tcp", asciiHost, "80", mustProxyTestURL(t, "http://bücher.example/path"),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("managed IDN route error = %v", err)
	}
	if got, _ := managedAddress.Load().(string); got != asciiHost+":80" {
		t.Fatalf("managed IDN address = %q", got)
	}
}

func TestPublicNetworkProxyHalfClosesManagedConnectionWrappers(t *testing.T) {
	t.Parallel()
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	wrapped := &closeWriteTrackingConn{Conn: left}
	closePublicProxyWrite(wrapped)
	if !wrapped.closed.Load() {
		t.Fatal("managed connection wrapper did not receive CloseWrite")
	}
}

func TestPublicNetworkProxyRejectsTranslationAndTransitionDNSBeforeDial(t *testing.T) {
	t.Parallel()
	for _, address := range []string{
		"64:ff9b::a9fe:a9fe",
		"64:ff9b:1::a9fe:a9fe",
		"2001:0000:4136:e378:8000:63bf:3fff:fdd2",
		"2002:a9fe:a9fe::1",
	} {
		t.Run(address, func(t *testing.T) {
			var dialled atomic.Bool
			proxy := &publicNetworkProxy{
				resolver: proxyStaticResolver{"media.example": {{IP: net.ParseIP(address)}}},
				managedDial: func(context.Context, string, string, *url.URL) (net.Conn, error) {
					dialled.Store(true)
					return nil, nil
				},
			}
			_, err := proxy.dialPublicHost(context.Background(), "tcp", "media.example", "443", mustProxyTestURL(t, "https://media.example/"))
			if err == nil {
				t.Fatalf("expected %s to be blocked", address)
			}
			if dialled.Load() {
				t.Fatalf("dialer was called for blocked address %s", address)
			}
		})
	}
}

func mustProxyTestURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestPublicNetworkProxyRevalidatesRedirectDestination(t *testing.T) {
	t.Parallel()
	var privatePathReached atomic.Bool
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirect" {
			w.Header().Set("Location", "http://127.0.0.1/private")
			w.WriteHeader(http.StatusFound)
			return
		}
		privatePathReached.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()
	originAddress := origin.Listener.Addr().String()

	dialer := &net.Dialer{}
	proxy, err := startPublicNetworkProxy(context.Background(), proxyManagedDialProvider(
		func(ctx context.Context, network string, _ string, _ *url.URL) (net.Conn, error) {
			return dialer.DialContext(ctx, network, originAddress)
		},
	))
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	proxy.resolver = proxyStaticResolver{"public.example": {{IP: net.ParseIP("8.8.8.8")}}}

	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := client.Get("http://public.example/redirect")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("redirect response status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
	if privatePathReached.Load() {
		t.Fatal("redirect reached a loopback destination")
	}
}

var _ networkpolicy.Resolver = proxyStaticResolver{}
