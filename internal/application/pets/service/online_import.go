package service

import (
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/pets/dto"
)

const (
	onlinePetImportStateRunning   = "running"
	onlinePetImportStateCompleted = "completed"
	onlinePetImportStateFailed    = "failed"

	onlinePetBrowserStatusNotOpen       = "not_open"
	onlinePetBrowserStatusOpening       = "opening"
	onlinePetBrowserStatusOpen          = "open"
	onlinePetBrowserStatusBrowserClosed = "browser_closed"
	onlinePetBrowserStatusCompleted     = "completed"
	onlinePetBrowserStatusFailed        = "failed"

	onlinePetImportSiteCodexPetsNet = "codex-pets-net"
	onlinePetImportSiteCodexpetXYZ  = "codexpet-xyz"
	onlinePetImportSitePetdexDev    = "petdex-dev"
)

type onlinePetImportSession struct {
	ID                 string
	SiteID             string
	SiteLabel          string
	URL                string
	State              string
	BrowserStatus      string
	Runtime            *browsercdp.Runtime
	TabCancel          context.CancelFunc
	TargetID           string
	DownloadDir        string
	UserDataDir        string
	ImportedPets       []dto.Pet
	ErrorCode          string
	Error              string
	UpdatedAt          time.Time
	processedDownloads map[string]struct{}
	cleanupOnce        sync.Once
}

func (service *Service) StartOnlinePetImport(ctx context.Context, request dto.StartOnlinePetImportRequest) (dto.OnlinePetImportSession, error) {
	siteID := normalizeOnlinePetImportSiteID(request.SiteID)
	siteLabel, siteURL, err := onlinePetImportSite(siteID)
	if err != nil {
		return dto.OnlinePetImportSession{}, err
	}

	downloadDir, err := os.MkdirTemp("", "xiadown-pet-download-*")
	if err != nil {
		return dto.OnlinePetImportSession{}, err
	}
	userDataDir, err := os.MkdirTemp("", "xiadown-pet-browser-*")
	if err != nil {
		_ = os.RemoveAll(downloadDir)
		return dto.OnlinePetImportSession{}, err
	}

	runtime, err := browsercdp.Start(context.Background(), browsercdp.LaunchOptions{
		PreferredBrowser: service.preferredBrowser(ctx),
		Headless:         false,
		UserDataDir:      userDataDir,
		ExtraArgs:        []string{"--disable-popup-blocking"},
	})
	if err != nil {
		_ = os.RemoveAll(downloadDir)
		_ = os.RemoveAll(userDataDir)
		return dto.OnlinePetImportSession{}, err
	}

	tabCtx, tabCancel, targetID, err := browsercdp.AttachOrCreatePageTarget(runtime, 5*time.Second)
	if err != nil {
		runtime.Stop()
		_ = os.RemoveAll(downloadDir)
		_ = os.RemoveAll(userDataDir)
		return dto.OnlinePetImportSession{}, err
	}

	if err := allowPetDownloads(runtime, downloadDir); err != nil {
		tabCancel()
		runtime.Stop()
		_ = os.RemoveAll(downloadDir)
		_ = os.RemoveAll(userDataDir)
		return dto.OnlinePetImportSession{}, err
	}

	session := &onlinePetImportSession{
		ID:                 uuid.NewString(),
		SiteID:             siteID,
		SiteLabel:          siteLabel,
		URL:                siteURL,
		State:              onlinePetImportStateRunning,
		BrowserStatus:      onlinePetBrowserStatusOpen,
		Runtime:            runtime,
		TabCancel:          tabCancel,
		TargetID:           strings.TrimSpace(targetID),
		DownloadDir:        downloadDir,
		UserDataDir:        userDataDir,
		UpdatedAt:          service.now().UTC(),
		processedDownloads: make(map[string]struct{}),
	}
	service.putOnlinePetImportSession(session)
	service.listenOnlinePetDownloads(session.ID, runtime, downloadDir)
	service.watchOnlinePetImportTarget(session.ID, runtime, session.TargetID)
	service.startOnlinePetImportMonitor(session.ID)
	go service.navigateOnlinePetImportSession(session.ID, tabCtx, siteURL)
	return service.snapshotOnlinePetImportSession(session.ID), nil
}

