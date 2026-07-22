package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	applicationrss "xiadown/internal/application/rss"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	rssBilibiliPlayerWindowName          = "rss-bilibili-video-player"
	rssBilibiliPlayerSource              = "rss-bilibili-video-player"
	rssBilibiliPlayerEventName           = "rss:bilibili-video-player"
	rssBilibiliPlayerOrigin              = "https://www.bilibili.com"
	rssBilibiliPlayerBlankURL            = "about:blank"
	rssBilibiliFullscreenExitRequestType = "fullscreen-exit-request"
	rssBilibiliIdentityViolationType     = "identity-violation"
	maxRSSBilibiliRawMessageBytes        = 64 << 10
)

var rssBilibiliPlayerSessionCounter atomic.Uint64

type RSSVideoPlayerPrepareRequest struct {
	RequestID       uint64   `json:"requestId"`
	PlatformVideoID string   `json:"platformVideoId"`
	StartSeconds    float64  `json:"startSeconds,omitempty"`
	Volume          *float64 `json:"volume,omitempty"`
	Muted           bool     `json:"muted,omitempty"`
	Autoplay        *bool    `json:"autoplay,omitempty"`
}

type RSSVideoPlayerPrepareTransactionRequest struct {
	RequestID uint64 `json:"requestId"`
}

type RSSVideoPlayerPrepareResponse struct {
	Platform        string `json:"platform"`
	Adapter         string `json:"adapter"`
	PlatformVideoID string `json:"platformVideoId"`
	PlayerURL       string `json:"playerUrl"`
	Authenticated   bool   `json:"authenticated"`
	SessionID       string `json:"sessionId"`
}

type RSSVideoPlayerSessionRequest struct {
	SessionID string `json:"sessionId,omitempty"`
}

type RSSVideoPlayerSeekRequest struct {
	SessionID string  `json:"sessionId"`
	Seconds   float64 `json:"seconds"`
}

type RSSVideoPlayerVolumeRequest struct {
	SessionID string  `json:"sessionId"`
	Volume    float64 `json:"volume"`
	Muted     bool    `json:"muted"`
}

type RSSVideoPlayerRateRequest struct {
	SessionID string  `json:"sessionId"`
	Rate      float64 `json:"rate"`
}

type RSSVideoPlayerSelectionRequest struct {
	SessionID string `json:"sessionId"`
	Value     string `json:"value"`
}

type RSSVideoPlayerOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type RSSVideoPlayerControls struct {
	PlayPause    bool `json:"playPause"`
	Seek         bool `json:"seek"`
	Volume       bool `json:"volume"`
	PlaybackRate bool `json:"playbackRate"`
	Fullscreen   bool `json:"fullscreen"`
	Captions     bool `json:"captions"`
	Quality      bool `json:"quality"`
	Danmaku      bool `json:"danmaku"`
}

type RSSVideoPlayerSelections struct {
	PlaybackRateID string `json:"playbackRateId,omitempty"`
	CaptionID      string `json:"captionId"`
	QualityID      string `json:"qualityId,omitempty"`
}

type RSSVideoPlayerStatus struct {
	Provider            string                   `json:"provider"`
	SessionID           string                   `json:"sessionId,omitempty"`
	Available           bool                     `json:"available"`
	PlatformVideoID     string                   `json:"platformVideoId,omitempty"`
	State               string                   `json:"state"`
	Title               string                   `json:"title,omitempty"`
	Publisher           string                   `json:"publisher,omitempty"`
	PublishedAt         string                   `json:"publishedAt,omitempty"`
	ViewCount           uint64                   `json:"viewCount,omitempty"`
	LikeCount           uint64                   `json:"likeCount,omitempty"`
	CurrentTime         float64                  `json:"currentTime"`
	Duration            float64                  `json:"duration"`
	BufferedTime        float64                  `json:"bufferedTime"`
	Volume              float64                  `json:"volume"`
	Muted               bool                     `json:"muted"`
	PlaybackRate        float64                  `json:"playbackRate"`
	Fullscreen          bool                     `json:"fullscreen"`
	Controls            RSSVideoPlayerControls   `json:"controls"`
	PlaybackRateOptions []RSSVideoPlayerOption   `json:"playbackRateOptions"`
	CaptionOptions      []RSSVideoPlayerOption   `json:"captionOptions"`
	QualityOptions      []RSSVideoPlayerOption   `json:"qualityOptions"`
	DanmakuEnabled      bool                     `json:"danmakuEnabled"`
	Selections          RSSVideoPlayerSelections `json:"selections"`
	ErrorMessage        string                   `json:"errorMessage,omitempty"`
}

type rssVideoPlayer interface {
	Prepare(context.Context, RSSVideoPlayerPrepareRequest) (RSSVideoPlayerPrepareResponse, error)
	AcceptPrepare(uint64) error
	CancelPrepare(uint64) error
	Play(string) error
	Pause(string) error
	Seek(string, float64) error
	SetVolume(string, float64, bool) error
	SetPlaybackRate(string, float64) error
	ToggleCaptions(string) error
	SelectCaption(string, string) error
	SelectQuality(string, string) error
	ToggleDanmaku(string) error
	RequestFullscreen(string) error
	ExitFullscreen(string) error
	Status() RSSVideoPlayerStatus
	Show(ListenEmbeddedVideoRect) (bool, error)
	Hide(uint64) (bool, error)
	Close(string) error
	HandleRawMessage(application.Window, string, *application.OriginInfo) bool
}

// RSSVideoPlayerHandler owns a playback-only native WebView. App Sessions are
// used solely as a credential source by the application service; this handler
// never opens or navigates the App Session browser.
type RSSVideoPlayerHandler struct {
	player rssVideoPlayer
}

// RSSVideoPlayerRawMessageHandler is deliberately separate from the Wails
// service. Raw WebView messages need application.Window provenance and must
// never become a renderer-callable binding.
type RSSVideoPlayerRawMessageHandler struct {
	player rssVideoPlayer
}

func NewRSSVideoPlayerHandler(
	app *application.App,
	windows *WindowManager,
	service *applicationrss.VideoPlayerService,
) *RSSVideoPlayerHandler {
	return &RSSVideoPlayerHandler{
		player: &rssBilibiliVideoPlayer{
			app:     app,
			windows: windows,
			service: service,
			status:  newRSSVideoPlayerStatus(),
		},
	}
}

func NewRSSVideoPlayerRawMessageHandler(
	handler *RSSVideoPlayerHandler,
) *RSSVideoPlayerRawMessageHandler {
	if handler == nil {
		return &RSSVideoPlayerRawMessageHandler{}
	}
	return &RSSVideoPlayerRawMessageHandler{player: handler.player}
}

func (handler *RSSVideoPlayerHandler) ServiceName() string {
	return "RSSVideoPlayerHandler"
}

func (handler *RSSVideoPlayerHandler) Prepare(
	ctx context.Context,
	request RSSVideoPlayerPrepareRequest,
) (RSSVideoPlayerPrepareResponse, error) {
	if handler == nil || handler.player == nil {
		return RSSVideoPlayerPrepareResponse{}, fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return RSSVideoPlayerPrepareResponse{}, err
	}
	return handler.player.Prepare(ctx, request)
}

func (handler *RSSVideoPlayerHandler) AcceptPrepare(
	ctx context.Context,
	request RSSVideoPlayerPrepareTransactionRequest,
) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.AcceptPrepare(request.RequestID)
}

