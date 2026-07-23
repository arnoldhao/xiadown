package librarybackup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domainbackup "xiadown/internal/domain/librarybackup"
	"xiadown/internal/infrastructure/persistence"
)

// restoredLibraryTables is an explicit allowlist. A restore intentionally
// does not replace application settings/accounts, Library access policy,
// device grants, Tailscale route ownership/audit, or live process state.
var restoredLibraryTables = []string{
	"library_libraries",
	"library_files",
	"library_subtitle_documents",
	"transcode_presets",
	"library_operations",
	"library_operation_outputs",
	"library_operation_chunks",
	"library_history_records",
	"library_history_files",
	"library_workspace_states",
	"library_workspace_state_head",
	"library_file_events",
	"library_import_batches",
	"library_import_candidates",
	// Sync journal state is ordered before the Listen Local graph so the
	// reverse clear pass removes Tracks/Playlists first (allowing their safety
	// triggers to run), then clears those trigger-generated rows before copy.
	"listen_local_music_sync_state",
	"listen_local_music_memberships",
	"listen_local_music_tombstones",
	"listen_local_music_changes",
	"listen_local_music_mutation_receipts",
	"listen_local_music_track_states",
	"listen_local_music_lyric_documents",
	"listen_local_music_lyric_selections",
	"listen_local_music_play_sessions",
	"listen_local_music_play_event_checkpoints",
	"listen_local_music_play_event_receipts",
	"listen_local_tracks",
	"listen_local_playlists",
	"listen_local_playlist_items",
	"listen_live_columns",
	"listen_live_channels",
	"library_catalogs",
	"library_catalog_items",
	"library_item_assets",
	"library_storage_roots",
	"library_collections",
	"library_collection_items",
	"library_tags",
	"library_item_tags",
	"library_user_states",
	"library_representations",
	"library_metadata_entries",
	"library_legacy_mappings",
	"library_migration_checkpoints",
	"rss_workspaces",
	"rss_sync_state",
	"rss_categories",
	"rss_collections",
	"rss_subscriptions",
	"rss_subscription_field_revisions",
	"rss_subscription_history",
	"rss_entries",
	"rss_entry_origins",
	"rss_entry_downloads",
	"rss_observation_sources",
	"rss_sources",
	"rss_collection_subscriptions",
	"rss_collection_entries",
	"rss_client_mutations",
	"rss_public_mutations",
	"rss_changes",
	"rss_tombstones",
}

// Process identifiers are machine- and process-lifetime state. Task, history,
// workspace, file-event, subtitle-document, custom transcode-preset, and import
// records are durable Library metadata and remain part of the allowlisted restore.
var clearedRuntimeTables = []string{
	"library_external_processes",
	"rss_fetch_leases",
}

type logicalRestoreResult struct {
	identity               domainbackup.Manifest
	currentSecurityTrusted bool
}

