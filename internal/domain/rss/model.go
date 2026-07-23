package rss

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("rss item not found")
	ErrDuplicateFeed       = errors.New("rss subscription already exists")
	ErrRevisionConflict    = errors.New("rss state revision conflict")
	ErrSyncResetRequired   = errors.New("rss synchronization reset required")
	ErrIdempotencyConflict = errors.New("rss mutation idempotency conflict")
	ErrInvalidRequest      = errors.New("invalid rss request")
)

type EntryKind string

const (
	EntryKindArticle EntryKind = "article"
	EntryKindSocial  EntryKind = "social"
	EntryKindImage   EntryKind = "image"
	EntryKindVideo   EntryKind = "video"
)

type ViewType string

const (
	ViewTypeAuto    ViewType = "auto"
	ViewTypeArticle ViewType = "article"
	ViewTypeSocial  ViewType = "social"
	ViewTypeImage   ViewType = "image"
	ViewTypeVideo   ViewType = "video"
)

// SubscriptionSourceAccess describes where a feed descriptor may be used.
// Device-local subscriptions intentionally never enter the Desktop namespace;
// the value is retained here so both sides share one wire vocabulary.
type SubscriptionSourceAccess string

const (
	SubscriptionSourceDeviceLocal    SubscriptionSourceAccess = "deviceLocal"
	SubscriptionSourceDesktopManaged SubscriptionSourceAccess = "desktopManaged"
	SubscriptionSourceSharedPublic   SubscriptionSourceAccess = "sharedPublic"
)

type Subscription struct {
	ID           string                   `json:"id"`
	WorkspaceID  string                   `json:"workspaceId"`
	FeedURL      string                   `json:"feedUrl"`
	SourceAccess SubscriptionSourceAccess `json:"sourceAccess"`
	// PublicFeedURL is a separately-authorized descriptor. It must only be
	// projected to a device that currently has rss.fetch.
	PublicFeedURL    string     `json:"publicFeedURL,omitempty"`
	SiteURL          string     `json:"siteUrl,omitempty"`
	Title            string     `json:"title"`
	Description      string     `json:"description,omitempty"`
	IconURL          string     `json:"iconUrl,omitempty"`
	ViewType         ViewType   `json:"viewType"`
	ResolvedViewType ViewType   `json:"resolvedViewType,omitempty"`
	CategoryID       string     `json:"categoryId,omitempty"`
	SortOrder        int        `json:"sortOrder"`
	Enabled          bool       `json:"enabled"`
	UnreadCount      int        `json:"unreadCount"`
	ETag             string     `json:"-"`
	LastModified     string     `json:"-"`
	ValidatorURL     string     `json:"-"`
	LastFetchedAt    *time.Time `json:"lastFetchedAt,omitempty"`
	LastSuccessAt    *time.Time `json:"lastSuccessAt,omitempty"`
	LastError        string     `json:"lastError,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	Revision         int64      `json:"revision"`
}

type Entry struct {
	ID             string `json:"id"`
	SubscriptionID string `json:"subscriptionId"`
	ExternalID     string `json:"externalId"`
	// OriginKey is the versioned cross-device feed identity. It is persisted in
	// the dedicated origin mapping, not exposed by canonical entry DTOs.
	OriginKey            string              `json:"-"`
	ObservedAt           time.Time           `json:"-"`
	URL                  string              `json:"url,omitempty"`
	Title                string              `json:"title"`
	Author               string              `json:"author,omitempty"`
	Summary              string              `json:"summary,omitempty"`
	ContentHTML          string              `json:"contentHtml,omitempty"`
	Kind                 EntryKind           `json:"kind"`
	ImageURLs            []string            `json:"imageUrls"`
	Media                []Media             `json:"media"`
	MediaURL             string              `json:"mediaUrl,omitempty"`
	MediaType            string              `json:"mediaType,omitempty"`
	ThumbnailURL         string              `json:"thumbnailUrl,omitempty"`
	Platform             string              `json:"platform,omitempty"`
	PlatformVideoID      string              `json:"platformVideoId,omitempty"`
	PlaybackURL          string              `json:"playbackUrl,omitempty"`
	DownloadTarget       string              `json:"downloadTarget,omitempty"`
	PublishedAt          *time.Time          `json:"publishedAt,omitempty"`
	SourceUpdatedAt      *time.Time          `json:"sourceUpdatedAt,omitempty"`
	ReadAt               *time.Time          `json:"readAt,omitempty"`
	StarredAt            *time.Time          `json:"starredAt,omitempty"`
	ArticleProgress      *ArticleProgress    `json:"articleProgress,omitempty"`
	VideoProgressSeconds *float64            `json:"videoProgressSeconds,omitempty"`
	VideoDurationSeconds *float64            `json:"videoDurationSeconds,omitempty"`
	VideoCompleted       bool                `json:"videoCompleted"`
	FieldRevisions       StateFieldRevisions `json:"fieldRevisions"`
	StateRevision        int64               `json:"stateRevision"`
	ReadStateUpdatedAt   *time.Time          `json:"readStateUpdatedAt,omitempty"`
	Revision             int64               `json:"revision"`
	CreatedAt            time.Time           `json:"createdAt"`
	ModifiedAt           time.Time           `json:"modifiedAt"`
	ContentHash          string              `json:"-"`
}

type ArticleProgress struct {
	Fraction        float64 `json:"fraction"`
	Anchor          string  `json:"anchor,omitempty"`
	ContentRevision int64   `json:"contentRevision"`
}

type StateFieldRevisions struct {
	Read                 int64 `json:"read"`
	Starred              int64 `json:"starred"`
	ArticleProgress      int64 `json:"articleProgress"`
	VideoProgressSeconds int64 `json:"videoProgressSeconds"`
}

type Media struct {
	URL       string `json:"url"`
	MIMEType  string `json:"mimeType,omitempty"`
	Kind      string `json:"kind"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Duration  int64  `json:"durationMs,omitempty"`
}

