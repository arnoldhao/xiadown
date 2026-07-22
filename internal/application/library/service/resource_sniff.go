package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/application/sniffprofile"
)

const (
	resourceSniffStateRunning           = "running"
	resourceSniffStateClosing           = "closing"
	resourceSniffStateClosed            = "closed"
	resourceSniffBrowserStatusOpen      = "open"
	resourceSniffBrowserStatusClosing   = "closing"
	resourceSniffBrowserStatusTabClosed = "tab_closed"
	resourceSniffBrowserStatusClosed    = "browser_closed"
	resourceMediaProbeTimeout           = 5 * time.Second
	resourceSniffIdentityPoll           = 250 * time.Millisecond
	resourceSniffTargetSyncPoll         = 1 * time.Second
	resourceSniffIdentityProbe          = 350 * time.Millisecond
	resourceSniffResponseBodyTimeout    = 6 * time.Second
	resourceSniffGracefulCloseDispatch  = 3 * time.Second
	resourceSniffForceCloseTimeout      = 10 * time.Second
	resourceMediaSnapshotTTL            = 10 * time.Minute
)

type resourceSniffSession struct {
	ID                string
	URL               string
	Runtime           *browsercdp.Runtime
	TabCtx            context.Context
	Cancel            context.CancelFunc
	Capture           *resourceCaptureState
	Tabs              map[string]*resourceSniffTab
	Attaching         map[string]struct{}
	Targets           *browsercdp.PageTargetWatcher
	TargetID          string
	ActiveID          string
	LastMedia         *resourceMedia
	LastMediaIDs      []string
	CurrentURL        string
	Title             string
	State             string
	LaunchFingerprint string
	CloseGeneration   int64
	CloseRequestedAt  time.Time
	Mode              string
	BrowserID         string
	ProfileID         string
}

type resourceMediaSnapshotMetadata struct {
	sessionID string
	expiresAt time.Time
}

type resourceSniffTab struct {
	TargetID          string
	TargetSessionID   string
	Ctx               context.Context
	Cancel            context.CancelFunc
	Capture           *resourceCaptureState
	CurrentURL        string
	Title             string
	Visibility        string
	HasFocus          bool
	PendingNavigation bool
	LastSeen          time.Time
}

func (service *LibraryService) StartResourceSniff(ctx context.Context, request dto.StartResourceSniffRequest) (dto.StartResourceSniffResult, error) {
	resolvedURL := strings.TrimSpace(request.URL)
	if resolvedURL != "" {
		var err error
		resolvedURL, _, err = validateDownloadURL(resolvedURL)
		if err != nil {
			return dto.StartResourceSniffResult{}, err
		}
	}
	mode := normalizeResourceSniffMode(request.Mode)
	switch mode {
	case "managed_profile":
		return service.startManagedResourceSniff(ctx, resolvedURL, request)
	case "current_browser":
		return service.startCurrentBrowserResourceSniff(ctx, resolvedURL, request)
	default:
		return dto.StartResourceSniffResult{}, apperrors.New(apperrors.CodeResourceBrowserLaunchFailed, "unsupported resource browser mode")
	}
}

