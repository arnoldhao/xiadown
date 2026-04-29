package service

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	urlpkg "net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"xiadown/internal/application/sprites/dto"
)

const (
	spriteColumns            = 8
	spriteRows               = 8
	spriteFrameCount         = spriteColumns * spriteRows
	spriteMinWidth           = 1024
	spriteMinHeight          = 1024
	spriteMaxWidth           = 5120
	spriteMaxHeight          = 5120
	spriteMaxZipSizeBytes    = 15 * 1024 * 1024
	spritePreviewSize        = 256
	spriteManifestAppID      = "cc.dreamapp.xiadown"
	spriteManifestFileName   = "manifest.json"
	spriteExportFileName     = "sprite.png"
	spritePreviewFileName    = "preview.webp"
	spriteMagentaKeyRedMin   = 250
	spriteMagentaKeyGreenMax = 5
	spriteMagentaKeyBlueMin  = 250

	scopeBuiltin  = "builtin"
	scopeImported = "imported"

	spriteStatusReady   = "ready"
	spriteStatusInvalid = "invalid"
)

var (
	builtinSpriteNamespace = uuid.MustParse("314d8d8f-8da0-4d2a-8c9f-9d81289b01bf")
	backgroundSamplePoints = [][2]float64{
		{0, 0},
		{0.5, 0},
		{1, 0},
		{0, 0.5},
		{1, 0.5},
		{0, 1},
		{0.5, 1},
		{1, 1},
	}
)

type backgroundColor struct {
	r uint8
	g uint8
	b uint8
}

type spriteManifest struct {
	App         string               `json:"app"`
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	FrameCount  int                  `json:"frameCount"`
	Columns     int                  `json:"columns"`
	Rows        int                  `json:"rows"`
	SpriteFile  string               `json:"spriteFile"`
	SourceType  string               `json:"sourceType,omitempty"`
	Origin      string               `json:"origin,omitempty"`
	SliceGrid   spriteSliceGrid      `json:"sliceGrid,omitempty"`
	Author      spriteAuthorManifest `json:"author"`
	CreatedAt   string               `json:"createdAt"`
	Version     string               `json:"version"`
}

type spriteSliceGrid struct {
	X []int `json:"x,omitempty"`
	Y []int `json:"y,omitempty"`
}

type spriteAuthorManifest struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type importedSource struct {
	imagePath  string
	sourceName string
	manifest   spriteManifest
}

type extractedImageFile struct {
	path string
	name string
}

type spriteDimensionValidation struct {
	sheetWidth        int
	sheetHeight       int
	validationMessage string
}

type spriteImportInspection struct {
	sourcePath        string
	sourceType        string
	previewPath       string
	imageWidth        int
	imageHeight       int
	sheetWidth        int
	sheetHeight       int
	sliceGrid         spriteSliceGrid
	validationMessage string
	source            importedSource
}

type fileFingerprint struct {
	size    int64
	modTime time.Time
}

type MetadataRepository interface {
	List(ctx context.Context) ([]dto.Sprite, error)
	Save(ctx context.Context, sprite dto.Sprite, coverPNG []byte) error
	Delete(ctx context.Context, id string) error
}

type Option func(*Service)

type Service struct {
	mu             sync.Mutex
	baseDir        string
	builtinFS      fs.FS
	builtinRoot    string
	devBuiltinDir  string
	metadataRepo   MetadataRepository
	metadataPrimed bool
	builtinEnsured bool
	httpClient     *http.Client
	now            func() time.Time
}

