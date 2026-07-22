package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	musicIOSRepresentationSource   = "xiadown.music.ios-compatible-v1"
	musicIOSRepresentationPresetID = "builtin-audio-aac-m4a-192k"
)

// GetIOSCompatibleRepresentationStatuses projects the durable Library
// operation lifecycle for one Music page with a single journal read. It never
// exposes operation IDs, paths, command output, or error text.
func (service *LibraryService) GetIOSCompatibleRepresentationStatuses(
	ctx context.Context,
	trackIDs []string,
) (map[string]library.ListenLocalMusicCompatibleRepresentationStatus, error) {
	if service == nil || service.localTracks == nil || service.operations == nil {
		return nil,
			library.ErrListenLocalMusicCompatibleRepresentationUnavailable
	}
	normalized := make([]string, 0, len(trackIDs))
	seen := make(map[string]struct{}, len(trackIDs))
	for _, trackID := range trackIDs {
		trackID = strings.TrimSpace(trackID)
		if trackID == "" {
			return nil, library.ErrListenLocalMusicCompatibleRepresentationUnavailable
		}
		if _, ok := seen[trackID]; ok {
			continue
		}
		seen[trackID] = struct{}{}
		normalized = append(normalized, trackID)
	}
	if len(normalized) == 0 {
		return map[string]library.ListenLocalMusicCompatibleRepresentationStatus{}, nil
	}
	operations, err := service.operations.List(ctx)
	if err != nil {
		return nil, err
	}
	return musicIOSRepresentationStatuses(operations, normalized), nil
}

// RequestIOSCompatibleRepresentation reuses Desktop's managed transcode
// pipeline with one fixed iOS baseline preset. The client request UUID is
// persisted as RunID, making an identical retry replay the same lifecycle.
func (service *LibraryService) RequestIOSCompatibleRepresentation(
	ctx context.Context,
	trackID string,
	requestID string,
) (library.ListenLocalMusicCompatibleRepresentationStatus, error) {
	trackID = strings.TrimSpace(trackID)
	requestID = strings.ToLower(strings.TrimSpace(requestID))
	if service == nil || service.localTracks == nil || service.localMusicMemberships == nil ||
		service.files == nil || service.operations == nil ||
		trackID == "" || uuid.Validate(requestID) != nil {
		return library.ListenLocalMusicCompatibleRepresentationStatus{},
			library.ErrListenLocalMusicCompatibleRepresentationUnavailable
	}

	service.musicIOSRepresentationMu.Lock()
	defer service.musicIOSRepresentationMu.Unlock()

	track, err := service.localTracks.Get(ctx, trackID)
	if err != nil {
		return library.ListenLocalMusicCompatibleRepresentationStatus{}, err
	}
	if track.Availability != library.ListenLocalTrackAvailable {
		return library.ListenLocalMusicCompatibleRepresentationStatus{},
			library.ErrListenLocalMusicCompatibleRepresentationUnavailable
	}
	file, err := service.files.Get(ctx, trackID)
	if err != nil || file.State.Deleted || strings.TrimSpace(file.Storage.LocalPath) == "" {
		if err != nil {
			return library.ListenLocalMusicCompatibleRepresentationStatus{}, err
		}
		return library.ListenLocalMusicCompatibleRepresentationStatus{},
			library.ErrListenLocalMusicCompatibleRepresentationUnavailable
	}

	operations, err := service.operations.List(ctx)
	if err != nil {
		return library.ListenLocalMusicCompatibleRepresentationStatus{}, err
	}
	for _, operation := range operations {
		request, ok := musicIOSRepresentationOperationRequest(operation)
		if !ok || strings.ToLower(strings.TrimSpace(request.RunID)) != requestID {
			continue
		}
		if strings.TrimSpace(request.FileID) != trackID {
			return library.ListenLocalMusicCompatibleRepresentationStatus{},
				library.ErrListenLocalMusicCompatibleRepresentationConflict
		}
		return musicIOSRepresentationStatusFromOperation(operation), nil
	}

	current := musicIOSRepresentationStatusForTrack(operations, trackID)
	if current.Status == library.ListenLocalMusicCompatibleRepresentationGenerating ||
		current.Status == library.ListenLocalMusicCompatibleRepresentationReady {
		return current, nil
	}

	_, err = service.CreateTranscodeJob(ctx, dto.CreateTranscodeJobRequest{
		FileID:   trackID,
		PresetID: musicIOSRepresentationPresetID,
		Source:   musicIOSRepresentationSource,
		RunID:    requestID,
	})
	if err != nil {
		return library.ListenLocalMusicCompatibleRepresentationStatus{}, err
	}
	// The compatible-version lifecycle is part of the paired Track projection,
	// even before bytes exist. Advancing the resource revision makes other
	// devices observe the queued state through the ordinary Music change feed.
	_ = service.invalidateIOSCompatibleRepresentationTrack(ctx, trackID)
	return library.ListenLocalMusicCompatibleRepresentationStatus{
		Status: library.ListenLocalMusicCompatibleRepresentationGenerating,
	}, nil
}

func musicIOSRepresentationStatusForTrack(
	operations []library.LibraryOperation,
	trackID string,
) library.ListenLocalMusicCompatibleRepresentationStatus {
	trackID = strings.TrimSpace(trackID)
	return musicIOSRepresentationStatuses(operations, []string{trackID})[trackID]
}

