package libraryapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	applicationrss "xiadown/internal/application/rss"
	domainrss "xiadown/internal/domain/rss"
)

type rssResourceRoundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip rssResourceRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type rssMutableHTTPClientProvider struct {
	client *http.Client
	calls  int
}

func (provider *rssMutableHTTPClientProvider) HTTPClient() *http.Client {
	provider.calls++
	return provider.client
}

func (provider *rssMutableHTTPClientProvider) PublicDialURLContext(
	_ context.Context,
	_, _ string,
	logicalURL *url.URL,
) (net.Conn, error) {
	transport, ok := provider.client.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		return nil, errors.New("test managed route is unavailable")
	}
	_, err := transport.Proxy(&http.Request{URL: logicalURL})
	if err != nil {
		return nil, err
	}
	return nil, errors.New("test managed route unexpectedly accepted the request")
}

type rssBlockingReadCloser struct {
	closed chan struct{}
	once   sync.Once
}

func newRSSBlockingReadCloser() *rssBlockingReadCloser {
	return &rssBlockingReadCloser{closed: make(chan struct{})}
}

func (body *rssBlockingReadCloser) Read([]byte) (int, error) {
	<-body.closed
	return 0, io.ErrClosedPipe
}

func (body *rssBlockingReadCloser) Close() error {
	body.once.Do(func() { close(body.closed) })
	return nil
}

type rssFailingReadCloser struct {
	reader  *bytes.Reader
	failure error
}

func (body *rssFailingReadCloser) Read(buffer []byte) (int, error) {
	if body.reader.Len() > 0 {
		return body.reader.Read(buffer)
	}
	return 0, body.failure
}

func (body *rssFailingReadCloser) Close() error { return nil }

type rssPrefixThenBlockingReadCloser struct {
	reader *bytes.Reader
	closed chan struct{}
	once   sync.Once
}

func (body *rssPrefixThenBlockingReadCloser) Read(buffer []byte) (int, error) {
	if body.reader.Len() > 0 {
		return body.reader.Read(buffer)
	}
	<-body.closed
	return 0, io.ErrClosedPipe
}

func (body *rssPrefixThenBlockingReadCloser) Close() error {
	body.once.Do(func() { close(body.closed) })
	return nil
}

type rssPacedReadCloser struct {
	reader   *bytes.Reader
	pattern  []byte
	delay    time.Duration
	maxChunk int
	closed   chan struct{}
	once     sync.Once
}

func (body *rssPacedReadCloser) Read(buffer []byte) (int, error) {
	select {
	case <-body.closed:
		return 0, io.ErrClosedPipe
	case <-time.After(body.delay):
	}
	if body.maxChunk > 0 && len(buffer) > body.maxChunk {
		buffer = buffer[:body.maxChunk]
	}
	if body.reader != nil {
		return body.reader.Read(buffer)
	}
	if len(body.pattern) == 0 {
		body.pattern = []byte{0}
	}
	for index := range buffer {
		buffer[index] = body.pattern[index%len(body.pattern)]
	}
	return len(buffer), nil
}

func (body *rssPacedReadCloser) Close() error {
	body.once.Do(func() { close(body.closed) })
	return nil
}

type rssResourceServiceStub struct {
	*rssServiceStub
	subscription applicationrss.RemoteResource
	entry        applicationrss.RemoteResource
	entryID      string
	slot         string
}

type rssStaticResourceServiceStub struct {
	*rssServiceStub
	resource applicationrss.RemoteResource
}

type rssDiscoveryResourceServiceStub struct {
	*rssResourceServiceStub
	discovery     applicationrss.RemoteResource
	discoveryKind applicationrss.DiscoveryResourceKind
	discoveryID   string
}

type rssSurfaceBoundaryResourceService struct {
	*rssServiceStub
	resource            applicationrss.RemoteResource
	desktopSubscription int
	desktopEntry        int
	pairedSubscription  int
	pairedEntry         int
}

func (stub *rssSurfaceBoundaryResourceService) ResolveSubscriptionResource(context.Context, string) (applicationrss.RemoteResource, error) {
	stub.desktopSubscription++
	return stub.resource, nil
}

func (stub *rssSurfaceBoundaryResourceService) ResolveEntryResource(context.Context, string, string) (applicationrss.RemoteResource, error) {
	stub.desktopEntry++
	return stub.resource, nil
}

func (stub *rssSurfaceBoundaryResourceService) ResolveSyncSubscriptionResource(context.Context, string) (applicationrss.RemoteResource, error) {
	stub.pairedSubscription++
	return applicationrss.RemoteResource{}, domainrss.ErrNotFound
}

func (stub *rssSurfaceBoundaryResourceService) ResolveSyncEntryResource(context.Context, string, string) (applicationrss.RemoteResource, error) {
	stub.pairedEntry++
	return applicationrss.RemoteResource{}, domainrss.ErrNotFound
}

func (stub *rssStaticResourceServiceStub) ResolveSubscriptionResource(context.Context, string) (applicationrss.RemoteResource, error) {
	return stub.resource, nil
}

func (stub *rssStaticResourceServiceStub) ResolveEntryResource(context.Context, string, string) (applicationrss.RemoteResource, error) {
	return stub.resource, nil
}

func (stub *rssStaticResourceServiceStub) ResolveSyncSubscriptionResource(context.Context, string) (applicationrss.RemoteResource, error) {
	return stub.resource, nil
}

func (stub *rssStaticResourceServiceStub) ResolveSyncEntryResource(context.Context, string, string) (applicationrss.RemoteResource, error) {
	return stub.resource, nil
}

func (stub *rssResourceServiceStub) ResolveSubscriptionResource(context.Context, string) (applicationrss.RemoteResource, error) {
	return stub.subscription, nil
}

func (stub *rssResourceServiceStub) ResolveEntryResource(_ context.Context, id, slot string) (applicationrss.RemoteResource, error) {
	stub.entryID = id
	stub.slot = slot
	return stub.entry, nil
}

func (stub *rssResourceServiceStub) ResolveSyncSubscriptionResource(ctx context.Context, id string) (applicationrss.RemoteResource, error) {
	return stub.ResolveSubscriptionResource(ctx, id)
}

