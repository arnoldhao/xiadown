//go:build (!darwin && !windows) || (darwin && !cgo) || ios

package equalizeraudio

import "xiadown/internal/application/equalizer"

type Engine struct{}

func NewEngine(_ ...Option) *Engine {
	return &Engine{}
}

func (engine *Engine) Features() equalizer.EngineFeatures {
	return equalizer.EngineFeatures{}
}

func (engine *Engine) Supported() bool {
	return false
}

func (engine *Engine) IsRunning() bool {
	return false
}

func (engine *Engine) HasObservedAudio() bool {
	return false
}

func (engine *Engine) VisualizerFrame() equalizer.VisualizerFrame {
	return equalizer.VisualizerFrame{
		Running:  false,
		Bands:    make([]float64, equalizer.VisualizerBandCount),
		Waveform: make([]float64, equalizer.VisualizerWaveformCount),
	}
}

func (engine *Engine) HasCapturePermission() bool {
	return false
}

func (engine *Engine) RequestCapturePermission() bool {
	return false
}

func (engine *Engine) Start(equalizer.Settings) *equalizer.StartFailure {
	return &equalizer.StartFailure{Code: equalizer.StartFailureUnsupported}
}

func (engine *Engine) Apply(equalizer.Settings) {}

func (engine *Engine) Stop() {}
