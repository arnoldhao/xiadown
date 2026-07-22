//go:build windows

package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ffprobeFixtureResponseSuffix = ".xiadown-test-response.json"

func TestMain(m *testing.M) {
	executable, err := os.Executable()
	if err == nil && strings.EqualFold(filepath.Base(executable), ffprobeExecutableName()) {
		response, readErr := os.ReadFile(executable + ffprobeFixtureResponseSuffix)
		if readErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "read ffprobe fixture response: %v\n", readErr)
			os.Exit(2)
		}
		if _, writeErr := os.Stdout.Write(response); writeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "write ffprobe fixture response: %v\n", writeErr)
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func writeFFprobeTestFixture(t *testing.T, directory string, response string) string {
	t.Helper()

	sourcePath, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("open test executable: %v", err)
	}
	defer source.Close()

	targetPath := filepath.Join(directory, ffprobeExecutableName())
	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		t.Fatalf("create ffprobe fixture: %v", err)
	}
	if _, err = io.Copy(target, source); err != nil {
		_ = target.Close()
		t.Fatalf("copy ffprobe fixture: %v", err)
	}
	if err = target.Close(); err != nil {
		t.Fatalf("close ffprobe fixture: %v", err)
	}
	if err = os.WriteFile(targetPath+ffprobeFixtureResponseSuffix, []byte(response), 0o600); err != nil {
		t.Fatalf("write ffprobe fixture response: %v", err)
	}
	return targetPath
}
