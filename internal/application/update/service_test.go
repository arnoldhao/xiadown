package update

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"xiadown/internal/application/events"
	"xiadown/internal/application/softwareupdate"
	domainupdate "xiadown/internal/domain/update"
)

type catalogProviderStub struct {
	fetchCount int
	catalog    softwareupdate.Catalog
	catalogs   []softwareupdate.Catalog
	err        error
}

type downloaderStub struct {
	path string
	err  error
}

func (stub *downloaderStub) Download(_ context.Context, _ string, progress func(int)) (string, error) {
	if progress != nil {
		progress(100)
	}
	if stub.err != nil {
		return "", stub.err
	}
	return stub.path, nil
}

type installerStub struct {
	installErr            error
	restartErr            error
	restarted             bool
	selectedDownloadURLs  []string
	selectDownloadInvoked bool
	preparedInfo          domainupdate.Info
	hasPreparedInfo       bool
	clearPreparedInvoked  bool
	pendingWhatsNew       domainupdate.WhatsNew
	hasPendingWhatsNew    bool
	seenWhatsNewVersion   string
	markSeenVersion       string
}

func (stub *installerStub) Install(_ context.Context, _ string, _ domainupdate.Info) error {
	return stub.installErr
}

func (stub *installerStub) RestartToApply(_ context.Context) error {
	stub.restarted = true
	return stub.restartErr
}

func (stub *installerStub) SelectDownloadURLs(_ context.Context, urls []string) []string {
	stub.selectDownloadInvoked = true
	if stub.selectedDownloadURLs != nil {
		return stub.selectedDownloadURLs
	}
	return urls
}

func (stub *installerStub) PreparedUpdate(_ context.Context) (domainupdate.Info, bool, error) {
	return stub.preparedInfo, stub.hasPreparedInfo, nil
}

func (stub *installerStub) ClearPreparedUpdate(_ context.Context) error {
	stub.clearPreparedInvoked = true
	return nil
}

func (stub *installerStub) PendingWhatsNew(_ context.Context) (domainupdate.WhatsNew, bool, error) {
	return stub.pendingWhatsNew, stub.hasPendingWhatsNew, nil
}

func (stub *installerStub) SeenWhatsNewVersion(_ context.Context) (string, error) {
	return stub.seenWhatsNewVersion, nil
}

func (stub *installerStub) MarkWhatsNewSeen(_ context.Context, version string) error {
	stub.markSeenVersion = version
	return nil
}

func (stub *catalogProviderStub) FetchCatalog(_ context.Context, _ softwareupdate.Request) (softwareupdate.Catalog, error) {
	if stub.err != nil {
		return softwareupdate.Catalog{}, stub.err
	}
	if len(stub.catalogs) > 0 {
		index := stub.fetchCount
		if index >= len(stub.catalogs) {
			index = len(stub.catalogs) - 1
		}
		stub.fetchCount++
		return stub.catalogs[index], nil
	}
	stub.fetchCount++
	return stub.catalog, nil
}

func newCatalogService(provider *catalogProviderStub) *softwareupdate.Service {
	return softwareupdate.NewService(softwareupdate.ServiceParams{
		CatalogProvider: provider,
	})
}

func buildCatalog(version string, downloadURL string) softwareupdate.Catalog {
	return softwareupdate.Catalog{
		App: &softwareupdate.AppRelease{
			Version: version,
			Asset: softwareupdate.Asset{
				Sources: []softwareupdate.DownloadSource{
					{
						Name:     "test",
						URL:      downloadURL,
						Priority: 10,
						Enabled:  true,
					},
				},
			},
		},
	}
}

