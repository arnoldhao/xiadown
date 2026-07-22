package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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
	settingsdto "xiadown/internal/application/settings/dto"
)

const (
	petColumns           = 8
	petRows              = 9
	petCellWidth         = 192
	petCellHeight        = 208
	petSheetWidth        = petColumns * petCellWidth
	petSheetHeight       = petRows * petCellHeight
	petFrameCount        = petColumns * petRows
	petMaxZipSizeBytes   = 15 * 1024 * 1024
	petManifestFileName  = "pet.json"
	petSheetFileName     = "spritesheet.webp"
	builtinStateFileName = ".builtin-pets-state.json"
	builtinStateVersion  = 2

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

type embeddedBuiltinPet struct {
	entryName     string
	manifestBytes []byte
	sheetBytes    []byte
}

type builtinPetInstallState struct {
	ID             string  `json:"id"`
	ManifestSize   int64   `json:"manifestSize"`
	ManifestDigest string  `json:"manifestDigest"`
	SheetSize      int64   `json:"sheetSize"`
	SheetDigest    string  `json:"sheetDigest"`
	Metadata       dto.Pet `json:"metadata"`
}

type builtinPetsInstallState struct {
	Version int                      `json:"version"`
	Digest  string                   `json:"digest"`
	Pets    []builtinPetInstallState `json:"pets"`
}

type petLoadOptions struct {
	ValidationCode    string
	ValidationMessage string
	ImageWidth        int
	ImageHeight       int
	TrustValidation   bool
	Cached            *dto.Pet
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
	settingsReader SettingsReader
	networkGateway NetworkGatewayProvider
	builtinReady   bool
	now            func() time.Time
}

type SettingsReader interface {
	GetSettings(ctx context.Context) (settingsdto.Settings, error)
}

// NetworkGatewayProvider supplies the process-lifetime loopback proxy used by
// browser-backed features. The endpoint stays stable while its upstream policy
// changes, so a running Chromium profile never falls back to its own system
// route when XiaDown is in direct mode.
type NetworkGatewayProvider interface {
	ConsumerProxyURL() string
	ConsumerProxyAttestation() (string, string)
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

func WithSettingsReader(reader SettingsReader) Option {
	return func(service *Service) {
		service.settingsReader = reader
	}
}

func WithNetworkGateway(provider NetworkGatewayProvider) Option {
	return func(service *Service) {
		service.networkGateway = provider
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

	embedded, digest, err := service.readEmbeddedBuiltinPetsLocked()
	if err != nil {
		return err
	}
	if state, stateErr := service.readBuiltinPetsInstallStateLocked(); stateErr == nil &&
		state.Version == builtinStateVersion && state.Digest == digest &&
		service.builtinPetsInstallStateUsableLocked(ctx, state) {
		return nil
	}

	synced := make(map[string]struct{}, len(embedded))
	installed := make([]builtinPetInstallState, 0, len(embedded))
	for _, source := range embedded {
		pet, err := service.syncEmbeddedBuiltinPetLocked(source)
		if err != nil {
			return err
		}
		synced[pet.ID] = struct{}{}
		service.saveMetadataLocked(ctx, pet)
		manifestSize, manifestDigest, err := petFileIdentity(
			filepath.Join(service.petDir(scopeBuiltin, pet.ID), petManifestFileName),
		)
		if err != nil {
			return fmt.Errorf("identify installed pet manifest %s: %w", pet.ID, err)
		}
		sheetSize, sheetDigest, err := petFileIdentity(
			filepath.Join(service.petDir(scopeBuiltin, pet.ID), petSheetFileName),
		)
		if err != nil {
			return fmt.Errorf("identify installed pet spritesheet %s: %w", pet.ID, err)
		}
		installed = append(installed, builtinPetInstallState{
			ID:             pet.ID,
			ManifestSize:   manifestSize,
			ManifestDigest: manifestDigest,
			SheetSize:      sheetSize,
			SheetDigest:    sheetDigest,
			Metadata:       pet,
		})
	}
	if err := service.pruneStaleBuiltinPetsLocked(ctx, synced); err != nil {
		return err
	}
	return service.writeBuiltinPetsInstallStateLocked(builtinPetsInstallState{
		Version: builtinStateVersion, Digest: digest, Pets: installed,
	})
}

func (service *Service) readEmbeddedBuiltinPetsLocked() ([]embeddedBuiltinPet, string, error) {
	digest := sha256.New()
	result := make([]embeddedBuiltinPet, 0)
	if service.builtinFS == nil || service.builtinRoot == "" {
		return result, hex.EncodeToString(digest.Sum(nil)), nil
	}
	entries, err := fs.ReadDir(service.builtinFS, service.builtinRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return result, hex.EncodeToString(digest.Sum(nil)), nil
		}
		return nil, "", fmt.Errorf("read embedded pets: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourceDir := filepath.ToSlash(filepath.Join(service.builtinRoot, entry.Name()))
		manifestBytes, err := fs.ReadFile(service.builtinFS, filepath.ToSlash(filepath.Join(sourceDir, petManifestFileName)))
		if err != nil {
			return nil, "", fmt.Errorf("read embedded pet manifest %s: %w", entry.Name(), err)
		}
		sheetBytes, err := fs.ReadFile(service.builtinFS, filepath.ToSlash(filepath.Join(sourceDir, petSheetFileName)))
		if err != nil {
			return nil, "", fmt.Errorf("read embedded pet spritesheet %s: %w", entry.Name(), err)
		}
		for _, part := range [][]byte{[]byte(entry.Name()), manifestBytes, sheetBytes} {
			var size [8]byte
			binary.BigEndian.PutUint64(size[:], uint64(len(part)))
			_, _ = digest.Write(size[:])
			_, _ = digest.Write(part)
		}
		result = append(result, embeddedBuiltinPet{
			entryName: entry.Name(), manifestBytes: manifestBytes, sheetBytes: sheetBytes,
		})
	}
	return result, hex.EncodeToString(digest.Sum(nil)), nil
}

func (service *Service) syncEmbeddedBuiltinPetLocked(source embeddedBuiltinPet) (dto.Pet, error) {
	manifest, err := decodeManifest(source.manifestBytes)
	if err != nil {
		return dto.Pet{}, fmt.Errorf("decode embedded pet manifest %s: %w", source.entryName, err)
	}
	manifest.ID = normalizePetID(manifest.ID, source.entryName)
	manifest.DisplayName = normalizePetDisplayName(manifest.DisplayName, manifest.Name, manifest.ID)
	manifest.SpritesheetPath = petSheetFileName

	width, height, validationCode, validation := validateSpritesheetBytes(source.sheetBytes)
	targetDir := service.petDir(scopeBuiltin, manifest.ID)
	if err := writePetPackage(targetDir, manifest, source.sheetBytes); err != nil {
		return dto.Pet{}, err
	}
	return service.petFromStoredPackage(scopeBuiltin, targetDir, petLoadOptions{
		ValidationCode:    validationCode,
		ValidationMessage: validation,
		ImageWidth:        width,
		ImageHeight:       height,
		TrustValidation:   true,
	})
}

func (service *Service) readBuiltinPetsInstallStateLocked() (builtinPetsInstallState, error) {
	payload, err := os.ReadFile(filepath.Join(service.baseDir, builtinStateFileName))
	if err != nil {
		return builtinPetsInstallState{}, err
	}
	var state builtinPetsInstallState
	if err := json.Unmarshal(payload, &state); err != nil {
		return builtinPetsInstallState{}, err
	}
	return state, nil
}

func (service *Service) builtinPetsInstallStateUsableLocked(ctx context.Context, state builtinPetsInstallState) bool {
	metadataByID := make(map[string]dto.Pet)
	if service.metadataRepo != nil {
		items, err := service.metadataRepo.List(ctx)
		if err != nil {
			return false
		}
		metadataByID = make(map[string]dto.Pet, len(items))
		for _, item := range items {
			if strings.TrimSpace(item.ID) != "" {
				metadataByID[item.ID] = item
			}
		}
	}
	expected := make(map[string]builtinPetInstallState, len(state.Pets))
	for _, pet := range state.Pets {
		if strings.TrimSpace(pet.ID) == "" || filepath.Base(pet.ID) != pet.ID ||
			pet.ManifestSize <= 0 || strings.TrimSpace(pet.ManifestDigest) == "" ||
			pet.SheetSize <= 0 || strings.TrimSpace(pet.SheetDigest) == "" ||
			pet.Metadata.ID != pet.ID || pet.Metadata.Scope != scopeBuiltin {
			return false
		}
		if _, duplicate := expected[pet.ID]; duplicate {
			return false
		}
		expected[pet.ID] = pet
		manifestSize, manifestDigest, err := petFileIdentity(
			filepath.Join(service.petDir(scopeBuiltin, pet.ID), petManifestFileName),
		)
		if err != nil || manifestSize != pet.ManifestSize || manifestDigest != pet.ManifestDigest {
			return false
		}
		sheetSize, sheetDigest, err := petFileIdentity(
			filepath.Join(service.petDir(scopeBuiltin, pet.ID), petSheetFileName),
		)
		if err != nil || sheetSize != pet.SheetSize || sheetDigest != pet.SheetDigest {
			return false
		}
		if service.metadataRepo != nil {
			stored, ok := metadataByID[pet.ID]
			if !ok || !samePetMetadata(stored, pet.Metadata) {
				return false
			}
		}
	}
	entries, err := os.ReadDir(service.scopeDir(scopeBuiltin))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := expected[entry.Name()]; !ok {
			return false
		}
		delete(expected, entry.Name())
	}
	return len(expected) == 0
}

func petFileIdentity(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, "", err
	}
	if !info.Mode().IsRegular() {
		return 0, "", fmt.Errorf("pet asset is not a regular file")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return 0, "", err
	}
	return info.Size(), hex.EncodeToString(digest.Sum(nil)), nil
}

func (service *Service) writeBuiltinPetsInstallStateLocked(state builtinPetsInstallState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode built-in pets state: %w", err)
	}
	temporary, err := os.CreateTemp(service.baseDir, ".builtin-pets-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create built-in pets state: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set built-in pets state permissions: %w", err)
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write built-in pets state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close built-in pets state: %w", err)
	}
	if err := os.Rename(temporaryPath, filepath.Join(service.baseDir, builtinStateFileName)); err != nil {
		return fmt.Errorf("install built-in pets state: %w", err)
	}
	return nil
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
	pet, err := service.petFromStoredPackage(scopeImported, targetDir, petLoadOptions{
		ImageWidth:      inspection.imageWidth,
		ImageHeight:     inspection.imageHeight,
		TrustValidation: true,
	})
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
	cachedPets := service.cachedMetadataByIDLocked(context.Background())
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
			cached := cachedPets[entry.Name()]
			pet, err := service.petFromStoredPackage(scope, service.petDir(scope, entry.Name()), petLoadOptions{
				Cached: cached,
			})
			if err != nil {
				continue
			}
			pets = append(pets, pet)
		}
	}
	return pets, nil
}

