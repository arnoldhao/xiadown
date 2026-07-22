package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	deletedLibraryItemKindTask = "task"
	deletedLibraryItemKindFile = "file"

	deletedLibraryItemSourceTask = "operation_history"
	deletedLibraryItemSourceFile = "legacy_file"

	defaultDeletedLibraryItemsLimit = 100
	maximumDeletedLibraryItemsLimit = 500
	libraryFileStatusPurged         = "purged"
)

// ListDeletedLibraryItems returns only aggregates owned by LibraryService.
// Catalog trash is deliberately queried through CatalogHandler so revisioned
// Catalog lifecycle never becomes coupled to the legacy Library boundary.
func (service *LibraryService) ListDeletedLibraryItems(
	ctx context.Context,
	request dto.ListDeletedLibraryItemsRequest,
) (dto.ListDeletedLibraryItemsResponse, error) {
	kinds, err := normalizeDeletedLibraryItemKinds(request.Kinds)
	if err != nil {
		return dto.ListDeletedLibraryItemsResponse{}, err
	}
	limit := request.Limit
	if limit <= 0 {
		limit = defaultDeletedLibraryItemsLimit
	}
	if limit > maximumDeletedLibraryItemsLimit || request.Offset < 0 {
		return dto.ListDeletedLibraryItemsResponse{}, fmt.Errorf("invalid deleted Library pagination")
	}

	items := make([]dto.DeletedLibraryItemDTO, 0)
	if _, include := kinds[deletedLibraryItemKindTask]; include {
		tasks, listErr := service.listDeletedTaskItems(ctx, strings.TrimSpace(request.LibraryID))
		if listErr != nil {
			return dto.ListDeletedLibraryItemsResponse{}, listErr
		}
		items = append(items, tasks...)
	}
	if _, include := kinds[deletedLibraryItemKindFile]; include {
		files, listErr := service.listDeletedFileItems(ctx, strings.TrimSpace(request.LibraryID))
		if listErr != nil {
			return dto.ListDeletedLibraryItemsResponse{}, listErr
		}
		items = append(items, files...)
	}

	category := strings.ToLower(strings.TrimSpace(request.Category))
	query := strings.ToLower(strings.TrimSpace(request.Query))
	filtered := items[:0]
	for _, item := range items {
		if category != "" && category != "all" && strings.ToLower(item.Category) != category {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Title), query) {
			continue
		}
		filtered = append(filtered, item)
	}
	items = filtered
	sort.SliceStable(items, func(left, right int) bool {
		if items[left].DeletedAt != items[right].DeletedAt {
			return items[left].DeletedAt > items[right].DeletedAt
		}
		if items[left].Kind != items[right].Kind {
			return items[left].Kind < items[right].Kind
		}
		return items[left].ID < items[right].ID
	})

	response := dto.ListDeletedLibraryItemsResponse{
		Items: []dto.DeletedLibraryItemDTO{}, Total: len(items), Limit: limit, Offset: request.Offset,
	}
	if request.Offset >= len(items) {
		return response, nil
	}
	end := request.Offset + limit
	if end > len(items) {
		end = len(items)
	}
	response.Items = append(response.Items, items[request.Offset:end]...)
	return response, nil
}

func (service *LibraryService) RestoreDeletedLibraryItem(
	ctx context.Context,
	request dto.DeletedLibraryItemMutationRequest,
) (dto.DeletedLibraryItemMutationResponse, error) {
	kind := strings.ToLower(strings.TrimSpace(request.Kind))
	id := strings.TrimSpace(request.ID)
	if kind != deletedLibraryItemKindFile || id == "" {
		return dto.DeletedLibraryItemMutationResponse{}, fmt.Errorf("only deleted files can be restored")
	}
	result, err := service.RestoreDeletedLibraryFiles(ctx, dto.RestoreDeletedLibraryFilesRequest{FileIDs: []string{id}})
	if err != nil {
		return dto.DeletedLibraryItemMutationResponse{}, err
	}
	if result.Restored != 1 {
		return dto.DeletedLibraryItemMutationResponse{}, fmt.Errorf("deleted file cannot be restored")
	}
	return dto.DeletedLibraryItemMutationResponse{Kind: kind, ID: id, Restored: true}, nil
}

func normalizeDeletedLibraryItemKinds(values []string) (map[string]struct{}, error) {
	if len(values) == 0 {
		return map[string]struct{}{deletedLibraryItemKindTask: {}, deletedLibraryItemKindFile: {}}, nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case deletedLibraryItemKindTask, deletedLibraryItemKindFile:
			result[normalized] = struct{}{}
		case "":
		default:
			return nil, fmt.Errorf("invalid deleted item kind")
		}
	}
	return result, nil
}

