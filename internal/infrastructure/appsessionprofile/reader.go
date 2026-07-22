package appsessionprofile

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/browserprofile"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

const (
	maxCookieRows         = 100_000
	maxCookieDatabaseSize = 512 << 20
)

type keychainItem struct {
	service string
	account string
}

func (reader *Reader) ReadCurrentBrowserCookies(
	ctx context.Context,
	browserID string,
	domains []string,
) ([]appcookies.Record, error) {
	if reader == nil {
		return nil, appsessions.ErrUnsupported
	}
	if ctx == nil {
		ctx = context.Background()
	}
	browserID = strings.ToLower(strings.TrimSpace(browserID))
	if browserID != string(browsercdp.BrowserChrome) {
		return nil, appsessions.ErrUnsupported
	}
	runtimeBrowser, err := browsercdp.StartBorrowedCurrentBrowser(ctx, browserID)
	if err != nil {
		return nil, currentBrowserCookieReadError(err)
	}
	defer runtimeBrowser.Stop()
	records, err := browsercdp.SnapshotBorrowedCookiesForDomains(ctx, runtimeBrowser, domains)
	if err != nil {
		return nil, currentBrowserCookieReadError(err)
	}
	records = appcookies.FilterByDomains(records, domains)
	if len(records) == 0 {
		return nil, appsessions.ErrNoCookies
	}
	return records, nil
}

func currentBrowserCookieReadError(err error) error {
	if err == nil {
		return nil
	}
	switch browsercdp.CurrentBrowserErrorState(err) {
	case browsercdp.CurrentBrowserStateUnsupportedBrowser,
		browsercdp.CurrentBrowserStateUnsupportedVersion,
		browsercdp.CurrentBrowserStateNotInstalled:
		return fmt.Errorf("%w: current browser is unsupported", appsessions.ErrBrowserCookieProtected)
	case browsercdp.CurrentBrowserStateNotRunning,
		browsercdp.CurrentBrowserStateRemoteDebuggingDisabled,
		browsercdp.CurrentBrowserStatePermissionDenied,
		browsercdp.CurrentBrowserStateEndpointUnavailable:
		return fmt.Errorf("%w: current browser consent is unavailable", appsessions.ErrBrowserCookieAccessRequired)
	}
	if errors.Is(err, browsercdp.ErrBorrowedCookieRuntimeUnavailable) ||
		errors.Is(err, browsercdp.ErrBorrowedCookieTargetUnavailable) {
		return fmt.Errorf("%w: current browser target is unavailable", appsessions.ErrBrowserCookieAccessRequired)
	}
	return err
}

type browserDefinition struct {
	id            string
	root          string
	localState    string
	keychainItems []keychainItem
}

type Reader struct {
	networkGateway browserprofile.NetworkGateway
}

func New(gateways ...browserprofile.NetworkGateway) *Reader {
	reader := &Reader{}
	if len(gateways) > 0 {
		reader.networkGateway = gateways[0]
	}
	return reader
}

