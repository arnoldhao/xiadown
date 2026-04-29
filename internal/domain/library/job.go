package library

import (
	"strings"
	"time"
)

type OperationStatus string

const (
	OperationStatusQueued    OperationStatus = "queued"
	OperationStatusRunning   OperationStatus = "running"
	OperationStatusSucceeded OperationStatus = "succeeded"
	OperationStatusFailed    OperationStatus = "failed"
	OperationStatusCanceled  OperationStatus = "canceled"
)

type OperationCorrelation struct {
	RequestID         string
	RunID             string
	ParentOperationID string
}

type OperationMeta struct {
	Platform    string
	Uploader    string
	PublishTime string
}

type OperationProgress struct {
	Stage     string
	Percent   *int
	Current   *int64
	Total     *int64
	Speed     string
	Message   string
	UpdatedAt string
}

type OperationOutputFile struct {
	FileID    string
	Kind      string
	Format    string
	SizeBytes *int64
	IsPrimary bool
	Deleted   bool
}

type OperationMetrics struct {
	FileCount      int
	TotalSizeBytes *int64
	DurationMs     *int64
}

type LibraryOperation struct {
	ID           string
	LibraryID    string
	Kind         string
	Status       OperationStatus
	DisplayName  string
	Correlation  OperationCorrelation
	InputJSON    string
	OutputJSON   string
	SourceDomain string
	SourceIcon   string
	Meta         OperationMeta
	Progress     *OperationProgress
	OutputFiles  []OperationOutputFile
	Metrics      OperationMetrics
	ErrorCode    string
	ErrorMessage string
	CreatedAt    time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type LibraryOperationParams struct {
	ID           string
	LibraryID    string
	Kind         string
	Status       string
	DisplayName  string
	Correlation  OperationCorrelation
	InputJSON    string
	OutputJSON   string
	SourceDomain string
	SourceIcon   string
	Meta         OperationMeta
	Progress     *OperationProgress
	OutputFiles  []OperationOutputFile
	Metrics      OperationMetrics
	ErrorCode    string
	ErrorMessage string
	CreatedAt    *time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

func NewLibraryOperation(params LibraryOperationParams) (LibraryOperation, error) {
	id := strings.TrimSpace(params.ID)
	libraryID := strings.TrimSpace(params.LibraryID)
	kind := strings.TrimSpace(params.Kind)
	status := OperationStatus(strings.TrimSpace(params.Status))
	displayName := strings.TrimSpace(params.DisplayName)
	if id == "" || libraryID == "" || displayName == "" {
		return LibraryOperation{}, ErrInvalidLibraryOperation
	}
	switch kind {
	case "download", "transcode":
	default:
		return LibraryOperation{}, ErrInvalidLibraryOperation
	}
	switch status {
	case OperationStatusQueued, OperationStatusRunning, OperationStatusSucceeded, OperationStatusFailed, OperationStatusCanceled:
	default:
		return LibraryOperation{}, ErrInvalidLibraryOperation
	}
	createdAt := time.Now().UTC()
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	return LibraryOperation{
		ID:           id,
		LibraryID:    libraryID,
		Kind:         kind,
		Status:       status,
		DisplayName:  displayName,
		Correlation:  params.Correlation,
		InputJSON:    strings.TrimSpace(params.InputJSON),
		OutputJSON:   strings.TrimSpace(params.OutputJSON),
		SourceDomain: strings.TrimSpace(params.SourceDomain),
		SourceIcon:   strings.TrimSpace(params.SourceIcon),
		Meta:         params.Meta,
		Progress:     params.Progress,
		OutputFiles:  params.OutputFiles,
		Metrics:      params.Metrics,
		ErrorCode:    strings.TrimSpace(params.ErrorCode),
		ErrorMessage: strings.TrimSpace(params.ErrorMessage),
		CreatedAt:    createdAt,
		StartedAt:    params.StartedAt,
		FinishedAt:   params.FinishedAt,
	}, nil
}
