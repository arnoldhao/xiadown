package service

import "testing"

type ytdlpGatewayResolverStub struct {
	gateway          string
	upstream         string
	attestationURL   string
	attestationToken string
}

func (stub ytdlpGatewayResolverStub) ConsumerProxyURL() string { return stub.gateway }

func (stub ytdlpGatewayResolverStub) ConsumerProxyAttestation() (string, string) {
	return stub.attestationURL, stub.attestationToken
}

func (stub ytdlpGatewayResolverStub) ResolveProxy(string) (string, error) {
	return stub.upstream, nil
}

func TestManagedBrowserNetworkRouteIncludesGatewayAttestation(t *testing.T) {
	t.Parallel()
	service := &LibraryService{proxyClient: ytdlpGatewayResolverStub{
		gateway:          "http://127.0.0.1:43199",
		attestationURL:   "http://192.0.2.1/.well-known/xiadown-network-route",
		attestationToken: "test",
	}}
	route, err := service.managedBrowserNetworkRoute()
	if err != nil {
		t.Fatal(err)
	}
	if route.ProxyURL != "http://127.0.0.1:43199" || route.AttestationToken != "test" {
		t.Fatalf("unexpected managed route: %#v", route)
	}
}

func TestResolveYTDLPProxyPrefersStableConsumerGateway(t *testing.T) {
	t.Parallel()

	service := &LibraryService{proxyClient: ytdlpGatewayResolverStub{
		gateway:  "http://127.0.0.1:43199",
		upstream: "http://upstream.example:8080",
	}}
	if got := service.resolveYTDLPProxy("https://music.youtube.com/watch?v=test"); got != "http://127.0.0.1:43199" {
		t.Fatalf("resolved proxy = %q, want stable gateway", got)
	}
}

func TestResolveYTDLPProxyKeepsLegacyResolverFallback(t *testing.T) {
	t.Parallel()

	service := &LibraryService{proxyClient: ytdlpGatewayResolverStub{
		upstream: "http://upstream.example:8080",
	}}
	if got := service.resolveYTDLPProxy("https://example.com/video"); got != "http://upstream.example:8080" {
		t.Fatalf("resolved proxy = %q, want fallback upstream", got)
	}
}

func TestManagedConsumerProxyURLRequiresLoopbackGateway(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		gateway string
		want    string
	}{
		{name: "IPv4", gateway: "http://127.0.0.1:43199/", want: "http://127.0.0.1:43199"},
		{name: "IPv6", gateway: "http://[::1]:43199", want: "http://[::1]:43199"},
		{name: "localhost", gateway: "http://localhost:43199", want: "http://localhost:43199"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &LibraryService{proxyClient: ytdlpGatewayResolverStub{gateway: test.gateway}}
			got, err := service.managedConsumerProxyURL()
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("managed gateway = %q, want %q", got, test.want)
			}
		})
	}
}

func TestManagedConsumerProxyURLFailsClosed(t *testing.T) {
	t.Parallel()

	for _, gateway := range []string{
		"",
		"http://proxy.example:8080",
		"https://127.0.0.1:43199",
		"http://user:pass@127.0.0.1:43199",
		"http://127.0.0.1",
		"http://127.0.0.1:43199/path",
	} {
		gateway := gateway
		t.Run(gateway, func(t *testing.T) {
			t.Parallel()
			service := &LibraryService{proxyClient: ytdlpGatewayResolverStub{gateway: gateway}}
			if _, err := service.managedConsumerProxyURL(); err == nil {
				t.Fatalf("gateway %q was accepted", gateway)
			}
		})
	}

	if _, err := (&LibraryService{}).managedConsumerProxyURL(); err == nil {
		t.Fatal("missing gateway provider was accepted")
	}
}
