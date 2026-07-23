package dto

// Catalog DTOs deliberately expose logical item IDs and keep the legacy file
// metadata nested under assets. This gives the desktop workspace one coherent
// model without losing information needed by preview and playback surfaces.

type CatalogDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	IsDefault   bool   `json:"isDefault"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type CatalogCountDTO struct {
	All    int `json:"all"`
	Video  int `json:"video"`
	Audio  int `json:"audio"`
	Books  int `json:"books"`
	Images int `json:"images"`
	Others int `json:"others"`
}

type CatalogStatusCountDTO struct {
	Active      int `json:"active"`
	NeedsReview int `json:"needsReview"`
	Missing     int `json:"missing"`
	Trashed     int `json:"trashed"`
}

type CatalogHealthCountDTO struct {
	AssetLinks             int `json:"assetLinks"`
	ItemsWithoutAssets     int `json:"itemsWithoutAssets"`
	UnavailableAssetFiles  int `json:"unavailableAssetFiles"`
	LegacyFilesWithErrors  int `json:"legacyFilesWithErrors"`
	OfflineStorageRoots    int `json:"offlineStorageRoots"`
	ReadOnlyStorageRoots   int `json:"readOnlyStorageRoots"`
	StorageRootsWithErrors int `json:"storageRootsWithErrors"`
}

type CatalogOverviewDTO struct {
	Catalog        CatalogDTO            `json:"catalog"`
	TotalSizeBytes int64                 `json:"totalSizeBytes"`
	Categories     CatalogCountDTO       `json:"categories"`
	Statuses       CatalogStatusCountDTO `json:"statuses"`
	Health         CatalogHealthCountDTO `json:"health"`
}

type CatalogItemDTO struct {
	ID             string `json:"id"`
	CatalogID      string `json:"catalogId"`
	Category       string `json:"category"`
	Kind           string `json:"kind,omitempty"`
	Format         string `json:"format,omitempty"`
	DurationMs     *int64 `json:"durationMs,omitempty"`
	SizeBytes      *int64 `json:"sizeBytes,omitempty"`
	PrimaryAssetID string `json:"primaryAssetId,omitempty"`
	PrimaryFileID  string `json:"primaryFileId,omitempty"`
	ArtworkAssetID string `json:"artworkAssetId,omitempty"`
	ArtworkFileID  string `json:"artworkFileId,omitempty"`
	Status         string `json:"status"`
	Title          string `json:"title"`
	SortTitle      string `json:"sortTitle"`
	Description    string `json:"description,omitempty"`
	Revision       int64  `json:"revision"`
	TrashedAt      string `json:"trashedAt,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type CatalogItemAssetDTO struct {
	ID            string          `json:"id"`
	ItemID        string          `json:"itemId"`
	FileID        string          `json:"fileId"`
	StorageRootID string          `json:"storageRootId,omitempty"`
	Role          string          `json:"role"`
	Label         string          `json:"label,omitempty"`
	Position      int             `json:"position"`
	FileAvailable bool            `json:"fileAvailable"`
	File          *LibraryFileDTO `json:"file,omitempty"`
	CreatedAt     string          `json:"createdAt"`
	UpdatedAt     string          `json:"updatedAt"`
}