func TestSafeCheckAlwaysRunsWhenUpdateAlreadyAvailableToday(t *testing.T) {
	t.Parallel()

	provider := &catalogProviderStub{
		catalog: buildCatalog("1.2.4", "https://example.com/download.zip"),
	}
	service := NewService(ServiceParams{Catalog: newCatalogService(provider)})
	now := time.Date(2026, time.April, 1, 15, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	service.now = func() time.Time { return now }
	service.state = domainupdate.Info{
		Kind:           domainupdate.KindApp,
		CurrentVersion: "1.2.3",
		LatestVersion:  "1.2.4",
		DownloadURL:    "https://example.com/download.zip",
		Status:         domainupdate.StatusAvailable,
		CheckedAt:      now.Add(-2 * time.Hour),
	}

	service.safeCheck(context.Background(), "1.2.3")

	if provider.fetchCount != 1 {
		t.Fatalf("expected auto-check to run, got %d fetches", provider.fetchCount)
	}
}

func TestManualCheckStillRunsWhenUpdateAlreadyAvailableToday(t *testing.T) {
	t.Parallel()

	provider := &catalogProviderStub{
		catalog: buildCatalog("1.2.4", "https://example.com/download.zip"),
	}
	service := NewService(ServiceParams{Catalog: newCatalogService(provider)})
	now := time.Date(2026, time.April, 1, 18, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	service.now = func() time.Time { return now }
	service.state = domainupdate.Info{
		Kind:           domainupdate.KindApp,
		CurrentVersion: "1.2.3",
		LatestVersion:  "1.2.4",
		DownloadURL:    "https://example.com/download.zip",
		Status:         domainupdate.StatusAvailable,
		CheckedAt:      now.Add(-1 * time.Hour),
	}

	if _, err := service.CheckForUpdate(context.Background(), "1.2.3"); err != nil {
		t.Fatalf("manual check failed: %v", err)
	}

	if provider.fetchCount != 1 {
		t.Fatalf("expected manual check to bypass auto-check skip, got %d fetches", provider.fetchCount)
	}
}

func TestCheckForUpdatePublishesDreamFMLiveCatalogUpdate(t *testing.T) {
	t.Parallel()

	previous := buildCatalog("1.2.3", "https://example.com/download.zip")
	previous.DreamFM.LiveChannel = softwareupdate.RemoteContentRef{
		SchemaVersion: 1,
		URL:           "https://updates.example.com/dream.fm/live/channel.json",
		Version:       "2026.04.26.1",
		MinAppVersion: "0.0.1",
		TTLSeconds:    300,
		Fallback:      "embedded",
	}
	next := buildCatalog("1.2.3", "https://example.com/download.zip")
	next.DreamFM.LiveChannel = softwareupdate.RemoteContentRef{
		SchemaVersion: 1,
		URL:           "https://updates.example.com/dream.fm/live/channel.json",
		Version:       "2026.04.26.2",
		MinAppVersion: "0.0.1",
		TTLSeconds:    300,
		Fallback:      "embedded",
	}
	provider := &catalogProviderStub{catalogs: []softwareupdate.Catalog{previous, next}}
	catalog := newCatalogService(provider)
	if _, err := catalog.RefreshCatalog(context.Background(), softwareupdate.Request{}); err != nil {
		t.Fatalf("prime catalog failed: %v", err)
	}

	bus := events.NewInMemoryBus()
	received := make([]events.Event, 0, 1)
	unsubscribe := bus.Subscribe(dreamFMLiveCatalogTopic, func(event events.Event) {
		received = append(received, event)
	})
	defer unsubscribe()

	service := NewService(ServiceParams{Catalog: catalog, Bus: bus})
	if _, err := service.CheckForUpdate(context.Background(), "1.2.3"); err != nil {
		t.Fatalf("check for update failed: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected one DreamFM live catalog event, got %d", len(received))
	}
	if received[0].Type != dreamFMLiveCatalogUpdatedType {
		t.Fatalf("unexpected event type: %s", received[0].Type)
	}
	payload, ok := received[0].Payload.(dreamFMLiveCatalogUpdatePayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", received[0].Payload)
	}
	if payload.Version != "2026.04.26.2" {
		t.Fatalf("unexpected DreamFM live catalog version: %s", payload.Version)
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("unexpected DreamFM live catalog schema version: %d", payload.SchemaVersion)
	}
	if payload.MinAppVersion != "0.0.1" {
		t.Fatalf("unexpected DreamFM live catalog min app version: %s", payload.MinAppVersion)
	}
	if payload.TTLSeconds != 300 {
		t.Fatalf("unexpected DreamFM live catalog ttl: %d", payload.TTLSeconds)
	}
	if payload.Fallback != "embedded" {
		t.Fatalf("unexpected DreamFM live catalog fallback: %s", payload.Fallback)
	}
	if payload.Fingerprint == "" {
		t.Fatal("expected DreamFM live catalog fingerprint")
	}
}

func TestCheckForUpdateSkipsDreamFMLiveCatalogEventOnInitialSnapshot(t *testing.T) {
	t.Parallel()

	catalog := buildCatalog("1.2.3", "https://example.com/download.zip")
	catalog.DreamFM.LiveChannel = softwareupdate.RemoteContentRef{
		SchemaVersion: 1,
		URL:           "https://updates.example.com/dream.fm/live/channel.json",
		Version:       "2026.04.26.1",
		TTLSeconds:    300,
		Fallback:      "embedded",
	}
	bus := events.NewInMemoryBus()
	received := make([]events.Event, 0, 1)
	unsubscribe := bus.Subscribe(dreamFMLiveCatalogTopic, func(event events.Event) {
		received = append(received, event)
	})
	defer unsubscribe()

	service := NewService(ServiceParams{
		Catalog: newCatalogService(&catalogProviderStub{catalog: catalog}),
		Bus:     bus,
	})
	if _, err := service.CheckForUpdate(context.Background(), "1.2.3"); err != nil {
		t.Fatalf("check for update failed: %v", err)
	}

	if len(received) != 0 {
		t.Fatalf("expected no initial DreamFM live catalog event, got %d", len(received))
	}
}

func TestCheckForUpdateReturnsNoUpdateWhenCurrentVersionIsNewerThanLatest(t *testing.T) {
	t.Parallel()

	provider := &catalogProviderStub{
		catalog: buildCatalog("1.3.0", "https://example.com/download.zip"),
	}
	service := NewService(ServiceParams{Catalog: newCatalogService(provider)})

	info, err := service.CheckForUpdate(context.Background(), "2.0.0")
	if err != nil {
		t.Fatalf("check for update failed: %v", err)
	}
	if info.Status != domainupdate.StatusNoUpdate {
		t.Fatalf("expected no_update status, got %q", info.Status)
	}
	if info.CurrentVersion != "2.0.0" {
		t.Fatalf("expected current version 2.0.0, got %q", info.CurrentVersion)
	}
	if info.LatestVersion != "1.3.0" {
		t.Fatalf("expected latest version 1.3.0, got %q", info.LatestVersion)
	}
}

func TestCheckForUpdateUsesInstallerDownloadURLSelector(t *testing.T) {
	t.Parallel()

	provider := &catalogProviderStub{
		catalog: buildCatalog("1.2.4", "https://example.com/xiadown-windows-x64-1.2.4-installer.exe"),
	}
	installer := &installerStub{
		selectedDownloadURLs: []string{
			"https://example.com/xiadown-windows-x64-1.2.4.zip",
		},
	}
	service := NewService(ServiceParams{Catalog: newCatalogService(provider), Installer: installer})

	info, err := service.CheckForUpdate(context.Background(), "1.2.3")
	if err != nil {
		t.Fatalf("check for update failed: %v", err)
	}
	if !installer.selectDownloadInvoked {
		t.Fatal("expected installer download selector to be called")
	}
	if info.DownloadURL != "https://example.com/xiadown-windows-x64-1.2.4.zip" {
		t.Fatalf("expected selected portable URL, got %q", info.DownloadURL)
	}

	urls := service.resolveDownloadURLsLocked()
	if len(urls) != 1 {
		t.Fatalf("expected selected portable URL, got %#v", urls)
	}
	if urls[0] != "https://example.com/xiadown-windows-x64-1.2.4.zip" {
		t.Fatalf("expected portable URL first, got %#v", urls)
	}
}

func TestDownloadUpdatePublishesErrorWhenInstallerUnavailable(t *testing.T) {
	t.Parallel()

	installerErr := errors.New("installer not implemented")
	service := NewService(ServiceParams{
		Downloader: &downloaderStub{path: "/tmp/xiadown-update.exe"},
		Installer:  &installerStub{installErr: installerErr},
	})
	service.state = domainupdate.Info{
		Kind:        domainupdate.KindApp,
		Status:      domainupdate.StatusAvailable,
		DownloadURL: "https://example.com/xiadown-update.exe",
	}

	info, err := service.DownloadUpdate(context.Background())
	if !errors.Is(err, installerErr) {
		t.Fatalf("expected installer error, got %v", err)
	}
	if info.Status != domainupdate.StatusError {
		t.Fatalf("expected error status, got %q", info.Status)
	}
	if info.Message != installerErr.Error() {
		t.Fatalf("expected error message %q, got %q", installerErr.Error(), info.Message)
	}
}

func TestDownloadUpdatePublishesErrorWhenChecksumMismatches(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "update-*.zip")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	if _, err := file.WriteString("hello"); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file failed: %v", err)
	}

	service := NewService(ServiceParams{
		Downloader: &downloaderStub{path: file.Name()},
		Installer:  &installerStub{},
	})
	service.state = domainupdate.Info{
		Kind:        domainupdate.KindApp,
		Status:      domainupdate.StatusAvailable,
		DownloadURL: "https://example.com/xiadown-update.zip",
	}
	service.downloadSHA256 = "sha256:deadbeef"

	info, err := service.DownloadUpdate(context.Background())
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if info.Status != domainupdate.StatusError {
		t.Fatalf("expected error status, got %q", info.Status)
	}
	if info.Message != "download checksum mismatch" {
		t.Fatalf("unexpected error message: %q", info.Message)
	}
}

