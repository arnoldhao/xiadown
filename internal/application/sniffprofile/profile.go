package sniffprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/browsercdp"
)

const (
	infoEntryLimit      = 20000
	defaultCreateTries  = 50
	defaultCreateDelay  = 10 * time.Millisecond
	fallbackBrowserName = "default"
	manifestFileName    = "profile.json"
	profileDataDirName  = "data"
	profileTrashDirName = ".xiadown-trash"
	profileOwner        = "com.dreamapp.xiadown"
	profileResource     = "sniff-profile"
	profileFormat       = 1
)

var (
	errInfoLimit       = errors.New("sniff profile info limit reached")
	errProfileNotFound = errors.New("sniff profile not found")
	userConfigDir      = os.UserConfigDir
	detectBrowsers     = browsercdp.DetectCandidates
	readManagedRoot    = os.ReadDir
	profileMu          sync.Mutex
	// lifecycleMu closes the gap between resolving a Profile for a new Sniff
	// runtime and publishing that runtime to the active-session registry. Profile
	// mutations take the write side while their active check and filesystem
	// operation run, so a Profile cannot be cleared or removed in that gap.
	lifecycleMu sync.RWMutex
)

// LockForRuntimeStart protects Profile resolution until the caller has either
// published the new runtime to its active-session registry or abandoned the
// start. The returned release function must be called exactly once.
func LockForRuntimeStart() func() {
	lifecycleMu.RLock()
	return lifecycleMu.RUnlock
}

// LockForRead protects a short-lived read/open operation from racing a Profile
// mutation. The returned release function must be called exactly once.
func LockForRead() func() {
	lifecycleMu.RLock()
	return lifecycleMu.RUnlock
}

// LockForMutation makes the caller's active-runtime check and subsequent
// Profile mutation one atomic lifecycle operation relative to runtime start.
// The returned release function must be called exactly once.
func LockForMutation() func() {
	lifecycleMu.Lock()
	return lifecycleMu.Unlock
}

