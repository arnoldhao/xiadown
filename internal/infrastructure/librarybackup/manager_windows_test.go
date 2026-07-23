//go:build windows

package librarybackup

import (
	"context"
	"path/filepath"
	"testing"

	"xiadown/internal/infrastructure/persistence"
)

func TestOpenSQLiteReadOnlyAcceptsWindowsDrivePath(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "metadata snapshot.sqlite")
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, "CREATE TABLE fixture (value TEXT)"); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	readOnly, err := openSQLiteReadOnly(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	var count int
	if err := readOnly.QueryRowContext(ctx, "SELECT COUNT(*) FROM fixture").Scan(&count); err != nil {
		t.Fatal(err)
	}
}
