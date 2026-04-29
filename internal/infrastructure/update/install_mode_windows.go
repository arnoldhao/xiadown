//go:build windows

package update

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const windowsUninstallKeyPath = `Software\Microsoft\Windows\CurrentVersion\Uninstall\DreamAppXiaDown`

func detectWindowsInstallKind(currentExe string) installKind {
	normalizedExe := strings.TrimSpace(currentExe)
	if normalizedExe == "" {
		return installKindUnknown
	}

	installDir := filepath.Dir(normalizedExe)
	uninstallerPath := filepath.Join(installDir, "uninstall.exe")
	hasUninstaller := regularFileExists(uninstallerPath)
	registryMatches := windowsUninstallRegistryMatches(normalizedExe, installDir)

	if hasUninstaller && registryMatches {
		return installKindInstalled
	}
	if !hasUninstaller && !registryMatches {
		return installKindPortable
	}
	return installKindUnknown
}

func windowsUninstallRegistryMatches(currentExe string, installDir string) bool {
	roots := []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER}
	access := uint32(registry.QUERY_VALUE)
	for _, root := range roots {
		key, err := registry.OpenKey(root, windowsUninstallKeyPath, access|registry.WOW64_64KEY)
		if err != nil {
			key, err = registry.OpenKey(root, windowsUninstallKeyPath, access)
		}
		if err != nil {
			continue
		}

		displayIcon, _, _ := key.GetStringValue("DisplayIcon")
		uninstallString, _, _ := key.GetStringValue("UninstallString")
		_ = key.Close()

		if registryInstallValueMatches(displayIcon, currentExe, installDir) ||
			registryInstallValueMatches(uninstallString, currentExe, installDir) {
			return true
		}
	}
	return false
}

func registryInstallValueMatches(raw string, currentExe string, installDir string) bool {
	value := normalizeWindowsRegistryPathText(raw)
	if value == "" {
		return false
	}
	exe := normalizeWindowsRegistryPathText(currentExe)
	dir := normalizeWindowsRegistryPathText(installDir)
	if exe != "" && strings.Contains(value, exe) {
		return true
	}
	return dir != "" && strings.Contains(value, dir) && strings.Contains(value, "uninstall.exe")
}

func normalizeWindowsRegistryPathText(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, `"`)
	value = strings.ReplaceAll(value, "/", `\`)
	return strings.ToLower(value)
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