func (reader *Reader) ReadBrowserProfileCookies(
	ctx context.Context,
	browserID string,
	profileID string,
	domains []string,
) ([]appcookies.Record, error) {
	if reader == nil {
		return nil, appsessions.ErrUnsupported
	}
	if ctx == nil {
		ctx = context.Background()
	}
	profileID = strings.TrimSpace(profileID)
	if strings.HasPrefix(profileID, "profile-") {
		profile, err := browserprofile.Resolve(browserID, profileID)
		if err != nil {
			return nil, appsessions.ErrInvalidSession
		}
		records, err := browserprofile.SnapshotCookies(ctx, profile, domains, reader.networkGateway)
		if err != nil {
			switch browserprofile.CookieAccessErrorState(err) {
			case browserprofile.CookieAccessStateAccessRequired:
				return nil, fmt.Errorf("%w: current browser consent is required", appsessions.ErrBrowserCookieAccessRequired)
			case browserprofile.CookieAccessStateProtectedUnsupported:
				return nil, fmt.Errorf("%w: browser app-bound encryption", appsessions.ErrBrowserCookieProtected)
			}
			return nil, fmt.Errorf("read browser profile snapshot: %w", err)
		}
		records = appcookies.FilterByDomains(records, domains)
		if len(records) == 0 {
			return nil, appsessions.ErrNoCookies
		}
		return records, nil
	}
	definition, err := platformBrowserDefinition(strings.ToLower(strings.TrimSpace(browserID)))
	if err != nil {
		return nil, err
	}
	profileDir, cookiePath, err := resolveProfileCookieDatabase(definition, profileID)
	if err != nil {
		return nil, err
	}
	snapshot, cleanup, err := snapshotCookieDatabase(cookiePath)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	database, err := sqlite3driver.Open(snapshot, nil)
	if err != nil {
		return nil, fmt.Errorf("open browser cookie snapshot: %w", err)
	}
	defer database.Close()

	query, args := browserCookieQuery(domains)
	if query == "" {
		return nil, appsessions.ErrInvalidSession
	}
	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query browser cookie snapshot: %w", err)
	}
	defer rows.Close()

	var decryptor cookieDecryptor
	var decryptorErr error
	result := make([]appcookies.Record, 0, 128)
	decryptFailures := 0
	rowCount := 0
	for rows.Next() {
		rowCount++
		if rowCount > maxCookieRows {
			return nil, fmt.Errorf("browser profile cookie limit exceeded")
		}
		var (
			hostKey, name, value, path string
			encryptedValue             []byte
			expiresUTC                 int64
			secure, httpOnly, sameSite int
		)
		if err := rows.Scan(
			&hostKey, &name, &value, &encryptedValue, &path,
			&expiresUTC, &secure, &httpOnly, &sameSite,
		); err != nil {
			return nil, fmt.Errorf("scan browser cookie: %w", err)
		}
		if strings.TrimSpace(name) == "" {
			continue
		}
		if value == "" && len(encryptedValue) > 0 {
			if decryptor == nil && decryptorErr == nil {
				decryptor, decryptorErr = newPlatformCookieDecryptor(ctx, definition, profileDir)
			}
			if decryptorErr != nil {
				decryptFailures++
				continue
			}
			value, err = decryptor.Decrypt(hostKey, encryptedValue)
			if err != nil {
				decryptFailures++
				continue
			}
		}
		if value == "" {
			continue
		}
		if path == "" {
			path = "/"
		}
		result = append(result, appcookies.Record{
			Name:     name,
			Value:    value,
			Domain:   hostKey,
			Path:     path,
			Expires:  chromiumTimeToUnix(expiresUTC),
			Secure:   secure != 0,
			HttpOnly: httpOnly != 0,
			SameSite: chromiumSameSite(sameSite),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read browser cookies: %w", err)
	}
	if len(result) == 0 {
		if decryptFailures > 0 {
			if decryptorErr != nil {
				return nil, fmt.Errorf("decrypt browser cookies: %w", decryptorErr)
			}
			return nil, fmt.Errorf("browser cookies could not be decrypted")
		}
		return nil, appsessions.ErrNoCookies
	}
	return result, nil
}

type cookieDecryptor interface {
	Decrypt(host string, encrypted []byte) (string, error)
}

func resolveProfileCookieDatabase(definition browserDefinition, profileID string) (string, string, error) {
	root := filepath.Clean(strings.TrimSpace(definition.root))
	if root == "" || root == "." {
		return "", "", appsessions.ErrUnsupported
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", appsessions.ErrNoCookies
		}
		return "", "", fmt.Errorf("resolve browser profile root: %w", err)
	}
	if isXiaDownOwnedPath(resolvedRoot) {
		return "", "", appsessions.ErrUnsupported
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" || len(profileID) > 256 || strings.ContainsAny(profileID, "/\\\x00") || profileID == "." || profileID == ".." {
		return "", "", appsessions.ErrInvalidSession
	}
	candidates := []string{filepath.Join(resolvedRoot, profileID)}
	if strings.EqualFold(profileID, "default") {
		candidates = []string{filepath.Join(resolvedRoot, "Default"), resolvedRoot}
	}
	for _, candidate := range candidates {
		resolvedProfile, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr != nil {
			continue
		}
		if !pathWithin(resolvedRoot, resolvedProfile) || isXiaDownOwnedPath(resolvedProfile) {
			continue
		}
		for _, cookiePath := range []string{
			filepath.Join(resolvedProfile, "Network", "Cookies"),
			filepath.Join(resolvedProfile, "Cookies"),
		} {
			info, statErr := os.Lstat(cookiePath)
			if statErr == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				return resolvedProfile, cookiePath, nil
			}
		}
	}
	return "", "", appsessions.ErrNoCookies
}

