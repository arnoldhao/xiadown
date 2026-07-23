package library

import (
	"context"
	"time"
)

type LibraryRepository interface {
	List(ctx context.Context) ([]Library, error)
	Get(ctx context.Context, id string) (Library, error)
	Save(ctx context.Context, item Library) error
	Delete(ctx context.Context, id string) error
}

// CatalogRepository is intentionally separate from LibraryRepository. Library
// remains the legacy download bundle while Catalog is the durable user-facing
// library aggregate.
type CatalogRepository interface {
	List(ctx context.Context) ([]Catalog, error)
	Get(ctx context.Context, id string) (Catalog, error)
	Save(ctx context.Context, item Catalog) error
	Delete(ctx context.Context, id string) error
}

type CatalogItemRepository interface {
	ListByCatalogID(ctx context.Context, catalogID string) ([]Item, error)
	Get(ctx context.Context, id string) (Item, error)
	Save(ctx context.Context, item Item) error
	Delete(ctx context.Context, id string) error
}

// CatalogItemSnapshotRepository is an optional, read-only extension for
// clients that need to build a stable Catalog generation without offset
// pagination. Implementations must return non-trashed items ordered by the
// stable Item ID, strictly after afterID, and bounded by limit.
type CatalogItemSnapshotRepository interface {
	ListSnapshotPageByCatalogID(ctx context.Context, catalogID, afterID string, limit int) ([]Item, error)
}

type ItemAssetRepository interface {
	ListByItemID(ctx context.Context, itemID string) ([]ItemAsset, error)
	Get(ctx context.Context, id string) (ItemAsset, error)
	Save(ctx context.Context, item ItemAsset) error
	Delete(ctx context.Context, id string) error
}

// RepresentationRepository stores the technical variants that make one Item
// playable, readable, previewable, or indexable. ItemAsset remains the stable
// physical-file link; Representation describes how that asset can be used.
type RepresentationRepository interface {
	ListRepresentationsByItemID(ctx context.Context, itemID string) ([]Representation, error)
	GetRepresentation(ctx context.Context, id string) (Representation, error)
	SaveRepresentation(ctx context.Context, item Representation) error
	DeleteRepresentation(ctx context.Context, id string) error
}

// MetadataEntryRepository stores typed, independently revisioned metadata.
// Repeated values are ordered by Position rather than being hidden in a JSON
// bag on Item.
type MetadataEntryRepository interface {
	ListMetadataEntriesByItemID(ctx context.Context, itemID string) ([]MetadataEntry, error)
	ListMetadataEntriesByRepresentationID(ctx context.Context, representationID string) ([]MetadataEntry, error)
	GetMetadataEntry(ctx context.Context, id string) (MetadataEntry, error)
	SaveMetadataEntry(ctx context.Context, item MetadataEntry) error
	DeleteMetadataEntry(ctx context.Context, id string) error
}

type StorageRootRepository interface {
	ListByCatalogID(ctx context.Context, catalogID string) ([]StorageRoot, error)
	Get(ctx context.Context, id string) (StorageRoot, error)
	Save(ctx context.Context, item StorageRoot) error
	Delete(ctx context.Context, id string) error
}

type CatalogCollectionRepository interface {
	ListByCatalogID(ctx context.Context, catalogID string) ([]Collection, error)
	Get(ctx context.Context, id string) (Collection, error)
	Save(ctx context.Context, item Collection) error
	Delete(ctx context.Context, id string) error
	ListItems(ctx context.Context, collectionID string) ([]CollectionItem, error)
	ReplaceItems(ctx context.Context, collection Collection, items []CollectionItem) error
}

type CatalogTagRepository interface {
	ListByCatalogID(ctx context.Context, catalogID string) ([]Tag, error)
	Save(ctx context.Context, item Tag) error
	Delete(ctx context.Context, id string) error
	ListByItemID(ctx context.Context, itemID string) ([]ItemTag, error)
	ReplaceItemTags(ctx context.Context, itemID string, tags []ItemTag) error
}

type UserStateRepository interface {
	Get(ctx context.Context, catalogID, itemID, userID string) (UserState, error)
	Save(ctx context.Context, item UserState) error
	Delete(ctx context.Context, id string) error
}

type DeviceGrantRepository interface {
	ListByCatalogID(ctx context.Context, catalogID string) ([]DeviceGrant, error)
	Get(ctx context.Context, id string) (DeviceGrant, error)
	Save(ctx context.Context, item DeviceGrant) error
	Delete(ctx context.Context, id string) error
}

