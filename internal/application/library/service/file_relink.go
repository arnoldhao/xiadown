package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	libraryRelinkConfidenceExact     = "exact"
	libraryRelinkConfidenceCandidate = "candidate"
	libraryRelinkMinimumScore        = 80
)

func (service *LibraryService) ListMissingLibraryFiles(ctx context.Context) (dto.ListMissingLibraryFilesResponse, error) {
	response, _, err := service.listMissingLibraryFileItems(ctx, true, nil)
	return response, err
}

func (service *LibraryService) ScanMissingLibraryFiles(ctx context.Context, request dto.ScanMissingLibraryFilesRequest) (dto.ScanMissingLibraryFilesResponse, error) {
	directory, err := resolveRelinkScanDirectory(request.Directory)
	if err != nil {
		return dto.ScanMissingLibraryFilesResponse{}, err
	}
	filter := map[string]struct{}{}
	for _, fileID := range request.FileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID != "" {
			filter[fileID] = struct{}{}
		}
	}
	if len(filter) == 0 {
		filter = nil
	}

	missingResponse, missingItems, err := service.listMissingLibraryFileItems(ctx, true, filter)
	if err != nil {
		return dto.ScanMissingLibraryFilesResponse{}, err
	}
	index, scannedFiles, err := buildRelinkCandidateIndex(directory)
	if err != nil {
		return dto.ScanMissingLibraryFilesResponse{}, err
	}

	matches := make([]dto.LibraryRelinkMatchDTO, 0)
	for _, item := range missingItems {
		for _, candidatePath := range relinkCandidatePathsForFile(index, item) {
			match := service.evaluateLibraryRelinkMatch(ctx, item, candidatePath)
			if libraryRelinkMatchAccepted(match) {
				matches = append(matches, match)
			}
		}
	}
	sortLibraryRelinkMatches(matches)

	return dto.ScanMissingLibraryFilesResponse{
		Directory:    directory,
		Checked:      missingResponse.Checked,
		MissingCount: len(missingItems),
		ScannedFiles: scannedFiles,
		Matches:      matches,
	}, nil
}

func (service *LibraryService) ApplyLibraryRelinks(ctx context.Context, request dto.ApplyLibraryRelinksRequest) (dto.ApplyLibraryRelinksResponse, error) {
	if service == nil || service.files == nil {
		return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("library file repository unavailable")
	}
	if len(request.Matches) == 0 {
		return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("no relink matches selected")
	}
	seen := make(map[string]struct{}, len(request.Matches))
	result := dto.ApplyLibraryRelinksResponse{Files: make([]dto.LibraryFileDTO, 0, len(request.Matches))}
	for _, selection := range request.Matches {
		fileID := strings.TrimSpace(selection.FileID)
		if fileID == "" {
			return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("fileId is required")
		}
		if _, exists := seen[fileID]; exists {
			return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("duplicate relink selection for file %s", fileID)
		}
		seen[fileID] = struct{}{}
		path, err := resolveRelinkSelectedPath(selection.Path)
		if err != nil {
			return dto.ApplyLibraryRelinksResponse{}, err
		}
		item, err := service.files.Get(ctx, fileID)
		if err != nil {
			return dto.ApplyLibraryRelinksResponse{}, err
		}
		match := service.evaluateLibraryRelinkMatch(ctx, item, path)
		if !libraryRelinkMatchAccepted(match) {
			return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("selected file does not match %s", resolveLibraryFileDisplayName(item))
		}
		fileDTO, err := service.applyLibraryFileRelink(ctx, item, path)
		if err != nil {
			return dto.ApplyLibraryRelinksResponse{}, err
		}
		result.Relinked++
		result.Files = append(result.Files, fileDTO)
	}
	return result, nil
}

func (service *LibraryService) ListMissingListenLocalFiles(ctx context.Context) (dto.ListMissingLibraryFilesResponse, error) {
	response, _, err := service.listMissingListenLocalFileItems(ctx, true, nil)
	return response, err
}