type CatalogRepresentationDTO struct {
	ID                string `json:"id"`
	CatalogID         string `json:"catalogId"`
	ItemID            string `json:"itemId"`
	AssetID           string `json:"assetId"`
	StorageRootID     string `json:"storageRootId,omitempty"`
	Kind              string `json:"kind"`
	Purpose           string `json:"purpose"`
	MediaType         string `json:"mediaType,omitempty"`
	Container         string `json:"container,omitempty"`
	Codec             string `json:"codec,omitempty"`
	Width             *int   `json:"width,omitempty"`
	Height            *int   `json:"height,omitempty"`
	DurationMs        *int64 `json:"durationMs,omitempty"`
	BitrateBps        *int64 `json:"bitrateBps,omitempty"`
	Language          string `json:"language,omitempty"`
	ChecksumAlgorithm string `json:"checksumAlgorithm,omitempty"`
	Checksum          string `json:"checksum,omitempty"`
	SizeBytes         *int64 `json:"sizeBytes,omitempty"`
	Availability      string `json:"availability"`
	Revision          int64  `json:"revision"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type SaveCatalogRepresentationRequest struct {
	ID                string `json:"id,omitempty"`
	ItemID            string `json:"itemId"`
	AssetID           string `json:"assetId"`
	ExpectedRevision  int64  `json:"expectedRevision,omitempty"`
	Kind              string `json:"kind"`
	Purpose           string `json:"purpose,omitempty"`
	MediaType         string `json:"mediaType,omitempty"`
	Container         string `json:"container,omitempty"`
	Codec             string `json:"codec,omitempty"`
	Width             *int   `json:"width,omitempty"`
	Height            *int   `json:"height,omitempty"`
	DurationMs        *int64 `json:"durationMs,omitempty"`
	BitrateBps        *int64 `json:"bitrateBps,omitempty"`
	Language          string `json:"language,omitempty"`
	ChecksumAlgorithm string `json:"checksumAlgorithm,omitempty"`
	Checksum          string `json:"checksum,omitempty"`
	SizeBytes         *int64 `json:"sizeBytes,omitempty"`
	Availability      string `json:"availability,omitempty"`
	ActorID           string `json:"actorId,omitempty"`
}

type ListCatalogRepresentationsRequest struct {
	ItemID string `json:"itemId"`
}

type CatalogMetadataEntryDTO struct {
	ID               string   `json:"id"`
	CatalogID        string   `json:"catalogId"`
	ItemID           string   `json:"itemId"`
	RepresentationID string   `json:"representationId,omitempty"`
	Namespace        string   `json:"namespace"`
	Key              string   `json:"key"`
	ValueType        string   `json:"valueType"`
	ValueJSON        string   `json:"valueJson"`
	Language         string   `json:"language,omitempty"`
	Position         int      `json:"position"`
	Source           string   `json:"source"`
	Provenance       string   `json:"provenance"`
	Confidence       *float64 `json:"confidence,omitempty"`
	Locked           bool     `json:"locked"`
	Revision         int64    `json:"revision"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

type SaveCatalogMetadataEntryRequest struct {
	ID               string   `json:"id,omitempty"`
	ItemID           string   `json:"itemId"`
	RepresentationID string   `json:"representationId,omitempty"`
	ExpectedRevision int64    `json:"expectedRevision,omitempty"`
	Namespace        string   `json:"namespace"`
	Key              string   `json:"key"`
	ValueType        string   `json:"valueType"`
	ValueJSON        string   `json:"valueJson"`
	Language         string   `json:"language,omitempty"`
	Position         int      `json:"position,omitempty"`
	Source           string   `json:"source"`
	Provenance       string   `json:"provenance"`
	Confidence       *float64 `json:"confidence,omitempty"`
	Locked           bool     `json:"locked,omitempty"`
	ActorID          string   `json:"actorId,omitempty"`
}

type ListCatalogMetadataEntriesRequest struct {
	ItemID           string `json:"itemId"`
	RepresentationID string `json:"representationId,omitempty"`
}

type CatalogItemDetailDTO struct {
	Item            CatalogItemDTO             `json:"item"`
	Assets          []CatalogItemAssetDTO      `json:"assets"`
	Representations []CatalogRepresentationDTO `json:"representations"`
	Metadata        []CatalogMetadataEntryDTO  `json:"metadata"`
	Tags            []CatalogTagDTO            `json:"tags"`
	UserState       *CatalogUserStateDTO       `json:"userState,omitempty"`
}

type ListCatalogItemsRequest struct {
	Category       string `json:"category,omitempty"`
	Status         string `json:"status,omitempty"`
	ExcludeTrashed bool   `json:"excludeTrashed,omitempty"`
	Query          string `json:"query,omitempty"`
	Sort           string `json:"sort,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	Offset         int    `json:"offset,omitempty"`
}

type ListCatalogItemsResponse struct {
	Items  []CatalogItemDTO `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type GetCatalogItemRequest struct {
	ID     string `json:"id"`
	UserID string `json:"userId,omitempty"`
}

type UpdateCatalogItemRequest struct {
	ID               string  `json:"id"`
	ExpectedRevision int64   `json:"expectedRevision"`
	Title            *string `json:"title,omitempty"`
	Description      *string `json:"description,omitempty"`
	Category         *string `json:"category,omitempty"`
	ActorID          string  `json:"actorId,omitempty"`
	UserID           string  `json:"userId,omitempty"`
}

type CatalogItemLifecycleRequest struct {
	ID               string `json:"id"`
	ExpectedRevision int64  `json:"expectedRevision"`
	ActorID          string `json:"actorId,omitempty"`
	UserID           string `json:"userId,omitempty"`
}

type ListCatalogItemActivityRequest struct {
	ItemID string `json:"itemId"`
	Limit  int    `json:"limit,omitempty"`
}

type CatalogItemActivityDTO struct {
	Action     string `json:"action"`
	Revision   int64  `json:"revision"`
	Actor      string `json:"actor"`
	OccurredAt string `json:"occurredAt"`
}

type CatalogCollectionDTO struct {
	ID               string   `json:"id"`
	CatalogID        string   `json:"catalogId"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	Kind             string   `json:"kind"`
	SmartQuery       string   `json:"smartQuery,omitempty"`
	Revision         int64    `json:"revision"`
	ItemIDs          []string `json:"itemIds"`
	ItemIDsTruncated bool     `json:"itemIdsTruncated,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

type CatalogCollectionItemDTO struct {
	ID           string `json:"id"`
	CollectionID string `json:"collectionId"`
	ItemID       string `json:"itemId"`
	Position     int    `json:"position"`
}

type CatalogCollectionItemsPageDTO struct {
	CatalogID    string                     `json:"catalogId"`
	CollectionID string                     `json:"collectionId"`
	Items        []CatalogCollectionItemDTO `json:"items"`
	NextOffset   int                        `json:"nextOffset"`
	HasMore      bool                       `json:"hasMore"`
}

type SaveCatalogCollectionRequest struct {
	ID               string `json:"id,omitempty"`
	ExpectedRevision int64  `json:"expectedRevision,omitempty"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Kind             string `json:"kind,omitempty"`
	SmartQuery       string `json:"smartQuery,omitempty"`
}

type ReplaceCatalogCollectionItemsRequest struct {
	CollectionID     string   `json:"collectionId"`
	ItemIDs          []string `json:"itemIds"`
	ExpectedRevision int64    `json:"expectedRevision"`
}

type CatalogTagDTO struct {
	ID             string `json:"id"`
	CatalogID      string `json:"catalogId"`
	Name           string `json:"name"`
	NormalizedName string `json:"normalizedName"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type SaveCatalogTagRequest struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

type ReplaceCatalogItemTagsRequest struct {
	ItemID string   `json:"itemId"`
	TagIDs []string `json:"tagIds"`
}

type CatalogStorageRootDTO struct {
	ID            string `json:"id"`
	CatalogID     string `json:"catalogId"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	VolumeID      string `json:"volumeId,omitempty"`
	Mode          string `json:"mode"`
	Status        string `json:"status"`
	LastCheckedAt string `json:"lastCheckedAt,omitempty"`
	LastError     string `json:"lastError,omitempty"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type SaveCatalogStorageRootRequest struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	VolumeID string `json:"volumeId,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

type CheckCatalogStorageRootRequest struct {
	ID string `json:"id"`
}

type CatalogUserStateDTO struct {
	ID           string  `json:"id"`
	CatalogID    string  `json:"catalogId"`
	ItemID       string  `json:"itemId"`
	UserID       string  `json:"userId"`
	Favorite     bool    `json:"favorite"`
	Rating       int     `json:"rating"`
	Progress     float64 `json:"progress"`
	PositionMs   int64   `json:"positionMs"`
	Locator      string  `json:"locator,omitempty"`
	Completed    bool    `json:"completed"`
	Revision     int64   `json:"revision"`
	LastOpenedAt string  `json:"lastOpenedAt,omitempty"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type GetCatalogUserStateRequest struct {
	ItemID string `json:"itemId"`
	UserID string `json:"userId"`
}

type UpdateCatalogUserStateRequest struct {
	ItemID           string   `json:"itemId"`
	UserID           string   `json:"userId"`
	ExpectedRevision int64    `json:"expectedRevision"`
	Favorite         *bool    `json:"favorite,omitempty"`
	Rating           *int     `json:"rating,omitempty"`
	Progress         *float64 `json:"progress,omitempty"`
	PositionMs       *int64   `json:"positionMs,omitempty"`
	Locator          *string  `json:"locator,omitempty"`
	Completed        *bool    `json:"completed,omitempty"`
	OpenedNow        bool     `json:"openedNow,omitempty"`
	ActorID          string   `json:"actorId,omitempty"`
}

type CatalogMigrationAuditRequest struct {
	MigrationID string `json:"migrationId,omitempty"`
}

type CatalogMigrationAuditCountDTO struct {
	LegacyFiles                 int64 `json:"legacyFiles"`
	LegacyMappings              int64 `json:"legacyMappings"`
	AssetLinks                  int64 `json:"assetLinks"`
	Items                       int64 `json:"items"`
	ActiveItems                 int64 `json:"activeItems"`
	MissingItems                int64 `json:"missingItems"`
	TrashedItems                int64 `json:"trashedItems"`
	NeedsReviewItems            int64 `json:"needsReviewItems"`
	PreservedFileIDs            int64 `json:"preservedFileIds"`
	PreservedPhysicalReferences int64 `json:"preservedPhysicalReferences"`
	Representations             int64 `json:"representations"`
	MetadataEntries             int64 `json:"metadataEntries"`
}

type CatalogMigrationAuditFindingDTO struct {
	UnmappedLegacyFiles              int64 `json:"unmappedLegacyFiles"`
	DuplicateMappings                int64 `json:"duplicateMappings"`
	DanglingAssets                   int64 `json:"danglingAssets"`
	MissingMappingSources            int64 `json:"missingMappingSources"`
	MissingMappingTargets            int64 `json:"missingMappingTargets"`
	MappingAssetMismatches           int64 `json:"mappingAssetMismatches"`
	ChangedPhysicalReferences        int64 `json:"changedPhysicalReferences"`
	AssetsWithoutRepresentations     int64 `json:"assetsWithoutRepresentations"`
	DanglingRepresentations          int64 `json:"danglingRepresentations"`
	RepresentationMismatches         int64 `json:"representationMismatches"`
	DanglingMetadataEntries          int64 `json:"danglingMetadataEntries"`
	MetadataRepresentationMismatches int64 `json:"metadataRepresentationMismatches"`
	Total                            int64 `json:"total"`
}

type CatalogMigrationAuditIssueDTO struct {
	Kind             string `json:"kind"`
	SourceID         string `json:"sourceId,omitempty"`
	TargetID         string `json:"targetId,omitempty"`
	AssetID          string `json:"assetId,omitempty"`
	RepresentationID string `json:"representationId,omitempty"`
	MetadataEntryID  string `json:"metadataEntryId,omitempty"`
	Description      string `json:"description"`
}

type CatalogMigrationAuditDTO struct {
	CatalogID   string                          `json:"catalogId"`
	MigrationID string                          `json:"migrationId"`
	Healthy     bool                            `json:"healthy"`
	Counts      CatalogMigrationAuditCountDTO   `json:"counts"`
	Findings    CatalogMigrationAuditFindingDTO `json:"findings"`
	Issues      []CatalogMigrationAuditIssueDTO `json:"issues"`
	AuditedAt   string                          `json:"auditedAt"`
}
