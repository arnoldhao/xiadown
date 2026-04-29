package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/dependencies"
	"xiadown/internal/domain/library"
)

const (
	ytdlpCheckStatusOK       = "ok"
	ytdlpCheckStatusFailed   = "failed"
	ytdlpVersionCheckTimeout = 10 * time.Second
)

const (
	ytdlpErrorCodeDependencyMissing = "dependency_missing"
	ytdlpErrorCodeAuthRequired      = "auth_required"
	ytdlpErrorCodeRateLimited       = "rate_limited"
	ytdlpErrorCodeNetworkError      = "network_error"
	ytdlpErrorCodeOutputMissing     = "output_missing"
	ytdlpErrorCodeMisconfig         = "misconfig"
	ytdlpErrorCodeParsing           = "parsing_error"
	ytdlpErrorCodeExitCode          = "exit_code"
)

func shouldAutoRetryYTDLP(request dto.CreateYTDLPJobRequest, detail string) bool {
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if mode != "" && mode != "quick" {
		return false
	}
	if request.RetryCount > 0 {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(detail))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "login") || strings.Contains(lower, "sign in") || strings.Contains(lower, "cookies") {
		return false
	}
	transientSignals := []string{
		"http error 429",
		"http error 5",
		"status code 429",
		"too many requests",
		"timed out",
		"timeout",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"tls handshake timeout",
		"network is unreachable",
		"temporary failure",
	}
	for _, signal := range transientSignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

func (service *LibraryService) scheduleAutoRetryYTDLP(ctx context.Context, operation library.LibraryOperation, request dto.CreateYTDLPJobRequest, detail string) (string, bool) {
	if !shouldAutoRetryYTDLP(request, detail) {
		return "", false
	}
	retryRequest := withYTDLPOperationLibrary(request, operation)
	retryRequest.RetryOf = operation.ID
	retryRequest.RetryCount = request.RetryCount + 1
	newOperation, newHistory, _, err := service.createDownloadOperation(ctx, retryRequest)
	if err != nil {
		return "", false
	}
	go service.runYTDLPOperation(context.Background(), newOperation, newHistory, retryRequest)
	return newOperation.ID, true
}

func (service *LibraryService) CheckYTDLPOperationFailure(ctx context.Context, request dto.CheckYTDLPOperationFailureRequest) (dto.CheckYTDLPOperationFailureResponse, error) {
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		return dto.CheckYTDLPOperationFailureResponse{}, fmt.Errorf("operation id is required")
	}
	operation, err := service.operations.Get(ctx, operationID)
	if err != nil {
		return dto.CheckYTDLPOperationFailureResponse{}, err
	}
	if operation.Kind != "download" {
		return dto.CheckYTDLPOperationFailureResponse{}, fmt.Errorf("operation kind is not download")
	}

	input := dto.CreateYTDLPJobRequest{}
	_ = json.Unmarshal([]byte(operation.InputJSON), &input)
	domain := extractRegistrableDomain(input.URL)
	items := make([]dto.CheckYTDLPOperationFailureItem, 0, 5)

	if strings.TrimSpace(input.URL) != "" || domain != "" {
		ok, message := service.checkYTDLPConnectivity(ctx, input.URL, domain)
		status := ytdlpCheckStatusOK
		action := ""
		if !ok {
			status = ytdlpCheckStatusFailed
			action = "general"
		}
		items = append(items, dto.CheckYTDLPOperationFailureItem{
			ID:      "connectivity",
			Label:   "Network connectivity",
			Status:  status,
			Message: message,
			Action:  action,
		})
	}

	toolNames := []dependencies.DependencyName{dependencies.DependencyYTDLP, dependencies.DependencyFFmpeg, dependencies.DependencyBun}
	for _, toolName := range toolNames {
		status, message := service.checkYTDLPTool(ctx, toolName)
		items = append(items, dto.CheckYTDLPOperationFailureItem{
			ID:      string(toolName),
			Label:   string(toolName),
			Status:  status,
			Message: message,
			Action:  resolveToolAction(status),
		})
	}
	if status, message := service.checkYTDLPVersion(ctx); status != "" || message != "" {
		items = append(items, dto.CheckYTDLPOperationFailureItem{
			ID:      "yt-dlp-version",
			Label:   "yt-dlp version",
			Status:  status,
			Message: message,
			Action:  resolveToolAction(status),
		})
	}

	canRetry := operation.Status == library.OperationStatusFailed
	for _, item := range items {
		if item.Status != ytdlpCheckStatusOK && (item.ID == "connectivity" || item.ID == string(dependencies.DependencyYTDLP) || item.ID == string(dependencies.DependencyFFmpeg) || item.ID == string(dependencies.DependencyBun)) {
			canRetry = false
			break
		}
	}

	return dto.CheckYTDLPOperationFailureResponse{Items: items, CanRetry: canRetry}, nil
}

