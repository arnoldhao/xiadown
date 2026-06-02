package equalizer

import "testing"

type fakeEngine struct {
	supported      bool
	features       EngineFeatures
	running        bool
	observedAudio  bool
	visualizer     VisualizerFrame
	startFailure   *StartFailure
	startCallCount int
	stopCallCount  int
}

func (engine *fakeEngine) Features() EngineFeatures {
	if engine.features.Equalizer || engine.features.Visualizer {
		return engine.features
	}
	return EngineFeatures{Equalizer: true, Visualizer: true}
}

func (engine *fakeEngine) Supported() bool {
	return engine.supported
}

func (engine *fakeEngine) IsRunning() bool {
	return engine.running
}

func (engine *fakeEngine) HasObservedAudio() bool {
	return engine.observedAudio
}

func (engine *fakeEngine) VisualizerFrame() VisualizerFrame {
	if len(engine.visualizer.Bands) > 0 || len(engine.visualizer.Waveform) > 0 {
		return engine.visualizer
	}
	return emptyVisualizerFrame(engine.running)
}

func (engine *fakeEngine) HasCapturePermission() bool {
	return true
}

func (engine *fakeEngine) RequestCapturePermission() bool {
	return true
}

func (engine *fakeEngine) Start(Settings) *StartFailure {
	engine.startCallCount++
	if engine.startFailure != nil {
		engine.running = false
		return engine.startFailure
	}
	engine.running = true
	return nil
}

func (engine *fakeEngine) Apply(Settings) {}

func (engine *fakeEngine) Stop() {
	engine.running = false
	engine.stopCallCount++
}

func TestActivePlaybackNoAudioSourceIsPermissionNeeded(t *testing.T) {
	engine := &fakeEngine{
		supported:    true,
		startFailure: &StartFailure{Code: StartFailureNoAudioSource},
	}
	service := NewService(nil, engine)

	service.mu.Lock()
	service.settings.Enabled = true
	service.attemptStartLocked(true)
	snapshot := service.snapshotLocked()
	service.mu.Unlock()

	if snapshot.Status.Code != StatusPermissionNeeded || !snapshot.Status.PermissionRequired {
		t.Fatalf("expected no audio source during active playback to be reported as permission needed, got %#v", snapshot.Status)
	}
}

func TestTapVerificationFailureIsPermissionNeeded(t *testing.T) {
	engine := &fakeEngine{supported: true, running: true}
	service := NewService(nil, engine)

	service.mu.Lock()
	service.settings.Enabled = true
	service.flagTapVerificationFailureLocked()
	snapshot := service.snapshotLocked()
	service.mu.Unlock()

	if snapshot.Status.Code != StatusPermissionNeeded || !snapshot.Status.PermissionRequired {
		t.Fatalf("expected tap verification failure to be reported as permission needed, got %#v", snapshot.Status)
	}
	if engine.stopCallCount == 0 {
		t.Fatal("expected tap verification failure to stop the engine")
	}
}

