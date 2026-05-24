package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/library/dto"
	appytdlp "xiadown/internal/application/ytdlp"
	"xiadown/internal/domain/library"
)

const (
	resourceFormatID                = "resource-best"
	resourceErrorCodeResolveFailed  = string(apperrors.CodeResourceResolveFailed)
	resourceErrorCodeDownloadFailed = string(apperrors.CodeResourceDownloadFailed)
	resourceErrorCodeOutputFailed   = string(apperrors.CodeResourceOutputFailed)
)

func (service *LibraryService) runDownloadOperation(ctx context.Context, operation library.LibraryOperation, history library.HistoryRecord, request dto.CreateYTDLPJobRequest) {
	if strings.TrimSpace(request.ResourceSessionID) != "" || strings.TrimSpace(request.ResourceMediaID) != "" {
		if media, ok := service.resourceMediaForQueuedOperation(request); ok && resourceMediaRequiresYTDLP(media) {
			service.discardResourceMediaSnapshots(request.ResourceMediaID, request.FormatID)
			request = service.prepareResourceYTDLPRequest(ctx, &operation, &history, request, media)
			service.runYTDLPOperationWithHeaders(ctx, operation, history, request, media.RequestHeaders)
			return
		}
		service.runResourceOperation(ctx, operation, history, request)
		return
	}
	service.runYTDLPOperation(ctx, operation, history, request)
}

func resourceMediaRequiresYTDLP(media resourceMedia) bool {
	return resourceSniffRawManifestStream(media.URL, media.MimeType, media.ContentType)
}

func (service *LibraryService) prepareResourceYTDLPRequest(ctx context.Context, operation *library.LibraryOperation, history *library.HistoryRecord, request dto.CreateYTDLPJobRequest, media resourceMedia) dto.CreateYTDLPJobRequest {
	request.URL = strings.TrimSpace(media.URL)
	request.ResourceSessionID = ""
	request.ResourceMediaID = ""
	request.FormatID = ""
	if strings.TrimSpace(request.Quality) == "" {
		request.Quality = "best"
	}
	if title := firstNonEmpty(strings.TrimSpace(request.Title), strings.TrimSpace(media.Title), strings.TrimSpace(media.PageURL), strings.TrimSpace(media.URL)); title != "" {
		request.Title = title
		if operation != nil {
			operation.DisplayName = title
		}
		if history != nil {
			history.DisplayName = title
		}
	}
	if strings.TrimSpace(media.Extractor) != "" {
		request.Extractor = media.Extractor
		if operation != nil {
			operation.Meta.Platform = media.Extractor
		}
	}
	if strings.TrimSpace(media.Author) != "" {
		request.Author = media.Author
		if operation != nil {
			operation.Meta.Uploader = media.Author
		}
	}
	if strings.TrimSpace(request.ThumbnailURL) == "" && strings.TrimSpace(media.ThumbnailURL) != "" {
		request.ThumbnailURL = media.ThumbnailURL
	}
	if operation != nil {
		operation.SourceDomain = firstNonEmpty(strings.TrimSpace(media.Domain), extractRegistrableDomain(media.PageURL), extractRegistrableDomain(media.URL), operation.SourceDomain)
		if service != nil && service.iconResolver != nil && strings.TrimSpace(operation.SourceIcon) == "" && strings.TrimSpace(operation.SourceDomain) != "" {
			if icon, err := service.iconResolver.ResolveDomainIcon(ctx, operation.SourceDomain); err == nil {
				operation.SourceIcon = icon
			}
		}
		if inputJSON, err := json.Marshal(request); err == nil {
			operation.InputJSON = string(inputJSON)
		}
	}
	return request
}

func resourceFormatOption(media resourceMedia) dto.YTDLPFormatOption {
	return resourceFormatOptionWithID(resourceFormatID, media)
}