func (service *Service) preferredBrowser(ctx context.Context) string {
	if service == nil || service.settingsReader == nil {
		return ""
	}
	current, err := service.settingsReader.GetSettings(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(current.SniffBrowser)
}

func (service *Service) GetOnlinePetImportSession(_ context.Context, request dto.GetOnlinePetImportSessionRequest) (dto.OnlinePetImportSession, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return dto.OnlinePetImportSession{}, newPetError(petErrorCodeOnlineSessionRequired, "pet import session is required")
	}
	service.mu.Lock()
	_, ok := service.importSessions[sessionID]
	service.mu.Unlock()
	if !ok {
		return dto.OnlinePetImportSession{}, newPetErrorf(petErrorCodeOnlineSessionNotFound, "pet import session %q not found", sessionID)
	}
	return service.snapshotOnlinePetImportSession(sessionID), nil
}

func (service *Service) FinishOnlinePetImportSession(_ context.Context, request dto.FinishOnlinePetImportSessionRequest) (dto.OnlinePetImportSession, error) {
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return dto.OnlinePetImportSession{}, newPetError(petErrorCodeOnlineSessionRequired, "pet import session is required")
	}
	snapshot := service.snapshotOnlinePetImportSession(sessionID)
	if strings.TrimSpace(snapshot.SessionID) == "" {
		return dto.OnlinePetImportSession{}, newPetErrorf(petErrorCodeOnlineSessionNotFound, "pet import session %q not found", sessionID)
	}
	service.finishOnlinePetImportSession(sessionID, onlinePetBrowserStatusCompleted)
	snapshot.State = onlinePetImportStateCompleted
	snapshot.BrowserStatus = onlinePetBrowserStatusCompleted
	return snapshot, nil
}

func (service *Service) putOnlinePetImportSession(session *onlinePetImportSession) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.importSessions == nil {
		service.importSessions = make(map[string]*onlinePetImportSession)
	}
	service.importSessions[session.ID] = session
}

func (service *Service) snapshotOnlinePetImportSession(sessionID string) dto.OnlinePetImportSession {
	service.mu.Lock()
	defer service.mu.Unlock()
	session, ok := service.importSessions[strings.TrimSpace(sessionID)]
	if !ok || session == nil {
		return dto.OnlinePetImportSession{}
	}
	return snapshotOnlinePetImportSessionLocked(session)
}

func snapshotOnlinePetImportSessionLocked(session *onlinePetImportSession) dto.OnlinePetImportSession {
	importedPets := append([]dto.Pet(nil), session.ImportedPets...)
	updatedAt := ""
	if !session.UpdatedAt.IsZero() {
		updatedAt = session.UpdatedAt.Format(time.RFC3339Nano)
	}
	return dto.OnlinePetImportSession{
		SessionID:     session.ID,
		SiteID:        session.SiteID,
		SiteLabel:     session.SiteLabel,
		URL:           session.URL,
		State:         session.State,
		BrowserStatus: session.BrowserStatus,
		ImportedPets:  importedPets,
		ErrorCode:     session.ErrorCode,
		Error:         session.Error,
		UpdatedAt:     updatedAt,
	}
}

func (service *Service) updateOnlinePetImportSession(sessionID string, update func(*onlinePetImportSession)) {
	service.mu.Lock()
	defer service.mu.Unlock()
	session := service.importSessions[strings.TrimSpace(sessionID)]
	if session == nil {
		return
	}
	update(session)
	session.UpdatedAt = service.now().UTC()
}

func (service *Service) finishOnlinePetImportSession(sessionID string, finalBrowserStatus string) {
	service.mu.Lock()
	session := service.importSessions[strings.TrimSpace(sessionID)]
	if session != nil {
		session.State = onlinePetImportStateCompleted
		session.BrowserStatus = strings.TrimSpace(finalBrowserStatus)
		session.UpdatedAt = service.now().UTC()
		delete(service.importSessions, session.ID)
	}
	service.mu.Unlock()
	if session == nil {
		return
	}
	cleanupOnlinePetImportSessionResources(session)
}

func (service *Service) ShutdownOnlinePetImportSessions() {
	service.mu.Lock()
	sessions := make([]*onlinePetImportSession, 0, len(service.importSessions))
	for sessionID, session := range service.importSessions {
		if session == nil {
			delete(service.importSessions, sessionID)
			continue
		}
		session.State = onlinePetImportStateCompleted
		session.BrowserStatus = onlinePetBrowserStatusBrowserClosed
		session.UpdatedAt = service.now().UTC()
		sessions = append(sessions, session)
		delete(service.importSessions, sessionID)
	}
	service.mu.Unlock()
	for _, session := range sessions {
		cleanupOnlinePetImportSessionResources(session)
	}
}

