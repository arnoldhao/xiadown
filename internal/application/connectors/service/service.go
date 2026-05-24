package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/google/uuid"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/connectors/dto"
	appcookies "xiadown/internal/application/cookies"
	settingsdto "xiadown/internal/application/settings/dto"
	"xiadown/internal/application/sitepolicy"
	"xiadown/internal/domain/connectors"
)

type ConnectorsService struct {
	repo connectors.Repository
	now  func() time.Time

	mu                  sync.Mutex
	sessions            map[string]*connectorSession
	sessionsByConnector map[string]string
	startBrowser        func(preferredBrowser string, headless bool, userDataDir string, persistentProfile bool) (*browsercdp.Runtime, context.Context, context.CancelFunc, target.ID, error)
	readCookies         func(ctx context.Context) ([]appcookies.Record, error)
	removeAll           func(path string) error
	newSessionID        func() string
	settingsReader      SettingsReader
}

const (
	connectorSessionStateRunning   = "running"
	connectorSessionStateCompleted = "completed"
	connectorSessionStateFailed    = "failed"
)

const (
	connectorSessionPurposeConnect = "connect"
	connectorSessionPurposeOpen    = "open"
)

const (
	connectorBrowserStatusNotOpen       = "not_open"
	connectorBrowserStatusOpen          = "open"
	connectorBrowserStatusTabClosed     = "tab_closed"
	connectorBrowserStatusBrowserClosed = "browser_closed"
	connectorBrowserStatusCompleted     = "completed"
	connectorBrowserStatusFailed        = "failed"
	connectorBrowserStatusUnknown       = "unknown"
	connectorDefaultProfileBrowser      = "default"
)

type connectorSession struct {
	ID                string
	ConnectorID       string
	ConnectorType     connectors.ConnectorType
	CredentialMode    connectors.CredentialMode
	Purpose           string
	Runtime           *browsercdp.Runtime
	TabCtx            context.Context
	Cancel            context.CancelFunc
	UserDataDir       string
	RemoveUserDataDir bool
	ProfileBrowser    string
	TargetURL         string
	TargetID          target.ID
	State             string
	LastCookies       []appcookies.Record
	LastCookiesAt     time.Time
	FinalResult       *dto.FinishConnectorConnectResult
	FinalError        string
	ConnectorSnapshot dto.Connector
	cleanupOnce       sync.Once
	finalizeOnce      sync.Once
	finalizeDone      chan struct{}
}

type SettingsReader interface {
	GetSettings(ctx context.Context) (settingsdto.Settings, error)
}

type Option func(*ConnectorsService)

