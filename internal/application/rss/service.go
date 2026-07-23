package rss

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"xiadown/internal/application/networkpolicy"
	domainrss "xiadown/internal/domain/rss"

	"github.com/google/uuid"
	"go.uber.org/zap"
	xhtml "golang.org/x/net/html"
)

const (
	maxFeedResponseBytes       = 10 << 20
	feedRequestTimeout         = 25 * time.Second
	maxFeedValidatorBytes      = 1024
	maxFeedValidatorURLBytes   = 4096
	maxFeedDiscoveryHTMLBytes  = 2 << 20
	maxFeedDiscoveryHTMLTokens = 100_000
	maxConcurrentRSSRefreshes  = 4
	markAllReadPageSize        = 500
	previewLeaseTTL            = 5 * time.Minute
	maxPreviewLeases           = 16
	maxPreviewTokenBytes       = 128
)

type HTTPClientProvider interface{ HTTPClient() *http.Client }

type Service struct {
	repository             domainrss.Repository
	syncRepository         domainrss.SyncRepository
	discoveryRepository    domainrss.DiscoveryRepository
	organizationRepository domainrss.OrganizationRepository
	clients                HTTPClientProvider
	resolver               networkpolicy.Resolver
	mirrors                []string
	now                    func() time.Time
	locks                  sync.Map
	discoveryMu            sync.Mutex
	discoveryRefreshing    atomic.Bool
	discoveryTimeout       time.Duration
	discoveryFetcher       func(context.Context) (domainrss.DiscoveryCache, error)
	previewMu              sync.Mutex
	previewLeases          map[string]previewLease
	previewLeaseBytes      int64
	pendingHydrations      *pendingHydrationQueue
}

type fetchMetadata struct {
	ResolvedURL  string
	ETag         string
	LastModified string
	ValidatorURL string
	NotModified  bool
}

func NewService(repository domainrss.Repository, clients HTTPClientProvider) *Service {
	discoveryRepository, _ := repository.(domainrss.DiscoveryRepository)
	syncRepository, _ := repository.(domainrss.SyncRepository)
	organizationRepository, _ := repository.(domainrss.OrganizationRepository)
	return &Service{
		repository: repository, syncRepository: syncRepository, discoveryRepository: discoveryRepository,
		organizationRepository: organizationRepository, clients: clients,
		resolver:          net.DefaultResolver,
		mirrors:           append([]string(nil), DefaultRSSHubMirrors...),
		now:               func() time.Time { return time.Now().UTC() },
		discoveryTimeout:  discoveryRequestTimeout,
		previewLeases:     make(map[string]previewLease),
		pendingHydrations: newPendingHydrationQueue(maxPendingRSSHydrations),
	}
}

func (service *Service) ListSubscriptions(ctx context.Context) ([]domainrss.Subscription, error) {
	return service.repository.ListSubscriptions(ctx)
}

func (service *Service) PreviewSubscription(ctx context.Context, request PreviewSubscriptionRequest) (PreviewSubscriptionResult, error) {
	canonical, err := normalizeFeedURL(request.URL)
	if err != nil {
		return PreviewSubscriptionResult{}, err
	}
	view, err := normalizeViewType(request.ViewType)
	if err != nil {
		return PreviewSubscriptionResult{}, err
	}
	feed, metadata, err := service.fetchAndParse(ctx, canonical, "", "", "")
	if err != nil {
		return PreviewSubscriptionResult{}, err
	}
	feed = service.enrichParsedFeedIcon(ctx, feed, "")
	now := service.now()
	snapshot := newPreviewFeedSnapshot(canonical, feed, now)
	subscription, entries := snapshot.materialize("rss-preview", canonical, view, now)
	if len(entries) > 6 {
		entries = entries[:6]
	}
	token := service.storePreviewLease(canonical, snapshot, metadata, now)
	return PreviewSubscriptionResult{
		Subscription: subscription, Entries: entries, ResolvedURL: metadata.ResolvedURL, PreviewToken: token,
	}, nil
}