// DeviceGrantManagementRepository owns security-sensitive grant writes. A
// mutation and its Catalog change record must commit atomically, while
// expectedRevision prevents concurrent administrators from silently
// overwriting one another.
type DeviceGrantManagementRepository interface {
	DeviceGrantRepository
	SaveDeviceGrantMutation(ctx context.Context, item DeviceGrant, expectedRevision int64, kind CatalogChangeKind, actorID string) error
	RecordDeviceGrantLastSeen(ctx context.Context, catalogID, id string, seenAt time.Time) error
}

type CatalogChangeRepository interface {
	ListAfter(ctx context.Context, catalogID string, sequence int64, limit int) ([]CatalogChange, error)
	Save(ctx context.Context, item CatalogChange) error
	SaveTombstone(ctx context.Context, item Tombstone) error
	DeleteExpiredTombstones(ctx context.Context, before time.Time) (int, error)
}

// CatalogSyncStateRepository returns the persistent feed generation together
// with the current high-water cursor. Clients must discard their incremental
// cursor whenever the epoch changes.
type CatalogSyncStateRepository interface {
	GetCatalogSyncState(ctx context.Context, catalogID string) (CatalogSyncState, error)
}

// CatalogMutationRepository owns writes that must update an aggregate and its
// durable change feed as one unit. expectedRevision is zero only when creating
// a new revisioned aggregate.
type CatalogMutationRepository interface {
	SaveItemMutation(ctx context.Context, item Item, expectedRevision int64, kind CatalogChangeKind, actorID string, tombstoneExpiresAt *time.Time) error
	SaveUserStateMutation(ctx context.Context, item UserState, expectedRevision int64, actorID string) error
	SaveCollectionMutation(ctx context.Context, item Collection, expectedRevision int64, actorID string) error
	ReplaceCollectionItemsMutation(ctx context.Context, item Collection, members []CollectionItem, expectedRevision int64, actorID string) error
	// Tags predate revisioned Catalog entities. Their change revision is the
	// monotonic mutation generation for the stable tag ID within one sync epoch.
	SaveTagMutation(ctx context.Context, item Tag, actorID string) error
	// Item-tag membership is one aggregate keyed by itemID, not by the random
	// binding row IDs. Replacing it (including with an empty set) emits one
	// item_tag upsert whose revision is that aggregate's mutation generation.
	ReplaceItemTagsMutation(ctx context.Context, catalogID, itemID string, members []ItemTag, actorID string, occurredAt time.Time) error
}

// CatalogProfessionalMutationRepository is an optional extension used by the
// desktop management surface. It keeps each revisioned professional entity
// and its sync cursor in one transaction while preserving the stable
// CatalogService constructor for legacy embedders.
type CatalogProfessionalMutationRepository interface {
	RepresentationRepository
	MetadataEntryRepository
	SaveRepresentationMutation(ctx context.Context, item Representation, expectedRevision int64, actorID string) error
	SaveMetadataEntryMutation(ctx context.Context, item MetadataEntry, expectedRevision int64, actorID string) error
	DeleteMetadataEntryMutation(ctx context.Context, item MetadataEntry, actorID string) error
	// SaveItemMetadataBatchMutation commits an optional Item revision together
	// with every metadata-entry upsert/delete and their change-feed records. An
	// entry in upserts carries its next revision; an entry in deletes carries
	// the current revision that must still exist.
	SaveItemMetadataBatchMutation(
		ctx context.Context,
		item *Item,
		expectedItemRevision int64,
		upserts []MetadataEntry,
		deletes []MetadataEntry,
		actorID string,
	) error
}

type CatalogMigrationRepository interface {
	GetMapping(ctx context.Context, migrationID string, sourceType LegacyEntityType, sourceID string) (LegacyMapping, error)
	SaveMapping(ctx context.Context, item LegacyMapping) error
	GetCheckpoint(ctx context.Context, migrationID string, phase MigrationPhase) (MigrationCheckpoint, error)
	SaveCheckpoint(ctx context.Context, item MigrationCheckpoint) error
}

// CatalogBackfillWriter owns the transaction boundary for legacy Catalog
// projection. SaveBundle must atomically persist all items, assets, mappings,
// and the checkpoint cursor for one legacy Library bundle.
type CatalogBackfillWriter interface {
	EnsureCatalog(ctx context.Context, item Catalog) error
	GetBackfillCheckpoint(ctx context.Context, migrationID string, phase MigrationPhase) (MigrationCheckpoint, bool, error)
	SaveBackfillBundle(ctx context.Context, bundle CatalogBackfillBundle) error
	SaveRuntimeProjection(ctx context.Context, bundle CatalogBackfillBundle) error
	SaveBackfillCheckpoint(ctx context.Context, item MigrationCheckpoint) error
}