func (service *LibraryService) ScanMissingListenLocalFiles(ctx context.Context, request dto.ScanMissingLibraryFilesRequest) (dto.ScanMissingLibraryFilesResponse, error) {
	directory, err := resolveRelinkScanDirectory(request.Directory)
	if err != nil {
		return dto.ScanMissingLibraryFilesResponse{}, err
	}
	filter := map[string]struct{}{}
	for _, fileID := range request.FileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID != "" {
			filter[fileID] = struct{}{}
		}
	}
	if len(filter) == 0 {
		filter = nil
	}

	missingResponse, missingItems, err := service.listMissingListenLocalFileItems(ctx, true, filter)
	if err != nil {
		return dto.ScanMissingLibraryFilesResponse{}, err
	}
	index, scannedFiles, err := buildRelinkCandidateIndex(directory)
	if err != nil {
		return dto.ScanMissingLibraryFilesResponse{}, err
	}

	matches := make([]dto.LibraryRelinkMatchDTO, 0)
	for _, item := range missingItems {
		for _, candidatePath := range relinkCandidatePathsForFile(index, item) {
			match := service.evaluateLibraryRelinkMatch(ctx, item, candidatePath)
			if libraryRelinkMatchAccepted(match) {
				matches = append(matches, match)
			}
		}
	}
	sortLibraryRelinkMatches(matches)

	return dto.ScanMissingLibraryFilesResponse{
		Directory:    directory,
		Checked:      missingResponse.Checked,
		MissingCount: len(missingItems),
		ScannedFiles: scannedFiles,
		Matches:      matches,
	}, nil
}

func (service *LibraryService) ApplyListenLocalRelinks(ctx context.Context, request dto.ApplyLibraryRelinksRequest) (dto.ApplyLibraryRelinksResponse, error) {
	if service == nil || service.files == nil {
		return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("library file repository unavailable")
	}
	if len(request.Matches) == 0 {
		return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("no relink matches selected")
	}
	filter := map[string]struct{}{}
	for _, selection := range request.Matches {
		fileID := strings.TrimSpace(selection.FileID)
		if fileID != "" {
			filter[fileID] = struct{}{}
		}
	}
	_, missingItems, err := service.listMissingListenLocalFileItems(ctx, true, filter)
	if err != nil {
		return dto.ApplyLibraryRelinksResponse{}, err
	}
	missingByID := make(map[string]library.LibraryFile, len(missingItems))
	for _, item := range missingItems {
		missingByID[item.ID] = item
	}

	seen := make(map[string]struct{}, len(request.Matches))
	result := dto.ApplyLibraryRelinksResponse{Files: make([]dto.LibraryFileDTO, 0, len(request.Matches))}
	for _, selection := range request.Matches {
		fileID := strings.TrimSpace(selection.FileID)
		if fileID == "" {
			return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("fileId is required")
		}
		if _, exists := seen[fileID]; exists {
			return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("duplicate relink selection for file %s", fileID)
		}
		seen[fileID] = struct{}{}
		item, ok := missingByID[fileID]
		if !ok {
			return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("listen local file is not missing: %s", fileID)
		}
		path, err := resolveRelinkSelectedPath(selection.Path)
		if err != nil {
			return dto.ApplyLibraryRelinksResponse{}, err
		}
		match := service.evaluateLibraryRelinkMatch(ctx, item, path)
		if !libraryRelinkMatchAccepted(match) {
			return dto.ApplyLibraryRelinksResponse{}, fmt.Errorf("selected file does not match %s", resolveLibraryFileDisplayName(item))
		}
		fileDTO, err := service.applyLibraryFileRelink(ctx, item, path)
		if err != nil {
			return dto.ApplyLibraryRelinksResponse{}, err
		}
		result.Relinked++
		result.Files = append(result.Files, fileDTO)
	}
	return result, nil
}

func sortLibraryRelinkMatches(matches []dto.LibraryRelinkMatchDTO) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].FileID != matches[j].FileID {
			return matches[i].Name < matches[j].Name
		}
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].NewPath < matches[j].NewPath
	})
}