func (stub *rssResourceServiceStub) ResolveSyncEntryResource(ctx context.Context, id, slot string) (applicationrss.RemoteResource, error) {
	return stub.ResolveEntryResource(ctx, id, slot)
}

func (stub *rssDiscoveryResourceServiceStub) ResolveDiscoveryResource(
	_ context.Context,
	kind applicationrss.DiscoveryResourceKind,
	id string,
) (applicationrss.RemoteResource, error) {
	stub.discoveryKind = kind
	stub.discoveryID = id
	return stub.discovery, nil
}

func TestValidRSSResourceSlotRequiresCanonicalBoundedIndex(t *testing.T) {
	for _, slot := range []string{
		"thumbnail",
		"image-0",
		"image-63",
		"media-0",
		"media-63",
		"media-0-thumbnail",
		"media-63-thumbnail",
	} {
		if !validRSSResourceSlot(slot) {
			t.Errorf("valid slot %q was rejected", slot)
		}
	}
	for _, slot := range []string{
		"image-",
		"image-00",
		"image-01",
		"image-+1",
		"image--0",
		"image-64",
		"image-1-thumbnail",
		"media-00",
		"media-01-thumbnail",
		"media-64",
		"media-64-thumbnail",
		"media-\uff11",
	} {
		if validRSSResourceSlot(slot) {
			t.Errorf("invalid slot %q was accepted", slot)
		}
	}
}

func TestRSSResourceSurfacesUseSeparatePublicEligibilityResolvers(t *testing.T) {
	payload := rssCacheTestPNG(t, color.RGBA{R: 80, G: 120, B: 160, A: 255})
	service := &rssSurfaceBoundaryResourceService{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://cdn.example/local.png", Kind: applicationrss.RemoteResourceImage,
			Role: applicationrss.RemoteResourceRoleContentImage,
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
			Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: request,
		}, nil
	})}

	desktopHandler := api.LocalResourceHandler()
	for _, target := range []string{
		"/api/rss/subscriptions/local-subscription/icon",
		"/api/rss/entries/local-entry/resources/image-0",
	} {
		recorder := httptest.NewRecorder()
		desktopHandler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("desktop resource %s = %d %q", target, recorder.Code, recorder.Body.String())
		}
	}

	pairedSubscription := httptest.NewRequest(http.MethodGet, "/api/v1/rss/subscriptions/local-subscription/icon", nil)
	pairedSubscription.SetPathValue("id", "local-subscription")
	pairedSubscriptionRecorder := httptest.NewRecorder()
	api.getSubscriptionResource(pairedSubscriptionRecorder, pairedSubscription)
	if pairedSubscriptionRecorder.Code != http.StatusNotFound {
		t.Fatalf("paired subscription resource=%d %q", pairedSubscriptionRecorder.Code, pairedSubscriptionRecorder.Body.String())
	}
	pairedEntry := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/local-entry/resources/image-0", nil)
	pairedEntry.SetPathValue("id", "local-entry")
	pairedEntry.SetPathValue("slot", "image-0")
	pairedEntryRecorder := httptest.NewRecorder()
	api.getEntryResource(pairedEntryRecorder, pairedEntry)
	if pairedEntryRecorder.Code != http.StatusNotFound {
		t.Fatalf("paired entry resource=%d %q", pairedEntryRecorder.Code, pairedEntryRecorder.Body.String())
	}
	if service.desktopSubscription != 1 || service.desktopEntry != 1 ||
		service.pairedSubscription != 1 || service.pairedEntry != 1 {
		t.Fatalf("resolver calls desktop=(%d,%d) paired=(%d,%d)",
			service.desktopSubscription, service.desktopEntry,
			service.pairedSubscription, service.pairedEntry)
	}
}

func TestRSSResourceProxyServesAntiHotlinkRasterWithOnlyServerDerivedReferer(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	service := &rssResourceServiceStub{
		rssServiceStub: &rssServiceStub{},
		entry: applicationrss.RemoteResource{
			URL: "https://cdn.example/cover.png", Kind: applicationrss.RemoteResourceImage,
			RefererOrigin: "https://sspai.com/post/123?token=publisher-secret#comments",
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		for _, forbidden := range []string{
			"Authorization", "Cookie", "Origin", "Range",
			"If-None-Match", "If-Modified-Since", "If-Range",
		} {
			if value := request.Header.Get(forbidden); value != "" {
				t.Errorf("upstream %s = %q", forbidden, value)
			}
		}
		if got := request.Header.Get("Referer"); got != "https://sspai.com/" {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("anti-hotlink")),
				Request:    request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"image/png"}, "Etag": {`"cover-v1"`},
			},
			Body: io.NopCloser(bytes.NewReader(png)), ContentLength: int64(len(png)), Request: request,
		}, nil
	})}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/entry-1/resources/image-0", nil)
	request.SetPathValue("id", "entry-1")
	request.SetPathValue("slot", "image-0")
	request.Header.Set("Authorization", "Bearer paired-device-secret")
	request.Header.Set("Cookie", "app-session=secret")
	request.Header.Set("Origin", "https://reader.example")
	request.Header.Set("Referer", "https://reader.example/feed")
	request.Header.Set("Range", "bytes=0-10")
	request.Header.Set("If-None-Match", `"old-slot-etag"`)
	request.Header.Set("If-Modified-Since", "Mon, 13 Jul 2026 09:00:00 GMT")
	request.Header.Set("If-Range", `"old-slot-range"`)
	recorder := httptest.NewRecorder()

	api.getEntryResource(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/png" ||
		recorder.Header().Get("Cache-Control") != "private, no-store" ||
		recorder.Header().Get("ETag") != "" || recorder.Header().Get("Last-Modified") != "" ||
		!bytes.Equal(recorder.Body.Bytes(), png) {
		t.Fatalf("response = %d %#v %q", recorder.Code, recorder.Header(), recorder.Body.Bytes())
	}
	if service.entryID != "entry-1" || service.slot != "image-0" {
		t.Fatalf("resolved = %q %q", service.entryID, service.slot)
	}
}

