package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteUserStateAndDeviceGrantRepositoriesRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openCatalogSyncTestDB(t, ctx)
	now := time.Date(2026, 7, 13, 5, 0, 0, 0, time.UTC)
	seedCatalogSyncTestGraph(t, ctx, db.SQL, now)

	lastOpened := now.Add(time.Minute)
	state, err := library.NewUserState(library.UserStateParams{
		ID: "state-1", CatalogID: "catalog-1", ItemID: "item-1", UserID: "user-1",
		Favorite: true, Rating: 4, Progress: 0.5, PositionMs: 3_000, Locator: "chapter-2",
		Revision: 2, LastOpenedAt: &lastOpened, CreatedAt: &now, UpdatedAt: &lastOpened,
	})
	if err != nil {
		t.Fatalf("new user state: %v", err)
	}
	stateRepo := NewSQLiteUserStateRepository(db.Bun)
	if err := stateRepo.Save(ctx, state); err != nil {
		t.Fatalf("save user state: %v", err)
	}
	loadedState, err := stateRepo.Get(ctx, state.CatalogID, state.ItemID, state.UserID)
	if err != nil {
		t.Fatalf("get user state: %v", err)
	}
	if loadedState.ID != state.ID || loadedState.Revision != 2 || !loadedState.Favorite || loadedState.Locator != "chapter-2" {
		t.Fatalf("unexpected user state: %#v", loadedState)
	}
	state.Progress = 1
	state.Completed = true
	state.Revision = 3
	state.UpdatedAt = lastOpened.Add(time.Minute)
	if err := stateRepo.Save(ctx, state); err != nil {
		t.Fatalf("update user state: %v", err)
	}
	loadedState, err = stateRepo.Get(ctx, state.CatalogID, state.ItemID, state.UserID)
	if err != nil || loadedState.Revision != 3 || !loadedState.Completed {
		t.Fatalf("unexpected updated user state: %#v, err=%v", loadedState, err)
	}
	if err := stateRepo.Delete(ctx, state.ID); err != nil {
		t.Fatalf("delete user state: %v", err)
	}
	if _, err := stateRepo.Get(ctx, state.CatalogID, state.ItemID, state.UserID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted user state error = %v, want sql.ErrNoRows", err)
	}

	expires := now.Add(24 * time.Hour)
	grant, err := library.NewDeviceGrant(library.DeviceGrantParams{
		ID: "grant-1", CatalogID: "catalog-1", DeviceID: "phone-1", DeviceName: "Arnold iPhone",
		CredentialHash: "credential-hash-1", PublicKeyHash: "public-key-hash-1",
		Scopes:    []string{"tasks.read", "library.read", "library.read"},
		ExpiresAt: &expires, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new device grant: %v", err)
	}
	grantRepo := NewSQLiteDeviceGrantRepository(db.Bun)
	if err := grantRepo.Save(ctx, grant); err != nil {
		t.Fatalf("save device grant: %v", err)
	}
	loadedGrant, err := grantRepo.Get(ctx, grant.ID)
	if err != nil {
		t.Fatalf("get device grant: %v", err)
	}
	if loadedGrant.CredentialHash != grant.CredentialHash || len(loadedGrant.Scopes) != 2 ||
		!loadedGrant.HasScope(library.DeviceScopeLibraryRead) || !loadedGrant.HasScope(library.DeviceScopeTasksRead) {
		t.Fatalf("unexpected device grant: %#v", loadedGrant)
	}
	revokedAt := now.Add(time.Hour)
	grant.Status = library.DeviceGrantStatusRevoked
	grant.RevokedAt = &revokedAt
	grant.UpdatedAt = revokedAt
	if err := grantRepo.Save(ctx, grant); err != nil {
		t.Fatalf("revoke device grant: %v", err)
	}
	grants, err := grantRepo.ListByCatalogID(ctx, grant.CatalogID)
	if err != nil || len(grants) != 1 || grants[0].Status != library.DeviceGrantStatusRevoked {
		t.Fatalf("unexpected device grants: %#v, err=%v", grants, err)
	}
}

