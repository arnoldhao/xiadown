package service

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"xiadown/internal/domain/library"
)

const (
	operationOutputPathsKey    = "outputPaths"
	operationTemporaryPathsKey = "temporaryPaths"
)

var operationOutputArtifactFileStringKeys = []string{
	"mainPath",
	"outputPath",
}

var operationOutputArtifactStringKeys = []string{
	"mainPath",
	"outputPath",
	"thumbnailPreviewPath",
}

var operationOutputArtifactListKeys = []string{
	"afterMovePaths",
	operationOutputPathsKey,
	operationTemporaryPathsKey,
}

var operationOutputFileArtifactListKeys = []string{
	"afterMovePaths",
	operationOutputPathsKey,
}

func (service *LibraryService) deleteUntrackedOperationOutputArtifacts(ctx context.Context, item library.LibraryOperation) error {
	paths := collectOperationOutputArtifactPaths(item.OutputJSON)
	paths = append(paths, service.collectResidualOperationArtifactPaths(ctx, item)...)
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		for _, candidate := range operationOutputArtifactPathVariants(path) {
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			if err := deleteLocalArtifactFileIfExists(candidate); err != nil {
				return err
			}
		}
	}
	return nil
}

func (service *LibraryService) collectResidualOperationArtifactPaths(ctx context.Context, item library.LibraryOperation) []string {
	tokens := collectOperationArtifactSearchTokens(item)
	if len(tokens) == 0 {
		return nil
	}
	baseDir, err := service.resolveYTDLPArtifactSearchBaseDir(ctx)
	if err != nil || strings.TrimSpace(baseDir) == "" {
		return nil
	}

	paths := make([]string, 0)
	_ = filepath.WalkDir(baseDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil || entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if !isResidualOperationArtifactName(name) {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil
		}
		if !operationArtifactMayBelongToOperation(info, item) {
			return nil
		}
		for _, token := range tokens {
			if strings.Contains(name, token) {
				paths = append(paths, path)
				break
			}
		}
		return nil
	})
	return paths
}

func (service *LibraryService) withOperationTemporaryArtifactPaths(ctx context.Context, item library.LibraryOperation) (library.LibraryOperation, bool) {
	paths := service.collectResidualOperationArtifactPaths(ctx, item)
	if len(paths) == 0 {
		return item, false
	}
	outputJSON, changed := withOperationArtifactPaths(item.OutputJSON, operationTemporaryPathsKey, paths...)
	if changed {
		item.OutputJSON = outputJSON
	}
	return item, changed
}

func operationArtifactMayBelongToOperation(info fs.FileInfo, item library.LibraryOperation) bool {
	if info == nil || item.CreatedAt.IsZero() {
		return true
	}
	return !info.ModTime().Before(item.CreatedAt)
}

func (service *LibraryService) resolveYTDLPArtifactSearchBaseDir(ctx context.Context) (string, error) {
	downloadDirectory, err := service.resolveDownloadDirectory(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(downloadDirectory) == "" {
		downloadDirectory, err = libraryBaseDir()
		if err != nil {
			return "", err
		}
	}
	baseDir := filepath.Join(downloadDirectory, "yt-dlp")
	if defaultBaseDir, defaultErr := libraryBaseDir(); defaultErr == nil {
		if !sameCleanPath(downloadDirectory, defaultBaseDir) && filepath.Base(filepath.Clean(downloadDirectory)) != "xiadown" {
			baseDir = filepath.Join(downloadDirectory, "xiadown", "yt-dlp")
		}
	}
	return baseDir, nil
}

func isResidualOperationArtifactName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return strings.HasSuffix(trimmed, ".part") ||
		strings.Contains(trimmed, ".part-") ||
		strings.HasSuffix(trimmed, ".ytdl")
}

func collectOperationArtifactSearchTokens(item library.LibraryOperation) []string {
	request := struct {
		URL string `json:"url"`
	}{}
	_ = json.Unmarshal([]byte(strings.TrimSpace(item.InputJSON)), &request)
	return extractOperationArtifactURLTokens(request.URL)
}

func extractOperationArtifactURLTokens(rawURL string) []string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil
	}
	tokens := make([]string, 0, 3)
	appendToken := func(value string) {
		token := normalizeOperationArtifactSearchToken(value)
		if token == "" {
			return
		}
		for _, existing := range tokens {
			if existing == token {
				return
			}
		}
		tokens = append(tokens, token)
	}

	appendToken(parsed.Query().Get("v"))
	appendToken(parsed.Query().Get("id"))

	pathParts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	host := strings.ToLower(parsed.Hostname())
	for index, part := range pathParts {
		unescaped, err := url.PathUnescape(part)
		if err != nil {
			unescaped = part
		}
		if strings.Contains(host, "youtu.be") ||
			index == len(pathParts)-1 ||
			(index > 0 && isLikelyMediaIDPathPrefix(pathParts[index-1])) {
			appendToken(unescaped)
		}
	}
	return tokens
}