func TestRSSResourceProxyDropsPrivateRefererMetadata(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	service := &rssStaticResourceServiceStub{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://cdn.example/icon.png", Kind: applicationrss.RemoteResourceImage,
			Role: applicationrss.RemoteResourceRoleIcon, RefererOrigin: "http://127.0.0.1/private?secret=value",
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Referer"); got != "" {
			t.Errorf("private Referer metadata reached upstream: %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(png)),
			Request:    request,
		}, nil
	})}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/subscriptions/subscription-1/icon", nil)
	request.SetPathValue("id", "subscription-1")
	request.Header.Set("Referer", "https://client.example/private-page")
	recorder := httptest.NewRecorder()

	api.getSubscriptionResource(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("response status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
}

func TestRSSResourceProxyPreservesBoundedSingleRangeForDirectMedia(t *testing.T) {
	mp4 := append([]byte{0, 0, 0, 24}, []byte("ftypmp42safe-media")...)
	rangeEnd := len(mp4) - 1
	service := &rssResourceServiceStub{
		rssServiceStub: &rssServiceStub{},
		entry: applicationrss.RemoteResource{
			URL: "https://cdn.example/movie.mp4", Kind: applicationrss.RemoteResourceMedia, MIMEType: "video/mp4",
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Range"); got != fmt.Sprintf("bytes=0-%d", rangeEnd) {
			t.Errorf("upstream Range = %q", got)
		}
		if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" || request.Header.Get("If-Range") != "" {
			t.Error("credentials reached media origin")
		}
		return &http.Response{
			StatusCode: http.StatusPartialContent,
			Header: http.Header{
				"Content-Type": {"video/mp4"}, "Content-Range": {fmt.Sprintf("bytes 0-%d/1024", rangeEnd)},
				"Accept-Ranges": {"bytes"}, "Last-Modified": {"Mon, 13 Jul 2026 10:00:00 GMT"},
			},
			Body: io.NopCloser(bytes.NewReader(mp4)), ContentLength: int64(len(mp4)), Request: request,
		}, nil
	})}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/entry-1/resources/media-0", nil)
	request.SetPathValue("id", "entry-1")
	request.SetPathValue("slot", "media-0")
	request.Header.Set("Authorization", "Bearer paired-device-secret")
	request.Header.Set("Range", fmt.Sprintf("bytes=0-%d", rangeEnd))
	request.Header.Set("If-Range", `"previous-source"`)
	recorder := httptest.NewRecorder()

	api.getEntryResource(recorder, request)
	if recorder.Code != http.StatusPartialContent || recorder.Header().Get("Content-Type") != "video/mp4" ||
		recorder.Header().Get("Content-Range") != fmt.Sprintf("bytes 0-%d/1024", rangeEnd) ||
		recorder.Header().Get("Accept-Ranges") != "bytes" ||
		recorder.Header().Get("Cache-Control") != "private, no-store" ||
		recorder.Header().Get("Last-Modified") != "" ||
		!bytes.Equal(recorder.Body.Bytes(), mp4) {
		t.Fatalf("response = %d %#v %q", recorder.Code, recorder.Header(), recorder.Body.Bytes())
	}
}

func TestRSSResourceProxyValidatesRangeResponsesAndBoundsChunked206(t *testing.T) {
	service := &rssResourceServiceStub{
		rssServiceStub: &rssServiceStub{},
		entry: applicationrss.RemoteResource{
			URL: "https://cdn.example/movie.mp4", Kind: applicationrss.RemoteResourceMedia, MIMEType: "video/mp4",
		},
	}
	t.Run("chunked response is capped and extra byte is consumed", func(t *testing.T) {
		api, err := NewRSSAPI(service)
		if err != nil {
			t.Fatal(err)
		}
		payload := []byte{0, 0, 0, 10, 'f', 't', 'y', 'p', 'm', 'p'}
		reader := bytes.NewReader(append(append([]byte{}, payload...), 'x'))
		api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusPartialContent,
				Header:     http.Header{"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 0-9/20"}},
				Body:       io.NopCloser(reader), ContentLength: -1, Request: request,
			}, nil
		})}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/media-0", nil)
		request.SetPathValue("id", "e")
		request.SetPathValue("slot", "media-0")
		request.Header.Set("Range", "bytes=0-9")
		recorder := httptest.NewRecorder()
		api.getEntryResource(recorder, request)
		if recorder.Code != http.StatusPartialContent || !bytes.Equal(recorder.Body.Bytes(), payload) ||
			recorder.Header().Get("Content-Length") != "10" || reader.Len() != 0 {
			t.Fatalf("chunked 206 = %d headers=%#v body=%q remaining=%d", recorder.Code, recorder.Header(), recorder.Body.Bytes(), reader.Len())
		}
	})

	t.Run("mismatched content range fails closed", func(t *testing.T) {
		api, err := NewRSSAPI(service)
		if err != nil {
			t.Fatal(err)
		}
		api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusPartialContent,
				Header:     http.Header{"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 5-14/20"}},
				Body:       io.NopCloser(strings.NewReader("0123456789")), ContentLength: 10, Request: request,
			}, nil
		})}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/media-0", nil)
		request.SetPathValue("id", "e")
		request.SetPathValue("slot", "media-0")
		request.Header.Set("Range", "bytes=0-9")
		recorder := httptest.NewRecorder()
		api.getEntryResource(recorder, request)
		if recorder.Code != http.StatusBadGateway {
			t.Fatalf("mismatched 206 status = %d", recorder.Code)
		}
	})

	t.Run("upstream unsatisfiable maps to 416", func(t *testing.T) {
		api, err := NewRSSAPI(service)
		if err != nil {
			t.Fatal(err)
		}
		api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusRequestedRangeNotSatisfiable,
				Header:     http.Header{"Content-Range": {"bytes */20"}},
				Body:       http.NoBody, ContentLength: 0, Request: request,
			}, nil
		})}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/media-0", nil)
		request.SetPathValue("id", "e")
		request.SetPathValue("slot", "media-0")
		request.Header.Set("Range", "bytes=99-100")
		recorder := httptest.NewRecorder()
		api.getEntryResource(recorder, request)
		if recorder.Code != http.StatusRequestedRangeNotSatisfiable || recorder.Header().Get("Content-Range") != "bytes */20" {
			t.Fatalf("upstream 416 = %d %#v", recorder.Code, recorder.Header())
		}
	})
}