func (service *LibraryService) startManagedResourceSniff(
	ctx context.Context,
	resolvedURL string,
	request dto.StartResourceSniffRequest,
) (dto.StartResourceSniffResult, error) {
	const mode = "managed_profile"
	releaseProfileStart := sniffprofile.LockForRuntimeStart()
	defer releaseProfileStart()
	profile, profilePath, err := sniffprofile.Resolve(request.ProfileID, request.BrowserID)
	if err != nil {
		return dto.StartResourceSniffResult{}, apperrors.Wrap(apperrors.CodeResourceBrowserLaunchFailed, "initialize resource browser profile", err)
	}

	networkRoute, err := service.managedBrowserNetworkRoute()
	if err != nil {
		return dto.StartResourceSniffResult{}, apperrors.Wrap(
			apperrors.CodeResourceBrowserLaunchFailed,
			"initialize resource browser network route",
			err,
		)
	}
	extraArgs := []string{
		"--autoplay-policy=no-user-gesture-required",
	}
	preferredBrowser := profile.BrowserID
	persistentProfile := strings.TrimSpace(profilePath) != ""
	launchFingerprint := resourceSniffLaunchFingerprint(mode, preferredBrowser, profilePath, networkRoute.ProxyURL, persistentProfile)

	service.resourceSniffLifecycleMu.Lock()
	defer service.resourceSniffLifecycleMu.Unlock()

	var reusableSession *resourceSniffSession
	for _, session := range service.resourceSniffSessionSnapshot() {
		if reusableSession == nil && resourceSniffSessionReusableForLaunch(session, launchFingerprint) {
			reusableSession = session
			continue
		}
		if popped := service.popResourceSniffSession(session.ID); popped != nil {
			service.discardResourceMediaSnapshots(popped.LastMediaIDs...)
			cleanupResourceSniffSession(popped)
		}
	}
	if reusableSession != nil {
		session, err := service.restartResourceSniffSession(ctx, reusableSession.ID, resolvedURL)
		if err == nil {
			return dto.StartResourceSniffResult{Session: &session}, nil
		}
		zap.L().Warn(
			"resource sniff cdp browser reuse failed; restarting browser",
			zap.String("sessionID", reusableSession.ID),
			zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
			zap.Inline(zapFieldsObject(resourceSniffErrorLogFields("resource_sniff_reuse_failed", err))),
		)
		if popped := service.popResourceSniffSession(reusableSession.ID); popped != nil {
			service.discardResourceMediaSnapshots(popped.LastMediaIDs...)
			cleanupResourceSniffSession(popped)
		}
	}
	runtime, err := browsercdp.Start(ctx, browsercdp.LaunchOptions{
		PreferredBrowser:  preferredBrowser,
		Headless:          false,
		UserDataDir:       profilePath,
		ExtraArgs:         extraArgs,
		PersistentProfile: persistentProfile,
		NetworkRoute:      networkRoute,
	})
	if err != nil {
		if errors.Is(err, browsercdp.ErrNoSupportedBrowser) {
			return dto.StartResourceSniffResult{}, apperrors.Wrap(apperrors.CodeResourceBrowserUnavailable, "resource sniff browser unavailable", err)
		}
		return dto.StartResourceSniffResult{}, apperrors.Wrap(apperrors.CodeResourceBrowserLaunchFailed, "start resource browser", err)
	}
	runtimeInfo := runtime.ProcessInfo()
	zap.L().Debug(
		"resource sniff cdp browser started",
		zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
		zap.String("executableRef", resourceSniffLogReference(runtimeInfo.ExecutablePath)),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	tabCtx, tabCancel, targetID, err := browsercdp.AttachOrCreatePageTarget(runtime, 5*time.Second)
	if err != nil {
		zap.L().Warn(
			"resource sniff cdp attach failed; stopping browser",
			zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
			zap.Int("pid", runtimeInfo.PID),
			zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
			zap.Inline(zapFieldsObject(resourceSniffErrorLogFields("resource_sniff_attach_failed", err))),
		)
		runtime.Stop()
		return dto.StartResourceSniffResult{}, err
	}

	capture := newResourceCaptureState()
	sessionID := uuid.NewString()
	tab := &resourceSniffTab{
		TargetID:          strings.TrimSpace(targetID),
		TargetSessionID:   browsercdp.TargetSessionIDFromContext(tabCtx),
		Ctx:               tabCtx,
		Cancel:            tabCancel,
		Capture:           capture,
		CurrentURL:        resolvedURL,
		PendingNavigation: resolvedURL != "",
		LastSeen:          time.Now(),
	}
	session := &resourceSniffSession{
		ID:                sessionID,
		URL:               resolvedURL,
		Runtime:           runtime,
		TabCtx:            tabCtx,
		Cancel:            tabCancel,
		Capture:           capture,
		Tabs:              map[string]*resourceSniffTab{tab.TargetID: tab},
		Attaching:         map[string]struct{}{},
		TargetID:          tab.TargetID,
		ActiveID:          tab.TargetID,
		CurrentURL:        resolvedURL,
		State:             resourceSniffStateRunning,
		LaunchFingerprint: launchFingerprint,
		Mode:              mode,
		BrowserID:         profile.BrowserID,
		ProfileID:         profile.ProfileID,
	}
	if err := sniffprofile.MarkUsed(profile.ProfileID); err != nil {
		fields := []zap.Field{zap.String("profileID", profile.ProfileID)}
		fields = append(fields, resourceSniffErrorLogFields("resource_sniff_profile_mark_failed", err)...)
		zap.L().Warn("mark resource sniff profile used", fields...)
	}
	service.putResourceSniffSession(session)
	zap.L().Debug(
		"resource sniff session stored",
		zap.String("sessionID", sessionID),
		zap.String("targetID", tab.TargetID),
		zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	service.watchResourceSniffTab(sessionID, tab)
	service.enableResourceSniffTargetDiscovery(sessionID)
	go service.monitorResourceSniffSession(sessionID)
	go service.navigateResourceSniffInitialPage(sessionID, tab, resolvedURL, runtimeInfo)

	mappedSession := service.mapResourceSniffSession(session)
	return dto.StartResourceSniffResult{Session: &mappedSession}, nil
}

func (service *LibraryService) GetCurrentResourceSniffBrowserStatus(
	ctx context.Context,
	request dto.CurrentResourceSniffBrowserStatusRequest,
) dto.CurrentResourceSniffBrowserStatus {
	status := browsercdp.InspectCurrentBrowser(ctx, request.BrowserID)
	return dto.CurrentResourceSniffBrowserStatus{
		BrowserID:      status.BrowserID,
		State:          status.State,
		Installed:      status.Installed,
		Running:        status.Running,
		Supported:      status.Supported,
		Ready:          status.Ready,
		Version:        status.Version,
		MinimumVersion: status.MinimumVersion,
		ProfileName:    status.ProfileName,
		Detail:         status.Detail,
	}
}

func (service *LibraryService) startCurrentBrowserResourceSniff(
	ctx context.Context,
	resolvedURL string,
	request dto.StartResourceSniffRequest,
) (dto.StartResourceSniffResult, error) {
	const mode = "current_browser"
	browserID := strings.ToLower(strings.TrimSpace(request.BrowserID))
	if browserID != string(browsercdp.BrowserChrome) {
		return dto.StartResourceSniffResult{}, apperrors.New(
			apperrors.CodeResourceCurrentBrowserUnsupported,
			"only the current stable Chrome browser is supported",
		)
	}
	if strings.TrimSpace(request.ProfileID) != "" {
		return dto.StartResourceSniffResult{}, apperrors.New(
			apperrors.CodeInvalidInput,
			"current browser mode does not accept a profile identifier",
		)
	}

	service.resourceSniffLifecycleMu.Lock()
	defer service.resourceSniffLifecycleMu.Unlock()
	for _, session := range service.resourceSniffSessionSnapshot() {
		if popped := service.popResourceSniffSession(session.ID); popped != nil {
			service.discardResourceMediaSnapshots(popped.LastMediaIDs...)
			cleanupResourceSniffSession(popped)
		}
	}

	runtimeBrowser, err := browsercdp.StartBorrowedCurrentBrowser(ctx, browserID)
	if err != nil {
		return dto.StartResourceSniffResult{}, currentBrowserResourceSniffError(err)
	}
	capture := newResourceCaptureState()
	sessionID := uuid.NewString()
	session := &resourceSniffSession{
		ID:                sessionID,
		URL:               resolvedURL,
		Runtime:           runtimeBrowser,
		Capture:           capture,
		Tabs:              map[string]*resourceSniffTab{},
		Attaching:         map[string]struct{}{},
		State:             resourceSniffStateRunning,
		LaunchFingerprint: resourceSniffLaunchFingerprint(mode, browserID, "", "", false),
		Mode:              mode,
		BrowserID:         browserID,
	}
	service.putResourceSniffSession(session)
	// Current-browser mode never navigates on the user's behalf. It captures
	// requests that occur after the user starts sniffing in the selected Chrome
	// profile. Existing and newly created page targets are attached with
	// detach-only contexts; stopping sniff never closes tabs or Chrome.
	service.enableResourceSniffTargetDiscovery(sessionID)
	service.syncResourceSniffTargets(sessionID)
	go service.monitorResourceSniffSession(sessionID)
	currentSession, _ := service.getResourceSniffSession(sessionID)
	mappedSession := service.mapResourceSniffSession(currentSession)
	return dto.StartResourceSniffResult{Session: &mappedSession}, nil
}

func currentBrowserResourceSniffError(err error) error {
	switch browsercdp.CurrentBrowserErrorState(err) {
	case browsercdp.CurrentBrowserStateUnsupportedBrowser, browsercdp.CurrentBrowserStateUnsupportedVersion:
		return apperrors.Wrap(apperrors.CodeResourceCurrentBrowserUnsupported, "current Chrome is unsupported", err)
	case browsercdp.CurrentBrowserStateNotInstalled, browsercdp.CurrentBrowserStateNotRunning:
		return apperrors.Wrap(apperrors.CodeResourceCurrentBrowserNotRunning, "current Chrome is not running", err)
	case browsercdp.CurrentBrowserStateRemoteDebuggingDisabled:
		return apperrors.Wrap(
			apperrors.CodeResourceCurrentBrowserRemoteDebugging,
			"enable Chrome Remote Debugging before connecting",
			err,
		)
	case browsercdp.CurrentBrowserStatePermissionDenied:
		return apperrors.Wrap(
			apperrors.CodeResourceCurrentBrowserPermission,
			"approve the Chrome debugging connection",
			err,
		)
	default:
		return apperrors.Wrap(
			apperrors.CodeResourceCurrentBrowserEndpointUnavailable,
			"current Chrome debugging endpoint is unavailable",
			err,
		)
	}
}

func normalizeResourceSniffMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "managed", "managed_profile", "xiadown":
		return "managed_profile"
	case "current", "current_browser", "borrowed":
		return "current_browser"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func (service *LibraryService) navigateResourceSniffInitialPage(sessionID string, tab *resourceSniffTab, resolvedURL string, runtimeInfo browsercdp.ProcessInfo) {
	if service == nil || tab == nil || tab.Ctx == nil {
		return
	}
	zap.L().Debug(
		"resource sniff initial navigation started",
		zap.String("sessionID", sessionID),
		zap.String("targetID", tab.TargetID),
		zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	var navigationErr error
	if session, ok := service.getResourceSniffSession(sessionID); !ok || session == nil || session.Runtime == nil {
		navigationErr = errors.New("resource sniff browser session is unavailable")
	} else {
		navigationErr = session.Runtime.VerifyNetworkRoute(tab.Ctx)
	}
	if navigationErr == nil {
		actions := []chromedp.Action{resourceSniffNetworkEnable()}
		if strings.TrimSpace(resolvedURL) != "" {
			actions = append(actions, resourceSniffNavigate(resolvedURL))
		}
		navigationErr = chromedp.Run(tab.Ctx, actions...)
	}
	if err := navigationErr; err != nil {
		if errors.Is(err, context.Canceled) {
			zap.L().Debug(
				"resource sniff initial navigation canceled",
				zap.String("sessionID", sessionID),
				zap.String("targetID", tab.TargetID),
				zap.Int("pid", runtimeInfo.PID),
				zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
			)
			return
		}
		session := service.popResourceSniffSession(sessionID)
		if session == nil {
			zap.L().Warn(
				"resource sniff initial navigation failed after session removed",
				zap.String("sessionID", sessionID),
				zap.String("targetID", tab.TargetID),
				zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
				zap.Int("pid", runtimeInfo.PID),
				zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
				zap.Inline(zapFieldsObject(resourceSniffErrorLogFields("resource_sniff_navigation_failed", err))),
			)
			return
		}
		zap.L().Warn(
			"resource sniff initial navigation failed; cleaning session",
			zap.String("sessionID", sessionID),
			zap.String("targetID", tab.TargetID),
			zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
			zap.Int("pid", runtimeInfo.PID),
			zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
			zap.Inline(zapFieldsObject(resourceSniffErrorLogFields("resource_sniff_navigation_failed", err))),
		)
		service.discardResourceMediaSnapshots(session.LastMediaIDs...)
		cleanupResourceSniffSession(session)
		return
	}
	zap.L().Debug(
		"resource sniff initial navigation finished",
		zap.String("sessionID", sessionID),
		zap.String("targetID", tab.TargetID),
		zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	service.updateResourceSniffSession(sessionID, func(current *resourceSniffSession) {
		if strings.TrimSpace(resolvedURL) != "" && current.TargetID == tab.TargetID {
			current.CurrentURL = resolvedURL
		}
		if strings.TrimSpace(resolvedURL) != "" && current.Tabs != nil {
			if currentTab := current.Tabs[tab.TargetID]; currentTab != nil {
				currentTab.CurrentURL = resolvedURL
			}
		}
	})
}

func (service *LibraryService) GetResourceSniffSession(ctx context.Context, request dto.GetResourceSniffSessionRequest) (dto.ResourceSniffSession, error) {
	service.syncResourceSniffTargets(request.SessionID)
	service.probeResourceSniffSessionPageIdentity(request.SessionID, resourceSniffIdentityProbe)
	session, ok := service.getResourceSniffSession(request.SessionID)
	if !ok {
		return dto.ResourceSniffSession{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	return service.mapResourceSniffSession(session), nil
}

func (service *LibraryService) CancelResourceSniff(ctx context.Context, request dto.CancelResourceSniffRequest) error {
	sessionID := strings.TrimSpace(request.SessionID)
	zap.L().Debug("resource sniff cancel requested", zap.String("sessionID", sessionID))
	service.resourceSniffLifecycleMu.Lock()
	defer service.resourceSniffLifecycleMu.Unlock()
	if !service.requestResourceSniffGracefulClose(sessionID) {
		zap.L().Debug("resource sniff cancel ignored; session not found", zap.String("sessionID", sessionID))
	}
	return nil
}

func (service *LibraryService) ShutdownResourceSniffSessions(ctx context.Context) int {
	if service == nil {
		return 0
	}
	service.resourceSniffMu.Lock()
	sessions := make([]*resourceSniffSession, 0, len(service.resourceSniffs))
	for sessionID, session := range service.resourceSniffs {
		if session != nil {
			sessions = append(sessions, session)
		}
		delete(service.resourceSniffs, sessionID)
	}
	service.resourceSniffMu.Unlock()

	zap.L().Debug("resource sniff shutdown requested", zap.Int("count", len(sessions)))
	for index, session := range sessions {
		if ctx != nil {
			select {
			case <-ctx.Done():
				zap.L().Warn("resource sniff shutdown stopped by context", zap.Int("remaining", len(sessions)-index))
				return len(sessions)
			default:
			}
		}
		service.discardResourceMediaSnapshots(session.LastMediaIDs...)
		cleanupResourceSniffSession(session)
	}
	return len(sessions)
}

func (service *LibraryService) consumeResourceSniffMedia(sessionID string, formatID string) (resourceMedia, bool) {
	session := service.popResourceSniffSession(sessionID)
	if session == nil {
		if strings.TrimSpace(formatID) != "" {
			return service.consumeResourceMediaSnapshot(formatID)
		}
		return resourceMedia{}, false
	}
	var media resourceMedia
	ok := false
	if strings.TrimSpace(formatID) != "" {
		media, ok = service.consumeResourceMediaSnapshot(formatID)
	}
	if !ok && session.LastMedia != nil {
		media = *session.LastMedia
		ok = true
	}
	service.discardResourceMediaSnapshots(session.LastMediaIDs...)
	if !ok {
		if session != nil {
			cleanupResourceSniffSession(session)
		}
		return resourceMedia{}, false
	}
	cleanupResourceSniffSession(session)
	return media, true
}

func resourceSniffLaunchFingerprint(mode string, preferredBrowser string, profilePath string, proxyURL string, persistentProfile bool) string {
	parts := []string{
		normalizeResourceSniffMode(mode),
		strings.ToLower(strings.TrimSpace(preferredBrowser)),
		strings.TrimSpace(profilePath),
		strings.TrimSpace(proxyURL),
		fmt.Sprint(persistentProfile),
	}
	return strings.Join(parts, "\x00")
}

func resourceSniffSessionReusableForLaunch(session *resourceSniffSession, launchFingerprint string) bool {
	if session == nil || session.Runtime == nil {
		return false
	}
	if normalizeResourceSniffMode(session.Mode) != "managed_profile" || session.Runtime.IsBorrowed() {
		return false
	}
	if session.State != resourceSniffStateRunning {
		return false
	}
	if strings.TrimSpace(session.LaunchFingerprint) == "" || session.LaunchFingerprint != launchFingerprint {
		return false
	}
	return session.Runtime.Status().Ready
}

func (service *LibraryService) restartResourceSniffSession(ctx context.Context, sessionID string, resolvedURL string) (dto.ResourceSniffSession, error) {
	sessionID = strings.TrimSpace(sessionID)
	resolvedURL = strings.TrimSpace(resolvedURL)
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok || session == nil || session.Runtime == nil {
		return dto.ResourceSniffSession{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	if !resourceSniffMayDiscoverTargets(session) {
		return dto.ResourceSniffSession{}, apperrors.New(apperrors.CodeResourceResolveFailed, "borrowed resource sniff sessions cannot be reused")
	}
	if !session.Runtime.Status().Ready {
		return dto.ResourceSniffSession{}, apperrors.New(apperrors.CodeResourceBrowserLaunchFailed, "resource sniff browser is closing")
	}
	service.syncResourceSniffTargets(sessionID)
	tab := service.pickResourceSniffActiveTab(sessionID)
	if tab == nil || tab.Ctx == nil || strings.TrimSpace(tab.TargetID) == "" {
		tabCtx, tabCancel, targetID, err := browsercdp.AttachOrCreatePageTarget(session.Runtime, 5*time.Second)
		if err != nil {
			return dto.ResourceSniffSession{}, err
		}
		tab = &resourceSniffTab{
			TargetID:          strings.TrimSpace(targetID),
			TargetSessionID:   browsercdp.TargetSessionIDFromContext(tabCtx),
			Ctx:               tabCtx,
			Cancel:            tabCancel,
			Capture:           newResourceCaptureState(),
			CurrentURL:        resolvedURL,
			PendingNavigation: true,
			LastSeen:          time.Now(),
		}
		service.resourceSniffMu.Lock()
		current := service.resourceSniffs[sessionID]
		if current == nil {
			service.resourceSniffMu.Unlock()
			tabCancel()
			return dto.ResourceSniffSession{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
		}
		if current.Tabs == nil {
			current.Tabs = map[string]*resourceSniffTab{}
		}
		current.Tabs[tab.TargetID] = tab
		current.TabCtx = tabCtx
		current.Cancel = tabCancel
		current.Capture = tab.Capture
		current.TargetID = tab.TargetID
		current.ActiveID = tab.TargetID
		service.resourceSniffMu.Unlock()
		service.watchResourceSniffTab(sessionID, tab)
	} else {
		if tab.Capture == nil {
			tab.Capture = newResourceCaptureState()
		}
	}

	var mediaIDs []string
	captures := make([]*resourceCaptureState, 0)
	service.resourceSniffMu.Lock()
	current := service.resourceSniffs[sessionID]
	if current == nil {
		service.resourceSniffMu.Unlock()
		return dto.ResourceSniffSession{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	current.URL = resolvedURL
	current.CurrentURL = resolvedURL
	current.Title = ""
	current.State = resourceSniffStateRunning
	current.CloseRequestedAt = time.Time{}
	current.CloseGeneration++
	current.LastMedia = nil
	mediaIDs = append(mediaIDs, current.LastMediaIDs...)
	current.LastMediaIDs = nil
	if current.Capture != nil {
		captures = append(captures, current.Capture)
	}
	if current.Tabs == nil {
		current.Tabs = map[string]*resourceSniffTab{}
	}
	if _, exists := current.Tabs[tab.TargetID]; !exists {
		current.Tabs[tab.TargetID] = tab
	}
	for _, item := range current.Tabs {
		if item == nil {
			continue
		}
		if item.Capture != nil {
			captures = append(captures, item.Capture)
		}
		if item.TargetID == tab.TargetID {
			item.CurrentURL = resolvedURL
			item.Title = ""
			item.PendingNavigation = true
			item.LastSeen = time.Now()
		}
	}
	current.TargetID = tab.TargetID
	current.ActiveID = tab.TargetID
	service.resourceSniffMu.Unlock()

	for _, capture := range captures {
		capture.clear()
	}
	service.discardResourceMediaSnapshots(mediaIDs...)
	runtimeInfo := session.Runtime.ProcessInfo()
	zap.L().Debug(
		"resource sniff cdp browser reused",
		zap.String("sessionID", sessionID),
		zap.String("targetID", tab.TargetID),
		zap.String("sourceRef", resourceSniffLogReference(resolvedURL)),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	go service.navigateResourceSniffInitialPage(sessionID, tab, resolvedURL, runtimeInfo)
	return service.GetResourceSniffSession(ctx, dto.GetResourceSniffSessionRequest{SessionID: sessionID})
}

func (service *LibraryService) claimResourceMediaForQueuedOperation(request dto.CreateYTDLPJobRequest) (dto.CreateYTDLPJobRequest, string, error) {
	if strings.TrimSpace(request.ResourceSessionID) == "" && strings.TrimSpace(request.ResourceMediaID) == "" {
		return request, "", nil
	}
	media, ok := service.takeResourceMediaForQueuedOperation(request)
	if !ok {
		return request, "", apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff media snapshot is unavailable")
	}
	claimedID := service.putResourceMediaSnapshot(cloneResourceMedia(media))
	if claimedID == "" {
		return request, "", apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff media snapshot is unavailable")
	}
	request.ResourceMediaID = claimedID
	request.FormatID = claimedID
	return request, claimedID, nil
}

func (service *LibraryService) takeResourceMediaForQueuedOperation(request dto.CreateYTDLPJobRequest) (resourceMedia, bool) {
	seen := make(map[string]struct{}, 2)
	for _, mediaID := range []string{request.FormatID, request.ResourceMediaID} {
		mediaID = strings.TrimSpace(mediaID)
		if mediaID == "" {
			continue
		}
		if _, duplicate := seen[mediaID]; duplicate {
			continue
		}
		seen[mediaID] = struct{}{}
		if media, ok := service.consumeResourceMediaSnapshot(mediaID); ok {
			return media, true
		}
	}
	if sessionID := strings.TrimSpace(request.ResourceSessionID); sessionID != "" {
		return service.peekResourceSniffLastMedia(sessionID)
	}
	return resourceMedia{}, false
}

func (service *LibraryService) resourceMediaForQueuedOperation(request dto.CreateYTDLPJobRequest) (resourceMedia, bool) {
	for _, mediaID := range []string{request.FormatID, request.ResourceMediaID} {
		if media, ok := service.peekResourceMediaSnapshot(mediaID); ok {
			return media, true
		}
	}
	if sessionID := strings.TrimSpace(request.ResourceSessionID); sessionID != "" {
		return service.peekResourceSniffLastMedia(sessionID)
	}
	return resourceMedia{}, false
}

func (service *LibraryService) putResourceMediaSnapshotsForFormats(mediaOptions []resourceMedia) ([]dto.YTDLPFormatOption, []string, string) {
	if len(mediaOptions) == 0 {
		return nil, nil, ""
	}
	formats := make([]dto.YTDLPFormatOption, 0, len(mediaOptions))
	mediaIDs := make([]string, 0, len(mediaOptions))
	defaultMediaID := ""
	for _, media := range mediaOptions {
		mediaID := service.putResourceMediaSnapshot(media)
		if mediaID == "" {
			continue
		}
		if defaultMediaID == "" {
			defaultMediaID = mediaID
		}
		mediaIDs = append(mediaIDs, mediaID)
		formats = append(formats, resourceFormatOptionWithID(mediaID, media))
	}
	return formats, mediaIDs, defaultMediaID
}

func (service *LibraryService) putResourceMediaSnapshot(media resourceMedia) string {
	if service == nil || strings.TrimSpace(media.URL) == "" {
		return ""
	}
	mediaID := uuid.NewString()
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	now := service.now()
	service.pruneExpiredResourceMediaSnapshotsLocked(now)
	if service.resourceMedia == nil {
		service.resourceMedia = make(map[string]resourceMedia)
	}
	if service.resourceMediaMeta == nil {
		service.resourceMediaMeta = make(map[string]resourceMediaSnapshotMetadata)
	}
	service.resourceMedia[mediaID] = cloneResourceMedia(media)
	service.resourceMediaMeta[mediaID] = resourceMediaSnapshotMetadata{expiresAt: now.Add(resourceMediaSnapshotTTL)}
	service.scheduleResourceMediaCleanupLocked(now)
	return mediaID
}

func (service *LibraryService) putResourceMediaSnapshotForSession(media resourceMedia, sessionID string) string {
	if service == nil || strings.TrimSpace(media.URL) == "" {
		return ""
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	mediaID := uuid.NewString()
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	now := service.now()
	service.pruneExpiredResourceMediaSnapshotsLocked(now)
	session := service.resourceSniffs[sessionID]
	if session == nil {
		return ""
	}
	if service.resourceMedia == nil {
		service.resourceMedia = make(map[string]resourceMedia)
	}
	if service.resourceMediaMeta == nil {
		service.resourceMediaMeta = make(map[string]resourceMediaSnapshotMetadata)
	}
	service.resourceMedia[mediaID] = cloneResourceMedia(media)
	service.resourceMediaMeta[mediaID] = resourceMediaSnapshotMetadata{
		sessionID: sessionID,
		expiresAt: now.Add(resourceMediaSnapshotTTL),
	}
	session.LastMediaIDs = append(session.LastMediaIDs, mediaID)
	service.scheduleResourceMediaCleanupLocked(now)
	return mediaID
}

func (service *LibraryService) peekResourceMediaSnapshot(mediaID string) (resourceMedia, bool) {
	if service == nil {
		return resourceMedia{}, false
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	now := service.now()
	service.pruneExpiredResourceMediaSnapshotsLocked(now)
	service.scheduleResourceMediaCleanupLocked(now)
	mediaID = strings.TrimSpace(mediaID)
	if service.resourceMediaSnapshotExpiredLocked(mediaID, now) {
		service.deleteResourceMediaSnapshotLocked(mediaID)
		service.scheduleResourceMediaCleanupLocked(now)
		return resourceMedia{}, false
	}
	media, ok := service.resourceMedia[mediaID]
	if !ok {
		return resourceMedia{}, false
	}
	return cloneResourceMedia(media), true
}

func (service *LibraryService) peekResourceSniffLastMedia(sessionID string) (resourceMedia, bool) {
	if service == nil {
		return resourceMedia{}, false
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || session.LastMedia == nil {
		return resourceMedia{}, false
	}
	return cloneResourceMedia(*session.LastMedia), true
}

func cloneResourceMedia(media resourceMedia) resourceMedia {
	media.RequestHeaders = cloneStringMap(media.RequestHeaders)
	if len(media.Subtitles) > 0 {
		subtitles := make([]resourceSubtitle, len(media.Subtitles))
		for index, subtitle := range media.Subtitles {
			subtitle.RequestHeaders = cloneStringMap(subtitle.RequestHeaders)
			subtitles[index] = subtitle
		}
		media.Subtitles = subtitles
	}
	return media
}

func (service *LibraryService) discardResourceMediaSnapshots(mediaIDs ...string) {
	if service == nil || len(mediaIDs) == 0 {
		return
	}
	ids := append([]string(nil), mediaIDs...)
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	for _, mediaID := range ids {
		service.deleteResourceMediaSnapshotLocked(strings.TrimSpace(mediaID))
	}
	service.scheduleResourceMediaCleanupLocked(service.now())
}

func (service *LibraryService) consumeResourceMediaSnapshot(mediaID string) (resourceMedia, bool) {
	if service == nil {
		return resourceMedia{}, false
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	mediaID = strings.TrimSpace(mediaID)
	now := service.now()
	service.pruneExpiredResourceMediaSnapshotsLocked(now)
	service.scheduleResourceMediaCleanupLocked(now)
	if service.resourceMediaSnapshotExpiredLocked(mediaID, now) {
		service.deleteResourceMediaSnapshotLocked(mediaID)
		service.scheduleResourceMediaCleanupLocked(now)
		return resourceMedia{}, false
	}
	media, ok := service.resourceMedia[mediaID]
	service.deleteResourceMediaSnapshotLocked(mediaID)
	service.scheduleResourceMediaCleanupLocked(now)
	return cloneResourceMedia(media), ok
}

func (service *LibraryService) resourceMediaSnapshotExpiredLocked(mediaID string, now time.Time) bool {
	metadata, ok := service.resourceMediaMeta[mediaID]
	return ok && !metadata.expiresAt.IsZero() && !now.Before(metadata.expiresAt)
}

func (service *LibraryService) pruneExpiredResourceMediaSnapshotsLocked(now time.Time) {
	for mediaID := range service.resourceMediaMeta {
		if service.resourceMediaSnapshotExpiredLocked(mediaID, now) {
			service.deleteResourceMediaSnapshotLocked(mediaID)
		}
	}
}

func (service *LibraryService) scheduleResourceMediaCleanupLocked(now time.Time) {
	if service.resourceMediaCleanupTimer != nil {
		service.resourceMediaCleanupTimer.Stop()
		service.resourceMediaCleanupTimer = nil
	}
	var earliest time.Time
	for _, metadata := range service.resourceMediaMeta {
		if metadata.expiresAt.IsZero() {
			continue
		}
		if earliest.IsZero() || metadata.expiresAt.Before(earliest) {
			earliest = metadata.expiresAt
		}
	}
	if earliest.IsZero() {
		return
	}
	delay := earliest.Sub(now)
	if delay < 0 {
		delay = 0
	}
	service.resourceMediaCleanupTimer = time.AfterFunc(delay, service.cleanupExpiredResourceMediaSnapshots)
}

func (service *LibraryService) cleanupExpiredResourceMediaSnapshots() {
	if service == nil {
		return
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	service.resourceMediaCleanupTimer = nil
	now := service.now()
	service.pruneExpiredResourceMediaSnapshotsLocked(now)
	service.scheduleResourceMediaCleanupLocked(now)
}

func (service *LibraryService) deleteResourceMediaSnapshotLocked(mediaID string) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return
	}
	metadata := service.resourceMediaMeta[mediaID]
	delete(service.resourceMedia, mediaID)
	delete(service.resourceMediaMeta, mediaID)
	if metadata.sessionID == "" {
		return
	}
	session := service.resourceSniffs[metadata.sessionID]
	if session == nil || len(session.LastMediaIDs) == 0 {
		return
	}
	for index, currentID := range session.LastMediaIDs {
		if strings.TrimSpace(currentID) == mediaID {
			session.LastMediaIDs = append(session.LastMediaIDs[:index], session.LastMediaIDs[index+1:]...)
			return
		}
	}
}

func (service *LibraryService) putResourceSniffSession(session *resourceSniffSession) {
	if service == nil || session == nil {
		return
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	if service.resourceSniffs == nil {
		service.resourceSniffs = make(map[string]*resourceSniffSession)
	}
	service.resourceSniffs[session.ID] = session
}

func (service *LibraryService) resourceSniffSessionSnapshot() []*resourceSniffSession {
	if service == nil {
		return nil
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	ids := make([]string, 0, len(service.resourceSniffs))
	for id := range service.resourceSniffs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sessions := make([]*resourceSniffSession, 0, len(ids))
	for _, id := range ids {
		if session := service.resourceSniffs[id]; session != nil {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func (service *LibraryService) getResourceSniffSession(sessionID string) (*resourceSniffSession, bool) {
	if service == nil {
		return nil, false
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session, ok := service.resourceSniffs[strings.TrimSpace(sessionID)]
	return session, ok
}

func (service *LibraryService) updateResourceSniffSession(sessionID string, update func(*resourceSniffSession)) {
	if service == nil || update == nil {
		return
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	if session := service.resourceSniffs[strings.TrimSpace(sessionID)]; session != nil {
		update(session)
	}
}

func (service *LibraryService) popResourceSniffSession(sessionID string) *resourceSniffSession {
	if service == nil {
		return nil
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	sessionID = strings.TrimSpace(sessionID)
	session := service.resourceSniffs[sessionID]
	delete(service.resourceSniffs, sessionID)
	return session
}

func (service *LibraryService) popResourceSniffClosingSession(sessionID string, generation int64) *resourceSniffSession {
	if service == nil {
		return nil
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	sessionID = strings.TrimSpace(sessionID)
	session := service.resourceSniffs[sessionID]
	if session == nil || session.CloseGeneration != generation {
		return nil
	}
	delete(service.resourceSniffs, sessionID)
	return session
}

func (service *LibraryService) requestResourceSniffGracefulClose(sessionID string) bool {
	if service == nil {
		return false
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	var runtime *browsercdp.Runtime
	var targets *browsercdp.PageTargetWatcher
	var generation int64
	var runtimeInfo browsercdp.ProcessInfo
	service.resourceSniffMu.Lock()
	session := service.resourceSniffs[sessionID]
	if session == nil {
		service.resourceSniffMu.Unlock()
		return false
	}
	if session.State == resourceSniffStateClosing {
		service.resourceSniffMu.Unlock()
		return true
	}
	session.State = resourceSniffStateClosing
	session.CloseRequestedAt = service.now()
	session.CloseGeneration++
	generation = session.CloseGeneration
	runtime = session.Runtime
	targets = session.Targets
	session.Targets = nil
	if runtime != nil {
		runtimeInfo = runtime.ProcessInfo()
	}
	service.resourceSniffMu.Unlock()

	if targets != nil {
		targets.Stop()
	}
	zap.L().Debug(
		"resource sniff graceful close requested",
		zap.String("sessionID", sessionID),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	if runtime != nil {
		runtime.RequestGracefulClose(resourceSniffGracefulCloseDispatch)
	}
	go service.finishResourceSniffGracefulClose(sessionID, generation, runtime)
	return true
}

func (service *LibraryService) finishResourceSniffGracefulClose(sessionID string, generation int64, runtime *browsercdp.Runtime) {
	if runtime != nil && !runtime.WaitStopped(resourceSniffForceCloseTimeout) {
		info := runtime.ProcessInfo()
		zap.L().Warn(
			"resource sniff browser did not exit after graceful close; forcing terminate",
			zap.String("sessionID", sessionID),
			zap.Int("pid", info.PID),
			zap.Int("processGroupID", info.ProcessGroupID),
		)
		runtime.ForceTerminate(300 * time.Millisecond)
		runtime.WaitStopped(2 * time.Second)
	}
	session := service.popResourceSniffClosingSession(sessionID, generation)
	if session == nil {
		return
	}
	service.discardResourceMediaSnapshots(session.LastMediaIDs...)
	cleanupResourceSniffSessionContexts(session)
}

func (service *LibraryService) mapResourceSniffSession(session *resourceSniffSession) dto.ResourceSniffSession {
	if session == nil {
		return dto.ResourceSniffSession{}
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	if current := service.resourceSniffs[strings.TrimSpace(session.ID)]; current != nil {
		session = current
	}
	tabCount := resourceSniffActiveTabCountLocked(session)
	state, browserStatus := resourceSniffSessionStateAndBrowserStatus(
		session.State,
		session.Runtime != nil && session.Runtime.Status().Ready,
		tabCount,
	)
	activeID := strings.TrimSpace(session.ActiveID)
	if activeID != "" {
		if tab := session.Tabs[activeID]; tab == nil || resourceSniffIgnoredTargetURL(tab.CurrentURL) {
			activeID = ""
		}
	}
	var currentURL string
	var title string
	if activeID == "" {
		if tab := service.pickResourceSniffActiveTabLocked(session); tab != nil {
			activeID = tab.TargetID
			session.ActiveID = tab.TargetID
			session.CurrentURL = firstNonEmpty(strings.TrimSpace(tab.CurrentURL), session.CurrentURL, session.URL)
			session.Title = resourceCleanMetadataText(tab.Title)
			currentURL = session.CurrentURL
			title = session.Title
		} else {
			session.ActiveID = ""
			session.CurrentURL = ""
			session.Title = ""
		}
	} else if tab := session.Tabs[activeID]; tab != nil {
		currentURL = firstNonEmpty(strings.TrimSpace(tab.CurrentURL), session.CurrentURL, session.URL)
		title = resourceCleanMetadataText(firstNonEmpty(tab.Title, session.Title))
	}
	// Preserve the browser source explicitly so clients never infer ownership
	// from a URL, profile label, or process side effect.
	return dto.ResourceSniffSession{
		SessionID:      session.ID,
		State:          state,
		BrowserStatus:  browserStatus,
		URL:            session.URL,
		CurrentURL:     currentURL,
		Title:          title,
		ActiveTargetID: activeID,
		TabCount:       tabCount,
		Mode:           firstNonEmpty(strings.TrimSpace(session.Mode), "managed_profile"),
		BrowserID:      strings.TrimSpace(session.BrowserID),
		ProfileID:      strings.TrimSpace(session.ProfileID),
	}
}

func resourceSniffSessionStateAndBrowserStatus(state string, runtimeReady bool, tabCount int) (string, string) {
	if state == resourceSniffStateClosing {
		return resourceSniffStateClosing, resourceSniffBrowserStatusClosing
	}
	if !runtimeReady {
		return resourceSniffStateClosed, resourceSniffBrowserStatusClosed
	}
	if tabCount <= 0 {
		return state, resourceSniffBrowserStatusTabClosed
	}
	return state, resourceSniffBrowserStatusOpen
}

func resourceSniffActiveTabCountLocked(session *resourceSniffSession) int {
	if session == nil || len(session.Tabs) == 0 {
		return 0
	}
	count := 0
	for _, tab := range session.Tabs {
		if tab == nil || resourceSniffIgnoredTargetURL(tab.CurrentURL) {
			continue
		}
		count++
	}
	return count
}

func (service *LibraryService) watchResourceSniffTab(sessionID string, tab *resourceSniffTab) {
	if service == nil || tab == nil || tab.Ctx == nil || tab.Capture == nil {
		return
	}
	go func() {
		<-tab.Ctx.Done()
		service.removeResourceSniffTab(sessionID, tab.TargetID, "target_context_done")
	}()
	chromedp.ListenTarget(tab.Ctx, func(ev any) {
		switch event := ev.(type) {
		case *network.EventRequestWillBeSent:
			if event.RedirectResponse != nil {
				tab.Capture.recordResponse(
					event.RequestID,
					event.RedirectResponse.URL,
					event.RedirectResponse.Status,
					event.RedirectResponse.MimeType,
					event.RedirectResponse.Headers,
					event.Type,
					event.RedirectHasExtraInfo,
				)
			}
			if event.Request != nil {
				tab.Capture.recordRequest(event.RequestID, event.Request.URL, event.DocumentURL, event.Request.Headers)
				service.touchResourceSniffTab(sessionID, tab.TargetID, event.Request.URL)
			}
		case *network.EventRequestWillBeSentExtraInfo:
			tab.Capture.recordRequestHeaders(event.RequestID, event.Headers)
		case *network.EventResponseReceived:
			if event.Response != nil {
				tab.Capture.recordResponse(event.RequestID, event.Response.URL, event.Response.Status, event.Response.MimeType, event.Response.Headers, event.Type, event.HasExtraInfo)
				service.touchResourceSniffTab(sessionID, tab.TargetID, event.Response.URL)
			}
		case *network.EventLoadingFinished:
			if tab.Capture.shouldCaptureResponseBody(event.RequestID) {
				go captureResourceSniffResponseBody(tab.Ctx, tab.Capture, event.RequestID)
			}
			tab.Capture.markRequestFinished(event.RequestID)
		case *network.EventLoadingFailed:
			tab.Capture.markRequestFinished(event.RequestID)
		}
	})
}

func captureResourceSniffResponseBody(ctx context.Context, capture *resourceCaptureState, requestID network.RequestID) {
	if ctx == nil || capture == nil {
		return
	}
	bodyCtx, cancel := context.WithTimeout(ctx, resourceSniffResponseBodyTimeout)
	defer cancel()
	var body []byte
	err := chromedp.Run(bodyCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		body, err = network.GetResponseBody(requestID).Do(ctx)
		return err
	}))
	if err != nil {
		return
	}
	capture.recordResponseBody(requestID, body)
}

func (service *LibraryService) enableResourceSniffTargetDiscovery(sessionID string) {
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok || !resourceSniffMayDiscoverTargets(session) {
		return
	}
	watcher, err := browsercdp.WatchPageTargets(session.Runtime, func(event browsercdp.TargetEvent) {
		switch event.Kind {
		case browsercdp.TargetEventCreated, browsercdp.TargetEventInfoChanged:
			service.handleResourceSniffTargetInfo(sessionID, event.Info)
		case browsercdp.TargetEventAttached:
			service.rememberResourceSniffTargetSessionID(sessionID, event.TargetID, event.SessionID)
		case browsercdp.TargetEventDestroyed, browsercdp.TargetEventCrashed:
			service.removeResourceSniffTab(sessionID, event.TargetID, string(event.Kind))
		case browsercdp.TargetEventDetached:
			service.handleResourceSniffTargetDetached(sessionID, event.TargetID, event.SessionID)
		}
	})
	if err != nil {
		fields := []zap.Field{zap.String("sessionID", sessionID)}
		fields = append(fields, resourceSniffErrorLogFields("resource_sniff_target_discovery_failed", err)...)
		zap.L().Warn("resource sniff target discovery failed", fields...)
		return
	}
	service.updateResourceSniffSession(sessionID, func(current *resourceSniffSession) {
		if current.Targets != nil {
			current.Targets.Stop()
		}
		current.Targets = watcher
		if current.TargetID != "" && current.TabCtx != nil {
			watcher.RememberTargetSession(current.TargetID, browsercdp.TargetSessionIDFromContext(current.TabCtx))
		}
		for _, tab := range current.Tabs {
			if tab != nil {
				watcher.RememberTargetSession(tab.TargetID, tab.TargetSessionID)
			}
		}
	})
	zap.L().Debug("resource sniff target discovery enabled", zap.String("sessionID", sessionID))
}

func (service *LibraryService) handleResourceSniffTargetInfo(sessionID string, info *targetpkg.Info) {
	if info == nil || info.Type != "page" {
		return
	}
	if session, ok := service.getResourceSniffSession(sessionID); !ok ||
		(session.Runtime != nil && session.Runtime.IsBorrowed() && !session.Runtime.BorrowedPageTargetInScope(info)) {
		return
	}
	targetID := strings.TrimSpace(string(info.TargetID))
	if targetID == "" {
		return
	}
	if resourceSniffIgnoredTargetURL(info.URL) {
		if resourceSniffTrackPendingTargetURL(info.URL) {
			if service.keepPendingResourceSniffTab(sessionID, targetID) {
				return
			}
			go service.attachResourceSniffTarget(sessionID, targetID, info.URL, info.Title, false)
			return
		}
		service.removeResourceSniffTab(sessionID, targetID, "ignored_target_url")
		return
	}
	go service.attachResourceSniffTarget(sessionID, targetID, info.URL, info.Title, false)
}

func (service *LibraryService) attachResourceSniffTarget(sessionID string, targetID string, pageURL string, title string, markActive bool) {
	targetID = strings.TrimSpace(targetID)
	if service == nil || targetID == "" {
		return
	}
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok || session.Runtime == nil || !session.Runtime.Status().Ready {
		return
	}
	service.resourceSniffMu.Lock()
	if current := service.resourceSniffs[sessionID]; current != nil {
		if current.Tabs == nil {
			current.Tabs = map[string]*resourceSniffTab{}
		}
		if tab := current.Tabs[targetID]; tab != nil {
			pageChanged := strings.TrimSpace(pageURL) != "" &&
				strings.TrimSpace(pageURL) != strings.TrimSpace(tab.CurrentURL)
			if pageChanged {
				tab.PendingNavigation = true
			}
			tab.CurrentURL = firstNonEmpty(strings.TrimSpace(pageURL), tab.CurrentURL)
			tab.Title = firstNonEmpty(resourceCleanMetadataText(title), tab.Title)
			if markActive {
				tab.LastSeen = time.Now()
				current.ActiveID = targetID
			}
			if markActive || current.ActiveID == targetID || current.ActiveID == "" {
				current.ActiveID = firstNonEmpty(current.ActiveID, targetID)
				current.CurrentURL = firstNonEmpty(strings.TrimSpace(tab.CurrentURL), current.CurrentURL, current.URL)
				current.Title = resourceCleanMetadataText(tab.Title)
			}
			service.resourceSniffMu.Unlock()
			return
		}
		if current.Attaching == nil {
			current.Attaching = map[string]struct{}{}
		}
		if _, attaching := current.Attaching[targetID]; attaching {
			service.resourceSniffMu.Unlock()
			return
		}
		current.Attaching[targetID] = struct{}{}
	} else {
		service.resourceSniffMu.Unlock()
		return
	}
	service.resourceSniffMu.Unlock()

	var tabCtx context.Context
	var tabCancel context.CancelFunc
	var attachErr error
	if session.Runtime.IsBorrowed() {
		tabCtx, tabCancel, _, attachErr = browsercdp.AttachBorrowedPageTarget(session.Runtime, targetID, 5*time.Second)
	} else {
		tabCtx, tabCancel = chromedp.NewContext(session.Runtime.BrowserContext(), chromedp.WithTargetID(targetpkg.ID(targetID)))
		attachErr = chromedp.Run(tabCtx)
	}
	if attachErr != nil {
		service.clearResourceSniffTargetAttaching(sessionID, targetID)
		service.logResourceSniffTargetAttachFailure(sessionID, targetID, "attach", attachErr)
		return
	}
	if err := chromedp.Run(tabCtx, resourceSniffNetworkEnable()); err != nil {
		service.clearResourceSniffTargetAttaching(sessionID, targetID)
		if tabCancel != nil {
			tabCancel()
		}
		service.logResourceSniffTargetAttachFailure(sessionID, targetID, "network_enable", err)
		return
	}
	pendingNavigation := true
	tab := &resourceSniffTab{
		TargetID:          targetID,
		TargetSessionID:   browsercdp.TargetSessionIDFromContext(tabCtx),
		Ctx:               tabCtx,
		Cancel:            tabCancel,
		Capture:           newResourceCaptureState(),
		CurrentURL:        strings.TrimSpace(pageURL),
		Title:             resourceCleanMetadataText(title),
		PendingNavigation: pendingNavigation,
		LastSeen:          time.Now(),
	}
	service.watchResourceSniffTab(sessionID, tab)
	service.resourceSniffMu.Lock()
	stored := false
	sessionGone := false
	if current := service.resourceSniffs[sessionID]; current != nil {
		delete(current.Attaching, targetID)
		if current.Tabs == nil {
			current.Tabs = map[string]*resourceSniffTab{}
		}
		if existing := current.Tabs[targetID]; existing == nil {
			current.Tabs[targetID] = tab
			stored = true
			if markActive {
				current.ActiveID = targetID
			}
			if markActive || current.ActiveID == targetID || current.ActiveID == "" {
				current.ActiveID = firstNonEmpty(current.ActiveID, targetID)
				current.CurrentURL = firstNonEmpty(strings.TrimSpace(tab.CurrentURL), current.CurrentURL, current.URL)
				current.Title = resourceCleanMetadataText(tab.Title)
			}
		} else {
			pageChanged := strings.TrimSpace(pageURL) != "" &&
				strings.TrimSpace(pageURL) != strings.TrimSpace(existing.CurrentURL)
			if pageChanged {
				existing.PendingNavigation = true
			}
			existing.CurrentURL = firstNonEmpty(strings.TrimSpace(pageURL), existing.CurrentURL)
			existing.Title = firstNonEmpty(resourceCleanMetadataText(title), existing.Title)
			if markActive {
				existing.LastSeen = time.Now()
				current.ActiveID = targetID
			}
			if markActive || current.ActiveID == targetID || current.ActiveID == "" {
				current.ActiveID = firstNonEmpty(current.ActiveID, targetID)
				current.CurrentURL = firstNonEmpty(strings.TrimSpace(existing.CurrentURL), current.CurrentURL, current.URL)
				current.Title = resourceCleanMetadataText(existing.Title)
			}
		}
	} else {
		sessionGone = true
	}
	service.resourceSniffMu.Unlock()
	if !stored && sessionGone {
		tabCancel()
	}
}

func resourceSniffNetworkEnable() *network.EnableParams {
	return network.Enable().
		WithMaxTotalBufferSize(resourceSniffNetworkTotalBufferBytes).
		WithMaxResourceBufferSize(resourceSniffNetworkResourceBytes)
}

func resourceSniffNavigate(rawURL string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, errorText, _, err := page.Navigate(strings.TrimSpace(rawURL)).Do(ctx)
		if err != nil {
			return err
		}
		if strings.TrimSpace(errorText) != "" {
			return fmt.Errorf("page navigate failed: %s", strings.TrimSpace(errorText))
		}
		return nil
	})
}

func (service *LibraryService) keepPendingResourceSniffTab(sessionID string, targetID string) bool {
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || session.Tabs == nil {
		return false
	}
	tab := session.Tabs[strings.TrimSpace(targetID)]
	return tab != nil && tab.PendingNavigation
}

func (service *LibraryService) clearResourceSniffTargetAttaching(sessionID string, targetID string) {
	if service == nil {
		return
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || session.Attaching == nil {
		return
	}
	delete(session.Attaching, strings.TrimSpace(targetID))
}

func (service *LibraryService) logResourceSniffTargetAttachFailure(sessionID string, targetID string, stage string, err error) {
	if service == nil || err == nil {
		return
	}
	service.resourceSniffMu.Lock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || session.State != resourceSniffStateRunning {
		service.resourceSniffMu.Unlock()
		return
	}
	mode := normalizeResourceSniffMode(session.Mode)
	service.resourceSniffMu.Unlock()
	zap.L().Warn(
		"resource sniff page attach failed",
		zap.String("sessionID", strings.TrimSpace(sessionID)),
		zap.String("targetID", strings.TrimSpace(targetID)),
		zap.String("mode", mode),
		zap.String("stage", strings.TrimSpace(stage)),
		zap.Inline(zapFieldsObject(resourceSniffErrorLogFields("resource_sniff_page_attach_failed", err))),
	)
}

func (service *LibraryService) rememberResourceSniffTargetSessionID(sessionID string, targetID string, targetSessionID string) {
	if service == nil {
		return
	}
	targetID = strings.TrimSpace(targetID)
	targetSessionID = strings.TrimSpace(targetSessionID)
	if targetID == "" || targetSessionID == "" {
		return
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || session.Tabs == nil {
		return
	}
	if tab := session.Tabs[targetID]; tab != nil {
		// The browser emits attach events for every CDP client. Keep the session
		// that owns this tab instead of allowing DevTools or another client to
		// replace it; detach recovery must only react to XiaDown's own session.
		if strings.TrimSpace(tab.TargetSessionID) == "" {
			contextSessionID := browsercdp.TargetSessionIDFromContext(tab.Ctx)
			if contextSessionID != "" && contextSessionID == targetSessionID {
				tab.TargetSessionID = contextSessionID
			}
		}
	}
}

func (service *LibraryService) handleResourceSniffTargetDetached(sessionID string, targetID string, targetSessionID string) {
	if service == nil {
		return
	}
	// Remove only XiaDown's detached page session. If the user-owned tab still
	// exists, the normal one-second target sync attaches a fresh CDP session;
	// stopping a sniff never reaches this path because its state is closing.
	service.removeResourceSniffTabMatchingSession(
		sessionID,
		targetID,
		targetSessionID,
		true,
		string(browsercdp.TargetEventDetached),
	)
}

func resourceSniffTabMatchesDetach(tab *resourceSniffTab, targetID string, targetSessionID string) bool {
	if tab == nil {
		return false
	}
	targetID = strings.TrimSpace(targetID)
	targetSessionID = strings.TrimSpace(targetSessionID)
	storedTargetID := strings.TrimSpace(tab.TargetID)
	storedSessionID := strings.TrimSpace(tab.TargetSessionID)
	return targetID != "" && targetSessionID != "" &&
		storedTargetID == targetID && storedSessionID != "" && storedSessionID == targetSessionID
}

func (service *LibraryService) syncResourceSniffTargets(sessionID string) {
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok || !resourceSniffMayDiscoverTargets(session) || !session.Runtime.Status().Ready {
		return
	}
	manager := session.Runtime.TargetManager()
	if manager == nil {
		return
	}
	seen := map[string]struct{}{}
	for _, info := range manager.ListPageTargets() {
		if info == nil || info.Type != "page" {
			continue
		}
		if session.Runtime.IsBorrowed() && !session.Runtime.BorrowedPageTargetInScope(info) {
			continue
		}
		targetID := strings.TrimSpace(string(info.TargetID))
		if targetID == "" {
			continue
		}
		if resourceSniffIgnoredTargetURL(info.URL) {
			if resourceSniffTrackPendingTargetURL(info.URL) {
				seen[targetID] = struct{}{}
				if service.keepPendingResourceSniffTab(sessionID, targetID) {
					continue
				}
				service.attachResourceSniffTarget(sessionID, targetID, info.URL, info.Title, false)
				continue
			}
			service.removeResourceSniffTab(sessionID, targetID, "ignored_target_url")
			continue
		}
		seen[targetID] = struct{}{}
		service.attachResourceSniffTarget(sessionID, targetID, info.URL, info.Title, false)
	}
	service.removeMissingResourceSniffTabs(sessionID, seen)
}

func resourceSniffMayDiscoverTargets(session *resourceSniffSession) bool {
	if session == nil || session.Runtime == nil {
		return false
	}
	return resourceSniffTargetDiscoveryAllowed(session.Mode, session.Runtime.Ownership())
}

func resourceSniffTargetDiscoveryAllowed(mode string, ownership browsercdp.RuntimeOwnership) bool {
	switch normalizeResourceSniffMode(mode) {
	case "managed_profile":
		return ownership == browsercdp.RuntimeOwnershipOwned
	case "current_browser":
		return ownership == browsercdp.RuntimeOwnershipBorrowed
	default:
		return false
	}
}

func (service *LibraryService) resolveResourceSniffActiveTab(sessionID string) *resourceSniffTab {
	service.syncResourceSniffTargets(sessionID)
	service.probeResourceSniffSessionPageIdentity(sessionID, resourceSniffIdentityProbe)
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok {
		return nil
	}
	return service.pickResourceSniffActiveTab(session.ID)
}

func (service *LibraryService) monitorResourceSniffSession(sessionID string) {
	ticker := time.NewTicker(resourceSniffIdentityPoll)
	defer ticker.Stop()
	lastSync := time.Time{}
	for {
		session, ok := service.getResourceSniffSession(sessionID)
		if !ok {
			return
		}
		if session.State == resourceSniffStateClosing {
			return
		}
		if session.Runtime == nil || !session.Runtime.Status().Ready {
			service.updateResourceSniffSession(sessionID, func(current *resourceSniffSession) {
				current.State = resourceSniffStateClosed
			})
			return
		}
		if lastSync.IsZero() || time.Since(lastSync) >= resourceSniffTargetSyncPoll {
			service.syncResourceSniffTargets(sessionID)
			lastSync = time.Now()
		}
		service.probeResourceSniffSessionPageIdentity(sessionID, resourceSniffIdentityProbe)
		<-ticker.C
	}
}

func (service *LibraryService) probeResourceSniffSessionPageIdentity(sessionID string, timeout time.Duration) {
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok || session.Runtime == nil || !session.Runtime.Status().Ready {
		return
	}
	if timeout <= 0 {
		timeout = resourceSniffIdentityProbe
	}
	tabs := service.resourceSniffTabs(sessionID)
	if len(tabs) == 0 {
		return
	}
	type identityResult struct {
		targetID string
		identity map[string]any
	}
	results := make(chan identityResult, len(tabs))
	var wg sync.WaitGroup
	for _, tab := range tabs {
		if tab == nil || tab.Ctx == nil {
			continue
		}
		wg.Add(1)
		go func(tab *resourceSniffTab) {
			defer wg.Done()
			identity := map[string]any{}
			probeCtx, cancel := context.WithTimeout(tab.Ctx, timeout)
			err := chromedp.Run(probeCtx, chromedp.Evaluate(resourcePageIdentityScript(), &identity))
			cancel()
			if err != nil {
				return
			}
			results <- identityResult{targetID: tab.TargetID, identity: identity}
		}(tab)
	}
	wg.Wait()
	close(results)
	for result := range results {
		service.updateResourceSniffTabIdentity(sessionID, result.targetID, result.identity)
	}
	session, ok = service.getResourceSniffSession(sessionID)
	if !ok {
		return
	}
	tab := service.pickResourceSniffActiveTab(session.ID)
	if tab == nil {
		return
	}
	service.updateResourceSniffSession(sessionID, func(current *resourceSniffSession) {
		current.ActiveID = tab.TargetID
		current.CurrentURL = firstNonEmpty(strings.TrimSpace(tab.CurrentURL), current.CurrentURL, current.URL)
		current.Title = resourceCleanMetadataText(tab.Title)
	})
}

func (service *LibraryService) resourceSniffTabs(sessionID string) []*resourceSniffTab {
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || len(session.Tabs) == 0 {
		return nil
	}
	result := make([]*resourceSniffTab, 0, len(session.Tabs))
	for _, tab := range session.Tabs {
		result = append(result, tab)
	}
	return result
}

func (service *LibraryService) updateResourceSniffTabIdentity(sessionID string, targetID string, identity map[string]any) {
	if service == nil || len(identity) == 0 {
		return
	}
	location := strings.TrimSpace(fmtString(identity["location"]))
	title := resourceCleanMetadataText(fmtString(identity["title"]))
	visibility := strings.TrimSpace(fmtString(identity["visibilityState"]))
	hasFocus := fmtBool(identity["hasFocus"])
	readyState := strings.ToLower(strings.TrimSpace(fmtString(identity["readyState"])))
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil {
		return
	}
	tab := session.Tabs[strings.TrimSpace(targetID)]
	if tab == nil {
		return
	}
	tab.CurrentURL = firstNonEmpty(location, tab.CurrentURL)
	tab.Title = firstNonEmpty(title, tab.Title)
	tab.Visibility = visibility
	tab.HasFocus = hasFocus
	if !resourceSniffIgnoredTargetURL(location) &&
		(readyState == "interactive" || readyState == "complete") {
		tab.PendingNavigation = false
	}
	if strings.EqualFold(visibility, "visible") {
		tab.LastSeen = time.Now()
		session.ActiveID = tab.TargetID
	}
}

func (service *LibraryService) touchResourceSniffTab(sessionID string, targetID string, rawURL string) {
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil {
		return
	}
	tab := session.Tabs[strings.TrimSpace(targetID)]
	if tab == nil {
		return
	}
	tab.LastSeen = time.Now()
}

func (service *LibraryService) removeResourceSniffTab(sessionID string, targetID string, reason string) {
	service.removeResourceSniffTabMatchingSession(sessionID, targetID, "", false, reason)
}

func (service *LibraryService) removeResourceSniffTabMatchingSession(
	sessionID string,
	targetID string,
	targetSessionID string,
	requireRunning bool,
	reason string,
) bool {
	if service == nil {
		return false
	}
	sessionID = strings.TrimSpace(sessionID)
	targetID = strings.TrimSpace(targetID)
	targetSessionID = strings.TrimSpace(targetSessionID)
	if targetID == "" && targetSessionID == "" {
		return false
	}
	service.resourceSniffMu.Lock()
	session := service.resourceSniffs[sessionID]
	if session == nil || len(session.Tabs) == 0 {
		service.resourceSniffMu.Unlock()
		return false
	}
	if requireRunning && session.State != resourceSniffStateRunning {
		service.resourceSniffMu.Unlock()
		return false
	}
	if targetID == "" {
		for currentTargetID, currentTab := range session.Tabs {
			if resourceSniffTabMatchesDetach(currentTab, currentTargetID, targetSessionID) {
				targetID = currentTargetID
				break
			}
		}
	}
	tab := session.Tabs[targetID]
	if tab == nil || (targetSessionID != "" && !resourceSniffTabMatchesDetach(tab, targetID, targetSessionID)) {
		service.resourceSniffMu.Unlock()
		return false
	}
	removedURL := strings.TrimSpace(tab.CurrentURL)
	removedTitle := strings.TrimSpace(tab.Title)
	wasActive := session.ActiveID == targetID
	delete(session.Tabs, targetID)
	if session.Attaching != nil {
		delete(session.Attaching, targetID)
	}
	if wasActive {
		session.ActiveID = ""
		if next := service.pickResourceSniffActiveTabLocked(session); next != nil {
			session.ActiveID = next.TargetID
			session.CurrentURL = firstNonEmpty(strings.TrimSpace(next.CurrentURL), session.CurrentURL, session.URL)
			session.Title = resourceCleanMetadataText(next.Title)
		} else {
			session.CurrentURL = ""
			session.Title = ""
		}
	}
	activeID := session.ActiveID
	remainingTabs := len(session.Tabs)
	activeTabCount := resourceSniffActiveTabCountLocked(session)
	service.resourceSniffMu.Unlock()

	zap.L().Debug(
		"resource sniff tab removed",
		zap.String("sessionID", strings.TrimSpace(sessionID)),
		zap.String("targetID", targetID),
		zap.String("reason", strings.TrimSpace(reason)),
		zap.Bool("wasActive", wasActive),
		zap.String("activeTargetID", activeID),
		zap.Int("remainingTabs", remainingTabs),
		zap.Int("activeTabCount", activeTabCount),
		zap.String("sourceRef", resourceSniffLogReference(removedURL)),
		zap.Bool("hadTitle", strings.TrimSpace(removedTitle) != ""),
	)
	if tab != nil && tab.Cancel != nil {
		tab.Cancel()
	}
	return true
}

func (service *LibraryService) removeResourceSniffTabBySessionID(sessionID string, targetSessionID string, reason string) {
	service.removeResourceSniffTabMatchingSession(sessionID, "", targetSessionID, false, reason)
}

func (service *LibraryService) removeMissingResourceSniffTabs(sessionID string, seen map[string]struct{}) {
	service.resourceSniffMu.Lock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || len(session.Tabs) == 0 {
		service.resourceSniffMu.Unlock()
		return
	}
	type removedTabLog struct {
		targetID       string
		url            string
		title          string
		wasActive      bool
		activeID       string
		remainingTabs  int
		activeTabCount int
	}
	changed := false
	cancelTabs := make([]context.CancelFunc, 0)
	removedLogs := make([]removedTabLog, 0)
	for targetID, tab := range session.Tabs {
		if _, ok := seen[targetID]; ok {
			continue
		}
		wasActive := session.ActiveID == targetID
		removedURL := ""
		removedTitle := ""
		if tab != nil {
			removedURL = strings.TrimSpace(tab.CurrentURL)
			removedTitle = strings.TrimSpace(tab.Title)
		}
		delete(session.Tabs, targetID)
		if session.Attaching != nil {
			delete(session.Attaching, targetID)
		}
		changed = true
		if tab != nil && tab.Cancel != nil {
			cancelTabs = append(cancelTabs, tab.Cancel)
		}
		if session.ActiveID == targetID {
			session.ActiveID = ""
		}
		removedLogs = append(removedLogs, removedTabLog{
			targetID:  targetID,
			url:       removedURL,
			title:     removedTitle,
			wasActive: wasActive,
		})
	}
	var cancelAfterUnlock []context.CancelFunc
	if changed && session.ActiveID == "" {
		if next := service.pickResourceSniffActiveTabLocked(session); next != nil {
			session.ActiveID = next.TargetID
			session.CurrentURL = firstNonEmpty(strings.TrimSpace(next.CurrentURL), session.CurrentURL, session.URL)
			session.Title = resourceCleanMetadataText(next.Title)
		} else {
			session.CurrentURL = ""
			session.Title = ""
		}
	}
	for index := range removedLogs {
		removedLogs[index].activeID = session.ActiveID
		removedLogs[index].remainingTabs = len(session.Tabs)
		removedLogs[index].activeTabCount = resourceSniffActiveTabCountLocked(session)
	}
	cancelAfterUnlock = cancelTabs
	service.resourceSniffMu.Unlock()
	for _, item := range removedLogs {
		zap.L().Debug(
			"resource sniff tab removed",
			zap.String("sessionID", strings.TrimSpace(sessionID)),
			zap.String("targetID", item.targetID),
			zap.String("reason", "sync_missing"),
			zap.Bool("wasActive", item.wasActive),
			zap.String("activeTargetID", item.activeID),
			zap.Int("remainingTabs", item.remainingTabs),
			zap.Int("activeTabCount", item.activeTabCount),
			zap.String("sourceRef", resourceSniffLogReference(item.url)),
			zap.Bool("hadTitle", strings.TrimSpace(item.title) != ""),
		)
	}
	for _, cancel := range cancelAfterUnlock {
		cancel()
	}
}

func (service *LibraryService) pickResourceSniffActiveTab(sessionID string) *resourceSniffTab {
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	return service.pickResourceSniffActiveTabLocked(session)
}

func (service *LibraryService) pickResourceSniffActiveTabLocked(session *resourceSniffSession) *resourceSniffTab {
	if session == nil || len(session.Tabs) == 0 {
		return nil
	}
	var best *resourceSniffTab
	bestScore := -1
	for _, tab := range session.Tabs {
		if tab == nil || resourceSniffIgnoredTargetURL(tab.CurrentURL) {
			continue
		}
		score := 0
		if strings.EqualFold(tab.Visibility, "visible") {
			score += 1000
		}
		if tab.HasFocus {
			score += 200
		}
		if tab.TargetID == session.ActiveID {
			score += 100
		}
		if strings.TrimSpace(tab.CurrentURL) != "" && !strings.HasPrefix(strings.ToLower(tab.CurrentURL), "about:") {
			score += 40
		}
		if isResourceDownloadURL(tab.CurrentURL) {
			score += 60
		}
		if !tab.LastSeen.IsZero() {
			age := time.Since(tab.LastSeen)
			if age < 30*time.Second {
				score += 30
			} else if age < 5*time.Minute {
				score += 10
			}
		}
		if best == nil || score > bestScore || (score == bestScore && tab.LastSeen.After(best.LastSeen)) {
			best = tab
			bestScore = score
		}
	}
	return best
}

func resourceSniffIgnoredTargetURL(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	return lower == "" ||
		lower == "about:blank" ||
		strings.HasPrefix(lower, "devtools://") ||
		strings.HasPrefix(lower, "chrome://") ||
		strings.HasPrefix(lower, "edge://")
}

func resourceSniffTrackPendingTargetURL(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	return lower == "" ||
		lower == "about:blank" ||
		lower == "chrome://newtab/" ||
		lower == "edge://newtab/"
}

func fmtString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(fmt.Sprint(value), "\""))
}

func fmtBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func cleanupResourceSniffSession(session *resourceSniffSession) {
	if session == nil {
		return
	}
	runtimeInfo := browsercdp.ProcessInfo{}
	if session.Runtime != nil {
		runtimeInfo = session.Runtime.ProcessInfo()
	}
	zap.L().Debug(
		"resource sniff cleanup started",
		zap.String("sessionID", session.ID),
		zap.String("targetID", session.TargetID),
		zap.Int("tabCount", len(session.Tabs)),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	cancelContexts := collectResourceSniffCancelContexts(session)
	runtimeStopped := true
	if session.Runtime != nil {
		session.Runtime.Stop()
		runtimeStopped = session.Runtime.Stopped()
	}
	zap.L().Debug(
		"resource sniff cleanup runtime stop finished",
		zap.String("sessionID", session.ID),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
		zap.Bool("runtimeStopped", runtimeStopped),
	)
	if !runtimeStopped {
		zap.L().Warn(
			"resource sniff browser runtime did not stop cleanly",
			zap.String("sessionID", session.ID),
			zap.Int("pid", runtimeInfo.PID),
			zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
		)
	}
	cancelResourceSniffContextsAsync(session.ID, cancelContexts)
}

func cleanupResourceSniffSessionContexts(session *resourceSniffSession) {
	if session == nil {
		return
	}
	cancelResourceSniffContextsAsync(session.ID, collectResourceSniffCancelContexts(session))
}

func collectResourceSniffCancelContexts(session *resourceSniffSession) []context.CancelFunc {
	if session == nil {
		return nil
	}
	cancelContexts := make([]context.CancelFunc, 0, len(session.Tabs)+1)
	if session.Targets != nil {
		session.Targets.Stop()
		session.Targets = nil
	}
	if session.Cancel != nil {
		cancelContexts = append(cancelContexts, session.Cancel)
	}
	for _, tab := range session.Tabs {
		if tab != nil && tab.Cancel != nil {
			cancelContexts = append(cancelContexts, tab.Cancel)
		}
	}
	return cancelContexts
}

func cancelResourceSniffContextsAsync(sessionID string, cancels []context.CancelFunc) {
	if len(cancels) == 0 {
		return
	}
	zap.L().Debug("resource sniff context cleanup scheduled", zap.String("sessionID", sessionID), zap.Int("count", len(cancels)))
	for index, cancel := range cancels {
		if cancel == nil {
			continue
		}
		go func(index int, cancel context.CancelFunc) {
			defer func() {
				if recovered := recover(); recovered != nil {
					fields := []zap.Field{zap.String("sessionID", sessionID), zap.Int("index", index)}
					fields = append(fields, resourceSniffPanicLogFields("resource_sniff_cleanup_panic", recovered)...)
					zap.L().Warn("resource sniff context cleanup panicked", fields...)
				}
			}()
			cancel()
			zap.L().Debug("resource sniff context cleanup finished", zap.String("sessionID", sessionID), zap.Int("index", index))
		}(index, cancel)
	}
}
