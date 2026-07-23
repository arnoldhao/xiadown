package appsessionvault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/appsessions"
	"xiadown/internal/infrastructure/persistence/sqlitedto"
)

const (
	vaultKeyID         = masterKeyAccount
	vaultFormatVersion = 1
	maxSecretBytes     = 4 << 20
	maxSiteKeyBytes    = 128
)

type vaultRow = sqlitedto.AppSessionSecretRow

// Vault encrypts App Session secrets before they reach SQLite. One device-
// local master key protects every row; that key is loaded or created once per
// Vault instance and then retained only in process memory.
type Vault struct {
	db       *bun.DB
	keyStore masterKeyStore
	rand     io.Reader
	now      func() time.Time

	keyMu     sync.Mutex
	masterKey []byte
}

func New(db *bun.DB) *Vault {
	return &Vault{
		db:       db,
		keyStore: newPlatformMasterKeyStore(),
		rand:     rand.Reader,
		now:      time.Now,
	}
}

func (vault *Vault) LoadAppSessionSecret(ctx context.Context, siteKey string) ([]byte, error) {
	if vault == nil || vault.db == nil {
		return nil, appsessions.ErrUnsupported
	}
	normalized, err := normalizeSiteKey(siteKey)
	if err != nil {
		return nil, err
	}
	ctx = nonNilContext(ctx)
	row := new(vaultRow)
	if err := vault.db.NewSelect().Model(row).Where("site_key = ?", normalized).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appsessions.ErrNoCookies
		}
		return nil, fmt.Errorf("load App Session vault row: %w", err)
	}
	if row.KeyID != vaultKeyID || row.FormatVersion != vaultFormatVersion {
		return nil, fmt.Errorf("unsupported App Session vault format")
	}
	if len(row.Ciphertext) > maxSecretBytes+aes.BlockSize || len(row.Nonce) != 12 {
		return nil, fmt.Errorf("invalid App Session vault ciphertext")
	}
	key, err := vault.cachedMasterKey(ctx)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, row.Nonce, row.Ciphertext, vaultAdditionalData(normalized))
	if err != nil {
		return nil, fmt.Errorf("authenticate App Session vault secret: %w", err)
	}
	if len(plaintext) == 0 || len(plaintext) > maxSecretBytes {
		return nil, fmt.Errorf("invalid App Session vault plaintext")
	}
	return plaintext, nil
}

func (vault *Vault) SaveAppSessionSecret(ctx context.Context, siteKey string, plaintext []byte) error {
	if vault == nil || vault.db == nil {
		return appsessions.ErrUnsupported
	}
	ctx = nonNilContext(ctx)
	row, err := vault.sealAppSessionSecret(ctx, siteKey, plaintext)
	if err != nil {
		return err
	}
	if _, err := vault.db.NewInsert().Model(&row).
		On("CONFLICT(site_key) DO UPDATE").
		Set("key_id = EXCLUDED.key_id").
		Set("format_version = EXCLUDED.format_version").
		Set("nonce = EXCLUDED.nonce").
		Set("ciphertext = EXCLUDED.ciphertext").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("save App Session vault row: %w", err)
	}
	return nil
}

// CommitImportedAppSession atomically replaces one site's encrypted secret
// and metadata. It never opens or mutates the source browser profile.
func (vault *Vault) CommitImportedAppSession(ctx context.Context, session appsessions.Session, plaintext []byte) error {
	if vault == nil || vault.db == nil {
		return appsessions.ErrUnsupported
	}
	ctx = nonNilContext(ctx)
	secret, err := vault.sealAppSessionSecret(ctx, session.SiteKey, plaintext)
	if err != nil {
		return err
	}
	metadata, err := appSessionRow(session)
	if err != nil {
		return err
	}
	tx, err := vault.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin imported App Session commit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.NewInsert().Model(&metadata).
		On("CONFLICT(id) DO UPDATE").
		Set("site_key = EXCLUDED.site_key").
		Set("status = EXCLUDED.status").
		Set("account_display_name = EXCLUDED.account_display_name").
		Set("account_handle = EXCLUDED.account_handle").
		Set("account_avatar_url = EXCLUDED.account_avatar_url").
		Set("account_tier_key = EXCLUDED.account_tier_key").
		Set("account_tier_label = EXCLUDED.account_tier_label").
		Set("account_badges_json = EXCLUDED.account_badges_json").
		Set("account_metadata_json = EXCLUDED.account_metadata_json").
		Set("account_verification_status = EXCLUDED.account_verification_status").
		Set("account_verification_error = EXCLUDED.account_verification_error").
		Set("account_verification_started_at = EXCLUDED.account_verification_started_at").
		Set("last_verified_at = EXCLUDED.last_verified_at").
		Set("source_type = EXCLUDED.source_type").
		Set("source_browser = EXCLUDED.source_browser").
		Set("source_profile = EXCLUDED.source_profile").
		Set("last_synced_at = EXCLUDED.last_synced_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("save imported App Session metadata: %w", err)
	}
	if _, err := tx.NewInsert().Model(&secret).
		On("CONFLICT(site_key) DO UPDATE").
		Set("key_id = EXCLUDED.key_id").
		Set("format_version = EXCLUDED.format_version").
		Set("nonce = EXCLUDED.nonce").
		Set("ciphertext = EXCLUDED.ciphertext").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("save imported App Session secret: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit imported App Session: %w", err)
	}
	return nil
}

