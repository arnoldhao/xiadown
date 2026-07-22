package libraryaccess

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	domain "xiadown/internal/domain/libraryaccess"
)

const (
	StateDisabled     = "disabled"
	StateStarting     = "starting"
	StateRunning      = "running"
	StateDisconnected = "disconnected"
	StateUnavailable  = "unavailable"
	StateError        = "error"
)

type LANInfo struct {
	State     string
	Port      int
	Address   string
	LastError string
}

// LANManager is the runtime seam for the future isolated public API listener
// and DNS-SD advertiser. It deliberately does not expose the existing local
// application HTTP server.
type LANManager interface {
	Inspect(context.Context) LANInfo
	Enable(ctx context.Context, requestedPort int, deviceName string) (LANInfo, error)
	Disable(context.Context) error
}

type Service struct {
	repo              domain.Repository
	tailscale         domain.TailscaleManager
	lan               LANManager
	defaultDeviceName string

	mu                 sync.Mutex
	lastLANError       string
	lastTailscaleError string
}

func NewService(repo domain.Repository, tailscale domain.TailscaleManager, lan LANManager, defaultDeviceName string) *Service {
	defaultDeviceName = strings.TrimSpace(defaultDeviceName)
	if defaultDeviceName == "" {
		defaultDeviceName = "XiaDown"
	}
	return &Service{repo: repo, tailscale: tailscale, lan: lan, defaultDeviceName: defaultDeviceName}
}

func (service *Service) GetConfig(ctx context.Context) (Config, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	config, err := service.loadConfig(ctx)
	if err != nil {
		return Config{}, err
	}
	return configDTO(config), nil
}

// UpdateConfig stores desired state independently from runtime state. A
// temporary network or Tailscale failure must not erase the user's choice;
// Apply can safely retry it later.
func (service *Service) UpdateConfig(ctx context.Context, request UpdateConfigRequest) (Config, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	current, err := service.loadConfig(ctx)
	if err != nil {
		return Config{}, err
	}
	params := domain.ConfigParams{
		RemoteEnabled: current.RemoteEnabled, LANEnabled: current.LANEnabled,
		LANPort: current.LANPort, TailscaleEnabled: current.TailscaleEnabled,
		TailscaleHTTPSPort: current.TailscaleHTTPSPort,
		TailscalePath:      current.TailscalePath, DeviceName: current.DeviceName,
	}
	if request.RemoteEnabled != nil {
		params.RemoteEnabled = *request.RemoteEnabled
	}
	if request.LANEnabled != nil {
		params.LANEnabled = *request.LANEnabled
	}
	if request.LANPort != nil {
		params.LANPort = *request.LANPort
	}
	if request.TailscaleEnabled != nil {
		params.TailscaleEnabled = *request.TailscaleEnabled
	}
	if request.TailscaleHTTPSPort != nil {
		params.TailscaleHTTPSPort = *request.TailscaleHTTPSPort
	}
	if request.TailscalePath != nil {
		params.TailscalePath = *request.TailscalePath
	}
	if request.DeviceName != nil {
		params.DeviceName = *request.DeviceName
	}
	updated, err := domain.NewConfig(params)
	if err != nil {
		return Config{}, err
	}
	if err := service.repo.Save(ctx, updated); err != nil {
		return Config{}, err
	}
	return configDTO(updated), nil
}

func (service *Service) GetStatus(ctx context.Context) (Status, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	config, err := service.loadConfig(ctx)
	if err != nil {
		return Status{}, err
	}
	return service.statusLocked(ctx, config), nil
}

// Apply reconciles desired state with injectable LAN and Tailscale runtimes.
// localAPIPort must be the loopback port of the new isolated public API, never
// the process-token local application server.
func (service *Service) Apply(ctx context.Context, localAPIPort int) (Status, error) {
	return service.apply(ctx, localAPIPort, true)
}

// applyAutomatically is used only by the background reconciler. A retained
// firewall/elevation error requires an explicit desktop action; automatic
// healing of another component must never turn into another Windows UAC prompt.
func (service *Service) applyAutomatically(ctx context.Context, localAPIPort int) (Status, error) {
	return service.apply(ctx, localAPIPort, false)
}

