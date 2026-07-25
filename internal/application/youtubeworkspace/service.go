package youtubeworkspace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var youtubeVideoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var youtubePlaylistIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{10,100}$`)

var youtubeGuestHomeFallbackBrowseIDs = []string{
	"FEnews_destination",
	"FEsports_destination",
	"FEgaming_destination",
	"FElearning_destination",
}

const (
	youtubeGuestHomeFallbackItemCap     = 20
	youtubeGuestHomeFallbackConcurrency = 2
	youtubeGuestHomeFallbackTimeout     = 5 * time.Second

	youtubeContinuationEndpointCapacity = 1024
	youtubeContinuationEndpointTTL      = time.Hour
)

type innerTubeRequester interface {
	requestRead(
		context.Context,
		string,
		map[string]any,
		innerTubeAuthPolicy,
	) (map[string]any, error)
}

type innerTubeUserAgentSetter interface {
	setUserAgent(string)
}

type innerTubeCacheInvalidator interface {
	clearCache()
}

type Service struct {
	requester innerTubeRequester

	continuationMu        sync.Mutex
	continuationEndpoints map[string]continuationEndpointEntry
	continuationSequence  uint64
	continuationNow       func() time.Time

	guestHomeFallbackTimeout time.Duration
}

type continuationEndpointEntry struct {
	endpoint  string
	expiresAt time.Time
	sequence  uint64
}

func NewService(
	cookies innerTubeCookieProvider,
	httpClientProvider innerTubeHTTPClientProvider,
) *Service {
	return &Service{
		requester:             newInnerTubeClient(cookies, httpClientProvider),
		continuationEndpoints: make(map[string]continuationEndpointEntry),
	}
}

// SetUserAgent keeps the InnerTube WEB identity aligned with the platform App
// Session WebView. Changing it also invalidates the client's request cache.
func (service *Service) SetUserAgent(userAgent string) {
	if service == nil || service.requester == nil {
		return
	}
	if setter, ok := service.requester.(innerTubeUserAgentSetter); ok {
		setter.setUserAgent(userAgent)
	}
}

// ForceRefresh clears account-scoped InnerTube responses and continuation
// routing so subsequent browse requests use current network and cookie state.
func (service *Service) ForceRefresh() {
	if service == nil {
		return
	}
	if invalidator, ok := service.requester.(innerTubeCacheInvalidator); ok {
		invalidator.clearCache()
	}
	service.continuationMu.Lock()
	service.continuationEndpoints = make(map[string]continuationEndpointEntry)
	service.continuationSequence = 0
	service.continuationMu.Unlock()
}

type routeSpec struct {
	title         string
	webURL        string
	endpoint      string
	browseID      string
	authPolicy    innerTubeAuthPolicy
	filter        innerTubeItemFilter
	requiresQuery bool
}

var routeSpecs = map[string]routeSpec{
	"search": {
		title:         "Search",
		webURL:        "https://www.youtube.com/results",
		endpoint:      "search",
		authPolicy:    innerTubeAuthOptional,
		filter:        innerTubeItemsAll,
		requiresQuery: true,
	},
	"home": {
		title:      "Home",
		webURL:     "https://www.youtube.com/",
		endpoint:   "browse",
		browseID:   "FEwhat_to_watch",
		authPolicy: innerTubeAuthOptional,
		filter:     innerTubeItemsVideosOnly,
	},
	"subscriptions": {
		title:      "Subscriptions",
		webURL:     "https://www.youtube.com/feed/subscriptions",
		endpoint:   "browse",
		browseID:   "FEsubscriptions",
		authPolicy: innerTubeAuthRequired,
		filter:     innerTubeItemsVideosOnly,
	},
	"explore": {
		title:      "Explore",
		webURL:     "https://www.youtube.com/gaming",
		endpoint:   "browse",
		browseID:   "FEgaming_destination",
		authPolicy: innerTubeAuthOptional,
		filter:     innerTubeItemsVideosOnly,
	},
	"shorts": {
		title:      "Shorts",
		webURL:     "https://www.youtube.com/shorts/",
		endpoint:   "browse",
		browseID:   "FEwhat_to_watch",
		authPolicy: innerTubeAuthOptional,
		filter:     innerTubeItemsShortsOnly,
	},
	"liked-videos": {
		title:      "Liked videos",
		webURL:     "https://www.youtube.com/playlist?list=LL",
		endpoint:   "browse",
		browseID:   "VLLL",
		authPolicy: innerTubeAuthRequired,
		filter:     innerTubeItemsVideosOnly,
	},
	"watch-later": {
		title:      "Watch later",
		webURL:     "https://www.youtube.com/playlist?list=WL",
		endpoint:   "browse",
		browseID:   "VLWL",
		authPolicy: innerTubeAuthRequired,
		filter:     innerTubeItemsVideosOnly,
	},
	"playlists": {
		title:      "Playlists",
		webURL:     "https://www.youtube.com/feed/playlists",
		endpoint:   "browse",
		browseID:   "FEplaylist_aggregation",
		authPolicy: innerTubeAuthRequired,
		filter:     innerTubeItemsPlaylistsOnly,
	},
	"history": {
		title:      "History",
		webURL:     "https://www.youtube.com/feed/history",
		endpoint:   "browse",
		browseID:   "FEhistory",
		authPolicy: innerTubeAuthRequired,
		filter:     innerTubeItemsVideosOnly,
	},
}

type resolvedBrowseRequest struct {
	endpoint   string
	body       map[string]any
	authPolicy innerTubeAuthPolicy
	filter     innerTubeItemFilter
}

func (service *Service) Browse(ctx context.Context, request BrowseRequest) (BrowsePage, error) {
	if service == nil || service.requester == nil {
		return BrowsePage{}, errors.New("youtube workspace browse backend unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	routeID := strings.ToLower(strings.TrimSpace(request.RouteID))
	if routeID == "" {
		routeID = "home"
	}
	spec, ok := routeSpecs[routeID]
	if !ok {
		return BrowsePage{}, fmt.Errorf("unsupported youtube workspace route %q", routeID)
	}

	playlistID := strings.TrimSpace(request.PlaylistID)
	if request.PlaylistID != "" {
		if routeID != "playlists" {
			return BrowsePage{}, fmt.Errorf("youtube playlist browsing is not available for route %q", routeID)
		}
		if !youtubePlaylistIDPattern.MatchString(playlistID) {
			return BrowsePage{}, errors.New("invalid youtube playlist id")
		}
	}

	page := BrowsePage{
		RouteID: routeID,
		Title:   spec.title,
		WebURL:  routeWebURL(spec, request.Query),
		Items:   []Video{},
	}
	if playlistID != "" {
		page.WebURL = "https://www.youtube.com/playlist?list=" + url.QueryEscape(playlistID)
	}

	continuation := strings.TrimSpace(request.Continuation)
	query := strings.TrimSpace(request.Query)
	if spec.requiresQuery && query == "" && continuation == "" {
		page.EmptyReason = "search_query_required"
		return page, nil
	}

	ctx = withInnerTubeLocale(ctx, request.Locale)

	resolved := service.resolveBrowseRequest(routeID, spec, playlistID, query, continuation)
	data, err := service.requester.requestRead(
		ctx,
		resolved.endpoint,
		resolved.body,
		resolved.authPolicy,
	)
	if err != nil {
		return service.handleBrowseError(page, routeID, err)
	}
	// A YouTube continuation advances past the entire server response, not an
	// arbitrary client-side slice. Preserve every parsed item from this response
	// so the next continuation cannot skip unseen videos or playlists.
	parsed := parseInnerTubeItems(data, resolved.filter, 0)
	if routeID == "home" && continuation == "" && len(parsed.Items) == 0 {
		parsed.Items, err = service.browseGuestHomeFallback(ctx)
		if err != nil {
			return service.handleBrowseError(page, routeID, err)
		}
		parsed.Continuation = ""
	}

	// YouTube Home does not always expose Shorts. Fall back to a real InnerTube
	// search rather than manufacturing keyword recommendations.
	if routeID == "shorts" && continuation == "" && len(parsed.Items) == 0 {
		const fallbackEndpoint = "search"
		fallbackData, fallbackErr := service.requester.requestRead(
			ctx,
			fallbackEndpoint,
			map[string]any{"query": "#shorts"},
			innerTubeAuthOptional,
		)
		if fallbackErr != nil {
			return service.handleBrowseError(page, routeID, fallbackErr)
		}
		parsed = parseInnerTubeItems(fallbackData, innerTubeItemsShortsOnly, 0)
		resolved.endpoint = fallbackEndpoint
	}

	page.Items = parsed.Items
	page.Continuation = parsed.Continuation
	if parsed.Continuation != "" {
		service.rememberContinuationEndpoint(routeID, parsed.Continuation, resolved.endpoint)
	}
	if len(page.Items) == 0 {
		page.EmptyReason = "no_videos"
	}
	return page, nil
}

// browseGuestHomeFallback handles signed-out Home responses. YouTube can return
// only an empty recommendation shell for FEwhat_to_watch without an account,
// while these public destination feeds remain useful. They are merged into one
// flat XiaDown page and intentionally have no shared continuation.
func (service *Service) browseGuestHomeFallback(ctx context.Context) ([]Video, error) {
	if service == nil || service.requester == nil {
		return []Video{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	timeout := service.guestHomeFallbackTimeout
	if timeout <= 0 {
		timeout = youtubeGuestHomeFallbackTimeout
	}
	fallbackCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type fallbackResult struct {
		index int
		items []Video
	}

	results := make(chan fallbackResult, len(youtubeGuestHomeFallbackBrowseIDs))
	semaphore := make(chan struct{}, youtubeGuestHomeFallbackConcurrency)
	for index, browseID := range youtubeGuestHomeFallbackBrowseIDs {
		go func() {
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-fallbackCtx.Done():
				return
			}

			data, err := service.requester.requestRead(
				fallbackCtx,
				"browse",
				map[string]any{"browseId": browseID},
				innerTubeAuthOptional,
			)
			if err != nil {
				select {
				case results <- fallbackResult{index: index}:
				case <-fallbackCtx.Done():
				}
				return
			}
			parsed := parseInnerTubeItems(data, innerTubeItemsVideosOnly, 0)
			select {
			case results <- fallbackResult{index: index, items: parsed.Items}:
			case <-fallbackCtx.Done():
			}
		}()
	}

	resultsByBrowseID := make([][]Video, len(youtubeGuestHomeFallbackBrowseIDs))
	completed := 0
	collecting := true
	for completed < len(youtubeGuestHomeFallbackBrowseIDs) && collecting {
		select {
		case result := <-results:
			resultsByBrowseID[result.index] = result.items
			completed++
		case <-fallbackCtx.Done():
			collecting = false
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Preserve results that completed concurrently with the internal deadline.
	for {
		select {
		case result := <-results:
			resultsByBrowseID[result.index] = result.items
		default:
			goto merge
		}
	}

merge:
	items := make([]Video, 0, len(youtubeGuestHomeFallbackBrowseIDs)*youtubeGuestHomeFallbackItemCap)
	seen := make(map[string]struct{})
	for _, resultItems := range resultsByBrowseID {
		accepted := 0
		for _, item := range resultItems {
			if accepted >= youtubeGuestHomeFallbackItemCap {
				break
			}
			identity := strings.TrimSpace(item.VideoID)
			if identity == "" {
				continue
			}
			if _, duplicate := seen[identity]; duplicate {
				continue
			}
			seen[identity] = struct{}{}
			items = append(items, item)
			accepted++
		}
	}
	if items == nil {
		return []Video{}, nil
	}
	return items, nil
}

func (service *Service) resolveBrowseRequest(
	routeID string,
	spec routeSpec,
	playlistID string,
	query string,
	continuation string,
) resolvedBrowseRequest {
	endpoint := spec.endpoint
	body := make(map[string]any, 1)
	authPolicy := spec.authPolicy
	filter := spec.filter

	if continuation != "" {
		if remembered := service.continuationEndpoint(routeID, continuation); remembered != "" {
			endpoint = remembered
		}
		body["continuation"] = continuation
		if playlistID != "" {
			authPolicy = innerTubeAuthOptional
			filter = innerTubeItemsVideosOnly
		}
		return resolvedBrowseRequest{
			endpoint:   endpoint,
			body:       body,
			authPolicy: authPolicy,
			filter:     filter,
		}
	}
	if playlistID != "" {
		return resolvedBrowseRequest{
			endpoint:   "browse",
			body:       map[string]any{"browseId": "VL" + playlistID},
			authPolicy: innerTubeAuthOptional,
			filter:     innerTubeItemsVideosOnly,
		}
	}
	if spec.requiresQuery {
		body["query"] = query
	} else {
		body["browseId"] = spec.browseID
	}
	return resolvedBrowseRequest{
		endpoint:   endpoint,
		body:       body,
		authPolicy: authPolicy,
		filter:     filter,
	}
}

func (service *Service) handleBrowseError(
	page BrowsePage,
	routeID string,
	err error,
) (BrowsePage, error) {
	if errors.Is(err, errYouTubeInnerTubeNotAuthenticated) ||
		errors.Is(err, errYouTubeInnerTubeAuthExpired) {
		page.RequiresAuthentication = true
		page.EmptyReason = "youtube_sign_in_required"
		page.Items = []Video{}
		page.Continuation = ""
		return page, nil
	}
	return BrowsePage{}, fmt.Errorf("browse youtube %s: %w", routeID, err)
}

func (service *Service) rememberContinuationEndpoint(routeID string, token string, endpoint string) {
	if service == nil || strings.TrimSpace(token) == "" || strings.TrimSpace(endpoint) == "" {
		return
	}
	service.continuationMu.Lock()
	if service.continuationEndpoints == nil {
		service.continuationEndpoints = make(map[string]continuationEndpointEntry)
	}
	now := service.continuationTime()
	service.removeExpiredContinuationEndpointsLocked(now)
	key := continuationEndpointKey(routeID, token)
	if _, exists := service.continuationEndpoints[key]; !exists &&
		len(service.continuationEndpoints) >= youtubeContinuationEndpointCapacity {
		service.removeOldestContinuationEndpointLocked()
	}
	service.continuationSequence++
	service.continuationEndpoints[key] = continuationEndpointEntry{
		endpoint:  endpoint,
		expiresAt: now.Add(youtubeContinuationEndpointTTL),
		sequence:  service.continuationSequence,
	}
	service.continuationMu.Unlock()
}

func (service *Service) continuationEndpoint(routeID string, token string) string {
	if service == nil {
		return ""
	}
	service.continuationMu.Lock()
	defer service.continuationMu.Unlock()
	service.removeExpiredContinuationEndpointsLocked(service.continuationTime())
	return service.continuationEndpoints[continuationEndpointKey(routeID, token)].endpoint
}

func (service *Service) continuationTime() time.Time {
	if service.continuationNow != nil {
		return service.continuationNow()
	}
	return time.Now()
}

func (service *Service) removeExpiredContinuationEndpointsLocked(now time.Time) {
	for key, entry := range service.continuationEndpoints {
		if !now.Before(entry.expiresAt) {
			delete(service.continuationEndpoints, key)
		}
	}
}

func (service *Service) removeOldestContinuationEndpointLocked() {
	var oldestKey string
	var oldestSequence uint64
	for key, entry := range service.continuationEndpoints {
		if oldestKey == "" || entry.sequence < oldestSequence {
			oldestKey = key
			oldestSequence = entry.sequence
		}
	}
	if oldestKey != "" {
		delete(service.continuationEndpoints, oldestKey)
	}
}

func continuationEndpointKey(routeID string, token string) string {
	return strings.TrimSpace(routeID) + "\x00" + strings.TrimSpace(token)
}

func (service *Service) PreparePlayback(video Video) (PlaybackDescriptor, error) {
	videoID := strings.TrimSpace(video.VideoID)
	if !youtubeVideoIDPattern.MatchString(videoID) {
		return PlaybackDescriptor{}, errors.New("invalid youtube video id")
	}
	title := strings.TrimSpace(video.Title)
	if title == "" {
		title = videoID
	}
	return PlaybackDescriptor{
		Source:          "youtube",
		MediaKind:       "video",
		VideoID:         videoID,
		Title:           title,
		Artist:          strings.TrimSpace(video.Channel),
		ChannelID:       strings.TrimSpace(video.ChannelID),
		ThumbnailURL:    strings.TrimSpace(video.ThumbnailURL),
		DurationSeconds: max(0, video.DurationSeconds),
		ViewCount:       max(0, video.ViewCount),
		PublishedLabel:  strings.TrimSpace(video.PublishedLabel),
		WebURL:          "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID),
	}, nil
}

func routeWebURL(spec routeSpec, query string) string {
	if !spec.requiresQuery || strings.TrimSpace(query) == "" {
		return spec.webURL
	}
	return spec.webURL + "?search_query=" + url.QueryEscape(strings.TrimSpace(query))
}
