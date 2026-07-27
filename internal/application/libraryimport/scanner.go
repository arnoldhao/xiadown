package libraryimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"

	"xiadown/internal/application/fileclassification"
	importdomain "xiadown/internal/domain/libraryimport"
)

var importCandidateNamespace = uuid.MustParse("e17cbb15-c7ec-5486-aae3-a3f646fb475b")

type Scanner struct {
	inspector MediaInspector
	now       func() time.Time
}

type scanOptions struct {
	BatchID       string
	HiddenPolicy  importdomain.HiddenPolicy
	SymlinkPolicy importdomain.SymlinkPolicy
}

func NewScanner(inspector MediaInspector) *Scanner {
	return &Scanner{inspector: inspector, now: func() time.Time { return time.Now().UTC() }}
}

func (scanner *Scanner) Scan(ctx context.Context, sourcePaths []string, options scanOptions) ([]importdomain.Candidate, error) {
	if strings.TrimSpace(options.BatchID) == "" || len(sourcePaths) == 0 {
		return nil, fmt.Errorf("import batch and at least one source are required")
	}
	paths := make([]scannedPath, 0)
	seen := make(map[string]struct{})
	for _, rawRoot := range sourcePaths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		root, err := filepath.Abs(strings.TrimSpace(rawRoot))
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(root)
		if err != nil {
			paths = append(paths, scannedPath{
				sourcePath: root, relativePath: filepath.Base(root),
				skippedCode: "source_unavailable", skippedError: err.Error(),
			})
			continue
		}
		if info.IsDir() {
			err = scanner.walkDirectory(ctx, root, options, seen, &paths)
		} else {
			err = scanner.collectPath(ctx, root, filepath.Base(root), options, seen, &paths)
		}
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(paths, func(left, right int) bool {
		return paths[left].sourcePath < paths[right].sourcePath
	})
	candidates := make([]importdomain.Candidate, 0, len(paths))
	seenDigest := make(map[string]string)
	for _, item := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		candidate, err := scanner.inspectPath(ctx, options.BatchID, item)
		if err != nil {
			return nil, err
		}
		if candidate.Status == importdomain.CandidateReady {
			key := digestKey(candidate.SizeBytes, candidate.ContentHash)
			if duplicateID := seenDigest[key]; duplicateID != "" {
				candidate.Status = importdomain.CandidateDuplicate
				candidate.DuplicateCandidateID = duplicateID
			} else {
				seenDigest[key] = candidate.ID
			}
		}
		candidate, err = importdomain.NewCandidate(candidate)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

type scannedPath struct {
	sourcePath   string
	relativePath string
	wasSymlink   bool
	skippedCode  string
	skippedError string
}

func (scanner *Scanner) walkDirectory(ctx context.Context, root string, options scanOptions, seen map[string]struct{}, paths *[]scannedPath) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			relative, _ := filepath.Rel(root, path)
			*paths = append(*paths, scannedPath{
				sourcePath: path, relativePath: relative,
				skippedCode: "scan_unreadable", skippedError: walkErr.Error(),
			})
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if options.HiddenPolicy == importdomain.HiddenExclude && (pathHasHiddenComponent(relative) || platformPathHidden(path)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		return scanner.collectPath(ctx, path, relative, options, seen, paths)
	})
}

func (scanner *Scanner) collectPath(ctx context.Context, path, relative string, options scanOptions, seen map[string]struct{}, paths *[]scannedPath) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		*paths = append(*paths, scannedPath{
			sourcePath: path, relativePath: relative,
			skippedCode: "source_unavailable", skippedError: err.Error(),
		})
		return nil
	}
	if options.HiddenPolicy == importdomain.HiddenExclude && (pathHasHiddenComponent(relative) || platformPathHidden(path)) {
		return nil
	}
	item := scannedPath{sourcePath: path, relativePath: relative}
	if info.Mode()&os.ModeSymlink != 0 {
		item.wasSymlink = true
		if options.SymlinkPolicy == importdomain.SymlinkSkip {
			item.skippedCode = "symlink_skipped"
			item.skippedError = "symbolic links are excluded by this import policy"
			*paths = append(*paths, item)
			return nil
		}
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			item.skippedCode = "symlink_unresolvable"
			item.skippedError = resolveErr.Error()
			*paths = append(*paths, item)
			return nil
		}
		resolvedInfo, statErr := os.Stat(resolved)
		if statErr != nil || !resolvedInfo.Mode().IsRegular() {
			item.skippedCode = "symlink_not_regular_file"
			item.skippedError = "only symbolic links to regular files can be followed"
			*paths = append(*paths, item)
			return nil
		}
		item.sourcePath, err = filepath.Abs(resolved)
		if err != nil {
			return err
		}
		info = resolvedInfo
	}
	if !info.Mode().IsRegular() {
		item.skippedCode = "not_regular_file"
		item.skippedError = "only regular files can be imported"
		*paths = append(*paths, item)
		return nil
	}
	canonical := canonicalPathKey(item.sourcePath)
	if _, exists := seen[canonical]; exists {
		return nil
	}
	seen[canonical] = struct{}{}
	*paths = append(*paths, item)
	return nil
}

