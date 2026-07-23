package libraryrepo

import (
	"context"
	"errors"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSQLiteDeviceGrantMutationIsAtomicAuditedAndRevisionGuarded(t *testing.T) {
	ctx := context.Background()
	db, err := openLibraryRepoTestDatabase(t, ctx, "device-grants.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	catalog, _ := library.NewCatalog(library.CatalogParams{
		ID: "catalog-1", Name: "Library", Status: "active", IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err := NewSQLiteCatalogRepository(db.Bun).Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	grant, err := library.NewDeviceGrant(library.DeviceGrantParams{
		ID: "grant-1", CatalogID: catalog.ID, DeviceID: "iphone-1", DeviceName: "iPhone",
		CredentialHash: "credential-hash", PublicKeyHash: "public-key-hash",
		Scopes: []string{"library.read", "tasks.read"}, Revision: 1,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewSQLiteDeviceGrantRepository(db.Bun)
	if err := repo.SaveDeviceGrantMutation(ctx, grant, 0, library.CatalogChangeUpsert, "pairing"); err != nil {
		t.Fatalf("create grant mutation: %v", err)
	}

	updatedAt := now.Add(time.Minute)
	updated := grant
	updated.Scopes = []library.DeviceScope{library.DeviceScopeLibraryRead, library.DeviceScopeTasksCreate}
	updated.Revision = 2
	updated.UpdatedAt = updatedAt
	if _, err := db.SQL.ExecContext(ctx, `
CREATE TRIGGER reject_device_grant_audit
BEFORE INSERT ON library_catalog_changes
WHEN NEW.entity_id = 'grant-1' AND NEW.revision = 2
BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END
`); err != nil {
		t.Fatalf("create rejecting trigger: %v", err)
	}
	if err := repo.SaveDeviceGrantMutation(ctx, updated, 1, library.CatalogChangeUpsert, "local:desktop"); err == nil {
		t.Fatal("expected audit write failure")
	}
	loaded, err := repo.Get(ctx, grant.ID)
	if err != nil || loaded.Revision != 1 || loaded.HasScope(library.DeviceScopeTasksCreate) {
		t.Fatalf("grant escaped rolled-back audit transaction: %#v err=%v", loaded, err)
	}
	if _, err := db.SQL.ExecContext(ctx, "DROP TRIGGER reject_device_grant_audit"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveDeviceGrantMutation(ctx, updated, 1, library.CatalogChangeUpsert, "local:desktop"); err != nil {
		t.Fatalf("update grant: %v", err)
	}
	if err := repo.SaveDeviceGrantMutation(ctx, updated, 1, library.CatalogChangeUpsert, "local:desktop"); !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("stale update must fail, got %v", err)
	}

	revokedAt := now.Add(2 * time.Minute)
	revoked := updated
	revoked.Status = library.DeviceGrantStatusRevoked
	revoked.RevokedAt = &revokedAt
	revoked.Revision = 3
	revoked.UpdatedAt = revokedAt
	if err := repo.SaveDeviceGrantMutation(ctx, revoked, 2, library.CatalogChangeDelete, "local:desktop"); err != nil {
		t.Fatalf("revoke grant: %v", err)
	}
	replacementAt := now.Add(3 * time.Minute)
	replacement := revoked
	replacement.DeviceName = "Replacement iPhone"
	replacement.CredentialHash = "replacement-credential-hash"
	replacement.PublicKeyHash = "replacement-public-key-hash"
	replacement.Scopes = []library.DeviceScope{
		library.DeviceScopeLibraryRead,
		library.DeviceScopeTasksRead,
	}
	replacement.Status = library.DeviceGrantStatusActive
	replacement.RevokedAt = nil
	replacement.Revision = 4
	replacement.UpdatedAt = replacementAt
	if err := repo.SaveDeviceGrantMutation(ctx, replacement, 3, library.CatalogChangeUpsert, "pairing"); err != nil {
		t.Fatalf("replace revoked grant: %v", err)
	}
	loaded, err = repo.Get(ctx, grant.ID)
	if err != nil || loaded.ID != grant.ID || loaded.DeviceID != grant.DeviceID ||
		loaded.CreatedAt != grant.CreatedAt || loaded.Status != library.DeviceGrantStatusActive ||
		loaded.RevokedAt != nil || loaded.Revision != 4 ||
		loaded.CredentialHash != replacement.CredentialHash ||
		loaded.PublicKeyHash != replacement.PublicKeyHash ||
		!loaded.HasScope(library.DeviceScopeLibraryRead) ||
		!loaded.HasScope(library.DeviceScopeTasksRead) ||
		loaded.HasScope(library.DeviceScopeTasksCreate) {
		t.Fatalf("unsafe SQLite revoked-grant replacement: %#v err=%v", loaded, err)
	}
	changes, err := NewSQLiteCatalogChangeRepository(db.Bun).ListAfter(ctx, catalog.ID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 4 || changes[1].Revision != 2 || changes[1].ActorID != "local:desktop" ||
		changes[2].Kind != library.CatalogChangeDelete || changes[2].Revision != 3 ||
		changes[3].Kind != library.CatalogChangeUpsert || changes[3].Revision != 4 ||
		changes[3].ActorID != "pairing" {
		t.Fatalf("unexpected device grant audit changes: %#v", changes)
	}
}
