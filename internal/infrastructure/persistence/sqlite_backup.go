package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

// SQLiteApplicationID returns XiaDown's stable SQLite application identity.
// Restore tooling uses it to reject unrelated SQLite files before replacement.
func SQLiteApplicationID() int64 {
	return sqliteApplicationID
}

// CurrentSQLiteSchemaVersion is the newest schema this build can open. Older
// compatible snapshots are allowed and will be migrated by OpenSQLite.
func CurrentSQLiteSchemaVersion() int {
	if len(sqliteMigrations) == 0 {
		return 0
	}
	return sqliteMigrations[len(sqliteMigrations)-1].version
}

// CreateConsistentSQLiteSnapshot uses SQLite's online backup primitive so the
// snapshot includes committed WAL content without copying mutable sidecars.
// It deliberately avoids VACUUM INTO through database/sql: ncruces implements
// cancellable statements with an interrupt-guard VDBE, and SQLite rejects
// VACUUM while that guard is active with "SQL statements in progress". The
// caller owns fsync, permissions, and publication of targetPath.
func CreateConsistentSQLiteSnapshot(ctx context.Context, db *sql.DB, targetPath string) error {
	if db == nil || strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("sqlite snapshot database and target are required")
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return fmt.Errorf("sqlite snapshot target already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect sqlite snapshot target: %w", err)
	}
	targetURI, err := sqliteSnapshotTargetURI(targetPath)
	if err != nil {
		return fmt.Errorf("resolve sqlite snapshot target: %w", err)
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create sqlite snapshot target: %w", err)
	}
	if err := target.Close(); err != nil {
		_ = os.Remove(targetPath)
		return fmt.Errorf("close sqlite snapshot target: %w", err)
	}
	snapshotComplete := false
	defer func() {
		if !snapshotComplete {
			_ = os.Remove(targetPath)
		}
	}()

	connection, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire sqlite snapshot connection: %w", err)
	}
	defer connection.Close()

	err = connection.Raw(func(driverConnection any) error {
		sqliteConnection, ok := driverConnection.(sqlite3driver.Conn)
		if !ok {
			return fmt.Errorf("unsupported sqlite snapshot driver %T", driverConnection)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return sqliteConnection.Raw().Backup("main", targetURI)
	})
	if err != nil {
		// Online backup can create the destination before a later copy failure.
		// Never leave a partial snapshot for callers to mistake for a valid one.
		return fmt.Errorf("backup sqlite snapshot: %w", err)
	}
	snapshotComplete = true
	return nil
}

func sqliteSnapshotTargetURI(targetPath string) (string, error) {
	absolute, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	uriPath := filepath.ToSlash(absolute)
	if volume := filepath.VolumeName(absolute); volume != "" && !strings.HasPrefix(uriPath, "//") {
		return "file:" + (&url.URL{Path: uriPath}).EscapedPath() + "?mode=rw", nil
	}
	return (&url.URL{Scheme: "file", Path: uriPath}).String() + "?mode=rw", nil
}

// VerifySQLiteIntegrity runs the same structural and foreign-key checks used
// around XiaDown migrations without mutating the database.
func VerifySQLiteIntegrity(ctx context.Context, db *sql.DB, quick bool) error {
	return checkSQLiteIntegrity(ctx, db, quick)
}

// VerifySQLiteMigrationLedger checks every recorded migration name and
// checksum against this build without applying pending migrations. Restore
// validation uses it before any on-disk replacement.
func VerifySQLiteMigrationLedger(ctx context.Context, db *sql.DB) error {
	_, err := pendingSQLiteMigrations(ctx, db)
	return err
}
