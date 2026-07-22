package rss

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

type sharedPublicServiceRepositoryStub struct {
	*syncServiceRepositoryStub
	subscriptionMutation domainrss.SubscriptionMutation
	observation          domainrss.FeedObservation
	lease                domainrss.FetchLeaseRequest
	mutationResult       domainrss.SubscriptionMutationResult
	observationResult    domainrss.ObservationResult
	leaseResult          domainrss.FetchLeaseResult
	mutationErr          error
	observationErr       error
	leaseErr             error
	mutationCalls        int
	observationCalls     int
	leaseCalls           int
}

func (stub *sharedPublicServiceRepositoryStub) ApplySubscriptionMutation(
	_ context.Context,
	mutation domainrss.SubscriptionMutation,
) (domainrss.SubscriptionMutationResult, error) {
	stub.mutationCalls++
	stub.subscriptionMutation = mutation
	return stub.mutationResult, stub.mutationErr
}

func (stub *sharedPublicServiceRepositoryStub) ApplyFeedObservation(
	_ context.Context,
	observation domainrss.FeedObservation,
) (domainrss.ObservationResult, error) {
	stub.observationCalls++
	stub.observation = observation
	return stub.observationResult, stub.observationErr
}

func (stub *sharedPublicServiceRepositoryStub) AcquireFetchLease(
	_ context.Context,
	request domainrss.FetchLeaseRequest,
) (domainrss.FetchLeaseResult, error) {
	stub.leaseCalls++
	stub.lease = request
	return stub.leaseResult, stub.leaseErr
}

func TestRSSOriginKeyV1HasStableCrossPlatformGoldenValues(t *testing.T) {
	const subscriptionID = "fdc30b89-77f5-49a4-a448-a70e8f7bb023"
	publishedAt := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		guid       string
		link       string
		title      string
		published  *time.Time
		enclosures []domainrss.ObservationEnclosure
		want       string
	}{
		{
			name: "guid has priority",
			guid: "  feed-guid-123  ", link: "https://ignored.example/post", title: "Ignored",
			want: "rss-origin-v1:f7dd43952de832607d8e698719450dfc377a8877990106c30f8f1229206fd4e6",
		},
		{
			name: "canonical link has second priority",
			link: "HTTPS://EXAMPLE.com:443/Post?id=1#ignored-fragment", title: "Ignored",
			want: "rss-origin-v1:746af939be1dac75d2e4979f8cf354b597df26c3e1c83e57313f8dc96334d3e0",
		},
		{
			name:  "fallback normalizes time title and first enclosure",
			title: "  Hello\tWORLD  ", published: &publishedAt,
			enclosures: []domainrss.ObservationEnclosure{{URL: "https://MEDIA.example.com:443/a.mp3#fragment"}},
			want:       "rss-origin-v1:09b94ea79086caba7c1ddaea84522d5458c0fadfb6c8fed62ddd1a07dbca05b6",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := RSSOriginKeyV1(subscriptionID, test.guid, test.link, test.title, test.published, test.enclosures)
			if got != test.want {
				t.Fatalf("origin key=%q want=%q", got, test.want)
			}
		})
	}

	first := RSSOriginKeyV1(subscriptionID, "guid", "https://one.example/", "One", nil, nil)
	second := RSSOriginKeyV1(subscriptionID, "guid", "https://two.example/", "Two", &publishedAt,
		[]domainrss.ObservationEnclosure{{URL: "https://three.example/media"}})
	if first != second {
		t.Fatalf("GUID identity unexpectedly depended on lower-priority fields: %q != %q", first, second)
	}
	otherSubscription := RSSOriginKeyV1("248c47e6-87b3-4609-88cf-da31f688dc53", "guid", "", "", nil, nil)
	if first == otherSubscription {
		t.Fatal("origin identity was not namespaced by subscription ID")
	}
}

