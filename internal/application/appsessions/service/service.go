package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

type AppSessionsService struct {
	repo                       appsessions.Repository
	provider                   AppSessionProvider
	accountFetcher             AppSessionAccountFetcher
	changeListener             AppSessionChangeListener
	now                        func() time.Time
	newSessionID               func() string
	accountVerificationTimeout time.Duration

	mu            sync.Mutex
	sessions      map[string]*browserSession
	sessionsByApp map[string]string
}

type AppSessionProvider interface {
	AppSessionsSupported() bool
	StartAppSession(ctx context.Context, request AppSessionStartRequest) (AppSessionBrowser, error)
	LoadAppSessionCookies(ctx context.Context, siteKey string) ([]appcookies.Record, error)
	SaveAppSessionCookies(ctx context.Context, siteKey string, records []appcookies.Record) error
	ClearAppSession(ctx context.Context, siteKey string, domains []string) error
}

type AppSessionAccountFetcher func(ctx context.Context, siteKey string, records []appcookies.Record) (dto.AppSessionAccount, error)

type AppSessionChangeEvent struct {
	Action     string
	AppSession dto.AppSession
	Saved      bool
	Reason     string
}

type AppSessionChangeListener func(ctx context.Context, event AppSessionChangeEvent)

type Option func(*AppSessionsService)

func NewAppSessionsService(repo appsessions.Repository, options ...Option) *AppSessionsService {
	service := &AppSessionsService{
		repo:                       repo,
		now:                        time.Now,
		newSessionID:               uuid.NewString,
		accountVerificationTimeout: 30 * time.Second,
		sessions:                   make(map[string]*browserSession),
		sessionsByApp:              make(map[string]string),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func WithProvider(provider AppSessionProvider) Option {
	return func(service *AppSessionsService) {
		service.provider = provider
	}
}

func WithAccountFetcher(fetcher AppSessionAccountFetcher) Option {
	return func(service *AppSessionsService) {
		service.accountFetcher = fetcher
	}
}

func WithChangeListener(listener AppSessionChangeListener) Option {
	return func(service *AppSessionsService) {
		service.changeListener = listener
	}
}

func (service *AppSessionsService) SetChangeListener(listener AppSessionChangeListener) {
	if service == nil {
		return
	}
	service.changeListener = listener
}

func (service *AppSessionsService) EnsureDefaults(ctx context.Context) error {
	if service == nil || service.repo == nil {
		return nil
	}
	existing, err := service.repo.List(ctx)
	if err != nil {
		return err
	}
	supported := supportedSiteKeySet()
	seen := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		siteKey := strings.TrimSpace(item.SiteKey)
		if _, ok := supported[siteKey]; !ok {
			if err := service.repo.Delete(ctx, item.ID); err != nil {
				return err
			}
			continue
		}
		seen[siteKey] = struct{}{}
	}
	for _, siteKey := range supportedSiteKeys() {
		if _, ok := seen[siteKey]; ok {
			continue
		}
		now := service.now()
		session, err := appsessions.NewSession(appsessions.SessionParams{
			ID:        siteAppSessionID(siteKey),
			SiteKey:   siteKey,
			Status:    string(appsessions.StatusDisconnected),
			CreatedAt: &now,
			UpdatedAt: &now,
		})
		if err != nil {
			return err
		}
		if err := service.repo.Save(ctx, session); err != nil {
			return err
		}
	}
	return nil
}

func (service *AppSessionsService) ListAppSessions(ctx context.Context) ([]dto.AppSession, error) {
	if err := service.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	items, err := service.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	supported := supportedSiteKeySet()
	result := make([]dto.AppSession, 0, len(items))
	for _, item := range items {
		if _, ok := supported[item.SiteKey]; !ok {
			continue
		}
		result = append(result, service.mapSessionDTO(ctx, item))
	}
	order := supportedSiteKeyOrder()
	sortAppSessionDTOs(result, order)
	return result, nil
}

func (service *AppSessionsService) EnsureAppSession(ctx context.Context, siteKey string) (appsessions.Session, error) {
	siteKey = strings.TrimSpace(siteKey)
	if !isSupportedSiteKey(siteKey) {
		return appsessions.Session{}, appsessions.ErrInvalidSession
	}
	if service == nil || service.repo == nil {
		return appsessions.Session{}, appsessions.ErrSessionNotFound
	}
	session, err := service.repo.GetBySiteKey(ctx, siteKey)
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, appsessions.ErrSessionNotFound) {
		return appsessions.Session{}, err
	}
	now := service.now()
	session, err = appsessions.NewSession(appsessions.SessionParams{
		ID:        siteAppSessionID(siteKey),
		SiteKey:   siteKey,
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		return appsessions.Session{}, err
	}
	if err := service.repo.Save(ctx, session); err != nil {
		return appsessions.Session{}, err
	}
	return session, nil
}

func (service *AppSessionsService) ClearAppSession(ctx context.Context, request dto.ClearAppSessionRequest) error {
	session, err := service.sessionByID(ctx, request.ID)
	if err != nil {
		return err
	}
	if service.provider != nil {
		if err := service.provider.ClearAppSession(ctx, session.SiteKey, appSessionCookieDomains(session.SiteKey)); err != nil &&
			!errors.Is(err, appsessions.ErrNoCookies) &&
			!errors.Is(err, appsessions.ErrUnsupported) {
			return err
		}
	}
	now := service.now()
	updated, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        session.ID,
		SiteKey:   session.SiteKey,
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &session.CreatedAt,
		UpdatedAt: &now,
	})
	if err != nil {
		return err
	}
	if err := service.repo.Save(ctx, updated); err != nil {
		return err
	}
	service.notifyAppSessionChanged(ctx, AppSessionChangeEvent{
		Action:     "clear",
		AppSession: service.mapSessionDTOWithCookies(updated, nil),
		Saved:      false,
	})
	return nil
}

func (service *AppSessionsService) RecordsForSiteKey(ctx context.Context, siteKey string) ([]appcookies.Record, error) {
	siteKey = strings.TrimSpace(siteKey)
	if !isSupportedSiteKey(siteKey) {
		return nil, appsessions.ErrSessionNotFound
	}
	if _, err := service.EnsureAppSession(ctx, siteKey); err != nil {
		return nil, err
	}
	records := service.storedCookies(ctx, siteKey)
	if len(records) == 0 {
		return nil, appsessions.ErrNoCookies
	}
	return records, nil
}

func (service *AppSessionsService) ShutdownSessions() int {
	if service == nil {
		return 0
	}
	service.mu.Lock()
	sessions := make([]*browserSession, 0, len(service.sessions))
	for sessionID, session := range service.sessions {
		if session != nil {
			sessions = append(sessions, session)
		}
		delete(service.sessions, sessionID)
	}
	service.sessionsByApp = make(map[string]string)
	service.mu.Unlock()
	for _, session := range sessions {
		service.cleanupSession(session)
	}
	return len(sessions)
}
