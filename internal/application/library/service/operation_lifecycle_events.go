package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/domain/library"
)

const (
	operationHistoryCategory      = "operation"
	operationEventHistoryCategory = "operation_event"
	operationEventRenamed         = "operation_renamed"
	operationEventCanceled        = "operation_canceled"
	operationEventResumed         = "operation_resumed"
	operationEventDeleteRequested = "operation_delete_requested"
	operationEventDeleted         = "operation_deleted"
)

// operationHistoryEventAtomicRepository is an optional persistence capability
// used by rename. Production SQLite repositories implement it so the mutable
// operation projection, mutable primary history and immutable activity record
// cannot diverge. Narrow repositories fall back to compensated writes below.
type operationHistoryEventAtomicRepository interface {
	SaveWithHistoryEvent(
		context.Context,
		library.LibraryOperation,
		*library.HistoryRecord,
		library.HistoryRecord,
	) error
}

// appendOperationDeleteLifecycleEvent persists the complete task snapshot on
// both sides of the destructive boundary. The deterministic identifier makes
// a retry of the same deletion mode idempotent, while a changed deletion mode
// remains a distinct user request.
func (service *LibraryService) appendOperationDeleteLifecycleEvent(
	ctx context.Context,
	before library.LibraryOperation,
	action string,
	caller string,
	occurredAt time.Time,
	retainLiveReference bool,
) (library.HistoryRecord, error) {
	if service == nil || service.histories == nil {
		return library.HistoryRecord{}, nil
	}
	if occurredAt.IsZero() {
		occurredAt = service.now()
	}
	refs := library.HistoryRecordRefs{SubjectOperationID: before.ID}
	if retainLiveReference {
		refs.OperationID = before.ID
	}
	eventID := uuid.NewSHA1(
		uuid.NameSpaceOID,
		[]byte("xiadown/library/operation-delete\x00"+before.ID+"\x00"+action+"\x00"+caller),
	).String()
	event, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          eventID,
		LibraryID:   before.LibraryID,
		Category:    operationEventHistoryCategory,
		Action:      action,
		DisplayName: before.DisplayName,
		Status:      string(before.Status),
		Source: library.HistoryRecordSource{
			Kind:   "user_action",
			Caller: caller,
			Actor:  libraryFileEventActorDesktop,
		},
		Refs:    refs,
		Files:   snapshotOperationOutputFiles(before.OutputFiles),
		Metrics: snapshotOperationMetrics(before.Metrics),
		OperationMeta: &library.OperationRecordMeta{
			Kind:         before.Kind,
			ErrorCode:    before.ErrorCode,
			ErrorMessage: before.ErrorMessage,
		},
		OccurredAt: &occurredAt,
		CreatedAt:  &occurredAt,
		UpdatedAt:  &occurredAt,
	})
	if err != nil {
		return library.HistoryRecord{}, err
	}
	if err := service.histories.Save(ctx, event); err != nil {
		return library.HistoryRecord{}, err
	}
	service.publishHistoryUpdate(toHistoryDTO(event))
	return event, nil
}

// appendOperationLifecycleEvent records the operation snapshot immediately
// before a user-triggered transition. The primary operation History row may
// continue to track the current state, while these rows remain immutable facts
// about earlier attempts and states.
func (service *LibraryService) appendOperationLifecycleEvent(
	ctx context.Context,
	before library.LibraryOperation,
	action string,
	occurredAt time.Time,
) (library.HistoryRecord, error) {
	// A few narrow unit-test services predate History wiring. Production
	// services always provide the repository, matching the file-event boundary.
	if service == nil || service.histories == nil {
		return library.HistoryRecord{}, nil
	}
	event, err := service.newOperationLifecycleEvent(before, action, occurredAt)
	if err != nil {
		return library.HistoryRecord{}, err
	}
	if err := service.histories.Save(ctx, event); err != nil {
		return library.HistoryRecord{}, err
	}
	service.publishHistoryUpdate(toHistoryDTO(event))
	return event, nil
}

func (service *LibraryService) newOperationLifecycleEvent(
	before library.LibraryOperation,
	action string,
	occurredAt time.Time,
) (library.HistoryRecord, error) {
	if occurredAt.IsZero() {
		occurredAt = service.now()
	}
	return library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          uuid.NewString(),
		LibraryID:   before.LibraryID,
		Category:    operationEventHistoryCategory,
		Action:      action,
		DisplayName: before.DisplayName,
		Status:      string(before.Status),
		Source: library.HistoryRecordSource{
			Kind:  "user_action",
			Actor: libraryFileEventActorDesktop,
		},
		Refs: library.HistoryRecordRefs{
			OperationID:        before.ID,
			SubjectOperationID: before.ID,
		},
		Files:      snapshotOperationOutputFiles(before.OutputFiles),
		Metrics:    snapshotOperationMetrics(before.Metrics),
		OccurredAt: &occurredAt,
		CreatedAt:  &occurredAt,
		UpdatedAt:  &occurredAt,
	})
}

