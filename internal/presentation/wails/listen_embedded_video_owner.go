package wails

import (
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var listenEmbeddedVideoOwner = struct {
	mu    sync.Mutex
	owner string
}{}

type listenNativeWindowFeatureGuard struct {
	windowIDs sync.Map
}

func (guard *listenNativeWindowFeatureGuard) Claim(windowID uint) bool {
	if guard == nil || windowID == 0 {
		return false
	}
	_, loaded := guard.windowIDs.LoadOrStore(windowID, struct{}{})
	return !loaded
}

func (guard *listenNativeWindowFeatureGuard) Release(windowID uint) {
	if guard == nil || windowID == 0 {
		return
	}
	guard.windowIDs.Delete(windowID)
}

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

func listenShowNativeEmbeddedWebViewForOwner(owner string, playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if owner == "" {
		return false
	}
	listenEmbeddedVideoOwner.mu.Lock()
	defer listenEmbeddedVideoOwner.mu.Unlock()
	if listenEmbeddedVideoOwner.owner != owner {
		return false
	}
	return showListenNativeEmbeddedWebViewWindow(playerWindow, hostWindow, rect)
}

func rssShowNativeEmbeddedWebViewForOwner(owner string, playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if owner == "" {
		return false
	}
	listenEmbeddedVideoOwner.mu.Lock()
	defer listenEmbeddedVideoOwner.mu.Unlock()
	if listenEmbeddedVideoOwner.owner != owner {
		return false
	}
	return showRSSNativeEmbeddedWebViewWindow(playerWindow, hostWindow, rect)
}

func rssShowNativeInteractiveEmbeddedWebViewForOwner(owner string, playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if owner == "" {
		return false
	}
	listenEmbeddedVideoOwner.mu.Lock()
	defer listenEmbeddedVideoOwner.mu.Unlock()
	if listenEmbeddedVideoOwner.owner != owner {
		return false
	}
	rect.Interactive = true
	return showRSSNativeInteractiveEmbeddedWebViewWindow(playerWindow, hostWindow, rect)
}

// listenEmbeddedVideoRevealReady treats the DOM resize acknowledgement as an
// advisory handoff signal. Once the native WebView is mounted under the active
// host, keeping the React surface opaque after a delayed or rounded-size ACK
// would hide a valid video surface indefinitely.
func listenEmbeddedVideoRevealReady(nativeShown bool, _ bool, ownerActive bool) bool {
	return nativeShown && ownerActive
}
