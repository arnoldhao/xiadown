package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// appSessionStorageSchemaSQL deliberately creates empty, new-generation
// tables. It never selects from or mutates site_app_sessions; that legacy
// table remains outside the runtime and is owned by Data Manager cleanup.
const appSessionStorageSchemaSQL = `
CREATE TABLE IF NOT EXISTS app_sessions (
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
	source_type TEXT,
	source_browser TEXT,
	source_profile TEXT,
	last_synced_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app_session_secrets (
	site_key TEXT PRIMARY KEY
		CHECK (length(trim(site_key)) > 0 AND site_key = lower(trim(site_key))),
	key_id TEXT NOT NULL CHECK (key_id = 'master-key'),
	format_version INTEGER NOT NULL CHECK (format_version = 1),
	nonce BLOB NOT NULL CHECK (length(nonce) = 12),
	ciphertext BLOB NOT NULL CHECK (length(ciphertext) >= 16),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (site_key) REFERENCES app_sessions(site_key) ON DELETE CASCADE
);
`

func applyAppSessionStorageSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, appSessionStorageSchemaSQL); err != nil {
		return fmt.Errorf("apply App Session storage schema: %w", err)
	}
	return nil
}
