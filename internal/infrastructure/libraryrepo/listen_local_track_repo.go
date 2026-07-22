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

type SQLiteListenLocalTrackRepository struct{ db *bun.DB }

type listenLocalTrackRow struct {
	bun.BaseModel            `bun:"table:listen_local_tracks"`
	FileID                   string         `bun:"file_id,pk"`
	LibraryID                string         `bun:"library_id"`
	Revision                 int64          `bun:"revision"`
	ContentIdentityRevision  int64          `bun:"content_identity_revision"`
	ContentIdentitySignature string         `bun:"content_identity_signature"`
	MetadataRevision         int64          `bun:"metadata_revision"`
	ResourceRevision         int64          `bun:"resource_revision"`
	LocalPath                string         `bun:"local_path"`
	Title                    string         `bun:"title"`
	Author                   sql.NullString `bun:"author"`
	Album                    sql.NullString `bun:"album"`
	AlbumArtist              sql.NullString `bun:"album_artist"`
	Genre                    sql.NullString `bun:"genre"`
	TrackNumber              sql.NullInt64  `bun:"track_number"`
	DiscNumber               sql.NullInt64  `bun:"disc_number"`
	Year                     sql.NullInt64  `bun:"year"`
	CoverLocalPath           sql.NullString `bun:"cover_local_path"`
	Format                   sql.NullString `bun:"format"`
	AudioCodec               sql.NullString `bun:"audio_codec"`
	DurationMs               sql.NullInt64  `bun:"duration_ms"`
	SizeBytes                sql.NullInt64  `bun:"size_bytes"`
	ModTimeUnix              int64          `bun:"mod_time_unix"`
	Availability             string         `bun:"availability"`
	LastCheckedAt            time.Time      `bun:"last_checked_at"`
	ProbeError               sql.NullString `bun:"probe_error"`
	CreatedAt                time.Time      `bun:"created_at"`
	UpdatedAt                time.Time      `bun:"updated_at"`
}

func NewSQLiteListenLocalTrackRepository(db *bun.DB) *SQLiteListenLocalTrackRepository {
	return &SQLiteListenLocalTrackRepository{db: db}
}

func (repo *SQLiteListenLocalTrackRepository) List(ctx context.Context, options library.ListenLocalTrackListOptions) ([]library.ListenLocalTrack, error) {
	rows := make([]listenLocalTrackRow, 0)
	query := repo.db.NewSelect().Model(&rows)
	if len(options.FileIDs) > 0 {
		fileIDs := make([]string, 0, len(options.FileIDs))
		seen := make(map[string]struct{}, len(options.FileIDs))
		for _, rawFileID := range options.FileIDs {
			fileID := strings.TrimSpace(rawFileID)
			if fileID == "" {
				continue
			}
			if _, exists := seen[fileID]; exists {
				continue
			}
			seen[fileID] = struct{}{}
			fileIDs = append(fileIDs, fileID)
		}
		if len(fileIDs) == 0 {
			query.Where("1 = 0")
		} else {
			query.Where("file_id IN (?)", bun.In(fileIDs))
		}
	}
	if !options.IncludeUnavailable {
		query.Where("availability = ?", library.ListenLocalTrackAvailable)
	}
	if search := strings.ToLower(strings.TrimSpace(options.Query)); search != "" {
		like := "%" + search + "%"
		query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.
				WhereOr("LOWER(title) LIKE ?", like).
				WhereOr("LOWER(author) LIKE ?", like).
				WhereOr("LOWER(album) LIKE ?", like).
				WhereOr("LOWER(album_artist) LIKE ?", like).
				WhereOr("LOWER(genre) LIKE ?", like).
				WhereOr("LOWER(local_path) LIKE ?", like)
		})
	}
	if artist := strings.TrimSpace(options.Artist); artist != "" {
		query.Where("LOWER(COALESCE(NULLIF(album_artist, ''), author, '')) = LOWER(?)", artist)
	}
	if album := strings.TrimSpace(options.Album); album != "" {
		query.Where("LOWER(COALESCE(album, '')) = LOWER(?)", album)
	}
	orderBy := func(expressions ...string) {
		for _, expression := range expressions {
			// These are fixed repository expressions, not caller input. OrderExpr
			// is required because Bun's Order parser treats COLLATE as an invalid
			// sort direction.
			query.OrderExpr(expression)
		}
	}
	switch strings.ToLower(strings.TrimSpace(options.Sort)) {
	case "recently_added":
		orderBy("created_at DESC", "title COLLATE NOCASE ASC")
	case "artists":
		orderBy("COALESCE(NULLIF(album_artist, ''), author, '') COLLATE NOCASE ASC", "album COLLATE NOCASE ASC", "disc_number ASC", "track_number ASC", "title COLLATE NOCASE ASC")
	case "albums":
		orderBy("album COLLATE NOCASE ASC", "disc_number ASC", "track_number ASC", "title COLLATE NOCASE ASC")
	case "songs":
		orderBy("title COLLATE NOCASE ASC", "author COLLATE NOCASE ASC")
	default:
		orderBy("mod_time_unix DESC", "title COLLATE NOCASE ASC")
	}
	if options.Limit > 0 {
		query.Limit(options.Limit)
	}
	if options.Offset > 0 {
		query.Offset(options.Offset)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	return mapListenLocalTrackRows(rows)
}

