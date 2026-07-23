package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"xiadown/internal/domain/settings"
)

func dialHTTPConnectProxy(ctx context.Context, proxyURL *url.URL, target string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return dialHTTPConnectProxyWithDialer(ctx, proxyURL, target, timeout, dialer.DialContext)
}

type fixedDirectResolver struct {
	addresses []net.IPAddr
	err       error
}

func (resolver fixedDirectResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return append([]net.IPAddr(nil), resolver.addresses...), resolver.err
}

type fixedSystemRouteResolver struct {
	proxy *url.URL
	calls atomic.Int32
}

func (resolver *fixedSystemRouteResolver) Resolve(*url.URL) (*url.URL, error) {
	resolver.calls.Add(1)
	return resolver.proxy, nil
}

func (*fixedSystemRouteResolver) Close() {}

func TestRegisteredLoopbackPolicyExplicitlyForcesDirect(t *testing.T) {
	registry := newInternalLoopbackRegistry()
	if err := registry.register("http://127.0.0.1:45678/private/token"); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:45678/media", nil)
	loopback, permitted, forceDirect := registry.permitsRequest(request, "127.0.0.1:12345")
	if !loopback || !permitted || !forceDirect {
		t.Fatalf("registered route = loopback %t permitted %t direct %t", loopback, permitted, forceDirect)
	}

	unregistered, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:45679/media", nil)
	loopback, permitted, forceDirect = registry.permitsRequest(unregistered, "127.0.0.1:12345")
	if !loopback || permitted || forceDirect {
		t.Fatalf("unregistered route = loopback %t permitted %t direct %t", loopback, permitted, forceDirect)
	}
}

func TestRegisteredLoopbackHTTPStaysDirectInEveryProxyMode(t *testing.T) {
	destinationHits := atomic.Int32{}
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()

	proxyHits := atomic.Int32{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyHits.Add(1)
		http.Error(w, "local target escaped", http.StatusBadGateway)
	}))
	defer upstream.Close()
	proxyHost, proxyPort := hostPortFromURL(t, upstream.URL)

	unusedListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	unusedHost, unusedPortText, _ := net.SplitHostPort(unusedListener.Addr().String())
	_ = unusedListener.Close()
	var unusedPort int
	_, _ = fmt.Sscanf(unusedPortText, "%d", &unusedPort)

	tests := []struct {
		name   string
		config Config
		bind   func(*routeState)
	}{
		{
			name: "manual-http",
			config: Config{Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
				Host: proxyHost, Port: proxyPort, Timeout: 2 * time.Second},
		},
		{
			name: "manual-socks5",
			config: Config{Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeSocks5,
				Host: unusedHost, Port: unusedPort, Timeout: 2 * time.Second},
		},
		{
			name:   "system",
			config: Config{Mode: settings.ProxyModeSystem, Timeout: 2 * time.Second},
			bind: func(state *routeState) {
				if state.systemProxy != nil {
					state.systemProxy.Close()
				}
				proxyURL, parseErr := url.Parse(upstream.URL)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				state.systemProxy = &fixedSystemRouteResolver{proxy: proxyURL}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beforeDestination := destinationHits.Load()
			beforeProxy := proxyHits.Load()
			manager, err := NewManager(test.config)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			if test.bind != nil {
				test.bind(manager.gateway.active.Load())
			}
			registerInternalTestTarget(t, manager, destination.URL)
			response, err := manager.HTTPClient().Get(destination.URL + "/private/token")
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusNoContent || destinationHits.Load() != beforeDestination+1 {
				t.Fatalf("destination response = %d, hits = %d", response.StatusCode, destinationHits.Load()-beforeDestination)
			}
			if proxyHits.Load() != beforeProxy {
				t.Fatal("registered loopback request reached the upstream proxy")
			}
		})
	}
}