func (service *Service) apply(ctx context.Context, localAPIPort int, allowManualTransportRetry bool) (Status, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	config, err := service.loadConfig(ctx)
	if err != nil {
		return Status{}, err
	}

	lanDesired := config.RemoteEnabled && config.LANEnabled
	service.lastLANError = ""
	if service.lan == nil {
		if lanDesired {
			service.lastLANError = "LAN runtime unavailable"
		}
	} else if lanDesired {
		observed := service.lan.Inspect(ctx)
		if !allowManualTransportRetry && requiresManualLANIntervention(observed.LastError) {
			service.lastLANError = strings.TrimSpace(observed.LastError)
		} else {
			if _, err := service.lan.Enable(ctx, config.LANPort, config.DeviceName); err != nil {
				service.lastLANError = err.Error()
			}
		}
	} else if err := service.lan.Disable(ctx); err != nil {
		service.lastLANError = err.Error()
	}

	tailscaleDesired := config.RemoteEnabled && config.TailscaleEnabled
	if !allowManualTransportRetry && tailscaleDesired {
		observed := service.statusLocked(ctx, config).Tailscale
		if requiresManualTailscaleIntervention(observed.LastError) {
			service.lastTailscaleError = strings.TrimSpace(observed.LastError)
		} else {
			service.lastTailscaleError = service.reconcileTailscale(ctx, config, localAPIPort, true)
		}
	} else {
		service.lastTailscaleError = service.reconcileTailscale(ctx, config, localAPIPort, tailscaleDesired)
	}
	return service.statusLocked(ctx, config), nil
}

func (service *Service) reconcileTailscale(
	ctx context.Context,
	config domain.Config,
	localAPIPort int,
	desired bool,
) string {
	managed, err := service.repo.GetManagedTailscaleRoute(ctx)
	if err != nil && !errors.Is(err, domain.ErrManagedTailscaleRouteNotFound) {
		return fmt.Sprintf("read XiaDown Tailscale route ownership: %v", err)
	}
	if errors.Is(err, domain.ErrManagedTailscaleRouteNotFound) {
		managed = domain.ManagedTailscaleRoute{}
	}

	if !desired {
		if !managed.Claimed() {
			// Without durable ownership evidence XiaDown must not infer that a
			// coincidentally matching loopback Serve route belongs to it.
			return ""
		}
		return service.disableManagedTailscaleRoute(ctx, managed)
	}

	if localAPIPort < 1 || localAPIPort > 65535 {
		return "isolated public API port unavailable"
	}
	if managed.Claimed() && !managed.SameEndpoint(config.TailscaleHTTPSPort, config.TailscalePath) {
		if message := service.disableManagedTailscaleRoute(ctx, managed); message != "" {
			// Never add the new route until the previous exact owned route is
			// confirmed off. This is what prevents stale paths accumulating.
			return message
		}
		managed = domain.ManagedTailscaleRoute{}
	}
	return service.enableManagedTailscaleRoute(
		ctx, managed, localAPIPort, config.TailscaleHTTPSPort, config.TailscalePath,
	)
}

