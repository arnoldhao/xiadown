package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/HugoSmits86/nativewebp"
	"xiadown/internal/application/sprites/dto"
)

func TestInspectSpriteSourceReturnsReadyDraftForPNG(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "gege.png")
	writeTestSpritePNG(t, sourcePath, 1254, 1254)

	service := NewService(baseDir, nil, "", "")
	draft, err := service.InspectSpriteSource(context.Background(), dto.InspectSpriteRequest{Path: sourcePath})
	if err != nil {
		t.Fatalf("InspectSpriteSource() error = %v", err)
	}

	if draft.Status != spriteStatusReady {
		t.Fatalf("draft.Status = %q, want %q", draft.Status, spriteStatusReady)
	}
	if draft.Columns != spriteColumns || draft.Rows != spriteRows {
		t.Fatalf("draft grid = %dx%d, want %dx%d", draft.Columns, draft.Rows, spriteColumns, spriteRows)
	}
	if draft.FrameCount != spriteFrameCount {
		t.Fatalf("draft.FrameCount = %d, want %d", draft.FrameCount, spriteFrameCount)
	}
	if draft.Name != "gege" {
		t.Fatalf("draft.Name = %q, want %q", draft.Name, "gege")
	}
	if draft.PreviewPath != sourcePath {
		t.Fatalf("draft.PreviewPath = %q, want %q", draft.PreviewPath, sourcePath)
	}
}

func TestInspectSpriteSourcePrefillsManifestFromZIP(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "gege.zip")
	imagePath := filepath.Join(t.TempDir(), spriteExportFileName)
	writeTestSpritePNG(t, imagePath, 1254, 1254)

	manifest := spriteManifest{
		App:         spriteManifestAppID,
		Name:        "芙莉莲/Frieren",
		Description: "根据社区二创图，由🍌生成",
		SpriteFile:  spriteExportFileName,
		Author: spriteAuthorManifest{
			DisplayName: "he fan",
		},
		Version: "1.0",
	}
	writeTestSpriteZIP(t, zipPath, imagePath, manifest)

	service := NewService(baseDir, nil, "", "")
	draft, err := service.InspectSpriteSource(context.Background(), dto.InspectSpriteRequest{Path: zipPath})
	if err != nil {
		t.Fatalf("InspectSpriteSource() error = %v", err)
	}

	if draft.Status != spriteStatusReady {
		t.Fatalf("draft.Status = %q, want %q", draft.Status, spriteStatusReady)
	}
	if draft.Name != manifest.Name {
		t.Fatalf("draft.Name = %q, want %q", draft.Name, manifest.Name)
	}
	if draft.Description != manifest.Description {
		t.Fatalf("draft.Description = %q, want %q", draft.Description, manifest.Description)
	}
	if draft.AuthorDisplayName != manifest.Author.DisplayName {
		t.Fatalf("draft.AuthorDisplayName = %q, want %q", draft.AuthorDisplayName, manifest.Author.DisplayName)
	}
	if draft.Version != manifest.Version {
		t.Fatalf("draft.Version = %q, want %q", draft.Version, manifest.Version)
	}
	if draft.PreviewPath != "" {
		t.Fatalf("draft.PreviewPath = %q, want empty for zip source", draft.PreviewPath)
	}
}

