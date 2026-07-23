package wails

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	applicationrss "xiadown/internal/application/rss"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	rssSitePlayerWindowName = "rss-site-page-player"
	rssSitePlayerBlankURL   = "about:blank"
	maxRSSSitePrepares      = 64
)

var rssSitePlayerSessionCounter atomic.Uint64

type RSSSitePlayerPrepareRequest struct {
	RequestID uint64 `json:"requestId"`
	URL       string `json:"url"`
}

type RSSSitePlayerPrepareTransactionRequest struct {
	RequestID uint64 `json:"requestId"`
}

type RSSSitePlayerPrepareResponse struct {
	SessionID         string `json:"sessionId"`
	URL               string `json:"url"`
	SiteKey           string `json:"siteKey"`
	CredentialsLoaded bool   `json:"credentialsLoaded"`
}

type RSSSitePlayerSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type RSSSitePlayerShowRequest struct {
	SessionID string                  `json:"sessionId"`
	Rect      ListenEmbeddedVideoRect `json:"rect"`
}

type RSSSitePlayerHideRequest struct {
	SessionID string `json:"sessionId"`
	Sequence  uint64 `json:"sequence"`
}

type rssSitePlayer interface {
	Prepare(context.Context, RSSSitePlayerPrepareRequest) (RSSSitePlayerPrepareResponse, error)
	AcceptPrepare(uint64) error
	CancelPrepare(uint64) error
	Show(string, ListenEmbeddedVideoRect) (bool, error)
	Hide(string, uint64) (bool, error)
	Close(string) error
}

// RSSSitePlayerHandler owns the fallback, site-interactive playback WebView.
// It intentionally has no media bridge: playback, volume and fullscreen are
// operated through the site's own controls inside the native overlay.
type RSSSitePlayerHandler struct {
	player rssSitePlayer
}

func NewRSSSitePlayerHandler(
	app *application.App,
	windows *WindowManager,
	service *applicationrss.SitePlayerService,
) *RSSSitePlayerHandler {
	return &RSSSitePlayerHandler{player: &rssSitePagePlayer{
		app:     app,
		windows: windows,
		service: service,
	}}
}

func (handler *RSSSitePlayerHandler) ServiceName() string {
	return "RSSSitePlayerHandler"
}

func (handler *RSSSitePlayerHandler) Prepare(
	ctx context.Context,
	request RSSSitePlayerPrepareRequest,
) (RSSSitePlayerPrepareResponse, error) {
	if handler == nil || handler.player == nil {
		return RSSSitePlayerPrepareResponse{}, fmt.Errorf("RSS site player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return RSSSitePlayerPrepareResponse{}, err
	}
	return handler.player.Prepare(ctx, request)
}

func (handler *RSSSitePlayerHandler) AcceptPrepare(
	ctx context.Context,
	request RSSSitePlayerPrepareTransactionRequest,
) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS site player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.AcceptPrepare(request.RequestID)
}

func (handler *RSSSitePlayerHandler) CancelPrepare(
	ctx context.Context,
	request RSSSitePlayerPrepareTransactionRequest,
) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS site player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.CancelPrepare(request.RequestID)
}

func (handler *RSSSitePlayerHandler) Show(
	ctx context.Context,
	request RSSSitePlayerShowRequest,
) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("RSS site player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return false, err
	}
	// The fallback page must remain directly operable even if a stale renderer
	// bundle sends the underlay default.
	request.Rect.Interactive = true
	return handler.player.Show(request.SessionID, request.Rect)
}

func (handler *RSSSitePlayerHandler) Hide(
	ctx context.Context,
	request RSSSitePlayerHideRequest,
) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, nil
	}
	if err := contextError(ctx); err != nil {
		return false, err
	}
	return handler.player.Hide(request.SessionID, request.Sequence)
}