func NewConnectorsService(repo connectors.Repository, options ...Option) *ConnectorsService {
	service := &ConnectorsService{
		repo:                repo,
		now:                 time.Now,
		sessions:            make(map[string]*connectorSession),
		sessionsByConnector: make(map[string]string),
		startBrowser:        startConnectorBrowser,
		readCookies:         readConnectorCookies,
		removeAll:           os.RemoveAll,
		newSessionID:        uuid.NewString,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func WithSettingsReader(reader SettingsReader) Option {
	return func(service *ConnectorsService) {
		service.settingsReader = reader
	}
}

func (service *ConnectorsService) preferredBrowser(ctx context.Context) string {
	if service == nil || service.settingsReader == nil {
		return ""
	}
	current, err := service.settingsReader.GetSettings(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(current.DefaultBrowser)
}

func (service *ConnectorsService) EnsureDefaults(ctx context.Context) error {
	defaults := []struct {
		ID   string
		Type connectors.ConnectorType
	}{
		{ID: "connector-youtube", Type: connectors.ConnectorYouTube},
		{ID: "connector-bilibili", Type: connectors.ConnectorBilibili},
		{ID: "connector-tiktok", Type: connectors.ConnectorTikTok},
		{ID: "connector-china-private", Type: connectors.ConnectorChinaPrivate},
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
		normalized, changed, err := service.normalizeConnectorCredential(item)
		if err != nil {
			return err
		}
		if changed {
			if err := service.repo.Save(ctx, normalized); err != nil {
				return err
			}
			item = normalized
		}
		seen[item.ID] = struct{}{}
	}
	for _, item := range defaults {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		now := service.now()
		connector, err := connectors.NewConnector(connectors.ConnectorParams{
			ID:             item.ID,
			Type:           string(item.Type),
			Status:         string(connectors.StatusDisconnected),
			CredentialMode: string(connectors.DefaultCredentialMode(item.Type)),
			CreatedAt:      &now,
			UpdatedAt:      &now,
		})
		if err != nil {
			return err
		}
		connector, _, err = service.normalizeConnectorCredential(connector)
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
		normalized, changed, err := service.normalizeConnectorCredential(item)
		if err != nil {
			return nil, err
		}
		if changed {
			if err := service.repo.Save(ctx, normalized); err != nil {
				return nil, err
			}
			item = normalized
		} else {
			item = normalized
		}
		cleaned, cleanedChanged, err := service.clearMissingConnectorProfileBinding(item)
		if err != nil {
			return nil, err
		}
		if cleanedChanged {
			if err := service.repo.Save(ctx, cleaned); err != nil {
				return nil, err
			}
			item = cleaned
		}
		bound, _, err := service.bindConfiguredConnectorProfileToCurrentBrowser(ctx, item)
		if err != nil {
			return nil, err
		}
		result = append(result, mapConnectorDTO(bound))
	}
	return result, nil
}

func (service *ConnectorsService) EnsureProfileConnector(ctx context.Context, connectorType string) (dto.Connector, error) {
	targetType := connectors.ConnectorType(strings.TrimSpace(connectorType))
	if targetType == "" || !isSupportedConnectorType(targetType) {
		return dto.Connector{}, connectors.ErrInvalidConnector
	}
	if connectors.DefaultCredentialMode(targetType) != connectors.CredentialModeProfile {
		return dto.Connector{}, connectors.ErrInvalidConnector
	}
	if err := service.EnsureDefaults(ctx); err != nil {
		return dto.Connector{}, err
	}
	items, err := service.repo.List(ctx)
	if err != nil {
		return dto.Connector{}, err
	}
	for _, item := range items {
		if !strings.EqualFold(string(item.Type), string(targetType)) {
			continue
		}
		current := item
		changed := false
		if current.CredentialMode != connectors.CredentialModeProfile {
			now := service.now()
			current, err = connectors.NewConnector(connectors.ConnectorParams{
				ID:             current.ID,
				Type:           string(current.Type),
				Status:         string(connectors.StatusDisconnected),
				CredentialMode: string(connectors.CredentialModeProfile),
				ProfileKey:     current.ProfileKey,
				ProfilePath:    current.ProfilePath,
				ProfileBrowser: current.ProfileBrowser,
				CreatedAt:      &current.CreatedAt,
				UpdatedAt:      &now,
			})
			if err != nil {
				return dto.Connector{}, err
			}
			changed = true
		}
		normalized, normalizedChanged, err := service.normalizeConnectorCredential(current)
		if err != nil {
			return dto.Connector{}, err
		}
		if normalizedChanged {
			changed = true
		}
		bound, boundChanged, err := service.bindConnectorProfileToCurrentBrowser(ctx, normalized)
		if err != nil {
			return dto.Connector{}, err
		}
		if boundChanged {
			changed = true
		}
		profilePath := strings.TrimSpace(bound.ProfilePath)
		if profilePath == "" {
			return dto.Connector{}, connectors.ErrInvalidConnector
		}
		if err := os.MkdirAll(profilePath, 0o700); err != nil {
			return dto.Connector{}, err
		}
		_ = os.Chmod(profilePath, 0o700)
		if changed {
			if err := service.repo.Save(ctx, bound); err != nil {
				return dto.Connector{}, err
			}
		}
		return mapConnectorDTO(bound), nil
	}
	return dto.Connector{}, connectors.ErrConnectorNotFound
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
	credentialMode := strings.TrimSpace(request.CredentialMode)
	profileKey := ""
	profilePath := ""
	profileBrowser := ""
	if existing, err := service.repo.Get(ctx, id); err == nil {
		if connectorType == "" {
			connectorType = string(existing.Type)
		}
		if status == "" {
			status = string(existing.Status)
		}
		if credentialMode == "" {
			credentialMode = string(existing.CredentialMode)
		}
		if cookiesPath == "" {
			cookiesPath = existing.CookiesPath
		}
		profileKey = existing.ProfileKey
		profilePath = existing.ProfilePath
		profileBrowser = existing.ProfileBrowser
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
		CredentialMode: credentialMode,
		CookiesPath:    cookiesPath,
		CookiesJSON:    cookiesJSON,
		ProfileKey:     profileKey,
		ProfilePath:    profilePath,
		ProfileBrowser: profileBrowser,
		LastVerifiedAt: lastVerifiedAt,
		CreatedAt:      createdAt,
		UpdatedAt:      &now,
	})
	if err != nil {
		return dto.Connector{}, err
	}
	connector, _, err = service.normalizeConnectorCredential(connector)
	if err != nil {
		return dto.Connector{}, err
	}
	connector, _, err = service.bindConnectorProfileToCurrentBrowser(ctx, connector)
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
	connector, _, err = service.normalizeConnectorCredential(connector)
	if err != nil {
		return err
	}
	clearProfilePath := connectorProfileClearPath(connector)
	now := service.now()
	profilePath := connector.ProfilePath
	profileBrowser := connector.ProfileBrowser
	if connector.CredentialMode == connectors.CredentialModeProfile {
		profilePath = ""
		profileBrowser = ""
	}
	updated, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             connector.ID,
		Type:           string(connector.Type),
		Status:         string(connectors.StatusDisconnected),
		CredentialMode: string(connector.CredentialMode),
		CookiesJSON:    "",
		ProfileKey:     connector.ProfileKey,
		ProfilePath:    profilePath,
		ProfileBrowser: profileBrowser,
		CreatedAt:      &connector.CreatedAt,
		UpdatedAt:      &now,
	})
	if err != nil {
		return err
	}
	if connector.CredentialMode == connectors.CredentialModeProfile && strings.TrimSpace(clearProfilePath) != "" && service.removeAll != nil {
		_ = service.removeAll(clearProfilePath)
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
	session.cleanupOnce.Do(func() {
		if session.Runtime != nil {
			session.Runtime.Stop()
		}
		cancelConnectorSessionContextAsync(session.ID, session.Cancel)
		if session.RemoveUserDataDir && service.removeAll != nil && strings.TrimSpace(session.UserDataDir) != "" {
			_ = service.removeAll(session.UserDataDir)
		}
	})
}

func cancelConnectorSessionContextAsync(sessionID string, cancel context.CancelFunc) {
	if cancel == nil {
		return
	}
	go func() {
		defer func() { _ = recover() }()
		cancel()
	}()
}

func (service *ConnectorsService) ShutdownSessions() int {
	if service == nil {
		return 0
	}
	service.mu.Lock()
	sessions := make([]*connectorSession, 0, len(service.sessions))
	for sessionID, session := range service.sessions {
		if session != nil {
			sessions = append(sessions, session)
		}
		delete(service.sessions, sessionID)
	}
	service.sessionsByConnector = make(map[string]string)
	service.mu.Unlock()

	for _, session := range sessions {
		service.cleanupSession(session)
	}
	return len(sessions)
}

func (service *ConnectorsService) normalizeConnectorCredential(connector connectors.Connector) (connectors.Connector, bool, error) {
	mode := connector.CredentialMode
	if mode == "" {
		mode = connectors.DefaultCredentialMode(connector.Type)
	}
	changed := mode != connector.CredentialMode
	status := connector.Status
	lastVerifiedAt := connector.LastVerifiedAt
	profileKey := connector.ProfileKey
	profilePath := connector.ProfilePath
	profileBrowser := connector.ProfileBrowser
	cookiesPath := connector.CookiesPath
	cookiesJSON := connector.CookiesJSON
	if mode == connectors.CredentialModeProfile {
		if connector.CredentialMode != connectors.CredentialModeProfile {
			if status != connectors.StatusDisconnected || lastVerifiedAt != nil {
				status = connectors.StatusDisconnected
				lastVerifiedAt = nil
				changed = true
			}
		}
		if cookiesPath != "" || cookiesJSON != "" {
			cookiesPath = ""
			cookiesJSON = ""
			changed = true
		}
		if strings.TrimSpace(profileKey) == "" {
			profileKey = defaultConnectorProfileKey(connector)
			changed = true
		}
		if strings.TrimSpace(profilePath) == "" {
			if strings.TrimSpace(profileBrowser) != "" {
				profileBrowser = ""
				changed = true
			}
			if status != connectors.StatusDisconnected || lastVerifiedAt != nil {
				status = connectors.StatusDisconnected
				lastVerifiedAt = nil
				changed = true
			}
		}
	} else if mode == connectors.CredentialModeCookies {
		if profileKey != "" || profilePath != "" || profileBrowser != "" {
			profileKey = ""
			profilePath = ""
			profileBrowser = ""
			changed = true
		}
	}
	if !changed {
		return connector, false, nil
	}
	updated, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             connector.ID,
		Type:           string(connector.Type),
		Status:         string(status),
		CredentialMode: string(mode),
		CookiesPath:    cookiesPath,
		CookiesJSON:    cookiesJSON,
		ProfileKey:     profileKey,
		ProfilePath:    profilePath,
		ProfileBrowser: profileBrowser,
		LastVerifiedAt: lastVerifiedAt,
		CreatedAt:      &connector.CreatedAt,
		UpdatedAt:      &connector.UpdatedAt,
	})
	if err != nil {
		return connectors.Connector{}, false, err
	}
	return updated, true, nil
}

