package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoggerRejectsUnwritableLogFilePath(t *testing.T) {
	directory := t.TempDir()
	// A directory at the target filename makes the exact sink unusable while
	// leaving the parent directory itself writable. This exercises the eager
	// file probe rather than only MkdirAll failures.
	if err := os.Mkdir(filepath.Join(directory, "app.log"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLogger(Config{Directory: directory}); err == nil {
		t.Fatal("NewLogger accepted an unusable log file path")
	}
}