func (service *LibraryService) RetryYTDLPOperation(ctx context.Context, request dto.RetryYTDLPOperationRequest) (dto.LibraryOperationDTO, error) {
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operation id is required")
	}
	operation, err := service.operations.Get(ctx, operationID)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if operation.Kind != "download" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operation kind is not download")
	}
	input := dto.CreateYTDLPJobRequest{}
	if err := json.Unmarshal([]byte(operation.InputJSON), &input); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if trimmed := strings.TrimSpace(request.Source); trimmed != "" {
		input.Source = trimmed
	}
	if trimmed := strings.TrimSpace(request.RunID); trimmed != "" {
		input.RunID = trimmed
	}
	if trimmed := strings.TrimSpace(request.Caller); trimmed != "" {
		input.Caller = trimmed
	}
	input = withYTDLPOperationLibrary(input, operation)
	input.RetryOf = operation.ID
	input.RetryCount = input.RetryCount + 1
	newOperation, newHistory, _, err := service.createDownloadOperation(ctx, input)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	go service.runYTDLPOperation(context.Background(), newOperation, newHistory, input)
	return toOperationDTO(newOperation), nil
}

func resolveToolAction(status string) string {
	if status == ytdlpCheckStatusFailed {
		return "dependencies"
	}
	return ""
}

func (service *LibraryService) checkYTDLPTool(ctx context.Context, name dependencies.DependencyName) (string, string) {
	execPath, err := service.resolveTool(ctx, name)
	if err != nil {
		return ytdlpCheckStatusFailed, err.Error()
	}
	if !pathExists(execPath) {
		return ytdlpCheckStatusFailed, fmt.Sprintf("not found: %s", strings.TrimSpace(execPath))
	}
	return ytdlpCheckStatusOK, strings.TrimSpace(execPath)
}

func (service *LibraryService) checkYTDLPVersion(ctx context.Context) (string, string) {
	execPath, err := service.resolveTool(ctx, dependencies.DependencyYTDLP)
	if err != nil {
		return ytdlpCheckStatusFailed, err.Error()
	}
	versionCtx, cancel := context.WithTimeout(ctx, ytdlpVersionCheckTimeout)
	defer cancel()
	output, err := execCommandOutput(versionCtx, execPath, "--version")
	version := normalizeToolVersion(output)
	if version != "" {
		return ytdlpCheckStatusOK, version
	}
	if err != nil {
		return ytdlpCheckStatusFailed, err.Error()
	}
	return ytdlpCheckStatusFailed, "version not found"
}

func normalizeToolVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.TrimPrefix(trimmed, "v")
}

func withYTDLPOperationLibrary(request dto.CreateYTDLPJobRequest, operation library.LibraryOperation) dto.CreateYTDLPJobRequest {
	request.LibraryID = strings.TrimSpace(operation.LibraryID)
	return request
}

func (service *LibraryService) checkYTDLPConnectivity(ctx context.Context, url string, domain string) (bool, string) {
	target := strings.TrimSpace(url)
	if target == "" && strings.TrimSpace(domain) != "" {
		target = "https://" + strings.TrimSpace(domain)
	}
	if target == "" {
		return true, ""
	}
	requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodHead, target, nil)
	if err != nil {
		return false, err.Error()
	}
	client := &http.Client{Timeout: 3 * time.Second}
	if provider, ok := service.proxyClient.(interface{ HTTPClient() *http.Client }); ok {
		if proxyClient := provider.HTTPClient(); proxyClient != nil {
			client = proxyClient
		}
	}
	resp, err := client.Do(req)
	if err == nil && resp != nil && resp.StatusCode == http.StatusMethodNotAllowed {
		_ = resp.Body.Close()
		resp = nil
		err = fmt.Errorf("head not allowed")
	}
	if err != nil {
		getReq, getErr := http.NewRequestWithContext(requestCtx, http.MethodGet, target, nil)
		if getErr != nil {
			return false, err.Error()
		}
		getReq.Header.Set("Range", "bytes=0-0")
		resp, err = client.Do(getReq)
		if err != nil {
			return false, err.Error()
		}
	}
	if resp != nil {
		defer resp.Body.Close()
		return true, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return false, "request failed"
}

func execCommandOutput(ctx context.Context, execPath string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, execPath, args...)
	configureProcessGroup(command)
	output, err := command.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", err
		}
		return trimmed, fmt.Errorf("%w: %s", err, truncateOutput(output))
	}
	return trimmed, nil
}