func TestSharedPublicSubscriptionMutationValidatesURLAndCanonicalizesHash(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	repository := &sharedPublicServiceRepositoryStub{syncServiceRepositoryStub: &syncServiceRepositoryStub{}}
	service := NewService(repository, nil)
	service.now = func() time.Time { return now }
	service.resolver = rssStaticResolver{
		"feeds.example.com":   {{IP: net.ParseIP("8.8.8.8")}},
		"private.example.com": {{IP: net.ParseIP("10.0.0.8")}},
	}
	const subscriptionID = "7f7a68d0-49ab-4e90-88bb-28fd77551d4e"
	const mutationID = "6553f2ea-6359-4ced-b554-05905ca16632"
	_, err := service.MutateSubscriptionForDevice(context.Background(), " iphone-a ", subscriptionID,
		SharedSubscriptionMutationRequest{
			MutationID: mutationID, Operation: domainrss.SubscriptionMutationAdd,
			Title: "Public feed", ViewType: "article",
			SourceAccess:  domainrss.SubscriptionSourceSharedPublic,
			PublicFeedURL: " HTTPS://feeds.example.com:443/public.xml#discard ",
		})
	if err != nil {
		t.Fatalf("valid shared-public add: %v", err)
	}
	mutation := repository.subscriptionMutation
	if repository.mutationCalls != 1 || mutation.DeviceID != "iphone-a" || mutation.MutationID != mutationID ||
		mutation.SubscriptionID != subscriptionID || mutation.SourceAccess != domainrss.SubscriptionSourceSharedPublic ||
		mutation.PublicFeedURL != "https://feeds.example.com/public.xml" || len(mutation.RequestHash) != 64 ||
		!mutation.ChangedAt.Equal(now) {
		t.Fatalf("normalized subscription mutation: %#v", mutation)
	}
	if strings.Contains(mutation.PublicFeedURL, "#") || strings.Contains(mutation.PublicFeedURL, "@") {
		t.Fatalf("unsafe URL survived normalization: %q", mutation.PublicFeedURL)
	}

	expectedRevision := int64(3)
	title, sortOrder := "Renamed", 5
	request := SharedSubscriptionMutationRequest{
		MutationID: mutationID, Operation: domainrss.SubscriptionMutationUpdate,
		ExpectedRevision: &expectedRevision, FieldMask: []string{"sortOrder", "title"},
		Title: title, SortOrder: &sortOrder,
	}
	if _, err := service.MutateSubscriptionForDevice(context.Background(), "iphone-a", subscriptionID, request); err != nil {
		t.Fatalf("first canonical update: %v", err)
	}
	firstHash := repository.subscriptionMutation.RequestHash
	request.FieldMask = []string{"title", "sortOrder"}
	if _, err := service.MutateSubscriptionForDevice(context.Background(), "iphone-a", subscriptionID, request); err != nil {
		t.Fatalf("reordered canonical update: %v", err)
	}
	if repository.subscriptionMutation.RequestHash != firstHash || strings.Join(repository.subscriptionMutation.FieldMask, ",") != "sortOrder,title" {
		t.Fatalf("field-mask order changed canonical mutation: %#v", repository.subscriptionMutation)
	}

	invalidURLs := []string{
		"http://feeds.example.com/public.xml",
		"https://user:password@feeds.example.com/public.xml",
		"https://feeds.example.com/public.xml?access_token=secret",
		"https://feeds.example.com/public.xml?API_KEY=secret",
		"https://feeds.example.com/public.xml?X-Amz-Signature=secret",
		"https://feeds.example.com/public.xml?X-Amz-Credential=secret",
		"https://feeds.example.com/public.xml?X-Goog-Signature=secret",
		"https://feeds.example.com/public.xml?format=rss",
		"https://feeds.example.com/public.xml?",
		"https://127.0.0.1/feed.xml",
		"https://metadata.google.internal/feed.xml",
		"https://private.example.com/feed.xml",
	}
	for _, rawURL := range invalidURLs {
		before := repository.mutationCalls
		_, err := service.MutateSubscriptionForDevice(context.Background(), "iphone-a", subscriptionID,
			SharedSubscriptionMutationRequest{
				MutationID: mutationID, Operation: domainrss.SubscriptionMutationAdd,
				SourceAccess: domainrss.SubscriptionSourceSharedPublic, PublicFeedURL: rawURL,
			})
		if err == nil {
			t.Fatalf("unsafe publicFeedURL was accepted: %q", rawURL)
		}
		if repository.mutationCalls != before {
			t.Fatalf("unsafe URL %q reached repository", rawURL)
		}
	}
}

