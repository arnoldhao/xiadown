package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	listenLocalProbeTimeout       = 20 * time.Second
	listenLocalRefreshWorkerCount = 4
)

type listenLocalCoverLookup struct {
	byOperationID map[string]string
	byRootFileID  map[string]string
}

func (service *LibraryService) ListListenLocalTracks(ctx context.Context, request dto.ListListenLocalTracksRequest) ([]dto.ListenLocalTrackDTO, error) {
	if service == nil || service.localTracks == nil {
		return []dto.ListenLocalTrackDTO{}, nil
	}
	items, err := service.localTracks.List(ctx, library.ListenLocalTrackListOptions{
		Query:              request.Query,
		Artist:             request.Artist,
		Album:              request.Album,
		Sort:               request.Sort,
		IncludeUnavailable: request.IncludeUnavailable,
		Limit:              request.Limit,
		Offset:             request.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]dto.ListenLocalTrackDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toListenLocalTrackDTO(item))
	}
	return result, nil
}

func (service *LibraryService) RefreshListenLocalIndex(ctx context.Context, request dto.RefreshListenLocalIndexRequest) (dto.ListenLocalIndexRefreshResponse, error) {
	response := dto.ListenLocalIndexRefreshResponse{}
	if service == nil || service.localTracks == nil || service.files == nil {
		return response, nil
	}

	if fileID := strings.TrimSpace(request.FileID); fileID != "" {
		fileItem, err := service.files.Get(ctx, fileID)
		if err != nil {
			return response, err
		}
		files, _ := service.files.ListByLibraryID(ctx, fileItem.LibraryID)
		lookup := buildListenLocalCoverLookup(files)
		service.refreshListenLocalTrack(ctx, fileItem, nil, lookup, &response)
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
	lookups := make(map[string]listenLocalCoverLookup)
	for _, item := range files {
		if err := ctx.Err(); err != nil {
			return response, err
		}
		if _, exists := lookups[item.LibraryID]; exists {
			continue
		}
		libraryFiles, _ := service.files.ListByLibraryID(ctx, item.LibraryID)
		lookups[item.LibraryID] = buildListenLocalCoverLookup(libraryFiles)
	}
	if len(files) == 0 {
		return response, nil
	}

	workerCount := min(listenLocalRefreshWorkerCount, len(files))
	jobs := make(chan library.LibraryFile)
	var responseMu sync.Mutex
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-jobs:
					if !ok {
						return
					}
					if ctx.Err() != nil {
						return
					}
					itemResponse := dto.ListenLocalIndexRefreshResponse{}
					service.refreshListenLocalTrack(ctx, item, nil, lookups[item.LibraryID], &itemResponse)
					responseMu.Lock()
					mergeListenLocalIndexRefreshResponse(&response, itemResponse)
					responseMu.Unlock()
				}
			}
		}()
	}

dispatch:
	for _, item := range files {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- item:
		}
	}
	close(jobs)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return response, err
	}
	return response, nil
}

func mergeListenLocalIndexRefreshResponse(target *dto.ListenLocalIndexRefreshResponse, item dto.ListenLocalIndexRefreshResponse) {
	if target == nil {
		return
	}
	target.Scanned += item.Scanned
	target.Added += item.Added
	target.Updated += item.Updated
	target.Removed += item.Removed
	target.Missing += item.Missing
	target.Failed += item.Failed
}