func TestInspectSpriteSourceRejectsDirectorySpriteFileOutsideDirectory(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourceRoot := t.TempDir()
	spriteDir := filepath.Join(sourceRoot, "sprite")
	if err := os.MkdirAll(spriteDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	outsidePath := filepath.Join(sourceRoot, "outside.png")
	writeTestSpritePNG(t, outsidePath, 1254, 1254)
	writeTestManifest(t, filepath.Join(spriteDir, spriteManifestFileName), spriteManifest{
		App:        spriteManifestAppID,
		SpriteFile: filepath.Join("..", "outside.png"),
	})

	service := NewService(baseDir, nil, "", "")
	_, err := service.InspectSpriteSource(context.Background(), dto.InspectSpriteRequest{Path: spriteDir})
	if err == nil {
		t.Fatalf("InspectSpriteSource() error = nil, want path escape error")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("InspectSpriteSource() error = %q, want path escape error", err.Error())
	}
}

func TestImportSpriteCreatesSpritePNGAndManifest(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "gege.png")
	writeTestSpritePNG(t, sourcePath, 1254, 1254)

	service := NewService(baseDir, nil, "", "")
	sprite, err := service.ImportSprite(context.Background(), dto.ImportSpriteRequest{
		Path:              sourcePath,
		Name:              "gege",
		Description:       "builtin sprite",
		AuthorDisplayName: "tester",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("ImportSprite() error = %v", err)
	}

	if sprite.Status != spriteStatusReady {
		t.Fatalf("sprite.Status = %q, want %q", sprite.Status, spriteStatusReady)
	}
	if sprite.ImageWidth != 1254 || sprite.ImageHeight != 1254 {
		t.Fatalf("sprite image = %dx%d, want 1254x1254", sprite.ImageWidth, sprite.ImageHeight)
	}
	if filepath.Base(sprite.SpritePath) != spriteExportFileName {
		t.Fatalf("sprite.SpritePath = %q, want sprite.png path", sprite.SpritePath)
	}

	spriteDir := filepath.Join(baseDir, scopeImported, sprite.ID)
	assertPNGSize(t, filepath.Join(spriteDir, spriteExportFileName), 1254, 1254)
	if _, err := os.Stat(filepath.Join(spriteDir, "original.png")); !os.IsNotExist(err) {
		t.Fatalf("legacy original.png exists after import, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(spriteDir, "runtime.png")); !os.IsNotExist(err) {
		t.Fatalf("legacy runtime.png exists after import, err = %v", err)
	}

	manifestData, err := os.ReadFile(filepath.Join(spriteDir, spriteManifestFileName))
	if err != nil {
		t.Fatalf("ReadFile(manifest) error = %v", err)
	}
	var manifest spriteManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("json.Unmarshal(manifest) error = %v", err)
	}
	if manifest.App != spriteManifestAppID {
		t.Fatalf("manifest.App = %q, want %q", manifest.App, spriteManifestAppID)
	}
	if manifest.SpriteFile != spriteExportFileName {
		t.Fatalf("manifest.SpriteFile = %q, want %q", manifest.SpriteFile, spriteExportFileName)
	}
	if bytes.Contains(manifestData, []byte("frameWidth")) || bytes.Contains(manifestData, []byte("frameHeight")) {
		t.Fatalf("manifest contains deprecated frame size fields: %s", manifestData)
	}
	if len(manifest.SliceGrid.X) != spriteColumns+1 || len(manifest.SliceGrid.Y) != spriteRows+1 {
		t.Fatalf("manifest slice grid lengths = %dx%d, want %dx%d", len(manifest.SliceGrid.X), len(manifest.SliceGrid.Y), spriteColumns+1, spriteRows+1)
	}
	if manifest.SliceGrid.X[0] != 0 || manifest.SliceGrid.X[len(manifest.SliceGrid.X)-1] != 1254 {
		t.Fatalf("manifest slice x = %v, want source 0..1254", manifest.SliceGrid.X)
	}

	storedImage := readPNG(t, filepath.Join(spriteDir, spriteExportFileName))
	if got := storedImage.NRGBAAt(0, 0); got.R != 255 || got.G != 0 || got.B != 255 || got.A != 255 {
		t.Fatalf("stored pixel at 0,0 = %#v, want original magenta background", got)
	}
	if got := storedImage.NRGBAAt(1254/2, 1254/2).A; got == 0 {
		t.Fatalf("stored alpha at center = %d, want opaque content", got)
	}
}

func TestUpdateSpriteSlicesWritesManifestWithoutGeneratingRuntime(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "sliced.png")
	writeStripedSpritePNG(t, sourcePath, 1024, 1024)

	service := NewService(baseDir, nil, "", "")
	sprite, err := service.ImportSprite(context.Background(), dto.ImportSpriteRequest{
		Path:              sourcePath,
		Name:              "sliced",
		Description:       "custom grid",
		AuthorDisplayName: "tester",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("ImportSprite() error = %v", err)
	}

	grid := dto.SpriteSliceGrid{
		X: []int{0, 64, 192, 320, 448, 576, 704, 832, 1024},
		Y: []int{0, 128, 256, 384, 512, 640, 768, 896, 1024},
	}
	updated, err := service.UpdateSpriteSlices(context.Background(), dto.UpdateSpriteSlicesRequest{
		ID:        sprite.ID,
		SliceGrid: grid,
	})
	if err != nil {
		t.Fatalf("UpdateSpriteSlices() error = %v", err)
	}
	if updated.UpdatedAt == "" {
		t.Fatalf("updated.UpdatedAt is empty")
	}

	manifest, err := service.GetSpriteManifest(context.Background(), dto.GetSpriteManifestRequest{ID: sprite.ID})
	if err != nil {
		t.Fatalf("GetSpriteManifest() error = %v", err)
	}
	if manifest.SheetWidth != 1024 || manifest.SheetHeight != 1024 {
		t.Fatalf("manifest sheet = %dx%d, want 1024x1024", manifest.SheetWidth, manifest.SheetHeight)
	}
	if got := manifest.SliceGrid.X[1]; got != 64 {
		t.Fatalf("manifest.SliceGrid.X[1] = %d, want 64", got)
	}

	spriteDir := filepath.Join(baseDir, scopeImported, sprite.ID)
	assertPNGSize(t, filepath.Join(spriteDir, spriteExportFileName), 1024, 1024)
	if _, err := os.Stat(filepath.Join(spriteDir, "runtime.png")); !os.IsNotExist(err) {
		t.Fatalf("legacy runtime.png exists after slice update, err = %v", err)
	}
}

func TestImportSpritePreservesNearMagentaSourcePixels(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "near-magenta.png")
	writeTestSpritePNGWithBackground(t, sourcePath, 1254, 1254, color.NRGBA{R: 225, G: 12, B: 228, A: 255})

	service := NewService(baseDir, nil, "", "")
	sprite, err := service.ImportSprite(context.Background(), dto.ImportSpriteRequest{
		Path:              sourcePath,
		Name:              "near-magenta",
		Description:       "near magenta sprite",
		AuthorDisplayName: "tester",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("ImportSprite() error = %v", err)
	}

	storedImage := readPNG(t, filepath.Join(baseDir, scopeImported, sprite.ID, spriteExportFileName))
	if got := storedImage.NRGBAAt(0, 0); got.R != 225 || got.G != 12 || got.B != 228 || got.A != 255 {
		t.Fatalf("stored pixel at 0,0 = %#v, want original near-magenta background", got)
	}
	if got := storedImage.NRGBAAt(1254/2, 1254/2).A; got == 0 {
		t.Fatalf("stored alpha at center = %d, want opaque content", got)
	}
}

func TestImportSpritePreservesInteriorMagentaKeySourcePixels(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "interior-magenta.png")
	writeTestSpritePNGWithInteriorMagentaKey(t, sourcePath, 1254, 1254)

	service := NewService(baseDir, nil, "", "")
	sprite, err := service.ImportSprite(context.Background(), dto.ImportSpriteRequest{
		Path:              sourcePath,
		Name:              "interior-magenta",
		Description:       "interior magenta sprite",
		AuthorDisplayName: "tester",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("ImportSprite() error = %v", err)
	}

	storedImage := readPNG(t, filepath.Join(baseDir, scopeImported, sprite.ID, spriteExportFileName))
	if got := storedImage.NRGBAAt(0, 0); got.R != 255 || got.G != 0 || got.B != 255 || got.A != 255 {
		t.Fatalf("stored pixel at edge = %#v, want original magenta background", got)
	}
	frameSize := 1254 / spriteColumns
	if got := storedImage.NRGBAAt(frameSize/2, frameSize/2); got.R != 255 || got.G != 0 || got.B != 255 || got.A != 255 {
		t.Fatalf("stored pixel at interior magenta key = %#v, want original magenta key", got)
	}
	if got := storedImage.NRGBAAt(frameSize/4, frameSize/2).A; got == 0 {
		t.Fatalf("stored alpha at sprite content = %d, want opaque", got)
	}
}

func TestExportSpriteWritesManifestJSONSpritePNGAndPreviewWebP(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "gege.png")
	writeTestSpritePNG(t, sourcePath, 1254, 1254)

	service := NewService(baseDir, nil, "", "")
	sprite, err := service.ImportSprite(context.Background(), dto.ImportSpriteRequest{
		Path:              sourcePath,
		Name:              "gege",
		Description:       "builtin sprite",
		AuthorDisplayName: "tester",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("ImportSprite() error = %v", err)
	}

	exportPath := filepath.Join(t.TempDir(), "gege-export")
	if err := service.ExportSprite(context.Background(), dto.ExportSpriteRequest{
		ID:         sprite.ID,
		OutputPath: exportPath,
	}); err != nil {
		t.Fatalf("ExportSprite() error = %v", err)
	}

	archive, err := zip.OpenReader(exportPath + ".zip")
	if err != nil {
		t.Fatalf("zip.OpenReader() error = %v", err)
	}
	defer archive.Close()

	names := make([]string, 0, len(archive.File))
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	sort.Strings(names)
	expectedNames := []string{spriteManifestFileName, spritePreviewFileName, spriteExportFileName}
	sort.Strings(expectedNames)
	if len(names) != len(expectedNames) || strings.Join(names, ",") != strings.Join(expectedNames, ",") {
		t.Fatalf("archive files = %v, want %v", names, expectedNames)
	}

	for _, file := range archive.File {
		switch file.Name {
		case spriteManifestFileName:
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("manifest.Open() error = %v", err)
			}
			defer rc.Close()
			var manifest spriteManifest
			if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
				t.Fatalf("Decode(manifest) error = %v", err)
			}
			if manifest.App != spriteManifestAppID {
				t.Fatalf("manifest.App = %q, want %q", manifest.App, spriteManifestAppID)
			}
		case spriteExportFileName:
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("sprite.Open() error = %v", err)
			}
			defer rc.Close()
			img, err := png.Decode(rc)
			if err != nil {
				t.Fatalf("png.Decode() error = %v", err)
			}
			if img.Bounds().Dx() != 1254 || img.Bounds().Dy() != 1254 {
				t.Fatalf("exported sprite size = %dx%d, want 1254x1254", img.Bounds().Dx(), img.Bounds().Dy())
			}
		case spritePreviewFileName:
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("preview.Open() error = %v", err)
			}
			defer rc.Close()
			img, err := nativewebp.Decode(rc)
			if err != nil {
				t.Fatalf("nativewebp.Decode() error = %v", err)
			}
			if img.Bounds().Dx() != spritePreviewSize || img.Bounds().Dy() != spritePreviewSize {
				t.Fatalf("preview size = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), spritePreviewSize, spritePreviewSize)
			}
		}
	}
}

