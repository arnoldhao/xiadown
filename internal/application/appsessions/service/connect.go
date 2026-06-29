package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/sitepolicy"
	"xiadown/internal/domain/appsessions"
)

const (
	browserSessionStateRunning   = "running"
	browserSessionStateCompleted = "completed"
	browserSessionStateFailed    = "failed"
)

const (
	browserSessionPurposeConnect = "connect"
	browserSessionPurposeOpen    = "open"
)

const (
	browserStatusOpen          = "open"
	browserStatusBrowserClosed = "browser_closed"
	browserStatusCompleted     = "completed"
	browserStatusFailed        = "failed"
)

type browserSession struct {
	ID            string
	AppSessionID  string
	SiteKey       string
	Purpose       string
	Browser       AppSessionBrowser
	TargetURL     string
	State         string
	LastCookies   []appcookies.Record
	LastCookiesAt time.Time
	FinalResult   *dto.FinishAppSessionConnectResult
	FinalError    string
	Snapshot      dto.AppSession
	cleanupOnce   sync.Once
	finalizeOnce  sync.Once
	finalizeDone  chan struct{}
}

type AppSessionStartRequest struct {
	SessionID      string
	AppSessionID   string
	SiteKey        string
	TargetURL      string
	InitialCookies []appcookies.Record
	Domains        []string
}

type AppSessionBrowser interface {
	Cookies(ctx context.Context) ([]appcookies.Record, error)
	Done() <-chan struct{}
	Close()
}

func (service *AppSessionsService) StartAppSessionConnect(ctx context.Context, request dto.StartAppSessionConnectRequest) (dto.StartAppSessionConnectResult, error) {
	session, err := service.sessionByID(ctx, request.ID)
	if err != nil {
		return dto.StartAppSessionConnectResult{}, err
	}
	targetURL, err := targetURLForSite(session.SiteKey, request.TargetURL)
	if err != nil {
		return dto.StartAppSessionConnectResult{}, err
	}
	targetURL = appSessionLoginURL(session.SiteKey, targetURL)
	return service.startBrowserSession(ctx, session, browserSessionPurposeConnect, targetURL)
}

func (service *AppSessionsService) OpenAppSessionSite(ctx context.Context, request dto.OpenAppSessionSiteRequest) (dto.StartAppSessionConnectResult, error) {
	session, err := service.sessionByID(ctx, request.ID)
	if err != nil {
		return dto.StartAppSessionConnectResult{}, err
	}
	targetURL, err := targetURLForSite(session.SiteKey, request.TargetURL)
	if err != nil {
		return dto.StartAppSessionConnectResult{}, err
	}
	return service.startBrowserSession(ctx, session, browserSessionPurposeOpen, targetURL)
}

func (service *AppSessionsService) startBrowserSession(ctx context.Context, appSession appsessions.Session, purpose string, targetURL string) (dto.StartAppSessionConnectResult, error) {
	if service == nil || service.provider == nil || !service.provider.AppSessionsSupported() {
		return dto.StartAppSessionConnectResult{}, appsessions.ErrUnsupported
	}
	sessionID := service.newSessionID()
	if purpose == browserSessionPurposeConnect {
		if err := service.ClearAppSession(ctx, dto.ClearAppSessionRequest{ID: appSession.ID}); err != nil {
			return dto.StartAppSessionConnectResult{}, err
		}
		if current, err := service.repo.Get(ctx, appSession.ID); err == nil {
			appSession = current
		}
	}
	initialCookies := service.storedCookies(ctx, appSession.SiteKey)
	browser, err := service.provider.StartAppSession(ctx, AppSessionStartRequest{
		SessionID:      sessionID,
		AppSessionID:   appSession.ID,
		SiteKey:        appSession.SiteKey,
		TargetURL:      targetURL,
		InitialCookies: initialCookies,
		Domains:        appSessionCookieDomains(appSession.SiteKey),
	})
	if err != nil {
		return dto.StartAppSessionConnectResult{}, err
	}
	runtime := &browserSession{
		ID:           sessionID,
		AppSessionID: appSession.ID,
		SiteKey:      appSession.SiteKey,
		Purpose:      purpose,
		Browser:      browser,
		TargetURL:    targetURL,
		State:        browserSessionStateRunning,
		Snapshot:     service.mapSessionDTOWithCookies(appSession, initialCookies),
		finalizeDone: make(chan struct{}),
	}
	replaced := service.putSession(runtime)
	service.cleanupSession(replaced)
	service.startBrowserMonitor(sessionID)
	return dto.StartAppSessionConnectResult{
		SessionID:  sessionID,
		AppSession: service.mapSessionDTOWithCookies(appSession, initialCookies),
		TargetURL:  targetURL,
	}, nil
}

