package appreset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

const (
	markerFormatVersion = 1
	markerFileName      = ".xiadown-reset-pending-v1"
	markerSizeLimit     = 4096
)

// Paths describes the XiaDown-owned roots that make up an installation's
// persistent state. Every managed root must be one direct child of its base;
// reset never accepts a caller-supplied path outside these boundaries.
type Paths struct {
	ConfigBase string
	ConfigRoot string
	CacheBase  string
	CacheRoot  string
	LogBase    string
	LogRoot    string
	MarkerPath string
}

type Result struct {
	Applied bool
}

type keyDeleter func(context.Context) error

// Manager implements a two-phase application reset. Schedule writes a durable
// marker while the application is still running. ApplyPending consumes it on
// the next launch, before XiaDown opens its database or log files.
type Manager struct {
	paths     Paths
	deleteKey keyDeleter
	now       func() time.Time
	newID     func() string
}

type resetMarker struct {
	FormatVersion int    `json:"formatVersion"`
	ResetID       string `json:"resetId"`
	RequestedAt   string `json:"requestedAt"`
}

type managedRoot struct {
	kind string
	base string
	path string
}

func PathsForRoots(configBase, cacheBase, logRoot string) Paths {
	configBase = filepath.Clean(strings.TrimSpace(configBase))
	cacheBase = filepath.Clean(strings.TrimSpace(cacheBase))
	logRoot = filepath.Clean(strings.TrimSpace(logRoot))
	return Paths{
		ConfigBase: configBase,
		ConfigRoot: filepath.Join(configBase, "xiadown"),
		CacheBase:  cacheBase,
		CacheRoot:  filepath.Join(cacheBase, "xiadown"),
		LogBase:    filepath.Dir(logRoot),
		LogRoot:    logRoot,
		MarkerPath: filepath.Join(configBase, markerFileName),
	}
}

func New(paths Paths, deleteKey func(context.Context) error) (*Manager, error) {
	paths = cleanPaths(paths)
	if err := validatePaths(paths); err != nil {
		return nil, err
	}
	if deleteKey == nil {
		return nil, fmt.Errorf("application reset key deleter is required")
	}
	return &Manager{
		paths:     paths,
		deleteKey: deleteKey,
		now:       time.Now,
		newID:     uuid.NewString,
	}, nil
}

func (manager *Manager) Schedule(ctx context.Context) error {
	if manager == nil {
		return fmt.Errorf("application reset unavailable")
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := ensureBaseDirectory(manager.paths.ConfigBase); err != nil {
		return fmt.Errorf("validate application reset marker directory: %w", err)
	}

	// A valid existing marker makes scheduling idempotent. Never overwrite a
	// malformed or symlinked marker, because doing so could hide tampering.
	if _, err := readMarker(manager.paths.MarkerPath); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	marker := resetMarker{
		FormatVersion: markerFormatVersion,
		ResetID:       manager.newID(),
		RequestedAt:   manager.now().UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("encode application reset marker: %w", err)
	}
	if len(payload) > markerSizeLimit {
		return fmt.Errorf("application reset marker is too large")
	}

	temporaryPath := manager.paths.MarkerPath + ".tmp-" + manager.newID()
	file, err := os.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create application reset marker: %w", err)
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := file.Write(payload); err != nil {
		return fmt.Errorf("write application reset marker: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync application reset marker: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close application reset marker: %w", err)
	}
	if err := os.Rename(temporaryPath, manager.paths.MarkerPath); err != nil {
		return fmt.Errorf("publish application reset marker: %w", err)
	}
	committed = true
	if err := syncDirectory(manager.paths.ConfigBase); err != nil {
		return fmt.Errorf("sync application reset marker directory: %w", err)
	}
	return nil
}

func (manager *Manager) ApplyPending(ctx context.Context) (Result, error) {
	if manager == nil {
		return Result{}, fmt.Errorf("application reset unavailable")
	}
	// Validate the marker's ownership base before reading through it. In
	// particular, a symlinked config base must never be able to supply a marker
	// that causes secure-storage deletion.
	if err := ensureBaseDirectory(manager.paths.ConfigBase); err != nil {
		return Result{}, fmt.Errorf("validate application reset marker directory: %w", err)
	}
	marker, err := readMarker(manager.paths.MarkerPath)
	if errors.Is(err, fs.ErrNotExist) {
		return Result{}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if err := contextErr(ctx); err != nil {
		return Result{}, err
	}

	roots := collapseManagedRoots(manager.managedRoots())
	for _, root := range roots {
		if err := preflightManagedRoot(root, marker.ResetID); err != nil {
			return Result{}, fmt.Errorf("validate %s reset data: %w", root.kind, err)
		}
	}

	// Delete the key before filesystem state. If the process crashes after this
	// point, the marker remains and blocks the old encrypted database from being
	// opened; the next launch safely resumes the reset.
	if err := manager.deleteKey(ctx); err != nil {
		return Result{}, fmt.Errorf("delete Session Vault master key: %w", err)
	}
	for _, root := range roots {
		if err := resetManagedRoot(ctx, root, marker.ResetID); err != nil {
			return Result{}, fmt.Errorf("reset %s data: %w", root.kind, err)
		}
	}
	if err := removeRegularMarker(manager.paths.MarkerPath, manager.paths.ConfigBase); err != nil {
		return Result{}, err
	}
	if err := syncDirectory(manager.paths.ConfigBase); err != nil {
		return Result{}, fmt.Errorf("sync application reset completion: %w", err)
	}
	return Result{Applied: true}, nil
}

func preflightManagedRoot(root managedRoot, resetID string) error {
	if err := ensureBaseDirectory(root.base); err != nil {
		return err
	}
	for _, path := range []string{
		root.path,
		filepath.Join(root.base, ".xiadown-reset-trash-"+resetID+"-"+root.kind),
	} {
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing application reset symlink")
		}
	}
	return nil
}

func (manager *Manager) managedRoots() []managedRoot {
	return []managedRoot{
		{kind: "config", base: manager.paths.ConfigBase, path: manager.paths.ConfigRoot},
		{kind: "cache", base: manager.paths.CacheBase, path: manager.paths.CacheRoot},
		{kind: "logs", base: manager.paths.LogBase, path: manager.paths.LogRoot},
	}
}

func cleanPaths(paths Paths) Paths {
	paths.ConfigBase = filepath.Clean(strings.TrimSpace(paths.ConfigBase))
	paths.ConfigRoot = filepath.Clean(strings.TrimSpace(paths.ConfigRoot))
	paths.CacheBase = filepath.Clean(strings.TrimSpace(paths.CacheBase))
	paths.CacheRoot = filepath.Clean(strings.TrimSpace(paths.CacheRoot))
	paths.LogBase = filepath.Clean(strings.TrimSpace(paths.LogBase))
	paths.LogRoot = filepath.Clean(strings.TrimSpace(paths.LogRoot))
	paths.MarkerPath = filepath.Clean(strings.TrimSpace(paths.MarkerPath))
	return paths
}

func validatePaths(paths Paths) error {
	for _, root := range []managedRoot{
		{kind: "config", base: paths.ConfigBase, path: paths.ConfigRoot},
		{kind: "cache", base: paths.CacheBase, path: paths.CacheRoot},
		{kind: "logs", base: paths.LogBase, path: paths.LogRoot},
	} {
		if err := validateDirectChild(root.base, root.path); err != nil {
			return fmt.Errorf("invalid %s reset root: %w", root.kind, err)
		}
	}
	if err := validateDirectChild(paths.ConfigBase, paths.MarkerPath); err != nil {
		return fmt.Errorf("invalid application reset marker path: %w", err)
	}
	if filepath.Base(paths.MarkerPath) != markerFileName {
		return fmt.Errorf("invalid application reset marker name")
	}
	return nil
}

func validateDirectChild(base, path string) error {
	if base == "" || path == "" || base == "." || path == "." || !filepath.IsAbs(base) || !filepath.IsAbs(path) {
		return fmt.Errorf("managed reset paths must be absolute")
	}
	relative, err := filepath.Rel(base, path)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) ||
		strings.Contains(relative, string(filepath.Separator)) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("managed reset path is not a direct child")
	}
	return nil
}

