//go:build windows

package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const dependencyVersionFixtureSuffix = ".xiadown-test-version"

func TestMain(m *testing.M) {
	executable, err := os.Executable()
	if err == nil {
		response, readErr := os.ReadFile(executable + dependencyVersionFixtureSuffix)
		if readErr == nil {
			if _, writeErr := os.Stdout.Write(response); writeErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "write dependency version fixture: %v\n", writeErr)
				os.Exit(2)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func writeDependencyVersionTestExecutable(t *testing.T, directory string, name string, version string) string {
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

	targetPath := filepath.Join(directory, executableNameForBinary(name))
	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		t.Fatalf("create %s version fixture: %v", name, err)
	}
	if _, err = io.Copy(target, source); err != nil {
		_ = target.Close()
		t.Fatalf("copy %s version fixture: %v", name, err)
	}
	if err = target.Close(); err != nil {
		t.Fatalf("close %s version fixture: %v", name, err)
	}
	response := name + " version " + version + "\n"
	if err = os.WriteFile(targetPath+dependencyVersionFixtureSuffix, []byte(response), 0o600); err != nil {
		t.Fatalf("write %s version response: %v", name, err)
	}
	return targetPath
}