func (service *LibraryService) listMissingLibraryFileItems(ctx context.Context, updateState bool, filter map[string]struct{}) (dto.ListMissingLibraryFilesResponse, []library.LibraryFile, error) {
	response := dto.ListMissingLibraryFilesResponse{Missing: []dto.MissingLibraryFileDTO{}}
	if service == nil || service.files == nil {
		return response, nil, nil
	}
	items, err := service.files.List(ctx)
	if err != nil {
		return response, nil, err
	}
	now := service.now()
	missingItems := make([]library.LibraryFile, 0)
	for _, item := range items {
		if item.State.Deleted || strings.TrimSpace(item.Storage.LocalPath) == "" {
			continue
		}
		if filter != nil {
			if _, ok := filter[item.ID]; !ok {
				continue
			}
		}
		response.Checked++
		if localFileExists(item.Storage.LocalPath) {
			if updateState && item.State.LastError == missingLocalFileError {
				item.State.LastError = ""
				item.State.LastChecked = now.Format(time.RFC3339)
				item.UpdatedAt = now
				if err := service.files.Save(ctx, item); err != nil {
					return response, nil, err
				}
				service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
			}
			continue
		}
		if updateState {
			item.State.LastError = missingLocalFileError
			item.State.LastChecked = now.Format(time.RFC3339)
			item.UpdatedAt = now
			if err := service.files.Save(ctx, item); err != nil {
				return response, nil, err
			}
			service.syncListenLocalTrackFromFile(ctx, item, nil)
			service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
		}
		missingItems = append(missingItems, item)
		response.Missing = append(response.Missing, toMissingLibraryFileDTO(item))
	}
	return response, missingItems, nil
}

func (service *LibraryService) listMissingListenLocalFileItems(ctx context.Context, updateState bool, filter map[string]struct{}) (dto.ListMissingLibraryFilesResponse, []library.LibraryFile, error) {
	response := dto.ListMissingLibraryFilesResponse{Missing: []dto.MissingLibraryFileDTO{}}
	if service == nil || service.files == nil || service.localTracks == nil {
		return response, nil, nil
	}
	tracks, err := service.localTracks.List(ctx, library.ListenLocalTrackListOptions{IncludeUnavailable: true})
	if err != nil {
		return response, nil, err
	}
	now := service.now()
	missingItems := make([]library.LibraryFile, 0)
	for _, track := range tracks {
		fileID := strings.TrimSpace(track.FileID)
		if fileID == "" {
			continue
		}
		if filter != nil {
			if _, ok := filter[fileID]; !ok {
				continue
			}
		}
		item, err := service.files.Get(ctx, fileID)
		if err != nil {
			continue
		}
		if item.State.Deleted || strings.TrimSpace(item.Storage.LocalPath) == "" {
			continue
		}
		response.Checked++
		if localFileExists(item.Storage.LocalPath) {
			if updateState && item.State.LastError == missingLocalFileError {
				item.State.LastError = ""
				item.State.LastChecked = now.Format(time.RFC3339)
				item.UpdatedAt = now
				if err := service.files.Save(ctx, item); err != nil {
					return response, nil, err
				}
				service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
			}
			if updateState && track.Availability == library.ListenLocalTrackMissing {
				track.Availability = library.ListenLocalTrackAvailable
				track.LocalPath = item.Storage.LocalPath
				track.LastCheckedAt = now
				track.ProbeError = ""
				track.UpdatedAt = now
				if err := service.localTracks.Save(ctx, track); err != nil {
					return response, nil, err
				}
			}
			continue
		}
		if updateState {
			item.State.LastError = missingLocalFileError
			item.State.LastChecked = now.Format(time.RFC3339)
			item.UpdatedAt = now
			if err := service.files.Save(ctx, item); err != nil {
				return response, nil, err
			}
			service.syncListenLocalTrackFromFile(ctx, item, nil)
			service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
		}
		missingItems = append(missingItems, item)
		response.Missing = append(response.Missing, toMissingLibraryFileDTO(item))
	}
	return response, missingItems, nil
}

func toMissingLibraryFileDTO(item library.LibraryFile) dto.MissingLibraryFileDTO {
	result := dto.MissingLibraryFileDTO{
		FileID:      item.ID,
		LibraryID:   item.LibraryID,
		Name:        resolveLibraryFileDisplayName(item),
		Kind:        string(item.Kind),
		OldPath:     item.Storage.LocalPath,
		Format:      mediaFormatFromFile(item),
		LastChecked: item.State.LastChecked,
		LastError:   item.State.LastError,
		UpdatedAt:   item.UpdatedAt.Format(time.RFC3339),
	}
	if item.Media != nil {
		result.SizeBytes = item.Media.SizeBytes
		result.DurationMs = item.Media.DurationMs
	}
	return result
}

