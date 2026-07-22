package library

import "time"

type CatalogEntityType string

const (
	CatalogEntityCatalog        CatalogEntityType = "catalog"
	CatalogEntityItem           CatalogEntityType = "item"
	CatalogEntityItemAsset      CatalogEntityType = "item_asset"
	CatalogEntityRepresentation CatalogEntityType = "representation"
	CatalogEntityMetadataEntry  CatalogEntityType = "metadata_entry"
	CatalogEntityStorageRoot    CatalogEntityType = "storage_root"
	CatalogEntityCollection     CatalogEntityType = "collection"
	CatalogEntityCollectionItem CatalogEntityType = "collection_item"
	CatalogEntityTag            CatalogEntityType = "tag"
	CatalogEntityItemTag        CatalogEntityType = "item_tag"
	CatalogEntityUserState      CatalogEntityType = "user_state"
	CatalogEntityDeviceGrant    CatalogEntityType = "device_grant"
)

type CatalogChangeKind string

const (
	CatalogChangeUpsert CatalogChangeKind = "upsert"
	CatalogChangeDelete CatalogChangeKind = "delete"
)

type CatalogChange struct {
	Sequence   int64
	CatalogID  string
	EntityType CatalogEntityType
	EntityID   string
	Kind       CatalogChangeKind
	Revision   int64
	ActorID    string
	OccurredAt time.Time
}

type CatalogChangeParams struct {
	Sequence   int64
	CatalogID  string
	EntityType string
	EntityID   string
	Kind       string
	Revision   int64
	ActorID    string
	OccurredAt time.Time
}

// Change aliases the catalog-qualified name for application code that already
// operates inside a catalog boundary.
type Change = CatalogChange
type ChangeParams = CatalogChangeParams

func NewChange(params ChangeParams) (Change, error) {
	return NewCatalogChange(params)
}

func NewCatalogChange(params CatalogChangeParams) (CatalogChange, error) {
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	entityID, entityIDOK := normalizeCatalogID(params.EntityID)
	actorID, actorIDOK := normalizeOptionalCatalogID(params.ActorID)
	entityType := CatalogEntityType(normalizeCatalogEnum(params.EntityType))
	kind := CatalogChangeKind(normalizeCatalogEnum(params.Kind))
	if params.Sequence <= 0 || !catalogIDOK || !entityIDOK || !actorIDOK ||
		!isCatalogEntityType(entityType) || params.Revision <= 0 {
		return CatalogChange{}, ErrInvalidCatalogChange
	}
	switch kind {
	case CatalogChangeUpsert, CatalogChangeDelete:
	default:
		return CatalogChange{}, ErrInvalidCatalogChange
	}
	if params.OccurredAt.IsZero() {
		params.OccurredAt = time.Now().UTC()
	}
	return CatalogChange{
		Sequence: params.Sequence, CatalogID: catalogID, EntityType: entityType,
		EntityID: entityID, Kind: kind, Revision: params.Revision,
		ActorID: actorID, OccurredAt: params.OccurredAt.UTC(),
	}, nil
}

type Tombstone struct {
	Sequence   int64
	CatalogID  string
	EntityType CatalogEntityType
	EntityID   string
	Revision   int64
	DeletedAt  time.Time
	ExpiresAt  *time.Time
}

type TombstoneParams struct {
	Sequence   int64
	CatalogID  string
	EntityType string
	EntityID   string
	Revision   int64
	DeletedAt  time.Time
	ExpiresAt  *time.Time
}

func NewTombstone(params TombstoneParams) (Tombstone, error) {
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	entityID, entityIDOK := normalizeCatalogID(params.EntityID)
	entityType := CatalogEntityType(normalizeCatalogEnum(params.EntityType))
	if params.Sequence <= 0 || !catalogIDOK || !entityIDOK || !isCatalogEntityType(entityType) || params.Revision <= 0 {
		return Tombstone{}, ErrInvalidTombstone
	}
	if params.DeletedAt.IsZero() {
		params.DeletedAt = time.Now().UTC()
	}
	deletedAt := params.DeletedAt.UTC()
	expiresAt := normalizeOptionalCatalogTime(params.ExpiresAt)
	if expiresAt != nil && !expiresAt.After(deletedAt) {
		return Tombstone{}, ErrInvalidTombstone
	}
	return Tombstone{
		Sequence: params.Sequence, CatalogID: catalogID, EntityType: entityType,
		EntityID: entityID, Revision: params.Revision, DeletedAt: deletedAt, ExpiresAt: expiresAt,
	}, nil
}

func isCatalogEntityType(value CatalogEntityType) bool {
	switch value {
	case CatalogEntityCatalog, CatalogEntityItem, CatalogEntityItemAsset, CatalogEntityRepresentation,
		CatalogEntityMetadataEntry, CatalogEntityStorageRoot,
		CatalogEntityCollection, CatalogEntityCollectionItem, CatalogEntityTag, CatalogEntityItemTag,
		CatalogEntityUserState, CatalogEntityDeviceGrant:
		return true
	default:
		return false
	}
}

func normalizeOptionalCatalogID(value string) (string, bool) {
	if normalizeCatalogEnum(value) == "" {
		return "", true
	}
	return normalizeCatalogID(value)
}
