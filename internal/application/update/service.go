package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"xiadown/internal/application/events"
	"xiadown/internal/application/softwareupdate"
	"xiadown/internal/domain/update"
)

type Downloader interface {
	Download(ctx context.Context, url string, progress func(int)) (string, error)
}

type Installer interface {
	Install(ctx context.Context, artifactPath string, prepared update.Info) error
	RestartToApply(ctx context.Context) error
}

type downloadURLSelector interface {
	SelectDownloadURLs(ctx context.Context, urls []string) []string
}

type preparedUpdateInspector interface {
	PreparedUpdate(ctx context.Context) (update.Info, bool, error)
	ClearPreparedUpdate(ctx context.Context) error
}

const (
	dreamFMLiveCatalogTopic       = "dreamfm.live.catalog"
	dreamFMLiveCatalogUpdatedType = "catalog-updated"
)

type dreamFMLiveCatalogUpdatePayload struct {
	SchemaVersion int    `json:"schemaVersion,omitempty"`
	URL           string `json:"url"`
	Version       string `json:"version,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
	MinAppVersion string `json:"minAppVersion,omitempty"`
	TTLSeconds    int    `json:"ttlSeconds,omitempty"`
	Fallback      string `json:"fallback,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	Hash          string `json:"hash,omitempty"`
	Fingerprint   string `json:"fingerprint"`
}

type whatsNewStore interface {
	PendingWhatsNew(ctx context.Context) (update.WhatsNew, bool, error)
	SeenWhatsNewVersion(ctx context.Context) (string, error)
	MarkWhatsNewSeen(ctx context.Context, version string) error
}

type Notifier interface {
	SetUpdateAvailable(available bool)
	NotifyUpdateState(info update.Info)
}

type Service struct {
	mu                  sync.Mutex
	state               update.Info
	catalog             *softwareupdate.Service
	downloader          Downloader
	installer           Installer
	bus                 events.Bus
	notifier            Notifier
	now                 func() time.Time
	scheduleTicker      *time.Ticker
	cancelSchedule      context.CancelFunc
	downloadURLs        []string
	downloadSHA256      string
	autoPrepareInFlight bool
}

type ServiceParams struct {
	Catalog    *softwareupdate.Service
	Downloader Downloader
	Installer  Installer
	Bus        events.Bus
	Notifier   Notifier
}

func NewService(params ServiceParams) *Service {
	return &Service{
		catalog:    params.Catalog,
		downloader: params.Downloader,
		installer:  params.Installer,
		bus:        params.Bus,
		notifier:   params.Notifier,
		now:        time.Now,
		state: update.Info{
			Kind:   update.KindApp,
			Status: update.StatusIdle,
		},
	}
}

func (service *Service) State() update.Info {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.state
}

// PublishCurrentState pushes current state to subscribers/notifier.
func (service *Service) PublishCurrentState() {
	service.publishState()
}

// SetCurrentVersion seeds the current version so it can be surfaced before any check.
func (service *Service) SetCurrentVersion(version string) {
	service.mu.Lock()
	service.state.CurrentVersion = update.NormalizeVersion(version)
	if service.state.CurrentVersion != "" &&
		service.state.PreparedVersion != "" &&
		update.CompareVersion(service.state.CurrentVersion, service.state.PreparedVersion) >= 0 {
		service.clearPreparedStateLocked()
	}
	if service.state.CurrentVersion != "" &&
		service.state.LatestVersion != "" &&
		update.CompareVersion(service.state.CurrentVersion, service.state.LatestVersion) >= 0 &&
		!service.state.HasPreparedUpdate() {
		service.state.Status = update.StatusIdle
		service.state.Progress = 0
		service.state.DownloadURL = ""
		service.state.Message = ""
		service.downloadURLs = nil
		service.downloadSHA256 = ""
	}
	service.mu.Unlock()
}

