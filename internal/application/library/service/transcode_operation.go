package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type registeredLocalOutputParams struct {
	LibraryID     string
	RootFileID    string
	Name          string
	DisplayName   string
	Metadata      library.FileMetadata
	Kind          string
	OperationID   string
	OperationKind string
	OutputPath    string
	SourceMedia   *library.MediaInfo
	OccurredAt    time.Time
}

type ffmpegProgressReporter struct {
	service     *LibraryService
	operation   *library.LibraryOperation
	durationMs  int64
	mu          sync.Mutex
	currentMs   int64
	fps         string
	speed       string
	lastPercent int
	lastCurrent int64
	lastSpeed   string
	lastPublish time.Time
}

type embeddedTranscodeStageResult struct {
	sourceFile  library.LibraryFile
	outputFile  library.LibraryFile
	outputPath  string
	output      library.OperationOutputFile
	finishedAt  time.Time
	displayName string
}

func newFFmpegProgressReporter(service *LibraryService, operation *library.LibraryOperation, durationMs int64) *ffmpegProgressReporter {
	return &ffmpegProgressReporter{
		service:     service,
		operation:   operation,
		durationMs:  durationMs,
		lastPercent: -1,
	}
}

func (reporter *ffmpegProgressReporter) HandleLine(line string) {
	if reporter == nil || reporter.service == nil || reporter.operation == nil || reporter.service.operations == nil {
		return
	}
	key, value, ok := parseFFmpegProgressLine(line)
	if !ok {
		return
	}

	reporter.mu.Lock()
	defer reporter.mu.Unlock()

	if currentMs, ok := parseFFmpegProgressMillis(key, value); ok {
		reporter.currentMs = currentMs
		return
	}
	switch key {
	case "fps":
		reporter.fps = normalizeFFmpegProgressFPS(value)
	case "speed":
		reporter.speed = normalizeFFmpegProgressSpeed(value)
	case "progress":
		reporter.persistLocked(strings.EqualFold(strings.TrimSpace(value), "end"))
	}
}

func (reporter *ffmpegProgressReporter) persistLocked(completed bool) {
	currentMs := reporter.currentMs
	speed := reporter.resolveSpeedLabel()
	if currentMs <= 0 && speed == "" && !completed {
		return
	}

	totalMs := reporter.durationMs
	percent := -1
	if totalMs > 0 {
		if completed {
			currentMs = totalMs
			percent = 100
		} else {
			if currentMs > totalMs {
				currentMs = totalMs
			}
			percent = int((currentMs * 100) / totalMs)
			if percent >= 100 {
				percent = 99
			}
		}
	}

	if !completed &&
		percent == reporter.lastPercent &&
		currentMs == reporter.lastCurrent &&
		speed == reporter.lastSpeed &&
		!reporter.lastPublish.IsZero() &&
		time.Since(reporter.lastPublish) < 500*time.Millisecond {
		return
	}

	now := reporter.service.now().Format(time.RFC3339)
	progress := &library.OperationProgress{
		Stage:     progressText("library.progress.transcoding"),
		Message:   progressText("library.progressDetail.ffmpegRenderingOutput"),
		Speed:     speed,
		UpdatedAt: now,
	}
	if currentMs > 0 {
		value := currentMs
		progress.Current = &value
	}
	if totalMs > 0 {
		value := totalMs
		progress.Total = &value
	}
	if percent >= 0 {
		value := percent
		progress.Percent = &value
	}

	reporter.lastPercent = percent
	reporter.lastCurrent = currentMs
	reporter.lastSpeed = speed
	reporter.lastPublish = time.Now()
	reporter.operation.Progress = progress
	if reporter.operation.Status == library.OperationStatusQueued {
		reporter.operation.Status = library.OperationStatusRunning
	}
	if !reporter.service.operationCanAcceptProgress(context.Background(), reporter.operation.ID) {
		return
	}
	if err := reporter.service.operations.Save(context.Background(), *reporter.operation); err != nil {
		return
	}
	reporter.service.publishOperationUpdate(toOperationDTO(*reporter.operation))
}

