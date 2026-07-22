package rss

import (
	"context"
	"errors"
	"testing"

	domainrss "xiadown/internal/domain/rss"
)

type rssResourceRepositoryStub struct {
	domainrss.Repository
	subscription domainrss.Subscription
	entry        domainrss.Entry
}

func (stub rssResourceRepositoryStub) GetSubscription(_ context.Context, id string) (domainrss.Subscription, error) {
	if id != stub.subscription.ID {
		return domainrss.Subscription{}, domainrss.ErrNotFound
	}
	return stub.subscription, nil
}

func (stub rssResourceRepositoryStub) GetEntry(_ context.Context, id string) (domainrss.Entry, error) {
	if id != stub.entry.ID {
		return domainrss.Entry{}, domainrss.ErrNotFound
	}
	return stub.entry, nil
}

func TestResolveRSSRemoteResourceUsesPersistedOpaqueSlots(t *testing.T) {
	repository := rssResourceRepositoryStub{
		subscription: domainrss.Subscription{
			ID: "subscription-1", WorkspaceID: domainrss.DefaultWorkspaceID,
			SiteURL: "https://publisher.example/home?session=publisher-secret#section",
			IconURL: "https://cdn.example/icon.png#fragment",
		},
		entry: domainrss.Entry{
			ID: "entry-1", SubscriptionID: "subscription-1",
			URL:          "https://articles.publisher.example/posts/1?token=entry-secret#comments",
			ThumbnailURL: "https://cdn.example/cover.jpg",
			ImageURLs:    []string{"https://cdn.example/body.jpg"},
			Media: []domainrss.Media{{
				URL: "https://cdn.example/movie.mp4", Kind: "video", MIMEType: "video/mp4",
				Thumbnail: "https://cdn.example/poster.jpg",
			}},
		},
	}
	service := NewService(repository, nil)

	icon, err := service.ResolveSubscriptionResource(context.Background(), "subscription-1")
	if err != nil || icon.URL != "https://cdn.example/icon.png" || icon.Kind != RemoteResourceImage ||
		icon.Role != RemoteResourceRoleIcon || icon.RefererOrigin != "https://publisher.example/" {
		t.Fatalf("icon = %#v, err = %v", icon, err)
	}
	image, err := service.ResolveEntryResource(context.Background(), "entry-1", "image-0")
	if err != nil || image.URL != "https://cdn.example/body.jpg" || image.Kind != RemoteResourceImage ||
		image.Role != RemoteResourceRoleContentImage || image.RefererOrigin != "https://articles.publisher.example/" {
		t.Fatalf("image = %#v, err = %v", image, err)
	}
	media, err := service.ResolveEntryResource(context.Background(), "entry-1", "media-0")
	if err != nil || media.URL != "https://cdn.example/movie.mp4" || media.Kind != RemoteResourceMedia || media.MIMEType != "video/mp4" || media.Role != RemoteResourceRoleMedia {
		t.Fatalf("media = %#v, err = %v", media, err)
	}
	poster, err := service.ResolveEntryResource(context.Background(), "entry-1", "media-0-thumbnail")
	if err != nil || poster.URL != "https://cdn.example/poster.jpg" || poster.Kind != RemoteResourceImage || poster.Role != RemoteResourceRoleMediaThumbnail {
		t.Fatalf("poster = %#v, err = %v", poster, err)
	}
}

func TestResolveRSSDiscoveryIconsUsesOpaquePersistedCatalogIDs(t *testing.T) {
	route := newDiscoveryRoute(discoveryRouteInput{
		SourceID: "bilibili", SourceName: "Bilibili", SourceURL: "https://www.bilibili.com/read/catalog?secret=source#routes",
		SiteURL:   "https://www.bilibili.com/video/BV1example?secret=site#player",
		RoutePath: "bilibili/ranking/0", ExamplePath: "bilibili/ranking/0",
		Title: "Bilibili ranking", URL: "rsshub://bilibili/ranking/0", Categories: []string{"multimedia"},
	})
	repository := &discoveryRepositoryStub{cache: domainrss.DiscoveryCache{Routes: []domainrss.DiscoveryRoute{route}}}
	service := NewService(repository, nil)

	for _, test := range []struct {
		kind DiscoveryResourceKind
		id   string
	}{
		{kind: DiscoveryResourceRouteIcon, id: route.ID},
		{kind: DiscoveryResourceCategoryIcon, id: "multimedia"},
		{kind: DiscoveryResourceCategoryIcon, id: "all"},
	} {
		resource, err := service.ResolveDiscoveryResource(context.Background(), test.kind, test.id)
		if err != nil {
			t.Fatalf("ResolveDiscoveryResource(%q, %q): %v", test.kind, test.id, err)
		}
		if resource.URL != "https://www.bilibili.com/favicon.ico" ||
			resource.Kind != RemoteResourceImage || resource.Role != RemoteResourceRoleIcon ||
			resource.RefererOrigin != "https://www.bilibili.com/" {
			t.Fatalf("resource = %#v", resource)
		}
	}

	for _, test := range []struct {
		kind DiscoveryResourceKind
		id   string
	}{
		{kind: DiscoveryResourceRouteIcon, id: "rsshub:missing"},
		{kind: DiscoveryResourceCategoryIcon, id: "missing"},
		{kind: DiscoveryResourceKind("sources"), id: "bilibili"},
	} {
		if _, err := service.ResolveDiscoveryResource(context.Background(), test.kind, test.id); !errors.Is(err, domainrss.ErrNotFound) {
			t.Fatalf("ResolveDiscoveryResource(%q, %q) error = %v", test.kind, test.id, err)
		}
	}
}

