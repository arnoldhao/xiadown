package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/HugoSmits86/nativewebp"
	"github.com/google/uuid"
	"xiadown/internal/application/pets/dto"
)

const (
	petColumns          = 8
	petRows             = 9
	petCellWidth        = 192
	petCellHeight       = 208
	petSheetWidth       = petColumns * petCellWidth
	petSheetHeight      = petRows * petCellHeight
	petFrameCount       = petColumns * petRows
	petMaxZipSizeBytes  = 15 * 1024 * 1024
	petManifestFileName = "pet.json"
	petSheetFileName    = "spritesheet.webp"

	scopeBuiltin  = "builtin"
	scopeImported = "imported"

	petOriginLocal = "local"

	petStatusReady   = "ready"
	petStatusInvalid = "invalid"
)

var petIDPattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type petManifest struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	Name            string `json:"name,omitempty"`
	Description     string `json:"description"`
	SpritesheetPath string `json:"spritesheetPath"`
}

type petImportInspection struct {
	path              string
	manifest          petManifest
	spritesheetBytes  []byte
	imageWidth        int
	imageHeight       int
	status            string
	validationCode    string
	validationMessage string
}

type MetadataRepository interface {
	List(ctx context.Context) ([]dto.Pet, error)
	Save(ctx context.Context, pet dto.Pet) error
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
	importSessions map[string]*onlinePetImportSession
	builtinReady   bool
	now            func() time.Time
}

func NewService(baseDir string, builtinFS fs.FS, builtinRoot string, devBuiltinDir string, options ...Option) *Service {
	service := &Service{
		baseDir:        strings.TrimSpace(baseDir),
		builtinFS:      builtinFS,
		builtinRoot:    strings.Trim(strings.TrimSpace(builtinRoot), "/"),
		devBuiltinDir:  strings.TrimSpace(devBuiltinDir),
		importSessions: make(map[string]*onlinePetImportSession),
		now:            time.Now,
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

func DefaultPetsBaseDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(configDir, "xiadown", "pets"), nil
}

func (service *Service) EnsureBuiltinPets(ctx context.Context) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.ensureBuiltinPetsLocked(ctx); err != nil {
		return err
	}
	service.builtinReady = true
	return nil
}

func (service *Service) ListPets(ctx context.Context) ([]dto.Pet, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	if !service.builtinReady {
		if err := service.ensureBuiltinPetsLocked(ctx); err != nil {
			return nil, err
		}
		service.builtinReady = true
	}

	pets, err := service.scanPetsLocked()
	if err != nil {
		return nil, err
	}
	service.syncMetadataLocked(ctx, pets)
	sortPets(pets)
	return pets, nil
}

func (service *Service) InspectPetSource(_ context.Context, request dto.InspectPetRequest) (dto.PetImportDraft, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	inspection, err := service.inspectZipLocked(request.Path)
	if err != nil {
		return dto.PetImportDraft{}, err
	}
	return inspection.toDraft(), nil
}

func (service *Service) ImportPet(ctx context.Context, request dto.ImportPetRequest) (dto.Pet, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	inspection, err := service.inspectZipLocked(request.Path)
	if err != nil {
		return dto.Pet{}, err
	}
	if inspection.status != petStatusReady {
		return dto.Pet{}, newPetError(inspection.validationCode, inspection.validationMessage)
	}

	pet, err := service.storeImportedPetLocked(inspection, request.Origin)
	if err != nil {
		return dto.Pet{}, err
	}
	service.saveMetadataLocked(ctx, pet)
	return pet, nil
}

func (service *Service) GetPetManifest(ctx context.Context, request dto.GetPetManifestRequest) (dto.PetManifest, error) {
	pet, err := service.findPet(ctx, request.ID)
	if err != nil {
		return dto.PetManifest{}, err
	}
	return dto.PetManifest{
		ID:              pet.ID,
		DisplayName:     pet.DisplayName,
		Description:     pet.Description,
		Scope:           pet.Scope,
		SpritesheetPath: pet.SpritesheetPath,
		ImageWidth:      pet.ImageWidth,
		ImageHeight:     pet.ImageHeight,
		SheetWidth:      pet.ImageWidth,
		SheetHeight:     pet.ImageHeight,
		Columns:         pet.Columns,
		Rows:            pet.Rows,
		CellWidth:       pet.CellWidth,
		CellHeight:      pet.CellHeight,
		CanDelete:       pet.Scope == scopeImported,
		UpdatedAt:       pet.UpdatedAt,
	}, nil
}

