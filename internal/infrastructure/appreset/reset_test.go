package appreset

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func resetTestManager(t *testing.T, deleteKey func(context.Context) error) (*Manager, Paths) {
	t.Helper()
	root := t.TempDir()
	configBase := filepath.Join(root, "config-base")
	cacheBase := filepath.Join(root, "cache-base")
	logRoot := filepath.Join(root, "log-base", "xiadown-logs")
	for _, directory := range []string{configBase, cacheBase, filepath.Dir(logRoot)} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	paths := PathsForRoots(configBase, cacheBase, logRoot)
	manager, err := New(paths, deleteKey)
	if err != nil {
		t.Fatal(err)
	}
	return manager, paths
}

func writeResetFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPendingResetDeletesAllOwnedRootsAndMasterKey(t *testing.T) {
	ctx := context.Background()
	keyDeletes := 0
	var paths Paths
	manager, paths := resetTestManager(t, func(context.Context) error {
		keyDeletes++
		if _, err := os.Stat(filepath.Join(paths.ConfigRoot, "data.db")); err != nil {
			t.Fatalf("database disappeared before Session Vault key deletion: %v", err)
		}
		return nil
	})
	writeResetFixture(t, filepath.Join(paths.ConfigRoot, "data.db"), "old-ciphertext")
	writeResetFixture(t, filepath.Join(paths.CacheRoot, "rss", "cache"), "cache")
	writeResetFixture(t, filepath.Join(paths.LogRoot, "app.log"), "log")
	unrelated := filepath.Join(paths.ConfigBase, "another-app", "keep")
	writeResetFixture(t, unrelated, "keep")

	if err := manager.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.ConfigRoot, "data.db")); err != nil {
		t.Fatalf("Schedule must not mutate live data: %v", err)
	}
	result, err := manager.ApplyPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || keyDeletes != 1 {
		t.Fatalf("result=%#v keyDeletes=%d", result, keyDeletes)
	}
	for _, root := range []string{paths.ConfigRoot, paths.CacheRoot, paths.LogRoot} {
		if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("owned root still exists %s: %v", root, err)
		}
	}
	if payload, err := os.ReadFile(unrelated); err != nil || string(payload) != "keep" {
		t.Fatalf("unrelated user data changed: %q %v", payload, err)
	}
	if _, err := os.Lstat(paths.MarkerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed reset marker still exists: %v", err)
	}
	result, err = manager.ApplyPending(ctx)
	if err != nil || result.Applied || keyDeletes != 1 {
		t.Fatalf("second apply result=%#v keyDeletes=%d err=%v", result, keyDeletes, err)
	}
}

func TestPendingResetRetriesAfterKeyDeletionFailureWithoutExposingOldCiphertext(t *testing.T) {
	ctx := context.Background()
	calls := 0
	manager, paths := resetTestManager(t, func(context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("secure storage locked")
		}
		return nil
	})
	oldDatabase := filepath.Join(paths.ConfigRoot, "data.db")
	writeResetFixture(t, oldDatabase, "old-encrypted-session")
	if err := manager.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ApplyPending(ctx); err == nil {
		t.Fatal("expected first reset attempt to fail closed")
	}
	if payload, err := os.ReadFile(oldDatabase); err != nil || string(payload) != "old-encrypted-session" {
		t.Fatalf("failed attempt partially changed database: %q %v", payload, err)
	}
	if _, err := os.Stat(paths.MarkerPath); err != nil {
		t.Fatalf("retry marker was removed: %v", err)
	}
	if result, err := manager.ApplyPending(ctx); err != nil || !result.Applied {
		t.Fatalf("retry result=%#v err=%v", result, err)
	}
	if _, err := os.Stat(oldDatabase); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old ciphertext survived completed reset: %v", err)
	}
	writeResetFixture(t, oldDatabase, "fresh-install-database")
	payload, err := os.ReadFile(oldDatabase)
	if err != nil || string(payload) != "fresh-install-database" {
		t.Fatalf("old ciphertext was revived after reset: %q %v", payload, err)
	}
}

func TestPendingResetResumesAfterRootWasAlreadyMovedToTrash(t *testing.T) {
	ctx := context.Background()
	manager, paths := resetTestManager(t, func(context.Context) error { return nil })
	writeResetFixture(t, filepath.Join(paths.ConfigRoot, "data.db"), "old")
	if err := manager.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	marker, err := readMarker(paths.MarkerPath)
	if err != nil {
		t.Fatal(err)
	}
	trashPath := filepath.Join(paths.ConfigBase, ".xiadown-reset-trash-"+marker.ResetID+"-config")
	if err := os.Rename(paths.ConfigRoot, trashPath); err != nil {
		t.Fatal(err)
	}
	if result, err := manager.ApplyPending(ctx); err != nil || !result.Applied {
		t.Fatalf("resume result=%#v err=%v", result, err)
	}
	if _, err := os.Lstat(trashPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphaned reset trash survived: %v", err)
	}
}

func TestPendingResetRejectsSymlinkAndCanRetry(t *testing.T) {
	ctx := context.Background()
	keyDeletes := 0
	manager, paths := resetTestManager(t, func(context.Context) error {
		keyDeletes++
		return nil
	})
	externalRoot := t.TempDir()
	externalFile := filepath.Join(externalRoot, "keep")
	writeResetFixture(t, externalFile, "keep")
	if err := os.Symlink(externalRoot, paths.ConfigRoot); err != nil {
		t.Fatal(err)
	}
	if err := manager.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ApplyPending(ctx); err == nil {
		t.Fatal("reset followed a symbolic link")
	}
	if payload, err := os.ReadFile(externalFile); err != nil || string(payload) != "keep" {
		t.Fatalf("symlink target changed: %q %v", payload, err)
	}
	if _, err := os.Stat(paths.MarkerPath); err != nil {
		t.Fatalf("marker must remain after a failed reset: %v", err)
	}
	if err := os.Remove(paths.ConfigRoot); err != nil {
		t.Fatal(err)
	}
	writeResetFixture(t, filepath.Join(paths.ConfigRoot, "data.db"), "old")
	if result, err := manager.ApplyPending(ctx); err != nil || !result.Applied {
		t.Fatalf("retry result=%#v err=%v", result, err)
	}
	if keyDeletes != 1 {
		t.Fatalf("preflight should delete the key only after paths are safe: calls=%d", keyDeletes)
	}
}

func TestResetRejectsEscapingPathsAndSymlinkMarker(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	paths := PathsForRoots(base, base, filepath.Join(base, "logs"))
	paths.ConfigRoot = filepath.Join(root, "outside")
	if _, err := New(paths, func(context.Context) error { return nil }); err == nil {
		t.Fatal("accepted reset path outside its ownership base")
	}

	manager, paths := resetTestManager(t, func(context.Context) error { return nil })
	target := filepath.Join(t.TempDir(), "marker")
	writeResetFixture(t, target, "not-a-marker")
	if err := os.Symlink(target, paths.MarkerPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.Schedule(context.Background()); err == nil {
		t.Fatal("Schedule replaced a symlinked marker")
	}
}
