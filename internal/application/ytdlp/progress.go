package ytdlp

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	ytdlpPercentRe = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)%`)
	ytdlpTotalRe   = regexp.MustCompile(`(?i)\bof\s+([0-9.]+)\s*([kmgtp]?i?b)\b`)
	ytdlpSpeedRe   = regexp.MustCompile(`(?i)\bat\s+([0-9.]+)\s*([kmgtp]?i?b/s)\b`)
)

type JSONProgress struct {
	Status          string
	DownloadedBytes *int64
	TotalBytes      *int64
	TotalEstimate   *int64
	Speed           string
	Filename        string
	TmpFilename     string
	FragmentIndex   *int
	FragmentCount   *int
}

func ParseProgressJSON(line string, prefix string) (JSONProgress, bool) {
	if line == "" {
		return JSONProgress{}, false
	}
	index := strings.Index(line, prefix)
	if index < 0 {
		return JSONProgress{}, false
	}
	payload := strings.TrimSpace(line[index+len(prefix):])
	if payload == "" {
		return JSONProgress{}, false
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return JSONProgress{}, false
	}
	progressRaw, ok := raw["progress"].(map[string]any)
	if !ok {
		return JSONProgress{}, false
	}
	result := JSONProgress{
		Status:      strings.TrimSpace(getString(progressRaw, "status")),
		Filename:    strings.TrimSpace(getString(progressRaw, "filename")),
		TmpFilename: strings.TrimSpace(getString(progressRaw, "tmpfilename")),
	}
	result.Speed = parseProgressSpeed(progressRaw)
	if downloaded, ok := getInt64(progressRaw, "downloaded_bytes"); ok && downloaded >= 0 {
		result.DownloadedBytes = &downloaded
	}
	if total, ok := getInt64(progressRaw, "total_bytes"); ok && total > 0 {
		result.TotalBytes = &total
	}
	if estimate, ok := getInt64(progressRaw, "total_bytes_estimate"); ok && estimate > 0 {
		result.TotalEstimate = &estimate
	}
	if fragmentIndex := getInt(progressRaw, "fragment_index"); fragmentIndex > 0 {
		result.FragmentIndex = &fragmentIndex
	}
	if fragmentCount := getInt(progressRaw, "fragment_count"); fragmentCount > 0 {
		result.FragmentCount = &fragmentCount
	}
	if result.Status == "" && result.DownloadedBytes == nil && result.TotalBytes == nil && result.TotalEstimate == nil && result.Speed == "" {
		return JSONProgress{}, false
	}
	return result, true
}

func parseProgressSpeed(values map[string]any) string {
	if values == nil {
		return ""
	}
	if speedLabel := strings.TrimSpace(getString(values, "speed_str", "_speed_str")); speedLabel != "" && !isMissingValue(speedLabel) {
		return speedLabel
	}
	if raw, ok := values["speed"]; ok {
		switch value := raw.(type) {
		case float64:
			if value > 0 {
				return formatByteRate(value)
			}
		case float32:
			if value > 0 {
				return formatByteRate(float64(value))
			}
		case int:
			if value > 0 {
				return formatByteRate(float64(value))
			}
		case int64:
			if value > 0 {
				return formatByteRate(float64(value))
			}
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" || isMissingValue(trimmed) {
				return ""
			}
			if parsed, ok := parseOptionalFloat(trimmed); ok && parsed > 0 {
				return formatByteRate(parsed)
			}
			return trimmed
		}
	}
	return ""
}

func ComputeJSONProgressPercent(progress JSONProgress) (*float64, *int64) {
	if strings.EqualFold(progress.Status, "finished") {
		percent := 100.0
		total := progress.TotalBytes
		if total == nil || *total <= 0 {
			total = progress.TotalEstimate
		}
		return floatPtr(percent), total
	}
	if progress.DownloadedBytes == nil {
		return nil, progress.TotalBytes
	}
	total := progress.TotalBytes
	if total == nil || *total <= 0 {
		total = progress.TotalEstimate
	}
	if total == nil || *total <= 0 {
		return nil, total
	}
	percent := (float64(*progress.DownloadedBytes) / float64(*total)) * 100
	if percent < 0 {
		percent = 0
	} else if percent > 100 {
		percent = 100
	}
	return floatPtr(percent), total
}

func StageFromProgressStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "starting":
		return "Starting"
	case "downloading":
		return "Downloading"
	case "processing", "post_processing":
		return "Post-processing"
	case "error":
		return "Failed"
	case "finished":
		return "Post-processing"
	default:
		return ""
	}
}

func ParseDownloadProgress(line string) (float64, *int64, string, bool) {
	if !strings.Contains(strings.ToLower(line), "[download]") {
		return 0, nil, "", false
	}
	percentMatch := ytdlpPercentRe.FindStringSubmatch(line)
	if len(percentMatch) < 2 {
		return 0, nil, "", false
	}
	percent, err := strconv.ParseFloat(percentMatch[1], 64)
	if err != nil {
		return 0, nil, "", false
	}

	var totalBytes *int64
	totalMatch := ytdlpTotalRe.FindStringSubmatch(line)
	if len(totalMatch) >= 3 {
		if size, ok := parseByteSize(totalMatch[1], totalMatch[2]); ok {
			totalBytes = &size
		}
	}

	speed := ""
	speedMatch := ytdlpSpeedRe.FindStringSubmatch(line)
	if len(speedMatch) >= 3 {
		speed = strings.TrimSpace(speedMatch[1] + " " + speedMatch[2])
	}

	return percent, totalBytes, speed, true
}

func DetectStage(line string) (string, int, int, bool) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "destination:"):
		if stage := detectTargetStage(line); stage != "" {
			return stage, 0, 0, true
		}
		return "", 0, 0, false
	case strings.Contains(lower, "downloading webpage"),
		strings.Contains(lower, "downloading api json"),
		strings.Contains(lower, "downloading m3u8 information"),
		strings.Contains(lower, "downloading m3u8 info"),
		strings.Contains(lower, "downloading m3u8"):
		return "Fetching metadata", 0, 0, true
	case strings.Contains(lower, "downloading thumbnail"):
		return "Downloading thumbnail", 0, 0, true
	case strings.Contains(lower, "downloading video"):
		return "Downloading video", 1, 4, true
	case strings.Contains(lower, "downloading audio"):
		return "Downloading audio", 2, 4, true
	case strings.Contains(lower, "downloading subtitles"),
		strings.Contains(lower, "downloading subtitle"),
		strings.Contains(lower, "writing video subtitles"),
		strings.Contains(lower, "writing subtitles"):
		return "Downloading subtitles", 0, 0, true
	case strings.Contains(lower, "merging formats"):
		return "Muxing", 3, 4, true
	case strings.Contains(lower, "deleting original"):
		return "Cleaning up", 0, 0, true
	case strings.Contains(lower, "post-process"),
		strings.Contains(lower, "postprocessing"),
		strings.Contains(lower, "extracting"),
		strings.Contains(lower, "converting"),
		strings.Contains(lower, "fixup"),
		strings.Contains(lower, "remuxing"):
		return "Post-processing", 4, 4, true
	default:
		return "", 0, 0, false
	}
}

func detectTargetStage(line string) string {
	if line == "" {
		return ""
	}
	_, raw, ok := strings.Cut(line, ":")
	if !ok {
		return ""
	}
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return "Downloading"
	}
	if isAudioExtension(ext) {
		return "Downloading audio"
	}
	if isSubtitleExtension(ext) {
		return "Downloading subtitles"
	}
	return "Downloading video"
}

func isAudioExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case "m4a", "aac", "mp3", "opus", "ogg", "flac", "wav", "mka":
		return true
	default:
		return false
	}
}

func isSubtitleExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case "srt", "vtt", "ass", "ssa", "sub", "lrc", "xml":
		return true
	default:
		return false
	}
}

func parseByteSize(value string, unit string) (int64, bool) {
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	unit = strings.ToUpper(strings.TrimSpace(unit))
	if unit == "" || unit == "B" {
		return int64(floatValue), true
	}
	base := float64(1000)
	if strings.Contains(unit, "I") {
		base = 1024
	}
	prefix := unit[:1]
	multiplier := float64(1)
	switch prefix {
	case "K":
		multiplier = base
	case "M":
		multiplier = base * base
	case "G":
		multiplier = base * base * base
	case "T":
		multiplier = base * base * base * base
	case "P":
		multiplier = base * base * base * base * base
	default:
		multiplier = 1
	}
	return int64(floatValue * multiplier), true
}

func parseOptionalFloat(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || isMissingValue(value) {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func isMissingValue(value string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if strings.HasPrefix(normalized, "UNKNOWN ") {
		return true
	}
	switch normalized {
	case "N/A", "NA", "NONE", "NULL", "UNKNOWN", "--":
		return true
	default:
		return false
	}
}

func formatByteRate(value float64) string {
	if value <= 0 {
		return ""
	}
	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}
	precision := 0
	if value < 10 && unitIndex > 0 {
		precision = 1
	}
	return strconv.FormatFloat(value, 'f', precision, 64) + " " + units[unitIndex]
}

func floatPtr(value float64) *float64 {
	return &value
}

func getString(values map[string]any, keys ...string) string {
	if values == nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if text, ok := value.(string); ok {
				if trimmed := strings.TrimSpace(text); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func getInt(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	if raw, ok := values[key]; ok {
		switch value := raw.(type) {
		case float64:
			return int(value)
		case int:
			return value
		case int64:
			return int(value)
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func getInt64(values map[string]any, key string) (int64, bool) {
	if values == nil {
		return 0, false
	}
	if raw, ok := values[key]; ok {
		switch value := raw.(type) {
		case float64:
			return int64(value), true
		case int:
			return int64(value), true
		case int64:
			return value, true
		case string:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}