func (service *ConnectorsService) clearMissingConnectorProfileBinding(connector connectors.Connector) (connectors.Connector, bool, error) {
	mode := connector.CredentialMode
	if mode == "" {
		mode = connectors.DefaultCredentialMode(connector.Type)
	}
	if mode != connectors.CredentialModeProfile || strings.TrimSpace(connector.ProfilePath) == "" || !connectorProfilePathGone(connector.ProfilePath) {
		return connector, false, nil
	}
	updated, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             connector.ID,
		Type:           string(connector.Type),
		Status:         string(connectors.StatusDisconnected),
		CredentialMode: string(mode),
		CookiesPath:    connector.CookiesPath,
		CookiesJSON:    connector.CookiesJSON,
		ProfileKey:     connector.ProfileKey,
		CreatedAt:      &connector.CreatedAt,
		UpdatedAt:      &connector.UpdatedAt,
	})
	if err != nil {
		return connectors.Connector{}, false, err
	}
	return updated, true, nil
}

func defaultConnectorProfileKey(connector connectors.Connector) string {
	id := strings.TrimSpace(connector.ID)
	if id == "" {
		id = string(connector.Type)
	}
	return sanitizeConnectorProfileKey(id)
}

func defaultConnectorProfileRootPath(profileKey string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "xiadown", "browser-profiles", "connectors", sanitizeConnectorProfileKey(profileKey)), nil
}

