package appsessionvault

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"xiadown/internal/domain/appsessions"
	"xiadown/internal/infrastructure/persistence"
)

type memoryMasterKeyStore struct {
	mu         sync.Mutex
	key        []byte
	loadCalls  int
	storeCalls int
}

func (store *memoryMasterKeyStore) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.loadCalls++
	if len(store.key) == 0 {
		return nil, errMasterKeyNotFound
	}
	return append([]byte(nil), store.key...), nil
}

func (store *memoryMasterKeyStore) Store(ctx context.Context, key []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.storeCalls++
	if len(store.key) != 0 {
		return errMasterKeyAlreadyExists
	}
	store.key = append([]byte(nil), key...)
	return nil
}

func (store *memoryMasterKeyStore) Delete(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.key = nil
	return nil
}

func TestVaultEncryptsPerSiteAndCachesSingleMasterKey(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path:                     filepath.Join(t.TempDir(), "vault.db"),
		SkipPreMigrationSnapshot: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	seedVaultSessions(t, ctx, database, "youtube", "bilibili")

	keyStore := new(memoryMasterKeyStore)
	vault := New(database.Bun)
	vault.keyStore = keyStore
	youtubeSecret := []byte(`[{"name":"SAPISID","value":"youtube-secret"}]`)
	bilibiliSecret := []byte(`[{"name":"SESSDATA","value":"bilibili-secret"}]`)
	if err := vault.SaveAppSessionSecret(ctx, " YouTube ", youtubeSecret); err != nil {
		t.Fatal(err)
	}
	if err := vault.SaveAppSessionSecret(ctx, "bilibili", bilibiliSecret); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_session_secrets`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("vault row count = %d, want 2", count)
	}
	var nonce, ciphertext []byte
	if err := database.SQL.QueryRowContext(ctx, `
SELECT nonce, ciphertext FROM app_session_secrets WHERE site_key = 'youtube'
`).Scan(&nonce, &ciphertext); err != nil {
		t.Fatal(err)
	}
	if len(nonce) != 12 || len(ciphertext) <= len(youtubeSecret) {
		t.Fatalf("nonce/ciphertext lengths = %d/%d", len(nonce), len(ciphertext))
	}
	if bytes.Contains(ciphertext, []byte("youtube-secret")) {
		t.Fatal("SQLite ciphertext contains plaintext cookie value")
	}
	loaded, err := vault.LoadAppSessionSecret(ctx, "youtube")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, youtubeSecret) {
		t.Fatalf("loaded secret = %q, want %q", loaded, youtubeSecret)
	}
	if keyStore.loadCalls != 1 || keyStore.storeCalls != 1 || len(keyStore.key) != masterKeyBytes {
		t.Fatalf("master key store calls load=%d store=%d keyBytes=%d", keyStore.loadCalls, keyStore.storeCalls, len(keyStore.key))
	}

	secondProcessVault := New(database.Bun)
	secondProcessVault.keyStore = keyStore
	loaded, err = secondProcessVault.LoadAppSessionSecret(ctx, "bilibili")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, bilibiliSecret) {
		t.Fatalf("second vault loaded secret = %q, want %q", loaded, bilibiliSecret)
	}
	if keyStore.loadCalls != 2 || keyStore.storeCalls != 1 {
		t.Fatalf("second vault store calls load=%d store=%d", keyStore.loadCalls, keyStore.storeCalls)
	}
}

func TestVaultUsesUniqueSiteRowAndFreshNonceOnOverwrite(t *testing.T) {
	ctx, database, vault := newTestVault(t)
	first := []byte(`[{"value":"first"}]`)
	second := []byte(`[{"value":"second"}]`)
	if err := vault.SaveAppSessionSecret(ctx, "youtube", first); err != nil {
		t.Fatal(err)
	}
	var firstNonce, firstCiphertext []byte
	if err := database.SQL.QueryRowContext(ctx, `SELECT nonce, ciphertext FROM app_session_secrets WHERE site_key = 'youtube'`).Scan(&firstNonce, &firstCiphertext); err != nil {
		t.Fatal(err)
	}
	if err := vault.SaveAppSessionSecret(ctx, "YOUTUBE", second); err != nil {
		t.Fatal(err)
	}
	var count int
	var secondNonce, secondCiphertext []byte
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_session_secrets`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT nonce, ciphertext FROM app_session_secrets WHERE site_key = 'youtube'`).Scan(&secondNonce, &secondCiphertext); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("vault row count = %d, want 1", count)
	}
	if bytes.Equal(firstNonce, secondNonce) || bytes.Equal(firstCiphertext, secondCiphertext) {
		t.Fatal("vault overwrite reused nonce or ciphertext")
	}
	loaded, err := vault.LoadAppSessionSecret(ctx, "youtube")
	if err != nil || !bytes.Equal(loaded, second) {
		t.Fatalf("loaded overwrite = %q, err=%v", loaded, err)
	}
}

func TestVaultFailsClosedOnCiphertextTamperingAndSiteSwap(t *testing.T) {
	ctx, database, vault := newTestVault(t)
	secret := []byte(`[{"value":"secret"}]`)
	if err := vault.SaveAppSessionSecret(ctx, "youtube", secret); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO app_session_secrets (
	site_key, key_id, format_version, nonce, ciphertext, created_at, updated_at
)
SELECT 'bilibili', key_id, format_version, nonce, ciphertext, created_at, updated_at
FROM app_session_secrets WHERE site_key = 'youtube'
`); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.LoadAppSessionSecret(ctx, "bilibili"); err == nil || errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("site-swapped ciphertext error = %v, want authentication failure", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE app_session_secrets
SET ciphertext = CAST(substr(ciphertext, 1, length(ciphertext) - 1) || char(0) AS BLOB)
WHERE site_key = 'youtube'
`); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.LoadAppSessionSecret(ctx, "youtube"); err == nil || errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("tampered ciphertext error = %v, want authentication failure", err)
	}
}

func TestVaultMissingAndDeleteDoNotLoadMasterKey(t *testing.T) {
	ctx, _, vault := newTestVault(t)
	store := vault.keyStore.(*memoryMasterKeyStore)
	if _, err := vault.LoadAppSessionSecret(ctx, "youtube"); !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("missing secret error = %v", err)
	}
	if err := vault.DeleteAppSessionSecret(ctx, "youtube"); err != nil {
		t.Fatal(err)
	}
	if store.loadCalls != 0 || store.storeCalls != 0 {
		t.Fatalf("empty operations touched master key store: load=%d store=%d", store.loadCalls, store.storeCalls)
	}
}

func TestCommitImportedAppSessionAtomicallyReplacesOneSite(t *testing.T) {
	ctx, database, vault := newTestVault(t)
	createdAt := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	lastSyncedAt := updatedAt
	session, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                        "site-app-session-youtube",
		SiteKey:                   "youtube",
		Status:                    string(appsessions.StatusConnected),
		SourceType:                string(appsessions.SourceTypeBrowserProfile),
		SourceBrowser:             "chrome",
		SourceProfile:             "profile-opaque",
		LastSyncedAt:              &lastSyncedAt,
		AccountVerificationStatus: string(appsessions.AccountVerificationUnverified),
		CreatedAt:                 &createdAt,
		UpdatedAt:                 &updatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte(`[{"name":"SAPISID","value":"imported-secret"}]`)
	if err := vault.CommitImportedAppSession(ctx, session, secret); err != nil {
		t.Fatal(err)
	}

	var rows int
	var status, sourceType, sourceBrowser, sourceProfile string
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*), max(status), max(source_type), max(source_browser), max(source_profile)
FROM app_sessions WHERE site_key = 'youtube'
`).Scan(&rows, &status, &sourceType, &sourceBrowser, &sourceProfile); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || status != "connected" || sourceType != "browser_profile" ||
		sourceBrowser != "chrome" || sourceProfile != "profile-opaque" {
		t.Fatalf("committed metadata rows=%d status=%q source=%q/%q/%q",
			rows, status, sourceType, sourceBrowser, sourceProfile)
	}
	loaded, err := vault.LoadAppSessionSecret(ctx, "youtube")
	if err != nil || !bytes.Equal(loaded, secret) {
		t.Fatalf("committed secret = %q, err=%v", loaded, err)
	}
}

