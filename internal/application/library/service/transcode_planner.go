package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type transcodePlan struct {
	request     dto.CreateTranscodeJobRequest
	preset      *library.TranscodePreset
	outputType  library.TranscodeOutputType
	sourceProbe mediaProbe
}

type containerCompat struct {
	videoCodecs map[string]struct{}
	audioCodecs map[string]struct{}
}

var transcodeContainerCompat = map[string]containerCompat{
	"mp4":  {videoCodecs: setOf("h264", "h265"), audioCodecs: setOf("aac", "mp3", "copy")},
	"mov":  {videoCodecs: setOf("h264", "h265"), audioCodecs: setOf("aac", "mp3", "copy")},
	"mkv":  {videoCodecs: setOf("h264", "h265", "vp9", "copy"), audioCodecs: setOf("aac", "mp3", "opus", "copy")},
	"webm": {videoCodecs: setOf("vp9"), audioCodecs: setOf("opus", "copy")},
	"mp3":  {videoCodecs: setOf(), audioCodecs: setOf("mp3")},
	"m4a":  {videoCodecs: setOf(), audioCodecs: setOf("aac", "mp3", "copy")},
	"ogg":  {videoCodecs: setOf(), audioCodecs: setOf("opus", "copy")},
	"flac": {videoCodecs: setOf(), audioCodecs: setOf("flac", "copy")},
	"wav":  {videoCodecs: setOf(), audioCodecs: setOf("pcm", "copy")},
}

var audioContainers = map[string]struct{}{
	"mp3":  {},
	"m4a":  {},
	"wav":  {},
	"flac": {},
	"opus": {},
	"ogg":  {},
}

var scaleTargets = map[string][2]int{
	"2160p": {3840, 2160},
	"1080p": {1920, 1080},
	"720p":  {1280, 720},
	"480p":  {854, 480},
}

func (service *LibraryService) resolveTranscodePlan(ctx context.Context, request dto.CreateTranscodeJobRequest, probe mediaProbe) (transcodePlan, error) {
	presetID := strings.TrimSpace(request.PresetID)
	if presetID != "" {
		preset, err := service.getTranscodePreset(ctx, presetID)
		if err != nil {
			return transcodePlan{}, err
		}
		if err := validatePresetForProbe(preset, probe); err != nil {
			return transcodePlan{}, err
		}
		resolved := applyPresetToRequest(request, preset)
		return transcodePlan{request: resolved, preset: &preset, outputType: preset.OutputType, sourceProbe: probe}, nil
	}

	if hasManualTranscodeConfig(request) {
		preset, err := presetFromRequest(request)
		if err != nil {
			return transcodePlan{}, err
		}
		if err := validatePresetForProbe(preset, probe); err != nil {
			return transcodePlan{}, err
		}
		return transcodePlan{request: request, preset: &preset, outputType: preset.OutputType, sourceProbe: probe}, nil
	}

	preset, err := service.selectDefaultPreset(ctx, probe)
	if err != nil {
		return transcodePlan{}, err
	}
	if err := validatePresetForProbe(preset, probe); err != nil {
		return transcodePlan{}, err
	}
	resolved := applyPresetToRequest(request, preset)
	return transcodePlan{request: resolved, preset: &preset, outputType: preset.OutputType, sourceProbe: probe}, nil
}

func (service *LibraryService) resolveTranscodePlanWithoutProbe(ctx context.Context, request dto.CreateTranscodeJobRequest, sourcePath string) (transcodePlan, error) {
	presetID := strings.TrimSpace(request.PresetID)
	if presetID != "" {
		preset, err := service.getTranscodePreset(ctx, presetID)
		if err != nil {
			return transcodePlan{}, err
		}
		resolved := applyPresetToRequest(request, preset)
		return transcodePlan{request: resolved, preset: &preset, outputType: preset.OutputType}, nil
	}

	if hasManualTranscodeConfig(request) {
		preset, err := presetFromRequest(request)
		if err != nil {
			return transcodePlan{}, err
		}
		return transcodePlan{request: request, preset: &preset, outputType: preset.OutputType}, nil
	}

	defaultID := "builtin-video-h264-mp4-original"
	format := normalizeTranscodeFormat(request.Format)
	if isAudioContainer(format) || isAudioContainer(normalizeFileExtension(sourcePath)) {
		defaultID = "builtin-audio-mp3-320k"
	}
	preset, err := service.lookupDefaultPreset(ctx, defaultID)
	if err != nil {
		return transcodePlan{}, err
	}
	resolved := applyPresetToRequest(request, preset)
	return transcodePlan{request: resolved, preset: &preset, outputType: preset.OutputType}, nil
}

