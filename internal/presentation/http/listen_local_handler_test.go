package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type listenLocalPlaylistIdentityServiceStub struct {
	ListenLocalLibraryService
	currentRevision int64
	getCalls        int
	mutationErr     error
	updateRequest   dto.UpdateListenLocalPlaylistRequest
	deleteRequest   dto.DeleteListenLocalPlaylistRequest
	addRequest      dto.AddListenLocalPlaylistItemsRequest
	replaceRequest  dto.ReplaceListenLocalPlaylistItemsRequest
	removeRequest   dto.RemoveListenLocalPlaylistItemRequest
}

func (stub *listenLocalPlaylistIdentityServiceStub) GetListenLocalPlaylist(
	_ context.Context,
	id string,
) (dto.ListenLocalPlaylistDetailDTO, error) {
	stub.getCalls++
	revision := stub.currentRevision
	if revision < 1 {
		revision = 1
	}
	return dto.ListenLocalPlaylistDetailDTO{
		Playlist: dto.ListenLocalPlaylistDTO{ID: id, Name: "Mix", Revision: revision},
	}, nil
}

func (stub *listenLocalPlaylistIdentityServiceStub) UpdateListenLocalPlaylist(
	_ context.Context,
	request dto.UpdateListenLocalPlaylistRequest,
) (dto.ListenLocalPlaylistDTO, error) {
	stub.updateRequest = request
	if stub.mutationErr != nil {
		return dto.ListenLocalPlaylistDTO{}, stub.mutationErr
	}
	return dto.ListenLocalPlaylistDTO{ID: request.ID, Name: request.Name, Revision: request.ExpectedRevision + 1}, nil
}

func (stub *listenLocalPlaylistIdentityServiceStub) DeleteListenLocalPlaylist(
	_ context.Context,
	request dto.DeleteListenLocalPlaylistRequest,
) error {
	stub.deleteRequest = request
	return stub.mutationErr
}

func (stub *listenLocalPlaylistIdentityServiceStub) AddListenLocalPlaylistItems(
	_ context.Context,
	request dto.AddListenLocalPlaylistItemsRequest,
) (dto.ListenLocalPlaylistDetailDTO, error) {
	stub.addRequest = request
	if stub.mutationErr != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, stub.mutationErr
	}
	return dto.ListenLocalPlaylistDetailDTO{Playlist: dto.ListenLocalPlaylistDTO{ID: request.ID, Name: "Mix", Revision: 2}}, nil
}

func (stub *listenLocalPlaylistIdentityServiceStub) ReplaceListenLocalPlaylistItems(
	_ context.Context,
	request dto.ReplaceListenLocalPlaylistItemsRequest,
) (dto.ListenLocalPlaylistDetailDTO, error) {
	stub.replaceRequest = request
	if stub.mutationErr != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, stub.mutationErr
	}
	return dto.ListenLocalPlaylistDetailDTO{Playlist: dto.ListenLocalPlaylistDTO{ID: request.ID, Name: "Mix", Revision: 2}}, nil
}

func (stub *listenLocalPlaylistIdentityServiceStub) RemoveListenLocalPlaylistItem(
	_ context.Context,
	request dto.RemoveListenLocalPlaylistItemRequest,
) (dto.ListenLocalPlaylistDetailDTO, error) {
	stub.removeRequest = request
	if stub.mutationErr != nil {
		return dto.ListenLocalPlaylistDetailDTO{}, stub.mutationErr
	}
	return dto.ListenLocalPlaylistDetailDTO{Playlist: dto.ListenLocalPlaylistDTO{ID: request.ID, Name: "Mix", Revision: 2}}, nil
}

func TestWriteListenLocalDomainErrorMapsMetadataSafetyFailures(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "invalid", err: library.ErrInvalidListenLocalMetadata, status: http.StatusBadRequest, code: "metadata_invalid"},
		{name: "unsupported", err: library.ErrListenLocalMetadataUnsupported, status: http.StatusUnprocessableEntity, code: "metadata_unsupported"},
		{name: "file changed", err: library.ErrListenLocalFileChanged, status: http.StatusConflict, code: "metadata_file_changed"},
		{name: "file busy", err: library.ErrListenLocalFileBusy, status: http.StatusLocked, code: "metadata_file_busy"},
		{name: "permission", err: library.ErrListenLocalFilePermission, status: http.StatusForbidden, code: "metadata_file_permission"},
		{name: "index stale", err: library.ErrListenLocalMetadataIndexStale, status: http.StatusServiceUnavailable, code: "metadata_committed_index_stale"},
		{name: "playlist revision", err: &library.ListenLocalMusicRevisionConflictError{CurrentRevision: 4}, status: http.StatusConflict, code: "playlist_revision_conflict"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeListenLocalDomainError(response, test.err)
			if response.Code != test.status {
				t.Fatalf("got status %d, want %d", response.Code, test.status)
			}
			payload := struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}{}
			if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload.Error.Code != test.code {
				t.Fatalf("got code %q, want %q", payload.Error.Code, test.code)
			}
			if payload.Error.Message != test.err.Error() {
				t.Fatalf("got message %q, want %q", payload.Error.Message, test.err.Error())
			}
		})
	}
}

