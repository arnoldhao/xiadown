package libraryapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gabriel-vasile/mimetype"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	applicationrss "xiadown/internal/application/rss"
	"xiadown/internal/domain/rss"
)

const (
	defaultMaxConcurrentRSSResourceStreams             = 16
	defaultReservedDesktopRSSResourceStreams           = 4
	defaultMaxConcurrentPairedRSSResourceStreams       = defaultMaxConcurrentRSSResourceStreams - defaultReservedDesktopRSSResourceStreams
	maxRSSRemoteImageBytes                             = 12 << 20
	maxRSSRemoteMediaBytes                             = int64(8) << 30
	rssRemoteSniffBytes                                = 512
	maxRSSRemoteImageDimension                         = 32768
	maxRSSRemoteImagePixels                      int64 = 16_000_000
	maxRSSRemoteIconPixels                       int64 = 1_000_000
	maxRSSRemoteThumbnailPixels                  int64 = 4_000_000
	maxRSSRemoteAnimatedImageFrames                    = 256
	maxRSSRemoteAnimatedTotalPixels              int64 = 64_000_000
	maxRSSRemoteICOImages                              = 64
	defaultRSSRemoteImageTotalTimeout                  = 30 * time.Second
	defaultRSSRemoteMediaReadIdleTimeout               = 30 * time.Second
	defaultRSSRemoteMediaTotalTimeout                  = 2 * time.Hour
	rssResourceStreamCompleteTrailer                   = "X-XiaDown-RSS-Stream-Complete"
)

type rssResourceTimeoutPolicy struct {
	imageTotal    time.Duration
	mediaReadIdle time.Duration
	mediaTotal    time.Duration
}

var defaultRSSResourceTimeoutPolicy = rssResourceTimeoutPolicy{
	imageTotal:    defaultRSSRemoteImageTotalTimeout,
	mediaReadIdle: defaultRSSRemoteMediaReadIdleTimeout,
	mediaTotal:    defaultRSSRemoteMediaTotalTimeout,
}

var singleByteRangePattern = regexp.MustCompile(`^bytes=(?:[0-9]+-[0-9]*|-[0-9]+)$`)
var contentByteRangePattern = regexp.MustCompile(`^bytes ([0-9]+)-([0-9]+)/([0-9]+)$`)
var rssDesktopResourceRevisionPattern = regexp.MustCompile(`^v=[1-9][0-9]{0,18}$`)

// RSSResourceService resolves only resources already attached to persisted RSS
// entities. Public clients cannot turn rss.read into an arbitrary URL relay,
// and signed source URLs never appear in XiaDown endpoint paths or access logs.
type RSSResourceService interface {
	ResolveSubscriptionResource(context.Context, string) (applicationrss.RemoteResource, error)
	ResolveEntryResource(context.Context, string, string) (applicationrss.RemoteResource, error)
}

// RSSPairedResourceService applies the public sync eligibility boundary before
// resolving opaque slots. The loopback desktop surface intentionally uses the
// broader RSSResourceService so Inbox/notification sources keep working.
type RSSPairedResourceService interface {
	ResolveSyncSubscriptionResource(context.Context, string) (applicationrss.RemoteResource, error)
	ResolveSyncEntryResource(context.Context, string, string) (applicationrss.RemoteResource, error)
}

// RSSDiscoveryResourceService is desktop-only. Discovery is not part of the
// paired-device sync API, but its icons use the same restricted proxy and
// response validation as persisted subscription and entry resources.
type RSSDiscoveryResourceService interface {
	ResolveDiscoveryResource(
		context.Context,
		applicationrss.DiscoveryResourceKind,
		string,
	) (applicationrss.RemoteResource, error)
}

type rssResourceSurface uint8

const (
	rssResourceSurfacePaired rssResourceSurface = iota
	rssResourceSurfaceDesktop
)

func (api *RSSAPI) getSubscriptionResource(w http.ResponseWriter, request *http.Request) {
	api.serveSubscriptionResource(w, request, strings.TrimSpace(request.PathValue("id")), rssResourceSurfacePaired)
}

func (api *RSSAPI) getEntryResource(w http.ResponseWriter, request *http.Request) {
	api.serveEntryResource(
		w,
		request,
		strings.TrimSpace(request.PathValue("id")),
		strings.TrimSpace(request.PathValue("slot")),
		rssResourceSurfacePaired,
	)
}

// LocalResourceHandler is registered only on the tokenized loopback realtime
// server. It shares the exact resolver, network policy, MIME and size boundary
// used by the authenticated public API.
func (api *RSSAPI) LocalResourceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		// Errors must never become a browser-cached broken-image response. A
		// successfully resolved and validated image overwrites this conservative
		// default with its version-aware Desktop cache policy.
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
		switch {
		case len(parts) == 5 && parts[0] == "api" && parts[1] == "rss" &&
			parts[2] == "subscriptions" && parts[4] == "icon":
			api.serveSubscriptionResource(w, request, parts[3], rssResourceSurfaceDesktop)
		case len(parts) == 6 && parts[0] == "api" && parts[1] == "rss" &&
			parts[2] == "entries" && parts[4] == "resources":
			api.serveEntryResource(w, request, parts[3], parts[5], rssResourceSurfaceDesktop)
		case len(parts) == 6 && parts[0] == "api" && parts[1] == "rss" &&
			parts[2] == "discovery" && parts[5] == "icon":
			api.serveDiscoveryResource(w, request, parts[3], parts[4])
		default:
			http.NotFound(w, request)
		}
	})
}

