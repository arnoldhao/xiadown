//go:build linux

package browsercdp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func currentChromePIDMatches(pid int, candidate Candidate) bool {
	if pid <= 0 || strings.TrimSpace(candidate.ExecPath) == "" {
		return false
	}
	actual, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return false
	}
	actual, err = filepath.EvalSymlinks(actual)
	if err != nil {
		return false
	}
	expected, err := filepath.EvalSymlinks(candidate.ExecPath)
	if err != nil {
		return false
	}
	return filepath.Clean(actual) == filepath.Clean(expected)
}
