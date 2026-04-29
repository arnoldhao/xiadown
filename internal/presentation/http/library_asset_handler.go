package http

import (
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type LibraryAssetHandler struct{}

func NewLibraryAssetHandler() *LibraryAssetHandler {
	return &LibraryAssetHandler{}
}

func (handler *LibraryAssetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	setCORSHeaders(w, r)

	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if rawPath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	normalized := normalizeAssetPath(rawPath)
	if normalized == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	cleaned := filepath.Clean(normalized)
	info, err := os.Stat(cleaned)
	if err != nil {
		resolved := resolveAssetPathFallback(cleaned)
		if resolved != cleaned {
			cleaned = resolved
			info, err = os.Stat(cleaned)
		}
		if err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
	}
	if info.IsDir() {
		http.Error(w, "path is a directory", http.StatusBadRequest)
		return
	}

	file, err := os.Open(cleaned)
	if err != nil {
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if contentType := mime.TypeByExtension(filepath.Ext(cleaned)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else if strings.EqualFold(filepath.Ext(cleaned), ".vrm") || strings.EqualFold(filepath.Ext(cleaned), ".vrma") {
		w.Header().Set("Content-Type", "model/gltf-binary")
	}
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func normalizeAssetPath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	value = decodeRepeatedly(value, 2)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "file://") {
		value = value[len("file://"):]
	} else if strings.HasPrefix(lower, "file:") {
		value = value[len("file:"):]
	}
	if len(value) >= 3 && value[0] == '/' && value[2] == ':' {
		value = value[1:]
	}
	return value
}

func decodeRepeatedly(value string, max int) string {
	current := value
	for i := 0; i < max; i++ {
		decoded, err := url.PathUnescape(current)
		if err != nil {
			return current
		}
		if decoded == current {
			break
		}
		current = decoded
	}
	return current
}

func resolveAssetPathFallback(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if pathExists(trimmed) {
		return trimmed
	}
	dir := filepath.Dir(trimmed)
	base := strings.TrimSuffix(filepath.Base(trimmed), filepath.Ext(trimmed))
	if base == "" {
		return trimmed
	}
	ext := strings.ToLower(filepath.Ext(trimmed))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return trimmed
	}
	type candidate struct {
		path  string
		score int
		size  int64
	}
	best := candidate{path: trimmed, score: 0, size: -1}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		nameExt := strings.ToLower(filepath.Ext(name))
		nameBase := strings.TrimSuffix(name, filepath.Ext(name))
		if nameBase == "" {
			continue
		}
		score := 0
		if ext != "" && nameExt == ext {
			if nameBase == base {
				score = 100
			} else if strings.HasPrefix(nameBase, base+".f") {
				score = 90
			}
		} else if nameBase == base {
			score = 80
		} else if strings.HasPrefix(nameBase, base+".f") {
			score = 70
		} else if strings.HasPrefix(nameBase, base) {
			score = 60
		}
		if score == 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		if score > best.score || (score == best.score && size > best.size) {
			best = candidate{path: filepath.Join(dir, name), score: score, size: size}
		}
	}
	return best.path
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
