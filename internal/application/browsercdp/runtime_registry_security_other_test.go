//go:build !windows

package browsercdp

import (
	"os"
	"testing"
)

func assertPrivateRuntimeRegistryPath(t *testing.T, path string, _ bool, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat private runtime registry path: %v", err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("runtime registry permissions = %o, want %o", got, want)
	}
}
