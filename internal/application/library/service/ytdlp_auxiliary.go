package service

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	appytdlp "xiadown/internal/application/ytdlp"
	"xiadown/internal/domain/library"
	ydlpinfr "xiadown/internal/infrastructure/ytdlp"
)

const maxYTDLPThumbnailBytes int64 = 32 << 20

func wantsYTDLPSubtitles(request dto.CreateYTDLPJobRequest) bool {
	return request.SubtitleAll || len(request.SubtitleLangs) > 0
}

func extractYTDLPThumbnailURL(result ydlpinfr.RunResult) string {
	for _, item := range result.Metadata {
		if thumbnailURL := strings.TrimSpace(resolveYTDLPThumbnail(item)); thumbnailURL != "" {
			return thumbnailURL
		}
	}
	return ""
}

func mergeYTDLPRunResults(base ydlpinfr.RunResult, extra ydlpinfr.RunResult) ydlpinfr.RunResult {
	base.Logs = append(base.Logs, extra.Logs...)
	base.Metadata = append(base.Metadata, extra.Metadata...)
	base.OutputPaths = append(base.OutputPaths, extra.OutputPaths...)
	base.AfterMovePaths = append(base.AfterMovePaths, extra.AfterMovePaths...)
	base.SubtitleLogPaths = append(base.SubtitleLogPaths, extra.SubtitleLogPaths...)
	base.Output = joinYTDLPText(base.Output, extra.Output)
	base.Warnings = joinYTDLPText(base.Warnings, extra.Warnings)
	base.Stderr = joinYTDLPText(base.Stderr, extra.Stderr)
	return base
}

func joinYTDLPText(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + "\n" + right
}

func (service *LibraryService) downloadYTDLPSubtitles(
	ctx context.Context,
	reporter *ytdlpProgressReporter,
	execPath string,
	request dto.CreateYTDLPJobRequest,
	outputTemplate string,
	subtitleTemplate string,
	outputPath string,
	cookiesPath string,
) ytdlpAuxiliaryStep {
	step := ytdlpAuxiliaryStep{}
	if !wantsYTDLPSubtitles(request) {
		return step
	}
	subtitleOutputTemplate, err := prepareYTDLPSubtitleOutputTemplate(outputPath, subtitleTemplate)
	if err != nil {
		step.warning = fmt.Sprintf("subtitle download failed: %v", err)
		return step
	}
	if strings.TrimSpace(subtitleOutputTemplate) == "" {
		subtitleOutputTemplate = outputTemplate
	}
	if reporter != nil {
		reporter.updateStage("Downloading subtitles", 0, 0, "Downloading subtitles")
	}
	command, err := appytdlp.BuildSubtitleCommand(ctx, appytdlp.CommandOptions{
		ExecPath:         execPath,
		Tools:            service.tools,
		Request:          request,
		OutputTemplate:   subtitleOutputTemplate,
		SubtitleTemplate: subtitleOutputTemplate,
		CookiesPath:      cookiesPath,
		ProxyURL:         service.resolveYTDLPProxy(request.URL),
	})
	if err != nil {
		step.warning = fmt.Sprintf("subtitle download failed: %v", err)
		return step
	}
	defer command.Cancel()
	if command.Cleanup != nil {
		defer command.Cleanup()
	}
	result, runErr := service.executeYTDLPCommand(library.LibraryOperation{}, command, reporter, nil)
	step.result = result
	if runErr == nil {
		return step
	}
	detail := buildYTDLPFailureDetailFromLogs(result.Output, result.Stderr, result.Warnings, 400)
	if detail == "" {
		detail = runErr.Error()
	}
	step.warning = fmt.Sprintf("subtitle download failed: %s", detail)
	return step
}

func prepareYTDLPSubtitleOutputTemplate(outputPath string, fallbackTemplate string) (string, error) {
	trimmedOutputPath := strings.TrimSpace(outputPath)
	if trimmedOutputPath == "" {
		return strings.TrimSpace(fallbackTemplate), nil
	}
	subtitleDir := filepath.Join(filepath.Dir(trimmedOutputPath), "subtitles")
	if err := os.MkdirAll(subtitleDir, 0o755); err != nil {
		return "", err
	}
	baseName := strings.TrimSpace(strings.TrimSuffix(filepath.Base(trimmedOutputPath), filepath.Ext(trimmedOutputPath)))
	if baseName == "" || baseName == "." {
		baseName = "subtitle"
	}
	return filepath.Join(escapeYTDLPOutputTemplateLiteral(subtitleDir), escapeYTDLPOutputTemplateLiteral(baseName)+".%(ext)s"), nil
}

