//go:build !windows

package libraryapi

import (
	"os"
	"testing"
)

func assertPrivatePublicMusicResourceDirectory(t *testing.T, _ string, info os.FileInfo) {
	t.Helper()
	if info == nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("cache directory mode=%v, want 0700", info.Mode())
	}
}

func assertProtectedPublicMusicResourceBlob(t *testing.T, _ string, info os.FileInfo) {
	t.Helper()
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("CAS blob is writable: mode=%v", info.Mode())
	}
}
