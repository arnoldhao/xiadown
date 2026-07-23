package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type ListenLocalLibraryService interface {
	ListListenLocalTracks(context.Context, dto.ListListenLocalTracksRequest) ([]dto.ListenLocalTrackDTO, error)
	RefreshListenLocalIndex(context.Context, dto.RefreshListenLocalIndexRequest) (dto.ListenLocalIndexRefreshResponse, error)
	RemoveListenLocalTrack(context.Context, dto.RemoveListenLocalTrackRequest) error
	ClearMissingListenLocalTracks(context.Context) (dto.ClearMissingListenLocalTracksResponse, error)
	UpdateListenLocalTrackMetadata(context.Context, dto.UpdateListenLocalTrackMetadataRequest) (dto.ListenLocalTrackDTO, error)
	ListListenLocalPlaylists(context.Context) ([]dto.ListenLocalPlaylistDTO, error)
	GetListenLocalPlaylist(context.Context, string) (dto.ListenLocalPlaylistDetailDTO, error)
	CreateListenLocalPlaylist(context.Context, dto.CreateListenLocalPlaylistRequest) (dto.ListenLocalPlaylistDTO, error)
	UpdateListenLocalPlaylist(context.Context, dto.UpdateListenLocalPlaylistRequest) (dto.ListenLocalPlaylistDTO, error)
	DeleteListenLocalPlaylist(context.Context, dto.DeleteListenLocalPlaylistRequest) error
	AddListenLocalPlaylistItems(context.Context, dto.AddListenLocalPlaylistItemsRequest) (dto.ListenLocalPlaylistDetailDTO, error)
	ReplaceListenLocalPlaylistItems(context.Context, dto.ReplaceListenLocalPlaylistItemsRequest) (dto.ListenLocalPlaylistDetailDTO, error)
	RemoveListenLocalPlaylistItem(context.Context, dto.RemoveListenLocalPlaylistItemRequest) (dto.ListenLocalPlaylistDetailDTO, error)
}

type ListenLocalHandler struct {
	library ListenLocalLibraryService
}

// listenLocalRevisionField distinguishes a legacy body which omitted
// expectedRevision from a malformed body which supplied zero, null, or a
// non-integer value. Only the former is eligible for the compatibility
// fallback below.
type listenLocalRevisionField struct {
	present bool
	value   int64
}

