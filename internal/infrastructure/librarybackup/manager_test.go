package librarybackup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"

	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
	domainbackup "xiadown/internal/domain/librarybackup"
	domainrss "xiadown/internal/domain/rss"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/rssrepo"
)

type backupFixture struct {
	manager      *Manager
	database     *persistence.Database
	databasePath string
	backupDir    string
	markerPath   string
	now          *time.Time
}

func newBackupFixture(t *testing.T, retention domainbackup.RetentionPolicy) *backupFixture {
	t.Helper()
	ctx := context.Background()
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "data.db")
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: databasePath})
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('legacy-bundle', 'Legacy bundle', '{}', '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z');
INSERT INTO library_files (
  id, library_id, kind, name, display_name, storage_mode, storage_local_path,
  origin_kind, origin_import_path, state_json, created_at, updated_at
) VALUES (
  'file-movie', 'legacy-bundle', 'video', 'movie.mp4', 'Movie', 'local_path',
  '/Volumes/Library/Videos/Movie.mp4', 'import', '/private/source/Movie.mp4',
  '{"status":"active"}', '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
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
  'asset-movie', 'item-movie', 'file-movie', 'original', 0,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_storage_roots (
  id, catalog_id, name, path, volume_id, mode, status, created_at, updated_at
) VALUES (
  'root-main', 'catalog-default', 'Main', '/Volumes/Library', 'volume-secret',
  'referenced', 'online', '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_device_grants (
  id, catalog_id, device_id, device_name, credential_hash, public_key_hash,
  scopes_json, status, created_at, updated_at, revision
) VALUES (
  'grant-one', 'catalog-default', 'device-one', 'Phone', 'credential-hash-must-not-leak',
  'public-key-hash-must-not-leak', '["library.read"]', 'active',
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z', 1
);
`)
	if err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	backupDir := filepath.Join(directory, "backups")
	markerPath := filepath.Join(directory, "restore-marker.json")
	manager, err := NewManager(Config{
		DB: database.SQL, DatabasePath: databasePath, BackupDirectory: backupDir,
		RestoreMarkerPath: markerPath, AppName: "XiaDown", AppVersion: "2.0.0-test",
		Retention: retention, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return &backupFixture{
		manager: manager, database: database, databasePath: databasePath,
		backupDir: backupDir, markerPath: markerPath, now: &now,
	}
}

func TestCreateVerifyAndListMetadataBackup(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	manifest, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !manifest.MetadataOnly || manifest.ContentIncluded {
		t.Fatalf("content flags = metadataOnly:%t contentIncluded:%t", manifest.MetadataOnly, manifest.ContentIncluded)
	}
	if manifest.Database.ApplicationID != persistence.SQLiteApplicationID() ||
		manifest.Database.SchemaVersion != persistence.CurrentSQLiteSchemaVersion() {
		t.Fatalf("database identity = %+v", manifest.Database)
	}
	if len(manifest.Catalogs) != 1 || manifest.Catalogs[0].ID != "catalog-default" ||
		manifest.Catalogs[0].ItemCount != 1 || manifest.Catalogs[0].AssetCount != 1 {
		t.Fatalf("catalog inventory = %+v", manifest.Catalogs)
	}
	if len(manifest.Files) != 1 {
		t.Fatalf("file inventory count = %d", len(manifest.Files))
	}
	file := manifest.Files[0]
	if file.FileID != "file-movie" || file.AssetID != "asset-movie" || file.StorageRoot != "root-main" || file.RelativePath != "Videos/Movie.mp4" {
		t.Fatalf("file inventory = %+v", file)
	}
	manifestPath := filepath.Join(fixture.backupDir, backupPrefix+manifest.BackupID+manifestSuffix)
	manifestJSON, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	for _, forbidden := range []string{
		"/Volumes/Library", "/private/source", "volume-secret",
		"credential-hash-must-not-leak", "public-key-hash-must-not-leak",
	} {
		if strings.Contains(string(manifestJSON), forbidden) {
			t.Fatalf("manifest leaked forbidden value %q", forbidden)
		}
	}
	verification, err := fixture.manager.Verify(ctx, manifest.BackupID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !verification.Valid || verification.DatabaseSHA256 != manifest.Database.SHA256 {
		t.Fatalf("verification = %+v", verification)
	}
	backups, err := fixture.manager.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(backups) != 1 || backups[0].State != "ready" || backups[0].BackupID != manifest.BackupID {
		t.Fatalf("backups = %+v", backups)
	}
	if matches, _ := filepath.Glob(filepath.Join(fixture.backupDir, ".*.tmp")); len(matches) != 0 {
		t.Fatalf("temporary backup files remain: %v", matches)
	}
}

func TestCreateMetadataBackupWithConcurrentReadAndPooledConnections(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	fixture.database.SQL.SetMaxOpenConns(2)
	fixture.database.SQL.SetMaxIdleConns(2)

	// Keep a real SQLite read statement active on one pooled connection. The
	// application routinely has list queries in flight while a user starts a
	// backup, so snapshot creation must use another connection without requiring
	// callers to drain unrelated reads first.
	readRows, err := fixture.database.SQL.QueryContext(context.Background(), `
SELECT id FROM library_files
UNION ALL
SELECT id FROM library_files
ORDER BY id
`)
	if err != nil {
		t.Fatalf("start concurrent library read: %v", err)
	}
	defer readRows.Close()
	if !readRows.Next() {
		t.Fatalf("concurrent library read has no first row: %v", readRows.Err())
	}
	var fileID string
	if err := readRows.Scan(&fileID); err != nil || fileID != "file-movie" {
		t.Fatalf("concurrent library read first row = %q, error = %v", fileID, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create while a pooled read is active: %v", err)
	}
	if verification, err := fixture.manager.Verify(ctx, first.BackupID); err != nil || !verification.Valid {
		t.Fatalf("Verify first pooled backup = %+v, error = %v", verification, err)
	}
	assertSnapshotCatalogItemTitle(t, fixture, first, "Movie")

	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_catalog_items
SET title = 'Movie Updated', sort_title = 'Movie Updated', revision = revision + 1
WHERE id = 'item-movie'
`); err != nil {
		t.Fatalf("update live metadata between backups: %v", err)
	}
	*fixture.now = fixture.now.Add(time.Minute)

	second, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create repeated backup while a pooled read is active: %v", err)
	}
	if second.BackupID == first.BackupID {
		t.Fatalf("repeated backup reused id %q", second.BackupID)
	}
	if verification, err := fixture.manager.Verify(ctx, second.BackupID); err != nil || !verification.Valid {
		t.Fatalf("Verify repeated pooled backup = %+v, error = %v", verification, err)
	}
	assertSnapshotCatalogItemTitle(t, fixture, second, "Movie Updated")

	backups, err := fixture.manager.List(ctx)
	if err != nil {
		t.Fatalf("List repeated pooled backups: %v", err)
	}
	if len(backups) != 2 || backups[0].BackupID != second.BackupID || backups[1].BackupID != first.BackupID {
		t.Fatalf("repeated pooled backups = %+v", backups)
	}
}

func assertSnapshotCatalogItemTitle(t *testing.T, fixture *backupFixture, manifest domainbackup.Manifest, want string) {
	t.Helper()
	database, err := openReadOnlySQLite(filepath.Join(fixture.backupDir, manifest.Database.FileName))
	if err != nil {
		t.Fatalf("open metadata snapshot %q: %v", manifest.BackupID, err)
	}
	defer database.Close()
	var got string
	if err := database.QueryRow("SELECT title FROM library_catalog_items WHERE id = 'item-movie'").Scan(&got); err != nil {
		t.Fatalf("read metadata snapshot %q: %v", manifest.BackupID, err)
	}
	if got != want {
		t.Fatalf("metadata snapshot %q title = %q, want %q", manifest.BackupID, got, want)
	}
}

