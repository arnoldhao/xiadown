package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"xiadown/internal/domain/settings"
)

type recordingFixedSystemProxyResolver struct {
	mu      sync.Mutex
	proxy   *url.URL
	targets []string
}

func (resolver *recordingFixedSystemProxyResolver) Resolve(target *url.URL) (*url.URL, error) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	resolver.targets = append(resolver.targets, target.String())
	return resolver.proxy, nil
}

func (*recordingFixedSystemProxyResolver) Close() {}

func (resolver *recordingFixedSystemProxyResolver) recordedTargets() []string {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	return append([]string(nil), resolver.targets...)
}

func TestPublicDialContextRejectsSpecialDestinations(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	for _, address := range []string{
		"127.0.0.1:80",
		"10.1.2.3:443",
		"169.254.169.254:80",
		"100.64.1.2:80",
		"[::1]:443",
		"[fe80::1]:443",
		"[fd00::1]:443",
		"[64:ff9b::a9fe:a9fe]:443",
		"[64:ff9b:1::a9fe:a9fe]:443",
		"[2001::1]:443",
		"[2002:a9fe:a9fe::1]:443",
	} {
		t.Run(address, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if conn, err := manager.PublicDialContext(ctx, "tcp", address); err == nil {
				conn.Close()
				t.Fatalf("special destination %s was accepted", address)
			}
		})
	}
}

func TestPublicDialURLContextRejectsLogicalAuthorityMismatch(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	logicalURL, _ := url.Parse("https://1.0.0.1/private/path")
	connection, err := manager.PublicDialURLContext(context.Background(), "tcp", "1.1.1.1:443", logicalURL)
	if connection != nil {
		connection.Close()
		t.Fatal("mismatched public logical URL returned a connection")
	}
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched public logical URL error = %v", err)
	}
}

func TestPublicHTTPClientUsesActiveManualProxyAndPinnedPublicTarget(t *testing.T) {
	t.Parallel()
	var connectCount atomic.Int32
	var connectTarget atomic.Value
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusBadRequest)
			return
		}
		connectCount.Add(1)
		connectTarget.Store(request.Host)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking unavailable", http.StatusInternalServerError)
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = rw.Flush()
		tunneledRequest, err := http.ReadRequest(rw.Reader)
		if err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, tunneledRequest.Body)
		tunneledRequest.Body.Close()
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 6\r\nConnection: close\r\n\r\npublic")
	}))
	defer proxyServer.Close()

	host, port := hostPortFromURL(t, proxyServer.URL)
	manager, err := NewManager(Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
		Host: host, Port: port, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	response, err := manager.PublicHTTPClient().Get("http://1.1.1.1/route-check")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "public" {
		t.Fatalf("public response = %d %q", response.StatusCode, body)
	}
	if connectCount.Load() != 1 {
		t.Fatalf("CONNECT count = %d", connectCount.Load())
	}
	if got, _ := connectTarget.Load().(string); got != "1.1.1.1:80" {
		t.Fatalf("pinned CONNECT target = %q", got)
	}
}

func TestPinnedPublicTransportNeverDelegatesLogicalHostname(t *testing.T) {
	t.Parallel()
	connectTarget := make(chan string, 1)
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusBadRequest)
			return
		}
		connectTarget <- request.Host
		connection, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buffered.Flush()
		tunneled, err := http.ReadRequest(buffered.Reader)
		if err != nil {
			return
		}
		tunneled.Body.Close()
		_, _ = io.WriteString(connection, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	}))
	defer proxyServer.Close()

	host, port := hostPortFromURL(t, proxyServer.URL)
	config := Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
		Host: host, Port: port, Timeout: 2 * time.Second,
	}
	state, err := newRouteState(config, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer state.close()
	transport, err := state.forwardTransport(
		"media.example:80",
		[]string{"1.1.1.1:80"},
		buildProxyURL(config),
		true,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	response, err := client.Get("http://media.example/resource")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("public response = %d", response.StatusCode)
	}
	select {
	case got := <-connectTarget:
		if got != "1.1.1.1:80" {
			t.Fatalf("public CONNECT target = %q, want pinned IP literal", got)
		}
	case <-time.After(time.Second):
		t.Fatal("public proxy did not receive CONNECT")
	}
}

