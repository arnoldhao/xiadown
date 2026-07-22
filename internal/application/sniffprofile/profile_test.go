package sniffprofile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/browsercdp"
)

func TestLifecycleMutationWaitsForRuntimeStartBoundary(t *testing.T) {
	releaseStart := LockForRuntimeStart()
	mutationAcquired := make(chan struct{})
	mutationDone := make(chan struct{})
	go func() {
		releaseMutation := LockForMutation()
		close(mutationAcquired)
		releaseMutation()
		close(mutationDone)
	}()

	select {
	case <-mutationAcquired:
		t.Fatal("Profile mutation entered before runtime start published its state")
	case <-time.After(50 * time.Millisecond):
	}

	releaseStart()
	select {
	case <-mutationDone:
	case <-time.After(time.Second):
		t.Fatal("Profile mutation remained blocked after runtime start released its boundary")
	}
}

func useTemporaryConfigDir(t *testing.T) string {
	t.Helper()
	original := userConfigDir
	originalDetectBrowsers := detectBrowsers
	base := t.TempDir()
	userConfigDir = func() (string, error) { return base, nil }
	detectBrowsers = func() []browsercdp.Candidate {
		return []browsercdp.Candidate{
			{ID: browsercdp.BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
		}
	}
	t.Cleanup(func() {
		userConfigDir = original
		detectBrowsers = originalDetectBrowsers
	})
	return base
}

func useDetectedBrowsers(t *testing.T, candidates ...browsercdp.Candidate) {
	t.Helper()
	original := detectBrowsers
	detectBrowsers = func() []browsercdp.Candidate {
		return append([]browsercdp.Candidate(nil), candidates...)
	}
	t.Cleanup(func() { detectBrowsers = original })
}

func TestNewProfilesIgnoreLegacyBrowserDirectories(t *testing.T) {
	base := useTemporaryConfigDir(t)
	legacy := filepath.Join(base, "xiadown", "browser-profiles", "sniff", "chrome")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "Cookies"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}

	if profiles := ExistingProfiles(); len(profiles) != 0 {
		t.Fatalf("legacy profile must not be loaded: %#v", profiles)
	}
	legacyProfiles := LegacyInfos()
	if len(legacyProfiles) != 1 || legacyProfiles[0].Browser != "chrome" {
		t.Fatalf("expected exact legacy inventory, got %#v", legacyProfiles)
	}

	manifest, dataDir, err := Create("Personal", "chrome")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(dataDir)) != manifest.ProfileID || filepath.Base(dataDir) != profileDataDirName {
		t.Fatalf("unexpected managed layout: %q", dataDir)
	}
	profiles := ExistingProfiles()
	if len(profiles) != 1 || profiles[0].ProfileID != manifest.ProfileID || profiles[0].DisplayName != "Personal" {
		t.Fatalf("unexpected managed profiles: %#v", profiles)
	}
}

