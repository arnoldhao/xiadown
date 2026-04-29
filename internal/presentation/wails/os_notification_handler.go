package wails

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"go.uber.org/zap"
)

type OSNotificationHandler struct {
	notifications *notifications.NotificationService
	app           *application.App
	httpClient    osNotificationHTTPClientProvider

	mu         sync.Mutex
	started    bool
	startupErr error
}

type osNotificationHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type OSNotificationRequest struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Subtitle string         `json:"subtitle,omitempty"`
	Body     string         `json:"body,omitempty"`
	IconURL  string         `json:"iconUrl,omitempty"`
	ImageURL string         `json:"imageUrl,omitempty"`
	Source   string         `json:"source,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

func NewOSNotificationHandler(notificationService *notifications.NotificationService, app *application.App) *OSNotificationHandler {
	return NewOSNotificationHandlerWithHTTPClientProvider(notificationService, app, nil)
}

func NewOSNotificationHandlerWithHTTPClientProvider(notificationService *notifications.NotificationService, app *application.App, httpClient osNotificationHTTPClientProvider) *OSNotificationHandler {
	if notificationService == nil {
		notificationService = notifications.New()
	}
	return &OSNotificationHandler{notifications: notificationService, app: app, httpClient: httpClient}
}

func (handler *OSNotificationHandler) ServiceName() string {
	return "OSNotificationHandler"
}

func (handler *OSNotificationHandler) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if handler == nil || handler.notifications == nil {
		return nil
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if handler.started {
		return nil
	}
	releaseStartupThread, err := prepareOSNotificationServiceStartup()
	if err != nil {
		zap.L().Warn("prepare os notification startup", zap.Error(err))
	}
	defer releaseStartupThread()
	if err := handler.notifications.ServiceStartup(ctx, options); err != nil {
		handler.startupErr = err
		zap.L().Warn("os notifications unavailable", zap.Error(err))
		return nil
	}
	handler.started = true
	handler.startupErr = nil
	return nil
}

func (handler *OSNotificationHandler) ServiceShutdown() error {
	if handler == nil || handler.notifications == nil {
		return nil
	}

	handler.mu.Lock()
	started := handler.started
	handler.started = false
	handler.mu.Unlock()
	if !started {
		return nil
	}
	if err := handler.notifications.ServiceShutdown(); err != nil {
		zap.L().Warn("shutdown os notifications", zap.Error(err))
	}
	return nil
}

func (handler *OSNotificationHandler) IsAppActive() bool {
	if handler == nil || handler.app == nil {
		return false
	}
	for _, window := range handler.app.Window.GetAll() {
		if window != nil && window.IsFocused() {
			return true
		}
	}
	return false
}

func (handler *OSNotificationHandler) Send(ctx context.Context, request OSNotificationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if handler == nil || handler.notifications == nil {
		return fmt.Errorf("os notifications unavailable")
	}
	if handler.IsAppActive() {
		return nil
	}

	handler.mu.Lock()
	started := handler.started
	startupErr := handler.startupErr
	handler.mu.Unlock()
	if !started {
		if startupErr != nil {
			return fmt.Errorf("os notifications unavailable: %w", startupErr)
		}
		return fmt.Errorf("os notifications unavailable")
	}

	options, err := normalizeOSNotificationRequest(request)
	if err != nil {
		return err
	}

	authorized, err := handler.notifications.CheckNotificationAuthorization()
	if err != nil {
		zap.L().Warn("check notification authorization failed", zap.Error(err))
	}
	if !authorized {
		authorized, err = handler.notifications.RequestNotificationAuthorization()
		if err != nil {
			return fmt.Errorf("request notification authorization: %w", err)
		}
		if !authorized {
			return fmt.Errorf("notification authorization denied")
		}
	}

	if sent, err := sendRichOSNotification(ctx, options, strings.TrimSpace(request.ImageURL), handler.httpClient); sent {
		return nil
	} else if err != nil {
		zap.L().Warn("rich os notification failed", zap.Error(err))
	}

	if err := handler.notifications.SendNotification(options); err != nil {
		return fmt.Errorf("send notification: %w", err)
	}
	return nil
}

func normalizeOSNotificationRequest(request OSNotificationRequest) (notifications.NotificationOptions, error) {
	id := strings.TrimSpace(request.ID)
	if id == "" {
		id = fmt.Sprintf("os_%d", time.Now().UnixNano())
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		return notifications.NotificationOptions{}, fmt.Errorf("notification title cannot be empty")
	}
	subtitle := strings.TrimSpace(request.Subtitle)
	body := strings.TrimSpace(request.Body)
	if body == "" {
		body = subtitle
	}

	data := make(map[string]any, len(request.Data)+3)
	for key, value := range request.Data {
		if strings.TrimSpace(key) == "" {
			continue
		}
		data[key] = value
	}
	if source := strings.TrimSpace(request.Source); source != "" {
		data["source"] = source
	}
	if iconURL := strings.TrimSpace(request.IconURL); iconURL != "" {
		data["iconUrl"] = iconURL
	}
	if imageURL := strings.TrimSpace(request.ImageURL); imageURL != "" {
		data["imageUrl"] = imageURL
	}
	if len(data) == 0 {
		data = nil
	}

	return notifications.NotificationOptions{
		ID:       id,
		Title:    title,
		Subtitle: subtitle,
		Body:     body,
		Data:     data,
	}, nil
}
