package rss

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

const (
	maxRSSHistoryNoProgress = 3

	historyErrorCanceled    = "RSS history request canceled"
	historyErrorTimedOut    = "RSS history request timed out"
	historyErrorUnavailable = "RSS history request failed"
	historyErrorInvalidLink = "RSS history link is invalid or blocked"
)

type BackfillHistoryRequest struct {
	SubscriptionID string `json:"subscriptionId,omitempty"`
	Kind           string `json:"kind,omitempty"`
}

type BackfillHistorySourceResult struct {
	SubscriptionID string `json:"subscriptionId"`
	Attempted      bool   `json:"attempted"`
	Capability     string `json:"capability"`
	Exhausted      bool   `json:"exhausted"`
	NoProgress     int    `json:"noProgress"`
	Created        int    `json:"created"`
	Updated        int    `json:"updated"`
	Error          string `json:"error,omitempty"`
}

type BackfillHistoryResult struct {
	Subscriptions int                           `json:"subscriptions"`
	Attempted     int                           `json:"attempted"`
	Supported     int                           `json:"supported"`
	Unsupported   int                           `json:"unsupported"`
	Exhausted     int                           `json:"exhausted"`
	Created       int                           `json:"created"`
	Updated       int                           `json:"updated"`
	Failed        int                           `json:"failed"`
	HasMore       bool                          `json:"hasMore"`
	Sources       []BackfillHistorySourceResult `json:"sources"`
}

// BackfillHistory fetches at most one historical page for each selected
// source. A subscription scope includes that source even when paused. An
// aggregate kind scope intentionally includes every enabled source: automatic
// and mixed feeds can contain a requested kind without resolving to it as
// their dominant layout.
func (service *Service) BackfillHistory(
	ctx context.Context,
	request BackfillHistoryRequest,
) (BackfillHistoryResult, error) {
	if service == nil {
		return BackfillHistoryResult{}, errors.New("RSS service unavailable")
	}
	kind, err := normalizeEntryKindFilter(request.Kind)
	if err != nil {
		return BackfillHistoryResult{}, err
	}
	if service.repository == nil {
		return BackfillHistoryResult{}, errors.New("RSS service unavailable")
	}
	historyRepository, ok := service.repository.(domainrss.HistoryRepository)
	if !ok {
		return BackfillHistoryResult{}, errors.New("RSS history repository unavailable")
	}

	var subscriptions []domainrss.Subscription
	if id := strings.TrimSpace(request.SubscriptionID); id != "" {
		subscription, err := service.repository.GetSubscription(ctx, id)
		if err != nil {
			return BackfillHistoryResult{}, err
		}
		// A source without a successful head fetch has no trustworthy RFC 5005
		// starting point. Return a terminal empty result so older clients do not
		// loop on capability=unknown/hasMore=true while hydration is pending.
		if subscription.LastSuccessAt != nil {
			subscriptions = []domainrss.Subscription{subscription}
		}
	} else {
		items, err := service.repository.ListSubscriptions(ctx)
		if err != nil {
			return BackfillHistoryResult{}, err
		}
		subscriptions = make([]domainrss.Subscription, 0, len(items))
		for _, item := range items {
			// Pending subscriptions are owned by the immediate hydration queue.
			// Aggregate history work must not race their first successful fetch
			// or treat a provisional feed URL as an RFC 5005 head document.
			if item.Enabled && item.LastSuccessAt != nil {
				subscriptions = append(subscriptions, item)
			}
		}
	}

	return runBoundedRSSHistoryBackfills(
		ctx,
		subscriptions,
		maxConcurrentRSSRefreshes,
		func(ctx context.Context, subscription domainrss.Subscription) BackfillHistorySourceResult {
			return service.backfillOneHistoryPage(ctx, historyRepository, subscription, kind)
		},
	)
}