func (service *Service) markOnlinePetImportBrowserClosed(sessionID string) {
	service.mu.Lock()
	session := service.importSessions[strings.TrimSpace(sessionID)]
	if session != nil {
		session.BrowserStatus = onlinePetBrowserStatusBrowserClosed
		session.UpdatedAt = service.now().UTC()
	}
	service.mu.Unlock()
	if session != nil {
		cleanupOnlinePetImportSessionResources(session)
	}
}

func cleanupOnlinePetImportSessionResources(session *onlinePetImportSession) {
	if session == nil {
		return
	}
	session.cleanupOnce.Do(func() {
		if session.Runtime != nil {
			session.Runtime.Stop()
		}
		cancelOnlinePetImportContextAsync(session.ID, session.TabCancel)
		if strings.TrimSpace(session.DownloadDir) != "" {
			_ = os.RemoveAll(session.DownloadDir)
		}
		if strings.TrimSpace(session.UserDataDir) != "" {
			_ = os.RemoveAll(session.UserDataDir)
		}
	})
}

func cancelOnlinePetImportContextAsync(_ string, cancel context.CancelFunc) {
	if cancel == nil {
		return
	}
	go func() {
		defer func() {
			_ = recover()
		}()
		cancel()
	}()
}

func (service *Service) navigateOnlinePetImportSession(sessionID string, tabCtx context.Context, siteURL string) {
	if tabCtx == nil {
		return
	}
	if err := chromedp.Run(tabCtx, chromedp.Navigate(siteURL)); err != nil {
		service.failOnlinePetImportSession(sessionID, err)
	}
}

func (service *Service) failOnlinePetImportSession(sessionID string, err error) {
	if err == nil {
		return
	}
	service.mu.Lock()
	session := service.importSessions[strings.TrimSpace(sessionID)]
	if session != nil {
		session.State = onlinePetImportStateFailed
		session.BrowserStatus = onlinePetBrowserStatusFailed
		session.ErrorCode = ""
		session.Error = err.Error()
		session.UpdatedAt = service.now().UTC()
	}
	service.mu.Unlock()
	if session != nil {
		cleanupOnlinePetImportSessionResources(session)
	}
}

func (service *Service) listenOnlinePetDownloads(sessionID string, runtime *browsercdp.Runtime, downloadDir string) {
	suggested := make(map[string]string)
	var downloadMu sync.Mutex
	chromedp.ListenBrowser(runtime.BrowserContext(), func(event any) {
		switch value := event.(type) {
		case *browser.EventDownloadWillBegin:
			downloadMu.Lock()
			suggested[value.GUID] = value.SuggestedFilename
			downloadMu.Unlock()
		case *browser.EventDownloadProgress:
			switch value.State {
			case browser.DownloadProgressStateCompleted:
				path := strings.TrimSpace(value.FilePath)
				downloadMu.Lock()
				if path == "" {
					path = filepath.Join(downloadDir, suggested[value.GUID])
				}
				downloadMu.Unlock()
				if strings.EqualFold(filepath.Ext(path), ".zip") {
					service.importDownloadedPet(sessionID, path)
				}
			case browser.DownloadProgressStateCanceled:
				service.updateOnlinePetImportSession(sessionID, func(session *onlinePetImportSession) {
					session.ErrorCode = petErrorCodeOnlineDownloadCanceled
					session.Error = "pet download was canceled"
				})
			}
		}
	})
}

func (service *Service) watchOnlinePetImportTarget(sessionID string, runtime *browsercdp.Runtime, targetID string) {
	targetID = strings.TrimSpace(targetID)
	if service == nil || runtime == nil || runtime.BrowserContext() == nil || targetID == "" {
		return
	}
	chromedp.ListenBrowser(runtime.BrowserContext(), func(event any) {
		switch current := event.(type) {
		case *targetpkg.EventTargetDestroyed:
			if strings.TrimSpace(string(current.TargetID)) == targetID {
				service.markOnlinePetImportBrowserClosed(sessionID)
			}
		case *targetpkg.EventTargetCrashed:
			if strings.TrimSpace(string(current.TargetID)) == targetID {
				service.markOnlinePetImportBrowserClosed(sessionID)
			}
		}
	})
}

