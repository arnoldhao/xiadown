package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func (service *LibraryService) ListListenLocalPlaylists(ctx context.Context) ([]dto.ListenLocalPlaylistDTO, error) {
	if service == nil || service.localPlaylists == nil {
		return []dto.ListenLocalPlaylistDTO{}, nil
	}
	playlists, err := service.localPlaylists.List(ctx)
	if err != nil {
		return nil, err
	}
	playlistIDs := make([]string, 0, len(playlists))
	for _, playlist := range playlists {
		playlistIDs = append(playlistIDs, playlist.ID)
	}
	itemCounts, err := service.localPlaylists.CountItems(ctx, playlistIDs)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ListenLocalPlaylistDTO, 0, len(playlists))
	for _, playlist := range playlists {
		result = append(result, toListenLocalPlaylistDTO(playlist, itemCounts[playlist.ID]))
	}
	return result, nil
}

func (service *LibraryService) GetListenLocalPlaylist(ctx context.Context, id string) (dto.ListenLocalPlaylistDetailDTO, error) {
	if service == nil || service.localPlaylists == nil || service.localTracks == nil {
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrListenLocalPlaylistNotFound
	}
	playlist, err := service.localPlaylists.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	items, err := service.localPlaylists.ListItems(ctx, playlist.ID)
	if err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	result := dto.ListenLocalPlaylistDetailDTO{
		Playlist: toListenLocalPlaylistDTO(playlist, len(items)),
		Items:    make([]dto.ListenLocalPlaylistItemDTO, 0, len(items)),
	}
	if len(items) == 0 {
		return result, nil
	}
	fileIDs := make([]string, 0, len(items))
	for _, item := range items {
		fileIDs = append(fileIDs, item.FileID)
	}
	tracks, err := service.localTracks.List(ctx, library.ListenLocalTrackListOptions{
		FileIDs:            fileIDs,
		IncludeUnavailable: true,
	})
	if err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	tracksByFileID := make(map[string]library.ListenLocalTrack, len(tracks))
	for _, track := range tracks {
		tracksByFileID[track.FileID] = track
	}
	for _, item := range items {
		track, exists := tracksByFileID[item.FileID]
		trackDTO := dto.ListenLocalTrackDTO{}
		if exists {
			trackDTO = toListenLocalTrackDTO(track)
		} else {
			trackDTO = dto.ListenLocalTrackDTO{
				ID: item.FileID, FileID: item.FileID,
				Title:        item.TrackDisplaySnapshot.Title,
				Author:       item.TrackDisplaySnapshot.Author,
				Album:        item.TrackDisplaySnapshot.Album,
				DurationMs:   item.TrackDisplaySnapshot.DurationMs,
				Availability: library.ListenLocalTrackMissing,
				ProbeError:   "track is no longer in the local music index",
			}
		}
		result.Items = append(result.Items, dto.ListenLocalPlaylistItemDTO{
			ID:       item.ID,
			Position: item.Position,
			AddedAt:  item.AddedAt.Format(time.RFC3339),
			Track:    trackDTO,
		})
	}
	return result, nil
}

