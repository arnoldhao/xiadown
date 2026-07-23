package libraryrepo

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSQLiteListenLocalPlaylistRepositoryRoundTripAndOrdering(t *testing.T) {
	ctx := context.Background()
	db, err := openLibraryRepoTestDatabase(t, ctx, "local-playlists.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{ID: "library-1", Name: "Music", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}
	if err := NewSQLiteLibraryRepository(db.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	trackRepo := NewSQLiteListenLocalTrackRepository(db.Bun)
	fileRepo := NewSQLiteFileRepository(db.Bun)
	for index, id := range []string{"file-a", "file-b", "file-c"} {
		fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
			ID: id, LibraryID: libraryItem.ID, Kind: string(library.FileKindAudio), Name: id + ".mp3",
			Storage:   library.FileStorage{Mode: "local_path", LocalPath: filepath.Join(t.TempDir(), id+".mp3")},
			Origin:    library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{ImportPath: filepath.Join(t.TempDir(), id+".mp3")}},
			CreatedAt: &now, UpdatedAt: &now,
		})
		if err != nil {
			t.Fatalf("new file: %v", err)
		}
		if err := fileRepo.Save(ctx, fileItem); err != nil {
			t.Fatalf("save file: %v", err)
		}
		track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
			FileID: id, LibraryID: libraryItem.ID, LocalPath: fileItem.Storage.LocalPath,
			Title: id, Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &now,
			CreatedAt: &now, UpdatedAt: &now, ModTimeUnix: int64(index),
		})
		if err != nil {
			t.Fatalf("new track: %v", err)
		}
		if err := trackRepo.Save(ctx, track); err != nil {
			t.Fatalf("save track: %v", err)
		}
	}

	repo := NewSQLiteListenLocalPlaylistRepository(db.Bun)
	playlist, err := library.NewListenLocalPlaylist(library.ListenLocalPlaylistParams{ID: "playlist-1", Name: "Favorites", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatalf("new playlist: %v", err)
	}
	if err := repo.Save(ctx, playlist); err != nil {
		t.Fatalf("save playlist: %v", err)
	}
	stalePlaylistMetadata := playlist
	renamedPlaylist := playlist
	renamedPlaylist.Name = "Road Trip"
	renamedPlaylist.UpdatedAt = now.Add(time.Second)
	if err := repo.Save(ctx, renamedPlaylist); err != nil {
		t.Fatalf("rename playlist with current revision: %v", err)
	}
	playlist, err = repo.Get(ctx, playlist.ID)
	if err != nil || playlist.Name != "Road Trip" || playlist.Revision != stalePlaylistMetadata.Revision+1 {
		t.Fatalf("renamed playlist=%#v err=%v", playlist, err)
	}
	stalePlaylistMetadata.Name = "Stale Rename"
	stalePlaylistMetadata.UpdatedAt = now.Add(2 * time.Second)
	if err := repo.Save(ctx, stalePlaylistMetadata); !errors.Is(err, library.ErrListenLocalMusicRevisionConflict) {
		t.Fatalf("stale rename error=%v, want revision conflict", err)
	}
	if err := repo.Delete(ctx, playlist.ID, stalePlaylistMetadata.Revision); !errors.Is(err, library.ErrListenLocalMusicRevisionConflict) {
		t.Fatalf("stale delete error=%v, want revision conflict", err)
	}
	itemA, _ := library.NewListenLocalPlaylistItem(playlist.ID, "file-a", 0, now)
	itemB, _ := library.NewListenLocalPlaylistItem(playlist.ID, "file-b", 1, now.Add(time.Second))
	itemC, _ := library.NewListenLocalPlaylistItem(playlist.ID, "file-c", 2, now.Add(2*time.Second))
	playlist.UpdatedAt = now.Add(2 * time.Second)
	if err := repo.ReplaceItems(ctx, playlist, []library.ListenLocalPlaylistItem{itemA, itemB, itemC}); err != nil {
		t.Fatalf("replace items: %v", err)
	}
	storedPlaylist, err := repo.Get(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("get playlist after replace: %v", err)
	}
	playlist.Revision = storedPlaylist.Revision
	playlist.UpdatedAt = now.Add(2500 * time.Millisecond)
	if err := repo.ReplaceItems(ctx, playlist, []library.ListenLocalPlaylistItem{itemA, itemB, itemC}); err != nil {
		t.Fatalf("no-op replace items: %v", err)
	}
	noOpPlaylist, err := repo.Get(ctx, playlist.ID)
	if err != nil || noOpPlaylist.Revision != storedPlaylist.Revision || !noOpPlaylist.UpdatedAt.Equal(storedPlaylist.UpdatedAt) {
		t.Fatalf("no-op playlist replace = %#v, prior=%#v, err=%v", noOpPlaylist, storedPlaylist, err)
	}
	items, err := repo.ListItems(ctx, playlist.ID)
	if err != nil || len(items) != 3 || items[0].FileID != "file-a" || items[1].FileID != "file-b" || items[2].FileID != "file-c" {
		t.Fatalf("unexpected items: %#v, err=%v", items, err)
	}
	counts, err := repo.CountItems(ctx, []string{playlist.ID, "empty-playlist", playlist.ID})
	if err != nil {
		t.Fatalf("count items: %v", err)
	}
	if counts[playlist.ID] != 3 || counts["empty-playlist"] != 0 {
		t.Fatalf("unexpected item counts: %#v", counts)
	}
	selectedTracks, err := trackRepo.List(ctx, library.ListenLocalTrackListOptions{
		FileIDs:            []string{"file-b", "file-b"},
		IncludeUnavailable: true,
	})
	if err != nil {
		t.Fatalf("list tracks by file IDs: %v", err)
	}
	if len(selectedTracks) != 1 || selectedTracks[0].FileID != "file-b" {
		t.Fatalf("unexpected tracks selected by file ID: %#v", selectedTracks)
	}
	itemB.Position = 0
	itemA.Position = 1
	itemC.Position = 2
	playlist.Revision = noOpPlaylist.Revision
	playlist.UpdatedAt = now.Add(3 * time.Second)
	if err := repo.ReplaceItems(ctx, playlist, []library.ListenLocalPlaylistItem{itemB, itemA, itemC}); err != nil {
		t.Fatalf("reorder items: %v", err)
	}
	items, err = repo.ListItems(ctx, playlist.ID)
	if err != nil || items[0].FileID != "file-b" || items[1].FileID != "file-a" || items[2].FileID != "file-c" {
		t.Fatalf("unexpected reordered items: %#v, err=%v", items, err)
	}

	// Model a paired-device commit between the legacy service's read and its
	// ReplaceItems call. The stale replacement must roll back before deleting
	// the newly committed stable Item.
	stalePlaylist, err := repo.Get(ctx, playlist.ID)
	if err != nil {
		t.Fatal(err)
	}
	pairedItem, err := library.NewListenLocalPlaylistItem(playlist.ID, "file-c", 3, now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	pairedRow := listenLocalPlaylistItemRow{
		ID: pairedItem.ID, PlaylistID: pairedItem.PlaylistID, FileID: pairedItem.FileID,
		Position: pairedItem.Position, AddedAt: pairedItem.AddedAt, Revision: pairedItem.Revision,
		TrackDisplayTitle: "file-c",
	}
	if _, err := db.Bun.NewInsert().Model(&pairedRow).Exec(ctx); err != nil {
		t.Fatalf("seed paired item: %v", err)
	}
	if _, err := db.Bun.NewUpdate().Model((*listenLocalPlaylistRow)(nil)).
		Set("revision = revision + 1").Set("updated_at = ?", now.Add(4*time.Second)).
		Where("id = ?", playlist.ID).Exec(ctx); err != nil {
		t.Fatalf("commit paired playlist revision: %v", err)
	}
	stalePlaylist.UpdatedAt = now.Add(5 * time.Second)
	if err := repo.ReplaceItems(ctx, stalePlaylist, []library.ListenLocalPlaylistItem{itemB, itemA, itemC}); !errors.Is(err, library.ErrListenLocalMusicRevisionConflict) {
		t.Fatalf("stale legacy replacement error=%v, want revision conflict", err)
	}
	items, err = repo.ListItems(ctx, playlist.ID)
	if err != nil || len(items) != 4 || items[3].ID != pairedItem.ID {
		t.Fatalf("stale legacy replacement overwrote paired item: %#v err=%v", items, err)
	}
	currentPlaylist, err := repo.Get(ctx, playlist.ID)
	if err != nil {
		t.Fatal(err)
	}
	currentPlaylist.UpdatedAt = now.Add(6 * time.Second)
	if err := repo.ReplaceItems(ctx, currentPlaylist, []library.ListenLocalPlaylistItem{itemB, itemA, itemC}); err != nil {
		t.Fatalf("fresh replacement after conflict: %v", err)
	}
	if err := trackRepo.Delete(ctx, "file-a"); err != nil {
		t.Fatalf("delete middle playlist track: %v", err)
	}
	items, err = repo.ListItems(ctx, playlist.ID)
	if err != nil || len(items) != 3 || items[0].FileID != "file-b" || items[0].Position != 0 ||
		items[1].FileID != "file-a" || items[1].Position != 1 || items[1].TrackDisplaySnapshot.Title != "file-a" ||
		items[2].FileID != "file-c" || items[2].Position != 2 {
		t.Fatalf("playlist item/display snapshot was not retained after track delete: %#v, err=%v", items, err)
	}
	if _, err := db.Bun.NewUpdate().Model((*listenLocalTrackRow)(nil)).
		Set("availability = ?", library.ListenLocalTrackMissing).
		Where("file_id = ?", "file-c").
		Exec(ctx); err != nil {
		t.Fatalf("mark track missing: %v", err)
	}
	removed, err := trackRepo.DeleteUnavailable(ctx)
	if err != nil || removed != 1 {
		t.Fatalf("delete unavailable tracks: removed=%d err=%v", removed, err)
	}
	items, err = repo.ListItems(ctx, playlist.ID)
	if err != nil || len(items) != 3 || items[2].FileID != "file-c" || items[2].TrackDisplaySnapshot.Title != "file-c" {
		t.Fatalf("playlist item/display snapshot was not retained after clearing missing tracks: %#v, err=%v", items, err)
	}
	playlistBeforeDelete, err := repo.Get(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("get playlist before delete: %v", err)
	}
	if err := repo.Delete(ctx, playlist.ID, playlistBeforeDelete.Revision); err != nil {
		t.Fatalf("delete playlist: %v", err)
	}
	if _, err := repo.Get(ctx, playlist.ID); !errors.Is(err, library.ErrListenLocalPlaylistNotFound) {
		t.Fatalf("expected missing playlist, got %v", err)
	}
}
