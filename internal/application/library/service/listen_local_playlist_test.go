package service

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
)

type gatedListenLocalPlaylistRepository struct {
	library.ListenLocalPlaylistRepository
	calls        atomic.Int32
	firstEntered chan struct{}
	releaseFirst chan struct{}
	overlapped   chan struct{}
}

type queryCounter struct{ count atomic.Int32 }

func (counter *queryCounter) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	counter.count.Add(1)
	return ctx
}

func (*queryCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

type omittingListenLocalTrackRepository struct {
	library.ListenLocalTrackRepository
	omitFileID string
}

func (repo *omittingListenLocalTrackRepository) List(ctx context.Context, options library.ListenLocalTrackListOptions) ([]library.ListenLocalTrack, error) {
	tracks, err := repo.ListenLocalTrackRepository.List(ctx, options)
	if err != nil {
		return nil, err
	}
	result := make([]library.ListenLocalTrack, 0, len(tracks))
	for _, track := range tracks {
		if track.FileID != repo.omitFileID {
			result = append(result, track)
		}
	}
	return result, nil
}

func (repo *gatedListenLocalPlaylistRepository) ListItems(ctx context.Context, playlistID string) ([]library.ListenLocalPlaylistItem, error) {
	if repo.calls.Add(1) == 1 {
		close(repo.firstEntered)
		<-repo.releaseFirst
	} else {
		select {
		case <-repo.releaseFirst:
		default:
			select {
			case repo.overlapped <- struct{}{}:
			default:
			}
		}
	}
	return repo.ListenLocalPlaylistRepository.ListItems(ctx, playlistID)
}

func TestListenLocalPlaylistLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openLibraryServiceTestDatabase(t, "playlist-service.db")

	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{ID: "library-1", Name: "Music", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}
	if err := libraryrepo.NewSQLiteLibraryRepository(db.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	fileRepo := libraryrepo.NewSQLiteFileRepository(db.Bun)
	trackRepo := libraryrepo.NewSQLiteListenLocalTrackRepository(db.Bun)
	for _, id := range []string{"file-a", "file-b"} {
		path := filepath.Join(t.TempDir(), id+".mp3")
		fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
			ID: id, LibraryID: libraryItem.ID, Kind: string(library.FileKindAudio), Name: id + ".mp3",
			Storage:   library.FileStorage{Mode: "local_path", LocalPath: path},
			Origin:    library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{ImportPath: path}},
			CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new file: %v", err)
		}
		if err := fileRepo.Save(ctx, fileItem); err != nil {
			t.Fatalf("save file: %v", err)
		}
		track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
			FileID: id, LibraryID: libraryItem.ID, LocalPath: path, Title: id,
			Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &now,
			CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new track: %v", err)
		}
		if err := trackRepo.Save(ctx, track); err != nil {
			t.Fatalf("save track: %v", err)
		}
	}

	playlistRepo := libraryrepo.NewSQLiteListenLocalPlaylistRepository(db.Bun)
	service := &LibraryService{
		localTracks:    trackRepo,
		localPlaylists: playlistRepo,
		nowFunc:        func() time.Time { return now },
	}
	created, err := service.CreateListenLocalPlaylist(ctx, dto.CreateListenLocalPlaylistRequest{Name: " Favorites "})
	if err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	if created.Name != "Favorites" || created.Revision != 1 || created.ItemCount != 0 {
		t.Fatalf("unexpected created playlist: %#v", created)
	}
	created, err = service.UpdateListenLocalPlaylist(ctx, dto.UpdateListenLocalPlaylistRequest{
		ID: created.ID, Name: "Road Trip", ExpectedRevision: created.Revision,
	})
	if err != nil || created.Name != "Road Trip" || created.Revision != 2 {
		t.Fatalf("rename current playlist: %#v err=%v", created, err)
	}
	detail, err := service.AddListenLocalPlaylistItems(ctx, dto.AddListenLocalPlaylistItemsRequest{
		ID: created.ID, FileIDs: []string{"file-a", "file-b"}, ExpectedRevision: created.Revision,
	})
	if err != nil {
		t.Fatalf("add playlist items: %v", err)
	}
	if len(detail.Items) != 2 || detail.Items[0].Track.FileID != "file-a" || detail.Items[1].Track.FileID != "file-b" {
		t.Fatalf("unexpected playlist detail: %#v", detail)
	}
	if detail.Items[0].ID == "" || detail.Items[1].ID == "" || detail.Items[0].ID == detail.Items[1].ID {
		t.Fatalf("playlist detail did not expose stable item identities: %#v", detail.Items)
	}
	if _, err := db.Bun.NewUpdate().Table("listen_local_playlist_items").
		Set("position = ?", 3).
		Where("playlist_id = ?", created.ID).
		Where("file_id = ?", "file-b").
		Exec(ctx); err != nil {
		t.Fatalf("create sparse legacy playlist positions: %v", err)
	}
	detail, err = service.ReplaceListenLocalPlaylistItems(ctx, dto.ReplaceListenLocalPlaylistItemsRequest{
		ID: created.ID, FileIDs: []string{"file-a", "file-b"}, ExpectedRevision: detail.Playlist.Revision,
	})
	if err != nil {
		t.Fatalf("repair sparse positions during legacy reorder: %v", err)
	}
	if len(detail.Items) != 2 || detail.Items[0].Position != 0 || detail.Items[1].Position != 1 {
		t.Fatalf("sparse playlist positions were not repaired: %#v", detail.Items)
	}
	detail, err = service.ReplaceListenLocalPlaylistItems(ctx, dto.ReplaceListenLocalPlaylistItemsRequest{
		ID: created.ID, FileIDs: []string{"file-b", "file-a"}, ExpectedRevision: detail.Playlist.Revision,
	})
	if err != nil {
		t.Fatalf("reorder playlist items: %v", err)
	}
	if detail.Items[0].Track.FileID != "file-b" || detail.Items[1].Track.FileID != "file-a" {
		t.Fatalf("unexpected reorder: %#v", detail.Items)
	}
	fileBItemID := detail.Items[0].ID
	detail, err = service.RemoveListenLocalPlaylistItem(ctx, dto.RemoveListenLocalPlaylistItemRequest{
		ID: created.ID, ItemID: fileBItemID, ExpectedRevision: detail.Playlist.Revision,
	})
	if err != nil {
		t.Fatalf("remove playlist item: %v", err)
	}
	if len(detail.Items) != 1 || detail.Items[0].Position != 0 || detail.Items[0].Track.FileID != "file-a" {
		t.Fatalf("unexpected items after remove: %#v", detail.Items)
	}

	duplicateDetail, err := service.AddListenLocalPlaylistItems(ctx, dto.AddListenLocalPlaylistItemsRequest{
		ID: created.ID, FileIDs: []string{"file-a", "file-a"}, ExpectedRevision: detail.Playlist.Revision,
	})
	if err != nil || len(duplicateDetail.Items) != 3 ||
		duplicateDetail.Items[0].ID == duplicateDetail.Items[1].ID ||
		duplicateDetail.Items[0].ID == duplicateDetail.Items[2].ID ||
		duplicateDetail.Items[1].ID == duplicateDetail.Items[2].ID {
		t.Fatalf("duplicate Track items lost stable identity: %#v err=%v", duplicateDetail.Items, err)
	}
	if _, err := service.ReplaceListenLocalPlaylistItems(ctx, dto.ReplaceListenLocalPlaylistItemsRequest{
		ID: created.ID, FileIDs: []string{"file-a", "file-a", "file-a"}, ExpectedRevision: duplicateDetail.Playlist.Revision,
	}); !errors.Is(err, library.ErrInvalidListenLocalPlaylist) {
		t.Fatalf("ambiguous legacy reorder error=%v, want invalid playlist", err)
	}
	if _, err := service.RemoveListenLocalPlaylistItem(ctx, dto.RemoveListenLocalPlaylistItemRequest{
		ID: created.ID, FileID: "file-a", ExpectedRevision: duplicateDetail.Playlist.Revision,
	}); !errors.Is(err, library.ErrInvalidListenLocalPlaylist) {
		t.Fatalf("ambiguous legacy remove error=%v, want invalid playlist", err)
	}
	unchanged, err := service.GetListenLocalPlaylist(ctx, created.ID)
	if err != nil || len(unchanged.Items) != 3 {
		t.Fatalf("ambiguous legacy call removed duplicate items: %#v err=%v", unchanged.Items, err)
	}
	reversedIDs := []string{unchanged.Items[2].ID, unchanged.Items[1].ID, unchanged.Items[0].ID}
	reordered, err := service.ReplaceListenLocalPlaylistItems(ctx, dto.ReplaceListenLocalPlaylistItemsRequest{
		ID: created.ID, ItemIDs: reversedIDs, ExpectedRevision: unchanged.Playlist.Revision,
	})
	if err != nil || len(reordered.Items) != 3 || reordered.Items[0].ID != reversedIDs[0] ||
		reordered.Items[1].ID != reversedIDs[1] || reordered.Items[2].ID != reversedIDs[2] {
		t.Fatalf("identity reorder result=%#v err=%v", reordered.Items, err)
	}
	remaining, err := service.RemoveListenLocalPlaylistItem(ctx, dto.RemoveListenLocalPlaylistItemRequest{
		ID: created.ID, ItemID: reversedIDs[0], ExpectedRevision: reordered.Playlist.Revision,
	})
	if err != nil || len(remaining.Items) != 2 || remaining.Items[0].ID != reversedIDs[1] || remaining.Items[1].ID != reversedIDs[2] {
		t.Fatalf("identity remove result=%#v err=%v", remaining.Items, err)
	}
	staleRevision := remaining.Playlist.Revision - 1
	staleMutations := []struct {
		name string
		run  func() error
	}{
		{
			name: "rename",
			run: func() error {
				_, err := service.UpdateListenLocalPlaylist(ctx, dto.UpdateListenLocalPlaylistRequest{
					ID: created.ID, Name: "Stale Rename", ExpectedRevision: staleRevision,
				})
				return err
			},
		},
		{
			name: "delete",
			run: func() error {
				return service.DeleteListenLocalPlaylist(ctx, dto.DeleteListenLocalPlaylistRequest{
					ID: created.ID, ExpectedRevision: staleRevision,
				})
			},
		},
		{
			name: "add",
			run: func() error {
				_, err := service.AddListenLocalPlaylistItems(ctx, dto.AddListenLocalPlaylistItemsRequest{
					ID: created.ID, FileIDs: []string{"file-b"}, ExpectedRevision: staleRevision,
				})
				return err
			},
		},
		{
			name: "reorder",
			run: func() error {
				_, err := service.ReplaceListenLocalPlaylistItems(ctx, dto.ReplaceListenLocalPlaylistItemsRequest{
					ID:               created.ID,
					ItemIDs:          []string{remaining.Items[1].ID, remaining.Items[0].ID},
					ExpectedRevision: staleRevision,
				})
				return err
			},
		},
		{
			name: "remove",
			run: func() error {
				_, err := service.RemoveListenLocalPlaylistItem(ctx, dto.RemoveListenLocalPlaylistItemRequest{
					ID: created.ID, ItemID: remaining.Items[0].ID, ExpectedRevision: staleRevision,
				})
				return err
			},
		},
	}
	for _, mutation := range staleMutations {
		t.Run("stale "+mutation.name, func(t *testing.T) {
			if err := mutation.run(); !errors.Is(err, library.ErrListenLocalMusicRevisionConflict) {
				t.Fatalf("error=%v, want revision conflict", err)
			}
		})
	}
	unchangedAfterStale, err := service.GetListenLocalPlaylist(ctx, created.ID)
	if err != nil || unchangedAfterStale.Playlist.Revision != remaining.Playlist.Revision ||
		len(unchangedAfterStale.Items) != len(remaining.Items) || unchangedAfterStale.Playlist.Name != "Road Trip" {
		t.Fatalf("stale mutations changed playlist: %#v err=%v", unchangedAfterStale, err)
	}
	if err := service.DeleteListenLocalPlaylist(ctx, dto.DeleteListenLocalPlaylistRequest{
		ID: created.ID, ExpectedRevision: remaining.Playlist.Revision,
	}); err != nil {
		t.Fatalf("delete playlist: %v", err)
	}
}