func TestInstallSpriteFromURLDownloadsVerifiesAndImports(t *testing.T) {
	t.Parallel()

	sourcePath := filepath.Join(t.TempDir(), "gege.png")
	writeTestSpritePNG(t, sourcePath, 1254, 1254)
	zipPath := filepath.Join(t.TempDir(), "gege.zip")
	writeTestSpriteZIP(t, zipPath, sourcePath, spriteManifest{
		App:        spriteManifestAppID,
		Name:       "Remote Gege",
		SpriteFile: spriteExportFileName,
		Version:    "1.0",
	})
	payload, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatalf("ReadFile(zip) error = %v", err)
	}
	checksum := sha256.Sum256(payload)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/gege.zip" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	service := NewService(t.TempDir(), nil, "", "", WithHTTPClient(server.Client()))
	sprite, err := service.InstallSpriteFromURL(context.Background(), dto.InstallSpriteFromURLRequest{
		URL:               server.URL + "/gege.zip",
		SHA256:            hex.EncodeToString(checksum[:]),
		Size:              int64(len(payload)),
		Name:              "Catalog Gege",
		Description:       "from catalog",
		AuthorDisplayName: "catalog",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("InstallSpriteFromURL() error = %v", err)
	}
	if sprite.Name != "Catalog Gege" || sprite.Author.DisplayName != "catalog" {
		t.Fatalf("sprite metadata = %q/%q, want catalog metadata", sprite.Name, sprite.Author.DisplayName)
	}
	if sprite.Origin != "online" {
		t.Fatalf("sprite.Origin = %q, want online", sprite.Origin)
	}
	if _, err := os.Stat(sprite.SpritePath); err != nil {
		t.Fatalf("installed sprite image stat error = %v", err)
	}
}

