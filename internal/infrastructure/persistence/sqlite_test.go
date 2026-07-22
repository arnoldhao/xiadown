package persistence

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	sqlite3 "github.com/ncruces/go-sqlite3"
	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

func TestOpenSQLiteRecordsMigrationIdentityAndSnapshotsLegacyDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	rawDB, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	if _, err := rawDB.ExecContext(ctx, `
CREATE TABLE legacy_marker (value TEXT NOT NULL);
INSERT INTO legacy_marker (value) VALUES ('preserve-me');
`); err != nil {
		_ = rawDB.Close()
		t.Fatalf("seed legacy database: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw sqlite: %v", err)
	}

	db, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	if db.MigrationSnapshotPath == "" {
		t.Fatal("legacy migration did not create a snapshot")
	}
	if _, err := os.Stat(db.MigrationSnapshotPath); err != nil {
		t.Fatalf("stat migration snapshot: %v", err)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(path); err != nil {
			t.Fatalf("stat database permissions: %v", err)
		} else if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("database permissions = %v; want owner-only", info.Mode().Perm())
		}
		if info, err := os.Stat(db.MigrationSnapshotPath); err != nil {
			t.Fatalf("stat snapshot permissions: %v", err)
		} else if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("snapshot permissions = %v; want owner-only", info.Mode().Perm())
		}
	}

	var (
		version    int
		name       string
		checksum   string
		appliedAt  string
		durationMS int64
	)
	if err := db.SQL.QueryRowContext(ctx, `
SELECT version, name, checksum, applied_at, duration_ms
FROM schema_migrations
WHERE version = 1
`).Scan(&version, &name, &checksum, &appliedAt, &durationMS); err != nil {
		t.Fatalf("read migration ledger: %v", err)
	}
	if version != 1 || name != "legacy_schema_baseline" || len(checksum) != 64 || appliedAt == "" || durationMS < 0 {
		t.Fatalf("unexpected migration ledger row: version=%d name=%q checksum=%q applied_at=%q duration_ms=%d", version, name, checksum, appliedAt, durationMS)
	}

	var applicationID, userVersion int
	if err := db.SQL.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		t.Fatalf("read application_id: %v", err)
	}
	if err := db.SQL.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	latestVersion := sqliteMigrations[len(sqliteMigrations)-1].version
	if applicationID != sqliteApplicationID || userVersion != latestVersion {
		t.Fatalf("schema identity = (%d, %d), want (%d, %d)", applicationID, userVersion, sqliteApplicationID, latestVersion)
	}

	snapshotDB, err := sql.Open("sqlite3", db.MigrationSnapshotPath)
	if err != nil {
		t.Fatalf("open migration snapshot: %v", err)
	}
	var marker string
	if err := snapshotDB.QueryRowContext(ctx, "SELECT value FROM legacy_marker").Scan(&marker); err != nil {
		_ = snapshotDB.Close()
		t.Fatalf("read legacy data from migration snapshot: %v", err)
	}
	if marker != "preserve-me" {
		t.Fatalf("snapshot marker = %q, want preserve-me", marker)
	}
	if err := snapshotDB.Close(); err != nil {
		t.Fatalf("close migration snapshot: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close migrated database: %v", err)
	}

	reopened, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen migrated database: %v", err)
	}
	defer reopened.Close()
	if reopened.MigrationSnapshotPath != "" {
		t.Fatalf("reopen unexpectedly created snapshot %q", reopened.MigrationSnapshotPath)
	}
	matches, err := filepath.Glob(path + ".pre-migration-*.bak")
	if err != nil {
		t.Fatalf("glob migration snapshots: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("snapshot count = %d, want 1", len(matches))
	}
}

