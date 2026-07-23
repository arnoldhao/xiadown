package library

import "errors"

var (
	ErrCatalogRevisionConflict    = errors.New("catalog revision conflict")
	ErrInvalidCatalog             = errors.New("invalid catalog")
	ErrInvalidCatalogItem         = errors.New("invalid catalog item")
	ErrInvalidItemAsset           = errors.New("invalid catalog item asset")
	ErrInvalidRepresentation      = errors.New("invalid catalog representation")
	ErrInvalidMetadataEntry       = errors.New("invalid catalog metadata entry")
	ErrInvalidStorageRoot         = errors.New("invalid catalog storage root")
	ErrInvalidCollection          = errors.New("invalid catalog collection")
	ErrInvalidCollectionItem      = errors.New("invalid catalog collection item")
	ErrInvalidTag                 = errors.New("invalid catalog tag")
	ErrInvalidItemTag             = errors.New("invalid catalog item tag")
	ErrInvalidUserState           = errors.New("invalid catalog user state")
	ErrInvalidDeviceGrant         = errors.New("invalid catalog device grant")
	ErrInvalidCatalogChange       = errors.New("invalid catalog change")
	ErrInvalidCatalogSyncState    = errors.New("invalid catalog sync state")
	ErrInvalidTombstone           = errors.New("invalid catalog tombstone")
	ErrInvalidLegacyMapping       = errors.New("invalid catalog legacy mapping")
	ErrInvalidMigrationCheckpoint = errors.New("invalid catalog migration checkpoint")
)
