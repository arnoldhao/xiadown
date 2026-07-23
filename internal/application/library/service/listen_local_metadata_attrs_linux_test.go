//go:build linux

package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPrepareListenLocalMetadataReplacementPreservesLinuxXattrs(t *testing.T) {
	directory := t.TempDir()
	originalPath := filepath.Join(directory, "original.mp3")
	replacementPath := filepath.Join(directory, "replacement.mp3")
	if err := os.WriteFile(originalPath, []byte("original"), 0o640); err != nil {
		t.Fatalf("write original: %v", err)
	}
	if err := os.WriteFile(replacementPath, []byte("replacement"), 0o600); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	name := "user.xiadown.metadata-test"
	want := []byte("preserve-me")
	if err := unix.Setxattr(originalPath, name, want, 0); err != nil {
		t.Skipf("filesystem does not support test xattrs: %v", err)
	}
	original, err := os.Stat(originalPath)
	if err != nil {
		t.Fatalf("stat original: %v", err)
	}
	if err := prepareListenLocalMetadataReplacement(originalPath, replacementPath, original); err != nil {
		t.Fatalf("prepare replacement: %v", err)
	}
	size, err := unix.Getxattr(replacementPath, name, nil)
	if err != nil {
		t.Fatalf("get replacement xattr size: %v", err)
	}
	got := make([]byte, size)
	if _, err := unix.Getxattr(replacementPath, name, got); err != nil {
		t.Fatalf("get replacement xattr: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("xattr changed: got %q want %q", got, want)
	}
	if mode := mustListenLocalFileMode(t, replacementPath).Perm(); mode != 0o640 {
		t.Fatalf("mode changed to %v", mode)
	}
}