func (service *Service) RestorePreparedUpdate(ctx context.Context) (update.Info, error) {
	inspector, ok := service.installer.(preparedUpdateInspector)
	if !ok || inspector == nil {
		return service.State(), nil
	}

	prepared, found, err := inspector.PreparedUpdate(ctx)
	if err != nil {
		return service.State(), err
	}
	if !found {
		return service.State(), nil
	}

	preparedVersion := update.NormalizeVersion(prepared.PreparedVersion)
	currentVersion := update.NormalizeVersion(service.State().CurrentVersion)
	if preparedVersion != "" && currentVersion != "" && update.CompareVersion(currentVersion, preparedVersion) >= 0 {
		if clearErr := inspector.ClearPreparedUpdate(ctx); clearErr != nil {
			zap.L().Warn("update: clear stale prepared update failed", zap.Error(clearErr))
		}
		service.mu.Lock()
		service.clearPreparedStateLocked()
		state := service.state
		service.mu.Unlock()
		return state, nil
	}

	service.mu.Lock()
	service.state.Kind = update.KindApp
	if strings.TrimSpace(service.state.LatestVersion) == "" ||
		update.CompareVersion(preparedVersion, service.state.LatestVersion) >= 0 {
		service.state.LatestVersion = preparedVersion
		service.state.Changelog = prepared.PreparedChangelog
	}
	service.state.PreparedVersion = preparedVersion
	service.state.PreparedChangelog = prepared.PreparedChangelog
	service.setPreparedReadyLocked()
	state := service.state
	service.mu.Unlock()
	service.notifyAvailability(true)
	return state, nil
}

func (service *Service) GetWhatsNew(ctx context.Context) (update.WhatsNew, error) {
	store, ok := service.installer.(whatsNewStore)
	if !ok || store == nil {
		return update.WhatsNew{}, nil
	}

	currentVersion := update.NormalizeVersion(service.State().CurrentVersion)
	if !isReleaseVersion(currentVersion) {
		return update.WhatsNew{}, nil
	}

	seenVersion, err := store.SeenWhatsNewVersion(ctx)
	if err != nil {
		return update.WhatsNew{}, err
	}
	seenVersion = update.NormalizeVersion(seenVersion)

	pending, found, err := store.PendingWhatsNew(ctx)
	if err != nil {
		return update.WhatsNew{}, err
	}
	if found {
		pendingVersion := update.NormalizeVersion(pending.Version)
		switch {
		case pendingVersion != "" &&
			update.CompareVersion(currentVersion, pendingVersion) == 0 &&
			(seenVersion == "" || update.CompareVersion(pendingVersion, seenVersion) > 0):
			pending.CurrentVersion = currentVersion
			if strings.TrimSpace(pending.Changelog) == "" {
				pending.Changelog = service.resolveReleaseNotes(ctx, pendingVersion)
			}
			return pending, nil
		case pendingVersion != "" && update.CompareVersion(currentVersion, pendingVersion) > 0:
			// A newer version is already running; ignore the older pending notice.
		case pendingVersion != "" && update.CompareVersion(currentVersion, pendingVersion) < 0:
			return update.WhatsNew{}, nil
		}
	}

	if seenVersion != "" && update.CompareVersion(currentVersion, seenVersion) <= 0 {
		return update.WhatsNew{}, nil
	}

	return update.WhatsNew{
		Version:        currentVersion,
		CurrentVersion: currentVersion,
		Changelog:      service.resolveReleaseNotes(ctx, currentVersion),
	}, nil
}

func (service *Service) DismissWhatsNew(ctx context.Context, version string) error {
	store, ok := service.installer.(whatsNewStore)
	if !ok || store == nil {
		return nil
	}
	return store.MarkWhatsNewSeen(ctx, update.NormalizeVersion(version))
}