func escapeYTDLPOutputTemplateLiteral(value string) string {
	return strings.ReplaceAll(value, "%", "%%")
}

func (service *LibraryService) downloadYTDLPThumbnail(
	ctx context.Context,
	reporter *ytdlpProgressReporter,
	request dto.CreateYTDLPJobRequest,
	outputPath string,
) (string, error) {
	thumbnailURL := strings.TrimSpace(request.ThumbnailURL)
	if thumbnailURL == "" {
		return "", nil
	}
	if reporter != nil {
		reporter.updateStage("Downloading thumbnail", 0, 0, "Downloading thumbnail")
	}
	targetPath, err := resolveYTDLPThumbnailTargetPath(outputPath, ".jpg")
	if err != nil {
		return "", err
	}
	return service.downloadYTDLPThumbnailToTarget(ctx, thumbnailURL, targetPath, true)
}

func (service *LibraryService) downloadYTDLPThumbnailPrefetch(
	ctx context.Context,
	request dto.CreateYTDLPJobRequest,
	outputTemplate string,
	operationID string,
) (string, error) {
	thumbnailURL := strings.TrimSpace(request.ThumbnailURL)
	if thumbnailURL == "" {
		return "", nil
	}
	targetPath, err := prepareYTDLPThumbnailPrefetchTargetPath(outputTemplate, operationID, ".jpg")
	if err != nil {
		return "", err
	}
	return service.downloadYTDLPThumbnailToTarget(ctx, thumbnailURL, targetPath, true)
}

func prepareYTDLPThumbnailPrefetchTargetPath(outputTemplate string, operationID string, extension string) (string, error) {
	baseDir := resolveYTDLPPrefetchBaseDir(outputTemplate)
	if baseDir == "" {
		return "", fmt.Errorf("yt-dlp output base dir not resolved")
	}
	sanitizedOperationID := sanitizeFileName(strings.TrimSpace(operationID))
	if sanitizedOperationID == "" {
		return "", fmt.Errorf("operation id is required")
	}
	resolvedExtension := strings.TrimSpace(extension)
	if resolvedExtension == "" {
		resolvedExtension = ".jpg"
	}
	if !strings.HasPrefix(resolvedExtension, ".") {
		resolvedExtension = "." + resolvedExtension
	}
	targetDir := filepath.Join(baseDir, ".thumbnail-prefetch", sanitizedOperationID)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(targetDir, "thumbnail"+resolvedExtension), nil
}

func (service *LibraryService) promotePrefetchedYTDLPThumbnail(tempPath string, outputPath string) (string, error) {
	resolvedTemp := strings.TrimSpace(tempPath)
	if resolvedTemp == "" {
		return "", nil
	}
	if !pathExists(resolvedTemp) {
		return "", fmt.Errorf("prefetched thumbnail not found: %s", resolvedTemp)
	}
	targetPath, err := resolveYTDLPThumbnailTargetPath(outputPath, filepath.Ext(resolvedTemp))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := moveFileWithCopyFallback(resolvedTemp, targetPath); err != nil {
		return "", err
	}
	_ = os.Remove(filepath.Dir(resolvedTemp))
	return targetPath, nil
}

