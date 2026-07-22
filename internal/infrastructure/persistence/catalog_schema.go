package persistence

import (
	"context"
	"database/sql"
)

// catalogSchemaSQL is additive by design. The legacy library_libraries table
// remains the download/import bundle boundary and library_files remains the
// physical asset registry. The catalog tables add the durable user-facing
// organization layer without changing existing file IDs or paths.
const catalogSchemaSQL = `
CREATE TABLE IF NOT EXISTS library_catalogs (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('active','archived')),
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  CHECK (length(trim(name)) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS library_catalogs_one_default_idx
  ON library_catalogs(is_default)
  WHERE is_default = TRUE;

CREATE TABLE IF NOT EXISTS library_catalog_items (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  category TEXT NOT NULL CHECK (category IN ('video','audio','book','image','other')),
  status TEXT NOT NULL CHECK (status IN ('active','needs_review','missing','trashed')),
  title TEXT NOT NULL,
  sort_title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  subtype TEXT NOT NULL DEFAULT '',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  trashed_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  CHECK (length(trim(title)) > 0),
  CHECK (
    (status = 'trashed' AND trashed_at IS NOT NULL) OR
    (status != 'trashed' AND trashed_at IS NULL)
  )
);

CREATE INDEX IF NOT EXISTS library_catalog_items_browse_idx
  ON library_catalog_items(catalog_id, status, category, updated_at DESC, id);
CREATE INDEX IF NOT EXISTS library_catalog_items_sort_title_idx
  ON library_catalog_items(catalog_id, category, sort_title COLLATE NOCASE, id);

CREATE TABLE IF NOT EXISTS library_item_assets (
  id TEXT PRIMARY KEY,
  item_id TEXT NOT NULL,
  file_id TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('original','representation','attachment','artwork')),
  label TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (item_id) REFERENCES library_catalog_items(id) ON DELETE CASCADE,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE RESTRICT,
  UNIQUE (item_id, file_id),
  UNIQUE (item_id, role, position)
);

CREATE INDEX IF NOT EXISTS library_item_assets_file_idx
  ON library_item_assets(file_id, item_id);

CREATE TABLE IF NOT EXISTS library_storage_roots (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  volume_id TEXT NOT NULL DEFAULT '',
  mode TEXT NOT NULL CHECK (mode IN ('managed','referenced')),
  status TEXT NOT NULL CHECK (status IN ('online','offline','read_only','error')),
  last_checked_at TIMESTAMP,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  UNIQUE (catalog_id, path),
  CHECK ((status = 'error' AND length(trim(last_error)) > 0) OR
         (status != 'error' AND last_error = ''))
);

CREATE INDEX IF NOT EXISTS library_storage_roots_status_idx
  ON library_storage_roots(catalog_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS library_collections (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('manual','smart','playlist','album','shelf','series')),
  smart_query TEXT NOT NULL DEFAULT '',
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  CHECK ((kind = 'smart' AND length(trim(smart_query)) > 0) OR
         (kind != 'smart' AND smart_query = ''))
);

CREATE INDEX IF NOT EXISTS library_collections_browse_idx
  ON library_collections(catalog_id, kind, updated_at DESC, name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS library_collection_items (
  id TEXT PRIMARY KEY,
  collection_id TEXT NOT NULL,
  item_id TEXT NOT NULL,
  position INTEGER NOT NULL CHECK (position >= 0),
  added_at TIMESTAMP NOT NULL,
  FOREIGN KEY (collection_id) REFERENCES library_collections(id) ON DELETE CASCADE,
  FOREIGN KEY (item_id) REFERENCES library_catalog_items(id) ON DELETE CASCADE,
  UNIQUE (collection_id, item_id),
  UNIQUE (collection_id, position)
);

CREATE TABLE IF NOT EXISTS library_tags (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  name TEXT NOT NULL,
  normalized_name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  UNIQUE (catalog_id, normalized_name)
);

CREATE TABLE IF NOT EXISTS library_item_tags (
  id TEXT PRIMARY KEY,
  item_id TEXT NOT NULL,
  tag_id TEXT NOT NULL,
  added_by TEXT NOT NULL DEFAULT '',
  added_at TIMESTAMP NOT NULL,
  FOREIGN KEY (item_id) REFERENCES library_catalog_items(id) ON DELETE CASCADE,
  FOREIGN KEY (tag_id) REFERENCES library_tags(id) ON DELETE CASCADE,
  UNIQUE (item_id, tag_id)
);

CREATE TABLE IF NOT EXISTS library_user_states (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  item_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  favorite BOOLEAN NOT NULL DEFAULT FALSE,
  rating INTEGER NOT NULL DEFAULT 0 CHECK (rating BETWEEN 0 AND 5),
  progress REAL NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 1),
  position_ms INTEGER NOT NULL DEFAULT 0 CHECK (position_ms >= 0),
  locator TEXT NOT NULL DEFAULT '',
  completed BOOLEAN NOT NULL DEFAULT FALSE,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  last_opened_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  FOREIGN KEY (item_id) REFERENCES library_catalog_items(id) ON DELETE CASCADE,
  UNIQUE (catalog_id, item_id, user_id),
  CHECK (completed = FALSE OR progress = 1)
);

CREATE INDEX IF NOT EXISTS library_user_states_recent_idx
  ON library_user_states(catalog_id, user_id, last_opened_at DESC);

CREATE TABLE IF NOT EXISTS library_device_grants (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  device_name TEXT NOT NULL,
  credential_hash TEXT NOT NULL,
  public_key_hash TEXT NOT NULL,
  scopes_json TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('active','revoked')),
  expires_at TIMESTAMP,
  last_seen_at TIMESTAMP,
  revoked_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  UNIQUE (catalog_id, device_id),
  UNIQUE (credential_hash),
  CHECK ((status = 'active' AND revoked_at IS NULL) OR
         (status = 'revoked' AND revoked_at IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS library_catalog_changes (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  catalog_id TEXT NOT NULL,
  entity_type TEXT NOT NULL CHECK (entity_type IN (
    'catalog','item','item_asset','storage_root','collection','collection_item',
    'tag','item_tag','user_state','device_grant'
  )),
  entity_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('upsert','delete')),
  revision INTEGER NOT NULL CHECK (revision > 0),
  actor_id TEXT NOT NULL DEFAULT '',
  occurred_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS library_catalog_changes_cursor_idx
  ON library_catalog_changes(catalog_id, sequence);

CREATE TABLE IF NOT EXISTS library_catalog_tombstones (
  sequence INTEGER PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK (revision > 0),
  deleted_at TIMESTAMP NOT NULL,
  expires_at TIMESTAMP,
  FOREIGN KEY (sequence) REFERENCES library_catalog_changes(sequence) ON DELETE CASCADE,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  UNIQUE (catalog_id, entity_type, entity_id),
  CHECK (expires_at IS NULL OR expires_at > deleted_at)
);

CREATE INDEX IF NOT EXISTS library_catalog_tombstones_expiry_idx
  ON library_catalog_tombstones(expires_at)
  WHERE expires_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS library_legacy_mappings (
  migration_id TEXT NOT NULL,
  catalog_id TEXT NOT NULL,
  source_type TEXT NOT NULL CHECK (source_type IN (
    'legacy_library','library_file','listen_track','listen_playlist',
    'listen_playlist_item','operation','workspace_state'
  )),
  source_id TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  source_fingerprint TEXT NOT NULL DEFAULT '',
  migrated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (migration_id, source_type, source_id),
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS library_legacy_mappings_target_idx
  ON library_legacy_mappings(catalog_id, target_type, target_id);

CREATE TABLE IF NOT EXISTS library_migration_checkpoints (
  migration_id TEXT NOT NULL,
  catalog_id TEXT NOT NULL,
  phase TEXT NOT NULL CHECK (phase IN (
    'preflight','expand','backfill','shadow_read','cutover','stabilize'
  )),
  status TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed')),
  cursor TEXT NOT NULL DEFAULT '',
  processed INTEGER NOT NULL DEFAULT 0 CHECK (processed >= 0),
  failed INTEGER NOT NULL DEFAULT 0 CHECK (failed >= 0 AND failed <= processed),
  last_error TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (migration_id, phase),
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE
);
`

func applyCatalogSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, catalogSchemaSQL); err != nil {
		return err
	}
	return tx.Commit()
}
