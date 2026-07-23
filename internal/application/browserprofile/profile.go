package browserprofile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"

	"xiadown/internal/application/browsercdp"
	appcookies "xiadown/internal/application/cookies"
)

const browserProfileCopyLimit = 512 << 20

const localStateReadLimit = 16 << 20

const (
	ProfileStateReady                = "ready"
	ProfileStatePermissionRequired   = "permission_required"
	ProfileStateNoData               = "no_profile_data"
	ProfileStateInvalidData          = "invalid_profile_data"
	ProfileStateUnavailable          = "unavailable"
	ProfileStateBrowserRunning       = "browser_running"
	ProfileStateAccessRequired       = CookieAccessStateAccessRequired
	ProfileStateProtectedUnsupported = CookieAccessStateProtectedUnsupported
)

var appSessionBrowserIDs = []string{"chrome", "edge", "brave", "arc", "vivaldi", "opera"}

var (
	userConfigDir                   = os.UserConfigDir
	detectProfileExecutableIdentity = browsercdp.DetectExecutableIdentity
)

type Profile struct {
	ID           string `json:"id"`
	BrowserID    string `json:"browserId"`
	BrowserLabel string `json:"browserLabel,omitempty"`
	Label        string `json:"label"`
	DisplayPath  string `json:"displayPath,omitempty"`
	IsDefault    bool   `json:"isDefault,omitempty"`
	Available    bool   `json:"available"`
	State        string `json:"state,omitempty"`
	Error        string `json:"error,omitempty"`

	userDataRoot string
	relativeDir  string
	snapshotFile string
	executable   browsercdp.ExecutableIdentity
}

type DiscoveryResult struct {
	BrowserID    string    `json:"browserId"`
	BrowserLabel string    `json:"browserLabel"`
	Available    bool      `json:"available"`
	State        string    `json:"state"`
	Error        string    `json:"error,omitempty"`
	Profiles     []Profile `json:"profiles"`
}

type Source struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// profileRootCandidate keeps browser release channels separate without
// exposing their on-disk locations through the Wails boundary. A profile ID
// includes its root, so the same relative profile directory in Stable and
// Beta remains unambiguous.
type profileRootCandidate struct {
	root       string
	channel    string
	executable browsercdp.ExecutableIdentity
}

// NetworkGateway supplies XiaDown's process-lifetime browser route. Profile
// snapshots never navigate to the public web, but Chromium is still a
// remote-capable child process and must start behind the same fail-closed,
// attested gateway as every other production browser.
type NetworkGateway interface {
	ConsumerProxyURL() string
	ConsumerProxyAttestation() (string, string)
}

type localState struct {
	Profile struct {
		InfoCache map[string]struct {
			Name     string `json:"name"`
			UserName string `json:"user_name"`
		} `json:"info_cache"`
	} `json:"profile"`
	OSCrypt struct {
		AppBoundEncryptedKey string `json:"app_bound_encrypted_key"`
	} `json:"os_crypt"`
}

func List() []Profile {
	result := make([]Profile, 0)
	// Profile discovery is deliberately based on the browser's on-disk data,
	// not on whether a browser process is running. Executable detection is only
	// used to keep an installed browser visible when it has no profile data yet.
	for _, browserID := range appSessionBrowserIDs {
		result = append(result, Discover(browserID).Profiles...)
	}
	result = append(result, Discover("safari").Profiles...)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].BrowserID != result[j].BrowserID {
			return result[i].BrowserID < result[j].BrowserID
		}
		if result[i].IsDefault != result[j].IsDefault {
			return result[i].IsDefault
		}
		return strings.ToLower(result[i].Label) < strings.ToLower(result[j].Label)
	})
	if result == nil {
		return []Profile{}
	}
	return result
}

// ListSources discovers browser applications only. It never reads browser
// profile or cookie data, so it is safe to call when the source sheet opens.
func ListSources() []Source {
	candidates := make(map[string]browsercdp.Candidate)
	for _, candidate := range browsercdp.DetectCandidates() {
		candidates[string(candidate.ID)] = candidate
	}
	result := make([]Source, 0, len(appSessionBrowserIDs)+1)
	for _, browserID := range appSessionBrowserIDs {
		candidate := candidates[browserID]
		result = append(result, Source{
			ID:        browserID,
			Label:     browserLabel(browserID),
			Available: candidate.Available,
			Error:     candidate.Error,
		})
	}
	if safari, ok := safariSource(); ok {
		result = append(result, safari)
	}
	return result
}