func (service *Service) ExportPet(ctx context.Context, request dto.ExportPetRequest) error {
	pet, err := service.findPet(ctx, request.ID)
	if err != nil {
		return err
	}
	outputPath := strings.TrimSpace(request.OutputPath)
	if outputPath == "" {
		return fmt.Errorf("output path is required")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create pet export: %w", err)
	}
	fileClosed := false
	defer func() {
		if !fileClosed {
			_ = file.Close()
		}
	}()

	writer := zip.NewWriter(file)

	petDir := filepath.Dir(pet.SpritesheetPath)
	if err := addFileToZip(writer, filepath.Join(petDir, petManifestFileName), petManifestFileName); err != nil {
		return err
	}
	if err := addFileToZip(writer, filepath.Join(petDir, petSheetFileName), petSheetFileName); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finalize pet export: %w", err)
	}
	closeErr := file.Close()
	fileClosed = true
	if closeErr != nil {
		return fmt.Errorf("close pet export: %w", closeErr)
	}
	return nil
}

func (service *Service) DeletePet(ctx context.Context, request dto.DeletePetRequest) error {
	service.mu.Lock()
	defer service.mu.Unlock()

	pet, err := service.findPetLocked(strings.TrimSpace(request.ID))
	if err != nil {
		return err
	}
	if pet.Scope == scopeBuiltin {
		return fmt.Errorf("built-in pets cannot be deleted")
	}
	if err := os.RemoveAll(filepath.Dir(pet.SpritesheetPath)); err != nil {
		return fmt.Errorf("delete pet files: %w", err)
	}
	if service.metadataRepo != nil {
		_ = service.metadataRepo.Delete(ctx, pet.ID)
	}
	return nil
}

func (service *Service) findPet(ctx context.Context, id string) (dto.Pet, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if !service.builtinReady {
		if err := service.ensureBuiltinPetsLocked(ctx); err != nil {
			return dto.Pet{}, err
		}
		service.builtinReady = true
	}
	return service.findPetLocked(id)
}

func (service *Service) findPetLocked(id string) (dto.Pet, error) {
	pets, err := service.scanPetsLocked()
	if err != nil {
		return dto.Pet{}, err
	}
	trimmed := strings.TrimSpace(id)
	for _, pet := range pets {
		if pet.ID == trimmed {
			return pet, nil
		}
	}
	return dto.Pet{}, fmt.Errorf("pet %q not found", trimmed)
}

func (service *Service) ensureBuiltinPetsLocked(ctx context.Context) error {
	if err := os.MkdirAll(service.scopeDir(scopeBuiltin), 0o755); err != nil {
		return fmt.Errorf("create built-in pets directory: %w", err)
	}
	if err := os.MkdirAll(service.scopeDir(scopeImported), 0o755); err != nil {
		return fmt.Errorf("create imported pets directory: %w", err)
	}

	synced := map[string]struct{}{}
	if service.builtinFS != nil && service.builtinRoot != "" {
		entries, err := fs.ReadDir(service.builtinFS, service.builtinRoot)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("read embedded pets: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pet, err := service.syncEmbeddedBuiltinPetLocked(entry.Name())
			if err != nil {
				return err
			}
			synced[pet.ID] = struct{}{}
			service.saveMetadataLocked(ctx, pet)
		}
	}
	return service.pruneStaleBuiltinPetsLocked(ctx, synced)
}

