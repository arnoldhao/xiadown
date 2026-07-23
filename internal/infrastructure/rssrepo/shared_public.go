package rssrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domainrss "xiadown/internal/domain/rss"

	"github.com/uptrace/bun"
)

const (
	sharedPublicMutationKindSubscription = "subscription"
	sharedPublicMutationKindObservation  = "observation"
	minimumFetchLeaseTTL                 = 30 * time.Second
	maximumFetchLeaseTTL                 = 10 * time.Minute
)

func (repo *SQLiteRepository) ApplySubscriptionMutation(
	ctx context.Context,
	mutation domainrss.SubscriptionMutation,
) (domainrss.SubscriptionMutationResult, error) {
	if repo == nil || repo.db == nil {
		return domainrss.SubscriptionMutationResult{}, errors.New("rss repository unavailable")
	}
	repo.sharedPublicMutationMu.Lock()
	defer repo.sharedPublicMutationMu.Unlock()
	var result domainrss.SubscriptionMutationResult
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		found, err := loadPublicMutationResultTx(ctx, tx, mutation.DeviceID, mutation.MutationID, sharedPublicMutationKindSubscription, mutation.RequestHash, &result)
		if err != nil || found {
			return err
		}
		result, err = applySubscriptionMutationTx(ctx, tx, mutation)
		if err != nil {
			return err
		}
		return storePublicMutationResultTx(ctx, tx, mutation.DeviceID, mutation.MutationID, sharedPublicMutationKindSubscription, mutation.RequestHash, result, mutation.ChangedAt)
	})
	return result, err
}

