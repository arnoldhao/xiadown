package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type listenLocalMusicCreatePlaylistPayload struct {
	Name string `json:"name"`
}

type listenLocalMusicRenamePlaylistPayload struct {
	Name string `json:"name"`
}

type listenLocalMusicAddPlaylistItemPayload struct {
	ClientItemID string `json:"clientItemId"`
	TrackID      string `json:"trackId"`
	Position     *int   `json:"position,omitempty"`
}

type listenLocalMusicRemovePlaylistItemPayload struct {
	PlaylistID string `json:"playlistId"`
}

type listenLocalMusicReorderPlaylistPayload struct {
	ItemIDs *[]string `json:"itemIds"`
}

type listenLocalMusicSetMembershipPayload struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type listenLocalMusicDeletePlaylistPayload struct{}

type listenLocalMusicPlaylistResult struct {
	ID        string                               `json:"id"`
	Name      string                               `json:"name"`
	Revision  int64                                `json:"revision"`
	Items     []listenLocalMusicPlaylistItemResult `json:"items"`
	CreatedAt time.Time                            `json:"createdAt"`
	UpdatedAt time.Time                            `json:"updatedAt"`
	DeletedAt *time.Time                           `json:"deletedAt"`
}

type listenLocalMusicPlaylistItemResult struct {
	ID                   string                                     `json:"id"`
	PlaylistID           string                                     `json:"playlistId"`
	TrackID              string                                     `json:"trackId"`
	OrderKey             string                                     `json:"orderKey"`
	AddedAt              time.Time                                  `json:"addedAt"`
	Revision             int64                                      `json:"revision"`
	DeletedAt            *time.Time                                 `json:"deletedAt"`
	TrackDisplaySnapshot listenLocalMusicTrackDisplaySnapshotResult `json:"trackDisplaySnapshot"`
}

type listenLocalMusicTrackDisplaySnapshotResult struct {
	Title      string `json:"title"`
	ArtistName string `json:"artistName,omitempty"`
	AlbumTitle string `json:"albumTitle,omitempty"`
	DurationMs *int64 `json:"durationMs,omitempty"`
}

func (repo *SQLiteListenLocalMusicWriteRepository) applyManageMutationTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	switch mutation.Type {
	case "createPlaylist":
		return repo.createPlaylistTx(ctx, tx, mutation)
	case "renamePlaylist":
		return repo.renamePlaylistTx(ctx, tx, mutation)
	case "deletePlaylist":
		return repo.deletePlaylistTx(ctx, tx, mutation)
	case "addPlaylistItem":
		return repo.addPlaylistItemTx(ctx, tx, mutation)
	case "removePlaylistItem":
		return repo.removePlaylistItemTx(ctx, tx, mutation)
	case "reorderPlaylist":
		return repo.reorderPlaylistTx(ctx, tx, mutation)
	case "setMembership":
		return repo.setMembershipTx(ctx, tx, mutation)
	default:
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
}

func (repo *SQLiteListenLocalMusicWriteRepository) createPlaylistTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicCreatePlaylistPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if mutation.ExpectedRevision != 0 || !validClientMusicUUID(mutation.EntityID) || payload.Name == "" ||
		len([]rune(payload.Name)) > library.ListenLocalPlaylistNameMaxLength {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	existing := new(listenLocalPlaylistRow)
	err := tx.NewSelect().Model(existing).Where("id = ?", mutation.EntityID).Scan(ctx)
	if err == nil {
		current, currentErr := loadListenLocalMusicPlaylistResultTx(ctx, tx, *existing)
		if currentErr != nil {
			return library.ListenLocalMusicMutationResult{}, currentErr
		}
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(existing.Revision, current)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return library.ListenLocalMusicMutationResult{}, err
	}
	tombstoneRevision, _, err := listenLocalTombstoneRevision(
		ctx, tx, library.ListenLocalMusicEntityPlaylist, mutation.EntityID,
	)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if tombstoneRevision > 0 {
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(tombstoneRevision, nil)
	}
	row := listenLocalPlaylistRow{
		ID: mutation.EntityID, Name: payload.Name, Revision: 1,
		CreatedAt: mutation.OccurredAt, UpdatedAt: mutation.OccurredAt,
	}
	if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	canonical, err := loadListenLocalMusicPlaylistResultTx(ctx, tx, row)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if err := appendListenLocalMusicPayloadChange(
		ctx, tx, library.ListenLocalMusicEntityPlaylist, row.ID, row.Revision, canonical, mutation.OccurredAt,
	); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	return newListenLocalMusicMutationResult(mutation, row.Revision, map[string]any{"playlist": canonical})
}

