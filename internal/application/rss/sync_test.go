package rss

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	domainrss "xiadown/internal/domain/rss"
)

type syncServiceRepositoryStub struct {
	stateRepositoryStub
	entry         domainrss.Entry
	subscriptions []domainrss.Subscription
	overview      domainrss.SyncOverview
	entryPage     domainrss.SyncEntryPage
	entryQuery    domainrss.EntryQuery
	snapshot      domainrss.SyncSnapshotPage
	snapshotQuery domainrss.SyncSnapshotQuery
	changes       domainrss.ChangePage
	changeQuery   domainrss.SyncChangeQuery
	stateMutation domainrss.StateMutation
	stateResult   domainrss.EntryState
	stateErr      error
}

func (stub *syncServiceRepositoryStub) GetEntry(context.Context, string) (domainrss.Entry, error) {
	return stub.entry, nil
}

func (stub *syncServiceRepositoryStub) GetSyncEntry(context.Context, string) (domainrss.Entry, error) {
	return stub.entry, nil
}

func (stub *syncServiceRepositoryStub) GetSyncSubscription(_ context.Context, id string) (domainrss.Subscription, error) {
	for _, item := range stub.subscriptions {
		if item.ID == id {
			return item, nil
		}
	}
	return domainrss.Subscription{}, domainrss.ErrNotFound
}

func (stub *syncServiceRepositoryStub) ListSubscriptions(context.Context) ([]domainrss.Subscription, error) {
	return stub.subscriptions, nil
}

func (stub *syncServiceRepositoryStub) GetSyncOverview(context.Context, domainrss.SyncScope) (domainrss.SyncOverview, error) {
	return stub.overview, nil
}

func (stub *syncServiceRepositoryStub) ListSyncEntries(_ context.Context, query domainrss.EntryQuery) (domainrss.SyncEntryPage, error) {
	stub.entryQuery = query
	return stub.entryPage, nil
}

func (stub *syncServiceRepositoryStub) ListSyncSnapshot(_ context.Context, query domainrss.SyncSnapshotQuery) (domainrss.SyncSnapshotPage, error) {
	stub.snapshotQuery = query
	return stub.snapshot, nil
}

func (stub *syncServiceRepositoryStub) ListSyncChanges(_ context.Context, query domainrss.SyncChangeQuery) (domainrss.ChangePage, error) {
	stub.changeQuery = query
	return stub.changes, nil
}

func (stub *syncServiceRepositoryStub) ApplyStateMutation(_ context.Context, mutation domainrss.StateMutation) (domainrss.EntryState, error) {
	stub.stateMutation = mutation
	return stub.stateResult, stub.stateErr
}