func (service *LibraryService) listDeletedTaskItems(
	ctx context.Context,
	libraryID string,
) ([]dto.DeletedLibraryItemDTO, error) {
	if service == nil || service.histories == nil {
		return []dto.DeletedLibraryItemDTO{}, nil
	}
	libraryIDs, err := service.deletedItemLibraryIDs(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	type taskLifecycleState struct {
		deleted   *library.HistoryRecord
		requested *library.HistoryRecord
	}
	states := make(map[string]taskLifecycleState)
	for _, currentLibraryID := range libraryIDs {
		records, listErr := service.histories.ListByLibraryID(ctx, currentLibraryID)
		if listErr != nil {
			return nil, listErr
		}
		for _, record := range records {
			if record.Category != operationEventHistoryCategory ||
				(record.Action != operationEventDeleted && record.Action != operationEventDeleteRequested) {
				continue
			}
			subjectID := historySubjectOperationID(record)
			if subjectID == "" {
				continue
			}
			current := states[subjectID]
			recordCopy := record
			if record.Action == operationEventDeleted {
				if current.deleted == nil || historyRecordAfter(record, *current.deleted) {
					current.deleted = &recordCopy
				}
			} else if current.requested == nil || historyRecordAfter(record, *current.requested) {
				current.requested = &recordCopy
			}
			states[subjectID] = current
		}
	}
	result := make([]dto.DeletedLibraryItemDTO, 0, len(states))
	for subjectID, state := range states {
		record := state.deleted
		if record == nil && state.requested != nil {
			// A durable intent is a truthful deletion tombstone only once the
			// operation aggregate is actually gone. If a later completion write
			// failed, this check prevents the already-removed task from vanishing
			// from Deleted without misrepresenting a failed pre-delete attempt.
			if service.operations == nil {
				continue
			}
			_, getErr := service.operations.Get(ctx, subjectID)
			switch {
			case getErr == nil:
				continue
			case errors.Is(getErr, library.ErrOperationNotFound):
				record = state.requested
			default:
				return nil, getErr
			}
		}
		if record == nil {
			continue
		}
		historyDTO := toHistoryDTO(*record)
		category := "task"
		if record.OperationMeta != nil && strings.TrimSpace(record.OperationMeta.Kind) != "" {
			category = strings.ToLower(strings.TrimSpace(record.OperationMeta.Kind))
		}
		result = append(result, dto.DeletedLibraryItemDTO{
			ID: subjectID, Kind: deletedLibraryItemKindTask, Source: deletedLibraryItemSourceTask,
			LibraryID: record.LibraryID, Title: record.DisplayName, Category: category,
			Status: "deleted", DeletedAt: record.OccurredAt.UTC().Format(time.RFC3339),
			CanRestore: false,
			Detail:     dto.DeletedLibraryItemDetail{TaskHistory: &historyDTO},
		})
	}
	return result, nil
}

func (service *LibraryService) listDeletedFileItems(
	ctx context.Context,
	libraryID string,
) ([]dto.DeletedLibraryItemDTO, error) {
	if service == nil || service.files == nil {
		return []dto.DeletedLibraryItemDTO{}, nil
	}
	var (
		files []library.LibraryFile
		err   error
	)
	if libraryID != "" {
		files, err = service.files.ListByLibraryID(ctx, libraryID)
	} else {
		files, err = service.files.List(ctx)
	}
	if err != nil {
		return nil, err
	}
	result := make([]dto.DeletedLibraryItemDTO, 0)
	for _, item := range files {
		if !libraryFileRecoverableDeleted(item) {
			continue
		}
		fileDTO := service.mustBuildFileDTO(ctx, item)
		title := strings.TrimSpace(fileDTO.DisplayName)
		if title == "" {
			title = strings.TrimSpace(item.Name)
		}
		result = append(result, dto.DeletedLibraryItemDTO{
			ID: item.ID, Kind: deletedLibraryItemKindFile, Source: deletedLibraryItemSourceFile,
			LibraryID: item.LibraryID, Title: title, Category: string(catalogItemCategory(item)),
			Status: "deleted", DeletedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
			CanRestore: localRegularFileExists(item.Storage.LocalPath),
			Detail:     dto.DeletedLibraryItemDetail{File: &fileDTO},
		})
	}
	return result, nil
}

func (service *LibraryService) deletedItemLibraryIDs(ctx context.Context, requested string) ([]string, error) {
	if requested != "" {
		return []string{requested}, nil
	}
	if service == nil || service.libraries == nil {
		return []string{}, nil
	}
	libraries, err := service.libraries.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(libraries))
	for _, item := range libraries {
		result = append(result, item.ID)
	}
	return result, nil
}

func libraryFileRecoverableDeleted(item library.LibraryFile) bool {
	return libraryFileSoftDeleted(item) && !strings.EqualFold(strings.TrimSpace(item.State.Status), libraryFileStatusPurged)
}

func historySubjectOperationID(item library.HistoryRecord) string {
	if subject := strings.TrimSpace(item.Refs.SubjectOperationID); subject != "" {
		return subject
	}
	return strings.TrimSpace(item.Refs.OperationID)
}

func historyRecordAfter(left, right library.HistoryRecord) bool {
	if !left.OccurredAt.Equal(right.OccurredAt) {
		return left.OccurredAt.After(right.OccurredAt)
	}
	if !left.CreatedAt.Equal(right.CreatedAt) {
		return left.CreatedAt.After(right.CreatedAt)
	}
	return left.ID > right.ID
}
