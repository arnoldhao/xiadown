package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"xiadown/internal/domain/library"
)

const (
	catalogCardPreviewFormatVersion = "v1"
	catalogLogPreviewReadBytes      = 16 << 10
	catalogLogPreviewLineLimit      = 7
	catalogLogPreviewRuneLimit      = 96
	catalogLogDetailReadBytes       = 256 << 10
	catalogLogDetailLineLimit       = 2_000
	catalogLogDetailRuneLimit       = 4_096
)

var (
	ErrCatalogCardPreviewNotFound    = errors.New("catalog card preview not found")
	ErrCatalogCardPreviewUnavailable = errors.New("catalog card preview unavailable")

	catalogLogANSIPattern = regexp.MustCompile(
		"\\x1b(?:\\[[0-?]*[ -/]*[@-~]|\\][^\\x07]*(?:\\x07|\\x1b\\\\))",
	)
	catalogLogURLPattern              = regexp.MustCompile(`https?://[^\s<>"']+`)
	catalogLogCredentialHeaderPattern = regexp.MustCompile(
		`(?i)\b(authorization|proxy-authorization|cookie|set-cookie)\s*:\s*.*$`,
	)
	catalogLogCredentialArgumentPattern = regexp.MustCompile(
		`(?i)--(cookies?|username|password|token|api-key)\s+\S+`,
	)
	catalogLogBearerPattern = regexp.MustCompile(
		`(?i)\b(bearer|basic)\s+[a-z0-9+/_=.\-]+`,
	)
	catalogLogSecretValuePattern = regexp.MustCompile(
		`(?i)\b(token|access[_-]?token|refresh[_-]?token|api[_-]?key|password|passwd|secret|signature|sig)\b(\s*[:=]\s*)([^\s,;]+)`,
	)
)

// CatalogPDFCardPreview keeps an already-open, registered Catalog source
// inside the token-protected desktop HTTP boundary. Callers own File and must
// close it after serving the request.
type CatalogPDFCardPreview struct {
	File    *os.File
	Name    string
	Size    int64
	ModTime time.Time
	ETag    string
}

type CatalogLogCardPreview struct {
	Lines     []string
	Truncated bool
	ETag      string
}

type CatalogLogTextPreview struct {
	Text      string
	Truncated bool
	ETag      string
}

type catalogCardPreviewSource struct {
	item library.Item
	file library.LibraryFile
	open *os.File
	info os.FileInfo
}

// CatalogCardPreviewService exposes only bounded preview primitives for an
// opaque Catalog item ID. It never accepts a path from the caller.
type CatalogCardPreviewService struct {
	items  library.CatalogItemRepository
	assets library.ItemAssetRepository
	files  library.FileRepository
	roots  library.StorageRootRepository
}

func NewCatalogCardPreviewService(
	items library.CatalogItemRepository,
	assets library.ItemAssetRepository,
	files library.FileRepository,
	roots ...library.StorageRootRepository,
) *CatalogCardPreviewService {
	service := &CatalogCardPreviewService{items: items, assets: assets, files: files}
	if len(roots) > 0 {
		service.roots = roots[0]
	}
	return service
}

func (service *CatalogCardPreviewService) ResolvePDF(
	ctx context.Context,
	itemID string,
) (CatalogPDFCardPreview, error) {
	source, err := service.resolveSource(ctx, itemID)
	if err != nil {
		return CatalogPDFCardPreview{}, err
	}
	if !catalogCardPreviewIsPDF(source.file, source.open.Name()) {
		_ = source.open.Close()
		return CatalogPDFCardPreview{}, ErrCatalogCardPreviewNotFound
	}
	key := catalogCardPreviewCacheKey(source.item.ID, source.file.ID, source.info)
	return CatalogPDFCardPreview{
		File:    source.open,
		Name:    catalogCardPreviewSafeName(source.open.Name(), "document.pdf"),
		Size:    source.info.Size(),
		ModTime: source.info.ModTime(),
		ETag:    `"xiadown-pdf-card-preview-` + key + `"`,
	}, nil
}

func (service *CatalogCardPreviewService) ResolveLog(
	ctx context.Context,
	itemID string,
) (CatalogLogCardPreview, error) {
	source, err := service.resolveSource(ctx, itemID)
	if err != nil {
		return CatalogLogCardPreview{}, err
	}
	defer source.open.Close()
	if !catalogCardPreviewIsLog(source.file, source.open.Name()) {
		return CatalogLogCardPreview{}, ErrCatalogCardPreviewNotFound
	}
	lines, truncated, err := readCatalogLogPreview(source.open, source.info.Size())
	if err != nil {
		return CatalogLogCardPreview{}, ErrCatalogCardPreviewUnavailable
	}
	key := catalogCardPreviewCacheKey(source.item.ID, source.file.ID, source.info)
	return CatalogLogCardPreview{
		Lines: lines, Truncated: truncated,
		ETag: `"xiadown-log-card-preview-` + key + `"`,
	}, nil
}

