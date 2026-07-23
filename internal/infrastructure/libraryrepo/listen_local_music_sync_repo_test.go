package libraryrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSQLiteListenLocalMusicRepositoriesRevisionTombstoneAndDuplicateItemFoundation(t *testing.T) {
	ctx := context.Background()
	db, err := openLibraryRepoTestDatabase(t, ctx, "music-sync.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 21, 11, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{ID: "library-1", Name: "Music", CreatedAt: &now, UpdatedAt: &now})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteLibraryRepository(db.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatal(err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "track-1", LibraryID: libraryItem.ID, Kind: string(library.FileKindAudio), Name: "track.mp3",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: "/tmp/track.mp3"},
		Origin:    library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{ImportPath: "/tmp/track.mp3"}},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteFileRepository(db.Bun).Save(ctx, fileItem); err != nil {
		t.Fatal(err)
	}

	duration := int64(180_000)
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: fileItem.ID, LibraryID: libraryItem.ID, LocalPath: fileItem.Storage.LocalPath,
		Title: "Original", Author: "Artist", Album: "Album", DurationMs: &duration,
		Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	trackRepo := NewSQLiteListenLocalTrackRepository(db.Bun)
	if err := trackRepo.Save(ctx, track); err != nil {
		t.Fatal(err)
	}
	stored, err := trackRepo.Get(ctx, track.FileID)
	if err != nil || stored.Revision != 1 || stored.ContentIdentityRevision != 1 ||
		stored.MetadataRevision != 1 || stored.ResourceRevision != 1 {
		t.Fatalf("initial Track = %#v err=%v", stored, err)
	}

	stored.LastCheckedAt = now.Add(time.Minute)
	stored.UpdatedAt = now.Add(time.Minute)
	if err := trackRepo.Save(ctx, stored); err != nil {
		t.Fatal(err)
	}
	stored, _ = trackRepo.Get(ctx, track.FileID)
	if stored.Revision != 1 {
		t.Fatalf("diagnostic timestamp-only save advanced revision to %d", stored.Revision)
	}
	if !stored.UpdatedAt.Equal(now) {
		t.Fatalf("diagnostic timestamp-only save changed entity updatedAt to %s", stored.UpdatedAt)
	}
	stored.Title = "Edited"
	stored.UpdatedAt = now.Add(2 * time.Minute)
	if err := trackRepo.Save(ctx, stored); err != nil {
		t.Fatal(err)
	}
	stored, _ = trackRepo.Get(ctx, track.FileID)
	if stored.Revision != 2 || stored.ContentIdentityRevision != 1 ||
		stored.MetadataRevision != 2 || stored.ResourceRevision != 1 {
		t.Fatalf("metadata edit revisions = (%d,%d,%d,%d), want (2,1,2,1)",
			stored.Revision, stored.ContentIdentityRevision, stored.MetadataRevision, stored.ResourceRevision)
	}
	changedDuration := int64(181_000)
	stored.DurationMs = &changedDuration
	stored.UpdatedAt = now.Add(3 * time.Minute)
	if err := trackRepo.Save(ctx, stored); err != nil {
		t.Fatal(err)
	}
	stored, _ = trackRepo.Get(ctx, track.FileID)
	if stored.Revision != 3 || stored.ContentIdentityRevision != 2 ||
		stored.MetadataRevision != 2 || stored.ResourceRevision != 2 {
		t.Fatalf("timeline edit revisions = (%d,%d,%d,%d), want (3,2,2,2)",
			stored.Revision, stored.ContentIdentityRevision, stored.MetadataRevision, stored.ResourceRevision)
	}

	playlistRepo := NewSQLiteListenLocalPlaylistRepository(db.Bun)
	playlist, err := library.NewListenLocalPlaylist(library.ListenLocalPlaylistParams{
		ID: "playlist-1", Name: "Duplicates", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := playlistRepo.Save(ctx, playlist); err != nil {
		t.Fatal(err)
	}
	first, _ := library.NewListenLocalPlaylistItem(playlist.ID, track.FileID, 0, now)
	second, _ := library.NewListenLocalPlaylistItem(playlist.ID, track.FileID, 1, now.Add(time.Second))
	playlist.UpdatedAt = now.Add(4 * time.Minute)
	if err := playlistRepo.ReplaceItems(ctx, playlist, []library.ListenLocalPlaylistItem{first, second}); err != nil {
		t.Fatalf("persist duplicate Track playlist items: %v", err)
	}
	items, err := playlistRepo.ListItems(ctx, playlist.ID)
	if err != nil || len(items) != 2 || items[0].ID == items[1].ID || items[0].FileID != items[1].FileID {
		t.Fatalf("duplicate Track items = %#v err=%v", items, err)
	}

	if err := trackRepo.Delete(ctx, track.FileID); err != nil {
		t.Fatal(err)
	}
	if _, err := trackRepo.Get(ctx, track.FileID); !errors.Is(err, library.ErrFileNotFound) {
		t.Fatalf("deleted Track get error = %v", err)
	}
	items, err = playlistRepo.ListItems(ctx, playlist.ID)
	if err != nil || len(items) != 2 || items[0].TrackDisplaySnapshot.Title != "Edited" || items[1].TrackDisplaySnapshot.Title != "Edited" {
		t.Fatalf("Track deletion did not preserve item snapshots: %#v err=%v", items, err)
	}
	var tombstoneRevision, tombstoneIdentity, tombstoneMetadata, tombstoneResource int64
	if err := db.SQL.QueryRowContext(ctx, `
SELECT revision, content_identity_revision, metadata_revision, resource_revision FROM listen_local_music_tombstones
WHERE entity_type = 'track' AND entity_id = ?
`, track.FileID).Scan(&tombstoneRevision, &tombstoneIdentity, &tombstoneMetadata, &tombstoneResource); err != nil {
		t.Fatal(err)
	}
	if tombstoneRevision != 4 || tombstoneIdentity != 2 || tombstoneMetadata != 2 || tombstoneResource != 2 {
		t.Fatalf("Track tombstone revisions = (%d,%d,%d,%d), want (4,2,2,2)",
			tombstoneRevision, tombstoneIdentity, tombstoneMetadata, tombstoneResource)
	}

	stored.UpdatedAt = now.Add(5 * time.Minute)
	if err := trackRepo.Save(ctx, stored); err != nil {
		t.Fatal(err)
	}
	resurrected, err := trackRepo.Get(ctx, track.FileID)
	if err != nil || resurrected.Revision != 5 || resurrected.ContentIdentityRevision != 2 ||
		resurrected.MetadataRevision != 2 || resurrected.ResourceRevision != 2 {
		t.Fatalf("resurrected Track = %#v err=%v", resurrected, err)
	}

	membershipRepo := NewSQLiteListenLocalMusicMembershipRepository(db.Bun)
	excluded, err := library.NewListenLocalMusicMembership(library.ListenLocalMusicMembershipParams{
		FileID: track.FileID, State: "excluded", Reason: "user", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := membershipRepo.Save(ctx, excluded); err != nil {
		t.Fatal(err)
	}
	noOpExcluded := excluded
	noOpExcluded.UpdatedAt = now.Add(30 * time.Second)
	if err := membershipRepo.Save(ctx, noOpExcluded); err != nil {
		t.Fatal(err)
	}
	noOpMembership, err := membershipRepo.Get(ctx, track.FileID)
	if err != nil || noOpMembership.Revision != 1 || !noOpMembership.UpdatedAt.Equal(now) {
		t.Fatalf("no-op membership save = %#v err=%v", noOpMembership, err)
	}
	included := excluded
	included.State = library.ListenLocalMusicMembershipIncluded
	included.Reason = ""
	included.UpdatedAt = now.Add(time.Minute)
	if err := membershipRepo.Save(ctx, included); err != nil {
		t.Fatal(err)
	}
	membership, err := membershipRepo.Get(ctx, track.FileID)
	if err != nil || membership.Revision != 2 || membership.State != library.ListenLocalMusicMembershipIncluded {
		t.Fatalf("membership = %#v err=%v", membership, err)
	}
}

func TestSQLiteListenLocalTrackContentSignatureSeparatesTimelineFromMetadataResources(t *testing.T) {
	ctx := context.Background()
	db, err := openLibraryRepoTestDatabase(t, ctx, "music-content-identity.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 21, 15, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "library-content", Name: "Music", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteLibraryRepository(db.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatal(err)
	}
	fileItem, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "track-content", LibraryID: libraryItem.ID, Kind: string(library.FileKindAudio), Name: "content.mp3",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: "/tmp/content.mp3"},
		Origin:    library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{ImportPath: "/tmp/content.mp3"}},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteFileRepository(db.Bun).Save(ctx, fileItem); err != nil {
		t.Fatal(err)
	}

	duration, size := int64(180_000), int64(1_000)
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: fileItem.ID, LibraryID: libraryItem.ID, LocalPath: fileItem.Storage.LocalPath,
		Title: "Before", DurationMs: &duration, SizeBytes: &size, ModTimeUnix: 10,
		Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewSQLiteListenLocalTrackRepository(db.Bun)
	if err := repo.Save(ctx, track); err != nil {
		t.Fatal(err)
	}

	// Migrated rows start without a private signature. Recording their first
	// successful baseline must not invent a public change or invalidate resume
	// state for audio that may be unchanged.
	track.ContentIdentitySignature = "mci1p:" + strings.Repeat("1", 64)
	track.UpdatedAt = now.Add(30 * time.Second)
	if err := repo.Save(ctx, track); err != nil {
		t.Fatal(err)
	}
	track, err = repo.Get(ctx, track.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if track.ContentIdentitySignature == "" || track.Revision != 1 || track.ContentIdentityRevision != 1 ||
		track.MetadataRevision != 1 || track.ResourceRevision != 1 || !track.UpdatedAt.Equal(now) {
		t.Fatalf("migration baseline = %#v, want private signature with unchanged public revisions/timestamp", track)
	}

	// A tag rewrite changes metadata and physical byte identity, but its sampled
	// audio packets remain the same. Resource/metadata revisions advance while
	// playback progress remains attached to the same content identity.
	track.Title = "After"
	changedSize := int64(1_200)
	track.SizeBytes = &changedSize
	track.ModTimeUnix = 20
	track.UpdatedAt = now.Add(time.Minute)
	if err := repo.Save(ctx, track); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.Get(ctx, track.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 2 || stored.ContentIdentityRevision != 1 || stored.MetadataRevision != 2 || stored.ResourceRevision != 2 {
		t.Fatalf("metadata rewrite revisions=(%d,%d,%d,%d), want (2,1,2,2)",
			stored.Revision, stored.ContentIdentityRevision, stored.MetadataRevision, stored.ResourceRevision)
	}

	stored.CoverLocalPath = "/tmp/new-cover.jpg"
	stored.UpdatedAt = now.Add(2 * time.Minute)
	if err := repo.Save(ctx, stored); err != nil {
		t.Fatal(err)
	}
	stored, err = repo.Get(ctx, track.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 3 || stored.ContentIdentityRevision != 1 || stored.MetadataRevision != 2 || stored.ResourceRevision != 3 {
		t.Fatalf("artwork-only revisions=(%d,%d,%d,%d), want (3,1,2,3)",
			stored.Revision, stored.ContentIdentityRevision, stored.MetadataRevision, stored.ResourceRevision)
	}

	// A replacement with exactly the same duration and stat identity still
	// advances both logical content and byte-resource identities.
	stored.ContentIdentitySignature = "mci1p:" + strings.Repeat("2", 64)
	stored.UpdatedAt = now.Add(3 * time.Minute)
	if err := repo.Save(ctx, stored); err != nil {
		t.Fatal(err)
	}
	stored, err = repo.Get(ctx, track.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 4 || stored.ContentIdentityRevision != 2 || stored.MetadataRevision != 2 || stored.ResourceRevision != 4 {
		t.Fatalf("same-duration replacement revisions=(%d,%d,%d,%d), want (4,2,2,4)",
			stored.Revision, stored.ContentIdentityRevision, stored.MetadataRevision, stored.ResourceRevision)
	}
}