// LoadDesktopEntryImage resolves an opaque persisted entry slot and returns
// the same validated bytes used by the tokenized Desktop resource handler.
// Callers never provide an upstream URL: resolution, SSRF policy, MIME and
// decoded-image limits, and the shared memory/disk cache all remain owned by
// this API.
func (api *RSSAPI) LoadDesktopEntryImage(
	ctx context.Context,
	id string,
	slot string,
) ([]byte, string, error) {
	id = strings.TrimSpace(id)
	slot = strings.TrimSpace(slot)
	if api == nil || api.resourceService == nil {
		return nil, "", errors.New("RSS image loader unavailable")
	}
	if !validPublicRSSID(id) || !validRSSResourceSlot(slot) {
		return nil, "", rss.ErrInvalidRequest
	}
	resource, err := api.resourceService.ResolveEntryResource(ctx, id, slot)
	if err != nil {
		return nil, "", err
	}
	if resource.Kind != applicationrss.RemoteResourceImage {
		return nil, "", rss.ErrNotFound
	}
	image, err := api.loadRSSImage(ctx, resource, rssResourceSurfaceDesktop)
	if err != nil {
		return nil, "", err
	}
	// Cache entries are shared across requests. Return an owned copy so a
	// downstream file writer can never mutate cached renderer bytes.
	return bytes.Clone(image.data), image.contentType, nil
}

func (api *RSSAPI) serveDiscoveryResource(
	w http.ResponseWriter,
	request *http.Request,
	kind, id string,
) {
	discoveryKind := applicationrss.DiscoveryResourceKind(strings.TrimSpace(kind))
	if request.Method != http.MethodGet || !validPublicRSSID(id) || api.discoveryResources == nil ||
		(discoveryKind != applicationrss.DiscoveryResourceCategoryIcon &&
			discoveryKind != applicationrss.DiscoveryResourceRouteIcon) {
		if request.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(w, request)
		return
	}
	resource, err := api.discoveryResources.ResolveDiscoveryResource(request.Context(), discoveryKind, strings.TrimSpace(id))
	if err != nil {
		writeRSSResourceResolveError(w, err)
		return
	}
	api.proxyRSSResource(w, request, resource, rssResourceSurfaceDesktop, false)
}

func (api *RSSAPI) serveSubscriptionResource(
	w http.ResponseWriter,
	request *http.Request,
	id string,
	surface rssResourceSurface,
) {
	if request.Method != http.MethodGet || !validPublicRSSID(id) || api.resourceService == nil {
		if request.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(w, request)
		return
	}
	var resource applicationrss.RemoteResource
	var err error
	if surface == rssResourceSurfacePaired {
		paired, ok := api.resourceService.(RSSPairedResourceService)
		if !ok {
			http.NotFound(w, request)
			return
		}
		resource, err = paired.ResolveSyncSubscriptionResource(request.Context(), id)
	} else {
		resource, err = api.resourceService.ResolveSubscriptionResource(request.Context(), id)
	}
	if err != nil {
		writeRSSResourceResolveError(w, err)
		return
	}
	api.proxyRSSResource(w, request, resource, surface, true)
}

func (api *RSSAPI) serveEntryResource(
	w http.ResponseWriter,
	request *http.Request,
	id, slot string,
	surface rssResourceSurface,
) {
	if request.Method != http.MethodGet || !validPublicRSSID(id) || !validRSSResourceSlot(slot) || api.resourceService == nil {
		if request.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(w, request)
		return
	}
	var resource applicationrss.RemoteResource
	var err error
	if surface == rssResourceSurfacePaired {
		paired, ok := api.resourceService.(RSSPairedResourceService)
		if !ok {
			http.NotFound(w, request)
			return
		}
		resource, err = paired.ResolveSyncEntryResource(request.Context(), id, slot)
	} else {
		resource, err = api.resourceService.ResolveEntryResource(request.Context(), id, slot)
	}
	if err != nil {
		writeRSSResourceResolveError(w, err)
		return
	}
	api.proxyRSSResource(w, request, resource, surface, true)
}

func validRSSResourceSlot(slot string) bool {
	if slot == "thumbnail" {
		return true
	}
	if len(slot) == 0 || len(slot) > 64 {
		return false
	}
	number := ""
	switch {
	case strings.HasPrefix(slot, "image-"):
		number = strings.TrimPrefix(slot, "image-")
	case strings.HasPrefix(slot, "media-"):
		number = strings.TrimPrefix(slot, "media-")
		number = strings.TrimSuffix(number, "-thumbnail")
	default:
		return false
	}
	if number == "" || (len(number) > 1 && number[0] == '0') {
		return false
	}
	for index := 0; index < len(number); index++ {
		if number[index] < '0' || number[index] > '9' {
			return false
		}
	}
	index, err := strconv.Atoi(number)
	return err == nil && index >= 0 && index < 64
}

func writeRSSResourceResolveError(w http.ResponseWriter, err error) {
	if errors.Is(err, rss.ErrNotFound) {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}
	http.Error(w, "resource unavailable", http.StatusBadGateway)
}

