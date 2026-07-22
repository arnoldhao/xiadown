package library

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestNewCatalogAndItemNormalizeStableFields(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	catalog, err := NewCatalog(CatalogParams{
		ID: " catalog-1 ", Name: " Main Library ", Status: " ACTIVE ", CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	if catalog.ID != "catalog-1" || catalog.Name != "Main Library" || catalog.Status != CatalogStatusActive {
		t.Fatalf("unexpected catalog normalization: %#v", catalog)
	}
	if catalog.CreatedAt.Location() != time.UTC || !catalog.UpdatedAt.Equal(catalog.CreatedAt) {
		t.Fatalf("unexpected catalog times: %#v", catalog)
	}

	item, err := NewItem(ItemParams{
		ID: "item-1", CatalogID: catalog.ID, Category: " VIDEO ", Title: " Demo ", CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("NewItem: %v", err)
	}
	if item.Category != ItemCategoryVideo || item.Status != ItemStatusActive || item.Revision != 1 || item.SortTitle != "Demo" {
		t.Fatalf("unexpected item defaults: %#v", item)
	}
}

func TestNewItemEnforcesLifecycleAndOptimisticRevision(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	trashedAt := createdAt.Add(time.Hour)
	item, err := NewItem(ItemParams{
		ID: "item-1", CatalogID: "catalog-1", Category: "book", Status: "trashed",
		Title: "Book", Revision: 4, TrashedAt: &trashedAt, CreatedAt: &createdAt, UpdatedAt: &trashedAt,
	})
	if err != nil || item.Revision != 4 || item.TrashedAt == nil {
		t.Fatalf("expected valid trashed item, got %#v, %v", item, err)
	}

	invalid := []ItemParams{
		{ID: "item-1", CatalogID: "catalog-1", Category: "task", Title: "Task"},
		{ID: "item-1", CatalogID: "catalog-1", Category: "book", Status: "trashed", Title: "Book"},
		{ID: "item-1", CatalogID: "catalog-1", Category: "book", Title: "Book", Revision: -1},
		{ID: "item with spaces", CatalogID: "catalog-1", Category: "book", Title: "Book"},
	}
	for _, params := range invalid {
		if _, err := NewItem(params); !errors.Is(err, ErrInvalidCatalogItem) {
			t.Fatalf("expected invalid item for %#v, got %v", params, err)
		}
	}
}

func TestNewItemAssetKeepsPhysicalFilesBehindExplicitRoles(t *testing.T) {
	t.Parallel()

	asset, err := NewItemAsset(ItemAssetParams{
		ID: "link-1", ItemID: "item-1", FileID: "file-1", Role: " REPRESENTATION ", Label: " 1080p ", Position: 1,
	})
	if err != nil {
		t.Fatalf("NewItemAsset: %v", err)
	}
	if asset.Role != ItemAssetRoleRepresentation || asset.Label != "1080p" || asset.FileID != "file-1" {
		t.Fatalf("unexpected item asset: %#v", asset)
	}
	if _, err := NewItemAsset(ItemAssetParams{ID: "link", ItemID: "item", FileID: "file", Role: "primary"}); !errors.Is(err, ErrInvalidItemAsset) {
		t.Fatalf("expected unknown role to be rejected, got %v", err)
	}
}

func TestRepresentationIsIndependentFromPhysicalAssetRole(t *testing.T) {
	t.Parallel()

	width, height := 1920, 1080
	duration, bitrate, size := int64(90_000), int64(8_000_000), int64(42_000_000)
	representation, err := NewRepresentation(RepresentationParams{
		ID: "representation-1", CatalogID: "catalog-1", ItemID: "item-1", AssetID: "asset-1",
		Kind: "OPTIMIZED", MediaType: "Video/MP4", Container: "mp4", Codec: "h264",
		Width: &width, Height: &height, DurationMs: &duration, BitrateBps: &bitrate,
		Language: "en-US", SizeBytes: &size, Availability: "available", Revision: 3,
	})
	if err != nil {
		t.Fatalf("NewRepresentation: %v", err)
	}
	if representation.Kind != RepresentationKindOptimized || representation.Purpose != RepresentationPurposePlayback ||
		representation.MediaType != "video/mp4" || representation.Revision != 3 {
		t.Fatalf("unexpected representation: %#v", representation)
	}
	*representation.Width = 1
	if width != 1920 {
		t.Fatal("representation retained caller pointer")
	}
	badSize := int64(-1)
	if _, err := NewRepresentation(RepresentationParams{
		ID: "representation-1", CatalogID: "catalog-1", ItemID: "item-1", AssetID: "asset-1",
		Kind: "optimized", SizeBytes: &badSize,
	}); !errors.Is(err, ErrInvalidRepresentation) {
		t.Fatalf("negative size should fail, got %v", err)
	}
	if _, err := NewRepresentation(RepresentationParams{
		ID: "representation-1", CatalogID: "catalog-1", ItemID: "item-1", AssetID: "asset-1",
		Kind: "optimized", ChecksumAlgorithm: "sha256", Checksum: "not-a-digest",
	}); !errors.Is(err, ErrInvalidRepresentation) {
		t.Fatalf("invalid checksum should fail, got %v", err)
	}
}

func TestMetadataEntryIsTypedProvenancedAndRevisioned(t *testing.T) {
	t.Parallel()

	confidence := 0.98
	entry, err := NewMetadataEntry(MetadataEntryParams{
		ID: "metadata-1", CatalogID: "catalog-1", ItemID: "item-1", RepresentationID: "representation-1",
		Namespace: " DC.Terms ", Key: "CREATED", ValueType: "datetime",
		ValueJSON: `"2026-07-13T08:00:00Z"`, Language: "en", Position: 0,
		Source: "embedded", Provenance: "ffprobe:stream/0", Confidence: &confidence,
		Locked: true, Revision: 2,
	})
	if err != nil {
		t.Fatalf("NewMetadataEntry: %v", err)
	}
	if entry.Namespace != "dc.terms" || entry.Key != "created" || entry.Source != MetadataSourceEmbedded ||
		entry.Confidence == nil || *entry.Confidence != confidence || string(entry.Value) != `"2026-07-13T08:00:00Z"` {
		t.Fatalf("unexpected metadata entry: %#v", entry)
	}
	*entry.Confidence = 0
	if confidence != 0.98 {
		t.Fatal("metadata entry retained caller confidence pointer")
	}
	invalid := []MetadataEntryParams{
		{ID: "m", CatalogID: "c", ItemID: "i", Namespace: "dc", Key: "year", ValueType: "integer", ValueJSON: `2026.5`, Source: "user", Provenance: "user"},
		{ID: "m", CatalogID: "c", ItemID: "i", Namespace: "dc", Key: "date", ValueType: "date", ValueJSON: `"2026-99-99"`, Source: "user", Provenance: "user"},
		{ID: "m", CatalogID: "c", ItemID: "i", Namespace: "dc", Key: "title", ValueType: "string", ValueJSON: `"Title"`, Source: "unknown", Provenance: "user"},
		{ID: "m", CatalogID: "c", ItemID: "i", Namespace: "bad key", Key: "title", ValueType: "string", ValueJSON: `"Title"`, Source: "user", Provenance: "user"},
	}
	for _, params := range invalid {
		if _, err := NewMetadataEntry(params); !errors.Is(err, ErrInvalidMetadataEntry) {
			t.Fatalf("expected invalid metadata entry for %#v, got %v", params, err)
		}
	}
}

func TestStorageRootCollectionAndTagsEnforceManagementInvariants(t *testing.T) {
	t.Parallel()

	root, err := NewStorageRoot(StorageRootParams{
		ID: "root-1", CatalogID: "catalog-1", Name: " Media ", Path: " /Volumes/Media ",
		Mode: "managed", Status: "offline",
	})
	if err != nil || root.Path != "/Volumes/Media" || root.Mode != StorageRootModeManaged {
		t.Fatalf("unexpected storage root: %#v, %v", root, err)
	}
	if _, err := NewStorageRoot(StorageRootParams{
		ID: "root-1", CatalogID: "catalog-1", Name: "Media", Path: "/tmp/media",
		Mode: "referenced", Status: "error",
	}); !errors.Is(err, ErrInvalidStorageRoot) {
		t.Fatalf("error status without details must fail, got %v", err)
	}

	collection, err := NewCollection(CollectionParams{
		ID: "collection-1", CatalogID: "catalog-1", Name: "Recent Videos", Kind: "smart", SmartQuery: "category:video",
	})
	if err != nil || collection.Kind != CollectionKindSmart || collection.Revision != 1 {
		t.Fatalf("unexpected smart collection: %#v, %v", collection, err)
	}
	if _, err := NewCollection(CollectionParams{
		ID: "collection-1", CatalogID: "catalog-1", Name: "Broken", Kind: "smart",
	}); !errors.Is(err, ErrInvalidCollection) {
		t.Fatalf("smart collection without query must fail, got %v", err)
	}

	tag, err := NewTag(TagParams{ID: "tag-1", CatalogID: "catalog-1", Name: "  Road   Trip  "})
	if err != nil || tag.Name != "Road Trip" || tag.NormalizedName != "road trip" {
		t.Fatalf("unexpected tag: %#v, %v", tag, err)
	}
}

func TestUserStateRejectsAmbiguousProgress(t *testing.T) {
	t.Parallel()

	state, err := NewUserState(UserStateParams{
		ID: "state-1", CatalogID: "catalog-1", ItemID: "item-1", UserID: "user-1",
		Progress: 1, Completed: true, Rating: 5, PositionMs: 1200,
	})
	if err != nil || state.Revision != 1 {
		t.Fatalf("unexpected user state: %#v, %v", state, err)
	}
	for _, progress := range []float64{-0.1, 1.1, math.NaN()} {
		_, err := NewUserState(UserStateParams{
			ID: "state-1", CatalogID: "catalog-1", ItemID: "item-1", UserID: "user-1", Progress: progress,
		})
		if !errors.Is(err, ErrInvalidUserState) {
			t.Fatalf("expected progress %v to fail, got %v", progress, err)
		}
	}
}
