package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"xiadown/internal/domain/settings"
)

func dialSOCKS5(ctx context.Context, socksAddress, target, username, password string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return dialSOCKS5WithDialer(ctx, socksAddress, target, username, password, timeout, dialer.DialContext)
}

func TestDialSOCKS5SupportsUsernamePasswordAuthentication(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()
		serverDone <- serveAuthenticatedSOCKSHandshake(conn, "alice", "wonderland")
	}()

	conn, err := dialSOCKS5(context.Background(), listener.Addr().String(), "music.example:443", "alice", "wonderland", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("SOCKS5 server did not complete")
	}
}

func TestDialSOCKS5RejectsAuthenticationDowngrade(t *testing.T) {
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
		if header[0] != 0x05 || len(methods) != 1 || methods[0] != 0x02 {
			serverDone <- io.ErrUnexpectedEOF
			return
		}
		_, writeErr := connection.Write([]byte{0x05, 0x00})
		serverDone <- writeErr
	}()

	connection, err := dialSOCKS5(
		context.Background(),
		listener.Addr().String(),
		"music.example:443",
		"alice",
		"wonderland",
		time.Second,
	)
	if connection != nil {
		connection.Close()
		t.Fatal("SOCKS5 authentication downgrade returned a connection")
	}
	if err == nil || !strings.Contains(err.Error(), "not offered") {
		t.Fatalf("downgrade error = %v", err)
	}
	select {
	case serverErr := <-serverDone:
		if serverErr != nil {
			t.Fatal(serverErr)
		}
	case <-time.After(time.Second):
		t.Fatal("SOCKS5 downgrade server did not complete")
	}
}

func TestTrustedSOCKSRouteUsesDomainAddressAfterLocalLoopbackCheck(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	observed := make(chan socks5ObservedTarget, 1)
	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer connection.Close()
		serverDone <- serveNoAuthSOCKSHTTP(connection, observed)
	}()

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeSocks5,
		Host: host, Port: port, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	// The safety lookup sees a non-loopback but poisoned answer. SOCKS must
	// still receive the logical hostname using domain-name ATYP (0x03).
	manager.gateway.active.Load().directDNS = fixedDirectResolver{
		addresses: []net.IPAddr{{IP: net.ParseIP("157.240.7.20")}},
	}

	response, requestErr := manager.HTTPClient().Get("http://music.example/probe")
	if response != nil {
		response.Body.Close()
	}
	select {
	case serverErr := <-serverDone:
		if serverErr != nil {
			t.Fatal(serverErr)
		}
	case <-time.After(time.Second):
		t.Fatal("SOCKS5 server did not complete")
	}
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("proxied response = %d", response.StatusCode)
	}
	select {
	case target := <-observed:
		if target.addressType != 0x03 || target.host != "music.example" || target.port != 80 {
			t.Fatalf("SOCKS target = ATYP %#x %q:%d", target.addressType, target.host, target.port)
		}
	default:
		t.Fatal("SOCKS5 server did not record a destination")
	}
}

