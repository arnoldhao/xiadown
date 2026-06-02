package equalizer

import "math"

const (
	MinGainDB = -12
	MaxGainDB = 12
)

type BandType string

const (
	BandTypeLowShelf  BandType = "lowShelf"
	BandTypePeaking   BandType = "peaking"
	BandTypeHighShelf BandType = "highShelf"
)

type Band struct {
	ID          string   `json:"id"`
	Frequency   float64  `json:"frequencyHz"`
	Q           float64  `json:"q"`
	Type        BandType `json:"type"`
	Display     string   `json:"display"`
	DisplayHz   string   `json:"displayHz"`
	Description string   `json:"description,omitempty"`
}

type Preset struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	GainsDB []float64 `json:"gainsDb"`
}

type Settings struct {
	Enabled                bool                `json:"enabled"`
	PreampDB               float64             `json:"preampDb"`
	BandGainsDB            []float64           `json:"bandGainsDb"`
	Preset                 string              `json:"preset"`
	VisualizerMode         VisualizerMode      `json:"visualizerMode"`
	VisualizerPlacement    VisualizerPlacement `json:"visualizerPlacement"`
	ArtworkVisualizerMode  VisualizerMode      `json:"artworkVisualizerMode"`
	SpectrumVisualizerMode VisualizerMode      `json:"spectrumVisualizerMode"`
}

type VisualizerMode string

const (
	VisualizerModeOff        VisualizerMode = "off"
	VisualizerModeHalo       VisualizerMode = "halo"
	VisualizerModeRing       VisualizerMode = "ring"
	VisualizerModeNeonPulse  VisualizerMode = "neonPulse"
	VisualizerModePondRipple VisualizerMode = "pondRipple"
	VisualizerModeBars       VisualizerMode = "bars"
	VisualizerModeMirror     VisualizerMode = "mirror"
	VisualizerModeWaveform   VisualizerMode = "waveform"
	VisualizerModeHeatmap    VisualizerMode = "heatmap"
)

type VisualizerPlacement string

const (
	VisualizerPlacementOff      VisualizerPlacement = "off"
	VisualizerPlacementArtwork  VisualizerPlacement = "artwork"
	VisualizerPlacementSpectrum VisualizerPlacement = "spectrum"
)

const (
	VisualizerBandCount     = 32
	VisualizerWaveformCount = 64
)

type StatusCode string

const (
	StatusOff              StatusCode = "off"
	StatusActive           StatusCode = "active"
	StatusStandby          StatusCode = "standby"
	StatusPermissionNeeded StatusCode = "permission_needed"
	StatusUnsupported      StatusCode = "unsupported"
	StatusError            StatusCode = "error"
)

type Status struct {
	Code               StatusCode `json:"code"`
	Running            bool       `json:"running"`
	Supported          bool       `json:"supported"`
	PermissionRequired bool       `json:"permissionRequired"`
	Message            string     `json:"message,omitempty"`
	Detail             string     `json:"detail,omitempty"`
}

type Snapshot struct {
	Settings Settings `json:"settings"`
	Status   Status   `json:"status"`
	Bands    []Band   `json:"bands"`
	Presets  []Preset `json:"presets"`
}

type EngineFeatures struct {
	Equalizer  bool `json:"equalizer"`
	Visualizer bool `json:"visualizer"`
}

type VisualizerFrame struct {
	Running                bool      `json:"running"`
	Sequence               uint64    `json:"sequence"`
	Level                  float64   `json:"level"`
	Bands                  []float64 `json:"bands"`
	Waveform               []float64 `json:"waveform"`
	AnalysisTimeSeconds    float64   `json:"analysisTimeSeconds"`
	FrameTimeOffsetSeconds float64   `json:"frameTimeOffsetSeconds"`
}

type Store interface {
	Load() (Settings, bool, error)
	Save(Settings) error
}

type StartFailureCode string

const (
	StartFailureUnsupported             StartFailureCode = "unsupported"
	StartFailurePermissionDenied        StartFailureCode = "permission_denied"
	StartFailureNoAudioSource           StartFailureCode = "no_audio_source"
	StartFailureTapCreation             StartFailureCode = "tap_creation"
	StartFailureAggregateDeviceCreation StartFailureCode = "aggregate_device_creation"
	StartFailureInvalidTapFormat        StartFailureCode = "invalid_tap_format"
	StartFailureIOProcInstall           StartFailureCode = "io_proc_install"
	StartFailureEngineStart             StartFailureCode = "engine_start"
	StartFailureUnknown                 StartFailureCode = "unknown"
)

type StartFailure struct {
	Code   StartFailureCode
	Detail string
}

func (failure StartFailure) IsWaitingForPlayback() bool {
	return failure.Code == StartFailureNoAudioSource
}

func (failure StartFailure) IsPermissionLikely() bool {
	return failure.Code == StartFailurePermissionDenied || failure.Code == StartFailureTapCreation
}

