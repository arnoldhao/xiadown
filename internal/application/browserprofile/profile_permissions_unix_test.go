//go:build !windows

package browserprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListAtRootDistinguishesPermissionDenialFromNoProfiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "protected")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"info_cache":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
	_, state := listAtRootDetailed("chrome", root)
	if os.Geteuid() != 0 && state != ProfileStatePermissionRequired {
		t.Fatalf("permission denial must not become an empty profile state: %q", state)
	}
}
