package librarybackup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainbackup "xiadown/internal/domain/librarybackup"
	"xiadown/internal/infrastructure/persistence"
)

type StartupRestoreConfig struct {
	DatabasePath              string
	BackupDirectory           string
	RestoreMarkerPath         string
	ExpectedApplicationID     int64
	MaxSupportedSchemaVersion int
	testFault                 func(string) error
}

const (
	restoreFaultAfterPreviousMain  = "after_previous_main"
	restoreFaultAfterCandidateMain = "after_candidate_main"
	restoreFaultAfterInstalledMark = "after_installed_marker"
)

func (manager *Manager) PlanRestore(ctx context.Context, backupID string) (domainbackup.RestorePlan, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if _, err := os.Lstat(manager.restoreMarkerPath); err == nil {
		return domainbackup.RestorePlan{}, domainbackup.ErrRestorePending
	} else if !os.IsNotExist(err) {
		return domainbackup.RestorePlan{}, fmt.Errorf("inspect restore marker: %w", err)
	}
	if _, err := manager.verifyLocked(ctx, backupID); err != nil {
		return domainbackup.RestorePlan{}, err
	}
	target, _, _, err := manager.resolveBackup(backupID)
	if err != nil {
		return domainbackup.RestorePlan{}, err
	}
	rollback, err := manager.createLocked(ctx, domainbackup.PurposeRestoreRollback)
	if err != nil {
		return domainbackup.RestorePlan{}, fmt.Errorf("create pre-restore rollback snapshot: %w", err)
	}
	requestedAt := manager.clock().UTC()
	marker := domainbackup.RestoreMarker{
		FormatVersion:        domainbackup.MarkerFormatVersion,
		Phase:                domainbackup.RestorePhasePlanned,
		BackupID:             backupID,
		BackupManifestName:   backupPrefix + backupID + manifestSuffix,
		BackupDatabaseName:   target.Database.FileName,
		BackupDatabaseSHA256: target.Database.SHA256,
		RollbackBackupID:     rollback.BackupID,
		RequestedAt:          requestedAt,
	}
	if err := writeAtomicJSON(manager.restoreMarkerPath, marker); err != nil {
		_ = manager.removeBackupLocked(rollback)
		return domainbackup.RestorePlan{}, fmt.Errorf("write next-launch restore marker: %w", err)
	}
	if err := manager.pruneLocked(); err != nil {
		return domainbackup.RestorePlan{}, fmt.Errorf("restore planned but retention cleanup failed: %w", err)
	}
	return domainbackup.RestorePlan{
		BackupID: backupID, RollbackBackupID: rollback.BackupID,
		RequestedAt: requestedAt, AppliesOnLaunch: true,
	}, nil
}

func (manager *Manager) PendingRestore(_ context.Context) (*domainbackup.RestorePlan, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	marker, err := manager.readRestoreMarkerLocked()
	if err != nil {
		if errors.Is(err, domainbackup.ErrNoRestorePending) {
			return nil, nil
		}
		return nil, err
	}
	return &domainbackup.RestorePlan{
		BackupID: marker.BackupID, RollbackBackupID: marker.RollbackBackupID,
		RequestedAt: marker.RequestedAt, AppliesOnLaunch: true,
	}, nil
}

func (manager *Manager) CancelPendingRestore(_ context.Context) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	marker, err := manager.readRestoreMarkerLocked()
	if err != nil {
		return err
	}
	if err := os.Remove(manager.restoreMarkerPath); err != nil {
		return fmt.Errorf("remove restore marker: %w", err)
	}
	if err := syncDirectory(filepath.Dir(manager.restoreMarkerPath)); err != nil {
		return err
	}
	if rollback, _, _, err := manager.resolveBackup(marker.RollbackBackupID); err == nil && rollback.Purpose == domainbackup.PurposeRestoreRollback {
		if err := manager.removeBackupLocked(rollback); err != nil {
			return fmt.Errorf("remove cancelled rollback snapshot: %w", err)
		}
	} else {
		// The marker identity is already validated and determines both names.
		// Clean an incomplete/tampered rollback pair without trusting fields
		// from its manifest, so Cancel cannot strand retention-exempt orphans.
		for _, path := range []string{
			manager.backupPath(backupPrefix + marker.RollbackBackupID + manifestSuffix),
			manager.backupPath(backupPrefix + marker.RollbackBackupID + databaseSuffix),
		} {
			if removeErr := removeBackupArtifactDurably(path); removeErr != nil {
				return fmt.Errorf("remove cancelled rollback artifact: %w", removeErr)
			}
		}
	}
	return manager.pruneLocked()
}

