package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	connectorsservice "xiadown/internal/application/connectors/service"
	"xiadown/internal/application/library/dto"
	appytdlp "xiadown/internal/application/ytdlp"
	"xiadown/internal/domain/dependencies"
	"xiadown/internal/domain/library"
	ydlpinfr "xiadown/internal/infrastructure/ytdlp"
)

type ytdlpOutputSnapshot struct {
	files              []library.LibraryFile
	outputFiles        []library.OperationOutputFile
	outputPaths        []string
	resolvedSubtitles  []string
	resolvedThumbnails []string
}

type ytdlpAuxiliaryStep struct {
	warning string
	result  ydlpinfr.RunResult
}

type ytdlpThumbnailPrefetch struct {
	mu       sync.Mutex
	started  bool
	done     chan struct{}
	tempPath string
	err      error
}

func (prefetch *ytdlpThumbnailPrefetch) start(download func() (string, error), onReady func(string)) {
	if prefetch == nil || download == nil {
		return
	}
	prefetch.mu.Lock()
	if prefetch.started {
		prefetch.mu.Unlock()
		return
	}
	prefetch.started = true
	prefetch.done = make(chan struct{})
	done := prefetch.done
	prefetch.mu.Unlock()

	go func() {
		path, err := download()
		trimmedPath := strings.TrimSpace(path)
		prefetch.mu.Lock()
		prefetch.tempPath = trimmedPath
		prefetch.err = err
		prefetch.mu.Unlock()
		if err == nil && trimmedPath != "" && onReady != nil {
			onReady(trimmedPath)
		}
		close(done)
	}()
}

func (prefetch *ytdlpThumbnailPrefetch) Start(
	ctx context.Context,
	service *LibraryService,
	request dto.CreateYTDLPJobRequest,
	outputTemplate string,
	operationID string,
	onReady func(string),
) {
	if prefetch == nil || service == nil || strings.TrimSpace(request.ThumbnailURL) == "" {
		return
	}
	prefetch.start(func() (string, error) {
		return service.downloadYTDLPThumbnailPrefetch(ctx, request, outputTemplate, operationID)
	}, onReady)
}

func (prefetch *ytdlpThumbnailPrefetch) Wait() (string, error) {
	if prefetch == nil {
		return "", nil
	}
	prefetch.mu.Lock()
	done := prefetch.done
	started := prefetch.started
	prefetch.mu.Unlock()
	if !started || done == nil {
		return "", nil
	}
	<-done
	prefetch.mu.Lock()
	defer prefetch.mu.Unlock()
	return strings.TrimSpace(prefetch.tempPath), prefetch.err
}

func (prefetch *ytdlpThumbnailPrefetch) Cleanup() {
	if prefetch == nil {
		return
	}
	prefetch.mu.Lock()
	done := prefetch.done
	tempPath := strings.TrimSpace(prefetch.tempPath)
	prefetch.mu.Unlock()
	if done != nil {
		<-done
		prefetch.mu.Lock()
		tempPath = strings.TrimSpace(prefetch.tempPath)
		prefetch.mu.Unlock()
	}
	if tempPath == "" {
		return
	}
	_ = os.Remove(tempPath)
	_ = os.Remove(filepath.Dir(tempPath))
}

func (service *LibraryService) createDownloadOperation(ctx context.Context, request dto.CreateYTDLPJobRequest) (library.LibraryOperation, library.HistoryRecord, library.Library, error) {
	displayName := strings.TrimSpace(request.Title)
	if displayName == "" {
		displayName = strings.TrimSpace(request.URL)
	}
	now := service.now()
	operationID := uuid.NewString()
	libraryItem, err := service.ensureLibrary(ctx, ensureLibraryParams{
		LibraryID:          request.LibraryID,
		InitialNameFromID:  true,
		CreatedBySource:    "download",
		TriggerOperationID: operationID,
	})
	if err != nil {
		return library.LibraryOperation{}, library.HistoryRecord{}, library.Library{}, err
	}
	request.LibraryID = libraryItem.ID
	inputJSON, err := json.Marshal(request)
	if err != nil {
		return library.LibraryOperation{}, library.HistoryRecord{}, library.Library{}, err
	}
	correlation := library.OperationCorrelation{RunID: strings.TrimSpace(request.RunID)}
	meta := library.OperationMeta{Platform: strings.TrimSpace(request.Extractor), Uploader: strings.TrimSpace(request.Author)}
	sourceDomain := extractRegistrableDomain(request.URL)
	sourceIcon := ""
	if service.iconResolver != nil && sourceDomain != "" {
		if icon, iconErr := service.iconResolver.ResolveDomainIcon(ctx, sourceDomain); iconErr == nil {
			sourceIcon = icon
		}
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:           operationID,
		LibraryID:    libraryItem.ID,
		Kind:         "download",
		Status:       string(library.OperationStatusQueued),
		DisplayName:  displayName,
		Correlation:  correlation,
		InputJSON:    string(inputJSON),
		OutputJSON:   "{}",
		SourceDomain: sourceDomain,
		SourceIcon:   sourceIcon,
		Meta:         meta,
		CreatedAt:    &now,
	})
	if err != nil {
		return library.LibraryOperation{}, library.HistoryRecord{}, library.Library{}, err
	}
	if err := service.operations.Save(ctx, operation); err != nil {
		return library.LibraryOperation{}, library.HistoryRecord{}, library.Library{}, err
	}
	history, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          uuid.NewString(),
		LibraryID:   libraryItem.ID,
		Category:    "operation",
		Action:      operation.Kind,
		DisplayName: operation.DisplayName,
		Status:      string(operation.Status),
		Source: library.HistoryRecordSource{
			Kind:   resolveHistorySourceKind(request.Source),
			Caller: strings.TrimSpace(request.Caller),
			RunID:  strings.TrimSpace(request.RunID),
		},
		Refs:    library.HistoryRecordRefs{OperationID: operation.ID},
		Metrics: operation.Metrics,
		OperationMeta: &library.OperationRecordMeta{
			Kind: operation.Kind,
		},
		OccurredAt: &now,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	})
	if err != nil {
		return library.LibraryOperation{}, library.HistoryRecord{}, library.Library{}, err
	}
	if err := service.histories.Save(ctx, history); err != nil {
		return library.LibraryOperation{}, library.HistoryRecord{}, library.Library{}, err
	}
	if err := service.touchLibrary(ctx, libraryItem.ID, now); err != nil {
		return library.LibraryOperation{}, library.HistoryRecord{}, library.Library{}, err
	}
	service.publishOperationUpdate(toOperationDTO(operation))
	service.publishHistoryUpdate(toHistoryDTO(history))
	return operation, history, libraryItem, nil
}