func prepareLogicalRestore(
	ctx context.Context,
	config StartupRestoreConfig,
	databaseSource string,
	workingPath string,
	sourcePath string,
	stagingPath string,
) (logicalRestoreResult, error) {
	for _, path := range []string{workingPath, sourcePath, stagingPath} {
		if err := removeSQLiteArtifacts(path); err != nil {
			return logicalRestoreResult{}, err
		}
	}

	// Migrate only a disposable copy of the requested backup. Old-schema
	// backups are therefore verified against this build before any swap.
	if err := copyFileSynced(databaseSource, sourcePath); err != nil {
		return logicalRestoreResult{}, fmt.Errorf("copy restore source for staged migration: %w", err)
	}
	migratedSource := sourcePath + ".migrated"
	if err := migrateRestoreSource(ctx, sourcePath, migratedSource); err != nil {
		return logicalRestoreResult{}, err
	}
	defer removeSQLiteArtifacts(migratedSource)

	baseCreated, trusted := snapshotCurrentBase(ctx, config.DatabasePath, workingPath)
	if baseCreated && !trusted {
		candidate, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: workingPath})
		if err == nil {
			err = resetSecurityDomain(ctx, candidate.SQL)
		}
		if err == nil {
			err = validateReadableSecurityDomain(ctx, candidate.SQL)
		}
		if candidate != nil {
			if closeErr := candidate.Close(); err == nil {
				err = closeErr
			}
		}
		if err != nil {
			_ = removeSQLiteArtifacts(workingPath)
			baseCreated = false
		}
	}
	if !baseCreated {
		if err := removeSQLiteArtifacts(workingPath); err != nil {
			return logicalRestoreResult{}, err
		}
		fresh, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: workingPath})
		if err != nil {
			return logicalRestoreResult{}, fmt.Errorf("create isolated restore base: %w", err)
		}
		if err := resetSecurityDomain(ctx, fresh.SQL); err != nil {
			_ = fresh.Close()
			return logicalRestoreResult{}, err
		}
		if err := fresh.Close(); err != nil {
			return logicalRestoreResult{}, fmt.Errorf("close isolated restore base: %w", err)
		}
	}

	working, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: workingPath})
	if err != nil {
		return logicalRestoreResult{}, fmt.Errorf("open logical restore candidate: %w", err)
	}
	closeWorking := true
	defer func() {
		if closeWorking {
			_ = working.Close()
		}
	}()
	if err := replaceAllowedLibraryMetadata(ctx, working.SQL, migratedSource); err != nil {
		return logicalRestoreResult{}, err
	}
	if err := persistence.VerifySQLiteIntegrity(ctx, working.SQL, false); err != nil {
		return logicalRestoreResult{}, fmt.Errorf("verify logical restore candidate: %w", err)
	}
	if err := persistence.CreateConsistentSQLiteSnapshot(ctx, working.SQL, stagingPath); err != nil {
		return logicalRestoreResult{}, fmt.Errorf("compact logical restore candidate: %w", err)
	}
	if err := restrictBackupFile(stagingPath); err != nil {
		return logicalRestoreResult{}, fmt.Errorf("secure logical restore candidate: %w", err)
	}
	if err := syncFile(stagingPath); err != nil {
		return logicalRestoreResult{}, fmt.Errorf("sync logical restore candidate: %w", err)
	}
	if err := working.Close(); err != nil {
		return logicalRestoreResult{}, fmt.Errorf("close logical restore candidate: %w", err)
	}
	closeWorking = false

	identity, err := inspectSnapshot(ctx, stagingPath, snapshotInspection{
		ExpectedApplicationID: config.ExpectedApplicationID,
		MaxSchemaVersion:      config.MaxSupportedSchemaVersion,
	})
	if err != nil {
		return logicalRestoreResult{}, fmt.Errorf("inspect logical restore candidate: %w", err)
	}
	return logicalRestoreResult{identity: identity, currentSecurityTrusted: trusted}, nil
}

func migrateRestoreSource(ctx context.Context, sourcePath, compactPath string) (returnErr error) {
	// OpenSQLite intentionally retains a pre-migration snapshot for a live
	// database. This source is already a disposable copy of a verified backup,
	// so retaining that second full database would leak sensitive metadata
	// outside backup retention. Clean it on every success and failure path,
	// including failures where OpenSQLite cannot return MigrationSnapshotPath.
	defer func() {
		returnErr = errors.Join(returnErr, cleanupRestoreMigrationSnapshots(sourcePath))
	}()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: sourcePath,
		// sourcePath is an fsynced disposable copy of the already retained backup.
		// Creating another migration snapshot would add no recovery value and can
		// strand a full metadata database if the process crashes mid-migration.
		SkipPreMigrationSnapshot: true,
	})
	if err != nil {
		return fmt.Errorf("migrate staged restore source: %w", err)
	}
	if database.MigrationSnapshotPath != "" {
		_ = database.Close()
		return errors.New("disposable restore source unexpectedly created a migration snapshot")
	}
	if err := persistence.VerifySQLiteIntegrity(ctx, database.SQL, false); err != nil {
		_ = database.Close()
		return fmt.Errorf("verify migrated restore source: %w", err)
	}
	if err := persistence.CreateConsistentSQLiteSnapshot(ctx, database.SQL, compactPath); err != nil {
		_ = database.Close()
		return fmt.Errorf("compact migrated restore source: %w", err)
	}
	if err := database.Close(); err != nil {
		return fmt.Errorf("close migrated restore source: %w", err)
	}
	if err := restrictBackupFile(compactPath); err != nil {
		return err
	}
	return syncFile(compactPath)
}