func TestCommitImportedAppSessionRollsBackMetadataWhenSecretWriteFails(t *testing.T) {
	ctx, database, vault := newTestVault(t)
	oldSecret := []byte(`[{"name":"SAPISID","value":"old-secret"}]`)
	if err := vault.SaveAppSessionSecret(ctx, "youtube", oldSecret); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE app_sessions
SET status = 'connected', source_type = 'xiadown_profile', updated_at = '2026-07-17T09:00:00Z'
WHERE site_key = 'youtube';
CREATE TRIGGER reject_app_session_secret_replace
BEFORE INSERT ON app_session_secrets
WHEN NEW.site_key = 'youtube'
BEGIN
	SELECT RAISE(ABORT, 'forced secret failure');
END;
`); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	replacement, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                        "site-app-session-youtube",
		SiteKey:                   "youtube",
		Status:                    string(appsessions.StatusConnected),
		SourceType:                string(appsessions.SourceTypeBrowserProfile),
		SourceBrowser:             "edge",
		SourceProfile:             "profile-replacement",
		AccountVerificationStatus: string(appsessions.AccountVerificationUnverified),
		CreatedAt:                 &createdAt,
		UpdatedAt:                 &updatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.CommitImportedAppSession(ctx, replacement, []byte(`[{"value":"new-secret"}]`)); err == nil {
		t.Fatal("forced secret failure unexpectedly committed")
	}

	var status, sourceType string
	var sourceBrowser, sourceProfile sql.NullString
	if err := database.SQL.QueryRowContext(ctx, `
SELECT status, source_type, source_browser, source_profile
FROM app_sessions WHERE site_key = 'youtube'
`).Scan(&status, &sourceType, &sourceBrowser, &sourceProfile); err != nil {
		t.Fatal(err)
	}
	if status != "connected" || sourceType != "xiadown_profile" || sourceBrowser.Valid || sourceProfile.Valid {
		t.Fatalf("metadata changed despite rolled-back secret: status=%q source=%q/%v/%v",
			status, sourceType, sourceBrowser, sourceProfile)
	}
	loaded, err := vault.LoadAppSessionSecret(ctx, "youtube")
	if err != nil || !bytes.Equal(loaded, oldSecret) {
		t.Fatalf("old secret was not preserved: value=%q err=%v", loaded, err)
	}
}

func TestDarwinKeychainServiceIsVaultOnly(t *testing.T) {
	if DarwinKeychainService != "com.dreamapp.xiadown.session-vault" {
		t.Fatalf("Darwin Keychain service = %q", DarwinKeychainService)
	}
	if masterKeyAccount != "master-key" || vaultKeyID != masterKeyAccount {
		t.Fatalf("vault master key account/id = %q/%q", masterKeyAccount, vaultKeyID)
	}
}

func newTestVault(t *testing.T) (context.Context, *persistence.Database, *Vault) {
	t.Helper()
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path:                     filepath.Join(t.TempDir(), "vault.db"),
		SkipPreMigrationSnapshot: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	vault := New(database.Bun)
	vault.keyStore = new(memoryMasterKeyStore)
	seedVaultSessions(t, ctx, database, "youtube", "bilibili")
	return ctx, database, vault
}

func seedVaultSessions(t *testing.T, ctx context.Context, database *persistence.Database, siteKeys ...string) {
	t.Helper()
	for _, siteKey := range siteKeys {
		if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO app_sessions (
	id, site_key, status, account_verification_status, created_at, updated_at
) VALUES (?, ?, 'disconnected', 'unverified', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(site_key) DO NOTHING
`, "site-app-session-"+siteKey, siteKey); err != nil {
			t.Fatal(err)
		}
	}
}