func (service *LibraryService) runYTDLPOperation(ctx context.Context, operation library.LibraryOperation, history library.HistoryRecord, request dto.CreateYTDLPJobRequest) {
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

	execPath, err := service.resolveTool(ctx, dependencies.DependencyYTDLP)
	if err != nil {
		service.failYTDLPOperation(ctx, &operation, &history, err, ytdlpErrorCodeDependencyMissing, "")
		return
	}
	outputTemplate, subtitleTemplate, _, err := service.prepareYTDLPOutput(ctx)
	if err != nil {
		service.failYTDLPOperation(ctx, &operation, &history, err, resolveYTDLPErrorCode("", err), "")
		return
	}

	cookiesPath := strings.TrimSpace(request.CookiesPath)
	cleanupCookies := func() {}
	if request.UseConnector && strings.TrimSpace(request.ConnectorID) != "" && service.connectors != nil {
		exported, exportErr := service.connectors.ExportConnectorCookies(ctx, request.ConnectorID, connectorsservice.CookiesExportTXT)
		if exportErr != nil {
			service.failYTDLPOperation(ctx, &operation, &history, exportErr, resolveYTDLPErrorCode("", exportErr), "")
			return
		}
		cookiesPath = exported
		cleanupCookies = func() { _ = os.Remove(exported) }
	}
	defer cleanupCookies()

	logPolicy := resolveYTDLPLogPolicy(request)
	persistLogsOnFailure := logPolicy != ytdlpLogPolicyNever
	persistLogsOnSuccess := logPolicy == ytdlpLogPolicyAlways
	reporter := newYTDLPProgressReporter(service, &operation)
	thumbnailPrefetch := &ytdlpThumbnailPrefetch{}
	command, err := appytdlp.BuildCommand(ctx, appytdlp.CommandOptions{
		ExecPath:       execPath,
		Tools:          service.tools,
		Request:        request,
		OutputTemplate: outputTemplate,
		CookiesPath:    cookiesPath,
		ProxyURL:       service.resolveYTDLPProxy(request.URL),
	})
	if err != nil {
		service.failYTDLPOperation(ctx, &operation, &history, err, resolveYTDLPErrorCode("", err), "")
		return
	}
	defer command.Cancel()
	if command.Cleanup != nil {
		defer command.Cleanup()
	}
	defer thumbnailPrefetch.Cleanup()
	if request.WriteThumbnail && strings.TrimSpace(request.ThumbnailURL) != "" {
		thumbnailPrefetch.Start(context.Background(), service, request, outputTemplate, operation.ID, reporter.publishThumbnailPreviewPath)
	}

	result, runErr := service.executeYTDLPCommand(operation, command, reporter, func(_ string, line string) {
		info := parseJSONMap(line)
		if len(info) == 0 {
			return
		}
		reporter.WithOperationLock(func() {
			service.applyYTDLPMetadataToOperationAndHistory(
				context.Background(),
				&operation,
				&history,
				&request,
				map[string]any{"info": info},
			)
			if request.WriteThumbnail {
				thumbnailPrefetch.Start(context.Background(), service, request, outputTemplate, operation.ID, reporter.publishThumbnailPreviewPath)
			}
		})
	})
	if runErr != nil {
		logSnapshot := service.persistYTDLPLogs(ctx, operation, result, persistLogsOnFailure, persistLogsOnSuccess, runErr)
		origin := appytdlp.Origin{Source: strings.TrimSpace(request.Source), RunID: strings.TrimSpace(request.RunID), Caller: strings.TrimSpace(request.Caller)}
		metadataPayload := appytdlp.BuildMetadataPayload(result, logSnapshot, command.Cmd.Env, command.SanitizedArgs, origin)
		service.applyYTDLPMetadata(&operation, &request, metadataPayload)
		_, _, afterMovePaths, outputPaths := appytdlp.ResolveOutputPath(outputTemplate, result)
		detail := buildYTDLPFailureDetailFromLogs(result.Output, result.Stderr, result.Warnings, 2000)
		errorCode := resolveYTDLPErrorCode(detail, runErr)
		if retryID, ok := service.scheduleAutoRetryYTDLP(context.Background(), operation, request, detail); ok {
			detail = buildYTDLPFailureDetail(detail, fmt.Sprintf("auto-retry scheduled: %s", retryID), 2000)
		}
		failureErr := fmt.Errorf("yt-dlp failed: %w", runErr)
		if strings.TrimSpace(detail) != "" {
			failureErr = fmt.Errorf("%s", detail)
		}
		service.failYTDLPOperation(ctx, &operation, &history, failureErr, errorCode, buildOperationOutputPayload("", afterMovePaths, outputPaths, nil, metadataPayload, logSnapshot))
		return
	}
	service.applyYTDLPMetadata(&operation, &request, appytdlp.BuildMetadataPayload(result, appytdlp.LogSnapshot{}, nil, nil, appytdlp.Origin{}))

	outputPath, resolvedAfterMove, afterMovePaths, outputPaths := appytdlp.ResolveOutputPath(outputTemplate, result)
	outputPath = resolveExistingOutputPath(outputPath, outputPaths)
	if strings.TrimSpace(outputPath) == "" || !pathExists(outputPath) {
		logSnapshot := service.persistYTDLPLogs(ctx, operation, result, persistLogsOnFailure, persistLogsOnSuccess, nil)
		origin := appytdlp.Origin{Source: strings.TrimSpace(request.Source), RunID: strings.TrimSpace(request.RunID), Caller: strings.TrimSpace(request.Caller)}
		metadataPayload := appytdlp.BuildMetadataPayload(result, logSnapshot, command.Cmd.Env, command.SanitizedArgs, origin)
		service.applyYTDLPMetadata(&operation, &request, metadataPayload)
		detail := buildYTDLPFailureDetailFromLogs(result.Output, result.Stderr, result.Warnings, 2000)
		errorCode := resolveYTDLPErrorCode(detail, nil)
		if errorCode == "" {
			errorCode = ytdlpErrorCodeOutputMissing
		}
		service.failYTDLPOperation(ctx, &operation, &history, fmt.Errorf("yt-dlp produced no output"), errorCode, buildOperationOutputPayload("", afterMovePaths, outputPaths, nil, metadataPayload, logSnapshot))
		return
	}

	title := strings.TrimSpace(operation.DisplayName)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	}
	primaryFile, err := service.createDownloadedPrimaryFile(ctx, operation, request, title, outputPath, started)
	if err != nil {
		logSnapshot := service.persistYTDLPLogs(ctx, operation, result, persistLogsOnFailure, persistLogsOnSuccess, nil)
		origin := appytdlp.Origin{Source: strings.TrimSpace(request.Source), RunID: strings.TrimSpace(request.RunID), Caller: strings.TrimSpace(request.Caller)}
		metadataPayload := appytdlp.BuildMetadataPayload(result, logSnapshot, command.Cmd.Env, command.SanitizedArgs, origin)
		service.applyYTDLPMetadata(&operation, &request, metadataPayload)
		service.failYTDLPOperation(ctx, &operation, &history, err, resolveYTDLPErrorCode("", err), buildOperationOutputPayload(outputPath, afterMovePaths, outputPaths, nil, metadataPayload, logSnapshot))
		return
	}

	auxiliaryWarnings := make([]string, 0, 2)
	explicitThumbnailPaths := make([]string, 0, 1)
	if request.WriteThumbnail {
		prefetchWarning := ""
		if prefetchedThumbnailPath, err := thumbnailPrefetch.Wait(); err != nil {
			prefetchWarning = fmt.Sprintf("thumbnail prefetch failed: %v", err)
		} else if strings.TrimSpace(prefetchedThumbnailPath) != "" {
			if promotedPath, promoteErr := service.promotePrefetchedYTDLPThumbnail(prefetchedThumbnailPath, outputPath); promoteErr != nil {
				prefetchWarning = fmt.Sprintf("thumbnail download failed: %v", promoteErr)
			} else if strings.TrimSpace(promotedPath) != "" {
				explicitThumbnailPaths = append(explicitThumbnailPaths, promotedPath)
			}
		}
		if len(explicitThumbnailPaths) == 0 {
			if previewPath := service.persistedYTDLPThumbnailPreviewPath(ctx, operation); previewPath != "" {
				if promotedPath, promoteErr := service.promotePrefetchedYTDLPThumbnail(previewPath, outputPath); promoteErr != nil {
					auxiliaryWarnings = append(auxiliaryWarnings, fmt.Sprintf("thumbnail preview promotion failed: %v", promoteErr))
				} else if strings.TrimSpace(promotedPath) != "" {
					explicitThumbnailPaths = append(explicitThumbnailPaths, promotedPath)
				}
			}
		}
		if len(explicitThumbnailPaths) == 0 && prefetchWarning != "" {
			auxiliaryWarnings = append(auxiliaryWarnings, prefetchWarning)
		}
		if request.ThumbnailURL == "" {
			request.ThumbnailURL = extractYTDLPThumbnailURL(result)
		}
		if len(explicitThumbnailPaths) == 0 {
			if thumbnailPath, err := service.downloadYTDLPThumbnail(ctx, reporter, request, outputPath); err != nil {
				auxiliaryWarnings = append(auxiliaryWarnings, fmt.Sprintf("thumbnail download failed: %v", err))
			} else if strings.TrimSpace(thumbnailPath) != "" {
				explicitThumbnailPaths = append(explicitThumbnailPaths, thumbnailPath)
			}
		}
	}
	if wantsYTDLPSubtitles(request) {
		if step := service.downloadYTDLPSubtitles(ctx, reporter, execPath, request, outputTemplate, subtitleTemplate, outputPath, cookiesPath); step.warning != "" {
			auxiliaryWarnings = append(auxiliaryWarnings, step.warning)
			result = mergeYTDLPRunResults(result, step.result)
		} else {
			result = mergeYTDLPRunResults(result, step.result)
		}
	}

	logSnapshot := service.persistYTDLPLogs(ctx, operation, result, persistLogsOnFailure, persistLogsOnSuccess, nil)
	origin := appytdlp.Origin{Source: strings.TrimSpace(request.Source), RunID: strings.TrimSpace(request.RunID), Caller: strings.TrimSpace(request.Caller)}
	metadataPayload := appytdlp.BuildMetadataPayload(result, logSnapshot, command.Cmd.Env, command.SanitizedArgs, origin)
	if len(auxiliaryWarnings) > 0 {
		if metadataPayload == nil {
			metadataPayload = map[string]any{}
		}
		metadataPayload["auxiliaryWarnings"] = auxiliaryWarnings
	}
	service.applyYTDLPMetadata(&operation, &request, metadataPayload)

	outputSnapshot, err := service.buildYTDLPOutputs(ctx, request, operation, primaryFile, started, outputPath, resolvedAfterMove, outputPaths, result.SubtitleLogPaths, explicitThumbnailPaths)
	if err != nil {
		service.failYTDLPOperation(ctx, &operation, &history, err, resolveYTDLPErrorCode("", err), buildOperationOutputPayload(outputPath, afterMovePaths, outputPaths, nil, metadataPayload, logSnapshot))
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
			service.failYTDLPOperation(
				ctx,
				&operation,
				&history,
				transcodeErr,
				resolveYTDLPErrorCode("", transcodeErr),
				buildOperationOutputPayload(finalMainPath, afterMovePaths, updatedSnapshot.outputPaths, updatedSnapshot.outputFiles, metadataPayload, logSnapshot),
			)
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
	operation.OutputJSON = buildOperationOutputPayload(finalMainPath, afterMovePaths, outputSnapshot.outputPaths, outputSnapshot.outputFiles, metadataPayload, logSnapshot)
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
		service.syncDreamFMLocalTrackFromFile(ctx, fileItem, nil)
		service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
	}
}

