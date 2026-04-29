package service

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/google/uuid"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/connectors/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/sitepolicy"
	"xiadown/internal/domain/connectors"
)

type ConnectorsService struct {
	repo connectors.Repository
	now  func() time.Time

	mu                  sync.Mutex
	sessions            map[string]*connectorSession
	sessionsByConnector map[string]string
	startBrowser        func(preferredBrowser string, headless bool, userDataDir string) (*browsercdp.Runtime, context.Context, context.CancelFunc, error)
	readCookies         func(ctx context.Context) ([]appcookies.Record, error)
	removeAll           func(path string) error
	newSessionID        func() string
}

const (
	connectorSessionStateRunning   = "running"
	connectorSessionStateCompleted = "completed"
	connectorSessionStateFailed    = "failed"
)

const (
	connectorBrowserStatusNotOpen       = "not_open"
	connectorBrowserStatusOpen          = "open"
	connectorBrowserStatusTabClosed     = "tab_closed"
	connectorBrowserStatusBrowserClosed = "browser_closed"
	connectorBrowserStatusCompleted     = "completed"
	connectorBrowserStatusFailed        = "failed"
	connectorBrowserStatusUnknown       = "unknown"
)

type connectorSession struct {
	ID                string
	ConnectorID       string
	ConnectorType     connectors.ConnectorType
	Runtime           *browsercdp.Runtime
	TabCtx            context.Context
	Cancel            context.CancelFunc
	UserDataDir       string
	TargetID          target.ID
	State             string
	LastCookies       []appcookies.Record
	LastCookiesAt     time.Time
	FinalResult       *dto.FinishConnectorConnectResult
	FinalError        string
	ConnectorSnapshot dto.Connector
	finalizeOnce      sync.Once
	finalizeDone      chan struct{}
}

func NewConnectorsService(repo connectors.Repository) *ConnectorsService {
	return &ConnectorsService{
		repo:                repo,
		now:                 time.Now,
		sessions:            make(map[string]*connectorSession),
		sessionsByConnector: make(map[string]string),
		startBrowser:        startConnectorBrowser,
		readCookies:         readConnectorCookies,
		removeAll:           os.RemoveAll,
		newSessionID:        uuid.NewString,
	}
}

func (service *ConnectorsService) preferredBrowser(ctx context.Context) string {
	_ = ctx
	return ""
}

func (service *ConnectorsService) EnsureDefaults(ctx context.Context) error {
	defaults := []struct {
		ID   string
		Type connectors.ConnectorType
	}{
		{ID: "connector-youtube", Type: connectors.ConnectorYouTube},
		{ID: "connector-bilibili", Type: connectors.ConnectorBilibili},
		{ID: "connector-tiktok", Type: connectors.ConnectorTikTok},
		{ID: "connector-douyin", Type: connectors.ConnectorDouyin},
		{ID: "connector-instagram", Type: connectors.ConnectorInstagram},
		{ID: "connector-x", Type: connectors.ConnectorX},
		{ID: "connector-facebook", Type: connectors.ConnectorFacebook},
		{ID: "connector-vimeo", Type: connectors.ConnectorVimeo},
		{ID: "connector-twitch", Type: connectors.ConnectorTwitch},
		{ID: "connector-niconico", Type: connectors.ConnectorNiconico},
	}
	existing, err := service.repo.List(ctx)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		if !isSupportedConnectorType(item.Type) {
			if err := service.repo.Delete(ctx, item.ID); err != nil {
				return err
			}
			continue
		}
		seen[item.ID] = struct{}{}
	}
	for _, item := range defaults {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		now := service.now()
		connector, err := connectors.NewConnector(connectors.ConnectorParams{
			ID:        item.ID,
			Type:      string(item.Type),
			Status:    string(connectors.StatusDisconnected),
			CreatedAt: &now,
			UpdatedAt: &now,
		})
		if err != nil {
			return err
		}
		if err := service.repo.Save(ctx, connector); err != nil {
			return err
		}
	}
	return nil
}

func (service *ConnectorsService) ListConnectors(ctx context.Context) ([]dto.Connector, error) {
	items, err := service.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.Connector, 0, len(items))
	for _, item := range items {
		if !isSupportedConnectorType(item.Type) {
			continue
		}
		result = append(result, mapConnectorDTO(item))
	}
	return result, nil
}

