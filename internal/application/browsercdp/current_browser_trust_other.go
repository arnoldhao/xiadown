//go:build !darwin && !linux && !windows

package browsercdp

import (
	"errors"
	"os"
)

func validateTrustedCurrentBrowserOwner(_ string, _ os.FileInfo, _ bool) error {
	return errors.New("current browser ownership checks are unsupported on this platform")
}

func currentChromePlatformProcessRunning(_ Candidate) bool { return false }

func currentChromePIDMatches(_ int, _ Candidate) bool { return false }
