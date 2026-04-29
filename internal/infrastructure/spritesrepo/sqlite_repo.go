package spritesrepo

import (
	"context"
	"database/sql"
	"encoding/base64"
	"strings"
	"time"

	"xiadown/internal/application/sprites/dto"
	"xiadown/internal/infrastructure/persistence/sqlitedto"

	"github.com/uptrace/bun"
)

type SQLiteSpriteRepository struct {
	db *bun.DB
}

type spriteRow = sqlitedto.SpriteRow

func NewSQLiteSpriteRepository(db *bun.DB) *SQLiteSpriteRepository {
	return &SQLiteSpriteRepository{db: db}
}

func (repo *SQLiteSpriteRepository) List(ctx context.Context) ([]dto.Sprite, error) {
	rows := make([]spriteRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Scan(ctx); err != nil {
		return nil, err
	}

	result := make([]dto.Sprite, 0, len(rows))
	for _, row := range rows {
		result = append(result, rowToSprite(row))
	}
	return result, nil
}

func (repo *SQLiteSpriteRepository) Save(ctx context.Context, sprite dto.Sprite, coverPNG []byte) error {
	createdAt := parseSpriteCreatedAt(sprite.CreatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	row := spriteRow{
		ID:                strings.TrimSpace(sprite.ID),
		Name:              strings.TrimSpace(sprite.Name),
		Description:       strings.TrimSpace(sprite.Description),
		FrameCount:        sprite.FrameCount,
		DeprecatedWidth:   0,
		DeprecatedHeight:  0,
		Columns:           sprite.Columns,
		Rows:              sprite.Rows,
		SpriteFile:        strings.TrimSpace(sprite.SpriteFile),
		SpritePath:        strings.TrimSpace(sprite.SpritePath),
		SourceType:        strings.TrimSpace(sprite.SourceType),
		Origin:            strings.TrimSpace(sprite.Origin),
		Scope:             strings.TrimSpace(sprite.Scope),
		Status:            strings.TrimSpace(sprite.Status),
		ValidationMessage: nullString(sprite.ValidationMessage),
		ImageWidth:        sprite.ImageWidth,
		ImageHeight:       sprite.ImageHeight,
		AuthorID:          strings.TrimSpace(sprite.Author.ID),
		AuthorDisplayName: strings.TrimSpace(sprite.Author.DisplayName),
		CreatedAt:         createdAt,
		Version:           strings.TrimSpace(sprite.Version),
		CoverPNG:          append([]byte(nil), coverPNG...),
		UpdatedAt:         time.Now().UTC(),
	}

	_, err := repo.db.NewInsert().
		Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("description = EXCLUDED.description").
		Set("frame_count = EXCLUDED.frame_count").
		Set("frame_width = EXCLUDED.frame_width").
		Set("frame_height = EXCLUDED.frame_height").
		Set("columns = EXCLUDED.columns").
		Set("rows = EXCLUDED.rows").
		Set("sprite_file = EXCLUDED.sprite_file").
		Set("sprite_path = EXCLUDED.sprite_path").
		Set("source_type = EXCLUDED.source_type").
		Set("origin = EXCLUDED.origin").
		Set("scope = EXCLUDED.scope").
		Set("status = EXCLUDED.status").
		Set("validation_message = EXCLUDED.validation_message").
		Set("image_width = EXCLUDED.image_width").
		Set("image_height = EXCLUDED.image_height").
		Set("author_id = EXCLUDED.author_id").
		Set("author_display_name = EXCLUDED.author_display_name").
		Set("created_at = EXCLUDED.created_at").
		Set("version = EXCLUDED.version").
		Set("cover_png = EXCLUDED.cover_png").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteSpriteRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().
		Model((*spriteRow)(nil)).
		Where("id = ?", strings.TrimSpace(id)).
		Exec(ctx)
	return err
}

func rowToSprite(row spriteRow) dto.Sprite {
	return dto.Sprite{
		ID:                row.ID,
		Name:              row.Name,
		Description:       row.Description,
		FrameCount:        row.FrameCount,
		Columns:           row.Columns,
		Rows:              row.Rows,
		SpriteFile:        row.SpriteFile,
		SpritePath:        row.SpritePath,
		SourceType:        row.SourceType,
		Origin:            row.Origin,
		Scope:             row.Scope,
		Status:            row.Status,
		ValidationMessage: stringOrEmpty(row.ValidationMessage),
		ImageWidth:        row.ImageWidth,
		ImageHeight:       row.ImageHeight,
		Author: dto.SpriteAuthor{
			ID:          row.AuthorID,
			DisplayName: row.AuthorDisplayName,
		},
		CreatedAt:         row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         row.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Version:           row.Version,
		CoverImageDataURL: pngDataToDataURI(row.CoverPNG),
	}
}

func parseSpriteCreatedAt(value string) time.Time {
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

func pngDataToDataURI(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(payload)
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