func TestResolveRequiresLaunchBrowserToMatchProfileManifest(t *testing.T) {
	useTemporaryConfigDir(t)
	useDetectedBrowsers(t,
		browsercdp.Candidate{ID: browsercdp.BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
		browsercdp.Candidate{ID: browsercdp.BrowserEdge, Label: "Edge", ExecPath: "/tmp/edge", Available: true},
	)
	manifest, dataDir, err := Create("Chrome Sniff", "chrome")
	if err != nil {
		t.Fatal(err)
	}

	resolved, resolvedDataDir, err := Resolve(manifest.ProfileID, "chrome")
	if err != nil {
		t.Fatalf("matching launch browser should resolve: %v", err)
	}
	if resolved.BrowserID != "chrome" || resolved.ProfileID != manifest.ProfileID || resolvedDataDir != dataDir {
		t.Fatalf("unexpected resolved profile: %#v %q", resolved, resolvedDataDir)
	}
	if _, _, err := Resolve(manifest.ProfileID, "edge"); err == nil {
		t.Fatal("expected launch browser/profile manifest mismatch to be rejected")
	}

	// An omitted browser is safe because the launch path derives it from the
	// manifest rather than trusting an independent payload value.
	resolved, _, err = Resolve(manifest.ProfileID, "")
	if err != nil {
		t.Fatalf("manifest-derived browser should resolve: %v", err)
	}
	if resolved.BrowserID != manifest.BrowserID {
		t.Fatalf("browser must be derived from manifest: got %q want %q", resolved.BrowserID, manifest.BrowserID)
	}
}

func TestResolveRejectsExplicitSafariWithoutCreatingFallbackProfile(t *testing.T) {
	useTemporaryConfigDir(t)
	useDetectedBrowsers(t,
		browsercdp.Candidate{ID: browsercdp.BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
	)

	if _, _, err := Resolve("", "safari"); !errors.Is(err, browsercdp.ErrNoSupportedBrowser) {
		t.Fatalf("explicit Safari selection error = %v, want ErrNoSupportedBrowser", err)
	}
	profiles, err := ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Fatalf("Safari selection created a fallback Chromium Profile: %#v", profiles)
	}
}

func TestCreateRejectsExplicitUnavailableOrUnsupportedBrowser(t *testing.T) {
	useTemporaryConfigDir(t)
	useDetectedBrowsers(t,
		browsercdp.Candidate{ID: browsercdp.BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
		browsercdp.Candidate{ID: browsercdp.BrowserEdge, Label: "Edge", Available: false, Error: "browser executable not found"},
	)

	for _, browserID := range []string{"edge", "safari"} {
		if _, _, err := Create("Unavailable", browserID); !errors.Is(err, browsercdp.ErrNoSupportedBrowser) {
			t.Fatalf("Create(%q) error = %v, want ErrNoSupportedBrowser", browserID, err)
		}
	}
	profiles, err := ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Fatalf("strict Create left dead Profiles behind: %#v", profiles)
	}
}

func TestResolveRejectsExplicitUnavailableBrowserWithoutFallback(t *testing.T) {
	useTemporaryConfigDir(t)
	useDetectedBrowsers(t,
		browsercdp.Candidate{ID: browsercdp.BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
		browsercdp.Candidate{ID: browsercdp.BrowserEdge, Label: "Edge", Available: false, Error: "browser executable not found"},
	)

	if _, _, err := Resolve("", "edge"); !errors.Is(err, browsercdp.ErrNoSupportedBrowser) {
		t.Fatalf("explicit unavailable Edge selection error = %v, want ErrNoSupportedBrowser", err)
	}
	profiles, err := ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Fatalf("unavailable Edge selection created a Chrome fallback Profile: %#v", profiles)
	}
}

func TestResolveValidatesExistingProfileBrowserAgainstExactCandidate(t *testing.T) {
	useTemporaryConfigDir(t)
	manifest, _, err := Create("Chrome Sniff", "chrome")
	if err != nil {
		t.Fatal(err)
	}
	useDetectedBrowsers(t,
		browsercdp.Candidate{ID: browsercdp.BrowserChrome, Label: "Chrome", Available: false, Error: "browser executable not found"},
		browsercdp.Candidate{ID: browsercdp.BrowserEdge, Label: "Edge", ExecPath: "/tmp/edge", Available: true},
	)

	if _, _, err := Resolve(manifest.ProfileID, ""); !errors.Is(err, browsercdp.ErrNoSupportedBrowser) {
		t.Fatalf("existing Chrome Profile with only Edge available error = %v, want ErrNoSupportedBrowser", err)
	}
}

func TestListProfilesReturnsRootResolutionError(t *testing.T) {
	original := userConfigDir
	userConfigDir = func() (string, error) { return "", errors.New("config unavailable") }
	t.Cleanup(func() { userConfigDir = original })

	if profiles, err := ListProfiles(); err == nil || profiles != nil {
		t.Fatalf("ListProfiles() = %#v, %v; want surfaced root error", profiles, err)
	}
	if profiles := ExistingProfiles(); len(profiles) != 0 {
		t.Fatalf("best-effort ExistingProfiles() = %#v, want empty", profiles)
	}
}

func TestListProfilesReturnsManagedRootReadError(t *testing.T) {
	useTemporaryConfigDir(t)
	if _, _, err := Create("Chrome Sniff", "chrome"); err != nil {
		t.Fatal(err)
	}
	original := readManagedRoot
	readManagedRoot = func(string) ([]os.DirEntry, error) {
		return nil, os.ErrPermission
	}
	t.Cleanup(func() { readManagedRoot = original })

	if profiles, err := ListProfiles(); !errors.Is(err, os.ErrPermission) || profiles != nil {
		t.Fatalf("ListProfiles() = %#v, %v; want permission error", profiles, err)
	}
}

func TestEnsureDefaultNeverAdoptsLegacyBrowserProfile(t *testing.T) {
	base := useTemporaryConfigDir(t)
	legacyDataDir := filepath.Join(base, "xiadown", "browser-profiles", "sniff", "chrome")
	if err := os.MkdirAll(legacyDataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyCookie := filepath.Join(legacyDataDir, "Cookies")
	if err := os.WriteFile(legacyCookie, []byte("legacy-cookie"), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest, dataDir, err := EnsureDefault("chrome")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProfileID == "chrome" || filepath.Clean(dataDir) == filepath.Clean(legacyDataDir) {
		t.Fatalf("legacy directory was adopted by runtime: %#v %q", manifest, dataDir)
	}
	if filepath.Base(dataDir) != profileDataDirName || filepath.Base(filepath.Dir(dataDir)) != manifest.ProfileID {
		t.Fatalf("runtime profile is not in the owned UUID layout: %#v %q", manifest, dataDir)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "Cookies")); !os.IsNotExist(err) {
		t.Fatalf("legacy cookie data leaked into runtime profile: %v", err)
	}
	if payload, err := os.ReadFile(legacyCookie); err != nil || string(payload) != "legacy-cookie" {
		t.Fatalf("legacy profile was mutated: %q %v", payload, err)
	}
}

func TestEnsureDefaultUsesOneDeterministicManagedProfile(t *testing.T) {
	useTemporaryConfigDir(t)

	first, firstDataDir, err := EnsureDefault("chrome")
	if err != nil {
		t.Fatal(err)
	}
	second, secondDataDir, err := EnsureDefault("chrome")
	if err != nil {
		t.Fatal(err)
	}
	if first.ProfileID != second.ProfileID || firstDataDir != secondDataDir {
		t.Fatalf("default profile changed: first=%#v %q second=%#v %q", first, firstDataDir, second, secondDataDir)
	}
	if !first.IsDefault || !second.IsDefault {
		t.Fatalf("default role was not persisted: first=%#v second=%#v", first, second)
	}
	profiles, err := ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || !profiles[0].IsDefault {
		t.Fatalf("unexpected default inventory: %#v", profiles)
	}
}

func TestListProfilesInfersOnlyUsedLegacyImplicitDefault(t *testing.T) {
	useTemporaryConfigDir(t)
	unused, _, err := Create("XiaDown Chrome", "chrome")
	if err != nil {
		t.Fatal(err)
	}
	used, usedDataDir, err := Create("XiaDown Chrome", "chrome")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(usedDataDir, "Cookies"), []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MarkUsed(used.ProfileID); err != nil {
		t.Fatal(err)
	}

	profiles, err := ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("unexpected profiles: %#v", profiles)
	}
	defaults := 0
	redundant := 0
	for _, profile := range profiles {
		if profile.Redundant {
			redundant++
			if profile.ProfileID != unused.ProfileID {
				t.Fatalf("used profile was marked redundant: %#v", profiles)
			}
		}
		if !profile.IsDefault {
			continue
		}
		defaults++
		if profile.ProfileID != used.ProfileID {
			t.Fatalf("unused duplicate %q won over used profile %q", profile.ProfileID, used.ProfileID)
		}
	}
	if defaults != 1 {
		t.Fatalf("default count = %d, profiles=%#v; unused=%q", defaults, profiles, unused.ProfileID)
	}
	if redundant != 1 {
		t.Fatalf("redundant count = %d, profiles=%#v", redundant, profiles)
	}
}

func TestEnsureDefaultAdoptsInferredLegacyProfileWithoutCreatingAnother(t *testing.T) {
	useTemporaryConfigDir(t)
	legacyDefault, _, err := Create("XiaDown Chrome", "chrome")
	if err != nil {
		t.Fatal(err)
	}

	resolved, _, err := EnsureDefault("chrome")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ProfileID != legacyDefault.ProfileID || !resolved.IsDefault {
		t.Fatalf("implicit default was not adopted: created=%#v resolved=%#v", legacyDefault, resolved)
	}
	profiles, err := ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].ProfileID != legacyDefault.ProfileID || !profiles[0].IsDefault {
		t.Fatalf("adoption created another default: %#v", profiles)
	}
}

