//go:build darwin && !ios

package appsessionvault

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// TestDarwinSessionVaultKeychainSmoke is opt-in because it writes a temporary
// generic-password item. Running the store phase with one build and the load
// phase with a newly signed build at the same executable path verifies the
// in-place update contract: the Keychain ACL follows the stable designated
// requirement rather than a particular executable hash.
func TestDarwinSessionVaultKeychainSmoke(t *testing.T) {
	mode := os.Getenv("XIADOWN_SESSION_VAULT_KEYCHAIN_SMOKE")
	if mode == "" {
		t.Skip("requires an explicitly selected Keychain smoke-test phase")
	}
	store := darwinMasterKeyStore{
		service: DarwinKeychainService + ".smoke-test",
		account: "stable-designated-requirement",
	}
	ctx := context.Background()
	want := bytes.Repeat([]byte{0x5a}, masterKeyBytes)
	switch mode {
	case "store":
		if err := store.Delete(context.Background()); err != nil {
			t.Fatalf("remove stale smoke-test key: %v", err)
		}
		if err := store.Store(ctx, want); err != nil {
			t.Fatalf("store Session Vault Keychain smoke-test key: %v", err)
		}
	case "load":
		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("load Session Vault Keychain smoke-test key: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("Session Vault Keychain smoke-test key changed during round trip")
		}
	case "delete":
		if err := store.Delete(ctx); err != nil {
			t.Fatalf("remove Session Vault Keychain smoke-test key: %v", err)
		}
	default:
		t.Fatalf("unknown Keychain smoke-test phase %q for pid %d", mode, os.Getpid())
	}
}