func (repo *SQLiteListenLocalMusicWriteRepository) renamePlaylistTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicRenamePlaylistPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" || len([]rune(payload.Name)) > library.ListenLocalPlaylistNameMaxLength {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	row, err := loadListenLocalMusicPlaylistMutationTx(ctx, tx, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if mutation.ExpectedRevision != row.Revision {
		current, currentErr := loadListenLocalMusicPlaylistResultTx(ctx, tx, row)
		if currentErr != nil {
			return library.ListenLocalMusicMutationResult{}, currentErr
		}
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(row.Revision, current)
	}
	if row.Name == payload.Name {
		canonical, err := loadListenLocalMusicPlaylistResultTx(ctx, tx, row)
		if err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
		return newListenLocalMusicMutationResult(mutation, row.Revision, map[string]any{"playlist": canonical})
	}
	row.Name = payload.Name
	row.Revision++
	row.UpdatedAt = mutation.OccurredAt
	if _, err := tx.NewUpdate().Model(&row).Column("name", "revision", "updated_at").WherePK().Exec(ctx); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	canonical, err := loadListenLocalMusicPlaylistResultTx(ctx, tx, row)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if err := appendListenLocalMusicPayloadChange(
		ctx, tx, library.ListenLocalMusicEntityPlaylist, row.ID, row.Revision, canonical, mutation.OccurredAt,
	); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	return newListenLocalMusicMutationResult(mutation, row.Revision, map[string]any{"playlist": canonical})
}

func (repo *SQLiteListenLocalMusicWriteRepository) deletePlaylistTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicDeletePlaylistPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	row, err := loadListenLocalMusicPlaylistMutationTx(ctx, tx, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if mutation.ExpectedRevision != row.Revision {
		current, currentErr := loadListenLocalMusicPlaylistResultTx(ctx, tx, row)
		if currentErr != nil {
			return library.ListenLocalMusicMutationResult{}, currentErr
		}
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(row.Revision, current)
	}
	deletedRevision := row.Revision + 1
	if _, err := tx.NewDelete().Model(&row).WherePK().Exec(ctx); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	return newListenLocalMusicMutationResult(mutation, deletedRevision, map[string]any{
		"deleted": true, "deletedRevision": deletedRevision,
	})
}