func snapshotCookieDatabase(source string) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "xiadown-browser-profile-")
	if err != nil {
		return "", nil, err
	}
	_ = os.Chmod(tempDir, 0o700)
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	destination := filepath.Join(tempDir, "Cookies")
	if err := copyRegularFile(source, destination); err != nil {
		cleanup()
		return "", nil, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := copyRegularFile(source+suffix, destination+suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanup()
			return "", nil, err
		}
	}
	return destination, cleanup, nil
}

func copyRegularFile(source string, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("browser cookie source is not a regular file")
	}
	if info.Size() < 0 || info.Size() > maxCookieDatabaseSize {
		return fmt.Errorf("browser cookie source exceeds size limit")
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.CopyN(output, input, info.Size()+1)
	closeErr := output.Close()
	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		return copyErr
	}
	if written != info.Size() {
		return fmt.Errorf("browser cookie source changed while copying")
	}
	return closeErr
}

func browserCookieQuery(domains []string) (string, []any) {
	set := make(map[string]struct{})
	for _, domain := range domains {
		domain = strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
		if domain != "" && !strings.ContainsAny(domain, "%_\\") {
			set[domain] = struct{}{}
		}
	}
	normalized := make([]string, 0, len(set))
	for domain := range set {
		normalized = append(normalized, domain)
	}
	sort.Strings(normalized)
	if len(normalized) == 0 {
		return "", nil
	}
	clauses := make([]string, 0, len(normalized))
	args := make([]any, 0, len(normalized)*2)
	for _, domain := range normalized {
		clauses = append(clauses, "(lower(host_key) = ? OR lower(host_key) LIKE ?)")
		args = append(args, domain, "%."+domain)
	}
	query := `
SELECT host_key, name, value, encrypted_value, path, expires_utc,
	is_secure, is_httponly, samesite
FROM cookies
WHERE length(name) > 0 AND (` + strings.Join(clauses, " OR ") + `)
ORDER BY host_key, path, name
LIMIT 100001
`
	return query, args
}

func chromiumTimeToUnix(value int64) int64 {
	const windowsEpochOffsetSeconds int64 = 11_644_473_600
	if value <= 0 {
		return 0
	}
	seconds := value / 1_000_000
	if seconds <= windowsEpochOffsetSeconds {
		return 0
	}
	return seconds - windowsEpochOffsetSeconds
}

func chromiumSameSite(value int) string {
	switch value {
	case 0:
		return "none"
	case 1:
		return "lax"
	case 2:
		return "strict"
	default:
		return ""
	}
}

func stripHostDigest(host string, plaintext []byte) []byte {
	if len(plaintext) < sha256.Size {
		return plaintext
	}
	digest := sha256.Sum256([]byte(host))
	if string(plaintext[:sha256.Size]) == string(digest[:]) {
		return plaintext[sha256.Size:]
	}
	return plaintext
}

func pathWithin(root string, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func isXiaDownOwnedPath(candidate string) bool {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	resolvedConfig, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		resolvedConfig = filepath.Clean(configDir)
	}
	for _, name := range []string{"xiadown", "XiaDown"} {
		if pathWithin(filepath.Join(resolvedConfig, name), candidate) {
			return true
		}
	}
	return false
}
