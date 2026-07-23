package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"xiadown/internal/domain/settings"
)

func directConfig() Config {
	return Config{Mode: settings.ProxyModeNone, Scheme: settings.ProxySchemeHTTP, Timeout: 2 * time.Second}
}

func TestManagerGatewayIsImmediateStableAndUsedByHTTPClient(t *testing.T) {
	t.Parallel()
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Destination", "direct")
		_, _ = io.WriteString(w, "ok")
	}))
	defer destination.Close()

	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	registerInternalTestTarget(t, manager, destination.URL)

	gatewayURL := manager.GatewayURL()
	parsedGateway, err := url.Parse(gatewayURL)
	if err != nil {
		t.Fatalf("invalid gateway URL %q: %v", gatewayURL, err)
	}
	if ip := net.ParseIP(parsedGateway.Hostname()); ip == nil || !ip.IsLoopback() {
		t.Fatalf("gateway must bind loopback, got %q", parsedGateway.Hostname())
	}
	if manager.ConsumerProxyURL() != gatewayURL {
		t.Fatalf("consumer URL = %q, gateway URL = %q", manager.ConsumerProxyURL(), gatewayURL)
	}
	resolved, err := manager.ResolveProxy(destination.URL)
	if err != nil || resolved != gatewayURL {
		t.Fatalf("ResolveProxy = %q, %v; want %q", resolved, err, gatewayURL)
	}

	client := manager.HTTPClient()
	response, err := client.Get(destination.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.Header.Get("X-Destination") != "direct" {
		t.Fatalf("request did not pass through destination")
	}

	nextConfig := directConfig()
	nextConfig.Timeout = 3 * time.Second
	if err := manager.Apply(nextConfig); err != nil {
		t.Fatal(err)
	}
	if manager.GatewayURL() != gatewayURL {
		t.Fatalf("gateway changed across Apply: %q -> %q", gatewayURL, manager.GatewayURL())
	}
	if manager.HTTPClient() == client {
		t.Fatal("HTTPClient was not refreshed for the applied timeout policy")
	}
	if manager.HTTPClient().Timeout != 3*time.Second {
		t.Fatalf("HTTPClient timeout = %s", manager.HTTPClient().Timeout)
	}
	if manager.Generation() != 2 {
		t.Fatalf("generation = %d, want 2", manager.Generation())
	}
}

