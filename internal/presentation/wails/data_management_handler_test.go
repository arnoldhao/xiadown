package wails

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	appsessionsdto "xiadown/internal/application/appsessions/dto"
	"xiadown/internal/infrastructure/persistence"
)

type dataManagementSessionsStub struct {
	items   []appsessionsdto.AppSession
	cleared []string
}

func (stub *dataManagementSessionsStub) ListAppSessions(context.Context) ([]appsessionsdto.AppSession, error) {
	return append([]appsessionsdto.AppSession(nil), stub.items...), nil
}

func (stub *dataManagementSessionsStub) ClearAppSession(_ context.Context, request appsessionsdto.ClearAppSessionRequest) error {
	stub.cleared = append(stub.cleared, request.ID)
	return nil
}

type dataManagementActivityStub struct {
	busy     bool
	profiles []string
}

type dataManagementDependencyActivityStub struct{ active bool }

func (stub dataManagementDependencyActivityStub) HasActiveInstalls() bool { return stub.active }

func (stub dataManagementActivityStub) HasActiveDataOperations() bool { return stub.busy }
func (stub dataManagementActivityStub) ActiveResourceSniffProfileIDs() []string {
	return append([]string(nil), stub.profiles...)
}

type dataManagementResetterStub struct {
	calls int
	err   error
}

func (stub *dataManagementResetterStub) Schedule(context.Context) error {
	stub.calls++
	return stub.err
}

type dataManagementQuitterStub struct{ called chan struct{} }

func (stub dataManagementQuitterStub) Quit() {
	select {
	case stub.called <- struct{}{}:
	default:
	}
}

