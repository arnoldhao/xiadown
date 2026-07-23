//go:build darwin && !ios

package browserprofile

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

type safariProfileFixtureRow struct {
	title      string
	identifier string
	deleted    bool
}

func writeSafariTabsFixture(t *testing.T, path string, rows []safariProfileFixtureRow, keepCommittedWAL bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := sqlite3driver.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if keepCommittedWAL {
		if _, err := database.Exec("PRAGMA journal_mode = WAL"); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec("PRAGMA wal_autocheckpoint = 0"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(`
		CREATE TABLE bookmarks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			parent INTEGER NOT NULL DEFAULT 0,
			type INTEGER NOT NULL,
			subtype INTEGER NOT NULL,
			title TEXT,
			external_uuid TEXT,
			server_id TEXT,
			syncable INTEGER NOT NULL DEFAULT 1,
			hidden INTEGER NOT NULL DEFAULT 0,
			special_id INTEGER NOT NULL DEFAULT 0,
			order_index INTEGER NOT NULL DEFAULT 0,
			deleted INTEGER DEFAULT 0
		)`); err != nil {
		t.Fatal(err)
	}
	if keepCommittedWAL {
		if _, err := database.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range rows {
		deleted := 0
		hidden := 0
		if row.deleted {
			deleted = 1
			hidden = 1
		}
		serverID := ""
		if row.identifier == "DefaultProfile" {
			serverID = "DefaultProfile"
		}
		if _, err := database.Exec(
			"INSERT INTO bookmarks(type, subtype, title, external_uuid, server_id, hidden, deleted) VALUES(1, 0, ?, ?, ?, ?, ?)",
			row.title,
			row.identifier,
			serverID,
			hidden,
			deleted,
		); err != nil {
			t.Fatal(err)
		}
	}
}

func writeSafariCookieFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, safariCookieFixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fileDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(payload)
}

func TestListSafariProfilesUsesOpaqueReadOnlySource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "Cookies.binarycookies")
	if err := os.WriteFile(source, safariCookieFixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles := listSafariProfilesAtPaths([]string{source})
	if len(profiles) != 1 || !profiles[0].Available || profiles[0].State != ProfileStateReady {
		t.Fatalf("unexpected Safari profiles: %#v", profiles)
	}
	if !strings.HasPrefix(profiles[0].ID, "profile-") || profiles[0].snapshotFile != source {
		t.Fatalf("unexpected Safari profile identity: %#v", profiles[0])
	}
	payload, err := json.Marshal(profiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), root) || strings.Contains(string(payload), "snapshotFile") {
		t.Fatalf("Safari source path leaked to frontend: %s", payload)
	}
}

func TestListSafariProfilesRejectsSymlinkCookieSource(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, safariCookieFixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "Cookies.binarycookies")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	profiles := listSafariProfilesAtPaths([]string{link})
	if len(profiles) != 1 || profiles[0].Available || profiles[0].State != ProfileStateInvalidData {
		t.Fatalf("symlink Safari source must fail closed: %#v", profiles)
	}
}