func (api *RSSAPI) proxyRSSResource(
	w http.ResponseWriter,
	request *http.Request,
	resource applicationrss.RemoteResource,
	surface rssResourceSurface,
	versionedDesktopCacheEligible bool,
) {
	setRSSResourceResponsePolicy(w.Header(), resource.Kind)
	writer := &idleWriteResponseWriter{
		ResponseWriter: w,
		controller:     http.NewResponseController(w),
		timeout:        defaultAssetWriteIdleTimeout,
	}
	writer.refreshDeadline()
	defer writer.clearDeadline()
	w = writer

	if resource.Kind == applicationrss.RemoteResourceImage {
		image, err := api.loadRSSImage(request.Context(), resource, surface)
		if err != nil {
			writeRSSImageLoadError(w, err)
			return
		}
		writeRSSImage(w, request, image, surface, versionedDesktopCacheEligible)
		return
	}
	if resource.Kind != applicationrss.RemoteResourceMedia {
		http.NotFound(w, request)
		return
	}

	resourceClient := api.currentRSSResourceClient()
	if resourceClient == nil {
		http.Error(w, "resource proxy unavailable", http.StatusServiceUnavailable)
		return
	}
	timeouts := api.resourceTimeouts.normalized()
	releaseSlot, acquired := api.acquireRSSResourceSlot(surface)
	if !acquired {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "resource stream limit reached", http.StatusTooManyRequests)
		return
	}
	defer releaseSlot()

	resourceCtx, cancelResource := context.WithTimeout(request.Context(), timeouts.mediaTotal)
	defer cancelResource()
	upstream, err := http.NewRequestWithContext(resourceCtx, http.MethodGet, resource.URL, nil)
	if err != nil {
		http.Error(w, "resource unavailable", http.StatusBadGateway)
		return
	}
	upstream.Header.Set("User-Agent", "XiaDown RSS Resource/1.0")
	upstream.Header.Set("Accept-Encoding", "identity")
	if refererOrigin := resource.SafeRefererOrigin(); refererOrigin != "" {
		upstream.Header.Set("Referer", refererOrigin)
	}
	upstream.Header.Set("Accept", "video/*,audio/*;q=0.9,application/octet-stream;q=0.2")
	if rawRange := strings.TrimSpace(request.Header.Get("Range")); rawRange != "" {
		if len(rawRange) > 128 || !singleByteRangePattern.MatchString(rawRange) {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		upstream.Header.Set("Range", rawRange)
	}

	response, err := resourceClient.Do(upstream)
	if err != nil {
		http.Error(w, "resource unavailable", http.StatusBadGateway)
		return
	}
	response.Body = newRSSUpstreamReadCloser(response.Body, resourceCtx, timeouts.mediaReadIdle)
	defer response.Body.Close()
	if response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
		setRSSResourceResponsePolicy(w.Header(), resource.Kind)
		if value := validRSSUnsatisfiedContentRange(response.Header.Get("Content-Range")); value != "" {
			w.Header().Set("Content-Range", value)
		}
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if response.StatusCode == http.StatusNotModified {
		// This endpoint never sends conditional validators, so a 304 can only
		// be an invalid or malicious upstream response.
		http.Error(w, "resource unavailable", http.StatusBadGateway)
		return
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
		http.Error(w, "resource unavailable", http.StatusBadGateway)
		return
	}
	api.proxyRSSMedia(w, request, response, resource)
}

func (api *RSSAPI) loadRSSImage(
	ctx context.Context,
	resource applicationrss.RemoteResource,
	surface rssResourceSurface,
) (rssCachedImage, error) {
	loader := func(loadCtx context.Context) (rssCachedImage, error) {
		return api.fetchRSSImage(loadCtx, resource, surface)
	}
	if api.imageCache == nil {
		return loader(ctx)
	}
	return api.imageCache.get(ctx, resource, loader)
}

type rssImageLoadError struct {
	status    int
	cacheable bool
	cause     error
}

func (err *rssImageLoadError) Error() string {
	switch err.status {
	case http.StatusTooManyRequests:
		return "RSS resource stream limit reached"
	case http.StatusServiceUnavailable:
		return "RSS resource proxy unavailable"
	default:
		return "RSS image unavailable"
	}
}

func (err *rssImageLoadError) Unwrap() error { return err.cause }

func (err *rssImageLoadError) rssImageNegativeCacheable() bool { return err.cacheable }

func (api *RSSAPI) fetchRSSImage(
	ctx context.Context,
	resource applicationrss.RemoteResource,
	surface rssResourceSurface,
) (rssCachedImage, error) {
	resourceClient := api.currentRSSResourceClient()
	if resourceClient == nil {
		return rssCachedImage{}, &rssImageLoadError{status: http.StatusServiceUnavailable}
	}
	releaseSlot, acquired := api.acquireRSSResourceSlot(surface)
	if !acquired {
		return rssCachedImage{}, &rssImageLoadError{status: http.StatusTooManyRequests}
	}
	defer releaseSlot()

	resourceCtx, cancelResource := context.WithTimeout(ctx, api.resourceTimeouts.normalized().imageTotal)
	defer cancelResource()
	upstream, err := http.NewRequestWithContext(resourceCtx, http.MethodGet, resource.URL, nil)
	if err != nil {
		return rssCachedImage{}, &rssImageLoadError{status: http.StatusBadGateway, cause: err}
	}
	upstream.Header.Set("User-Agent", "XiaDown RSS Resource/1.0")
	upstream.Header.Set("Accept-Encoding", "identity")
	// AVIF is intentionally omitted: Go has no configured, reliable AVIF
	// DecodeConfig implementation for enforcing dimensions before a WebView
	// sees the payload. Explicit AVIF resources therefore fail closed below.
	upstream.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/gif,image/bmp,image/x-icon;q=0.8")
	if refererOrigin := resource.SafeRefererOrigin(); refererOrigin != "" {
		upstream.Header.Set("Referer", refererOrigin)
	}

	response, err := resourceClient.Do(upstream)
	if err != nil {
		return rssCachedImage{}, &rssImageLoadError{status: http.StatusBadGateway, cacheable: true, cause: err}
	}
	response.Body = newRSSUpstreamReadCloser(response.Body, resourceCtx, 0)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
		return rssCachedImage{}, &rssImageLoadError{status: http.StatusBadGateway, cacheable: true}
	}
	image, err := readValidatedRSSImage(response, resource.Role)
	if err != nil {
		return rssCachedImage{}, &rssImageLoadError{status: http.StatusBadGateway, cacheable: true, cause: err}
	}
	return image, nil
}

func writeRSSImageLoadError(w http.ResponseWriter, err error) {
	setRSSResourceResponsePolicy(w.Header(), applicationrss.RemoteResourceImage)
	w.Header().Del("ETag")
	w.Header().Del("Last-Modified")
	status := http.StatusBadGateway
	message := "image unavailable"
	var loadErr *rssImageLoadError
	if errors.As(err, &loadErr) {
		status = loadErr.status
		switch status {
		case http.StatusTooManyRequests:
			w.Header().Set("Retry-After", "1")
			message = "resource stream limit reached"
		case http.StatusServiceUnavailable:
			message = "resource proxy unavailable"
		}
	}
	http.Error(w, message, status)
}

