package libraryaccess

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	domain "xiadown/internal/domain/libraryaccess"
)

type repositoryStub struct {
	config         domain.Config
	err            error
	saves          []domain.Config
	managed        domain.ManagedTailscaleRoute
	managedErr     error
	transitionErrs []error
	transitions    []domain.TailscaleRouteTransition
	events         *[]string
}

func (repo *repositoryStub) Get(context.Context) (domain.Config, error) {
	return repo.config, repo.err
}

func (repo *repositoryStub) Save(_ context.Context, config domain.Config) error {
	repo.config = config
	repo.err = nil
	repo.saves = append(repo.saves, config)
	return nil
}

func (repo *repositoryStub) GetManagedTailscaleRoute(context.Context) (domain.ManagedTailscaleRoute, error) {
	if repo.managedErr != nil {
		return domain.ManagedTailscaleRoute{}, repo.managedErr
	}
	if repo.managed.State == "" {
		return domain.ManagedTailscaleRoute{}, domain.ErrManagedTailscaleRouteNotFound
	}
	return repo.managed, nil
}

func (repo *repositoryStub) TransitionManagedTailscaleRoute(
	_ context.Context,
	transition domain.TailscaleRouteTransition,
) (domain.ManagedTailscaleRoute, error) {
	repo.transitions = append(repo.transitions, transition)
	if repo.events != nil {
		*repo.events = append(*repo.events, fmt.Sprintf(
			"persist:%s:%s:%d:%s",
			transition.Action, transition.Result, transition.HTTPSPort, transition.Path,
		))
	}
	index := len(repo.transitions) - 1
	if index < len(repo.transitionErrs) && repo.transitionErrs[index] != nil {
		return domain.ManagedTailscaleRoute{}, repo.transitionErrs[index]
	}
	repo.managedErr = nil
	repo.managed = domain.ManagedTailscaleRoute{
		HTTPSPort:          transition.HTTPSPort,
		Path:               transition.Path,
		BackendPort:        transition.BackendPort,
		PendingBackendPort: transition.PendingBackendPort,
		State:              transition.State,
		LastAction:         transition.Action,
		LastResult:         transition.Result,
		LastError:          transition.Error,
		Revision:           repo.managed.Revision + 1,
	}
	return repo.managed, nil
}

type tailscaleStub struct {
	info         domain.TailscaleInfo
	enableErr    error
	disableErr   error
	enableCalls  []tailscaleEnableCall
	disableCalls []tailscaleDisableCall
	events       *[]string
}

type tailscaleEnableCall struct {
	localPort, httpsPort int
	path                 string
	ownership            domain.TailscaleRouteOwnership
}

type tailscaleDisableCall struct {
	httpsPort int
	path      string
	ownership domain.TailscaleRouteOwnership
}

func (stub *tailscaleStub) Inspect(context.Context, int, string) domain.TailscaleInfo {
	info := stub.info
	if info.Installed && info.Connected && info.LastError == "" && !info.RouteChecked {
		info.RouteChecked = true
	}
	return info
}

func (stub *tailscaleStub) Enable(
	_ context.Context,
	localPort, httpsPort int,
	routePath string,
	ownership domain.TailscaleRouteOwnership,
) error {
	stub.enableCalls = append(stub.enableCalls, tailscaleEnableCall{
		localPort: localPort, httpsPort: httpsPort, path: routePath, ownership: ownership,
	})
	if stub.events != nil {
		*stub.events = append(*stub.events, fmt.Sprintf("enable:%d:%s", httpsPort, routePath))
	}
	if stub.enableErr == nil {
		stub.info.RouteChecked = true
		stub.info.RouteExists = true
		stub.info.RouteTarget = fmt.Sprintf("http://127.0.0.1:%d", localPort)
		stub.info.RouteBackendPort = localPort
		host := "studio.example.ts.net"
		if httpsPort != domain.DefaultTailscaleHTTPSPort {
			host = fmt.Sprintf("%s:%d", host, httpsPort)
		}
		stub.info.ServeURL = fmt.Sprintf("https://%s%s", host, routePath)
	}
	return stub.enableErr
}

