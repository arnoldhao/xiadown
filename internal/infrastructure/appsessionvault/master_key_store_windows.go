//go:build windows

package appsessionvault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsMasterKeyStore struct{}

func newPlatformMasterKeyStore() masterKeyStore {
	return windowsMasterKeyStore{}
}

func inspectPlatformMasterKey(ctx context.Context) (int, int64, error) {
	if err := contextError(ctx); err != nil {
		return 0, 0, err
	}
	path, err := windowsMasterKeyPath()
	if err != nil {
		return 0, 0, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("inspect DPAPI master key: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return 0, 0, fmt.Errorf("DPAPI master key is not a regular file")
	}
	return 1, info.Size(), nil
}

func (windowsMasterKeyStore) Load(ctx context.Context) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	path, err := windowsMasterKeyPath()
	if err != nil {
		return nil, err
	}
	protected, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errMasterKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read DPAPI master key: %w", err)
	}
	key, err := windowsUnprotect(protected)
	if err != nil {
		return nil, fmt.Errorf("unprotect DPAPI master key: %w", err)
	}
	return key, nil
}

func (windowsMasterKeyStore) Store(ctx context.Context, key []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(key) != masterKeyBytes {
		return fmt.Errorf("invalid master key length")
	}
	protected, err := windowsProtect(key)
	if err != nil {
		return fmt.Errorf("protect DPAPI master key: %w", err)
	}
	path, err := windowsMasterKeyPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create DPAPI master key directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return errMasterKeyAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create DPAPI master key: %w", err)
	}
	written := false
	defer func() {
		_ = file.Close()
		if !written {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(protected); err != nil {
		return fmt.Errorf("write DPAPI master key: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync DPAPI master key: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close DPAPI master key: %w", err)
	}
	written = true
	return nil
}

func (windowsMasterKeyStore) Delete(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	path, err := windowsMasterKeyPath()
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect DPAPI master key: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("DPAPI master key is not a regular file")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete DPAPI master key: %w", err)
	}
	return nil
}

func deleteLegacyAppSessionSecrets(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("resolve legacy App Session directory: %w", err)
		}
	}
	root := filepath.Join(base, "XiaDown", "app-sessions")
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect legacy App Session directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("legacy App Session path is not a real directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := contextError(ctx); err != nil {
			return err
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Base(entry.Name()) != entry.Name() ||
			!strings.HasSuffix(strings.ToLower(entry.Name()), ".json.dpapi") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		entryInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("legacy App Session secret is not a regular file")
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete legacy App Session secret: %w", err)
		}
	}
	return nil
}

func windowsProtect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	description, _ := windows.UTF16PtrFromString(DarwinKeychainService)
	if err := windows.CryptProtectData(
		&in, description, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out,
	); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}

func windowsUnprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty DPAPI payload")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(
		&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out,
	); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}

func windowsMasterKeyPath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve App Session vault config directory: %w", err)
		}
	}
	return filepath.Join(base, "XiaDown", "session-vault", masterKeyAccount+".dpapi"), nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
