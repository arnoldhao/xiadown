package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
	"go.uber.org/zap"
)

// sqliteApplicationID is "XDWN" encoded as a big-endian 32-bit integer. It
// lets diagnostics distinguish a XiaDown database from an unrelated SQLite
// file before attempting any migration.
const sqliteApplicationID = 0x5844574e

const schemaMigrationsSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMP NOT NULL,
	duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0)
);
`

type sqliteMigration struct {
	version   int
	name      string
	signature string
	apply     func(context.Context, *sql.DB) error
	// applyTx is used for migrations whose schema change and ledger row must be
	// committed atomically. The v1 legacy baseline remains on apply because its
	// schema builder is deliberately idempotent and includes connection-scoped
	// compatibility repairs that cannot run inside one transaction.
	applyTx func(context.Context, *sql.Tx) error
}

type sqliteSchemaIdentity struct {
	applicationID int64
	userVersion   int
}

func (migration sqliteMigration) checksum() string {
	digest := sha256.Sum256([]byte(migration.signature))
	return hex.EncodeToString(digest[:])
}

// The initial migration records the existing idempotent schema builder as a
// baseline. Future changes should be appended as numbered migrations instead
// of changing this signature or an already-released migration.
var sqliteMigrations = []sqliteMigration{
	{
		version:   1,
		name:      "legacy_schema_baseline",
		signature: "v1:applySchema+ensureSQLiteColumns+librarySchema+memoryChunksFTS",
		apply:     applySchema,
	},
	{
		version:   2,
		name:      "library_catalog_foundation",
		signature: catalogSchemaSQL,
		apply:     applyCatalogSchema,
		applyTx:   sqliteMigrationSQL(catalogSchemaSQL, "apply catalog schema"),
	},
	{
		version:   3,
		name:      "library_access_foundation",
		signature: libraryAccessSchemaSQL,
		apply:     applyLibraryAccessSchema,
		applyTx:   sqliteMigrationSQL(libraryAccessSchemaSQL, "apply library access schema"),
	},
	{
		version:   4,
		name:      "library_device_grant_management",
		signature: libraryDeviceGrantManagementSchemaSQL,
		apply:     applyLibraryDeviceGrantManagementSchema,
		applyTx:   sqliteMigrationSQL(libraryDeviceGrantManagementSchemaSQL, "apply library device grant management schema"),
	},
	{
		version:   5,
		name:      "library_representation_metadata_foundation",
		signature: representationMetadataSchemaSQL + catalogChangeEntityExpansionSQL,
		apply:     applyRepresentationMetadataSchema,
		applyTx:   applyRepresentationMetadataSchemaTx,
	},
	{
		version:   6,
		name:      "library_import_batches",
		signature: libraryImportSchemaSQL,
		apply:     applyLibraryImportSchema,
		applyTx:   sqliteMigrationSQL(libraryImportSchemaSQL, "apply library import schema"),
	},
	{
		version:   7,
		name:      "library_tailscale_route_ownership",
		signature: libraryTailscaleRouteSchemaSQL,
		apply:     applyLibraryTailscaleRouteSchema,
		applyTx:   sqliteMigrationSQL(libraryTailscaleRouteSchemaSQL, "apply library tailscale route schema"),
	},
	{
		version:   8,
		name:      "library_catalog_sync_epoch",
		signature: libraryCatalogSyncStateSchemaSQL,
		apply:     applyLibraryCatalogSyncStateSchema,
		applyTx:   sqliteMigrationSQL(libraryCatalogSyncStateSchemaSQL, "apply library catalog sync state schema"),
	},
	{
		version:   9,
		name:      "rss_station_foundation",
		signature: rssSchemaSQL,
		apply:     applyRSSSchema,
		applyTx:   sqliteMigrationSQL(rssSchemaSQL, "apply RSS schema"),
	},
	{
		version:   10,
		name:      "rss_discovery_catalog",
		signature: rssDiscoverySchemaSQL,
		apply:     applyRSSDiscoverySchema,
		applyTx:   sqliteMigrationSQL(rssDiscoverySchemaSQL, "apply RSS discovery schema"),
	},
	{
		version:   11,
		name:      "rss_discovery_parameters",
		signature: rssDiscoveryParametersSchemaSQL,
		apply:     applyRSSDiscoveryParametersSchema,
		applyTx:   sqliteMigrationSQL(rssDiscoveryParametersSchemaSQL, "apply RSS discovery parameters migration"),
	},
	{
		version:   12,
		name:      "rss_public_sync_v2",
		signature: rssSyncV2SchemaSQL,
		apply:     applyRSSSyncV2Schema,
		applyTx:   sqliteMigrationSQL(rssSyncV2SchemaSQL, "apply RSS public sync v2 schema"),
	},
	{
		version:   13,
		name:      "rss_validator_provenance",
		signature: rssValidatorProvenanceSchemaSQL,
		apply:     applyRSSValidatorProvenanceSchema,
		applyTx:   sqliteMigrationSQL(rssValidatorProvenanceSchemaSQL, "apply RSS validator provenance schema"),
	},
	{
		version:   14,
		name:      "rss_starred_unread_index",
		signature: rssStarredIndexSchemaSQL,
		apply:     applyRSSStarredIndexSchema,
		applyTx:   sqliteMigrationSQL(rssStarredIndexSchemaSQL, "apply RSS starred index migration"),
	},
	{
		version:   15,
		name:      "rss_subscription_history",
		signature: rssHistorySchemaSQL,
		apply:     applyRSSHistorySchema,
		applyTx:   sqliteMigrationSQL(rssHistorySchemaSQL, "apply RSS history schema"),
	},
	{
		version:   16,
		name:      "app_session_storage_v1",
		signature: appSessionStorageSchemaSQL,
		apply:     applyAppSessionStorageSchema,
		applyTx:   sqliteMigrationSQL(appSessionStorageSchemaSQL, "apply App Session storage schema"),
	},
	{
		version:   17,
		name:      "rss_organization_and_sources",
		signature: rssOrganizationSchemaSQL,
		apply:     applyRSSOrganizationSchema,
		applyTx:   sqliteMigrationSQL(rssOrganizationSchemaSQL, "apply RSS organization schema"),
	},
	{
		version:   18,
		name:      "rss_collection_item_limits",
		signature: rssCollectionLimitsSchemaSQL,
		apply:     applyRSSCollectionLimitsSchema,
		applyTx:   applyRSSCollectionLimitsSchemaTx,
	},
	{
		version:   19,
		name:      "library_history_stable_operation_subject",
		signature: libraryHistoryStableOperationSubjectSQL,
		apply:     applyLibraryHistoryStableOperationSubject,
		applyTx:   applyLibraryHistoryStableOperationSubjectTx,
	},
	{
		version:   20,
		name:      "library_history_operation_events",
		signature: libraryHistoryOperationEventSchemaSQL,
		apply:     applyLibraryHistoryOperationEventSchema,
		applyTx:   applyLibraryHistoryOperationEventSchemaTx,
	},
	{
		version:   21,
		name:      "library_file_event_stable_subject",
		signature: libraryFileEventStableSubjectSchemaSQL,
		apply:     applyLibraryFileEventStableSubjectSchema,
		applyTx:   applyLibraryFileEventStableSubjectSchemaTx,
	},
	{
		version:   22,
		name:      "library_history_operation_kind",
		signature: libraryHistoryOperationKindSchemaSQL,
		apply:     applyLibraryHistoryOperationKindSchema,
		applyTx:   applyLibraryHistoryOperationKindSchemaTx,
	},
	{
		version:   23,
		name:      "library_history_operation_event_kind_backfill",
		signature: libraryHistoryOperationEventKindBackfillSQL,
		apply:     applyLibraryHistoryOperationEventKindBackfill,
		applyTx:   sqliteMigrationSQL(libraryHistoryOperationEventKindBackfillSQL, "backfill Library operation-event kind"),
	},
	{
		version:   24,
		name:      "listen_local_music_sync_foundation",
		signature: listenLocalMusicSyncFoundationPreSQL + "uuid-backfill-v1" + listenLocalMusicSyncFoundationPostSQL,
		apply:     applyListenLocalMusicSyncFoundation,
		applyTx:   applyListenLocalMusicSyncFoundationTx,
	},
	{
		version:   25,
		name:      "listen_local_music_sync_state",
		signature: "track-metadata-resource-revisions-v1" + listenLocalMusicSyncStateSchemaSQL,
		apply:     applyListenLocalMusicSyncStateSchema,
		applyTx:   applyListenLocalMusicSyncStateSchemaTx,
	},
	{
		version:   26,
		name:      "listen_local_music_write_protocol",
		signature: listenLocalMusicWriteSchemaSQL,
		apply:     applyListenLocalMusicWriteSchema,
		applyTx:   applyListenLocalMusicWriteSchemaTx,
	},
	{
		version:   27,
		name:      "rss_shared_public_protocol",
		signature: rssSharedPublicSchemaSQL,
		apply:     applyRSSSharedPublicSchema,
		applyTx:   applyRSSSharedPublicSchemaTx,
	},
	{
		version:   28,
		name:      "listen_local_music_content_identity",
		signature: listenLocalMusicContentIdentitySchemaSQL,
		apply:     applyListenLocalMusicContentIdentitySchema,
		applyTx:   applyListenLocalMusicContentIdentitySchemaTx,
	},
	{
		version:   29,
		name:      "library_catalog_snapshot_index",
		signature: libraryCatalogSnapshotIndexSchemaSQL,
		apply:     applyLibraryCatalogSnapshotIndexSchema,
		applyTx:   sqliteMigrationSQL(libraryCatalogSnapshotIndexSchemaSQL, "apply Library Catalog snapshot index migration"),
	},
	{
		version:   30,
		name:      "listen_local_music_projection_epoch",
		signature: listenLocalMusicProjectionEpochSchemaSQL,
		apply:     applyListenLocalMusicProjectionEpochSchema,
		applyTx:   sqliteMigrationSQL(listenLocalMusicProjectionEpochSchemaSQL, "rotate Listen Local Music projection epoch"),
	},
	{
		version:   31,
		name:      "listen_local_music_legacy_resource_epoch",
		signature: listenLocalMusicLegacyResourceEpochSchemaSQL,
		apply:     applyListenLocalMusicLegacyResourceEpochSchema,
		applyTx:   sqliteMigrationSQL(listenLocalMusicLegacyResourceEpochSchemaSQL, "rotate Listen Local Music legacy resource epoch"),
	},
	{
		version:   32,
		name:      "library_storage_root_ownership",
		signature: libraryStorageRootSchemaSQL,
		apply:     applyLibraryStorageRootSchema,
		applyTx:   applyLibraryStorageRootSchemaTx,
	},
	{
		version:   33,
		name:      "library_storage_root_sync",
		signature: libraryRootSyncSchemaSQL,
		apply:     applyLibraryRootSyncSchema,
		applyTx:   sqliteMigrationSQL(libraryRootSyncSchemaSQL, "apply Library storage root sync schema"),
	},
	{
		version:   34,
		name:      "library_storage_root_emoji",
		signature: libraryStorageRootEmojiSchemaSQL,
		apply:     applyLibraryStorageRootEmojiSchema,
		applyTx:   applyLibraryStorageRootEmojiSchemaTx,
	},
	{
		version:   35,
		name:      "library_storage_root_sync_candidate_index",
		signature: libraryRootSyncCandidateIndexSQL,
		apply:     applyLibraryRootSyncCandidateIndex,
		applyTx:   sqliteMigrationSQL(libraryRootSyncCandidateIndexSQL, "apply Library storage root sync candidate index"),
	},
	{
		version:   36,
		name:      "library_storage_root_sync_availability_indexes",
		signature: libraryRootSyncAvailabilityIndexSQL,
		apply:     applyLibraryRootSyncAvailabilityIndex,
		applyTx:   sqliteMigrationSQL(libraryRootSyncAvailabilityIndexSQL, "apply Library storage root sync availability indexes"),
	},
	{
		version:   37,
		name:      "library_transient_root_import_cleanup",
		signature: libraryTransientRootImportCleanupSQL,
		apply:     applyLibraryTransientRootImportCleanup,
		applyTx:   applyLibraryTransientRootImportCleanupTx,
	},
}

func prepareAndApplySQLiteMigrations(
	ctx context.Context,
	db *sql.DB,
	databasePath string,
	databaseExisted bool,
	skipPreMigrationSnapshot bool,
) (string, error) {
	identity, err := validateSQLiteApplicationID(ctx, db)
	if err != nil {
		return "", err
	}
	if err := checkSQLiteIntegrity(ctx, db, true); err != nil {
		return "", fmt.Errorf("sqlite pre-migration check: %w", err)
	}

	pending, err := pendingSQLiteMigrations(ctx, db)
	if err != nil {
		return "", err
	}
	if len(pending) == 0 {
		latestVersion := latestSQLiteMigrationVersion()
		identityRepaired := identity.applicationID != sqliteApplicationID || identity.userVersion != latestVersion
		if identityRepaired {
			if err := setSQLiteSchemaIdentity(ctx, db); err != nil {
				return "", err
			}
		}
		// The quick structural/foreign-key preflight and full migration-ledger
		// validation above remain mandatory on every open. With no schema change,
		// a second full integrity_check cannot validate any new state, and writing
		// unchanged application_id/user_version values only dirties the database.
		return "", nil
	}

	var snapshotPath string
	if databaseExisted && len(pending) > 0 && !skipPreMigrationSnapshot {
		snapshotPath, err = createSQLiteMigrationSnapshot(ctx, db, databasePath, pending[len(pending)-1].version)
		if err != nil {
			return "", fmt.Errorf("create sqlite pre-migration snapshot: %w", err)
		}
	}

	if _, err := db.ExecContext(ctx, schemaMigrationsSQL); err != nil {
		return snapshotPath, fmt.Errorf("create schema migration ledger: %w", err)
	}
	for _, migration := range pending {
		startedAt := time.Now()
		if migration.applyTx != nil {
			if err := applyAndRecordSQLiteMigration(ctx, db, migration, startedAt); err != nil {
				return snapshotPath, err
			}
			continue
		}
		// Only the legacy v1 baseline uses this path. Its builder is idempotent,
		// so a process interruption before the ledger write safely replays it.
		if migration.apply == nil {
			return snapshotPath, fmt.Errorf("apply sqlite migration %d (%s): missing migration callback", migration.version, migration.name)
		}
		if err := migration.apply(ctx, db); err != nil {
			return snapshotPath, fmt.Errorf("apply sqlite migration %d (%s): %w", migration.version, migration.name, err)
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO schema_migrations (version, name, checksum, applied_at, duration_ms)
VALUES (?, ?, ?, ?, ?)
`, migration.version, migration.name, migration.checksum(), time.Now().UTC(), time.Since(startedAt).Milliseconds()); err != nil {
			return snapshotPath, fmt.Errorf("record sqlite migration %d (%s): %w", migration.version, migration.name, err)
		}
	}

	if err := setSQLiteSchemaIdentity(ctx, db); err != nil {
		return snapshotPath, err
	}
	if err := checkSQLiteIntegrity(ctx, db, false); err != nil {
		return snapshotPath, fmt.Errorf("sqlite post-migration check: %w", err)
	}
	if err := recordSQLiteFullIntegrityCheckSuccess(databasePath, time.Now()); err != nil {
		// The database has already passed its mandatory post-migration check. A
		// sidecar timestamp failure only means the deferred checker may run again.
		zap.L().Warn("sqlite: record post-migration integrity check", zap.Error(err))
	}
	return snapshotPath, nil
}

