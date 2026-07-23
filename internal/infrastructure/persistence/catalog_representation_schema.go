package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// representationMetadataSchemaSQL expands the Catalog without modifying the
// legacy physical-file registry. Existing item assets are projected into
// Representation rows, and legacy JSON metadata is retained as provenance-
// marked MetadataEntry values. No library_files column, path, or document is
// updated by this migration.
const representationMetadataSchemaSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS library_catalog_items_id_catalog_idx
  ON library_catalog_items(id, catalog_id);
CREATE UNIQUE INDEX IF NOT EXISTS library_item_assets_id_item_idx
  ON library_item_assets(id, item_id);

CREATE TABLE IF NOT EXISTS library_representations (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  item_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN (
    'original','optimized','thumbnail','transcript','subtitle','artwork','preview','attachment'
  )),
  purpose TEXT NOT NULL CHECK (purpose IN (
    'primary','playback','preview','accessibility','artwork','attachment','indexing'
  )),
  media_type TEXT NOT NULL DEFAULT '',
  container TEXT NOT NULL DEFAULT '',
  codec TEXT NOT NULL DEFAULT '',
  width INTEGER CHECK (width IS NULL OR width > 0),
  height INTEGER CHECK (height IS NULL OR height > 0),
  duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
  bitrate_bps INTEGER CHECK (bitrate_bps IS NULL OR bitrate_bps > 0),
  language TEXT NOT NULL DEFAULT '',
  checksum_algorithm TEXT NOT NULL DEFAULT '' CHECK (checksum_algorithm IN ('','sha256','blake3')),
  checksum TEXT NOT NULL DEFAULT '',
  size_bytes INTEGER CHECK (size_bytes IS NULL OR size_bytes >= 0),
  availability TEXT NOT NULL CHECK (availability IN ('available','processing','offline','missing','corrupt')),
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  FOREIGN KEY (item_id, catalog_id) REFERENCES library_catalog_items(id, catalog_id) ON DELETE CASCADE,
  FOREIGN KEY (asset_id, item_id) REFERENCES library_item_assets(id, item_id) ON DELETE CASCADE,
  UNIQUE (item_id, asset_id, kind, purpose),
  CHECK (
    (checksum_algorithm = '' AND checksum = '') OR
    (checksum_algorithm IN ('sha256','blake3') AND length(checksum) = 64 AND checksum = lower(checksum))
  ),
  CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS library_representations_id_item_idx
  ON library_representations(id, item_id);
CREATE INDEX IF NOT EXISTS library_representations_browse_idx
  ON library_representations(catalog_id, item_id, kind, purpose, availability, id);
CREATE INDEX IF NOT EXISTS library_representations_asset_idx
  ON library_representations(asset_id, item_id);

