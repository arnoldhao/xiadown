package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"

	"xiadown/internal/infrastructure/persistence"
)

var libraryServiceTestDatabaseTemplate struct {
	sync.Once
	contents []byte
	err      error
}

func openLibraryServiceTestDatabase(t *testing.T, name string) *persistence.Database {
	t.Helper()
	contents, err := latestLibraryServiceTestDatabase()
	if err != nil {
		t.Fatalf("prepare sqlite test template: %v", err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("copy sqlite test template: %v", err)
	}
	database, err := persistence.OpenSQLite(context.Background(), persistence.SQLiteConfig{
		Path: path, SkipPreMigrationSnapshot: true,
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func latestLibraryServiceTestDatabase() ([]byte, error) {
	libraryServiceTestDatabaseTemplate.Do(func() {
		directory, err := os.MkdirTemp("", "xiadown-library-service-test-template-")
		if err != nil {
			libraryServiceTestDatabaseTemplate.err = fmt.Errorf("create template directory: %w", err)
			return
		}
		defer func() { _ = os.RemoveAll(directory) }()

		ctx := context.Background()
		path := filepath.Join(directory, "latest.db")
		database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
		if err != nil {
			libraryServiceTestDatabaseTemplate.err = fmt.Errorf("open template database: %w", err)
			return
		}
		if err := database.Close(); err != nil {
			libraryServiceTestDatabaseTemplate.err = fmt.Errorf("close template database: %w", err)
			return
		}
		if err := checkpointLibraryServiceTestDatabase(ctx, path); err != nil {
			libraryServiceTestDatabaseTemplate.err = err
			return
		}
		libraryServiceTestDatabaseTemplate.contents, libraryServiceTestDatabaseTemplate.err = os.ReadFile(path)
	})
	if libraryServiceTestDatabaseTemplate.err != nil {
		return nil, libraryServiceTestDatabaseTemplate.err
	}
	return libraryServiceTestDatabaseTemplate.contents, nil
}

func checkpointLibraryServiceTestDatabase(ctx context.Context, path string) error {
	database, err := sqlite3driver.Open(path)
	if err != nil {
		return fmt.Errorf("open template database for checkpoint: %w", err)
	}
	database.SetMaxOpenConns(1)
	var busy, logFrames, checkpointedFrames int
	if err := database.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(
		&busy, &logFrames, &checkpointedFrames,
	); err != nil {
		_ = database.Close()
		return fmt.Errorf("checkpoint template database: %w", err)
	}
	if busy != 0 {
		_ = database.Close()
		return fmt.Errorf(
			"checkpoint template database remained busy (log=%d checkpointed=%d)",
			logFrames, checkpointedFrames,
		)
	}
	if err := database.Close(); err != nil {
		return fmt.Errorf("close checkpoint connection: %w", err)
	}
	return nil
}