func (stub *tailscaleStub) Disable(
	_ context.Context,
	httpsPort int,
	routePath string,
	ownership domain.TailscaleRouteOwnership,
) error {
	stub.disableCalls = append(stub.disableCalls, tailscaleDisableCall{
		httpsPort: httpsPort, path: routePath, ownership: ownership,
	})
	if stub.events != nil {
		*stub.events = append(*stub.events, fmt.Sprintf("disable:%d:%s", httpsPort, routePath))
	}
	if stub.disableErr == nil {
		stub.info.RouteChecked = true
		stub.info.RouteExists = false
		stub.info.RouteTarget = ""
		stub.info.RouteBackendPort = 0
		stub.info.ServeURL = ""
	}
	return stub.disableErr
}

type lanStub struct {
	info         LANInfo
	enableErr    error
	disableErr   error
	enableCalls  int
	disableCalls int
	port         int
	deviceName   string
}

func (stub *lanStub) Inspect(context.Context) LANInfo { return stub.info }
func (stub *lanStub) Enable(_ context.Context, port int, deviceName string) (LANInfo, error) {
	stub.enableCalls++
	stub.port = port
	stub.deviceName = deviceName
	return stub.info, stub.enableErr
}
func (stub *lanStub) Disable(context.Context) error {
	stub.disableCalls++
	return stub.disableErr
}

func testConfig(t *testing.T, remote, lan, tailscale bool) domain.Config {
	t.Helper()
	config, err := domain.NewConfig(domain.ConfigParams{
		RemoteEnabled: remote, LANEnabled: lan, LANPort: 0,
		TailscaleEnabled: tailscale, TailscaleHTTPSPort: 443,
		TailscalePath: "/xiadown", DeviceName: "Studio",
	})
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	return config
}

func activeManagedRoute(httpsPort int, routePath string) domain.ManagedTailscaleRoute {
	return domain.ManagedTailscaleRoute{
		HTTPSPort:   httpsPort,
		Path:        routePath,
		BackendPort: 43123,
		State:       domain.TailscaleRouteStateActive,
		LastAction:  domain.TailscaleRouteActionEnable,
		LastResult:  domain.TailscaleRouteResultSucceeded,
		Revision:    2,
	}
}

func exactRouteInfo(backendPort int, serveURL string) domain.TailscaleInfo {
	return domain.TailscaleInfo{
		Installed:        true,
		Connected:        true,
		DNSName:          "studio.example.ts.net",
		ServeURL:         serveURL,
		RouteChecked:     true,
		RouteExists:      true,
		RouteTarget:      fmt.Sprintf("http://127.0.0.1:%d", backendPort),
		RouteBackendPort: backendPort,
	}
}

func TestServiceCreatesAndPersistsSafeDefaults(t *testing.T) {
	repo := &repositoryStub{err: domain.ErrConfigNotFound}
	service := NewService(repo, nil, nil, " Arnold's Mac ")
	config, err := service.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if config.RemoteEnabled || !config.LANEnabled || config.LANPort != domain.DefaultLANPort ||
		config.TailscaleEnabled || config.TailscaleHTTPSPort != 443 ||
		config.TailscalePath != "/xiadown" || config.DeviceName != "Arnold's Mac" {
		t.Fatalf("unexpected defaults: %+v", config)
	}
	if len(repo.saves) != 1 {
		t.Fatalf("save count = %d, want 1", len(repo.saves))
	}
}

