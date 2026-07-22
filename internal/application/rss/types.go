package rss

import (
	"encoding/json"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

const (
	RSSHubScheme = "rsshub://"
)

var DefaultRSSHubMirrors = []string{
	"https://rsshub.rssforever.com",
	"https://rsshub.umzzz.com",
}

type PreviewSubscriptionRequest struct {
	URL      string `json:"url"`
	ViewType string `json:"viewType,omitempty"`
}

type PreviewSubscriptionResult struct {
	Subscription domainrss.Subscription `json:"subscription"`
	Entries      []domainrss.Entry      `json:"entries"`
	ResolvedURL  string                 `json:"resolvedUrl"`
	PreviewToken string                 `json:"previewToken"`
}

type AddSubscriptionRequest struct {
	URL          string `json:"url"`
	Title        string `json:"title,omitempty"`
	ViewType     string `json:"viewType,omitempty"`
	PreviewToken string `json:"previewToken,omitempty"`
	AllowPending bool   `json:"allowPending,omitempty"`
}

type UpdateSubscriptionRequest struct {
	ID         string  `json:"id"`
	Title      string  `json:"title,omitempty"`
	ViewType   string  `json:"viewType,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
	CategoryID *string `json:"categoryId,omitempty"`
	SortOrder  *int    `json:"sortOrder,omitempty"`
}

type SubscriptionRequest struct {
	ID string `json:"id"`
}

type RefreshRequest struct {
	ID string `json:"id,omitempty"`
}

type RefreshResult struct {
	Subscriptions int `json:"subscriptions"`
	Created       int `json:"created"`
	Updated       int `json:"updated"`
	Failed        int `json:"failed"`
}

type ListEntriesRequest struct {
	SubscriptionID string `json:"subscriptionId,omitempty"`
	CollectionID   string `json:"collectionId,omitempty"`
	CategoryID     string `json:"categoryId,omitempty"`
	SourceKind     string `json:"sourceKind,omitempty"`
	Kind           string `json:"kind,omitempty"`
	Query          string `json:"query,omitempty"`
	UnreadOnly     bool   `json:"unreadOnly,omitempty"`
	StarredOnly    bool   `json:"starredOnly,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	Offset         int    `json:"offset,omitempty"`
}

// MarkAllReadRequest scopes a desktop collection-wide read operation. The
// public device synchronization API deliberately does not expose this bulk
// mutation: paired devices continue to send revision-guarded mutations one
// entry at a time.
type MarkAllReadRequest struct {
	SubscriptionID string `json:"subscriptionId,omitempty"`
	CollectionID   string `json:"collectionId,omitempty"`
	CategoryID     string `json:"categoryId,omitempty"`
	SourceKind     string `json:"sourceKind,omitempty"`
	Kind           string `json:"kind,omitempty"`
	StarredOnly    bool   `json:"starredOnly,omitempty"`
}

type CreateCategoryRequest struct {
	Title     string `json:"title"`
	SortOrder *int   `json:"sortOrder,omitempty"`
}

type UpdateCategoryRequest struct {
	ID               string `json:"id"`
	Title            string `json:"title,omitempty"`
	SortOrder        *int   `json:"sortOrder,omitempty"`
	ExpectedRevision *int64 `json:"expectedRevision,omitempty"`
}

type ReorderRequest struct {
	IDs []string `json:"ids"`
}

type ReorderSubscriptionsRequest struct {
	CategoryID string   `json:"categoryId,omitempty"`
	IDs        []string `json:"ids"`
}

type CreateCollectionRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Kind        string `json:"kind"`
	ViewType    string `json:"viewType,omitempty"`
	SortOrder   *int   `json:"sortOrder,omitempty"`
}

type UpdateCollectionRequest struct {
	ID               string  `json:"id"`
	Title            string  `json:"title,omitempty"`
	Description      *string `json:"description,omitempty"`
	ViewType         string  `json:"viewType,omitempty"`
	SortOrder        *int    `json:"sortOrder,omitempty"`
	ExpectedRevision *int64  `json:"expectedRevision,omitempty"`
}

type ReplaceCollectionItemsRequest struct {
	ID      string   `json:"id"`
	ItemIDs []string `json:"itemIds"`
}

type UpdateCollectionItemsRequest = ReplaceCollectionItemsRequest

