package rss

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"xiadown/internal/application/networkpolicy"
)

type rssStaticResolver map[string][]net.IPAddr

func (resolver rssStaticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	addresses, ok := resolver[host]
	if !ok {
		return nil, fmt.Errorf("unexpected DNS lookup for %s", host)
	}
	return addresses, nil
}

type rssSequenceResolver struct {
	mu        sync.Mutex
	addresses [][]net.IPAddr
}

func (resolver *rssSequenceResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	if len(resolver.addresses) == 0 {
		return nil, errors.New("unexpected extra DNS lookup")
	}
	addresses := resolver.addresses[0]
	resolver.addresses = resolver.addresses[1:]
	return addresses, nil
}

type rssHTTPClientProvider struct{ client *http.Client }

func (provider rssHTTPClientProvider) HTTPClient() *http.Client { return provider.client }
func (rssHTTPClientProvider) allowPinnedPublicRouteTestSeam()   {}

type rssManagedRouteProvider struct {
	rssHTTPClientProvider
	dial func(context.Context, string, string, *url.URL) (net.Conn, error)
}

func (provider rssManagedRouteProvider) PublicDialURLContext(
	ctx context.Context,
	network string,
	address string,
	logicalURL *url.URL,
) (net.Conn, error) {
	return provider.dial(ctx, network, address, logicalURL)
}

type rssRoundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip rssRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestRSSClientsFailClosedWithoutManagedRoute(t *testing.T) {
	t.Parallel()

	for name, client := range map[string]*http.Client{
		"resource": NewRemoteResourceHTTPClient(nil),
		"feed":     NewService(nil, nil).httpClient(),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := client.Get("http://8.8.8.8/")
			if err == nil || !strings.Contains(err.Error(), "requires the managed App route") {
				t.Fatalf("unmanaged RSS route error = %v", err)
			}
		})
	}
}

func TestRSSHTTPClientRejectsPrivateDNSBeforeDirectDial(t *testing.T) {
	t.Parallel()
	var dialled atomic.Bool
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dialled.Store(true)
			return nil, errors.New("unexpected dial")
		},
	}}})
	service.resolver = rssStaticResolver{
		"private.example": {{IP: net.ParseIP("10.0.0.8")}},
	}

	_, err := service.httpClient().Get("http://private.example/feed")
	if !errors.Is(err, networkpolicy.ErrDestinationBlocked) {
		t.Fatalf("expected blocked destination, got %v", err)
	}
	if dialled.Load() {
		t.Fatal("direct dialer was called for a private DNS answer")
	}
}

