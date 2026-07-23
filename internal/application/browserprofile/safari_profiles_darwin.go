//go:build darwin && !ios

package browserprofile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	sqlite3driver "github.com/ncruces/go-sqlite3/driver"
)

const (
	safariTabsDatabaseReadLimit = int64(2 << 30)
	safariTabsWALReadLimit      = int64(512 << 20)
	safariTabsSHMReadLimit      = int64(64 << 20)
	safariMaxProfileCount       = 128
	safariMaxProfileLabelBytes  = 512
	safariMaxDataStoreEntries   = 512
)

type safariProfileLocations struct {
	tabsDatabase         string
	defaultCookiePaths   []string
	websiteDataStoreRoot string
	identityRoot         string
}

type safariProfileMetadata struct {
	identifier string
	label      string
	isDefault  bool
}

func safariSource() (Source, bool) {
	const executable = "/Applications/Safari.app/Contents/MacOS/Safari"
	info, err := os.Lstat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Source{ID: "safari", Label: "Safari", Available: false, Error: "browser executable not found"}, true
	}
	return Source{ID: "safari", Label: "Safari", Available: true}, true
}

func listSafariProfiles() []Profile {
	home, err := os.UserHomeDir()
	if err != nil {
		return []Profile{unavailableProfile("safari", "safari", profileStateForError(err))}
	}
	containerLibrary := filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library")
	return listSafariProfilesAtLocations(safariProfileLocations{
		tabsDatabase: filepath.Join(containerLibrary, "Safari", "SafariTabs.db"),
		defaultCookiePaths: []string{
			filepath.Join(containerLibrary, "Cookies", "Cookies.binarycookies"),
			filepath.Join(home, "Library", "Cookies", "Cookies.binarycookies"),
		},
		// WebKit's public source defines identifier-backed persistent stores as
		// Library/WebKit/WebsiteDataStore/<canonical UUID> and the cookie file
		// inside each store as Cookies/Cookies.binarycookies. Safari uses its
		// own container Library as that base directory.
		websiteDataStoreRoot: filepath.Join(containerLibrary, "WebKit", "WebsiteDataStore"),
		identityRoot:         containerLibrary,
	})
}

// listSafariProfilesAtPaths preserves default-profile discovery for older
// Safari versions and focused cookie-source tests. Safari 17+ discovery uses
// listSafariProfilesAtLocations so profile metadata and stores stay paired.
func listSafariProfilesAtPaths(paths []string) []Profile {
	identityRoot := "safari"
	if len(paths) > 0 {
		identityRoot = filepath.Dir(paths[0])
	}
	return listSafariProfilesAtLocations(safariProfileLocations{
		defaultCookiePaths: paths,
		identityRoot:       identityRoot,
	})
}