func applySubscriptionMutationTx(
	ctx context.Context,
	tx bun.Tx,
	mutation domainrss.SubscriptionMutation,
) (domainrss.SubscriptionMutationResult, error) {
	result := domainrss.SubscriptionMutationResult{
		MutationID: mutation.MutationID,
		Operation:  string(mutation.Operation),
	}
	switch mutation.Operation {
	case domainrss.SubscriptionMutationAdd, domainrss.SubscriptionMutationPromote:
		var existingID string
		err := tx.NewSelect().Model((*subscriptionRow)(nil)).Column("id").Where("id = ?", mutation.SubscriptionID).Scan(ctx, &existingID)
		if err == nil {
			return result, domainrss.ErrRevisionConflict
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
		var tombstoneCount int
		if err := tx.NewSelect().Model((*tombstoneRow)(nil)).ColumnExpr("COUNT(*)").
			Where("workspace_id = ?", domainrss.DefaultWorkspaceID).
			Where("entity_type = 'subscription'").Where("entity_id = ?", mutation.SubscriptionID).
			Scan(ctx, &tombstoneCount); err != nil {
			return result, err
		}
		if tombstoneCount != 0 {
			return result, domainrss.ErrRevisionConflict
		}
		canonical := subscriptionRow{}
		if err := tx.NewSelect().Model(&canonical).
			Where("source_access = ?", string(domainrss.SubscriptionSourceSharedPublic)).
			Where("public_feed_url = ?", strings.TrimSpace(mutation.PublicFeedURL)).Scan(ctx); err == nil {
			item := subscriptionFromRow(canonical)
			if err := tx.NewSelect().Model((*entryRow)(nil)).ColumnExpr("COUNT(*)").
				Where("subscription_id = ?", canonical.ID).Where("read_at IS NULL").Scan(ctx, &item.UnreadCount); err != nil {
				return result, err
			}
			projection := syncSubscriptionProjection(item)
			result.Subscription = &projection
			result.Revision = canonical.Revision
			break
		} else if !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
		item := domainrss.Subscription{
			ID: mutation.SubscriptionID, WorkspaceID: domainrss.DefaultWorkspaceID,
			FeedURL:      "shared-public:" + mutation.SubscriptionID,
			SourceAccess: domainrss.SubscriptionSourceSharedPublic, PublicFeedURL: strings.TrimSpace(mutation.PublicFeedURL),
			Title: strings.TrimSpace(mutation.Title), ViewType: mutation.ViewType,
			Enabled: true, CreatedAt: mutation.ChangedAt, UpdatedAt: mutation.ChangedAt, Revision: 1,
		}
		if item.Title == "" {
			item.Title = "Untitled Feed"
		}
		if item.ViewType == "" {
			item.ViewType = domainrss.ViewTypeAuto
		}
		if mutation.CategoryID != nil {
			item.CategoryID = strings.TrimSpace(*mutation.CategoryID)
		}
		if mutation.SortOrder != nil {
			item.SortOrder = *mutation.SortOrder
		}
		if mutation.Enabled != nil {
			item.Enabled = *mutation.Enabled
		}
		row := subscriptionToRow(item)
		if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
			return result, err
		}
		if err := initializeSubscriptionFieldRevisionsTx(ctx, tx, item.ID, item.Revision); err != nil {
			return result, err
		}
		projection := syncSubscriptionProjection(item)
		if err := appendChange(ctx, tx, "subscription", item.ID, "upsert", item.Revision, projection, item.UpdatedAt); err != nil {
			return result, err
		}
		result.Subscription = &projection
		result.Revision = item.Revision

	case domainrss.SubscriptionMutationUpdate:
		if mutation.ExpectedRevision == nil {
			return result, domainrss.ErrInvalidRequest
		}
		row := subscriptionRow{ID: mutation.SubscriptionID}
		if err := tx.NewSelect().Model(&row).WherePK().Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return result, domainrss.ErrNotFound
			}
			return result, err
		}
		if normalizedSourceAccess(domainrss.SubscriptionSourceAccess(row.SourceAccess)) != domainrss.SubscriptionSourceSharedPublic {
			return result, domainrss.ErrInvalidRequest
		}
		if row.Revision != *mutation.ExpectedRevision {
			return result, domainrss.ErrRevisionConflict
		}
		currentRevision := row.Revision
		for _, field := range mutation.FieldMask {
			switch field {
			case "title":
				row.Title = strings.TrimSpace(mutation.Title)
			case "viewType":
				row.ViewType = string(mutation.ViewType)
			case "categoryId":
				if mutation.CategoryID == nil {
					row.CategoryID = nil
				} else {
					row.CategoryID = nullableStringPointer(*mutation.CategoryID)
				}
			case "sortOrder":
				if mutation.SortOrder == nil {
					return result, domainrss.ErrInvalidRequest
				}
				row.SortOrder = *mutation.SortOrder
			case "enabled":
				if mutation.Enabled == nil {
					return result, domainrss.ErrInvalidRequest
				}
				row.Enabled = *mutation.Enabled
			case "publicFeedURL":
				row.PublicFeedURL = strings.TrimSpace(mutation.PublicFeedURL)
			default:
				// sourceAccess is deliberately not remotely mutable. Promoting a
				// Desktop-private source still requires explicit Desktop UI consent.
				return result, domainrss.ErrInvalidRequest
			}
		}
		row.Revision++
		row.UpdatedAt = mutation.ChangedAt
		updated, err := tx.NewUpdate().Model(&row).
			Column("title", "view_type", "category_id", "sort_order", "enabled", "public_feed_url", "updated_at", "revision").
			Where("id = ?", row.ID).Where("revision = ?", currentRevision).Exec(ctx)
		if err != nil {
			return result, err
		}
		count, _ := updated.RowsAffected()
		if count == 0 {
			return result, domainrss.ErrRevisionConflict
		}
		if err := recordSubscriptionFieldRevisionsTx(ctx, tx, row.ID, mutation.FieldMask, row.Revision); err != nil {
			return result, err
		}
		item := subscriptionFromRow(row)
		if err := tx.NewSelect().Model((*entryRow)(nil)).ColumnExpr("COUNT(*)").
			Where("subscription_id = ?", row.ID).Where("read_at IS NULL").Scan(ctx, &item.UnreadCount); err != nil {
			return result, err
		}
		projection := syncSubscriptionProjection(item)
		if err := appendChange(ctx, tx, "subscription", row.ID, "upsert", row.Revision, projection, mutation.ChangedAt); err != nil {
			return result, err
		}
		result.Subscription = &projection
		result.Revision = row.Revision

	case domainrss.SubscriptionMutationDelete:
		if mutation.ExpectedRevision == nil {
			return result, domainrss.ErrInvalidRequest
		}
		row := subscriptionRow{ID: mutation.SubscriptionID}
		if err := tx.NewSelect().Model(&row).WherePK().Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return result, domainrss.ErrNotFound
			}
			return result, err
		}
		if normalizedSourceAccess(domainrss.SubscriptionSourceAccess(row.SourceAccess)) != domainrss.SubscriptionSourceSharedPublic {
			return result, domainrss.ErrInvalidRequest
		}
		if row.Revision != *mutation.ExpectedRevision {
			return result, domainrss.ErrRevisionConflict
		}
		deleteRevision := row.Revision + 1
		if err := appendChange(ctx, tx, "subscription", row.ID, "delete", deleteRevision, map[string]string{"id": row.ID}, mutation.ChangedAt); err != nil {
			return result, err
		}
		sequence, err := changeCursorTx(ctx, tx)
		if err != nil {
			return result, err
		}
		if _, err := tx.NewInsert().Model(&tombstoneRow{
			WorkspaceID: domainrss.DefaultWorkspaceID, EntityType: "subscription", EntityID: row.ID,
			DeletedSequence: sequence, DeletedAt: mutation.ChangedAt,
		}).On("CONFLICT (workspace_id, entity_type, entity_id) DO UPDATE").
			Set("deleted_sequence = EXCLUDED.deleted_sequence").Set("deleted_at = EXCLUDED.deleted_at").Exec(ctx); err != nil {
			return result, err
		}
		deleted, err := tx.NewDelete().Model((*subscriptionRow)(nil)).Where("id = ?", row.ID).Where("revision = ?", row.Revision).Exec(ctx)
		if err != nil {
			return result, err
		}
		count, _ := deleted.RowsAffected()
		if count == 0 {
			return result, domainrss.ErrRevisionConflict
		}
		result.DeletedID = row.ID
		result.Revision = deleteRevision

	default:
		return result, domainrss.ErrInvalidRequest
	}
	cursor, err := changeCursorTx(ctx, tx)
	if err != nil {
		return result, err
	}
	result.ChangeCursor = cursor
	return result, nil
}

