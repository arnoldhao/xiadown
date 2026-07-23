package wails

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"xiadown/internal/application/listenplayback"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const PlaybackCoordinatorSnapshotEvent = "playback:snapshot"

type PlaybackSessionCloseRequest struct {
	SessionID string `json:"sessionId"`
}

type PlaybackCoordinatorSeekRequest struct {
	Seconds float64 `json:"seconds"`
}

type PlaybackCoordinatorVolumeRequest struct {
	Volume float64 `json:"volume"`
	Muted  bool    `json:"muted"`
}

// PlaybackCoordinatorHandler is the Wails boundary for global playback. It
// owns no UI state; every command returns the same snapshot shape that is also
// emitted to all application windows.
type PlaybackCoordinatorHandler struct {
	app         *application.App
	coordinator *listenplayback.PlaybackCoordinator

	shutdownOnce sync.Once
	unsubscribe  func()
}

func NewPlaybackCoordinatorHandler(app *application.App, coordinator *listenplayback.PlaybackCoordinator) *PlaybackCoordinatorHandler {
	handler := &PlaybackCoordinatorHandler{app: app, coordinator: coordinator}
	if coordinator != nil {
		handler.unsubscribe = coordinator.Subscribe(handler.publish)
	}
	return handler
}

func (handler *PlaybackCoordinatorHandler) ServiceName() string {
	return "PlaybackCoordinatorHandler"
}

func (handler *PlaybackCoordinatorHandler) StartPersistent(
	ctx context.Context,
	request listenplayback.PlaybackSessionRequest,
) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	request.Focus = listenplayback.PlaybackFocusPersistent
	request.PreviewResumePolicy = ""
	return handler.coordinator.StartSession(ctx, request)
}

func (handler *PlaybackCoordinatorHandler) StartTransientPreview(
	ctx context.Context,
	request listenplayback.PlaybackSessionRequest,
) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	request.Focus = listenplayback.PlaybackFocusTransientPreview
	if request.PreviewResumePolicy == "" {
		request.PreviewResumePolicy = listenplayback.PreviewResumeIfPreviouslyPlaying
	}
	return handler.coordinator.StartSession(ctx, request)
}

func (handler *PlaybackCoordinatorHandler) Close(
	ctx context.Context,
	request PlaybackSessionCloseRequest,
) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	sessionID := strings.TrimSpace(request.SessionID)
	if sessionID == "" {
		return handler.coordinator.Snapshot(), fmt.Errorf("playback session id is required")
	}
	return handler.coordinator.CloseSession(ctx, sessionID)
}

func (handler *PlaybackCoordinatorHandler) CloseSession(
	ctx context.Context,
	request PlaybackSessionCloseRequest,
) (listenplayback.PlaybackSnapshot, error) {
	return handler.Close(ctx, request)
}

func (handler *PlaybackCoordinatorHandler) Snapshot(context.Context) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	return handler.coordinator.Snapshot(), nil
}

func (handler *PlaybackCoordinatorHandler) Play(ctx context.Context) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	return handler.coordinator.Play(ctx)
}

func (handler *PlaybackCoordinatorHandler) Pause(ctx context.Context) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	return handler.coordinator.Pause(ctx)
}

func (handler *PlaybackCoordinatorHandler) Stop(ctx context.Context) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	return handler.coordinator.Stop(ctx)
}

func (handler *PlaybackCoordinatorHandler) Seek(
	ctx context.Context,
	request PlaybackCoordinatorSeekRequest,
) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	return handler.coordinator.Seek(ctx, request.Seconds)
}

func (handler *PlaybackCoordinatorHandler) SetVolume(
	ctx context.Context,
	request PlaybackCoordinatorVolumeRequest,
) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	return handler.coordinator.SetVolume(ctx, request.Volume, request.Muted)
}

func (handler *PlaybackCoordinatorHandler) Next(ctx context.Context) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	if err := handler.coordinator.Next(ctx); err != nil {
		return handler.coordinator.Snapshot(), err
	}
	return handler.coordinator.Snapshot(), nil
}

func (handler *PlaybackCoordinatorHandler) Previous(ctx context.Context) (listenplayback.PlaybackSnapshot, error) {
	if err := handler.ensureCoordinator(); err != nil {
		return listenplayback.PlaybackSnapshot{}, err
	}
	if err := handler.coordinator.Previous(ctx); err != nil {
		return handler.coordinator.Snapshot(), err
	}
	return handler.coordinator.Snapshot(), nil
}

func ShutdownPlaybackCoordinatorHandler(handler *PlaybackCoordinatorHandler) {
	if handler == nil {
		return
	}
	handler.shutdownOnce.Do(func() {
		if handler.unsubscribe != nil {
			handler.unsubscribe()
		}
	})
}

func (handler *PlaybackCoordinatorHandler) publish(snapshot listenplayback.PlaybackSnapshot) {
	if handler == nil || handler.app == nil {
		return
	}
	handler.app.Event.Emit(PlaybackCoordinatorSnapshotEvent, snapshot)
}

func (handler *PlaybackCoordinatorHandler) ensureCoordinator() error {
	if handler == nil || handler.coordinator == nil {
		return fmt.Errorf("playback coordinator unavailable")
	}
	return nil
}