func (service *LibraryService) persistOperationAndHistory(ctx context.Context, operation *library.LibraryOperation, history *library.HistoryRecord) error {
	if operation == nil || history == nil {
		return nil
	}
	if err := service.operations.Save(ctx, *operation); err != nil {
		return err
	}
	if err := service.histories.Save(ctx, *history); err != nil {
		return err
	}
	if err := service.touchLibrary(ctx, operation.LibraryID, service.now()); err != nil {
		return err
	}
	service.publishOperationUpdate(toOperationDTO(*operation))
	service.publishHistoryUpdate(toHistoryDTO(*history))
	service.trackCompletedOperation(ctx, *operation)
	return nil
}

func (service *LibraryService) failYTDLPOperation(ctx context.Context, operation *library.LibraryOperation, history *library.HistoryRecord, runErr error, errorCode string, outputJSON string) {
	if operation == nil || history == nil {
		return
	}
	finished := service.now()
	if errors.Is(runErr, context.Canceled) {
		operation.Status = library.OperationStatusCanceled
		operation.ErrorCode = "download_canceled"
		operation.ErrorMessage = ""
	} else {
		operation.Status = library.OperationStatusFailed
		operation.ErrorCode = strings.TrimSpace(errorCode)
		operation.ErrorMessage = strings.TrimSpace(runErr.Error())
	}
	operation.FinishedAt = &finished
	operation.Metrics.DurationMs = durationMsBetween(operation.StartedAt, operation.FinishedAt)
	if strings.TrimSpace(outputJSON) != "" {
		operation.OutputJSON = outputJSON
	}
	if service != nil && service.operations != nil {
		if latest, err := service.operations.Get(ctx, operation.ID); err == nil {
			if mergedOutputJSON, changed := mergeOperationOutputArtifactPaths(operation.OutputJSON, latest.OutputJSON); changed {
				operation.OutputJSON = mergedOutputJSON
			}
			if mergedOutputJSON, changed := mergeOperationTemporaryArtifactPaths(operation.OutputJSON, latest.OutputJSON); changed {
				operation.OutputJSON = mergedOutputJSON
			}
		}
	}
	if updated, changed := service.withOperationTemporaryArtifactPaths(ctx, *operation); changed {
		operation.OutputJSON = updated.OutputJSON
	}
	if operation.Progress == nil {
		operation.Progress = &library.OperationProgress{}
	}
	operation.Progress.Stage = "Failed"
	operation.Progress.Message = terminalProgressMessage(operation.Kind, operation.Status)
	operation.Progress.UpdatedAt = finished.Format(time.RFC3339)
	history.Status = string(operation.Status)
	history.DisplayName = operation.DisplayName
	history.Metrics = operation.Metrics
	history.OperationMeta = &library.OperationRecordMeta{Kind: operation.Kind, ErrorCode: operation.ErrorCode, ErrorMessage: operation.ErrorMessage}
	history.UpdatedAt = finished
	history.OccurredAt = finished
	_ = service.persistOperationAndHistory(ctx, operation, history)
}

