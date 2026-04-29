package library

import (
	"strings"
	"time"
)

type OperationChunkStatus string

const (
	OperationChunkStatusQueued    OperationChunkStatus = "queued"
	OperationChunkStatusRunning   OperationChunkStatus = "running"
	OperationChunkStatusSucceeded OperationChunkStatus = "succeeded"
	OperationChunkStatusFailed    OperationChunkStatus = "failed"
	OperationChunkStatusCanceled  OperationChunkStatus = "canceled"
)

type OperationChunk struct {
	ID           string
	OperationID  string
	LibraryID    string
	ChunkIndex   int
	Status       OperationChunkStatus
	SourceRange  string
	InputHash    string
	RequestHash  string
	PromptHash   string
	ResponseHash string
	ResultJSON   string
	UsageJSON    string
	RetryCount   int
	ErrorMessage string
	StartedAt    *time.Time
	FinishedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type OperationChunkParams struct {
	ID           string
	OperationID  string
	LibraryID    string
	ChunkIndex   int
	Status       string
	SourceRange  string
	InputHash    string
	RequestHash  string
	PromptHash   string
	ResponseHash string
	ResultJSON   string
	UsageJSON    string
	RetryCount   int
	ErrorMessage string
	StartedAt    *time.Time
	FinishedAt   *time.Time
	CreatedAt    *time.Time
	UpdatedAt    *time.Time
}

func NewOperationChunk(params OperationChunkParams) (OperationChunk, error) {
	id := strings.TrimSpace(params.ID)
	operationID := strings.TrimSpace(params.OperationID)
	libraryID := strings.TrimSpace(params.LibraryID)
	status := OperationChunkStatus(strings.TrimSpace(params.Status))
	if id == "" || operationID == "" || libraryID == "" || params.ChunkIndex <= 0 {
		return OperationChunk{}, ErrInvalidOperationChunk
	}
	switch status {
	case OperationChunkStatusQueued, OperationChunkStatusRunning, OperationChunkStatusSucceeded, OperationChunkStatusFailed, OperationChunkStatusCanceled:
	default:
		return OperationChunk{}, ErrInvalidOperationChunk
	}
	createdAt := time.Now().UTC()
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil && !params.UpdatedAt.IsZero() {
		updatedAt = params.UpdatedAt.UTC()
	}
	return OperationChunk{
		ID:           id,
		OperationID:  operationID,
		LibraryID:    libraryID,
		ChunkIndex:   params.ChunkIndex,
		Status:       status,
		SourceRange:  strings.TrimSpace(params.SourceRange),
		InputHash:    strings.TrimSpace(params.InputHash),
		RequestHash:  strings.TrimSpace(params.RequestHash),
		PromptHash:   strings.TrimSpace(params.PromptHash),
		ResponseHash: strings.TrimSpace(params.ResponseHash),
		ResultJSON:   strings.TrimSpace(params.ResultJSON),
		UsageJSON:    strings.TrimSpace(params.UsageJSON),
		RetryCount:   params.RetryCount,
		ErrorMessage: strings.TrimSpace(params.ErrorMessage),
		StartedAt:    params.StartedAt,
		FinishedAt:   params.FinishedAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}
