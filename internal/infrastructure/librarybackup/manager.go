package librarybackup

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"

	domainbackup "xiadown/internal/domain/librarybackup"
	"xiadown/internal/infrastructure/persistence"
)

const (
	backupPrefix         = "xiadown-library-metadata-v1-"
	manifestSuffix       = ".manifest.json"
	databaseSuffix       = ".sqlite"
	defaultMaxBackups    = 14
	defaultMaxAge        = 90 * 24 * time.Hour
	maximumMaxBackups    = 10_000
	maximumMaxAgeDays    = 36_500
	maximumMaxAge        = time.Duration(maximumMaxAgeDays) * 24 * time.Hour
	defaultRestoreMarker = ".restore-next-launch.json"
	backupDirectoryMode  = 0o700
	backupFileMode       = 0o600
)

type Config struct {
	DB                        *sql.DB
	DatabasePath              string
	BackupDirectory           string
	RestoreMarkerPath         string
	AppName                   string
	AppVersion                string
	ExpectedApplicationID     int64
	MaxSupportedSchemaVersion int
	Retention                 domainbackup.RetentionPolicy
	Clock                     func() time.Time
}

type Manager struct {
	db                *sql.DB
	databasePath      string
	backupDirectory   string
	restoreMarkerPath string
	appName           string
	appVersion        string
	expectedAppID     int64
	maxSchemaVersion  int
	retention         domainbackup.RetentionPolicy
	clock             func() time.Time

	mu sync.Mutex
}

func NewManager(config Config) (*Manager, error) {
	if config.DB == nil || strings.TrimSpace(config.DatabasePath) == "" || strings.TrimSpace(config.BackupDirectory) == "" {
		return nil, domainbackup.ErrInvalidConfiguration
	}
	databasePath, err := filepath.Abs(config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("resolve library database path: %w", err)
	}
	backupDirectory, err := filepath.Abs(config.BackupDirectory)
	if err != nil {
		return nil, fmt.Errorf("resolve library backup directory: %w", err)
	}
	markerPath := strings.TrimSpace(config.RestoreMarkerPath)
	if markerPath == "" {
		markerPath = databasePath + defaultRestoreMarker
	}
	markerPath, err = filepath.Abs(markerPath)
	if err != nil {
		return nil, fmt.Errorf("resolve restore marker path: %w", err)
	}
	if filepath.Clean(markerPath) == filepath.Clean(databasePath) {
		return nil, domainbackup.ErrInvalidConfiguration
	}
	if err := verifyLiveDatabasePath(config.DB, databasePath); err != nil {
		return nil, err
	}

	expectedAppID := config.ExpectedApplicationID
	if expectedAppID == 0 {
		expectedAppID = persistence.SQLiteApplicationID()
	}
	maxSchema := config.MaxSupportedSchemaVersion
	if maxSchema == 0 {
		maxSchema = persistence.CurrentSQLiteSchemaVersion()
	}
	retention, err := normalizeRetention(config.Retention)
	if err != nil {
		return nil, err
	}
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}
	appName := strings.TrimSpace(config.AppName)
	if appName == "" {
		appName = "XiaDown"
	}
	appVersion := strings.TrimSpace(config.AppVersion)
	if appVersion == "" {
		appVersion = "unknown"
	}
	if expectedAppID <= 0 || maxSchema <= 0 {
		return nil, domainbackup.ErrInvalidConfiguration
	}
	if err := ensurePrivateDirectory(backupDirectory); err != nil {
		return nil, err
	}
	if err := ensurePrivateDirectory(filepath.Dir(markerPath)); err != nil {
		return nil, err
	}
	manager := &Manager{
		db: config.DB, databasePath: databasePath, backupDirectory: backupDirectory,
		restoreMarkerPath: markerPath, appName: appName, appVersion: appVersion,
		expectedAppID: expectedAppID, maxSchemaVersion: maxSchema,
		retention: retention, clock: clock,
	}
	if err := manager.reconcileBackupDirectoryLocked(); err != nil {
		return nil, fmt.Errorf("reconcile metadata backup directory: %w", err)
	}
	return manager, nil
}

