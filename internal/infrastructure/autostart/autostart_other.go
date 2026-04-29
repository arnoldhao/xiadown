//go:build !darwin && !windows
// +build !darwin,!windows

package autostart

func setEnabled(_ string, _ string, _ string, _ string, _ bool) error {
	return nil
}
