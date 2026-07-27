package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

const libraryStorageRootEmojiSchemaSQL = `
ALTER TABLE library_storage_roots
  ADD COLUMN emoji TEXT NOT NULL DEFAULT '';

UPDATE library_storage_roots
SET emoji = CASE ABS(RANDOM()) % 16
  WHEN 0 THEN '📁'
  WHEN 1 THEN '🗂️'
  WHEN 2 THEN '💾'
  WHEN 3 THEN '🎬'
  WHEN 4 THEN '🎵'
  WHEN 5 THEN '📚'
  WHEN 6 THEN '🖼️'
  WHEN 7 THEN '🌈'
  WHEN 8 THEN '🚀'
  WHEN 9 THEN '⭐'
  WHEN 10 THEN '🌙'
  WHEN 11 THEN '☁️'
  WHEN 12 THEN '🧰'
  WHEN 13 THEN '🗄️'
  WHEN 14 THEN '📦'
  ELSE '💿'
END
WHERE TRIM(emoji) = '';
`

func applyLibraryStorageRootEmojiSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Library storage root emoji migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyLibraryStorageRootEmojiSchemaTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Library storage root emoji migration: %w", err)
	}
	return nil
}

func applyLibraryStorageRootEmojiSchemaTx(ctx context.Context, tx *sql.Tx) error {
	var exists int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM pragma_table_info('library_storage_roots') WHERE name = 'emoji'`,
	).Scan(&exists); err != nil {
		return fmt.Errorf("inspect library_storage_roots.emoji: %w", err)
	}
	if exists == 0 {
		if _, err := tx.ExecContext(
			ctx,
			`ALTER TABLE library_storage_roots ADD COLUMN emoji TEXT NOT NULL DEFAULT ''`,
		); err != nil {
			return fmt.Errorf("add library_storage_roots.emoji: %w", err)
		}
	}
	const backfillEmoji = `
UPDATE library_storage_roots
SET emoji = CASE ABS(RANDOM()) % 16
  WHEN 0 THEN '📁'
  WHEN 1 THEN '🗂️'
  WHEN 2 THEN '💾'
  WHEN 3 THEN '🎬'
  WHEN 4 THEN '🎵'
  WHEN 5 THEN '📚'
  WHEN 6 THEN '🖼️'
  WHEN 7 THEN '🌈'
  WHEN 8 THEN '🚀'
  WHEN 9 THEN '⭐'
  WHEN 10 THEN '🌙'
  WHEN 11 THEN '☁️'
  WHEN 12 THEN '🧰'
  WHEN 13 THEN '🗄️'
  WHEN 14 THEN '📦'
  ELSE '💿'
END
WHERE TRIM(emoji) = '';
`
	if _, err := tx.ExecContext(ctx, backfillEmoji); err != nil {
		return fmt.Errorf("backfill Library storage root emoji: %w", err)
	}
	return nil
}
