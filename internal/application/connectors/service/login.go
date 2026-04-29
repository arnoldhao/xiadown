package service

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cdptarget "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/connectors/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/sitepolicy"
	"xiadown/internal/domain/connectors"
)

func (service *ConnectorsService) StartConnectorConnect(ctx context.Context, request dto.StartConnectorConnectRequest) (dto.StartConnectorConnectResult, error) {
	id := strings.TrimSpace(request.ID)
	if id == "" {
		return dto.StartConnectorConnectResult{}, connectors.ErrInvalidConnector
	}
	connector, err := service.repo.Get(ctx, id)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	connector, err = service.clearConnectorCookiesBeforeReconnect(ctx, connector)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	targetURL, err := connectorHomeURL(connector.Type)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}

	sessionID := service.newSessionID()
	userDataDir := connectorSessionDir(connector.Type, sessionID)
	runtime, tabCtx, cancel, err := service.startBrowser(service.preferredBrowser(ctx), false, userDataDir)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	if err := chromedp.Run(tabCtx, chromedp.Navigate(targetURL)); err != nil {
		cancel()
		runtime.Stop()
		if service.removeAll != nil {
			_ = service.removeAll(userDataDir)
		}
		return dto.StartConnectorConnectResult{}, err
	}

	session := &connectorSession{
		ID:                sessionID,
		ConnectorID:       connector.ID,
		ConnectorType:     connector.Type,
		Runtime:           runtime,
		TabCtx:            tabCtx,
		Cancel:            cancel,
		UserDataDir:       userDataDir,
		State:             connectorSessionStateRunning,
		ConnectorSnapshot: mapConnectorDTO(connector),
		finalizeDone:      make(chan struct{}),
	}
	session.TargetID = connectorTargetIDFromContext(tabCtx)

	replaced := service.putSession(session)
	service.cleanupSession(replaced)
	service.startConnectSessionMonitor(sessionID)
	log.Printf("connectors: started connect session id=%s connector=%s target=%s userDataDir=%s", sessionID, connector.Type, session.TargetID, userDataDir)

	return dto.StartConnectorConnectResult{
		SessionID: sessionID,
		Connector: mapConnectorDTO(connector),
	}, nil
}

func (service *ConnectorsService) FinishConnectorConnect(ctx context.Context, request dto.FinishConnectorConnectRequest) (dto.FinishConnectorConnectResult, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return dto.FinishConnectorConnectResult{}, connectors.ErrConnectorSessionGone
	}
	result, _, err := service.finalizeConnectSession(ctx, sessionID, "manual_finish")
	if err != nil {
		return dto.FinishConnectorConnectResult{}, err
	}
	return result, nil
}

func (service *ConnectorsService) CancelConnectorConnect(ctx context.Context, request dto.CancelConnectorConnectRequest) error {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return connectors.ErrConnectorSessionGone
	}
	log.Printf("connectors: canceled connect session id=%s", sessionID)
	service.cleanupSession(service.popSession(sessionID))
	return nil
}

