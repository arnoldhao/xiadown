package browsercdp

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type TargetEventKind string

const (
	TargetEventCreated     TargetEventKind = "target_created"
	TargetEventInfoChanged TargetEventKind = "target_info_changed"
	TargetEventAttached    TargetEventKind = "target_attached"
	TargetEventDetached    TargetEventKind = "target_detached"
	TargetEventDestroyed   TargetEventKind = "target_destroyed"
	TargetEventCrashed     TargetEventKind = "target_crashed"
)

type TargetEvent struct {
	Kind      TargetEventKind
	TargetID  string
	SessionID string
	Info      *targetpkg.Info
	Status    string
	ErrorCode int64
}

type PageTargetWatcher struct {
	cancel     context.CancelFunc
	manager    *PageTargetManager
	listenerID uint64

	mu                sync.Mutex
	targetBySession   map[string]string
	excludedTargetIDs map[string]struct{}
}

type PageTargetManager struct {
	cancel context.CancelFunc
	events chan any
	done   chan struct{}

	mu              sync.RWMutex
	targets         map[string]*targetpkg.Info
	targetBySession map[string]string
	excludedTargets map[string]struct{}
	listeners       map[uint64]func(TargetEvent)
	nextListenerID  uint64
	waiters         map[uint64]pageTargetWaiter
	nextWaiterID    uint64
}

type pageTargetWaiter struct {
	predicate func(*targetpkg.Info) bool
	ch        chan *targetpkg.Info
}

func startPageTargetManager(runtime *Runtime) (*PageTargetManager, error) {
	if runtime == nil {
		return nil, errors.New("browser runtime unavailable")
	}
	browserCtx := runtime.BrowserContext()
	if browserCtx == nil {
		return nil, errors.New("browser context unavailable")
	}
	watchCtx, cancel := context.WithCancel(browserCtx)
	manager := &PageTargetManager{
		cancel:          cancel,
		events:          make(chan any, 4096),
		done:            make(chan struct{}),
		targets:         map[string]*targetpkg.Info{},
		targetBySession: map[string]string{},
		excludedTargets: map[string]struct{}{},
		listeners:       map[uint64]func(TargetEvent){},
		waiters:         map[uint64]pageTargetWaiter{},
	}
	go manager.run(watchCtx)
	chromedp.ListenBrowser(watchCtx, func(ev any) {
		select {
		case manager.events <- ev:
		case <-watchCtx.Done():
		default:
			go manager.handleEvent(nil, ev)
		}
	})

	execCtx, execCancel, err := RuntimeBrowserExecutorContext(runtime, 3*time.Second)
	if err != nil {
		cancel()
		manager.waitStopped()
		return nil, err
	}
	defer execCancel()
	if err := targetpkg.SetDiscoverTargets(true).Do(execCtx); err != nil {
		cancel()
		manager.waitStopped()
		return nil, err
	}
	infos, err := targetpkg.GetTargets().Do(execCtx)
	if err != nil {
		cancel()
		manager.waitStopped()
		return nil, err
	}
	for _, info := range infos {
		manager.handleTargetInfo(nil, TargetEventInfoChanged, info)
	}
	return manager, nil
}

func WatchPageTargets(runtime *Runtime, handler func(TargetEvent)) (*PageTargetWatcher, error) {
	if runtime == nil {
		return nil, errors.New("browser runtime unavailable")
	}
	manager := runtime.TargetManager()
	if manager == nil {
		return nil, errors.New("browser target manager unavailable")
	}
	return manager.Watch(handler), nil
}

func (watcher *PageTargetWatcher) Stop() {
	if watcher == nil {
		return
	}
	if watcher.manager != nil {
		watcher.manager.removeListener(watcher.listenerID)
		return
	}
	if watcher.cancel == nil {
		return
	}
	watcher.cancel()
}

