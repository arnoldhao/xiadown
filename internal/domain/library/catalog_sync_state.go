package library

import (
	"strings"
	"time"
)

const CatalogSyncEpochLength = 32

// CatalogSyncState identifies one generation of the incremental Catalog feed.
// Cursor values are meaningful only while Epoch matches. A logical restore
// rotates the epoch so a remote client can never mistake an older Catalog
// snapshot for an incremental continuation of its local cache.
type CatalogSyncState struct {
	CatalogID string
	Epoch     string
	Cursor    int64
	RotatedAt time.Time
}

type CatalogSyncStateParams struct {
	CatalogID string
	Epoch     string
	Cursor    int64
	RotatedAt time.Time
}

func NewCatalogSyncState(params CatalogSyncStateParams) (CatalogSyncState, error) {
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	epoch := strings.TrimSpace(params.Epoch)
	if !catalogIDOK || !validCatalogSyncEpoch(epoch) || params.Cursor < 0 || params.RotatedAt.IsZero() {
		return CatalogSyncState{}, ErrInvalidCatalogSyncState
	}
	return CatalogSyncState{
		CatalogID: catalogID,
		Epoch:     epoch,
		Cursor:    params.Cursor,
		RotatedAt: params.RotatedAt.UTC(),
	}, nil
}

func validCatalogSyncEpoch(value string) bool {
	if len(value) != CatalogSyncEpochLength {
		return false
	}
	for _, character := range value {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') {
			continue
		}
		return false
	}
	return true
}