func listSafariProfilesAtLocations(locations safariProfileLocations) []Profile {
	if strings.TrimSpace(locations.identityRoot) == "" {
		locations.identityRoot = "safari"
	}

	metadata, metadataState := readSafariProfileMetadata(locations.tabsDatabase)
	storeIdentifiers, storeState := listSafariWebsiteDataStores(locations.websiteDataStoreRoot)
	defaultLabel := "Default"
	metadataByIdentifier := make(map[string]safariProfileMetadata, len(metadata))
	for _, entry := range metadata {
		if entry.isDefault && strings.TrimSpace(entry.label) != "" {
			defaultLabel = entry.label
			continue
		}
		if !entry.isDefault {
			metadataByIdentifier[entry.identifier] = entry
		}
	}

	result := make([]Profile, 0, len(metadata)+1)
	result = append(result, safariCookieProfile(
		locations.identityRoot,
		"DefaultProfile",
		defaultLabel,
		true,
		locations.defaultCookiePaths,
	))

	identifierSet := make(map[string]struct{}, len(metadataByIdentifier)+len(storeIdentifiers))
	for identifier := range metadataByIdentifier {
		identifierSet[identifier] = struct{}{}
	}
	for _, identifier := range storeIdentifiers {
		identifierSet[identifier] = struct{}{}
	}
	identifiers := make([]string, 0, len(identifierSet))
	for identifier := range identifierSet {
		identifiers = append(identifiers, identifier)
	}
	sort.Strings(identifiers)
	genericIndex := 0
	for _, identifier := range identifiers {
		label := strings.TrimSpace(metadataByIdentifier[identifier].label)
		if label == "" {
			genericIndex++
			label = fmt.Sprintf("Profile %d", genericIndex)
		}
		result = append(result, safariWebsiteDataStoreProfile(
			locations.identityRoot,
			locations.websiteDataStoreRoot,
			identifier,
			label,
		))
	}

	// If neither the production metadata query nor WebKit's identifier store
	// directory can be inspected, keep one path-free unavailable row. When the
	// directory fallback succeeds, generic labels are preferable to surfacing a
	// private-schema failure or hiding usable stores.
	restrictedState := strongerSafariProfileState(metadataState, storeState)
	if len(identifiers) == 0 && (restrictedState == ProfileStatePermissionRequired || restrictedState == ProfileStateInvalidData) {
		result = append(result, Profile{
			ID:           profileIdentifier("safari", locations.identityRoot, ".profiles-unavailable"),
			BrowserID:    "safari",
			BrowserLabel: "Safari",
			Label:        "Other Profiles",
			Available:    false,
			State:        restrictedState,
			Error:        profileError(restrictedState),
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
	return result
}

func safariCookieProfile(identityRoot, identifier, label string, isDefault bool, paths []string) Profile {
	state := ProfileStateNoData
	snapshotFile := ""
	for _, cookiePath := range paths {
		candidateState := inspectSafariCookieSource(cookiePath)
		if candidateState == ProfileStateReady {
			snapshotFile = cookiePath
			state = ProfileStateReady
			break
		}
		state = strongerSafariProfileState(state, candidateState)
	}
	label = strings.TrimSpace(label)
	if label == "" {
		if isDefault {
			label = "Default"
		} else {
			label = "Profile"
		}
	}
	return newSafariProfile(identityRoot, identifier, label, isDefault, state, snapshotFile)
}

func newSafariProfile(identityRoot, identifier, label string, isDefault bool, state, snapshotFile string) Profile {
	return Profile{
		ID:           profileIdentifier("safari", identityRoot, identifier),
		BrowserID:    "safari",
		BrowserLabel: "Safari",
		Label:        label,
		IsDefault:    isDefault,
		Available:    state == ProfileStateReady,
		State:        state,
		Error:        profileError(state),
		snapshotFile: snapshotFile,
	}
}

func safariWebsiteDataStoreProfile(identityRoot, storeRoot, identifier, label string) Profile {
	cookiePath, state := safariWebsiteDataStoreCookieSource(storeRoot, identifier)
	if state != ProfileStateReady {
		return newSafariProfile(identityRoot, identifier, label, false, state, "")
	}
	state = inspectSafariCookieSource(cookiePath)
	snapshotFile := ""
	if state == ProfileStateReady {
		snapshotFile = cookiePath
	}
	return newSafariProfile(identityRoot, identifier, label, false, state, snapshotFile)
}

func safariWebsiteDataStoreCookieSource(root, identifier string) (string, string) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", ProfileStateNoData
	}
	parsed, err := uuid.Parse(identifier)
	if err != nil || parsed == uuid.Nil || identifier != parsed.String() {
		return "", ProfileStateInvalidData
	}
	for _, directory := range []string{root, filepath.Join(root, identifier), filepath.Join(root, identifier, "Cookies")} {
		info, err := os.Lstat(directory)
		if err != nil {
			return "", profileStateForError(err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", ProfileStateInvalidData
		}
	}
	return filepath.Join(root, identifier, "Cookies", "Cookies.binarycookies"), ProfileStateReady
}

func listSafariWebsiteDataStores(root string) ([]string, string) {
	root = strings.TrimSpace(root)
	if root == "" {
		return []string{}, ProfileStateNoData
	}
	info, err := os.Lstat(root)
	if err != nil {
		return []string{}, profileStateForError(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return []string{}, ProfileStateInvalidData
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return []string{}, profileStateForError(err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	state := ProfileStateReady
	if len(entries) > safariMaxDataStoreEntries {
		entries = entries[:safariMaxDataStoreEntries]
		state = ProfileStateInvalidData
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		parsed, parseErr := uuid.Parse(name)
		if parseErr != nil || parsed == uuid.Nil || name != parsed.String() {
			continue
		}
		path := filepath.Join(root, name)
		entryInfo, statErr := os.Lstat(path)
		if statErr != nil {
			state = strongerSafariProfileState(state, profileStateForError(statErr))
			continue
		}
		if !entryInfo.IsDir() || entryInfo.Mode()&os.ModeSymlink != 0 {
			state = ProfileStateInvalidData
			continue
		}
		result = append(result, name)
	}
	return result, state
}

func inspectSafariCookieSource(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ProfileStateNoData
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return profileStateForError(err)
	}
	if !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 {
		return ProfileStateInvalidData
	}
	info, err := os.Lstat(path)
	if err != nil {
		return profileStateForError(err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 16 || info.Size() > safariCookieCopyLimit {
		return ProfileStateInvalidData
	}
	file, err := os.Open(path)
	if err != nil {
		return profileStateForError(err)
	}
	header := make([]byte, 4)
	_, readErr := io.ReadFull(file, header)
	closeErr := file.Close()
	if readErr != nil {
		return profileStateForError(readErr)
	}
	if closeErr != nil || string(header) != "cook" {
		return ProfileStateInvalidData
	}
	return ProfileStateReady
}

func strongerSafariProfileState(current, candidate string) string {
	rank := func(state string) int {
		switch state {
		case ProfileStatePermissionRequired:
			return 3
		case ProfileStateInvalidData:
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

func readSafariProfileMetadata(path string) ([]safariProfileMetadata, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return []safariProfileMetadata{}, ProfileStateNoData
	}
	if state := inspectSafariTabsDatabase(path); state != ProfileStateReady {
		return []safariProfileMetadata{}, state
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return []safariProfileMetadata{}, ProfileStateInvalidData
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String() + "?mode=ro"
	database, err := sqlite3driver.Open(uri)
	if err != nil {
		return []safariProfileMetadata{}, profileStateForError(err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	connection, err := database.Conn(ctx)
	if err != nil {
		return []safariProfileMetadata{}, sqliteProfileState(err)
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		return []safariProfileMetadata{}, sqliteProfileState(err)
	}
	transaction, err := connection.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return []safariProfileMetadata{}, sqliteProfileState(err)
	}
	defer transaction.Rollback()

	columns, err := safariBookmarksColumns(ctx, transaction)
	if err != nil {
		return []safariProfileMetadata{}, sqliteProfileState(err)
	}
	for _, required := range []string{
		"title", "external_uuid", "parent", "syncable", "type", "subtype",
		"hidden", "special_id", "order_index",
	} {
		if !columns[required] {
			return []safariProfileMetadata{}, ProfileStateInvalidData
		}
	}
	serverIDExpression := "NULL"
	if columns["server_id"] {
		serverIDExpression = "server_id"
	}
	result := make([]safariProfileMetadata, 0, 4)
	seen := make(map[string]struct{})
	invalidRow := false
	if columns["server_id"] {
		// Safari identifies the single default profile separately from named
		// profiles. Keep these predicates aligned with its shipped query rather
		// than inferring the default from a secondary-profile subtype.
		defaultRows, err := transaction.QueryContext(ctx,
			"SELECT title FROM bookmarks WHERE server_id = 'DefaultProfile'"+
				" AND parent = 0 AND syncable = 1 AND type = 1 AND subtype = 0 LIMIT ?",
			2,
		)
		if err != nil {
			return []safariProfileMetadata{}, sqliteProfileState(err)
		}
		for defaultRows.Next() {
			var title sql.NullString
			if err := defaultRows.Scan(&title); err != nil {
				_ = defaultRows.Close()
				return result, sqliteProfileState(err)
			}
			label := strings.TrimSpace(title.String)
			if !utf8.ValidString(label) || len(label) > safariMaxProfileLabelBytes {
				invalidRow = true
				continue
			}
			if _, duplicate := seen["DefaultProfile"]; duplicate {
				invalidRow = true
				continue
			}
			seen["DefaultProfile"] = struct{}{}
			result = append(result, safariProfileMetadata{
				identifier: "DefaultProfile",
				label:      label,
				isDefault:  true,
			})
		}
		if err := defaultRows.Err(); err != nil {
			_ = defaultRows.Close()
			return result, sqliteProfileState(err)
		}
		if err := defaultRows.Close(); err != nil {
			return result, sqliteProfileState(err)
		}
	}
	// These predicates are extracted verbatim from Safari's shipped profile
	// query. A separate subtype=2 query exists in Safari for a different model
	// and must not be used to identify named browser profiles.
	query := "SELECT title, external_uuid, " + serverIDExpression + " FROM bookmarks" +
		" WHERE parent = 0 AND syncable = 1 AND type = 1 AND subtype = 0" +
		" AND hidden = 0 AND special_id = 0 ORDER BY order_index ASC LIMIT ?"
	rows, err := transaction.QueryContext(ctx, query, safariMaxProfileCount+1)
	if err != nil {
		return []safariProfileMetadata{}, sqliteProfileState(err)
	}
	defer rows.Close()
	for rows.Next() {
		if len(result) >= safariMaxProfileCount {
			invalidRow = true
			break
		}
		var title, external, serverID sql.NullString
		if err := rows.Scan(&title, &external, &serverID); err != nil {
			return result, sqliteProfileState(err)
		}
		identifier := strings.TrimSpace(external.String)
		label := strings.TrimSpace(title.String)
		if !utf8.ValidString(label) || len(label) > safariMaxProfileLabelBytes {
			invalidRow = true
			continue
		}
		entry := safariProfileMetadata{label: label}
		if strings.TrimSpace(serverID.String) == "DefaultProfile" {
			// The dedicated default-profile query above intentionally has fewer
			// predicates than the named-profile query. Avoid adding it twice when
			// it also happens to satisfy this result set.
			continue
		} else {
			parsed, parseErr := uuid.Parse(identifier)
			if parseErr != nil || parsed == uuid.Nil || len(identifier) != 36 || !strings.EqualFold(identifier, parsed.String()) {
				invalidRow = true
				continue
			}
			// WebKit's UUID formatter uses canonical lowercase; use that exact
			// spelling so case-sensitive home volumes resolve the data store.
			entry.identifier = parsed.String()
		}
		if _, duplicate := seen[entry.identifier]; duplicate {
			invalidRow = true
			continue
		}
		seen[entry.identifier] = struct{}{}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return result, sqliteProfileState(err)
	}
	if err := rows.Close(); err != nil {
		return result, sqliteProfileState(err)
	}
	if err := transaction.Commit(); err != nil {
		return result, sqliteProfileState(err)
	}
	if invalidRow {
		return result, ProfileStateInvalidData
	}
	return result, ProfileStateReady
}

func inspectSafariTabsDatabase(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		return profileStateForError(err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 16 || info.Size() > safariTabsDatabaseReadLimit {
		return ProfileStateInvalidData
	}
	file, err := os.Open(path)
	if err != nil {
		return profileStateForError(err)
	}
	header := make([]byte, 16)
	_, readErr := io.ReadFull(file, header)
	closeErr := file.Close()
	if readErr != nil {
		return profileStateForError(readErr)
	}
	if closeErr != nil || string(header) != "SQLite format 3\x00" {
		return ProfileStateInvalidData
	}
	for suffix, limit := range map[string]int64{"-wal": safariTabsWALReadLimit, "-shm": safariTabsSHMReadLimit} {
		sidecar, err := os.Lstat(path + suffix)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return profileStateForError(err)
		}
		if !sidecar.Mode().IsRegular() || sidecar.Mode()&os.ModeSymlink != 0 || sidecar.Size() < 0 || sidecar.Size() > limit {
			return ProfileStateInvalidData
		}
	}
	return ProfileStateReady
}

func safariBookmarksColumns(ctx context.Context, transaction *sql.Tx) (map[string]bool, error) {
	rows, err := transaction.QueryContext(ctx, "PRAGMA table_info(bookmarks)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		result[strings.ToLower(strings.TrimSpace(name))] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Safari profile metadata table is missing")
	}
	return result, nil
}

func sqliteProfileState(err error) string {
	if err == nil {
		return ProfileStateReady
	}
	if errors.Is(err, os.ErrPermission) {
		return ProfileStatePermissionRequired
	}
	return ProfileStateInvalidData
}