func resolveYTDLPErrorCode(detail string, err error) string {
	combined := strings.TrimSpace(detail)
	if err != nil {
		errText := strings.TrimSpace(err.Error())
		if combined == "" {
			combined = errText
		} else if errText != "" && !strings.Contains(combined, errText) {
			combined = combined + " " + errText
		}
	}
	if code := classifyYTDLPErrorCode(combined); code != "" {
		return code
	}
	if isYTDLPMisconfigError(err) {
		return ytdlpErrorCodeMisconfig
	}
	if isYTDLPExitCodeError(err) {
		return ytdlpErrorCodeExitCode
	}
	return ""
}

func buildYTDLPFailureDetail(primary string, warning string, limit int) string {
	detail := mergeYTDLPFailureDetail(primary, warning)
	if limit > 0 && len(detail) > limit {
		detail = detail[:limit] + "..."
	}
	return detail
}

func buildYTDLPFailureDetailFromLogs(output string, stderr string, warning string, limit int) string {
	segments := make([]string, 0, 3)
	addSegment := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		for _, existing := range segments {
			if existing == trimmed || strings.Contains(existing, trimmed) || strings.Contains(trimmed, existing) {
				return
			}
		}
		segments = append(segments, trimmed)
	}
	if strings.TrimSpace(stderr) != "" {
		addSegment(stderr)
	} else {
		addSegment(warning)
	}
	if strings.TrimSpace(output) != "" {
		addSegment(output)
	}
	detail := strings.Join(segments, "\n")
	if limit > 0 && len(detail) > limit {
		detail = detail[:limit] + "..."
	}
	return detail
}

func mergeYTDLPFailureDetail(primary string, warning string) string {
	primary = strings.TrimSpace(primary)
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return primary
	}
	if primary == "" || isYTDLPPlaceholderDetail(primary) {
		return warning
	}
	if strings.Contains(primary, warning) {
		return primary
	}
	return warning + "\n" + primary
}

func isYTDLPPlaceholderDetail(detail string) bool {
	lower := strings.ToLower(strings.TrimSpace(detail))
	switch lower {
	case "na", "n/a", "null", "none":
		return true
	default:
		return false
	}
}

func classifyYTDLPErrorCode(detail string) string {
	lower := strings.ToLower(strings.TrimSpace(detail))
	if lower == "" {
		return ""
	}
	if isYTDLPDependencyMissing(lower) {
		return ytdlpErrorCodeDependencyMissing
	}
	if isYTDLPAuthRequired(lower) {
		return ytdlpErrorCodeAuthRequired
	}
	if isYTDLPRateLimited(lower) {
		return ytdlpErrorCodeRateLimited
	}
	if isYTDLPNetworkError(lower) {
		return ytdlpErrorCodeNetworkError
	}
	if isYTDLPParsingError(lower) {
		return ytdlpErrorCodeParsing
	}
	return ""
}

func isYTDLPMisconfigError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, exec.ErrDot)
}

func isYTDLPExitCodeError(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() != 0
	}
	return false
}

func isYTDLPDependencyMissing(lower string) bool {
	if containsAny(lower, []string{
		"ffmpeg not found",
		"ffprobe not found",
		"yt-dlp is not installed",
		"yt-dlp not found",
		"bun not found",
	}) {
		return true
	}
	if containsAny(lower, []string{"ffmpeg", "ffprobe", "yt-dlp", "bun"}) && containsAny(lower, []string{
		"not found",
		"no such file",
		"command not found",
		"executable file not found",
		"executable not found",
		"is not recognized as an internal or external command",
	}) {
		return true
	}
	return false
}

func isYTDLPAuthRequired(lower string) bool {
	return containsAny(lower, []string{
		"cookies",
		"cookie",
		"login",
		"log in",
		"sign in",
		"signin",
		"authentication",
		"authorization",
		"unauthorized",
		"forbidden",
		"account required",
		"members only",
	})
}

func isYTDLPRateLimited(lower string) bool {
	return containsAny(lower, []string{
		"429",
		"too many requests",
		"rate limit",
		"rate-limit",
		"temporarily blocked",
		"try again later",
	})
}

func isYTDLPNetworkError(lower string) bool {
	return containsAny(lower, []string{
		"network is unreachable",
		"connection refused",
		"connection reset",
		"connection aborted",
		"tls handshake timeout",
		"i/o timeout",
		"timed out",
		"temporary failure in name resolution",
		"no route to host",
	})
}

func isYTDLPParsingError(lower string) bool {
	return containsAny(lower, []string{
		"unable to extract",
		"unsupported url",
		"unsupported site",
		"extractor error",
		"json parse",
		"failed to parse",
	})
}

func containsAny(value string, items []string) bool {
	for _, item := range items {
		if strings.Contains(value, item) {
			return true
		}
	}
	return false
}

func truncateOutput(output []byte) string {
	const maxBytes = 2000
	if len(output) <= maxBytes {
		return strings.TrimSpace(string(output))
	}
	return strings.TrimSpace(string(output[:maxBytes])) + "..."
}