func TestRSSResourceProxyRejectsInvalidRangesAndActiveImageContent(t *testing.T) {
	service := &rssResourceServiceStub{
		rssServiceStub: &rssServiceStub{},
		entry: applicationrss.RemoteResource{
			URL: "https://cdn.example/cover.png", Kind: applicationrss.RemoteResourceImage,
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`<html><script>fetch('/private')</script></html>`)
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
			Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: request,
		}, nil
	})}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/entry-1/resources/image-0", nil)
	request.SetPathValue("id", "entry-1")
	request.SetPathValue("slot", "image-0")
	recorder := httptest.NewRecorder()
	api.getEntryResource(recorder, request)
	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "image unavailable") {
		t.Fatalf("active image response = %d %q", recorder.Code, recorder.Body.String())
	}

	service.entry.Kind = applicationrss.RemoteResourceMedia
	service.entry.MIMEType = "video/mp4"
	badRange := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/entry-1/resources/media-0", nil)
	badRange.SetPathValue("id", "entry-1")
	badRange.SetPathValue("slot", "media-0")
	badRange.Header.Set("Range", "bytes=0-10,20-30")
	recorder = httptest.NewRecorder()
	api.getEntryResource(recorder, badRange)
	if recorder.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("multi-range status = %d", recorder.Code)
	}
}

func TestRSSLocalResourceHandlerHasNoRawURLRelay(t *testing.T) {
	service := &rssResourceServiceStub{rssServiceStub: &rssServiceStub{}}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/rss/resource?url=http://127.0.0.1/private", nil)
	recorder := httptest.NewRecorder()
	api.LocalResourceHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("raw URL relay status = %d", recorder.Code)
	}
	if recorder.Header().Get("Cache-Control") != "private, no-store" ||
		recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("local resource error policy = %#v", recorder.Header())
	}
}

func TestRSSLocalResourceHandlerProxiesOpaqueDiscoveryIcons(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	service := &rssDiscoveryResourceServiceStub{
		rssResourceServiceStub: &rssResourceServiceStub{rssServiceStub: &rssServiceStub{}},
		discovery: applicationrss.RemoteResource{
			URL: "https://catalog.example/favicon.ico", Kind: applicationrss.RemoteResourceImage,
			Role: applicationrss.RemoteResourceRoleIcon, RefererOrigin: "https://catalog.example/",
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(png)), ContentLength: int64(len(png)), Request: request,
		}, nil
	})}
	handler := api.LocalResourceHandler()
	for _, test := range []struct {
		path string
		kind applicationrss.DiscoveryResourceKind
		id   string
	}{
		{path: "/api/rss/discovery/categories/multimedia/icon", kind: applicationrss.DiscoveryResourceCategoryIcon, id: "multimedia"},
		{path: "/api/rss/discovery/routes/rsshub:bilibili-ranking/icon?v=1", kind: applicationrss.DiscoveryResourceRouteIcon, id: "rsshub:bilibili-ranking"},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || service.discoveryKind != test.kind || service.discoveryID != test.id {
			t.Fatalf("GET %s = %d, resolver=(%q,%q)", test.path, recorder.Code, service.discoveryKind, service.discoveryID)
		}
		if recorder.Header().Get("Cache-Control") != "private, max-age=0, must-revalidate" {
			t.Fatalf("discovery cache policy for %s = %q", test.path, recorder.Header().Get("Cache-Control"))
		}
		if strings.Contains(request.URL.Path, "catalog.example") {
			t.Fatalf("desktop route leaked upstream URL: %q", request.URL.Path)
		}
	}

	for _, path := range []string{
		"/api/rss/discovery/sources/bilibili/icon",
		"/api/rss/discovery/routes//icon",
		"/api/rss/discovery/routes/rsshub:bilibili-ranking/resource",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("invalid discovery resource %q = %d", path, recorder.Code)
		}
	}
}

func TestRSSResourceConcurrencyReservesDesktopLoopbackSlots(t *testing.T) {
	service := &rssStaticResourceServiceStub{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://cdn.example/movie.mp4", Kind: applicationrss.RemoteResourceMedia, MIMEType: "video/mp4",
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	if cap(api.resourceSlots) != defaultMaxConcurrentRSSResourceStreams ||
		cap(api.pairedResourceSlots) != defaultMaxConcurrentPairedRSSResourceStreams ||
		defaultMaxConcurrentRSSResourceStreams-defaultMaxConcurrentPairedRSSResourceStreams != defaultReservedDesktopRSSResourceStreams {
		t.Fatalf("resource slot capacities = total %d paired %d", cap(api.resourceSlots), cap(api.pairedResourceSlots))
	}

	started := make(chan *rssBlockingReadCloser, defaultMaxConcurrentRSSResourceStreams)
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := newRSSBlockingReadCloser()
		started <- body
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"video/mp4"}},
			Body: body, ContentLength: -1, Request: request,
		}, nil
	})}

	results := make(chan int, defaultMaxConcurrentRSSResourceStreams)
	var wait sync.WaitGroup
	var bodies []*rssBlockingReadCloser
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			for _, body := range bodies {
				_ = body.Close()
			}
			wait.Wait()
		})
	}
	defer cleanup()
	waitForStart := func() {
		t.Helper()
		select {
		case body := <-started:
			bodies = append(bodies, body)
		case <-time.After(time.Second):
			cleanup()
			t.Fatal("resource stream did not reach its blocked upstream body")
		}
	}
	startPublic := func() {
		wait.Add(1)
		go func() {
			defer wait.Done()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/media-0", nil)
			request.SetPathValue("id", "e")
			request.SetPathValue("slot", "media-0")
			recorder := httptest.NewRecorder()
			api.getEntryResource(recorder, request)
			results <- recorder.Code
		}()
	}
	localHandler := api.LocalResourceHandler()
	startDesktop := func() {
		wait.Add(1)
		go func() {
			defer wait.Done()
			request := httptest.NewRequest(http.MethodGet, "/api/rss/entries/e/resources/media-0", nil)
			recorder := httptest.NewRecorder()
			localHandler.ServeHTTP(recorder, request)
			results <- recorder.Code
		}()
	}

	for index := 0; index < defaultMaxConcurrentPairedRSSResourceStreams; index++ {
		startPublic()
		waitForStart()
	}
	if got := len(api.pairedResourceSlots); got != defaultMaxConcurrentPairedRSSResourceStreams {
		t.Fatalf("paired slots in use = %d", got)
	}
	if got := len(api.resourceSlots); got != defaultMaxConcurrentPairedRSSResourceStreams {
		t.Fatalf("global slots after paired saturation = %d", got)
	}

	// A paired request cannot claim Desktop capacity by presenting a loopback
	// Host: the surface comes from the statically registered handler.
	pairedOverflow := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/media-0", nil)
	pairedOverflow.Host = "127.0.0.1:8080"
	pairedOverflow.SetPathValue("id", "e")
	pairedOverflow.SetPathValue("slot", "media-0")
	pairedOverflowRecorder := httptest.NewRecorder()
	api.getEntryResource(pairedOverflowRecorder, pairedOverflow)
	if pairedOverflowRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("paired overflow with loopback Host = %d", pairedOverflowRecorder.Code)
	}

	for index := 0; index < defaultReservedDesktopRSSResourceStreams; index++ {
		startDesktop()
		waitForStart()
	}
	if got := len(api.resourceSlots); got != defaultMaxConcurrentRSSResourceStreams {
		t.Fatalf("global slots after Desktop reserve use = %d", got)
	}
	desktopOverflow := httptest.NewRequest(http.MethodGet, "/api/rss/entries/e/resources/media-0", nil)
	desktopOverflowRecorder := httptest.NewRecorder()
	localHandler.ServeHTTP(desktopOverflowRecorder, desktopOverflow)
	if desktopOverflowRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("Desktop overflow = %d", desktopOverflowRecorder.Code)
	}

	cleanup()
	close(results)
	for status := range results {
		if status == http.StatusTooManyRequests {
			t.Fatal("an admitted stream was unexpectedly rejected")
		}
	}
	if len(api.resourceSlots) != 0 || len(api.pairedResourceSlots) != 0 {
		t.Fatalf("resource slots leaked: total=%d paired=%d", len(api.resourceSlots), len(api.pairedResourceSlots))
	}
}