type Engine interface {
	Features() EngineFeatures
	Supported() bool
	IsRunning() bool
	HasObservedAudio() bool
	VisualizerFrame() VisualizerFrame
	HasCapturePermission() bool
	RequestCapturePermission() bool
	Start(Settings) *StartFailure
	Apply(Settings)
	Stop()
}

var DefaultBands = []Band{
	{ID: "60", Frequency: 60, Q: 0.71, Type: BandTypeLowShelf, Display: "60", DisplayHz: "Hz"},
	{ID: "150", Frequency: 150, Q: 0.55, Type: BandTypePeaking, Display: "150", DisplayHz: "Hz"},
	{ID: "400", Frequency: 400, Q: 0.5, Type: BandTypePeaking, Display: "400", DisplayHz: "Hz"},
	{ID: "1000", Frequency: 1000, Q: 0.5, Type: BandTypePeaking, Display: "1K", DisplayHz: "Hz"},
	{ID: "2400", Frequency: 2400, Q: 0.55, Type: BandTypePeaking, Display: "2.4K", DisplayHz: "Hz"},
	{ID: "15000", Frequency: 15000, Q: 0.71, Type: BandTypeHighShelf, Display: "15K", DisplayHz: "Hz"},
}

var Presets = []Preset{
	{ID: "flat", Name: "Flat", GainsDB: []float64{0, 0, 0, 0, 0, 0}},
	{ID: "acoustic", Name: "Acoustic", GainsDB: []float64{4, 3, 2, 1, 2, 3}},
	{ID: "bassBooster", Name: "Bass Booster", GainsDB: []float64{5, 4, 3, 0, 0, 0}},
	{ID: "bassReducer", Name: "Bass Reducer", GainsDB: []float64{-5, -4, -3, -1, 0, 0}},
	{ID: "classical", Name: "Classical", GainsDB: []float64{4, 3, 0, 0, 2, 4}},
	{ID: "dance", Name: "Dance", GainsDB: []float64{4, 5, 2, -1, 2, 4}},
	{ID: "deep", Name: "Deep", GainsDB: []float64{4, 3, 1, 0, -2, -3}},
	{ID: "electronic", Name: "Electronic", GainsDB: []float64{4, 3, -1, 1, 2, 4}},
	{ID: "hipHop", Name: "Hip-Hop", GainsDB: []float64{5, 3, 1, 0, 1, 3}},
	{ID: "jazz", Name: "Jazz", GainsDB: []float64{3, 2, 1, 2, 1, 3}},
	{ID: "latin", Name: "Latin", GainsDB: []float64{4, 1, -1, 0, 1, 4}},
	{ID: "loudness", Name: "Loudness", GainsDB: []float64{5, 3, 0, 0, 2, 5}},
	{ID: "lounge", Name: "Lounge", GainsDB: []float64{-2, 0, 2, 3, 2, -1}},
	{ID: "piano", Name: "Piano", GainsDB: []float64{2, 1, 0, 2, 3, 2}},
	{ID: "pop", Name: "Pop", GainsDB: []float64{-1, 1, 3, 4, 2, -1}},
	{ID: "rnb", Name: "R&B", GainsDB: []float64{3, 4, 2, -1, 2, 3}},
	{ID: "rock", Name: "Rock", GainsDB: []float64{4, 3, -1, -1, 2, 4}},
	{ID: "smallSpeakers", Name: "Small Speakers", GainsDB: []float64{4, 3, 1, 0, -2, -3}},
	{ID: "spokenWord", Name: "Spoken Word", GainsDB: []float64{-2, -1, 1, 4, 3, -1}},
	{ID: "trebleBooster", Name: "Treble Booster", GainsDB: []float64{0, 0, 0, 1, 3, 5}},
	{ID: "trebleReducer", Name: "Treble Reducer", GainsDB: []float64{0, 0, 0, -1, -3, -5}},
	{ID: "vocalBooster", Name: "Vocal Booster", GainsDB: []float64{-1, 0, 3, 5, 3, 0}},
}

func FlatSettings() Settings {
	return Settings{
		Enabled:                false,
		PreampDB:               0,
		BandGainsDB:            make([]float64, len(DefaultBands)),
		Preset:                 "flat",
		VisualizerMode:         VisualizerModeRing,
		VisualizerPlacement:    VisualizerPlacementArtwork,
		ArtworkVisualizerMode:  VisualizerModeRing,
		SpectrumVisualizerMode: VisualizerModeBars,
	}
}

func ClampSettings(current Settings) Settings {
	current.PreampDB = clampGain(current.PreampDB)
	expected := len(DefaultBands)
	if len(current.BandGainsDB) < expected {
		next := make([]float64, expected)
		copy(next, current.BandGainsDB)
		current.BandGainsDB = next
	} else if len(current.BandGainsDB) > expected {
		current.BandGainsDB = append([]float64(nil), current.BandGainsDB[:expected]...)
	} else {
		current.BandGainsDB = append([]float64(nil), current.BandGainsDB...)
	}
	for index := range current.BandGainsDB {
		current.BandGainsDB[index] = clampGain(current.BandGainsDB[index])
	}
	if current.Preset == "" {
		current.Preset = "flat"
	}
	current = ClampVisualizerSettings(current)
	return current
}

