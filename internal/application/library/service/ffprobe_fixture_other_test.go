//go:build !windows

package service

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFFprobeTestFixture(t *testing.T, directory string, response string) string {
	t.Helper()

	path := filepath.Join(directory, ffprobeExecutableName())
	script := "#!/bin/sh\ncat <<'JSON'\n" + response + "\nJSON\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write ffprobe fixture: %v", err)
	}
	return path
}