func NewService(baseDir string, builtinFS fs.FS, builtinRoot string, devBuiltinDir string, options ...Option) *Service {
	service := &Service{
		baseDir:       strings.TrimSpace(baseDir),
		builtinFS:     builtinFS,
		builtinRoot:   strings.Trim(strings.TrimSpace(builtinRoot), "/"),
		devBuiltinDir: strings.TrimSpace(devBuiltinDir),
		httpClient:    &http.Client{Timeout: 90 * time.Second},
		now:           time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func WithMetadataRepository(repo MetadataRepository) Option {
	return func(service *Service) {
		service.metadataRepo = repo
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(service *Service) {
		if client != nil {
			service.httpClient = client
		}
	}
}

func DefaultSpritesBaseDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(configDir, "xiadown", "sprites"), nil
}

func (service *Service) EnsureBuiltinSprites(_ context.Context) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.ensureBuiltinSpritesLocked(); err != nil {
		return err
	}
	service.builtinEnsured = true
	return nil
}

func (service *Service) ListSprites(ctx context.Context) ([]dto.Sprite, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	if !service.builtinEnsured {
		if err := service.ensureBuiltinSpritesLocked(); err != nil {
			return nil, err
		}
		service.builtinEnsured = true
	}

	if service.metadataRepo != nil {
		if !service.metadataPrimed {
			if err := service.primeMetadataLocked(); err != nil {
				return nil, err
			}
		}
		if service.metadataPrimed {
			sprites, err := service.metadataRepo.List(ctx)
			if err == nil {
				sortSprites(sprites)
				return sprites, nil
			}
			service.metadataPrimed = false
		}
	}

	builtinSprites, err := service.listScopeLocked(scopeBuiltin, service.builtinDir())
	if err != nil {
		return nil, err
	}
	importedSprites, err := service.listScopeLocked(scopeImported, service.importedDir())
	if err != nil {
		return nil, err
	}

	sprites := append(builtinSprites, importedSprites...)
	sortSprites(sprites)
	return sprites, nil
}

func (service *Service) primeMetadataLocked() error {
	if service.metadataRepo == nil {
		service.metadataPrimed = false
		return nil
	}

	builtinSprites, err := service.listScopeLocked(scopeBuiltin, service.builtinDir())
	if err != nil {
		return err
	}
	importedSprites, err := service.listScopeLocked(scopeImported, service.importedDir())
	if err != nil {
		return err
	}

	sprites := append(builtinSprites, importedSprites...)
	syncedIDs := make(map[string]struct{}, len(sprites))
	cacheHealthy := true
	for _, sprite := range sprites {
		if strings.TrimSpace(sprite.ID) != "" {
			syncedIDs[sprite.ID] = struct{}{}
		}
		if _, ok := service.syncSpriteMetadataLocked(sprite); !ok {
			cacheHealthy = false
		}
	}
	if !cacheHealthy {
		service.metadataPrimed = false
		return nil
	}

	cachedSprites, err := service.metadataRepo.List(context.Background())
	if err != nil {
		service.metadataPrimed = false
		return nil
	}
	for _, sprite := range cachedSprites {
		if _, ok := syncedIDs[sprite.ID]; ok {
			continue
		}
		if !service.deleteSpriteMetadataLocked(sprite.ID) {
			service.metadataPrimed = false
			return nil
		}
	}

	service.metadataPrimed = true
	return nil
}

func sortSprites(sprites []dto.Sprite) {
	sort.SliceStable(sprites, func(left int, right int) bool {
		if sprites[left].Scope != sprites[right].Scope {
			return sprites[left].Scope == scopeBuiltin
		}
		if sprites[left].Status != sprites[right].Status {
			return sprites[left].Status == spriteStatusReady
		}
		return strings.ToLower(sprites[left].Name) < strings.ToLower(sprites[right].Name)
	})
}

func (service *Service) InspectSpriteSource(_ context.Context, request dto.InspectSpriteRequest) (dto.SpriteImportDraft, error) {
	inspection, cleanup, err := service.inspectImportSource(strings.TrimSpace(request.Path))
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return dto.SpriteImportDraft{}, err
	}

	return service.buildImportDraft(inspection), nil
}

func (service *Service) ImportSprite(_ context.Context, request dto.ImportSpriteRequest) (dto.Sprite, error) {
	sourcePath := strings.TrimSpace(request.Path)
	inspection, cleanup, err := service.inspectImportSource(sourcePath)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return dto.Sprite{}, err
	}
	if inspection.validationMessage != "" {
		return dto.Sprite{}, fmt.Errorf("%s", inspection.validationMessage)
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	if err := service.ensureBuiltinSpritesLocked(); err != nil {
		return dto.Sprite{}, err
	}
	service.builtinEnsured = true

	return service.storeSpriteFromInspectionLocked(scopeImported, inspection, request, filepath.Base(sourcePath))
}