// saveOperationRenameWithHistoryEvent keeps the task title and both history
// projections consistent. SQLite performs the write atomically; older and
// narrow repositories are compensated back to the pre-rename snapshots when
// either history write fails.
func (service *LibraryService) saveOperationRenameWithHistoryEvent(
	ctx context.Context,
	before library.LibraryOperation,
	after library.LibraryOperation,
	primaryBefore *library.HistoryRecord,
	primaryAfter *library.HistoryRecord,
	event library.HistoryRecord,
) error {
	if service == nil || service.operations == nil {
		return library.ErrOperationNotFound
	}
	if service.histories == nil {
		return service.operations.Save(ctx, after)
	}
	if repository, ok := service.operations.(operationHistoryEventAtomicRepository); ok {
		return repository.SaveWithHistoryEvent(ctx, after, primaryAfter, event)
	}

	if err := service.operations.Save(ctx, after); err != nil {
		return err
	}
	primaryAttempted := false
	if primaryAfter != nil {
		primaryAttempted = true
		if err := service.histories.Save(ctx, *primaryAfter); err != nil {
			return service.rollbackOperationRename(ctx, err, before, primaryBefore, primaryAttempted)
		}
	}
	if err := service.histories.Save(ctx, event); err != nil {
		return service.rollbackOperationRename(ctx, err, before, primaryBefore, primaryAttempted)
	}
	return nil
}

func (service *LibraryService) rollbackOperationRename(
	ctx context.Context,
	cause error,
	before library.LibraryOperation,
	primaryBefore *library.HistoryRecord,
	primaryAttempted bool,
) error {
	joined := []error{cause}
	if err := service.operations.Save(ctx, before); err != nil {
		joined = append(joined, fmt.Errorf("restore operation after failed rename activity: %w", err))
	}
	if primaryAttempted && primaryBefore != nil {
		if err := service.histories.Save(ctx, *primaryBefore); err != nil {
			joined = append(joined, fmt.Errorf("restore operation history after failed rename activity: %w", err))
		}
	}
	return errors.Join(joined...)
}

// syncPrimaryOperationHistory updates only the mutable operation summary. The
// lookup deliberately excludes lifecycle events, so an event can never be
// overwritten when it sorts ahead of the primary row.
func (service *LibraryService) syncPrimaryOperationHistory(
	ctx context.Context,
	item library.LibraryOperation,
	occurredAt time.Time,
) error {
	if service == nil {
		return nil
	}
	service.operationOutputMutationMu.Lock()
	defer service.operationOutputMutationMu.Unlock()
	service.reconcileBackgroundOperationDisplayNameLocked(ctx, &item, false)
	return service.syncPrimaryOperationHistoryLocked(ctx, item, occurredAt)
}

func (service *LibraryService) syncPrimaryOperationHistoryLocked(
	ctx context.Context,
	item library.LibraryOperation,
	occurredAt time.Time,
) error {
	history, ok, err := service.findHistoryByOperationID(ctx, item.LibraryID, item.ID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	history.Status = string(item.Status)
	history.DisplayName = item.DisplayName
	history.Files = snapshotOperationOutputFiles(item.OutputFiles)
	history.Metrics = snapshotOperationMetrics(item.Metrics)
	history.OperationMeta = &library.OperationRecordMeta{
		Kind:         item.Kind,
		ErrorCode:    item.ErrorCode,
		ErrorMessage: item.ErrorMessage,
	}
	history.OccurredAt = occurredAt
	history.UpdatedAt = occurredAt
	if err := service.histories.Save(ctx, history); err != nil {
		return err
	}
	service.publishHistoryUpdate(toHistoryDTO(history))
	return nil
}

// reconcileBackgroundOperationDisplayNameLocked resolves the only field with
// two independent owners: background jobs may discover an automatic metadata
// title until the first Companion rename, after which the durable user title
// permanently wins over every long-lived runner snapshot. Callers must hold
// operationOutputMutationMu through their subsequent Save and publish.
func (service *LibraryService) reconcileBackgroundOperationDisplayNameLocked(
	ctx context.Context,
	item *library.LibraryOperation,
	allowAutomaticMetadataTitle bool,
) {
	if service == nil || service.operations == nil || item == nil {
		return
	}
	latest, err := service.operations.Get(ctx, item.ID)
	if err != nil || latest.DisplayName == item.DisplayName {
		return
	}
	if allowAutomaticMetadataTitle {
		renamed, renameErr := service.operationHasRenameEvent(ctx, item.LibraryID, item.ID)
		if renameErr == nil && !renamed {
			return
		}
	}
	item.DisplayName = latest.DisplayName
}

func (service *LibraryService) operationHasRenameEvent(
	ctx context.Context,
	libraryID string,
	operationID string,
) (bool, error) {
	if service == nil || service.histories == nil {
		return false, nil
	}
	items, err := service.histories.ListByLibraryID(ctx, strings.TrimSpace(libraryID))
	if err != nil {
		return false, err
	}
	operationID = strings.TrimSpace(operationID)
	for _, item := range items {
		if item.Category != operationEventHistoryCategory || item.Action != operationEventRenamed {
			continue
		}
		eventOperationID := strings.TrimSpace(item.Refs.SubjectOperationID)
		if eventOperationID == "" {
			eventOperationID = strings.TrimSpace(item.Refs.OperationID)
		}
		if eventOperationID == operationID {
			return true, nil
		}
	}
	return false, nil
}

func snapshotOperationOutputFiles(files []library.OperationOutputFile) []library.OperationOutputFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]library.OperationOutputFile, len(files))
	for index, file := range files {
		result[index] = file
		if file.SizeBytes != nil {
			size := *file.SizeBytes
			result[index].SizeBytes = &size
		}
	}
	return result
}

func snapshotOperationMetrics(metrics library.OperationMetrics) library.OperationMetrics {
	result := metrics
	if metrics.TotalSizeBytes != nil {
		value := *metrics.TotalSizeBytes
		result.TotalSizeBytes = &value
	}
	if metrics.DurationMs != nil {
		value := *metrics.DurationMs
		result.DurationMs = &value
	}
	return result
}