func (service *Service) syncEmbeddedBuiltinPetLocked(entryName string) (dto.Pet, error) {
	sourceDir := filepath.ToSlash(filepath.Join(service.builtinRoot, entryName))
	manifestBytes, err := fs.ReadFile(service.builtinFS, filepath.ToSlash(filepath.Join(sourceDir, petManifestFileName)))
	if err != nil {
		return dto.Pet{}, fmt.Errorf("read embedded pet manifest %s: %w", entryName, err)
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return dto.Pet{}, fmt.Errorf("decode embedded pet manifest %s: %w", entryName, err)
	}
	manifest.ID = normalizePetID(manifest.ID, entryName)
	manifest.DisplayName = normalizePetDisplayName(manifest.DisplayName, manifest.Name, manifest.ID)
	manifest.SpritesheetPath = petSheetFileName

	sheetBytes, err := fs.ReadFile(service.builtinFS, filepath.ToSlash(filepath.Join(sourceDir, petSheetFileName)))
	if err != nil {
		return dto.Pet{}, fmt.Errorf("read embedded pet spritesheet %s: %w", entryName, err)
	}
	width, height, validationCode, validation := validateSpritesheetBytes(sheetBytes)
	targetDir := service.petDir(scopeBuiltin, manifest.ID)
	if err := writePetPackage(targetDir, manifest, sheetBytes); err != nil {
		return dto.Pet{}, err
	}
	return service.petFromStoredPackage(scopeBuiltin, targetDir, validationCode, validation, width, height)
}

func (service *Service) pruneStaleBuiltinPetsLocked(ctx context.Context, synced map[string]struct{}) error {
	entries, err := os.ReadDir(service.scopeDir(scopeBuiltin))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read built-in pets directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := synced[entry.Name()]; ok {
			continue
		}
		if err := os.RemoveAll(service.petDir(scopeBuiltin, entry.Name())); err != nil {
			return fmt.Errorf("remove stale built-in pet: %w", err)
		}
		if service.metadataRepo != nil {
			_ = service.metadataRepo.Delete(ctx, entry.Name())
		}
	}
	return nil
}

func (service *Service) inspectZipLocked(sourcePath string) (petImportInspection, error) {
	trimmed := strings.TrimSpace(sourcePath)
	if trimmed == "" {
		return petImportInspection{}, newPetError(petErrorCodePackagePathRequired, "pet package path is required")
	}
	if !strings.EqualFold(filepath.Ext(trimmed), ".zip") {
		return petImportInspection{}, newPetError(petErrorCodePackageUnsupportedType, "pet import only supports .zip packages")
	}
	stat, err := os.Stat(trimmed)
	if err != nil {
		return petImportInspection{}, wrapPetError(petErrorCodePackageReadFailed, err, "read pet package")
	}
	if stat.Size() > petMaxZipSizeBytes {
		return petImportInspection{}, newPetErrorf(petErrorCodePackageTooLarge, "pet package exceeds the %s limit", formatByteLimit(petMaxZipSizeBytes))
	}

	reader, err := zip.OpenReader(trimmed)
	if err != nil {
		return petImportInspection{}, wrapPetError(petErrorCodePackageOpenFailed, err, "open pet package")
	}
	defer reader.Close()

	manifestBytes, err := readZipFileByBase(&reader.Reader, petManifestFileName)
	if err != nil {
		return petImportInspection{}, err
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return petImportInspection{}, wrapPetError(petErrorCodeManifestDecodeFailed, err, "decode pet manifest")
	}
	manifest.DisplayName = normalizePetDisplayName(manifest.DisplayName, manifest.Name, manifest.ID)
	if manifest.SpritesheetPath = strings.TrimSpace(manifest.SpritesheetPath); manifest.SpritesheetPath == "" {
		manifest.SpritesheetPath = petSheetFileName
	}
	sheetName := filepath.Base(filepath.ToSlash(manifest.SpritesheetPath))
	if sheetName == "." || sheetName == string(filepath.Separator) {
		sheetName = petSheetFileName
	}
	sheetBytes, err := readZipFileByBase(&reader.Reader, sheetName)
	if err != nil && sheetName != petSheetFileName {
		sheetBytes, err = readZipFileByBase(&reader.Reader, petSheetFileName)
	}
	if err != nil {
		return petImportInspection{}, err
	}

	width, height, validationCode, validation := validateSpritesheetBytes(sheetBytes)
	status := petStatusReady
	if validation != "" {
		status = petStatusInvalid
	}
	return petImportInspection{
		path:              trimmed,
		manifest:          manifest,
		spritesheetBytes:  sheetBytes,
		imageWidth:        width,
		imageHeight:       height,
		status:            status,
		validationCode:    validationCode,
		validationMessage: validation,
	}, nil
}