func TestRSSResourceAPIRebuildsRestrictedClientAfterProxyProviderChanges(t *testing.T) {
	proxyFailureClient := func(message string) *http.Client {
		return &http.Client{Transport: &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
			return nil, errors.New(message)
		}}}
	}
	provider := &rssMutableHTTPClientProvider{client: proxyFailureClient("old proxy credentials")}
	api, err := NewRSSAPI(&rssResourceServiceStub{rssServiceStub: &rssServiceStub{}}, provider)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, "http://8.8.8.8/resource", nil)
	_, firstErr := api.currentRSSResourceClient().Do(request)
	if firstErr == nil || !strings.Contains(firstErr.Error(), "old proxy credentials") {
		t.Fatalf("first proxy error = %v", firstErr)
	}
	provider.client = proxyFailureClient("new proxy configuration")
	request, _ = http.NewRequest(http.MethodGet, "http://8.8.8.8/resource", nil)
	_, secondErr := api.currentRSSResourceClient().Do(request)
	if secondErr == nil || !strings.Contains(secondErr.Error(), "new proxy configuration") ||
		strings.Contains(secondErr.Error(), "old proxy credentials") {
		t.Fatalf("updated proxy error = %v", secondErr)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want one per request", provider.calls)
	}
}

func TestRSSResourceProxyEnforcesImageSizeAndRejectsStreamingManifests(t *testing.T) {
	api := &RSSAPI{}
	recorder := httptest.NewRecorder()
	api.proxyRSSImage(recorder, &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
		Body: io.NopCloser(strings.NewReader("ignored")), ContentLength: maxRSSRemoteImageBytes + 1,
	})
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("oversized image status = %d", recorder.Code)
	}

	manifest := []byte("#EXTM3U\n#EXT-X-VERSION:3\n")
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/media-0", nil)
	api.proxyRSSMedia(recorder, request, &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/vnd.apple.mpegurl"}},
		Body:       io.NopCloser(bytes.NewReader(manifest)), ContentLength: int64(len(manifest)),
	}, applicationrss.RemoteResource{
		URL: "https://cdn.example/stream.m3u8", Kind: applicationrss.RemoteResourceMedia,
		MIMEType: "application/vnd.apple.mpegurl",
	})
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("manifest status = %d", recorder.Code)
	}
}

func TestRSSResourceProxyAbsoluteImageTimeoutReleasesStreamSlot(t *testing.T) {
	tests := []struct {
		name   string
		header bool
		body   io.ReadCloser
	}{
		{name: "response header stall", header: true},
		{name: "blocking body", body: newRSSBlockingReadCloser()},
		{name: "trickling body", body: &rssPacedReadCloser{
			pattern: []byte{0x89}, delay: 4 * time.Millisecond, maxChunk: 1, closed: make(chan struct{}),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &rssResourceServiceStub{
				rssServiceStub: &rssServiceStub{},
				entry: applicationrss.RemoteResource{
					URL: "https://cdn.example/cover.png", Kind: applicationrss.RemoteResourceImage,
				},
			}
			api, err := NewRSSAPI(service)
			if err != nil {
				t.Fatal(err)
			}
			api.resourceTimeouts = rssResourceTimeoutPolicy{
				imageTotal: 35 * time.Millisecond, mediaReadIdle: time.Second, mediaTotal: time.Second,
			}
			api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if test.header {
					<-request.Context().Done()
					return nil, request.Context().Err()
				}
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
					Body: test.body, ContentLength: -1, Request: request,
				}, nil
			})}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/image-0", nil)
			request.SetPathValue("id", "e")
			request.SetPathValue("slot", "image-0")
			recorder := httptest.NewRecorder()
			started := time.Now()
			api.getEntryResource(recorder, request)
			if recorder.Code != http.StatusBadGateway {
				t.Fatalf("timeout response = %d %q", recorder.Code, recorder.Body.String())
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("timeout took %s", elapsed)
			}
			if occupied := len(api.resourceSlots); occupied != 0 {
				t.Fatalf("occupied resource slots = %d", occupied)
			}
		})
	}
}