func (service *Service) CheckForUpdate(ctx context.Context, currentVersion string) (update.Info, error) {
	service.mu.Lock()
	if currentVersion != "" {
		service.state.CurrentVersion = update.NormalizeVersion(currentVersion)
	}
	if service.state.Status == update.StatusDownloading || service.state.Status == update.StatusInstalling {
		state := service.state
		service.mu.Unlock()
		return state, nil
	}
	service.setStatusLocked(update.StatusChecking, 0, "")
	state := service.state
	service.mu.Unlock()
	service.publishSnapshot(state)
	zap.L().Info("update: checking for updates",
		zap.String("currentVersion", state.CurrentVersion),
	)

	if service.catalog == nil {
		return service.publishError(fmt.Errorf("software update catalog not configured"))
	}

	previousSnapshot := service.catalog.Snapshot()
	snapshot, refreshErr := service.catalog.RefreshCatalog(ctx, softwareupdate.Request{AppVersion: state.CurrentVersion})
	if refreshErr == nil {
		service.publishDreamFMLiveCatalogUpdate(previousSnapshot, snapshot)
	}
	release, err := service.catalog.ResolveAppRelease(ctx, softwareupdate.AppRequest{
		CurrentVersion: state.CurrentVersion,
	})

	if err != nil {
		return service.publishCheckError(err)
	}

	downloadURLs := service.selectDownloadURLs(ctx, release.Asset.DownloadURLs())

	service.mu.Lock()
	latest := update.NormalizeVersion(release.Version)
	current := update.NormalizeVersion(service.state.CurrentVersion)
	service.state.LatestVersion = latest
	service.state.Changelog = release.Notes
	service.state.DownloadURL = ""
	if len(downloadURLs) > 0 {
		service.state.DownloadURL = downloadURLs[0]
	}
	service.state.CheckedAt = service.now()
	service.downloadURLs = downloadURLs
	service.downloadSHA256 = normalizeSHA256(release.Asset.SHA256)
	zap.L().Info("update: check result",
		zap.String("currentVersion", current),
		zap.String("latestVersion", latest),
		zap.Bool("hasDownload", service.state.DownloadURL != ""),
	)

	if current != "" && latest != "" && update.CompareVersion(current, latest) >= 0 {
		if service.state.HasPreparedUpdate() {
			service.setPreparedReadyLocked()
			state := service.state
			service.mu.Unlock()
			service.notifyAvailability(true)
			service.publishSnapshot(state)
			return state, nil
		}
		service.setStatusLocked(update.StatusNoUpdate, 0, "")
		state := service.state
		service.downloadURLs = nil
		service.downloadSHA256 = ""
		service.mu.Unlock()
		service.notifyAvailability(false)
		service.publishSnapshot(state)
		return state, nil
	}

	if service.state.DownloadURL == "" {
		service.mu.Unlock()
		return service.publishPrepareError(fmt.Errorf("no downloadable asset for update"), service.capturePreparedFallback())
	}

	if service.state.HasPreparedUpdate() &&
		update.CompareVersion(service.state.PreparedVersion, latest) == 0 {
		service.setPreparedReadyLocked()
		state := service.state
		service.mu.Unlock()
		service.notifyAvailability(true)
		service.publishSnapshot(state)
		return state, nil
	}

	service.setStatusLocked(update.StatusAvailable, 0, "")
	state = service.state
	shouldAutoPrepare := service.shouldAutoPrepareLocked()
	service.mu.Unlock()
	service.notifyAvailability(true)
	service.publishSnapshot(state)
	if shouldAutoPrepare {
		service.scheduleAutoPrepare()
	}
	return state, nil
}

