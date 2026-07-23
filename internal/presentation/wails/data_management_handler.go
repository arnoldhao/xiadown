package wails

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	appsessionsdto "xiadown/internal/application/appsessions/dto"
	datamanagementdto "xiadown/internal/application/datamanagement/dto"
	"xiadown/internal/application/sniffprofile"
)

const dataManagementWalkLimit = 100000

type DataManagementAppSessions interface {
	ListAppSessions(context.Context) ([]appsessionsdto.AppSession, error)
	ClearAppSession(context.Context, appsessionsdto.ClearAppSessionRequest) error
}

type DataManagementActivity interface {
	HasActiveDataOperations() bool
	ActiveResourceSniffProfileIDs() []string
}

type DataManagementDependencyActivity interface {
	HasActiveInstalls() bool
}

type DataManagementResetter interface {
	Schedule(context.Context) error
}

type DataManagementConfig struct {
	ConfigRoot      string
	CacheRoot       string
	LogDirectory    string
	DatabasePath    string
	Database        *sql.DB
	BackupDirectory string
	AppSessions     DataManagementAppSessions
	Activity        DataManagementActivity
	Dependencies    DataManagementDependencyActivity
	// SessionVaultKeyInventory reports metadata only (count/bytes). It must not
	// load or expose Session Vault key material.
	SessionVaultKeyInventory func(context.Context) (int, int64, error)
	Resetter                 DataManagementResetter
	Quitter                  appQuitter
}

type DataManagementHandler struct {
	config         DataManagementConfig
	resetMu        sync.Mutex
	resetScheduled bool
}

type DataManagementSnapshot = datamanagementdto.DataManagementSnapshot
type DataManagementCategory = datamanagementdto.DataManagementCategory
type DataManagementItem = datamanagementdto.DataManagementItem
type CleanDataManagementRequest = datamanagementdto.CleanDataManagementRequest
type DataManagementCleanResult = datamanagementdto.CleanDataManagementResult
type CleanDataManagementResponse = datamanagementdto.CleanDataManagementResponse
type ResetApplicationResponse = datamanagementdto.ResetApplicationResponse

type measuredPath struct {
	bytes     int64
	files     int
	truncated bool
}

func NewDataManagementHandler(config DataManagementConfig) *DataManagementHandler {
	config.ConfigRoot = cleanOptionalPath(config.ConfigRoot)
	config.CacheRoot = cleanOptionalPath(config.CacheRoot)
	config.LogDirectory = cleanOptionalPath(config.LogDirectory)
	config.DatabasePath = cleanOptionalPath(config.DatabasePath)
	config.BackupDirectory = cleanOptionalPath(config.BackupDirectory)
	return &DataManagementHandler{config: config}
}

func cleanOptionalPath(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return filepath.Clean(value)
}

func (handler *DataManagementHandler) ServiceName() string { return "DataManagementHandler" }