type EntryState struct {
	EntryID              string              `json:"entryId"`
	SubjectID            string              `json:"subjectId"`
	Read                 bool                `json:"read"`
	ReadAt               *time.Time          `json:"readAt,omitempty"`
	Starred              bool                `json:"starred"`
	StarredAt            *time.Time          `json:"starredAt,omitempty"`
	ArticleProgress      *ArticleProgress    `json:"articleProgress,omitempty"`
	VideoProgressSeconds *float64            `json:"videoProgressSeconds,omitempty"`
	VideoDurationSeconds *float64            `json:"videoDurationSeconds,omitempty"`
	VideoCompleted       bool                `json:"videoCompleted"`
	FieldRevisions       StateFieldRevisions `json:"fieldRevisions"`
	Revision             int64               `json:"revision"`
	UpdatedAt            time.Time           `json:"updatedAt"`
	UpdatedBy            string              `json:"updatedBy,omitempty"`
	MutationID           string              `json:"mutationId,omitempty"`
}

type EntryStateField string

const (
	EntryStateFieldRead                 EntryStateField = "read"
	EntryStateFieldStarred              EntryStateField = "starred"
	EntryStateFieldArticleProgress      EntryStateField = "articleProgress"
	EntryStateFieldVideoProgressSeconds EntryStateField = "videoProgressSeconds"
)

type EntryQuery struct {
	SubscriptionID string
	CollectionID   string
	CategoryID     string
	SourceKind     SourceKind
	Kind           EntryKind
	Query          string
	UnreadOnly     bool
	StarredOnly    bool
	Limit          int
	Offset         int
}

type EntryPage struct {
	Items      []Entry `json:"items"`
	Total      int     `json:"total"`
	NextOffset int     `json:"nextOffset,omitempty"`
	Snapshot   int64   `json:"snapshot"`
}

