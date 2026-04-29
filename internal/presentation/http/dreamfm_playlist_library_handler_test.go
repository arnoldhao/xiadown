package http

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDreamFMPlaylistLibraryHandlerAddsPlaylist(t *testing.T) {
	var gotPlaylistID string
	handler := NewDreamFMPlaylistLibraryHandler(fakeDreamFMMusicClient{
		subscribeFunc: func(_ context.Context, playlistID string) error {
			gotPlaylistID = playlistID
			return nil
		},
	})
	request := httptest.NewRequest("POST", "/api/dreamfm/library/playlist", strings.NewReader(`{"playlistId":"VLPL1234567890","action":"add"}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if gotPlaylistID != "VLPL1234567890" {
		t.Fatalf("unexpected playlist id: %q", gotPlaylistID)
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestDreamFMPlaylistLibraryHandlerRemovesPlaylist(t *testing.T) {
	var gotPlaylistID string
	handler := NewDreamFMPlaylistLibraryHandler(fakeDreamFMMusicClient{
		unsubscribeFunc: func(_ context.Context, playlistID string) error {
			gotPlaylistID = playlistID
			return nil
		},
	})
	request := httptest.NewRequest("POST", "/api/dreamfm/library/playlist", strings.NewReader(`{"playlistId":"VLPL1234567890","action":"remove"}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if gotPlaylistID != "VLPL1234567890" {
		t.Fatalf("unexpected playlist id: %q", gotPlaylistID)
	}
}