func TestInstallSpriteFromURLRejectsHTTP(t *testing.T) {
	t.Parallel()

	service := NewService(t.TempDir(), nil, "", "")
	_, err := service.InstallSpriteFromURL(context.Background(), dto.InstallSpriteFromURLRequest{
		URL: "http://example.test/gege.zip",
	})
	if err == nil {
		t.Fatalf("InstallSpriteFromURL() error = nil, want https error")
	}
	if !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("InstallSpriteFromURL() error = %q, want https error", err.Error())
	}
}

func TestInstallSpriteFromURLRequiresChecksum(t *testing.T) {
	t.Parallel()

	service := NewService(t.TempDir(), nil, "", "")
	_, err := service.InstallSpriteFromURL(context.Background(), dto.InstallSpriteFromURLRequest{
		URL: "https://example.test/gege.zip",
	})
	if err == nil {
		t.Fatalf("InstallSpriteFromURL() error = nil, want checksum error")
	}
	if !strings.Contains(err.Error(), "checksum is required") {
		t.Fatalf("InstallSpriteFromURL() error = %q, want checksum error", err.Error())
	}
}

func TestDeleteSpriteRemovesImportedDirectory(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "gege.png")
	writeTestSpritePNG(t, sourcePath, 1254, 1254)

	service := NewService(baseDir, nil, "", "")
	sprite, err := service.ImportSprite(context.Background(), dto.ImportSpriteRequest{
		Path:              sourcePath,
		Name:              "gege",
		Description:       "imported sprite",
		AuthorDisplayName: "tester",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("ImportSprite() error = %v", err)
	}

	spriteDir := filepath.Join(baseDir, scopeImported, sprite.ID)
	if err := service.DeleteSprite(context.Background(), dto.DeleteSpriteRequest{ID: sprite.ID}); err != nil {
		t.Fatalf("DeleteSprite() error = %v", err)
	}
	if _, err := os.Stat(spriteDir); !os.IsNotExist(err) {
		t.Fatalf("sprite dir still exists after delete, err = %v", err)
	}

	sprites, err := service.ListSprites(context.Background())
	if err != nil {
		t.Fatalf("ListSprites() error = %v", err)
	}
	for _, listed := range sprites {
		if listed.ID == sprite.ID {
			t.Fatalf("deleted sprite %q still returned by ListSprites()", sprite.ID)
		}
	}
}

