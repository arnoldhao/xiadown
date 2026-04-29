package telemetryrepo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteStateRepositoryTracksRetentionState(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "data.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	repo := NewSQLiteStateRepository(database.Bun)
	repo.newInstallID = func() string { return "install-1" }

	firstLaunch := time.Date(2026, 4, 1, 9, 30, 0, 0, time.FixedZone("UTC+8", 8*3600))
	state, err := repo.IncrementLaunchCount(ctx, firstLaunch)
	if err != nil {
		t.Fatalf("increment first launch: %v", err)
	}
	if state.LaunchCount != 1 {
		t.Fatalf("unexpected launch count: %d", state.LaunchCount)
	}
	if state.DistinctDaysUsed != 1 || state.DistinctDaysUsedLastMonth != 1 {
		t.Fatalf("unexpected day counters: total=%d lastMonth=%d", state.DistinctDaysUsed, state.DistinctDaysUsedLastMonth)
	}

	state, err = repo.RecordSessionSummary(ctx, firstLaunch.Add(10*time.Minute), 600)
	if err != nil {
		t.Fatalf("record summary: %v", err)
	}
	if state.CompletedSessionCount != 1 || state.TotalSessionSeconds != 600 {
		t.Fatalf("unexpected session totals: count=%d seconds=%f", state.CompletedSessionCount, state.TotalSessionSeconds)
	}
	if state.PreviousSessionSeconds == nil || *state.PreviousSessionSeconds != 600 {
		t.Fatalf("unexpected previous session seconds: %#v", state.PreviousSessionSeconds)
	}

	secondLaunch := firstLaunch.AddDate(0, 0, 1)
	state, err = repo.IncrementLaunchCount(ctx, secondLaunch)
	if err != nil {
		t.Fatalf("increment second launch: %v", err)
	}
	if state.LaunchCount != 2 {
		t.Fatalf("unexpected second launch count: %d", state.LaunchCount)
	}
	if state.DistinctDaysUsed != 2 || state.DistinctDaysUsedLastMonth != 2 {
		t.Fatalf("unexpected second day counters: total=%d lastMonth=%d", state.DistinctDaysUsed, state.DistinctDaysUsedLastMonth)
	}
	if state.CompletedSessionCount != 1 || state.TotalSessionSeconds != 600 {
		t.Fatalf("session totals were not preserved: count=%d seconds=%f", state.CompletedSessionCount, state.TotalSessionSeconds)
	}
}