func (service *LibraryService) CreateListenLocalPlaylist(ctx context.Context, request dto.CreateListenLocalPlaylistRequest) (dto.ListenLocalPlaylistDTO, error) {
	if service == nil || service.localPlaylists == nil {
		return dto.ListenLocalPlaylistDTO{}, library.ErrInvalidListenLocalPlaylist
	}
	now := service.now()
	playlist, err := library.NewListenLocalPlaylist(library.ListenLocalPlaylistParams{
		ID: uuid.NewString(), Name: request.Name, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		return dto.ListenLocalPlaylistDTO{}, err
	}
	if err := service.localPlaylists.Save(ctx, playlist); err != nil {
		return dto.ListenLocalPlaylistDTO{}, err
	}
	return toListenLocalPlaylistDTO(playlist, 0), nil
}

func (service *LibraryService) UpdateListenLocalPlaylist(ctx context.Context, request dto.UpdateListenLocalPlaylistRequest) (dto.ListenLocalPlaylistDTO, error) {
	if service == nil || service.localPlaylists == nil {
		return dto.ListenLocalPlaylistDTO{}, library.ErrListenLocalPlaylistNotFound
	}
	service.localPlaylistMutationMu.Lock()
	defer service.localPlaylistMutationMu.Unlock()
	current, err := service.localPlaylists.Get(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return dto.ListenLocalPlaylistDTO{}, err
	}
	if err := requireListenLocalPlaylistRevision(request.ExpectedRevision, current.Revision); err != nil {
		return dto.ListenLocalPlaylistDTO{}, err
	}
	now := service.now()
	updated, err := library.NewListenLocalPlaylist(library.ListenLocalPlaylistParams{
		ID: current.ID, Name: request.Name, Revision: current.Revision,
		CreatedAt: &current.CreatedAt, UpdatedAt: &now,
	})
	if err != nil {
		return dto.ListenLocalPlaylistDTO{}, err
	}
	if err := service.localPlaylists.Save(ctx, updated); err != nil {
		return dto.ListenLocalPlaylistDTO{}, err
	}
	saved, err := service.localPlaylists.Get(ctx, updated.ID)
	if err != nil {
		return dto.ListenLocalPlaylistDTO{}, err
	}
	items, err := service.localPlaylists.ListItems(ctx, saved.ID)
	if err != nil {
		return dto.ListenLocalPlaylistDTO{}, err
	}
	return toListenLocalPlaylistDTO(saved, len(items)), nil
}

func (service *LibraryService) DeleteListenLocalPlaylist(ctx context.Context, request dto.DeleteListenLocalPlaylistRequest) error {
	if service == nil || service.localPlaylists == nil {
		return library.ErrListenLocalPlaylistNotFound
	}
	service.localPlaylistMutationMu.Lock()
	defer service.localPlaylistMutationMu.Unlock()
	if request.ExpectedRevision < 1 {
		return library.ErrInvalidListenLocalPlaylist
	}
	return service.localPlaylists.Delete(ctx, strings.TrimSpace(request.ID), request.ExpectedRevision)
}

func (service *LibraryService) AddListenLocalPlaylistItems(ctx context.Context, request dto.AddListenLocalPlaylistItemsRequest) (dto.ListenLocalPlaylistDetailDTO, error) {
	if service == nil {
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrListenLocalPlaylistNotFound
	}
	service.localPlaylistMutationMu.Lock()
	defer service.localPlaylistMutationMu.Unlock()
	playlist, current, err := service.listenLocalPlaylistWithItems(ctx, request.ID)
	if err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	if err := requireListenLocalPlaylistRevision(request.ExpectedRevision, playlist.Revision); err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	now := service.now()
	for _, rawFileID := range request.FileIDs {
		fileID := strings.TrimSpace(rawFileID)
		if fileID == "" {
			return dto.ListenLocalPlaylistDetailDTO{}, library.ErrInvalidListenLocalPlaylist
		}
		if service.localTracks == nil {
			return dto.ListenLocalPlaylistDetailDTO{}, library.ErrFileNotFound
		}
		track, err := service.localTracks.Get(ctx, fileID)
		if err != nil {
			return dto.ListenLocalPlaylistDetailDTO{}, err
		}
		item, err := library.NewListenLocalPlaylistItem(playlist.ID, fileID, len(current), now)
		if err != nil {
			return dto.ListenLocalPlaylistDetailDTO{}, err
		}
		item.TrackDisplaySnapshot = library.ListenLocalTrackSnapshot(track)
		current = append(current, item)
	}
	if err := service.saveListenLocalPlaylistItems(ctx, playlist, current); err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	return service.GetListenLocalPlaylist(ctx, playlist.ID)
}

func (service *LibraryService) ReplaceListenLocalPlaylistItems(ctx context.Context, request dto.ReplaceListenLocalPlaylistItemsRequest) (dto.ListenLocalPlaylistDetailDTO, error) {
	if service == nil {
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrListenLocalPlaylistNotFound
	}
	service.localPlaylistMutationMu.Lock()
	defer service.localPlaylistMutationMu.Unlock()
	playlist, current, err := service.listenLocalPlaylistWithItems(ctx, request.ID)
	if err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	if err := requireListenLocalPlaylistRevision(request.ExpectedRevision, playlist.Revision); err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	useItemIDs := len(request.ItemIDs) > 0
	if useItemIDs && len(request.FileIDs) > 0 {
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrInvalidListenLocalPlaylist
	}
	orderedIDs := request.FileIDs
	byIdentity := make(map[string]library.ListenLocalPlaylistItem, len(current))
	if useItemIDs {
		orderedIDs = request.ItemIDs
		for _, item := range current {
			byIdentity[item.ID] = item
		}
	} else {
		// Legacy fileId ordering is safe only while every Track occurs once.
		// A duplicate Track has multiple stable Items and must never be silently
		// collapsed by the compatibility path.
		for _, item := range current {
			if _, duplicate := byIdentity[item.FileID]; duplicate {
				return dto.ListenLocalPlaylistDetailDTO{}, library.ErrInvalidListenLocalPlaylist
			}
			byIdentity[item.FileID] = item
		}
	}
	if len(orderedIDs) != len(current) {
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrInvalidListenLocalPlaylist
	}
	reordered := make([]library.ListenLocalPlaylistItem, 0, len(current))
	seen := make(map[string]struct{}, len(current))
	for position, rawID := range orderedIDs {
		identity := strings.TrimSpace(rawID)
		item, exists := byIdentity[identity]
		if !exists {
			return dto.ListenLocalPlaylistDetailDTO{}, library.ErrInvalidListenLocalPlaylist
		}
		if _, duplicate := seen[identity]; duplicate {
			return dto.ListenLocalPlaylistDetailDTO{}, library.ErrInvalidListenLocalPlaylist
		}
		item.Position = position
		reordered = append(reordered, item)
		seen[identity] = struct{}{}
	}
	if err := service.saveListenLocalPlaylistItems(ctx, playlist, reordered); err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	return service.GetListenLocalPlaylist(ctx, playlist.ID)
}

func (service *LibraryService) RemoveListenLocalPlaylistItem(ctx context.Context, request dto.RemoveListenLocalPlaylistItemRequest) (dto.ListenLocalPlaylistDetailDTO, error) {
	if service == nil {
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrListenLocalPlaylistNotFound
	}
	service.localPlaylistMutationMu.Lock()
	defer service.localPlaylistMutationMu.Unlock()
	playlist, current, err := service.listenLocalPlaylistWithItems(ctx, request.ID)
	if err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	if err := requireListenLocalPlaylistRevision(request.ExpectedRevision, playlist.Revision); err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	itemID := strings.TrimSpace(request.ItemID)
	fileID := strings.TrimSpace(request.FileID)
	if itemID == "" && fileID == "" {
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrInvalidListenLocalPlaylist
	}
	next := make([]library.ListenLocalPlaylistItem, 0, len(current))
	matchCount := 0
	for _, item := range current {
		matches := item.ID == itemID
		if itemID == "" {
			matches = item.FileID == fileID
		}
		if matches {
			matchCount++
			continue
		}
		item.Position = len(next)
		next = append(next, item)
	}
	if matchCount == 0 {
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrFileNotFound
	}
	if itemID == "" && matchCount > 1 {
		// The legacy fileId route cannot identify which duplicate Item to remove.
		// Reject it before persisting instead of deleting every occurrence.
		return dto.ListenLocalPlaylistDetailDTO{}, library.ErrInvalidListenLocalPlaylist
	}
	if err := service.saveListenLocalPlaylistItems(ctx, playlist, next); err != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, err
	}
	return service.GetListenLocalPlaylist(ctx, playlist.ID)
}

