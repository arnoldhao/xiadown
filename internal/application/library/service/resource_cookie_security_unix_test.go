//go:build !windows

package service

import (
	"os"
	"testing"
)

func assertPrivateResourceCookieJar(t *testing.T, _ string, mode os.FileMode) {
	t.Helper()
	if permission := mode.Perm(); permission != 0o600 {
		t.Fatalf("temporary jar mode = %o, want 600", permission)
	}
}
