package logging

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func DefaultLogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "xiadown"), nil
	case "windows":
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "xiadown", "logs"), nil
		}
		return filepath.Join(home, "AppData", "Local", "xiadown", "logs"), nil
	default:
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(dataHome, "xiadown", "logs"), nil
	}
}

func OpenLogDir(path string) error {
	if path == "" {
		return fmt.Errorf("log directory is empty")
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(path, 0o755); mkErr != nil {
				return fmt.Errorf("create log directory: %w", mkErr)
			}
		} else {
			return fmt.Errorf("access log directory: %w", err)
		}
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Start()
}
