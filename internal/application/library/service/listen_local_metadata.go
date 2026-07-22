package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const (
	listenLocalMetadataMaxTextLength = 512
	listenLocalMetadataWriteTimeout  = 10 * time.Minute
)

var listenLocalMetadataExtensions = map[string]struct{}{
	".flac": {},
	".m4a":  {},
	".m4b":  {},
	".mp3":  {},
	".mp4":  {},
	".oga":  {},
	".ogg":  {},
	".opus": {},
}

func (service *LibraryService) UpdateListenLocalTrackMetadata(
	ctx context.Context,
	request dto.UpdateListenLocalTrackMetadataRequest,
) (dto.ListenLocalTrackDTO, error) {
	if service == nil || service.localTracks == nil {
		return dto.ListenLocalTrackDTO{}, library.ErrFileNotFound
	}
	normalized, err := normalizeListenLocalMetadataRequest(request)
	if err != nil {
		return dto.ListenLocalTrackDTO{}, err
	}

	unlock := service.lockListenLocalTrackMutation(normalized.FileID)
	defer unlock()

	track, err := service.localTracks.Get(ctx, normalized.FileID)
	if err != nil {
		return dto.ListenLocalTrackDTO{}, err
	}
	if track.Availability != library.ListenLocalTrackAvailable {
		return dto.ListenLocalTrackDTO{}, library.ErrFileNotFound
	}
	path := strings.TrimSpace(track.LocalPath)
	hasLibraryFile := false
	originalDisplayName := ""
	if service.files != nil {
		fileItem, fileErr := service.files.Get(ctx, normalized.FileID)
		if fileErr != nil {
			return dto.ListenLocalTrackDTO{}, fileErr
		}
		if fileItem.State.Deleted || !sameListenLocalPath(path, fileItem.Storage.LocalPath) {
			return dto.ListenLocalTrackDTO{}, library.ErrListenLocalFileChanged
		}
		originalDisplayName = fileItem.DisplayName
		hasLibraryFile = true
	}
	if !listenLocalMetadataWritable(path) {
		return dto.ListenLocalTrackDTO{}, fmt.Errorf(
			"%w: %s",
			library.ErrListenLocalMetadataUnsupported,
			strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
		)
	}

	writer := service.localMetadataWriter
	if writer == nil {
		writer = service.writeListenLocalMetadataWithFFmpeg
	}
	writeCtx, cancel := context.WithTimeout(ctx, listenLocalMetadataWriteTimeout)
	defer cancel()
	if err := writer(writeCtx, path, normalized); err != nil {
		return dto.ListenLocalTrackDTO{}, err
	}

	stat, err := os.Stat(path)
	if err != nil || stat == nil || !stat.Mode().IsRegular() {
		if err == nil {
			err = errors.New("updated local track is not a regular file")
		}
		return dto.ListenLocalTrackDTO{}, fmt.Errorf("%w: %v", library.ErrListenLocalMetadataIndexStale, err)
	}
	now := service.now()
	sizeBytes := stat.Size()
	track.Title = normalized.Title
	track.Author = normalized.Author
	track.Album = normalized.Album
	track.AlbumArtist = normalized.AlbumArtist
	track.Genre = normalized.Genre
	track.TrackNumber = normalized.TrackNumber
	track.DiscNumber = normalized.DiscNumber
	track.Year = normalized.Year
	track.SizeBytes = &sizeBytes
	track.ModTimeUnix = stat.ModTime().Unix()
	track.Availability = library.ListenLocalTrackAvailable
	track.LastCheckedAt = now
	track.ProbeError = ""
	track.UpdatedAt = now
	if saveErr := service.localTracks.Save(ctx, track); saveErr != nil {
		// The atomic file replacement has already committed. Retry once while the
		// per-file lock is held so a transient SQLite busy error does not leave the
		// index stale. If persistence is still unavailable, return an explicit
		// recoverable error; a later local-index refresh reads the committed tags.
		if retryErr := service.localTracks.Save(ctx, track); retryErr != nil {
			return dto.ListenLocalTrackDTO{}, fmt.Errorf(
				"%w: initial save: %v; retry: %v",
				library.ErrListenLocalMetadataIndexStale,
				saveErr,
				retryErr,
			)
		}
	}
	if hasLibraryFile {
		if syncErr := service.syncCommittedListenLocalMetadata(
			ctx,
			normalized.FileID,
			path,
			originalDisplayName,
			normalized,
			sizeBytes,
			now,
		); syncErr != nil {
			return dto.ListenLocalTrackDTO{}, fmt.Errorf(
				"%w: synchronize Library and Catalog metadata: %w",
				library.ErrListenLocalMetadataIndexStale,
				syncErr,
			)
		}
	}
	return toListenLocalTrackDTO(track), nil
}