func (service *LibraryService) ytdlpAuxiliaryHTTPClient() *http.Client {
	if service != nil && service.proxyClient != nil {
		if provider, ok := service.proxyClient.(interface{ HTTPClient() *http.Client }); ok {
			if client := provider.HTTPClient(); client != nil {
				return client
			}
		}
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func resolveThumbnailExtension(contentType string, path string) (string, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(contentType))
	if normalizedType != "" {
		if parsed, _, err := mime.ParseMediaType(normalizedType); err == nil {
			normalizedType = strings.TrimSpace(parsed)
		}
		if !strings.HasPrefix(normalizedType, "image/") {
			return "", fmt.Errorf("unsupported thumbnail content type: %s", normalizedType)
		}
		switch normalizedType {
		case "image/jpeg":
			return ".jpg", nil
		case "image/png":
			return ".png", nil
		case "image/webp":
			return ".webp", nil
		case "image/gif":
			return ".gif", nil
		case "image/avif":
			return ".avif", nil
		}
		if extensions, err := mime.ExtensionsByType(normalizedType); err == nil {
			for _, extension := range extensions {
				if strings.TrimSpace(extension) != "" {
					return extension, nil
				}
			}
		}
	}
	switch strings.ToLower(strings.TrimSpace(filepath.Ext(path))) {
	case ".jpg", ".jpeg":
		return ".jpg", nil
	case ".png":
		return ".png", nil
	case ".webp":
		return ".webp", nil
	case ".gif":
		return ".gif", nil
	case ".avif":
		return ".avif", nil
	}
	return ".jpg", nil
}

func (service *LibraryService) downloadYTDLPThumbnailToTarget(
	ctx context.Context,
	thumbnailURL string,
	targetPath string,
	replaceExtension bool,
) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(thumbnailURL))
	if err != nil {
		return "", err
	}
	if scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme)); scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("unsupported thumbnail url scheme: %s", parsedURL.Scheme)
	}
	targetDir := filepath.Dir(strings.TrimSpace(targetPath))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}

	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, thumbnailURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "image/*")

	client := service.ytdlpAuxiliaryHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("thumbnail request returned status %d", resp.StatusCode)
	}
	extension, err := resolveThumbnailExtension(resp.Header.Get("Content-Type"), parsedURL.Path)
	if err != nil {
		return "", err
	}
	resolvedTargetPath := strings.TrimSpace(targetPath)
	if replaceExtension {
		resolvedTargetPath = strings.TrimSuffix(resolvedTargetPath, filepath.Ext(resolvedTargetPath)) + extension
	}
	tempFile, err := os.CreateTemp(targetDir, ".thumbnail-*"+extension)
	if err != nil {
		return "", err
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		if tempPath != "" {
			_ = os.Remove(tempPath)
		}
	}()

	written, err := io.Copy(tempFile, io.LimitReader(resp.Body, maxYTDLPThumbnailBytes+1))
	if err != nil {
		return "", err
	}
	if written <= 0 {
		return "", fmt.Errorf("thumbnail response is empty")
	}
	if written > maxYTDLPThumbnailBytes {
		return "", fmt.Errorf("thumbnail exceeds max bytes: %d > %d", written, maxYTDLPThumbnailBytes)
	}
	if err := tempFile.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(resolvedTargetPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := moveFileWithCopyFallback(tempPath, resolvedTargetPath); err != nil {
		return "", err
	}
	tempPath = ""
	return resolvedTargetPath, nil
}

func resolveYTDLPThumbnailTargetPath(outputPath string, extension string) (string, error) {
	trimmedOutputPath := strings.TrimSpace(outputPath)
	if trimmedOutputPath == "" {
		return "", fmt.Errorf("output path is required")
	}
	resolvedExtension := strings.TrimSpace(extension)
	if resolvedExtension == "" {
		resolvedExtension = ".jpg"
	}
	if !strings.HasPrefix(resolvedExtension, ".") {
		resolvedExtension = "." + resolvedExtension
	}
	targetDir := filepath.Join(filepath.Dir(trimmedOutputPath), "thumbnails")
	baseName := sanitizeFileName(strings.TrimSuffix(filepath.Base(trimmedOutputPath), filepath.Ext(trimmedOutputPath)))
	if baseName == "" {
		baseName = "thumbnail"
	}
	return filepath.Join(targetDir, baseName+"-thumbnail"+resolvedExtension), nil
}

func resolveYTDLPPrefetchBaseDir(outputTemplate string) string {
	templateDir := filepath.Dir(strings.TrimSpace(outputTemplate))
	if templateDir == "" || templateDir == "." || templateDir == "/" {
		return ""
	}
	outputBaseDir := filepath.Dir(templateDir)
	if strings.EqualFold(filepath.Base(filepath.Clean(outputBaseDir)), "yt-dlp") {
		parent := filepath.Dir(outputBaseDir)
		if parent != "" && parent != "." && parent != string(filepath.Separator) {
			return parent
		}
	}
	return outputBaseDir
}

func moveFileWithCopyFallback(sourcePath string, targetPath string) error {
	if err := os.Rename(sourcePath, targetPath); err == nil {
		return nil
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = targetFile.Close()
	}()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	if err := targetFile.Close(); err != nil {
		return err
	}
	// The destination is durable at this point; callers can retry temp cleanup later.
	_ = os.Remove(sourcePath)
	return nil
}
