package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func (service *LibraryService) GetWorkspaceProject(
	ctx context.Context,
	request dto.GetWorkspaceProjectRequest,
) (dto.WorkspaceProjectDTO, error) {
	item, err := service.libraries.Get(ctx, strings.TrimSpace(request.LibraryID))
	if err != nil {
		return dto.WorkspaceProjectDTO{}, err
	}
	return service.buildWorkspaceProjectDTO(ctx, item)
}

func (service *LibraryService) buildWorkspaceProjectDTO(
	ctx context.Context,
	item library.Library,
) (dto.WorkspaceProjectDTO, error) {
	files, err := service.files.ListByLibraryID(ctx, item.ID)
	if err != nil {
		return dto.WorkspaceProjectDTO{}, err
	}
	operations, err := service.operations.ListByLibraryID(ctx, item.ID)
	if err != nil {
		return dto.WorkspaceProjectDTO{}, err
	}
	moduleConfig, err := service.getModuleConfig(ctx)
	if err != nil {
		return dto.WorkspaceProjectDTO{}, err
	}
	var workspaceHead *dto.WorkspaceStateRecordDTO
	if service.workspace != nil {
		if head, headErr := service.workspace.GetHeadByLibraryID(ctx, item.ID); headErr == nil {
			mapped := toWorkspaceDTO(head)
			workspaceHead = &mapped
		} else if headErr != nil && headErr != library.ErrWorkspaceStateNotFound {
			return dto.WorkspaceProjectDTO{}, headErr
		}
	}
	taskBuckets := make(map[string]dto.WorkspaceTrackTasksDTO)
	for _, operation := range operations {
		if operation.Status != library.OperationStatusQueued && operation.Status != library.OperationStatusRunning {
			continue
		}
		fileID := resolveWorkspaceOperationSourceFileID(operation)
		if fileID == "" {
			continue
		}
		bucket := taskBuckets[fileID]
		if operation.Kind == "transcode" {
			task := toWorkspaceTaskSummaryDTO(operation)
			bucket.Transcode = &task
		}
		taskBuckets[fileID] = bucket
	}
	videoTracks := make([]dto.WorkspaceVideoTrackDTO, 0)
	subtitleTracks := make([]dto.WorkspaceSubtitleTrackDTO, 0)
	for _, file := range files {
		fileDTO, buildErr := service.buildFileDTOWithConfig(ctx, file, moduleConfig)
		if buildErr != nil {
			fileDTO = toLibraryFileDTO(file)
		}
		switch file.Kind {
		case library.FileKindVideo, library.FileKindAudio, library.FileKindTranscode:
			videoTracks = append(videoTracks, dto.WorkspaceVideoTrackDTO{
				TrackID: file.ID,
				File:    fileDTO,
				Display: buildWorkspaceVideoDisplay(fileDTO),
			})
		case library.FileKindSubtitle:
			subtitleTracks = append(subtitleTracks, dto.WorkspaceSubtitleTrackDTO{
				TrackID:      file.ID,
				Role:         resolveWorkspaceSubtitleRole(file),
				File:         fileDTO,
				Display:      buildWorkspaceSubtitleDisplay(fileDTO),
				RunningTasks: normalizeWorkspaceTrackTasks(taskBuckets[file.ID]),
			})
		}
	}
	sort.SliceStable(videoTracks, func(i, j int) bool {
		return videoTracks[i].File.CreatedAt > videoTracks[j].File.CreatedAt
	})
	sort.SliceStable(subtitleTracks, func(i, j int) bool {
		return subtitleTracks[i].File.CreatedAt > subtitleTracks[j].File.CreatedAt
	})
	updatedAt := item.UpdatedAt
	return dto.WorkspaceProjectDTO{
		Version:        dto.WorkspaceProjectSchemaVersion,
		LibraryID:      item.ID,
		Title:          item.Name,
		UpdatedAt:      updatedAt.Format(time.RFC3339),
		ViewStateHead:  workspaceHead,
		VideoTracks:    videoTracks,
		SubtitleTracks: subtitleTracks,
	}, nil
}

func resolveWorkspaceOperationSourceFileID(operation library.LibraryOperation) string {
	switch strings.TrimSpace(operation.Kind) {
	case "transcode":
		request := dto.CreateTranscodeJobRequest{}
		if err := json.Unmarshal([]byte(operation.InputJSON), &request); err != nil {
			return ""
		}
		return strings.TrimSpace(request.FileID)
	default:
		return ""
	}
}

func toWorkspaceTaskSummaryDTO(operation library.LibraryOperation) dto.WorkspaceTaskSummaryDTO {
	result := dto.WorkspaceTaskSummaryDTO{
		OperationID: operation.ID,
		Kind:        operation.Kind,
		Status:      string(operation.Status),
		DisplayName: operation.DisplayName,
	}
	if operation.Progress != nil {
		result.Stage = strings.TrimSpace(operation.Progress.Stage)
		if operation.Progress.Current != nil {
			result.Current = *operation.Progress.Current
		}
		if operation.Progress.Total != nil {
			result.Total = *operation.Progress.Total
		}
		result.UpdatedAt = strings.TrimSpace(operation.Progress.UpdatedAt)
	}
	return result
}

func normalizeWorkspaceTrackTasks(value dto.WorkspaceTrackTasksDTO) dto.WorkspaceTrackTasksDTO {
	return value
}

func resolveWorkspaceSubtitleRole(file library.LibraryFile) string {
	_ = file
	return "source"
}

func buildWorkspaceVideoDisplay(file dto.LibraryFileDTO) dto.WorkspaceTrackDisplayDTO {
	parts := make([]string, 0, 5)
	if resolution := resolveLibraryFileResolution(file.Media); resolution != "" {
		parts = append(parts, resolution)
	}
	if frameRate := resolveLibraryFileFrameRate(file.Media); frameRate != "" {
		parts = append(parts, strings.Replace(frameRate, "fps", " fps", 1))
	}
	if codec := resolveLibraryFileCodec(file); codec != "" {
		parts = append(parts, strings.ToUpper(codec))
	}
	if format := resolveLibraryFileFormat(file); format != "" {
		parts = append(parts, strings.ToUpper(format))
	}
	display := dto.WorkspaceTrackDisplayDTO{
		Label: strings.Join(parts, " · "),
		Hint:  strings.TrimSpace(file.Name),
	}
	if display.Label == "" {
		display.Label = strings.TrimSpace(file.Name)
	}
	if strings.EqualFold(file.Kind, "transcode") {
		display.Badges = append(display.Badges, "转码")
	}
	return display
}

func buildWorkspaceSubtitleDisplay(file dto.LibraryFileDTO) dto.WorkspaceTrackDisplayDTO {
	language := normalizeLibraryFileLanguage(file.Media)
	if language == "" {
		language = "TRACK"
	}
	format := strings.ToUpper(resolveLibraryFileFormat(file))
	parts := []string{language}
	if format != "" {
		parts = append(parts, format)
	}
	display := dto.WorkspaceTrackDisplayDTO{
		Label: strings.Join(parts, " · "),
		Hint:  strings.TrimSpace(file.Name),
	}
	if strings.TrimSpace(display.Label) == "" {
		display.Label = strings.TrimSpace(file.Name)
	}
	return display
}
