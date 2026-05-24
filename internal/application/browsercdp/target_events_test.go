package browsercdp

import (
	"context"
	"errors"
	"testing"
	"time"

	targetpkg "github.com/chromedp/cdproto/target"
)

func TestPageTargetWatcherMapsDetachSessionToTarget(t *testing.T) {
	t.Parallel()

	watcher := &PageTargetWatcher{targetBySession: map[string]string{}}
	events := make([]TargetEvent, 0)
	handler := func(event TargetEvent) {
		events = append(events, event)
	}

	watcher.handleEvent(handler, &targetpkg.EventAttachedToTarget{
		SessionID: "session-1",
		TargetInfo: &targetpkg.Info{
			TargetID: "target-1",
			Type:     "page",
			URL:      "https://example.test/",
		},
	})
	watcher.handleEvent(handler, &targetpkg.EventDetachedFromTarget{
		SessionID: "session-1",
	})

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Kind != TargetEventAttached || events[0].TargetID != "target-1" || events[0].SessionID != "session-1" {
		t.Fatalf("unexpected attached event: %#v", events[0])
	}
	if events[1].Kind != TargetEventDetached || events[1].TargetID != "target-1" || events[1].SessionID != "session-1" {
		t.Fatalf("unexpected detached event: %#v", events[1])
	}
}

func TestPageTargetWatcherIgnoresNonPageTargets(t *testing.T) {
	t.Parallel()

	watcher := &PageTargetWatcher{targetBySession: map[string]string{}}
	called := false
	watcher.handleEvent(func(TargetEvent) {
		called = true
	}, &targetpkg.EventTargetCreated{
		TargetInfo: &targetpkg.Info{
			TargetID: "worker-1",
			Type:     "worker",
			URL:      "https://example.test/worker.js",
		},
	})

	if called {
		t.Fatal("expected non-page target event to be ignored")
	}
}

func TestPageTargetManagerTracksPageTargetLifecycle(t *testing.T) {
	t.Parallel()

	manager := &PageTargetManager{
		targets:         map[string]*targetpkg.Info{},
		targetBySession: map[string]string{},
		listeners:       map[uint64]func(TargetEvent){},
		waiters:         map[uint64]pageTargetWaiter{},
	}

	manager.handleEvent(nil, &targetpkg.EventTargetCreated{
		TargetInfo: &targetpkg.Info{
			TargetID: "target-1",
			Type:     "page",
			URL:      "https://example.test/",
		},
	})

	info, ok := manager.PageTargetInfo("target-1")
	if !ok {
		t.Fatal("expected target to be tracked")
	}
	if info.URL != "https://example.test/" {
		t.Fatalf("unexpected target url: %q", info.URL)
	}

	manager.handleEvent(nil, &targetpkg.EventAttachedToTarget{
		SessionID: "session-1",
		TargetInfo: &targetpkg.Info{
			TargetID: "target-1",
			Type:     "page",
			URL:      "https://example.test/",
		},
	})
	manager.handleEvent(nil, &targetpkg.EventDetachedFromTarget{SessionID: "session-1"})

	if !manager.PageTargetExists("target-1") {
		t.Fatal("expected detached target session to leave page target tracked")
	}

	manager.handleEvent(nil, &targetpkg.EventTargetDestroyed{TargetID: "target-1"})

	if manager.PageTargetExists("target-1") {
		t.Fatal("expected destroyed target to be removed")
	}
}

