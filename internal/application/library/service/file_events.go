package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	libraryFileEventActorDesktop          = "desktop-library"
	libraryFileEventOperationOutputDetach = "operation_output_detached"
	libraryFileEventCreated               = "file_created"
	libraryFileEventDeleted               = "file_deleted"
	libraryFileEventRenamed               = "file_renamed"
	libraryFileEventRelinked              = "file_relinked"
	libraryFileEventRestored              = "file_restored"
	libraryFileEventMissingDetected       = "file_missing_detected"
	libraryFileEventAvailableAgain        = "file_available_again"
)

type appendLibraryFileEventParams struct {
	EventType   string
	Category    string
	Actor       string
	OperationID string
	FileID      string
	LibraryID   string
	Before      *dto.FileEventFileSnapshotDTO
	After       *dto.FileEventFileSnapshotDTO
	Changes     []dto.FileFieldChangeDTO
	DeleteFile  bool
	OccurredAt  time.Time
}

// appendLibraryFileEvent is the single append boundary for Library file
// activity. Repositories that predate file events may be absent in narrow
// tests and migrations; production repositories persist and publish the event
// before the caller reports success.
func (service *LibraryService) appendLibraryFileEvent(
	ctx context.Context,
	params appendLibraryFileEventParams,
) (library.FileEventRecord, error) {
	if service == nil || service.fileEvents == nil {
		return library.FileEventRecord{}, nil
	}
	eventRecord, err := service.newLibraryFileEvent(params)
	if err != nil {
		return library.FileEventRecord{}, err
	}
	if err := service.fileEvents.Save(ctx, eventRecord); err != nil {
		return library.FileEventRecord{}, err
	}
	service.publishFileEventUpdate(toFileEventDTO(eventRecord))
	return eventRecord, nil
}

func (service *LibraryService) newLibraryFileEvent(
	params appendLibraryFileEventParams,
) (library.FileEventRecord, error) {
	fileID := strings.TrimSpace(params.FileID)
	libraryID := strings.TrimSpace(params.LibraryID)
	if fileID == "" {
		if params.Before != nil {
			fileID = strings.TrimSpace(params.Before.FileID)
		}
		if fileID == "" && params.After != nil {
			fileID = strings.TrimSpace(params.After.FileID)
		}
	}
	if fileID == "" || libraryID == "" {
		return library.FileEventRecord{}, library.ErrInvalidFileEvent
	}
	occurredAt := params.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = service.now()
	}
	actor := strings.TrimSpace(params.Actor)
	if actor == "" {
		actor = libraryFileEventActorDesktop
	}
	operationID := strings.TrimSpace(params.OperationID)
	detailJSON := marshalJSON(dto.FileEventDetailDTO{
		Cause: dto.FileEventCauseDTO{
			Category:    strings.TrimSpace(params.Category),
			OperationID: operationID,
			Actor:       actor,
		},
		Before:     cloneFileEventSnapshot(params.Before),
		After:      cloneFileEventSnapshot(params.After),
		Changes:    append([]dto.FileFieldChangeDTO(nil), params.Changes...),
		DeleteFile: params.DeleteFile,
	})
	return library.NewFileEventRecord(library.FileEventRecordParams{
		ID:          uuid.NewString(),
		LibraryID:   libraryID,
		FileID:      fileID,
		OperationID: operationID,
		EventType:   strings.TrimSpace(params.EventType),
		DetailJSON:  detailJSON,
		CreatedAt:   &occurredAt,
	})
}

type libraryFileEventAtomicRepository interface {
	SaveWithFileEvent(context.Context, library.LibraryFile, library.FileEventRecord) error
}

type libraryFileRenameAtomicRepository interface {
	RenameDisplayNameWithFileEvent(context.Context, library.LibraryFile, library.FileEventRecord) error
}

type libraryFileDisplayNamePreservingRepository interface {
	SavePreservingDisplayName(context.Context, library.LibraryFile) error
}

// saveLibraryFileWithEvent commits the mutable file projection and its
// immutable lifecycle fact atomically when the repository supports it. Narrow
// test repositories fall back to a compensated two-step write.
func (service *LibraryService) saveLibraryFileWithEvent(
	ctx context.Context,
	before library.LibraryFile,
	after library.LibraryFile,
	params appendLibraryFileEventParams,
) (library.FileEventRecord, error) {
	if service == nil || service.files == nil {
		return library.FileEventRecord{}, library.ErrFileNotFound
	}
	if service.fileEvents == nil {
		return library.FileEventRecord{}, service.files.Save(ctx, after)
	}
	eventRecord, err := service.newLibraryFileEvent(params)
	if err != nil {
		return library.FileEventRecord{}, err
	}
	if repository, ok := service.files.(libraryFileRenameAtomicRepository); ok &&
		eventRecord.EventType == libraryFileEventRenamed {
		if err := repository.RenameDisplayNameWithFileEvent(ctx, after, eventRecord); err != nil {
			return library.FileEventRecord{}, err
		}
	} else if repository, ok := service.files.(libraryFileEventAtomicRepository); ok {
		if err := repository.SaveWithFileEvent(ctx, after, eventRecord); err != nil {
			return library.FileEventRecord{}, err
		}
	} else {
		if err := service.files.Save(ctx, after); err != nil {
			return library.FileEventRecord{}, err
		}
		if err := service.fileEvents.Save(ctx, eventRecord); err != nil {
			// Both states are metadata-only here, so the fallback can safely
			// compensate instead of leaving an un-audited committed mutation.
			if rollbackErr := service.files.Save(ctx, before); rollbackErr != nil {
				return library.FileEventRecord{}, errors.Join(
					err,
					fmt.Errorf("restore library file after failed activity write: %w", rollbackErr),
				)
			}
			return library.FileEventRecord{}, err
		}
	}
	service.publishFileEventUpdate(toFileEventDTO(eventRecord))
	return eventRecord, nil
}