func (handler *RSSVideoPlayerHandler) CancelPrepare(
	ctx context.Context,
	request RSSVideoPlayerPrepareTransactionRequest,
) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.CancelPrepare(request.RequestID)
}

func (handler *RSSVideoPlayerHandler) Play(ctx context.Context, request RSSVideoPlayerSessionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.Play(request.SessionID)
}

func (handler *RSSVideoPlayerHandler) Pause(ctx context.Context, request RSSVideoPlayerSessionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.Pause(request.SessionID)
}

func (handler *RSSVideoPlayerHandler) Seek(ctx context.Context, request RSSVideoPlayerSeekRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.Seek(request.SessionID, request.Seconds)
}

func (handler *RSSVideoPlayerHandler) SetVolume(ctx context.Context, request RSSVideoPlayerVolumeRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.SetVolume(request.SessionID, request.Volume, request.Muted)
}

func (handler *RSSVideoPlayerHandler) SetPlaybackRate(ctx context.Context, request RSSVideoPlayerRateRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.SetPlaybackRate(request.SessionID, request.Rate)
}

func (handler *RSSVideoPlayerHandler) ToggleCaptions(ctx context.Context, request RSSVideoPlayerSessionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.ToggleCaptions(request.SessionID)
}

func (handler *RSSVideoPlayerHandler) SelectCaption(ctx context.Context, request RSSVideoPlayerSelectionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.SelectCaption(request.SessionID, request.Value)
}

func (handler *RSSVideoPlayerHandler) SelectQuality(ctx context.Context, request RSSVideoPlayerSelectionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.SelectQuality(request.SessionID, request.Value)
}

func (handler *RSSVideoPlayerHandler) ToggleDanmaku(ctx context.Context, request RSSVideoPlayerSessionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.ToggleDanmaku(request.SessionID)
}

func (handler *RSSVideoPlayerHandler) RequestFullscreen(ctx context.Context, request RSSVideoPlayerSessionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.RequestFullscreen(request.SessionID)
}

func (handler *RSSVideoPlayerHandler) ExitFullscreen(ctx context.Context, request RSSVideoPlayerSessionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.ExitFullscreen(request.SessionID)
}

func (handler *RSSVideoPlayerHandler) Status(
	ctx context.Context,
) (RSSVideoPlayerStatus, error) {
	if handler == nil || handler.player == nil {
		return RSSVideoPlayerStatus{}, fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return RSSVideoPlayerStatus{}, err
	}
	return handler.player.Status(), nil
}

func (handler *RSSVideoPlayerHandler) Show(
	ctx context.Context,
	rect ListenEmbeddedVideoRect,
) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return false, err
	}
	return handler.player.Show(rect)
}

func (handler *RSSVideoPlayerHandler) Hide(
	ctx context.Context,
	request ListenEmbeddedVideoHideRequest,
) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("RSS video player unavailable")
	}
	if err := contextError(ctx); err != nil {
		return false, err
	}
	return handler.player.Hide(request.Sequence)
}

// Close accepts an optional session identity. An empty identity preserves the
// legacy close-current-player behavior, while a stale non-empty identity is a
// no-op so an unmounting React effect cannot tear down a newer playback.
func (handler *RSSVideoPlayerHandler) Close(
	ctx context.Context,
	request RSSVideoPlayerSessionRequest,
) error {
	if handler == nil || handler.player == nil {
		return nil
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return handler.player.Close(request.SessionID)
}

func (handler *RSSVideoPlayerRawMessageHandler) HandleRawMessage(
	window application.Window,
	message string,
	originInfo *application.OriginInfo,
) bool {
	if handler == nil || handler.player == nil {
		return false
	}
	return handler.player.HandleRawMessage(window, message, originInfo)
}

func (handler *RSSVideoPlayerHandler) Shutdown() error {
	if handler == nil || handler.player == nil {
		return nil
	}
	return handler.player.Close("")
}

const maxRSSVideoPrepareTransactions = 64

type rssVideoPrepareTicket struct {
	requestID  uint64
	generation uint64
}

// rssVideoPrepareCoordinator makes the destructive native-window commit the
// final step of Prepare. Resolution may finish in any order, but only the
// newest, non-canceled ticket can enter commit. The callback runs while the
// coordinator is locked so a newer Begin cannot slip between the generation
// check and closeCurrentWindow.
type rssVideoPrepareCoordinator struct {
	mu            sync.Mutex
	generation    uint64
	latest        uint64
	highestIssued uint64
	seen          map[uint64]struct{}
	finished      map[uint64]struct{}
	canceled      map[uint64]struct{}
	committed     map[uint64]string
}

func (coordinator *rssVideoPrepareCoordinator) begin(requestID uint64) (rssVideoPrepareTicket, error) {
	if requestID == 0 {
		return rssVideoPrepareTicket{}, fmt.Errorf("RSS Bilibili prepare request id is required")
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	if requestID < coordinator.highestIssued {
		coordinator.finished[requestID] = struct{}{}
		coordinator.pruneLocked()
		return rssVideoPrepareTicket{}, context.Canceled
	}
	if requestID > coordinator.highestIssued {
		coordinator.highestIssued = requestID
	}
	if _, finished := coordinator.finished[requestID]; finished {
		return rssVideoPrepareTicket{}, context.Canceled
	}
	if _, duplicate := coordinator.seen[requestID]; duplicate {
		return rssVideoPrepareTicket{}, context.Canceled
	}
	if _, canceled := coordinator.canceled[requestID]; canceled {
		delete(coordinator.canceled, requestID)
		coordinator.finished[requestID] = struct{}{}
		coordinator.pruneLocked()
		return rssVideoPrepareTicket{}, context.Canceled
	}
	coordinator.seen[requestID] = struct{}{}
	coordinator.latest = requestID
	coordinator.generation++
	coordinator.pruneLocked()
	return rssVideoPrepareTicket{requestID: requestID, generation: coordinator.generation}, nil
}

func (coordinator *rssVideoPrepareCoordinator) commit(
	ticket rssVideoPrepareTicket,
	commit func() (RSSVideoPlayerPrepareResponse, error),
) (RSSVideoPlayerPrepareResponse, error) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	_, canceled := coordinator.canceled[ticket.requestID]
	_, finished := coordinator.finished[ticket.requestID]
	if finished || canceled || ticket.generation != coordinator.generation || coordinator.latest != ticket.requestID {
		coordinator.finishLocked(ticket.requestID)
		return RSSVideoPlayerPrepareResponse{}, context.Canceled
	}
	response, err := commit()
	if err != nil {
		coordinator.finishLocked(ticket.requestID)
		return RSSVideoPlayerPrepareResponse{}, err
	}
	coordinator.committed[ticket.requestID] = response.SessionID
	coordinator.pruneLocked()
	return response, nil
}

func (coordinator *rssVideoPrepareCoordinator) fail(ticket rssVideoPrepareTicket) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	coordinator.finishLocked(ticket.requestID)
}