func (manager *Manager) readRestoreMarkerLocked() (domainbackup.RestoreMarker, error) {
	return readRestoreMarker(manager.restoreMarkerPath)
}

// ApplyPendingRestore must be called before OpenSQLite. It never operates on
// an open database. The online Wails service can only write a marker; the next
// process launch performs this crash-recoverable same-directory swap.
func ApplyPendingRestore(ctx context.Context, config StartupRestoreConfig) (domainbackup.RestoreApplyResult, error) {
	resolved, err := normalizeStartupRestoreConfig(config)
	if err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	marker, err := readRestoreMarker(resolved.RestoreMarkerPath)
	if err != nil {
		if errors.Is(err, domainbackup.ErrNoRestorePending) {
			return domainbackup.RestoreApplyResult{Applied: false}, nil
		}
		return domainbackup.RestoreApplyResult{}, err
	}
	manifestPath, databaseSource, err := resolveMarkerArtifacts(resolved, marker)
	if err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	expected := snapshotInspection{
		ExpectedApplicationID: resolved.ExpectedApplicationID,
		MaxSchemaVersion:      resolved.MaxSupportedSchemaVersion,
	}
	previousPath := resolved.DatabasePath + ".restore-previous"
	stagingPath := resolved.DatabasePath + ".restore-staging"
	workingPath := resolved.DatabasePath + ".restore-working"
	sourcePath := resolved.DatabasePath + ".restore-source"

	if marker.Phase == domainbackup.RestorePhaseRollingBack {
		if err := finishRestoreRollback(ctx, resolved, &marker, previousPath, stagingPath, workingPath, sourcePath, expected, true); err != nil {
			return domainbackup.RestoreApplyResult{}, err
		}
		return domainbackup.RestoreApplyResult{Applied: false}, nil
	}
	if marker.Phase == domainbackup.RestorePhaseInstalled {
		if err := verifyCurrentDatabaseIdentity(ctx, resolved.DatabasePath, expected); err != nil {
			rollbackErr := beginAndFinishRestoreRollback(
				ctx, resolved, &marker, previousPath, stagingPath, workingPath, sourcePath, expected, true,
			)
			if rollbackErr != nil {
				return domainbackup.RestoreApplyResult{}, errors.Join(
					fmt.Errorf("verify installed pending restore: %w", err), rollbackErr,
				)
			}
			return domainbackup.RestoreApplyResult{Applied: false}, nil
		}
		return domainbackup.RestoreApplyResult{
			Applied: true, BackupID: marker.BackupID, RollbackBackupID: marker.RollbackBackupID,
		}, nil
	}
	if err := recoverInterruptedSwap(ctx, resolved, &marker, previousPath, stagingPath, workingPath, sourcePath, expected); err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	// This is the final gate before the first destructive namespace change.
	// Interrupted swaps are rolled back above without depending on a secondary
	// artifact that might have been externally removed after the swap began.
	if err := verifyRollbackArtifact(ctx, resolved, marker, expected); err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return domainbackup.RestoreApplyResult{}, fmt.Errorf("read restore manifest: %w", err)
	}
	if err := validateManifestNames(manifest, marker.BackupID, marker.BackupManifestName); err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	if manifest.Database.FileName != marker.BackupDatabaseName || manifest.Database.SHA256 != marker.BackupDatabaseSHA256 {
		return domainbackup.RestoreApplyResult{}, fmt.Errorf("%w: restore marker does not match manifest", domainbackup.ErrInvalidBackup)
	}
	if _, err := verifyManifestSnapshot(ctx, manifest, databaseSource, expected); err != nil {
		return domainbackup.RestoreApplyResult{}, fmt.Errorf("verify restore source: %w", err)
	}
	if err := requireRegularPrivateFile(resolved.DatabasePath); err != nil {
		return domainbackup.RestoreApplyResult{}, fmt.Errorf("current database is unavailable: %w", err)
	}
	prepared, err := prepareLogicalRestore(ctx, resolved, databaseSource, workingPath, sourcePath, stagingPath)
	if err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	marker.PreviousWAL, err = regularFileExists(resolved.DatabasePath + "-wal")
	if err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	marker.PreviousSHM, err = regularFileExists(resolved.DatabasePath + "-shm")
	if err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	if err := replaceAtomicJSON(resolved.RestoreMarkerPath, marker); err != nil {
		return domainbackup.RestoreApplyResult{}, fmt.Errorf("record restore sidecar state: %w", err)
	}
	if err := moveCurrentDatabaseAside(resolved.DatabasePath, previousPath, resolved.testFault); err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	if err := durableRename(stagingPath, resolved.DatabasePath, false); err != nil {
		rollbackErr := beginAndFinishRestoreRollback(ctx, resolved, &marker, previousPath, stagingPath, workingPath, sourcePath, expected, false)
		return domainbackup.RestoreApplyResult{}, errors.Join(fmt.Errorf("install restored database: %w", err), rollbackErr)
	}
	if err := syncDirectory(filepath.Dir(resolved.DatabasePath)); err != nil {
		rollbackErr := beginAndFinishRestoreRollback(ctx, resolved, &marker, previousPath, stagingPath, workingPath, sourcePath, expected, false)
		return domainbackup.RestoreApplyResult{}, errors.Join(fmt.Errorf("sync restored database directory: %w", err), rollbackErr)
	}
	if err := runRestoreFault(resolved.testFault, restoreFaultAfterCandidateMain); err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	if _, err := verifyManifestSnapshot(ctx, prepared.identity, resolved.DatabasePath, expected); err != nil {
		rollbackErr := beginAndFinishRestoreRollback(ctx, resolved, &marker, previousPath, stagingPath, workingPath, sourcePath, expected, false)
		return domainbackup.RestoreApplyResult{}, errors.Join(fmt.Errorf("verify installed database: %w", err), rollbackErr)
	}
	marker.Phase = domainbackup.RestorePhaseInstalled
	marker.InstalledAt = time.Now().UTC()
	marker.InstalledSHA256 = prepared.identity.Database.SHA256
	if err := replaceAtomicJSON(resolved.RestoreMarkerPath, marker); err != nil {
		rollbackErr := beginAndFinishRestoreRollback(ctx, resolved, &marker, previousPath, stagingPath, workingPath, sourcePath, expected, false)
		return domainbackup.RestoreApplyResult{}, errors.Join(fmt.Errorf("commit installed restore marker: %w", err), rollbackErr)
	}
	if err := runRestoreFault(resolved.testFault, restoreFaultAfterInstalledMark); err != nil {
		return domainbackup.RestoreApplyResult{}, err
	}
	return domainbackup.RestoreApplyResult{
		Applied: true, BackupID: marker.BackupID, RollbackBackupID: marker.RollbackBackupID,
	}, nil
}

