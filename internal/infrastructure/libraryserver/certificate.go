package libraryserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

type CertificateFiles struct {
	CertificatePath string
	PrivateKeyPath  string
}

type CertificateOptions struct {
	DNSNames    []string
	IPAddresses []net.IP
	Clock       func() time.Time
	Random      io.Reader
	Validity    time.Duration
}

type TLSIdentity struct {
	Certificate      tls.Certificate
	Fingerprint      string
	NotAfter         time.Time
	Persistent       bool
	Rotated          bool
	PersistenceError string
}

var certificateDNSNameProfile = idna.New(
	idna.MapForLookup(),
	idna.BidiRule(),
	idna.VerifyDNSLength(true),
)

func LoadOrCreateCertificate(files CertificateFiles, options CertificateOptions) (TLSIdentity, error) {
	files.CertificatePath = strings.TrimSpace(files.CertificatePath)
	files.PrivateKeyPath = strings.TrimSpace(files.PrivateKeyPath)
	if files.CertificatePath == "" || files.PrivateKeyPath == "" || files.CertificatePath == files.PrivateKeyPath {
		return TLSIdentity{}, errors.New("Library TLS certificate and key paths are required and must differ")
	}
	clock := options.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	now := clock().UTC()
	certificatePEM, certificateErr := os.ReadFile(files.CertificatePath)
	privateKeyPEM, keyErr := os.ReadFile(files.PrivateKeyPath)
	hadPersistedArtifact := !errors.Is(certificateErr, os.ErrNotExist) || !errors.Is(keyErr, os.ErrNotExist)
	if certificateErr == nil && keyErr == nil {
		// Repair permissions before accepting a persisted private key. If either
		// the ACL repair or identity validation fails, rotate below instead of
		// making an unrelated desktop startup fail permanently.
		if permissionErr := restrictTLSPrivateKey(files.PrivateKeyPath); permissionErr == nil {
			if identity, parseErr := parseIdentity(certificatePEM, privateKeyPEM, now); parseErr == nil {
				if !certificateNeedsRenewal(identity.Certificate.Leaf, now) {
					identity.Persistent = true
					return identity, nil
				}
			}
		}
	}

	randomSource := options.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	validity := options.Validity
	if validity == 0 {
		validity = 5 * 365 * 24 * time.Hour
	}
	if validity < 24*time.Hour {
		return TLSIdentity{}, errors.New("Library TLS certificate validity must be at least one day")
	}
	generatedCertificatePEM, generatedPrivateKeyPEM, err := generateCertificate(options, now, validity, randomSource)
	if err != nil {
		return TLSIdentity{}, err
	}
	identity, err := parseIdentity(generatedCertificatePEM, generatedPrivateKeyPEM, now)
	if err != nil {
		return TLSIdentity{}, fmt.Errorf("verify generated Library TLS identity: %w", err)
	}
	identity.Rotated = hadPersistedArtifact
	if err := persistIdentityFiles(files, generatedCertificatePEM, generatedPrivateKeyPEM); err != nil {
		// An in-memory identity is still cryptographically sound and lets the
		// loopback API and the rest of the app start. LAN clients receive this
		// session's fingerprint through pairing; PersistenceError makes the
		// pin churn explicit to the bootstrap log instead of hiding it.
		identity.PersistenceError = err.Error()
		return identity, nil
	}
	identity.Persistent = true
	return identity, nil
}

func certificateNeedsRenewal(leaf *x509.Certificate, now time.Time) bool {
	if leaf == nil {
		return true
	}
	// Renew before a certificate can expire during a normal long-running app
	// session. Ten percent of its lifetime keeps short-lived test/development
	// identities useful; production's five-year identity is capped at 30 days.
	renewalWindow := leaf.NotAfter.Sub(leaf.NotBefore) / 10
	if maximum := 30 * 24 * time.Hour; renewalWindow > maximum {
		renewalWindow = maximum
	}
	return !now.UTC().Add(renewalWindow).Before(leaf.NotAfter)
}

func persistIdentityFiles(files CertificateFiles, certificatePEM, privateKeyPEM []byte) error {
	// Each destination is replaced from a fully written, fsynced temporary file.
	// A process interruption between the two renames can leave a mismatched pair,
	// but LoadOrCreateCertificate detects that state and safely rotates it on the
	// next launch instead of serving either half.
	if err := writePrivateFile(files.PrivateKeyPath, privateKeyPEM, 0o600); err != nil {
		return fmt.Errorf("persist Library TLS private key: %w", err)
	}
	if err := writePrivateFile(files.CertificatePath, certificatePEM, 0o644); err != nil {
		return fmt.Errorf("persist Library TLS certificate: %w", err)
	}
	return nil
}

func generateCertificate(options CertificateOptions, now time.Time, validity time.Duration, randomSource io.Reader) ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), randomSource)
	if err != nil {
		return nil, nil, fmt.Errorf("generate Library TLS private key: %w", err)
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(randomSource, serialLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("generate Library TLS serial: %w", err)
	}
	if serial.Sign() == 0 {
		serial.SetInt64(1)
	}
	dnsNames := uniqueDNSNames(append([]string{"localhost"}, options.DNSNames...))
	ipAddresses := uniqueIPAddresses(append([]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}, options.IPAddresses...))
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "XiaDown Library"},
		NotBefore:    now.Add(-5 * time.Minute), NotAfter: now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames, IPAddresses: ipAddresses,
	}
	der, err := x509.CreateCertificate(randomSource, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create Library TLS certificate: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("encode Library TLS private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), nil
}

func parseIdentity(certificatePEM, privateKeyPEM []byte, now time.Time) (TLSIdentity, error) {
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return TLSIdentity{}, err
	}
	if len(certificate.Certificate) != 1 {
		return TLSIdentity{}, errors.New("Library TLS identity must contain exactly one leaf certificate")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return TLSIdentity{}, err
	}
	if now.UTC().Before(leaf.NotBefore) || !now.UTC().Before(leaf.NotAfter) {
		return TLSIdentity{}, errors.New("Library TLS certificate is not currently valid")
	}
	certificate.Leaf = leaf
	fingerprint := sha256.Sum256(leaf.Raw)
	return TLSIdentity{
		Certificate: certificate,
		Fingerprint: strings.ToUpper(hex.EncodeToString(fingerprint[:])),
		NotAfter:    leaf.NotAfter.UTC(),
	}, nil
}

func writePrivateFile(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".xiadown-tls-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := restrictTLSFile(temporaryPath, mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	if mode.Perm() == 0o600 {
		return restrictTLSPrivateKey(path)
	}
	return nil
}

func uniqueDNSNames(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value, ok := normalizeCertificateDNSName(value)
		if !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeCertificateDNSName(value string) (string, bool) {
	if !utf8.ValidString(value) {
		return "", false
	}
	value = strings.TrimSuffix(strings.TrimSpace(value), ".")
	if value == "" {
		return "", false
	}
	value, err := certificateDNSNameProfile.ToASCII(value)
	if err != nil {
		return "", false
	}
	value = strings.ToLower(strings.TrimSuffix(value, "."))
	if value == "" || net.ParseIP(value) != nil {
		return "", false
	}
	return value, true
}

func uniqueIPAddresses(values []net.IP) []net.IP {
	result := make([]net.IP, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		canonical := value.String()
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, append(net.IP(nil), value...))
	}
	return result
}