type ModuleConfigRepository interface {
	Get(ctx context.Context) (ModuleConfig, error)
	Save(ctx context.Context, config ModuleConfig) error
}

type FileRepository interface {
	List(ctx context.Context) ([]LibraryFile, error)
	ListByLibraryID(ctx context.Context, libraryID string) ([]LibraryFile, error)
	Get(ctx context.Context, id string) (LibraryFile, error)
	Save(ctx context.Context, item LibraryFile) error
	Delete(ctx context.Context, id string) error
}

type ListenLocalTrackRepository interface {
	List(ctx context.Context, options ListenLocalTrackListOptions) ([]ListenLocalTrack, error)
	Get(ctx context.Context, fileID string) (ListenLocalTrack, error)
	Save(ctx context.Context, item ListenLocalTrack) error
	Delete(ctx context.Context, fileID string) error
	DeleteUnavailable(ctx context.Context) (int, error)
}

type ListenLocalPlaylistRepository interface {
	List(ctx context.Context) ([]ListenLocalPlaylist, error)
	CountItems(ctx context.Context, playlistIDs []string) (map[string]int, error)
	Get(ctx context.Context, id string) (ListenLocalPlaylist, error)
	Save(ctx context.Context, item ListenLocalPlaylist) error
	Delete(ctx context.Context, id string, expectedRevision int64) error
	ListItems(ctx context.Context, playlistID string) ([]ListenLocalPlaylistItem, error)
	// ReplaceItems atomically replaces the ordered membership and persists the
	// playlist metadata (including UpdatedAt) in the same transaction.
	ReplaceItems(ctx context.Context, playlist ListenLocalPlaylist, items []ListenLocalPlaylistItem) error
}

type ListenLocalMusicMembershipRepository interface {
	Get(ctx context.Context, fileID string) (ListenLocalMusicMembership, error)
	Save(ctx context.Context, item ListenLocalMusicMembership) error
}

type ListenLiveChannelRepository interface {
	List(ctx context.Context) (ListenLiveCatalogSnapshot, error)
	Replace(ctx context.Context, snapshot ListenLiveCatalogSnapshot) error
}

type OperationRepository interface {
	List(ctx context.Context) ([]LibraryOperation, error)
	ListByLibraryID(ctx context.Context, libraryID string) ([]LibraryOperation, error)
	Get(ctx context.Context, id string) (LibraryOperation, error)
	Save(ctx context.Context, item LibraryOperation) error
	Delete(ctx context.Context, id string) error
}

type ExternalProcessRepository interface {
	List(ctx context.Context) ([]ExternalProcess, error)
	Save(ctx context.Context, item ExternalProcess) error
	Delete(ctx context.Context, id string) error
}

type OperationChunkRepository interface {
	ListByOperationID(ctx context.Context, operationID string) ([]OperationChunk, error)
	Save(ctx context.Context, item OperationChunk) error
	DeleteByOperationID(ctx context.Context, operationID string) error
}

type HistoryRepository interface {
	ListByLibraryID(ctx context.Context, libraryID string) ([]HistoryRecord, error)
	Get(ctx context.Context, id string) (HistoryRecord, error)
	Save(ctx context.Context, item HistoryRecord) error
	Delete(ctx context.Context, id string) error
	DeleteByOperationID(ctx context.Context, operationID string) error
}

type WorkspaceStateRepository interface {
	ListByLibraryID(ctx context.Context, libraryID string) ([]WorkspaceStateRecord, error)
	GetHeadByLibraryID(ctx context.Context, libraryID string) (WorkspaceStateRecord, error)
	Save(ctx context.Context, item WorkspaceStateRecord) error
}

type FileEventRepository interface {
	ListByLibraryID(ctx context.Context, libraryID string) ([]FileEventRecord, error)
	Save(ctx context.Context, item FileEventRecord) error
}

type SubtitleDocumentRepository interface {
	Get(ctx context.Context, id string) (SubtitleDocument, error)
	GetByFileID(ctx context.Context, fileID string) (SubtitleDocument, error)
	Save(ctx context.Context, document SubtitleDocument) error
	DeleteByFileID(ctx context.Context, fileID string) error
}

type TranscodePresetRepository interface {
	List(ctx context.Context) ([]TranscodePreset, error)
	Get(ctx context.Context, id string) (TranscodePreset, error)
	Save(ctx context.Context, preset TranscodePreset) error
	Delete(ctx context.Context, id string) error
}
