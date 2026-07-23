package libraryrepo

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"xiadown/internal/application/library/catalogaudit"
	"xiadown/internal/infrastructure/persistence"
)

const catalogAuditMigrationID = "catalog-foundation-v1"

func TestSQLiteCatalogAuditorReportsHealthyProjection(t *testing.T) {
	ctx := context.Background()
	db := openCatalogAuditTestDB(t, ctx)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	seedHealthyCatalogAuditGraph(t, ctx, db.SQL, now)

	// query_only makes the read-only contract executable: this audit fails if a
	// future implementation accidentally attempts a repair or bookkeeping write.
	db.SQL.SetMaxOpenConns(1)
	if _, err := db.SQL.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		t.Fatalf("enable SQLite query_only: %v", err)
	}
	report, err := NewSQLiteCatalogAuditor(db.Bun).Audit(ctx, catalogaudit.Request{
		CatalogID: "catalog-1", MigrationID: catalogAuditMigrationID,
	})
	if err != nil {
		t.Fatalf("audit healthy projection: %v", err)
	}
	if !report.IsHealthy() {
		t.Fatalf("healthy graph reported findings: %#v", report)
	}
	if report.Counts.LegacyFiles != 4 || report.Counts.LegacyMappings != 4 || report.Counts.AssetLinks != 4 {
		t.Fatalf("unexpected projection counts: %#v", report.Counts)
	}
	if report.Counts.Items != 4 || report.Counts.ActiveItems != 1 || report.Counts.MissingItems != 1 ||
		report.Counts.TrashedItems != 1 || report.Counts.NeedsReviewItems != 1 {
		t.Fatalf("unexpected status counts: %#v", report.Counts)
	}
	if report.Counts.PreservedFileIDs != 4 || report.Counts.PreservedPhysicalReferences != 4 {
		t.Fatalf("legacy physical references were not preserved: %#v", report.Counts)
	}
	if report.Counts.Representations != 4 || report.Counts.MetadataEntries != 0 {
		t.Fatalf("unexpected professional entity counts: %#v", report.Counts)
	}
	if report.AuditedAt.IsZero() || len(report.Issues) != 0 || report.Findings.Total() != 0 {
		t.Fatalf("unexpected healthy report metadata: %#v", report)
	}
}