func TestSharedPublicObservationValidatesOriginAndUsesCanonicalIngestPipeline(t *testing.T) {
	now := time.Date(2026, 7, 21, 13, 0, 0, 0, time.UTC)
	const subscriptionID = "fdc30b89-77f5-49a4-a448-a70e8f7bb023"
	const mutationID = "72233b29-c756-4a81-af79-2fff7dcc289f"
	publicFeedURL := "https://feeds.example.com/base/feed.xml"
	repository := &sharedPublicServiceRepositoryStub{syncServiceRepositoryStub: &syncServiceRepositoryStub{
		subscriptions: []domainrss.Subscription{{
			ID: subscriptionID, WorkspaceID: domainrss.DefaultWorkspaceID,
			SourceAccess: domainrss.SubscriptionSourceSharedPublic, PublicFeedURL: publicFeedURL,
			Title: "Shared feed", ViewType: domainrss.ViewTypeArticle, Enabled: true,
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), Revision: 1,
		}},
	}}
	service := NewService(repository, nil)
	service.now = func() time.Time { return now }
	publishedAt := now.Add(-2 * time.Hour)
	entry := domainrss.ObservationEntry{
		GUID: "post-guid-1", CanonicalLink: "https://feeds.example.com/posts/one",
		PublishedAt: &publishedAt, Title: "Observed article", Author: "Author",
		Summary: "Summary", ContentHTML: `<article><p>Safe body</p><script>bad()</script></article>`,
		Enclosures: []domainrss.ObservationEnclosure{{
			URL: "https://media.example.com/audio.mp3", MIMEType: "Audio/MPEG", ByteLength: 1024,
		}},
	}
	entry.OriginKey = RSSOriginKeyV1(subscriptionID, entry.GUID, entry.CanonicalLink, entry.Title, entry.PublishedAt, entry.Enclosures)
	request := FeedObservationRequest{
		MutationID: mutationID, UpstreamETag: `"etag-v1"`, LastModified: "Mon, 21 Jul 2026 12:00:00 GMT",
		FetchedAt: now.Add(-time.Minute), ContentHash: strings.Repeat("A", 64), Entries: []domainrss.ObservationEntry{entry},
	}
	repository.observationResult = domainrss.ObservationResult{MutationID: mutationID, Created: 1}
	result, err := service.SubmitFeedObservationForDevice(context.Background(), " iphone-a ", subscriptionID, request)
	if err != nil {
		t.Fatalf("submit valid observation: %v", err)
	}
	if result.Created != 1 || repository.observationCalls != 1 {
		t.Fatalf("result=%#v calls=%d", result, repository.observationCalls)
	}
	observation := repository.observation
	if observation.DeviceID != "iphone-a" || observation.MutationID != mutationID ||
		observation.SubscriptionID != subscriptionID || observation.ContentHash != strings.Repeat("a", 64) ||
		len(observation.RequestHash) != 64 || !observation.AcceptedAt.Equal(now) ||
		len(observation.Entries) != 1 || len(observation.CanonicalEntries) != 1 {
		t.Fatalf("normalized observation: %#v", observation)
	}
	canonical := observation.CanonicalEntries[0]
	if canonical.OriginKey != entry.OriginKey || !canonical.ObservedAt.Equal(request.FetchedAt) ||
		canonical.SubscriptionID != subscriptionID || canonical.URL != entry.CanonicalLink ||
		!strings.Contains(canonical.ContentHTML, "Safe body") || strings.Contains(strings.ToLower(canonical.ContentHTML), "script") ||
		len(canonical.Media) != 1 || canonical.Media[0].MIMEType != "audio/mpeg" {
		t.Fatalf("observation bypassed canonical ingest/sanitizer: %#v", canonical)
	}
	firstHash := observation.RequestHash
	if _, err := service.SubmitFeedObservationForDevice(context.Background(), "iphone-a", subscriptionID, request); err != nil {
		t.Fatalf("repeat normalized observation: %v", err)
	}
	if repository.observation.RequestHash != firstHash {
		t.Fatalf("same observation produced a different request hash: %q -> %q", firstHash, repository.observation.RequestHash)
	}

	invalidRequests := []FeedObservationRequest{
		func() FeedObservationRequest {
			value := request
			value.Entries = append([]domainrss.ObservationEntry(nil), request.Entries...)
			value.Entries[0].OriginKey = "rss-origin-v1:" + strings.Repeat("0", 64)
			return value
		}(),
		func() FeedObservationRequest {
			value := request
			value.Entries = append([]domainrss.ObservationEntry(nil), request.Entries...)
			value.Entries[0].CanonicalLink = "https://127.0.0.1/private"
			value.Entries[0].OriginKey = RSSOriginKeyV1(subscriptionID, value.Entries[0].GUID, value.Entries[0].CanonicalLink,
				value.Entries[0].Title, value.Entries[0].PublishedAt, value.Entries[0].Enclosures)
			return value
		}(),
		func() FeedObservationRequest {
			value := request
			value.UpstreamETag = "bad\r\nInjected: header"
			return value
		}(),
		func() FeedObservationRequest {
			value := request
			value.ContentHash = "not-a-sha256"
			return value
		}(),
	}
	for index, invalid := range invalidRequests {
		before := repository.observationCalls
		if _, err := service.SubmitFeedObservationForDevice(context.Background(), "iphone-a", subscriptionID, invalid); err == nil {
			t.Fatalf("invalid observation %d was accepted", index)
		}
		if repository.observationCalls != before {
			t.Fatalf("invalid observation %d reached repository", index)
		}
	}

	deviceLocalRepository := &sharedPublicServiceRepositoryStub{syncServiceRepositoryStub: &syncServiceRepositoryStub{
		subscriptions: []domainrss.Subscription{{ID: subscriptionID, SourceAccess: domainrss.SubscriptionSourceDesktopManaged}},
	}}
	deviceLocalService := NewService(deviceLocalRepository, nil)
	deviceLocalService.now = func() time.Time { return now }
	if _, err := deviceLocalService.SubmitFeedObservationForDevice(context.Background(), "iphone-a", subscriptionID, request); err == nil {
		t.Fatal("desktopManaged subscription accepted a device observation")
	}
	if deviceLocalRepository.observationCalls != 0 {
		t.Fatal("desktopManaged observation reached repository")
	}
}