func (handler *RSSSitePlayerHandler) Close(
	ctx context.Context,
	request RSSSitePlayerSessionRequest,
) error {
	if handler == nil || handler.player == nil {
		return nil
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.Close(request.SessionID)
}

func (handler *RSSSitePlayerHandler) Shutdown() error {
	if handler == nil || handler.player == nil {
		return nil
	}
	return handler.player.Close("")
}

type rssSitePrepareTicket struct {
	requestID  uint64
	generation uint64
}

// rssSitePrepareCoordinator closes the async RPC cancellation gap: only the
// latest non-cancelled request may create a native window, and CancelPrepare
// can overtake Prepare without leaving a hidden playback window behind.
type rssSitePrepareCoordinator struct {
	mu            sync.Mutex
	generation    uint64
	latest        uint64
	highestIssued uint64
	seen          map[uint64]struct{}
	finished      map[uint64]struct{}
	canceled      map[uint64]struct{}
	committed     map[uint64]string
}

func (coordinator *rssSitePrepareCoordinator) begin(requestID uint64) (rssSitePrepareTicket, error) {
	if requestID == 0 {
		return rssSitePrepareTicket{}, fmt.Errorf("RSS site prepare request id is required")
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	if requestID < coordinator.highestIssued {
		coordinator.finished[requestID] = struct{}{}
		coordinator.pruneLocked()
		return rssSitePrepareTicket{}, context.Canceled
	}
	if requestID > coordinator.highestIssued {
		coordinator.highestIssued = requestID
	}
	if _, done := coordinator.finished[requestID]; done {
		return rssSitePrepareTicket{}, context.Canceled
	}
	if _, duplicate := coordinator.seen[requestID]; duplicate {
		return rssSitePrepareTicket{}, context.Canceled
	}
	if _, canceled := coordinator.canceled[requestID]; canceled {
		delete(coordinator.canceled, requestID)
		coordinator.finished[requestID] = struct{}{}
		coordinator.pruneLocked()
		return rssSitePrepareTicket{}, context.Canceled
	}
	coordinator.seen[requestID] = struct{}{}
	coordinator.latest = requestID
	coordinator.generation++
	coordinator.pruneLocked()
	return rssSitePrepareTicket{requestID: requestID, generation: coordinator.generation}, nil
}

func (coordinator *rssSitePrepareCoordinator) commit(
	ticket rssSitePrepareTicket,
	commit func() (RSSSitePlayerPrepareResponse, error),
) (RSSSitePlayerPrepareResponse, error) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	_, canceled := coordinator.canceled[ticket.requestID]
	_, finished := coordinator.finished[ticket.requestID]
	if canceled || finished || coordinator.latest != ticket.requestID || coordinator.generation != ticket.generation {
		coordinator.finishLocked(ticket.requestID)
		return RSSSitePlayerPrepareResponse{}, context.Canceled
	}
	response, err := commit()
	if err != nil {
		coordinator.finishLocked(ticket.requestID)
		return RSSSitePlayerPrepareResponse{}, err
	}
	coordinator.committed[ticket.requestID] = response.SessionID
	coordinator.pruneLocked()
	return response, nil
}

func (coordinator *rssSitePrepareCoordinator) fail(ticket rssSitePrepareTicket) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	coordinator.finishLocked(ticket.requestID)
}