func (repo *SQLiteRepository) ApplyFeedObservation(
	ctx context.Context,
	observation domainrss.FeedObservation,
) (domainrss.ObservationResult, error) {
	if repo == nil || repo.db == nil {
		return domainrss.ObservationResult{}, errors.New("rss repository unavailable")
	}
	repo.sharedPublicMutationMu.Lock()
	defer repo.sharedPublicMutationMu.Unlock()
	var result domainrss.ObservationResult
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		found, err := loadPublicMutationResultTx(ctx, tx, observation.DeviceID, observation.MutationID, sharedPublicMutationKindObservation, observation.RequestHash, &result)
		if err != nil || found {
			return err
		}
		subscription := subscriptionRow{ID: observation.SubscriptionID}
		if err := tx.NewSelect().Model(&subscription).WherePK().Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domainrss.ErrNotFound
			}
			return err
		}
		if normalizedSourceAccess(domainrss.SubscriptionSourceAccess(subscription.SourceAccess)) != domainrss.SubscriptionSourceSharedPublic {
			return domainrss.ErrInvalidRequest
		}

		result = domainrss.ObservationResult{MutationID: observation.MutationID, AcceptedAt: observation.AcceptedAt.UTC()}
		if result.AcceptedAt.IsZero() {
			result.AcceptedAt = time.Now().UTC()
		}
		changes := make([]pendingEntryChange, 0, len(observation.CanonicalEntries))
		for _, item := range observation.CanonicalEntries {
			existing := entryRow{}
			err := tx.NewSelect().Model(&existing).
				Where("subscription_id = ?", observation.SubscriptionID).
				Where("external_id = ?", item.ExternalID).Scan(ctx)
			if err == nil && !observation.FetchedAt.After(existing.ModifiedAt) {
				result.Mappings = append(result.Mappings, domainrss.OriginEntryMapping{
					OriginKey: item.OriginKey, EntryID: existing.ID, ContentRevision: existing.Revision,
				})
				continue
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			partial := domainrss.UpsertResult{}
			entryChanges, err := upsertFeedEntries(ctx, tx, observation.SubscriptionID, []domainrss.Entry{item}, &partial)
			if err != nil {
				return err
			}
			result.Created += partial.Created
			result.Updated += partial.Updated
			changes = append(changes, entryChanges...)
			origin := entryOriginRow{SubscriptionID: observation.SubscriptionID, OriginKey: item.OriginKey}
			if err := tx.NewSelect().Model(&origin).WherePK().Scan(ctx); err != nil {
				return err
			}
			resolved := entryRow{ID: origin.EntryID}
			if err := tx.NewSelect().Model(&resolved).WherePK().Scan(ctx); err != nil {
				return err
			}
			result.Mappings = append(result.Mappings, domainrss.OriginEntryMapping{
				OriginKey: item.OriginKey, EntryID: resolved.ID, ContentRevision: resolved.Revision,
			})
		}
		if err := appendPendingEntryChanges(ctx, tx, changes); err != nil {
			return err
		}
		observationSource := observationSourceRow{
			SubscriptionID: observation.SubscriptionID, DeviceID: observation.DeviceID,
			UpstreamETag: observation.UpstreamETag, LastModified: observation.LastModified,
			ContentHash: observation.ContentHash, FetchedAt: observation.FetchedAt,
			AcceptedAt: result.AcceptedAt,
		}
		if _, err := tx.NewInsert().Model(&observationSource).
			On("CONFLICT (subscription_id, device_id) DO UPDATE").
			Set("upstream_etag = CASE WHEN fetched_at < EXCLUDED.fetched_at THEN EXCLUDED.upstream_etag ELSE upstream_etag END").
			Set("last_modified = CASE WHEN fetched_at < EXCLUDED.fetched_at THEN EXCLUDED.last_modified ELSE last_modified END").
			Set("content_hash = CASE WHEN fetched_at < EXCLUDED.fetched_at THEN EXCLUDED.content_hash ELSE content_hash END").
			Set("fetched_at = MAX(fetched_at, EXCLUDED.fetched_at)").
			Set("accepted_at = EXCLUDED.accepted_at").Exec(ctx); err != nil {
			return err
		}
		result.ChangeCursor, err = changeCursorTx(ctx, tx)
		if err != nil {
			return err
		}
		return storePublicMutationResultTx(ctx, tx, observation.DeviceID, observation.MutationID, sharedPublicMutationKindObservation, observation.RequestHash, result, result.AcceptedAt)
	})
	return result, err
}