func (service *Service) enableManagedTailscaleRoute(
	ctx context.Context,
	managed domain.ManagedTailscaleRoute,
	localAPIPort int,
	httpsPort int,
	routePath string,
) string {
	observation, observeErr := service.observeTailscaleRoute(ctx, managed, httpsPort, routePath)
	if observeErr != nil {
		if !errors.Is(observeErr, domain.ErrTailscaleRouteOwnershipConflict) {
			return observeErr.Error()
		}
		backendPort := knownManagedBackend(managed)
		if _, err := service.transitionTailscaleRoute(
			ctx, httpsPort, routePath, backendPort, localAPIPort,
			domain.TailscaleRouteStateEnabling,
			domain.TailscaleRouteActionEnable,
			domain.TailscaleRouteResultPending,
			"",
		); err != nil {
			return fmt.Sprintf("persist XiaDown Tailscale route conflict before enable: %v", err)
		}
		return service.failAndReleaseTailscaleRoute(
			ctx, httpsPort, routePath, backendPort, localAPIPort,
			domain.TailscaleRouteActionEnable, observeErr.Error(),
		)
	}
	backendPort := observation.backendPort
	if !observation.exists && backendPort == 0 {
		// Preserve prior ownership evidence across a temporarily missing route.
		// If the exact old handler reappears between this observation and the
		// manager's mandatory preflight, replacing it is still safe; a different
		// target remains an ownership conflict.
		backendPort = knownManagedBackend(managed)
	}
	ownership := mutationOwnership(managed, backendPort)
	if _, err := service.transitionTailscaleRoute(
		ctx,
		httpsPort,
		routePath,
		backendPort,
		localAPIPort,
		domain.TailscaleRouteStateEnabling,
		domain.TailscaleRouteActionEnable,
		domain.TailscaleRouteResultPending,
		"",
	); err != nil {
		return fmt.Sprintf("persist XiaDown Tailscale route before enable: %v", err)
	}

	if service.tailscale == nil {
		return service.failTailscaleTransition(
			ctx, httpsPort, routePath, backendPort, localAPIPort,
			domain.TailscaleRouteActionEnable,
			"Tailscale runtime unavailable",
		)
	}
	if err := service.tailscale.Enable(
		ctx, localAPIPort, httpsPort, routePath,
		ownership,
	); err != nil {
		if errors.Is(err, domain.ErrTailscaleRouteOwnershipConflict) {
			return service.failAndReleaseTailscaleRoute(
				ctx, httpsPort, routePath, backendPort, localAPIPort,
				domain.TailscaleRouteActionEnable, err.Error(),
			)
		}
		return service.failTailscaleTransition(
			ctx, httpsPort, routePath, backendPort, localAPIPort,
			domain.TailscaleRouteActionEnable, err.Error(),
		)
	}
	if _, err := service.transitionTailscaleRoute(
		ctx,
		httpsPort,
		routePath,
		localAPIPort,
		0,
		domain.TailscaleRouteStateActive,
		domain.TailscaleRouteActionEnable,
		domain.TailscaleRouteResultSucceeded,
		"",
	); err != nil {
		return fmt.Sprintf("XiaDown Tailscale route enabled but success audit failed: %v", err)
	}
	return ""
}

func (service *Service) disableManagedTailscaleRoute(
	ctx context.Context,
	managed domain.ManagedTailscaleRoute,
) string {
	observation, observeErr := service.observeTailscaleRoute(
		ctx, managed, managed.HTTPSPort, managed.Path,
	)
	if observeErr != nil {
		if !errors.Is(observeErr, domain.ErrTailscaleRouteOwnershipConflict) {
			return observeErr.Error()
		}
		backendPort := knownManagedBackend(managed)
		if backendPort == 0 {
			if err := service.auditTailscaleRouteRelease(
				ctx, managed.HTTPSPort, managed.Path, 0, observeErr.Error(),
			); err != nil {
				return fmt.Sprintf("%s (ownership release audit failed: %v)", observeErr.Error(), err)
			}
			return observeErr.Error()
		}
		if _, err := service.transitionTailscaleRoute(
			ctx, managed.HTTPSPort, managed.Path, backendPort, 0,
			domain.TailscaleRouteStateDisabling,
			domain.TailscaleRouteActionDisable,
			domain.TailscaleRouteResultPending,
			"",
		); err != nil {
			return fmt.Sprintf("persist XiaDown Tailscale route conflict before disable: %v", err)
		}
		return service.failAndReleaseTailscaleRoute(
			ctx, managed.HTTPSPort, managed.Path, backendPort, 0,
			domain.TailscaleRouteActionDisable, observeErr.Error(),
		)
	}
	backendPort := observation.backendPort
	if !observation.exists && backendPort == 0 {
		backendPort = knownManagedBackend(managed)
	}
	if backendPort == 0 {
		if err := service.auditTailscaleRouteRelease(
			ctx, managed.HTTPSPort, managed.Path, 0,
			"legacy XiaDown Tailscale route was absent; ownership released",
		); err != nil {
			return fmt.Sprintf("release absent legacy XiaDown Tailscale route ownership: %v", err)
		}
		return ""
	}
	ownership := mutationOwnership(managed, backendPort)
	if _, err := service.transitionTailscaleRoute(
		ctx,
		managed.HTTPSPort,
		managed.Path,
		backendPort,
		0,
		domain.TailscaleRouteStateDisabling,
		domain.TailscaleRouteActionDisable,
		domain.TailscaleRouteResultPending,
		"",
	); err != nil {
		return fmt.Sprintf("persist XiaDown Tailscale route before disable: %v", err)
	}

	if service.tailscale == nil {
		return service.failTailscaleTransition(
			ctx, managed.HTTPSPort, managed.Path, backendPort, 0,
			domain.TailscaleRouteActionDisable,
			"Tailscale runtime unavailable",
		)
	}
	if err := service.tailscale.Disable(
		ctx, managed.HTTPSPort, managed.Path,
		ownership,
	); err != nil {
		if errors.Is(err, domain.ErrTailscaleRouteOwnershipConflict) {
			return service.failAndReleaseTailscaleRoute(
				ctx, managed.HTTPSPort, managed.Path, backendPort, 0,
				domain.TailscaleRouteActionDisable, err.Error(),
			)
		}
		return service.failTailscaleTransition(
			ctx, managed.HTTPSPort, managed.Path, backendPort, 0,
			domain.TailscaleRouteActionDisable, err.Error(),
		)
	}
	if _, err := service.transitionTailscaleRoute(
		ctx,
		managed.HTTPSPort,
		managed.Path,
		backendPort,
		0,
		domain.TailscaleRouteStateInactive,
		domain.TailscaleRouteActionDisable,
		domain.TailscaleRouteResultSucceeded,
		"",
	); err != nil {
		return fmt.Sprintf("XiaDown Tailscale route disabled but success audit failed: %v", err)
	}
	return ""
}