func TestStableSQLiteMigrationFastPathAvoidsWritesAndFullIntegrityCheck(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "stable-fast-path.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	type pragmaCall struct{ name, argument string }
	var (
		mu      sync.Mutex
		pragmas []pragmaCall
	)
	tracedDB, err := sqlite3driver.Open(path, func(conn *sqlite3.Conn) error {
		return conn.SetAuthorizer(func(action sqlite3.AuthorizerActionCode, name, argument, _, _ string) sqlite3.AuthorizerReturnCode {
			if action == sqlite3.AUTH_PRAGMA {
				mu.Lock()
				pragmas = append(pragmas, pragmaCall{name: strings.ToLower(name), argument: argument})
				mu.Unlock()
			}
			return sqlite3.AUTH_OK
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tracedDB.Close()

	if _, err := prepareAndApplySQLiteMigrations(ctx, tracedDB, path, true, false); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	calls := append([]pragmaCall(nil), pragmas...)
	mu.Unlock()
	var sawQuickCheck, sawForeignKeyCheck bool
	for _, call := range calls {
		switch call.name {
		case "quick_check":
			sawQuickCheck = true
		case "foreign_key_check":
			sawForeignKeyCheck = true
		case "integrity_check":
			t.Fatalf("stable fast path ran full integrity_check: %#v", calls)
		case "application_id", "user_version":
			if call.argument != "" {
				t.Fatalf("stable fast path rewrote %s=%q: %#v", call.name, call.argument, calls)
			}
		}
	}
	if !sawQuickCheck || !sawForeignKeyCheck {
		t.Fatalf("stable fast path omitted preflight: quick=%t foreign_keys=%t calls=%#v", sawQuickCheck, sawForeignKeyCheck, calls)
	}
}

func TestStableSQLiteMigrationFastPathRepairsSchemaIdentity(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "stable-identity-repair.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, "PRAGMA application_id = 0; PRAGMA user_version = 0;"); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var applicationID, userVersion int
	if err := reopened.SQL.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		t.Fatal(err)
	}
	if err := reopened.SQL.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if applicationID != sqliteApplicationID || userVersion != latestSQLiteMigrationVersion() {
		t.Fatalf(
			"repaired schema identity = (%d, %d), want (%d, %d)",
			applicationID, userVersion, sqliteApplicationID, latestSQLiteMigrationVersion(),
		)
	}
}

func TestPendingSQLiteMigrationRetainsFullPostIntegrityCheck(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "pending-post-check.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	latestVersion := latestSQLiteMigrationVersion()
	if _, err := database.SQL.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = ?", latestVersion); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	var (
		mu      sync.Mutex
		pragmas []string
	)
	tracedDB, err := sqlite3driver.Open(path, func(conn *sqlite3.Conn) error {
		return conn.SetAuthorizer(func(action sqlite3.AuthorizerActionCode, name, _, _, _ string) sqlite3.AuthorizerReturnCode {
			if action == sqlite3.AUTH_PRAGMA {
				mu.Lock()
				pragmas = append(pragmas, strings.ToLower(name))
				mu.Unlock()
			}
			return sqlite3.AUTH_OK
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tracedDB.Close()
	if _, err := prepareAndApplySQLiteMigrations(ctx, tracedDB, path, true, true); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	calls := append([]string(nil), pragmas...)
	mu.Unlock()
	if !slices.Contains(calls, "quick_check") || !slices.Contains(calls, "integrity_check") {
		t.Fatalf("pending migration checks = %v, want quick_check and integrity_check", calls)
	}
}

func TestSQLiteFullIntegrityCheckUsesLowFrequencySuccessMarker(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "periodic-integrity.db")
	now := time.Now()
	due, err := sqliteFullIntegrityCheckDue(databasePath, now)
	if err != nil || !due {
		t.Fatalf("missing marker due=%t err=%v, want due", due, err)
	}
	if err := recordSQLiteFullIntegrityCheckFailure(databasePath, errors.New("fixture failure"), now); err != nil {
		t.Fatal(err)
	}
	if err := recordSQLiteFullIntegrityCheckSuccess(databasePath, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sqliteFullIntegrityCheckFailureMarkerPath(databasePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failure marker survived successful check: %v", err)
	}
	due, err = sqliteFullIntegrityCheckDue(databasePath, now.Add(sqliteFullIntegrityCheckInterval-time.Minute))
	if err != nil || due {
		t.Fatalf("recent marker due=%t err=%v, want not due", due, err)
	}
	due, err = sqliteFullIntegrityCheckDue(databasePath, now.Add(sqliteFullIntegrityCheckInterval+time.Minute))
	if err != nil || !due {
		t.Fatalf("expired marker due=%t err=%v, want due", due, err)
	}
}

func TestSQLiteIntegrityStatusExposesDeferredCheckFailureToMaintenance(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "integrity-status.db")
	checkedAt := time.Date(2026, time.July, 20, 8, 9, 10, 11, time.UTC)
	if status := InspectSQLiteIntegrityStatus(databasePath); status.State != SQLiteIntegrityStatePending {
		t.Fatalf("initial integrity status = %#v, want pending", status)
	}
	if err := recordSQLiteFullIntegrityCheckSuccess(databasePath, checkedAt); err != nil {
		t.Fatal(err)
	}
	if status := InspectSQLiteIntegrityStatus(databasePath); status.State != SQLiteIntegrityStateHealthy || status.CheckedAt != checkedAt.Format(time.RFC3339Nano) {
		t.Fatalf("successful integrity status = %#v", status)
	}
	if err := recordSQLiteFullIntegrityCheckFailure(databasePath, errors.New("fixture integrity failure"), checkedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	status := InspectSQLiteIntegrityStatus(databasePath)
	if status.State != SQLiteIntegrityStateFailed || status.Detail != "fixture integrity failure" {
		t.Fatalf("failed integrity status = %#v", status)
	}
	// A failure must be retried on the next launch even if an earlier success
	// marker would otherwise suppress the weekly check.
	due, err := sqliteFullIntegrityCheckDue(databasePath, checkedAt.Add(2*time.Minute))
	if err != nil || !due {
		t.Fatalf("failed integrity status due=%t err=%v, want due", due, err)
	}
	if err := recordSQLiteFullIntegrityCheckSuccess(databasePath, checkedAt.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if status := InspectSQLiteIntegrityStatus(databasePath); status.State != SQLiteIntegrityStateHealthy {
		t.Fatalf("recovered integrity status = %#v", status)
	}
}

func TestSQLiteIntegrityMarkersNormalizeRelativeDatabasePath(t *testing.T) {
	relative := filepath.Join("testdata", "relative-integrity.db")
	want, err := filepath.Abs(relative + ".integrity-check")
	if err != nil {
		t.Fatal(err)
	}
	if got := sqliteFullIntegrityCheckMarkerPath(relative); got != want {
		t.Fatalf("normalized marker path = %q, want %q", got, want)
	}
}

func TestSQLiteIntegrityMarkerRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	databasePath := filepath.Join(t.TempDir(), "symlink-integrity.db")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	markerPath := sqliteFullIntegrityCheckFailureMarkerPath(databasePath)
	if err := os.Symlink(outside, markerPath); err != nil {
		t.Fatal(err)
	}
	if err := recordSQLiteFullIntegrityCheckFailure(databasePath, errors.New("must not overwrite"), time.Now()); err == nil {
		t.Fatal("symlink integrity marker was accepted")
	}
	if status := InspectSQLiteIntegrityStatus(databasePath); status.State != SQLiteIntegrityStateUnavailable {
		t.Fatalf("symlink integrity status = %#v, want unavailable", status)
	}
	if due, err := sqliteFullIntegrityCheckDue(databasePath, time.Now()); err != nil || !due {
		t.Fatalf("symlink failure marker due=%t err=%v, want forced recheck", due, err)
	}
	if contents, err := os.ReadFile(outside); err != nil || string(contents) != "preserve" {
		t.Fatalf("outside marker target changed: contents=%q err=%v", contents, err)
	}

	successDatabasePath := filepath.Join(t.TempDir(), "symlink-success.db")
	successMarkerPath := sqliteFullIntegrityCheckMarkerPath(successDatabasePath)
	if err := os.Symlink(outside, successMarkerPath); err != nil {
		t.Fatal(err)
	}
	if due, err := sqliteFullIntegrityCheckDue(successDatabasePath, time.Now()); err != nil || !due {
		t.Fatalf("symlink success marker due=%t err=%v, want forced recheck", due, err)
	}
	if status := InspectSQLiteIntegrityStatus(successDatabasePath); status.State != SQLiteIntegrityStateUnavailable {
		t.Fatalf("symlink success integrity status = %#v, want unavailable", status)
	}
}

func TestSQLiteFullIntegrityCheckCanRunDeferred(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "deferred-integrity.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	markerPath := sqliteFullIntegrityCheckMarkerPath(databasePath)
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("post-migration full check did not record success: %v", err)
	}
	if err := os.Remove(markerPath); err != nil {
		t.Fatal(err)
	}
	done := schedulePeriodicSQLiteFullIntegrityCheck(context.Background(), databasePath, database.SQL, 0)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(markerPath); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("deferred full integrity check did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("deferred integrity goroutine did not exit")
	}
}

func TestDatabaseCloseCancelsDeferredIntegrityCheckWithoutFailureMarker(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "cancel-deferred-integrity.db")
	database, err := OpenSQLite(ctx, SQLiteConfig{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(sqliteFullIntegrityCheckMarkerPath(databasePath)); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, SQLiteConfig{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	maintenanceDone := reopened.maintenanceDone
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-maintenanceDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Database.Close did not cancel deferred integrity check")
	}
	if _, err := os.Stat(sqliteFullIntegrityCheckFailureMarkerPath(databasePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled maintenance wrote failure marker: %v", err)
	}
}

func TestOpenSQLiteSkipsMigrationSnapshotOnlyWhenExplicitlyRequested(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "disposable-legacy.db")
	rawDB, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rawDB.ExecContext(ctx, `
CREATE TABLE disposable_marker (value TEXT NOT NULL);
INSERT INTO disposable_marker (value) VALUES ('preserve-through-migration');
`); err != nil {
		_ = rawDB.Close()
		t.Fatal(err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatal(err)
	}

	database, err := OpenSQLite(ctx, SQLiteConfig{
		Path: path, SkipPreMigrationSnapshot: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if database.MigrationSnapshotPath != "" {
		t.Fatalf("disposable migration snapshot = %q", database.MigrationSnapshotPath)
	}
	matches, err := filepath.Glob(path + ".pre-migration-v*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("disposable migration created snapshots: %v", matches)
	}
	var marker string
	if err := database.SQL.QueryRowContext(ctx, "SELECT value FROM disposable_marker").Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if marker != "preserve-through-migration" {
		t.Fatalf("disposable marker = %q", marker)
	}
}

func TestOpenSQLiteAddsCatalogFoundationWithoutChangingLegacyFiles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog-foundation.db")
	db, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer db.Close()

	_, err = db.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('legacy-bundle', 'Legacy bundle', '{}', '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z');
INSERT INTO library_files (
  id, library_id, kind, name, display_name, storage_mode, storage_local_path,
  origin_kind, origin_import_path, state_json, created_at, updated_at
) VALUES (
  'stable-file-id', 'legacy-bundle', 'video', 'movie.mp4', 'Movie', 'local_path',
  '/library/movie.mp4', 'import', '/library/movie.mp4', '{"status":"active"}',
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (
  'catalog-default', 'Library', '', 'active', TRUE,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, revision, created_at, updated_at
) VALUES (
  'item-movie', 'catalog-default', 'video', 'active', 'Movie', 'Movie', 1,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_item_assets (
  id, item_id, file_id, role, position, created_at, updated_at
) VALUES (
  'item-asset-movie', 'item-movie', 'stable-file-id', 'original', 0,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
`)
	if err != nil {
		t.Fatalf("seed additive catalog mapping: %v", err)
	}

	var (
		fileID      string
		legacyID    string
		catalogID   string
		itemAssetID string
	)
	err = db.SQL.QueryRowContext(ctx, `
SELECT f.id, f.library_id, i.catalog_id, ia.id
FROM library_files f
JOIN library_item_assets ia ON ia.file_id = f.id
JOIN library_catalog_items i ON i.id = ia.item_id
WHERE f.id = 'stable-file-id'
`).Scan(&fileID, &legacyID, &catalogID, &itemAssetID)
	if err != nil {
		t.Fatalf("read catalog mapping: %v", err)
	}
	if fileID != "stable-file-id" || legacyID != "legacy-bundle" || catalogID != "catalog-default" || itemAssetID != "item-asset-movie" {
		t.Fatalf("catalog mapping changed legacy identity: file=%q legacy=%q catalog=%q link=%q", fileID, legacyID, catalogID, itemAssetID)
	}

	var migrationCount int
	if err := db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationCount != len(sqliteMigrations) {
		t.Fatalf("migration count = %d, want %d", migrationCount, len(sqliteMigrations))
	}
}

func TestOpenSQLiteRejectsMigrationChecksumMismatch(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "checksum.db")
	db, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, "UPDATE schema_migrations SET checksum = 'tampered' WHERE version = 1"); err != nil {
		_ = db.Close()
		t.Fatalf("tamper migration checksum: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	_, err = OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("OpenSQLite error = %v, want checksum mismatch", err)
	}
}

func TestTransactionalSQLiteMigrationRollsBackSchemaWhenLedgerWriteFails(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "atomic-migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, schemaMigrationsSQL+`
CREATE TABLE atomic_migration_fixture (id INTEGER PRIMARY KEY);
INSERT INTO schema_migrations (version, name, checksum, applied_at, duration_ms)
VALUES (99, 'occupied', 'occupied', CURRENT_TIMESTAMP, 0);
`); err != nil {
		t.Fatal(err)
	}

	migration := sqliteMigration{
		version: 99, name: "atomic_fixture", signature: "atomic-fixture-v1",
		applyTx: sqliteMigrationSQL(
			"ALTER TABLE atomic_migration_fixture ADD COLUMN migrated TEXT NOT NULL DEFAULT '';",
			"apply atomic fixture",
		),
	}
	if err := applyAndRecordSQLiteMigration(ctx, db, migration, time.Now()); err == nil {
		t.Fatal("migration unexpectedly succeeded despite occupied ledger version")
	}
	var columns int
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_table_info('atomic_migration_fixture') WHERE name = 'migrated'
`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatalf("schema change survived failed ledger write: columns=%d", columns)
	}
}

func TestOpenSQLiteRejectsForeignKeyViolationBeforeMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "foreign-key-violation.db")
	rawDB, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	rawDB.SetMaxOpenConns(1)
	if _, err := rawDB.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		_ = rawDB.Close()
		t.Fatalf("disable foreign keys for corrupt fixture: %v", err)
	}
	if _, err := rawDB.ExecContext(ctx, `
CREATE TABLE parent (id INTEGER PRIMARY KEY);
CREATE TABLE child (
	id INTEGER PRIMARY KEY,
	parent_id INTEGER NOT NULL REFERENCES parent(id)
);
INSERT INTO child (id, parent_id) VALUES (1, 99);
`); err != nil {
		_ = rawDB.Close()
		t.Fatalf("seed foreign key violation: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw sqlite: %v", err)
	}

	_, err = OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err == nil || !strings.Contains(err.Error(), "foreign_key_check") {
		t.Fatalf("OpenSQLite error = %v, want foreign_key_check failure", err)
	}
	matches, globErr := filepath.Glob(path + ".pre-migration-*.bak")
	if globErr != nil {
		t.Fatalf("glob migration snapshots: %v", globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("invalid database created %d snapshots before passing preflight", len(matches))
	}
}

func TestOpenSQLiteMigratesLibraryFilesCompletedKinds(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data.db")
	rawDB, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	_, err = rawDB.ExecContext(ctx, `
CREATE TABLE library_libraries (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_by_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE TABLE library_files (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
	  kind TEXT NOT NULL CHECK (kind IN ('video','audio','subtitle','thumbnail','transcode','other')),
  name TEXT NOT NULL,
  metadata_json TEXT,
  display_name TEXT,

  storage_mode TEXT NOT NULL CHECK (storage_mode IN ('local_path','db_document','hybrid')),
  storage_local_path TEXT,
  storage_document_id TEXT,

  origin_kind TEXT NOT NULL CHECK (origin_kind IN ('import','download','transcode')),
  origin_operation_id TEXT,
  origin_import_batch_id TEXT,
  origin_import_path TEXT,
  origin_imported_at TIMESTAMP,
  origin_keep_source_file BOOLEAN,

  lineage_root_file_id TEXT,
  latest_operation_id TEXT,

  state_json TEXT NOT NULL,
  media_json TEXT,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,

  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (lineage_root_file_id) REFERENCES library_files(id) ON DELETE SET NULL,

  CHECK (
	    (kind IN ('video','audio','thumbnail','transcode','other') AND storage_mode IN ('local_path','hybrid') AND COALESCE(storage_local_path,'') <> '') OR
    (kind = 'subtitle' AND storage_mode IN ('db_document','hybrid') AND COALESCE(storage_document_id,'') <> '')
  ),
  CHECK (
    (origin_kind = 'import' AND COALESCE(origin_import_path,'') <> '' AND origin_operation_id IS NULL) OR
    (origin_kind IN ('download','transcode') AND COALESCE(origin_operation_id,'') <> '' AND origin_import_path IS NULL)
  )
);
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('lib-1', 'Library', '{}', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
INSERT INTO library_files (
  id,
  library_id,
  kind,
  name,
  metadata_json,
  display_name,
  storage_mode,
  storage_local_path,
  storage_document_id,
  origin_kind,
  origin_operation_id,
  origin_import_batch_id,
  origin_import_path,
  origin_imported_at,
  origin_keep_source_file,
  lineage_root_file_id,
  latest_operation_id,
  state_json,
  media_json,
  created_at,
  updated_at
) VALUES (
  'file-video',
  'lib-1',
  'video',
  'video',
  NULL,
  'video',
  'local_path',
  '/tmp/video.mp4',
  NULL,
  'download',
  'op-video',
  NULL,
  NULL,
  NULL,
  NULL,
  NULL,
  'op-video',
  '{"status":"active"}',
  NULL,
  '2026-01-01T00:00:00Z',
  '2026-01-01T00:00:00Z'
);
`)
	if err != nil {
		_ = rawDB.Close()
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw sqlite: %v", err)
	}

	db, err := OpenSQLite(ctx, SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer db.Close()

	for _, kind := range []string{"other", "document", "font", "api", "archive", "manifest"} {
		kind := kind
		_, err = db.SQL.ExecContext(ctx, `
	INSERT INTO library_files (
	  id,
	  library_id,
  kind,
  name,
  display_name,
  storage_mode,
  storage_local_path,
  origin_kind,
  origin_operation_id,
  latest_operation_id,
  state_json,
  created_at,
	  updated_at
	) VALUES (
	  ?,
	  'lib-1',
	  ?,
	  'payload',
	  'payload',
	  'local_path',
  '/tmp/payload.bin',
  'download',
  'op-other',
  'op-other',
	  '{"status":"active"}',
	  '2026-01-01T00:00:00Z',
	  '2026-01-01T00:00:00Z'
	)`, "file-"+kind, kind)
		if err != nil {
			t.Fatalf("insert %s file after migration: %v", kind, err)
		}
	}
}

func TestOpenSQLiteConfiguresEveryPooledConnection(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, SQLiteConfig{Path: filepath.Join(t.TempDir(), "pooled-connections.db")})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer db.Close()
	db.SQL.SetMaxOpenConns(2)

	// OpenSQLite's schema work leaves its first physical connection idle. Hold
	// that connection while acquiring the next one so the test must exercise a
	// newly opened pooled connection.
	conn1, err := db.SQL.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire first connection: %v", err)
	}
	defer conn1.Close()

	conn2, err := db.SQL.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire second connection: %v", err)
	}
	defer conn2.Close()

	for index, conn := range []*sql.Conn{conn1, conn2} {
		var foreignKeys int
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("read foreign_keys on connection %d: %v", index+1, err)
		}
		if foreignKeys != 1 {
			t.Fatalf("foreign_keys on connection %d = %d, want 1", index+1, foreignKeys)
		}

		var synchronous int
		if err := conn.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
			t.Fatalf("read synchronous on connection %d: %v", index+1, err)
		}
		if synchronous != 1 {
			t.Fatalf("synchronous on connection %d = %d, want NORMAL (1)", index+1, synchronous)
		}
	}

	seedStatements := []string{
		`
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('library-pool', 'Pool Test', '{}', '2026-07-11T00:00:00Z', '2026-07-11T00:00:00Z')`,
		`
INSERT INTO library_files (
  id, library_id, kind, name, display_name, storage_mode,
  storage_local_path, origin_kind, origin_import_path, state_json,
  created_at, updated_at
) VALUES (
  'track-pool', 'library-pool', 'audio', 'track.mp3', 'Track', 'local_path',
  '/tmp/track.mp3', 'import', '/tmp/track.mp3', '{"status":"active"}',
  '2026-07-11T00:00:00Z', '2026-07-11T00:00:00Z'
)`,
		`
INSERT INTO listen_local_tracks (
  file_id, library_id, local_path, title, mod_time_unix, availability,
  last_checked_at, created_at, updated_at
) VALUES (
  'track-pool', 'library-pool', '/tmp/track.mp3', 'Track', 0, 'available',
  '2026-07-11T00:00:00Z', '2026-07-11T00:00:00Z', '2026-07-11T00:00:00Z'
)`,
		`
INSERT INTO listen_local_playlists (id, name, created_at, updated_at)
VALUES ('playlist-pool', 'Pool Playlist', '2026-07-11T00:00:00Z', '2026-07-11T00:00:00Z')`,
		`
		INSERT INTO listen_local_playlist_items (
		  id, playlist_id, file_id, position, added_at, track_display_title
		)
		VALUES ('playlist-item-pool', 'playlist-pool', 'track-pool', 0, '2026-07-11T00:00:00Z', 'Track')`,
	}
	for _, statement := range seedStatements {
		if _, err := conn1.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed playlist on first connection: %v", err)
		}
	}

	if _, err := conn2.ExecContext(ctx, "DELETE FROM listen_local_playlists WHERE id = ?", "playlist-pool"); err != nil {
		t.Fatalf("delete playlist on second connection: %v", err)
	}
	var itemCount int
	if err := conn2.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM listen_local_playlist_items WHERE playlist_id = ?",
		"playlist-pool",
	).Scan(&itemCount); err != nil {
		t.Fatalf("count playlist items on second connection: %v", err)
	}
	if itemCount != 0 {
		t.Fatalf("playlist cascade on second connection left %d items, want 0", itemCount)
	}
}