func (service *LibraryService) resolveTool(ctx context.Context, name dependencies.DependencyName) (string, error) {
	if service == nil || service.tools == nil {
		return "", fmt.Errorf("tool resolver not configured")
	}
	return service.tools.ResolveExecPath(ctx, name)
}

func (service *LibraryService) executeYTDLPCommand(
	operation library.LibraryOperation,
	command appytdlp.Command,
	reporter *ytdlpProgressReporter,
	logLine func(pipe string, line string),
) (ydlpinfr.RunResult, error) {
	return ydlpinfr.Run(ydlpinfr.RunOptions{
		Command:        command.Cmd,
		Progress:       reporter,
		LogLine:        logLine,
		OutputPath:     reporter.publishOutputArtifactPath,
		PrintFilePath:  command.PrintFilePath,
		ProgressPrefix: ytdlpProgressPrefix,
		OnStarted: func(cmd *exec.Cmd) func() {
			return appytdlp.StartProcessGroupKiller(command.Ctx, cmd, 2*time.Second)
		},
	})
}

func (service *LibraryService) persistYTDLPLogs(ctx context.Context, operation library.LibraryOperation, result ydlpinfr.RunResult, persistOnFailure bool, persistOnSuccess bool, runErr error) appytdlp.LogSnapshot {
	// yt-dlp stdout/stderr stays in memory for progress, metadata, and error summaries.
	// Avoid persisting full command logs so downloads do not create extra log folders.
	return appytdlp.LogSnapshot{}
}

