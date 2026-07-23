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

type subscriptionHistoryRow struct {
	bun.BaseModel  `bun:"table:rss_subscription_history"`
	SubscriptionID string     `bun:"subscription_id,pk"`
	CursorURL      string     `bun:"cursor_url"`
	Capability     string     `bun:"capability"`
	Exhausted      bool       `bun:"exhausted"`
	NoProgress     int        `bun:"no_progress_count"`
	LastAttemptAt  *time.Time `bun:"last_attempt_at"`
	LastSuccessAt  *time.Time `bun:"last_success_at"`
	LastError      string     `bun:"last_error"`
	UpdatedAt      time.Time  `bun:"updated_at"`
}

func (repo *SQLiteRepository) GetSubscriptionHistory(
	ctx context.Context,
	subscriptionID string,
) (domainrss.SubscriptionHistoryState, error) {
	if repo == nil || repo.db == nil {
		return domainrss.SubscriptionHistoryState{}, errors.New("rss repository unavailable")
	}
	row := subscriptionHistoryRow{SubscriptionID: strings.TrimSpace(subscriptionID)}
	if row.SubscriptionID == "" {
		return domainrss.SubscriptionHistoryState{}, domainrss.ErrNotFound
	}
	err := repo.db.NewSelect().Model(&row).WherePK().Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.SubscriptionHistoryState{}, domainrss.ErrNotFound
	}
	if err != nil {
		return domainrss.SubscriptionHistoryState{}, err
	}
	return subscriptionHistoryFromRow(row), nil
}

func (repo *SQLiteRepository) PutSubscriptionHistory(
	ctx context.Context,
	state domainrss.SubscriptionHistoryState,
) error {
	if repo == nil || repo.db == nil {
		return errors.New("rss repository unavailable")
	}
	state.SubscriptionID = strings.TrimSpace(state.SubscriptionID)
	state.CursorURL = strings.TrimSpace(state.CursorURL)
	state.LastError = strings.TrimSpace(state.LastError)
	if state.SubscriptionID == "" || state.NoProgress < 0 || !validHistoryCapability(state.Capability) {
		return domainrss.ErrInvalidRequest
	}
	if state.UpdatedAt.IsZero() {
		return domainrss.ErrInvalidRequest
	}
	row := subscriptionHistoryToRow(state)
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT (subscription_id) DO UPDATE").
		Set("cursor_url = EXCLUDED.cursor_url").
		Set("capability = EXCLUDED.capability").
		Set("exhausted = EXCLUDED.exhausted").
		Set("no_progress_count = EXCLUDED.no_progress_count").
		Set("last_attempt_at = EXCLUDED.last_attempt_at").
		Set("last_success_at = EXCLUDED.last_success_at").
		Set("last_error = EXCLUDED.last_error").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// UpsertHistoricalFeed persists an RFC 5005 archive page without applying the
// mutable-head-feed overwrite policy used by UpsertFeed. Archive pages may
// repeat the same item in several documents, so an existing item is replaced
// only when the publisher supplied a strictly newer source update timestamp.
func (repo *SQLiteRepository) UpsertHistoricalFeed(
	ctx context.Context,
	update domainrss.FeedUpdate,
	visibleKind domainrss.EntryKind,
) (domainrss.HistoricalUpsertResult, error) {
	result := domainrss.HistoricalUpsertResult{}
	if visibleKind != "" && visibleKind != domainrss.EntryKindArticle &&
		visibleKind != domainrss.EntryKindSocial && visibleKind != domainrss.EntryKindImage &&
		visibleKind != domainrss.EntryKindVideo {
		return result, domainrss.ErrInvalidRequest
	}
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		desktopLocal, err := isDesktopLocalSourceSubscriptionTx(ctx, tx, update.Subscription.ID)
		if err != nil {
			return err
		}
		subscription := subscriptionToRow(update.Subscription)
		updated, err := tx.NewUpdate().Model(&subscription).
			Column("site_url", "title", "description", "icon_url", "view_type", "enabled", "etag", "last_modified", "validator_url", "last_fetched_at", "last_success_at", "last_error", "updated_at", "revision").
			Where("id = ?", subscription.ID).
			Where("revision = ?", subscription.Revision-1).
			Exec(ctx)
		if err != nil {
			return err
		}
		count, _ := updated.RowsAffected()
		if count == 0 {
			return domainrss.ErrRevisionConflict
		}
		entryChanges, err := upsertHistoricalFeedEntries(
			ctx, tx, update.Subscription.ID, update.Entries, visibleKind, &result,
		)
		if err != nil {
			return err
		}
		if err := tx.NewSelect().Model((*entryRow)(nil)).
			ColumnExpr("COUNT(*)").
			Where("subscription_id = ?", update.Subscription.ID).
			Where("read_at IS NULL").
			Scan(ctx, &update.Subscription.UnreadCount); err != nil {
			return err
		}
		if desktopLocal {
			return nil
		}
		if err := appendChange(
			ctx, tx, "subscription", update.Subscription.ID, "upsert", update.Subscription.Revision,
			syncSubscriptionProjection(update.Subscription), update.Subscription.UpdatedAt,
		); err != nil {
			return err
		}
		return appendPendingEntryChanges(ctx, tx, entryChanges)
	})
	return result, err
}

