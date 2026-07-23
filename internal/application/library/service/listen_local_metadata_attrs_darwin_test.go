//go:build darwin && cgo

package service

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPrepareListenLocalMetadataReplacementPreservesDarwinXattrsAndACL(t *testing.T) {
	directory := t.TempDir()
	originalPath := filepath.Join(directory, "original.mp3")
	replacementPath := filepath.Join(directory, "replacement.mp3")
	if err := os.WriteFile(originalPath, []byte("original"), 0o640); err != nil {
		t.Fatalf("write original: %v", err)
	}
	if err := os.WriteFile(replacementPath, []byte("replacement"), 0o600); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	xattrName := "com.xiadown.metadata-test"
	xattrValue := []byte("preserve-me")
	if err := unix.Setxattr(originalPath, xattrName, xattrValue, 0); err != nil {
		t.Skipf("filesystem does not support test xattrs: %v", err)
	}

	t.Cleanup(func() {
		_ = exec.Command("/bin/chmod", "-N", originalPath).Run()
		_ = exec.Command("/bin/chmod", "-N", replacementPath).Run()
	})
	aclAdded := exec.Command("/bin/chmod", "+a", "everyone deny delete", originalPath).Run() == nil
	original, err := os.Stat(originalPath)
	if err != nil {
		t.Fatalf("stat original: %v", err)
	}
	if err := prepareListenLocalMetadataReplacement(originalPath, replacementPath, original); err != nil {
		t.Fatalf("prepare replacement: %v", err)
	}

	valueSize, err := unix.Getxattr(replacementPath, xattrName, nil)
	if err != nil {
		t.Fatalf("get replacement xattr size: %v", err)
	}
	value := make([]byte, valueSize)
	if _, err := unix.Getxattr(replacementPath, xattrName, value); err != nil {
		t.Fatalf("get replacement xattr: %v", err)
	}
	if !bytes.Equal(value, xattrValue) {
		t.Fatalf("xattr changed: got %q want %q", value, xattrValue)
	}
	if mode := mustListenLocalFileMode(t, replacementPath).Perm(); mode != 0o640 {
		t.Fatalf("mode changed to %v", mode)
	}
	replacement, err := os.Stat(replacementPath)
	if err != nil {
		t.Fatalf("stat replacement: %v", err)
	}
	originalStat, originalOK := original.Sys().(*syscall.Stat_t)
	replacementStat, replacementOK := replacement.Sys().(*syscall.Stat_t)
	if !originalOK || !replacementOK || originalStat.Birthtimespec != replacementStat.Birthtimespec {
		t.Fatalf("birth time was not preserved: original=%#v replacement=%#v", originalStat, replacementStat)
	}
	if aclAdded {
		output, err := exec.Command("/bin/ls", "-le", replacementPath).CombinedOutput()
		if err != nil {
			t.Fatalf("inspect replacement ACL: %v: %s", err, output)
		}
		if !bytes.Contains(output, []byte("everyone deny delete")) {
			t.Fatalf("replacement ACL was not preserved: %s", output)
		}
	}
}
