package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

// Keep this value synchronized with maxRSSCollectionItems in the application
// layer. The trigger is deliberately a database invariant as collection member
// tables can also be written by restore, migration, or future sync paths.
const rssCollectionLimitsSchemaSQL = `
CREATE TRIGGER rss_collection_subscriptions_max_items
BEFORE INSERT ON rss_collection_subscriptions
WHEN NOT EXISTS (
  SELECT 1 FROM rss_collection_subscriptions
  WHERE collection_id = NEW.collection_id AND subscription_id = NEW.subscription_id
) AND (
  SELECT COUNT(*) FROM rss_collection_subscriptions
  WHERE collection_id = NEW.collection_id
) >= 10000
BEGIN
  SELECT RAISE(ABORT, 'rss collection item limit exceeded');
END;

CREATE TRIGGER rss_collection_subscriptions_max_items_update
BEFORE UPDATE OF collection_id ON rss_collection_subscriptions
WHEN OLD.collection_id <> NEW.collection_id AND (
  SELECT COUNT(*) FROM rss_collection_subscriptions
  WHERE collection_id = NEW.collection_id
) >= 10000
BEGIN
  SELECT RAISE(ABORT, 'rss collection item limit exceeded');
END;

CREATE TRIGGER rss_collection_entries_max_items
BEFORE INSERT ON rss_collection_entries
WHEN NOT EXISTS (
  SELECT 1 FROM rss_collection_entries
  WHERE collection_id = NEW.collection_id AND entry_id = NEW.entry_id
) AND (
  SELECT COUNT(*) FROM rss_collection_entries
  WHERE collection_id = NEW.collection_id
) >= 10000
BEGIN
  SELECT RAISE(ABORT, 'rss collection item limit exceeded');
END;

CREATE TRIGGER rss_collection_entries_max_items_update
BEFORE UPDATE OF collection_id ON rss_collection_entries
WHEN OLD.collection_id <> NEW.collection_id AND (
  SELECT COUNT(*) FROM rss_collection_entries
  WHERE collection_id = NEW.collection_id
) >= 10000
BEGIN
  SELECT RAISE(ABORT, 'rss collection item limit exceeded');
END;
`

func applyRSSCollectionLimitsSchema(ctx context.Context, db *sql.DB) error {
	return applyRSSCollectionLimits(ctx, db)
}

func applyRSSCollectionLimitsSchemaTx(ctx context.Context, tx *sql.Tx) error {
	return applyRSSCollectionLimits(ctx, tx)
}

type rssCollectionLimitsSQL interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func applyRSSCollectionLimits(ctx context.Context, database rssCollectionLimitsSQL) error {
	// Existing v17 databases may already exceed the newly enforced limit because
	// earlier application checks only bounded each request batch. Keep those
	// databases open and preserve user data: the triggers prevent further growth
	// while still allowing deletes or a bounded replacement to repair the set.
	if _, err := database.ExecContext(ctx, rssCollectionLimitsSchemaSQL); err != nil {
		return fmt.Errorf("apply RSS collection limits schema: %w", err)
	}
	return nil
}