func TestListSpritesPrimesMetadataRepositoryAndReturnsCover(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "gege.png")
	writeTestSpritePNG(t, sourcePath, 1254, 1254)

	writer := NewService(baseDir, nil, "", "")
	imported, err := writer.ImportSprite(context.Background(), dto.ImportSpriteRequest{
		Path:              sourcePath,
		Name:              "gege",
		Description:       "imported sprite",
		AuthorDisplayName: "tester",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("ImportSprite() error = %v", err)
	}

	repo := newSpriteMetadataRepoStub()
	service := NewService(baseDir, nil, "", "", WithMetadataRepository(repo))
	sprites, err := service.ListSprites(context.Background())
	if err != nil {
		t.Fatalf("ListSprites() error = %v", err)
	}
	if len(sprites) != 1 {
		t.Fatalf("len(sprites) = %d, want 1", len(sprites))
	}
	if sprites[0].ID != imported.ID {
		t.Fatalf("sprites[0].ID = %q, want %q", sprites[0].ID, imported.ID)
	}
	if sprites[0].CoverImageDataURL == "" {
		t.Fatalf("sprites[0].CoverImageDataURL = empty, want cached cover")
	}
	if len(repo.items) != 1 {
		t.Fatalf("len(repo.items) = %d, want 1", len(repo.items))
	}
	if len(repo.covers[imported.ID]) == 0 {
		t.Fatalf("repo.covers[%q] = empty, want encoded cover", imported.ID)
	}
	coverImage := readPNGFromBytes(t, repo.covers[imported.ID])
	if got := coverImage.NRGBAAt(0, 0).A; got != 0 {
		t.Fatalf("cover alpha at 0,0 = %d, want transparent background", got)
	}
	if !strings.HasPrefix(sprites[0].CoverImageDataURL, "data:image/png;base64,") {
		t.Fatalf("sprites[0].CoverImageDataURL = %q, want png data URL", sprites[0].CoverImageDataURL)
	}
}

func TestDeleteSpriteRemovesMetadataCacheEntry(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "gege.png")
	writeTestSpritePNG(t, sourcePath, 1254, 1254)

	repo := newSpriteMetadataRepoStub()
	service := NewService(baseDir, nil, "", "", WithMetadataRepository(repo))
	sprite, err := service.ImportSprite(context.Background(), dto.ImportSpriteRequest{
		Path:              sourcePath,
		Name:              "gege",
		Description:       "imported sprite",
		AuthorDisplayName: "tester",
		Version:           "1.0",
	})
	if err != nil {
		t.Fatalf("ImportSprite() error = %v", err)
	}
	if _, ok := repo.items[sprite.ID]; !ok {
		t.Fatalf("repo.items missing imported sprite %q", sprite.ID)
	}

	if err := service.DeleteSprite(context.Background(), dto.DeleteSpriteRequest{ID: sprite.ID}); err != nil {
		t.Fatalf("DeleteSprite() error = %v", err)
	}
	if _, ok := repo.items[sprite.ID]; ok {
		t.Fatalf("repo.items still contains deleted sprite %q", sprite.ID)
	}
}

