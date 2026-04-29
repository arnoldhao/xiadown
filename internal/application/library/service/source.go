package service

import (
	"fmt"
	"strconv"
	"strings"
)

func getString(values map[string]any, keys ...string) string {
	if values == nil {
		return ""
	}
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func getInt64(values map[string]any, key string) (int64, error) {
	if values == nil {
		return 0, fmt.Errorf("missing key %s", key)
	}
	raw, ok := values[key]
	if !ok {
		return 0, fmt.Errorf("missing key %s", key)
	}
	switch value := raw.(type) {
	case float64:
		return int64(value), nil
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported value for %s", key)
	}
}