CREATE TABLE IF NOT EXISTS library_metadata_entries (
  id TEXT PRIMARY KEY,
  catalog_id TEXT NOT NULL,
  item_id TEXT NOT NULL,
  representation_id TEXT,
  namespace TEXT NOT NULL,
  key TEXT NOT NULL,
  value_type TEXT NOT NULL CHECK (value_type IN (
    'string','integer','number','boolean','date','datetime','duration_ms','object','array','json'
  )),
  value_json TEXT NOT NULL CHECK (json_valid(value_json)),
  language TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
  source TEXT NOT NULL CHECK (source IN ('user','embedded','sidecar','remote','derived','migration','system')),
  provenance TEXT NOT NULL,
  confidence REAL CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
  locked BOOLEAN NOT NULL DEFAULT FALSE,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  FOREIGN KEY (item_id, catalog_id) REFERENCES library_catalog_items(id, catalog_id) ON DELETE CASCADE,
  FOREIGN KEY (representation_id, item_id) REFERENCES library_representations(id, item_id) ON DELETE CASCADE,
  CHECK (length(namespace) > 0 AND length(key) > 0 AND length(trim(provenance)) > 0),
  CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS library_metadata_entries_item_key_idx
  ON library_metadata_entries(item_id, namespace, key, language, position)
  WHERE representation_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS library_metadata_entries_representation_key_idx
  ON library_metadata_entries(representation_id, namespace, key, language, position)
  WHERE representation_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS library_metadata_entries_catalog_idx
  ON library_metadata_entries(catalog_id, item_id, namespace, key, position, id);
CREATE INDEX IF NOT EXISTS library_metadata_entries_source_idx
  ON library_metadata_entries(catalog_id, source, locked, updated_at DESC, id);

CREATE TRIGGER IF NOT EXISTS library_item_assets_create_representation
AFTER INSERT ON library_item_assets
WHEN EXISTS (SELECT 1 FROM library_catalog_items WHERE id = NEW.item_id)
 AND EXISTS (SELECT 1 FROM library_files WHERE id = NEW.file_id)
BEGIN
  INSERT OR IGNORE INTO library_representations (
    id, catalog_id, item_id, asset_id, kind, purpose,
    media_type, container, codec, width, height, duration_ms, bitrate_bps,
    language, checksum_algorithm, checksum, size_bytes,
    availability, revision, created_at, updated_at
  )
  SELECT
    NEW.id,
    items.catalog_id,
    NEW.item_id,
    NEW.id,
    CASE
      WHEN files.kind = 'thumbnail' THEN 'thumbnail'
      WHEN files.kind = 'transcode' THEN 'optimized'
      WHEN files.kind = 'subtitle' THEN 'subtitle'
      WHEN NEW.role = 'original' THEN 'original'
      WHEN NEW.role = 'artwork' THEN 'artwork'
      WHEN NEW.role = 'representation' THEN 'optimized'
      ELSE 'attachment'
    END,
    CASE
      WHEN files.kind = 'thumbnail' THEN 'preview'
      WHEN files.kind = 'transcode' THEN 'playback'
      WHEN files.kind = 'subtitle' THEN 'accessibility'
      WHEN NEW.role = 'original' THEN 'primary'
      WHEN NEW.role = 'artwork' THEN 'artwork'
      WHEN NEW.role = 'representation' THEN 'playback'
      ELSE 'attachment'
    END,
    '',
    CASE WHEN json_valid(files.media_json) THEN COALESCE(trim(json_extract(files.media_json, '$.Format')), '') ELSE '' END,
    CASE WHEN json_valid(files.media_json) THEN COALESCE(
      NULLIF(trim(json_extract(files.media_json, '$.Codec')), ''),
      NULLIF(trim(json_extract(files.media_json, '$.VideoCodec')), ''),
      NULLIF(trim(json_extract(files.media_json, '$.AudioCodec')), ''),
      ''
    ) ELSE '' END,
    CASE WHEN json_valid(files.media_json) AND CAST(json_extract(files.media_json, '$.Width') AS INTEGER) > 0
      THEN CAST(json_extract(files.media_json, '$.Width') AS INTEGER) END,
    CASE WHEN json_valid(files.media_json) AND CAST(json_extract(files.media_json, '$.Height') AS INTEGER) > 0
      THEN CAST(json_extract(files.media_json, '$.Height') AS INTEGER) END,
    CASE WHEN json_valid(files.media_json) AND CAST(json_extract(files.media_json, '$.DurationMs') AS INTEGER) >= 0
      THEN CAST(json_extract(files.media_json, '$.DurationMs') AS INTEGER) END,
    CASE WHEN json_valid(files.media_json) AND COALESCE(
        CAST(json_extract(files.media_json, '$.BitrateKbps') AS INTEGER),
        CAST(json_extract(files.media_json, '$.VideoBitrateKbps') AS INTEGER),
        CAST(json_extract(files.media_json, '$.AudioBitrateKbps') AS INTEGER)
      ) > 0
      THEN 1000 * COALESCE(
        CAST(json_extract(files.media_json, '$.BitrateKbps') AS INTEGER),
        CAST(json_extract(files.media_json, '$.VideoBitrateKbps') AS INTEGER),
        CAST(json_extract(files.media_json, '$.AudioBitrateKbps') AS INTEGER)
      ) END,
    '', '', '',
    CASE WHEN json_valid(files.media_json) AND CAST(json_extract(files.media_json, '$.SizeBytes') AS INTEGER) >= 0
      THEN CAST(json_extract(files.media_json, '$.SizeBytes') AS INTEGER) END,
    CASE
      WHEN items.status = 'missing' THEN 'missing'
      WHEN json_valid(files.state_json) AND COALESCE(json_extract(files.state_json, '$.Deleted'), 0) <> 0 THEN 'missing'
      ELSE 'available'
    END,
    1, NEW.created_at, NEW.updated_at
  FROM library_catalog_items AS items, library_files AS files
  WHERE items.id = NEW.item_id AND files.id = NEW.file_id;

  INSERT OR IGNORE INTO library_metadata_entries (
    id, catalog_id, item_id, representation_id, namespace, key,
    value_type, value_json, language, position, source, provenance,
    confidence, locked, revision, created_at, updated_at
  )
  SELECT
    lower(hex(randomblob(16))), items.catalog_id, NEW.item_id, NULL,
    'xiadown.legacy.item', 'metadata', 'json',
    CASE WHEN json_valid(items.metadata_json) THEN items.metadata_json ELSE json_quote(items.metadata_json) END,
    '', 0, 'migration', 'legacy.library_catalog_items.metadata_json',
    NULL, FALSE, 1, items.created_at, items.updated_at
  FROM library_catalog_items AS items
  WHERE items.id = NEW.item_id
    AND items.metadata_json IS NOT NULL
    AND trim(items.metadata_json) NOT IN ('', '{}', 'null');

  INSERT OR IGNORE INTO library_metadata_entries (
    id, catalog_id, item_id, representation_id, namespace, key,
    value_type, value_json, language, position, source, provenance,
    confidence, locked, revision, created_at, updated_at
  )
  SELECT
    lower(hex(randomblob(16))), items.catalog_id, NEW.item_id, NEW.id,
    'xiadown.legacy.file', 'metadata', 'json',
    CASE WHEN json_valid(files.metadata_json) THEN files.metadata_json ELSE json_quote(files.metadata_json) END,
    '', 0, 'migration', 'legacy.library_files.metadata_json',
    NULL, FALSE, 1, files.created_at, files.updated_at
  FROM library_catalog_items AS items, library_files AS files
  WHERE items.id = NEW.item_id AND files.id = NEW.file_id
    AND files.metadata_json IS NOT NULL
    AND trim(files.metadata_json) NOT IN ('', '{}', 'null');
END;

INSERT OR IGNORE INTO library_representations (
  id, catalog_id, item_id, asset_id, kind, purpose,
  media_type, container, codec, width, height, duration_ms, bitrate_bps,
  language, checksum_algorithm, checksum, size_bytes, availability,
  revision, created_at, updated_at
)
SELECT
  assets.id,
  items.catalog_id,
  assets.item_id,
  assets.id,
  CASE
    WHEN files.kind = 'thumbnail' THEN 'thumbnail'
    WHEN files.kind = 'transcode' THEN 'optimized'
    WHEN files.kind = 'subtitle' THEN 'subtitle'
    WHEN assets.role = 'original' THEN 'original'
    WHEN assets.role = 'artwork' THEN 'artwork'
    WHEN assets.role = 'representation' THEN 'optimized'
    ELSE 'attachment'
  END,
  CASE
    WHEN files.kind = 'thumbnail' THEN 'preview'
    WHEN files.kind = 'transcode' THEN 'playback'
    WHEN files.kind = 'subtitle' THEN 'accessibility'
    WHEN assets.role = 'original' THEN 'primary'
    WHEN assets.role = 'artwork' THEN 'artwork'
    WHEN assets.role = 'representation' THEN 'playback'
    ELSE 'attachment'
  END,
  '',
  CASE WHEN json_valid(files.media_json) THEN COALESCE(trim(json_extract(files.media_json, '$.Format')), '') ELSE '' END,
  CASE WHEN json_valid(files.media_json) THEN COALESCE(
    NULLIF(trim(json_extract(files.media_json, '$.Codec')), ''),
    NULLIF(trim(json_extract(files.media_json, '$.VideoCodec')), ''),
    NULLIF(trim(json_extract(files.media_json, '$.AudioCodec')), ''),
    ''
  ) ELSE '' END,
  CASE WHEN json_valid(files.media_json) AND CAST(json_extract(files.media_json, '$.Width') AS INTEGER) > 0
    THEN CAST(json_extract(files.media_json, '$.Width') AS INTEGER) END,
  CASE WHEN json_valid(files.media_json) AND CAST(json_extract(files.media_json, '$.Height') AS INTEGER) > 0
    THEN CAST(json_extract(files.media_json, '$.Height') AS INTEGER) END,
  CASE WHEN json_valid(files.media_json) AND CAST(json_extract(files.media_json, '$.DurationMs') AS INTEGER) >= 0
    THEN CAST(json_extract(files.media_json, '$.DurationMs') AS INTEGER) END,
  CASE WHEN json_valid(files.media_json) AND COALESCE(
      CAST(json_extract(files.media_json, '$.BitrateKbps') AS INTEGER),
      CAST(json_extract(files.media_json, '$.VideoBitrateKbps') AS INTEGER),
      CAST(json_extract(files.media_json, '$.AudioBitrateKbps') AS INTEGER)
    ) > 0
    THEN 1000 * COALESCE(
      CAST(json_extract(files.media_json, '$.BitrateKbps') AS INTEGER),
      CAST(json_extract(files.media_json, '$.VideoBitrateKbps') AS INTEGER),
      CAST(json_extract(files.media_json, '$.AudioBitrateKbps') AS INTEGER)
    ) END,
  '', '', '',
  CASE WHEN json_valid(files.media_json) AND CAST(json_extract(files.media_json, '$.SizeBytes') AS INTEGER) >= 0
    THEN CAST(json_extract(files.media_json, '$.SizeBytes') AS INTEGER) END,
  CASE
    WHEN items.status = 'missing' THEN 'missing'
    WHEN json_valid(files.state_json) AND COALESCE(json_extract(files.state_json, '$.Deleted'), 0) <> 0 THEN 'missing'
    ELSE 'available'
  END,
  1,
  assets.created_at,
  assets.updated_at
FROM library_item_assets AS assets
JOIN library_catalog_items AS items ON items.id = assets.item_id
JOIN library_files AS files ON files.id = assets.file_id;

INSERT OR IGNORE INTO library_metadata_entries (
  id, catalog_id, item_id, representation_id, namespace, key,
  value_type, value_json, language, position, source, provenance,
  confidence, locked, revision, created_at, updated_at
)
SELECT
  lower(hex(randomblob(16))), catalog_id, id, NULL,
  'xiadown.legacy.item', 'metadata', 'json',
  CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE json_quote(metadata_json) END,
  '', 0, 'migration', 'legacy.library_catalog_items.metadata_json',
  NULL, FALSE, 1, created_at, updated_at
FROM library_catalog_items
WHERE metadata_json IS NOT NULL
  AND trim(metadata_json) NOT IN ('', '{}', 'null');

INSERT OR IGNORE INTO library_metadata_entries (
  id, catalog_id, item_id, representation_id, namespace, key,
  value_type, value_json, language, position, source, provenance,
  confidence, locked, revision, created_at, updated_at
)
SELECT
  lower(hex(randomblob(16))), items.catalog_id, assets.item_id, representations.id,
  'xiadown.legacy.file', 'metadata', 'json',
  CASE WHEN json_valid(files.metadata_json) THEN files.metadata_json ELSE json_quote(files.metadata_json) END,
  '', 0, 'migration', 'legacy.library_files.metadata_json',
  NULL, FALSE, 1, files.created_at, files.updated_at
FROM library_item_assets AS assets
JOIN library_catalog_items AS items ON items.id = assets.item_id
JOIN library_files AS files ON files.id = assets.file_id
JOIN library_representations AS representations
  ON representations.asset_id = assets.id AND representations.item_id = assets.item_id
WHERE files.metadata_json IS NOT NULL
  AND trim(files.metadata_json) NOT IN ('', '{}', 'null');
`

// catalogChangeEntityExpansionSQL preserves every existing change and
// tombstone while widening the closed entity-type set. SQLite cannot ALTER a
// CHECK constraint, so the two small coordination tables are rebuilt inside
// the same transaction. Physical asset tables are never touched.
const catalogChangeEntityExpansionSQL = `
ALTER TABLE library_catalog_tombstones RENAME TO library_catalog_tombstones_v4;
ALTER TABLE library_catalog_changes RENAME TO library_catalog_changes_v4;
DROP INDEX library_catalog_changes_cursor_idx;
DROP INDEX library_catalog_tombstones_expiry_idx;

CREATE TABLE library_catalog_changes (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  catalog_id TEXT NOT NULL,
  entity_type TEXT NOT NULL CHECK (entity_type IN (
    'catalog','item','item_asset','representation','metadata_entry','storage_root','collection','collection_item',
    'tag','item_tag','user_state','device_grant'
  )),
  entity_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('upsert','delete')),
  revision INTEGER NOT NULL CHECK (revision > 0),
  actor_id TEXT NOT NULL DEFAULT '',
  occurred_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE
);

