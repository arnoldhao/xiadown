package libraryimport

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	importdomain "xiadown/internal/domain/libraryimport"
)

type managedDirectory interface {
	absolutePath() string
	openExisting(name string) (*os.File, error)
	createExclusive(name string, mode os.FileMode) (*os.File, error)
	remove(name string) error
	publishNoReplace(sourceName, destinationName string) error
	sync() error
	close() error
}

func copyIntoManagedRoot(ctx context.Context, root string, candidate importdomain.Candidate) (string, error) {
	root, err := canonicalManagedRoot(root)
	if err != nil {
		return "", err
	}
	hash := strings.ToLower(strings.TrimSpace(candidate.ContentHash))
	if len(hash) != 64 {
		return "", fmt.Errorf("invalid candidate sha256")
	}
	name := sanitizeManagedFilename(candidate.DisplayName)
	relativeDir := filepath.Join(string(candidate.Category), hash[:2], hash[2:4])
	directory, err := openManagedDirectory(root, relativeDir, true, 0o755)
	if err != nil {
		return "", err
	}
	defer directory.close()
	destinationName := hash + "-" + name
	destination := filepath.Join(directory.absolutePath(), destinationName)
	stageToken := sanitizeStageToken(candidate.ID)
	if stageToken == "" {
		stageToken = hash[:16]
	}
	stageName := ".xiadown-import-" + stageToken + ".stage"
	if !pathWithinRoot(destination, root) {
		return "", fmt.Errorf("managed destination escaped root")
	}
	if matches, matchErr := managedFileMatchesDigest(ctx, directory, destinationName, candidate.SizeBytes, hash); matchErr == nil && matches {
		_ = directory.remove(stageName)
		return destination, nil
	} else if matchErr != nil && !errors.Is(matchErr, os.ErrNotExist) {
		return "", matchErr
	} else if existing, openErr := directory.openExisting(destinationName); openErr == nil {
		_ = existing.Close()
		return "", importdomain.ErrDestinationExists
	} else if !errors.Is(openErr, os.ErrNotExist) {
		return "", openErr
	}
	if matches, matchErr := managedFileMatchesDigest(ctx, directory, stageName, candidate.SizeBytes, hash); matchErr == nil && matches {
		if err := directory.publishNoReplace(stageName, destinationName); err != nil {
			if destinationMatches, verifyErr := managedFileMatchesDigest(ctx, directory, destinationName, candidate.SizeBytes, hash); verifyErr == nil && destinationMatches {
				_ = directory.remove(stageName)
				return destination, nil
			}
			return "", err
		}
		if err := directory.sync(); err != nil {
			return "", err
		}
		return destination, nil
	} else if matchErr != nil && !errors.Is(matchErr, os.ErrNotExist) {
		return "", matchErr
	} else if removeErr := directory.remove(stageName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return "", removeErr
	}

	stage, err := directory.createExclusive(stageName, 0o600)
	if err != nil {
		return "", err
	}
	published := false
	defer func() {
		_ = stage.Close()
		if !published {
			_ = directory.remove(stageName)
		}
	}()
	if err := copyFileWithContext(ctx, stage, candidate.SourcePath); err != nil {
		return "", err
	}
	if err := stage.Chmod(0o644); err != nil {
		return "", err
	}
	if err := stage.Sync(); err != nil {
		return "", err
	}
	if err := stage.Close(); err != nil {
		return "", err
	}
	if matches, verifyErr := managedFileMatchesDigest(ctx, directory, stageName, candidate.SizeBytes, hash); verifyErr != nil {
		return "", verifyErr
	} else if !matches {
		return "", fmt.Errorf("staged copy checksum mismatch")
	}
	if err := directory.publishNoReplace(stageName, destinationName); err != nil {
		if matches, verifyErr := managedFileMatchesDigest(ctx, directory, destinationName, candidate.SizeBytes, hash); verifyErr == nil && matches {
			return destination, nil
		}
		if errors.Is(err, os.ErrExist) {
			return "", importdomain.ErrDestinationExists
		}
		return "", err
	}
	published = true
	if err := directory.sync(); err != nil {
		return "", err
	}
	return destination, nil
}

func copyFileWithContext(ctx context.Context, destination *os.File, sourcePath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			if _, err := destination.Write(buffer[:read]); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func managedPathMatchesDigest(ctx context.Context, root, path string, size int64, digest string) (bool, error) {
	root, err := canonicalManagedRoot(root)
	if err != nil {
		return false, err
	}
	path, err = filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return false, err
	}
	if !pathWithinRoot(path, root) {
		return false, fmt.Errorf("managed path escaped root")
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false, err
	}
	directory, err := openManagedDirectory(root, filepath.Dir(relative), false, 0)
	if err != nil {
		return false, err
	}
	defer directory.close()
	return managedFileMatchesDigest(ctx, directory, filepath.Base(relative), size, digest)
}

func managedFileMatchesDigest(
	ctx context.Context,
	directory managedDirectory,
	name string,
	size int64,
	digest string,
) (bool, error) {
	file, err := directory.openExisting(name)
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return false, nil
	}
	hasher := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			if _, err := hasher.Write(buffer[:read]); err != nil {
				return false, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return false, readErr
		}
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)) == strings.ToLower(strings.TrimSpace(digest)), nil
}

func canonicalManagedRoot(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", importdomain.ErrManagedRootMissing
	}
	root, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	// DryRun stores the fully resolved root. A different resolution at Commit
	// means an ancestor was replaced by a symlink/junction after authorization.
	if canonicalPathKey(root) != canonicalPathKey(resolved) {
		return "", fmt.Errorf("managed root changed through a symlink or reparse point")
	}
	return resolved, nil
}

func managedRelativeComponents(relative string) ([]string, error) {
	relative = filepath.Clean(strings.TrimSpace(relative))
	if relative == "." || relative == "" {
		return nil, nil
	}
	if filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return nil, fmt.Errorf("managed path must be relative")
	}
	components := strings.Split(relative, string(os.PathSeparator))
	for _, component := range components {
		if component == "" || component == "." || component == ".." || filepath.Base(component) != component {
			return nil, fmt.Errorf("invalid managed path component")
		}
	}
	return components, nil
}

func validateManagedLeafName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return fmt.Errorf("invalid managed filename")
	}
	return nil
}

func sanitizeManagedFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	var builder strings.Builder
	for _, value := range name {
		if unicode.IsControl(value) || strings.ContainsRune(`<>:"/\|?*`, value) {
			builder.WriteRune('_')
			continue
		}
		builder.WriteRune(value)
	}
	result := strings.Trim(builder.String(), " .")
	if result == "" {
		return "content.bin"
	}
	stemWithoutExtension := strings.TrimSuffix(result, filepath.Ext(result))
	switch strings.ToUpper(stemWithoutExtension) {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		result = "_" + result
	}
	if len([]rune(result)) > 180 {
		extension := filepath.Ext(result)
		stem := strings.TrimSuffix(result, extension)
		maxStem := 180 - len([]rune(extension))
		if maxStem < 1 {
			return string([]rune(result)[:180])
		}
		stemRunes := []rune(stem)
		if len(stemRunes) > maxStem {
			stemRunes = stemRunes[:maxStem]
		}
		result = string(stemRunes) + extension
	}
	return result
}

func sanitizeStageToken(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' {
			builder.WriteRune(character)
		} else {
			builder.WriteRune('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

func pathWithinRoot(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