func (handler *DataManagementHandler) GetSnapshot(ctx context.Context) (DataManagementSnapshot, error) {
	if handler == nil {
		return DataManagementSnapshot{}, fmt.Errorf("data management unavailable")
	}
	busy := handler.config.Activity != nil && handler.config.Activity.HasActiveDataOperations()
	componentsBusy := busy || (handler.config.Dependencies != nil && handler.config.Dependencies.HasActiveInstalls())

	imageCache := measureManagedPaths(handler.config.CacheRoot, handler.imageCachePath())
	rssCache := measureManagedPaths(handler.config.CacheRoot, handler.rssCachePath())
	faviconCache := measureManagedPaths(handler.config.CacheRoot, handler.faviconCachePath())
	archivedLogs := handler.measureArchivedLogs()
	activeLogs := handler.measureActiveLogs()
	dependencies := measureManagedPaths(handler.config.ConfigRoot, filepath.Join(handler.config.ConfigRoot, "dependencies"))
	database := measureManagedPaths(handler.config.ConfigRoot, handler.config.DatabasePath, handler.config.DatabasePath+"-wal", handler.config.DatabasePath+"-shm")
	backups := measureManagedPaths(handler.config.ConfigRoot, handler.config.BackupDirectory)
	legacyBackups := measureManagedPaths(handler.config.ConfigRoot, handler.legacyDatabaseBackups()...)
	userContent := measureManagedPaths(handler.config.ConfigRoot, handler.userContentPaths()...)
	updateStage := measureManagedPaths(handler.config.ConfigRoot, filepath.Join(handler.config.ConfigRoot, "update-stage"))

	currentProfiles := sniffprofile.ExistingProfiles()
	var currentProfileMeasure measuredPath
	for _, profile := range currentProfiles {
		currentProfileMeasure.bytes += profile.SizeBytes
		currentProfileMeasure.files += profile.FileCount
	}
	currentProfileItem := newDataManagementItem("browser-profiles", "currentSniffProfiles", currentProfileMeasure, "protected", false, false, busy)
	currentProfileItem.ItemCount = len(currentProfiles)
	if !busy && len(currentProfiles) > 0 {
		currentProfileItem.State = "ready"
	}

	legacyProfiles := sniffprofile.LegacyInfos()
	var legacyProfileMeasure measuredPath
	for _, profile := range legacyProfiles {
		legacyProfileMeasure.bytes += profile.SizeBytes
		legacyProfileMeasure.files += profile.FileCount
	}
	legacyProfileItem := newDataManagementItem("legacy.sniff-profiles", "legacySniffProfiles", legacyProfileMeasure, "safe", true, false, false)
	legacyProfileItem.ItemCount = len(legacyProfiles)
	if len(legacyProfiles) > 0 {
		legacyProfileItem.State = "legacy"
	}

	sessionItem := DataManagementItem{
		ID:             "app-sessions",
		LabelKey:       "dataManagement.resource.currentAppSessions.label",
		DescriptionKey: "dataManagement.resource.currentAppSessions.description",
		Risk:           "protected",
		Clearable:      false,
		State:          "empty",
	}
	if secretCount, secretBytes, err := handler.appSessionSecretInventory(ctx); err == nil {
		sessionItem.ItemCount = secretCount
		sessionItem.SizeBytes = secretBytes
		if secretCount > 0 {
			sessionItem.State = "ready"
		}
	}
	if handler.config.AppSessions != nil {
		if sessions, err := handler.config.AppSessions.ListAppSessions(ctx); err == nil {
			connectedCount := 0
			for _, session := range sessions {
				if session.Status == "connected" || session.CredentialState == "connected" || session.CookiesCount > 0 {
					connectedCount++
				}
			}
			if connectedCount > sessionItem.ItemCount {
				sessionItem.ItemCount = connectedCount
			}
			if sessionItem.ItemCount > 0 {
				sessionItem.State = "ready"
			}
		} else {
			sessionItem.State = "error"
		}
	}

	legacySecretCount, legacyBytes, legacyErr := legacyAppSessionSecretInventory()
	legacyMetadataCount := handler.legacyAppSessionMetadataCount(ctx)
	legacySessionItem := DataManagementItem{
		ID:                "legacy.app-sessions",
		LabelKey:          "dataManagement.resource.legacyAppSessions.label",
		DescriptionKey:    "dataManagement.resource.legacyAppSessions.description",
		SizeBytes:         legacyBytes,
		ItemCount:         legacySecretCount + legacyMetadataCount,
		State:             "empty",
		Risk:              "safe",
		Clearable:         legacyErr == nil && legacySecretCount+legacyMetadataCount > 0,
		SelectedByDefault: false,
	}
	if legacyErr != nil {
		legacySessionItem.State = "error"
	} else if legacySessionItem.ItemCount > 0 {
		legacySessionItem.State = "legacy"
	}

	databaseItem := newDataManagementItem("database", "database", database, "protected", false, false, false)
	databaseItem.State = "protected"
	backupItem := newDataManagementItem("database-backups", "databaseBackups", backups, "protected", false, false, false)
	if backups.bytes > 0 || backups.files > 0 {
		backupItem.State = "protected"
	}
	legacyBackupItem := newDataManagementItem("legacy.database-backups", "legacyDatabaseBackups", legacyBackups, "safe", true, false, false)
	if legacyBackups.bytes > 0 {
		legacyBackupItem.State = "legacy"
	}

	userItem := newDataManagementItem("user-content", "userContent", userContent, "protected", false, false, false)
	userItem.State = "protected"
	updateItem := newDataManagementItem("update-stage", "updateStage", updateStage, "protected", false, false, false)
	updateItem.State = "protected"
	dependencyItem := newDataManagementItem("dependencies", "dependencies", dependencies, "protected", false, false, componentsBusy)
	if !componentsBusy && (dependencies.bytes > 0 || dependencies.files > 0) {
		dependencyItem.State = "protected"
	}
	activeLogItem := newDataManagementItem("active-logs", "activeLogs", activeLogs, "protected", false, false, false)
	activeLogItem.State = "protected"
	sessionVaultKeyItem := newDataManagementItem("session-vault-key", "sessionVaultKey", measuredPath{}, "protected", false, false, false)
	sessionVaultKeyItem.State = "protected"
	if handler.config.SessionVaultKeyInventory != nil {
		count, bytes, inventoryErr := handler.config.SessionVaultKeyInventory(ctx)
		if inventoryErr != nil || count < 0 || bytes < 0 {
			sessionVaultKeyItem.State = "error"
		} else {
			sessionVaultKeyItem.ItemCount = count
			sessionVaultKeyItem.SizeBytes = bytes
		}
	}

	obsoleteItems := visibleObsoleteItems(legacySessionItem, legacyProfileItem, legacyBackupItem)

	categories := []DataManagementCategory{
		newDataManagementCategory("core", "dataManagement.category.core", []DataManagementItem{
			databaseItem,
			backupItem,
			dependencyItem,
			activeLogItem,
			sessionVaultKeyItem,
			sessionItem,
			currentProfileItem,
			userItem,
			updateItem,
		}),
		newDataManagementCategory("reclaimable", "dataManagement.category.reclaimable", []DataManagementItem{
			newDataManagementItem("image-cache", "imageCache", imageCache, "safe", true, true, false),
			newDataManagementItem("rss-cache", "rssCache", rssCache, "safe", true, true, false),
			newDataManagementItem("favicon-cache", "faviconCache", faviconCache, "safe", true, true, false),
			newDataManagementItem("archived-logs", "archivedLogs", archivedLogs, "safe", true, true, false),
		}),
		newDataManagementCategory("obsolete", "dataManagement.category.obsolete", obsoleteItems),
	}

	var total int64
	var safe int64
	for categoryIndex := range categories {
		for _, item := range categories[categoryIndex].Items {
			categories[categoryIndex].TotalBytes += item.SizeBytes
			total += item.SizeBytes
			if categories[categoryIndex].ID == "reclaimable" && item.Clearable && item.SelectedByDefault {
				safe += item.SizeBytes
			}
		}
	}
	return DataManagementSnapshot{
		TotalBytes:           total,
		SafeReclaimableBytes: safe,
		ScannedAt:            time.Now().UTC().Format(time.RFC3339),
		Categories:           categories,
	}, nil
}

