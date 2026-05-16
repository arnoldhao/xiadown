package wails

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var listenEmbeddedVideoOwner = struct {
	mu    sync.Mutex
	owner string
}{}

func listenEmbeddedVideoOwnerID(window *application.WebviewWindow) string {
	if window == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", window.Name(), window.ID())
}

func listenClaimEmbeddedVideoOwner(window *application.WebviewWindow) string {
	owner := listenEmbeddedVideoOwnerID(window)
	if owner == "" {
		return ""
	}
	listenEmbeddedVideoOwner.mu.Lock()
	listenEmbeddedVideoOwner.owner = owner
	listenEmbeddedVideoOwner.mu.Unlock()
	return owner
}

func listenEmbeddedVideoOwnerActive(owner string) bool {
	if owner == "" {
		return false
	}
	listenEmbeddedVideoOwner.mu.Lock()
	defer listenEmbeddedVideoOwner.mu.Unlock()
	return listenEmbeddedVideoOwner.owner == owner
}

func listenReleaseEmbeddedVideoOwner(owner string) bool {
	if owner == "" {
		return false
	}
	listenEmbeddedVideoOwner.mu.Lock()
	defer listenEmbeddedVideoOwner.mu.Unlock()
	if listenEmbeddedVideoOwner.owner != owner {
		return false
	}
	listenEmbeddedVideoOwner.owner = ""
	return true
}

func listenShowNativeEmbeddedWebViewForOwner(owner string, playerNativeWindow unsafe.Pointer, hostNativeWindow unsafe.Pointer, rect ListenEmbeddedVideoRect) bool {
	if owner == "" {
		return false
	}
	listenEmbeddedVideoOwner.mu.Lock()
	defer listenEmbeddedVideoOwner.mu.Unlock()
	if listenEmbeddedVideoOwner.owner != owner {
		return false
	}
	return showListenNativeEmbeddedWebView(playerNativeWindow, hostNativeWindow, rect)
}