func TestEnsureBuiltinSpritesUsesEmbeddedDirectoryStructure(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	embeddedRoot := filepath.Join(t.TempDir(), "embedded")
	spriteDir := filepath.Join(embeddedRoot, "gege")
	if err := os.MkdirAll(spriteDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	spritePath := filepath.Join(spriteDir, spriteExportFileName)
	writeTestSpritePNG(t, spritePath, 1254, 1254)
	writeTestManifest(t, filepath.Join(spriteDir, spriteManifestFileName), spriteManifest{
		App:         spriteManifestAppID,
		Name:        "gege",
		SpriteFile:  spriteExportFileName,
		Description: "embedded sprite",
		Version:     "1.0",
	})

	service := NewService(baseDir, os.DirFS(embeddedRoot), ".", "")
	if err := service.EnsureBuiltinSprites(context.Background()); err != nil {
		t.Fatalf("EnsureBuiltinSprites() error = %v", err)
	}

	sprites, err := service.ListSprites(context.Background())
	if err != nil {
		t.Fatalf("ListSprites() error = %v", err)
	}
	if len(sprites) != 1 {
		t.Fatalf("len(sprites) = %d, want 1", len(sprites))
	}

	sprite := sprites[0]
	if sprite.Scope != scopeBuiltin {
		t.Fatalf("sprite.Scope = %q, want %q", sprite.Scope, scopeBuiltin)
	}
	if sprite.Status != spriteStatusReady {
		t.Fatalf("sprite.Status = %q, want %q", sprite.Status, spriteStatusReady)
	}
	if sprite.Rows != spriteRows || sprite.Columns != spriteColumns {
		t.Fatalf("sprite grid = %dx%d, want %dx%d", sprite.Columns, sprite.Rows, spriteColumns, spriteRows)
	}
	if sprite.FrameCount != spriteFrameCount {
		t.Fatalf("sprite.FrameCount = %d, want %d", sprite.FrameCount, spriteFrameCount)
	}
	spriteStorageDir := filepath.Join(baseDir, scopeBuiltin, sprite.ID)
	assertPNGSize(t, filepath.Join(spriteStorageDir, spriteExportFileName), 1254, 1254)
	if _, err := os.Stat(filepath.Join(spriteStorageDir, "original.png")); !os.IsNotExist(err) {
		t.Fatalf("legacy original.png exists after builtin sync, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(spriteStorageDir, "runtime.png")); !os.IsNotExist(err) {
		t.Fatalf("legacy runtime.png exists after builtin sync, err = %v", err)
	}
}

func TestListSpritesReadsStoredSpritePNGWithoutRuntime(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	spriteID := "stored-sprite"
	spriteDir := filepath.Join(baseDir, scopeImported, spriteID)
	if err := os.MkdirAll(spriteDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	writeTestSpritePNGWithBackground(t, filepath.Join(spriteDir, spriteExportFileName), 1254, 1254, color.NRGBA{R: 225, G: 12, B: 228, A: 255})
	writeTestManifest(t, filepath.Join(spriteDir, spriteManifestFileName), spriteManifest{
		App:         spriteManifestAppID,
		ID:          spriteID,
		Name:        "stored sprite",
		SpriteFile:  spriteExportFileName,
		Description: "stored sprite",
		Version:     "1.0",
	})

	service := NewService(baseDir, nil, "", "")
	sprites, err := service.ListSprites(context.Background())
	if err != nil {
		t.Fatalf("ListSprites() error = %v", err)
	}
	if len(sprites) != 1 {
		t.Fatalf("len(sprites) = %d, want 1", len(sprites))
	}
	if sprites[0].SpriteFile != spriteExportFileName {
		t.Fatalf("sprites[0].SpriteFile = %q, want %q", sprites[0].SpriteFile, spriteExportFileName)
	}
	if sprites[0].SpritePath != filepath.Join(spriteDir, spriteExportFileName) {
		t.Fatalf("sprites[0].SpritePath = %q, want stored sprite path", sprites[0].SpritePath)
	}
}

func TestInspectSpriteSourceRejectsNonSquareSheet(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "not-square.png")
	writeTestSpritePNG(t, sourcePath, 1254, 1200)

	service := NewService(baseDir, nil, "", "")
	draft, err := service.InspectSpriteSource(context.Background(), dto.InspectSpriteRequest{Path: sourcePath})
	if err != nil {
		t.Fatalf("InspectSpriteSource() error = %v", err)
	}

	if draft.Status != spriteStatusInvalid {
		t.Fatalf("draft.Status = %q, want %q", draft.Status, spriteStatusInvalid)
	}
	if draft.ValidationMessage == "" {
		t.Fatalf("draft.ValidationMessage = empty, want validation error")
	}
	if !strings.Contains(draft.ValidationMessage, "square") {
		t.Fatalf("draft.ValidationMessage = %q, want square validation error", draft.ValidationMessage)
	}
}

func TestInspectSpriteSourceRejectsSheetLargerThanMaximum(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "too-large.png")
	writeTestSpritePNG(t, sourcePath, spriteMaxWidth+1, spriteMaxHeight+1)

	service := NewService(baseDir, nil, "", "")
	draft, err := service.InspectSpriteSource(context.Background(), dto.InspectSpriteRequest{Path: sourcePath})
	if err != nil {
		t.Fatalf("InspectSpriteSource() error = %v", err)
	}

	if draft.Status != spriteStatusInvalid {
		t.Fatalf("draft.Status = %q, want %q", draft.Status, spriteStatusInvalid)
	}
	if draft.ValidationMessage == "" {
		t.Fatalf("draft.ValidationMessage = empty, want validation error")
	}
}

func TestInspectSpriteSourceRejectsSheetSmallerThanMinimum(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "too-small.png")
	writeTestSpritePNG(t, sourcePath, spriteMinWidth-1, spriteMinHeight-1)

	service := NewService(baseDir, nil, "", "")
	draft, err := service.InspectSpriteSource(context.Background(), dto.InspectSpriteRequest{Path: sourcePath})
	if err != nil {
		t.Fatalf("InspectSpriteSource() error = %v", err)
	}

	if draft.Status != spriteStatusInvalid {
		t.Fatalf("draft.Status = %q, want %q", draft.Status, spriteStatusInvalid)
	}
	if draft.ValidationMessage == "" {
		t.Fatalf("draft.ValidationMessage = empty, want validation error")
	}
}