type relinkCandidateIndex struct {
	byName     map[string][]string
	byBaseName map[string][]string
}

func buildRelinkCandidateIndex(directory string) (relinkCandidateIndex, int, error) {
	index := relinkCandidateIndex{
		byName:     map[string][]string{},
		byBaseName: map[string][]string{},
	}
	scanned := 0
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry == nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info == nil || !info.Mode().IsRegular() {
			return nil
		}
		scanned++
		base := filepath.Base(path)
		nameKey := normalizeRelinkNameKey(base)
		if nameKey != "" {
			index.byName[nameKey] = append(index.byName[nameKey], path)
		}
		baseKey := normalizeRelinkNameKey(strings.TrimSuffix(base, filepath.Ext(base)))
		if baseKey != "" {
			index.byBaseName[baseKey] = append(index.byBaseName[baseKey], path)
		}
		return nil
	})
	return index, scanned, err
}

func relinkCandidatePathsForFile(index relinkCandidateIndex, item library.LibraryFile) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0)
	appendPaths := func(paths []string) {
		for _, path := range paths {
			cleaned := filepath.Clean(strings.TrimSpace(path))
			if cleaned == "" {
				continue
			}
			if _, exists := seen[cleaned]; exists {
				continue
			}
			seen[cleaned] = struct{}{}
			result = append(result, cleaned)
		}
	}
	oldBase := filepath.Base(strings.TrimSpace(item.Storage.LocalPath))
	if oldBase != "" && oldBase != "." {
		appendPaths(index.byName[normalizeRelinkNameKey(oldBase)])
		appendPaths(index.byBaseName[normalizeRelinkNameKey(strings.TrimSuffix(oldBase, filepath.Ext(oldBase)))])
	}
	if item.Name != "" {
		appendPaths(index.byName[normalizeRelinkNameKey(item.Name)])
		appendPaths(index.byBaseName[normalizeRelinkNameKey(strings.TrimSuffix(item.Name, filepath.Ext(item.Name)))])
	}
	return result
}

func (service *LibraryService) evaluateLibraryRelinkMatch(ctx context.Context, item library.LibraryFile, candidatePath string) dto.LibraryRelinkMatchDTO {
	cleaned := filepath.Clean(strings.TrimSpace(candidatePath))
	probe := service.probeLocalMedia(ctx, cleaned)
	score := 0
	reasons := make([]string, 0, 5)

	oldBase := filepath.Base(strings.TrimSpace(item.Storage.LocalPath))
	newBase := filepath.Base(cleaned)
	if strings.EqualFold(oldBase, newBase) && oldBase != "" && oldBase != "." {
		score += 50
		reasons = append(reasons, "file_name")
	} else if sameRelinkBaseName(oldBase, newBase) {
		score += 30
		reasons = append(reasons, "base_name")
	}

	expectedFormat := strings.ToLower(strings.TrimSpace(mediaFormatFromFile(item)))
	candidateFormat := strings.ToLower(strings.TrimSpace(probe.Format))
	if expectedFormat != "" && candidateFormat != "" && expectedFormat == candidateFormat {
		score += 20
		reasons = append(reasons, "format")
	}

	if expectedSize := mediaSizeFromFile(item); expectedSize != nil && *expectedSize > 0 && probe.SizeBytes > 0 {
		switch {
		case *expectedSize == probe.SizeBytes:
			score += 40
			reasons = append(reasons, "size")
		case relinkSizesNearlyEqual(*expectedSize, probe.SizeBytes):
			score += 15
			reasons = append(reasons, "near_size")
		}
	}

	if item.Media != nil && item.Media.DurationMs != nil && *item.Media.DurationMs > 0 && probe.DurationMs > 0 {
		if absInt64(*item.Media.DurationMs-probe.DurationMs) <= 2000 {
			score += 25
			reasons = append(reasons, "duration")
		}
	}
	if item.Media != nil && item.Media.Width != nil && item.Media.Height != nil && probe.Width > 0 && probe.Height > 0 {
		if *item.Media.Width == probe.Width && *item.Media.Height == probe.Height {
			score += 10
			reasons = append(reasons, "dimensions")
		}
	}

	confidence := "mismatch"
	if score >= libraryRelinkMinimumScore {
		confidence = libraryRelinkConfidenceCandidate
	}
	if hasRelinkReason(reasons, "file_name") && hasRelinkReason(reasons, "size") {
		confidence = libraryRelinkConfidenceExact
	}

	result := dto.LibraryRelinkMatchDTO{
		FileID:     item.ID,
		LibraryID:  item.LibraryID,
		Name:       resolveLibraryFileDisplayName(item),
		Kind:       string(item.Kind),
		OldPath:    item.Storage.LocalPath,
		NewPath:    cleaned,
		Format:     firstNonEmpty(candidateFormat, expectedFormat),
		Score:      score,
		Confidence: confidence,
		Reasons:    reasons,
	}
	if probe.SizeBytes > 0 {
		value := probe.SizeBytes
		result.SizeBytes = &value
	}
	if probe.DurationMs > 0 {
		value := probe.DurationMs
		result.DurationMs = &value
	}
	return result
}

