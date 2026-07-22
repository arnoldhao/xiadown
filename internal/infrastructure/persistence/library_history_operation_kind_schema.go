package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const libraryHistoryOperationKindSchemaSQL = `
ALTER TABLE library_history_records ADD COLUMN operation_kind TEXT;

UPDATE library_history_records
SET operation_kind = action
WHERE category = 'operation'
  AND (operation_kind IS NULL OR trim(operation_kind) = '');
`

func applyLibraryHistoryOperationKindSchema(ctx context.Context, db *sql.DB) error {
	present, err := sqliteTableHasColumn(ctx, db, "library_history_records", "operation_kind")
	if err != nil {
		return fmt.Errorf("inspect Library history operation kind: %w", err)
	}
	if !present {
		if _, err := db.ExecContext(ctx, `ALTER TABLE library_history_records ADD COLUMN operation_kind TEXT`); err != nil {
			return fmt.Errorf("add Library history operation kind: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, libraryHistoryOperationKindBackfillSQL); err != nil {
		return fmt.Errorf("backfill Library history operation kind: %w", err)
	}
	return nil
}

const libraryHistoryOperationKindBackfillSQL = `
UPDATE library_history_records
SET operation_kind = action
WHERE category = 'operation'
  AND (operation_kind IS NULL OR trim(operation_kind) = '');
`

func applyLibraryHistoryOperationKindSchemaTx(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(library_history_records)`)
	if err != nil {
		return fmt.Errorf("inspect Library history operation kind: %w", err)
	}
	present := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		if name == "operation_kind" {
			present = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !present {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE library_history_records ADD COLUMN operation_kind TEXT`); err != nil {
			return fmt.Errorf("add Library history operation kind: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, libraryHistoryOperationKindBackfillSQL); err != nil {
		return fmt.Errorf("backfill Library history operation kind: %w", err)
	}
	return nil
}