func normalizeStartupRestoreConfig(config StartupRestoreConfig) (StartupRestoreConfig, error) {
	if strings.TrimSpace(config.DatabasePath) == "" || strings.TrimSpace(config.BackupDirectory) == "" {
		return StartupRestoreConfig{}, domainbackup.ErrInvalidConfiguration
	}
	var err error
	config.DatabasePath, err = filepath.Abs(strings.TrimSpace(config.DatabasePath))
	if err != nil {
		return StartupRestoreConfig{}, domainbackup.ErrInvalidConfiguration
	}
	config.BackupDirectory, err = filepath.Abs(strings.TrimSpace(config.BackupDirectory))
	if err != nil {
		return StartupRestoreConfig{}, domainbackup.ErrInvalidConfiguration
	}
	if strings.TrimSpace(config.RestoreMarkerPath) == "" {
		config.RestoreMarkerPath = config.DatabasePath + defaultRestoreMarker
	}
	config.RestoreMarkerPath, err = filepath.Abs(config.RestoreMarkerPath)
	if err != nil || config.RestoreMarkerPath == config.DatabasePath {
		return StartupRestoreConfig{}, domainbackup.ErrInvalidConfiguration
	}
	if config.ExpectedApplicationID == 0 {
		config.ExpectedApplicationID = persistence.SQLiteApplicationID()
	}
	if config.MaxSupportedSchemaVersion == 0 {
		config.MaxSupportedSchemaVersion = persistence.CurrentSQLiteSchemaVersion()
	}
	if config.ExpectedApplicationID <= 0 || config.MaxSupportedSchemaVersion <= 0 {
		return StartupRestoreConfig{}, domainbackup.ErrInvalidConfiguration
	}
	return config, nil
}

