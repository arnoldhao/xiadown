//go:build windows

package wails

import (
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"github.com/wailsapp/wails/webview2/pkg/edge"
)

type listenWindowsFakeChromiumCallbacks struct {
	WebResourceRequestedCallback             func(*edge.ICoreWebView2WebResourceRequest, *edge.ICoreWebView2WebResourceRequestedEventArgs)
	ContainsFullScreenElementChangedCallback func(*edge.ICoreWebView2, *edge.ICoreWebView2ContainsFullScreenElementChangedEventArgs)
}

// These deliberately distinct pointer types model Wails' internal WebView2
// package, whose Go type identity differs from the standalone edge package.
type listenWindowsPrivateRequest struct{ marker uintptr }
type listenWindowsPrivateRequestArgs struct{ marker uintptr }

type listenWindowsFakePrivateChromiumCallbacks struct {
	WebResourceRequestedCallback func(*listenWindowsPrivateRequest, *listenWindowsPrivateRequestArgs)
}

func TestListenWindowsWebViewBridgeCopiesAndChainsPrivateCallbacks(t *testing.T) {
	var requestOrder []string
	fake := &listenWindowsFakeChromiumCallbacks{
		WebResourceRequestedCallback: func(*edge.ICoreWebView2WebResourceRequest, *edge.ICoreWebView2WebResourceRequestedEventArgs) {
			requestOrder = append(requestOrder, "wails")
		},
	}
	bridge := &listenWindowsWebViewBridge{chromium: reflect.ValueOf(fake).Elem()}
	if !bridge.WrapWebResourceRequested(func(*edge.ICoreWebView2WebResourceRequest) {
		requestOrder = append(requestOrder, "xiadown")
	}) {
		t.Fatal("failed to wrap WebResourceRequested callback")
	}
	runtime.GC()
	fake.WebResourceRequestedCallback(
		&edge.ICoreWebView2WebResourceRequest{},
		&edge.ICoreWebView2WebResourceRequestedEventArgs{},
	)
	if got, want := requestOrder, []string{"wails", "xiadown"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("callback order = %v, want %v", got, want)
	}

	requestOrder = nil
	if !bridge.WrapWebResourceRequested(func(*edge.ICoreWebView2WebResourceRequest) {
		requestOrder = append(requestOrder, "second")
	}) {
		t.Fatal("failed to install second WebResourceRequested wrapper")
	}
	runtime.GC()
	fake.WebResourceRequestedCallback(
		&edge.ICoreWebView2WebResourceRequest{},
		&edge.ICoreWebView2WebResourceRequestedEventArgs{},
	)
	if got, want := requestOrder, []string{"wails", "xiadown", "second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nested callback order = %v, want %v", got, want)
	}

	requestOrder = nil
	fake.WebResourceRequestedCallback(&edge.ICoreWebView2WebResourceRequest{}, nil)
	if len(requestOrder) != 0 {
		t.Fatalf("invalid callback arguments reached chained callbacks: %v", requestOrder)
	}

	var fullscreenFallbacks int
	fake.ContainsFullScreenElementChangedCallback = func(*edge.ICoreWebView2, *edge.ICoreWebView2ContainsFullScreenElementChangedEventArgs) {
		fullscreenFallbacks++
	}
	if !bridge.WrapContainsFullScreenElementChanged(func(*edge.ICoreWebView2) bool { return true }) {
		t.Fatal("failed to wrap fullscreen callback")
	}
	fake.ContainsFullScreenElementChangedCallback(
		&edge.ICoreWebView2{},
		nil,
	)
	if fullscreenFallbacks != 0 {
		t.Fatalf("embedded fullscreen unexpectedly invoked Wails fallback %d times", fullscreenFallbacks)
	}
}

func TestListenWindowsWebViewBridgeFullscreenFallsBackWhenUnhandled(t *testing.T) {
	var fallbacks int
	fake := &listenWindowsFakeChromiumCallbacks{
		ContainsFullScreenElementChangedCallback: func(*edge.ICoreWebView2, *edge.ICoreWebView2ContainsFullScreenElementChangedEventArgs) {
			fallbacks++
		},
	}
	bridge := &listenWindowsWebViewBridge{chromium: reflect.ValueOf(fake).Elem()}
	if !bridge.WrapContainsFullScreenElementChanged(func(*edge.ICoreWebView2) bool { return false }) {
		t.Fatal("failed to wrap fullscreen callback")
	}
	fake.ContainsFullScreenElementChangedCallback(
		&edge.ICoreWebView2{},
		nil,
	)
	if fallbacks != 1 {
		t.Fatalf("Wails fallback count = %d, want 1", fallbacks)
	}
}

func TestListenWindowsWebViewBridgeAdaptsPrivateCallbackPointerTypes(t *testing.T) {
	request := &listenWindowsPrivateRequest{marker: 42}
	args := &listenWindowsPrivateRequestArgs{marker: 7}
	var originalRequest unsafe.Pointer
	var adaptedRequest unsafe.Pointer
	fake := &listenWindowsFakePrivateChromiumCallbacks{
		WebResourceRequestedCallback: func(got *listenWindowsPrivateRequest, _ *listenWindowsPrivateRequestArgs) {
			originalRequest = unsafe.Pointer(got)
		},
	}
	bridge := &listenWindowsWebViewBridge{chromium: reflect.ValueOf(fake).Elem()}
	if !bridge.WrapWebResourceRequested(func(got *edge.ICoreWebView2WebResourceRequest) {
		adaptedRequest = unsafe.Pointer(got)
	}) {
		t.Fatal("failed to wrap private WebResourceRequested callback type")
	}
	runtime.GC()
	fake.WebResourceRequestedCallback(request, args)
	if originalRequest != unsafe.Pointer(request) || adaptedRequest != unsafe.Pointer(request) {
		t.Fatalf("private callback pointer changed: original=%p adapted=%p want=%p", originalRequest, adaptedRequest, request)
	}
}

func TestListenWindowsWebViewBridgeContainsCallbackPanics(t *testing.T) {
	var chained int
	fake := &listenWindowsFakeChromiumCallbacks{
		WebResourceRequestedCallback: func(*edge.ICoreWebView2WebResourceRequest, *edge.ICoreWebView2WebResourceRequestedEventArgs) {
			chained++
		},
	}
	bridge := &listenWindowsWebViewBridge{chromium: reflect.ValueOf(fake).Elem()}
	if !bridge.WrapWebResourceRequested(func(*edge.ICoreWebView2WebResourceRequest) {
		panic("callback failure")
	}) {
		t.Fatal("failed to wrap WebResourceRequested callback")
	}
	// reflect.MakeFunc callbacks must never unwind through the native WebView2
	// trampoline, even when the application callback fails.
	fake.WebResourceRequestedCallback(
		&edge.ICoreWebView2WebResourceRequest{},
		&edge.ICoreWebView2WebResourceRequestedEventArgs{},
	)
	if chained != 1 {
		t.Fatalf("Wails callback count = %d, want 1", chained)
	}
}

func TestListenWindowsReflectedCOMPointerReturnsInterfacePointer(t *testing.T) {
	core := &edge.ICoreWebView2{}
	holder := &struct {
		core *edge.ICoreWebView2
	}{core: core}
	field := reflect.ValueOf(holder).Elem().FieldByName("core")
	got := listenWindowsReflectedCOMPointer(field)
	if got != unsafe.Pointer(core) {
		t.Fatalf("raw COM pointer = %p, want %p", got, core)
	}
}