func defaultConnectorProfilePath(profileKey string, browserID string) (string, error) {
	root, err := defaultConnectorProfileRootPath(profileKey)
	if err != nil {
		return "", err
	}
	browserID = sanitizeConnectorProfileKey(firstNonEmptyString(browserID, connectorDefaultProfileBrowser))
	return filepath.Join(root, browserID), nil
}

func (service *ConnectorsService) bindConnectorProfileToCurrentBrowser(ctx context.Context, connector connectors.Connector) (connectors.Connector, bool, error) {
	return bindConnectorProfileToBrowser(connector, service.resolveProfileBrowser(ctx))
}

func (service *ConnectorsService) bindConfiguredConnectorProfileToCurrentBrowser(ctx context.Context, connector connectors.Connector) (connectors.Connector, bool, error) {
	mode := connector.CredentialMode
	if mode == "" {
		mode = connectors.DefaultCredentialMode(connector.Type)
	}
	if mode != connectors.CredentialModeProfile {
		return connector, false, nil
	}
	if strings.TrimSpace(connector.ProfilePath) == "" && strings.TrimSpace(connector.ProfileBrowser) == "" {
		return connector, false, nil
	}
	return service.bindConnectorProfileToCurrentBrowser(ctx, connector)
}

func (service *ConnectorsService) resolveProfileBrowser(ctx context.Context) string {
	preferred := service.preferredBrowser(ctx)
	status := browsercdp.ResolveStatus(preferred, false)
	if strings.TrimSpace(status.ChosenBrowser) != "" {
		return strings.TrimSpace(status.ChosenBrowser)
	}
	if strings.TrimSpace(preferred) != "" {
		return sanitizeConnectorProfileKey(preferred)
	}
	return connectorDefaultProfileBrowser
}