func TestGatewayAttestationIsPrivateStableAndNeverLeavesGateway(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	attestationURL, token := manager.ConsumerProxyAttestation()
	if attestationURL == "" || token == "" || strings.Contains(attestationURL, manager.GatewayURL()) {
		t.Fatalf("invalid attestation URL/token: %q %q", attestationURL, token)
	}
	if strings.Contains(attestationURL, token) {
		t.Fatal("attestation secret leaked into the request URL")
	}
	attestationControl, err := url.Parse(attestationURL)
	if err != nil || !strings.HasSuffix(attestationControl.Hostname(), gatewayAttestationHostSuffix) {
		t.Fatalf("attestation did not use a private random authority: %q, %v", attestationURL, err)
	}
	randomLabel := strings.TrimSuffix(attestationControl.Hostname(), gatewayAttestationHostSuffix)
	if len(randomLabel) != 32 {
		t.Fatalf("attestation authority random label = %q", randomLabel)
	}
	proxyURL, err := url.Parse(manager.ConsumerProxyURL())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := client.Get(attestationURL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	challengeURL := response.Header.Get(gatewayConnectChallengeHeader)
	if response.StatusCode != http.StatusNoContent || challengeURL == "" || response.Header.Get(gatewayAttestationHeader) != "" {
		t.Fatalf("attestation begin response = %d, challenge %q, token %q", response.StatusCode, challengeURL, response.Header.Get(gatewayAttestationHeader))
	}
	if response.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("attestation response exposed its control-plane headers to page JavaScript")
	}
	challenge, err := url.Parse(challengeURL)
	expectedChallengeSuffix := gatewayConnectChallengeInfix + attestationControl.Hostname()
	if err != nil || !strings.HasSuffix(challenge.Hostname(), expectedChallengeSuffix) {
		t.Fatalf("invalid CONNECT challenge URL %q: %v", challengeURL, err)
	}
	proofID := strings.TrimSuffix(challenge.Hostname(), expectedChallengeSuffix)
	if len(proofID) != gatewayAttestationProofBytes*2 {
		t.Fatalf("CONNECT proof length = %d", len(proofID))
	}
	// Beginning challenges is stateless: an untrusted page cannot exhaust the
	// bounded table merely by issuing cross-origin no-cors HTTP requests.
	for range gatewayAttestationProofLimit * 2 {
		floodResponse, floodErr := client.Get(attestationURL)
		if floodErr != nil {
			t.Fatal(floodErr)
		}
		floodResponse.Body.Close()
		if floodResponse.StatusCode != http.StatusNoContent {
			t.Fatalf("flood begin response = %d", floodResponse.StatusCode)
		}
	}
	manager.gateway.attestationMu.Lock()
	pendingProofs := len(manager.gateway.attestationProof)
	manager.gateway.attestationMu.Unlock()
	if pendingProofs != 0 {
		t.Fatalf("stateless attestation begin allocated %d proof records", pendingProofs)
	}
	gatewayConnection, err := net.DialTimeout("tcp", proxyURL.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprintf(gatewayConnection, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", challenge.Host, challenge.Host)
	connectResponse, err := http.ReadResponse(bufio.NewReader(gatewayConnection), &http.Request{Method: http.MethodConnect})
	_ = gatewayConnection.Close()
	if err != nil || connectResponse.StatusCode != http.StatusBadGateway {
		t.Fatalf("CONNECT challenge response = %#v, %v", connectResponse, err)
	}
	connectResponse.Body.Close()
	completion, _ := url.Parse(attestationURL)
	query := completion.Query()
	query.Set("proof", proofID)
	completion.RawQuery = query.Encode()
	response, err = client.Get(completion.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || response.Header.Get(gatewayAttestationHeader) != token {
		t.Fatalf("attestation completion response = %d, %q", response.StatusCode, response.Header.Get(gatewayAttestationHeader))
	}
	response, err = client.Get(completion.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden || response.Header.Get(gatewayAttestationHeader) != "" {
		t.Fatalf("one-shot attestation proof was replayable: %d, %q", response.StatusCode, response.Header.Get(gatewayAttestationHeader))
	}

	nextConfig := directConfig()
	nextConfig.Timeout = 3 * time.Second
	if err := manager.Apply(nextConfig); err != nil {
		t.Fatal(err)
	}
	nextURL, nextToken := manager.ConsumerProxyAttestation()
	if nextURL != attestationURL || nextToken != token {
		t.Fatal("attestation changed across policy generation")
	}
}

func TestGatewayAttestationAuthorityNeverForwardsUnknownRoutes(t *testing.T) {
	t.Parallel()
	var proxyRequests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyRequests.Add(1)
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

	attestationURL, _ := manager.ConsumerProxyAttestation()
	unknown, _ := url.Parse(attestationURL)
	unknown.Path = "/untrusted-same-origin-page"
	proxyURL, _ := url.Parse(manager.ConsumerProxyURL())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := client.Get(unknown.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown attestation route response = %d", response.StatusCode)
	}
	if proxyRequests.Load() != 0 {
		t.Fatal("private attestation authority escaped to the configured proxy")
	}
}

func TestGatewayAttestationProofIsSignedAndShortLived(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	now := time.Unix(1_800_000_000, 0)
	proofID, err := manager.gateway.beginConnectAttestation(now)
	if err != nil {
		t.Fatal(err)
	}
	if !manager.gateway.validAttestationProofID(proofID, now) {
		t.Fatal("new attestation proof was invalid")
	}
	tampered := proofID[:len(proofID)-1] + "0"
	if tampered == proofID {
		tampered = proofID[:len(proofID)-1] + "1"
	}
	if manager.gateway.validAttestationProofID(tampered, now) {
		t.Fatal("tampered attestation proof was accepted")
	}
	if manager.gateway.validAttestationProofID(proofID, now.Add(gatewayAttestationProofTTL+time.Second)) {
		t.Fatal("expired attestation proof was accepted")
	}
}

func TestGatewayPreservesMediaRangeRequests(t *testing.T) {
	t.Parallel()
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Range") != "bytes=4-7" {
			http.Error(w, "range missing", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", "bytes 4-7/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "4567")
	}))
	defer destination.Close()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	registerInternalTestTarget(t, manager, destination.URL)
	request, _ := http.NewRequest(http.MethodGet, destination.URL, nil)
	request.Header.Set("Range", "bytes=4-7")
	response, err := manager.HTTPClient().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	payload, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusPartialContent || string(payload) != "4567" || response.Header.Get("Content-Range") != "bytes 4-7/10" {
		t.Fatalf("range response = status %d body %q content-range %q err %v", response.StatusCode, payload, response.Header.Get("Content-Range"), readErr)
	}
}

func TestGatewayAllowsOnlyRegisteredLoopbackTargets(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()

	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	response, err := manager.HTTPClient().Get(destination.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden || response.Header.Get("X-XiaDown-Network-Error") != "loopback-denied" {
		t.Fatalf("unregistered loopback response = %d, %q", response.StatusCode, response.Header.Get("X-XiaDown-Network-Error"))
	}
	if requests.Load() != 0 {
		t.Fatal("unregistered loopback request reached the destination")
	}

	registerInternalTestTarget(t, manager, destination.URL+"/token-scoped-path")
	response, err = manager.HTTPClient().Get(destination.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || requests.Load() != 1 {
		t.Fatalf("registered loopback response = %d, requests = %d", response.StatusCode, requests.Load())
	}

	if err := manager.RegisterInternalLoopbackURL("https://example.com/"); err == nil {
		t.Fatal("registered a non-loopback internal target")
	}
}

func TestManagerManualHTTPProxyAuthAndNoProxy(t *testing.T) {
	t.Parallel()
	var proxyRequests atomic.Int32
	var gotAuthorization atomic.Value
	upstreamProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		proxyRequests.Add(1)
		gotAuthorization.Store(request.Header.Get("Proxy-Authorization"))
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
		_, _ = io.WriteString(connection, "HTTP/1.1 202 Accepted\r\nX-Upstream-Proxy: used\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	}))
	defer upstreamProxy.Close()

	host, port := hostPortFromURL(t, upstreamProxy.URL)
	manager, err := NewManager(Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
		Host: host, Port: port, Username: "proxy-user", Password: "proxy-password", Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	response, err := manager.HTTPClient().Get("http://192.0.2.99/resource")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted || response.Header.Get("X-Upstream-Proxy") != "used" {
		t.Fatalf("unexpected proxied response: %d, %q", response.StatusCode, response.Header.Get("X-Upstream-Proxy"))
	}
	wantAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte("proxy-user:proxy-password"))
	if got, _ := gotAuthorization.Load().(string); got != wantAuthorization {
		t.Fatalf("Proxy-Authorization = %q, want %q", got, wantAuthorization)
	}
	if proxyRequests.Load() != 1 {
		t.Fatalf("initial proxied request count = %d", proxyRequests.Load())
	}

	config := manager.CurrentConfig()
	config.NoProxy = []string{"192.0.2.99"}
	if err := manager.Apply(config); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, "http://192.0.2.99/resource", nil)
	ctx, cancel := context.WithTimeout(request.Context(), 500*time.Millisecond)
	defer cancel()
	response, err = manager.HTTPClient().Do(request.WithContext(ctx))
	if err == nil {
		response.Body.Close()
		if response.StatusCode != http.StatusBadGateway {
			t.Fatalf("NoProxy request status = %d, want gateway failure", response.StatusCode)
		}
	}
	if proxyRequests.Load() != 1 {
		t.Fatalf("NoProxy request reached upstream proxy; count = %d", proxyRequests.Load())
	}
}

func TestApplyRejectsInvalidPolicyWithoutChangingActiveGeneration(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	gatewayURL := manager.GatewayURL()

	err = manager.Apply(Config{Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP, Timeout: time.Second})
	if err == nil {
		t.Fatal("invalid manual proxy was accepted")
	}
	if manager.Generation() != 1 {
		t.Fatalf("invalid Apply changed generation to %d", manager.Generation())
	}
	if manager.CurrentConfig().Mode != settings.ProxyModeNone || manager.GatewayURL() != gatewayURL {
		t.Fatal("invalid Apply changed active policy")
	}
}

func TestApplyClosesActiveConnectTunnel(t *testing.T) {
	t.Parallel()
	targetListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer targetListener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := targetListener.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()

	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	registerInternalTestTarget(t, manager, "http://"+targetListener.Addr().String())
	gatewayAddress := strings.TrimPrefix(manager.GatewayURL(), "http://")
	clientConn, err := net.DialTimeout("tcp", gatewayAddress, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
	_, _ = fmt.Fprintf(clientConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetListener.Addr(), targetListener.Addr())
	reader := bufio.NewReader(clientConn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d", response.StatusCode)
	}

	var targetConn net.Conn
	select {
	case targetConn = <-accepted:
		defer targetConn.Close()
	case <-time.After(time.Second):
		t.Fatal("gateway did not establish target connection")
	}
	if err := manager.Apply(directConfig()); err != nil {
		t.Fatal(err)
	}
	_ = clientConn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("CONNECT tunnel remained open after Apply")
	}
}

func TestGatewayConnectPreservesTCPHalfClose(t *testing.T) {
	t.Parallel()
	targetListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer targetListener.Close()
	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := targetListener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer connection.Close()
		payload, readErr := io.ReadAll(connection)
		if readErr != nil {
			serverDone <- readErr
			return
		}
		if string(payload) != "request" {
			serverDone <- fmt.Errorf("payload = %q", payload)
			return
		}
		_, writeErr := connection.Write([]byte("response"))
		serverDone <- writeErr
	}()

	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	registerInternalTestTarget(t, manager, "http://"+targetListener.Addr().String())
	connection, err := net.DialTimeout("tcp", strings.TrimPrefix(manager.GatewayURL(), "http://"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_, _ = fmt.Fprintf(connection, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetListener.Addr(), targetListener.Addr())
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT response = %#v, %v", response, err)
	}
	if _, err := connection.Write([]byte("request")); err != nil {
		t.Fatal(err)
	}
	if err := connection.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(reader)
	if err != nil || string(payload) != "response" {
		t.Fatalf("half-close response = %q, %v", payload, err)
	}
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("target did not observe half-close")
	}
}

func TestGatewayRejectsRecursiveSelfRoute(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	response, err := manager.HTTPClient().Get(manager.GatewayURL())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("self route status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
	if got := response.Header.Get("X-XiaDown-Network-Error"); got != "gateway-loop" {
		t.Fatalf("self route error class = %q", got)
	}
}

func TestGatewayForwardsWebSocketUpgradeAndClosesItOnApply(t *testing.T) {
	t.Parallel()
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if httpUpgradeType(request.Header) != "websocket" {
			http.Error(w, "upgrade required", http.StatusBadRequest)
			return
		}
		connection, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		_ = buffered.Flush()
		_, _ = io.Copy(connection, buffered)
	}))
	defer destination.Close()

	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	registerInternalTestTarget(t, manager, destination.URL)
	gatewayAddress := strings.TrimPrefix(manager.GatewayURL(), "http://")
	destinationURL, err := url.Parse(destination.URL)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", gatewayAddress, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_, _ = fmt.Fprintf(
		connection,
		"GET ws://%s/socket HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n",
		destinationURL.Host,
		destinationURL.Host,
	)
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade status = %d", response.StatusCode)
	}
	if got := response.Header.Get("X-XiaDown-Network-Generation"); got != "1" {
		t.Fatalf("upgrade generation = %q", got)
	}
	if _, err := connection.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, 4)
	if _, err := io.ReadFull(reader, echo); err != nil || string(echo) != "ping" {
		t.Fatalf("websocket echo = %q, %v", echo, err)
	}
	if err := manager.Apply(directConfig()); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("websocket tunnel remained open after Apply")
	}
}