func (service *LibraryService) RemoveListenLocalTrack(ctx context.Context, request dto.RemoveListenLocalTrackRequest) error {
	if service == nil || service.localTracks == nil {
		return nil
	}
	fileID := strings.TrimSpace(request.FileID)
	if fileID == "" {
		return library.ErrFileNotFound
	}
	unlock := service.lockListenLocalTrackMutation(fileID)
	defer unlock()
	if service.localMusicMemberships != nil {
		now := service.now()
		membership, err := library.NewListenLocalMusicMembership(library.ListenLocalMusicMembershipParams{
			FileID: fileID, State: string(library.ListenLocalMusicMembershipExcluded), Reason: "user",
			CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			return err
		}
		if err := service.localMusicMemberships.Save(ctx, membership); err != nil {
			return err
		}
	}
	return service.localTracks.Delete(ctx, fileID)
}

func (service *LibraryService) ClearMissingListenLocalTracks(ctx context.Context) (dto.ClearMissingListenLocalTracksResponse, error) {
	if service == nil || service.localTracks == nil {
		return dto.ClearMissingListenLocalTracksResponse{}, nil
	}
	unlock := service.lockAllListenLocalTrackMutations()
	defer unlock()
	removed, err := service.localTracks.DeleteUnavailable(ctx)
	if err != nil {
		return dto.ClearMissingListenLocalTracksResponse{}, err
	}
	return dto.ClearMissingListenLocalTracksResponse{Removed: removed}, nil
}

func (service *LibraryService) syncListenLocalTrackFromFile(ctx context.Context, fileItem library.LibraryFile, probe *mediaProbe) {
	if service == nil || service.localTracks == nil {
		return
	}
	files, err := service.files.ListByLibraryID(ctx, fileItem.LibraryID)
	if err != nil {
		files = []library.LibraryFile{fileItem}
	}
	response := dto.ListenLocalIndexRefreshResponse{}
	service.refreshListenLocalTrack(ctx, fileItem, probe, buildListenLocalCoverLookup(files), &response)
}

func (service *LibraryService) refreshListenLocalTrack(ctx context.Context, fileItem library.LibraryFile, providedProbe *mediaProbe, coverLookup listenLocalCoverLookup, response *dto.ListenLocalIndexRefreshResponse) {
	if response != nil {
		response.Scanned++
	}
	if service == nil || service.localTracks == nil {
		return
	}
	unlock := service.lockListenLocalTrackMutation(fileItem.ID)
	defer unlock()
	providedProbeMatchesCurrentPath := true
	if service.files != nil {
		// Refresh jobs are queued from a snapshot. A relink may have committed a
		// newer path while this job waited for the same-file mutation lock.
		if current, err := service.files.Get(ctx, fileItem.ID); err == nil {
			providedProbeMatchesCurrentPath = sameListenLocalPath(
				fileItem.Storage.LocalPath,
				current.Storage.LocalPath,
			)
			fileItem = current
		}
	}
	track, exists := service.currentListenLocalTrack(ctx, fileItem.ID)
	if service.localMusicMemberships != nil {
		membership, membershipErr := service.localMusicMemberships.Get(ctx, fileItem.ID)
		switch {
		case membershipErr == nil && membership.IsExcluded():
			if exists {
				if err := service.localTracks.Delete(ctx, fileItem.ID); err != nil {
					if response != nil {
						response.Failed++
					}
					return
				}
				if response != nil {
					response.Removed++
				}
			}
			return
		case membershipErr != nil && !errors.Is(membershipErr, library.ErrListenLocalMusicMembershipNotFound):
			if response != nil {
				response.Failed++
			}
			return
		}
	}
	if !isListenLocalMediaCandidate(fileItem) {
		if service.localTracks.Delete(ctx, fileItem.ID) == nil && response != nil {
			response.Removed++
		}
		return
	}

	now := service.now()
	stat, statErr := os.Stat(strings.TrimSpace(fileItem.Storage.LocalPath))
	if statErr != nil || stat == nil || stat.IsDir() {
		if shouldKeepMissingListenLocalTrack(fileItem, exists) {
			item := track
			buildErr := error(nil)
			if exists {
				// Losing the file must not erase metadata that was successfully
				// probed earlier. This data still powers missing-track repair and
				// playlist presentation while the file is unavailable.
				item.LocalPath = strings.TrimSpace(fileItem.Storage.LocalPath)
				item.Availability = library.ListenLocalTrackMissing
				item.LastCheckedAt = now
				item.UpdatedAt = now
				item.ProbeError = listenLocalMissingMessage(statErr)
				if cover := resolveListenLocalCoverPath(fileItem, coverLookup); cover != "" {
					item.CoverLocalPath = cover
				}
			} else {
				item, buildErr = service.buildMissingListenLocalTrack(fileItem, coverLookup, now, statErr)
			}
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
	if providedProbe != nil && !exists && providedProbeMatchesCurrentPath {
		probe = *providedProbe
	} else {
		// A supplied probe may have been captured before a concurrent metadata
		// replacement. Re-probe existing rows while holding the same-file lock so
		// stale tags can never become the final repository write.
		probeCtx, cancel := context.WithTimeout(ctx, listenLocalProbeTimeout)
		defer cancel()
		probed, err := service.probeRequiredMedia(probeCtx, fileItem.Storage.LocalPath)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if response != nil {
				response.Failed++
			}
			if exists {
				// The file is still present, so a transient decoder/tool failure must
				// not hide a previously indexed song or erase its known-good tags.
				track.Availability = library.ListenLocalTrackAvailable
				track.LastCheckedAt = now
				track.ProbeError = strings.TrimSpace(err.Error())
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
	probe.ContentIdentitySignature = stabilizeListenLocalContentIdentitySignature(
		track.ContentIdentitySignature,
		service.listenLocalContentIdentitySignature(
			ctx,
			fileItem.Storage.LocalPath,
			probe,
		),
	)

	item, err := service.buildAvailableListenLocalTrack(fileItem, probe, coverLookup, stat, now)
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

func (service *LibraryService) currentListenLocalTrack(ctx context.Context, fileID string) (library.ListenLocalTrack, bool) {
	if service == nil || service.localTracks == nil {
		return library.ListenLocalTrack{}, false
	}
	item, err := service.localTracks.Get(ctx, fileID)
	return item, err == nil
}

func (service *LibraryService) buildAvailableListenLocalTrack(fileItem library.LibraryFile, probe mediaProbe, coverLookup listenLocalCoverLookup, stat os.FileInfo, now time.Time) (library.ListenLocalTrack, error) {
	media := probe.toMediaInfo()
	return library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID:                   fileItem.ID,
		LibraryID:                fileItem.LibraryID,
		ContentIdentitySignature: probe.ContentIdentitySignature,
		LocalPath:                fileItem.Storage.LocalPath,
		Title:                    firstNonEmpty(probe.Title, resolveListenLocalTrackTitle(fileItem)),
		Author:                   firstNonEmpty(probe.Artist, fileItem.Metadata.Author),
		Album:                    probe.Album,
		AlbumArtist:              probe.AlbumArtist,
		Genre:                    probe.Genre,
		TrackNumber:              probe.TrackNumber,
		DiscNumber:               probe.DiscNumber,
		Year:                     probe.Year,
		CoverLocalPath:           resolveListenLocalCoverPath(fileItem, coverLookup),
		Format:                   media.Format,
		AudioCodec:               media.AudioCodec,
		DurationMs:               media.DurationMs,
		SizeBytes:                resolveListenLocalSize(&media, stat),
		ModTimeUnix:              stat.ModTime().Unix(),
		Availability:             library.ListenLocalTrackAvailable,
		LastCheckedAt:            &now,
		CreatedAt:                &fileItem.CreatedAt,
		UpdatedAt:                &now,
	})
}

func (service *LibraryService) buildMissingListenLocalTrack(fileItem library.LibraryFile, coverLookup listenLocalCoverLookup, now time.Time, statErr error) (library.ListenLocalTrack, error) {
	return library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID:         fileItem.ID,
		LibraryID:      fileItem.LibraryID,
		LocalPath:      fileItem.Storage.LocalPath,
		Title:          resolveListenLocalTrackTitle(fileItem),
		Author:         fileItem.Metadata.Author,
		CoverLocalPath: resolveListenLocalCoverPath(fileItem, coverLookup),
		Format:         mediaFormatFromFile(fileItem),
		AudioCodec:     "",
		Availability:   library.ListenLocalTrackMissing,
		LastCheckedAt:  &now,
		ProbeError:     listenLocalMissingMessage(statErr),
		CreatedAt:      &fileItem.CreatedAt,
		UpdatedAt:      &now,
	})
}

func listenLocalMissingMessage(statErr error) string {
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr.Error()
	}
	return "missing local file"
}

func isListenLocalMediaCandidate(fileItem library.LibraryFile) bool {
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

func shouldKeepMissingListenLocalTrack(fileItem library.LibraryFile, existed bool) bool {
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

func resolveListenLocalTrackTitle(fileItem library.LibraryFile) string {
	return firstNonEmpty(
		fileItem.Metadata.Title,
		fileItem.DisplayName,
		strings.TrimSuffix(resolveLibraryFileName(fileItem), filepath.Ext(resolveLibraryFileName(fileItem))),
		fileItem.Name,
		fileItem.ID,
	)
}

func resolveListenLocalSize(media *library.MediaInfo, stat os.FileInfo) *int64 {
	if media != nil && media.SizeBytes != nil {
		return media.SizeBytes
	}
	if stat == nil {
		return nil
	}
	value := stat.Size()
	return &value
}

func buildListenLocalCoverLookup(files []library.LibraryFile) listenLocalCoverLookup {
	lookup := listenLocalCoverLookup{
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

func resolveListenLocalCoverPath(fileItem library.LibraryFile, lookup listenLocalCoverLookup) string {
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

func toListenLocalTrackDTO(item library.ListenLocalTrack) dto.ListenLocalTrackDTO {
	return dto.ListenLocalTrackDTO{
		ID:               item.FileID,
		FileID:           item.FileID,
		LibraryID:        item.LibraryID,
		Title:            item.Title,
		Author:           item.Author,
		Album:            item.Album,
		AlbumArtist:      item.AlbumArtist,
		Genre:            item.Genre,
		TrackNumber:      item.TrackNumber,
		DiscNumber:       item.DiscNumber,
		Year:             item.Year,
		LocalPath:        item.LocalPath,
		CoverLocalPath:   item.CoverLocalPath,
		Format:           item.Format,
		AudioCodec:       item.AudioCodec,
		DurationMs:       item.DurationMs,
		SizeBytes:        item.SizeBytes,
		ModTimeUnix:      item.ModTimeUnix,
		Availability:     item.Availability,
		LastCheckedAt:    item.LastCheckedAt.Format(time.RFC3339),
		ProbeError:       item.ProbeError,
		CreatedAt:        item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.Format(time.RFC3339),
		MetadataWritable: listenLocalMetadataWritable(item.LocalPath),
	}
}