func (watcher *PageTargetWatcher) RememberTargetSession(targetID string, sessionID string) {
	if watcher == nil {
		return
	}
	targetID = strings.TrimSpace(targetID)
	sessionID = strings.TrimSpace(sessionID)
	if targetID == "" || sessionID == "" {
		return
	}
	if watcher.manager != nil {
		watcher.manager.RememberTargetSession(targetID, sessionID)
		return
	}
	watcher.mu.Lock()
	if watcher.targetBySession == nil {
		watcher.targetBySession = map[string]string{}
	}
	watcher.targetBySession[sessionID] = targetID
	watcher.mu.Unlock()
}

func (manager *PageTargetManager) Stop() {
	if manager == nil || manager.cancel == nil {
		return
	}
	manager.cancel()
	manager.waitStopped()
}

func (manager *PageTargetManager) Watch(handler func(TargetEvent)) *PageTargetWatcher {
	if manager == nil {
		return &PageTargetWatcher{}
	}
	listenerID := manager.addListener(handler)
	for _, info := range manager.ListPageTargets() {
		emitTargetEvent(handler, TargetEvent{
			Kind:     TargetEventInfoChanged,
			TargetID: strings.TrimSpace(string(info.TargetID)),
			Info:     clonePageTargetInfo(info),
		})
	}
	return &PageTargetWatcher{
		manager:    manager,
		listenerID: listenerID,
	}
}

func (manager *PageTargetManager) ListPageTargets() []*targetpkg.Info {
	if manager == nil {
		return nil
	}
	manager.mu.RLock()
	result := make([]*targetpkg.Info, 0, len(manager.targets))
	for targetID, info := range manager.targets {
		if _, excluded := manager.excludedTargets[targetID]; excluded || !isPageTargetInfo(info) {
			continue
		}
		result = append(result, clonePageTargetInfo(info))
	}
	manager.mu.RUnlock()
	sort.SliceStable(result, func(left int, right int) bool {
		return strings.TrimSpace(string(result[left].TargetID)) < strings.TrimSpace(string(result[right].TargetID))
	})
	return result
}