func (field *listenLocalRevisionField) UnmarshalJSON(data []byte) error {
	field.present = true
	if strings.TrimSpace(string(data)) == "null" {
		return library.ErrInvalidListenLocalPlaylist
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil || value < 1 {
		return library.ErrInvalidListenLocalPlaylist
	}
	field.value = value
	return nil
}

func (field listenLocalRevisionField) optional() *int64 {
	if !field.present {
		return nil
	}
	value := field.value
	return &value
}

type listenLocalUpdatePlaylistWireRequest struct {
	ID               string                   `json:"id,omitempty"`
	Name             string                   `json:"name"`
	ExpectedRevision listenLocalRevisionField `json:"expectedRevision"`
}

type listenLocalAddPlaylistItemsWireRequest struct {
	ID               string                   `json:"id,omitempty"`
	FileIDs          []string                 `json:"fileIds"`
	ExpectedRevision listenLocalRevisionField `json:"expectedRevision"`
}

type listenLocalReplacePlaylistItemsWireRequest struct {
	ID               string                   `json:"id,omitempty"`
	FileIDs          []string                 `json:"fileIds"`
	ItemIDs          []string                 `json:"itemIds,omitempty"`
	ExpectedRevision listenLocalRevisionField `json:"expectedRevision"`
}

func NewListenLocalHandler(library ListenLocalLibraryService) *ListenLocalHandler {
	return &ListenLocalHandler{library: library}
}

func (handler *ListenLocalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	setCORSHeaders(w, r)
	if handler == nil || handler.library == nil {
		writeListenLocalError(w, http.StatusServiceUnavailable, "local library unavailable")
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/listen/local"), "/")
	switch {
	case path == "playlists" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		handler.serveListPlaylists(w, r)
	case path == "playlists" && r.Method == http.MethodPost:
		handler.serveCreatePlaylist(w, r)
	case strings.HasPrefix(path, "playlists/"):
		handler.servePlaylistPath(w, r, strings.TrimPrefix(path, "playlists/"))
	case path == "" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		handler.serveList(w, r)
	case path == "refresh" && r.Method == http.MethodPost:
		handler.serveRefresh(w, r)
	case path == "metadata" && (r.Method == http.MethodPatch || r.Method == http.MethodPut):
		handler.serveUpdateMetadata(w, r)
	case path == "clear-missing" && r.Method == http.MethodPost:
		handler.serveClearMissing(w, r)
	case path == "" && r.Method == http.MethodDelete:
		handler.serveRemove(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (handler *ListenLocalHandler) serveListPlaylists(w http.ResponseWriter, r *http.Request) {
	items, err := handler.library.ListListenLocalPlaylists(r.Context())
	if err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLocalJSON(w, r, map[string]any{"items": items})
}

func (handler *ListenLocalHandler) serveCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	request := dto.CreateListenLocalPlaylistRequest{}
	if err := decodeListenLocalJSON(r, &request); err != nil {
		writeListenLocalError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := handler.library.CreateListenLocalPlaylist(r.Context(), request)
	if err != nil {
		writeListenLocalDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	writeListenLocalJSON(w, r, item)
}

func (handler *ListenLocalHandler) servePlaylistPath(w http.ResponseWriter, r *http.Request, suffix string) {
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeListenLocalError(w, http.StatusNotFound, "playlist not found")
		return
	}
	id := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			item, err := handler.library.GetListenLocalPlaylist(r.Context(), id)
			if err != nil {
				writeListenLocalDomainError(w, err)
				return
			}
			writeListenLocalJSON(w, r, item)
		case http.MethodPatch, http.MethodPut:
			wireRequest := listenLocalUpdatePlaylistWireRequest{}
			if err := decodeListenLocalJSON(r, &wireRequest); err != nil {
				writeListenLocalError(w, http.StatusBadRequest, err.Error())
				return
			}
			expectedRevision, err := handler.resolveListenLocalExpectedRevision(
				r.Context(), id, wireRequest.ExpectedRevision.optional(),
			)
			if err != nil {
				writeListenLocalDomainError(w, err)
				return
			}
			request := dto.UpdateListenLocalPlaylistRequest{
				ID: id, Name: wireRequest.Name, ExpectedRevision: expectedRevision,
			}
			item, err := handler.library.UpdateListenLocalPlaylist(r.Context(), request)
			if err != nil {
				writeListenLocalDomainError(w, err)
				return
			}
			writeListenLocalJSON(w, r, item)
		case http.MethodDelete:
			expectedRevision, err := listenLocalExpectedRevision(r)
			if err != nil {
				writeListenLocalDomainError(w, err)
				return
			}
			resolvedRevision, err := handler.resolveListenLocalExpectedRevision(r.Context(), id, expectedRevision)
			if err != nil {
				writeListenLocalDomainError(w, err)
				return
			}
			if err := handler.library.DeleteListenLocalPlaylist(r.Context(), dto.DeleteListenLocalPlaylistRequest{
				ID: id, ExpectedRevision: resolvedRevision,
			}); err != nil {
				writeListenLocalDomainError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) != 2 || parts[1] != "items" {
		writeListenLocalError(w, http.StatusNotFound, "playlist route not found")
		return
	}
	switch r.Method {
	case http.MethodPost:
		wireRequest := listenLocalAddPlaylistItemsWireRequest{}
		if err := decodeListenLocalJSON(r, &wireRequest); err != nil {
			writeListenLocalError(w, http.StatusBadRequest, err.Error())
			return
		}
		expectedRevision, err := handler.resolveListenLocalExpectedRevision(
			r.Context(), id, wireRequest.ExpectedRevision.optional(),
		)
		if err != nil {
			writeListenLocalDomainError(w, err)
			return
		}
		request := dto.AddListenLocalPlaylistItemsRequest{
			ID: id, FileIDs: wireRequest.FileIDs, ExpectedRevision: expectedRevision,
		}
		item, err := handler.library.AddListenLocalPlaylistItems(r.Context(), request)
		if err != nil {
			writeListenLocalDomainError(w, err)
			return
		}
		writeListenLocalJSON(w, r, item)
	case http.MethodPut:
		wireRequest := listenLocalReplacePlaylistItemsWireRequest{}
		if err := decodeListenLocalJSON(r, &wireRequest); err != nil {
			writeListenLocalError(w, http.StatusBadRequest, err.Error())
			return
		}
		expectedRevision, err := handler.resolveListenLocalExpectedRevision(
			r.Context(), id, wireRequest.ExpectedRevision.optional(),
		)
		if err != nil {
			writeListenLocalDomainError(w, err)
			return
		}
		request := dto.ReplaceListenLocalPlaylistItemsRequest{
			ID: id, FileIDs: wireRequest.FileIDs, ItemIDs: wireRequest.ItemIDs,
			ExpectedRevision: expectedRevision,
		}
		item, err := handler.library.ReplaceListenLocalPlaylistItems(r.Context(), request)
		if err != nil {
			writeListenLocalDomainError(w, err)
			return
		}
		writeListenLocalJSON(w, r, item)
	case http.MethodDelete:
		expectedRevision, err := listenLocalExpectedRevision(r)
		if err != nil {
			writeListenLocalDomainError(w, err)
			return
		}
		resolvedRevision, err := handler.resolveListenLocalExpectedRevision(r.Context(), id, expectedRevision)
		if err != nil {
			writeListenLocalDomainError(w, err)
			return
		}
		item, err := handler.library.RemoveListenLocalPlaylistItem(r.Context(), dto.RemoveListenLocalPlaylistItemRequest{
			ID: id, ItemID: strings.TrimSpace(r.URL.Query().Get("itemId")),
			FileID: strings.TrimSpace(r.URL.Query().Get("fileId")), ExpectedRevision: resolvedRevision,
		})
		if err != nil {
			writeListenLocalDomainError(w, err)
			return
		}
		writeListenLocalJSON(w, r, item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func listenLocalExpectedRevision(r *http.Request) (*int64, error) {
	values, exists := r.URL.Query()["expectedRevision"]
	if !exists {
		return nil, nil
	}
	if len(values) != 1 {
		return nil, library.ErrInvalidListenLocalPlaylist
	}
	raw := strings.TrimSpace(values[0])
	revision, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || revision < 1 {
		return nil, library.ErrInvalidListenLocalPlaylist
	}
	return &revision, nil
}

func (handler *ListenLocalHandler) resolveListenLocalExpectedRevision(
	ctx context.Context,
	playlistID string,
	expectedRevision *int64,
) (int64, error) {
	if expectedRevision != nil {
		if *expectedRevision < 1 {
			return 0, library.ErrInvalidListenLocalPlaylist
		}
		return *expectedRevision, nil
	}

	// Legacy loopback callers predate revision tokens. Snapshot the current
	// revision at the HTTP boundary, then send it through the same strict
	// service check as a modern caller. If another mutation wins between this
	// read and the service's mutation lock, the service returns 409 instead of
	// overwriting it.
	detail, err := handler.library.GetListenLocalPlaylist(ctx, strings.TrimSpace(playlistID))
	if err != nil {
		return 0, err
	}
	if detail.Playlist.Revision < 1 {
		return 0, library.ErrInvalidListenLocalPlaylist
	}
	return detail.Playlist.Revision, nil
}

func decodeListenLocalJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request must contain one JSON object")
		}
		return err
	}
	return nil
}

func (handler *ListenLocalHandler) serveList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
	items, err := handler.library.ListListenLocalTracks(r.Context(), dto.ListListenLocalTracksRequest{
		Query:              r.URL.Query().Get("query"),
		Artist:             r.URL.Query().Get("artist"),
		Album:              r.URL.Query().Get("album"),
		Sort:               r.URL.Query().Get("sort"),
		IncludeUnavailable: strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeUnavailable")), "true"),
		Limit:              limit,
		Offset:             offset,
	})
	if err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLocalJSON(w, r, map[string]any{"items": items})
}

func (handler *ListenLocalHandler) serveRefresh(w http.ResponseWriter, r *http.Request) {
	request := dto.RefreshListenLocalIndexRequest{
		FileID:    strings.TrimSpace(r.URL.Query().Get("fileId")),
		LibraryID: strings.TrimSpace(r.URL.Query().Get("libraryId")),
	}
	response, err := handler.library.RefreshListenLocalIndex(r.Context(), request)
	if err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLocalJSON(w, r, response)
}

func (handler *ListenLocalHandler) serveClearMissing(w http.ResponseWriter, r *http.Request) {
	response, err := handler.library.ClearMissingListenLocalTracks(r.Context())
	if err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeListenLocalJSON(w, r, response)
}

func (handler *ListenLocalHandler) serveUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	request := dto.UpdateListenLocalTrackMetadataRequest{}
	if err := decodeListenLocalJSON(r, &request); err != nil {
		writeListenLocalError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := handler.library.UpdateListenLocalTrackMetadata(r.Context(), request)
	if err != nil {
		writeListenLocalDomainError(w, err)
		return
	}
	writeListenLocalJSON(w, r, item)
}

func (handler *ListenLocalHandler) serveRemove(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimSpace(r.URL.Query().Get("fileId"))
	if fileID == "" {
		writeListenLocalError(w, http.StatusBadRequest, "fileId is required")
		return
	}
	if err := handler.library.RemoveListenLocalTrack(r.Context(), dto.RemoveListenLocalTrackRequest{FileID: fileID}); err != nil {
		writeListenLocalError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeListenLocalJSON(w http.ResponseWriter, r *http.Request, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeListenLocalError(w http.ResponseWriter, status int, message string) {
	writeListenLocalCodedError(w, status, "", message)
}

func writeListenLocalCodedError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	errorPayload := map[string]string{"message": strings.TrimSpace(message)}
	if normalizedCode := strings.TrimSpace(code); normalizedCode != "" {
		errorPayload["code"] = normalizedCode
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"error": errorPayload})
}

func writeListenLocalDomainError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := ""
	switch {
	case errors.Is(err, library.ErrListenLocalPlaylistNotFound), errors.Is(err, library.ErrFileNotFound):
		status = http.StatusNotFound
	case errors.Is(err, library.ErrInvalidListenLocalPlaylist):
		status = http.StatusBadRequest
	case errors.Is(err, library.ErrListenLocalMusicRevisionConflict):
		status = http.StatusConflict
		code = "playlist_revision_conflict"
	case errors.Is(err, library.ErrInvalidListenLocalMetadata):
		status = http.StatusBadRequest
		code = "metadata_invalid"
	case errors.Is(err, library.ErrListenLocalMetadataUnsupported):
		status = http.StatusUnprocessableEntity
		code = "metadata_unsupported"
	case errors.Is(err, library.ErrListenLocalFileChanged):
		status = http.StatusConflict
		code = "metadata_file_changed"
	case errors.Is(err, library.ErrListenLocalFileBusy):
		status = http.StatusLocked
		code = "metadata_file_busy"
	case errors.Is(err, library.ErrListenLocalFilePermission):
		status = http.StatusForbidden
		code = "metadata_file_permission"
	case errors.Is(err, library.ErrListenLocalMetadataIndexStale):
		status = http.StatusServiceUnavailable
		code = "metadata_committed_index_stale"
	}
	writeListenLocalCodedError(w, status, code, err.Error())
}