func readMarker(path string) (resetMarker, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return resetMarker{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return resetMarker{}, fmt.Errorf("application reset marker is not a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return resetMarker{}, fmt.Errorf("application reset marker permissions are too broad")
	}
	if info.Size() <= 0 || info.Size() > markerSizeLimit {
		return resetMarker{}, fmt.Errorf("invalid application reset marker size")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return resetMarker{}, fmt.Errorf("read application reset marker: %w", err)
	}
	var marker resetMarker
	if err := json.Unmarshal(payload, &marker); err != nil {
		return resetMarker{}, fmt.Errorf("decode application reset marker: %w", err)
	}
	if marker.FormatVersion != markerFormatVersion {
		return resetMarker{}, fmt.Errorf("unsupported application reset marker format")
	}
	if _, err := uuid.Parse(marker.ResetID); err != nil {
		return resetMarker{}, fmt.Errorf("invalid application reset marker id")
	}
	if _, err := time.Parse(time.RFC3339Nano, marker.RequestedAt); err != nil {
		return resetMarker{}, fmt.Errorf("invalid application reset marker time")
	}
	return marker, nil
}

func collapseManagedRoots(roots []managedRoot) []managedRoot {
	sort.SliceStable(roots, func(i, j int) bool { return len(roots[i].path) < len(roots[j].path) })
	result := make([]managedRoot, 0, len(roots))
	for _, candidate := range roots {
		covered := false
		for _, existing := range result {
			if samePath(existing.path, candidate.path) || pathContains(existing.path, candidate.path) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, candidate)
		}
	}
	return result
}

func resetManagedRoot(ctx context.Context, root managedRoot, resetID string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := ensureBaseDirectory(root.base); err != nil {
		return err
	}
	trashName := ".xiadown-reset-trash-" + resetID + "-" + root.kind
	trashPath := filepath.Join(root.base, trashName)
	if err := validateDirectChild(root.base, trashPath); err != nil {
		return err
	}
	if err := removeResetTrash(trashPath); err != nil {
		return err
	}

	info, err := os.Lstat(root.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to reset symbolic link")
	}
	if err := os.Rename(root.path, trashPath); err != nil {
		return err
	}
	if err := syncDirectory(root.base); err != nil {
		_ = os.Rename(trashPath, root.path)
		return err
	}
	if err := os.RemoveAll(trashPath); err != nil {
		return err
	}
	return syncDirectory(root.base)
}

func removeResetTrash(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing application reset trash symlink")
	}
	return os.RemoveAll(path)
}

func removeRegularMarker(path, base string) error {
	if err := ensureBaseDirectory(base); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("application reset marker is not a regular file")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove application reset marker: %w", err)
	}
	return nil
}

func ensureBaseDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(path, 0o700)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("managed reset base is not a real directory")
	}
	return nil
}

func pathContains(root, path string) bool {
	if runtime.GOOS == "windows" {
		root = strings.ToLower(filepath.Clean(root))
		path = strings.ToLower(filepath.Clean(path))
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && relative != ".." && !filepath.IsAbs(relative) &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	err = directory.Sync()
	if runtime.GOOS == "windows" || errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EPERM) {
		return nil
	}
	return err
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