func TestVerifyRejectsTamperedSnapshotAndManifest(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	manifest, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	databasePath := filepath.Join(fixture.backupDir, manifest.Database.FileName)
	file, err := os.OpenFile(databasePath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	if _, err := file.Write([]byte("tamper")); err != nil {
		t.Fatalf("tamper snapshot: %v", err)
	}
	_ = file.Close()
	if _, err := fixture.manager.Verify(ctx, manifest.BackupID); err == nil {
		t.Fatal("Verify accepted tampered snapshot")
	}

	second, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}
	manifestPath := filepath.Join(fixture.backupDir, backupPrefix+second.BackupID+manifestSuffix)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	decoded["credentialHash"] = "injected"
	data, _ = json.Marshal(decoded)
	if err := os.WriteFile(manifestPath, data, backupFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.Verify(ctx, second.BackupID); err == nil {
		t.Fatal("Verify accepted manifest with unknown security-sensitive field")
	}
}

func TestRetentionKeepsNewestBackups(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{MaxBackups: 2, MaxAgeDays: 365})
	ctx := context.Background()
	created := make([]string, 0, 3)
	for index := 0; index < 3; index++ {
		manifest, err := fixture.manager.Create(ctx)
		if err != nil {
			t.Fatalf("Create %d: %v", index, err)
		}
		created = append(created, manifest.BackupID)
		*fixture.now = fixture.now.Add(time.Hour)
	}
	backups, err := fixture.manager.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 || backups[0].BackupID != created[2] || backups[1].BackupID != created[1] {
		t.Fatalf("retained backups = %+v", backups)
	}
	if _, err := fixture.manager.Verify(ctx, created[0]); !errors.Is(err, domainbackup.ErrBackupNotFound) {
		t.Fatalf("oldest backup verify error = %v", err)
	}
}

func TestPlanRestoreDefersReplacementAndApplyOnNextLaunch(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create target: %v", err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_catalog_items
SET title = 'Changed after backup', sort_title = 'Changed after backup', revision = revision + 1
WHERE id = 'item-movie'
`); err != nil {
		t.Fatalf("change live database: %v", err)
	}
	plan, err := fixture.manager.PlanRestore(ctx, target.BackupID)
	if err != nil {
		t.Fatalf("PlanRestore: %v", err)
	}
	if !plan.AppliesOnLaunch || plan.RollbackBackupID == "" {
		t.Fatalf("restore plan = %+v", plan)
	}
	var liveTitle string
	if err := fixture.database.SQL.QueryRowContext(ctx, "SELECT title FROM library_catalog_items WHERE id = 'item-movie'").Scan(&liveTitle); err != nil {
		t.Fatal(err)
	}
	if liveTitle != "Changed after backup" {
		t.Fatalf("PlanRestore replaced online database, title=%q", liveTitle)
	}
	pending, err := fixture.manager.PendingRestore(ctx)
	if err != nil || pending == nil || pending.BackupID != target.BackupID {
		t.Fatalf("PendingRestore = %+v, %v", pending, err)
	}
	if _, err := fixture.manager.Verify(ctx, plan.RollbackBackupID); err != nil {
		t.Fatalf("verify rollback backup: %v", err)
	}
	_, _, rollbackPath, err := fixture.manager.resolveBackup(plan.RollbackBackupID)
	if err != nil {
		t.Fatalf("resolve rollback backup: %v", err)
	}
	rollbackDB, err := openReadOnlySQLite(rollbackPath)
	if err != nil {
		t.Fatalf("open rollback backup: %v", err)
	}
	var rollbackTitle string
	if err := rollbackDB.QueryRowContext(ctx, "SELECT title FROM library_catalog_items WHERE id = 'item-movie'").Scan(&rollbackTitle); err != nil {
		_ = rollbackDB.Close()
		t.Fatal(err)
	}
	_ = rollbackDB.Close()
	if rollbackTitle != "Changed after backup" {
		t.Fatalf("rollback snapshot omitted committed WAL content, title=%q", rollbackTitle)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatalf("close live database: %v", err)
	}
	result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	})
	if err != nil {
		t.Fatalf("ApplyPendingRestore: %v", err)
	}
	if !result.Applied || result.BackupID != target.BackupID || result.RollbackBackupID != plan.RollbackBackupID {
		t.Fatalf("apply result = %+v", result)
	}
	if _, err := os.Stat(fixture.markerPath); err != nil {
		t.Fatalf("two-phase restore marker missing before OpenSQLite: %v", err)
	}
	if _, err := os.Stat(fixture.databasePath + ".restore-previous"); err != nil {
		t.Fatalf("physical rollback database missing before OpenSQLite: %v", err)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatalf("open restored database: %v", err)
	}
	defer restored.Close()
	var restoredTitle string
	if err := restored.SQL.QueryRowContext(ctx, "SELECT title FROM library_catalog_items WHERE id = 'item-movie'").Scan(&restoredTitle); err != nil {
		t.Fatal(err)
	}
	if restoredTitle != "Movie" {
		t.Fatalf("restored title = %q", restoredTitle)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	if err := FinalizePendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil {
		t.Fatalf("FinalizePendingRestore: %v", err)
	}
	if _, err := os.Stat(fixture.markerPath); !os.IsNotExist(err) {
		t.Fatalf("restore marker remains after finalization: %v", err)
	}
}

func TestCreateRejectsWrongApplicationIDAndFutureSchema(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	if _, err := fixture.database.SQL.ExecContext(ctx, "PRAGMA application_id = 1234"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.Create(ctx); !errors.Is(err, domainbackup.ErrIncompatibleBackup) {
		t.Fatalf("wrong application ID error = %v", err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, "PRAGMA application_id = "+fmt.Sprint(persistence.SQLiteApplicationID())); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, "PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.Create(ctx); !errors.Is(err, domainbackup.ErrIncompatibleBackup) {
		t.Fatalf("future schema error = %v", err)
	}
}

func TestApplyPendingRestoreRecoversInterruptedSwap(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, "UPDATE library_catalog_items SET title = 'Current' WHERE id = 'item-movie'"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	previous := fixture.databasePath + ".restore-previous"
	if err := os.Rename(fixture.databasePath, previous); err != nil {
		t.Fatalf("simulate interrupted swap: %v", err)
	}
	if err := os.WriteFile(fixture.databasePath, []byte("incomplete replacement"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	})
	if err != nil {
		t.Fatalf("ApplyPendingRestore: %v", err)
	}
	if !result.Applied {
		t.Fatalf("apply result = %+v", result)
	}
	if _, err := os.Stat(previous); err != nil {
		t.Fatalf("previous swap artifact was finalized before OpenSQLite: %v", err)
	}
}

func TestCancelPendingRestoreRemovesOrphanRollbackBackup(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := fixture.manager.PlanRestore(ctx, target.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.manager.CancelPendingRestore(ctx); err != nil {
		t.Fatal(err)
	}
	pending, err := fixture.manager.PendingRestore(ctx)
	if err != nil || pending != nil {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
	if _, err := fixture.manager.Verify(ctx, plan.RollbackBackupID); !errors.Is(err, domainbackup.ErrBackupNotFound) {
		t.Fatalf("cancelled rollback backup should be removed, got: %v", err)
	}
}

func TestLogicalRestorePreservesCurrentSecurityDomainAndApplicationSettings(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO settings (id, appearance, version) VALUES (1, 'light', 1);
INSERT INTO library_access_settings (
  id, remote_enabled, lan_enabled, lan_port, tailscale_enabled,
  tailscale_https_port, tailscale_path, device_name
) VALUES (1, 0, 1, 0, 0, 443, '/xiadown', 'Old device');
`); err != nil {
		t.Fatalf("seed pre-backup settings: %v", err)
	}
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE settings SET appearance = 'dark', version = version + 1 WHERE id = 1;
UPDATE library_access_settings
SET remote_enabled = 1, tailscale_enabled = 1, lan_port = 48123,
    tailscale_https_port = 8443, tailscale_path = '/current', device_name = 'Current device'
WHERE id = 1;
UPDATE library_device_grants
SET credential_hash = 'current-revoked-credential-hash', status = 'revoked',
    revoked_at = '2026-07-13T09:00:00Z', revision = revision + 1,
    updated_at = '2026-07-13T09:00:00Z'