func (coordinator *rssVideoPrepareCoordinator) accept(requestID uint64) error {
	if requestID == 0 {
		return fmt.Errorf("RSS Bilibili prepare request id is required")
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	if requestID > coordinator.highestIssued {
		coordinator.highestIssued = requestID
	}
	if _, finished := coordinator.finished[requestID]; finished {
		return nil
	}
	coordinator.finishLocked(requestID)
	return nil
}

func (coordinator *rssVideoPrepareCoordinator) cancel(requestID uint64) (string, error) {
	if requestID == 0 {
		return "", fmt.Errorf("RSS Bilibili prepare request id is required")
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.ensureLocked()
	if requestID > coordinator.highestIssued {
		coordinator.highestIssued = requestID
	}
	if _, finished := coordinator.finished[requestID]; finished {
		return "", nil
	}
	if _, seen := coordinator.seen[requestID]; !seen {
		// Cancel may overtake Prepare at the RPC boundary. Remember it so Begin
		// fails closed when the delayed Prepare call eventually arrives.
		coordinator.canceled[requestID] = struct{}{}
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

func (coordinator *rssVideoPrepareCoordinator) ensureLocked() {
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

func (coordinator *rssVideoPrepareCoordinator) finishLocked(requestID uint64) {
	delete(coordinator.seen, requestID)
	delete(coordinator.canceled, requestID)
	delete(coordinator.committed, requestID)
	coordinator.finished[requestID] = struct{}{}
	if coordinator.latest == requestID {
		coordinator.latest = 0
	}
	coordinator.pruneLocked()
}

func (coordinator *rssVideoPrepareCoordinator) pruneLocked() {
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
	if len(ids) <= maxRSSVideoPrepareTransactions {
		return
	}
	ordered := make([]uint64, 0, len(ids))
	for requestID := range ids {
		ordered = append(ordered, requestID)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	for _, requestID := range ordered[:len(ordered)-maxRSSVideoPrepareTransactions] {
		if requestID == coordinator.latest {
			continue
		}
		delete(coordinator.seen, requestID)
		delete(coordinator.finished, requestID)
		delete(coordinator.canceled, requestID)
		delete(coordinator.committed, requestID)
	}
}

type rssBilibiliVideoPlayer struct {
	app     *application.App
	windows *WindowManager
	service *applicationrss.VideoPlayerService

	lifecycleMu sync.Mutex
	commandMu   sync.Mutex
	mu          sync.Mutex
	prepares    rssVideoPrepareCoordinator

	window           *application.WebviewWindow
	bridgeHook       func()
	playerURL        string
	authenticated    bool
	embeddedVisible  bool
	embeddedRect     ListenEmbeddedVideoRect
	embeddedSequence uint64
	status           RSSVideoPlayerStatus
	fullscreen       listenEmbeddedVideoFullscreenRequests

	fullscreenTransition   bool
	fullscreenGeneration   uint64
	nativeWindowFullscreen bool
	nativeFullscreenWaiter chan bool
}

func (player *rssBilibiliVideoPlayer) Prepare(
	ctx context.Context,
	request RSSVideoPlayerPrepareRequest,
) (RSSVideoPlayerPrepareResponse, error) {
	if player == nil || player.service == nil {
		return RSSVideoPlayerPrepareResponse{}, fmt.Errorf("RSS video player unavailable")
	}
	ticket, err := player.prepares.begin(request.RequestID)
	if err != nil {
		return RSSVideoPlayerPrepareResponse{}, err
	}
	descriptor, err := player.service.PrepareBilibili(ctx, request.PlatformVideoID)
	if err != nil {
		player.prepares.fail(ticket)
		return RSSVideoPlayerPrepareResponse{}, err
	}
	config, err := normalizeRSSBilibiliBridgeConfig(request)
	if err != nil {
		player.prepares.fail(ticket)
		return RSSVideoPlayerPrepareResponse{}, err
	}
	config.SessionID = nextRSSBilibiliPlayerSessionID()
	config.Adapter = descriptor.Adapter
	config.PlatformVideoID = descriptor.PlatformVideoID
	bridgeScript := rssBilibiliHTMLMediaBridgeScript(config)

	return player.prepares.commit(ticket, func() (RSSVideoPlayerPrepareResponse, error) {
		player.lifecycleMu.Lock()
		defer player.lifecycleMu.Unlock()
		player.closeCurrentWindow("")

		window, bridgeHook, err := player.createWindow(bridgeScript)
		if err != nil {
			return RSSVideoPlayerPrepareResponse{}, err
		}
		status := newRSSVideoPlayerStatus()
		status.SessionID = config.SessionID
		status.Available = true
		status.PlatformVideoID = descriptor.PlatformVideoID
		status.State = "loading"
		status.Volume = config.Volume
		status.Muted = config.Muted
		status.PlaybackRate = config.PlaybackRate
		status.Selections.PlaybackRateID = rssBilibiliPlaybackRateID(config.PlaybackRate)

		player.mu.Lock()
		player.window = window
		player.bridgeHook = bridgeHook
		player.playerURL = descriptor.PlayerURL
		player.authenticated = descriptor.Authenticated
		player.embeddedVisible = false
		player.embeddedSequence = 0
		player.status = status
		player.mu.Unlock()

		// The platform helper registers cookies before the first request and then
		// navigates this playback-only WebView. It never opens an App Session window.
		loadRSSVideoPlayerURL(window, descriptor.PlayerURL, descriptor.Cookies)
		return RSSVideoPlayerPrepareResponse{
			Platform:        descriptor.Platform,
			Adapter:         descriptor.Adapter,
			PlatformVideoID: descriptor.PlatformVideoID,
			PlayerURL:       descriptor.PlayerURL,
			Authenticated:   descriptor.Authenticated,
			SessionID:       config.SessionID,
		}, nil
	})
}

func (player *rssBilibiliVideoPlayer) AcceptPrepare(requestID uint64) error {
	if player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	return player.prepares.accept(requestID)
}

func (player *rssBilibiliVideoPlayer) CancelPrepare(requestID uint64) error {
	if player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	sessionID, err := player.prepares.cancel(requestID)
	if err != nil || sessionID == "" {
		return err
	}
	// Close remains session-aware. A late cancellation for an older committed
	// Prepare cannot tear down a newer native player installed in the meantime.
	return player.Close(sessionID)
}

func (player *rssBilibiliVideoPlayer) Play(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliSimpleCommandScript(sessionID, "play"))
	return nil
}

func (player *rssBilibiliVideoPlayer) Pause(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliSimpleCommandScript(sessionID, "pause"))
	return nil
}

func (player *rssBilibiliVideoPlayer) Seek(sessionID string, seconds float64) error {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("RSS Bilibili seek position must be finite")
	}
	seconds = math.Max(0, seconds)
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliNumberCommandScript(sessionID, "seek", seconds))
	return nil
}

func (player *rssBilibiliVideoPlayer) SetVolume(sessionID string, volume float64, muted bool) error {
	if math.IsNaN(volume) || math.IsInf(volume, 0) {
		return fmt.Errorf("RSS Bilibili volume must be finite")
	}
	volume = math.Max(0, math.Min(1, volume))
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliVolumeCommandScript(sessionID, volume, muted))
	player.mu.Lock()
	if player.status.SessionID == strings.TrimSpace(sessionID) {
		player.status.Volume = volume
		player.status.Muted = muted
	}
	status := cloneRSSVideoPlayerStatus(player.status)
	player.mu.Unlock()
	player.dispatchStatus(status)
	return nil
}

func (player *rssBilibiliVideoPlayer) SetPlaybackRate(sessionID string, rate float64) error {
	if !rssBilibiliPlaybackRateAllowed(rate) {
		return fmt.Errorf("unsupported RSS Bilibili playback rate %s", strconv.FormatFloat(rate, 'g', -1, 64))
	}
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliNumberCommandScript(sessionID, "rate", rate))
	player.mu.Lock()
	if player.status.SessionID == strings.TrimSpace(sessionID) {
		player.status.PlaybackRate = rate
		player.status.Selections.PlaybackRateID = rssBilibiliPlaybackRateID(rate)
	}
	status := cloneRSSVideoPlayerStatus(player.status)
	player.mu.Unlock()
	player.dispatchStatus(status)
	return nil
}

