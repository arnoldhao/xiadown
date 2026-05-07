package petsrepo

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"xiadown/internal/application/pets/dto"
	"xiadown/internal/infrastructure/persistence/sqlitedto"

	"github.com/uptrace/bun"
)

type SQLitePetRepository struct {
	db *bun.DB
}

type petRow = sqlitedto.PetRow

func NewSQLitePetRepository(db *bun.DB) *SQLitePetRepository {
	return &SQLitePetRepository{db: db}
}

func (repo *SQLitePetRepository) List(ctx context.Context) ([]dto.Pet, error) {
	rows := make([]petRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Scan(ctx); err != nil {
		return nil, err
	}

	result := make([]dto.Pet, 0, len(rows))
	for _, row := range rows {
		result = append(result, rowToPet(row))
	}
	return result, nil
}

func (repo *SQLitePetRepository) Save(ctx context.Context, pet dto.Pet) error {
	createdAt := parsePetCreatedAt(pet.CreatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	row := petRow{
		ID:                strings.TrimSpace(pet.ID),
		DisplayName:       strings.TrimSpace(pet.DisplayName),
		Description:       strings.TrimSpace(pet.Description),
		FrameCount:        pet.FrameCount,
		Columns:           pet.Columns,
		Rows:              pet.Rows,
		CellWidth:         pet.CellWidth,
		CellHeight:        pet.CellHeight,
		SpritesheetFile:   strings.TrimSpace(pet.SpritesheetFile),
		SpritesheetPath:   strings.TrimSpace(pet.SpritesheetPath),
		Origin:            strings.TrimSpace(pet.Origin),
		Scope:             strings.TrimSpace(pet.Scope),
		Status:            strings.TrimSpace(pet.Status),
		ValidationMessage: nullString(pet.ValidationMessage),
		ImageWidth:        pet.ImageWidth,
		ImageHeight:       pet.ImageHeight,
		CreatedAt:         createdAt,
		UpdatedAt:         time.Now().UTC(),
	}

	_, err := repo.db.NewInsert().
		Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("display_name = EXCLUDED.display_name").
		Set("description = EXCLUDED.description").
		Set("frame_count = EXCLUDED.frame_count").
		Set("columns = EXCLUDED.columns").
		Set("rows = EXCLUDED.rows").
		Set("cell_width = EXCLUDED.cell_width").
		Set("cell_height = EXCLUDED.cell_height").
		Set("spritesheet_file = EXCLUDED.spritesheet_file").
		Set("spritesheet_path = EXCLUDED.spritesheet_path").
		Set("origin = EXCLUDED.origin").
		Set("scope = EXCLUDED.scope").
		Set("status = EXCLUDED.status").
		Set("validation_message = EXCLUDED.validation_message").
		Set("image_width = EXCLUDED.image_width").
		Set("image_height = EXCLUDED.image_height").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLitePetRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().
		Model((*petRow)(nil)).
		Where("id = ?", strings.TrimSpace(id)).
		Exec(ctx)
	return err
}

func rowToPet(row petRow) dto.Pet {
	return dto.Pet{
		ID:                row.ID,
		DisplayName:       row.DisplayName,
		Description:       row.Description,
		FrameCount:        row.FrameCount,
		Columns:           row.Columns,
		Rows:              row.Rows,
		CellWidth:         row.CellWidth,
		CellHeight:        row.CellHeight,
		SpritesheetFile:   row.SpritesheetFile,
		SpritesheetPath:   row.SpritesheetPath,
		Origin:            row.Origin,
		Scope:             row.Scope,
		Status:            row.Status,
		ValidationMessage: stringOrEmpty(row.ValidationMessage),
		ImageWidth:        row.ImageWidth,
		ImageHeight:       row.ImageHeight,
		CreatedAt:         row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         row.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func parsePetCreatedAt(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func nullString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func stringOrEmpty(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