func (service *Service) importDownloadedPet(sessionID string, zipPath string) {
	zipPath = strings.TrimSpace(zipPath)
	if zipPath == "" {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session := service.importSessions[strings.TrimSpace(sessionID)]
	if session == nil {
		return
	}
	if _, ok := session.processedDownloads[zipPath]; ok {
		return
	}
	session.processedDownloads[zipPath] = struct{}{}

	inspection, err := service.inspectZipLocked(zipPath)
	if err != nil {
		session.ErrorCode, session.Error = petErrorDetails(err)
		session.UpdatedAt = service.now().UTC()
		return
	}
	if inspection.status != petStatusReady {
		session.ErrorCode = inspection.validationCode
		session.Error = inspection.validationMessage
		session.UpdatedAt = service.now().UTC()
		return
	}
	pet, err := service.storeImportedPetLocked(inspection, firstNonEmpty(session.URL, session.SiteLabel))
	if err != nil {
		session.ErrorCode, session.Error = petErrorDetails(err)
		session.UpdatedAt = service.now().UTC()
		return
	}
	service.saveMetadataLocked(context.Background(), pet)
	session.ImportedPets = append(session.ImportedPets, pet)
	session.ErrorCode = ""
	session.Error = ""
	session.UpdatedAt = service.now().UTC()
}

func resolvePetOriginHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return ""
	}
	if host := normalizeOriginHost(parsed.Host); host != "" {
		return host
	}
	if parsed.Opaque != "" {
		nested, nestedErr := url.Parse(parsed.Opaque)
		if nestedErr == nil && nested != nil {
			return normalizeOriginHost(nested.Host)
		}
	}
	return ""
}

func normalizeOriginHost(host string) string {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return ""
	}
	if splitHost, _, err := net.SplitHostPort(trimmed); err == nil {
		trimmed = splitHost
	}
	return strings.TrimPrefix(strings.ToLower(trimmed), "www.")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (service *Service) startOnlinePetImportMonitor(sessionID string) {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			service.mu.Lock()
			session := service.importSessions[strings.TrimSpace(sessionID)]
			if session == nil {
				service.mu.Unlock()
				return
			}
			state := session.State
			runtime := session.Runtime
			service.mu.Unlock()
			if state != onlinePetImportStateRunning {
				return
			}
			if runtime == nil || !runtime.Status().Ready {
				service.markOnlinePetImportBrowserClosed(sessionID)
				return
			}
			select {
			case <-ticker.C:
			case <-runtime.BrowserContext().Done():
				service.markOnlinePetImportBrowserClosed(sessionID)
				return
			}
		}
	}()
}

func allowPetDownloads(runtime *browsercdp.Runtime, downloadDir string) error {
	execCtx, execCancel, err := browsercdp.RuntimeBrowserExecutorContext(runtime, 10*time.Second)
	if err != nil {
		return err
	}
	defer execCancel()
	return browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllow).
		WithDownloadPath(downloadDir).
		WithEventsEnabled(true).
		Do(execCtx)
}

func normalizeOnlinePetImportSiteID(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.TrimRight(strings.ToLower(trimmed), "/")
	host := resolvePetOriginHost(trimmed)
	switch {
	case lower == "", lower == onlinePetImportSiteCodexPetsNet, lower == "codex-pets.net", host == "codex-pets.net":
		return onlinePetImportSiteCodexPetsNet
	case lower == onlinePetImportSiteCodexpetXYZ, lower == "codexpet.xyz", host == "codexpet.xyz":
		return onlinePetImportSiteCodexpetXYZ
	case lower == onlinePetImportSitePetdexDev, lower == "petdex.dev", host == "petdex.dev":
		return onlinePetImportSitePetdexDev
	default:
		return trimmed
	}
}

func onlinePetImportSite(siteID string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(siteID)) {
	case onlinePetImportSiteCodexPetsNet:
		return "codex-pets.net", "https://codex-pets.net", nil
	case onlinePetImportSiteCodexpetXYZ:
		return "codexpet.xyz", "https://codexpet.xyz/", nil
	case onlinePetImportSitePetdexDev:
		return "petdex.dev", "https://petdex.dev", nil
	default:
		return "", "", newPetErrorf(petErrorCodeOnlineUnsupportedSite, "unsupported pet import site %q", siteID)
	}
}
