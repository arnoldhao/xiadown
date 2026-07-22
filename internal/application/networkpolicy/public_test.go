package networkpolicy

import (
	"context"
	"errors"
	"net"
	"testing"
)

type staticResolver map[string][]net.IPAddr

func (resolver staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	return resolver[host], nil
}

func TestIsPublicIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		address string
		want    bool
	}{
		{"8.8.8.8", true},
		{"2606:4700:4700::1111", true},
		{"127.0.0.1", false},
		{"10.0.0.1", false},
		{"169.254.169.254", false},
		{"0.0.0.0", false},
		{"224.0.0.1", false},
		{"100.100.100.200", false},
		{"192.0.0.8", false},
		{"192.88.99.1", false},
		{"198.18.0.1", false},
		{"::1", false},
		{"fe80::1", false},
		{"fc00::1", false},
		{"ff02::1", false},
		{"2001:db8::1", false},
		{"64:ff9b::a9fe:a9fe", false},
		{"64:ff9b:1::a9fe:a9fe", false},
		{"100::1", false},
		{"100:0:0:1::1", false},
		{"2001::1", false},
		{"2001:20::1", false},
		{"2002:a9fe:a9fe::1", false},
		{"2620:4f:8000::1", false},
		{"3fff::1", false},
		{"5f00::1", false},
		{"::ffff:169.254.169.254", false},
		{"::2", false},
		{"fec0::1", false},
		{"4000::1", false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.address, func(t *testing.T) {
			t.Parallel()
			if got := IsPublicIP(net.ParseIP(test.address)); got != test.want {
				t.Fatalf("IsPublicIP(%q) = %v, want %v", test.address, got, test.want)
			}
		})
	}
}

func TestResolvePublicIPsRejectsIPv4TranslationAndTransitionAnswers(t *testing.T) {
	t.Parallel()
	for _, address := range []string{
		"64:ff9b::a9fe:a9fe",
		"64:ff9b:1::a9fe:a9fe",
		"2001:0000:4136:e378:8000:63bf:3fff:fdd2",
		"2002:a9fe:a9fe::1",
	} {
		_, err := ResolvePublicIPs(context.Background(), staticResolver{
			"media.example": {{IP: net.ParseIP(address)}},
		}, "media.example")
		if !errors.Is(err, ErrDestinationBlocked) {
			t.Fatalf("expected %s to be blocked, got %v", address, err)
		}
	}
}

func TestResolvePublicIPsRejectsMixedDNSAnswers(t *testing.T) {
	t.Parallel()
	_, err := ResolvePublicIPs(context.Background(), staticResolver{
		"media.example": {{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("127.0.0.1")}},
	}, "media.example")
	if !errors.Is(err, ErrDestinationBlocked) {
		t.Fatalf("expected blocked destination, got %v", err)
	}
}

func TestValidatePublicHTTPURLRejectsMetadataAndCredentials(t *testing.T) {
	t.Parallel()
	for _, rawURL := range []string{
		"http://169.254.169.254/latest/meta-data",
		"https://metadata.google.internal/computeMetadata/v1",
		"https://user:password@example.com/video",
	} {
		if _, err := ValidatePublicHTTPURL(rawURL); err == nil {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
}