func (service *LibraryService) runEmbeddedTranscodeStage(
	ctx context.Context,
	operation *library.LibraryOperation,
	sourceFile library.LibraryFile,
	request dto.CreateTranscodeJobRequest,
) (embeddedTranscodeStageResult, error) {
	result := embeddedTranscodeStageResult{sourceFile: sourceFile}
	if service == nil || operation == nil {
		return result, fmt.Errorf("operation is required")
	}
	if strings.TrimSpace(sourceFile.LibraryID) == "" {
		return result, fmt.Errorf("source file is not attached to a library")
	}
	request = service.enrichTranscodeRequestForSource(ctx, request, sourceFile)

	probe, err := service.probeRequiredMedia(ctx, sourceFile.Storage.LocalPath)
	if err != nil {
		return result, err
	}
	plan, err := service.resolveTranscodePlan(ctx, request, probe)
	if err != nil {
		return result, err
	}
	ffmpegExecPath, err := resolveFFmpegExecPath(ctx, service.tools)
	if err != nil {
		return result, err
	}

	displayName := resolveTranscodeDisplayName(request, sourceFile, plan.preset)
	outputName := resolveTranscodeOutputName(sourceFile.Storage.LocalPath, plan.preset)
	outputPath, err := service.deriveManagedOutputPath(ctx, outputName, plan.request.Format, sourceFile.Storage.LocalPath)
	if err != nil {
		return result, err
	}
	if err := ensureManagedOutputParentDir(outputPath); err != nil {
		return result, err
	}
	ffmpegArgs, err := buildFFmpegTranscodeArgs(plan, sourceFile.Storage.LocalPath, outputPath)
	if err != nil {
		return result, err
	}

	progressTime := service.now()
	operation.Progress = buildOperationProgress(
		progressTime,
		progressText("library.progress.transcoding"),
		0,
		1,
		progressText("library.progressDetail.ffmpegRenderingOutput"),
	)
	if err := service.saveAndPublishOperation(ctx, *operation); err != nil {
		return result, err
	}

	outputText, err := service.runFFmpegCommandWithProgress(
		ctx,
		operation,
		ffmpegExecPath,
		ffmpegArgs,
		filepath.Dir(sourceFile.Storage.LocalPath),
		probe.DurationMs,
	)
	if err != nil {
		message := strings.TrimSpace(outputText)
		if message == "" {
			message = err.Error()
		}
		return result, fmt.Errorf("ffmpeg transcode failed: %s", message)
	}
	if !pathExists(outputPath) {
		return result, fmt.Errorf("ffmpeg produced no output file")
	}

	finishedAt := service.now()
	outputFile, err := service.registerManagedLocalOutputFile(ctx, registeredLocalOutputParams{
		LibraryID:     sourceFile.LibraryID,
		RootFileID:    rootFileID(sourceFile),
		Name:          outputName,
		DisplayName:   displayName,
		Metadata:      buildTranscodeFileMetadata(sourceFile, strings.TrimSpace(request.Title)),
		Kind:          string(library.FileKindTranscode),
		OperationID:   operation.ID,
		OperationKind: "transcode",
		OutputPath:    outputPath,
		SourceMedia:   sourceFile.Media,
		OccurredAt:    finishedAt,
	})
	if err != nil {
		return result, err
	}
	outputFile.LatestOperationID = operation.ID
	outputFile.UpdatedAt = finishedAt
	if err := service.files.Save(ctx, outputFile); err != nil {
		return result, err
	}

	sourceFile.LatestOperationID = operation.ID
	sourceFile.UpdatedAt = finishedAt
	if err := service.files.Save(ctx, sourceFile); err != nil {
		return result, err
	}
	if request.DeleteSourceFileAfterTranscode {
		sourceFile = service.cleanupSourceFileAfterSuccessfulTranscode(ctx, sourceFile, operation.ID)
	}

	result.sourceFile = sourceFile
	result.outputFile = outputFile
	result.outputPath = outputPath
	result.output = library.OperationOutputFile{
		FileID:    outputFile.ID,
		Kind:      string(outputFile.Kind),
		Format:    mediaFormatFromFile(outputFile),
		SizeBytes: mediaSizeFromFile(outputFile),
		IsPrimary: true,
		Deleted:   outputFile.State.Deleted,
	}
	result.finishedAt = finishedAt
	result.displayName = displayName
	return result, nil
}

