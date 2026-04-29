package ytdlp

import (
	"net/url"
	"strings"
)

func SanitizeArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	copied := append([]string{}, args...)
	for i := 0; i < len(copied); i++ {
		if copied[i] != "--proxy" || i+1 >= len(copied) {
			continue
		}
		copied[i+1] = MaskProxyURL(copied[i+1])
	}
	return copied
}

func MaskProxyURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.User == nil {
		return trimmed
	}
	parsed.User = url.User("****")
	return parsed.String()
}

func BuildEnvSnapshot(env []string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	keys := []string{"PATH", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY"}
	result := map[string]string{}
	for _, key := range keys {
		if value, ok := findEnvValue(env, key); ok && strings.TrimSpace(value) != "" {
			if strings.Contains(strings.ToLower(key), "proxy") {
				result[key] = MaskProxyURL(value)
			} else {
				result[key] = value
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func findEnvValue(env []string, key string) (string, bool) {
	if key == "" {
		return "", false
	}
	prefix := strings.ToUpper(key) + "="
	for _, entry := range env {
		if strings.HasPrefix(strings.ToUpper(entry), prefix) {
			return entry[len(prefix):], true
		}
	}
	return "", false
}
