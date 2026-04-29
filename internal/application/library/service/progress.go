package service

import (
	"context"
	"strings"

	"xiadown/internal/application/events"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	libraryTopicOperation        = "library.operation"
	libraryTopicFile             = "library.file"
	libraryTopicHistory          = "library.history"
	libraryTopicWorkspace        = "library.workspace"
	libraryTopicWorkspaceProject = "library.workspace_project"
	libraryTopicEvent            = "library.file_event"
	libraryEventUpsert           = "upsert"
	libraryEventDelete           = "delete"
)

func (service *LibraryService) publishEvent(topic string, eventType string, payload interface{}) {
	if service == nil || service.bus == nil {
		return
	}
	_ = service.bus.Publish(context.Background(), events.Event{Topic: topic, Type: eventType, Payload: payload})
}

func (service *LibraryService) publishOperationUpdate(item dto.LibraryOperationDTO) {
	service.publishEvent(libraryTopicOperation, libraryEventUpsert, item)
}

func (service *LibraryService) publishOperationDelete(id string) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return
	}
	service.publishEvent(libraryTopicOperation, libraryEventDelete, map[string]string{"id": trimmed})
}

func (service *LibraryService) publishFileUpdate(item dto.LibraryFileDTO) {
	service.publishEvent(libraryTopicFile, libraryEventUpsert, item)
}

func (service *LibraryService) publishHistoryUpdate(item dto.LibraryHistoryRecordDTO) {
	service.publishEvent(libraryTopicHistory, libraryEventUpsert, item)
}

func (service *LibraryService) publishWorkspaceUpdate(item dto.WorkspaceStateRecordDTO) {
	service.publishEvent(libraryTopicWorkspace, libraryEventUpsert, item)
}

func (service *LibraryService) publishFileEventUpdate(item dto.FileEventRecordDTO) {
	service.publishEvent(libraryTopicEvent, libraryEventUpsert, item)
}

func (service *LibraryService) trackCompletedOperation(ctx context.Context, operation library.LibraryOperation) {
	if service == nil || service.telemetry == nil {
		return
	}
	if operation.Status != library.OperationStatusSucceeded {
		return
	}
	service.telemetry.TrackLibraryOperationCompleted(ctx, strings.TrimSpace(operation.ID), strings.TrimSpace(operation.Kind))
}
