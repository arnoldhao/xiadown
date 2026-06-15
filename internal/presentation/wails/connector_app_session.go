package wails

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	appsessionidentity "xiadown/internal/application/appsessions"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

const connectorAppSessionBlankURL = "about:blank"

type NativeAppSessionProvider struct {
	app *application.App
}

type connectorAppSessionWindow struct {
	mu         sync.Mutex
	window     *application.WebviewWindow
	domains    []string
	done       chan struct{}
	last       []appcookies.Record
	closing    bool
	allowClose bool
	completed  bool
	closeOnce  sync.Once
}

func NewNativeAppSessionProvider(app *application.App) *NativeAppSessionProvider {
	return &NativeAppSessionProvider{app: app}
}

func (provider *NativeAppSessionProvider) AppSessionsSupported() bool {
	return connectorAppSessionNativeSupported()
}

func (provider *NativeAppSessionProvider) StartAppSession(ctx context.Context, request appsessionsservice.AppSessionStartRequest) (appsessionsservice.AppSessionBrowser, error) {
	if provider == nil || provider.app == nil {
		return nil, appsessions.ErrUnsupported
	}
	if !connectorAppSessionNativeSupported() {
		return nil, appsessions.ErrUnsupported
	}
	targetURL := strings.TrimSpace(request.TargetURL)
	if targetURL == "" {
		targetURL = "https://www.youtube.com/"
	}
	title := "Connect " + siteAppSessionWindowTitle(request.SiteKey)
	handle := &connectorAppSessionWindow{
		domains: append([]string(nil), request.Domains...),
		done:    make(chan struct{}),
	}
	window := provider.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "site-app-session-" + strings.TrimSpace(request.SessionID),
		Title:     title,
		Width:     560,
		Height:    720,
		MinWidth:  420,
		MinHeight: 520,
		URL:       connectorAppSessionBlankURL,
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{
				FullscreenEnabled: application.Disabled,
			},
		},
	})
	handle.window = window
	prepareConnectorAppSessionNativeWindow(window, targetURL, request.SiteKey, request.InitialCookies, request.Domains)
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		handle.handleWindowClosing(event)
	})
	loadConnectorAppSessionNativeURL(window, targetURL)
	window.Show()
	window.Focus()
	return handle, nil
}

func (provider *NativeAppSessionProvider) LoadAppSessionCookies(_ context.Context, siteKey string) ([]appcookies.Record, error) {
	records, err := loadSiteAppSessionStoredCookies(siteKey)
	if err != nil {
		return nil, appsessions.ErrNoCookies
	}
	return records, nil
}

func (provider *NativeAppSessionProvider) SaveAppSessionCookies(_ context.Context, siteKey string, records []appcookies.Record) error {
	if err := saveSiteAppSessionStoredCookies(siteKey, records); err != nil {
		if errors.Is(err, appsessions.ErrUnsupported) {
			return appsessions.ErrUnsupported
		}
		if errors.Is(err, appsessions.ErrNoCookies) {
			return appsessions.ErrNoCookies
		}
		return err
	}
	return nil
}