func (service *Service) AddSubscription(ctx context.Context, request AddSubscriptionRequest) (domainrss.Subscription, error) {
	canonical, err := normalizeFeedURL(request.URL)
	if err != nil {
		return domainrss.Subscription{}, err
	}
	view, err := normalizeViewType(request.ViewType)
	if err != nil {
		return domainrss.Subscription{}, err
	}
	now := service.now()
	snapshot, metadata, previewStatus := service.claimPreviewLease(request.PreviewToken, canonical, now)
	if previewStatus == previewLeaseBusy {
		return domainrss.Subscription{}, errPreviewLeaseInUse
	}
	if previewStatus == previewLeaseMissing && request.AllowPending {
		if err := service.ensureNoSubscriptionAlias(ctx, canonical, fetchMetadata{}); err != nil {
			return domainrss.Subscription{}, err
		}
		return service.createPendingSubscription(ctx, canonical, view, request.Title, now)
	}
	id := "rss-subscription-" + uuid.NewString()
	var subscription domainrss.Subscription
	var entries []domainrss.Entry
	if previewStatus == previewLeaseAcquired {
		subscription, entries = snapshot.materialize(id, canonical, view, now)
	} else {
		feed, fetchedMetadata, fetchErr := service.fetchAndParse(ctx, canonical, "", "", "")
		metadata = fetchedMetadata
		if fetchErr != nil {
			return domainrss.Subscription{}, fetchErr
		}
		feed = service.enrichParsedFeedIcon(ctx, feed, "")
		subscription = subscriptionFromParsedFeed(id, canonical, view, feed, now)
		entries = entriesFromFeed(subscription.ID, subscription.ViewType, feed, now)
	}
	if title := strings.TrimSpace(request.Title); title != "" {
		subscription.Title = limitString(title, 512)
	}
	if err := service.ensureNoSubscriptionAlias(ctx, canonical, metadata); err != nil {
		if previewStatus == previewLeaseAcquired {
			service.releasePreviewLease(request.PreviewToken, canonical, service.now())
		}
		return domainrss.Subscription{}, err
	}
	subscription.ETag = metadata.ETag
	subscription.LastModified = metadata.LastModified
	subscription.ValidatorURL = metadata.ValidatorURL
	subscription.LastFetchedAt = &now
	subscription.LastSuccessAt = &now
	created, _, err := service.repository.CreateFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription,
		Entries:      entries,
	})
	if err != nil {
		if previewStatus == previewLeaseAcquired {
			service.releasePreviewLease(request.PreviewToken, canonical, service.now())
		}
		return domainrss.Subscription{}, err
	}
	if previewStatus == previewLeaseAcquired {
		service.consumePreviewLease(request.PreviewToken, canonical)
	}
	return created, nil
}

func (service *Service) createPendingSubscription(
	ctx context.Context,
	canonical string,
	view domainrss.ViewType,
	title string,
	now time.Time,
) (domainrss.Subscription, error) {
	item := subscriptionFromParsedFeed("rss-subscription-"+uuid.NewString(), canonical, view, parsedFeed{}, now)
	if customTitle := strings.TrimSpace(title); customTitle != "" {
		item.Title = limitString(customTitle, 512)
	}
	item.LastError = "Preview unavailable; waiting for the first successful refresh."
	created, err := service.repository.CreateSubscription(ctx, item)
	if err != nil {
		return domainrss.Subscription{}, err
	}
	// The repository commit has completed. Queueing after this point guarantees
	// the worker can observe the subscription and keeps request cancellation
	// from aborting its first hydration attempt.
	service.pendingHydrations.Enqueue(created.ID)
	return created, nil
}

func (service *Service) UpdateSubscription(ctx context.Context, request UpdateSubscriptionRequest) (domainrss.Subscription, error) {
	item, err := service.repository.GetSubscription(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return domainrss.Subscription{}, err
	}
	if title := strings.TrimSpace(request.Title); title != "" {
		item.Title = limitString(title, 512)
	}
	if strings.TrimSpace(request.ViewType) != "" {
		view, viewErr := normalizeViewType(request.ViewType)
		if viewErr != nil {
			return domainrss.Subscription{}, viewErr
		}
		item.ViewType = view
	}
	if request.Enabled != nil {
		item.Enabled = *request.Enabled
	}
	if request.CategoryID != nil {
		categoryID := strings.TrimSpace(*request.CategoryID)
		if categoryID != "" {
			if service.organizationRepository == nil {
				return domainrss.Subscription{}, domainrss.ErrInvalidRequest
			}
			if _, categoryErr := service.organizationRepository.GetCategory(ctx, categoryID); categoryErr != nil {
				return domainrss.Subscription{}, categoryErr
			}
		}
		item.CategoryID = categoryID
	}
	if request.SortOrder != nil {
		item.SortOrder = normalizeSortOrder(*request.SortOrder)
	}
	item.Revision++
	item.UpdatedAt = service.now()
	updated, err := service.repository.UpdateSubscription(ctx, item)
	if err != nil {
		return domainrss.Subscription{}, err
	}
	// UpdateSubscription persists the requested source-of-truth fields, while
	// resolvedViewType is a projection over both the subscription and its
	// existing entries. Re-read after the commit so callers never need a later
	// ListSubscriptions refresh to observe the authoritative presentation.
	return service.repository.GetSubscription(ctx, updated.ID)
}

