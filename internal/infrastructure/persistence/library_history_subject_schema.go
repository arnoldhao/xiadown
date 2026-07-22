package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const libraryHistoryStableOperationSubjectSQL = `
ALTER TABLE library_history_records ADD COLUMN subject_operation_id TEXT;

UPDATE library_history_records
SET subject_operation_id = operation_id
WHERE operation_id IS NOT NULL AND trim(operation_id) <> '';
`

func applyLibraryHistoryStableOperationSubject(ctx context.Context, db *sql.DB) error {
	present, err := sqliteTableHasColumn(ctx, db, "library_history_records", "subject_operation_id")
	if err != nil {
		return fmt.Errorf("inspect Library history operation subject: %w", err)
	}
	if !present {
		if _, err := db.ExecContext(ctx, `ALTER TABLE library_history_records ADD COLUMN subject_operation_id TEXT`); err != nil {
			return fmt.Errorf("add stable Library history operation subject: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `
UPDATE library_history_records
SET subject_operation_id = operation_id
WHERE subject_operation_id IS NULL
  AND operation_id IS NOT NULL
  AND trim(operation_id) <> ''
`); err != nil {
		return fmt.Errorf("add stable Library history operation subject: %w", err)
	}
	return nil
}

func applyLibraryHistoryStableOperationSubjectTx(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(library_history_records)`)
	if err != nil {
		return fmt.Errorf("inspect Library history operation subject: %w", err)
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
		if name == "subject_operation_id" {
			present = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !present {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE library_history_records ADD COLUMN subject_operation_id TEXT`); err != nil {
			return fmt.Errorf("add stable Library history operation subject: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE library_history_records
SET subject_operation_id = operation_id
WHERE subject_operation_id IS NULL
  AND operation_id IS NOT NULL
  AND trim(operation_id) <> ''
`); err != nil {
		return fmt.Errorf("backfill stable Library history operation subject: %w", err)
	}
	return nil
}
