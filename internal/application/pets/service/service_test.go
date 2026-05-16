package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/pets/dto"
)

func TestListPetsUsesMetadataCacheWithoutDecodingSpritesheet(t *testing.T) {
	baseDir := t.TempDir()
	petDir := filepath.Join(baseDir, scopeImported, "cached-pet")
	sheetPath, updatedAt := writeStoredPetPackage(t, petDir, "cached-pet", "Cached Pet", []byte("not a webp"))

	repo := &petMetadataRepoStub{
		items: []dto.Pet{{
			ID:              "cached-pet",
			DisplayName:     "Cached Pet",
			Description:     "cached metadata",
			FrameCount:      petFrameCount,
			Columns:         petColumns,
			Rows:            petRows,
			CellWidth:       petCellWidth,
			CellHeight:      petCellHeight,
			SpritesheetFile: petSheetFileName,
			SpritesheetPath: sheetPath,
			Origin:          petOriginLocal,
			Scope:           scopeImported,
			Status:          petStatusReady,
			ImageWidth:      petSheetWidth,
			ImageHeight:     petSheetHeight,
			UpdatedAt:       updatedAt,
		}},
	}
	service := NewService(baseDir, nil, "", "", WithMetadataRepository(repo))

	pets, err := service.ListPets(context.Background())
	if err != nil {
		t.Fatalf("ListPets returned error: %v", err)
	}
	if len(pets) != 1 {
		t.Fatalf("pet count = %d, want 1", len(pets))
	}
	pet := pets[0]
	if pet.Status != petStatusReady {
		t.Fatalf("pet status = %q, want ready", pet.Status)
	}
	if pet.ImageWidth != petSheetWidth || pet.ImageHeight != petSheetHeight {
		t.Fatalf("pet size = %dx%d, want %dx%d", pet.ImageWidth, pet.ImageHeight, petSheetWidth, petSheetHeight)
	}
	if len(repo.saved) != 0 {
		t.Fatalf("expected cached metadata to avoid a resave, saved %d pets", len(repo.saved))
	}
}

func TestListPetsUsesLegacyCreatedAtCacheTimestamp(t *testing.T) {
	baseDir := t.TempDir()
	petDir := filepath.Join(baseDir, scopeImported, "legacy-pet")
	sheetPath, updatedAt := writeStoredPetPackage(t, petDir, "legacy-pet", "Legacy Pet", []byte("not a webp"))

	repo := &petMetadataRepoStub{
		items: []dto.Pet{{
			ID:              "legacy-pet",
			DisplayName:     "Legacy Pet",
			Description:     "cached metadata",
			FrameCount:      petFrameCount,
			Columns:         petColumns,
			Rows:            petRows,
			CellWidth:       petCellWidth,
			CellHeight:      petCellHeight,
			SpritesheetFile: petSheetFileName,
			SpritesheetPath: sheetPath,
			Origin:          petOriginLocal,
			Scope:           scopeImported,
			Status:          petStatusReady,
			ImageWidth:      petSheetWidth,
			ImageHeight:     petSheetHeight,
			CreatedAt:       updatedAt,
			UpdatedAt:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339Nano),
		}},
	}
	service := NewService(baseDir, nil, "", "", WithMetadataRepository(repo))

	pets, err := service.ListPets(context.Background())
	if err != nil {
		t.Fatalf("ListPets returned error: %v", err)
	}
	if len(pets) != 1 {
		t.Fatalf("pet count = %d, want 1", len(pets))
	}
	if pets[0].Status != petStatusReady {
		t.Fatalf("pet status = %q, want ready", pets[0].Status)
	}
}

func TestListPetsRefreshesStaleMetadataCache(t *testing.T) {
	baseDir := t.TempDir()
	petDir := filepath.Join(baseDir, scopeImported, "stale-pet")
	sheetPath, _ := writeStoredPetPackage(t, petDir, "stale-pet", "Stale Pet", []byte("not a webp"))

	repo := &petMetadataRepoStub{
		items: []dto.Pet{{
			ID:              "stale-pet",
			DisplayName:     "Stale Pet",
			Description:     "cached metadata",
			FrameCount:      petFrameCount,
			Columns:         petColumns,
			Rows:            petRows,
			CellWidth:       petCellWidth,
			CellHeight:      petCellHeight,
			SpritesheetFile: petSheetFileName,
			SpritesheetPath: sheetPath,
			Origin:          petOriginLocal,
			Scope:           scopeImported,
			Status:          petStatusReady,
			ImageWidth:      petSheetWidth,
			ImageHeight:     petSheetHeight,
			UpdatedAt:       time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339Nano),
		}},
	}
	service := NewService(baseDir, nil, "", "", WithMetadataRepository(repo))

	pets, err := service.ListPets(context.Background())
	if err != nil {
		t.Fatalf("ListPets returned error: %v", err)
	}
	if len(pets) != 1 {
		t.Fatalf("pet count = %d, want 1", len(pets))
	}
	if pets[0].Status != petStatusInvalid {
		t.Fatalf("pet status = %q, want invalid", pets[0].Status)
	}
	if pets[0].ValidationCode != petErrorCodeSpritesheetDecodeFailed {
		t.Fatalf("validation code = %q, want %q", pets[0].ValidationCode, petErrorCodeSpritesheetDecodeFailed)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected stale metadata to be refreshed once, saved %d pets", len(repo.saved))
	}
}

func writeStoredPetPackage(t *testing.T, petDir string, id string, name string, sheetBytes []byte) (string, string) {
	t.Helper()
	if err := os.MkdirAll(petDir, 0o755); err != nil {
		t.Fatalf("create pet dir: %v", err)
	}
	manifest := []byte(`{"id":"` + id + `","displayName":"` + name + `","description":"cached metadata","spritesheetPath":"spritesheet.webp"}`)
	if err := os.WriteFile(filepath.Join(petDir, petManifestFileName), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	sheetPath := filepath.Join(petDir, petSheetFileName)
	if err := os.WriteFile(sheetPath, sheetBytes, 0o644); err != nil {
		t.Fatalf("write spritesheet: %v", err)
	}
	modTime := time.Date(2025, 5, 14, 9, 30, 0, 123456789, time.UTC)
	if err := os.Chtimes(sheetPath, modTime, modTime); err != nil {
		t.Fatalf("set spritesheet modtime: %v", err)
	}
	return sheetPath, modTime.Format(time.RFC3339Nano)
}

type petMetadataRepoStub struct {
	items   []dto.Pet
	saved   []dto.Pet
	deleted []string
}

func (repo *petMetadataRepoStub) List(context.Context) ([]dto.Pet, error) {
	return append([]dto.Pet(nil), repo.items...), nil
}

func (repo *petMetadataRepoStub) Save(_ context.Context, pet dto.Pet) error {
	repo.saved = append(repo.saved, pet)
	return nil
}

func (repo *petMetadataRepoStub) Delete(_ context.Context, id string) error {
	repo.deleted = append(repo.deleted, id)
	return nil
}