type FeedUpdate struct {
	Subscription Subscription
	Entries      []Entry
}

type UpsertResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
}

type ReadMutation struct {
	EntryID          string
	Read             bool
	ExpectedRevision *int64
	DeviceID         string
	MutationID       string
	ChangedAt        time.Time
}

type StateMutation struct {
	Scope                SyncScope
	EntryID              string
	Field                EntryStateField
	Read                 *bool
	Starred              *bool
	ArticleProgress      *ArticleProgress
	VideoProgressSeconds *float64
	VideoDurationSeconds *float64
	ExpectedRevision     int64
	DeviceID             string
	MutationID           string
	RequestHash          string
	ChangedAt            time.Time
	// AllowDesktopLocal is set only by the trusted desktop bridge. Paired
	// clients must never use source-backed Inbox/notification entries by ID.
	AllowDesktopLocal bool
}

type Change struct {
	Sequence   int64           `json:"sequence"`
	EntityType string          `json:"entityType"`
	EntityID   string          `json:"entityId"`
	Operation  string          `json:"operation"`
	Revision   int64           `json:"revision"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	ChangedAt  time.Time       `json:"changedAt"`
}

type ChangePage struct {
	Changes      []Change `json:"changes"`
	Epoch        string   `json:"epoch"`
	Cursor       int64    `json:"cursor"`
	HighWater    int64    `json:"highWater"`
	RetainedFrom int64    `json:"retainedFrom"`
	HasMore      bool     `json:"hasMore"`
}

type SyncScope struct {
	WorkspaceID string
	SubjectID   string
}

type SyncPosition struct {
	Epoch        string `json:"epoch"`
	Cursor       int64  `json:"cursor"`
	RetainedFrom int64  `json:"retainedFrom"`
}

type SyncResetError struct {
	Position SyncPosition
}

func (err *SyncResetError) Error() string { return ErrSyncResetRequired.Error() }
func (err *SyncResetError) Unwrap() error { return ErrSyncResetRequired }

type StateConflictError struct {
	State EntryState
}

func (err *StateConflictError) Error() string { return ErrRevisionConflict.Error() }
func (err *StateConflictError) Unwrap() error { return ErrRevisionConflict }

type SyncOverview struct {
	CatalogID    string   `json:"catalogId"`
	WorkspaceID  string   `json:"workspaceId"`
	SubjectID    string   `json:"subjectId"`
	Epoch        string   `json:"epoch"`
	HighWater    int64    `json:"highWater"`
	RetainedFrom int64    `json:"retainedFrom"`
	Capabilities []string `json:"capabilities"`
}

// SyncSubscription is the device-facing projection. It never contains the
// private FeedURL, conditional request metadata, or refresh errors. The
// separate PublicFeedURL is populated only at an rss.fetch-aware HTTP boundary.
type SyncSubscription struct {
	ID            string                   `json:"id"`
	WorkspaceID   string                   `json:"workspaceId"`
	Title         string                   `json:"title"`
	Description   string                   `json:"description,omitempty"`
	IconAvailable bool                     `json:"iconAvailable"`
	ViewType      ViewType                 `json:"viewType"`
	CategoryID    string                   `json:"categoryId,omitempty"`
	SortOrder     int                      `json:"sortOrder"`
	Enabled       bool                     `json:"enabled"`
	UnreadCount   int                      `json:"unreadCount"`
	CreatedAt     time.Time                `json:"createdAt"`
	UpdatedAt     time.Time                `json:"updatedAt"`
	Revision      int64                    `json:"revision"`
	SourceAccess  SubscriptionSourceAccess `json:"sourceAccess"`
	PublicFeedURL string                   `json:"publicFeedURL,omitempty"`
}

type SubscriptionMutationOperation string

const (
	SubscriptionMutationAdd     SubscriptionMutationOperation = "add"
	SubscriptionMutationPromote SubscriptionMutationOperation = "promote"
	SubscriptionMutationUpdate  SubscriptionMutationOperation = "update"
	SubscriptionMutationDelete  SubscriptionMutationOperation = "delete"
)

// SubscriptionMutation is the normalized, hash-bound command applied by the
// Desktop canonical RSS repository. FieldMask is required for updates so
// unrelated metadata can be merged without accidentally clearing fields.
type SubscriptionMutation struct {
	DeviceID         string
	MutationID       string
	RequestHash      string
	Operation        SubscriptionMutationOperation
	SubscriptionID   string
	ExpectedRevision *int64
	FieldMask        []string
	Title            string
	ViewType         ViewType
	CategoryID       *string
	SortOrder        *int
	Enabled          *bool
	SourceAccess     SubscriptionSourceAccess
	PublicFeedURL    string
	ChangedAt        time.Time
}

type SubscriptionMutationResult struct {
	MutationID   string            `json:"mutationId"`
	Operation    string            `json:"operation"`
	Subscription *SyncSubscription `json:"subscription,omitempty"`
	DeletedID    string            `json:"deletedId,omitempty"`
	Revision     int64             `json:"revision"`
	ChangeCursor int64             `json:"changeCursor"`
}

type ObservationEnclosure struct {
	URL          string `json:"url"`
	MIMEType     string `json:"mimeType,omitempty"`
	ByteLength   int64  `json:"byteLength,omitempty"`
	ThumbnailURL string `json:"thumbnailURL,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	DurationMS   int64  `json:"durationMs,omitempty"`
}

