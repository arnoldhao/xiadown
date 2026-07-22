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

type SQLiteListenLocalPlaylistRepository struct{ db *bun.DB }

type listenLocalPlaylistRow struct {
	bun.BaseModel `bun:"table:listen_local_playlists"`
	ID            string    `bun:"id,pk"`
	Name          string    `bun:"name"`
	Revision      int64     `bun:"revision"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type listenLocalPlaylistItemRow struct {
	bun.BaseModel          `bun:"table:listen_local_playlist_items"`
	ID                     string        `bun:"id,pk"`
	PlaylistID             string        `bun:"playlist_id"`
	FileID                 string        `bun:"file_id"`
	Position               int           `bun:"position"`
	AddedAt                time.Time     `bun:"added_at"`
	Revision               int64         `bun:"revision"`
	DeletedAt              sql.NullTime  `bun:"deleted_at"`
	TrackDisplayTitle      string        `bun:"track_display_title"`
	TrackDisplayAuthor     string        `bun:"track_display_author"`
	TrackDisplayAlbum      string        `bun:"track_display_album"`
	TrackDisplayDurationMs sql.NullInt64 `bun:"track_display_duration_ms"`
}

type listenLocalPlaylistItemCountRow struct {
	bun.BaseModel `bun:"table:listen_local_playlist_items"`
	PlaylistID    string `bun:"playlist_id"`
	ItemCount     int    `bun:"item_count"`
}

func NewSQLiteListenLocalPlaylistRepository(db *bun.DB) *SQLiteListenLocalPlaylistRepository {
	return &SQLiteListenLocalPlaylistRepository{db: db}
}

func (repo *SQLiteListenLocalPlaylistRepository) List(ctx context.Context) ([]library.ListenLocalPlaylist, error) {
	rows := make([]listenLocalPlaylistRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Order("updated_at DESC", "name ASC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.ListenLocalPlaylist, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainListenLocalPlaylist(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteListenLocalPlaylistRepository) CountItems(ctx context.Context, playlistIDs []string) (map[string]int, error) {
	counts := make(map[string]int, len(playlistIDs))
	uniqueIDs := make([]string, 0, len(playlistIDs))
	for _, rawID := range playlistIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := counts[id]; exists {
			continue
		}
		counts[id] = 0
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return counts, nil
	}

	rows := make([]listenLocalPlaylistItemCountRow, 0, len(uniqueIDs))
	if err := repo.db.NewSelect().Model(&rows).
		Column("playlist_id").
		ColumnExpr("COUNT(*) AS item_count").
		Where("playlist_id IN (?)", bun.In(uniqueIDs)).
		Where("deleted_at IS NULL").
		Group("playlist_id").
		Scan(ctx); err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.PlaylistID] = row.ItemCount
	}
	return counts, nil
}

func (repo *SQLiteListenLocalPlaylistRepository) Get(ctx context.Context, id string) (library.ListenLocalPlaylist, error) {
	row := new(listenLocalPlaylistRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ListenLocalPlaylist{}, library.ErrListenLocalPlaylistNotFound
		}
		return library.ListenLocalPlaylist{}, err
	}
	return toDomainListenLocalPlaylist(*row)
}

func (repo *SQLiteListenLocalPlaylistRepository) Save(ctx context.Context, item library.ListenLocalPlaylist) error {
	row := listenLocalPlaylistRow{
		ID: item.ID, Name: item.Name, Revision: item.Revision, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if row.Revision < 1 {
		row.Revision = 1
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		existing := new(listenLocalPlaylistRow)
		findErr := tx.NewSelect().Model(existing).Where("id = ?", row.ID).Scan(ctx)
		isNew := errors.Is(findErr, sql.ErrNoRows)
		if findErr != nil && !isNew {
			return findErr
		}
		if isNew {
			tombstoneRevision, _, err := listenLocalTombstoneRevision(ctx, tx, listenLocalEntityPlaylist, row.ID)
			if err != nil {
				return err
			}
			row.Revision = max(row.Revision, tombstoneRevision+1, 1)
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				return err
			}
			if err := clearListenLocalTombstone(ctx, tx, listenLocalEntityPlaylist, row.ID); err != nil {
				return err
			}
			return appendListenLocalChange(ctx, tx, listenLocalEntityPlaylist, row.ID, "upsert", row.Revision, row.UpdatedAt)
		}

		if row.Revision != existing.Revision {
			return &library.ListenLocalMusicRevisionConflictError{CurrentRevision: existing.Revision}
		}
		if existing.Name == row.Name {
			return nil
		}
		nextRevision := existing.Revision + 1
		updated, err := tx.NewUpdate().Model((*listenLocalPlaylistRow)(nil)).
			Set("name = ?", row.Name).
			Set("revision = ?", nextRevision).
			Set("updated_at = ?", row.UpdatedAt).
			Where("id = ?", row.ID).
			Where("revision = ?", row.Revision).
			Exec(ctx)
		if err != nil {
			return err
		}
		affected, _ := updated.RowsAffected()
		if affected == 0 {
			return &library.ListenLocalMusicRevisionConflictError{CurrentRevision: existing.Revision}
		}
		return appendListenLocalChange(ctx, tx, listenLocalEntityPlaylist, row.ID, "upsert", nextRevision, row.UpdatedAt)
	})
}

func (repo *SQLiteListenLocalPlaylistRepository) Delete(ctx context.Context, id string, expectedRevision int64) error {
	id = strings.TrimSpace(id)
	if id == "" || expectedRevision < 1 {
		return library.ErrInvalidListenLocalPlaylist
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		result, err := tx.NewDelete().Model((*listenLocalPlaylistRow)(nil)).
			Where("id = ?", id).
			Where("revision = ?", expectedRevision).
			Exec(ctx)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			return nil
		}
		current := new(listenLocalPlaylistRow)
		if err := tx.NewSelect().Model(current).Where("id = ?", id).Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return library.ErrListenLocalPlaylistNotFound
			}
			return err
		}
		return &library.ListenLocalMusicRevisionConflictError{CurrentRevision: current.Revision}
	})
}

func (repo *SQLiteListenLocalPlaylistRepository) ListItems(ctx context.Context, playlistID string) ([]library.ListenLocalPlaylistItem, error) {
	rows := make([]listenLocalPlaylistItemRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("playlist_id = ?", strings.TrimSpace(playlistID)).
		Where("deleted_at IS NULL").
		Order("position ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.ListenLocalPlaylistItem, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainListenLocalPlaylistItem(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteListenLocalPlaylistRepository) ReplaceItems(ctx context.Context, playlist library.ListenLocalPlaylist, items []library.ListenLocalPlaylistItem) error {
	playlistID := strings.TrimSpace(playlist.ID)
	if playlistID == "" {
		return library.ErrInvalidListenLocalPlaylist
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		currentPlaylist := new(listenLocalPlaylistRow)
		if err := tx.NewSelect().Model(currentPlaylist).Where("id = ?", playlistID).Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return library.ErrListenLocalPlaylistNotFound
			}
			return err
		}
		if playlist.Revision != currentPlaylist.Revision {
			return &library.ListenLocalMusicRevisionConflictError{CurrentRevision: currentPlaylist.Revision}
		}
		existingRows := make([]listenLocalPlaylistItemRow, 0)
		if err := tx.NewSelect().Model(&existingRows).
			Where("playlist_id = ?", playlistID).
			Where("deleted_at IS NULL").
			Order("position ASC").
			Scan(ctx); err != nil {
			return err
		}
		existingByID := make(map[string]listenLocalPlaylistItemRow, len(existingRows))
		maxPosition := 0
		for _, row := range existingRows {
			existingByID[row.ID] = row
			maxPosition = max(maxPosition, row.Position)
		}

		desiredRows := make([]listenLocalPlaylistItemRow, 0, len(items))
		desiredIDs := make(map[string]struct{}, len(items))
		for position, item := range items {
			if item.PlaylistID != playlistID || item.Position != position || strings.TrimSpace(item.ID) == "" {
				return library.ErrInvalidListenLocalPlaylist
			}
			if _, duplicate := desiredIDs[item.ID]; duplicate {
				return library.ErrInvalidListenLocalPlaylist
			}
			desiredIDs[item.ID] = struct{}{}
			row := listenLocalPlaylistItemRow{
				ID: item.ID, PlaylistID: playlistID, FileID: item.FileID, Position: position,
				AddedAt: item.AddedAt, Revision: max(item.Revision, 1),
				TrackDisplayTitle:      item.TrackDisplaySnapshot.Title,
				TrackDisplayAuthor:     item.TrackDisplaySnapshot.Author,
				TrackDisplayAlbum:      item.TrackDisplaySnapshot.Album,
				TrackDisplayDurationMs: nullInt64(item.TrackDisplaySnapshot.DurationMs),
			}
			if strings.TrimSpace(row.TrackDisplayTitle) == "" {
				track := new(listenLocalTrackRow)
				if err := tx.NewSelect().Model(track).Where("file_id = ?", row.FileID).Scan(ctx); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return library.ErrFileNotFound
					}
					return err
				}
				row.TrackDisplayTitle = track.Title
				row.TrackDisplayAuthor = stringOrEmpty(track.Author)
				row.TrackDisplayAlbum = stringOrEmpty(track.Album)
				row.TrackDisplayDurationMs = track.DurationMs
			}
			desiredRows = append(desiredRows, row)
		}

		itemsChanged := false
		for _, existing := range existingRows {
			if _, keep := desiredIDs[existing.ID]; keep {
				continue
			}
			if _, err := tx.NewDelete().Model((*listenLocalPlaylistItemRow)(nil)).Where("id = ?", existing.ID).Exec(ctx); err != nil {
				return err
			}
			itemsChanged = true
		}
		if len(existingRows) > 0 {
			offset := maxPosition + len(existingRows) + len(items) + 1
			if _, err := tx.NewUpdate().Model((*listenLocalPlaylistItemRow)(nil)).
				Set("position = position + ?", offset).
				Where("playlist_id = ?", playlistID).
				Where("deleted_at IS NULL").
				Exec(ctx); err != nil {
				return err
			}
		}

		for _, row := range desiredRows {
			existing, exists := existingByID[row.ID]
			if exists {
				row.Revision = existing.Revision
				changed := listenLocalPlaylistItemChanged(existing, row)
				if changed {
					row.Revision++
					itemsChanged = true
				}
				if _, err := tx.NewUpdate().Model((*listenLocalPlaylistItemRow)(nil)).
					Set("file_id = ?", row.FileID).
					Set("position = ?", row.Position).
					Set("added_at = ?", row.AddedAt).
					Set("revision = ?", row.Revision).
					Set("track_display_title = ?", row.TrackDisplayTitle).
					Set("track_display_author = ?", row.TrackDisplayAuthor).
					Set("track_display_album = ?", row.TrackDisplayAlbum).
					Set("track_display_duration_ms = ?", row.TrackDisplayDurationMs).
					Where("id = ?", row.ID).
					Exec(ctx); err != nil {
					return err
				}
				if changed {
					if err := appendListenLocalChange(ctx, tx, listenLocalEntityPlaylistItem, row.ID, "upsert", row.Revision, playlist.UpdatedAt); err != nil {
						return err
					}
				}
				continue
			}
			tombstoneRevision, _, err := listenLocalTombstoneRevision(ctx, tx, listenLocalEntityPlaylistItem, row.ID)
			if err != nil {
				return err
			}
			row.Revision = max(row.Revision, tombstoneRevision+1, 1)
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				return err
			}
			if err := clearListenLocalTombstone(ctx, tx, listenLocalEntityPlaylistItem, row.ID); err != nil {
				return err
			}
			if err := appendListenLocalChange(ctx, tx, listenLocalEntityPlaylistItem, row.ID, "upsert", row.Revision, playlist.UpdatedAt); err != nil {
				return err
			}
			itemsChanged = true
		}

		nextPlaylistRevision := currentPlaylist.Revision
		if itemsChanged {
			nextPlaylistRevision++
		}
		updatedAt := playlist.UpdatedAt
		if !itemsChanged {
			updatedAt = currentPlaylist.UpdatedAt
		}
		updated, err := tx.NewUpdate().Model((*listenLocalPlaylistRow)(nil)).
			Set("revision = ?", nextPlaylistRevision).
			Set("updated_at = ?", updatedAt).
			Where("id = ?", playlistID).
			Where("revision = ?", currentPlaylist.Revision).
			Exec(ctx)
		if err != nil {
			return err
		}
		affected, _ := updated.RowsAffected()
		if affected == 0 {
			return &library.ListenLocalMusicRevisionConflictError{CurrentRevision: currentPlaylist.Revision}
		}
		if itemsChanged {
			return appendListenLocalChange(ctx, tx, listenLocalEntityPlaylist, playlistID, "upsert", nextPlaylistRevision, updatedAt)
		}
		return nil
	})
}

func toDomainListenLocalPlaylist(row listenLocalPlaylistRow) (library.ListenLocalPlaylist, error) {
	return library.NewListenLocalPlaylist(library.ListenLocalPlaylistParams{
		ID: row.ID, Name: row.Name, Revision: row.Revision, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func toDomainListenLocalPlaylistItem(row listenLocalPlaylistItemRow) (library.ListenLocalPlaylistItem, error) {
	return library.NewListenLocalPlaylistItemWithParams(library.ListenLocalPlaylistItemParams{
		ID: row.ID, PlaylistID: row.PlaylistID, FileID: row.FileID, Position: row.Position,
		Revision: row.Revision, AddedAt: &row.AddedAt,
		TrackDisplaySnapshot: library.ListenLocalTrackDisplaySnapshot{
			Title: row.TrackDisplayTitle, Author: row.TrackDisplayAuthor, Album: row.TrackDisplayAlbum,
			DurationMs: int64OrNil(row.TrackDisplayDurationMs),
		},
	})
}

func listenLocalPlaylistItemChanged(before, after listenLocalPlaylistItemRow) bool {
	return before.FileID != after.FileID ||
		before.Position != after.Position ||
		!before.AddedAt.Equal(after.AddedAt) ||
		before.TrackDisplayTitle != after.TrackDisplayTitle ||
		before.TrackDisplayAuthor != after.TrackDisplayAuthor ||
		before.TrackDisplayAlbum != after.TrackDisplayAlbum ||
		before.TrackDisplayDurationMs != after.TrackDisplayDurationMs
}
