package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const dreamFMLocalProbeTimeout = 20 * time.Second

type dreamFMLocalCoverLookup struct {
	byOperationID map[string]string
	byRootFileID  map[string]string
}

func (service *LibraryService) ListDreamFMLocalTracks(ctx context.Context, request dto.ListDreamFMLocalTracksRequest) ([]dto.DreamFMLocalTrackDTO, error) {
	if service == nil || service.localTracks == nil {
		return []dto.DreamFMLocalTrackDTO{}, nil
	}
	items, err := service.localTracks.List(ctx, library.DreamFMLocalTrackListOptions{
		Query:              request.Query,
		IncludeUnavailable: request.IncludeUnavailable,
		Limit:              request.Limit,
		Offset:             request.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]dto.DreamFMLocalTrackDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toDreamFMLocalTrackDTO(item))
	}
	return result, nil
}

func (service *LibraryService) RefreshDreamFMLocalIndex(ctx context.Context, request dto.RefreshDreamFMLocalIndexRequest) (dto.DreamFMLocalIndexRefreshResponse, error) {
	response := dto.DreamFMLocalIndexRefreshResponse{}
	if service == nil || service.localTracks == nil || service.files == nil {
		return response, nil
	}

	if fileID := strings.TrimSpace(request.FileID); fileID != "" {
		fileItem, err := service.files.Get(ctx, fileID)
		if err != nil {
			return response, err
		}
		files, _ := service.files.ListByLibraryID(ctx, fileItem.LibraryID)
		lookup := buildDreamFMLocalCoverLookup(files)
		service.refreshDreamFMLocalTrack(ctx, fileItem, nil, lookup, &response)
		return response, nil
	}

	files, err := service.files.List(ctx)
	if err != nil {
		return response, err
	}
	if libraryID := strings.TrimSpace(request.LibraryID); libraryID != "" {
		filtered := make([]library.LibraryFile, 0, len(files))
		for _, item := range files {
			if item.LibraryID == libraryID {
				filtered = append(filtered, item)
			}
		}
		files = filtered
	}
	lookups := make(map[string]dreamFMLocalCoverLookup)
	for _, item := range files {
		if _, exists := lookups[item.LibraryID]; exists {
			continue
		}
		libraryFiles, _ := service.files.ListByLibraryID(ctx, item.LibraryID)
		lookups[item.LibraryID] = buildDreamFMLocalCoverLookup(libraryFiles)
	}
	for _, item := range files {
		service.refreshDreamFMLocalTrack(ctx, item, nil, lookups[item.LibraryID], &response)
	}
	return response, nil
}

func (service *LibraryService) RemoveDreamFMLocalTrack(ctx context.Context, request dto.RemoveDreamFMLocalTrackRequest) error {
	if service == nil || service.localTracks == nil {
		return nil
	}
	fileID := strings.TrimSpace(request.FileID)
	if fileID == "" {
		return library.ErrFileNotFound
	}
	return service.localTracks.Delete(ctx, fileID)
}

func (service *LibraryService) ClearMissingDreamFMLocalTracks(ctx context.Context) (dto.ClearMissingDreamFMLocalTracksResponse, error) {
	if service == nil || service.localTracks == nil {
		return dto.ClearMissingDreamFMLocalTracksResponse{}, nil
	}
	removed, err := service.localTracks.DeleteUnavailable(ctx)
	if err != nil {
		return dto.ClearMissingDreamFMLocalTracksResponse{}, err
	}
	return dto.ClearMissingDreamFMLocalTracksResponse{Removed: removed}, nil
}

func (service *LibraryService) syncDreamFMLocalTrackFromFile(ctx context.Context, fileItem library.LibraryFile, probe *mediaProbe) {
	if service == nil || service.localTracks == nil {
		return
	}
	files, err := service.files.ListByLibraryID(ctx, fileItem.LibraryID)
	if err != nil {
		files = []library.LibraryFile{fileItem}
	}
	response := dto.DreamFMLocalIndexRefreshResponse{}
	service.refreshDreamFMLocalTrack(ctx, fileItem, probe, buildDreamFMLocalCoverLookup(files), &response)
}

