package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultLaunchAgentLabel = "com.xiadown.app"
const autoStartLaunchArgument = "--autostart"

type Manager struct {
	appName  string
	execPath string
	label    string
}

func NewManager(appName string) (*Manager, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}

	return &Manager{
		appName:  strings.TrimSpace(appName),
		execPath: filepath.Clean(execPath),
		label:    defaultLaunchAgentLabel,
	}, nil
}

func (manager *Manager) SetEnabled(enabled bool) error {
	if manager == nil {
		return nil
	}
	if manager.execPath == "" {
		return fmt.Errorf("autostart executable path is empty")
	}
	return setEnabled(manager.appName, manager.label, manager.execPath, autoStartLaunchArgument, enabled)
}