func libraryRelinkMatchAccepted(match dto.LibraryRelinkMatchDTO) bool {
	switch strings.TrimSpace(match.Confidence) {
	case libraryRelinkConfidenceExact, libraryRelinkConfidenceCandidate:
		return match.Score >= libraryRelinkMinimumScore
	default:
		return false
	}
}

func (service *LibraryService) applyLibraryFileRelink(ctx context.Context, item library.LibraryFile, newPath string) (dto.LibraryFileDTO, error) {
	oldPath := strings.TrimSpace(item.Storage.LocalPath)
	cleaned := filepath.Clean(strings.TrimSpace(newPath))
	now := service.now()
	item.Storage.LocalPath = cleaned
	item.Name = resolveStoredFileName(cleaned, item.Name)
	item.Media = mergeRelinkMediaInfo(item.Media, service.probeLocalMedia(ctx, cleaned))
	item.State.LastError = ""
	item.State.LastChecked = now.Format(time.RFC3339)
	item.UpdatedAt = now
	if err := service.files.Save(ctx, item); err != nil {
		return dto.LibraryFileDTO{}, err
	}
	if err := service.touchLibrary(ctx, item.LibraryID, now); err != nil {
		return dto.LibraryFileDTO{}, err
	}
	if err := service.updateOperationOutputPathReferences(ctx, item, oldPath, cleaned); err != nil {
		return dto.LibraryFileDTO{}, err
	}
	service.refreshListenLocalTracksForLibrary(ctx, item.LibraryID)
	fileDTO, err := service.buildFileDTO(ctx, item)
	if err != nil {
		return dto.LibraryFileDTO{}, err
	}
	service.publishFileUpdate(fileDTO)
	return fileDTO, nil
}

func mergeRelinkMediaInfo(current *library.MediaInfo, probe mediaProbe) *library.MediaInfo {
	next := &library.MediaInfo{}
	if current != nil {
		copyValue := *current
		next = &copyValue
	}
	probed := probe.toMediaInfo()
	if strings.TrimSpace(probed.Format) != "" {
		next.Format = probed.Format
	}
	if strings.TrimSpace(probed.Codec) != "" {
		next.Codec = probed.Codec
	}
	if strings.TrimSpace(probed.VideoCodec) != "" {
		next.VideoCodec = probed.VideoCodec
	}
	if strings.TrimSpace(probed.AudioCodec) != "" {
		next.AudioCodec = probed.AudioCodec
	}
	if probed.DurationMs != nil {
		next.DurationMs = probed.DurationMs
	}
	if probed.Width != nil {
		next.Width = probed.Width
	}
	if probed.Height != nil {
		next.Height = probed.Height
	}
	if probed.FrameRate != nil {
		next.FrameRate = probed.FrameRate
	}
	if probed.BitrateKbps != nil {
		next.BitrateKbps = probed.BitrateKbps
	}
	if probed.VideoBitrateKbps != nil {
		next.VideoBitrateKbps = probed.VideoBitrateKbps
	}
	if probed.AudioBitrateKbps != nil {
		next.AudioBitrateKbps = probed.AudioBitrateKbps
	}
	if probed.Channels != nil {
		next.Channels = probed.Channels
	}
	if probed.SizeBytes != nil {
		next.SizeBytes = probed.SizeBytes
	}
	if probed.DPI != nil {
		next.DPI = probed.DPI
	}
	return next
}