// Manifest is the ownership boundary for a XiaDown-managed browser profile.
// Directories without this exact manifest belong to the legacy layout and are
// deliberately never opened by the runtime.
type Manifest struct {
	Owner         string `json:"owner"`
	Resource      string `json:"resource"`
	FormatVersion int    `json:"formatVersion"`
	ProfileID     string `json:"profileId"`
	DisplayName   string `json:"displayName"`
	BrowserID     string `json:"browserId"`
	IsDefault     bool   `json:"isDefault,omitempty"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	LastUsedAt    string `json:"lastUsedAt,omitempty"`
}

type Info struct {
	ProfileID      string `json:"profileId"`
	DisplayName    string `json:"displayName"`
	Browser        string `json:"browser"`
	IsDefault      bool   `json:"isDefault,omitempty"`
	Redundant      bool   `json:"redundant,omitempty"`
	Exists         bool   `json:"exists"`
	SizeBytes      int64  `json:"sizeBytes"`
	FileCount      int    `json:"fileCount"`
	DirectoryCount int    `json:"directoryCount"`
	LastUsedAt     string `json:"lastUsedAt,omitempty"`
	Truncated      bool   `json:"truncated,omitempty"`
	Error          string `json:"error,omitempty"`
}

func ResolveBrowserID(preferred string) string {
	browserID, err := resolveProfileBrowser(preferred)
	if err == nil {
		return browserID
	}
	if preferred = strings.TrimSpace(preferred); preferred != "" {
		return sanitizeBrowserID(preferred)
	}
	return fallbackBrowserName
}

func resolveProfileBrowser(preferred string) (string, error) {
	if requested := strings.TrimSpace(preferred); requested != "" {
		return requireAvailableBrowser(requested)
	}
	candidate, ok := browsercdp.ChooseCandidate(detectBrowsers(), "")
	if !ok {
		// Empty selections are retained for legacy/internal flows which create a
		// Profile before a supported browser is installed. Runtime resolution
		// still rejects this placeholder until an exact candidate is available.
		return fallbackBrowserName, nil
	}
	return string(candidate.ID), nil
}

// EnsureDefault returns a new-layout managed profile. It never adopts a
// browser-named directory from the legacy layout.
func EnsureDefault(preferredBrowser string) (Manifest, string, error) {
	profileMu.Lock()
	defer profileMu.Unlock()
	browserID, err := resolveProfileBrowser(preferredBrowser)
	if err != nil {
		return Manifest{}, "", err
	}
	profiles, err := ListProfiles()
	if err != nil {
		return Manifest{}, "", err
	}
	for _, profile := range profiles {
		if profile.Browser == browserID && profile.IsDefault {
			manifest, path, err := Load(profile.ProfileID)
			if err != nil {
				continue
			}
			// Version-one Profiles did not record the default role. Persist the
			// role when an inferred default is first resolved so a later rename
			// cannot make XiaDown create a second default Profile.
			if !manifest.IsDefault {
				manifest.IsDefault = true
				manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				root, rootErr := rootPath()
				if rootErr != nil {
					return Manifest{}, "", rootErr
				}
				if writeErr := writeManifest(filepath.Join(root, manifest.ProfileID), manifest); writeErr != nil {
					return Manifest{}, "", writeErr
				}
			}
			return manifest, path, nil
		}
	}
	label := browserLabel(browserID)
	return createDefaultProfile("XiaDown "+label, browserID)
}

func Create(displayName string, preferredBrowser string) (Manifest, string, error) {
	profileMu.Lock()
	defer profileMu.Unlock()
	browserID, err := resolveProfileBrowser(preferredBrowser)
	if err != nil {
		return Manifest{}, "", err
	}
	return createProfile(displayName, browserID)
}

func createProfile(displayName string, browserID string) (Manifest, string, error) {
	return createProfileWithID(uuid.NewString(), displayName, browserID, false)
}

// createDefaultProfile uses a deterministic UUID so two overlapping Wails dev
// processes cannot each create their own implicit default. Directory creation
// is the cross-process atomic boundary; a contender waits briefly for the
// winner to finish its manifest instead of falling back to a second UUID.
func createDefaultProfile(displayName string, browserID string) (Manifest, string, error) {
	browserID = sanitizeBrowserID(browserID)
	profileID := uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte(profileOwner+"/"+profileResource+"/default/"+browserID),
	).String()
	for attempt := 0; attempt < defaultCreateTries; attempt++ {
		manifest, dataDir, loadErr := Load(profileID)
		if loadErr == nil {
			if manifest.BrowserID != browserID || !manifest.IsDefault {
				return Manifest{}, "", fmt.Errorf("invalid default sniff profile owner")
			}
			return manifest, dataDir, nil
		}
		manifest, dataDir, createErr := createProfileWithID(profileID, displayName, browserID, true)
		if createErr == nil {
			return manifest, dataDir, nil
		}
		if !os.IsExist(createErr) {
			return Manifest{}, "", createErr
		}
		time.Sleep(defaultCreateDelay)
	}
	return Manifest{}, "", fmt.Errorf("timed out waiting for default sniff profile")
}

func createProfileWithID(profileID string, displayName string, browserID string, isDefault bool) (Manifest, string, error) {
	root, err := rootPath()
	if err != nil {
		return Manifest{}, "", err
	}
	if err := ensureManagedRoot(root); err != nil {
		return Manifest{}, "", err
	}
	browserID = sanitizeBrowserID(browserID)
	id, err := normalizeProfileID(profileID)
	if err != nil {
		return Manifest{}, "", err
	}
	profileDir := filepath.Join(root, id)
	dataDir := filepath.Join(profileDir, profileDataDirName)
	if err := os.Mkdir(profileDir, 0o700); err != nil {
		return Manifest{}, "", err
	}
	if err := os.Mkdir(dataDir, 0o700); err != nil {
		_ = os.Remove(profileDir)
		return Manifest{}, "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = "XiaDown " + browserLabel(browserID)
	}
	manifest := Manifest{
		Owner:         profileOwner,
		Resource:      profileResource,
		FormatVersion: profileFormat,
		ProfileID:     id,
		DisplayName:   name,
		BrowserID:     browserID,
		IsDefault:     isDefault,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := writeManifest(profileDir, manifest); err != nil {
		_ = os.RemoveAll(profileDir)
		return Manifest{}, "", err
	}
	return manifest, dataDir, nil
}

func Load(profileID string) (Manifest, string, error) {
	id, err := normalizeProfileID(profileID)
	if err != nil {
		return Manifest{}, "", err
	}
	root, err := rootPath()
	if err != nil {
		return Manifest{}, "", err
	}
	if err := validateManagedRoot(root); err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, "", errProfileNotFound
		}
		return Manifest{}, "", err
	}
	profileDir := filepath.Join(root, id)
	manifest, err := readManifest(profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, "", errProfileNotFound
		}
		return Manifest{}, "", err
	}
	if manifest.ProfileID != id {
		return Manifest{}, "", fmt.Errorf("sniff profile manifest id mismatch")
	}
	dataDir := filepath.Join(profileDir, profileDataDirName)
	dataInfo, err := os.Lstat(dataDir)
	if os.IsNotExist(err) {
		if err := os.Mkdir(dataDir, 0o700); err != nil {
			return Manifest{}, "", err
		}
	} else if err != nil {
		return Manifest{}, "", err
	} else if dataInfo.Mode()&os.ModeSymlink != 0 || !dataInfo.IsDir() {
		return Manifest{}, "", fmt.Errorf("invalid sniff profile data directory")
	}
	_ = os.Chmod(dataDir, 0o700)
	return manifest, dataDir, nil
}

func Resolve(profileID string, preferredBrowser string) (Manifest, string, error) {
	if strings.TrimSpace(profileID) == "" {
		requestedBrowser := strings.TrimSpace(preferredBrowser)
		if requestedBrowser != "" {
			browserID, err := requireAvailableBrowser(requestedBrowser)
			if err != nil {
				return Manifest{}, "", err
			}
			return EnsureDefault(browserID)
		}
		manifest, dataDir, err := EnsureDefault("")
		if err != nil {
			return Manifest{}, "", err
		}
		if _, err := requireAvailableBrowser(manifest.BrowserID); err != nil {
			return Manifest{}, "", err
		}
		return manifest, dataDir, nil
	}
	manifest, dataDir, err := Load(profileID)
	if err != nil {
		return Manifest{}, "", err
	}
	if requested := strings.TrimSpace(preferredBrowser); requested != "" {
		requested = sanitizeBrowserID(requested)
		if requested != manifest.BrowserID {
			return Manifest{}, "", fmt.Errorf("sniff profile browser mismatch")
		}
	}
	if _, err := requireAvailableBrowser(manifest.BrowserID); err != nil {
		return Manifest{}, "", err
	}
	return manifest, dataDir, nil
}

func requireAvailableBrowser(browserID string) (string, error) {
	requested := sanitizeBrowserID(browserID)
	candidate, ok := browsercdp.ChooseCandidate(detectBrowsers(), requested)
	if !ok || string(candidate.ID) != requested || !candidate.Available {
		return "", fmt.Errorf("sniff profile browser %q unavailable: %w", requested, browsercdp.ErrNoSupportedBrowser)
	}
	return string(candidate.ID), nil
}

func MarkUsed(profileID string) error {
	profileMu.Lock()
	defer profileMu.Unlock()
	isDefault, err := profileCarriesDefaultRole(profileID)
	if err != nil {
		return err
	}
	manifest, _, err := Load(profileID)
	if err != nil {
		return err
	}
	root, err := rootPath()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	manifest.IsDefault = manifest.IsDefault || isDefault
	manifest.LastUsedAt = now
	manifest.UpdatedAt = now
	return writeManifest(filepath.Join(root, manifest.ProfileID), manifest)
}

func Rename(profileID string, displayName string) (Manifest, error) {
	profileMu.Lock()
	defer profileMu.Unlock()
	name := strings.TrimSpace(displayName)
	if name == "" {
		return Manifest{}, fmt.Errorf("sniff profile name is required")
	}
	isDefault, err := profileCarriesDefaultRole(profileID)
	if err != nil {
		return Manifest{}, err
	}
	manifest, _, err := Load(profileID)
	if err != nil {
		return Manifest{}, err
	}
	root, err := rootPath()
	if err != nil {
		return Manifest{}, err
	}
	manifest.DisplayName = name
	manifest.IsDefault = manifest.IsDefault || isDefault
	manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeManifest(filepath.Join(root, manifest.ProfileID), manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Clear(profileID string) error {
	profileMu.Lock()
	defer profileMu.Unlock()
	isDefault, err := profileCarriesDefaultRole(profileID)
	if err != nil {
		return err
	}
	manifest, dataDir, err := Load(profileID)
	if err != nil {
		return err
	}
	root, err := rootPath()
	if err != nil {
		return err
	}
	trashPath, err := moveManagedPathToTrash(root, dataDir)
	if err != nil {
		return err
	}
	if err := os.Mkdir(dataDir, 0o700); err != nil {
		_ = os.Rename(trashPath, dataDir)
		return err
	}
	manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.LastUsedAt = ""
	manifest.IsDefault = manifest.IsDefault || isDefault
	manifestErr := writeManifest(filepath.Join(root, manifest.ProfileID), manifest)
	removeErr := os.RemoveAll(trashPath)
	return errors.Join(manifestErr, removeErr)
}

func Delete(profileID string) error {
	profileMu.Lock()
	defer profileMu.Unlock()
	id, err := normalizeProfileID(profileID)
	if err != nil {
		return err
	}
	root, err := rootPath()
	if err != nil {
		return err
	}
	if err := validateManagedRoot(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	profileDir := filepath.Join(root, id)
	manifest, err := readManifest(profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if manifest.ProfileID != id {
		return fmt.Errorf("sniff profile manifest id mismatch")
	}
	trashPath, err := moveManagedPathToTrash(root, profileDir)
	if err != nil {
		return err
	}
	return os.RemoveAll(trashPath)
}

// ListProfiles returns the complete managed-profile inventory or a root-level
// error. A missing root is the only empty-success case; callers at UI and API
// boundaries must not misrepresent a permission, validation, or directory-read
// failure as a machine with no Profiles.
func ListProfiles() ([]Info, error) {
	root, err := rootPath()
	if err != nil {
		return nil, fmt.Errorf("resolve sniff profile root: %w", err)
	}
	if err := validateManagedRoot(root); err != nil {
		if os.IsNotExist(err) {
			return []Info{}, nil
		}
		return nil, fmt.Errorf("validate sniff profile root: %w", err)
	}
	entries, err := readManagedRoot(root)
	if err != nil {
		return nil, fmt.Errorf("read sniff profile root: %w", err)
	}
	result := make([]Info, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		profileDir := filepath.Join(root, entry.Name())
		manifest, err := readManifest(profileDir)
		if err != nil || manifest.ProfileID != entry.Name() {
			// Legacy and foreign directories are intentionally invisible here.
			continue
		}
		info := infoForPath(manifest, filepath.Join(profileDir, profileDataDirName))
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		leftUsed, _ := time.Parse(time.RFC3339, result[i].LastUsedAt)
		rightUsed, _ := time.Parse(time.RFC3339, result[j].LastUsedAt)
		if !leftUsed.Equal(rightUsed) {
			return leftUsed.After(rightUsed)
		}
		return strings.ToLower(result[i].DisplayName) < strings.ToLower(result[j].DisplayName)
	})
	classifyDefaultProfiles(result)
	return result, nil
}

// classifyDefaultProfiles preserves exactly one visible default per browser.
// Older version-one manifests had no role field, so an automatically named
// profile is inferred as the default. When a dev hot reload previously raced
// and produced two copies, the most recently used (then largest) copy wins;
// an empty unused copy remains visible in Browser Data but is flagged so the
// new Sniff flow does not offer it as a runnable Profile.
func classifyDefaultProfiles(profiles []Info) {
	byBrowser := make(map[string][]int)
	explicitDefault := make([]bool, len(profiles))
	for index := range profiles {
		explicitDefault[index] = profiles[index].IsDefault
		profiles[index].IsDefault = false
		profiles[index].Redundant = false
		byBrowser[profiles[index].Browser] = append(byBrowser[profiles[index].Browser], index)
	}
	for browserID, indexes := range byBrowser {
		winner := -1
		for _, index := range indexes {
			if explicitDefault[index] && preferredDefault(profiles, index, winner) {
				winner = index
			}
		}
		if winner < 0 {
			implicitName := "XiaDown " + browserLabel(browserID)
			for _, index := range indexes {
				if strings.EqualFold(strings.TrimSpace(profiles[index].DisplayName), implicitName) && preferredDefault(profiles, index, winner) {
					winner = index
				}
			}
		}
		if winner >= 0 {
			profiles[winner].IsDefault = true
			implicitName := "XiaDown " + browserLabel(browserID)
			for _, index := range indexes {
				if index == winner ||
					!strings.EqualFold(strings.TrimSpace(profiles[index].DisplayName), implicitName) ||
					strings.TrimSpace(profiles[index].LastUsedAt) != "" ||
					profiles[index].SizeBytes != 0 ||
					profiles[index].FileCount != 0 ||
					profiles[index].DirectoryCount != 0 {
					continue
				}
				// Preserve the directory for explicit cleanup in Browser Data,
				// but do not offer an empty hot-reload race copy as a runnable
				// Profile in the new Sniff flow.
				profiles[index].Redundant = true
			}
		}
	}
}

func preferredDefault(profiles []Info, candidate int, current int) bool {
	if current < 0 {
		return true
	}
	candidateUsed, candidateErr := time.Parse(time.RFC3339, profiles[candidate].LastUsedAt)
	currentUsed, currentErr := time.Parse(time.RFC3339, profiles[current].LastUsedAt)
	if candidateErr == nil || currentErr == nil {
		if candidateErr != nil {
			return false
		}
		if currentErr != nil || !candidateUsed.Equal(currentUsed) {
			return currentErr != nil || candidateUsed.After(currentUsed)
		}
	}
	if profiles[candidate].SizeBytes != profiles[current].SizeBytes {
		return profiles[candidate].SizeBytes > profiles[current].SizeBytes
	}
	return profiles[candidate].ProfileID < profiles[current].ProfileID
}

func profileCarriesDefaultRole(profileID string) (bool, error) {
	id, err := normalizeProfileID(profileID)
	if err != nil {
		return false, err
	}
	profiles, err := ListProfiles()
	if err != nil {
		return false, err
	}
	for _, profile := range profiles {
		if profile.ProfileID == id {
			return profile.IsDefault, nil
		}
	}
	return false, errProfileNotFound
}

// ExistingProfiles is the intentional best-effort compatibility view used by
// cleanup and legacy internal flows. Interactive callers should use
// ListProfiles so filesystem failures remain visible.
func ExistingProfiles() []Info {
	profiles, err := ListProfiles()
	if err != nil {
		return []Info{}
	}
	return profiles
}

// LegacyInfos inventories only unowned directories. It must never inspect
// their browser data or make them available to a runtime.
func LegacyInfos() []Info {
	root, err := rootPath()
	if err != nil {
		return []Info{}
	}
	if err := validateManagedRoot(root); err != nil {
		return []Info{}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return []Info{}
	}
	result := make([]Info, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !isLegacyBrowserDirectory(entry.Name()) {
			continue
		}
		profileDir := filepath.Join(root, entry.Name())
		if manifest, manifestErr := readManifest(profileDir); manifestErr == nil && manifest.ProfileID == entry.Name() {
			continue
		}
		result = append(result, InfoForPath(entry.Name(), profileDir))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Browser < result[j].Browser })
	return result
}

func ClearLegacy() error {
	profileMu.Lock()
	defer profileMu.Unlock()
	root, err := rootPath()
	if err != nil {
		return err
	}
	if err := validateManagedRoot(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, info := range LegacyInfos() {
		name := filepath.Base(strings.TrimSpace(info.PathForCleanup()))
		if name == "." || name == "" {
			continue
		}
		path := filepath.Join(root, name)
		if manifest, manifestErr := readManifest(path); manifestErr == nil && manifest.ProfileID == name {
			continue
		}
		pathInfo, statErr := os.Lstat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.IsDir() {
			return fmt.Errorf("invalid legacy sniff profile directory")
		}
		trashPath, moveErr := moveManagedPathToTrash(root, path)
		if moveErr != nil {
			return moveErr
		}
		if err := os.RemoveAll(trashPath); err != nil {
			return err
		}
	}
	return nil
}

// PathForCleanup is intentionally package-internal data encoded in Error for
// legacy compatibility? No: legacy Info does not expose paths to callers. The
// browser field is the exact directory basename produced by LegacyInfos.
func (info Info) PathForCleanup() string { return info.Browser }

// Compatibility helpers now resolve only new-layout profiles. They remain for
// callers migrating away from the old browser-keyed API.
func PathForPreferredBrowser(preferred string) (string, error) {
	_, path, err := EnsureDefault(preferred)
	return path, err
}

func PathForBrowserID(browserID string) (string, error) {
	return PathForPreferredBrowser(browserID)
}

func InfoForPreferredBrowser(preferred string) Info {
	browserID := ResolveBrowserID(preferred)
	var fallback Info
	for _, info := range ExistingProfiles() {
		if info.Browser != browserID {
			continue
		}
		if info.IsDefault {
			return info
		}
		if fallback.ProfileID == "" {
			fallback = info
		}
	}
	if fallback.ProfileID != "" {
		return fallback
	}
	return Info{Browser: browserID}
}

func InfoForPath(browserID string, path string) Info {
	return infoForPath(Manifest{BrowserID: sanitizeBrowserID(browserID), DisplayName: browserLabel(browserID)}, path)
}

func EnsureDirectoryForPreferredBrowser(preferred string) (string, error) {
	return PathForPreferredBrowser(preferred)
}

func ClearPreferredBrowser(preferred string) error {
	browserID := ResolveBrowserID(preferred)
	var fallback string
	for _, profile := range ExistingProfiles() {
		if profile.Browser != browserID {
			continue
		}
		if profile.IsDefault {
			return Clear(profile.ProfileID)
		}
		if fallback == "" {
			fallback = profile.ProfileID
		}
	}
	if fallback != "" {
		return Clear(fallback)
	}
	return nil
}

func ExistingBrowserInfos() []Info { return ExistingProfiles() }

func RootPath() (string, error) { return rootPath() }

func rootPath() (string, error) {
	configDir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "xiadown", "browser-profiles", "sniff"), nil
}

func readManifest(profileDir string) (Manifest, error) {
	profileInfo, err := os.Lstat(profileDir)
	if err != nil {
		return Manifest{}, err
	}
	if profileInfo.Mode()&os.ModeSymlink != 0 || !profileInfo.IsDir() {
		return Manifest{}, fmt.Errorf("invalid sniff profile directory")
	}
	manifestPath := filepath.Join(profileDir, manifestFileName)
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return Manifest{}, err
	}
	if manifestInfo.Mode()&os.ModeSymlink != 0 || !manifestInfo.Mode().IsRegular() {
		return Manifest{}, fmt.Errorf("invalid sniff profile manifest file")
	}
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return Manifest{}, err
	}
	manifest.ProfileID = strings.TrimSpace(manifest.ProfileID)
	manifest.DisplayName = strings.TrimSpace(manifest.DisplayName)
	manifest.BrowserID = sanitizeBrowserID(manifest.BrowserID)
	if manifest.Owner != profileOwner || manifest.Resource != profileResource || manifest.FormatVersion != profileFormat || manifest.ProfileID == "" || manifest.DisplayName == "" {
		return Manifest{}, fmt.Errorf("invalid sniff profile manifest")
	}
	if _, err := normalizeProfileID(manifest.ProfileID); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func writeManifest(profileDir string, manifest Manifest) error {
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	profileInfo, err := os.Lstat(profileDir)
	if err != nil {
		return err
	}
	if profileInfo.Mode()&os.ModeSymlink != 0 || !profileInfo.IsDir() {
		return fmt.Errorf("invalid sniff profile directory")
	}
	temporaryFile, err := os.CreateTemp(profileDir, ".profile-*.tmp")
	if err != nil {
		return err
	}
	temporary := temporaryFile.Name()
	defer os.Remove(temporary)
	if err := temporaryFile.Chmod(0o600); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if _, err := temporaryFile.Write(append(payload, '\n')); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(profileDir, manifestFileName))
}

func infoForPath(manifest Manifest, path string) Info {
	info := Info{
		ProfileID:   strings.TrimSpace(manifest.ProfileID),
		DisplayName: strings.TrimSpace(manifest.DisplayName),
		Browser:     sanitizeBrowserID(manifest.BrowserID),
		IsDefault:   manifest.IsDefault,
		LastUsedAt:  strings.TrimSpace(manifest.LastUsedAt),
	}
	stat, err := os.Lstat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			info.Error = err.Error()
		}
		return info
	}
	if stat.Mode()&os.ModeSymlink != 0 {
		info.Error = "refusing sniff profile data symlink"
		return info
	}
	info.Exists = true
	if !stat.IsDir() {
		info.SizeBytes = stat.Size()
		info.FileCount = 1
		return info
	}
	visited := 0
	walkErr := filepath.WalkDir(path, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || currentPath == path {
			return nil
		}
		visited++
		if visited > infoEntryLimit {
			info.Truncated = true
			return errInfoLimit
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
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

func normalizeProfileID(value string) (string, error) {
	id := strings.TrimSpace(value)
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != strings.ToLower(id) {
		return "", fmt.Errorf("invalid sniff profile id")
	}
	return id, nil
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
		case current >= 'a' && current <= 'z', current >= 'A' && current <= 'Z', current >= '0' && current <= '9', current == '-', current == '_', current == '.':
			builder.WriteRune(current)
		default:
			builder.WriteByte('_')
		}
	}
	result := strings.ToLower(strings.Trim(builder.String(), "._-"))
	if result == "" {
		return fallbackBrowserName
	}
	return result
}

func browserLabel(browserID string) string {
	for _, candidate := range detectBrowsers() {
		if string(candidate.ID) == browserID && strings.TrimSpace(candidate.Label) != "" {
			return candidate.Label
		}
	}
	if browserID == "" || browserID == fallbackBrowserName {
		return "Browser"
	}
	return strings.ToUpper(browserID[:1]) + browserID[1:]
}

func isLegacyBrowserDirectory(name string) bool {
	normalized := sanitizeBrowserID(name)
	if normalized != strings.TrimSpace(strings.ToLower(name)) {
		return false
	}
	switch normalized {
	case "chrome", "chromium", "edge", "brave", "vivaldi", "opera", "opera-gx", "arc", "yandex", "helium", fallbackBrowserName:
		return true
	default:
		return false
	}
}

func ensureManagedRoot(root string) error {
	root = filepath.Clean(root)
	// The OS-provided configuration directory is the trust anchor. Below that
	// boundary XiaDown creates each component itself and refuses to follow an
	// existing symlink into unrelated data.
	configRoot := filepath.Dir(filepath.Dir(filepath.Dir(root)))
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		return err
	}
	relative, err := filepath.Rel(configRoot, root)
	if err != nil || relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid sniff profile root")
	}
	current := configRoot
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			if err := os.Mkdir(current, 0o700); err != nil {
				return err
			}
			continue
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("invalid sniff profile root component")
		}
	}
	return nil
}

func validateManagedRoot(root string) error {
	root = filepath.Clean(root)
	configRoot := filepath.Dir(filepath.Dir(filepath.Dir(root)))
	relative, err := filepath.Rel(configRoot, root)
	if err != nil || relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid sniff profile root")
	}
	current := configRoot
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("invalid sniff profile root component")
		}
	}
	return nil
}

func moveManagedPathToTrash(root string, path string) (string, error) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sniff profile path escapes managed root")
	}
	if err := validateManagedRoot(root); err != nil {
		return "", err
	}
	if err := validateManagedPath(root, path); err != nil {
		return "", err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("refusing sniff profile symlink")
	}
	trashRoot := filepath.Join(root, profileTrashDirName)
	trashInfo, err := os.Lstat(trashRoot)
	if os.IsNotExist(err) {
		if err := os.Mkdir(trashRoot, 0o700); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else if trashInfo.Mode()&os.ModeSymlink != 0 || !trashInfo.IsDir() {
		return "", fmt.Errorf("invalid sniff profile trash directory")
	}
	trashPath := filepath.Join(trashRoot, uuid.NewString())
	if err := os.Rename(path, trashPath); err != nil {
		return "", err
	}
	return trashPath, nil
}

func validateManagedPath(root string, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("sniff profile path escapes managed root")
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing sniff profile symlink")
		}
	}
	return nil
}