func (service *Service) petFromStoredPackage(scope string, petDir string, options petLoadOptions) (dto.Pet, error) {
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
	width := options.ImageWidth
	height := options.ImageHeight
	validationCode := strings.TrimSpace(options.ValidationCode)
	validation := strings.TrimSpace(options.ValidationMessage)
	updatedAt := service.now().UTC()
	if stat, statErr := os.Stat(sheetPath); statErr == nil {
		updatedAt = stat.ModTime().UTC()
	}
	// SQLite persists timestamps with microsecond precision. Canonicalising the
	// filesystem timestamp before it enters the install state keeps a repository
	// round-trip from making an unchanged built-in pet look stale on every launch.
	updatedAt = updatedAt.Truncate(time.Microsecond)
	updatedAtValue := updatedAt.Format(time.RFC3339Nano)
	if !options.TrustValidation {
		if cachedPetUsable(options.Cached, manifest.ID, scope, sheetPath, updatedAtValue) {
			width = options.Cached.ImageWidth
			height = options.Cached.ImageHeight
			validationCode = strings.TrimSpace(options.Cached.ValidationCode)
			validation = strings.TrimSpace(options.Cached.ValidationMessage)
		} else {
			sheetBytes, readErr := os.ReadFile(sheetPath)
			if readErr != nil {
				width = 0
				height = 0
				validationCode = petErrorCodePackageMissingSpritesheet
				validation = fmt.Sprintf("read pet spritesheet: %v", readErr)
			} else {
				width, height, validationCode, validation = validateSpritesheetBytes(sheetBytes)
			}
		}
	}
	status := petStatusReady
	if validation != "" {
		status = petStatusInvalid
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
		UpdatedAt:         updatedAtValue,
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
		if !samePetMetadata(existingByID[pets[index].ID], pets[index]) {
			service.saveMetadataLocked(ctx, pets[index])
		}
	}
	for _, pet := range existing {
		if _, ok := known[pet.ID]; !ok {
			_ = service.metadataRepo.Delete(ctx, pet.ID)
		}
	}
}