func TestServiceUpdateMergesAndValidates(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, false, true, false)}
	service := NewService(repo, nil, nil, "unused")
	remote := true
	port := 43123
	path := "/mobile"
	got, err := service.UpdateConfig(context.Background(), UpdateConfigRequest{
		RemoteEnabled: &remote, LANPort: &port, TailscalePath: &path,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !got.RemoteEnabled || !got.LANEnabled || got.LANPort != 43123 ||
		got.TailscaleEnabled || got.TailscalePath != "/mobile" {
		t.Fatalf("merged config: %+v", got)
	}
	bad := "/../admin"
	if _, err := service.UpdateConfig(context.Background(), UpdateConfigRequest{TailscalePath: &bad}); !errors.Is(err, domain.ErrInvalidConfig) {
		t.Fatalf("invalid update error = %v", err)
	}
	if len(repo.saves) != 1 {
		t.Fatalf("invalid config must not persist; saves=%d", len(repo.saves))
	}
}

func TestServiceApplyReconcilesRemoteGatedLANAndTailscale(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, true, true)}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{
		Installed: true, Connected: true, Version: "1.82", Tailnet: "Example",
		Device: "studio", ServeURL: "https://studio.example.ts.net/xiadown",
	}}
	lan := &lanStub{info: LANInfo{State: StateRunning, Port: 43123}}
	service := NewService(repo, tailscale, lan, "unused")
	status, err := service.Apply(context.Background(), 43123)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(tailscale.enableCalls) != 1 || tailscale.enableCalls[0] != (tailscaleEnableCall{
		localPort: 43123, httpsPort: 443, path: "/xiadown",
	}) {
		t.Fatalf("tailscale enables: %#v", tailscale.enableCalls)
	}
	if lan.enableCalls != 1 || lan.port != domain.DefaultLANPort || lan.deviceName != "Studio" {
		t.Fatalf("lan enable: calls=%d port=%d device=%q", lan.enableCalls, lan.port, lan.deviceName)
	}
	if !status.DesiredEnabled || !status.LAN.DesiredEnabled || status.LAN.State != StateRunning ||
		!status.Tailscale.DesiredEnabled || status.Tailscale.State != StateRunning ||
		status.Tailscale.Version != "1.82" || status.Tailscale.ServeURL == "" {
		t.Fatalf("status: %+v", status)
	}

	repo.config = testConfig(t, false, true, true)
	status, err = service.Apply(context.Background(), 43123)
	if err != nil {
		t.Fatalf("disable apply: %v", err)
	}
	if lan.disableCalls != 1 || len(tailscale.disableCalls) != 1 ||
		tailscale.disableCalls[0] != (tailscaleDisableCall{
			httpsPort: 443, path: "/xiadown",
			ownership: domain.TailscaleRouteOwnership{BackendPort: 43123},
		}) {
		t.Fatalf("disable calls: lan=%d tailscale=%#v", lan.disableCalls, tailscale.disableCalls)
	}
	if status.DesiredEnabled || status.LAN.DesiredEnabled || status.Tailscale.DesiredEnabled ||
		status.Tailscale.State != StateDisabled {
		t.Fatalf("disabled status: %+v", status)
	}
}

func TestServiceChangesRouteByDisablingOldExactEndpointBeforeEnablingNew(t *testing.T) {
	config := testConfig(t, true, false, true)
	config.TailscaleHTTPSPort = 8443
	config.TailscalePath = "/mobile"
	events := make([]string, 0, 6)
	repo := &repositoryStub{
		config:  config,
		managed: activeManagedRoute(443, "/xiadown"),
		events:  &events,
	}
	tailscale := &tailscaleStub{
		info:   exactRouteInfo(43123, "https://studio.example.ts.net/xiadown"),
		events: &events,
	}

	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	wantEvents := []string{
		"persist:disable:pending:443:/xiadown",
		"disable:443:/xiadown",
		"persist:disable:succeeded:443:/xiadown",
		"persist:enable:pending:8443:/mobile",
		"enable:8443:/mobile",
		"persist:enable:succeeded:8443:/mobile",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("route migration order = %#v, want %#v", events, wantEvents)
	}
	if status.Tailscale.State != StateRunning || !repo.managed.SameEndpoint(8443, "/mobile") ||
		repo.managed.State != domain.TailscaleRouteStateActive {
		t.Fatalf("new route status/state = %+v / %+v", status.Tailscale, repo.managed)
	}
}

