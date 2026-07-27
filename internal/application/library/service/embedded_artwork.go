package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/domain/library"
)

const (
	embeddedArtworkCacheVersion = "v1"
	embeddedArtworkWorkerCount  = 2
	embeddedArtworkMaxBytes     = 8 << 20
)

var embeddedArtworkNamespace = uuid.MustParse(
	"c8ee5a32-c4e1-4418-8bfc-89d1128d53a4",
)

type embeddedArtworkResult struct {
	source  library.LibraryFile
	cover   library.LibraryFile
	probe   mediaProbe
	created bool
}

// BackfillEmbeddedArtwork repairs older root imports in the background. A
// fingerprinted negative marker avoids re-running ffprobe for audio files that
// do not contain artwork, while changed files are checked again.
func (service *LibraryService) BackfillEmbeddedArtwork(ctx context.Context) error {
	if service == nil || service.files == nil ||
		strings.TrimSpace(service.embeddedArtworkDirectory) == "" {
		return nil
	}
	if err := ensureEmbeddedArtworkDirectory(service.embeddedArtworkDirectory); err != nil {
		return err
	}
	files, err := service.files.List(ctx)
	if err != nil {
		return err
	}
	byLibrary := make(map[string][]library.LibraryFile)
	for _, item := range files {
		byLibrary[item.LibraryID] = append(byLibrary[item.LibraryID], item)
	}
	candidates := make([]library.LibraryFile, 0)
	existing := make([]embeddedArtworkResult, 0)
	for _, item := range files {
		if !embeddedArtworkSourceCandidate(item) {
			continue
		}
		if cover, ok := usableRegisteredEmbeddedArtwork(
			item,
			byLibrary[item.LibraryID],
		); ok {
			if cover.ID == embeddedArtworkFileID(item.ID) &&
				service.embeddedArtworkNeedsReconcile(ctx, item, cover) {
				existing = append(existing, embeddedArtworkResult{
					source: item,
					cover:  cover,
				})
			}
			continue
		}
		if !service.embeddedArtworkSourceAvailable(ctx, item) {
			continue
		}
		if marker, markerErr := service.embeddedArtworkNegativeMarker(item); markerErr == nil &&
			pathExists(marker) {
			continue
		}
		candidates = append(candidates, item)
	}
	// Bootstrap runs the durable Catalog projection before this background
	// repair starts. Existing generated covers therefore need only their local
	// music association reconciled here; re-projecting every legacy bundle a
	// second time would compete with the storage-root startup scan.
	if err := service.reconcileEmbeddedArtwork(ctx, existing, false); err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	workerCount := min(embeddedArtworkWorkerCount, len(candidates))
	jobs := make(chan library.LibraryFile)
	results := make(chan embeddedArtworkResult, len(candidates))
	errs := make(chan error, len(candidates))
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case source, ok := <-jobs:
					if !ok {
						return
					}
					result, ensureErr := service.ensureEmbeddedArtwork(
						ctx,
						source,
						nil,
					)
					if ensureErr != nil {
						errs <- ensureErr
						continue
					}
					if result.created {
						results <- result
					}
				}
			}
		}()
	}

dispatch:
	for _, candidate := range candidates {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- candidate:
		}
	}
	close(jobs)
	workers.Wait()
	close(results)
	close(errs)

	changed := make([]embeddedArtworkResult, 0, len(results))
	for result := range results {
		changed = append(changed, result)
	}

	reconcileCtx := ctx
	reconcileCancel := func() {}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) && len(changed) > 0 {
		reconcileCtx, reconcileCancel = context.WithTimeout(
			context.WithoutCancel(ctx),
			30*time.Second,
		)
	}
	defer reconcileCancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		if err := service.reconcileEmbeddedArtwork(
			reconcileCtx,
			changed,
			true,
		); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case firstErr := <-errs:
		return firstErr
	default:
		return nil
	}
}

