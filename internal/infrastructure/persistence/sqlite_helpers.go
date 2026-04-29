package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
)

func deriveLibraryStoredName(localPath string, fallback string) string {
	trimmedPath := strings.TrimSpace(localPath)
	if trimmedPath == "" {
		return strings.TrimSpace(fallback)
	}
	base := strings.TrimSpace(filepath.Base(trimmedPath))
	if base == "" || base == "." {
		return strings.TrimSpace(fallback)
	}
	stem := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if stem != "" {
		return stem
	}
	return base
}

func sqliteTableHasColumn(ctx context.Context, db *sql.DB, table string, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func sqliteTableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	row := db.QueryRowContext(ctx, "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?", table)
	var exists int
	if err := row.Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func createMemoryChunksFTSTable(ctx context.Context, db *sql.DB) error {
	const createFTS = `
CREATE VIRTUAL TABLE IF NOT EXISTS memory_chunks_fts USING fts5(
	content,
	assistant_id UNINDEXED,
	file_path UNINDEXED,
	line_start UNINDEXED,
	line_end UNINDEXED,
	chunk_id UNINDEXED
);
`
	if _, err := db.ExecContext(ctx, createFTS); err != nil {
		return fmt.Errorf("create memory_chunks_fts table: %w", err)
	}
	return nil
}
