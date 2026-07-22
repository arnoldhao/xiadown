package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	sqlite3 "github.com/ncruces/go-sqlite3"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

const (
	sqliteCompilationCacheAppDirectory = "xiadown"
	sqliteCompilationCacheDirectory    = "sqlite-wasm"
	sqliteCompilationCacheLockName     = "sqlite-wasm.lock"
	sqliteCompilationCacheManifestName = ".integrity-v1.json"
	sqliteCompilationCacheManifestV1   = 1
	sqliteCompilationCacheMaxArtifacts = 2
	sqliteCompilationCacheMaxBytes     = 32 << 20
)

type sqliteCompilationCacheManifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

type sqliteCompilationCacheFileState struct {
	size           int64
	modificationNS int64
}

type sqliteCompilationCacheFileLock struct {
	file       *os.File
	releaseMu  sync.Mutex
	released   bool
	releaseErr error
}

var (
	sqliteCompilationCache               wazero.CompilationCache
	sqliteCompilationCachePath           string
	sqliteCompilationCachePersistent     bool
	sqliteCompilationCacheManifestMu     sync.Mutex
	sqliteCompilationCacheManifestStored bool
	sqliteCompilationCacheManifestCheck  bool
	sqliteCompilationCacheInitialFiles   map[string]sqliteCompilationCacheFileState
	sqliteCompilationCacheLock           *sqliteCompilationCacheFileLock
)

func init() {
	if sqlite3.RuntimeConfig != nil {
		return
	}

	cache, cachePath, cacheLock, err := newPersistentSQLiteCompilationCache()
	if err != nil {
		// Cache setup must never prevent XiaDown from opening its database. Keep
		// one in-memory cache alive for the process when the user cache directory
		// is unavailable or fails the ownership/symlink checks below.
		cache = wazero.NewCompilationCache()
	} else {
		sqliteCompilationCachePath = cachePath
		sqliteCompilationCachePersistent = true
		sqliteCompilationCacheLock = cacheLock
	}
	sqliteCompilationCache = cache

	sqlite3.RuntimeConfig = wazero.NewRuntimeConfig().
		WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesThreads).
		WithCompilationCache(cache)
}

func newPersistentSQLiteCompilationCache() (wazero.CompilationCache, string, *sqliteCompilationCacheFileLock, error) {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, "", nil, fmt.Errorf("resolve user cache directory: %w", err)
	}
	if err := ensureSQLiteCacheBaseDirectory(userCacheDir); err != nil {
		return nil, "", nil, err
	}
	appDir := filepath.Join(userCacheDir, sqliteCompilationCacheAppDirectory)
	if err := ensurePrivateSQLiteCacheDirectory(appDir); err != nil {
		return nil, "", nil, err
	}
	cacheLock, acquired, err := tryAcquireSQLiteCompilationCacheFileLock(
		filepath.Join(appDir, sqliteCompilationCacheLockName),
	)
	if err != nil {
		return nil, "", nil, err
	}
	if !acquired {
		return nil, "", nil, fmt.Errorf("sqlite WASM compilation cache is locked by another process")
	}
	keepLock := false
	defer func() {
		if !keepLock {
			_ = cacheLock.Release()
		}
	}()

	cacheDir, err := prepareSQLiteCompilationCacheDirectory(userCacheDir)
	if err != nil {
		return nil, "", nil, err
	}
	cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
	if err != nil {
		return nil, "", nil, fmt.Errorf("open sqlite WASM compilation cache: %w", err)
	}
	// wazero creates its version/OS/architecture-specific child lazily. Secure
	// that directory too before the cache can be used.
	if err := secureSQLiteCompilationCacheTree(cacheDir); err != nil {
		_ = cache.Close(context.Background())
		return nil, "", nil, err
	}
	if _, err := os.Stat(filepath.Join(cacheDir, sqliteCompilationCacheManifestName)); err == nil {
		files, err := statSQLiteCompilationCacheFiles(cacheDir)
		if err != nil {
			_ = cache.Close(context.Background())
			return nil, "", nil, err
		}
		// The manifest was validated by prepareSQLiteCompilationCacheDirectory.
		// Keep a cheap metadata snapshot so a cache hit does not hash the 8 MB
		// artifact a second time merely to rediscover that nothing changed.
		sqliteCompilationCacheManifestStored = true
		sqliteCompilationCacheManifestCheck = true
		sqliteCompilationCacheInitialFiles = files
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = cache.Close(context.Background())
		return nil, "", nil, fmt.Errorf("inspect sqlite compilation cache manifest: %w", err)
	}
	keepLock = true
	return cache, cacheDir, cacheLock, nil
}