func TestRSSResourceProxyBoundsMediaIdleAndTotalWhileAllowingPacedRange(t *testing.T) {
	// Run the deadline interactions on a fake clock. A busy host must not turn
	// the 4 ms paced source into an apparent 30 ms upstream idle period.
	mp4 := make([]byte, 1024)
	copy(mp4, []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2'})
	tests := []struct {
		name       string
		body       func() *rssPacedReadCloser
		idle       time.Duration
		total      time.Duration
		wantStatus int
		wantBody   []byte
		wantTotal  bool
	}{
		{
			name: "continuous paced range", body: func() *rssPacedReadCloser {
				return &rssPacedReadCloser{
					reader: bytes.NewReader(mp4), delay: 4 * time.Millisecond, maxChunk: 128, closed: make(chan struct{}),
				}
			}, idle: 30 * time.Millisecond, total: 500 * time.Millisecond,
			wantStatus: http.StatusPartialContent, wantBody: mp4,
		},
		{
			name: "continuous trickle reaches absolute limit", body: func() *rssPacedReadCloser {
				return &rssPacedReadCloser{
					pattern: mp4[:128], delay: 4 * time.Millisecond, maxChunk: 128, closed: make(chan struct{}),
				}
			}, idle: 30 * time.Millisecond, total: 25 * time.Millisecond,
			wantStatus: http.StatusPartialContent, wantTotal: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				body := test.body()
				service := &rssResourceServiceStub{
					rssServiceStub: &rssServiceStub{},
					entry: applicationrss.RemoteResource{
						URL: "https://cdn.example/movie.mp4", Kind: applicationrss.RemoteResourceMedia, MIMEType: "video/mp4",
					},
				}
				api, err := NewRSSAPI(service)
				if err != nil {
					t.Fatal(err)
				}
				api.resourceTimeouts = rssResourceTimeoutPolicy{
					imageTotal: time.Second, mediaReadIdle: test.idle, mediaTotal: test.total,
				}
				api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusPartialContent,
						Header: http.Header{
							"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 0-1023/2048"},
							"Accept-Ranges": {"bytes"},
						},
						Body: body, ContentLength: 1024, Request: request,
					}, nil
				})}
				request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/media-0", nil)
				request.SetPathValue("id", "e")
				request.SetPathValue("slot", "media-0")
				request.Header.Set("Range", "bytes=0-1023")
				recorder := httptest.NewRecorder()
				started := time.Now()
				api.getEntryResource(recorder, request)
				elapsed := time.Since(started)
				if recorder.Code != test.wantStatus {
					t.Fatalf("media response = %d %q", recorder.Code, recorder.Body.String())
				}
				if test.wantBody != nil && !bytes.Equal(recorder.Body.Bytes(), test.wantBody) {
					t.Fatalf("paced body length = %d, want %d", recorder.Body.Len(), len(test.wantBody))
				}
				if test.wantTotal {
					if recorder.Body.Len() == 0 || recorder.Body.Len() >= len(mp4) {
						t.Fatalf("total timeout body length = %d, want a non-empty truncated response", recorder.Body.Len())
					}
					if elapsed != test.total {
						t.Fatalf("total timeout elapsed = %s, want %s", elapsed, test.total)
					}
				} else if elapsed <= test.idle {
					t.Fatalf("paced response elapsed = %s, want more than idle window %s", elapsed, test.idle)
				}
				if elapsed > time.Second {
					t.Fatalf("bounded media took %s", elapsed)
				}
				if occupied := len(api.resourceSlots); occupied != 0 {
					t.Fatalf("occupied resource slots = %d", occupied)
				}
			})
		})
	}

	t.Run("idle body", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			body := newRSSBlockingReadCloser()
			service := &rssResourceServiceStub{
				rssServiceStub: &rssServiceStub{},
				entry: applicationrss.RemoteResource{
					URL: "https://cdn.example/movie.mp4", Kind: applicationrss.RemoteResourceMedia, MIMEType: "video/mp4",
				},
			}
			api, err := NewRSSAPI(service)
			if err != nil {
				t.Fatal(err)
			}
			api.resourceTimeouts = rssResourceTimeoutPolicy{
				imageTotal: time.Second, mediaReadIdle: 30 * time.Millisecond, mediaTotal: 500 * time.Millisecond,
			}
			api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"video/mp4"}},
					Body: body, ContentLength: -1, Request: request,
				}, nil
			})}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/media-0", nil)
			request.SetPathValue("id", "e")
			request.SetPathValue("slot", "media-0")
			recorder := httptest.NewRecorder()
			started := time.Now()
			api.getEntryResource(recorder, request)
			if recorder.Code != http.StatusBadGateway || len(api.resourceSlots) != 0 {
				t.Fatalf("idle media = %d slots=%d", recorder.Code, len(api.resourceSlots))
			}
			if elapsed := time.Since(started); elapsed != 30*time.Millisecond {
				t.Fatalf("idle timeout elapsed = %s, want 30ms", elapsed)
			}
		})
	})
}