func upsertHistoricalFeedEntries(
	ctx context.Context,
	tx bun.Tx,
	subscriptionID string,
	entries []domainrss.Entry,
	visibleKind domainrss.EntryKind,
	result *domainrss.HistoricalUpsertResult,
) ([]pendingEntryChange, error) {
	changes := make([]pendingEntryChange, 0, len(entries))
	for _, item := range entries {
		if item.SubscriptionID != subscriptionID {
			return nil, fmt.Errorf(
				"RSS entry %q belongs to subscription %q, want %q",
				item.ID, item.SubscriptionID, subscriptionID,
			)
		}
		row := entryToRow(item)
		existing := entryRow{}
		err := tx.NewSelect().Model(&existing).
			Where("subscription_id = ?", row.SubscriptionID).
			Where("external_id = ?", row.ExternalID).
			Scan(ctx)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			row.Revision = 1
			item.Revision = 1
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				return nil, err
			}
			result.Total.Created++
			if historicalEntryVisible(item.Kind, "", visibleKind) {
				result.Visible.Created++
			}
			changes = append(changes, pendingEntryChange{
				entityID: item.ID, revision: item.Revision,
				payload: syncEntryProjection(item), changedAt: item.ModifiedAt,
			})
		case err != nil:
			return nil, err
		case !strictlyNewerSourceUpdate(existing.SourceUpdatedAt, row.SourceUpdatedAt):
			// RFC 5005 archive documents are immutable snapshots. Missing,
			// equal, and older source timestamps cannot replace a document
			// already imported from another archive page.
			continue
		default:
			row.ID = existing.ID
			item.ID = existing.ID
			row.CreatedAt = existing.CreatedAt
			item.CreatedAt = existing.CreatedAt
			row.ReadAt = existing.ReadAt
			row.StarredAt = existing.StarredAt
			row.ArticleProgressFraction = existing.ArticleProgressFraction
			row.ArticleProgressAnchor = existing.ArticleProgressAnchor
			row.ArticleProgressContentRevision = existing.ArticleProgressContentRevision
			row.VideoProgressSeconds = existing.VideoProgressSeconds
			row.VideoDurationSeconds = existing.VideoDurationSeconds
			row.VideoCompleted = existing.VideoCompleted
			row.ReadRevision = existing.ReadRevision
			row.StarredRevision = existing.StarredRevision
			row.ArticleProgressRevision = existing.ArticleProgressRevision
			row.VideoProgressSecondsRevision = existing.VideoProgressSecondsRevision
			row.StateRevision = existing.StateRevision
			row.ReadStateUpdatedAt = existing.ReadStateUpdatedAt
			row.ReadStateDeviceID = existing.ReadStateDeviceID
			row.ReadStateSubjectID = existing.ReadStateSubjectID
			item.ReadAt = existing.ReadAt
			item.StarredAt = existing.StarredAt
			applyStateRowToEntry(&item, existing)
			item.StateRevision = existing.StateRevision
			item.ReadStateUpdatedAt = existing.ReadStateUpdatedAt
			row.Revision = existing.Revision + 1
			item.Revision = row.Revision
			if _, err := tx.NewUpdate().Model(&row).WherePK().Exec(ctx); err != nil {
				return nil, err
			}
			result.Total.Updated++
			if historicalEntryVisible(item.Kind, domainrss.EntryKind(existing.Kind), visibleKind) {
				result.Visible.Updated++
			}
			changes = append(changes, pendingEntryChange{
				entityID: item.ID, revision: item.Revision,
				payload: syncEntryProjection(item), changedAt: item.ModifiedAt,
			})
		}
	}
	return changes, nil
}

func strictlyNewerSourceUpdate(existing, incoming *time.Time) bool {
	if existing == nil || incoming == nil {
		return false
	}
	return incoming.After(*existing)
}

func historicalEntryVisible(
	incoming domainrss.EntryKind,
	existing domainrss.EntryKind,
	filter domainrss.EntryKind,
) bool {
	return filter == "" || incoming == filter || existing == filter
}

func validHistoryCapability(value domainrss.HistoryCapability) bool {
	switch value {
	case domainrss.HistoryCapabilityUnknown,
		domainrss.HistoryCapabilityAvailable,
		domainrss.HistoryCapabilityUnsupported:
		return true
	default:
		return false
	}
}

func subscriptionHistoryFromRow(row subscriptionHistoryRow) domainrss.SubscriptionHistoryState {
	return domainrss.SubscriptionHistoryState{
		SubscriptionID: row.SubscriptionID,
		CursorURL:      row.CursorURL,
		Capability:     domainrss.HistoryCapability(row.Capability),
		Exhausted:      row.Exhausted,
		NoProgress:     row.NoProgress,
		LastAttemptAt:  row.LastAttemptAt,
		LastSuccessAt:  row.LastSuccessAt,
		LastError:      row.LastError,
		UpdatedAt:      row.UpdatedAt,
	}
}

func subscriptionHistoryToRow(state domainrss.SubscriptionHistoryState) subscriptionHistoryRow {
	return subscriptionHistoryRow{
		SubscriptionID: state.SubscriptionID,
		CursorURL:      state.CursorURL,
		Capability:     string(state.Capability),
		Exhausted:      state.Exhausted,
		NoProgress:     state.NoProgress,
		LastAttemptAt:  state.LastAttemptAt,
		LastSuccessAt:  state.LastSuccessAt,
		LastError:      state.LastError,
		UpdatedAt:      state.UpdatedAt.UTC(),
	}
}