func (coordinator *rssSitePrepareCoordinator) accept(requestID uint64) error {
	if requestID == 0 {
		return fmt.Errorf("RSS site prepare request id is required")
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	if requestID > coordinator.highestIssued {
		coordinator.highestIssued = requestID
	}
	if _, done := coordinator.finished[requestID]; !done {
		coordinator.finishLocked(requestID)
	}
	return nil
}

func (coordinator *rssSitePrepareCoordinator) cancel(requestID uint64) (string, error) {
	if requestID == 0 {
		return "", fmt.Errorf("RSS site prepare request id is required")
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	if requestID > coordinator.highestIssued {
		coordinator.highestIssued = requestID
	}
	if _, done := coordinator.finished[requestID]; done {
		return "", nil
	}
	if _, seen := coordinator.seen[requestID]; !seen {
		coordinator.canceled[requestID] = struct{}{}
		// A newer renderer intent may be cancelled before its Prepare RPC
		// reaches Go. It still supersedes every older in-flight Prepare; without
		// invalidating the generation here, an older ticket could commit and
		// navigate a hidden WebView after the user has already left it.
		if coordinator.latest != 0 && requestID > coordinator.latest {
			coordinator.generation++
			coordinator.latest = 0
		}
		coordinator.pruneLocked()
		return "", nil
	}
	if coordinator.latest == requestID {
		coordinator.generation++
		coordinator.latest = 0
	}
	sessionID := coordinator.committed[requestID]
	coordinator.finishLocked(requestID)
	return sessionID, nil
}

func (coordinator *rssSitePrepareCoordinator) ensureLocked() {
	if coordinator.seen == nil {
		coordinator.seen = make(map[uint64]struct{})
	}
	if coordinator.finished == nil {
		coordinator.finished = make(map[uint64]struct{})
	}
	if coordinator.canceled == nil {
		coordinator.canceled = make(map[uint64]struct{})
	}
	if coordinator.committed == nil {
		coordinator.committed = make(map[uint64]string)
	}
}

func (coordinator *rssSitePrepareCoordinator) finishLocked(requestID uint64) {
	delete(coordinator.seen, requestID)
	delete(coordinator.canceled, requestID)
	delete(coordinator.committed, requestID)
	coordinator.finished[requestID] = struct{}{}
	if coordinator.latest == requestID {
		coordinator.latest = 0
	}
	coordinator.pruneLocked()
}

func (coordinator *rssSitePrepareCoordinator) pruneLocked() {
	ids := make(map[uint64]struct{}, len(coordinator.seen)+len(coordinator.finished)+len(coordinator.canceled)+len(coordinator.committed))
	for requestID := range coordinator.seen {
		ids[requestID] = struct{}{}
	}
	for requestID := range coordinator.finished {
		ids[requestID] = struct{}{}
	}
	for requestID := range coordinator.canceled {
		ids[requestID] = struct{}{}
	}
	for requestID := range coordinator.committed {
		ids[requestID] = struct{}{}
	}
	if len(ids) <= maxRSSSitePrepares {
		return
	}
	ordered := make([]uint64, 0, len(ids))
	for requestID := range ids {
		ordered = append(ordered, requestID)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	for _, requestID := range ordered[:len(ordered)-maxRSSSitePrepares] {
		if requestID == coordinator.latest {
			continue
		}
		delete(coordinator.seen, requestID)
		delete(coordinator.finished, requestID)
		delete(coordinator.canceled, requestID)
		delete(coordinator.committed, requestID)
	}
}

type rssSitePagePlayer struct {
	app     *application.App
	windows *WindowManager
	service *applicationrss.SitePlayerService

	lifecycleMu sync.Mutex
	commandMu   sync.Mutex
	mu          sync.Mutex
	prepares    rssSitePrepareCoordinator

	window           *application.WebviewWindow
	sessionID        string
	playerURL        string
	embeddedVisible  bool
	embeddedSequence uint64
}

func (player *rssSitePagePlayer) Prepare(
	ctx context.Context,
	request RSSSitePlayerPrepareRequest,
) (RSSSitePlayerPrepareResponse, error) {
	if player == nil || player.service == nil {
		return RSSSitePlayerPrepareResponse{}, fmt.Errorf("RSS site player unavailable")
	}
	ticket, err := player.prepares.begin(request.RequestID)
	if err != nil {
		return RSSSitePlayerPrepareResponse{}, err
	}
	descriptor, err := player.service.Prepare(ctx, request.URL)
	if err != nil {
		player.prepares.fail(ticket)
		return RSSSitePlayerPrepareResponse{}, err
	}
	sessionID := fmt.Sprintf("rss-site-%d", rssSitePlayerSessionCounter.Add(1))
	return player.prepares.commit(ticket, func() (RSSSitePlayerPrepareResponse, error) {
		player.lifecycleMu.Lock()
		defer player.lifecycleMu.Unlock()
		player.closeCurrentWindow("")
		window, err := player.createWindow()
		if err != nil {
			return RSSSitePlayerPrepareResponse{}, err
		}
		player.mu.Lock()
		player.window = window
		player.sessionID = sessionID
		player.playerURL = descriptor.URL
		player.embeddedVisible = false
		player.embeddedSequence = 0
		player.mu.Unlock()
		loadRSSSitePlayerURL(
			window,
			descriptor.URL,
			descriptor.SiteKey,
			descriptor.Cookies,
			descriptor.AllowedDomains,
			descriptor.RegistrableSite,
		)
		return RSSSitePlayerPrepareResponse{
			SessionID:         sessionID,
			URL:               descriptor.URL,
			SiteKey:           descriptor.SiteKey,
			CredentialsLoaded: descriptor.CredentialsLoaded,
		}, nil
	})
}

func (player *rssSitePagePlayer) AcceptPrepare(requestID uint64) error {
	if player == nil {
		return fmt.Errorf("RSS site player unavailable")
	}
	return player.prepares.accept(requestID)
}

func (player *rssSitePagePlayer) CancelPrepare(requestID uint64) error {
	if player == nil {
		return fmt.Errorf("RSS site player unavailable")
	}
	sessionID, err := player.prepares.cancel(requestID)
	if err != nil || sessionID == "" {
		return err
	}
	return player.Close(sessionID)
}

func (player *rssSitePagePlayer) Show(sessionID string, rect ListenEmbeddedVideoRect) (bool, error) {
	if player == nil {
		return false, fmt.Errorf("RSS site player unavailable")
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if player.window == nil || strings.TrimSpace(sessionID) == "" || player.sessionID != strings.TrimSpace(sessionID) {
		player.mu.Unlock()
		return false, fmt.Errorf("RSS site player session is no longer active")
	}
	if player.windows == nil || player.windows.mainWindow == nil {
		player.mu.Unlock()
		return false, fmt.Errorf("RSS site player host window unavailable")
	}
	rect = normalizeListenEmbeddedVideoRect(rect)
	rect.Interactive = true
	if rect.Sequence > 0 && rect.Sequence < player.embeddedSequence {
		player.mu.Unlock()
		return false, nil
	}
	if rect.Sequence > player.embeddedSequence {
		player.embeddedSequence = rect.Sequence
	}
	window := player.window
	player.embeddedVisible = true
	player.mu.Unlock()
	if fullscreenOwnsPresentation, known := listenNativeEmbeddedVideoFullscreenOwnsPresentation(window.NativeWindow()); known && fullscreenOwnsPresentation {
		return true, nil
	}
	owner := listenClaimEmbeddedVideoOwner(window)
	shown := rssShowNativeInteractiveEmbeddedWebViewForOwner(owner, window, player.windows.mainWindow, rect)
	if !shown {
		listenReleaseEmbeddedVideoOwner(owner)
		player.mu.Lock()
		if player.window == window {
			player.embeddedVisible = false
		}
		player.mu.Unlock()
		return false, fmt.Errorf("interactive RSS site player unavailable")
	}
	return listenEmbeddedVideoRevealReady(shown, true, listenEmbeddedVideoOwnerActive(owner)), nil
}

func (player *rssSitePagePlayer) Hide(sessionID string, sequence uint64) (bool, error) {
	if player == nil {
		return false, nil
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if player.window == nil || strings.TrimSpace(sessionID) == "" || player.sessionID != strings.TrimSpace(sessionID) {
		player.mu.Unlock()
		return false, nil
	}
	if sequence > 0 && sequence < player.embeddedSequence {
		player.mu.Unlock()
		return false, nil
	}
	if sequence > player.embeddedSequence {
		player.embeddedSequence = sequence
	}
	window := player.window
	wasVisible := player.embeddedVisible
	player.embeddedVisible = false
	player.mu.Unlock()
	exitRSSSitePlayerDocumentFullscreen(window)
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	window.Hide()
	return wasVisible, nil
}

func (player *rssSitePagePlayer) Close(sessionID string) error {
	if player == nil {
		return nil
	}
	player.lifecycleMu.Lock()
	defer player.lifecycleMu.Unlock()
	player.closeCurrentWindow(sessionID)
	return nil
}

func (player *rssSitePagePlayer) closeCurrentWindow(sessionID string) bool {
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if wanted := strings.TrimSpace(sessionID); wanted != "" && wanted != player.sessionID {
		player.mu.Unlock()
		return false
	}
	window := player.window
	player.window = nil
	player.sessionID = ""
	player.playerURL = ""
	player.embeddedVisible = false
	player.embeddedSequence = 0
	player.mu.Unlock()
	if window != nil {
		exitRSSSitePlayerDocumentFullscreen(window)
		listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
		hideListenNativeEmbeddedWebView(window.NativeWindow())
		releaseRSSSitePlayerWindowFeatures(window)
		window.Close()
	}
	return window != nil
}

func exitRSSSitePlayerDocumentFullscreen(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	window.ExecJS(`(() => {
		try {
			if (document.fullscreenElement && typeof document.exitFullscreen === "function") {
				void document.exitFullscreen();
				return;
			}
			if (document.webkitFullscreenElement && typeof document.webkitExitFullscreen === "function") {
				document.webkitExitFullscreen();
				return;
			}
			const video = document.querySelector("video");
			if (video && video.webkitDisplayingFullscreen && typeof video.webkitExitFullscreen === "function") {
				video.webkitExitFullscreen();
			}
		} catch (_) {}
	})()`)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ownsPresentation, known := listenNativeEmbeddedVideoFullscreenOwnsPresentation(window.NativeWindow())
		if !known || !ownsPresentation {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (player *rssSitePagePlayer) createWindow() (*application.WebviewWindow, error) {
	if player.app == nil {
		return nil, fmt.Errorf("RSS site player application unavailable")
	}
	window := player.app.Window.NewWithOptions(withRemoteWebViewPermissionPolicy(application.WebviewWindowOptions{
		Name:                       rssSitePlayerWindowName,
		Title:                      "RSS Site Video",
		Width:                      960,
		Height:                     540,
		MinWidth:                   320,
		MinHeight:                  180,
		URL:                        rssSitePlayerBlankURL,
		Hidden:                     true,
		Frameless:                  true,
		DefaultContextMenuDisabled: true,
		BackgroundColour:           application.NewRGBA(0, 0, 0, 255),
		Mac: application.MacWindow{WebviewPreferences: application.MacWebviewPreferences{
			FullscreenEnabled: application.Enabled,
		}},
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
			Permissions: map[application.CoreWebView2PermissionKind]application.CoreWebView2PermissionState{
				remoteMediaWebViewAutoplayPermissionKind: application.CoreWebView2PermissionStateAllow,
			},
		},
	}))
	if window == nil {
		return nil, fmt.Errorf("failed to create RSS site player window")
	}
	registerWebViewRemoteCapabilityPolicy(window)
	return window, nil
}