func TestRSSUnknownLengthMediaHasDetectableTerminalState(t *testing.T) {
	payload := make([]byte, 128<<10)
	copy(payload, []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2'})
	service := &rssStaticResourceServiceStub{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://cdn.example/movie.mp4", Kind: applicationrss.RemoteResourceMedia, MIMEType: "video/mp4",
		},
	}

	newServer := func(t *testing.T, body io.ReadCloser, idleTimeout time.Duration) (*RSSAPI, *httptest.Server) {
		t.Helper()
		api, err := NewRSSAPI(service)
		if err != nil {
			t.Fatal(err)
		}
		api.resourceTimeouts = rssResourceTimeoutPolicy{
			imageTotal: time.Second, mediaReadIdle: idleTimeout, mediaTotal: time.Second,
		}
		api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"video/mp4"}},
				Body: body, ContentLength: -1, Request: request,
			}, nil
		})}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			request.SetPathValue("id", "e")
			request.SetPathValue("slot", "media-0")
			api.getEntryResource(w, request)
		}))
		return api, server
	}

	t.Run("clean EOF sends the affirmative completion trailer", func(t *testing.T) {
		api, server := newServer(t, io.NopCloser(bytes.NewReader(payload)), time.Second)
		defer server.Close()
		response, err := server.Client().Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK || !bytes.Equal(body, payload) {
			t.Fatalf("clean stream = status %d bytes %d err %v", response.StatusCode, len(body), readErr)
		}
		if response.ContentLength != -1 || response.Trailer.Get(rssResourceStreamCompleteTrailer) != "1" {
			t.Fatalf("clean stream metadata = length %d trailer %#v", response.ContentLength, response.Trailer)
		}
		if len(api.resourceSlots) != 0 || len(api.pairedResourceSlots) != 0 {
			t.Fatalf("clean stream leaked slots: total=%d paired=%d", len(api.resourceSlots), len(api.pairedResourceSlots))
		}
	})

	tests := []struct {
		name        string
		body        func() io.ReadCloser
		idleTimeout time.Duration
	}{
		{
			name: "upstream read failure",
			body: func() io.ReadCloser {
				return &rssFailingReadCloser{reader: bytes.NewReader(payload), failure: errors.New("upstream reset")}
			},
			idleTimeout: time.Second,
		},
		{
			name: "upstream read idle timeout",
			body: func() io.ReadCloser {
				return &rssPrefixThenBlockingReadCloser{reader: bytes.NewReader(payload), closed: make(chan struct{})}
			},
			idleTimeout: 25 * time.Millisecond,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, server := newServer(t, test.body(), test.idleTimeout)
			defer server.Close()
			response, requestErr := server.Client().Get(server.URL)
			if requestErr == nil {
				_, readErr := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if readErr == nil {
					t.Fatal("aborted unknown-length stream ended as a successful response")
				}
				if response.Trailer.Get(rssResourceStreamCompleteTrailer) != "" {
					t.Fatalf("aborted stream advertised completion: %#v", response.Trailer)
				}
			}
			deadline := time.Now().Add(time.Second)
			for (len(api.resourceSlots) != 0 || len(api.pairedResourceSlots) != 0) && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if len(api.resourceSlots) != 0 || len(api.pairedResourceSlots) != 0 {
				t.Fatalf("aborted stream leaked slots: total=%d paired=%d", len(api.resourceSlots), len(api.pairedResourceSlots))
			}
		})
	}
}

func TestRSSUnknownLengthMediaLimitProbeAbortsOverflow(t *testing.T) {
	const limit = int64(16 << 10)
	run := func(t *testing.T, payload []byte) (*http.Response, error, error) {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Add("Trailer", rssResourceStreamCompleteTrailer)
			w.WriteHeader(http.StatusOK)
			streamRSSUnknownLengthMedia(w, bufio.NewReader(bytes.NewReader(payload)), limit)
		}))
		defer server.Close()
		response, requestErr := server.Client().Get(server.URL)
		if requestErr != nil {
			return response, requestErr, nil
		}
		_, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		return response, nil, readErr
	}

	t.Run("exact limit completes only after EOF probe", func(t *testing.T) {
		response, requestErr, readErr := run(t, make([]byte, limit))
		if requestErr != nil || readErr != nil {
			t.Fatalf("exact-limit stream errors = request %v read %v", requestErr, readErr)
		}
		if response.Trailer.Get(rssResourceStreamCompleteTrailer) != "1" {
			t.Fatalf("exact-limit completion trailer = %#v", response.Trailer)
		}
	})

	t.Run("one byte over limit resets the response", func(t *testing.T) {
		response, requestErr, readErr := run(t, make([]byte, limit+1))
		if requestErr == nil && readErr == nil {
			t.Fatal("over-limit stream ended as a successful response")
		}
		if response != nil && response.Trailer.Get(rssResourceStreamCompleteTrailer) != "" {
			t.Fatalf("over-limit stream advertised completion: %#v", response.Trailer)
		}
	})
}

func TestRSSResourceProxyRejectsDecodedImageAndAnimationBombs(t *testing.T) {
	var oversizedPNG bytes.Buffer
	if err := png.Encode(&oversizedPNG, image.NewNRGBA(image.Rect(0, 0, maxRSSRemoteImageDimension+1, 1))); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	(&RSSAPI{}).proxyRSSImage(recorder, &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
		Body: io.NopCloser(bytes.NewReader(oversizedPNG.Bytes())), ContentLength: int64(oversizedPNG.Len()),
	})
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("oversized decoded dimensions status = %d", recorder.Code)
	}

	thumbnailPNG := pngBytes(t, image.NewNRGBA(image.Rect(0, 0, 2001, 2001)))
	if !safeRSSRasterImage(thumbnailPNG, "image/png") ||
		safeRSSRasterImageRole(thumbnailPNG, "image/png", applicationrss.RemoteResourceRoleThumbnail) {
		t.Fatal("thumbnail-specific 4 MP decoded-image limit was not enforced")
	}

	palette := color.Palette{color.Black, color.White}
	animation := &gif.GIF{}
	for index := 0; index <= maxRSSRemoteAnimatedImageFrames; index++ {
		frame := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
		animation.Image = append(animation.Image, frame)
		animation.Delay = append(animation.Delay, 0)
	}
	var encodedGIF bytes.Buffer
	if err := gif.EncodeAll(&encodedGIF, animation); err != nil {
		t.Fatal(err)
	}
	if safeRSSRasterImage(encodedGIF.Bytes(), "image/gif") {
		t.Fatal("GIF with too many frames passed the animation bound")
	}
	boundedAnimation := &gif.GIF{
		Image: []*image.Paletted{
			image.NewPaletted(image.Rect(0, 0, 1, 1), palette),
			image.NewPaletted(image.Rect(0, 0, 1, 1), palette),
		},
		Delay: []int{0, 0},
	}
	var boundedGIF bytes.Buffer
	if err := gif.EncodeAll(&boundedGIF, boundedAnimation); err != nil {
		t.Fatal(err)
	}
	if !safeRSSRasterImage(boundedGIF.Bytes(), "image/gif") ||
		safeRSSRasterImageRole(boundedGIF.Bytes(), "image/gif", applicationrss.RemoteResourceRoleIcon) ||
		safeRSSRasterImageRole(boundedGIF.Bytes(), "image/gif", applicationrss.RemoteResourceRoleThumbnail) {
		t.Fatal("animated GIF did not remain content-only")
	}

	animatedWebP := []byte("RIFF\x0c\x00\x00\x00WEBPANIM\x00\x00\x00\x00")
	if safeRSSWebP(animatedWebP) {
		t.Fatal("animated WebP passed the static-only policy")
	}

	largeCanvasGIF := []byte{'G', 'I', 'F', '8', '9', 'a', 0x40, 0x1f, 0xa0, 0x0f, 0, 0, 0}
	for index := 0; index < 6; index++ {
		largeCanvasGIF = append(largeCanvasGIF,
			0x2c, 0, 0, 0, 0, 1, 0, 1, 0, 0, // 1x1 image descriptor
			2, 1, 0, 0, // LZW code size and one data sub-block
		)
	}
	largeCanvasGIF = append(largeCanvasGIF, 0x3b)
	if safeRSSGIF(largeCanvasGIF, 8000, 4000) {
		t.Fatal("GIF exceeded the aggregate canvas-pixel animation bound")
	}

	validPNG := pngBytes(t, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	animationControl := make([]byte, 8)
	binary.BigEndian.PutUint32(animationControl[:4], 1)
	frameControl := make([]byte, 26)
	binary.BigEndian.PutUint32(frameControl[4:8], 2) // wider than the IHDR canvas
	binary.BigEndian.PutUint32(frameControl[8:12], 1)
	iendOffset := bytes.Index(validPNG, []byte("IEND")) - 4
	malformedAPNG := append([]byte{}, validPNG[:iendOffset]...)
	malformedAPNG = append(malformedAPNG, rssTestPNGChunk("acTL", animationControl)...)
	malformedAPNG = append(malformedAPNG, rssTestPNGChunk("fcTL", frameControl)...)
	malformedAPNG = append(malformedAPNG, validPNG[iendOffset:]...)
	if safeRSSRasterImage(malformedAPNG, "image/png") {
		t.Fatal("APNG frame outside the IHDR canvas passed validation")
	}

	ico := make([]byte, 6+16+40)
	copy(ico, []byte{0, 0, 1, 0, 1, 0})
	ico[6], ico[7] = 16, 16
	ico[9] = 0
	putLittleEndian32(ico[14:18], 40)
	putLittleEndian32(ico[18:22], 22)
	putLittleEndian32(ico[22:26], 40)
	putLittleEndian32(ico[26:30], 16)
	putLittleEndian32(ico[30:34], 32)
	if !safeRSSICO(ico) {
		t.Fatal("bounded single-image ICO was rejected")
	}
}