func (manager *PageTargetManager) PageTargetMap() map[string]*targetpkg.Info {
	if manager == nil {
		return nil
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	result := make(map[string]*targetpkg.Info, len(manager.targets))
	for targetID, info := range manager.targets {
		if _, excluded := manager.excludedTargets[targetID]; excluded || !isPageTargetInfo(info) {
			continue
		}
		result[targetID] = clonePageTargetInfo(info)
	}
	return result
}

func (manager *PageTargetManager) PageTargetInfo(targetID string) (*targetpkg.Info, bool) {
	if manager == nil {
		return nil, false
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil, false
	}
	manager.mu.RLock()
	info, ok := manager.targets[targetID]
	_, excluded := manager.excludedTargets[targetID]
	manager.mu.RUnlock()
	if !ok || excluded || !isPageTargetInfo(info) {
		return nil, false
	}
	return clonePageTargetInfo(info), true
}

func (manager *PageTargetManager) PageTargetExists(targetID string) bool {
	_, ok := manager.PageTargetInfo(targetID)
	return ok
}

func (manager *PageTargetManager) RememberPageTargetID(targetID string) {
	if manager == nil {
		return
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return
	}
	manager.recordTargetInfo(&targetpkg.Info{
		TargetID: targetpkg.ID(targetID),
		Type:     "page",
	})
}

// ExcludeTargetID reserves an internal page target so it can never be
// surfaced to tab consumers, even if Chromium later changes its URL.
func (manager *PageTargetManager) ExcludeTargetID(targetID string) {
	if manager == nil {
		return
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return
	}
	manager.mu.Lock()
	if manager.excludedTargets == nil {
		manager.excludedTargets = map[string]struct{}{}
	}
	manager.excludedTargets[targetID] = struct{}{}
	delete(manager.targets, targetID)
	for sessionID, mappedTargetID := range manager.targetBySession {
		if mappedTargetID == targetID {
			delete(manager.targetBySession, sessionID)
		}
	}
	manager.mu.Unlock()
}

func (manager *PageTargetManager) RememberTargetSession(targetID string, sessionID string) {
	if manager == nil {
		return
	}
	targetID = strings.TrimSpace(targetID)
	sessionID = strings.TrimSpace(sessionID)
	if targetID == "" || sessionID == "" {
		return
	}
	manager.mu.Lock()
	if _, excluded := manager.excludedTargets[targetID]; excluded {
		manager.mu.Unlock()
		return
	}
	if manager.targetBySession == nil {
		manager.targetBySession = map[string]string{}
	}
	manager.targetBySession[sessionID] = targetID
	manager.mu.Unlock()
}

func (manager *PageTargetManager) WaitPageTarget(ctx context.Context, predicate func(*targetpkg.Info) bool) (*targetpkg.Info, error) {
	if manager == nil {
		return nil, errors.New("browser target manager unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	manager.mu.Lock()
	if manager.targets == nil {
		manager.targets = map[string]*targetpkg.Info{}
	}
	if manager.waiters == nil {
		manager.waiters = map[uint64]pageTargetWaiter{}
	}
	for _, info := range manager.targets {
		if matchPageTargetPredicate(predicate, info) {
			result := clonePageTargetInfo(info)
			manager.mu.Unlock()
			return result, nil
		}
	}
	manager.nextWaiterID++
	waiterID := manager.nextWaiterID
	ch := make(chan *targetpkg.Info, 1)
	manager.waiters[waiterID] = pageTargetWaiter{predicate: predicate, ch: ch}
	manager.mu.Unlock()

	select {
	case info := <-ch:
		return clonePageTargetInfo(info), nil
	case <-manager.done:
		manager.mu.Lock()
		delete(manager.waiters, waiterID)
		manager.mu.Unlock()
		return nil, context.Canceled
	case <-ctx.Done():
		manager.mu.Lock()
		delete(manager.waiters, waiterID)
		manager.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (manager *PageTargetManager) run(ctx context.Context) {
	defer close(manager.done)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-manager.events:
			manager.handleEvent(nil, ev)
		}
	}
}

func (manager *PageTargetManager) waitStopped() {
	if manager == nil || manager.done == nil {
		return
	}
	select {
	case <-manager.done:
	case <-time.After(time.Second):
	}
}

func (manager *PageTargetManager) addListener(handler func(TargetEvent)) uint64 {
	if manager == nil || handler == nil {
		return 0
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.nextListenerID++
	listenerID := manager.nextListenerID
	manager.listeners[listenerID] = handler
	return listenerID
}

func (manager *PageTargetManager) removeListener(listenerID uint64) {
	if manager == nil || listenerID == 0 {
		return
	}
	manager.mu.Lock()
	delete(manager.listeners, listenerID)
	manager.mu.Unlock()
}

func (watcher *PageTargetWatcher) handleEvent(handler func(TargetEvent), ev any) {
	switch event := ev.(type) {
	case *targetpkg.EventTargetCreated:
		watcher.handleTargetInfo(handler, TargetEventCreated, event.TargetInfo)
	case *targetpkg.EventTargetInfoChanged:
		watcher.handleTargetInfo(handler, TargetEventInfoChanged, event.TargetInfo)
	case *targetpkg.EventAttachedToTarget:
		if isManagedNetworkProbeTargetInfo(event.TargetInfo) {
			watcher.excludeTargetID(strings.TrimSpace(string(event.TargetInfo.TargetID)))
			return
		}
		if !isPageTargetInfo(event.TargetInfo) {
			return
		}
		targetID := strings.TrimSpace(string(event.TargetInfo.TargetID))
		if watcher.isExcludedTargetID(targetID) {
			return
		}
		sessionID := strings.TrimSpace(string(event.SessionID))
		watcher.RememberTargetSession(targetID, sessionID)
		emitTargetEvent(handler, TargetEvent{
			Kind:      TargetEventAttached,
			TargetID:  targetID,
			SessionID: sessionID,
			Info:      clonePageTargetInfo(event.TargetInfo),
		})
	case *targetpkg.EventDetachedFromTarget:
		sessionID := strings.TrimSpace(string(event.SessionID))
		targetID := watcher.forgetTargetSession(sessionID)
		if targetID == "" || watcher.isExcludedTargetID(targetID) {
			return
		}
		emitTargetEvent(handler, TargetEvent{
			Kind:      TargetEventDetached,
			TargetID:  targetID,
			SessionID: sessionID,
		})
	case *targetpkg.EventTargetDestroyed:
		targetID := strings.TrimSpace(string(event.TargetID))
		if watcher.consumeExcludedTargetID(targetID) {
			return
		}
		emitTargetEvent(handler, TargetEvent{
			Kind:     TargetEventDestroyed,
			TargetID: targetID,
		})
	case *targetpkg.EventTargetCrashed:
		targetID := strings.TrimSpace(string(event.TargetID))
		if watcher.consumeExcludedTargetID(targetID) {
			return
		}
		emitTargetEvent(handler, TargetEvent{
			Kind:      TargetEventCrashed,
			TargetID:  targetID,
			Status:    strings.TrimSpace(event.Status),
			ErrorCode: event.ErrorCode,
		})
	}
}

func (manager *PageTargetManager) handleEvent(handler func(TargetEvent), ev any) {
	switch event := ev.(type) {
	case *targetpkg.EventTargetCreated:
		manager.handleTargetInfo(handler, TargetEventCreated, event.TargetInfo)
	case *targetpkg.EventTargetInfoChanged:
		manager.handleTargetInfo(handler, TargetEventInfoChanged, event.TargetInfo)
	case *targetpkg.EventAttachedToTarget:
		if isManagedNetworkProbeTargetInfo(event.TargetInfo) {
			manager.ExcludeTargetID(strings.TrimSpace(string(event.TargetInfo.TargetID)))
			return
		}
		if !isPageTargetInfo(event.TargetInfo) {
			return
		}
		targetID := strings.TrimSpace(string(event.TargetInfo.TargetID))
		if manager.isExcludedTargetID(targetID) {
			return
		}
		sessionID := strings.TrimSpace(string(event.SessionID))
		manager.RememberTargetSession(targetID, sessionID)
		manager.handleTargetInfo(nil, TargetEventInfoChanged, event.TargetInfo)
		manager.emit(TargetEvent{
			Kind:      TargetEventAttached,
			TargetID:  targetID,
			SessionID: sessionID,
			Info:      clonePageTargetInfo(event.TargetInfo),
		})
		emitTargetEvent(handler, TargetEvent{
			Kind:      TargetEventAttached,
			TargetID:  targetID,
			SessionID: sessionID,
			Info:      clonePageTargetInfo(event.TargetInfo),
		})
	case *targetpkg.EventDetachedFromTarget:
		sessionID := strings.TrimSpace(string(event.SessionID))
		targetID := manager.forgetTargetSession(sessionID)
		if targetID == "" || manager.isExcludedTargetID(targetID) {
			return
		}
		manager.emit(TargetEvent{
			Kind:      TargetEventDetached,
			TargetID:  targetID,
			SessionID: sessionID,
		})
		emitTargetEvent(handler, TargetEvent{
			Kind:      TargetEventDetached,
			TargetID:  targetID,
			SessionID: sessionID,
		})
	case *targetpkg.EventTargetDestroyed:
		targetID := strings.TrimSpace(string(event.TargetID))
		if manager.consumeExcludedTargetID(targetID) {
			return
		}
		manager.removeTarget(targetID)
		manager.emit(TargetEvent{
			Kind:     TargetEventDestroyed,
			TargetID: targetID,
		})
		emitTargetEvent(handler, TargetEvent{
			Kind:     TargetEventDestroyed,
			TargetID: targetID,
		})
	case *targetpkg.EventTargetCrashed:
		targetID := strings.TrimSpace(string(event.TargetID))
		if manager.consumeExcludedTargetID(targetID) {
			return
		}
		manager.removeTarget(targetID)
		manager.emit(TargetEvent{
			Kind:      TargetEventCrashed,
			TargetID:  targetID,
			Status:    strings.TrimSpace(event.Status),
			ErrorCode: event.ErrorCode,
		})
		emitTargetEvent(handler, TargetEvent{
			Kind:      TargetEventCrashed,
			TargetID:  targetID,
			Status:    strings.TrimSpace(event.Status),
			ErrorCode: event.ErrorCode,
		})
	}
}

func (watcher *PageTargetWatcher) handleTargetInfo(handler func(TargetEvent), kind TargetEventKind, info *targetpkg.Info) {
	if isManagedNetworkProbeTargetInfo(info) {
		watcher.excludeTargetID(strings.TrimSpace(string(info.TargetID)))
		return
	}
	if !isPageTargetInfo(info) {
		return
	}
	if watcher.isExcludedTargetID(strings.TrimSpace(string(info.TargetID))) {
		return
	}
	emitTargetEvent(handler, TargetEvent{
		Kind:     kind,
		TargetID: strings.TrimSpace(string(info.TargetID)),
		Info:     clonePageTargetInfo(info),
	})
}

func (manager *PageTargetManager) handleTargetInfo(handler func(TargetEvent), kind TargetEventKind, info *targetpkg.Info) {
	if isManagedNetworkProbeTargetInfo(info) {
		manager.ExcludeTargetID(strings.TrimSpace(string(info.TargetID)))
		return
	}
	if !isPageTargetInfo(info) {
		return
	}
	targetID := strings.TrimSpace(string(info.TargetID))
	cloned := clonePageTargetInfo(info)
	waiters, recorded := manager.recordTargetInfo(cloned)
	if !recorded {
		return
	}
	event := TargetEvent{
		Kind:     kind,
		TargetID: targetID,
		Info:     clonePageTargetInfo(cloned),
	}
	for _, waiter := range waiters {
		select {
		case waiter <- clonePageTargetInfo(cloned):
		default:
		}
	}
	manager.emit(event)
	emitTargetEvent(handler, event)
}

func (manager *PageTargetManager) recordTargetInfo(info *targetpkg.Info) ([]chan *targetpkg.Info, bool) {
	if manager == nil || !isPageTargetInfo(info) {
		return nil, false
	}
	targetID := strings.TrimSpace(string(info.TargetID))
	manager.mu.Lock()
	if _, excluded := manager.excludedTargets[targetID]; excluded {
		manager.mu.Unlock()
		return nil, false
	}
	if manager.targets == nil {
		manager.targets = map[string]*targetpkg.Info{}
	}
	manager.targets[targetID] = clonePageTargetInfo(info)
	waiters := make([]chan *targetpkg.Info, 0)
	for waiterID, waiter := range manager.waiters {
		if !matchPageTargetPredicate(waiter.predicate, info) {
			continue
		}
		waiters = append(waiters, waiter.ch)
		delete(manager.waiters, waiterID)
	}
	manager.mu.Unlock()
	return waiters, true
}

func (watcher *PageTargetWatcher) excludeTargetID(targetID string) {
	if watcher == nil {
		return
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return
	}
	watcher.mu.Lock()
	if watcher.excludedTargetIDs == nil {
		watcher.excludedTargetIDs = map[string]struct{}{}
	}
	watcher.excludedTargetIDs[targetID] = struct{}{}
	for sessionID, mappedTargetID := range watcher.targetBySession {
		if mappedTargetID == targetID {
			delete(watcher.targetBySession, sessionID)
		}
	}
	watcher.mu.Unlock()
}

func (watcher *PageTargetWatcher) consumeExcludedTargetID(targetID string) bool {
	if watcher == nil {
		return false
	}
	targetID = strings.TrimSpace(targetID)
	watcher.mu.Lock()
	_, excluded := watcher.excludedTargetIDs[targetID]
	delete(watcher.excludedTargetIDs, targetID)
	watcher.mu.Unlock()
	return excluded
}

func (watcher *PageTargetWatcher) isExcludedTargetID(targetID string) bool {
	if watcher == nil {
		return false
	}
	targetID = strings.TrimSpace(targetID)
	watcher.mu.Lock()
	_, excluded := watcher.excludedTargetIDs[targetID]
	watcher.mu.Unlock()
	return excluded
}

func (manager *PageTargetManager) isExcludedTargetID(targetID string) bool {
	if manager == nil {
		return false
	}
	targetID = strings.TrimSpace(targetID)
	manager.mu.RLock()
	_, excluded := manager.excludedTargets[targetID]
	manager.mu.RUnlock()
	return excluded
}

func (manager *PageTargetManager) consumeExcludedTargetID(targetID string) bool {
	if manager == nil {
		return false
	}
	targetID = strings.TrimSpace(targetID)
	manager.mu.Lock()
	_, excluded := manager.excludedTargets[targetID]
	if excluded {
		delete(manager.excludedTargets, targetID)
		delete(manager.targets, targetID)
		for sessionID, mappedTargetID := range manager.targetBySession {
			if mappedTargetID == targetID {
				delete(manager.targetBySession, sessionID)
			}
		}
	}
	manager.mu.Unlock()
	return excluded
}

func (manager *PageTargetManager) removeTarget(targetID string) {
	if manager == nil {
		return
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return
	}
	manager.mu.Lock()
	delete(manager.targets, targetID)
	for sessionID, mappedTargetID := range manager.targetBySession {
		if mappedTargetID == targetID {
			delete(manager.targetBySession, sessionID)
		}
	}
	manager.mu.Unlock()
}

func (watcher *PageTargetWatcher) forgetTargetSession(sessionID string) string {
	if watcher == nil {
		return ""
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	targetID := watcher.targetBySession[sessionID]
	delete(watcher.targetBySession, sessionID)
	return targetID
}

func (manager *PageTargetManager) forgetTargetSession(sessionID string) string {
	if manager == nil {
		return ""
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	targetID := manager.targetBySession[sessionID]
	delete(manager.targetBySession, sessionID)
	return targetID
}

func (manager *PageTargetManager) emit(event TargetEvent) {
	if manager == nil {
		return
	}
	manager.mu.RLock()
	listeners := make([]func(TargetEvent), 0, len(manager.listeners))
	for _, listener := range manager.listeners {
		listeners = append(listeners, listener)
	}
	manager.mu.RUnlock()
	for _, listener := range listeners {
		emitTargetEvent(listener, event)
	}
}

func TargetSessionIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	chromeCtx := chromedp.FromContext(ctx)
	if chromeCtx == nil || chromeCtx.Target == nil {
		return ""
	}
	return strings.TrimSpace(string(chromeCtx.Target.SessionID))
}

func emitTargetEvent(handler func(TargetEvent), event TargetEvent) {
	if handler == nil {
		return
	}
	handler(event)
}

func matchPageTargetPredicate(predicate func(*targetpkg.Info) bool, info *targetpkg.Info) bool {
	if !isPageTargetInfo(info) {
		return false
	}
	return predicate == nil || predicate(clonePageTargetInfo(info))
}

func isPageTargetInfo(info *targetpkg.Info) bool {
	return info != nil && info.Type == "page" && strings.TrimSpace(string(info.TargetID)) != "" && !isManagedNetworkProbeTargetInfo(info)
}

func clonePageTargetInfo(info *targetpkg.Info) *targetpkg.Info {
	if info == nil {
		return nil
	}
	cloned := *info
	return &cloned
}