// acquireRSSResourceSlot preserves four slots for the tokenized Desktop
// loopback surface without increasing the process-wide RSS stream budget.
// The surface is injected by the handler registration point; it is never
// inferred from attacker-controlled Host, forwarding, or authorization headers.
func (api *RSSAPI) acquireRSSResourceSlot(surface rssResourceSurface) (func(), bool) {
	if api == nil || api.resourceSlots == nil {
		return nil, false
	}
	pairedAcquired := false
	if surface == rssResourceSurfacePaired {
		if api.pairedResourceSlots == nil {
			return nil, false
		}
		select {
		case api.pairedResourceSlots <- struct{}{}:
			pairedAcquired = true
		default:
			return nil, false
		}
	} else if surface != rssResourceSurfaceDesktop {
		return nil, false
	}
	select {
	case api.resourceSlots <- struct{}{}:
		return func() {
			<-api.resourceSlots
			if pairedAcquired {
				<-api.pairedResourceSlots
			}
		}, true
	default:
		if pairedAcquired {
			<-api.pairedResourceSlots
		}
		return nil, false
	}
}

func (api *RSSAPI) currentRSSResourceClient() *http.Client {
	if api == nil {
		return nil
	}
	if api.resourceClient != nil {
		return api.resourceClient
	}
	if api.resourceProvider != nil {
		return applicationrss.NewRemoteResourceHTTPClient(api.resourceProvider)
	}
	return nil
}

func setRSSResourceResponsePolicy(header http.Header, _ applicationrss.RemoteResourceKind) {
	header.Set("Cache-Control", "private, no-store")
	header.Set("X-Content-Type-Options", "nosniff")
}

func (policy rssResourceTimeoutPolicy) normalized() rssResourceTimeoutPolicy {
	if policy.imageTotal <= 0 {
		policy.imageTotal = defaultRSSRemoteImageTotalTimeout
	}
	if policy.mediaReadIdle <= 0 {
		policy.mediaReadIdle = defaultRSSRemoteMediaReadIdleTimeout
	}
	if policy.mediaTotal <= 0 {
		policy.mediaTotal = defaultRSSRemoteMediaTotalTimeout
	}
	return policy
}

var errRSSUpstreamReadIdle = errors.New("RSS upstream read idle timeout")

// rssUpstreamReadCloser bounds body phases independently of the downstream
// write deadline. The absolute request context and a per-Read idle timer both
// actively close the upstream body, which unblocks net/http reads and releases
// the shared RSS stream slot even when an origin stops sending bytes.
type rssUpstreamReadCloser struct {
	body        io.ReadCloser
	ctx         context.Context
	idleTimeout time.Duration
	stopContext func() bool

	closeOnce sync.Once
	closeErr  error
	failureMu sync.Mutex
	failure   error
}

func newRSSUpstreamReadCloser(body io.ReadCloser, ctx context.Context, idleTimeout time.Duration) io.ReadCloser {
	reader := &rssUpstreamReadCloser{body: body, ctx: ctx, idleTimeout: idleTimeout}
	reader.stopContext = context.AfterFunc(ctx, func() {
		reader.abort(ctx.Err())
	})
	return reader
}

func (reader *rssUpstreamReadCloser) Read(buffer []byte) (int, error) {
	if reader == nil || reader.body == nil {
		return 0, io.ErrClosedPipe
	}
	var timer *time.Timer
	var timerDone chan struct{}
	if reader.idleTimeout > 0 {
		timerDone = make(chan struct{})
		timer = time.AfterFunc(reader.idleTimeout, func() {
			reader.abort(errRSSUpstreamReadIdle)
			close(timerDone)
		})
	}
	n, err := reader.body.Read(buffer)
	if timer != nil && !timer.Stop() {
		<-timerDone
	}
	if failure := reader.readFailure(); failure != nil {
		return n, failure
	}
	if err != nil && reader.ctx != nil && reader.ctx.Err() != nil {
		return n, reader.ctx.Err()
	}
	return n, err
}

func (reader *rssUpstreamReadCloser) Close() error {
	if reader == nil {
		return nil
	}
	if reader.stopContext != nil {
		reader.stopContext()
	}
	return reader.closeBody()
}

func (reader *rssUpstreamReadCloser) abort(err error) {
	if err == nil {
		return
	}
	reader.failureMu.Lock()
	if reader.failure == nil {
		reader.failure = err
	}
	reader.failureMu.Unlock()
	_ = reader.closeBody()
}

func (reader *rssUpstreamReadCloser) readFailure() error {
	reader.failureMu.Lock()
	defer reader.failureMu.Unlock()
	return reader.failure
}

func (reader *rssUpstreamReadCloser) closeBody() error {
	reader.closeOnce.Do(func() {
		if reader.body != nil {
			reader.closeErr = reader.body.Close()
		}
	})
	return reader.closeErr
}

func (api *RSSAPI) proxyRSSImage(w http.ResponseWriter, response *http.Response, roles ...applicationrss.RemoteResourceRole) {
	role := applicationrss.RemoteResourceRoleContentImage
	if len(roles) > 0 && roles[0] != "" {
		role = roles[0]
	}
	image, err := readValidatedRSSImage(response, role)
	if err != nil {
		http.Error(w, "image unavailable", http.StatusBadGateway)
		return
	}
	writeRSSImage(w, nil, image, rssResourceSurfacePaired, false)
}

