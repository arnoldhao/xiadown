package browsercdp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func validateTrustedCurrentBrowserDirectory(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || !filepath.IsAbs(path) {
		return errors.New("current browser directory must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("current browser directory is not a real directory")
	}
	return validateTrustedCurrentBrowserOwner(path, info, true)
}

func readTrustedCurrentBrowserFile(root string, path string, limit int64) ([]byte, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	path = filepath.Clean(strings.TrimSpace(path))
	if limit <= 0 || root == "." || path == "." || !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return nil, errors.New("current browser metadata path is invalid")
	}
	if filepath.Dir(path) != root {
		return nil, errors.New("current browser metadata escaped the trusted directory")
	}
	if err := validateTrustedCurrentBrowserDirectory(root); err != nil {
		return nil, err
	}
	lstat, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !lstat.Mode().IsRegular() || lstat.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("current browser metadata is not a regular file")
	}
	if lstat.Size() < 0 || lstat.Size() > limit {
		return nil, errors.New("current browser metadata exceeds the safe read limit")
	}
	if err := validateTrustedCurrentBrowserOwner(path, lstat, false); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Mode()&os.ModeSymlink != 0 || !os.SameFile(lstat, stat) {
		_ = file.Close()
		return nil, errors.New("current browser metadata changed while opening")
	}
	if err := validateTrustedCurrentBrowserOwner(path, stat, false); err != nil {
		_ = file.Close()
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(data)) > limit {
		return nil, errors.New("current browser metadata exceeds the safe read limit")
	}
	return data, nil
}

func currentChromeProcessRunning(userDataDir string, candidate Candidate) bool {
	userDataDir = filepath.Clean(strings.TrimSpace(userDataDir))
	if userDataDir == "." || !filepath.IsAbs(userDataDir) {
		return false
	}
	if currentChromePlatformProcessRunning(candidate) {
		return true
	}
	// Chromium's default-profile singleton lock includes the root browser PID
	// on Unix. Treat it only as a status hint and verify that PID belongs to
	// Chrome before reporting the bridge as disabled rather than not running.
	lockPath := filepath.Join(userDataDir, "SingletonLock")
	target, err := os.Readlink(lockPath)
	if err != nil {
		return false
	}
	separator := strings.LastIndex(strings.TrimSpace(target), "-")
	if separator < 0 || separator == len(target)-1 {
		return false
	}
	pid, err := strconv.Atoi(target[separator+1:])
	if err != nil || pid <= 0 {
		return false
	}
	return currentChromePIDMatches(pid, candidate)
}

func trustedCurrentBrowserOwnerError(path string) error {
	// Do not include the protected path in errors which may cross Wails.
	_ = path
	return fmt.Errorf("current browser metadata is not owned by the current user")
}
