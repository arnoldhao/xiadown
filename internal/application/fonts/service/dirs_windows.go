//go:build windows

package service

import (
	"os"
	"path/filepath"
)

func platformFontDirectories(home string) []string {
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = `C:\\Windows`
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	dirs := []string{
		filepath.Join(windir, "Fonts"),
	}
	if localAppData != "" {
		dirs = append(dirs, filepath.Join(localAppData, "Microsoft", "Windows", "Fonts"))
	}

	_ = home
	return dirs
}
