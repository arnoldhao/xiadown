package library

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewListenLocalPlaylistNormalizesNameAndTimes(t *testing.T) {
	createdAt := time.Date(2026, 7, 10, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	item, err := NewListenLocalPlaylist(ListenLocalPlaylistParams{
		ID: " playlist-1 ", Name: " Road Trip ", CreatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new playlist: %v", err)
	}
	if item.ID != "playlist-1" || item.Name != "Road Trip" {
		t.Fatalf("unexpected normalized playlist: %#v", item)
	}
	if item.CreatedAt.Location() != time.UTC || !item.UpdatedAt.Equal(item.CreatedAt) {
		t.Fatalf("unexpected timestamps: %#v", item)
	}
}

func TestNewListenLocalPlaylistRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"", "   ", strings.Repeat("x", ListenLocalPlaylistNameMaxLength+1)} {
		_, err := NewListenLocalPlaylist(ListenLocalPlaylistParams{ID: "playlist-1", Name: name})
		if !errors.Is(err, ErrInvalidListenLocalPlaylist) {
			t.Fatalf("expected invalid playlist for %q, got %v", name, err)
		}
	}
}