func (inspection petImportInspection) toDraft() dto.PetImportDraft {
	return dto.PetImportDraft{
		Path:              inspection.path,
		DisplayName:       inspection.manifest.DisplayName,
		Description:       strings.TrimSpace(inspection.manifest.Description),
		FrameCount:        petFrameCount,
		Columns:           petColumns,
		Rows:              petRows,
		CellWidth:         petCellWidth,
		CellHeight:        petCellHeight,
		SpritesheetFile:   petSheetFileName,
		Status:            inspection.status,
		ValidationCode:    inspection.validationCode,
		ValidationMessage: inspection.validationMessage,
		ImageWidth:        inspection.imageWidth,
		ImageHeight:       inspection.imageHeight,
	}
}

func (service *Service) storeImportedPetLocked(inspection petImportInspection, origin string) (dto.Pet, error) {
	baseID := normalizePetID(inspection.manifest.ID, inspection.manifest.DisplayName)
	petID := service.uniqueImportedPetIDLocked(baseID)
	manifest := petManifest{
		ID:              petID,
		DisplayName:     inspection.manifest.DisplayName,
		Description:     strings.TrimSpace(inspection.manifest.Description),
		SpritesheetPath: petSheetFileName,
	}
	targetDir := service.petDir(scopeImported, petID)
	if err := writePetPackage(targetDir, manifest, inspection.spritesheetBytes); err != nil {
		return dto.Pet{}, err
	}
	pet, err := service.petFromStoredPackage(scopeImported, targetDir, "", "", inspection.imageWidth, inspection.imageHeight)
	if err != nil {
		return dto.Pet{}, err
	}
	pet.Origin = normalizePetOrigin(origin)
	return pet, nil
}

func (service *Service) uniqueImportedPetIDLocked(baseID string) string {
	baseID = normalizePetID(baseID, uuid.NewString())
	if !service.petDirExists(scopeImported, baseID) && !service.petDirExists(scopeBuiltin, baseID) {
		return baseID
	}
	for i := 0; i < 100; i++ {
		candidate := normalizePetID(fmt.Sprintf("%s-%s", baseID, uuid.NewString()[:8]), baseID)
		if !service.petDirExists(scopeImported, candidate) && !service.petDirExists(scopeBuiltin, candidate) {
			return candidate
		}
	}
	return uuid.NewString()
}

func (service *Service) scanPetsLocked() ([]dto.Pet, error) {
	pets := make([]dto.Pet, 0)
	for _, scope := range []string{scopeBuiltin, scopeImported} {
		entries, err := os.ReadDir(service.scopeDir(scope))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read %s pets directory: %w", scope, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pet, err := service.petFromStoredPackage(scope, service.petDir(scope, entry.Name()), "", "", 0, 0)
			if err != nil {
				continue
			}
			pets = append(pets, pet)
		}
	}
	return pets, nil
}