func bindConnectorProfileToBrowser(connector connectors.Connector, browserID string) (connectors.Connector, bool, error) {
	mode := connector.CredentialMode
	if mode == "" {
		mode = connectors.DefaultCredentialMode(connector.Type)
	}
	if mode != connectors.CredentialModeProfile {
		return connector, false, nil
	}
	browserID = sanitizeConnectorProfileKey(firstNonEmptyString(browserID, connector.ProfileBrowser, connectorDefaultProfileBrowser))
	profileKey := strings.TrimSpace(connector.ProfileKey)
	if profileKey == "" {
		profileKey = defaultConnectorProfileKey(connector)
	}
	profilePath := strings.TrimSpace(connector.ProfilePath)
	if !strings.EqualFold(strings.TrimSpace(connector.ProfileBrowser), browserID) || profilePath == "" {
		var err error
		profilePath, err = defaultConnectorProfilePath(profileKey, browserID)
		if err != nil {
			return connectors.Connector{}, false, err
		}
	}
	changed := mode != connector.CredentialMode ||
		profileKey != connector.ProfileKey ||
		profilePath != connector.ProfilePath ||
		browserID != connector.ProfileBrowser
	if !changed {
		return connector, false, nil
	}
	updated, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             connector.ID,
		Type:           string(connector.Type),
		Status:         string(connector.Status),
		CredentialMode: string(mode),
		CookiesPath:    connector.CookiesPath,
		CookiesJSON:    connector.CookiesJSON,
		ProfileKey:     profileKey,
		ProfilePath:    profilePath,
		ProfileBrowser: browserID,
		LastVerifiedAt: connector.LastVerifiedAt,
		CreatedAt:      &connector.CreatedAt,
		UpdatedAt:      &connector.UpdatedAt,
	})
	if err != nil {
		return connectors.Connector{}, false, err
	}
	return updated, true, nil
}

func sanitizeConnectorProfileKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "default"
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, current := range trimmed {
		switch {
		case current >= 'a' && current <= 'z',
			current >= 'A' && current <= 'Z',
			current >= '0' && current <= '9',
			current == '-',
			current == '_',
			current == '.':
			builder.WriteRune(current)
		default:
			builder.WriteByte('_')
		}
	}
	result := strings.Trim(builder.String(), "._-")
	if result == "" {
		return "default"
	}
	return result
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func connectorProfilePathGone(profilePath string) bool {
	profilePath = strings.TrimSpace(profilePath)
	if profilePath == "" {
		return false
	}
	stat, err := os.Stat(profilePath)
	if err != nil {
		return os.IsNotExist(err)
	}
	return !stat.IsDir()
}

func connectorProfileClearPath(connector connectors.Connector) string {
	profilePath := strings.TrimSpace(connector.ProfilePath)
	profileKey := strings.TrimSpace(connector.ProfileKey)
	if profileKey == "" {
		profileKey = defaultConnectorProfileKey(connector)
	}
	rootPath, err := defaultConnectorProfileRootPath(profileKey)
	if err != nil {
		return profilePath
	}
	if profilePath == "" || pathContains(rootPath, profilePath) {
		return rootPath
	}
	return profilePath
}

func pathContains(parent string, child string) bool {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)
	if parent == "" || child == "" {
		return false
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(parentAbs, childAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
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
	snapshotTargetURL := session.TargetURL
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
		if bound, _, bindErr := service.bindConnectorProfileToCurrentBrowser(ctx, current); bindErr == nil {
			current = bound
		}
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
		TargetURL:           snapshotTargetURL,
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
		connectors.ConnectorChinaPrivate,
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
