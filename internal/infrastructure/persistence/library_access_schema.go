package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const libraryAccessSchemaSQL = `
CREATE TABLE IF NOT EXISTS library_access_settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	remote_enabled INTEGER NOT NULL DEFAULT 0 CHECK (remote_enabled IN (0, 1)),
	lan_enabled INTEGER NOT NULL DEFAULT 1 CHECK (lan_enabled IN (0, 1)),
	lan_port INTEGER NOT NULL DEFAULT 0 CHECK (lan_port BETWEEN 0 AND 65535),
	tailscale_enabled INTEGER NOT NULL DEFAULT 0 CHECK (tailscale_enabled IN (0, 1)),
	tailscale_https_port INTEGER NOT NULL DEFAULT 443 CHECK (tailscale_https_port BETWEEN 1 AND 65535),
	tailscale_path TEXT NOT NULL DEFAULT '/xiadown' CHECK (
		length(tailscale_path) > 1 AND
		substr(tailscale_path, 1, 1) = '/' AND
		instr(tailscale_path, '?') = 0 AND
		instr(tailscale_path, '#') = 0
	),
	device_name TEXT NOT NULL CHECK (length(trim(device_name)) BETWEEN 1 AND 120),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS library_access_settings_updated_at
AFTER UPDATE ON library_access_settings
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
	UPDATE library_access_settings
	SET updated_at = CURRENT_TIMESTAMP
	WHERE id = NEW.id;
END;
`

func applyLibraryAccessSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin library access schema migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, libraryAccessSchemaSQL); err != nil {
		return fmt.Errorf("apply library access schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit library access schema migration: %w", err)
	}
	return nil
}
