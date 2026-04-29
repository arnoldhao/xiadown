//go:build windows

package wails

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	windowsGetUserTilePathOrdinal = 261
	windowsUserTileCurrentUser    = 0
	windowsUserTileLargeImage     = 0x80000000
	windowsUserTilePathBufferLen  = 2048
)

func loadCurrentUserProfile(_ context.Context) (CurrentUserProfile, error) {
	profile := baseCurrentUserProfile()
	avatarPath := resolveWindowsCurrentUserPicture()
	return finalizeCurrentUserProfile(profile, currentUserAvatar{Path: avatarPath})
}

func resolveWindowsCurrentUserPicture() string {
	currentSID := resolveWindowsCurrentUserSID()
	if currentSID != "" {
		if path := resolveWindowsAccountPictureFromRegistry(currentSID); path != "" {
			return path
		}
	}
	if path := resolveWindowsCurrentUserTileFromShell(); path != "" {
		return path
	}
	if path := resolveWindowsCurrentUserTileFromTemp(); path != "" {
		return path
	}
	return resolveWindowsAccountPictureFromDirectories()
}

func resolveWindowsCurrentUserTileFromShell() string {
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	if err := shell32.Load(); err != nil {
		return ""
	}

	procAddr, err := windows.GetProcAddressByOrdinal(windows.Handle(shell32.Handle()), windowsGetUserTilePathOrdinal)
	if err != nil {
		return ""
	}

	buffer := make([]uint16, windowsUserTilePathBufferLen)
	result, _, _ := syscall.SyscallN(
		procAddr,
		windowsUserTileCurrentUser,
		windowsUserTileLargeImage,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if int32(result) < 0 {
		return ""
	}

	path := strings.TrimSpace(syscall.UTF16ToString(buffer))
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func resolveWindowsCurrentUserTileFromTemp() string {
	tempDir := strings.TrimSpace(os.TempDir())
	if tempDir == "" {
		return ""
	}
	for _, name := range resolveWindowsCurrentUserTileNames() {
		fullPath := filepath.Join(tempDir, name+".bmp")
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}
	return ""
}

func resolveWindowsCurrentUserSID() string {
	current, err := user.Current()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(current.Uid)
}

func resolveWindowsCurrentUserTileNames() []string {
	current, err := user.Current()
	if err != nil {
		return nil
	}
	rawUsername := strings.TrimSpace(current.Username)
	normalizedUsername := normalizeCurrentUsername(rawUsername)
	candidates := []string{
		normalizeWindowsUserTileName(rawUsername),
		normalizeWindowsUserTileName(normalizedUsername),
		normalizedUsername,
	}
	result := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func normalizeWindowsUserTileName(value string) string {
	replaced := strings.ReplaceAll(strings.TrimSpace(value), `\`, "+")
	replaced = strings.ReplaceAll(replaced, `/`, "+")
	return strings.TrimSpace(replaced)
}

func resolveWindowsAccountPictureFromRegistry(sid string) string {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\AccountPicture\Users\`+strings.TrimSpace(sid),
		registry.READ,
	)
	if err != nil {
		return ""
	}
	defer key.Close()

	valueNames, err := key.ReadValueNames(-1)
	if err != nil {
		return ""
	}

	type candidate struct {
		path  string
		score int
	}
	best := candidate{}
	for _, valueName := range valueNames {
		if !strings.HasPrefix(strings.ToLower(valueName), "image") {
			continue
		}
		path, _, err := key.GetStringValue(valueName)
		if err != nil {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		score := parseWindowsImageValueScore(valueName)
		if score >= best.score {
			best = candidate{path: path, score: score}
		}
	}
	return best.path
}

func parseWindowsImageValueScore(valueName string) int {
	trimmed := strings.TrimSpace(valueName)
	if len(trimmed) <= len("Image") {
		return 0
	}
	score, err := strconv.Atoi(trimmed[len("Image"):])
	if err != nil {
		return 0
	}
	return score
}

func resolveWindowsAccountPictureFromDirectories() string {
	candidates := []string{
		filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "AccountPictures"),
		filepath.Join(os.Getenv("PROGRAMDATA"), "Microsoft", "User Account Pictures"),
	}

	type fileCandidate struct {
		path    string
		modTime int64
	}
	files := make([]fileCandidate, 0)
	for _, dir := range candidates {
		trimmedDir := strings.TrimSpace(dir)
		if trimmedDir == "" {
			continue
		}
		entries, err := os.ReadDir(trimmedDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(entry.Name()))
			switch filepath.Ext(name) {
			case ".png", ".jpg", ".jpeg", ".bmp":
			default:
				continue
			}
			fullPath := filepath.Join(trimmedDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileCandidate{
				path:    fullPath,
				modTime: info.ModTime().Unix(),
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime == files[j].modTime {
			return files[i].path > files[j].path
		}
		return files[i].modTime > files[j].modTime
	})
	if len(files) == 0 {
		return ""
	}
	return files[0].path
}
