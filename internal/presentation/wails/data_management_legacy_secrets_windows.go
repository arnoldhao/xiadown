//go:build windows

package wails

import (
	"os"
	"path/filepath"
	"strings"
)

func legacyAppSessionSecretInventory() (int, int64, error) {
	entries, root, err := legacyWindowsAppSessionFiles()
	if err != nil {
		return 0, 0, err
	}
	var size int64
	for _, name := range entries {
		if info, statErr := os.Lstat(filepath.Join(root, name)); statErr == nil && info.Mode().IsRegular() {
			size += info.Size()
		}
	}
	return len(entries), size, nil
}

func clearLegacyAppSessionSecrets() error {
	entries, root, err := legacyWindowsAppSessionFiles()
	if err != nil {
		return err
	}
	for _, name := range entries {
		path := filepath.Join(root, name)
		if err := safeRemoveFile(path, root); err != nil {
			return err
		}
	}
	return nil
}

func legacyWindowsAppSessionFiles() ([]string, string, error) {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		return []string{}, "", nil
	}
	root := filepath.Join(base, "XiaDown", "app-sessions")
	if err := ensureNoSymlinkComponents(root, root); err != nil {
		return nil, root, err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []string{}, root, nil
	}
	if err != nil {
		return nil, root, err
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json.dpapi") || filepath.Base(entry.Name()) != entry.Name() {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		result = append(result, entry.Name())
	}
	return result, root, nil
}