func (service *Service) DownloadUpdate(ctx context.Context) (update.Info, error) {
	service.mu.Lock()
	if service.state.Status == update.StatusDownloading || service.state.Status == update.StatusInstalling {
		state := service.state
		service.mu.Unlock()
		return state, nil
	}
	downloadURLs := service.resolveDownloadURLsLocked()
	expectedSHA256 := service.downloadSHA256
	fallback := service.capturePreparedFallbackLocked()
	service.setStatusLocked(update.StatusDownloading, 0, "")
	state := service.state
	service.mu.Unlock()
	service.publishSnapshot(state)

	if len(downloadURLs) == 0 {
		return service.publishPrepareError(fmt.Errorf("missing download url"), fallback)
	}
	if service.downloader == nil {
		return service.publishPrepareError(fmt.Errorf("downloader not configured"), fallback)
	}

	var path string
	var err error
	for _, downloadURL := range downloadURLs {
		path, err = service.downloader.Download(ctx, downloadURL, func(progress int) {
			service.mu.Lock()
			service.state.Progress = progress
			state := service.state
			service.mu.Unlock()
			service.publishSnapshot(state)
		})
		if err == nil {
			if verifyErr := verifyDownloadedAsset(path, expectedSHA256); verifyErr == nil {
				break
			} else {
				err = verifyErr
			}
		}
		zap.L().Warn("update: download source failed", zap.String("url", downloadURL), zap.Error(err))
	}
	if err != nil {
		return service.publishPrepareError(err, fallback)
	}

	service.mu.Lock()
	service.state.Progress = 100
	service.state.Message = path
	service.setStatusLocked(update.StatusInstalling, 100, "")
	installingState := service.state
	service.mu.Unlock()
	service.publishSnapshot(installingState)

	if service.installer != nil {
		if err := service.installer.Install(ctx, path, installingState); err != nil {
			return service.publishPrepareError(err, fallback)
		}
	}

	service.mu.Lock()
	service.state.PreparedVersion = update.NormalizeVersion(service.state.LatestVersion)
	service.state.PreparedChangelog = service.state.Changelog
	service.setStatusLocked(update.StatusReadyToRestart, 100, "")
	finalState := service.state
	service.mu.Unlock()
	service.notifyAvailability(true)
	service.publishSnapshot(finalState)

	return finalState, nil
}

func (service *Service) RestartToApply(ctx context.Context) (update.Info, error) {
	if service.installer == nil {
		return service.publishError(fmt.Errorf("installer not configured"))
	}
	if err := service.installer.RestartToApply(ctx); err != nil {
		return service.publishError(err)
	}
	service.mu.Lock()
	service.clearPreparedStateLocked()
	service.setStatusLocked(update.StatusIdle, 0, "")
	service.downloadURLs = nil
	service.downloadSHA256 = ""
	state := service.state
	service.mu.Unlock()
	service.notifyAvailability(false)
	service.publishSnapshot(state)
	return state, nil
}