type CreateSourceRequest struct {
	Kind      string `json:"kind"`
	Handle    string `json:"handle"`
	Title     string `json:"title"`
	SortOrder *int   `json:"sortOrder,omitempty"`
}

type UpdateSourceRequest struct {
	ID               string `json:"id"`
	Title            string `json:"title,omitempty"`
	Enabled          *bool  `json:"enabled,omitempty"`
	SortOrder        *int   `json:"sortOrder,omitempty"`
	ExpectedRevision *int64 `json:"expectedRevision,omitempty"`
}

type CreateSourceEntryRequest struct {
	SourceID    string     `json:"sourceId"`
	ExternalID  string     `json:"externalId,omitempty"`
	URL         string     `json:"url,omitempty"`
	Title       string     `json:"title"`
	Author      string     `json:"author,omitempty"`
	Summary     string     `json:"summary,omitempty"`
	ContentHTML string     `json:"contentHtml,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

type MarkAllReadResult struct {
	Updated int `json:"updated"`
}

type SetEntryReadRequest struct {
	ID               string `json:"id"`
	Read             bool   `json:"read"`
	ExpectedRevision *int64 `json:"expectedRevision,omitempty"`
	MutationID       string `json:"mutationId,omitempty"`
}

type SetEntryStateRequest struct {
	ID                   string                     `json:"id"`
	Field                domainrss.EntryStateField  `json:"field"`
	Read                 *bool                      `json:"read,omitempty"`
	Starred              *bool                      `json:"starred,omitempty"`
	ArticleProgress      *domainrss.ArticleProgress `json:"articleProgress,omitempty"`
	VideoProgressSeconds *float64                   `json:"videoProgressSeconds,omitempty"`
	VideoDurationSeconds *float64                   `json:"videoDurationSeconds,omitempty"`
	ExpectedRevision     *int64                     `json:"expectedRevision"`
	MutationID           string                     `json:"mutationId"`
}

type ListChangesRequest struct {
	Epoch string `json:"epoch,omitempty"`
	After int64  `json:"after,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type SyncSnapshotRequest struct {
	Epoch     string `json:"epoch"`
	HighWater int64  `json:"highWater"`
	Cursor    string `json:"cursor,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type SyncSnapshotResult struct {
	Records      []domainrss.SyncSnapshotRecord `json:"records"`
	Epoch        string                         `json:"epoch"`
	HighWater    int64                          `json:"highWater"`
	RetainedFrom int64                          `json:"retainedFrom"`
	NextCursor   string                         `json:"nextCursor,omitempty"`
	HasMore      bool                           `json:"hasMore"`
}

type SyncEntryDetail struct {
	domainrss.SyncEntry
	ContentHTML string                    `json:"contentHtml"`
	ImageSlots  []bool                    `json:"imageSlots"`
	MediaSlots  []domainrss.SyncMediaSlot `json:"mediaSlots"`
}

type SyncEntryPage = domainrss.SyncEntryPage

// canonicalStateMutation is used only to derive the idempotency hash. Keeping
// this wire-independent shape here makes whitespace and JSON member ordering
// irrelevant while still binding a key to entry, field, expected revision,
// and value.
type canonicalStateMutation struct {
	EntryID              string                    `json:"entryId"`
	Field                domainrss.EntryStateField `json:"field"`
	ExpectedRevision     int64                     `json:"expectedRevision"`
	Value                json.RawMessage           `json:"value"`
	VideoDurationSeconds *float64                  `json:"videoDurationSeconds,omitempty"`
}

type DiscoveryRequest struct {
	Query        string `json:"query,omitempty"`
	CategoryID   string `json:"categoryId,omitempty"`
	Language     string `json:"language,omitempty"`
	Sort         string `json:"sort,omitempty"`
	Offset       int    `json:"offset,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	ForceRefresh bool   `json:"forceRefresh,omitempty"`
}

type DiscoveryResult struct {
	Categories         []domainrss.DiscoveryCategory `json:"categories"`
	Routes             []domainrss.DiscoveryRoute    `json:"routes"`
	TotalRouteCount    int                           `json:"totalRouteCount"`
	FilteredRouteCount int                           `json:"filteredRouteCount"`
	Offset             int                           `json:"offset"`
	Limit              int                           `json:"limit"`
	HasMore            bool                          `json:"hasMore"`
	SourceURL          string                        `json:"sourceUrl"`
	FetchedAt          string                        `json:"fetchedAt"`
}