func TestServiceDoesNotEnableNewRouteWhenOldExactDisableFails(t *testing.T) {
	config := testConfig(t, true, false, true)
	config.TailscaleHTTPSPort = 8443
	config.TailscalePath = "/mobile"
	repo := &repositoryStub{config: config, managed: activeManagedRoute(443, "/xiadown")}
	tailscale := &tailscaleStub{
		info:       exactRouteInfo(43123, "https://studio.example.ts.net/xiadown"),
		disableErr: errors.New("old route permission denied"),
	}

	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.disableCalls) != 1 || tailscale.disableCalls[0] != (tailscaleDisableCall{
		httpsPort: 443, path: "/xiadown",
		ownership: domain.TailscaleRouteOwnership{BackendPort: 43123},
	}) {
		t.Fatalf("old exact disable calls = %#v", tailscale.disableCalls)
	}
	if len(tailscale.enableCalls) != 0 {
		t.Fatalf("new route enabled after old cleanup failure: %#v", tailscale.enableCalls)
	}
	if status.Tailscale.State != StateError || status.Tailscale.LastError != "old route permission denied" ||
		!repo.managed.SameEndpoint(443, "/xiadown") || repo.managed.LastResult != domain.TailscaleRouteResultFailed {
		t.Fatalf("failed cleanup audit/status = %+v / %+v", status.Tailscale, repo.managed)
	}
}

func TestServiceApplyKeepsDesiredStateAndReportsRuntimeErrors(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, true, true)}
	tailscale := &tailscaleStub{
		info:      domain.TailscaleInfo{Installed: true, Connected: true},
		enableErr: errors.New("serve permission denied"),
	}
	lan := &lanStub{info: LANInfo{State: StateDisabled}, enableErr: errors.New("bind denied")}
	service := NewService(repo, tailscale, lan, "unused")
	status, err := service.Apply(context.Background(), 43123)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !status.DesiredEnabled || status.LAN.State != StateError || status.LAN.LastError != "bind denied" ||
		status.Tailscale.State != StateError || status.Tailscale.LastError != "serve permission denied" {
		t.Fatalf("error status: %+v", status)
	}
	if !repo.config.RemoteEnabled || !repo.config.LANEnabled || !repo.config.TailscaleEnabled {
		t.Fatalf("desired config was lost: %+v", repo.config)
	}
}

func TestServiceDoesNotEnableTailscaleWithoutIsolatedPublicPort(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, false, true)}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{Installed: true, Connected: true}}
	service := NewService(repo, tailscale, nil, "unused")
	status, err := service.Apply(context.Background(), 0)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(tailscale.enableCalls) != 0 {
		t.Fatalf("unsafe enable calls: %#v", tailscale.enableCalls)
	}
	if status.Tailscale.State != StateError || status.Tailscale.LastError != "isolated public API port unavailable" {
		t.Fatalf("status: %+v", status)
	}
}

func TestServiceDoesNotCallConnectedTailscaleRunningBeforeServeRouteIsConfirmed(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, false, true)}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{
		Installed: true, Connected: true, DNSName: "studio.example.ts.net",
	}}
	service := NewService(repo, tailscale, nil, "unused")
	status, err := service.GetStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Tailscale.State != StateStarting || status.Tailscale.ServeURL != "" {
		t.Fatalf("unconfirmed Serve route presented as running: %+v", status.Tailscale)
	}
}

func TestServiceStatusIgnoresMissingOptionalTailscaleCLIWhenDisabled(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, true, false)}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{Installed: false}}
	lan := &lanStub{info: LANInfo{State: StateRunning, Port: 43123}}

	status, err := NewService(repo, tailscale, lan, "unused").GetStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.DesiredEnabled || !status.LAN.DesiredEnabled || status.LAN.State != StateRunning ||
		status.LAN.LastError != "" {
		t.Fatalf("healthy LAN-only status = %+v", status)
	}
	if status.Tailscale.DesiredEnabled || status.Tailscale.Installed || status.Tailscale.LastError != "" {
		t.Fatalf("disabled missing Tailscale status = %+v", status.Tailscale)
	}
}

