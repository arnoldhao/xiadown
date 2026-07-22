package persistence

import (
	"context"
	"database/sql"
)

// libraryCatalogSyncStateSchemaSQL deliberately lives outside the restorable
// Catalog graph. A backup contains this table because it snapshots the whole
// private database, but logical restore never copies it and always rotates the
// current epoch after installing restored Catalog content.
const libraryCatalogSyncStateSchemaSQL = `
CREATE TABLE IF NOT EXISTS library_catalog_sync_state (
  catalog_id TEXT PRIMARY KEY,
  epoch TEXT NOT NULL UNIQUE,
  rotated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (catalog_id) REFERENCES library_catalogs(id) ON DELETE CASCADE,
  CHECK (length(epoch) = 32 AND epoch NOT GLOB '*[^0-9a-f]*')
);

INSERT INTO library_catalog_sync_state (catalog_id, epoch, rotated_at)
SELECT catalog.id, lower(hex(randomblob(16))), CURRENT_TIMESTAMP
FROM library_catalogs AS catalog
WHERE NOT EXISTS (
  SELECT 1 FROM library_catalog_sync_state AS state
  WHERE state.catalog_id = catalog.id
);

CREATE TRIGGER IF NOT EXISTS library_catalog_sync_state_after_catalog_insert
AFTER INSERT ON library_catalogs
FOR EACH ROW
BEGIN
  INSERT INTO library_catalog_sync_state (catalog_id, epoch, rotated_at)
  VALUES (NEW.id, lower(hex(randomblob(16))), CURRENT_TIMESTAMP);
END;
`

func applyLibraryCatalogSyncStateSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, libraryCatalogSyncStateSchemaSQL)
	return err
}
