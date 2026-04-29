package browsercdp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	pagepkg "github.com/chromedp/cdproto/page"
	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"go.uber.org/zap"

	appcookies "xiadown/internal/application/cookies"
)

const (
	defaultSnapshotLimit         = 200
	defaultSSRFValidationTimeout = 2 * time.Second
)

var lookupIPAddrsForHost = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

type SSRFPolicy struct {
	DangerouslyAllowPrivateNetwork bool
	AllowedHostnames               map[string]struct{}
	HostnameAllowlist              []string
}

type SessionOptions struct {
	SessionKey       string
	ProfileName      string
	PreferredBrowser string
	Headless         bool
	UserDataDir      string
	SSRFRules        SSRFPolicy
	Cookies          ConnectorCookieProvider
}

type SessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]map[string]*Session
}

type Session struct {
	mu sync.Mutex

	options SessionOptions
	runtime *Runtime

	tabs         map[string]*sessionTab
	activeTarget string

	pendingDialogs map[string]PendingDialog
	cookieSync     map[string]string
}

type sessionTab struct {
	TargetID string
	ctx      context.Context
	cancel   context.CancelFunc

	mu                sync.RWMutex
	cleanupCancels    []context.CancelFunc
	refs              map[string]snapshotRef
	evaluateResult    any
	lastURL           string
	title             string
	lastState         *PageState
	stateVersion      uint64
	nextRefID         uint64
	blockedRequestErr string
	fetchEnabled      bool
}

type newTabWaiter struct {
	ctx    context.Context
	ids    <-chan targetpkg.ID
	cancel context.CancelFunc
	stop   func() bool
}

func (waiter *newTabWaiter) close() {
	if waiter == nil {
		return
	}
	if waiter.stop != nil {
		waiter.stop()
	}
	if waiter.cancel != nil {
		waiter.cancel()
	}
}

type snapshotRef struct {
	Selector string
	Role     string
	Name     string
	Nth      int
}

func clearBlockedRequestError(tab *sessionTab) {
	if tab == nil {
		return
	}
	tab.mu.Lock()
	tab.blockedRequestErr = ""
	tab.mu.Unlock()
}

func setBlockedRequestError(tab *sessionTab, err error) {
	if tab == nil || err == nil {
		return
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return
	}
	tab.mu.Lock()
	if tab.blockedRequestErr == "" {
		tab.blockedRequestErr = message
	}
	tab.mu.Unlock()
}

func consumeBlockedRequestError(tab *sessionTab) error {
	if tab == nil {
		return nil
	}
	tab.mu.Lock()
	message := strings.TrimSpace(tab.blockedRequestErr)
	tab.blockedRequestErr = ""
	tab.mu.Unlock()
	if message == "" {
		return nil
	}
	return errors.New(message)
}

type snapshotCapture struct {
	Version      uint64
	URL          string
	Title        string
	Items        []SnapshotItem
	Refs         map[string]snapshotRef
	Truncated    bool
	ViewportOnly bool
}

type SnapshotItem struct {
	Ref   string `json:"ref,omitempty"`
	Role  string `json:"role,omitempty"`
	Name  string `json:"name,omitempty"`
	Text  string `json:"text,omitempty"`
	Depth int    `json:"depth,omitempty"`
	Nth   int    `json:"nth,omitempty"`
}

type PageState struct {
	Version      uint64         `json:"version"`
	URL          string         `json:"url"`
	Title        string         `json:"title,omitempty"`
	Items        []SnapshotItem `json:"items"`
	ItemCount    int            `json:"itemCount"`
	Truncated    bool           `json:"truncated"`
	ViewportOnly bool           `json:"viewportOnly"`
	CapturedAt   string         `json:"capturedAt"`
}

