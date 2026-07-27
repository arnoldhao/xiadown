package persistence

import (
	"context"
	"database/sql"
)

const libraryRootSyncAvailabilityIndexSQL = `
CREATE INDEX IF NOT EXISTS library_storage_root_sync_entries_file_idx
  ON library_storage_root_sync_entries(file_id, root_id, status)
  WHERE file_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS library_storage_root_sync_entries_status_idx
  ON library_storage_root_sync_entries(root_id, status, relative_path);

CREATE INDEX IF NOT EXISTS library_catalog_items_runtime_browse_idx
  ON library_catalog_items(catalog_id, updated_at DESC, id)
  WHERE status <> 'trashed' AND trashed_at IS NULL;

CREATE INDEX IF NOT EXISTS library_catalog_items_category_runtime_browse_idx
  ON library_catalog_items(catalog_id, category, updated_at DESC, id)
  WHERE status <> 'trashed' AND trashed_at IS NULL;
`

func applyLibraryRootSyncAvailabilityIndex(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, libraryRootSyncAvailabilityIndexSQL)
	return err
}