func resourceFormatOptionWithID(id string, media resourceMedia) dto.YTDLPFormatOption {
	ext := strings.TrimPrefix(strings.TrimSpace(media.Ext), ".")
	if ext == "" {
		ext = strings.TrimPrefix(resourceMediaDefaultExt(media), ".")
	}
	height := resourceFormatOptionHeight(media)
	id = strings.TrimSpace(id)
	if id == "" {
		id = resourceFormatID
	}
	hasVideo, hasAudio := resourceMediaTrackFlags(media)
	return dto.YTDLPFormatOption{
		ID:       id,
		Label:    resourceFormatLabelForMedia(media, height, ext),
		HasVideo: hasVideo,
		HasAudio: hasAudio,
		Ext:      ext,
		Height:   height,
		VCodec:   media.VCodec,
		ACodec:   media.ACodec,
		Filesize: media.SizeBytes,
	}
}

func resourceFormatLabelForMedia(media resourceMedia, height int, ext string) string {
	if media.QualityHeight <= 0 && media.Width > 0 && media.Height > 0 && media.Width < media.Height {
		return resourceFormatResolutionLabel(media.Width, media.Height, ext, media.VCodec, media.SizeBytes)
	}
	return resourceFormatLabel(height, ext, media.VCodec, media.SizeBytes)
}

func resourceMediaTrackFlags(media resourceMedia) (bool, bool) {
	switch resourceMediaDeclaredKind(media) {
	case "subtitle", "image":
		return false, false
	case "audio":
		return false, true
	case "video":
		return true, true
	case "document", "font", "api", "archive", "manifest", "other":
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(media.Kind)) {
	case "image", "subtitle", "document", "font", "api", "archive", "live", "manifest", "segment", "other":
		return false, false
	case "audio":
		return false, true
	case "video":
		return true, true
	case "":
	default:
		return false, false
	}
	if resourceMediaLooksAudio(media) {
		return false, true
	}
	if resourceMediaLooksImage(media) || resourceMediaLooksSubtitle(media) {
		return false, false
	}
	if strings.TrimSpace(media.Kind) == "" &&
		resourceMediaMimeToken(media) == "" &&
		resourceMediaExtToken(media) == "" &&
		strings.TrimSpace(media.VCodec) == "" &&
		strings.TrimSpace(media.ACodec) == "" {
		return false, false
	}
	return true, true
}

func resourceMediaLibraryFileKind(media resourceMedia) string {
	switch resourceMediaDeclaredKind(media) {
	case "subtitle":
		return string(library.FileKindSubtitle)
	case "image":
		return string(library.FileKindThumbnail)
	case "audio":
		return string(library.FileKindAudio)
	case "video":
		return string(library.FileKindVideo)
	case "document":
		return string(library.FileKindDocument)
	case "font":
		return string(library.FileKindFont)
	case "api":
		return string(library.FileKindAPI)
	case "archive":
		return string(library.FileKindArchive)
	case "manifest":
		return string(library.FileKindManifest)
	}
	switch strings.ToLower(strings.TrimSpace(media.Kind)) {
	case "subtitle":
		return string(library.FileKindSubtitle)
	case "image":
		return string(library.FileKindThumbnail)
	case "audio":
		return string(library.FileKindAudio)
	case "video":
		return string(library.FileKindVideo)
	case "document":
		return string(library.FileKindDocument)
	case "font":
		return string(library.FileKindFont)
	case "api":
		return string(library.FileKindAPI)
	case "archive":
		return string(library.FileKindArchive)
	case "manifest", "live":
		return string(library.FileKindManifest)
	case "other", "segment":
		return string(library.FileKindOther)
	case "":
	default:
		return string(library.FileKindOther)
	}
	if resourceMediaLooksSubtitle(media) {
		return string(library.FileKindSubtitle)
	}
	if resourceMediaLooksImage(media) {
		return string(library.FileKindThumbnail)
	}
	hasVideo, hasAudio := resourceMediaTrackFlags(media)
	if !hasVideo && hasAudio {
		return string(library.FileKindAudio)
	}
	if !hasVideo && !hasAudio {
		return string(library.FileKindOther)
	}
	if strings.TrimSpace(media.Kind) == "" && strings.TrimSpace(media.MimeType) == "" && strings.TrimSpace(media.ContentType) == "" && strings.TrimSpace(media.Ext) == "" {
		return string(library.FileKindOther)
	}
	return string(library.FileKindVideo)
}