type PendingDialog struct {
	Message   string    `json:"message,omitempty"`
	Type      string    `json:"type,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ScrollDelta struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type ActionResult struct {
	OK               bool           `json:"ok"`
	TargetID         string         `json:"targetId,omitempty"`
	URL              string         `json:"url,omitempty"`
	Title            string         `json:"title,omitempty"`
	StateVersion     uint64         `json:"stateVersion,omitempty"`
	State            *PageState     `json:"state,omitempty"`
	Items            []SnapshotItem `json:"items,omitempty"`
	Action           string         `json:"action,omitempty"`
	OpenedNewTab     bool           `json:"openedNewTab,omitempty"`
	PreviousTargetID string         `json:"previousTargetId,omitempty"`
	PreviousURL      string         `json:"previousURL,omitempty"`
	Navigated        bool           `json:"navigated,omitempty"`
	Waited           bool           `json:"waited,omitempty"`
	Scroll           *ScrollDelta   `json:"scroll,omitempty"`
	Paths            []string       `json:"paths,omitempty"`
	Result           any            `json:"result,omitempty"`
	Pending          *PendingDialog `json:"pending,omitempty"`
	StateAvailable   bool           `json:"stateAvailable"`
	StateError       string         `json:"stateError,omitempty"`
	Reset            bool           `json:"reset,omitempty"`
	Restarted        bool           `json:"restarted,omitempty"`
	Ready            bool           `json:"ready,omitempty"`
	Closed           bool           `json:"closed,omitempty"`
}

type CommandOptions struct {
	Limit   int
	Timeout time.Duration
	WaitFor *WaitRequest
}

type WaitRequest struct {
	Time     time.Duration
	Selector string
	Text     string
	TextGone string
	URL      string
	Fn       string
	Timeout  time.Duration
}

type ScrollRequest struct {
	TargetID string
	Ref      string
	DeltaX   int
	DeltaY   int
	Limit    int
	Timeout  time.Duration
}

type UploadRequest struct {
	TargetID string
	Ref      string
	Paths    []string
	Limit    int
	Timeout  time.Duration
}

type DialogRequest struct {
	TargetID   string
	Accept     *bool
	PromptText string
	Limit      int
	Timeout    time.Duration
}

type ActRequest struct {
	Kind       string
	TargetID   string
	Ref        string
	Text       string
	Key        string
	Value      string
	Expression string
	Width      int
	Height     int
	Wait       WaitRequest
	WaitFor    *WaitRequest
	Limit      int
	Timeout    time.Duration
}

type FatalError struct {
	Err error
}

func (err *FatalError) Error() string {
	if err == nil || err.Err == nil {
		return "browser runtime unavailable"
	}
	return err.Err.Error()
}

func (err *FatalError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

type InvalidRefError struct {
	Ref string
}

func (err *InvalidRefError) Error() string {
	if err == nil || strings.TrimSpace(err.Ref) == "" {
		return "ref not found; run snapshot again to get fresh refs"
	}
	return fmt.Sprintf("ref %q not found; run snapshot again to get fresh refs", strings.TrimSpace(err.Ref))
}

type ConnectorCookieError struct {
	URL string
	Err error
}

func (err *ConnectorCookieError) Error() string {
	if err == nil || err.Err == nil {
		return "connector cookie sync failed"
	}
	return fmt.Sprintf("connector cookie sync failed for %s: %v", strings.TrimSpace(err.URL), err.Err)
}

func (err *ConnectorCookieError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

type WaitTimeoutError struct {
	Condition string
}

func (err *WaitTimeoutError) Error() string {
	if err == nil || strings.TrimSpace(err.Condition) == "" {
		return "wait timeout"
	}
	return fmt.Sprintf("wait %s timeout", strings.TrimSpace(err.Condition))
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: map[string]map[string]*Session{},
	}
}

func (registry *SessionRegistry) GetOrCreate(sessionKey string, profileName string, options SessionOptions) *Session {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		sessionKey = "default"
	}
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		profileName = "xiadown"
	}
	bucket, ok := registry.sessions[sessionKey]
	if !ok {
		bucket = map[string]*Session{}
		registry.sessions[sessionKey] = bucket
	}
	session, ok := bucket[profileName]
	if !ok {
		options.SessionKey = sessionKey
		options.ProfileName = profileName
		session = &Session{
			options:        normalizeSessionOptions(options),
			tabs:           map[string]*sessionTab{},
			pendingDialogs: map[string]PendingDialog{},
			cookieSync:     map[string]string{},
		}
		bucket[profileName] = session
		return session
	}
	session.mu.Lock()
	session.options = normalizeSessionOptions(options)
	session.mu.Unlock()
	return session
}

func (registry *SessionRegistry) CloseSessionKey(sessionKey string) {
	if registry == nil {
		return
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		sessionKey = "default"
	}
	registry.mu.Lock()
	bucket := registry.sessions[sessionKey]
	delete(registry.sessions, sessionKey)
	sessions := make([]*Session, 0, len(bucket))
	for _, session := range bucket {
		if session != nil {
			sessions = append(sessions, session)
		}
	}
	registry.mu.Unlock()
	for _, session := range sessions {
		session.stop()
	}
}

func (registry *SessionRegistry) CloseAll() {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	sessions := make([]*Session, 0)
	for sessionKey, bucket := range registry.sessions {
		delete(registry.sessions, sessionKey)
		for _, session := range bucket {
			if session != nil {
				sessions = append(sessions, session)
			}
		}
	}
	registry.mu.Unlock()
	for _, session := range sessions {
		session.stop()
	}
}

func IsFatalError(err error) bool {
	if err == nil {
		return false
	}
	var fatal *FatalError
	if errors.As(err, &fatal) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "browser runtime unavailable"),
		strings.Contains(message, "context canceled"),
		strings.Contains(message, "target closed"),
		strings.Contains(message, "connection closed"),
		strings.Contains(message, "websocket"),
		strings.Contains(message, "session closed"),
		strings.Contains(message, "browser session reset"):
		return true
	default:
		return false
	}
}

func (session *Session) Reset(restart bool) (ActionResult, error) {
	session.stop()
	if restart {
		if err := session.ensureStarted(); err != nil {
			return ActionResult{}, session.wrapError(err)
		}
	}
	return ActionResult{
		OK:        true,
		Reset:     true,
		Restarted: restart,
		Ready:     restart,
	}, nil
}

func (session *Session) Open(ctx context.Context, targetURL string, options CommandOptions) (ActionResult, error) {
	if err := session.assertURLAllowed(targetURL); err != nil {
		zap.L().Warn(
			"browser open blocked by url policy",
			append(session.logFields(),
				sanitizedURLField("url", targetURL),
				zap.Error(err),
			)...,
		)
		return ActionResult{}, err
	}
	if err := session.ensureStarted(); err != nil {
		zap.L().Warn(
			"browser open start runtime failed",
			append(session.logFields(),
				sanitizedURLField("url", targetURL),
				zap.Error(err),
			)...,
		)
		return ActionResult{}, session.wrapError(err)
	}
	cookiesStartedAt := time.Now()
	if err := session.ensureCookiesForURLOnBrowser(ctx, targetURL); err != nil {
		if isRecoverableCookieSyncError(err) {
			zap.L().Warn(
				"browser open cookie sync failed; retrying on fresh runtime",
				append(session.logFields(),
					sanitizedURLField("url", targetURL),
					zap.Error(err),
				)...,
			)
			session.stop()
			if retryErr := session.ensureStarted(); retryErr == nil {
				err = session.ensureCookiesForURLOnBrowser(ctx, targetURL)
			} else {
				err = retryErr
			}
		}
		zap.L().Warn(
			"browser open cookie sync failed",
			append(session.logFields(),
				sanitizedURLField("url", targetURL),
				zap.Duration("elapsed", time.Since(cookiesStartedAt).Round(time.Millisecond)),
				zap.Error(err),
			)...,
		)
		return ActionResult{}, err
	}
	createTabStartedAt := time.Now()
	tab, err := session.createTab()
	if err != nil {
		zap.L().Warn(
			"browser open create tab failed",
			append(session.logFields(),
				sanitizedURLField("url", targetURL),
				zap.Duration("elapsed", time.Since(createTabStartedAt).Round(time.Millisecond)),
				zap.Error(err),
			)...,
		)
		return ActionResult{}, session.wrapError(err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			zap.L().Warn(
				"browser open cleaning up failed tab",
				append(session.logFields(),
					sanitizedURLField("url", targetURL),
					zap.String("targetId", tab.TargetID),
				)...,
			)
			session.detachTab(tab.TargetID)
			cancelTabContexts(tab)
		}
	}()
	session.setActiveTarget(tab.TargetID)
	clearBlockedRequestError(tab)
	navigateTimeout := normalizeTimeout(options.Timeout, 30*time.Second)
	navigateStartedAt := time.Now()
	if err := session.openTab(tab, targetURL, navigateTimeout); err != nil {
		zap.L().Warn(
			"browser open navigation failed",
			append(session.logFields(),
				sanitizedURLField("url", targetURL),
				zap.String("targetId", tab.TargetID),
				zap.Duration("elapsed", time.Since(navigateStartedAt).Round(time.Millisecond)),
				zap.Error(err),
			)...,
		)
		return ActionResult{}, session.wrapError(err)
	}
	cleanup = false
	if options.WaitFor != nil {
		waitTimeout := normalizeTimeout(options.WaitFor.Timeout, options.Timeout)
		waitStartedAt := time.Now()
		if err := session.waitOnTab(ctx, tab, *options.WaitFor, waitTimeout); err != nil {
			if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
				return ActionResult{}, blockedErr
			}
			zap.L().Warn(
				"browser open wait failed",
				append(session.logFields(),
					sanitizedURLField("url", targetURL),
					zap.String("targetId", tab.TargetID),
					zap.Duration("elapsed", time.Since(waitStartedAt).Round(time.Millisecond)),
					zap.Error(err),
				)...,
			)
			return ActionResult{}, session.wrapError(err)
		}
	}
	if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
		return ActionResult{}, blockedErr
	}
	stateTimeout := captureTimeout(options.Timeout)
	stateStartedAt := time.Now()
	result, err := session.collectActionResult(tab, options.Limit, stateTimeout, false)
	if err != nil {
		zap.L().Warn(
			"browser open state capture failed",
			append(session.logFields(),
				sanitizedURLField("url", targetURL),
				zap.String("targetId", tab.TargetID),
				zap.Duration("elapsed", time.Since(stateStartedAt).Round(time.Millisecond)),
				zap.Error(err),
			)...,
		)
		return ActionResult{}, session.wrapError(err)
	}
	result.OpenedNewTab = true
	result.URL = preferredPageURL(result.URL, tabURL(tab), targetURL)
	if result.State != nil {
		result.State.URL = preferredPageURL(result.State.URL, result.URL)
	}
	return result, nil
}

func (session *Session) Navigate(ctx context.Context, targetID string, targetURL string, newTab bool, options CommandOptions) (ActionResult, error) {
	navigateStartedAt := time.Now()
	if err := session.assertURLAllowed(targetURL); err != nil {
		return ActionResult{}, err
	}
	if newTab {
		return session.Open(ctx, targetURL, options)
	}
	if err := session.ensureStarted(); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	tab, err := session.resolveTab(targetID, true)
	if err != nil {
		if targetID == "" && errors.Is(err, errNoOpenTab) {
			return session.Open(ctx, targetURL, options)
		}
		return ActionResult{}, session.wrapError(err)
	}
	clearBlockedRequestError(tab)
	if err := session.ensureCookiesForURL(ctx, tab, targetURL); err != nil {
		if isRecoverableCookieSyncError(err) {
			zap.L().Warn(
				"browser navigate cookie sync failed; restarting runtime and falling back to open",
				append(session.logFields(),
					zap.String("targetId", tab.TargetID),
					sanitizedURLField("url", targetURL),
					zap.Error(err),
				)...,
			)
			session.stop()
			recovered, recoveredErr := session.Open(ctx, targetURL, options)
			if recoveredErr == nil {
				recovered.OpenedNewTab = true
				return recovered, nil
			}
			return ActionResult{}, recoveredErr
		}
		return ActionResult{}, err
	}
	tab, err = session.navigateTab(tab, targetURL, normalizeTimeout(options.Timeout, 30*time.Second))
	if err != nil {
		currentTargetID := ""
		if tab != nil {
			currentTargetID = tab.TargetID
		}
		zap.L().Warn(
			"browser navigate failed before state capture",
			append(session.logFields(),
				zap.String("targetId", currentTargetID),
				sanitizedURLField("url", targetURL),
				zap.Duration("elapsed", time.Since(navigateStartedAt).Round(time.Millisecond)),
				zap.Error(err),
			)...,
		)
		return ActionResult{}, session.wrapError(wrapRuntimeHangError(err))
	}
	if options.WaitFor != nil {
		if err := session.waitOnTab(ctx, tab, *options.WaitFor, normalizeTimeout(options.WaitFor.Timeout, options.Timeout)); err != nil {
			if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
				return ActionResult{}, blockedErr
			}
			zap.L().Warn(
				"browser navigate wait failed",
				append(session.logFields(),
					zap.String("targetId", tab.TargetID),
					sanitizedURLField("url", targetURL),
					zap.Duration("elapsed", time.Since(navigateStartedAt).Round(time.Millisecond)),
					zap.Error(err),
				)...,
			)
			return ActionResult{}, session.wrapError(err)
		}
	}
	if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
		return ActionResult{}, blockedErr
	}
	stateTimeout := captureTimeout(options.Timeout)
	result, err := session.collectActionResult(tab, options.Limit, stateTimeout, false)
	if err != nil {
		zap.L().Warn(
			"browser navigate state capture failed",
			append(session.logFields(),
				zap.String("targetId", tab.TargetID),
				sanitizedURLField("url", targetURL),
				zap.Duration("elapsed", time.Since(navigateStartedAt).Round(time.Millisecond)),
				zap.Error(err),
			)...,
		)
		if isTargetLookupError(err) {
			zap.L().Warn(
				"browser navigate falling back to open after target loss",
				append(session.logFields(),
					zap.String("targetId", tab.TargetID),
					sanitizedURLField("url", targetURL),
				)...,
			)
			session.stop()
			recovered, recoveredErr := session.Open(ctx, targetURL, options)
			if recoveredErr == nil {
				recovered.OpenedNewTab = true
				return recovered, nil
			}
			zap.L().Warn(
				"browser navigate recovery open failed after runtime restart",
				append(session.logFields(),
					zap.String("targetId", tab.TargetID),
					sanitizedURLField("url", targetURL),
					zap.Error(recoveredErr),
				)...,
			)
		}
		return ActionResult{}, session.wrapError(err)
	}
	result = session.stabilizeActionResult(tab, result, options.Limit, stateTimeout, minDuration(2*time.Second, normalizeTimeout(options.Timeout, 30*time.Second)/2))
	if shouldRetryActionState(result) && strings.TrimSpace(result.URL) == strings.TrimSpace(targetURL) {
		zap.L().Warn(
			"browser navigate state remained unstable after stabilization; falling back to open",
			append(session.logFields(),
				zap.String("targetId", result.TargetID),
				sanitizedURLField("url", targetURL),
				zap.String("reason", actionStateReason(result)),
			)...,
		)
		fallback, openErr := session.Open(ctx, targetURL, options)
		if openErr == nil {
			fallback.OpenedNewTab = true
			return fallback, nil
		}
		session.stop()
		recovered, recoveredErr := session.Open(ctx, targetURL, options)
		if recoveredErr == nil {
			recovered.OpenedNewTab = true
			return recovered, nil
		}
	}
	result.OpenedNewTab = false
	return result, nil
}

func (session *Session) State(targetID string, limit int) (ActionResult, error) {
	if err := session.ensureStarted(); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	tab, err := session.resolveTab(targetID, true)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
		return ActionResult{}, blockedErr
	}
	result, err := session.collectActionResult(tab, limit, 10*time.Second, false)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	return result, nil
}

func (session *Session) Wait(ctx context.Context, targetID string, request WaitRequest, options CommandOptions) (ActionResult, error) {
	if err := session.ensureStarted(); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	tab, err := session.resolveTab(targetID, true)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	if err := session.ensureRequestInterception(tab); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	if err := session.waitOnTab(ctx, tab, request, normalizeTimeout(request.Timeout, options.Timeout)); err != nil {
		if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
			return ActionResult{}, blockedErr
		}
		return ActionResult{}, session.wrapError(err)
	}
	if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
		return ActionResult{}, blockedErr
	}
	result, err := session.collectActionResult(tab, options.Limit, captureTimeout(options.Timeout), false)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	result = session.stabilizeActionResult(tab, result, options.Limit, captureTimeout(options.Timeout), minDuration(1500*time.Millisecond, normalizeTimeout(options.Timeout, 15*time.Second)/2))
	result.Waited = true
	return result, nil
}

func (session *Session) Scroll(request ScrollRequest) (ActionResult, error) {
	if err := session.ensureStarted(); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	tab, err := session.resolveTab(request.TargetID, true)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	runCtx, cancel := context.WithTimeout(tab.ctx, normalizeTimeout(request.Timeout, 15*time.Second))
	defer cancel()
	if strings.TrimSpace(request.Ref) != "" {
		selector, err := resolveRefSelector(tab, request.Ref)
		if err != nil {
			return ActionResult{}, err
		}
		script := fmt.Sprintf(`(() => { const el = document.querySelector(%q); if (!el) throw new Error("element not found"); el.scrollIntoView({block: "center", inline: "center"}); if (%d !== 0 || %d !== 0) { el.scrollBy(%d, %d); } })()`, selector, request.DeltaX, request.DeltaY, request.DeltaX, request.DeltaY)
		if err := chromedp.Run(runCtx, chromedp.EvaluateAsDevTools(script, nil)); err != nil {
			return ActionResult{}, session.wrapError(err)
		}
	} else {
		script := fmt.Sprintf(`window.scrollBy(%d, %d)`, request.DeltaX, request.DeltaY)
		if err := chromedp.Run(runCtx, chromedp.EvaluateAsDevTools(script, nil)); err != nil {
			return ActionResult{}, session.wrapError(err)
		}
	}
	time.Sleep(200 * time.Millisecond)
	result, err := session.collectActionResult(tab, request.Limit, captureTimeout(request.Timeout), false)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	result = session.stabilizeActionResult(tab, result, request.Limit, captureTimeout(request.Timeout), minDuration(1500*time.Millisecond, normalizeTimeout(request.Timeout, 15*time.Second)/2))
	result.Scroll = &ScrollDelta{X: request.DeltaX, Y: request.DeltaY}
	return result, nil
}

func (session *Session) Upload(request UploadRequest) (ActionResult, error) {
	if err := session.ensureStarted(); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	tab, err := session.resolveTab(request.TargetID, true)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	selector, err := resolveRefSelector(tab, request.Ref)
	if err != nil {
		return ActionResult{}, err
	}
	if len(request.Paths) == 0 {
		return ActionResult{}, errors.New("paths are required")
	}
	runCtx, cancel := context.WithTimeout(tab.ctx, normalizeTimeout(request.Timeout, 15*time.Second))
	defer cancel()
	if err := chromedp.Run(runCtx, chromedp.SetUploadFiles(selector, request.Paths, chromedp.ByQuery)); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	clearTabState(tab)
	result, err := session.collectActionResult(tab, request.Limit, captureTimeout(request.Timeout), false)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	result = session.stabilizeActionResult(tab, result, request.Limit, captureTimeout(request.Timeout), minDuration(1500*time.Millisecond, normalizeTimeout(request.Timeout, 15*time.Second)/2))
	result.Paths = append([]string(nil), request.Paths...)
	return result, nil
}

func (session *Session) Dialog(request DialogRequest) (ActionResult, error) {
	if err := session.ensureStarted(); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	tab, err := session.resolveTab(request.TargetID, true)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	targetID := tab.TargetID
	session.mu.Lock()
	pending, exists := session.pendingDialogs[targetID]
	session.mu.Unlock()
	if !exists {
		return ActionResult{
			OK:       true,
			TargetID: targetID,
		}, nil
	}
	if request.Accept == nil {
		dialog := pending
		return ActionResult{
			OK:       true,
			TargetID: targetID,
			Pending:  &dialog,
		}, nil
	}
	runCtx, cancel := context.WithTimeout(tab.ctx, normalizeTimeout(request.Timeout, 15*time.Second))
	defer cancel()
	if err := chromedp.Run(runCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return pagepkg.HandleJavaScriptDialog(*request.Accept).WithPromptText(request.PromptText).Do(ctx)
	})); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	session.mu.Lock()
	delete(session.pendingDialogs, targetID)
	session.mu.Unlock()
	clearTabState(tab)
	result, err := session.collectActionResult(tab, request.Limit, captureTimeout(request.Timeout), false)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	result = session.stabilizeActionResult(tab, result, request.Limit, captureTimeout(request.Timeout), minDuration(1500*time.Millisecond, normalizeTimeout(request.Timeout, 15*time.Second)/2))
	return result, nil
}

func (session *Session) Act(ctx context.Context, request ActRequest) (ActionResult, error) {
	actStartedAt := time.Now()
	if err := session.ensureStarted(); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	tab, err := session.resolveTab(request.TargetID, true)
	if err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	if err := session.ensureRequestInterception(tab); err != nil {
		return ActionResult{}, session.wrapError(err)
	}
	previousTargetID := tab.TargetID
	previousURL := tabURL(tab)
	clearBlockedRequestError(tab)
	beforeTargets, _ := session.snapshotPageTargets()
	var newTabWaiter *newTabWaiter
	if actMayOpenNewTab(request.Kind) {
		newTabWaiter, err = session.prepareNewTabWaiter(ctx, tab)
		if err != nil {
			return ActionResult{}, session.wrapError(err)
		}
		defer newTabWaiter.close()
	}
	switch request.Kind {
	case "click":
		err = session.actClick(tab, request)
	case "type":
		err = session.actType(tab, request)
	case "press":
		err = session.actPress(tab, request)
	case "hover":
		err = session.actHover(tab, request)
	case "select":
		err = session.actSelect(tab, request)
	case "fill":
		err = session.actFill(tab, request)
	case "resize":
		err = session.actResize(tab, request)
	case "wait":
		err = session.waitOnTab(ctx, tab, request.Wait, normalizeTimeout(request.Wait.Timeout, request.Timeout))
	case "evaluate":
		err = session.actEvaluate(tab, request)
	case "close":
		_, err = session.closeTab(tab.TargetID, normalizeTimeout(request.Timeout, 15*time.Second))
	default:
		err = fmt.Errorf("act kind not supported: %s", strings.TrimSpace(request.Kind))
	}
	if err != nil {
		if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
			return ActionResult{}, blockedErr
		}
		zap.L().Warn(
			"browser act command failed",
			append(session.logFields(),
				zap.String("kind", strings.TrimSpace(request.Kind)),
				zap.String("targetId", tab.TargetID),
				zap.String("ref", strings.TrimSpace(request.Ref)),
				zap.Duration("elapsed", time.Since(actStartedAt).Round(time.Millisecond)),
				zap.Error(err),
			)...,
		)
		if request.Kind != "wait" {
			err = wrapRuntimeHangError(err)
		}
		return ActionResult{}, session.wrapError(err)
	}
	if request.Kind == "close" {
		return ActionResult{
			OK:       true,
			TargetID: previousTargetID,
			Closed:   true,
		}, nil
	}
	if actInvalidatesState(request.Kind) {
		clearTabState(tab)
	}
	currentTab := tab
	openedNewTab := false
	if newTabWaiter != nil {
		if detectedTab, ok := session.waitForNewTab(newTabWaiter, 1500*time.Millisecond); ok && detectedTab != nil {
			currentTab = detectedTab
			openedNewTab = detectedTab.TargetID != previousTargetID
		}
	}
	if request.WaitFor != nil {
		if err := session.waitOnTab(ctx, currentTab, *request.WaitFor, normalizeTimeout(request.WaitFor.Timeout, request.Timeout)); err != nil {
			zap.L().Warn(
				"browser act wait failed",
				append(session.logFields(),
					zap.String("kind", strings.TrimSpace(request.Kind)),
					zap.String("targetId", currentTab.TargetID),
					zap.String("ref", strings.TrimSpace(request.Ref)),
					zap.Duration("elapsed", time.Since(actStartedAt).Round(time.Millisecond)),
					zap.Error(err),
				)...,
			)
			return ActionResult{}, session.wrapError(err)
		}
	} else if actNeedsSettle(request.Kind) {
		if err := sleepWithContext(ctx, 250*time.Millisecond); err != nil {
			return ActionResult{}, session.wrapError(err)
		}
	}
	if request.WaitFor == nil {
		switch request.Kind {
		case "click", "press":
			if navigatedTab, observed, observeErr := session.observeActionNavigation(currentTab, beforeTargets, previousURL, 2*time.Second); observeErr != nil {
				return ActionResult{}, session.wrapError(observeErr)
			} else if observed && navigatedTab != nil {
				currentTab = navigatedTab
			}
		}
	}
	if blockedErr := consumeBlockedRequestError(currentTab); blockedErr != nil {
		return ActionResult{}, blockedErr
	}
	stateTimeout := captureTimeout(request.Timeout)
	result, err := session.collectActionResult(currentTab, request.Limit, stateTimeout, false)
	if err != nil {
		zap.L().Warn(
			"browser act state capture failed",
			append(session.logFields(),
				zap.String("kind", strings.TrimSpace(request.Kind)),
				zap.String("targetId", currentTab.TargetID),
				zap.Duration("elapsed", time.Since(actStartedAt).Round(time.Millisecond)),
				zap.Error(err),
			)...,
		)
		return ActionResult{}, session.wrapError(err)
	}
	result = session.stabilizeActionResult(currentTab, result, request.Limit, stateTimeout, minDuration(2*time.Second, normalizeTimeout(request.Timeout, 20*time.Second)/2))
	result.Action = request.Kind
	result.OpenedNewTab = openedNewTab
	result.PreviousTargetID = previousTargetID
	result.PreviousURL = previousURL
	result.Navigated = openedNewTab || !urlsEqual(previousURL, result.URL)
	if request.Kind == "evaluate" {
		result.Result = evaluateResult(tab)
	}
	return result, nil
}

var errNoOpenTab = errors.New("no browser tab is open")

func normalizeSessionOptions(options SessionOptions) SessionOptions {
	options.SessionKey = strings.TrimSpace(options.SessionKey)
	if options.SessionKey == "" {
		options.SessionKey = "default"
	}
	options.ProfileName = strings.TrimSpace(options.ProfileName)
	if options.ProfileName == "" {
		options.ProfileName = "xiadown"
	}
	options.PreferredBrowser = strings.ToLower(strings.TrimSpace(options.PreferredBrowser))
	if options.UserDataDir == "" {
		options.UserDataDir = ResolveProfileUserDataDir(options.SessionKey, options.ProfileName)
	}
	if options.SSRFRules.AllowedHostnames == nil {
		options.SSRFRules.AllowedHostnames = map[string]struct{}{}
	}
	return options
}

func (session *Session) optionsSnapshot() SessionOptions {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.options
}

func (session *Session) ensureStarted() error {
	session.mu.Lock()
	if session.runtime != nil && session.runtime.Status().Ready {
		session.mu.Unlock()
		return nil
	}
	options := session.options
	session.mu.Unlock()

	startCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runtime, err := Start(startCtx, LaunchOptions{
		PreferredBrowser: options.PreferredBrowser,
		Headless:         options.Headless,
		UserDataDir:      options.UserDataDir,
	})
	if err != nil {
		return err
	}
	session.mu.Lock()
	if session.runtime != nil && session.runtime.Status().Ready {
		session.mu.Unlock()
		runtime.Stop()
		return nil
	}
	session.runtime = runtime
	session.mu.Unlock()
	return nil
}

func (session *Session) stop() {
	session.mu.Lock()
	runtime := session.runtime
	tabs := make([]*sessionTab, 0, len(session.tabs))
	for _, tab := range session.tabs {
		tabs = append(tabs, tab)
	}
	session.runtime = nil
	session.tabs = map[string]*sessionTab{}
	session.activeTarget = ""
	session.pendingDialogs = map[string]PendingDialog{}
	session.cookieSync = map[string]string{}
	session.mu.Unlock()

	for _, tab := range tabs {
		cancelTabContexts(tab)
	}
	if runtime != nil {
		runtime.Stop()
	}
}

func (session *Session) wrapError(err error) error {
	if err == nil {
		return nil
	}
	if IsFatalError(err) {
		session.stop()
		if _, ok := err.(*FatalError); ok {
			return err
		}
		return &FatalError{Err: err}
	}
	return err
}

func (session *Session) createTab() (*sessionTab, error) {
	if err := session.ensureStarted(); err != nil {
		return nil, err
	}
	reusableTargetID, err := session.resolveReusableTargetID()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(reusableTargetID) != "" {
		return session.attachTargetAsTab(reusableTargetID, false)
	}
	targetID, err := session.createBlankTarget()
	if err != nil {
		return nil, err
	}
	return session.attachTargetAsTab(targetID, false)
}

func (session *Session) createBlankTarget() (string, error) {
	options := session.optionsSnapshot()
	runCtx, cancel, err := session.newBrowserExecutorContext(10 * time.Second)
	if err != nil {
		return "", err
	}
	defer cancel()

	createTarget := targetpkg.CreateTarget("about:blank")
	if !options.Headless {
		createTarget = createTarget.WithNewWindow(true)
	}
	createdTargetID, err := createTarget.Do(runCtx)
	if err != nil {
		return "", wrapRuntimeHangError(err)
	}
	targetID := strings.TrimSpace(string(createdTargetID))
	if targetID == "" {
		return "", errors.New("create target returned empty target id")
	}
	return targetID, nil
}

func (session *Session) openTab(tab *sessionTab, targetURL string, timeout time.Duration) error {
	if session == nil || tab == nil {
		return errors.New("tab unavailable")
	}
	if err := session.ensureRequestInterception(tab); err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(tab.ctx, normalizeTimeout(timeout, 30*time.Second))
	defer cancel()

	var currentURL string
	var title string
	if err := chromedp.Run(runCtx,
		chromedp.Navigate(strings.TrimSpace(targetURL)),
		chromedp.Location(&currentURL),
		chromedp.Title(&title),
	); err != nil {
		return wrapRuntimeHangError(err)
	}

	finalURL := preferredPageURL(currentURL, targetURL)
	if err := session.assertObservedURLAllowed(finalURL); err != nil {
		return err
	}
	storeNavigation(tab, finalURL)
	tab.mu.Lock()
	if title = strings.TrimSpace(title); title != "" {
		tab.title = title
	}
	tab.mu.Unlock()
	return nil
}

func (session *Session) attachTargetAsTab(targetID string, createNew bool) (*sessionTab, error) {
	if err := session.ensureStarted(); err != nil {
		return nil, err
	}
	session.mu.Lock()
	if trimmed := strings.TrimSpace(targetID); trimmed != "" {
		if existing, ok := session.tabs[trimmed]; ok {
			session.activeTarget = existing.TargetID
			session.mu.Unlock()
			return existing, nil
		}
	}
	runtime := session.runtime
	session.mu.Unlock()
	if runtime == nil {
		return nil, errors.New("browser runtime unavailable")
	}
	var (
		tabCtx context.Context
		cancel context.CancelFunc
	)
	trimmedTargetID := strings.TrimSpace(targetID)
	if !createNew && trimmedTargetID != "" {
		tabCtx, cancel = chromedp.NewContext(runtime.BrowserContext(), chromedp.WithTargetID(targetpkg.ID(trimmedTargetID)))
	} else {
		tabCtx, cancel = chromedp.NewContext(runtime.BrowserContext())
	}
	tab := &sessionTab{
		TargetID: trimmedTargetID,
		ctx:      tabCtx,
		cancel:   cancel,
		refs:     map[string]snapshotRef{},
	}
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		chromeCtx := chromedp.FromContext(ctx)
		if chromeCtx == nil || chromeCtx.Target == nil {
			return errors.New("tab target unavailable")
		}
		resolvedTargetID := string(chromeCtx.Target.TargetID)
		if strings.TrimSpace(resolvedTargetID) == "" {
			return errors.New("tab target unavailable")
		}
		if strings.TrimSpace(tab.TargetID) == "" {
			tab.TargetID = resolvedTargetID
			return nil
		}
		if !strings.EqualFold(strings.TrimSpace(tab.TargetID), strings.TrimSpace(resolvedTargetID)) {
			return fmt.Errorf("tab target mismatch: expected %s got %s", strings.TrimSpace(tab.TargetID), strings.TrimSpace(resolvedTargetID))
		}
		return nil
	})); err != nil {
		cancel()
		return nil, wrapRuntimeHangError(err)
	}
	activateCtx, activateCancel, activateErr := session.newBrowserExecutorContext(5 * time.Second)
	if activateErr == nil {
		_ = targetpkg.ActivateTarget(targetpkg.ID(tab.TargetID)).Do(activateCtx)
		activateCancel()
	}
	if err := session.attachTab(tab); err != nil {
		cancel()
		return nil, err
	}
	session.mu.Lock()
	session.tabs[tab.TargetID] = tab
	session.activeTarget = tab.TargetID
	session.mu.Unlock()
	return tab, nil
}

func (session *Session) attachTab(tab *sessionTab) error {
	chromedp.ListenTarget(tab.ctx, func(ev any) {
		switch event := ev.(type) {
		case *pagepkg.EventJavascriptDialogOpening:
			session.mu.Lock()
			session.pendingDialogs[tab.TargetID] = PendingDialog{
				Message:   strings.TrimSpace(event.Message),
				Type:      string(event.Type),
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}
			session.mu.Unlock()
		case *fetch.EventRequestPaused:
			go session.handlePausedRequest(tab, event)
		}
	})
	return nil
}

func (session *Session) enableRequestInterception(tab *sessionTab) error {
	return session.runOnTab(tab, 5*time.Second, chromedp.ActionFunc(func(ctx context.Context) error {
		return fetch.Enable().WithPatterns([]*fetch.RequestPattern{
			{
				URLPattern:   "*",
				RequestStage: fetch.RequestStageRequest,
			},
		}).Do(ctx)
	}))
}

func (session *Session) ensureRequestInterception(tab *sessionTab) error {
	if tab == nil {
		return errors.New("tab unavailable")
	}
	tab.mu.RLock()
	enabled := tab.fetchEnabled
	tab.mu.RUnlock()
	if enabled {
		return nil
	}
	if err := session.enableRequestInterception(tab); err != nil {
		return err
	}
	tab.mu.Lock()
	tab.fetchEnabled = true
	tab.mu.Unlock()
	return nil
}

func (session *Session) handlePausedRequest(tab *sessionTab, event *fetch.EventRequestPaused) {
	if tab == nil || event == nil {
		return
	}
	requestURL := ""
	if event.Request != nil {
		requestURL = strings.TrimSpace(event.Request.URL)
	}
	err := session.runOnTab(tab, 5*time.Second, chromedp.ActionFunc(func(ctx context.Context) error {
		if event.Request == nil {
			return fetch.ContinueRequest(event.RequestID).Do(ctx)
		}
		options := session.optionsSnapshot()
		if err := assertRequestURLAllowed(ctx, requestURL, options.SSRFRules); err != nil {
			blockedErr := fmt.Errorf("blocked by SSRF policy for %s: %w", requestURL, err)
			setBlockedRequestError(tab, blockedErr)
			zap.L().Warn(
				"browser request blocked by ssrf policy",
				append(session.logFields(),
					zap.String("targetId", tab.TargetID),
					sanitizedURLField("url", requestURL),
					zap.Error(err),
				)...,
			)
			return fetch.FailRequest(event.RequestID, network.ErrorReasonBlockedByClient).Do(ctx)
		}
		return fetch.ContinueRequest(event.RequestID).Do(ctx)
	}))
	if err != nil {
		zap.L().Warn(
			"browser request interception handling failed",
			append(session.logFields(),
				zap.String("targetId", tab.TargetID),
				sanitizedURLField("url", requestURL),
				zap.Error(err),
			)...,
		)
	}
}

func (session *Session) resolveTab(targetID string, allowActive bool) (*sessionTab, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	targetID = strings.TrimSpace(targetID)
	if targetID != "" {
		tab, ok := session.tabs[targetID]
		if !ok {
			return nil, errors.New("tab not found")
		}
		return tab, nil
	}
	if allowActive && session.activeTarget != "" {
		if tab, ok := session.tabs[session.activeTarget]; ok {
			return tab, nil
		}
	}
	for _, tab := range session.tabs {
		return tab, nil
	}
	return nil, errNoOpenTab
}

func (session *Session) closeTab(targetID string, timeout time.Duration) (ActionResult, error) {
	tab, err := session.resolveTab(targetID, false)
	if err != nil {
		return ActionResult{}, err
	}
	cancelTabContexts(tab)
	session.mu.Lock()
	runtime := session.runtime
	session.mu.Unlock()
	if runtime != nil {
		runCtx, cancel, err := session.newBrowserExecutorContext(normalizeTimeout(timeout, 15*time.Second))
		if err == nil {
			_ = targetpkg.CloseTarget(targetpkg.ID(tab.TargetID)).Do(runCtx)
			cancel()
		}
	}
	session.detachTab(tab.TargetID)
	return ActionResult{
		OK:       true,
		TargetID: tab.TargetID,
		Closed:   true,
	}, nil
}

func (session *Session) detachTab(targetID string) {
	session.mu.Lock()
	defer session.mu.Unlock()
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return
	}
	delete(session.tabs, targetID)
	delete(session.pendingDialogs, targetID)
	if session.activeTarget == targetID {
		session.activeTarget = ""
		for _, tab := range session.tabs {
			session.activeTarget = tab.TargetID
			break
		}
	}
}

func (session *Session) browserPageTargets(timeout time.Duration) (map[string]*targetpkg.Info, error) {
	execCtx, cancel, err := session.newBrowserExecutorContext(timeout)
	if err != nil {
		return nil, err
	}
	defer cancel()
	infos, err := targetpkg.GetTargets().Do(execCtx)
	if err != nil {
		return nil, err
	}
	result := map[string]*targetpkg.Info{}
	for _, info := range infos {
		if info == nil || info.Type != "page" {
			continue
		}
		result[string(info.TargetID)] = info
	}
	return result, nil
}

func (session *Session) setActiveTarget(targetID string) {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.activeTarget = strings.TrimSpace(targetID)
}

func (session *Session) snapshotPageTargets() (map[string]*targetpkg.Info, error) {
	return session.browserPageTargets(3 * time.Second)
}

func (session *Session) prepareNewTabWaiter(parent context.Context, tab *sessionTab) (*newTabWaiter, error) {
	if tab == nil {
		return nil, errors.New("tab unavailable")
	}
	tab.mu.RLock()
	baseCtx := tab.ctx
	tab.mu.RUnlock()
	if baseCtx == nil {
		return nil, errors.New("tab context unavailable")
	}
	waitCtx, cancel := context.WithCancel(baseCtx)
	var stop func() bool
	if parent != nil {
		if err := parent.Err(); err != nil {
			cancel()
			return nil, err
		}
		stop = context.AfterFunc(parent, cancel)
	}
	return &newTabWaiter{
		ctx: waitCtx,
		ids: chromedp.WaitNewTarget(waitCtx, func(info *targetpkg.Info) bool {
			return info != nil && info.Type == "page"
		}),
		cancel: cancel,
		stop:   stop,
	}, nil
}

func (session *Session) waitForNewTab(waiter *newTabWaiter, timeout time.Duration) (*sessionTab, bool) {
	if waiter == nil {
		return nil, false
	}
	timeout = normalizeTimeout(timeout, 1500*time.Millisecond)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-waiter.ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	case targetID, ok := <-waiter.ids:
		if !ok {
			return nil, false
		}
		trimmedTargetID := strings.TrimSpace(string(targetID))
		if trimmedTargetID == "" {
			return nil, false
		}
		tab, err := session.attachTargetAsTab(trimmedTargetID, false)
		if err != nil {
			return nil, false
		}
		return tab, true
	}
}

func (session *Session) resolveReusableTargetID() (string, error) {
	session.mu.Lock()
	runtime := session.runtime
	managedTabs := len(session.tabs)
	session.mu.Unlock()
	if runtime == nil || managedTabs > 0 {
		return "", nil
	}
	infos, err := session.browserPageTargets(3 * time.Second)
	if err != nil {
		return "", err
	}
	return pickReusableTargetID(mapTargetInfos(infos)), nil
}

func (session *Session) assertURLAllowed(rawURL string) error {
	options := session.optionsSnapshot()
	return assertNavigationURLAllowed(rawURL, options.SSRFRules)
}

func AssertURLAllowed(rawURL string, policy SSRFPolicy) error {
	return assertNavigationURLAllowed(rawURL, policy)
}

func assertNavigationURLAllowed(rawURL string, policy SSRFPolicy) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultSSRFValidationTimeout)
	defer cancel()
	return assertURLAllowedWithSchemes(ctx, rawURL, policy, map[string]struct{}{
		"http":  {},
		"https": {},
	}, true)
}

func assertRequestURLAllowed(ctx context.Context, rawURL string, policy SSRFPolicy) error {
	return assertURLAllowedWithSchemes(ctx, rawURL, policy, map[string]struct{}{
		"http":  {},
		"https": {},
		"ws":    {},
		"wss":   {},
	}, false)
}

func assertURLAllowedWithSchemes(ctx context.Context, rawURL string, policy SSRFPolicy, allowedSchemes map[string]struct{}, rejectUnsupportedScheme bool) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return errors.New("targetUrl is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if _, ok := allowedSchemes[scheme]; !ok {
		if rejectUnsupportedScheme {
			return errors.New("only http(s) urls are supported")
		}
		return nil
	}
	return assertParsedURLAllowed(ctx, parsed, policy)
}

func assertParsedURLAllowed(ctx context.Context, parsed *url.URL, policy SSRFPolicy) error {
	if parsed == nil {
		return errors.New("invalid url")
	}
	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if hostname == "" {
		return errors.New("url hostname is required")
	}
	if isHostnameAllowed(hostname, policy) {
		return nil
	}
	if policy.DangerouslyAllowPrivateNetwork {
		return nil
	}
	if hostname == "localhost" || strings.HasSuffix(hostname, ".local") || strings.HasSuffix(hostname, ".internal") {
		return fmt.Errorf("blocked private hostname: %s", hostname)
	}
	if ip := net.ParseIP(hostname); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("blocked private IP: %s", hostname)
		}
		return nil
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), defaultSSRFValidationTimeout)
		defer cancel()
	}
	records, err := lookupIPAddrsForHost(ctx, hostname)
	if err != nil {
		return fmt.Errorf("could not validate hostname %s: %w", hostname, err)
	}
	if len(records) == 0 {
		return fmt.Errorf("could not validate hostname %s: no IPs returned", hostname)
	}
	for _, record := range records {
		if isPrivateOrLocalIP(record.IP) {
			return fmt.Errorf("blocked hostname resolving to private IP: %s -> %s", hostname, record.IP.String())
		}
	}
	return nil
}

func (session *Session) assertObservedURLAllowed(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	return session.assertURLAllowed(trimmed)
}

func (session *Session) ensureCookiesForURL(ctx context.Context, tab *sessionTab, targetURL string) error {
	if session == nil || tab == nil {
		return nil
	}
	options := session.optionsSnapshot()
	if options.Cookies == nil {
		return nil
	}
	cookies, err := options.Cookies.ResolveCookiesForURL(ctx, targetURL)
	if err != nil {
		return &ConnectorCookieError{URL: targetURL, Err: err}
	}
	if len(cookies) == 0 {
		return nil
	}
	syncKeys := cookieSyncKeys(targetURL, cookies)
	fingerprint := cookieFingerprint(cookies)
	session.mu.Lock()
	if hasCookieSyncFingerprint(session.cookieSync, syncKeys, fingerprint) {
		session.mu.Unlock()
		return nil
	}
	session.mu.Unlock()
	runCtx, cancel, err := session.newBrowserExecutorContext(10 * time.Second)
	if err != nil {
		return &ConnectorCookieError{URL: targetURL, Err: err}
	}
	defer cancel()
	if err := SetCookiesOnBrowser(runCtx, targetURL, cookies); err != nil {
		return &ConnectorCookieError{URL: targetURL, Err: err}
	}
	session.mu.Lock()
	rememberCookieSyncFingerprint(session.cookieSync, syncKeys, fingerprint)
	session.mu.Unlock()
	return nil
}

func (session *Session) ensureCookiesForURLOnBrowser(ctx context.Context, targetURL string) error {
	options := session.optionsSnapshot()
	if options.Cookies == nil {
		return nil
	}
	cookies, err := options.Cookies.ResolveCookiesForURL(ctx, targetURL)
	if err != nil {
		return &ConnectorCookieError{URL: targetURL, Err: err}
	}
	if len(cookies) == 0 {
		return nil
	}
	syncKeys := cookieSyncKeys(targetURL, cookies)
	fingerprint := cookieFingerprint(cookies)
	session.mu.Lock()
	if hasCookieSyncFingerprint(session.cookieSync, syncKeys, fingerprint) {
		session.mu.Unlock()
		return nil
	}
	runtime := session.runtime
	session.mu.Unlock()
	if runtime == nil {
		return errors.New("browser runtime unavailable")
	}
	runCtx, cancel, err := session.newBrowserExecutorContext(10 * time.Second)
	if err != nil {
		return err
	}
	defer cancel()
	if err := SetCookiesOnBrowser(runCtx, targetURL, cookies); err != nil {
		return &ConnectorCookieError{URL: targetURL, Err: err}
	}
	session.mu.Lock()
	rememberCookieSyncFingerprint(session.cookieSync, syncKeys, fingerprint)
	session.mu.Unlock()
	return nil
}

func (session *Session) collectActionResult(tab *sessionTab, limit int, timeout time.Duration, stateRequired bool) (ActionResult, error) {
	if err := consumeBlockedRequestError(tab); err != nil {
		return ActionResult{}, err
	}
	pageState, err := session.collectPageState(tab, limit, timeout)
	if err != nil && isTargetLookupError(err) {
		rebound := session.rebindTabAfterNavigation(tab, tabURL(tab))
		if rebound != nil {
			tab = rebound
			pageState, err = session.collectPageState(tab, limit, timeout)
		}
	} else if err != nil && shouldDeferStateCapture(err) {
		rebound := session.rebindTabFromBrowserTargets(tab, tabURL(tab))
		if rebound != nil && rebound.TargetID != "" && rebound.TargetID != tab.TargetID {
			tab = rebound
			pageState, err = session.collectPageState(tab, limit, timeout)
		}
	}
	if err == nil {
		if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
			return ActionResult{}, blockedErr
		}
		return resultFromPageState(tab, pageState), nil
	}
	if stateRequired || !shouldDeferStateCapture(err) {
		return ActionResult{}, err
	}
	session.refreshTabMetadataFromBrowser(tab)
	storedURL := tabURL(tab)
	url, title := session.readPageMetadata(tab, 1200*time.Millisecond)
	url = preferredPageURL(url, storedURL)
	zap.L().Warn(
		"browser state capture deferred",
		append(session.logFields(),
			zap.String("targetId", tab.TargetID),
			sanitizedURLField("url", url),
			zap.Duration("timeout", timeout),
			zap.Error(err),
		)...,
	)
	if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
		return ActionResult{}, blockedErr
	}
	return ActionResult{
		OK:             true,
		TargetID:       tab.TargetID,
		URL:            url,
		Title:          title,
		StateAvailable: false,
		StateError:     strings.TrimSpace(err.Error()),
	}, nil
}

func (session *Session) collectPageState(tab *sessionTab, limit int, timeout time.Duration) (*PageState, error) {
	snapshotCtx, snapshotCancel, err := newTabRunContext(tab, timeout)
	if err != nil {
		return nil, err
	}
	defer snapshotCancel()
	capture, err := collectSnapshot(tab, snapshotCtx, limit, timeout)
	if err != nil {
		return nil, err
	}
	pageState := &PageState{
		Version:      capture.Version,
		URL:          capture.URL,
		Title:        capture.Title,
		Items:        capture.Items,
		ItemCount:    len(capture.Items),
		Truncated:    capture.Truncated,
		ViewportOnly: capture.ViewportOnly,
		CapturedAt:   time.Now().Format(time.RFC3339),
	}
	tab.mu.Lock()
	tab.refs = capture.Refs
	tab.lastURL = capture.URL
	tab.title = capture.Title
	tab.lastState = pageState
	tab.stateVersion = capture.Version
	tab.mu.Unlock()
	return pageState, nil
}

func resultFromPageState(tab *sessionTab, pageState *PageState) ActionResult {
	result := ActionResult{
		OK:             true,
		StateAvailable: true,
	}
	if tab != nil {
		result.TargetID = tab.TargetID
	}
	if pageState != nil {
		result.URL = pageState.URL
		result.Title = pageState.Title
		result.StateVersion = pageState.Version
		result.State = pageState
		result.Items = pageState.Items
	}
	return result
}

func collectSnapshot(tab *sessionTab, runBaseCtx context.Context, limit int, timeout time.Duration) (*snapshotCapture, error) {
	if limit <= 0 {
		limit = defaultSnapshotLimit
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxScan := limit * 20
	if maxScan < 500 {
		maxScan = 500
	}
	if maxScan > 3000 {
		maxScan = 3000
	}
	timeBudgetMs := int(timeout / time.Millisecond / 2)
	if timeBudgetMs < 250 {
		timeBudgetMs = 250
	}
	if timeBudgetMs > 1500 {
		timeBudgetMs = 1500
	}
	script := fmt.Sprintf(`(() => {
	  const limit = %d;
	  const maxScan = %d;
	  const timeBudgetMs = %d;
	  const startedAt = (typeof performance !== "undefined" && performance.now) ? performance.now() : Date.now();
	  let truncated = false;
	  const now = () => ((typeof performance !== "undefined" && performance.now) ? performance.now() : Date.now());
	  const inferRole = (el) => {
	    const explicit = (el.getAttribute("role") || "").trim();
	    if (explicit) return explicit.toLowerCase();
	    const tag = el.tagName.toLowerCase();
    if (tag === "a") return "link";
    if (tag === "button") return "button";
    if (tag === "textarea" || tag === "input") return "textbox";
    if (tag === "select") return "combobox";
    return "element";
  };
  const cssPath = (el) => {
    if (el.id) return "#" + CSS.escape(el.id);
    const parts = [];
    let node = el;
    while (node && node.nodeType === 1 && parts.length < 6) {
      let part = node.tagName.toLowerCase();
      if (node.classList && node.classList.length > 0) {
        part += "." + Array.from(node.classList).slice(0, 2).map((item) => CSS.escape(item)).join(".");
      }
      const parent = node.parentElement;
      if (parent) {
        const siblings = Array.from(parent.children).filter((candidate) => candidate.tagName === node.tagName);
        if (siblings.length > 1) {
          part += ":nth-of-type(" + (siblings.indexOf(node) + 1) + ")";
        }
      }
      parts.unshift(part);
      node = parent;
    }
    return parts.join(" > ");
  };
	  const visible = (el) => {
	    const style = window.getComputedStyle(el);
	    const rect = el.getBoundingClientRect();
	    return style && style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
	  };
	  const elements = document.querySelectorAll('a,button,input,textarea,select,summary,[role="button"],[role="link"],[role="menuitem"],[tabindex]:not([tabindex="-1"])');
	  const candidates = [];
	  const scanLimit = Math.min(elements.length, maxScan);
	  if (elements.length > scanLimit) {
	    truncated = true;
	  }
	  for (let index = 0; index < scanLimit; index += 1) {
	    if (candidates.length >= limit) {
	      truncated = true;
	      break;
	    }
	    if ((now() - startedAt) > timeBudgetMs) {
	      truncated = true;
	      break;
	    }
	    const el = elements[index];
	    if (!el) continue;
	    try {
	      if (!visible(el)) continue;
	      const text = ((el.value || el.textContent || "") + "").replace(/\s+/g, " ").trim();
	      const name = ((el.getAttribute("aria-label") || el.getAttribute("title") || el.placeholder || text) + "").replace(/\s+/g, " ").trim();
	      candidates.push({
	        selector: cssPath(el),
	        role: inferRole(el),
	        name,
	        text,
	      });
	    } catch (_err) {
	      truncated = true;
	    }
	  }
	  return {
	    url: document.location.toString(),
	    title: document.title,
	    truncated,
	    candidates,
	  };
	})()`, limit, maxScan, timeBudgetMs)
	var payload struct {
		URL        string `json:"url"`
		Title      string `json:"title"`
		Truncated  bool   `json:"truncated"`
		Candidates []struct {
			Selector string `json:"selector"`
			Role     string `json:"role"`
			Name     string `json:"name"`
			Text     string `json:"text"`
		} `json:"candidates"`
	}
	runCtx, cancel := context.WithTimeout(runBaseCtx, timeout)
	defer cancel()
	captureStartedAt := time.Now()
	if err := chromedp.Run(runCtx, chromedp.EvaluateAsDevTools(script, &payload)); err != nil {
		zap.L().Warn(
			"browser snapshot capture failed",
			zap.String("targetId", tab.TargetID),
			zap.Duration("elapsed", time.Since(captureStartedAt).Round(time.Millisecond)),
			zap.Duration("timeout", timeout),
			zap.Error(err),
		)
		return nil, err
	}
	currentURL := strings.TrimSpace(payload.URL)
	title := strings.TrimSpace(payload.Title)
	version, refsAllocated := allocateStateRefs(tab, len(payload.Candidates))
	items := make([]SnapshotItem, 0, len(payload.Candidates))
	refs := map[string]snapshotRef{}
	countByRoleName := map[string]int{}
	for index, item := range payload.Candidates {
		ref := refsAllocated[index]
		key := item.Role + "\n" + item.Name
		nth := countByRoleName[key]
		countByRoleName[key] = nth + 1
		items = append(items, SnapshotItem{
			Ref:   ref,
			Role:  item.Role,
			Name:  item.Name,
			Text:  item.Text,
			Depth: 0,
			Nth:   nth,
		})
		refs[ref] = snapshotRef{
			Selector: item.Selector,
			Role:     item.Role,
			Name:     item.Name,
			Nth:      nth,
		}
	}
	return &snapshotCapture{
		Version:      version,
		URL:          currentURL,
		Title:        title,
		Items:        items,
		Refs:         refs,
		Truncated:    payload.Truncated || len(payload.Candidates) >= limit,
		ViewportOnly: false,
	}, nil
}

func allocateStateRefs(tab *sessionTab, count int) (uint64, []string) {
	tab.mu.Lock()
	defer tab.mu.Unlock()
	tab.stateVersion++
	version := tab.stateVersion
	refs := make([]string, count)
	for index := 0; index < count; index++ {
		tab.nextRefID++
		refs[index] = fmt.Sprintf("e%d", tab.nextRefID)
	}
	return version, refs
}

func clearTabState(tab *sessionTab) {
	if tab == nil {
		return
	}
	tab.mu.Lock()
	tab.refs = map[string]snapshotRef{}
	tab.lastState = nil
	tab.mu.Unlock()
}

func resolveRefSelector(tab *sessionTab, ref string) (string, error) {
	if tab == nil {
		return "", errors.New("tab unavailable")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", errors.New("ref is required")
	}
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	item, ok := tab.refs[ref]
	if !ok {
		return "", &InvalidRefError{Ref: ref}
	}
	if strings.TrimSpace(item.Selector) == "" {
		return "", errors.New("ref selector unavailable")
	}
	return item.Selector, nil
}

func (session *Session) readPageMetadata(tab *sessionTab, timeout time.Duration) (string, string) {
	if session == nil || tab == nil {
		return "", ""
	}
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}
	baseCtx, cancel, err := newTabRunContext(tab, timeout)
	if err != nil {
		tab.mu.RLock()
		defer tab.mu.RUnlock()
		return strings.TrimSpace(tab.lastURL), strings.TrimSpace(tab.title)
	}
	defer cancel()
	var metadata struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := chromedp.Run(baseCtx, chromedp.EvaluateAsDevTools(`({
		url: document.location.toString(),
		title: document.title,
	})`, &metadata)); err == nil {
		currentURL := strings.TrimSpace(metadata.URL)
		title := strings.TrimSpace(metadata.Title)
		tab.mu.Lock()
		currentURL = preferredPageURL(currentURL, tab.lastURL)
		if currentURL != "" {
			tab.lastURL = currentURL
		}
		if title != "" {
			tab.title = title
		}
		tab.mu.Unlock()
		return currentURL, title
	}
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	return strings.TrimSpace(tab.lastURL), strings.TrimSpace(tab.title)
}

func (session *Session) newBrowserExecutorContext(timeout time.Duration) (context.Context, context.CancelFunc, error) {
	session.mu.Lock()
	runtime := session.runtime
	session.mu.Unlock()
	if runtime == nil {
		return nil, nil, errors.New("browser runtime unavailable")
	}
	baseCtx := runtime.BrowserContext()
	if baseCtx == nil {
		return nil, nil, errors.New("browser context unavailable")
	}
	chromeCtx := chromedp.FromContext(baseCtx)
	if chromeCtx == nil || chromeCtx.Browser == nil {
		return nil, nil, errors.New("browser executor unavailable")
	}
	runCtx, cancel := context.WithTimeout(baseCtx, timeout)
	return cdp.WithExecutor(runCtx, chromeCtx.Browser), cancel, nil
}

func (session *Session) browserTargetInfo(targetID string, timeout time.Duration) *targetpkg.Info {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil
	}
	targets, err := session.browserPageTargets(timeout)
	if err != nil {
		return nil
	}
	return targets[targetID]
}

func (session *Session) refreshTabMetadataFromBrowser(tab *sessionTab) {
	if tab == nil {
		return
	}
	info := session.browserTargetInfo(tab.TargetID, 1500*time.Millisecond)
	if info == nil {
		return
	}
	currentURL := strings.TrimSpace(info.URL)
	title := strings.TrimSpace(info.Title)
	tab.mu.Lock()
	currentURL = preferredPageURL(currentURL, tab.lastURL)
	if currentURL != "" {
		tab.lastURL = currentURL
	}
	if title != "" {
		tab.title = title
	}
	tab.mu.Unlock()
}

func (session *Session) observeActionNavigation(tab *sessionTab, before map[string]*targetpkg.Info, previousURL string, timeout time.Duration) (*sessionTab, bool, error) {
	if tab == nil {
		return tab, false, nil
	}
	timeout = normalizeTimeout(timeout, 5*time.Second)
	deadline := time.Now().Add(timeout)
	for {
		if err := consumeBlockedRequestError(tab); err != nil {
			return tab, false, err
		}
		targets, err := session.browserPageTargets(500 * time.Millisecond)
		if err == nil {
			if info := targets[tab.TargetID]; info != nil {
				currentURL := strings.TrimSpace(info.URL)
				currentTitle := strings.TrimSpace(info.Title)
				if shouldTreatNavigationAsComplete(currentURL, previousURL, "") {
					if err := session.assertObservedURLAllowed(currentURL); err != nil {
						return tab, false, err
					}
					tab.mu.Lock()
					tab.lastURL = currentURL
					if currentTitle != "" {
						tab.title = currentTitle
					}
					tab.mu.Unlock()
					rebound := session.rebindTabAfterNavigation(tab, currentURL)
					if rebound != nil {
						tab = rebound
					}
					storeNavigation(tab, currentURL)
					return tab, true, nil
				}
			}
			for targetID, info := range targets {
				if info == nil || strings.TrimSpace(targetID) == "" || targetID == tab.TargetID {
					continue
				}
				if _, existed := before[targetID]; existed {
					continue
				}
				currentURL := strings.TrimSpace(info.URL)
				currentTitle := strings.TrimSpace(info.Title)
				if !shouldTreatNavigationAsComplete(currentURL, previousURL, "") {
					continue
				}
				if err := session.assertObservedURLAllowed(currentURL); err != nil {
					return tab, false, err
				}
				rebound, attachErr := session.attachTargetAsTab(targetID, false)
				if attachErr != nil || rebound == nil {
					continue
				}
				rebound.mu.Lock()
				rebound.lastURL = currentURL
				if currentTitle != "" {
					rebound.title = currentTitle
				}
				rebound.mu.Unlock()
				if rebound.TargetID != tab.TargetID {
					session.detachTab(tab.TargetID)
				}
				storeNavigation(rebound, currentURL)
				return rebound, true, nil
			}
		} else {
			info := session.browserTargetInfo(tab.TargetID, 500*time.Millisecond)
			if info == nil {
				if time.Now().After(deadline) {
					if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
						return tab, false, blockedErr
					}
					return tab, false, nil
				}
				time.Sleep(150 * time.Millisecond)
				continue
			}
			currentURL := strings.TrimSpace(info.URL)
			currentTitle := strings.TrimSpace(info.Title)
			if shouldTreatNavigationAsComplete(currentURL, previousURL, "") {
				if err := session.assertObservedURLAllowed(currentURL); err != nil {
					return tab, false, err
				}
				tab.mu.Lock()
				tab.lastURL = currentURL
				if currentTitle != "" {
					tab.title = currentTitle
				}
				tab.mu.Unlock()
				rebound := session.rebindTabAfterNavigation(tab, currentURL)
				if rebound != nil {
					tab = rebound
				}
				storeNavigation(tab, currentURL)
				return tab, true, nil
			}
		}
		if time.Now().After(deadline) {
			if blockedErr := consumeBlockedRequestError(tab); blockedErr != nil {
				return tab, false, blockedErr
			}
			return tab, false, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func mapTargetInfos(targets map[string]*targetpkg.Info) []*targetpkg.Info {
	if len(targets) == 0 {
		return nil
	}
	result := make([]*targetpkg.Info, 0, len(targets))
	for _, info := range targets {
		if info != nil {
			result = append(result, info)
		}
	}
	return result
}

func (session *Session) runOnTab(tab *sessionTab, timeout time.Duration, actions ...chromedp.Action) error {
	return session.runOnTabFunc(tab, timeout, func(ctx context.Context) error {
		return chromedp.Run(ctx, actions...)
	})
}

func (session *Session) runOnTabFunc(tab *sessionTab, timeout time.Duration, fn func(context.Context) error) error {
	if session == nil || tab == nil {
		return errors.New("tab unavailable")
	}
	baseCtx, cancel, err := newTabRunContext(tab, timeout)
	if err != nil {
		return err
	}
	defer cancel()
	return fn(baseCtx)
}

func newTabRunContextWithParent(parent context.Context, tab *sessionTab, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	runCtx, cancel, err := newTabRunContext(tab, timeout)
	if err != nil {
		return nil, nil, err
	}
	if parent == nil {
		return runCtx, cancel, nil
	}
	if err := parent.Err(); err != nil {
		cancel()
		return nil, nil, err
	}
	stop := context.AfterFunc(parent, cancel)
	return runCtx, func() {
		stop()
		cancel()
	}, nil
}

func newTabRunContext(tab *sessionTab, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if tab == nil {
		return nil, nil, errors.New("tab unavailable")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tab.mu.RLock()
	baseCtx := tab.ctx
	tab.mu.RUnlock()
	if baseCtx == nil {
		return nil, nil, errors.New("tab context unavailable")
	}
	runCtx, cancel := context.WithTimeout(baseCtx, timeout)
	return runCtx, cancel, nil
}

func cancelTabContexts(tab *sessionTab) {
	if tab == nil {
		return
	}
	tab.mu.Lock()
	cancels := append([]context.CancelFunc(nil), tab.cleanupCancels...)
	if tab.cancel != nil {
		cancels = append(cancels, tab.cancel)
	}
	tab.cleanupCancels = nil
	tab.cancel = nil
	tab.ctx = nil
	tab.mu.Unlock()
	for index := len(cancels) - 1; index >= 0; index-- {
		if cancels[index] != nil {
			cancels[index]()
		}
	}
}

func (session *Session) navigateTab(tab *sessionTab, targetURL string, timeout time.Duration) (*sessionTab, error) {
	previousURL := tabURL(tab)
	if err := session.ensureRequestInterception(tab); err != nil {
		return nil, err
	}
	commandStartedAt := time.Now()
	var errorText string
	script := fmt.Sprintf(`(() => { window.location.href = %q; return true; })()`, strings.TrimSpace(targetURL))
	err := session.runOnTab(tab, timeout, chromedp.EvaluateAsDevTools(script, nil))
	if err != nil {
		zap.L().Warn(
			"browser navigate command failed",
			zap.String("targetId", tab.TargetID),
			sanitizedURLField("url", targetURL),
			sanitizedURLField("previousURL", previousURL),
			zap.Duration("elapsed", time.Since(commandStartedAt).Round(time.Millisecond)),
			zap.Error(err),
		)
		return nil, err
	}
	if errorText != "" {
		navigationErr := fmt.Errorf("page load error %s", errorText)
		zap.L().Warn(
			"browser navigate command returned load error",
			zap.String("targetId", tab.TargetID),
			sanitizedURLField("url", targetURL),
			sanitizedURLField("previousURL", previousURL),
			zap.Duration("elapsed", time.Since(commandStartedAt).Round(time.Millisecond)),
			zap.Error(navigationErr),
		)
		return nil, navigationErr
	}
	settleTimeout := minDuration(3*time.Second, timeout)
	finalURL, ok, observeErr := session.observeNavigationURL(tab, previousURL, targetURL, settleTimeout)
	if observeErr != nil {
		return nil, observeErr
	}
	if !ok {
		finalURL = strings.TrimSpace(targetURL)
	}
	if err := session.assertObservedURLAllowed(finalURL); err != nil {
		return nil, err
	}
	tab = session.rebindTabAfterNavigation(tab, finalURL)
	storeNavigation(tab, finalURL)
	return tab, nil
}

func (session *Session) rebindTabAfterNavigation(tab *sessionTab, finalURL string) *sessionTab {
	if tab == nil {
		return nil
	}
	targets, err := session.snapshotPageTargets()
	if err != nil {
		return tab
	}
	if _, ok := targets[tab.TargetID]; ok {
		return tab
	}
	expectedURL := strings.TrimSpace(finalURL)
	if expectedURL == "" {
		return tab
	}
	for targetID, info := range targets {
		if info == nil || strings.TrimSpace(targetID) == "" {
			continue
		}
		if !shouldTreatNavigationAsComplete(info.URL, "", expectedURL) {
			continue
		}
		rebound, attachErr := session.attachTargetAsTab(targetID, false)
		if attachErr != nil || rebound == nil {
			continue
		}
		if rebound.TargetID != tab.TargetID {
			session.detachTab(tab.TargetID)
		}
		return rebound
	}
	return tab
}

func (session *Session) rebindTabFromBrowserTargets(tab *sessionTab, previousURL string) *sessionTab {
	if tab == nil {
		return nil
	}
	targets, err := session.browserPageTargets(1500 * time.Millisecond)
	if err != nil || len(targets) == 0 {
		return nil
	}
	currentTargetID := strings.TrimSpace(tab.TargetID)
	previousURL = strings.TrimSpace(previousURL)
	type candidate struct {
		targetID string
		info     *targetpkg.Info
	}
	candidates := make([]candidate, 0, len(targets))
	for targetID, info := range targets {
		if info == nil {
			continue
		}
		targetID = strings.TrimSpace(targetID)
		if targetID == "" || targetID == currentTargetID {
			continue
		}
		currentURL := strings.TrimSpace(info.URL)
		if currentURL == "" || isReusablePageURL(currentURL) || urlsEqual(currentURL, previousURL) {
			continue
		}
		candidates = append(candidates, candidate{targetID: targetID, info: info})
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(left int, right int) bool {
		leftInfo := candidates[left].info
		rightInfo := candidates[right].info
		if leftInfo.Attached != rightInfo.Attached {
			return !leftInfo.Attached
		}
		return candidates[left].targetID < candidates[right].targetID
	})
	chosen := candidates[0]
	rebound, attachErr := session.attachTargetAsTab(chosen.targetID, false)
	if attachErr != nil || rebound == nil {
		return nil
	}
	currentURL := strings.TrimSpace(chosen.info.URL)
	currentTitle := strings.TrimSpace(chosen.info.Title)
	rebound.mu.Lock()
	if currentURL != "" {
		rebound.lastURL = currentURL
	}
	if currentTitle != "" {
		rebound.title = currentTitle
	}
	rebound.mu.Unlock()
	if rebound.TargetID != currentTargetID {
		session.detachTab(currentTargetID)
	}
	return rebound
}

func (session *Session) observeNavigationURL(tab *sessionTab, previousURL string, targetURL string, timeout time.Duration) (string, bool, error) {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := consumeBlockedRequestError(tab); err != nil {
			return "", false, err
		}
		currentURL, _ := session.readPageMetadata(tab, 1200*time.Millisecond)
		err := error(nil)
		if err == nil && shouldTreatNavigationAsComplete(currentURL, previousURL, targetURL) {
			currentURL = strings.TrimSpace(currentURL)
			if err := session.assertObservedURLAllowed(currentURL); err != nil {
				return "", false, err
			}
			return currentURL, true, nil
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if err := consumeBlockedRequestError(tab); err != nil {
		return "", false, err
	}
	return "", false, nil
}

func storeNavigation(tab *sessionTab, finalURL string) {
	if tab == nil {
		return
	}
	tab.mu.Lock()
	tab.lastURL = strings.TrimSpace(finalURL)
	tab.title = ""
	tab.refs = map[string]snapshotRef{}
	tab.lastState = nil
	tab.mu.Unlock()
}

func (session *Session) waitOnTab(parent context.Context, tab *sessionTab, request WaitRequest, fallbackTimeout time.Duration) error {
	timeout := normalizeTimeout(request.Timeout, fallbackTimeout)
	if request.Time > 0 {
		return sleepWithContext(parent, request.Time)
	}
	runCtx, cancel, err := newTabRunContextWithParent(parent, tab, timeout)
	if err != nil {
		return err
	}
	defer cancel()
	if selector := strings.TrimSpace(request.Selector); selector != "" {
		return chromedp.Run(runCtx, chromedp.PollFunction(
			`(selector) => {
				const el = document.querySelector(selector);
				if (!el) return false;
				const style = window.getComputedStyle(el);
				const rect = el.getBoundingClientRect();
				return style && style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
			}`,
			nil,
			chromedp.WithPollingInterval(200*time.Millisecond),
			chromedp.WithPollingTimeout(timeout),
			chromedp.WithPollingArgs(selector),
		))
	}
	if text := strings.TrimSpace(request.Text); text != "" {
		return chromedp.Run(runCtx, chromedp.PollFunction(
			`(expected) => {
				const text = document.body ? document.body.innerText : "";
				return text.includes(expected);
			}`,
			nil,
			chromedp.WithPollingInterval(200*time.Millisecond),
			chromedp.WithPollingTimeout(timeout),
			chromedp.WithPollingArgs(text),
		))
	}
	if textGone := strings.TrimSpace(request.TextGone); textGone != "" {
		return chromedp.Run(runCtx, chromedp.PollFunction(
			`(expected) => {
				const text = document.body ? document.body.innerText : "";
				return !text.includes(expected);
			}`,
			nil,
			chromedp.WithPollingInterval(200*time.Millisecond),
			chromedp.WithPollingTimeout(timeout),
			chromedp.WithPollingArgs(textGone),
		))
	}
	if urlWait := strings.TrimSpace(request.URL); urlWait != "" {
		return chromedp.Run(runCtx, chromedp.PollFunction(
			`(expected) => document.location.toString() === expected`,
			nil,
			chromedp.WithPollingInterval(200*time.Millisecond),
			chromedp.WithPollingTimeout(timeout),
			chromedp.WithPollingArgs(urlWait),
		))
	}
	if fn := strings.TrimSpace(request.Fn); fn != "" {
		return chromedp.Run(runCtx, chromedp.Poll(
			fn,
			nil,
			chromedp.WithPollingInterval(200*time.Millisecond),
			chromedp.WithPollingTimeout(timeout),
		))
	}
	return errors.New("wait requires at least one of: timeMs, text, textGone, selector, url, fn")
}

func shouldRetryActionState(result ActionResult) bool {
	currentURL := strings.TrimSpace(result.URL)
	if currentURL == "" || isReusablePageURL(currentURL) {
		return true
	}
	if !result.StateAvailable && result.State == nil {
		return true
	}
	if result.State == nil {
		return false
	}
	return strings.TrimSpace(result.Title) == "" && result.State.ItemCount == 0
}

func actionStateReason(result ActionResult) string {
	currentURL := strings.TrimSpace(result.URL)
	switch {
	case currentURL == "":
		return "missing-url"
	case isReusablePageURL(currentURL):
		return "placeholder-url"
	case !result.StateAvailable && result.State == nil:
		if strings.TrimSpace(result.StateError) != "" {
			return "state-unavailable:" + strings.TrimSpace(result.StateError)
		}
		return "state-unavailable"
	case result.State == nil:
		return "state-missing"
	case strings.TrimSpace(result.Title) == "" && result.State.ItemCount == 0:
		return "empty-title-and-items"
	default:
		return "stable"
	}
}

func (session *Session) stabilizeActionResult(tab *sessionTab, result ActionResult, limit int, timeout time.Duration, maxWait time.Duration) ActionResult {
	if tab == nil || maxWait <= 0 {
		return result
	}
	deadline := time.Now().Add(maxWait)
	attempts := 0
	for shouldRetryActionState(result) && time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		attempts++
		retried, err := session.collectActionResult(tab, limit, timeout, false)
		if err != nil {
			zap.L().Warn(
				"browser action state stabilization failed",
				append(session.logFields(),
					zap.String("targetId", tab.TargetID),
					zap.Int("attempts", attempts),
					zap.String("reason", actionStateReason(result)),
					zap.Error(err),
				)...,
			)
			break
		}
		result = retried
	}
	return result
}

func (session *Session) actClick(tab *sessionTab, request ActRequest) error {
	selector, err := resolveRefSelector(tab, request.Ref)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%q);
		if (!el) throw new Error("element not found");
		el.scrollIntoView({ block: "center", inline: "center" });
		const invoke = () => {
			el.focus?.();
			if (typeof PointerEvent === "function") {
				el.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true, cancelable: true, pointerType: "mouse", isPrimary: true, button: 0, buttons: 1 }));
				el.dispatchEvent(new PointerEvent("pointerup", { bubbles: true, cancelable: true, pointerType: "mouse", isPrimary: true, button: 0, buttons: 0 }));
			}
			el.dispatchEvent(new MouseEvent("mousedown", { bubbles: true, cancelable: true, button: 0, buttons: 1 }));
			el.dispatchEvent(new MouseEvent("mouseup", { bubbles: true, cancelable: true, button: 0, buttons: 0 }));
			el.click();
		};
		setTimeout(invoke, 0);
		return true;
	})()`, selector)
	return session.runOnTab(tab, normalizeTimeout(request.Timeout, 15*time.Second), chromedp.EvaluateAsDevTools(script, nil))
}

