package libraryimport

import "context"

// Repository owns the durable batch boundary. ReplaceScan must atomically
// persist the completed dry-run snapshot (batch counts plus all candidates).
type Repository interface {
	CreateBatch(ctx context.Context, batch Batch) (Batch, bool, error)
	ReplaceScan(ctx context.Context, batch Batch, candidates []Candidate) error
	GetBatch(ctx context.Context, id string) (Batch, error)
	GetBatchByRequestKey(ctx context.Context, requestKey string) (Batch, error)
	ListBatches(ctx context.Context, limit int) ([]Batch, error)
	ListCandidates(ctx context.Context, batchID string) ([]Candidate, error)
	SaveBatch(ctx context.Context, batch Batch) error
	SaveCandidate(ctx context.Context, candidate Candidate) error
}
