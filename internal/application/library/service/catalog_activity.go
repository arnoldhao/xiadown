package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	defaultCatalogItemActivityLimit = 20
	maximumCatalogItemActivityLimit = 100
	catalogActivityChangePageSize   = 500

	catalogItemActivityUpdated  = "catalog_item_updated"
	catalogItemActivityTrashed  = "catalog_item_trashed"
	catalogItemActivityRestored = "catalog_item_restored"
)

// ListCatalogItemActivity projects the append-only Catalog change feed into
// user-facing item activity. It intentionally ignores migration/system writes:
// those changes keep synchronization correct but are not user actions.
func (service *CatalogService) ListCatalogItemActivity(
	ctx context.Context,
	request dto.ListCatalogItemActivityRequest,
) ([]dto.CatalogItemActivityDTO, error) {
	if service == nil || service.changes == nil {
		return nil, errors.New("catalog change repository unavailable")
	}
	itemID := strings.TrimSpace(request.ItemID)
	if itemID == "" {
		return nil, library.ErrInvalidCatalogItem
	}
	catalog, item, err := service.defaultCatalogItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	limit := normalizeCatalogItemActivityLimit(request.Limit)
	itemChanges := make([]library.CatalogChange, 0, limit+1)
	cursor := int64(0)
	for {
		page, listErr := service.changes.ListAfter(ctx, catalog.ID, cursor, catalogActivityChangePageSize)
		if listErr != nil {
			return nil, listErr
		}
		if len(page) == 0 {
			break
		}
		nextCursor := cursor
		for _, change := range page {
			if change.Sequence > nextCursor {
				nextCursor = change.Sequence
			}
			if change.EntityType != library.CatalogEntityItem || change.EntityID != item.ID {
				continue
			}
			// Keep non-user writes in the lifecycle state machine even though they
			// are hidden from the returned activity. A historical restore with an
			// empty/system actor must still close the preceding trash transition,
			// otherwise the next user edit would be mislabeled as a restore.
			itemChanges = append(itemChanges, change)
		}
		if nextCursor <= cursor {
			break
		}
		cursor = nextCursor
		if len(page) < catalogActivityChangePageSize {
			break
		}
	}
	return projectCatalogItemActivity(itemChanges, limit), nil
}

func normalizeCatalogItemActivityLimit(limit int) int {
	if limit <= 0 {
		return defaultCatalogItemActivityLimit
	}
	if limit > maximumCatalogItemActivityLimit {
		return maximumCatalogItemActivityLimit
	}
	return limit
}

func catalogActivityUserActor(actor string) bool {
	normalized := strings.ToLower(strings.TrimSpace(actor))
	if normalized == "" {
		return false
	}
	for _, noise := range []string{"migration", "system", "backfill"} {
		if normalized == noise || strings.HasPrefix(normalized, noise+":") ||
			strings.HasPrefix(normalized, noise+"/") || strings.HasPrefix(normalized, noise+"-") {
			return false
		}
	}
	return true
}

// projectCatalogItemActivity expects ascending change-feed order and returns
// the newest user activities first. User changes advance the lifecycle state;
// legacy actorless direct-next-revision restores close it without becoming
// visible, while unrelated migration/system writes remain hidden.
func projectCatalogItemActivity(
	changes []library.CatalogChange,
	limit int,
) []dto.CatalogItemActivityDTO {
	limit = normalizeCatalogItemActivityLimit(limit)
	result := make([]dto.CatalogItemActivityDTO, 0, min(limit, len(changes)))
	previousWasDelete := false
	trashedRevision := int64(0)
	for _, change := range changes {
		action := ""
		userAction := catalogActivityUserActor(change.ActorID)
		switch change.Kind {
		case library.CatalogChangeDelete:
			action = catalogItemActivityTrashed
			previousWasDelete = true
			trashedRevision = change.Revision
		case library.CatalogChangeUpsert:
			action = catalogItemActivityUpdated
			if previousWasDelete {
				if userAction {
					action = catalogItemActivityRestored
					previousWasDelete = false
				} else if change.Revision == trashedRevision+1 {
					// Older Library builds omitted actorId on restore. A direct
					// next revision still closes the lifecycle state, whereas
					// unrelated projection/migration noise must not consume it.
					previousWasDelete = false
				}
			}
		default:
			continue
		}
		if !userAction {
			continue
		}
		result = append(result, dto.CatalogItemActivityDTO{
			Action: action, Revision: change.Revision, Actor: strings.TrimSpace(change.ActorID),
			OccurredAt: change.OccurredAt.UTC().Format(time.RFC3339),
		})
		if len(result) > limit {
			copy(result, result[len(result)-limit:])
			result = result[:limit]
		}
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	if result == nil {
		return []dto.CatalogItemActivityDTO{}
	}
	return result
}