func readValidatedRSSImage(
	response *http.Response,
	role applicationrss.RemoteResourceRole,
) (rssCachedImage, error) {
	if response == nil || response.Body == nil || response.StatusCode != http.StatusOK ||
		response.ContentLength > maxRSSRemoteImageBytes {
		return rssCachedImage{}, errors.New("invalid RSS image response")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxRSSRemoteImageBytes+1))
	if err != nil || len(body) == 0 || len(body) > maxRSSRemoteImageBytes {
		return rssCachedImage{}, errors.New("invalid RSS image body")
	}
	contentType := canonicalRasterImageMIME(body, response.Header.Get("Content-Type"))
	role = normalizedRSSImageRole(role)
	if contentType == "" || !safeRSSRasterImage(body, contentType) || !safeRSSRasterImageRole(body, contentType, role) {
		return rssCachedImage{}, errors.New("unsafe RSS image body")
	}
	return rssCachedImage{data: body, contentType: contentType, etag: rssImageContentETag(body)}, nil
}

func writeRSSImage(
	w http.ResponseWriter,
	request *http.Request,
	image rssCachedImage,
	surface rssResourceSurface,
	versionedDesktopCacheEligible bool,
) {
	setRSSResourceResponsePolicy(w.Header(), applicationrss.RemoteResourceImage)
	if surface == rssResourceSurfaceDesktop {
		// The opaque entity route can point at a different source after a feed
		// update. Keep the response browser-cacheable but require validation so
		// every request still resolves the current entity/slot before a 304.
		cacheControl := "private, max-age=0, must-revalidate"
		if versionedDesktopCacheEligible && request != nil &&
			rssDesktopResourceRevisionPattern.MatchString(request.URL.RawQuery) {
			cacheControl = "private, max-age=3600, immutable"
		}
		w.Header().Set("Cache-Control", cacheControl)
		w.Header().Set("ETag", image.etag)
		if request != nil && rssIfNoneMatch(request.Header.Get("If-None-Match"), image.etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	} else {
		w.Header().Del("ETag")
	}
	w.Header().Del("Last-Modified")
	w.Header().Set("Content-Type", image.contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(image.data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image.data)
}

func rssIfNoneMatch(value, etag string) bool {
	value = strings.TrimSpace(value)
	etag = strings.TrimSpace(etag)
	if value == "" || etag == "" || len(value) > 8192 {
		return false
	}
	normalize := func(tag string) string {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "W/") || strings.HasPrefix(tag, "w/") {
			tag = strings.TrimSpace(tag[2:])
		}
		return tag
	}
	want := normalize(etag)
	for _, candidate := range strings.Split(value, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || normalize(candidate) == want {
			return true
		}
	}
	return false
}

func safeRSSRasterImageRole(body []byte, contentType string, role applicationrss.RemoteResourceRole) bool {
	limit := maxRSSRemoteImagePixels
	switch role {
	case applicationrss.RemoteResourceRoleIcon:
		limit = maxRSSRemoteIconPixels
	case applicationrss.RemoteResourceRoleThumbnail, applicationrss.RemoteResourceRoleMediaThumbnail:
		limit = maxRSSRemoteThumbnailPixels
	}
	if limit >= maxRSSRemoteImagePixels {
		return true
	}
	contentType = normalizedMIME(contentType)
	if contentType == "image/vnd.microsoft.icon" || contentType == "image/x-icon" {
		pixels, ok := rssICOTotalPixels(body)
		return ok && pixels <= limit
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width) > limit/int64(config.Height) {
		return false
	}
	// Icons, list thumbnails and posters are rendered in large collections;
	// reject animation entirely so a feed cannot multiply their decoded work.
	switch contentType {
	case "image/gif":
		frames, ok := inspectRSSGIF(body, config.Width, config.Height)
		return ok && frames == 1
	case "image/png":
		return !rssPNGHasAnimation(body)
	default:
		return true
	}
}

func (api *RSSAPI) proxyRSSMedia(w http.ResponseWriter, request *http.Request, response *http.Response, resource applicationrss.RemoteResource) {
	if response.ContentLength > maxRSSRemoteMediaBytes {
		http.Error(w, "media unavailable", http.StatusBadGateway)
		return
	}
	copyLimit := maxRSSRemoteMediaBytes
	downstreamLength := response.ContentLength
	if response.StatusCode == http.StatusPartialContent {
		start, end, total, ok := parseContentByteRange(response.Header.Get("Content-Range"))
		if !ok || total > maxRSSRemoteMediaBytes ||
			!rssContentRangeMatchesRequest(request.Header.Get("Range"), start, end, total) ||
			(response.ContentLength >= 0 && response.ContentLength != end-start+1) {
			http.Error(w, "media unavailable", http.StatusBadGateway)
			return
		}
		copyLimit = end - start + 1
		downstreamLength = copyLimit
	} else if response.ContentLength >= 0 {
		copyLimit = response.ContentLength
	}
	reader := bufio.NewReaderSize(response.Body, rssRemoteSniffBytes)
	sniffBytes := int64(rssRemoteSniffBytes)
	if copyLimit < sniffBytes {
		sniffBytes = copyLimit
	}
	prefix, peekErr := reader.Peek(int(sniffBytes))
	if len(prefix) == 0 && peekErr != nil {
		http.Error(w, "media unavailable", http.StatusBadGateway)
		return
	}
	contentType := canonicalMediaMIME(prefix, response.Header.Get("Content-Type"), resource.MIMEType, resource.URL, request.Header.Get("Range"))
	if contentType == "" {
		http.Error(w, "media unavailable", http.StatusBadGateway)
		return
	}

	setRSSResourceResponsePolicy(w.Header(), applicationrss.RemoteResourceMedia)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if value := strings.TrimSpace(response.Header.Get("Accept-Ranges")); strings.EqualFold(value, "bytes") {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	if response.StatusCode == http.StatusPartialContent {
		if value := strings.TrimSpace(response.Header.Get("Content-Range")); value != "" {
			w.Header().Set("Content-Range", value)
		}
	}
	if downstreamLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(downstreamLength, 10))
	}
	unknownFullResponse := response.StatusCode == http.StatusOK && response.ContentLength < 0
	if unknownFullResponse {
		// A chunked response otherwise ends successfully when a handler simply
		// returns after an upstream read failure. Declare an affirmative completion
		// trailer before committing headers; failures abort the transport below.
		w.Header().Add("Trailer", rssResourceStreamCompleteTrailer)
	}
	w.WriteHeader(response.StatusCode)
	if unknownFullResponse {
		streamRSSUnknownLengthMedia(w, reader, copyLimit)
		return
	}
	written, copyErr := io.Copy(w, io.LimitReader(reader, copyLimit))
	if response.StatusCode == http.StatusPartialContent && response.ContentLength < 0 &&
		copyErr == nil && written == copyLimit {
		// A chunked 206 has no transport-enforced body length. Probe one extra
		// byte so an overlong origin is detected and its connection is closed;
		// the downstream body remains capped at the declared Content-Range.
		var extra [1]byte
		_, _ = reader.Read(extra[:])
	}
}

