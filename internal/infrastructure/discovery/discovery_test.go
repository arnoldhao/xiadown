package discovery

import (
	"net"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestEligibleInterfaceIndicesExcludeVPNAndVirtualAdapters(t *testing.T) {
	interfaces := []net.Interface{
		{Index: 1, Name: "lo0", Flags: net.FlagUp | net.FlagLoopback | net.FlagMulticast},
		{Index: 4, Name: "en0", Flags: net.FlagUp | net.FlagBroadcast | net.FlagMulticast},
		{Index: 7, Name: "Ethernet", Flags: net.FlagUp | net.FlagBroadcast | net.FlagMulticast},
		{Index: 8, Name: "Tailscale", Flags: net.FlagUp | net.FlagMulticast},
		{Index: 9, Name: "utun4", Flags: net.FlagUp | net.FlagMulticast},
		{Index: 10, Name: "vEthernet (WSL)", Flags: net.FlagUp | net.FlagMulticast},
		{Index: 11, Name: "en9", Flags: net.FlagBroadcast | net.FlagMulticast},
	}
	if actual := EligibleInterfaceIndices(interfaces); !reflect.DeepEqual(actual, []int{4, 7}) {
		t.Fatalf("eligible interfaces = %v", actual)
	}
}

func TestAdvertisementContractHasNoSensitiveTXTData(t *testing.T) {
	validated, err := ValidateAdvertisement(Advertisement{Name: "XiaDown", Port: 443, InterfaceIndices: []int{7, 4, 7}})
	if err != nil {
		t.Fatalf("ValidateAdvertisement: %v", err)
	}
	if !reflect.DeepEqual(validated.InterfaceIndices, []int{4, 7}) {
		t.Fatalf("validated interface indices = %v", validated.InterfaceIndices)
	}
	expected := []string{"api=1", "tls=1", "pair=required"}
	if !reflect.DeepEqual(TXTRecords(), expected) {
		t.Fatalf("TXT records = %v", TXTRecords())
	}
}

func TestAdvertisementNameUsesUTF8ByteLimit(t *testing.T) {
	t.Parallel()
	if _, err := ValidateAdvertisement(Advertisement{Name: strings.Repeat("库", 21), Port: 43127}); err != nil {
		t.Fatalf("63-byte name rejected: %v", err)
	}
	for _, name := range []string{strings.Repeat("a", 64), strings.Repeat("库", 22), "bad\x00name"} {
		if _, err := ValidateAdvertisement(Advertisement{Name: name, Port: 43127}); err == nil {
			t.Fatalf("invalid service name %q was accepted", name)
		}
	}
}

func TestWindowsDNSServiceHostName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "physical host", input: "STUDIO-PC", want: "STUDIO-PC.local"},
		{name: "trailing root", input: "studio.local.", want: "studio.local"},
		{name: "existing suffix is case insensitive", input: "Studio.LOCAL", want: "Studio.LOCAL"},
		{name: "surrounding whitespace", input: "  studio  ", want: "studio.local"},
		{name: "maximum UTF-8 label", input: strings.Repeat("库", 21), want: strings.Repeat("库", 21) + ".local"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := windowsDNSServiceHostName(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("windowsDNSServiceHostName(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}

	for _, input := range []string{
		"",
		"   ",
		"studio\x00pc",
		"studio..local",
		strings.Repeat("x", 64),
		strings.Repeat("x", 250) + ".local",
		strings.Join([]string{
			strings.Repeat("x", 63),
			strings.Repeat("y", 63),
			strings.Repeat("z", 63),
			strings.Repeat("w", 63),
		}, "."),
	} {
		if _, err := windowsDNSServiceHostName(input); err == nil {
			t.Fatalf("windowsDNSServiceHostName(%q) unexpectedly succeeded", input)
		}
	}
}

func TestWindowsDNSSDNativeCallErrorSuppressesSuccessErrno(t *testing.T) {
	t.Parallel()

	for _, callErr := range []error{nil, syscall.Errno(0)} {
		err := windowsDNSSDNativeCallError("native constructor returned null", callErr)
		if got := err.Error(); got != "native constructor returned null" {
			t.Fatalf("unexpected error %q", got)
		}
	}

	err := windowsDNSSDNativeCallError("native call failed", syscall.Errno(5))
	if !strings.HasPrefix(err.Error(), "native call failed: ") {
		t.Fatalf("native errno was discarded: %q", err)
	}
}

func TestReleaseWindowsDNSSDCallbackInstance(t *testing.T) {
	t.Parallel()

	var released []uintptr
	release := func(instance uintptr) {
		released = append(released, instance)
	}

	releaseWindowsDNSSDCallbackInstance(0, release)
	releaseWindowsDNSSDCallbackInstance(0x1234, release)
	if !reflect.DeepEqual(released, []uintptr{0x1234}) {
		t.Fatalf("released callback instances = %#v", released)
	}
}

func TestEligibleLANEndpointsArePrivateInterfaceBoundAndIPv6Capable(t *testing.T) {
	physical := net.Interface{Index: 4, Name: "Ethernet", Flags: net.FlagUp | net.FlagMulticast}
	vpn := net.Interface{Index: 8, Name: "Tailscale", Flags: net.FlagUp | net.FlagMulticast}
	endpoints := EligibleLANEndpoints([]InterfaceCandidate{
		{Interface: physical, Addresses: []net.Addr{
			&net.IPNet{IP: net.ParseIP("203.0.113.7"), Mask: net.CIDRMask(24, 32)},
			&net.IPNet{IP: net.ParseIP("fd00::7"), Mask: net.CIDRMask(64, 128)},
			&net.IPNet{IP: net.ParseIP("192.168.1.7"), Mask: net.CIDRMask(24, 32)},
			&net.IPAddr{IP: net.ParseIP("fe80::7")},
		}},
		{Interface: vpn, Addresses: []net.Addr{
			&net.IPNet{IP: net.ParseIP("100.64.0.7"), Mask: net.CIDRMask(32, 32)},
		}},
	})
	if len(endpoints) != 3 {
		t.Fatalf("eligible endpoints = %#v", endpoints)
	}
	if got := endpoints[0].ListenAddress(43127); got != "192.168.1.7:43127" {
		t.Fatalf("preferred endpoint = %q", got)
	}
	if got := endpoints[1].ListenAddress(43127); got != "[fd00::7]:43127" {
		t.Fatalf("ULA endpoint = %q", got)
	}
	if got := endpoints[2].ListenAddress(43127); got != "[fe80::7%Ethernet]:43127" {
		t.Fatalf("link-local endpoint = %q", got)
	}
}