INSERT INTO library_catalog_changes (
  sequence, catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
)
SELECT sequence, catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
FROM library_catalog_changes_v4
ORDER BY sequence;

CREATE INDEX library_catalog_changes_cursor_idx
  ON library_catalog_changes(catalog_id, sequence);

CREATE TABLE library_catalog_tombstones (
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

INSERT INTO library_catalog_tombstones (
  sequence, catalog_id, entity_type, entity_id, revision, deleted_at, expires_at
)
SELECT sequence, catalog_id, entity_type, entity_id, revision, deleted_at, expires_at
FROM library_catalog_tombstones_v4;

CREATE INDEX library_catalog_tombstones_expiry_idx
  ON library_catalog_tombstones(expires_at)
  WHERE expires_at IS NOT NULL;

DROP TABLE library_catalog_tombstones_v4;
DROP TABLE library_catalog_changes_v4;
`

func applyRepresentationMetadataSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin representation metadata migration: %w", err)
	}
	defer tx.Rollback()
	if err := applyRepresentationMetadataSchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit representation metadata migration: %w", err)
	}
	return nil
}

func applyRepresentationMetadataSchemaTx(ctx context.Context, tx *sql.Tx) error {
	var changeCountBefore, tombstoneCountBefore int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_changes").Scan(&changeCountBefore); err != nil {
		return fmt.Errorf("count catalog changes before representation metadata migration: %w", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_tombstones").Scan(&tombstoneCountBefore); err != nil {
		return fmt.Errorf("count catalog tombstones before representation metadata migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, representationMetadataSchemaSQL); err != nil {
		return fmt.Errorf("apply representation metadata schema: %w", err)
	}
	if _, err := tx.ExecContext(ctx, catalogChangeEntityExpansionSQL); err != nil {
		return fmt.Errorf("expand catalog change entity types: %w", err)
	}

	var changeCountAfter, tombstoneCountAfter int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_changes").Scan(&changeCountAfter); err != nil {
		return fmt.Errorf("count catalog changes after representation metadata migration: %w", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_catalog_tombstones").Scan(&tombstoneCountAfter); err != nil {
		return fmt.Errorf("count catalog tombstones after representation metadata migration: %w", err)
	}
	if changeCountAfter != changeCountBefore || tombstoneCountAfter != tombstoneCountBefore {
		return fmt.Errorf(
			"representation metadata migration changed coordination row counts: changes %d->%d, tombstones %d->%d",
			changeCountBefore, changeCountAfter, tombstoneCountBefore, tombstoneCountAfter,
		)
	}
	foreignKeyRows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("check representation metadata foreign keys: %w", err)
	}
	hasForeignKeyViolation := foreignKeyRows.Next()
	if closeErr := foreignKeyRows.Close(); closeErr != nil {
		return fmt.Errorf("close representation metadata foreign key check: %w", closeErr)
	}
	if hasForeignKeyViolation {
		return fmt.Errorf("representation metadata migration introduced a foreign-key violation")
	}
	return nil
}
