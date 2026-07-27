package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const sqliteUUIDGlob = "[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]-" +
	"[0-9a-f][0-9a-f][0-9a-f][0-9a-f]-" +
	"[0-9a-f][0-9a-f][0-9a-f][0-9a-f]-" +
	"[0-9a-f][0-9a-f][0-9a-f][0-9a-f]-" +
	"[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]"

const libraryTransientRootImportCleanupSQL = `
CREATE TEMP TABLE IF NOT EXISTS xiadown_transient_root_import_files (
  file_id TEXT PRIMARY KEY
);
DELETE FROM xiadown_transient_root_import_files;

INSERT INTO xiadown_transient_root_import_files (file_id)
SELECT DISTINCT files.id
FROM library_files AS files
JOIN library_storage_root_sync_entries AS entries
  ON entries.file_id = files.id
WHERE files.origin_kind = 'import'
  AND entries.status = 'missing'
  AND (
    lower(entries.relative_path) GLOB '*.` + sqliteUUIDGlob + `.tmp.*'
    OR lower(entries.relative_path) GLOB '*.` + sqliteUUIDGlob + `.tmp'
  );

CREATE TEMP TABLE IF NOT EXISTS xiadown_transient_root_import_items (
  item_id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  next_revision INTEGER NOT NULL
);
DELETE FROM xiadown_transient_root_import_items;

INSERT INTO xiadown_transient_root_import_items (
  item_id,
  catalog_id,
  next_revision
)
SELECT DISTINCT items.id, items.catalog_id, items.revision + 1
FROM library_catalog_items AS items
JOIN library_item_assets AS assets
  ON assets.item_id = items.id
JOIN xiadown_transient_root_import_files AS candidates
  ON candidates.file_id = assets.file_id
WHERE items.status <> 'trashed'
  AND NOT EXISTS (
    SELECT 1
    FROM library_item_assets AS healthy_assets
    WHERE healthy_assets.item_id = items.id
      AND healthy_assets.role IN ('original', 'representation')
      AND healthy_assets.file_id NOT IN (
        SELECT file_id FROM xiadown_transient_root_import_files
      )
  );

UPDATE library_files
SET state_json = json_set(
      CASE WHEN json_valid(state_json) THEN state_json ELSE '{}' END,
      '$.Status', 'purged',
      '$.Deleted', json('true'),
      '$.Archived', json('false'),
      '$.LastError', ''
    ),
    updated_at = CURRENT_TIMESTAMP
WHERE id IN (SELECT file_id FROM xiadown_transient_root_import_files);

DELETE FROM listen_local_tracks
WHERE file_id IN (SELECT file_id FROM xiadown_transient_root_import_files);

UPDATE library_storage_root_sync_states
SET missing_count = MAX(
      0,
      missing_count - (
        SELECT COUNT(*)
        FROM library_storage_root_sync_entries AS entries
        JOIN xiadown_transient_root_import_files AS candidates
          ON candidates.file_id = entries.file_id
        WHERE entries.root_id = library_storage_root_sync_states.root_id
          AND entries.status = 'missing'
      )
    ),
    updated_at = CURRENT_TIMESTAMP
WHERE EXISTS (
  SELECT 1
  FROM library_storage_root_sync_entries AS entries
  JOIN xiadown_transient_root_import_files AS candidates
    ON candidates.file_id = entries.file_id
  WHERE entries.root_id = library_storage_root_sync_states.root_id
);

DELETE FROM library_storage_root_sync_entries
WHERE file_id IN (SELECT file_id FROM xiadown_transient_root_import_files);

UPDATE library_catalog_items
SET status = 'trashed',
    revision = (
      SELECT next_revision
      FROM xiadown_transient_root_import_items AS candidates
      WHERE candidates.item_id = library_catalog_items.id
    ),
    trashed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id IN (SELECT item_id FROM xiadown_transient_root_import_items);

INSERT INTO library_catalog_changes (
  catalog_id,
  entity_type,
  entity_id,
  kind,
  revision,
  actor_id,
  occurred_at
)
SELECT catalog_id, 'item', item_id, 'delete', next_revision, '', CURRENT_TIMESTAMP
FROM xiadown_transient_root_import_items;

DROP TABLE xiadown_transient_root_import_items;
DROP TABLE xiadown_transient_root_import_files;
`

func applyLibraryTransientRootImportCleanup(
	ctx context.Context,
	db *sql.DB,
) error {
	_, err := db.ExecContext(ctx, libraryTransientRootImportCleanupSQL)
	if err != nil {
		return fmt.Errorf("clean transient Library root imports: %w", err)
	}
	return nil
}

func applyLibraryTransientRootImportCleanupTx(
	ctx context.Context,
	tx *sql.Tx,
) error {
	_, err := tx.ExecContext(ctx, libraryTransientRootImportCleanupSQL)
	if err != nil {
		return fmt.Errorf("clean transient Library root imports: %w", err)
	}
	return nil
}
