package libraryaccess

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrManagedTailscaleRouteNotFound = errors.New("managed tailscale route not found")
	ErrInvalidTailscaleTransition    = errors.New("invalid managed tailscale route transition")
)

type TailscaleRouteState string

const (
	TailscaleRouteStateInactive  TailscaleRouteState = "inactive"
	TailscaleRouteStateUnknown   TailscaleRouteState = "unknown"
	TailscaleRouteStateEnabling  TailscaleRouteState = "enabling"
	TailscaleRouteStateActive    TailscaleRouteState = "active"
	TailscaleRouteStateDisabling TailscaleRouteState = "disabling"
	TailscaleRouteStateError     TailscaleRouteState = "error"
)

type TailscaleRouteAction string

const (
	TailscaleRouteActionAdopt   TailscaleRouteAction = "adopt"
	TailscaleRouteActionEnable  TailscaleRouteAction = "enable"
	TailscaleRouteActionDisable TailscaleRouteAction = "disable"
	TailscaleRouteActionRelease TailscaleRouteAction = "release"
)

type TailscaleRouteResult string

const (
	TailscaleRouteResultPending   TailscaleRouteResult = "pending"
	TailscaleRouteResultSucceeded TailscaleRouteResult = "succeeded"
	TailscaleRouteResultFailed    TailscaleRouteResult = "failed"
)

// ManagedTailscaleRoute is XiaDown's persisted ownership record for one exact
// Tailscale Serve route. A non-inactive state is intentionally conservative:
// XiaDown must reconcile that exact port/path before adopting another route.
type ManagedTailscaleRoute struct {
	HTTPSPort          int
	Path               string
	BackendPort        int
	PendingBackendPort int
	State              TailscaleRouteState
	LastAction         TailscaleRouteAction
	LastResult         TailscaleRouteResult
	LastError          string
	Revision           int64
	UpdatedAt          time.Time
}

func (route ManagedTailscaleRoute) Claimed() bool {
	return route.State != "" && route.State != TailscaleRouteStateInactive
}

func (route ManagedTailscaleRoute) SameEndpoint(httpsPort int, routePath string) bool {
	return route.HTTPSPort == httpsPort && route.Path == strings.TrimSpace(routePath)
}

func (route ManagedTailscaleRoute) Ownership() TailscaleRouteOwnership {
	return TailscaleRouteOwnership{
		BackendPort:        route.BackendPort,
		PendingBackendPort: route.PendingBackendPort,
	}
}

// TailscaleRouteTransition is persisted atomically to both the current state
// row and the append-only audit ledger before/after each external Serve call.
type TailscaleRouteTransition struct {
	HTTPSPort          int
	Path               string
	BackendPort        int
	PendingBackendPort int
	State              TailscaleRouteState
	Action             TailscaleRouteAction
	Result             TailscaleRouteResult
	Error              string
}

func NewTailscaleRouteTransition(
	httpsPort int,
	routePath string,
	backendPort int,
	pendingBackendPort int,
	state TailscaleRouteState,
	action TailscaleRouteAction,
	result TailscaleRouteResult,
	errorMessage string,
) (TailscaleRouteTransition, error) {
	routePath = strings.TrimSpace(routePath)
	errorMessage = strings.TrimSpace(errorMessage)
	if httpsPort < 1 || httpsPort > 65535 || !validTailscalePath(routePath) ||
		backendPort < 0 || backendPort > 65535 || pendingBackendPort < 0 || pendingBackendPort > 65535 ||
		!validTailscaleTransition(backendPort, pendingBackendPort, state, action, result, errorMessage) {
		return TailscaleRouteTransition{}, ErrInvalidTailscaleTransition
	}
	return TailscaleRouteTransition{
		HTTPSPort:          httpsPort,
		Path:               routePath,
		BackendPort:        backendPort,
		PendingBackendPort: pendingBackendPort,
		State:              state,
		Action:             action,
		Result:             result,
		Error:              errorMessage,
	}, nil
}

func validTailscaleTransition(
	backendPort int,
	pendingBackendPort int,
	state TailscaleRouteState,
	action TailscaleRouteAction,
	result TailscaleRouteResult,
	errorMessage string,
) bool {
	if action == TailscaleRouteActionRelease {
		return state == TailscaleRouteStateInactive && result == TailscaleRouteResultSucceeded &&
			pendingBackendPort == 0 && errorMessage != ""
	}
	if result == TailscaleRouteResultFailed {
		if state != TailscaleRouteStateError || errorMessage == "" {
			return false
		}
	} else if errorMessage != "" {
		return false
	}

	switch action {
	case TailscaleRouteActionAdopt:
		return backendPort == 0 && pendingBackendPort == 0 &&
			state == TailscaleRouteStateUnknown && result == TailscaleRouteResultPending
	case TailscaleRouteActionEnable:
		return (pendingBackendPort > 0 && state == TailscaleRouteStateEnabling && result == TailscaleRouteResultPending) ||
			(backendPort > 0 && pendingBackendPort == 0 && state == TailscaleRouteStateActive && result == TailscaleRouteResultSucceeded) ||
			(pendingBackendPort > 0 && state == TailscaleRouteStateError && result == TailscaleRouteResultFailed)
	case TailscaleRouteActionDisable:
		return backendPort > 0 && pendingBackendPort == 0 &&
			((state == TailscaleRouteStateDisabling && result == TailscaleRouteResultPending) ||
				(state == TailscaleRouteStateInactive && result == TailscaleRouteResultSucceeded) ||
				(state == TailscaleRouteStateError && result == TailscaleRouteResultFailed))
	default:
		return false
	}
}