func (repo *SQLiteListenLocalMusicWriteRepository) addPlaylistItemTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicAddPlaylistItemPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	payload.ClientItemID = strings.ToLower(strings.TrimSpace(payload.ClientItemID))
	payload.TrackID = strings.TrimSpace(payload.TrackID)
	if !validClientMusicUUID(payload.ClientItemID) || payload.TrackID == "" ||
		(payload.Position != nil && *payload.Position < 0) {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	playlist, err := loadListenLocalMusicPlaylistMutationTx(ctx, tx, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if err := requireListenLocalMusicPlaylistRevisionTx(ctx, tx, playlist, mutation.ExpectedRevision); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	track, err := requireActiveListenLocalMusicTrackTx(ctx, tx, payload.TrackID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	var collision int
	if err := tx.NewSelect().Table("listen_local_playlist_items").ColumnExpr("COUNT(*)").
		Where("id = ?", payload.ClientItemID).Scan(ctx, &collision); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	tombstoneRevision, _, err := listenLocalTombstoneRevision(
		ctx, tx, library.ListenLocalMusicEntityPlaylistItem, payload.ClientItemID,
	)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if collision > 0 || tombstoneRevision > 0 {
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(max(tombstoneRevision, int64(1)), nil)
	}
	items, err := loadListenLocalMusicPlaylistRowsMutationTx(ctx, tx, playlist.ID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	position := len(items)
	if payload.Position != nil {
		position = min(*payload.Position, len(items))
	}
	row := listenLocalPlaylistItemRow{
		ID: payload.ClientItemID, PlaylistID: playlist.ID, FileID: payload.TrackID, Position: position,
		AddedAt: mutation.OccurredAt, Revision: 1,
		TrackDisplayTitle: track.Title, TrackDisplayAuthor: stringOrEmpty(track.Author),
		TrackDisplayAlbum: track.Album.String, TrackDisplayDurationMs: track.DurationMs,
	}
	desired := make([]listenLocalPlaylistItemRow, 0, len(items)+1)
	desired = append(desired, items[:position]...)
	desired = append(desired, row)
	desired = append(desired, items[position:]...)
	return repo.replacePlaylistItemsMutationTx(ctx, tx, mutation, playlist, items, desired)
}

func (repo *SQLiteListenLocalMusicWriteRepository) removePlaylistItemTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicRemovePlaylistItemPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	payload.PlaylistID = strings.TrimSpace(payload.PlaylistID)
	if payload.PlaylistID == "" {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	playlist, err := loadListenLocalMusicPlaylistMutationTx(ctx, tx, payload.PlaylistID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if err := requireListenLocalMusicPlaylistRevisionTx(ctx, tx, playlist, mutation.ExpectedRevision); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	items, err := loadListenLocalMusicPlaylistRowsMutationTx(ctx, tx, playlist.ID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	desired := make([]listenLocalPlaylistItemRow, 0, max(0, len(items)-1))
	found := false
	for _, item := range items {
		if item.ID == mutation.EntityID {
			found = true
			continue
		}
		desired = append(desired, item)
	}
	if !found {
		return library.ListenLocalMusicMutationResult{}, sql.ErrNoRows
	}
	return repo.replacePlaylistItemsMutationTx(ctx, tx, mutation, playlist, items, desired)
}

func (repo *SQLiteListenLocalMusicWriteRepository) reorderPlaylistTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicReorderPlaylistPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	playlist, err := loadListenLocalMusicPlaylistMutationTx(ctx, tx, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if err := requireListenLocalMusicPlaylistRevisionTx(ctx, tx, playlist, mutation.ExpectedRevision); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	items, err := loadListenLocalMusicPlaylistRowsMutationTx(ctx, tx, playlist.ID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if payload.ItemIDs == nil || len(*payload.ItemIDs) != len(items) {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	byID := make(map[string]listenLocalPlaylistItemRow, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	desired := make([]listenLocalPlaylistItemRow, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, rawID := range *payload.ItemIDs {
		id := strings.ToLower(strings.TrimSpace(rawID))
		if !validClientMusicUUID(id) {
			return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
		}
		item, exists := byID[id]
		if _, duplicate := seen[id]; !exists || duplicate {
			return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
		}
		seen[id] = struct{}{}
		desired = append(desired, item)
	}
	return repo.replacePlaylistItemsMutationTx(ctx, tx, mutation, playlist, items, desired)
}

func (repo *SQLiteListenLocalMusicWriteRepository) replacePlaylistItemsMutationTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
	playlist listenLocalPlaylistRow,
	current []listenLocalPlaylistItemRow,
	desired []listenLocalPlaylistItemRow,
) (library.ListenLocalMusicMutationResult, error) {
	currentByID := make(map[string]listenLocalPlaylistItemRow, len(current))
	desiredIDs := make(map[string]struct{}, len(desired))
	maxPosition := 0
	for _, item := range current {
		currentByID[item.ID] = item
		maxPosition = max(maxPosition, item.Position)
	}
	for position := range desired {
		desired[position].Position = position
		if desired[position].PlaylistID != playlist.ID {
			return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
		}
		if _, duplicate := desiredIDs[desired[position].ID]; duplicate {
			return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
		}
		desiredIDs[desired[position].ID] = struct{}{}
	}
	changed := len(current) != len(desired)
	for position, item := range desired {
		existing, exists := currentByID[item.ID]
		if !exists || existing.Position != position {
			changed = true
		}
	}
	if !changed {
		canonical, err := loadListenLocalMusicPlaylistResultTx(ctx, tx, playlist)
		if err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
		return newListenLocalMusicMutationResult(mutation, playlist.Revision, map[string]any{"playlist": canonical})
	}

	for _, item := range current {
		if _, keep := desiredIDs[item.ID]; keep {
			continue
		}
		if _, err := tx.NewDelete().Model(&item).WherePK().Exec(ctx); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
	}
	if len(current) > 0 {
		offset := maxPosition + len(current) + len(desired) + 1
		if _, err := tx.NewUpdate().Model((*listenLocalPlaylistItemRow)(nil)).
			Set("position = position + ?", offset).
			Where("playlist_id = ?", playlist.ID).
			Where("deleted_at IS NULL").
			Exec(ctx); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
	}
	for _, item := range desired {
		existing, exists := currentByID[item.ID]
		if exists {
			item.Revision = existing.Revision
			if existing.Position != item.Position {
				item.Revision++
			}
			if _, err := tx.NewUpdate().Model(&item).
				Column("position", "revision").WherePK().Exec(ctx); err != nil {
				return library.ListenLocalMusicMutationResult{}, err
			}
			if existing.Position != item.Position {
				canonical := listenLocalMusicPlaylistItemResultFromRow(item)
				if err := appendListenLocalMusicPayloadChange(
					ctx, tx, library.ListenLocalMusicEntityPlaylistItem, item.ID, item.Revision,
					canonical, mutation.OccurredAt,
				); err != nil {
					return library.ListenLocalMusicMutationResult{}, err
				}
			}
			continue
		}
		if _, err := tx.NewInsert().Model(&item).Exec(ctx); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
		if err := clearListenLocalTombstone(ctx, tx, library.ListenLocalMusicEntityPlaylistItem, item.ID); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
		canonical := listenLocalMusicPlaylistItemResultFromRow(item)
		if err := appendListenLocalMusicPayloadChange(
			ctx, tx, library.ListenLocalMusicEntityPlaylistItem, item.ID, item.Revision, canonical, mutation.OccurredAt,
		); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
	}

	playlist.Revision++
	playlist.UpdatedAt = mutation.OccurredAt
	if _, err := tx.NewUpdate().Model(&playlist).Column("revision", "updated_at").WherePK().Exec(ctx); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	canonical, err := loadListenLocalMusicPlaylistResultTx(ctx, tx, playlist)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if err := appendListenLocalMusicPayloadChange(
		ctx, tx, library.ListenLocalMusicEntityPlaylist, playlist.ID, playlist.Revision, canonical, mutation.OccurredAt,
	); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	return newListenLocalMusicMutationResult(mutation, playlist.Revision, map[string]any{"playlist": canonical})
}

func (repo *SQLiteListenLocalMusicWriteRepository) setMembershipTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicSetMembershipPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	payload.State = strings.ToLower(strings.TrimSpace(payload.State))
	payload.Reason = strings.ToLower(strings.TrimSpace(payload.Reason))
	if (payload.State != "included" && payload.State != "excluded") ||
		(payload.Reason != "" && payload.Reason != "user" && payload.Reason != "unsupported" && payload.Reason != "policy") {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	var fileCount int
	if err := tx.NewSelect().Table("library_files").ColumnExpr("COUNT(*)").Where("id = ?", mutation.EntityID).Scan(ctx, &fileCount); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if fileCount == 0 {
		return library.ListenLocalMusicMutationResult{}, sql.ErrNoRows
	}
	row := new(listenLocalMusicMembershipRow)
	err := tx.NewSelect().Model(row).Where("file_id = ?", mutation.EntityID).Scan(ctx)
	found := err == nil
	if errors.Is(err, sql.ErrNoRows) {
		row = &listenLocalMusicMembershipRow{FileID: mutation.EntityID}
	} else if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if mutation.ExpectedRevision != row.Revision {
		var current any
		if found {
			membership, err := toDomainListenLocalMusicMembership(*row)
			if err != nil {
				return library.ListenLocalMusicMutationResult{}, err
			}
			current = listenLocalMusicMembershipResultFromDomain(membership)
		}
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(row.Revision, current)
	}
	if found && row.State == payload.State && row.Reason == payload.Reason {
		canonical, err := toDomainListenLocalMusicMembership(*row)
		if err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
		return newListenLocalMusicMutationResult(mutation, row.Revision, map[string]any{
			"membership":      listenLocalMusicMembershipResultFromDomain(canonical),
			"reindexRequired": payload.State == "included",
		})
	}
	now := mutation.OccurredAt
	if !found {
		row.CreatedAt = now
	}
	row.State = payload.State
	row.Reason = payload.Reason
	row.Revision = max(row.Revision+1, int64(1))
	row.UpdatedAt = now
	if _, err := tx.NewInsert().Model(row).
		On("CONFLICT(file_id) DO UPDATE").
		Set("state = EXCLUDED.state").
		Set("reason = EXCLUDED.reason").
		Set("revision = EXCLUDED.revision").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if err := clearListenLocalTombstone(ctx, tx, library.ListenLocalMusicEntityMembership, row.FileID); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	canonical, err := toDomainListenLocalMusicMembership(*row)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	membershipResult := listenLocalMusicMembershipResultFromDomain(canonical)
	if err := appendListenLocalMusicPayloadChange(
		ctx, tx, library.ListenLocalMusicEntityMembership, row.FileID, row.Revision, membershipResult, now,
	); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if payload.State == "excluded" {
		if _, err := tx.NewDelete().Model((*listenLocalTrackRow)(nil)).Where("file_id = ?", row.FileID).Exec(ctx); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
	}
	return newListenLocalMusicMutationResult(mutation, row.Revision, map[string]any{
		"membership": membershipResult, "reindexRequired": payload.State == "included",
	})
}

func loadListenLocalMusicPlaylistMutationTx(ctx context.Context, tx bun.Tx, playlistID string) (listenLocalPlaylistRow, error) {
	row := new(listenLocalPlaylistRow)
	err := tx.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(playlistID)).Scan(ctx)
	return *row, err
}

func requireListenLocalMusicPlaylistRevisionTx(
	ctx context.Context,
	tx bun.Tx,
	playlist listenLocalPlaylistRow,
	expected int64,
) error {
	if expected == playlist.Revision {
		return nil
	}
	current, err := loadListenLocalMusicPlaylistResultTx(ctx, tx, playlist)
	if err != nil {
		return err
	}
	return listenLocalMusicCurrentRevisionConflict(playlist.Revision, current)
}

func loadListenLocalMusicPlaylistRowsMutationTx(
	ctx context.Context,
	tx bun.Tx,
	playlistID string,
) ([]listenLocalPlaylistItemRow, error) {
	rows := make([]listenLocalPlaylistItemRow, 0)
	err := tx.NewSelect().Model(&rows).
		Where("playlist_id = ?", playlistID).
		Where("deleted_at IS NULL").
		Order("position ASC", "id ASC").Scan(ctx)
	return rows, err
}

func loadListenLocalMusicPlaylistResultTx(
	ctx context.Context,
	tx bun.Tx,
	playlist listenLocalPlaylistRow,
) (listenLocalMusicPlaylistResult, error) {
	rows, err := loadListenLocalMusicPlaylistRowsMutationTx(ctx, tx, playlist.ID)
	if err != nil {
		return listenLocalMusicPlaylistResult{}, err
	}
	result := listenLocalMusicPlaylistResult{
		ID: playlist.ID, Name: playlist.Name, Revision: playlist.Revision,
		Items:     make([]listenLocalMusicPlaylistItemResult, 0, len(rows)),
		CreatedAt: playlist.CreatedAt.UTC(), UpdatedAt: playlist.UpdatedAt.UTC(),
	}
	for _, row := range rows {
		result.Items = append(result.Items, listenLocalMusicPlaylistItemResultFromRow(row))
	}
	return result, nil
}

func listenLocalMusicPlaylistItemResultFromRow(row listenLocalPlaylistItemRow) listenLocalMusicPlaylistItemResult {
	return listenLocalMusicPlaylistItemResult{
		ID: row.ID, PlaylistID: row.PlaylistID, TrackID: row.FileID,
		OrderKey: listenLocalMusicOrderKey(row.Position), AddedAt: row.AddedAt.UTC(), Revision: row.Revision,
		TrackDisplaySnapshot: listenLocalMusicTrackDisplaySnapshotResult{
			Title: row.TrackDisplayTitle, ArtistName: row.TrackDisplayAuthor,
			AlbumTitle: row.TrackDisplayAlbum, DurationMs: int64OrNil(row.TrackDisplayDurationMs),
		},
	}
}

func listenLocalMusicOrderKey(position int) string {
	return fmt.Sprintf("%020d", max(position, 0))
}

type listenLocalMusicMembershipResult struct {
	FileID    string    `json:"fileId"`
	State     string    `json:"state"`
	Reason    string    `json:"reason,omitempty"`
	Revision  int64     `json:"revision"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func listenLocalMusicMembershipResultFromDomain(item library.ListenLocalMusicMembership) listenLocalMusicMembershipResult {
	return listenLocalMusicMembershipResult{
		FileID: item.FileID, State: string(item.State), Reason: item.Reason, Revision: item.Revision,
		CreatedAt: item.CreatedAt.UTC(), UpdatedAt: item.UpdatedAt.UTC(),
	}
}