func runBoundedRSSHistoryBackfills(
	ctx context.Context,
	subscriptions []domainrss.Subscription,
	workerLimit int,
	backfill func(context.Context, domainrss.Subscription) BackfillHistorySourceResult,
) (BackfillHistoryResult, error) {
	result := BackfillHistoryResult{
		Subscriptions: len(subscriptions),
		Sources:       make([]BackfillHistorySourceResult, len(subscriptions)),
	}
	for index, subscription := range subscriptions {
		result.Sources[index].SubscriptionID = subscription.ID
	}
	if len(subscriptions) == 0 {
		return result, nil
	}
	if workerLimit <= 0 {
		workerLimit = maxConcurrentRSSRefreshes
	}
	workerCount := min(workerLimit, len(subscriptions))
	var wait sync.WaitGroup
	var next atomic.Int64
	for range workerCount {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				index := int(next.Add(1) - 1)
				if index >= len(subscriptions) {
					return
				}
				result.Sources[index] = backfill(ctx, subscriptions[index])
			}
		}()
	}
	wait.Wait()

	for _, source := range result.Sources {
		if source.Attempted {
			result.Attempted++
		}
		if source.Capability == string(domainrss.HistoryCapabilityAvailable) {
			result.Supported++
		} else if source.Capability == string(domainrss.HistoryCapabilityUnsupported) {
			result.Unsupported++
		}
		if !source.Exhausted && source.Capability != string(domainrss.HistoryCapabilityUnsupported) {
			result.HasMore = true
		}
		if source.Exhausted {
			result.Exhausted++
		}
		result.Created += source.Created
		result.Updated += source.Updated
		if source.Error != "" {
			result.Failed++
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func (service *Service) backfillOneHistoryPage(
	ctx context.Context,
	historyRepository domainrss.HistoryRepository,
	subscription domainrss.Subscription,
	visibleKind domainrss.EntryKind,
) BackfillHistorySourceResult {
	result := BackfillHistorySourceResult{
		SubscriptionID: subscription.ID,
		Capability:     string(domainrss.HistoryCapabilityUnknown),
	}
	lockValue, _ := service.locks.LoadOrStore(subscription.ID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	// The list snapshot was captured before this per-feed lock. A refresh or a
	// preceding backfill may have advanced the subscription revision while this
	// worker waited, so all writes must be based on a fresh locked read.
	current, err := service.repository.GetSubscription(ctx, subscription.ID)
	if err != nil {
		return historyBackfillFailure(result, err)
	}
	subscription = current

	state, err := historyRepository.GetSubscriptionHistory(ctx, subscription.ID)
	if errors.Is(err, domainrss.ErrNotFound) {
		state = domainrss.SubscriptionHistoryState{
			SubscriptionID: subscription.ID,
			Capability:     domainrss.HistoryCapabilityUnknown,
		}
	} else if err != nil {
		return historyBackfillFailure(result, err)
	}
	result.Capability = string(state.Capability)
	result.Exhausted = state.Exhausted
	result.NoProgress = state.NoProgress
	if state.Exhausted {
		return result
	}

	if strings.TrimSpace(state.CursorURL) == "" {
		result.Attempted = true
		feed, metadata, fetchErr := service.fetchAndParse(ctx, subscriptionFetchURL(subscription), "", "", "")
		if fetchErr != nil {
			return service.persistHistoryFailure(ctx, historyRepository, state, result, fetchErr)
		}
		cursor, cursorErr := normalizeHistoryCursor(feed.HistoryURL)
		if cursorErr != nil {
			return service.persistUnsupportedHistory(ctx, historyRepository, state, result, cursorErr)
		}
		if cursor == "" {
			return service.persistUnsupportedHistory(ctx, historyRepository, state, result, nil)
		}
		resolvedHead, _ := normalizeHistoryCursor(metadata.ResolvedURL)
		if resolvedHead != "" && cursor == resolvedHead {
			return service.persistUnsupportedHistory(
				ctx, historyRepository, state, result, errors.New("RSS history link points to the current feed"),
			)
		}
		state.CursorURL = cursor
		state.Capability = domainrss.HistoryCapabilityAvailable
		state.Exhausted = false
	}

	result.Attempted = true
	result.Capability = string(domainrss.HistoryCapabilityAvailable)
	attemptedAt := service.now()
	state.LastAttemptAt = &attemptedAt
	state.UpdatedAt = attemptedAt
	feed, _, fetchErr := service.fetchAndParse(ctx, state.CursorURL, "", "", "")
	if fetchErr != nil {
		return service.persistHistoryFailure(ctx, historyRepository, state, result, fetchErr)
	}

	entries := entriesFromFeed(subscription.ID, subscription.ViewType, feed, attemptedAt)
	markHistoricalEntriesRead(entries, attemptedAt)
	upsert := domainrss.HistoricalUpsertResult{}
	if len(entries) > 0 {
		subscription.Revision++
		subscription.UpdatedAt = attemptedAt
		upsert, err = historyRepository.UpsertHistoricalFeed(ctx, domainrss.FeedUpdate{
			Subscription: subscription,
			Entries:      entries,
		}, visibleKind)
		if err != nil {
			return service.persistHistoryFailure(ctx, historyRepository, state, result, err)
		}
	}
	result.Created = upsert.Visible.Created
	result.Updated = upsert.Visible.Updated
	if upsert.Total.Created == 0 && upsert.Total.Updated == 0 {
		state.NoProgress++
	} else {
		state.NoProgress = 0
	}

	nextCursor, cursorErr := normalizeHistoryCursor(feed.HistoryURL)
	if cursorErr != nil {
		return service.persistHistoryFailure(ctx, historyRepository, state, result, cursorErr)
	}
	currentCursor := state.CursorURL
	state.CursorURL = nextCursor
	state.Capability = domainrss.HistoryCapabilityAvailable
	state.Exhausted = nextCursor == "" || nextCursor == currentCursor || state.NoProgress >= maxRSSHistoryNoProgress
	state.LastSuccessAt = &attemptedAt
	state.LastError = ""
	state.UpdatedAt = attemptedAt
	if err := historyRepository.PutSubscriptionHistory(ctx, state); err != nil {
		return historyBackfillFailure(result, err)
	}
	result.Exhausted = state.Exhausted
	result.NoProgress = state.NoProgress
	return result
}

func (service *Service) persistUnsupportedHistory(
	ctx context.Context,
	repository domainrss.HistoryRepository,
	state domainrss.SubscriptionHistoryState,
	result BackfillHistorySourceResult,
	cause error,
) BackfillHistorySourceResult {
	now := service.now()
	state.CursorURL = ""
	state.Capability = domainrss.HistoryCapabilityUnsupported
	state.Exhausted = true
	state.LastAttemptAt = &now
	state.UpdatedAt = now
	state.LastError = ""
	if cause != nil {
		state.LastError = historyErrorInvalidLink
	}
	if err := repository.PutSubscriptionHistory(ctx, state); err != nil {
		return historyBackfillFailure(result, err)
	}
	result.Capability = string(state.Capability)
	result.Exhausted = true
	result.NoProgress = state.NoProgress
	result.Error = state.LastError
	return result
}

func (service *Service) persistHistoryFailure(
	ctx context.Context,
	repository domainrss.HistoryRepository,
	state domainrss.SubscriptionHistoryState,
	result BackfillHistorySourceResult,
	cause error,
) BackfillHistorySourceResult {
	now := service.now()
	state.LastAttemptAt = &now
	state.UpdatedAt = now
	state.LastError = limitString(historyErrorText(cause), 2048)
	if err := repository.PutSubscriptionHistory(ctx, state); err != nil {
		cause = errors.Join(cause, err)
	}
	result.Capability = string(state.Capability)
	result.Exhausted = state.Exhausted
	result.NoProgress = state.NoProgress
	return historyBackfillFailure(result, cause)
}

func historyBackfillFailure(result BackfillHistorySourceResult, err error) BackfillHistorySourceResult {
	result.Error = limitString(historyErrorText(err), 2048)
	return result
}

func historyErrorText(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return historyErrorCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return historyErrorTimedOut
	}
	// Error text is persisted and projected through Wails. Never include an
	// upstream error because it may embed an opaque RFC 5005 cursor, including
	// userinfo, query credentials, or publisher tokens in path segments.
	return historyErrorUnavailable
}

func normalizeHistoryCursor(rawURL string) (string, error) {
	if strings.TrimSpace(rawURL) == "" {
		return "", nil
	}
	cursor, err := normalizeFeedValidatorURL(rawURL)
	if err != nil {
		return "", errors.New(historyErrorInvalidLink)
	}
	return cursor, nil
}

func markHistoricalEntriesRead(entries []domainrss.Entry, readAt time.Time) {
	readAt = readAt.UTC()
	for index := range entries {
		entries[index].ReadAt = &readAt
		entries[index].ReadStateUpdatedAt = &readAt
		entries[index].FieldRevisions.Read = 1
		entries[index].StateRevision = 1
	}
}