func (service *LibraryService) runTranscodeOperation(ctx context.Context, operation library.LibraryOperation, request dto.CreateTranscodeJobRequest) {
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

	now := service.now()
	operation.Status = library.OperationStatusRunning
	operation.StartedAt = &now
	operation.FinishedAt = nil
	operation.ErrorCode = ""
	operation.ErrorMessage = ""
	operation.Progress = buildOperationProgress(
		now,
		progressText("library.progress.preparing"),
		0,
		1,
		progressText("library.progressDetail.preparingFfmpegTranscode"),
	)
	operation.OutputJSON = buildTranscodeOperationOutput(request, "running", "")
	if err := service.saveAndPublishOperation(ctx, operation); err != nil {
		return
	}

	sourceFile, err := service.resolveSourceFileForTranscode(ctx, request)
	if err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}
	request = service.enrichTranscodeRequestForSource(ctx, request, sourceFile)
	probe, err := service.probeRequiredMedia(ctx, sourceFile.Storage.LocalPath)
	if err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}
	plan, err := service.resolveTranscodePlan(ctx, request, probe)
	if err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}
	ffmpegExecPath, err := resolveFFmpegExecPath(ctx, service.tools)
	if err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}

	displayName := resolveTranscodeDisplayName(request, sourceFile, plan.preset)
	outputName := resolveTranscodeOutputName(sourceFile.Storage.LocalPath, plan.preset)
	outputPath, err := service.deriveManagedOutputPath(ctx, outputName, plan.request.Format, sourceFile.Storage.LocalPath)
	if err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}
	if err := ensureManagedOutputParentDir(outputPath); err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}

	ffmpegArgs, err := buildFFmpegTranscodeArgs(
		plan,
		sourceFile.Storage.LocalPath,
		outputPath,
	)
	if err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}

	progressTime := service.now()
	operation.Progress = buildOperationProgress(
		progressTime,
		progressText("library.progress.transcoding"),
		0,
		1,
		progressText("library.progressDetail.ffmpegRenderingOutput"),
	)
	operation.OutputJSON = buildTranscodeOperationOutput(request, "running", outputPath)
	if err := service.saveAndPublishOperation(ctx, operation); err != nil {
		return
	}

	outputText, err := service.runFFmpegCommandWithProgress(
		ctx,
		&operation,
		ffmpegExecPath,
		ffmpegArgs,
		filepath.Dir(sourceFile.Storage.LocalPath),
		probe.DurationMs,
	)
	if err != nil {
		message := strings.TrimSpace(outputText)
		if message == "" {
			message = err.Error()
		}
		service.failTranscodeOperation(ctx, operation, request, fmt.Errorf("ffmpeg transcode failed: %s", message))
		return
	}
	if !pathExists(outputPath) {
		service.failTranscodeOperation(ctx, operation, request, fmt.Errorf("ffmpeg produced no output file"))
		return
	}

	finishedAt := service.now()
	outputFile, err := service.registerManagedLocalOutputFile(ctx, registeredLocalOutputParams{
		LibraryID:     sourceFile.LibraryID,
		RootFileID:    rootFileID(sourceFile),
		Name:          outputName,
		DisplayName:   displayName,
		Metadata:      buildTranscodeFileMetadata(sourceFile, strings.TrimSpace(request.Title)),
		Kind:          string(library.FileKindTranscode),
		OperationID:   operation.ID,
		OperationKind: "transcode",
		OutputPath:    outputPath,
		SourceMedia:   sourceFile.Media,
		OccurredAt:    finishedAt,
	})
	if err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}
	outputFile.LatestOperationID = operation.ID
	outputFile.UpdatedAt = finishedAt
	if err := service.files.Save(ctx, outputFile); err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}

	files := []library.LibraryFile{outputFile}
	operationOutputs := []library.OperationOutputFile{{
		FileID:    outputFile.ID,
		Kind:      string(outputFile.Kind),
		Format:    mediaFormatFromFile(outputFile),
		SizeBytes: mediaSizeFromFile(outputFile),
		IsPrimary: true,
		Deleted:   outputFile.State.Deleted,
	}}

	sourceFile.LatestOperationID = operation.ID
	sourceFile.UpdatedAt = finishedAt
	if err := service.files.Save(ctx, sourceFile); err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}

	operation.Status = library.OperationStatusSucceeded
	operation.OutputFiles = operationOutputs
	operation.Metrics = buildOperationMetricsForOperation(files, operation.StartedAt, &finishedAt)
	operation.FinishedAt = &finishedAt
	operation.ErrorCode = ""
	operation.ErrorMessage = ""
	operation.Progress = buildOperationProgress(
		finishedAt,
		progressText("library.status.succeeded"),
		1,
		1,
		progressText("library.progressDetail.ffmpegTranscodeCompleted"),
	)
	operation.OutputJSON = buildTranscodeOperationOutput(request, "completed", outputPath)
	if err := service.operations.Save(ctx, operation); err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}

	history, err := library.NewHistoryRecord(library.HistoryRecordParams{
		ID:          uuid.NewString(),
		LibraryID:   sourceFile.LibraryID,
		Category:    "operation",
		Action:      "transcode",
		DisplayName: displayName,
		Status:      string(operation.Status),
		Source:      library.HistoryRecordSource{Kind: resolveHistorySourceKind(request.Source), RunID: strings.TrimSpace(request.RunID)},
		Refs: library.HistoryRecordRefs{
			OperationID: operation.ID,
			FileIDs:     extractLibraryFileIDs(files),
		},
		OccurredAt: &finishedAt,
		CreatedAt:  &finishedAt,
		UpdatedAt:  &finishedAt,
	})
	if err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}
	history.Files = operation.OutputFiles
	history.Metrics = operation.Metrics
	history.OperationMeta = &library.OperationRecordMeta{Kind: "transcode"}
	if err := service.histories.Save(ctx, history); err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}
	if err := service.touchLibrary(ctx, sourceFile.LibraryID, finishedAt); err != nil {
		service.failTranscodeOperation(ctx, operation, request, err)
		return
	}
	if request.DeleteSourceFileAfterTranscode {
		sourceFile = service.cleanupSourceFileAfterSuccessfulTranscode(ctx, sourceFile, operation.ID)
	}

	service.publishOperationUpdate(toOperationDTO(operation))
	service.publishHistoryUpdate(toHistoryDTO(history))
	service.publishFileUpdate(service.mustBuildFileDTO(ctx, sourceFile))
	for _, file := range files {
		service.syncDreamFMLocalTrackFromFile(ctx, file, nil)
		service.publishFileUpdate(service.mustBuildFileDTO(ctx, file))
	}
}