type ObservationEntry struct {
	OriginKey     string                 `json:"originKey"`
	GUID          string                 `json:"guid,omitempty"`
	CanonicalLink string                 `json:"canonicalLink,omitempty"`
	PublishedAt   *time.Time             `json:"publishedAt,omitempty"`
	UpdatedAt     *time.Time             `json:"updatedAt,omitempty"`
	Title         string                 `json:"title"`
	Author        string                 `json:"author,omitempty"`
	Summary       string                 `json:"summary,omitempty"`
	ContentHTML   string                 `json:"contentHTML,omitempty"`
	Enclosures    []ObservationEnclosure `json:"enclosures,omitempty"`
}

type FeedObservation struct {
	DeviceID         string
	MutationID       string
	RequestHash      string
	SubscriptionID   string
	UpstreamETag     string
	LastModified     string
	FetchedAt        time.Time
	AcceptedAt       time.Time
	ContentHash      string
	Entries          []ObservationEntry
	CanonicalEntries []Entry
}

type OriginEntryMapping struct {
	OriginKey       string `json:"originKey"`
	EntryID         string `json:"entryId"`
	ContentRevision int64  `json:"contentRevision"`
}

type ObservationResult struct {
	MutationID   string               `json:"mutationId"`
	AcceptedAt   time.Time            `json:"acceptedAt"`
	Created      int                  `json:"created"`
	Updated      int                  `json:"updated"`
	Mappings     []OriginEntryMapping `json:"mappings"`
	ChangeCursor int64                `json:"changeCursor"`
}

type FetchLeaseRequest struct {
	DeviceID       string
	SubscriptionID string
	LeaseID        string
	RequestedTTL   time.Duration
	RequestedAt    time.Time
}

type FetchLeaseResult struct {
	LeaseID           string    `json:"leaseId,omitempty"`
	Granted           bool      `json:"granted"`
	ExpiresAt         time.Time `json:"expiresAt,omitempty"`
	RetryAfterSeconds int       `json:"retryAfterSeconds,omitempty"`
}

