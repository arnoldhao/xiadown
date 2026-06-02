package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/library/dto"
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
	resourceSniffAuthProbe              = 2 * time.Second
	resourceSniffAuthProbeTimeout       = 1200 * time.Millisecond
	resourceSniffPageSnapshotTimeout    = 2 * time.Second
	resourceSniffResponseBodyTimeout    = 6 * time.Second
	resourceSniffGracefulCloseDispatch  = 3 * time.Second
	resourceSniffForceCloseTimeout      = 10 * time.Second
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
	AuthStatus        string
	AuthUser          string
	AuthSite          string
	AuthURL           string
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

type resourceSniffMediaSelection struct {
	Extractor    resourceExtractor
	PageURL      string
	PageDomain   string
	PageMeta     map[string]string
	Media        resourceMedia
	MediaOptions []resourceMedia
}

func (service *LibraryService) StartResourceSniff(ctx context.Context, request dto.StartResourceSniffRequest) (dto.StartResourceSniffResult, error) {
	resolvedURL, _, err := validateDownloadURL(request.URL)
	if err != nil {
		return dto.StartResourceSniffResult{}, err
	}
	knownExtractor := resourceKnownExtractor(resolvedURL)
	profilePath, err := service.resourceConnectorProfilePath(ctx, resolvedURL)
	if err != nil {
		return dto.StartResourceSniffResult{}, apperrors.Wrap(apperrors.CodeResourceBrowserLaunchFailed, "initialize resource browser profile", err)
	}
	if knownExtractor && strings.TrimSpace(profilePath) == "" {
		return dto.StartResourceSniffResult{Failure: resourceSniffProfileConnectionRequiredFailure(resourceSniffSiteName(resolvedURL, nil)).toDTO()}, nil
	}

	extraArgs := []string{"--autoplay-policy=no-user-gesture-required"}
	proxyURL := strings.TrimSpace(service.resolveYTDLPProxy(resolvedURL))
	if proxyURL != "" {
		extraArgs = append(extraArgs, "--proxy-server="+proxyURL)
	}
	preferredBrowser := service.preferredResourceBrowser(ctx)
	persistentProfile := strings.TrimSpace(profilePath) != ""
	launchFingerprint := resourceSniffLaunchFingerprint(preferredBrowser, profilePath, proxyURL, persistentProfile)

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
			zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
			zap.Error(err),
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
		zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
		zap.String("execPath", runtimeInfo.ExecutablePath),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	tabCtx, tabCancel, targetID, err := browsercdp.AttachOrCreatePageTarget(runtime, 5*time.Second)
	if err != nil {
		zap.L().Warn(
			"resource sniff cdp attach failed; stopping browser",
			zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
			zap.Int("pid", runtimeInfo.PID),
			zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
			zap.Error(err),
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
		PendingNavigation: true,
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
	}
	service.putResourceSniffSession(session)
	zap.L().Debug(
		"resource sniff session stored",
		zap.String("sessionID", sessionID),
		zap.String("targetID", tab.TargetID),
		zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
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

func (service *LibraryService) navigateResourceSniffInitialPage(sessionID string, tab *resourceSniffTab, resolvedURL string, runtimeInfo browsercdp.ProcessInfo) {
	if service == nil || tab == nil || tab.Ctx == nil {
		return
	}
	zap.L().Debug(
		"resource sniff initial navigation started",
		zap.String("sessionID", sessionID),
		zap.String("targetID", tab.TargetID),
		zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	if err := chromedp.Run(tab.Ctx, resourceSniffNetworkEnable(), resourceSniffNavigate(resolvedURL)); err != nil {
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
				zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
				zap.Int("pid", runtimeInfo.PID),
				zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
				zap.Error(err),
			)
			return
		}
		zap.L().Warn(
			"resource sniff initial navigation failed; cleaning session",
			zap.String("sessionID", sessionID),
			zap.String("targetID", tab.TargetID),
			zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
			zap.Int("pid", runtimeInfo.PID),
			zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
			zap.Error(err),
		)
		service.discardResourceMediaSnapshots(session.LastMediaIDs...)
		cleanupResourceSniffSession(session)
		return
	}
	zap.L().Debug(
		"resource sniff initial navigation finished",
		zap.String("sessionID", sessionID),
		zap.String("targetID", tab.TargetID),
		zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
		zap.Int("pid", runtimeInfo.PID),
		zap.Int("processGroupID", runtimeInfo.ProcessGroupID),
	)
	service.updateResourceSniffSession(sessionID, func(current *resourceSniffSession) {
		if current.TargetID == tab.TargetID {
			current.CurrentURL = resolvedURL
		}
		if current.Tabs != nil {
			if currentTab := current.Tabs[tab.TargetID]; currentTab != nil {
				currentTab.PendingNavigation = false
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

func (service *LibraryService) ParseResourceSniff(ctx context.Context, request dto.ParseResourceSniffRequest) (dto.ParseResourceSniffResponse, error) {
	session, ok := service.getResourceSniffSession(request.SessionID)
	if !ok {
		return dto.ParseResourceSniffResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session not found")
	}
	if session.Runtime == nil || !session.Runtime.Status().Ready {
		return dto.ParseResourceSniffResponse{}, apperrors.New(apperrors.CodeResourceBrowserLaunchFailed, "resource sniff browser is closed")
	}

	tab := service.resolveResourceSniffActiveTab(session.ID)
	if tab == nil || tab.Ctx == nil || tab.Capture == nil {
		return dto.ParseResourceSniffResponse{}, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff tab is unavailable")
	}

	pageMeta := map[string]string{}
	pageSubtitleMeta := map[string]string{}
	extractor := resourceExtractorForURL(firstNonEmpty(session.CurrentURL, session.URL))
	parseStartedAt := time.Now()
	parseCtx, parseCancel := context.WithTimeout(tab.Ctx, resourceSniffPageSnapshotTimeout)
	defer parseCancel()
	snapshotStartedAt := time.Now()
	runErr := chromedp.Run(
		parseCtx,
		chromedp.Evaluate(resourcePageKickScript(), nil),
		chromedp.Evaluate(extractor.PageMetaScript(), &pageMeta, resourceEvalAwaitPromise),
		chromedp.Evaluate(resourcePageSubtitleMetaScript(), &pageSubtitleMeta, resourceEvalAwaitPromise),
	)
	snapshotElapsed := time.Since(snapshotStartedAt)
	pageMeta = mergePageMeta(pageMeta, pageSubtitleMeta)
	accepted, rejected := tab.Capture.snapshot()
	apiResponses := tab.Capture.apiResponsesSnapshot()
	capturedSubtitles := tab.Capture.subtitlesSnapshot()
	pageURL := firstNonEmpty(strings.TrimSpace(pageMeta["location"]), tab.CurrentURL, session.CurrentURL, session.URL)
	if strings.TrimSpace(pageMeta["location"]) == "" && strings.TrimSpace(pageURL) != "" {
		if pageMeta == nil {
			pageMeta = map[string]string{}
		}
		pageMeta["location"] = pageURL
	}
	extractor = resourceExtractorForURL(pageURL)
	if failure, failed := resourcePreflightFailure(tab.Ctx, extractor, pageURL, pageMeta); failed {
		structuredMedia, noMediaHints := resourceStructuredDataFromAPIResponses(extractor, apiResponses)
		resourceSniffLogDouyinParseDecision(request.SessionID, "recommend_login_required", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, failure, runErr)
		return resourceSniffParseFailureResponse(failure), nil
	}
	snapshot := newResourceSniffSnapshot(pageURL, pageMeta, accepted, rejected, apiResponses, capturedSubtitles, parseStartedAt)
	structuredMedia, noMediaHints, augmentElapsed := service.extractResourceStructuredData(tab.Ctx, extractor, snapshot)
	resourceSniffLogDouyinParseTiming(request.SessionID, "after_snapshot_and_augment", pageURL, pageMeta,
		zap.Int64("snapshotElapsedMs", snapshotElapsed.Milliseconds()),
		zap.Int64("augmentElapsedMs", augmentElapsed.Milliseconds()),
		zap.Bool("snapshotTimedOut", errors.Is(runErr, context.DeadlineExceeded)),
		zap.Int("acceptedCount", len(accepted)),
		zap.Int("structuredMediaCount", len(structuredMedia)),
		zap.Error(runErr),
	)
	resourceSniffLogDouyinParseState(request.SessionID, "after_snapshot_and_augment", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, runErr)
	if failure, failed := resourceNoMediaHintFailure(resourceExtractorForURL(pageURL), pageMeta, noMediaHints, parseStartedAt); failed {
		resourceSniffLogDouyinParseDecision(request.SessionID, "pre_selection_no_media_hint", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, failure, runErr)
		return resourceSniffParseFailureResponse(failure), nil
	}
	selectionStartedAt := time.Now()
	selection, ok := service.selectResourceSniffMedia(pageURL, pageMeta, accepted, structuredMedia, parseStartedAt)
	selectionElapsed := time.Since(selectionStartedAt)
	extractor = selection.Extractor
	pageMeta = selection.PageMeta
	pageURL = selection.PageURL
	resourceSniffLogDouyinParseTiming(request.SessionID, "after_selection", pageURL, pageMeta,
		zap.Int64("snapshotElapsedMs", snapshotElapsed.Milliseconds()),
		zap.Int64("augmentElapsedMs", augmentElapsed.Milliseconds()),
		zap.Int64("selectionElapsedMs", selectionElapsed.Milliseconds()),
		zap.Int64("totalElapsedMs", time.Since(parseStartedAt).Milliseconds()),
		zap.Bool("selected", ok),
		zap.Int("acceptedCount", len(accepted)),
		zap.Int("structuredMediaCount", len(structuredMedia)),
		zap.Error(runErr),
	)
	if runErr != nil && !ok {
		if failure, failed := resourceNoMediaHintFailure(extractor, pageMeta, noMediaHints, parseStartedAt); failed {
			resourceSniffLogDouyinParseDecision(request.SessionID, "snapshot_error_no_media_hint", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, failure, runErr)
			return resourceSniffParseFailureResponse(failure), nil
		}
		if errors.Is(runErr, context.DeadlineExceeded) {
			failure := resourceSniffNoMediaDetectedFailure(resourceSniffSiteName(pageURL, extractor))
			resourceSniffLogDouyinParseDecision(request.SessionID, "snapshot_timeout_no_media", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, failure, runErr)
			return resourceSniffParseFailureResponse(failure), nil
		}
		return dto.ParseResourceSniffResponse{}, apperrors.Wrap(apperrors.CodeResourceResolveFailed, "parse resource sniff", runErr)
	}
	if !ok {
		if extractor.VerificationRequired(pageMeta, rejected) {
			failure := resourceSniffVerificationRequiredFailure(resourceSniffSiteName(pageURL, extractor))
			resourceSniffLogDouyinParseDecision(request.SessionID, "verification_required", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, failure, runErr)
			return resourceSniffParseFailureResponse(failure), nil
		}
		if failure, failed := resourceNoMediaHintFailure(extractor, pageMeta, noMediaHints, parseStartedAt); failed {
			resourceSniffLogDouyinParseDecision(request.SessionID, "post_selection_no_media_hint", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, failure, runErr)
			return resourceSniffParseFailureResponse(failure), nil
		}
		failure := resourceSniffNoMediaDetectedFailure(resourceSniffSiteName(pageURL, extractor))
		resourceSniffLogDouyinParseDecision(request.SessionID, "no_media_detected", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, failure, runErr)
		return resourceSniffParseFailureResponse(failure), nil
	}
	media := selection.Media
	mediaOptions := selection.MediaOptions
	subtitles := resourceSubtitlesForPage(pageURL, pageMeta, capturedSubtitles, structuredMedia, parseStartedAt)
	mediaOptions = attachResourceSubtitlesToMediaOptions(mediaOptions, subtitles)
	if len(mediaOptions) > 0 {
		media = mediaOptions[0]
	} else {
		media.Subtitles = dedupeResourceSubtitles(append(media.Subtitles, subtitles...))
	}
	formatOptions, mediaIDs, mediaID := service.putResourceMediaSnapshotsForFormats(mediaOptions)
	if len(formatOptions) == 0 {
		failure := resourceSniffNoMediaDetectedFailure(resourceSniffSiteName(pageURL, extractor))
		resourceSniffLogDouyinParseDecision(request.SessionID, "empty_format_options", pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, failure, runErr)
		return resourceSniffParseFailureResponse(failure), nil
	}
	resourceSniffLogDouyinMediaDecision(request.SessionID, pageURL, pageMeta, accepted, rejected, structuredMedia, noMediaHints, media, len(formatOptions), runErr)
	var oldMediaIDs []string
	service.updateResourceSniffSession(session.ID, func(current *resourceSniffSession) {
		oldMediaIDs = append(oldMediaIDs, current.LastMediaIDs...)
		current.LastMedia = &media
		current.LastMediaIDs = append([]string(nil), mediaIDs...)
		current.CurrentURL = pageURL
		current.Title = resourceCleanMetadataText(pageMeta["title"])
		current.ActiveID = tab.TargetID
	})
	service.discardResourceMediaSnapshots(oldMediaIDs...)

	mediaResponse := dto.ParseYTDLPDownloadResponse{
		Title:             media.Title,
		Domain:            media.Domain,
		Extractor:         media.Extractor,
		Author:            media.Author,
		ThumbnailURL:      media.ThumbnailURL,
		PageURL:           pageURL,
		ResourceSessionID: session.ID,
		ResourceMediaID:   mediaID,
		Formats:           formatOptions,
		Subtitles:         resourceSubtitleOptions(subtitles),
	}
	return dto.ParseResourceSniffResponse{Media: &mediaResponse}, nil
}

func resourceNoMediaHintFailure(extractor resourceExtractor, pageMeta map[string]string, hints []resourceNoMediaHint, since time.Time) (resourceSniffFailure, bool) {
	if extractor == nil {
		return resourceSniffFailure{}, false
	}
	provider, ok := extractor.(resourceNoMediaHintProvider)
	if !ok {
		return resourceSniffFailure{}, false
	}
	return provider.NoMediaFailure(pageMeta, hints, since)
}

func resourceSniffLogDouyinParseState(sessionID string, stage string, pageURL string, pageMeta map[string]string, accepted []resourceCandidate, rejected []resourceRejectedCandidate, structuredMedia []resourceStructuredMedia, hints []resourceNoMediaHint, runErr error) {
	if !resourceSniffIsDouyinPage(pageURL, pageMeta) {
		return
	}
	fields := resourceSniffDouyinLogFields(sessionID, stage, pageURL, pageMeta, accepted, rejected, structuredMedia, hints, runErr)
	zap.L().Debug("resource sniff douyin parse state", fields...)
}

func resourceSniffLogDouyinParseDecision(sessionID string, stage string, pageURL string, pageMeta map[string]string, accepted []resourceCandidate, rejected []resourceRejectedCandidate, structuredMedia []resourceStructuredMedia, hints []resourceNoMediaHint, failure resourceSniffFailure, runErr error) {
	if !resourceSniffIsDouyinPage(pageURL, pageMeta) {
		return
	}
	fields := resourceSniffDouyinLogFields(sessionID, stage, pageURL, pageMeta, accepted, rejected, structuredMedia, hints, runErr)
	fields = append(fields,
		zap.String("failureCode", failure.Code),
		zap.String("failureAction", failure.Action),
		zap.Bool("failureRetryable", failure.Retryable),
	)
	zap.L().Debug("resource sniff douyin parse decision", fields...)
}

func resourceSniffLogDouyinMediaDecision(sessionID string, pageURL string, pageMeta map[string]string, accepted []resourceCandidate, rejected []resourceRejectedCandidate, structuredMedia []resourceStructuredMedia, hints []resourceNoMediaHint, media resourceMedia, formatCount int, runErr error) {
	if !resourceSniffIsDouyinPage(pageURL, pageMeta) {
		return
	}
	fields := resourceSniffDouyinLogFields(sessionID, "media_selected", pageURL, pageMeta, accepted, rejected, structuredMedia, hints, runErr)
	fields = append(fields,
		zap.String("mediaURL", resourceSniffLogURL(media.URL, 240)),
		zap.String("mediaExtractor", media.Extractor),
		zap.String("mediaTitle", media.Title),
		zap.String("mediaAuthor", media.Author),
		zap.Int("formatCount", formatCount),
	)
	zap.L().Debug("resource sniff douyin parse decision", fields...)
}

func resourceSniffLogDouyinParseTiming(sessionID string, stage string, pageURL string, pageMeta map[string]string, timingFields ...zap.Field) {
	if !resourceSniffIsDouyinPage(pageURL, pageMeta) {
		return
	}
	fields := []zap.Field{
		zap.String("sessionID", sessionID),
		zap.String("stage", stage),
		zap.String("pageURL", resourceSniffLogURL(firstNonEmpty(pageMeta["location"], pageURL), 240)),
		zap.String("awemeID", strings.TrimSpace(pageMeta["awemeID"])),
		zap.String("visibleAwemeID", strings.TrimSpace(pageMeta["visibleAwemeID"])),
	}
	fields = append(fields, timingFields...)
	zap.L().Debug("resource sniff douyin parse timing", fields...)
}

func resourceSniffDouyinLogFields(sessionID string, stage string, pageURL string, pageMeta map[string]string, accepted []resourceCandidate, rejected []resourceRejectedCandidate, structuredMedia []resourceStructuredMedia, hints []resourceNoMediaHint, runErr error) []zap.Field {
	reason, blocker, verificationBlocked := resourceDouyinBlockReason(pageMeta, rejected)
	fields := []zap.Field{
		zap.String("sessionID", sessionID),
		zap.String("stage", stage),
		zap.String("pageURL", resourceSniffLogURL(firstNonEmpty(pageMeta["location"], pageURL), 240)),
		zap.String("title", resourceSniffTruncate(pageMeta["title"], 160)),
		zap.String("awemeID", strings.TrimSpace(pageMeta["awemeID"])),
		zap.String("visibleAwemeID", strings.TrimSpace(pageMeta["visibleAwemeID"])),
		zap.String("visibleAwemeIDs", resourceSniffTruncate(pageMeta["visibleAwemeIDs"], 240)),
		zap.String("visibleLiveID", strings.TrimSpace(pageMeta["visibleLiveID"])),
		zap.String("visibleLiveIDs", resourceSniffTruncate(pageMeta["visibleLiveIDs"], 240)),
		zap.String("videoCount", strings.TrimSpace(pageMeta["videoCount"])),
		zap.String("videoWidth", strings.TrimSpace(pageMeta["videoWidth"])),
		zap.String("videoHeight", strings.TrimSpace(pageMeta["videoHeight"])),
		zap.String("videoCurrentSrc", resourceSniffLogURL(pageMeta["videoCurrentSrc"], 240)),
		zap.String("videoSrc", resourceSniffLogURL(pageMeta["videoSrc"], 240)),
		zap.String("videoItems", resourceSniffLogVideoItems(pageMeta["videoItems"], 360)),
		zap.Strings("currentNoMediaIDs", resourceDouyinNoMediaHintIDsFromPageMeta(pageMeta)),
		zap.Int("noMediaHintCount", len(hints)),
		zap.Strings("liveHints", resourceSniffNoMediaHintSummaries(hints, 8)),
		zap.Int("structuredMediaCount", len(structuredMedia)),
		zap.Strings("structuredMedia", resourceSniffStructuredMediaSummaries(structuredMedia, 8)),
		zap.Int("acceptedCount", len(accepted)),
		zap.Int("rejectedCount", len(rejected)),
		zap.Strings("acceptedCandidates", resourceSniffCandidateSummaries(accepted, 5)),
		zap.Strings("verificationRejectedSignals", resourceSniffRejectedVerificationSummaries(rejected, 8)),
		zap.Bool("verificationBlocked", verificationBlocked),
		zap.String("verificationReason", reason),
		zap.String("verificationBlocker", resourceSniffLogURL(blocker, 240)),
	}
	if runErr != nil {
		fields = append(fields, zap.Error(runErr))
	}
	return fields
}

func resourceSniffIsDouyinPage(pageURL string, pageMeta map[string]string) bool {
	rawURL := firstNonEmpty(pageMeta["location"], pageURL)
	if strings.EqualFold(resourceExtractorForURL(rawURL).Name(), (resourceDouyinSiteRules{}).Name()) {
		return true
	}
	return strings.Contains(strings.ToLower(rawURL), "douyin.com")
}

func resourceSniffNoMediaHintSummaries(hints []resourceNoMediaHint, limit int) []string {
	if len(hints) == 0 || limit == 0 {
		return nil
	}
	result := make([]string, 0, minInt(len(hints), limit))
	for _, hint := range hints {
		if limit > 0 && len(result) >= limit {
			break
		}
		result = append(result, fmt.Sprintf(
			"kind=%s id=%s alt=%s source=%s",
			strings.TrimSpace(hint.Kind),
			strings.TrimSpace(hint.ID),
			strings.Join(dedupeResourceStrings(hint.AltIDs), "|"),
			resourceSniffLogURL(hint.SourceURL, 160),
		))
	}
	return result
}

func resourceSniffStructuredMediaSummaries(items []resourceStructuredMedia, limit int) []string {
	if len(items) == 0 || limit == 0 {
		return nil
	}
	result := make([]string, 0, minInt(len(items), limit))
	for _, item := range items {
		if limit > 0 && len(result) >= limit {
			break
		}
		result = append(result, fmt.Sprintf(
			"id=%s quality=%d url=%s",
			strings.TrimSpace(item.ID),
			item.QualityHeight,
			resourceSniffLogURL(item.VideoURL, 160),
		))
	}
	return result
}

func resourceSniffCandidateSummaries(items []resourceCandidate, limit int) []string {
	if len(items) == 0 || limit == 0 {
		return nil
	}
	result := make([]string, 0, minInt(len(items), limit))
	for _, item := range items {
		if limit > 0 && len(result) >= limit {
			break
		}
		result = append(result, fmt.Sprintf(
			"score=%d status=%d type=%s url=%s",
			item.score,
			item.status,
			item.mimeType,
			resourceSniffLogURL(item.url, 160),
		))
	}
	return result
}

func resourceSniffRejectedVerificationSummaries(items []resourceRejectedCandidate, limit int) []string {
	if len(items) == 0 || limit == 0 {
		return nil
	}
	result := make([]string, 0, minInt(len(items), limit))
	for _, item := range items {
		if limit > 0 && len(result) >= limit {
			break
		}
		reason := resourceDouyinRejectedVerificationSignal(item)
		if reason == "" {
			continue
		}
		result = append(result, fmt.Sprintf(
			"signal=%s reject=%s status=%d type=%s url=%s",
			reason,
			item.reason,
			item.status,
			firstNonEmpty(item.mimeType, item.resourceType),
			resourceSniffLogURL(item.url, 160),
		))
	}
	return result
}

func resourceSniffLogURL(value string, limit int) string {
	return resourceSniffTruncate(resourceSniffSanitizeLogURL(value), limit)
}

func resourceSniffSanitizeLogURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return resourceSniffStripLogURLSecrets(trimmed)
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	sanitized := strings.TrimSpace(parsed.String())
	if sanitized == "" {
		return resourceSniffStripLogURLSecrets(trimmed)
	}
	return resourceSniffStripLogURLSecrets(sanitized)
}

func resourceSniffStripLogURLSecrets(value string) string {
	if idx := strings.IndexAny(value, "?#"); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return strings.TrimSpace(value)
}

func resourceSniffLogVideoItems(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(trimmed), &items); err == nil {
		for _, item := range items {
			for _, key := range []string{"currentSrc", "src", "poster", "name", "url"} {
				raw, ok := item[key].(string)
				if !ok || !resourceSniffLooksLikeURL(raw) {
					continue
				}
				item[key] = resourceSniffSanitizeLogURL(raw)
			}
		}
		if data, err := json.Marshal(items); err == nil {
			return resourceSniffTruncate(string(data), limit)
		}
	}
	return resourceSniffLogURL(trimmed, limit)
}

func resourceSniffLooksLikeURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "://") || strings.HasPrefix(lower, "blob:")
}

func resourceSniffTruncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func mergePageMeta(base map[string]string, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return base
	}
	if base == nil {
		base = map[string]string{}
	}
	for key, value := range extra {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		base[key] = value
	}
	return base
}

func (service *LibraryService) selectResourceSniffMedia(pageURL string, pageMeta map[string]string, candidates []resourceCandidate, structuredMedia []resourceStructuredMedia, since time.Time) (resourceSniffMediaSelection, bool) {
	pageURL = firstNonEmpty(strings.TrimSpace(pageMeta["location"]), strings.TrimSpace(pageURL))
	pageDomain := extractRegistrableDomain(pageURL)
	extractor := resourceExtractorForURL(pageURL)
	if extractor.Name() == (resourceDefaultSiteRules{}).Name() {
		globalSelection, globalOK := buildResourceSniffMediaSelection(
			service,
			extractor,
			pageURL,
			pageDomain,
			candidates,
			pageMeta,
			nil,
			since,
		)
		if globalOK {
			return globalSelection, true
		}
		return resourceSniffMediaSelection{
			Extractor:  extractor,
			PageURL:    pageURL,
			PageDomain: pageDomain,
			PageMeta:   pageMeta,
		}, false
	}

	if augmenter, ok := extractor.(resourceStructuredMediaAugmenter); ok {
		structuredMedia = augmenter.AugmentStructuredMedia(service, pageURL, pageMeta, structuredMedia)
	}

	enrichedPageMeta := extractor.EnrichPageMeta(pageMeta, structuredMedia)
	enrichedPageURL := firstNonEmpty(strings.TrimSpace(enrichedPageMeta["location"]), pageURL)
	enrichedPageDomain := extractRegistrableDomain(enrichedPageURL)
	extractor = resourceExtractorForURL(enrichedPageURL)
	specialSelection, specialOK := buildResourceSniffMediaSelection(
		service,
		extractor,
		enrichedPageURL,
		enrichedPageDomain,
		candidates,
		enrichedPageMeta,
		structuredMedia,
		since,
	)
	if specialOK {
		specialSelection = enrichResourceSniffSelectionWithStructuredMedia(specialSelection, structuredMedia)
		return specialSelection, true
	}
	return resourceSniffMediaSelection{
		Extractor:  extractor,
		PageURL:    enrichedPageURL,
		PageDomain: enrichedPageDomain,
		PageMeta:   enrichedPageMeta,
	}, false
}

func buildResourceSniffMediaSelection(
	service *LibraryService,
	extractor resourceExtractor,
	pageURL string,
	pageDomain string,
	candidates []resourceCandidate,
	pageMeta map[string]string,
	structuredMedia []resourceStructuredMedia,
	since time.Time,
) (resourceSniffMediaSelection, bool) {
	if extractor == nil {
		extractor = resourceDefaultSiteRules{}
	}
	selection := resourceSniffMediaSelection{
		Extractor:  extractor,
		PageURL:    pageURL,
		PageDomain: pageDomain,
		PageMeta:   pageMeta,
	}
	var mediaOptions []resourceMedia
	if len(structuredMedia) > 0 && extractor.Name() == (resourceDefaultSiteRules{}).Name() {
		mediaOptions = resourceMediaOptionsForPage(service, extractor, pageURL, pageDomain, candidates, pageMeta, structuredMedia, since)
		if len(mediaOptions) > 0 {
			mediaOptions = service.probePrimaryResourceMediaOptionSize(mediaOptions)
			mediaOptions = enrichResourceMediaOptionsWithStructuredMedia(mediaOptions, structuredMedia)
			selection.Media = mediaOptions[0]
			selection.MediaOptions = mediaOptions
			return selection, true
		}
	}
	candidate, ok := extractor.SelectCandidate(candidates, pageMeta, since)
	if !ok {
		return selection, false
	}
	media := extractor.MediaFromCandidate(service, pageURL, pageDomain, candidate, pageMeta)
	mediaOptions = resourceMediaOptionsForPage(service, extractor, pageURL, pageDomain, candidates, pageMeta, structuredMedia, since)
	if len(mediaOptions) == 0 && strings.TrimSpace(media.URL) != "" {
		mediaOptions = []resourceMedia{media}
	}
	if len(mediaOptions) == 0 {
		return selection, false
	}
	mediaOptions = service.probePrimaryResourceMediaOptionSize(mediaOptions)
	mediaOptions = enrichResourceMediaOptionsWithStructuredMedia(mediaOptions, structuredMedia)
	selection.Media = mediaOptions[0]
	selection.MediaOptions = mediaOptions
	return selection, true
}

func enrichResourceSniffSelectionWithStructuredMedia(selection resourceSniffMediaSelection, mediaItems []resourceStructuredMedia) resourceSniffMediaSelection {
	if len(mediaItems) == 0 {
		return selection
	}
	selection.Media = enrichResourceMediaWithStructuredMedia(selection.Media, mediaItems)
	selection.MediaOptions = enrichResourceMediaOptionsWithStructuredMedia(selection.MediaOptions, mediaItems)
	if len(selection.MediaOptions) > 0 {
		selection.Media = selection.MediaOptions[0]
	}
	return selection
}

func enrichResourceMediaOptionsWithStructuredMedia(mediaOptions []resourceMedia, mediaItems []resourceStructuredMedia) []resourceMedia {
	if len(mediaOptions) == 0 || len(mediaItems) == 0 {
		return mediaOptions
	}
	result := append([]resourceMedia(nil), mediaOptions...)
	for index := range result {
		result[index] = enrichResourceMediaWithStructuredMedia(result[index], mediaItems)
	}
	return result
}

func enrichResourceMediaWithStructuredMedia(media resourceMedia, mediaItems []resourceStructuredMedia) resourceMedia {
	structured, ok := resourceStructuredMediaForMediaURL(media.URL, mediaItems)
	if !ok {
		return media
	}
	media.PageURL = firstNonEmpty(structured.PageURL, media.PageURL)
	media.Title = firstNonEmpty(structured.Title, media.Title)
	media.Author = firstNonEmpty(structured.Author, media.Author)
	media.ThumbnailURL = firstNonEmpty(resourceSecureImageURL(structured.ThumbnailURL), media.ThumbnailURL)
	media.FormatNote = firstNonEmpty(structured.FormatNote, media.FormatNote)
	media.VCodec = firstNonEmpty(structured.VCodec, media.VCodec)
	media.ACodec = firstNonEmpty(structured.ACodec, media.ACodec)
	media.Width = firstPositiveInt(structured.Width, media.Width)
	media.Height = firstPositiveInt(structured.Height, media.Height)
	media.QualityHeight = firstPositiveInt(structured.QualityHeight, media.QualityHeight)
	media.SizeBytes = firstPositiveInt64(structured.SizeBytes, media.SizeBytes)
	media.Subtitles = dedupeResourceSubtitles(append(media.Subtitles, structured.Subtitles...))
	media.RequestHeaders = mergeHeaders(media.RequestHeaders, structured.Headers)
	return media
}

func resourceStructuredMediaForMediaURL(mediaURL string, mediaItems []resourceStructuredMedia) (resourceStructuredMedia, bool) {
	mediaURL = strings.TrimSpace(mediaURL)
	if mediaURL == "" || len(mediaItems) == 0 {
		return resourceStructuredMedia{}, false
	}
	best := resourceStructuredMedia{}
	bestScore := 0
	for _, item := range mediaItems {
		score := resourceVideoSourceMatchScore(item.VideoURL, []string{mediaURL})
		if score <= 0 {
			continue
		}
		if bestScore == 0 || score > bestScore || (score == bestScore && resourceStructuredMediaBetter(item, best)) {
			best = item
			bestScore = score
		}
	}
	return best, bestScore > 0
}

func (service *LibraryService) probePrimaryResourceMediaOptionSize(mediaOptions []resourceMedia) []resourceMedia {
	if service == nil || len(mediaOptions) == 0 {
		return mediaOptions
	}
	primary := mediaOptions[0]
	if primary.SizeBytes > 0 || strings.TrimSpace(primary.URL) == "" {
		return mediaOptions
	}
	if resourceSniffRawManifestStream(primary.URL, primary.MimeType, primary.ContentType) {
		return mediaOptions
	}
	ctx, cancel := context.WithTimeout(context.Background(), resourceMediaProbeTimeout)
	defer cancel()
	probe, err := probeResourceDownload(ctx, service.ytdlpAuxiliaryHTTPClient(), primary.URL, primary.RequestHeaders, primary.SizeBytes)
	if err != nil || probe.TotalSize <= 0 {
		return mediaOptions
	}
	result := append([]resourceMedia(nil), mediaOptions...)
	result[0].SizeBytes = probe.TotalSize
	return result
}

func resourceEvalAwaitPromise(params *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
	return params.WithAwaitPromise(true)
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

func resourceSniffLaunchFingerprint(preferredBrowser string, profilePath string, proxyURL string, persistentProfile bool) string {
	parts := []string{
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
	current.AuthStatus = ""
	current.AuthUser = ""
	current.AuthSite = ""
	current.AuthURL = ""
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
		zap.String("url", resourceSniffLogURL(resolvedURL, 240)),
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
	media, ok := service.resourceMediaForQueuedOperation(request)
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
	if service.resourceMedia == nil {
		service.resourceMedia = make(map[string]resourceMedia)
	}
	service.resourceMedia[mediaID] = media
	return mediaID
}

func (service *LibraryService) peekResourceMediaSnapshot(mediaID string) (resourceMedia, bool) {
	if service == nil {
		return resourceMedia{}, false
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	media, ok := service.resourceMedia[strings.TrimSpace(mediaID)]
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
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	for _, mediaID := range mediaIDs {
		delete(service.resourceMedia, strings.TrimSpace(mediaID))
	}
}

func (service *LibraryService) consumeResourceMediaSnapshot(mediaID string) (resourceMedia, bool) {
	if service == nil {
		return resourceMedia{}, false
	}
	service.resourceSniffMu.Lock()
	defer service.resourceSniffMu.Unlock()
	mediaID = strings.TrimSpace(mediaID)
	media, ok := service.resourceMedia[mediaID]
	delete(service.resourceMedia, mediaID)
	return media, ok
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
	authStatus := strings.TrimSpace(session.AuthStatus)
	authUser := strings.TrimSpace(session.AuthUser)
	authSite := strings.TrimSpace(session.AuthSite)
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
	if authSite == "" || extractRegistrableDomain(session.AuthURL) != extractRegistrableDomain(currentURL) {
		authStatus = ""
		authUser = ""
		authSite = ""
	}
	return dto.ResourceSniffSession{
		SessionID:         session.ID,
		State:             state,
		BrowserStatus:     browserStatus,
		URL:               session.URL,
		CurrentURL:        currentURL,
		Title:             title,
		ActiveTargetID:    activeID,
		TabCount:          tabCount,
		UnoptimizedDomain: resourceSniffUnoptimizedDomain(currentURL),
		AuthStatus:        authStatus,
		AuthUser:          authUser,
		AuthSite:          authSite,
	}
}

func resourceSniffUnoptimizedDomain(rawURL string) string {
	domain := extractRegistrableDomain(rawURL)
	if domain == "" {
		return ""
	}
	if resourceKnownExtractor(domain) {
		return ""
	}
	return domain
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
			if event.Request != nil {
				tab.Capture.recordRequest(event.RequestID, event.Request.URL, event.DocumentURL, event.Request.Headers)
				service.touchResourceSniffTab(sessionID, tab.TargetID, event.Request.URL)
			}
		case *network.EventRequestWillBeSentExtraInfo:
			tab.Capture.recordRequestHeaders(event.RequestID, event.Headers)
		case *network.EventResponseReceived:
			if event.Response != nil {
				tab.Capture.recordResponse(event.RequestID, event.Response.URL, event.Response.Status, event.Response.MimeType, event.Response.Headers, event.Type)
				service.touchResourceSniffTab(sessionID, tab.TargetID, event.Response.URL)
			}
		case *network.EventLoadingFinished:
			if tab.Capture.shouldCaptureResponseBody(event.RequestID) {
				go captureResourceSniffResponseBody(tab.Ctx, tab.Capture, event.RequestID)
			}
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
	if !ok || session.Runtime == nil {
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
			if strings.TrimSpace(event.TargetID) != "" {
				if current, ok := service.getResourceSniffSession(sessionID); ok && current.Runtime != nil {
					if manager := current.Runtime.TargetManager(); manager != nil && manager.PageTargetExists(event.TargetID) {
						return
					}
				}
				service.removeResourceSniffTab(sessionID, event.TargetID, string(event.Kind))
				return
			}
			service.removeResourceSniffTabBySessionID(sessionID, event.SessionID, string(event.Kind))
		}
	})
	if err != nil {
		zap.L().Warn("resource sniff target discovery failed", zap.String("sessionID", sessionID), zap.Error(err))
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
			tab.CurrentURL = firstNonEmpty(strings.TrimSpace(pageURL), tab.CurrentURL)
			tab.Title = firstNonEmpty(resourceCleanMetadataText(title), tab.Title)
			if !resourceSniffIgnoredTargetURL(pageURL) {
				tab.PendingNavigation = false
			}
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

	tabCtx, tabCancel := chromedp.NewContext(session.Runtime.BrowserContext(), chromedp.WithTargetID(targetpkg.ID(targetID)))
	if err := chromedp.Run(tabCtx, resourceSniffNetworkEnable()); err != nil {
		service.clearResourceSniffTargetAttaching(sessionID, targetID)
		return
	}
	pendingNavigation := resourceSniffIgnoredTargetURL(pageURL) && resourceSniffTrackPendingTargetURL(pageURL)
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
			existing.CurrentURL = firstNonEmpty(strings.TrimSpace(pageURL), existing.CurrentURL)
			existing.Title = firstNonEmpty(resourceCleanMetadataText(title), existing.Title)
			if !resourceSniffIgnoredTargetURL(pageURL) {
				existing.PendingNavigation = false
			}
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
		tab.TargetSessionID = targetSessionID
	}
}

func (service *LibraryService) syncResourceSniffTargets(sessionID string) {
	session, ok := service.getResourceSniffSession(sessionID)
	if !ok || session.Runtime == nil || !session.Runtime.Status().Ready {
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
	lastAuthProbe := time.Time{}
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
		if lastAuthProbe.IsZero() || time.Since(lastAuthProbe) >= resourceSniffAuthProbe {
			service.probeResourceSniffSessionAuth(sessionID, resourceSniffAuthProbeTimeout)
			lastAuthProbe = time.Now()
		}
		<-ticker.C
	}
}

func (service *LibraryService) probeResourceSniffSessionAuth(sessionID string, timeout time.Duration) {
	tab := service.resolveResourceSniffActiveTab(sessionID)
	if tab == nil || tab.Ctx == nil {
		return
	}
	pageURL := strings.TrimSpace(tab.CurrentURL)
	if pageURL == "" || resourceSniffIgnoredTargetURL(pageURL) {
		return
	}
	if timeout <= 0 {
		timeout = resourceSniffAuthProbeTimeout
	}
	probeCtx, cancel := context.WithTimeout(tab.Ctx, timeout)
	info := resourceSniffAuthInfoForPage(probeCtx, pageURL)
	cancel()
	if strings.TrimSpace(info.Site) == "" {
		service.updateResourceSniffSession(sessionID, func(current *resourceSniffSession) {
			current.AuthStatus = ""
			current.AuthUser = ""
			current.AuthSite = ""
			current.AuthURL = pageURL
		})
		return
	}
	service.updateResourceSniffSession(sessionID, func(current *resourceSniffSession) {
		if strings.TrimSpace(current.ActiveID) != strings.TrimSpace(tab.TargetID) {
			return
		}
		current.AuthStatus = info.Status
		current.AuthUser = info.User
		current.AuthSite = info.Site
		current.AuthURL = pageURL
	})
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
		meta     map[string]any
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
			pageMeta := map[string]any{}
			probeCtx, cancel := context.WithTimeout(tab.Ctx, timeout)
			err := chromedp.Run(probeCtx, chromedp.Evaluate(resourcePageIdentityScript(), &pageMeta))
			cancel()
			if err != nil {
				return
			}
			results <- identityResult{targetID: tab.TargetID, meta: pageMeta}
		}(tab)
	}
	wg.Wait()
	close(results)
	for result := range results {
		service.updateResourceSniffTabIdentity(sessionID, result.targetID, result.meta)
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

func (service *LibraryService) updateResourceSniffTabIdentity(sessionID string, targetID string, pageMeta map[string]any) {
	if service == nil || len(pageMeta) == 0 {
		return
	}
	location := strings.TrimSpace(fmtString(pageMeta["location"]))
	title := resourceCleanMetadataText(fmtString(pageMeta["title"]))
	visibility := strings.TrimSpace(fmtString(pageMeta["visibilityState"]))
	hasFocus := fmtBool(pageMeta["hasFocus"])
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
	if !resourceSniffIgnoredTargetURL(location) {
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
	service.resourceSniffMu.Lock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || len(session.Tabs) == 0 {
		service.resourceSniffMu.Unlock()
		return
	}
	targetID = strings.TrimSpace(targetID)
	tab := session.Tabs[targetID]
	if tab == nil {
		service.resourceSniffMu.Unlock()
		return
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
		zap.String("url", resourceSniffLogURL(removedURL, 240)),
		zap.String("title", removedTitle),
	)
	if tab != nil && tab.Cancel != nil {
		tab.Cancel()
	}
}

func (service *LibraryService) removeResourceSniffTabBySessionID(sessionID string, targetSessionID string, reason string) {
	targetSessionID = strings.TrimSpace(targetSessionID)
	if service == nil || targetSessionID == "" {
		return
	}
	service.resourceSniffMu.Lock()
	session := service.resourceSniffs[strings.TrimSpace(sessionID)]
	if session == nil || len(session.Tabs) == 0 {
		service.resourceSniffMu.Unlock()
		return
	}
	targetID := ""
	for currentTargetID, tab := range session.Tabs {
		if tab != nil && strings.TrimSpace(tab.TargetSessionID) == targetSessionID {
			targetID = currentTargetID
			break
		}
	}
	service.resourceSniffMu.Unlock()
	if targetID == "" {
		return
	}
	service.removeResourceSniffTab(sessionID, targetID, reason)
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
			zap.String("url", resourceSniffLogURL(item.url, 240)),
			zap.String("title", item.title),
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
					zap.L().Warn(
						"resource sniff context cleanup panicked",
						zap.String("sessionID", sessionID),
						zap.Int("index", index),
						zap.Any("panic", recovered),
					)
				}
			}()
			cancel()
			zap.L().Debug("resource sniff context cleanup finished", zap.String("sessionID", sessionID), zap.Int("index", index))
		}(index, cancel)
	}
}