func (service *Service) petFromStoredPackage(scope string, petDir string, validationCode string, validation string, width int, height int) (dto.Pet, error) {
	manifestBytes, err := os.ReadFile(filepath.Join(petDir, petManifestFileName))
	if err != nil {
		return dto.Pet{}, fmt.Errorf("read pet manifest: %w", err)
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return dto.Pet{}, fmt.Errorf("decode pet manifest: %w", err)
	}
	manifest.ID = normalizePetID(manifest.ID, filepath.Base(petDir))
	manifest.DisplayName = normalizePetDisplayName(manifest.DisplayName, manifest.Name, manifest.ID)
	sheetPath := filepath.Join(petDir, petSheetFileName)
	if width == 0 || height == 0 || validation == "" {
		sheetBytes, readErr := os.ReadFile(sheetPath)
		if readErr != nil {
			validationCode = petErrorCodePackageMissingSpritesheet
			validation = fmt.Sprintf("read pet spritesheet: %v", readErr)
		} else {
			width, height, validationCode, validation = validateSpritesheetBytes(sheetBytes)
		}
	}
	status := petStatusReady
	if validation != "" {
		status = petStatusInvalid
	}
	updatedAt := service.now().UTC()
	if stat, statErr := os.Stat(sheetPath); statErr == nil {
		updatedAt = stat.ModTime().UTC()
	}
	return dto.Pet{
		ID:                manifest.ID,
		DisplayName:       manifest.DisplayName,
		Description:       strings.TrimSpace(manifest.Description),
		FrameCount:        petFrameCount,
		Columns:           petColumns,
		Rows:              petRows,
		CellWidth:         petCellWidth,
		CellHeight:        petCellHeight,
		SpritesheetFile:   petSheetFileName,
		SpritesheetPath:   sheetPath,
		Scope:             scope,
		Status:            status,
		ValidationCode:    validationCode,
		ValidationMessage: validation,
		ImageWidth:        width,
		ImageHeight:       height,
		CreatedAt:         updatedAt.Format(time.RFC3339),
		UpdatedAt:         updatedAt.Format(time.RFC3339Nano),
	}, nil
}

func (service *Service) syncMetadataLocked(ctx context.Context, pets []dto.Pet) {
	if service.metadataRepo == nil {
		return
	}
	existing, err := service.metadataRepo.List(ctx)
	if err != nil {
		existing = nil
	}
	existingByID := make(map[string]dto.Pet, len(existing))
	for _, pet := range existing {
		existingByID[pet.ID] = pet
	}
	known := make(map[string]struct{}, len(pets))
	for index := range pets {
		if pets[index].Scope == scopeImported {
			if strings.TrimSpace(pets[index].Origin) == "" {
				pets[index].Origin = normalizePetOrigin(existingByID[pets[index].ID].Origin)
			} else {
				pets[index].Origin = normalizePetOrigin(pets[index].Origin)
			}
		} else {
			pets[index].Origin = strings.TrimSpace(pets[index].Origin)
		}
		known[pets[index].ID] = struct{}{}
		service.saveMetadataLocked(ctx, pets[index])
	}
	for _, pet := range existing {
		if _, ok := known[pet.ID]; !ok {
			_ = service.metadataRepo.Delete(ctx, pet.ID)
		}
	}
}

func (service *Service) saveMetadataLocked(ctx context.Context, pet dto.Pet) {
	if service.metadataRepo == nil {
		return
	}
	_ = service.metadataRepo.Save(ctx, pet)
}

func (service *Service) scopeDir(scope string) string {
	return filepath.Join(service.baseDir, scope)
}

func (service *Service) petDir(scope string, id string) string {
	return filepath.Join(service.scopeDir(scope), strings.TrimSpace(id))
}

func (service *Service) petDirExists(scope string, id string) bool {
	stat, err := os.Stat(service.petDir(scope, id))
	return err == nil && stat.IsDir()
}

func sortPets(pets []dto.Pet) {
	sort.SliceStable(pets, func(left int, right int) bool {
		if pets[left].Scope != pets[right].Scope {
			return pets[left].Scope == scopeBuiltin
		}
		if pets[left].Status != pets[right].Status {
			return pets[left].Status == petStatusReady
		}
		return strings.ToLower(pets[left].DisplayName) < strings.ToLower(pets[right].DisplayName)
	})
}

func decodeManifest(payload []byte) (petManifest, error) {
	var manifest petManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return petManifest{}, err
	}
	manifest.DisplayName = normalizePetDisplayName(manifest.DisplayName, manifest.Name, manifest.ID)
	manifest.Description = strings.TrimSpace(manifest.Description)
	if strings.TrimSpace(manifest.SpritesheetPath) == "" {
		manifest.SpritesheetPath = petSheetFileName
	}
	return manifest, nil
}

