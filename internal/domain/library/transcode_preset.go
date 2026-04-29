package library

import (
	"strings"
	"time"
)

type TranscodeOutputType string

const (
	TranscodeOutputVideo TranscodeOutputType = "video"
	TranscodeOutputAudio TranscodeOutputType = "audio"
)

type TranscodePreset struct {
	ID               string
	Name             string
	OutputType       TranscodeOutputType
	Container        string
	VideoCodec       string
	AudioCodec       string
	QualityMode      string
	CRF              int
	BitrateKbps      int
	AudioBitrateKbps int
	Scale            string
	Width            int
	Height           int
	FFmpegPreset     string
	AllowUpscale     bool
	RequiresVideo    bool
	RequiresAudio    bool
	IsBuiltin        bool
	Description      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TranscodePresetParams struct {
	ID               string
	Name             string
	OutputType       string
	Container        string
	VideoCodec       string
	AudioCodec       string
	QualityMode      string
	CRF              int
	BitrateKbps      int
	AudioBitrateKbps int
	Scale            string
	Width            int
	Height           int
	FFmpegPreset     string
	AllowUpscale     bool
	RequiresVideo    bool
	RequiresAudio    bool
	IsBuiltin        bool
	Description      string
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
}

func NewTranscodePreset(params TranscodePresetParams) (TranscodePreset, error) {
	id := strings.TrimSpace(params.ID)
	if id == "" {
		return TranscodePreset{}, ErrInvalidPreset
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return TranscodePreset{}, ErrInvalidPreset
	}
	outputType := TranscodeOutputType(strings.ToLower(strings.TrimSpace(params.OutputType)))
	switch outputType {
	case TranscodeOutputVideo, TranscodeOutputAudio:
	default:
		return TranscodePreset{}, ErrInvalidPreset
	}
	container := strings.TrimSpace(params.Container)
	if container == "" {
		return TranscodePreset{}, ErrInvalidPreset
	}
	createdAt := time.Now()
	if params.CreatedAt != nil {
		createdAt = *params.CreatedAt
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil {
		updatedAt = *params.UpdatedAt
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return TranscodePreset{
		ID:               id,
		Name:             name,
		OutputType:       outputType,
		Container:        container,
		VideoCodec:       strings.TrimSpace(params.VideoCodec),
		AudioCodec:       strings.TrimSpace(params.AudioCodec),
		QualityMode:      strings.TrimSpace(params.QualityMode),
		CRF:              params.CRF,
		BitrateKbps:      params.BitrateKbps,
		AudioBitrateKbps: params.AudioBitrateKbps,
		Scale:            strings.TrimSpace(params.Scale),
		Width:            params.Width,
		Height:           params.Height,
		FFmpegPreset:     strings.TrimSpace(params.FFmpegPreset),
		AllowUpscale:     params.AllowUpscale,
		RequiresVideo:    params.RequiresVideo,
		RequiresAudio:    params.RequiresAudio,
		IsBuiltin:        params.IsBuiltin,
		Description:      strings.TrimSpace(params.Description),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}, nil
}