func tryAcquireSQLiteCompilationCacheFileLock(path string) (*sqliteCompilationCacheFileLock, bool, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("sqlite compilation cache lock is a symlink: %s", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("inspect sqlite compilation cache lock: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("open sqlite compilation cache lock: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf("secure sqlite compilation cache lock: %w", err)
	}
	locked, err := tryLockSQLiteCacheFile(file)
	if err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf("lock sqlite compilation cache: %w", err)
	}
	if !locked {
		_ = file.Close()
		return nil, false, nil
	}
	return &sqliteCompilationCacheFileLock{file: file}, true, nil
}

func (lock *sqliteCompilationCacheFileLock) Release() error {
	if lock == nil {
		return nil
	}
	lock.releaseMu.Lock()
	defer lock.releaseMu.Unlock()
	if lock.released {
		return lock.releaseErr
	}
	lock.released = true
	unlockErr := unlockSQLiteCacheFile(lock.file)
	closeErr := lock.file.Close()
	lock.releaseErr = errors.Join(unlockErr, closeErr)
	return lock.releaseErr
}

func releaseSQLiteCompilationCacheFileLock() error {
	return sqliteCompilationCacheLock.Release()
}

func prepareSQLiteCompilationCacheDirectory(userCacheDir string) (string, error) {
	if !filepath.IsAbs(userCacheDir) {
		return "", fmt.Errorf("user cache directory must be absolute: %q", userCacheDir)
	}
	if err := ensureSQLiteCacheBaseDirectory(userCacheDir); err != nil {
		return "", err
	}

	appDir := filepath.Join(userCacheDir, sqliteCompilationCacheAppDirectory)
	if err := ensurePrivateSQLiteCacheDirectory(appDir); err != nil {
		return "", err
	}
	cacheDir := filepath.Join(appDir, sqliteCompilationCacheDirectory)
	if err := ensurePrivateSQLiteCacheDirectory(cacheDir); err != nil {
		return "", err
	}
	if err := secureSQLiteCompilationCacheTree(cacheDir); err != nil {
		return "", err
	}
	if err := validateOrResetSQLiteCompilationCache(cacheDir); err != nil {
		return "", err
	}
	if err := pruneStaleSQLiteCompilationCacheVersions(
		cacheDir,
		sqliteCompilationCacheVersionDirectoryName(),
	); err != nil {
		return "", err
	}
	return cacheDir, nil
}

// sqliteCompilationCacheVersionDirectoryName mirrors wazero's documented
// version/OS/architecture cache partition. Keeping the calculation here lets
// XiaDown reclaim entries left by dependency upgrades without importing
// wazero's internal/version package.
func sqliteCompilationCacheVersionDirectoryName() string {
	version := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dependency := range info.Deps {
			if strings.Contains(dependency.Path, "github.com/tetratelabs/wazero") {
				version = dependency.Version
			}
		}
		if version == "" || version == "(devel)" {
			version = info.Main.Version
		}
	}
	if version == "" || version == "(devel)" {
		version = "dev"
	}
	return fmt.Sprintf("wazero-%s-%s-%s", version, runtime.GOARCH, runtime.GOOS)
}

