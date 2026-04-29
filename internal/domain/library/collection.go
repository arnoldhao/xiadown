package library

import (
	"strings"
	"time"
)

type CreateMeta struct {
	Source             string
	TriggerOperationID string
	ImportBatchID      string
	Actor              string
}

type RetentionConfig struct {
	WorkspaceStatesMax int
	FileEventsMax      int
	HistoryMax         int
	OperationLogsMax   int
}

type WorkspaceConfig struct {
	FastReadLatestState bool
}

type ModuleConfig struct {
	Retention RetentionConfig
	Workspace WorkspaceConfig
}

type Library struct {
	ID        string
	Name      string
	CreatedBy CreateMeta
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LibraryParams struct {
	ID        string
	Name      string
	CreatedBy CreateMeta
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

func DefaultModuleConfig() ModuleConfig {
	return ModuleConfig{
		Retention: RetentionConfig{
			WorkspaceStatesMax: 20,
			FileEventsMax:      200,
			HistoryMax:         200,
			OperationLogsMax:   50,
		},
		Workspace: WorkspaceConfig{FastReadLatestState: true},
	}
}

func NormalizeModuleConfig(config ModuleConfig) ModuleConfig {
	defaults := DefaultModuleConfig()
	result := config
	if result.Retention.WorkspaceStatesMax <= 0 {
		result.Retention = defaults.Retention
	}
	return result
}

func NewLibrary(params LibraryParams) (Library, error) {
	id := strings.TrimSpace(params.ID)
	name := strings.TrimSpace(params.Name)
	if id == "" || name == "" {
		return Library{}, ErrInvalidLibrary
	}
	createdAt := time.Now().UTC()
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil && !params.UpdatedAt.IsZero() {
		updatedAt = params.UpdatedAt.UTC()
	}
	meta := params.CreatedBy
	meta.Source = strings.TrimSpace(meta.Source)
	meta.TriggerOperationID = strings.TrimSpace(meta.TriggerOperationID)
	meta.ImportBatchID = strings.TrimSpace(meta.ImportBatchID)
	meta.Actor = strings.TrimSpace(meta.Actor)
	return Library{
		ID:        id,
		Name:      name,
		CreatedBy: meta,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}