func TestPageTargetManagerWaitsForMatchingTarget(t *testing.T) {
	t.Parallel()

	manager := &PageTargetManager{
		targets:         map[string]*targetpkg.Info{},
		targetBySession: map[string]string{},
		listeners:       map[uint64]func(TargetEvent){},
		waiters:         map[uint64]pageTargetWaiter{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan *targetpkg.Info, 1)
	errs := make(chan error, 1)
	go func() {
		info, err := manager.WaitPageTarget(ctx, func(info *targetpkg.Info) bool {
			return info != nil && info.URL == "https://match.test/"
		})
		if err != nil {
			errs <- err
			return
		}
		result <- info
	}()

	manager.handleEvent(nil, &targetpkg.EventTargetInfoChanged{
		TargetInfo: &targetpkg.Info{
			TargetID: "target-2",
			Type:     "page",
			URL:      "https://match.test/",
		},
	})

	select {
	case err := <-errs:
		t.Fatalf("wait target failed: %v", err)
	case info := <-result:
		if info == nil || info.TargetID != "target-2" {
			t.Fatalf("unexpected wait target: %#v", info)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for target")
	}
}

func TestPageTargetManagerWaitStopsWhenManagerStops(t *testing.T) {
	t.Parallel()

	manager := &PageTargetManager{
		done:            make(chan struct{}),
		targets:         map[string]*targetpkg.Info{},
		targetBySession: map[string]string{},
		listeners:       map[uint64]func(TargetEvent){},
		waiters:         map[uint64]pageTargetWaiter{},
	}
	result := make(chan error, 1)
	go func() {
		_, err := manager.WaitPageTarget(context.Background(), func(info *targetpkg.Info) bool {
			return info != nil && info.URL == "https://never.test/"
		})
		result <- err
	}()

	close(manager.done)

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stopped manager")
	}
}

func TestPageTargetManagerRememberPageTargetID(t *testing.T) {
	t.Parallel()

	manager := &PageTargetManager{
		targets:         map[string]*targetpkg.Info{},
		targetBySession: map[string]string{},
		listeners:       map[uint64]func(TargetEvent){},
		waiters:         map[uint64]pageTargetWaiter{},
	}

	manager.RememberPageTargetID("target-1")

	info, ok := manager.PageTargetInfo("target-1")
	if !ok {
		t.Fatal("expected remembered target to exist")
	}
	if info.Type != "page" {
		t.Fatalf("expected page target type, got %q", info.Type)
	}
}

func TestPageTargetManagerWatchReceivesSnapshotAndStops(t *testing.T) {
	t.Parallel()

	manager := &PageTargetManager{
		targets: map[string]*targetpkg.Info{
			"target-1": {
				TargetID: "target-1",
				Type:     "page",
				URL:      "https://example.test/",
			},
		},
		targetBySession: map[string]string{},
		listeners:       map[uint64]func(TargetEvent){},
		waiters:         map[uint64]pageTargetWaiter{},
	}
	events := make(chan TargetEvent, 4)
	watcher := manager.Watch(func(event TargetEvent) {
		events <- event
	})

	select {
	case event := <-events:
		if event.Kind != TargetEventInfoChanged || event.TargetID != "target-1" {
			t.Fatalf("unexpected snapshot event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for snapshot event")
	}

	watcher.Stop()
	manager.handleEvent(nil, &targetpkg.EventTargetCreated{
		TargetInfo: &targetpkg.Info{
			TargetID: "target-2",
			Type:     "page",
			URL:      "https://later.test/",
		},
	})

	select {
	case event := <-events:
		t.Fatalf("expected stopped watcher to receive no event, got %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPageTargetManagerWatchPreservesEventOrder(t *testing.T) {
	t.Parallel()

	manager := &PageTargetManager{
		targets:         map[string]*targetpkg.Info{},
		targetBySession: map[string]string{},
		listeners:       map[uint64]func(TargetEvent){},
		waiters:         map[uint64]pageTargetWaiter{},
	}
	events := make([]TargetEventKind, 0, 2)
	manager.Watch(func(event TargetEvent) {
		events = append(events, event.Kind)
	})

	manager.handleEvent(nil, &targetpkg.EventTargetCreated{
		TargetInfo: &targetpkg.Info{
			TargetID: "target-1",
			Type:     "page",
		},
	})
	manager.handleEvent(nil, &targetpkg.EventTargetDestroyed{TargetID: "target-1"})

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %#v", len(events), events)
	}
	if events[0] != TargetEventCreated || events[1] != TargetEventDestroyed {
		t.Fatalf("unexpected event order: %#v", events)
	}
}