func TestServiceStatusPresentsEnabledMissingTailscaleAsUnavailableWithoutPATHError(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, false, true)}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{Installed: false}}

	status, err := NewService(repo, tailscale, nil, "unused").GetStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Tailscale.DesiredEnabled || status.Tailscale.Installed ||
		status.Tailscale.State != StateUnavailable || status.Tailscale.LastError != "" {
		t.Fatalf("enabled missing Tailscale status = %+v", status.Tailscale)
	}
	if strings.Contains(strings.ToLower(status.Tailscale.LastError), "%path%") {
		t.Fatalf("raw executable lookup error leaked: %+v", status.Tailscale)
	}
}

func TestServiceRemoteOffAfterRestartRemovesPersistedOldExactRoute(t *testing.T) {
	config := testConfig(t, false, true, true)
	config.TailscaleHTTPSPort = 8443
	config.TailscalePath = "/new-config"
	repo := &repositoryStub{
		config:  config,
		managed: activeManagedRoute(443, "/old-owned"),
	}
	tailscale := &tailscaleStub{info: exactRouteInfo(43123, "https://studio.example.ts.net/old-owned")}
	service := NewService(repo, tailscale, nil, "unused")
	if _, err := service.Apply(context.Background(), 43123); err != nil {
		t.Fatal(err)
	}
	if len(tailscale.disableCalls) != 1 || tailscale.disableCalls[0] != (tailscaleDisableCall{
		httpsPort: 443, path: "/old-owned",
		ownership: domain.TailscaleRouteOwnership{BackendPort: 43123},
	}) {
		t.Fatalf("persisted old exact route was not removed: %#v", tailscale.disableCalls)
	}
	if repo.managed.State != domain.TailscaleRouteStateInactive || repo.managed.LastResult != domain.TailscaleRouteResultSucceeded {
		t.Fatalf("disable was not audited: %+v", repo.managed)
	}

	// A fresh service sees the durable inactive state and does not issue a
	// second off command, even if inspection happens to report a URL.
	tailscale.disableCalls = nil
	service = NewService(repo, tailscale, nil, "unused")
	if _, err := service.Apply(context.Background(), 43123); err != nil {
		t.Fatal(err)
	}
	if len(tailscale.disableCalls) != 0 {
		t.Fatalf("inactive persisted route invoked disable: %#v", tailscale.disableCalls)
	}
}

func TestServiceRemoteOffNeverAdoptsOrDisablesAnUnownedMatchingRoute(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, false, true, true)}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{
		Installed: true, Connected: true, DNSName: "studio.example.ts.net",
		ServeURL: "https://studio.example.ts.net/xiadown",
	}}
	if _, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123); err != nil {
		t.Fatal(err)
	}
	if len(tailscale.disableCalls) != 0 {
		t.Fatalf("unowned matching route was disabled: %#v", tailscale.disableCalls)
	}
	if len(repo.transitions) != 0 {
		t.Fatalf("unowned route was adopted: %#v", repo.transitions)
	}
}

func TestServiceEnableFailureIsDurablyClaimedAndRemoteOffCleansItAfterRestart(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, false, true)}
	tailscale := &tailscaleStub{
		info:      domain.TailscaleInfo{Installed: true, Connected: true},
		enableErr: errors.New("serve command failed after dispatch"),
	}
	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if status.Tailscale.State != StateError || repo.managed.State != domain.TailscaleRouteStateError ||
		!repo.managed.SameEndpoint(443, "/xiadown") {
		t.Fatalf("failed enable ownership/status = %+v / %+v", status.Tailscale, repo.managed)
	}

	repo.config = testConfig(t, false, false, true)
	tailscale.enableErr = nil
	status, err = NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.disableCalls) != 1 || tailscale.disableCalls[0] != (tailscaleDisableCall{
		httpsPort: 443, path: "/xiadown",
		ownership: domain.TailscaleRouteOwnership{BackendPort: 43123},
	}) {
		t.Fatalf("restart cleanup calls = %#v", tailscale.disableCalls)
	}
	if status.Tailscale.State != StateDisabled || repo.managed.State != domain.TailscaleRouteStateInactive {
		t.Fatalf("restart cleanup status/state = %+v / %+v", status.Tailscale, repo.managed)
	}
}