func TestRestartToApplyInvokesInstallerAndResetsState(t *testing.T) {
	t.Parallel()

	installer := &installerStub{}
	service := NewService(ServiceParams{Installer: installer})
	service.state = domainupdate.Info{
		Kind:          domainupdate.KindApp,
		Status:        domainupdate.StatusReadyToRestart,
		LatestVersion: "1.2.4",
		Progress:      100,
	}

	info, err := service.RestartToApply(context.Background())
	if err != nil {
		t.Fatalf("restart to apply failed: %v", err)
	}
	if !installer.restarted {
		t.Fatal("expected installer restart hook to be called")
	}
	if info.Status != domainupdate.StatusIdle {
		t.Fatalf("expected idle status, got %q", info.Status)
	}
	if info.Progress != 0 {
		t.Fatalf("expected progress to reset, got %d", info.Progress)
	}
}

func TestRestartToApplyPublishesErrorWhenInstallerFails(t *testing.T) {
	t.Parallel()

	restartErr := errors.New("launch helper failed")
	installer := &installerStub{restartErr: restartErr}
	service := NewService(ServiceParams{Installer: installer})
	service.state = domainupdate.Info{
		Kind:   domainupdate.KindApp,
		Status: domainupdate.StatusReadyToRestart,
	}

	info, err := service.RestartToApply(context.Background())
	if !errors.Is(err, restartErr) {
		t.Fatalf("expected restart error, got %v", err)
	}
	if info.Status != domainupdate.StatusError {
		t.Fatalf("expected error status, got %q", info.Status)
	}
	if info.Message != restartErr.Error() {
		t.Fatalf("expected error message %q, got %q", restartErr.Error(), info.Message)
	}
}

