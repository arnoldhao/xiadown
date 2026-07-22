package service

import (
	"context"
	"testing"

	"xiadown/internal/application/library/dto"
)

func TestBeginShutdownRejectsNewRunsAndTaskCreation(t *testing.T) {
	service := &LibraryService{
		runCancels: make(map[string]context.CancelFunc),
		runDone:    make(map[string]chan struct{}),
	}
	service.BeginShutdown()

	if service.registerOperationRun("late-operation", func() {}) {
		t.Fatal("operation registered after the shutdown gate closed")
	}
	if _, err := service.CreateYTDLPJob(context.Background(), dtoCreateYTDLPRequestForShutdownTest()); err == nil {
		t.Fatal("task creation succeeded after the shutdown gate closed")
	}
}

func dtoCreateYTDLPRequestForShutdownTest() dto.CreateYTDLPJobRequest {
	return dto.CreateYTDLPJobRequest{URL: "https://example.com/video"}
}

func TestShutdownActiveRunsClosesAdmissionBeforeTakingSnapshot(t *testing.T) {
	service := &LibraryService{
		runCancels: make(map[string]context.CancelFunc),
		runDone:    make(map[string]chan struct{}),
	}
	cancelled := make(chan struct{})
	if !service.registerOperationRun("active-operation", func() { close(cancelled) }) {
		t.Fatal("failed to register active operation")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := service.ShutdownActiveRuns(ctx); got != 1 {
		t.Fatalf("ShutdownActiveRuns() = %d, want 1", got)
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("active operation was not cancelled")
	}
	if service.registerOperationRun("late-operation", func() {}) {
		t.Fatal("operation registered after ShutdownActiveRuns began")
	}
}