func TestApplyCancelsActivePlainHTTPResponse(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("a"))
		flusher.Flush()
		close(started)
		<-request.Context().Done()
	}))
	defer destination.Close()

	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	registerInternalTestTarget(t, manager, destination.URL)
	response, err := manager.HTTPClient().Get(destination.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("streaming response did not start")
	}
	first := make([]byte, 1)
	if _, err := io.ReadFull(response.Body, first); err != nil || string(first) != "a" {
		t.Fatalf("initial response = %q, %v", first, err)
	}
	if err := manager.Apply(directConfig()); err != nil {
		t.Fatal(err)
	}
	readDone := make(chan error, 1)
	go func() {
		_, readErr := response.Body.Read(first)
		readDone <- readErr
	}()
	select {
	case readErr := <-readDone:
		if readErr == nil {
			t.Fatal("old generation response remained readable after Apply")
		}
	case <-time.After(time.Second):
		t.Fatal("old generation response was not cancelled")
	}
}

func TestGatewayRejectsOversizedHeaders(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	connection, err := net.DialTimeout("tcp", strings.TrimPrefix(manager.GatewayURL(), "http://"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_, _ = fmt.Fprintf(
		connection,
		"GET http://example.com/ HTTP/1.1\r\nHost: example.com\r\nX-Oversized: %s\r\n\r\n",
		strings.Repeat("x", gatewayMaxHeaderBytes+8192),
	)
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("oversized header status = %d", response.StatusCode)
	}
}

func TestCandidateTestDoesNotChangeActivePolicy(t *testing.T) {
	t.Parallel()
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		connection, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buffered.Flush()
		request, err := http.ReadRequest(buffered.Reader)
		if err != nil {
			return
		}
		request.Body.Close()
		_, _ = io.WriteString(connection, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	}))
	defer proxyServer.Close()
	host, port := hostPortFromURL(t, proxyServer.URL)

	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.testURL = "http://1.1.1.1/probe"
	candidate := Config{Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP, Host: host, Port: port, Timeout: time.Second}
	result, err := manager.Test(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("candidate test failed: %s", result.Message)
	}
	if manager.Generation() != 1 || manager.CurrentConfig().Mode != settings.ProxyModeNone {
		t.Fatal("candidate Test mutated active policy")
	}
}

