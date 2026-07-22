package wails

import (
	"bytes"
	"context"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/access"
	"xiadown/internal/domain/library"
)

type pairingGrantRepositoryStub struct{}

func (pairingGrantRepositoryStub) ListByCatalogID(context.Context, string) ([]library.DeviceGrant, error) {
	return nil, nil
}
func (pairingGrantRepositoryStub) Get(context.Context, string) (library.DeviceGrant, error) {
	return library.DeviceGrant{}, nil
}
func (pairingGrantRepositoryStub) Save(context.Context, library.DeviceGrant) error { return nil }
func (pairingGrantRepositoryStub) Delete(context.Context, string) error            { return nil }
func (pairingGrantRepositoryStub) SaveDeviceGrantMutation(
	context.Context, library.DeviceGrant, int64, library.CatalogChangeKind, string,
) error {
	return nil
}
func (pairingGrantRepositoryStub) RecordDeviceGrantLastSeen(context.Context, string, string, time.Time) error {
	return nil
}

func TestStartLibraryPairingReturnsVersionedDirectoryTransportLink(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	service, err := access.NewService(pairingGrantRepositoryStub{}, "catalog-1", access.Options{
		Clock: func() time.Time { return now }, Random: bytes.NewReader(make([]byte, 64)),
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewLibraryPairingHandler(
		service,
		func() string { return " AABB " },
		func() string { return "192.168.1.20:43127" },
		func(context.Context) string { return "https://studio.example.ts.net/xiadown" },
		func() []string { return []string{"[fd00::20]:43127"} },
	)
	result, err := handler.StartLibraryPairing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.PairingVersion != libraryPairingProtocolVersion || result.PairingLink == "" ||
		result.TLSFingerprint != "AABB" || result.TailscaleURL != "https://studio.example.ts.net/xiadown/" ||
		!reflect.DeepEqual(result.LANEndpoints, []string{
			"https://[fd00::20]:43127/", "https://192.168.1.20:43127/",
		}) {
		t.Fatalf("unexpected pairing result: %#v", result)
	}
	parsed, err := url.Parse(result.PairingLink)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("nonce") != result.Nonce || query.Get("code") != result.Code ||
		query.Get("expires") != result.ExpiresAt || query.Get("fingerprint") != result.TLSFingerprint ||
		!reflect.DeepEqual(query["lan"], result.LANEndpoints) ||
		!reflect.DeepEqual(query["remote"], []string{result.TailscaleURL}) {
		t.Fatalf("pairing link and result diverged: result=%#v query=%#v", result, query)
	}
}

func TestLibraryPairingDeepLinkRoundTripsReservedValuesAndRepeatedEndpoints(t *testing.T) {
	result := LibraryPairingResult{
		PairingVersion: 1,
		Nonce:          "nonce +/?&=#%",
		Code:           "012345",
		ExpiresAt:      "2026-07-13T18:30:00.000+08:00",
		TLSFingerprint: "AA:BB CC",
		LANEndpoints: []string{
			"https://192.168.1.20:43127",
			"https://[fd00::20]:43127",
		},
		TailscaleURL: "https://studio.example.ts.net/xiadown",
	}
	link := libraryPairingDeepLink(result)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse pairing link: %v", err)
	}
	if parsed.Scheme != "xiadown" || parsed.Host != "pair" || parsed.Path != "" {
		t.Fatalf("unexpected pairing link target: %s", link)
	}
	query := parsed.Query()
	for key, want := range map[string]string{
		"v": "1", "nonce": result.Nonce, "code": result.Code,
		"expires": result.ExpiresAt, "fingerprint": result.TLSFingerprint,
	} {
		if got := query.Get(key); got != want {
			t.Fatalf("%s = %q, want %q (link %s)", key, got, want, link)
		}
	}
	wantLAN := []string{"https://192.168.1.20:43127/", "https://[fd00::20]:43127/"}
	wantRemote := []string{"https://studio.example.ts.net/xiadown/"}
	if !reflect.DeepEqual(query["lan"], wantLAN) || !reflect.DeepEqual(query["remote"], wantRemote) {
		t.Fatalf("endpoint query values were not preserved: %#v", query)
	}
	if strings.Contains(link, result.Nonce) || strings.Contains(link, result.TailscaleURL) {
		t.Fatalf("reserved query values were not percent encoded: %s", link)
	}
}

func TestLibraryPairingTransportBaseRejectsNonHTTPSOrAmbiguousURLs(t *testing.T) {
	for _, value := range []string{
		"http://192.168.1.20:43127", "https://user@example.test/xiadown",
		"https://example.test/xiadown?token=secret", "https://example.test/xiadown#fragment",
	} {
		if got := libraryPairingTransportBase(value); got != "" {
			t.Fatalf("unsafe pairing endpoint %q normalized to %q", value, got)
		}
	}
}

func TestCrossDeviceLANEndpointsOmitsServerLocalIPv6Zone(t *testing.T) {
	endpoints, legacy := crossDeviceLANEndpoints([]string{
		"192.168.1.20:8443",
		"[fd00::20]:8443",
		"[fe80::20%en0]:8443",
		"192.168.1.20:8443",
	})
	want := []string{"https://192.168.1.20:8443", "https://[fd00::20]:8443"}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("endpoints = %#v, want %#v", endpoints, want)
	}
	if legacy != "192.168.1.20:8443" {
		t.Fatalf("legacy address = %q", legacy)
	}
}

func TestCrossDeviceLANEndpointsLeavesLinkLocalScopeToDNSSD(t *testing.T) {
	endpoints, legacy := crossDeviceLANEndpoints([]string{"[fe80::20%Ethernet]:8443"})
	if len(endpoints) != 0 || legacy != "" {
		t.Fatalf("server-local link scope escaped pairing response: %#v %q", endpoints, legacy)
	}
}