func TestRestorePreparedUpdateRestoresReadyState(t *testing.T) {
	t.Parallel()

	installer := &installerStub{
		hasPreparedInfo: true,
		preparedInfo: domainupdate.Info{
			PreparedVersion:   "1.2.4",
			PreparedChangelog: "Bug fixes",
		},
	}
	service := NewService(ServiceParams{Installer: installer})
	service.SetCurrentVersion("1.2.3")

	info, err := service.RestorePreparedUpdate(context.Background())
	if err != nil {
		t.Fatalf("restore prepared update failed: %v", err)
	}
	if info.Status != domainupdate.StatusReadyToRestart {
		t.Fatalf("expected ready_to_restart status, got %q", info.Status)
	}
	if info.PreparedVersion != "1.2.4" {
		t.Fatalf("expected prepared version 1.2.4, got %q", info.PreparedVersion)
	}
	if info.PreparedChangelog != "Bug fixes" {
		t.Fatalf("expected prepared changelog to be restored, got %q", info.PreparedChangelog)
	}
}

func TestRestorePreparedUpdateClearsStalePreparedPlan(t *testing.T) {
	t.Parallel()

	installer := &installerStub{
		hasPreparedInfo: true,
		preparedInfo: domainupdate.Info{
			PreparedVersion: "1.2.3",
		},
	}
	service := NewService(ServiceParams{Installer: installer})
	service.SetCurrentVersion("1.2.3")

	info, err := service.RestorePreparedUpdate(context.Background())
	if err != nil {
		t.Fatalf("restore prepared update failed: %v", err)
	}
	if !installer.clearPreparedInvoked {
		t.Fatal("expected stale prepared update to be cleared")
	}
	if info.PreparedVersion != "" {
		t.Fatalf("expected prepared version to be cleared, got %q", info.PreparedVersion)
	}
}