func TestSOCKSRoutePreservesCredentialsAndStrictNoProxy(t *testing.T) {
	t.Parallel()
	state, err := newRouteState(Config{
		Mode: settings.ProxyModeManual, Scheme: settings.ProxySchemeSocks5,
		Host: "proxy.example", Port: 1080, Username: "alice", Password: "wonderland",
		NoProxy: []string{"example.com:443"}, Timeout: time.Second,
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer state.close()

	bypassed, err := state.proxyForRequest(&http.Request{URL: &url.URL{Scheme: "https", Host: "api.example.com:443"}})
	if err != nil || bypassed != nil {
		t.Fatalf("NoProxy SOCKS route = %v, %v; want direct", bypassed, err)
	}
	proxied, err := state.proxyForRequest(&http.Request{URL: &url.URL{Scheme: "https", Host: "api.example.com:8443"}})
	if err != nil {
		t.Fatal(err)
	}
	if proxied == nil || proxied.Scheme != "socks5" || proxied.Host != "proxy.example:1080" {
		t.Fatalf("SOCKS route = %v", proxied)
	}
	password, hasPassword := proxied.User.Password()
	if proxied.User.Username() != "alice" || !hasPassword || password != "wonderland" {
		t.Fatal("SOCKS route lost authentication credentials")
	}
}

func serveAuthenticatedSOCKSHandshake(conn net.Conn, wantUsername, wantPassword string) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}
	foundAuth := false
	foundNoAuth := false
	for _, method := range methods {
		foundAuth = foundAuth || method == 0x02
		foundNoAuth = foundNoAuth || method == 0x00
	}
	if header[0] != 0x05 || !foundAuth || foundNoAuth {
		return io.ErrUnexpectedEOF
	}
	if _, err := conn.Write([]byte{0x05, 0x02}); err != nil {
		return err
	}
	authHeader := make([]byte, 2)
	if _, err := io.ReadFull(conn, authHeader); err != nil {
		return err
	}
	username := make([]byte, int(authHeader[1]))
	if _, err := io.ReadFull(conn, username); err != nil {
		return err
	}
	passwordLength := []byte{0}
	if _, err := io.ReadFull(conn, passwordLength); err != nil {
		return err
	}
	password := make([]byte, int(passwordLength[0]))
	if _, err := io.ReadFull(conn, password); err != nil {
		return err
	}
	if authHeader[0] != 0x01 || string(username) != wantUsername || string(password) != wantPassword {
		return io.ErrUnexpectedEOF
	}
	if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
		return err
	}

	connectHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, connectHeader); err != nil {
		return err
	}
	var err error
	switch connectHeader[3] {
	case 0x01:
		_, err = io.CopyN(io.Discard, conn, 4)
	case 0x04:
		_, err = io.CopyN(io.Discard, conn, 16)
	case 0x03:
		length := []byte{0}
		if _, err = io.ReadFull(conn, length); err == nil {
			_, err = io.CopyN(io.Discard, conn, int64(length[0]))
		}
	default:
		return io.ErrUnexpectedEOF
	}
	if err != nil {
		return err
	}
	if _, err := io.CopyN(io.Discard, conn, 2); err != nil {
		return err
	}
	_, err = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	return err
}

type socks5ObservedTarget struct {
	addressType byte
	host        string
	port        uint16
}

func serveNoAuthSOCKSHTTP(connection net.Conn, observed chan<- socks5ObservedTarget) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(connection, header); err != nil {
		return err
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(connection, methods); err != nil {
		return err
	}
	if header[0] != 0x05 || len(methods) != 1 || methods[0] != 0x00 {
		return fmt.Errorf("SOCKS methods = version %#x, methods %v", header[0], methods)
	}
	if _, err := connection.Write([]byte{0x05, 0x00}); err != nil {
		return err
	}

	connectHeader := make([]byte, 4)
	if _, err := io.ReadFull(connection, connectHeader); err != nil {
		return err
	}
	if connectHeader[0] != 0x05 || connectHeader[1] != 0x01 || connectHeader[2] != 0x00 || connectHeader[3] != 0x03 {
		return fmt.Errorf("SOCKS CONNECT header = %v", connectHeader)
	}
	length := []byte{0}
	if _, err := io.ReadFull(connection, length); err != nil {
		return err
	}
	host := make([]byte, int(length[0]))
	if _, err := io.ReadFull(connection, host); err != nil {
		return err
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(connection, portBytes); err != nil {
		return err
	}
	observed <- socks5ObservedTarget{
		addressType: connectHeader[3],
		host:        string(host),
		port:        uint16(portBytes[0])<<8 | uint16(portBytes[1]),
	}
	if _, err := connection.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return err
	}

	request, err := http.ReadRequest(bufio.NewReader(connection))
	if err != nil {
		return err
	}
	request.Body.Close()
	if request.Host != "music.example" {
		return fmt.Errorf("tunneled Host = %q", request.Host)
	}
	_, err = io.WriteString(connection, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	return err
}