// Discover performs lazy, single-browser profile discovery. UI callers should
// use this only after the user selects a browser so opening the source sheet
// does not touch every protected browser data directory at once.
func Discover(browserID string) DiscoveryResult {
	browserID = strings.ToLower(strings.TrimSpace(browserID))
	result := DiscoveryResult{
		BrowserID:    browserID,
		BrowserLabel: browserLabel(browserID),
		State:        ProfileStateNoData,
		Profiles:     []Profile{},
	}
	if browserID == "safari" {
		result.Profiles = listSafariProfiles()
	} else if supportsProfileDiscovery(browserID) {
		roots := profileRootCandidates(browserID)
		for _, candidate := range roots {
			profiles, state := listAtRootDetailed(browserID, candidate.root)
			for index := range profiles {
				profiles[index].Label = profileLabelForChannel(profiles[index].Label, candidate.channel)
				bindProfileExecutable(&profiles[index], candidate.executable)
			}
			result.Profiles = append(result.Profiles, profiles...)
			result.State = strongerProfileState(result.State, state)
		}
		if len(result.Profiles) == 0 && browserInstalled(browserID) {
			root := profileRoot(browserID)
			result.Profiles = append(result.Profiles, unavailableProfile(browserID, root, result.State))
		}
	}
	for _, profile := range result.Profiles {
		if profile.Available {
			result.Available = true
			result.State = ProfileStateReady
			result.Error = ""
			return result
		}
		result.State = strongerProfileState(result.State, profile.State)
	}
	result.Error = profileError(result.State)
	return result
}

// DiscoverForDomains augments lazy profile discovery with the access state of
// cookies that belong to the caller's fixed domain allowlist. Platform
// implementations may inspect protection metadata, but they must not decrypt
// or return cookie values. The ordinary Discover and Resolve paths deliberately
// remain unchanged so SnapshotCookies can re-check protection against a fresh,
// minimized snapshot immediately before a read.
func DiscoverForDomains(browserID string, domains []string) (DiscoveryResult, error) {
	result := Discover(browserID)
	allowedDomains, err := normalizeSnapshotDomains(domains)
	if err != nil {
		return result, err
	}
	if err := applyCookieProtectionDiscovery(&result, allowedDomains); err != nil {
		return result, err
	}
	recomputeDiscoveryAvailability(&result)
	return result, nil
}

func recomputeDiscoveryAvailability(result *DiscoveryResult) {
	if result == nil {
		return
	}
	result.Available = false
	result.State = ProfileStateNoData
	result.Error = ""
	for _, profile := range result.Profiles {
		if profile.Available {
			result.Available = true
			result.State = ProfileStateReady
			return
		}
		result.State = strongerProfileState(result.State, profile.State)
	}
	result.Error = profileError(result.State)
}

func supportsProfileDiscovery(browserID string) bool {
	for _, candidate := range appSessionBrowserIDs {
		if browserID == candidate {
			return true
		}
	}
	return false
}

func browserInstalled(browserID string) bool {
	for _, candidate := range browsercdp.DetectCandidates() {
		if string(candidate.ID) == browserID {
			return candidate.Available
		}
	}
	return false
}