func (service *Service) DeleteSubscription(ctx context.Context, request SubscriptionRequest) error {
	return service.repository.DeleteSubscription(ctx, strings.TrimSpace(request.ID), service.now())
}

func (service *Service) Refresh(ctx context.Context, request RefreshRequest) (RefreshResult, error) {
	if id := strings.TrimSpace(request.ID); id != "" {
		result, err := service.refreshOne(ctx, id)
		if err != nil {
			return RefreshResult{Subscriptions: 1, Failed: 1}, err
		}
		return RefreshResult{Subscriptions: 1, Created: result.Created, Updated: result.Updated}, nil
	}
	return service.refreshAll(ctx)
}

func (service *Service) ListEntries(ctx context.Context, request ListEntriesRequest) (domainrss.EntryPage, error) {
	kind, err := normalizeEntryKindFilter(request.Kind)
	if err != nil {
		return domainrss.EntryPage{}, err
	}
	sourceKind, err := normalizeSourceKindFilter(request.SourceKind)
	if err != nil {
		return domainrss.EntryPage{}, err
	}
	page, err := service.repository.ListEntries(ctx, domainrss.EntryQuery{
		SubscriptionID: strings.TrimSpace(request.SubscriptionID), CollectionID: strings.TrimSpace(request.CollectionID),
		CategoryID: strings.TrimSpace(request.CategoryID), SourceKind: sourceKind, Kind: kind,
		Query: strings.TrimSpace(request.Query), UnreadOnly: request.UnreadOnly, StarredOnly: request.StarredOnly,
		Limit: request.Limit, Offset: request.Offset,
	})
	if err != nil {
		return domainrss.EntryPage{}, err
	}
	for index := range page.Items {
		// List projections never hydrate or sanitize article bodies. Detail reads
		// are the only path that returns sanitized ContentHTML.
		page.Items[index].ContentHTML = ""
		page.Items[index].PlaybackURL = playbackURLForEntry(page.Items[index])
		page.Items[index].DownloadTarget = downloadTargetForEntry(page.Items[index])
	}
	return page, nil
}

func (service *Service) GetEntry(ctx context.Context, request SubscriptionRequest) (domainrss.Entry, error) {
	entry, err := service.repository.GetEntry(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return domainrss.Entry{}, err
	}
	entry.PlaybackURL = playbackURLForEntry(entry)
	entry.DownloadTarget = downloadTargetForEntry(entry)
	entry.ContentHTML = sanitizeEntryHTML(entry.ContentHTML, entry.URL)
	return entry, nil
}

func (service *Service) SetEntryRead(ctx context.Context, request SetEntryReadRequest) (domainrss.EntryState, error) {
	mutationID := strings.TrimSpace(request.MutationID)
	if mutationID == "" {
		mutationID = uuid.NewString()
	}
	return service.applyEntryRead(ctx, "desktop", request, mutationID)
}

// MarkAllRead marks every currently unread entry in a desktop collection as
// read. Each page is fetched again from offset zero because successful
// mutations remove rows from the unread result set; advancing an offset would
// otherwise skip an entire page. The operation intentionally uses the legacy
// desktop mutation boundary and does not alter the paired-device sync API.
func (service *Service) MarkAllRead(ctx context.Context, request MarkAllReadRequest) (MarkAllReadResult, error) {
	kind, err := normalizeEntryKindFilter(request.Kind)
	if err != nil {
		return MarkAllReadResult{}, err
	}
	sourceKind, err := normalizeSourceKindFilter(request.SourceKind)
	if err != nil {
		return MarkAllReadResult{}, err
	}
	query := domainrss.EntryQuery{
		SubscriptionID: strings.TrimSpace(request.SubscriptionID),
		CollectionID:   strings.TrimSpace(request.CollectionID),
		CategoryID:     strings.TrimSpace(request.CategoryID),
		SourceKind:     sourceKind,
		Kind:           kind,
		UnreadOnly:     true,
		StarredOnly:    request.StarredOnly,
		Limit:          markAllReadPageSize,
	}
	result := MarkAllReadResult{}
	processed := make(map[string]struct{})
	for {
		page, listErr := service.repository.ListEntries(ctx, query)
		if listErr != nil {
			return result, listErr
		}
		if len(page.Items) == 0 {
			return result, nil
		}
		for _, item := range page.Items {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				return result, errors.New("RSS unread entry is missing an ID")
			}
			if _, exists := processed[id]; exists {
				return result, errors.New("RSS mark-all-read collection did not advance")
			}
			if _, mutationErr := service.applyEntryRead(ctx, "desktop", SetEntryReadRequest{
				ID: id, Read: true,
			}, uuid.NewString()); mutationErr != nil {
				return result, mutationErr
			}
			processed[id] = struct{}{}
			result.Updated++
		}
	}
}

