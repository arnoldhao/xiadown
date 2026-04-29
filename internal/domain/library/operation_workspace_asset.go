package library

import "time"

type WorkspaceStateRecord struct {
	ID           string
	LibraryID    string
	StateVersion int
	StateJSON    string
	OperationID  string
	CreatedAt    time.Time
}

type WorkspaceStateRecordParams struct {
	ID           string
	LibraryID    string
	StateVersion int
	StateJSON    string
	OperationID  string
	CreatedAt    *time.Time
}

func NewWorkspaceStateRecord(params WorkspaceStateRecordParams) (WorkspaceStateRecord, error) {
	if params.ID == "" || params.LibraryID == "" || params.StateVersion <= 0 || params.StateJSON == "" {
		return WorkspaceStateRecord{}, ErrInvalidWorkspaceState
	}
	createdAt := time.Now().UTC()
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	return WorkspaceStateRecord{
		ID:           params.ID,
		LibraryID:    params.LibraryID,
		StateVersion: params.StateVersion,
		StateJSON:    params.StateJSON,
		OperationID:  params.OperationID,
		CreatedAt:    createdAt,
	}, nil
}
