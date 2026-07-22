package locallyrics

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func secureReadFile(rootDirectory string, filePath string, options Options) ([]byte, string, error) {
	options = normalizeOptions(options)
	if strings.TrimSpace(rootDirectory) == "" || strings.TrimSpace(filePath) == "" || strings.ContainsRune(filePath, '\x00') {
		return nil, "", ErrInvalidPath
	}

	rootAbsolute, err := filepath.Abs(rootDirectory)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbsolute)
	if err != nil {
		return nil, "", fmt.Errorf("resolve lyric directory: %w", err)
	}
	rootResolved = filepath.Clean(rootResolved)

	fileAbsolute, err := filepath.Abs(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	fileAbsolute = filepath.Clean(fileAbsolute)
	// First validate the lexical path against the lexical root. The resolved
	// root can legitimately differ on platforms where /var is a symlink to
	// /private/var; comparing a resolved root to an unresolved child would
	// incorrectly reject safe files.
	if !pathWithin(filepath.Clean(rootAbsolute), fileAbsolute) {
		return nil, "", ErrPathEscape
	}

	linkInfo, err := os.Lstat(fileAbsolute)
	if err != nil {
		return nil, "", err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 && !options.AllowSymlinks {
		return nil, "", ErrUnsafeFile
	}

	resolvedPath, err := filepath.EvalSymlinks(fileAbsolute)
	if err != nil {
		return nil, "", fmt.Errorf("resolve lyric file: %w", err)
	}
	resolvedPath = filepath.Clean(resolvedPath)
	if !pathWithin(rootResolved, resolvedPath) {
		return nil, "", ErrPathEscape
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, "", ErrUnsafeFile
	}
	if info.Size() > options.MaxBytes {
		return nil, "", ErrTooLarge
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 && !os.SameFile(linkInfo, info) {
		return nil, "", ErrUnsafeFile
	}

	content, err := io.ReadAll(io.LimitReader(file, options.MaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(content)) > options.MaxBytes {
		return nil, "", ErrTooLarge
	}
	return content, resolvedPath, nil
}

func pathWithin(root string, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || filepath.IsAbs(relative) {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func isMissingFile(errorValue error) bool {
	return errors.Is(errorValue, os.ErrNotExist)
}
