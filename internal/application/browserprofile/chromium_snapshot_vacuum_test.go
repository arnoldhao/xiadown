package browserprofile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// Chrome's production cookie database has substantially more indexes and
// columns than the small fixtures used by profile discovery tests. Keep a
// representative schema here because VACUUM must run after every prepared
// pruning statement has been finalized (see the production regression where
// SQLite returned "cannot VACUUM - SQL statements in progress").
func TestMinimizeChromiumCookieDatabaseCompactsChromeSchema(t *testing.T) {
	contexts := []struct {
		name string
		new  func() (context.Context, context.CancelFunc)
	}{
		{
			name: "cancellable",
			new: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
		},
		{
			name: "deadline",
			new: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 30*time.Second)
			},
		},
	}
	for _, testCase := range contexts {
		t.Run(testCase.name, func(t *testing.T) {
			testMinimizeChromiumCookieDatabaseCompactsChromeSchema(t, testCase.new)
		})
	}
}

func testMinimizeChromiumCookieDatabaseCompactsChromeSchema(
	t *testing.T,
	newContext func() (context.Context, context.CancelFunc),
) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Cookies")
	database, err := sqlite3driver.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE meta(key LONGVARCHAR NOT NULL UNIQUE PRIMARY KEY, value LONGVARCHAR)`,
		`CREATE TABLE cookies(
			creation_utc INTEGER NOT NULL,
			host_key TEXT NOT NULL,
			top_frame_site_key TEXT NOT NULL,
			name TEXT NOT NULL,
			value TEXT NOT NULL,
			encrypted_value BLOB NOT NULL,
			path TEXT NOT NULL,
			expires_utc INTEGER NOT NULL,
			is_secure INTEGER NOT NULL,
			is_httponly INTEGER NOT NULL,
			last_access_utc INTEGER NOT NULL,
			has_expires INTEGER NOT NULL DEFAULT 1,
			is_persistent INTEGER NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 1,
			samesite INTEGER NOT NULL DEFAULT -1,
			source_scheme INTEGER NOT NULL DEFAULT 0,
			source_port INTEGER NOT NULL DEFAULT -1,
			last_update_utc INTEGER NOT NULL DEFAULT 0,
			source_type INTEGER NOT NULL DEFAULT 0,
			has_cross_site_ancestor INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE UNIQUE INDEX cookies_unique_index ON cookies(
			host_key, top_frame_site_key, has_cross_site_ancestor,
			name, path, source_scheme, source_port
		)`,
		`INSERT INTO meta(key, value) VALUES ('version', '24')`,
		`INSERT INTO cookies(
			creation_utc, host_key, top_frame_site_key, name, value,
			encrypted_value, path, expires_utc, is_secure, is_httponly,
			last_access_utc
		) VALUES
			(1, '.youtube.com', '', 'allowed', '', x'763230736563726574', '/', 0, 1, 1, 1),
			(2, '.private.example', '', 'denied', '', x'76323064656e696564', '/', 0, 1, 1, 2)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			_ = database.Close()
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := newContext()
	defer cancel()
	if err := minimizeChromiumCookieDatabase(ctx, path, []string{"youtube.com"}); err != nil {
		t.Fatalf("minimize Chrome cookie schema: %v", err)
	}
	database, err = sqlite3driver.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cookies`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("minimized cookie count = %d, want 1", count)
	}
	var host string
	if err := database.QueryRow(`SELECT host_key FROM cookies`).Scan(&host); err != nil {
		t.Fatal(err)
	}
	if host != ".youtube.com" {
		t.Fatalf("minimized cookie host = %q", host)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(path + suffix); !os.IsNotExist(err) {
			t.Fatalf("minimized database retained sidecar %q: %v", suffix, err)
		}
	}
}
