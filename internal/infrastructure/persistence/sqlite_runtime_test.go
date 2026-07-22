package persistence

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const sqliteCacheLockHelperEnvironment = "XIADOWN_SQLITE_CACHE_LOCK_TEST_HELPER"

func TestSQLiteCompilationCacheLockIsCrossProcess(t *testing.T) {
	lockPath := os.Getenv(sqliteCacheLockHelperEnvironment)
	if lockPath != "" {
		lock, acquired, err := tryAcquireSQLiteCompilationCacheFileLock(lockPath)
		if err != nil || !acquired {
			t.Fatalf("helper acquire: acquired=%t err=%v", acquired, err)
		}
		defer lock.Release()
		if err := os.WriteFile(lockPath+".ready", []byte("ready"), 0o600); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(lockPath + ".release"); err == nil {
				return
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for parent release signal")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	lockPath = filepath.Join(t.TempDir(), "cross-process.lock")
	command := exec.Command(os.Args[0], "-test.run=^TestSQLiteCompilationCacheLockIsCrossProcess$")
	command.Env = append(os.Environ(), sqliteCacheLockHelperEnvironment+"="+lockPath)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	childFinished := false
	defer func() {
		if !childFinished {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(lockPath + ".ready"); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for child lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	contender, acquired, err := tryAcquireSQLiteCompilationCacheFileLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if acquired {
		_ = contender.Release()
		t.Fatal("second process acquired cache lock while helper held it")
	}
	if err := os.WriteFile(lockPath+".release", []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("helper failed: %v", err)
	}
	childFinished = true

	afterRelease, acquired, err := tryAcquireSQLiteCompilationCacheFileLock(lockPath)
	if err != nil || !acquired {
		t.Fatalf("acquire after helper exit: acquired=%t err=%v", acquired, err)
	}
	if err := afterRelease.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareSQLiteCompilationCacheDirectoryIsPrivate(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(baseDir, sqliteCompilationCacheAppDirectory, sqliteCompilationCacheDirectory)
	if cacheDir != want {
		t.Fatalf("cache directory = %q, want %q", cacheDir, want)
	}
	if runtime.GOOS == "windows" {
		return
	}
	for _, path := range []string{filepath.Dir(cacheDir), cacheDir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if permissions := info.Mode().Perm(); permissions != 0o700 {
			t.Fatalf("permissions for %s = %o, want 700", path, permissions)
		}
	}
}

func TestPrepareSQLiteCompilationCacheDirectoryRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	baseDir := t.TempDir()
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(baseDir, sqliteCompilationCacheAppDirectory)); err != nil {
		t.Fatal(err)
	}

	_, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("prepare cache error = %v, want symlink rejection", err)
	}
	if contents, err := os.ReadFile(sentinel); err != nil || string(contents) != "keep" {
		t.Fatalf("outside sentinel changed: contents=%q err=%v", contents, err)
	}
}

func TestPrepareSQLiteCompilationCacheDirectoryPrunesStaleWazeroVersions(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	activeVersion := sqliteCompilationCacheVersionDirectoryName()
	activeArtifact := filepath.Join(cacheDir, activeVersion, "current-artifact")
	staleArtifact := filepath.Join(cacheDir, "wazero-v0.0.1-test-os", "stale-artifact")
	unrelatedArtifact := filepath.Join(cacheDir, "keep-unrelated", "sentinel")
	for path, contents := range map[string]string{
		activeArtifact:    "current",
		staleArtifact:     "stale",
		unrelatedArtifact: "unrelated",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeSQLiteCompilationCacheManifest(cacheDir); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareSQLiteCompilationCacheDirectory(baseDir); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(activeArtifact); err != nil || string(contents) != "current" {
		t.Fatalf("active artifact changed: contents=%q err=%v", contents, err)
	}
	if _, err := os.Stat(staleArtifact); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale artifact still exists: %v", err)
	}
	if contents, err := os.ReadFile(unrelatedArtifact); err != nil || string(contents) != "unrelated" {
		t.Fatalf("unrelated cache entry changed: contents=%q err=%v", contents, err)
	}
	if err := validateOrResetSQLiteCompilationCache(cacheDir); err != nil {
		t.Fatalf("pruned cache manifest is stale: %v", err)
	}
}

func TestPrepareSQLiteCompilationCacheDirectoryBoundsActiveVersionArtifacts(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	versionDir := filepath.Join(cacheDir, sqliteCompilationCacheVersionDirectoryName())
	if err := os.MkdirAll(versionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	baseTime := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)
	for index := 0; index < 4; index++ {
		path := filepath.Join(versionDir, fmt.Sprintf("artifact-%d", index))
		if err := os.WriteFile(path, []byte(strings.Repeat("x", index+1)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, baseTime.Add(time.Duration(index)*time.Minute), baseTime.Add(time.Duration(index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeSQLiteCompilationCacheManifest(cacheDir); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareSQLiteCompilationCacheDirectory(baseDir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(versionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != sqliteCompilationCacheMaxArtifacts {
		t.Fatalf("active cache artifacts = %d, want %d: %v", len(entries), sqliteCompilationCacheMaxArtifacts, entries)
	}
	want := map[string]bool{"artifact-2": true, "artifact-3": true}
	for _, entry := range entries {
		if !want[entry.Name()] {
			t.Fatalf("unexpected retained artifact %q", entry.Name())
		}
	}
	if err := validateOrResetSQLiteCompilationCache(cacheDir); err != nil {
		t.Fatalf("bounded active cache manifest is stale: %v", err)
	}
}

func TestPruneStaleSQLiteCompilationCacheVersionsRejectsSymlinkTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	baseDir := t.TempDir()
	cacheDir, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleDir := filepath.Join(cacheDir, "wazero-v0.0.1-test-os")
	if err := os.Mkdir(staleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(staleDir, "outside")); err != nil {
		t.Fatal(err)
	}

	err = pruneStaleSQLiteCompilationCacheVersions(cacheDir, sqliteCompilationCacheVersionDirectoryName())
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("prune error = %v, want symlink rejection", err)
	}
	if contents, err := os.ReadFile(sentinel); err != nil || string(contents) != "keep" {
		t.Fatalf("outside sentinel changed: contents=%q err=%v", contents, err)
	}
}

func TestPruneStaleSQLiteCompilationCacheVersionsRejectsBroadTarget(t *testing.T) {
	cacheRoot := t.TempDir()
	err := pruneStaleSQLiteCompilationCacheVersions(cacheRoot, "wazero-current-test-os")
	if err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("prune broad target error = %v", err)
	}
}

func TestSQLiteCompilationCacheManifestClearsCorruptArtifact(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	versionDir := filepath.Join(cacheDir, sqliteCompilationCacheVersionDirectoryName())
	if err := os.Mkdir(versionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(versionDir, "compiled-module")
	if err := os.WriteFile(artifact, []byte("known-good-cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeSQLiteCompilationCacheManifest(cacheDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("corrupted-cache"), 0o600); err != nil {
		t.Fatal(err)
	}

	reopenedDir, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if reopenedDir != cacheDir {
		t.Fatalf("reopened cache directory = %q, want %q", reopenedDir, cacheDir)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("corrupt cache was not reset: %v", entries)
	}
}

func TestSQLiteCompilationCacheManifestPreservesVerifiedArtifact(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	versionDir := filepath.Join(cacheDir, sqliteCompilationCacheVersionDirectoryName())
	if err := os.Mkdir(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(versionDir, "compiled-module")
	if err := os.WriteFile(artifact, []byte("verified-cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeSQLiteCompilationCacheManifest(cacheDir); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareSQLiteCompilationCacheDirectory(baseDir); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("verified artifact was removed: %v", err)
	}
	if string(contents) != "verified-cache" {
		t.Fatalf("verified artifact = %q", contents)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(artifact); err != nil {
			t.Fatal(err)
		} else if permissions := info.Mode().Perm(); permissions != 0o600 {
			t.Fatalf("artifact permissions = %o, want 600", permissions)
		}
	}
}

func TestSQLiteCompilationCacheManifestIsNotRewrittenWhenUnchanged(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir, err := prepareSQLiteCompilationCacheDirectory(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	versionDir := filepath.Join(cacheDir, sqliteCompilationCacheVersionDirectoryName())
	if err := os.Mkdir(versionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "compiled-module"), []byte("verified-cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeSQLiteCompilationCacheManifest(cacheDir); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(cacheDir, sqliteCompilationCacheManifestName)
	before, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	// A rewrite would necessarily have a later timestamp on the filesystems used
	// in CI; force a visible gap without putting delay on the production path.
	time.Sleep(20 * time.Millisecond)
	if err := writeSQLiteCompilationCacheManifest(cacheDir); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("unchanged manifest was rewritten: before=%s after=%s", before.ModTime(), after.ModTime())
	}
}