// SetEntryReadForDevice is the public synchronization boundary for a paired
// device. Unlike the desktop method, callers must supply an idempotency key and
// an expected state revision; the authenticated device identity is supplied by
// the transport and is never accepted from request JSON.
func (service *Service) SetEntryReadForDevice(
	ctx context.Context,
	deviceID string,
	request SetEntryReadRequest,
) (domainrss.EntryState, error) {
	deviceID = strings.TrimSpace(deviceID)
	mutationID := strings.TrimSpace(request.MutationID)
	if deviceID == "" || strings.TrimSpace(request.ID) == "" || mutationID == "" ||
		request.ExpectedRevision == nil || *request.ExpectedRevision < 0 {
		return domainrss.EntryState{}, errors.New("invalid RSS entry state mutation")
	}
	if service.syncRepository != nil {
		read := request.Read
		return service.SetEntryStateForDevice(ctx, deviceID, SetEntryStateRequest{
			ID: strings.TrimSpace(request.ID), Field: domainrss.EntryStateFieldRead,
			Read: &read, ExpectedRevision: request.ExpectedRevision, MutationID: mutationID,
		})
	}
	return service.applyEntryRead(ctx, deviceID, request, mutationID)
}

func (service *Service) applyEntryRead(
	ctx context.Context,
	deviceID string,
	request SetEntryReadRequest,
	mutationID string,
) (domainrss.EntryState, error) {
	return service.repository.ApplyReadMutation(ctx, domainrss.ReadMutation{
		EntryID: strings.TrimSpace(request.ID), Read: request.Read,
		ExpectedRevision: request.ExpectedRevision, DeviceID: deviceID,
		MutationID: mutationID, ChangedAt: service.now(),
	})
}

func normalizeEntryKindFilter(value string) (domainrss.EntryKind, error) {
	kind := domainrss.EntryKind(strings.ToLower(strings.TrimSpace(value)))
	if kind != "" && kind != domainrss.EntryKindArticle && kind != domainrss.EntryKindSocial &&
		kind != domainrss.EntryKindImage && kind != domainrss.EntryKindVideo {
		return "", errors.New("invalid RSS entry kind")
	}
	return kind, nil
}

func (service *Service) ListChanges(ctx context.Context, request ListChangesRequest) (domainrss.ChangePage, error) {
	return service.repository.ListChanges(ctx, request.After, request.Limit)
}

