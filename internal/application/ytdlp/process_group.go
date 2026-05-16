package ytdlp

import (
	"context"
	"os/exec"
	"time"
)

func StartProcessGroupKiller(ctx context.Context, cmd *exec.Cmd, waitDelay time.Duration) func() {
	if cmd == nil {
		return func() {}
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
				_ = terminateProcessGroup(rootPID, processGroupID)
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
			_ = terminateProcessGroup(rootPID, processGroupID)
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
