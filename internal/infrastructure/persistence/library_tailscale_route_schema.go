package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const libraryTailscaleRouteSchemaSQL = `
CREATE TABLE IF NOT EXISTS library_access_tailscale_route_state (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	https_port INTEGER NOT NULL CHECK (https_port BETWEEN 1 AND 65535),
	route_path TEXT NOT NULL CHECK (
		length(route_path) > 1 AND
		substr(route_path, 1, 1) = '/' AND
		instr(route_path, '?') = 0 AND
		instr(route_path, '#') = 0 AND
		instr(route_path, '\') = 0
	),
	backend_port INTEGER NOT NULL CHECK (backend_port BETWEEN 0 AND 65535),
	pending_backend_port INTEGER NOT NULL CHECK (pending_backend_port BETWEEN 0 AND 65535),
	state TEXT NOT NULL CHECK (state IN ('inactive', 'unknown', 'enabling', 'active', 'disabling', 'error')),
	last_action TEXT NOT NULL CHECK (last_action IN ('adopt', 'enable', 'disable', 'release')),
	last_result TEXT NOT NULL CHECK (last_result IN ('pending', 'succeeded', 'failed')),
	last_error TEXT NOT NULL DEFAULT '',
	revision INTEGER NOT NULL CHECK (revision > 0),
	updated_at TIMESTAMP NOT NULL,
	CHECK (
		(last_action = 'adopt' AND backend_port = 0 AND pending_backend_port = 0 AND state = 'unknown' AND last_result = 'pending') OR
		(last_action = 'enable' AND pending_backend_port > 0 AND state = 'enabling' AND last_result = 'pending') OR
		(last_action = 'enable' AND backend_port > 0 AND pending_backend_port = 0 AND state = 'active' AND last_result = 'succeeded') OR
		(last_action = 'enable' AND pending_backend_port > 0 AND state = 'error' AND last_result = 'failed') OR
		(last_action = 'disable' AND backend_port > 0 AND pending_backend_port = 0 AND state = 'disabling' AND last_result = 'pending') OR
		(last_action = 'disable' AND backend_port > 0 AND pending_backend_port = 0 AND state = 'inactive' AND last_result = 'succeeded') OR
		(last_action = 'disable' AND backend_port > 0 AND pending_backend_port = 0 AND state = 'error' AND last_result = 'failed') OR
		(last_action = 'release' AND pending_backend_port = 0 AND state = 'inactive' AND last_result = 'succeeded')
	),
	CHECK (
		(last_result = 'failed' AND length(trim(last_error)) > 0) OR
		(last_action = 'release' AND last_result = 'succeeded' AND length(trim(last_error)) > 0) OR
		(last_result != 'failed' AND last_action != 'release' AND last_error = '')
	)
);

CREATE TABLE IF NOT EXISTS library_access_tailscale_route_audit (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	https_port INTEGER NOT NULL CHECK (https_port BETWEEN 1 AND 65535),
	route_path TEXT NOT NULL,
	backend_port INTEGER NOT NULL CHECK (backend_port BETWEEN 0 AND 65535),
	pending_backend_port INTEGER NOT NULL CHECK (pending_backend_port BETWEEN 0 AND 65535),
	state TEXT NOT NULL CHECK (state IN ('inactive', 'unknown', 'enabling', 'active', 'disabling', 'error')),
	action TEXT NOT NULL CHECK (action IN ('adopt', 'enable', 'disable', 'release')),
	result TEXT NOT NULL CHECK (result IN ('pending', 'succeeded', 'failed')),
	error TEXT NOT NULL DEFAULT '',
	transitioned_at TIMESTAMP NOT NULL,
	CHECK (
		(action = 'adopt' AND backend_port = 0 AND pending_backend_port = 0 AND state = 'unknown' AND result = 'pending') OR
		(action = 'enable' AND pending_backend_port > 0 AND state = 'enabling' AND result = 'pending') OR
		(action = 'enable' AND backend_port > 0 AND pending_backend_port = 0 AND state = 'active' AND result = 'succeeded') OR
		(action = 'enable' AND pending_backend_port > 0 AND state = 'error' AND result = 'failed') OR
		(action = 'disable' AND backend_port > 0 AND pending_backend_port = 0 AND state = 'disabling' AND result = 'pending') OR
		(action = 'disable' AND backend_port > 0 AND pending_backend_port = 0 AND state = 'inactive' AND result = 'succeeded') OR
		(action = 'disable' AND backend_port > 0 AND pending_backend_port = 0 AND state = 'error' AND result = 'failed') OR
		(action = 'release' AND pending_backend_port = 0 AND state = 'inactive' AND result = 'succeeded')
	),
	CHECK (
		(result = 'failed' AND length(trim(error)) > 0) OR
		(action = 'release' AND result = 'succeeded' AND length(trim(error)) > 0) OR
		(result != 'failed' AND action != 'release' AND error = '')
	)
);

CREATE INDEX IF NOT EXISTS library_access_tailscale_route_audit_time_idx
	ON library_access_tailscale_route_audit(transitioned_at DESC, id DESC);

CREATE TRIGGER IF NOT EXISTS library_access_tailscale_route_audit_no_update
BEFORE UPDATE ON library_access_tailscale_route_audit
BEGIN
	SELECT RAISE(ABORT, 'tailscale route audit is append-only');
END;

CREATE TRIGGER IF NOT EXISTS library_access_tailscale_route_audit_no_delete
BEFORE DELETE ON library_access_tailscale_route_audit
BEGIN
	SELECT RAISE(ABORT, 'tailscale route audit is append-only');
END;

-- A v6 database has no durable route ownership ledger. Only adopt the exact
-- endpoint when Remote and Tailscale were both desired at migration time;
-- disabled configurations are not sufficient evidence that XiaDown owns a
-- coincidentally matching user route.
INSERT INTO library_access_tailscale_route_state (
	id, https_port, route_path, backend_port, pending_backend_port,
	state, last_action, last_result,
	last_error, revision, updated_at
)
SELECT
	1, tailscale_https_port, tailscale_path, 0, 0,
	'unknown', 'adopt', 'pending',
	'', 1, CURRENT_TIMESTAMP
FROM library_access_settings
WHERE id = 1 AND remote_enabled = 1 AND tailscale_enabled = 1
ON CONFLICT (id) DO NOTHING;

INSERT INTO library_access_tailscale_route_audit (
	https_port, route_path, backend_port, pending_backend_port,
	state, action, result, error, transitioned_at
)
SELECT
	https_port, route_path, backend_port, pending_backend_port,
	state, last_action, last_result, last_error, updated_at
FROM library_access_tailscale_route_state AS current
WHERE current.revision = 1
	AND current.last_action = 'adopt'
	AND NOT EXISTS (
		SELECT 1
		FROM library_access_tailscale_route_audit AS audit
		WHERE audit.action = 'adopt'
	);
`

func applyLibraryTailscaleRouteSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin library tailscale route schema migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, libraryTailscaleRouteSchemaSQL); err != nil {
		return fmt.Errorf("apply library tailscale route schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit library tailscale route schema migration: %w", err)
	}
	return nil
}