func musicIOSRepresentationStatuses(
	operations []library.LibraryOperation,
	trackIDs []string,
) map[string]library.ListenLocalMusicCompatibleRepresentationStatus {
	requested := make(map[string]struct{}, len(trackIDs))
	result := make(map[string]library.ListenLocalMusicCompatibleRepresentationStatus, len(trackIDs))
	latestTerminal := make(map[string]library.LibraryOperation, len(trackIDs))
	for _, trackID := range trackIDs {
		trackID = strings.TrimSpace(trackID)
		if trackID == "" {
			continue
		}
		requested[trackID] = struct{}{}
		result[trackID] = library.ListenLocalMusicCompatibleRepresentationStatus{
			Status: library.ListenLocalMusicCompatibleRepresentationUnsupported,
		}
	}
	for index := range operations {
		operation := operations[index]
		request, ok := musicIOSRepresentationOperationRequest(operation)
		if !ok {
			continue
		}
		trackID := strings.TrimSpace(request.FileID)
		if _, ok := requested[trackID]; !ok {
			continue
		}
		status := musicIOSRepresentationStatusFromOperation(operation)
		if status.Status == library.ListenLocalMusicCompatibleRepresentationGenerating {
			result[trackID] = status
			continue
		}
		if result[trackID].Status == library.ListenLocalMusicCompatibleRepresentationGenerating {
			continue
		}
		latest, exists := latestTerminal[trackID]
		if !exists || operation.CreatedAt.After(latest.CreatedAt) {
			latestTerminal[trackID] = operation
		}
	}
	for trackID, operation := range latestTerminal {
		if result[trackID].Status != library.ListenLocalMusicCompatibleRepresentationGenerating {
			result[trackID] = musicIOSRepresentationStatusFromOperation(operation)
		}
	}
	return result
}

func musicIOSRepresentationOperationRequest(
	operation library.LibraryOperation,
) (dto.CreateTranscodeJobRequest, bool) {
	if operation.Kind != "transcode" || strings.TrimSpace(operation.InputJSON) == "" {
		return dto.CreateTranscodeJobRequest{}, false
	}
	var request dto.CreateTranscodeJobRequest
	if err := json.Unmarshal([]byte(operation.InputJSON), &request); err != nil ||
		strings.TrimSpace(request.Source) != musicIOSRepresentationSource ||
		strings.TrimSpace(request.PresetID) != musicIOSRepresentationPresetID ||
		strings.TrimSpace(request.FileID) == "" {
		return dto.CreateTranscodeJobRequest{}, false
	}
	return request, true
}

func musicIOSRepresentationStatusFromOperation(
	operation library.LibraryOperation,
) library.ListenLocalMusicCompatibleRepresentationStatus {
	switch operation.Status {
	case library.OperationStatusQueued, library.OperationStatusRunning:
		return library.ListenLocalMusicCompatibleRepresentationStatus{
			Status: library.ListenLocalMusicCompatibleRepresentationGenerating,
		}
	case library.OperationStatusSucceeded:
		return library.ListenLocalMusicCompatibleRepresentationStatus{
			Status: library.ListenLocalMusicCompatibleRepresentationReady,
		}
	case library.OperationStatusFailed, library.OperationStatusCanceled:
		return library.ListenLocalMusicCompatibleRepresentationStatus{
			Status:    library.ListenLocalMusicCompatibleRepresentationFailed,
			ErrorCode: "generation_failed",
		}
	default:
		return library.ListenLocalMusicCompatibleRepresentationStatus{
			Status: library.ListenLocalMusicCompatibleRepresentationUnsupported,
		}
	}
}

func (service *LibraryService) invalidateIOSCompatibleRepresentationTrack(
	ctx context.Context,
	trackID string,
) error {
	if service == nil || service.localTracks == nil {
		return nil
	}
	track, err := service.localTracks.Get(ctx, strings.TrimSpace(trackID))
	if err != nil {
		if errors.Is(err, library.ErrFileNotFound) {
			return nil
		}
		return err
	}
	track.ResourceRevision++
	track.UpdatedAt = service.now()
	return service.localTracks.Save(ctx, track)
}

func (service *LibraryService) excludeIOSCompatibleRepresentationOutput(
	ctx context.Context,
	fileID string,
) error {
	if service == nil || service.localMusicMemberships == nil {
		return library.ErrListenLocalMusicCompatibleRepresentationUnavailable
	}
	now := service.now()
	membership, err := library.NewListenLocalMusicMembership(library.ListenLocalMusicMembershipParams{
		FileID: strings.TrimSpace(fileID), State: string(library.ListenLocalMusicMembershipExcluded),
		Reason: "policy", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		return err
	}
	return service.localMusicMemberships.Save(ctx, membership)
}

func isIOSCompatibleMusicRepresentationRequest(request dto.CreateTranscodeJobRequest) bool {
	return strings.TrimSpace(request.Source) == musicIOSRepresentationSource &&
		strings.TrimSpace(request.PresetID) == musicIOSRepresentationPresetID &&
		strings.TrimSpace(request.FileID) != ""
}

var _ library.ListenLocalMusicCompatibleRepresentationCoordinator = (*LibraryService)(nil)