func TestRegisteredLoopbackCONNECTStaysDirect(t *testing.T) {
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()

	proxyHits := atomic.Int32{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyHits.Add(1)
		http.Error(w, "CONNECT escaped", http.StatusBadGateway)
	}))
	defer upstream.Close()
	host, port := hostPortFromURL(t, upstream.URL)
	manager, err := NewManager(Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
		Host: host, Port: port, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	registerInternalTestTarget(t, manager, destination.URL)

	proxyURL, _ := url.Parse(manager.GatewayURL())
	transport := destination.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(proxyURL)
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.MinVersion = tls.VersionTLS12
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Get(destination.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || proxyHits.Load() != 0 {
		t.Fatalf("CONNECT response = %d, upstream proxy hits = %d", response.StatusCode, proxyHits.Load())
	}
}

func TestDirectRouteRejectsLoopbackDNSBeforeTCPConnect(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	state, err := newRouteState(directConfig(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer state.close()
	state.directDNS = fixedDirectResolver{addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	connection, err := state.dialDirect(context.Background(), "tcp", net.JoinHostPort("rebinding.example", port))
	if connection != nil {
		connection.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "hostname resolving to loopback") {
		t.Fatalf("loopback DNS route = %v, %v", connection, err)
	}

	_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond))
	accepted, acceptErr := listener.Accept()
	if accepted != nil {
		accepted.Close()
		t.Fatal("rejected DNS alias completed a TCP connection")
	}
	var networkError net.Error
	if !errors.As(acceptErr, &networkError) || !networkError.Timeout() {
		t.Fatalf("unexpected accept result: %v", acceptErr)
	}
}

func TestProxiedRouteRejectsLoopbackDNSBeforeContactingProxy(t *testing.T) {
	t.Parallel()
	var proxyHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyHits.Add(1)
		http.Error(w, "escaped", http.StatusBadGateway)
	}))
	defer upstream.Close()
	host, port := hostPortFromURL(t, upstream.URL)
	manager, err := NewManager(Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
		Host: host, Port: port, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.gateway.active.Load().directDNS = fixedDirectResolver{
		addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}},
	}

	response, err := manager.HTTPClient().Get("http://rebinding.example/private")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("loopback alias response = %d", response.StatusCode)
	}
	if proxyHits.Load() != 0 {
		t.Fatal("hostname resolving to loopback reached the configured proxy")
	}
}

func TestTrustedProxyRouteFailsClosedWhenLocalSafetyLookupFails(t *testing.T) {
	t.Parallel()
	var proxyHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyHits.Add(1)
		http.Error(w, "escaped", http.StatusBadGateway)
	}))
	defer upstream.Close()
	host, port := hostPortFromURL(t, upstream.URL)
	manager, err := NewManager(Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
		Host: host, Port: port, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.gateway.active.Load().directDNS = fixedDirectResolver{
		err: &net.DNSError{Err: "no such host", Name: "remote-only.example", IsNotFound: true},
	}

	response, err := manager.HTTPClient().Get("http://remote-only.example/resource")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("failed safety lookup response = %d", response.StatusCode)
	}
	if proxyHits.Load() != 0 {
		t.Fatal("destination reached the proxy without a completed local safety lookup")
	}
}

func TestTrustedProxyRouteDelegatesLogicalHostnameAfterLoopbackCheck(t *testing.T) {
	t.Parallel()
	for _, mode := range []settings.ProxyMode{settings.ProxyModeManual, settings.ProxyModeSystem} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			connectTarget := make(chan string, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodConnect {
					http.Error(w, "CONNECT required", http.StatusBadRequest)
					return
				}
				connectTarget <- request.Host
				connection, buffered, hijackErr := w.(http.Hijacker).Hijack()
				if hijackErr != nil {
					return
				}
				defer connection.Close()
				_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
				_ = buffered.Flush()
				tunneled, readErr := http.ReadRequest(buffered.Reader)
				if readErr != nil {
					return
				}
				tunneled.Body.Close()
				_, _ = io.WriteString(connection, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
			}))
			defer upstream.Close()
			host, port := hostPortFromURL(t, upstream.URL)
			config := Config{Mode: mode, Scheme: settings.ProxySchemeHTTP, Timeout: 2 * time.Second}
			if mode == settings.ProxyModeManual {
				config.Host, config.Port = host, port
			}
			manager, err := NewManager(config)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			state := manager.gateway.active.Load()
			if mode == settings.ProxyModeSystem {
				state.systemProxy.Close()
				proxyURL, parseErr := url.Parse(upstream.URL)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				state.systemProxy = &fixedSystemRouteResolver{proxy: proxyURL}
			}
			// Reproduce a filtered/poisoned local DNS answer. The local lookup is
			// still consulted for the loopback-alias guard, but must not become
			// CONNECT's destination when the trusted proxy owns working DNS.
			state.directDNS = fixedDirectResolver{
				addresses: []net.IPAddr{{IP: net.ParseIP("157.240.7.20")}},
			}

			response, err := manager.HTTPClient().Get("http://music.example/probe")
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				t.Fatalf("proxied response = %d", response.StatusCode)
			}
			select {
			case got := <-connectTarget:
				if got != "music.example:80" {
					t.Fatalf("CONNECT target = %q, want logical hostname", got)
				}
			case <-time.After(time.Second):
				t.Fatal("trusted proxy did not receive CONNECT")
			}
		})
	}
}