func (player *rssBilibiliVideoPlayer) ToggleCaptions(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliSimpleCommandScript(sessionID, "toggleCaptions"))
	return nil
}

func (player *rssBilibiliVideoPlayer) SelectCaption(sessionID string, value string) error {
	value, err := normalizeRSSBilibiliCaptionSelectionValue(value)
	if err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliStringCommandScript(sessionID, "selectCaption", value))
	return nil
}

func (player *rssBilibiliVideoPlayer) SelectQuality(sessionID string, value string) error {
	value, err := normalizeRSSBilibiliSelectionValue("quality", value)
	if err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliStringCommandScript(sessionID, "selectQuality", value))
	return nil
}

func (player *rssBilibiliVideoPlayer) ToggleDanmaku(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	execListenYouTubeMusicJS(window, rssBilibiliSimpleCommandScript(sessionID, "toggleDanmaku"))
	return nil
}

func (player *rssBilibiliVideoPlayer) RequestFullscreen(sessionID string) error {
	return player.setFullscreen(sessionID, true)
}

func (player *rssBilibiliVideoPlayer) ExitFullscreen(sessionID string) error {
	return player.setFullscreen(sessionID, false)
}

func (player *rssBilibiliVideoPlayer) setFullscreen(sessionID string, enter bool) error {
	if player == nil {
		return fmt.Errorf("RSS video player unavailable")
	}
	sessionID = strings.TrimSpace(sessionID)
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	window, err := player.controlWindow(sessionID)
	if err != nil {
		return err
	}
	player.mu.Lock()
	visible := player.embeddedVisible
	active := player.status.Fullscreen
	transitioning := player.fullscreenTransition
	player.mu.Unlock()
	if enter && !visible {
		return fmt.Errorf("embedded RSS Bilibili video unavailable")
	}
	if !transitioning && active == enter {
		return nil
	}
	if listenEmbeddedVideoUsesNativeWindowFullscreen() {
		if enter {
			return player.requestNativeWindowFullscreenLocked(window, sessionID)
		}
		return player.exitNativeWindowFullscreenLocked(window, sessionID)
	}

	player.mu.Lock()
	if player.window != window || player.status.SessionID != sessionID {
		player.mu.Unlock()
		return fmt.Errorf("RSS Bilibili video session is no longer active")
	}
	player.fullscreenGeneration += 1
	generation := player.fullscreenGeneration
	player.fullscreenTransition = true
	player.mu.Unlock()
	if enter {
		execListenYouTubeMusicJS(
			window,
			rssBilibiliBooleanCommandScript(sessionID, "fullscreenPresentation", true),
		)
	}
	err = requestListenEmbeddedVideoFullscreenForSession(
		window,
		rssBilibiliPlayerSource,
		sessionID,
		&player.fullscreen,
		enter,
	)
	player.mu.Lock()
	valid := player.window == window &&
		player.status.SessionID == sessionID &&
		player.fullscreenGeneration == generation
	if valid && err != nil {
		player.fullscreenTransition = false
	}
	player.mu.Unlock()
	if !enter || err != nil {
		execListenYouTubeMusicJS(window, rssBilibiliBooleanCommandScript(
			sessionID,
			"fullscreenPresentation",
			false,
		))
	}
	if err == nil {
		// The bridge's fullscreenchange event is authoritative. A successful
		// command is the fallback for engines that resolve the request without
		// forwarding that event through the raw-message callback.
		player.updateFullscreenStatus(sessionID, enter)
	}
	return err
}

func (player *rssBilibiliVideoPlayer) requestNativeWindowFullscreenLocked(
	window *application.WebviewWindow,
	sessionID string,
) error {
	if player == nil || window == nil {
		return fmt.Errorf("embedded RSS Bilibili video unavailable")
	}
	waiter := make(chan bool, 1)
	player.mu.Lock()
	if player.window != window ||
		player.status.SessionID != sessionID ||
		!player.embeddedVisible {
		player.mu.Unlock()
		return fmt.Errorf("embedded RSS Bilibili video unavailable")
	}
	player.fullscreenGeneration += 1
	generation := player.fullscreenGeneration
	player.nativeWindowFullscreen = true
	player.nativeFullscreenWaiter = waiter
	player.fullscreenTransition = true
	player.status.Fullscreen = false
	player.mu.Unlock()

	// Match the YouTube station: return the singleton WebView to its owning
	// player window before asking the OS to fullscreen that native window.
	// Inline React geometry is suspended for the entire transition.
	execListenYouTubeMusicJS(window, rssBilibiliBooleanCommandScript(
		sessionID,
		"fullscreenPresentation",
		true,
	))
	if !detachListenNativeEmbeddedWebViewForFullscreen(window.NativeWindow()) {
		player.mu.Lock()
		if player.window == window &&
			player.status.SessionID == sessionID &&
			player.fullscreenGeneration == generation {
			player.nativeWindowFullscreen = false
			player.nativeFullscreenWaiter = nil
			player.fullscreenTransition = false
			player.status.Fullscreen = false
		}
		player.mu.Unlock()
		execListenYouTubeMusicJS(window, rssBilibiliBooleanCommandScript(
			sessionID,
			"fullscreenPresentation",
			false,
		))
		return fmt.Errorf("embedded RSS Bilibili video could not detach for native fullscreen")
	}
	window.SetTitle("Bilibili")
	var hostWindow *application.WebviewWindow
	if player.windows != nil {
		hostWindow = player.windows.mainWindow
	}
	prepareListenNativeFullscreenWindow(window, hostWindow, 960, 540)
	window.Show()
	window.Focus()
	if delay := listenNativeWindowFullscreenPreparationDelay(); delay > 0 {
		time.Sleep(delay)
	}

	player.mu.Lock()
	valid := player.window == window &&
		player.status.SessionID == sessionID &&
		player.fullscreenGeneration == generation &&
		player.nativeWindowFullscreen
	player.mu.Unlock()
	if !valid {
		return fmt.Errorf("stale RSS Bilibili fullscreen request")
	}
	window.Fullscreen()
	entered := waitForListenNativeWindowFullscreenState(
		waiter,
		true,
		listenEmbeddedVideoFullscreenTimeout,
		nil,
	)
	if !entered {
		entered = window.IsFullscreen()
	}

	player.mu.Lock()
	if player.nativeFullscreenWaiter == waiter {
		player.nativeFullscreenWaiter = nil
	}
	valid = player.window == window &&
		player.status.SessionID == sessionID &&
		player.fullscreenGeneration == generation &&
		player.nativeWindowFullscreen
	if entered && valid {
		changed := !player.status.Fullscreen || player.fullscreenTransition
		player.status.Fullscreen = true
		player.fullscreenTransition = false
		status := cloneRSSVideoPlayerStatus(player.status)
		player.mu.Unlock()
		if changed {
			player.dispatchStatus(status)
		}
		return nil
	}
	player.mu.Unlock()
	if valid {
		player.restoreAfterNativeWindowFullscreenLocked(window, generation)
	}
	return fmt.Errorf("the native RSS Bilibili video window did not enter fullscreen")
}

