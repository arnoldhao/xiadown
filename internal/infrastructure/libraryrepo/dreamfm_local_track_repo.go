package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type SQLiteDreamFMLocalTrackRepository struct{ db *bun.DB }

type dreamFMLocalTrackRow struct {
	bun.BaseModel  `bun:"table:dreamfm_local_tracks"`
	FileID         string         `bun:"file_id,pk"`
	LibraryID      string         `bun:"library_id"`
	LocalPath      string         `bun:"local_path"`
	Title          string         `bun:"title"`
	Author         sql.NullString `bun:"author"`
	CoverLocalPath sql.NullString `bun:"cover_local_path"`
	Format         sql.NullString `bun:"format"`
	AudioCodec     sql.NullString `bun:"audio_codec"`
	DurationMs     sql.NullInt64  `bun:"duration_ms"`
	SizeBytes      sql.NullInt64  `bun:"size_bytes"`
	ModTimeUnix    int64          `bun:"mod_time_unix"`
	Availability   string         `bun:"availability"`
	LastCheckedAt  time.Time      `bun:"last_checked_at"`
	ProbeError     sql.NullString `bun:"probe_error"`
	CreatedAt      time.Time      `bun:"created_at"`
	UpdatedAt      time.Time      `bun:"updated_at"`
}

func NewSQLiteDreamFMLocalTrackRepository(db *bun.DB) *SQLiteDreamFMLocalTrackRepository {
	return &SQLiteDreamFMLocalTrackRepository{db: db}
}

func (repo *SQLiteDreamFMLocalTrackRepository) List(ctx context.Context, options library.DreamFMLocalTrackListOptions) ([]library.DreamFMLocalTrack, error) {
	rows := make([]dreamFMLocalTrackRow, 0)
	query := repo.db.NewSelect().Model(&rows)
	if !options.IncludeUnavailable {
		query.Where("availability = ?", library.DreamFMLocalTrackAvailable)
	}
	if search := strings.ToLower(strings.TrimSpace(options.Query)); search != "" {
		like := "%" + search + "%"
		query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.
				WhereOr("LOWER(title) LIKE ?", like).
				WhereOr("LOWER(author) LIKE ?", like).
				WhereOr("LOWER(local_path) LIKE ?", like)
		})
	}
	query.Order("updated_at DESC", "title ASC")
	if options.Limit > 0 {
		query.Limit(options.Limit)
	}
	if options.Offset > 0 {
		query.Offset(options.Offset)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	return mapDreamFMLocalTrackRows(rows)
}

func (repo *SQLiteDreamFMLocalTrackRepository) Get(ctx context.Context, fileID string) (library.DreamFMLocalTrack, error) {
	row := new(dreamFMLocalTrackRow)
	if err := repo.db.NewSelect().Model(row).Where("file_id = ?", strings.TrimSpace(fileID)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.DreamFMLocalTrack{}, library.ErrFileNotFound
		}
		return library.DreamFMLocalTrack{}, err
	}
	return toDomainDreamFMLocalTrack(*row)
}

func (repo *SQLiteDreamFMLocalTrackRepository) Save(ctx context.Context, item library.DreamFMLocalTrack) error {
	row := dreamFMLocalTrackRow{
		FileID:         item.FileID,
		LibraryID:      item.LibraryID,
		LocalPath:      item.LocalPath,
		Title:          item.Title,
		Author:         nullString(item.Author),
		CoverLocalPath: nullString(item.CoverLocalPath),
		Format:         nullString(item.Format),
		AudioCodec:     nullString(item.AudioCodec),
		DurationMs:     nullInt64(item.DurationMs),
		SizeBytes:      nullInt64(item.SizeBytes),
		ModTimeUnix:    item.ModTimeUnix,
		Availability:   item.Availability,
		LastCheckedAt:  item.LastCheckedAt,
		ProbeError:     nullString(item.ProbeError),
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(file_id) DO UPDATE").
		Set("library_id = EXCLUDED.library_id").
		Set("local_path = EXCLUDED.local_path").
		Set("title = EXCLUDED.title").
		Set("author = EXCLUDED.author").
		Set("cover_local_path = EXCLUDED.cover_local_path").
		Set("format = EXCLUDED.format").
		Set("audio_codec = EXCLUDED.audio_codec").
		Set("duration_ms = EXCLUDED.duration_ms").
		Set("size_bytes = EXCLUDED.size_bytes").
		Set("mod_time_unix = EXCLUDED.mod_time_unix").
		Set("availability = EXCLUDED.availability").
		Set("last_checked_at = EXCLUDED.last_checked_at").
		Set("probe_error = EXCLUDED.probe_error").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteDreamFMLocalTrackRepository) Delete(ctx context.Context, fileID string) error {
	_, err := repo.db.NewDelete().Model((*dreamFMLocalTrackRow)(nil)).Where("file_id = ?", strings.TrimSpace(fileID)).Exec(ctx)
	return err
}

func (repo *SQLiteDreamFMLocalTrackRepository) DeleteUnavailable(ctx context.Context) (int, error) {
	result, err := repo.db.NewDelete().
		Model((*dreamFMLocalTrackRow)(nil)).
		Where("availability <> ?", library.DreamFMLocalTrackAvailable).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

func mapDreamFMLocalTrackRows(rows []dreamFMLocalTrackRow) ([]library.DreamFMLocalTrack, error) {
	result := make([]library.DreamFMLocalTrack, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainDreamFMLocalTrack(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func toDomainDreamFMLocalTrack(row dreamFMLocalTrackRow) (library.DreamFMLocalTrack, error) {
	return library.NewDreamFMLocalTrack(library.DreamFMLocalTrackParams{
		FileID:         row.FileID,
		LibraryID:      row.LibraryID,
		LocalPath:      row.LocalPath,
		Title:          row.Title,
		Author:         stringOrEmpty(row.Author),
		CoverLocalPath: stringOrEmpty(row.CoverLocalPath),
		Format:         stringOrEmpty(row.Format),
		AudioCodec:     stringOrEmpty(row.AudioCodec),
		DurationMs:     int64OrNil(row.DurationMs),
		SizeBytes:      int64OrNil(row.SizeBytes),
		ModTimeUnix:    row.ModTimeUnix,
		Availability:   row.Availability,
		LastCheckedAt:  &row.LastCheckedAt,
		ProbeError:     stringOrEmpty(row.ProbeError),
		CreatedAt:      &row.CreatedAt,
		UpdatedAt:      &row.UpdatedAt,
	})
}

func nullInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func int64OrNil(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	copyValue := value.Int64
	return &copyValue
}
