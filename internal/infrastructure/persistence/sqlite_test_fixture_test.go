package persistence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

var latestSQLiteTestTemplate struct {
	once     sync.Once
	contents []byte
	err      error
}

// openLatestSQLiteTestDatabase opens an isolated copy of the latest blank
// schema. Use it only when a test needs the current schema as fixture setup;
// migration, snapshot, integrity, WAL, permission, and reopen tests must keep
// constructing their database through the path they exercise.
func openLatestSQLiteTestDatabase(t *testing.T) *Database {
	t.Helper()
	latestSQLiteTestTemplate.once.Do(func() {
		templateDir, err := os.MkdirTemp("", "xiadown-persistence-latest-")
		if err != nil {
			latestSQLiteTestTemplate.err = fmt.Errorf("create latest SQLite test template directory: %w", err)
			return
		}
		defer os.RemoveAll(templateDir)

		path := filepath.Join(templateDir, "latest.db")
		database, err := OpenSQLite(context.Background(), SQLiteConfig{
			Path:                     path,
			SkipPreMigrationSnapshot: true,
		})
		if err != nil {
			latestSQLiteTestTemplate.err = fmt.Errorf("create latest SQLite test template: %w", err)
			return
		}
		if err := database.Close(); err != nil {
			latestSQLiteTestTemplate.err = fmt.Errorf("close latest SQLite test template: %w", err)
			return
		}
		checkpointDatabase, err := sqlite3driver.Open(path)
		if err != nil {
			latestSQLiteTestTemplate.err = fmt.Errorf("open latest SQLite test template for checkpoint: %w", err)
			return
		}
		checkpointDatabase.SetMaxOpenConns(1)
		var busy, logFrames, checkpointedFrames int
		if err := checkpointDatabase.QueryRowContext(
			context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)",
		).Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
			_ = checkpointDatabase.Close()
			latestSQLiteTestTemplate.err = fmt.Errorf("checkpoint latest SQLite test template: %w", err)
			return
		}
		if busy != 0 {
			_ = checkpointDatabase.Close()
			latestSQLiteTestTemplate.err = fmt.Errorf(
				"checkpoint latest SQLite test template remained busy (log=%d checkpointed=%d)",
				logFrames, checkpointedFrames,
			)
			return
		}
		if err := checkpointDatabase.Close(); err != nil {
			latestSQLiteTestTemplate.err = fmt.Errorf("close latest SQLite test template checkpoint connection: %w", err)
			return
		}
		latestSQLiteTestTemplate.contents, latestSQLiteTestTemplate.err = os.ReadFile(path)
		if latestSQLiteTestTemplate.err != nil {
			latestSQLiteTestTemplate.err = fmt.Errorf("read latest SQLite test template: %w", latestSQLiteTestTemplate.err)
		}
	})
	if latestSQLiteTestTemplate.err != nil {
		t.Fatal(latestSQLiteTestTemplate.err)
	}

	path := filepath.Join(t.TempDir(), "latest.db")
	if err := os.WriteFile(path, latestSQLiteTestTemplate.contents, 0o600); err != nil {
		t.Fatalf("copy latest SQLite test template: %v", err)
	}
	database, err := OpenSQLite(context.Background(), SQLiteConfig{
		Path:                     path,
		SkipPreMigrationSnapshot: true,
	})
	if err != nil {
		t.Fatalf("open latest SQLite test template copy: %v", err)
	}
	return database
}
