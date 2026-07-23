package browserprofile

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"xiadown/internal/application/browsercdp"
)

func TestCookieAccessErrorStateSurvivesWrapping(t *testing.T) {
	accessErr := protectedCookieAccessError("chrome")
	if state := CookieAccessErrorState(fmt.Errorf("read profile: %w", accessErr)); state != CookieAccessStateAccessRequired {
		t.Fatalf("wrapped Chrome cookie access state = %q", state)
	}
	if state := CookieAccessErrorState(protectedCookieAccessError("edge")); state != CookieAccessStateProtectedUnsupported {
		t.Fatalf("Edge cookie access state = %q", state)
	}
	if state := CookieAccessErrorState(errors.New("unrelated")); state != "" {
		t.Fatalf("unrelated error exposed cookie access state %q", state)
	}
}

func TestChromiumCookieDatabaseHasV20ReadsOnlyProtectionMetadata(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		cipherHex string
		want      bool
	}{
		{name: "app-bound", cipherHex: "76323001020304", want: true},
		{name: "legacy", cipherHex: "76313001020304", want: false},
		{name: "empty", cipherHex: "", want: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Cookies")
			writeProtectedCookieDatabaseFixture(t, path, ".youtube.com", testCase.cipherHex)
			got, err := chromiumCookieDatabaseHasV20(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != testCase.want {
				t.Fatalf("v20 protection detected = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestChromiumV20DetectionRunsAfterDomainMinimization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Cookies")
	writeProtectedCookieDatabaseRowsFixture(t, path, []protectedCookieFixtureRow{
		{host: ".youtube.com", cipherHex: "76313001020304"},
		{host: ".private.example", cipherHex: "76323001020304"},
	})
	if err := minimizeChromiumCookieDatabase(context.Background(), path, []string{"youtube.com"}); err != nil {
		t.Fatal(err)
	}
	hasV20, err := chromiumCookieDatabaseHasV20(path)
	if err != nil {
		t.Fatal(err)
	}
	if hasV20 {
		t.Fatal("a disallowed v20 row survived staged allowlist minimization")
	}
}

func TestChromiumV20ProtectionProbeIsScopedToAllowedDomains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Cookies")
	writeProtectedCookieDatabaseRowsFixture(t, path, []protectedCookieFixtureRow{
		{host: ".youtube.com", cipherHex: "76313001020304"},
		{host: ".private.example", cipherHex: "76323001020304"},
	})
	hasV20, err := chromiumCookieDatabaseHasV20ForDomains(path, []string{"youtube.com"})
	if err != nil {
		t.Fatal(err)
	}
	if hasV20 {
		t.Fatal("a v20 cookie outside the allowlist blocked direct profile access")
	}

	path = filepath.Join(t.TempDir(), "Cookies")
	writeProtectedCookieDatabaseRowsFixture(t, path, []protectedCookieFixtureRow{
		{host: ".accounts.youtube.com", cipherHex: "76323001020304"},
		{host: ".private.example", cipherHex: "76313001020304"},
	})
	hasV20, err = chromiumCookieDatabaseHasV20ForDomains(path, []string{"youtube.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasV20 {
		t.Fatal("a v20 cookie on an allowlisted subdomain was not detected")
	}
}

func TestSnapshotCookiesRejectsStagedV20BeforeGatewayOrBrowserLaunch(t *testing.T) {
	previousSnapshotRoot := snapshotRoot
	snapshotBase := t.TempDir()
	snapshotRoot = func() string { return filepath.Join(snapshotBase, "snapshots") }
	t.Cleanup(func() { snapshotRoot = previousSnapshotRoot })

	for _, testCase := range []struct {
		browserID string
		wantState string
	}{
		{browserID: "chrome", wantState: CookieAccessStateAccessRequired},
		{browserID: "edge", wantState: CookieAccessStateProtectedUnsupported},
	} {
		t.Run(testCase.browserID, func(t *testing.T) {
			root := t.TempDir()
			writeBrowserProfileFixture(t, filepath.Join(root, "Local State"), `{}`)
			writeProtectedCookieDatabaseFixture(
				t,
				filepath.Join(root, "Default", "Network", "Cookies"),
				".youtube.com",
				"76323001020304",
			)
			executable := filepath.Join(t.TempDir(), testCase.browserID+".exe")
			writeBrowserProfileFixture(t, executable, "test executable")
			identity := browsercdp.ExecutableIdentityForCandidate(browsercdp.Candidate{
				ID:        browsercdp.BrowserID(testCase.browserID),
				ExecPath:  executable,
				Available: true,
			}, "")
			profile := Profile{
				BrowserID:    testCase.browserID,
				Available:    true,
				userDataRoot: root,
				relativeDir:  "Default",
				executable:   identity,
			}
			_, err := SnapshotCookies(context.Background(), profile, []string{"youtube.com"}, nil)
			if state := CookieAccessErrorState(err); state != testCase.wantState {
				t.Fatalf("SnapshotCookies error/state = %v/%q, want %q", err, state, testCase.wantState)
			}
		})
	}
}

type protectedCookieFixtureRow struct {
	host      string
	cipherHex string
}

func writeProtectedCookieDatabaseFixture(t *testing.T, path string, host string, cipherHex string) {
	t.Helper()
	writeProtectedCookieDatabaseRowsFixture(t, path, []protectedCookieFixtureRow{{host: host, cipherHex: cipherHex}})
}

func writeProtectedCookieDatabaseRowsFixture(t *testing.T, path string, rows []protectedCookieFixtureRow) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := sqlite3driver.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE cookies(
		host_key TEXT NOT NULL,
		name TEXT NOT NULL,
		value TEXT NOT NULL,
		encrypted_value BLOB NOT NULL
	)`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	for index, row := range rows {
		ciphertext, err := hex.DecodeString(row.cipherHex)
		if err != nil {
			_ = database.Close()
			t.Fatal(err)
		}
		if _, err := database.Exec(
			`INSERT INTO cookies(host_key, name, value, encrypted_value) VALUES (?, ?, '', ?)`,
			row.host,
			fmt.Sprintf("cookie-%d", index),
			ciphertext,
		); err != nil {
			_ = database.Close()
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
}