func sqliteMigrationSQL(statement, label string) func(context.Context, *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		return nil
	}
}

func applyAndRecordSQLiteMigration(
	ctx context.Context,
	db *sql.DB,
	migration sqliteMigration,
	startedAt time.Time,
) error {
	if migration.applyTx == nil {
		return fmt.Errorf("apply sqlite migration %d (%s): missing transactional migration callback", migration.version, migration.name)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migration %d (%s): %w", migration.version, migration.name, err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := migration.applyTx(ctx, tx); err != nil {
		return fmt.Errorf("apply sqlite migration %d (%s): %w", migration.version, migration.name, err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO schema_migrations (version, name, checksum, applied_at, duration_ms)
VALUES (?, ?, ?, ?, ?)
`, migration.version, migration.name, migration.checksum(), time.Now().UTC(), time.Since(startedAt).Milliseconds()); err != nil {
		return fmt.Errorf("record sqlite migration %d (%s): %w", migration.version, migration.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration %d (%s): %w", migration.version, migration.name, err)
	}
	return nil
}

func pendingSQLiteMigrations(ctx context.Context, db *sql.DB) ([]sqliteMigration, error) {
	hasLedger, err := sqliteTableExists(ctx, db, "schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("inspect schema migration ledger: %w", err)
	}
	if !hasLedger {
		return append([]sqliteMigration(nil), sqliteMigrations...), nil
	}

	rows, err := db.QueryContext(ctx, "SELECT version, name, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read schema migration ledger: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]struct{})
	for rows.Next() {
		var version int
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			return nil, fmt.Errorf("scan schema migration ledger: %w", err)
		}
		migration, ok := sqliteMigrationByVersion(version)
		if !ok {
			return nil, fmt.Errorf("database schema version %d is newer than this XiaDown build", version)
		}
		if name != migration.name {
			return nil, fmt.Errorf("sqlite migration %d name mismatch: database has %q, expected %q", version, name, migration.name)
		}
		if checksum != migration.checksum() {
			return nil, fmt.Errorf("sqlite migration %d (%s) checksum mismatch", version, name)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema migration ledger: %w", err)
	}

	pending := make([]sqliteMigration, 0, len(sqliteMigrations)-len(applied))
	for _, migration := range sqliteMigrations {
		if _, ok := applied[migration.version]; !ok {
			pending = append(pending, migration)
		}
	}
	return pending, nil
}

func sqliteMigrationByVersion(version int) (sqliteMigration, bool) {
	for _, migration := range sqliteMigrations {
		if migration.version == version {
			return migration, true
		}
	}
	return sqliteMigration{}, false
}

func validateSQLiteApplicationID(ctx context.Context, db *sql.DB) (sqliteSchemaIdentity, error) {
	var applicationID int64
	if err := db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		return sqliteSchemaIdentity{}, fmt.Errorf("read sqlite application_id: %w", err)
	}
	if applicationID != 0 && applicationID != sqliteApplicationID {
		return sqliteSchemaIdentity{}, fmt.Errorf("sqlite application_id %d does not identify a XiaDown database", applicationID)
	}

	var userVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		return sqliteSchemaIdentity{}, fmt.Errorf("read sqlite user_version: %w", err)
	}
	if userVersion > len(sqliteMigrations) {
		return sqliteSchemaIdentity{}, fmt.Errorf("database user_version %d is newer than supported version %d", userVersion, len(sqliteMigrations))
	}
	return sqliteSchemaIdentity{applicationID: applicationID, userVersion: userVersion}, nil
}

func latestSQLiteMigrationVersion() int {
	if len(sqliteMigrations) == 0 {
		return 0
	}
	return sqliteMigrations[len(sqliteMigrations)-1].version
}

func setSQLiteSchemaIdentity(ctx context.Context, db *sql.DB) error {
	latestVersion := latestSQLiteMigrationVersion()
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", sqliteApplicationID)); err != nil {
		return fmt.Errorf("set sqlite application_id: %w", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", latestVersion)); err != nil {
		return fmt.Errorf("set sqlite user_version: %w", err)
	}
	return nil
}

func checkSQLiteIntegrity(ctx context.Context, db *sql.DB, quick bool) error {
	pragma := "PRAGMA integrity_check"
	if quick {
		pragma = "PRAGMA quick_check"
	}
	rows, err := db.QueryContext(ctx, pragma)
	if err != nil {
		return fmt.Errorf("run %s: %w", strings.TrimPrefix(pragma, "PRAGMA "), err)
	}
	defer rows.Close()
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(result), "ok") {
			return fmt.Errorf("%s reported: %s", strings.TrimPrefix(pragma, "PRAGMA "), result)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	foreignRows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("run foreign_key_check: %w", err)
	}
	defer foreignRows.Close()
	if foreignRows.Next() {
		return fmt.Errorf("foreign_key_check reported at least one violation")
	}
	return foreignRows.Err()
}

func sqliteDatabaseHasContent(databasePath string) (bool, error) {
	if databasePath == ":memory:" || strings.HasPrefix(databasePath, "file:") {
		return false, nil
	}
	info, err := os.Stat(databasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat sqlite database: %w", err)
	}
	return info.Size() > 0, nil
}

func createSQLiteMigrationSnapshot(ctx context.Context, db *sql.DB, databasePath string, targetVersion int) (string, error) {
	if databasePath == ":memory:" || strings.HasPrefix(databasePath, "file:") {
		return "", nil
	}
	timestamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	base := filepath.Base(databasePath)
	target := filepath.Join(
		filepath.Dir(databasePath),
		fmt.Sprintf("%s.pre-migration-v%d-%s.bak", base, targetVersion, timestamp),
	)
	// Use SQLite's online backup API rather than VACUUM INTO. The latter fails
	// with "SQL statements in progress" when the pooled connection has prepared
	// schema/foreign-key inspection statements, which is common for real legacy
	// databases. Online backup is transactionally consistent and includes
	// committed WAL content without depending on pool statement finalization.
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if err := conn.Raw(func(raw any) error {
		driverConn, ok := raw.(sqlite3driver.Conn)
		if !ok {
			return fmt.Errorf("unexpected SQLite driver connection %T", raw)
		}
		return driverConn.Raw().Backup("main", target)
	}); err != nil {
		_ = os.Remove(target)
		return "", err
	}
	if err := os.Chmod(target, 0o600); err != nil {
		_ = os.Remove(target)
		return "", fmt.Errorf("secure sqlite migration snapshot: %w", err)
	}
	return target, nil
}