func (service *Service) InstallSpriteFromURL(ctx context.Context, request dto.InstallSpriteFromURLRequest) (dto.Sprite, error) {
	downloadURL := strings.TrimSpace(request.URL)
	if downloadURL == "" {
		return dto.Sprite{}, fmt.Errorf("sprite download url is required")
	}
	parsedURL, err := urlpkg.Parse(downloadURL)
	if err != nil || parsedURL == nil || parsedURL.Host == "" {
		return dto.Sprite{}, fmt.Errorf("sprite download url is invalid")
	}
	if parsedURL.Scheme != "https" {
		return dto.Sprite{}, fmt.Errorf("sprite download url must use https")
	}
	expectedHash := strings.ToLower(strings.TrimSpace(request.SHA256))
	if len(expectedHash) != sha256.Size*2 {
		return dto.Sprite{}, fmt.Errorf("sprite archive checksum is required")
	}
	if _, err := hex.DecodeString(expectedHash); err != nil {
		return dto.Sprite{}, fmt.Errorf("sprite archive checksum is invalid")
	}
	if request.Size > spriteMaxZipSizeBytes {
		return dto.Sprite{}, fmt.Errorf("sprite archive must be at most %s", formatByteLimit(spriteMaxZipSizeBytes))
	}

	tempDir, err := os.MkdirTemp("", "xiadown-sprite-online-*")
	if err != nil {
		return dto.Sprite{}, fmt.Errorf("create sprite temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, spriteExportArchiveFileName(downloadURL))
	if err := downloadSpriteArchive(ctx, service.httpClient, downloadURL, archivePath, request.Size, expectedHash); err != nil {
		return dto.Sprite{}, err
	}
	return service.ImportSprite(ctx, dto.ImportSpriteRequest{
		Path:              archivePath,
		Name:              request.Name,
		Description:       request.Description,
		AuthorDisplayName: request.AuthorDisplayName,
		Version:           request.Version,
		Origin:            "online",
	})
}

func spriteExportArchiveFileName(downloadURL string) string {
	parsedURL, err := urlpkg.Parse(downloadURL)
	if err == nil && parsedURL != nil {
		baseName := strings.TrimSpace(path.Base(parsedURL.Path))
		if baseName != "" && baseName != "." && strings.EqualFold(filepath.Ext(baseName), ".zip") {
			return sanitizeImportFileName(baseName)
		}
	}
	return "sprite.zip"
}

func downloadSpriteArchive(ctx context.Context, client *http.Client, downloadURL string, outputPath string, expectedSize int64, expectedSHA256 string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create sprite download request: %w", err)
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download sprite archive: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("download sprite archive failed: %s", response.Status)
	}
	if expectedSize > 0 && response.ContentLength > 0 && response.ContentLength != expectedSize {
		return fmt.Errorf("sprite archive size mismatch")
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create sprite archive: %w", err)
	}

	hash := sha256.New()
	limitedReader := &io.LimitedReader{R: io.TeeReader(response.Body, hash), N: spriteMaxZipSizeBytes + 1}
	written, copyErr := io.Copy(outputFile, limitedReader)
	closeErr := outputFile.Close()
	if copyErr != nil {
		return fmt.Errorf("write sprite archive: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("finalize sprite archive: %w", closeErr)
	}
	if written > spriteMaxZipSizeBytes {
		return fmt.Errorf("sprite archive must be at most %s", formatByteLimit(spriteMaxZipSizeBytes))
	}
	if expectedSize > 0 && written != expectedSize {
		return fmt.Errorf("sprite archive size mismatch")
	}
	expectedHash := strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expectedHash == "" {
		return nil
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("sprite archive checksum mismatch")
	}
	return nil
}

func (service *Service) UpdateSprite(_ context.Context, request dto.UpdateSpriteRequest) (dto.Sprite, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	scope, spriteDir, err := service.findSpriteDirectoryLocked(strings.TrimSpace(request.ID))
	if err != nil {
		return dto.Sprite{}, err
	}
	if scope != scopeImported {
		return dto.Sprite{}, fmt.Errorf("builtin sprites are read only")
	}

	sprite, manifest, err := service.inspectSpriteDirectoryLocked(scopeImported, spriteDir)
	if err != nil {
		return dto.Sprite{}, err
	}
	if sprite.Status != spriteStatusReady {
		return dto.Sprite{}, fmt.Errorf("sprite is invalid and cannot be updated")
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		return dto.Sprite{}, fmt.Errorf("sprite name is required")
	}
	manifest.Name = normalizeSpriteName(name)
	manifest.Description = strings.TrimSpace(request.Description)
	manifest.Author.DisplayName = firstNonEmpty(strings.TrimSpace(request.AuthorDisplayName), defaultSpriteAuthorDisplayName())
	manifest.Version = normalizeSpriteVersion(strings.TrimSpace(request.Version))
	if err := writeSpriteManifest(spriteDir, manifest); err != nil {
		return dto.Sprite{}, err
	}

	updated, _, err := service.inspectSpriteDirectoryLocked(scopeImported, spriteDir)
	if err != nil {
		return dto.Sprite{}, err
	}
	updated, _ = service.syncSpriteMetadataLocked(updated)
	return updated, nil
}

func (service *Service) GetSpriteManifest(_ context.Context, request dto.GetSpriteManifestRequest) (dto.SpriteManifest, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	scope, spriteDir, err := service.findSpriteDirectoryLocked(strings.TrimSpace(request.ID))
	if err != nil {
		return dto.SpriteManifest{}, err
	}

	sprite, manifest, err := service.inspectSpriteDirectoryLocked(scope, spriteDir)
	if err != nil {
		return dto.SpriteManifest{}, err
	}

	spritePath := filepath.Join(spriteDir, spriteExportFileName)
	imageWidth, imageHeight, err := readImageSize(spritePath)
	if err != nil {
		return dto.SpriteManifest{}, err
	}
	sheetWidth, sheetHeight := spriteSliceGridSize(manifest.SliceGrid)
	return dto.SpriteManifest{
		ID:          sprite.ID,
		Name:        sprite.Name,
		Scope:       sprite.Scope,
		SpritePath:  spritePath,
		SourceType:  sprite.SourceType,
		ImageWidth:  imageWidth,
		ImageHeight: imageHeight,
		SheetWidth:  sheetWidth,
		SheetHeight: sheetHeight,
		Columns:     spriteColumns,
		Rows:        spriteRows,
		SliceGrid:   toDTOSpriteSliceGrid(manifest.SliceGrid),
		CanEdit:     scope == scopeImported && sprite.Status == spriteStatusReady,
		UpdatedAt:   sprite.UpdatedAt,
	}, nil
}

func (service *Service) UpdateSpriteSlices(_ context.Context, request dto.UpdateSpriteSlicesRequest) (dto.Sprite, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	scope, spriteDir, err := service.findSpriteDirectoryLocked(strings.TrimSpace(request.ID))
	if err != nil {
		return dto.Sprite{}, err
	}
	if scope != scopeImported {
		return dto.Sprite{}, fmt.Errorf("builtin sprites are read only")
	}

	sprite, manifest, err := service.inspectSpriteDirectoryLocked(scopeImported, spriteDir)
	if err != nil {
		return dto.Sprite{}, err
	}
	if sprite.Status != spriteStatusReady {
		return dto.Sprite{}, fmt.Errorf("sprite is invalid and cannot be updated")
	}

	spritePath := filepath.Join(spriteDir, spriteExportFileName)
	spriteFingerprint, err := statFileFingerprint(spritePath)
	if err != nil {
		return dto.Sprite{}, err
	}
	imageWidth, imageHeight, err := readImageSize(spritePath)
	if err != nil {
		return dto.Sprite{}, err
	}
	spec := validateSpriteDimensions(imageWidth, imageHeight)
	if spec.validationMessage != "" {
		return dto.Sprite{}, fmt.Errorf("%s", spec.validationMessage)
	}
	grid, err := validateSpriteSliceGrid(request.SliceGrid, spriteColumns, spriteRows, spec.sheetWidth, spec.sheetHeight)
	if err != nil {
		return dto.Sprite{}, err
	}
	if ok, err := sameFileFingerprint(spritePath, spriteFingerprint); err != nil {
		return dto.Sprite{}, err
	} else if !ok {
		return dto.Sprite{}, fmt.Errorf("sprite changed while updating slices")
	}

	manifest.SliceGrid = grid
	if err := writeSpriteManifest(spriteDir, manifest); err != nil {
		return dto.Sprite{}, err
	}

	updated, _, err := service.inspectSpriteDirectoryLocked(scopeImported, spriteDir)
	if err != nil {
		return dto.Sprite{}, err
	}
	updated, _ = service.syncSpriteMetadataLocked(updated)
	return updated, nil
}

func (service *Service) ExportSprite(_ context.Context, request dto.ExportSpriteRequest) error {
	service.mu.Lock()
	defer service.mu.Unlock()

	scope, spriteDir, err := service.findSpriteDirectoryLocked(strings.TrimSpace(request.ID))
	if err != nil {
		return err
	}
	_, manifest, err := service.inspectSpriteDirectoryLocked(scope, spriteDir)
	if err != nil {
		return err
	}

	outputPath := strings.TrimSpace(request.OutputPath)
	if outputPath == "" {
		return fmt.Errorf("output path is required")
	}
	if strings.ToLower(filepath.Ext(outputPath)) != ".zip" {
		outputPath += ".zip"
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}

	archiveFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create export file: %w", err)
	}
	defer archiveFile.Close()

	zipWriter := zip.NewWriter(archiveFile)

	manifestEntry, err := zipWriter.Create(spriteManifestFileName)
	if err != nil {
		return fmt.Errorf("create manifest entry: %w", err)
	}
	if err := json.NewEncoder(manifestEntry).Encode(manifest); err != nil {
		return fmt.Errorf("write manifest entry: %w", err)
	}

	imageFile, err := os.Open(filepath.Join(spriteDir, spriteExportFileName))
	if err != nil {
		return fmt.Errorf("open sprite image: %w", err)
	}
	defer imageFile.Close()

	imageEntry, err := zipWriter.Create(spriteExportFileName)
	if err != nil {
		return fmt.Errorf("create image entry: %w", err)
	}
	if _, err := io.Copy(imageEntry, imageFile); err != nil {
		return fmt.Errorf("write image entry: %w", err)
	}

	previewData, err := buildSpritePreviewWebP(filepath.Join(spriteDir, spriteExportFileName))
	if err != nil {
		return err
	}
	previewEntry, err := zipWriter.Create(spritePreviewFileName)
	if err != nil {
		return fmt.Errorf("create preview entry: %w", err)
	}
	if _, err := previewEntry.Write(previewData); err != nil {
		return fmt.Errorf("write preview entry: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("finalize sprite archive: %w", err)
	}
	return nil
}

func (service *Service) DeleteSprite(_ context.Context, request dto.DeleteSpriteRequest) error {
	service.mu.Lock()
	defer service.mu.Unlock()

	scope, spriteDir, err := service.findSpriteDirectoryLocked(strings.TrimSpace(request.ID))
	if err != nil {
		return err
	}
	if scope != scopeImported {
		return fmt.Errorf("builtin sprites cannot be deleted")
	}
	if err := os.RemoveAll(spriteDir); err != nil {
		return err
	}
	service.deleteSpriteMetadataLocked(strings.TrimSpace(request.ID))
	return nil
}

func (service *Service) ensureBuiltinSpritesLocked() error {
	if strings.TrimSpace(service.baseDir) == "" {
		return fmt.Errorf("sprite base directory is not configured")
	}
	if err := os.MkdirAll(service.builtinDir(), 0o755); err != nil {
		return fmt.Errorf("create builtin sprite directory: %w", err)
	}
	if err := os.MkdirAll(service.importedDir(), 0o755); err != nil {
		return fmt.Errorf("create imported sprite directory: %w", err)
	}

	syncedIDs := make(map[string]struct{})
	if service.builtinFS != nil && service.builtinRoot != "" {
		ids, err := service.syncBuiltinSpritesFromFSLocked()
		if err != nil {
			return err
		}
		for _, id := range ids {
			syncedIDs[id] = struct{}{}
		}
	}
	if service.devBuiltinDir != "" {
		if stat, err := os.Stat(service.devBuiltinDir); err == nil && stat.IsDir() {
			ids, err := service.syncBuiltinSpritesFromDirLocked()
			if err != nil {
				return err
			}
			for _, id := range ids {
				syncedIDs[id] = struct{}{}
			}
		}
	}

	if err := service.pruneBuiltinSpritesLocked(syncedIDs); err != nil {
		return err
	}
	service.builtinEnsured = true
	return nil
}

func (service *Service) syncBuiltinSpritesFromFSLocked() ([]string, error) {
	entries, err := fs.ReadDir(service.builtinFS, service.builtinRoot)
	if err != nil {
		if errorsIsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read embedded builtin sprites: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		srcPath := path.Join(service.builtinRoot, name)
		if !entry.IsDir() && !isSupportedSpriteImportFile(name) {
			continue
		}
		syncedID, err := service.syncBuiltinSourceFromFSLocked(srcPath, entry.IsDir())
		if err != nil {
			return nil, err
		}
		if syncedID != "" {
			ids = append(ids, syncedID)
		}
	}
	return ids, nil
}

func (service *Service) syncBuiltinSpritesFromDirLocked() ([]string, error) {
	entries, err := os.ReadDir(service.devBuiltinDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read development builtin sprites: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		sourcePath := filepath.Join(service.devBuiltinDir, name)
		if !entry.IsDir() && !isSupportedSpriteImportFile(name) {
			continue
		}
		syncedID, err := service.syncBuiltinSourceFromDiskLocked(sourcePath)
		if err != nil {
			return nil, err
		}
		if syncedID != "" {
			ids = append(ids, syncedID)
		}
	}
	return ids, nil
}

func (service *Service) listScopeLocked(scope string, root string) ([]dto.Sprite, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []dto.Sprite{}, nil
		}
		return nil, fmt.Errorf("read sprite directory: %w", err)
	}

	sprites := make([]dto.Sprite, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		spriteDir := filepath.Join(root, entry.Name())
		sprite, _, err := service.inspectSpriteDirectoryLocked(scope, spriteDir)
		if err != nil {
			continue
		}
		sprites = append(sprites, sprite)
	}
	return sprites, nil
}