func TestListenLocalPlaylistConcurrentStaleAddConflictsWithoutOverwrite(t *testing.T) {
	ctx := context.Background()
	db := openLibraryServiceTestDatabase(t, "playlist-concurrency.db")

	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{ID: "library-1", Name: "Music", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}
	if err := libraryrepo.NewSQLiteLibraryRepository(db.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	fileRepo := libraryrepo.NewSQLiteFileRepository(db.Bun)
	trackRepo := libraryrepo.NewSQLiteListenLocalTrackRepository(db.Bun)
	for _, id := range []string{"file-a", "file-b"} {
		path := filepath.Join(t.TempDir(), id+".mp3")
		fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
			ID: id, LibraryID: libraryItem.ID, Kind: string(library.FileKindAudio), Name: id + ".mp3",
			Storage:   library.FileStorage{Mode: "local_path", LocalPath: path},
			Origin:    library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{ImportPath: path}},
			CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new file: %v", err)
		}
		if err := fileRepo.Save(ctx, fileItem); err != nil {
			t.Fatalf("save file: %v", err)
		}
		track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
			FileID: id, LibraryID: libraryItem.ID, LocalPath: path, Title: id,
			Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &now,
			CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new track: %v", err)
		}
		if err := trackRepo.Save(ctx, track); err != nil {
			t.Fatalf("save track: %v", err)
		}
	}

	baseRepo := libraryrepo.NewSQLiteListenLocalPlaylistRepository(db.Bun)
	gatedRepo := &gatedListenLocalPlaylistRepository{
		ListenLocalPlaylistRepository: baseRepo,
		firstEntered:                  make(chan struct{}),
		releaseFirst:                  make(chan struct{}),
		overlapped:                    make(chan struct{}, 1),
	}
	service := &LibraryService{localTracks: trackRepo, localPlaylists: gatedRepo, nowFunc: func() time.Time { return now }}
	created, err := service.CreateListenLocalPlaylist(ctx, dto.CreateListenLocalPlaylistRequest{Name: "Concurrent"})
	if err != nil {
		t.Fatalf("create playlist: %v", err)
	}

	results := make(chan error, 2)
	go func() {
		_, err := service.AddListenLocalPlaylistItems(ctx, dto.AddListenLocalPlaylistItemsRequest{
			ID: created.ID, FileIDs: []string{"file-a"}, ExpectedRevision: created.Revision,
		})
		results <- err
	}()
	select {
	case <-gatedRepo.firstEntered:
	case <-time.After(time.Second):
		t.Fatal("first mutation did not reach item read")
	}
	go func() {
		_, err := service.AddListenLocalPlaylistItems(ctx, dto.AddListenLocalPlaylistItemsRequest{
			ID: created.ID, FileIDs: []string{"file-b"}, ExpectedRevision: created.Revision,
		})
		results <- err
	}()
	select {
	case <-gatedRepo.overlapped:
		t.Fatal("playlist mutations overlapped their read-modify-replace sections")
	case <-time.After(100 * time.Millisecond):
	}
	close(gatedRepo.releaseFirst)
	conflicts := 0
	for range 2 {
		if err := <-results; err != nil {
			if !errors.Is(err, library.ErrListenLocalMusicRevisionConflict) {
				t.Fatalf("concurrent add: %v", err)
			}
			conflicts++
		}
	}
	if conflicts != 1 {
		t.Fatalf("concurrent stale adds conflicts=%d, want one", conflicts)
	}
	detail, err := service.GetListenLocalPlaylist(ctx, created.ID)
	if err != nil {
		t.Fatalf("get playlist: %v", err)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("stale concurrent addition overwrote the winner: %#v", detail.Items)
	}
}