func TestListSyncEntriesUsesDedicatedProjectionRepositoryAndNormalizesDTO(t *testing.T) {
	repository := &syncServiceRepositoryStub{entryPage: domainrss.SyncEntryPage{
		Items: []domainrss.SyncEntry{{
			ID: "entry-1", SubscriptionID: "subscription-1",
			Title: strings.Repeat(" title ", 300), Author: strings.Repeat(" author ", 200),
			Summary: strings.Repeat("summary", 2000), Platform: strings.Repeat("platform", 20),
			PlatformVideoID: strings.Repeat("video", 100),
			ArticleProgress: &domainrss.ArticleProgress{Fraction: 0.5, Anchor: strings.Repeat("anchor", 200), ContentRevision: 7},
		}},
		Total: 2, NextOffset: 1,
	}}
	service := NewService(repository, nil)
	page, err := service.ListSyncEntries(context.Background(), ListEntriesRequest{
		SubscriptionID: " subscription-1 ", Kind: "VIDEO", Query: " query ",
		UnreadOnly: true, StarredOnly: true, Limit: 1, Offset: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.entryQuery.SubscriptionID != "subscription-1" || repository.entryQuery.Kind != domainrss.EntryKindVideo ||
		repository.entryQuery.Query != "query" || !repository.entryQuery.UnreadOnly ||
		!repository.entryQuery.StarredOnly || repository.entryQuery.Limit != 1 || repository.entryQuery.Offset != 3 {
		t.Fatalf("entry query = %#v", repository.entryQuery)
	}
	if len(page.Items) != 1 || page.Total != 2 || page.NextOffset != 1 ||
		len(page.Items[0].Title) > 1024 || len(page.Items[0].Author) > 512 ||
		len(page.Items[0].Summary) > maxSyncSummaryBytes || len(page.Items[0].Platform) > 64 ||
		len(page.Items[0].PlatformVideoID) > 256 || page.Items[0].ArticleProgress == nil ||
		len(page.Items[0].ArticleProgress.Anchor) > 512 {
		t.Fatalf("sync page was not normalized: %#v", page)
	}
}

func TestSyncServiceOverviewSnapshotAndChangesUseFixedOwnerScopeAndSafeDTOs(t *testing.T) {
	repository := &syncServiceRepositoryStub{
		overview: domainrss.SyncOverview{
			CatalogID: "database-catalog", WorkspaceID: domainrss.DefaultWorkspaceID,
			SubjectID: domainrss.DefaultSubjectID, Epoch: testSyncEpoch, HighWater: 12,
		},
		snapshot: domainrss.SyncSnapshotPage{
			Records: []domainrss.SyncSnapshotRecord{{
				EntityType: "subscription", EntityID: "sub-1", Revision: 1,
				Payload: json.RawMessage(`{"id":"sub-1","workspaceId":"rss-default","feedUrl":"https://secret.example/token","siteUrl":"http://127.0.0.1/","title":"Feed","viewType":"auto","enabled":true,"unreadCount":0,"createdAt":"2026-07-13T00:00:00Z","updatedAt":"2026-07-13T00:00:00Z","revision":1}`),
			}},
			Epoch: testSyncEpoch, HighWater: 12, NextStage: "entries", NextID: "entry-1", HasMore: true,
		},
	}
	service := NewService(repository, nil)
	overview, err := service.GetSyncOverview(context.Background(), "paired-catalog")
	if err != nil {
		t.Fatal(err)
	}
	if overview.CatalogID != "paired-catalog" || overview.WorkspaceID != domainrss.DefaultWorkspaceID ||
		overview.SubjectID != domainrss.DefaultSubjectID || len(overview.Capabilities) < 5 ||
		!strings.Contains(strings.Join(overview.Capabilities, ","), "opaque-resource-slots-v1") {
		t.Fatalf("overview = %#v", overview)
	}

	first, err := service.GetSyncSnapshot(context.Background(), SyncSnapshotRequest{
		Epoch: testSyncEpoch, HighWater: 12, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !first.HasMore || first.NextCursor == "" || repository.snapshotQuery.Scope.WorkspaceID != domainrss.DefaultWorkspaceID ||
		repository.snapshotQuery.Scope.SubjectID != domainrss.DefaultSubjectID || repository.snapshotQuery.Stage != "subscriptions" {
		t.Fatalf("snapshot result=%#v query=%#v", first, repository.snapshotQuery)
	}
	payload := string(first.Records[0].Payload)
	if strings.Contains(payload, "feedUrl") || strings.Contains(payload, "secret.example") || strings.Contains(payload, "127.0.0.1") {
		t.Fatalf("unsafe subscription snapshot payload = %s", payload)
	}

	repository.snapshot = domainrss.SyncSnapshotPage{Epoch: testSyncEpoch, HighWater: 12}
	if _, err := service.GetSyncSnapshot(context.Background(), SyncSnapshotRequest{
		Epoch: testSyncEpoch, HighWater: 12, Cursor: first.NextCursor, Limit: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if repository.snapshotQuery.Stage != "entries" || repository.snapshotQuery.AfterID != "entry-1" {
		t.Fatalf("decoded keyset cursor query = %#v", repository.snapshotQuery)
	}
	if _, err := service.GetSyncSnapshot(context.Background(), SyncSnapshotRequest{
		Epoch: testSyncEpoch, HighWater: 13, Cursor: first.NextCursor, Limit: 1,
	}); !errors.Is(err, domainrss.ErrInvalidRequest) {
		t.Fatalf("cursor reused at another high-water mark: %v", err)
	}

	repository.changes = domainrss.ChangePage{
		Epoch: testSyncEpoch, Cursor: 13, HighWater: 13,
		Changes: []domainrss.Change{{
			Sequence: 13, EntityType: "entry", EntityID: "entry-1", Revision: 2,
			Payload: json.RawMessage(`{"id":"entry-1","subscriptionId":"sub-1","title":"Post","kind":"article","contentHtml":"<script>bad()</script>","read":false,"starred":false,"videoCompleted":false,"fieldRevisions":{"read":0,"starred":0,"articleProgress":0,"videoProgressSeconds":0},"stateRevision":0,"contentRevision":2,"createdAt":"2026-07-13T00:00:00Z","modifiedAt":"2026-07-13T00:00:00Z"}`),
		}},
	}
	changes, err := service.ListSyncChanges(context.Background(), ListChangesRequest{Epoch: testSyncEpoch, After: 12, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if repository.changeQuery.Scope.SubjectID != domainrss.DefaultSubjectID || len(changes.Changes) != 1 ||
		strings.Contains(string(changes.Changes[0].Payload), "contentHtml") || strings.Contains(string(changes.Changes[0].Payload), "<script>") {
		t.Fatalf("safe changes=%#v query=%#v", changes, repository.changeQuery)
	}
}

func TestSyncServiceStateMutationCanonicalHashBindsPayload(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	repository := &syncServiceRepositoryStub{stateResult: domainrss.EntryState{EntryID: "entry-1", Revision: 1}}
	service := NewService(repository, nil)
	service.now = func() time.Time { return now }
	revision := int64(0)
	starred := true
	request := SetEntryStateRequest{
		ID: "entry-1", Field: domainrss.EntryStateFieldStarred, Starred: &starred,
		ExpectedRevision: &revision, MutationID: "mutation-1",
	}
	if _, err := service.SetEntryStateForDevice(context.Background(), " iphone-1 ", request); err != nil {
		t.Fatal(err)
	}
	firstHash := repository.stateMutation.RequestHash
	if repository.stateMutation.Scope.WorkspaceID != domainrss.DefaultWorkspaceID ||
		repository.stateMutation.Scope.SubjectID != domainrss.DefaultSubjectID ||
		repository.stateMutation.DeviceID != "iphone-1" || len(firstHash) != 64 ||
		!repository.stateMutation.ChangedAt.Equal(now) || repository.stateMutation.AllowDesktopLocal {
		t.Fatalf("state mutation = %#v", repository.stateMutation)
	}
	if _, err := service.SetEntryStateForDevice(context.Background(), "iphone-1", request); err != nil {
		t.Fatal(err)
	}
	if repository.stateMutation.RequestHash != firstHash {
		t.Fatalf("canonical hash changed for same payload: %q -> %q", firstHash, repository.stateMutation.RequestHash)
	}
	starred = false
	request.Starred = &starred
	if _, err := service.SetEntryStateForDevice(context.Background(), "iphone-1", request); err != nil {
		t.Fatal(err)
	}
	if repository.stateMutation.RequestHash == firstHash {
		t.Fatal("canonical hash did not bind the state value")
	}

	progress, duration := 12.5, 30.0
	if _, err := service.SetEntryStateForDevice(context.Background(), "iphone-1", SetEntryStateRequest{
		ID: "entry-1", Field: domainrss.EntryStateFieldVideoProgressSeconds,
		VideoProgressSeconds: &progress, VideoDurationSeconds: &duration,
		ExpectedRevision: &revision, MutationID: "video-1",
	}); err != nil {
		t.Fatal(err)
	}
	if repository.stateMutation.VideoProgressSeconds == nil || *repository.stateMutation.VideoProgressSeconds != progress ||
		repository.stateMutation.VideoDurationSeconds == nil || *repository.stateMutation.VideoDurationSeconds != duration {
		t.Fatalf("video mutation = %#v", repository.stateMutation)
	}
	request.Starred = &starred
	if _, err := service.SetEntryState(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !repository.stateMutation.AllowDesktopLocal || repository.stateMutation.DeviceID != "desktop" {
		t.Fatalf("desktop mutation did not opt into local sources: %#v", repository.stateMutation)
	}
}

func TestSyncServiceDetailSanitizesLegacyHTMLAndExternalResourcesOnRead(t *testing.T) {
	repository := &syncServiceRepositoryStub{entry: domainrss.Entry{
		ID: "entry-1", SubscriptionID: "sub-1", URL: "https://example.com/posts/1", Title: "Post",
		ContentHTML: `<p onclick="bad()"><a href="../next">Next</a><img src="./cover.jpg"><script>bad()</script></p>`,
		ImageURLs:   []string{"https://example.com/posts/cover.jpg", "http://127.0.0.1/private.jpg"},
		Media: []domainrss.Media{
			{URL: "https://cdn.example.com/video.mp4", Kind: "video"},
			{URL: "http://192.168.1.2/video.mp4", Kind: "video"},
		},
		MediaURL: "http://127.0.0.1/media.mp4", Revision: 3,
	}}
	service := NewService(repository, nil)
	detail, err := service.GetSyncEntry(context.Background(), SubscriptionRequest{ID: "entry-1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(detail.ContentHTML, "script") || strings.Contains(detail.ContentHTML, "onclick") ||
		strings.Contains(detail.ContentHTML, "href=") || strings.Contains(detail.ContentHTML, "src=") ||
		!strings.Contains(detail.ContentHTML, "Next") || !strings.Contains(detail.ContentHTML, `data-xiadown-slot="image-0"`) ||
		len(detail.ImageSlots) != 2 || !detail.ImageSlots[0] || detail.ImageSlots[1] ||
		len(detail.MediaSlots) != 2 || !detail.MediaSlots[0].Available || detail.MediaSlots[1].Available {
		t.Fatalf("safe detail = %#v", detail)
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"example.com/cover", "cdn.example.com/video", "127.0.0.1/media"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("paired detail leaked source resource %q: %s", secret, encoded)
		}
	}
}

func TestSyncServiceDetailKeepsInlineImageOrderAsOpaqueSlots(t *testing.T) {
	repository := &syncServiceRepositoryStub{entry: domainrss.Entry{
		ID: "entry-inline", SubscriptionID: "sub-1", URL: "https://publisher.example/posts/1", Revision: 7,
		ContentHTML: `<h2 id="first">First</h2><p>Before</p>` +
			`<picture><source src="https://cdn.example/large.jpg"><img src="https://cdn.example/fallback.jpg" alt="Cover"></picture>` +
			`<p>Middle</p><img src="https://cdn.example/second.jpg"><p>After</p>`,
		ImageURLs: []string{
			"https://cdn.example/large.jpg",
			"https://cdn.example/fallback.jpg",
			"https://cdn.example/second.jpg",
		},
	}}

	detail, err := NewService(repository, nil).GetSyncEntry(context.Background(), SubscriptionRequest{ID: repository.entry.ID})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(detail.ContentHTML, "https://") || strings.Contains(detail.ContentHTML, "src=") {
		t.Fatalf("structured body leaked a reusable source: %s", detail.ContentHTML)
	}
	first := strings.Index(detail.ContentHTML, `data-xiadown-slot="image-1"`)
	second := strings.Index(detail.ContentHTML, `data-xiadown-slot="image-2"`)
	if first < 0 || second <= first || strings.Contains(detail.ContentHTML, `data-xiadown-slot="image-0"`) ||
		!strings.Contains(detail.ContentHTML, "Before") || !strings.Contains(detail.ContentHTML, "Middle") || !strings.Contains(detail.ContentHTML, "After") {
		t.Fatalf("structured inline slots = %s", detail.ContentHTML)
	}
}

func TestSyncServiceDetailPreservesRemoteResourceSlotIndexes(t *testing.T) {
	repository := &syncServiceRepositoryStub{entry: domainrss.Entry{
		ID:             "entry-1",
		SubscriptionID: "sub-1",
		ImageURLs: []string{
			"http://127.0.0.1/private.jpg",
			"https://images.example.com/public.jpg",
		},
		Media: []domainrss.Media{
			{
				URL:       "http://192.168.1.2/private.mp4",
				Thumbnail: "https://images.example.com/poster-0.jpg",
				Kind:      "video",
			},
			{
				URL:       "https://cdn.example.com/video-1.mp4",
				Thumbnail: "http://127.0.0.1/private-poster.jpg",
				Kind:      "video",
			},
		},
	}}

	detail, err := NewService(repository, nil).GetSyncEntry(
		context.Background(), SubscriptionRequest{ID: "entry-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.ImageSlots) != 2 || detail.ImageSlots[0] || !detail.ImageSlots[1] {
		t.Fatalf("image slot projection = %#v", detail.ImageSlots)
	}
	if len(detail.MediaSlots) != 2 || detail.MediaSlots[0].Available || !detail.MediaSlots[0].ThumbnailAvailable ||
		!detail.MediaSlots[1].Available || detail.MediaSlots[1].ThumbnailAvailable {
		t.Fatalf("media slot projection = %#v", detail.MediaSlots)
	}
}

func TestSyncServicePairedDTOExposesAvailabilityWithoutReusableSourceURLs(t *testing.T) {
	const signed = "signed-secret-credential"
	repository := &syncServiceRepositoryStub{
		subscriptions: []domainrss.Subscription{{
			ID: "sub-1", WorkspaceID: domainrss.DefaultWorkspaceID,
			SiteURL: "https://publisher.example/private/" + signed,
			IconURL: "https://cdn.example/icon.png?token=" + signed,
			Title:   "Private feed", ViewType: domainrss.ViewTypeAuto, Revision: 1,
		}},
		entry: domainrss.Entry{
			ID: "entry-1", SubscriptionID: "sub-1", Title: "Private item", Revision: 1,
			URL:          "https://publisher.example/article/" + signed,
			ThumbnailURL: "https://cdn.example/thumb.jpg?sig=" + signed,
			ContentHTML: `<p><a href="https://publisher.example/open/` + signed + `">Read</a>` +
				`<img src="https://cdn.example/body.jpg?sig=` + signed + `"></p>`,
			ImageURLs: []string{"https://cdn.example/body.jpg?sig=" + signed},
			Media: []domainrss.Media{{
				URL: "https://cdn.example/video.mp4?sig=" + signed, Thumbnail: "https://cdn.example/poster.jpg?sig=" + signed,
				Kind: "video", MIMEType: "video/mp4",
			}},
			MediaURL: "https://cdn.example/media.mp4?sig=" + signed, PlaybackURL: "https://player.example/" + signed,
			DownloadTarget: "https://download.example/" + signed,
		},
	}
	service := NewService(repository, nil)
	subscriptions, err := service.ListSyncSubscriptions(context.Background())
	if err != nil || len(subscriptions) != 1 || !subscriptions[0].IconAvailable {
		t.Fatalf("subscriptions = %#v error=%v", subscriptions, err)
	}
	detail, err := service.GetSyncEntry(context.Background(), SubscriptionRequest{ID: "entry-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !detail.ThumbnailAvailable || len(detail.ImageSlots) != 1 || !detail.ImageSlots[0] ||
		len(detail.MediaSlots) != 1 || !detail.MediaSlots[0].Available || !detail.MediaSlots[0].ThumbnailAvailable {
		t.Fatalf("availability projection = %#v", detail)
	}
	encoded, err := json.Marshal(struct {
		Subscriptions []domainrss.SyncSubscription `json:"subscriptions"`
		Detail        SyncEntryDetail              `json:"detail"`
	}{subscriptions, detail})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{signed, "cdn.example", "publisher.example", "player.example", "download.example", "iconUrl", "thumbnailUrl", "imageUrls", "mediaUrl", "playbackUrl", "downloadTarget"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("paired DTO leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestGetSyncEntryClampsLegacyMediaMetadata(t *testing.T) {
	repository := &syncServiceRepositoryStub{
		entry: domainrss.Entry{
			ID:             "entry-legacy-media",
			SubscriptionID: "subscription-1",
			Title:          "Legacy media",
			Kind:           domainrss.EntryKindVideo,
			Revision:       1,
			StateRevision:  1,
			Media: []domainrss.Media{
				{Kind: "video", Width: -10, Height: -20, Duration: -30},
				{Kind: "video", Width: 100_000, Height: 200_000, Duration: 9 * 24 * 60 * 60 * 1000},
			},
		},
	}
	service := NewService(repository, nil)

	detail, err := service.GetSyncEntry(context.Background(), SubscriptionRequest{ID: repository.entry.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.MediaSlots) != 2 {
		t.Fatalf("media slots = %#v", detail.MediaSlots)
	}
	if got := detail.MediaSlots[0]; got.Width != 0 || got.Height != 0 || got.Duration != 0 {
		t.Fatalf("negative legacy metadata was not removed: %#v", got)
	}
	if got := detail.MediaSlots[1]; got.Width != maxRSSMediaDimension || got.Height != maxRSSMediaDimension || got.Duration != maxRSSMediaDurationMillis {
		t.Fatalf("oversized legacy metadata was not clamped: %#v", got)
	}
}

func TestLegacySyncSubscriptionListHasAHardCompatibilityLimit(t *testing.T) {
	items := make([]domainrss.Subscription, maxLegacySyncSubscriptions+25)
	for index := range items {
		items[index] = domainrss.Subscription{
			ID: fmt.Sprintf("sub-%04d", index), WorkspaceID: domainrss.DefaultWorkspaceID,
			Title: "Feed", ViewType: domainrss.ViewTypeAuto, Revision: 1,
		}
	}
	service := NewService(&syncServiceRepositoryStub{subscriptions: items}, nil)
	result, err := service.ListSyncSubscriptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != maxLegacySyncSubscriptions {
		t.Fatalf("legacy subscription count = %d", len(result))
	}
}

func TestSyncServiceDetailBoundsHTMLSummaryImagesAndMedia(t *testing.T) {
	images := make([]string, 0, 100)
	media := make([]domainrss.Media, 0, 100)
	for index := 0; index < 100; index++ {
		images = append(images, fmt.Sprintf("https://images.example.com/%d.jpg", index))
		media = append(media, domainrss.Media{URL: fmt.Sprintf("https://cdn.example.com/%d.mp4", index), Kind: "video"})
	}
	repository := &syncServiceRepositoryStub{entry: domainrss.Entry{
		ID: "entry-1", SubscriptionID: "sub-1", URL: "https://example.com/posts/1", Title: "Post",
		Summary:     strings.Repeat("界", 4000),
		ContentHTML: "<p>" + strings.Repeat("界", 400000) + "</p>",
		ImageURLs:   images, Media: media, Revision: 1,
	}}
	detail, err := NewService(repository, nil).GetSyncEntry(context.Background(), SubscriptionRequest{ID: "entry-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.ContentHTML) > maxSyncContentHTMLBytes || !utf8.ValidString(detail.ContentHTML) {
		t.Fatalf("contentHtml bytes=%d utf8=%v", len(detail.ContentHTML), utf8.ValidString(detail.ContentHTML))
	}
	if len(detail.Summary) > maxSyncSummaryBytes || !utf8.ValidString(detail.Summary) {
		t.Fatalf("summary bytes=%d utf8=%v", len(detail.Summary), utf8.ValidString(detail.Summary))
	}
	if len(detail.ImageSlots) != maxSyncImageItems || len(detail.MediaSlots) != maxSyncMediaItems {
		t.Fatalf("bounded images/media = %d/%d", len(detail.ImageSlots), len(detail.MediaSlots))
	}
}

const testSyncEpoch = "0123456789abcdef0123456789abcdef"
