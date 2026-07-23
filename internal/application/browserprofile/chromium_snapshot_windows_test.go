//go:build windows

package browserprofile

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlite3 "github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func TestSQLiteFileURIKeepsWindowsDriveOutOfAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser snapshot", "Cookies")
	uri, err := sqliteFileURI(path, "rw")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "file" || parsed.Host != "" || parsed.Opaque == "" {
		t.Fatalf("Windows SQLite URI = %q, want local opaque file URI with empty authority", uri)
	}
	wantPrefix := filepath.ToSlash(filepath.VolumeName(path)) + "/"
	if !strings.HasPrefix(strings.ToLower(parsed.Opaque), strings.ToLower(wantPrefix)) {
		t.Fatalf("Windows SQLite URI path = %q, want prefix %q", parsed.Opaque, wantPrefix)
	}
}

func TestSQLiteFileURIAllowsWindowsOnlineBackup(t *testing.T) {
	db, err := sqlite3.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Exec("CREATE TABLE fixture (value TEXT)"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "browser snapshot", "Cookies")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	targetURI, err := sqliteFileURI(target, "rw")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Backup("main", targetURI); err != nil {
		t.Fatalf("online backup to %q: %v", targetURI, err)
	}
}
