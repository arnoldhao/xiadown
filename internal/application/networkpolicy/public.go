package networkpolicy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// ErrDestinationBlocked identifies a destination which must not be reached by
// requests submitted through a remote/public API. Callers should not retry it
// through another resolver or transport because that would re-open DNS
// rebinding and redirect based SSRF paths.
var ErrDestinationBlocked = errors.New("network destination is blocked")

// Resolver is the subset of net.Resolver used by the public-network policy.
// Keeping it small also lets the fetch-boundary tests provide deterministic
// DNS answers.
type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

// ValidatePublicHTTPURL performs the syntax and literal-host portion of the
// policy. Hostnames are resolved again immediately before every connection by
// ResolvePublicIPs; validation here alone is intentionally not a security
// boundary.
func ValidatePublicHTTPURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid public URL: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if (scheme != "http" && scheme != "https") || parsed.User != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return nil, fmt.Errorf("%w: URL must be an http(s) URL without credentials", ErrDestinationBlocked)
	}
	if err := ValidatePublicHostname(parsed.Hostname()); err != nil {
		return nil, err
	}
	return parsed, nil
}

// ValidatePublicHostname blocks special-use names and unsafe IP literals. DNS
// names which pass this check still require ResolvePublicIPs at dial time.
func ValidatePublicHostname(hostname string) error {
	host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(hostname), "."))
	if host == "" || strings.Contains(host, "%") {
		return fmt.Errorf("%w: invalid hostname", ErrDestinationBlocked)
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if !IsPublicIP(ip) {
			return fmt.Errorf("%w: non-public IP address", ErrDestinationBlocked)
		}
		return nil
	}
	for _, suffix := range []string{"localhost", "local", "internal", "home.arpa", "localdomain"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return fmt.Errorf("%w: special-use hostname", ErrDestinationBlocked)
		}
	}
	switch host {
	case "metadata", "instance-data", "metadata.google.internal", "kubernetes.default.svc":
		return fmt.Errorf("%w: metadata hostname", ErrDestinationBlocked)
	}
	return nil
}

// ResolvePublicIPs resolves a hostname at the last responsible moment and
// rejects the complete answer set if any record is non-public. Rejecting mixed
// answers prevents an attacker from racing a safe address against a private
// one in clients which retry multiple addresses.
func ResolvePublicIPs(ctx context.Context, resolver Resolver, hostname string) ([]net.IPAddr, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if err := ValidatePublicHostname(hostname); err != nil {
		return nil, err
	}
	if ip := net.ParseIP(strings.Trim(strings.TrimSpace(hostname), "[]")); ip != nil {
		return []net.IPAddr{{IP: ip}}, nil
	}
	addresses, err := resolver.LookupIPAddr(ctx, strings.TrimSuffix(strings.TrimSpace(hostname), "."))
	if err != nil {
		return nil, fmt.Errorf("resolve public destination: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve public destination: no addresses")
	}
	for _, address := range addresses {
		if strings.TrimSpace(address.Zone) != "" || !IsPublicIP(address.IP) {
			return nil, fmt.Errorf("%w: DNS returned a non-public IP address", ErrDestinationBlocked)
		}
	}
	return addresses, nil
}

// IsPublicIP is deliberately stricter than net.IP.IsGlobalUnicast. The latter
// includes shared carrier space and documentation/benchmark networks which
// should never be useful download destinations and are commonly routed inside
// private environments.
func IsPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if !ip.IsGlobalUnicast() || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return !blockedIPv4(ipv4)
	}
	return !blockedIPv6(ip)
}

func blockedIPv4(ip net.IP) bool {
	// Shared carrier-grade NAT, IETF protocol assignments, documentation,
	// benchmarking and the reserved high address range.
	return (ip[0] == 100 && ip[1]&0xc0 == 64) ||
		(ip[0] == 192 && ip[1] == 0 && ip[2] == 0) ||
		(ip[0] == 192 && ip[1] == 0 && ip[2] == 2) ||
		(ip[0] == 192 && ip[1] == 88 && ip[2] == 99) ||
		(ip[0] == 198 && (ip[1] == 18 || ip[1] == 19)) ||
		(ip[0] == 198 && ip[1] == 51 && ip[2] == 100) ||
		(ip[0] == 203 && ip[1] == 0 && ip[2] == 113) ||
		ip[0] >= 240
}

// blockedIPv6Prefixes is a fail-closed snapshot of the IANA IPv6
// Special-Purpose Address Registry. Even entries marked globally reachable
// are intentionally blocked: translation/transition prefixes can embed an
// otherwise forbidden IPv4 destination, while protocol anycast and ORCHID
// addresses are not valid public download origins.
var blockedIPv6Prefixes = []netip.Prefix{
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("::ffff:0:0/96"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("100:0:0:1::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("2620:4f:8000::/48"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
}

var allocatedPublicIPv6 = netip.MustParsePrefix("2000::/3")

func blockedIPv6(ip net.IP) bool {
	ip = ip.To16()
	if ip == nil {
		return true
	}
	var raw [16]byte
	copy(raw[:], ip)
	address := netip.AddrFrom16(raw)
	// As of the IANA IPv6 unicast allocation registry, publicly routable
	// unicast space is allocated from 2000::/3. Requiring that allocation makes
	// future/reserved space fail closed (including deprecated site-local and
	// IPv4-compatible forms) instead of relying on net.IP.IsGlobalUnicast.
	if !allocatedPublicIPv6.Contains(address) {
		return true
	}
	for _, prefix := range blockedIPv6Prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