func (service *LibraryService) ensureEmbeddedArtwork(
	ctx context.Context,
	source library.LibraryFile,
	providedProbe *mediaProbe,
) (embeddedArtworkResult, error) {
	result := embeddedArtworkResult{source: source}
	if !embeddedArtworkSourceCandidate(source) ||
		strings.TrimSpace(service.embeddedArtworkDirectory) == "" {
		return result, nil
	}
	if err := ensureEmbeddedArtworkDirectory(service.embeddedArtworkDirectory); err != nil {
		return result, err
	}
	info, err := os.Stat(source.Storage.LocalPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return result, nil
	}
	libraryFiles, err := service.files.ListByLibraryID(ctx, source.LibraryID)
	if err == nil && usableEmbeddedArtworkAlreadyRegistered(source, libraryFiles) {
		return result, nil
	}

	probe := mediaProbe{}
	if providedProbe != nil {
		probe = *providedProbe
	} else {
		marker, markerErr := service.embeddedArtworkNegativeMarker(source)
		if markerErr == nil && pathExists(marker) {
			return result, nil
		}
		probe, err = service.probeRequiredMedia(ctx, source.Storage.LocalPath)
		if err != nil {
			return result, err
		}
	}
	result.probe = probe
	if !probe.StreamInfo {
		return result, nil
	}
	if !isAudioOnlyProbe(probe) || probe.AttachedPicCount <= 0 {
		if marker, markerErr := service.embeddedArtworkNegativeMarker(source); markerErr == nil {
			_ = os.WriteFile(marker, nil, 0o600)
		}
		return result, nil
	}

	service.embeddedArtworkMu.Lock()
	defer service.embeddedArtworkMu.Unlock()
	if libraryFiles, listErr := service.files.ListByLibraryID(
		ctx,
		source.LibraryID,
	); listErr == nil && usableEmbeddedArtworkAlreadyRegistered(
		source,
		libraryFiles,
	) {
		return result, nil
	}

	coverID := embeddedArtworkFileID(source.ID)
	cacheKey := embeddedArtworkCacheKey(coverID)
	outputPath := filepath.Join(
		service.embeddedArtworkDirectory,
		cacheKey+".jpg",
	)
	tempPath := filepath.Join(
		service.embeddedArtworkDirectory,
		"."+cacheKey+"."+uuid.NewString()+".tmp.jpg",
	)
	extractor := service.embeddedArtworkExtractor
	if extractor == nil {
		extractor = service.extractEmbeddedArtwork
	}
	if err := extractor(ctx, source.Storage.LocalPath, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return result, err
	}
	info, err = os.Stat(tempPath)
	if err != nil || !info.Mode().IsRegular() ||
		info.Size() <= 0 || info.Size() > embeddedArtworkMaxBytes {
		_ = os.Remove(tempPath)
		return result, errors.New("embedded artwork output is invalid")
	}
	if err := promoteTranscodeTemporaryOutput(tempPath, outputPath); err != nil {
		_ = os.Remove(tempPath)
		return result, err
	}
	if err := os.Chmod(outputPath, 0o600); err != nil {
		return result, err
	}
	info, err = os.Stat(outputPath)
	if err != nil {
		return result, err
	}

	createdAt := source.CreatedAt
	if existing, getErr := service.files.Get(ctx, coverID); getErr == nil {
		createdAt = existing.CreatedAt
	}
	updatedAt := service.now()
	size := info.Size()
	name := strings.TrimSuffix(source.Name, filepath.Ext(source.Name)) +
		" cover.jpg"
	cover, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: coverID, LibraryID: source.LibraryID,
		Kind:        string(library.FileKindThumbnail),
		Name:        name,
		DisplayName: firstNonEmpty(source.DisplayName, source.Name) + " cover",
		Metadata:    source.Metadata,
		Storage: library.FileStorage{
			Mode:      "local_path",
			LocalPath: outputPath,
		},
		Origin:  source.Origin,
		Lineage: library.FileLineage{RootFileID: source.ID},
		Media: &library.MediaInfo{
			Format: "jpg", Codec: "mjpeg", SizeBytes: &size,
		},
		State:     library.FileState{Status: "active"},
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	})
	if err != nil {
		return result, err
	}
	if err := service.files.Save(ctx, cover); err != nil {
		return result, err
	}
	result.cover = cover
	result.created = true
	return result, nil
}

func (service *LibraryService) extractEmbeddedArtwork(
	ctx context.Context,
	inputPath string,
	outputPath string,
) error {
	execPath, err := resolveFFmpegExecPath(ctx, service.tools)
	if err != nil {
		return err
	}
	extractCtx, cancel := withLocalMediaProbeTimeout(ctx)
	defer cancel()
	args := []string{"-nostdin", "-v", "error", "-y"}
	args = appendLocalMediaFFmpegInput(args, inputPath)
	args = append(
		args,
		"-map", "0:v:0",
		"-frames:v", "1",
		"-an", "-sn", "-dn",
		"-vf", "scale=1024:1024:force_original_aspect_ratio=decrease",
		"-c:v", "mjpeg",
		"-q:v", "3",
		outputPath,
	)
	command := exec.CommandContext(extractCtx, execPath, args...)
	configureLocalMediaToolCommand(command)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return fmt.Errorf("extract embedded artwork: %w", err)
	}
	return nil
}

func embeddedArtworkSourceCandidate(item library.LibraryFile) bool {
	if item.State.Deleted || item.Origin.Kind != "import" ||
		item.Origin.Import == nil ||
		strings.TrimSpace(item.Storage.LocalPath) == "" {
		return false
	}
	switch item.Kind {
	case library.FileKindAudio:
		return true
	case library.FileKindTranscode:
		return item.Media != nil &&
			strings.TrimSpace(item.Media.AudioCodec) != "" &&
			strings.TrimSpace(item.Media.VideoCodec) == ""
	default:
		return false
	}
}

func usableEmbeddedArtworkAlreadyRegistered(
	source library.LibraryFile,
	files []library.LibraryFile,
) bool {
	_, ok := usableRegisteredEmbeddedArtwork(source, files)
	return ok
}

