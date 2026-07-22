package libraryaccess

import (
	"errors"
	"testing"
)

func TestNewTailscaleRouteTransitionValidatesLifecycleCombinations(t *testing.T) {
	valid := []struct {
		backend int
		pending int
		state   TailscaleRouteState
		action  TailscaleRouteAction
		result  TailscaleRouteResult
		err     string
	}{
		{0, 0, TailscaleRouteStateUnknown, TailscaleRouteActionAdopt, TailscaleRouteResultPending, ""},
		{0, 43123, TailscaleRouteStateEnabling, TailscaleRouteActionEnable, TailscaleRouteResultPending, ""},
		{43123, 0, TailscaleRouteStateActive, TailscaleRouteActionEnable, TailscaleRouteResultSucceeded, ""},
		{0, 43123, TailscaleRouteStateError, TailscaleRouteActionEnable, TailscaleRouteResultFailed, "enable failed"},
		{43123, 0, TailscaleRouteStateDisabling, TailscaleRouteActionDisable, TailscaleRouteResultPending, ""},
		{43123, 0, TailscaleRouteStateInactive, TailscaleRouteActionDisable, TailscaleRouteResultSucceeded, ""},
		{43123, 0, TailscaleRouteStateError, TailscaleRouteActionDisable, TailscaleRouteResultFailed, "disable failed"},
		{43123, 0, TailscaleRouteStateInactive, TailscaleRouteActionRelease, TailscaleRouteResultSucceeded, "external rewrite"},
	}
	for _, candidate := range valid {
		if _, err := NewTailscaleRouteTransition(
			443, "/xiadown", candidate.backend, candidate.pending,
			candidate.state, candidate.action, candidate.result, candidate.err,
		); err != nil {
			t.Fatalf("valid transition (%s, %s, %s) failed: %v", candidate.state, candidate.action, candidate.result, err)
		}
	}

	invalid := []TailscaleRouteTransition{
		{HTTPSPort: 443, Path: "/xiadown", BackendPort: 43123, State: TailscaleRouteStateActive, Action: TailscaleRouteActionEnable, Result: TailscaleRouteResultPending},
		{HTTPSPort: 443, Path: "/xiadown", State: TailscaleRouteStateError, Action: TailscaleRouteActionDisable, Result: TailscaleRouteResultFailed},
		{HTTPSPort: 443, Path: "/xiadown", BackendPort: 43123, State: TailscaleRouteStateInactive, Action: TailscaleRouteActionDisable, Result: TailscaleRouteResultSucceeded, Error: "unexpected"},
		{HTTPSPort: 0, Path: "/xiadown", PendingBackendPort: 43123, State: TailscaleRouteStateEnabling, Action: TailscaleRouteActionEnable, Result: TailscaleRouteResultPending},
		{HTTPSPort: 443, Path: "/../admin", PendingBackendPort: 43123, State: TailscaleRouteStateEnabling, Action: TailscaleRouteActionEnable, Result: TailscaleRouteResultPending},
	}
	for _, candidate := range invalid {
		if _, err := NewTailscaleRouteTransition(
			candidate.HTTPSPort, candidate.Path, candidate.BackendPort, candidate.PendingBackendPort, candidate.State,
			candidate.Action, candidate.Result, candidate.Error,
		); !errors.Is(err, ErrInvalidTailscaleTransition) {
			t.Fatalf("invalid transition %+v error = %v", candidate, err)
		}
	}
}

func TestManagedTailscaleRouteClaimIsExplicit(t *testing.T) {
	if (ManagedTailscaleRoute{}).Claimed() {
		t.Fatal("zero-value route must not be treated as owned")
	}
	if (ManagedTailscaleRoute{State: TailscaleRouteStateInactive}).Claimed() {
		t.Fatal("inactive route must not be treated as owned")
	}
	if !(ManagedTailscaleRoute{State: TailscaleRouteStateUnknown}).Claimed() {
		t.Fatal("unknown migrated route must remain conservatively claimed")
	}
}
