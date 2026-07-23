package persistence

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibraryTailscaleRouteMigrationAdoptsOnlyPreviouslyDesiredExactRoute(t *testing.T) {
	for _, testCase := range []struct {
		name             string
		remoteEnabled    bool
		tailscaleEnabled bool
		wantAdopted      bool
	}{
		{name: "remote and tailscale desired", remoteEnabled: true, tailscaleEnabled: true, wantAdopted: true},
		{name: "remote disabled", remoteEnabled: false, tailscaleEnabled: true, wantAdopted: false},
		{name: "tailscale disabled", remoteEnabled: true, tailscaleEnabled: false, wantAdopted: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "v6.db")
			database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
			if err != nil {
				t.Fatalf("create database: %v", err)
			}
			if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_access_settings (
    id, remote_enabled, lan_enabled, lan_port, tailscale_enabled,
    tailscale_https_port, tailscale_path, device_name
)
VALUES (1, ?, 1, 0, ?, 8443, '/mobile', 'Studio')
ON CONFLICT (id) DO UPDATE SET
    remote_enabled = EXCLUDED.remote_enabled,
    tailscale_enabled = EXCLUDED.tailscale_enabled,
    tailscale_https_port = EXCLUDED.tailscale_https_port,
    tailscale_path = EXCLUDED.tailscale_path
`, testCase.remoteEnabled, testCase.tailscaleEnabled); err != nil {
				_ = database.Close()
				t.Fatalf("seed v6 settings: %v", err)
			}
			if _, err := database.SQL.ExecContext(ctx, `
DROP TRIGGER library_catalog_sync_state_after_catalog_insert;
DROP TABLE library_catalog_sync_state;
DROP TABLE library_access_tailscale_route_audit;
DROP TABLE library_access_tailscale_route_state;
DELETE FROM schema_migrations WHERE version IN (7, 8);
PRAGMA user_version = 6;
`); err != nil {
				_ = database.Close()
				t.Fatalf("rewind to v6: %v", err)
			}
			if err := database.Close(); err != nil {
				t.Fatalf("close v6 database: %v", err)
			}

			upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
			if err != nil {
				t.Fatalf("upgrade v6 database: %v", err)
			}
			defer upgraded.Close()
			if upgraded.MigrationSnapshotPath == "" {
				t.Fatal("v6 upgrade did not create a pre-migration snapshot")
			}

			var stateCount int
			if err := upgraded.SQL.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM library_access_tailscale_route_state",
			).Scan(&stateCount); err != nil {
				t.Fatalf("count adopted state: %v", err)
			}
			if testCase.wantAdopted {
				if stateCount != 1 {
					t.Fatalf("adopted state count = %d, want 1", stateCount)
				}
				var httpsPort, backendPort, pendingBackendPort, revision int
				var routePath, state, action, result, message string
				if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT https_port, route_path, backend_port, pending_backend_port,
       state, last_action, last_result, last_error, revision
FROM library_access_tailscale_route_state
WHERE id = 1
`).Scan(&httpsPort, &routePath, &backendPort, &pendingBackendPort,
					&state, &action, &result, &message, &revision); err != nil {
					t.Fatalf("read adopted state: %v", err)
				}
				if httpsPort != 8443 || routePath != "/mobile" || backendPort != 0 || pendingBackendPort != 0 || state != "unknown" ||
					action != "adopt" || result != "pending" || message != "" || revision != 1 {
					t.Fatalf("adopted state = port=%d path=%q backend=%d pending=%d state=%q action=%q result=%q error=%q revision=%d",
						httpsPort, routePath, backendPort, pendingBackendPort, state, action, result, message, revision)
				}
			} else if stateCount != 0 {
				t.Fatalf("disabled configuration created %d ownership claims", stateCount)
			}

			var auditCount int
			if err := upgraded.SQL.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM library_access_tailscale_route_audit",
			).Scan(&auditCount); err != nil {
				t.Fatalf("count adoption audit: %v", err)
			}
			wantAuditCount := 0
			if testCase.wantAdopted {
				wantAuditCount = 1
			}
			if auditCount != wantAuditCount {
				t.Fatalf("adoption audit count = %d, want %d", auditCount, wantAuditCount)
			}
			if testCase.wantAdopted {
				var backendPort, pendingBackendPort int
				if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT backend_port, pending_backend_port
FROM library_access_tailscale_route_audit
WHERE action = 'adopt'
`).Scan(&backendPort, &pendingBackendPort); err != nil {
					t.Fatalf("read adoption audit ownership: %v", err)
				}
				if backendPort != 0 || pendingBackendPort != 0 {
					t.Fatalf("adoption audit ownership = (%d, %d), want unknown (0, 0)", backendPort, pendingBackendPort)
				}
			}
		})
	}
}

func TestLibraryTailscaleRouteMigrationChecksumIncludesBackendOwnershipSchema(t *testing.T) {
	migration, ok := sqliteMigrationByVersion(7)
	if !ok {
		t.Fatal("v7 Tailscale ownership migration is missing")
	}
	if migration.signature != libraryTailscaleRouteSchemaSQL ||
		!strings.Contains(migration.signature, "backend_port INTEGER NOT NULL") ||
		!strings.Contains(migration.signature, "pending_backend_port INTEGER NOT NULL") {
		t.Fatal("v7 migration checksum signature does not cover backend ownership columns")
	}

	ctx := context.Background()
	database := openLatestSQLiteTestDatabase(t)
	defer database.Close()
	var checksum string
	if err := database.SQL.QueryRowContext(ctx,
		"SELECT checksum FROM schema_migrations WHERE version = 7",
	).Scan(&checksum); err != nil {
		t.Fatalf("read v7 migration checksum: %v", err)
	}
	if checksum != migration.checksum() {
		t.Fatalf("v7 checksum = %q, want %q", checksum, migration.checksum())
	}
}