func Resolve(browserID string, profileID string) (Profile, error) {
	browserID = strings.ToLower(strings.TrimSpace(browserID))
	profileID = strings.TrimSpace(profileID)
	if browserID == "" || profileID == "" {
		return Profile{}, fmt.Errorf("browser and profile are required")
	}
	profiles := []Profile{}
	if browserID == "safari" {
		profiles = listSafariProfiles()
	} else {
		for _, candidate := range profileRootCandidates(browserID) {
			channelProfiles := listAtRoot(browserID, candidate.root)
			for index := range channelProfiles {
				channelProfiles[index].Label = profileLabelForChannel(channelProfiles[index].Label, candidate.channel)
				bindProfileExecutable(&channelProfiles[index], candidate.executable)
			}
			profiles = append(profiles, channelProfiles...)
		}
	}
	for _, profile := range profiles {
		if profile.ID == profileID {
			if !profile.Available {
				return Profile{}, fmt.Errorf("browser profile is unavailable: %s", profile.State)
			}
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("browser profile not found")
}

func SnapshotCookies(ctx context.Context, profile Profile, domains []string, gateway NetworkGateway) ([]appcookies.Record, error) {
	if !profile.Available {
		return nil, fmt.Errorf("browser profile is unavailable")
	}
	allowedDomains, err := normalizeSnapshotDomains(domains)
	if err != nil {
		return nil, err
	}
	if profile.BrowserID == "safari" {
		return snapshotSafariCookies(profile, allowedDomains)
	}
	if strings.TrimSpace(profile.userDataRoot) == "" || strings.TrimSpace(profile.relativeDir) == "" {
		return nil, fmt.Errorf("browser profile is unavailable")
	}
	if profile.executable == (browsercdp.ExecutableIdentity{}) || !profile.executable.Available() {
		return nil, browsercdp.ErrExactExecutableUnavailable
	}
	stagedRoot, cleanupSnapshot, err := createSnapshotDirectory()
	if err != nil {
		return nil, err
	}
	defer cleanupSnapshot()

	if err := stageProfile(ctx, profile, stagedRoot, allowedDomains); err != nil {
		return nil, err
	}
	stagedCookies, err := stagedChromiumCookieDatabasePath(profile, stagedRoot)
	if err != nil {
		return nil, err
	}
	hasV20, err := chromiumCookieDatabaseHasV20(stagedCookies)
	if err != nil {
		return nil, err
	}
	if hasV20 {
		return nil, protectedCookieAccessError(profile.BrowserID)
	}
	networkRoute, err := managedNetworkRoute(gateway)
	if err != nil {
		return nil, err
	}
	extraArgs := []string{"--disable-notifications"}
	if profile.relativeDir != "." {
		extraArgs = append(extraArgs, "--profile-directory="+profile.relativeDir)
	}
	launchCtx, cancelLaunch := context.WithTimeout(ctx, 20*time.Second)
	defer cancelLaunch()
	runtimeBrowser, err := browsercdp.Start(launchCtx, browsercdp.LaunchOptions{
		PreferredBrowser:   profile.BrowserID,
		ExecutableIdentity: profile.executable,
		Headless:           true,
		UserDataDir:        stagedRoot,
		PersistentProfile:  true,
		ExtraArgs:          extraArgs,
		NetworkRoute:       networkRoute,
	})
	if err != nil {
		return nil, fmt.Errorf("open read-only browser profile snapshot: %w", err)
	}
	defer func() {
		runtimeBrowser.Stop()
		runtimeBrowser.WaitStopped(5 * time.Second)
	}()

	tabCtx, tabCancel, _, err := browsercdp.AttachOrCreatePageTarget(runtimeBrowser, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer tabCancel()
	var browserCookies []*network.Cookie
	readCtx, readCancel := context.WithTimeout(tabCtx, 8*time.Second)
	defer readCancel()
	err = chromedp.Run(readCtx, chromedp.ActionFunc(func(actionCtx context.Context) error {
		var readErr error
		browserCookies, readErr = storage.GetCookies().Do(actionCtx)
		return readErr
	}))
	if err != nil {
		return nil, fmt.Errorf("read browser profile cookies: %w", err)
	}
	result := make([]appcookies.Record, 0, len(browserCookies))
	for _, cookie := range browserCookies {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		expires := int64(0)
		if cookie.Expires > 0 {
			expires = int64(cookie.Expires)
		}
		result = append(result, appcookies.Record{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Expires:  expires,
			HttpOnly: cookie.HTTPOnly,
			Secure:   cookie.Secure,
			SameSite: strings.ToLower(string(cookie.SameSite)),
		})
	}
	// CDP is deliberately treated as a second trust boundary. Even though the
	// staged SQLite database was minimized before launch, never return a cookie
	// outside the caller's allowlist if Chromium synthesizes or restores one.
	return filterSnapshotCookieRecords(result, allowedDomains), nil
}

func filterSnapshotCookieRecords(records []appcookies.Record, allowedDomains []string) []appcookies.Record {
	return appcookies.FilterByDomains(records, allowedDomains)
}

func managedNetworkRoute(gateway NetworkGateway) (*browsercdp.ManagedNetworkRoute, error) {
	if gateway == nil {
		return nil, fmt.Errorf("managed network gateway is unavailable")
	}
	proxyURL := strings.TrimSpace(gateway.ConsumerProxyURL())
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed == nil || !strings.EqualFold(parsed.Scheme, "http") ||
		parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("managed network gateway URL is invalid")
	}
	host := strings.TrimSpace(parsed.Hostname())
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("managed network gateway is not loopback")
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil || port == 0 {
		return nil, fmt.Errorf("managed network gateway port is invalid")
	}
	attestationURL, attestationToken := gateway.ConsumerProxyAttestation()
	if strings.TrimSpace(attestationURL) == "" || strings.TrimSpace(attestationToken) == "" {
		return nil, fmt.Errorf("managed network gateway attestation is unavailable")
	}
	return &browsercdp.ManagedNetworkRoute{
		ProxyURL:         strings.TrimSuffix(proxyURL, "/"),
		AttestationURL:   strings.TrimSpace(attestationURL),
		AttestationToken: strings.TrimSpace(attestationToken),
	}, nil
}

func listAtRoot(browserID string, root string) []Profile {
	profiles, _ := listAtRootDetailed(browserID, root)
	return profiles
}

func listAtRootDetailed(browserID string, root string) ([]Profile, string) {
	browserID = strings.ToLower(strings.TrimSpace(browserID))
	root = strings.TrimSpace(root)
	if browserID == "" || root == "" {
		return []Profile{}, ProfileStateNoData
	}
	stat, err := os.Lstat(root)
	if err != nil {
		return []Profile{}, profileStateForError(err)
	}
	if !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
		return []Profile{}, ProfileStateInvalidData
	}
	labels, labelsErr := readProfileLabelsDetailed(root)
	if labelsErr != nil {
		return []Profile{}, profileStateForError(labelsErr)
	}
	names := make(map[string]struct{})
	for name := range labels {
		if validRelativeProfileDir(name) {
			names[name] = struct{}{}
		}
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		return []Profile{}, profileStateForError(readErr)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !validRelativeProfileDir(entry.Name()) {
			continue
		}
		if entry.Name() == "Default" || strings.HasPrefix(entry.Name(), "Profile ") {
			names[entry.Name()] = struct{}{}
		}
	}
	if hasCookies, _ := profileHasCookieStoreDetailed(root); hasCookies {
		names["."] = struct{}{}
	}
	result := make([]Profile, 0, len(names))
	for relative := range names {
		profilePath := root
		if relative != "." {
			profilePath = filepath.Join(root, relative)
		}
		if stat, err := os.Lstat(profilePath); err != nil || !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
			continue
		}
		label := strings.TrimSpace(labels[relative])
		if label == "" {
			switch relative {
			case ".", "Default":
				label = "Default"
			default:
				label = relative
			}
		}
		hasCookies, cookieErr := profileHasCookieStoreDetailed(profilePath)
		if !hasCookies && cookieErr == nil {
			continue
		}
		state := ProfileStateReady
		available := true
		if cookieErr != nil {
			state = profileStateForError(cookieErr)
			available = false
		}
		result = append(result, Profile{
			ID:           profileIdentifier(browserID, root, relative),
			BrowserID:    browserID,
			BrowserLabel: browserLabel(browserID),
			Label:        label,
			DisplayPath:  profileDisplayPath(browserID, root, relative),
			IsDefault:    relative == "." || relative == "Default",
			Available:    available,
			State:        state,
			Error:        profileError(state),
			userDataRoot: root,
			relativeDir:  relative,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].IsDefault != result[j].IsDefault {
			return result[i].IsDefault
		}
		if result[i].Available != result[j].Available {
			return result[i].Available
		}
		return strings.ToLower(result[i].Label) < strings.ToLower(result[j].Label)
	})
	return result, ProfileStateNoData
}