func TestInspectSpriteSourceRejectsArchiveLargerThanLimit(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "too-large.zip")
	if err := os.WriteFile(zipPath, make([]byte, spriteMaxZipSizeBytes+1), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	service := NewService(baseDir, nil, "", "")
	_, err := service.InspectSpriteSource(context.Background(), dto.InspectSpriteRequest{Path: zipPath})
	if err == nil {
		t.Fatalf("InspectSpriteSource() error = nil, want archive size error")
	}
	if !strings.Contains(err.Error(), "15MB") {
		t.Fatalf("InspectSpriteSource() error = %q, want 15MB limit", err.Error())
	}
}

func TestInspectSpriteSourceRejectsArchiveContentsLargerThanLimit(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "compressed-too-large.zip")
	writeLargeCompressedSpriteZIP(t, zipPath, spriteMaxZipSizeBytes+1)

	service := NewService(baseDir, nil, "", "")
	_, err := service.InspectSpriteSource(context.Background(), dto.InspectSpriteRequest{Path: zipPath})
	if err == nil {
		t.Fatalf("InspectSpriteSource() error = nil, want archive content size error")
	}
	if !strings.Contains(err.Error(), "15MB") {
		t.Fatalf("InspectSpriteSource() error = %q, want 15MB limit", err.Error())
	}
}

func writeTestSpritePNG(t *testing.T, path string, width int, height int) {
	t.Helper()

	writeTestSpritePNGWithBackground(t, path, width, height, color.NRGBA{R: 255, G: 0, B: 255, A: 255})
}

func writeTestSpritePNGWithBackground(t *testing.T, path string, width int, height int, background color.NRGBA) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	content := color.NRGBA{R: 32, G: 48, B: 64, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, background)
		}
	}

	boxWidth := maxInt(width/4, 32)
	boxHeight := maxInt(height/4, 32)
	startX := (width - boxWidth) / 2
	startY := (height - boxHeight) / 2
	for y := startY; y < startY+boxHeight; y++ {
		for x := startX; x < startX+boxWidth; x++ {
			img.SetNRGBA(x, y, content)
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
}

func writeTestSpritePNGWithInteriorMagentaKey(t *testing.T, path string, width int, height int) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	magenta := color.NRGBA{R: 255, G: 0, B: 255, A: 255}
	content := color.NRGBA{R: 32, G: 48, B: 64, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, magenta)
		}
	}

	frameWidth := maxInt(width/spriteColumns, 1)
	frameHeight := maxInt(height/spriteRows, 1)
	contentLeft := frameWidth / 8
	contentTop := frameHeight / 8
	contentRight := frameWidth - contentLeft
	contentBottom := frameHeight - contentTop
	for y := contentTop; y < contentBottom; y++ {
		for x := contentLeft; x < contentRight; x++ {
			img.SetNRGBA(x, y, content)
		}
	}

	holeLeft := (frameWidth * 2) / 5
	holeTop := (frameHeight * 2) / 5
	holeRight := (frameWidth * 3) / 5
	holeBottom := (frameHeight * 3) / 5
	for y := holeTop; y < holeBottom; y++ {
		for x := holeLeft; x < holeRight; x++ {
			img.SetNRGBA(x, y, magenta)
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
}

func writeStripedSpritePNG(t *testing.T, path string, width int, height int) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x < 64 {
				img.SetNRGBA(x, y, color.NRGBA{R: 240, G: 32, B: 32, A: 255})
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{R: 32, G: 180, B: 80, A: 255})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
}

