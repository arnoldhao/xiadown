// Package discovery advertises the isolated XiaDown Library API over the
// cross-platform DNS-SD/mDNS standard. Platform implementations deliberately
// use the operating system DNS-SD stack; Windows does not depend on Bonjour.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
)

const ServiceType = "_xiadown._tcp"

var ErrUnavailable = errors.New("dns-sd unavailable")

type Advertisement struct {
	Name             string
	Port             int
	InterfaceIndices []int
}

// LANEndpoint is an address owned by an eligible physical LAN interface. The
// listener uses the same interface index that DNS-SD advertises, preventing a
// wildcard socket from unintentionally exposing the API on VPN, container, or
// public interfaces.
type LANEndpoint struct {
	InterfaceIndex int
	InterfaceName  string
	IP             net.IP
	Zone           string
}

type InterfaceCandidate struct {
	Interface net.Interface
	Addresses []net.Addr
}

type Registration interface {
	Close() error
}

type Advertiser interface {
	Register(context.Context, Advertisement) (Registration, error)
}

func NewAdvertiser() Advertiser { return newPlatformAdvertiser() }

func ValidateAdvertisement(value Advertisement) (Advertisement, error) {
	value.Name = strings.TrimSpace(value.Name)
	if value.Name == "" || !utf8.ValidString(value.Name) || len([]byte(value.Name)) > 63 ||
		strings.ContainsRune(value.Name, '\x00') || value.Port < 1 || value.Port > 65535 {
		return Advertisement{}, fmt.Errorf("invalid DNS-SD advertisement")
	}
	seen := make(map[int]struct{}, len(value.InterfaceIndices))
	indices := make([]int, 0, len(value.InterfaceIndices))
	for _, index := range value.InterfaceIndices {
		if index <= 0 {
			return Advertisement{}, fmt.Errorf("invalid DNS-SD interface index")
		}
		if _, exists := seen[index]; exists {
			continue
		}
		seen[index] = struct{}{}
		indices = append(indices, index)
	}
	sort.Ints(indices)
	value.InterfaceIndices = indices
	return value, nil
}

func TXTRecords() []string { return []string{"api=1", "tls=1", "pair=required"} }

// windowsDNSServiceHostName turns the physical Windows DNS hostname into the
// fully-qualified multicast hostname required by DnsServiceConstructInstance.
// It stays platform-neutral so the native boundary can be unit-tested on every
// development host.
func windowsDNSServiceHostName(value string) (string, error) {
	hostName := strings.TrimSuffix(strings.TrimSpace(value), ".")
	if hostName == "" || !utf8.ValidString(hostName) || strings.ContainsRune(hostName, '\x00') {
		return "", fmt.Errorf("invalid Windows DNS-SD host name")
	}
	if !strings.HasSuffix(strings.ToLower(hostName), ".local") {
		hostName += ".local"
	}
	if len([]byte(hostName)) > 253 {
		return "", fmt.Errorf("invalid Windows DNS-SD host name")
	}
	for _, label := range strings.Split(hostName, ".") {
		if label == "" || len([]byte(label)) > 63 {
			return "", fmt.Errorf("invalid Windows DNS-SD host name")
		}
	}
	return hostName, nil
}