func TestListenLocalPlaylistReadsUseBoundedQueriesAndPreserveMissingTrackError(t *testing.T) {
	ctx := context.Background()
	db := openLibraryServiceTestDatabase(t, "playlist-reads.db")

	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{ID: "library-1", Name: "Music", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}
	if err := libraryrepo.NewSQLiteLibraryRepository(db.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	fileRepo := libraryrepo.NewSQLiteFileRepository(db.Bun)
	trackRepo := libraryrepo.NewSQLiteListenLocalTrackRepository(db.Bun)
	for _, id := range []string{"file-a", "file-b", "file-without-track"} {
		path := filepath.Join(t.TempDir(), id+".mp3")
		fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
			ID: id, LibraryID: libraryItem.ID, Kind: string(library.FileKindAudio), Name: id + ".mp3",
			Storage:   library.FileStorage{Mode: "local_path", LocalPath: path},
			Origin:    library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{ImportPath: path}},
			CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new file: %v", err)
		}
		if err := fileRepo.Save(ctx, fileItem); err != nil {
			t.Fatalf("save file: %v", err)
		}
		availability := library.ListenLocalTrackAvailable
		if id == "file-b" {
			availability = library.ListenLocalTrackMissing
		}
		track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
			FileID: id, LibraryID: libraryItem.ID, LocalPath: path, Title: id,
			Availability: availability, LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new track: %v", err)
		}
		if err := trackRepo.Save(ctx, track); err != nil {
			t.Fatalf("save track: %v", err)
		}
	}

	playlistRepo := libraryrepo.NewSQLiteListenLocalPlaylistRepository(db.Bun)
	playlistItems := map[string][]string{
		"playlist-a": {"file-b", "file-a"},
		"playlist-b": {"file-a"},
		"playlist-c": {"file-without-track"},
	}
	for index, playlistID := range []string{"playlist-a", "playlist-b", "playlist-c"} {
		updatedAt := now.Add(time.Duration(index) * time.Minute)
		playlist, err := library.NewListenLocalPlaylist(library.ListenLocalPlaylistParams{
			ID: playlistID, Name: playlistID, CreatedAt: &now, UpdatedAt: &updatedAt,
		})
		if err != nil {
			t.Fatalf("new playlist: %v", err)
		}
		if err := playlistRepo.Save(ctx, playlist); err != nil {
			t.Fatalf("save playlist: %v", err)
		}
		items := make([]library.ListenLocalPlaylistItem, 0, len(playlistItems[playlistID]))
		for position, fileID := range playlistItems[playlistID] {
			item, err := library.NewListenLocalPlaylistItem(playlistID, fileID, position, now)
			if err != nil {
				t.Fatalf("new playlist item: %v", err)
			}
			items = append(items, item)
		}
		if err := playlistRepo.ReplaceItems(ctx, playlist, items); err != nil {
			t.Fatalf("replace playlist items: %v", err)
		}
	}

	service := &LibraryService{
		localTracks: &omittingListenLocalTrackRepository{
			ListenLocalTrackRepository: trackRepo,
			omitFileID:                 "file-without-track",
		},
		localPlaylists: playlistRepo,
		nowFunc:        func() time.Time { return now },
	}
	counter := new(queryCounter)
	db.Bun.AddQueryHook(counter)

	playlists, err := service.ListListenLocalPlaylists(ctx)
	if err != nil {
		t.Fatalf("list playlists: %v", err)
	}
	if got := counter.count.Load(); got != 2 {
		t.Fatalf("playlist listing used %d queries, want 2", got)
	}
	counts := make(map[string]int, len(playlists))
	for _, playlist := range playlists {
		counts[playlist.ID] = playlist.ItemCount
	}
	if counts["playlist-a"] != 2 || counts["playlist-b"] != 1 || counts["playlist-c"] != 1 {
		t.Fatalf("unexpected playlist counts: %#v", counts)
	}

	counter.count.Store(0)
	detail, err := service.GetListenLocalPlaylist(ctx, "playlist-a")
	if err != nil {
		t.Fatalf("get playlist: %v", err)
	}
	if got := counter.count.Load(); got != 3 {
		t.Fatalf("playlist detail used %d queries, want 3", got)
	}
	if len(detail.Items) != 2 || detail.Items[0].Track.FileID != "file-b" || detail.Items[1].Track.FileID != "file-a" {
		t.Fatalf("playlist detail did not preserve item order: %#v", detail.Items)
	}

	counter.count.Store(0)
	missingDetail, err := service.GetListenLocalPlaylist(ctx, "playlist-c")
	if err != nil {
		t.Fatalf("missing playlist track fallback error = %v", err)
	}
	if len(missingDetail.Items) != 1 || missingDetail.Items[0].Track.FileID != "file-without-track" ||
		missingDetail.Items[0].Track.Availability != library.ListenLocalTrackMissing || missingDetail.Items[0].Track.Title == "" {
		t.Fatalf("missing playlist track did not use display snapshot: %#v", missingDetail.Items)
	}
	if got := counter.count.Load(); got != 3 {
		t.Fatalf("missing-track playlist detail used %d queries, want 3", got)
	}
}