func (service *LibraryService) syncCommittedListenLocalMetadata(
	ctx context.Context,
	fileID string,
	expectedPath string,
	originalDisplayName string,
	metadata dto.UpdateListenLocalTrackMetadataRequest,
	sizeBytes int64,
	updatedAt time.Time,
) error {
	// The physical tag rewrite can take several minutes. LibraryFile is mutable
	// through other surfaces during that time (rename, relink, delete, probes),
	// so never save the snapshot read before the rewrite. Re-read and merge only
	// the fields owned by this operation on every attempt; this also prevents a
	// retry after a transient database error from replaying another stale copy.
	var fileItem library.LibraryFile
	var firstSaveErr error
	for attempt := 0; attempt < 2; attempt++ {
		current, err := service.files.Get(ctx, fileID)
		if err != nil {
			if errors.Is(err, library.ErrFileNotFound) {
				return fmt.Errorf("%w: Library file was removed after metadata write", library.ErrListenLocalFileChanged)
			}
			return fmt.Errorf("reload Library file after metadata write: %w", err)
		}
		if current.State.Deleted || !sameListenLocalPath(expectedPath, current.Storage.LocalPath) {
			return library.ErrListenLocalFileChanged
		}
		// DisplayName is the Library list title. Apply the tag title only when
		// it has not been renamed since the pre-write snapshot; an explicit
		// concurrent RenameFile always wins.
		if current.DisplayName == originalDisplayName {
			current.DisplayName = metadata.Title
		}
		current.Metadata.Title = metadata.Title
		current.Metadata.Author = metadata.Author
		if current.UpdatedAt.Before(updatedAt) {
			current.UpdatedAt = updatedAt
		}
		current.State.LastChecked = updatedAt.Format(time.RFC3339)
		if current.Media == nil {
			current.Media = &library.MediaInfo{SizeBytes: &sizeBytes}
		} else {
			// A repository may return a shallow domain copy. Clone Media before
			// changing SizeBytes so a failed Save cannot mutate cached state.
			media := *current.Media
			media.SizeBytes = &sizeBytes
			current.Media = &media
		}
		if err := service.files.Save(ctx, current); err != nil {
			if firstSaveErr == nil {
				firstSaveErr = err
				continue
			}
			return fmt.Errorf("save Library file metadata: initial save: %v; retry: %w", firstSaveErr, err)
		}
		fileItem = current
		break
	}
	if fileItem.ID == "" {
		return fmt.Errorf("save Library file metadata: %w", firstSaveErr)
	}
	if service.libraries != nil {
		if err := retryListenLocalMetadataIndexWrite(func() error {
			return service.touchLibrary(ctx, fileItem.LibraryID, fileItem.UpdatedAt)
		}); err != nil {
			service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
			return fmt.Errorf("touch Library after metadata update: %w", err)
		}
	}
	if err := service.syncCatalogProjection(ctx, fileItem.LibraryID); err != nil {
		service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
		return err
	}
	if service.catalogMetadataSync != nil {
		if err := service.catalogMetadataSync.SyncListenLocalTrackMetadata(ctx, fileItem, metadata); err != nil {
			service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
			return fmt.Errorf("update Catalog item metadata: %w", err)
		}
	}
	service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
	return nil
}

func retryListenLocalMetadataIndexWrite(write func() error) error {
	var firstErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := write(); err == nil {
			return nil
		} else if firstErr == nil {
			firstErr = err
		} else {
			return fmt.Errorf("initial save: %v; retry: %w", firstErr, err)
		}
	}
	return firstErr
}

func normalizeListenLocalMetadataRequest(
	request dto.UpdateListenLocalTrackMetadataRequest,
) (dto.UpdateListenLocalTrackMetadataRequest, error) {
	request.FileID = strings.TrimSpace(request.FileID)
	request.Title = strings.TrimSpace(request.Title)
	request.Author = strings.TrimSpace(request.Author)
	request.Album = strings.TrimSpace(request.Album)
	request.AlbumArtist = strings.TrimSpace(request.AlbumArtist)
	request.Genre = strings.TrimSpace(request.Genre)
	if request.FileID == "" || request.Title == "" {
		return dto.UpdateListenLocalTrackMetadataRequest{}, fmt.Errorf(
			"%w: fileId and title are required",
			library.ErrInvalidListenLocalMetadata,
		)
	}
	for name, value := range map[string]string{
		"title":        request.Title,
		"author":       request.Author,
		"album":        request.Album,
		"album artist": request.AlbumArtist,
		"genre":        request.Genre,
	} {
		if strings.ContainsRune(value, '\x00') || len([]rune(value)) > listenLocalMetadataMaxTextLength {
			return dto.UpdateListenLocalTrackMetadataRequest{}, fmt.Errorf(
				"%w: %s is invalid",
				library.ErrInvalidListenLocalMetadata,
				name,
			)
		}
	}
	if request.TrackNumber < 0 || request.TrackNumber > 9999 ||
		request.DiscNumber < 0 || request.DiscNumber > 9999 ||
		(request.Year != 0 && (request.Year < 1000 || request.Year > 9999)) {
		return dto.UpdateListenLocalTrackMetadataRequest{}, fmt.Errorf(
			"%w: track, disc, or year is out of range",
			library.ErrInvalidListenLocalMetadata,
		)
	}
	return request, nil
}

