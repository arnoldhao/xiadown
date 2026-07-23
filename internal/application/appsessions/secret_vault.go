package appsessions

import "context"

// SecretVault is the durable source for App Session secrets. Implementations
// must keep the plaintext out of application metadata tables and bind every
// stored secret to its normalized site key.
type SecretVault interface {
	LoadAppSessionSecret(ctx context.Context, siteKey string) ([]byte, error)
	SaveAppSessionSecret(ctx context.Context, siteKey string, plaintext []byte) error
	DeleteAppSessionSecret(ctx context.Context, siteKey string) error
}