func (service *LibraryService) selectDefaultPreset(ctx context.Context, probe mediaProbe) (library.TranscodePreset, error) {
	hasVideo := probe.Width > 0 || probe.Height > 0 || strings.TrimSpace(probe.VideoCodec) != ""
	hasAudio := strings.TrimSpace(probe.AudioCodec) != "" || probe.Channels > 0
	if !hasVideo && !hasAudio {
		return library.TranscodePreset{}, fmt.Errorf("no media streams detected")
	}
	defaultID := "builtin-video-h264-mp4-original"
	if !hasVideo && hasAudio {
		defaultID = "builtin-audio-mp3-320k"
	}
	return service.lookupDefaultPreset(ctx, defaultID)
}

func (service *LibraryService) lookupDefaultPreset(ctx context.Context, defaultID string) (library.TranscodePreset, error) {
	return service.getTranscodePreset(ctx, defaultID)
}

func resolveTranscodeDisplayName(request dto.CreateTranscodeJobRequest, sourceFile library.LibraryFile, preset *library.TranscodePreset) string {
	title := resolveLibraryFileTitle(sourceFile, strings.TrimSpace(request.Title))
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(sourceFile.Storage.LocalPath), filepath.Ext(sourceFile.Storage.LocalPath))
	}
	return appendTranscodePresetSuffix(title, resolveTranscodePresetLabel(preset))
}

func resolveTranscodeOutputName(sourcePath string, preset *library.TranscodePreset) string {
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	return appendTranscodePresetSuffix(base, resolveTranscodePresetLabel(preset))
}

func resolveTranscodePresetLabel(preset *library.TranscodePreset) string {
	if preset == nil {
		return ""
	}
	return strings.TrimSpace(preset.Name)
}

func appendTranscodePresetSuffix(base string, suffix string) string {
	trimmedBase := strings.TrimSpace(base)
	trimmedSuffix := strings.TrimSpace(suffix)
	if trimmedBase == "" || trimmedSuffix == "" {
		return trimmedBase
	}
	if hasSuffixFold(trimmedBase, trimmedSuffix) {
		return trimmedBase
	}
	return fmt.Sprintf("%s - %s", trimmedBase, trimmedSuffix)
}

func hasSuffixFold(value string, suffix string) bool {
	if suffix == "" {
		return false
	}
	if len(value) < len(suffix) {
		return false
	}
	return strings.EqualFold(value[len(value)-len(suffix):], suffix)
}

func hasManualTranscodeConfig(request dto.CreateTranscodeJobRequest) bool {
	if strings.TrimSpace(request.Format) != "" {
		return true
	}
	if strings.TrimSpace(request.VideoCodec) != "" {
		return true
	}
	if strings.TrimSpace(request.AudioCodec) != "" {
		return true
	}
	if strings.TrimSpace(request.QualityMode) != "" {
		return true
	}
	if request.CRF > 0 || request.BitrateKbps > 0 || request.AudioBitrateKbps > 0 {
		return true
	}
	if strings.TrimSpace(request.Scale) != "" || request.Width > 0 || request.Height > 0 {
		return true
	}
	if strings.TrimSpace(request.Preset) != "" {
		return true
	}
	return false
}