func (service *ConnectorsService) UpsertConnector(ctx context.Context, request dto.UpsertConnectorRequest) (dto.Connector, error) {
	id := strings.TrimSpace(request.ID)
	connectorType := strings.TrimSpace(request.Type)
	status := strings.TrimSpace(request.Status)
	cookiesPath := strings.TrimSpace(request.CookiesPath)
	if id == "" {
		id = uuid.NewString()
	}
	if connectorType != "" && !isSupportedConnectorType(connectors.ConnectorType(connectorType)) {
		return dto.Connector{}, connectors.ErrInvalidConnector
	}
	now := service.now()
	createdAt := (*time.Time)(nil)
	var lastVerifiedAt *time.Time
	cookiesJSON := ""
	if existing, err := service.repo.Get(ctx, id); err == nil {
		if connectorType == "" {
			connectorType = string(existing.Type)
		}
		if status == "" {
			status = string(existing.Status)
		}
		if cookiesPath == "" {
			cookiesPath = existing.CookiesPath
		}
		createdAt = &existing.CreatedAt
		lastVerifiedAt = existing.LastVerifiedAt
		cookiesJSON = existing.CookiesJSON
	} else if err != connectors.ErrConnectorNotFound {
		return dto.Connector{}, err
	}
	if status == string(connectors.StatusConnected) {
		lastVerifiedAt = &now
	}

	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             id,
		Type:           connectorType,
		Status:         status,
		CookiesPath:    cookiesPath,
		CookiesJSON:    cookiesJSON,
		LastVerifiedAt: lastVerifiedAt,
		CreatedAt:      createdAt,
		UpdatedAt:      &now,
	})
	if err != nil {
		return dto.Connector{}, err
	}

	if err := service.repo.Save(ctx, connector); err != nil {
		return dto.Connector{}, err
	}

	return mapConnectorDTO(connector), nil
}

func (service *ConnectorsService) ClearConnector(ctx context.Context, request dto.ClearConnectorRequest) error {
	id := strings.TrimSpace(request.ID)
	if id == "" {
		return connectors.ErrInvalidConnector
	}
	connector, err := service.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	now := service.now()
	updated, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:          connector.ID,
		Type:        string(connector.Type),
		Status:      string(connectors.StatusDisconnected),
		CookiesJSON: "",
		CreatedAt:   &connector.CreatedAt,
		UpdatedAt:   &now,
	})
	if err != nil {
		return err
	}
	return service.repo.Save(ctx, updated)
}

