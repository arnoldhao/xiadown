package persistence

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestFreshAppSessionStorageDoesNotCreateLegacyTable(t *testing.T) {
	ctx := context.Background()
	database, err := OpenSQLite(ctx, SQLiteConfig{
		Path:                     filepath.Join(t.TempDir(), "fresh.db"),
		SkipPreMigrationSnapshot: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var legacyTables, currentTables int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'site_app_sessions'
`).Scan(&legacyTables); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'table' AND name IN ('app_sessions', 'app_session_secrets')
`).Scan(&currentTables); err != nil {
		t.Fatal(err)
	}
	if legacyTables != 0 || currentTables != 2 {
		t.Fatalf("legacy tables=%d current tables=%d", legacyTables, currentTables)
	}
}

func TestAppSessionStorageMigrationDoesNotReadOrMutateLegacyState(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "app-session-v15.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
CREATE TABLE site_app_sessions (
	id TEXT PRIMARY KEY,
	site_key TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL,
	account_display_name TEXT,
	account_handle TEXT,
	account_avatar_url TEXT,
	account_tier_key TEXT,
	account_tier_label TEXT,
	account_badges_json TEXT,
	account_metadata_json TEXT,
	account_verification_status TEXT NOT NULL DEFAULT 'unverified',
	account_verification_error TEXT,
	account_verification_started_at TIMESTAMP,
	last_verified_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO site_app_sessions (
	id, site_key, status, account_display_name, account_handle,
	account_avatar_url, account_tier_key, account_tier_label,
	account_badges_json, account_metadata_json,
	account_verification_status, account_verification_error,
	account_verification_started_at, last_verified_at,
	created_at, updated_at
) VALUES (
	'site-app-session-youtube', 'youtube', 'connected', 'Legacy User', '@legacy',
	'https://example.com/avatar', 'premium', 'Premium',
	'["legacy"]', '{"legacy":true}',
	'verified', 'legacy error',
	'2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z',
	'2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z'
);
DROP TABLE app_session_secrets;
DROP TABLE app_sessions;
DELETE FROM schema_migrations WHERE version = 16;
PRAGMA user_version = 15;
`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	if upgraded.MigrationSnapshotPath == "" {
		t.Fatal("v15 App Session storage upgrade did not create a snapshot")
	}
	var version, metadataTables, secretTables, metadataColumns, secretColumns, metadataRows, secretRows int
	if err := upgraded.SQL.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'app_sessions'
`).Scan(&metadataTables); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'app_session_secrets'
`).Scan(&secretTables); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('app_sessions')
WHERE name IN (
	'id', 'site_key', 'status', 'source_type', 'source_browser',
	'source_profile', 'last_synced_at', 'created_at', 'updated_at'
)
`).Scan(&metadataColumns); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('app_session_secrets')
WHERE name IN ('site_key', 'key_id', 'format_version', 'nonce', 'ciphertext', 'created_at', 'updated_at')
`).Scan(&secretColumns); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_sessions`).Scan(&metadataRows); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_session_secrets`).Scan(&secretRows); err != nil {
		t.Fatal(err)
	}
	if version != sqliteMigrations[len(sqliteMigrations)-1].version || metadataTables != 1 || secretTables != 1 || metadataColumns != 9 || secretColumns != 7 || metadataRows != 0 || secretRows != 0 {
		t.Fatalf("version=%d metadata=(%d,%d,%d) secrets=(%d,%d,%d)",
			version, metadataTables, metadataColumns, metadataRows, secretTables, secretColumns, secretRows)
	}

	var status, verificationStatus string
	var displayName, handle, avatarURL, tierKey, tierLabel sql.NullString
	var badgesJSON, metadataJSON, verificationError sql.NullString
	var verificationStartedAt, lastVerifiedAt sql.NullTime
	if err := upgraded.SQL.QueryRowContext(ctx, `
SELECT status, account_display_name, account_handle, account_avatar_url,
	account_tier_key, account_tier_label, account_badges_json,
	account_metadata_json, account_verification_status,
	account_verification_error, account_verification_started_at, last_verified_at
FROM site_app_sessions WHERE site_key = 'youtube'
`).Scan(
		&status, &displayName, &handle, &avatarURL,
		&tierKey, &tierLabel, &badgesJSON,
		&metadataJSON, &verificationStatus,
		&verificationError, &verificationStartedAt, &lastVerifiedAt,
	); err != nil {
		t.Fatal(err)
	}
	if status != "connected" || verificationStatus != "verified" ||
		!displayName.Valid || !handle.Valid || !avatarURL.Valid || !tierKey.Valid || !tierLabel.Valid ||
		!badgesJSON.Valid || !metadataJSON.Valid || !verificationError.Valid ||
		!verificationStartedAt.Valid || !lastVerifiedAt.Valid {
		t.Fatalf("legacy App Session state was mutated: status=%q verification=%q", status, verificationStatus)
	}
}
