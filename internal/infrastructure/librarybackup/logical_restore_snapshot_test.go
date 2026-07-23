package librarybackup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

func TestMigrateRestoreSourceNeverCreatesPreMigrationSnapshot(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "data.db.restore-source")
	compactPath := sourcePath + ".migrated"
	raw, err := sqlite3driver.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `
CREATE TABLE disposable_restore_marker (value TEXT NOT NULL);
INSERT INTO disposable_restore_marker (value) VALUES ('old-schema');
`); err != nil {
		_ = raw.Close()
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	if err := migrateRestoreSource(ctx, sourcePath, compactPath); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(sourcePath + ".pre-migration-v*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("disposable restore migration created snapshots: %v", matches)
	}
	database, err := openReadOnlySQLite(compactPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var marker string
	if err := database.QueryRowContext(ctx, "SELECT value FROM disposable_restore_marker").Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if marker != "old-schema" {
		t.Fatalf("restored migration marker = %q", marker)
	}
}

func TestCleanupRestoreMigrationSnapshotsContinuesAfterInvalidArtifact(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "data.db.restore-source")
	regularPath := sourcePath + ".pre-migration-v7-clean.bak"
	invalidPath := sourcePath + ".pre-migration-v8-invalid.bak"
	if err := os.WriteFile(regularPath, []byte("sensitive metadata"), backupFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(invalidPath, backupDirectoryMode); err != nil {
		t.Fatal(err)
	}

	if err := cleanupRestoreMigrationSnapshots(sourcePath); err == nil {
		t.Fatal("invalid staged artifact did not fail closed")
	}
	if _, err := os.Lstat(regularPath); !os.IsNotExist(err) {
		t.Fatalf("regular staged snapshot remains after partial cleanup: %v", err)
	}
	if info, err := os.Lstat(invalidPath); err != nil || !info.IsDir() {
		t.Fatalf("invalid artifact was followed or mutated: info=%v err=%v", info, err)
	}
}