type observedTailscaleRoute struct {
	exists      bool
	backendPort int
}

func (service *Service) observeTailscaleRoute(
	ctx context.Context,
	managed domain.ManagedTailscaleRoute,
	httpsPort int,
	routePath string,
) (observedTailscaleRoute, error) {
	if service.tailscale == nil {
		return observedTailscaleRoute{}, errors.New("Tailscale runtime unavailable")
	}
	info := service.tailscale.Inspect(ctx, httpsPort, routePath)
	switch {
	case !info.Installed:
		return observedTailscaleRoute{}, errors.New("Tailscale is not installed")
	case strings.TrimSpace(info.LastError) != "":
		return observedTailscaleRoute{}, fmt.Errorf("inspect exact Tailscale Serve route: %s", info.LastError)
	case !info.Connected:
		return observedTailscaleRoute{}, errors.New("Tailscale is disconnected")
	case !info.RouteChecked:
		return observedTailscaleRoute{}, errors.New("exact Tailscale Serve route could not be verified")
	case !info.RouteExists:
		return observedTailscaleRoute{exists: false}, nil
	}

	if managed.Claimed() {
		if managed.State == domain.TailscaleRouteStateUnknown &&
			managed.BackendPort == 0 && managed.PendingBackendPort == 0 &&
			info.RouteBackendPort > 0 {
			// The v7 migration is the only path allowed to adopt one legacy
			// loopback target without a previously persisted backend port.
			return observedTailscaleRoute{exists: true, backendPort: info.RouteBackendPort}, nil
		}
		if managed.Ownership().AllowsBackendPort(info.RouteBackendPort) {
			return observedTailscaleRoute{exists: true, backendPort: info.RouteBackendPort}, nil
		}
	}
	return observedTailscaleRoute{exists: true}, fmt.Errorf(
		"%w: HTTPS port %d path %s is occupied or was changed outside XiaDown",
		domain.ErrTailscaleRouteOwnershipConflict,
		httpsPort,
		routePath,
	)
}

func knownManagedBackend(managed domain.ManagedTailscaleRoute) int {
	if !managed.Claimed() {
		return 0
	}
	if managed.BackendPort > 0 {
		return managed.BackendPort
	}
	return managed.PendingBackendPort
}

func mutationOwnership(
	managed domain.ManagedTailscaleRoute,
	observedBackendPort int,
) domain.TailscaleRouteOwnership {
	ports := make([]int, 0, 2)
	addPort := func(port int) {
		if port <= 0 {
			return
		}
		for _, existing := range ports {
			if existing == port {
				return
			}
		}
		ports = append(ports, port)
	}
	addPort(observedBackendPort)
	if managed.Claimed() {
		addPort(managed.BackendPort)
		addPort(managed.PendingBackendPort)
	}
	ownership := domain.TailscaleRouteOwnership{}
	if len(ports) > 0 {
		ownership.BackendPort = ports[0]
	}
	if len(ports) > 1 {
		ownership.PendingBackendPort = ports[1]
	}
	return ownership
}