func (session *Session) actType(tab *sessionTab, request ActRequest) error {
	selector, err := resolveRefSelector(tab, request.Ref)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%q);
		if (!el) throw new Error("element not found");
		el.focus();
		const nextValue = String((el.value ?? "")) + %q;
		el.value = nextValue;
		el.dispatchEvent(new InputEvent("input", { bubbles: true, data: %q, inputType: "insertText" }));
		el.dispatchEvent(new Event("change", { bubbles: true }));
		return nextValue;
	})()`, selector, request.Text, request.Text)
	return session.runOnTab(tab, normalizeTimeout(request.Timeout, 15*time.Second), chromedp.EvaluateAsDevTools(script, nil))
}

func (session *Session) actPress(tab *sessionTab, request ActRequest) error {
	if strings.TrimSpace(request.Key) == "" {
		return errors.New("key is required")
	}
	script := fmt.Sprintf(`(() => {
		const target = document.activeElement || document.body;
		const key = %q;
		setTimeout(() => {
			const down = new KeyboardEvent("keydown", { key, bubbles: true });
			const press = new KeyboardEvent("keypress", { key, bubbles: true });
			const up = new KeyboardEvent("keyup", { key, bubbles: true });
			target.dispatchEvent(down);
			target.dispatchEvent(press);
			if (key === "Enter" && target && target.form) {
				if (typeof target.form.requestSubmit === "function") {
					target.form.requestSubmit();
				} else {
					target.form.submit();
				}
			}
			target.dispatchEvent(up);
		}, 0);
		return true;
	})()`, request.Key)
	return session.runOnTab(tab, normalizeTimeout(request.Timeout, 15*time.Second), chromedp.EvaluateAsDevTools(script, nil))
}

func (session *Session) actHover(tab *sessionTab, request ActRequest) error {
	selector, err := resolveRefSelector(tab, request.Ref)
	if err != nil {
		return err
	}
	return session.runOnTab(tab, normalizeTimeout(request.Timeout, 15*time.Second), chromedp.EvaluateAsDevTools(fmt.Sprintf(`(() => { const el = document.querySelector(%q); if (!el) throw new Error("element not found"); el.dispatchEvent(new MouseEvent("mouseover", {bubbles:true})); el.dispatchEvent(new MouseEvent("mouseenter", {bubbles:true})); })()`, selector), nil))
}

func (session *Session) actSelect(tab *sessionTab, request ActRequest) error {
	selector, err := resolveRefSelector(tab, request.Ref)
	if err != nil {
		return err
	}
	return session.runOnTab(tab, normalizeTimeout(request.Timeout, 15*time.Second), chromedp.EvaluateAsDevTools(fmt.Sprintf(`(() => { const el = document.querySelector(%q); if (!el) throw new Error("element not found"); el.value = %q; el.dispatchEvent(new Event("input", {bubbles:true})); el.dispatchEvent(new Event("change", {bubbles:true})); })()`, selector, request.Value), nil))
}

func (session *Session) actFill(tab *sessionTab, request ActRequest) error {
	selector, err := resolveRefSelector(tab, request.Ref)
	if err != nil {
		return err
	}
	return session.runOnTab(tab, normalizeTimeout(request.Timeout, 15*time.Second), chromedp.EvaluateAsDevTools(fmt.Sprintf(`(() => { const el = document.querySelector(%q); if (!el) throw new Error("element not found"); el.focus(); el.value = %q; el.dispatchEvent(new Event("input", {bubbles:true})); el.dispatchEvent(new Event("change", {bubbles:true})); })()`, selector, request.Value), nil))
}

func (session *Session) actResize(tab *sessionTab, request ActRequest) error {
	if request.Width <= 0 {
		return errors.New("width is required")
	}
	if request.Height <= 0 {
		return errors.New("height is required")
	}
	return session.runOnTabFunc(tab, normalizeTimeout(request.Timeout, 15*time.Second), func(ctx context.Context) error {
		return emulation.SetDeviceMetricsOverride(int64(request.Width), int64(request.Height), 1, false).Do(ctx)
	})
}

func (session *Session) actEvaluate(tab *sessionTab, request ActRequest) error {
	if strings.TrimSpace(request.Expression) == "" {
		return errors.New("expression is required")
	}
	var result any
	if err := session.runOnTab(tab, normalizeTimeout(request.Timeout, 15*time.Second), chromedp.Evaluate(request.Expression, &result)); err != nil {
		return err
	}
	tab.mu.Lock()
	tab.evaluateResult = result
	tab.mu.Unlock()
	return nil
}

func evaluateResult(tab *sessionTab) any {
	if tab == nil {
		return nil
	}
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	return tab.evaluateResult
}

func tabURL(tab *sessionTab) string {
	if tab == nil {
		return ""
	}
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	return tab.lastURL
}

func preferredPageURL(candidates ...string) string {
	fallback := ""
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if fallback == "" {
			fallback = candidate
		}
		if !isReusablePageURL(candidate) {
			return candidate
		}
	}
	return fallback
}

func pickReusableTargetID(infos []*targetpkg.Info) string {
	choose := func(requireUnattached bool, preferBlank bool) string {
		for _, info := range infos {
			if info == nil || info.Type != "page" {
				continue
			}
			if requireUnattached && info.Attached {
				continue
			}
			if preferBlank && !isReusablePageURL(info.URL) {
				continue
			}
			return string(info.TargetID)
		}
		return ""
	}
	for _, candidate := range []string{
		choose(true, true),
		choose(true, false),
		choose(false, true),
		choose(false, false),
	} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}

func isReusablePageURL(rawURL string) bool {
	switch strings.TrimSpace(strings.ToLower(rawURL)) {
	case "", "about:blank", "chrome://newtab/", "chrome-search://local-ntp/local-ntp.html":
		return true
	default:
		return false
	}
}

func shouldTreatNavigationAsComplete(observedURL string, previousURL string, targetURL string) bool {
	observed := strings.TrimSpace(observedURL)
	if observed == "" || observed == "about:blank" {
		return false
	}
	if urlsEqual(observed, targetURL) {
		return true
	}
	previous := strings.TrimSpace(previousURL)
	if previous == "" || previous == "about:blank" {
		return true
	}
	return !urlsEqual(observed, previous)
}

func urlsEqual(left string, right string) bool {
	return strings.TrimSpace(left) == strings.TrimSpace(right)
}

func cookieSyncKeys(rawURL string, records []appcookies.Record) []string {
	keys := map[string]struct{}{}
	if parsed, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
		if hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname())); hostname != "" {
			keys[hostname] = struct{}{}
		}
	}
	for _, record := range records {
		domain := strings.ToLower(strings.TrimSpace(record.Domain))
		domain = strings.TrimPrefix(domain, ".")
		if domain == "" {
			continue
		}
		keys[domain] = struct{}{}
	}
	if len(keys) == 0 {
		trimmed := strings.ToLower(strings.TrimSpace(rawURL))
		if trimmed != "" {
			keys[trimmed] = struct{}{}
		}
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func hasCookieSyncFingerprint(values map[string]string, keys []string, fingerprint string) bool {
	if len(values) == 0 || len(keys) == 0 || fingerprint == "" {
		return false
	}
	for _, key := range keys {
		if current := strings.TrimSpace(values[strings.ToLower(strings.TrimSpace(key))]); current == fingerprint {
			return true
		}
	}
	return false
}

func rememberCookieSyncFingerprint(values map[string]string, keys []string, fingerprint string) {
	if len(keys) == 0 || fingerprint == "" {
		return
	}
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		values[key] = fingerprint
	}
}

func isRecoverableCookieSyncError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "context deadline exceeded"),
		strings.Contains(message, "browser context unavailable"),
		strings.Contains(message, "browser executor unavailable"),
		strings.Contains(message, "target closed"),
		strings.Contains(message, "session closed"),
		strings.Contains(message, "connection closed"),
		strings.Contains(message, "execution context was destroyed"),
		strings.Contains(message, "cannot find context with specified id"),
		strings.Contains(message, "unique context id not found"):
		return true
	default:
		return false
	}
}

func wrapRuntimeHangError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "context deadline exceeded"),
		strings.Contains(message, "target closed"),
		strings.Contains(message, "connection closed"),
		strings.Contains(message, "session closed"),
		strings.Contains(message, "websocket"):
		return &FatalError{Err: err}
	default:
		return err
	}
}

func cookieFingerprint(records []appcookies.Record) string {
	if len(records) == 0 {
		return ""
	}
	items := append([]appcookies.Record(nil), records...)
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		switch {
		case left.Domain != right.Domain:
			return left.Domain < right.Domain
		case left.Path != right.Path:
			return left.Path < right.Path
		default:
			return left.Name < right.Name
		}
	})
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, strings.Join([]string{
			strings.TrimSpace(item.Domain),
			strings.TrimSpace(item.Path),
			strings.TrimSpace(item.Name),
			item.Value,
		}, "\n"))
	}
	return strings.Join(parts, "\n---\n")
}

func normalizeTimeout(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		value = fallback
	}
	if value <= 0 {
		value = 15 * time.Second
	}
	if value < 500*time.Millisecond {
		return 500 * time.Millisecond
	}
	if value > 120*time.Second {
		return 120 * time.Second
	}
	return value
}

func captureTimeout(timeout time.Duration) time.Duration {
	timeout = normalizeTimeout(timeout, 15*time.Second)
	scaled := timeout / 5
	if scaled < time.Second {
		scaled = time.Second
	}
	if scaled > 5*time.Second {
		scaled = 5 * time.Second
	}
	return scaled
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	if ctx == nil {
		time.Sleep(delay)
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (session *Session) logFields() []zap.Field {
	options := session.optionsSnapshot()
	return []zap.Field{
		zap.String("profile", options.ProfileName),
		zap.String("preferredBrowser", options.PreferredBrowser),
		zap.Bool("headless", options.Headless),
	}
}

func sanitizedURLField(key string, rawURL string) zap.Field {
	return zap.String(key, sanitizeLogURL(rawURL))
}

func sanitizeLogURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func minDuration(left time.Duration, right time.Duration) time.Duration {
	if left <= 0 {
		return right
	}
	if right <= 0 {
		return left
	}
	if left < right {
		return left
	}
	return right
}

func actInvalidatesState(kind string) bool {
	switch kind {
	case "click", "type", "press", "hover", "select", "fill", "resize":
		return true
	default:
		return false
	}
}

func actNeedsSettle(kind string) bool {
	switch kind {
	case "click", "type", "press", "hover", "select", "fill", "resize", "wait":
		return true
	default:
		return false
	}
}

func actMayOpenNewTab(kind string) bool {
	switch kind {
	case "click", "press":
		return true
	default:
		return false
	}
}

func shouldDeferStateCapture(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "context deadline exceeded"),
		strings.Contains(message, "execution context was destroyed"),
		strings.Contains(message, "cannot find context with specified id"),
		strings.Contains(message, "unique context id not found"):
		return true
	default:
		return false
	}
}

func isTargetLookupError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "no target with given id found")
}

func isHostnameAllowed(hostname string, policy SSRFPolicy) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return false
	}
	if _, ok := policy.AllowedHostnames[hostname]; ok {
		return true
	}
	for _, pattern := range policy.HostnameAllowlist {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if pattern == hostname {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if strings.HasSuffix(hostname, "."+suffix) || hostname == suffix {
				return true
			}
			continue
		}
		if matched, _ := filepath.Match(pattern, hostname); matched {
			return true
		}
	}
	return false
}

func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && (ip4[1]&0xC0) == 64 {
			return true
		}
	}
	return false
}
