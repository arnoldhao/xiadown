package appsessionvault

import (
	"context"
	"errors"
)

const (
	// DarwinKeychainService is intentionally unrelated to the legacy
	// connector-app-session service. The only item in this service is the
	// process-cached vault master key.
	DarwinKeychainService = "com.dreamapp.xiadown.session-vault"
	legacyKeychainService = "com.dreamapp.xiadown.connector-app-session"
	masterKeyAccount      = "master-key"
	masterKeyBytes        = 32
)

var (
	errMasterKeyNotFound      = errors.New("session vault master key not found")
	errMasterKeyAlreadyExists = errors.New("session vault master key already exists")
)

type masterKeyStore interface {
	Load(ctx context.Context) ([]byte, error)
	Store(ctx context.Context, key []byte) error
	Delete(ctx context.Context) error
}

// MasterKeyInventory reports only storage metadata for the single device-local
// Session Vault key. Platform implementations must not load, decrypt, or return
// the key material while producing this inventory.
func MasterKeyInventory(ctx context.Context) (itemCount int, sizeBytes int64, err error) {
	return inspectPlatformMasterKey(ctx)
}

// DeleteMasterKey removes the single device-local key used by the current
// Session Vault. It is intentionally separate from deleting individual
// sessions and is only used by the whole-application reset workflow.
func DeleteMasterKey(ctx context.Context) error {
	return newPlatformMasterKeyStore().Delete(ctx)
}

// DeleteApplicationResetSecrets clears both the current single-key vault and
// credentials left by the retired per-session store. Both operations are
// idempotent so a crash-interrupted reset can safely retry on the next launch.
func DeleteApplicationResetSecrets(ctx context.Context) error {
	return errors.Join(
		DeleteMasterKey(ctx),
		deleteLegacyAppSessionSecrets(ctx),
	)
}
