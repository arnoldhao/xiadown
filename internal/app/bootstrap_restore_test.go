package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"

	domainbackup "xiadown/internal/domain/librarybackup"
	infrastructurelibrarybackup "xiadown/internal/infrastructure/librarybackup"
	"xiadown/internal/infrastructure/persistence"
)

const libraryRestoreProcessHelperEnv = "XIADOWN_LIBRARY_RESTORE_PROCESS_HELPER"

// TestLibraryMetadataRestoreAcrossProcessRestart exercises the production
// startup boundary instead of only calling the restore primitives in the same
// process that created the backup. The child test process opens the database
// through openDatabaseAt, which applies the pending marker before SQLite is
// opened and finalizes the physical rollback only after OpenSQLite succeeds.
func TestLibraryMetadataRestoreAcrossProcessRestart(t *testing.T) {
	if os.Getenv(libraryRestoreProcessHelperEnv) == "1" {
		runLibraryMetadataRestoreProcessHelper(t)
		return
	}
	ctx := context.Background()
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "data.db")
	backupDirectory := filepath.Join(directory, "backups")
	markerPath := filepath.Join(directory, "restore.json")
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO settings (id, appearance, version) VALUES (1, 'light', 1);
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (
  'catalog', 'Library', '', 'active', TRUE,
  '2026-07-19T00:00:00Z', '2026-07-19T00:00:00Z'
);
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, revision, created_at, updated_at
) VALUES (
  'item', 'catalog', 'video', 'active', 'Backup title', 'Backup title', 1,
  '2026-07-19T00:00:00Z', '2026-07-19T00:00:00Z'
);
`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	manager, err := infrastructurelibrarybackup.NewManager(infrastructurelibrarybackup.Config{
		DB: database.SQL, DatabasePath: databasePath, BackupDirectory: backupDirectory,
		RestoreMarkerPath: markerPath, AppName: "XiaDown", AppVersion: "process-test",
	})
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	target, err := manager.Create(ctx)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE library_catalog_items
SET title = 'Current title', sort_title = 'Current title', revision = revision + 1
WHERE id = 'item';
UPDATE settings SET appearance = 'dark', version = version + 1 WHERE id = 1;
`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	plan, err := manager.PlanRestore(ctx, target.BackupID)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if plan.RollbackBackupID == "" || !plan.AppliesOnLaunch {
		_ = database.Close()
		t.Fatalf("restore plan = %+v", plan)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	childCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(childCtx, executable, "-test.run=^TestLibraryMetadataRestoreAcrossProcessRestart$", "-test.v")
	command.Env = append(os.Environ(),
		libraryRestoreProcessHelperEnv+"=1",
		"XIADOWN_LIBRARY_RESTORE_TEST_DATABASE="+databasePath,
		"XIADOWN_LIBRARY_RESTORE_TEST_BACKUPS="+backupDirectory,
		"XIADOWN_LIBRARY_RESTORE_TEST_MARKER="+markerPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("restored startup process failed: %v\n%s", err, output)
	}
	if childCtx.Err() != nil {
		t.Fatalf("restored startup process timed out: %v\n%s", childCtx.Err(), output)
	}

	for _, artifact := range []string{
		markerPath,
		databasePath + ".restore-previous",
		databasePath + ".restore-staging",
		databasePath + ".restore-working",
		databasePath + ".restore-source",
	} {
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatalf("completed process restore left %s: %v", filepath.Base(artifact), err)
		}
	}
	reopened, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	assertRestoredProcessState(t, reopened, "Backup title", "dark")

	// Planning a restore creates a committed snapshot of the pre-restore state.
	// Keep proving that recovery point remains valid after successful startup;
	// it is a separate metadata backup, not the temporary physical swap file.
	reopenedManager, err := infrastructurelibrarybackup.NewManager(infrastructurelibrarybackup.Config{
		DB: reopened.SQL, DatabasePath: databasePath, BackupDirectory: backupDirectory,
		RestoreMarkerPath: markerPath, AppName: "XiaDown", AppVersion: "process-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if verification, err := reopenedManager.Verify(ctx, plan.RollbackBackupID); err != nil || !verification.Valid {
		t.Fatalf("pre-restore rollback backup = %+v, %v", verification, err)
	}
}