func (service *AppSessionsService) FinishAppSessionConnect(ctx context.Context, request dto.FinishAppSessionConnectRequest) (dto.FinishAppSessionConnectResult, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return dto.FinishAppSessionConnectResult{}, appsessions.ErrSessionGone
	}
	if session, ok := service.getSession(sessionID); ok && session != nil && session.Purpose == browserSessionPurposeOpen {
		return dto.FinishAppSessionConnectResult{}, appsessions.ErrSessionGone
	}
	result, _, err := service.finalizeSession(ctx, sessionID, "manual_finish")
	if err != nil {
		return dto.FinishAppSessionConnectResult{}, err
	}
	return result, nil
}

func (service *AppSessionsService) CancelAppSessionConnect(_ context.Context, request dto.CancelAppSessionConnectRequest) error {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return appsessions.ErrSessionGone
	}
	service.cleanupSession(service.popSession(sessionID))
	return nil
}

func (service *AppSessionsService) GetAppSessionConnectSession(ctx context.Context, request dto.GetAppSessionConnectSessionRequest) (dto.AppSessionConnectSession, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return dto.AppSessionConnectSession{}, appsessions.ErrSessionGone
	}
	session, ok := service.getSession(sessionID)
	if !ok || session == nil {
		return dto.AppSessionConnectSession{}, appsessions.ErrSessionGone
	}
	return service.snapshotSession(ctx, session), nil
}

