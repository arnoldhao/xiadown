package libraryrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type SQLiteCatalogSyncStateRepository struct{ db *bun.DB }

func NewSQLiteCatalogSyncStateRepository(db *bun.DB) *SQLiteCatalogSyncStateRepository {
	return &SQLiteCatalogSyncStateRepository{db: db}
}

func (repo *SQLiteCatalogSyncStateRepository) GetCatalogSyncState(
	ctx context.Context,
	catalogID string,
) (library.CatalogSyncState, error) {
	if repo == nil || repo.db == nil || strings.TrimSpace(catalogID) == "" {
		return library.CatalogSyncState{}, library.ErrInvalidCatalogSyncState
	}
	var epoch string
	var rotatedAt time.Time
	var cursor int64
	err := repo.db.QueryRowContext(ctx, `
SELECT state.epoch, state.rotated_at,
       COALESCE(MAX(changes.sequence), 0) AS cursor
FROM library_catalog_sync_state AS state
LEFT JOIN library_catalog_changes AS changes
  ON changes.catalog_id = state.catalog_id
WHERE state.catalog_id = ?
GROUP BY state.catalog_id, state.epoch, state.rotated_at
`, strings.TrimSpace(catalogID)).Scan(&epoch, &rotatedAt, &cursor)
	if err != nil {
		if err == sql.ErrNoRows {
			return library.CatalogSyncState{}, err
		}
		return library.CatalogSyncState{}, fmt.Errorf("read Catalog sync state: %w", err)
	}
	return library.NewCatalogSyncState(library.CatalogSyncStateParams{
		CatalogID: catalogID, Epoch: epoch, Cursor: cursor, RotatedAt: rotatedAt,
	})
}

var _ library.CatalogSyncStateRepository = (*SQLiteCatalogSyncStateRepository)(nil)