func runLibraryMetadataRestoreProcessHelper(t *testing.T) {
	database, err := openDatabaseAt(
		context.Background(),
		os.Getenv("XIADOWN_LIBRARY_RESTORE_TEST_DATABASE"),
		os.Getenv("XIADOWN_LIBRARY_RESTORE_TEST_BACKUPS"),
		os.Getenv("XIADOWN_LIBRARY_RESTORE_TEST_MARKER"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	assertRestoredProcessState(t, database, "Backup title", "dark")
}

func assertRestoredProcessState(t *testing.T, database *persistence.Database, wantTitle, wantAppearance string) {
	t.Helper()
	var title string
	if err := database.SQL.QueryRowContext(context.Background(), `
SELECT title FROM library_catalog_items WHERE id = 'item'
`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != wantTitle {
		t.Fatalf("restored Library title = %q, want %q", title, wantTitle)
	}
	var appearance string
	if err := database.SQL.QueryRowContext(context.Background(), `
SELECT appearance FROM settings WHERE id = 1
`).Scan(&appearance); err != nil {
		t.Fatal(err)
	}
	if appearance != wantAppearance {
		t.Fatalf("preserved application setting = %q, want %q", appearance, wantAppearance)
	}
}

// TestLibraryMetadataRestoreFromExternalBackupIsolated is an opt-in drill for
// a real user-created backup. It copies the committed pair into t.TempDir and
// performs every verify, plan, replacement, and cleanup operation there. The
// source manifest, source snapshot, and real XiaDown data.db are read-only and
// can never become restore targets.
func TestLibraryMetadataRestoreFromExternalBackupIsolated(t *testing.T) {
	externalManifestPath := strings.TrimSpace(os.Getenv("XIADOWN_LIBRARY_RESTORE_EXTERNAL_MANIFEST"))
	if externalManifestPath == "" {
		t.Skip("set XIADOWN_LIBRARY_RESTORE_EXTERNAL_MANIFEST to run the isolated external-backup drill")
	}
	externalManifestPath = requireRegularExternalRestoreFile(t, externalManifestPath)
	manifestBytes, err := os.ReadFile(externalManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest domainbackup.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode external backup manifest: %v", err)
	}
	if !validExternalRestoreBackupID(manifest.BackupID) {
		t.Fatalf("external backup ID is unsafe: %q", manifest.BackupID)
	}
	expectedBase := "xiadown-library-metadata-v1-" + manifest.BackupID
	if filepath.Base(externalManifestPath) != expectedBase+".manifest.json" {
		t.Fatalf("external manifest name does not match its backup ID: %s", filepath.Base(externalManifestPath))
	}
	if manifest.Database.FileName != expectedBase+".sqlite" {
		t.Fatalf("external backup database name does not match its backup ID: %q", manifest.Database.FileName)
	}
	externalDatabasePath := requireRegularExternalRestoreFile(
		t,
		filepath.Join(filepath.Dir(externalManifestPath), manifest.Database.FileName),
	)
	externalManifestHash := restoreTestFileSHA256(t, externalManifestPath)
	externalDatabaseHash := restoreTestFileSHA256(t, externalDatabasePath)
	t.Cleanup(func() {
		if got := restoreTestFileSHA256(t, externalManifestPath); got != externalManifestHash {
			t.Errorf("external manifest changed during isolated restore drill")
		}
		if got := restoreTestFileSHA256(t, externalDatabasePath); got != externalDatabaseHash {
			t.Errorf("external SQLite snapshot changed during isolated restore drill")
		}
	})

	temporaryRoot := t.TempDir()
	backupDirectory := filepath.Join(temporaryRoot, "backups")
	if err := os.Mkdir(backupDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	temporaryManifestPath := filepath.Join(backupDirectory, "xiadown-library-metadata-v1-"+manifest.BackupID+".manifest.json")
	temporaryDatabasePath := filepath.Join(backupDirectory, manifest.Database.FileName)
	for _, target := range []string{temporaryManifestPath, temporaryDatabasePath} {
		if filepath.Dir(filepath.Clean(target)) != filepath.Clean(backupDirectory) {
			t.Fatalf("external restore fixture target escaped its temporary backup directory: %s", target)
		}
	}
	copyExternalRestoreFixtureFile(t, externalManifestPath, temporaryManifestPath)
	copyExternalRestoreFixtureFile(t, externalDatabasePath, temporaryDatabasePath)

	source := openRestoreTestSQLiteReadOnly(t, temporaryDatabasePath)
	sourceCounts := externalRestoreMetadataCounts(t, source)
	sourceItem := externalRestoreItemProbe(t, source)
	sourceTrack := externalRestoreTrackProbe(t, source)
	sourceLiveChannel := externalRestoreLiveChannelProbe(t, source)
	assertRestoreTestSQLiteHealth(t, source)
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	databasePath := filepath.Join(temporaryRoot, "current.db")
	markerPath := filepath.Join(temporaryRoot, "restore-next-launch.json")
	current, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := current.SQL.ExecContext(ctx, `
INSERT INTO settings (id, appearance, version) VALUES (1, 'dark', 1);
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (
  'external-restore-current-sentinel', 'Current sentinel', '', 'active', TRUE,
  '2026-07-19T00:00:00Z', '2026-07-19T00:00:00Z'
);
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, revision, created_at, updated_at
) VALUES (
  'external-restore-current-sentinel', 'external-restore-current-sentinel', 'video',
  'active', 'CURRENT SENTINEL', 'CURRENT SENTINEL', 1,
  '2026-07-19T00:00:00Z', '2026-07-19T00:00:00Z'
);
`); err != nil {
		_ = current.Close()
		t.Fatal(err)
	}
	manager, err := infrastructurelibrarybackup.NewManager(infrastructurelibrarybackup.Config{
		DB: current.SQL, DatabasePath: databasePath, BackupDirectory: backupDirectory,
		RestoreMarkerPath: markerPath, AppName: "XiaDown", AppVersion: "external-restore-test",
	})
	if err != nil {
		_ = current.Close()
		t.Fatal(err)
	}
	verification, err := manager.Verify(ctx, manifest.BackupID)
	if err != nil || !verification.Valid {
		_ = current.Close()
		t.Fatalf("verify copied external backup = %+v, %v", verification, err)
	}
	if verification.DatabaseSHA256 != manifest.Database.SHA256 {
		_ = current.Close()
		t.Fatalf("verified external snapshot SHA-256 = %q, manifest = %q", verification.DatabaseSHA256, manifest.Database.SHA256)
	}
	plan, err := manager.PlanRestore(ctx, manifest.BackupID)
	if err != nil {
		_ = current.Close()
		t.Fatal(err)
	}
	if plan.RollbackBackupID == "" || !plan.AppliesOnLaunch {
		_ = current.Close()
		t.Fatalf("external restore plan = %+v", plan)
	}
	if err := current.Close(); err != nil {
		t.Fatal(err)
	}

	restored, err := openDatabaseAt(ctx, databasePath, backupDirectory, markerPath)
	if err != nil {
		t.Fatalf("production startup restore of copied external backup: %v", err)
	}
	defer restored.Close()
	if counts := externalRestoreMetadataCounts(t, restored.SQL); !equalRestoreMetadataCounts(counts, sourceCounts) {
		t.Fatalf("restored metadata counts = %v, source snapshot counts = %v", counts, sourceCounts)
	}
	t.Logf("restored external metadata counts: %v", sourceCounts)
	if item := externalRestoreItemProbe(t, restored.SQL); item != sourceItem {
		t.Fatalf("restored catalog item probe = %+v, source = %+v", item, sourceItem)
	}
	if track := externalRestoreTrackProbe(t, restored.SQL); track != sourceTrack {
		t.Fatalf("restored local track probe = %+v, source = %+v", track, sourceTrack)
	}
	if channel := externalRestoreLiveChannelProbe(t, restored.SQL); channel != sourceLiveChannel {
		t.Fatalf("restored live channel probe = %+v, source = %+v", channel, sourceLiveChannel)
	}
	var appearance string
	if err := restored.SQL.QueryRowContext(ctx, "SELECT appearance FROM settings WHERE id = 1").Scan(&appearance); err != nil {
		t.Fatal(err)
	}
	if appearance != "dark" {
		t.Fatalf("current settings were rolled back: appearance = %q", appearance)
	}
	var sentinelCount int
	if err := restored.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_catalog_items WHERE id = 'external-restore-current-sentinel'
`).Scan(&sentinelCount); err != nil {
		t.Fatal(err)
	}
	if sentinelCount != 0 {
		t.Fatalf("current Library sentinel survived metadata restore")
	}
	assertRestoreTestSQLiteHealth(t, restored.SQL)
	for _, artifact := range []string{
		markerPath,
		databasePath + ".restore-previous",
		databasePath + ".restore-staging",
		databasePath + ".restore-working",
		databasePath + ".restore-source",
		databasePath + ".restore-source.migrated",
	} {
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatalf("completed external restore left %s: %v", filepath.Base(artifact), err)
		}
	}
	restoredManager, err := infrastructurelibrarybackup.NewManager(infrastructurelibrarybackup.Config{
		DB: restored.SQL, DatabasePath: databasePath, BackupDirectory: backupDirectory,
		RestoreMarkerPath: markerPath, AppName: "XiaDown", AppVersion: "external-restore-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rollback, err := restoredManager.Verify(ctx, plan.RollbackBackupID); err != nil || !rollback.Valid {
		t.Fatalf("verify external-drill rollback backup = %+v, %v", rollback, err)
	}
}

var externalRestoreCountTables = []string{
	"library_catalog_items",
	"library_files",
	"library_item_assets",
	"listen_local_tracks",
	"listen_live_columns",
	"listen_live_channels",
	"rss_categories",
	"rss_collections",
	"rss_collection_subscriptions",
	"rss_collection_entries",
	"rss_sources",
}

type externalRestoreCatalogItemProbe struct {
	Present  bool
	ID       string
	Title    string
	Revision int64
}

func externalRestoreItemProbe(t *testing.T, database *sql.DB) externalRestoreCatalogItemProbe {
	t.Helper()
	probe := externalRestoreCatalogItemProbe{}
	err := database.QueryRowContext(context.Background(), `
SELECT id, title, revision
FROM library_catalog_items
ORDER BY id
LIMIT 1
`).Scan(&probe.ID, &probe.Title, &probe.Revision)
	if err == sql.ErrNoRows {
		return probe
	}
	if err != nil {
		t.Fatal(err)
	}
	probe.Present = true
	return probe
}

type externalRestoreLocalTrackProbe struct {
	Present      bool
	FileID       string
	Title        string
	Author       sql.NullString
	Album        sql.NullString
	AlbumArtist  sql.NullString
	Genre        sql.NullString
	TrackNumber  sql.NullInt64
	DiscNumber   sql.NullInt64
	Year         sql.NullInt64
	ModTimeUnix  int64
	Availability string
}

func externalRestoreTrackProbe(t *testing.T, database *sql.DB) externalRestoreLocalTrackProbe {
	t.Helper()
	probe := externalRestoreLocalTrackProbe{}
	err := database.QueryRowContext(context.Background(), `
SELECT file_id, title, author, album, album_artist, genre,
       track_number, disc_number, year, mod_time_unix, availability
FROM listen_local_tracks
ORDER BY file_id
LIMIT 1
`).Scan(
		&probe.FileID, &probe.Title, &probe.Author,
		&probe.Album, &probe.AlbumArtist, &probe.Genre,
		&probe.TrackNumber, &probe.DiscNumber, &probe.Year,
		&probe.ModTimeUnix, &probe.Availability,
	)
	if err == sql.ErrNoRows {
		return probe
	}
	if err != nil {
		t.Fatal(err)
	}
	probe.Present = true
	return probe
}

type externalRestoreLiveChannelSnapshot struct {
	Present      bool
	ID           string
	ColumnID     string
	Title        string
	Channel      string
	Description  string
	Source       string
	VideoID      string
	ThumbnailURL string
	Enabled      int64
	SortOrder    int64
}

func externalRestoreLiveChannelProbe(t *testing.T, database *sql.DB) externalRestoreLiveChannelSnapshot {
	t.Helper()
	probe := externalRestoreLiveChannelSnapshot{}
	err := database.QueryRowContext(context.Background(), `
SELECT id, column_id, title, channel, description, source, video_id,
       thumbnail_url, CAST(enabled AS INTEGER), sort_order
FROM listen_live_channels
ORDER BY id
LIMIT 1
`).Scan(
		&probe.ID, &probe.ColumnID, &probe.Title, &probe.Channel,
		&probe.Description, &probe.Source, &probe.VideoID,
		&probe.ThumbnailURL, &probe.Enabled, &probe.SortOrder,
	)
	if err == sql.ErrNoRows {
		return probe
	}
	if err != nil {
		t.Fatal(err)
	}
	probe.Present = true
	return probe
}

func externalRestoreMetadataCounts(t *testing.T, database *sql.DB) map[string]int64 {
	t.Helper()
	counts := make(map[string]int64, len(externalRestoreCountTables))
	for _, table := range externalRestoreCountTables {
		var count int64
		if err := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

func equalRestoreMetadataCounts(left, right map[string]int64) bool {
	for _, table := range externalRestoreCountTables {
		if left[table] != right[table] {
			return false
		}
	}
	return true
}

func assertRestoreTestSQLiteHealth(t *testing.T, database *sql.DB) {
	t.Helper()
	quickRows, err := database.QueryContext(context.Background(), "PRAGMA quick_check")
	if err != nil {
		t.Fatal(err)
	}
	var quickResults []string
	for quickRows.Next() {
		var result string
		if err := quickRows.Scan(&result); err != nil {
			_ = quickRows.Close()
			t.Fatal(err)
		}
		quickResults = append(quickResults, result)
	}
	if err := quickRows.Err(); err != nil {
		_ = quickRows.Close()
		t.Fatal(err)
	}
	if err := quickRows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(quickResults) != 1 || quickResults[0] != "ok" {
		t.Fatalf("SQLite quick_check = %v", quickResults)
	}
	foreignKeys, err := database.QueryContext(context.Background(), "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if foreignKeys.Next() {
		_ = foreignKeys.Close()
		t.Fatal("SQLite foreign_key_check reported a violation")
	}
	if err := foreignKeys.Err(); err != nil {
		_ = foreignKeys.Close()
		t.Fatal(err)
	}
	if err := foreignKeys.Close(); err != nil {
		t.Fatal(err)
	}
}

func requireRegularExternalRestoreFile(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("external restore fixture is not a regular file: %s", absolute)
	}
	return absolute
}

func validExternalRestoreBackupID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func copyExternalRestoreFixtureFile(t *testing.T, sourcePath, targetPath string) {
	t.Helper()
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = source.Close()
		t.Fatal(err)
	}
	_, copyErr := io.Copy(target, source)
	syncErr := target.Sync()
	targetCloseErr := target.Close()
	sourceCloseErr := source.Close()
	for _, operation := range []struct {
		name string
		err  error
	}{
		{"copy", copyErr},
		{"sync", syncErr},
		{"close target", targetCloseErr},
		{"close source", sourceCloseErr},
	} {
		if operation.err != nil {
			t.Fatalf("%s external restore fixture: %v", operation.name, operation.err)
		}
	}
}

func restoreTestFileSHA256(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func openRestoreTestSQLiteReadOnly(t *testing.T, path string) *sql.DB {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	uriPath := filepath.ToSlash(absolute)
	var uri string
	if volume := filepath.VolumeName(absolute); volume != "" && !strings.HasPrefix(uriPath, "//") {
		uri = "file:" + (&url.URL{Path: uriPath}).EscapedPath()
	} else {
		uri = (&url.URL{Scheme: "file", Path: uriPath}).String()
	}
	uri += "?mode=ro&immutable=1"
	database, err := sqlite3driver.Open(uri)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func TestOpenDatabaseAtAutomaticallyRollsBackInvalidInstalledRestore(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "data.db")
	backupDirectory := filepath.Join(directory, "backups")
	markerPath := filepath.Join(directory, "restore.json")
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_catalogs (
  id, name, description, status, is_default, created_at, updated_at
) VALUES (
  'catalog', 'Library', '', 'active', TRUE,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, revision, created_at, updated_at
) VALUES (
  'item', 'catalog', 'video', 'active', 'Backup title', 'Backup title', 1,
  '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z'
);
`); err != nil {
		t.Fatal(err)
	}
	manager, err := infrastructurelibrarybackup.NewManager(infrastructurelibrarybackup.Config{
		DB: database.SQL, DatabasePath: databasePath, BackupDirectory: backupDirectory,
		RestoreMarkerPath: markerPath, AppName: "XiaDown", AppVersion: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := manager.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE library_catalog_items
SET title = 'Current title', sort_title = 'Current title', revision = revision + 1
WHERE id = 'item'
`); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PlanRestore(ctx, target.BackupID); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := infrastructurelibrarybackup.ApplyPendingRestore(ctx, infrastructurelibrarybackup.StartupRestoreConfig{
		DatabasePath: databasePath, BackupDirectory: backupDirectory, RestoreMarkerPath: markerPath,
	})
	if err != nil || !result.Applied {
		t.Fatalf("install restore = %+v, %v", result, err)
	}
	file, err := os.OpenFile(databasePath, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("not-a-sqlite-database"), 0); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	recovered, err := openDatabaseAt(ctx, databasePath, backupDirectory, markerPath)
	if err != nil {
		t.Fatalf("openDatabaseAt should launch from the automatic rollback: %v", err)
	}
	defer recovered.Close()
	var title string
	if err := recovered.SQL.QueryRowContext(ctx, "SELECT title FROM library_catalog_items WHERE id = 'item'").Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Current title" {
		t.Fatalf("automatic rollback title = %q", title)
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("failed restore marker remains after automatic rollback: %v", err)
	}
	if _, err := os.Stat(databasePath + ".restore-previous"); !os.IsNotExist(err) {
		t.Fatalf("physical previous database remains after automatic rollback: %v", err)
	}
}
