package persistence_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"xiadown/internal/application/library/catalogaudit"
	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/infrastructure/libraryrepo"
	"xiadown/internal/infrastructure/persistence"
)

func TestCatalogUpgradeV047Fixture(t *testing.T) {
	fixtureSQL, err := os.ReadFile(filepath.Join("testdata", "v0.4.7-library.sql"))
	if err != nil {
		t.Fatalf("read committed v0.4.7 fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "v0.4.7.db")
	legacyDB, err := sqlite3driver.Open(path)
	if err != nil {
		t.Fatalf("open committed v0.4.7 fixture: %v", err)
	}
	legacyDB.SetMaxOpenConns(1)
	for _, statement := range strings.Split(string(fixtureSQL), ";") {
		statement = strings.TrimSpace(statement)
		for strings.HasPrefix(statement, "--") {
			newline := strings.IndexByte(statement, '\n')
			if newline < 0 {
				statement = ""
				break
			}
			statement = strings.TrimSpace(statement[newline+1:])
		}
		if statement == "" {
			continue
		}
		if _, err := legacyDB.ExecContext(context.Background(), statement); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("seed committed v0.4.7 fixture statement: %v\n%s", err, statement)
		}
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close committed v0.4.7 fixture: %v", err)
	}
	runCatalogUpgradeFixture(t, path, true)
}

// TestCatalogUpgradeExternalFixture remains available for release rehearsals
// against a disposable copy of a real user database. The committed v0.4.7
// fixture above ensures the migration path is still exercised in every CI run.
func TestCatalogUpgradeExternalFixture(t *testing.T) {
	path := os.Getenv("XIADOWN_UPGRADE_FIXTURE")
	if path == "" {
		t.Skip("set XIADOWN_UPGRADE_FIXTURE to a disposable database copy")
	}
	runCatalogUpgradeFixture(t, path, false)
}

func runCatalogUpgradeFixture(t *testing.T, path string, verifyLegacySession bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: path})
	if err != nil {
		t.Fatalf("upgrade copied database: %v", err)
	}
	defer db.Close()
	if db.MigrationSnapshotPath == "" {
		t.Fatal("legacy fixture upgrade did not create a pre-migration snapshot")
	}

	backfill := libraryservice.NewLegacyCatalogBackfillService(
		libraryrepo.NewSQLiteLibraryRepository(db.Bun),
		libraryrepo.NewSQLiteFileRepository(db.Bun),
		libraryrepo.NewSQLiteCatalogBackfillWriter(db.Bun),
	)
	result, err := backfill.Run(ctx)
	if err != nil {
		t.Fatalf("backfill copied database: %v", err)
	}
	if result.CatalogID == "" {
		t.Fatal("fixture contains no legacy files")
	}
	report, err := libraryrepo.NewSQLiteCatalogAuditor(db.Bun).Audit(ctx, catalogaudit.Request{
		CatalogID: result.CatalogID, MigrationID: libraryservice.LegacyCatalogProjectionID,
	})
	if err != nil {
		t.Fatalf("audit copied database: %v", err)
	}
	if !report.IsHealthy() {
		t.Fatalf("unhealthy copied database migration: counts=%+v findings=%+v issues=%+v", report.Counts, report.Findings, report.Issues)
	}
	if report.Counts.LegacyFiles != report.Counts.LegacyMappings ||
		report.Counts.LegacyFiles != report.Counts.AssetLinks {
		t.Fatalf("copy migration is not one-to-one: %+v", report.Counts)
	}
	if verifyLegacySession {
		var displayName, verification string
		if err := db.SQL.QueryRowContext(ctx, `
SELECT account_display_name, account_verification_status
FROM site_app_sessions
WHERE id = 'legacy-youtube-session'
`).Scan(&displayName, &verification); err != nil {
			t.Fatalf("read preserved v0.4.7 App Session: %v", err)
		}
		if displayName != "Legacy User" || verification != "verified" {
			t.Fatalf("legacy App Session changed: displayName=%q verification=%q", displayName, verification)
		}
		if report.Counts.LegacyFiles != 2 {
			t.Fatalf("committed v0.4.7 fixture legacy files = %d, want 2", report.Counts.LegacyFiles)
		}
	}
}