func (player *rssBilibiliVideoPlayer) exitNativeWindowFullscreenLocked(
	window *application.WebviewWindow,
	sessionID string,
) error {
	if player == nil || window == nil {
		return fmt.Errorf("embedded RSS Bilibili video unavailable")
	}
	waiter := make(chan bool, 1)
	player.mu.Lock()
	if player.window != window || player.status.SessionID != sessionID {
		player.mu.Unlock()
		return fmt.Errorf("RSS Bilibili video session is no longer active")
	}
	if !player.nativeWindowFullscreen {
		player.status.Fullscreen = false
		player.fullscreenTransition = false
		player.mu.Unlock()
		return nil
	}
	generation := player.fullscreenGeneration
	player.nativeFullscreenWaiter = waiter
	player.fullscreenTransition = true
	player.mu.Unlock()

	if window.IsFullscreen() {
		window.UnFullscreen()
	} else {
		player.restoreAfterNativeWindowFullscreenLocked(window, generation)
		return nil
	}
	exited := waitForListenNativeWindowFullscreenState(
		waiter,
		false,
		2*listenEmbeddedVideoFullscreenTimeout,
		func(active bool) {
			// Exit can race a delayed native did-enter notification. Retry the
			// native exit when that notification arrives, as the YouTube player does.
			if active {
				window.UnFullscreen()
			}
		},
	)
	if exited {
		return nil
	}
	player.mu.Lock()
	if player.nativeFullscreenWaiter == waiter {
		player.nativeFullscreenWaiter = nil
	}
	valid := player.window == window &&
		player.status.SessionID == sessionID &&
		player.fullscreenGeneration == generation &&
		player.nativeWindowFullscreen
	player.mu.Unlock()
	if valid && !window.IsFullscreen() {
		player.restoreAfterNativeWindowFullscreenLocked(window, generation)
		return nil
	}
	return fmt.Errorf("the native RSS Bilibili video window did not exit fullscreen")
}

func (player *rssBilibiliVideoPlayer) handleNativeWindowFullscreenEvent(
	window *application.WebviewWindow,
	active bool,
) {
	if player == nil || window == nil || !listenEmbeddedVideoUsesNativeWindowFullscreen() {
		return
	}
	player.mu.Lock()
	if player.window != window || !player.nativeWindowFullscreen {
		player.mu.Unlock()
		return
	}
	waiter := player.nativeFullscreenWaiter
	generation := player.fullscreenGeneration
	changed := false
	var status RSSVideoPlayerStatus
	if active {
		changed = !player.status.Fullscreen || player.fullscreenTransition
		player.status.Fullscreen = true
		player.fullscreenTransition = false
		if changed {
			status = cloneRSSVideoPlayerStatus(player.status)
		}
	} else {
		// Keep geometry suspended until the WebView has been returned to the
		// latest inline rect. The restore publishes the authoritative false state.
		player.fullscreenTransition = true
	}
	player.mu.Unlock()
	completeListenNativeFullscreenWaiter(waiter, active)
	if active {
		if changed {
			player.dispatchStatus(status)
		}
		return
	}
	go player.restoreAfterNativeWindowFullscreen(window, generation)
}

