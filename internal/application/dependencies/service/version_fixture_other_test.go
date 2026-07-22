//go:build !windows

package service

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDependencyVersionTestExecutable(t *testing.T, directory string, name string, version string) string {
	t.Helper()
	path := filepath.Join(directory, executableNameForBinary(name))
	script := "#!/bin/sh\nprintf '" + name + " version " + version + "\\n'"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write %s version fixture: %v", name, err)
	}
	return path
}
