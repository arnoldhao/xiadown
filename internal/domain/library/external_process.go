package library

import "time"

type ExternalProcess struct {
	ID             string
	OperationID    string
	Kind           string
	Tool           string
	PID            int
	ProcessGroupID int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