func (service *LibraryService) refreshDreamFMLocalTrack(ctx context.Context, fileItem library.LibraryFile, providedProbe *mediaProbe, coverLookup dreamFMLocalCoverLookup, response *dto.DreamFMLocalIndexRefreshResponse) {
	if response != nil {
		response.Scanned++
	}
	if service == nil || service.localTracks == nil {
		return
	}
	if !isDreamFMLocalMediaCandidate(fileItem) {
		if service.localTracks.Delete(ctx, fileItem.ID) == nil && response != nil {
			response.Removed++
		}
		return
	}

	now := service.now()
	track, exists := service.currentDreamFMLocalTrack(ctx, fileItem.ID)
	stat, statErr := os.Stat(strings.TrimSpace(fileItem.Storage.LocalPath))
	if statErr != nil || stat == nil || stat.IsDir() {
		if shouldKeepMissingDreamFMLocalTrack(fileItem, exists) {
			item, buildErr := service.buildMissingDreamFMLocalTrack(fileItem, coverLookup, now, statErr)
			if buildErr == nil && service.localTracks.Save(ctx, item) == nil && response != nil {
				response.Missing++
			}
		} else if exists {
			if service.localTracks.Delete(ctx, fileItem.ID) == nil && response != nil {
				response.Removed++
			}
		}
		return
	}

	var probe mediaProbe
	if providedProbe != nil {
		probe = *providedProbe
	} else {
		probeCtx, cancel := context.WithTimeout(ctx, dreamFMLocalProbeTimeout)
		defer cancel()
		probed, err := service.probeRequiredMedia(probeCtx, fileItem.Storage.LocalPath)
		if err != nil {
			if response != nil {
				response.Failed++
			}
			if exists {
				track.Availability = library.DreamFMLocalTrackMissing
				track.LastCheckedAt = now
				track.ProbeError = strings.TrimSpace(err.Error())
				track.UpdatedAt = now
				_ = service.localTracks.Save(ctx, track)
			}
			return
		}
		probe = probed
	}

	if !isAudioOnlyProbe(probe) {
		if exists {
			if service.localTracks.Delete(ctx, fileItem.ID) == nil && response != nil {
				response.Removed++
			}
		}
		return
	}

	item, err := service.buildAvailableDreamFMLocalTrack(fileItem, probe, coverLookup, stat, now)
	if err != nil {
		if response != nil {
			response.Failed++
		}
		return
	}
	if err := service.localTracks.Save(ctx, item); err != nil {
		if response != nil {
			response.Failed++
		}
		return
	}
	if response != nil {
		if exists {
			response.Updated++
		} else {
			response.Added++
		}
	}
}

func (service *LibraryService) currentDreamFMLocalTrack(ctx context.Context, fileID string) (library.DreamFMLocalTrack, bool) {
	if service == nil || service.localTracks == nil {
		return library.DreamFMLocalTrack{}, false
	}
	item, err := service.localTracks.Get(ctx, fileID)
	return item, err == nil
}

func (service *LibraryService) buildAvailableDreamFMLocalTrack(fileItem library.LibraryFile, probe mediaProbe, coverLookup dreamFMLocalCoverLookup, stat os.FileInfo, now time.Time) (library.DreamFMLocalTrack, error) {
	media := probe.toMediaInfo()
	return library.NewDreamFMLocalTrack(library.DreamFMLocalTrackParams{
		FileID:         fileItem.ID,
		LibraryID:      fileItem.LibraryID,
		LocalPath:      fileItem.Storage.LocalPath,
		Title:          resolveDreamFMLocalTrackTitle(fileItem),
		Author:         fileItem.Metadata.Author,
		CoverLocalPath: resolveDreamFMLocalCoverPath(fileItem, coverLookup),
		Format:         media.Format,
		AudioCodec:     media.AudioCodec,
		DurationMs:     media.DurationMs,
		SizeBytes:      resolveDreamFMLocalSize(&media, stat),
		ModTimeUnix:    stat.ModTime().Unix(),
		Availability:   library.DreamFMLocalTrackAvailable,
		LastCheckedAt:  &now,
		CreatedAt:      &fileItem.CreatedAt,
		UpdatedAt:      &now,
	})
}

func (service *LibraryService) buildMissingDreamFMLocalTrack(fileItem library.LibraryFile, coverLookup dreamFMLocalCoverLookup, now time.Time, statErr error) (library.DreamFMLocalTrack, error) {
	message := "missing local file"
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		message = statErr.Error()
	}
	return library.NewDreamFMLocalTrack(library.DreamFMLocalTrackParams{
		FileID:         fileItem.ID,
		LibraryID:      fileItem.LibraryID,
		LocalPath:      fileItem.Storage.LocalPath,
		Title:          resolveDreamFMLocalTrackTitle(fileItem),
		Author:         fileItem.Metadata.Author,
		CoverLocalPath: resolveDreamFMLocalCoverPath(fileItem, coverLookup),
		Format:         mediaFormatFromFile(fileItem),
		AudioCodec:     "",
		Availability:   library.DreamFMLocalTrackMissing,
		LastCheckedAt:  &now,
		ProbeError:     message,
		CreatedAt:      &fileItem.CreatedAt,
		UpdatedAt:      &now,
	})
}

