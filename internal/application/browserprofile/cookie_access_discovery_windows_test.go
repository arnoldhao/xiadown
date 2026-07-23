//go:build windows

package browserprofile

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"xiadown/internal/application/browsercdp"
)

func TestDiscoverForDomainsClassifiesAllowedV20ByBrowser(t *testing.T) {
	for _, testCase := range []struct {
		browserID string
		wantState string
	}{
		{browserID: "chrome", wantState: ProfileStateAccessRequired},
		{browserID: "edge", wantState: ProfileStateProtectedUnsupported},
		{browserID: "brave", wantState: ProfileStateProtectedUnsupported},
	} {
		t.Run(testCase.browserID, func(t *testing.T) {
			root := configureWindowsProfileDiscovery(t, testCase.browserID)
			writeProtectedCookieDatabaseRowsFixture(
				t,
				filepath.Join(root, "Default", "Network", "Cookies"),
				[]protectedCookieFixtureRow{
					{host: ".youtube.com", cipherHex: "76323001020304"},
					{host: ".private.example", cipherHex: "76313001020304"},
				},
			)

			discovery, err := DiscoverForDomains(testCase.browserID, []string{"youtube.com"})
			if err != nil {
				t.Fatal(err)
			}
			profile := onlyDiscoveredProfile(t, discovery)
			if profile.Available || profile.State != testCase.wantState || profile.Error != testCase.wantState {
				t.Fatalf("protected profile = %#v, want unavailable %q", profile, testCase.wantState)
			}
			if discovery.Available || discovery.State != testCase.wantState || discovery.Error != testCase.wantState {
				t.Fatalf("protected discovery = %#v, want unavailable %q", discovery, testCase.wantState)
			}

			payload, err := json.Marshal(discovery)
			if err != nil {
				t.Fatal(err)
			}
			serialized := string(payload)
			for _, sensitive := range []string{root, ".youtube.com", ".private.example", "cookie-0", "cookie-1"} {
				if strings.Contains(serialized, sensitive) {
					t.Fatalf("protection discovery leaked %q in %s", sensitive, serialized)
				}
			}
		})
	}
}

func TestDiscoverForDomainsIgnoresV20OutsideAllowlist(t *testing.T) {
	root := configureWindowsProfileDiscovery(t, "chrome")
	writeProtectedCookieDatabaseRowsFixture(
		t,
		filepath.Join(root, "Default", "Network", "Cookies"),
		[]protectedCookieFixtureRow{
			{host: ".youtube.com", cipherHex: "76313001020304"},
			{host: ".private.example", cipherHex: "76323001020304"},
		},
	)

	discovery, err := DiscoverForDomains("chrome", []string{"youtube.com"})
	if err != nil {
		t.Fatal(err)
	}
	profile := onlyDiscoveredProfile(t, discovery)
	if !profile.Available || profile.State != ProfileStateReady {
		t.Fatalf("non-allowlisted v20 blocked profile: %#v", profile)
	}
	if !discovery.Available || discovery.State != ProfileStateReady || discovery.Error != "" {
		t.Fatalf("non-allowlisted v20 blocked discovery: %#v", discovery)
	}
}

func TestDiscoverForDomainsKeepsLockedProfileAsBrowserRunning(t *testing.T) {
	root := configureWindowsProfileDiscovery(t, "chrome")
	cookies := filepath.Join(root, "Default", "Network", "Cookies")
	writeProtectedCookieDatabaseFixture(t, cookies, ".youtube.com", "76323001020304")

	path, err := windows.UTF16PtrFromString(cookies)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		path,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)

	discovery, err := DiscoverForDomains("chrome", []string{"youtube.com"})
	if err != nil {
		t.Fatal(err)
	}
	profile := onlyDiscoveredProfile(t, discovery)
	if profile.Available || profile.State != ProfileStateBrowserRunning {
		t.Fatalf("locked profile = %#v, want browser_running", profile)
	}
	if discovery.Available || discovery.State != ProfileStateBrowserRunning {
		t.Fatalf("locked discovery = %#v, want browser_running", discovery)
	}
}

func TestDiscoverForDomainsClassifiesAppBoundBeforeLockedCookieProbe(t *testing.T) {
	root := configureWindowsProfileDiscovery(t, "chrome")
	writeBrowserProfileFixture(
		t,
		filepath.Join(root, "Local State"),
		`{"profile":{"info_cache":{"Default":{"name":"Personal"}}},"os_crypt":{"app_bound_encrypted_key":"QVBQQg=="}}`,
	)
	cookies := filepath.Join(root, "Default", "Network", "Cookies")
	writeProtectedCookieDatabaseFixture(t, cookies, ".youtube.com", "76323001020304")

	path, err := windows.UTF16PtrFromString(cookies)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		path,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)

	discovery, err := DiscoverForDomains("chrome", []string{"youtube.com"})
	if err != nil {
		t.Fatal(err)
	}
	profile := onlyDiscoveredProfile(t, discovery)
	if profile.Available || profile.State != ProfileStateAccessRequired {
		t.Fatalf("app-bound locked profile = %#v, want access_required", profile)
	}
	if discovery.Available || discovery.State != ProfileStateAccessRequired {
		t.Fatalf("app-bound locked discovery = %#v, want access_required", discovery)
	}
}

func configureWindowsProfileDiscovery(t *testing.T, browserID string) string {
	t.Helper()
	localRoot := t.TempDir()
	t.Setenv("LOCALAPPDATA", localRoot)
	t.Setenv("APPDATA", filepath.Join(localRoot, "Roaming"))

	executable := filepath.Join(t.TempDir(), browserID+".exe")
	writeBrowserProfileFixture(t, executable, "test executable")
	identity := browsercdp.ExecutableIdentityForCandidate(browsercdp.Candidate{
		ID:        browsercdp.BrowserID(browserID),
		ExecPath:  executable,
		Available: true,
	}, "")
	originalDetector := detectProfileExecutableIdentity
	detectProfileExecutableIdentity = func(candidateBrowserID string, channel string) browsercdp.ExecutableIdentity {
		if strings.EqualFold(strings.TrimSpace(candidateBrowserID), browserID) && strings.TrimSpace(channel) == "" {
			return identity
		}
		return browsercdp.ExecutableIdentity{}
	}
	t.Cleanup(func() { detectProfileExecutableIdentity = originalDetector })

	root := profileRoot(browserID)
	writeBrowserProfileFixture(
		t,
		filepath.Join(root, "Local State"),
		`{"profile":{"info_cache":{"Default":{"name":"Personal"}}}}`,
	)
	return root
}

func onlyDiscoveredProfile(t *testing.T, discovery DiscoveryResult) Profile {
	t.Helper()
	if len(discovery.Profiles) != 1 {
		t.Fatalf("discovery returned %d profiles: %#v", len(discovery.Profiles), discovery)
	}
	return discovery.Profiles[0]
}
