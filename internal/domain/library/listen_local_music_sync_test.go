package library

import (
	"testing"
	"time"
)

func TestListenLocalMusicSyncDomainDefaultsAndValidation(t *testing.T) {
	now := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	track, err := NewListenLocalTrack(ListenLocalTrackParams{
		FileID: "track-1", LibraryID: "library-1", LocalPath: "/music/track.mp3", Title: "Track",
		Availability: ListenLocalTrackAvailable, LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if track.Revision != 1 || track.ContentIdentityRevision != 1 {
		t.Fatalf("track revisions = (%d,%d), want (1,1)", track.Revision, track.ContentIdentityRevision)
	}
	if _, err := NewListenLocalTrack(ListenLocalTrackParams{
		FileID: "track-1", LibraryID: "library-1", LocalPath: "/music/track.mp3", Title: "Track",
		Revision: -1, ContentIdentityRevision: 1,
	}); err == nil {
		t.Fatal("negative Track revision unexpectedly accepted")
	}

	first, err := NewListenLocalPlaylistItem("playlist-1", track.FileID, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewListenLocalPlaylistItem("playlist-1", track.FileID, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || second.ID == "" || first.ID == second.ID || first.Revision != 1 || second.Revision != 1 {
		t.Fatalf("playlist item identities/revisions are not independent: first=%#v second=%#v", first, second)
	}

	membership, err := NewListenLocalMusicMembership(ListenLocalMusicMembershipParams{
		FileID: track.FileID, State: " EXCLUDED ", Reason: " USER ", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !membership.IsUserExcluded() || membership.Revision != 1 {
		t.Fatalf("unexpected membership: %#v", membership)
	}
	if _, err := NewListenLocalMusicMembership(ListenLocalMusicMembershipParams{
		FileID: track.FileID, State: "excluded", Reason: "unknown",
	}); err == nil {
		t.Fatal("unknown membership reason unexpectedly accepted")
	}
}
