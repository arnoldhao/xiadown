package service

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/domain/library"
)

const externalProcessKillDelay = 2 * time.Second

func (service *LibraryService) trackExternalProcess(operationID string, kind string, tool string, cmd *exec.Cmd) func() {
	if service == nil || service.processes == nil || cmd == nil || cmd.Process == nil {
		return func() {}
	}
	trimmedOperationID := strings.TrimSpace(operationID)
	trimmedKind := strings.TrimSpace(kind)
	trimmedTool := strings.TrimSpace(tool)
	if trimmedOperationID == "" || trimmedKind == "" || trimmedTool == "" {
		return func() {}
	}

	rootPID, processGroupID := commandProcessIDs(cmd)
	if rootPID <= 0 {
		return func() {}
	}
	now := service.now()
	record := library.ExternalProcess{
		ID:             uuid.NewString(),
		OperationID:    trimmedOperationID,
		Kind:           trimmedKind,
		Tool:           trimmedTool,
		PID:            rootPID,
		ProcessGroupID: processGroupID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 2*time.Second)
	saveErr := service.processes.Save(saveCtx, record)
	saveCancel()
	if saveErr != nil {
		return func() {}
	}

	return func() {
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = service.processes.Delete(deleteCtx, record.ID)
		deleteCancel()
	}
}

func (service *LibraryService) cleanupStaleExternalProcesses(ctx context.Context, operationsByID map[string]library.LibraryOperation) {
	if service == nil || service.processes == nil {
		return
	}
	records, err := service.processes.List(ctx)
	if err != nil {
		return
	}
	for _, record := range records {
		operation, found := operationsByID[strings.TrimSpace(record.OperationID)]
		if !found || operation.Status != library.OperationStatusSucceeded {
			_ = terminateExternalProcessGroup(record.PID, record.ProcessGroupID)
		}
		_ = service.processes.Delete(ctx, record.ID)
	}
}

func startProcessGroupKiller(ctx context.Context, cmd *exec.Cmd, waitDelay time.Duration) func() {
	if ctx == nil {
		ctx = context.Background()
	}
	rootPID, processGroupID := commandProcessIDs(cmd)
	if rootPID <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			timer := time.NewTimer(waitDelay)
			select {
			case <-timer.C:
				_ = terminateExternalProcessGroup(rootPID, processGroupID)
			case <-done:
				if !timer.Stop() {
					<-timer.C
				}
			}
		case <-done:
		}
	}()
	return func() {
		if ctx.Err() != nil {
			_ = terminateExternalProcessGroup(rootPID, processGroupID)
		}
		select {
		case <-done:
		default:
			close(done)
		}
	}
}

func commandProcessIDs(cmd *exec.Cmd) (int, int) {
	if cmd == nil || cmd.Process == nil {
		return 0, 0
	}
	rootPID := cmd.Process.Pid
	processGroupID := processGroupID(cmd)
	if processGroupID <= 0 {
		processGroupID = rootPID
	}
	return rootPID, processGroupID
}
