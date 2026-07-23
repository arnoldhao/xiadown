package browserprofile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite3 "github.com/ncruces/go-sqlite3"
	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

const chromiumSnapshotBusyTimeout = 3 * time.Second

const chromiumProtectionProbeBusyTimeout = 250 * time.Millisecond

// snapshotChromiumCookieDatabase uses SQLite's online backup API rather than
// copying the main database and WAL/SHM files independently. The source is
// opened read-only and a read transaction pins one coherent WAL snapshot for
// both the size check and backup.
func snapshotChromiumCookieDatabase(source string, destination string) (err error) {
	sourceInfo, err := validateChromiumSQLiteSource(source)
	if err != nil {
		return err
	}
	if err := validateChromiumSQLiteSidecars(source); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("browser cookie snapshot destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(destination)
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(destination)
		}
	}()

	sourcePath, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	connection, err := sqlite3.OpenFlags(
		sourcePath,
		sqlite3.OPEN_READONLY|sqlite3.OPEN_PRIVATECACHE,
	)
	if err != nil {
		return fmt.Errorf("open read-only browser cookie database: %w", err)
	}
	defer connection.Close()
	if readOnly, missing := connection.ReadOnly("main"); !readOnly || missing {
		return fmt.Errorf("browser cookie database did not open read-only")
	}
	if err := connection.BusyTimeout(chromiumSnapshotBusyTimeout); err != nil {
		return fmt.Errorf("configure browser cookie snapshot timeout: %w", err)
	}
	if err := connection.Exec("BEGIN DEFERRED"); err != nil {
		return fmt.Errorf("begin browser cookie snapshot: %w", err)
	}
	transactionOpen := true
	defer func() {
		if transactionOpen {
			_ = connection.Exec("ROLLBACK")
		}
	}()

	// Reading sqlite_schema establishes the deferred transaction's snapshot
	// before page_count is measured and the online backup starts.
	if err := stepSQLiteStatement(connection, "SELECT 1 FROM sqlite_schema LIMIT 1"); err != nil {
		return fmt.Errorf("establish browser cookie snapshot: %w", err)
	}
	pageSize, err := sqlitePragmaInt64(connection, "PRAGMA page_size")
	if err != nil {
		return err
	}
	pageCount, err := sqlitePragmaInt64(connection, "PRAGMA page_count")
	if err != nil {
		return err
	}
	if pageSize < 512 || pageSize > 65536 || pageSize&(pageSize-1) != 0 || pageCount <= 0 || pageCount > browserProfileCopyLimit/pageSize {
		return fmt.Errorf("browser cookie database exceeds safe snapshot limit")
	}

	destinationURI, err := sqliteFileURI(destination, "rw")
	if err != nil {
		return err
	}
	if err := connection.Backup("main", destinationURI); err != nil {
		return fmt.Errorf("create consistent browser cookie snapshot: %w", err)
	}
	if err := connection.Exec("ROLLBACK"); err != nil {
		return fmt.Errorf("finish browser cookie snapshot transaction: %w", err)
	}
	transactionOpen = false

	currentSource, err := os.Lstat(source)
	if err != nil || !currentSource.Mode().IsRegular() || currentSource.Mode()&os.ModeSymlink != 0 || !os.SameFile(sourceInfo, currentSource) {
		return fmt.Errorf("browser cookie source changed during snapshot")
	}
	if err := validateChromiumSQLiteSidecars(source); err != nil {
		return err
	}
	destinationInfo, err := os.Lstat(destination)
	if err != nil {
		return err
	}
	if !destinationInfo.Mode().IsRegular() || destinationInfo.Mode()&os.ModeSymlink != 0 || destinationInfo.Size() < 16 || destinationInfo.Size() > browserProfileCopyLimit {
		return fmt.Errorf("browser cookie snapshot is invalid")
	}
	if err := os.Chmod(destination, 0o600); err != nil {
		return err
	}
	return nil
}

