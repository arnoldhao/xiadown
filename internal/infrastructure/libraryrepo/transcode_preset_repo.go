package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/persistence/sqlitedto"
)

type SQLiteTranscodePresetRepository struct {
	db *bun.DB
}

type transcodePresetRow = sqlitedto.TranscodePresetRow

func NewSQLiteTranscodePresetRepository(db *bun.DB) *SQLiteTranscodePresetRepository {
	return &SQLiteTranscodePresetRepository{db: db}
}

func (repo *SQLiteTranscodePresetRepository) List(ctx context.Context) ([]library.TranscodePreset, error) {
	rows := make([]transcodePresetRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Order("is_builtin DESC").
		Order("name ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.TranscodePreset, 0, len(rows))
	for _, row := range rows {
		item, err := library.NewTranscodePreset(library.TranscodePresetParams{
			ID:               row.ID,
			Name:             row.Name,
			OutputType:       row.OutputType,
			Container:        row.Container,
			VideoCodec:       stringOrEmpty(row.VideoCodec),
			AudioCodec:       stringOrEmpty(row.AudioCodec),
			QualityMode:      stringOrEmpty(row.QualityMode),
			CRF:              intOrZero(row.CRF),
			BitrateKbps:      intOrZero(row.BitrateKbps),
			AudioBitrateKbps: intOrZero(row.AudioBitrateKbps),
			Scale:            stringOrEmpty(row.Scale),
			Width:            intOrZero(row.Width),
			Height:           intOrZero(row.Height),
			FFmpegPreset:     stringOrEmpty(row.FFmpegPreset),
			AllowUpscale:     row.AllowUpscale,
			RequiresVideo:    row.RequiresVideo,
			RequiresAudio:    row.RequiresAudio,
			IsBuiltin:        row.IsBuiltin,
			Description:      stringOrEmpty(row.Description),
			CreatedAt:        &row.CreatedAt,
			UpdatedAt:        &row.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteTranscodePresetRepository) Get(ctx context.Context, id string) (library.TranscodePreset, error) {
	row := new(transcodePresetRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.TranscodePreset{}, library.ErrPresetNotFound
		}
		return library.TranscodePreset{}, err
	}
	return library.NewTranscodePreset(library.TranscodePresetParams{
		ID:               row.ID,
		Name:             row.Name,
		OutputType:       row.OutputType,
		Container:        row.Container,
		VideoCodec:       stringOrEmpty(row.VideoCodec),
		AudioCodec:       stringOrEmpty(row.AudioCodec),
		QualityMode:      stringOrEmpty(row.QualityMode),
		CRF:              intOrZero(row.CRF),
		BitrateKbps:      intOrZero(row.BitrateKbps),
		AudioBitrateKbps: intOrZero(row.AudioBitrateKbps),
		Scale:            stringOrEmpty(row.Scale),
		Width:            intOrZero(row.Width),
		Height:           intOrZero(row.Height),
		FFmpegPreset:     stringOrEmpty(row.FFmpegPreset),
		AllowUpscale:     row.AllowUpscale,
		RequiresVideo:    row.RequiresVideo,
		RequiresAudio:    row.RequiresAudio,
		IsBuiltin:        row.IsBuiltin,
		Description:      stringOrEmpty(row.Description),
		CreatedAt:        &row.CreatedAt,
		UpdatedAt:        &row.UpdatedAt,
	})
}

func (repo *SQLiteTranscodePresetRepository) Save(ctx context.Context, preset library.TranscodePreset) error {
	createdAt := preset.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := preset.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	row := transcodePresetRow{
		ID:               preset.ID,
		Name:             preset.Name,
		OutputType:       string(preset.OutputType),
		Container:        preset.Container,
		VideoCodec:       nullString(preset.VideoCodec),
		AudioCodec:       nullString(preset.AudioCodec),
		QualityMode:      nullString(preset.QualityMode),
		CRF:              nullInt(preset.CRF),
		BitrateKbps:      nullInt(preset.BitrateKbps),
		AudioBitrateKbps: nullInt(preset.AudioBitrateKbps),
		Scale:            nullString(preset.Scale),
		Width:            nullInt(preset.Width),
		Height:           nullInt(preset.Height),
		FFmpegPreset:     nullString(preset.FFmpegPreset),
		AllowUpscale:     preset.AllowUpscale,
		RequiresVideo:    preset.RequiresVideo,
		RequiresAudio:    preset.RequiresAudio,
		IsBuiltin:        preset.IsBuiltin,
		Description:      nullString(preset.Description),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("output_type = EXCLUDED.output_type").
		Set("container = EXCLUDED.container").
		Set("video_codec = EXCLUDED.video_codec").
		Set("audio_codec = EXCLUDED.audio_codec").
		Set("quality_mode = EXCLUDED.quality_mode").
		Set("crf = EXCLUDED.crf").
		Set("bitrate_kbps = EXCLUDED.bitrate_kbps").
		Set("audio_bitrate_kbps = EXCLUDED.audio_bitrate_kbps").
		Set("scale = EXCLUDED.scale").
		Set("width = EXCLUDED.width").
		Set("height = EXCLUDED.height").
		Set("ffmpeg_preset = EXCLUDED.ffmpeg_preset").
		Set("allow_upscale = EXCLUDED.allow_upscale").
		Set("requires_video = EXCLUDED.requires_video").
		Set("requires_audio = EXCLUDED.requires_audio").
		Set("is_builtin = EXCLUDED.is_builtin").
		Set("description = EXCLUDED.description").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteTranscodePresetRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*transcodePresetRow)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func nullInt(value int) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}

func intOrZero(value sql.NullInt64) int {
	if value.Valid {
		return int(value.Int64)
	}
	return 0
}