func (repo *SQLiteRepository) AcquireFetchLease(
	ctx context.Context,
	request domainrss.FetchLeaseRequest,
) (domainrss.FetchLeaseResult, error) {
	if repo == nil || repo.db == nil {
		return domainrss.FetchLeaseResult{}, errors.New("rss repository unavailable")
	}
	var result domainrss.FetchLeaseResult
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var sourceAccess string
		if err := tx.NewSelect().Model((*subscriptionRow)(nil)).Column("source_access").
			Where("id = ?", request.SubscriptionID).Scan(ctx, &sourceAccess); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domainrss.ErrNotFound
			}
			return err
		}
		if normalizedSourceAccess(domainrss.SubscriptionSourceAccess(sourceAccess)) != domainrss.SubscriptionSourceSharedPublic {
			return domainrss.ErrInvalidRequest
		}
		ttl := request.RequestedTTL
		if ttl < minimumFetchLeaseTTL {
			ttl = minimumFetchLeaseTTL
		}
		if ttl > maximumFetchLeaseTTL {
			ttl = maximumFetchLeaseTTL
		}
		now := request.RequestedAt.UTC()
		candidate := fetchLeaseRow{
			SubscriptionID: request.SubscriptionID, LeaseID: request.LeaseID, DeviceID: request.DeviceID,
			AcquiredAt: now, ExpiresAt: now.Add(ttl),
		}
		granted := fetchLeaseRow{}
		err := tx.NewRaw(`
INSERT INTO rss_fetch_leases (subscription_id, lease_id, device_id, acquired_at, expires_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (subscription_id) DO UPDATE SET
  lease_id = excluded.lease_id,
  device_id = excluded.device_id,
  acquired_at = excluded.acquired_at,
  expires_at = excluded.expires_at
WHERE rss_fetch_leases.expires_at <= ?
RETURNING subscription_id, lease_id, device_id, acquired_at, expires_at
`, candidate.SubscriptionID, candidate.LeaseID, candidate.DeviceID, candidate.AcquiredAt, candidate.ExpiresAt, now).Scan(ctx, &granted)
		if err == nil {
			result = domainrss.FetchLeaseResult{LeaseID: granted.LeaseID, Granted: true, ExpiresAt: granted.ExpiresAt}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		current := fetchLeaseRow{SubscriptionID: request.SubscriptionID}
		if err := tx.NewSelect().Model(&current).WherePK().Scan(ctx); err != nil {
			return err
		}
		if current.DeviceID == request.DeviceID && current.ExpiresAt.After(now) {
			result = domainrss.FetchLeaseResult{LeaseID: current.LeaseID, Granted: true, ExpiresAt: current.ExpiresAt}
			return nil
		}
		result = domainrss.FetchLeaseResult{
			Granted: false, ExpiresAt: current.ExpiresAt,
			RetryAfterSeconds: max(1, int(current.ExpiresAt.Sub(now).Seconds())),
		}
		return nil
	})
	return result, err
}