func listenLocalMetadataWritable(path string) bool {
	_, ok := listenLocalMetadataExtensions[strings.ToLower(filepath.Ext(strings.TrimSpace(path)))]
	return ok
}

func sameListenLocalPath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func (service *LibraryService) writeListenLocalMetadataWithFFmpeg(
	ctx context.Context,
	path string,
	metadata dto.UpdateListenLocalTrackMetadataRequest,
) (returnErr error) {
	path = strings.TrimSpace(path)
	if !listenLocalMetadataWritable(path) {
		return library.ErrListenLocalMetadataUnsupported
	}
	original, err := snapshotListenLocalFile(ctx, path)
	if err != nil {
		return listenLocalMetadataFilesystemError(err)
	}
	linkCount, err := listenLocalFileLinkCount(path, original.info)
	if err != nil {
		return err
	}
	if linkCount > 1 {
		return listenLocalMetadataPreservationError("hard-linked copies")
	}
	ffmpegPath, err := resolveFFmpegExecPath(ctx, service.tools)
	if err != nil {
		return err
	}
	ffprobePath, err := resolveFFprobeExecPath(ctx, service.tools)
	if err != nil {
		return err
	}
	originalManifest, err := probeListenLocalMetadataManifest(ctx, ffprobePath, path)
	if err != nil {
		return err
	}
	originalRawTags, err := probeListenLocalRawTagManifest(ctx, path)
	if err != nil {
		return err
	}

	extension := strings.ToLower(filepath.Ext(path))
	temporary, err := os.CreateTemp(filepath.Dir(path), ".xiadown-metadata-*"+extension)
	if err != nil {
		return listenLocalMetadataFilesystemError(err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		cleanupErr := os.Remove(temporaryPath)
		if cleanupErr == nil || os.IsNotExist(cleanupErr) {
			return
		}
		if returnErr == nil {
			returnErr = fmt.Errorf("clean local metadata temporary file: %w", cleanupErr)
			return
		}
		returnErr = fmt.Errorf("%w; temporary file cleanup also failed: %v", returnErr, cleanupErr)
	}()
	if closeErr := temporary.Close(); closeErr != nil {
		return closeErr
	}

	args := buildListenLocalMetadataFFmpegArgsWithManifest(
		path,
		temporaryPath,
		metadata,
		originalManifest,
		originalRawTags.hasID3v1,
	)
	command := exec.CommandContext(ctx, ffmpegPath, args...)
	configureLocalMediaToolCommand(command)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("write local track metadata: %s", detail)
	}
	if err := prepareListenLocalMetadataReplacement(path, temporaryPath, original.info); err != nil {
		return listenLocalMetadataFilesystemError(err)
	}
	if err := syncListenLocalMetadataFile(temporaryPath); err != nil {
		return listenLocalMetadataFilesystemError(err)
	}
	probe, err := service.ffprobeLocalMedia(ctx, temporaryPath)
	if err != nil {
		return fmt.Errorf("verify local track metadata: %w", err)
	}
	if err := verifyListenLocalMetadata(probe, metadata); err != nil {
		return err
	}
	updatedManifest, err := probeListenLocalMetadataManifest(ctx, ffprobePath, temporaryPath)
	if err != nil {
		return err
	}
	if err := verifyListenLocalMetadataPreserved(originalManifest, updatedManifest); err != nil {
		return err
	}
	updatedRawTags, err := probeListenLocalRawTagManifest(ctx, temporaryPath)
	if err != nil {
		return err
	}
	if err := verifyListenLocalRawTagsPreserved(originalRawTags, updatedRawTags); err != nil {
		return err
	}

	current, err := snapshotListenLocalFile(ctx, path)
	if err != nil {
		if errors.Is(err, library.ErrListenLocalMetadataUnsupported) {
			return library.ErrListenLocalFileChanged
		}
		return listenLocalMetadataFilesystemError(err)
	}
	if !sameListenLocalFileSnapshot(original, current) {
		return library.ErrListenLocalFileChanged
	}
	if err := replaceListenLocalMetadataFile(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func listenLocalMetadataFilesystemError(err error) error {
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %v", library.ErrListenLocalFileChanged, err)
	}
	if err != nil && os.IsPermission(err) && !errors.Is(err, library.ErrListenLocalFilePermission) {
		return fmt.Errorf("%w: %v", library.ErrListenLocalFilePermission, err)
	}
	return err
}

func buildListenLocalMetadataFFmpegArgs(
	inputPath string,
	outputPath string,
	metadata dto.UpdateListenLocalTrackMetadataRequest,
) []string {
	return buildListenLocalMetadataFFmpegArgsWithManifest(
		inputPath,
		outputPath,
		metadata,
		listenLocalMetadataManifest{},
		false,
	)
}

func buildListenLocalMetadataFFmpegArgsWithManifest(
	inputPath string,
	outputPath string,
	metadata dto.UpdateListenLocalTrackMetadataRequest,
	original listenLocalMetadataManifest,
	writeID3v1 bool,
) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
	}
	args = appendLocalMediaFFmpegInput(args, inputPath)
	args = append(args,
		"-map", "0",
		"-map_metadata", "0",
		"-map_chapters", "0",
		"-c", "copy",
	)
	extension := strings.ToLower(filepath.Ext(outputPath))
	if extension == ".m4a" || extension == ".m4b" || extension == ".mp4" {
		// FFmpeg exposes the chapter track in MP4/M4A as bin_data, but the ipod
		// muxer rejects mapping that track back verbatim. Mapping chapters below
		// recreates the equivalent text track. Any unrelated data stream loss is
		// caught by the pre-replacement preservation manifest.
		args = append(args, "-map", "-0:d?")
	}
	fields := [][2]string{
		{"title", metadata.Title},
		{"artist", metadata.Author},
		{"album", metadata.Album},
		{"album_artist", metadata.AlbumArtist},
		{"genre", metadata.Genre},
		{"track", listenLocalMetadataOrdinalWithTotal(metadata.TrackNumber, original, "track", "tracknumber", "track_number")},
		{"disc", listenLocalMetadataOrdinalWithTotal(metadata.DiscNumber, original, "disc", "discnumber", "disc_number", "disk")},
		{"date", optionalListenLocalMetadataNumber(metadata.Year)},
	}
	for _, field := range fields {
		args = append(args, "-metadata", field[0]+"="+field[1])
	}
	if extension == ".ogg" || extension == ".oga" || extension == ".opus" {
		// Ogg-family muxers store Vorbis comments on the audio stream rather
		// than at format scope.
		for _, field := range fields {
			args = append(args, "-metadata:s:a:0", field[0]+"="+field[1])
		}
	}
	if extension == ".mp3" {
		args = append(args, "-id3v2_version", "3")
		if writeID3v1 {
			args = append(args, "-write_id3v1", "1")
		}
	}
	return append(args, outputPath)
}