func (scanner *Scanner) inspectPath(ctx context.Context, batchID string, item scannedPath) (importdomain.Candidate, error) {
	now := scanner.now().UTC()
	displayName := filepath.Base(item.sourcePath)
	base := importdomain.Candidate{
		ID:      uuid.NewSHA1(importCandidateNamespace, []byte(batchID+"\x00"+canonicalPathKey(item.sourcePath))).String(),
		BatchID: batchID, SourcePath: item.sourcePath, RelativePath: item.relativePath,
		DisplayName: displayName, Extension: strings.ToLower(filepath.Ext(displayName)),
		Category: importdomain.CategoryOther, WasSymlink: item.wasSymlink,
		Status: importdomain.CandidateReady, CreatedAt: now, UpdatedAt: now,
	}
	if item.skippedCode != "" {
		base.Status = importdomain.CandidateSkipped
		base.ErrorCode = item.skippedCode
		base.ErrorMessage = item.skippedError
		return base, nil
	}
	info, err := os.Stat(item.sourcePath)
	if err != nil {
		base.Status = importdomain.CandidateSkipped
		base.ErrorCode = "source_unavailable"
		base.ErrorMessage = err.Error()
		return base, nil
	}
	base.SizeBytes = info.Size()
	base.ModifiedAt = info.ModTime().UTC()
	base.HashAlgorithm = "sha256"
	base.ContentHash, err = hashFileSHA256(ctx, item.sourcePath)
	if err != nil {
		base.HashAlgorithm = ""
		base.Status = importdomain.CandidateSkipped
		base.ErrorCode = "hash_failed"
		base.ErrorMessage = err.Error()
		return base, nil
	}
	base.Category = categoryFromExtension(base.Extension)
	if detected, detectErr := mimetype.DetectFile(item.sourcePath); detectErr == nil && detected != nil {
		base.MIMEType = detected.String()
	}
	if scanner.inspector != nil && shouldProbeMedia(base.Category, base.Extension) {
		if probe, probeErr := scanner.inspector.InspectProfessionalImport(ctx, item.sourcePath); probeErr == nil {
			base.MediaProbed = true
			if probe.HasVideo {
				base.Category = importdomain.CategoryVideo
			} else if probe.HasAudio {
				base.Category = importdomain.CategoryAudio
			}
		}
	}
	if base.Category == importdomain.CategoryOther &&
		fileclassification.IsAmbiguousMPEGTransportPath(item.sourcePath) &&
		fileclassification.LooksLikeMPEGTransportStream(item.sourcePath) {
		base.Category = importdomain.CategoryVideo
	}
	return base, nil
}

func hashFileSHA256(ctx context.Context, path string) (string, error) {
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

func categoryFromExtension(extension string) importdomain.Category {
	extension = strings.ToLower(strings.TrimSpace(extension))
	switch extension {
	case ".avi", ".flv", ".m2ts", ".m4v", ".mkv", ".mov", ".mp4", ".mpeg", ".mpg", ".ogv", ".webm", ".wmv":
		return importdomain.CategoryVideo
	case ".aac", ".aif", ".aiff", ".alac", ".ape", ".caf", ".flac", ".m4a", ".m4b", ".mp3", ".mpga", ".oga", ".ogg", ".opus", ".wav", ".wave", ".weba", ".wma":
		return importdomain.CategoryAudio
	case ".azw", ".azw3", ".cbr", ".cbz", ".epub", ".fb2", ".mobi", ".pdf":
		return importdomain.CategoryBook
	case ".avif", ".bmp", ".gif", ".heic", ".heif", ".ico", ".jpeg", ".jpg", ".png", ".svg", ".tif", ".tiff", ".webp":
		return importdomain.CategoryImage
	default:
		return importdomain.CategoryOther
	}
}

func shouldProbeMedia(category importdomain.Category, extension string) bool {
	if category == importdomain.CategoryVideo || category == importdomain.CategoryAudio || category == importdomain.CategoryOther {
		return true
	}
	return strings.TrimSpace(extension) == ""
}

func digestKey(size int64, digest string) string { return fmt.Sprintf("%d:%s", size, digest) }

func pathHasHiddenComponent(path string) bool {
	for _, component := range strings.Split(filepath.Clean(path), string(os.PathSeparator)) {
		if strings.HasPrefix(component, ".") && component != "." && component != ".." {
			return true
		}
	}
	return false
}

func canonicalPathKey(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(cleaned)
	}
	return cleaned
}