// streamRSSUnknownLengthMedia gives an unknown-length 200 response an
// unambiguous terminal state. A clean EOF is affirmed by the declared trailer.
// Any upstream/read/write failure or byte-limit overflow panics with
// http.ErrAbortHandler so net/http closes the HTTP/1 connection or resets the
// HTTP/2 stream instead of emitting a successful chunk terminator.
func streamRSSUnknownLengthMedia(w http.ResponseWriter, reader *bufio.Reader, copyLimit int64) {
	written, copyErr := io.Copy(w, io.LimitReader(reader, copyLimit))
	if copyErr != nil {
		panic(http.ErrAbortHandler)
	}
	if written == copyLimit {
		// LimitReader reports EOF at the cap. Probe one byte to distinguish an
		// exactly-sized clean body from an origin that exceeded the 8 GiB policy.
		_, probeErr := reader.Peek(1)
		if probeErr == nil || !errors.Is(probeErr, io.EOF) {
			panic(http.ErrAbortHandler)
		}
	}
	w.Header().Set(rssResourceStreamCompleteTrailer, "1")
}

func validRSSUnsatisfiedContentRange(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "bytes */") {
		return ""
	}
	total, err := strconv.ParseInt(strings.TrimPrefix(value, "bytes */"), 10, 64)
	if err != nil || total <= 0 || total > maxRSSRemoteMediaBytes {
		return ""
	}
	return "bytes */" + strconv.FormatInt(total, 10)
}

func canonicalRasterImageMIME(body []byte, declared string) string {
	detected := normalizedMIME(mimetype.Detect(body).String())
	if !allowedRasterImageMIME(detected) {
		return ""
	}
	declared = normalizedMIME(declared)
	if declared != "" && declared != "application/octet-stream" && !allowedRasterImageMIME(declared) {
		return ""
	}
	return detected
}

func allowedRasterImageMIME(value string) bool {
	switch value {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif",
		"image/bmp", "image/vnd.microsoft.icon", "image/x-icon":
		return true
	default:
		return false
	}
}