func TestRSSManagedRouteKeepsLogicalURLAndRSSPhaseLimits(t *testing.T) {
	t.Parallel()
	var gotLogical atomic.Value
	provider := rssManagedRouteProvider{
		rssHTTPClientProvider: rssHTTPClientProvider{client: &http.Client{}},
		dial: func(ctx context.Context, _, _ string, logicalURL *url.URL) (net.Conn, error) {
			gotLogical.Store(logicalURL.String())
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	timeouts := remoteResourceTransportTimeouts{
		dial: 25 * time.Millisecond, tlsHandshake: 40 * time.Millisecond, responseHeader: 50 * time.Millisecond,
	}
	client := newRemoteResourceHTTPClientWithTimeouts(provider, net.DefaultResolver, timeouts)
	managed, ok := client.Transport.(*managedPublicHTTPTransport)
	if !ok || managed.timeouts != timeouts {
		t.Fatalf("managed RSS transport = %#v", client.Transport)
	}
	started := time.Now()
	_, err := client.Get("http://8.8.8.8/feed/path?policy=one")
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("managed RSS dial timeout = %v after %s", err, time.Since(started))
	}
	if got, _ := gotLogical.Load().(string); got != "http://8.8.8.8/feed/path?policy=one" {
		t.Fatalf("managed RSS logical URL = %q", got)
	}
}

func TestRSSFeedClientUsesManagedRouteWithLogicalURL(t *testing.T) {
	t.Parallel()
	var gotLogical atomic.Value
	provider := rssManagedRouteProvider{
		rssHTTPClientProvider: rssHTTPClientProvider{client: &http.Client{}},
		dial: func(_ context.Context, _, _ string, logicalURL *url.URL) (net.Conn, error) {
			gotLogical.Store(logicalURL.String())
			return nil, errors.New("managed route test stop")
		},
	}
	service := NewService(nil, provider)
	client := service.httpClient()
	if _, ok := client.Transport.(*managedPublicHTTPTransport); !ok {
		t.Fatalf("RSS feed transport = %T, want managed route", client.Transport)
	}
	_, err := client.Get("https://xn--fiqs8s.example/feed/path?policy=one")
	if err == nil || !strings.Contains(err.Error(), "managed route test stop") {
		t.Fatalf("RSS managed feed error = %v", err)
	}
	if got, _ := gotLogical.Load().(string); got != "https://xn--fiqs8s.example/feed/path?policy=one" {
		t.Fatalf("managed RSS feed logical URL = %q", got)
	}
}

func TestRSSLegacyAuthenticatedSOCKSFailsClosed(t *testing.T) {
	t.Parallel()
	var dialled atomic.Bool
	proxyURL, _ := url.Parse("socks5://rss:secret@127.0.0.1:9")
	client := newRemoteResourceHTTPClient(
		rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				dialled.Store(true)
				return nil, errors.New("unexpected dial")
			},
		}}},
		rssStaticResolver{"public.example": {{IP: net.ParseIP("8.8.8.8")}}},
	)
	_, err := client.Get("http://public.example/feed")
	if err == nil || !strings.Contains(err.Error(), "authenticated RSS SOCKS requires") {
		t.Fatalf("legacy authenticated SOCKS error = %v", err)
	}
	if dialled.Load() {
		t.Fatal("legacy authenticated SOCKS attempted an insecure handshake")
	}
}

func TestRSSRemoteResourceClientRechecksDNSForEveryConnection(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "image/png")
		_, _ = io.WriteString(response, "resource")
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")

	var dialCount atomic.Int32
	dialer := &net.Dialer{}
	provider := rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			dialCount.Add(1)
			return dialer.DialContext(ctx, network, originAddress)
		},
	}}}
	client := newRemoteResourceHTTPClient(provider, &rssSequenceResolver{addresses: [][]net.IPAddr{
		{{IP: net.ParseIP("8.8.8.8")}},
		{{IP: net.ParseIP("127.0.0.1")}},
	}})

	response, err := client.Get("http://rebind.example/cover.png")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	_, err = client.Get("http://rebind.example/cover.png")
	if !errors.Is(err, networkpolicy.ErrDestinationBlocked) {
		t.Fatalf("expected rebound resource destination to be blocked, got %v", err)
	}
	if got := dialCount.Load(); got != 1 {
		t.Fatalf("resource dial count = %d, want 1", got)
	}
}

func TestRSSRemoteResourceClientRevalidatesRedirectDestination(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/resource" {
			t.Errorf("private redirect reached origin path %q", request.URL.Path)
		}
		response.Header().Set("Location", "http://private.example/internal")
		response.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")
	dialer := &net.Dialer{}
	provider := rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, originAddress)
		},
	}}}
	client := newRemoteResourceHTTPClient(provider, rssStaticResolver{
		"public.example":  {{IP: net.ParseIP("8.8.8.8")}},
		"private.example": {{IP: net.ParseIP("10.0.0.8")}},
	})
	_, err := client.Get("http://public.example/resource")
	if !errors.Is(err, networkpolicy.ErrDestinationBlocked) {
		t.Fatalf("expected private resource redirect to be blocked, got %v", err)
	}
}

func TestRSSRemoteResourceClientSuppressesRedirectReferer(t *testing.T) {
	t.Parallel()
	var redirectedReferer atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/signed" {
			response.Header().Set("Location", "/final")
			response.WriteHeader(http.StatusFound)
			return
		}
		redirectedReferer.Store(request.Header.Get("Referer"))
		_, _ = io.WriteString(response, "ok")
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")
	dialer := &net.Dialer{}
	provider := rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, originAddress)
		},
	}}}
	client := newRemoteResourceHTTPClient(provider, rssStaticResolver{
		"public.example": {{IP: net.ParseIP("8.8.8.8")}},
	})
	response, err := client.Get("http://public.example/signed?token=upstream-secret")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if got, _ := redirectedReferer.Load().(string); got != "" {
		t.Fatalf("redirect Referer leaked signed source URL: %q", got)
	}
}