func (service *LibraryService) failTranscodeOperation(ctx context.Context, operation library.LibraryOperation, request dto.CreateTranscodeJobRequest, runErr error) {
	if service == nil || service.operations == nil {
		return
	}
	currentOperation := operation
	if item, err := service.operations.Get(ctx, operation.ID); err == nil {
		currentOperation = item
	}
	if errors.Is(runErr, context.Canceled) {
		currentOperation.Status = library.OperationStatusCanceled
		currentOperation.ErrorCode = "transcode_canceled"
		currentOperation.ErrorMessage = ""
	} else {
		currentOperation.Status = library.OperationStatusFailed
		currentOperation.ErrorCode = "transcode_failed"
		currentOperation.ErrorMessage = strings.TrimSpace(runErr.Error())
	}
	now := service.now()
	currentOperation.FinishedAt = &now
	currentOperation.Progress = buildOperationProgress(
		now,
		progressText(progressStageLocaleKey(string(currentOperation.Status))),
		0,
		1,
		terminalProgressMessage(currentOperation.Kind, currentOperation.Status),
	)
	currentOperation.OutputJSON = buildTranscodeOperationOutput(
		request,
		strings.TrimSpace(string(currentOperation.Status)),
		extractTranscodeOutputPath(currentOperation.OutputJSON),
	)
	if err := service.operations.Save(ctx, currentOperation); err != nil {
		return
	}
	service.publishOperationUpdate(toOperationDTO(currentOperation))
}

func (service *LibraryService) cleanupSourceFileAfterSuccessfulTranscode(
	ctx context.Context,
	sourceFile library.LibraryFile,
	transcodeOperationID string,
) library.LibraryFile {
	if service == nil || service.files == nil {
		return sourceFile
	}
	if sourceFile.State.Deleted {
		return sourceFile
	}

	sourceFile.LatestOperationID = strings.TrimSpace(transcodeOperationID)
	if err := service.markLibraryFileDeleted(ctx, sourceFile, true); err != nil {
		return sourceFile
	}

	if updated, err := service.files.Get(ctx, sourceFile.ID); err == nil {
		sourceFile = updated
	}
	service.syncOperationAndHistoryForDeletedOutput(ctx, sourceFile.LibraryID, sourceFile.Origin.OperationID, sourceFile.ID)
	return sourceFile
}

func (service *LibraryService) syncOperationAndHistoryForDeletedOutput(
	ctx context.Context,
	libraryID string,
	operationID string,
	fileID string,
) {
	if service == nil {
		return
	}
	trimmedOperationID := strings.TrimSpace(operationID)
	trimmedFileID := strings.TrimSpace(fileID)
	if trimmedOperationID == "" || trimmedFileID == "" {
		return
	}

	if service.operations != nil {
		operation, err := service.operations.Get(ctx, trimmedOperationID)
		if err == nil {
			updatedOutputFiles, changed := markOperationOutputFileDeleted(operation.OutputFiles, trimmedFileID)
			if changed {
				operation.OutputFiles = updatedOutputFiles
				operation.Metrics = service.rebuildOperationMetricsFromOutputs(ctx, updatedOutputFiles, operation.StartedAt, operation.FinishedAt)
				if saveErr := service.operations.Save(ctx, operation); saveErr == nil {
					service.publishOperationUpdate(toOperationDTO(operation))
				}
			}
		}
	}

	if service.histories != nil {
		histories, err := service.histories.ListByLibraryID(ctx, strings.TrimSpace(libraryID))
		if err != nil {
			return
		}
		for _, history := range histories {
			if history.Refs.OperationID != trimmedOperationID {
				continue
			}
			updatedFiles, changed := markOperationOutputFileDeleted(history.Files, trimmedFileID)
			if !changed {
				return
			}
			history.Files = updatedFiles
			durationMs := history.Metrics.DurationMs
			history.Metrics = service.rebuildOperationMetricsFromOutputs(ctx, updatedFiles, nil, nil)
			history.Metrics.DurationMs = durationMs
			history.UpdatedAt = service.now()
			if saveErr := service.histories.Save(ctx, history); saveErr == nil {
				service.publishHistoryUpdate(toHistoryDTO(history))
			}
			return
		}
	}
}

