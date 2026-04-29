//go:build darwin

package service

import (
	"os"
	"path/filepath"
	"sort"
)

func platformFontDirectories(home string) []string {
	dirs := []string{
		"/System/Library/Fonts",
	}
	dirs = append(dirs, darwinSystemFontAssetDirectories("/System/Library/AssetsV2")...)
	dirs = append(dirs,
		"/Library/Fonts",
		filepath.Join(home, "Library", "Fonts"),
	)
	return dirs
}

func darwinSystemFontAssetDirectories(base string) []string {
	matches, err := filepath.Glob(filepath.Join(base, "com_apple_MobileAsset_Font*"))
	if err != nil {
		return nil
	}

	dirs := make([]string, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || !info.IsDir() {
			continue
		}
		dirs = append(dirs, match)
	}
	sort.Strings(dirs)
	return dirs
}