func TestListenLocalPlaylistItemRoutesCarryStableIdentity(t *testing.T) {
	stub := new(listenLocalPlaylistIdentityServiceStub)
	handler := NewListenLocalHandler(stub)

	reorder := httptest.NewRecorder()
	handler.ServeHTTP(reorder, httptest.NewRequest(
		http.MethodPut,
		"/api/listen/local/playlists/playlist-1/items",
		strings.NewReader(`{"itemIds":["item-b","item-a"],"expectedRevision":3}`),
	))
	if reorder.Code != http.StatusOK || stub.replaceRequest.ID != "playlist-1" ||
		len(stub.replaceRequest.ItemIDs) != 2 || stub.replaceRequest.ItemIDs[0] != "item-b" ||
		stub.replaceRequest.ExpectedRevision != 3 {
		t.Fatalf("identity reorder status=%d request=%#v body=%s", reorder.Code, stub.replaceRequest, reorder.Body.String())
	}

	remove := httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(
		http.MethodDelete,
		"/api/listen/local/playlists/playlist-1/items?itemId=item%2Fone&expectedRevision=3",
		nil,
	))
	if remove.Code != http.StatusOK || stub.removeRequest.ID != "playlist-1" ||
		stub.removeRequest.ItemID != "item/one" || stub.removeRequest.FileID != "" ||
		stub.removeRequest.ExpectedRevision != 3 {
		t.Fatalf("identity remove status=%d request=%#v body=%s", remove.Code, stub.removeRequest, remove.Body.String())
	}
}

func TestListenLocalPlaylistMutationRoutesCarryExpectedRevision(t *testing.T) {
	stub := new(listenLocalPlaylistIdentityServiceStub)
	handler := NewListenLocalHandler(stub)

	rename := httptest.NewRecorder()
	handler.ServeHTTP(rename, httptest.NewRequest(
		http.MethodPatch,
		"/api/listen/local/playlists/playlist-1",
		strings.NewReader(`{"name":"Renamed","expectedRevision":7}`),
	))
	if rename.Code != http.StatusOK || stub.updateRequest.ID != "playlist-1" ||
		stub.updateRequest.Name != "Renamed" || stub.updateRequest.ExpectedRevision != 7 {
		t.Fatalf("rename status=%d request=%#v body=%s", rename.Code, stub.updateRequest, rename.Body.String())
	}

	add := httptest.NewRecorder()
	handler.ServeHTTP(add, httptest.NewRequest(
		http.MethodPost,
		"/api/listen/local/playlists/playlist-1/items",
		strings.NewReader(`{"fileIds":["track-1"],"expectedRevision":7}`),
	))
	if add.Code != http.StatusOK || stub.addRequest.ID != "playlist-1" ||
		len(stub.addRequest.FileIDs) != 1 || stub.addRequest.ExpectedRevision != 7 {
		t.Fatalf("add status=%d request=%#v body=%s", add.Code, stub.addRequest, add.Body.String())
	}

	remove := httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(
		http.MethodDelete,
		"/api/listen/local/playlists/playlist-1?expectedRevision=7",
		nil,
	))
	if remove.Code != http.StatusNoContent || stub.deleteRequest.ID != "playlist-1" ||
		stub.deleteRequest.ExpectedRevision != 7 {
		t.Fatalf("delete status=%d request=%#v body=%s", remove.Code, stub.deleteRequest, remove.Body.String())
	}
	if stub.getCalls != 0 {
		t.Fatalf("explicit revisions unexpectedly resolved playlist %d times", stub.getCalls)
	}

	zero := httptest.NewRecorder()
	handler.ServeHTTP(zero, httptest.NewRequest(
		http.MethodPatch,
		"/api/listen/local/playlists/playlist-1",
		strings.NewReader(`{"name":"Invalid","expectedRevision":0}`),
	))
	if zero.Code != http.StatusBadRequest || stub.getCalls != 0 {
		t.Fatalf("explicit zero revision status=%d gets=%d body=%s", zero.Code, stub.getCalls, zero.Body.String())
	}

	duplicated := httptest.NewRecorder()
	handler.ServeHTTP(duplicated, httptest.NewRequest(
		http.MethodDelete,
		"/api/listen/local/playlists/playlist-1?expectedRevision=7&expectedRevision=8",
		nil,
	))
	if duplicated.Code != http.StatusBadRequest || stub.getCalls != 0 {
		t.Fatalf("delete with duplicate expectedRevision status=%d gets=%d body=%s",
			duplicated.Code, stub.getCalls, duplicated.Body.String())
	}
}