func (service *Service) cachedMetadataByIDLocked(ctx context.Context) map[string]*dto.Pet {
	if service.metadataRepo == nil {
		return nil
	}
	existing, err := service.metadataRepo.List(ctx)
	if err != nil || len(existing) == 0 {
		return nil
	}
	result := make(map[string]*dto.Pet, len(existing))
	for index := range existing {
		if strings.TrimSpace(existing[index].ID) == "" {
			continue
		}
		result[existing[index].ID] = &existing[index]
	}
	return result
}

func cachedPetUsable(cached *dto.Pet, id string, scope string, sheetPath string, updatedAt string) bool {
	if cached == nil {
		return false
	}
	if cached.ID != id || cached.Scope != scope {
		return false
	}
	if !samePetPath(cached.SpritesheetPath, sheetPath) {
		return false
	}
	if !samePetTimestamp(cached.UpdatedAt, updatedAt) &&
		!samePetTimestamp(cached.CreatedAt, updatedAt) {
		return false
	}
	if cached.FrameCount != petFrameCount ||
		cached.Columns != petColumns ||
		cached.Rows != petRows ||
		cached.CellWidth != petCellWidth ||
		cached.CellHeight != petCellHeight ||
		cached.SpritesheetFile != petSheetFileName {
		return false
	}
	if cached.Status == petStatusReady {
		return cached.ImageWidth > 0 && cached.ImageHeight > 0
	}
	return cached.Status == petStatusInvalid && strings.TrimSpace(cached.ValidationMessage) != ""
}

