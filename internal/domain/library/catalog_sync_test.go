package library

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestDeviceGrantNormalizesScopesAndNeverNeedsRawCredentials(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	grant, err := NewDeviceGrant(DeviceGrantParams{
		ID: "grant-1", CatalogID: "catalog-1", DeviceID: "iphone-1", DeviceName: " Arnold's iPhone ",
		CredentialHash: "sha256:credential", PublicKeyHash: "sha256:public-key",
		Scopes: []string{"tasks.read", "library.read", "tasks.read"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("NewDeviceGrant: %v", err)
	}
	wantScopes := []DeviceScope{DeviceScopeLibraryRead, DeviceScopeTasksRead}
	if !reflect.DeepEqual(grant.Scopes, wantScopes) || !grant.HasScope(DeviceScopeLibraryRead) {
		t.Fatalf("unexpected scopes: %#v", grant.Scopes)
	}
	if _, err := NewDeviceGrant(DeviceGrantParams{
		ID: "grant-2", CatalogID: "catalog-1", DeviceID: "iphone-2", DeviceName: "iPhone",
		CredentialHash: "hash", PublicKeyHash: "public-key-hash", Scopes: []string{"library.manage"},
	}); !errors.Is(err, ErrInvalidDeviceGrant) {
		t.Fatalf("unknown management scope must fail closed, got %v", err)
	}
	if !grant.IsEffective(now) {
		t.Fatal("active non-expired grant should be effective")
	}
	if _, err := NewDeviceGrant(DeviceGrantParams{
		ID: "grant-1", CatalogID: "catalog-1", DeviceID: "iphone-1", DeviceName: "iPhone",
		CredentialHash: "raw credential", PublicKeyHash: "hash", Scopes: []string{"library.read"},
	}); !errors.Is(err, ErrInvalidDeviceGrant) {
		t.Fatalf("credential values containing whitespace must fail, got %v", err)
	}
}

func TestDeviceGrantAcceptsRSSScopes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	grant, err := NewDeviceGrant(DeviceGrantParams{
		ID: "grant-rss", CatalogID: "catalog-1", DeviceID: "iphone-rss", DeviceName: "RSS Reader",
		CredentialHash: "sha256:credential", PublicKeyHash: "sha256:public-key",
		Scopes: []string{"rss.state", "rss.fetch", "rss.read", "rss.manage", "rss.read"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("NewDeviceGrant: %v", err)
	}
	wantScopes := []DeviceScope{DeviceScopeRSSFetch, DeviceScopeRSSManage, DeviceScopeRSSRead, DeviceScopeRSSState}
	if !reflect.DeepEqual(grant.Scopes, wantScopes) || !grant.HasScope(DeviceScopeRSSManage) ||
		!grant.HasScope(DeviceScopeRSSFetch) {
		t.Fatalf("unexpected RSS scopes: %#v", grant.Scopes)
	}
}

func TestDeviceGrantAcceptsMusicScopes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	grant, err := NewDeviceGrant(DeviceGrantParams{
		ID: "grant-music", CatalogID: "catalog-1", DeviceID: "iphone-music", DeviceName: "Music Player",
		CredentialHash: "sha256:credential", PublicKeyHash: "sha256:public-key",
		Scopes: []string{"music.state", "music.read", "music.manage", "music.read"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("NewDeviceGrant: %v", err)
	}
	wantScopes := []DeviceScope{DeviceScopeMusicManage, DeviceScopeMusicRead, DeviceScopeMusicState}
	if !reflect.DeepEqual(grant.Scopes, wantScopes) || !grant.HasScope(DeviceScopeMusicManage) {
		t.Fatalf("unexpected Music scopes: %#v", grant.Scopes)
	}
}

func TestCatalogChangeAndTombstoneProvideMonotonicSyncContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	change, err := NewCatalogChange(CatalogChangeParams{
		Sequence: 42, CatalogID: "catalog-1", EntityType: "ITEM", EntityID: "item-1",
		Kind: "delete", Revision: 7, OccurredAt: now,
	})
	if err != nil || change.Sequence != 42 || change.EntityType != CatalogEntityItem || change.Kind != CatalogChangeDelete {
		t.Fatalf("unexpected change: %#v, %v", change, err)
	}
	expiresAt := now.Add(30 * 24 * time.Hour)
	tombstone, err := NewTombstone(TombstoneParams{
		Sequence: 42, CatalogID: "catalog-1", EntityType: "item", EntityID: "item-1",
		Revision: 7, DeletedAt: now, ExpiresAt: &expiresAt,
	})
	if err != nil || tombstone.ExpiresAt == nil || tombstone.Sequence != change.Sequence {
		t.Fatalf("unexpected tombstone: %#v, %v", tombstone, err)
	}
	if _, err := NewCatalogChange(CatalogChangeParams{
		Sequence: 0, CatalogID: "catalog-1", EntityType: "item", EntityID: "item-1", Kind: "upsert", Revision: 1,
	}); !errors.Is(err, ErrInvalidCatalogChange) {
		t.Fatalf("non-positive sequence must fail, got %v", err)
	}
}

func TestCatalogSyncStateScopesCursorToPersistentEpoch(t *testing.T) {
	t.Parallel()
	rotatedAt := time.Date(2026, 7, 13, 8, 0, 0, 123, time.FixedZone("CST", 8*60*60))
	state, err := NewCatalogSyncState(CatalogSyncStateParams{
		CatalogID: " catalog-1 ", Epoch: "0123456789abcdef0123456789abcdef",
		Cursor: 42, RotatedAt: rotatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.CatalogID != "catalog-1" || state.Cursor != 42 || state.RotatedAt.Location() != time.UTC {
		t.Fatalf("sync state = %+v", state)
	}
	for name, params := range map[string]CatalogSyncStateParams{
		"uppercase epoch": {CatalogID: "catalog-1", Epoch: "0123456789ABCDEF0123456789ABCDEF", RotatedAt: rotatedAt},
		"short epoch":     {CatalogID: "catalog-1", Epoch: "abcdef", RotatedAt: rotatedAt},
		"negative cursor": {CatalogID: "catalog-1", Epoch: "0123456789abcdef0123456789abcdef", Cursor: -1, RotatedAt: rotatedAt},
		"zero rotation":   {CatalogID: "catalog-1", Epoch: "0123456789abcdef0123456789abcdef"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewCatalogSyncState(params); !errors.Is(err, ErrInvalidCatalogSyncState) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestCatalogChangeAcceptsProfessionalLibraryEntities(t *testing.T) {
	t.Parallel()

	for _, entityType := range []CatalogEntityType{CatalogEntityRepresentation, CatalogEntityMetadataEntry} {
		change, err := NewCatalogChange(CatalogChangeParams{
			Sequence: 1, CatalogID: "catalog-1", EntityType: string(entityType), EntityID: "entity-1",
			Kind: "upsert", Revision: 1,
		})
		if err != nil || change.EntityType != entityType {
			t.Fatalf("entity type %q rejected: %#v, %v", entityType, change, err)
		}
	}
}

func TestLegacyMappingAndCheckpointAreIdempotencyKeys(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	mapping, err := NewLegacyMapping(LegacyMappingParams{
		MigrationID: "catalog-v1", CatalogID: "catalog-1", SourceType: "library_file", SourceID: "file-1",
		TargetType: "item_asset", TargetID: "link-1", SourceFingerprint: "sha256:abc", MigratedAt: now,
	})
	if err != nil || mapping.SourceType != LegacyEntityFile || mapping.TargetType != CatalogEntityItemAsset {
		t.Fatalf("unexpected mapping: %#v, %v", mapping, err)
	}

	startedAt := now.Add(time.Minute)
	updatedAt := startedAt.Add(time.Minute)
	checkpoint, err := NewMigrationCheckpoint(MigrationCheckpointParams{
		MigrationID: "catalog-v1", CatalogID: "catalog-1", Phase: "backfill", Status: "running",
		Cursor: "file-00042", Processed: 42, Failed: 1, StartedAt: &startedAt, CreatedAt: &now, UpdatedAt: &updatedAt,
	})
	if err != nil || checkpoint.Cursor != "file-00042" || checkpoint.Status != MigrationStatusRunning {
		t.Fatalf("unexpected checkpoint: %#v, %v", checkpoint, err)
	}
	if _, err := NewMigrationCheckpoint(MigrationCheckpointParams{
		MigrationID: "catalog-v1", CatalogID: "catalog-1", Phase: "backfill", Status: "completed",
		Processed: 42, StartedAt: &startedAt, CreatedAt: &now, UpdatedAt: &updatedAt,
	}); !errors.Is(err, ErrInvalidMigrationCheckpoint) {
		t.Fatalf("completed checkpoint without finish time must fail, got %v", err)
	}
}