func TestCheckForUpdateAutoPreparesLatestVersion(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "update-*.zip")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file failed: %v", err)
	}

	provider := &catalogProviderStub{
		catalog: buildCatalog("1.2.4", "https://example.com/download.zip"),
	}
	bus := events.NewInMemoryBus()
	readyCh := make(chan domainupdate.Info, 1)
	bus.Subscribe("update.status", func(event events.Event) {
		info, ok := event.Payload.(domainupdate.Info)
		if ok && info.Status == domainupdate.StatusReadyToRestart {
			select {
			case readyCh <- info:
			default:
			}
		}
	})

	service := NewService(ServiceParams{
		Catalog:    newCatalogService(provider),
		Downloader: &downloaderStub{path: file.Name()},
		Installer:  &installerStub{},
		Bus:        bus,
	})

	if _, err := service.CheckForUpdate(context.Background(), "1.2.3"); err != nil {
		t.Fatalf("check for update failed: %v", err)
	}

	select {
	case info := <-readyCh:
		if info.PreparedVersion != "1.2.4" {
			t.Fatalf("expected prepared version 1.2.4, got %q", info.PreparedVersion)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for auto prepare to reach ready_to_restart")
	}

	info := service.State()
	if info.Status != domainupdate.StatusReadyToRestart {
		t.Fatalf("expected auto prepare to reach ready_to_restart, got %q", info.Status)
	}
	if info.PreparedVersion != "1.2.4" {
		t.Fatalf("expected prepared version 1.2.4, got %q", info.PreparedVersion)
	}
}

func TestCheckForUpdatePreservesPreparedStateWhenRefreshFails(t *testing.T) {
	t.Parallel()

	provider := &catalogProviderStub{err: errors.New("manifest unavailable")}
	service := NewService(ServiceParams{Catalog: newCatalogService(provider)})
	service.state = domainupdate.Info{
		Kind:              domainupdate.KindApp,
		CurrentVersion:    "1.2.3",
		LatestVersion:     "1.2.4",
		PreparedVersion:   "1.2.4",
		PreparedChangelog: "Bug fixes",
		Status:            domainupdate.StatusReadyToRestart,
		Progress:          100,
	}

	info, err := service.CheckForUpdate(context.Background(), "1.2.3")
	if err == nil {
		t.Fatal("expected check to fail")
	}
	if info.Status != domainupdate.StatusReadyToRestart {
		t.Fatalf("expected ready_to_restart status, got %q", info.Status)
	}
	if info.PreparedVersion != "1.2.4" {
		t.Fatalf("expected prepared version to be preserved, got %q", info.PreparedVersion)
	}
}

