package service

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

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
	return service.trackExternalProcessIDs(trimmedOperationID, trimmedKind, trimmedTool, rootPID, processGroupID)
}

func (service *LibraryService) trackExternalProcessIDs(operationID string, kind string, tool string, rootPID int, processGroupID int) func() {
	if service == nil || service.processes == nil {
		return func() {}
	}
	if rootPID <= 0 {
		return func() {}
	}
	trimmedOperationID := strings.TrimSpace(operationID)
	trimmedKind := strings.TrimSpace(kind)
	trimmedTool := strings.TrimSpace(tool)
	if trimmedOperationID == "" || trimmedKind == "" || trimmedTool == "" {
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
		zap.L().Warn(
			"external process tracking save failed",
			zap.String("operationID", trimmedOperationID),
			zap.String("kind", trimmedKind),
			zap.String("tool", trimmedTool),
			zap.Int("pid", rootPID),
			zap.Int("processGroupID", processGroupID),
			zap.Error(saveErr),
		)
		return func() {}
	}
	zap.L().Info(
		"external process tracking saved",
		zap.String("recordID", record.ID),
		zap.String("operationID", trimmedOperationID),
		zap.String("kind", trimmedKind),
		zap.String("tool", trimmedTool),
		zap.Int("pid", rootPID),
		zap.Int("processGroupID", processGroupID),
	)

	return func() {
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 2*time.Second)
		deleteErr := service.processes.Delete(deleteCtx, record.ID)
		deleteCancel()
		if deleteErr != nil {
			zap.L().Warn(
				"external process tracking delete failed",
				zap.String("recordID", record.ID),
				zap.String("operationID", trimmedOperationID),
				zap.String("kind", trimmedKind),
				zap.String("tool", trimmedTool),
				zap.Int("pid", rootPID),
				zap.Int("processGroupID", processGroupID),
				zap.Error(deleteErr),
			)
			return
		}
		zap.L().Info(
			"external process tracking deleted",
			zap.String("recordID", record.ID),
			zap.String("operationID", trimmedOperationID),
			zap.String("kind", trimmedKind),
			zap.String("tool", trimmedTool),
			zap.Int("pid", rootPID),
			zap.Int("processGroupID", processGroupID),
		)
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
