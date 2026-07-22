package wails

import (
	"context"
	"errors"
	"testing"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

type appSessionSecretVaultStub struct {
	secrets map[string][]byte
}

func (vault *appSessionSecretVaultStub) LoadAppSessionSecret(_ context.Context, siteKey string) ([]byte, error) {
	value := vault.secrets[siteKey]
	if len(value) == 0 {
		return nil, appsessions.ErrNoCookies
	}
	return append([]byte(nil), value...), nil
}

func (vault *appSessionSecretVaultStub) SaveAppSessionSecret(_ context.Context, siteKey string, plaintext []byte) error {
	if vault.secrets == nil {
		vault.secrets = make(map[string][]byte)
	}
	vault.secrets[siteKey] = append([]byte(nil), plaintext...)
	return nil
}

func (vault *appSessionSecretVaultStub) DeleteAppSessionSecret(_ context.Context, siteKey string) error {
	delete(vault.secrets, siteKey)
	return nil
}

func TestNativeAppSessionProviderPersistsOnlyThroughInjectedVault(t *testing.T) {
	ctx := context.Background()
	vault := &appSessionSecretVaultStub{}
	provider := NewNativeAppSessionProvider(nil, vault)
	want := []appcookies.Record{{
		Name: "SESSDATA", Value: "secret", Domain: ".bilibili.com", Path: "/", Secure: true,
	}}
	if err := provider.SaveAppSessionCookies(ctx, "bilibili", want); err != nil {
		t.Fatal(err)
	}
	provider.cookieCache = nativeAppSessionCookieCache{}
	got, err := provider.LoadAppSessionCookies(ctx, "bilibili")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("vault round trip = %#v, want %#v", got, want)
	}
	if err := provider.clearStoredCookies(ctx, "bilibili", nil); err != nil {
		t.Fatal(err)
	}
	provider.cookieCache = nativeAppSessionCookieCache{}
	if _, err := provider.LoadAppSessionCookies(ctx, "bilibili"); !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("load after vault delete error = %v", err)
	}
}

func TestNativeAppSessionProviderHasNoPlatformPersistenceFallback(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	if err := provider.SaveAppSessionCookies(context.Background(), "bilibili", []appcookies.Record{{
		Name: "SESSDATA", Value: "secret", Domain: ".bilibili.com", Path: "/",
	}}); !errors.Is(err, appsessions.ErrUnsupported) {
		t.Fatalf("save without vault error = %v, want unsupported", err)
	}
}