func resourceMediaLooksImage(media resourceMedia) bool {
	if declaredKind := resourceMediaDeclaredKind(media); declaredKind != "" {
		return declaredKind == "image"
	}
	if strings.EqualFold(strings.TrimSpace(media.Kind), "image") {
		return true
	}
	mime := resourceMediaMimeToken(media)
	ext := resourceMediaExtToken(media)
	if strings.HasPrefix(mime, "image/") {
		return true
	}
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif", "avif", "bmp", "ico", "svg":
		return true
	default:
		return false
	}
}

func resourceMediaLooksSubtitle(media resourceMedia) bool {
	if declaredKind := resourceMediaDeclaredKind(media); declaredKind != "" {
		return declaredKind == "subtitle"
	}
	if strings.EqualFold(strings.TrimSpace(media.Kind), "subtitle") {
		return true
	}
	mime := resourceMediaMimeToken(media)
	ext := resourceMediaExtToken(media)
	if strings.HasPrefix(mime, "text/vtt") ||
		strings.Contains(mime, "subrip") ||
		strings.Contains(mime, "ttml") {
		return true
	}
	switch ext {
	case "vtt", "webvtt", "srt", "ass", "ssa", "ttml", "dfxp", "itt", "lrc", "sbv", "fcpxml":
		return true
	default:
		return false
	}
}

func resourceMediaLooksAudio(media resourceMedia) bool {
	if declaredKind := resourceMediaDeclaredKind(media); declaredKind != "" {
		return declaredKind == "audio"
	}
	mime := resourceMediaMimeToken(media)
	ext := resourceMediaExtToken(media)
	if strings.HasPrefix(mime, "audio/") {
		return true
	}
	switch ext {
	case "mp3", "m4a", "aac", "wav", "flac", "ogg", "opus":
		return true
	default:
		return false
	}
}

func resourceMediaDeclaredKind(media resourceMedia) string {
	mime := resourceMediaMimeToken(media)
	ext := resourceMediaExtToken(media)
	switch {
	case strings.HasPrefix(mime, "text/vtt") ||
		strings.Contains(mime, "subrip") ||
		strings.Contains(mime, "ttml"):
		return "subtitle"
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case resourceSniffRawManifestStream(media.URL, media.MimeType, media.ContentType):
		return "manifest"
	case strings.Contains(mime, "json"):
		return "api"
	case strings.Contains(mime, "pdf") ||
		strings.Contains(mime, "msword") ||
		strings.Contains(mime, "officedocument") ||
		strings.Contains(mime, "spreadsheet") ||
		strings.Contains(mime, "presentation") ||
		strings.Contains(mime, "wordprocessing"):
		return "document"
	case strings.HasPrefix(mime, "font/") || strings.Contains(mime, "font"):
		return "font"
	case strings.Contains(mime, "zip") ||
		strings.Contains(mime, "rar") ||
		strings.Contains(mime, "7z"):
		return "archive"
	}
	switch ext {
	case "vtt", "webvtt", "srt", "ass", "ssa", "ttml", "dfxp", "itt", "lrc", "sbv", "fcpxml":
		return "subtitle"
	case "jpg", "jpeg", "png", "webp", "gif", "avif", "bmp", "ico", "svg":
		return "image"
	case "mp3", "m4a", "aac", "wav", "flac", "ogg", "opus":
		return "audio"
	case "mp4", "m4v", "mov", "webm", "flv":
		return "video"
	case "m3u8", "mpd", "f4m", "ism":
		return "manifest"
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx":
		return "document"
	case "woff", "woff2", "ttf", "otf", "eot":
		return "font"
	case "json":
		return "api"
	case "zip", "rar", "7z", "dmg", "pkg", "exe":
		return "archive"
	default:
		return ""
	}
}

func resourceMediaMimeToken(media resourceMedia) string {
	return strings.ToLower(strings.TrimSpace(firstNonEmpty(media.MimeType, media.ContentType)))
}