func (service *LibraryService) prepareYTDLPOutput(ctx context.Context) (string, string, string, error) {
	downloadDirectory, err := service.resolveDownloadDirectory(ctx)
	if err != nil {
		return "", "", "", err
	}
	if downloadDirectory == "" {
		fallback, err := libraryBaseDir()
		if err != nil {
			return "", "", "", err
		}
		downloadDirectory = fallback
	}
	baseDir := filepath.Join(downloadDirectory, "yt-dlp")
	if defaultBaseDir, defaultErr := libraryBaseDir(); defaultErr == nil {
		if !sameCleanPath(downloadDirectory, defaultBaseDir) && filepath.Base(filepath.Clean(downloadDirectory)) != "xiadown" {
			baseDir = filepath.Join(downloadDirectory, "xiadown", "yt-dlp")
		}
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", "", "", err
	}
	outputTemplate := filepath.Join(baseDir, "%(extractor)s", "%(uploader)s-%(title)s-%(format_note)s-%(id)s.%(ext)s")
	subtitleTemplate := filepath.Join(baseDir, "%(extractor)s", "subtitles", "%(uploader)s-%(title)s-%(id)s.%(ext)s")
	thumbnailTemplate := filepath.Join(baseDir, "%(extractor)s", "thumbnails", "%(uploader)s-%(title)s-%(id)s.%(ext)s")
	return outputTemplate, subtitleTemplate, thumbnailTemplate, nil
}

func sameCleanPath(left string, right string) bool {
	trimmedLeft := strings.TrimSpace(left)
	trimmedRight := strings.TrimSpace(right)
	if trimmedLeft == "" || trimmedRight == "" {
		return false
	}
	return filepath.Clean(trimmedLeft) == filepath.Clean(trimmedRight)
}

