package service

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestCatalogCardPreviewResolvesRegisteredPDFByOpaqueItemID(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "guide.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.7\npreview fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newCatalogCardPreviewTestService(t, "item-pdf", "book", "document", path, now)

	preview, err := service.ResolvePDF(context.Background(), "item-pdf")
	if err != nil {
		t.Fatalf("resolve PDF: %v", err)
	}
	defer preview.File.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(preview.File, header); err != nil {
		t.Fatalf("read PDF: %v", err)
	}
	if string(header) != "%PDF" || preview.ETag == "" || preview.Size <= 0 ||
		preview.ModTime.IsZero() {
		t.Fatalf("unexpected PDF preview: %#v header=%q", preview, header)
	}

	if _, err := service.ResolvePDF(context.Background(), "../guide.pdf"); err != ErrCatalogCardPreviewNotFound {
		t.Fatalf("path-shaped ID error = %v", err)
	}
}

func TestCatalogLogCardPreviewReadsBoundedTailAndRedactsSecrets(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "download.log")
	prefix := strings.Repeat("old line that must stay outside the bounded tail\n", 8_000)
	content := prefix + strings.Join([]string{
		"\x1b[31mstarting final phase\x1b[0m",
		"Authorization: Bearer AUTH-CANARY",
		"Cookie: session=COOKIE-CANARY",
		"request https://user:TOKEN@example.test/private/file?sig=QUERY-CANARY#fragment",
		"api_key=KEY-CANARY",
		"download complete",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newCatalogCardPreviewTestService(t, "item-log", "other", "other", path, now)

	preview, err := service.ResolveLog(context.Background(), "item-log")
	if err != nil {
		t.Fatalf("resolve LOG: %v", err)
	}
	joined := strings.Join(preview.Lines, "\n")
	for _, forbidden := range []string{
		"\x1b", "AUTH-CANARY", "COOKIE-CANARY", "TOKEN", "QUERY-CANARY",
		"KEY-CANARY",
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("LOG preview leaked %q: %q", forbidden, joined)
		}
	}
	if !strings.Contains(joined, "https://example.test/private/file?…") ||
		!strings.Contains(joined, "download complete") ||
		len(preview.Lines) > catalogLogPreviewLineLimit ||
		!preview.Truncated || preview.ETag == "" {
		t.Fatalf("unexpected LOG preview: %#v", preview)
	}

	detail, err := service.ResolveLogText(context.Background(), "item-log")
	if err != nil {
		t.Fatalf("resolve LOG text: %v", err)
	}
	for _, forbidden := range []string{
		"\x1b", "AUTH-CANARY", "COOKIE-CANARY", "TOKEN", "QUERY-CANARY",
		"KEY-CANARY",
	} {
		if strings.Contains(detail.Text, forbidden) {
			t.Fatalf("LOG text preview leaked %q: %q", forbidden, detail.Text)
		}
	}
	if !strings.Contains(detail.Text, "download complete") ||
		!detail.Truncated || detail.ETag == "" ||
		len(detail.Text) > catalogLogDetailReadBytes {
		t.Fatalf("unexpected LOG text preview: %#v", detail)
	}
}

