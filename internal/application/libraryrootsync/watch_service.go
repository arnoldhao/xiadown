package libraryrootsync

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	domain "xiadown/internal/domain/libraryrootsync"
)

func (service *Service) startWatcher(root Root) {
	if !service.watcher.Available() || !isScannableRoot(root) {
		return
	}
	service.mu.Lock()
	existing := service.watches[root.ID]
	if existing != nil &&
		canonicalPath(existing.path) == canonicalPath(root.Path) {
		service.mu.Unlock()
		return
	}
	if existing != nil {
		existing.cancel()
	}
	ctx, cancel := context.WithCancel(service.baseContext)
	run := &watchRun{
		cancel: cancel,
		done:   make(chan struct{}),
		path:   root.Path,
	}
	service.watches[root.ID] = run
	service.mu.Unlock()
	go service.runWatcher(ctx, root, run)
}

func (service *Service) runWatcher(
	ctx context.Context,
	root Root,
	run *watchRun,
) {
	defer close(run.done)
	state, _ := service.repository.GetState(ctx, root.ID)
	watchRootPath := root.Path
	if resolved, err := filepath.EvalSymlinks(root.Path); err == nil {
		watchRootPath = resolved
	}
	events := make(chan watchEvent, 4096)
	overflows := make(chan watchEvent, 1)
	watchErr := make(chan error, 1)
	go func() {
		watchErr <- service.watcher.Watch(
			ctx,
			root.Path,
			state.WatcherCursor,
			func(event watchEvent) {
				enqueueWatchEvent(ctx, events, overflows, event)
			},
		)
	}()

	var timer *time.Timer
	var timerChannel <-chan time.Time
	request := scanRequest{paths: make(map[string]struct{}), settle: true}
	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(watcherDebounce)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(watcherDebounce)
		}
		timerChannel = timer.C
	}
	flush := func() {
		if request.full || len(request.paths) > 0 {
			service.enqueue(root, request)
		}
		request = scanRequest{
			paths:  make(map[string]struct{}),
			settle: true,
		}
		timerChannel = nil
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
		service.mu.Lock()
		if service.watches[root.ID] == run {
			delete(service.watches, root.ID)
		}
		service.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-watchErr:
			if err != nil && !errors.Is(err, context.Canceled) &&
				!errors.Is(err, errNativeWatcherUnavailable) {
				request.full = true
				request.paths = nil
				flush()
			}
			return
		case event := <-events:
			if event.checkpoint {
				_ = service.repository.AdvanceWatcherCursor(
					ctx,
					root.ID,
					event.cursor,
				)
				continue
			}
			if event.cursor > request.cursor {
				request.cursor = event.cursor
			}
			if event.overflow {
				request.full = true
				request.paths = nil
			} else if !request.full {
				path, err := filepath.Abs(event.path)
				if err == nil {
					if relative, err := safeRelativePath(
						watchRootPath,
						path,
					); err == nil {
						if relative == "." && event.directory {
							if len(request.paths) == 0 {
								_ = service.repository.AdvanceWatcherCursor(
									ctx,
									root.ID,
									event.cursor,
								)
							}
							continue
						}
						request.paths[filepath.Join(
							root.Path,
							filepath.FromSlash(relative),
						)] = struct{}{}
					}
				}
			}
			resetTimer()
		case event := <-overflows:
			if event.cursor > request.cursor {
				request.cursor = event.cursor
			}
			request.full = true
			request.paths = nil
			resetTimer()
		case <-timerChannel:
			flush()
		}
	}
}

// enqueueWatchEvent keeps overflow notification independent from the saturated
// event queue. Otherwise the notification itself is dropped precisely when it
// is needed, leaving the root stale until the next periodic reconciliation.
func enqueueWatchEvent(
	ctx context.Context,
	events chan<- watchEvent,
	overflows chan<- watchEvent,
	event watchEvent,
) {
	select {
	case <-ctx.Done():
		return
	case events <- event:
		return
	default:
	}
	select {
	case <-ctx.Done():
	case overflows <- watchEvent{overflow: true, cursor: event.cursor}:
	default:
	}
}

func (service *Service) queuePath(root Root, path string, cursor uint64) {
	service.enqueue(root, scanRequest{
		paths:  map[string]struct{}{path: {}},
		settle: true,
		cursor: cursor,
	})
}

// Ensure the domain import remains intentional: watcher startup can resume
// from a persisted FSEvents cursor even when no scan is currently active.
var _ domain.Status = domain.StatusWatching