func (vault *Vault) sealAppSessionSecret(ctx context.Context, siteKey string, plaintext []byte) (vaultRow, error) {
	normalized, err := normalizeSiteKey(siteKey)
	if err != nil {
		return vaultRow{}, err
	}
	if len(plaintext) == 0 {
		return vaultRow{}, appsessions.ErrNoCookies
	}
	if len(plaintext) > maxSecretBytes {
		return vaultRow{}, fmt.Errorf("App Session vault secret exceeds %d bytes", maxSecretBytes)
	}
	key, err := vault.cachedMasterKey(ctx)
	if err != nil {
		return vaultRow{}, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return vaultRow{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(vault.randomSource(), nonce); err != nil {
		return vaultRow{}, fmt.Errorf("generate App Session vault nonce: %w", err)
	}
	now := vault.currentTime().UTC().Round(0)
	return vaultRow{
		SiteKey:       normalized,
		KeyID:         vaultKeyID,
		FormatVersion: vaultFormatVersion,
		Nonce:         nonce,
		Ciphertext:    gcm.Seal(nil, nonce, plaintext, vaultAdditionalData(normalized)),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func appSessionRow(session appsessions.Session) (sqlitedto.AppSessionRow, error) {
	if strings.TrimSpace(session.ID) == "" || strings.TrimSpace(session.SiteKey) == "" {
		return sqlitedto.AppSessionRow{}, appsessions.ErrInvalidSession
	}
	createdAt := session.CreatedAt
	updatedAt := session.UpdatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return sqlitedto.AppSessionRow{
		ID:                           session.ID,
		SiteKey:                      strings.ToLower(strings.TrimSpace(session.SiteKey)),
		Status:                       string(session.Status),
		AccountDisplayName:           nullableString(session.AccountDisplayName),
		AccountHandle:                nullableString(session.AccountHandle),
		AccountAvatarURL:             nullableString(session.AccountAvatarURL),
		AccountTierKey:               nullableString(session.AccountTierKey),
		AccountTierLabel:             nullableString(session.AccountTierLabel),
		AccountBadgesJSON:            nullableString(session.AccountBadgesJSON),
		AccountMetadataJSON:          nullableString(session.AccountMetadataJSON),
		AccountVerificationStatus:    nullableString(string(session.AccountVerificationStatus)),
		AccountVerificationError:     nullableString(session.AccountVerificationError),
		AccountVerificationStartedAt: nullableTime(session.AccountVerificationStartedAt),
		LastVerifiedAt:               nullableTime(session.LastVerifiedAt),
		SourceType:                   nullableString(string(session.SourceType)),
		SourceBrowser:                nullableString(session.SourceBrowser),
		SourceProfile:                nullableString(session.SourceProfile),
		LastSyncedAt:                 nullableTime(session.LastSyncedAt),
		CreatedAt:                    createdAt,
		UpdatedAt:                    updatedAt,
	}, nil
}

func nullableString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	return sql.NullString{String: trimmed, Valid: trimmed != ""}
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}

func (vault *Vault) DeleteAppSessionSecret(ctx context.Context, siteKey string) error {
	if vault == nil || vault.db == nil {
		return appsessions.ErrUnsupported
	}
	normalized, err := normalizeSiteKey(siteKey)
	if err != nil {
		return err
	}
	ctx = nonNilContext(ctx)
	if _, err := vault.db.NewDelete().Model((*vaultRow)(nil)).Where("site_key = ?", normalized).Exec(ctx); err != nil {
		return fmt.Errorf("delete App Session vault row: %w", err)
	}
	return nil
}

func (vault *Vault) cachedMasterKey(ctx context.Context) ([]byte, error) {
	if vault == nil || vault.keyStore == nil {
		return nil, appsessions.ErrUnsupported
	}
	vault.keyMu.Lock()
	defer vault.keyMu.Unlock()
	if len(vault.masterKey) == masterKeyBytes {
		return vault.masterKey, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, err := vault.keyStore.Load(ctx)
	if errors.Is(err, errMasterKeyNotFound) {
		candidate := make([]byte, masterKeyBytes)
		if _, randomErr := io.ReadFull(vault.randomSource(), candidate); randomErr != nil {
			return nil, fmt.Errorf("generate App Session vault master key: %w", randomErr)
		}
		if storeErr := vault.keyStore.Store(ctx, candidate); storeErr != nil {
			if !errors.Is(storeErr, errMasterKeyAlreadyExists) {
				return nil, fmt.Errorf("store App Session vault master key: %w", storeErr)
			}
			key, err = vault.keyStore.Load(ctx)
			if err != nil {
				return nil, fmt.Errorf("load concurrent App Session vault master key: %w", err)
			}
		} else {
			key = candidate
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("load App Session vault master key: %w", err)
	}
	if len(key) != masterKeyBytes {
		return nil, fmt.Errorf("invalid App Session vault master key length")
	}
	vault.masterKey = append([]byte(nil), key...)
	return vault.masterKey, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize App Session vault cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize App Session vault GCM: %w", err)
	}
	return gcm, nil
}

func vaultAdditionalData(siteKey string) []byte {
	return []byte("xiadown:app-session-vault\x00master-key\x00format-v1\x00" + siteKey)
}

func normalizeSiteKey(siteKey string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(siteKey))
	if normalized == "" || len(normalized) > maxSiteKeyBytes {
		return "", appsessions.ErrInvalidSession
	}
	for _, current := range normalized {
		switch {
		case current >= 'a' && current <= 'z',
			current >= '0' && current <= '9',
			current == '-', current == '_', current == '.':
		default:
			return "", appsessions.ErrInvalidSession
		}
	}
	return normalized, nil
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (vault *Vault) randomSource() io.Reader {
	if vault != nil && vault.rand != nil {
		return vault.rand
	}
	return rand.Reader
}

func (vault *Vault) currentTime() time.Time {
	if vault != nil && vault.now != nil {
		return vault.now()
	}
	return time.Now()
}