func markOperationOutputFileDeleted(items []library.OperationOutputFile, fileID string) ([]library.OperationOutputFile, bool) {
	trimmedFileID := strings.TrimSpace(fileID)
	if trimmedFileID == "" || len(items) == 0 {
		return items, false
	}
	updated := append([]library.OperationOutputFile(nil), items...)
	changed := false
	for index := range updated {
		if updated[index].FileID != trimmedFileID || updated[index].Deleted {
			continue
		}
		updated[index].Deleted = true
		changed = true
	}
	return updated, changed
}

func (service *LibraryService) rebuildOperationMetricsFromOutputs(
	ctx context.Context,
	outputFiles []library.OperationOutputFile,
	startedAt *time.Time,
	finishedAt *time.Time,
) library.OperationMetrics {
	files := make([]library.LibraryFile, 0, len(outputFiles))
	if service != nil && service.files != nil {
		for _, output := range outputFiles {
			fileID := strings.TrimSpace(output.FileID)
			if fileID == "" {
				continue
			}
			fileItem, err := service.files.Get(ctx, fileID)
			if err != nil {
				continue
			}
			files = append(files, fileItem)
		}
	}
	return buildOperationMetricsForOperation(files, startedAt, finishedAt)
}

func (service *LibraryService) registerManagedLocalOutputFile(ctx context.Context, params registeredLocalOutputParams) (library.LibraryFile, error) {
	info, err := os.Stat(params.OutputPath)
	if err != nil {
		return library.LibraryFile{}, err
	}
	now := params.OccurredAt
	if now.IsZero() {
		now = service.now()
	}
	probedProbe, err := service.probeRequiredMedia(ctx, params.OutputPath)
	if err != nil {
		return library.LibraryFile{}, err
	}
	probedMedia := probedProbe.toMediaInfo()
	media := mergeMediaInfo(cloneMediaInfo(params.SourceMedia), &probedMedia)
	if media == nil {
		media = &library.MediaInfo{}
	}
	if strings.TrimSpace(media.Format) == "" {
		media.Format = normalizeTranscodeFormat(filepath.Ext(params.OutputPath))
	}
	sizeValue := info.Size()
	media.SizeBytes = &sizeValue
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:          uuid.NewString(),
		LibraryID:   params.LibraryID,
		Kind:        params.Kind,
		Name:        params.Name,
		DisplayName: params.DisplayName,
		Metadata:    params.Metadata,
		Storage:     library.FileStorage{Mode: "local_path", LocalPath: params.OutputPath},
		Origin:      library.FileOrigin{Kind: params.OperationKind, OperationID: params.OperationID},
		Lineage:     library.FileLineage{RootFileID: strings.TrimSpace(params.RootFileID)},
		Media:       media,
		State:       library.FileState{Status: "active"},
		CreatedAt:   &now,
		UpdatedAt:   &now,
	})
	if err != nil {
		return library.LibraryFile{}, err
	}
	if err := service.files.Save(ctx, fileItem); err != nil {
		return library.LibraryFile{}, err
	}
	service.syncDreamFMLocalTrackFromFile(ctx, fileItem, &probedProbe)
	return fileItem, nil
}

func (service *LibraryService) runFFmpegCommandWithProgress(
	ctx context.Context,
	operation *library.LibraryOperation,
	execPath string,
	args []string,
	workDir string,
	durationMs int64,
) (string, error) {
	commandArgs := withFFmpegProgressArgs(args)
	command := exec.CommandContext(ctx, execPath, commandArgs...)
	command.Dir = strings.TrimSpace(workDir)
	configureProcessGroup(command)

	stdout, err := command.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := command.Start(); err != nil {
		return "", err
	}

	reporter := newFFmpegProgressReporter(service, operation, durationMs)
	var stderrBuilder strings.Builder
	var stdoutErr error
	var stderrErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		stdoutErr = scanFFmpegOutput(stdout, nil, reporter.HandleLine)
	}()
	go func() {
		defer wg.Done()
		stderrErr = scanFFmpegOutput(stderr, &stderrBuilder, nil)
	}()

	wg.Wait()
	waitErr := command.Wait()

	if stdoutErr != nil {
		return strings.TrimSpace(stderrBuilder.String()), stdoutErr
	}
	if stderrErr != nil {
		return strings.TrimSpace(stderrBuilder.String()), stderrErr
	}
	return strings.TrimSpace(stderrBuilder.String()), waitErr
}

func ensureManagedOutputParentDir(outputPath string) error {
	parentDir := strings.TrimSpace(filepath.Dir(outputPath))
	if parentDir == "" || parentDir == "." {
		return nil
	}
	return os.MkdirAll(parentDir, 0o755)
}

func withFFmpegProgressArgs(args []string) []string {
	if len(args) == 0 {
		return []string{"-nostdin", "-progress", "pipe:1", "-nostats"}
	}
	for index := 0; index < len(args)-1; index++ {
		if args[index] == "-progress" {
			return append([]string{}, args...)
		}
	}
	result := append([]string{}, args[:len(args)-1]...)
	result = append(result, "-nostdin", "-progress", "pipe:1", "-nostats", args[len(args)-1])
	return result
}

