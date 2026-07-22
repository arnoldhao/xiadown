package proxy

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

type directIPResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

// resolveAndPinDirectAddresses resolves once before connect. A non-loopback
// authority that resolves to loopback is rejected before any TCP side effect;
// the selected IP literals are then pinned through the dial.
func (s *routeState) resolveAndPinDirectAddresses(ctx context.Context, network, address string) ([]string, bool, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return nil, false, errors.New("direct route requires a valid host and port")
	}
	if _, err := parsePort(port); err != nil {
		return nil, false, err
	}
	_, explicitlyLoopback := canonicalNetworkTarget(address, "")

	resolver := s.directDNS
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	var resolved []net.IPAddr
	ipHost, zone := host, ""
	if index := strings.LastIndex(ipHost, "%"); index >= 0 {
		ipHost, zone = ipHost[:index], ipHost[index+1:]
	}
	if literal := net.ParseIP(ipHost); literal != nil {
		resolved = []net.IPAddr{{IP: literal, Zone: zone}}
	} else {
		resolved, err = resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, explicitlyLoopback, err
		}
	}

	const directAddressLimit = 8
	pinned := make([]string, 0, min(len(resolved), directAddressLimit))
	seen := make(map[string]struct{}, len(resolved))
	for _, candidate := range resolved {
		if candidate.IP == nil || !networkAllowsIP(network, candidate.IP) {
			continue
		}
		if explicitlyLoopback != candidate.IP.IsLoopback() {
			if candidate.IP.IsLoopback() {
				return nil, explicitlyLoopback, errors.New("network policy rejected a hostname resolving to loopback")
			}
			return nil, explicitlyLoopback, errors.New("network policy rejected a loopback authority resolving outside loopback")
		}
		candidateHost := candidate.IP.String()
		if candidate.Zone != "" {
			candidateHost += "%" + candidate.Zone
		}
		pinnedAddress := net.JoinHostPort(candidateHost, port)
		if _, duplicate := seen[pinnedAddress]; duplicate {
			continue
		}
		seen[pinnedAddress] = struct{}{}
		pinned = append(pinned, pinnedAddress)
		if len(pinned) == directAddressLimit {
			break
		}
	}
	if len(pinned) == 0 {
		return nil, explicitlyLoopback, errors.New("direct destination has no address for requested network")
	}
	return pinned, explicitlyLoopback, nil
}

func dialPinnedAddresses(ctx context.Context, network string, addresses []string, timeout time.Duration) (net.Conn, error) {
	if len(addresses) == 0 {
		return nil, errors.New("direct destination has no pinned address")
	}
	dialContext, cancel := context.WithCancel(ctx)
	defer cancel()
	type dialResult struct {
		connection net.Conn
		err        error
	}
	results := make(chan dialResult, len(addresses))
	for index, address := range addresses {
		index, address := index, address
		go func() {
			if index > 0 {
				timer := time.NewTimer(time.Duration(index) * publicDialFallbackDelay)
				defer timer.Stop()
				select {
				case <-dialContext.Done():
					results <- dialResult{err: dialContext.Err()}
					return
				case <-timer.C:
				}
			}
			connection, err := (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext(dialContext, network, address)
			results <- dialResult{connection: connection, err: err}
		}()
	}

	var lastErr error
	for completed := 0; completed < len(addresses); completed++ {
		result := <-results
		if result.err != nil {
			lastErr = result.err
			continue
		}
		cancel()
		remaining := len(addresses) - completed - 1
		if remaining > 0 {
			go func() {
				for range remaining {
					late := <-results
					if late.connection != nil {
						_ = late.connection.Close()
					}
				}
			}()
		}
		return result.connection, nil
	}
	if lastErr == nil {
		lastErr = errors.New("direct destination connection failed")
	}
	return nil, lastErr
}
