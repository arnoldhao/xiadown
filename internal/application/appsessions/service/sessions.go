package service

import (
	"context"
	"strings"
	"time"

	"xiadown/internal/application/appsessions/dto"
	"xiadown/internal/domain/appsessions"
)

func (service *AppSessionsService) sessionByID(ctx context.Context, id string) (appsessions.Session, error) {
	if service == nil || service.repo == nil {
		return appsessions.Session{}, appsessions.ErrSessionNotFound
	}
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return appsessions.Session{}, appsessions.ErrInvalidSession
	}
	session, err := service.repo.Get(ctx, trimmed)
	if err == nil {
		return session, nil
	}
	if strings.HasPrefix(trimmed, "site-app-session-") {
		siteKey := strings.TrimPrefix(trimmed, "site-app-session-")
		if isSupportedSiteKey(siteKey) {
			return service.EnsureAppSession(ctx, siteKey)
		}
	}
	return appsessions.Session{}, err
}

func (service *AppSessionsService) putSession(session *browserSession) *browserSession {
	if service == nil || session == nil {
		return nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	var replaced *browserSession
	if currentID, ok := service.sessionsByApp[session.AppSessionID]; ok && currentID != session.ID {
		replaced = service.sessions[currentID]
		delete(service.sessions, currentID)
	}
	service.sessions[session.ID] = session
	service.sessionsByApp[session.AppSessionID] = session.ID
	return replaced
}

func (service *AppSessionsService) getSession(sessionID string) (*browserSession, bool) {
	if service == nil {
		return nil, false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, ok := service.sessions[sessionID]
	return session, ok
}

func (service *AppSessionsService) updateSession(sessionID string, update func(session *browserSession)) (*browserSession, bool) {
	if service == nil {
		return nil, false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, ok := service.sessions[sessionID]
	if !ok || session == nil {
		return nil, false
	}
	update(session)
	return session, true
}

func (service *AppSessionsService) popSession(sessionID string) *browserSession {
	if service == nil {
		return nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session := service.sessions[sessionID]
	if session == nil {
		return nil
	}
	delete(service.sessions, sessionID)
	if currentID, ok := service.sessionsByApp[session.AppSessionID]; ok && currentID == sessionID {
		delete(service.sessionsByApp, session.AppSessionID)
	}
	return session
}

func (service *AppSessionsService) cleanupSession(session *browserSession) {
	if session == nil {
		return
	}
	session.cleanupOnce.Do(func() {
		if session.Browser != nil {
			session.Browser.Close()
		}
	})
}

func (service *AppSessionsService) startBrowserMonitor(sessionID string) {
	session, ok := service.getSession(sessionID)
	if !ok || session == nil || session.Browser == nil {
		return
	}
	go func(browser AppSessionBrowser) {
		for {
			session, ok := service.getSession(sessionID)
			if !ok || session == nil {
				return
			}
			service.mu.Lock()
			state := session.State
			service.mu.Unlock()
			if state != browserSessionStateRunning {
				return
			}
			<-browser.Done()
			if session.Purpose == browserSessionPurposeConnect || session.Purpose == browserSessionPurposeOpen {
				service.triggerSessionFinalize(sessionID, "browser_closed")
			} else {
				service.updateSession(sessionID, func(current *browserSession) {
					current.State = browserSessionStateCompleted
				})
				service.cleanupSession(session)
			}
			return
		}
	}(session.Browser)
}

func (service *AppSessionsService) triggerSessionFinalize(sessionID string, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if _, _, err := service.finalizeSession(ctx, sessionID, reason); err != nil &&
		!errorsIsSessionGone(err) {
		service.updateSession(sessionID, func(current *browserSession) {
			current.State = browserSessionStateFailed
			current.FinalError = err.Error()
		})
	}
}

func (service *AppSessionsService) notifyAppSessionChanged(ctx context.Context, event AppSessionChangeEvent) {
	if service == nil || service.changeListener == nil {
		return
	}
	service.changeListener(ctx, event)
}

func errorsIsSessionGone(err error) bool {
	return err == appsessions.ErrSessionGone || err == appsessions.ErrSessionNotFound
}

func (service *AppSessionsService) snapshotSession(ctx context.Context, session *browserSession) dto.AppSessionConnectSession {
	currentCookies := filterAppSessionCookies(session.SiteKey, session.LastCookies)
	saved := false
	rawCount := 0
	filteredCount := 0
	domains := []string(nil)
	reason := ""
	appSession := session.Snapshot
	if session.FinalResult != nil {
		saved = session.FinalResult.Saved
		rawCount = session.FinalResult.RawCookiesCount
		filteredCount = session.FinalResult.FilteredCookiesCount
		domains = append([]string(nil), session.FinalResult.Domains...)
		reason = session.FinalResult.Reason
		appSession = session.FinalResult.AppSession
	} else {
		rawCount = len(session.LastCookies)
		filteredCount = len(currentCookies)
		domains = cookieDomains(currentCookies)
		if current, err := service.repo.Get(ctx, session.AppSessionID); err == nil {
			appSession = service.mapSessionDTOWithCookies(current, currentCookies)
		}
	}
	lastCookiesAt := ""
	if !session.LastCookiesAt.IsZero() {
		lastCookiesAt = session.LastCookiesAt.Format(time.RFC3339)
	}
	return dto.AppSessionConnectSession{
		SessionID:            session.ID,
		AppSessionID:         session.AppSessionID,
		State:                session.State,
		BrowserStatus:        browserStatusForSession(session),
		TargetURL:            session.TargetURL,
		CurrentCookiesCount:  len(currentCookies),
		Saved:                saved,
		RawCookiesCount:      rawCount,
		FilteredCookiesCount: filteredCount,
		Domains:              domains,
		Reason:               reason,
		Error:                session.FinalError,
		LastCookiesAt:        lastCookiesAt,
		AppSession:           appSession,
	}
}

func browserStatusForSession(session *browserSession) string {
	if session == nil {
		return browserStatusBrowserClosed
	}
	switch session.State {
	case browserSessionStateCompleted:
		return browserStatusCompleted
	case browserSessionStateFailed:
		return browserStatusFailed
	default:
		return browserStatusOpen
	}
}
