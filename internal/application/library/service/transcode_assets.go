package service

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

var transcodeCoverExtensions = map[string]struct{}{
	"jpg":  {},
	"jpeg": {},
	"png":  {},
	"webp": {},
}

var transcodeSubtitleExtensions = map[string]struct{}{
	"srt": {},
	"vtt": {},
	"ass": {},
	"ssa": {},
}

func (service *LibraryService) enrichTranscodeRequestForSource(ctx context.Context, request dto.CreateTranscodeJobRequest, sourceFile library.LibraryFile) dto.CreateTranscodeJobRequest {
	if strings.TrimSpace(request.Title) == "" {
		request.Title = resolveLibraryFileTitle(sourceFile, sourceFile.DisplayName)
	}
	if strings.TrimSpace(request.Author) == "" {
		request.Author = strings.TrimSpace(sourceFile.Metadata.Author)
	}
	if strings.TrimSpace(request.Extractor) == "" {
		request.Extractor = strings.TrimSpace(sourceFile.Metadata.Extractor)
	}
	request.CoverPath = service.resolveAutomaticTranscodeCoverPath(ctx, request, sourceFile)
	request.SubtitlePaths = service.resolveAutomaticTranscodeSubtitlePaths(ctx, request, sourceFile)
	return request
}

func resolveSnapshotTranscodeCoverPath(snapshot ytdlpOutputSnapshot) string {
	for _, candidate := range snapshot.resolvedThumbnails {
		if resolved := normalizeExistingTranscodeCoverPath(candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func normalizeExistingTranscodeCoverPath(path string) string {
	return normalizeExistingTranscodeAssetPath(path, transcodeCoverExtensions)
}

func normalizeExistingTranscodeSubtitlePath(path string) string {
	return normalizeExistingTranscodeAssetPath(path, transcodeSubtitleExtensions)
}

func normalizeExistingTranscodeAssetPath(path string, allowedExtensions map[string]struct{}) string {
	resolved := resolveAssetPathFallback(path)
	if resolved == "" {
		return ""
	}
	extension := normalizeFileExtension(resolved)
	if len(allowedExtensions) > 0 {
		if _, ok := allowedExtensions[extension]; !ok {
			return ""
		}
	}
	info, err := os.Stat(resolved)
	if err != nil || info.IsDir() {
		return ""
	}
	return resolved
}

func (service *LibraryService) resolveAutomaticTranscodeCoverPath(ctx context.Context, request dto.CreateTranscodeJobRequest, sourceFile library.LibraryFile) string {
	if explicit := normalizeExistingTranscodeCoverPath(request.CoverPath); explicit != "" {
		return explicit
	}
	for _, candidate := range service.findRelatedTranscodeFiles(ctx, sourceFile, library.FileKindThumbnail) {
		if resolved := normalizeExistingTranscodeCoverPath(candidate.Storage.LocalPath); resolved != "" {
			return resolved
		}
	}
	for _, candidate := range transcodeCoverSidecarCandidates(sourceFile.Storage.LocalPath) {
		if resolved := normalizeExistingTranscodeCoverPath(candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func (service *LibraryService) resolveAutomaticTranscodeSubtitlePaths(ctx context.Context, request dto.CreateTranscodeJobRequest, sourceFile library.LibraryFile) []string {
	candidates := make([]string, 0, len(request.SubtitlePaths)+4)
	candidates = append(candidates, request.SubtitlePaths...)
	for _, file := range service.findRelatedTranscodeFiles(ctx, sourceFile, library.FileKindSubtitle) {
		candidates = append(candidates, file.Storage.LocalPath)
	}
	candidates = append(candidates, transcodeSubtitleSidecarCandidates(sourceFile.Storage.LocalPath)...)

	result := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		resolved := normalizeExistingTranscodeSubtitlePath(candidate)
		if resolved == "" {
			continue
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		result = append(result, resolved)
	}
	return result
}

func (service *LibraryService) findRelatedTranscodeFiles(ctx context.Context, sourceFile library.LibraryFile, kind library.FileKind) []library.LibraryFile {
	if service == nil || service.files == nil || strings.TrimSpace(sourceFile.LibraryID) == "" {
		return nil
	}
	files, err := service.files.ListByLibraryID(ctx, sourceFile.LibraryID)
	if err != nil {
		return nil
	}
	type scoredFile struct {
		file  library.LibraryFile
		score int
	}
	scored := make([]scoredFile, 0)
	for _, candidate := range files {
		score := relatedTranscodeFileScore(sourceFile, candidate, kind)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredFile{file: candidate, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].file.CreatedAt.After(scored[j].file.CreatedAt)
	})
	result := make([]library.LibraryFile, 0, len(scored))
	for _, item := range scored {
		result = append(result, item.file)
	}
	return result
}

func relatedTranscodeFileScore(sourceFile library.LibraryFile, candidate library.LibraryFile, kind library.FileKind) int {
	if candidate.Kind != kind || candidate.State.Deleted || strings.TrimSpace(candidate.Storage.LocalPath) == "" {
		return 0
	}
	if sourceFile.ID != "" && candidate.ID == sourceFile.ID {
		return 0
	}

	score := 0
	sourceRootID := strings.TrimSpace(rootFileID(sourceFile))
	candidateRootID := strings.TrimSpace(rootFileID(candidate))
	if sourceRootID != "" && candidateRootID == sourceRootID {
		score += 100
	}
	sourceOperationID := strings.TrimSpace(sourceFile.Origin.OperationID)
	candidateOperationID := strings.TrimSpace(candidate.Origin.OperationID)
	if sourceOperationID != "" && candidateOperationID == sourceOperationID {
		score += 80
	}
	if sourceOperationID != "" && strings.TrimSpace(candidate.LatestOperationID) == sourceOperationID {
		score += 40
	}
	sourceTitle := strings.ToLower(strings.TrimSpace(sourceFile.Metadata.Title))
	candidateTitle := strings.ToLower(strings.TrimSpace(candidate.Metadata.Title))
	if sourceTitle != "" && candidateTitle != "" && sourceTitle == candidateTitle {
		score += 10
	}
	return score
}

func transcodeCoverSidecarCandidates(sourcePath string) []string {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil
	}
	dir := filepath.Dir(sourcePath)
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if dir == "" || dir == "." || base == "" {
		return nil
	}
	names := []string{
		filepath.Join(dir, "thumbnails", base+"-thumbnail"),
		filepath.Join(dir, base+"-thumbnail"),
		filepath.Join(dir, base),
		filepath.Join(dir, "cover"),
		filepath.Join(dir, "folder"),
		filepath.Join(dir, "front"),
		filepath.Join(dir, "album"),
	}
	return expandTranscodeAssetCandidates(names, transcodeCoverExtensions)
}

func transcodeSubtitleSidecarCandidates(sourcePath string) []string {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil
	}
	dir := filepath.Dir(sourcePath)
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if dir == "" || dir == "." || base == "" {
		return nil
	}
	names := []string{
		filepath.Join(dir, "subtitles", base),
		filepath.Join(dir, base),
	}
	return expandTranscodeAssetCandidates(names, transcodeSubtitleExtensions)
}

func expandTranscodeAssetCandidates(baseNames []string, extensions map[string]struct{}) []string {
	orderedExtensions := make([]string, 0, len(extensions))
	for extension := range extensions {
		orderedExtensions = append(orderedExtensions, extension)
	}
	sort.Strings(orderedExtensions)

	result := make([]string, 0, len(baseNames)*len(orderedExtensions))
	for _, baseName := range baseNames {
		for _, extension := range orderedExtensions {
			result = append(result, baseName+"."+extension)
		}
	}
	return result
}