func TestCatalogCardPreviewRejectsMismatchedKindsAndUnavailableItems(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newCatalogCardPreviewTestService(t, "item-text", "other", "document", path, now)
	if _, err := service.ResolvePDF(context.Background(), "item-text"); err != ErrCatalogCardPreviewNotFound {
		t.Fatalf("text as PDF error = %v", err)
	}
	if _, err := service.ResolveLog(context.Background(), "item-text"); err != ErrCatalogCardPreviewNotFound {
		t.Fatalf("text as LOG error = %v", err)
	}
	if _, err := service.ResolveLogText(context.Background(), "item-text"); err != ErrCatalogCardPreviewNotFound {
		t.Fatalf("text as LOG detail error = %v", err)
	}

	item := service.items.(catalogVideoThumbnailItemRepository).items["item-text"]
	missing, err := library.NewItem(library.ItemParams{
		ID: item.ID, CatalogID: item.CatalogID, Category: string(item.Category), Status: "missing",
		Title: item.Title, Revision: item.Revision, CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.items = catalogVideoThumbnailItemRepository{items: map[string]library.Item{missing.ID: missing}}
	if _, err := service.ResolveLog(context.Background(), "item-text"); err != ErrCatalogCardPreviewNotFound {
		t.Fatalf("missing item error = %v", err)
	}
}

func TestCatalogCardPreviewAndOpenLocationShortCircuitOfflineRoot(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "offline.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.7\nstill on disk\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newCatalogCardPreviewTestService(
		t,
		"item-offline",
		"book",
		"document",
		path,
		now,
	)
	fileRepository := service.files.(catalogPreviewFileRepository)
	file := fileRepository.items["file-item-offline"]
	file.Storage.RootID = "root-offline"
	fileRepository.items[file.ID] = file
	service.files = fileRepository
	service.roots = catalogPreviewRootRepository{root: library.StorageRoot{
		ID: "root-offline", Status: library.StorageRootStatusOffline,
	}}

	if _, err := service.ResolvePDF(context.Background(), "item-offline"); err != ErrCatalogCardPreviewNotFound {
		t.Fatalf("offline PDF error = %v", err)
	}
	libraryService := &LibraryService{
		files:        fileRepository,
		storageRoots: service.roots,
	}
	if err := libraryService.OpenFileLocation(
		context.Background(),
		dto.OpenFileLocationRequest{FileID: file.ID},
	); err == nil || !strings.Contains(err.Error(), "not currently available") {
		t.Fatalf("offline open-location error = %v", err)
	}

	rootLookupErr := errors.New("storage root lookup failed")
	service.roots = catalogPreviewRootRepository{getErr: rootLookupErr}
	if _, err := service.ResolvePDF(context.Background(), "item-offline"); err != ErrCatalogCardPreviewNotFound {
		t.Fatalf("unresolved-root PDF error = %v", err)
	}
	libraryService.storageRoots = service.roots
	if err := libraryService.OpenFileLocation(
		context.Background(),
		dto.OpenFileLocationRequest{FileID: file.ID},
	); err == nil || !strings.Contains(err.Error(), "not currently available") {
		t.Fatalf("unresolved-root open-location error = %v", err)
	}
}

type catalogPreviewRootRepository struct {
	root   library.StorageRoot
	getErr error
}

func (repo catalogPreviewRootRepository) ListByCatalogID(
	context.Context,
	string,
) ([]library.StorageRoot, error) {
	return []library.StorageRoot{repo.root}, nil
}

func (repo catalogPreviewRootRepository) Get(
	context.Context,
	string,
) (library.StorageRoot, error) {
	return repo.root, repo.getErr
}

func (catalogPreviewRootRepository) Save(context.Context, library.StorageRoot) error {
	return nil
}

func (catalogPreviewRootRepository) Delete(context.Context, string) error {
	return nil
}

func newCatalogCardPreviewTestService(
	t *testing.T,
	itemID string,
	category string,
	kind string,
	path string,
	now time.Time,
) *CatalogCardPreviewService {
	t.Helper()
	file := catalogListPreviewFile(t, "file-"+itemID, kind, path, now)
	asset := catalogListPreviewAsset(t, "asset-"+itemID, itemID, file.ID, "original", now)
	item, err := library.NewItem(library.ItemParams{
		ID: itemID, CatalogID: "catalog-1", Category: category, Status: "active",
		Title: filepath.Base(path), Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewCatalogCardPreviewService(
		catalogVideoThumbnailItemRepository{items: map[string]library.Item{item.ID: item}},
		catalogPreviewAssetRepository{items: []library.ItemAsset{asset}},
		catalogPreviewFileRepository{items: map[string]library.LibraryFile{file.ID: file}},
	)
}
