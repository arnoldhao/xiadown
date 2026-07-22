package libraryapi

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	applicationrss "xiadown/internal/application/rss"
)

type rssMutableImageService struct {
	*rssServiceStub
	mu       sync.RWMutex
	resource applicationrss.RemoteResource
	resolves atomic.Int32
}

func (service *rssMutableImageService) ResolveSubscriptionResource(
	context.Context,
	string,
) (applicationrss.RemoteResource, error) {
	return service.currentResource(), nil
}

func (service *rssMutableImageService) ResolveEntryResource(
	context.Context,
	string,
	string,
) (applicationrss.RemoteResource, error) {
	return service.currentResource(), nil
}

func (service *rssMutableImageService) ResolveSyncSubscriptionResource(
	ctx context.Context,
	id string,
) (applicationrss.RemoteResource, error) {
	return service.ResolveSubscriptionResource(ctx, id)
}

func (service *rssMutableImageService) ResolveSyncEntryResource(
	ctx context.Context,
	id string,
	slot string,
) (applicationrss.RemoteResource, error) {
	return service.ResolveEntryResource(ctx, id, slot)
}

func (service *rssMutableImageService) currentResource() applicationrss.RemoteResource {
	service.resolves.Add(1)
	service.mu.RLock()
	defer service.mu.RUnlock()
	return service.resource
}

func (service *rssMutableImageService) setResource(resource applicationrss.RemoteResource) {
	service.mu.Lock()
	service.resource = resource
	service.mu.Unlock()
}

func TestRSSDesktopImageCacheUsesInternalETagAndStillResolvesEveryRequest(t *testing.T) {
	payload := rssCacheTestPNG(t, color.RGBA{R: 44, G: 88, B: 132, A: 255})
	service := &rssMutableImageService{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://cdn.example/cover.png", Kind: applicationrss.RemoteResourceImage,
			Role: applicationrss.RemoteResourceRoleThumbnail,
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	var upstreamCalls atomic.Int32
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		upstreamCalls.Add(1)
		if request.Header.Get("If-None-Match") != "" || request.Header.Get("If-Modified-Since") != "" {
			t.Fatalf("client validators reached upstream: %#v", request.Header)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":  {"image/png"},
				"Etag":          {`"private-upstream-validator"`},
				"Last-Modified": {"Mon, 13 Jul 2026 10:00:00 GMT"},
			},
			Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: request,
		}, nil
	})}
	handler := api.LocalResourceHandler()
	requestDesktop := func(target, ifNoneMatch string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		if ifNoneMatch != "" {
			request.Header.Set("If-None-Match", ifNoneMatch)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	const desktopTarget = "/api/rss/entries/entry-1/resources/image-0"
	first := requestDesktop(desktopTarget, "")
	if first.Code != http.StatusOK || !bytes.Equal(first.Body.Bytes(), payload) {
		t.Fatalf("first desktop image = %d %#v %d bytes", first.Code, first.Header(), first.Body.Len())
	}
	etag := first.Header().Get("ETag")
	if etag == "" || etag == `"private-upstream-validator"` ||
		first.Header().Get("Cache-Control") != "private, max-age=0, must-revalidate" {
		t.Fatalf("desktop cache headers = %#v", first.Header())
	}
	second := requestDesktop(desktopTarget, etag)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 || second.Header().Get("ETag") != etag {
		t.Fatalf("conditional desktop image = %d %#v %q", second.Code, second.Header(), second.Body.String())
	}
	versioned := requestDesktop(desktopTarget+"?v=42", "")
	if versioned.Code != http.StatusOK ||
		versioned.Header().Get("Cache-Control") != "private, max-age=3600, immutable" ||
		versioned.Header().Get("ETag") != etag || !bytes.Equal(versioned.Body.Bytes(), payload) {
		t.Fatalf("versioned desktop image = %d %#v %d bytes", versioned.Code, versioned.Header(), versioned.Body.Len())
	}

	pairedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/entry-1/resources/image-0", nil)
	pairedRequest.SetPathValue("id", "entry-1")
	pairedRequest.SetPathValue("slot", "image-0")
	pairedRequest.Header.Set("If-None-Match", etag)
	paired := httptest.NewRecorder()
	api.getEntryResource(paired, pairedRequest)
	if paired.Code != http.StatusOK || paired.Header().Get("Cache-Control") != "private, no-store" ||
		paired.Header().Get("ETag") != "" || !bytes.Equal(paired.Body.Bytes(), payload) {
		t.Fatalf("paired cached image = %d %#v %d bytes", paired.Code, paired.Header(), paired.Body.Len())
	}
	if upstreamCalls.Load() != 1 || service.resolves.Load() != 4 {
		t.Fatalf("calls: upstream=%d resolves=%d", upstreamCalls.Load(), service.resolves.Load())
	}
}

func TestRSSDesktopVersionedImageCachePolicyIsStrictAndDiscoveryStaysRevalidated(t *testing.T) {
	payload := rssCacheTestPNG(t, color.RGBA{R: 7, G: 14, B: 21, A: 255})
	image := rssCachedImage{data: payload, contentType: "image/png", etag: rssImageContentETag(payload)}
	for _, test := range []struct {
		name       string
		target     string
		eligible   bool
		surface    rssResourceSurface
		wantPolicy string
	}{
		{name: "versioned entry", target: "/api/rss/entries/e/resources/image-0?v=1", eligible: true, surface: rssResourceSurfaceDesktop, wantPolicy: "private, max-age=3600, immutable"},
		{name: "maximum revision", target: "/api/rss/entries/e/resources/image-0?v=9999999999999999999", eligible: true, surface: rssResourceSurfaceDesktop, wantPolicy: "private, max-age=3600, immutable"},
		{name: "unversioned", target: "/api/rss/entries/e/resources/image-0", eligible: true, surface: rssResourceSurfaceDesktop, wantPolicy: "private, max-age=0, must-revalidate"},
		{name: "leading zero", target: "/api/rss/entries/e/resources/image-0?v=01", eligible: true, surface: rssResourceSurfaceDesktop, wantPolicy: "private, max-age=0, must-revalidate"},
		{name: "extra parameter", target: "/api/rss/entries/e/resources/image-0?v=1&x=2", eligible: true, surface: rssResourceSurfaceDesktop, wantPolicy: "private, max-age=0, must-revalidate"},
		{name: "encoded key", target: "/api/rss/entries/e/resources/image-0?%76=1", eligible: true, surface: rssResourceSurfaceDesktop, wantPolicy: "private, max-age=0, must-revalidate"},
		{name: "discovery", target: "/api/rss/discovery/routes/example/icon?v=1", eligible: false, surface: rssResourceSurfaceDesktop, wantPolicy: "private, max-age=0, must-revalidate"},
		{name: "paired", target: "/api/v1/rss/entries/e/resources/image-0?v=1", eligible: true, surface: rssResourceSurfacePaired, wantPolicy: "private, no-store"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			recorder := httptest.NewRecorder()
			writeRSSImage(recorder, request, image, test.surface, test.eligible)
			if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != test.wantPolicy {
				t.Fatalf("response = %d %#v", recorder.Code, recorder.Header())
			}
		})
	}
}