func TestRSSRemoteResourceClientScopesPublisherRefererAcrossRedirects(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		location     string
		wantReferer  string
		redirectHost string
	}{
		{
			name: "related subdomain keeps publisher origin", location: "http://media.sspai.com/final",
			redirectHost: "media.sspai.com", wantReferer: "https://sspai.com/",
		},
		{
			name: "unrelated site drops publisher origin", location: "http://unrelated.example/final",
			redirectHost: "unrelated.example", wantReferer: "",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			finalHeaders := make(chan http.Header, 1)
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/start" {
					response.Header().Set("Location", test.location)
					response.WriteHeader(http.StatusFound)
					return
				}
				finalHeaders <- request.Header.Clone()
				_, _ = io.WriteString(response, "ok")
			}))
			defer server.Close()
			originAddress := strings.TrimPrefix(server.URL, "http://")
			dialer := &net.Dialer{}
			provider := rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return dialer.DialContext(ctx, network, originAddress)
				},
			}}}
			client := newRemoteResourceHTTPClient(provider, rssStaticResolver{
				"cdnfile.sspai.com": {{IP: net.ParseIP("8.8.8.8")}},
				test.redirectHost:   {{IP: net.ParseIP("1.1.1.1")}},
			})
			request, err := http.NewRequest(http.MethodGet, "http://cdnfile.sspai.com/start?token=resource-secret", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Referer", "https://sspai.com/")
			request.Header.Set("Authorization", "Bearer must-not-redirect")
			request.Header.Set("Cookie", "session=must-not-redirect")
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			headers := <-finalHeaders
			if got := headers.Get("Referer"); got != test.wantReferer {
				t.Fatalf("redirect Referer = %q, want %q", got, test.wantReferer)
			}
			if headers.Get("Authorization") != "" || headers.Get("Cookie") != "" {
				t.Fatalf("redirect credentials leaked: %#v", headers)
			}
		})
	}
}

func TestRSSRemoteResourceClientScopesValidatorsAcrossRedirects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		location       string
		resolver       rssStaticResolver
		wantValidators bool
	}{
		{
			name: "same origin different path", location: "/final", wantValidators: false,
			resolver: rssStaticResolver{"public.example": {{IP: net.ParseIP("8.8.8.8")}}},
		},
		{
			name: "cross origin then same redirected origin", location: "http://other.example/redirect", wantValidators: false,
			resolver: rssStaticResolver{
				"public.example": {{IP: net.ParseIP("8.8.8.8")}},
				"other.example":  {{IP: net.ParseIP("1.1.1.1")}},
			},
		},
		{
			name: "cross origin then initial origin", location: "http://other.example/return", wantValidators: false,
			resolver: rssStaticResolver{
				"public.example": {{IP: net.ParseIP("8.8.8.8")}},
				"other.example":  {{IP: net.ParseIP("1.1.1.1")}},
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			finalHeaders := make(chan http.Header, 1)
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/signed" {
					response.Header().Set("Location", test.location)
					response.WriteHeader(http.StatusFound)
					return
				}
				if request.URL.Path == "/redirect" {
					response.Header().Set("Location", "/final")
					response.WriteHeader(http.StatusFound)
					return
				}
				if request.URL.Path == "/return" {
					response.Header().Set("Location", "http://public.example/final")
					response.WriteHeader(http.StatusFound)
					return
				}
				finalHeaders <- request.Header.Clone()
				_, _ = io.WriteString(response, "ok")
			}))
			defer server.Close()
			originAddress := strings.TrimPrefix(server.URL, "http://")
			dialer := &net.Dialer{}
			provider := rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return dialer.DialContext(ctx, network, originAddress)
				},
			}}}
			client := newRemoteResourceHTTPClient(provider, test.resolver)
			request, err := http.NewRequest(http.MethodGet, "http://public.example/signed?token=secret", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("If-None-Match", `"private-etag"`)
			request.Header.Set("If-Modified-Since", "Mon, 13 Jul 2026 10:00:00 GMT")
			request.Header.Set("If-Range", `"private-range"`)
			request.Header.Set("Range", "bytes=10-20")
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			headers := <-finalHeaders
			for _, name := range []string{"If-None-Match", "If-Modified-Since", "If-Range"} {
				if test.wantValidators && headers.Get(name) == "" {
					t.Errorf("exact-URL redirect dropped %s", name)
				}
				if !test.wantValidators && headers.Get(name) != "" {
					t.Errorf("redirect retained %s = %q", name, headers.Get(name))
				}
			}
			if got := headers.Get("Range"); got != "bytes=10-20" {
				t.Errorf("redirect Range = %q", got)
			}
			if got := headers.Get("Referer"); got != "" {
				t.Errorf("redirect Referer = %q", got)
			}
		})
	}
}