WHERE id = 'grant-one';
INSERT INTO library_catalog_changes (
  catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
) VALUES (
  'catalog-default', 'device_grant', 'grant-one', 'upsert', 2,
  'security-current', '2026-07-13T09:00:00Z'
);
INSERT INTO library_access_tailscale_route_state (
  id, https_port, route_path, backend_port, pending_backend_port,
  state, last_action, last_result, last_error, revision, updated_at
) VALUES (
  1, 8443, '/current', 48123, 0,
  'active', 'enable', 'succeeded', '', 7, '2026-07-13T09:00:00Z'
);
INSERT INTO library_access_tailscale_route_audit (
  https_port, route_path, backend_port, pending_backend_port,
  state, action, result, error, transitioned_at
) VALUES (
  8443, '/current', 48123, 0,
  'active', 'enable', 'succeeded', '', '2026-07-13T09:00:00Z'
);
UPDATE library_catalog_items
SET title = 'Current title', sort_title = 'Current title', revision = revision + 1
WHERE id = 'item-movie';
`); err != nil {
		t.Fatalf("mutate current security state: %v", err)
	}
	plan, err := fixture.manager.PlanRestore(ctx, target.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	})
	if err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore = %+v, %v", result, err)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	var title, appearance string
	if err := restored.SQL.QueryRowContext(ctx, "SELECT title FROM library_catalog_items WHERE id = 'item-movie'").Scan(&title); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, "SELECT appearance FROM settings WHERE id = 1").Scan(&appearance); err != nil {
		t.Fatal(err)
	}
	if title != "Movie" || appearance != "dark" {
		t.Fatalf("restored title/settings = %q/%q", title, appearance)
	}
	var grantStatus, credentialHash string
	var grantRevision int
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT status, credential_hash, revision FROM library_device_grants WHERE id = 'grant-one'
`).Scan(&grantStatus, &credentialHash, &grantRevision); err != nil {
		t.Fatal(err)
	}
	if grantStatus != "revoked" || credentialHash != "current-revoked-credential-hash" || grantRevision != 2 {
		t.Fatalf("security grant rolled back = %q/%q/%d", grantStatus, credentialHash, grantRevision)
	}
	var remote, tailscale, port int
	var routePath, deviceName string
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT remote_enabled, tailscale_enabled, lan_port, tailscale_path, device_name
FROM library_access_settings WHERE id = 1
`).Scan(&remote, &tailscale, &port, &routePath, &deviceName); err != nil {
		t.Fatal(err)
	}
	if remote != 1 || tailscale != 1 || port != 48123 || routePath != "/current" || deviceName != "Current device" {
		t.Fatalf("access settings rolled back = %d/%d/%d/%q/%q", remote, tailscale, port, routePath, deviceName)
	}
	var routeRevision, routeBackend, auditCount, grantChangeCount int
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT revision, backend_port FROM library_access_tailscale_route_state WHERE id = 1
`).Scan(&routeRevision, &routeBackend); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_access_tailscale_route_audit").Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_catalog_changes
WHERE entity_type = 'device_grant' AND actor_id = 'security-current'
`).Scan(&grantChangeCount); err != nil {
		t.Fatal(err)
	}
	if routeRevision != 7 || routeBackend != 48123 || auditCount != 1 || grantChangeCount != 1 {
		t.Fatalf("route/change security state = rev:%d backend:%d audit:%d changes:%d", routeRevision, routeBackend, auditCount, grantChangeCount)
	}
	if err := persistence.VerifySQLiteIntegrity(ctx, restored.SQL, false); err != nil {
		t.Fatalf("restored database integrity: %v", err)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	if err := FinalizePendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.Verify(ctx, plan.RollbackBackupID); err != nil {
		t.Fatalf("retained rollback snapshot is invalid: %v", err)
	}
}

func TestLogicalRestoreRotatesSyncEpochAndKeepsCursorMonotonic(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	var sourceEpoch string
	if err := fixture.database.SQL.QueryRowContext(ctx, `