// ScheduleAutoCheck starts a ticker-based auto check; call StopAutoCheck on shutdown.
func (service *Service) ScheduleAutoCheck(ctx context.Context, initialDelay time.Duration, interval time.Duration, currentVersion string) {
	service.StopAutoCheck()
	if initialDelay <= 0 {
		initialDelay = 3 * time.Minute
	}
	if interval <= 0 {
		interval = time.Hour
	}

	runCtx, cancel := context.WithCancel(ctx)
	service.cancelSchedule = cancel

	go func() {
		select {
		case <-time.After(initialDelay):
			service.safeCheck(runCtx, currentVersion)
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
				service.safeCheck(runCtx, currentVersion)
			case <-runCtx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (service *Service) StopAutoCheck() {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.cancelSchedule != nil {
		service.cancelSchedule()
	}
	if service.scheduleTicker != nil {
		service.scheduleTicker.Stop()
	}
	service.scheduleTicker = nil
	service.cancelSchedule = nil
}

func (service *Service) safeCheck(ctx context.Context, currentVersion string) {
	_, _ = service.CheckForUpdate(ctx, currentVersion) // errors already published
}

func (service *Service) scheduleAutoPrepare() {
	service.mu.Lock()
	if !service.shouldAutoPrepareLocked() {
		service.mu.Unlock()
		return
	}
	latestVersion := service.state.LatestVersion
	service.autoPrepareInFlight = true
	service.mu.Unlock()

	go func() {
		defer service.finishAutoPrepare()
		if _, err := service.DownloadUpdate(context.Background()); err != nil {
			zap.L().Warn("update: auto-prepare failed", zap.String("latestVersion", latestVersion), zap.Error(err))
		}
	}()
}

func (service *Service) finishAutoPrepare() {
	service.mu.Lock()
	service.autoPrepareInFlight = false
	service.mu.Unlock()
}

func (service *Service) publishCheckError(err error) (update.Info, error) {
	service.mu.Lock()
	service.state.CheckedAt = service.now()
	if service.state.HasPreparedUpdate() {
		service.setPreparedReadyLocked()
		state := service.state
		service.mu.Unlock()
		service.notifyAvailability(true)
		service.publishSnapshot(state)
		return state, err
	}
	service.setStatusLocked(update.StatusError, service.state.Progress, err.Error())
	state := service.state
	service.mu.Unlock()
	service.publishSnapshot(state)
	return state, err
}

func (service *Service) publishPrepareError(err error, fallback preparedFallback) (update.Info, error) {
	service.mu.Lock()
	if fallback.HasPreparedUpdate(service.state.CurrentVersion) {
		service.state.PreparedVersion = fallback.Version
		service.state.PreparedChangelog = fallback.Changelog
		service.setPreparedReadyLocked()
		state := service.state
		service.mu.Unlock()
		service.notifyAvailability(true)
		service.publishSnapshot(state)
		return state, err
	}
	service.setStatusLocked(update.StatusError, service.state.Progress, err.Error())
	state := service.state
	service.mu.Unlock()
	service.publishSnapshot(state)
	return state, err
}

func (service *Service) publishError(err error) (update.Info, error) {
	service.mu.Lock()
	service.setStatusLocked(update.StatusError, service.state.Progress, err.Error())
	state := service.state
	service.mu.Unlock()
	service.publishSnapshot(state)
	return state, err
}

func (service *Service) setStatusLocked(status update.Status, progress int, message string) {
	service.state.Status = status
	if progress >= 0 && progress <= 100 {
		service.state.Progress = progress
	}
	service.state.Message = message
}

func (service *Service) setPreparedReadyLocked() {
	service.state.Status = update.StatusReadyToRestart
	service.state.Progress = 100
	service.state.Message = ""
}

func (service *Service) clearPreparedStateLocked() {
	service.state.PreparedVersion = ""
	service.state.PreparedChangelog = ""
}

func (service *Service) shouldAutoPrepareLocked() bool {
	if service.autoPrepareInFlight {
		return false
	}
	if service.state.Status != update.StatusAvailable {
		return false
	}
	if strings.TrimSpace(service.state.DownloadURL) == "" {
		return false
	}
	latestVersion := update.NormalizeVersion(service.state.LatestVersion)
	if latestVersion == "" {
		return false
	}
	preparedVersion := update.NormalizeVersion(service.state.PreparedVersion)
	return preparedVersion == "" || update.CompareVersion(latestVersion, preparedVersion) > 0
}

func (service *Service) publishState() {
	service.mu.Lock()
	state := service.state
	service.mu.Unlock()
	service.publishSnapshot(state)
}

func (service *Service) publishSnapshot(state update.Info) {
	if service.bus != nil {
		_ = service.bus.Publish(context.Background(), events.Event{
			Topic:   "update.status",
			Type:    "status",
			Payload: state,
		})
	}
	if service.notifier != nil {
		service.notifier.NotifyUpdateState(state)
	}
}

func (service *Service) publishDreamFMLiveCatalogUpdate(previous softwareupdate.Snapshot, next softwareupdate.Snapshot) {
	if service.bus == nil || previous.CheckedAt.IsZero() {
		return
	}
	previousRef := previous.Catalog.DreamFM.LiveChannel
	nextRef := next.Catalog.DreamFM.LiveChannel
	if !nextRef.Configured() {
		return
	}
	previousFingerprint := previousRef.Fingerprint()
	nextFingerprint := nextRef.Fingerprint()
	if nextFingerprint == "" || previousFingerprint == nextFingerprint {
		return
	}
	updatedAt := ""
	if !nextRef.UpdatedAt.IsZero() {
		updatedAt = nextRef.UpdatedAt.UTC().Format(time.RFC3339)
	}
	_ = service.bus.Publish(context.Background(), events.Event{
		Topic: dreamFMLiveCatalogTopic,
		Type:  dreamFMLiveCatalogUpdatedType,
		Payload: dreamFMLiveCatalogUpdatePayload{
			SchemaVersion: nextRef.SchemaVersion,
			URL:           strings.TrimSpace(nextRef.URL),
			Version:       strings.TrimSpace(nextRef.Version),
			UpdatedAt:     updatedAt,
			MinAppVersion: strings.TrimSpace(nextRef.MinAppVersion),
			TTLSeconds:    nextRef.TTLSeconds,
			Fallback:      strings.TrimSpace(nextRef.Fallback),
			SHA256:        strings.TrimSpace(nextRef.SHA256),
			Hash:          strings.TrimSpace(nextRef.Hash),
			Fingerprint:   nextFingerprint,
		},
	})
}

func (service *Service) notifyAvailability(available bool) {
	if service.notifier == nil {
		return
	}
	service.notifier.SetUpdateAvailable(available)
}

func (service *Service) resolveDownloadURLsLocked() []string {
	if len(service.downloadURLs) > 0 {
		return slices.Clone(service.downloadURLs)
	}
	if strings.TrimSpace(service.state.DownloadURL) == "" {
		return nil
	}
	return []string{service.state.DownloadURL}
}

type preparedFallback struct {
	Version   string
	Changelog string
}

func (fallback preparedFallback) HasPreparedUpdate(currentVersion string) bool {
	preparedVersion := update.NormalizeVersion(fallback.Version)
	current := update.NormalizeVersion(currentVersion)
	return preparedVersion != "" && update.CompareVersion(preparedVersion, current) > 0
}

func (service *Service) capturePreparedFallback() preparedFallback {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.capturePreparedFallbackLocked()
}

func (service *Service) capturePreparedFallbackLocked() preparedFallback {
	return preparedFallback{
		Version:   service.state.PreparedVersion,
		Changelog: service.state.PreparedChangelog,
	}
}

func (service *Service) resolveReleaseNotes(ctx context.Context, version string) string {
	if service.catalog == nil || !isReleaseVersion(version) {
		return ""
	}
	release, err := service.catalog.ResolveAppReleaseByVersion(ctx, version)
	if err != nil {
		zap.L().Warn("update: resolve release notes failed",
			zap.String("version", version),
			zap.Error(err),
		)
		return ""
	}
	return strings.TrimSpace(release.Notes)
}

func isReleaseVersion(version string) bool {
	normalized := update.NormalizeVersion(version)
	if normalized == "" {
		return false
	}
	parts := strings.Split(normalized, ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

func normalizeSHA256(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.TrimPrefix(value, "sha256:")
	return value
}

func verifyDownloadedAsset(path string, expectedSHA256 string) error {
	expected := normalizeSHA256(expectedSHA256)
	if expected == "" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("download checksum mismatch")
	}
	return nil
}

func (service *Service) selectDownloadURLs(ctx context.Context, urls []string) []string {
	if selector, ok := service.installer.(downloadURLSelector); ok && selector != nil {
		if selected := selector.SelectDownloadURLs(ctx, urls); len(selected) > 0 {
			return selected
		}
	}
	return slices.Clone(urls)
}
