package librarybackup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	domainbackup "xiadown/internal/domain/librarybackup"
)

func TestNewManagerReconcilesInterruptedPublicationAndStaleTemporaries(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	orphanName := backupPrefix + "20260712T000000.000000000Z-orphan" + databaseSuffix
	staleDatabaseTemp := "." + backupPrefix + "20260712T000000.000000000Z-stale-deadbeef.sqlite.tmp"
	staleManifestTemp := "." + backupPrefix + "20260712T000000.000000000Z-stale-deadbeef.manifest.tmp"
	staleDeleteTombstone := "." + backupPrefix + "20260712T000000.000000000Z-stale-deadbeef.delete.tmp"
	for _, name := range []string{orphanName, staleDatabaseTemp, staleManifestTemp, staleDeleteTombstone} {
		path := filepath.Join(fixture.backupDir, name)
		if err := os.WriteFile(path, []byte("private backup material"), backupFileMode); err != nil {
			t.Fatal(err)
		}
	}

	newReconciliationTestManager(t, fixture)
	for _, name := range []string{orphanName, staleDatabaseTemp, staleManifestTemp, staleDeleteTombstone} {
		if _, err := os.Lstat(filepath.Join(fixture.backupDir, name)); !os.IsNotExist(err) {
			t.Fatalf("interrupted artifact %q remains: %v", name, err)
		}
	}
}

func TestNewManagerReconcilesDatabaseLeftByManifestFirstDelete(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	manifest, err := fixture.manager.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(fixture.backupDir, backupPrefix+manifest.BackupID+manifestSuffix)
	databasePath := filepath.Join(fixture.backupDir, manifest.Database.FileName)
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	newReconciliationTestManager(t, fixture)
	if _, err := os.Lstat(databasePath); !os.IsNotExist(err) {
		t.Fatalf("manifest-first-delete database remains: %v", err)
	}
}

func TestNewManagerKeepsCommittedBackupWhileReconcilingTemporaryArtifact(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	manifest, err := fixture.manager.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	temporaryName := "." + backupPrefix + "interrupted-deadbeef.sqlite.tmp"
	if err := os.WriteFile(filepath.Join(fixture.backupDir, temporaryName), []byte("private"), backupFileMode); err != nil {
		t.Fatal(err)
	}

	newReconciliationTestManager(t, fixture)
	for _, path := range []string{
		filepath.Join(fixture.backupDir, manifest.Database.FileName),
		filepath.Join(fixture.backupDir, backupPrefix+manifest.BackupID+manifestSuffix),
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("committed backup artifact was collected: %v", err)
		}
	}
	if _, err := os.Lstat(filepath.Join(fixture.backupDir, temporaryName)); !os.IsNotExist(err) {
		t.Fatalf("temporary backup artifact remains: %v", err)
	}
}

func TestReconciliationProtectsPendingRestoreTargetAndRollback(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	target, err := fixture.manager.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := fixture.manager.PlanRestore(context.Background(), target.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	for _, backupID := range []string{target.BackupID, plan.RollbackBackupID} {
		manifestPath := filepath.Join(fixture.backupDir, backupPrefix+backupID+manifestSuffix)
		if err := os.Remove(manifestPath); err != nil {
			t.Fatal(err)
		}
	}

	newReconciliationTestManager(t, fixture)
	for _, backupID := range []string{target.BackupID, plan.RollbackBackupID} {
		databasePath := filepath.Join(fixture.backupDir, backupPrefix+backupID+databaseSuffix)
		if _, err := os.Lstat(databasePath); err != nil {
			t.Fatalf("protected restore artifact %q was collected: %v", backupID, err)
		}
	}
}

func TestReconciliationFailsSafeForUnreadableRestoreMarker(t *testing.T) {
	fixture := newBackupFixture(t, domainbackup.RetentionPolicy{})
	orphanName := backupPrefix + "20260712T000000.000000000Z-protected" + databaseSuffix
	orphanPath := filepath.Join(fixture.backupDir, orphanName)
	if err := os.WriteFile(orphanPath, []byte("private backup material"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.markerPath, []byte("not-json"), backupFileMode); err != nil {
		t.Fatal(err)
	}

	newReconciliationTestManager(t, fixture)
	if _, err := os.Lstat(orphanPath); err != nil {
		t.Fatalf("orphan was collected while restore ownership was uncertain: %v", err)
	}
}

func newReconciliationTestManager(t *testing.T, fixture *backupFixture) *Manager {
	t.Helper()
	manager, err := NewManager(Config{
		DB: fixture.database.SQL, DatabasePath: fixture.databasePath,
		BackupDirectory: fixture.backupDir, RestoreMarkerPath: fixture.markerPath,
		AppName: "XiaDown", AppVersion: "2.0.0-test",
		Clock: func() time.Time { return *fixture.now },
	})
	if err != nil {
		t.Fatalf("NewManager for reconciliation: %v", err)
	}
	return manager
}
