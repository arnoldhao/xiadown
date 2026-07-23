//go:build windows

package wails

import (
	"context"
	"errors"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestConnectorWindowsDevToolsCompletionKeepsNativeRootUntilFinalRelease(t *testing.T) {
	handler := newConnectorWindowsDevToolsCompletedHandler()
	connectorWindowsTrackDevToolsHandler(handler)
	key := uintptr(unsafe.Pointer(handler))
	t.Cleanup(func() { connectorWindowsDevToolsPending.Delete(key) })

	if got := connectorWindowsDevToolsCompletedHandlerAddRef(handler); got != 2 {
		t.Fatalf("native AddRef = %d, want 2", got)
	}
	if got := connectorWindowsDevToolsCompletedHandlerRelease(handler); got != 1 {
		t.Fatalf("creator Release = %d, want 1", got)
	}
	result, err := windows.UTF16PtrFromString(`{"ok":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := connectorWindowsDevToolsCompletedHandlerInvoke(handler, 0, result); got != uintptr(windows.S_OK) {
		t.Fatalf("Invoke HRESULT = %#x", got)
	}
	// A duplicate native callback must not overwrite the first publication or
	// close the completion channel twice.
	connectorWindowsDevToolsCompletedHandlerInvoke(handler, uintptr(syscall.ERROR_ACCESS_DENIED), nil)
	if handler.result != `{"ok":true}` || handler.err != nil {
		t.Fatalf("published result = %q, err = %v", handler.result, handler.err)
	}
	handler = nil
	runtime.GC()
	value, rooted := connectorWindowsDevToolsPending.Load(key)
	if !rooted {
		t.Fatal("handler was unrooted before WebView2 released its native reference")
	}
	rootedHandler, ok := value.(*connectorWindowsDevToolsCompletedHandler)
	if !ok || rootedHandler == nil {
		t.Fatalf("rooted handler has type %T", value)
	}
	if got := connectorWindowsDevToolsCompletedHandlerRelease(rootedHandler); got != 0 {
		t.Fatalf("native Release = %d, want 0", got)
	}
	if _, rooted := connectorWindowsDevToolsPending.Load(key); rooted {
		t.Fatal("handler remained rooted after its final native Release")
	}
}

func TestConnectorWindowsCookieCompletionPublishesOnceAndUntracksAtZero(t *testing.T) {
	handler := newConnectorWindowsGetCookiesCompletedHandler(nil, nil)
	connectorWindowsTrackGetCookiesHandler(handler)
	key := uintptr(unsafe.Pointer(handler))
	t.Cleanup(func() { connectorWindowsGetCookiesPending.Delete(key) })

	if got := connectorWindowsGetCookiesCompletedHandlerAddRef(handler); got != 2 {
		t.Fatalf("native AddRef = %d, want 2", got)
	}
	if got := connectorWindowsGetCookiesCompletedHandlerRelease(handler); got != 1 {
		t.Fatalf("creator Release = %d, want 1", got)
	}
	connectorWindowsGetCookiesCompletedHandlerInvoke(handler, uintptr(syscall.ERROR_ACCESS_DENIED), nil)
	connectorWindowsGetCookiesCompletedHandlerInvoke(handler, 0, nil)
	if handler.err != syscall.ERROR_ACCESS_DENIED {
		t.Fatalf("published error = %v, want %v", handler.err, syscall.ERROR_ACCESS_DENIED)
	}
	handler = nil
	runtime.GC()
	value, rooted := connectorWindowsGetCookiesPending.Load(key)
	if !rooted {
		t.Fatal("handler was unrooted before WebView2 released its native reference")
	}
	rootedHandler, ok := value.(*connectorWindowsGetCookiesCompletedHandler)
	if !ok || rootedHandler == nil {
		t.Fatalf("rooted handler has type %T", value)
	}
	if got := connectorWindowsGetCookiesCompletedHandlerRelease(rootedHandler); got != 0 {
		t.Fatalf("native Release = %d, want 0", got)
	}
	if _, rooted := connectorWindowsGetCookiesPending.Load(key); rooted {
		t.Fatal("handler remained rooted after its final native Release")
	}
}

func TestConnectorWindowsUnsubmittedCompletionHandlersUntrackAtCreatorRelease(t *testing.T) {
	devTools := newConnectorWindowsDevToolsCompletedHandler()
	connectorWindowsTrackDevToolsHandler(devTools)
	devToolsKey := uintptr(unsafe.Pointer(devTools))
	if got := connectorWindowsDevToolsCompletedHandlerRelease(devTools); got != 0 {
		t.Fatalf("unsubmitted DevTools creator Release = %d, want 0", got)
	}
	if _, rooted := connectorWindowsDevToolsPending.Load(devToolsKey); rooted {
		t.Fatal("unsubmitted DevTools handler remained rooted")
	}

	cookies := newConnectorWindowsGetCookiesCompletedHandler(nil, nil)
	connectorWindowsTrackGetCookiesHandler(cookies)
	cookiesKey := uintptr(unsafe.Pointer(cookies))
	if got := connectorWindowsGetCookiesCompletedHandlerRelease(cookies); got != 0 {
		t.Fatalf("unsubmitted cookies creator Release = %d, want 0", got)
	}
	if _, rooted := connectorWindowsGetCookiesPending.Load(cookiesKey); rooted {
		t.Fatal("unsubmitted cookies handler remained rooted")
	}
}

func TestConnectorWindowsCanceledContextsDoNotQueueCOMWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := readConnectorWindowsWebViewCookiesForURI(ctx, nil, "", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled cookie read error = %v, want context.Canceled", err)
	}
	if _, err := connectorWindowsCallDevToolsProtocolMethod(ctx, nil, "Runtime.evaluate", `{}`); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled DevTools call error = %v, want context.Canceled", err)
	}
}