func (player *rssBilibiliVideoPlayer) restoreAfterNativeWindowFullscreen(
	window *application.WebviewWindow,
	generation uint64,
) {
	if player == nil || window == nil {
		return
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.restoreAfterNativeWindowFullscreenLocked(window, generation)
}

// restoreAfterNativeWindowFullscreenLocked returns the WebView to the latest
// inline geometry before clearing the fullscreen status. The caller holds
// commandMu so delayed Show/Hide calls cannot reparent the singleton WebView
// during the native transition.
func (player *rssBilibiliVideoPlayer) restoreAfterNativeWindowFullscreenLocked(
	window *application.WebviewWindow,
	generation uint64,
) {
	player.mu.Lock()
	if player.window != window ||
		player.fullscreenGeneration != generation ||
		!player.nativeWindowFullscreen {
		player.mu.Unlock()
		return
	}
	shouldEmbed := player.embeddedVisible
	rect := player.embeddedRect
	sessionID := player.status.SessionID
	player.mu.Unlock()

	execListenYouTubeMusicJS(window, rssBilibiliBooleanCommandScript(
		sessionID,
		"fullscreenPresentation",
		false,
	))
	window.Hide()
	shown := false
	if shouldEmbed && player.windows != nil && player.windows.mainWindow != nil {
		rect.Interactive = false
		owner := listenClaimEmbeddedVideoOwner(window)
		shown = rssShowNativeEmbeddedWebViewForOwner(
			owner,
			window,
			player.windows.mainWindow,
			rect,
		)
		if !shown {
			listenReleaseEmbeddedVideoOwner(owner)
		}
	} else {
		listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
		hideListenNativeEmbeddedWebView(window.NativeWindow())
	}

	player.mu.Lock()
	if player.window != window ||
		player.status.SessionID != sessionID ||
		player.fullscreenGeneration != generation {
		player.mu.Unlock()
		return
	}
	changed := player.status.Fullscreen || player.fullscreenTransition
	player.status.Fullscreen = false
	player.fullscreenTransition = false
	player.nativeWindowFullscreen = false
	waiter := player.nativeFullscreenWaiter
	player.nativeFullscreenWaiter = nil
	status := cloneRSSVideoPlayerStatus(player.status)
	player.mu.Unlock()
	completeListenNativeFullscreenWaiter(waiter, false)
	if changed {
		player.dispatchStatus(status)
	}
	_ = shown // A resumed frontend geometry pass retries a failed inline embed.
}

func (player *rssBilibiliVideoPlayer) Status() RSSVideoPlayerStatus {
	if player == nil {
		return newRSSVideoPlayerStatus()
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	return cloneRSSVideoPlayerStatus(player.status)
}

func (player *rssBilibiliVideoPlayer) Show(rect ListenEmbeddedVideoRect) (bool, error) {
	if player == nil {
		return false, fmt.Errorf("RSS video player unavailable")
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	window := player.window
	if window == nil || player.status.SessionID == "" || player.playerURL == "" {
		player.mu.Unlock()
		return false, fmt.Errorf("RSS Bilibili video is not prepared")
	}
	if player.windows == nil || player.windows.mainWindow == nil {
		player.mu.Unlock()
		return false, fmt.Errorf("RSS video host window unavailable")
	}
	rect = normalizeListenEmbeddedVideoRect(rect)
	if rect.Sequence > 0 && rect.Sequence < player.embeddedSequence {
		player.mu.Unlock()
		return false, nil
	}
	if rect.Sequence > player.embeddedSequence {
		player.embeddedSequence = rect.Sequence
	}
	// Always retain the latest inline rect. It is restored after native
	// fullscreen before the frontend is told that geometry may resume.
	player.embeddedRect = rect
	player.embeddedVisible = true
	fullscreenOwnsPresentation := player.status.Fullscreen ||
		player.fullscreenTransition ||
		player.nativeWindowFullscreen
	player.mu.Unlock()
	if fullscreenOwnsPresentation {
		return true, nil
	}
	if fullscreenOwnsPresentation, known := listenNativeEmbeddedVideoFullscreenOwnsPresentation(window.NativeWindow()); known && fullscreenOwnsPresentation {
		return true, nil
	}
	// React owns the controls; the Bilibili document is a video-only surface.
	// Its injected stylesheet also disables pointer interaction in the page.
	rect.Interactive = false
	owner := listenClaimEmbeddedVideoOwner(window)
	shown := rssShowNativeEmbeddedWebViewForOwner(
		owner,
		window,
		player.windows.mainWindow,
		rect,
	)
	if !shown {
		listenReleaseEmbeddedVideoOwner(owner)
		player.mu.Lock()
		player.embeddedVisible = false
		player.mu.Unlock()
		return false, fmt.Errorf("embedded RSS Bilibili video unavailable")
	}
	return listenEmbeddedVideoRevealReady(
		shown,
		true,
		listenEmbeddedVideoOwnerActive(owner),
	), nil
}

func (player *rssBilibiliVideoPlayer) Hide(sequence uint64) (bool, error) {
	if player == nil {
		return false, nil
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if sequence > 0 && sequence < player.embeddedSequence {
		player.mu.Unlock()
		return false, nil
	}
	if sequence > player.embeddedSequence {
		player.embeddedSequence = sequence
	}
	window := player.window
	if window == nil {
		player.mu.Unlock()
		return false, nil
	}
	wasVisible := player.embeddedVisible
	sessionID := player.status.SessionID
	nativeFullscreen := player.nativeWindowFullscreen &&
		listenEmbeddedVideoUsesNativeWindowFullscreen()
	elementFullscreen := !nativeFullscreen &&
		(player.status.Fullscreen || player.fullscreenTransition)
	generation := player.fullscreenGeneration
	player.embeddedVisible = false
	player.embeddedRect = ListenEmbeddedVideoRect{}
	player.mu.Unlock()
	if nativeFullscreen {
		exitErr := player.exitNativeWindowFullscreenLocked(window, sessionID)
		if window.IsFullscreen() {
			// Never reparent a WebView while the OS still owns its fullscreen
			// presentation. A delayed unfullscreen hook completes cleanup.
			window.UnFullscreen()
			return wasVisible, exitErr
		}
		player.restoreAfterNativeWindowFullscreenLocked(window, generation)
		return wasVisible, exitErr
	}
	if elementFullscreen {
		_ = requestListenEmbeddedVideoFullscreenForSession(
			window,
			rssBilibiliPlayerSource,
			sessionID,
			&player.fullscreen,
			false,
		)
	}
	player.mu.Lock()
	if player.window == window && player.status.SessionID == sessionID {
		player.status.Fullscreen = false
		player.fullscreenTransition = false
	}
	player.mu.Unlock()
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	window.Hide()
	return wasVisible, nil
}

func (player *rssBilibiliVideoPlayer) Close(sessionID string) error {
	if player == nil {
		return nil
	}
	player.lifecycleMu.Lock()
	defer player.lifecycleMu.Unlock()
	player.closeCurrentWindow(sessionID)
	return nil
}

func (player *rssBilibiliVideoPlayer) closeCurrentWindow(sessionID string) bool {
	wantedSession := strings.TrimSpace(sessionID)
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if wantedSession != "" && wantedSession != player.status.SessionID {
		player.mu.Unlock()
		return false
	}
	window := player.window
	bridgeHook := player.bridgeHook
	waiter := player.nativeFullscreenWaiter
	player.window = nil
	player.bridgeHook = nil
	player.playerURL = ""
	player.authenticated = false
	player.embeddedVisible = false
	player.embeddedRect = ListenEmbeddedVideoRect{}
	player.embeddedSequence = 0
	player.fullscreenGeneration += 1
	player.fullscreenTransition = false
	player.nativeWindowFullscreen = false
	player.nativeFullscreenWaiter = nil
	player.status = newRSSVideoPlayerStatus()
	player.mu.Unlock()
	completeListenNativeFullscreenWaiter(waiter, false)
	if bridgeHook != nil {
		bridgeHook()
	}
	if window != nil {
		if window.IsFullscreen() {
			window.UnFullscreen()
			deadline := time.Now().Add(3 * time.Second)
			for window.IsFullscreen() && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
		}
		listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
		hideListenNativeEmbeddedWebView(window.NativeWindow())
		releaseRSSVideoPlayerWindowFeatures(window)
		window.Close()
	}
	return window != nil
}

func (player *rssBilibiliVideoPlayer) handlePlayerWindowClose(
	window *application.WebviewWindow,
) {
	if player == nil || window == nil {
		return
	}
	player.mu.Lock()
	if player.window != window {
		player.mu.Unlock()
		return
	}
	sessionID := player.status.SessionID
	player.mu.Unlock()
	_ = player.Close(sessionID)
}

func (player *rssBilibiliVideoPlayer) createWindow(
	bridgeScript string,
) (*application.WebviewWindow, func(), error) {
	if player.app == nil {
		return nil, nil, fmt.Errorf("RSS video player application unavailable")
	}
	window := player.app.Window.NewWithOptions(withRemoteWebViewPermissionPolicy(application.WebviewWindowOptions{
		Name:                       rssBilibiliPlayerWindowName,
		Title:                      "RSS Bilibili Video",
		Width:                      960,
		Height:                     540,
		MinWidth:                   320,
		MinHeight:                  180,
		URL:                        rssBilibiliPlayerBlankURL,
		Hidden:                     true,
		Frameless:                  true,
		DefaultContextMenuDisabled: true,
		BackgroundColour:           application.NewRGBA(0, 0, 0, 255),
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{
				FullscreenEnabled: application.Enabled,
			},
		},
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
			Permissions: map[application.CoreWebView2PermissionKind]application.CoreWebView2PermissionState{
				remoteMediaWebViewAutoplayPermissionKind: application.CoreWebView2PermissionStateAllow,
			},
		},
	}))
	if window == nil {
		return nil, nil, fmt.Errorf("failed to create RSS Bilibili player window")
	}
	registerWebViewRemoteCapabilityPolicy(window)
	bridgeHook, installed := attachRSSVideoPlayerDocumentStartBridge(window, bridgeScript)
	if !installed {
		releaseRSSVideoPlayerWindowFeatures(window)
		window.Close()
		return nil, nil, fmt.Errorf("failed to install RSS Bilibili document-start bridge")
	}
	fullscreenHook := window.RegisterHook(events.Common.WindowFullscreen, func(_ *application.WindowEvent) {
		player.handleNativeWindowFullscreenEvent(window, true)
	})
	unfullscreenHook := window.RegisterHook(events.Common.WindowUnFullscreen, func(_ *application.WindowEvent) {
		player.handleNativeWindowFullscreenEvent(window, false)
	})
	closingHook := window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		go player.handlePlayerWindowClose(window)
	})
	escapeHook := installRSSVideoPlayerNativeFullscreenEscape(window)
	cleanup := func() {
		if bridgeHook != nil {
			bridgeHook()
		}
		if fullscreenHook != nil {
			fullscreenHook()
		}
		if unfullscreenHook != nil {
			unfullscreenHook()
		}
		if closingHook != nil {
			closingHook()
		}
		if escapeHook != nil {
			escapeHook()
		}
	}
	return window, cleanup, nil
}