// pruneStaleSQLiteCompilationCacheVersions bounds the regenerable cache to the
// version that the current process can actually consume. It intentionally
// leaves unrelated entries alone: only wazero version directories immediately
// below XiaDown's exact sqlite-wasm cache root are eligible for removal.
func pruneStaleSQLiteCompilationCacheVersions(cacheDir, activeVersionDirectory string) error {
	if filepath.Base(cacheDir) != sqliteCompilationCacheDirectory ||
		filepath.Base(filepath.Dir(cacheDir)) != sqliteCompilationCacheAppDirectory {
		return fmt.Errorf("refuse to prune unexpected sqlite compilation cache path: %s", cacheDir)
	}
	if filepath.Base(activeVersionDirectory) != activeVersionDirectory ||
		!strings.HasPrefix(activeVersionDirectory, "wazero-") {
		return fmt.Errorf("invalid active sqlite compilation cache version directory: %q", activeVersionDirectory)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("read sqlite compilation cache directory: %w", err)
	}
	removed := false
	for _, entry := range entries {
		if entry.Name() == activeVersionDirectory ||
			!strings.HasPrefix(entry.Name(), "wazero-") {
			continue
		}
		path := filepath.Join(cacheDir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect stale sqlite compilation cache entry %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse to prune sqlite compilation cache symlink: %s", path)
		}
		if !info.IsDir() {
			// A file merely named like a wazero version is not an artifact layout
			// XiaDown owns, so preserve it rather than broadening deletion scope.
			continue
		}
		if err := validateSQLiteCompilationCacheRemovalTarget(cacheDir, path); err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove stale sqlite compilation cache version %s: %w", path, err)
		}
		removed = true
	}
	activeRemoved, err := pruneSQLiteCompilationCacheArtifacts(
		cacheDir,
		filepath.Join(cacheDir, activeVersionDirectory),
	)
	if err != nil {
		return err
	}
	removed = removed || activeRemoved
	if !removed {
		return nil
	}
	// Pruning changes the verified file set. Commit its new digest while the
	// cross-process cache lock is still held so the next launch does not mistake
	// expected reclamation for corruption and rebuild the active artifact.
	return writeSQLiteCompilationCacheManifest(cacheDir)
}

type sqliteCompilationCacheArtifact struct {
	path         string
	name         string
	size         int64
	modification time.Time
}

// pruneSQLiteCompilationCacheArtifacts prevents SQLite WASM upgrades from
// accumulating indefinitely inside one wazero version partition. A missing
// current artifact is harmless: wazero recompiles it during Initialize, after
// which this policy runs again and keeps the newly written file.
func pruneSQLiteCompilationCacheArtifacts(cacheDir, versionDir string) (bool, error) {
	info, err := os.Lstat(versionDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("sqlite compilation cache version is not a directory: %s", versionDir)
	}
	if err := validateSQLiteCompilationCacheRemovalTarget(cacheDir, versionDir); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(versionDir)
	if err != nil {
		return false, err
	}
	artifacts := make([]sqliteCompilationCacheArtifact, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(versionDir, entry.Name())
		entryInfo, err := os.Lstat(path)
		if err != nil {
			return false, err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 || !entryInfo.Mode().IsRegular() {
			return false, fmt.Errorf("sqlite compilation cache artifact is not a regular file: %s", path)
		}
		artifacts = append(artifacts, sqliteCompilationCacheArtifact{
			path: path, name: entry.Name(), size: entryInfo.Size(), modification: entryInfo.ModTime(),
		})
	}
	sort.SliceStable(artifacts, func(i, j int) bool {
		if !artifacts[i].modification.Equal(artifacts[j].modification) {
			return artifacts[i].modification.After(artifacts[j].modification)
		}
		return artifacts[i].name < artifacts[j].name
	})
	var retainedBytes int64
	removed := false
	for index, artifact := range artifacts {
		keep := index < sqliteCompilationCacheMaxArtifacts &&
			(index == 0 || retainedBytes+artifact.size <= sqliteCompilationCacheMaxBytes)
		if keep {
			retainedBytes += artifact.size
			continue
		}
		relative, err := filepath.Rel(versionDir, artifact.path)
		if err != nil || filepath.Base(relative) != relative || relative == "." {
			return false, fmt.Errorf("refuse to remove unexpected sqlite compilation cache artifact: %s", artifact.path)
		}
		if err := os.Remove(artifact.path); err != nil {
			return false, fmt.Errorf("remove stale sqlite compilation cache artifact %s: %w", artifact.path, err)
		}
		removed = true
	}
	return removed, nil
}

func validateSQLiteCompilationCacheRemovalTarget(cacheDir, target string) error {
	relative, err := filepath.Rel(cacheDir, target)
	if err != nil || relative == "." || filepath.IsAbs(relative) ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		strings.Contains(relative, string(filepath.Separator)) ||
		!strings.HasPrefix(relative, "wazero-") {
		return fmt.Errorf("refuse to remove unexpected sqlite compilation cache target: %s", target)
	}
	return filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse to remove sqlite compilation cache tree containing symlink: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refuse to remove sqlite compilation cache tree containing special file: %s", path)
		}
		return nil
	})
}

func ensureSQLiteCacheBaseDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create user cache directory: %w", err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("inspect user cache directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("user cache directory is a symlink: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("user cache path is not a directory: %s", path)
	}
	return nil
}

func ensurePrivateSQLiteCacheDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return fmt.Errorf("create private sqlite cache directory %s: %w", path, err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("inspect sqlite cache directory %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("sqlite cache directory is a symlink: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("sqlite cache path is not a directory: %s", path)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure sqlite cache directory %s: %w", path, err)
	}
	return nil
}

// secureSQLiteCompilationCacheTree keeps executable cache artifacts private
// and rejects links or special files. wazero validates its version header and
// compiled-code checksum; XiaDown additionally records whole-file digests after
// a successful compile so corruption is removed before the next process uses it.
func secureSQLiteCompilationCacheTree(cacheDir string) error {
	return filepath.WalkDir(cacheDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("sqlite compilation cache contains a symlink: %s", path)
		}
		if entry.IsDir() {
			if err := os.Chmod(path, 0o700); err != nil {
				return fmt.Errorf("secure sqlite compilation cache directory %s: %w", path, err)
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("sqlite compilation cache contains a special file: %s", path)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("secure sqlite compilation cache file %s: %w", path, err)
		}
		return nil
	})
}

func validateOrResetSQLiteCompilationCache(cacheDir string) error {
	actual, err := digestSQLiteCompilationCacheFiles(cacheDir)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(cacheDir, sqliteCompilationCacheManifestName)
	contents, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		if len(actual) == 0 {
			return nil
		}
		return resetSQLiteCompilationCache(cacheDir)
	}
	if err != nil {
		return fmt.Errorf("read sqlite compilation cache manifest: %w", err)
	}

	var manifest sqliteCompilationCacheManifest
	if err := json.Unmarshal(contents, &manifest); err != nil ||
		manifest.Version != sqliteCompilationCacheManifestV1 ||
		!equalSQLiteCompilationCacheDigests(actual, manifest.Files) {
		return resetSQLiteCompilationCache(cacheDir)
	}
	return nil
}

func digestSQLiteCompilationCacheFiles(cacheDir string) (map[string]string, error) {
	digests := make(map[string]string)
	err := filepath.WalkDir(cacheDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("sqlite compilation cache contains a symlink: %s", path)
		}
		if filepath.Base(path) == sqliteCompilationCacheManifestName {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("sqlite compilation cache contains a special file: %s", path)
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		relative, err := filepath.Rel(cacheDir, path)
		if err != nil {
			return err
		}
		digests[filepath.ToSlash(relative)] = hex.EncodeToString(hash.Sum(nil))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("digest sqlite compilation cache: %w", err)
	}
	return digests, nil
}

func equalSQLiteCompilationCacheDigests(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, digest := range left {
		if right[path] != digest {
			return false
		}
	}
	return true
}