func encodeManifest(manifest petManifest) ([]byte, error) {
	manifest.ID = normalizePetID(manifest.ID, manifest.DisplayName)
	manifest.DisplayName = normalizePetDisplayName(manifest.DisplayName, manifest.Name, manifest.ID)
	manifest.Name = ""
	manifest.SpritesheetPath = petSheetFileName
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func writePetPackage(targetDir string, manifest petManifest, sheetBytes []byte) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("clear pet directory: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create pet directory: %w", err)
	}
	payload, err := encodeManifest(manifest)
	if err != nil {
		return fmt.Errorf("encode pet manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, petManifestFileName), payload, 0o644); err != nil {
		return fmt.Errorf("write pet manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, petSheetFileName), sheetBytes, 0o644); err != nil {
		return fmt.Errorf("write pet spritesheet: %w", err)
	}
	return nil
}

func validateSpritesheetBytes(payload []byte) (int, int, string, string) {
	img, err := nativewebp.Decode(bytes.NewReader(payload))
	if err != nil {
		return 0, 0, petErrorCodeSpritesheetDecodeFailed, fmt.Sprintf("decode pet spritesheet: %v", err)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width != petSheetWidth || height != petSheetHeight {
		return width, height, petErrorCodeSpritesheetSizeInvalid, fmt.Sprintf("pet spritesheet must be %dx%d, got %dx%d", petSheetWidth, petSheetHeight, width, height)
	}
	return width, height, "", ""
}

func readZipFileByBase(reader *zip.Reader, name string) ([]byte, error) {
	base := filepath.Base(filepath.ToSlash(name))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(filepath.ToSlash(file.Name)) != base {
			continue
		}
		if file.UncompressedSize64 > petMaxZipSizeBytes {
			return nil, newPetErrorf(petErrorCodePackageContentsTooLarge, "pet package contents exceed the %s limit", formatByteLimit(petMaxZipSizeBytes))
		}
		rc, err := file.Open()
		if err != nil {
			return nil, wrapPetError(petErrorCodeArchiveFileOpenFailed, err, "open archived pet file")
		}
		defer rc.Close()
		var buffer bytes.Buffer
		written, err := io.CopyN(&buffer, rc, petMaxZipSizeBytes+1)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, wrapPetError(petErrorCodeArchiveFileReadFailed, err, "read archived pet file")
		}
		if written > petMaxZipSizeBytes {
			return nil, newPetErrorf(petErrorCodePackageContentsTooLarge, "pet package contents exceed the %s limit", formatByteLimit(petMaxZipSizeBytes))
		}
		return buffer.Bytes(), nil
	}
	switch base {
	case petManifestFileName:
		return nil, newPetError(petErrorCodePackageMissingManifest, "pet package is missing pet.json")
	case petSheetFileName:
		return nil, newPetError(petErrorCodePackageMissingSpritesheet, "pet package is missing spritesheet.webp")
	default:
		return nil, newPetErrorf(petErrorCodePackageMissingSpritesheet, "pet package is missing %s", base)
	}
}

func addFileToZip(writer *zip.Writer, sourcePath string, archiveName string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open pet export file: %w", err)
	}
	defer source.Close()
	target, err := writer.Create(archiveName)
	if err != nil {
		return fmt.Errorf("create pet export entry: %w", err)
	}
	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("write pet export entry: %w", err)
	}
	return nil
}

func normalizePetDisplayName(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return "Pet"
}

func normalizePetID(values ...string) string {
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		trimmed = petIDPattern.ReplaceAllString(trimmed, "-")
		trimmed = strings.Trim(trimmed, ".-_")
		if trimmed != "" {
			if len(trimmed) > 64 {
				trimmed = strings.Trim(trimmed[:64], ".-_")
			}
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return uuid.NewString()
}

func normalizePetOrigin(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return petOriginLocal
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "direct", petOriginLocal:
		return petOriginLocal
	}
	if origin := resolvePetOriginHost(trimmed); origin != "" {
		return origin
	}
	return strings.TrimPrefix(lower, "www.")
}

func formatByteLimit(bytes int64) string {
	const mb = 1024 * 1024
	if bytes >= mb && bytes%mb == 0 {
		return fmt.Sprintf("%d MB", bytes/mb)
	}
	return fmt.Sprintf("%d bytes", bytes)
}
