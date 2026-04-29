package app

import (
	"context"
	"errors"
	"testing"

	domainupdate "xiadown/internal/domain/update"
)

type preparedUpdateStartupStub struct {
	prepared       domainupdate.Info
	found          bool
	preparedErr    error
	clearInvoked   bool
	restartInvoked bool
	restartErr     error
}

func (stub *preparedUpdateStartupStub) PreparedUpdate(_ context.Context) (domainupdate.Info, bool, error) {
	return stub.prepared, stub.found, stub.preparedErr
}

func (stub *preparedUpdateStartupStub) ClearPreparedUpdate(_ context.Context) error {
	stub.clearInvoked = true
	return nil
}

func (stub *preparedUpdateStartupStub) RestartToApply(_ context.Context) error {
	stub.restartInvoked = true
	return stub.restartErr
}

func TestMaybeApplyPreparedUpdateOnLaunchStartsHelperForOlderCurrentVersion(t *testing.T) {
	t.Parallel()

	installer := &preparedUpdateStartupStub{
		found: true,
		prepared: domainupdate.Info{
			PreparedVersion: "2.0.7",
		},
	}

	applied, err := maybeApplyPreparedUpdateOnLaunch(context.Background(), "2.0.6", installer)
	if err != nil {
		t.Fatalf("maybeApplyPreparedUpdateOnLaunch failed: %v", err)
	}
	if !applied {
		t.Fatal("expected prepared update to be applied on launch")
	}
	if !installer.restartInvoked {
		t.Fatal("expected restart helper to be invoked")
	}
}

func TestMaybeApplyPreparedUpdateOnLaunchClearsStalePreparedPlan(t *testing.T) {
	t.Parallel()

	installer := &preparedUpdateStartupStub{
		found: true,
		prepared: domainupdate.Info{
			PreparedVersion: "2.0.7",
		},
	}

	applied, err := maybeApplyPreparedUpdateOnLaunch(context.Background(), "2.0.7", installer)
	if err != nil {
		t.Fatalf("maybeApplyPreparedUpdateOnLaunch failed: %v", err)
	}
	if applied {
		t.Fatal("did not expect prepared update to apply when current version is already latest")
	}
	if !installer.clearInvoked {
		t.Fatal("expected stale prepared plan to be cleared")
	}
}

func TestMaybeApplyPreparedUpdateOnLaunchSkipsNonReleaseVersion(t *testing.T) {
	t.Parallel()

	installer := &preparedUpdateStartupStub{
		found: true,
		prepared: domainupdate.Info{
			PreparedVersion: "2.0.7",
		},
	}

	applied, err := maybeApplyPreparedUpdateOnLaunch(context.Background(), "dev", installer)
	if err != nil {
		t.Fatalf("maybeApplyPreparedUpdateOnLaunch failed: %v", err)
	}
	if applied {
		t.Fatal("did not expect dev build to auto-apply prepared update")
	}
	if installer.restartInvoked {
		t.Fatal("did not expect restart helper to run")
	}
}

func TestMaybeApplyPreparedUpdateOnLaunchReturnsRestartError(t *testing.T) {
	t.Parallel()

	restartErr := errors.New("helper launch failed")
	installer := &preparedUpdateStartupStub{
		found: true,
		prepared: domainupdate.Info{
			PreparedVersion: "2.0.7",
		},
		restartErr: restartErr,
	}

	applied, err := maybeApplyPreparedUpdateOnLaunch(context.Background(), "2.0.6", installer)
	if !errors.Is(err, restartErr) {
		t.Fatalf("expected restart error, got %v", err)
	}
	if applied {
		t.Fatal("did not expect prepared update to report success")
	}
}