// saveLibraryFilePreservingDisplayName persists fields owned by background
// maintenance without allowing an older aggregate snapshot to undo a rename.
// SQLite implements this as a single upsert that omits display_name on
// conflict. Narrow repositories use a reload-and-merge fallback.
func (service *LibraryService) saveLibraryFilePreservingDisplayName(
	ctx context.Context,
	item library.LibraryFile,
) error {
	if service == nil || service.files == nil {
		return library.ErrFileNotFound
	}
	if repository, ok := service.files.(libraryFileDisplayNamePreservingRepository); ok {
		return repository.SavePreservingDisplayName(ctx, item)
	}
	current, err := service.files.Get(ctx, item.ID)
	if err == nil {
		item.DisplayName = current.DisplayName
	} else if !errors.Is(err, library.ErrFileNotFound) {
		return err
	}
	return service.files.Save(ctx, item)
}

func fileEventSnapshot(item library.LibraryFile) *dto.FileEventFileSnapshotDTO {
	name := strings.TrimSpace(resolveLibraryFileDisplayName(item))
	if name == "" {
		name = strings.TrimSpace(item.ID)
	}
	return &dto.FileEventFileSnapshotDTO{
		FileID:     strings.TrimSpace(item.ID),
		Kind:       strings.TrimSpace(string(item.Kind)),
		Name:       name,
		LocalPath:  strings.TrimSpace(item.Storage.LocalPath),
		DocumentID: strings.TrimSpace(item.Storage.DocumentID),
	}
}

func (service *LibraryService) appendLibraryFileCreatedEvent(
	ctx context.Context,
	item library.LibraryFile,
	category string,
	occurredAt time.Time,
) error {
	_, err := service.appendLibraryFileEvent(ctx, appendLibraryFileEventParams{
		EventType:   libraryFileEventCreated,
		Category:    strings.TrimSpace(category),
		OperationID: fileEventOperationID(item),
		FileID:      item.ID,
		LibraryID:   item.LibraryID,
		After:       fileEventSnapshot(item),
		Changes: []dto.FileFieldChangeDTO{{
			Field: "fileLifecycle", Before: "absent", After: "active",
		}},
		OccurredAt: occurredAt,
	})
	return err
}

func operationOutputFileEventSnapshot(output library.OperationOutputFile) *dto.FileEventFileSnapshotDTO {
	fileID := strings.TrimSpace(output.FileID)
	return &dto.FileEventFileSnapshotDTO{
		FileID: fileID,
		Kind:   strings.TrimSpace(output.Kind),
		Name:   fileID,
	}
}

func cloneFileEventSnapshot(value *dto.FileEventFileSnapshotDTO) *dto.FileEventFileSnapshotDTO {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func fileLifecycleValue(item library.LibraryFile) string {
	if libraryFileSoftDeleted(item) {
		return "deleted"
	}
	return "active"
}

// Keep file-level mutations discoverable from the Task that produced the
// file, even when the action was initiated from the standalone Library card or
// maintenance screen rather than from the Task preview itself.
func fileEventOperationID(item library.LibraryFile) string {
	if operationID := strings.TrimSpace(item.Origin.OperationID); operationID != "" {
		return operationID
	}
	return strings.TrimSpace(item.LatestOperationID)
}

func (service *LibraryService) appendLibraryFileAvailabilityTransition(
	ctx context.Context,
	before library.LibraryFile,
	after library.LibraryFile,
	occurredAt time.Time,
) error {
	wasMissing := strings.TrimSpace(before.State.LastError) == missingLocalFileError
	isMissing := strings.TrimSpace(after.State.LastError) == missingLocalFileError
	if wasMissing == isMissing {
		return nil
	}
	eventType := libraryFileEventMissingDetected
	beforeAvailability := "available"
	afterAvailability := "missing"
	if !isMissing {
		eventType = libraryFileEventAvailableAgain
		beforeAvailability = "missing"
		afterAvailability = "available"
	}
	_, err := service.appendLibraryFileEvent(ctx, appendLibraryFileEventParams{
		EventType:   eventType,
		Category:    "maintenance",
		OperationID: fileEventOperationID(after),
		FileID:      after.ID,
		LibraryID:   after.LibraryID,
		Before:      fileEventSnapshot(before),
		After:       fileEventSnapshot(after),
		Changes: []dto.FileFieldChangeDTO{{
			Field: "availability", Before: beforeAvailability, After: afterAvailability,
		}},
		OccurredAt: occurredAt,
	})
	return err
}
