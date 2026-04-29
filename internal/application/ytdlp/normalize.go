package ytdlp

import (
	"strings"

	ydlpinfr "xiadown/internal/infrastructure/ytdlp"
)

func ResolveOutputPath(outputTemplate string, result ydlpinfr.RunResult) (string, string, []string, []string) {
	afterMovePaths := append([]string{}, result.AfterMovePaths...)
	outputPaths := append([]string{}, result.OutputPaths...)

	pickPreferred := func(paths []string) string {
		chosen := ""
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			if chosen == "" {
				chosen = path
				continue
			}
			if ydlpinfr.IsSubtitlePath(chosen) && !ydlpinfr.IsSubtitlePath(path) {
				chosen = path
			}
		}
		return chosen
	}
	resolvedAfterMove := pickPreferred(afterMovePaths)
	fallbackOutputPath := pickPreferred(outputPaths)

	outputPath := strings.TrimSpace(resolvedAfterMove)
	if outputPath == "" {
		outputPath = strings.TrimSpace(fallbackOutputPath)
	}
	if outputPath == "" {
		outputPath = outputTemplate
	}
	return outputPath, resolvedAfterMove, afterMovePaths, outputPaths
}

func BuildMetadataPayload(result ydlpinfr.RunResult, logSnapshot LogSnapshot, env []string, sanitizedArgs []string, origin Origin) map[string]any {
	payload := map[string]any{}
	if len(result.Metadata) == 1 {
		payload["info"] = result.Metadata[0]
	} else if len(result.Metadata) > 1 {
		payload["info"] = result.Metadata
	}
	if logSnapshot.Path != "" {
		payload["log"] = map[string]any{
			"path":      logSnapshot.Path,
			"lineCount": logSnapshot.LineCount,
			"sizeBytes": logSnapshot.SizeBytes,
		}
	}
	if stderrSummary := strings.TrimSpace(result.Stderr); stderrSummary != "" {
		payload["stderr"] = stderrSummary
	}
	if warnings := strings.TrimSpace(result.Warnings); warnings != "" {
		payload["warnings"] = warnings
	}
	if envSnapshot := ydlpinfr.BuildEnvSnapshot(env); envSnapshot != nil {
		payload["env"] = envSnapshot
	}
	if len(sanitizedArgs) > 0 {
		payload["args"] = sanitizedArgs
	}
	if origin.Source != "" || origin.RunID != "" || origin.Caller != "" {
		payload["origin"] = map[string]any{
			"source": origin.Source,
			"runId":  origin.RunID,
			"caller": origin.Caller,
		}
	}
	if len(payload) == 0 {
		return nil
	}
	return payload
}
