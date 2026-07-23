package proxy

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"xiadown/internal/application/networkpolicy"
)

const (
	publicDialFallbackDelay = 250 * time.Millisecond
	publicDialAddressLimit  = 8
)

func resolveAndPinPublicAddresses(ctx context.Context, network, address string) ([]string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return nil, errors.New("public route requires a valid host and port")
	}
	if _, err := parsePort(port); err != nil {
		return nil, err
	}

	addresses, err := networkpolicy.ResolvePublicIPs(ctx, net.DefaultResolver, host)
	if err != nil {
		return nil, err
	}

	pinned := make([]string, 0, min(len(addresses), publicDialAddressLimit))
	for _, candidate := range addresses {
		if networkAllowsIP(network, candidate.IP) && len(pinned) < publicDialAddressLimit {
			pinned = append(pinned, net.JoinHostPort(candidate.IP.String(), port))
		}
	}
	if len(pinned) == 0 {
		return nil, errors.New("public destination has no address for requested network")
	}
	return pinned, nil
}

func isPublicDestinationIP(ip net.IP) bool {
	return networkpolicy.IsPublicIP(ip)
}

func networkAllowsIP(network string, ip net.IP) bool {
	switch network {
	case "tcp4":
		return ip.To4() != nil
	case "tcp6":
		return ip.To4() == nil
	default:
		return true
	}
}