func scanFFmpegOutput(reader io.Reader, builder *strings.Builder, handler func(string)) error {
	if reader == nil {
		return nil
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if builder != nil && strings.TrimSpace(line) != "" {
			if builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(line)
		}
		if handler != nil {
			handler(line)
		}
	}
	return scanner.Err()
}

func parseFFmpegProgressLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", "", false
	}
	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(parts[1]), true
}

func parseFFmpegProgressMillis(key string, value string) (int64, bool) {
	switch strings.TrimSpace(key) {
	case "out_time":
		return parseTimestampToMilliseconds(value)
	case "out_time_ms", "out_time_us":
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || parsed < 0 {
			return 0, false
		}
		return parsed / 1000, true
	default:
		return 0, false
	}
}

func parseTimestampToMilliseconds(value string) (int64, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) != 3 {
		return 0, false
	}
	hours, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || hours < 0 {
		return 0, false
	}
	minutes, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || minutes < 0 {
		return 0, false
	}
	secondsValue, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err != nil || secondsValue < 0 {
		return 0, false
	}
	totalMillis := (hours*3600+minutes*60)*1000 + int64(secondsValue*1000)
	if totalMillis < 0 {
		return 0, false
	}
	return totalMillis, true
}

func normalizeFFmpegProgressSpeed(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "N/A") {
		return ""
	}
	return trimmed
}

func normalizeFFmpegProgressFPS(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "N/A") {
		return ""
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || parsed <= 0 {
		return ""
	}
	if parsed >= 100 {
		return strconv.FormatFloat(parsed, 'f', 0, 64) + " fps"
	}
	return strconv.FormatFloat(parsed, 'f', 1, 64) + " fps"
}

func (reporter *ffmpegProgressReporter) resolveSpeedLabel() string {
	if reporter == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(reporter.fps); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(reporter.speed)
}

func buildFFmpegTranscodeArgs(
	plan transcodePlan,
	inputPath string,
	outputPath string,
) ([]string, error) {
	outputType := plan.outputType
	container := normalizeContainer(plan.request.Format)
	audioOutput := outputType == library.TranscodeOutputAudio || isAudioContainer(container)
	coverInputIndex := -1

	args := []string{"-y", "-i", inputPath}
	if audioOutput && ffmpegAudioContainerSupportsCover(container) {
		if coverPath := strings.TrimSpace(plan.request.CoverPath); coverPath != "" {
			coverInputIndex = 1
			args = append(args, "-i", coverPath)
		}
	}

	subtitleInputIndices := make([]int, 0, len(plan.request.SubtitlePaths))
	sourceSubtitleMaps := []string{}
	subtitleCodec := ""
	if !audioOutput {
		sourceSubtitleMaps = ffmpegSourceSubtitleMapSpecs(container, plan.sourceProbe)
		sidecarSubtitlePaths := ffmpegSidecarSubtitlePaths(container, plan.request.SubtitlePaths)
		if len(sourceSubtitleMaps) > 0 || len(sidecarSubtitlePaths) > 0 {
			subtitleCodec = ffmpegSubtitleCodecForContainer(container)
		}
		nextInputIndex := 1
		if coverInputIndex >= 0 {
			nextInputIndex++
		}
		for _, subtitlePath := range sidecarSubtitlePaths {
			subtitleInputIndices = append(subtitleInputIndices, nextInputIndex)
			args = append(args, "-i", subtitlePath)
			nextInputIndex++
		}
	}

	filters := make([]string, 0, 1)
	scaleFilter, err := buildFFmpegScaleFilter(plan.request)
	if err != nil {
		return nil, err
	}
	if scaleFilter != "" {
		filters = append(filters, scaleFilter)
	}
	if len(filters) > 0 {
		args = append(args, "-vf", strings.Join(filters, ","))
	}

	if audioOutput {
		args = append(args, "-map", "0:a:0?", "-vn")
		if coverInputIndex >= 0 {
			args = args[:len(args)-1]
			args = append(args, "-map", fmt.Sprintf("%d:v:0?", coverInputIndex))
		} else if ffmpegCanPreserveSourceAttachedPicture(plan) {
			args = args[:len(args)-1]
			args = append(args, "-map", "0:v:0?")
		}
	} else {
		videoCodec := normalizeVideoCodecName(plan.request.VideoCodec)
		if videoCodec == "" {
			videoCodec = "h264"
		}
		if len(filters) > 0 && videoCodec == "copy" {
			videoCodec = "h264"
		}
		args = append(args, "-map", "0:v:0?", "-map", "0:a:0?")
		for _, mapSpec := range sourceSubtitleMaps {
			args = append(args, "-map", mapSpec)
		}
		for _, inputIndex := range subtitleInputIndices {
			args = append(args, "-map", fmt.Sprintf("%d:0?", inputIndex))
		}
		selectedVideoCodec := ffmpegVideoCodec(videoCodec)
		args = append(args, "-c:v", selectedVideoCodec)
		isHardwareVideoCodec := ffmpegIsHardwareVideoCodec(selectedVideoCodec)
		if videoCodec != "copy" {
			args = appendVideoCodecQualityArgs(args, videoCodec, plan.request, isHardwareVideoCodec)
			qualityMode := strings.ToLower(strings.TrimSpace(plan.request.QualityMode))
			switch qualityMode {
			case "bitrate":
				if plan.request.BitrateKbps > 0 {
					args = append(args, "-b:v", fmt.Sprintf("%dk", plan.request.BitrateKbps))
				}
			default:
				if plan.request.CRF > 0 && !isHardwareVideoCodec {
					if videoCodec == "vp9" {
						args = append(args, "-b:v", "0")
					}
					args = append(args, "-crf", strconv.Itoa(plan.request.CRF))
				}
			}
		}
	}

	audioCodec := normalizeAudioCodecName(plan.request.AudioCodec)
	if audioCodec == "" {
		if outputType == library.TranscodeOutputAudio {
			audioCodec = firstNonEmpty(defaultAudioCodecForContainer(container), "mp3")
		} else {
			audioCodec = firstNonEmpty(defaultAudioCodecForContainer(container), "aac")
		}
	}
	if audioCodec != "" {
		args = append(args, "-c:a", ffmpegAudioCodec(audioCodec))
		if ffmpegAudioCodecSupportsBitrate(audioCodec) && plan.request.AudioBitrateKbps > 0 {
			args = append(args, "-b:a", fmt.Sprintf("%dk", plan.request.AudioBitrateKbps))
		}
	}
	if audioOutput && (coverInputIndex >= 0 || ffmpegCanPreserveSourceAttachedPicture(plan)) {
		args = append(
			args,
			"-c:v",
			ffmpegAudioCoverCodec(container),
			"-disposition:v:0",
			"attached_pic",
			"-metadata:s:v",
			"title=Album cover",
			"-metadata:s:v",
			"comment=Cover (front)",
		)
	}
	if subtitleCodec != "" {
		args = append(args, "-c:s", subtitleCodec)
	}
	args = append(args, "-map_metadata", "0")
	if ffmpegContainerSupportsChapters(container) {
		args = append(args, "-map_chapters", "0")
	}
	args = appendFFmpegTranscodeMetadataArgs(args, plan.request)
	if container == "mp3" {
		args = append(args, "-id3v2_version", "3")
	}
	if container == "mp4" || container == "mov" {
		args = append(args, "-movflags", "+faststart")
	}

	args = append(args, outputPath)
	return args, nil
}