func TestRSSResourceProxyNeverReusesValidatorsAcrossSlotSourceChanges(t *testing.T) {
	pngBody, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	service := &rssResourceServiceStub{
		rssServiceStub: &rssServiceStub{},
		entry: applicationrss.RemoteResource{
			URL: "https://source-a.example/cover.png", Kind: applicationrss.RemoteResourceImage,
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	var origins []string
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		origins = append(origins, request.URL.Host)
		for _, forbidden := range []string{"If-None-Match", "If-Modified-Since", "If-Range"} {
			if got := request.Header.Get(forbidden); got != "" {
				t.Errorf("source %s received %s = %q", request.URL.Host, forbidden, got)
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"image/png"}, "Etag": {`"upstream-etag"`},
				"Last-Modified": {"Mon, 13 Jul 2026 10:00:00 GMT"},
			},
			Body: io.NopCloser(bytes.NewReader(pngBody)), ContentLength: int64(len(pngBody)), Request: request,
		}, nil
	})}

	requestSlot := func(etag string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/e/resources/image-0", nil)
		request.SetPathValue("id", "e")
		request.SetPathValue("slot", "image-0")
		request.Header.Set("If-None-Match", etag)
		request.Header.Set("If-Modified-Since", "Mon, 13 Jul 2026 09:00:00 GMT")
		recorder := httptest.NewRecorder()
		api.getEntryResource(recorder, request)
		return recorder
	}
	first := requestSlot(`"source-a-validator"`)
	service.entry.URL = "https://source-b.example/cover.png"
	second := requestSlot(`"upstream-etag"`)
	if !slices.Equal(origins, []string{"source-a.example", "source-b.example"}) {
		t.Fatalf("upstream origins = %v", origins)
	}
	for index, recorder := range []*httptest.ResponseRecorder{first, second} {
		if recorder.Code != http.StatusOK || recorder.Header().Get("ETag") != "" ||
			recorder.Header().Get("Last-Modified") != "" || recorder.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("response %d = %d %#v", index, recorder.Code, recorder.Header())
		}
	}

	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		origins = append(origins, request.URL.Host)
		for _, forbidden := range []string{"If-None-Match", "If-Modified-Since", "If-Range"} {
			if got := request.Header.Get(forbidden); got != "" {
				t.Errorf("source %s received %s = %q", request.URL.Host, forbidden, got)
			}
		}
		return &http.Response{
			StatusCode: http.StatusNotModified,
			Header:     http.Header{"Etag": {`"unexpected"`}},
			Body:       http.NoBody, Request: request,
		}, nil
	})}
	// Force a new descriptor/cache key. A client validator from source B must
	// never be treated as proof for a newly-resolved source C representation.
	service.entry.URL = "https://source-c.example/cover.png"
	unexpected := requestSlot(`"source-b-validator"`)
	if unexpected.Code != http.StatusBadGateway || unexpected.Header().Get("ETag") != "" ||
		unexpected.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unexpected 304 response = %d %#v", unexpected.Code, unexpected.Header())
	}
	if !slices.Equal(origins, []string{"source-a.example", "source-b.example", "source-c.example"}) {
		t.Fatalf("upstream origins after source change = %v", origins)
	}
}

func pngBytes(t *testing.T, source image.Image) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func rssTestPNGChunk(kind string, data []byte) []byte {
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], []byte(kind))
	copy(chunk[8:8+len(data)], data)
	// The production safety parser is structural; the image decoder validates
	// real PNG CRCs. Tests leave the synthetic ancillary CRC at zero.
	return chunk
}

func putLittleEndian32(destination []byte, value uint32) {
	destination[0] = byte(value)
	destination[1] = byte(value >> 8)
	destination[2] = byte(value >> 16)
	destination[3] = byte(value >> 24)
}