type SyncEntry struct {
	ID                   string              `json:"id"`
	SubscriptionID       string              `json:"subscriptionId"`
	Title                string              `json:"title"`
	Author               string              `json:"author,omitempty"`
	Summary              string              `json:"summary,omitempty"`
	Kind                 EntryKind           `json:"kind"`
	ThumbnailAvailable   bool                `json:"thumbnailAvailable"`
	Platform             string              `json:"platform,omitempty"`
	PlatformVideoID      string              `json:"platformVideoId,omitempty"`
	PublishedAt          *time.Time          `json:"publishedAt,omitempty"`
	SourceUpdatedAt      *time.Time          `json:"sourceUpdatedAt,omitempty"`
	Read                 bool                `json:"read"`
	ReadAt               *time.Time          `json:"readAt,omitempty"`
	Starred              bool                `json:"starred"`
	StarredAt            *time.Time          `json:"starredAt,omitempty"`
	ArticleProgress      *ArticleProgress    `json:"articleProgress,omitempty"`
	VideoProgressSeconds *float64            `json:"videoProgressSeconds,omitempty"`
	VideoDurationSeconds *float64            `json:"videoDurationSeconds,omitempty"`
	VideoCompleted       bool                `json:"videoCompleted"`
	FieldRevisions       StateFieldRevisions `json:"fieldRevisions"`
	StateRevision        int64               `json:"stateRevision"`
	ContentRevision      int64               `json:"contentRevision"`
	CreatedAt            time.Time           `json:"createdAt"`
	ModifiedAt           time.Time           `json:"modifiedAt"`
}

type SyncEntryPage struct {
	Items      []SyncEntry `json:"items"`
	Total      int         `json:"total"`
	NextOffset int         `json:"nextOffset,omitempty"`
}

type SyncMediaSlot struct {
	Available          bool   `json:"available"`
	ThumbnailAvailable bool   `json:"thumbnailAvailable"`
	MIMEType           string `json:"mimeType,omitempty"`
	Kind               string `json:"kind"`
	Width              int    `json:"width,omitempty"`
	Height             int    `json:"height,omitempty"`
	Duration           int64  `json:"durationMs,omitempty"`
}

type SyncSnapshotRecord struct {
	EntityType string          `json:"entityType"`
	EntityID   string          `json:"entityId"`
	Revision   int64           `json:"revision"`
	Payload    json.RawMessage `json:"payload"`
}

type SyncSnapshotQuery struct {
	Scope     SyncScope
	Epoch     string
	HighWater int64
	Stage     string
	AfterID   string
	Limit     int
}

type SyncSnapshotPage struct {
	Records      []SyncSnapshotRecord
	Epoch        string
	HighWater    int64
	RetainedFrom int64
	NextStage    string
	NextID       string
	HasMore      bool
}

type SyncChangeQuery struct {
	Scope SyncScope
	Epoch string
	After int64
	Limit int
}

// DiscoveryCategory and DiscoveryRoute are the persisted, transport-neutral
// representation of the RSSHub catalog. Parameterized routes retain their
// template and structured form metadata; ExampleValue is display-only and is
// never treated as a submitted default.
type DiscoveryCategory struct {
	ID        string   `json:"id"`
	Count     int      `json:"count"`
	Examples  []string `json:"examples"`
	IconURL   string   `json:"iconUrl"`
	IconLabel string   `json:"iconLabel"`
}

type DiscoveryParameterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type DiscoveryParameter struct {
	Name         string                     `json:"name"`
	Description  string                     `json:"description"`
	DefaultValue *string                    `json:"defaultValue"`
	ExampleValue string                     `json:"exampleValue"`
	Optional     bool                       `json:"optional"`
	CatchAll     bool                       `json:"catchAll"`
	Type         string                     `json:"type"`
	Options      []DiscoveryParameterOption `json:"options"`
}