func (handler *DataManagementHandler) Clean(ctx context.Context, request CleanDataManagementRequest) (CleanDataManagementResponse, error) {
	results := make([]DataManagementCleanResult, 0, len(request.ResourceIDs))
	seen := make(map[string]struct{}, len(request.ResourceIDs))
	for _, rawID := range request.ResourceIDs {
		resourceID := strings.TrimSpace(rawID)
		if resourceID == "" {
			continue
		}
		if _, duplicate := seen[resourceID]; duplicate {
			continue
		}
		seen[resourceID] = struct{}{}
		result := handler.cleanOne(ctx, resourceID)
		results = append(results, result)
	}
	snapshot, err := handler.GetSnapshot(ctx)
	if err != nil {
		return CleanDataManagementResponse{Results: results}, err
	}
	return CleanDataManagementResponse{Results: results, Snapshot: snapshot}, nil
}

func (handler *DataManagementHandler) ResetApplication(ctx context.Context) (ResetApplicationResponse, error) {
	if handler == nil || handler.config.Resetter == nil || handler.config.Quitter == nil {
		return ResetApplicationResponse{}, fmt.Errorf("application reset unavailable")
	}
	if handler.hasActiveOperations() {
		return ResetApplicationResponse{}, fmt.Errorf("application reset is unavailable while operations are active")
	}

	handler.resetMu.Lock()
	defer handler.resetMu.Unlock()
	if handler.resetScheduled {
		return ResetApplicationResponse{Scheduled: true}, nil
	}
	if err := handler.config.Resetter.Schedule(ctx); err != nil {
		return ResetApplicationResponse{}, err
	}
	handler.resetScheduled = true
	go func() {
		time.Sleep(150 * time.Millisecond)
		handler.config.Quitter.Quit()
	}()
	return ResetApplicationResponse{Scheduled: true}, nil
}

