package libraryaccess

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	domain "xiadown/internal/domain/libraryaccess"
)

func TestAutoReconcileNeededClassifiesRecoverableState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "remote off", status: Status{}, want: false},
		{name: "healthy", status: Status{DesiredEnabled: true,
			LAN:       LANStatus{DesiredEnabled: true, State: StateRunning},
			Tailscale: TailscaleStatus{DesiredEnabled: true, State: StateRunning}}, want: false},
		{name: "offline boot", status: Status{DesiredEnabled: true,
			LAN: LANStatus{DesiredEnabled: true, State: StateError, LastError: "no eligible private LAN address"}}, want: true},
		{name: "tailscale disconnected", status: Status{DesiredEnabled: true,
			Tailscale: TailscaleStatus{DesiredEnabled: true, State: StateDisconnected}}, want: true},
		{name: "firewall approval", status: Status{DesiredEnabled: true,
			LAN: LANStatus{DesiredEnabled: true, State: StateError, LastError: "administrator approval is required for firewall"}}, want: false},
		{name: "route ownership conflict", status: Status{DesiredEnabled: true,
			Tailscale: TailscaleStatus{DesiredEnabled: true, State: StateError, LastError: "route was changed outside XiaDown"}}, want: false},
		{name: "tailnet Serve authorization", status: Status{DesiredEnabled: true,
			Tailscale: TailscaleStatus{DesiredEnabled: true, State: StateError,
				LastError: "Tailscale command timed out; output: Serve is not enabled on your tailnet"}}, want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := autoReconcileNeeded(test.status); got != test.want {
				t.Fatalf("autoReconcileNeeded() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestReconcileObservationSeesLiveStateBehindCachedError(t *testing.T) {
	t.Parallel()
	before := Status{
		DesiredEnabled:         true,
		Tailscale:              TailscaleStatus{DesiredEnabled: true, State: StateError, LastError: "Tailscale is disconnected"},
		observedTailscaleState: StateDisconnected,
	}
	after := before
	after.observedTailscaleState = StateStarting
	if reconcileObservation(before) == reconcileObservation(after) {
		t.Fatal("live Tailscale transition was hidden by cached Apply error")
	}
}

func TestReconcilerNetworkChangesNeverRetryManualTransportErrors(t *testing.T) {
	tests := []struct {
		name    string
		service func(*testing.T) (*Service, *lanStub, *tailscaleStub)
	}{
		{
			name: "firewall elevation",
			service: func(t *testing.T) (*Service, *lanStub, *tailscaleStub) {
				lan := &lanStub{info: LANInfo{State: StateError, LastError: "administrator approval is required for firewall"}}
				repo := &repositoryStub{config: testConfig(t, true, true, false)}
				return NewService(repo, nil, lan, "Studio"), lan, nil
			},
		},
		{
			name: "Tailscale ownership conflict",
			service: func(t *testing.T) (*Service, *lanStub, *tailscaleStub) {
				tailscale := &tailscaleStub{info: domain.TailscaleInfo{
					Installed: true, Connected: true, RouteChecked: true, RouteExists: true,
					RouteBackendPort: 49999, RouteTarget: "http://127.0.0.1:49999",
				}}
				repo := &repositoryStub{
					config:  testConfig(t, true, false, true),
					managed: activeManagedRoute(443, "/xiadown"),
				}
				return NewService(repo, tailscale, nil, "Studio"), nil, tailscale
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, lan, tailscale := test.service(t)
			ctx, cancel := context.WithCancel(context.Background())
			var revision atomic.Int64
			var applyResults atomic.Int64
			done := make(chan struct{})
			go func() {
				service.RunReconciler(ctx, func() int { return 43123 }, ReconcilerOptions{
					PollInterval: 2 * time.Millisecond, MaximumBackoff: 4 * time.Millisecond,
					AttemptTimeout: 50 * time.Millisecond,
					NetworkRevision: func() string {
						return time.Unix(0, revision.Add(1)).String()
					},
					OnResult: func(Status, error) { applyResults.Add(1) },
				})
				close(done)
			}()
			time.Sleep(20 * time.Millisecond)
			cancel()
			<-done
			if applyResults.Load() != 0 {
				t.Fatalf("network churn invoked Apply %d times", applyResults.Load())
			}
			if lan != nil && lan.enableCalls != 0 {
				t.Fatalf("network churn retried firewall elevation %d times", lan.enableCalls)
			}
			if tailscale != nil && (len(tailscale.enableCalls) != 0 || len(tailscale.disableCalls) != 0) {
				t.Fatalf("network churn mutated conflicted Tailscale route: enable=%d disable=%d", len(tailscale.enableCalls), len(tailscale.disableCalls))
			}
		})
	}
}

func TestAutomaticTailscaleHealingSkipsManualFirewallRetry(t *testing.T) {
	lan := &lanStub{info: LANInfo{State: StateError, Port: 8443, LastError: "firewall elevation approval required"}}
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{Installed: true, Connected: true, RouteChecked: true}}
	repo := &repositoryStub{config: testConfig(t, true, true, true)}
	service := NewService(repo, tailscale, lan, "Studio")
	status, err := service.applyAutomatically(context.Background(), 43123)
	if err != nil {
		t.Fatal(err)
	}
	if lan.enableCalls != 0 {
		t.Fatalf("Tailscale healing retried firewall elevation %d times", lan.enableCalls)
	}
	if len(tailscale.enableCalls) != 1 {
		t.Fatalf("Tailscale was not independently healed: %#v", tailscale.enableCalls)
	}
	if status.LAN.State != StateError || status.Tailscale.State != StateRunning {
		t.Fatalf("component-aware status = %#v", status)
	}
}

func TestReconcilerPerformsInitialAutomaticAttemptImmediately(t *testing.T) {
	tailscale := &tailscaleStub{info: domain.TailscaleInfo{
		Installed: true, Connected: true, RouteChecked: true,
	}}
	repo := &repositoryStub{config: testConfig(t, true, false, true)}
	service := NewService(repo, tailscale, nil, "Studio")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	result := make(chan Status, 1)
	go func() {
		service.RunReconciler(ctx, func() int { return 43123 }, ReconcilerOptions{
			InitialReconcile: true,
			PollInterval:     time.Hour,
			AttemptTimeout:   time.Second,
			OnResult: func(status Status, err error) {
				if err != nil {
					t.Errorf("initial reconcile: %v", err)
				}
				result <- status
				cancel()
			},
		})
		close(done)
	}()
	select {
	case status := <-result:
		if status.Tailscale.State != StateRunning {
			t.Fatalf("initial status = %#v", status)
		}
	case <-time.After(time.Second):
		cancel()
		t.Fatal("initial reconcile waited for the poll interval")
	}
	<-done
	if len(tailscale.enableCalls) != 1 {
		t.Fatalf("initial enable calls = %#v", tailscale.enableCalls)
	}
}

func TestReconcilerInitialAttemptCleansUpPersistedDisabledRoute(t *testing.T) {
	repo := &repositoryStub{
		config:  testConfig(t, false, false, false),
		managed: activeManagedRoute(443, "/xiadown"),
	}
	tailscale := &tailscaleStub{info: exactRouteInfo(43123, "https://studio.example.ts.net/xiadown")}
	service := NewService(repo, tailscale, nil, "Studio")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	result := make(chan Status, 1)
	go func() {
		service.RunReconciler(ctx, func() int { return 44000 }, ReconcilerOptions{
			InitialReconcile: true,
			PollInterval:     time.Hour,
			AttemptTimeout:   time.Second,
			OnResult: func(status Status, _ error) {
				result <- status
				cancel()
			},
		})
		close(done)
	}()
	select {
	case status := <-result:
		if status.Tailscale.State != StateDisabled {
			t.Fatalf("disabled initial status = %#v", status)
		}
	case <-time.After(time.Second):
		cancel()
		t.Fatal("initial cleanup waited for the poll interval")
	}
	<-done
	if len(tailscale.disableCalls) != 1 || tailscale.disableCalls[0].ownership != (domain.TailscaleRouteOwnership{BackendPort: 43123}) {
		t.Fatalf("initial cleanup calls = %#v", tailscale.disableCalls)
	}
}
