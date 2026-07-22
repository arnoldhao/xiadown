package browserprofile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	snapshotDirectoryPrefix = "snapshot-"
	snapshotIDBytes         = 16
	snapshotStaleAfter      = 24 * time.Hour
)

var (
	snapshotRoot = defaultSnapshotRoot
	snapshotNow  = time.Now

	snapshotOwnershipCheck = snapshotOwnedByCurrentUser
	activeSnapshots        sync.Map
)

// CleanupStaleSnapshots removes only expired XiaDown browser-profile snapshots.
// The root and every candidate must be a real directory owned by the current
// user; symlinks, foreign-owned entries, unexpected names, fresh entries, and
// snapshots active in this process are never traversed or removed.
func CleanupStaleSnapshots(ctx context.Context) error {
	root, err := validateSnapshotRoot(false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read browser snapshot root: %w", err)
	}
	now := snapshotNow()
	for _, entry := range entries {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		if !validSnapshotDirectoryName(entry.Name()) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if _, active := activeSnapshots.Load(path); active {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("inspect browser snapshot: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		owned, err := snapshotOwnershipCheck(path, info)
		if err != nil || !owned {
			continue
		}
		age := now.Sub(info.ModTime())
		if age < snapshotStaleAfter || age < 0 {
			continue
		}
		// RemoveAll does not follow a top-level symlink. Re-check immediately
		// before deletion so a replaced entry fails closed.
		current, err := os.Lstat(path)
		if err != nil || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, current) {
			continue
		}
		owned, err = snapshotOwnershipCheck(path, current)
		if err != nil || !owned {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove stale browser snapshot: %w", err)
		}
	}
	return nil
}

func createSnapshotDirectory() (string, func(), error) {
	root, err := validateSnapshotRoot(true)
	if err != nil {
		return "", nil, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		name, err := newSnapshotDirectoryName()
		if err != nil {
			return "", nil, fmt.Errorf("create browser snapshot identifier: %w", err)
		}
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0o700); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", nil, fmt.Errorf("create browser snapshot directory: %w", err)
		}
		if err := secureSnapshotDirectory(path); err != nil {
			_ = os.RemoveAll(path)
			return "", nil, err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			_ = os.RemoveAll(path)
			if err == nil {
				err = fmt.Errorf("browser snapshot path is not a directory")
			}
			return "", nil, err
		}
		owned, err := snapshotOwnershipCheck(path, info)
		if err != nil || !owned {
			_ = os.RemoveAll(path)
			if err == nil {
				err = fmt.Errorf("browser snapshot directory has an unexpected owner")
			}
			return "", nil, err
		}
		activeSnapshots.Store(path, struct{}{})
		var once sync.Once
		cleanup := func() {
			once.Do(func() {
				activeSnapshots.Delete(path)
				_ = os.RemoveAll(path)
			})
		}
		return path, cleanup, nil
	}
	return "", nil, fmt.Errorf("create unique browser snapshot directory")
}

func validateSnapshotRoot(create bool) (string, error) {
	root := filepath.Clean(strings.TrimSpace(snapshotRoot()))
	if root == "" || root == "." || root == string(filepath.Separator) || !filepath.IsAbs(root) {
		return "", fmt.Errorf("browser snapshot root is invalid")
	}
	created := false
	if create {
		if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
				return "", fmt.Errorf("create browser snapshot parent: %w", err)
			}
			if err := os.Mkdir(root, 0o700); err != nil {
				if !errors.Is(err, os.ErrExist) {
					return "", fmt.Errorf("create browser snapshot root: %w", err)
				}
			} else {
				created = true
			}
		} else if err != nil {
			return "", err
		}
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("browser snapshot root is not a real directory")
	}
	// A directory created by this process may inherit an administrative owner
	// on Windows even though the current user has full control. Establish the
	// private owner and ACL before applying the same fail-closed ownership check
	// used for pre-existing roots. A root that raced into existence is treated
	// as pre-existing and is never modified before its owner is validated.
	if create && created {
		if err := secureSnapshotDirectory(root); err != nil {
			return "", err
		}
	}
	owned, err := snapshotOwnershipCheck(root, info)
	if err != nil {
		return "", fmt.Errorf("inspect browser snapshot root owner: %w", err)
	}
	if !owned {
		return "", fmt.Errorf("browser snapshot root has an unexpected owner")
	}
	if create && !created {
		if err := secureSnapshotDirectory(root); err != nil {
			return "", err
		}
	}
	return root, nil
}

func newSnapshotDirectoryName() (string, error) {
	var random [snapshotIDBytes]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return snapshotDirectoryPrefix + hex.EncodeToString(random[:]), nil
}

func validSnapshotDirectoryName(name string) bool {
	if len(name) != len(snapshotDirectoryPrefix)+snapshotIDBytes*2 || !strings.HasPrefix(name, snapshotDirectoryPrefix) {
		return false
	}
	for _, value := range name[len(snapshotDirectoryPrefix):] {
		if (value < '0' || value > '9') && (value < 'a' || value > 'f') {
			return false
		}
	}
	return true
}

func defaultSnapshotRoot() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheDir) == "" {
		// Browser snapshots contain authentication material. If the platform
		// cannot provide a per-user cache root, fail closed rather than falling
		// back to a shared temporary directory.
		return ""
	}
	return filepath.Join(cacheDir, "xiadown", "browser-profile-snapshots")
}