// ResolveLogText is intentionally bounded and is only requested when the
// user opens the text dialog. It shares the card preview's redaction rules.
func (service *CatalogCardPreviewService) ResolveLogText(
	ctx context.Context,
	itemID string,
) (CatalogLogTextPreview, error) {
	source, err := service.resolveSource(ctx, itemID)
	if err != nil {
		return CatalogLogTextPreview{}, err
	}
	defer source.open.Close()
	if !catalogCardPreviewIsLog(source.file, source.open.Name()) {
		return CatalogLogTextPreview{}, ErrCatalogCardPreviewNotFound
	}
	text, truncated, err := readCatalogLogText(source.open, source.info.Size())
	if err != nil {
		return CatalogLogTextPreview{}, ErrCatalogCardPreviewUnavailable
	}
	key := catalogCardPreviewCacheKey(source.item.ID, source.file.ID, source.info)
	return CatalogLogTextPreview{
		Text: text, Truncated: truncated,
		ETag: `"xiadown-log-text-preview-` + key + `"`,
	}, nil
}

func (service *CatalogCardPreviewService) resolveSource(
	ctx context.Context,
	itemID string,
) (catalogCardPreviewSource, error) {
	if service == nil || service.items == nil || service.assets == nil || service.files == nil {
		return catalogCardPreviewSource{}, ErrCatalogCardPreviewUnavailable
	}
	if !ValidCatalogCardPreviewItemID(itemID) {
		return catalogCardPreviewSource{}, ErrCatalogCardPreviewNotFound
	}
	itemID = strings.TrimSpace(itemID)
	item, err := service.items.Get(ctx, itemID)
	if err != nil || item.Status == library.ItemStatusMissing || item.Status == library.ItemStatusTrashed {
		return catalogCardPreviewSource{}, ErrCatalogCardPreviewNotFound
	}
	assets, err := service.assets.ListByItemID(ctx, item.ID)
	if err != nil {
		return catalogCardPreviewSource{}, ErrCatalogCardPreviewUnavailable
	}
	filesByAssetID := make(map[string]library.LibraryFile, len(assets))
	for _, asset := range assets {
		if asset.Role != library.ItemAssetRoleOriginal &&
			asset.Role != library.ItemAssetRoleRepresentation {
			continue
		}
		file, fileErr := service.files.Get(ctx, asset.FileID)
		if fileErr != nil {
			continue
		}
		filesByAssetID[asset.ID] = file
	}
	_, file, ok := selectCatalogPrimaryAsset(assets, filesByAssetID)
	if !ok || !catalogFileCanAttemptRead(ctx, file, service.roots) {
		return catalogCardPreviewSource{}, ErrCatalogCardPreviewNotFound
	}
	rawPath := strings.TrimSpace(file.Storage.LocalPath)
	path, err := filepath.Abs(rawPath)
	if err != nil || rawPath == "" || !filepath.IsAbs(path) {
		return catalogCardPreviewSource{}, ErrCatalogCardPreviewNotFound
	}
	open, err := os.Open(filepath.Clean(path))
	if err != nil {
		return catalogCardPreviewSource{}, ErrCatalogCardPreviewNotFound
	}
	info, err := open.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		_ = open.Close()
		return catalogCardPreviewSource{}, ErrCatalogCardPreviewNotFound
	}
	return catalogCardPreviewSource{item: item, file: file, open: open, info: info}, nil
}

// ValidCatalogCardPreviewItemID applies the opaque Catalog ID boundary before
// repository or filesystem work.
func ValidCatalogCardPreviewItemID(value string) bool {
	return ValidCatalogVideoThumbnailItemID(value)
}

func catalogCardPreviewIsPDF(file library.LibraryFile, path string) bool {
	if strings.EqualFold(filepath.Ext(path), ".pdf") {
		return true
	}
	if file.Media == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(file.Media.Format)) {
	case "pdf", "application/pdf":
		return true
	default:
		return false
	}
}

func catalogCardPreviewIsLog(file library.LibraryFile, path string) bool {
	if strings.EqualFold(filepath.Ext(path), ".log") {
		return true
	}
	if file.Media == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(file.Media.Format)) {
	case "log", "text/x-log":
		return true
	default:
		return false
	}
}