func TestSharedPublicLeaseApplicationBoundaryValidatesAndForwardsTTL(t *testing.T) {
	now := time.Date(2026, 7, 21, 14, 0, 0, 0, time.UTC)
	repository := &sharedPublicServiceRepositoryStub{
		syncServiceRepositoryStub: &syncServiceRepositoryStub{},
		leaseResult:               domainrss.FetchLeaseResult{Granted: true, LeaseID: "repository-lease"},
	}
	service := NewService(repository, nil)
	service.now = func() time.Time { return now }
	const subscriptionID = "248c47e6-87b3-4609-88cf-da31f688dc53"
	result, err := service.AcquireFetchLeaseForDevice(context.Background(), " iphone-a ", subscriptionID,
		FetchLeaseApplicationRequest{})
	if err != nil {
		t.Fatalf("acquire default lease: %v", err)
	}
	if !result.Granted || repository.leaseCalls != 1 || repository.lease.DeviceID != "iphone-a" ||
		repository.lease.SubscriptionID != subscriptionID || repository.lease.RequestedTTL != 2*time.Minute ||
		repository.lease.LeaseID == "" || !repository.lease.RequestedAt.Equal(now) {
		t.Fatalf("forwarded lease request: %#v", repository.lease)
	}
	if _, err := service.AcquireFetchLeaseForDevice(context.Background(), "iphone-a", subscriptionID,
		FetchLeaseApplicationRequest{TTLSeconds: 601}); err == nil {
		t.Fatal("over-limit lease TTL was accepted")
	}
	if repository.leaseCalls != 1 {
		t.Fatalf("invalid lease reached repository: %d calls", repository.leaseCalls)
	}

	repository.leaseErr = errors.New("repository failure")
	if _, err := service.AcquireFetchLeaseForDevice(context.Background(), "iphone-a", subscriptionID,
		FetchLeaseApplicationRequest{TTLSeconds: 30}); !errors.Is(err, repository.leaseErr) {
		t.Fatalf("repository lease error = %v", err)
	}
}