func TestCandidateFailureMessageRedactsCredentials(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	host, portText, _ := net.SplitHostPort(address)
	var port int
	_, _ = fmt.Sscanf(portText, "%d", &port)

	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.testURL = "http://credential-check.test/"
	result, err := manager.Test(context.Background(), Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeHTTP,
		Host: host, Port: port, Username: "sensitive-user", Password: "sensitive-password", Timeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("candidate unexpectedly succeeded")
	}
	if strings.Contains(result.Message, "sensitive-user") || strings.Contains(result.Message, "sensitive-password") {
		t.Fatalf("candidate failure leaked credentials: %q", result.Message)
	}
}

func TestManagerDoesNotMutateGlobalProxyEnvironment(t *testing.T) {
	names := []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"}
	for _, name := range names {
		// Windows environment-variable names are case-insensitive, so the two
		// spellings of each proxy variable must share one expected value.
		t.Setenv(name, "sentinel-"+strings.ToUpper(name))
	}
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	config := directConfig()
	if err := manager.Apply(config); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if got := os.Getenv(name); got != "sentinel-"+strings.ToUpper(name) {
			t.Fatalf("%s mutated to %q", name, got)
		}
	}
}

func TestChildProxyEnvironmentIsScopedAndSanitized(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	base := []string{"PATH=/bin", "http_proxy=http://old", "Https_Proxy=http://old-two", "NO_PROXY=*", "CUSTOM=value"}
	got := manager.ChildProxyEnvironment(base)
	joined := "\n" + strings.Join(got, "\n") + "\n"
	if strings.Contains(joined, "old") || strings.Contains(joined, "NO_PROXY=*") {
		t.Fatalf("inherited proxy environment survived: %v", got)
	}
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		if !strings.Contains(joined, "\n"+name+"="+manager.GatewayURL()+"\n") {
			t.Fatalf("missing %s gateway binding in %v", name, got)
		}
	}
	if !strings.Contains(joined, "\nNO_PROXY=\n") || !strings.Contains(joined, "\nno_proxy=\n") || !strings.Contains(joined, "\nPATH=/bin\n") {
		t.Fatalf("unexpected child environment: %v", got)
	}
	if base[1] != "http_proxy=http://old" {
		t.Fatal("ChildProxyEnvironment mutated base slice")
	}
}