func (service *Service) inspectSpriteDirectoryLocked(scope string, spriteDir string) (dto.Sprite, spriteManifest, error) {
	stat, err := os.Stat(spriteDir)
	if err != nil {
		return dto.Sprite{}, spriteManifest{}, fmt.Errorf("stat sprite directory: %w", err)
	}

	manifest := readSpriteManifest(spriteDir)
	spritePath := filepath.Join(spriteDir, spriteExportFileName)
	spriteStat, err := os.Stat(spritePath)
	if err != nil || spriteStat.IsDir() {
		return dto.Sprite{}, spriteManifest{}, fmt.Errorf("sprite image not found in %s", spriteDir)
	}
	imageWidth, imageHeight, err := readImageSize(spritePath)
	if err != nil {
		return dto.Sprite{}, spriteManifest{}, err
	}
	spec := validateSpriteDimensions(imageWidth, imageHeight)
	sliceGrid := normalizeSpriteSliceGrid(manifest.SliceGrid, spec.sheetWidth, spec.sheetHeight)

	createdAt := stat.ModTime().UTC()
	if parsed := parseSpriteTimestamp(strings.TrimSpace(manifest.CreatedAt)); !parsed.IsZero() {
		createdAt = parsed.UTC()
	}
	updatedAt := maxTime(stat.ModTime(), spriteStat.ModTime()).UTC()
	if manifestStat, err := os.Stat(filepath.Join(spriteDir, spriteManifestFileName)); err == nil {
		updatedAt = maxTime(updatedAt, manifestStat.ModTime()).UTC()
	}

	normalized := spriteManifest{
		App:         spriteManifestAppID,
		ID:          normalizeSpriteID(scope, filepath.Base(spriteDir), strings.TrimSpace(manifest.ID)),
		Name:        normalizeSpriteName(firstNonEmpty(strings.TrimSpace(manifest.Name), filepath.Base(spriteDir))),
		Description: strings.TrimSpace(manifest.Description),
		FrameCount:  spriteFrameCount,
		Columns:     spriteColumns,
		Rows:        spriteRows,
		SpriteFile:  spriteExportFileName,
		SourceType:  normalizeSpriteSourceType(manifest.SourceType),
		Origin:      normalizeSpriteOrigin(manifest.Origin),
		SliceGrid:   sliceGrid,
		Author: spriteAuthorManifest{
			ID:          normalizeSpriteAuthorID(scope, filepath.Base(spriteDir), strings.TrimSpace(manifest.Author.ID)),
			DisplayName: firstNonEmpty(strings.TrimSpace(manifest.Author.DisplayName), defaultSpriteAuthorDisplayName()),
		},
		CreatedAt: createdAt.Format(time.RFC3339),
		Version:   normalizeSpriteVersion(strings.TrimSpace(manifest.Version)),
	}

	status := spriteStatusReady
	if spec.validationMessage != "" {
		status = spriteStatusInvalid
	}

	return dto.Sprite{
		ID:                normalized.ID,
		Name:              normalized.Name,
		Description:       normalized.Description,
		FrameCount:        normalized.FrameCount,
		Columns:           normalized.Columns,
		Rows:              normalized.Rows,
		SpriteFile:        spriteExportFileName,
		SpritePath:        spritePath,
		SourceType:        normalized.SourceType,
		Origin:            normalized.Origin,
		Scope:             scope,
		Status:            status,
		ValidationMessage: spec.validationMessage,
		ImageWidth:        imageWidth,
		ImageHeight:       imageHeight,
		Author: dto.SpriteAuthor{
			ID:          normalized.Author.ID,
			DisplayName: normalized.Author.DisplayName,
		},
		CreatedAt: normalized.CreatedAt,
		UpdatedAt: updatedAt.Format(time.RFC3339Nano),
		Version:   normalized.Version,
	}, normalized, nil
}

