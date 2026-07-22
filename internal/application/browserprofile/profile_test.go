package browserprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"xiadown/internal/application/browsercdp"
	appcookies "xiadown/internal/application/cookies"
)

func writeBrowserProfileFixture(t *testing.T, path string, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeBrowserCookieDatabaseFixture(t *testing.T, path string, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := sqlite3driver.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE fixture (value TEXT NOT NULL)`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO fixture(value) VALUES (?)`, value); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE cookies (host_key TEXT, name TEXT, value TEXT)`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO cookies(host_key, name, value) VALUES ('.example.com', 'session', ?)`, value); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
}

func readBrowserCookieDatabaseFixture(t *testing.T, path string) string {
	t.Helper()
	database, err := sqlite3driver.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var value string
	if err := database.QueryRow(`SELECT value FROM fixture ORDER BY rowid DESC LIMIT 1`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func readBrowserCookieRows(t *testing.T, path string) map[string]string {
	t.Helper()
	database, err := sqlite3driver.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	rows, err := database.Query(`SELECT host_key, value FROM cookies ORDER BY host_key`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var host, value string
		if err := rows.Scan(&host, &value); err != nil {
			t.Fatal(err)
		}
		result[host] = value
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestListAtRootUsesOpaqueIDsAndLocalStateLabels(t *testing.T) {
	root := t.TempDir()
	writeBrowserProfileFixture(t, filepath.Join(root, "Default", "Network", "Cookies"), "SQLite format 3\x00")
	writeBrowserProfileFixture(t, filepath.Join(root, "Profile 1", "Network", "Cookies"), "SQLite format 3\x00")
	writeBrowserProfileFixture(t, filepath.Join(root, "System Profile", "Network", "Cookies"), "SQLite format 3\x00")
	state := map[string]any{
		"profile": map[string]any{
			"info_cache": map[string]any{
				"Default":   map[string]any{"name": "Personal"},
				"Profile 1": map[string]any{"name": "Work"},
			},
		},
	}
	payload, _ := json.Marshal(state)
	writeBrowserProfileFixture(t, filepath.Join(root, "Local State"), string(payload))

	profiles := listAtRoot("chrome", root)
	if len(profiles) != 2 {
		t.Fatalf("expected two user profiles, got %#v", profiles)
	}
	labels := map[string]bool{profiles[0].Label: true, profiles[1].Label: true}
	if !labels["Personal"] || !labels["Work"] {
		t.Fatalf("expected Local State labels, got %#v", profiles)
	}
	for _, profile := range profiles {
		if !strings.HasPrefix(profile.ID, "profile-") || strings.Contains(profile.ID, root) {
			t.Fatalf("profile id must be opaque: %#v", profile)
		}
		if strings.TrimSpace(profile.DisplayPath) == "" {
			t.Fatalf("profile must expose a safe display path: %#v", profile)
		}
		serialized, err := json.Marshal(profile)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(serialized), root) || strings.Contains(string(serialized), "relativeDir") {
			t.Fatalf("profile path leaked to frontend: %s", serialized)
		}
	}
}

func TestProfileDisplayPathRedactsUserHome(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("APPDATA", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		t.Skip("user home is unavailable")
	}
	root := filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	display := profileDisplayPath("chrome", root, "Default")
	expected := filepath.Join("~", "Library", "Application Support", "Google", "Chrome", "Default")
	if display != expected {
		t.Fatalf("expected redacted display path %q, got %q", expected, display)
	}
	if strings.Contains(display, home) {
		t.Fatalf("display path leaked user home %q", display)
	}
}

func TestDiscoverFindsChromeReleaseChannelProfiles(t *testing.T) {
	configRoot := t.TempDir()
	originalUserConfigDir := userConfigDir
	userConfigDir = func() (string, error) { return configRoot, nil }
	t.Cleanup(func() { userConfigDir = originalUserConfigDir })
	t.Setenv("LOCALAPPDATA", configRoot)
	stableExecutable := filepath.Join(t.TempDir(), "Google Chrome")
	betaExecutable := filepath.Join(t.TempDir(), "Google Chrome Beta")
	writeBrowserProfileFixture(t, stableExecutable, "stable executable")
	writeBrowserProfileFixture(t, betaExecutable, "beta executable")
	originalExecutableDetector := detectProfileExecutableIdentity
	detectProfileExecutableIdentity = func(browserID string, channel string) browsercdp.ExecutableIdentity {
		executable := stableExecutable
		if strings.EqualFold(strings.TrimSpace(channel), "beta") {
			executable = betaExecutable
		}
		return browsercdp.ExecutableIdentityForCandidate(browsercdp.Candidate{
			ID:        browsercdp.BrowserID(strings.ToLower(strings.TrimSpace(browserID))),
			ExecPath:  executable,
			Available: true,
		}, channel)
	}
	t.Cleanup(func() { detectProfileExecutableIdentity = originalExecutableDetector })

	stableRoot := profileRoot("chrome")
	var betaRoot string
	switch runtime.GOOS {
	case "darwin":
		betaRoot = filepath.Join(configRoot, "Google", "Chrome Beta")
	case "windows":
		betaRoot = filepath.Join(configRoot, "Google", "Chrome Beta", "User Data")
	default:
		betaRoot = filepath.Join(configRoot, "google-chrome-beta")
	}
	writeBrowserProfileFixture(t, filepath.Join(stableRoot, "Default", "Network", "Cookies"), "SQLite format 3\x00")
	writeBrowserProfileFixture(t, filepath.Join(stableRoot, "Local State"), `{"profile":{"info_cache":{"Default":{"name":"Personal"}}}}`)
	writeBrowserProfileFixture(t, filepath.Join(betaRoot, "Default", "Network", "Cookies"), "SQLite format 3\x00")
	writeBrowserProfileFixture(t, filepath.Join(betaRoot, "Local State"), `{"profile":{"info_cache":{"Default":{"name":"Work"}}}}`)

	discovery := Discover("chrome")
	if !discovery.Available || discovery.State != ProfileStateReady || len(discovery.Profiles) != 2 {
		t.Fatalf("Chrome Stable/Beta discovery = %#v", discovery)
	}
	profileByLabel := func(result DiscoveryResult, label string) Profile {
		t.Helper()
		for _, profile := range result.Profiles {
			if profile.Label == label {
				return profile
			}
		}
		t.Fatalf("profile %q missing from %#v", label, result.Profiles)
		return Profile{}
	}
	stableProfile := profileByLabel(discovery, "Personal")
	betaProfile := profileByLabel(discovery, "Work · Beta")
	if stableProfile.userDataRoot != stableRoot || betaProfile.userDataRoot != betaRoot {
		t.Fatalf("release channel roots were mixed: stable=%#v beta=%#v", stableProfile, betaProfile)
	}
	if stableProfile.executable == betaProfile.executable {
		t.Fatal("Stable and Beta profiles received the same executable identity")
	}
	resolvedStable, err := Resolve("chrome", stableProfile.ID)
	if err != nil || resolvedStable.userDataRoot != stableRoot || resolvedStable.Label != "Personal" {
		t.Fatalf("resolve Chrome Stable profile = %#v, %v", resolvedStable, err)
	}
	resolvedBeta, err := Resolve("chrome", betaProfile.ID)
	if err != nil || resolvedBeta.userDataRoot != betaRoot || resolvedBeta.Label != "Work · Beta" {
		t.Fatalf("resolve Chrome Beta profile = %#v, %v", resolvedBeta, err)
	}
	payload, err := json.Marshal(discovery)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(payload)
	if strings.Contains(serialized, configRoot) ||
		strings.Contains(serialized, stableExecutable) ||
		strings.Contains(serialized, betaExecutable) ||
		strings.Contains(strings.ToLower(serialized), "executable") ||
		strings.Contains(serialized, "userDataRoot") {
		t.Fatalf("release channel path leaked through JSON: %s", payload)
	}

	if err := os.Remove(stableExecutable); err != nil {
		t.Fatal(err)
	}
	onlyBeta := Discover("chrome")
	if profileByLabel(onlyBeta, "Personal").Available {
		t.Fatal("Stable profile remained available after its exact executable disappeared")
	}
	if !profileByLabel(onlyBeta, "Work · Beta").Available {
		t.Fatal("Beta profile became unavailable while its exact executable still existed")
	}
	if _, err := Resolve("chrome", stableProfile.ID); err == nil {
		t.Fatal("Resolve accepted Stable profile without the Stable executable")
	}
	if _, err := Resolve("chrome", betaProfile.ID); err != nil {
		t.Fatalf("Resolve rejected Beta profile with Beta executable: %v", err)
	}

	writeBrowserProfileFixture(t, stableExecutable, "stable executable")
	if err := os.Remove(betaExecutable); err != nil {
		t.Fatal(err)
	}
	onlyStable := Discover("chrome")
	if !profileByLabel(onlyStable, "Personal").Available {
		t.Fatal("Stable profile became unavailable while its executable still existed")
	}
	if profileByLabel(onlyStable, "Work · Beta").Available {
		t.Fatal("Beta profile remained available after its exact executable disappeared")
	}
	if _, err := Resolve("chrome", betaProfile.ID); err == nil {
		t.Fatal("Resolve accepted Beta profile without the Beta executable")
	}
	if _, err := Resolve("chrome", stableProfile.ID); err != nil {
		t.Fatalf("Resolve rejected unaffected Stable profile: %v", err)
	}
	if _, err := SnapshotCookies(context.Background(), resolvedBeta, []string{"example.com"}, nil); !errors.Is(err, browsercdp.ErrExactExecutableUnavailable) {
		t.Fatalf("resolved Beta profile did not fail closed after executable disappearance: %v", err)
	}
}

func TestListAtRootRejectsSymlinkRoot(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "profile-root")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	profiles, state := listAtRootDetailed("chrome", link)
	if len(profiles) != 0 || state != ProfileStateInvalidData {
		t.Fatalf("symlink root profiles/state = %#v/%q", profiles, state)
	}
}

func TestStageProfileCopiesOnlySelectedProfileCookieState(t *testing.T) {
	root := t.TempDir()
	writeBrowserProfileFixture(t, filepath.Join(root, "Local State"), `{}`)
	writeBrowserCookieDatabaseFixture(t, filepath.Join(root, "Default", "Network", "Cookies"), "selected")
	writeBrowserCookieDatabaseFixture(t, filepath.Join(root, "Profile 1", "Network", "Cookies"), "other")
	writeBrowserProfileFixture(t, filepath.Join(root, "Default", "History"), "private-history")
	staged := t.TempDir()
	profile := Profile{BrowserID: "chrome", userDataRoot: root, relativeDir: "Default"}
	if err := stageProfile(context.Background(), profile, staged, []string{"example.com"}); err != nil {
		t.Fatal(err)
	}
	if value := readBrowserCookieDatabaseFixture(t, filepath.Join(staged, "Default", "Network", "Cookies")); value != "selected" {
		t.Fatalf("selected cookie database was not staged: %q", value)
	}
	for _, path := range []string{
		filepath.Join(staged, "Profile 1"),
		filepath.Join(staged, "Default", "History"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unnecessary browser data should not be copied: %s (%v)", path, err)
		}
	}
}

func TestStageProfileIsAReadOnlySnapshotOfSourceProfile(t *testing.T) {
	root := t.TempDir()
	localState := filepath.Join(root, "Local State")
	cookies := filepath.Join(root, "Default", "Network", "Cookies")
	preferences := filepath.Join(root, "Default", "Preferences")
	writeBrowserProfileFixture(t, localState, `{"source":"local-state"}`)
	writeBrowserCookieDatabaseFixture(t, cookies, "source-cookies")
	writeBrowserProfileFixture(t, preferences, `{"source":"preferences"}`)

	type sourceState struct {
		payload string
		mode    os.FileMode
		modTime int64
	}
	before := make(map[string]sourceState)
	for _, path := range []string{localState, cookies, preferences} {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = sourceState{payload: string(payload), mode: info.Mode(), modTime: info.ModTime().UnixNano()}
	}

	staged := filepath.Join(t.TempDir(), "snapshot")
	if err := os.Mkdir(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	profile := Profile{BrowserID: "chrome", userDataRoot: root, relativeDir: "Default"}
	if err := stageProfile(context.Background(), profile, staged, []string{"example.com"}); err != nil {
		t.Fatal(err)
	}

	for _, stagedPath := range []string{
		filepath.Join(staged, "Local State"),
		filepath.Join(staged, "Default", "Network", "Cookies"),
		filepath.Join(staged, "Default", "Preferences"),
	} {
		if err := os.WriteFile(stagedPath, []byte("changed-in-snapshot"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for path, expected := range before {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(payload) != expected.payload || info.Mode() != expected.mode || info.ModTime().UnixNano() != expected.modTime {
			t.Fatalf("source profile was mutated during staging: %s", path)
		}
	}
}

func TestStageProfileCreatesConsistentWALSnapshotWithoutMutatingSource(t *testing.T) {
	root := t.TempDir()
	writeBrowserProfileFixture(t, filepath.Join(root, "Local State"), `{}`)
	cookies := filepath.Join(root, "Default", "Network", "Cookies")
	if err := os.MkdirAll(filepath.Dir(cookies), 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := sqlite3driver.Open("file:" + filepath.ToSlash(cookies) + "?_pragma=journal_mode(wal)&_pragma=wal_autocheckpoint(0)")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE fixture (value TEXT NOT NULL); INSERT INTO fixture(value) VALUES ('checkpointed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE cookies (host_key TEXT, name TEXT, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO cookies(host_key, name, value) VALUES ('.example.com', 'session', 'allowed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO fixture(value) VALUES ('latest-wal-transaction')`); err != nil {
		t.Fatal(err)
	}
	wal := cookies + "-wal"
	if info, err := os.Stat(wal); err != nil || info.Size() <= 32 {
		t.Fatalf("expected an uncheckpointed WAL, info=%v err=%v", info, err)
	}

	type sourceState struct {
		payload []byte
		mode    os.FileMode
		modTime time.Time
	}
	before := make(map[string]sourceState)
	for _, path := range []string{cookies, wal} {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = sourceState{payload: payload, mode: info.Mode(), modTime: info.ModTime()}
	}

	staged := filepath.Join(t.TempDir(), "snapshot")
	if err := os.Mkdir(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	profile := Profile{BrowserID: "chrome", userDataRoot: root, relativeDir: "Default"}
	if err := stageProfile(context.Background(), profile, staged, []string{"example.com"}); err != nil {
		t.Fatal(err)
	}
	stagedCookies := filepath.Join(staged, "Default", "Network", "Cookies")
	if value := readBrowserCookieDatabaseFixture(t, stagedCookies); value != "latest-wal-transaction" {
		t.Fatalf("staged snapshot missed latest WAL transaction: %q", value)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Lstat(stagedCookies + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("staged snapshot must not copy SQLite sidecar %q: %v", suffix, err)
		}
	}
	for path, expected := range before {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(payload, expected.payload) || info.Mode() != expected.mode || !info.ModTime().Equal(expected.modTime) {
			t.Fatalf("read-only snapshot mutated source SQLite file %s", filepath.Base(path))
		}
	}
}

func TestStageProfileMinimizesCookiesBeforeBrowserLaunch(t *testing.T) {
	root := t.TempDir()
	writeBrowserProfileFixture(t, filepath.Join(root, "Local State"), `{}`)
	cookies := filepath.Join(root, "Default", "Network", "Cookies")
	writeBrowserCookieDatabaseFixture(t, cookies, "allowed-root")
	database, err := sqlite3driver.Open(cookies)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		host  string
		name  string
		value string
	}{
		{host: ".sub.example.com", name: "allowed-subdomain", value: "ALLOWED-SUBDOMAIN"},
		{host: ".evil.test", name: "denied", value: "DENIED-SECRET-MARKER"},
		{host: ".notexample.com", name: "suffix-confusion", value: "DENIED-SUFFIX-MARKER"},
	} {
		if _, err := database.Exec(`INSERT INTO cookies(host_key, name, value) VALUES (?, ?, ?)`, row.host, row.name, row.value); err != nil {
			_ = database.Close()
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	sourcePayload, err := os.ReadFile(cookies)
	if err != nil {
		t.Fatal(err)
	}
	sourceInfo, err := os.Stat(cookies)
	if err != nil {
		t.Fatal(err)
	}

	staged := filepath.Join(t.TempDir(), "snapshot")
	if err := os.Mkdir(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	profile := Profile{BrowserID: "chrome", userDataRoot: root, relativeDir: "Default"}
	if err := stageProfile(context.Background(), profile, staged, []string{".Example.COM."}); err != nil {
		t.Fatal(err)
	}
	stagedCookies := filepath.Join(staged, "Default", "Network", "Cookies")
	rows := readBrowserCookieRows(t, stagedCookies)
	if len(rows) != 2 || rows[".example.com"] != "allowed-root" || rows[".sub.example.com"] != "ALLOWED-SUBDOMAIN" {
		t.Fatalf("staged cookie database was not minimized to exact/subdomain matches: %#v", rows)
	}
	stagedPayload, err := os.ReadFile(stagedCookies)
	if err != nil {
		t.Fatal(err)
	}
	for _, denied := range [][]byte{[]byte("DENIED-SECRET-MARKER"), []byte("DENIED-SUFFIX-MARKER")} {
		if bytes.Contains(stagedPayload, denied) {
			t.Fatalf("VACUUM retained denied cookie value %q in staged database", denied)
		}
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(stagedCookies + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("minimized staged database retained sidecar %q: %v", suffix, err)
		}
	}
	afterPayload, err := os.ReadFile(cookies)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(cookies)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sourcePayload, afterPayload) || sourceInfo.Mode() != afterInfo.Mode() || !sourceInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("minimizing staged cookies mutated the source database")
	}
	if sourceRows := readBrowserCookieRows(t, cookies); len(sourceRows) != 4 {
		t.Fatalf("source cookies were pruned: %#v", sourceRows)
	}
}

func TestSnapshotDomainAllowlistIsBoundedAndCDPResultsAreDefensivelyFiltered(t *testing.T) {
	for _, domains := range [][]string{
		nil,
		{"*.example.com"},
		{"bad_domain.example"},
		{strings.Repeat("a", snapshotDomainLabelLimit+1) + ".com"},
	} {
		if _, err := normalizeSnapshotDomains(domains); err == nil {
			t.Fatalf("expected invalid snapshot domains to be rejected: %#v", domains)
		}
	}
	records := filterSnapshotCookieRecords([]appcookies.Record{
		{Name: "allowed", Domain: ".example.com", Value: "keep"},
		{Name: "subdomain", Domain: "login.example.com", Value: "keep-sub"},
		{Name: "denied", Domain: ".evil.test", Value: "drop"},
	}, []string{"example.com"})
	if len(records) != 2 || records[0].Name != "allowed" || records[1].Name != "subdomain" {
		t.Fatalf("defensive CDP filter retained an unrelated cookie: %#v", records)
	}
}

func TestCopyRegularFileRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	writeBrowserProfileFixture(t, target, "secret")
	link := filepath.Join(root, "Cookies")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if err := copyRegularFile(link, filepath.Join(root, "copy"), browserProfileCopyLimit, false); err == nil {
		t.Fatal("expected symlink profile file to be rejected")
	}
}

func TestStageProfileRejectsCookieStoreThroughSymlinkedDirectory(t *testing.T) {
	root := t.TempDir()
	writeBrowserProfileFixture(t, filepath.Join(root, "Local State"), `{}`)
	outside := filepath.Join(t.TempDir(), "outside-network")
	writeBrowserCookieDatabaseFixture(t, filepath.Join(outside, "Cookies"), "outside")
	profileDir := filepath.Join(root, "Default")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(profileDir, "Network")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	staged := t.TempDir()
	profile := Profile{BrowserID: "chrome", userDataRoot: root, relativeDir: "Default"}
	if err := stageProfile(context.Background(), profile, staged, []string{"example.com"}); err == nil {
		t.Fatal("expected a cookie store through a symlinked directory to be rejected")
	}
	if value := readBrowserCookieDatabaseFixture(t, filepath.Join(outside, "Cookies")); value != "outside" {
		t.Fatalf("source outside profile root was changed: %q", value)
	}
}

func TestStageProfileRejectsSQLiteSidecarSymlink(t *testing.T) {
	root := t.TempDir()
	writeBrowserProfileFixture(t, filepath.Join(root, "Local State"), `{}`)
	cookies := filepath.Join(root, "Default", "Network", "Cookies")
	writeBrowserCookieDatabaseFixture(t, cookies, "source")
	target := filepath.Join(t.TempDir(), "outside-wal")
	if err := os.WriteFile(target, []byte("not-a-wal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, cookies+"-wal"); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	profile := Profile{BrowserID: "chrome", userDataRoot: root, relativeDir: "Default"}
	if err := stageProfile(context.Background(), profile, t.TempDir(), []string{"example.com"}); err == nil {
		t.Fatal("expected a symlinked SQLite sidecar to be rejected")
	}
}

func TestNoProfileDataIsAStateNotAnAccessError(t *testing.T) {
	profile := unavailableProfile("chrome", "/browser/root", ProfileStateNoData)
	if profile.State != ProfileStateNoData || profile.Error != "" {
		t.Fatalf("expected empty profile data to remain a non-error state: %#v", profile)
	}
}