func (service *ConnectorsService) finalizeConnectSession(ctx context.Context, sessionID string, reason string) (dto.FinishConnectorConnectResult, bool, error) {
	session, ok := service.getSession(sessionID)
	if !ok || session == nil {
		return dto.FinishConnectorConnectResult{}, false, connectors.ErrConnectorSessionGone
	}
	triggered := false
	session.finalizeOnce.Do(func() {
		triggered = true
		result, err := service.performFinalize(ctx, session, reason)
		service.mu.Lock()
		defer service.mu.Unlock()
		if err != nil {
			session.State = connectorSessionStateFailed
			session.FinalError = err.Error()
		} else {
			session.State = connectorSessionStateCompleted
			session.FinalError = ""
			session.FinalResult = &result
			session.ConnectorSnapshot = result.Connector
		}
		close(session.finalizeDone)
	})
	<-session.finalizeDone

	session, ok = service.getSession(sessionID)
	if !ok || session == nil {
		return dto.FinishConnectorConnectResult{}, triggered, connectors.ErrConnectorSessionGone
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if session.FinalError != "" {
		return dto.FinishConnectorConnectResult{}, triggered, errors.New(session.FinalError)
	}
	if session.FinalResult == nil {
		return dto.FinishConnectorConnectResult{}, triggered, connectors.ErrConnectorSessionDead
	}
	return *session.FinalResult, triggered, nil
}

func (service *ConnectorsService) performFinalize(ctx context.Context, session *connectorSession, reason string) (dto.FinishConnectorConnectResult, error) {
	if session == nil {
		return dto.FinishConnectorConnectResult{}, connectors.ErrConnectorSessionGone
	}

	log.Printf("connectors: finalize requested session=%s connector=%s reason=%s", session.ID, session.ConnectorType, reason)
	records, err := readConnectorCookiesFromSession(session)
	service.mu.Lock()
	cachedRecords := append([]appcookies.Record(nil), session.LastCookies...)
	service.mu.Unlock()
	if err != nil {
		log.Printf("connectors: live cookie read failed session=%s connector=%s reason=%s err=%v", session.ID, session.ConnectorType, reason, err)
		records = cachedRecords
	} else if len(records) == 0 && len(cachedRecords) > 0 {
		records = cachedRecords
	} else {
		service.updateSession(session.ID, func(current *connectorSession) {
			current.LastCookies = append([]appcookies.Record(nil), records...)
			current.LastCookiesAt = service.now()
		})
	}

	policy, _ := sitepolicy.ForConnectorType(string(session.ConnectorType))
	filtered := appcookies.FilterByDomains(records, policy.Domains)
	log.Printf("connectors: finalize cookies session=%s connector=%s reason=%s raw=%d filtered=%d domains=%s", session.ID, session.ConnectorType, reason, len(records), len(filtered), strings.Join(cookieDomains(filtered), ","))

	current, err := service.repo.Get(ctx, session.ConnectorID)
	if err != nil {
		service.cleanupSession(session)
		return dto.FinishConnectorConnectResult{}, err
	}

	result := dto.FinishConnectorConnectResult{
		SessionID:            session.ID,
		Saved:                len(filtered) > 0,
		RawCookiesCount:      len(records),
		FilteredCookiesCount: len(filtered),
		Domains:              cookieDomains(filtered),
		Reason:               reason,
		Connector:            mapConnectorDTO(current),
	}
	if len(filtered) == 0 {
		service.cleanupSession(session)
		log.Printf("connectors: finalize completed without matching cookies session=%s connector=%s reason=%s", session.ID, session.ConnectorType, reason)
		return result, nil
	}

	cookiesJSON, err := encodeCookies(filtered)
	if err != nil {
		service.cleanupSession(session)
		return dto.FinishConnectorConnectResult{}, err
	}
	now := service.now()
	updated, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             current.ID,
		Type:           string(current.Type),
		Status:         string(connectors.StatusConnected),
		CookiesJSON:    cookiesJSON,
		LastVerifiedAt: &now,
		CreatedAt:      &current.CreatedAt,
		UpdatedAt:      &now,
	})
	if err != nil {
		service.cleanupSession(session)
		return dto.FinishConnectorConnectResult{}, err
	}
	if err := service.repo.Save(ctx, updated); err != nil {
		service.cleanupSession(session)
		return dto.FinishConnectorConnectResult{}, err
	}
	result.Connector = mapConnectorDTO(updated)
	service.cleanupSession(session)
	log.Printf("connectors: finalize saved cookies session=%s connector=%s reason=%s filtered=%d", session.ID, session.ConnectorType, reason, len(filtered))
	return result, nil
}

func (service *ConnectorsService) startConnectSessionMonitor(sessionID string) {
	session, ok := service.getSession(sessionID)
	if !ok || session == nil {
		return
	}
	service.watchConnectSessionTarget(sessionID, session)
	service.watchConnectSessionBrowser(sessionID, session)

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			session, ok := service.getSession(sessionID)
			if !ok || session == nil {
				return
			}
			service.mu.Lock()
			state := session.State
			runtime := session.Runtime
			targetID := session.TargetID
			tabCtx := session.TabCtx
			service.mu.Unlock()
			if state != connectorSessionStateRunning {
				return
			}
			if runtime == nil || !runtime.Status().Ready {
				service.triggerSessionFinalizeWithCookieSnapshot(sessionID, "browser_closed")
				return
			}
			service.cacheSessionCookies(sessionID, tabCtx)
			if targetID != "" {
				exists, err := connectorTargetExists(runtime, targetID)
				if err == nil && !exists {
					service.triggerSessionFinalizeWithCookieSnapshot(sessionID, "tab_closed")
					return
				}
			}
			var browserDone <-chan struct{}
			if browserCtx := runtime.BrowserContext(); browserCtx != nil {
				browserDone = browserCtx.Done()
			}

			select {
			case <-ticker.C:
			case <-browserDone:
				service.triggerSessionFinalizeWithCookieSnapshot(sessionID, "browser_closed")
				return
			}
		}
	}()
}

