package rss

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

type stateRepositoryStub struct {
	mutation  domainrss.ReadMutation
	result    domainrss.EntryState
	listPage  domainrss.EntryPage
	listQuery domainrss.EntryQuery
	entry     domainrss.Entry
	err       error
	calls     int
}

func (stub *stateRepositoryStub) ListSubscriptions(context.Context) ([]domainrss.Subscription, error) {
	return nil, nil
}

func (stub *stateRepositoryStub) GetSubscription(context.Context, string) (domainrss.Subscription, error) {
	return domainrss.Subscription{}, nil
}

func (stub *stateRepositoryStub) CreateSubscription(_ context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	return item, nil
}

func (stub *stateRepositoryStub) CreateFeed(_ context.Context, update domainrss.FeedUpdate) (domainrss.Subscription, domainrss.UpsertResult, error) {
	return update.Subscription, domainrss.UpsertResult{Created: len(update.Entries)}, nil
}

func (stub *stateRepositoryStub) UpdateSubscription(_ context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	return item, nil
}

func (stub *stateRepositoryStub) DeleteSubscription(context.Context, string, time.Time) error {
	return nil
}

func (stub *stateRepositoryStub) UpsertFeed(context.Context, domainrss.FeedUpdate) (domainrss.UpsertResult, error) {
	return domainrss.UpsertResult{}, nil
}

func (stub *stateRepositoryStub) ListEntries(_ context.Context, query domainrss.EntryQuery) (domainrss.EntryPage, error) {
	stub.listQuery = query
	return stub.listPage, nil
}

func (stub *stateRepositoryStub) GetEntry(context.Context, string) (domainrss.Entry, error) {
	return stub.entry, nil
}

func TestRSSServiceKeepsArticleBodiesDetailOnly(t *testing.T) {
	const body = `<article><p>Full body</p><script>bad()</script></article>`
	repository := &stateRepositoryStub{
		listPage: domainrss.EntryPage{Items: []domainrss.Entry{{
			ID: "entry-1", URL: "https://example.com/post", Title: "Post", ContentHTML: body,
		}}},
		entry: domainrss.Entry{ID: "entry-1", URL: "https://example.com/post", Title: "Post", ContentHTML: body},
	}
	service := NewService(repository, nil)
	page, err := service.ListEntries(context.Background(), ListEntriesRequest{StarredOnly: true})
	if err != nil || len(page.Items) != 1 || page.Items[0].ContentHTML != "" {
		t.Fatalf("list page=%#v error=%v", page, err)
	}
	if !repository.listQuery.StarredOnly {
		t.Fatalf("desktop list query dropped starredOnly: %#v", repository.listQuery)
	}
	detail, err := service.GetEntry(context.Background(), SubscriptionRequest{ID: "entry-1"})
	if err != nil || !strings.Contains(detail.ContentHTML, "Full body") || strings.Contains(detail.ContentHTML, "script") {
		t.Fatalf("detail body=%q error=%v", detail.ContentHTML, err)
	}
}

func (stub *stateRepositoryStub) ApplyReadMutation(_ context.Context, mutation domainrss.ReadMutation) (domainrss.EntryState, error) {
	stub.calls++
	stub.mutation = mutation
	return stub.result, stub.err
}

func (stub *stateRepositoryStub) ListChanges(context.Context, int64, int) (domainrss.ChangePage, error) {
	return domainrss.ChangePage{}, nil
}