type DiscoveryRoute struct {
	ID                string               `json:"id"`
	Title             string               `json:"title"`
	URL               string               `json:"url"`
	IconURL           string               `json:"iconUrl"`
	Provider          string               `json:"provider"`
	Description       string               `json:"description"`
	SourceID          string               `json:"sourceId"`
	SourceName        string               `json:"sourceName"`
	SourceURL         string               `json:"sourceUrl"`
	SiteURL           string               `json:"siteUrl"`
	RoutePath         string               `json:"routePath"`
	ExamplePath       string               `json:"examplePath"`
	Categories        []string             `json:"categories"`
	Heat              int                  `json:"heat"`
	Language          string               `json:"language"`
	Region            string               `json:"region"`
	ViewType          ViewType             `json:"viewType"`
	RequiresConfig    bool                 `json:"requiresConfig"`
	RequiresPuppeteer bool                 `json:"requiresPuppeteer"`
	NeedsParameters   bool                 `json:"needsParameters"`
	Parameters        []DiscoveryParameter `json:"parameters"`
}

type DiscoveryCache struct {
	Routes    []DiscoveryRoute
	SourceURL string
	FetchedAt time.Time
}

type DiscoveryState struct {
	SourceURL  string
	FetchedAt  time.Time
	RouteCount int
}

type DiscoveryQuery struct {
	Query      string
	RouteID    string
	CategoryID string
	Language   string
	Sort       string
	Offset     int
	Limit      int
}

type DiscoveryPage struct {
	State              DiscoveryState
	Categories         []DiscoveryCategory
	Routes             []DiscoveryRoute
	FilteredRouteCount int
	Offset             int
	Limit              int
	HasMore            bool
}

const (
	DefaultWorkspaceID = "rss-default"
	DefaultSubjectID   = "rss-owner"
)

type Repository interface {
	ListSubscriptions(context.Context) ([]Subscription, error)
	GetSubscription(context.Context, string) (Subscription, error)
	CreateSubscription(context.Context, Subscription) (Subscription, error)
	CreateFeed(context.Context, FeedUpdate) (Subscription, UpsertResult, error)
	UpdateSubscription(context.Context, Subscription) (Subscription, error)
	DeleteSubscription(context.Context, string, time.Time) error
	UpsertFeed(context.Context, FeedUpdate) (UpsertResult, error)
	ListEntries(context.Context, EntryQuery) (EntryPage, error)
	GetEntry(context.Context, string) (Entry, error)
	ApplyReadMutation(context.Context, ReadMutation) (EntryState, error)
	ListChanges(context.Context, int64, int) (ChangePage, error)
}

// SyncRepository is deliberately separate from Repository so the Wails
// desktop contract remains unchanged and narrow desktop test doubles do not
// have to implement the public replication protocol.
type SyncRepository interface {
	GetSyncOverview(context.Context, SyncScope) (SyncOverview, error)
	GetSyncSubscription(context.Context, string) (Subscription, error)
	GetSyncEntry(context.Context, string) (Entry, error)
	ListSyncEntries(context.Context, EntryQuery) (SyncEntryPage, error)
	ListSyncSnapshot(context.Context, SyncSnapshotQuery) (SyncSnapshotPage, error)
	ListSyncChanges(context.Context, SyncChangeQuery) (ChangePage, error)
	ApplyStateMutation(context.Context, StateMutation) (EntryState, error)
}

// SharedPublicRepository owns the transactional mutation receipts, fetch
// leases, canonical observation ingest and RSS change journal writes.
type SharedPublicRepository interface {
	ApplySubscriptionMutation(context.Context, SubscriptionMutation) (SubscriptionMutationResult, error)
	ApplyFeedObservation(context.Context, FeedObservation) (ObservationResult, error)
	AcquireFetchLease(context.Context, FetchLeaseRequest) (FetchLeaseResult, error)
}

// DiscoveryRepository is separate from Repository so narrow state-only test
// doubles and future alternate feed stores do not need to implement catalog
// caching. The SQLite repository implements both interfaces.
type DiscoveryRepository interface {
	GetDiscoveryState(context.Context) (DiscoveryState, error)
	QueryDiscovery(context.Context, DiscoveryQuery) (DiscoveryPage, error)
	FindDiscoveryRoute(context.Context, DiscoveryQuery) (DiscoveryRoute, error)
	ReplaceDiscoveryCache(context.Context, DiscoveryCache) error
}