func profileDisplayPath(browserID string, root string, relative string) string {
	root = filepath.Clean(strings.TrimSpace(root))
	relative = filepath.Clean(strings.TrimSpace(relative))
	if root == "." || root == "" || relative == "" {
		return ""
	}
	profilePath := root
	if relative != "." {
		profilePath = filepath.Join(root, relative)
	}

	if runtime.GOOS == "windows" {
		for _, candidate := range []struct {
			root   string
			prefix string
		}{
			{root: os.Getenv("LOCALAPPDATA"), prefix: "%LOCALAPPDATA%"},
			{root: os.Getenv("APPDATA"), prefix: "%APPDATA%"},
		} {
			if display, ok := displayPathWithin(candidate.root, profilePath, candidate.prefix); ok {
				return display
			}
		}
	}
	if runtime.GOOS != "windows" {
		if display, ok := displayPathWithin(os.Getenv("XDG_CONFIG_HOME"), profilePath, "$XDG_CONFIG_HOME"); ok {
			return display
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if display, ok := displayPathWithin(home, profilePath, "~"); ok {
			return display
		}
	}

	// Custom browser data roots can live outside standard user directories.
	// Keep their absolute location private while still identifying the profile.
	label := strings.TrimSpace(browserLabel(browserID))
	if label == "" {
		label = "Browser"
	}
	if relative == "." {
		return label
	}
	return filepath.Join(label, relative)
}

func displayPathWithin(root string, target string, prefix string) (string, bool) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", false
	}
	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	targetAbsolute, err := filepath.Abs(target)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(rootAbsolute, targetAbsolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	if relative == "." {
		return prefix, true
	}
	return filepath.Join(prefix, relative), true
}