func samePetMetadata(left dto.Pet, right dto.Pet) bool {
	return left.ID == right.ID &&
		left.DisplayName == right.DisplayName &&
		left.Description == right.Description &&
		left.FrameCount == right.FrameCount &&
		left.Columns == right.Columns &&
		left.Rows == right.Rows &&
		left.CellWidth == right.CellWidth &&
		left.CellHeight == right.CellHeight &&
		left.SpritesheetFile == right.SpritesheetFile &&
		samePetPath(left.SpritesheetPath, right.SpritesheetPath) &&
		normalizePetOrigin(left.Origin) == normalizePetOrigin(right.Origin) &&
		left.Scope == right.Scope &&
		left.Status == right.Status &&
		strings.TrimSpace(left.ValidationCode) == strings.TrimSpace(right.ValidationCode) &&
		strings.TrimSpace(left.ValidationMessage) == strings.TrimSpace(right.ValidationMessage) &&
		left.ImageWidth == right.ImageWidth &&
		left.ImageHeight == right.ImageHeight &&
		samePetTimestamp(left.UpdatedAt, right.UpdatedAt)
}

func samePetTimestamp(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == right {
		return true
	}
	leftTime, leftErr := time.Parse(time.RFC3339Nano, left)
	rightTime, rightErr := time.Parse(time.RFC3339Nano, right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return leftTime.UTC().Truncate(time.Microsecond).Equal(
		rightTime.UTC().Truncate(time.Microsecond),
	)
}

func samePetPath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if left == right {
		return true
	}
	return strings.EqualFold(left, right)
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
