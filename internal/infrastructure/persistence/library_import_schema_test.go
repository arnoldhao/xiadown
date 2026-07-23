package persistence

import (
	"context"
	"testing"
)

func TestLibraryImportSchemaIsAdditiveAndEnforcesCopyRoot(t *testing.T) {
	ctx := context.Background()
	database := openLatestSQLiteTestDatabase(t)
	defer database.Close()
	if err := ApplyLibraryImportSchema(ctx, database.SQL); err != nil {
		t.Fatal(err)
	}
	_, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_import_batches (
  id, request_key, library_id, mode, managed_root, hidden_policy, symlink_policy,
  status, created_at, updated_at
) VALUES ('batch', 'request', 'library', 'copy', '', 'exclude', 'skip', 'scanning', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
`)
	if err == nil {
		t.Fatal("expected copy batch without managed root to be rejected")
	}
	var legacyCount int
	if err := database.SQL.QueryRowContext(ctx, `SELECT count(*) FROM library_files`).Scan(&legacyCount); err != nil {
		t.Fatalf("legacy file table missing after additive schema: %v", err)
	}
}