func TestVisualizerModeIsClampedAndPreservedByReset(t *testing.T) {
	engine := &fakeEngine{supported: true}
	service := NewService(nil, engine)

	snapshot, err := service.SetVisualizerMode(VisualizerModeHeatmap)
	if err != nil {
		t.Fatalf("set visualizer mode: %v", err)
	}
	if snapshot.Settings.VisualizerMode != VisualizerModeHeatmap {
		t.Fatalf("expected heatmap mode, got %q", snapshot.Settings.VisualizerMode)
	}
	if snapshot.Settings.VisualizerPlacement != VisualizerPlacementSpectrum {
		t.Fatalf("expected spectrum placement, got %q", snapshot.Settings.VisualizerPlacement)
	}
	if snapshot.Settings.SpectrumVisualizerMode != VisualizerModeHeatmap {
		t.Fatalf("expected remembered heatmap spectrum mode, got %q", snapshot.Settings.SpectrumVisualizerMode)
	}

	snapshot, err = service.SetVisualizerMode(VisualizerModeHalo)
	if err != nil {
		t.Fatalf("set halo visualizer mode: %v", err)
	}
	if snapshot.Settings.VisualizerMode != VisualizerModeHalo {
		t.Fatalf("expected halo mode, got %q", snapshot.Settings.VisualizerMode)
	}
	if snapshot.Settings.VisualizerPlacement != VisualizerPlacementArtwork {
		t.Fatalf("expected artwork placement, got %q", snapshot.Settings.VisualizerPlacement)
	}
	if snapshot.Settings.ArtworkVisualizerMode != VisualizerModeHalo {
		t.Fatalf("expected remembered halo artwork mode, got %q", snapshot.Settings.ArtworkVisualizerMode)
	}
	if snapshot.Settings.SpectrumVisualizerMode != VisualizerModeHeatmap {
		t.Fatalf("expected spectrum memory to preserve heatmap, got %q", snapshot.Settings.SpectrumVisualizerMode)
	}

	snapshot, err = service.SetVisualizerMode(VisualizerModeNeonPulse)
	if err != nil {
		t.Fatalf("set neon pulse visualizer mode: %v", err)
	}
	if snapshot.Settings.VisualizerMode != VisualizerModeNeonPulse {
		t.Fatalf("expected neon pulse mode, got %q", snapshot.Settings.VisualizerMode)
	}
	if snapshot.Settings.ArtworkVisualizerMode != VisualizerModeNeonPulse {
		t.Fatalf("expected remembered neon pulse artwork mode, got %q", snapshot.Settings.ArtworkVisualizerMode)
	}

	snapshot, err = service.SetVisualizerMode(VisualizerModePondRipple)
	if err != nil {
		t.Fatalf("set pond ripple visualizer mode: %v", err)
	}
	if snapshot.Settings.VisualizerMode != VisualizerModePondRipple {
		t.Fatalf("expected pond ripple mode, got %q", snapshot.Settings.VisualizerMode)
	}
	if snapshot.Settings.ArtworkVisualizerMode != VisualizerModePondRipple {
		t.Fatalf("expected remembered pond ripple artwork mode, got %q", snapshot.Settings.ArtworkVisualizerMode)
	}

	snapshot, err = service.SetVisualizerMode(VisualizerModeOff)
	if err != nil {
		t.Fatalf("set off visualizer mode: %v", err)
	}
	if snapshot.Settings.VisualizerMode != VisualizerModeOff {
		t.Fatalf("expected off mode, got %q", snapshot.Settings.VisualizerMode)
	}
	if snapshot.Settings.VisualizerPlacement != VisualizerPlacementOff {
		t.Fatalf("expected off placement, got %q", snapshot.Settings.VisualizerPlacement)
	}
	if snapshot.Settings.ArtworkVisualizerMode != VisualizerModePondRipple {
		t.Fatalf("expected artwork memory to preserve pond ripple, got %q", snapshot.Settings.ArtworkVisualizerMode)
	}
	if snapshot.Settings.SpectrumVisualizerMode != VisualizerModeHeatmap {
		t.Fatalf("expected spectrum memory to preserve heatmap, got %q", snapshot.Settings.SpectrumVisualizerMode)
	}

	snapshot, err = service.Reset()
	if err != nil {
		t.Fatalf("reset equalizer: %v", err)
	}
	if snapshot.Settings.VisualizerMode != VisualizerModeOff {
		t.Fatalf("expected reset to preserve off mode, got %q", snapshot.Settings.VisualizerMode)
	}
	if snapshot.Settings.VisualizerPlacement != VisualizerPlacementOff {
		t.Fatalf("expected reset to preserve off placement, got %q", snapshot.Settings.VisualizerPlacement)
	}
	if snapshot.Settings.ArtworkVisualizerMode != VisualizerModePondRipple {
		t.Fatalf("expected reset to preserve pond ripple artwork mode, got %q", snapshot.Settings.ArtworkVisualizerMode)
	}
	if snapshot.Settings.SpectrumVisualizerMode != VisualizerModeHeatmap {
		t.Fatalf("expected reset to preserve heatmap spectrum mode, got %q", snapshot.Settings.SpectrumVisualizerMode)
	}

	snapshot, err = service.SetVisualizerMode(VisualizerMode("unknown"))
	if err != nil {
		t.Fatalf("set unknown visualizer mode: %v", err)
	}
	if snapshot.Settings.VisualizerMode != VisualizerModeRing {
		t.Fatalf("expected unknown mode to clamp to ring, got %q", snapshot.Settings.VisualizerMode)
	}
	if snapshot.Settings.ArtworkVisualizerMode != VisualizerModeRing {
		t.Fatalf("expected unknown mode to update artwork memory to ring, got %q", snapshot.Settings.ArtworkVisualizerMode)
	}
	if snapshot.Settings.SpectrumVisualizerMode != VisualizerModeHeatmap {
		t.Fatalf("expected unknown mode to preserve spectrum memory, got %q", snapshot.Settings.SpectrumVisualizerMode)
	}
}

