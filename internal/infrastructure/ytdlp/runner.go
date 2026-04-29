package ytdlp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	defaultProgressPrefix = "progress:"
)

func Run(options RunOptions) (RunResult, error) {
	if options.Command == nil {
		return RunResult{}, fmt.Errorf("yt-dlp command is required")
	}
	progressPrefix := strings.TrimSpace(options.ProgressPrefix)
	if progressPrefix == "" {
		progressPrefix = defaultProgressPrefix
	}
	outputCollector := newOutputCollector(4000)
	warningCollector := newOutputCollector(2000)
	stderrCollector := newOutputCollector(2000)

	var outputPathsMu sync.Mutex
	outputPaths := make([]string, 0, 8)
	outputPathSeen := map[string]struct{}{}

	var metadataMu sync.Mutex
	metadataList := make([]map[string]any, 0, 2)
	metadataSeen := map[string]struct{}{}

	var logMu sync.Mutex
	logEntries := make([]LogEntry, 0, 128)

	var afterMoveMu sync.Mutex
	afterMovePaths := make([]string, 0, 4)
	afterMoveSeen := map[string]struct{}{}

	var subtitleLogMu sync.Mutex
	subtitleLogPaths := make([]string, 0, 4)
	subtitleLogSeen := map[string]struct{}{}

	recordOutputPath := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" || isPlaceholderDetail(trimmed) {
			return
		}
		shouldPublish := false
		outputPathsMu.Lock()
		if _, ok := outputPathSeen[trimmed]; !ok {
			outputPathSeen[trimmed] = struct{}{}
			outputPaths = append(outputPaths, trimmed)
			shouldPublish = true
		}
		outputPathsMu.Unlock()
		if shouldPublish && options.OutputPath != nil {
			options.OutputPath(trimmed)
		}
	}
	recordAfterMovePath := func(path string) {
		trimmed := strings.Trim(strings.TrimSpace(path), "\"")
		if trimmed == "" || isPlaceholderDetail(trimmed) {
			return
		}
		afterMoveMu.Lock()
		defer afterMoveMu.Unlock()
		if _, ok := afterMoveSeen[trimmed]; ok {
			return
		}
		afterMoveSeen[trimmed] = struct{}{}
		afterMovePaths = append(afterMovePaths, trimmed)
		recordOutputPath(trimmed)
	}
	recordSubtitleLog := func(path string) {
		trimmed := strings.Trim(strings.TrimSpace(path), "\"")
		if trimmed == "" || isPlaceholderDetail(trimmed) {
			return
		}
		subtitleLogMu.Lock()
		defer subtitleLogMu.Unlock()
		if _, ok := subtitleLogSeen[trimmed]; ok {
			return
		}
		subtitleLogSeen[trimmed] = struct{}{}
		subtitleLogPaths = append(subtitleLogPaths, trimmed)
	}
	recordMetadata := func(info map[string]any) {
		if len(info) == 0 {
			return
		}
		items := collectMetadata(info)
		if len(items) == 0 {
			return
		}
		metadataMu.Lock()
		defer metadataMu.Unlock()
		for _, item := range items {
			payload := string(mustJSON(item))
			if _, ok := metadataSeen[payload]; ok {
				continue
			}
			metadataSeen[payload] = struct{}{}
			metadataList = append(metadataList, item)
		}
	}
	recordLogEntry := func(entry LogEntry) {
		logMu.Lock()
		logEntries = append(logEntries, entry)
		logMu.Unlock()
	}
	considerOutputPath := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		if !filepath.IsAbs(path) {
			return
		}
		recordOutputPath(path)
	}
	maybeUpdateOutputPath := func(trimmed string) {
		if trimmed == "" || isProgressLine(trimmed, progressPrefix) {
			return
		}
		if filepath.IsAbs(trimmed) {
			considerOutputPath(trimmed)
			return
		}
		if extracted := extractOutputPathFromLine(trimmed); extracted != "" {
			considerOutputPath(extracted)
		}
	}
	handleEntry := func(entry LogEntry) {
		trimmed := strings.TrimSpace(entry.Line)
		if trimmed == "" {
			return
		}
		if options.LogLine != nil {
			options.LogLine(entry.Pipe, trimmed)
		}
		if isProgressLine(trimmed, progressPrefix) {
			if options.Progress != nil {
				options.Progress.HandleLine(trimmed)
			}
			return
		}
		if entry.Pipe == "stderr" {
			stderrCollector.Append(trimmed)
		}
		if info, ok := parseJSONLine(trimmed); ok {
			recordMetadata(info)
			collectOutputPaths(info, recordOutputPath, recordSubtitleLog)
		}
		if entry.Pipe == "stdout" {
			if options.PrintFilePath == "" && filepath.IsAbs(trimmed) {
				recordAfterMovePath(trimmed)
			}
		}
		if subtitlePath := extractSubtitlePathFromLine(trimmed); subtitlePath != "" {
			recordSubtitleLog(subtitlePath)
		}
		maybeUpdateOutputPath(trimmed)
		if isWarningLine(trimmed) {
			warningCollector.Append(trimmed)
		}
		outputCollector.Append(trimmed)
		if options.Progress != nil {
			options.Progress.HandleLine(trimmed)
		}
		entry.Line = trimmed
		recordLogEntry(entry)
	}

	stdoutWriter := newTimestampWriter("stdout", handleEntry)
	stderrWriter := newTimestampWriter("stderr", handleEntry)
	options.Command.Stdout = stdoutWriter
	options.Command.Stderr = stderrWriter

	if err := options.Command.Start(); err != nil {
		return RunResult{}, err
	}
	var stop func()
	if options.OnStarted != nil {
		stop = options.OnStarted(options.Command)
	}
	err := options.Command.Wait()
	if stop != nil {
		stop()
	}
	stdoutWriter.Flush()
	stderrWriter.Flush()

	if options.PrintFilePath != "" {
		if data, readErr := os.ReadFile(options.PrintFilePath); readErr == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				recordAfterMovePath(trimmed)
			}
		}
	}

	metadataMu.Lock()
	metadataSnapshot := append([]map[string]any{}, metadataList...)
	metadataMu.Unlock()
	logMu.Lock()
	logSnapshot := append([]LogEntry{}, logEntries...)
	logMu.Unlock()
	outputPathsMu.Lock()
	outputSnapshot := append([]string{}, outputPaths...)
	outputPathsMu.Unlock()
	afterMoveMu.Lock()
	afterMoveSnapshot := append([]string{}, afterMovePaths...)
	afterMoveMu.Unlock()
	subtitleLogMu.Lock()
	subtitleSnapshot := append([]string{}, subtitleLogPaths...)
	subtitleLogMu.Unlock()

	result := RunResult{
		Logs:             logSnapshot,
		Metadata:         metadataSnapshot,
		OutputPaths:      outputSnapshot,
		AfterMovePaths:   afterMoveSnapshot,
		SubtitleLogPaths: subtitleSnapshot,
		Output:           outputCollector.String(),
		Warnings:         warningCollector.String(),
		Stderr:           stderrCollector.String(),
	}
	return result, err
}

func isProgressLine(line string, prefix string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, prefix)
}