func TestListSafariProfilesDiscoversSafari17StoresFromConsistentReadOnlyWAL(t *testing.T) {
	root := t.TempDir()
	tabsDatabase := filepath.Join(root, "Safari", "SafariTabs.db")
	workUUIDInDatabase := "25237EC2-1111-4222-8333-123456789ABC"
	workUUIDOnDisk := strings.ToLower(workUUIDInDatabase)
	missingUUID := "341B650C-2222-4333-8444-ABCDEF123456"
	deletedUUID := "451B650C-3333-4444-8555-ABCDEF123456"
	writeSafariTabsFixture(t, tabsDatabase, []safariProfileFixtureRow{
		{title: "Personal", identifier: "DefaultProfile"},
		{title: "Work", identifier: workUUIDInDatabase},
		{title: "No cookies yet", identifier: missingUUID},
		{title: "Deleted", identifier: deletedUUID, deleted: true},
	}, true)

	defaultCookie := filepath.Join(root, "Cookies", "Cookies.binarycookies")
	workCookie := filepath.Join(root, "WebKit", "WebsiteDataStore", workUUIDOnDisk, "Cookies", "Cookies.binarycookies")
	writeSafariCookieFixture(t, defaultCookie)
	writeSafariCookieFixture(t, workCookie)

	mainBefore := fileDigest(t, tabsDatabase)
	walPath := tabsDatabase + "-wal"
	walBefore := fileDigest(t, walPath)
	profiles := listSafariProfilesAtLocations(safariProfileLocations{
		tabsDatabase:         tabsDatabase,
		defaultCookiePaths:   []string{defaultCookie},
		websiteDataStoreRoot: filepath.Join(root, "WebKit", "WebsiteDataStore"),
		identityRoot:         root,
	})
	if got := fileDigest(t, tabsDatabase); got != mainBefore {
		t.Fatal("SafariTabs.db was mutated during discovery")
	}
	if got := fileDigest(t, walPath); got != walBefore {
		t.Fatal("SafariTabs.db WAL was mutated during discovery")
	}
	if len(profiles) != 3 {
		t.Fatalf("expected default, Work, and missing-cookie profiles from committed WAL, got %#v", profiles)
	}
	byLabel := make(map[string]Profile, len(profiles))
	for _, profile := range profiles {
		byLabel[profile.Label] = profile
		payload, err := json.Marshal(profile)
		if err != nil {
			t.Fatal(err)
		}
		serialized := string(payload)
		for _, privateValue := range []string{root, workUUIDInDatabase, workUUIDOnDisk, strings.ToLower(missingUUID), "snapshotFile"} {
			if strings.Contains(serialized, privateValue) {
				t.Fatalf("Safari profile metadata leaked through JSON: %s", serialized)
			}
		}
	}
	if profile := byLabel["Personal"]; !profile.Available || !profile.IsDefault || profile.State != ProfileStateReady {
		t.Fatalf("unexpected default Safari profile: %#v", profile)
	}
	if profile := byLabel["Work"]; !profile.Available || profile.IsDefault || profile.State != ProfileStateReady || profile.snapshotFile != workCookie {
		t.Fatalf("unexpected secondary Safari profile: %#v", profile)
	}
	workCookieBefore := fileDigest(t, workCookie)
	records, err := snapshotSafariCookies(byLabel["Work"], []string{"example.com"})
	if err != nil || len(records) != 1 {
		t.Fatalf("read secondary Safari cookie snapshot: records=%#v err=%v", records, err)
	}
	if got := fileDigest(t, workCookie); got != workCookieBefore {
		t.Fatal("secondary Safari cookie source was mutated")
	}
	if profile := byLabel["No cookies yet"]; profile.Available || profile.State != ProfileStateNoData {
		t.Fatalf("missing secondary store must be explicit no-data: %#v", profile)
	}
	if _, ok := byLabel["Deleted"]; ok {
		t.Fatal("deleted Safari profile metadata must not be listed")
	}
}

func TestListSafariProfilesRejectsSymlinkSafariTabsDatabase(t *testing.T) {
	root := t.TempDir()
	realDatabase := filepath.Join(root, "real.db")
	writeSafariTabsFixture(t, realDatabase, []safariProfileFixtureRow{{identifier: "DefaultProfile"}}, false)
	tabsDatabase := filepath.Join(root, "SafariTabs.db")
	if err := os.Symlink(realDatabase, tabsDatabase); err != nil {
		t.Fatal(err)
	}
	defaultCookie := filepath.Join(root, "Cookies.binarycookies")
	writeSafariCookieFixture(t, defaultCookie)

	profiles := listSafariProfilesAtLocations(safariProfileLocations{
		tabsDatabase:       tabsDatabase,
		defaultCookiePaths: []string{defaultCookie},
		identityRoot:       root,
	})
	if len(profiles) != 2 || !profiles[0].Available || profiles[1].Available || profiles[1].State != ProfileStateInvalidData {
		t.Fatalf("symlink SafariTabs.db must preserve default and fail other profiles closed: %#v", profiles)
	}
}

func TestListSafariProfilesRejectsSymlinkSecondaryCookieStore(t *testing.T) {
	root := t.TempDir()
	tabsDatabase := filepath.Join(root, "SafariTabs.db")
	identifier := "25237EC2-1111-4222-8333-123456789ABC"
	writeSafariTabsFixture(t, tabsDatabase, []safariProfileFixtureRow{
		{identifier: "DefaultProfile"},
		{title: "Work", identifier: identifier},
	}, false)
	defaultCookie := filepath.Join(root, "DefaultCookies.binarycookies")
	writeSafariCookieFixture(t, defaultCookie)
	target := filepath.Join(root, "cookie-target")
	writeSafariCookieFixture(t, target)
	secondaryCookie := filepath.Join(root, "stores", strings.ToLower(identifier), "Cookies", "Cookies.binarycookies")
	if err := os.MkdirAll(filepath.Dir(secondaryCookie), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, secondaryCookie); err != nil {
		t.Fatal(err)
	}

	profiles := listSafariProfilesAtLocations(safariProfileLocations{
		tabsDatabase:         tabsDatabase,
		defaultCookiePaths:   []string{defaultCookie},
		websiteDataStoreRoot: filepath.Join(root, "stores"),
		identityRoot:         root,
	})
	if len(profiles) != 2 {
		t.Fatalf("unexpected profiles: %#v", profiles)
	}
	for _, profile := range profiles {
		if profile.Label == "Work" && (profile.Available || profile.State != ProfileStateInvalidData) {
			t.Fatalf("symlink secondary cookie store must fail closed: %#v", profile)
		}
	}
}