func ffmpegAudioContainerSupportsCover(container string) bool {
	switch normalizeContainer(container) {
	case "mp3", "m4a", "flac":
		return true
	default:
		return false
	}
}

func ffmpegAudioCoverCodec(container string) string {
	switch normalizeContainer(container) {
	case "mp3", "m4a", "flac":
		return "mjpeg"
	default:
		return "copy"
	}
}

func ffmpegCanPreserveSourceAttachedPicture(plan transcodePlan) bool {
	container := normalizeContainer(plan.request.Format)
	return ffmpegAudioContainerSupportsCover(container) &&
		plan.sourceProbe.AttachedPicCount > 0 &&
		!plan.sourceProbe.HasVideo
}

func ffmpegContainerSupportsChapters(container string) bool {
	switch normalizeContainer(container) {
	case "mp4", "mov", "mkv", "mp3", "m4a":
		return true
	default:
		return false
	}
}

func appendFFmpegTranscodeMetadataArgs(args []string, request dto.CreateTranscodeJobRequest) []string {
	if title := strings.TrimSpace(request.Title); title != "" {
		args = append(args, "-metadata", "title="+title)
	}
	if author := strings.TrimSpace(request.Author); author != "" {
		args = append(args, "-metadata", "artist="+author, "-metadata", "album_artist="+author)
	}
	if extractor := strings.TrimSpace(request.Extractor); extractor != "" {
		args = append(args, "-metadata", "comment=Source: "+extractor)
	}
	return args
}

func ffmpegSubtitleCodecForContainer(container string) string {
	switch normalizeContainer(container) {
	case "mkv":
		return "copy"
	case "mp4", "mov":
		return "mov_text"
	case "webm":
		return "webvtt"
	default:
		return ""
	}
}

func ffmpegSourceSubtitleMapSpecs(container string, probe mediaProbe) []string {
	subtitleCodec := ffmpegSubtitleCodecForContainer(container)
	if subtitleCodec == "" || len(probe.SubtitleStreams) == 0 {
		return nil
	}
	result := make([]string, 0, len(probe.SubtitleStreams))
	for _, stream := range probe.SubtitleStreams {
		if !ffmpegSubtitleCodecCanMux(container, stream.Codec) {
			continue
		}
		if stream.Index >= 0 {
			result = append(result, fmt.Sprintf("0:%d?", stream.Index))
			continue
		}
		result = append(result, "0:s?")
	}
	return result
}