func (service *AppSessionsService) finalizeSession(ctx context.Context, sessionID string, reason string) (dto.FinishAppSessionConnectResult, bool, error) {
	session, ok := service.getSession(sessionID)
	if !ok || session == nil {
		return dto.FinishAppSessionConnectResult{}, false, appsessions.ErrSessionGone
	}
	triggered := false
	var completedResult dto.FinishAppSessionConnectResult
	completed := false
	session.finalizeOnce.Do(func() {
		triggered = true
		result, err := service.performFinalize(ctx, session, reason)
		service.mu.Lock()
		defer service.mu.Unlock()
		if err != nil {
			session.State = browserSessionStateFailed
			session.FinalError = err.Error()
		} else {
			session.State = browserSessionStateCompleted
			session.FinalError = ""
			session.FinalResult = &result
			session.Snapshot = result.AppSession
			completedResult = result
			completed = true
		}
		close(session.finalizeDone)
	})
	<-session.finalizeDone
	if triggered && completed {
		service.notifyAppSessionChanged(ctx, AppSessionChangeEvent{
			Action:     appSessionFinalizeAction(session.Purpose),
			AppSession: completedResult.AppSession,
			Saved:      completedResult.Saved,
			Reason:     completedResult.Reason,
		})
	}

	session, ok = service.getSession(sessionID)
	if !ok || session == nil {
		return dto.FinishAppSessionConnectResult{}, triggered, appsessions.ErrSessionGone
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if session.FinalError != "" {
		return dto.FinishAppSessionConnectResult{}, triggered, errors.New(session.FinalError)
	}
	if session.FinalResult == nil {
		return dto.FinishAppSessionConnectResult{}, triggered, appsessions.ErrSessionDead
	}
	return *session.FinalResult, triggered, nil
}

func appSessionFinalizeAction(purpose string) string {
	if strings.TrimSpace(purpose) == browserSessionPurposeOpen {
		return "verify-started"
	}
	return "finish"
}

func (service *AppSessionsService) performFinalize(ctx context.Context, session *browserSession, reason string) (dto.FinishAppSessionConnectResult, error) {
	if session == nil {
		return dto.FinishAppSessionConnectResult{}, appsessions.ErrSessionGone
	}
	records := append([]appcookies.Record(nil), session.LastCookies...)
	if session.Browser != nil {
		readCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		liveRecords, err := session.Browser.Cookies(readCtx)
		cancel()
		if err == nil && len(liveRecords) > 0 {
			records = liveRecords
		}
	}
	filtered := filterAppSessionCookies(session.SiteKey, records)
	hasAuthCookies := appSessionHasAuthenticationCookies(session.SiteKey, filtered)
	current, err := service.repo.Get(ctx, session.AppSessionID)
	if err != nil {
		current, err = service.EnsureAppSession(ctx, session.SiteKey)
		if err != nil {
			service.cleanupSession(session)
			return dto.FinishAppSessionConnectResult{}, err
		}
	}
	result := dto.FinishAppSessionConnectResult{
		SessionID:            session.ID,
		Saved:                hasAuthCookies,
		RawCookiesCount:      len(records),
		FilteredCookiesCount: len(filtered),
		Domains:              cookieDomains(filtered),
		Reason:               reason,
		AppSession:           service.mapSessionDTOWithCookies(current, filtered),
	}
	if !hasAuthCookies {
		service.cleanupSession(session)
		return result, nil
	}
	if service.provider != nil {
		if err := service.provider.SaveAppSessionCookies(ctx, session.SiteKey, filtered); err != nil {
			service.cleanupSession(session)
			return dto.FinishAppSessionConnectResult{}, err
		}
	}
	now := service.now().UTC().Round(0)
	verificationStatus := appsessions.AccountVerificationUnsupported
	verificationError := ""
	var verificationStartedAt *time.Time
	if appSessionRequiresAccountVerification(session.SiteKey) {
		if service.accountFetcher != nil {
			startedAt := now
			verificationStatus = appsessions.AccountVerificationVerifying
			verificationStartedAt = &startedAt
		} else {
			verificationStatus = appsessions.AccountVerificationUnverified
			verificationError = appSessionVerificationErrorMessage(appsessions.ErrUnsupported)
		}
	}
	updated, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                           current.ID,
		SiteKey:                      current.SiteKey,
		Status:                       string(appsessions.StatusConnected),
		AccountVerificationStatus:    string(verificationStatus),
		AccountVerificationError:     verificationError,
		AccountVerificationStartedAt: verificationStartedAt,
		CreatedAt:                    &current.CreatedAt,
		UpdatedAt:                    &now,
	})
	if err != nil {
		service.cleanupSession(session)
		return dto.FinishAppSessionConnectResult{}, err
	}
	if err := service.repo.Save(ctx, updated); err != nil {
		service.cleanupSession(session)
		return dto.FinishAppSessionConnectResult{}, err
	}
	result.AppSession = service.mapSessionDTOWithCookies(updated, filtered)
	service.cleanupSession(session)
	if verificationStatus == appsessions.AccountVerificationVerifying && verificationStartedAt != nil {
		service.startAppSessionAccountVerification(updated, filtered, *verificationStartedAt)
	}
	return result, nil
}

func appSessionRequiresAccountVerification(siteKey string) bool {
	switch strings.TrimSpace(siteKey) {
	case "youtube", "bilibili", "tiktok", "instagram", "x", "facebook", "vimeo", "twitch", "niconico":
		return true
	default:
		return false
	}
}

func appSessionAccountVerified(account dto.AppSessionAccount) bool {
	return strings.TrimSpace(account.DisplayName) != "" ||
		strings.TrimSpace(account.Handle) != "" ||
		strings.TrimSpace(account.AvatarURL) != ""
}

func targetURLForSite(siteKey string, requestedURL string) (string, error) {
	targetURL := strings.TrimSpace(requestedURL)
	if targetURL == "" {
		targetURL = appSessionHomeURL(siteKey)
	}
	if targetURL == "" {
		return "", appsessions.ErrInvalidSession
	}
	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", appsessions.ErrInvalidSession
	}
	policy, ok := sitepolicy.ForSiteKey(siteKey)
	if !ok || !sitepolicy.MatchDomains(targetURL, policy.Domains) {
		return "", appsessions.ErrInvalidSession
	}
	return targetURL, nil
}
