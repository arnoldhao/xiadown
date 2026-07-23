package app

import (
	"context"
	"fmt"
	"os"

	"xiadown/internal/infrastructure/appreset"
	"xiadown/internal/infrastructure/appsessionvault"
	"xiadown/internal/infrastructure/logging"
)

func newApplicationResetManager() (*appreset.Manager, error) {
	configBase, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve application reset config directory: %w", err)
	}
	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("resolve application reset cache directory: %w", err)
	}
	logRoot, err := logging.DefaultLogDir()
	if err != nil {
		return nil, fmt.Errorf("resolve application reset log directory: %w", err)
	}
	return appreset.New(
		appreset.PathsForRoots(configBase, cacheBase, logRoot),
		appsessionvault.DeleteApplicationResetSecrets,
	)
}

// ApplyPendingApplicationReset must run before startup logging and database
// initialization. A pending marker therefore cannot expose an old database
// after its Session Vault key has already been removed.
func ApplyPendingApplicationReset(ctx context.Context) (bool, error) {
	manager, err := newApplicationResetManager()
	if err != nil {
		return false, err
	}
	result, err := manager.ApplyPending(ctx)
	return result.Applied, err
}
