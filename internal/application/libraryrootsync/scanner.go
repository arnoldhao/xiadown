package libraryrootsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/google/uuid"

	"xiadown/internal/application/fileclassification"
	libraryservice "xiadown/internal/application/library/service"
	"xiadown/internal/domain/library"
)

type discoveredFile struct {
	path       string
	relative   string
	info       os.FileInfo
	wasSymlink bool
}

func walkScanTargets(
	ctx context.Context,
	rootPath string,
	targetPaths []string,
	visit func(discoveredFile) error,
) error {
	rootPath = filepath.Clean(rootPath)
	targets := normalizeScanTargets(rootPath, targetPaths)
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Lstat(target)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				relative, relativeErr := safeRelativePath(rootPath, target)
				if relativeErr == nil && relative != "." &&
					!isTransientLibraryArtifact(relative) {
					if err := visit(discoveredFile{path: target, relative: relative}); err != nil {
						return err
					}
				}
				continue
			}
			return err
		}
		if !info.IsDir() {
			item, ok, err := scanFile(rootPath, target, info)
			if err != nil {
				return err
			}
			if ok {
				if err := visit(item); err != nil {
					return err
				}
			}
			continue
		}
		err = filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if path == target {
				return nil
			}
			relative, err := safeRelativePath(rootPath, path)
			if err != nil {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if pathHasHiddenComponent(relative) || platformPathHidden(path) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			item, ok, err := scanFile(rootPath, path, info)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			return visit(item)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func scanFile(rootPath, path string, info os.FileInfo) (discoveredFile, bool, error) {
	relative, err := safeRelativePath(rootPath, path)
	if err != nil || relative == "." {
		return discoveredFile{}, false, err
	}
	if pathHasHiddenComponent(relative) || platformPathHidden(path) ||
		isTransientLibraryArtifact(relative) {
		return discoveredFile{}, false, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return discoveredFile{}, false, nil
	}
	if !info.Mode().IsRegular() {
		return discoveredFile{}, false, nil
	}
	return discoveredFile{path: path, relative: relative, info: info}, true, nil
}

// isTransientLibraryArtifact excludes incomplete downloader files and
// XiaDown's operation-scoped FFmpeg outputs. Those files can be visible for a
// few seconds inside a managed root, but they are not durable Library assets.
func isTransientLibraryArtifact(path string) bool {
	name := strings.TrimSpace(filepath.Base(path))
	if name == "" {
		return false
	}
	lowerName := strings.ToLower(name)
	for _, suffix := range []string{
		".aria2",
		".crdownload",
		".download",
		".part",
		".ytdl",
	} {
		if strings.HasSuffix(lowerName, suffix) {
			return true
		}
	}

	withoutExtension := name
	if extension := filepath.Ext(withoutExtension); !strings.EqualFold(extension, ".tmp") {
		withoutExtension = strings.TrimSuffix(withoutExtension, extension)
	}
	if !strings.HasSuffix(strings.ToLower(withoutExtension), ".tmp") {
		return false
	}
	operationScoped := withoutExtension[:len(withoutExtension)-len(".tmp")]
	separator := strings.LastIndex(operationScoped, ".")
	if separator < 0 || separator == len(operationScoped)-1 {
		return false
	}
	_, err := uuid.Parse(operationScoped[separator+1:])
	return err == nil
}

func normalizeScanTargets(rootPath string, targetPaths []string) []string {
	if len(targetPaths) == 0 {
		return []string{rootPath}
	}
	byPath := make(map[string]struct{}, len(targetPaths))
	for _, rawPath := range targetPaths {
		path := filepath.Clean(strings.TrimSpace(rawPath))
		if path == "." || path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(rootPath, path)
		}
		if _, err := safeRelativePath(rootPath, path); err != nil {
			continue
		}
		byPath[path] = struct{}{}
	}
	items := make([]string, 0, len(byPath))
	for path := range byPath {
		items = append(items, path)
	}
	sort.Strings(items)
	result := make([]string, 0, len(items))
	for _, path := range items {
		covered := false
		for _, parent := range result {
			if pathWithin(path, parent) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, path)
		}
	}
	return result
}

func safeRelativePath(rootPath, path string) (string, error) {
	relative, err := filepath.Rel(rootPath, path)
	if err != nil || filepath.IsAbs(relative) || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", os.ErrPermission
	}
	return filepath.ToSlash(relative), nil
}

func pathWithin(path, parent string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && !filepath.IsAbs(relative) &&
		relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func pathHasHiddenComponent(relative string) bool {
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
	}
	return false
}

func hashFile(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			if _, err := digest.Write(buffer[:read]); err != nil {
				return "", err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func canonicalPath(path string) string {
	cleaned := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

func fileKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".m4a", ".aac", ".flac", ".wav", ".ogg", ".opus", ".wma":
		return string(library.FileKindAudio)
	case ".mp4", ".mkv", ".webm", ".mov", ".avi", ".m4v", ".m2ts", ".flv", ".wmv":
		return string(library.FileKindVideo)
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md", ".rtf", ".epub":
		return string(library.FileKindDocument)
	case ".woff", ".woff2", ".ttf", ".otf", ".eot":
		return string(library.FileKindFont)
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz":
		return string(library.FileKindArchive)
	case ".m3u8", ".mpd", ".f4m", ".ism":
		return string(library.FileKindManifest)
	default:
		return string(library.FileKindOther)
	}
}

func (service *Service) resolvedFileKind(
	ctx context.Context,
	path string,
) string {
	kind := fileKind(path)
	if !fileclassification.IsAmbiguousMPEGTransportPath(path) ||
		!fileclassification.LooksLikeMPEGTransportStream(path) {
		return kind
	}
	if inspector, ok := service.importer.(interface {
		InspectProfessionalImport(
			context.Context,
			string,
		) (libraryservice.ProfessionalImportProbe, error)
	}); ok {
		if probe, err := inspector.InspectProfessionalImport(ctx, path); err == nil {
			switch {
			case probe.HasVideo:
				return string(library.FileKindVideo)
			case probe.HasAudio:
				return string(library.FileKindAudio)
			}
		}
	}
	// A structurally valid transport stream remains a video-compatible asset
	// when media probing is unavailable.
	return string(library.FileKindVideo)
}
