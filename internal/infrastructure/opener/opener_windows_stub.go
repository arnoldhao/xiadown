//go:build !windows

package opener

import "fmt"

func openDirectoryWindows(path string) error {
	return fmt.Errorf("windows directory opener unavailable for %q", path)
}

func revealPathWindows(path string) error {
	return fmt.Errorf("windows reveal opener unavailable for %q", path)
}

func openURLWindows(rawURL string) error {
	return fmt.Errorf("windows url opener unavailable for %q", rawURL)
}
