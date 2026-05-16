package service

import (
	"context"
	"fmt"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func (service *LibraryService) ProbeTranscodeInput(ctx context.Context, request dto.ProbeTranscodeInputRequest) (dto.ProbeTranscodeInputResponse, error) {
	sourcePath, err := service.resolveTranscodeProbePath(ctx, request)
	if err != nil {
		return dto.ProbeTranscodeInputResponse{}, err
	}
	probe, err := service.probeRequiredMedia(ctx, sourcePath)
	if err != nil {
		return dto.ProbeTranscodeInputResponse{}, err
	}
	presets, err := service.listTranscodePresetModels(ctx)
	if err != nil {
		return dto.ProbeTranscodeInputResponse{}, err
	}
	response := service.buildTranscodeInputProbeResponse(ctx, probe, presets)
	return response, nil
}

func (service *LibraryService) resolveTranscodeProbePath(ctx context.Context, request dto.ProbeTranscodeInputRequest) (string, error) {
	if fileID := strings.TrimSpace(request.FileID); fileID != "" {
		if service == nil || service.files == nil {
			return "", fmt.Errorf("library file repository not configured")
		}
		fileItem, err := service.files.Get(ctx, fileID)
		if err != nil {
			return "", err
		}
		localPath := strings.TrimSpace(fileItem.Storage.LocalPath)
		if localPath == "" {
			return "", fmt.Errorf("source file has no local path")
		}
		return localPath, nil
	}
	return service.resolveInputPath(ctx, request.InputPath, request.Source, false)
}

func (service *LibraryService) buildTranscodeInputProbeResponse(ctx context.Context, probe mediaProbe, presets []library.TranscodePreset) dto.ProbeTranscodeInputResponse {
	compatibleIDs, compatibility := resolveTranscodePresetCompatibility(presets, probe)
	recommendedID := service.recommendedTranscodePresetID(ctx, probe, compatibleIDs)
	return dto.ProbeTranscodeInputResponse{
		Media:               toLibraryMediaInfoDTO(probe.toMediaInfo()),
		MediaType:           resolveTranscodeProbeMediaType(probe),
		CompatiblePresetIDs: compatibleIDs,
		PresetCompatibility: compatibility,
		RecommendedPresetID: recommendedID,
	}
}

func resolveTranscodePresetCompatibility(presets []library.TranscodePreset, probe mediaProbe) ([]string, []dto.TranscodePresetCompatibilityDTO) {
	compatibleIDs := make([]string, 0, len(presets))
	compatibility := make([]dto.TranscodePresetCompatibilityDTO, 0, len(presets))
	for _, preset := range presets {
		item := dto.TranscodePresetCompatibilityDTO{PresetID: preset.ID, Compatible: true}
		if err := validatePresetForProbe(preset, probe); err != nil {
			item.Compatible = false
			item.Reason = err.Error()
		} else {
			compatibleIDs = append(compatibleIDs, preset.ID)
		}
		compatibility = append(compatibility, item)
	}
	return compatibleIDs, compatibility
}

func (service *LibraryService) recommendedTranscodePresetID(ctx context.Context, probe mediaProbe, compatibleIDs []string) string {
	if len(compatibleIDs) == 0 {
		return ""
	}
	compatible := make(map[string]struct{}, len(compatibleIDs))
	for _, id := range compatibleIDs {
		compatible[id] = struct{}{}
	}
	if preset, err := service.selectDefaultPreset(ctx, probe); err == nil {
		if _, ok := compatible[preset.ID]; ok {
			return preset.ID
		}
	}
	return compatibleIDs[0]
}

func resolveTranscodeProbeMediaType(probe mediaProbe) string {
	if mediaProbeHasVideo(probe) {
		return "video"
	}
	if mediaProbeHasAudio(probe) {
		return "audio"
	}
	return "unknown"
}

func toLibraryMediaInfoDTO(media library.MediaInfo) dto.LibraryMediaInfoDTO {
	return dto.LibraryMediaInfoDTO{
		Format:           media.Format,
		Codec:            media.Codec,
		VideoCodec:       media.VideoCodec,
		AudioCodec:       media.AudioCodec,
		DurationMs:       media.DurationMs,
		Width:            media.Width,
		Height:           media.Height,
		FrameRate:        media.FrameRate,
		BitrateKbps:      media.BitrateKbps,
		VideoBitrateKbps: media.VideoBitrateKbps,
		AudioBitrateKbps: media.AudioBitrateKbps,
		Channels:         media.Channels,
		SizeBytes:        media.SizeBytes,
		DPI:              media.DPI,
	}
}
