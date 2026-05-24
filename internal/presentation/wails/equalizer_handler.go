package wails

import (
	"context"
	"fmt"

	"xiadown/internal/application/equalizer"
)

type EqualizerHandler struct {
	service *equalizer.Service
}

func NewEqualizerHandler(service *equalizer.Service) *EqualizerHandler {
	return &EqualizerHandler{service: service}
}

func (handler *EqualizerHandler) ServiceName() string {
	return "EqualizerHandler"
}

func (handler *EqualizerHandler) Snapshot(_ context.Context) (equalizer.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return equalizer.Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	return handler.service.Snapshot(), nil
}

func (handler *EqualizerHandler) SetEnabled(_ context.Context, enabled bool) (equalizer.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return equalizer.Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	return handler.service.SetEnabled(enabled)
}

func (handler *EqualizerHandler) ApplyPreset(_ context.Context, presetID string) (equalizer.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return equalizer.Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	return handler.service.ApplyPreset(presetID)
}

func (handler *EqualizerHandler) SetBandGain(_ context.Context, index int, gainDB float64) (equalizer.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return equalizer.Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	return handler.service.SetBandGain(index, gainDB)
}

func (handler *EqualizerHandler) SetPreamp(_ context.Context, gainDB float64) (equalizer.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return equalizer.Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	return handler.service.SetPreamp(gainDB)
}

func (handler *EqualizerHandler) SetVisualizerMode(_ context.Context, mode string) (equalizer.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return equalizer.Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	return handler.service.SetVisualizerMode(equalizer.VisualizerMode(mode))
}

func (handler *EqualizerHandler) VisualizerFrame(_ context.Context) (equalizer.VisualizerFrame, error) {
	if handler == nil || handler.service == nil {
		return equalizer.VisualizerFrame{}, fmt.Errorf("equalizer service unavailable")
	}
	return handler.service.VisualizerFrame(), nil
}

func (handler *EqualizerHandler) Reset(_ context.Context) (equalizer.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return equalizer.Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	return handler.service.Reset()
}

func (handler *EqualizerHandler) Retry(_ context.Context) (equalizer.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return equalizer.Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	handler.service.RetryStartNowIfEnabled()
	return handler.service.Snapshot(), nil
}

func (handler *EqualizerHandler) OpenPermissionGuide(_ context.Context, permissionName string, hint string) error {
	return openPermissionGuide(screenSystemAudioPermissionGuideRequest(permissionName, hint))
}