func resolveMarkerArtifacts(config StartupRestoreConfig, marker domainbackup.RestoreMarker) (string, string, error) {
	if (marker.FormatVersion != domainbackup.MarkerFormatVersion && marker.FormatVersion != domainbackup.LegacyMarkerFormatVersion) ||
		!validBackupID(marker.BackupID) ||
		!validBackupID(marker.RollbackBackupID) || marker.RequestedAt.IsZero() ||
		marker.BackupManifestName != backupPrefix+marker.BackupID+manifestSuffix ||
		marker.BackupDatabaseName != backupPrefix+marker.BackupID+databaseSuffix ||
		!safeBaseName(marker.BackupManifestName) || !safeBaseName(marker.BackupDatabaseName) ||
		len(marker.BackupDatabaseSHA256) != 64 {
		return "", "", domainbackup.ErrInvalidBackup
	}
	if marker.Phase != domainbackup.RestorePhasePlanned &&
		marker.Phase != domainbackup.RestorePhaseInstalled &&
		marker.Phase != domainbackup.RestorePhaseRollingBack {
		return "", "", domainbackup.ErrInvalidBackup
	}
	if marker.Phase == domainbackup.RestorePhaseInstalled &&
		(marker.InstalledAt.IsZero() || len(marker.InstalledSHA256) != 64) {
		return "", "", domainbackup.ErrInvalidBackup
	}
	return filepath.Join(config.BackupDirectory, marker.BackupManifestName),
		filepath.Join(config.BackupDirectory, marker.BackupDatabaseName), nil
}