func loadPublicMutationResultTx(
	ctx context.Context,
	tx bun.Tx,
	deviceID, mutationID, kind, requestHash string,
	result any,
) (bool, error) {
	row := publicMutationRow{DeviceID: deviceID, MutationID: mutationID}
	if err := tx.NewSelect().Model(&row).WherePK().Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if row.MutationKind != kind || row.RequestHash != requestHash {
		return false, domainrss.ErrIdempotencyConflict
	}
	if err := json.Unmarshal([]byte(row.ResultJSON), result); err != nil {
		return false, fmt.Errorf("decode RSS public mutation receipt: %w", err)
	}
	return true, nil
}

func storePublicMutationResultTx(
	ctx context.Context,
	tx bun.Tx,
	deviceID, mutationID, kind, requestHash string,
	result any,
	createdAt time.Time,
) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.NewInsert().Model(&publicMutationRow{
		DeviceID: deviceID, MutationID: mutationID, MutationKind: kind,
		RequestHash: requestHash, ResultJSON: string(encoded), CreatedAt: createdAt.UTC(),
	}).Exec(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM rss_public_mutations WHERE created_at < ?
`, createdAt.UTC().Add(-rssMutationReceiptTTL)); err != nil {
		return fmt.Errorf("expire RSS public mutation receipts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM rss_public_mutations
WHERE rowid IN (
  SELECT rowid FROM rss_public_mutations
  ORDER BY created_at DESC, rowid DESC
  LIMIT -1 OFFSET ?
)
`, maxRetainedRSSMutationReceipts); err != nil {
		return fmt.Errorf("bound RSS public mutation receipts: %w", err)
	}
	return nil
}

func changeCursorTx(ctx context.Context, tx bun.Tx) (int64, error) {
	var cursor int64
	err := tx.NewSelect().Model((*changeRow)(nil)).ColumnExpr("COALESCE(MAX(sequence), 0)").
		Where("workspace_id = ?", domainrss.DefaultWorkspaceID).Scan(ctx, &cursor)
	return cursor, err
}

var subscriptionRevisionFields = []string{
	"title", "viewType", "categoryId", "sortOrder", "enabled", "sourceAccess", "publicFeedURL",
}

func initializeSubscriptionFieldRevisionsTx(ctx context.Context, tx bun.Tx, subscriptionID string, revision int64) error {
	return recordSubscriptionFieldRevisionsTx(ctx, tx, subscriptionID, subscriptionRevisionFields, revision)
}

func recordSubscriptionFieldRevisionsTx(
	ctx context.Context,
	tx bun.Tx,
	subscriptionID string,
	fields []string,
	revision int64,
) error {
	for _, field := range fields {
		if _, err := tx.NewRaw(`
INSERT INTO rss_subscription_field_revisions (subscription_id, field_name, revision)
VALUES (?, ?, ?)
ON CONFLICT (subscription_id, field_name) DO UPDATE SET revision = excluded.revision
`, subscriptionID, field, revision).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func recordChangedSubscriptionFieldRevisionsTx(ctx context.Context, tx bun.Tx, existing, incoming subscriptionRow) error {
	fields := make([]string, 0, len(subscriptionRevisionFields))
	if existing.Title != incoming.Title {
		fields = append(fields, "title")
	}
	if existing.ViewType != incoming.ViewType {
		fields = append(fields, "viewType")
	}
	if nullableStringValue(existing.CategoryID) != nullableStringValue(incoming.CategoryID) {
		fields = append(fields, "categoryId")
	}
	if existing.SortOrder != incoming.SortOrder {
		fields = append(fields, "sortOrder")
	}
	if existing.Enabled != incoming.Enabled {
		fields = append(fields, "enabled")
	}
	if existing.SourceAccess != incoming.SourceAccess {
		fields = append(fields, "sourceAccess")
	}
	if existing.PublicFeedURL != incoming.PublicFeedURL {
		fields = append(fields, "publicFeedURL")
	}
	return recordSubscriptionFieldRevisionsTx(ctx, tx, incoming.ID, fields, incoming.Revision)
}
