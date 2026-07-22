package rss

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	domainrss "xiadown/internal/domain/rss"

	"go.uber.org/zap"
)

const maxPendingRSSHydrations = 64

var pendingRSSHydrationRetryDelays = []time.Duration{
	10 * time.Second,
	time.Minute,
	5 * time.Minute,
}

type pendingHydrationState struct {
	failures int
	timer    *time.Timer
}

type pendingHydrationQueue struct {
	jobs        chan string
	mu          sync.Mutex
	ids         map[string]*pendingHydrationState
	retryDelays []time.Duration
}

func newPendingHydrationQueue(capacity int) *pendingHydrationQueue {
	if capacity <= 0 {
		capacity = maxPendingRSSHydrations
	}
	return &pendingHydrationQueue{
		jobs:        make(chan string, capacity),
		ids:         make(map[string]*pendingHydrationState, capacity),
		retryDelays: append([]time.Duration(nil), pendingRSSHydrationRetryDelays...),
	}
}

// Enqueue retains work even before Service.Run starts. A full queue returns
// false without blocking AddSubscription; the periodic refresh remains the
// durable fallback for every enabled pending subscription.
func (queue *pendingHydrationQueue) Enqueue(subscriptionID string) bool {
	if queue == nil {
		return false
	}
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return false
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if _, exists := queue.ids[subscriptionID]; exists {
		return true
	}
	// Timers waiting to retry are tracked even while they are not occupying a
	// channel slot. Bound the complete tracked set, not only the buffered jobs,
	// so a long outage cannot temporarily grow the queue beyond its contract.
	if len(queue.ids) >= cap(queue.jobs) {
		return false
	}
	queue.ids[subscriptionID] = &pendingHydrationState{}
	select {
	case queue.jobs <- subscriptionID:
		return true
	default:
		delete(queue.ids, subscriptionID)
		return false
	}
}

func (queue *pendingHydrationQueue) complete(subscriptionID string) {
	queue.mu.Lock()
	if state := queue.ids[subscriptionID]; state != nil && state.timer != nil {
		state.timer.Stop()
	}
	delete(queue.ids, subscriptionID)
	queue.mu.Unlock()
}

func (queue *pendingHydrationQueue) scheduleRetry(
	ctx context.Context,
	subscriptionID string,
) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	state := queue.ids[subscriptionID]
	if state == nil || ctx.Err() != nil {
		delete(queue.ids, subscriptionID)
		return false
	}
	state.failures++
	if state.failures > len(queue.retryDelays) {
		delete(queue.ids, subscriptionID)
		return false
	}
	delay := queue.retryDelays[state.failures-1]
	state.timer = time.AfterFunc(delay, func() {
		queue.enqueueRetry(ctx, subscriptionID)
	})
	return true
}

func (queue *pendingHydrationQueue) enqueueRetry(ctx context.Context, subscriptionID string) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	state := queue.ids[subscriptionID]
	if state == nil {
		return
	}
	state.timer = nil
	if ctx.Err() != nil {
		delete(queue.ids, subscriptionID)
		return
	}
	select {
	case queue.jobs <- subscriptionID:
	default:
		// Capacity is shared with the tracked-ID set, so this should only be
		// reachable during shutdown races. Periodic refresh is the fallback.
		delete(queue.ids, subscriptionID)
	}
}

func (queue *pendingHydrationQueue) stopRetries() {
	if queue == nil {
		return
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	for subscriptionID, state := range queue.ids {
		if state.timer != nil {
			state.timer.Stop()
		}
		delete(queue.ids, subscriptionID)
	}
}

func (queue *pendingHydrationQueue) startWorkers(
	ctx context.Context,
	workerLimit int,
	refresh func(context.Context, string) (domainrss.UpsertResult, error),
) *sync.WaitGroup {
	wait := &sync.WaitGroup{}
	if queue == nil || refresh == nil {
		return wait
	}
	if workerLimit <= 0 {
		workerLimit = maxConcurrentRSSRefreshes
	}
	for range workerLimit {
		wait.Add(1)
		go func() {
			defer wait.Done()
			queue.runWorker(ctx, refresh)
		}()
	}
	return wait
}

func (queue *pendingHydrationQueue) runWorker(
	ctx context.Context,
	refresh func(context.Context, string) (domainrss.UpsertResult, error),
) {
	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case subscriptionID := <-queue.jobs:
			_, err := refresh(ctx, subscriptionID)
			if err == nil || errors.Is(err, domainrss.ErrNotFound) {
				queue.complete(subscriptionID)
				continue
			}
			if ctx.Err() == nil {
				retrying := queue.scheduleRetry(ctx, subscriptionID)
				fields := []zap.Field{zap.Bool("retrying", retrying)}
				fields = append(fields, rssSafeLogErrorFields(err)...)
				zap.L().Debug("hydrate pending RSS subscription", fields...)
			} else {
				queue.complete(subscriptionID)
			}
		}
	}
}

func (service *Service) hydratePendingSubscription(
	ctx context.Context,
	subscriptionID string,
) (domainrss.UpsertResult, error) {
	subscription, err := service.repository.GetSubscription(ctx, subscriptionID)
	if errors.Is(err, domainrss.ErrNotFound) {
		return domainrss.UpsertResult{}, nil
	}
	if err != nil {
		return domainrss.UpsertResult{}, err
	}
	if !subscription.Enabled || subscription.LastSuccessAt != nil {
		return domainrss.UpsertResult{}, nil
	}
	return service.refreshOne(ctx, subscriptionID)
}