func (service *ConnectorsService) watchConnectSessionTarget(sessionID string, session *connectorSession) {
	if session == nil || session.TabCtx == nil {
		return
	}
	targetID := session.TargetID
	if targetID == "" {
		return
	}
	chromedp.ListenTarget(session.TabCtx, func(ev any) {
		switch current := ev.(type) {
		case *cdptarget.EventTargetDestroyed:
			if targetID != "" && current.TargetID != targetID {
				return
			}
			service.triggerSessionFinalizeWithCookieSnapshot(sessionID, "tab_closed")
		case *cdptarget.EventTargetCrashed:
			if targetID != "" && current.TargetID != targetID {
				return
			}
			service.triggerSessionFinalizeWithCookieSnapshot(sessionID, "tab_closed")
		}
	})
}

func (service *ConnectorsService) clearConnectorCookiesBeforeReconnect(ctx context.Context, connector connectors.Connector) (connectors.Connector, error) {
	if strings.TrimSpace(connector.CookiesJSON) == "" && connector.Status != connectors.StatusConnected && connector.Status != connectors.StatusExpired {
		return connector, nil
	}
	now := service.now()
	updated, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:          connector.ID,
		Type:        string(connector.Type),
		Status:      string(connectors.StatusDisconnected),
		CookiesPath: connector.CookiesPath,
		CookiesJSON: "",
		CreatedAt:   &connector.CreatedAt,
		UpdatedAt:   &now,
	})
	if err != nil {
		return connectors.Connector{}, err
	}
	if err := service.repo.Save(ctx, updated); err != nil {
		return connectors.Connector{}, err
	}
	return updated, nil
}

func (service *ConnectorsService) watchConnectSessionBrowser(sessionID string, session *connectorSession) {
	if session == nil || session.Runtime == nil || session.Runtime.BrowserContext() == nil {
		return
	}
	go func(browserCtx context.Context) {
		<-browserCtx.Done()
		service.triggerSessionFinalizeWithCookieSnapshot(sessionID, "browser_closed")
	}(session.Runtime.BrowserContext())
}

func (service *ConnectorsService) triggerSessionFinalizeWithCookieSnapshot(sessionID string, reason string) {
	go func() {
		if session, ok := service.getSession(sessionID); ok && session != nil {
			service.cacheSessionCookies(sessionID, session.TabCtx)
		}
		_, _, err := service.finalizeConnectSession(context.Background(), sessionID, reason)
		if err != nil && !errors.Is(err, connectors.ErrConnectorSessionGone) {
			log.Printf("connectors: auto-finalize failed session=%s reason=%s err=%v", sessionID, reason, err)
		}
	}()
}

func (service *ConnectorsService) cacheSessionCookies(sessionID string, tabCtx context.Context) bool {
	cookies, err := readConnectorCookiesWithTimeout(tabCtx, 2*time.Second)
	if err != nil {
		return false
	}
	service.updateSession(sessionID, func(current *connectorSession) {
		current.LastCookies = append([]appcookies.Record(nil), cookies...)
		current.LastCookiesAt = service.now()
	})
	return true
}

func connectorTargetExists(runtime *browsercdp.Runtime, targetID cdptarget.ID) (bool, error) {
	return connectorTargetExistsWithTimeout(runtime, targetID, 3*time.Second)
}

func connectorTargetExistsWithTimeout(runtime *browsercdp.Runtime, targetID cdptarget.ID, timeout time.Duration) (bool, error) {
	if runtime == nil || targetID == "" {
		return true, nil
	}
	execCtx, cancel, err := browsercdp.RuntimeBrowserExecutorContext(runtime, timeout)
	if err != nil {
		return false, err
	}
	defer cancel()

	var exists bool
	targets, err := cdptarget.GetTargets().Do(execCtx)
	if err != nil {
		return false, err
	}
	for _, info := range targets {
		if info != nil && info.TargetID == targetID {
			exists = true
			break
		}
	}
	return exists, nil
}

func connectorHomeURL(connectorType connectors.ConnectorType) (string, error) {
	switch connectorType {
	case connectors.ConnectorYouTube:
		return "https://www.youtube.com/", nil
	case connectors.ConnectorBilibili:
		return "https://www.bilibili.com/", nil
	case connectors.ConnectorTikTok:
		return "https://www.tiktok.com/", nil
	case connectors.ConnectorDouyin:
		return "https://www.douyin.com/", nil
	case connectors.ConnectorInstagram:
		return "https://www.instagram.com/", nil
	case connectors.ConnectorX:
		return "https://x.com/", nil
	case connectors.ConnectorFacebook:
		return "https://www.facebook.com/", nil
	case connectors.ConnectorVimeo:
		return "https://vimeo.com/", nil
	case connectors.ConnectorTwitch:
		return "https://www.twitch.tv/", nil
	case connectors.ConnectorNiconico:
		return "https://www.nicovideo.jp/", nil
	default:
		return "", connectors.ErrInvalidConnector
	}
}

