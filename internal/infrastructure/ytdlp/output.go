package ytdlp

import (
	"path/filepath"
	"strings"
)

func collectOutputPaths(info map[string]any, recordPath func(string), recordSubtitle func(string)) {
	if info == nil {
		return
	}
	walkInfo(info, func(entry map[string]any) {
		collectPathFields(entry, recordPath)
		if rawDownloads, ok := entry["requested_downloads"].([]any); ok {
			for _, item := range rawDownloads {
				if downloadMap, ok := item.(map[string]any); ok {
					collectPathFields(downloadMap, recordPath)
				}
			}
		}
		if rawFormats, ok := entry["requested_formats"].([]any); ok {
			for _, item := range rawFormats {
				if formatMap, ok := item.(map[string]any); ok {
					collectPathFields(formatMap, recordPath)
				}
			}
		}
		if rawSubtitles, ok := entry["requested_subtitles"].(map[string]any); ok {
			for _, item := range rawSubtitles {
				if subtitleMap, ok := item.(map[string]any); ok {
					collectPathFields(subtitleMap, recordSubtitle)
				}
			}
		}
	})
}

func collectPathFields(values map[string]any, record func(string)) {
	if values == nil || record == nil {
		return
	}
	for _, key := range []string{"filepath", "_filename", "filename"} {
		value := strings.TrimSpace(getString(values, key))
		if value == "" || isPlaceholderDetail(value) {
			continue
		}
		trimmed := strings.Trim(value, "\"")
		if !filepath.IsAbs(trimmed) {
			continue
		}
		record(trimmed)
	}
}

func extractOutputPathFromLine(line string) string {
	lower := strings.ToLower(line)
	if lower == "" {
		return ""
	}
	if index := strings.LastIndex(lower, "destination:"); index >= 0 {
		return strings.Trim(strings.TrimSpace(line[index+len("destination:"):]), "\"")
	}
	if index := strings.LastIndex(lower, " to:"); index >= 0 && strings.Contains(lower, "writing ") {
		return strings.Trim(strings.TrimSpace(line[index+len(" to:"):]), "\"")
	}
	if index := strings.LastIndex(lower, "merging formats into"); index >= 0 {
		rest := strings.TrimSpace(line[index+len("merging formats into"):])
		rest = strings.Trim(rest, "\"")
		return rest
	}
	return ""
}

func extractSubtitlePathFromLine(line string) string {
	lower := strings.ToLower(line)
	if lower == "" {
		return ""
	}
	if !strings.Contains(lower, "writing") || !strings.Contains(lower, "subtitle") {
		return ""
	}
	if index := strings.LastIndex(lower, " to:"); index >= 0 {
		return strings.Trim(strings.TrimSpace(line[index+len(" to:"):]), "\"")
	}
	return ""
}