func TestSetEntryReadForDeviceForcesTransportIdentityAndSyncMetadata(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	revision := int64(3)
	repository := &stateRepositoryStub{result: domainrss.EntryState{EntryID: "entry-1", Revision: 4}}
	service := NewService(repository, nil)
	service.now = func() time.Time { return now }

	result, err := service.SetEntryReadForDevice(context.Background(), " iphone-1 ", SetEntryReadRequest{
		ID: " entry-1 ", Read: true, ExpectedRevision: &revision, MutationID: " mutation-1 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision != 4 || repository.calls != 1 {
		t.Fatalf("result=%#v calls=%d", result, repository.calls)
	}
	mutation := repository.mutation
	if mutation.EntryID != "entry-1" || !mutation.Read || mutation.ExpectedRevision == nil ||
		*mutation.ExpectedRevision != 3 || mutation.DeviceID != "iphone-1" || mutation.MutationID != "mutation-1" ||
		!mutation.ChangedAt.Equal(now) {
		t.Fatalf("mutation = %#v", mutation)
	}
}

func TestSetEntryReadForDeviceRejectsIncompleteSyncMetadata(t *testing.T) {
	revision := int64(0)
	negative := int64(-1)
	for name, test := range map[string]struct {
		deviceID string
		request  SetEntryReadRequest
	}{
		"missing device":    {request: SetEntryReadRequest{ID: "entry-1", ExpectedRevision: &revision, MutationID: "m-1"}},
		"missing entry":     {deviceID: "iphone-1", request: SetEntryReadRequest{ExpectedRevision: &revision, MutationID: "m-1"}},
		"missing revision":  {deviceID: "iphone-1", request: SetEntryReadRequest{ID: "entry-1", MutationID: "m-1"}},
		"negative revision": {deviceID: "iphone-1", request: SetEntryReadRequest{ID: "entry-1", ExpectedRevision: &negative, MutationID: "m-1"}},
		"missing mutation":  {deviceID: "iphone-1", request: SetEntryReadRequest{ID: "entry-1", ExpectedRevision: &revision}},
	} {
		t.Run(name, func(t *testing.T) {
			repository := &stateRepositoryStub{}
			service := NewService(repository, nil)
			if _, err := service.SetEntryReadForDevice(context.Background(), test.deviceID, test.request); err == nil {
				t.Fatal("invalid public state mutation was accepted")
			}
			if repository.calls != 0 {
				t.Fatalf("repository calls = %d", repository.calls)
			}
		})
	}
}

func TestDesktopSetEntryReadKeepsDesktopIdentityAndGeneratedMutationID(t *testing.T) {
	repository := &stateRepositoryStub{}
	service := NewService(repository, nil)
	if _, err := service.SetEntryRead(context.Background(), SetEntryReadRequest{ID: "entry-1", Read: true}); err != nil {
		t.Fatal(err)
	}
	if repository.mutation.DeviceID != "desktop" || repository.mutation.MutationID == "" {
		t.Fatalf("desktop mutation = %#v", repository.mutation)
	}
}

type bulkReadRepositoryStub struct {
	*stateRepositoryStub
	entries   []domainrss.Entry
	read      map[string]bool
	queries   []domainrss.EntryQuery
	mutations []domainrss.ReadMutation
}

func (stub *bulkReadRepositoryStub) ListEntries(ctx context.Context, query domainrss.EntryQuery) (domainrss.EntryPage, error) {
	if err := ctx.Err(); err != nil {
		return domainrss.EntryPage{}, err
	}
	stub.queries = append(stub.queries, query)
	filtered := make([]domainrss.Entry, 0, len(stub.entries))
	for _, item := range stub.entries {
		if query.SubscriptionID != "" && item.SubscriptionID != query.SubscriptionID {
			continue
		}
		if query.Kind != "" && item.Kind != query.Kind {
			continue
		}
		if query.StarredOnly && item.StarredAt == nil {
			continue
		}
		if query.UnreadOnly && (item.ReadAt != nil || stub.read[item.ID]) {
			continue
		}
		filtered = append(filtered, item)
	}
	total := len(filtered)
	start := query.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	end := min(start+limit, total)
	page := domainrss.EntryPage{Items: append([]domainrss.Entry(nil), filtered[start:end]...), Total: total}
	if end < total {
		page.NextOffset = end
	}
	return page, nil
}

func (stub *bulkReadRepositoryStub) ApplyReadMutation(_ context.Context, mutation domainrss.ReadMutation) (domainrss.EntryState, error) {
	if !mutation.Read || mutation.DeviceID != "desktop" || strings.TrimSpace(mutation.MutationID) == "" {
		return domainrss.EntryState{}, fmt.Errorf("invalid bulk read mutation: %#v", mutation)
	}
	if stub.read == nil {
		stub.read = make(map[string]bool)
	}
	stub.read[mutation.EntryID] = true
	stub.mutations = append(stub.mutations, mutation)
	return domainrss.EntryState{EntryID: mutation.EntryID, Read: true}, nil
}

func TestMarkAllReadCoversEveryFilteredPageWithoutOffsetSkipping(t *testing.T) {
	now := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)
	starredAt := now
	readAt := now
	repository := &bulkReadRepositoryStub{
		stateRepositoryStub: &stateRepositoryStub{},
		read:                make(map[string]bool),
	}
	for index := 0; index < 1203; index++ {
		repository.entries = append(repository.entries, domainrss.Entry{
			ID: fmt.Sprintf("matching-%04d", index), SubscriptionID: "subscription-a",
			Kind: domainrss.EntryKindArticle, StarredAt: &starredAt,
		})
	}
	for index := 0; index < 3; index++ {
		repository.entries = append(repository.entries, domainrss.Entry{
			ID: fmt.Sprintf("unstarred-%d", index), SubscriptionID: "subscription-a",
			Kind: domainrss.EntryKindArticle,
		})
	}
	for index := 0; index < 4; index++ {
		repository.entries = append(repository.entries, domainrss.Entry{
			ID: fmt.Sprintf("video-%d", index), SubscriptionID: "subscription-a",
			Kind: domainrss.EntryKindVideo, StarredAt: &starredAt,
		})
	}
	for index := 0; index < 5; index++ {
		repository.entries = append(repository.entries, domainrss.Entry{
			ID: fmt.Sprintf("other-subscription-%d", index), SubscriptionID: "subscription-b",
			Kind: domainrss.EntryKindArticle, StarredAt: &starredAt,
		})
	}
	for index := 0; index < 2; index++ {
		repository.entries = append(repository.entries, domainrss.Entry{
			ID: fmt.Sprintf("already-read-%d", index), SubscriptionID: "subscription-a",
			Kind: domainrss.EntryKindArticle, StarredAt: &starredAt, ReadAt: &readAt,
		})
	}

	service := NewService(repository, nil)
	result, err := service.MarkAllRead(context.Background(), MarkAllReadRequest{
		SubscriptionID: " subscription-a ", Kind: " ARTICLE ", StarredOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 1203 || len(repository.mutations) != 1203 {
		t.Fatalf("result=%#v mutations=%d", result, len(repository.mutations))
	}
	if len(repository.queries) != 4 {
		t.Fatalf("list calls=%d, want three data pages plus one empty page", len(repository.queries))
	}
	for _, query := range repository.queries {
		if query.Offset != 0 || query.Limit != markAllReadPageSize || !query.UnreadOnly || !query.StarredOnly ||
			query.SubscriptionID != "subscription-a" || query.Kind != domainrss.EntryKindArticle {
			t.Fatalf("bulk list query = %#v", query)
		}
	}
	for index := 0; index < 1203; index++ {
		if !repository.read[fmt.Sprintf("matching-%04d", index)] {
			t.Fatalf("matching entry %d remained unread", index)
		}
	}
	for _, id := range []string{"unstarred-0", "video-0", "other-subscription-0"} {
		if repository.read[id] {
			t.Fatalf("out-of-scope entry %q was marked read", id)
		}
	}

	listCalls := len(repository.queries)
	if _, err := service.MarkAllRead(context.Background(), MarkAllReadRequest{Kind: "podcast"}); err == nil {
		t.Fatal("invalid collection kind was accepted")
	}
	if len(repository.queries) != listCalls {
		t.Fatal("invalid collection kind reached the repository")
	}
}