func TestDownloadUpdateRestoresPreviousPreparedVersionWhenNewerPrepareFails(t *testing.T) {
	t.Parallel()

	installerErr := errors.New("prepare latest failed")
	service := NewService(ServiceParams{
		Downloader: &downloaderStub{path: "/tmp/xiadown-update.zip"},
		Installer:  &installerStub{installErr: installerErr},
	})
	service.state = domainupdate.Info{
		Kind:              domainupdate.KindApp,
		CurrentVersion:    "1.2.3",
		LatestVersion:     "1.2.5",
		PreparedVersion:   "1.2.4",
		PreparedChangelog: "Prepared 1.2.4",
		DownloadURL:       "https://example.com/xiadown-update.zip",
		Status:            domainupdate.StatusAvailable,
	}

	info, err := service.DownloadUpdate(context.Background())
	if !errors.Is(err, installerErr) {
		t.Fatalf("expected installer error, got %v", err)
	}
	if info.Status != domainupdate.StatusReadyToRestart {
		t.Fatalf("expected ready_to_restart status, got %q", info.Status)
	}
	if info.PreparedVersion != "1.2.4" {
		t.Fatalf("expected prepared version 1.2.4 to be preserved, got %q", info.PreparedVersion)
	}
	if info.LatestVersion != "1.2.5" {
		t.Fatalf("expected latest version 1.2.5 to stay visible, got %q", info.LatestVersion)
	}
}

func TestGetWhatsNewReturnsPendingPreparedNoticeForCurrentVersion(t *testing.T) {
	t.Parallel()

	installer := &installerStub{
		hasPendingWhatsNew: true,
		pendingWhatsNew: domainupdate.WhatsNew{
			Version:   "2.0.7",
			Changelog: "## Prepared update",
		},
		seenWhatsNewVersion: "2.0.6",
	}
	service := NewService(ServiceParams{Installer: installer})
	service.SetCurrentVersion("2.0.7")

	notice, err := service.GetWhatsNew(context.Background())
	if err != nil {
		t.Fatalf("GetWhatsNew failed: %v", err)
	}
	if notice.Version != "2.0.7" {
		t.Fatalf("expected version 2.0.7, got %q", notice.Version)
	}
	if notice.Changelog != "## Prepared update" {
		t.Fatalf("expected prepared changelog, got %q", notice.Changelog)
	}
}

func TestGetWhatsNewUsesManifestCurrentVersionReleaseNotes(t *testing.T) {
	t.Parallel()

	installer := &installerStub{seenWhatsNewVersion: "2.0.6"}
	catalog := newCatalogService(&catalogProviderStub{
		catalog: softwareupdate.Catalog{
			App: &softwareupdate.AppRelease{
				Version: "2.0.7",
				Notes:   "## Current release notes",
			},
		},
	})
	service := NewService(ServiceParams{
		Catalog:   catalog,
		Installer: installer,
	})
	service.SetCurrentVersion("2.0.7")

	notice, err := service.GetWhatsNew(context.Background())
	if err != nil {
		t.Fatalf("GetWhatsNew failed: %v", err)
	}
	if notice.Version != "2.0.7" {
		t.Fatalf("expected version 2.0.7, got %q", notice.Version)
	}
	if notice.Changelog != "## Current release notes" {
		t.Fatalf("expected current release notes, got %q", notice.Changelog)
	}
}

func TestDismissWhatsNewMarksSeenVersion(t *testing.T) {
	t.Parallel()

	installer := &installerStub{}
	service := NewService(ServiceParams{Installer: installer})

	if err := service.DismissWhatsNew(context.Background(), "2.0.7"); err != nil {
		t.Fatalf("DismissWhatsNew failed: %v", err)
	}
	if installer.markSeenVersion != "2.0.7" {
		t.Fatalf("expected seen version 2.0.7, got %q", installer.markSeenVersion)
	}
}
