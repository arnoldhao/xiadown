package libraryserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"xiadown/internal/application/libraryaccess"
	"xiadown/internal/infrastructure/discovery"
	"xiadown/internal/infrastructure/firewall"
)

type LANRuntimeConfig struct {
	Server     *Server
	Identity   *TLSIdentity
	Advertiser discovery.Advertiser
	Firewall   firewall.Manager
	Program    string
	// LANEndpoints is injectable for deterministic tests. Production uses the
	// current eligible physical-interface inventory on every Apply.
	LANEndpoints func() ([]discovery.LANEndpoint, error)
}

// LANRuntime is the adapter between desired Library Access state and the
// isolated TLS listener, OS DNS-SD registration, and Windows Private-profile
// firewall rule.
type LANRuntime struct {
	config LANRuntimeConfig

	mu           sync.Mutex
	registration discovery.Registration
	cancel       context.CancelFunc
	deviceName   string
	stablePort   int
	firewallPort int
	firewallErr  error
	info         libraryaccess.LANInfo
}

func NewLANRuntime(config LANRuntimeConfig) (*LANRuntime, error) {
	if config.Server == nil || config.Identity == nil || config.Advertiser == nil {
		return nil, errors.New("Library LAN runtime requires server, TLS identity, and DNS-SD advertiser")
	}
	return &LANRuntime{config: config, info: libraryaccess.LANInfo{State: libraryaccess.StateDisabled}}, nil
}

func (runtime *LANRuntime) Inspect(context.Context) libraryaccess.LANInfo {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.info
}

func (runtime *LANRuntime) Enable(ctx context.Context, requestedPort int, deviceName string) (libraryaccess.LANInfo, error) {
	deviceName = strings.TrimSpace(deviceName)
	if requestedPort < 0 || requestedPort > 65535 || deviceName == "" {
		return libraryaccess.LANInfo{}, errors.New("invalid Library LAN runtime configuration")
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	endpointProvider := runtime.config.LANEndpoints
	if endpointProvider == nil {
		endpointProvider = discovery.SystemEligibleLANEndpoints
	}
	endpoints, endpointErr := endpointProvider()
	if endpointErr != nil || len(endpoints) == 0 {
		transportErr := runtime.stopTransportLocked(ctx)
		if endpointErr == nil {
			endpointErr = fmt.Errorf("%w: no eligible LAN endpoint", discovery.ErrUnavailable)
		}
		return runtime.failLocked(errors.Join(endpointErr, transportErr))
	}
	bindPort := requestedPort
	if bindPort == 0 && runtime.stablePort > 0 {
		// A zero preference means "keep the runtime-selected stable port", not
		// "allocate a fresh port on every interface inventory change".
		bindPort = runtime.stablePort
	}
	desiredAddresses := make([]string, 0, len(endpoints))
	listenerEndpoints := make([]discovery.LANEndpoint, 0, len(endpoints))
	interfaceIndices := make([]int, 0, len(endpoints))
	seenInterfaces := make(map[int]struct{}, len(endpoints))
	seenAddresses := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		address := endpoint.ListenAddress(bindPort)
		if _, exists := seenAddresses[address]; !exists {
			seenAddresses[address] = struct{}{}
			desiredAddresses = append(desiredAddresses, address)
			listenerEndpoints = append(listenerEndpoints, endpoint)
		}
		if _, exists := seenInterfaces[endpoint.InterfaceIndex]; !exists {
			seenInterfaces[endpoint.InterfaceIndex] = struct{}{}
			interfaceIndices = append(interfaceIndices, endpoint.InterfaceIndex)
		}
	}
	if runtime.stablePort > 0 && runtime.registration != nil && len(runtime.config.Server.LANAddresses()) > 0 &&
		bindPort == runtime.stablePort && runtime.deviceName == deviceName &&
		listenersMatchEndpoints(runtime.config.Server.LANAddresses(), listenerEndpoints) {
		if runtime.firewallErr != nil {
			return runtime.retryFirewallLocked(ctx)
		}
		return runtime.info, nil
	}
	if err := runtime.stopAdvertisementLocked(); err != nil {
		return runtime.failLocked(err)
	}
	runtime.info = libraryaccess.LANInfo{State: libraryaccess.StateStarting, Port: runtime.stablePort}

	listener, err := runtime.config.Server.EnableLANAddresses(desiredAddresses, runtime.config.Identity)
	if err != nil {
		return runtime.failLocked(err)
	}
	runtime.stablePort = listener.Port
	rollback := true
	defer func() {
		if rollback {
			_ = runtime.config.Server.DisableLAN(context.Background())
		}
	}()
	// Interface and device-name changes do not affect a Program+Port-scoped
	// Windows rule. Reuse a verified rule for the stable port; only a real port
	// change removes the old rule before configuring the replacement.
	firewallErr := runtime.ensureFirewallLocked(ctx, listener.Port)
	registrationCtx, cancel := context.WithCancel(context.Background())
	registration, err := runtime.config.Advertiser.Register(registrationCtx, discovery.Advertisement{
		Name: strings.TrimSpace(deviceName), Port: listener.Port, InterfaceIndices: interfaceIndices,
	})
	if err != nil {
		cancel()
		return runtime.failLocked(fmt.Errorf("advertise Library DNS-SD service: %w", err))
	}
	runtime.registration = registration
	runtime.cancel = cancel
	runtime.deviceName = deviceName
	runtime.info = libraryaccess.LANInfo{State: libraryaccess.StateRunning, Port: listener.Port, Address: listener.Address}
	if firewallErr != nil {
		runtime.info.State = libraryaccess.StateError
		runtime.info.LastError = firewallErr.Error()
	}
	rollback = false
	return runtime.info, firewallErr
}