func TestRSSRemoteResourceClientBoundsResponseHeaderWait(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")
	dialer := &net.Dialer{}
	provider := rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		// Deliberately unbounded provider values must not disable the RSS cap.
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, originAddress)
		},
		TLSHandshakeTimeout:   0,
		ResponseHeaderTimeout: 0,
	}}}
	const testDeadline = 40 * time.Millisecond
	client := newRemoteResourceHTTPClientWithTimeouts(provider, rssStaticResolver{
		"public.example": {{IP: net.ParseIP("8.8.8.8")}},
	}, remoteResourceTransportTimeouts{
		dial: testDeadline, tlsHandshake: testDeadline, responseHeader: testDeadline,
	})
	started := time.Now()
	_, err := client.Get("http://public.example/stall")
	if err == nil {
		t.Fatal("response-header stall did not time out")
	}
	var timeoutError interface{ Timeout() bool }
	if !errors.As(err, &timeoutError) || !timeoutError.Timeout() {
		t.Fatalf("response-header error = %v, want timeout", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("test timeout took %s", elapsed)
	}
}

func TestRSSRemoteResourceTransportForcesEveryConnectionPhaseDeadline(t *testing.T) {
	t.Parallel()
	const phaseDeadline = 45 * time.Millisecond
	deadlineObserved := make(chan time.Duration, 1)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				return nil, errors.New("dial context had no deadline")
			}
			deadlineObserved <- time.Until(deadline)
			return nil, errors.New("test dial stopped")
		},
		TLSHandshakeTimeout:   24 * time.Hour,
		ResponseHeaderTimeout: 0,
	}
	forceRemoteResourceTransportTimeouts(transport, remoteResourceTransportTimeouts{
		dial: phaseDeadline, tlsHandshake: phaseDeadline, responseHeader: phaseDeadline,
	})
	if transport.TLSHandshakeTimeout != phaseDeadline || transport.ResponseHeaderTimeout != phaseDeadline {
		t.Fatalf("forced phase timeouts = TLS %s header %s", transport.TLSHandshakeTimeout, transport.ResponseHeaderTimeout)
	}
	_, _ = transport.DialContext(context.Background(), "tcp", "public.example:443")
	observed := <-deadlineObserved
	if observed <= 0 || observed > phaseDeadline {
		t.Fatalf("dial deadline remaining = %s", observed)
	}
	if transport.MaxResponseHeaderBytes != rssRemoteMaxResponseHeaderBytes {
		t.Fatalf("forced response header bytes = %d", transport.MaxResponseHeaderBytes)
	}
	smaller := &http.Transport{MaxResponseHeaderBytes: 64 << 10}
	forceRemoteResourceTransportTimeouts(smaller, defaultRemoteResourceTransportTimeouts)
	if smaller.MaxResponseHeaderBytes != 64<<10 {
		t.Fatalf("smaller provider response header limit = %d", smaller.MaxResponseHeaderBytes)
	}
}

func TestRSSRemoteResourceClientRejectsOversizedResponseHeaders(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("X-Oversized", strings.Repeat("x", int(rssRemoteMaxResponseHeaderBytes)+4096))
		_, _ = io.WriteString(response, "ok")
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")
	dialer := &net.Dialer{}
	client := newRemoteResourceHTTPClient(rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, originAddress)
		},
	}}}, rssStaticResolver{"public.example": {{IP: net.ParseIP("8.8.8.8")}}})
	response, err := client.Get("http://public.example/feed")
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "response headers exceeded") {
		t.Fatalf("oversized response header error = %v", err)
	}
}