func cleanupRestoreMigrationSnapshots(sourcePath string) error {
	directory := filepath.Dir(sourcePath)
	prefix := filepath.Base(sourcePath) + ".pre-migration-v"
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("list staged migration snapshots: %w", err)
	}
	removed := false
	var cleanupErr error
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".bak") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("inspect staged migration snapshot %s: %w", name, infoErr))
			continue
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("staged migration snapshot is not a regular file: %s", name))
			continue
		}
		if err := removeRestoreMigrationSnapshotDurably(sourcePath, filepath.Join(directory, name)); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove staged migration snapshot %s: %w", name, err))
			continue
		}
		removed = true
	}
	if removed {
		cleanupErr = errors.Join(cleanupErr, syncDirectory(directory))
	}
	return cleanupErr
}

func removeRestoreMigrationSnapshotDurably(sourcePath, snapshotPath string) error {
	directory := filepath.Dir(sourcePath)
	tombstone := sourcePath + ".pre-migration-vdelete-" + randomSuffix() + ".bak"
	if err := durableRename(snapshotPath, tombstone, false); err != nil {
		return err
	}
	if err := syncDirectory(directory); err != nil {
		return err
	}
	if err := os.Remove(tombstone); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDirectory(directory)
}

// snapshotCurrentBase preserves non-Library settings/accounts whenever the
// current database itself is readable. Security trust is reported separately:
// an untrusted security domain is reset on the disposable snapshot, while a
// structurally unreadable database falls back to a fresh isolated base.
func snapshotCurrentBase(ctx context.Context, databasePath, targetPath string) (bool, bool) {
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: databasePath})
	if err != nil {
		return false, false
	}
	defer database.Close()
	securityTrusted := validateReadableSecurityDomain(ctx, database.SQL) == nil
	if err := persistence.CreateConsistentSQLiteSnapshot(ctx, database.SQL, targetPath); err != nil {
		return false, false
	}
	if err := restrictBackupFile(targetPath); err != nil {
		_ = removeSQLiteArtifacts(targetPath)
		return false, false
	}
	if err := syncFile(targetPath); err != nil {
		_ = removeSQLiteArtifacts(targetPath)
		return false, false
	}
	return true, securityTrusted
}

func validateReadableSecurityDomain(ctx context.Context, db *sql.DB) error {
	for table, query := range map[string]string{
		"library_device_grants": `
SELECT id, catalog_id, credential_hash, status, revision FROM library_device_grants LIMIT 0`,
		"library_access_settings": `
SELECT id, remote_enabled, tailscale_enabled, tailscale_https_port, tailscale_path
FROM library_access_settings LIMIT 0`,
		"library_access_tailscale_route_state": `
SELECT id, https_port, route_path, backend_port, pending_backend_port, state, revision
FROM library_access_tailscale_route_state LIMIT 0`,
		"library_access_tailscale_route_audit": `
SELECT id, https_port, route_path, backend_port, pending_backend_port, state, action, result
FROM library_access_tailscale_route_audit LIMIT 0`,
	} {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("read security table %s: %w", table, err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close security table %s validation: %w", table, err)
		}
	}
	for trigger, operation := range map[string]string{
		"library_access_tailscale_route_audit_no_update": "BEFORE UPDATE ON library_access_tailscale_route_audit",
		"library_access_tailscale_route_audit_no_delete": "BEFORE DELETE ON library_access_tailscale_route_audit",
	} {
		var definition string
		if err := db.QueryRowContext(ctx, `
SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?
`, trigger).Scan(&definition); err != nil {
			return fmt.Errorf("validate security audit trigger %s: %w", trigger, err)
		}
		if !strings.Contains(definition, operation) ||
			!strings.Contains(definition, "RAISE(ABORT, 'tailscale route audit is append-only')") {
			return fmt.Errorf("validate security audit trigger %s: %w", trigger, errors.New("append-only definition is invalid"))
		}
	}
	return nil
}