func (player *rssBilibiliVideoPlayer) controlWindow(sessionID string) (*application.WebviewWindow, error) {
	if player == nil {
		return nil, fmt.Errorf("RSS video player unavailable")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("RSS Bilibili video session id is required")
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.window == nil || player.status.SessionID == "" {
		return nil, fmt.Errorf("RSS Bilibili video is not prepared")
	}
	if player.status.SessionID != sessionID {
		return nil, fmt.Errorf("RSS Bilibili video session is no longer active")
	}
	return player.window, nil
}

func (player *rssBilibiliVideoPlayer) hideLocked(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	window.Hide()
	player.embeddedVisible = false
}

func (player *rssBilibiliVideoPlayer) HandleRawMessage(
	window application.Window,
	message string,
	originInfo *application.OriginInfo,
) bool {
	if player == nil || window == nil {
		return false
	}
	return player.handleRawMessage(window.Name(), window.ID(), message, originInfo)
}

func (player *rssBilibiliVideoPlayer) handleRawMessage(
	windowName string,
	windowID uint,
	message string,
	originInfo *application.OriginInfo,
) bool {
	if windowName != rssBilibiliPlayerWindowName {
		return false
	}
	// This bridge is reachable from a remote publisher document. Reject an
	// untrusted or oversized message before JSON decoding can amplify it into a
	// large native allocation on the application's main process.
	if originInfo == nil || len(message) == 0 || len(message) > maxRSSBilibiliRawMessageBytes {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return false
	}
	if listenPayloadString(payload, "source") != rssBilibiliPlayerSource ||
		!rssBilibiliRawMessageOriginTrusted(originInfo, listenPayloadBool(payload, "mainFrame")) {
		return false
	}

	player.mu.Lock()
	activeWindow := player.window
	activeSessionID := player.status.SessionID
	player.mu.Unlock()
	if activeWindow == nil || windowID != activeWindow.ID() {
		return false
	}
	sessionID := listenPayloadString(payload, "sessionId")
	if sessionID == "" || sessionID != activeSessionID {
		// The message belongs to this isolated player window but not to its
		// current React owner. Consume it without mutating the newer session.
		return true
	}

	switch listenPayloadString(payload, "type") {
	case listenEmbeddedVideoFullscreenResultType:
		requestID, _ := listenPayloadUint64(payload, "requestId")
		player.fullscreen.complete(requestID, listenEmbeddedVideoFullscreenResult{
			succeeded: listenPayloadBool(payload, "succeeded"),
			message:   listenPayloadString(payload, "message"),
		})
		return true
	case listenEmbeddedVideoFullscreenChangeType:
		player.updateFullscreenStatus(sessionID, listenPayloadBool(payload, "active"))
		return true
	case rssBilibiliFullscreenExitRequestType:
		// The Bilibili control bar lives inside the native fullscreen WebView.
		// Route its fullscreen button and Escape key back through the same
		// session-scoped native exit used by the React transport.
		go func() { _ = player.ExitFullscreen(sessionID) }()
		return true
	case rssBilibiliIdentityViolationType:
		// A same-document Bilibili route can replace the media element without
		// crossing the native top-level navigation delegate. Tear down the
		// authenticated session after its exact window/origin/session provenance
		// has been verified above; pausing one media node is not sufficient.
		go func() {
			_ = player.ExitFullscreen(sessionID)
			_ = player.Close(sessionID)
		}()
		return true
	case "state":
		player.updatePlaybackStatus(sessionID, payload)
		return true
	default:
		return true
	}
}

func (player *rssBilibiliVideoPlayer) updateFullscreenStatus(sessionID string, active bool) {
	player.mu.Lock()
	if player.status.SessionID != sessionID {
		player.mu.Unlock()
		return
	}
	window := player.window
	nativeWindowFullscreen := player.nativeWindowFullscreen
	if player.status.Fullscreen == active && !player.fullscreenTransition {
		player.mu.Unlock()
		if !active && !nativeWindowFullscreen && window != nil {
			execListenYouTubeMusicJS(window, rssBilibiliBooleanCommandScript(
				sessionID,
				"fullscreenPresentation",
				false,
			))
		}
		return
	}
	player.status.Fullscreen = active
	player.fullscreenTransition = false
	status := cloneRSSVideoPlayerStatus(player.status)
	player.mu.Unlock()
	if !active && !nativeWindowFullscreen && window != nil {
		execListenYouTubeMusicJS(window, rssBilibiliBooleanCommandScript(
			sessionID,
			"fullscreenPresentation",
			false,
		))
	}
	player.dispatchStatus(status)
}

func (player *rssBilibiliVideoPlayer) updatePlaybackStatus(sessionID string, payload map[string]any) {
	player.mu.Lock()
	if player.status.SessionID != sessionID {
		player.mu.Unlock()
		return
	}
	if state := normalizeRSSBilibiliPlaybackState(listenPayloadString(payload, "state")); state != "" {
		player.status.State = state
	}
	if available, ok := listenPayloadBoolValue(payload, "available"); ok {
		player.status.Available = available
	}
	if title := listenPayloadString(payload, "title"); title != "" {
		player.status.Title = title
	}
	if publisher, present := rssBilibiliPayloadStringValue(payload, "publisher"); present {
		player.status.Publisher = publisher
	}
	if publishedAt, present := rssBilibiliPayloadStringValue(payload, "publishedAt"); present {
		player.status.PublishedAt = publishedAt
	}
	if viewCount, ok := listenPayloadFloat(payload, "viewCount"); ok && viewCount <= 9_007_199_254_740_991 {
		player.status.ViewCount = uint64(math.Floor(viewCount))
	}
	if likeCount, ok := listenPayloadFloat(payload, "likeCount"); ok && likeCount <= 9_007_199_254_740_991 {
		player.status.LikeCount = uint64(math.Floor(likeCount))
	}
	if currentTime, ok := listenPayloadFloat(payload, "currentTime"); ok {
		player.status.CurrentTime = currentTime
	}
	if duration, ok := listenPayloadFloat(payload, "duration"); ok {
		player.status.Duration = duration
	}
	if bufferedTime, ok := listenPayloadFloat(payload, "bufferedTime"); ok {
		player.status.BufferedTime = bufferedTime
	}
	if volume, ok := listenPayloadFloat(payload, "volume"); ok {
		player.status.Volume = math.Min(1, volume)
	}
	if muted, ok := listenPayloadBoolValue(payload, "muted"); ok {
		player.status.Muted = muted
	}
	// Fullscreen state is owned by the session-scoped fullscreenchange/native
	// window hooks. Periodic media snapshots must not end a transition early.
	if rate, ok := listenPayloadFloat(payload, "playbackRate"); ok && rate > 0 {
		player.status.PlaybackRate = rate
		player.status.Selections.PlaybackRateID = rssBilibiliPlaybackRateID(rate)
	}
	if controls, ok := payload["controls"].(map[string]any); ok {
		player.status.Controls = RSSVideoPlayerControls{
			PlayPause:    listenPayloadBool(controls, "playPause"),
			Seek:         listenPayloadBool(controls, "seek"),
			Volume:       listenPayloadBool(controls, "volume"),
			PlaybackRate: listenPayloadBool(controls, "playbackRate"),
			Fullscreen:   listenPayloadBool(controls, "fullscreen"),
			Captions:     listenPayloadBool(controls, "captions"),
			Quality:      listenPayloadBool(controls, "quality"),
			Danmaku:      listenPayloadBool(controls, "danmaku"),
		}
	}
	if options, ok := rssBilibiliPayloadOptions(payload, "captionOptions"); ok {
		player.status.CaptionOptions = options
	}
	if options, ok := rssBilibiliPayloadOptions(payload, "qualityOptions"); ok {
		player.status.QualityOptions = options
	}
	if enabled, ok := listenPayloadBoolValue(payload, "danmakuEnabled"); ok {
		player.status.DanmakuEnabled = enabled
	}
	if selections, ok := payload["selections"].(map[string]any); ok {
		if captionID, present := rssBilibiliPayloadStringValue(selections, "captionId"); present {
			player.status.Selections.CaptionID = captionID
		}
		if qualityID, present := rssBilibiliPayloadStringValue(selections, "qualityId"); present {
			player.status.Selections.QualityID = qualityID
		}
	}
	if errorMessage := listenPayloadString(payload, "errorMessage"); errorMessage != "" {
		player.status.ErrorMessage = errorMessage
		player.status.State = "error"
	} else if player.status.State != "error" {
		player.status.ErrorMessage = ""
	}
	status := cloneRSSVideoPlayerStatus(player.status)
	player.mu.Unlock()
	player.dispatchStatus(status)
}

func (player *rssBilibiliVideoPlayer) dispatchStatus(status RSSVideoPlayerStatus) {
	if player == nil || player.windows == nil || status.SessionID == "" {
		return
	}
	player.windows.dispatchWindowEvent(rssBilibiliPlayerEventName, status)
}

func newRSSVideoPlayerStatus() RSSVideoPlayerStatus {
	return RSSVideoPlayerStatus{
		Provider:            applicationrss.BilibiliVideoPlatform,
		State:               "idle",
		Volume:              1,
		PlaybackRate:        1,
		PlaybackRateOptions: rssBilibiliPlaybackRateOptions(),
		Selections: RSSVideoPlayerSelections{
			PlaybackRateID: "1",
			CaptionID:      "",
		},
	}
}

func cloneRSSVideoPlayerStatus(status RSSVideoPlayerStatus) RSSVideoPlayerStatus {
	status.PlaybackRateOptions = append([]RSSVideoPlayerOption(nil), status.PlaybackRateOptions...)
	status.CaptionOptions = append([]RSSVideoPlayerOption(nil), status.CaptionOptions...)
	status.QualityOptions = append([]RSSVideoPlayerOption(nil), status.QualityOptions...)
	return status
}

func nextRSSBilibiliPlayerSessionID() string {
	return fmt.Sprintf(
		"rss-bilibili-%d-%d",
		time.Now().UnixNano(),
		rssBilibiliPlayerSessionCounter.Add(1),
	)
}

func rssBilibiliPlaybackRateOptions() []RSSVideoPlayerOption {
	return []RSSVideoPlayerOption{
		{ID: "0.5", Label: "0.5×"},
		{ID: "0.75", Label: "0.75×"},
		{ID: "1", Label: "1×"},
		{ID: "1.25", Label: "1.25×"},
		{ID: "1.5", Label: "1.5×"},
		{ID: "2", Label: "2×"},
	}
}

func rssBilibiliPlaybackRateAllowed(rate float64) bool {
	if math.IsNaN(rate) || math.IsInf(rate, 0) {
		return false
	}
	for _, option := range rssBilibiliPlaybackRateOptions() {
		parsed, _ := strconv.ParseFloat(option.ID, 64)
		if math.Abs(parsed-rate) < 0.000001 {
			return true
		}
	}
	return false
}

func rssBilibiliPlaybackRateID(rate float64) string {
	for _, option := range rssBilibiliPlaybackRateOptions() {
		parsed, _ := strconv.ParseFloat(option.ID, 64)
		if math.Abs(parsed-rate) < 0.000001 {
			return option.ID
		}
	}
	return strconv.FormatFloat(rate, 'g', -1, 64)
}

func normalizeRSSBilibiliSelectionValue(kind string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("RSS Bilibili %s selection is required", kind)
	}
	if len(value) > 256 || strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("RSS Bilibili %s selection is invalid", kind)
	}
	return value, nil
}