func TestRSSFeedClientRedirectsNeverLeakCookiesReferrersOrValidators(t *testing.T) {
	t.Parallel()
	finalHeaders := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/signed":
			response.Header().Set("Location", "http://other.example/middle")
			response.WriteHeader(http.StatusFound)
		case "/middle":
			response.Header().Set("Location", "http://public.example/final")
			response.WriteHeader(http.StatusFound)
		default:
			finalHeaders <- request.Header.Clone()
			_, _ = io.WriteString(response, "<rss></rss>")
		}
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")
	dialer := &net.Dialer{}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	publicOrigin, _ := url.Parse("http://public.example/")
	jar.SetCookies(publicOrigin, []*http.Cookie{{Name: "feed-session", Value: "secret"}})
	var providerRedirectCalled atomic.Bool
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{
		Transport: &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, originAddress)
		}},
		Jar: jar,
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			providerRedirectCalled.Store(true)
			request.Header.Set("Authorization", "provider-secret")
			return nil
		},
	}})
	service.resolver = rssStaticResolver{
		"public.example": {{IP: net.ParseIP("8.8.8.8")}},
		"other.example":  {{IP: net.ParseIP("1.1.1.1")}},
	}
	request, err := http.NewRequest(http.MethodGet, "http://public.example/signed?token=source-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("If-None-Match", `"private-etag"`)
	request.Header.Set("If-Modified-Since", "Mon, 13 Jul 2026 10:00:00 GMT")
	response, err := service.httpClient().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	headers := <-finalHeaders
	for _, name := range []string{
		"Authorization", "Cookie", "Origin", "Referer", "Proxy-Authorization",
		"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-IP",
		"If-None-Match", "If-Modified-Since", "If-Range",
	} {
		if got := headers.Get(name); got != "" {
			t.Errorf("redirect leaked %s = %q", name, got)
		}
	}
	if providerRedirectCalled.Load() {
		t.Fatal("provider CheckRedirect callback was invoked")
	}
}

func TestRSSHTTPClientPinsValidatedIPForDirectConnections(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Host != "public.example" {
			t.Errorf("origin Host = %q, want public.example", request.Host)
		}
		_, _ = io.WriteString(response, "ok")
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")

	var dialAddress atomic.Value
	dialer := &net.Dialer{}
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialAddress.Store(address)
			return dialer.DialContext(ctx, network, originAddress)
		},
	}}})
	service.resolver = rssStaticResolver{
		"public.example": {{IP: net.ParseIP("8.8.8.8")}},
	}

	response, err := service.httpClient().Get("http://public.example/feed")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if got, _ := dialAddress.Load().(string); got != "8.8.8.8:80" {
		t.Fatalf("dial address = %q, want 8.8.8.8:80", got)
	}
}

func TestRSSHTTPClientRechecksDNSForEveryConnection(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(response, "ok")
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")

	var dialCount atomic.Int32
	dialer := &net.Dialer{}
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			dialCount.Add(1)
			return dialer.DialContext(ctx, network, originAddress)
		},
	}}})
	service.resolver = &rssSequenceResolver{addresses: [][]net.IPAddr{
		{{IP: net.ParseIP("8.8.8.8")}},
		{{IP: net.ParseIP("127.0.0.1")}},
	}}
	client := service.httpClient()

	response, err := client.Get("http://rebind.example/feed")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	_, err = client.Get("http://rebind.example/feed")
	if !errors.Is(err, networkpolicy.ErrDestinationBlocked) {
		t.Fatalf("expected rebound DNS answer to be blocked, got %v", err)
	}
	if got := dialCount.Load(); got != 1 {
		t.Fatalf("direct dial count = %d, want 1", got)
	}
}