func TestRSSRemoteResourceRefererOriginIsPublicAndOriginOnly(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "public path and query", raw: "https://sspai.com/post/123?token=secret#comments", want: "https://sspai.com/"},
		{name: "public non-default port", raw: "http://publisher.example:8080/path", want: "http://publisher.example:8080/"},
		{name: "private literal", raw: "http://127.0.0.1/private", want: ""},
		{name: "special-use host", raw: "https://metadata.google.internal/latest", want: ""},
		{name: "credentialed URL", raw: "https://user:password@publisher.example/path", want: ""},
		{name: "non-http URL", raw: "file:///etc/passwd", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			resource := RemoteResource{RefererOrigin: test.raw}
			if got := resource.SafeRefererOrigin(); got != test.want {
				t.Fatalf("SafeRefererOrigin(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestResolveRSSRemoteResourceRejectsUnknownAndPrivateSources(t *testing.T) {
	repository := rssResourceRepositoryStub{
		subscription: domainrss.Subscription{
			ID: "subscription-1", WorkspaceID: domainrss.DefaultWorkspaceID,
			IconURL: "http://127.0.0.1/private.png",
		},
		entry: domainrss.Entry{
			ID: "entry-1", SubscriptionID: "subscription-1",
			ImageURLs: []string{"http://metadata.internal/latest"},
			Media:     []domainrss.Media{{URL: "https://cdn.example/file.bin", Kind: "document"}},
		},
	}
	service := NewService(repository, nil)
	for _, resolve := range []func() error{
		func() error {
			_, err := service.ResolveSubscriptionResource(context.Background(), "subscription-1")
			return err
		},
		func() error {
			_, err := service.ResolveEntryResource(context.Background(), "entry-1", "image-0")
			return err
		},
		func() error {
			_, err := service.ResolveEntryResource(context.Background(), "entry-1", "image-1")
			return err
		},
		func() error {
			_, err := service.ResolveEntryResource(context.Background(), "entry-1", "media-0")
			return err
		},
		func() error {
			_, err := service.ResolveEntryResource(context.Background(), "entry-1", "../thumbnail")
			return err
		},
	} {
		if err := resolve(); !errors.Is(err, domainrss.ErrNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	}
}

func TestIndexedResourceSlotRequiresASCIIUnsignedDecimal(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		slot   string
		prefix string
		suffix string
		index  int
	}{
		{name: "image zero", slot: "image-0", prefix: "image-", index: 0},
		{name: "media maximum", slot: "media-63", prefix: "media-", index: 63},
		{name: "media thumbnail", slot: "media-1-thumbnail", prefix: "media-", suffix: "-thumbnail", index: 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			index, ok := indexedResourceSlot(testCase.slot, testCase.prefix, testCase.suffix)
			if !ok || index != testCase.index {
				t.Fatalf("indexedResourceSlot(%q) = %d, %v", testCase.slot, index, ok)
			}
		})
	}

	for _, slot := range []string{
		"image-",
		"image-+1",
		"image--0",
		"image-00",
		"image-01",
		"image- 1",
		"image-\uff11",
		"image-64",
	} {
		t.Run("reject "+slot, func(t *testing.T) {
			if index, ok := indexedResourceSlot(slot, "image-", ""); ok {
				t.Fatalf("indexedResourceSlot(%q) unexpectedly accepted index %d", slot, index)
			}
		})
	}
}