func listenLocalMetadataOrdinalWithTotal(
	value int,
	manifest listenLocalMetadataManifest,
	tagKeys ...string,
) string {
	result := optionalListenLocalMetadataNumber(value)
	if value <= 0 {
		return result
	}
	tags := make([]map[string]string, 0, 1+len(manifest.Streams))
	tags = append(tags, manifest.Format.Tags)
	for _, stream := range manifest.Streams {
		if strings.EqualFold(strings.TrimSpace(stream.CodecType), "audio") {
			tags = append(tags, stream.Tags)
		}
	}
	for _, candidateTags := range tags {
		normalized := normalizeListenLocalManifestTags(candidateTags)
		for _, key := range tagKeys {
			candidate := strings.TrimSpace(normalized[strings.ToLower(strings.TrimSpace(key))])
			parts := strings.SplitN(candidate, "/", 2)
			if len(parts) != 2 {
				continue
			}
			total, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err == nil && total > 0 {
				if value > total {
					total = value
				}
				return result + "/" + strconv.Itoa(total)
			}
		}
	}
	return result
}

func optionalListenLocalMetadataNumber(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func verifyListenLocalMetadata(
	probe mediaProbe,
	metadata dto.UpdateListenLocalTrackMetadataRequest,
) error {
	stringFields := [][3]string{
		{"title", probe.Title, metadata.Title},
		{"artist", probe.Artist, metadata.Author},
		{"album", probe.Album, metadata.Album},
		{"album artist", probe.AlbumArtist, metadata.AlbumArtist},
		{"genre", probe.Genre, metadata.Genre},
	}
	for _, field := range stringFields {
		if strings.TrimSpace(field[1]) != strings.TrimSpace(field[2]) {
			return fmt.Errorf("verify local track metadata: %s was not written", field[0])
		}
	}
	if probe.TrackNumber != metadata.TrackNumber || probe.DiscNumber != metadata.DiscNumber || probe.Year != metadata.Year {
		return errors.New("verify local track metadata: numeric tags were not written")
	}
	return nil
}

func syncListenLocalMetadataFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
