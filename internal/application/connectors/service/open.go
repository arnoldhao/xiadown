package service

import (
	"context"
	"strings"

	"github.com/chromedp/chromedp"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/connectors/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/connectors"
)

func (service *ConnectorsService) OpenConnectorSite(ctx context.Context, request dto.OpenConnectorSiteRequest) (dto.StartConnectorConnectResult, error) {
	id := strings.TrimSpace(request.ID)
	if id == "" {
		return dto.StartConnectorConnectResult{}, connectors.ErrInvalidConnector
	}
	connector, err := service.repo.Get(ctx, id)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	connector, changed, err := service.normalizeConnectorCredential(connector)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	if changed {
		if err := service.repo.Save(ctx, connector); err != nil {
			return dto.StartConnectorConnectResult{}, err
		}
	}
	connector, changed, err = service.bindConnectorProfileToCurrentBrowser(ctx, connector)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	if changed {
		if err := service.repo.Save(ctx, connector); err != nil {
			return dto.StartConnectorConnectResult{}, err
		}
	}
	targetURL, err := connectorTargetURL(connector.Type, request.TargetURL)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	sessionID := service.newSessionID()
	userDataDir := connectorOpenDir(connector.Type, sessionID)
	removeUserDataDir := true
	persistentProfile := false
	var cookies []appcookies.Record
	if connector.CredentialMode == connectors.CredentialModeProfile {
		userDataDir = connector.ProfilePath
		removeUserDataDir = false
		persistentProfile = true
	} else {
		cookies = decodeCookies(connector.CookiesJSON)
		if len(cookies) == 0 {
			return dto.StartConnectorConnectResult{}, connectors.ErrNoCookies
		}
	}
	runtime, tabCtx, cancel, targetID, err := service.startBrowser(service.preferredBrowser(ctx), false, userDataDir, persistentProfile)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	session := &connectorSession{
		ID:                sessionID,
		ConnectorID:       connector.ID,
		ConnectorType:     connector.Type,
		CredentialMode:    connector.CredentialMode,
		Purpose:           connectorSessionPurposeOpen,
		Runtime:           runtime,
		TabCtx:            tabCtx,
		Cancel:            cancel,
		UserDataDir:       userDataDir,
		RemoveUserDataDir: removeUserDataDir,
		ProfileBrowser:    connector.ProfileBrowser,
		TargetURL:         targetURL,
		State:             connectorSessionStateRunning,
		ConnectorSnapshot: mapConnectorDTO(connector),
		finalizeDone:      make(chan struct{}),
		TargetID:          targetID,
	}
	if targetID == "" {
		session.TargetID = connectorTargetIDFromContext(tabCtx)
	}
	replaced := service.putSession(session)
	service.cleanupSession(replaced)
	go service.navigateOpenConnectorSiteSession(sessionID, targetURL, cookies)

	return dto.StartConnectorConnectResult{
		SessionID: sessionID,
		Connector: mapConnectorDTO(connector),
		TargetURL: targetURL,
	}, nil
}

func (service *ConnectorsService) navigateOpenConnectorSiteSession(sessionID string, targetURL string, cookies []appcookies.Record) {
	session, ok := service.getSession(sessionID)
	if !ok || session == nil || session.TabCtx == nil {
		return
	}
	if session.CredentialMode == connectors.CredentialModeCookies {
		if err := chromedp.Run(session.TabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return setOpenConnectorCookies(ctx, targetURL, cookies)
		})); err != nil {
			service.failOpenConnectorSiteSession(sessionID, session, err)
			return
		}
	}
	if err := chromedp.Run(session.TabCtx, chromedp.Navigate(targetURL)); err != nil {
		service.failOpenConnectorSiteSession(sessionID, session, err)
	}
}

func setOpenConnectorCookies(ctx context.Context, targetURL string, cookies []appcookies.Record) error {
	if err := browsercdp.SetCookiesOnBrowser(ctx, targetURL, cookies); err != nil {
		if fallbackErr := browsercdp.SetCookies(ctx, targetURL, cookies); fallbackErr != nil {
			return err
		}
	}
	return nil
}

func (service *ConnectorsService) failOpenConnectorSiteSession(sessionID string, session *connectorSession, err error) {
	if err == nil {
		return
	}
	service.updateSession(sessionID, func(current *connectorSession) {
		current.State = connectorSessionStateFailed
		current.FinalError = err.Error()
	})
	service.cleanupSession(session)
}