func TestRSSImageCacheCoalescesConcurrentIdenticalResources(t *testing.T) {
	payload := rssCacheTestPNG(t, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	service := &rssMutableImageService{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://cdn.example/shared.png", Kind: applicationrss.RemoteResourceImage,
			Role: applicationrss.RemoteResourceRoleContentImage,
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var upstreamCalls atomic.Int32
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if upstreamCalls.Add(1) == 1 {
			close(started)
		}
		<-release
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
			Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: request,
		}, nil
	})}
	handler := api.LocalResourceHandler()
	const requestCount = 12
	results := make(chan int, requestCount)
	var wait sync.WaitGroup
	for range requestCount {
		wait.Add(1)
		go func() {
			defer wait.Done()
			request := httptest.NewRequest(http.MethodGet, "/api/rss/entries/entry-1/resources/image-0", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			results <- recorder.Code
		}()
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("upstream image request did not start")
	}
	// Give the remaining requests time to resolve their current entity and join
	// the same in-flight cache fill before the single upstream response lands.
	deadline := time.Now().Add(time.Second)
	for service.resolves.Load() < requestCount && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(release)
	wait.Wait()
	close(results)
	for status := range results {
		if status != http.StatusOK {
			t.Fatalf("coalesced image status = %d", status)
		}
	}
	if upstreamCalls.Load() != 1 || service.resolves.Load() != requestCount {
		t.Fatalf("calls: upstream=%d resolves=%d", upstreamCalls.Load(), service.resolves.Load())
	}
}