func resetSecurityDomain(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
DELETE FROM library_device_grants;
DELETE FROM library_access_tailscale_route_state;
INSERT INTO library_access_settings (
  id, remote_enabled, lan_enabled, lan_port, tailscale_enabled,
  tailscale_https_port, tailscale_path, device_name
) VALUES (1, 0, 1, 0, 0, 443, '/xiadown', 'XiaDown')
ON CONFLICT (id) DO UPDATE SET
  remote_enabled = 0,
  tailscale_enabled = 0;

DROP TRIGGER IF EXISTS library_access_tailscale_route_audit_no_update;
DROP TRIGGER IF EXISTS library_access_tailscale_route_audit_no_delete;
CREATE TRIGGER library_access_tailscale_route_audit_no_update
BEFORE UPDATE ON library_access_tailscale_route_audit
BEGIN
  SELECT RAISE(ABORT, 'tailscale route audit is append-only');
END;
CREATE TRIGGER library_access_tailscale_route_audit_no_delete
BEFORE DELETE ON library_access_tailscale_route_audit
BEGIN
  SELECT RAISE(ABORT, 'tailscale route audit is append-only');
END;
`); err != nil {
		return fmt.Errorf("reset untrusted Library security domain: %w", err)
	}
	return nil
}

func replaceAllowedLibraryMetadata(ctx context.Context, db *sql.DB, sourcePath string) (returnErr error) {
	connection, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable candidate foreign keys for logical restore: %w", err)
	}
	defer func() {
		if _, err := connection.ExecContext(context.Background(), "PRAGMA foreign_keys = ON"); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("re-enable candidate foreign keys: %w", err)
		}
	}()
	if _, err := connection.ExecContext(ctx, "ATTACH DATABASE ? AS restore_source", sourcePath); err != nil {
		return fmt.Errorf("attach migrated restore source: %w", err)
	}
	defer connection.ExecContext(context.Background(), "DETACH DATABASE restore_source")

	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orphanedGrantCount int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM main.library_device_grants AS current_grant
LEFT JOIN restore_source.library_catalogs AS restored_catalog
  ON restored_catalog.id = current_grant.catalog_id
WHERE restored_catalog.id IS NULL
`).Scan(&orphanedGrantCount); err != nil {
		return fmt.Errorf("validate current device-grant catalog identities: %w", err)
	}
	if orphanedGrantCount > 0 {
		return fmt.Errorf(
			"logical Library restore rejected: %d current device grant(s) reference catalog identities absent from the backup",
			orphanedGrantCount,
		)
	}

	// Preserve only current device-grant change-feed entries. Grant rows,
	// settings, route state, and route audit remain in main throughout.
	if _, err := tx.ExecContext(ctx, `
CREATE TEMP TABLE restore_current_grant_changes AS
SELECT sequence, catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
FROM library_catalog_changes
WHERE entity_type = 'device_grant';
`); err != nil {
		return fmt.Errorf("preserve current device-grant changes: %w", err)
	}

	for _, table := range clearedRuntimeTables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM main."+table); err != nil {
			return fmt.Errorf("clear non-restorable table %s: %w", table, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM main.library_catalog_tombstones"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM main.library_catalog_changes"); err != nil {
		return err
	}

	// Child-first deletion prevents user-defined triggers from observing a
	// half-old graph even though FK enforcement is temporarily disabled.
	for index := len(restoredLibraryTables) - 1; index >= 0; index-- {
		table := restoredLibraryTables[index]
		if _, err := tx.ExecContext(ctx, "DELETE FROM main."+table); err != nil {
			return fmt.Errorf("clear restorable table %s: %w", table, err)
		}
	}
	// Foreign keys are intentionally disabled for the graph replacement, so
	// deleting Catalog rows does not cascade into the non-restorable sync table.
	// Clear it explicitly before Catalog INSERT triggers allocate provisional
	// epochs; the final rotation below remains the authoritative generation.
	if _, err := tx.ExecContext(ctx, "DELETE FROM main.library_catalog_sync_state"); err != nil {
		return fmt.Errorf("clear current Catalog sync state: %w", err)
	}

	copyTables := []string{
		"library_libraries",
		"library_files",
		"library_subtitle_documents",
		"transcode_presets",
		"library_operations",
		"library_operation_outputs",
		"library_operation_chunks",
		"library_history_records",
		"library_history_files",
		"library_workspace_states",
		"library_workspace_state_head",
		"library_file_events",
		"library_import_batches",
		"library_import_candidates",
		"listen_local_tracks",
		"listen_local_playlists",
		"listen_local_playlist_items",
		"listen_local_music_memberships",
		"listen_local_music_tombstones",
		"listen_local_music_changes",
		"listen_local_music_mutation_receipts",
		"listen_local_music_track_states",
		"listen_local_music_lyric_documents",
		"listen_local_music_lyric_selections",
		"listen_local_music_play_sessions",
		"listen_local_music_play_event_checkpoints",
		"listen_local_music_play_event_receipts",
		"listen_live_columns",
		"listen_live_channels",
		"library_catalogs",
		"library_catalog_items",
		"library_storage_roots",
		"library_collections",
		"library_collection_items",
		"library_tags",
		"library_item_tags",
		"library_user_states",
		"library_item_assets",
		"rss_workspaces",
		"rss_categories",
		"rss_collections",
		"rss_subscriptions",
		"rss_subscription_field_revisions",
		"rss_subscription_history",
		"rss_entries",
		"rss_entry_origins",
		"rss_entry_downloads",
		"rss_observation_sources",
		"rss_sources",
		"rss_collection_subscriptions",
		"rss_collection_entries",
		"rss_client_mutations",
		"rss_public_mutations",
		"rss_changes",
		"rss_tombstones",
	}
	for _, table := range copyTables {
		if err := copyRestoreTableByColumnName(ctx, tx, table); err != nil {
			return fmt.Errorf("restore allowlisted table %s: %w", table, err)
		}
	}
	// Item-asset insertion projects compatibility representation rows. Replace
	// those projections with the authoritative rows from the source snapshot.
	if _, err := tx.ExecContext(ctx, "DELETE FROM main.library_metadata_entries; DELETE FROM main.library_representations;"); err != nil {
		return fmt.Errorf("reset projected representation metadata: %w", err)
	}
	for _, table := range []string{
		"library_representations",
		"library_metadata_entries",
		"library_legacy_mappings",
		"library_migration_checkpoints",
	} {
		if err := copyRestoreTableByColumnName(ctx, tx, table); err != nil {
			return fmt.Errorf("restore allowlisted table %s: %w", table, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO main.library_catalog_changes (
  sequence, catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
)
SELECT sequence, catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
FROM restore_source.library_catalog_changes
WHERE entity_type <> 'device_grant'
ORDER BY sequence;

INSERT INTO main.library_catalog_tombstones (
  sequence, catalog_id, entity_type, entity_id, revision, deleted_at, expires_at
)
SELECT tombstone.sequence, tombstone.catalog_id, tombstone.entity_type,
       tombstone.entity_id, tombstone.revision, tombstone.deleted_at, tombstone.expires_at
FROM restore_source.library_catalog_tombstones AS tombstone
JOIN restore_source.library_catalog_changes AS change ON change.sequence = tombstone.sequence
WHERE change.entity_type <> 'device_grant'
ORDER BY tombstone.sequence;

INSERT INTO main.library_catalog_changes (
  catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
)
SELECT change.catalog_id, change.entity_type, change.entity_id, change.kind,
       change.revision, change.actor_id, change.occurred_at
FROM restore_current_grant_changes AS change
JOIN main.library_catalogs AS catalog ON catalog.id = change.catalog_id
ORDER BY change.sequence;
`); err != nil {
		return fmt.Errorf("restore catalog change feed without security rollback: %w", err)
	}

	// A change-feed cursor is only meaningful within one generation. Never copy
	// an epoch from the backup or preserve the pre-restore epoch. RSS retention,
	// however, describes which rows physically exist in the restored journal and
	// must survive the rotation; otherwise after=0 would be accepted against a
	// pruned change feed. Clamp untrusted/legacy metadata to the restored high
	// water and default a missing source row to zero.
	if _, err := tx.ExecContext(ctx, `
DELETE FROM main.library_catalog_sync_state;
INSERT INTO main.library_catalog_sync_state (catalog_id, epoch, rotated_at)
SELECT id, lower(hex(randomblob(16))), CURRENT_TIMESTAMP
FROM main.library_catalogs
ORDER BY id;

DELETE FROM main.rss_sync_state;
INSERT INTO main.rss_sync_state (workspace_id, epoch, rotated_at, retained_from)
SELECT workspace.id,
       lower(hex(randomblob(16))),
       CURRENT_TIMESTAMP,
       CASE
         WHEN COALESCE(source.retained_from, 0) < 0 THEN 0
         WHEN COALESCE(source.retained_from, 0) > COALESCE(journal.high_water, 0)
           THEN COALESCE(journal.high_water, 0)
         ELSE COALESCE(source.retained_from, 0)
       END
FROM main.rss_workspaces AS workspace
LEFT JOIN restore_source.rss_sync_state AS source
  ON source.workspace_id = workspace.id
LEFT JOIN (
  SELECT workspace_id, MAX(sequence) AS high_water
  FROM main.rss_changes
  GROUP BY workspace_id
) AS journal ON journal.workspace_id = workspace.id
ORDER BY workspace.id;

DELETE FROM main.listen_local_music_sync_state;
INSERT INTO main.listen_local_music_sync_state (
  id, epoch, high_water, minimum_cursor, updated_at
)
SELECT 1,
       lower(hex(randomblob(16))),
       COALESCE(journal.high_water, 0),
       CASE
         WHEN COALESCE(source.minimum_cursor, 0) < 0 THEN 0
         WHEN COALESCE(source.minimum_cursor, 0) > COALESCE(journal.high_water, 0)
           THEN COALESCE(journal.high_water, 0)
         ELSE COALESCE(source.minimum_cursor, 0)
       END,
       CURRENT_TIMESTAMP
FROM (SELECT 1 AS id) AS singleton
LEFT JOIN restore_source.listen_local_music_sync_state AS source ON source.id = singleton.id
LEFT JOIN (
  SELECT MAX(sequence) AS high_water
  FROM main.listen_local_music_changes
) AS journal ON TRUE;
`); err != nil {
		return fmt.Errorf("rotate Library, Music, and RSS sync epochs after logical restore: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit logical Library restore: %w", err)
	}
	if _, err := connection.ExecContext(ctx, "DETACH DATABASE restore_source"); err != nil {
		return fmt.Errorf("detach migrated restore source: %w", err)
	}
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("re-enable candidate foreign keys: %w", err)
	}
	var enabled int
	if err := connection.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
		if err == nil {
			err = errors.New("foreign key enforcement remained disabled")
		}
		return err
	}
	rows, err := connection.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("run restored foreign-key check: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowID sql.NullInt64
		var parent string
		var constraint int
		if err := rows.Scan(&table, &rowID, &parent, &constraint); err != nil {
			return err
		}
		return fmt.Errorf("restored foreign-key violation in %s row %v referencing %s", table, rowID, parent)
	}
	return rows.Err()
}

// copyRestoreTableByColumnName deliberately avoids INSERT ... SELECT *.
// SQLite preserves the physical column order produced by ALTER TABLE, so an
// upgraded backup can have a different ordinal layout from a newly created
// database at the same schema version. Mapping the current table's columns by
// name prevents values from being shifted into unrelated fields during a
// logical restore.
func copyRestoreTableByColumnName(ctx context.Context, tx *sql.Tx, table string) error {
	currentColumns, err := restoreTableColumns(ctx, tx, "main", table)
	if err != nil {
		return fmt.Errorf("inspect current table columns: %w", err)
	}
	sourceColumns, err := restoreTableColumns(ctx, tx, "restore_source", table)
	if err != nil {
		return fmt.Errorf("inspect source table columns: %w", err)
	}
	if len(currentColumns) == 0 || len(sourceColumns) == 0 {
		return errors.New("restore table has no writable columns")
	}
	if len(currentColumns) != len(sourceColumns) {
		return fmt.Errorf(
			"restore table column count differs: current=%d source=%d",
			len(currentColumns), len(sourceColumns),
		)
	}
	sourceSet := make(map[string]bool, len(sourceColumns))
	for _, column := range sourceColumns {
		if sourceSet[column] {
			return fmt.Errorf("restore source has duplicate column %q", column)
		}
		sourceSet[column] = true
	}
	quotedColumns := make([]string, 0, len(currentColumns))
	for _, column := range currentColumns {
		if !sourceSet[column] {
			return fmt.Errorf("restore source is missing current column %q", column)
		}
		quotedColumns = append(quotedColumns, quoteRestoreIdentifier(column))
	}
	columns := strings.Join(quotedColumns, ", ")
	statement := "INSERT INTO main." + quoteRestoreIdentifier(table) +
		" (" + columns + ") SELECT " + columns +
		" FROM restore_source." + quoteRestoreIdentifier(table)
	_, err = tx.ExecContext(ctx, statement)
	return err
}

func restoreTableColumns(ctx context.Context, tx *sql.Tx, schema, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT name
FROM pragma_table_xinfo(?, ?)
WHERE hidden = 0
ORDER BY cid
`, table, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		if column == "" {
			return nil, errors.New("restore table contains an unnamed column")
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}

func quoteRestoreIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func removeSQLiteArtifacts(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(candidate); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove restore work artifact %s: %w", filepath.Base(candidate), err)
		}
	}
	return nil
}
