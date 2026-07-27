package library

import "time"

// ItemAvailability describes whether at least one primary physical
// representation of a logical Catalog item can be used right now. It is kept
// separate from ItemStatus: an active item may be temporarily offline because
// its volume is not mounted, while a missing item remains part of its original
// media category.
type ItemAvailability string

const (
	ItemAvailabilityAvailable ItemAvailability = "available"
	ItemAvailabilityChecking  ItemAvailability = "checking"
	ItemAvailabilityOffline   ItemAvailability = "offline"
	ItemAvailabilityMissing   ItemAvailability = "missing"
	ItemAvailabilityError     ItemAvailability = "error"
)

// CatalogItemPresentationAsset is the bounded read model used to build
// Library cards and companion availability. SQLite can materialize all assets
// for one page in a single query, avoiding per-card filesystem probes and N+1
// repository calls.
type CatalogItemPresentationAsset struct {
	ItemID          string
	AssetID         string
	FileID          string
	Role            ItemAssetRole
	Position        int
	Kind            FileKind
	LocalPath       string
	FileState       FileState
	Media           *MediaInfo
	FileUpdatedAt   time.Time
	StorageRootID   string
	StorageRootMode StorageRootMode
	RootStatus      StorageRootStatus
	SyncEntryStatus string
	SyncStateStatus string
	PreviewArtwork  bool
}
