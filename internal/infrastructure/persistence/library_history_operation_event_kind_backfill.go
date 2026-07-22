package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// v23 deliberately follows the already-defined v22 column migration instead
// of changing v22's checksum. Existing desktop databases may have applied v22
// before this legacy operation-event backfill was added.
const libraryHistoryOperationEventKindBackfillSQL = `
UPDATE library_history_records AS event
SET operation_kind = (
  SELECT primary_history.action
  FROM library_history_records AS primary_history
  WHERE primary_history.library_id = event.library_id
    AND primary_history.category = 'operation'
    AND trim(primary_history.action) <> ''
    AND COALESCE(
      NULLIF(trim(primary_history.subject_operation_id), ''),
      NULLIF(trim(primary_history.operation_id), '')
    ) = COALESCE(
      NULLIF(trim(event.subject_operation_id), ''),
      NULLIF(trim(event.operation_id), '')
    )
  ORDER BY primary_history.updated_at DESC, primary_history.id ASC
  LIMIT 1
)
WHERE event.category = 'operation_event'
  AND (event.operation_kind IS NULL OR trim(event.operation_kind) = '')
  AND COALESCE(
    NULLIF(trim(event.subject_operation_id), ''),
    NULLIF(trim(event.operation_id), '')
  ) IS NOT NULL
  AND EXISTS (
    SELECT 1
    FROM library_history_records AS primary_history
    WHERE primary_history.library_id = event.library_id
      AND primary_history.category = 'operation'
      AND trim(primary_history.action) <> ''
      AND COALESCE(
        NULLIF(trim(primary_history.subject_operation_id), ''),
        NULLIF(trim(primary_history.operation_id), '')
      ) = COALESCE(
        NULLIF(trim(event.subject_operation_id), ''),
        NULLIF(trim(event.operation_id), '')
      )
  );
`

func applyLibraryHistoryOperationEventKindBackfill(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, libraryHistoryOperationEventKindBackfillSQL); err != nil {
		return fmt.Errorf("backfill Library operation-event kind: %w", err)
	}
	return nil
}