func readRestoreMarker(path string) (domainbackup.RestoreMarker, error) {
	if err := requireRegularPrivateFile(path); err != nil {
		if os.IsNotExist(err) {
			return domainbackup.RestoreMarker{}, domainbackup.ErrNoRestorePending
		}
		return domainbackup.RestoreMarker{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domainbackup.RestoreMarker{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var marker domainbackup.RestoreMarker
	if err := decoder.Decode(&marker); err != nil {
		return domainbackup.RestoreMarker{}, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return domainbackup.RestoreMarker{}, err
	}
	if marker.FormatVersion != domainbackup.MarkerFormatVersion && marker.FormatVersion != domainbackup.LegacyMarkerFormatVersion {
		return domainbackup.RestoreMarker{}, domainbackup.ErrInvalidBackup
	}
	if marker.FormatVersion == domainbackup.LegacyMarkerFormatVersion && marker.Phase == "" {
		marker.Phase = domainbackup.RestorePhasePlanned
	}
	return marker, nil
}

func writeAtomicJSON(path string, value any) error {
	if _, err := os.Lstat(path); err == nil {
		return domainbackup.ErrRestorePending
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := ensurePrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	temporary := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"-"+randomSuffix()+".tmp")
	if err := writeJSONFileExclusive(temporary, value); err != nil {
		return err
	}
	defer os.Remove(temporary)
	if err := durableRename(temporary, path, false); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func replaceAtomicJSON(path string, value any) error {
	if err := requireRegularPrivateFile(path); err != nil {
		return err
	}
	if err := ensurePrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	temporary := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"-"+randomSuffix()+".tmp")
	if err := writeJSONFileExclusive(temporary, value); err != nil {
		return err
	}
	defer os.Remove(temporary)
	if err := durableRename(temporary, path, true); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func verifyRollbackArtifact(
	ctx context.Context,
	config StartupRestoreConfig,
	marker domainbackup.RestoreMarker,
	expected snapshotInspection,
) error {
	if marker.RollbackBackupID == marker.BackupID {
		return fmt.Errorf("%w: rollback snapshot aliases restore source", domainbackup.ErrInvalidBackup)
	}
	manifestName := backupPrefix + marker.RollbackBackupID + manifestSuffix
	manifest, err := readManifest(filepath.Join(config.BackupDirectory, manifestName))
	if err != nil {
		return fmt.Errorf("read rollback manifest: %w", err)
	}
	if err := validateManifestNames(manifest, marker.RollbackBackupID, manifestName); err != nil {
		return fmt.Errorf("validate rollback manifest: %w", err)
	}
	if manifest.Purpose != domainbackup.PurposeRestoreRollback {
		return fmt.Errorf("%w: restore rollback artifact has the wrong purpose", domainbackup.ErrInvalidBackup)
	}
	databasePath := filepath.Join(config.BackupDirectory, manifest.Database.FileName)
	if _, err := verifyManifestSnapshot(ctx, manifest, databasePath, expected); err != nil {
		return fmt.Errorf("verify rollback artifact before replacement: %w", err)
	}
	return nil
}

func recoverInterruptedSwap(
	ctx context.Context,
	config StartupRestoreConfig,
	marker *domainbackup.RestoreMarker,
	previousPath string,
	stagingPath string,
	workingPath string,
	sourcePath string,
	expected snapshotInspection,
) error {
	previousMain, err := regularFileExists(previousPath)
	if err != nil {
		return err
	}
	previousWAL, err := regularFileExists(previousPath + "-wal")
	if err != nil {
		return err
	}
	previousSHM, err := regularFileExists(previousPath + "-shm")
	if err != nil {
		return err
	}
	if previousMain {
		currentMain, err := regularFileExists(config.DatabasePath)
		if err != nil {
			return err
		}
		currentWAL, err := regularFileExists(config.DatabasePath + "-wal")
		if err != nil {
			return err
		}
		currentSHM, err := regularFileExists(config.DatabasePath + "-shm")
		if err != nil {
			return err
		}
		// Marker v1 moved the main file first. If it crashed while moving
		// sidecars, an old WAL/SHM can still have the current basename while
		// the old main already has the previous basename.
		legacyMainFirst := marker.FormatVersion == domainbackup.LegacyMarkerFormatVersion && !currentMain
		marker.PreviousWAL = marker.PreviousWAL || previousWAL || (legacyMainFirst && currentWAL)
		marker.PreviousSHM = marker.PreviousSHM || previousSHM || (legacyMainFirst && currentSHM)
		if err := beginAndFinishRestoreRollback(ctx, config, marker, previousPath, stagingPath, workingPath, sourcePath, expected, false); err != nil {
			return fmt.Errorf("recover interrupted restore replacement: %w", err)
		}
		return nil
	}
	if previousWAL || previousSHM {
		// The new swap order moves sidecars before the main file. An orphan
		// previous sidecar therefore means the old main file is still current.
		if err := restoreOrphanPreviousSidecars(config.DatabasePath, previousPath, previousWAL, previousSHM); err != nil {
			return fmt.Errorf("recover interrupted pre-swap sidecars: %w", err)
		}
		marker.PreviousWAL = false
		marker.PreviousSHM = false
		if err := replaceAtomicJSON(config.RestoreMarkerPath, *marker); err != nil {
			return err
		}
	}
	for _, path := range []string{stagingPath, workingPath, sourcePath, sourcePath + ".migrated"} {
		if err := removeSQLiteArtifacts(path); err != nil {
			return err
		}
	}
	return syncDirectory(filepath.Dir(config.DatabasePath))
}

func moveCurrentDatabaseAside(databasePath, previousPath string, fault ...func(string) error) error {
	for _, path := range []string{previousPath, previousPath + "-wal", previousPath + "-shm"} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("restore swap artifact already exists: %s", filepath.Base(path))
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	movedSuffixes := make([]string, 0, 2)
	for _, suffix := range []string{"-wal", "-shm"} {
		exists, err := regularFileExists(databasePath + suffix)
		if err != nil {
			_ = restoreOrphanPreviousSidecars(databasePath, previousPath, suffixIn(movedSuffixes, "-wal"), suffixIn(movedSuffixes, "-shm"))
			return err
		}
		if !exists {
			continue
		}
		if err := durableRename(databasePath+suffix, previousPath+suffix, false); err != nil {
			_ = restoreOrphanPreviousSidecars(databasePath, previousPath, suffixIn(movedSuffixes, "-wal"), suffixIn(movedSuffixes, "-shm"))
			return fmt.Errorf("preserve SQLite sidecar for rollback: %w", err)
		}
		movedSuffixes = append(movedSuffixes, suffix)
	}
	// Main-file movement is the commit point. A crash before this line leaves
	// only recognizable orphan previous sidecars; a crash after it leaves a
	// complete previous database that recovery can roll back.
	if err := durableRename(databasePath, previousPath, false); err != nil {
		_ = restoreOrphanPreviousSidecars(databasePath, previousPath, suffixIn(movedSuffixes, "-wal"), suffixIn(movedSuffixes, "-shm"))
		return fmt.Errorf("preserve current database for rollback: %w", err)
	}
	if len(fault) > 0 {
		if err := runRestoreFault(fault[0], restoreFaultAfterPreviousMain); err != nil {
			return err
		}
	}
	return syncDirectory(filepath.Dir(databasePath))
}

func runRestoreFault(inject func(string) error, point string) error {
	if inject == nil {
		return nil
	}
	if err := inject(point); err != nil {
		return fmt.Errorf("simulated restore interruption at %s: %w", point, err)
	}
	return nil
}

func suffixIn(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func restoreOrphanPreviousSidecars(databasePath, previousPath string, restoreWAL, restoreSHM bool) error {
	for _, sidecar := range []struct {
		suffix  string
		restore bool
	}{{"-wal", restoreWAL}, {"-shm", restoreSHM}} {
		if !sidecar.restore {
			continue
		}
		exists, err := regularFileExists(previousPath + sidecar.suffix)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if err := durableRename(previousPath+sidecar.suffix, databasePath+sidecar.suffix, true); err != nil {
			return err
		}
	}
	return syncDirectory(filepath.Dir(databasePath))
}

func beginAndFinishRestoreRollback(
	ctx context.Context,
	config StartupRestoreConfig,
	marker *domainbackup.RestoreMarker,
	previousPath string,
	stagingPath string,
	workingPath string,
	sourcePath string,
	expected snapshotInspection,
	clearMarker bool,
) error {
	marker.Phase = domainbackup.RestorePhaseRollingBack
	if err := replaceAtomicJSON(config.RestoreMarkerPath, *marker); err != nil {
		return fmt.Errorf("record restore rollback phase: %w", err)
	}
	return finishRestoreRollback(ctx, config, marker, previousPath, stagingPath, workingPath, sourcePath, expected, clearMarker)
}

func finishRestoreRollback(
	ctx context.Context,
	config StartupRestoreConfig,
	marker *domainbackup.RestoreMarker,
	previousPath string,
	stagingPath string,
	workingPath string,
	sourcePath string,
	expected snapshotInspection,
	clearMarker bool,
) error {
	if err := restorePreviousDatabase(config.DatabasePath, previousPath, marker.PreviousWAL, marker.PreviousSHM); err != nil {
		return fmt.Errorf("roll back database replacement: %w", err)
	}
	if err := verifyCurrentDatabaseIdentity(ctx, config.DatabasePath, expected); err != nil {
		return fmt.Errorf("verify rolled-back database: %w", err)
	}
	for _, path := range []string{stagingPath, workingPath, sourcePath, sourcePath + ".migrated"} {
		if err := removeSQLiteArtifacts(path); err != nil {
			return err
		}
	}
	if clearMarker {
		return removeRestoreMarker(config.RestoreMarkerPath)
	}
	marker.Phase = domainbackup.RestorePhasePlanned
	marker.InstalledAt = time.Time{}
	marker.InstalledSHA256 = ""
	marker.PreviousWAL = false
	marker.PreviousSHM = false
	return replaceAtomicJSON(config.RestoreMarkerPath, *marker)
}

func restorePreviousDatabase(databasePath, previousPath string, expectedWAL, expectedSHM bool) error {
	previousWAL, err := regularFileExists(previousPath + "-wal")
	if err != nil {
		return err
	}
	previousSHM, err := regularFileExists(previousPath + "-shm")
	if err != nil {
		return err
	}
	expectedWAL = expectedWAL || previousWAL
	expectedSHM = expectedSHM || previousSHM
	for _, sidecar := range []struct {
		suffix   string
		expected bool
	}{{"-wal", expectedWAL}, {"-shm", expectedSHM}} {
		sourceExists, err := regularFileExists(previousPath + sidecar.suffix)
		if err != nil {
			return err
		}
		if sourceExists {
			if err := durableRename(previousPath+sidecar.suffix, databasePath+sidecar.suffix, true); err != nil {
				return fmt.Errorf("restore previous SQLite sidecar: %w", err)
			}
			continue
		}
		if sidecar.expected {
			currentExists, err := regularFileExists(databasePath + sidecar.suffix)
			if err != nil {
				return err
			}
			if !currentExists {
				return fmt.Errorf("expected previous SQLite sidecar %s is missing", sidecar.suffix)
			}
			continue
		}
		if err := os.Remove(databasePath + sidecar.suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	previousMain, err := regularFileExists(previousPath)
	if err != nil {
		return err
	}
	if previousMain {
		if err := durableRename(previousPath, databasePath, true); err != nil {
			return err
		}
	} else if currentMain, err := regularFileExists(databasePath); err != nil {
		return err
	} else if !currentMain {
		return errors.New("previous and current SQLite main files are both missing")
	}
	return syncDirectory(filepath.Dir(databasePath))
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("restore artifact is not a regular file: %s", filepath.Base(path))
	}
	return true, nil
}

func verifyCurrentDatabaseIdentity(ctx context.Context, databasePath string, expected snapshotInspection) error {
	db, err := openSQLiteReadOnly(databasePath, false)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := persistence.VerifySQLiteIntegrity(ctx, db, false); err != nil {
		return err
	}
	var applicationID int64
	var schemaVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schemaVersion); err != nil {
		return err
	}
	if applicationID != expected.ExpectedApplicationID || schemaVersion <= 0 || schemaVersion > expected.MaxSchemaVersion {
		return domainbackup.ErrIncompatibleBackup
	}
	return nil
}

// FinalizePendingRestore removes the physical rollback copy only after the
// installed database has successfully passed OpenSQLite and all migrations.
func FinalizePendingRestore(ctx context.Context, config StartupRestoreConfig) error {
	resolved, err := normalizeStartupRestoreConfig(config)
	if err != nil {
		return err
	}
	marker, err := readRestoreMarker(resolved.RestoreMarkerPath)
	if errors.Is(err, domainbackup.ErrNoRestorePending) {
		return nil
	}
	if err != nil {
		return err
	}
	if marker.Phase != domainbackup.RestorePhaseInstalled {
		return fmt.Errorf("restore cannot be finalized in phase %q", marker.Phase)
	}
	expected := snapshotInspection{resolved.ExpectedApplicationID, resolved.MaxSupportedSchemaVersion}
	if err := verifyCurrentDatabaseIdentity(ctx, resolved.DatabasePath, expected); err != nil {
		return fmt.Errorf("verify opened restore before finalization: %w", err)
	}
	for _, path := range []string{
		resolved.DatabasePath + ".restore-previous",
		resolved.DatabasePath + ".restore-staging",
		resolved.DatabasePath + ".restore-working",
		resolved.DatabasePath + ".restore-source",
		resolved.DatabasePath + ".restore-source.migrated",
	} {
		if err := removeSQLiteArtifacts(path); err != nil {
			return err
		}
	}
	if err := syncDirectory(filepath.Dir(resolved.DatabasePath)); err != nil {
		return err
	}
	return removeRestoreMarker(resolved.RestoreMarkerPath)
}

// RollbackPendingRestore is called when OpenSQLite or a migration fails after
// installation. It restores the exact pre-swap files, verifies them, and
// clears the pending request so a bad backup cannot create a launch loop.
func RollbackPendingRestore(ctx context.Context, config StartupRestoreConfig) error {
	resolved, err := normalizeStartupRestoreConfig(config)
	if err != nil {
		return err
	}
	marker, err := readRestoreMarker(resolved.RestoreMarkerPath)
	if err != nil {
		return err
	}
	expected := snapshotInspection{resolved.ExpectedApplicationID, resolved.MaxSupportedSchemaVersion}
	return beginAndFinishRestoreRollback(
		ctx, resolved, &marker,
		resolved.DatabasePath+".restore-previous",
		resolved.DatabasePath+".restore-staging",
		resolved.DatabasePath+".restore-working",
		resolved.DatabasePath+".restore-source",
		expected, true,
	)
}

func removeRestoreMarker(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear completed restore marker: %w", err)
	}
	return syncDirectory(filepath.Dir(path))
}

func copyFileSynced(source, target string) error {
	if err := requireRegularPrivateFile(source); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, backupFileMode)
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		_ = output.Close()
		if remove {
			_ = os.Remove(target)
		}
	}()
	if err := restrictBackupFile(target); err != nil {
		return fmt.Errorf("set private restore artifact permissions: %w", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	remove = false
	return nil
}
