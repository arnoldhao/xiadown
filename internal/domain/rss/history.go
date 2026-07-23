package rss

import (
	"context"
	"time"
)

type HistoryCapability string

const (
	HistoryCapabilityUnknown     HistoryCapability = "unknown"
	HistoryCapabilityAvailable   HistoryCapability = "available"
	HistoryCapabilityUnsupported HistoryCapability = "unsupported"
)

// SubscriptionHistoryState is local fetch machinery, not synchronized user
// data. CursorURL can contain an opaque publisher token and must never be
// projected through Wails, public HTTP, logs, or RSS change records.
type SubscriptionHistoryState struct {
	SubscriptionID string
	CursorURL      string
	Capability     HistoryCapability
	Exhausted      bool
	NoProgress     int
	LastAttemptAt  *time.Time
	LastSuccessAt  *time.Time
	LastError      string
	UpdatedAt      time.Time
}

// HistoricalUpsertResult separates the entries written to the local archive
// from the entries visible in the collection that requested the backfill.
// Total drives RFC 5005 cursor progress; Visible is projected to the caller so
// a kind-scoped list never mistakes unrelated persisted entries for progress.
type HistoricalUpsertResult struct {
	Total   UpsertResult
	Visible UpsertResult
}

// HistoryRepository stays separate from Repository so non-SQLite and narrow
// state test doubles retain the existing desktop RSS contract.
type HistoryRepository interface {
	GetSubscriptionHistory(context.Context, string) (SubscriptionHistoryState, error)
	PutSubscriptionHistory(context.Context, SubscriptionHistoryState) error
	UpsertHistoricalFeed(context.Context, FeedUpdate, EntryKind) (HistoricalUpsertResult, error)
}
