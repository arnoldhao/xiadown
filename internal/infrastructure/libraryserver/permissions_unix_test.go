//go:build !windows

package libraryserver

import (
	"os"
	"testing"
)

func assertPrivateTLSKey(t *testing.T, _ string, mode os.FileMode) {
	t.Helper()
	if permissions := mode.Perm(); permissions != 0o600 {
		t.Fatalf("private key permissions are %o, expected 600", permissions)
	}
}