func (runtime *LANRuntime) ensureFirewallLocked(ctx context.Context, port int) error {
	if runtime.config.Firewall == nil || strings.TrimSpace(runtime.config.Program) == "" {
		runtime.firewallErr = nil
		return nil
	}
	if runtime.firewallPort == port && runtime.firewallErr == nil {
		return nil
	}
	if runtime.firewallPort > 0 && runtime.firewallPort != port {
		if err := runtime.config.Firewall.Disable(ctx); err != nil && !errors.Is(err, firewall.ErrUnavailable) {
			runtime.firewallErr = fmt.Errorf("replace LAN firewall rule: %w", err)
			return runtime.firewallErr
		}
		runtime.firewallPort = 0
	}
	if err := runtime.config.Firewall.Enable(ctx, firewall.Rule{Program: runtime.config.Program, Port: port}); err != nil {
		if errors.Is(err, firewall.ErrUnavailable) {
			// Unsupported platforms need no managed rule, but remember the port so
			// ordinary interface refreshes remain side-effect free.
			runtime.firewallPort = port
			runtime.firewallErr = nil
			return nil
		}
		// Windows firewall rule creation normally requires elevation. The
		// listener is already TLS-only and the attempted rule is constrained to
		// Private + LocalSubnet, so retain it and surface a retryable error.
		runtime.firewallErr = fmt.Errorf("configure LAN firewall: %w", err)
		return runtime.firewallErr
	}
	runtime.firewallPort = port
	runtime.firewallErr = nil
	return nil
}

func listenersMatchEndpoints(addresses []string, endpoints []discovery.LANEndpoint) bool {
	if len(addresses) != len(endpoints) || len(addresses) == 0 {
		return false
	}
	port := 0
	for index, address := range addresses {
		current, err := net.ResolveTCPAddr("tcp", strings.TrimSpace(address))
		if err != nil || current == nil || current.IP == nil ||
			!current.IP.Equal(endpoints[index].IP) || strings.TrimSpace(current.Zone) != strings.TrimSpace(endpoints[index].Zone) {
			return false
		}
		if index == 0 {
			port = current.Port
		} else if current.Port != port {
			return false
		}
	}
	return port > 0
}

// retryFirewallLocked lets a user approve elevation and apply the same
// Private/LocalSubnet rule without interrupting the active TLS listener or
// replacing its DNS-SD registration.
func (runtime *LANRuntime) retryFirewallLocked(ctx context.Context) (libraryaccess.LANInfo, error) {
	if runtime.config.Firewall == nil || strings.TrimSpace(runtime.config.Program) == "" {
		return runtime.info, runtime.firewallErr
	}
	err := runtime.ensureFirewallLocked(ctx, runtime.stablePort)
	if err != nil {
		runtime.info.State = libraryaccess.StateError
		runtime.info.LastError = runtime.firewallErr.Error()
		return runtime.info, runtime.firewallErr
	}
	runtime.firewallErr = nil
	runtime.info.State = libraryaccess.StateRunning
	runtime.info.LastError = ""
	return runtime.info, nil
}

func (runtime *LANRuntime) Disable(ctx context.Context) error {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.disableLocked(ctx)
}

func (runtime *LANRuntime) disableLocked(ctx context.Context) error {
	result := runtime.stopTransportLocked(ctx)
	if runtime.config.Firewall != nil {
		if err := runtime.config.Firewall.Disable(ctx); err != nil && !errors.Is(err, firewall.ErrUnavailable) {
			result = errors.Join(result, err)
		}
	}
	runtime.deviceName = ""
	runtime.stablePort = 0
	runtime.firewallPort = 0
	runtime.firewallErr = nil
	runtime.info = libraryaccess.LANInfo{State: libraryaccess.StateDisabled}
	return result
}

func (runtime *LANRuntime) stopAdvertisementLocked() error {
	var result error
	if runtime.cancel != nil {
		runtime.cancel()
		runtime.cancel = nil
	}
	if runtime.registration != nil {
		result = errors.Join(result, runtime.registration.Close())
		runtime.registration = nil
	}
	return result
}

func (runtime *LANRuntime) stopTransportLocked(ctx context.Context) error {
	return errors.Join(runtime.stopAdvertisementLocked(), runtime.config.Server.DisableLAN(ctx))
}

func (runtime *LANRuntime) failLocked(err error) (libraryaccess.LANInfo, error) {
	runtime.info = libraryaccess.LANInfo{State: libraryaccess.StateError, Port: runtime.stablePort, LastError: err.Error()}
	return runtime.info, err
}

var _ libraryaccess.LANManager = (*LANRuntime)(nil)
