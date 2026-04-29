package service

import (
	"context"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestDefaultTranscodePresetsExposeExpandedBuiltinSet(t *testing.T) {
	presets := defaultTranscodePresets(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if len(presets) != 52 {
		t.Fatalf("expected 52 builtin transcode presets, got %d", len(presets))
	}

	seen := make(map[string]struct{}, len(presets))
	for _, preset := range presets {
		if _, ok := seen[preset.ID]; ok {
			t.Fatalf("duplicate preset id detected: %s", preset.ID)
		}
		seen[preset.ID] = struct{}{}
	}

	required := []string{
		"builtin-video-h264-mp4-original",
		"builtin-video-h265-mov-2160p",
		"builtin-video-vp9-mkv-1080p",
		"builtin-video-vp9-webm-720p",
		"builtin-audio-mp3-192k",
		"builtin-audio-aac-m4a-256k",
		"builtin-audio-opus-ogg-128k",
		"builtin-audio-flac-lossless",
		"builtin-audio-wav-pcm",
	}
	for _, id := range required {
		if _, ok := seen[id]; !ok {
			t.Fatalf("expected builtin preset %s to exist", id)
		}
	}
}

func TestDefaultTranscodePresetsPreferHighQualityDefaults(t *testing.T) {
	presets := defaultTranscodePresets(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	byID := make(map[string]library.TranscodePreset, len(presets))
	for _, preset := range presets {
		byID[preset.ID] = preset
	}

	h264 := byID["builtin-video-h264-mp4-original"]
	if h264.CRF != defaultH264VideoCRF {
		t.Fatalf("expected H.264 builtin CRF %d, got %d", defaultH264VideoCRF, h264.CRF)
	}
	if h264.AudioBitrateKbps != defaultAACAudioBitrateKbps {
		t.Fatalf("expected H.264 builtin AAC bitrate %d, got %d", defaultAACAudioBitrateKbps, h264.AudioBitrateKbps)
	}
	if h264.FFmpegPreset != defaultFFmpegPreset {
		t.Fatalf("expected H.264 builtin preset %q, got %q", defaultFFmpegPreset, h264.FFmpegPreset)
	}

	vp9 := byID["builtin-video-vp9-webm-original"]
	if vp9.CRF != defaultVP9VideoCRF {
		t.Fatalf("expected VP9 builtin CRF %d, got %d", defaultVP9VideoCRF, vp9.CRF)
	}
	if vp9.AudioBitrateKbps != defaultOpusAudioBitrateKbps {
		t.Fatalf("expected VP9 builtin Opus bitrate %d, got %d", defaultOpusAudioBitrateKbps, vp9.AudioBitrateKbps)
	}
}

func TestListTranscodePresetsOverridesBuiltinRowsWithBackendDefaults(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &transcodePresetTestRepo{
		items: []library.TranscodePreset{
			mustNewTranscodePreset(t, library.TranscodePresetParams{
				ID:               "builtin-video-h264-mp4-original",
				Name:             "Mutated Builtin",
				OutputType:       "video",
				Container:        "mp4",
				VideoCodec:       "h264",
				AudioCodec:       "aac",
				QualityMode:      "crf",
				CRF:              35,
				AudioBitrateKbps: 64,
				FFmpegPreset:     "ultrafast",
				RequiresVideo:    true,
				IsBuiltin:        true,
				CreatedAt:        &now,
				UpdatedAt:        &now,
			}),
			mustNewTranscodePreset(t, library.TranscodePresetParams{
				ID:               "custom-audio",
				Name:             "Custom Audio",
				OutputType:       "audio",
				Container:        "mp3",
				AudioCodec:       "mp3",
				AudioBitrateKbps: 160,
				RequiresAudio:    true,
				CreatedAt:        &now,
				UpdatedAt:        &now,
			}),
		},
	}
	service := &LibraryService{
		presets: repo,
		nowFunc: func() time.Time { return now },
	}

	presets, err := service.ListTranscodePresets(context.Background())
	if err != nil {
		t.Fatalf("ListTranscodePresets returned error: %v", err)
	}

	byID := make(map[string]library.TranscodePreset, len(presets))
	for _, preset := range presets {
		model, err := library.NewTranscodePreset(library.TranscodePresetParams{
			ID:               preset.ID,
			Name:             preset.Name,
			OutputType:       preset.OutputType,
			Container:        preset.Container,
			VideoCodec:       preset.VideoCodec,
			AudioCodec:       preset.AudioCodec,
			QualityMode:      preset.QualityMode,
			CRF:              preset.CRF,
			BitrateKbps:      preset.BitrateKbps,
			AudioBitrateKbps: preset.AudioBitrateKbps,
			Scale:            preset.Scale,
			Width:            preset.Width,
			Height:           preset.Height,
			FFmpegPreset:     preset.FFmpegPreset,
			AllowUpscale:     preset.AllowUpscale,
			RequiresVideo:    preset.RequiresVideo,
			RequiresAudio:    preset.RequiresAudio,
			IsBuiltin:        preset.IsBuiltin,
			CreatedAt:        &now,
			UpdatedAt:        &now,
		})
		if err != nil {
			t.Fatalf("failed to normalize DTO back to model: %v", err)
		}
		byID[preset.ID] = model
	}

	builtin := byID["builtin-video-h264-mp4-original"]
	if builtin.Name != "H.264 MP4 Original" {
		t.Fatalf("expected builtin name from backend defaults, got %q", builtin.Name)
	}
	if builtin.CRF != defaultH264VideoCRF || builtin.AudioBitrateKbps != defaultAACAudioBitrateKbps {
		t.Fatalf("expected builtin quality defaults to override sqlite row, got %#v", builtin)
	}
	if builtin.FFmpegPreset != defaultFFmpegPreset {
		t.Fatalf("expected builtin ffmpeg preset %q, got %q", defaultFFmpegPreset, builtin.FFmpegPreset)
	}

	custom := byID["custom-audio"]
	if custom.Name != "Custom Audio" || custom.AudioBitrateKbps != 160 {
		t.Fatalf("expected custom preset to remain unchanged, got %#v", custom)
	}
}

type transcodePresetTestRepo struct {
	items []library.TranscodePreset
}

func (repo *transcodePresetTestRepo) List(_ context.Context) ([]library.TranscodePreset, error) {
	return append([]library.TranscodePreset(nil), repo.items...), nil
}

func (repo *transcodePresetTestRepo) Get(_ context.Context, id string) (library.TranscodePreset, error) {
	for _, item := range repo.items {
		if item.ID == id {
			return item, nil
		}
	}
	return library.TranscodePreset{}, library.ErrPresetNotFound
}

func (repo *transcodePresetTestRepo) Save(_ context.Context, preset library.TranscodePreset) error {
	for index := range repo.items {
		if repo.items[index].ID == preset.ID {
			repo.items[index] = preset
			return nil
		}
	}
	repo.items = append(repo.items, preset)
	return nil
}

func (repo *transcodePresetTestRepo) Delete(_ context.Context, id string) error {
	filtered := repo.items[:0]
	for _, item := range repo.items {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	repo.items = filtered
	return nil
}

func mustNewTranscodePreset(t *testing.T, params library.TranscodePresetParams) library.TranscodePreset {
	t.Helper()
	preset, err := library.NewTranscodePreset(params)
	if err != nil {
		t.Fatalf("NewTranscodePreset returned error: %v", err)
	}
	return preset
}
