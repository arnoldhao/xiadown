package rssrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	domainrss "xiadown/internal/domain/rss"

	"github.com/uptrace/bun"
)

// Keep this synchronized with the application collection bound and the
// persistence triggers. The repository check avoids constructing an
// inevitably failing replacement one row at a time; the triggers remain the
// final invariant for every database writer.
const maxRSSRepositoryCollectionItems = 10_000

type categoryRow struct {
	bun.BaseModel     `bun:"table:rss_categories,alias:category_row"`
	ID                string    `bun:"id,pk"`
	WorkspaceID       string    `bun:"workspace_id"`
	Title             string    `bun:"title"`
	SortOrder         int       `bun:"sort_order"`
	SubscriptionCount int       `bun:"subscription_count,scanonly"`
	UnreadCount       int       `bun:"unread_count,scanonly"`
	CreatedAt         time.Time `bun:"created_at"`
	UpdatedAt         time.Time `bun:"updated_at"`
	Revision          int64     `bun:"revision"`
}

type collectionRow struct {
	bun.BaseModel `bun:"table:rss_collections,alias:collection_row"`
	ID            string    `bun:"id,pk"`
	WorkspaceID   string    `bun:"workspace_id"`
	Title         string    `bun:"title"`
	Description   string    `bun:"description"`
	Kind          string    `bun:"kind"`
	ViewType      string    `bun:"view_type"`
	SortOrder     int       `bun:"sort_order"`
	ItemCount     int       `bun:"item_count,scanonly"`
	UnreadCount   int       `bun:"unread_count,scanonly"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
	Revision      int64     `bun:"revision"`
}

type sourceRow struct {
	bun.BaseModel  `bun:"table:rss_sources,alias:source_row"`
	ID             string    `bun:"id,pk"`
	WorkspaceID    string    `bun:"workspace_id"`
	SubscriptionID string    `bun:"subscription_id"`
	Kind           string    `bun:"kind"`
	Handle         string    `bun:"handle"`
	SortOrder      int       `bun:"sort_order"`
	CreatedAt      time.Time `bun:"created_at"`
	UpdatedAt      time.Time `bun:"updated_at"`
	Revision       int64     `bun:"revision"`
	Title          string    `bun:"title,scanonly"`
	Enabled        bool      `bun:"enabled,scanonly"`
	UnreadCount    int       `bun:"unread_count,scanonly"`
}

func (repo *SQLiteRepository) ListCategories(ctx context.Context) ([]domainrss.Category, error) {
	rows := make([]categoryRow, 0)
	if err := selectCategoryCounts(repo.db.NewSelect().Model(&rows)).
		Where("category_row.workspace_id = ?", domainrss.DefaultWorkspaceID).
		OrderExpr("category_row.sort_order ASC, category_row.title COLLATE NOCASE ASC, category_row.id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	items := make([]domainrss.Category, 0, len(rows))
	for _, row := range rows {
		items = append(items, categoryFromRow(row))
	}
	return items, nil
}

func (repo *SQLiteRepository) GetCategory(ctx context.Context, id string) (domainrss.Category, error) {
	row := categoryRow{ID: strings.TrimSpace(id)}
	err := selectCategoryCounts(repo.db.NewSelect().Model(&row)).
		Where("category_row.id = ?", row.ID).
		Where("category_row.workspace_id = ?", domainrss.DefaultWorkspaceID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.Category{}, domainrss.ErrNotFound
	}
	return categoryFromRow(row), err
}

func selectCategoryCounts(query *bun.SelectQuery) *bun.SelectQuery {
	return query.Column("category_row.*").
		ColumnExpr("(SELECT COUNT(*) FROM rss_subscriptions subscription WHERE subscription.category_id = category_row.id) AS subscription_count").
		ColumnExpr(`(SELECT COUNT(*) FROM rss_entries entry
			WHERE entry.read_at IS NULL AND entry.subscription_id IN
			(SELECT subscription.id FROM rss_subscriptions subscription WHERE subscription.category_id = category_row.id)) AS unread_count`)
}

func (repo *SQLiteRepository) CreateCategory(ctx context.Context, item domainrss.Category) (domainrss.Category, error) {
	row := categoryToRow(item)
	if _, err := repo.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return domainrss.Category{}, normalizeOrganizationWriteError(err)
	}
	return repo.GetCategory(ctx, item.ID)
}

func (repo *SQLiteRepository) UpdateCategory(ctx context.Context, item domainrss.Category) (domainrss.Category, error) {
	row := categoryToRow(item)
	result, err := repo.db.NewUpdate().Model(&row).
		Column("title", "sort_order", "updated_at", "revision").
		Where("id = ?", row.ID).Where("workspace_id = ?", domainrss.DefaultWorkspaceID).
		Where("revision = ?", row.Revision-1).Exec(ctx)
	if err != nil {
		return domainrss.Category{}, normalizeOrganizationWriteError(err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return domainrss.Category{}, domainrss.ErrRevisionConflict
	}
	return repo.GetCategory(ctx, item.ID)
}

func (repo *SQLiteRepository) DeleteCategory(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return domainrss.ErrNotFound
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var count int
		if err := tx.NewSelect().Model((*categoryRow)(nil)).ColumnExpr("COUNT(*)").
			Where("id = ?", id).Where("workspace_id = ?", domainrss.DefaultWorkspaceID).Scan(ctx, &count); err != nil {
			return err
		}
		if count == 0 {
			return domainrss.ErrNotFound
		}
		rows := make([]subscriptionRow, 0)
		if err := tx.NewSelect().Model(&rows).Where("category_id = ?", id).Scan(ctx); err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, row := range rows {
			desktopLocal, err := isDesktopLocalSourceSubscriptionTx(ctx, tx, row.ID)
			if err != nil {
				return err
			}
			row.CategoryID = nil
			row.Revision++
			row.UpdatedAt = now
			if _, err := tx.NewUpdate().Model(&row).Column("category_id", "updated_at", "revision").WherePK().Exec(ctx); err != nil {
				return err
			}
			if !desktopLocal {
				if err := appendChange(ctx, tx, "subscription", row.ID, "upsert", row.Revision, syncSubscriptionProjection(subscriptionFromRow(row)), now); err != nil {
					return err
				}
			}
		}
		_, err := tx.NewDelete().Model((*categoryRow)(nil)).Where("id = ?", id).Exec(ctx)
		return err
	})
}

func (repo *SQLiteRepository) ReorderCategories(ctx context.Context, ids []string, changedAt time.Time) ([]domainrss.Category, error) {
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		rows := make([]categoryRow, 0)
		if err := tx.NewSelect().Model(&rows).Where("workspace_id = ?", domainrss.DefaultWorkspaceID).Scan(ctx); err != nil {
			return err
		}
		if err := validateExactOrganizationOrder(ids, categoryIDs(rows)); err != nil {
			return err
		}
		for index, id := range ids {
			if _, err := tx.NewUpdate().Model((*categoryRow)(nil)).
				Set("sort_order = ?", index).Set("updated_at = ?", changedAt.UTC()).Set("revision = revision + 1").
				Where("id = ?", id).Where("workspace_id = ?", domainrss.DefaultWorkspaceID).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repo.ListCategories(ctx)
}

func (repo *SQLiteRepository) ReorderSubscriptions(ctx context.Context, categoryID string, ids []string, changedAt time.Time) ([]domainrss.Subscription, error) {
	categoryID = strings.TrimSpace(categoryID)
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		rows := make([]subscriptionRow, 0)
		query := tx.NewSelect().Model(&rows).
			Where("workspace_id = ?", domainrss.DefaultWorkspaceID).
			Where("NOT EXISTS (SELECT 1 FROM rss_sources source WHERE source.subscription_id = subscription_row.id)")
		if categoryID == "" {
			query = query.Where("category_id IS NULL")
		} else {
			query = query.Where("category_id = ?", categoryID)
		}
		if err := query.Scan(ctx); err != nil {
			return err
		}
		existingIDs := make([]string, 0, len(rows))
		byID := make(map[string]subscriptionRow, len(rows))
		for _, row := range rows {
			existingIDs = append(existingIDs, row.ID)
			byID[row.ID] = row
		}
		if err := validateExactOrganizationOrder(ids, existingIDs); err != nil {
			return err
		}
		for index, id := range ids {
			row := byID[id]
			row.SortOrder = index
			row.UpdatedAt = changedAt.UTC()
			row.Revision++
			if _, err := tx.NewUpdate().Model(&row).Column("sort_order", "updated_at", "revision").WherePK().Exec(ctx); err != nil {
				return err
			}
			if err := appendChange(ctx, tx, "subscription", id, "upsert", row.Revision, syncSubscriptionProjection(subscriptionFromRow(row)), changedAt.UTC()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	all, err := repo.ListSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domainrss.Subscription, 0, len(ids))
	for _, item := range all {
		if item.CategoryID == categoryID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo *SQLiteRepository) ListCollections(ctx context.Context) ([]domainrss.Collection, error) {
	rows := make([]collectionRow, 0)
	if err := selectCollectionCounts(repo.db.NewSelect().Model(&rows)).
		Where("collection_row.workspace_id = ?", domainrss.DefaultWorkspaceID).
		OrderExpr("collection_row.sort_order ASC, collection_row.title COLLATE NOCASE ASC, collection_row.id ASC").Scan(ctx); err != nil {
		return nil, err
	}
	items := make([]domainrss.Collection, 0, len(rows))
	for _, row := range rows {
		items = append(items, collectionFromRow(row))
	}
	return items, nil
}

func (repo *SQLiteRepository) GetCollection(ctx context.Context, id string) (domainrss.Collection, error) {
	row := collectionRow{ID: strings.TrimSpace(id)}
	err := selectCollectionCounts(repo.db.NewSelect().Model(&row)).
		Where("collection_row.id = ?", row.ID).Where("collection_row.workspace_id = ?", domainrss.DefaultWorkspaceID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.Collection{}, domainrss.ErrNotFound
	}
	return collectionFromRow(row), err
}

func selectCollectionCounts(query *bun.SelectQuery) *bun.SelectQuery {
	return query.Column("collection_row.*").
		ColumnExpr(`CASE collection_row.kind
			WHEN 'subscriptions' THEN (SELECT COUNT(*) FROM rss_collection_subscriptions member WHERE member.collection_id = collection_row.id)
			ELSE (SELECT COUNT(*) FROM rss_collection_entries member WHERE member.collection_id = collection_row.id)
		END AS item_count`).
		ColumnExpr(`CASE collection_row.kind
			WHEN 'subscriptions' THEN (SELECT COUNT(*) FROM rss_entries entry WHERE entry.read_at IS NULL AND entry.subscription_id IN
				(SELECT member.subscription_id FROM rss_collection_subscriptions member WHERE member.collection_id = collection_row.id))
			ELSE (SELECT COUNT(*) FROM rss_entries entry WHERE entry.read_at IS NULL AND entry.id IN
				(SELECT member.entry_id FROM rss_collection_entries member WHERE member.collection_id = collection_row.id))
		END AS unread_count`)
}

func (repo *SQLiteRepository) CreateCollection(ctx context.Context, item domainrss.Collection) (domainrss.Collection, error) {
	row := collectionToRow(item)
	if _, err := repo.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return domainrss.Collection{}, normalizeOrganizationWriteError(err)
	}
	return repo.GetCollection(ctx, item.ID)
}

func (repo *SQLiteRepository) UpdateCollection(ctx context.Context, item domainrss.Collection) (domainrss.Collection, error) {
	row := collectionToRow(item)
	result, err := repo.db.NewUpdate().Model(&row).
		Column("title", "description", "view_type", "sort_order", "updated_at", "revision").
		Where("id = ?", row.ID).Where("workspace_id = ?", domainrss.DefaultWorkspaceID).
		Where("revision = ?", row.Revision-1).Exec(ctx)
	if err != nil {
		return domainrss.Collection{}, normalizeOrganizationWriteError(err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return domainrss.Collection{}, domainrss.ErrRevisionConflict
	}
	return repo.GetCollection(ctx, item.ID)
}

func (repo *SQLiteRepository) DeleteCollection(ctx context.Context, id string) error {
	result, err := repo.db.NewDelete().Model((*collectionRow)(nil)).
		Where("id = ?", strings.TrimSpace(id)).Where("workspace_id = ?", domainrss.DefaultWorkspaceID).Exec(ctx)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return domainrss.ErrNotFound
	}
	return nil
}

func (repo *SQLiteRepository) ListCollectionItems(ctx context.Context, id string) (domainrss.CollectionItems, error) {
	item, err := repo.GetCollection(ctx, id)
	if err != nil {
		return domainrss.CollectionItems{}, err
	}
	ids := make([]string, 0)
	var query *bun.SelectQuery
	if item.Kind == domainrss.CollectionKindSubscriptions {
		query = repo.db.NewSelect().Table("rss_collection_subscriptions").Column("subscription_id").Where("collection_id = ?", item.ID)
	} else {
		query = repo.db.NewSelect().Table("rss_collection_entries").Column("entry_id").Where("collection_id = ?", item.ID)
	}
	if err := query.OrderExpr("sort_order ASC, added_at ASC").Scan(ctx, &ids); err != nil {
		return domainrss.CollectionItems{}, err
	}
	return domainrss.CollectionItems{CollectionID: item.ID, Kind: item.Kind, ItemIDs: ids}, nil
}

func (repo *SQLiteRepository) ReplaceCollectionItems(ctx context.Context, id string, kind domainrss.CollectionKind, ids []string, changedAt time.Time) (domainrss.Collection, error) {
	return repo.mutateCollectionItems(ctx, id, kind, ids, changedAt, "replace")
}

func (repo *SQLiteRepository) AddCollectionItems(ctx context.Context, id string, kind domainrss.CollectionKind, ids []string, changedAt time.Time) (domainrss.Collection, error) {
	return repo.mutateCollectionItems(ctx, id, kind, ids, changedAt, "add")
}

func (repo *SQLiteRepository) RemoveCollectionItems(ctx context.Context, id string, kind domainrss.CollectionKind, ids []string, changedAt time.Time) (domainrss.Collection, error) {
	return repo.mutateCollectionItems(ctx, id, kind, ids, changedAt, "remove")
}

func (repo *SQLiteRepository) mutateCollectionItems(
	ctx context.Context,
	id string,
	kind domainrss.CollectionKind,
	ids []string,
	changedAt time.Time,
	mode string,
) (domainrss.Collection, error) {
	// SQLite permits multiple readers but only one writer. Serialize this
	// read/validate/write sequence within the repository so concurrent batches
	// observe the previous batch's committed size instead of racing a stale
	// count. The database triggers remain the final invariant for other writers
	// and other repository instances.
	repo.collectionMutationMu.Lock()
	defer repo.collectionMutationMu.Unlock()

	changed := false
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		row := collectionRow{ID: strings.TrimSpace(id)}
		if err := tx.NewSelect().Model(&row).WherePK().Where("workspace_id = ?", domainrss.DefaultWorkspaceID).Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domainrss.ErrNotFound
			}
			return err
		}
		if domainrss.CollectionKind(row.Kind) != kind {
			return domainrss.ErrInvalidRequest
		}
		if err := validateCollectionItemIDs(ctx, tx, kind, ids); err != nil {
			return err
		}
		if mode == "replace" && collectionItemIDCountExceedsLimit(ids) {
			return fmt.Errorf("%w: collection contains too many items", domainrss.ErrInvalidRequest)
		}
		table, column := "rss_collection_entries", "entry_id"
		if kind == domainrss.CollectionKindSubscriptions {
			table, column = "rss_collection_subscriptions", "subscription_id"
		}
		if mode == "replace" {
			if _, err := tx.NewDelete().Table(table).Where("collection_id = ?", row.ID).Exec(ctx); err != nil {
				return err
			}
			changed = true
		}
		if mode == "remove" {
			for _, itemID := range ids {
				result, err := tx.NewDelete().Table(table).Where("collection_id = ?", row.ID).Where(column+" = ?", itemID).Exec(ctx)
				if err != nil {
					return err
				}
				count, _ := result.RowsAffected()
				changed = changed || count > 0
			}
		} else {
			start := 0
			if mode == "add" {
				if err := tx.NewSelect().Table(table).ColumnExpr("COALESCE(MAX(sort_order), -1) + 1").Where("collection_id = ?", row.ID).Scan(ctx, &start); err != nil {
					return err
				}
			}
			for index, itemID := range ids {
				result, err := tx.ExecContext(ctx,
					"INSERT INTO "+table+" (collection_id, "+column+", sort_order, added_at) VALUES (?, ?, ?, ?) "+
						"ON CONFLICT (collection_id, "+column+") DO NOTHING",
					row.ID, itemID, start+index, changedAt.UTC(),
				)
				if err != nil {
					return err
				}
				count, _ := result.RowsAffected()
				changed = changed || count > 0
			}
		}
		if changed {
			_, err := tx.NewUpdate().Model((*collectionRow)(nil)).
				Set("updated_at = ?", changedAt.UTC()).Set("revision = revision + 1").Where("id = ?", row.ID).Exec(ctx)
			return err
		}
		return nil
	})
	if err != nil {
		return domainrss.Collection{}, normalizeOrganizationWriteError(err)
	}
	return repo.GetCollection(ctx, id)
}

func collectionItemIDCountExceedsLimit(ids []string) bool {
	if len(ids) <= maxRSSRepositoryCollectionItems {
		return false
	}
	unique := make(map[string]struct{}, maxRSSRepositoryCollectionItems+1)
	for _, id := range ids {
		unique[id] = struct{}{}
		if len(unique) > maxRSSRepositoryCollectionItems {
			return true
		}
	}
	return false
}

func validateCollectionItemIDs(ctx context.Context, tx bun.Tx, kind domainrss.CollectionKind, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	for start := 0; start < len(ids); start += 400 {
		end := min(start+400, len(ids))
		var count int
		query := tx.NewSelect().Table("rss_entries").ColumnExpr("COUNT(*)").Where("id IN (?)", bun.In(ids[start:end])).
			Where("subscription_id IN (SELECT id FROM rss_subscriptions WHERE workspace_id = ?)", domainrss.DefaultWorkspaceID)
		if kind == domainrss.CollectionKindSubscriptions {
			query = tx.NewSelect().Table("rss_subscriptions").ColumnExpr("COUNT(*)").Where("id IN (?)", bun.In(ids[start:end])).
				Where("workspace_id = ?", domainrss.DefaultWorkspaceID)
		}
		if err := query.Scan(ctx, &count); err != nil {
			return err
		}
		if count != end-start {
			return domainrss.ErrNotFound
		}
	}
	return nil
}

func (repo *SQLiteRepository) ListSources(ctx context.Context) ([]domainrss.Source, error) {
	rows := make([]sourceRow, 0)
	if err := selectSourceProjection(repo.db.NewSelect().Model(&rows)).
		Where("source_row.workspace_id = ?", domainrss.DefaultWorkspaceID).
		OrderExpr("source_row.kind ASC, source_row.sort_order ASC, title COLLATE NOCASE ASC, source_row.id ASC").Scan(ctx); err != nil {
		return nil, err
	}
	items := make([]domainrss.Source, 0, len(rows))
	for _, row := range rows {
		items = append(items, sourceFromRow(row))
	}
	return items, nil
}

func (repo *SQLiteRepository) GetSource(ctx context.Context, id string) (domainrss.Source, error) {
	row := sourceRow{ID: strings.TrimSpace(id)}
	err := selectSourceProjection(repo.db.NewSelect().Model(&row)).
		Where("source_row.id = ?", row.ID).Where("source_row.workspace_id = ?", domainrss.DefaultWorkspaceID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.Source{}, domainrss.ErrNotFound
	}
	return sourceFromRow(row), err
}

func selectSourceProjection(query *bun.SelectQuery) *bun.SelectQuery {
	return query.Column("source_row.*").
		ColumnExpr("(SELECT subscription.title FROM rss_subscriptions subscription WHERE subscription.id = source_row.subscription_id) AS title").
		ColumnExpr("(SELECT subscription.enabled FROM rss_subscriptions subscription WHERE subscription.id = source_row.subscription_id) AS enabled").
		ColumnExpr("(SELECT COUNT(*) FROM rss_entries entry WHERE entry.subscription_id = source_row.subscription_id AND entry.read_at IS NULL) AS unread_count")
}

func (repo *SQLiteRepository) CreateSource(ctx context.Context, item domainrss.Source, subscription domainrss.Subscription) (domainrss.Source, error) {
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		subscriptionRow := subscriptionToRow(subscription)
		if _, err := tx.NewInsert().Model(&subscriptionRow).Exec(ctx); err != nil {
			return normalizeOrganizationWriteError(err)
		}
		row := sourceToRow(item)
		if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
			return normalizeOrganizationWriteError(err)
		}
		return nil
	})
	if err != nil {
		return domainrss.Source{}, err
	}
	return repo.GetSource(ctx, item.ID)
}

func (repo *SQLiteRepository) UpdateSource(ctx context.Context, item domainrss.Source, subscription domainrss.Subscription) (domainrss.Source, error) {
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		existing := sourceRow{ID: strings.TrimSpace(item.ID)}
		if err := tx.NewSelect().Model(&existing).WherePK().Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domainrss.ErrNotFound
			}
			return err
		}
		if item.Revision != existing.Revision+1 || existing.SubscriptionID != subscription.ID {
			return domainrss.ErrRevisionConflict
		}
		existingSubscription := subscriptionRow{ID: subscription.ID}
		if err := tx.NewSelect().Model(&existingSubscription).WherePK().Scan(ctx); err != nil {
			return err
		}
		if subscription.Revision != existingSubscription.Revision+1 {
			return domainrss.ErrRevisionConflict
		}
		row := sourceToRow(item)
		if _, err := tx.NewUpdate().Model(&row).Column("sort_order", "updated_at", "revision").WherePK().Exec(ctx); err != nil {
			return err
		}
		subscriptionRow := subscriptionToRow(subscription)
		if _, err := tx.NewUpdate().Model(&subscriptionRow).
			Column("title", "enabled", "updated_at", "revision").WherePK().Exec(ctx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domainrss.Source{}, err
	}
	return repo.GetSource(ctx, item.ID)
}

func (repo *SQLiteRepository) GetSourceEntry(ctx context.Context, sourceID, externalID string) (domainrss.Entry, error) {
	row := entryRow{}
	err := repo.db.NewSelect().Model(&row).
		Where("subscription_id = (SELECT subscription_id FROM rss_sources WHERE id = ? AND workspace_id = ?)", strings.TrimSpace(sourceID), domainrss.DefaultWorkspaceID).
		Where("external_id = ?", strings.TrimSpace(externalID)).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.Entry{}, domainrss.ErrNotFound
	}
	return entryFromRow(row), err
}

func categoryToRow(item domainrss.Category) categoryRow {
	return categoryRow{ID: item.ID, WorkspaceID: item.WorkspaceID, Title: item.Title, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Revision: item.Revision}
}

func categoryFromRow(row categoryRow) domainrss.Category {
	return domainrss.Category{ID: row.ID, WorkspaceID: row.WorkspaceID, Title: row.Title, SortOrder: row.SortOrder,
		SubscriptionCount: row.SubscriptionCount, UnreadCount: row.UnreadCount,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision}
}

func collectionToRow(item domainrss.Collection) collectionRow {
	return collectionRow{ID: item.ID, WorkspaceID: item.WorkspaceID, Title: item.Title, Description: item.Description,
		Kind: string(item.Kind), ViewType: string(item.ViewType), SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Revision: item.Revision}
}

func collectionFromRow(row collectionRow) domainrss.Collection {
	return domainrss.Collection{ID: row.ID, WorkspaceID: row.WorkspaceID, Title: row.Title, Description: row.Description,
		Kind: domainrss.CollectionKind(row.Kind), ViewType: domainrss.ViewType(row.ViewType), SortOrder: row.SortOrder,
		ItemCount: row.ItemCount, UnreadCount: row.UnreadCount,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision}
}

func sourceToRow(item domainrss.Source) sourceRow {
	return sourceRow{ID: item.ID, WorkspaceID: item.WorkspaceID, SubscriptionID: item.SubscriptionID,
		Kind: string(item.Kind), Handle: item.Handle, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Revision: item.Revision}
}

func sourceFromRow(row sourceRow) domainrss.Source {
	return domainrss.Source{ID: row.ID, WorkspaceID: row.WorkspaceID, SubscriptionID: row.SubscriptionID,
		Kind: domainrss.SourceKind(row.Kind), Handle: row.Handle, Title: row.Title, Enabled: row.Enabled,
		SortOrder: row.SortOrder, UnreadCount: row.UnreadCount,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision}
}

func categoryIDs(rows []categoryRow) []string {
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.ID)
	}
	return result
}

func validateExactOrganizationOrder(incoming, existing []string) error {
	if len(incoming) != len(existing) {
		return fmt.Errorf("%w: reorder must contain every item exactly once", domainrss.ErrInvalidRequest)
	}
	seen := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		seen[id] = struct{}{}
	}
	for _, id := range incoming {
		if _, ok := seen[id]; !ok {
			return fmt.Errorf("%w: reorder contains an unknown item", domainrss.ErrInvalidRequest)
		}
		delete(seen, id)
	}
	if len(seen) != 0 {
		return fmt.Errorf("%w: reorder omits an item", domainrss.ErrInvalidRequest)
	}
	return nil
}

func normalizeOrganizationWriteError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "foreign key constraint") ||
		strings.Contains(message, "check constraint") || strings.Contains(message, "rss collection item limit exceeded") {
		return fmt.Errorf("%w: %v", domainrss.ErrInvalidRequest, err)
	}
	return err
}
