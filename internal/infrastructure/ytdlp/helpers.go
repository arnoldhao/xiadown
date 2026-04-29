package ytdlp

import (
	"encoding/json"
	"strings"
)

func mustJSON(value any) []byte {
	payload, _ := json.Marshal(value)
	return payload
}

func parseJSONLine(line string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func walkInfo(info map[string]any, fn func(map[string]any)) {
	if info == nil {
		return
	}
	fn(info)
	if entries, ok := info["entries"].([]any); ok {
		for _, entry := range entries {
			child, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			walkInfo(child, fn)
		}
	}
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

func isPlaceholderDetail(detail string) bool {
	lower := strings.ToLower(strings.TrimSpace(detail))
	switch lower {
	case "na", "n/a", "null", "none":
		return true
	default:
		return false
	}
}

func isWarningLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "{") {
		return false
	}
	upper := strings.ToUpper(trimmed)
	return strings.HasPrefix(upper, "WARNING") ||
		strings.HasPrefix(upper, "ERROR") ||
		strings.HasPrefix(upper, "FATAL") ||
		strings.Contains(upper, "WARNING:") ||
		strings.Contains(upper, "ERROR:") ||
		strings.Contains(upper, "FATAL:")
}