func TestServiceNeverMutatesServeBeforeOwnershipIntentPersists(t *testing.T) {
	repo := &repositoryStub{
		config:         testConfig(t, true, false, true),
		transitionErrs: []error{errors.New("database read-only")},
	}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{Installed: true, Connected: true}}
	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.enableCalls) != 0 || len(tailscale.disableCalls) != 0 {
		t.Fatalf("Serve mutated without durable ownership: enable=%#v disable=%#v", tailscale.enableCalls, tailscale.disableCalls)
	}
	if status.Tailscale.State != StateError || status.Tailscale.LastError != "persist XiaDown Tailscale route before enable: database read-only" {
		t.Fatalf("persistence failure status = %+v", status.Tailscale)
	}
}

func TestServiceRefusesInitiallyOccupiedRouteAndReleasesFailedClaim(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, false, true)}
	tailscale := &tailscaleStub{info: exactRouteInfo(49999, "https://studio.example.ts.net/xiadown")}

	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.enableCalls) != 0 || len(tailscale.disableCalls) != 0 {
		t.Fatalf("occupied route was mutated: enable=%#v disable=%#v", tailscale.enableCalls, tailscale.disableCalls)
	}
	if len(repo.transitions) != 3 ||
		repo.transitions[0].Action != domain.TailscaleRouteActionEnable ||
		repo.transitions[0].Result != domain.TailscaleRouteResultPending ||
		repo.transitions[1].Action != domain.TailscaleRouteActionEnable ||
		repo.transitions[1].Result != domain.TailscaleRouteResultFailed ||
		repo.transitions[2].Action != domain.TailscaleRouteActionRelease ||
		repo.transitions[2].Result != domain.TailscaleRouteResultSucceeded {
		t.Fatalf("occupation audit = %#v", repo.transitions)
	}
	if repo.managed.State != domain.TailscaleRouteStateInactive || repo.managed.Claimed() ||
		status.Tailscale.State != StateError || status.Tailscale.ServeURL != "" ||
		!strings.Contains(status.Tailscale.LastError, domain.ErrTailscaleRouteOwnershipConflict.Error()) {
		t.Fatalf("occupation status/state = %+v / %+v", status.Tailscale, repo.managed)
	}
}

func TestServiceStatusNeverAttributesUnclaimedCandidateRouteToXiaDown(t *testing.T) {
	repo := &repositoryStub{config: testConfig(t, true, false, true)}
	tailscale := &tailscaleStub{info: exactRouteInfo(49999, "https://studio.example.ts.net/xiadown")}

	status, err := NewService(repo, tailscale, nil, "unused").GetStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Tailscale.State == StateRunning || status.Tailscale.ServeURL != "" ||
		status.Tailscale.State != StateStarting {
		t.Fatalf("unclaimed candidate was presented as XiaDown: %+v", status.Tailscale)
	}
}

func TestServiceStatusRejectsClaimedRouteTargetMismatch(t *testing.T) {
	repo := &repositoryStub{
		config:  testConfig(t, true, false, true),
		managed: activeManagedRoute(443, "/xiadown"),
	}
	tailscale := &tailscaleStub{info: exactRouteInfo(49999, "https://studio.example.ts.net/xiadown")}

	status, err := NewService(repo, tailscale, nil, "unused").GetStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Tailscale.State != StateError || status.Tailscale.ServeURL != "" ||
		status.Tailscale.LastError != "XiaDown Tailscale route was changed outside XiaDown" {
		t.Fatalf("mismatched claim status = %+v", status.Tailscale)
	}
}

