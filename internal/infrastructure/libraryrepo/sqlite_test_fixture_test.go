package libraryrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"xiadown/internal/infrastructure/persistence"
)

var libraryRepoTestDatabaseTemplate struct {
	sync.Once
	contents []byte
	err      error
}

// openLibraryRepoTestDatabase gives every test its own latest-schema database
// while avoiding replaying the complete migration history for every fixture.
func openLibraryRepoTestDatabase(
	t *testing.T,
	ctx context.Context,
	name string,
) (*persistence.Database, error) {
	t.Helper()
	contents, err := latestLibraryRepoTestDatabase()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return nil, fmt.Errorf("copy sqlite test template: %w", err)
	}
	return persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: path, SkipPreMigrationSnapshot: true,
	})
}

func latestLibraryRepoTestDatabase() ([]byte, error) {
	libraryRepoTestDatabaseTemplate.Do(func() {
		directory, err := os.MkdirTemp("", "xiadown-libraryrepo-test-template-")
		if err != nil {
			libraryRepoTestDatabaseTemplate.err = fmt.Errorf("create template directory: %w", err)
			return
		}
		defer func() { _ = os.RemoveAll(directory) }()

		ctx := context.Background()
		path := filepath.Join(directory, "latest.db")
		database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
		if err != nil {
			libraryRepoTestDatabaseTemplate.err = fmt.Errorf("open template database: %w", err)
			return
		}
		if err := closeLibraryRepoTestDatabase(database, path); err != nil {
			libraryRepoTestDatabaseTemplate.err = err
			return
		}
		libraryRepoTestDatabaseTemplate.contents, libraryRepoTestDatabaseTemplate.err = os.ReadFile(path)
	})
	if libraryRepoTestDatabaseTemplate.err != nil {
		return nil, libraryRepoTestDatabaseTemplate.err
	}
	return libraryRepoTestDatabaseTemplate.contents, nil
}

func closeLibraryRepoTestDatabase(database *persistence.Database, path string) error {
	if err := database.Close(); err != nil {
		return fmt.Errorf("close template database: %w", err)
	}
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() != 0 {
		return fmt.Errorf("close template database: WAL still contains %d bytes", info.Size())
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect closed template WAL: %w", err)
	}
	return nil
}
