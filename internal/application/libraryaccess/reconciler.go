package libraryaccess

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ReconcilerOptions struct {
	PollInterval     time.Duration
	MaximumBackoff   time.Duration
	AttemptTimeout   time.Duration
	InitialReconcile bool
	NetworkRevision  func() string
	OnResult         func(Status, error)
}

// RunReconciler watches observed LAN/Tailscale health until ctx is cancelled.
// It reconciles immediately after a relevant state or interface change and
// uses bounded exponential retries for transient failures. Ownership conflicts
// and firewall-elevation errors remain manual so background healing cannot
// overwrite another Tailscale route or repeatedly prompt for administrator
// approval.
func (service *Service) RunReconciler(
	ctx context.Context,
	localPortProvider func() int,
	options ReconcilerOptions,
) {
	if service == nil || localPortProvider == nil {
		return
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 15 * time.Second
	}
	if options.MaximumBackoff < options.PollInterval {
		options.MaximumBackoff = 2 * time.Minute
	}
	if options.AttemptTimeout <= 0 {
		options.AttemptTimeout = 20 * time.Second
	}
	networkRevision := readNetworkRevision(options.NetworkRevision)
	initialCtx, initialCancel := context.WithTimeout(ctx, options.AttemptTimeout)
	var lastStatus Status
	var initialErr error
	if options.InitialReconcile {
		lastStatus, initialErr = service.applyAutomatically(initialCtx, localPortProvider())
	} else {
		lastStatus, initialErr = service.GetStatus(initialCtx)
	}
	initialCancel()
	if options.InitialReconcile && options.OnResult != nil {
		options.OnResult(lastStatus, initialErr)
	}
	lastObservation := reconcileObservation(lastStatus)
	nextAttempt := time.Time{}
	backoff := options.PollInterval

	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			attemptCtx, cancel := context.WithTimeout(ctx, options.AttemptTimeout)
			status, statusErr := service.GetStatus(attemptCtx)
			cancel()
			if statusErr != nil {
				if options.OnResult != nil {
					options.OnResult(Status{}, statusErr)
				}
				continue
			}

			currentNetworkRevision := readNetworkRevision(options.NetworkRevision)
			observation := reconcileObservation(status)
			networkChanged := currentNetworkRevision != networkRevision
			observationChanged := observation != lastObservation
			retryable := autoReconcileNeeded(status)
			shouldApply := (networkChanged && networkReconcileNeeded(status)) ||
				(retryable && (observationChanged || nextAttempt.IsZero() || !now.Before(nextAttempt)))
			if shouldApply {
				applyCtx, applyCancel := context.WithTimeout(ctx, options.AttemptTimeout)
				status, statusErr = service.applyAutomatically(applyCtx, localPortProvider())
				applyCancel()
				if options.OnResult != nil {
					options.OnResult(status, statusErr)
				}
				if statusErr != nil || autoReconcileNeeded(status) {
					nextAttempt = now.Add(backoff)
					backoff *= 2
					if backoff > options.MaximumBackoff {
						backoff = options.MaximumBackoff
					}
				} else {
					nextAttempt = time.Time{}
					backoff = options.PollInterval
				}
				observation = reconcileObservation(status)
			}
			networkRevision = currentNetworkRevision
			lastObservation = observation
		}
	}
}

func readNetworkRevision(provider func() string) string {
	if provider == nil {
		return ""
	}
	return strings.TrimSpace(provider())
}

func reconcileObservation(status Status) string {
	return fmt.Sprintf("%t|%t|%s|%s|%s|%t|%s|%s|%s|%s|%s",
		status.DesiredEnabled,
		status.LAN.DesiredEnabled,
		status.LAN.State,
		status.observedLANState,
		status.LAN.LastError,
		status.Tailscale.DesiredEnabled,
		status.Tailscale.State,
		status.observedTailscaleState,
		status.Tailscale.LastError,
		status.Tailscale.Device,
		status.Tailscale.Tailnet,
	)
}

func autoReconcileNeeded(status Status) bool {
	if !status.DesiredEnabled {
		return false
	}
	if status.LAN.DesiredEnabled && status.LAN.State != StateRunning && retryableLANStatus(status.LAN) {
		return true
	}
	return status.Tailscale.DesiredEnabled && status.Tailscale.State != StateRunning && retryableTailscaleStatus(status.Tailscale)
}

func retryableLANStatus(status LANStatus) bool {
	if requiresManualLANIntervention(status.LastError) {
		return false
	}
	return status.State != StateDisabled
}

func networkReconcileNeeded(status Status) bool {
	// A physical-interface revision only affects the LAN listener. Tailscale
	// route ownership and connectivity are reconciled from their own observed
	// state; they must not be retried merely because Wi-Fi changed.
	return status.DesiredEnabled && status.LAN.DesiredEnabled && retryableLANStatus(status.LAN)
}

func requiresManualLANIntervention(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "administrator") || strings.Contains(message, "approval") ||
		strings.Contains(message, "elevation") || strings.Contains(message, "firewall")
}

func retryableTailscaleStatus(status TailscaleStatus) bool {
	if requiresManualTailscaleIntervention(status.LastError) {
		return false
	}
	return status.State != StateDisabled
}

func requiresManualTailscaleIntervention(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "ownership") || strings.Contains(message, "changed outside") ||
		strings.Contains(message, "occupied") || strings.Contains(message, "serve is not enabled") ||
		strings.Contains(message, "login.tailscale.com/f/serve")
}
