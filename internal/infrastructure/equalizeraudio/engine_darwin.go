//go:build darwin && cgo && !ios

package equalizeraudio

/*
#cgo darwin CFLAGS: -x objective-c -fblocks -mmacosx-version-min=10.15
#cgo darwin LDFLAGS: -framework ApplicationServices -framework AppKit -framework CoreAudio -framework CoreFoundation -framework CoreGraphics -framework Foundation -framework WebKit
#include "native_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"xiadown/internal/application/equalizer"
)

type Engine struct{}

func NewEngine(_ ...Option) *Engine {
	return &Engine{}
}

func (engine *Engine) Features() equalizer.EngineFeatures {
	return equalizer.EngineFeatures{
		Equalizer:  true,
		Visualizer: true,
	}
}

func (engine *Engine) Supported() bool {
	return C.xia_equalizer_supported() == 1
}

func (engine *Engine) IsRunning() bool {
	return C.xia_equalizer_is_running() == 1
}

func (engine *Engine) HasObservedAudio() bool {
	return C.xia_equalizer_has_observed_audio() == 1
}

func (engine *Engine) VisualizerFrame() equalizer.VisualizerFrame {
	bands := make([]C.float, equalizer.VisualizerBandCount)
	waveform := make([]C.float, equalizer.VisualizerWaveformCount)
	var level C.double
	var sequence C.ulonglong
	var analysisTimeSeconds C.double
	var frameTimeOffsetSeconds C.double
	running := C.xia_equalizer_visualizer_frame(
		&bands[0],
		C.int(len(bands)),
		&waveform[0],
		C.int(len(waveform)),
		&level,
		&sequence,
		&analysisTimeSeconds,
		&frameTimeOffsetSeconds,
	) == 1
	return equalizer.VisualizerFrame{
		Running:                running,
		Sequence:               uint64(sequence),
		Level:                  float64(level),
		Bands:                  floatSliceFromC(bands),
		Waveform:               floatSliceFromC(waveform),
		AnalysisTimeSeconds:    float64(analysisTimeSeconds),
		FrameTimeOffsetSeconds: float64(frameTimeOffsetSeconds),
	}
}

func (engine *Engine) HasCapturePermission() bool {
	return C.xia_equalizer_has_capture_permission() == 1
}

func (engine *Engine) RequestCapturePermission() bool {
	return C.xia_equalizer_request_capture_permission() == 1
}

func (engine *Engine) Start(settings equalizer.Settings) *equalizer.StartFailure {
	settings = equalizer.ClampSettings(settings)
	var detail C.int
	status := C.xia_equalizer_start(
		cBool(settings.Enabled),
		C.double(settings.PreampDB),
		gainPointer(settings.BandGainsDB),
		C.int(len(settings.BandGainsDB)),
		&detail,
	)
	if status == C.XiaEQStartSuccess {
		return nil
	}
	return startFailureFromNativeStatus(status, int(detail))
}

func (engine *Engine) Apply(settings equalizer.Settings) {
	settings = equalizer.ClampSettings(settings)
	C.xia_equalizer_apply(
		cBool(settings.Enabled),
		C.double(settings.PreampDB),
		gainPointer(settings.BandGainsDB),
		C.int(len(settings.BandGainsDB)),
	)
}

func (engine *Engine) Stop() {
	C.xia_equalizer_stop()
}

func gainPointer(gains []float64) *C.double {
	if len(gains) == 0 {
		return nil
	}
	return (*C.double)(unsafe.Pointer(&gains[0]))
}

func cBool(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

func floatSliceFromC(values []C.float) []float64 {
	result := make([]float64, len(values))
	for index, value := range values {
		result[index] = float64(value)
	}
	return result
}

func startFailureFromNativeStatus(status C.int, detail int) *equalizer.StartFailure {
	failure := &equalizer.StartFailure{Detail: detailString(detail)}
	switch status {
	case C.XiaEQStartUnsupported:
		failure.Code = equalizer.StartFailureUnsupported
	case C.XiaEQStartPermissionDenied:
		failure.Code = equalizer.StartFailurePermissionDenied
	case C.XiaEQStartNoAudioSource:
		failure.Code = equalizer.StartFailureNoAudioSource
	case C.XiaEQStartTapCreation:
		failure.Code = equalizer.StartFailureTapCreation
	case C.XiaEQStartAggregateCreation:
		failure.Code = equalizer.StartFailureAggregateDeviceCreation
	case C.XiaEQStartInvalidTapFormat:
		failure.Code = equalizer.StartFailureInvalidTapFormat
	case C.XiaEQStartIOProcInstall:
		failure.Code = equalizer.StartFailureIOProcInstall
	case C.XiaEQStartDeviceStart:
		failure.Code = equalizer.StartFailureEngineStart
	default:
		failure.Code = equalizer.StartFailureUnknown
	}
	return failure
}

func detailString(detail int) string {
	if detail == 0 {
		return ""
	}
	return fmt.Sprintf("OSStatus %d", detail)
}