func (service *Service) Run(ctx context.Context, initialDelay, interval time.Duration) {
	hydrationWorkers := service.pendingHydrations.startWorkers(
		ctx, maxConcurrentRSSRefreshes, service.hydratePendingSubscription,
	)
	defer hydrationWorkers.Wait()
	defer service.pendingHydrations.stopRetries()
	if initialDelay < 0 {
		initialDelay = 0
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	timer := time.NewTimer(initialDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	service.refreshInBackground(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.refreshInBackground(ctx)
		}
	}
}

func (service *Service) refreshInBackground(ctx context.Context) {
	if _, err := service.refreshAll(ctx); err != nil && !errors.Is(err, context.Canceled) {
		zap.L().Debug("refresh RSS subscriptions", rssSafeLogErrorFields(err)...)
	}
}

func (service *Service) refreshAll(ctx context.Context) (RefreshResult, error) {
	subscriptions, err := service.repository.ListSubscriptions(ctx)
	if err != nil {
		return RefreshResult{}, err
	}
	return runBoundedRSSRefreshes(ctx, subscriptions, maxConcurrentRSSRefreshes, service.refreshOne)
}

func runBoundedRSSRefreshes(
	ctx context.Context,
	subscriptions []domainrss.Subscription,
	workerLimit int,
	refresh func(context.Context, string) (domainrss.UpsertResult, error),
) (RefreshResult, error) {
	result := RefreshResult{}
	for _, subscription := range subscriptions {
		if subscription.Enabled {
			result.Subscriptions++
		}
	}
	if workerLimit <= 0 {
		workerLimit = maxConcurrentRSSRefreshes
	}
	workerCount := min(workerLimit, result.Subscriptions)
	var resultMu sync.Mutex
	var wait sync.WaitGroup
	var next atomic.Int64
	for range workerCount {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				index := next.Add(1) - 1
				if index >= int64(len(subscriptions)) {
					return
				}
				subscription := subscriptions[index]
				if !subscription.Enabled {
					continue
				}
				upsert, refreshErr := refresh(ctx, subscription.ID)
				resultMu.Lock()
				if refreshErr != nil {
					result.Failed++
				} else {
					result.Created += upsert.Created
					result.Updated += upsert.Updated
				}
				resultMu.Unlock()
			}
		}()
	}
	wait.Wait()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func (service *Service) refreshOne(ctx context.Context, id string) (domainrss.UpsertResult, error) {
	lockValue, _ := service.locks.LoadOrStore(id, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	subscription, err := service.repository.GetSubscription(ctx, id)
	if err != nil {
		return domainrss.UpsertResult{}, err
	}
	now := service.now()
	wasPending := subscription.LastSuccessAt == nil
	feedURL := subscriptionFetchURL(subscription)
	feed, metadata, fetchErr := service.fetchAndParse(
		ctx, feedURL, subscription.ValidatorURL, subscription.ETag, subscription.LastModified,
	)
	if fetchErr != nil {
		subscription.LastFetchedAt = &now
		subscription.LastError = limitString(fetchErr.Error(), 2048)
		subscription.UpdatedAt = now
		subscription.Revision++
		_, persistErr := service.repository.UpdateSubscription(ctx, subscription)
		return domainrss.UpsertResult{}, errors.Join(fetchErr, persistErr)
	}
	subscription.LastFetchedAt = &now
	subscription.LastSuccessAt = &now
	subscription.LastError = ""
	subscription.ETag = metadata.ETag
	subscription.LastModified = metadata.LastModified
	subscription.ValidatorURL = metadata.ValidatorURL
	subscription.UpdatedAt = now
	subscription.Revision++
	if metadata.NotModified {
		subscription = service.backfillSubscriptionMetadata(ctx, subscription)
		_, err := service.repository.UpdateSubscription(ctx, subscription)
		return domainrss.UpsertResult{}, err
	}
	feed = service.enrichParsedFeedIcon(ctx, feed, subscription.SiteURL)
	if wasPending && strings.TrimSpace(feed.Title) != "" &&
		subscription.Title == feedHostLabel(firstNonEmpty(subscription.SiteURL, feedURL)) {
		subscription.Title = limitString(strings.TrimSpace(feed.Title), 512)
	}
	if feed.SiteURL != "" {
		subscription.SiteURL = feed.SiteURL
	}
	if feed.Description != "" {
		subscription.Description = feed.Description
	}
	if feed.IconURL != "" {
		subscription.IconURL = feed.IconURL
	}
	return service.repository.UpsertFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription,
		Entries:      entriesFromFeed(subscription.ID, subscription.ViewType, feed, now),
	})
}

func subscriptionFetchURL(item domainrss.Subscription) string {
	if item.SourceAccess == domainrss.SubscriptionSourceSharedPublic && strings.TrimSpace(item.PublicFeedURL) != "" {
		return strings.TrimSpace(item.PublicFeedURL)
	}
	return strings.TrimSpace(item.FeedURL)
}

func (service *Service) fetchAndParse(ctx context.Context, canonical, validatorURL, etag, lastModified string) (parsedFeed, fetchMetadata, error) {
	candidates, err := service.resolveFetchCandidates(canonical)
	if err != nil {
		return parsedFeed{}, fetchMetadata{}, err
	}
	errorsByURL := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		body, contentType, metadata, fetchErr := service.fetch(ctx, candidate, validatorURL, etag, lastModified)
		if fetchErr != nil {
			errorsByURL = append(errorsByURL, fetchErr.Error())
			continue
		}
		if metadata.NotModified {
			return parsedFeed{}, metadata, nil
		}
		feed, parseErr := parseFeed(body, contentType)
		if parseErr == nil {
			feed = resolveFeedURLs(feed, metadata.ResolvedURL)
			feed = enrichParsedFeedPublisherSite(feed)
			return feed, metadata, nil
		}
		if discovered := discoverFeedURL(body, metadata.ResolvedURL); discovered != "" && discovered != candidate {
			discoveredBody, discoveredType, discoveredMetadata, discoveredErr := service.fetch(ctx, discovered, "", "", "")
			if discoveredErr == nil {
				if discoveredFeed, discoveredParseErr := parseFeed(discoveredBody, discoveredType); discoveredParseErr == nil {
					discoveredFeed = resolveFeedURLs(discoveredFeed, discoveredMetadata.ResolvedURL)
					discoveredFeed = enrichParsedFeedPublisherSite(discoveredFeed)
					return discoveredFeed, discoveredMetadata, nil
				}
			}
		}
		errorsByURL = append(errorsByURL, fmt.Sprintf(
			"%s: invalid feed (errorRef=%s)",
			redactedFeedURL(candidate),
			rssOpaqueLogReference(parseErr.Error()),
		))
	}
	return parsedFeed{}, fetchMetadata{}, fmt.Errorf("RSS fetch failed: %s", strings.Join(errorsByURL, "; "))
}

