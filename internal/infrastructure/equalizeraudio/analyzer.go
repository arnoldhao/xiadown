package equalizeraudio

import (
	"math"
	"sync"
	"time"

	"xiadown/internal/application/equalizer"
)

type visualizerAnalyzer struct {
	mu sync.Mutex

	sampleRate   float64
	coefficients [equalizer.VisualizerBandCount]float64

	running         bool
	hasObserved     bool
	sequence        uint64
	level           float64
	bands           [equalizer.VisualizerBandCount]float64
	waveform        [equalizer.VisualizerWaveformCount]float64
	analysisSeconds float64
	frameTime       time.Time
}

func newVisualizerAnalyzer(sampleRate float64) *visualizerAnalyzer {
	analyzer := &visualizerAnalyzer{}
	analyzer.setSampleRate(sampleRate)
	return analyzer
}

func (analyzer *visualizerAnalyzer) setRunning(running bool) {
	analyzer.mu.Lock()
	defer analyzer.mu.Unlock()
	for index := range analyzer.bands {
		analyzer.bands[index] = 0
	}
	for index := range analyzer.waveform {
		analyzer.waveform[index] = 0
	}
	analyzer.running = running
	analyzer.level = 0
	analyzer.analysisSeconds = 0
	analyzer.frameTime = time.Time{}
	analyzer.sequence++
	if !running {
		analyzer.hasObserved = false
	}
}

func (analyzer *visualizerAnalyzer) setSampleRate(sampleRate float64) {
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	analyzer.sampleRate = sampleRate
	minFrequency := 45.0
	maxFrequency := math.Min(16000, sampleRate*0.45)
	if maxFrequency <= minFrequency {
		maxFrequency = minFrequency * 2
	}
	for band := 0; band < equalizer.VisualizerBandCount; band++ {
		ratio := float64(band) / float64(equalizer.VisualizerBandCount-1)
		frequency := minFrequency * math.Pow(maxFrequency/minFrequency, ratio)
		normalized := 2 * math.Pi * frequency / sampleRate
		analyzer.coefficients[band] = 2 * math.Cos(normalized)
	}
}

func (analyzer *visualizerAnalyzer) analyzePCM16(interleaved []int16, channels int, frames int) {
	if analyzer == nil || frames <= 8 || channels <= 0 || len(interleaved) < frames*channels {
		return
	}

	mono := make([]float64, frames)
	observed := false
	for frame := 0; frame < frames; frame++ {
		offset := frame * channels
		left := float64(interleaved[offset]) / 32768
		right := left
		if channels >= 2 {
			right = float64(interleaved[offset+1]) / 32768
		}
		sample := (left + right) * 0.5
		mono[frame] = sample
		if sample != 0 {
			observed = true
		}
	}
	if observed {
		analyzer.mu.Lock()
		analyzer.hasObserved = true
		analyzer.mu.Unlock()
	}
	analyzer.analyzeMono(mono)
}

func (analyzer *visualizerAnalyzer) analyzeSilence(frames int) {
	if analyzer == nil || frames <= 8 {
		return
	}
	analyzer.analyzeMono(make([]float64, frames))
}