func presetFromRequest(request dto.CreateTranscodeJobRequest) (library.TranscodePreset, error) {
	outputType := inferOutputType(request)
	container := normalizeTranscodeFormat(request.Format)
	if strings.TrimSpace(request.Format) == "" && outputType == library.TranscodeOutputAudio {
		container = "mp3"
	}
	audioCodec := strings.TrimSpace(request.AudioCodec)
	if outputType == library.TranscodeOutputAudio && audioCodec == "" {
		audioCodec = defaultAudioCodecForContainer(container)
	}
	return library.NewTranscodePreset(library.TranscodePresetParams{
		ID:               "manual",
		Name:             "Manual",
		OutputType:       string(outputType),
		Container:        container,
		VideoCodec:       strings.TrimSpace(request.VideoCodec),
		AudioCodec:       audioCodec,
		QualityMode:      strings.TrimSpace(request.QualityMode),
		CRF:              request.CRF,
		BitrateKbps:      request.BitrateKbps,
		AudioBitrateKbps: request.AudioBitrateKbps,
		Scale:            strings.TrimSpace(request.Scale),
		Width:            request.Width,
		Height:           request.Height,
		FFmpegPreset:     strings.TrimSpace(request.Preset),
		AllowUpscale:     false,
		RequiresVideo:    outputType == library.TranscodeOutputVideo,
		RequiresAudio:    outputType == library.TranscodeOutputAudio,
		IsBuiltin:        false,
	})
}

func applyPresetToRequest(request dto.CreateTranscodeJobRequest, preset library.TranscodePreset) dto.CreateTranscodeJobRequest {
	resolved := request
	resolved.Format = preset.Container
	resolved.VideoCodec = preset.VideoCodec
	resolved.AudioCodec = preset.AudioCodec
	resolved.QualityMode = preset.QualityMode
	resolved.CRF = preset.CRF
	resolved.BitrateKbps = preset.BitrateKbps
	resolved.AudioBitrateKbps = preset.AudioBitrateKbps
	resolved.Scale = preset.Scale
	resolved.Width = preset.Width
	resolved.Height = preset.Height
	resolved.Preset = preset.FFmpegPreset
	if preset.OutputType == library.TranscodeOutputAudio {
		resolved.VideoCodec = ""
		resolved.QualityMode = ""
		resolved.CRF = 0
		resolved.BitrateKbps = 0
		resolved.Scale = ""
		resolved.Width = 0
		resolved.Height = 0
		resolved.Preset = ""
	}
	return resolved
}

func inferOutputType(request dto.CreateTranscodeJobRequest) library.TranscodeOutputType {
	format := normalizeTranscodeFormat(request.Format)
	if isAudioContainer(format) {
		return library.TranscodeOutputAudio
	}
	if strings.TrimSpace(request.VideoCodec) == "" && strings.TrimSpace(request.AudioCodec) != "" {
		return library.TranscodeOutputAudio
	}
	return library.TranscodeOutputVideo
}

func validatePresetForProbe(preset library.TranscodePreset, probe mediaProbe) error {
	hasVideo := probe.Width > 0 || probe.Height > 0 || strings.TrimSpace(probe.VideoCodec) != ""
	hasAudio := strings.TrimSpace(probe.AudioCodec) != "" || probe.Channels > 0
	if preset.OutputType == library.TranscodeOutputVideo && !hasVideo {
		return fmt.Errorf("input has no video stream")
	}
	if preset.OutputType == library.TranscodeOutputAudio && !hasAudio {
		return fmt.Errorf("input has no audio stream")
	}
	if preset.RequiresVideo && !hasVideo {
		return fmt.Errorf("preset requires video stream")
	}
	if preset.RequiresAudio && !hasAudio {
		return fmt.Errorf("preset requires audio stream")
	}
	if err := validatePresetScale(preset, probe); err != nil {
		return err
	}
	return validatePresetCodecs(preset, probe, hasVideo, hasAudio)
}

func validatePresetScale(preset library.TranscodePreset, probe mediaProbe) error {
	if preset.OutputType != library.TranscodeOutputVideo {
		return nil
	}
	targetWidth, targetHeight, err := resolvePresetScale(preset)
	if err != nil {
		return err
	}
	if targetWidth == 0 || targetHeight == 0 {
		return nil
	}
	if probe.Width <= 0 || probe.Height <= 0 {
		return nil
	}
	inputShort := minInt(probe.Width, probe.Height)
	targetShort := minInt(targetWidth, targetHeight)
	if targetShort > inputShort && !preset.AllowUpscale {
		return fmt.Errorf("preset resolution exceeds input size")
	}
	return nil
}

