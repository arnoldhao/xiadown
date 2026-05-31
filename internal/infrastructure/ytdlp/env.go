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
		if i+1 >= len(copied) {
			continue
		}
		switch copied[i] {
		case "--proxy":
			copied[i+1] = MaskProxyURL(copied[i+1])
		case "--add-header", "--add-headers":
			copied[i+1] = MaskHeaderArg(copied[i+1])
		case "--extractor-args":
			copied[i+1] = MaskExtractorArgs(copied[i+1])
		}
	}
	return copied
}

func MaskExtractorArgs(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	masked := trimmed
	for _, name := range []string{"hls_key", "key_query", "fragment_query", "variant_query"} {
		masked = maskExtractorArgValue(masked, name)
	}
	return masked
}

func maskExtractorArgValue(raw string, name string) string {
	target := strings.ToLower(strings.TrimSpace(name)) + "="
	if target == "=" {
		return raw
	}
	masked := raw
	searchFrom := 0
	for {
		lower := strings.ToLower(masked)
		if searchFrom >= len(lower) {
			return masked
		}
		relativeIndex := strings.Index(lower[searchFrom:], target)
		if relativeIndex < 0 {
			return masked
		}
		index := searchFrom + relativeIndex
		start := index + len(target)
		end := len(masked)
		for offset, r := range masked[start:] {
			switch r {
			case ';', ' ':
				end = start + offset
			}
			if end != len(masked) {
				break
			}
		}
		masked = masked[:start] + "****" + masked[end:]
		searchFrom = start + len("****")
	}
}

func MaskHeaderArg(raw string) string {
	name, value, ok := strings.Cut(raw, ":")
	if !ok {
		return raw
	}
	if !sensitiveHeaderName(name) {
		return raw
	}
	if strings.TrimSpace(value) == "" {
		return strings.TrimSpace(name) + ":"
	}
	return strings.TrimSpace(name) + ": ****"
}

func sensitiveHeaderName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "cookie", "proxy-authorization", "x-csrf-token", "x-xsrf-token":
		return true
	default:
		return false
	}
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