func TestListenLocalPlaylistLegacyMutationShapesResolveCurrentRevision(t *testing.T) {
	stub := &listenLocalPlaylistIdentityServiceStub{currentRevision: 12}
	handler := NewListenLocalHandler(stub)

	rename := httptest.NewRecorder()
	handler.ServeHTTP(rename, httptest.NewRequest(
		http.MethodPatch,
		"/api/listen/local/playlists/playlist-1",
		strings.NewReader(`{"name":"Legacy rename"}`),
	))
	if rename.Code != http.StatusOK || stub.updateRequest.ExpectedRevision != 12 {
		t.Fatalf("legacy rename status=%d request=%#v body=%s", rename.Code, stub.updateRequest, rename.Body.String())
	}

	add := httptest.NewRecorder()
	handler.ServeHTTP(add, httptest.NewRequest(
		http.MethodPost,
		"/api/listen/local/playlists/playlist-1/items",
		strings.NewReader(`{"fileIds":["track-1"]}`),
	))
	if add.Code != http.StatusOK || stub.addRequest.ExpectedRevision != 12 ||
		len(stub.addRequest.FileIDs) != 1 || stub.addRequest.FileIDs[0] != "track-1" {
		t.Fatalf("legacy add status=%d request=%#v body=%s", add.Code, stub.addRequest, add.Body.String())
	}

	reorder := httptest.NewRecorder()
	handler.ServeHTTP(reorder, httptest.NewRequest(
		http.MethodPut,
		"/api/listen/local/playlists/playlist-1/items",
		strings.NewReader(`{"fileIds":["track-b","track-a"]}`),
	))
	if reorder.Code != http.StatusOK || stub.replaceRequest.ExpectedRevision != 12 ||
		len(stub.replaceRequest.FileIDs) != 2 || len(stub.replaceRequest.ItemIDs) != 0 {
		t.Fatalf("legacy reorder status=%d request=%#v body=%s", reorder.Code, stub.replaceRequest, reorder.Body.String())
	}

	removeItem := httptest.NewRecorder()
	handler.ServeHTTP(removeItem, httptest.NewRequest(
		http.MethodDelete,
		"/api/listen/local/playlists/playlist-1/items?fileId=track%2Fone",
		nil,
	))
	if removeItem.Code != http.StatusOK || stub.removeRequest.ExpectedRevision != 12 ||
		stub.removeRequest.FileID != "track/one" || stub.removeRequest.ItemID != "" {
		t.Fatalf("legacy remove status=%d request=%#v body=%s", removeItem.Code, stub.removeRequest, removeItem.Body.String())
	}

	removePlaylist := httptest.NewRecorder()
	handler.ServeHTTP(removePlaylist, httptest.NewRequest(
		http.MethodDelete,
		"/api/listen/local/playlists/playlist-1",
		nil,
	))
	if removePlaylist.Code != http.StatusNoContent || stub.deleteRequest.ExpectedRevision != 12 {
		t.Fatalf("legacy delete status=%d request=%#v body=%s",
			removePlaylist.Code, stub.deleteRequest, removePlaylist.Body.String())
	}
	if stub.getCalls != 5 {
		t.Fatalf("legacy mutations resolved revision %d times, want 5", stub.getCalls)
	}
}

func TestListenLocalPlaylistLegacyRevisionFallbackStillConflictsOnRace(t *testing.T) {
	stub := &listenLocalPlaylistIdentityServiceStub{
		currentRevision: 4,
		mutationErr:     &library.ListenLocalMusicRevisionConflictError{CurrentRevision: 5},
	}
	handler := NewListenLocalHandler(stub)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(
		http.MethodPost,
		"/api/listen/local/playlists/playlist-1/items",
		strings.NewReader(`{"fileIds":["track-1"]}`),
	))
	if response.Code != http.StatusConflict || stub.getCalls != 1 || stub.addRequest.ExpectedRevision != 4 ||
		!strings.Contains(response.Body.String(), `"code":"playlist_revision_conflict"`) {
		t.Fatalf("legacy race status=%d gets=%d request=%#v body=%s",
			response.Code, stub.getCalls, stub.addRequest, response.Body.String())
	}
}