func TestSQLiteCatalogAuditorDetectsInconsistentProjectionWithoutRepair(t *testing.T) {
	ctx := context.Background()
	db := openCatalogAuditTestDB(t, ctx)
	now := time.Date(2026, 7, 13, 13, 0, 0, 0, time.UTC)
	seedHealthyCatalogAuditGraph(t, ctx, db.SQL, now)

	if _, err := db.SQL.ExecContext(ctx, `
DELETE FROM library_legacy_mappings
WHERE migration_id = ? AND source_type = 'library_file' AND source_id = 'file-4'
`, catalogAuditMigrationID); err != nil {
		t.Fatalf("remove mapping: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
UPDATE library_legacy_mappings SET target_id = 'asset-1'
WHERE migration_id = ? AND source_type = 'library_file' AND source_id = 'file-2'
`, catalogAuditMigrationID); err != nil {
		t.Fatalf("duplicate mapping target: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
UPDATE library_legacy_mappings SET target_id = 'asset-missing'
WHERE migration_id = ? AND source_type = 'library_file' AND source_id = 'file-3'
`, catalogAuditMigrationID); err != nil {
		t.Fatalf("break mapping target: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
INSERT INTO library_legacy_mappings (
  migration_id, catalog_id, source_type, source_id, target_type, target_id,
  source_fingerprint, migrated_at
) VALUES (?, 'catalog-1', 'library_file', 'file-gone', 'item_asset', 'asset-gone', '', ?)
`, catalogAuditMigrationID, now); err != nil {
		t.Fatalf("insert missing mapping endpoints: %v", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
UPDATE library_files SET storage_local_path = '/moved/behind-the-migration/file-1.bin'
WHERE id = 'file-1'
`); err != nil {
		t.Fatalf("change physical reference: %v", err)
	}

	// Simulate damage from an external/manual SQLite edit. Normal repository
	// writes cannot create this dangling foreign key.
	db.SQL.SetMaxOpenConns(1)
	connection, err := db.SQL.Conn(ctx)
	if err != nil {
		t.Fatalf("open dedicated SQLite connection: %v", err)
	}
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		connection.Close()
		t.Fatalf("disable foreign keys for corruption fixture: %v", err)
	}
	if _, err := connection.ExecContext(ctx, `
UPDATE library_item_assets SET file_id = 'file-missing' WHERE id = 'asset-4'
`); err != nil {
		connection.Close()
		t.Fatalf("create dangling asset fixture: %v", err)
	}
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		connection.Close()
		t.Fatalf("restore foreign keys: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close dedicated SQLite connection: %v", err)
	}

	auditor := NewSQLiteCatalogAuditor(db.Bun)
	report, err := auditor.Audit(ctx, catalogaudit.Request{
		CatalogID: "catalog-1", MigrationID: catalogAuditMigrationID,
	})
	if err != nil {
		t.Fatalf("audit inconsistent projection: %v", err)
	}
	if report.IsHealthy() {
		t.Fatalf("inconsistent graph reported healthy: %#v", report)
	}
	if report.Findings.UnmappedLegacyFiles != 1 || report.Findings.DuplicateMappings != 1 ||
		report.Findings.DanglingAssets != 1 || report.Findings.MissingMappingSources != 1 ||
		report.Findings.MissingMappingTargets != 2 || report.Findings.MappingAssetMismatches != 1 ||
		report.Findings.ChangedPhysicalReferences != 1 {
		t.Fatalf("unexpected findings: %#v; issues=%#v", report.Findings, report.Issues)
	}
	for _, kind := range []catalogaudit.IssueKind{
		catalogaudit.IssueUnmappedLegacyFile,
		catalogaudit.IssueDuplicateMapping,
		catalogaudit.IssueDanglingAsset,
		catalogaudit.IssueMissingMappingSource,
		catalogaudit.IssueMissingMappingTarget,
		catalogaudit.IssueMappingAssetMismatch,
		catalogaudit.IssueChangedPhysicalReference,
	} {
		if !catalogAuditHasIssue(report, kind) {
			t.Errorf("missing issue kind %q in %#v", kind, report.Issues)
		}
	}

	// Audit is observational: running it did not restore or delete any of the
	// manually damaged rows.
	var danglingFileID string
	if err := db.SQL.QueryRowContext(ctx, `
SELECT file_id FROM library_item_assets WHERE id = 'asset-4'
`).Scan(&danglingFileID); err != nil {
		t.Fatalf("read dangling asset after audit: %v", err)
	}
	if danglingFileID != "file-missing" {
		t.Fatalf("audit repaired data unexpectedly: asset-4.file_id=%q", danglingFileID)
	}
	var missingMappingCount int
	if err := db.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_legacy_mappings
WHERE migration_id = ? AND source_id = 'file-4'
`, catalogAuditMigrationID).Scan(&missingMappingCount); err != nil {
		t.Fatalf("count missing mapping after audit: %v", err)
	}
	if missingMappingCount != 0 {
		t.Fatalf("audit recreated mapping unexpectedly: count=%d", missingMappingCount)
	}
}

func TestSQLiteCatalogAuditorDetectsProfessionalEntityIntegrityFailures(t *testing.T) {
	ctx := context.Background()
	db := openCatalogAuditTestDB(t, ctx)
	now := time.Date(2026, 7, 13, 13, 0, 0, 0, time.UTC)
	seedHealthyCatalogAuditGraph(t, ctx, db.SQL, now)

	if _, err := db.SQL.ExecContext(ctx, "DELETE FROM library_representations WHERE id = 'asset-4'"); err != nil {
		t.Fatalf("remove representation: %v", err)
	}
	db.SQL.SetMaxOpenConns(1)
	connection, err := db.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		connection.Close()
		t.Fatal(err)
	}
	if _, err := connection.ExecContext(ctx, `
INSERT INTO library_representations (
  id, catalog_id, item_id, asset_id, kind, purpose, availability, revision, created_at, updated_at
) VALUES
  ('dangling-representation', 'catalog-1', 'missing-item', 'missing-asset', 'original', 'primary', 'missing', 1, ?, ?),
  ('mismatched-representation', 'catalog-1', 'item-1', 'asset-2', 'optimized', 'playback', 'available', 1, ?, ?)
`, now, now, now, now); err != nil {
		connection.Close()
		t.Fatalf("seed corrupt representations: %v", err)
	}
	if _, err := connection.ExecContext(ctx, `
INSERT INTO library_metadata_entries (
  id, catalog_id, item_id, representation_id, namespace, key, value_type, value_json,
  language, position, source, provenance, locked, revision, created_at, updated_at
) VALUES
  ('dangling-metadata', 'catalog-1', 'missing-item', 'missing-representation', 'dc', 'title', 'string', '"Missing"', '', 0, 'system', 'audit fixture', FALSE, 1, ?, ?),
  ('mismatched-metadata', 'catalog-1', 'item-1', 'asset-2', 'dc', 'title', 'string', '"Wrong item"', '', 0, 'system', 'audit fixture', FALSE, 1, ?, ?)
`, now, now, now, now); err != nil {
		connection.Close()
		t.Fatalf("seed corrupt metadata: %v", err)
	}
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		connection.Close()
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	report, err := NewSQLiteCatalogAuditor(db.Bun).Audit(ctx, catalogaudit.Request{
		CatalogID: "catalog-1", MigrationID: catalogAuditMigrationID,
	})
	if err != nil {
		t.Fatalf("audit professional integrity: %v", err)
	}
	if report.IsHealthy() || report.Findings.AssetsWithoutRepresentations != 1 ||
		report.Findings.DanglingRepresentations != 1 || report.Findings.RepresentationMismatches != 1 ||
		report.Findings.DanglingMetadataEntries != 1 || report.Findings.MetadataRepresentationMismatches != 1 {
		t.Fatalf("unexpected professional findings: %+v issues=%+v", report.Findings, report.Issues)
	}
	for _, kind := range []catalogaudit.IssueKind{
		catalogaudit.IssueAssetWithoutRepresentation,
		catalogaudit.IssueDanglingRepresentation,
		catalogaudit.IssueRepresentationMismatch,
		catalogaudit.IssueDanglingMetadataEntry,
		catalogaudit.IssueMetadataRepresentationMismatch,
	} {
		if !catalogAuditHasIssue(report, kind) {
			t.Fatalf("missing professional issue %q: %+v", kind, report.Issues)
		}
	}
}

func TestSQLiteCatalogAuditorRejectsBlankScope(t *testing.T) {
	ctx := context.Background()
	db := openCatalogAuditTestDB(t, ctx)
	if _, err := NewSQLiteCatalogAuditor(db.Bun).Audit(ctx, catalogaudit.Request{}); err != catalogaudit.ErrInvalidRequest {
		t.Fatalf("blank audit request error = %v, want %v", err, catalogaudit.ErrInvalidRequest)
	}
}

func openCatalogAuditTestDB(t *testing.T, ctx context.Context) *persistence.Database {
	t.Helper()
	db, err := openLibraryRepoTestDatabase(t, ctx, "catalog-audit.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedHealthyCatalogAuditGraph(t *testing.T, ctx context.Context, db *sql.DB, now time.Time) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_libraries (id, name, created_by_json, created_at, updated_at)
VALUES ('legacy-library', 'Legacy Library', '{}', ?, ?)
`, now, now); err != nil {
		t.Fatalf("seed legacy library: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_catalogs (id, name, status, is_default, created_at, updated_at)
VALUES ('catalog-1', 'Library', 'active', TRUE, ?, ?)
`, now, now); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	statuses := []string{"active", "missing", "trashed", "needs_review"}
	for index, status := range statuses {
		id := index + 1
		var trashedAt any
		if status == "trashed" {
			trashedAt = now
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, revision, trashed_at,
  created_at, updated_at
) VALUES (?, 'catalog-1', 'other', ?, ?, ?, 1, ?, ?, ?)
`, fmtAuditID("item", id), status, fmtAuditID("Item", id), fmtAuditID("Item", id), trashedAt, now, now); err != nil {
			t.Fatalf("seed catalog item %d: %v", id, err)
		}

		path := filepath.Join(string(filepath.Separator), "library", fmtAuditID("file", id)+".bin")
		name := fmtAuditID("file", id) + ".bin"
		displayName := fmtAuditID("File", id)
		if _, err := db.ExecContext(ctx, `
INSERT INTO library_files (
  id, library_id, kind, name, display_name,
  storage_mode, storage_local_path,
  origin_kind, origin_import_batch_id, origin_import_path, origin_imported_at,
  state_json, created_at, updated_at
) VALUES (?, 'legacy-library', 'other', ?, ?, 'local_path', ?,
          'import', 'batch-1', ?, ?, '{}', ?, ?)
`, fmtAuditID("file", id), name, displayName, path, path, now, now, now); err != nil {
			t.Fatalf("seed legacy file %d: %v", id, err)
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO library_item_assets (
  id, item_id, file_id, role, label, position, created_at, updated_at
) VALUES (?, ?, ?, 'original', 'Original', 0, ?, ?)
`, fmtAuditID("asset", id), fmtAuditID("item", id), fmtAuditID("file", id), now, now); err != nil {
			t.Fatalf("seed item asset %d: %v", id, err)
		}
		fingerprint := catalogaudit.FingerprintLegacyFileReference(catalogaudit.LegacyFileReference{
			ID: fmtAuditID("file", id), LibraryID: "legacy-library", Kind: "other",
			Name: name, DisplayName: displayName, StorageMode: "local_path", LocalPath: path,
			SourceUpdatedAt: now,
		})
		if _, err := db.ExecContext(ctx, `
INSERT INTO library_legacy_mappings (
  migration_id, catalog_id, source_type, source_id, target_type, target_id,
  source_fingerprint, migrated_at
) VALUES (?, 'catalog-1', 'library_file', ?, 'item_asset', ?, ?, ?)
`, catalogAuditMigrationID, fmtAuditID("file", id), fmtAuditID("asset", id), fingerprint, now); err != nil {
			t.Fatalf("seed legacy mapping %d: %v", id, err)
		}
	}
}

func catalogAuditHasIssue(report catalogaudit.Report, kind catalogaudit.IssueKind) bool {
	for _, issue := range report.Issues {
		if issue.Kind == kind {
			return true
		}
	}
	return false
}

func fmtAuditID(prefix string, value int) string {
	return prefix + "-" + strconv.Itoa(value)
}