func (service *ConnectorsService) putSession(session *connectorSession) *connectorSession {
	if service == nil || session == nil {
		return nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()

	var replaced *connectorSession
	if currentID, ok := service.sessionsByConnector[session.ConnectorID]; ok && currentID != "" {
		replaced = service.sessions[currentID]
		delete(service.sessions, currentID)
	}
	service.sessions[session.ID] = session
	service.sessionsByConnector[session.ConnectorID] = session.ID
	return replaced
}

func (service *ConnectorsService) getSession(sessionID string) (*connectorSession, bool) {
	if service == nil {
		return nil, false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, ok := service.sessions[sessionID]
	return session, ok
}

func (service *ConnectorsService) updateSession(sessionID string, update func(session *connectorSession)) (*connectorSession, bool) {
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

func (service *ConnectorsService) popSession(sessionID string) *connectorSession {
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
	if currentID, ok := service.sessionsByConnector[session.ConnectorID]; ok && currentID == sessionID {
		delete(service.sessionsByConnector, session.ConnectorID)
	}
	return session
}

func (service *ConnectorsService) cleanupSession(session *connectorSession) {
	if session == nil {
		return
	}
	if session.Cancel != nil {
		session.Cancel()
	}
	if session.Runtime != nil {
		session.Runtime.Stop()
	}
	if service.removeAll != nil && strings.TrimSpace(session.UserDataDir) != "" {
		_ = service.removeAll(session.UserDataDir)
	}
}

func (service *ConnectorsService) GetConnectorConnectSession(ctx context.Context, request dto.GetConnectorConnectSessionRequest) (dto.ConnectorConnectSession, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return dto.ConnectorConnectSession{}, connectors.ErrConnectorSessionGone
	}
	session, ok := service.getSession(sessionID)
	if !ok {
		return dto.ConnectorConnectSession{}, connectors.ErrConnectorSessionGone
	}
	return service.snapshotSession(ctx, session), nil
}

func (service *ConnectorsService) snapshotSession(ctx context.Context, session *connectorSession) dto.ConnectorConnectSession {
	if session == nil {
		return dto.ConnectorConnectSession{}
	}
	service.mu.Lock()
	snapshotID := session.ID
	snapshotConnectorID := session.ConnectorID
	snapshotConnectorType := session.ConnectorType
	snapshotState := session.State
	snapshotRuntime := session.Runtime
	snapshotTargetID := session.TargetID
	snapshotLastCookies := append([]appcookies.Record(nil), session.LastCookies...)
	snapshotLastCookiesAt := session.LastCookiesAt
	snapshotFinalError := session.FinalError
	snapshotConnector := session.ConnectorSnapshot
	var snapshotFinalResult *dto.FinishConnectorConnectResult
	if session.FinalResult != nil {
		copyResult := *session.FinalResult
		copyResult.Domains = append([]string(nil), session.FinalResult.Domains...)
		snapshotFinalResult = &copyResult
	}
	service.mu.Unlock()

	connector := snapshotConnector
	if snapshotFinalResult != nil {
		connector = snapshotFinalResult.Connector
	} else if current, err := service.repo.Get(ctx, snapshotConnectorID); err == nil {
		connector = mapConnectorDTO(current)
	}
	lastCookiesAt := ""
	if !snapshotLastCookiesAt.IsZero() {
		lastCookiesAt = snapshotLastCookiesAt.Format(time.RFC3339)
	}
	result := dto.ConnectorConnectSession{
		SessionID:           snapshotID,
		ConnectorID:         snapshotConnectorID,
		State:               snapshotState,
		BrowserStatus:       connectorSessionBrowserStatus(snapshotState, snapshotRuntime, snapshotTargetID, snapshotFinalResult, snapshotFinalError),
		CurrentCookiesCount: connectorSessionCookiesCount(snapshotConnectorType, snapshotLastCookies),
		Error:               snapshotFinalError,
		LastCookiesAt:       lastCookiesAt,
		Connector:           connector,
	}
	if snapshotFinalResult != nil {
		result.Saved = snapshotFinalResult.Saved
		result.RawCookiesCount = snapshotFinalResult.RawCookiesCount
		result.FilteredCookiesCount = snapshotFinalResult.FilteredCookiesCount
		result.CurrentCookiesCount = snapshotFinalResult.FilteredCookiesCount
		result.Domains = append([]string(nil), snapshotFinalResult.Domains...)
		result.Reason = snapshotFinalResult.Reason
	}
	return result
}

func connectorSessionBrowserStatus(state string, runtime *browsercdp.Runtime, targetID target.ID, finalResult *dto.FinishConnectorConnectResult, finalError string) string {
	if strings.TrimSpace(finalError) != "" || state == connectorSessionStateFailed {
		return connectorBrowserStatusFailed
	}
	if finalResult != nil || state == connectorSessionStateCompleted {
		if finalResult != nil {
			switch strings.TrimSpace(finalResult.Reason) {
			case "browser_closed":
				return connectorBrowserStatusBrowserClosed
			case "tab_closed":
				return connectorBrowserStatusTabClosed
			}
		}
		return connectorBrowserStatusCompleted
	}
	if runtime == nil {
		return connectorBrowserStatusNotOpen
	}
	if !runtime.Status().Ready {
		return connectorBrowserStatusBrowserClosed
	}
	if targetID != "" {
		exists, err := connectorTargetExistsWithTimeout(runtime, targetID, 750*time.Millisecond)
		if err != nil {
			return connectorBrowserStatusUnknown
		}
		if !exists {
			return connectorBrowserStatusTabClosed
		}
	}
	return connectorBrowserStatusOpen
}

func connectorSessionCookiesCount(connectorType connectors.ConnectorType, records []appcookies.Record) int {
	if len(records) == 0 {
		return 0
	}
	policy, ok := sitepolicy.ForConnectorType(string(connectorType))
	if !ok || len(policy.Domains) == 0 {
		return len(records)
	}
	return len(appcookies.FilterByDomains(records, policy.Domains))
}

func isSupportedConnectorType(connectorType connectors.ConnectorType) bool {
	switch connectorType {
	case connectors.ConnectorYouTube,
		connectors.ConnectorBilibili,
		connectors.ConnectorTikTok,
		connectors.ConnectorDouyin,
		connectors.ConnectorInstagram,
		connectors.ConnectorX,
		connectors.ConnectorFacebook,
		connectors.ConnectorVimeo,
		connectors.ConnectorTwitch,
		connectors.ConnectorNiconico:
		return true
	default:
		return false
	}
}