func TestChildProxyEnvironmentFailsClosedAfterManagerClose(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	closedGateway := manager.GatewayURL()
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	joined := "\n" + strings.Join(manager.ChildProxyEnvironment([]string{"HTTP_PROXY=http://inherited", "NO_PROXY=*"}), "\n") + "\n"
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		if !strings.Contains(joined, "\n"+name+"="+closedGateway+"\n") {
			t.Fatalf("closed child environment did not fail closed for %s: %s", name, joined)
		}
	}
	if strings.Contains(joined, "inherited") || strings.Contains(joined, "NO_PROXY=*") {
		t.Fatalf("closed child environment retained an inherited bypass: %s", joined)
	}
}

func TestCloseStopsGatewayAndResolveProxy(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(directConfig())
	if err != nil {
		t.Fatal(err)
	}
	address := strings.TrimPrefix(manager.GatewayURL(), "http://")
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := manager.ResolveProxy("https://example.com"); err == nil {
		t.Fatal("ResolveProxy succeeded after Close")
	}
	conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("gateway still accepted connections after Close")
	}
}

func TestParseWindowsProxyForScheme(t *testing.T) {
	t.Parallel()
	proxyURL, err := parseWindowsProxyForScheme("http=one.test:8080;https=two.test:8443;socks=three.test:1080", "https")
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyURL.String(); got != "http://two.test:8443" {
		t.Fatalf("HTTPS proxy = %q", got)
	}
	proxyURL, err = parseWindowsProxyForScheme("socks=three.test:1080", "https")
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyURL.String(); got != "socks5://three.test:1080" {
		t.Fatalf("SOCKS proxy = %q", got)
	}
}

func hostPortFromURL(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	_, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatal(err)
	}
	return parsed.Hostname(), port
}

func registerInternalTestTarget(t *testing.T, manager *Manager, rawURL string) {
	t.Helper()
	if err := manager.RegisterInternalLoopbackURL(rawURL); err != nil {
		t.Fatalf("register test network target %q: %v", rawURL, err)
	}
}