func TestRSSHTTPClientRejectsPrivateRedirectBeforeSecondDial(t *testing.T) {
	t.Parallel()
	var privatePathReached atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/feed" {
			response.Header().Set("Location", "http://private.example/internal")
			response.WriteHeader(http.StatusFound)
			return
		}
		privatePathReached.Store(true)
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	originAddress := strings.TrimPrefix(server.URL, "http://")

	var dialCount atomic.Int32
	dialer := &net.Dialer{}
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			dialCount.Add(1)
			return dialer.DialContext(ctx, network, originAddress)
		},
	}}})
	service.resolver = rssStaticResolver{
		"public.example":  {{IP: net.ParseIP("8.8.8.8")}},
		"private.example": {{IP: net.ParseIP("127.0.0.1")}},
	}

	_, err := service.httpClient().Get("http://public.example/feed")
	if !errors.Is(err, networkpolicy.ErrDestinationBlocked) {
		t.Fatalf("expected private redirect to be blocked, got %v", err)
	}
	if privatePathReached.Load() {
		t.Fatal("private redirect path was reached")
	}
	if got := dialCount.Load(); got != 1 {
		t.Fatalf("dial count = %d, want 1", got)
	}
}

func TestRSSHTTPClientPinsDestinationThroughHTTPProxy(t *testing.T) {
	t.Parallel()
	var connectHost atomic.Value
	var originHost atomic.Value
	var originRequestURI atomic.Value
	var connectHookCalled atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			t.Errorf("proxy method = %q, want CONNECT", request.Method)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		connectHost.Store(request.Host)
		if got := request.Header.Get("X-RSS-Proxy"); got != "static" {
			t.Errorf("proxy CONNECT header = %q, want static", got)
		}
		if got := request.Header.Get("Proxy-Authorization"); got != "Basic cnNzOnNlY3JldA==" {
			t.Errorf("proxy authorization = %q", got)
		}
		hijacker, ok := response.(http.Hijacker)
		if !ok {
			t.Error("HTTP proxy response does not support hijacking")
			return
		}
		connection, buffered, err := hijacker.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			t.Error(err)
			return
		}
		if err := buffered.Flush(); err != nil {
			t.Error(err)
			return
		}
		originRequest, err := http.ReadRequest(buffered.Reader)
		if err != nil {
			t.Error(err)
			return
		}
		_ = originRequest.Body.Close()
		originHost.Store(originRequest.Host)
		originRequestURI.Store(originRequest.RequestURI)
		_, _ = io.WriteString(connection, "HTTP/1.1 200 OK\r\nContent-Length: 7\r\nConnection: close\r\n\r\nproxied")
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL.User = url.UserPassword("rss", "secret")

	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		Proxy:              http.ProxyURL(proxyURL),
		ProxyConnectHeader: http.Header{"X-RSS-Proxy": {"static"}},
		OnProxyConnectResponse: func(_ context.Context, _ *url.URL, _ *http.Request, response *http.Response) error {
			connectHookCalled.Store(true)
			if response.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected CONNECT status %d", response.StatusCode)
			}
			return nil
		},
	}}})
	service.resolver = rssStaticResolver{
		"public.example": {{IP: net.ParseIP("8.8.8.8")}},
	}
	response, err := service.httpClient().Get("http://public.example/f%65ed?type=rss")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if got, _ := connectHost.Load().(string); got != "8.8.8.8:80" {
		t.Fatalf("proxy CONNECT target = %q, want pinned IP", got)
	}
	if got, _ := originHost.Load().(string); got != "public.example" {
		t.Fatalf("tunneled origin Host = %q, want public.example", got)
	}
	if got, _ := originRequestURI.Load().(string); got != "/f%65ed?type=rss" {
		t.Fatalf("tunneled request target = %q, want origin-form path and query", got)
	}
	if !connectHookCalled.Load() {
		t.Fatal("OnProxyConnectResponse was not called")
	}
}