func ClampVisualizerSettings(current Settings) Settings {
	activeMode := ClampVisualizerMode(current.VisualizerMode)
	if !isArtworkVisualizerMode(current.ArtworkVisualizerMode) {
		if isArtworkVisualizerMode(activeMode) {
			current.ArtworkVisualizerMode = activeMode
		} else {
			current.ArtworkVisualizerMode = VisualizerModeRing
		}
	}
	if !isSpectrumVisualizerMode(current.SpectrumVisualizerMode) {
		if isSpectrumVisualizerMode(activeMode) {
			current.SpectrumVisualizerMode = activeMode
		} else {
			current.SpectrumVisualizerMode = VisualizerModeBars
		}
	}
	switch current.VisualizerPlacement {
	case VisualizerPlacementOff,
		VisualizerPlacementArtwork,
		VisualizerPlacementSpectrum:
	default:
		current.VisualizerPlacement = VisualizerPlacementForMode(activeMode)
	}
	current.VisualizerMode = VisualizerModeForPlacement(
		current.VisualizerPlacement,
		current.ArtworkVisualizerMode,
		current.SpectrumVisualizerMode,
	)
	return current
}

func ClampVisualizerMode(mode VisualizerMode) VisualizerMode {
	switch mode {
	case VisualizerModeOff,
		VisualizerModeHalo,
		VisualizerModeRing,
		VisualizerModeNeonPulse,
		VisualizerModePondRipple,
		VisualizerModeBars,
		VisualizerModeMirror,
		VisualizerModeWaveform,
		VisualizerModeHeatmap:
		return mode
	default:
		return VisualizerModeRing
	}
}

func ClampVisualizerPlacement(placement VisualizerPlacement) VisualizerPlacement {
	switch placement {
	case VisualizerPlacementOff,
		VisualizerPlacementArtwork,
		VisualizerPlacementSpectrum:
		return placement
	default:
		return VisualizerPlacementArtwork
	}
}

func ApplyVisualizerMode(settings *Settings, mode VisualizerMode) {
	if settings == nil {
		return
	}
	mode = ClampVisualizerMode(mode)
	settings.VisualizerMode = mode
	settings.VisualizerPlacement = VisualizerPlacementForMode(mode)
	if isArtworkVisualizerMode(mode) {
		settings.ArtworkVisualizerMode = mode
	}
	if isSpectrumVisualizerMode(mode) {
		settings.SpectrumVisualizerMode = mode
	}
}

func VisualizerPlacementForMode(mode VisualizerMode) VisualizerPlacement {
	if mode == VisualizerModeOff {
		return VisualizerPlacementOff
	}
	if isSpectrumVisualizerMode(mode) {
		return VisualizerPlacementSpectrum
	}
	return VisualizerPlacementArtwork
}

func VisualizerModeForPlacement(
	placement VisualizerPlacement,
	artworkMode VisualizerMode,
	spectrumMode VisualizerMode,
) VisualizerMode {
	switch ClampVisualizerPlacement(placement) {
	case VisualizerPlacementOff:
		return VisualizerModeOff
	case VisualizerPlacementSpectrum:
		if isSpectrumVisualizerMode(spectrumMode) {
			return spectrumMode
		}
		return VisualizerModeBars
	default:
		if isArtworkVisualizerMode(artworkMode) {
			return artworkMode
		}
		return VisualizerModeRing
	}
}

func isArtworkVisualizerMode(mode VisualizerMode) bool {
	switch mode {
	case VisualizerModeHalo,
		VisualizerModeRing,
		VisualizerModeNeonPulse,
		VisualizerModePondRipple:
		return true
	default:
		return false
	}
}

func isSpectrumVisualizerMode(mode VisualizerMode) bool {
	switch mode {
	case VisualizerModeBars,
		VisualizerModeMirror,
		VisualizerModeWaveform,
		VisualizerModeHeatmap:
		return true
	default:
		return false
	}
}

func PresetByID(id string) (Preset, bool) {
	for _, preset := range Presets {
		if preset.ID == id {
			copyPreset := Preset{ID: preset.ID, Name: preset.Name, GainsDB: append([]float64(nil), preset.GainsDB...)}
			return copyPreset, true
		}
	}
	return Preset{}, false
}

func cloneBands() []Band {
	return append([]Band(nil), DefaultBands...)
}

func clonePresets() []Preset {
	result := make([]Preset, 0, len(Presets))
	for _, preset := range Presets {
		result = append(result, Preset{
			ID:      preset.ID,
			Name:    preset.Name,
			GainsDB: append([]float64(nil), preset.GainsDB...),
		})
	}
	return result
}

func cloneSettings(current Settings) Settings {
	current.BandGainsDB = append([]float64(nil), current.BandGainsDB...)
	return current
}

func clampGain(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Min(math.Max(value, MinGainDB), MaxGainDB)
}