func usableRegisteredEmbeddedArtwork(
	source library.LibraryFile,
	files []library.LibraryFile,
) (library.LibraryFile, bool) {
	path := resolveListenLocalCoverPath(
		source,
		buildListenLocalCoverLookup(files),
	)
	if strings.TrimSpace(path) == "" {
		return library.LibraryFile{}, false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return library.LibraryFile{}, false
	}
	for _, item := range files {
		if item.Kind == library.FileKindThumbnail &&
			sameCleanPath(item.Storage.LocalPath, path) {
			return item, true
		}
	}
	return library.LibraryFile{}, false
}

func (service *LibraryService) embeddedArtworkNegativeMarker(
	source library.LibraryFile,
) (string, error) {
	info, err := os.Stat(source.Storage.LocalPath)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, value := range []string{
		embeddedArtworkCacheVersion,
		source.ID,
		strconv.FormatInt(info.Size(), 10),
		strconv.FormatInt(info.ModTime().UnixNano(), 10),
	} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return filepath.Join(
		service.embeddedArtworkDirectory,
		hex.EncodeToString(hash.Sum(nil))+".none",
	), nil
}

func embeddedArtworkCacheKey(fileID string) string {
	digest := sha256.Sum256([]byte(
		embeddedArtworkCacheVersion + "\x00" + strings.TrimSpace(fileID),
	))
	return hex.EncodeToString(digest[:])
}

func embeddedArtworkFileID(sourceFileID string) string {
	return uuid.NewSHA1(
		embeddedArtworkNamespace,
		[]byte(strings.TrimSpace(sourceFileID)),
	).String()
}

func (service *LibraryService) embeddedArtworkSourceAvailable(
	ctx context.Context,
	source library.LibraryFile,
) bool {
	if service.storageRoots != nil &&
		strings.TrimSpace(source.Storage.RootID) != "" {
		root, err := service.storageRoots.Get(ctx, source.Storage.RootID)
		if err != nil ||
			root.Status == library.StorageRootStatusOffline ||
			root.Status == library.StorageRootStatusError {
			return false
		}
	}
	info, err := os.Stat(source.Storage.LocalPath)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func (service *LibraryService) embeddedArtworkReadyMarker(
	source library.LibraryFile,
) string {
	return filepath.Join(
		service.embeddedArtworkDirectory,
		embeddedArtworkCacheKey(embeddedArtworkFileID(source.ID))+".ready",
	)
}

func (service *LibraryService) embeddedArtworkNeedsReconcile(
	ctx context.Context,
	source library.LibraryFile,
	cover library.LibraryFile,
) bool {
	if !pathExists(service.embeddedArtworkReadyMarker(source)) {
		return true
	}
	if service.localTracks == nil {
		return false
	}
	track, err := service.localTracks.Get(ctx, source.ID)
	if err != nil {
		return false
	}
	return !sameCleanPath(track.CoverLocalPath, cover.Storage.LocalPath)
}

func (service *LibraryService) reconcileEmbeddedArtwork(
	ctx context.Context,
	results []embeddedArtworkResult,
	projectCatalog bool,
) error {
	if len(results) == 0 {
		return nil
	}
	changedLibraries := make(map[string]struct{})
	changedCounts := make(map[string]int)
	for _, result := range results {
		if err := service.syncEmbeddedArtworkTrack(ctx, result); err != nil {
			return err
		}
		changedLibraries[result.source.LibraryID] = struct{}{}
		changedCounts[result.source.LibraryID]++
	}
	libraryIDs := make([]string, 0, len(changedLibraries))
	for libraryID := range changedLibraries {
		libraryIDs = append(libraryIDs, libraryID)
	}
	sort.Strings(libraryIDs)
	if projectCatalog {
		if err := service.syncCatalogProjection(ctx, libraryIDs...); err != nil {
			return err
		}
	}
	now := service.now()
	for _, libraryID := range libraryIDs {
		if err := service.touchLibrary(ctx, libraryID, now); err != nil {
			return err
		}
		service.publishEvent(
			libraryTopicFile,
			"batch",
			map[string]any{
				"libraryId":    libraryID,
				"changedCount": changedCounts[libraryID],
			},
		)
	}
	for _, result := range results {
		if err := os.WriteFile(
			service.embeddedArtworkReadyMarker(result.source),
			nil,
			0o600,
		); err != nil {
			return err
		}
	}
	return nil
}

func (service *LibraryService) syncEmbeddedArtworkTrack(
	ctx context.Context,
	result embeddedArtworkResult,
) error {
	if service.localTracks == nil {
		return nil
	}
	unlock := service.lockListenLocalTrackMutation(result.source.ID)
	track, err := service.localTracks.Get(ctx, result.source.ID)
	if err == nil {
		track.CoverLocalPath = result.cover.Storage.LocalPath
		track.UpdatedAt = service.now()
		err = service.localTracks.Save(ctx, track)
		unlock()
		return err
	}
	unlock()
	if !errors.Is(err, library.ErrFileNotFound) {
		return err
	}
	var probe *mediaProbe
	if result.probe.StreamInfo {
		probe = &result.probe
	}
	service.syncListenLocalTrackFromFile(ctx, result.source, probe)
	return nil
}

func ensureEmbeddedArtworkDirectory(path string) error {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("embedded artwork directory is unavailable")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("embedded artwork directory is unavailable")
	}
	return os.Chmod(path, 0o700)
}