func verifyLiveDatabasePath(db *sql.DB, configuredPath string) error {
	var actualPath string
	if err := db.QueryRow("SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&actualPath); err != nil {
		return fmt.Errorf("inspect live SQLite database path: %w", err)
	}
	actualPath, err := filepath.Abs(actualPath)
	if err != nil || strings.TrimSpace(actualPath) == "" {
		return domainbackup.ErrInvalidConfiguration
	}
	configuredInfo, err := os.Stat(configuredPath)
	if err != nil {
		return fmt.Errorf("inspect configured SQLite database: %w", err)
	}
	actualInfo, err := os.Stat(actualPath)
	if err != nil {
		return fmt.Errorf("inspect live SQLite database: %w", err)
	}
	if !configuredInfo.Mode().IsRegular() || !actualInfo.Mode().IsRegular() || !os.SameFile(configuredInfo, actualInfo) {
		return fmt.Errorf("%w: configured path does not identify the live SQLite database", domainbackup.ErrInvalidConfiguration)
	}
	return nil
}

func normalizeRetention(policy domainbackup.RetentionPolicy) (domainbackup.RetentionPolicy, error) {
	if policy.MaxBackups < 0 || policy.MaxBackups > maximumMaxBackups ||
		policy.MaxAge < 0 || policy.MaxAge > maximumMaxAge ||
		policy.MaxAgeDays < 0 || policy.MaxAgeDays > maximumMaxAgeDays {
		return domainbackup.RetentionPolicy{}, domainbackup.ErrInvalidConfiguration
	}
	if policy.MaxBackups == 0 {
		policy.MaxBackups = defaultMaxBackups
	}
	if policy.MaxAge == 0 {
		if policy.MaxAgeDays > 0 {
			policy.MaxAge = time.Duration(policy.MaxAgeDays) * 24 * time.Hour
		} else {
			policy.MaxAge = defaultMaxAge
		}
	}
	if policy.MaxAgeDays == 0 {
		policy.MaxAgeDays = int(policy.MaxAge / (24 * time.Hour))
	}
	return policy, nil
}

func (manager *Manager) Create(ctx context.Context) (domainbackup.Manifest, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	manifest, err := manager.createLocked(ctx, domainbackup.PurposeUser)
	if err != nil {
		return domainbackup.Manifest{}, err
	}
	if err := manager.pruneLocked(); err != nil {
		return manifest, fmt.Errorf("metadata backup %s created but retention cleanup failed: %w", manifest.BackupID, err)
	}
	return manifest, nil
}

func (manager *Manager) createLocked(ctx context.Context, purpose string) (domainbackup.Manifest, error) {
	if err := ensurePrivateDirectory(manager.backupDirectory); err != nil {
		return domainbackup.Manifest{}, err
	}
	backupID, err := newBackupID(manager.clock().UTC())
	if err != nil {
		return domainbackup.Manifest{}, err
	}
	base := backupPrefix + backupID
	databaseName := base + databaseSuffix
	manifestName := base + manifestSuffix
	databasePath := filepath.Join(manager.backupDirectory, databaseName)
	manifestPath := filepath.Join(manager.backupDirectory, manifestName)
	temporaryDatabase := filepath.Join(manager.backupDirectory, "."+base+"-"+randomSuffix()+".sqlite.tmp")
	temporaryManifest := filepath.Join(manager.backupDirectory, "."+base+"-"+randomSuffix()+".manifest.tmp")
	cleanup := func() {
		_ = os.Remove(temporaryDatabase)
		_ = os.Remove(temporaryManifest)
	}
	defer cleanup()

	if err := persistence.CreateConsistentSQLiteSnapshot(ctx, manager.db, temporaryDatabase); err != nil {
		return domainbackup.Manifest{}, err
	}
	if err := restrictBackupFile(temporaryDatabase); err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("set metadata snapshot permissions: %w", err)
	}
	if err := syncFile(temporaryDatabase); err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("sync metadata snapshot: %w", err)
	}

	manifest, err := inspectSnapshot(ctx, temporaryDatabase, snapshotInspection{
		ExpectedApplicationID: manager.expectedAppID,
		MaxSchemaVersion:      manager.maxSchemaVersion,
	})
	if err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("inspect metadata snapshot: %w", err)
	}
	manifest.FormatVersion = domainbackup.ManifestFormatVersion
	manifest.BackupID = backupID
	manifest.Purpose = purpose
	manifest.AppName = manager.appName
	manifest.AppVersion = manager.appVersion
	manifest.CreatedAt = manager.clock().UTC()
	manifest.MetadataOnly = true
	manifest.ContentIncluded = false
	manifest.Database.FileName = databaseName

	if err := writeJSONFileExclusive(temporaryManifest, manifest); err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("write metadata backup manifest: %w", err)
	}
	if err := ensureDestinationAbsent(databasePath, manifestPath); err != nil {
		return domainbackup.Manifest{}, err
	}
	if err := durableRename(temporaryDatabase, databasePath, false); err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("publish metadata snapshot: %w", err)
	}
	if err := syncDirectory(manager.backupDirectory); err != nil {
		_ = removeBackupArtifactDurably(databasePath)
		return domainbackup.Manifest{}, fmt.Errorf("sync metadata backup directory: %w", err)
	}
	// The manifest is the commit record and is deliberately published last.
	if err := durableRename(temporaryManifest, manifestPath, false); err != nil {
		_ = removeBackupArtifactDurably(databasePath)
		return domainbackup.Manifest{}, fmt.Errorf("publish metadata backup manifest: %w", err)
	}
	if err := syncDirectory(manager.backupDirectory); err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("sync published metadata backup: %w", err)
	}
	return manifest, nil
}

