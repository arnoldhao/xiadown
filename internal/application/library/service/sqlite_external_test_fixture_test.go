package service_test

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

var catalogServiceTestDatabaseTemplate struct {
	sync.Once
	contents []byte
	err      error
}

func openCatalogServiceTestDatabase(t *testing.T, name string) *persistence.Database {
	t.Helper()
	contents, err := latestCatalogServiceTestDatabase()
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

func latestCatalogServiceTestDatabase() ([]byte, error) {
	catalogServiceTestDatabaseTemplate.Do(func() {
		directory, err := os.MkdirTemp("", "xiadown-catalog-service-test-template-")
		if err != nil {
			catalogServiceTestDatabaseTemplate.err = fmt.Errorf("create template directory: %w", err)
			return
		}
		defer func() { _ = os.RemoveAll(directory) }()

		ctx := context.Background()
		path := filepath.Join(directory, "latest.db")
		database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
		if err != nil {
			catalogServiceTestDatabaseTemplate.err = fmt.Errorf("open template database: %w", err)
			return
		}
		if err := database.Close(); err != nil {
			catalogServiceTestDatabaseTemplate.err = fmt.Errorf("close template database: %w", err)
			return
		}
		checkpointDatabase, err := sqlite3driver.Open(path)
		if err != nil {
			catalogServiceTestDatabaseTemplate.err = fmt.Errorf("open template database for checkpoint: %w", err)
			return
		}
		checkpointDatabase.SetMaxOpenConns(1)
		var busy, logFrames, checkpointedFrames int
		if err := checkpointDatabase.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(
			&busy, &logFrames, &checkpointedFrames,
		); err != nil {
			_ = checkpointDatabase.Close()
			catalogServiceTestDatabaseTemplate.err = fmt.Errorf("checkpoint template database: %w", err)
			return
		}
		if busy != 0 {
			_ = checkpointDatabase.Close()
			catalogServiceTestDatabaseTemplate.err = fmt.Errorf(
				"checkpoint template database remained busy (log=%d checkpointed=%d)",
				logFrames, checkpointedFrames,
			)
			return
		}
		if err := checkpointDatabase.Close(); err != nil {
			catalogServiceTestDatabaseTemplate.err = fmt.Errorf("close checkpoint connection: %w", err)
			return
		}
		catalogServiceTestDatabaseTemplate.contents, catalogServiceTestDatabaseTemplate.err = os.ReadFile(path)
	})
	if catalogServiceTestDatabaseTemplate.err != nil {
		return nil, catalogServiceTestDatabaseTemplate.err
	}
	return catalogServiceTestDatabaseTemplate.contents, nil
}