func resourceMediaExtToken(media resourceMedia) string {
	ext := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(media.Ext), "."))
	if ext == "" {
		ext = resourceSniffURLPathExt(media.URL)
	}
	return ext
}

func resourceMediaDefaultExt(media resourceMedia) string {
	switch resourceMediaLibraryFileKind(media) {
	case string(library.FileKindSubtitle):
		return ".vtt"
	case string(library.FileKindThumbnail):
		return ".jpg"
	case string(library.FileKindAudio):
		return ".mp3"
	case string(library.FileKindDocument):
		return ".pdf"
	case string(library.FileKindFont):
		return ".woff2"
	case string(library.FileKindAPI):
		return ".json"
	case string(library.FileKindArchive):
		return ".zip"
	case string(library.FileKindManifest):
		return ".m3u8"
	case string(library.FileKindOther):
		return ".bin"
	default:
		return ".mp4"
	}
}

func normalizeResourceMediaWithDownloadedFile(media resourceMedia, outputPath string) resourceMedia {
	detectedContentType := detectDownloadedResourceContentType(outputPath)
	if detectedContentType == "" {
		return normalizeUnknownResourceMediaKind(media)
	}
	detectedKind := resourceMediaDeclaredKind(resourceMedia{ContentType: detectedContentType, MimeType: detectedContentType})
	switch detectedKind {
	case "video", "audio", "image", "subtitle":
	default:
		if strings.TrimSpace(media.ContentType) == "" {
			media.ContentType = detectedContentType
		}
		if strings.TrimSpace(media.MimeType) == "" {
			media.MimeType = detectedContentType
		}
		return normalizeUnknownResourceMediaKind(media)
	}
	currentKind := resourceMediaDeclaredKind(media)
	if currentKind != "" && currentKind == detectedKind {
		media.Kind = detectedKind
		if strings.TrimSpace(media.ContentType) == "" {
			media.ContentType = detectedContentType
		}
		if strings.TrimSpace(media.MimeType) == "" {
			media.MimeType = detectedContentType
		}
		return media
	}
	media.Kind = detectedKind
	media.ContentType = detectedContentType
	media.MimeType = detectedContentType
	if ext := resourceExtension("", detectedContentType); ext != "" {
		media.Ext = ext
	}
	return media
}

func normalizeUnknownResourceMediaKind(media resourceMedia) resourceMedia {
	if resourceMediaDeclaredKind(media) != "" {
		return media
	}
	switch strings.ToLower(strings.TrimSpace(media.Kind)) {
	case "video", "audio", "image", "subtitle", "document", "font", "api", "archive", "manifest", "other":
		return media
	case "live":
		media.Kind = "manifest"
	default:
		media.Kind = "other"
	}
	return media
}