func statSQLiteCompilationCacheFiles(cacheDir string) (map[string]sqliteCompilationCacheFileState, error) {
	files := make(map[string]sqliteCompilationCacheFileState)
	err := filepath.WalkDir(cacheDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Base(path) == sqliteCompilationCacheManifestName {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("sqlite compilation cache contains a symlink: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("sqlite compilation cache contains a special file: %s", path)
		}
		relative, err := filepath.Rel(cacheDir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = sqliteCompilationCacheFileState{
			size:           info.Size(),
			modificationNS: info.ModTime().UnixNano(),
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect sqlite compilation cache files: %w", err)
	}
	return files, nil
}

func equalSQLiteCompilationCacheFileStates(
	left, right map[string]sqliteCompilationCacheFileState,
) bool {
	if len(left) != len(right) {
		return false
	}
	for path, state := range left {
		if right[path] != state {
			return false
		}
	}
	return true
}

func resetSQLiteCompilationCache(cacheDir string) error {
	// This guard keeps the removal target exact even if this helper is changed
	// later. Never clear a broad user cache directory on a malformed path.
	if filepath.Base(cacheDir) != sqliteCompilationCacheDirectory ||
		filepath.Base(filepath.Dir(cacheDir)) != sqliteCompilationCacheAppDirectory {
		return fmt.Errorf("refuse to reset unexpected sqlite compilation cache path: %s", cacheDir)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("read sqlite compilation cache directory: %w", err)
	}
	for _, entry := range entries {
		path := filepath.Join(cacheDir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse to reset sqlite compilation cache containing symlink: %s", path)
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("reset sqlite compilation cache entry %s: %w", path, err)
		}
	}
	return nil
}

func recordSQLiteCompilationCacheIntegrity() error {
	if !sqliteCompilationCachePersistent || sqliteCompilationCachePath == "" {
		return nil
	}
	sqliteCompilationCacheManifestMu.Lock()
	defer sqliteCompilationCacheManifestMu.Unlock()
	if err := pruneStaleSQLiteCompilationCacheVersions(
		sqliteCompilationCachePath,
		sqliteCompilationCacheVersionDirectoryName(),
	); err != nil {
		return err
	}
	if sqliteCompilationCacheManifestStored {
		if !sqliteCompilationCacheManifestCheck {
			return nil
		}
		currentFiles, err := statSQLiteCompilationCacheFiles(sqliteCompilationCachePath)
		if err == nil && equalSQLiteCompilationCacheFileStates(sqliteCompilationCacheInitialFiles, currentFiles) {
			sqliteCompilationCacheManifestCheck = false
			sqliteCompilationCacheInitialFiles = nil
			return nil
		}
	}
	if err := writeSQLiteCompilationCacheManifest(sqliteCompilationCachePath); err != nil {
		return err
	}
	sqliteCompilationCacheManifestStored = true
	sqliteCompilationCacheManifestCheck = false
	sqliteCompilationCacheInitialFiles = nil
	return nil
}

func writeSQLiteCompilationCacheManifest(cacheDir string) error {
	if err := secureSQLiteCompilationCacheTree(cacheDir); err != nil {
		return err
	}
	digests, err := digestSQLiteCompilationCacheFiles(cacheDir)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(cacheDir, sqliteCompilationCacheManifestName)
	if contents, readErr := os.ReadFile(manifestPath); readErr == nil {
		var current sqliteCompilationCacheManifest
		if json.Unmarshal(contents, &current) == nil &&
			current.Version == sqliteCompilationCacheManifestV1 &&
			equalSQLiteCompilationCacheDigests(digests, current.Files) {
			// Cache hits do not modify wazero's artifact, so avoid an otherwise
			// redundant manifest rename/fsync on every normal application launch.
			return nil
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read sqlite compilation cache manifest: %w", readErr)
	}
	manifest := sqliteCompilationCacheManifest{
		Version: sqliteCompilationCacheManifestV1,
		Files:   digests,
	}
	contents, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	temporary, err := os.CreateTemp(cacheDir, ".integrity-*.tmp")
	if err != nil {
		return fmt.Errorf("create sqlite compilation cache manifest: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		return err
	}
	// This is a regenerable compilation-cache manifest, not user data. An
	// fsync here added a visible one-time launch stall on APFS; a power loss can
	// at worst leave a missing or malformed manifest, which the next launch
	// already detects and repairs by rebuilding the cache.
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, manifestPath); err != nil {
		return fmt.Errorf("commit sqlite compilation cache manifest: %w", err)
	}
	committed = true
	return nil
}