type spriteMetadataRepoStub struct {
	items  map[string]dto.Sprite
	covers map[string][]byte
}

func newSpriteMetadataRepoStub() *spriteMetadataRepoStub {
	return &spriteMetadataRepoStub{
		items:  make(map[string]dto.Sprite),
		covers: make(map[string][]byte),
	}
}

func (repo *spriteMetadataRepoStub) List(_ context.Context) ([]dto.Sprite, error) {
	result := make([]dto.Sprite, 0, len(repo.items))
	for id, sprite := range repo.items {
		cloned := sprite
		cloned.CoverImageDataURL = coverPNGToDataURL(repo.covers[id])
		result = append(result, cloned)
	}
	return result, nil
}

func (repo *spriteMetadataRepoStub) Save(_ context.Context, sprite dto.Sprite, coverPNG []byte) error {
	cloned := sprite
	cloned.CoverImageDataURL = ""
	repo.items[sprite.ID] = cloned
	repo.covers[sprite.ID] = append([]byte(nil), coverPNG...)
	return nil
}

func (repo *spriteMetadataRepoStub) Delete(_ context.Context, id string) error {
	delete(repo.items, id)
	delete(repo.covers, id)
	return nil
}

func writeTestSpriteZIP(t *testing.T, zipPath string, imagePath string, manifest spriteManifest) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	manifestEntry, err := writer.Create(spriteManifestFileName)
	if err != nil {
		t.Fatalf("writer.Create(manifest) error = %v", err)
	}
	if err := json.NewEncoder(manifestEntry).Encode(manifest); err != nil {
		t.Fatalf("Encode(manifest) error = %v", err)
	}

	imageEntry, err := writer.Create(spriteExportFileName)
	if err != nil {
		t.Fatalf("writer.Create(image) error = %v", err)
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if _, err := imageEntry.Write(imageData); err != nil {
		t.Fatalf("Write(image) error = %v", err)
	}
}

func writeLargeCompressedSpriteZIP(t *testing.T, zipPath string, imageSize int) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	manifestEntry, err := writer.Create(spriteManifestFileName)
	if err != nil {
		t.Fatalf("writer.Create(manifest) error = %v", err)
	}
	if err := json.NewEncoder(manifestEntry).Encode(spriteManifest{
		App:        spriteManifestAppID,
		SpriteFile: spriteExportFileName,
	}); err != nil {
		t.Fatalf("Encode(manifest) error = %v", err)
	}

	imageEntry, err := writer.Create(spriteExportFileName)
	if err != nil {
		t.Fatalf("writer.Create(image) error = %v", err)
	}
	if _, err := imageEntry.Write(bytes.Repeat([]byte{0}, imageSize)); err != nil {
		t.Fatalf("Write(image) error = %v", err)
	}
}

func writeTestManifest(t *testing.T, path string, manifest spriteManifest) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func assertPNGSize(t *testing.T, path string, width int, height int) {
	t.Helper()

	img := readPNG(t, path)
	if img.Bounds().Dx() != width || img.Bounds().Dy() != height {
		t.Fatalf("%s size = %dx%d, want %dx%d", path, img.Bounds().Dx(), img.Bounds().Dy(), width, height)
	}
}

func readPNG(t *testing.T, path string) *image.NRGBA {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(%s) error = %v", path, err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("png.Decode(%s) error = %v", path, err)
	}

	bounds := img.Bounds()
	result := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			result.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return result
}

func readPNGFromBytes(t *testing.T, payload []byte) *image.NRGBA {
	t.Helper()

	img, err := png.Decode(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("png.Decode(bytes) error = %v", err)
	}

	bounds := img.Bounds()
	result := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			result.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return result
}
