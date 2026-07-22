package libraryimportrepo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	importdomain "xiadown/internal/domain/libraryimport"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteRepositoryPersistsIdempotentDryRunSnapshot(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: filepath.Join(t.TempDir(), "imports.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := persistence.ApplyLibraryImportSchema(ctx, database.SQL); err != nil {
		t.Fatal(err)
	}
	repository := NewSQLiteRepository(database.Bun)
	now := time.Now().UTC()
	batch, err := importdomain.NewBatch(importdomain.Batch{
		ID: "batch-1", RequestKey: "request-1", LibraryID: "library-1",
		Mode: importdomain.ModeReferenced, HiddenPolicy: importdomain.HiddenExclude,
		SymlinkPolicy: importdomain.SymlinkSkip, Status: importdomain.BatchScanning,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	created, wasCreated, err := repository.CreateBatch(ctx, batch)
	if err != nil || !wasCreated || created.ID != batch.ID {
		t.Fatalf("create batch: %+v, created=%v, err=%v", created, wasCreated, err)
	}
	second, wasCreated, err := repository.CreateBatch(ctx, importdomain.Batch{
		ID: "different", RequestKey: batch.RequestKey, LibraryID: batch.LibraryID,
		Mode: batch.Mode, HiddenPolicy: batch.HiddenPolicy, SymlinkPolicy: batch.SymlinkPolicy,
		Status: importdomain.BatchScanning, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil || wasCreated || second.ID != batch.ID {
		t.Fatalf("request idempotency failed: %+v, created=%v, err=%v", second, wasCreated, err)
	}
	candidate, err := importdomain.NewCandidate(importdomain.Candidate{
		ID: "candidate-1", BatchID: batch.ID, SourcePath: "/selected/movie.mp4", DisplayName: "movie.mp4",
		Extension: ".mp4", Category: importdomain.CategoryVideo, SizeBytes: 3,
		HashAlgorithm: "sha256", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Status: importdomain.CandidateReady, ModifiedAt: now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Status = importdomain.BatchReady
	batch.Counts = importdomain.CountsFor([]importdomain.Candidate{candidate})
	batch.UpdatedAt = now.Add(time.Second)
	if err := repository.ReplaceScan(ctx, batch, []importdomain.Candidate{candidate}); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.GetBatch(ctx, batch.ID)
	if err != nil || loaded.Counts.Total != 1 || loaded.Counts.Ready != 1 || loaded.Status != importdomain.BatchReady {
		t.Fatalf("unexpected loaded batch: %+v, err=%v", loaded, err)
	}
	candidates, err := repository.ListCandidates(ctx, batch.ID)
	if err != nil || len(candidates) != 1 || candidates[0].ContentHash != candidate.ContentHash {
		t.Fatalf("unexpected candidates: %+v, err=%v", candidates, err)
	}
}