func TestPublicRouteUsesCanonicalSystemPACOriginAndPinsSocketTarget(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var connectTargets []string
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusBadRequest)
			return
		}
		mu.Lock()
		connectTargets = append(connectTargets, request.Host)
		mu.Unlock()
		connection, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buffered.Flush()
	}))
	defer proxyServer.Close()
	proxyURL, _ := url.Parse(proxyServer.URL)
	resolver := &recordingFixedSystemProxyResolver{proxy: proxyURL}
	manager, err := NewManager(Config{Mode: settings.ProxyModeSystem, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	state := manager.gateway.active.Load()
	state.systemProxy.Close()
	state.systemProxy = resolver

	tests := []struct {
		logical string
		address string
	}{
		{logical: "http://1.1.1.1/pac/plain?track=one", address: "1.1.1.1:80"},
		{logical: "https://1.0.0.1/pac/tls?track=two", address: "1.0.0.1:443"},
	}
	for _, test := range tests {
		logicalURL, _ := url.Parse(test.logical)
		connection, dialErr := manager.PublicDialURLContext(context.Background(), "tcp", test.address, logicalURL)
		if dialErr != nil {
			t.Fatalf("dial %s: %v", test.logical, dialErr)
		}
		connection.Close()
	}

	if got := resolver.recordedTargets(); len(got) != len(tests) || got[0] != "http://1.1.1.1/" || got[1] != "https://1.0.0.1/" {
		t.Fatalf("system PAC targets = %v", got)
	}
	mu.Lock()
	gotConnectTargets := append([]string(nil), connectTargets...)
	mu.Unlock()
	if len(gotConnectTargets) != len(tests) || gotConnectTargets[0] != tests[0].address || gotConnectTargets[1] != tests[1].address {
		t.Fatalf("pinned CONNECT targets = %v", gotConnectTargets)
	}
}

func TestSystemPACOriginIsIdenticalForGatewayPublicAndWSSRoutes(t *testing.T) {
	t.Parallel()
	deadProxy, _ := url.Parse("http://127.0.0.1:1")
	resolver := &recordingFixedSystemProxyResolver{proxy: deadProxy}
	manager, err := NewManager(Config{Mode: settings.ProxyModeSystem, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	state := manager.gateway.active.Load()
	state.systemProxy.Close()
	state.systemProxy = resolver

	_, _ = manager.HTTPClient().Get("https://1.1.1.1/private/backend?secret=one")
	_, _ = manager.PublicHTTPClient().Get("https://1.1.1.1/private/public?secret=two")
	_, _ = state.proxyForLogicalURL(context.Background(), mustParseURL(t, "wss://1.1.1.1/private/socket?secret=three"))

	got := resolver.recordedTargets()
	want := []string{"https://1.1.1.1/", "https://1.1.1.1/", "https://1.1.1.1/"}
	if len(got) != len(want) {
		t.Fatalf("system origin decisions = %v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("system origin decision %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestApplyClosesPublicRouteConnections(t *testing.T) {
	t.Parallel()
	proxyListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxyListener.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := proxyListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		request, err := http.ReadRequest(reader)
		if err != nil || request.Method != http.MethodConnect {
			return
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		accepted <- struct{}{}
		buffer := make([]byte, 64)
		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				if _, writeErr := conn.Write(buffer[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	host, portText, _ := net.SplitHostPort(proxyListener.Addr().String())
	var port int
	_, _ = fmt.Sscanf(portText, "%d", &port)
	config := Config{Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP, Host: host, Port: port, Timeout: 2 * time.Second}
	manager, err := NewManager(config)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	conn, err := manager.PublicDialContext(context.Background(), "tcp", "1.1.1.1:443")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("manual proxy did not receive public CONNECT")
	}
	if _, err := io.WriteString(conn, "ping"); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 4)
	if _, err := io.ReadFull(conn, buffer); err != nil || string(buffer) != "ping" {
		t.Fatalf("echo before Apply = %q, %v", buffer, err)
	}
	if err := manager.Apply(config); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := conn.Read(buffer); err == nil || !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "reset") {
		t.Fatalf("old public connection was not closed by Apply: %v", err)
	}
}
