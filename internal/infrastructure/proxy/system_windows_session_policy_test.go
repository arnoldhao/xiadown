package proxy

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWindowsAutomaticProxySessionStateSerializesNativeCalls(t *testing.T) {
	t.Parallel()
	var state windowsAutomaticProxySessionState
	var opens atomic.Int32
	var closes atomic.Int32
	openHandle := func() (uintptr, error) {
		opens.Add(1)
		return 73, nil
	}
	closeHandle := func(handle uintptr) {
		if handle != 73 {
			t.Errorf("closed handle = %d, want 73", handle)
		}
		closes.Add(1)
	}

	const workers = 12
	handles := make([]uintptr, workers)
	for i := range handles {
		handle, err := state.acquire(openHandle)
		if err != nil {
			t.Fatal(err)
		}
		handles[i] = handle
	}

	start := make(chan struct{})
	errorsFound := make(chan error, workers)
	var inFlight atomic.Int32
	var maximumInFlight atomic.Int32
	var calls atomic.Int32
	var wait sync.WaitGroup
	for _, handle := range handles {
		handle := handle
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			err := state.serializedCall(handle, func(actual uintptr) error {
				if actual != 73 {
					return errors.New("serialized call received the wrong handle")
				}
				current := inFlight.Add(1)
				for observed := maximumInFlight.Load(); current > observed && !maximumInFlight.CompareAndSwap(observed, current); observed = maximumInFlight.Load() {
				}
				time.Sleep(2 * time.Millisecond)
				calls.Add(1)
				inFlight.Add(-1)
				return nil
			})
			if err != nil {
				errorsFound <- err
			}
			state.release(closeHandle)
		}()
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	state.close(closeHandle)

	if got := opens.Load(); got != 1 {
		t.Fatalf("open calls = %d, want 1", got)
	}
	if got := calls.Load(); got != workers {
		t.Fatalf("native calls = %d, want %d", got, workers)
	}
	if got := maximumInFlight.Load(); got != 1 {
		t.Fatalf("maximum concurrent native calls = %d, want 1", got)
	}
	if got := closes.Load(); got != 1 {
		t.Fatalf("close calls = %d, want 1", got)
	}
}

func TestWindowsAutomaticProxySessionStateCloseDoesNotDeadlockQueuedCalls(t *testing.T) {
	t.Parallel()
	var state windowsAutomaticProxySessionState
	openHandle := func() (uintptr, error) { return 91, nil }
	closedHandle := make(chan uintptr, 1)
	closeHandle := func(handle uintptr) { closedHandle <- handle }

	firstHandle, err := state.acquire(openHandle)
	if err != nil {
		t.Fatal(err)
	}
	secondHandle, err := state.acquire(openHandle)
	if err != nil {
		t.Fatal(err)
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		callErr := state.serializedCall(firstHandle, func(uintptr) error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
		state.release(closeHandle)
		firstDone <- callErr
	}()
	<-firstEntered

	secondAttempted := make(chan struct{})
	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondAttempted)
		callErr := state.serializedCall(secondHandle, func(uintptr) error {
			close(secondEntered)
			return nil
		})
		state.release(closeHandle)
		secondDone <- callErr
	}()
	<-secondAttempted

	closeReturned := make(chan struct{})
	go func() {
		state.close(closeHandle)
		close(closeReturned)
	}()
	select {
	case <-closeReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("Close deadlocked behind a queued native call")
	}
	if _, err := state.acquire(openHandle); err == nil {
		t.Fatal("acquire succeeded after Close")
	}
	select {
	case handle := <-closedHandle:
		t.Fatalf("handle %d closed while acquired calls were active", handle)
	default:
	}

	close(releaseFirst)
	for name, done := range map[string]<-chan error{"first": firstDone, "second": secondDone} {
		select {
		case callErr := <-done:
			if callErr != nil {
				t.Fatalf("%s call failed: %v", name, callErr)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s call did not finish", name)
		}
	}
	select {
	case <-secondEntered:
	default:
		t.Fatal("queued native call never entered")
	}
	select {
	case handle := <-closedHandle:
		if handle != 91 {
			t.Fatalf("closed handle = %d, want 91", handle)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handle was not closed after the final release")
	}

	// Close is idempotent and must not close the same native handle twice.
	state.close(closeHandle)
	select {
	case handle := <-closedHandle:
		t.Fatalf("handle closed twice: %d", handle)
	default:
	}
}
