package libraryrepo

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type SQLiteListenLiveChannelRepository struct{ db *bun.DB }

type listenLiveColumnRow struct {
	bun.BaseModel `bun:"table:listen_live_columns"`
	ID            string    `bun:"id,pk"`
	Title         string    `bun:"title"`
	SortOrder     int       `bun:"sort_order"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type listenLiveChannelRow struct {
	bun.BaseModel `bun:"table:listen_live_channels"`
	ID            string    `bun:"id,pk"`
	ColumnID      string    `bun:"column_id"`
	Title         string    `bun:"title"`
	Channel       string    `bun:"channel"`
	Description   string    `bun:"description"`
	Source        string    `bun:"source"`
	VideoID       string    `bun:"video_id"`
	ThumbnailURL  string    `bun:"thumbnail_url"`
	Enabled       bool      `bun:"enabled"`
	SortOrder     int       `bun:"sort_order"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

func NewSQLiteListenLiveChannelRepository(db *bun.DB) *SQLiteListenLiveChannelRepository {
	return &SQLiteListenLiveChannelRepository{db: db}
}

func (repo *SQLiteListenLiveChannelRepository) List(ctx context.Context) (library.ListenLiveCatalogSnapshot, error) {
	columns := make([]listenLiveColumnRow, 0)
	if err := repo.db.NewSelect().Model(&columns).Order("sort_order ASC", "title ASC").Scan(ctx); err != nil {
		return library.ListenLiveCatalogSnapshot{}, err
	}
	channels := make([]listenLiveChannelRow, 0)
	if err := repo.db.NewSelect().Model(&channels).Order("sort_order ASC", "title ASC").Scan(ctx); err != nil {
		return library.ListenLiveCatalogSnapshot{}, err
	}
	return library.ListenLiveCatalogSnapshot{
		Columns:  mapListenLiveColumnRows(columns),
		Channels: mapListenLiveChannelRows(channels),
	}, nil
}

func (repo *SQLiteListenLiveChannelRepository) Replace(ctx context.Context, snapshot library.ListenLiveCatalogSnapshot) error {
	now := time.Now().UTC()
	columns := make([]listenLiveColumnRow, 0, len(snapshot.Columns))
	for _, item := range snapshot.Columns {
		item = library.NormalizeListenLiveColumn(item, now)
		if item.ID == "" || item.Title == "" {
			continue
		}
		columns = append(columns, listenLiveColumnRow{
			ID:        item.ID,
			Title:     item.Title,
			SortOrder: item.SortOrder,
			CreatedAt: item.CreatedAt,
			UpdatedAt: now,
		})
	}
	channels := make([]listenLiveChannelRow, 0, len(snapshot.Channels))
	for _, item := range snapshot.Channels {
		item = library.NormalizeListenLiveChannel(item, now)
		if item.ID == "" || item.ColumnID == "" || item.Title == "" || item.VideoID == "" {
			continue
		}
		channels = append(channels, listenLiveChannelRow{
			ID:           item.ID,
			ColumnID:     item.ColumnID,
			Title:        item.Title,
			Channel:      item.Channel,
			Description:  item.Description,
			Source:       item.Source,
			VideoID:      item.VideoID,
			ThumbnailURL: item.ThumbnailURL,
			Enabled:      item.Enabled,
			SortOrder:    item.SortOrder,
			CreatedAt:    item.CreatedAt,
			UpdatedAt:    now,
		})
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*listenLiveChannelRow)(nil)).Where("1 = 1").Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*listenLiveColumnRow)(nil)).Where("1 = 1").Exec(ctx); err != nil {
			return err
		}
		if len(columns) > 0 {
			if _, err := tx.NewInsert().Model(&columns).Exec(ctx); err != nil {
				return err
			}
		}
		if len(channels) > 0 {
			if _, err := tx.NewInsert().Model(&channels).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func mapListenLiveColumnRows(rows []listenLiveColumnRow) []library.ListenLiveColumn {
	items := make([]library.ListenLiveColumn, 0, len(rows))
	for _, row := range rows {
		items = append(items, library.ListenLiveColumn{
			ID:        row.ID,
			Title:     row.Title,
			SortOrder: row.SortOrder,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return items
}

func mapListenLiveChannelRows(rows []listenLiveChannelRow) []library.ListenLiveChannel {
	items := make([]library.ListenLiveChannel, 0, len(rows))
	for _, row := range rows {
		items = append(items, library.ListenLiveChannel{
			ID:           row.ID,
			ColumnID:     row.ColumnID,
			Title:        row.Title,
			Channel:      row.Channel,
			Description:  row.Description,
			Source:       row.Source,
			VideoID:      row.VideoID,
			ThumbnailURL: row.ThumbnailURL,
			Enabled:      row.Enabled,
			SortOrder:    row.SortOrder,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		})
	}
	return items
}