func TestRenamePersistsInferredDefaultRole(t *testing.T) {
	useTemporaryConfigDir(t)
	implicit, _, err := Create("XiaDown Chrome", "chrome")
	if err != nil {
		t.Fatal(err)
	}

	renamed, err := Rename(implicit.ProfileID, "Creator")
	if err != nil {
		t.Fatal(err)
	}
	if !renamed.IsDefault {
		t.Fatalf("rename lost inferred default role: %#v", renamed)
	}
	resolved, _, err := EnsureDefault("chrome")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ProfileID != implicit.ProfileID {
		t.Fatalf("rename caused another default profile: implicit=%q resolved=%#v", implicit.ProfileID, resolved)
	}
}

func TestForeignOrOldManifestNeverEntersRuntimeProfiles(t *testing.T) {
	base := useTemporaryConfigDir(t)
	profileID := "10e35b61-cc10-4e85-b2f4-553b25f78e9f"
	profileDir := filepath.Join(base, "xiadown", "browser-profiles", "sniff", profileID)
	if err := os.MkdirAll(filepath.Join(profileDir, profileDataDirName), 0o700); err != nil {
		t.Fatal(err)
	}
	foreignManifest := `{
  "owner": "legacy.xiadown",
  "resource": "sniff-profile",
  "formatVersion": 1,
  "profileId": "10e35b61-cc10-4e85-b2f4-553b25f78e9f",
  "displayName": "Old profile",
  "browserId": "chrome"
}`
	if err := os.WriteFile(filepath.Join(profileDir, manifestFileName), []byte(foreignManifest), 0o600); err != nil {
		t.Fatal(err)
	}

	if profiles := ExistingProfiles(); len(profiles) != 0 {
		t.Fatalf("foreign/old manifest must not enter runtime profiles: %#v", profiles)
	}
	if _, _, err := Load(profileID); err == nil {
		t.Fatal("foreign/old manifest must not be loadable by runtime")
	}
}

