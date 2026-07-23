package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"xiadown/internal/application/pets/dto"
	"xiadown/internal/infrastructure/persistence"
	"xiadown/internal/infrastructure/petsrepo"
)

func TestEnsureBuiltinPetsSkipsUnchangedEmbeddedPackage(t *testing.T) {
	baseDir := t.TempDir()
	builtin := testBuiltinPetFS([]byte("not-a-real-webp"))
	repo := &petMetadataRepoStub{}
	first := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
	if err := first.EnsureBuiltinPets(context.Background()); err != nil {
		t.Fatalf("first EnsureBuiltinPets returned error: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("first ensure saved %d metadata rows, want 1", len(repo.saved))
	}
	sheetPath := filepath.Join(baseDir, scopeBuiltin, "gege", petSheetFileName)
	before, err := os.Stat(sheetPath)
	if err != nil {
		t.Fatalf("stat installed spritesheet: %v", err)
	}

	repo.saved = nil
	second := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
	if err := second.EnsureBuiltinPets(context.Background()); err != nil {
		t.Fatalf("second EnsureBuiltinPets returned error: %v", err)
	}
	after, err := os.Stat(sheetPath)
	if err != nil {
		t.Fatalf("stat unchanged spritesheet: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("unchanged embedded pet was rewritten")
	}
	if len(repo.saved) != 0 {
		t.Fatalf("unchanged ensure saved %d metadata rows, want 0", len(repo.saved))
	}
}

func TestPetMetadataTimestampComparisonUsesSQLitePrecision(t *testing.T) {
	left := dto.Pet{
		ID:        "gege",
		UpdatedAt: "2026-07-20T12:13:43.864360027Z",
	}
	right := left
	right.UpdatedAt = "2026-07-20T12:13:43.86436Z"

	if !samePetMetadata(left, right) {
		t.Fatal("timestamps representing the same SQLite microsecond should match")
	}

	right.UpdatedAt = "2026-07-20T12:13:43.864361Z"
	if samePetMetadata(left, right) {
		t.Fatal("timestamps from different microseconds must not match")
	}
}

func TestEnsureBuiltinPetsAcceptsSQLiteTimestampRoundTrip(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "pets.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	baseDir := t.TempDir()
	repo := petsrepo.NewSQLitePetRepository(database.Bun)
	builtin := testBuiltinPetFS([]byte("not-a-real-webp"))
	first := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
	if err := first.EnsureBuiltinPets(ctx); err != nil {
		t.Fatalf("first EnsureBuiltinPets returned error: %v", err)
	}

	// Emulate an install state written with filesystem nanoseconds by an older
	// build while SQLite has already rounded the same instant to microseconds.
	state, err := first.readBuiltinPetsInstallStateLocked()
	if err != nil {
		t.Fatalf("read built-in state: %v", err)
	}
	if len(state.Pets) != 1 {
		t.Fatalf("built-in state pet count = %d, want 1", len(state.Pets))
	}
	storedTime, err := time.Parse(time.RFC3339Nano, state.Pets[0].Metadata.UpdatedAt)
	if err != nil {
		t.Fatalf("parse state timestamp: %v", err)
	}
	state.Pets[0].Metadata.UpdatedAt = storedTime.Add(27 * time.Nanosecond).Format(time.RFC3339Nano)
	if err := first.writeBuiltinPetsInstallStateLocked(state); err != nil {
		t.Fatalf("write legacy-precision state: %v", err)
	}
	statePath := filepath.Join(baseDir, builtinStateFileName)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read legacy-precision state: %v", err)
	}

	second := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
	if err := second.EnsureBuiltinPets(ctx); err != nil {
		t.Fatalf("second EnsureBuiltinPets returned error: %v", err)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read retained state: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("SQLite timestamp rounding caused the unchanged built-in pet state to be rewritten")
	}
}

func TestEnsureBuiltinPetsRefreshesChangedOrMissingEmbeddedPackage(t *testing.T) {
	baseDir := t.TempDir()
	repo := &petMetadataRepoStub{}
	first := NewService(baseDir, testBuiltinPetFS([]byte("old-sheet")), "embedded/pets", "", WithMetadataRepository(repo))
	if err := first.EnsureBuiltinPets(context.Background()); err != nil {
		t.Fatalf("first EnsureBuiltinPets returned error: %v", err)
	}

	repo.saved = nil
	updatedSheet := []byte("new-sheet")
	second := NewService(baseDir, testBuiltinPetFS(updatedSheet), "embedded/pets", "", WithMetadataRepository(repo))
	if err := second.EnsureBuiltinPets(context.Background()); err != nil {
		t.Fatalf("updated EnsureBuiltinPets returned error: %v", err)
	}
	sheetPath := filepath.Join(baseDir, scopeBuiltin, "gege", petSheetFileName)
	payload, err := os.ReadFile(sheetPath)
	if err != nil {
		t.Fatalf("read refreshed spritesheet: %v", err)
	}
	if string(payload) != string(updatedSheet) {
		t.Fatalf("refreshed spritesheet = %q, want %q", payload, updatedSheet)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("changed ensure saved %d metadata rows, want 1", len(repo.saved))
	}

	if err := os.Remove(sheetPath); err != nil {
		t.Fatalf("remove installed spritesheet: %v", err)
	}
	repo.saved = nil
	third := NewService(baseDir, testBuiltinPetFS(updatedSheet), "embedded/pets", "", WithMetadataRepository(repo))
	if err := third.EnsureBuiltinPets(context.Background()); err != nil {
		t.Fatalf("repair EnsureBuiltinPets returned error: %v", err)
	}
	if _, err := os.Stat(sheetPath); err != nil {
		t.Fatalf("missing spritesheet was not repaired: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("repair ensure saved %d metadata rows, want 1", len(repo.saved))
	}
}

func TestEnsureBuiltinPetsRepairsSameSizeInstalledCorruption(t *testing.T) {
	baseDir := t.TempDir()
	sourceSheet := []byte("same-size-source")
	repo := &petMetadataRepoStub{}
	builtin := testBuiltinPetFS(sourceSheet)
	first := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
	if err := first.EnsureBuiltinPets(context.Background()); err != nil {
		t.Fatalf("first EnsureBuiltinPets returned error: %v", err)
	}

	sheetPath := filepath.Join(baseDir, scopeBuiltin, "gege", petSheetFileName)
	corrupted := append([]byte(nil), sourceSheet...)
	corrupted[0] ^= 0xff
	if err := os.WriteFile(sheetPath, corrupted, 0o644); err != nil {
		t.Fatalf("corrupt installed spritesheet: %v", err)
	}
	repo.saved = nil
	second := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
	if err := second.EnsureBuiltinPets(context.Background()); err != nil {
		t.Fatalf("repair EnsureBuiltinPets returned error: %v", err)
	}
	payload, err := os.ReadFile(sheetPath)
	if err != nil {
		t.Fatalf("read repaired spritesheet: %v", err)
	}
	if string(payload) != string(sourceSheet) {
		t.Fatalf("repaired spritesheet = %q, want %q", payload, sourceSheet)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("same-size corruption repair saved %d metadata rows, want 1", len(repo.saved))
	}
}

func TestEnsureBuiltinPetsRepairsMissingOrStaleMetadata(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*petMetadataRepoStub)
	}{
		{
			name: "missing",
			mutate: func(repo *petMetadataRepoStub) {
				repo.items = nil
			},
		},
		{
			name: "stale",
			mutate: func(repo *petMetadataRepoStub) {
				repo.items[0].DisplayName = "Stale metadata"
				repo.items[0].ImageWidth++
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseDir := t.TempDir()
			repo := &petMetadataRepoStub{}
			builtin := testBuiltinPetFS([]byte("metadata-test-sheet"))
			first := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
			if err := first.EnsureBuiltinPets(context.Background()); err != nil {
				t.Fatalf("first EnsureBuiltinPets returned error: %v", err)
			}
			if len(repo.items) != 1 {
				t.Fatalf("first ensure metadata count = %d, want 1", len(repo.items))
			}

			test.mutate(repo)
			repo.saved = nil
			second := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
			if err := second.EnsureBuiltinPets(context.Background()); err != nil {
				t.Fatalf("metadata repair EnsureBuiltinPets returned error: %v", err)
			}
			if len(repo.saved) != 1 {
				t.Fatalf("metadata repair saved %d rows, want 1", len(repo.saved))
			}
			if len(repo.items) != 1 || repo.items[0].DisplayName != "Gege" {
				t.Fatalf("metadata was not repaired: %#v", repo.items)
			}

			repo.saved = nil
			third := NewService(baseDir, builtin, "embedded/pets", "", WithMetadataRepository(repo))
			if err := third.EnsureBuiltinPets(context.Background()); err != nil {
				t.Fatalf("post-repair EnsureBuiltinPets returned error: %v", err)
			}
			if len(repo.saved) != 0 {
				t.Fatalf("stable metadata was saved %d times after repair, want 0", len(repo.saved))
			}
		})
	}
}

func testBuiltinPetFS(sheet []byte) fstest.MapFS {
	return fstest.MapFS{
		"embedded/pets/gege/pet.json":         &fstest.MapFile{Data: []byte(`{"id":"gege","displayName":"Gege","description":"Built in","spritesheetPath":"spritesheet.webp"}`)},
		"embedded/pets/gege/spritesheet.webp": &fstest.MapFile{Data: append([]byte(nil), sheet...)},
	}
}

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
	for index := range repo.items {
		if repo.items[index].ID == pet.ID {
			repo.items[index] = pet
			return nil
		}
	}
	repo.items = append(repo.items, pet)
	return nil
}

func (repo *petMetadataRepoStub) Delete(_ context.Context, id string) error {
	repo.deleted = append(repo.deleted, id)
	filtered := repo.items[:0]
	for _, pet := range repo.items {
		if pet.ID != id {
			filtered = append(filtered, pet)
		}
	}
	repo.items = filtered
	return nil
}