func TestVisualizerSettingsMigrateFromLegacyMode(t *testing.T) {
	settings := ClampSettings(Settings{
		VisualizerMode: VisualizerModeWaveform,
	})
	if settings.VisualizerPlacement != VisualizerPlacementSpectrum {
		t.Fatalf("expected legacy waveform to migrate to spectrum placement, got %q", settings.VisualizerPlacement)
	}
	if settings.VisualizerMode != VisualizerModeWaveform {
		t.Fatalf("expected active waveform mode, got %q", settings.VisualizerMode)
	}
	if settings.SpectrumVisualizerMode != VisualizerModeWaveform {
		t.Fatalf("expected waveform spectrum memory, got %q", settings.SpectrumVisualizerMode)
	}
	if settings.ArtworkVisualizerMode != VisualizerModeRing {
		t.Fatalf("expected default ring artwork memory, got %q", settings.ArtworkVisualizerMode)
	}
}

func TestVisualizerFrameOnlyReportsWhileRunning(t *testing.T) {
	engine := &fakeEngine{
		supported: true,
		running:   true,
		visualizer: VisualizerFrame{
			Running:                true,
			Sequence:               42,
			Level:                  0.75,
			Bands:                  []float64{0.1, 0.2},
			Waveform:               []float64{0.3, -0.3},
			AnalysisTimeSeconds:    12.5,
			FrameTimeOffsetSeconds: -0.019,
		},
	}
	service := NewService(nil, engine)

	frame := service.VisualizerFrame()
	if frame.Running || frame.Sequence != 0 || frame.Level != 0 {
		t.Fatalf("expected disabled equalizer to hide visualizer frame, got %#v", frame)
	}
	if frame.AnalysisTimeSeconds != 0 || frame.FrameTimeOffsetSeconds != 0 {
		t.Fatalf("expected disabled visualizer frame to clear timing metadata, got %#v", frame)
	}

	if _, err := service.SetEnabled(true); err != nil {
		t.Fatalf("enable equalizer: %v", err)
	}
	frame = service.VisualizerFrame()
	if !frame.Running || frame.Sequence != 42 || frame.Level != 0.75 {
		t.Fatalf("expected running frame from engine, got %#v", frame)
	}
	if frame.AnalysisTimeSeconds != 12.5 {
		t.Fatalf("expected analysis time from engine, got %#v", frame)
	}
	if frame.FrameTimeOffsetSeconds != -0.019 {
		t.Fatalf("expected native frame time offset from engine, got %#v", frame)
	}
}

func TestVisualizerOnlyEngineRunsWhenVisualizerIsNotOff(t *testing.T) {
	engine := &fakeEngine{
		supported: true,
		features:  EngineFeatures{Visualizer: true},
	}
	service := NewService(nil, engine)

	snapshot := service.Snapshot()
	if snapshot.Status.Code != StatusStandby {
		t.Fatalf("expected visualizer-only engine to wait for playback, got %#v", snapshot.Status)
	}
	if engine.startCallCount != 0 {
		t.Fatal("expected visualizer-only engine to avoid starting before playback")
	}

	service.RetryStartNowIfEnabled()
	if engine.startCallCount != 0 {
		t.Fatal("expected visualizer-only retry to avoid starting before playback")
	}

	service.ObservePlayback(true, 0)
	service.RetryStartNowIfEnabled()
	snapshot = service.Snapshot()
	if snapshot.Status.Code != StatusActive {
		t.Fatalf("expected visualizer-only engine to start during playback, got %#v", snapshot.Status)
	}
	if engine.startCallCount == 0 {
		t.Fatal("expected visualizer-only engine to start after playback")
	}

	service.ObservePlayback(false, 0)
	snapshot = service.Snapshot()
	if snapshot.Status.Code != StatusStandby {
		t.Fatalf("expected visualizer-only engine to return to standby after playback stops, got %#v", snapshot.Status)
	}
	if engine.stopCallCount == 0 {
		t.Fatal("expected visualizer-only engine to stop after playback stops")
	}

	snapshot, err := service.SetVisualizerMode(VisualizerModeOff)
	if err != nil {
		t.Fatalf("set visualizer off: %v", err)
	}
	if snapshot.Status.Code != StatusOff {
		t.Fatalf("expected visualizer-only engine to stop when visualizer is off, got %#v", snapshot.Status)
	}
	if engine.stopCallCount == 0 {
		t.Fatal("expected visualizer-only engine to stop when visualizer is off")
	}
}

func TestVisualizerOnlyNoAudioSourceIsNotPermissionNeeded(t *testing.T) {
	engine := &fakeEngine{
		supported:    true,
		features:     EngineFeatures{Visualizer: true},
		startFailure: &StartFailure{Code: StartFailureNoAudioSource},
	}
	service := NewService(nil, engine)

	service.mu.Lock()
	service.attemptStartLocked(true)
	snapshot := service.snapshotLocked()
	service.mu.Unlock()

	if snapshot.Status.Code == StatusPermissionNeeded || snapshot.Status.PermissionRequired {
		t.Fatalf("expected visualizer-only no audio source to avoid permission state, got %#v", snapshot.Status)
	}
}