func TestRSSProxyCONNECTResponseHeaderBudget(t *testing.T) {
	tests := []struct {
		name       string
		headerSize int
		wantErr    bool
	}{
		{name: "normal", headerSize: 256},
		{name: "exact boundary", headerSize: int(rssRemoteMaxResponseHeaderBytes)},
		{name: "one byte over", headerSize: int(rssRemoteMaxResponseHeaderBytes) + 1, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientSide, proxySide := net.Pipe()
			proxyDone := make(chan error, 1)
			const tunneled = "origin-bytes"
			go func() {
				defer proxySide.Close()
				request, err := http.ReadRequest(bufio.NewReader(proxySide))
				if err != nil {
					proxyDone <- err
					return
				}
				_ = request.Body.Close()
				if request.Method != http.MethodConnect || request.Host != "8.8.8.8:80" {
					proxyDone <- fmt.Errorf("unexpected CONNECT request %s %q", request.Method, request.Host)
					return
				}
				_, err = io.WriteString(proxySide, rssProxyCONNECTResponse(test.headerSize)+tunneled)
				proxyDone <- err
			}()

			proxyURL, err := url.Parse("http://proxy.example:8080")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			connection, err := dialHTTPProxyTunnel(
				ctx,
				"tcp",
				"8.8.8.8:80",
				func(context.Context, string, string) (net.Conn, error) { return clientSide, nil },
				&http.Transport{},
				proxyURL,
			)
			if test.wantErr {
				if connection != nil {
					_ = connection.Close()
					t.Fatal("over-limit CONNECT response returned a tunnel")
				}
				if !errors.Is(err, errRSSProxyConnectResponseHeadersTooLarge) {
					t.Fatalf("over-limit CONNECT error = %v", err)
				}
				if strings.Contains(err.Error(), "proxy-secret") {
					t.Fatalf("CONNECT error leaked response header: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				defer connection.Close()
				payload := make([]byte, len(tunneled))
				if _, err := io.ReadFull(connection, payload); err != nil {
					t.Fatalf("read buffered tunnel bytes: %v", err)
				}
				if string(payload) != tunneled {
					t.Fatalf("tunnel payload = %q", payload)
				}
			}
			if proxyErr := <-proxyDone; proxyErr != nil && !test.wantErr {
				t.Fatalf("proxy: %v", proxyErr)
			}
		})
	}
}

func rssProxyCONNECTResponse(size int) string {
	prefix := "HTTP/1.1 200 Connection Established\r\nX-Secret: proxy-secret\r\nX-Fill: "
	suffix := "\r\n\r\n"
	if size < len(prefix)+len(suffix) {
		size = len(prefix) + len(suffix)
	}
	return prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix
}

func TestRSSHTTPClientRejectsPrivateDNSBeforeProxyDial(t *testing.T) {
	t.Parallel()
	proxyURL, err := url.Parse("http://proxy.example:8080")
	if err != nil {
		t.Fatal(err)
	}
	var dialled atomic.Bool
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dialled.Store(true)
			return nil, errors.New("unexpected proxy dial")
		},
	}}})
	service.resolver = rssStaticResolver{
		"private.example": {{IP: net.ParseIP("10.0.0.8")}},
	}

	_, err = service.httpClient().Get("http://private.example/feed")
	if !errors.Is(err, networkpolicy.ErrDestinationBlocked) {
		t.Fatalf("expected blocked proxy destination, got %v", err)
	}
	if dialled.Load() {
		t.Fatal("proxy was dialled for a private target DNS answer")
	}
}

func TestProxyConnectHeaderPreservesDynamicCallbackAndCredentials(t *testing.T) {
	t.Parallel()
	proxyURL, err := url.Parse("http://rss:secret@proxy.example:8080")
	if err != nil {
		t.Fatal(err)
	}
	var callbackTarget string
	base := &http.Transport{
		ProxyConnectHeader: http.Header{"X-Static": {"ignored when callback exists"}},
		GetProxyConnectHeader: func(_ context.Context, gotProxyURL *url.URL, target string) (http.Header, error) {
			if gotProxyURL.String() != proxyURL.String() {
				t.Fatalf("callback proxy URL = %q", gotProxyURL.String())
			}
			callbackTarget = target
			return http.Header{"X-Dynamic": {"yes"}}, nil
		},
	}
	header, err := proxyConnectHeader(context.Background(), base, proxyURL, "8.8.8.8:80")
	if err != nil {
		t.Fatal(err)
	}
	if callbackTarget != "8.8.8.8:80" {
		t.Fatalf("callback target = %q, want pinned target", callbackTarget)
	}
	if got := header.Get("X-Dynamic"); got != "yes" {
		t.Fatalf("dynamic CONNECT header = %q", got)
	}
	if got := header.Get("X-Static"); got != "" {
		t.Fatalf("static CONNECT header = %q, want callback semantics", got)
	}
	if got := header.Get("Proxy-Authorization"); got != "Basic cnNzOnNlY3JldA==" {
		t.Fatalf("proxy authorization = %q", got)
	}
}

