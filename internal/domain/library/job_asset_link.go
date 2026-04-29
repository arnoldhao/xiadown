package library

import "time"

type FileEventRecord struct {
	ID          string
	LibraryID   string
	FileID      string
	OperationID string
	EventType   string
	DetailJSON  string
	CreatedAt   time.Time
}

type FileEventRecordParams struct {
	ID          string
	LibraryID   string
	FileID      string
	OperationID string
	EventType   string
	DetailJSON  string
	CreatedAt   *time.Time
}

func NewFileEventRecord(params FileEventRecordParams) (FileEventRecord, error) {
	if params.ID == "" || params.LibraryID == "" || params.FileID == "" || params.EventType == "" || params.DetailJSON == "" {
		return FileEventRecord{}, ErrInvalidFileEvent
	}
	createdAt := time.Now().UTC()
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	return FileEventRecord{
		ID:          params.ID,
		LibraryID:   params.LibraryID,
		FileID:      params.FileID,
		OperationID: params.OperationID,
		EventType:   params.EventType,
		DetailJSON:  params.DetailJSON,
		CreatedAt:   createdAt,
	}, nil
}