func catalogCardPreviewSafeName(path string, fallback string) string {
	name := strings.TrimSpace(filepath.Base(path))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fallback
	}
	return name
}

func catalogCardPreviewCacheKey(itemID string, fileID string, info os.FileInfo) string {
	hash := sha256.New()
	for _, value := range []string{
		catalogCardPreviewFormatVersion,
		strings.TrimSpace(itemID),
		strings.TrimSpace(fileID),
		strconv.FormatInt(info.Size(), 10),
		strconv.FormatInt(info.ModTime().UnixNano(), 10),
	} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func readCatalogLogPreview(file *os.File, size int64) ([]string, bool, error) {
	if file == nil || size <= 0 {
		return []string{}, false, nil
	}
	start := max(int64(0), size-catalogLogPreviewReadBytes)
	buffer := make([]byte, size-start)
	read, err := file.ReadAt(buffer, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	buffer = buffer[:read]
	if start > 0 {
		if boundary := bytes.IndexAny(buffer, "\r\n"); boundary >= 0 {
			buffer = buffer[boundary+1:]
		} else {
			buffer = nil
		}
	}
	content := strings.ReplaceAll(strings.ToValidUTF8(string(buffer), "�"), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	rawLines := strings.Split(content, "\n")
	lines := make([]string, 0, catalogLogPreviewLineLimit)
	for index := len(rawLines) - 1; index >= 0 && len(lines) < catalogLogPreviewLineLimit; index-- {
		line := sanitizeCatalogLogPreviewLine(rawLines[index])
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return lines, start > 0 || len(rawLines) > catalogLogPreviewLineLimit, nil
}

func sanitizeCatalogLogPreviewLine(value string) string {
	return sanitizeCatalogLogLine(value, catalogLogPreviewRuneLimit, true)
}

func readCatalogLogText(file *os.File, size int64) (string, bool, error) {
	if file == nil || size <= 0 {
		return "", false, nil
	}
	start := max(int64(0), size-catalogLogDetailReadBytes)
	buffer := make([]byte, size-start)
	read, err := file.ReadAt(buffer, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false, err
	}
	buffer = buffer[:read]
	if start > 0 {
		if boundary := bytes.IndexAny(buffer, "\r\n"); boundary >= 0 {
			buffer = buffer[boundary+1:]
		} else {
			buffer = nil
		}
	}
	content := strings.ReplaceAll(strings.ToValidUTF8(string(buffer), "�"), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	rawLines := strings.Split(content, "\n")
	truncated := start > 0 || len(rawLines) > catalogLogDetailLineLimit
	if len(rawLines) > catalogLogDetailLineLimit {
		rawLines = rawLines[len(rawLines)-catalogLogDetailLineLimit:]
	}
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		lines = append(lines, sanitizeCatalogLogLine(line, catalogLogDetailRuneLimit, false))
	}
	return strings.Join(lines, "\n"), truncated, nil
}

func sanitizeCatalogLogLine(value string, runeLimit int, collapseWhitespace bool) string {
	value = catalogLogANSIPattern.ReplaceAllString(value, "")
	value = catalogLogURLPattern.ReplaceAllStringFunc(value, redactCatalogLogPreviewURL)
	value = catalogLogCredentialHeaderPattern.ReplaceAllString(value, "$1: [redacted]")
	value = catalogLogCredentialArgumentPattern.ReplaceAllString(value, "--$1 [redacted]")
	value = catalogLogBearerPattern.ReplaceAllString(value, "$1 [redacted]")
	value = catalogLogSecretValuePattern.ReplaceAllString(value, "$1$2[redacted]")

	var cleaned strings.Builder
	for _, character := range value {
		if character == '\t' || (!unicode.IsControl(character) && character != unicode.ReplacementChar) {
			cleaned.WriteRune(character)
		} else {
			cleaned.WriteByte(' ')
		}
	}
	value = cleaned.String()
	if collapseWhitespace {
		value = strings.Join(strings.Fields(value), " ")
	} else {
		value = strings.TrimRight(value, " ")
	}
	runes := []rune(value)
	if len(runes) > runeLimit {
		value = string(runes[:runeLimit-1]) + "…"
	}
	return value
}

func redactCatalogLogPreviewURL(value string) string {
	trailing := ""
	for len(value) > 0 && strings.ContainsRune(".,);]}", rune(value[len(value)-1])) {
		trailing = value[len(value)-1:] + trailing
		value = value[:len(value)-1]
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "[redacted-url]" + trailing
	}
	hadQuery := parsed.RawQuery != ""
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	result := parsed.String()
	if hadQuery {
		result += "?…"
	}
	return result + trailing
}