// minimizeChromiumCookieDatabase removes every cookie outside the normalized
// allowlist before Chromium is launched. It operates only on the staged online
// backup, switches it out of WAL mode, enables secure deletion, then VACUUMs so
// deleted values are not retained in free pages.
func minimizeChromiumCookieDatabase(ctx context.Context, path string, domains []string) (err error) {
	allowedDomains, err := normalizeSnapshotDomains(domains)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 16 || info.Size() > browserProfileCopyLimit {
		return fmt.Errorf("staged browser cookie database is invalid")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	database, err := sqlite3driver.Open(path, func(connection *sqlite3.Conn) error {
		return connection.BusyTimeout(chromiumSnapshotBusyTimeout)
	})
	if err != nil {
		return fmt.Errorf("open staged browser cookie database: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	closed := false
	defer func() {
		if !closed {
			_ = database.Close()
		}
	}()

	var journalMode string
	if err := database.QueryRowContext(ctx, "PRAGMA journal_mode=DELETE").Scan(&journalMode); err != nil {
		return fmt.Errorf("disable staged browser cookie WAL: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(journalMode), "delete") {
		return fmt.Errorf("staged browser cookie database retained WAL mode")
	}
	var secureDelete int
	if err := database.QueryRowContext(ctx, "PRAGMA secure_delete=ON").Scan(&secureDelete); err != nil {
		return fmt.Errorf("enable secure browser cookie deletion: %w", err)
	}
	if secureDelete == 0 {
		return fmt.Errorf("secure browser cookie deletion is unavailable")
	}

	predicate, args := chromiumDomainPredicate(allowedDomains)
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin browser cookie minimization: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback()
		}
	}()
	if _, err := transaction.ExecContext(ctx, "DELETE FROM cookies WHERE host_key IS NULL OR NOT ("+predicate+")", args...); err != nil {
		return fmt.Errorf("minimize staged browser cookies: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit browser cookie minimization: %w", err)
	}
	committed = true
	// Finish pruning and close its database/sql connection before compaction so
	// all prepared statements and the driver's context interrupt guard are
	// finalized. VACUUM runs on a separate raw connection because SQLite requires
	// its VDBE to be the only active statement on that connection.
	if err := database.Close(); err != nil {
		return fmt.Errorf("close pruned browser cookie database: %w", err)
	}
	closed = true
	connection, err := sqlite3.OpenFlags(
		path,
		sqlite3.OPEN_READWRITE|sqlite3.OPEN_PRIVATECACHE,
	)
	if err != nil {
		return fmt.Errorf("reopen pruned browser cookie database: %w", err)
	}
	connectionClosed := false
	defer func() {
		if !connectionClosed {
			_ = connection.Close()
		}
	}()
	if err := connection.BusyTimeout(chromiumSnapshotBusyTimeout); err != nil {
		return fmt.Errorf("configure browser cookie compaction timeout: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Do not call sqlite3.Conn.SetInterrupt before VACUUM. ncruces implements
	// cancellable contexts by preparing and stepping a persistent recursive CTE
	// (Conn.pending), deliberately keeping one VDBE active so a later interrupt
	// cannot be lost. SQLite requires the VACUUM VDBE to be the only active one,
	// so that guard deterministically produces "SQL statements in progress" for
	// every ordinary cancellable Wails context. This local snapshot is bounded to
	// browserProfileCopyLimit; finish its secure compaction synchronously, then
	// honor cancellation before returning or exposing the snapshot to Chromium.
	if err := connection.Exec("VACUUM"); err != nil {
		return fmt.Errorf("compact minimized browser cookie database: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	disallowed, err := countDisallowedChromiumCookies(connection, predicate, args)
	if err != nil {
		return fmt.Errorf("verify minimized browser cookies: %w", err)
	}
	if disallowed != 0 {
		return fmt.Errorf("staged browser cookie minimization was incomplete")
	}
	if err := connection.Close(); err != nil {
		return fmt.Errorf("close minimized browser cookie database: %w", err)
	}
	connectionClosed = true

	info, err = os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 16 || info.Size() > browserProfileCopyLimit {
		return fmt.Errorf("minimized browser cookie database is invalid")
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		sidecar := path + suffix
		sidecarInfo, statErr := os.Lstat(sidecar)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return statErr
		}
		if !sidecarInfo.Mode().IsRegular() || sidecarInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("staged browser cookie sidecar is invalid")
		}
		if err := os.Remove(sidecar); err != nil {
			return err
		}
	}
	return nil
}

func countDisallowedChromiumCookies(connection *sqlite3.Conn, predicate string, args []any) (int64, error) {
	if connection == nil {
		return 0, fmt.Errorf("browser cookie database is unavailable")
	}
	statement, tail, err := connection.Prepare(
		"SELECT COUNT(*) FROM cookies WHERE host_key IS NULL OR NOT (" + predicate + ")",
	)
	if err != nil {
		return 0, err
	}
	defer statement.Close()
	if strings.TrimSpace(tail) != "" {
		return 0, fmt.Errorf("browser cookie verification contains trailing SQL")
	}
	for index, argument := range args {
		value, ok := argument.(string)
		if !ok {
			return 0, fmt.Errorf("browser cookie verification argument is invalid")
		}
		if err := statement.BindText(index+1, value); err != nil {
			return 0, err
		}
	}
	if !statement.Step() {
		if err := statement.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("browser cookie verification result is unavailable")
	}
	result := statement.ColumnInt64(0)
	if err := statement.Err(); err != nil {
		return 0, err
	}
	return result, nil
}

// chromiumCookieDatabaseHasV20 checks only the already-minimized staged
// database. A v20 row was encrypted by Chromium's Windows App-Bound provider;
// launching that row through a non-default temporary user-data directory makes
// its key deliberately unavailable.
func chromiumCookieDatabaseHasV20(path string) (bool, error) {
	return chromiumCookieDatabaseHasV20Where(
		path,
		"SELECT 1 FROM cookies WHERE substr(encrypted_value, 1, 3) = x'763230' LIMIT 1",
		nil,
		false,
	)
}

// chromiumCookieDatabaseHasV20ForDomains reads only enough protection metadata
// to answer whether a cookie in the fixed allowlist uses Windows Chromium's
// v20 format. The query returns one constant and never selects a host, name,
// value, or ciphertext.
func chromiumCookieDatabaseHasV20ForDomains(path string, domains []string) (bool, error) {
	allowedDomains, err := normalizeSnapshotDomains(domains)
	if err != nil {
		return false, err
	}
	predicate, args := chromiumDomainPredicate(allowedDomains)
	return chromiumCookieDatabaseHasV20Where(
		path,
		"SELECT 1 FROM cookies WHERE ("+predicate+") AND substr(encrypted_value, 1, 3) = x'763230' LIMIT 1",
		args,
		true,
	)
}

func chromiumCookieDatabaseHasV20Where(
	path string,
	query string,
	args []any,
	validateSidecars bool,
) (bool, error) {
	info, err := validateChromiumSQLiteSource(path)
	if err != nil {
		return false, err
	}
	if validateSidecars {
		if err := validateChromiumSQLiteSidecars(path); err != nil {
			return false, err
		}
	}
	connection, err := sqlite3.OpenFlags(path, sqlite3.OPEN_READONLY|sqlite3.OPEN_PRIVATECACHE)
	if err != nil {
		return false, fmt.Errorf("open browser cookie database for protection check: %w", err)
	}
	defer connection.Close()
	if readOnly, missing := connection.ReadOnly("main"); !readOnly || missing {
		return false, fmt.Errorf("browser cookie protection check did not open read-only")
	}
	if err := connection.BusyTimeout(chromiumProtectionProbeBusyTimeout); err != nil {
		return false, fmt.Errorf("configure browser cookie protection timeout: %w", err)
	}
	statement, tail, err := connection.Prepare(query)
	if err != nil {
		return false, fmt.Errorf("inspect browser cookie protection: %w", err)
	}
	defer statement.Close()
	if strings.TrimSpace(tail) != "" {
		return false, fmt.Errorf("browser cookie protection query contains trailing SQL")
	}
	for index, argument := range args {
		value, ok := argument.(string)
		if !ok {
			return false, fmt.Errorf("browser cookie protection argument is invalid")
		}
		if err := statement.BindText(index+1, value); err != nil {
			return false, fmt.Errorf("bind browser cookie protection argument: %w", err)
		}
	}
	hasV20 := statement.Step()
	if err := statement.Err(); err != nil {
		return false, fmt.Errorf("inspect browser cookie protection: %w", err)
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, current) {
		return false, fmt.Errorf("browser cookie database changed during protection check")
	}
	if validateSidecars {
		if err := validateChromiumSQLiteSidecars(path); err != nil {
			return false, err
		}
	}
	return hasV20, nil
}

func chromiumDomainPredicate(domains []string) (string, []any) {
	clauses := make([]string, 0, len(domains))
	args := make([]any, 0, len(domains)*2)
	for _, domain := range domains {
		clauses = append(clauses, "(lower(ltrim(host_key, '.')) = ? OR lower(ltrim(host_key, '.')) LIKE ?)")
		args = append(args, domain, "%."+domain)
	}
	return strings.Join(clauses, " OR "), args
}

func validateChromiumSQLiteSource(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 16 || info.Size() > browserProfileCopyLimit {
		return nil, fmt.Errorf("browser cookie source is not a valid regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	header := make([]byte, 16)
	_, readErr := io.ReadFull(file, header)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if string(header) != "SQLite format 3\x00" {
		return nil, fmt.Errorf("browser cookie source has an invalid header")
	}
	return info, nil
}

func validateChromiumSQLiteSidecars(source string) error {
	var total int64
	if info, err := os.Lstat(source); err == nil {
		total = info.Size()
	} else {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		info, err := os.Lstat(source + suffix)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > browserProfileCopyLimit {
			return fmt.Errorf("browser cookie sidecar is not a valid regular file")
		}
		if suffix == "-wal" {
			if total > browserProfileCopyLimit-info.Size() {
				return fmt.Errorf("browser cookie source exceeds safe snapshot limit")
			}
			total += info.Size()
		}
	}
	return nil
}

func sqliteFileURI(path string, mode string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	uriPath := filepath.ToSlash(absolute)
	if volume := filepath.VolumeName(absolute); volume != "" && !strings.HasPrefix(uriPath, "//") {
		// ncruces' Windows VFS accepts SQLite's drive-letter URI form
		// file:C:/Cookies. net/url's hierarchical form emits file://C:/Cookies
		// (making C: the authority), while file:///C:/Cookies is rejected by
		// this VFS. Escape only the path and keep it in URI-opaque form.
		return "file:" + (&url.URL{Path: uriPath}).EscapedPath() + "?mode=" + url.QueryEscape(mode), nil
	}
	uri := (&url.URL{Scheme: "file", Path: uriPath}).String()
	return uri + "?mode=" + url.QueryEscape(mode), nil
}

func sqlitePragmaInt64(connection *sqlite3.Conn, query string) (int64, error) {
	statement, _, err := connection.Prepare(query)
	if err != nil {
		return 0, err
	}
	defer statement.Close()
	if !statement.Step() {
		if err := statement.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("browser cookie database metadata is unavailable")
	}
	return statement.ColumnInt64(0), nil
}

func stepSQLiteStatement(connection *sqlite3.Conn, query string) error {
	statement, _, err := connection.Prepare(query)
	if err != nil {
		return err
	}
	defer statement.Close()
	_ = statement.Step()
	return statement.Err()
}
