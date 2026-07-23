package libraryimport

import (
	"errors"
	"testing"
	"time"
)

func TestBatchAndCandidateStateMachines(t *testing.T) {
	now := time.Now().UTC()
	batch, err := NewBatch(Batch{
		ID: "batch", RequestKey: "request", Mode: ModeReferenced,
		HiddenPolicy: HiddenExclude, SymlinkPolicy: SymlinkSkip,
		Status: BatchScanning, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !batch.CanTransition(BatchReady) || batch.CanTransition(BatchCompleted) {
		t.Fatal("unexpected batch transition policy")
	}
	candidate, err := NewCandidate(Candidate{
		ID: "candidate", BatchID: batch.ID, SourcePath: "/tmp/movie.mp4", DisplayName: "movie.mp4",
		Category: CategoryVideo, SizeBytes: 1, HashAlgorithm: "sha256",
		ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Status:      CandidateReady, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !candidate.CanTransition(CandidateImporting) || candidate.CanTransition(CandidateSucceeded) {
		t.Fatal("unexpected candidate transition policy")
	}
}

func TestCopyBatchRequiresManagedRoot(t *testing.T) {
	_, err := NewBatch(Batch{
		ID: "batch", RequestKey: "request", Mode: ModeCopy,
		HiddenPolicy: HiddenExclude, SymlinkPolicy: SymlinkSkip, Status: BatchScanning,
	})
	if !errors.Is(err, ErrManagedRootMissing) {
		t.Fatalf("expected managed root error, got %v", err)
	}
}

func TestCountsForKeepsRetryableWorkVisible(t *testing.T) {
	items := []Candidate{
		{Status: CandidateReady, SizeBytes: 10},
		{Status: CandidateRegistered, SizeBytes: 20},
		{Status: CandidateSucceeded, SizeBytes: 30},
		{Status: CandidateDuplicate, SizeBytes: 30},
	}
	counts := CountsFor(items)
	if counts.Total != 4 || counts.Ready != 2 || counts.Succeeded != 1 || counts.Duplicate != 1 || counts.TotalBytes != 90 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
}