func (handler *DataManagementHandler) cleanOne(ctx context.Context, resourceID string) DataManagementCleanResult {
	before, _ := handler.GetSnapshot(ctx)
	beforeBytes := snapshotResourceBytes(before, resourceID)
	result := DataManagementCleanResult{ResourceID: resourceID}
	var err error
	switch {
	case resourceID == "image-cache":
		err = handler.cleanDirectory(handler.imageCachePath(), handler.config.CacheRoot)
	case resourceID == "rss-cache":
		err = handler.cleanDirectory(handler.rssCachePath(), handler.config.CacheRoot)
	case resourceID == "favicon-cache":
		err = handler.cleanDirectory(handler.faviconCachePath(), handler.config.CacheRoot)
	case resourceID == "archived-logs":
		err = handler.cleanArchivedLogs()
	case resourceID == "legacy.app-sessions":
		err = handler.clearLegacyAppSessions(ctx)
	case resourceID == "legacy.sniff-profiles":
		err = sniffprofile.ClearLegacy()
	case resourceID == "legacy.database-backups":
		err = handler.cleanLegacyDatabaseBackups()
	default:
		result.Status = "denied"
		result.Message = "resource is protected or unknown"
		return result
	}
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			result.Status = "already_missing"
			return result
		}
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}
	after, _ := handler.GetSnapshot(ctx)
	afterBytes := snapshotResourceBytes(after, resourceID)
	result.Status = "cleared"
	if beforeBytes > afterBytes {
		result.BytesFreed = beforeBytes - afterBytes
	}
	return result
}

func (handler *DataManagementHandler) hasActiveOperations() bool {
	if handler == nil {
		return false
	}
	if handler.config.Activity != nil && handler.config.Activity.HasActiveDataOperations() {
		return true
	}
	return handler.config.Dependencies != nil && handler.config.Dependencies.HasActiveInstalls()
}

func (handler *DataManagementHandler) appSessionSecretInventory(ctx context.Context) (int, int64, error) {
	if handler == nil || handler.config.Database == nil {
		return 0, 0, nil
	}
	var count int
	var bytes int64
	err := handler.config.Database.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(length(nonce) + length(ciphertext)), 0)