func TestClearProfilePreservesManifestAndOnlyClearsData(t *testing.T) {
	useTemporaryConfigDir(t)
	manifest, dataDir, err := Create("Sniff", "chrome")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "Cookies"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Clear(manifest.ProfileID); err != nil {
		t.Fatal(err)
	}
	loaded, loadedDataDir, err := Load(manifest.ProfileID)
	if err != nil {
		t.Fatalf("profile should remain loadable: %v", err)
	}
	if loaded.ProfileID != manifest.ProfileID || loadedDataDir != dataDir {
		t.Fatalf("unexpected profile after clear: %#v %q", loaded, loadedDataDir)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "Cookies")); !os.IsNotExist(err) {
		t.Fatalf("browser data should be removed, got %v", err)
	}
}

func TestClearLegacyOnlyRemovesKnownLegacyDirectories(t *testing.T) {
	base := useTemporaryConfigDir(t)
	root := filepath.Join(base, "xiadown", "browser-profiles", "sniff")
	for _, name := range []string{"vivaldi", "unrelated-user-folder"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := ClearLegacy(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "vivaldi")); !os.IsNotExist(err) {
		t.Fatalf("known legacy directory should be removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "unrelated-user-folder")); err != nil {
		t.Fatalf("unknown directory must be preserved: %v", err)
	}
}

func TestManagedProfilesRejectSymlinkedOwnedRoot(t *testing.T) {
	base := useTemporaryConfigDir(t)
	external := filepath.Join(base, "external")
	if err := os.MkdirAll(external, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(base, "xiadown")); err != nil {
		t.Fatal(err)
	}

	if _, _, err := Create("Unsafe", "chrome"); err == nil {
		t.Fatal("expected symlinked managed root to be rejected")
	}
	if profiles, err := ListProfiles(); err == nil || profiles != nil {
		t.Fatalf("strict Profile listing hid symlinked root: profiles=%#v err=%v", profiles, err)
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("managed profile must not be created through a symlink: %#v", entries)
	}
}

func TestLoadAndClearRejectSymlinkedProfileData(t *testing.T) {
	base := useTemporaryConfigDir(t)
	manifest, dataDir, err := Create("Sniff", "chrome")
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(base, "external-browser-data")
	if err := os.MkdirAll(external, 0o700); err != nil {
		t.Fatal(err)
	}
	externalCookie := filepath.Join(external, "Cookies")
	if err := os.WriteFile(externalCookie, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dataDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, dataDir); err != nil {
		t.Fatal(err)
	}

	if _, _, err := Load(manifest.ProfileID); err == nil {
		t.Fatal("expected symlinked data directory to be rejected")
	}
	if err := Clear(manifest.ProfileID); err == nil {
		t.Fatal("expected clear to reject symlinked data directory")
	}
	if payload, err := os.ReadFile(externalCookie); err != nil || string(payload) != "keep" {
		t.Fatalf("external browser data was modified: %q %v", payload, err)
	}
}

func TestClearRejectsSymlinkedTrashDirectory(t *testing.T) {
	base := useTemporaryConfigDir(t)
	manifest, dataDir, err := Create("Sniff", "chrome")
	if err != nil {
		t.Fatal(err)
	}
	cookie := filepath.Join(dataDir, "Cookies")
	if err := os.WriteFile(cookie, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := RootPath()
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(base, "external-trash")
	if err := os.MkdirAll(external, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, profileTrashDirName)); err != nil {
		t.Fatal(err)
	}

	if err := Clear(manifest.ProfileID); err == nil {
		t.Fatal("expected symlinked trash directory to be rejected")
	}
	if payload, err := os.ReadFile(cookie); err != nil || string(payload) != "keep" {
		t.Fatalf("profile data was modified after rejected clear: %q %v", payload, err)
	}
}