func TestServiceExternalRewriteIsAuditedAndReleasedWithoutMutation(t *testing.T) {
	repo := &repositoryStub{
		config:  testConfig(t, false, false, true),
		managed: activeManagedRoute(443, "/xiadown"),
	}
	tailscale := &tailscaleStub{info: exactRouteInfo(49999, "https://studio.example.ts.net/xiadown")}

	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.disableCalls) != 0 || len(tailscale.enableCalls) != 0 {
		t.Fatalf("externally rewritten route was mutated: enable=%#v disable=%#v", tailscale.enableCalls, tailscale.disableCalls)
	}
	if len(repo.transitions) != 3 ||
		repo.transitions[0].Action != domain.TailscaleRouteActionDisable ||
		repo.transitions[0].Result != domain.TailscaleRouteResultPending ||
		repo.transitions[1].Action != domain.TailscaleRouteActionDisable ||
		repo.transitions[1].Result != domain.TailscaleRouteResultFailed ||
		repo.transitions[2].Action != domain.TailscaleRouteActionRelease || repo.managed.Claimed() {
		t.Fatalf("external rewrite audit/state = %#v / %+v", repo.transitions, repo.managed)
	}
	if status.Tailscale.State != StateError || status.Tailscale.ServeURL != "" {
		t.Fatalf("external rewrite status = %+v", status.Tailscale)
	}

	// The release is durable: a fresh service no longer retries a destructive
	// cleanup against the externally-owned route.
	status, err = NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.transitions) != 3 || len(tailscale.disableCalls) != 0 ||
		status.Tailscale.State != StateDisabled || status.Tailscale.ServeURL != "" {
		t.Fatalf("released ownership retried or leaked status: %+v / %#v", status.Tailscale, repo.transitions)
	}
}

func TestServiceRestartReconcilesPersistedPendingOwnedBackend(t *testing.T) {
	repo := &repositoryStub{
		config: testConfig(t, true, false, true),
		managed: domain.ManagedTailscaleRoute{
			HTTPSPort:          443,
			Path:               "/xiadown",
			BackendPort:        43000,
			PendingBackendPort: 43123,
			State:              domain.TailscaleRouteStateEnabling,
			LastAction:         domain.TailscaleRouteActionEnable,
			LastResult:         domain.TailscaleRouteResultPending,
		},
	}
	tailscale := &tailscaleStub{info: exactRouteInfo(43123, "https://studio.example.ts.net/xiadown")}

	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 44000)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.enableCalls) != 1 || tailscale.enableCalls[0] != (tailscaleEnableCall{
		localPort: 44000, httpsPort: 443, path: "/xiadown",
		ownership: domain.TailscaleRouteOwnership{BackendPort: 43123, PendingBackendPort: 43000},
	}) {
		t.Fatalf("restart ownership call = %#v", tailscale.enableCalls)
	}
	if repo.managed.State != domain.TailscaleRouteStateActive || repo.managed.BackendPort != 44000 ||
		repo.managed.PendingBackendPort != 0 || status.Tailscale.State != StateRunning {
		t.Fatalf("restart reconciliation = %+v / %+v", repo.managed, status.Tailscale)
	}
}

func TestServiceMissingClaimedRoutePreservesBackendEvidenceForManagerRecheck(t *testing.T) {
	repo := &repositoryStub{
		config:  testConfig(t, true, false, true),
		managed: activeManagedRoute(443, "/xiadown"),
	}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{
		Installed: true, Connected: true, RouteChecked: true,
	}}

	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 44000)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.enableCalls) != 1 || tailscale.enableCalls[0].ownership != (domain.TailscaleRouteOwnership{BackendPort: 43123}) {
		t.Fatalf("missing route lost persisted ownership evidence: %#v", tailscale.enableCalls)
	}
	if len(repo.transitions) != 2 || repo.transitions[0].BackendPort != 43123 ||
		repo.transitions[0].PendingBackendPort != 44000 || repo.managed.BackendPort != 44000 ||
		status.Tailscale.State != StateRunning {
		t.Fatalf("missing route reconciliation = %#v / %+v / %+v", repo.transitions, repo.managed, status.Tailscale)
	}
}