FROM app_session_secrets
`).Scan(&count, &bytes)
	if err != nil {
		return 0, 0, err
	}
	if count < 0 || bytes < 0 {
		return 0, 0, fmt.Errorf("invalid App Session secret inventory")
	}
	return count, bytes, nil
}

func (handler *DataManagementHandler) legacyAppSessionMetadataCount(ctx context.Context) int {
	if handler == nil || handler.config.Database == nil {
		return 0
	}
	var count int
	if err := handler.config.Database.QueryRowContext(ctx, "SELECT COUNT(*) FROM site_app_sessions").Scan(&count); err != nil || count < 0 {
		return 0
	}
	return count
}

func (handler *DataManagementHandler) clearLegacyAppSessions(ctx context.Context) error {
	if err := clearLegacyAppSessionSecrets(); err != nil {
		return err
	}
	if handler == nil || handler.config.Database == nil {
		return nil
	}
	if _, err := handler.config.Database.ExecContext(ctx, "DELETE FROM site_app_sessions"); err != nil && !strings.Contains(strings.ToLower(err.Error()), "no such table") {
		return err
	}
	return nil
}

func (handler *DataManagementHandler) cachePaths() []string {
	if handler.config.CacheRoot == "" {
		return []string{}
	}
	root := filepath.Join(handler.config.CacheRoot, "xiadown")
	return []string{
		filepath.Join(root, "com.xiadown.listen.imagecache"),
		filepath.Join(root, "rss", "resources", "v1"),
		filepath.Join(root, "library", "favicons"),
	}
}

func (handler *DataManagementHandler) imageCachePath() string {
	return filepath.Join(handler.config.CacheRoot, "xiadown", "com.xiadown.listen.imagecache")
}

func (handler *DataManagementHandler) rssCachePath() string {
	return filepath.Join(handler.config.CacheRoot, "xiadown", "rss", "resources", "v1")
}

func (handler *DataManagementHandler) faviconCachePath() string {
	return filepath.Join(handler.config.CacheRoot, "xiadown", "library", "favicons")
}

func (handler *DataManagementHandler) userContentPaths() []string {
	if handler.config.ConfigRoot == "" {
		return []string{}
	}
	return []string{
		filepath.Join(handler.config.ConfigRoot, "pets"),
		filepath.Join(handler.config.ConfigRoot, "sprites"),
		filepath.Join(handler.config.ConfigRoot, "listen-lyrics"),
		filepath.Join(handler.config.ConfigRoot, "library-access"),
		filepath.Join(handler.config.ConfigRoot, "equalizer.json"),
	}
}

func (handler *DataManagementHandler) legacyDatabaseBackups() []string {
	if handler.config.ConfigRoot == "" {
		return []string{}
	}
	matches, _ := filepath.Glob(filepath.Join(handler.config.ConfigRoot, "data.db.pre-migration-*.bak"))
	sort.Strings(matches)
	return matches
}

func (handler *DataManagementHandler) activeProfileSet() map[string]struct{} {
	result := make(map[string]struct{})
	if handler.config.Activity == nil {
		return result
	}
	for _, profileID := range handler.config.Activity.ActiveResourceSniffProfileIDs() {
		if trimmed := strings.TrimSpace(profileID); trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

func (handler *DataManagementHandler) measureArchivedLogs() measuredPath {
	if err := ensureNoSymlinkComponents(handler.config.LogDirectory, handler.config.LogDirectory); err != nil {
		return measuredPath{}
	}
	entries, err := os.ReadDir(handler.config.LogDirectory)
	if err != nil {
		return measuredPath{}
	}
	var measured measuredPath
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !isArchivedLogName(entry.Name()) {
			continue
		}
		if info, err := entry.Info(); err == nil {
			measured.bytes += info.Size()
			measured.files++
		}
	}
	return measured
}

func (handler *DataManagementHandler) measureActiveLogs() measuredPath {
	if err := ensureNoSymlinkComponents(handler.config.LogDirectory, handler.config.LogDirectory); err != nil {
		return measuredPath{}
	}
	var measured measuredPath
	for _, name := range []string{"app.log", "startup.log"} {
		path := filepath.Join(handler.config.LogDirectory, name)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}
		measured.bytes += info.Size()
		measured.files++
	}
	return measured
}

func (handler *DataManagementHandler) cleanArchivedLogs() error {
	if err := ensureNoSymlinkComponents(handler.config.LogDirectory, handler.config.LogDirectory); err != nil {
		return err
	}
	entries, err := os.ReadDir(handler.config.LogDirectory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !isArchivedLogName(entry.Name()) {
			continue
		}
		path := filepath.Join(handler.config.LogDirectory, entry.Name())
		if err := safeRemoveFile(path, handler.config.LogDirectory); err != nil {
			return err
		}
	}
	return nil
}

func isArchivedLogName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "app.log" || name == "startup.log" {
		return false
	}
	if !strings.HasPrefix(name, "app-") && !strings.HasPrefix(name, "startup-") {
		return false
	}
	return strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".log.gz")
}

func (handler *DataManagementHandler) cleanDirectories(paths ...string) error {
	for _, path := range paths {
		if err := handler.cleanDirectory(path, handler.config.CacheRoot); err != nil {
			return err
		}
	}
	return nil
}

func (handler *DataManagementHandler) cleanDirectory(path string, ownershipRoot string) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(ownershipRoot) == "" {
		return fmt.Errorf("managed path is empty")
	}
	path = filepath.Clean(path)
	root := filepath.Clean(ownershipRoot)
	if !pathWithin(root, path) || path == root {
		return fmt.Errorf("managed path escapes ownership root")
	}
	if err := ensureNoSymlinkComponents(root, path); err != nil {
		return err
	}
	stat, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if stat.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to clear symlink")
	}
	trashRoot, err := prepareTrashRoot(root)
	if err != nil {
		return err
	}
	trashPath := filepath.Join(trashRoot, uuid.NewString())
	if err := os.Rename(path, trashPath); err != nil {
		return err
	}
	if stat.IsDir() {
		if err := os.MkdirAll(path, stat.Mode().Perm()); err != nil {
			_ = os.Rename(trashPath, path)
			return err
		}
	}
	return os.RemoveAll(trashPath)
}

func (handler *DataManagementHandler) cleanLegacyDatabaseBackups() error {
	for _, path := range handler.legacyDatabaseBackups() {
		if err := safeRemoveFile(path, handler.config.ConfigRoot); err != nil {
			return err
		}
	}
	return nil
}

func safeRemoveFile(path string, ownershipRoot string) error {
	path = filepath.Clean(path)
	root := filepath.Clean(ownershipRoot)
	if !pathWithin(root, path) || path == root {
		return fmt.Errorf("managed file escapes ownership root")
	}
	if err := ensureNoSymlinkComponents(root, path); err != nil {
		return err
	}
	stat, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed file is not a regular file")
	}
	trashRoot, err := prepareTrashRoot(root)
	if err != nil {
		return err
	}
	trashPath := filepath.Join(trashRoot, uuid.NewString())
	if err := os.Rename(path, trashPath); err != nil {
		return err
	}
	return os.Remove(trashPath)
}

func measureManagedPaths(root string, paths ...string) measuredPath {
	var result measuredPath
	root = strings.TrimSpace(root)
	if root == "" {
		return result
	}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || !pathWithin(filepath.Clean(root), filepath.Clean(path)) {
			continue
		}
		if err := ensureNoSymlinkComponents(root, path); err != nil {
			continue
		}
		measured := measurePath(path)
		result.bytes += measured.bytes
		result.files += measured.files
		result.truncated = result.truncated || measured.truncated
	}
	return result
}

// ensureNoSymlinkComponents rejects a managed root or any existing component
// below it that is a symbolic link. A lexical containment check alone is not
// sufficient: an intermediate link could redirect an allowlisted cache or
// profile path into unrelated user data.
func ensureNoSymlinkComponents(root string, path string) error {
	root = filepath.Clean(strings.TrimSpace(root))
	path = filepath.Clean(strings.TrimSpace(path))
	if root == "." || path == "." {
		return fmt.Errorf("managed path is empty")
	}
	if path != root && !pathWithin(root, path) {
		return fmt.Errorf("managed path escapes ownership root")
	}

	rootInfo, err := os.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing managed root symlink")
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("managed root is not a directory")
	}
	if path == root {
		return nil
	}

	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing managed path symlink")
		}
	}
	return nil
}

func prepareTrashRoot(root string) (string, error) {
	if err := ensureNoSymlinkComponents(root, root); err != nil {
		return "", err
	}
	trashRoot := filepath.Join(filepath.Clean(root), ".xiadown-trash")
	info, err := os.Lstat(trashRoot)
	if os.IsNotExist(err) {
		if err := os.Mkdir(trashRoot, 0o700); err != nil {
			return "", err
		}
		return trashRoot, nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("managed trash path is not a directory")
	}
	return trashRoot, nil
}

func measurePath(path string) measuredPath {
	path = strings.TrimSpace(path)
	if path == "" {
		return measuredPath{}
	}
	stat, err := os.Lstat(path)
	if err != nil || stat.Mode()&os.ModeSymlink != 0 {
		return measuredPath{}
	}
	if !stat.IsDir() {
		return measuredPath{bytes: stat.Size(), files: 1}
	}
	var result measuredPath
	visited := 0
	_ = filepath.WalkDir(path, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || currentPath == path {
			return nil
		}
		visited++
		if visited > dataManagementWalkLimit {
			result.truncated = true
			return filepath.SkipAll
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			result.bytes += info.Size()
			result.files++
		}
		return nil
	})
	return result
}

func pathWithin(root string, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func newDataManagementCategory(id string, labelKey string, items []DataManagementItem) DataManagementCategory {
	if items == nil {
		items = []DataManagementItem{}
	}
	return DataManagementCategory{ID: id, LabelKey: labelKey, Items: items}
}

func newDataManagementItem(id string, key string, measured measuredPath, risk string, clearable bool, selected bool, busy bool) DataManagementItem {
	state := "empty"
	if measured.bytes > 0 || measured.files > 0 {
		state = "ready"
	}
	if busy {
		state = "busy"
	}
	return DataManagementItem{
		ID:                id,
		LabelKey:          "dataManagement.resource." + key + ".label",
		DescriptionKey:    "dataManagement.resource." + key + ".description",
		SizeBytes:         measured.bytes,
		ItemCount:         measured.files,
		State:             state,
		Risk:              risk,
		Clearable:         clearable && (measured.bytes > 0 || measured.files > 0),
		SelectedByDefault: selected && (measured.bytes > 0 || measured.files > 0),
	}
}

func snapshotResourceBytes(snapshot DataManagementSnapshot, resourceID string) int64 {
	for _, category := range snapshot.Categories {
		for _, item := range category.Items {
			if item.ID == resourceID {
				return item.SizeBytes
			}
		}
	}
	return 0
}

func visibleObsoleteItems(items ...DataManagementItem) []DataManagementItem {
	result := make([]DataManagementItem, 0, len(items))
	for _, item := range items {
		if item.State == "error" || item.ItemCount > 0 || item.SizeBytes > 0 {
			result = append(result, item)
		}
	}
	return result
}