func (analyzer *visualizerAnalyzer) analyzeMono(mono []float64) {
	frameCount := len(mono)
	if frameCount <= 8 {
		return
	}

	var bands [equalizer.VisualizerBandCount]float64
	var waveform [equalizer.VisualizerWaveformCount]float64
	var sumSquares float64

	for index := 0; index < equalizer.VisualizerWaveformCount; index++ {
		start := int(int64(index) * int64(frameCount) / int64(equalizer.VisualizerWaveformCount))
		end := int(int64(index+1) * int64(frameCount) / int64(equalizer.VisualizerWaveformCount))
		if end <= start {
			end = start + 1
		}
		var sum float64
		var count int
		for frame := start; frame < end && frame < frameCount; frame++ {
			sum += mono[frame]
			count++
		}
		if count > 0 {
			waveform[index] = sum / float64(count)
		}
	}

	for _, sample := range mono {
		sumSquares += sample * sample
	}

	for band := 0; band < equalizer.VisualizerBandCount; band++ {
		var q1, q2 float64
		coefficient := analyzer.coefficients[band]
		for _, sample := range mono {
			q0 := coefficient*q1 - q2 + sample
			q2 = q1
			q1 = q0
		}
		power := q1*q1 + q2*q2 - coefficient*q1*q2
		if power < 0 || math.IsNaN(power) || math.IsInf(power, 0) {
			power = 0
		}
		magnitude := math.Sqrt(power) / float64(frameCount)
		db := 20 * math.Log10(magnitude*3.0+1.0e-8)
		bands[band] = clampUnit((db + 58.0) / 58.0)
	}

	rms := math.Sqrt(sumSquares / float64(frameCount))
	levelDB := 20 * math.Log10(rms*2.0+1.0e-8)
	level := clampUnit((levelDB + 54.0) / 54.0)
	durationSeconds := float64(frameCount) / analyzer.sampleRate
	analyzer.commit(bands, waveform, level, durationSeconds)
}

func (analyzer *visualizerAnalyzer) commit(
	bands [equalizer.VisualizerBandCount]float64,
	waveform [equalizer.VisualizerWaveformCount]float64,
	level float64,
	durationSeconds float64,
) {
	analyzer.mu.Lock()
	defer analyzer.mu.Unlock()

	for band := 0; band < equalizer.VisualizerBandCount; band++ {
		current := analyzer.bands[band]
		target := clampUnit(bands[band])
		alpha := 0.16
		if target > current {
			alpha = 0.42
		}
		analyzer.bands[band] = current + (target-current)*alpha
	}
	for index := 0; index < equalizer.VisualizerWaveformCount; index++ {
		target := waveform[index]
		if math.IsNaN(target) || math.IsInf(target, 0) {
			target = 0
		}
		target = math.Min(1, math.Max(-1, target))
		current := analyzer.waveform[index]
		analyzer.waveform[index] = current + (target-current)*0.5
	}
	currentLevel := analyzer.level
	targetLevel := clampUnit(level)
	levelAlpha := 0.12
	if targetLevel > currentLevel {
		levelAlpha = 0.5
	}
	analyzer.level = currentLevel + (targetLevel-currentLevel)*levelAlpha
	if durationSeconds > 0 && !math.IsNaN(durationSeconds) && !math.IsInf(durationSeconds, 0) {
		analyzer.analysisSeconds += durationSeconds
	}
	analyzer.frameTime = time.Now()
	analyzer.running = true
	analyzer.sequence++
}

func (analyzer *visualizerAnalyzer) hasObservedAudio() bool {
	if analyzer == nil {
		return false
	}
	analyzer.mu.Lock()
	defer analyzer.mu.Unlock()
	return analyzer.hasObserved
}

func (analyzer *visualizerAnalyzer) frame() equalizer.VisualizerFrame {
	if analyzer == nil {
		return equalizer.VisualizerFrame{
			Running:  false,
			Bands:    make([]float64, equalizer.VisualizerBandCount),
			Waveform: make([]float64, equalizer.VisualizerWaveformCount),
		}
	}

	analyzer.mu.Lock()
	defer analyzer.mu.Unlock()

	bands := make([]float64, equalizer.VisualizerBandCount)
	waveform := make([]float64, equalizer.VisualizerWaveformCount)
	copy(bands, analyzer.bands[:])
	copy(waveform, analyzer.waveform[:])

	var offset float64
	if !analyzer.frameTime.IsZero() {
		offset = time.Since(analyzer.frameTime).Seconds()
	}
	return equalizer.VisualizerFrame{
		Running:                analyzer.running,
		Sequence:               analyzer.sequence,
		Level:                  clampUnit(analyzer.level),
		Bands:                  bands,
		Waveform:               waveform,
		AnalysisTimeSeconds:    analyzer.analysisSeconds,
		FrameTimeOffsetSeconds: offset,
	}
}

func clampUnit(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Min(1, math.Max(0, value))
}