func (provider *NativeAppSessionProvider) ClearAppSession(ctx context.Context, siteKey string, domains []string) error {
	if err := clearSiteAppSessionStoredCookies(siteKey, domains); err != nil {
		if errors.Is(err, appsessions.ErrUnsupported) {
			return appsessions.ErrUnsupported
		}
		if errors.Is(err, appsessions.ErrNoCookies) {
			return appsessions.ErrNoCookies
		}
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := clearConnectorAppSessionNativeRuntimeData(ctx, provider.app, siteKey, domains); err != nil &&
		!errors.Is(err, appsessions.ErrUnsupported) &&
		!errors.Is(err, appsessions.ErrNoCookies) {
		log.Printf("app sessions: clear native runtime data failed site=%s error=%v", siteKey, err)
	}
	return nil
}

func (session *connectorAppSessionWindow) Cookies(ctx context.Context) ([]appcookies.Record, error) {
	if session == nil {
		return nil, appsessions.ErrSessionGone
	}
	session.mu.Lock()
	window := session.window
	cached := append([]appcookies.Record(nil), session.last...)
	completed := session.completed
	session.mu.Unlock()
	if completed && len(cached) > 0 {
		return cached, nil
	}
	if window == nil {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, appsessions.ErrSessionDead
	}
	records, err := readConnectorAppSessionNativeWindowCookies(ctx, window, session.domains)
	if err != nil {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, err
	}
	session.mu.Lock()
	session.last = append([]appcookies.Record(nil), records...)
	session.mu.Unlock()
	return records, nil
}

func (session *connectorAppSessionWindow) Done() <-chan struct{} {
	if session == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return session.done
}

func (session *connectorAppSessionWindow) Close() {
	if session == nil {
		return
	}
	session.closeOnce.Do(func() {
		captureBeforeClose := connectorAppSessionCaptureBeforeClose()
		var completeNow bool
		session.mu.Lock()
		window := session.window
		if session.completed {
			window = nil
		} else {
			session.closing = true
			session.allowClose = true
			if !captureBeforeClose || window == nil {
				completeNow = session.completeCloseLocked(window)
			}
		}
		session.mu.Unlock()
		if window != nil {
			window.Close()
		}
		if completeNow {
			session.closeDone()
		} else if captureBeforeClose && window != nil {
			session.completeClose(window)
		}
	})
}

func (session *connectorAppSessionWindow) handleWindowClosing(event *application.WindowEvent) {
	if session == nil {
		return
	}
	if !connectorAppSessionCaptureBeforeClose() {
		session.mu.Lock()
		completeNow := session.completeCloseLocked(session.window)
		session.mu.Unlock()
		if completeNow {
			session.closeDone()
		}
		return
	}
	session.mu.Lock()
	if session.allowClose {
		completeNow := session.completeCloseLocked(session.window)
		session.mu.Unlock()
		if completeNow {
			session.closeDone()
		}
		return
	}
	if session.closing {
		session.mu.Unlock()
		event.Cancel()
		return
	}
	session.closing = true
	session.mu.Unlock()

	event.Cancel()
	go session.captureCookiesAndClose()
}

func (session *connectorAppSessionWindow) captureCookiesAndClose() {
	if session == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	records, err := session.Cookies(ctx)
	cancel()
	if err == nil && len(records) > 0 {
		session.mu.Lock()
		session.last = append([]appcookies.Record(nil), records...)
		session.mu.Unlock()
		log.Printf("app sessions: captured cookies before native close count=%d", len(records))
	} else if err != nil {
		log.Printf("app sessions: capture cookies before native close failed: %v", err)
	} else {
		log.Printf("app sessions: capture cookies before native close found no cookies")
	}

	session.mu.Lock()
	window := session.window
	if session.completed {
		session.mu.Unlock()
		return
	}
	session.allowClose = true
	session.mu.Unlock()

	if window != nil {
		window.Close()
		return
	}

	session.completeClose(nil)
}

func (session *connectorAppSessionWindow) completeClose(window *application.WebviewWindow) {
	if session == nil {
		return
	}
	session.mu.Lock()
	completeNow := session.completeCloseLocked(window)
	session.mu.Unlock()
	if completeNow {
		session.closeDone()
	}
}

func (session *connectorAppSessionWindow) completeCloseLocked(window *application.WebviewWindow) bool {
	if session == nil {
		return false
	}
	if window == nil || session.window == window {
		session.window = nil
	}
	session.closing = true
	session.allowClose = true
	if session.completed {
		return false
	}
	session.completed = true
	return true
}

func (session *connectorAppSessionWindow) closeDone() {
	close(session.done)
}

func siteAppSessionAccount(siteKey string) string {
	return fmt.Sprintf("site-app-session:%s", strings.TrimSpace(siteKey))
}

func appSessionWebViewUserAgent(siteKey string) string {
	return appsessionidentity.WebViewUserAgent(siteKey)
}

func siteAppSessionWindowTitle(siteKey string) string {
	switch strings.TrimSpace(siteKey) {
	case "youtube":
		return "YouTube"
	case "bilibili":
		return "Bilibili"
	case "tiktok":
		return "TikTok"
	case "instagram":
		return "Instagram"
	case "x":
		return "X"
	case "facebook":
		return "Facebook"
	case "vimeo":
		return "Vimeo"
	case "twitch":
		return "Twitch"
	case "niconico":
		return "Niconico"
	default:
		return "App Session"
	}
}