func (repo *SQLiteListenLocalTrackRepository) Get(ctx context.Context, fileID string) (library.ListenLocalTrack, error) {
	row := new(listenLocalTrackRow)
	if err := repo.db.NewSelect().Model(row).Where("file_id = ?", strings.TrimSpace(fileID)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ListenLocalTrack{}, library.ErrFileNotFound
		}
		return library.ListenLocalTrack{}, err
	}
	return toDomainListenLocalTrack(*row)
}

func (repo *SQLiteListenLocalTrackRepository) Save(ctx context.Context, item library.ListenLocalTrack) error {
	row := listenLocalTrackRow{
		FileID:                   item.FileID,
		LibraryID:                item.LibraryID,
		Revision:                 item.Revision,
		ContentIdentityRevision:  item.ContentIdentityRevision,
		ContentIdentitySignature: item.ContentIdentitySignature,
		MetadataRevision:         item.MetadataRevision,
		ResourceRevision:         item.ResourceRevision,
		LocalPath:                item.LocalPath,
		Title:                    item.Title,
		Author:                   nullString(item.Author),
		Album:                    nullString(item.Album),
		AlbumArtist:              nullString(item.AlbumArtist),
		Genre:                    nullString(item.Genre),
		TrackNumber:              nullPositiveInt(item.TrackNumber),
		DiscNumber:               nullPositiveInt(item.DiscNumber),
		Year:                     nullPositiveInt(item.Year),
		CoverLocalPath:           nullString(item.CoverLocalPath),
		Format:                   nullString(item.Format),
		AudioCodec:               nullString(item.AudioCodec),
		DurationMs:               nullInt64(item.DurationMs),
		SizeBytes:                nullInt64(item.SizeBytes),
		ModTimeUnix:              item.ModTimeUnix,
		Availability:             item.Availability,
		LastCheckedAt:            item.LastCheckedAt,
		ProbeError:               nullString(item.ProbeError),
		CreatedAt:                item.CreatedAt,
		UpdatedAt:                item.UpdatedAt,
	}
	if row.Revision < 1 {
		row.Revision = 1
	}
	if row.ContentIdentityRevision < 1 {
		row.ContentIdentityRevision = 1
	}
	if row.MetadataRevision < 1 {
		row.MetadataRevision = 1
	}
	if row.ResourceRevision < 1 {
		row.ResourceRevision = 1
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		existing := new(listenLocalTrackRow)
		findErr := tx.NewSelect().Model(existing).Where("file_id = ?", row.FileID).Scan(ctx)
		isNew := errors.Is(findErr, sql.ErrNoRows)
		if findErr != nil && !isNew {
			return findErr
		}
		changed := isNew
		if isNew {
			tombstoneRevision, tombstoneContentIdentity, tombstoneMetadata, tombstoneResource, err := listenLocalTrackTombstoneRevisions(ctx, tx, row.FileID)
			if err != nil {
				return err
			}
			row.Revision = max(row.Revision, tombstoneRevision+1, 1)
			row.ContentIdentityRevision = max(row.ContentIdentityRevision, tombstoneContentIdentity, 1)
			row.MetadataRevision = max(row.MetadataRevision, tombstoneMetadata, 1)
			row.ResourceRevision = max(row.ResourceRevision, tombstoneResource, 1)
		} else {
			row.CreatedAt = existing.CreatedAt
			row.ContentIdentityRevision = max(row.ContentIdentityRevision, existing.ContentIdentityRevision)
			row.MetadataRevision = max(row.MetadataRevision, existing.MetadataRevision)
			row.ResourceRevision = max(row.ResourceRevision, existing.ResourceRevision)
			if listenLocalTrackTimelineChanged(*existing, row) && row.ContentIdentityRevision == existing.ContentIdentityRevision {
				row.ContentIdentityRevision++
			}
			if listenLocalTrackMetadataChanged(*existing, row) && row.MetadataRevision == existing.MetadataRevision {
				row.MetadataRevision++
			}
			if listenLocalTrackResourcesChanged(*existing, row) && row.ResourceRevision == existing.ResourceRevision {
				row.ResourceRevision++
			}
			changed = listenLocalTrackObservableChanged(*existing, row)
			row.Revision = existing.Revision
			if changed {
				row.Revision++
			} else {
				// Probe bookkeeping is intentionally outside the paired entity
				// revision. Keep the public entity timestamp stable as well so a
				// no-op cannot become an unjournaled observable change.
				row.UpdatedAt = existing.UpdatedAt
			}
		}
		if err := upsertListenLocalTrackRow(ctx, tx, &row); err != nil {
			return err
		}
		if !changed {
			return nil
		}
		if err := clearListenLocalTombstone(ctx, tx, listenLocalEntityTrack, row.FileID); err != nil {
			return err
		}
		return appendListenLocalChange(ctx, tx, listenLocalEntityTrack, row.FileID, "upsert", row.Revision, row.UpdatedAt)
	})
}