func TestListSafariProfilesRejectsOversizedSecondaryCookieStore(t *testing.T) {
	root := t.TempDir()
	tabsDatabase := filepath.Join(root, "SafariTabs.db")
	identifier := "25237EC2-1111-4222-8333-123456789ABC"
	writeSafariTabsFixture(t, tabsDatabase, []safariProfileFixtureRow{
		{identifier: "DefaultProfile"},
		{title: "Work", identifier: identifier},
	}, false)
	cookiePath := filepath.Join(root, "stores", strings.ToLower(identifier), "Cookies", "Cookies.binarycookies")
	if err := os.MkdirAll(filepath.Dir(cookiePath), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(cookiePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(safariCookieCopyLimit + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	profiles := listSafariProfilesAtLocations(safariProfileLocations{
		tabsDatabase:         tabsDatabase,
		defaultCookiePaths:   []string{filepath.Join(root, "missing-default")},
		websiteDataStoreRoot: filepath.Join(root, "stores"),
		identityRoot:         root,
	})
	for _, profile := range profiles {
		if profile.Label == "Work" && (profile.Available || profile.State != ProfileStateInvalidData) {
			t.Fatalf("oversized secondary cookie store must fail closed: %#v", profile)
		}
	}
}

func TestListSafariProfilesMarksMalformedMetadataWithoutLeakingIt(t *testing.T) {
	root := t.TempDir()
	tabsDatabase := filepath.Join(root, "SafariTabs.db")
	writeSafariTabsFixture(t, tabsDatabase, []safariProfileFixtureRow{
		{identifier: "DefaultProfile"},
		{title: "Broken", identifier: "not-a-profile-uuid"},
	}, false)
	defaultCookie := filepath.Join(root, "Cookies.binarycookies")
	writeSafariCookieFixture(t, defaultCookie)

	profiles := listSafariProfilesAtLocations(safariProfileLocations{
		tabsDatabase:       tabsDatabase,
		defaultCookiePaths: []string{defaultCookie},
		identityRoot:       root,
	})
	if len(profiles) != 2 || profiles[1].Label != "Other Profiles" || profiles[1].State != ProfileStateInvalidData {
		t.Fatalf("malformed profile metadata must produce a stable restricted state: %#v", profiles)
	}
	payload, err := json.Marshal(profiles[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), root) || strings.Contains(string(payload), "not-a-profile-uuid") {
		t.Fatalf("restricted profile leaked source metadata: %s", payload)
	}
}

func TestListSafariProfilesFallsBackToStableOpaqueWebsiteDataStoreLabels(t *testing.T) {
	root := t.TempDir()
	storeRoot := filepath.Join(root, "WebsiteDataStore")
	tabsDatabase := filepath.Join(root, "SafariTabs.db")
	database, err := sqlite3driver.Open(tabsDatabase)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("CREATE TABLE bookmarks (id INTEGER PRIMARY KEY, subtype INTEGER)"); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	identifiers := []string{
		"341b650c-2222-4333-8444-abcdef123456",
		"25237ec2-1111-4222-8333-123456789abc",
	}
	// Create in reverse lexical order to prove labels do not depend on
	// filesystem enumeration order.
	for _, identifier := range identifiers {
		writeSafariCookieFixture(t, filepath.Join(storeRoot, identifier, "Cookies", "Cookies.binarycookies"))
	}
	profiles := listSafariProfilesAtLocations(safariProfileLocations{
		tabsDatabase:         tabsDatabase,
		defaultCookiePaths:   []string{filepath.Join(root, "missing-default-cookies")},
		websiteDataStoreRoot: storeRoot,
		identityRoot:         root,
	})
	if len(profiles) != 3 {
		t.Fatalf("expected default plus two directory-fallback profiles, got %#v", profiles)
	}
	byLabel := make(map[string]Profile, len(profiles))
	for _, profile := range profiles {
		byLabel[profile.Label] = profile
		payload, err := json.Marshal(profile)
		if err != nil {
			t.Fatal(err)
		}
		serialized := string(payload)
		for _, privateValue := range append([]string{root}, identifiers...) {
			if strings.Contains(serialized, privateValue) {
				t.Fatalf("fallback Safari profile leaked UUID or path: %s", serialized)
			}
		}
	}
	if !byLabel["Profile 1"].Available || !byLabel["Profile 2"].Available {
		t.Fatalf("expected stable generic fallback labels, got %#v", profiles)
	}
	if !strings.Contains(byLabel["Profile 1"].snapshotFile, identifiers[1]) || !strings.Contains(byLabel["Profile 2"].snapshotFile, identifiers[0]) {
		t.Fatalf("generic labels are not assigned in stable UUID order: %#v", profiles)
	}
}