func TestServiceReleasedRouteNeverReusesStaleBackendOwnership(t *testing.T) {
	repo := &repositoryStub{
		config: testConfig(t, true, false, true),
		managed: domain.ManagedTailscaleRoute{
			HTTPSPort:   443,
			Path:        "/xiadown",
			BackendPort: 43123,
			State:       domain.TailscaleRouteStateInactive,
			LastAction:  domain.TailscaleRouteActionRelease,
			LastResult:  domain.TailscaleRouteResultSucceeded,
			LastError:   "external rewrite",
		},
	}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{
		Installed: true, Connected: true, RouteChecked: true,
	}}

	if _, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 44000); err != nil {
		t.Fatal(err)
	}
	if len(tailscale.enableCalls) != 1 || tailscale.enableCalls[0].ownership != (domain.TailscaleRouteOwnership{}) {
		t.Fatalf("released route reused stale ownership: %#v", tailscale.enableCalls)
	}
	if repo.transitions[0].BackendPort != 0 || repo.transitions[0].PendingBackendPort != 44000 {
		t.Fatalf("released backend leaked into new claim: %+v", repo.transitions[0])
	}
}

func TestServiceLegacyUnknownClaimAdoptsOneObservedLoopbackTarget(t *testing.T) {
	repo := &repositoryStub{
		config: testConfig(t, true, false, true),
		managed: domain.ManagedTailscaleRoute{
			HTTPSPort:  443,
			Path:       "/xiadown",
			State:      domain.TailscaleRouteStateUnknown,
			LastAction: domain.TailscaleRouteActionAdopt,
			LastResult: domain.TailscaleRouteResultPending,
		},
	}
	tailscale := &tailscaleStub{info: exactRouteInfo(43000, "https://studio.example.ts.net/xiadown")}

	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.enableCalls) != 1 || tailscale.enableCalls[0].ownership != (domain.TailscaleRouteOwnership{BackendPort: 43000}) {
		t.Fatalf("legacy adoption call = %#v", tailscale.enableCalls)
	}
	if repo.managed.State != domain.TailscaleRouteStateActive || repo.managed.BackendPort != 43123 ||
		repo.managed.PendingBackendPort != 0 || status.Tailscale.State != StateRunning {
		t.Fatalf("legacy adoption result = %+v / %+v", repo.managed, status.Tailscale)
	}
}

func TestServiceLegacyUnknownClaimCanDisableOnlyObservedLoopbackTarget(t *testing.T) {
	repo := &repositoryStub{
		config: testConfig(t, false, false, true),
		managed: domain.ManagedTailscaleRoute{
			HTTPSPort:  443,
			Path:       "/xiadown",
			State:      domain.TailscaleRouteStateUnknown,
			LastAction: domain.TailscaleRouteActionAdopt,
			LastResult: domain.TailscaleRouteResultPending,
		},
	}
	tailscale := &tailscaleStub{info: exactRouteInfo(43000, "https://studio.example.ts.net/xiadown")}

	status, err := NewService(repo, tailscale, nil, "unused").Apply(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if len(tailscale.disableCalls) != 1 || tailscale.disableCalls[0].ownership != (domain.TailscaleRouteOwnership{BackendPort: 43000}) {
		t.Fatalf("legacy disable call = %#v", tailscale.disableCalls)
	}
	if repo.managed.State != domain.TailscaleRouteStateInactive || repo.managed.BackendPort != 43000 ||
		status.Tailscale.State != StateDisabled {
		t.Fatalf("legacy disable result = %+v / %+v", repo.managed, status.Tailscale)
	}
}
