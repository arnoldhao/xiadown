package ytdlp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type PathResolver func(string) string

var subtitleExtensions = map[string]struct{}{
	".srt":  {},
	".vtt":  {},
	".ass":  {},
	".ssa":  {},
	".sub":  {},
	".ttml": {},
	".lrc":  {},
	".xml":  {},
}

func FindSubtitleOutputs(mainPath string, startedAt time.Time, resolver PathResolver, extraDirs ...string) []string {
	resolved := strings.TrimSpace(mainPath)
	if resolver != nil {
		if candidate := resolver(mainPath); candidate != "" {
			resolved = candidate
		}
	}
	if strings.TrimSpace(resolved) == "" {
		return nil
	}
	baseName := strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))
	outputID := extractOutputID(baseName)
	dirs := map[string]struct{}{
		filepath.Dir(resolved): {},
	}
	for _, dir := range extraDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		dirs[dir] = struct{}{}
	}
	paths := make([]string, 0)
	seen := map[string]struct{}{}
	for dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		byPrefix := make([]string, 0, 4)
		byRecent := make([]string, 0, 4)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if _, ok := subtitleExtensions[ext]; !ok {
				continue
			}
			if baseName != "" && strings.HasPrefix(name, baseName+".") {
				byPrefix = append(byPrefix, filepath.Join(dir, name))
				continue
			}
			if startedAt.IsZero() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if !isRecentFile(info, startedAt) {
				continue
			}
			nameBase := strings.TrimSuffix(name, filepath.Ext(name))
			if outputID != "" && !strings.Contains(nameBase, outputID) {
				continue
			}
			byRecent = append(byRecent, filepath.Join(dir, name))
		}
		for _, candidate := range byPrefix {
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			paths = append(paths, candidate)
		}
		if len(byPrefix) == 0 {
			for _, candidate := range byRecent {
				if _, ok := seen[candidate]; ok {
					continue
				}
				seen[candidate] = struct{}{}
				paths = append(paths, candidate)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

func IsSubtitlePath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := subtitleExtensions[ext]
	return ok
}

func BuildSubtitleTitle(baseTitle string, mainPath string, subtitlePath string) string {
	title := strings.TrimSpace(baseTitle)
	if title == "" {
		title = baseNameWithoutExtension(mainPath)
	}
	mainBase := strings.TrimSuffix(filepath.Base(mainPath), filepath.Ext(mainPath))
	subBase := strings.TrimSuffix(filepath.Base(subtitlePath), filepath.Ext(subtitlePath))
	if mainBase == "" || !strings.HasPrefix(subBase, mainBase) {
		return title
	}
	lang := strings.TrimPrefix(subBase, mainBase)
	lang = strings.Trim(lang, ".-_ ")
	if lang == "" {
		return title
	}
	return fmt.Sprintf("%s (%s)", title, strings.ToUpper(lang))
}

func baseNameWithoutExtension(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	if base == "" || base == "." {
		return ""
	}
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func extractOutputID(baseName string) string {
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		return ""
	}
	parts := strings.Split(baseName, "-")
	if len(parts) < 2 {
		return ""
	}
	id := strings.TrimSpace(parts[len(parts)-1])
	if id == "" {
		return ""
	}
	return id
}

func isRecentFile(info os.FileInfo, startedAt time.Time) bool {
	if info == nil {
		return false
	}
	if startedAt.IsZero() {
		return true
	}
	threshold := startedAt.Add(-2 * time.Second)
	return info.ModTime().After(threshold)
}
