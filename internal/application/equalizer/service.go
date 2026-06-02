package equalizer

import (
	"fmt"
	"sync"
	"time"
)

const (
	retryDelay                   = 500 * time.Millisecond
	tapVerificationPollInterval  = 2 * time.Second
	tapVerificationProgressFloor = 8
)

type Service struct {
	mu sync.Mutex

	store  Store
	engine Engine

	settings Settings

	lastFailure               *StartFailure
	inferredPermissionDenial  bool
	requestPermissionNextTime bool

	playbackActive   bool
	playbackProgress float64

	retryGeneration        uint64
	verificationGeneration uint64
}

func NewService(store Store, engine Engine) *Service {
	settings := FlatSettings()
	if store != nil {
		if loaded, ok, err := store.Load(); err == nil && ok {
			settings = loaded
		}
	}
	service := &Service{
		store:    store,
		engine:   engine,
		settings: ClampSettings(settings),
	}
	service.syncEngine(false)
	return service
}

func (service *Service) Snapshot() Snapshot {
	if service == nil {
		return Snapshot{
			Settings: FlatSettings(),
			Status: Status{
				Code:      StatusUnsupported,
				Supported: false,
				Message:   "Equalizer service is unavailable.",
			},
			Bands:   cloneBands(),
			Presets: clonePresets(),
		}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.snapshotLocked()
}

func (service *Service) SetEnabled(enabled bool) (Snapshot, error) {
	return service.update(func(settings *Settings) {
		settings.Enabled = enabled
	}, enabled)
}

func (service *Service) ApplyPreset(presetID string) (Snapshot, error) {
	preset, ok := PresetByID(presetID)
	if !ok {
		return service.Snapshot(), fmt.Errorf("unknown equalizer preset %q", presetID)
	}
	return service.update(func(settings *Settings) {
		settings.Preset = preset.ID
		settings.BandGainsDB = append([]float64(nil), preset.GainsDB...)
	}, false)
}

func (service *Service) SetBandGain(index int, gainDB float64) (Snapshot, error) {
	if index < 0 || index >= len(DefaultBands) {
		return service.Snapshot(), fmt.Errorf("equalizer band index out of range")
	}
	return service.update(func(settings *Settings) {
		settings.BandGainsDB[index] = gainDB
		settings.Preset = "custom"
	}, false)
}

func (service *Service) SetPreamp(gainDB float64) (Snapshot, error) {
	return service.update(func(settings *Settings) {
		settings.PreampDB = gainDB
	}, false)
}

func (service *Service) SetVisualizerMode(mode VisualizerMode) (Snapshot, error) {
	return service.update(func(settings *Settings) {
		ApplyVisualizerMode(settings, mode)
	}, false)
}

func (service *Service) Reset() (Snapshot, error) {
	return service.update(func(settings *Settings) {
		enabled := settings.Enabled
		visualizerMode := settings.VisualizerMode
		visualizerPlacement := settings.VisualizerPlacement
		artworkVisualizerMode := settings.ArtworkVisualizerMode
		spectrumVisualizerMode := settings.SpectrumVisualizerMode
		*settings = FlatSettings()
		settings.Enabled = enabled
		settings.VisualizerMode = visualizerMode
		settings.VisualizerPlacement = visualizerPlacement
		settings.ArtworkVisualizerMode = artworkVisualizerMode
		settings.SpectrumVisualizerMode = spectrumVisualizerMode
	}, false)
}

func (service *Service) RetryStartIfEnabled() {
	if service == nil {
		return
	}
	service.mu.Lock()
	if !service.shouldRunEngineLocked() || service.engine == nil || service.engine.IsRunning() {
		service.mu.Unlock()
		return
	}
	service.retryGeneration++
	generation := service.retryGeneration
	service.mu.Unlock()

	go func() {
		time.Sleep(retryDelay)
		service.mu.Lock()
		defer service.mu.Unlock()
		if generation != service.retryGeneration || !service.shouldRunEngineLocked() || service.engine == nil || service.engine.IsRunning() {
			return
		}
		service.attemptStartLocked(true)
	}()
}

func (service *Service) RetryStartNowIfEnabled() {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if !service.shouldRunEngineLocked() || service.engine == nil || service.engine.IsRunning() {
		return
	}
	if service.shouldWaitForPlaybackBeforeStartLocked(service.playbackActive) {
		service.retryGeneration++
		service.verificationGeneration++
		service.lastFailure = nil
		service.inferredPermissionDenial = false
		service.requestPermissionNextTime = false
		service.engine.Stop()
		return
	}
	service.retryGeneration++
	service.attemptStartLocked(service.playbackActive)
}

func (service *Service) ObservePlayback(active bool, progress float64) {
	if service == nil {
		return
	}
	service.mu.Lock()
	service.playbackActive = active
	if progress >= 0 {
		service.playbackProgress = progress
	}
	shouldSyncInactive := !active && service.shouldStopOnInactivePlaybackLocked()
	service.mu.Unlock()
	if active {
		service.RetryStartIfEnabled()
		return
	}
	if shouldSyncInactive {
		service.syncEngine(false)
	}
}

func (service *Service) Stop() {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.retryGeneration++
	service.verificationGeneration++
	if service.engine != nil {
		service.engine.Stop()
	}
}

func (service *Service) VisualizerFrame() VisualizerFrame {
	if service == nil || service.engine == nil {
		return emptyVisualizerFrame(false)
	}
	service.mu.Lock()
	running := service.shouldRunEngineLocked() && service.engine.IsRunning()
	service.mu.Unlock()
	if !running {
		return emptyVisualizerFrame(false)
	}
	frame := service.engine.VisualizerFrame()
	frame.Running = running && frame.Running
	if len(frame.Bands) == 0 {
		frame.Bands = make([]float64, VisualizerBandCount)
	}
	if len(frame.Waveform) == 0 {
		frame.Waveform = make([]float64, VisualizerWaveformCount)
	}
	return frame
}

func emptyVisualizerFrame(running bool) VisualizerFrame {
	return VisualizerFrame{
		Running:  running,
		Bands:    make([]float64, VisualizerBandCount),
		Waveform: make([]float64, VisualizerWaveformCount),
	}
}

func (service *Service) update(mutator func(*Settings), requestPermission bool) (Snapshot, error) {
	if service == nil {
		return Snapshot{}, fmt.Errorf("equalizer service unavailable")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	mutator(&service.settings)
	service.settings = ClampSettings(service.settings)
	if requestPermission {
		service.inferredPermissionDenial = false
		service.lastFailure = nil
		service.requestPermissionNextTime = service.settings.Enabled
	}
	if service.store != nil {
		if err := service.store.Save(service.settings); err != nil {
			return service.snapshotLocked(), err
		}
	}
	service.syncEngineLocked(false)
	return service.snapshotLocked(), nil
}

func (service *Service) syncEngine(playbackKnownActive bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.syncEngineLocked(playbackKnownActive)
}

func (service *Service) syncEngineLocked(playbackKnownActive bool) {
	if service.engine == nil || !service.engine.Supported() {
		if service.engine != nil {
			service.engine.Stop()
		}
		service.lastFailure = &StartFailure{Code: StartFailureUnsupported}
		return
	}
	if service.shouldRunEngineLocked() {
		if service.shouldWaitForPlaybackBeforeStartLocked(playbackKnownActive) {
			service.retryGeneration++
			service.verificationGeneration++
			service.lastFailure = nil
			service.inferredPermissionDenial = false
			service.requestPermissionNextTime = false
			service.engine.Stop()
			return
		}
		service.attemptStartLocked(playbackKnownActive)
		return
	}
	service.retryGeneration++
	service.verificationGeneration++
	service.engine.Stop()
	service.lastFailure = nil
	service.inferredPermissionDenial = false
	service.requestPermissionNextTime = false
}

func (service *Service) attemptStartLocked(playbackKnownActive bool) {
	if service.engine == nil {
		return
	}
	if !service.engine.Supported() {
		service.lastFailure = &StartFailure{Code: StartFailureUnsupported}
		return
	}
	if !service.engine.HasCapturePermission() {
		if service.requestPermissionNextTime {
			service.requestPermissionNextTime = false
			_ = service.engine.RequestCapturePermission()
		}
		if !service.engine.HasCapturePermission() {
			service.flagPermissionDenialLocked()
			return
		}
	}
	service.requestPermissionNextTime = false

	if service.engine.IsRunning() {
		service.engine.Apply(service.settings)
		service.lastFailure = nil
		service.inferredPermissionDenial = false
		return
	}

	failure := service.engine.Start(service.settings)
	if failure == nil {
		service.lastFailure = nil
		service.inferredPermissionDenial = false
		service.engine.Apply(service.settings)
		service.scheduleTapVerificationLocked()
		return
	}
	if failure.IsWaitingForPlayback() && !playbackKnownActive {
		service.lastFailure = nil
		return
	}
	if failure.IsWaitingForPlayback() && playbackKnownActive && service.engineFeaturesLocked().Equalizer {
		service.flagPermissionDenialLocked()
		return
	}
	if failure.IsWaitingForPlayback() && playbackKnownActive {
		service.lastFailure = failure
		return
	}
	if failure.IsPermissionLikely() {
		service.flagPermissionDenialLocked()
		return
	}
	service.lastFailure = failure
}

func (service *Service) flagPermissionDenialLocked() {
	service.verificationGeneration++
	service.inferredPermissionDenial = true
	service.lastFailure = nil
	if service.engine != nil {
		service.engine.Stop()
	}
}

func (service *Service) scheduleTapVerificationLocked() {
	if service.engine == nil {
		return
	}
	service.verificationGeneration++
	generation := service.verificationGeneration
	initialProgress := service.playbackProgress

	go func() {
		ticker := time.NewTicker(tapVerificationPollInterval)
		defer ticker.Stop()
		for range ticker.C {
			service.mu.Lock()
			if generation != service.verificationGeneration ||
				service.engine == nil ||
				!service.engine.IsRunning() ||
				!service.shouldRunEngineLocked() {
				service.mu.Unlock()
				return
			}
			if service.engine.HasObservedAudio() {
				service.mu.Unlock()
				return
			}
			if !service.playbackActive {
				service.mu.Unlock()
				continue
			}
			progressed := service.playbackProgress - initialProgress
			if progressed < tapVerificationProgressFloor {
				service.mu.Unlock()
				continue
			}
			service.flagTapVerificationFailureLocked()
			service.mu.Unlock()
			return
		}
	}()
}

func (service *Service) flagTapVerificationFailureLocked() {
	if !service.engineFeaturesLocked().Equalizer {
		service.verificationGeneration++
		service.inferredPermissionDenial = false
		service.lastFailure = &StartFailure{Code: StartFailureNoAudioSource}
		if service.engine != nil {
			service.engine.Stop()
		}
		return
	}
	service.flagPermissionDenialLocked()
}

func (service *Service) snapshotLocked() Snapshot {
	settings := cloneSettings(service.settings)
	return Snapshot{
		Settings: settings,
		Status:   service.statusLocked(),
		Bands:    cloneBands(),
		Presets:  clonePresets(),
	}
}

func (service *Service) statusLocked() Status {
	if service.engine == nil || !service.engine.Supported() {
		return Status{
			Code:      StatusUnsupported,
			Supported: false,
			Message:   "The equalizer requires macOS 14.2 or later. Visualization requires Windows 10 build 20348 or later.",
		}
	}
	features := service.engineFeaturesLocked()
	if service.inferredPermissionDenial {
		return Status{
			Code:               StatusPermissionNeeded,
			Running:            false,
			Supported:          true,
			PermissionRequired: true,
			Message:            "Open System Settings > Privacy & Security > Screen & System Audio Recording and enable XiaDown, then retry playback or toggle the equalizer off and on.",
		}
	}
	if !service.shouldRunEngineLocked() {
		return Status{Code: StatusOff, Supported: true}
	}
	if service.engine.IsRunning() {
		message := "Equalizer is processing XiaDown Listen audio."
		if !features.Equalizer && features.Visualizer {
			message = "Visualizer is analyzing XiaDown Listen audio."
		}
		return Status{
			Code:      StatusActive,
			Running:   true,
			Supported: true,
			Message:   message,
		}
	}
	if service.lastFailure == nil {
		message := "The equalizer activates as soon as Listen playback starts."
		if !features.Equalizer && features.Visualizer {
			message = "The visualizer activates as soon as Listen playback starts."
		}
		return Status{
			Code:      StatusStandby,
			Supported: true,
			Message:   message,
		}
	}
	if features.Equalizer && service.lastFailure.IsPermissionLikely() {
		return Status{
			Code:               StatusPermissionNeeded,
			Supported:          true,
			PermissionRequired: true,
			Message:            service.lastFailure.userFacingMessage(),
			Detail:             service.lastFailure.Detail,
		}
	}
	return Status{
		Code:      StatusError,
		Supported: true,
		Message:   service.lastFailure.userFacingMessage(),
		Detail:    service.lastFailure.Detail,
	}
}

func (service *Service) shouldRunEngineLocked() bool {
	if service.engine == nil || !service.engine.Supported() {
		return false
	}
	features := service.engineFeaturesLocked()
	if features.Equalizer {
		return service.settings.Enabled
	}
	return features.Visualizer && service.settings.VisualizerMode != VisualizerModeOff
}

func (service *Service) engineFeaturesLocked() EngineFeatures {
	if service.engine == nil {
		return EngineFeatures{}
	}
	return service.engine.Features()
}

func (service *Service) shouldWaitForPlaybackBeforeStartLocked(playbackKnownActive bool) bool {
	features := service.engineFeaturesLocked()
	return !features.Equalizer && features.Visualizer && !playbackKnownActive && !service.playbackActive
}

func (service *Service) shouldStopOnInactivePlaybackLocked() bool {
	features := service.engineFeaturesLocked()
	return !features.Equalizer && features.Visualizer
}

func (failure StartFailure) userFacingMessage() string {
	switch failure.Code {
	case StartFailureNoAudioSource:
		return "Couldn't find XiaDown's active audio output. Start Listen playback, then retry the equalizer."
	case StartFailurePermissionDenied:
		return "Open System Settings > Privacy & Security > Screen & System Audio Recording and enable XiaDown."
	case StartFailureTapCreation:
		if failure.Detail != "" {
			return "Couldn't capture XiaDown's audio. Check Screen & System Audio Recording permission."
		}
		return "Couldn't capture XiaDown's audio."
	case StartFailureAggregateDeviceCreation:
		return "Couldn't create the equalizer audio device. Restarting XiaDown usually fixes this."
	case StartFailureInvalidTapFormat:
		return "The system didn't report a valid audio format for XiaDown's output."
	case StartFailureIOProcInstall:
		return "Couldn't install the equalizer audio I/O callback."
	case StartFailureEngineStart:
		return "The equalizer audio engine failed to start."
	case StartFailureUnsupported:
		return "This audio feature is unavailable on this system."
	default:
		return "The equalizer engine failed."
	}
}