func (service *Service) failTailscaleTransition(
	ctx context.Context,
	httpsPort int,
	routePath string,
	backendPort int,
	pendingBackendPort int,
	action domain.TailscaleRouteAction,
	message string,
) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "unknown Tailscale route error"
	}
	if _, err := service.transitionTailscaleRoute(
		ctx,
		httpsPort,
		routePath,
		backendPort,
		pendingBackendPort,
		domain.TailscaleRouteStateError,
		action,
		domain.TailscaleRouteResultFailed,
		message,
	); err != nil {
		return fmt.Sprintf("%s (failure audit failed: %v)", message, err)
	}
	return message
}

func (service *Service) failAndReleaseTailscaleRoute(
	ctx context.Context,
	httpsPort int,
	routePath string,
	backendPort int,
	pendingBackendPort int,
	action domain.TailscaleRouteAction,
	message string,
) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "Tailscale route ownership changed outside XiaDown"
	}
	if _, err := service.transitionTailscaleRoute(
		ctx, httpsPort, routePath, backendPort, pendingBackendPort,
		domain.TailscaleRouteStateError, action,
		domain.TailscaleRouteResultFailed, message,
	); err != nil {
		return fmt.Sprintf("%s (failure audit failed: %v)", message, err)
	}
	if err := service.auditTailscaleRouteRelease(ctx, httpsPort, routePath, backendPort, message); err != nil {
		return fmt.Sprintf("%s (ownership release audit failed: %v)", message, err)
	}
	return message
}

func (service *Service) auditTailscaleRouteRelease(
	ctx context.Context,
	httpsPort int,
	routePath string,
	backendPort int,
	message string,
) error {
	if _, err := service.transitionTailscaleRoute(
		ctx, httpsPort, routePath, backendPort, 0,
		domain.TailscaleRouteStateInactive,
		domain.TailscaleRouteActionRelease,
		domain.TailscaleRouteResultSucceeded,
		strings.TrimSpace(message),
	); err != nil {
		return err
	}
	return nil
}

func (service *Service) transitionTailscaleRoute(
	ctx context.Context,
	httpsPort int,
	routePath string,
	backendPort int,
	pendingBackendPort int,
	state domain.TailscaleRouteState,
	action domain.TailscaleRouteAction,
	result domain.TailscaleRouteResult,
	errorMessage string,
) (domain.ManagedTailscaleRoute, error) {
	transition, err := domain.NewTailscaleRouteTransition(
		httpsPort, routePath, backendPort, pendingBackendPort,
		state, action, result, errorMessage,
	)
	if err != nil {
		return domain.ManagedTailscaleRoute{}, err
	}
	return service.repo.TransitionManagedTailscaleRoute(ctx, transition)
}

func (service *Service) loadConfig(ctx context.Context) (domain.Config, error) {
	if service == nil || service.repo == nil {
		return domain.Config{}, errors.New("library access repository unavailable")
	}
	config, err := service.repo.Get(ctx)
	if err == nil {
		return config, nil
	}
	if !errors.Is(err, domain.ErrConfigNotFound) {
		return domain.Config{}, err
	}
	config, err = domain.DefaultConfig(service.defaultDeviceName)
	if err != nil {
		return domain.Config{}, err
	}
	if err := service.repo.Save(ctx, config); err != nil {
		return domain.Config{}, fmt.Errorf("save default library access config: %w", err)
	}
	return config, nil
}