func (manager *Manager) List(ctx context.Context) ([]domainbackup.BackupSummary, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.listLocked(ctx)
}

func (manager *Manager) listLocked(_ context.Context) ([]domainbackup.BackupSummary, error) {
	entries, err := os.ReadDir(manager.backupDirectory)
	if err != nil {
		return nil, fmt.Errorf("list metadata backups: %w", err)
	}
	result := make([]domainbackup.BackupSummary, 0)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !isManifestName(entry.Name()) {
			continue
		}
		backupID, ok := backupIDFromManifestName(entry.Name())
		if !ok {
			continue
		}
		manifest, err := readManifest(filepath.Join(manager.backupDirectory, entry.Name()))
		if err != nil {
			info, _ := entry.Info()
			createdAt := time.Time{}
			if info != nil {
				createdAt = info.ModTime().UTC()
			}
			result = append(result, domainbackup.BackupSummary{
				BackupID: backupID, CreatedAt: createdAt, State: "invalid", Error: "manifest is unreadable",
			})
			continue
		}
		if err := validateManifestNames(manifest, backupID, entry.Name()); err != nil {
			result = append(result, domainbackup.BackupSummary{
				BackupID: backupID, CreatedAt: manifest.CreatedAt, State: "invalid", Error: "manifest identity is invalid",
			})
			continue
		}
		databasePath := filepath.Join(manager.backupDirectory, manifest.Database.FileName)
		databaseInfo, err := os.Lstat(databasePath)
		if err != nil || !databaseInfo.Mode().IsRegular() || databaseInfo.Mode()&os.ModeSymlink != 0 || databaseInfo.Size() != manifest.Database.SizeBytes {
			result = append(result, domainbackup.BackupSummary{
				BackupID: backupID, CreatedAt: manifest.CreatedAt, State: "invalid", Error: "snapshot is missing or has the wrong size",
			})
			continue
		}
		catalogIDs := make([]string, 0, len(manifest.Catalogs))
		for _, catalog := range manifest.Catalogs {
			catalogIDs = append(catalogIDs, catalog.ID)
		}
		result = append(result, domainbackup.BackupSummary{
			BackupID: manifest.BackupID, Purpose: manifest.Purpose, AppVersion: manifest.AppVersion,
			SchemaVersion: manifest.Database.SchemaVersion, CatalogIDs: catalogIDs,
			CreatedAt: manifest.CreatedAt, SizeBytes: manifest.Database.SizeBytes,
			MetadataOnly: manifest.MetadataOnly, ContentIncluded: manifest.ContentIncluded, State: "ready",
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].BackupID > result[j].BackupID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (manager *Manager) Verify(ctx context.Context, backupID string) (domainbackup.VerificationResult, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.verifyLocked(ctx, backupID)
}

func (manager *Manager) verifyLocked(ctx context.Context, backupID string) (domainbackup.VerificationResult, error) {
	manifest, _, databasePath, err := manager.resolveBackup(backupID)
	if err != nil {
		return domainbackup.VerificationResult{}, err
	}
	actual, err := verifyManifestSnapshot(ctx, manifest, databasePath, snapshotInspection{
		ExpectedApplicationID: manager.expectedAppID,
		MaxSchemaVersion:      manager.maxSchemaVersion,
	})
	if err != nil {
		return domainbackup.VerificationResult{}, err
	}
	return domainbackup.VerificationResult{
		BackupID: backupID, VerifiedAt: manager.clock().UTC(), Valid: true,
		ApplicationID: actual.Database.ApplicationID, SchemaVersion: actual.Database.SchemaVersion,
		DatabaseSHA256: actual.Database.SHA256,
	}, nil
}

func verifyManifestSnapshot(
	ctx context.Context,
	manifest domainbackup.Manifest,
	databasePath string,
	expected snapshotInspection,
) (domainbackup.Manifest, error) {
	actual, err := inspectSnapshot(ctx, databasePath, expected)
	if err != nil {
		return domainbackup.Manifest{}, err
	}
	if manifest.Database.SHA256 != actual.Database.SHA256 ||
		manifest.Database.SizeBytes != actual.Database.SizeBytes ||
		manifest.Database.ApplicationID != actual.Database.ApplicationID ||
		manifest.Database.SchemaVersion != actual.Database.SchemaVersion ||
		!reflect.DeepEqual(manifest.Catalogs, actual.Catalogs) ||
		!reflect.DeepEqual(manifest.Files, actual.Files) {
		return domainbackup.Manifest{}, fmt.Errorf("%w: snapshot does not match its manifest", domainbackup.ErrInvalidBackup)
	}
	return actual, nil
}

func (manager *Manager) resolveBackup(backupID string) (domainbackup.Manifest, string, string, error) {
	if !validBackupID(backupID) {
		return domainbackup.Manifest{}, "", "", domainbackup.ErrBackupNotFound
	}
	manifestName := backupPrefix + backupID + manifestSuffix
	manifestPath := filepath.Join(manager.backupDirectory, manifestName)
	manifest, err := readManifest(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return domainbackup.Manifest{}, "", "", domainbackup.ErrBackupNotFound
		}
		return domainbackup.Manifest{}, "", "", fmt.Errorf("%w: %v", domainbackup.ErrInvalidBackup, err)
	}
	if err := validateManifestNames(manifest, backupID, manifestName); err != nil {
		return domainbackup.Manifest{}, "", "", err
	}
	databasePath := filepath.Join(manager.backupDirectory, manifest.Database.FileName)
	if err := requireRegularPrivateFile(databasePath); err != nil {
		if os.IsNotExist(err) {
			return domainbackup.Manifest{}, "", "", fmt.Errorf("%w: snapshot file is missing", domainbackup.ErrInvalidBackup)
		}
		return domainbackup.Manifest{}, "", "", err
	}
	return manifest, manifestPath, databasePath, nil
}

func inspectSnapshot(ctx context.Context, path string, expected snapshotInspection) (domainbackup.Manifest, error) {
	if err := requireRegularPrivateFile(path); err != nil {
		return domainbackup.Manifest{}, err
	}
	hash, size, err := fileSHA256(path)
	if err != nil {
		return domainbackup.Manifest{}, err
	}
	db, err := openReadOnlySQLite(path)
	if err != nil {
		return domainbackup.Manifest{}, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("open metadata snapshot: %w", err)
	}
	if err := persistence.VerifySQLiteIntegrity(ctx, db, true); err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("%w: %v", domainbackup.ErrInvalidBackup, err)
	}
	if err := persistence.VerifySQLiteMigrationLedger(ctx, db); err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("%w: %v", domainbackup.ErrIncompatibleBackup, err)
	}
	var applicationID int64
	var schemaVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		return domainbackup.Manifest{}, err
	}
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schemaVersion); err != nil {
		return domainbackup.Manifest{}, err
	}
	if applicationID != expected.ExpectedApplicationID {
		return domainbackup.Manifest{}, fmt.Errorf("%w: SQLite application_id %d", domainbackup.ErrIncompatibleBackup, applicationID)
	}
	if schemaVersion <= 0 || schemaVersion > expected.MaxSchemaVersion {
		return domainbackup.Manifest{}, fmt.Errorf("%w: SQLite schema version %d is not supported", domainbackup.ErrIncompatibleBackup, schemaVersion)
	}
	catalogs, files, err := buildInventory(ctx, db)
	if err != nil {
		return domainbackup.Manifest{}, fmt.Errorf("read metadata inventory: %w", err)
	}
	return domainbackup.Manifest{
		MetadataOnly: true, ContentIncluded: false,
		Database: domainbackup.DatabaseIdentity{
			SHA256: hash, SizeBytes: size, ApplicationID: applicationID, SchemaVersion: schemaVersion,
		},
		Catalogs: catalogs, Files: files,
	}, nil
}

