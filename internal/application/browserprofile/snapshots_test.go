package browserprofile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func useSnapshotTestRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "controlled-snapshots")
	previousRoot := snapshotRoot
	previousNow := snapshotNow
	previousOwnership := snapshotOwnershipCheck
	snapshotRoot = func() string { return root }
	snapshotNow = time.Now
	snapshotOwnershipCheck = snapshotOwnedByCurrentUser
	t.Cleanup(func() {
		snapshotRoot = previousRoot
		snapshotNow = previousNow
		snapshotOwnershipCheck = previousOwnership
	})
	return root
}

func createSnapshotCleanupCandidate(t *testing.T, root string, age time.Duration) string {
	t.Helper()
	name, err := newSnapshotDirectoryName()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, name)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := secureSnapshotDirectory(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "sensitive-cookie-data"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCleanupStaleSnapshotsEnforcesBoundaries(t *testing.T) {
	root := useSnapshotTestRoot(t)
	if _, err := validateSnapshotRoot(true); err != nil {
		t.Fatal(err)
	}
	stale := createSnapshotCleanupCandidate(t, root, snapshotStaleAfter+time.Hour)
	fresh := createSnapshotCleanupCandidate(t, root, time.Minute)
	foreign := createSnapshotCleanupCandidate(t, root, snapshotStaleAfter+time.Hour)

	unexpected := filepath.Join(root, "not-a-browser-snapshot")
	if err := os.Mkdir(unexpected, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-snapshotStaleAfter - time.Hour)
	if err := os.Chtimes(unexpected, old, old); err != nil {
		t.Fatal(err)
	}

	regularName, err := newSnapshotDirectoryName()
	if err != nil {
		t.Fatal(err)
	}
	regular := filepath.Join(root, regularName)
	if err := os.WriteFile(regular, []byte("do-not-delete"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(regular, old, old); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "outside-target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "must-survive"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkName, err := newSnapshotDirectoryName()
	if err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(root, symlinkName)
	if err := os.Symlink(target, symlink); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	realOwnershipCheck := snapshotOwnershipCheck
	snapshotOwnershipCheck = func(path string, info os.FileInfo) (bool, error) {
		if path == foreign {
			return false, nil
		}
		return realOwnershipCheck(path, info)
	}
	if err := CleanupStaleSnapshots(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired owned snapshot was not removed: %v", err)
	}
	for _, path := range []string{fresh, foreign, unexpected, regular, symlink} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("unsafe cleanup removed protected entry %s: %v", filepath.Base(path), err)
		}
	}
	if payload, err := os.ReadFile(filepath.Join(target, "must-survive")); err != nil || string(payload) != "outside" {
		t.Fatalf("cleanup followed a symlink outside its root: %q %v", payload, err)
	}
}

func TestCleanupStaleSnapshotsSkipsActiveSnapshot(t *testing.T) {
	useSnapshotTestRoot(t)
	path, cleanup, err := createSnapshotDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	old := time.Now().Add(-snapshotStaleAfter - time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := CleanupStaleSnapshots(context.Background()); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(path); err != nil || !info.IsDir() {
		t.Fatalf("active snapshot was removed: %v", err)
	}
	cleanup()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("normal cleanup did not remove snapshot: %v", err)
	}
}

func TestSnapshotDirectoryIsPrivateAndRootSymlinkIsRejected(t *testing.T) {
	root := useSnapshotTestRoot(t)
	path, cleanup, err := createSnapshotDirectory()
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if runtime.GOOS != "windows" {
		info, err := os.Stat(root)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("snapshot root permissions = %o, want 0700", info.Mode().Perm())
		}
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot cleanup left data behind: %v", err)
	}

	target := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "snapshot-root-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	snapshotRoot = func() string { return link }
	if err := CleanupStaleSnapshots(context.Background()); err == nil {
		t.Fatal("expected a symlink snapshot root to be rejected")
	}
}