func detectDownloadedResourceContentType(outputPath string) string {
	file, err := os.Open(strings.TrimSpace(outputPath))
	if err != nil {
		return ""
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil || n <= 0 {
		return ""
	}
	return http.DetectContentType(buffer[:n])
}

func resourceFormatOptionHeight(media resourceMedia) int {
	if media.QualityHeight > 0 {
		return media.QualityHeight
	}
	if media.Width > 0 && media.Height > 0 && media.Width < media.Height {
		return 0
	}
	return media.Height
}

func resourceFormatResolutionLabel(width int, height int, ext string, vcodec string, sizeBytes int64) string {
	parts := make([]string, 0, 4)
	parts = append(parts, fmt.Sprintf("%dx%d", width, height))
	if trimmed := strings.TrimSpace(ext); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if codecLabel := resourceVideoCodecLabel(vcodec); codecLabel != "" {
		parts = append(parts, codecLabel)
	}
	if size := formatBytesLabel(sizeBytes); size != "" {
		parts = append(parts, size)
	}
	return strings.Join(parts, " · ")
}

func resourceFormatLabel(height int, ext string, vcodec string, sizeBytes int64) string {
	parts := make([]string, 0, 4)
	if height > 0 {
		parts = append(parts, fmt.Sprintf("%dp", height))
	} else {
		parts = append(parts, "Unknown")
	}
	if trimmed := strings.TrimSpace(ext); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if codecLabel := resourceVideoCodecLabel(vcodec); codecLabel != "" {
		parts = append(parts, codecLabel)
	}
	if size := formatBytesLabel(sizeBytes); size != "" {
		parts = append(parts, size)
	}
	return strings.Join(parts, " · ")
}

func resourceVideoCodecLabel(vcodec string) string {
	vc := strings.ToLower(strings.TrimSpace(vcodec))
	if vc == "" || vc == "none" {
		return ""
	}
	return normalizeCodecLabel(vc)
}

func (service *LibraryService) runResourceOperation(ctx context.Context, operation library.LibraryOperation, history library.HistoryRecord, request dto.CreateYTDLPJobRequest) {
	ctx, cancel := context.WithCancel(ctx)
	if !service.registerOperationRun(operation.ID, cancel) {
		cancel()
		return
	}
	defer func() {
		service.unregisterOperationRun(operation.ID)
		cancel()
	}()
	if !service.operationCanAcceptProgress(ctx, operation.ID) {
		return
	}

	started := service.now()
	operation.Status = library.OperationStatusRunning
	operation.StartedAt = &started
	operation.Progress = &library.OperationProgress{
		Stage:     progressText("library.progress.preparing"),
		UpdatedAt: started.Format(time.RFC3339),
		Message:   progressText("library.progressDetail.preparingDownload"),
	}
	history.Status = string(operation.Status)
	history.DisplayName = operation.DisplayName
	history.UpdatedAt = started
	if err := service.persistOperationAndHistory(ctx, &operation, &history); err != nil {
		return
	}

	var media resourceMedia
	if sessionID := strings.TrimSpace(request.ResourceSessionID); sessionID != "" {
		var ok bool
		media, ok = service.consumeResourceSniffMedia(sessionID, request.FormatID)
		if !ok {
			service.failYTDLPOperation(ctx, &operation, &history, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff result is unavailable"), resourceErrorCodeResolveFailed, "")
			return
		}
	} else if mediaID := strings.TrimSpace(request.ResourceMediaID); mediaID != "" {
		var ok bool
		if formatID := strings.TrimSpace(request.FormatID); formatID != "" && formatID != mediaID {
			media, ok = service.consumeResourceMediaSnapshot(formatID)
			if ok {
				service.discardResourceMediaSnapshots(mediaID)
			}
		}
		if !ok {
			media, ok = service.consumeResourceMediaSnapshot(mediaID)
		}
		if !ok {
			service.failYTDLPOperation(ctx, &operation, &history, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff media snapshot is unavailable"), resourceErrorCodeResolveFailed, "")
			return
		}
	} else {
		service.failYTDLPOperation(ctx, &operation, &history, apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff session is required"), resourceErrorCodeResolveFailed, "")
		return
	}
	request = service.applyResourceMediaToOperation(ctx, &operation, &history, request, media)
	if err := service.persistOperationAndHistory(ctx, &operation, &history); err != nil {
		return
	}

	outputPath, err := service.prepareResourceOutputPath(ctx, media, operation.ID)
	if err != nil {
		service.failYTDLPOperation(ctx, &operation, &history, err, resourceErrorCodeOutputFailed, resourceOperationOutputPayload("", nil, nil, resourceMetadataPayload(media, nil)))
		return
	}

	reporter := newYTDLPProgressReporter(service, &operation)
	reporter.stageCode = "downloading"
	thumbnailPrefetch := &ytdlpThumbnailPrefetch{}
	defer thumbnailPrefetch.Cleanup()
	if request.WriteThumbnail && strings.TrimSpace(request.ThumbnailURL) != "" {
		thumbnailPrefetch.StartForOutputPath(
			context.Background(),
			service,
			request,
			outputPath,
			operation.ID,
			reporter.publishThumbnailPreviewPath,
		)
	}
	result, err := service.downloadResourceFile(ctx, resourceDownloadOptions{
		URL:        media.URL,
		OutputPath: outputPath,
		Headers:    media.RequestHeaders,
		ProxyURL:   firstNonEmpty(service.resolveYTDLPProxy(media.URL), service.resolveYTDLPProxy(request.URL)),
		TotalSize:  media.SizeBytes,
		Progress: func(downloaded int64, total int64, speed string) {
			current := downloaded
			var totalPtr *int64
			var percentPtr *float64
			if total > 0 {
				totalValue := total
				totalPtr = &totalValue
				percent := (float64(downloaded) / float64(total)) * 100
				percentPtr = &percent
			}
			reporter.persistProgress(&current, totalPtr, percentPtr, buildProgressMessage("", speed), speed)
		},
	})
	if err != nil {
		service.failYTDLPOperation(ctx, &operation, &history, err, resourceErrorCodeDownloadFailed, resourceOperationOutputPayload("", nil, nil, resourceMetadataPayload(media, nil)))
		return
	}

	outputPath = result.Path
	if result.SizeBytes > 0 {
		media.SizeBytes = result.SizeBytes
	}
	media = normalizeResourceMediaWithDownloadedFile(media, outputPath)
	title := strings.TrimSpace(operation.DisplayName)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	}
	primaryFile, err := service.createResourcePrimaryFile(ctx, operation, request, media, title, outputPath, started)
	if err != nil {
		service.failYTDLPOperation(ctx, &operation, &history, err, resourceErrorCodeOutputFailed, resourceOperationOutputPayload(outputPath, []string{outputPath}, nil, resourceMetadataPayload(media, nil)))
		return
	}

	warnings := make([]string, 0, 2)
	thumbnailPaths, thumbnailWarnings := service.collectDownloadedThumbnailPaths(
		ctx,
		reporter,
		operation,
		request,
		outputPath,
		thumbnailPrefetch,
	)
	warnings = append(warnings, thumbnailWarnings...)
	subtitlePaths := make([]string, 0)
	if wantsYTDLPSubtitles(request) {
		paths, subtitleWarnings := service.downloadResourceSubtitles(ctx, request, media, outputPath)
		subtitlePaths = append(subtitlePaths, paths...)
		warnings = append(warnings, subtitleWarnings...)
	}

	metadataPayload := resourceMetadataPayload(media, warnings)
	outputSnapshot, err := service.buildYTDLPOutputs(ctx, request, operation, primaryFile, started, outputPath, "", []string{outputPath}, subtitlePaths, thumbnailPaths)
	if err != nil {
		service.failYTDLPOperation(ctx, &operation, &history, err, resourceErrorCodeOutputFailed, resourceOperationOutputPayload(outputPath, []string{outputPath}, nil, metadataPayload))
		return
	}

	finalMainPath := outputPath
	if strings.TrimSpace(request.TranscodePresetID) != "" {
		updatedSnapshot, transcodeMainPath, transcodeErr := service.runDownloadEmbeddedTranscode(ctx, &operation, request, primaryFile, title, outputSnapshot)
		if transcodeErr != nil {
			operation.OutputFiles = updatedSnapshot.outputFiles
			operation.Metrics = buildOperationMetricsForOperation(updatedSnapshot.files, operation.StartedAt, operation.FinishedAt)
			history.Files = operation.OutputFiles
			history.Metrics = operation.Metrics
			service.failYTDLPOperation(ctx, &operation, &history, transcodeErr, resourceErrorCodeOutputFailed, resourceOperationOutputPayload(finalMainPath, updatedSnapshot.outputPaths, updatedSnapshot.outputFiles, metadataPayload))
			for _, fileItem := range updatedSnapshot.files {
				service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
			}
			return
		}
		outputSnapshot = updatedSnapshot
		if strings.TrimSpace(transcodeMainPath) != "" {
			finalMainPath = transcodeMainPath
		}
	}
	reporter.Finalize()

	finished := service.now()
	operation.Status = library.OperationStatusSucceeded
	operation.DisplayName = title
	operation.FinishedAt = &finished
	operation.OutputFiles = outputSnapshot.outputFiles
	operation.Metrics = buildOperationMetricsForOperation(outputSnapshot.files, operation.StartedAt, operation.FinishedAt)
	operation.OutputJSON = resourceOperationOutputPayload(finalMainPath, outputSnapshot.outputPaths, outputSnapshot.outputFiles, metadataPayload)
	history.Status = string(operation.Status)
	history.DisplayName = operation.DisplayName
	history.Files = operation.OutputFiles
	history.Metrics = operation.Metrics
	history.OperationMeta = &library.OperationRecordMeta{Kind: operation.Kind}
	history.OccurredAt = finished
	history.UpdatedAt = finished
	if err := service.persistOperationAndHistory(ctx, &operation, &history); err != nil {
		return
	}
	for _, fileItem := range outputSnapshot.files {
		service.syncListenLocalTrackFromFile(ctx, fileItem, nil)
		service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
	}
}

func (service *LibraryService) createResourcePrimaryFile(ctx context.Context, operation library.LibraryOperation, request dto.CreateYTDLPJobRequest, media resourceMedia, title string, outputPath string, createdAt time.Time) (library.LibraryFile, error) {
	switch resourceMediaLibraryFileKind(media) {
	case string(library.FileKindSubtitle):
		return service.createDownloadedSubtitleFile(ctx, operation, outputPath, title, createdAt)
	case string(library.FileKindThumbnail),
		string(library.FileKindOther),
		string(library.FileKindDocument),
		string(library.FileKindFont),
		string(library.FileKindAPI),
		string(library.FileKindArchive),
		string(library.FileKindManifest):
		return service.createDownloadedBinaryFile(ctx, operation, resourceMediaLibraryFileKind(media), outputPath, title, createdAt)
	case string(library.FileKindAudio):
		fileRequest := request
		fileRequest.Quality = "audio"
		return service.createDownloadedPrimaryFile(ctx, operation, fileRequest, title, outputPath, createdAt)
	default:
		fileRequest := request
		fileRequest.Quality = "video"
		return service.createDownloadedPrimaryFile(ctx, operation, fileRequest, title, outputPath, createdAt)
	}
}

func (service *LibraryService) applyResourceMediaToOperation(ctx context.Context, operation *library.LibraryOperation, history *library.HistoryRecord, request dto.CreateYTDLPJobRequest, media resourceMedia) dto.CreateYTDLPJobRequest {
	if operation == nil {
		return request
	}
	if pageURL := strings.TrimSpace(media.PageURL); pageURL != "" {
		request.URL = pageURL
	}
	title := firstNonEmpty(strings.TrimSpace(request.Title), strings.TrimSpace(media.Title), strings.TrimSpace(request.URL))
	if title != "" {
		request.Title = title
		operation.DisplayName = title
		if history != nil {
			history.DisplayName = title
		}
	}
	if strings.TrimSpace(media.Extractor) != "" {
		request.Extractor = media.Extractor
		operation.Meta.Platform = media.Extractor
	}
	if strings.TrimSpace(media.Author) != "" {
		request.Author = media.Author
		operation.Meta.Uploader = media.Author
	}
	if strings.TrimSpace(request.ThumbnailURL) == "" && strings.TrimSpace(media.ThumbnailURL) != "" {
		request.ThumbnailURL = media.ThumbnailURL
	}
	if strings.TrimSpace(media.Domain) != "" {
		operation.SourceDomain = media.Domain
	}
	if service != nil && service.iconResolver != nil && strings.TrimSpace(operation.SourceIcon) == "" && strings.TrimSpace(operation.SourceDomain) != "" {
		if icon, err := service.iconResolver.ResolveDomainIcon(ctx, operation.SourceDomain); err == nil {
			operation.SourceIcon = icon
		}
	}
	if inputJSON, err := json.Marshal(request); err == nil {
		operation.InputJSON = string(inputJSON)
	}
	return request
}

func (service *LibraryService) prepareResourceOutputPath(ctx context.Context, media resourceMedia, operationID string) (string, error) {
	downloadDirectory, err := service.resolveDownloadDirectory(ctx)
	if err != nil {
		return "", err
	}
	if downloadDirectory == "" {
		fallback, err := libraryBaseDir()
		if err != nil {
			return "", err
		}
		downloadDirectory = fallback
	}
	baseDir := filepath.Join(downloadDirectory, "resource")
	if defaultBaseDir, defaultErr := libraryBaseDir(); defaultErr == nil {
		if !sameCleanPath(downloadDirectory, defaultBaseDir) && filepath.Base(filepath.Clean(downloadDirectory)) != "xiadown" {
			baseDir = filepath.Join(downloadDirectory, "xiadown", "resource")
		}
	}
	domainDir := sanitizeFileName(firstNonEmpty(media.Domain, extractRegistrableDomain(media.PageURL), "douyin.com"))
	if domainDir == "" {
		domainDir = "resource"
	}
	outputDir := filepath.Join(baseDir, domainDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	baseName := resourceOutputBaseName(media, operationID)
	ext := strings.TrimSpace(media.Ext)
	if ext == "" {
		ext = resourceMediaDefaultExt(media)
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return filepath.Join(outputDir, baseName+ext), nil
}

func resourceOutputBaseName(media resourceMedia, operationID string) string {
	parts := make([]string, 0, 2)
	if author := sanitizeFileName(media.Author); author != "" {
		parts = append(parts, author)
	}
	if title := sanitizeFileName(media.Title); title != "" {
		parts = append(parts, title)
	}
	base := sanitizeFileName(strings.Join(parts, "-"))
	if base == "" {
		base = "douyin"
	}
	if runes := []rune(base); len(runes) > 120 {
		base = strings.TrimSpace(string(runes[:120]))
	}
	shortID := strings.TrimSpace(operationID)
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if shortID != "" {
		base = base + "-" + shortID
	}
	return base
}

func resourceMetadataPayload(media resourceMedia, warnings []string) map[string]any {
	info := map[string]any{
		"title":       strings.TrimSpace(media.Title),
		"webpage_url": strings.TrimSpace(media.PageURL),
		"kind":        strings.TrimSpace(media.Kind),
		"extractor":   strings.TrimSpace(media.Extractor),
		"uploader":    strings.TrimSpace(media.Author),
		"thumbnail":   strings.TrimSpace(media.ThumbnailURL),
		"ext":         strings.TrimPrefix(strings.TrimSpace(media.Ext), "."),
		"filesize":    media.SizeBytes,
		"width":       int64(media.Width),
		"height":      int64(media.Height),
	}
	for key, value := range info {
		switch typed := value.(type) {
		case string:
			if typed == "" {
				delete(info, key)
			}
		case int64:
			if typed <= 0 {
				delete(info, key)
			}
		}
	}
	payload := map[string]any{
		"engine": "resource",
		"info":   info,
	}
	if len(warnings) > 0 {
		payload["auxiliaryWarnings"] = warnings
	}
	return payload
}

func resourceOperationOutputPayload(mainPath string, outputPaths []string, outputFiles []library.OperationOutputFile, metadataPayload map[string]any) string {
	return buildOperationOutputPayload(mainPath, nil, outputPaths, outputFiles, metadataPayload, appytdlp.LogSnapshot{})
}

func resourceResolveErrorCode(err error) string {
	if err == nil {
		return resourceErrorCodeResolveFailed
	}
	if code := apperrors.CodeOf(err); code != "" {
		return string(code)
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "profile") ||
		strings.Contains(lower, "cookies") ||
		strings.Contains(lower, "验证码") ||
		strings.Contains(lower, "登录") ||
		strings.Contains(lower, "captcha") ||
		strings.Contains(lower, "verify") {
		return string(apperrors.CodeResourceVerificationRequired)
	}
	return resourceErrorCodeResolveFailed
}

func resourceRetryUnavailableError() error {
	return apperrors.New(apperrors.CodeResourceResolveFailed, "resource sniff downloads cannot be retried yet; start a new sniff session")
}