func (service *Service) inspectImportSource(sourcePath string) (spriteImportInspection, func(), error) {
	source, cleanup, err := service.prepareImportSource(sourcePath)
	if err != nil {
		return spriteImportInspection{}, nil, err
	}

	width, height, err := readImageSize(source.imagePath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return spriteImportInspection{}, nil, err
	}

	spec := validateSpriteDimensions(width, height)
	inspection := spriteImportInspection{
		sourcePath:        sourcePath,
		sourceType:        detectSpriteSourceType(sourcePath),
		imageWidth:        width,
		imageHeight:       height,
		sheetWidth:        spec.sheetWidth,
		sheetHeight:       spec.sheetHeight,
		sliceGrid:         normalizeSpriteSliceGrid(source.manifest.SliceGrid, spec.sheetWidth, spec.sheetHeight),
		validationMessage: spec.validationMessage,
		source:            source,
	}
	if isSupportedSpriteImage(sourcePath) {
		inspection.previewPath = sourcePath
	}
	return inspection, cleanup, nil
}

func (service *Service) buildImportDraft(inspection spriteImportInspection) dto.SpriteImportDraft {
	manifest := inspection.source.manifest
	status := spriteStatusReady
	if inspection.validationMessage != "" {
		status = spriteStatusInvalid
	}

	return dto.SpriteImportDraft{
		Path:              inspection.sourcePath,
		PreviewPath:       inspection.previewPath,
		SourceType:        inspection.sourceType,
		Name:              normalizeSpriteName(firstNonEmpty(strings.TrimSpace(manifest.Name), inspection.source.sourceName)),
		Description:       strings.TrimSpace(manifest.Description),
		AuthorDisplayName: firstNonEmpty(strings.TrimSpace(manifest.Author.DisplayName), defaultSpriteAuthorDisplayName()),
		Version:           normalizeSpriteVersion(strings.TrimSpace(manifest.Version)),
		FrameCount:        spriteFrameCount,
		Columns:           spriteColumns,
		Rows:              spriteRows,
		SpriteFile:        spriteExportFileName,
		Status:            status,
		ValidationMessage: inspection.validationMessage,
		ImageWidth:        inspection.imageWidth,
		ImageHeight:       inspection.imageHeight,
	}
}