func isDreamFMLocalMediaCandidate(fileItem library.LibraryFile) bool {
	if fileItem.State.Deleted || strings.TrimSpace(fileItem.Storage.LocalPath) == "" {
		return false
	}
	switch fileItem.Kind {
	case library.FileKindAudio, library.FileKindVideo, library.FileKindTranscode:
		return true
	default:
		return false
	}
}

func shouldKeepMissingDreamFMLocalTrack(fileItem library.LibraryFile, existed bool) bool {
	if existed {
		return true
	}
	if fileItem.Kind == library.FileKindAudio {
		return true
	}
	return isLikelyAudioPath(fileItem.Storage.LocalPath)
}

func isLikelyAudioPath(path string) bool {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(strings.TrimSpace(path)), ".")) {
	case "aac", "aif", "aiff", "alac", "caf", "flac", "m4a", "m4b", "mp3", "mpga", "oga", "ogg", "opus", "wav", "wave", "weba":
		return true
	default:
		return false
	}
}

func isAudioOnlyProbe(probe mediaProbe) bool {
	return probe.StreamInfo && probe.HasAudio && !probe.HasVideo
}

func resolveDreamFMLocalTrackTitle(fileItem library.LibraryFile) string {
	return firstNonEmpty(
		fileItem.Metadata.Title,
		fileItem.DisplayName,
		strings.TrimSuffix(resolveLibraryFileName(fileItem), filepath.Ext(resolveLibraryFileName(fileItem))),
		fileItem.Name,
		fileItem.ID,
	)
}

func resolveDreamFMLocalSize(media *library.MediaInfo, stat os.FileInfo) *int64 {
	if media != nil && media.SizeBytes != nil {
		return media.SizeBytes
	}
	if stat == nil {
		return nil
	}
	value := stat.Size()
	return &value
}

func buildDreamFMLocalCoverLookup(files []library.LibraryFile) dreamFMLocalCoverLookup {
	lookup := dreamFMLocalCoverLookup{
		byOperationID: make(map[string]string),
		byRootFileID:  make(map[string]string),
	}
	for _, fileItem := range files {
		if fileItem.State.Deleted || fileItem.Kind != library.FileKindThumbnail || strings.TrimSpace(fileItem.Storage.LocalPath) == "" {
			continue
		}
		operationKeys := []string{fileItem.LatestOperationID, fileItem.Origin.OperationID}
		for _, key := range operationKeys {
			key = strings.TrimSpace(key)
			if key != "" {
				lookup.byOperationID[key] = fileItem.Storage.LocalPath
			}
		}
		rootKeys := []string{fileItem.Lineage.RootFileID, fileItem.ID}
		for _, key := range rootKeys {
			key = strings.TrimSpace(key)
			if key != "" {
				lookup.byRootFileID[key] = fileItem.Storage.LocalPath
			}
		}
	}
	return lookup
}

func resolveDreamFMLocalCoverPath(fileItem library.LibraryFile, lookup dreamFMLocalCoverLookup) string {
	for _, key := range []string{fileItem.LatestOperationID, fileItem.Origin.OperationID} {
		if value := lookup.byOperationID[strings.TrimSpace(key)]; strings.TrimSpace(value) != "" {
			return value
		}
	}
	for _, key := range []string{fileItem.Lineage.RootFileID, fileItem.ID} {
		if value := lookup.byRootFileID[strings.TrimSpace(key)]; strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func toDreamFMLocalTrackDTO(item library.DreamFMLocalTrack) dto.DreamFMLocalTrackDTO {
	return dto.DreamFMLocalTrackDTO{
		ID:             item.FileID,
		FileID:         item.FileID,
		LibraryID:      item.LibraryID,
		Title:          item.Title,
		Author:         item.Author,
		LocalPath:      item.LocalPath,
		CoverLocalPath: item.CoverLocalPath,
		Format:         item.Format,
		AudioCodec:     item.AudioCodec,
		DurationMs:     item.DurationMs,
		SizeBytes:      item.SizeBytes,
		Availability:   item.Availability,
		LastCheckedAt:  item.LastCheckedAt.Format(time.RFC3339),
		ProbeError:     item.ProbeError,
		UpdatedAt:      item.UpdatedAt.Format(time.RFC3339),
	}
}