func TestSQLiteCatalogChangeRepositoryAllocatesSequenceAndAtomicallySavesDeletes(t *testing.T) {
	ctx := context.Background()
	db := openCatalogSyncTestDB(t, ctx)
	now := time.Date(2026, 7, 13, 6, 0, 0, 0, time.UTC)
	seedCatalogSyncTestGraph(t, ctx, db.SQL, now)
	repo := NewSQLiteCatalogChangeRepository(db.Bun)

	zeroSequence := library.CatalogChange{
		CatalogID: "catalog-1", EntityType: library.CatalogEntityItem, EntityID: "item-1",
		Kind: library.CatalogChangeUpsert, Revision: 1, ActorID: "user-1", OccurredAt: now,
	}
	if err := repo.Save(ctx, zeroSequence); err != nil {
		t.Fatalf("append zero-sequence change: %v", err)
	}
	positiveSequence := zeroSequence
	positiveSequence.Sequence = 900
	positiveSequence.Revision = 2
	positiveSequence.OccurredAt = now.Add(time.Minute)
	if err := repo.Save(ctx, positiveSequence); err != nil {
		t.Fatalf("append positive-sequence change: %v", err)
	}
	changes, err := repo.ListAfter(ctx, "catalog-1", 0, 100)
	if err != nil {
		t.Fatalf("list changes: %v", err)
	}
	if len(changes) != 2 || changes[0].Sequence != 1 || changes[1].Sequence != 2 {
		t.Fatalf("SQLite did not allocate monotonic sequences: %#v", changes)
	}
	if changes[1].Sequence == positiveSequence.Sequence {
		t.Fatalf("caller-controlled sequence was persisted: %#v", changes[1])
	}

	deleteChange := library.CatalogChange{
		Sequence: 42, CatalogID: "catalog-1", EntityType: library.CatalogEntityItem,
		EntityID: "item-deleted", Kind: library.CatalogChangeDelete, Revision: 3,
		ActorID: "user-1", OccurredAt: now.Add(2 * time.Minute),
	}
	if err := repo.Save(ctx, deleteChange); err != nil {
		t.Fatalf("append delete change: %v", err)
	}
	assertCatalogSyncDeletePair(t, ctx, db.SQL, "item-deleted", 3)

	expiresAt := now.Add(48 * time.Hour)
	tombstone := library.Tombstone{
		Sequence: 800, CatalogID: "catalog-1", EntityType: library.CatalogEntityItem,
		EntityID: "item-expiring", Revision: 4, DeletedAt: now.Add(3 * time.Minute),
		ExpiresAt: &expiresAt,
	}
	if err := repo.SaveTombstone(ctx, tombstone); err != nil {
		t.Fatalf("save tombstone: %v", err)
	}
	assertCatalogSyncDeletePair(t, ctx, db.SQL, "item-expiring", 4)

	if _, err := db.SQL.ExecContext(ctx, `
CREATE TRIGGER reject_catalog_sync_test_tombstone
BEFORE INSERT ON library_catalog_tombstones
WHEN NEW.entity_id = 'item-rejected'
BEGIN
  SELECT RAISE(ABORT, 'reject test tombstone');
END
`); err != nil {
		t.Fatalf("create rejecting trigger: %v", err)
	}
	failedDelete := deleteChange
	failedDelete.EntityID = "item-rejected"
	failedDelete.Revision = 5
	if err := repo.Save(ctx, failedDelete); err == nil {
		t.Fatal("delete append unexpectedly succeeded when tombstone insert failed")
	}
	var rejectedChangeCount int
	if err := db.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_catalog_changes WHERE entity_id = 'item-rejected'
`).Scan(&rejectedChangeCount); err != nil {
		t.Fatalf("count rejected changes: %v", err)
	}
	if rejectedChangeCount != 0 {
		t.Fatalf("delete change survived rolled-back tombstone insert: %d", rejectedChangeCount)
	}

	removed, err := repo.DeleteExpiredTombstones(ctx, expiresAt.Add(time.Second))
	if err != nil || removed != 1 {
		t.Fatalf("delete expired tombstones: removed=%d err=%v", removed, err)
	}
	var retainedChangeCount int
	if err := db.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM library_catalog_changes WHERE entity_id = 'item-expiring'
`).Scan(&retainedChangeCount); err != nil {
		t.Fatalf("count retained delete changes: %v", err)
	}
	if retainedChangeCount != 1 {
		t.Fatalf("expired tombstone cleanup removed durable change history: %d", retainedChangeCount)
	}
}