func TestRSSImageCacheKeyTracksResolvedSlotSourceAndNeverForwardsClientValidator(t *testing.T) {
	firstPayload := rssCacheTestPNG(t, color.RGBA{R: 220, A: 255})
	secondPayload := rssCacheTestPNG(t, color.RGBA{B: 220, A: 255})
	service := &rssMutableImageService{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://source-a.example/cover.png", Kind: applicationrss.RemoteResourceImage,
			Role: applicationrss.RemoteResourceRoleThumbnail,
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	var upstreamCalls atomic.Int32
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		upstreamCalls.Add(1)
		if request.Header.Get("If-None-Match") != "" || request.Header.Get("If-Modified-Since") != "" {
			t.Fatalf("slot client validator reached %s", request.URL.Host)
		}
		payload := firstPayload
		if request.URL.Host == "source-b.example" {
			payload = secondPayload
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
			Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: request,
		}, nil
	})}
	handler := api.LocalResourceHandler()
	request := httptest.NewRequest(http.MethodGet, "/api/rss/entries/entry-1/resources/thumbnail", nil)
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request)
	oldETag := first.Header().Get("ETag")

	service.setResource(applicationrss.RemoteResource{
		URL: "https://source-b.example/cover.png", Kind: applicationrss.RemoteResourceImage,
		Role: applicationrss.RemoteResourceRoleThumbnail,
	})
	request = httptest.NewRequest(http.MethodGet, "/api/rss/entries/entry-1/resources/thumbnail", nil)
	request.Header.Set("If-None-Match", oldETag)
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request)
	if second.Code != http.StatusOK || bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) ||
		second.Header().Get("ETag") == oldETag {
		t.Fatalf("changed slot source = %d first=%#v second=%#v", second.Code, first.Header(), second.Header())
	}
	if upstreamCalls.Load() != 2 || service.resolves.Load() != 2 {
		t.Fatalf("calls: upstream=%d resolves=%d", upstreamCalls.Load(), service.resolves.Load())
	}
}

func TestRSSImageNegativeCacheBoundsRepeatedFailuresAndKeepsErrorsNoStore(t *testing.T) {
	service := &rssMutableImageService{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://cdn.example/missing.png", Kind: applicationrss.RemoteResourceImage,
			Role: applicationrss.RemoteResourceRoleContentImage,
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	api.imageCache = newRSSImageCacheWithConfig(rssImageCacheConfig{now: func() time.Time { return now }})
	var upstreamCalls atomic.Int32
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		upstreamCalls.Add(1)
		return &http.Response{
			StatusCode: http.StatusNotFound, Header: make(http.Header), Body: http.NoBody, Request: request,
		}, nil
	})}
	handler := api.LocalResourceHandler()
	requestImage := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/rss/entries/entry-1/resources/image-0", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}
	for index := 0; index < 2; index++ {
		response := requestImage()
		if response.Code != http.StatusBadGateway || response.Header().Get("Cache-Control") != "private, no-store" ||
			response.Header().Get("ETag") != "" {
			t.Fatalf("negative response %d = %d %#v", index, response.Code, response.Header())
		}
	}
	if upstreamCalls.Load() != 1 || service.resolves.Load() != 2 {
		t.Fatalf("negative cache calls: upstream=%d resolves=%d", upstreamCalls.Load(), service.resolves.Load())
	}
	now = now.Add(defaultRSSImageNegativeCacheTTL + time.Second)
	if response := requestImage(); response.Code != http.StatusBadGateway {
		t.Fatalf("expired negative cache response = %d", response.Code)
	}
	if upstreamCalls.Load() != 2 || service.resolves.Load() != 3 {
		t.Fatalf("expired negative cache calls: upstream=%d resolves=%d", upstreamCalls.Load(), service.resolves.Load())
	}
}

func TestRSSImageCacheServesPreviouslyValidatedStaleBytesOnRefreshFailure(t *testing.T) {
	payload := rssCacheTestPNG(t, color.RGBA{R: 10, G: 120, B: 30, A: 255})
	service := &rssMutableImageService{
		rssServiceStub: &rssServiceStub{},
		resource: applicationrss.RemoteResource{
			URL: "https://cdn.example/stale.png", Kind: applicationrss.RemoteResourceImage,
			Role: applicationrss.RemoteResourceRoleContentImage,
		},
	}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	api.imageCache = newRSSImageCacheWithConfig(rssImageCacheConfig{
		imageTTL: time.Minute, negativeTTL: 5 * time.Minute, staleRetention: time.Hour,
		now: func() time.Time { return now },
	})
	var upstreamCalls atomic.Int32
	api.resourceClient = &http.Client{Transport: rssResourceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if upstreamCalls.Add(1) > 1 {
			return &http.Response{
				StatusCode: http.StatusBadGateway, Header: make(http.Header), Body: http.NoBody, Request: request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
			Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: request,
		}, nil
	})}
	handler := api.LocalResourceHandler()
	requestImage := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/rss/entries/entry-1/resources/image-0", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}
	if first := requestImage(); first.Code != http.StatusOK || !bytes.Equal(first.Body.Bytes(), payload) {
		t.Fatalf("initial image = %d %d bytes", first.Code, first.Body.Len())
	}
	now = now.Add(time.Minute + time.Second)
	for index := 0; index < 2; index++ {
		response := requestImage()
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), payload) {
			t.Fatalf("stale image %d = %d %d bytes", index, response.Code, response.Body.Len())
		}
	}
	if upstreamCalls.Load() != 2 {
		t.Fatalf("stale retry gate upstream calls = %d", upstreamCalls.Load())
	}
}

func rssCacheTestPNG(t *testing.T, value color.RGBA) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			canvas.SetRGBA(x, y, value)
		}
	}
	return pngBytes(t, canvas)
}