type snapshotInspection struct {
	ExpectedApplicationID int64
	MaxSchemaVersion      int
}

func openReadOnlySQLite(path string) (*sql.DB, error) {
	return openSQLiteReadOnly(path, true)
}

func openSQLiteReadOnly(path string, immutable bool) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(absolute)
	var uri string
	if volume := filepath.VolumeName(absolute); volume != "" && !strings.HasPrefix(uriPath, "//") {
		// ncruces' Windows VFS accepts SQLite's drive-letter URI form
		// file:C:/snapshot.sqlite. net/url's hierarchical form emits
		// file://C:/snapshot.sqlite and incorrectly treats C: as the authority.
		uri = "file:" + (&url.URL{Path: uriPath}).EscapedPath()
	} else {
		uri = (&url.URL{Scheme: "file", Path: uriPath}).String()
	}
	uri += "?mode=ro"
	if immutable {
		uri += "&immutable=1"
	}
	return sqlite3driver.Open(uri)
}

func readManifest(path string) (domainbackup.Manifest, error) {
	if err := requireRegularPrivateFile(path); err != nil {
		return domainbackup.Manifest{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domainbackup.Manifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest domainbackup.Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return domainbackup.Manifest{}, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return domainbackup.Manifest{}, err
	}
	if manifest.FormatVersion != domainbackup.ManifestFormatVersion || !manifest.MetadataOnly || manifest.ContentIncluded {
		return domainbackup.Manifest{}, domainbackup.ErrInvalidBackup
	}
	return manifest, nil
}

func validateManifestNames(manifest domainbackup.Manifest, backupID, manifestName string) error {
	if !validBackupID(backupID) || manifest.BackupID != backupID ||
		manifestName != backupPrefix+backupID+manifestSuffix ||
		manifest.Database.FileName != backupPrefix+backupID+databaseSuffix ||
		!safeBaseName(manifest.Database.FileName) {
		return domainbackup.ErrInvalidBackup
	}
	if manifest.Purpose != domainbackup.PurposeUser && manifest.Purpose != domainbackup.PurposeRestoreRollback {
		return domainbackup.ErrInvalidBackup
	}
	if manifest.CreatedAt.IsZero() || manifest.Database.SHA256 == "" || manifest.Database.SizeBytes <= 0 {
		return domainbackup.ErrInvalidBackup
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("unexpected JSON after document")
		}
		return err
	}
	return nil
}

func fileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}

func writeJSONFileExclusive(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, backupFileMode)
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err := restrictBackupFile(path); err != nil {
		return fmt.Errorf("set private JSON file permissions: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	remove = false
	return nil
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, backupDirectoryMode); err != nil {
		return fmt.Errorf("create private backup directory: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("backup directory is not a regular directory")
	}
	if err := restrictBackupDirectory(path); err != nil {
		return fmt.Errorf("set backup directory permissions: %w", err)
	}
	return nil
}

func requireRegularPrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("backup artifact is not a regular file")
	}
	return nil
}

func ensureDestinationAbsent(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("backup destination already exists")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func newBackupID(now time.Time) (string, error) {
	random := make([]byte, 10)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return now.Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(random), nil
}

func randomSuffix() string {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(random)
}

func validBackupID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func safeBaseName(value string) bool {
	return value != "" && filepath.Base(value) == value && value != "." && value != ".."
}

func isManifestName(name string) bool {
	_, ok := backupIDFromManifestName(name)
	return ok
}

func backupIDFromManifestName(name string) (string, bool) {
	if !strings.HasPrefix(name, backupPrefix) || !strings.HasSuffix(name, manifestSuffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, backupPrefix), manifestSuffix)
	return id, validBackupID(id)
}

func backupIDFromDatabaseName(name string) (string, bool) {
	if !strings.HasPrefix(name, backupPrefix) || !strings.HasSuffix(name, databaseSuffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, backupPrefix), databaseSuffix)
	return id, validBackupID(id)
}
