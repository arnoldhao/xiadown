package libraryaccessrepo

import (
	"context"
	"errors"
	"testing"

	"xiadown/internal/domain/libraryaccess"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	repo := NewSQLiteRepository(database.Bun)

	if _, err := repo.Get(ctx); !errors.Is(err, libraryaccess.ErrConfigNotFound) {
		t.Fatalf("empty Get error = %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_access_settings (
  id, remote_enabled, lan_enabled, lan_port, tailscale_enabled,
  tailscale_https_port, tailscale_path, device_name
) VALUES (1, 0, 1, 0, 0, 443, '/xiadown', 'Legacy Desktop')
`); err != nil {
		t.Fatalf("seed legacy zero port: %v", err)
	}
	legacy, err := repo.Get(ctx)
	if err != nil || legacy.LANPort != libraryaccess.DefaultLANPort {
		t.Fatalf("legacy config = %+v, %v", legacy, err)
	}
	if _, err := database.SQL.ExecContext(ctx, "DELETE FROM library_access_settings"); err != nil {
		t.Fatalf("clear legacy config: %v", err)
	}
	config, err := libraryaccess.NewConfig(libraryaccess.ConfigParams{
		RemoteEnabled: true, LANEnabled: true, LANPort: 43001,
		TailscaleEnabled: true, TailscaleHTTPSPort: 8443,
		TailscalePath: "/xiadown", DeviceName: "Desktop",
	})
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	if err := repo.Save(ctx, config); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != config {
		t.Fatalf("round trip = %+v, want %+v", got, config)
	}

	config.DeviceName = "Updated Desktop"
	config.RemoteEnabled = false
	if err := repo.Save(ctx, config); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = repo.Get(ctx)
	if err != nil || got != config {
		t.Fatalf("updated config = %+v, %v, want %+v", got, err, config)
	}
}

func TestSQLiteRepositoryPersistsManagedTailscaleRouteStateAndAudit(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	repo := NewSQLiteRepository(database.Bun)

	if _, err := repo.GetManagedTailscaleRoute(ctx); !errors.Is(err, libraryaccess.ErrManagedTailscaleRouteNotFound) {
		t.Fatalf("empty managed route error = %v", err)
	}
	transitions := []libraryaccess.TailscaleRouteTransition{
		mustTransition(t, 8443, "/mobile", 0, 43123, libraryaccess.TailscaleRouteStateEnabling,
			libraryaccess.TailscaleRouteActionEnable, libraryaccess.TailscaleRouteResultPending, ""),
		mustTransition(t, 8443, "/mobile", 43123, 0, libraryaccess.TailscaleRouteStateActive,
			libraryaccess.TailscaleRouteActionEnable, libraryaccess.TailscaleRouteResultSucceeded, ""),
		mustTransition(t, 8443, "/mobile", 43123, 0, libraryaccess.TailscaleRouteStateDisabling,
			libraryaccess.TailscaleRouteActionDisable, libraryaccess.TailscaleRouteResultPending, ""),
		mustTransition(t, 8443, "/mobile", 43123, 0, libraryaccess.TailscaleRouteStateError,
			libraryaccess.TailscaleRouteActionDisable, libraryaccess.TailscaleRouteResultFailed, "permission denied"),
		mustTransition(t, 8443, "/mobile", 43123, 0, libraryaccess.TailscaleRouteStateInactive,
			libraryaccess.TailscaleRouteActionRelease, libraryaccess.TailscaleRouteResultSucceeded, "external rewrite"),
	}
	for index, transition := range transitions {
		got, err := repo.TransitionManagedTailscaleRoute(ctx, transition)
		if err != nil {
			t.Fatalf("transition %d: %v", index, err)
		}
		if got.Revision != int64(index+1) || got.HTTPSPort != transition.HTTPSPort ||
			got.Path != transition.Path || got.BackendPort != transition.BackendPort ||
			got.PendingBackendPort != transition.PendingBackendPort || got.State != transition.State ||
			got.LastAction != transition.Action || got.LastResult != transition.Result ||
			got.LastError != transition.Error || got.UpdatedAt.IsZero() {
			t.Fatalf("transition %d state = %+v, want %+v", index, got, transition)
		}
	}

	got, err := repo.GetManagedTailscaleRoute(ctx)
	if err != nil {
		t.Fatalf("get managed route: %v", err)
	}
	if got.Revision != 5 || got.LastAction != libraryaccess.TailscaleRouteActionRelease ||
		got.LastResult != libraryaccess.TailscaleRouteResultSucceeded ||
		got.LastError != "external rewrite" || got.Claimed() {
		t.Fatalf("managed route = %+v", got)
	}
	var auditCount int
	if err := database.SQL.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM library_access_tailscale_route_audit",
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != len(transitions) {
		t.Fatalf("audit count = %d, want %d", auditCount, len(transitions))
	}
	var backendPort, pendingBackendPort int
	var action, result, message string
	if err := database.SQL.QueryRowContext(ctx, `
SELECT backend_port, pending_backend_port, action, result, error
FROM library_access_tailscale_route_audit
ORDER BY id DESC
LIMIT 1
`).Scan(&backendPort, &pendingBackendPort, &action, &result, &message); err != nil {
		t.Fatalf("read last audit: %v", err)
	}
	if backendPort != 43123 || pendingBackendPort != 0 ||
		action != "release" || result != "succeeded" || message != "external rewrite" {
		t.Fatalf("last audit = (%d, %d, %q, %q, %q)",
			backendPort, pendingBackendPort, action, result, message)
	}
	if _, err := database.SQL.ExecContext(ctx,
		"UPDATE library_access_tailscale_route_audit SET error = 'rewritten' WHERE id = 1",
	); err == nil {
		t.Fatal("append-only audit allowed update")
	}
	if _, err := database.SQL.ExecContext(ctx,
		"DELETE FROM library_access_tailscale_route_audit WHERE id = 1",
	); err == nil {
		t.Fatal("append-only audit allowed delete")
	}
}

func TestSQLiteRepositoryRollsBackStateWhenAuditAppendFails(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	repo := NewSQLiteRepository(database.Bun)
	active := mustTransition(t, 443, "/xiadown", 43123, 0, libraryaccess.TailscaleRouteStateActive,
		libraryaccess.TailscaleRouteActionEnable, libraryaccess.TailscaleRouteResultSucceeded, "")
	if _, err := repo.TransitionManagedTailscaleRoute(ctx, active); err != nil {
		t.Fatalf("seed active route: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
CREATE TRIGGER block_tailscale_route_audit
BEFORE INSERT ON library_access_tailscale_route_audit
BEGIN
  SELECT RAISE(ABORT, 'audit blocked');
END;
`); err != nil {
		t.Fatalf("create audit failure trigger: %v", err)
	}
	pendingDisable := mustTransition(t, 443, "/xiadown", 43123, 0, libraryaccess.TailscaleRouteStateDisabling,
		libraryaccess.TailscaleRouteActionDisable, libraryaccess.TailscaleRouteResultPending, "")
	if _, err := repo.TransitionManagedTailscaleRoute(ctx, pendingDisable); err == nil {
		t.Fatal("transition unexpectedly succeeded with blocked audit")
	}

	got, err := repo.GetManagedTailscaleRoute(ctx)
	if err != nil {
		t.Fatalf("get route after rollback: %v", err)
	}
	if got.State != libraryaccess.TailscaleRouteStateActive || got.Revision != 1 ||
		got.LastAction != libraryaccess.TailscaleRouteActionEnable {
		t.Fatalf("state update was not rolled back: %+v", got)
	}
	var auditCount int
	if err := database.SQL.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM library_access_tailscale_route_audit",
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit after rollback: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit count after rollback = %d, want 1", auditCount)
	}
}

func mustTransition(
	t *testing.T,
	httpsPort int,
	routePath string,
	backendPort int,
	pendingBackendPort int,
	state libraryaccess.TailscaleRouteState,
	action libraryaccess.TailscaleRouteAction,
	result libraryaccess.TailscaleRouteResult,
	message string,
) libraryaccess.TailscaleRouteTransition {
	t.Helper()
	transition, err := libraryaccess.NewTailscaleRouteTransition(
		httpsPort, routePath, backendPort, pendingBackendPort, state, action, result, message,
	)
	if err != nil {
		t.Fatalf("new transition: %v", err)
	}
	return transition
}