SELECT epoch FROM library_catalog_sync_state WHERE catalog_id = 'catalog-default'
`).Scan(&sourceEpoch); err != nil {
		t.Fatal(err)
	}
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_catalog_sync_state
SET epoch = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', rotated_at = '2026-07-13T09:00:00Z'
WHERE catalog_id = 'catalog-default';
INSERT INTO library_catalog_changes (
  catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
) VALUES (
  'catalog-default', 'catalog', 'catalog-default', 'upsert', 2,
  'current-before-restore', '2026-07-13T09:00:00Z'
);
`); err != nil {
		t.Fatal(err)
	}
	var cursorBefore int64
	if err := fixture.database.SQL.QueryRowContext(ctx,
		"SELECT MAX(sequence) FROM library_catalog_changes",
	).Scan(&cursorBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore = %+v, %v", result, err)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var restoredEpoch string
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT epoch FROM library_catalog_sync_state WHERE catalog_id = 'catalog-default'
`).Scan(&restoredEpoch); err != nil {
		t.Fatal(err)
	}
	if len(restoredEpoch) != 32 || restoredEpoch == sourceEpoch || restoredEpoch == "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("logical restore did not rotate sync epoch: source=%q restored=%q", sourceEpoch, restoredEpoch)
	}
	result, err := restored.SQL.ExecContext(ctx, `
INSERT INTO library_catalog_changes (
  catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
) VALUES (
  'catalog-default', 'catalog', 'catalog-default', 'upsert', 3,
  'after-restore', '2026-07-13T10:00:00Z'
)
`)
	if err != nil {
		t.Fatal(err)
	}
	nextCursor, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if nextCursor <= cursorBefore {
		t.Fatalf("post-restore cursor = %d, want > %d", nextCursor, cursorBefore)
	}
}

func TestLogicalRestorePreservesPrunedRSSRetentionBoundary(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	var sourceEpoch string
	if err := fixture.database.SQL.QueryRowContext(ctx, `
SELECT epoch FROM rss_sync_state WHERE workspace_id = ?
`, domainrss.DefaultWorkspaceID).Scan(&sourceEpoch); err != nil {
		t.Fatal(err)
	}
	sequences := make([]int64, 0, 6)
	for index := 0; index < 6; index++ {
		if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO rss_changes (
  workspace_id, entity_type, entity_id, operation, revision, payload_json, changed_at
) VALUES (?, 'subscription', ?, 'delete', 1, '{}', ?)
`, domainrss.DefaultWorkspaceID, fmt.Sprintf("rss-pruned-%d", index), fixture.now.Add(time.Duration(index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
		var sequence int64
		if err := fixture.database.SQL.QueryRowContext(ctx, `
SELECT MAX(sequence) FROM rss_changes WHERE workspace_id = ?
`, domainrss.DefaultWorkspaceID).Scan(&sequence); err != nil {
			t.Fatal(err)
		}
		sequences = append(sequences, sequence)
	}
	retainedFrom := sequences[2]
	sourceHighWater := sequences[len(sequences)-1]
	if _, err := fixture.database.SQL.ExecContext(ctx, `
DELETE FROM rss_changes WHERE workspace_id = ? AND sequence <= ?
`, domainrss.DefaultWorkspaceID, retainedFrom); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE rss_sync_state SET retained_from = ? WHERE workspace_id = ?
`, retainedFrom, domainrss.DefaultWorkspaceID); err != nil {
		t.Fatal(err)
	}

	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE rss_sync_state
SET epoch = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', retained_from = 0
WHERE workspace_id = ?
`, domainrss.DefaultWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO rss_changes (
  workspace_id, entity_type, entity_id, operation, revision, payload_json, changed_at
) VALUES (?, 'subscription', 'rss-current-only', 'delete', 1, '{}', ?)
`, domainrss.DefaultWorkspaceID, fixture.now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore = %+v, %v", result, err)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	repository := rssrepo.NewSQLiteRepository(restored.Bun)
	scope := domainrss.SyncScope{
		WorkspaceID: domainrss.DefaultWorkspaceID,
		SubjectID:   domainrss.DefaultSubjectID,
	}
	overview, err := repository.GetSyncOverview(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Epoch == sourceEpoch || overview.Epoch == "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		overview.RetainedFrom != retainedFrom || overview.HighWater != sourceHighWater {
		t.Fatalf(
			"restored RSS position = %#v, sourceEpoch=%q retained=%d highWater=%d",
			overview, sourceEpoch, retainedFrom, sourceHighWater,
		)
	}
	if _, err := repository.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: overview.Epoch, After: retainedFrom - 1, Limit: 20,
	}); !errors.Is(err, domainrss.ErrSyncResetRequired) {
		t.Fatalf("pre-retention cursor error = %v", err)
	}
	page, err := repository.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: overview.Epoch, After: retainedFrom, Limit: 20,
	})
	if err != nil || len(page.Changes) != 3 || page.Cursor != sourceHighWater || page.HighWater != sourceHighWater {
		t.Fatalf("retained RSS changes = %#v, error=%v", page, err)
	}
}

func TestLogicalRestoreReplacesRSSSubscriptionHistoryWithBackupState(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, created_at, updated_at, revision
) VALUES (
  'rss-source', 'rss-default', 'https://source.example/feed', 'Source',
  '2026-07-13T07:00:00Z', '2026-07-13T07:00:00Z', 1
);
INSERT INTO rss_subscription_history (
  subscription_id, cursor_url, capability, exhausted, no_progress_count,
  last_success_at, updated_at
) VALUES (
  'rss-source', 'https://source.example/page/2', 'available', FALSE, 0,
  '2026-07-13T07:05:00Z', '2026-07-13T07:05:00Z'
);
`); err != nil {
		t.Fatal(err)
	}
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE rss_subscription_history
SET cursor_url = 'https://source.example/current-only', updated_at = '2026-07-13T09:00:00Z'
WHERE subscription_id = 'rss-source';
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, created_at, updated_at, revision
) VALUES (
  'rss-current-only', 'rss-default', 'https://current.example/feed', 'Current only',
  '2026-07-13T09:00:00Z', '2026-07-13T09:00:00Z', 1
);
INSERT INTO rss_subscription_history (
  subscription_id, cursor_url, capability, exhausted, no_progress_count, updated_at
) VALUES (
  'rss-current-only', 'https://current.example/page/2', 'available', FALSE, 0,
  '2026-07-13T09:00:00Z'
);
`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore = %+v, %v", result, err)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var cursor string
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT cursor_url FROM rss_subscription_history WHERE subscription_id = 'rss-source'
`).Scan(&cursor); err != nil {
		t.Fatal(err)
	}
	if cursor != "https://source.example/page/2" {
		t.Fatalf("restored RSS history cursor = %q", cursor)
	}
	var currentOnly int
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM rss_subscription_history WHERE subscription_id = 'rss-current-only'
`).Scan(&currentOnly); err != nil {
		t.Fatal(err)
	}
	if currentOnly != 0 {
		t.Fatalf("current-only RSS history survived restore: %d", currentOnly)
	}
	if err := persistence.VerifySQLiteIntegrity(ctx, restored.SQL, false); err != nil {
		t.Fatalf("restored database integrity: %v", err)
	}
}

func TestLogicalRestoreReplacesRSSOrganizationAndListenLiveUserCatalog(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	const backupModuleConfig = `{"Retention":{"WorkspaceStatesMax":10,"FileEventsMax":100,"HistoryMax":100,"OperationLogsMax":25},"Workspace":{"FastReadLatestState":false}}`
	const currentModuleConfig = `{"Retention":{"WorkspaceStatesMax":30,"FileEventsMax":300,"HistoryMax":300,"OperationLogsMax":75},"Workspace":{"FastReadLatestState":true}}`
	if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO rss_categories (
  id, workspace_id, title, sort_order, created_at, updated_at, revision
) VALUES (
  'rss-category-backup', 'rss-default', 'Backup category', 0,
  '2026-07-13T06:00:00Z', '2026-07-13T06:00:00Z', 1
);
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, category_id, sort_order,
  created_at, updated_at, revision
) VALUES (
  'rss-subscription-backup', 'rss-default', 'https://backup.example/feed',
  'Backup subscription', 'rss-category-backup', 0,
  '2026-07-13T06:01:00Z', '2026-07-13T06:01:00Z', 1
);
INSERT INTO rss_entries (
  id, subscription_id, external_id, title, content_hash, created_at, modified_at
) VALUES (
  'rss-entry-backup', 'rss-subscription-backup', 'entry-backup',
  'Backup entry', 'hash-backup',
  '2026-07-13T06:02:00Z', '2026-07-13T06:02:00Z'
);
INSERT INTO rss_collections (
  id, workspace_id, title, kind, view_type, sort_order,
  created_at, updated_at, revision
) VALUES
  ('rss-collection-subscriptions-backup', 'rss-default', 'Backup feeds',
   'subscriptions', 'auto', 0,
   '2026-07-13T06:03:00Z', '2026-07-13T06:03:00Z', 1),
  ('rss-collection-entries-backup', 'rss-default', 'Backup articles',
   'entries', 'article', 1,
   '2026-07-13T06:04:00Z', '2026-07-13T06:04:00Z', 1);
INSERT INTO rss_collection_subscriptions (
  collection_id, subscription_id, sort_order, added_at
) VALUES (
  'rss-collection-subscriptions-backup', 'rss-subscription-backup', 0,
  '2026-07-13T06:05:00Z'
);
INSERT INTO rss_collection_entries (
  collection_id, entry_id, sort_order, added_at
) VALUES (
  'rss-collection-entries-backup', 'rss-entry-backup', 0,
  '2026-07-13T06:06:00Z'
);
INSERT INTO rss_sources (
  id, workspace_id, subscription_id, kind, handle, sort_order,
  created_at, updated_at, revision
) VALUES (
  'rss-source-backup', 'rss-default', 'rss-subscription-backup',
  'inbox', 'backup-handle', 0,
  '2026-07-13T06:07:00Z', '2026-07-13T06:07:00Z', 1
);
INSERT INTO listen_live_columns (
  id, title, sort_order, created_at, updated_at
) VALUES (
  'listen-live-column-backup', 'Backup live column', 0,
  '2026-07-13T06:08:00Z', '2026-07-13T06:08:00Z'
);
INSERT INTO listen_live_channels (
  id, column_id, title, channel, description, source, video_id,
  thumbnail_url, enabled, sort_order, created_at, updated_at
) VALUES (
  'listen-live-channel-backup', 'listen-live-column-backup',
  'Backup live channel', 'Backup author', 'Backup description',
  'https://youtube.example/watch?v=backup-live', 'backup-live',
  'https://img.example/backup.jpg', TRUE, 0,
  '2026-07-13T06:09:00Z', '2026-07-13T06:09:00Z'
);
INSERT INTO rss_discovery_routes (
  id, title, url, route_path, example_path
) VALUES (
  'rss-discovery-backup', 'Backup discovery cache',
  'https://discovery.example/backup', '/backup/:id', '/backup/example'
);
INSERT INTO rss_discovery_meta (
  source, source_url, fetched_at, route_count, updated_at
) VALUES (
  'rsshub', 'https://discovery.example',
  '2026-07-13T06:11:00Z', 1, '2026-07-13T06:11:00Z'
);
`); err != nil {
		t.Fatalf("seed backup RSS organization and Listen live catalog: %v", err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO library_module_config (id, config_json, updated_at)
VALUES (1, ?, '2026-07-13T06:10:00Z')
`, backupModuleConfig); err != nil {
		t.Fatalf("seed backup module config: %v", err)
	}
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
DELETE FROM rss_collection_subscriptions;
DELETE FROM rss_collection_entries;
UPDATE rss_categories
SET title = 'Current category', updated_at = '2026-07-13T09:00:00Z', revision = 2
WHERE id = 'rss-category-backup';
UPDATE rss_subscriptions
SET title = 'Current subscription', category_id = NULL,
    updated_at = '2026-07-13T09:00:00Z', revision = 2
WHERE id = 'rss-subscription-backup';
UPDATE rss_entries
SET title = 'Current entry', content_hash = 'hash-current',
    modified_at = '2026-07-13T09:00:00Z', revision = 2
WHERE id = 'rss-entry-backup';
UPDATE rss_collections
SET title = 'Current collection', updated_at = '2026-07-13T09:00:00Z', revision = 2;
UPDATE rss_sources
SET handle = 'current-handle', updated_at = '2026-07-13T09:00:00Z', revision = 2
WHERE id = 'rss-source-backup';
UPDATE listen_live_columns
SET title = 'Current live column', updated_at = '2026-07-13T09:00:00Z'
WHERE id = 'listen-live-column-backup';
UPDATE listen_live_channels
SET title = 'Current live channel', video_id = 'current-live',
    updated_at = '2026-07-13T09:00:00Z'
WHERE id = 'listen-live-channel-backup';
UPDATE rss_discovery_routes
SET title = 'Current discovery cache'
WHERE id = 'rss-discovery-backup';

INSERT INTO rss_categories (
  id, workspace_id, title, sort_order, created_at, updated_at, revision
) VALUES (
  'rss-category-current-only', 'rss-default', 'Current-only category', 1,
  '2026-07-13T09:01:00Z', '2026-07-13T09:01:00Z', 1
);
INSERT INTO rss_subscriptions (
  id, workspace_id, feed_url, title, category_id, sort_order,
  created_at, updated_at, revision
) VALUES (
  'rss-subscription-current-only', 'rss-default', 'https://current.example/feed',
  'Current-only subscription', 'rss-category-current-only', 0,
  '2026-07-13T09:02:00Z', '2026-07-13T09:02:00Z', 1
);
INSERT INTO rss_entries (
  id, subscription_id, external_id, title, content_hash, created_at, modified_at
) VALUES (
  'rss-entry-current-only', 'rss-subscription-current-only', 'entry-current',
  'Current-only entry', 'hash-current-only',
  '2026-07-13T09:03:00Z', '2026-07-13T09:03:00Z'
);
INSERT INTO rss_collections (
  id, workspace_id, title, kind, sort_order, created_at, updated_at, revision
) VALUES (
  'rss-collection-current-only', 'rss-default', 'Current-only collection',
  'subscriptions', 2,
  '2026-07-13T09:04:00Z', '2026-07-13T09:04:00Z', 1
);
INSERT INTO rss_collection_subscriptions (
  collection_id, subscription_id, sort_order, added_at
) VALUES (
  'rss-collection-current-only', 'rss-subscription-current-only', 0,
  '2026-07-13T09:05:00Z'
);
INSERT INTO rss_sources (
  id, workspace_id, subscription_id, kind, handle, sort_order,
  created_at, updated_at, revision
) VALUES (
  'rss-source-current-only', 'rss-default', 'rss-subscription-current-only',
  'notification', 'current-only-handle', 1,
  '2026-07-13T09:06:00Z', '2026-07-13T09:06:00Z', 1
);
INSERT INTO listen_live_columns (
  id, title, sort_order, created_at, updated_at
) VALUES (
  'listen-live-column-current-only', 'Current-only live column', 1,
  '2026-07-13T09:07:00Z', '2026-07-13T09:07:00Z'
);
INSERT INTO listen_live_channels (
  id, column_id, title, video_id, enabled, sort_order, created_at, updated_at
) VALUES (
  'listen-live-channel-current-only', 'listen-live-column-current-only',
  'Current-only live channel', 'current-only-live', TRUE, 1,
  '2026-07-13T09:08:00Z', '2026-07-13T09:08:00Z'
);
`); err != nil {
		t.Fatalf("mutate current RSS organization and Listen live catalog: %v", err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_module_config
SET config_json = ?, updated_at = '2026-07-13T09:00:00Z'
WHERE id = 1
`, currentModuleConfig); err != nil {
		t.Fatalf("mutate current module config: %v", err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore = %+v, %v", result, err)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()

	for table, want := range map[string]int{
		"rss_categories":               1,
		"rss_collections":              2,
		"rss_collection_subscriptions": 1,
		"rss_collection_entries":       1,
		"rss_sources":                  1,
		"listen_live_columns":          1,
		"listen_live_channels":         1,
	} {
		var count int
		if err := restored.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("restored %s count = %d, want %d", table, count, want)
		}
	}

	rssRepository := rssrepo.NewSQLiteRepository(restored.Bun)
	categories, err := rssRepository.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0].ID != "rss-category-backup" ||
		categories[0].Title != "Backup category" || categories[0].SubscriptionCount != 1 {
		t.Fatalf("restored RSS categories = %+v", categories)
	}
	collections, err := rssRepository.ListCollections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(collections) != 2 || collections[0].ID != "rss-collection-subscriptions-backup" ||
		collections[0].Title != "Backup feeds" || collections[0].ItemCount != 1 ||
		collections[1].ID != "rss-collection-entries-backup" ||
		collections[1].Title != "Backup articles" || collections[1].ItemCount != 1 {
		t.Fatalf("restored RSS collections = %+v", collections)
	}
	feedItems, err := rssRepository.ListCollectionItems(ctx, "rss-collection-subscriptions-backup")
	if err != nil {
		t.Fatal(err)
	}
	entryItems, err := rssRepository.ListCollectionItems(ctx, "rss-collection-entries-backup")
	if err != nil {
		t.Fatal(err)
	}
	if len(feedItems.ItemIDs) != 1 || feedItems.ItemIDs[0] != "rss-subscription-backup" ||
		len(entryItems.ItemIDs) != 1 || entryItems.ItemIDs[0] != "rss-entry-backup" {
		t.Fatalf("restored RSS collection members = feeds:%+v entries:%+v", feedItems, entryItems)
	}
	sources, err := rssRepository.ListSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].ID != "rss-source-backup" ||
		sources[0].Handle != "backup-handle" || sources[0].Title != "Backup subscription" {
		t.Fatalf("restored RSS sources = %+v", sources)
	}

	liveCatalog, err := libraryrepo.NewSQLiteListenLiveChannelRepository(restored.Bun).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(liveCatalog.Columns) != 1 || liveCatalog.Columns[0].ID != "listen-live-column-backup" ||
		liveCatalog.Columns[0].Title != "Backup live column" ||
		len(liveCatalog.Channels) != 1 || liveCatalog.Channels[0].ID != "listen-live-channel-backup" ||
		liveCatalog.Channels[0].Title != "Backup live channel" ||
		liveCatalog.Channels[0].VideoID != "backup-live" {
		t.Fatalf("restored Listen live user catalog = %+v", liveCatalog)
	}

	var moduleConfig, discoveryTitle string
	if err := restored.SQL.QueryRowContext(ctx, "SELECT config_json FROM library_module_config WHERE id = 1").Scan(&moduleConfig); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, "SELECT title FROM rss_discovery_routes WHERE id = 'rss-discovery-backup'").Scan(&discoveryTitle); err != nil {
		t.Fatal(err)
	}
	if moduleConfig != currentModuleConfig || discoveryTitle != "Current discovery cache" {
		t.Fatalf("non-restorable config/cache changed: module=%q discovery=%q", moduleConfig, discoveryTitle)
	}
	if err := persistence.VerifySQLiteIntegrity(ctx, restored.SQL, false); err != nil {
		t.Fatalf("restored database integrity: %v", err)
	}
}

func TestLogicalRestoreFailsClosedWhenCurrentSecurityDomainIsUntrusted(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	if _, err := fixture.database.SQL.ExecContext(ctx, "INSERT INTO settings (id, appearance, version) VALUES (1, 'light', 1)"); err != nil {
		t.Fatal(err)
	}
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO library_access_settings (
  id, remote_enabled, lan_enabled, lan_port, tailscale_enabled,
  tailscale_https_port, tailscale_path, device_name
) VALUES (1, 1, 1, 48123, 1, 8443, '/untrusted', 'Untrusted current');
UPDATE settings SET appearance = 'dark', version = version + 1 WHERE id = 1;
DROP TRIGGER library_access_tailscale_route_audit_no_delete;
UPDATE library_catalog_items
SET title = 'Current title', sort_title = 'Current title'
WHERE id = 'item-movie';
`); err != nil {
		t.Fatalf("make current security domain untrusted: %v", err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore = %+v, %v", result, err)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var grants, routeStates, remote, tailscale int
	var appearance string
	if err := restored.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_device_grants").Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_access_tailscale_route_state").Scan(&routeStates); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT remote_enabled, tailscale_enabled FROM library_access_settings WHERE id = 1
`).Scan(&remote, &tailscale); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, "SELECT appearance FROM settings WHERE id = 1").Scan(&appearance); err != nil {
		t.Fatal(err)
	}
	if grants != 0 || routeStates != 0 || remote != 0 || tailscale != 0 {
		t.Fatalf("untrusted security was revived: grants=%d routes=%d remote=%d tailscale=%d", grants, routeStates, remote, tailscale)
	}
	if appearance != "dark" {
		t.Fatalf("non-Library settings were lost during fail-closed security reset: %q", appearance)
	}
}

func TestLogicalRestoreRejectsCatalogIdentityChangeThatWouldOrphanCurrentGrant(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_catalogs SET is_default = FALSE WHERE id = 'catalog-default';
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (
  'catalog-current', 'Current default', '', 'active', TRUE,
  '2026-07-13T09:00:00Z', '2026-07-13T09:00:00Z'
);
INSERT INTO library_device_grants (
  id, catalog_id, device_id, device_name, credential_hash, public_key_hash,
  scopes_json, status, created_at, updated_at, revision
) VALUES (
  'grant-current-catalog', 'catalog-current', 'device-current', 'Current phone',
  'current-catalog-credential', 'current-catalog-public-key', '["library.read"]',
  'active', '2026-07-13T09:00:00Z', '2026-07-13T09:00:00Z', 1
);
`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err == nil || !strings.Contains(err.Error(), "current device grant") {
		t.Fatalf("restore should reject orphaned current grant, got %v", err)
	}
	if _, err := os.Stat(fixture.databasePath + ".restore-previous"); !os.IsNotExist(err) {
		t.Fatalf("swap began despite catalog security-identity conflict: %v", err)
	}
	current, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	var status string
	if err := current.SQL.QueryRowContext(ctx, `
SELECT status FROM library_device_grants WHERE id = 'grant-current-catalog'
`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("current grant changed after rejected restore: %q", status)
	}
}

func TestLogicalRestoreIncludesTasksHistoryWorkspaceImportsAndSubtitleMetadata(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO library_files (
  id, library_id, kind, name, display_name, storage_mode, storage_document_id,
  origin_kind, origin_import_path, state_json, created_at, updated_at
) VALUES (
  'file-subtitle', 'legacy-bundle', 'subtitle', 'movie.srt', 'Movie subtitle',
  'db_document', 'subtitle-document', 'import', '/source/movie.srt', '{}',
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_subtitle_documents (
  id, file_id, library_id, format, original_content, working_content, created_at, updated_at
) VALUES (
  'subtitle-document', 'file-subtitle', 'legacy-bundle', 'srt',
  'backup subtitle', 'backup subtitle', '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_operations (
  id, library_id, kind, status, display_name, correlation_json, input_json,
  output_json, file_count, created_at, started_at
) VALUES (
  'operation-backup', 'legacy-bundle', 'download', 'running', 'Backup task',
  '{}', '{}', '{}', 1, '2026-07-13T00:00:00Z', '2026-07-13T00:00:01Z'
);
INSERT INTO library_external_processes (
  id, operation_id, kind, tool, pid, created_at, updated_at
) VALUES (
  'process-backup', 'operation-backup', 'download', 'yt-dlp', 4242,
  '2026-07-13T00:00:01Z', '2026-07-13T00:00:01Z'
);
INSERT INTO library_operation_outputs (
  id, operation_id, library_id, file_id, is_primary, created_at
) VALUES (
  'output-backup', 'operation-backup', 'legacy-bundle', 'file-movie', 1,
  '2026-07-13T00:00:02Z'
);
INSERT INTO library_operation_chunks (
  id, operation_id, library_id, chunk_index, status, retry_count, created_at, updated_at
) VALUES (
  'chunk-backup', 'operation-backup', 'legacy-bundle', 0, 'running', 0,
  '2026-07-13T00:00:02Z', '2026-07-13T00:00:02Z'
);
INSERT INTO library_history_records (
  id, library_id, category, action, display_name, status, source_kind,
  operation_id, file_count, occurred_at, created_at, updated_at
) VALUES (
  'history-backup', 'legacy-bundle', 'operation', 'download', 'Backup history',
  'running', 'desktop', 'operation-backup', 1,
  '2026-07-13T00:00:03Z', '2026-07-13T00:00:03Z', '2026-07-13T00:00:03Z'
);
INSERT INTO library_history_files (
  id, history_id, file_id, kind, deleted, created_at
) VALUES (
  'history-file-backup', 'history-backup', 'file-movie', 'video', 0,
  '2026-07-13T00:00:03Z'
);
INSERT INTO library_workspace_states (
  id, library_id, state_version, state_json, operation_id, created_at
) VALUES (
  'workspace-backup', 'legacy-bundle', 1, '{"page":"tasks"}', 'operation-backup',
  '2026-07-13T00:00:04Z'
);
INSERT INTO library_workspace_state_head (library_id, latest_state_id, updated_at)
VALUES ('legacy-bundle', 'workspace-backup', '2026-07-13T00:00:04Z');
INSERT INTO library_file_events (
  id, library_id, file_id, operation_id, event_type, detail_json, created_at
) VALUES (
  'event-backup', 'legacy-bundle', 'file-movie', 'operation-backup',
  'created', '{}', '2026-07-13T00:00:05Z'
);
`); err != nil {
		t.Fatalf("seed durable Library management metadata: %v", err)
	}
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_operations SET display_name = 'Current task', status = 'succeeded' WHERE id = 'operation-backup';
UPDATE library_history_records SET display_name = 'Current history', status = 'succeeded' WHERE id = 'history-backup';
UPDATE library_subtitle_documents SET working_content = 'current subtitle' WHERE id = 'subtitle-document';
`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore = %+v, %v", result, err)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var operationName, operationStatus, historyName, subtitle string
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT display_name, status FROM library_operations WHERE id = 'operation-backup'
`).Scan(&operationName, &operationStatus); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT display_name FROM library_history_records WHERE id = 'history-backup'
`).Scan(&historyName); err != nil {
		t.Fatal(err)
	}
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT working_content FROM library_subtitle_documents WHERE id = 'subtitle-document'
`).Scan(&subtitle); err != nil {
		t.Fatal(err)
	}
	if operationName != "Backup task" || operationStatus != "running" || historyName != "Backup history" || subtitle != "backup subtitle" {
		t.Fatalf("durable Library metadata was lost: %q/%q/%q/%q", operationName, operationStatus, historyName, subtitle)
	}
	for _, table := range []string{
		"library_operation_outputs", "library_operation_chunks", "library_history_files",
		"library_workspace_states", "library_workspace_state_head", "library_file_events",
	} {
		var count int
		if err := restored.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("restored %s count = %d, want 1", table, count)
		}
	}
	var processCount int
	if err := restored.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_external_processes").Scan(&processCount); err != nil {
		t.Fatal(err)
	}
	if processCount != 0 {
		t.Fatalf("process IDs must not be restored, count=%d", processCount)
	}
}

func TestLogicalRestoreKeepsCustomTranscodePresetReferencesResolvable(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	if _, err := fixture.database.SQL.ExecContext(ctx, `
INSERT INTO transcode_presets (
  id, name, output_type, container, audio_codec, quality_mode,
  audio_bitrate_kbps, requires_audio, is_builtin, description, created_at, updated_at
) VALUES (
  'preset-custom-backup', 'Custom AAC Archive', 'audio', 'm4a', 'aac', 'bitrate',
  256, TRUE, FALSE, 'Must remain resolvable by restored tasks',
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
), (
  'builtin-obsolete-backup', 'Obsolete Builtin', 'audio', 'mp3', 'mp3', 'bitrate',
  96, TRUE, TRUE, 'Removed by startup normalization',
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_operations (
  id, library_id, kind, status, display_name, correlation_json, input_json,
  output_json, file_count, created_at
) VALUES (
  'operation-custom-preset-queued', 'legacy-bundle', 'download', 'queued',
  'Queued custom transcode', '{}',
  '{"url":"https://example.invalid/video","transcodePresetId":"preset-custom-backup"}',
  '{}', 0, '2026-07-13T00:00:01Z'
), (
  'operation-custom-preset-canceled', 'legacy-bundle', 'download', 'canceled',
  'Canceled custom transcode', '{}',
  '{"url":"https://example.invalid/audio","transcodePresetId":"preset-custom-backup"}',
  '{}', 0, '2026-07-13T00:00:02Z'
);
`); err != nil {
		t.Fatalf("seed custom preset references: %v", err)
	}

	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := fixture.database.SQL.ExecContext(ctx, `
DELETE FROM library_operations
WHERE id IN ('operation-custom-preset-queued', 'operation-custom-preset-canceled');
DELETE FROM transcode_presets
WHERE id IN ('preset-custom-backup', 'builtin-obsolete-backup');
`); err != nil {
		t.Fatalf("delete current preset graph: %v", err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatalf("PlanRestore: %v", err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatalf("close current database: %v", err)
	}
	if result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore = %+v, %v", result, err)
	}

	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatalf("open restored database: %v", err)
	}
	defer restored.Close()
	presetRepo := libraryrepo.NewSQLiteTranscodePresetRepository(restored.Bun)
	operationRepo := libraryrepo.NewSQLiteOperationRepository(restored.Bun)
	preset, err := presetRepo.Get(ctx, "preset-custom-backup")
	if err != nil {
		t.Fatalf("read restored custom preset through repository: %v", err)
	}
	if preset.IsBuiltin || preset.AudioBitrateKbps != 256 || preset.Container != "m4a" {
		t.Fatalf("restored custom preset = %+v", preset)
	}

	for _, expected := range []struct {
		id     string
		status library.OperationStatus
	}{
		{id: "operation-custom-preset-queued", status: library.OperationStatusQueued},
		{id: "operation-custom-preset-canceled", status: library.OperationStatusCanceled},
	} {
		operation, err := operationRepo.Get(ctx, expected.id)
		if err != nil {
			t.Fatalf("read restored operation %s through repository: %v", expected.id, err)
		}
		if operation.Status != expected.status {
			t.Fatalf("restored operation %s status = %q, want %q", expected.id, operation.Status, expected.status)
		}
		var input struct {
			TranscodePresetID string `json:"transcodePresetId"`
		}
		if err := json.Unmarshal([]byte(operation.InputJSON), &input); err != nil {
			t.Fatalf("decode restored operation %s input: %v", expected.id, err)
		}
		if input.TranscodePresetID != preset.ID {
			t.Fatalf("restored operation %s preset = %q, want %q", expected.id, input.TranscodePresetID, preset.ID)
		}
		if _, err := presetRepo.Get(ctx, input.TranscodePresetID); err != nil {
			t.Fatalf("resolve restored operation %s preset: %v", expected.id, err)
		}
	}

	service := libraryservice.NewLibraryService(
		nil, nil, nil, nil, nil,
		operationRepo, nil, nil, nil, nil,
		nil, nil, presetRepo, nil, nil,
		nil, nil, nil, nil,
	)
	if err := service.EnsureDefaultTranscodePresets(ctx); err != nil {
		t.Fatalf("normalize builtin presets after restore: %v", err)
	}
	if _, err := presetRepo.Get(ctx, preset.ID); err != nil {
		t.Fatalf("startup normalization removed restored custom preset: %v", err)
	}
	if _, err := presetRepo.Get(ctx, "builtin-obsolete-backup"); !errors.Is(err, library.ErrPresetNotFound) {
		t.Fatalf("obsolete restored builtin was not removed, error=%v", err)
	}
	if _, err := presetRepo.Get(ctx, "builtin-audio-aac-m4a-256k"); err != nil {
		t.Fatalf("canonical builtin was not normalized after restore: %v", err)
	}
	listedPresets, err := service.ListTranscodePresets(ctx)
	if err != nil {
		t.Fatalf("list presets through Library service after restore: %v", err)
	}
	customFound := false
	for _, listed := range listedPresets {
		if listed.ID == preset.ID {
			customFound = true
			break
		}
	}
	if !customFound {
		t.Fatalf("restored custom preset %q is not visible through Library service", preset.ID)
	}
}

func TestApplyRejectsMissingRollbackArtifactBeforeDestructiveSwap(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := fixture.manager.PlanRestore(ctx, target.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	rollbackManifest := filepath.Join(fixture.backupDir, backupPrefix+plan.RollbackBackupID+manifestSuffix)
	if err := os.Remove(rollbackManifest); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err == nil {
		t.Fatal("ApplyPendingRestore accepted a missing rollback artifact")
	}
	if _, err := os.Stat(fixture.databasePath); err != nil {
		t.Fatalf("current database was moved despite invalid rollback: %v", err)
	}
	if _, err := os.Stat(fixture.databasePath + ".restore-previous"); !os.IsNotExist(err) {
		t.Fatalf("destructive swap began before rollback validation: %v", err)
	}
}

func TestRestorePreviousDatabaseResumesPartialSidecarRollback(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "data.db")
	previousPath := databasePath + ".restore-previous"
	if err := os.WriteFile(previousPath, []byte("previous-main"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	// The old main has moved to previous. WAL still has the current basename,
	// while SHM already moved to previous: the legacy main-first crash state.
	if err := os.WriteFile(databasePath+"-wal", []byte("previous-wal"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(previousPath+"-shm", []byte("previous-shm"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	if err := restorePreviousDatabase(databasePath, previousPath, true, true); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		databasePath:          "previous-main",
		databasePath + "-wal": "previous-wal",
		databasePath + "-shm": "previous-shm",
	} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, %v; want %q", filepath.Base(path), data, err, want)
		}
	}
	if _, err := os.Stat(previousPath); !os.IsNotExist(err) {
		t.Fatalf("previous main remains after resumed rollback: %v", err)
	}
}

func TestRestoreOrphanPreviousSidecarsKeepsCurrentMain(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "data.db")
	previousPath := databasePath + ".restore-previous"
	if err := os.WriteFile(databasePath, []byte("current-main"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(previousPath+"-wal", []byte("old-wal"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(previousPath+"-shm", []byte("old-shm"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	if err := restoreOrphanPreviousSidecars(databasePath, previousPath, true, true); err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile(databasePath)
	if err != nil || string(main) != "current-main" {
		t.Fatalf("current main changed = %q, %v", main, err)
	}
	for path, want := range map[string]string{
		databasePath + "-wal": "old-wal",
		databasePath + "-shm": "old-shm",
	} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("orphan recovery %s = %q, %v", filepath.Base(path), data, err)
		}
	}
	for _, path := range []string{previousPath + "-wal", previousPath + "-shm"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("orphan previous sidecar remains: %s, %v", filepath.Base(path), err)
		}
	}
}

func TestRestoreFaultPointsRecoverWithoutLosingPreviousDatabase(t *testing.T) {
	for _, point := range []string{
		restoreFaultAfterPreviousMain,
		restoreFaultAfterCandidateMain,
		restoreFaultAfterInstalledMark,
	} {
		t.Run(point, func(t *testing.T) {
			fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
			ctx := context.Background()
			target, err := fixture.manager.Create(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_catalog_items
SET title = 'Current before fault', sort_title = 'Current before fault'
WHERE id = 'item-movie'
`); err != nil {
				t.Fatal(err)
			}
			if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
				t.Fatal(err)
			}
			if err := fixture.database.Close(); err != nil {
				t.Fatal(err)
			}
			faulted := false
			config := StartupRestoreConfig{
				DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
				RestoreMarkerPath: fixture.markerPath,
				testFault: func(actual string) error {
					if actual == point && !faulted {
						faulted = true
						return errors.New("power lost")
					}
					return nil
				},
			}
			if _, err := ApplyPendingRestore(ctx, config); err == nil || !faulted {
				t.Fatalf("fault %q was not injected, err=%v", point, err)
			}
			config.testFault = nil
			result, err := ApplyPendingRestore(ctx, config)
			if err != nil || !result.Applied {
				t.Fatalf("recovery after %q = %+v, %v", point, result, err)
			}
			restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
			if err != nil {
				t.Fatal(err)
			}
			var title string
			if err := restored.SQL.QueryRowContext(ctx, "SELECT title FROM library_catalog_items WHERE id = 'item-movie'").Scan(&title); err != nil {
				t.Fatal(err)
			}
			if title != "Movie" {
				t.Fatalf("recovered restore title after %q = %q", point, title)
			}
			if err := restored.Close(); err != nil {
				t.Fatal(err)
			}
			if err := FinalizePendingRestore(ctx, config); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOldSchemaRestoreMigratesInStagingAndFinalizesOnlyAfterOpen(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	downgradeBackupToSchemaSix(t, fixture, target, false)
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_catalog_items
SET title = 'Current after old backup', sort_title = 'Current after old backup'
WHERE id = 'item-movie'
`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil || !result.Applied {
		t.Fatalf("ApplyPendingRestore old schema = %+v, %v", result, err)
	}
	assertNoStagedMigrationSnapshots(t, fixture.databasePath)
	if _, err := os.Stat(fixture.databasePath + ".restore-previous"); err != nil {
		t.Fatalf("rollback main was removed before OpenSQLite: %v", err)
	}
	if _, err := os.Stat(fixture.markerPath); err != nil {
		t.Fatalf("installed marker was removed before OpenSQLite: %v", err)
	}
	readOnly, err := openReadOnlySQLite(fixture.databasePath)
	if err != nil {
		t.Fatal(err)
	}
	var stagedVersion int
	if err := readOnly.QueryRowContext(ctx, "PRAGMA user_version").Scan(&stagedVersion); err != nil {
		t.Fatal(err)
	}
	_ = readOnly.Close()
	if stagedVersion != persistence.CurrentSQLiteSchemaVersion() {
		t.Fatalf("installed staged schema version = %d", stagedVersion)
	}
	restored, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	var title string
	if err := restored.SQL.QueryRowContext(ctx, "SELECT title FROM library_catalog_items WHERE id = 'item-movie'").Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Movie" {
		t.Fatalf("old-schema restore title = %q", title)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	if err := FinalizePendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fixture.databasePath + ".restore-previous"); !os.IsNotExist(err) {
		t.Fatalf("rollback main remains after successful open/finalize: %v", err)
	}
}

func TestOldSchemaMigrationFailureLeavesCurrentDatabaseUntouched(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	ctx := context.Background()
	target, err := fixture.manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	downgradeBackupToSchemaSix(t, fixture, target, true)
	if _, err := fixture.database.SQL.ExecContext(ctx, `
UPDATE library_catalog_items
SET title = 'Current survives failed migration', sort_title = 'Current survives failed migration'
WHERE id = 'item-movie'
`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPendingRestore(ctx, StartupRestoreConfig{
		DatabasePath: fixture.databasePath, BackupDirectory: fixture.backupDir,
		RestoreMarkerPath: fixture.markerPath,
	}); err == nil {
		t.Fatal("malformed old schema unexpectedly migrated")
	}
	assertNoStagedMigrationSnapshots(t, fixture.databasePath)
	if _, err := os.Stat(fixture.databasePath + ".restore-previous"); !os.IsNotExist(err) {
		t.Fatalf("destructive swap began before staged migration succeeded: %v", err)
	}
	current, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: fixture.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	var title string
	if err := current.SQL.QueryRowContext(ctx, "SELECT title FROM library_catalog_items WHERE id = 'item-movie'").Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Current survives failed migration" {
		t.Fatalf("current database changed after staged migration failure: %q", title)
	}
}

func downgradeBackupToSchemaSix(t *testing.T, fixture *backupFixture, manifest domainbackup.Manifest, incompatibleRouteTable bool) {
	t.Helper()
	ctx := context.Background()
	databasePath := filepath.Join(fixture.backupDir, manifest.Database.FileName)
	database, err := sqlite3driver.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	statements := `
DROP TRIGGER library_catalog_sync_state_after_catalog_insert;
DROP TABLE library_catalog_sync_state;
DROP TABLE library_access_tailscale_route_audit;
DROP TABLE library_access_tailscale_route_state;
DELETE FROM schema_migrations WHERE version IN (7, 8);
PRAGMA user_version = 6;
`
	if incompatibleRouteTable {
		statements += "CREATE TABLE library_access_tailscale_route_state (id INTEGER PRIMARY KEY);\n"
	}
	if _, err := database.ExecContext(ctx, statements); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := restrictBackupFile(databasePath); err != nil {
		t.Fatal(err)
	}
	actual, err := inspectSnapshot(ctx, databasePath, snapshotInspection{
		ExpectedApplicationID: persistence.SQLiteApplicationID(),
		MaxSchemaVersion:      persistence.CurrentSQLiteSchemaVersion(),
	})
	if err != nil {
		t.Fatalf("inspect schema-six test backup: %v", err)
	}
	fileName := manifest.Database.FileName
	manifest.Database = actual.Database
	manifest.Database.FileName = fileName
	manifest.Catalogs = actual.Catalogs
	manifest.Files = actual.Files
	manifestPath := filepath.Join(fixture.backupDir, backupPrefix+manifest.BackupID+manifestSuffix)
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFileExclusive(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	if err := syncDirectory(fixture.backupDir); err != nil {
		t.Fatal(err)
	}
}

func assertNoStagedMigrationSnapshots(t *testing.T, databasePath string) {
	t.Helper()
	matches, err := filepath.Glob(databasePath + ".restore-source.pre-migration-v*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("staged migration snapshots remain: %v", matches)
	}
}

func TestConcurrentCreateIsSerializedAndPublishesCompletePairs(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{MaxBackups: 8, MaxAgeDays: 365})
	ctx := context.Background()
	const count = 4
	var wait sync.WaitGroup
	errorsChannel := make(chan error, count)
	wait.Add(count)
	for index := 0; index < count; index++ {
		go func() {
			defer wait.Done()
			_, err := fixture.manager.Create(ctx)
			errorsChannel <- err
		}()
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent Create: %v", err)
		}
	}
	backups, err := fixture.manager.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != count {
		t.Fatalf("backup count = %d, want %d", len(backups), count)
	}
	for _, backup := range backups {
		if backup.State != "ready" {
			t.Fatalf("incomplete backup published: %+v", backup)
		}
		if _, err := fixture.manager.Verify(ctx, backup.BackupID); err != nil {
			t.Fatalf("verify concurrent backup %s: %v", backup.BackupID, err)
		}
	}
}

func TestSafeLexicalRelativeIsCrossPlatformAndRejectsSibling(t *testing.T) {
	tests := []struct {
		root string
		file string
		want string
		ok   bool
	}{
		{"/library", "/library/video/a.mp4", "video/a.mp4", true},
		{"/library", "/library-other/a.mp4", "", false},
		{`C:\\Library`, `c:\\library\\Books\\a.epub`, "Books/a.epub", true},
		{`C:\\Library`, `D:\\Library\\a.epub`, "", false},
	}
	for _, test := range tests {
		got, ok := safeLexicalRelative(test.root, test.file)
		if got != test.want || ok != test.ok {
			t.Errorf("safeLexicalRelative(%q, %q) = %q, %t; want %q, %t", test.root, test.file, got, ok, test.want, test.ok)
		}
	}
}