func (service *Service) statusLocked(ctx context.Context, config domain.Config) Status {
	lanDesired := config.RemoteEnabled && config.LANEnabled
	lanStatus := LANStatus{DesiredEnabled: lanDesired, State: StateDisabled}
	if service.lan == nil {
		if lanDesired {
			lanStatus.State = StateUnavailable
		}
	} else {
		info := service.lan.Inspect(ctx)
		lanStatus.State = normalizeLANState(info.State, lanDesired)
		lanStatus.Port = info.Port
		lanStatus.LastError = strings.TrimSpace(info.LastError)
	}
	observedLANState := lanStatus.State
	if service.lastLANError != "" {
		lanStatus.State = StateError
		lanStatus.LastError = service.lastLANError
	}

	tailscaleDesired := config.RemoteEnabled && config.TailscaleEnabled
	tailscaleStatus := TailscaleStatus{DesiredEnabled: tailscaleDesired, State: StateDisabled}
	managed, managedErr := service.repo.GetManagedTailscaleRoute(ctx)
	inspectHTTPSPort := config.TailscaleHTTPSPort
	inspectPath := config.TailscalePath
	if managedErr == nil && managed.Claimed() {
		inspectHTTPSPort = managed.HTTPSPort
		inspectPath = managed.Path
	}
	if service.tailscale == nil {
		if tailscaleDesired {
			tailscaleStatus.State = StateUnavailable
		}
	} else {
		info := service.tailscale.Inspect(ctx, inspectHTTPSPort, inspectPath)
		candidateServeURL := info.ServeURL
		info.ServeURL = ""
		routeMatchesClaim := managedErr == nil && managed.Claimed() &&
			managed.State != domain.TailscaleRouteStateUnknown &&
			info.RouteChecked && info.RouteExists &&
			managed.Ownership().AllowsBackendPort(info.RouteBackendPort)
		if routeMatchesClaim {
			info.ServeURL = candidateServeURL
		}
		tailscaleStatus.Installed = info.Installed
		tailscaleStatus.Version = info.Version
		tailscaleStatus.Tailnet = info.Tailnet
		tailscaleStatus.Device = info.Device
		tailscaleStatus.ServeURL = info.ServeURL
		tailscaleStatus.LastError = strings.TrimSpace(info.LastError)
		tailscaleStatus.State = tailscaleState(info, tailscaleDesired)
		if managedErr == nil && managed.Claimed() && info.RouteChecked && info.RouteExists {
			legacyCandidate := managed.State == domain.TailscaleRouteStateUnknown &&
				managed.BackendPort == 0 && managed.PendingBackendPort == 0 &&
				info.RouteBackendPort > 0
			if !routeMatchesClaim && !legacyCandidate {
				tailscaleStatus.State = StateError
				tailscaleStatus.LastError = "XiaDown Tailscale route was changed outside XiaDown"
			}
		}
	}
	if managedErr != nil && !errors.Is(managedErr, domain.ErrManagedTailscaleRouteNotFound) {
		tailscaleStatus.State = StateError
		tailscaleStatus.LastError = fmt.Sprintf("read XiaDown Tailscale route ownership: %v", managedErr)
	} else if managedErr == nil {
		switch {
		case managed.LastResult == domain.TailscaleRouteResultFailed:
			tailscaleStatus.State = StateError
			tailscaleStatus.LastError = managed.LastError
		case managed.Claimed() && !tailscaleDesired && tailscaleStatus.State != StateError:
			tailscaleStatus.State = StateStarting
		case tailscaleDesired && managed.State != domain.TailscaleRouteStateActive && tailscaleStatus.State != StateError:
			tailscaleStatus.State = StateStarting
		}
	}
	observedTailscaleState := tailscaleStatus.State
	if service.lastTailscaleError != "" {
		tailscaleStatus.State = StateError
		tailscaleStatus.LastError = service.lastTailscaleError
	}
	return Status{
		DesiredEnabled: config.RemoteEnabled, LAN: lanStatus, Tailscale: tailscaleStatus,
		observedLANState: observedLANState, observedTailscaleState: observedTailscaleState,
	}
}

func normalizeLANState(state string, desired bool) string {
	state = strings.ToLower(strings.TrimSpace(state))
	switch state {
	case StateDisabled, StateStarting, StateRunning, StateUnavailable, StateError:
		return state
	case "":
		if desired {
			return StateStarting
		}
		return StateDisabled
	default:
		return StateError
	}
}

func tailscaleState(info domain.TailscaleInfo, desired bool) string {
	if !info.Installed {
		return StateUnavailable
	}
	if info.LastError != "" {
		return StateError
	}
	if !desired {
		return StateDisabled
	}
	if !info.Connected {
		return StateDisconnected
	}
	if strings.TrimSpace(info.ServeURL) == "" {
		return StateStarting
	}
	return StateRunning
}

func configDTO(config domain.Config) Config {
	return Config{
		RemoteEnabled: config.RemoteEnabled, LANEnabled: config.LANEnabled,
		LANPort: config.LANPort, TailscaleEnabled: config.TailscaleEnabled,
		TailscaleHTTPSPort: config.TailscaleHTTPSPort,
		TailscalePath:      config.TailscalePath, DeviceName: config.DeviceName,
	}
}
