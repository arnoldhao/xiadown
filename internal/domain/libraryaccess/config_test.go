package libraryaccess

import (
	"errors"
	"strings"
	"testing"
)

func TestNewConfigValidation(t *testing.T) {
	valid, err := DefaultConfig("Arnold's Mac")
	if err != nil {
		t.Fatalf("default config: %v", err)
	}
	if valid.RemoteEnabled || !valid.LANEnabled || valid.LANPort != DefaultLANPort ||
		valid.TailscaleHTTPSPort != 443 || valid.TailscalePath != "/xiadown" {
		t.Fatalf("unexpected defaults: %+v", valid)
	}
	legacyZero, err := NewConfig(ConfigParams{
		LANPort: 0, TailscaleHTTPSPort: 443, TailscalePath: "/xiadown", DeviceName: "Windows PC",
	})
	if err != nil || legacyZero.LANPort != DefaultLANPort {
		t.Fatalf("legacy zero LAN port = %+v, %v", legacyZero, err)
	}

	tests := []ConfigParams{
		{LANPort: -1, TailscaleHTTPSPort: 443, TailscalePath: "/xiadown", DeviceName: "Mac"},
		{LANPort: 65536, TailscaleHTTPSPort: 443, TailscalePath: "/xiadown", DeviceName: "Mac"},
		{TailscaleHTTPSPort: 0, TailscalePath: "/xiadown", DeviceName: "Mac"},
		{TailscaleHTTPSPort: 443, TailscalePath: "/", DeviceName: "Mac"},
		{TailscaleHTTPSPort: 443, TailscalePath: "/../other", DeviceName: "Mac"},
		{TailscaleHTTPSPort: 443, TailscalePath: "/xia down", DeviceName: "Mac"},
		{TailscaleHTTPSPort: 443, TailscalePath: "/xiadown?bad=1", DeviceName: "Mac"},
		{TailscaleHTTPSPort: 443, TailscalePath: "/xiadown", DeviceName: " "},
		{TailscaleHTTPSPort: 443, TailscalePath: "/xiadown", DeviceName: strings.Repeat("a", 64)},
		{TailscaleHTTPSPort: 443, TailscalePath: "/xiadown", DeviceName: strings.Repeat("库", 22)},
	}
	for index, params := range tests {
		if _, err := NewConfig(params); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("case %d error = %v, want ErrInvalidConfig", index, err)
		}
	}
	if _, err := NewConfig(ConfigParams{
		TailscaleHTTPSPort: 443, TailscalePath: "/xiadown", DeviceName: strings.Repeat("库", 21),
	}); err != nil {
		t.Fatalf("63-byte UTF-8 device name rejected: %v", err)
	}
}