func (service *LibraryService) refreshListenLocalTracksForLibrary(ctx context.Context, libraryID string) {
	if service == nil || service.files == nil || service.localTracks == nil {
		return
	}
	files, err := service.files.ListByLibraryID(ctx, libraryID)
	if err != nil {
		return
	}
	lookup := buildListenLocalCoverLookup(files)
	response := dto.ListenLocalIndexRefreshResponse{}
	for _, fileItem := range files {
		service.refreshListenLocalTrack(ctx, fileItem, nil, lookup, &response)
	}
}

func (service *LibraryService) updateOperationOutputPathReferences(ctx context.Context, item library.LibraryFile, oldPath string, newPath string) error {
	if service == nil || service.operations == nil {
		return nil
	}
	ids := make([]string, 0, 2)
	for _, id := range []string{item.LatestOperationID, item.Origin.OperationID} {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	seen := map[string]struct{}{}
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		operation, err := service.operations.Get(ctx, id)
		if err != nil {
			continue
		}
		outputJSON, changed := replaceOperationOutputPath(operation.OutputJSON, oldPath, newPath)
		if !changed {
			continue
		}
		operation.OutputJSON = outputJSON
		if err := service.operations.Save(ctx, operation); err != nil {
			return err
		}
		service.publishOperationUpdate(toOperationDTO(operation))
	}
	return nil
}

func replaceOperationOutputPath(outputJSON string, oldPath string, newPath string) (string, bool) {
	oldClean := filepath.Clean(strings.TrimSpace(oldPath))
	newClean := filepath.Clean(strings.TrimSpace(newPath))
	if oldClean == "." || newClean == "." || oldClean == newClean {
		return outputJSON, false
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(outputJSON)), &payload); err != nil {
		return outputJSON, false
	}
	changed := false
	replaceString := func(value string) string {
		if filepath.Clean(strings.TrimSpace(value)) == oldClean {
			changed = true
			return newClean
		}
		return value
	}
	for _, key := range operationOutputArtifactStringKeys {
		if value, ok := payload[key].(string); ok {
			payload[key] = replaceString(value)
		}
	}
	for _, key := range operationOutputArtifactListKeys {
		switch values := payload[key].(type) {
		case []any:
			next := make([]any, 0, len(values))
			for _, value := range values {
				if text, ok := value.(string); ok {
					next = append(next, replaceString(text))
				} else {
					next = append(next, value)
				}
			}
			payload[key] = next
		case []string:
			next := make([]string, 0, len(values))
			for _, value := range values {
				next = append(next, replaceString(value))
			}
			payload[key] = next
		}
	}
	if !changed {
		return outputJSON, false
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return outputJSON, false
	}
	return string(encoded), true
}

func resolveRelinkSelectedPath(path string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("path is required")
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory")
	}
	return cleaned, nil
}

func resolveRelinkScanDirectory(path string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("directory is required")
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}
	return cleaned, nil
}

func normalizeRelinkNameKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sameRelinkBaseName(left string, right string) bool {
	leftBase := normalizeRelinkNameKey(strings.TrimSuffix(filepath.Base(strings.TrimSpace(left)), filepath.Ext(left)))
	rightBase := normalizeRelinkNameKey(strings.TrimSuffix(filepath.Base(strings.TrimSpace(right)), filepath.Ext(right)))
	return leftBase != "" && leftBase == rightBase
}

func relinkSizesNearlyEqual(left int64, right int64) bool {
	diff := absInt64(left - right)
	allowance := left / 100
	if allowance < 4096 {
		allowance = 4096
	}
	return diff <= allowance
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func hasRelinkReason(reasons []string, target string) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}
