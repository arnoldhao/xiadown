package libraryserver

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestServerUsesBoundedMetadataWritesWhileAssetsMayRefreshDeadline(t *testing.T) {
	server, err := New(Config{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if server.config.WriteTimeout != 30*time.Second {
		t.Fatalf("default WriteTimeout = %v, want 30s metadata bound", server.config.WriteTimeout)
	}
	if server.config.ReadTimeout <= 0 || server.config.IdleTimeout <= 0 {
		t.Fatalf("request-side defaults were not retained: read=%v idle=%v", server.config.ReadTimeout, server.config.IdleTimeout)
	}

	explicit, err := New(Config{Handler: http.NotFoundHandler(), WriteTimeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("New explicit timeout: %v", err)
	}
	if explicit.config.WriteTimeout != 2*time.Minute {
		t.Fatalf("explicit WriteTimeout = %v", explicit.config.WriteTimeout)
	}
}

func TestServerBoundsConcurrentRequestsAcrossTransports(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server, err := New(Config{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			w.WriteHeader(http.StatusNoContent)
		}),
		MaxConcurrentRequests: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.newHTTPServer().Handler
	firstDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/library", nil))
		close(firstDone)
	}()
	<-started
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if second.Code != http.StatusServiceUnavailable || second.Header().Get("Retry-After") != "1" {
		t.Fatalf("overload response = %d %#v %s", second.Code, second.Header(), second.Body.String())
	}
	close(release)
	<-firstDone
}

func TestTLSFingerprintIsSynchronizedWithLANIdentityRefresh(t *testing.T) {
	server, err := New(Config{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	identities := []*TLSIdentity{{Fingerprint: "first"}, {Fingerprint: "second"}}
	var wait sync.WaitGroup
	wait.Add(3)
	go func() {
		defer wait.Done()
		for index := 0; index < 10_000; index++ {
			server.mu.Lock()
			server.config.TLSIdentity = identities[index%len(identities)]
			server.mu.Unlock()
		}
	}()
	for reader := 0; reader < 2; reader++ {
		go func() {
			defer wait.Done()
			for index := 0; index < 10_000; index++ {
				fingerprint := server.TLSFingerprint()
				if fingerprint != "" && fingerprint != "first" && fingerprint != "second" {
					t.Errorf("unexpected fingerprint %q", fingerprint)
					return
				}
			}
		}()
	}
	wait.Wait()
}

func TestServerBindsIPv4AndIPv6OnOneLANPort(t *testing.T) {
	probe, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	_ = probe.Close()

	directory := t.TempDir()
	identity, err := LoadOrCreateCertificate(CertificateFiles{
		CertificatePath: filepath.Join(directory, "library.crt"),
		PrivateKeyPath:  filepath.Join(directory, "library.key"),
	}, CertificateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	info, err := server.EnableLANAddresses([]string{"127.0.0.1:0", "[::1]:0"}, &identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Addresses) != 2 || info.Port == 0 {
		t.Fatalf("LAN info = %#v", info)
	}
	for _, address := range info.Addresses {
		_, port, splitErr := net.SplitHostPort(address)
		if splitErr != nil || port != portString(info.Port) {
			t.Fatalf("listener %q does not share port %d", address, info.Port)
		}
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13, InsecureSkipVerify: true, // generated certificate is pinned in production
		}}}
		response, requestErr := client.Get("https://" + address)
		if requestErr != nil {
			t.Fatalf("GET %s: %v", address, requestErr)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("GET %s status = %d", address, response.StatusCode)
		}
	}
}
