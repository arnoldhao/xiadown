package persistence

import (
	"context"
	"strings"
	"testing"
)

func TestLibraryAccessMigrationContainsOnlyNonSecretConfiguration(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, SQLiteConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.SQL.ExecContext(ctx, `
INSERT INTO library_access_settings (id, device_name) VALUES (1, 'Test Mac')
`); err != nil {
		t.Fatalf("insert defaults: %v", err)
	}
	var remote, lan, lanPort, tailscale, httpsPort int
	var route string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT remote_enabled, lan_enabled, lan_port, tailscale_enabled,
       tailscale_https_port, tailscale_path
FROM library_access_settings WHERE id = 1
`).Scan(&remote, &lan, &lanPort, &tailscale, &httpsPort, &route); err != nil {
		t.Fatalf("read defaults: %v", err)
	}
	if remote != 0 || lan != 1 || lanPort != 0 || tailscale != 0 || httpsPort != 443 || route != "/xiadown" {
		t.Fatalf("unexpected persisted defaults: remote=%d lan=%d lanPort=%d tailscale=%d https=%d path=%q",
			remote, lan, lanPort, tailscale, httpsPort, route)
	}

	rows, err := db.SQL.QueryContext(ctx, "PRAGMA table_info(library_access_settings)")
	if err != nil {
		t.Fatalf("table info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ordinal, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&ordinal, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "password") || strings.Contains(lower, "credential") {
			t.Fatalf("secret-bearing column %q must not be in access settings", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table info rows: %v", err)
	}
}