func TestSystemSOCKSPlainHTTPRejectsAuthenticationDowngrade(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer connection.Close()
		header := make([]byte, 2)
		if _, readErr := io.ReadFull(connection, header); readErr != nil {
			serverDone <- readErr
			return
		}
		methods := make([]byte, int(header[1]))
		if _, readErr := io.ReadFull(connection, methods); readErr != nil {
			serverDone <- readErr
			return
		}
		if header[0] != 0x05 || !bytes.Equal(methods, []byte{0x02}) {
			serverDone <- fmt.Errorf("SOCKS methods = %v", methods)
			return
		}
		_, writeErr := connection.Write([]byte{0x05, 0x00})
		serverDone <- writeErr
	}()

	manager, err := NewManager(Config{Mode: settings.ProxyModeSystem, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	state := manager.gateway.active.Load()
	state.systemProxy.Close()
	state.systemProxy = &fixedSystemRouteResolver{proxy: &url.URL{
		Scheme: "socks5", Host: listener.Addr().String(), User: url.UserPassword("alice", "wonderland"),
	}}
	response, err := manager.HTTPClient().Get("http://192.0.2.99/plain-http")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("downgraded SOCKS route response = %d", response.StatusCode)
	}
	select {
	case serverErr := <-serverDone:
		if serverErr != nil {
			t.Fatal(serverErr)
		}
	case <-time.After(time.Second):
		t.Fatal("system SOCKS route did not reach the strict handshake")
	}
}

func TestHTTPConnectProxyResponseHeadersAreBounded(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer connection.Close()
		request, readErr := http.ReadRequest(bufio.NewReader(connection))
		if readErr != nil {
			serverDone <- readErr
			return
		}
		request.Body.Close()
		_, writeErr := io.WriteString(connection, "HTTP/1.1 200 Connection Established\r\nX-Fill: "+strings.Repeat("a", gatewayProxyConnectHeaderBytes)+"\r\n\r\n")
		serverDone <- writeErr
	}()

	connection, err := dialHTTPConnectProxy(context.Background(), &url.URL{
		Scheme: "http", Host: listener.Addr().String(),
	}, "192.0.2.99:443", 2*time.Second)
	if connection != nil {
		connection.Close()
		t.Fatal("oversized CONNECT headers returned a tunnel")
	}
	if err == nil || !strings.Contains(err.Error(), "safety limit") {
		t.Fatalf("oversized CONNECT response error = %v", err)
	}
	select {
	case serverErr := <-serverDone:
		if serverErr != nil {
			t.Fatal(serverErr)
		}
	case <-time.After(time.Second):
		t.Fatal("oversized CONNECT server did not finish")
	}
}

func TestProxyHandshakeHelpersRequireGenerationOwnedDialer(t *testing.T) {
	t.Parallel()
	proxyURL, _ := url.Parse("http://127.0.0.1:9")
	if connection, err := dialHTTPConnectProxyWithDialer(
		context.Background(), proxyURL, "192.0.2.99:443", time.Second, nil,
	); err == nil || connection != nil || !strings.Contains(err.Error(), "generation-owned") {
		t.Fatalf("HTTP CONNECT without managed dialer = %v, %v", connection, err)
	}
	if connection, err := dialSOCKS5WithDialer(
		context.Background(), "127.0.0.1:9", "192.0.2.99:443", "", "", time.Second, nil,
	); err == nil || connection != nil || !strings.Contains(err.Error(), "generation-owned") {
		t.Fatalf("SOCKS5 without managed dialer = %v, %v", connection, err)
	}
}

func TestApplyCancelsPendingConnectHandshakeAndClosesOldProxySocket(t *testing.T) {
	proxyListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxyListener.Close()
	proxyReceived := make(chan struct{})
	proxyClosed := make(chan error, 1)
	go func() {
		connection, acceptErr := proxyListener.Accept()
		if acceptErr != nil {
			proxyClosed <- acceptErr
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		request, readErr := http.ReadRequest(reader)
		if readErr != nil {
			proxyClosed <- readErr
			return
		}
		request.Body.Close()
		close(proxyReceived)
		_, readErr = reader.ReadByte()
		proxyClosed <- readErr
	}()

	host, portText, _ := net.SplitHostPort(proxyListener.Addr().String())
	var port int
	_, _ = fmt.Sscanf(portText, "%d", &port)
	manager, err := NewManager(Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
		Host: host, Port: port, Username: "generation-user", Password: "generation-secret", Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	client, err := net.DialTimeout("tcp", strings.TrimPrefix(manager.GatewayURL(), "http://"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, _ = io.WriteString(client, "CONNECT 1.1.1.1:443 HTTP/1.1\r\nHost: 1.1.1.1:443\r\n\r\n")
	select {
	case <-proxyReceived:
	case <-time.After(time.Second):
		t.Fatal("old proxy did not receive the pending CONNECT")
	}

	started := time.Now()
	if err := manager.Apply(directConfig()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Apply waited for the old 30s handshake timeout: %s", elapsed)
	}
	select {
	case closeErr := <-proxyClosed:
		if closeErr == nil {
			t.Fatal("old proxy connection remained open after Apply")
		}
	case <-time.After(time.Second):
		t.Fatal("Apply did not close the pending old-generation proxy socket")
	}
}