func readProfileLabels(root string) map[string]string {
	result, _ := readProfileLabelsDetailed(root)
	return result
}

func readProfileLabelsDetailed(root string) (map[string]string, error) {
	result := make(map[string]string)
	state, err := readLocalStateDetailed(root)
	if err != nil {
		return result, err
	}
	for relative, info := range state.Profile.InfoCache {
		if !validRelativeProfileDir(relative) {
			continue
		}
		label := strings.TrimSpace(info.Name)
		if label == "" {
			label = strings.TrimSpace(info.UserName)
		}
		result[relative] = label
	}
	return result, nil
}

func readLocalStateDetailed(root string) (localState, error) {
	var state localState
	path := filepath.Join(root, "Local State")
	info, err := os.Lstat(path)
	if err != nil {
		return state, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > localStateReadLimit {
		return state, fmt.Errorf("browser Local State is invalid")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(payload, &state); err != nil {
		return state, fmt.Errorf("parse browser Local State: %w", err)
	}
	return state, nil
}

func profileHasCookieStore(profilePath string) bool {
	hasCookies, _ := profileHasCookieStoreDetailed(profilePath)
	return hasCookies
}

func profileHasCookieStoreDetailed(profilePath string) (bool, error) {
	for _, relative := range []string{filepath.Join("Network", "Cookies"), "Cookies"} {
		path := filepath.Join(profilePath, relative)
		stat, err := os.Lstat(path)
		if err == nil {
			if !stat.Mode().IsRegular() || stat.Mode()&os.ModeSymlink != 0 || stat.Size() < 16 || stat.Size() > browserProfileCopyLimit {
				return false, fmt.Errorf("browser cookie source is not a valid regular file")
			}
			file, openErr := os.Open(path)
			if openErr != nil {
				return false, openErr
			}
			header := make([]byte, 16)
			_, readErr := io.ReadFull(file, header)
			closeErr := file.Close()
			if readErr != nil {
				return false, readErr
			}
			if closeErr != nil {
				return false, closeErr
			}
			if string(header) != "SQLite format 3\x00" {
				return false, fmt.Errorf("browser cookie source has an invalid header")
			}
			return true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

func unavailableProfile(browserID string, root string, state string) Profile {
	if state == "" || state == ProfileStateReady {
		state = ProfileStateNoData
	}
	return Profile{
		ID:           profileIdentifier(browserID, root, ".unavailable"),
		BrowserID:    browserID,
		BrowserLabel: browserLabel(browserID),
		Label:        "Default",
		IsDefault:    true,
		Available:    false,
		State:        state,
		Error:        profileError(state),
	}
}

func profileStateForError(err error) string {
	if isBrowserProfileInUseError(err) {
		return ProfileStateBrowserRunning
	}
	if errors.Is(err, os.ErrPermission) {
		return ProfileStatePermissionRequired
	}
	if errors.Is(err, os.ErrNotExist) {
		return ProfileStateNoData
	}
	return ProfileStateInvalidData
}

func profileError(state string) string {
	if state == "" || state == ProfileStateReady || state == ProfileStateNoData {
		return ""
	}
	// This stable code is localized by the UI. It intentionally contains no
	// filesystem path or operating-system error that could disclose user data.
	return state
}

func bindProfileExecutable(profile *Profile, identity browsercdp.ExecutableIdentity) {
	if profile == nil {
		return
	}
	profile.executable = identity
	if profile.Available && !identity.Available() {
		profile.Available = false
		profile.State = ProfileStateUnavailable
		profile.Error = profileError(profile.State)
	}
}

func strongerProfileState(current string, candidate string) string {
	rank := func(state string) int {
		switch state {
		case ProfileStateBrowserRunning:
			return 4
		case ProfileStateAccessRequired, ProfileStateProtectedUnsupported:
			return 3
		case ProfileStatePermissionRequired:
			return 3
		case ProfileStateInvalidData:
			return 2
		case ProfileStateUnavailable:
			return 2
		case ProfileStateNoData:
			return 1
		default:
			return 0
		}
	}
	if rank(candidate) > rank(current) {
		return candidate
	}
	return current
}

func profileLabelForChannel(label string, channel string) string {
	label = strings.TrimSpace(label)
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return label
	}
	if label == "" {
		return channel
	}
	return label + " · " + channel
}

func browserLabel(browserID string) string {
	switch browserID {
	case "chrome":
		return "Chrome"
	case "edge":
		return "Edge"
	case "brave":
		return "Brave"
	case "arc":
		return "Arc"
	case "vivaldi":
		return "Vivaldi"
	case "opera":
		return "Opera"
	case "safari":
		return "Safari"
	default:
		return browserID
	}
}

func profileIdentifier(browserID string, root string, relative string) string {
	digest := sha256.Sum256([]byte(browserID + "\x00" + filepath.Clean(root) + "\x00" + relative))
	return "profile-" + hex.EncodeToString(digest[:12])
}

func validRelativeProfileDir(value string) bool {
	value = strings.TrimSpace(value)
	if value == "." {
		return true
	}
	return value != "" && filepath.Base(value) == value && value != ".." && !filepath.IsAbs(value)
}

func stageProfile(ctx context.Context, profile Profile, stagedRoot string, allowedDomains []string) error {
	if len(allowedDomains) == 0 {
		return fmt.Errorf("browser snapshot domain allowlist is empty")
	}
	if !validRelativeProfileDir(profile.relativeDir) {
		return fmt.Errorf("browser profile directory is invalid")
	}
	if err := validateRealDirectory(profile.userDataRoot); err != nil {
		return err
	}
	if err := validateRealDirectory(stagedRoot); err != nil {
		return err
	}
	if err := copyRegularFile(filepath.Join(profile.userDataRoot, "Local State"), filepath.Join(stagedRoot, "Local State"), localStateReadLimit, false); err != nil {
		return err
	}
	destinationProfile := stagedRoot
	sourceProfile := profile.userDataRoot
	if profile.relativeDir != "." {
		destinationProfile = filepath.Join(stagedRoot, profile.relativeDir)
		sourceProfile = filepath.Join(profile.userDataRoot, profile.relativeDir)
	}
	if err := validateContainedDirectory(profile.userDataRoot, sourceProfile); err != nil {
		return err
	}
	if err := os.MkdirAll(destinationProfile, 0o700); err != nil {
		return err
	}
	if err := validateContainedDirectory(stagedRoot, destinationProfile); err != nil {
		return err
	}
	for _, relative := range []string{"Preferences", "Secure Preferences"} {
		if err := copyRegularFile(filepath.Join(sourceProfile, relative), filepath.Join(destinationProfile, relative), browserProfileCopyLimit, true); err != nil {
			return err
		}
	}
	cookieRelative := ""
	for _, relative := range []string{filepath.Join("Network", "Cookies"), "Cookies"} {
		source := filepath.Join(sourceProfile, relative)
		_, err := os.Lstat(source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		cookieRelative = relative
		break
	}
	if cookieRelative == "" {
		return fmt.Errorf("browser profile has no cookie store")
	}
	sourceCookies := filepath.Join(sourceProfile, cookieRelative)
	if err := validateContainedFilePath(profile.userDataRoot, sourceCookies); err != nil {
		return err
	}
	destinationCookies := filepath.Join(destinationProfile, cookieRelative)
	if err := os.MkdirAll(filepath.Dir(destinationCookies), 0o700); err != nil {
		return err
	}
	if err := validateContainedFilePath(stagedRoot, destinationCookies); err != nil {
		return err
	}
	if err := snapshotChromiumCookieDatabase(sourceCookies, destinationCookies); err != nil {
		return err
	}
	if err := minimizeChromiumCookieDatabase(ctx, destinationCookies, allowedDomains); err != nil {
		return err
	}
	return nil
}

func stagedChromiumCookieDatabasePath(profile Profile, stagedRoot string) (string, error) {
	profileRoot := stagedRoot
	if profile.relativeDir != "." {
		profileRoot = filepath.Join(stagedRoot, profile.relativeDir)
	}
	for _, relative := range []string{filepath.Join("Network", "Cookies"), "Cookies"} {
		path := filepath.Join(profileRoot, relative)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 16 || info.Size() > browserProfileCopyLimit {
			return "", fmt.Errorf("staged browser cookie database is invalid")
		}
		return path, nil
	}
	return "", fmt.Errorf("staged browser profile has no cookie store")
}

func validateRealDirectory(path string) error {
	info, err := os.Lstat(filepath.Clean(path))
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("browser profile directory is not a real directory")
	}
	return nil
}

func validateContainedDirectory(root string, target string) error {
	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbsolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(rootAbsolute, targetAbsolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("browser profile path escapes its root")
	}
	if err := validateRealDirectory(rootAbsolute); err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	current := rootAbsolute
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if err := validateRealDirectory(current); err != nil {
			return err
		}
	}
	return nil
}

func validateContainedFilePath(root string, path string) error {
	return validateContainedDirectory(root, filepath.Dir(path))
}

func copyRegularFile(source string, destination string, limit int64, optional bool) error {
	stat, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			if optional {
				return nil
			}
			return err
		}
		return err
	}
	if !stat.Mode().IsRegular() || stat.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("browser profile file is not regular")
	}
	if stat.Size() > limit {
		return fmt.Errorf("browser profile file exceeds safe copy limit")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(output, io.LimitReader(input, limit+1))
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > limit {
		return fmt.Errorf("browser profile file exceeds safe copy limit")
	}
	return nil
}

func profileRootCandidates(browserID string) []profileRootCandidate {
	browserID = strings.ToLower(strings.TrimSpace(browserID))
	result := make([]profileRootCandidate, 0, 4)
	seen := make(map[string]struct{}, 4)
	add := func(root string, channel string) {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "." || !filepath.IsAbs(root) {
			return
		}
		if _, exists := seen[root]; exists {
			return
		}
		seen[root] = struct{}{}
		channel = strings.TrimSpace(channel)
		result = append(result, profileRootCandidate{
			root:       root,
			channel:    channel,
			executable: detectProfileExecutableIdentity(browserID, channel),
		})
	}

	add(profileRoot(browserID), "")
	configDir, _ := userConfigDir()
	switch runtime.GOOS {
	case "darwin":
		if strings.TrimSpace(configDir) == "" {
			break
		}
		switch browserID {
		case "chrome":
			add(filepath.Join(configDir, "Google", "Chrome Beta"), "Beta")
			add(filepath.Join(configDir, "Google", "Chrome Dev"), "Dev")
			add(filepath.Join(configDir, "Google", "Chrome Canary"), "Canary")
		case "edge":
			add(filepath.Join(configDir, "Microsoft Edge Beta"), "Beta")
			add(filepath.Join(configDir, "Microsoft Edge Dev"), "Dev")
			add(filepath.Join(configDir, "Microsoft Edge Canary"), "Canary")
		case "brave":
			add(filepath.Join(configDir, "BraveSoftware", "Brave-Browser-Beta"), "Beta")
			add(filepath.Join(configDir, "BraveSoftware", "Brave-Browser-Nightly"), "Nightly")
		case "vivaldi":
			add(filepath.Join(configDir, "Vivaldi Snapshot"), "Snapshot")
		}
	case "windows":
		local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if local == "" {
			break
		}
		switch browserID {
		case "chrome":
			add(filepath.Join(local, "Google", "Chrome Beta", "User Data"), "Beta")
			add(filepath.Join(local, "Google", "Chrome Dev", "User Data"), "Dev")
			add(filepath.Join(local, "Google", "Chrome SxS", "User Data"), "Canary")
		case "edge":
			add(filepath.Join(local, "Microsoft", "Edge Beta", "User Data"), "Beta")
			add(filepath.Join(local, "Microsoft", "Edge Dev", "User Data"), "Dev")
			add(filepath.Join(local, "Microsoft", "Edge SxS", "User Data"), "Canary")
		case "brave":
			add(filepath.Join(local, "BraveSoftware", "Brave-Browser-Beta", "User Data"), "Beta")
			add(filepath.Join(local, "BraveSoftware", "Brave-Browser-Nightly", "User Data"), "Nightly")
		}
	default:
		if strings.TrimSpace(configDir) == "" {
			break
		}
		switch browserID {
		case "chrome":
			add(filepath.Join(configDir, "google-chrome-beta"), "Beta")
			add(filepath.Join(configDir, "google-chrome-unstable"), "Dev")
		case "edge":
			add(filepath.Join(configDir, "microsoft-edge-beta"), "Beta")
			add(filepath.Join(configDir, "microsoft-edge-dev"), "Dev")
		case "brave":
			add(filepath.Join(configDir, "BraveSoftware", "Brave-Browser-Beta"), "Beta")
			add(filepath.Join(configDir, "BraveSoftware", "Brave-Browser-Nightly"), "Nightly")
		case "vivaldi":
			add(filepath.Join(configDir, "vivaldi-snapshot"), "Snapshot")
		}
	}
	return result
}

func profileRoot(browserID string) string {
	browserID = strings.ToLower(strings.TrimSpace(browserID))
	configDir, _ := userConfigDir()
	switch runtime.GOOS {
	case "darwin":
		switch browserID {
		case "chrome":
			return filepath.Join(configDir, "Google", "Chrome")
		case "chromium":
			return filepath.Join(configDir, "Chromium")
		case "edge":
			return filepath.Join(configDir, "Microsoft Edge")
		case "brave":
			return filepath.Join(configDir, "BraveSoftware", "Brave-Browser")
		case "vivaldi":
			return filepath.Join(configDir, "Vivaldi")
		case "opera":
			return filepath.Join(configDir, "com.operasoftware.Opera")
		case "opera-gx":
			return filepath.Join(configDir, "com.operasoftware.OperaGX")
		case "arc":
			return filepath.Join(configDir, "Arc", "User Data")
		case "yandex":
			return filepath.Join(configDir, "Yandex", "YandexBrowser")
		case "helium":
			return filepath.Join(configDir, "net.imput.helium")
		}
	case "windows":
		local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		roaming := strings.TrimSpace(os.Getenv("APPDATA"))
		switch browserID {
		case "chrome":
			return filepath.Join(local, "Google", "Chrome", "User Data")
		case "chromium":
			return filepath.Join(local, "Chromium", "User Data")
		case "edge":
			return filepath.Join(local, "Microsoft", "Edge", "User Data")
		case "brave":
			return filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data")
		case "vivaldi":
			return filepath.Join(local, "Vivaldi", "User Data")
		case "opera":
			return filepath.Join(roaming, "Opera Software", "Opera Stable")
		case "opera-gx":
			return filepath.Join(roaming, "Opera Software", "Opera GX Stable")
		case "arc":
			matches, _ := filepath.Glob(filepath.Join(local, "Packages", "TheBrowserCompany.Arc_*", "LocalCache", "Local", "Arc", "User Data"))
			if len(matches) > 0 {
				sort.Strings(matches)
				return matches[0]
			}
			return filepath.Join(local, "Arc", "User Data")
		case "yandex":
			return filepath.Join(local, "Yandex", "YandexBrowser", "User Data")
		}
	default:
		switch browserID {
		case "chrome":
			return filepath.Join(configDir, "google-chrome")
		case "chromium":
			return filepath.Join(configDir, "chromium")
		case "edge":
			return filepath.Join(configDir, "microsoft-edge")
		case "brave":
			return filepath.Join(configDir, "BraveSoftware", "Brave-Browser")
		case "vivaldi":
			return filepath.Join(configDir, "vivaldi")
		case "opera":
			return filepath.Join(configDir, "opera")
		case "yandex":
			return filepath.Join(configDir, "yandex-browser")
		}
	}
	return ""
}
