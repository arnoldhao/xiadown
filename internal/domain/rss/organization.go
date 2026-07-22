package rss

import (
	"context"
	"time"
)

// Category groups subscriptions in the desktop navigation. SortOrder is a
// stable user-controlled position; titles are used only as a deterministic
// tie-breaker.
type Category struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspaceId"`
	Title             string    `json:"title"`
	SortOrder         int       `json:"sortOrder"`
	SubscriptionCount int       `json:"subscriptionCount"`
	UnreadCount       int       `json:"unreadCount"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	Revision          int64     `json:"revision"`
}

type CollectionKind string

const (
	// CollectionKindSubscriptions is a custom timeline composed of feeds.
	CollectionKindSubscriptions CollectionKind = "subscriptions"
	// CollectionKindEntries is a named saved-item collection.
	CollectionKindEntries CollectionKind = "entries"
)

type Collection struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspaceId"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Kind        CollectionKind `json:"kind"`
	ViewType    ViewType       `json:"viewType"`
	SortOrder   int            `json:"sortOrder"`
	ItemCount   int            `json:"itemCount"`
	UnreadCount int            `json:"unreadCount"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	Revision    int64          `json:"revision"`
}

type CollectionItems struct {
	CollectionID string         `json:"collectionId"`
	Kind         CollectionKind `json:"kind"`
	ItemIDs      []string       `json:"itemIds"`
}

type SourceKind string

const (
	SourceKindInbox        SourceKind = "inbox"
	SourceKindNotification SourceKind = "notification"
)

// Source is a local Inbox or notification producer backed by an internal RSS
// subscription. SubscriptionID is intentionally exposed so collection and
// entry queries can use the same state model as remote feeds.
type Source struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspaceId"`
	SubscriptionID string     `json:"subscriptionId"`
	Kind           SourceKind `json:"kind"`
	Handle         string     `json:"handle"`
	Title          string     `json:"title"`
	Enabled        bool       `json:"enabled"`
	SortOrder      int        `json:"sortOrder"`
	UnreadCount    int        `json:"unreadCount"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	Revision       int64      `json:"revision"`
}

// OrganizationRepository is separate from Repository so focused feed-parser
// and entry-state test doubles keep their narrow contract.
type OrganizationRepository interface {
	ListCategories(context.Context) ([]Category, error)
	GetCategory(context.Context, string) (Category, error)
	CreateCategory(context.Context, Category) (Category, error)
	UpdateCategory(context.Context, Category) (Category, error)
	DeleteCategory(context.Context, string) error
	ReorderCategories(context.Context, []string, time.Time) ([]Category, error)
	ReorderSubscriptions(context.Context, string, []string, time.Time) ([]Subscription, error)

	ListCollections(context.Context) ([]Collection, error)
	GetCollection(context.Context, string) (Collection, error)
	CreateCollection(context.Context, Collection) (Collection, error)
	UpdateCollection(context.Context, Collection) (Collection, error)
	DeleteCollection(context.Context, string) error
	ListCollectionItems(context.Context, string) (CollectionItems, error)
	ReplaceCollectionItems(context.Context, string, CollectionKind, []string, time.Time) (Collection, error)
	AddCollectionItems(context.Context, string, CollectionKind, []string, time.Time) (Collection, error)
	RemoveCollectionItems(context.Context, string, CollectionKind, []string, time.Time) (Collection, error)

	ListSources(context.Context) ([]Source, error)
	GetSource(context.Context, string) (Source, error)
	CreateSource(context.Context, Source, Subscription) (Source, error)
	UpdateSource(context.Context, Source, Subscription) (Source, error)
	GetSourceEntry(context.Context, string, string) (Entry, error)
}
