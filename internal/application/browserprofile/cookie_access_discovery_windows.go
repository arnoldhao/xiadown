//go:build windows

package browserprofile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sqlite3 "github.com/ncruces/go-sqlite3"
)

// applyCookieProtectionDiscovery probes only already-discovered Chromium
// profiles. It returns stable availability states; no cookie name, host, value,
// ciphertext, filesystem path, or native database error crosses the boundary.
func applyCookieProtectionDiscovery(result *DiscoveryResult, domains []string) error {
	if result == nil || result.BrowserID == "safari" {
		return nil
	}
	type rootProbe struct {
		appBound bool
		checked  bool
	}
	rootProbes := make(map[string]rootProbe)
	for index := range result.Profiles {
		profile := &result.Profiles[index]
		root := filepath.Clean(strings.TrimSpace(profile.userDataRoot))
		probe := rootProbes[root]
		if !probe.checked && root != "." {
			probe.checked = true
			state, err := readLocalStateDetailed(root)
			if err == nil {
				probe.appBound = strings.TrimSpace(state.OSCrypt.AppBoundEncryptedKey) != ""
			}
			rootProbes[root] = probe
		}
		if probe.appBound {
			// App-Bound is a capability-level incompatibility for copied user-data
			// directories. Classify it before opening Cookies so a running Chrome
			// never produces the contradictory "close the browser" copy-path card.
			state := protectedProfileState(profile.BrowserID)
			profile.Available = false
			profile.State = state
			profile.Error = profileError(state)
			continue
		}
		if !profile.Available {
			continue
		}
		state := inspectProfileCookieProtection(*profile, domains)
		if state == ProfileStateReady {
			continue
		}
		profile.Available = false
		profile.State = state
		profile.Error = profileError(state)
	}
	return nil
}

func inspectProfileCookieProtection(profile Profile, domains []string) string {
	cookiePath, err := discoveredProfileCookieDatabasePath(profile)
	if err != nil {
		return profileStateForProtectionProbeError(err)
	}
	hasV20, err := chromiumCookieDatabaseHasV20ForDomains(cookiePath, domains)
	if err != nil {
		return profileStateForProtectionProbeError(err)
	}
	if !hasV20 {
		return ProfileStateReady
	}
	return protectedProfileState(profile.BrowserID)
}

func protectedProfileState(browserID string) string {
	if strings.EqualFold(strings.TrimSpace(browserID), "chrome") {
		return ProfileStateAccessRequired
	}
	return ProfileStateProtectedUnsupported
}

func discoveredProfileCookieDatabasePath(profile Profile) (string, error) {
	root := filepath.Clean(strings.TrimSpace(profile.userDataRoot))
	relative := strings.TrimSpace(profile.relativeDir)
	if root == "." || relative == "" {
		return "", fmt.Errorf("browser profile cookie source is unavailable")
	}
	profileRoot := root
	if relative != "." {
		profileRoot = filepath.Join(root, relative)
	}
	if err := validateContainedDirectory(root, profileRoot); err != nil {
		return "", err
	}
	for _, relativePath := range []string{filepath.Join("Network", "Cookies"), "Cookies"} {
		path := filepath.Join(profileRoot, relativePath)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("browser profile cookie source is invalid")
		}
		if err := validateContainedFilePath(root, path); err != nil {
			return "", err
		}
		return path, nil
	}
	return "", os.ErrNotExist
}

func profileStateForProtectionProbeError(err error) string {
	if errors.Is(err, sqlite3.BUSY) || errors.Is(err, sqlite3.LOCKED) {
		return ProfileStateBrowserRunning
	}
	return profileStateForError(err)
}
