//go:build linux

package service

import "path/filepath"

func platformFontDirectories(home string) []string {
	return []string{
		"/usr/share/fonts",
		"/usr/local/share/fonts",
		filepath.Join(home, ".local", "share", "fonts"),
		filepath.Join(home, ".fonts"),
	}
}
