package locallyricsreader

import (
	"sort"
	"strings"

	"github.com/dhowden/tag"
)

// lyricsFromRaw supplements Metadata.Lyrics for conventions that the library
// exposes only through Raw. It deliberately accepts only concrete text types
// and the library's exported Comm type; arbitrary Stringer values are ignored.
func lyricsFromRaw(raw map[string]interface{}) string {
	if len(raw) == 0 {
		return ""
	}

	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, wanted := range []string{"unsyncedlyrics", "lyrics", "uslt", "ult"} {
		for _, key := range keys {
			if normalizeRawKey(key) != wanted {
				continue
			}
			if text := concreteRawText(raw[key]); strings.TrimSpace(text) != "" {
				return text
			}
		}
	}

	// Some ID3 writers store lyrics in a user-text TXXX frame whose
	// description, rather than frame key, is LYRICS or UNSYNCEDLYRICS.
	for _, key := range keys {
		if normalizeRawKey(key) != "txxx" {
			continue
		}
		if text := describedLyrics(raw[key]); strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func concreteRawText(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case *tag.Comm:
		if typed != nil {
			return typed.Text
		}
	case tag.Comm:
		return typed.Text
	}
	return ""
}

func describedLyrics(value interface{}) string {
	var description, text string
	switch typed := value.(type) {
	case *tag.Comm:
		if typed == nil {
			return ""
		}
		description, text = typed.Description, typed.Text
	case tag.Comm:
		description, text = typed.Description, typed.Text
	default:
		return ""
	}
	key := normalizeRawKey(description)
	if key != "lyrics" && key != "unsyncedlyrics" {
		return ""
	}
	return text
}

func normalizeRawKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, " ", "")
	return key
}