func normalizeRSSBilibiliCaptionSelectionValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > 256 || strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("RSS Bilibili caption selection is invalid")
	}
	return value, nil
}

func rssBilibiliPayloadStringValue(payload map[string]any, key string) (string, bool) {
	value, exists := payload[key]
	if !exists {
		return "", false
	}
	text, ok := value.(string)
	if !ok || len(text) > 256 || strings.ContainsRune(text, '\x00') {
		return "", false
	}
	return strings.TrimSpace(text), true
}

func rssBilibiliPayloadOptions(payload map[string]any, key string) ([]RSSVideoPlayerOption, bool) {
	raw, exists := payload[key]
	if !exists {
		return nil, false
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	options := make([]RSSVideoPlayerOption, 0, min(len(items), 128))
	seen := make(map[string]struct{}, min(len(items), 128))
	for _, item := range items {
		if len(options) >= 128 {
			break
		}
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, idOK := rssBilibiliPayloadStringValue(entry, "id")
		label, labelOK := rssBilibiliPayloadStringValue(entry, "label")
		if !idOK || id == "" || len(id) > 128 {
			continue
		}
		if !labelOK || label == "" {
			label = id
		}
		if len(label) > 256 {
			label = label[:256]
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		options = append(options, RSSVideoPlayerOption{ID: id, Label: label})
	}
	return options, true
}

func normalizeRSSBilibiliPlaybackState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "idle", "loading", "buffering", "playing", "paused", "ended", "error":
		return strings.ToLower(strings.TrimSpace(state))
	default:
		return ""
	}
}

func rssBilibiliRawMessageOriginTrusted(originInfo *application.OriginInfo, mainFrameMarker bool) bool {
	if originInfo == nil || !mainFrameMarker {
		return false
	}
	origin, ok := rssBilibiliNormalizedOrigin(originInfo.Origin)
	if !ok || origin != rssBilibiliPlayerOrigin {
		// In particular, never consume bridge-looking messages emitted by the
		// initial about:blank document before the trusted player is ready.
		return false
	}
	topOrigin := ""
	if strings.TrimSpace(originInfo.TopOrigin) != "" {
		var topOK bool
		topOrigin, topOK = rssBilibiliNormalizedOrigin(originInfo.TopOrigin)
		if !topOK || topOrigin != rssBilibiliPlayerOrigin {
			return false
		}
	}
	if originInfo.IsMainFrame {
		return true
	}
	// The Windows raw WebView2 message path may not populate
	// OriginInfo.IsMainFrame. Its callback does provide both sender and
	// top-level sources, so the
	// same-origin equality plus the bridge's window.top marker is the strict
	// main-frame equivalent on Windows. Platforms without either proof fail
	// closed.
	return topOrigin != "" && topOrigin == origin
}

func rssBilibiliNormalizedOrigin(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.User != nil || parsed.Opaque != "" ||
		!strings.EqualFold(parsed.Scheme, "https") ||
		!strings.EqualFold(parsed.Hostname(), "www.bilibili.com") {
		return "", false
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", false
	}
	return rssBilibiliPlayerOrigin, true
}
