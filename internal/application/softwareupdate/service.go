package softwareupdate

import (
	"context"
	"strings"
	"sync"
	"time"
)

const (
	SourceManifest = "manifest"
)

type CatalogProvider interface {
	FetchCatalog(ctx context.Context, request Request) (Catalog, error)
}

type ServiceParams struct {
	CatalogProvider CatalogProvider
}

type Service struct {
	catalogProvider CatalogProvider
	now             func() time.Time

	mu              sync.Mutex
	snapshot        Snapshot
	refreshInFlight bool
	refreshWait     chan struct{}
	cancelSchedule  context.CancelFunc
	scheduleTicker  *time.Ticker
}

func NewService(params ServiceParams) *Service {
	return &Service{
		catalogProvider: params.CatalogProvider,
		now:             time.Now,
	}
}

func (service *Service) Snapshot() Snapshot {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.snapshot
}

func (service *Service) EnsureCatalog(ctx context.Context, maxAge time.Duration, request Request) (Snapshot, error) {
	service.mu.Lock()
	snapshot := service.snapshot
	service.mu.Unlock()
	if maxAge > 0 && snapshot.LastError == "" && !snapshot.CheckedAt.IsZero() && service.now().Sub(snapshot.CheckedAt) < maxAge {
		return snapshot, nil
	}
	return service.RefreshCatalog(ctx, request)
}

func (service *Service) RefreshCatalog(ctx context.Context, request Request) (Snapshot, error) {
	if service == nil || service.catalogProvider == nil {
		return Snapshot{}, ErrReleaseNotFound
	}

	for {
		service.mu.Lock()
		if !service.refreshInFlight {
			service.refreshInFlight = true
			service.refreshWait = make(chan struct{})
			service.mu.Unlock()
			break
		}
		wait := service.refreshWait
		service.mu.Unlock()

		select {
		case <-ctx.Done():
			return service.Snapshot(), ctx.Err()
		case <-wait:
		}
	}

	catalog, err := service.catalogProvider.FetchCatalog(ctx, request)
	snapshot := Snapshot{
		Catalog:    catalog,
		CheckedAt:  service.now(),
		LastSource: SourceManifest,
	}
	if err != nil {
		snapshot.LastError = err.Error()
	}

	service.mu.Lock()
	service.snapshot = snapshot
	wait := service.refreshWait
	service.refreshWait = nil
	service.refreshInFlight = false
	service.mu.Unlock()

	if wait != nil {
		close(wait)
	}

	if err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (service *Service) ResolveAppRelease(ctx context.Context, request AppRequest) (AppRelease, error) {
	if service == nil {
		return AppRelease{}, ErrReleaseNotFound
	}
	snapshot, err := service.EnsureCatalog(ctx, time.Minute, Request{
		Channel:    request.Channel,
		AppVersion: request.CurrentVersion,
	})
	if err == nil && snapshot.Catalog.App != nil {
		release := *snapshot.Catalog.App
		release.ResolvedBy = SourceManifest
		return release, nil
	}
	if err != nil {
		return AppRelease{}, err
	}
	return AppRelease{}, ErrReleaseNotFound
}

func (service *Service) ResolveAppReleaseByVersion(ctx context.Context, version string) (AppRelease, error) {
	normalizedVersion := normalizeAppReleaseVersion(version)
	if service == nil || normalizedVersion == "" {
		return AppRelease{}, ErrReleaseNotFound
	}

	snapshot, err := service.EnsureCatalog(ctx, time.Minute, Request{AppVersion: normalizedVersion})
	if err == nil && snapshot.Catalog.App != nil && sameAppReleaseVersion(snapshot.Catalog.App.Version, normalizedVersion) {
		release := *snapshot.Catalog.App
		release.ResolvedBy = SourceManifest
		return release, nil
	}
	if err != nil {
		return AppRelease{}, err
	}
	return AppRelease{}, ErrReleaseNotFound
}

func (service *Service) ResolveDependencyRelease(ctx context.Context, request DependencyRequest) (DependencyRelease, error) {
	if service == nil {
		return DependencyRelease{}, ErrReleaseNotFound
	}
	snapshot, err := service.EnsureCatalog(ctx, time.Minute, Request{
		Channel:    request.Channel,
		AppVersion: request.AppVersion,
	})
	if err == nil {
		if release, ok := snapshot.Catalog.Dependency(request.Name); ok {
			release.ResolvedBy = SourceManifest
			return release, nil
		}
	}
	if err != nil {
		return DependencyRelease{}, err
	}
	return DependencyRelease{}, ErrReleaseNotFound
}

func (service *Service) StartSchedule(ctx context.Context, initialDelay time.Duration, interval time.Duration, request Request) {
	service.StopSchedule()
	if initialDelay < 0 {
		initialDelay = 0
	}
	if interval <= 0 {
		interval = time.Hour
	}
	runCtx, cancel := context.WithCancel(ctx)

	service.mu.Lock()
	service.cancelSchedule = cancel
	service.mu.Unlock()

	go func() {
		select {
		case <-time.After(initialDelay):
			_, _ = service.RefreshCatalog(runCtx, request)
		case <-runCtx.Done():
			return
		}

		ticker := time.NewTicker(interval)
		service.mu.Lock()
		service.scheduleTicker = ticker
		service.mu.Unlock()

		for {
			select {
			case <-ticker.C:
				_, _ = service.RefreshCatalog(runCtx, request)
			case <-runCtx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func normalizeAppReleaseVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, "v"), "V")
	return trimmed
}

func sameAppReleaseVersion(left string, right string) bool {
	normalizedLeft := normalizeAppReleaseVersion(left)
	normalizedRight := normalizeAppReleaseVersion(right)
	return normalizedLeft != "" && normalizedLeft == normalizedRight
}

func (service *Service) StopSchedule() {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.cancelSchedule != nil {
		service.cancelSchedule()
	}
	if service.scheduleTicker != nil {
		service.scheduleTicker.Stop()
	}
	service.cancelSchedule = nil
	service.scheduleTicker = nil
}