func safeRSSRasterImage(body []byte, contentType string) bool {
	contentType = normalizedMIME(contentType)
	if contentType == "image/avif" {
		// Fail closed until the desktop binary has a reliable AVIF
		// DecodeConfig implementation. MIME sniffing alone cannot bound decoded
		// dimensions or animation work.
		return false
	}
	if contentType == "image/vnd.microsoft.icon" || contentType == "image/x-icon" {
		return safeRSSICO(body)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || !safeRSSImageDimensions(config.Width, config.Height) {
		return false
	}
	switch contentType {
	case "image/jpeg":
		return format == "jpeg"
	case "image/png":
		return format == "png" && safeRSSPNGAnimation(body, config.Width, config.Height)
	case "image/gif":
		return format == "gif" && safeRSSGIF(body, config.Width, config.Height)
	case "image/webp":
		return format == "webp" && safeRSSWebP(body)
	case "image/bmp":
		return format == "bmp"
	default:
		return false
	}
}

func safeRSSImageDimensions(width, height int) bool {
	if width <= 0 || height <= 0 || width > maxRSSRemoteImageDimension || height > maxRSSRemoteImageDimension {
		return false
	}
	return int64(width) <= maxRSSRemoteImagePixels/int64(height)
}

func safeRSSGIF(body []byte, canvasWidth, canvasHeight int) bool {
	frames, ok := inspectRSSGIF(body, canvasWidth, canvasHeight)
	return ok && frames <= maxRSSRemoteAnimatedImageFrames &&
		int64(canvasWidth)*int64(canvasHeight)*int64(frames) <= maxRSSRemoteAnimatedTotalPixels
}

func inspectRSSGIF(body []byte, canvasWidth, canvasHeight int) (int, bool) {
	if len(body) < 13 || (string(body[:6]) != "GIF87a" && string(body[:6]) != "GIF89a") {
		return 0, false
	}
	offset := 13
	if body[10]&0x80 != 0 {
		offset += 3 * (1 << ((body[10] & 0x07) + 1))
	}
	if offset > len(body) {
		return 0, false
	}
	frames := 0
	for offset < len(body) {
		switch body[offset] {
		case 0x3b: // trailer
			return frames, frames > 0 && frames <= maxRSSRemoteAnimatedImageFrames
		case 0x21: // extension: label followed by data sub-blocks
			offset += 2
			var ok bool
			offset, ok = skipRSSGIFSubBlocks(body, offset)
			if !ok {
				return 0, false
			}
		case 0x2c: // image descriptor
			if offset+10 > len(body) {
				return 0, false
			}
			left := int(binary.LittleEndian.Uint16(body[offset+1 : offset+3]))
			top := int(binary.LittleEndian.Uint16(body[offset+3 : offset+5]))
			width := int(binary.LittleEndian.Uint16(body[offset+5 : offset+7]))
			height := int(binary.LittleEndian.Uint16(body[offset+7 : offset+9]))
			if !safeRSSImageDimensions(width, height) || left > canvasWidth-width || top > canvasHeight-height {
				return 0, false
			}
			frames++
			if frames > maxRSSRemoteAnimatedImageFrames {
				return 0, false
			}
			packed := body[offset+9]
			offset += 10
			if packed&0x80 != 0 {
				offset += 3 * (1 << ((packed & 0x07) + 1))
			}
			if offset >= len(body) { // LZW minimum code size
				return 0, false
			}
			offset++
			var ok bool
			offset, ok = skipRSSGIFSubBlocks(body, offset)
			if !ok {
				return 0, false
			}
		default:
			return 0, false
		}
	}
	return 0, false
}

func skipRSSGIFSubBlocks(body []byte, offset int) (int, bool) {
	for offset < len(body) {
		size := int(body[offset])
		offset++
		if size == 0 {
			return offset, true
		}
		if size > len(body)-offset {
			return 0, false
		}
		offset += size
	}
	return 0, false
}

func safeRSSWebP(body []byte) bool {
	if len(body) < 12 || string(body[:4]) != "RIFF" || string(body[8:12]) != "WEBP" {
		return false
	}
	declaredSize := uint64(binary.LittleEndian.Uint32(body[4:8])) + 8
	if declaredSize < 12 || declaredSize > uint64(len(body)) {
		return false
	}
	limit := int(declaredSize)
	for offset := 12; offset < limit; {
		if offset+8 > limit {
			return false
		}
		kind := string(body[offset : offset+4])
		size := uint64(binary.LittleEndian.Uint32(body[offset+4 : offset+8]))
		if kind == "ANIM" || kind == "ANMF" {
			// Animated WebP is rejected because the configured decoder exposes no
			// reliable frame-count bound.
			return false
		}
		next := uint64(offset) + 8 + size + (size & 1)
		if next > uint64(limit) || next <= uint64(offset) {
			return false
		}
		offset = int(next)
	}
	return true
}

func safeRSSPNGAnimation(body []byte, canvasWidth, canvasHeight int) bool {
	if len(body) < 8 || !bytes.Equal(body[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return false
	}
	declaredFrames := uint32(0)
	frameControls := 0
	foundEnd := false
	for offset := 8; offset < len(body); {
		if offset+12 > len(body) {
			return false
		}
		size := uint64(binary.BigEndian.Uint32(body[offset : offset+4]))
		end := uint64(offset) + 12 + size
		if end > uint64(len(body)) || end <= uint64(offset) {
			return false
		}
		kind := string(body[offset+4 : offset+8])
		data := body[offset+8 : int(end)-4]
		switch kind {
		case "acTL":
			if declaredFrames != 0 || len(data) != 8 {
				return false
			}
			declaredFrames = binary.BigEndian.Uint32(data[:4])
			if declaredFrames == 0 || declaredFrames > maxRSSRemoteAnimatedImageFrames {
				return false
			}
			if int64(canvasWidth)*int64(canvasHeight)*int64(declaredFrames) > maxRSSRemoteAnimatedTotalPixels {
				return false
			}
		case "fcTL":
			if declaredFrames == 0 || len(data) != 26 {
				return false
			}
			width := uint64(binary.BigEndian.Uint32(data[4:8]))
			height := uint64(binary.BigEndian.Uint32(data[8:12]))
			xOffset := uint64(binary.BigEndian.Uint32(data[12:16]))
			yOffset := uint64(binary.BigEndian.Uint32(data[16:20]))
			if width == 0 || height == 0 || width > uint64(canvasWidth) || height > uint64(canvasHeight) ||
				xOffset > uint64(canvasWidth)-width || yOffset > uint64(canvasHeight)-height {
				return false
			}
			frameControls++
			if frameControls > maxRSSRemoteAnimatedImageFrames {
				return false
			}
		case "IEND":
			foundEnd = true
			offset = int(end)
			if offset != len(body) {
				// Do not interpret trailing attacker-controlled chunks differently
				// from the browser decoder.
				return false
			}
			if declaredFrames == 0 {
				return frameControls == 0
			}
			return frameControls == int(declaredFrames)
		}
		offset = int(end)
	}
	return foundEnd
}

func rssPNGHasAnimation(body []byte) bool {
	if len(body) < 8 || !bytes.Equal(body[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return false
	}
	for offset := 8; offset+12 <= len(body); {
		size := uint64(binary.BigEndian.Uint32(body[offset : offset+4]))
		end := uint64(offset) + 12 + size
		if end > uint64(len(body)) || end <= uint64(offset) {
			return false
		}
		if string(body[offset+4:offset+8]) == "acTL" {
			return true
		}
		offset = int(end)
	}
	return false
}

func safeRSSICO(body []byte) bool {
	if len(body) < 6 || binary.LittleEndian.Uint16(body[:2]) != 0 || binary.LittleEndian.Uint16(body[2:4]) != 1 {
		return false
	}
	count := int(binary.LittleEndian.Uint16(body[4:6]))
	directoryEnd := 6 + count*16
	if count <= 0 || count > maxRSSRemoteICOImages || directoryEnd > len(body) {
		return false
	}
	for index := 0; index < count; index++ {
		entry := body[6+index*16 : 6+(index+1)*16]
		width, height := int(entry[0]), int(entry[1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		if !safeRSSImageDimensions(width, height) || entry[3] != 0 {
			return false
		}
		size := uint64(binary.LittleEndian.Uint32(entry[8:12]))
		offset := uint64(binary.LittleEndian.Uint32(entry[12:16]))
		end := offset + size
		if size == 0 || offset < uint64(directoryEnd) || end < offset || end > uint64(len(body)) {
			return false
		}
		if !safeRSSICOEntry(body[offset:end], width, height) {
			return false
		}
	}
	return true
}

func rssICOTotalPixels(body []byte) (int64, bool) {
	if len(body) < 6 || binary.LittleEndian.Uint16(body[:2]) != 0 || binary.LittleEndian.Uint16(body[2:4]) != 1 {
		return 0, false
	}
	count := int(binary.LittleEndian.Uint16(body[4:6]))
	if count <= 0 || count > maxRSSRemoteICOImages || 6+count*16 > len(body) {
		return 0, false
	}
	var total int64
	for index := 0; index < count; index++ {
		entry := body[6+index*16 : 6+(index+1)*16]
		width, height := int64(entry[0]), int64(entry[1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		total += width * height
		if total > maxRSSRemoteImagePixels {
			return total, true
		}
	}
	return total, true
}

func safeRSSICOEntry(body []byte, directoryWidth, directoryHeight int) bool {
	if len(body) >= 8 && bytes.Equal(body[:8], []byte("\x89PNG\r\n\x1a\n")) {
		config, format, err := image.DecodeConfig(bytes.NewReader(body))
		return err == nil && format == "png" && config.Width == directoryWidth && config.Height == directoryHeight &&
			safeRSSImageDimensions(config.Width, config.Height) && safeRSSPNGAnimation(body, config.Width, config.Height) &&
			!rssPNGHasAnimation(body)
	}
	if len(body) < 12 {
		return false
	}
	headerSize := binary.LittleEndian.Uint32(body[:4])
	var width, doubledHeight int64
	switch {
	case headerSize == 12:
		width = int64(binary.LittleEndian.Uint16(body[4:6]))
		doubledHeight = int64(binary.LittleEndian.Uint16(body[6:8]))
	case headerSize >= 40 && uint64(headerSize) <= uint64(len(body)):
		width = int64(int32(binary.LittleEndian.Uint32(body[4:8])))
		doubledHeight = int64(int32(binary.LittleEndian.Uint32(body[8:12])))
		if doubledHeight < 0 {
			doubledHeight = -doubledHeight
		}
	default:
		return false
	}
	if width <= 0 || doubledHeight <= 0 || doubledHeight%2 != 0 {
		return false
	}
	height := doubledHeight / 2
	return width == int64(directoryWidth) && height == int64(directoryHeight) &&
		safeRSSImageDimensions(int(width), int(height))
}

func canonicalMediaMIME(prefix []byte, declared, feedMIME, rawURL, rawRange string) string {
	declared = normalizedMIME(declared)
	feedMIME = normalizedMIME(feedMIME)
	detected := normalizedMIME(mimetype.Detect(prefix).String())
	if isManifestMIME(declared) || isManifestMIME(feedMIME) || isManifestURL(rawURL) {
		return ""
	}
	if rangeStartsAfterZero(rawRange) {
		if allowedMediaMIME(declared) {
			return declared
		}
		return ""
	}
	if strings.HasPrefix(detected, "text/") || detected == "application/json" || strings.HasSuffix(detected, "+xml") || detected == "application/xml" {
		return ""
	}
	if allowedMediaMIME(declared) {
		return declared
	}
	if allowedMediaMIME(detected) {
		return detected
	}
	if allowedMediaMIME(feedMIME) && (declared == "" || declared == "application/octet-stream") {
		return feedMIME
	}
	return ""
}

func allowedMediaMIME(value string) bool {
	return strings.HasPrefix(value, "video/") || strings.HasPrefix(value, "audio/") || value == "application/ogg"
}

func isManifestMIME(value string) bool {
	switch value {
	case "application/vnd.apple.mpegurl", "application/x-mpegurl", "audio/mpegurl", "audio/x-mpegurl", "application/dash+xml":
		return true
	default:
		return false
	}
}

func isManifestURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	path := strings.ToLower(parsed.Path)
	return strings.HasSuffix(path, ".m3u8") || strings.HasSuffix(path, ".mpd")
}

func rangeStartsAfterZero(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "bytes=") {
		return false
	}
	start := strings.SplitN(strings.TrimPrefix(value, "bytes="), "-", 2)[0]
	return start != "" && start != "0"
}

func normalizedMIME(value string) string {
	parsed, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed)
}

func parseContentByteRange(value string) (start, end, total int64, ok bool) {
	matches := contentByteRangePattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 4 {
		return 0, 0, 0, false
	}
	values := make([]int64, 3)
	for index := range values {
		parsed, err := strconv.ParseInt(matches[index+1], 10, 64)
		if err != nil || parsed < 0 {
			return 0, 0, 0, false
		}
		values[index] = parsed
	}
	start, end, total = values[0], values[1], values[2]
	return start, end, total, start <= end && end < total && total > 0
}

func rssContentRangeMatchesRequest(value string, start, end, total int64) bool {
	value = strings.TrimSpace(value)
	if !singleByteRangePattern.MatchString(value) || total <= 0 {
		return false
	}
	raw := strings.TrimPrefix(value, "bytes=")
	parts := strings.SplitN(raw, "-", 2)
	if len(parts) != 2 {
		return false
	}
	if parts[0] == "" {
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix <= 0 {
			return false
		}
		expectedStart := int64(0)
		if suffix < total {
			expectedStart = total - suffix
		}
		return start == expectedStart && end == total-1
	}
	requestedStart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || requestedStart < 0 || start != requestedStart || start >= total {
		return false
	}
	expectedEnd := total - 1
	if parts[1] != "" {
		requestedEnd, parseErr := strconv.ParseInt(parts[1], 10, 64)
		if parseErr != nil || requestedEnd < requestedStart {
			return false
		}
		if requestedEnd < expectedEnd {
			expectedEnd = requestedEnd
		}
	}
	return end == expectedEnd
}
