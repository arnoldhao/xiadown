package sniffprofile

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"xiadown/internal/application/browsercdp"
)

const (
	infoEntryLimit      = 20000
	fallbackBrowserName = "default"
)

var errInfoLimit = errors.New("sniff profile info limit reached")

type Info struct {
	Browser        string `json:"browser"`
	Path           string `json:"path,omitempty"`
	Exists         bool   `json:"exists"`
	SizeBytes      int64  `json:"sizeBytes"`
	FileCount      int    `json:"fileCount"`
	DirectoryCount int    `json:"directoryCount"`
	Truncated      bool   `json:"truncated,omitempty"`
	Error          string `json:"error,omitempty"`
}

func ResolveBrowserID(preferred string) string {
	preferred = strings.TrimSpace(preferred)
	status := browsercdp.ResolveStatus(preferred, false)
	if strings.TrimSpace(status.ChosenBrowser) != "" {
		return sanitizeBrowserID(status.ChosenBrowser)
	}
	if preferred != "" {
		return sanitizeBrowserID(preferred)
	}
	return fallbackBrowserName
}

func PathForPreferredBrowser(preferred string) (string, error) {
	return PathForBrowserID(ResolveBrowserID(preferred))
}

func PathForBrowserID(browserID string) (string, error) {
	root, err := rootPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, sanitizeBrowserID(browserID)), nil
}

func InfoForPreferredBrowser(preferred string) Info {
	browserID := ResolveBrowserID(preferred)
	path, err := PathForBrowserID(browserID)
	if err != nil {
		return Info{Browser: browserID, Error: err.Error()}
	}
	return InfoForPath(browserID, path)
}

func InfoForPath(browserID string, path string) Info {
	info := Info{
		Browser: sanitizeBrowserID(browserID),
		Path:    strings.TrimSpace(path),
	}
	if info.Path == "" {
		return info
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		if !os.IsNotExist(err) {
			info.Error = err.Error()
		}
		return info
	}
	info.Exists = true
	if !stat.IsDir() {
		info.SizeBytes = stat.Size()
		info.FileCount = 1
		return info
	}
	visited := 0
	walkErr := filepath.WalkDir(info.Path, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == info.Path {
			return nil
		}
		visited++
		if visited > infoEntryLimit {
			info.Truncated = true
			return errInfoLimit
		}
		if entry.IsDir() {
			info.DirectoryCount++
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return nil
		}
		info.FileCount++
		info.SizeBytes += fileInfo.Size()
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errInfoLimit) {
		info.Error = walkErr.Error()
	}
	return info
}

func EnsureDirectoryForPreferredBrowser(preferred string) (string, error) {
	path, err := PathForPreferredBrowser(preferred)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(path, 0o700)
	return path, nil
}

func ClearPreferredBrowser(preferred string) error {
	path, err := PathForPreferredBrowser(preferred)
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return os.RemoveAll(path)
}

func ExistingBrowserInfos() []Info {
	root, err := rootPath()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	result := make([]Info, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		browser := sanitizeBrowserID(entry.Name())
		result = append(result, InfoForPath(browser, filepath.Join(root, browser)))
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Browser) < strings.ToLower(result[j].Browser)
	})
	return result
}

func rootPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "xiadown", "browser-profiles", "sniff"), nil
}

func sanitizeBrowserID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallbackBrowserName
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, current := range trimmed {
		switch {
		case current >= 'a' && current <= 'z',
			current >= 'A' && current <= 'Z',
			current >= '0' && current <= '9',
			current == '-',
			current == '_',
			current == '.':
			builder.WriteRune(current)
		default:
			builder.WriteByte('_')
		}
	}
	result := strings.Trim(builder.String(), "._-")
	if result == "" {
		return fallbackBrowserName
	}
	return result
}