func (service *LibraryService) buildYTDLPOutputs(ctx context.Context, request dto.CreateYTDLPJobRequest, operation library.LibraryOperation, primaryFile library.LibraryFile, started time.Time, outputPath string, resolvedAfterMove string, outputPaths []string, subtitleLogPaths []string, thumbnailPaths []string) (ytdlpOutputSnapshot, error) {
	files := []library.LibraryFile{primaryFile}
	outputs := []library.OperationOutputFile{{
		FileID:    primaryFile.ID,
		Kind:      string(primaryFile.Kind),
		Format:    mediaFormatFromFile(primaryFile),
		SizeBytes: mediaSizeFromFile(primaryFile),
		IsPrimary: true,
		Deleted:   primaryFile.State.Deleted,
	}}
	seenPaths := map[string]struct{}{strings.TrimSpace(primaryFile.Storage.LocalPath): {}}

	candidatePaths := make([]string, 0, len(outputPaths)+1)
	if resolved := strings.TrimSpace(resolvedAfterMove); resolved != "" {
		candidatePaths = append(candidatePaths, resolved)
	}
	candidatePaths = append(candidatePaths, outputPaths...)

	subtitleDirs := []string{filepath.Dir(outputPath), filepath.Join(filepath.Dir(outputPath), "subtitles")}
	thumbnailDirs := []string{filepath.Dir(outputPath), filepath.Join(filepath.Dir(outputPath), "thumbnails")}
	for _, candidate := range candidatePaths {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || !pathExists(candidate) {
			continue
		}
		dir := filepath.Dir(candidate)
		subtitleDirs = append(subtitleDirs, dir, filepath.Join(dir, "subtitles"))
		thumbnailDirs = append(thumbnailDirs, dir, filepath.Join(dir, "thumbnails"))
	}

	resolvedSubtitlePaths := make([]string, 0)
	for _, subtitlePath := range subtitleLogPaths {
		resolved := resolveExistingAuxiliaryPath(subtitlePath, subtitleDirs)
		if resolved != "" {
			resolvedSubtitlePaths = append(resolvedSubtitlePaths, resolved)
		}
	}
	resolvedSubtitlePaths = append(
		resolvedSubtitlePaths,
		ydlpinfr.FindSubtitleOutputs(outputPath, started, resolveAssetPathFallback, subtitleDirs...)...,
	)
	resolvedSubtitlePaths = dedupePaths(resolvedSubtitlePaths)
	for _, subtitlePath := range resolvedSubtitlePaths {
		if _, ok := seenPaths[subtitlePath]; ok {
			continue
		}
		fileItem, err := service.createDownloadedSubtitleFile(ctx, operation, subtitlePath, primaryFile.Name, started)
		if err != nil {
			return ytdlpOutputSnapshot{}, fmt.Errorf("create subtitle output %q: %w", subtitlePath, err)
		}
		seenPaths[subtitlePath] = struct{}{}
		files = append(files, fileItem)
		outputs = append(outputs, library.OperationOutputFile{FileID: fileItem.ID, Kind: string(fileItem.Kind), Format: mediaFormatFromFile(fileItem), SizeBytes: mediaSizeFromFile(fileItem), Deleted: fileItem.State.Deleted})
	}

	resolvedThumbnailPaths := ydlpinfr.FindThumbnailOutputs(outputPath, started, resolveAssetPathFallback, thumbnailDirs...)
	for _, thumbnailPath := range thumbnailPaths {
		resolved := resolveExistingAuxiliaryPath(thumbnailPath, thumbnailDirs)
		if resolved != "" {
			resolvedThumbnailPaths = append(resolvedThumbnailPaths, resolved)
		}
	}
	resolvedThumbnailPaths = dedupePaths(resolvedThumbnailPaths)
	for _, thumbnailPath := range resolvedThumbnailPaths {
		if _, ok := seenPaths[thumbnailPath]; ok {
			continue
		}
		fileItem, err := service.createDownloadedBinaryFile(ctx, operation, string(library.FileKindThumbnail), thumbnailPath, primaryFile.Name, started)
		if err != nil {
			continue
		}
		seenPaths[thumbnailPath] = struct{}{}
		files = append(files, fileItem)
		outputs = append(outputs, library.OperationOutputFile{FileID: fileItem.ID, Kind: string(fileItem.Kind), Format: mediaFormatFromFile(fileItem), SizeBytes: mediaSizeFromFile(fileItem), Deleted: fileItem.State.Deleted})
	}

	return ytdlpOutputSnapshot{files: files, outputFiles: outputs, outputPaths: dedupePaths(candidatePaths), resolvedSubtitles: resolvedSubtitlePaths, resolvedThumbnails: resolvedThumbnailPaths}, nil
}

func (service *LibraryService) runDownloadEmbeddedTranscode(
	ctx context.Context,
	operation *library.LibraryOperation,
	request dto.CreateYTDLPJobRequest,
	primaryFile library.LibraryFile,
	title string,
	snapshot ytdlpOutputSnapshot,
) (ytdlpOutputSnapshot, string, error) {
	if service == nil || operation == nil {
		return snapshot, "", fmt.Errorf("download operation is required")
	}

	transcodeRequest := dto.CreateTranscodeJobRequest{
		FileID:                         primaryFile.ID,
		LibraryID:                      primaryFile.LibraryID,
		RootFileID:                     rootFileID(primaryFile),
		PresetID:                       strings.TrimSpace(request.TranscodePresetID),
		Title:                          strings.TrimSpace(title),
		CoverPath:                      resolveSnapshotTranscodeCoverPath(snapshot),
		SubtitlePaths:                  dedupePaths(snapshot.resolvedSubtitles),
		Source:                         "YTDLP",
		RunID:                          strings.TrimSpace(request.RunID),
		DeleteSourceFileAfterTranscode: request.DeleteSourceFileAfterTranscode,
	}

	result, err := service.runEmbeddedTranscodeStage(ctx, operation, primaryFile, transcodeRequest)
	if err != nil {
		return snapshot, "", err
	}

	snapshot.files = replaceYTDLPOutputFile(snapshot.files, result.sourceFile)
	snapshot.files = append(snapshot.files, result.outputFile)
	snapshot.outputFiles = append(snapshot.outputFiles, result.output)
	snapshot.outputPaths = dedupePaths(append(snapshot.outputPaths, result.outputPath))
	if result.sourceFile.State.Deleted {
		snapshot.outputFiles, _ = markOperationOutputFileDeleted(snapshot.outputFiles, result.sourceFile.ID)
	}

	return snapshot, result.outputPath, nil
}

func replaceYTDLPOutputFile(items []library.LibraryFile, updated library.LibraryFile) []library.LibraryFile {
	if len(items) == 0 {
		return items
	}
	result := append([]library.LibraryFile(nil), items...)
	for index := range result {
		if result[index].ID != updated.ID {
			continue
		}
		result[index] = updated
		return result
	}
	return result
}

