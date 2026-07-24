package libraryserver

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCertificateIsPersistedWithStableFingerprintAndPrivatePermissions(t *testing.T) {
	directory := t.TempDir()
	files := CertificateFiles{
		CertificatePath: filepath.Join(directory, "library-cert.pem"),
		PrivateKeyPath:  filepath.Join(directory, "library-key.pem"),
	}
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	first, err := LoadOrCreateCertificate(files, CertificateOptions{Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Fingerprint) != 64 || first.Fingerprint != strings.ToUpper(first.Fingerprint) {
		t.Fatalf("unexpected SHA-256 fingerprint: %q", first.Fingerprint)
	}
	if !first.Persistent || first.Rotated || first.PersistenceError != "" {
		t.Fatalf("new identity persistence metadata = %#v", first)
	}
	keyInfo, err := os.Stat(files.PrivateKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateTLSKey(t, files.PrivateKeyPath, keyInfo.Mode())
	keyBefore, err := os.ReadFile(files.PrivateKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateCertificate(files, CertificateOptions{Clock: func() time.Time { return now.Add(24 * time.Hour) }})
	if err != nil {
		t.Fatal(err)
	}
	keyAfter, err := os.ReadFile(files.PrivateKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint != second.Fingerprint || string(keyBefore) != string(keyAfter) {
		t.Fatal("loading a persisted identity unexpectedly rotated its pin")
	}
	if !second.Persistent || second.Rotated || second.PersistenceError != "" {
		t.Fatalf("loaded identity persistence metadata = %#v", second)
	}
	if first.Certificate.Leaf == nil || first.Certificate.Leaf.Subject.CommonName != "XiaDown Library" {
		t.Fatal("generated identity does not expose its parsed leaf")
	}
}

func TestCertificateNormalizesUnicodeDNSNames(t *testing.T) {
	directory := t.TempDir()
	identity, err := LoadOrCreateCertificate(CertificateFiles{
		CertificatePath: filepath.Join(directory, "library-cert.pem"),
		PrivateKeyPath:  filepath.Join(directory, "library-key.pem"),
	}, CertificateOptions{DNSNames: []string{"春风又绿江南岸"}})
	if err != nil {
		t.Fatalf("Unicode host name blocked certificate generation: %v", err)
	}
	if !identity.Persistent || identity.Certificate.Leaf == nil {
		t.Fatalf("generated identity = %#v", identity)
	}
	wantDNSNames := []string{"localhost", "xn--6kr1jv0vy6j0yhm72am01b"}
	if !slices.Equal(identity.Certificate.Leaf.DNSNames, wantDNSNames) {
		t.Fatalf("certificate DNS names = %q, want %q", identity.Certificate.Leaf.DNSNames, wantDNSNames)
	}
	if err := identity.Certificate.Leaf.VerifyHostname("xn--6kr1jv0vy6j0yhm72am01b"); err != nil {
		t.Fatalf("normalized host name does not match the certificate: %v", err)
	}
}

func TestUniqueDNSNamesNormalizesDeduplicatesAndFiltersInvalidValues(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	overlongLabel := strings.Repeat("a", 64) + ".local"
	got := uniqueDNSNames([]string{
		" LOCALHOST ",
		"春风又绿江南岸",
		"XN--6KR1JV0VY6J0YHM72AM01B",
		"Example.COM.",
		"bad host/name",
		"bad_host",
		"bad\x00host",
		"127.0.0.1",
		overlongLabel,
		invalidUTF8,
		"",
	})
	want := []string{"localhost", "xn--6kr1jv0vy6j0yhm72am01b", "example.com"}
	if !slices.Equal(got, want) {
		t.Fatalf("normalized DNS names = %q, want %q", got, want)
	}
}

func TestCertificateRotatesExpiredNotYetValidAndCorruptIdentities(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(t *testing.T, files CertificateFiles, createdAt time.Time) time.Time
	}{
		{
			name: "expired",
			mutate: func(_ *testing.T, _ CertificateFiles, createdAt time.Time) time.Time {
				return createdAt.Add(24 * time.Hour)
			},
		},
		{
			name: "near-expiry",
			mutate: func(_ *testing.T, _ CertificateFiles, createdAt time.Time) time.Time {
				return createdAt.Add(22 * time.Hour)
			},
		},
		{
			name: "not-yet-valid-after-clock-rollback",
			mutate: func(_ *testing.T, _ CertificateFiles, createdAt time.Time) time.Time {
				return createdAt.Add(-time.Hour)
			},
		},
		{
			name: "corrupt-certificate",
			mutate: func(t *testing.T, files CertificateFiles, createdAt time.Time) time.Time {
				t.Helper()
				if err := os.WriteFile(files.CertificatePath, []byte("not a certificate"), 0o644); err != nil {
					t.Fatal(err)
				}
				return createdAt.Add(time.Hour)
			},
		},
		{
			name: "missing-certificate-half",
			mutate: func(t *testing.T, files CertificateFiles, createdAt time.Time) time.Time {
				t.Helper()
				if err := os.Remove(files.CertificatePath); err != nil {
					t.Fatal(err)
				}
				return createdAt.Add(time.Hour)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			directory := t.TempDir()
			files := CertificateFiles{
				CertificatePath: filepath.Join(directory, "library-cert.pem"),
				PrivateKeyPath:  filepath.Join(directory, "library-key.pem"),
			}
			createdAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
			first, err := LoadOrCreateCertificate(files, CertificateOptions{
				Clock: func() time.Time { return createdAt }, Validity: 24 * time.Hour,
			})
			if err != nil {
				t.Fatal(err)
			}
			oldPublicKey := append([]byte(nil), first.Certificate.Leaf.RawSubjectPublicKeyInfo...)
			loadAt := testCase.mutate(t, files, createdAt)
			second, err := LoadOrCreateCertificate(files, CertificateOptions{
				Clock: func() time.Time { return loadAt }, Validity: 48 * time.Hour,
			})
			if err != nil {
				t.Fatalf("recover invalid identity: %v", err)
			}
			if !second.Persistent || !second.Rotated || second.PersistenceError != "" {
				t.Fatalf("rotated identity metadata = %#v", second)
			}
			if second.Fingerprint == first.Fingerprint {
				t.Fatal("identity rotation retained the stale certificate pin")
			}
			if string(second.Certificate.Leaf.RawSubjectPublicKeyInfo) == string(oldPublicKey) {
				t.Fatal("identity rotation unexpectedly reused the old public key")
			}
			if loadAt.Before(second.Certificate.Leaf.NotBefore) || !loadAt.Before(second.NotAfter) {
				t.Fatalf("rotated certificate is not valid at load time: now=%v leaf=%#v", loadAt, second.Certificate.Leaf)
			}
		})
	}
}

func TestCertificatePersistenceFailureFallsBackToEphemeralIdentity(t *testing.T) {
	directory := t.TempDir()
	files := CertificateFiles{
		// A directory cannot be replaced by the staged certificate file. This
		// models an unrecoverable on-disk path conflict without permission-based
		// assertions that vary across root/Windows test environments.
		CertificatePath: filepath.Join(directory, "certificate-is-a-directory"),
		PrivateKeyPath:  filepath.Join(directory, "library-key.pem"),
	}
	if err := os.Mkdir(files.CertificatePath, 0o700); err != nil {
		t.Fatal(err)
	}
	identity, err := LoadOrCreateCertificate(files, CertificateOptions{})
	if err != nil {
		t.Fatalf("certificate path damage blocked the application: %v", err)
	}
	if identity.Persistent || identity.PersistenceError == "" || !identity.Rotated ||
		identity.Certificate.Leaf == nil || identity.Fingerprint == "" {
		t.Fatalf("ephemeral fallback identity = %#v", identity)
	}
}

func TestServerProvidesLoopbackBackendAndPinnedLANTLS(t *testing.T) {
	directory := t.TempDir()
	identity, err := LoadOrCreateCertificate(CertificateFiles{
		CertificatePath: filepath.Join(directory, "cert.pem"),
		PrivateKeyPath:  filepath.Join(directory, "key.pem"),
	}, CertificateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/health" {
			http.NotFound(w, request)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server, err := New(Config{
		Handler: handler, BackendAddress: "127.0.0.1:0", LANAddress: "127.0.0.1:0", TLSIdentity: &identity,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Shutdown(context.Background()) }()
	if server.BackendAddress() == "" || server.LANAddress() == "" || server.TLSFingerprint() != identity.Fingerprint {
		t.Fatalf("unexpected listener metadata: backend=%q lan=%q fingerprint=%q", server.BackendAddress(), server.LANAddress(), server.TLSFingerprint())
	}

	backendResponse, err := http.Get("http://" + server.BackendAddress() + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = backendResponse.Body.Close()
	if backendResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected backend response: %d", backendResponse.StatusCode)
	}

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS13,
		// The mobile/Windows client pins the self-signed certificate fingerprint
		// distributed in the pairing QR code instead of using the Web PKI.
		InsecureSkipVerify: true, //nolint:gosec -- custom pin verification follows
		VerifyConnection: func(state tls.ConnectionState) error {
			actual := sha256.Sum256(state.PeerCertificates[0].Raw)
			if strings.ToUpper(hex.EncodeToString(actual[:])) != identity.Fingerprint {
				return errPinMismatch
			}
			return nil
		},
	}}}
	lanResponse, err := client.Get("https://" + server.LANAddress() + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = lanResponse.Body.Close()
	if lanResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected LAN TLS response: %d", lanResponse.StatusCode)
	}
}

func TestServerRejectsNonLoopbackTailscaleBackend(t *testing.T) {
	server, err := New(Config{Handler: http.NotFoundHandler(), BackendAddress: "0.0.0.0:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected non-loopback backend rejection, got %v", err)
	}
}

func TestServerRejectsWildcardLANListener(t *testing.T) {
	directory := t.TempDir()
	identity, err := LoadOrCreateCertificate(CertificateFiles{
		CertificatePath: filepath.Join(directory, "cert.pem"),
		PrivateKeyPath:  filepath.Join(directory, "key.pem"),
	}, CertificateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{
		Handler: http.NotFoundHandler(), LANAddress: "0.0.0.0:0", TLSIdentity: &identity,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "explicit unicast") {
		t.Fatalf("expected wildcard LAN rejection, got %v", err)
	}
}

type pinMismatchError struct{}

func (pinMismatchError) Error() string { return "Library TLS certificate fingerprint mismatch" }

var errPinMismatch pinMismatchError