func resolvePresetScale(preset library.TranscodePreset) (int, int, error) {
	scale := strings.ToLower(strings.TrimSpace(preset.Scale))
	if scale == "" || scale == "original" {
		return 0, 0, nil
	}
	if scale == "custom" {
		if preset.Width <= 0 || preset.Height <= 0 {
			return 0, 0, fmt.Errorf("custom scale requires width and height")
		}
		return preset.Width, preset.Height, nil
	}
	if target, ok := scaleTargets[scale]; ok {
		return target[0], target[1], nil
	}
	return 0, 0, fmt.Errorf("unsupported scale preset")
}

func validatePresetCodecs(preset library.TranscodePreset, probe mediaProbe, hasVideo bool, hasAudio bool) error {
	container := normalizeContainer(preset.Container)
	videoCodec := normalizeVideoCodecName(preset.VideoCodec)
	audioCodec := normalizeAudioCodecName(preset.AudioCodec)

	if preset.OutputType == library.TranscodeOutputVideo {
		if videoCodec == "" {
			videoCodec = "h264"
		}
		if videoCodec == "copy" {
			if !hasVideo {
				return fmt.Errorf("no video stream to copy")
			}
			inputCodec := normalizeVideoCodecName(probe.VideoCodec)
			if inputCodec == "" {
				return fmt.Errorf("unable to detect input video codec")
			}
			if !supportsVideoCodec(container, inputCodec) {
				return fmt.Errorf("container does not support input video codec")
			}
		} else if !supportsVideoCodec(container, videoCodec) {
			return fmt.Errorf("container does not support video codec")
		}
	}

	if audioCodec == "" {
		if preset.OutputType == library.TranscodeOutputAudio {
			audioCodec = defaultAudioCodecForContainer(container)
			if audioCodec == "" {
				return fmt.Errorf("audio codec is required")
			}
		} else {
			audioCodec = "copy"
		}
	}
	if audioCodec == "copy" {
		if !hasAudio {
			if preset.OutputType == library.TranscodeOutputAudio || preset.RequiresAudio {
				return fmt.Errorf("no audio stream to copy")
			}
			return nil
		}
		inputCodec := normalizeAudioCodecName(probe.AudioCodec)
		if inputCodec == "" {
			return fmt.Errorf("unable to detect input audio codec")
		}
		if !supportsAudioCodec(container, inputCodec) {
			return fmt.Errorf("container does not support input audio codec")
		}
		return nil
	}
	if !supportsAudioCodec(container, audioCodec) {
		return fmt.Errorf("container does not support audio codec")
	}
	return nil
}

func normalizeContainer(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func supportsVideoCodec(container string, codec string) bool {
	if codec == "" {
		return false
	}
	compat, ok := transcodeContainerCompat[container]
	if !ok {
		return true
	}
	if len(compat.videoCodecs) == 0 {
		return false
	}
	_, ok = compat.videoCodecs[codec]
	return ok
}

func supportsAudioCodec(container string, codec string) bool {
	if codec == "" {
		return false
	}
	compat, ok := transcodeContainerCompat[container]
	if !ok {
		return true
	}
	if len(compat.audioCodecs) == 0 {
		return false
	}
	_, ok = compat.audioCodecs[codec]
	return ok
}

func isAudioContainer(format string) bool {
	_, ok := audioContainers[normalizeContainer(format)]
	return ok
}

func normalizeVideoCodecName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "h265", "hevc":
		return "h265"
	case "h264":
		return "h264"
	case "vp9":
		return "vp9"
	case "copy":
		return "copy"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeAudioCodecName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "aac":
		return "aac"
	case "mp3":
		return "mp3"
	case "opus":
		return "opus"
	case "flac":
		return "flac"
	case "pcm", "pcm_s16le", "wav":
		return "pcm"
	case "copy":
		return "copy"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func defaultAudioCodecForContainer(container string) string {
	switch normalizeContainer(container) {
	case "mp3":
		return "mp3"
	case "m4a":
		return "aac"
	case "ogg", "opus":
		return "opus"
	case "flac":
		return "flac"
	case "wav":
		return "pcm"
	default:
		return ""
	}
}

func setOf(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