func TestSQLiteCatalogMigrationRepositoryRoundTripAndCheckpointResume(t *testing.T) {
	ctx := context.Background()
	db := openCatalogSyncTestDB(t, ctx)
	now := time.Date(2026, 7, 13, 7, 0, 0, 0, time.UTC)
	seedCatalogSyncTestGraph(t, ctx, db.SQL, now)
	repo := NewSQLiteCatalogMigrationRepository(db.Bun)

	mapping, err := library.NewLegacyMapping(library.LegacyMappingParams{
		MigrationID: "catalog-v1", CatalogID: "catalog-1", SourceType: "library_file",
		SourceID: "file-1", TargetType: "item", TargetID: "item-1",
		SourceFingerprint: "sha256:first", MigratedAt: now,
	})
	if err != nil {
		t.Fatalf("new mapping: %v", err)
	}
	if err := repo.SaveMapping(ctx, mapping); err != nil {
		t.Fatalf("save mapping: %v", err)
	}
	loadedMapping, err := repo.GetMapping(ctx, mapping.MigrationID, mapping.SourceType, mapping.SourceID)
	if err != nil || loadedMapping.TargetID != "item-1" || loadedMapping.SourceFingerprint != "sha256:first" {
		t.Fatalf("unexpected mapping: %#v, err=%v", loadedMapping, err)
	}
	mapping.SourceFingerprint = "sha256:second"
	mapping.MigratedAt = now.Add(time.Minute)
	if err := repo.SaveMapping(ctx, mapping); err != nil {
		t.Fatalf("resave idempotent mapping: %v", err)
	}
	loadedMapping, err = repo.GetMapping(ctx, mapping.MigrationID, mapping.SourceType, mapping.SourceID)
	if err != nil || loadedMapping.SourceFingerprint != "sha256:second" {
		t.Fatalf("unexpected updated mapping: %#v, err=%v", loadedMapping, err)
	}

	startedAt := now.Add(time.Minute)
	updatedAt := startedAt.Add(time.Minute)
	checkpoint, err := library.NewMigrationCheckpoint(library.MigrationCheckpointParams{
		MigrationID: "catalog-v1", CatalogID: "catalog-1", Phase: "backfill", Status: "running",
		Cursor: "file-00042", Processed: 42, Failed: 1, StartedAt: &startedAt,
		CreatedAt: &now, UpdatedAt: &updatedAt,
	})
	if err != nil {
		t.Fatalf("new checkpoint: %v", err)
	}
	if err := repo.SaveCheckpoint(ctx, checkpoint); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	loadedCheckpoint, err := repo.GetCheckpoint(ctx, checkpoint.MigrationID, checkpoint.Phase)
	if err != nil || loadedCheckpoint.Cursor != "file-00042" || loadedCheckpoint.Processed != 42 {
		t.Fatalf("unexpected checkpoint: %#v, err=%v", loadedCheckpoint, err)
	}
	checkpoint.Cursor = "file-00084"
	checkpoint.Processed = 84
	checkpoint.UpdatedAt = updatedAt.Add(time.Minute)
	if err := repo.SaveCheckpoint(ctx, checkpoint); err != nil {
		t.Fatalf("advance checkpoint: %v", err)
	}
	loadedCheckpoint, err = repo.GetCheckpoint(ctx, checkpoint.MigrationID, checkpoint.Phase)
	if err != nil || loadedCheckpoint.Cursor != "file-00084" || loadedCheckpoint.Processed != 84 {
		t.Fatalf("unexpected resumed checkpoint: %#v, err=%v", loadedCheckpoint, err)
	}
}

func openCatalogSyncTestDB(t *testing.T, ctx context.Context) *persistence.Database {
	t.Helper()
	db, err := openLibraryRepoTestDatabase(t, ctx, "catalog-sync.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedCatalogSyncTestGraph(t *testing.T, ctx context.Context, db *sql.DB, now time.Time) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_catalogs (id, name, status, is_default, created_at, updated_at)
VALUES ('catalog-1', 'Library', 'active', TRUE, ?, ?)
`, now, now); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO library_catalog_items (
  id, catalog_id, category, status, title, sort_title, revision, created_at, updated_at
) VALUES ('item-1', 'catalog-1', 'video', 'active', 'Video', 'Video', 1, ?, ?)
`, now, now); err != nil {
		t.Fatalf("seed catalog item: %v", err)
	}
}

func assertCatalogSyncDeletePair(t *testing.T, ctx context.Context, db *sql.DB, entityID string, revision int64) {
	t.Helper()
	var changeSequence, tombstoneSequence int64
	err := db.QueryRowContext(ctx, `
SELECT changes.sequence, tombstones.sequence
FROM library_catalog_changes AS changes
JOIN library_catalog_tombstones AS tombstones ON tombstones.sequence = changes.sequence
WHERE changes.entity_id = ? AND changes.kind = 'delete' AND changes.revision = ?
`, entityID, revision).Scan(&changeSequence, &tombstoneSequence)
	if err != nil {
		t.Fatalf("load delete/tombstone pair for %s: %v", entityID, err)
	}
	if changeSequence <= 0 || changeSequence != tombstoneSequence {
		t.Fatalf("invalid delete/tombstone pair for %s: change=%d tombstone=%d", entityID, changeSequence, tombstoneSequence)
	}
}