func (service *Service) storeSpriteFromInspectionLocked(scope string, inspection spriteImportInspection, request dto.ImportSpriteRequest, sourceSeed string) (dto.Sprite, error) {
	if inspection.validationMessage != "" {
		return dto.Sprite{}, fmt.Errorf("%s", inspection.validationMessage)
	}
	targetID := service.resolveTargetSpriteIDLocked(scope, inspection, sourceSeed)
	targetDir := filepath.Join(service.spriteRootForScope(scope), targetID)
	if scope == scopeBuiltin {
		if err := os.RemoveAll(targetDir); err != nil {
			return dto.Sprite{}, fmt.Errorf("reset builtin sprite directory: %w", err)
		}
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return dto.Sprite{}, fmt.Errorf("create sprite directory: %w", err)
	}

	if err := writeStoredSpritePNG(inspection.source.imagePath, filepath.Join(targetDir, spriteExportFileName)); err != nil {
		return dto.Sprite{}, err
	}

	manifest := inspection.source.manifest
	createdAt := service.now().UTC()
	if parsed := parseSpriteTimestamp(strings.TrimSpace(manifest.CreatedAt)); !parsed.IsZero() {
		createdAt = parsed.UTC()
	}

	normalized := spriteManifest{
		App:         spriteManifestAppID,
		ID:          targetID,
		Name:        normalizeSpriteName(firstNonEmpty(strings.TrimSpace(request.Name), strings.TrimSpace(manifest.Name), inspection.source.sourceName)),
		Description: firstNonEmpty(strings.TrimSpace(request.Description), strings.TrimSpace(manifest.Description)),
		FrameCount:  spriteFrameCount,
		Columns:     spriteColumns,
		Rows:        spriteRows,
		SpriteFile:  spriteExportFileName,
		SourceType:  normalizeSpriteSourceType(inspection.sourceType),
		Origin:      normalizeSpriteOrigin(firstNonEmpty(strings.TrimSpace(request.Origin), strings.TrimSpace(manifest.Origin))),
		SliceGrid:   normalizeSpriteSliceGrid(inspection.sliceGrid, inspection.sheetWidth, inspection.sheetHeight),
		Author: spriteAuthorManifest{
			ID:          normalizeSpriteAuthorID(scope, sourceSeed, strings.TrimSpace(manifest.Author.ID)),
			DisplayName: firstNonEmpty(strings.TrimSpace(request.AuthorDisplayName), strings.TrimSpace(manifest.Author.DisplayName), defaultSpriteAuthorDisplayName()),
		},
		CreatedAt: createdAt.Format(time.RFC3339),
		Version:   normalizeSpriteVersion(firstNonEmpty(strings.TrimSpace(request.Version), strings.TrimSpace(manifest.Version))),
	}

	if err := writeSpriteManifest(targetDir, normalized); err != nil {
		return dto.Sprite{}, err
	}

	sprite, _, err := service.inspectSpriteDirectoryLocked(scope, targetDir)
	if err != nil {
		return dto.Sprite{}, err
	}
	sprite, _ = service.syncSpriteMetadataLocked(sprite)
	return sprite, nil
}