// windowsDNSSDNativeCallError filters ERROR_SUCCESS from LazyProc.Call. The
// Windows DNS-SD status APIs return their primary result directly, so the
// accompanying syscall.Errno(0) is not a real error and must never surface as
// "The operation completed successfully."
func windowsDNSSDNativeCallError(message string, callErr error) error {
	if callErr == nil {
		return errors.New(message)
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == 0 {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %w", message, callErr)
}

// releaseWindowsDNSSDCallbackInstance applies the ownership contract of
// DNS_SERVICE_REGISTER_COMPLETE: every non-null pInstance passed to the
// callback is a system-allocated result that the caller must release. Keeping
// this decision separate from the syscall makes zero/non-zero ownership
// behavior testable on non-Windows development hosts.
func releaseWindowsDNSSDCallbackInstance(instance uintptr, release func(uintptr)) {
	if instance != 0 {
		release(instance)
	}
}

var excludedInterfaceName = regexp.MustCompile(`(?i)(^lo\d*$|loopback|tailscale|utun|(^|[-_.])(tun|tap|vpn)([-_.]|$)|docker|podman|veth|vmnet|virbr|hyper-v|vethernet|wsl)`)

// EligibleInterfaceIndices returns only active multicast-capable physical LAN
// interfaces. Tailscale and other virtual/VPN interfaces are excluded because
// Tailscale Serve has its own discovery and access boundary.
func EligibleInterfaceIndices(interfaces []net.Interface) []int {
	indices := make([]int, 0, len(interfaces))
	for _, iface := range interfaces {
		if !eligibleInterface(iface) {
			continue
		}
		indices = append(indices, iface.Index)
	}
	sort.Ints(indices)
	return indices
}

func eligibleInterface(iface net.Interface) bool {
	return iface.Index > 0 && iface.Flags&net.FlagUp != 0 &&
		iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagMulticast != 0 &&
		!excludedInterfaceName.MatchString(iface.Name)
}

// EligibleLANEndpoints accepts only private or link-local unicast addresses.
// A globally-routable address on a physical NIC is not a LAN-only boundary and
// is therefore never selected for the Local mode listener.
func EligibleLANEndpoints(candidates []InterfaceCandidate) []LANEndpoint {
	endpoints := make([]LANEndpoint, 0)
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		iface := candidate.Interface
		if !eligibleInterface(iface) {
			continue
		}
		for _, address := range candidate.Addresses {
			ip, zone := interfaceAddressIP(address)
			if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() ||
				(!ip.IsPrivate() && !ip.IsLinkLocalUnicast()) {
				continue
			}
			if ip.To4() != nil {
				ip = append(net.IP(nil), ip.To4()...)
				zone = ""
			} else {
				ip = append(net.IP(nil), ip.To16()...)
				if ip.IsLinkLocalUnicast() && strings.TrimSpace(zone) == "" {
					zone = iface.Name
				}
			}
			key := fmt.Sprintf("%d|%s|%s", iface.Index, ip.String(), zone)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			endpoints = append(endpoints, LANEndpoint{
				InterfaceIndex: iface.Index, InterfaceName: iface.Name, IP: ip, Zone: zone,
			})
		}
	}
	sort.Slice(endpoints, func(left, right int) bool {
		leftPriority := lanEndpointPriority(endpoints[left])
		rightPriority := lanEndpointPriority(endpoints[right])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if endpoints[left].InterfaceIndex != endpoints[right].InterfaceIndex {
			return endpoints[left].InterfaceIndex < endpoints[right].InterfaceIndex
		}
		return endpoints[left].IP.String() < endpoints[right].IP.String()
	})
	return endpoints
}

func SystemEligibleLANEndpoints() ([]LANEndpoint, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	candidates := make([]InterfaceCandidate, 0, len(interfaces))
	for _, iface := range interfaces {
		addresses, addressErr := iface.Addrs()
		if addressErr != nil {
			continue
		}
		candidates = append(candidates, InterfaceCandidate{Interface: iface, Addresses: addresses})
	}
	endpoints := EligibleLANEndpoints(candidates)
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("%w: no eligible private LAN address", ErrUnavailable)
	}
	return endpoints, nil
}

func (endpoint LANEndpoint) ListenAddress(port int) string {
	host := endpoint.IP.String()
	if endpoint.IP.To4() == nil && strings.TrimSpace(endpoint.Zone) != "" {
		host += "%" + strings.TrimSpace(endpoint.Zone)
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func interfaceAddressIP(address net.Addr) (net.IP, string) {
	switch value := address.(type) {
	case *net.IPNet:
		return value.IP, ""
	case *net.IPAddr:
		return value.IP, value.Zone
	default:
		if value == nil {
			return nil, ""
		}
		host, _, err := net.SplitHostPort(value.String())
		if err != nil {
			host = value.String()
		}
		zone := ""
		if index := strings.LastIndex(host, "%"); index >= 0 {
			zone = host[index+1:]
			host = host[:index]
		}
		return net.ParseIP(strings.Trim(host, "[]")), zone
	}
}

func lanEndpointPriority(endpoint LANEndpoint) int {
	if endpoint.IP.To4() != nil {
		if endpoint.IP.IsPrivate() {
			return 0
		}
		return 2
	}
	if endpoint.IP.IsPrivate() {
		return 1
	}
	return 3
}

func advertisementInterfaceIndices(value Advertisement) ([]int, error) {
	if len(value.InterfaceIndices) > 0 {
		return append([]int(nil), value.InterfaceIndices...), nil
	}
	return systemEligibleInterfaceIndices()
}

func systemEligibleInterfaceIndices() ([]int, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	indices := EligibleInterfaceIndices(interfaces)
	if len(indices) == 0 {
		return nil, fmt.Errorf("%w: no eligible LAN interface", ErrUnavailable)
	}
	return indices, nil
}

type registrationGroup struct {
	closeFuncs []func() error
	once       sync.Once
	err        error
}

func (group *registrationGroup) Close() error {
	group.once.Do(func() {
		for index := len(group.closeFuncs) - 1; index >= 0; index-- {
			group.err = errors.Join(group.err, group.closeFuncs[index]())
		}
		group.closeFuncs = nil
	})
	return group.err
}
