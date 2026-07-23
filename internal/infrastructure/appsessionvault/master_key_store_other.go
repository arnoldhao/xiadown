//go:build (!darwin && !windows) || ios

package appsessionvault

import (
	"context"

	"xiadown/internal/domain/appsessions"
)

type unsupportedMasterKeyStore struct{}

func newPlatformMasterKeyStore() masterKeyStore {
	return unsupportedMasterKeyStore{}
}

func inspectPlatformMasterKey(context.Context) (int, int64, error) { return 0, 0, nil }

func (unsupportedMasterKeyStore) Load(context.Context) ([]byte, error) {
	return nil, appsessions.ErrUnsupported
}

func (unsupportedMasterKeyStore) Store(context.Context, []byte) error {
	return appsessions.ErrUnsupported
}

func (unsupportedMasterKeyStore) Delete(context.Context) error { return nil }

func deleteLegacyAppSessionSecrets(context.Context) error { return nil }
