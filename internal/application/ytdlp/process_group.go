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
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			timer := time.NewTimer(waitDelay)
			select {
			case <-timer.C:
				_ = terminateProcessGroup(cmd)
			case <-done:
				if !timer.Stop() {
					<-timer.C
				}
			}
		case <-done:
		}
	}()
	return func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}
}