func (service *Service) prepareImportSource(sourcePath string) (importedSource, func(), error) {
	if sourcePath == "" {
		return importedSource{}, nil, fmt.Errorf("sprite path is required")
	}

	if stat, err := os.Stat(sourcePath); err == nil && stat.IsDir() {
		return prepareImportDirectory(sourcePath)
	}
	if isSupportedSpriteImage(sourcePath) {
		return importedSource{
			imagePath:  sourcePath,
			sourceName: strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)),
			manifest: spriteManifest{
				App:        spriteManifestAppID,
				SpriteFile: spriteExportFileName,
			},
		}, nil, nil
	}
	if strings.ToLower(filepath.Ext(sourcePath)) != ".zip" {
		return importedSource{}, nil, fmt.Errorf("sprite import only supports PNG or ZIP")
	}
	if stat, err := os.Stat(sourcePath); err != nil {
		return importedSource{}, nil, fmt.Errorf("stat sprite archive: %w", err)
	} else if stat.Size() > spriteMaxZipSizeBytes {
		return importedSource{}, nil, fmt.Errorf("sprite archive must be at most %s", formatByteLimit(spriteMaxZipSizeBytes))
	}

	archive, err := zip.OpenReader(sourcePath)
	if err != nil {
		return importedSource{}, nil, fmt.Errorf("open sprite archive: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "xiadown-sprite-import-*")
	if err != nil {
		archive.Close()
		return importedSource{}, nil, fmt.Errorf("create sprite temp directory: %w", err)
	}

	cleanup := func() {
		archive.Close()
		_ = os.RemoveAll(tempDir)
	}

	var manifest spriteManifest
	var images []extractedImageFile
	var extractedBytes int64
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		baseName := path.Base(file.Name)
		lowerBase := strings.ToLower(baseName)
		switch {
		case lowerBase == spriteManifestFileName:
			payload, written, err := readZipFile(file, spriteMaxZipSizeBytes-extractedBytes)
			if err != nil {
				cleanup()
				return importedSource{}, nil, err
			}
			extractedBytes += written
			if err := json.Unmarshal(payload, &manifest); err != nil {
				cleanup()
				return importedSource{}, nil, fmt.Errorf("decode manifest from archive: %w", err)
			}
		case isSupportedSpriteImage(baseName):
			extractedPath := filepath.Join(tempDir, sanitizeImportFileName(baseName))
			written, err := extractZipFile(file, extractedPath, spriteMaxZipSizeBytes-extractedBytes)
			if err != nil {
				cleanup()
				return importedSource{}, nil, err
			}
			extractedBytes += written
			images = append(images, extractedImageFile{path: extractedPath, name: baseName})
		}
	}

	if err := validateManifestApp(manifest); err != nil {
		cleanup()
		return importedSource{}, nil, err
	}

	if len(images) == 0 {
		cleanup()
		return importedSource{}, nil, fmt.Errorf("sprite archive does not contain a PNG sprite sheet")
	}

	selected := images[0]
	preferredName := firstNonEmpty(strings.TrimSpace(manifest.SpriteFile), spriteExportFileName)
	for _, candidate := range images {
		if strings.EqualFold(candidate.name, preferredName) {
			selected = candidate
			break
		}
	}

	sourceName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if preferredName := strings.TrimSpace(manifest.Name); preferredName != "" {
		sourceName = preferredName
	}

	return importedSource{
		imagePath:  selected.path,
		sourceName: sourceName,
		manifest:   manifest,
	}, cleanup, nil
}

func prepareImportDirectory(sourcePath string) (importedSource, func(), error) {
	manifest, err := readSpriteManifestFromDirectory(sourcePath)
	if err != nil {
		return importedSource{}, nil, err
	}
	if err := validateManifestApp(manifest); err != nil {
		return importedSource{}, nil, err
	}
	imageName, imagePath, err := findImportSpriteImageFile(sourcePath, manifest.SpriteFile)
	if err != nil {
		return importedSource{}, nil, err
	}

	sourceName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if preferredName := strings.TrimSpace(manifest.Name); preferredName != "" {
		sourceName = preferredName
	} else if sourceName == "" {
		sourceName = strings.TrimSuffix(imageName, filepath.Ext(imageName))
	}

	return importedSource{
		imagePath:  imagePath,
		sourceName: sourceName,
		manifest:   manifest,
	}, nil, nil
}

func (service *Service) findSpriteDirectoryLocked(id string) (string, string, error) {
	if id == "" {
		return "", "", fmt.Errorf("sprite id is required")
	}

	candidates := []struct {
		scope string
		root  string
	}{
		{scope: scopeBuiltin, root: service.builtinDir()},
		{scope: scopeImported, root: service.importedDir()},
	}
	for _, candidate := range candidates {
		spriteDir := filepath.Join(candidate.root, id)
		if stat, err := os.Stat(spriteDir); err == nil && stat.IsDir() {
			return candidate.scope, spriteDir, nil
		}
		entries, err := os.ReadDir(candidate.root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(candidate.root, entry.Name())
			sprite, _, err := service.inspectSpriteDirectoryLocked(candidate.scope, dir)
			if err != nil {
				continue
			}
			if sprite.ID == id {
				return candidate.scope, dir, nil
			}
		}
	}
	return "", "", fmt.Errorf("sprite %q not found", id)
}

func (service *Service) spriteDirExistsLocked(id string) bool {
	if id == "" {
		return false
	}
	for _, root := range []string{service.importedDir(), service.builtinDir()} {
		stat, err := os.Stat(filepath.Join(root, id))
		if err == nil && stat.IsDir() {
			return true
		}
	}
	return false
}