func ffmpegSidecarSubtitlePaths(container string, paths []string) []string {
	if ffmpegSubtitleCodecForContainer(container) == "" {
		return nil
	}
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved := normalizeExistingTranscodeSubtitlePath(path)
		if resolved == "" {
			continue
		}
		if !ffmpegSubtitleCodecCanMux(container, normalizeFileExtension(resolved)) {
			continue
		}
		result = append(result, resolved)
	}
	return dedupePaths(result)
}

func ffmpegSubtitleCodecCanMux(container string, codec string) bool {
	normalizedContainer := normalizeContainer(container)
	normalizedCodec := normalizeSubtitleFormat(codec)
	if normalizedCodec == "" {
		normalizedCodec = normalizeTranscodeFormat(codec)
	}
	switch normalizedContainer {
	case "mkv":
		return true
	case "mp4", "mov":
		switch normalizedCodec {
		case "srt", "subrip", "vtt", "webvtt", "mov_text", "text":
			return true
		default:
			return false
		}
	case "webm":
		switch normalizedCodec {
		case "vtt", "webvtt":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func appendVideoCodecQualityArgs(
	args []string,
	videoCodec string,
	request dto.CreateTranscodeJobRequest,
	isHardwareVideoCodec bool,
) []string {
	if isHardwareVideoCodec {
		return args
	}
	switch normalizeVideoCodecName(videoCodec) {
	case "vp9":
		// Official libvpx docs recommend `deadline=good`; `best` is slower and may produce worse quality.
		args = append(args, "-deadline", "good", "-cpu-used", "0", "-row-mt", "1")
	default:
		if preset := strings.TrimSpace(request.Preset); preset != "" {
			args = append(args, "-preset", preset)
		}
	}
	return args
}

func buildFFmpegScaleFilter(request dto.CreateTranscodeJobRequest) (string, error) {
	scale := strings.ToLower(strings.TrimSpace(request.Scale))
	if scale == "" || scale == "original" {
		return "", nil
	}
	width := request.Width
	height := request.Height
	if scale != "custom" {
		target, ok := scaleTargets[scale]
		if !ok {
			return "", fmt.Errorf("unsupported scale preset")
		}
		width = target[0]
		height = target[1]
	}
	if width <= 0 || height <= 0 {
		return "", fmt.Errorf("invalid scale dimensions")
	}
	return fmt.Sprintf(
		"scale=w=%d:h=%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
		width,
		height,
		width,
		height,
	), nil
}

func ffmpegVideoCodec(codec string) string {
	switch normalizeVideoCodecName(codec) {
	case "h264":
		return "libx264"
	case "h265":
		return "libx265"
	case "vp9":
		return "libvpx-vp9"
	default:
		return firstNonEmpty(codec, "libx264")
	}
}

func ffmpegIsHardwareVideoCodec(codec string) bool {
	normalized := strings.ToLower(strings.TrimSpace(codec))
	return strings.HasSuffix(normalized, "_nvenc") ||
		strings.HasSuffix(normalized, "_videotoolbox") ||
		strings.HasSuffix(normalized, "_qsv") ||
		strings.HasSuffix(normalized, "_vaapi") ||
		strings.HasSuffix(normalized, "_amf") ||
		strings.HasSuffix(normalized, "_v4l2m2m")
}

func ffmpegAudioCodec(codec string) string {
	switch normalizeAudioCodecName(codec) {
	case "aac":
		return "aac"
	case "mp3":
		return "libmp3lame"
	case "opus":
		return "libopus"
	case "flac":
		return "flac"
	case "pcm":
		return "pcm_s16le"
	default:
		return firstNonEmpty(codec, "aac")
	}
}

func ffmpegAudioCodecSupportsBitrate(codec string) bool {
	switch normalizeAudioCodecName(codec) {
	case "aac", "mp3", "opus":
		return true
	default:
		return false
	}
}

func buildTranscodeOperationOutput(request dto.CreateTranscodeJobRequest, status string, outputPath string) string {
	payload := map[string]any{
		"status":                         strings.TrimSpace(status),
		"deleteSourceFileAfterTranscode": request.DeleteSourceFileAfterTranscode,
		"outputPath":                     strings.TrimSpace(outputPath),
	}
	return marshalJSON(payload)
}

func extractTranscodeOutputPath(outputJSON string) string {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(outputJSON)), &payload); err != nil {
		return ""
	}
	if value, ok := payload["outputPath"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func extractLibraryFileIDs(files []library.LibraryFile) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		if trimmed := strings.TrimSpace(file.ID); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