func isLikelyMediaIDPathPrefix(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "embed", "shorts", "live", "video", "watch":
		return true
	default:
		return false
	}
}

func normalizeOperationArtifactSearchToken(value string) string {
	token := strings.Trim(strings.TrimSpace(value), "\"'` <>")
	if len(token) < 6 || len(token) > 80 {
		return ""
	}
	if strings.ContainsAny(token, `/\:`) {
		return ""
	}
	return token
}

func withOperationOutputArtifactPath(outputJSON string, path string) (string, bool) {
	return withOperationArtifactPaths(outputJSON, operationOutputPathsKey, path)
}

func withOperationArtifactPaths(outputJSON string, key string, paths ...string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" || len(paths) == 0 {
		return outputJSON, false
	}

	payload := map[string]any{}
	if strings.TrimSpace(outputJSON) != "" {
		_ = json.Unmarshal([]byte(outputJSON), &payload)
		if payload == nil {
			payload = map[string]any{}
		}
	}

	collectedPaths := make([]string, 0)
	appendPath := func(value string) bool {
		cleaned := normalizeOperationArtifactPath(value)
		if cleaned == "" {
			return false
		}
		for _, existing := range collectedPaths {
			if existing == cleaned {
				return false
			}
		}
		collectedPaths = append(collectedPaths, cleaned)
		return true
	}
	appendOperationArtifactList(payload[key], func(value string) {
		appendPath(value)
	})
	changed := false
	for _, path := range paths {
		if appendPath(path) {
			changed = true
		}
	}
	if !changed {
		return outputJSON, false
	}

	payload[key] = collectedPaths
	encoded, err := json.Marshal(payload)
	if err != nil {
		return outputJSON, false
	}
	return string(encoded), true
}

func mergeOperationOutputArtifactPaths(outputJSON string, sourceJSON string) (string, bool) {
	next := outputJSON
	changed := false
	for _, path := range collectOperationOutputFileArtifactPaths(sourceJSON) {
		updated, didChange := withOperationOutputArtifactPath(next, path)
		if didChange {
			next = updated
			changed = true
		}
	}
	return next, changed
}

func mergeOperationTemporaryArtifactPaths(outputJSON string, sourceJSON string) (string, bool) {
	next := outputJSON
	changed := false
	for _, path := range collectOperationArtifactPaths(sourceJSON, nil, []string{operationTemporaryPathsKey}) {
		updated, didChange := withOperationArtifactPaths(next, operationTemporaryPathsKey, path)
		if didChange {
			next = updated
			changed = true
		}
	}
	return next, changed
}

func collectOperationOutputArtifactPaths(outputJSON string) []string {
	return collectOperationArtifactPaths(outputJSON, operationOutputArtifactStringKeys, operationOutputArtifactListKeys)
}

func collectOperationOutputFileArtifactPaths(outputJSON string) []string {
	return collectOperationArtifactPaths(outputJSON, operationOutputArtifactFileStringKeys, operationOutputFileArtifactListKeys)
}

func collectOperationArtifactPaths(outputJSON string, stringKeys []string, listKeys []string) []string {
	trimmed := strings.TrimSpace(outputJSON)
	if trimmed == "" || trimmed == "{}" {
		return nil
	}

	payload := map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	paths := make([]string, 0)
	appendPath := func(value string) {
		cleaned := normalizeOperationArtifactPath(value)
		if cleaned == "" {
			return
		}
		if _, exists := seen[cleaned]; exists {
			return
		}
		seen[cleaned] = struct{}{}
		paths = append(paths, cleaned)
	}

	for _, key := range stringKeys {
		if value, ok := payload[key].(string); ok {
			appendPath(value)
		}
	}
	for _, key := range listKeys {
		appendOperationArtifactList(payload[key], appendPath)
	}

	return paths
}

func appendOperationArtifactList(value any, appendPath func(string)) {
	switch typed := value.(type) {
	case []string:
		for _, path := range typed {
			appendPath(path)
		}
	case []any:
		for _, item := range typed {
			if path, ok := item.(string); ok {
				appendPath(path)
			}
		}
	}
}

func normalizeOperationArtifactPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || !filepath.IsAbs(trimmed) {
		return ""
	}
	return filepath.Clean(trimmed)
}

func operationOutputArtifactPathVariants(path string) []string {
	cleaned := normalizeOperationArtifactPath(path)
	if cleaned == "" {
		return nil
	}
	return []string{
		cleaned,
		cleaned + ".part",
		cleaned + ".ytdl",
	}
}

func deleteLocalArtifactFileIfExists(path string) error {
	cleaned := normalizeOperationArtifactPath(path)
	if cleaned == "" {
		return nil
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return nil
	}
	return os.Remove(cleaned)
}