func (service *Service) resolveTargetSpriteIDLocked(scope string, inspection spriteImportInspection, sourceSeed string) string {
	if scope == scopeBuiltin {
		return service.resolveBuiltinTargetIDLocked(sourceSeed, strings.TrimSpace(inspection.source.manifest.ID))
	}

	if parsed, err := uuid.Parse(strings.TrimSpace(inspection.source.manifest.ID)); err == nil {
		id := parsed.String()
		if !service.spriteDirExistsLocked(id) {
			return id
		}
	}
	return uuid.NewString()
}

func (service *Service) resolveBuiltinTargetIDLocked(sourceSeed string, current string) string {
	if parsed, err := uuid.Parse(strings.TrimSpace(current)); err == nil {
		id := parsed.String()
		if stat, err := os.Stat(filepath.Join(service.builtinDir(), id)); err == nil && stat.IsDir() {
			return id
		}
		if stat, err := os.Stat(filepath.Join(service.importedDir(), id)); err == nil && stat.IsDir() {
			return normalizeSpriteID(scopeBuiltin, sourceSeed, "")
		}
		return id
	}
	return normalizeSpriteID(scopeBuiltin, sourceSeed, "")
}

func (service *Service) spriteRootForScope(scope string) string {
	if scope == scopeBuiltin {
		return service.builtinDir()
	}
	return service.importedDir()
}

func (service *Service) builtinDir() string {
	return filepath.Join(service.baseDir, scopeBuiltin)
}

func (service *Service) importedDir() string {
	return filepath.Join(service.baseDir, scopeImported)
}

func statFileFingerprint(path string) (fileFingerprint, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return fileFingerprint{}, fmt.Errorf("stat sprite file: %w", err)
	}
	if stat.IsDir() {
		return fileFingerprint{}, fmt.Errorf("sprite file %s is a directory", path)
	}
	return fileFingerprint{size: stat.Size(), modTime: stat.ModTime()}, nil
}

func sameFileFingerprint(path string, expected fileFingerprint) (bool, error) {
	current, err := statFileFingerprint(path)
	if err != nil {
		return false, err
	}
	return current.size == expected.size && current.modTime.Equal(expected.modTime), nil
}

func defaultSpriteAuthorDisplayName() string {
	current, err := user.Current()
	if err != nil {
		return "XiaDown"
	}
	if name := strings.TrimSpace(current.Name); name != "" {
		return name
	}
	if username := strings.TrimSpace(current.Username); username != "" {
		return username
	}
	return "XiaDown"
}

func (service *Service) syncBuiltinSourceFromFSLocked(sourcePath string, isDir bool) (string, error) {
	tempDir, err := os.MkdirTemp("", "xiadown-builtin-fs-*")
	if err != nil {
		return "", fmt.Errorf("create builtin staging directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	localPath := filepath.Join(tempDir, filepath.Base(sourcePath))
	if isDir {
		subFS, err := fs.Sub(service.builtinFS, sourcePath)
		if err != nil {
			return "", fmt.Errorf("open embedded builtin sprite directory: %w", err)
		}
		if err := os.CopyFS(localPath, subFS); err != nil {
			return "", fmt.Errorf("copy embedded builtin sprite directory: %w", err)
		}
	} else {
		if err := copyFileFS(service.builtinFS, sourcePath, localPath); err != nil {
			return "", err
		}
	}

	sprite, err := service.syncBuiltinSourceLocked(localPath)
	if err != nil {
		return "", err
	}
	return sprite.ID, nil
}

func (service *Service) syncBuiltinSourceFromDiskLocked(sourcePath string) (string, error) {
	sprite, err := service.syncBuiltinSourceLocked(sourcePath)
	if err != nil {
		return "", err
	}
	return sprite.ID, nil
}

func (service *Service) syncBuiltinSourceLocked(sourcePath string) (dto.Sprite, error) {
	inspection, cleanup, err := service.inspectImportSource(sourcePath)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return dto.Sprite{}, err
	}

	seed := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	return service.storeSpriteFromInspectionLocked(scopeBuiltin, inspection, dto.ImportSpriteRequest{}, seed)
}

func (service *Service) pruneBuiltinSpritesLocked(syncedIDs map[string]struct{}) error {
	entries, err := os.ReadDir(service.builtinDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read builtin sprite directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := syncedIDs[entry.Name()]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(service.builtinDir(), entry.Name())); err != nil {
			return fmt.Errorf("remove stale builtin sprite: %w", err)
		}
		service.deleteSpriteMetadataLocked(entry.Name())
	}
	return nil
}

func (service *Service) syncSpriteMetadataLocked(sprite dto.Sprite) (dto.Sprite, bool) {
	if service.metadataRepo == nil {
		return sprite, false
	}

	coverPNG, err := buildSpriteCoverPNG(sprite)
	if err == nil {
		sprite.CoverImageDataURL = coverPNGToDataURL(coverPNG)
	}
	if saveErr := service.metadataRepo.Save(context.Background(), sprite, coverPNG); saveErr != nil {
		service.metadataPrimed = false
		return sprite, false
	}
	return sprite, true
}

func (service *Service) deleteSpriteMetadataLocked(id string) bool {
	if service.metadataRepo == nil {
		return false
	}
	if err := service.metadataRepo.Delete(context.Background(), strings.TrimSpace(id)); err != nil {
		service.metadataPrimed = false
		return false
	}
	return true
}