func startConnectorBrowser(preferredBrowser string, headless bool, userDataDir string) (*browsercdp.Runtime, context.Context, context.CancelFunc, error) {
	runtime, err := browsercdp.Start(context.Background(), browsercdp.LaunchOptions{
		PreferredBrowser: preferredBrowser,
		Headless:         headless,
		UserDataDir:      userDataDir,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	tabCtx, cancel, err := attachConnectorTab(runtime)
	if err != nil {
		runtime.Stop()
		return nil, nil, nil, err
	}
	return runtime, tabCtx, cancel, nil
}

func attachConnectorTab(runtime *browsercdp.Runtime) (context.Context, context.CancelFunc, error) {
	if runtime == nil {
		return nil, nil, connectors.ErrConnectorSessionDead
	}
	tabCtx, cancel, _, err := browsercdp.AttachOrCreatePageTarget(runtime, 5*time.Second)
	if err != nil {
		return nil, nil, err
	}
	return tabCtx, cancel, nil
}

func connectorTargetIDFromContext(tabCtx context.Context) cdptarget.ID {
	if current := chromedp.FromContext(tabCtx); current != nil && current.Target != nil {
		return current.Target.TargetID
	}
	return ""
}

func readConnectorCookies(ctx context.Context) ([]appcookies.Record, error) {
	var records []appcookies.Record
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actionCtx context.Context) error {
		items, err := browsercdp.GetAllCookies(actionCtx)
		if err != nil {
			return err
		}
		records = items
		return nil
	})); err != nil {
		return nil, err
	}
	return records, nil
}

func readConnectorCookiesFromSession(session *connectorSession) ([]appcookies.Record, error) {
	if session == nil {
		return nil, connectors.ErrConnectorSessionGone
	}
	return readConnectorCookiesWithTimeout(session.TabCtx, 2*time.Second)
}

func readConnectorCookiesWithTimeout(ctx context.Context, timeout time.Duration) ([]appcookies.Record, error) {
	if ctx == nil {
		return nil, connectors.ErrConnectorSessionDead
	}
	if timeout <= 0 {
		return readConnectorCookies(ctx)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return readConnectorCookies(timeoutCtx)
}

func waitForConnectorTabClose(ctx context.Context, runtime *browsercdp.Runtime, tabCtx context.Context, targetID cdptarget.ID, captureCookies bool, readCookies func(context.Context) ([]appcookies.Record, error)) ([]appcookies.Record, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var latest []appcookies.Record
	targetDestroyed := watchConnectorTargetDestroyed(tabCtx, targetID)
	var browserDone <-chan struct{}
	if runtime != nil && runtime.BrowserContext() != nil {
		browserDone = runtime.BrowserContext().Done()
	}
	for {
		select {
		case <-ctx.Done():
			return latest, ctx.Err()
		case <-targetDestroyed:
			return latest, nil
		case <-browserDone:
			return latest, nil
		case <-ticker.C:
			if captureCookies && readCookies != nil {
				if cookies, err := readCookies(tabCtx); err == nil {
					latest = cookies
				}
			}
			if runtime == nil || !runtime.Status().Ready {
				return latest, nil
			}
			if targetID != "" {
				exists, err := connectorTargetExists(runtime, targetID)
				if err == nil && !exists {
					return latest, nil
				}
			}
			var currentURL string
			if err := chromedp.Run(tabCtx, chromedp.Location(&currentURL)); err != nil {
				return latest, nil
			}
		}
	}
}

func watchConnectorTargetDestroyed(tabCtx context.Context, targetID cdptarget.ID) <-chan struct{} {
	done := make(chan struct{}, 1)
	if tabCtx == nil || targetID == "" {
		return done
	}
	chromedp.ListenTarget(tabCtx, func(ev any) {
		switch current := ev.(type) {
		case *cdptarget.EventTargetDestroyed:
			if current.TargetID != targetID {
				return
			}
		case *cdptarget.EventTargetCrashed:
			if current.TargetID != targetID {
				return
			}
		default:
			return
		}
		select {
		case done <- struct{}{}:
		default:
		}
	})
	return done
}

func connectorSessionDir(connectorType connectors.ConnectorType, sessionID string) string {
	return filepath.Join(connectorSessionRootDir(), string(connectorType), sessionID)
}

func connectorOpenDir(connectorType connectors.ConnectorType, sessionID string) string {
	return filepath.Join(connectorSessionRootDir(), "open", string(connectorType), sessionID)
}

func connectorSessionRootDir() string {
	return filepath.Join(os.TempDir(), "xiadown", "connectors")
}

func cookieDomains(records []appcookies.Record) []string {
	if len(records) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(records))
	result := make([]string, 0, len(records))
	for _, record := range records {
		domain := strings.TrimSpace(record.Domain)
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		result = append(result, domain)
	}
	sort.Strings(result)
	return result
}