func (service *LibraryService) createDownloadedPrimaryFile(ctx context.Context, operation library.LibraryOperation, request dto.CreateYTDLPJobRequest, title string, outputPath string, createdAt time.Time) (library.LibraryFile, error) {
	kind := string(library.FileKindVideo)
	if strings.EqualFold(strings.TrimSpace(request.Quality), "audio") {
		kind = string(library.FileKindAudio)
	}
	probe, err := service.probeRequiredMedia(ctx, outputPath)
	if err != nil {
		return library.LibraryFile{}, err
	}
	media := probe.toMediaInfo()
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:                uuid.NewString(),
		LibraryID:         operation.LibraryID,
		Kind:              kind,
		Name:              resolveStoredFileName(outputPath, title),
		DisplayName:       strings.TrimSpace(title),
		Metadata:          buildDownloadFileMetadata(operation, title),
		Storage:           library.FileStorage{Mode: "local_path", LocalPath: outputPath},
		Origin:            library.FileOrigin{Kind: "download", OperationID: operation.ID},
		LatestOperationID: operation.ID,
		Media:             &media,
		State:             library.FileState{Status: "active"},
		CreatedAt:         &createdAt,
		UpdatedAt:         &createdAt,
	})
	if err != nil {
		return library.LibraryFile{}, err
	}
	if err := service.files.Save(ctx, fileItem); err != nil {
		return library.LibraryFile{}, err
	}
	if err := service.renameLibraryFromFirstFileIfNeeded(ctx, operation.LibraryID, fileItem.DisplayName, createdAt); err != nil {
		return library.LibraryFile{}, err
	}
	service.syncDreamFMLocalTrackFromFile(ctx, fileItem, &probe)
	return fileItem, nil
}

func (service *LibraryService) createDownloadedSubtitleFile(ctx context.Context, operation library.LibraryOperation, path string, baseTitle string, createdAt time.Time) (library.LibraryFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return library.LibraryFile{}, err
	}
	if service == nil || service.subtitles == nil {
		return library.LibraryFile{}, fmt.Errorf("subtitle document repository not configured")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return library.LibraryFile{}, err
	}
	format := detectSubtitleFormat("", path, "")
	title := ydlpinfr.BuildSubtitleTitle(baseTitle, baseTitle, path)
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	sizeValue := info.Size()
	fileID := uuid.NewString()
	documentID := uuid.NewString()
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:                fileID,
		LibraryID:         operation.LibraryID,
		Kind:              string(library.FileKindSubtitle),
		Name:              resolveStoredFileName(path, title),
		DisplayName:       strings.TrimSpace(title),
		Metadata:          buildDownloadFileMetadata(operation, baseTitle),
		Storage:           library.FileStorage{Mode: "hybrid", LocalPath: path, DocumentID: documentID},
		Origin:            library.FileOrigin{Kind: "download", OperationID: operation.ID},
		LatestOperationID: operation.ID,
		Media:             &library.MediaInfo{Format: format, SizeBytes: &sizeValue},
		State:             library.FileState{Status: "active"},
		CreatedAt:         &createdAt,
		UpdatedAt:         &createdAt,
	})
	if err != nil {
		return library.LibraryFile{}, err
	}
	document, err := library.NewSubtitleDocument(library.SubtitleDocumentParams{
		ID:              documentID,
		FileID:          fileID,
		LibraryID:       operation.LibraryID,
		Format:          format,
		OriginalContent: string(content),
		WorkingContent:  string(content),
		CreatedAt:       &createdAt,
		UpdatedAt:       &createdAt,
	})
	if err != nil {
		return library.LibraryFile{}, err
	}
	if err := service.files.Save(ctx, fileItem); err != nil {
		return library.LibraryFile{}, err
	}
	if err := service.subtitles.Save(ctx, document); err != nil {
		_ = service.files.Delete(ctx, fileItem.ID)
		return library.LibraryFile{}, err
	}
	return fileItem, nil
}

func (service *LibraryService) createDownloadedBinaryFile(ctx context.Context, operation library.LibraryOperation, kind string, path string, baseTitle string, createdAt time.Time) (library.LibraryFile, error) {
	probe := service.probeLocalMedia(ctx, path)
	media := probe.toMediaInfo()
	title := baseTitle
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if kind == string(library.FileKindThumbnail) {
		title = ydlpinfr.BuildThumbnailTitle(baseTitle, baseTitle, path)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:                uuid.NewString(),
		LibraryID:         operation.LibraryID,
		Kind:              kind,
		Name:              resolveStoredFileName(path, title),
		DisplayName:       strings.TrimSpace(title),
		Metadata:          buildDownloadFileMetadata(operation, baseTitle),
		Storage:           library.FileStorage{Mode: "local_path", LocalPath: path},
		Origin:            library.FileOrigin{Kind: "download", OperationID: operation.ID},
		LatestOperationID: operation.ID,
		Media:             &media,
		State:             library.FileState{Status: "active"},
		CreatedAt:         &createdAt,
		UpdatedAt:         &createdAt,
	})
	if err != nil {
		return library.LibraryFile{}, err
	}
	if err := service.files.Save(ctx, fileItem); err != nil {
		return library.LibraryFile{}, err
	}
	return fileItem, nil
}

func resolveExistingOutputPath(outputPath string, outputPaths []string) string {
	candidate := resolveAssetPathFallback(outputPath)
	if pathExists(candidate) {
		return candidate
	}
	fallback := ""
	for _, path := range outputPaths {
		resolved := resolveAssetPathFallback(path)
		if !pathExists(resolved) {
			continue
		}
		if ydlpinfr.IsSubtitlePath(resolved) {
			if fallback == "" {
				fallback = resolved
			}
			continue
		}
		return resolved
	}
	return fallback
}

func resolveExistingAuxiliaryPath(path string, dirs []string) string {
	trimmed := strings.Trim(strings.TrimSpace(path), "\"")
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		resolved := resolveAssetPathFallback(trimmed)
		if pathExists(resolved) {
			return resolved
		}
		return ""
	}
	for _, dir := range dirs {
		candidate := filepath.Join(strings.TrimSpace(dir), trimmed)
		resolved := resolveAssetPathFallback(candidate)
		if pathExists(resolved) {
			return resolved
		}
	}
	return ""
}