func writeDataManagementFixture(t *testing.T, path string, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func dataManagementTestHandler(t *testing.T, busy bool) (*DataManagementHandler, string, string, string, *dataManagementSessionsStub) {
	t.Helper()
	base := t.TempDir()
	configRoot := filepath.Join(base, "config")
	cacheRoot := filepath.Join(base, "cache")
	logRoot := filepath.Join(base, "logs")
	databasePath := filepath.Join(configRoot, "data.db")
	backupRoot := filepath.Join(configRoot, "library-backups")
	sessions := &dataManagementSessionsStub{items: []appsessionsdto.AppSession{{
		ID: "site-app-session-youtube", SiteKey: "youtube", Status: "connected", CredentialState: "connected", CookiesCount: 3,
	}}}
	handler := NewDataManagementHandler(DataManagementConfig{
		ConfigRoot:      configRoot,
		CacheRoot:       cacheRoot,
		LogDirectory:    logRoot,
		DatabasePath:    databasePath,
		BackupDirectory: backupRoot,
		AppSessions:     sessions,
		Activity:        dataManagementActivityStub{busy: busy},
		SessionVaultKeyInventory: func(context.Context) (int, int64, error) {
			return 1, 0, nil
		},
	})
	return handler, configRoot, cacheRoot, logRoot, sessions
}

func TestDataManagementSnapshotSeparatesSafeAndProtectedData(t *testing.T) {
	handler, configRoot, cacheRoot, logRoot, _ := dataManagementTestHandler(t, false)
	writeDataManagementFixture(t, filepath.Join(cacheRoot, "xiadown", "com.xiadown.listen.imagecache", "cover.png"), "cache")
	writeDataManagementFixture(t, filepath.Join(logRoot, "app.log"), "active")
	writeDataManagementFixture(t, filepath.Join(logRoot, "app-2026-07-17.log.gz"), "archive")
	writeDataManagementFixture(t, filepath.Join(configRoot, "data.db"), "database")
	writeDataManagementFixture(t, filepath.Join(configRoot, "library-backups", "backup.db"), "backup")

	snapshot, err := handler.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SafeReclaimableBytes != int64(len("cache")+len("archive")) {
		t.Fatalf("unexpected safe reclaimable size: %#v", snapshot)
	}
	if snapshotResourceBytes(snapshot, "database") != int64(len("database")) {
		t.Fatalf("database should be inventoried separately: %#v", snapshot)
	}
	if item := findDataManagementItem(snapshot, "database"); item == nil || item.Clearable || item.State != "protected" {
		t.Fatalf("database must be protected: %#v", item)
	}
	if len(snapshot.Categories) != 3 || snapshot.Categories[0].ID != "core" || snapshot.Categories[1].ID != "reclaimable" || snapshot.Categories[2].ID != "obsolete" {
		t.Fatalf("unexpected data categories: %#v", snapshot.Categories)
	}
	if item := findDataManagementItem(snapshot, "app-sessions"); item == nil || item.ItemCount != 1 || item.Clearable || item.LabelKey != "dataManagement.resource.currentAppSessions.label" {
		t.Fatalf("expected one protected current session: %#v", item)
	}
	if item := findDataManagementItem(snapshot, "user-content"); item == nil || item.Clearable || item.LabelKey != "dataManagement.resource.userContent.label" {
		t.Fatalf("personal content must have an explicit protected label: %#v", item)
	}
	if item := findDataManagementItem(snapshot, "update-stage"); item == nil || item.Clearable || item.LabelKey != "dataManagement.resource.updateStage.label" {
		t.Fatalf("staged updates must have an explicit protected label: %#v", item)
	}
	if item := findDataManagementItem(snapshot, "session-vault-key"); item == nil || item.Clearable || item.ItemCount != 1 {
		t.Fatalf("Session Vault key metadata must be visible and protected: %#v", item)
	}
}

func TestDataManagementInventoriesActiveLogsAsProtectedCoreData(t *testing.T) {
	handler, _, _, logRoot, _ := dataManagementTestHandler(t, false)
	appLog := filepath.Join(logRoot, "app.log")
	startupLog := filepath.Join(logRoot, "startup.log")
	writeDataManagementFixture(t, appLog, "active-app")
	writeDataManagementFixture(t, startupLog, "active-startup")

	snapshot, err := handler.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	item := findDataManagementItem(snapshot, "active-logs")
	if item == nil || item.Clearable || item.ItemCount != 2 || item.SizeBytes != int64(len("active-app")+len("active-startup")) {
		t.Fatalf("active logs must be inventoried as protected core data: %#v", item)
	}
	response, err := handler.Clean(context.Background(), CleanDataManagementRequest{ResourceIDs: []string{"active-logs", "session-vault-key"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, resourceID := range []string{"active-logs", "session-vault-key"} {
		if result := findDataManagementResult(response, resourceID); result == nil || result.Status != "denied" {
			t.Fatalf("protected resource %s cleanup should be denied: %#v", resourceID, result)
		}
	}
	for _, path := range []string{appLog, startupLog} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("active log must remain after denied cleanup: %s: %v", path, err)
		}
	}
}

func TestDataManagementOmitsEmptyObsoleteRows(t *testing.T) {
	items := visibleObsoleteItems(
		DataManagementItem{ID: "empty", State: "empty"},
		DataManagementItem{ID: "present-count", State: "legacy", ItemCount: 1},
		DataManagementItem{ID: "present-size", State: "legacy", SizeBytes: 1},
		DataManagementItem{ID: "error", State: "error"},
	)
	if len(items) != 3 || items[0].ID != "present-count" || items[1].ID != "present-size" || items[2].ID != "error" {
		t.Fatalf("unexpected visible obsolete items: %#v", items)
	}
}

func TestDataManagementInventoriesEncryptedAppSessionBytes(t *testing.T) {
	handler, _, _, _, _ := dataManagementTestHandler(t, false)
	database, err := persistence.OpenSQLite(context.Background(), persistence.SQLiteConfig{
		Path:                     filepath.Join(t.TempDir(), "data.db"),
		SkipPreMigrationSnapshot: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.SQL.Exec(`INSERT INTO app_sessions (id, site_key, status) VALUES ('session-youtube', 'youtube', 'connected')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.Exec(`
CREATE TABLE site_app_sessions (id TEXT PRIMARY KEY);
INSERT INTO site_app_sessions (id) VALUES ('legacy-youtube');
`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.Exec(`
INSERT INTO app_session_secrets (site_key, key_id, format_version, nonce, ciphertext)
VALUES ('youtube', 'master-key', 1, zeroblob(12), zeroblob(20))
`); err != nil {
		t.Fatal(err)
	}
	handler.config.Database = database.SQL

	snapshot, err := handler.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	item := findDataManagementItem(snapshot, "app-sessions")
	if item == nil || item.SizeBytes != 32 || item.ItemCount != 1 || item.State != "ready" {
		t.Fatalf("unexpected encrypted App Session inventory: %#v", item)
	}
	legacyItem := findDataManagementItem(snapshot, "legacy.app-sessions")
	if legacyItem == nil || legacyItem.LabelKey != "dataManagement.resource.legacyAppSessions.label" || legacyItem.LabelKey == item.LabelKey {
		t.Fatalf("legacy/current App Session labels must be distinct: current=%#v legacy=%#v", item, legacyItem)
	}
}

func TestDataManagementCleanPreservesActiveLogsAndCoreDatabase(t *testing.T) {
	handler, configRoot, cacheRoot, logRoot, sessions := dataManagementTestHandler(t, false)
	cacheFile := filepath.Join(cacheRoot, "xiadown", "com.xiadown.listen.imagecache", "cover.png")
	activeLog := filepath.Join(logRoot, "app.log")
	archivedLog := filepath.Join(logRoot, "app-2026-07-17.log.gz")
	database := filepath.Join(configRoot, "data.db")
	writeDataManagementFixture(t, cacheFile, "cache")
	writeDataManagementFixture(t, activeLog, "active")
	writeDataManagementFixture(t, archivedLog, "archive")
	writeDataManagementFixture(t, database, "database")

	response, err := handler.Clean(context.Background(), CleanDataManagementRequest{ResourceIDs: []string{
		"image-cache", "archived-logs", "app-sessions", "database", "database-backups", "dependencies", "browser-profiles", "user-content",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Fatalf("cache file should be removed: %v", err)
	}
	if _, err := os.Stat(archivedLog); !os.IsNotExist(err) {
		t.Fatalf("archived log should be removed: %v", err)
	}
	if _, err := os.Stat(activeLog); err != nil {
		t.Fatalf("active log must be preserved: %v", err)
	}
	if _, err := os.Stat(database); err != nil {
		t.Fatalf("core database must be preserved: %v", err)
	}
	if len(sessions.cleared) != 0 {
		t.Fatalf("current sessions are core data and must not be individually cleared: %#v", sessions.cleared)
	}
	for _, resourceID := range []string{"app-sessions", "database", "database-backups", "dependencies", "browser-profiles", "user-content"} {
		if result := findDataManagementResult(response, resourceID); result == nil || result.Status != "denied" {
			t.Fatalf("core resource %s cleanup should be denied: %#v", resourceID, result)
		}
	}
}

func TestDataManagementBlocksComponentsWhileOperationsAreActive(t *testing.T) {
	handler, configRoot, _, _, _ := dataManagementTestHandler(t, true)
	tool := filepath.Join(configRoot, "dependencies", "yt-dlp", "yt-dlp")
	writeDataManagementFixture(t, tool, "tool")

	response, err := handler.Clean(context.Background(), CleanDataManagementRequest{ResourceIDs: []string{"dependencies"}})
	if err != nil {
		t.Fatal(err)
	}
	if result := findDataManagementResult(response, "dependencies"); result == nil || result.Status != "denied" {
		t.Fatalf("dependencies must always reject individual cleanup: %#v", result)
	}
	if _, err := os.Stat(tool); err != nil {
		t.Fatalf("busy dependency must remain: %v", err)
	}
}

func TestDataManagementBlocksComponentsWhileDependencyInstallIsActive(t *testing.T) {
	handler, configRoot, _, _, _ := dataManagementTestHandler(t, false)
	handler.config.Dependencies = dataManagementDependencyActivityStub{active: true}
	tool := filepath.Join(configRoot, "dependencies", "yt-dlp", "yt-dlp")
	writeDataManagementFixture(t, tool, "tool")

	snapshot, err := handler.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if item := findDataManagementItem(snapshot, "dependencies"); item == nil || item.State != "busy" || item.Clearable {
		t.Fatalf("active dependency install must make components busy: %#v", item)
	}
	response, err := handler.Clean(context.Background(), CleanDataManagementRequest{ResourceIDs: []string{"dependencies"}})
	if err != nil {
		t.Fatal(err)
	}
	if result := findDataManagementResult(response, "dependencies"); result == nil || result.Status != "denied" {
		t.Fatalf("dependencies must always reject individual cleanup: %#v", result)
	}
	if _, err := os.Stat(tool); err != nil {
		t.Fatalf("dependency must remain during install: %v", err)
	}
}

func TestSafeRemoveFileRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	writeDataManagementFixture(t, target, "keep")
	link := filepath.Join(root, "app-old.log.gz")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := safeRemoveFile(link, root); err == nil {
		t.Fatal("expected symlink cleanup to be rejected")
	}
	if payload, err := os.ReadFile(target); err != nil || string(payload) != "keep" {
		t.Fatalf("symlink target was modified: %q %v", payload, err)
	}
}

func TestDataManagementRejectsIntermediateSymlink(t *testing.T) {
	handler, _, cacheRoot, _, _ := dataManagementTestHandler(t, false)
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external-cache")
	externalFile := filepath.Join(external, "com.xiadown.listen.imagecache", "keep.png")
	writeDataManagementFixture(t, externalFile, "keep")
	if err := os.Symlink(external, filepath.Join(cacheRoot, "xiadown")); err != nil {
		t.Fatal(err)
	}

	snapshot, err := handler.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshotResourceBytes(snapshot, "image-cache"); got != 0 {
		t.Fatalf("symlinked cache must not be inventoried, got %d bytes", got)
	}
	response, err := handler.Clean(context.Background(), CleanDataManagementRequest{ResourceIDs: []string{"image-cache"}})
	if err != nil {
		t.Fatal(err)
	}
	if result := findDataManagementResult(response, "image-cache"); result == nil || result.Status != "failed" {
		t.Fatalf("symlinked cache cleanup should fail closed: %#v", result)
	}
	if payload, err := os.ReadFile(externalFile); err != nil || string(payload) != "keep" {
		t.Fatalf("external cache was modified: %q %v", payload, err)
	}
}

func TestDataManagementRejectsSymlinkedTrashDirectory(t *testing.T) {
	handler, _, cacheRoot, _, _ := dataManagementTestHandler(t, false)
	cacheFile := filepath.Join(cacheRoot, "xiadown", "com.xiadown.listen.imagecache", "keep.png")
	writeDataManagementFixture(t, cacheFile, "keep")
	external := filepath.Join(t.TempDir(), "external-trash")
	if err := os.MkdirAll(external, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(cacheRoot, ".xiadown-trash")); err != nil {
		t.Fatal(err)
	}

	response, err := handler.Clean(context.Background(), CleanDataManagementRequest{ResourceIDs: []string{"image-cache"}})
	if err != nil {
		t.Fatal(err)
	}
	if result := findDataManagementResult(response, "image-cache"); result == nil || result.Status != "failed" {
		t.Fatalf("symlinked trash cleanup should fail closed: %#v", result)
	}
	if payload, err := os.ReadFile(cacheFile); err != nil || string(payload) != "keep" {
		t.Fatalf("cache was modified after rejected cleanup: %q %v", payload, err)
	}
}

func TestDataManagementOnlyClearsRecognizedRotatedLogs(t *testing.T) {
	handler, _, _, logRoot, _ := dataManagementTestHandler(t, false)
	rotated := filepath.Join(logRoot, "app-2026-07-17.log.gz")
	unrelatedGzip := filepath.Join(logRoot, "user-archive.gz")
	prefixOnly := filepath.Join(logRoot, "app-important-notes.txt")
	writeDataManagementFixture(t, rotated, "archive")
	writeDataManagementFixture(t, unrelatedGzip, "keep-gzip")
	writeDataManagementFixture(t, prefixOnly, "keep-notes")

	response, err := handler.Clean(context.Background(), CleanDataManagementRequest{ResourceIDs: []string{"archived-logs"}})
	if err != nil {
		t.Fatal(err)
	}
	if result := findDataManagementResult(response, "archived-logs"); result == nil || result.Status != "cleared" {
		t.Fatalf("expected recognized log cleanup: %#v", result)
	}
	if _, err := os.Stat(rotated); !os.IsNotExist(err) {
		t.Fatalf("recognized rotated log should be removed: %v", err)
	}
	for _, path := range []string{unrelatedGzip, prefixOnly} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("unrelated file must be preserved: %s: %v", path, err)
		}
	}
}

func TestDataManagementResetSchedulesThenQuits(t *testing.T) {
	handler, _, _, _, _ := dataManagementTestHandler(t, false)
	resetter := new(dataManagementResetterStub)
	quit := make(chan struct{}, 1)
	handler.config.Resetter = resetter
	handler.config.Quitter = dataManagementQuitterStub{called: quit}

	response, err := handler.ResetApplication(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !response.Scheduled || resetter.calls != 1 {
		t.Fatalf("unexpected reset response=%#v calls=%d", response, resetter.calls)
	}
	// Repeated requests are idempotent and do not replace the durable marker.
	response, err = handler.ResetApplication(context.Background())
	if err != nil || !response.Scheduled || resetter.calls != 1 {
		t.Fatalf("repeated reset response=%#v calls=%d err=%v", response, resetter.calls, err)
	}
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("application was not asked to quit after scheduling reset")
	}
}

func TestDataManagementResetRefusesActiveOperations(t *testing.T) {
	handler, _, _, _, _ := dataManagementTestHandler(t, true)
	resetter := new(dataManagementResetterStub)
	handler.config.Resetter = resetter
	handler.config.Quitter = dataManagementQuitterStub{called: make(chan struct{}, 1)}

	if _, err := handler.ResetApplication(context.Background()); err == nil {
		t.Fatal("reset should be refused while data operations are active")
	}
	if resetter.calls != 0 {
		t.Fatalf("busy reset wrote marker %d times", resetter.calls)
	}

	handler.config.Activity = dataManagementActivityStub{}
	handler.config.Dependencies = dataManagementDependencyActivityStub{active: true}
	if _, err := handler.ResetApplication(context.Background()); err == nil {
		t.Fatal("reset should be refused while dependency installation is active")
	}
	if resetter.calls != 0 {
		t.Fatalf("dependency-busy reset wrote marker %d times", resetter.calls)
	}
}

func TestDataManagementResetDoesNotQuitWhenMarkerFails(t *testing.T) {
	handler, _, _, _, _ := dataManagementTestHandler(t, false)
	resetter := &dataManagementResetterStub{err: errors.New("disk unavailable")}
	quit := make(chan struct{}, 1)
	handler.config.Resetter = resetter
	handler.config.Quitter = dataManagementQuitterStub{called: quit}

	if _, err := handler.ResetApplication(context.Background()); err == nil {
		t.Fatal("expected marker scheduling failure")
	}
	select {
	case <-quit:
		t.Fatal("application quit even though reset marker was not durable")
	case <-time.After(250 * time.Millisecond):
	}
}

func findDataManagementItem(snapshot DataManagementSnapshot, id string) *DataManagementItem {
	for categoryIndex := range snapshot.Categories {
		for itemIndex := range snapshot.Categories[categoryIndex].Items {
			item := &snapshot.Categories[categoryIndex].Items[itemIndex]
			if item.ID == id {
				return item
			}
		}
	}
	return nil
}

func findDataManagementResult(response CleanDataManagementResponse, id string) *DataManagementCleanResult {
	for index := range response.Results {
		if response.Results[index].ResourceID == id {
			return &response.Results[index]
		}
	}
	return nil
}