func (service *Service) fetch(ctx context.Context, rawURL, validatorURL, etag, lastModified string) ([]byte, string, fetchMetadata, error) {
	candidateFetchURL, err := normalizeFeedValidatorURL(rawURL)
	if err != nil {
		return nil, "", fetchMetadata{}, err
	}
	candidateValidatorURL := boundedFeedValidatorURL(candidateFetchURL)
	storedValidatorURL := ""
	if normalized, normalizeErr := normalizeFeedValidatorURL(validatorURL); normalizeErr == nil {
		storedValidatorURL = boundedFeedValidatorURL(normalized)
	}
	if candidateValidatorURL != storedValidatorURL || !boundedFeedValidator(etag) || !boundedFeedValidator(lastModified) {
		etag = ""
		lastModified = ""
	}
	requestCtx, cancel := context.WithTimeout(ctx, feedRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, candidateFetchURL, nil)
	if err != nil {
		return nil, "", fetchMetadata{}, err
	}
	request.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/feed+json, application/json, application/xml, text/xml, text/html;q=0.7, */*;q=0.2")
	request.Header.Set("User-Agent", "XiaDown RSS/1.0")
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		request.Header.Set("If-Modified-Since", lastModified)
	}
	client := service.httpClient()
	response, err := client.Do(request)
	if err != nil {
		return nil, "", fetchMetadata{}, newRedactedFeedError("fetch "+redactedFeedURL(rawURL), err)
	}
	defer response.Body.Close()
	resolvedRawURL := candidateValidatorURL
	if response.Request != nil && response.Request.URL != nil {
		resolvedRawURL = response.Request.URL.String()
	}
	resolvedFetchURL, normalizeErr := normalizeFeedValidatorURL(resolvedRawURL)
	if normalizeErr != nil {
		return nil, "", fetchMetadata{}, fmt.Errorf("validate resolved feed URL: %w", normalizeErr)
	}
	resolvedValidatorURL := boundedFeedValidatorURL(resolvedFetchURL)
	metadata := fetchMetadata{ResolvedURL: resolvedFetchURL}
	if response.StatusCode == http.StatusNotModified {
		if !responseCarriedFeedValidators(response, etag, lastModified) ||
			resolvedValidatorURL == "" || resolvedValidatorURL != storedValidatorURL {
			return nil, "", fetchMetadata{}, fmt.Errorf("fetch %s: unprovenanced HTTP 304", redactedFeedURL(rawURL))
		}
		metadata.ETag = etag
		metadata.LastModified = lastModified
		metadata.ValidatorURL = storedValidatorURL
		metadata.NotModified = true
		return nil, response.Header.Get("Content-Type"), metadata, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
		return nil, "", fetchMetadata{}, fmt.Errorf("fetch %s: HTTP %d", redactedFeedURL(rawURL), response.StatusCode)
	}
	responseETag := strings.TrimSpace(response.Header.Get("ETag"))
	responseLastModified := strings.TrimSpace(response.Header.Get("Last-Modified"))
	if resolvedValidatorURL != "" && boundedFeedValidator(responseETag) && boundedFeedValidator(responseLastModified) &&
		(responseETag != "" || responseLastModified != "") {
		metadata.ETag = responseETag
		metadata.LastModified = responseLastModified
		metadata.ValidatorURL = resolvedValidatorURL
	}
	limited := io.LimitReader(response.Body, maxFeedResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fetchMetadata{}, newRedactedFeedError("read "+redactedFeedURL(rawURL), err)
	}
	if len(body) > maxFeedResponseBytes {
		return nil, "", fetchMetadata{}, fmt.Errorf("feed response exceeds %d bytes", maxFeedResponseBytes)
	}
	return body, response.Header.Get("Content-Type"), metadata, nil
}

func normalizeFeedValidatorURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if len(trimmed) == 0 || len(trimmed) > maxFeedValidatorURLBytes {
		return "", errors.New("RSS feed URL exceeds the validator safety limit")
	}
	parsed, err := networkpolicy.ValidatePublicHTTPURL(trimmed)
	if err != nil {
		return "", err
	}
	scheme, host, port, ok := normalizedHTTPOrigin(parsed)
	if !ok {
		return "", errors.New("invalid RSS feed validator URL")
	}
	parsed.Scheme = scheme
	parsed.Host = canonicalHTTPValidatorHost(host, port, scheme)
	parsed.Fragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	if parsed.RawQuery == "" {
		parsed.ForceQuery = false
	}
	normalized := parsed.String()
	if len(normalized) > maxFeedValidatorURLBytes {
		return "", errors.New("RSS feed URL exceeds the validator safety limit")
	}
	return normalized, nil
}

func boundedFeedValidator(value string) bool {
	return len(value) <= maxFeedValidatorBytes
}

func boundedFeedValidatorURL(value string) string {
	if len(value) == 0 || len(value) > maxFeedValidatorURLBytes {
		return ""
	}
	return value
}

func responseCarriedFeedValidators(response *http.Response, etag, lastModified string) bool {
	if response == nil || response.Request == nil {
		return false
	}
	headers := response.Request.Header
	if headers.Get("If-None-Match") != etag || headers.Get("If-Modified-Since") != lastModified {
		return false
	}
	return etag != "" || lastModified != ""
}

type redactedFeedError struct {
	message string
	cause   error
}

func (err *redactedFeedError) Error() string { return err.message }
func (err *redactedFeedError) Unwrap() error { return err.cause }

func newRedactedFeedError(operation string, cause error) error {
	reason := "request failed"
	switch {
	case errors.Is(cause, context.DeadlineExceeded):
		reason = "request timed out"
	case errors.Is(cause, context.Canceled):
		reason = "request canceled"
	case errors.Is(cause, networkpolicy.ErrDestinationBlocked):
		reason = "network destination is blocked"
	default:
		var netErr net.Error
		if errors.As(cause, &netErr) && netErr.Timeout() {
			reason = "network timeout"
		}
	}
	return &redactedFeedError{message: operation + ": " + reason, cause: cause}
}

func redactedFeedURL(rawURL string) string {
	reference := rssOpaqueLogReference(rawURL)
	if reference == "" {
		return "<redacted-feed-url>"
	}
	return "<feed-ref:" + reference + ">"
}

func (service *Service) httpClient() *http.Client {
	base := http.DefaultClient
	if service.clients != nil {
		if provided := service.clients.HTTPClient(); provided != nil {
			base = provided
		}
	}
	clone := *base
	if managedDialer, ok := service.clients.(publicURLRouteDialer); ok {
		clone.Transport = &managedPublicHTTPTransport{
			dialer: managedDialer, timeouts: defaultRemoteResourceTransportTimeouts,
		}
	} else if _, allowed := service.clients.(pinnedPublicRouteTestSeam); allowed {
		clone.Transport = newPublicHTTPTransport(base.Transport, service.resolver)
	} else {
		clone.Transport = rejectedPublicHTTPTransport{err: errors.New("RSS feed request requires the managed App route")}
	}
	clone.Jar = nil
	clone.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many RSS redirects")
		}
		if request.URL == nil || len(request.URL.String()) > maxFeedValidatorURLBytes {
			return errors.New("RSS redirect URL exceeds the validator safety limit")
		}
		for _, header := range []string{
			"Authorization", "Cookie", "Origin", "Referer", "Proxy-Authorization",
			"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-IP",
		} {
			request.Header.Del(header)
		}
		if len(via) > 0 && !redirectStayedOnInitialValidatorURL(request.URL, via) {
			for _, header := range []string{"If-None-Match", "If-Modified-Since", "If-Range"} {
				request.Header.Del(header)
			}
		}
		parsed, err := networkpolicy.ValidatePublicHTTPURL(request.URL.String())
		if err != nil {
			return err
		}
		resolveCtx, cancel := context.WithTimeout(request.Context(), rssDNSDeadline)
		_, err = networkpolicy.ResolvePublicIPs(resolveCtx, service.resolver, parsed.Hostname())
		cancel()
		if err != nil {
			return err
		}
		return nil
	}
	return &clone
}

func (service *Service) resolveFetchCandidates(canonical string) ([]string, error) {
	if !strings.HasPrefix(strings.ToLower(canonical), RSSHubScheme) {
		return []string{canonical}, nil
	}
	route := strings.TrimLeft(strings.TrimSpace(canonical[len(RSSHubScheme):]), "/")
	if route == "" || strings.Contains(route, "://") || strings.Contains(route, "..") {
		return nil, errors.New("invalid RSSHub route")
	}
	items := make([]string, 0, len(service.mirrors))
	for _, mirror := range service.mirrors {
		items = append(items, strings.TrimRight(mirror, "/")+"/"+route)
	}
	return items, nil
}

func normalizeFeedURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "/") {
		value = RSSHubScheme + strings.TrimLeft(value, "/")
	}
	if strings.HasPrefix(strings.ToLower(value), RSSHubScheme) {
		route := strings.TrimLeft(strings.TrimSpace(value[len(RSSHubScheme):]), "/")
		if route == "" || strings.Contains(route, "://") || strings.Contains(route, "..") ||
			strings.ContainsAny(route, "{}*") || discoveryTemplatePattern.MatchString(route) {
			return "", errors.New("invalid RSSHub route")
		}
		return RSSHubScheme + route, nil
	}
	return normalizeFeedValidatorURL(value)
}

func normalizeViewType(value string) (domainrss.ViewType, error) {
	view := domainrss.ViewType(strings.ToLower(strings.TrimSpace(value)))
	if view == "" {
		view = domainrss.ViewTypeAuto
	}
	switch view {
	case domainrss.ViewTypeAuto, domainrss.ViewTypeArticle, domainrss.ViewTypeSocial,
		domainrss.ViewTypeImage, domainrss.ViewTypeVideo:
		return view, nil
	default:
		return "", errors.New("invalid RSS view type")
	}
}

func subscriptionFromParsedFeed(id, canonical string, view domainrss.ViewType, feed parsedFeed, now time.Time) domainrss.Subscription {
	title := strings.TrimSpace(feed.Title)
	if title == "" {
		title = feedHostLabel(firstNonEmpty(feed.SiteURL, canonical))
	}
	siteURL := safeEntryResourceURL(canonical, feed.SiteURL)
	iconURL := safeEntryResourceURL(firstNonEmpty(siteURL, canonical), feed.IconURL)
	return domainrss.Subscription{
		ID: id, WorkspaceID: domainrss.DefaultWorkspaceID, FeedURL: canonical,
		SourceAccess: domainrss.SubscriptionSourceDesktopManaged,
		SiteURL:      siteURL, Title: title, Description: feed.Description, IconURL: iconURL,
		ViewType: view, Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
}

func resolveFeedURLs(feed parsedFeed, sourceURL string) parsedFeed {
	feed.SiteURL = safeEntryResourceURL(sourceURL, feed.SiteURL)
	feed.IconURL = safeEntryResourceURL(firstNonEmpty(feed.SiteURL, sourceURL), feed.IconURL)
	feed.HistoryURL = resolveURL(sourceURL, feed.HistoryURL)
	for index := range feed.Entries {
		feed.Entries[index].URL = safeEntryResourceURL(firstNonEmpty(feed.SiteURL, sourceURL), feed.Entries[index].URL)
	}
	return feed
}

func discoverFeedURL(body []byte, pageURL string) string {
	if len(body) == 0 || len(body) > maxFeedDiscoveryHTMLBytes {
		return ""
	}
	tokenizer := xhtml.NewTokenizer(bytes.NewReader(body))
	for tokens := 0; tokens < maxFeedDiscoveryHTMLTokens; tokens++ {
		tokenType := tokenizer.Next()
		if tokenType == xhtml.ErrorToken {
			return ""
		}
		if tokenType != xhtml.StartTagToken && tokenType != xhtml.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if !strings.EqualFold(token.Data, "link") {
			continue
		}
		var rel, mimeType, href string
		for _, attribute := range token.Attr {
			switch strings.ToLower(attribute.Key) {
			case "rel":
				rel = strings.ToLower(attribute.Val)
			case "type":
				mimeType = strings.ToLower(attribute.Val)
			case "href":
				href = strings.TrimSpace(attribute.Val)
			}
		}
		if len(href) == 0 || len(href) > maxFeedValidatorURLBytes || !strings.Contains(rel, "alternate") {
			continue
		}
		if strings.Contains(mimeType, "rss") || strings.Contains(mimeType, "atom") || strings.Contains(mimeType, "feed+json") {
			resolved := resolveURL(pageURL, href)
			if len(resolved) <= maxFeedValidatorURLBytes {
				return resolved
			}
			return ""
		}
	}
	return ""
}

func feedHostLabel(rawURL string) string {
	if strings.HasPrefix(strings.ToLower(rawURL), RSSHubScheme) {
		return "RSSHub"
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return "RSS"
}

func playbackURLForEntry(entry domainrss.Entry) string {
	if entry.PlaybackURL != "" {
		return entry.PlaybackURL
	}
	_, _, playback := resolveVideoPlatform(entry.URL, entry.Media)
	return playback
}