func resolveAssetPathFallback(path string) string {
	trimmed := strings.Trim(strings.TrimSpace(path), "\"")
	if trimmed == "" {
		return ""
	}
	resolved, err := filepath.Abs(trimmed)
	if err == nil {
		return resolved
	}
	return trimmed
}

func pathExists(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}
	_, err := os.Stat(trimmed)
	return err == nil
}

func buildOperationOutputPayload(mainPath string, afterMovePaths []string, outputPaths []string, outputFiles []library.OperationOutputFile, metadataPayload map[string]any, logSnapshot appytdlp.LogSnapshot) string {
	payload := map[string]any{}
	if strings.TrimSpace(mainPath) != "" {
		payload["mainPath"] = strings.TrimSpace(mainPath)
	}
	if len(afterMovePaths) > 0 {
		payload["afterMovePaths"] = dedupePaths(afterMovePaths)
	}
	if len(outputPaths) > 0 {
		payload["outputPaths"] = dedupePaths(outputPaths)
	}
	if len(outputFiles) > 0 {
		items := make([]map[string]any, 0, len(outputFiles))
		for _, outputFile := range outputFiles {
			item := map[string]any{"fileId": outputFile.FileID, "kind": outputFile.Kind, "deleted": outputFile.Deleted}
			if strings.TrimSpace(outputFile.Format) != "" {
				item["format"] = outputFile.Format
			}
			if outputFile.SizeBytes != nil {
				item["sizeBytes"] = *outputFile.SizeBytes
			}
			items = append(items, item)
		}
		payload["outputFiles"] = items
	}
	if metadataPayload != nil {
		payload["metadata"] = metadataPayload
	}
	if logSnapshot.Path != "" {
		payload["log"] = map[string]any{"path": logSnapshot.Path, "sizeBytes": logSnapshot.SizeBytes, "lineCount": logSnapshot.LineCount}
	}
	if len(payload) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func (service *LibraryService) applyYTDLPMetadata(operation *library.LibraryOperation, request *dto.CreateYTDLPJobRequest, payload map[string]any) bool {
	if operation == nil || request == nil || payload == nil {
		return false
	}
	info := extractMetadataInfoFromPayload(payload)
	if len(info) == 0 {
		return false
	}
	changed := false
	if title := strings.TrimSpace(getString(info, "title", "fulltitle")); title != "" {
		if operation.DisplayName != title {
			operation.DisplayName = title
			changed = true
		}
		if strings.TrimSpace(request.Title) == "" || strings.TrimSpace(request.Title) != title {
			request.Title = title
			changed = true
		}
	}
	if platform := strings.TrimSpace(getString(info, "extractor", "extractor_key")); platform != "" {
		if operation.Meta.Platform != platform {
			operation.Meta.Platform = platform
			changed = true
		}
	}
	if uploader := strings.TrimSpace(getString(info, "uploader", "channel", "creator", "artist")); uploader != "" {
		if operation.Meta.Uploader != uploader {
			operation.Meta.Uploader = uploader
			changed = true
		}
	}
	if thumbnailURL := strings.TrimSpace(resolveYTDLPThumbnail(info)); thumbnailURL != "" && strings.TrimSpace(request.ThumbnailURL) == "" {
		request.ThumbnailURL = thumbnailURL
		changed = true
	}
	if publishTime := resolveMetadataPublishTime(info); publishTime != "" {
		if operation.Meta.PublishTime != publishTime {
			operation.Meta.PublishTime = publishTime
			changed = true
		}
	}
	if webpageURL := strings.TrimSpace(getString(info, "webpage_url", "original_url")); webpageURL != "" {
		if domain := extractRegistrableDomain(webpageURL); domain != "" {
			if operation.SourceDomain != domain {
				operation.SourceDomain = domain
				changed = true
			}
		}
	}
	if !changed {
		return false
	}
	if encoded, err := json.Marshal(request); err == nil {
		operation.InputJSON = string(encoded)
	}
	return true
}

func (service *LibraryService) applyYTDLPMetadataToOperationAndHistory(
	ctx context.Context,
	operation *library.LibraryOperation,
	history *library.HistoryRecord,
	request *dto.CreateYTDLPJobRequest,
	payload map[string]any,
) bool {
	if operation == nil || history == nil || request == nil {
		return false
	}
	if !service.applyYTDLPMetadata(operation, request, payload) {
		return false
	}
	history.DisplayName = operation.DisplayName
	history.UpdatedAt = service.now()
	if err := service.persistOperationAndHistory(ctx, operation, history); err != nil {
		return false
	}
	return true
}

func extractMetadataInfoFromPayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	raw, ok := payload["info"]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]any:
		return typed
	case []any:
		for _, entry := range typed {
			if info, ok := entry.(map[string]any); ok {
				return info
			}
		}
	}
	return nil
}

func resolveMetadataPublishTime(info map[string]any) string {
	if len(info) == 0 {
		return ""
	}
	if timestamp, err := getInt64(info, "timestamp"); err == nil && timestamp > 0 {
		return time.Unix(timestamp, 0).UTC().Format(time.RFC3339)
	}
	uploadDate := strings.TrimSpace(getString(info, "upload_date"))
	if len(uploadDate) == 8 {
		if parsed, err := time.Parse("20060102", uploadDate); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func dedupePaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		resolved := resolveAssetPathFallback(path)
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

func parseJSONMap(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}
	return payload
}
