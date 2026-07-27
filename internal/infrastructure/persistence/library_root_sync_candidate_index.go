package persistence

import (
	"context"
	"database/sql"
)

const libraryRootSyncCandidateIndexSQL = `
CREATE INDEX IF NOT EXISTS library_storage_root_sync_entries_size_idx
  ON library_storage_root_sync_entries(root_id, size_bytes, relative_path)
  WHERE status = 'active';
`

func applyLibraryRootSyncCandidateIndex(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, libraryRootSyncCandidateIndexSQL)
	return err
}
