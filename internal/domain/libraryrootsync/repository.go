package libraryrootsync

import "context"

type Repository interface {
	ListStates(ctx context.Context) ([]State, error)
	GetState(ctx context.Context, rootID string) (State, error)
	SaveState(ctx context.Context, state State) error
	MarkActiveStatesInterrupted(ctx context.Context) error
	AdvanceWatcherCursor(ctx context.Context, rootID string, cursor uint64) error

	GetEntry(ctx context.Context, rootID, relativePath string) (Entry, error)
	ListEntriesByStatus(ctx context.Context, rootID string, status EntryStatus) ([]Entry, error)
	ListActiveEntriesBySize(ctx context.Context, rootID string, sizeBytes int64) ([]Entry, error)
	FindActiveEntryByDigest(ctx context.Context, rootID string, sizeBytes int64, contentHash string) (Entry, error)
	UpsertEntry(ctx context.Context, entry Entry) error
	MarkUnseenEntriesMissing(ctx context.Context, rootID string, generation int64) (int, error)
	MarkPathMissing(ctx context.Context, rootID, relativePath string, recursive bool, generation int64) (int, error)
}
