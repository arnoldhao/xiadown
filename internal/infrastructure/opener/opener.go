package opener

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func OpenDirectory(path string) error {
	if path == "" {
		return fmt.Errorf("directory is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("access directory: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		return openDirectoryWindows(path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Start()
}

func RevealPath(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return fmt.Errorf("path is empty")
	}
	info, err := os.Stat(cleaned)
	if err == nil && info.IsDir() {
		return OpenDirectory(cleaned)
	}
	if err != nil {
		return OpenDirectory(filepath.Dir(cleaned))
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", cleaned)
	case "windows":
		return revealPathWindows(cleaned)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(cleaned))
	}

	return cmd.Start()
}

func OpenURL(rawURL string) error {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return fmt.Errorf("url is empty")
	}
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported url scheme: %s", scheme)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("url host is empty")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", trimmedURL)
	case "windows":
		return openURLWindows(trimmedURL)
	default:
		cmd = exec.Command("xdg-open", trimmedURL)
	}

	return cmd.Start()
}