func (service *LibraryService) listenLocalPlaylistWithItems(ctx context.Context, id string) (library.ListenLocalPlaylist, []library.ListenLocalPlaylistItem, error) {
	if service == nil || service.localPlaylists == nil {
		return library.ListenLocalPlaylist{}, nil, library.ErrListenLocalPlaylistNotFound
	}
	playlist, err := service.localPlaylists.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return library.ListenLocalPlaylist{}, nil, err
	}
	items, err := service.localPlaylists.ListItems(ctx, playlist.ID)
	if err != nil {
		return library.ListenLocalPlaylist{}, nil, err
	}
	// Older databases, or cascades performed outside the local-track
	// repository, may contain sparse positions. Mutations operate on list order,
	// so normalize the domain slice and let the next atomic replacement repair
	// the persisted positions.
	for position := range items {
		items[position].Position = position
	}
	return playlist, items, nil
}

func (service *LibraryService) saveListenLocalPlaylistItems(ctx context.Context, playlist library.ListenLocalPlaylist, items []library.ListenLocalPlaylistItem) error {
	playlist.UpdatedAt = service.now()
	return service.localPlaylists.ReplaceItems(ctx, playlist, items)
}

func requireListenLocalPlaylistRevision(expected, current int64) error {
	if expected < 1 {
		return library.ErrInvalidListenLocalPlaylist
	}
	if expected != current {
		return &library.ListenLocalMusicRevisionConflictError{CurrentRevision: current}
	}
	return nil
}

func toListenLocalPlaylistDTO(item library.ListenLocalPlaylist, itemCount int) dto.ListenLocalPlaylistDTO {
	return dto.ListenLocalPlaylistDTO{
		ID: item.ID, Name: item.Name, Revision: item.Revision, ItemCount: itemCount,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}