func (repo *SQLiteListenLocalTrackRepository) Delete(ctx context.Context, fileID string) error {
	_, err := repo.db.NewDelete().Model((*listenLocalTrackRow)(nil)).Where("file_id = ?", strings.TrimSpace(fileID)).Exec(ctx)
	return err
}

func (repo *SQLiteListenLocalTrackRepository) DeleteUnavailable(ctx context.Context) (int, error) {
	result, err := repo.db.NewDelete().
		Model((*listenLocalTrackRow)(nil)).
		Where("availability <> ?", library.ListenLocalTrackAvailable).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

func upsertListenLocalTrackRow(ctx context.Context, tx bun.Tx, row *listenLocalTrackRow) error {
	_, err := tx.NewInsert().Model(row).
		On("CONFLICT(file_id) DO UPDATE").
		Set("library_id = EXCLUDED.library_id").
		Set("revision = EXCLUDED.revision").
		Set("content_identity_revision = EXCLUDED.content_identity_revision").
		Set("content_identity_signature = EXCLUDED.content_identity_signature").
		Set("metadata_revision = EXCLUDED.metadata_revision").
		Set("resource_revision = EXCLUDED.resource_revision").
		Set("local_path = EXCLUDED.local_path").
		Set("title = EXCLUDED.title").
		Set("author = EXCLUDED.author").
		Set("album = EXCLUDED.album").
		Set("album_artist = EXCLUDED.album_artist").
		Set("genre = EXCLUDED.genre").
		Set("track_number = EXCLUDED.track_number").
		Set("disc_number = EXCLUDED.disc_number").
		Set("year = EXCLUDED.year").
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

func listenLocalTrackTimelineChanged(before, after listenLocalTrackRow) bool {
	return before.DurationMs != after.DurationMs || listenLocalTrackContentSignatureChanged(before, after)
}

func listenLocalTrackContentSignatureChanged(before, after listenLocalTrackRow) bool {
	beforeSignature := strings.TrimSpace(before.ContentIdentitySignature)
	afterSignature := strings.TrimSpace(after.ContentIdentitySignature)
	// Empty is the safe migration state. The first successful refresh records a
	// baseline without invalidating resume state for content that may be
	// unchanged; every subsequent non-empty mismatch is a timeline change.
	if beforeSignature == "" || afterSignature == "" || beforeSignature == afterSignature {
		return false
	}
	// Early development builds briefly persisted an mci1s stat fallback. It was
	// not capable of distinguishing tag rewrites from audio replacement, so its
	// first packet baseline is a private upgrade rather than a timeline change.
	if strings.HasPrefix(beforeSignature, "mci1s:") {
		return false
	}
	return true
}

func listenLocalTrackMetadataChanged(before, after listenLocalTrackRow) bool {
	return before.Title != after.Title ||
		before.Author != after.Author ||
		before.Album != after.Album ||
		before.AlbumArtist != after.AlbumArtist ||
		before.Genre != after.Genre ||
		before.TrackNumber != after.TrackNumber ||
		before.DiscNumber != after.DiscNumber ||
		before.Year != after.Year
}

func listenLocalTrackResourcesChanged(before, after listenLocalTrackRow) bool {
	return before.LocalPath != after.LocalPath ||
		before.CoverLocalPath != after.CoverLocalPath ||
		before.Format != after.Format ||
		before.AudioCodec != after.AudioCodec ||
		before.DurationMs != after.DurationMs ||
		before.SizeBytes != after.SizeBytes ||
		before.ModTimeUnix != after.ModTimeUnix ||
		listenLocalTrackContentSignatureChanged(before, after)
}

func listenLocalTrackObservableChanged(before, after listenLocalTrackRow) bool {
	return before.LibraryID != after.LibraryID ||
		before.ContentIdentityRevision != after.ContentIdentityRevision ||
		before.MetadataRevision != after.MetadataRevision ||
		before.ResourceRevision != after.ResourceRevision ||
		before.LocalPath != after.LocalPath ||
		before.Title != after.Title ||
		before.Author != after.Author ||
		before.Album != after.Album ||
		before.AlbumArtist != after.AlbumArtist ||
		before.Genre != after.Genre ||
		before.TrackNumber != after.TrackNumber ||
		before.DiscNumber != after.DiscNumber ||
		before.Year != after.Year ||
		before.CoverLocalPath != after.CoverLocalPath ||
		before.Format != after.Format ||
		before.AudioCodec != after.AudioCodec ||
		before.DurationMs != after.DurationMs ||
		before.SizeBytes != after.SizeBytes ||
		before.ModTimeUnix != after.ModTimeUnix ||
		before.Availability != after.Availability ||
		before.ProbeError != after.ProbeError
}

func mapListenLocalTrackRows(rows []listenLocalTrackRow) ([]library.ListenLocalTrack, error) {
	result := make([]library.ListenLocalTrack, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainListenLocalTrack(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func toDomainListenLocalTrack(row listenLocalTrackRow) (library.ListenLocalTrack, error) {
	return library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID:                   row.FileID,
		LibraryID:                row.LibraryID,
		Revision:                 row.Revision,
		ContentIdentityRevision:  row.ContentIdentityRevision,
		ContentIdentitySignature: row.ContentIdentitySignature,
		MetadataRevision:         row.MetadataRevision,
		ResourceRevision:         row.ResourceRevision,
		LocalPath:                row.LocalPath,
		Title:                    row.Title,
		Author:                   stringOrEmpty(row.Author),
		Album:                    stringOrEmpty(row.Album),
		AlbumArtist:              stringOrEmpty(row.AlbumArtist),
		Genre:                    stringOrEmpty(row.Genre),
		TrackNumber:              positiveIntOrZero(row.TrackNumber),
		DiscNumber:               positiveIntOrZero(row.DiscNumber),
		Year:                     positiveIntOrZero(row.Year),
		CoverLocalPath:           stringOrEmpty(row.CoverLocalPath),
		Format:                   stringOrEmpty(row.Format),
		AudioCodec:               stringOrEmpty(row.AudioCodec),
		DurationMs:               int64OrNil(row.DurationMs),
		SizeBytes:                int64OrNil(row.SizeBytes),
		ModTimeUnix:              row.ModTimeUnix,
		Availability:             row.Availability,
		LastCheckedAt:            &row.LastCheckedAt,
		ProbeError:               stringOrEmpty(row.ProbeError),
		CreatedAt:                &row.CreatedAt,
		UpdatedAt:                &row.UpdatedAt,
	})
}

func nullPositiveInt(value int) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}

func positiveIntOrZero(value sql.NullInt64) int {
	if !value.Valid || value.Int64 <= 0 {
		return 0
	}
	return int(value.Int64)
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