func TestRSSHTTPClientSeparatesHTTPSProxyAndOriginServerNames(t *testing.T) {
	t.Parallel()
	outerServerName := make(chan string, 1)
	innerServerName := make(chan string, 1)

	var proxy *httptest.Server
	proxy = httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect || request.Host != "8.8.8.8:443" {
			t.Errorf("CONNECT target = %s %q, want 8.8.8.8:443", request.Method, request.Host)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		hijacker, ok := response.(http.Hijacker)
		if !ok {
			t.Error("HTTPS proxy response does not support hijacking")
			return
		}
		connection, buffered, err := hijacker.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			t.Error(err)
			return
		}
		if err := buffered.Flush(); err != nil {
			t.Error(err)
			return
		}
		innerTLS := tls.Server(connection, &tls.Config{
			Certificates: proxy.TLS.Certificates,
			MinVersion:   tls.VersionTLS12,
			GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
				innerServerName <- hello.ServerName
				return nil, nil
			},
		})
		if err := innerTLS.Handshake(); err != nil {
			t.Error(err)
			return
		}
		originRequest, err := http.ReadRequest(bufio.NewReader(innerTLS))
		if err != nil {
			t.Error(err)
			return
		}
		_ = originRequest.Body.Close()
		_, _ = io.WriteString(innerTLS, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok")
	}))
	proxy.StartTLS()
	defer proxy.Close()
	proxy.TLS.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		outerServerName <- hello.ServerName
		return nil, nil
	}
	proxy.TLS.NextProtos = []string{"http/1.1"}
	proxyAddress := proxy.Listener.Addr().String()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL.Host = net.JoinHostPort("proxy.example", proxyURL.Port())

	dialer := &net.Dialer{}
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, proxyAddress)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, // test proxy certificate
	}}})
	service.resolver = rssStaticResolver{
		"origin.example": {{IP: net.ParseIP("8.8.8.8")}},
	}
	response, err := service.httpClient().Get("https://origin.example/feed")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if got := <-outerServerName; got != "proxy.example" {
		t.Fatalf("HTTPS proxy SNI = %q, want proxy.example", got)
	}
	if got := <-innerServerName; got != "origin.example" {
		t.Fatalf("origin SNI = %q, want origin.example", got)
	}
}

func TestClonePinnedHTTPRequestKeepsOriginFormForSOCKS(t *testing.T) {
	t.Parallel()
	request, err := http.NewRequest(http.MethodGet, "http://public.example/f%65ed?type=rss", nil)
	if err != nil {
		t.Fatal(err)
	}
	pinned := cloneRequestWithPinnedIP(request, request.URL, net.ParseIP("8.8.8.8"))
	if pinned.URL.Host != "8.8.8.8:80" {
		t.Fatalf("pinned URL host = %q, want 8.8.8.8:80", pinned.URL.Host)
	}
	if pinned.Host != "public.example" {
		t.Fatalf("origin Host header = %q, want public.example", pinned.Host)
	}
	if pinned.URL.Opaque != "" {
		t.Fatalf("SOCKS target URL opaque = %q, want empty", pinned.URL.Opaque)
	}
	if got := pinned.URL.RequestURI(); got != "/f%65ed?type=rss" {
		t.Fatalf("SOCKS origin-form request target = %q", got)
	}
}

func TestRSSHTTPClientFailsClosedForOpaqueRoundTripper(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	service := NewService(nil, rssHTTPClientProvider{client: &http.Client{Transport: rssRoundTripFunc(func(*http.Request) (*http.Response, error) {
		called.Store(true)
		return nil, errors.New("opaque transport called")
	})}})

	_, err := service.httpClient().Get("https://example.com/feed")
	if err == nil || !strings.Contains(err.Error(), "cannot enforce") {
		t.Fatalf("expected enforcing transport error, got %v", err)
	}
	if called.Load() {
		t.Fatal("opaque RoundTripper was called")
	}
}
