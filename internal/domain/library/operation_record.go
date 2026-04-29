package library

import (
	"strings"
	"time"
)

type HistoryRecordSource struct {
	Kind   string
	Caller string
	RunID  string
	Actor  string
}

type HistoryRecordRefs struct {
	OperationID   string
	ImportBatchID string
	FileIDs       []string
	FileEventIDs  []string
}

type ImportRecordMeta struct {
	ImportPath     string
	KeepSourceFile bool
	ImportedAt     string
}

type OperationRecordMeta struct {
	Kind         string
	ErrorCode    string
	ErrorMessage string
}

type HistoryRecord struct {
	ID            string
	LibraryID     string
	Category      string
	Action        string
	DisplayName   string
	Status        string
	Source        HistoryRecordSource
	Refs          HistoryRecordRefs
	Files         []OperationOutputFile
	Metrics       OperationMetrics
	ImportMeta    *ImportRecordMeta
	OperationMeta *OperationRecordMeta
	OccurredAt    time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type HistoryRecordParams struct {
	ID            string
	LibraryID     string
	Category      string
	Action        string
	DisplayName   string
	Status        string
	Source        HistoryRecordSource
	Refs          HistoryRecordRefs
	Files         []OperationOutputFile
	Metrics       OperationMetrics
	ImportMeta    *ImportRecordMeta
	OperationMeta *OperationRecordMeta
	OccurredAt    *time.Time
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
}

func NewHistoryRecord(params HistoryRecordParams) (HistoryRecord, error) {
	id := strings.TrimSpace(params.ID)
	libraryID := strings.TrimSpace(params.LibraryID)
	category := strings.TrimSpace(params.Category)
	action := strings.TrimSpace(params.Action)
	displayName := strings.TrimSpace(params.DisplayName)
	status := strings.TrimSpace(params.Status)
	if id == "" || libraryID == "" || displayName == "" || status == "" {
		return HistoryRecord{}, ErrInvalidHistoryRecord
	}
	switch category {
	case "operation":
		switch action {
		case "download", "transcode":
		default:
			return HistoryRecord{}, ErrInvalidHistoryRecord
		}
	case "import":
		switch action {
		case "import_video":
		default:
			return HistoryRecord{}, ErrInvalidHistoryRecord
		}
	default:
		return HistoryRecord{}, ErrInvalidHistoryRecord
	}
	occurredAt := time.Now().UTC()
	if params.OccurredAt != nil && !params.OccurredAt.IsZero() {
		occurredAt = params.OccurredAt.UTC()
	}
	createdAt := occurredAt
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil && !params.UpdatedAt.IsZero() {
		updatedAt = params.UpdatedAt.UTC()
	}
	return HistoryRecord{
		ID:            id,
		LibraryID:     libraryID,
		Category:      category,
		Action:        action,
		DisplayName:   displayName,
		Status:        status,
		Source:        params.Source,
		Refs:          params.Refs,
		Files:         params.Files,
		Metrics:       params.Metrics,
		ImportMeta:    params.ImportMeta,
		OperationMeta: params.OperationMeta,
		OccurredAt:    occurredAt,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}
